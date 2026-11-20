-- +goose Up
-- DEV SEED: the starting catalog (same as the old in-memory seed).
-- Idempotent. `docker compose down -v` wipes the volume and resets the stock.
INSERT INTO products (sku, name, price_cents, currency, on_hand) VALUES
    ('KB-1',  'Mechanical keyboard', 45000,  'BRL', 10),
    ('MS-1',  'Wireless mouse',      15000,  'BRL', 25),
    ('MN-1',  '27" monitor',         189900, 'BRL', 3),
    ('OUT-1', 'Always out of stock', 1000,   'BRL', 0)
ON CONFLICT (sku) DO NOTHING;

-- +goose Down
DELETE FROM products WHERE sku IN ('KB-1', 'MS-1', 'MN-1', 'OUT-1');
