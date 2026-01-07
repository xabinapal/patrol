// Package vault provides Vault/OpenBao server interaction utilities.
package vault

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/xabinapal/patrol/internal/types"
)

// buildHTTPClient creates an HTTP client with TLS configuration from the profile.
func buildHTTPClient(prof *types.Profile) (*http.Client, error) {
	tlsConfig := &tls.Config{}

	// Configure TLS skip verify
	if prof.TLSSkipVerify {
		tlsConfig.InsecureSkipVerify = true
	}

	// Load CA certificate if provided
	if prof.CACert != "" {
		caCert, err := os.ReadFile(prof.CACert)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, errors.New("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	// Load CA certificates from directory if provided
	if prof.CAPath != "" {
		files, err := os.ReadDir(prof.CAPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA path: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if tlsConfig.RootCAs != nil {
			caCertPool = tlsConfig.RootCAs
		}
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			caCert, err := os.ReadFile(filepath.Join(prof.CAPath, file.Name()))
			if err != nil {
				continue
			}
			caCertPool.AppendCertsFromPEM(caCert)
		}
		tlsConfig.RootCAs = caCertPool
	}

	// Load client certificate and key if provided
	if prof.ClientCert != "" && prof.ClientKey != "" {
		cert, err := tls.LoadX509KeyPair(prof.ClientCert, prof.ClientKey)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}, nil
}
