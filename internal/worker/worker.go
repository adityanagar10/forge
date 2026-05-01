package worker

import (
	"context"
	"forge/internal/job"
	"forge/internal/store"
	"log"
	"sync"
	"time"
)

type Handler func(ctx context.Context, j *job.Job) error

type Pool struct {
	store        store.JobStore
	handlers     map[string]Handler
	concurrency  int
	pollInterval time.Duration
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
}

type Config struct {
	Concurrency  int
	PollInterval time.Duration
}

func New(s store.JobStore, cfg Config) *Pool {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Pool{
		store:        s,
		handlers:     make(map[string]Handler),
		concurrency:  cfg.Concurrency,
		pollInterval: cfg.PollInterval,
		ctx:          ctx,
		cancel:       cancel,
	}
}

func (p *Pool) Register(jobType string, handler Handler) {
	p.handlers[jobType] = handler
}

func (p *Pool) Start() {
	for i := 0; i < p.concurrency; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

func (p *Pool) Stop() {
	log.Println("Shutting down workers...")
	p.cancel()
	p.wg.Wait()
	log.Println("All workers stopped")
}

func (p *Pool) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			log.Printf("Worker %d stopping", id)
			return
		default:
			p.processJobs()
		}
	}
}

func (p *Pool) processJobs() {
	for jobType, handler := range p.handlers {
		select {
		case <-p.ctx.Done():
			return
		default:
		}

		j, err := p.store.Dequeue(p.ctx, jobType)
		if err != nil {
			log.Printf("Error dequeuing job: %v", err)
			time.Sleep(p.pollInterval)
			continue
		}

		if j == nil {
			time.Sleep(p.pollInterval)
			continue
		}

		log.Printf("Processing job %s (type=%s, attempt=%d)", j.ID, j.Type, j.Attempts)

		err = handler(p.ctx, j)
		if err != nil {
			log.Printf("Job %s failed: %v", j.ID, err)
			p.store.Fail(p.ctx, j.ID, err.Error())
		} else {
			log.Printf("Job %s completed", j.ID)
			p.store.Complete(p.ctx, j.ID)
		}
	}
}
