package balancer

import (
	"sync"
)

type Backend struct {
	URL           string
	Weight        int
	CurrentWeight int
	Healthy       bool
	mu            sync.RWMutex
}

func NewBackend(url string, weight int) *Backend {
	return &Backend{
		URL:           url,
		Weight:        weight,
		CurrentWeight: 0,
		Healthy:       true,
	}
}

func (b *Backend) SetHealthy(healthy bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Healthy = healthy
}

func (b *Backend) IsHealthy() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.Healthy
}
