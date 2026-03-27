package health

import (
	"context"
	"time"
)

// Status represents the health status of a single dependency.
type Status struct {
	Name     string `json:"name"`
	Status   string `json:"status"` // "ok", "unavailable"
	Critical bool   `json:"critical"`
	Error    string `json:"error,omitempty"`
}

// CheckFunc is a function that checks the health of a dependency.
// It should return nil if healthy, or an error if unhealthy.
type CheckFunc func(ctx context.Context) error

type check struct {
	name     string
	critical bool
	fn       CheckFunc
}

// Checker manages health checks for application dependencies.
type Checker struct {
	checks  []check
	timeout time.Duration
}

// Option configures the Checker.
type Option func(*Checker)

// WithTimeout sets the timeout for individual health checks.
func WithTimeout(d time.Duration) Option {
	return func(c *Checker) {
		c.timeout = d
	}
}

// New creates a new Checker with default 5-second timeout.
func New(opts ...Option) *Checker {
	c := &Checker{
		timeout: 5 * time.Second,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Register adds a health check. Critical checks cause the overall status to be unhealthy on failure.
func (c *Checker) Register(name string, critical bool, fn CheckFunc) {
	c.checks = append(c.checks, check{name: name, critical: critical, fn: fn})
}

// RunAll executes all registered checks and returns overall health status.
// healthy is false if ANY critical check fails.
func (c *Checker) RunAll(ctx context.Context) (healthy bool, statuses []Status) {
	healthy = true
	statuses = make([]Status, 0, len(c.checks))

	for _, ch := range c.checks {
		checkCtx, cancel := context.WithTimeout(ctx, c.timeout)

		s := Status{
			Name:     ch.name,
			Critical: ch.critical,
		}

		if checkErr := ch.fn(checkCtx); checkErr != nil {
			s.Status = "unavailable"
			s.Error = checkErr.Error()
			if ch.critical {
				healthy = false
			}
		} else {
			s.Status = "ok"
		}

		cancel()
		statuses = append(statuses, s)
	}

	return healthy, statuses
}
