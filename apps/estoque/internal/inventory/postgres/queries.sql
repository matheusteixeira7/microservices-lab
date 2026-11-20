-- name: ListProducts :many
-- Read-only catalog lookup (no lock).
SELECT sku, name, price_cents, currency, on_hand, reserved
FROM products
WHERE sku = ANY(@skus::text[])
ORDER BY sku;

-- name: LockProducts :many
-- Locks the rows until the transaction ends. ORDER BY sku = the same lock
-- order in every transaction, so two transactions never deadlock.
SELECT sku, name, price_cents, currency, on_hand, reserved
FROM products
WHERE sku = ANY(@skus::text[])
ORDER BY sku
FOR UPDATE;

-- name: LockOrder :exec
-- FOR UPDATE only locks rows that exist. This lock works even before the
-- reservation row exists, so two requests for the same NEW order run one
-- after the other. Released automatically when the transaction ends.
SELECT pg_advisory_xact_lock(hashtextextended(@order_id::text, 0));

-- name: GetReservation :one
SELECT order_id, status FROM reservations WHERE order_id = @order_id;

-- name: ListReservationLines :many
SELECT sku, quantity FROM reservation_lines WHERE order_id = @order_id ORDER BY sku;

-- name: UpdateProductStock :execrows
UPDATE products
SET on_hand = @on_hand, reserved = @reserved, updated_at = now()
WHERE sku = @sku;

-- name: UpsertReservation :exec
INSERT INTO reservations (order_id, status)
VALUES (@order_id, @status)
ON CONFLICT (order_id) DO UPDATE SET status = excluded.status, updated_at = now();

-- name: InsertReservationLine :exec
-- Lines never change after the reservation is created.
INSERT INTO reservation_lines (order_id, sku, quantity)
VALUES (@order_id, @sku, @quantity)
ON CONFLICT (order_id, sku) DO NOTHING;
