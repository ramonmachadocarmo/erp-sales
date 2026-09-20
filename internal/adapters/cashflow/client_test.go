package cashflow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestScheduleSaleZeroAmountClearsInsteadOfScheduling(t *testing.T) {
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodPost {
			http.Error(w, "invalid", http.StatusBadRequest) // what cashflow does for amount <= 0
			return
		}
		w.WriteHeader(http.StatusNotFound) // nothing scheduled yet is fine
	}))
	defer srv.Close()

	err := New(srv.URL).ScheduleSale(context.Background(), "so1", "c1", "m", "t", 0, time.Now())
	if err != nil {
		t.Fatalf("zero-value order must not fail: %v", err)
	}
	if len(calls) != 1 || calls[0] != "DELETE /schedule/SALE/so1" {
		t.Fatalf("expected a single cancel call, got %v", calls)
	}
}

func TestScheduleSalePositiveAmountStillSchedules(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Method + " " + r.URL.Path
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	if err := New(srv.URL).ScheduleSale(context.Background(), "so1", "c1", "m", "t", 10, time.Now()); err != nil {
		t.Fatal(err)
	}
	if got != "POST /schedule" {
		t.Fatalf("got %s", got)
	}
}
