package cache

import (
	"testing"
	"time"

	"github.com/hawky-4s-/opencost-cloudcost-exporter/pkg/types"
)

func TestCache_GetSet(t *testing.T) {
	c := New(time.Hour, time.Hour*6)

	// Empty cache should return no data
	data, isStale, ok := c.Get()
	if ok {
		t.Error("Get() on empty cache should return ok=false")
	}
	if data != nil {
		t.Error("Get() on empty cache should return nil data")
	}
	if isStale {
		t.Error("Get() on empty cache should return isStale=false")
	}

	// Set data
	testData := &types.CloudCostResponse{Code: 200}
	c.Set(testData)

	// Should return fresh data
	data, isStale, ok = c.Get()
	if !ok {
		t.Error("Get() after Set() should return ok=true")
	}
	if data == nil {
		t.Error("Get() after Set() should return non-nil data")
	}
	if isStale {
		t.Error("Get() on fresh data should return isStale=false")
	}
	if data.Code != 200 {
		t.Errorf("data.Code = %v, want 200", data.Code)
	}
}

func TestCache_TTL(t *testing.T) {
	// Very short TTL for testing
	c := New(10*time.Millisecond, 50*time.Millisecond)

	testData := &types.CloudCostResponse{Code: 200}
	c.Set(testData)

	// Fresh data
	_, isStale, ok := c.Get()
	if !ok || isStale {
		t.Error("Data should be fresh immediately after Set()")
	}

	// Wait for TTL to expire
	time.Sleep(15 * time.Millisecond)

	// Stale data
	_, isStale, ok = c.Get()
	if !ok {
		t.Error("Data should still be available as stale")
	}
	if !isStale {
		t.Error("Data should be marked as stale after TTL expires")
	}

	// Wait for max stale to expire
	time.Sleep(50 * time.Millisecond)

	// Data should be too stale
	_, _, ok = c.Get()
	if ok {
		t.Error("Data should not be available after max stale expires")
	}
}

func TestCache_Age(t *testing.T) {
	c := New(time.Hour, time.Hour*6)

	// Empty cache age should be 0
	if c.Age() != 0 {
		t.Errorf("Age() on empty cache = %v, want 0", c.Age())
	}

	c.Set(&types.CloudCostResponse{})
	time.Sleep(10 * time.Millisecond)

	age := c.Age()
	if age < 10*time.Millisecond {
		t.Errorf("Age() = %v, want >= 10ms", age)
	}
}

func TestCache_Stats(t *testing.T) {
	c := New(10*time.Millisecond, 10*time.Millisecond)

	// Miss on empty cache
	c.Get()
	hits, misses := c.Stats()
	if hits != 0 || misses != 1 {
		t.Errorf("Stats() = (%v, %v), want (0, 1)", hits, misses)
	}

	// Hit after set
	c.Set(&types.CloudCostResponse{})
	c.Get()
	hits, misses = c.Stats()
	if hits != 1 || misses != 1 {
		t.Errorf("Stats() = (%v, %v), want (1, 1)", hits, misses)
	}
}

func TestCache_IsPopulated(t *testing.T) {
	c := New(time.Hour, time.Hour*6)

	if c.IsPopulated() {
		t.Error("IsPopulated() on empty cache should return false")
	}

	c.Set(&types.CloudCostResponse{})

	if !c.IsPopulated() {
		t.Error("IsPopulated() after Set() should return true")
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	c := New(time.Hour, time.Hour*6)

	// Run concurrent reads and writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				c.Set(&types.CloudCostResponse{Code: 200})
				c.Get()
				c.Age()
				c.Stats()
				c.IsPopulated()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
