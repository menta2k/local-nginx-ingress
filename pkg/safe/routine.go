package safe

import (
	"context"
	"runtime/debug"
	"sync"

	"github.com/rs/zerolog/log"
)

type routineCtx func(ctx context.Context)

// Pool is a pool of go routines.
type Pool struct {
	waitGroup sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewPool creates a Pool.
func NewPool(parentCtx context.Context) *Pool {
	ctx, cancel := context.WithCancel(parentCtx)
	return &Pool{
		ctx:    ctx,
		cancel: cancel,
	}
}

// GoCtx starts a recoverable goroutine with a context.
func (p *Pool) GoCtx(goroutine routineCtx) {
	p.waitGroup.Add(1)
	Go(func() {
		defer p.waitGroup.Done()
		goroutine(p.ctx)
	})
}

// Stop stops all started routines, waiting for their termination.
func (p *Pool) Stop() {
	p.cancel()
	p.waitGroup.Wait()
}

// Go starts a recoverable goroutine.
func Go(goroutine func()) {
	GoWithRecover(goroutine, defaultRecoverGoroutine)
}

// GoWithRecover starts a recoverable goroutine using given customRecover() function.
func GoWithRecover(goroutine func(), customRecover func(err any)) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				customRecover(err)
			}
		}()
		goroutine()
	}()
}

func defaultRecoverGoroutine(err any) {
	log.Error().Interface("error", err).Msg("Error in Go routine")
	log.Error().Msgf("Stack: %s", debug.Stack())
}

// OperationWithRecover wrap a backoff operation in a Recover.
func OperationWithRecover[T any](operation func() (T, error)) func() (T, error) {
	return func() (T, error) {
		defer func() {
			if res := recover(); res != nil {
				defaultRecoverGoroutine(res)
			}
		}()
		return operation()
	}
}