package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"proxy-kp/pkg/balancer"

	"go.uber.org/zap"
)

func TestChecker_NewChecker(t *testing.T) {
	b := balancer.NewSRR()
	logger := zap.NewNop()

	checker := NewChecker(b, 5*time.Second, 2*time.Second, "/healthz", 3, 15*time.Second, logger)

	if checker == nil {
		t.Error("Expected checker to be created")
	}
}

func TestChecker_BackendHealthy(t *testing.T) {
	b := balancer.NewSRR()
	logger := zap.NewNop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	backend := balancer.NewBackend(server.URL, 10)
	b.AddBackend(backend)

	checker := NewChecker(b, 100*time.Millisecond, 2*time.Second, "/healthz", 3, 15*time.Second, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	checker.Start(ctx)
	time.Sleep(200 * time.Millisecond)

	if !backend.IsHealthy() {
		t.Error("Backend should remain healthy")
	}

	checker.Stop()
}

func TestChecker_Stop(t *testing.T) {
	b := balancer.NewSRR()
	logger := zap.NewNop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	backend := balancer.NewBackend(server.URL, 10)
	b.AddBackend(backend)

	checker := NewChecker(b, 100*time.Millisecond, 2*time.Second, "/healthz", 3, 15*time.Second, logger)

	ctx, cancel := context.WithCancel(context.Background())

	checker.Start(ctx)
	time.Sleep(200 * time.Millisecond)

	cancel()
	checker.Stop()

	if !backend.IsHealthy() {
		t.Error("Backend should still be healthy after stop")
	}
}
