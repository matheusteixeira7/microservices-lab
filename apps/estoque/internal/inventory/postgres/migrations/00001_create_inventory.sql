-- +goose Up
-- The CHECKs repeat the domain rules: the domain is the first line of
-- defense, the database is the last one.
CREATE TABLE products (
    sku         text        PRIMARY KEY,
    name        text        NOT NULL,
    price_cents bigint      NOT NULL CHECK (price_cents >= 0),
    currency    text        NOT NULL,
    on_hand     integer     NOT NULL CHECK (on_hand >= 0),
    reserved    integer     NOT NULL DEFAULT 0,
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT products_reserved_check CHECK (reserved >= 0 AND reserved <= on_hand)
);

CREATE TABLE reservations (
    order_id   text        PRIMARY KEY,
    status     text        NOT NULL CHECK (status IN ('ACTIVE', 'COMMITTED', 'RELEASED')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- A tombstone (released before it was reserved) has no lines.
CREATE TABLE reservation_lines (
    order_id text    NOT NULL REFERENCES reservations (order_id) ON DELETE CASCADE,
    sku      text    NOT NULL REFERENCES products (sku),
    quantity integer NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (order_id, sku)
);

-- +goose Down
DROP TABLE reservation_lines;
DROP TABLE reservations;
DROP TABLE products;
