package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	inventoryv1 "apps/estoque/internal/gen/inventory/v1"
	"apps/estoque/internal/inventory/app"
	"apps/estoque/internal/inventory/grpcapi"
	"apps/estoque/internal/inventory/postgres"
)

// Só para desenvolvimento local (Postgres do docker compose na porta 5443).
// Em container, o compose passa DATABASE_URL.
const localDatabaseURL = "postgres://estoque:estoque@localhost:5443/estoque"

func Hello(name string) string {
	result := "Hello " + name
	return result
}

// pinger é o que o health check precisa do banco. Com uma interface, o teste
// usa um banco falso.
type pinger interface {
	Ping(ctx context.Context) error
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func healthHandler(db pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "db": "down"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "db": "up"})
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func main() {
	port := envOr("PORT", "3003")
	grpcPort := envOr("GRPC_PORT", "50051")
	databaseURL := envOr("DATABASE_URL", localDatabaseURL)

	// Para quando o container recebe SIGTERM (docker stop) ou Ctrl+C.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// `estoque migrate`: applies the migrations and exits (one-shot container).
	// The server itself never migrates, so it boots even with the database down.
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		if err := postgres.Migrate(ctx, databaseURL); err != nil {
			log.Fatalf("migrate: %v", err)
		}
		log.Print("migrations up to date")
		return
	}

	// O pool é preguiçoso: só conecta na primeira query, então o serviço sobe
	// mesmo com o banco fora do ar.
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("invalid DATABASE_URL: %v", err)
	}
	defer pool.Close()

	// --- inventory: domain + use cases on Postgres, exposed over gRPC ---
	inventory := app.NewService(postgres.NewStore(pool))

	grpcServer := grpc.NewServer()
	inventoryv1.RegisterInventoryServiceServer(grpcServer, grpcapi.NewServer(inventory))
	healthpb.RegisterHealthServer(grpcServer, health.NewServer()) // standard gRPC health check
	reflection.Register(grpcServer)                               // lets tools like grpcurl list the API

	grpcListener, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("listen gRPC: %v", err)
	}
	go func() {
		log.Printf("estoque gRPC listening on :%s", grpcPort)
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Fatal(err)
		}
	}()

	// --- HTTP: hello + health ---
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"message": Hello("API")})
	})
	mux.HandleFunc("GET /api/health", healthHandler(pool))

	server := &http.Server{Addr: ":" + port, Handler: mux}
	go func() {
		log.Printf("estoque listening on http://localhost:%s/api", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(shutdownCtx)
	gracefulStop(grpcServer, shutdownCtx)
}

// gracefulStop lets in-flight calls finish, but gives up when ctx expires.
func gracefulStop(server *grpc.Server, ctx context.Context) {
	done := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		server.Stop()
	}
}
