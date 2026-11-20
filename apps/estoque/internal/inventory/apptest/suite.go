// Package apptest is the shared test suite for app.Service + app.Store.
//
// It uses only the app.Store interface, so it runs against every store:
// memory (unit tests) and Postgres (integration tests). If the fake ever
// behaves differently from the real thing, this suite catches it.
package apptest

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"apps/estoque/internal/inventory/app"
	"apps/estoque/internal/inventory/domain"
)

// Factory returns an EMPTY store holding exactly the given products.
type Factory func(t *testing.T, products ...*domain.Product) app.Store

var ctx = context.Background()

func product(t *testing.T, sku string, onHand int) *domain.Product {
	t.Helper()
	p, err := domain.NewProduct(sku, "Product "+sku, domain.Money{Cents: 1000, Currency: "BRL"}, onHand)
	if err != nil {
		t.Fatalf("NewProduct: %v", err)
	}
	return p
}

// setup: A has 5 units, B has 1.
func setup(t *testing.T, newStore Factory) (*app.Service, app.Store) {
	t.Helper()
	store := newStore(t, product(t, "A", 5), product(t, "B", 1))
	return app.NewService(store), store
}

func stockOf(t *testing.T, store app.Store, sku string) (onHand, reserved int) {
	t.Helper()
	products, err := store.Products(ctx, []string{sku})
	if err != nil || len(products) != 1 {
		t.Fatalf("product %s: %v (found %d)", sku, err, len(products))
	}
	return products[0].OnHand(), products[0].Reserved()
}

func reservationOf(t *testing.T, store app.Store, orderID string) *domain.Reservation {
	t.Helper()
	var r *domain.Reservation
	err := store.InTx(ctx, func(ctx context.Context, tx app.Tx) error {
		var err error
		r, err = tx.Reservation(ctx, orderID)
		return err
	})
	if err != nil {
		t.Fatalf("load reservation %s: %v", orderID, err)
	}
	return r
}

func line(sku string, qty int) []domain.Line { return []domain.Line{{SKU: sku, Quantity: qty}} }

// Run runs every test of the suite.
func Run(t *testing.T, newStore Factory) {
	t.Run("FindProducts returns only existing ones", func(t *testing.T) {
		svc, _ := setup(t, newStore)

		products, err := svc.FindProducts(ctx, []string{"A", "NOPE"})

		want := []app.ProductInfo{{SKU: "A", Name: "Product A", PriceCents: 1000, Currency: "BRL"}}
		if err != nil || !reflect.DeepEqual(products, want) {
			t.Fatalf("products = %+v, err = %v; want %+v", products, err, want)
		}
	})

	t.Run("Reserve holds stock", func(t *testing.T) {
		svc, store := setup(t, newStore)

		if err := svc.Reserve(ctx, "order-1", line("A", 2)); err != nil {
			t.Fatalf("Reserve: %v", err)
		}
		if _, reserved := stockOf(t, store, "A"); reserved != 2 {
			t.Fatalf("reserved = %d, want 2", reserved)
		}
		if r := reservationOf(t, store, "order-1"); r == nil || r.Status() != domain.StatusActive {
			t.Fatalf("reservation = %+v, want ACTIVE", r)
		}
	})

	t.Run("Reserve out of stock changes nothing", func(t *testing.T) {
		svc, store := setup(t, newStore)

		err := svc.Reserve(ctx, "order-1", []domain.Line{{SKU: "A", Quantity: 1}, {SKU: "B", Quantity: 2}})

		var outOfStock *domain.OutOfStockError
		if !errors.As(err, &outOfStock) || !reflect.DeepEqual(outOfStock.SKUs, []string{"B"}) {
			t.Fatalf("err = %v, want OutOfStockError{B}", err)
		}
		if _, reserved := stockOf(t, store, "A"); reserved != 0 {
			t.Fatalf("A reserved = %d, want 0", reserved)
		}
		if r := reservationOf(t, store, "order-1"); r != nil {
			t.Fatalf("reservation = %+v, want none", r)
		}
	})

	t.Run("Reserve twice with the same order is idempotent", func(t *testing.T) {
		svc, store := setup(t, newStore)

		for range 2 {
			if err := svc.Reserve(ctx, "order-1", line("A", 2)); err != nil {
				t.Fatalf("Reserve: %v", err)
			}
		}
		if _, reserved := stockOf(t, store, "A"); reserved != 2 {
			t.Fatalf("reserved = %d, want 2 (not 4)", reserved)
		}
	})

	t.Run("Reserve same order with different items is rejected", func(t *testing.T) {
		svc, _ := setup(t, newStore)
		_ = svc.Reserve(ctx, "order-1", line("A", 2))

		if err := svc.Reserve(ctx, "order-1", line("A", 3)); !errors.Is(err, domain.ErrIdempotencyKeyReuse) {
			t.Fatalf("err = %v, want ErrIdempotencyKeyReuse", err)
		}
	})

	t.Run("Release gives stock back, twice is a no-op", func(t *testing.T) {
		svc, store := setup(t, newStore)
		_ = svc.Reserve(ctx, "order-1", line("A", 2))

		for range 2 {
			if err := svc.Release(ctx, "order-1"); err != nil {
				t.Fatalf("Release: %v", err)
			}
		}
		if onHand, reserved := stockOf(t, store, "A"); onHand != 5 || reserved != 0 {
			t.Fatalf("onHand=%d reserved=%d, want 5 and 0", onHand, reserved)
		}
	})

	t.Run("Release before Reserve blocks a late Reserve", func(t *testing.T) {
		svc, store := setup(t, newStore)

		if err := svc.Release(ctx, "order-1"); err != nil {
			t.Fatalf("Release: %v", err)
		}
		if err := svc.Reserve(ctx, "order-1", line("A", 2)); !errors.Is(err, domain.ErrReservationReleased) {
			t.Fatalf("err = %v, want ErrReservationReleased", err)
		}
		if _, reserved := stockOf(t, store, "A"); reserved != 0 {
			t.Fatalf("reserved = %d, want 0", reserved)
		}
	})

	t.Run("Commit takes stock out, release after commit fails", func(t *testing.T) {
		svc, store := setup(t, newStore)
		_ = svc.Reserve(ctx, "order-1", line("A", 2))

		for range 2 {
			if err := svc.Commit(ctx, "order-1"); err != nil {
				t.Fatalf("Commit: %v", err)
			}
		}
		if onHand, reserved := stockOf(t, store, "A"); onHand != 3 || reserved != 0 {
			t.Fatalf("onHand=%d reserved=%d, want 3 and 0", onHand, reserved)
		}
		if err := svc.Release(ctx, "order-1"); !errors.Is(err, domain.ErrInvalidTransition) {
			t.Fatalf("err = %v, want ErrInvalidTransition", err)
		}
	})

	t.Run("Commit unknown order", func(t *testing.T) {
		svc, _ := setup(t, newStore)

		if err := svc.Commit(ctx, "nope"); !errors.Is(err, app.ErrReservationNotFound) {
			t.Fatalf("err = %v, want ErrReservationNotFound", err)
		}
	})

	t.Run("store: a failed transaction changes nothing", func(t *testing.T) {
		_, store := setup(t, newStore)
		boom := errors.New("boom")

		err := store.InTx(ctx, func(ctx context.Context, tx app.Tx) error {
			products, _ := tx.LockProducts(ctx, []string{"A"})
			_ = products["A"].Reserve(2)
			_ = tx.SaveProducts(ctx, products["A"])
			_ = tx.SaveReservation(ctx, domain.NewReleasedTombstone("order-1"))
			return boom
		})

		if !errors.Is(err, boom) {
			t.Fatalf("err = %v, want boom", err)
		}
		if _, reserved := stockOf(t, store, "A"); reserved != 0 {
			t.Fatalf("reserved = %d, want 0 (rolled back)", reserved)
		}
		if r := reservationOf(t, store, "order-1"); r != nil {
			t.Fatalf("reservation = %+v, want none", r)
		}
	})

	t.Run("store: changes without Save are not kept", func(t *testing.T) {
		_, store := setup(t, newStore)

		_ = store.InTx(ctx, func(ctx context.Context, tx app.Tx) error {
			products, _ := tx.LockProducts(ctx, []string{"A"})
			return products["A"].Reserve(2) // changed, never saved
		})
		products, _ := store.Products(ctx, []string{"A"})
		_ = products[0].Reserve(1) // changing a loaded copy

		if _, reserved := stockOf(t, store, "A"); reserved != 0 {
			t.Fatalf("reserved = %d, want 0", reserved)
		}
	})

	// ---- concurrency: the reason the store has locks ----

	t.Run("race: 50 orders for the LAST unit, exactly one wins", func(t *testing.T) {
		svc, store := setup(t, newStore) // B has 1 unit
		errs := concurrently(50, func(i int) error {
			return svc.Reserve(ctx, fmt.Sprintf("order-%d", i), line("B", 1))
		})

		winners := 0
		for _, err := range errs {
			var outOfStock *domain.OutOfStockError
			switch {
			case err == nil:
				winners++
			case !errors.As(err, &outOfStock):
				t.Fatalf("unexpected error: %v", err)
			}
		}
		if winners != 1 {
			t.Fatalf("winners = %d, want exactly 1", winners)
		}
		if onHand, reserved := stockOf(t, store, "B"); onHand != 1 || reserved != 1 {
			t.Fatalf("onHand=%d reserved=%d, want 1 and 1", onHand, reserved)
		}
	})

	t.Run("race: the same NEW order 20 times reserves only once", func(t *testing.T) {
		svc, store := setup(t, newStore)
		errs := concurrently(20, func(int) error { return svc.Reserve(ctx, "order-1", line("A", 2)) })

		for _, err := range errs {
			if err != nil {
				t.Fatalf("every retry must succeed, got %v", err)
			}
		}
		if _, reserved := stockOf(t, store, "A"); reserved != 2 {
			t.Fatalf("reserved = %d, want 2 (not %d)", reserved, 2*len(errs))
		}
	})

	t.Run("race: Release and Reserve of the same new order never leave stock held", func(t *testing.T) {
		svc, store := setup(t, newStore)
		const orders = 20

		concurrently(orders*2, func(i int) error {
			orderID := fmt.Sprintf("order-%d", i/2)
			if i%2 == 0 {
				return svc.Reserve(ctx, orderID, line("A", 1))
			}
			return svc.Release(ctx, orderID)
		})

		// Whatever the order of arrival, every order ends RELEASED and no stock is held.
		for i := range orders {
			if r := reservationOf(t, store, fmt.Sprintf("order-%d", i)); r == nil || r.Status() != domain.StatusReleased {
				t.Fatalf("order-%d: reservation = %+v, want RELEASED", i, r)
			}
		}
		if _, reserved := stockOf(t, store, "A"); reserved != 0 {
			t.Fatalf("reserved = %d, want 0", reserved)
		}
	})
}

// concurrently runs fn(0..n-1) at the same time and returns every error.
func concurrently(n int, fn func(i int) error) []error {
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() { errs[i] = fn(i) })
	}
	wg.Wait()
	return errs
}
