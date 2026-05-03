package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/localpull/orders/internal/adapters/postgres/db"
	"github.com/localpull/orders/internal/order"
)

var _ order.ReadRepository = (*OrderReadRepo)(nil)

// OrderReadRepo implements order.ReadRepository using Postgres.
// It is the origin / source-of-truth reader; the Valkey adapter wraps it.
type OrderReadRepo struct {
	queries *db.Queries
}

func NewOrderReadRepo(pool *pgxpool.Pool) *OrderReadRepo {
	return &OrderReadRepo{queries: db.New(pool)}
}

func (r *OrderReadRepo) GetByID(ctx context.Context, id uuid.UUID) (order.Order, error) {
	row, err := r.queries.GetOrder(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return order.Order{}, fmt.Errorf("order %s: %w", id, order.ErrNotFound)
		}
		return order.Order{}, fmt.Errorf("order.GetByID %s: %w", id, err)
	}
	result, err := rowToDomain(row)
	if err != nil {
		return order.Order{}, fmt.Errorf("order.GetByID %s: map: %w", id, err)
	}
	return result, nil
}

// rawItem mirrors the JSON shape produced by json_build_object in the query.
// Pure: no side effects, used only in rowToDomain.
type rawItem struct {
	ProductID uuid.UUID `json:"product_id"`
	Quantity  int       `json:"quantity"`
	UnitPrice int64     `json:"unit_price"`
}

// rowToDomain is a pure mapping function: no I/O, no side effects.
func rowToDomain(row db.GetOrderRow) (order.Order, error) {
	var raw []rawItem
	if err := json.Unmarshal(row.Items, &raw); err != nil {
		return order.Order{}, err
	}

	items := make([]order.Item, len(raw))
	for i, it := range raw {
		items[i] = order.Item{
			ProductID: it.ProductID,
			Quantity:  it.Quantity,
			UnitPrice: it.UnitPrice,
		}
	}

	return order.Order{
		ID:         row.ID,
		CustomerID: row.CustomerID,
		Status:     order.Status(row.Status),
		Items:      items,
		CreatedAt:  row.CreatedAt,
	}, nil
}
