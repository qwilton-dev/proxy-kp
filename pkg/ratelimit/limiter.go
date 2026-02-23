package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Limiter struct {
	limiters map[string]*clientLimiter
	mutex    sync.RWMutex
	limit    rate.Limit
	burst    int
}

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func NewLimiter(requestsPerMinute int, burst int) *Limiter {
	reqPerSec := float64(requestsPerMinute) / 60.0

	return &Limiter{
		limiters: make(map[string]*clientLimiter),
		limit:    rate.Limit(reqPerSec),
		burst:    burst,
	}
}

func (r *Limiter) Allow(ip string) bool {
	r.mutex.RLock()
	limiter, exists := r.limiters[ip]
	r.mutex.RUnlock()

	if !exists {
		return r.createNewLimiter(ip)
	}

	r.mutex.Lock()
	limiter.lastSeen = time.Now()
	r.mutex.Unlock()

	return limiter.limiter.Allow()
}

func (r *Limiter) createNewLimiter(ip string) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if limiter, exists := r.limiters[ip]; exists {
		limiter.lastSeen = time.Now()
		return limiter.limiter.Allow()
	}

	limiter := &clientLimiter{
		limiter:  rate.NewLimiter(r.limit, r.burst),
		lastSeen: time.Now(),
	}
	r.limiters[ip] = limiter

	return limiter.limiter.Allow()
}

func (r *Limiter) getLimiter(ip string) *rate.Limiter {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if limiter, exists := r.limiters[ip]; exists {
		return limiter.limiter
	}

	return nil
}
