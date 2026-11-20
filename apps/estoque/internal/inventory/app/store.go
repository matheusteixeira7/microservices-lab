// Package app holds the inventory use cases. It orchestrates the domain and
// talks to storage only through the Store port below.
package app

import (
	"context"

	"apps/estoque/internal/inventory/domain"
)

// Store is the storage port. Postgres implements it with SELECT ... FOR UPDATE;
// the memory package implements it with a mutex.
type Store interface {
	// Products returns the products that exist (missing SKUs are absent).
	// Read-only: no lock.
	Products(ctx context.Context, skus []string) ([]*domain.Product, error)

	// InTx runs fn in one transaction. If fn returns an error, nothing that fn
	// saved is kept.
	InTx(ctx context.Context, fn func(ctx context.Context, tx Tx) error) error
}

// Tx is what a use case can do inside a transaction.
//
// Lock order, to avoid deadlocks in Postgres: first the reservation, then the
// products, always in sorted SKU order. Every use case follows it.
type Tx interface {
	// Reservation loads and locks the reservation of an order.
	// Returns (nil, nil) when there is none.
	Reservation(ctx context.Context, orderID string) (*domain.Reservation, error)

	// LockProducts loads and locks the products, in sorted SKU order.
	// Missing SKUs are absent from the map.
	LockProducts(ctx context.Context, skus []string) (map[string]*domain.Product, error)

	SaveProducts(ctx context.Context, products ...*domain.Product) error
	SaveReservation(ctx context.Context, reservation *domain.Reservation) error
}
