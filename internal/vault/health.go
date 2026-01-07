// Package vault provides Vault/OpenBao server interaction utilities.
package vault

import (
	"context"
	"fmt"
	"net/http"

	"github.com/xabinapal/patrol/internal/types"
)

// HealthExecutor provides an interface for executing Vault health checks.
type HealthExecutor interface {
	CheckHealth(ctx context.Context, prof *types.Profile) *HealthStatus
}

type healthExecutor struct{}

// NewHealthExecutor creates a new HealthExecutor.
func NewHealthExecutor() HealthExecutor {
	return &healthExecutor{}
}

// HealthStatus represents the health status of a Vault/OpenBao server.
type HealthStatus struct {
	Status  string
	Message string
}

func (e *healthExecutor) CheckHealth(ctx context.Context, prof *types.Profile) *HealthStatus {
	status := &HealthStatus{}

	client, err := buildHTTPClient(prof)
	if err != nil {
		status.Status = "error"
		status.Message = fmt.Sprintf("failed to create HTTP client: %v", err)
		return status
	}

	url := prof.Address + "/v1/sys/health"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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
