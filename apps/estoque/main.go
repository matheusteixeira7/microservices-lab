package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

func Hello(name string) string {
	result := "Hello " + name
	return result
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3003"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": Hello("API")})
	})

	log.Printf("estoque listening on http://localhost:%s/api", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
