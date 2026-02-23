package cache

import (
	"net/http"
	"time"
)

type Entry struct {
	Key       string
	Value     []byte
	Header    http.Header
	ExpiresAt time.Time
	CreatedAt time.Time
}

func NewEntry(key string, value []byte, header http.Header, ttl time.Duration) *Entry {
	now := time.Now()
	return &Entry{
		Key:       key,
		Value:     value,
		Header:    header,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}
}

func (e *Entry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}
