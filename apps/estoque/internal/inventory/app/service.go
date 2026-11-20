package app

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"apps/estoque/internal/inventory/domain"
)

// ErrReservationNotFound means there is no reservation for the order.
var ErrReservationNotFound = errors.New("reservation not found")

// ProductInfo is the catalog data other services need (plain data).
type ProductInfo struct {
	SKU        string
	Name       string
	PriceCents int64
	Currency   string
}

// Service holds the inventory use cases.
//
// Errors:
//   - *domain.OutOfStockError: a business "no", not a failure.
//   - other domain.Err* values: invalid requests or transitions.
//   - anything else: a technical failure (storage), wrapped with %w.
type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

// FindProducts returns catalog data for the SKUs that exist.
func (s *Service) FindProducts(ctx context.Context, skus []string) ([]ProductInfo, error) {
	products, err := s.store.Products(ctx, skus)
	if err != nil {
		return nil, fmt.Errorf("find products: %w", err)
	}
	infos := make([]ProductInfo, 0, len(products))
	for _, p := range products {
		infos = append(infos, ProductInfo{SKU: p.SKU(), Name: p.Name(), PriceCents: p.Price().Cents, Currency: p.Price().Currency})
	}
	return infos, nil
}

// Reserve holds stock for an order, all-or-nothing.
// The order id is the idempotency key: repeating the same request is safe.
func (s *Service) Reserve(ctx context.Context, orderID string, lines []domain.Line) error {
	reservation, err := domain.NewReservation(orderID, lines)
	if err != nil {
		return err
	}

	return s.store.InTx(ctx, func(ctx context.Context, tx Tx) error {
		existing, err := tx.Reservation(ctx, orderID)
		if err != nil {
			return fmt.Errorf("load reservation: %w", err)
		}
		if existing != nil {
			switch {
			case existing.Status() == domain.StatusReleased:
				return domain.ErrReservationReleased // cancelled before (or tombstone)
			case !existing.Matches(lines):
				return domain.ErrIdempotencyKeyReuse
			default:
				return nil // same request again: already done
			}
		}

		products, err := tx.LockProducts(ctx, reservation.SKUs())
		if err != nil {
			return fmt.Errorf("lock products: %w", err)
		}
		if err := domain.Place(reservation, products); err != nil {
			return err
		}
		return save(ctx, tx, reservation, products)
	})
}

// Release gives the stock of an order back (order cancelled).
// If the release arrives before the reservation, it leaves a tombstone so a
// late Reserve for the same order fails.
func (s *Service) Release(ctx context.Context, orderID string) error {
	return s.store.InTx(ctx, func(ctx context.Context, tx Tx) error {
		reservation, err := tx.Reservation(ctx, orderID)
		if err != nil {
			return fmt.Errorf("load reservation: %w", err)
		}
		if reservation == nil {
			if err := tx.SaveReservation(ctx, domain.NewReleasedTombstone(orderID)); err != nil {
				return fmt.Errorf("save tombstone: %w", err)
			}
			return nil
		}
		return s.transition(ctx, tx, reservation, domain.Release)
	})
}

// Commit takes the stock of an order out of the warehouse (order confirmed).
func (s *Service) Commit(ctx context.Context, orderID string) error {
	return s.store.InTx(ctx, func(ctx context.Context, tx Tx) error {
		reservation, err := tx.Reservation(ctx, orderID)
		if err != nil {
			return fmt.Errorf("load reservation: %w", err)
		}
		if reservation == nil {
			return fmt.Errorf("%w: order %s", ErrReservationNotFound, orderID)
		}
		return s.transition(ctx, tx, reservation, domain.Commit)
	})
}

func (s *Service) transition(
	ctx context.Context,
	tx Tx,
	reservation *domain.Reservation,
	apply func(*domain.Reservation, map[string]*domain.Product) error,
) error {
	products, err := tx.LockProducts(ctx, reservation.SKUs())
	if err != nil {
		return fmt.Errorf("lock products: %w", err)
	}
	if err := apply(reservation, products); err != nil {
		return err
	}
	return save(ctx, tx, reservation, products)
}

func save(ctx context.Context, tx Tx, reservation *domain.Reservation, products map[string]*domain.Product) error {
	if err := tx.SaveProducts(ctx, slices.Collect(maps.Values(products))...); err != nil {
		return fmt.Errorf("save products: %w", err)
	}
	if err := tx.SaveReservation(ctx, reservation); err != nil {
		return fmt.Errorf("save reservation: %w", err)
	}
	return nil
}
