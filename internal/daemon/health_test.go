package daemon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/xabinapal/patrol/internal/utils"
)

func TestHealthStatus(t *testing.T) {
	status := HealthStatus{
		Status:        "ok",
		TokensManaged: 3,
	}

	if status.Status != "ok" {
		t.Errorf("HealthStatus.Status = %q, want %q", status.Status, "ok")
	}
	if status.TokensManaged != 3 {
		t.Errorf("HealthStatus.TokensManaged = %d, want 3", status.TokensManaged)
	}
}

func TestHealthServer_RecordCheck(t *testing.T) {
	server := NewHealthServer("localhost:0")

	server.RecordCheck(5)

	if server.tokensManaged != 5 {
		t.Errorf("tokensManaged = %d, want 5", server.tokensManaged)
	}
	if server.lastCheck.IsZero() {
		t.Error("lastCheck should not be zero")
	}
}

func TestHealthServer_RecordRenewal(t *testing.T) {
	server := NewHealthServer("localhost:0")

	server.RecordRenewal()
	server.RecordRenewal()
	server.RecordRenewal()

	if server.renewalsTotal != 3 {
		t.Errorf("renewalsTotal = %d, want 3", server.renewalsTotal)
	}
}

func TestHealthServer_RecordError(t *testing.T) {
	server := NewHealthServer("localhost:0")

	server.RecordError()
	server.RecordError()

	if server.errorsTotal != 2 {
		t.Errorf("errorsTotal = %d, want 2", server.errorsTotal)
	}
}

func TestHealthServer_HandleHealth(t *testing.T) {
	server := NewHealthServer("localhost:0")
	server.RecordCheck(3)
	server.RecordRenewal()
	server.RecordError()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status code = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}

	var status HealthStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if status.Status != "ok" {
		t.Errorf("status.Status = %q, want %q", status.Status, "ok")
	}
	if status.TokensManaged != 3 {
		t.Errorf("status.TokensManaged = %d, want 3", status.TokensManaged)
	}
	if status.RenewalsTotal != 1 {
		t.Errorf("status.RenewalsTotal = %d, want 1", status.RenewalsTotal)
	}
	if status.ErrorsTotal != 1 {
		t.Errorf("status.ErrorsTotal = %d, want 1", status.ErrorsTotal)
	}
}

func TestHealthServer_HandleMetrics(t *testing.T) {
	server := NewHealthServer("localhost:0")
	server.RecordCheck(5)
	server.RecordRenewal()
	server.RecordRenewal()
	server.RecordError()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	server.handleMetrics(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status code = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body := w.Body.String()

	// Check for Prometheus metrics
	if !strings.Contains(body, "patrol_tokens_managed 5") {
		t.Error("Expected patrol_tokens_managed metric")
	}
	if !strings.Contains(body, "patrol_renewals_total 2") {
		t.Error("Expected patrol_renewals_total metric")
	}
	if !strings.Contains(body, "patrol_errors_total 1") {
		t.Error("Expected patrol_errors_total metric")
	}
	if !strings.Contains(body, "# HELP") {
		t.Error("Expected HELP comments in metrics")
	}
	if !strings.Contains(body, "# TYPE") {
		t.Error("Expected TYPE comments in metrics")
	}
}

func TestHealthServer_HandleRoot(t *testing.T) {
	server := NewHealthServer("localhost:0")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	server.handleRoot(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status code = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Patrol Daemon") {
		t.Error("Expected 'Patrol Daemon' in response")
	}
	if !strings.Contains(body, "/health") {
		t.Error("Expected link to /health")
	}
	if !strings.Contains(body, "/metrics") {
		t.Error("Expected link to /metrics")
	}
}

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		duration time.Duration
		want     string
	}{
		{45 * time.Second, "45s"},
		{5*time.Minute + 30*time.Second, "5m 30s"},
		{2*time.Hour + 15*time.Minute + 30*time.Second, "2h 15m 30s"},
		{48*time.Hour + 6*time.Hour + 30*time.Minute + 15*time.Second, "2d 6h 30m 15s"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := utils.FormatUptime(tt.duration); got != tt.want {
				t.Errorf("FormatUptime() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewHealthServer(t *testing.T) {
	server := NewHealthServer("localhost:8080")

	if server.addr != "localhost:8080" {
		t.Errorf("addr = %q, want %q", server.addr, "localhost:8080")
	}
	if server.startTime.IsZero() {
		t.Error("startTime should not be zero")
	}
}

func TestNewHealthServer_LocalhostDefault(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{":8080", "localhost:8080"},
		{"8080", "localhost:8080"},
		{"localhost:9090", "localhost:9090"},
		{"127.0.0.1:8080", "127.0.0.1:8080"},
		{"0.0.0.0:8080", "0.0.0.0:8080"}, // User explicitly requested binding to all interfaces
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			server := NewHealthServer(tt.input)
			if server.addr != tt.want {
				t.Errorf("NewHealthServer(%q).addr = %q, want %q", tt.input, server.addr, tt.want)
			}
		})
	}
}

func TestSecurityHeaders(t *testing.T) {
	server := NewHealthServer("localhost:0")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	_ = req // req is used implicitly by w

	// Use securityHeaders wrapped handler
	securityHeaders(server.handleHealth)(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	// Check security headers are set
	if v := resp.Header.Get("X-Content-Type-Options"); v != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want %q", v, "nosniff")
	}
	if v := resp.Header.Get("X-Frame-Options"); v != "DENY" {
		t.Errorf("X-Frame-Options = %q, want %q", v, "DENY")
	}
	if v := resp.Header.Get("Content-Security-Policy"); v != "default-src 'none'" {
		t.Errorf("Content-Security-Policy = %q, want %q", v, "default-src 'none'")
	}
	if v := resp.Header.Get("Cache-Control"); v != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", v, "no-store")
	}
}
