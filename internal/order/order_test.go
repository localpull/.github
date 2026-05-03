package order_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/localpull/orders/internal/order"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {

	validItem := order.Item{
		ProductID: uuid.New(),
		Quantity:  2,
		UnitPrice: 1000,
	}

	t.Run("valid order", func(t *testing.T) {
		t.Run("returns order with pending status and correct identifiers", func(t *testing.T) {
			// Arrange
			id := uuid.New()
			customerID := uuid.New()
			items := []order.Item{validItem}

			// Act
			got, err := order.New(id, customerID, items)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, id, got.ID)
			assert.Equal(t, customerID, got.CustomerID)
			assert.Equal(t, order.StatusPending, got.Status)
			assert.Len(t, got.Items, 1)
		})

		t.Run("preserves all items in order", func(t *testing.T) {
			// Arrange
			items := []order.Item{
				{ProductID: uuid.New(), Quantity: 1, UnitPrice: 500},
				{ProductID: uuid.New(), Quantity: 3, UnitPrice: 1500},
			}

			// Act
			got, err := order.New(uuid.New(), uuid.New(), items)

			// Assert
			require.NoError(t, err)
			assert.Len(t, got.Items, 2)
		})
	})

	t.Run("invalid order", func(t *testing.T) {
		tests := []struct {
			name    string
			items   []order.Item
			wantErr error
		}{
			{
				name:    "empty cart is rejected",
				items:   []order.Item{},
				wantErr: order.ErrEmptyCart,
			},
			{
				name:    "nil items slice is rejected",
				items:   nil,
				wantErr: order.ErrEmptyCart,
			},
			{
				name: "zero quantity is rejected",
				items: []order.Item{
					{ProductID: uuid.New(), Quantity: 0, UnitPrice: 100},
				},
				wantErr: order.ErrInvalidQuantity,
			},
			{
				name: "negative quantity is rejected",
				items: []order.Item{
					{ProductID: uuid.New(), Quantity: -1, UnitPrice: 100},
				},
				wantErr: order.ErrInvalidQuantity,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				// Arrange — done via test case

				// Act
				_, err := order.New(uuid.New(), uuid.New(), tc.items)

				// Assert
				assert.ErrorIs(t, err, tc.wantErr)
			})
		}
	})
}
