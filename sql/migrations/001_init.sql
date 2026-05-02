CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS orders (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID        NOT NULL,
    status      TEXT        NOT NULL CHECK (status IN ('pending','confirmed','cancelled')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS order_items (
    order_id   UUID   NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id UUID   NOT NULL,
    quantity   INT    NOT NULL CHECK (quantity > 0),
    unit_price BIGINT NOT NULL CHECK (unit_price >= 0), -- cents
    PRIMARY KEY (order_id, product_id)
);

-- Transactional outbox: events are written in the same transaction as the
-- order, then picked up by the OutboxRelay and forwarded to Watermill.
CREATE TABLE IF NOT EXISTS outbox_messages (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    topic        TEXT        NOT NULL,
    payload      JSONB       NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS outbox_unpublished
    ON outbox_messages (created_at)
    WHERE published_at IS NULL;
