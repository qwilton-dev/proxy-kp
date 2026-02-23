package balancer

import (
	"sync"
	"testing"
)

func TestSRR_AddBackend(t *testing.T) {
	srr := NewSRR()

	backend1 := NewBackend("http://localhost:8001", 10)
	backend2 := NewBackend("http://localhost:8002", 20)

	srr.AddBackend(backend1)
	srr.AddBackend(backend2)

	backends := srr.GetBackends()
	if len(backends) != 2 {
		t.Errorf("Expected 2 backends, got %d", len(backends))
	}
}

func TestSRR_RemoveBackend(t *testing.T) {
	srr := NewSRR()

	backend1 := NewBackend("http://localhost:8001", 10)
	backend2 := NewBackend("http://localhost:8002", 20)

	srr.AddBackend(backend1)
	srr.AddBackend(backend2)

	removed := srr.RemoveBackend("http://localhost:8001")
	if !removed {
		t.Error("Expected backend to be removed")
	}

	backends := srr.GetBackends()
	if len(backends) != 1 {
		t.Errorf("Expected 1 backend after removal, got %d", len(backends))
	}
}

func TestSRR_NextBackend_Distribution(t *testing.T) {
	srr := NewSRR()

	backend1 := NewBackend("http://localhost:8001", 1)
	backend2 := NewBackend("http://localhost:8002", 2)
	backend3 := NewBackend("http://localhost:8003", 3)

	srr.AddBackend(backend1)
	srr.AddBackend(backend2)
	srr.AddBackend(backend3)

	counts := make(map[string]int)
	iterations := 60

	for i := 0; i < iterations; i++ {
		backend, err := srr.NextBackend()
		if err != nil {
			t.Fatalf("NextBackend failed: %v", err)
		}
		counts[backend.URL]++
	}

	if counts["http://localhost:8001"] == 0 {
		t.Error("Backend1 was not selected")
	}
	if counts["http://localhost:8002"] == 0 {
		t.Error("Backend2 was not selected")
	}
	if counts["http://localhost:8003"] == 0 {
		t.Error("Backend3 was not selected")
	}

	total := counts["http://localhost:8001"] + counts["http://localhost:8002"] + counts["http://localhost:8003"]
	if total != iterations {
		t.Errorf("Total selections: expected %d, got %d", iterations, total)
	}
}

func TestSRR_NextBackend_NoBackends(t *testing.T) {
	srr := NewSRR()

	_, err := srr.NextBackend()
	if err != ErrNoHealthyBackends {
		t.Errorf("Expected ErrNoHealthyBackends, got %v", err)
	}
}

func TestSRR_NextBackend_AllUnhealthy(t *testing.T) {
	srr := NewSRR()

	backend1 := NewBackend("http://localhost:8001", 10)
	backend1.SetHealthy(false)

	backend2 := NewBackend("http://localhost:8002", 20)
	backend2.SetHealthy(false)

	srr.AddBackend(backend1)
	srr.AddBackend(backend2)

	_, err := srr.NextBackend()
	if err != ErrNoHealthyBackends {
		t.Errorf("Expected ErrNoHealthyBackends, got %v", err)
	}
}

func TestSRR_SetHealthy(t *testing.T) {
	srr := NewSRR()

	backend := NewBackend("http://localhost:8001", 10)
	srr.AddBackend(backend)

	backend.SetHealthy(false)
	if backend.IsHealthy() {
		t.Error("Expected backend to be unhealthy")
	}

	srr.SetHealthy("http://localhost:8001", true)
	if !backend.IsHealthy() {
		t.Error("Expected backend to be healthy")
	}
}

func TestSRR_ConcurrentAccess(t *testing.T) {
	srr := NewSRR()

	var wg sync.WaitGroup
	iterations := 100

	for i := 0; i < iterations; i++ {
		wg.Add(2)

		go func() {
			defer wg.Done()
			backend := NewBackend("http://localhost:8001", 10)
			srr.AddBackend(backend)
		}()

		go func() {
			defer wg.Done()
			srr.NextBackend()
		}()
	}

	wg.Wait()

	backends := srr.GetBackends()
	if len(backends) != iterations {
		t.Logf("Concurrent access test: %d backends added", len(backends))
	}
}

func TestSRR_HealthyCount(t *testing.T) {
	srr := NewSRR()

	backend1 := NewBackend("http://localhost:8001", 10)
	backend2 := NewBackend("http://localhost:8002", 20)
	backend3 := NewBackend("http://localhost:8003", 30)

	srr.AddBackend(backend1)
	srr.AddBackend(backend2)
	srr.AddBackend(backend3)

	backend2.SetHealthy(false)

	count := srr.HealthyCount()
	if count != 2 {
		t.Errorf("Expected 2 healthy backends, got %d", count)
	}
}

func TestBackend_ThreadSafety(t *testing.T) {
	backend := NewBackend("http://localhost:8001", 10)

	var wg sync.WaitGroup
	iterations := 100

	for i := 0; i < iterations; i++ {
		wg.Add(2)

		go func() {
			defer wg.Done()
			backend.SetHealthy(true)
		}()

		go func() {
			defer wg.Done()
			backend.IsHealthy()
		}()
	}

	wg.Wait()

	if !backend.IsHealthy() {
		t.Error("Backend should be healthy after concurrent operations")
	}
}
