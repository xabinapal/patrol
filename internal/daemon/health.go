package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/xabinapal/patrol/internal/utils"
)

// HealthServer provides an HTTP health endpoint.
type HealthServer struct {
	addr   string
	server *http.Server

	mu            sync.RWMutex
	startTime     time.Time
	lastCheck     time.Time
	tokensManaged int
	renewalsTotal int
	errorsTotal   int
}

// HealthStatus represents the health endpoint response.
type HealthStatus struct {
	Status        string    `json:"status"`
	Uptime        string    `json:"uptime"`
	UptimeSeconds float64   `json:"uptime_seconds"`
	LastCheck     time.Time `json:"last_check,omitempty"`
	TokensManaged int       `json:"tokens_managed"`
	RenewalsTotal int       `json:"renewals_total"`
	ErrorsTotal   int       `json:"errors_total"`
}

// DefaultHealthAddr is the default address for the health server.
const DefaultHealthAddr = "localhost:9090"

// NewHealthServer creates a new health server.
// For security, addresses without an explicit host default to localhost.
func NewHealthServer(addr string) *HealthServer {
	// Security: Default to localhost:9090 if empty
	if addr == "" {
		addr = DefaultHealthAddr
	}
	// Security: Default to localhost if no host specified (e.g., ":8080")
	if strings.HasPrefix(addr, ":") {
		addr = "localhost" + addr
	}
	// Security: If only a port number is provided (e.g., "8080")
	if !strings.Contains(addr, ":") {
		addr = "localhost:" + addr
	}

	return &HealthServer{
		addr:      addr,
		startTime: time.Now(),
	}
}

// securityHeaders adds security headers to HTTP responses.
func securityHeaders(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		w.Header().Set("Cache-Control", "no-store")
		next(w, r)
	}
}

// Start starts the health server.
func (h *HealthServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", securityHeaders(h.handleHealth))
	mux.HandleFunc("/metrics", securityHeaders(h.handleMetrics))
	mux.HandleFunc("/", securityHeaders(h.handleRoot))

	h.server = &http.Server{
		Addr:         h.addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() {
		if err := h.server.ListenAndServe(); err != http.ErrServerClosed {
			fmt.Printf("Health server error: %v\n", err)
		}
	}()

	return nil
}

// Stop stops the health server.
func (h *HealthServer) Stop() error {
	if h.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return h.server.Shutdown(ctx)
	}
	return nil
}

// RecordCheck records a token check.
func (h *HealthServer) RecordCheck(tokensManaged int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastCheck = time.Now()
	h.tokensManaged = tokensManaged
}

// RecordRenewal records a successful renewal.
func (h *HealthServer) RecordRenewal() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.renewalsTotal++
}

// RecordError records an error.
func (h *HealthServer) RecordError() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.errorsTotal++
}

func (h *HealthServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Patrol Daemon</title></head>
<body>
<h1>Patrol Daemon</h1>
<ul>
<li><a href="/health">Health Status (JSON)</a></li>
<li><a href="/metrics">Metrics (Prometheus)</a></li>
</ul>
</body>
</html>`)
}

func (h *HealthServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	uptime := time.Since(h.startTime)
	status := HealthStatus{
		Status:        "ok",
		Uptime:        utils.FormatUptime(uptime),
		UptimeSeconds: uptime.Seconds(),
		LastCheck:     h.lastCheck,
		TokensManaged: h.tokensManaged,
		RenewalsTotal: h.renewalsTotal,
		ErrorsTotal:   h.errorsTotal,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		// Encoding error - response may be partially written
		_ = err
	}
}

func (h *HealthServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	uptime := time.Since(h.startTime)

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	// Prometheus-style metrics
	fmt.Fprintf(w, "# HELP patrol_uptime_seconds Daemon uptime in seconds\n")
	fmt.Fprintf(w, "# TYPE patrol_uptime_seconds gauge\n")
	fmt.Fprintf(w, "patrol_uptime_seconds %.0f\n\n", uptime.Seconds())

	fmt.Fprintf(w, "# HELP patrol_tokens_managed Number of tokens being managed\n")
	fmt.Fprintf(w, "# TYPE patrol_tokens_managed gauge\n")
	fmt.Fprintf(w, "patrol_tokens_managed %d\n\n", h.tokensManaged)

	fmt.Fprintf(w, "# HELP patrol_renewals_total Total number of token renewals\n")
	fmt.Fprintf(w, "# TYPE patrol_renewals_total counter\n")
	fmt.Fprintf(w, "patrol_renewals_total %d\n\n", h.renewalsTotal)

	fmt.Fprintf(w, "# HELP patrol_errors_total Total number of errors\n")
	fmt.Fprintf(w, "# TYPE patrol_errors_total counter\n")
	fmt.Fprintf(w, "patrol_errors_total %d\n\n", h.errorsTotal)

	fmt.Fprintf(w, "# HELP patrol_last_check_timestamp Unix timestamp of last check\n")
	fmt.Fprintf(w, "# TYPE patrol_last_check_timestamp gauge\n")
	if !h.lastCheck.IsZero() {
		fmt.Fprintf(w, "patrol_last_check_timestamp %d\n", h.lastCheck.Unix())
	} else {
		fmt.Fprintf(w, "patrol_last_check_timestamp 0\n")
	}
}
