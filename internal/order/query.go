package order

import (
	"context"

	"github.com/google/uuid"
)

type GetOrderQuery struct {
	OrderID uuid.UUID
}

type GetOrderHandler struct {
	repo ReadRepository
}

func NewGetOrderHandler(repo ReadRepository) *GetOrderHandler {
	return &GetOrderHandler{repo: repo}
}

func (h *GetOrderHandler) Handle(ctx context.Context, q GetOrderQuery) (OrderView, error) {
	o, err := h.repo.GetByID(ctx, q.OrderID)
	if err != nil {
		return OrderView{}, err
	}
	return toView(o), nil
}
