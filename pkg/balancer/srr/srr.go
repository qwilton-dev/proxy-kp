package srr

import (
	"sync"

	"proxy-kp/pkg/balancer"
)

type SRR struct {
	backends      []*balancer.Backend
	currentWeight map[string]int
	mu            sync.RWMutex
}

func New() *SRR {
	return &SRR{
		backends:      make([]*balancer.Backend, 0),
		currentWeight: make(map[string]int),
	}
}

func (s *SRR) NextBackend() (*balancer.Backend, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.backends) == 0 {
		return nil, balancer.ErrNoHealthyBackends
	}

	var best *balancer.Backend
	totalWeight := 0

	for _, b := range s.backends {
		if !b.IsHealthy() {
			continue
		}
		totalWeight += b.Weight
		s.currentWeight[b.URL] += b.Weight
	}

	if totalWeight == 0 {
		return nil, balancer.ErrNoHealthyBackends
	}

	for _, b := range s.backends {
		if !b.IsHealthy() {
			continue
		}
		if best == nil || s.currentWeight[b.URL] > s.currentWeight[best.URL] {
			best = b
		}
	}

	if best == nil {
		return nil, balancer.ErrNoHealthyBackends
	}

	s.currentWeight[best.URL] -= totalWeight

	return best, nil
}

func (s *SRR) Acquire(url string) {}

func (s *SRR) Release(url string) {}

func (s *SRR) AddBackend(backend *balancer.Backend) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.backends = append(s.backends, backend)
}

func (s *SRR) RemoveBackend(url string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, b := range s.backends {
		if b.URL == url {
			s.backends = append(s.backends[:i], s.backends[i+1:]...)
			delete(s.currentWeight, url)
			return true
		}
	}
	return false
}

func (s *SRR) SetHealthy(url string, healthy bool) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, b := range s.backends {
		if b.URL == url {
			b.SetHealthy(healthy)
			return true
		}
	}
	return false
}

func (s *SRR) GetBackends() []*balancer.Backend {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*balancer.Backend, 0, len(s.backends))
	for _, b := range s.backends {
		result = append(result, b)
	}
	return result
}

func (s *SRR) HealthyCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, b := range s.backends {
		if b.IsHealthy() {
			count++
		}
	}
	return count
}
