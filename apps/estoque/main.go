package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3003"
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = localDatabaseURL
	}

	// Para quando o container recebe SIGTERM (docker stop) ou Ctrl+C.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// O pool é preguiçoso: só conecta na primeira query, então o serviço sobe
	// mesmo com o banco fora do ar.
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("invalid DATABASE_URL: %v", err)
	}
	defer pool.Close()

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
}
