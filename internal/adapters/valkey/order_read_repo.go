package valkey

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/localpull/orders/internal/order"
	vk "github.com/valkey-io/valkey-go"
)

// OrderReadRepo is a cache-aside decorator over order.ReadRepository.
// On a cache miss it delegates to the origin (Postgres), then warms the cache.
// The query handler only sees the order.ReadRepository interface — it has no
// idea whether the data came from Valkey or Postgres.
type OrderReadRepo struct {
	origin order.ReadRepository
	client vk.Client
	ttl    time.Duration
}

func NewOrderReadRepo(origin order.ReadRepository, client vk.Client) *OrderReadRepo {
	return &OrderReadRepo{origin: origin, client: client, ttl: 5 * time.Minute}
}

func (r *OrderReadRepo) GetByID(ctx context.Context, id uuid.UUID) (order.Order, error) {
	key := cacheKey(id)

	if hit, err := r.getFromCache(ctx, key); err == nil {
		return hit, nil
	}

	o, err := r.origin.GetByID(ctx, id)
	if err != nil {
		return order.Order{}, err
	}

	r.setInCache(ctx, key, o)
	return o, nil
}

// Invalidate removes the cached entry for an order.
// Called by the OrderProjector when an OrderCreated event is received.
func (r *OrderReadRepo) Invalidate(ctx context.Context, id uuid.UUID) error {
	return r.client.Do(ctx, r.client.B().Del().Key(cacheKey(id)).Build()).Error()
}

func (r *OrderReadRepo) getFromCache(ctx context.Context, key string) (order.Order, error) {
	val, err := r.client.Do(ctx, r.client.B().Get().Key(key).Build()).ToString()
	if err != nil {
		return order.Order{}, err
	}
	var o order.Order
	return o, json.Unmarshal([]byte(val), &o)
}

func (r *OrderReadRepo) setInCache(ctx context.Context, key string, o order.Order) {
	b, err := json.Marshal(o)
	if err != nil {
		slog.Warn("valkey: marshal failed", "key", key, "err", err)
		return
	}
	cmd := r.client.B().Set().Key(key).Value(string(b)).Ex(r.ttl).Build()
	if err := r.client.Do(ctx, cmd).Error(); err != nil {
		slog.Warn("valkey: set failed", "key", key, "err", err)
	}
}

func cacheKey(id uuid.UUID) string { return "order:" + id.String() }
