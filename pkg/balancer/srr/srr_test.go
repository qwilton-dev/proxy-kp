package srr

import (
	"sync"
	"testing"

	"proxy-kp/pkg/balancer"
)

func TestSRR_AddBackend(t *testing.T) {
	s := New()

	backend1 := balancer.NewBackend("http://localhost:8001", 10)
	backend2 := balancer.NewBackend("http://localhost:8002", 20)

	s.AddBackend(backend1)
	s.AddBackend(backend2)

	backends := s.GetBackends()
	if len(backends) != 2 {
		t.Errorf("Expected 2 backends, got %d", len(backends))
	}
}

func TestSRR_RemoveBackend(t *testing.T) {
	s := New()

	backend1 := balancer.NewBackend("http://localhost:8001", 10)
	backend2 := balancer.NewBackend("http://localhost:8002", 20)

	s.AddBackend(backend1)
	s.AddBackend(backend2)

	removed := s.RemoveBackend("http://localhost:8001")
	if !removed {
		t.Error("Expected backend to be removed")
	}

	backends := s.GetBackends()
	if len(backends) != 1 {
		t.Errorf("Expected 1 backend after removal, got %d", len(backends))
	}
}

func TestSRR_NextBackend_Distribution(t *testing.T) {
	s := New()

	backend1 := balancer.NewBackend("http://localhost:8001", 1)
	backend2 := balancer.NewBackend("http://localhost:8002", 2)
	backend3 := balancer.NewBackend("http://localhost:8003", 3)

	s.AddBackend(backend1)
	s.AddBackend(backend2)
	s.AddBackend(backend3)

	counts := make(map[string]int)
	iterations := 60

	for i := 0; i < iterations; i++ {
		backend, err := s.NextBackend()
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
	s := New()

	_, err := s.NextBackend()
	if err != balancer.ErrNoHealthyBackends {
		t.Errorf("Expected ErrNoHealthyBackends, got %v", err)
	}
}

func TestSRR_NextBackend_AllUnhealthy(t *testing.T) {
	s := New()

	backend1 := balancer.NewBackend("http://localhost:8001", 10)
	backend1.SetHealthy(false)

	backend2 := balancer.NewBackend("http://localhost:8002", 20)
	backend2.SetHealthy(false)

	s.AddBackend(backend1)
	s.AddBackend(backend2)

	_, err := s.NextBackend()
	if err != balancer.ErrNoHealthyBackends {
		t.Errorf("Expected ErrNoHealthyBackends, got %v", err)
	}
}

func TestSRR_SetHealthy(t *testing.T) {
	s := New()

	backend := balancer.NewBackend("http://localhost:8001", 10)
	s.AddBackend(backend)

	backend.SetHealthy(false)
	if backend.IsHealthy() {
		t.Error("Expected backend to be unhealthy")
	}

	s.SetHealthy("http://localhost:8001", true)
	if !backend.IsHealthy() {
		t.Error("Expected backend to be healthy")
	}
}

func TestSRR_ConcurrentAccess(t *testing.T) {
	s := New()

	var wg sync.WaitGroup
	iterations := 100

	for i := 0; i < iterations; i++ {
		wg.Add(2)

		go func() {
			defer wg.Done()
			backend := balancer.NewBackend("http://localhost:8001", 10)
			s.AddBackend(backend)
		}()

		go func() {
			defer wg.Done()
			s.NextBackend()
		}()
	}

	wg.Wait()

	backends := s.GetBackends()
	if len(backends) != iterations {
		t.Logf("Concurrent access test: %d backends added", len(backends))
	}
}

func TestSRR_HealthyCount(t *testing.T) {
	s := New()

	backend1 := balancer.NewBackend("http://localhost:8001", 10)
	backend2 := balancer.NewBackend("http://localhost:8002", 20)
	backend3 := balancer.NewBackend("http://localhost:8003", 30)

	s.AddBackend(backend1)
	s.AddBackend(backend2)
	s.AddBackend(backend3)

	backend2.SetHealthy(false)

	count := s.HealthyCount()
	if count != 2 {
		t.Errorf("Expected 2 healthy backends, got %d", count)
	}
}

func TestBackend_ThreadSafety(t *testing.T) {
	backend := balancer.NewBackend("http://localhost:8001", 10)

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
