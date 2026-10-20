package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type pingFunc func(ctx context.Context) error

func (f pingFunc) Ping(ctx context.Context) error { return f(ctx) }

var (
	up   = pingFunc(func(context.Context) error { return nil })
	down = pingFunc(func(context.Context) error { return errors.New("dial tcp: connection refused") })
)

func get(t *testing.T, h *Handler, path string) (int, map[string]any) {
	t.Helper()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("%s: decode %q: %v", path, rec.Body, err)
	}
	return rec.Code, body
}

func TestLiveIgnoresDependencies(t *testing.T) {
	code, body := get(t, NewHandler(map[string]Pinger{"postgres": down}), "/healthz")
	if code != http.StatusOK || body["status"] != "ok" {
		t.Fatalf("got %d %v, want 200 ok", code, body)
	}
}

func TestReadyAllUp(t *testing.T) {
	code, body := get(t, NewHandler(map[string]Pinger{"postgres": up, "redis": up}), "/readyz")
	if code != http.StatusOK || body["status"] != "ok" {
		t.Fatalf("got %d %v, want 200 ok", code, body)
	}
}

func TestReadyReportsWhichDependencyFailed(t *testing.T) {
	code, body := get(t, NewHandler(map[string]Pinger{"postgres": up, "redis": down}), "/readyz")
	if code != http.StatusServiceUnavailable || body["status"] != "degraded" {
		t.Fatalf("got %d %v, want 503 degraded", code, body)
	}
	checks := body["checks"].(map[string]any)
	if checks["postgres"] != "ok" || checks["redis"] != "unavailable" {
		t.Fatalf("checks = %v", checks)
	}
	// Error details go to the log, not to unauthenticated callers.
	if raw, _ := json.Marshal(body); strings.Contains(string(raw), "refused") {
		t.Fatalf("error detail leaked: %s", raw)
	}
}

func TestReadyTimesOutHungDependency(t *testing.T) {
	hung := pingFunc(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	start := time.Now()
	code, _ := get(t, NewHandler(map[string]Pinger{"minio": hung}), "/readyz")
	if code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", code)
	}
	if elapsed := time.Since(start); elapsed > checkTimeout+time.Second {
		t.Fatalf("readyz took %s, want it bounded by %s", elapsed, checkTimeout)
	}
}
