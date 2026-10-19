package health

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// checkTimeout bounds each dependency probe so a hung service can't hang the
// readiness endpoint (and the orchestrator polling it).
const checkTimeout = 2 * time.Second

// Pinger is anything the API depends on that can report whether it's reachable.
type Pinger interface {
	Ping(ctx context.Context) error
}

type Handler struct {
	deps map[string]Pinger
}

// NewHandler takes the API's dependencies keyed by the name reported in /readyz.
func NewHandler(deps map[string]Pinger) *Handler {
	return &Handler{deps: deps}
}

// RegisterRoutes wires the probes. They are public: orchestrators and load
// balancers call them without a token.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.Live)
	mux.HandleFunc("GET /readyz", h.Ready)
}

// Live reports that the process is up and serving. It deliberately checks no
// dependencies: a database outage shouldn't get the API container restarted.
func (h *Handler) Live(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready reports whether every dependency answers, so traffic is only routed
// to an instance that can actually serve it. Responds 503 if any check fails.
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	checks := make(map[string]string, len(h.deps))
	status := http.StatusOK

	for name, dep := range h.deps {
		ctx, cancel := context.WithTimeout(r.Context(), checkTimeout)
		err := dep.Ping(ctx)
		cancel()

		if err != nil {
			log.Printf("readiness: %s: %v", name, err)
			checks[name] = "unavailable"
			status = http.StatusServiceUnavailable
			continue
		}
		checks[name] = "ok"
	}

	overall := "ok"
	if status != http.StatusOK {
		overall = "degraded"
	}
	writeJSON(w, status, struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}{Status: overall, Checks: checks})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}
