package order

import "github.com/google/uuid"

// OrderCreated is published after a successful CreateOrder command.
// Kept in the domain package because it describes a business fact, not infra.
type OrderCreated struct {
	OrderID    uuid.UUID `json:"order_id"`
	CustomerID uuid.UUID `json:"customer_id"`
	Status     Status    `json:"status"`
}
