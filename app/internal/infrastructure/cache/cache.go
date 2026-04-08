package infrastructure

import (
	"sync"
	"time"

	"replikator/internal/metrics"
)

type CacheEntry struct {
	Timestamp time.Time
	Expiry    time.Duration
}

func (e *CacheEntry) IsExpired() bool {
	if e.Expiry == 0 {
		return false
	}
	return time.Since(e.Timestamp) > e.Expiry
}

type Cache struct {
	mu        sync.RWMutex
	entries   map[string]*CacheEntry
	defExpiry time.Duration
}

func NewCache(defaultExpiry time.Duration) *Cache {
	return &Cache{
		entries:   make(map[string]*CacheEntry),
		defExpiry: defaultExpiry,
	}
}

func (c *Cache) Get(key string) (bool, *CacheEntry) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return false, nil
	}

	if entry.IsExpired() {
		return false, nil
	}

	return true, entry
}

func (c *Cache) Set(key string, expiry time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = &CacheEntry{
		Timestamp: time.Now(),
		Expiry:    expiry,
	}
}

func (c *Cache) SetWithExpiry(key string, expiry time.Duration) {
	c.Set(key, expiry)
}

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)
}

func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
}

func (c *Cache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, entry := range c.entries {
		if entry.IsExpired() {
			delete(c.entries, key)
		}
	}
}

type NotFoundCache struct {
	*Cache
}

func NewNotFoundCache(expiry time.Duration) *NotFoundCache {
	return &NotFoundCache{
		Cache: NewCache(expiry),
	}
}

func (c *NotFoundCache) MarkNotFound(key string) {
	c.Set(key, c.defExpiry)
	metrics.RecordCacheMiss("not_found")
}

func (c *NotFoundCache) IsNotFound(key string) bool {
	exists, _ := c.Get(key)
	if exists {
		metrics.RecordCacheHit("not_found")
	}
	return exists
}

func (c *NotFoundCache) MarkFound(key string) {
	c.Delete(key)
	metrics.RecordCacheHit("not_found")
}

type MirrorCache struct {
	mu     sync.RWMutex
	mirror map[string]*MirrorEntry
}

type MirrorEntry struct {
	MirrorID   string
	SourceID   string
	IsAuto     bool
	CreatedAt  time.Time
	LastSyncAt time.Time
}

func NewMirrorCache() *MirrorCache {
	return &MirrorCache{
		mirror: make(map[string]*MirrorEntry),
	}
}

func (c *MirrorCache) Get(key string) (*MirrorEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.mirror[key]
	return entry, ok
}

func (c *MirrorCache) Set(key string, entry *MirrorEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.mirror[key] = entry
}

func (c *MirrorCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.mirror, key)
}

func (c *MirrorCache) ListBySource(sourceID string) []*MirrorEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*MirrorEntry, 0)
	for _, entry := range c.mirror {
		if entry.SourceID == sourceID {
			result = append(result, entry)
		}
	}
	return result
}

func (c *MirrorCache) DeleteBySource(sourceID string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	deleted := make([]string, 0)
	for key, entry := range c.mirror {
		if entry.SourceID == sourceID {
			delete(c.mirror, key)
			deleted = append(deleted, key)
		}
	}
	return deleted
}

func (c *MirrorCache) DeleteByNamespace(namespace string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	deleted := make([]string, 0)
	for key := range c.mirror {
		if isMirrorInNamespace(key, namespace) {
			delete(c.mirror, key)
			deleted = append(deleted, key)
		}
	}
	return deleted
}

func isMirrorInNamespace(key, namespace string) bool {
	for i := len(key) - 1; i >= 0; i-- {
		if key[i] == '/' {
			return key[:i] == namespace
		}
	}
	return false
}

func (c *MirrorCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.mirror = make(map[string]*MirrorEntry)
}

type SourceProperties struct {
	Allowed           bool
	AllowedNamespaces []string
	AutoEnabled       bool
	AutoNamespaces    []string
	Version           string
}

type PropertiesCache struct {
	mu       sync.RWMutex
	props    map[string]*SourceProperties
	notFound *NotFoundCache
}

func NewPropertiesCache(notFoundExpiry time.Duration) *PropertiesCache {
	return &PropertiesCache{
		props:    make(map[string]*SourceProperties),
		notFound: NewNotFoundCache(notFoundExpiry),
	}
}

func (c *PropertiesCache) Get(key string) (*SourceProperties, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	props, ok := c.props[key]
	if ok {
		metrics.RecordCacheHit("properties")
	} else {
		metrics.RecordCacheMiss("properties")
	}
	return props, ok
}

func (c *PropertiesCache) Set(key string, props *SourceProperties) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.props[key] = props
	c.notFound.MarkFound(key)
	metrics.RecordCacheHit("properties")
}

func (c *PropertiesCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.props, key)
	c.notFound.MarkFound(key)
}

func (c *PropertiesCache) IsNotFound(key string) bool {
	return c.notFound.IsNotFound(key)
}

func (c *PropertiesCache) MarkNotFound(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.props, key)
	c.notFound.MarkNotFound(key)
}

func (c *PropertiesCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.props = make(map[string]*SourceProperties)
	c.notFound.Clear()
}
