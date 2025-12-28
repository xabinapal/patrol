package notify

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/xabinapal/patrol/internal/config"
	"github.com/xabinapal/patrol/internal/utils"
)

func TestNotifyRenewal(t *testing.T) {
	mock := &mockBackend{}
	cfg := config.NotificationConfig{
		Enabled:   true,
		OnRenewal: true,
	}

	nt := New(cfg, WithBackend(mock))
	n, ok := nt.(*notifier)
	if !ok {
		t.Fatalf("expected notifier, got %T", nt)
	}

	profile := "test-profile"
	ttl := 2 * time.Hour
	err := n.NotifyRenewal(profile, ttl)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if len(mock.notifyCalls) != 1 {
		t.Fatalf("expected 1 notify call, got %d", len(mock.notifyCalls))
	}

	call := mock.notifyCalls[0]
	expectedTitle := "Patrol: Token Renewed"
	if call.title != expectedTitle {
		t.Errorf("expected title %q, got %q", expectedTitle, call.title)
	}

	expectedMessage := fmt.Sprintf("Token for '%s' renewed successfully.\nNew TTL: %s", profile, utils.FormatDuration(ttl))
	if call.message != expectedMessage {
		t.Errorf("expected message %q, got %q", expectedMessage, call.message)
	}

	if call.iconPath != "" {
		t.Errorf("expected empty iconPath, got %q", call.iconPath)
	}
}

func TestNotifyRenewalWithDisabledGlobal(t *testing.T) {
	mock := &mockBackend{}
	cfg := config.NotificationConfig{
		Enabled: false,
	}

	nt := New(cfg, WithBackend(mock))
	n, ok := nt.(*notifier)
	if !ok {
		t.Fatalf("expected notifier, got %T", nt)
	}

	err := n.NotifyRenewal("test-profile", time.Hour)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if len(mock.notifyCalls) != 0 {
		t.Errorf("expected no notify calls when disabled, got %d", len(mock.notifyCalls))
	}
}

func TestNotifyRenewalWithDisabledOnRenewal(t *testing.T) {
	mock := &mockBackend{}
	cfg := config.NotificationConfig{
		Enabled:   true,
		OnRenewal: false,
	}

	nt := New(cfg, WithBackend(mock))
	n, ok := nt.(*notifier)
	if !ok {
		t.Fatalf("expected notifier, got %T", nt)
	}

	err := n.NotifyRenewal("test-profile", time.Hour)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if len(mock.notifyCalls) != 0 {
		t.Errorf("expected no notify calls when renewal is disabled, got %d", len(mock.notifyCalls))
	}
}

func TestNotifyFailure(t *testing.T) {
	mock := &mockBackend{}
	cfg := config.NotificationConfig{
		Enabled:   true,
		OnFailure: true,
	}

	nt := New(cfg, WithBackend(mock))
	n, ok := nt.(*notifier)
	if !ok {
		t.Fatalf("expected notifier, got %T", nt)
	}

	profile := "test-profile"
	testErr := errors.New("connection timeout")
	err := n.NotifyFailure(profile, testErr)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if len(mock.alertCalls) != 1 {
		t.Fatalf("expected 1 alert call, got %d", len(mock.alertCalls))
	}

	call := mock.alertCalls[0]
	expectedTitle := "Patrol: Renewal Failed"
	if call.title != expectedTitle {
		t.Errorf("expected title %q, got %q", expectedTitle, call.title)
	}

	expectedMessage := fmt.Sprintf("Failed to renew token for '%s'.\nError: %v", profile, testErr)
	if call.message != expectedMessage {
		t.Errorf("expected message %q, got %q", expectedMessage, call.message)
	}

	if call.iconPath != "" {
		t.Errorf("expected empty iconPath, got %q", call.iconPath)
	}
}

func TestNotifyFailureWithDisabledGlobal(t *testing.T) {
	mock := &mockBackend{}
	cfg := config.NotificationConfig{
		Enabled: false,
	}

	nt := New(cfg, WithBackend(mock))
	n, ok := nt.(*notifier)
	if !ok {
		t.Fatalf("expected notifier, got %T", nt)
	}

	err := n.NotifyFailure("test-profile", errors.New("test error"))
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if len(mock.alertCalls) != 0 {
		t.Errorf("expected no alert calls when disabled, got %d", len(mock.alertCalls))
	}
}

func TestNotifyFailureWithDisabledOnFailure(t *testing.T) {
	mock := &mockBackend{}
	cfg := config.NotificationConfig{
		Enabled:   true,
		OnFailure: false,
	}

	nt := New(cfg, WithBackend(mock))
	n, ok := nt.(*notifier)
	if !ok {
		t.Fatalf("expected notifier, got %T", nt)
	}

	err := n.NotifyFailure("test-profile", errors.New("test error"))
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if len(mock.alertCalls) != 0 {
		t.Errorf("expected no alert calls when failure is disabled, got %d", len(mock.alertCalls))
	}
}

func TestNotifyBackendError(t *testing.T) {
	expectedErr := errors.New("backend error")
	mock := &mockBackend{
		notifyFunc: func(title, message, iconPath string) error {
			return expectedErr
		},
		alertFunc: func(title, message, iconPath string) error {
			return expectedErr
		},
	}

	cfg := config.NotificationConfig{
		Enabled:   true,
		OnRenewal: true,
		OnFailure: true,
	}

	nt := New(cfg, WithBackend(mock))
	n, ok := nt.(*notifier)
	if !ok {
		t.Fatalf("expected notifier, got %T", nt)
	}

	err := n.NotifyRenewal("test-profile", time.Hour)
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}

	err = n.NotifyFailure("test-profile", errors.New("test error"))
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}
