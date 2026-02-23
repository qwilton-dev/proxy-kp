package cache

import (
	"net/http"
	"sync"
	"time"
)

type Cache struct {
	entries map[string]*Entry
	mutex   sync.RWMutex
	ttl     time.Duration
}

func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		entries: make(map[string]*Entry),
		ttl:     ttl,
	}
}

func (c *Cache) Get(key string) ([]byte, http.Header, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return nil, nil, false
	}

	if entry.IsExpired() {
		return nil, nil, false
	}

	return entry.Value, entry.Header, true
}

func (c *Cache) Set(key string, value []byte, header http.Header) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	entry := NewEntry(key, value, header, c.ttl)
	c.entries[key] = entry
}

func (c *Cache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.entries, key)
}

func (c *Cache) CleanupExpired() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	count := 0
	now := time.Now()

	for key, entry := range c.entries {
		if now.After(entry.ExpiresAt) {
			delete(c.entries, key)
			count++
		}
	}

	return count
}

func (c *Cache) Size() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return len(c.entries)
}

func (c *Cache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.entries = make(map[string]*Entry)
}
