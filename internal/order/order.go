package order

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusConfirmed Status = "confirmed"
	StatusCancelled Status = "cancelled"
)

// Order is the aggregate root. It carries domain invariants and has no
// knowledge of any persistence or transport technology.
type Order struct {
	ID         uuid.UUID
	CustomerID uuid.UUID
	Status     Status
	Items      []Item
	CreatedAt  time.Time
}

type Item struct {
	ProductID uuid.UUID
	Quantity  int
	UnitPrice int64 // cents
}

// New enforces the invariants: at least one item, all quantities positive.
func New(id, customerID uuid.UUID, items []Item) (Order, error) {
	if len(items) == 0 {
		return Order{}, ErrEmptyCart
	}
	for _, it := range items {
		if it.Quantity <= 0 {
			return Order{}, ErrInvalidQuantity
		}
	}
	return Order{
		ID:         id,
		CustomerID: customerID,
		Status:     StatusPending,
		Items:      items,
	}, nil
}
