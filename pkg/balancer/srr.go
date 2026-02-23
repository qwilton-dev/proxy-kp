package balancer

import (
	"errors"
	"sync"
)

var ErrNoHealthyBackends = errors.New("no healthy backends available")

type SRR struct {
	backends []*Backend
	mu       sync.RWMutex
}

func NewSRR() *SRR {
	return &SRR{
		backends: make([]*Backend, 0),
	}
}

func (s *SRR) AddBackend(backend *Backend) {
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

func (s *SRR) GetBackends() []*Backend {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Backend, 0, len(s.backends))
	for _, b := range s.backends {
		result = append(result, b)
	}
	return result
}

func (s *SRR) NextBackend() (*Backend, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.backends) == 0 {
		return nil, ErrNoHealthyBackends
	}

	var best *Backend
	totalWeight := 0

	for _, b := range s.backends {
		if !b.IsHealthy() {
			continue
		}
		totalWeight += b.Weight
		b.CurrentWeight += b.Weight
	}

	if totalWeight == 0 {
		return nil, ErrNoHealthyBackends
	}

	for _, b := range s.backends {
		if !b.IsHealthy() {
			continue
		}

		if best == nil || b.CurrentWeight > best.CurrentWeight {
			best = b
		}
	}

	if best == nil {
		return nil, ErrNoHealthyBackends
	}

	best.CurrentWeight -= totalWeight

	return best, nil
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
