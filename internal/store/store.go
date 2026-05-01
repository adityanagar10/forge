package store

import (
	"context"

	"forge/internal/job"
)

type JobStore interface {
	Enqueue(ctx context.Context, job *job.Job) error
	Dequeue(ctx context.Context, jobType string) (*job.Job, error)
	Complete(ctx context.Context, id string) error
	Fail(ctx context.Context, id string, errMsg string) error
	Get(ctx context.Context, id string) (*job.Job, error)
	ListDead(ctx context.Context) ([]*job.Job, error)
	RetryDead(ctx context.Context, id string) error
	Stats(ctx context.Context) (*QueueStats, error)
}

type QueueStats struct {
	Pending    int64 `json:"pending"`
	Processing int64 `json:"processing"`
	Completed  int64 `json:"completed"`
	Dead       int64 `json:"dead"`
	Total      int64 `json:"total"`
}
