package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"forge/internal/job"
)

type PostgresStore struct {
	db *pgxpool.Pool
}

func NewPostgresStore(ctx context.Context, connString string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, err
	}

	err = pool.Ping(ctx)
	if err != nil {
		return nil, err
	}

	return &PostgresStore{db: pool}, nil
}

// Add new job to queue
func (s *PostgresStore) Enqueue(ctx context.Context, j *job.Job) error {
	query := `
		INSERT INTO jobs (id, type, status, payload, attempts, max_retries, priority, queue, scheduled_at, created_at, updated_at, last_error)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err := s.db.Exec(ctx, query,
		j.ID, j.Type, j.Status, j.Payload, j.Attempts, j.MaxRetries,
		j.Priority, j.Queue, j.ScheduledAt, j.CreatedAt, j.UpdatedAt, j.LastError,
	)
	return err
}

// Fetch next available job using advisory locks to prevent double-processing
func (s *PostgresStore) Dequeue(ctx context.Context, jobType string) (*job.Job, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	query := `
		SELECT id, type, status, payload, attempts, max_retries, priority, queue, scheduled_at, created_at, updated_at, last_error
		FROM jobs
		WHERE status = 'PENDING'
		  AND type = $1
		  AND scheduled_at <= $2
		  AND pg_try_advisory_xact_lock(hashtext(id))
		ORDER BY priority DESC, scheduled_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`

	now := time.Now().UnixMilli()
	var j job.Job
	err = tx.QueryRow(ctx, query, jobType, now).Scan(
		&j.ID, &j.Type, &j.Status, &j.Payload, &j.Attempts, &j.MaxRetries,
		&j.Priority, &j.Queue, &j.ScheduledAt, &j.CreatedAt, &j.UpdatedAt, &j.LastError,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	updateQuery := `
		UPDATE jobs
		SET status = 'PROCESSING', attempts = attempts + 1, updated_at = $1
		WHERE id = $2
	`
	_, err = tx.Exec(ctx, updateQuery, time.Now().UnixMilli(), j.ID)
	if err != nil {
		return nil, err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, err
	}

	j.Status = job.Processing
	j.Attempts++
	return &j, nil
}

// Mark job as successfully completed
func (s *PostgresStore) Complete(ctx context.Context, id string) error {
	query := `
		UPDATE jobs
		SET status = 'COMPLETED', updated_at = $1
		WHERE id = $2
	`
	_, err := s.db.Exec(ctx, query, time.Now().UnixMilli(), id)
	return err
}

// Mark job as failed; moves to DEAD if max retries exceeded
func (s *PostgresStore) Fail(ctx context.Context, id string, errMsg string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var j job.Job
	query := `SELECT attempts, max_retries FROM jobs WHERE id = $1`
	err = tx.QueryRow(ctx, query, id).Scan(&j.Attempts, &j.MaxRetries)
	if err != nil {
		return err
	}

	now := time.Now().UnixMilli()

	if j.Attempts >= j.MaxRetries {
		updateQuery := `
			UPDATE jobs
			SET status = $1, last_error = $2, updated_at = $3
			WHERE id = $4
		`
		_, err = tx.Exec(ctx, updateQuery, job.Dead, errMsg, now, id)
	} else {
		delay := j.NextRetryDelay()
		scheduledAt := time.Now().Add(delay).UnixMilli()
		updateQuery := `
			UPDATE jobs
			SET status = $1, last_error = $2, updated_at = $3, scheduled_at = $4
			WHERE id = $5
		`
		_, err = tx.Exec(ctx, updateQuery, job.Pending, errMsg, now, scheduledAt, id)
	}

	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Retrieve a job by ID
func (s *PostgresStore) Get(ctx context.Context, id string) (*job.Job, error) {
	query := `
		SELECT id, type, status, payload, attempts, max_retries, priority, queue, scheduled_at, created_at, updated_at, last_error
		FROM jobs
		WHERE id = $1
	`
	var j job.Job
	err := s.db.QueryRow(ctx, query, id).Scan(
		&j.ID, &j.Type, &j.Status, &j.Payload, &j.Attempts, &j.MaxRetries,
		&j.Priority, &j.Queue, &j.ScheduledAt, &j.CreatedAt, &j.UpdatedAt, &j.LastError,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("job not found: %s", id)
	}
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// List all dead jobs
func (s *PostgresStore) ListDead(ctx context.Context) ([]*job.Job, error) {
	query := `
		SELECT id, type, status, payload, attempts, max_retries, priority, queue, scheduled_at, created_at, updated_at, last_error
		FROM jobs
		WHERE status = 'DEAD'
		ORDER BY updated_at DESC
	`
	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*job.Job
	for rows.Next() {
		var j job.Job
		err := rows.Scan(
			&j.ID, &j.Type, &j.Status, &j.Payload, &j.Attempts, &j.MaxRetries,
			&j.Priority, &j.Queue, &j.ScheduledAt, &j.CreatedAt, &j.UpdatedAt, &j.LastError,
		)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, &j)
	}
	return jobs, nil
}

// Move dead job back to pending for retry
func (s *PostgresStore) RetryDead(ctx context.Context, id string) error {
	now := time.Now().UnixMilli()
	query := `
		UPDATE jobs
		SET status = 'PENDING', attempts = 0, scheduled_at = $1, updated_at = $1, last_error = ''
		WHERE id = $2 AND status = 'DEAD'
	`
	_, err := s.db.Exec(ctx, query, now, id)
	return err
}

// Get queue statistics
func (s *PostgresStore) Stats(ctx context.Context) (*QueueStats, error) {
	query := `
		SELECT
			COUNT(*) FILTER (WHERE status = 'PENDING') as pending,
			COUNT(*) FILTER (WHERE status = 'PROCESSING') as processing,
			COUNT(*) FILTER (WHERE status = 'COMPLETED') as completed,
			COUNT(*) FILTER (WHERE status = 'DEAD') as dead,
			COUNT(*) as total
		FROM jobs
	`
	var stats QueueStats
	err := s.db.QueryRow(ctx, query).Scan(
		&stats.Pending, &stats.Processing, &stats.Completed, &stats.Dead, &stats.Total,
	)
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// Close the database connection pool
func (s *PostgresStore) Close() {
	s.db.Close()
}
