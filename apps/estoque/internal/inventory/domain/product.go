package domain

import (
	"fmt"
	"strings"
)

// Money is an amount in integer cents plus a currency (no floats).
type Money struct {
	Cents    int64
	Currency string
}

// Product is an aggregate root: one SKU with its catalog data and stock.
//
// Invariant: 0 <= reserved <= onHand. Fields are unexported, so the only way
// to change stock is through the methods below.
type Product struct {
	sku      string
	name     string
	price    Money
	onHand   int // physically in the warehouse
	reserved int // promised to orders, not shipped yet
}

// NewProduct validates and creates a product.
func NewProduct(sku, name string, price Money, onHand int) (*Product, error) {
	switch {
	case strings.TrimSpace(sku) == "":
		return nil, fmt.Errorf("%w: sku is required", ErrInvalidProduct)
	case strings.TrimSpace(name) == "":
		return nil, fmt.Errorf("%w: name is required", ErrInvalidProduct)
	case price.Cents < 0 || strings.TrimSpace(price.Currency) == "":
		return nil, fmt.Errorf("%w: price must be >= 0 with a currency", ErrInvalidProduct)
	case onHand < 0:
		return nil, fmt.Errorf("%w: stock cannot be negative", ErrInvalidProduct)
	}
	return &Product{sku: sku, name: name, price: price, onHand: onHand}, nil
}

// RestoreProduct rebuilds a product loaded from storage (no validation).
func RestoreProduct(sku, name string, price Money, onHand, reserved int) *Product {
	return &Product{sku: sku, name: name, price: price, onHand: onHand, reserved: reserved}
}

func (p *Product) SKU() string    { return p.sku }
func (p *Product) Name() string   { return p.name }
func (p *Product) Price() Money   { return p.price }
func (p *Product) OnHand() int    { return p.onHand }
func (p *Product) Reserved() int  { return p.reserved }
func (p *Product) Available() int { return p.onHand - p.reserved }

// Reserve promises qty units to an order.
func (p *Product) Reserve(qty int) error {
	if qty <= 0 {
		return ErrInvalidQuantity
	}
	if qty > p.Available() {
		return fmt.Errorf("%w: %s has %d available, asked %d", ErrInsufficientStock, p.sku, p.Available(), qty)
	}
	p.reserved += qty
	return nil
}

// Release gives reserved units back (order cancelled).
func (p *Product) Release(qty int) error {
	if qty <= 0 {
		return ErrInvalidQuantity
	}
	if qty > p.reserved {
		return fmt.Errorf("%w: %s has %d reserved, asked %d", ErrNotReserved, p.sku, p.reserved, qty)
	}
	p.reserved -= qty
	return nil
}

// Commit takes reserved units out of the warehouse (order confirmed).
func (p *Product) Commit(qty int) error {
	if qty <= 0 {
		return ErrInvalidQuantity
	}
	if qty > p.reserved {
		return fmt.Errorf("%w: %s has %d reserved, asked %d", ErrNotReserved, p.sku, p.reserved, qty)
	}
	p.reserved -= qty
	p.onHand -= qty
	return nil
}
