package cache

import "golang.org/x/sync/singleflight"

// FlightGroup deduplicates concurrent requests for the same key.
// Prevents cache stampede (thundering herd) when many goroutines
// query the same resource during a cache miss — only one actually
// hits the database, and the others receive the same result.
//
// Usage:
//
//	fg := cache.NewFlightGroup()
//	val, err, _ := fg.Do("user:123", func() (any, error) {
//	    return repo.FindByID(ctx, id)
//	})
type FlightGroup struct {
	group singleflight.Group
}

// NewFlightGroup creates a new FlightGroup.
func NewFlightGroup() *FlightGroup {
	return &FlightGroup{}
}

// Do executes fn once for the given key, even if called concurrently.
// Subsequent callers with the same key block until the first completes
// and all receive the same result.
func (fg *FlightGroup) Do(key string, fn func() (any, error)) (any, error, bool) {
	return fg.group.Do(key, fn)
}
