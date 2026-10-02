package standalone

import (
	"context"
	"sync"
)

// Cancellable manages the cancellation lifecycle of an execution context.
type Cancellable struct {
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
}

// NewCancellable creates a new Cancellable wrapping a standard context.WithCancel.
func NewCancellable(parent context.Context) *Cancellable {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	return &Cancellable{
		ctx:    ctx,
		cancel: cancel,
	}
}

// Context returns the underlying context.
func (c *Cancellable) Context() context.Context {
	return c.ctx
}

// Cancel triggers cancellation of the underlying context.
func (c *Cancellable) Cancel() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel != nil {
		c.cancel()
	}
}

// Done returns the channel that is closed when work done on behalf of this context should be cancelled.
func (c *Cancellable) Done() <-chan struct{} {
	return c.ctx.Done()
}

// Err returns non-nil error after Done is closed.
func (c *Cancellable) Err() error {
	return c.ctx.Err()
}
