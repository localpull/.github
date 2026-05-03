package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/localpull/orders/internal/adapters/postgres/db"
	"github.com/localpull/orders/internal/order"
)

var _ order.WriteRepository = (*OrderWriteRepo)(nil)

// OrderWriteRepo implements order.WriteRepository using Postgres.
// The transactional outbox guarantees that the domain event is only published
// if the order write succeeds — no two-phase commit required.
type OrderWriteRepo struct {
	pool *pgxpool.Pool
}

func NewOrderWriteRepo(pool *pgxpool.Pool) *OrderWriteRepo {
	return &OrderWriteRepo{pool: pool}
}

func (r *OrderWriteRepo) Save(ctx context.Context, o order.Order) error {
	if err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return insertOrderTx(ctx, db.New(tx), o)
	}); err != nil {
		return fmt.Errorf("order.Save %s: %w", o.ID, err)
	}
	return nil
}

// insertOrderTx is a pure transaction body: it performs all writes for one
// order and returns the first error. No logging, no side effects beyond the tx.
func insertOrderTx(ctx context.Context, q *db.Queries, o order.Order) error {
	if err := q.InsertOrder(ctx, db.InsertOrderParams{
		ID:         o.ID,
		CustomerID: o.CustomerID,
		Status:     string(o.Status),
	}); err != nil {
		return err
	}

	for _, item := range o.Items {
		if err := q.InsertOrderItem(ctx, db.InsertOrderItemParams{
			OrderID:   o.ID,
			ProductID: item.ProductID,
			Quantity:  int32(item.Quantity),
			UnitPrice: item.UnitPrice,
		}); err != nil {
			return err
		}
	}

	payload, err := json.Marshal(order.OrderCreated{
		OrderID:    o.ID,
		CustomerID: o.CustomerID,
		Status:     o.Status,
	})
	if err != nil {
		return err
	}

	return q.InsertOutboxMessage(ctx, db.InsertOutboxMessageParams{
		ID:      uuid.New(),
		Topic:   "orders.created",
		Payload: payload,
	})
}
