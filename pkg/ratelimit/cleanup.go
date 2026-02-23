package ratelimit

import (
	"sync"
	"time"
)

func (r *Limiter) CleanupStale(idleTimeout time.Duration) int {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	now := time.Now()
	count := 0

	for ip, limiter := range r.limiters {
		if now.Sub(limiter.lastSeen) > idleTimeout {
			delete(r.limiters, ip)
			count++
		}
	}

	return count
}

func (r *Limiter) StartCleanupWorker(interval time.Duration, idleTimeout time.Duration, stopCh <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			r.CleanupStale(idleTimeout)
		}
	}
}

func (r *Limiter) Size() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return len(r.limiters)
}

type CleanupManager struct {
	limiter       *Limiter
	interval      time.Duration
	idleTimeout   time.Duration
	stopCh        chan struct{}
	stopOnce      sync.Once
	wg            sync.WaitGroup
}

func NewCleanupManager(limiter *Limiter, interval time.Duration, idleTimeout time.Duration) *CleanupManager {
	return &CleanupManager{
		limiter:     limiter,
		interval:    interval,
		idleTimeout: idleTimeout,
		stopCh:      make(chan struct{}),
	}
}

func (m *CleanupManager) Start() {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.limiter.StartCleanupWorker(m.interval, m.idleTimeout, m.stopCh)
	}()
}

func (m *CleanupManager) Stop() {
	m.stopOnce.Do(func() {
		close(m.stopCh)
	})
	m.wg.Wait()
}
