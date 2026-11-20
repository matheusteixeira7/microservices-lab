package grpcapi_test

import (
	"context"
	"errors"
	"net"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	inventoryv1 "apps/estoque/internal/gen/inventory/v1"
	"apps/estoque/internal/inventory/app"
	"apps/estoque/internal/inventory/domain"
	"apps/estoque/internal/inventory/grpcapi"
	"apps/estoque/internal/inventory/memory"
)

// startServer runs the real gRPC server over an in-memory connection
// (bufconn): real protocol, no ports.
func startServer(t *testing.T, store app.Store) inventoryv1.InventoryServiceClient {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	inventoryv1.RegisterInventoryServiceServer(server, grpcapi.NewServer(app.NewService(store)))
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return inventoryv1.NewInventoryServiceClient(conn)
}

// A has 5 units, B has 1.
func newClient(t *testing.T) inventoryv1.InventoryServiceClient {
	t.Helper()
	var products []*domain.Product
	for sku, qty := range map[string]int{"A": 5, "B": 1} {
		p, err := domain.NewProduct(sku, "Product "+sku, domain.Money{Cents: 1000, Currency: "BRL"}, qty)
		if err != nil {
			t.Fatalf("NewProduct: %v", err)
		}
		products = append(products, p)
	}
	return startServer(t, memory.NewStore(products...))
}

var ctx = context.Background()

func reserve(client inventoryv1.InventoryServiceClient, orderID, sku string, qty int32) (*inventoryv1.ReserveStockResponse, error) {
	return client.ReserveStock(ctx, &inventoryv1.ReserveStockRequest{
		OrderId: orderID,
		Items:   []*inventoryv1.ReserveItem{{Sku: sku, Quantity: qty}},
	})
}

func wantCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if got := status.Code(err); got != want {
		t.Fatalf("code = %s, want %s (err: %v)", got, want, err)
	}
}

func TestFindProductsReturnsOnlyExistingOnes(t *testing.T) {
	client := newClient(t)

	res, err := client.FindProducts(ctx, &inventoryv1.FindProductsRequest{Skus: []string{"A", "NOPE"}})

	if err != nil {
		t.Fatalf("FindProducts: %v", err)
	}
	if len(res.GetProducts()) != 1 {
		t.Fatalf("products = %v, want only A", res.GetProducts())
	}
	p := res.GetProducts()[0]
	if p.GetSku() != "A" || p.GetName() != "Product A" || p.GetPrice().GetCents() != 1000 || p.GetPrice().GetCurrency() != "BRL" {
		t.Fatalf("product = %v", p)
	}
}

func TestReserveStockReserved(t *testing.T) {
	client := newClient(t)

	res, err := reserve(client, "order-1", "A", 2)

	if err != nil {
		t.Fatalf("ReserveStock: %v", err)
	}
	if res.GetReserved() == nil {
		t.Fatalf("result = %v, want reserved", res.GetResult())
	}
}

func TestReserveStockOutOfStockIsAResultNotAnError(t *testing.T) {
	client := newClient(t)

	res, err := client.ReserveStock(ctx, &inventoryv1.ReserveStockRequest{
		OrderId: "order-1",
		Items: []*inventoryv1.ReserveItem{
			{Sku: "A", Quantity: 1},
			{Sku: "B", Quantity: 2},    // not enough
			{Sku: "NOPE", Quantity: 1}, // unknown
		},
	})

	if err != nil {
		t.Fatalf("out of stock must not be an error, got %v", err)
	}
	if got, want := res.GetOutOfStock().GetUnavailableSkus(), []string{"B", "NOPE"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unavailable = %v, want %v", got, want)
	}
}

func TestStatusCodes(t *testing.T) {
	t.Run("INVALID_ARGUMENT: empty order id", func(t *testing.T) {
		_, err := reserve(newClient(t), "", "A", 1)
		wantCode(t, err, codes.InvalidArgument)
	})
	t.Run("INVALID_ARGUMENT: quantity <= 0", func(t *testing.T) {
		_, err := reserve(newClient(t), "order-1", "A", 0)
		wantCode(t, err, codes.InvalidArgument)
	})
	t.Run("ALREADY_EXISTS: same order id, different items", func(t *testing.T) {
		client := newClient(t)
		_, _ = reserve(client, "order-1", "A", 1)
		_, err := reserve(client, "order-1", "A", 2)
		wantCode(t, err, codes.AlreadyExists)
	})
	t.Run("FAILED_PRECONDITION: reserve after release", func(t *testing.T) {
		client := newClient(t)
		_, _ = client.ReleaseStock(ctx, &inventoryv1.ReleaseStockRequest{OrderId: "order-1"})
		_, err := reserve(client, "order-1", "A", 1)
		wantCode(t, err, codes.FailedPrecondition)
	})
	t.Run("FAILED_PRECONDITION: release after commit", func(t *testing.T) {
		client := newClient(t)
		_, _ = reserve(client, "order-1", "A", 1)
		_, _ = client.CommitStock(ctx, &inventoryv1.CommitStockRequest{OrderId: "order-1"})
		_, err := client.ReleaseStock(ctx, &inventoryv1.ReleaseStockRequest{OrderId: "order-1"})
		wantCode(t, err, codes.FailedPrecondition)
	})
	t.Run("NOT_FOUND: commit without reservation", func(t *testing.T) {
		_, err := newClient(t).CommitStock(ctx, &inventoryv1.CommitStockRequest{OrderId: "nope"})
		wantCode(t, err, codes.NotFound)
	})
	t.Run("INTERNAL: storage failure, details not leaked", func(t *testing.T) {
		client := startServer(t, failingStore{err: errors.New("connection reset by peer")})
		_, err := reserve(client, "order-1", "A", 1)
		wantCode(t, err, codes.Internal)
		if strings.Contains(err.Error(), "connection reset") {
			t.Fatalf("internal details leaked: %v", err)
		}
	})
}

func TestReleaseAndCommitSucceed(t *testing.T) {
	client := newClient(t)
	_, _ = reserve(client, "order-1", "A", 1)
	_, _ = reserve(client, "order-2", "A", 1)

	if _, err := client.ReleaseStock(ctx, &inventoryv1.ReleaseStockRequest{OrderId: "order-1"}); err != nil {
		t.Fatalf("ReleaseStock: %v", err)
	}
	if _, err := client.CommitStock(ctx, &inventoryv1.CommitStockRequest{OrderId: "order-2"}); err != nil {
		t.Fatalf("CommitStock: %v", err)
	}
}

type failingStore struct{ err error }

func (s failingStore) Products(context.Context, []string) ([]*domain.Product, error) {
	return nil, s.err
}

func (s failingStore) InTx(context.Context, func(context.Context, app.Tx) error) error {
	return s.err
}
