// Package memory is an in-memory app.Store, for tests and as a reference for
// the Postgres version. It behaves like a database on purpose:
//   - it stores copies (rows), never the domain objects, so a change without
//     Save is not kept;
//   - a transaction holds a lock until it ends (one transaction at a time);
//   - writes are staged and only applied if the transaction succeeds.
package memory

import (
	"context"
	"slices"
	"sync"

	"apps/estoque/internal/inventory/app"
	"apps/estoque/internal/inventory/domain"
)

type productRow struct {
	sku, name        string
	price            domain.Money
	onHand, reserved int
}

type reservationRow struct {
	orderID string
	lines   []domain.Line
	status  domain.ReservationStatus
}

func toProductRow(p *domain.Product) productRow {
	return productRow{sku: p.SKU(), name: p.Name(), price: p.Price(), onHand: p.OnHand(), reserved: p.Reserved()}
}

func (r productRow) toDomain() *domain.Product {
	return domain.RestoreProduct(r.sku, r.name, r.price, r.onHand, r.reserved)
}

func toReservationRow(r *domain.Reservation) reservationRow {
	return reservationRow{orderID: r.OrderID(), lines: r.Lines(), status: r.Status()}
}

func (r reservationRow) toDomain() *domain.Reservation {
	return domain.RestoreReservation(r.orderID, r.lines, r.status)
}

// Store implements app.Store.
type Store struct {
	mu           sync.Mutex
	products     map[string]productRow
	reservations map[string]reservationRow
}

var _ app.Store = (*Store)(nil)

// NewStore creates a store with the given products.
func NewStore(products ...*domain.Product) *Store {
	s := &Store{products: map[string]productRow{}, reservations: map[string]reservationRow{}}
	for _, p := range products {
		s.products[p.SKU()] = toProductRow(p)
	}
	return s
}

func (s *Store) Products(_ context.Context, skus []string) ([]*domain.Product, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var found []*domain.Product
	for _, sku := range skus {
		if row, ok := s.products[sku]; ok {
			found = append(found, row.toDomain())
		}
	}
	return found, nil
}

// InTx holds the store lock for the whole transaction. Do not call Products
// from inside fn: it would wait for the same lock.
func (s *Store) InTx(ctx context.Context, fn func(ctx context.Context, tx app.Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t := &tx{store: s, products: map[string]productRow{}, reservations: map[string]reservationRow{}}
	if err := fn(ctx, t); err != nil {
		return err // staged writes are dropped: rollback
	}
	for sku, row := range t.products {
		s.products[sku] = row
	}
	for orderID, row := range t.reservations {
		s.reservations[orderID] = row
	}
	return nil
}

// Product returns a copy of a stored product (test helper).
func (s *Store) Product(sku string) (*domain.Product, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.products[sku]
	if !ok {
		return nil, false
	}
	return row.toDomain(), true
}

// Reservation returns a copy of a stored reservation (test helper).
func (s *Store) Reservation(orderID string) (*domain.Reservation, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.reservations[orderID]
	if !ok {
		return nil, false
	}
	return row.toDomain(), true
}

// tx stages writes; reads see staged writes first, then the store.
type tx struct {
	store        *Store
	products     map[string]productRow
	reservations map[string]reservationRow
}

func (t *tx) Reservation(_ context.Context, orderID string) (*domain.Reservation, error) {
	if row, ok := t.reservations[orderID]; ok {
		return row.toDomain(), nil
	}
	if row, ok := t.store.reservations[orderID]; ok {
		return row.toDomain(), nil
	}
	return nil, nil
}

func (t *tx) LockProducts(_ context.Context, skus []string) (map[string]*domain.Product, error) {
	sorted := slices.Sorted(slices.Values(skus)) // same lock order as Postgres
	found := map[string]*domain.Product{}
	for _, sku := range sorted {
		if row, ok := t.products[sku]; ok {
			found[sku] = row.toDomain()
		} else if row, ok := t.store.products[sku]; ok {
			found[sku] = row.toDomain()
		}
	}
	return found, nil
}

func (t *tx) SaveProducts(_ context.Context, products ...*domain.Product) error {
	for _, p := range products {
		t.products[p.SKU()] = toProductRow(p)
	}
	return nil
}

func (t *tx) SaveReservation(_ context.Context, r *domain.Reservation) error {
	t.reservations[r.OrderID()] = toReservationRow(r)
	return nil
}
