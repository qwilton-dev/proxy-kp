package proxy

import (
	"context"
	"net"
	"net/http"
	"time"

	"proxy-kp/pkg/cache"
	"proxy-kp/pkg/logger"
	"proxy-kp/pkg/ratelimit"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Middleware struct {
	logger     *logger.Logger
	limiter    *ratelimit.Limiter
	cache      *cache.Cache
	cacheEnabled bool
}

func NewMiddleware(logger *logger.Logger, limiter *ratelimit.Limiter, cache *cache.Cache, cacheEnabled bool) *Middleware {
	return &Middleware{
		logger:       logger,
		limiter:      limiter,
		cache:        cache,
		cacheEnabled: cacheEnabled,
	}
}

func (m *Middleware) Chain(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		requestID := uuid.New().String()
		r = r.WithContext(contextWithRequestID(r.Context(), requestID))
		w.Header().Set("X-Request-Id", requestID)

		log := m.logger.WithRequestID(requestID)

		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		defer func() {
			if err := recover(); err != nil {
				log.Error("Panic recovered",
					zap.Any("error", err),
					zap.String("path", r.URL.Path))
				wrapped.WriteHeader(http.StatusInternalServerError)
				wrapped.Write([]byte("Internal Server Error"))
			}

			duration := time.Since(start)
			log.Info("Request completed",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", wrapped.status),
				zap.Duration("duration", duration))
		}()

		if m.limiter != nil {
			ip := getClientIP(r)
			if !m.limiter.Allow(ip) {
				log.Warn("Rate limit exceeded",
					zap.String("client_ip", ip),
					zap.String("path", r.URL.Path))
				wrapped.WriteHeader(http.StatusTooManyRequests)
				wrapped.Write([]byte("Rate limit exceeded"))
				return
			}
		}

		if m.cacheEnabled && r.Method == http.MethodGet {
			cacheKey := getCacheKey(r)
			if cachedData, headers, found := m.cache.Get(cacheKey); found {
				log.Debug("Cache hit",
					zap.String("key", cacheKey),
					zap.String("path", r.URL.Path))
				for key, values := range headers {
					for _, value := range values {
						wrapped.Header().Add(key, value)
					}
				}
				wrapped.Write(cachedData)
				return
			}
			log.Debug("Cache miss", zap.String("key", cacheKey))
		}

		next.ServeHTTP(wrapped, r)
	})
}

func getClientIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

type contextKey string

const requestIDKey contextKey = "requestID"

func contextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.status = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	return rw.ResponseWriter.Write(b)
}
