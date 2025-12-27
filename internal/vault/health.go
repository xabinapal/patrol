// Package vault provides Vault/OpenBao server interaction utilities.
package vault

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/xabinapal/patrol/internal/config"
)

// HealthStatus represents the health status of a Vault/OpenBao server.
type HealthStatus struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// CheckHealth checks the health of a Vault/OpenBao server.
func CheckHealth(ctx context.Context, conn *config.Connection) *HealthStatus {
	testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	status := &HealthStatus{}

	// Test HTTP connectivity
	client := &http.Client{Timeout: 5 * time.Second}
	healthURL := conn.Address + "/v1/sys/health"

	req, err := http.NewRequestWithContext(testCtx, "GET", healthURL, nil)
	if err != nil {
		status.Status = "error"
		status.Message = fmt.Sprintf("invalid URL: %v", err)
		return status
	}

	resp, err := client.Do(req)
	if err != nil {
		status.Status = "error"
		status.Message = fmt.Sprintf("connection failed: %v", err)
		return status
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		status.Status = "healthy"
		status.Message = "initialized, unsealed, active"
	case 429, 472, 473:
		status.Status = "standby"
		status.Message = "standby node"
	case 501:
		status.Status = "uninitialized"
		status.Message = "server is not initialized"
	case 503:
		status.Status = "sealed"
		status.Message = "server is sealed"
	default:
		status.Status = "unknown"
		status.Message = fmt.Sprintf("unexpected status: %d", resp.StatusCode)
	}

	return status
}
