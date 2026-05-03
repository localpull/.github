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
		return insertOrderTx(ctx, tx, o)
	}); err != nil {
		return fmt.Errorf("order.Save %s: %w", o.ID, err)
	}
	return nil
}

// insertOrderTx is a pure transaction body: no logging, no side effects beyond
// the tx. Items are batched in a single round-trip via pgx.Batch instead of N
// sequential Exec calls, which halves latency for orders with multiple items.
func insertOrderTx(ctx context.Context, tx pgx.Tx, o order.Order) error {
	q := db.New(tx)

	if err := q.InsertOrder(ctx, db.InsertOrderParams{
		ID:         o.ID,
		CustomerID: o.CustomerID,
		Status:     string(o.Status),
	}); err != nil {
		return err
	}

	if len(o.Items) > 0 {
		batch := &pgx.Batch{}
		for _, item := range o.Items {
			batch.Queue(
				`INSERT INTO order_items (order_id, product_id, quantity, unit_price) VALUES ($1, $2, $3, $4)`,
				o.ID, item.ProductID, int32(item.Quantity), item.UnitPrice,
			)
		}
		br := tx.SendBatch(ctx, batch)
		defer br.Close()
		for range o.Items {
			if _, err := br.Exec(); err != nil {
				return err
			}
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
