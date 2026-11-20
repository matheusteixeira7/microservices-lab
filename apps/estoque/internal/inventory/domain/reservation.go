package domain

import (
	"fmt"
	"slices"
	"strings"
)

// ReservationStatus is the state of a reservation:
//
//	ACTIVE ──Commit()──▶ COMMITTED   (order confirmed: stock leaves the warehouse)
//	   │
//	   └────Release()──▶ RELEASED    (order cancelled: stock goes back)
//
// COMMITTED and RELEASED are final. Repeating a transition is a no-op.
type ReservationStatus string

const (
	StatusActive    ReservationStatus = "ACTIVE"
	StatusCommitted ReservationStatus = "COMMITTED"
	StatusReleased  ReservationStatus = "RELEASED"
)

// Line is one SKU and quantity of a reservation.
type Line struct {
	SKU      string
	Quantity int
}

// Reservation is an aggregate root: the stock held for one order.
// The order id is its identity and also the idempotency key.
type Reservation struct {
	orderID string
	lines   []Line
	status  ReservationStatus
}

// NewReservation validates the request. The stock is only held by Place.
func NewReservation(orderID string, lines []Line) (*Reservation, error) {
	if strings.TrimSpace(orderID) == "" {
		return nil, fmt.Errorf("%w: order id is required", ErrInvalidReservation)
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("%w: at least one line is required", ErrInvalidReservation)
	}
	seen := map[string]bool{}
	for _, line := range lines {
		switch {
		case strings.TrimSpace(line.SKU) == "":
			return nil, fmt.Errorf("%w: sku is required", ErrInvalidReservation)
		case line.Quantity <= 0:
			return nil, fmt.Errorf("%w: quantity of %s must be positive", ErrInvalidReservation, line.SKU)
		case seen[line.SKU]:
			return nil, fmt.Errorf("%w: sku %s appears more than once", ErrInvalidReservation, line.SKU)
		}
		seen[line.SKU] = true
	}
	return &Reservation{orderID: orderID, lines: slices.Clone(lines), status: StatusActive}, nil
}

// NewReleasedTombstone records "this order was cancelled" when the release
// arrives BEFORE the reservation (messages out of order). A late Reserve for
// the same order then fails instead of holding stock for a cancelled order.
func NewReleasedTombstone(orderID string) *Reservation {
	return &Reservation{orderID: orderID, status: StatusReleased}
}

// RestoreReservation rebuilds a reservation loaded from storage (no validation).
func RestoreReservation(orderID string, lines []Line, status ReservationStatus) *Reservation {
	return &Reservation{orderID: orderID, lines: slices.Clone(lines), status: status}
}

func (r *Reservation) OrderID() string           { return r.orderID }
func (r *Reservation) Status() ReservationStatus { return r.status }

// Lines returns a copy: changing it does not change the reservation.
func (r *Reservation) Lines() []Line { return slices.Clone(r.lines) }

// SKUs returns the SKUs of the reservation, sorted.
func (r *Reservation) SKUs() []string {
	skus := make([]string, 0, len(r.lines))
	for _, line := range r.lines {
		skus = append(skus, line.SKU)
	}
	slices.Sort(skus)
	return skus
}

// Matches reports whether lines are the same as the reservation's, in any order.
func (r *Reservation) Matches(lines []Line) bool {
	if len(lines) != len(r.lines) {
		return false
	}
	want := map[string]int{}
	for _, line := range r.lines {
		want[line.SKU] = line.Quantity
	}
	for _, line := range lines {
		if qty, ok := want[line.SKU]; !ok || qty != line.Quantity {
			return false
		}
	}
	return true
}

// Place holds stock for every line, or for none (all-or-nothing).
// products holds the products that exist; a missing SKU counts as unavailable.
func Place(r *Reservation, products map[string]*Product) error {
	var unavailable []string
	for _, line := range r.lines {
		p, ok := products[line.SKU]
		if !ok || p.Available() < line.Quantity {
			unavailable = append(unavailable, line.SKU)
		}
	}
	if len(unavailable) > 0 {
		slices.Sort(unavailable)
		return &OutOfStockError{SKUs: unavailable}
	}

	for _, line := range r.lines {
		if err := products[line.SKU].Reserve(line.Quantity); err != nil {
			return err // unreachable: availability was checked above
		}
	}
	return nil
}

// Release gives the stock back. Releasing twice is a no-op.
func Release(r *Reservation, products map[string]*Product) error {
	switch r.status {
	case StatusReleased:
		return nil
	case StatusCommitted:
		return fmt.Errorf("%w: cannot release a committed reservation", ErrInvalidTransition)
	}
	if err := apply(r, products, (*Product).Release); err != nil {
		return err
	}
	r.status = StatusReleased
	return nil
}

// Commit takes the stock out of the warehouse. Committing twice is a no-op.
func Commit(r *Reservation, products map[string]*Product) error {
	switch r.status {
	case StatusCommitted:
		return nil
	case StatusReleased:
		return fmt.Errorf("%w: cannot commit a released reservation", ErrInvalidTransition)
	}
	if err := apply(r, products, (*Product).Commit); err != nil {
		return err
	}
	r.status = StatusCommitted
	return nil
}

// apply runs op for every line. It stops at the first error; callers run it
// inside a transaction, so a partial change is never saved.
func apply(r *Reservation, products map[string]*Product, op func(*Product, int) error) error {
	for _, line := range r.lines {
		p, ok := products[line.SKU]
		if !ok {
			return fmt.Errorf("product %s of reservation %s not found", line.SKU, r.orderID)
		}
		if err := op(p, line.Quantity); err != nil {
			return err
		}
	}
	return nil
}
