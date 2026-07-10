package leastconn

import (
	"math"
	"sync"

	"proxy-kp/pkg/balancer"
)

type LeastConn struct {
	backends []*balancer.Backend
	conns    map[string]int64
	mu       sync.RWMutex
}

func New() *LeastConn {
	return &LeastConn{
		backends: make([]*balancer.Backend, 0),
		conns:    make(map[string]int64),
	}
}

func (l *LeastConn) NextBackend() (*balancer.Backend, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if len(l.backends) == 0 {
		return nil, balancer.ErrNoHealthyBackends
	}

	var best *balancer.Backend
	bestScore := math.MaxFloat64

	for _, b := range l.backends {
		if !b.IsHealthy() {
			continue
		}
		score := float64(l.conns[b.URL]) / float64(b.Weight)
		if best == nil || score < bestScore {
			best = b
			bestScore = score
		}
	}

	if best == nil {
		return nil, balancer.ErrNoHealthyBackends
	}

	return best, nil
}

func (l *LeastConn) Acquire(url string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.conns[url]++
}

func (l *LeastConn) Release(url string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.conns[url] > 0 {
		l.conns[url]--
	}
}

func (l *LeastConn) AddBackend(backend *balancer.Backend) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.backends = append(l.backends, backend)
}

func (l *LeastConn) RemoveBackend(url string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	for i, b := range l.backends {
		if b.URL == url {
			l.backends = append(l.backends[:i], l.backends[i+1:]...)
			delete(l.conns, url)
			return true
		}
	}
	return false
}

func (l *LeastConn) SetHealthy(url string, healthy bool) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for _, b := range l.backends {
		if b.URL == url {
			b.SetHealthy(healthy)
			return true
		}
	}
	return false
}

func (l *LeastConn) GetBackends() []*balancer.Backend {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]*balancer.Backend, 0, len(l.backends))
	for _, b := range l.backends {
		result = append(result, b)
	}
	return result
}

func (l *LeastConn) HealthyCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	count := 0
	for _, b := range l.backends {
		if b.IsHealthy() {
			count++
		}
	}
	return count
}
