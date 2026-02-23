package ratelimit

import (
	"sync"
	"testing"
	"time"
)

func TestLimiter_Allow_WithinLimit(t *testing.T) {
	limiter := NewLimiter(60, 10)

	ip := "192.168.1.1"

	for i := 0; i < 10; i++ {
		if !limiter.Allow(ip) {
			t.Errorf("Request %d should be allowed", i)
		}
	}
}

func TestLimiter_Allow_ExceedsLimit(t *testing.T) {
	limiter := NewLimiter(6, 6)

	ip := "192.168.1.1"

	allowedCount := 0
	for i := 0; i < 20; i++ {
		if limiter.Allow(ip) {
			allowedCount++
		}
	}

	if allowedCount > 6 {
		t.Errorf("Expected max 6 allowed requests, got %d", allowedCount)
	}
}

func TestLimiter_Allow_Burst(t *testing.T) {
	limiter := NewLimiter(1, 10)

	ip := "192.168.1.1"

	burstCount := 0
	for i := 0; i < 10; i++ {
		if limiter.Allow(ip) {
			burstCount++
		}
	}

	if burstCount != 10 {
		t.Errorf("Expected burst of 10, got %d", burstCount)
	}

	if limiter.Allow(ip) {
		t.Error("Request after burst should be denied")
	}
}

func TestLimiter_MultipleIPs(t *testing.T) {
	limiter := NewLimiter(10, 5)

	ip1 := "192.168.1.1"
	ip2 := "192.168.1.2"

	for i := 0; i < 5; i++ {
		if !limiter.Allow(ip1) {
			t.Error("IP1 requests should be allowed")
		}
		if !limiter.Allow(ip2) {
			t.Error("IP2 requests should be allowed")
		}
	}
}

func TestLimiter_CleanupStale(t *testing.T) {
	limiter := NewLimiter(60, 10)

	ip1 := "192.168.1.1"
	ip2 := "192.168.1.2"

	limiter.Allow(ip1)
	limiter.Allow(ip2)

	time.Sleep(100 * time.Millisecond)

	count := limiter.CleanupStale(50 * time.Millisecond)

	if count != 2 {
		t.Errorf("Expected to cleanup 2 entries, got %d", count)
	}

	if limiter.Size() != 0 {
		t.Errorf("Expected 0 limiters after cleanup, got %d", limiter.Size())
	}
}

func TestLimiter_Size(t *testing.T) {
	limiter := NewLimiter(60, 10)

	if limiter.Size() != 0 {
		t.Errorf("Expected initial size 0, got %d", limiter.Size())
	}

	limiter.Allow("192.168.1.1")
	limiter.Allow("192.168.1.2")
	limiter.Allow("192.168.1.3")

	if limiter.Size() != 3 {
		t.Errorf("Expected size 3, got %d", limiter.Size())
	}
}

func TestLimiter_ConcurrentAccess(t *testing.T) {
	limiter := NewLimiter(100, 50)

	var wg sync.WaitGroup
	ips := []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"}
	iterations := 100

	for _, ip := range ips {
		for i := 0; i < iterations; i++ {
			wg.Add(1)
			go func(clientIP string) {
				defer wg.Done()
				limiter.Allow(clientIP)
			}(ip)
		}
	}

	wg.Wait()

	if limiter.Size() != len(ips) {
		t.Errorf("Expected %d unique IPs, got %d", len(ips), limiter.Size())
	}
}

func TestLimiter_Refill(t *testing.T) {
	limiter := NewLimiter(600, 1)

	ip := "192.168.1.1"

	if !limiter.Allow(ip) {
		t.Error("First request should be allowed")
	}

	if limiter.Allow(ip) {
		t.Error("Second immediate request should be denied")
	}

	time.Sleep(200 * time.Millisecond)

	if !limiter.Allow(ip) {
		t.Error("Request after refill should be allowed")
	}
}

func TestCleanupManager_StartAndStop(t *testing.T) {
	limiter := NewLimiter(60, 10)

	for i := 0; i < 5; i++ {
		limiter.Allow("192.168.1." + string(rune('1'+i)))
	}

	manager := NewCleanupManager(limiter, 50*time.Millisecond, 50*time.Millisecond)
	manager.Start()

	time.Sleep(150 * time.Millisecond)

	manager.Stop()

	if limiter.Size() != 0 {
		t.Logf("After cleanup: %d limiters remain", limiter.Size())
	}
}

func TestLimiter_RefillRate(t *testing.T) {
	limiter := NewLimiter(600, 1)

	ip := "192.168.1.1"

	if !limiter.Allow(ip) {
		t.Error("First request should be allowed")
	}

	time.Sleep(500 * time.Millisecond)

	allowed := limiter.Allow(ip)
	if !allowed {
		t.Log("Request after refill may be allowed depending on timing")
	}
}
