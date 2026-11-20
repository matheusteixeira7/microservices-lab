package app_test

import (
	"context"
	"errors"
	"testing"

	"apps/estoque/internal/inventory/app"
	"apps/estoque/internal/inventory/apptest"
	"apps/estoque/internal/inventory/domain"
	"apps/estoque/internal/inventory/memory"
)

// The shared suite against the in-memory store. The same suite runs against
// Postgres in internal/inventory/postgres (integration tests).
func TestServiceWithMemoryStore(t *testing.T) {
	apptest.Run(t, func(_ *testing.T, products ...*domain.Product) app.Store {
		return memory.NewStore(products...)
	})
}

// Infrastructure failures must never look like a business "no".
func TestStoreFailuresAreNotOutOfStock(t *testing.T) {
	boom := errors.New("connection reset")
	svc := app.NewService(failingStore{err: boom})

	err := svc.Reserve(context.Background(), "order-1", []domain.Line{{SKU: "A", Quantity: 1}})

	var outOfStock *domain.OutOfStockError
	if !errors.Is(err, boom) || errors.As(err, &outOfStock) {
		t.Fatalf("err = %v, want the store error (wrapped), not OutOfStockError", err)
	}
}

type failingStore struct{ err error }

func (s failingStore) Products(context.Context, []string) ([]*domain.Product, error) {
	return nil, s.err
}

func (s failingStore) InTx(context.Context, func(context.Context, app.Tx) error) error {
	return s.err
}
