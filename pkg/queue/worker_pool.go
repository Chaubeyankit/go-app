package queue

import (
	"context"
	"fmt"
	"sync"

	"github.com/ankit.chaubey/myapp/pkg/logger"
	"go.uber.org/zap"
)

// WorkerPool runs N consumers concurrently, all reading from the same
// stream+group. Redis distributes messages across consumers automatically.
type WorkerPool struct {
	consumers []*Consumer
	wg        sync.WaitGroup
}

// NewWorkerPool creates a pool of `size` consumers.
// Each gets a unique name ("worker-1", "worker-2", ...) so Redis
// tracks their individual pending-entry lists.
func NewWorkerPool(rdb interface { /* *redis.Client */
}, stream, group string, size int, factory func(name string) *Consumer) *WorkerPool {
	pool := &WorkerPool{
		consumers: make([]*Consumer, size),
	}
	for i := 0; i < size; i++ {
		pool.consumers[i] = factory(fmt.Sprintf("%s-worker-%d", group, i+1))
	}
	return pool
}

// Start ensures the consumer group exists then launches all workers.
func (p *WorkerPool) Start(ctx context.Context) error {
	if len(p.consumers) == 0 {
		return nil
	}
	// All consumers share the same group — only need to create it once
	if err := p.consumers[0].EnsureGroup(ctx); err != nil {
		return fmt.Errorf("WorkerPool.Start EnsureGroup: %w", err)
	}

	for _, c := range p.consumers {
		c := c // capture loop var
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			c.Run(ctx)
		}()
	}

	logger.Info("worker pool started",
		zap.Int("workers", len(p.consumers)),
		zap.String("stream", p.consumers[0].stream),
		zap.String("group", p.consumers[0].group),
	)
	return nil
}

// Wait blocks until all workers have exited (ctx cancelled).
func (p *WorkerPool) Wait() {
	p.wg.Wait()
	logger.Info("worker pool stopped")
}

func (p *WorkerPool) Add(c *Consumer) {
	p.consumers = append(p.consumers, c)
}
