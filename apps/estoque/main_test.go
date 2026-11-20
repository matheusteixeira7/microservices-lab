package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHello(t *testing.T) {
	result := Hello("world")
	if result != "Hello world" {
		t.Error("Expected Hello to append 'world'")
	}
}

type fakeDB struct{ err error }

func (f fakeDB) Ping(context.Context) error { return f.err }

func TestHealthHandler(t *testing.T) {
	tests := []struct {
		name string
		db   fakeDB
		want int
	}{
		{"database up", fakeDB{}, http.StatusOK},
		{"database down", fakeDB{err: errors.New("connection refused")}, http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			healthHandler(tt.db)(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
