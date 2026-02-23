package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"proxy-kp/pkg/balancer"
	"proxy-kp/pkg/cache"
	"proxy-kp/pkg/logger"

	"go.uber.org/zap"
)

type Handler struct {
	balancer      *balancer.SRR
	cache         *cache.Cache
	logger        *logger.Logger
	cacheEnabled  bool
	client        *http.Client
}

func NewHandler(
	balancer *balancer.SRR,
	cache *cache.Cache,
	logger *logger.Logger,
	cacheEnabled bool,
) *Handler {
	return &Handler{
		balancer:     balancer,
		cache:        cache,
		logger:       logger,
		cacheEnabled: cacheEnabled,
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	backend, err := h.balancer.NextBackend()
	if err != nil {
		h.logger.Error("No healthy backends available",
			zap.String("path", r.URL.Path),
			zap.Error(err))
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}

	targetURL, err := url.Parse(backend.URL)
	if err != nil {
		h.logger.Error("Failed to parse backend URL",
			zap.String("backend", backend.URL),
			zap.Error(err))
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}

	// Construct full URL with path and query string
	proxyURL := targetURL.ResolveReference(&url.URL{
		Path:       r.URL.Path,
		RawPath:    r.URL.RawPath,
		RawQuery:   r.URL.RawQuery,
		Fragment:   r.URL.Fragment,
	})

	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, proxyURL.String(), r.Body)
	if err != nil {
		h.logger.Error("Failed to create proxy request",
			zap.String("backend", backend.URL),
			zap.Error(err))
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}

	copyHeader(proxyReq.Header, r.Header)

	h.setProxyHeaders(r, proxyReq, targetURL)

	log := h.logger.WithBackend(backend.URL)
	log.Info("Proxying request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.String("backend", backend.URL))

	start := time.Now()
	resp, err := h.client.Do(proxyReq)
	if err != nil {
		log.Error("Backend request failed",
			zap.String("path", r.URL.Path),
			zap.Error(err))
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	duration := time.Since(start)
	defer resp.Body.Close()

	log.Debug("Backend response received",
		zap.String("path", r.URL.Path),
		zap.Int("status", resp.StatusCode),
		zap.Duration("duration", duration))

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("Failed to read response body",
			zap.String("path", r.URL.Path),
			zap.Error(err))
		return
	}

	if h.cacheEnabled && r.Method == http.MethodGet && resp.StatusCode == http.StatusOK {
		cacheKey := getCacheKey(r)
		h.cache.Set(cacheKey, body, resp.Header)
		log.Debug("Response cached",
			zap.String("key", cacheKey),
			zap.Int("size", len(body)))
	}

	w.Write(body)
}

func (h *Handler) setProxyHeaders(originalReq *http.Request, proxyReq *http.Request, targetURL *url.URL) {
	proxyReq.Header.Set("X-Forwarded-For", getClientIP(originalReq))
	proxyReq.Header.Set("X-Forwarded-Host", originalReq.Host)
	proxyReq.Header.Set("X-Forwarded-Proto", getScheme(originalReq))

	if originalReq.Host != "" {
		proxyReq.Header.Set("X-Forwarded-Server", originalReq.Host)
	}

	if originalReq.URL.RawPath != "" {
		proxyReq.URL.RawPath = originalReq.URL.RawPath
	}
}

func getScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func getCacheKey(r *http.Request) string {
	return fmt.Sprintf("%s:%s", r.Method, r.URL.String())
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
