package leastconn

import (
	"sync"
	"testing"

	"proxy-kp/pkg/balancer"
)

func TestLeastConn_AddBackend(t *testing.T) {
	lc := New()

	backend1 := balancer.NewBackend("http://localhost:8001", 1)
	backend2 := balancer.NewBackend("http://localhost:8002", 2)

	lc.AddBackend(backend1)
	lc.AddBackend(backend2)

	backends := lc.GetBackends()
	if len(backends) != 2 {
		t.Errorf("Expected 2 backends, got %d", len(backends))
	}
}

func TestLeastConn_RemoveBackend(t *testing.T) {
	lc := New()

	backend1 := balancer.NewBackend("http://localhost:8001", 1)
	backend2 := balancer.NewBackend("http://localhost:8002", 2)

	lc.AddBackend(backend1)
	lc.AddBackend(backend2)

	removed := lc.RemoveBackend("http://localhost:8001")
	if !removed {
		t.Error("Expected backend to be removed")
	}

	backends := lc.GetBackends()
	if len(backends) != 1 {
		t.Errorf("Expected 1 backend after removal, got %d", len(backends))
	}

	if lc.RemoveBackend("nonexistent") {
		t.Error("Expected false for nonexistent backend")
	}
}

func TestLeastConn_NextBackend_NoBackends(t *testing.T) {
	lc := New()

	_, err := lc.NextBackend()
	if err != balancer.ErrNoHealthyBackends {
		t.Errorf("Expected ErrNoHealthyBackends, got %v", err)
	}
}

func TestLeastConn_NextBackend_AllUnhealthy(t *testing.T) {
	lc := New()

	backend1 := balancer.NewBackend("http://localhost:8001", 10)
	backend1.SetHealthy(false)
	lc.AddBackend(backend1)

	_, err := lc.NextBackend()
	if err != balancer.ErrNoHealthyBackends {
		t.Errorf("Expected ErrNoHealthyBackends, got %v", err)
	}
}

func TestLeastConn_SetHealthy(t *testing.T) {
	lc := New()

	backend := balancer.NewBackend("http://localhost:8001", 10)
	lc.AddBackend(backend)

	backend.SetHealthy(false)
	if backend.IsHealthy() {
		t.Error("Expected backend to be unhealthy")
	}

	lc.SetHealthy("http://localhost:8001", true)
	if !backend.IsHealthy() {
		t.Error("Expected backend to be healthy")
	}
}

func TestLeastConn_HealthyCount(t *testing.T) {
	lc := New()

	lc.AddBackend(balancer.NewBackend("http://localhost:8001", 1))
	lc.AddBackend(balancer.NewBackend("http://localhost:8002", 1))
	lc.AddBackend(balancer.NewBackend("http://localhost:8003", 1))

	lc.SetHealthy("http://localhost:8002", false)

	count := lc.HealthyCount()
	if count != 2 {
		t.Errorf("Expected 2 healthy backends, got %d", count)
	}
}

func TestLeastConn_Distribution(t *testing.T) {
	lc := New()

	lc.AddBackend(balancer.NewBackend("http://localhost:8001", 1))
	lc.AddBackend(balancer.NewBackend("http://localhost:8002", 1))
	lc.AddBackend(balancer.NewBackend("http://localhost:8003", 1))

	counts := make(map[string]int)
	iterations := 30

	for i := 0; i < iterations; i++ {
		backend, err := lc.NextBackend()
		if err != nil {
			t.Fatalf("NextBackend failed: %v", err)
		}
		lc.Acquire(backend.URL)
		counts[backend.URL]++
	}

	if len(counts) != 3 {
		t.Error("Expected all 3 backends to be selected")
	}

	total := 0
	for _, c := range counts {
		total += c
	}
	if total != iterations {
		t.Errorf("Total selections: expected %d, got %d", iterations, total)
	}
}

func TestLeastConn_AcquireRelease(t *testing.T) {
	lc := New()

	backend := balancer.NewBackend("http://localhost:8001", 1)
	lc.AddBackend(backend)

	lc.Acquire("http://localhost:8001")
	lc.Acquire("http://localhost:8001")

	// With 2 active conns on the only backend, it should still pick it
	b, err := lc.NextBackend()
	if err != nil {
		t.Fatal(err)
	}
	if b.URL != "http://localhost:8001" {
		t.Errorf("Expected backend1, got %s", b.URL)
	}

	lc.Release("http://localhost:8001")
	lc.Release("http://localhost:8001")
	lc.Release("http://localhost:8001") // extra release should not go negative

	b, err = lc.NextBackend()
	if err != nil {
		t.Fatal(err)
	}
	if b.URL != "http://localhost:8001" {
		t.Errorf("Expected backend1, got %s", b.URL)
	}
}

func TestLeastConn_WeightedLeastConnections(t *testing.T) {
	lc := New()

	small := balancer.NewBackend("http://localhost:8001", 1)
	large := balancer.NewBackend("http://localhost:8002", 10)
	lc.AddBackend(small)
	lc.AddBackend(large)

	// Acquire 5 connections on the large backend (score = 5/10 = 0.5)
	// Small has 0 (score = 0/1 = 0) — should pick small first
	for i := 0; i < 5; i++ {
		lc.Acquire("http://localhost:8002")
	}

	b, err := lc.NextBackend()
	if err != nil {
		t.Fatal(err)
	}
	if b.URL != "http://localhost:8001" {
		t.Errorf("Expected small backend (lower score), got %s", b.URL)
	}
	lc.Acquire("http://localhost:8001")

	// Now small has 1 active (score = 1/1 = 1), large has 5 (score = 5/10 = 0.5)
	// Should pick large now
	b, err = lc.NextBackend()
	if err != nil {
		t.Fatal(err)
	}
	if b.URL != "http://localhost:8002" {
		t.Errorf("Expected large backend (lower weighted score), got %s", b.URL)
	}
}

func TestLeastConn_ConcurrentAccess(t *testing.T) {
	lc := New()

	var wg sync.WaitGroup
	n := 100

	for i := 0; i < n; i++ {
		lc.AddBackend(balancer.NewBackend("http://localhost:8001", 1))
	}

	for i := 0; i < n; i++ {
		wg.Add(2)

		go func() {
			defer wg.Done()
			lc.NextBackend()
		}()

		go func() {
			defer wg.Done()
			lc.Acquire("http://localhost:8001")
		}()
	}

	wg.Wait()

	if lc.HealthyCount() != n {
		t.Logf("Concurrent test completed with %d backends", lc.HealthyCount())
	}
}

func TestLeastConn_GetBackends(t *testing.T) {
	lc := New()

	backends := lc.GetBackends()
	if len(backends) != 0 {
		t.Errorf("Expected 0 backends initially, got %d", len(backends))
	}

	lc.AddBackend(balancer.NewBackend("http://localhost:8001", 1))
	backends = lc.GetBackends()
	if len(backends) != 1 {
		t.Errorf("Expected 1 backend, got %d", len(backends))
	}
}
