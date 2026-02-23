package health

import (
	"context"
	"net/http"
	"sync"
	"time"

	"proxy-kp/pkg/balancer"

	"go.uber.org/zap"
)

type Checker struct {
	balancer          *balancer.SRR
	interval          time.Duration
	timeout           time.Duration
	endpoint          string
	failureThreshold  int
	recoveryInterval  time.Duration
	client            *http.Client
	logger            *zap.Logger
	mu                sync.RWMutex
	failures         map[string]int
	lastCheck        map[string]time.Time
	stopCh           chan struct{}
	stopOnce         sync.Once
	wg               sync.WaitGroup
}

func NewChecker(
	b *balancer.SRR,
	interval time.Duration,
	timeout time.Duration,
	endpoint string,
	failureThreshold int,
	recoveryInterval time.Duration,
	logger *zap.Logger,
) *Checker {
	return &Checker{
		balancer:         b,
		interval:         interval,
		timeout:          timeout,
		endpoint:         endpoint,
		failureThreshold: failureThreshold,
		recoveryInterval: recoveryInterval,
		client: &http.Client{
			Timeout: timeout,
		},
		logger:    logger,
		failures:  make(map[string]int),
		lastCheck: make(map[string]time.Time),
		stopCh:    make(chan struct{}),
	}
}

func (c *Checker) Start(ctx context.Context) {
	c.wg.Add(1)
	go c.run(ctx)
}

func (c *Checker) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	c.wg.Wait()
}

func (c *Checker) run(ctx context.Context) {
	defer c.wg.Done()

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.checkAllBackends()
		}
	}
}

func (c *Checker) checkAllBackends() {
	backends := c.balancer.GetBackends()

	for _, backend := range backends {
		go c.checkBackend(backend)
	}
}

func (c *Checker) checkBackend(backend *balancer.Backend) {
	var wasHealthy bool
	var lastCheck time.Time

	c.mu.Lock()
	wasHealthy = backend.IsHealthy()
	if !wasHealthy {
		lastCheck = c.lastCheck[backend.URL]
	}
	c.mu.Unlock()

	if !wasHealthy && time.Since(lastCheck) < c.recoveryInterval {
		return
	}

	url := backend.URL + c.endpoint
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		c.handleFailure(backend)
		return
	}

	start := time.Now()
	resp, err := c.client.Do(req)
	duration := time.Since(start)

	if err != nil {
		c.logger.Warn("Backend health check failed",
			zap.String("backend", backend.URL),
			zap.Error(err),
			zap.Duration("duration", duration))
		c.handleFailure(backend)
		return
	}
	defer resp.Body.Close()

	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastCheck[backend.URL] = time.Now()

	if resp.StatusCode == http.StatusOK {
		c.handleSuccess(backend)
		c.logger.Debug("Backend health check passed",
			zap.String("backend", backend.URL),
			zap.Duration("duration", duration))
	} else {
		c.logger.Warn("Backend health check failed",
			zap.String("backend", backend.URL),
			zap.Int("status_code", resp.StatusCode),
			zap.Duration("duration", duration))
		c.handleFailure(backend)
	}
}

func (c *Checker) handleFailure(backend *balancer.Backend) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.failures[backend.URL]++

	if c.failures[backend.URL] >= c.failureThreshold {
		if backend.IsHealthy() {
			backend.SetHealthy(false)
			c.logger.Error("Backend marked unhealthy",
				zap.String("backend", backend.URL),
				zap.Int("failures", c.failures[backend.URL]))
		}
	}
}

func (c *Checker) handleSuccess(backend *balancer.Backend) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.failures[backend.URL] > 0 {
		c.failures[backend.URL] = 0
	}

	if !backend.IsHealthy() {
		backend.SetHealthy(true)
		c.logger.Info("Backend recovered and marked healthy",
			zap.String("backend", backend.URL))
	}
}

func (c *Checker) GetFailureCount(url string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.failures[url]
}
