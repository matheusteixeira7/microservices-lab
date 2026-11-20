// Package postgres implements app.Store with Postgres (pgx + sqlc).
//
// Locking, in this order inside every transaction:
//  1. the order: pg_advisory_xact_lock (works even before the reservation row exists)
//  2. the products: SELECT ... FOR UPDATE, sorted by SKU
//
// Same order everywhere = no deadlocks. Every call uses the caller's ctx, so a
// client deadline cancels the transaction and frees the locks.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"apps/estoque/internal/inventory/app"
	"apps/estoque/internal/inventory/domain"
	"apps/estoque/internal/inventory/postgres/db"
)

type Store struct {
	pool *pgxpool.Pool
}

var _ app.Store = (*Store)(nil)

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Products(ctx context.Context, skus []string) ([]*domain.Product, error) {
	rows, err := db.New(s.pool).ListProducts(ctx, skus)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	products := make([]*domain.Product, 0, len(rows))
	for _, r := range rows {
		products = append(products, toProduct(r.Sku, r.Name, r.PriceCents, r.Currency, r.OnHand, r.Reserved))
	}
	return products, nil
}

func (s *Store) InTx(ctx context.Context, fn func(ctx context.Context, tx app.Tx) error) (err error) {
	pgTx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() {
		if err != nil {
			_ = pgTx.Rollback(context.WithoutCancel(ctx)) // also runs when ctx was cancelled
		}
	}()

	if err = fn(ctx, &tx{q: db.New(pgTx)}); err != nil {
		return err
	}
	if err = pgTx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

type tx struct {
	q *db.Queries
}

func (t *tx) Reservation(ctx context.Context, orderID string) (*domain.Reservation, error) {
	// 1st lock: the order. Two requests for the same order now run one after the other.
	if err := t.q.LockOrder(ctx, orderID); err != nil {
		return nil, fmt.Errorf("lock order: %w", err)
	}

	row, err := t.q.GetReservation(ctx, orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get reservation: %w", err)
	}

	lineRows, err := t.q.ListReservationLines(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("list reservation lines: %w", err)
	}
	lines := make([]domain.Line, 0, len(lineRows))
	for _, l := range lineRows {
		lines = append(lines, domain.Line{SKU: l.Sku, Quantity: int(l.Quantity)})
	}
	return domain.RestoreReservation(row.OrderID, lines, domain.ReservationStatus(row.Status)), nil
}

func (t *tx) LockProducts(ctx context.Context, skus []string) (map[string]*domain.Product, error) {
	// 2nd lock: the products, sorted by SKU (ORDER BY in the query).
	rows, err := t.q.LockProducts(ctx, skus)
	if err != nil {
		return nil, fmt.Errorf("lock products: %w", err)
	}
	products := make(map[string]*domain.Product, len(rows))
	for _, r := range rows {
		products[r.Sku] = toProduct(r.Sku, r.Name, r.PriceCents, r.Currency, r.OnHand, r.Reserved)
	}
	return products, nil
}

func (t *tx) SaveProducts(ctx context.Context, products ...*domain.Product) error {
	for _, p := range products {
		affected, err := t.q.UpdateProductStock(ctx, db.UpdateProductStockParams{
			Sku:      p.SKU(),
			OnHand:   int32(p.OnHand()),
			Reserved: int32(p.Reserved()),
		})
		if err != nil {
			return fmt.Errorf("update product %s: %w", p.SKU(), err)
		}
		if affected != 1 {
			return fmt.Errorf("update product %s: %d rows affected, want 1", p.SKU(), affected)
		}
	}
	return nil
}

func (t *tx) SaveReservation(ctx context.Context, r *domain.Reservation) error {
	err := t.q.UpsertReservation(ctx, db.UpsertReservationParams{OrderID: r.OrderID(), Status: string(r.Status())})
	if err != nil {
		return fmt.Errorf("save reservation %s: %w", r.OrderID(), err)
	}
	for _, line := range r.Lines() {
		err := t.q.InsertReservationLine(ctx, db.InsertReservationLineParams{
			OrderID:  r.OrderID(),
			Sku:      line.SKU,
			Quantity: int32(line.Quantity),
		})
		if err != nil {
			return fmt.Errorf("save reservation line %s/%s: %w", r.OrderID(), line.SKU, err)
		}
	}
	return nil
}

func toProduct(sku, name string, priceCents int64, currency string, onHand, reserved int32) *domain.Product {
	return domain.RestoreProduct(sku, name, domain.Money{Cents: priceCents, Currency: currency}, int(onHand), int(reserved))
}
