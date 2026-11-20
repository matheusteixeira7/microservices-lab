package memory_test

import (
	"context"
	"errors"
	"testing"

	"apps/estoque/internal/inventory/app"
	"apps/estoque/internal/inventory/domain"
	"apps/estoque/internal/inventory/memory"
)

func newStore(t *testing.T) *memory.Store {
	t.Helper()
	p, err := domain.NewProduct("A", "Product A", domain.Money{Cents: 1000, Currency: "BRL"}, 5)
	if err != nil {
		t.Fatalf("NewProduct: %v", err)
	}
	return memory.NewStore(p)
}

func TestFailedTransactionChangesNothing(t *testing.T) {
	store := newStore(t)
	boom := errors.New("boom")

	err := store.InTx(context.Background(), func(ctx context.Context, tx app.Tx) error {
		products, _ := tx.LockProducts(ctx, []string{"A"})
		_ = products["A"].Reserve(2)
		_ = tx.SaveProducts(ctx, products["A"])
		_ = tx.SaveReservation(ctx, domain.NewReleasedTombstone("order-1"))
		return boom
	})

	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	if p, _ := store.Product("A"); p.Reserved() != 0 {
		t.Fatalf("reserved = %d, want 0 (rolled back)", p.Reserved())
	}
	if _, ok := store.Reservation("order-1"); ok {
		t.Fatal("reservation should not be saved")
	}
}

func TestChangesWithoutSaveDoNotLeak(t *testing.T) {
	store := newStore(t)

	_ = store.InTx(context.Background(), func(ctx context.Context, tx app.Tx) error {
		products, _ := tx.LockProducts(ctx, []string{"A"})
		_ = products["A"].Reserve(2) // changed, but never saved
		return nil
	})
	products, _ := store.Products(context.Background(), []string{"A"})
	_ = products[0].Reserve(1) // changing a loaded copy

	if p, _ := store.Product("A"); p.Reserved() != 0 {
		t.Fatalf("reserved = %d, want 0 (the store must hold copies)", p.Reserved())
	}
}
