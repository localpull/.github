package order

import (
	"context"

	"github.com/google/uuid"
)

type CreateOrderCmd struct {
	OrderID    uuid.UUID
	CustomerID uuid.UUID
	Items      []Item
}

// CreateOrderHandler depends only on the WriteRepository port — never on
// a concrete *pgxpool.Pool or *sqlc.Queries.
type CreateOrderHandler struct {
	repo WriteRepository
}

func NewCreateOrderHandler(repo WriteRepository) *CreateOrderHandler {
	return &CreateOrderHandler{repo: repo}
}

func (h *CreateOrderHandler) Handle(ctx context.Context, cmd CreateOrderCmd) error {
	o, err := New(cmd.OrderID, cmd.CustomerID, cmd.Items)
	if err != nil {
		return err
	}
	return h.repo.Save(ctx, o)
}
