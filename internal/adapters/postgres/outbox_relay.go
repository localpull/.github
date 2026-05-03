package postgres

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/localpull/orders/internal/adapters/postgres/db"
)

// OutboxRelay polls the outbox table and forwards unpublished messages to
// the Watermill publisher. This gives at-least-once delivery without a
// distributed transaction or a separate change-data-capture pipeline.
//
// In production, replace the polling loop with a Watermill SQL subscriber
// (watermill-sql) or a CDC tool (Debezium) if sub-second latency matters.
type OutboxRelay struct {
	queries   *db.Queries
	publisher message.Publisher
	interval  time.Duration
	batchSize int32
}

func NewOutboxRelay(pool *pgxpool.Pool, pub message.Publisher) *OutboxRelay {
	return &OutboxRelay{
		queries:   db.New(pool),
		publisher: pub,
		interval:  500 * time.Millisecond,
		batchSize: 100,
	}
}

// Run blocks until ctx is cancelled.
func (r *OutboxRelay) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil // normal shutdown, not an error
			}
			return ctx.Err()
		case <-ticker.C:
			if err := r.flush(ctx); err != nil {
				slog.Error("outbox relay: flush error", "err", err)
			}
		}
	}
}

func (r *OutboxRelay) flush(ctx context.Context) error {
	msgs, err := r.queries.UnpublishedOutboxMessages(ctx, r.batchSize)
	if err != nil || len(msgs) == 0 {
		return err
	}

	published := make([]uuid.UUID, 0, len(msgs))
	for _, m := range msgs {
		msg := message.NewMessage(m.ID.String(), []byte(m.Payload))
		if err := r.publisher.Publish(m.Topic, msg); err != nil {
			// Mark what was published so far, leave the failed message for next tick.
			slog.Error("outbox relay: publish failed", "topic", m.Topic, "id", m.ID, "err", err)
			break
		}
		published = append(published, m.ID)
	}

	if len(published) == 0 {
		return nil
	}
	return r.queries.MarkOutboxPublished(ctx, published)
}
