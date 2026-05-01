package job

import "time"

type Status string

const (
	Pending    Status = "PENDING"
	Processing Status = "PROCESSING"
	Completed  Status = "COMPLETED"
	Dead       Status = "DEAD"
)

type Job struct {
	ID          string
	Type        string
	Status      Status
	Payload     []byte
	Attempts    int16
	MaxRetries  int16
	Priority    int16
	ScheduledAt int64
	CreatedAt   int64
	UpdatedAt   int64
	LastError   string
	Queue       string
}

func (j *Job) CanRetry() bool {
	return j.Attempts < j.MaxRetries
}

func (j *Job) NextRetryDelay() time.Duration {
	base := 5 * time.Second
	shift := j.Attempts
	if shift > 10 {
		shift = 10
	}
	return base * (1 << j.Attempts)
}
