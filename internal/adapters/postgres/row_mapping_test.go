package postgres

// Internal test package accesses the private rowToDomain function directly.
// rowToDomain is a pure function — no I/O, no side effects — ideal for unit tests.

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/localpull/orders/internal/adapters/postgres/db"
	"github.com/localpull/orders/internal/order"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRowToDomain(t *testing.T) {
	t.Run("full order with items", func(t *testing.T) {
		t.Run("maps all fields to the domain model correctly", func(t *testing.T) {
			// Arrange
			orderID := uuid.New()
			customerID := uuid.New()
			productID := uuid.New()
			createdAt := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

			rawItems, _ := json.Marshal([]map[string]any{
				{"product_id": productID.String(), "quantity": 2, "unit_price": 999},
			})
			row := db.GetOrderRow{
				ID:         orderID,
				CustomerID: customerID,
				Status:     "pending",
				CreatedAt:  createdAt,
				Items:      rawItems,
			}

			// Act
			got, err := rowToDomain(row)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, orderID, got.ID)
			assert.Equal(t, customerID, got.CustomerID)
			assert.Equal(t, order.StatusPending, got.Status)
			assert.True(t, got.CreatedAt.Equal(createdAt))
			require.Len(t, got.Items, 1)
			assert.Equal(t, productID, got.Items[0].ProductID)
			assert.Equal(t, 2, got.Items[0].Quantity)
			assert.Equal(t, int64(999), got.Items[0].UnitPrice)
		})
	})

	t.Run("order with empty items array", func(t *testing.T) {
		t.Run("returns domain order with empty items slice", func(t *testing.T) {
			// Arrange
			row := db.GetOrderRow{
				ID:         uuid.New(),
				CustomerID: uuid.New(),
				Status:     "confirmed",
				CreatedAt:  time.Now(),
				Items:      []byte(`[]`),
			}

			// Act
			got, err := rowToDomain(row)

			// Assert
			require.NoError(t, err)
			assert.Empty(t, got.Items)
		})
	})

	t.Run("malformed items JSON", func(t *testing.T) {
		t.Run("returns an error", func(t *testing.T) {
			// Arrange
			row := db.GetOrderRow{
				ID:         uuid.New(),
				CustomerID: uuid.New(),
				Status:     "pending",
				CreatedAt:  time.Now(),
				Items:      []byte(`not valid json`),
			}

			// Act
			_, err := rowToDomain(row)

			// Assert
			assert.Error(t, err)
		})
	})
}
