package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"
	"time"

	"proxy-kp/internal/config"
	"proxy-kp/pkg/balancer"
	"proxy-kp/pkg/cache"
	"proxy-kp/pkg/health"
	"proxy-kp/pkg/logger"
	"proxy-kp/pkg/ratelimit"
	tlsconfig "proxy-kp/pkg/tls"

	"go.uber.org/zap"
)

type Server struct {
	config         *config.Config
	logger         *logger.Logger
	server         *http.Server
	tlsServer      *http.Server
	balancer       *balancer.SRR
	healthChecker  *health.Checker
	limiter        *ratelimit.Limiter
	cache          *cache.Cache
	cleanupManager *ratelimit.CleanupManager
	middleware     *Middleware
	handler        *Handler
}

func NewServer(cfg *config.Config, log *logger.Logger) (*Server, error) {
	b := balancer.NewSRR()

	for _, backendCfg := range cfg.Backends {
		backend := balancer.NewBackend(backendCfg.URL, backendCfg.Weight)
		b.AddBackend(backend)
		log.Info("Backend added",
			zap.String("url", backendCfg.URL),
			zap.Int("weight", backendCfg.Weight))
	}

	c := cache.NewCache(cfg.Cache.TTL)

	var limiter *ratelimit.Limiter
	if cfg.RateLimit.Enabled {
		limiter = ratelimit.NewLimiter(cfg.RateLimit.RequestsPerMinute, cfg.RateLimit.Burst)
	}

	h := &health.Checker{}
	if cfg.HealthCheck.Interval > 0 {
		h = health.NewChecker(
			b,
			cfg.HealthCheck.Interval,
			cfg.HealthCheck.Timeout,
			cfg.HealthCheck.Endpoint,
			cfg.HealthCheck.FailureThreshold,
			cfg.HealthCheck.RecoveryInterval,
			log.Zap(),
		)
	}

	handler := NewHandler(b, c, log, cfg.Cache.Enabled)
	middleware := NewMiddleware(log, limiter, c, cfg.Cache.Enabled)

	s := &Server{
		config:        cfg,
		logger:        log,
		balancer:      b,
		healthChecker: h,
		limiter:       limiter,
		cache:         c,
		handler:       handler,
		middleware:    middleware,
	}

	if limiter != nil {
		s.cleanupManager = ratelimit.NewCleanupManager(limiter, 5*time.Minute, 5*time.Minute)
	}

	return s, nil
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.middleware.Chain(s.handler).ServeHTTP)

	var tlsConfig *tls.Config
	if s.config.TLS.Enabled {
		cfg, err := tlsconfig.NewConfig(s.config.TLS.CertFile, s.config.TLS.KeyFile).Load()
		if err != nil {
			return err
		}
		tlsConfig = cfg
	}

	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.HTTPPort),
		Handler:      mux,
		ReadTimeout:  s.config.Server.ReadTimeout,
		WriteTimeout: s.config.Server.WriteTimeout,
	}

	if s.config.TLS.Enabled {
		s.tlsServer = &http.Server{
			Addr:         fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.HTTPSPort),
			Handler:      mux,
			TLSConfig:    tlsConfig,
			ReadTimeout:  s.config.Server.ReadTimeout,
			WriteTimeout: s.config.Server.WriteTimeout,
		}
	}

	s.healthChecker.Start(ctx)
	if s.cleanupManager != nil {
		s.cleanupManager.Start()
	}

	errCh := make(chan error, 2)

	go func() {
		s.logger.Info("Starting HTTP server",
			zap.String("address", s.server.Addr))
		if err := s.server.ListenAndServe(); err != nil {
			errCh <- fmt.Errorf("HTTP server error: %w", err)
		}
	}()

	if s.config.TLS.Enabled {
		go func() {
			s.logger.Info("Starting HTTPS server",
				zap.String("address", s.tlsServer.Addr))
			if err := s.tlsServer.ListenAndServeTLS("", ""); err != nil {
				errCh <- fmt.Errorf("HTTPS server error: %w", err)
			}
		}()
	}

	select {
	case <-ctx.Done():
		s.logger.Info("Shutting down servers")
		return s.Shutdown()
	case err := <-errCh:
		return err
	}
}

func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	if s.healthChecker != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.healthChecker.Stop()
		}()
	}

	if s.cleanupManager != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.cleanupManager.Stop()
		}()
	}

	if s.server != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.server.Shutdown(ctx)
		}()
	}

	if s.tlsServer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.tlsServer.Shutdown(ctx)
		}()
	}

	wg.Wait()
	return nil
}
