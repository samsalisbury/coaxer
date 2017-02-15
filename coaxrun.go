package coaxer

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Promise encapsulates a future result.
type Promise struct {
	final  *Result
	result <-chan Result
	sync.RWMutex
}

// Result waits for the final result to be generated and then returns it.
func (p *Promise) Result() Result {
	p.RLock()
	defer p.RUnlock()
	if p.final == nil {
		p.RUnlock()
		p.Lock()
		f := <-p.result
		p.final = &f
		p.Unlock()
		p.RLock()
	}
	return *p.final
}

// coaxRun encapsulates a single run of coaxing a value.
type coaxRun struct {
	Coaxer
	// ctx is the cancellation context for this coaxRun.
	ctx context.Context
	// manifest is called up to attempts times to try to create value.
	manifest func() (interface{}, error)
	// desc is a description of the value being coaxed, for logging purposes
	// only.
	desc string
	// cache is populated once manifest succeeds.
	cache Result
	// once is used to invoke the attempter once.
	once sync.Once
	// result returns the final result once cache is populated.
	result chan Result
	// finalResult is used internally to ensure we only get one result.
	finalResult chan Result
}

func (run *coaxRun) future() Promise {
	run.once.Do(func() {
		go run.generateResult()
		go run.produce()
	})
	return Promise{result: run.result}
}

// generateResult is responsible for generating the final Result.
func (run *coaxRun) generateResult() {
	for remaining := run.Attempts; remaining > 0; remaining-- {
		if run.attemptOnce() {
			return
		}
	}
	run.finalise(Result{Error: fmt.Errorf("gave up after %d attempts", run.Attempts)})
}

func (run *coaxRun) attemptOnce() bool {
	var intermediate Result
	select {
	case <-run.ctx.Done():
		if run.ctx.Err() == nil {
			panic("nil context error")
		}
		return run.finalise(Result{Error: run.ctx.Err()})
	case intermediate = <-run.attempt():
	}
	if intermediate.Error == nil {
		log.Printf("INTERMEDIATE RESULT for %q", run.desc)
		return run.finalise(intermediate)
	}
	if temp, ok := intermediate.Error.(interface {
		Temporary() bool
	}); !ok || !temp.Temporary() {
		// Not temporary, return original error, suffixed (unrecoverable).
		return run.finalise(Result{Error: fmt.Errorf("%s (unrecoverable)", intermediate.Error)})
	}
	time.Sleep(run.Backoff)
	run.Backoff *= time.Duration(run.BackoffScale)
	return false
}

func (run *coaxRun) finalise(r Result) bool {
	run.finalResult <- r
	close(run.finalResult)
	return true
}

func (run *coaxRun) attempt() <-chan Result {
	r := make(chan Result)
	go func() {
		value, err := run.manifest()
		r <- Result{Value: value, Error: err}
	}()
	return r
}

// produce repeatedly produces the result on c.result, until the c.ctx is done.
func (run *coaxRun) produce() {
	defer close(run.result)
	var final Result
	select {
	case final = <-run.finalResult:
	case <-run.ctx.Done():
		final = Result{Error: run.ctx.Err()}
	}
	for {
		select {
		case <-run.ctx.Done():
			run.result <- final
			return
		case run.result <- final:
		}
	}
}
