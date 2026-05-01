package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"forge/internal/job"
	"forge/internal/store"
)

type Server struct {
	store store.JobStore
	mux   *http.ServeMux
}

func New(s store.JobStore) *Server {
	srv := &Server{
		store: s,
		mux:   http.NewServeMux(),
	}
	srv.routes()
	return srv
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("POST /jobs", s.cors(s.enqueue))
	s.mux.HandleFunc("GET /jobs/{id}", s.cors(s.getJob))
	s.mux.HandleFunc("GET /dlq", s.cors(s.listDLQ))
	s.mux.HandleFunc("POST /dlq/{id}/retry", s.cors(s.retryDead))
	s.mux.HandleFunc("GET /stats", s.cors(s.stats))
	s.mux.HandleFunc("OPTIONS /", s.corsOptions)
}

func (s *Server) cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		next(w, r)
	}
}

func (s *Server) corsOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.WriteHeader(http.StatusOK)
}

// POST /jobs - enqueue a new job
func (s *Server) enqueue(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type        string          `json:"type"`
		Payload     json.RawMessage `json:"payload"`
		Priority    int16           `json:"priority"`
		Queue       string          `json:"queue"`
		ScheduledAt *int64          `json:"scheduled_at"`
		MaxRetries  *int16          `json:"max_retries"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	now := time.Now().UnixMilli()
	scheduledAt := now
	if req.ScheduledAt != nil {
		scheduledAt = *req.ScheduledAt
	}

	maxRetries := int16(3)
	if req.MaxRetries != nil {
		maxRetries = *req.MaxRetries
	}

	queue := "default"
	if req.Queue != "" {
		queue = req.Queue
	}

	j := &job.Job{
		ID:          uuid.New().String(),
		Type:        req.Type,
		Status:      job.Pending,
		Payload:     req.Payload,
		Priority:    req.Priority,
		Queue:       queue,
		MaxRetries:  maxRetries,
		ScheduledAt: scheduledAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.store.Enqueue(r.Context(), j); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(j)
}

// GET /jobs/{id} - get job status
func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	j, err := s.store.Get(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(j)
}

// GET /dlq - list dead jobs
func (s *Server) listDLQ(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.store.ListDead(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

// POST /dlq/{id}/retry - retry a dead job
func (s *Server) retryDead(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.store.RetryDead(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GET /stats - dashboard stats
func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.Stats(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
