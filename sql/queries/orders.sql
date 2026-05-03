-- name: InsertOrder :exec
INSERT INTO orders (id, customer_id, status)
VALUES (@id, @customer_id, @status);

-- name: InsertOrderItem :exec
INSERT INTO order_items (order_id, product_id, quantity, unit_price)
VALUES (@order_id, @product_id, @quantity, @unit_price);

-- name: InsertOutboxMessage :exec
INSERT INTO outbox_messages (id, topic, payload)
VALUES (@id, @topic, @payload);

-- name: GetOrder :one
SELECT
    o.id,
    o.customer_id,
    o.status,
    o.created_at,
    COALESCE(
        json_agg(json_build_object(
            'product_id', oi.product_id,
            'quantity',   oi.quantity,
            'unit_price', oi.unit_price
        )) FILTER (WHERE oi.order_id IS NOT NULL),
        '[]'
    ) AS items
FROM orders o
LEFT JOIN order_items oi ON oi.order_id = o.id
WHERE o.id = @id
GROUP BY o.id;

-- name: UnpublishedOutboxMessages :many
SELECT id, topic, payload
FROM outbox_messages
WHERE published_at IS NULL
ORDER BY created_at
LIMIT @batch_size;

-- name: MarkOutboxPublished :exec
UPDATE outbox_messages
SET published_at = now()
WHERE id = ANY(@ids::uuid[]);
