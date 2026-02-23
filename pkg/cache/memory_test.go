package cache

import (
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestCache_SetAndGet(t *testing.T) {
	cache := NewCache(60 * time.Second)

	key := "test-key"
	value := []byte("test-value")
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	cache.Set(key, value, headers)

	retrieved, retrievedHeaders, found := cache.Get(key)
	if !found {
		t.Error("Expected to find value in cache")
	}

	if string(retrieved) != string(value) {
		t.Errorf("Expected %s, got %s", string(value), string(retrieved))
	}

	if retrievedHeaders.Get("Content-Type") != "application/json" {
		t.Error("Header not preserved")
	}
}

func TestCache_GetNotFound(t *testing.T) {
	cache := NewCache(60 * time.Second)

	_, _, found := cache.Get("non-existent")
	if found {
		t.Error("Expected not to find value")
	}
}

func TestCache_TTL_Expiration(t *testing.T) {
	cache := NewCache(10 * time.Millisecond)

	key := "test-key"
	value := []byte("test-value")
	headers := http.Header{}

	cache.Set(key, value, headers)

	time.Sleep(20 * time.Millisecond)

	_, _, found := cache.Get(key)
	if found {
		t.Error("Expected value to be expired")
	}
}

func TestCache_Delete(t *testing.T) {
	cache := NewCache(60 * time.Second)

	key := "test-key"
	value := []byte("test-value")
	headers := http.Header{}

	cache.Set(key, value, headers)

	cache.Delete(key)

	_, _, found := cache.Get(key)
	if found {
		t.Error("Expected value to be deleted")
	}
}

func TestCache_CleanupExpired(t *testing.T) {
	cache := NewCache(10 * time.Millisecond)

	key1 := "key1"
	key2 := "key2"

	cache.Set(key1, []byte("value1"), http.Header{})
	cache.Set(key2, []byte("value2"), http.Header{})

	time.Sleep(20 * time.Millisecond)

	count := cache.CleanupExpired()
	if count != 2 {
		t.Errorf("Expected to cleanup 2 entries, got %d", count)
	}

	if cache.Size() != 0 {
		t.Errorf("Expected cache size 0, got %d", cache.Size())
	}
}

func TestCache_Clear(t *testing.T) {
	cache := NewCache(60 * time.Second)

	cache.Set("key1", []byte("value1"), http.Header{})
	cache.Set("key2", []byte("value2"), http.Header{})

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("Expected cache size 0 after clear, got %d", cache.Size())
	}
}

func TestCache_Size(t *testing.T) {
	cache := NewCache(60 * time.Second)

	if cache.Size() != 0 {
		t.Errorf("Expected initial size 0, got %d", cache.Size())
	}

	cache.Set("key1", []byte("value1"), http.Header{})
	cache.Set("key2", []byte("value2"), http.Header{})
	cache.Set("key3", []byte("value3"), http.Header{})

	if cache.Size() != 3 {
		t.Errorf("Expected size 3, got %d", cache.Size())
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	cache := NewCache(60 * time.Second)

	var wg sync.WaitGroup
	iterations := 100

	for i := 0; i < iterations; i++ {
		wg.Add(3)

		go func(n int) {
			defer wg.Done()
			key := "key"
			cache.Set(key, []byte(string(rune(n))), http.Header{})
		}(i)

		go func() {
			defer wg.Done()
			cache.Get("key")
		}()

		go func() {
			defer wg.Done()
			cache.Size()
		}()
	}

	wg.Wait()
}

func TestEntry_IsExpired(t *testing.T) {
	entry := NewEntry("key", []byte("value"), http.Header{}, 10*time.Millisecond)

	if entry.IsExpired() {
		t.Error("Entry should not be expired immediately")
	}

	time.Sleep(20 * time.Millisecond)

	if !entry.IsExpired() {
		t.Error("Entry should be expired after TTL")
	}
}

func TestCache_UpdateExisting(t *testing.T) {
	cache := NewCache(60 * time.Second)

	key := "key"
	value1 := []byte("value1")
	value2 := []byte("value2")

	cache.Set(key, value1, http.Header{})
	cache.Set(key, value2, http.Header{})

	retrieved, _, found := cache.Get(key)
	if !found {
		t.Fatal("Expected to find value")
	}

	if string(retrieved) != string(value2) {
		t.Errorf("Expected %s, got %s", string(value2), string(retrieved))
	}

	if cache.Size() != 1 {
		t.Errorf("Expected size 1, got %d", cache.Size())
	}
}

func TestCache_MultipleKeys(t *testing.T) {
	cache := NewCache(60 * time.Second)

	data := map[string][]byte{
		"key1": []byte("value1"),
		"key2": []byte("value2"),
		"key3": []byte("value3"),
	}

	for key, value := range data {
		cache.Set(key, value, http.Header{})
	}

	if cache.Size() != len(data) {
		t.Errorf("Expected size %d, got %d", len(data), cache.Size())
	}

	for key, expectedValue := range data {
		retrieved, _, found := cache.Get(key)
		if !found {
			t.Errorf("Key %s not found", key)
			continue
		}

		if string(retrieved) != string(expectedValue) {
			t.Errorf("Key %s: expected %s, got %s", key, string(expectedValue), string(retrieved))
		}
	}
}
