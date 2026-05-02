package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/localpull/orders/internal/adapters/postgres/db"
	"github.com/localpull/orders/internal/order"
)

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
			return order.Order{}, order.ErrNotFound
		}
		return order.Order{}, err
	}
	return rowToDomain(row)
}

// rawItem mirrors the JSON shape produced by json_build_object in the query.
type rawItem struct {
	ProductID uuid.UUID `json:"product_id"`
	Quantity  int       `json:"quantity"`
	UnitPrice int64     `json:"unit_price"`
}

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
