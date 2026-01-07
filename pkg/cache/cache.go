// Package cache provides an in-memory cache for CloudCost API responses
// with TTL and stale data support for graceful degradation.
package cache

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/hawky-4s-/opencost-cloudcost-exporter/pkg/types"
)

// Cache stores CloudCost API responses with TTL and stale data support.
type Cache struct {
	mu        sync.RWMutex
	data      *types.CloudCostResponse
	fetchedAt time.Time
	ttl       time.Duration
	maxStale  time.Duration

	// Metrics (atomic for thread-safety)
	hits   atomic.Int64
	misses atomic.Int64
}

// New creates a new Cache with the specified TTL and max stale duration.
func New(ttl, maxStale time.Duration) *Cache {
	return &Cache{
		ttl:      ttl,
		maxStale: maxStale,
	}
}

// Get retrieves the cached data if available and not expired.
// Returns the data, whether it's stale, and whether data was found.
func (c *Cache) Get() (data *types.CloudCostResponse, isStale bool, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.data == nil {
		c.misses.Add(1)
		return nil, false, false
	}

	age := time.Since(c.fetchedAt)

	// Fresh data
	if age <= c.ttl {
		c.hits.Add(1)
		return c.data, false, true
	}

	// Stale but within max stale window
	if age <= c.ttl+c.maxStale {
		c.hits.Add(1)
		return c.data, true, true
	}

	// Too stale
	c.misses.Add(1)
	return nil, false, false
}

// Set stores new data in the cache.
func (c *Cache) Set(data *types.CloudCostResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = data
	c.fetchedAt = time.Now()
}

// Age returns the age of the cached data.
func (c *Cache) Age() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.data == nil {
		return 0
	}
	return time.Since(c.fetchedAt)
}

// Stats returns cache hit/miss statistics.
func (c *Cache) Stats() (hits, misses int64) {
	return c.hits.Load(), c.misses.Load()
}

// IsPopulated returns true if the cache has data.
func (c *Cache) IsPopulated() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data != nil
}
