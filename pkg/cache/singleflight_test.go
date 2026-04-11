package cache

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFlightGroup(t *testing.T) {
	fg := NewFlightGroup()
	require.NotNil(t, fg)
}

func TestFlightGroup_Do_ReturnsResult(t *testing.T) {
	fg := NewFlightGroup()
	val, doErr, shared := fg.Do("key1", func() (any, error) {
		return "hello", nil
	})
	assert.NoError(t, doErr)
	assert.Equal(t, "hello", val)
	assert.False(t, shared) // single caller, not shared
}

func TestFlightGroup_Do_ReturnsError(t *testing.T) {
	fg := NewFlightGroup()
	expectedErr := errors.New("db connection failed")
	val, doErr, _ := fg.Do("key1", func() (any, error) {
		return nil, expectedErr
	})
	assert.Nil(t, val)
	assert.ErrorIs(t, doErr, expectedErr)
}

func TestFlightGroup_Do_DeduplicatesConcurrentCalls(t *testing.T) {
	fg := NewFlightGroup()
	var callCount atomic.Int32

	fn := func() (any, error) {
		callCount.Add(1)
		time.Sleep(50 * time.Millisecond) // simulate slow DB query
		if callCount.Load() > 1 {
			return nil, errors.New("should not be called more than once")
		}
		return "result", nil
	}

	// Launch 10 concurrent calls for the same key
	var wg sync.WaitGroup
	results := make([]any, 10)
	errs := make([]error, 10)

	for i := range 10 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx], _ = fg.Do("same-key", fn)
		}(i)
	}
	wg.Wait()

	// fn should have been called exactly once
	assert.Equal(t, int32(1), callCount.Load(), "fn should execute only once for concurrent calls with same key")

	// All callers should get the same result
	for i := range 10 {
		assert.NoError(t, errs[i])
		assert.Equal(t, "result", results[i])
	}
}

func TestFlightGroup_Do_DifferentKeysRunIndependently(t *testing.T) {
	fg := NewFlightGroup()
	var callCount atomic.Int32

	fn := func() (any, error) {
		callCount.Add(1)
		return "result", nil
	}

	_, _, _ = fg.Do("key-a", fn)
	_, _, _ = fg.Do("key-b", fn)

	assert.Equal(t, int32(2), callCount.Load(), "different keys should execute independently")
}

func TestFlightGroup_Do_SubsequentCallsAfterCompletion(t *testing.T) {
	fg := NewFlightGroup()
	var callCount atomic.Int32

	fn := func() (any, error) {
		callCount.Add(1)
		return callCount.Load(), nil
	}

	// First call
	val1, _, _ := fg.Do("key", fn)
	// Second call (after first completed — should execute again)
	val2, _, _ := fg.Do("key", fn)

	assert.Equal(t, int32(2), callCount.Load(), "sequential calls should both execute")
	assert.Equal(t, int32(1), val1)
	assert.Equal(t, int32(2), val2)
}
