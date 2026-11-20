// Package domain holds the inventory rules: products, stock and reservations.
// It imports nothing outside the standard library.
package domain

import (
	"errors"
	"strings"
)

// Business-rule errors. Check them with errors.Is.
var (
	ErrInvalidProduct      = errors.New("invalid product")
	ErrInvalidQuantity     = errors.New("quantity must be a positive integer")
	ErrInsufficientStock   = errors.New("insufficient stock")
	ErrNotReserved         = errors.New("quantity is larger than what is reserved")
	ErrInvalidReservation  = errors.New("invalid reservation")
	ErrInvalidTransition   = errors.New("invalid reservation transition")
	ErrReservationReleased = errors.New("reservation was already released for this order")
	ErrIdempotencyKeyReuse = errors.New("order id already used for a reservation with different items")
)

// OutOfStockError is a business "no", not a failure: some SKUs cannot be
// reserved (not enough stock, or unknown). Check it with errors.As.
type OutOfStockError struct {
	// SKUs that could not be reserved, sorted.
	SKUs []string
}

func (e *OutOfStockError) Error() string {
	return "out of stock: " + strings.Join(e.SKUs, ", ")
}
