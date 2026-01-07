package vault

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xabinapal/patrol/internal/types"
)

func TestCheckHealth(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		expectedStatus string
		expectedMsg    string
	}{
		{
			name:           "healthy server",
			statusCode:     200,
			expectedStatus: "healthy",
			expectedMsg:    "initialized, unsealed, active",
		},
		{
			name:           "standby node (429)",
			statusCode:     429,
			expectedStatus: "standby",
			expectedMsg:    "standby node",
		},
		{
			name:           "standby node (472)",
			statusCode:     472,
			expectedStatus: "standby",
			expectedMsg:    "standby node",
		},
		{
			name:           "standby node (473)",
			statusCode:     473,
			expectedStatus: "standby",
			expectedMsg:    "standby node",
		},
		{
			name:           "uninitialized server",
			statusCode:     501,
			expectedStatus: "uninitialized",
			expectedMsg:    "server is not initialized",
		},
		{
			name:           "sealed server",
			statusCode:     503,
			expectedStatus: "sealed",
			expectedMsg:    "server is sealed",
		},
		{
			name:           "unknown status",
			statusCode:     404,
			expectedStatus: "unknown",
			expectedMsg:    "unexpected status: 404",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server that returns the specified status code
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/sys/health" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			prof := &types.Profile{
				Name:    "test",
				Address: server.URL,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			executor := NewHealthExecutor()
			status := executor.CheckHealth(ctx, prof)

			if status == nil {
				t.Fatal("CheckHealth() returned nil")
			}

			if status.Status != tt.expectedStatus {
				t.Errorf("CheckHealth() Status = %q, want %q", status.Status, tt.expectedStatus)
			}

			if status.Message != tt.expectedMsg {
				t.Errorf("CheckHealth() Message = %q, want %q", status.Message, tt.expectedMsg)
			}
		})
	}
}

func TestCheckHealth_ConnectionError(t *testing.T) {
	// Use an invalid address that will cause a connection error
	prof := &types.Profile{
		Name:    "test",
		Address: "http://localhost:99999", // Invalid port
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	executor := NewHealthExecutor()
	status := executor.CheckHealth(ctx, prof)

	if status == nil {
		t.Fatal("CheckHealth() returned nil")
	}

	if status.Status != "error" {
		t.Errorf("CheckHealth() Status = %q, want %q", status.Status, "error")
	}

	if status.Message == "" {
		t.Error("CheckHealth() Message should not be empty on error")
	}
}

func TestCheckHealth_InvalidURL(t *testing.T) {
	// Use an invalid URL format
	prof := &types.Profile{
		Name:    "test",
		Address: "://invalid-url",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	executor := NewHealthExecutor()
	status := executor.CheckHealth(ctx, prof)

	if status == nil {
		t.Fatal("CheckHealth() returned nil")
	}

	if status.Status != "error" {
		t.Errorf("CheckHealth() Status = %q, want %q", status.Status, "error")
	}

	if status.Message == "" {
		t.Error("CheckHealth() Message should not be empty on error")
	}
}

func TestCheckHealth_ContextTimeout(t *testing.T) {
	// Create a server that never responds (or takes too long)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than the context timeout
		time.Sleep(2 * time.Second)
		w.WriteHeader(200)
	}))
	defer server.Close()

	prof := &types.Profile{
		Name:    "test",
		Address: server.URL,
	}

	// Use a very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	executor := NewHealthExecutor()
	status := executor.CheckHealth(ctx, prof)

	if status == nil {
		t.Fatal("CheckHealth() returned nil")
	}

	// Should return an error status due to timeout
	if status.Status != "error" {
		t.Errorf("CheckHealth() Status = %q, want %q", status.Status, "error")
	}
}
