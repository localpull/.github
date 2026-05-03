package order_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/localpull/orders/internal/order"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetOrderHandler_Handle(t *testing.T) {
	ctx := t.Context()

	t.Run("found order is mapped to view correctly", func(t *testing.T) {
		t.Run("scalar fields are converted to strings", func(t *testing.T) {
			// Arrange
			id := uuid.New()
			customerID := uuid.New()
			createdAt := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

			stub := &stubReadRepository{order: order.Order{
				ID:         id,
				CustomerID: customerID,
				Status:     order.StatusPending,
				Items:      []order.Item{},
				CreatedAt:  createdAt,
			}}
			h := order.NewGetOrderHandler(stub)

			// Act
			view, err := h.Handle(ctx, order.GetOrderQuery{OrderID: id})

			// Assert
			require.NoError(t, err)
			assert.Equal(t, id.String(), view.ID)
			assert.Equal(t, customerID.String(), view.CustomerID)
			assert.Equal(t, string(order.StatusPending), view.Status)
			assert.Equal(t, "2024-01-15T10:00:00Z", view.CreatedAt)
		})

		t.Run("items are mapped to item views with correct values", func(t *testing.T) {
			// Arrange
			productID := uuid.New()
			stub := &stubReadRepository{order: order.Order{
				ID:         uuid.New(),
				CustomerID: uuid.New(),
				Status:     order.StatusConfirmed,
				Items: []order.Item{
					{ProductID: productID, Quantity: 3, UnitPrice: 500},
				},
				CreatedAt: time.Now(),
			}}
			h := order.NewGetOrderHandler(stub)

			// Act
			view, err := h.Handle(ctx, order.GetOrderQuery{OrderID: stub.order.ID})

			// Assert
			require.NoError(t, err)
			require.Len(t, view.Items, 1)
			assert.Equal(t, productID.String(), view.Items[0].ProductID)
			assert.Equal(t, 3, view.Items[0].Quantity)
			assert.Equal(t, int64(500), view.Items[0].UnitPrice)
		})

		t.Run("empty items list produces empty items array in view", func(t *testing.T) {
			// Arrange
			stub := &stubReadRepository{order: order.Order{
				ID:        uuid.New(),
				Items:     []order.Item{},
				CreatedAt: time.Now(),
			}}
			h := order.NewGetOrderHandler(stub)

			// Act
			view, err := h.Handle(ctx, order.GetOrderQuery{OrderID: stub.order.ID})

			// Assert
			require.NoError(t, err)
			assert.Empty(t, view.Items)
		})
	})

	t.Run("repository error is propagated to the caller", func(t *testing.T) {
		// Arrange
		stub := &stubReadRepository{err: order.ErrNotFound}
		h := order.NewGetOrderHandler(stub)

		// Act
		_, err := h.Handle(ctx, order.GetOrderQuery{OrderID: uuid.New()})

		// Assert
		assert.Error(t, err)
	})
}
