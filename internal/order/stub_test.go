package order_test

// stub_test.go holds in-memory test doubles for the order package tests.
// Placing them in a single file makes them easy to find and extend.

import (
	"context"

	"github.com/google/uuid"
	"github.com/localpull/orders/internal/order"
)

// stubReadRepository is an in-memory ReadRepository for unit tests.
type stubReadRepository struct {
	order order.Order
	err   error
}

func (s *stubReadRepository) GetByID(_ context.Context, _ uuid.UUID) (order.Order, error) {
	return s.order, s.err
}

// stubWriteRepository captures the saved order for assertion.
type stubWriteRepository struct {
	saved order.Order
	err   error
}

func (s *stubWriteRepository) Save(_ context.Context, o order.Order) error {
	s.saved = o
	return s.err
}
