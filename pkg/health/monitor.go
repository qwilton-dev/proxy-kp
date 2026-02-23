package health

import (
	"sync"
)

type Monitor struct {
	checker *Checker
	mu      sync.RWMutex
}

func NewMonitor(checker *Checker) *Monitor {
	return &Monitor{
		checker: checker,
	}
}

type BackendStatus struct {
	URL          string
	Healthy      bool
	FailureCount int
}

func (m *Monitor) GetStatus() []BackendStatus {
	backends := m.checker.balancer.GetBackends()
	status := make([]BackendStatus, 0, len(backends))

	for _, b := range backends {
		status = append(status, BackendStatus{
			URL:          b.URL,
			Healthy:      b.IsHealthy(),
			FailureCount: m.checker.GetFailureCount(b.URL),
		})
	}

	return status
}

func (m *Monitor) HealthyCount() int {
	return m.checker.balancer.HealthyCount()
}

func (m *Monitor) TotalCount() int {
	return len(m.checker.balancer.GetBackends())
}
