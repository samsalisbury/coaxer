package coaxer

import (
	"context"
	"sync"
	"time"
)

// Coaxer tries to coax a value into existence.
type Coaxer struct {
	// Attempts is the maximum number of times to try calling the manifest func
	// before giving up.
	Attempts,
	// Backoff is the current backoff duration (time between attempts).
	Backoff time.Duration
	// BackoffScale is the scaling factor used on Backoff on each attempt.
	BackoffScale int
	// Timeout is how long manifest is given per attempt before being abandoned.
	Timeout time.Duration
	// TimeoutScale is the scaling factor for the Timeout on each attempt.
	TimeoutScale int

	// manifest is called up to attempts times to try to create value.
	manifest func() (interface{}, error)
	// cache is populated once manifest succeeds.
	cache Result
	// once is used to invoke the attemptor once.
	once sync.Once
	// result returns the final result once cache is populated.
	result chan Result
}

const (
	// DefaultAttempts is the default number of attempts for a new Coaxer.
	DefaultAttempts = 3
	// DefaultBackoffScale is the default BackoffScale for a new Coaxer.
	DefaultBackoffScale = 2
	// DefaultTimeoutScale is the default TimeoutScale for a new Coaxer.
	DefaultTimeoutScale = 1
	// DefaultBackoff is the default initial Backoff duration for a new Coaxer.
	DefaultBackoff = 100 * time.Millisecond
	// DefaultTimeout is the default initial timeout for each attempt.
	DefaultTimeout = 2 * time.Second
)

// NewCoaxer returns a new Coaxer using manifest as its value constructor.
//
// You can optionally pass any number of functions to configure which will
// be called in the order passed on the new Coaxer before it is returned.
func NewCoaxer(manifest func() (interface{}, error), configure ...func(*Coaxer)) *Coaxer {
	c := &Coaxer{
		Attempts:     DefaultAttempts,
		Backoff:      DefaultBackoff,
		BackoffScale: DefaultBackoffScale,
		Timeout:      DefaultTimeout,
		TimeoutScale: DefaultTimeoutScale,
		manifest:     manifest,
		result:       make(chan Result),
	}
	for _, f := range configure {
		f(c)
	}
	return c
}

// Result is the result of coaxing.
type Result struct {
	// Value represents the final value obtained by coaxing. If Error is not
	// nil, then Value will be nil.
	Value interface{}
	// Error represents the fact that Value was not successfully coaxed into
	// existence. If error is not nil, then Value is nil.
	Error error
}

// Coax immediately returns a channel that will eventually produce the Result of
// this attempt to coax the result of manifest into existence.
func (c *Coaxer) Coax(ctx context.Context) <-chan Result {
	c.once.Do(func() { go c.generateResult(ctx, c.result) })
	return c.result
}

// generateResult is responsible for generating the final Result.
func (c *Coaxer) generateResult(ctx context.Context, r chan<- Result) {
	// TODO
}
