package projection

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
	vk "github.com/valkey-io/valkey-go"
)

// Invalidator is the subset of the Valkey read repo that the projector needs.
// Keeping it narrow makes the projector trivially testable with a stub.
type Invalidator interface {
	Invalidate(ctx context.Context, id uuid.UUID) error
}

// OrderProjector listens on "orders.created" and invalidates the Valkey cache
// so the next read fetches fresh data from Postgres.
//
// Strategy choice: invalidate-on-event rather than update-on-event.
// Simpler and avoids stale writes when events arrive out of order.
// For write-on-event (materialised view), replace the Del with HSet.
type OrderProjector struct {
	cache  Invalidator
	client vk.Client
}

func NewOrderProjector(cache Invalidator, client vk.Client) *OrderProjector {
	return &OrderProjector{cache: cache, client: client}
}

type orderCreatedEvent struct {
	OrderID    string `json:"order_id"`
	CustomerID string `json:"customer_id"`
	Status     string `json:"status"`
}

// Handler satisfies message.NoPublishHandlerFunc.
func (p *OrderProjector) Handler(msg *message.Message) error {
	var evt orderCreatedEvent
	if err := json.Unmarshal(msg.Payload, &evt); err != nil {
		return err
	}

	id, err := uuid.Parse(evt.OrderID)
	if err != nil {
		// Malformed event: ack and skip rather than requeue forever.
		slog.Warn("order projector: bad order_id", "payload", string(msg.Payload))
		msg.Ack()
		return nil
	}

	if err := p.cache.Invalidate(msg.Context(), id); err != nil {
		return err // nack → Watermill retries
	}
	return nil
}
