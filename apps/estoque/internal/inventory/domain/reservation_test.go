package domain

import (
	"errors"
	"reflect"
	"testing"
)

func stock(t *testing.T, onHand map[string]int) map[string]*Product {
	t.Helper()
	products := map[string]*Product{}
	for sku, qty := range onHand {
		p, err := NewProduct(sku, "Product "+sku, Money{Cents: 1000, Currency: "BRL"}, qty)
		if err != nil {
			t.Fatalf("NewProduct: %v", err)
		}
		products[sku] = p
	}
	return products
}

func TestNewReservationValidates(t *testing.T) {
	tests := []struct {
		name    string
		orderID string
		lines   []Line
	}{
		{"empty order id", " ", []Line{{SKU: "A", Quantity: 1}}},
		{"no lines", "order-1", nil},
		{"empty sku", "order-1", []Line{{SKU: "", Quantity: 1}}},
		{"zero quantity", "order-1", []Line{{SKU: "A", Quantity: 0}}},
		{"duplicate sku", "order-1", []Line{{SKU: "A", Quantity: 1}, {SKU: "A", Quantity: 2}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewReservation(tt.orderID, tt.lines); !errors.Is(err, ErrInvalidReservation) {
				t.Fatalf("err = %v, want ErrInvalidReservation", err)
			}
		})
	}
}

func TestReservationLinesReturnsACopy(t *testing.T) {
	r, _ := NewReservation("order-1", []Line{{SKU: "A", Quantity: 1}})

	r.Lines()[0].Quantity = 99

	if r.Lines()[0].Quantity != 1 {
		t.Fatal("changing the returned slice changed the reservation")
	}
}

func TestReservationMatchesIgnoresLineOrder(t *testing.T) {
	r, _ := NewReservation("order-1", []Line{{SKU: "A", Quantity: 1}, {SKU: "B", Quantity: 2}})

	if !r.Matches([]Line{{SKU: "B", Quantity: 2}, {SKU: "A", Quantity: 1}}) {
		t.Error("same lines in another order should match")
	}
	if r.Matches([]Line{{SKU: "A", Quantity: 1}, {SKU: "B", Quantity: 3}}) {
		t.Error("different quantity should not match")
	}
	if r.Matches([]Line{{SKU: "A", Quantity: 1}}) {
		t.Error("missing line should not match")
	}
}

func TestPlaceReservesEveryLine(t *testing.T) {
	products := stock(t, map[string]int{"A": 5, "B": 5})
	r, _ := NewReservation("order-1", []Line{{SKU: "A", Quantity: 2}, {SKU: "B", Quantity: 5}})

	if err := Place(r, products); err != nil {
		t.Fatalf("Place: %v", err)
	}
	if products["A"].Reserved() != 2 || products["B"].Reserved() != 5 {
		t.Fatalf("reserved A=%d B=%d", products["A"].Reserved(), products["B"].Reserved())
	}
	if r.Status() != StatusActive {
		t.Fatalf("status = %s, want ACTIVE", r.Status())
	}
}

func TestPlaceIsAllOrNothingAndReportsEveryUnavailableSKU(t *testing.T) {
	// A is fine, B is short, C does not exist.
	products := stock(t, map[string]int{"A": 5, "B": 1})
	r, _ := NewReservation("order-1", []Line{
		{SKU: "C", Quantity: 1},
		{SKU: "A", Quantity: 2},
		{SKU: "B", Quantity: 2},
	})

	err := Place(r, products)

	var outOfStock *OutOfStockError
	if !errors.As(err, &outOfStock) {
		t.Fatalf("err = %v, want *OutOfStockError", err)
	}
	if want := []string{"B", "C"}; !reflect.DeepEqual(outOfStock.SKUs, want) {
		t.Fatalf("SKUs = %v, want %v (sorted, unknown included)", outOfStock.SKUs, want)
	}
	if products["A"].Reserved() != 0 || products["B"].Reserved() != 0 {
		t.Fatal("nothing should be reserved when one line fails")
	}
}

// placed returns an ACTIVE reservation of 2 x A, with stock 5.
func placed(t *testing.T) (*Reservation, map[string]*Product) {
	t.Helper()
	products := stock(t, map[string]int{"A": 5})
	r, _ := NewReservation("order-1", []Line{{SKU: "A", Quantity: 2}})
	if err := Place(r, products); err != nil {
		t.Fatalf("Place: %v", err)
	}
	return r, products
}

func TestReleaseGivesStockBackAndIsIdempotent(t *testing.T) {
	r, products := placed(t)

	for range 2 { // second call is a no-op
		if err := Release(r, products); err != nil {
			t.Fatalf("Release: %v", err)
		}
	}
	if r.Status() != StatusReleased || products["A"].Reserved() != 0 || products["A"].OnHand() != 5 {
		t.Fatalf("status=%s reserved=%d onHand=%d", r.Status(), products["A"].Reserved(), products["A"].OnHand())
	}
}

func TestCommitTakesStockOutAndIsIdempotent(t *testing.T) {
	r, products := placed(t)

	for range 2 { // second call is a no-op
		if err := Commit(r, products); err != nil {
			t.Fatalf("Commit: %v", err)
		}
	}
	if r.Status() != StatusCommitted || products["A"].Reserved() != 0 || products["A"].OnHand() != 3 {
		t.Fatalf("status=%s reserved=%d onHand=%d", r.Status(), products["A"].Reserved(), products["A"].OnHand())
	}
}

func TestIllegalTransitions(t *testing.T) {
	t.Run("release after commit", func(t *testing.T) {
		r, products := placed(t)
		_ = Commit(r, products)
		if err := Release(r, products); !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("err = %v, want ErrInvalidTransition", err)
		}
	})
	t.Run("commit after release", func(t *testing.T) {
		r, products := placed(t)
		_ = Release(r, products)
		if err := Commit(r, products); !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("err = %v, want ErrInvalidTransition", err)
		}
	})
}

func TestReleasedTombstone(t *testing.T) {
	r := NewReleasedTombstone("order-1")

	if r.Status() != StatusReleased || len(r.Lines()) != 0 || r.OrderID() != "order-1" {
		t.Fatalf("tombstone = %+v", r)
	}
}
