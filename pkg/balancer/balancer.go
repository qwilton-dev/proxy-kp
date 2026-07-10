package balancer

import "errors"

var ErrNoHealthyBackends = errors.New("no healthy backends available")

type Balancer interface {
	NextBackend() (*Backend, error)
	Acquire(url string)
	Release(url string)
	AddBackend(backend *Backend)
	RemoveBackend(url string) bool
	SetHealthy(url string, healthy bool) bool
	GetBackends() []*Backend
	HealthyCount() int
}
