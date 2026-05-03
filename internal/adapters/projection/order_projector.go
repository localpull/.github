package projection

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
	"github.com/localpull/orders/internal/order"
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
	cache Invalidator
}

func NewOrderProjector(cache Invalidator) *OrderProjector {
	return &OrderProjector{cache: cache}
}

// Handler satisfies message.NoPublishHandlerFunc.
// It reuses order.OrderCreated to stay in sync with the event schema.
func (p *OrderProjector) Handler(msg *message.Message) error {
	var evt order.OrderCreated
	if err := json.Unmarshal(msg.Payload, &evt); err != nil {
		// Malformed event: ack and skip rather than requeue forever.
		slog.Warn("order projector: malformed payload", "payload_size", len(msg.Payload))
		msg.Ack()
		return nil
	}

	if err := p.cache.Invalidate(msg.Context(), evt.OrderID); err != nil {
		return err // nack → Watermill retries
	}
	return nil
}
