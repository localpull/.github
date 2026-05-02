package order

import (
	"context"

	"github.com/google/uuid"
)

// WriteRepository is the port for persisting orders (command side).
// The postgres adapter implements this; tests use a fake/in-memory stub.
type WriteRepository interface {
	Save(ctx context.Context, o Order) error
}

// ReadRepository is the port for reading orders (query side).
// Can be implemented by Postgres directly, or by a caching decorator that
// wraps Postgres with Valkey — the query handler never knows the difference.
type ReadRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (Order, error)
}
