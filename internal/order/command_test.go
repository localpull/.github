package order_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/localpull/orders/internal/order"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateOrderHandler_Handle(t *testing.T) {
	ctx := t.Context()

	validCmd := func() order.CreateOrderCmd {
		return order.CreateOrderCmd{
			OrderID:    uuid.New(),
			CustomerID: uuid.New(),
			Items: []order.Item{
				{ProductID: uuid.New(), Quantity: 1, UnitPrice: 100},
			},
		}
	}

	t.Run("successful creation", func(t *testing.T) {
		t.Run("delegates a valid order to the write repository with pending status", func(t *testing.T) {
			// Arrange
			repo := &stubWriteRepository{}
			h := order.NewCreateOrderHandler(repo)
			cmd := validCmd()

			// Act
			err := h.Handle(ctx, cmd)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, cmd.OrderID, repo.saved.ID)
			assert.Equal(t, order.StatusPending, repo.saved.Status)
		})
	})

	t.Run("domain invariant violations", func(t *testing.T) {
		tests := []struct {
			name    string
			cmd     order.CreateOrderCmd
			wantErr error
		}{
			{
				name: "empty items list is rejected before hitting the repository",
				cmd: order.CreateOrderCmd{
					OrderID:    uuid.New(),
					CustomerID: uuid.New(),
					Items:      nil,
				},
				wantErr: order.ErrEmptyCart,
			},
			{
				name: "item with zero quantity is rejected before hitting the repository",
				cmd: order.CreateOrderCmd{
					OrderID:    uuid.New(),
					CustomerID: uuid.New(),
					Items:      []order.Item{{ProductID: uuid.New(), Quantity: 0, UnitPrice: 100}},
				},
				wantErr: order.ErrInvalidQuantity,
			},
			{
				name: "item with negative quantity is rejected before hitting the repository",
				cmd: order.CreateOrderCmd{
					OrderID:    uuid.New(),
					CustomerID: uuid.New(),
					Items:      []order.Item{{ProductID: uuid.New(), Quantity: -5, UnitPrice: 100}},
				},
				wantErr: order.ErrInvalidQuantity,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				// Arrange
				repo := &stubWriteRepository{}
				h := order.NewCreateOrderHandler(repo)

				// Act
				err := h.Handle(ctx, tc.cmd)

				// Assert
				assert.ErrorIs(t, err, tc.wantErr)
				assert.Equal(t, uuid.UUID{}, repo.saved.ID,
					"repository.Save must not be called when domain validation fails")
			})
		}
	})

	t.Run("infrastructure failure", func(t *testing.T) {
		t.Run("repository error is wrapped and propagated to the caller", func(t *testing.T) {
			// Arrange
			repoErr := errors.New("connection refused")
			repo := &stubWriteRepository{err: repoErr}
			h := order.NewCreateOrderHandler(repo)

			// Act
			err := h.Handle(ctx, validCmd())

			// Assert
			assert.ErrorIs(t, err, repoErr)
		})
	})
}
