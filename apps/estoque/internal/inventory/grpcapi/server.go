// Package grpcapi is the gRPC adapter of the inventory: it translates
// protobuf <-> app.Service and domain errors -> gRPC status codes.
// No business rules here.
package grpcapi

import (
	"context"
	"errors"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	inventoryv1 "apps/estoque/internal/gen/inventory/v1"
	"apps/estoque/internal/inventory/app"
	"apps/estoque/internal/inventory/domain"
)

type Server struct {
	inventoryv1.UnimplementedInventoryServiceServer
	svc *app.Service
}

func NewServer(svc *app.Service) *Server {
	return &Server{svc: svc}
}

func (s *Server) FindProducts(ctx context.Context, req *inventoryv1.FindProductsRequest) (*inventoryv1.FindProductsResponse, error) {
	products, err := s.svc.FindProducts(ctx, req.GetSkus())
	if err != nil {
		return nil, toStatus(err)
	}
	res := &inventoryv1.FindProductsResponse{}
	for _, p := range products {
		res.Products = append(res.Products, &inventoryv1.Product{
			Sku:   p.SKU,
			Name:  p.Name,
			Price: &inventoryv1.Money{Cents: p.PriceCents, Currency: p.Currency},
		})
	}
	return res, nil
}

func (s *Server) ReserveStock(ctx context.Context, req *inventoryv1.ReserveStockRequest) (*inventoryv1.ReserveStockResponse, error) {
	lines := make([]domain.Line, 0, len(req.GetItems()))
	for _, item := range req.GetItems() {
		lines = append(lines, domain.Line{SKU: item.GetSku(), Quantity: int(item.GetQuantity())})
	}

	err := s.svc.Reserve(ctx, req.GetOrderId(), lines)

	// Out of stock is a normal answer (OK status), not an error.
	var outOfStock *domain.OutOfStockError
	switch {
	case err == nil:
		return &inventoryv1.ReserveStockResponse{
			Result: &inventoryv1.ReserveStockResponse_Reserved_{Reserved: &inventoryv1.ReserveStockResponse_Reserved{}},
		}, nil
	case errors.As(err, &outOfStock):
		return &inventoryv1.ReserveStockResponse{
			Result: &inventoryv1.ReserveStockResponse_OutOfStock_{
				OutOfStock: &inventoryv1.ReserveStockResponse_OutOfStock{UnavailableSkus: outOfStock.SKUs},
			},
		}, nil
	default:
		return nil, toStatus(err)
	}
}

func (s *Server) ReleaseStock(ctx context.Context, req *inventoryv1.ReleaseStockRequest) (*inventoryv1.ReleaseStockResponse, error) {
	if err := s.svc.Release(ctx, req.GetOrderId()); err != nil {
		return nil, toStatus(err)
	}
	return &inventoryv1.ReleaseStockResponse{}, nil
}

func (s *Server) CommitStock(ctx context.Context, req *inventoryv1.CommitStockRequest) (*inventoryv1.CommitStockResponse, error) {
	if err := s.svc.Commit(ctx, req.GetOrderId()); err != nil {
		return nil, toStatus(err)
	}
	return &inventoryv1.CommitStockResponse{}, nil
}

// toStatus maps errors to gRPC status codes (the table is in inventory.proto).
func toStatus(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidReservation), errors.Is(err, domain.ErrInvalidQuantity):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrIdempotencyKeyReuse):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrReservationReleased), errors.Is(err, domain.ErrInvalidTransition):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, app.ErrReservationNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "canceled")
	default:
		// Log the details here; never send them to the client.
		slog.Error("inventory: internal error", "err", err)
		return status.Error(codes.Internal, "internal error")
	}
}
