package domain

import (
	"errors"
	"testing"
)

func newKeyboard(t *testing.T, onHand int) *Product {
	t.Helper()
	p, err := NewProduct("KB-1", "Keyboard", Money{Cents: 45000, Currency: "BRL"}, onHand)
	if err != nil {
		t.Fatalf("NewProduct: %v", err)
	}
	return p
}

func TestNewProductValidates(t *testing.T) {
	brl := Money{Cents: 100, Currency: "BRL"}
	tests := []struct {
		name   string
		sku    string
		pname  string
		price  Money
		onHand int
	}{
		{"empty sku", " ", "Keyboard", brl, 1},
		{"empty name", "KB-1", "", brl, 1},
		{"negative price", "KB-1", "Keyboard", Money{Cents: -1, Currency: "BRL"}, 1},
		{"empty currency", "KB-1", "Keyboard", Money{Cents: 100}, 1},
		{"negative stock", "KB-1", "Keyboard", brl, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewProduct(tt.sku, tt.pname, tt.price, tt.onHand)
			if !errors.Is(err, ErrInvalidProduct) {
				t.Fatalf("err = %v, want ErrInvalidProduct", err)
			}
		})
	}
}

func TestProductReserve(t *testing.T) {
	p := newKeyboard(t, 5)

	if err := p.Reserve(5); err != nil { // exactly what is available
		t.Fatalf("Reserve(5): %v", err)
	}
	if p.Reserved() != 5 || p.Available() != 0 || p.OnHand() != 5 {
		t.Fatalf("reserved=%d available=%d onHand=%d", p.Reserved(), p.Available(), p.OnHand())
	}
}

func TestProductReserveMoreThanAvailableChangesNothing(t *testing.T) {
	p := newKeyboard(t, 5)
	_ = p.Reserve(3)

	err := p.Reserve(3) // only 2 left

	if !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("err = %v, want ErrInsufficientStock", err)
	}
	if p.Reserved() != 3 {
		t.Fatalf("reserved = %d, want 3 (unchanged)", p.Reserved())
	}
}

func TestProductRejectsNonPositiveQuantities(t *testing.T) {
	p := newKeyboard(t, 5)
	_ = p.Reserve(2)

	for _, qty := range []int{0, -1} {
		if err := p.Reserve(qty); !errors.Is(err, ErrInvalidQuantity) {
			t.Errorf("Reserve(%d) err = %v, want ErrInvalidQuantity", qty, err)
		}
		if err := p.Release(qty); !errors.Is(err, ErrInvalidQuantity) {
			t.Errorf("Release(%d) err = %v, want ErrInvalidQuantity", qty, err)
		}
		if err := p.Commit(qty); !errors.Is(err, ErrInvalidQuantity) {
			t.Errorf("Commit(%d) err = %v, want ErrInvalidQuantity", qty, err)
		}
	}
}

func TestProductRelease(t *testing.T) {
	p := newKeyboard(t, 5)
	_ = p.Reserve(3)

	if err := p.Release(2); err != nil {
		t.Fatalf("Release(2): %v", err)
	}
	if p.Reserved() != 1 || p.OnHand() != 5 {
		t.Fatalf("reserved=%d onHand=%d, want 1 and 5", p.Reserved(), p.OnHand())
	}
	if err := p.Release(2); !errors.Is(err, ErrNotReserved) { // only 1 reserved
		t.Fatalf("err = %v, want ErrNotReserved", err)
	}
}

func TestProductCommitTakesStockOut(t *testing.T) {
	p := newKeyboard(t, 5)
	_ = p.Reserve(3)

	if err := p.Commit(3); err != nil {
		t.Fatalf("Commit(3): %v", err)
	}
	if p.OnHand() != 2 || p.Reserved() != 0 || p.Available() != 2 {
		t.Fatalf("onHand=%d reserved=%d available=%d", p.OnHand(), p.Reserved(), p.Available())
	}
	if err := p.Commit(1); !errors.Is(err, ErrNotReserved) {
		t.Fatalf("err = %v, want ErrNotReserved", err)
	}
}
