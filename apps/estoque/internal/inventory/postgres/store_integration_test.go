//go:build integration

// Integration tests: a real Postgres in a throwaway container (Testcontainers),
// with the real migrations. Needs Docker. Run: pnpm nx integration estoque
package postgres_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"apps/estoque/internal/inventory/app"
	"apps/estoque/internal/inventory/apptest"
	"apps/estoque/internal/inventory/domain"
	"apps/estoque/internal/inventory/postgres"
)

func startPostgres(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:18-alpine", // same image as compose
		tcpostgres.WithDatabase("estoque"),
		tcpostgres.WithUsername("estoque"),
		tcpostgres.WithPassword("estoque"),
		tcpostgres.BasicWaitStrategies(),
	)
	testcontainers.CleanupContainer(t, container)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	url, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	if err := postgres.Migrate(ctx, url); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := postgres.Migrate(ctx, url); err != nil { // twice: must be a no-op
		t.Fatalf("migrate (second run): %v", err)
	}

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool, url
}

func TestPostgresStore(t *testing.T) {
	pool, _ := startPostgres(t)
	ctx := context.Background()

	t.Run("migrations seed the dev catalog", func(t *testing.T) {
		products, err := postgres.NewStore(pool).Products(ctx, []string{"KB-1", "MS-1", "MN-1", "OUT-1"})
		if err != nil || len(products) != 4 {
			t.Fatalf("seed: %d products, err %v", len(products), err)
		}
	})

	// The same suite as the memory store, now with real locks and transactions.
	apptest.Run(t, func(t *testing.T, products ...*domain.Product) app.Store {
		t.Helper()
		if _, err := pool.Exec(ctx, `TRUNCATE reservation_lines, reservations, products`); err != nil {
			t.Fatalf("reset: %v", err)
		}
		for _, p := range products {
			_, err := pool.Exec(ctx,
				`INSERT INTO products (sku, name, price_cents, currency, on_hand, reserved) VALUES ($1, $2, $3, $4, $5, $6)`,
				p.SKU(), p.Name(), p.Price().Cents, p.Price().Currency, p.OnHand(), p.Reserved())
			if err != nil {
				t.Fatalf("insert product: %v", err)
			}
		}
		return postgres.NewStore(pool)
	})

	t.Run("the database rejects overselling even if the code had a bug", func(t *testing.T) {
		_, err := pool.Exec(ctx, `UPDATE products SET reserved = on_hand + 1 WHERE sku = 'A'`)
		if err == nil {
			t.Fatal("CHECK (reserved <= on_hand) should reject this")
		}
	})
}
