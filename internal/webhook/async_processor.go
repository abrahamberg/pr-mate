package webhook

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type AsyncConfig struct {
	QueueSize int
	Workers   int
}

type AsyncProcessor struct {
	processor *Processor
	jobs      chan job

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type job struct {
	eventType  string
	payload    []byte
	deliveryID string
}

func NewAsyncProcessor(processor *Processor, cfg AsyncConfig) *AsyncProcessor {
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 100
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 1
	}

	ctx, cancel := context.WithCancel(context.Background())

	p := &AsyncProcessor{
		processor: processor,
		jobs:      make(chan job, cfg.QueueSize),
		cancel:    cancel,
	}

	for i := 0; i < cfg.Workers; i++ {
		p.wg.Add(1)
		go p.worker(ctx)
	}

	return p
}

func (p *AsyncProcessor) Enqueue(ctx context.Context, eventType string, payload []byte, deliveryID string) error {
	_ = ctx
	if p.processor == nil {
		return errors.New("webhook processor is nil")
	}

	j := job{eventType: eventType, payload: append([]byte(nil), payload...), deliveryID: deliveryID}

	select {
	case p.jobs <- j:
		return nil
	default:
		return errors.New("webhook queue full")
	}
}

func (p *AsyncProcessor) Stop(ctx context.Context) error {
	p.cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		p.wg.Wait()
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("stop webhook workers: %w", ctx.Err())
	case <-done:
		return nil
	}
}

func (p *AsyncProcessor) worker(ctx context.Context) {
	defer p.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case j := <-p.jobs:
			_ = p.processor.Process(context.Background(), j.eventType, j.payload, j.deliveryID)
		}
	}
}
