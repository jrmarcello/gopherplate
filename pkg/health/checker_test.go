package health

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRunAll_AllChecksPass(t *testing.T) {
	checker := New()
	checker.Register("database", true, func(_ context.Context) error {
		return nil
	})
	checker.Register("redis", false, func(_ context.Context) error {
		return nil
	})

	healthy, statuses := checker.RunAll(context.Background())

	assert.True(t, healthy)
	assert.Len(t, statuses, 2)
	assert.Equal(t, "ok", statuses[0].Status)
	assert.Equal(t, "ok", statuses[1].Status)
	assert.Empty(t, statuses[0].Error)
	assert.Empty(t, statuses[1].Error)
}

func TestRunAll_CriticalCheckFails(t *testing.T) {
	checker := New()
	checker.Register("database", true, func(_ context.Context) error {
		return errors.New("connection refused")
	})

	healthy, statuses := checker.RunAll(context.Background())

	assert.False(t, healthy)
	assert.Len(t, statuses, 1)
	assert.Equal(t, "unavailable", statuses[0].Status)
	assert.Equal(t, "connection refused", statuses[0].Error)
	assert.True(t, statuses[0].Critical)
}

func TestRunAll_NonCriticalCheckFails(t *testing.T) {
	checker := New()
	checker.Register("database", true, func(_ context.Context) error {
		return nil
	})
	checker.Register("redis", false, func(_ context.Context) error {
		return errors.New("redis timeout")
	})

	healthy, statuses := checker.RunAll(context.Background())

	assert.True(t, healthy)
	assert.Len(t, statuses, 2)
	assert.Equal(t, "ok", statuses[0].Status)
	assert.Equal(t, "unavailable", statuses[1].Status)
	assert.Equal(t, "redis timeout", statuses[1].Error)
	assert.False(t, statuses[1].Critical)
}

func TestRunAll_TimeoutExceeded(t *testing.T) {
	checker := New(WithTimeout(50 * time.Millisecond))
	checker.Register("slow_service", true, func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
			return nil
		}
	})

	healthy, statuses := checker.RunAll(context.Background())

	assert.False(t, healthy)
	assert.Len(t, statuses, 1)
	assert.Equal(t, "unavailable", statuses[0].Status)
	assert.Contains(t, statuses[0].Error, "context deadline exceeded")
}

func TestRunAll_NoChecksRegistered(t *testing.T) {
	checker := New()

	healthy, statuses := checker.RunAll(context.Background())

	assert.True(t, healthy)
	assert.Empty(t, statuses)
	assert.NotNil(t, statuses) // should be empty slice, not nil
}

func TestRunAll_MixedResults(t *testing.T) {
	checker := New()
	checker.Register("database_writer", true, func(_ context.Context) error {
		return nil
	})
	checker.Register("database_reader", false, func(_ context.Context) error {
		return errors.New("replica lag")
	})
	checker.Register("redis", false, func(_ context.Context) error {
		return nil
	})

	healthy, statuses := checker.RunAll(context.Background())

	assert.True(t, healthy) // only non-critical failed
	assert.Len(t, statuses, 3)

	assert.Equal(t, "database_writer", statuses[0].Name)
	assert.Equal(t, "ok", statuses[0].Status)

	assert.Equal(t, "database_reader", statuses[1].Name)
	assert.Equal(t, "unavailable", statuses[1].Status)
	assert.Equal(t, "replica lag", statuses[1].Error)

	assert.Equal(t, "redis", statuses[2].Name)
	assert.Equal(t, "ok", statuses[2].Status)
}

func TestNew_DefaultTimeout(t *testing.T) {
	checker := New()

	assert.Equal(t, 5*time.Second, checker.timeout)
}

func TestNew_CustomTimeout(t *testing.T) {
	checker := New(WithTimeout(10 * time.Second))

	assert.Equal(t, 10*time.Second, checker.timeout)
}

func TestRegister_AddsCheck(t *testing.T) {
	checker := New()

	assert.Empty(t, checker.checks)

	checker.Register("db", true, func(_ context.Context) error { return nil })

	assert.Len(t, checker.checks, 1)
	assert.Equal(t, "db", checker.checks[0].name)
	assert.True(t, checker.checks[0].critical)
}
