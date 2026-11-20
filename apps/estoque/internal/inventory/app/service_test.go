package app_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"apps/estoque/internal/inventory/app"
	"apps/estoque/internal/inventory/domain"
	"apps/estoque/internal/inventory/memory"
)

func product(t *testing.T, sku string, onHand int) *domain.Product {
	t.Helper()
	p, err := domain.NewProduct(sku, "Product "+sku, domain.Money{Cents: 1000, Currency: "BRL"}, onHand)
	if err != nil {
		t.Fatalf("NewProduct: %v", err)
	}
	return p
}

// setup: A has 5 units, B has 1.
func setup(t *testing.T) (*app.Service, *memory.Store) {
	t.Helper()
	store := memory.NewStore(product(t, "A", 5), product(t, "B", 1))
	return app.NewService(store), store
}

func stockOf(t *testing.T, store *memory.Store, sku string) (onHand, reserved int) {
	t.Helper()
	p, ok := store.Product(sku)
	if !ok {
		t.Fatalf("product %s not found", sku)
	}
	return p.OnHand(), p.Reserved()
}

var ctx = context.Background()

func TestFindProductsReturnsOnlyExistingOnes(t *testing.T) {
	svc, _ := setup(t)

	products, err := svc.FindProducts(ctx, []string{"A", "NOPE"})

	if err != nil {
		t.Fatalf("FindProducts: %v", err)
	}
	want := []app.ProductInfo{{SKU: "A", Name: "Product A", PriceCents: 1000, Currency: "BRL"}}
	if !reflect.DeepEqual(products, want) {
		t.Fatalf("products = %+v, want %+v", products, want)
	}
}

func TestReserveHoldsStock(t *testing.T) {
	svc, store := setup(t)

	if err := svc.Reserve(ctx, "order-1", []domain.Line{{SKU: "A", Quantity: 2}}); err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if _, reserved := stockOf(t, store, "A"); reserved != 2 {
		t.Fatalf("reserved = %d, want 2", reserved)
	}
}

func TestReserveOutOfStockChangesNothing(t *testing.T) {
	svc, store := setup(t)

	err := svc.Reserve(ctx, "order-1", []domain.Line{{SKU: "A", Quantity: 1}, {SKU: "B", Quantity: 2}})

	var outOfStock *domain.OutOfStockError
	if !errors.As(err, &outOfStock) || !reflect.DeepEqual(outOfStock.SKUs, []string{"B"}) {
		t.Fatalf("err = %v, want OutOfStockError{B}", err)
	}
	if _, reserved := stockOf(t, store, "A"); reserved != 0 {
		t.Fatalf("A reserved = %d, want 0", reserved)
	}
	if _, ok := store.Reservation("order-1"); ok {
		t.Fatal("no reservation should be saved")
	}
}

func TestReserveTwiceWithTheSameOrderIsIdempotent(t *testing.T) {
	svc, store := setup(t)
	lines := []domain.Line{{SKU: "A", Quantity: 2}}

	for range 2 {
		if err := svc.Reserve(ctx, "order-1", lines); err != nil {
			t.Fatalf("Reserve: %v", err)
		}
	}
	if _, reserved := stockOf(t, store, "A"); reserved != 2 {
		t.Fatalf("reserved = %d, want 2 (not 4)", reserved)
	}
}

func TestReserveSameOrderWithDifferentItemsIsRejected(t *testing.T) {
	svc, _ := setup(t)
	_ = svc.Reserve(ctx, "order-1", []domain.Line{{SKU: "A", Quantity: 2}})

	err := svc.Reserve(ctx, "order-1", []domain.Line{{SKU: "A", Quantity: 3}})

	if !errors.Is(err, domain.ErrIdempotencyKeyReuse) {
		t.Fatalf("err = %v, want ErrIdempotencyKeyReuse", err)
	}
}

func TestReleaseGivesStockBack(t *testing.T) {
	svc, store := setup(t)
	_ = svc.Reserve(ctx, "order-1", []domain.Line{{SKU: "A", Quantity: 2}})

	for range 2 { // the second release is a no-op
		if err := svc.Release(ctx, "order-1"); err != nil {
			t.Fatalf("Release: %v", err)
		}
	}
	if onHand, reserved := stockOf(t, store, "A"); onHand != 5 || reserved != 0 {
		t.Fatalf("onHand=%d reserved=%d, want 5 and 0", onHand, reserved)
	}
}

func TestReleaseBeforeReserveBlocksALateReserve(t *testing.T) {
	svc, store := setup(t)

	// The cancellation arrives before the reservation (messages out of order).
	if err := svc.Release(ctx, "order-1"); err != nil {
		t.Fatalf("Release: %v", err)
	}
	err := svc.Reserve(ctx, "order-1", []domain.Line{{SKU: "A", Quantity: 2}})

	if !errors.Is(err, domain.ErrReservationReleased) {
		t.Fatalf("err = %v, want ErrReservationReleased", err)
	}
	if _, reserved := stockOf(t, store, "A"); reserved != 0 {
		t.Fatalf("reserved = %d, want 0", reserved)
	}
}

func TestCommitTakesStockOut(t *testing.T) {
	svc, store := setup(t)
	_ = svc.Reserve(ctx, "order-1", []domain.Line{{SKU: "A", Quantity: 2}})

	for range 2 { // the second commit is a no-op
		if err := svc.Commit(ctx, "order-1"); err != nil {
			t.Fatalf("Commit: %v", err)
		}
	}
	if onHand, reserved := stockOf(t, store, "A"); onHand != 3 || reserved != 0 {
		t.Fatalf("onHand=%d reserved=%d, want 3 and 0", onHand, reserved)
	}
	if err := svc.Release(ctx, "order-1"); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("release after commit: err = %v, want ErrInvalidTransition", err)
	}
}

func TestCommitUnknownOrder(t *testing.T) {
	svc, _ := setup(t)

	if err := svc.Commit(ctx, "nope"); !errors.Is(err, app.ErrReservationNotFound) {
		t.Fatalf("err = %v, want ErrReservationNotFound", err)
	}
}

// The centerpiece: many orders race for the LAST unit. Exactly one may win.
// Run with -race.
func TestConcurrentReservesNeverOversell(t *testing.T) {
	svc, store := setup(t) // B has exactly 1 unit
	const buyers = 50

	var wg sync.WaitGroup
	errs := make([]error, buyers)
	for i := range buyers {
		wg.Go(func() {
			errs[i] = svc.Reserve(ctx, fmt.Sprintf("order-%d", i), []domain.Line{{SKU: "B", Quantity: 1}})
		})
	}
	wg.Wait()

	winners := 0
	for _, err := range errs {
		var outOfStock *domain.OutOfStockError
		switch {
		case err == nil:
			winners++
		case errors.As(err, &outOfStock):
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if winners != 1 {
		t.Fatalf("winners = %d, want exactly 1", winners)
	}
	if onHand, reserved := stockOf(t, store, "B"); onHand != 1 || reserved != 1 {
		t.Fatalf("onHand=%d reserved=%d, want 1 and 1", onHand, reserved)
	}
}

// Infrastructure failures must never look like a business "no".
func TestStoreFailuresAreNotOutOfStock(t *testing.T) {
	boom := errors.New("connection reset")
	svc := app.NewService(failingStore{err: boom})

	err := svc.Reserve(ctx, "order-1", []domain.Line{{SKU: "A", Quantity: 1}})

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
