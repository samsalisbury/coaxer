package coaxer

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type TemporaryError struct {
	Message string
	Temp    bool
}

func (te TemporaryError) Temporary() bool {
	return te.Temp
}

func (te TemporaryError) Error() string {
	return te.Message
}

// ensureEventuallyCancelled just waits for roughly a year and then calls
// cancel unless the context gets cancelled sooner than that. This is mainly to
// show govet that we are not leaking contexts. In fact, these contexts are
// usually cancelled much sooner in the test code.
func ensureEventuallyCancelled(ctx context.Context, cancel *context.CancelFunc) {
	select {
	case <-time.After(8760 * time.Hour):
		(*cancel)()
	case <-ctx.Done():
	}
}

func TestCoaxer(t *testing.T) {
	// Make a permanent error.
	permErr := func(message string) error { return fmt.Errorf(message) }

	// Make a temporary error.
	tempErr := func(message string, temporary bool) error {
		return TemporaryError{Message: message, Temp: temporary}
	}

	// cancellable returns a func to make a context and a reference to that
	// context's cancel func. The reference contains nil until after the first
	// returned func is called.
	var cancellable = func() (func() context.Context, *context.CancelFunc) {
		var ctx context.Context
		var cancel context.CancelFunc
		return func() context.Context {
			ctx, cancel = context.WithCancel(context.Background())
			go ensureEventuallyCancelled(ctx, &cancel)
			return ctx
		}, &cancel
	}

	// TestFuncs encapsulates a group of related functions for use in tests.
	// The TestFuncs are called in a specific order: Context, Manifest, Do.
	type TestFuncs struct {
		// Context produces the context to be passed to Coax at the beginning of
		// the test run.
		Context func() context.Context
		// Manifest produces either Value or Error.
		Manifest func() (interface{}, error)
		// Do is called immediately after Coax. This can be used to interfere
		// with the Manifest func in interesting ways.
		Do func()
	}

	testCases := []struct {
		// Desc describes this test.
		Desc string
		// Configure configures the coaxer.
		Configure func(*Coaxer)
		// Setup is invoked at the start of the test, and generates a TestFuncs
		// containing Context, Manifest and Do functions. This is a func so that
		// you can share state between the functions in a closure, which is
		// useful for synchronisation.
		Setup func() TestFuncs
		// Error is the expected error returned. The returned error's Error
		// method must return the same string as this one.
		Error error
		// Value is the expected value to receive from Coax. Its '% #v'
		// formatted string is checked for equality.
		Value interface{}
	}{
		{
			Desc: "(nil, nil) manifest returns (nil, nil)",
			Setup: func() TestFuncs {
				return TestFuncs{
					Context: context.Background,
					Manifest: func() (interface{}, error) {
						return nil, nil
					},
				}
			},
		},
		{
			Desc: "non-temporary error returns that error (unrecoverable)",
			Setup: func() TestFuncs {
				return TestFuncs{
					Manifest: func() (interface{}, error) {
						return nil, permErr("error 1")
					},
					Context: context.Background,
				}
			},
			Error: fmt.Errorf("error 1 (unrecoverable)"),
		},
		{
			Desc: "Temporary()=false error returns that error (unrecoverable)",
			Setup: func() TestFuncs {
				return TestFuncs{
					Manifest: func() (interface{}, error) {
						return nil, tempErr("error 1", false)
					},
					Context: context.Background,
				}
			},
			Error: fmt.Errorf("error 1 (unrecoverable)"),
		},
		{
			Desc: "persistent temporary error gives up after 3 attempts",
			Setup: func() TestFuncs {
				return TestFuncs{
					Manifest: func() (interface{}, error) {
						return nil, tempErr("error 2", true)
					},
					Context: context.Background,
				}
			},
			Error: fmt.Errorf("gave up after 3 attempts"),
		},
		{
			Desc: "context cancelled before a single Manifest call completes",
			Setup: func() TestFuncs {
				ctxFunc, cancel := cancellable()
				wait := make(chan struct{})
				return TestFuncs{
					Context: ctxFunc,
					Manifest: func() (interface{}, error) {
						<-wait // Wait for Do to be called.
						return nil, permErr("you should not see this")
					},
					Do: func() {
						defer close(wait) // Allow Manifest to run.
						(*cancel)()
						time.Sleep(time.Second)
					},
				}
			},
			Error: fmt.Errorf("context canceled"),
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.Desc, func(t *testing.T) {
			funcs := test.Setup()
			coaxer := NewCoaxer(test.Configure)
			promise := coaxer.Coax(funcs.Context(), funcs.Manifest, test.Desc)
			if funcs.Do != nil {
				funcs.Do()
			}

			// Wait for the actual result to be manifested.
			actual := promise.Result()

			// Assert the error is as expected.
			if actual.Error != nil {
				if test.Error == nil {
					// If we get unexpected error, assume there is no point
					// in further assertions.
					t.Fatalf("got error %q; want nil", actual.Error)
				}
				if actual.Error.Error() != test.Error.Error() {
					t.Errorf("got error %q; want %q", actual.Error, test.Error)
				}
			}
			if actual.Error == nil {
				if test.Error != nil {
					t.Errorf("got nil; want error %q", test.Error)
				}
			}

			// Assert the value is as expected.
			if actual.Value != nil {
				if test.Value == nil {
					t.Errorf("got a %T value; want nil (value was: % #v)", actual.Value, actual.Value)
				}
				a := fmt.Sprintf("% #v", actual.Value)
				e := fmt.Sprintf("% #v", test.Value)
				if a != e {
					t.Errorf("got value %q; want %q", a, e)
				}
			}
			if actual.Value == nil {
				if test.Value != nil {
					t.Errorf("got nil value; want % #v", test.Value)
				}
			}

		})
	}
}
