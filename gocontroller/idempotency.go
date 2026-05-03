package gocontroller

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

const idempotencyHeader = "Idempotency-Key"

type idempotencyEntry struct {
	statusCode int
	headers    map[string][]string
	body       []byte
	createdAt  time.Time
}

type idempotencyResponseWriter struct {
	http.ResponseWriter
	statusCode  int
	body        bytes.Buffer
	wroteHeader bool
	headers     map[string][]string
}

func (w *idempotencyResponseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.statusCode = code
	w.headers = make(map[string][]string)
	for k, vals := range w.ResponseWriter.Header() {
		for _, v := range vals {
			w.headers[k] = append(w.headers[k], v)
		}
	}
}

func (w *idempotencyResponseWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(data)
	if err == nil {
		w.body.Write(data[:n])
	}
	return n, err
}

// IdempotencyStore stores idempotent request responses.
type IdempotencyStore struct {
	mu      sync.RWMutex
	entries map[string]*idempotencyEntry
	ttl     time.Duration
	maxSize int
}

// NewIdempotencyStore creates a new in-memory idempotency store.
func NewIdempotencyStore(ttl time.Duration, maxSize int) *IdempotencyStore {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	if maxSize <= 0 {
		maxSize = 10000
	}
	store := &IdempotencyStore{
		entries: make(map[string]*idempotencyEntry),
		ttl:     ttl,
		maxSize: maxSize,
	}
	go store.evictLoop()
	return store
}

func (s *IdempotencyStore) evictLoop() {
	ticker := time.NewTicker(s.ttl / 2)
	defer ticker.Stop()
	for range ticker.C {
		s.evict()
	}
}

func (s *IdempotencyStore) evict() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for key, entry := range s.entries {
		if now.Sub(entry.createdAt) > s.ttl {
			delete(s.entries, key)
		}
	}
}

func (s *IdempotencyStore) get(key string) *idempotencyEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.entries[key]
	if !ok {
		return nil
	}
	if time.Since(entry.createdAt) > s.ttl {
		return nil
	}
	return entry
}

func (s *IdempotencyStore) set(key string, entry *idempotencyEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.entries) >= s.maxSize {
		var oldestKey string
		var oldestTime time.Time
		for k, v := range s.entries {
			if oldestKey == "" || v.createdAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.createdAt
			}
		}
		if oldestKey != "" {
			delete(s.entries, oldestKey)
		}
	}
	s.entries[key] = entry
}

// IdempotencyConfig configures the idempotency middleware.
type IdempotencyConfig struct {
	Store        *IdempotencyStore
	HeaderName   string
	Methods      []string
	SkipOnHeader func(*Context) bool
}

func (c *IdempotencyConfig) applyDefaults() {
	if c.Store == nil {
		c.Store = NewIdempotencyStore(0, 0)
	}
	if c.HeaderName == "" {
		c.HeaderName = idempotencyHeader
	}
	if len(c.Methods) == 0 {
		c.Methods = []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}
	}
}

// Idempotency returns a middleware that caches responses for duplicate requests.
func Idempotency(cfg IdempotencyConfig) Middleware {
	cfg.applyDefaults()

	methodSet := make(map[string]struct{}, len(cfg.Methods))
	for _, m := range cfg.Methods {
		methodSet[m] = struct{}{}
	}

	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			if _, ok := methodSet[ctx.Request.Method]; !ok {
				return next(ctx)
			}

			if cfg.SkipOnHeader != nil && cfg.SkipOnHeader(ctx) {
				return next(ctx)
			}

			key := ctx.Request.Header.Get(cfg.HeaderName)
			if key == "" {
				return next(ctx)
			}

			cacheKey := makeCacheKey(ctx, key)

			if entry := cfg.Store.get(cacheKey); entry != nil {
				for k, vals := range entry.headers {
					for _, v := range vals {
						ctx.ResponseWriter.Header().Set(k, v)
					}
				}
				ctx.ResponseWriter.Header().Set("X-Idempotent-Replayed", "true")
				ctx.ResponseWriter.WriteHeader(entry.statusCode)
				_, _ = ctx.ResponseWriter.Write(entry.body)
				return nil
			}

			recorder := &idempotencyResponseWriter{
				ResponseWriter: ctx.ResponseWriter,
				statusCode:     http.StatusOK,
			}
			ctx.ResponseWriter = recorder

			err := next(ctx)

			entry := &idempotencyEntry{
				statusCode: recorder.statusCode,
				headers:    recorder.headers,
				body:       recorder.body.Bytes(),
				createdAt:  time.Now(),
			}
			if entry.headers == nil {
				entry.headers = make(map[string][]string)
			}

			cfg.Store.set(cacheKey, entry)

			return err
		}
	}
}

func makeCacheKey(ctx *Context, idempotencyKey string) string {
	h := sha256.New()
	h.Write([]byte(ctx.Request.Method))
	h.Write([]byte(ctx.Request.URL.Path))
	h.Write([]byte(ctx.Request.URL.RawQuery))
	h.Write([]byte(idempotencyKey))
	return hex.EncodeToString(h.Sum(nil))
}
