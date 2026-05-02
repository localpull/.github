package order

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound  = errors.New("order: not found")
	ErrEmptyCart = errors.New("order: must have at least one item")
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

// New enforces the invariant: an order must always have at least one item.
func New(id, customerID uuid.UUID, items []Item) (Order, error) {
	if len(items) == 0 {
		return Order{}, ErrEmptyCart
	}
	return Order{
		ID:         id,
		CustomerID: customerID,
		Status:     StatusPending,
		Items:      items,
	}, nil
}

// OrderCreated is the domain event written to the outbox on a successful save.
// Kept in the domain package because it describes a business fact, not infra.
type OrderCreated struct {
	OrderID    uuid.UUID `json:"order_id"`
	CustomerID uuid.UUID `json:"customer_id"`
	Status     Status    `json:"status"`
}
