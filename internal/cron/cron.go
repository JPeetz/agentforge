// Package cron provides a native cron scheduler for AgentForge.
//
// Uses time.Ticker and a sorted job list — no external dependencies
// beyond the Go standard library. Supports two schedule formats:
//
//	1. Cron expressions: "30 8 * * *" (min hour dom month dow)
//	2. Duration shorthand: "@every 5m", "@every 30s", "@every 1h"
//
// The scheduler runs a goroutine that ticks every second, checks for
// jobs whose NextRun has passed, and fires them via a callback.
// Jobs are automatically rescheduled after execution.
package cron

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ── Types ───────────────────────────────────────────────────────────────────

// Job represents a single scheduled cron job.
type Job struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Schedule string         `json:"schedule"` // cron expr or @every shorthand
	Command  string         `json:"command"`  // agent ID, pipeline name, or tool name
	Args     map[string]any `json:"args,omitempty"`
	Enabled  bool           `json:"enabled"`
	LastRun  time.Time      `json:"lastRun,omitempty"`
	NextRun  time.Time      `json:"nextRun,omitempty"`
	nextFn   func(time.Time) time.Time // cached schedule function
}

// FireFunc is called when a job fires. It receives the job and
// should execute the associated command/agent/pipeline.
type FireFunc func(ctx context.Context, job Job)

// Scheduler manages a set of cron jobs with a 1-second polling loop.
type Scheduler struct {
	mu     sync.RWMutex
	jobs   map[string]*Job
	ids    []string // sorted for deterministic iteration
	fire   FireFunc
	logger *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc
	ticker *time.Ticker
	done   chan struct{}
}

// ── Constructor ─────────────────────────────────────────────────────────────

// New creates a new Scheduler with the given fire callback.
// Pass nil to use a discard-logger; the handler is called for every job fire.
func New(fire FireFunc) *Scheduler {
	return NewWithLogger(fire, nil)
}

// NewWithLogger creates a new Scheduler with a custom logger.
func NewWithLogger(fire FireFunc, logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{
		jobs:   make(map[string]*Job),
		fire:   fire,
		logger: logger,
		done:   make(chan struct{}),
	}
}

// ── Job Management ──────────────────────────────────────────────────────────

// Add parses the job's schedule, computes the next run time, and enqueues it.
// Returns an error if the schedule expression is invalid.
func (s *Scheduler) Add(job Job) error {
	nextFn, err := parseSchedule(job.Schedule)
	if err != nil {
		return fmt.Errorf("cron: parse schedule %q: %w", job.Schedule, err)
	}

	if job.ID == "" {
		job.ID = fmt.Sprintf("%s-%d", job.Name, time.Now().UnixNano())
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	job.nextFn = nextFn
	job.NextRun = nextFn(now)

	// If the job exists, merge fields
	if existing, ok := s.jobs[job.ID]; ok {
		existing.Name = job.Name
		existing.Schedule = job.Schedule
		existing.Command = job.Command
		existing.Args = job.Args
		existing.Enabled = job.Enabled
		existing.nextFn = job.nextFn
		existing.NextRun = job.NextRun
	} else {
		cp := job
		s.jobs[job.ID] = &cp
		s.ids = append(s.ids, job.ID)
		sort.Strings(s.ids)
	}

	s.logger.Info("cron job added",
		slog.String("id", job.ID),
		slog.String("name", job.Name),
		slog.String("schedule", job.Schedule),
		slog.String("nextRun", job.NextRun.Format(time.RFC3339)),
	)
	return nil
}

// Remove deletes a job by ID. Returns an error if the job doesn't exist.
func (s *Scheduler) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.jobs[id]; !ok {
		return fmt.Errorf("cron: job %q not found", id)
	}
	delete(s.jobs, id)
	// Filter ids slice
	filtered := make([]string, 0, len(s.ids))
	for _, jid := range s.ids {
		if jid != id {
			filtered = append(filtered, jid)
		}
	}
	s.ids = filtered
	s.logger.Info("cron job removed", slog.String("id", id))
	return nil
}

// List returns a snapshot of all registered jobs, sorted by ID.
func (s *Scheduler) List() []Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Job, 0, len(s.ids))
	for _, id := range s.ids {
		if j, ok := s.jobs[id]; ok {
			out = append(out, *j)
		}
	}
	return out
}

// Trigger runs a job immediately (in the current goroutine) and reschedules it.
func (s *Scheduler) Trigger(id string) error {
	s.mu.Lock()
	job, ok := s.jobs[id]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("cron: job %q not found", id)
	}
	// Clone for the fire callback
	cp := *job
	s.mu.Unlock()

	ctx := context.Background()
	if s.ctx != nil {
		ctx = s.ctx
	}

	s.fire(ctx, cp)

	// Mark last run and reschedule
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.LastRun = time.Now()
		if j.nextFn != nil {
			j.NextRun = j.nextFn(j.LastRun)
		}
	}
	s.logger.Info("cron job triggered manually",
		slog.String("id", id),
		slog.String("name", cp.Name),
	)
	return nil
}

// ── Lifecycle ───────────────────────────────────────────────────────────────

// Start begins the polling loop. The scheduler checks every second whether any
// jobs are due and fires them in separate goroutines. Calling Start on an
// already-running scheduler is a no-op.
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return // already running
	}
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.ticker = time.NewTicker(1 * time.Second)
	jobCount := len(s.jobs)
	s.mu.Unlock()

	s.logger.Info("cron scheduler started", slog.Int("jobs", jobCount))

	go func() {
		defer func() {
			s.mu.Lock()
			s.ticker.Stop()
			s.mu.Unlock()
			close(s.done)
		}()
		for {
			select {
			case <-s.ctx.Done():
				return
			case now := <-s.ticker.C:
				s.tick(now)
			}
		}
	}()
}

// Stop halts the polling loop and waits for any in-flight fires to complete.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if s.cancel == nil {
		s.mu.Unlock()
		return
	}
	s.cancel()
	s.mu.Unlock()
	<-s.done
	s.logger.Info("cron scheduler stopped")
}

// tick checks for due jobs and fires them.
func (s *Scheduler) tick(now time.Time) {
	s.mu.RLock()
	var due []*Job
	for _, id := range s.ids {
		job := s.jobs[id]
		if !job.Enabled {
			continue
		}
		if job.NextRun.IsZero() {
			continue
		}
		if now.After(job.NextRun) || now.Equal(job.NextRun) {
			cp := *job
			due = append(due, &cp)
		}
	}
	s.mu.RUnlock()

	for _, job := range due {
		job := job
		go func() {
			s.logger.Debug("cron firing job",
				slog.String("id", job.ID),
				slog.String("name", job.Name),
				slog.String("command", job.Command),
			)

			s.fire(s.ctx, *job)

			// Reschedule
			s.mu.Lock()
			if j, ok := s.jobs[job.ID]; ok {
				j.LastRun = time.Now()
				if j.nextFn != nil {
					j.NextRun = j.nextFn(j.LastRun)
				}
			}
			s.mu.Unlock()
		}()
	}
}

// Count returns the total number of registered jobs.
func (s *Scheduler) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.jobs)
}

// ── Schedule Parsing ────────────────────────────────────────────────────────

// parseSchedule parses a cron expression or @every shorthand into a function
// that computes the next run time given a reference time.
func parseSchedule(spec string) (func(time.Time) time.Time, error) {
	spec = strings.TrimSpace(spec)

	// @every shorthand
	if strings.HasPrefix(spec, "@every ") {
		durStr := strings.TrimSpace(strings.TrimPrefix(spec, "@every "))
		d, err := time.ParseDuration(durStr)
		if err != nil {
			return nil, fmt.Errorf("invalid @every duration %q: %w", durStr, err)
		}
		if d <= 0 {
			return nil, fmt.Errorf("@every duration must be positive")
		}
		return func(from time.Time) time.Time {
			return from.Add(d)
		}, nil
	}

	// Standard 5-field cron expression: min hour dom month dow
	return parseCronExpr(spec)
}

// parseCronExpr parses a "min hour dom month dow" cron expression.
// Field values:
//   - min:  0-59
//   - hour: 0-23
//   - dom:  1-31
//   - month: 1-12
//   - dow:  0-6 (0=Sun)
//
// Each field can be: "*", a single number, "*/N", or a comma-separated list.
func parseCronExpr(expr string) (func(time.Time) time.Time, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron expression must have 5 fields, got %d in %q", len(fields), expr)
	}

	const (
		minIdx   = 0
		hourIdx  = 1
		domIdx   = 2
		monthIdx = 3
		dowIdx   = 4
	)

	minField, err := parseField(fields[minIdx], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("minute field: %w", err)
	}
	hourField, err := parseField(fields[hourIdx], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("hour field: %w", err)
	}
	domField, err := parseField(fields[domIdx], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("day-of-month field: %w", err)
	}
	monthField, err := parseField(fields[monthIdx], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("month field: %w", err)
	}
	dowField, err := parseField(fields[dowIdx], 0, 7) // 7 also means Sunday
	if err != nil {
		return nil, fmt.Errorf("day-of-week field: %w", err)
	}

	return func(from time.Time) time.Time {
		// Start from the next minute, incrementing until we find a match.
		// Cap iterations to avoid infinite loops on impossible schedules.
		candidate := from.Truncate(time.Minute).Add(time.Minute)
		for i := 0; i < 525600; i++ { // max one year of minutes
			if matchField(minField, candidate.Minute()) &&
				matchField(hourField, candidate.Hour()) &&
				matchField(domField, candidate.Day()) &&
				matchField(monthField, int(candidate.Month())) &&
				matchField(dowField, int(candidate.Weekday())) {
				return candidate
			}
			candidate = candidate.Add(time.Minute)
		}
		// Fallback: return a time far in the future
		return from.AddDate(10, 0, 0)
	}, nil
}

// cronField holds parsed cron field values.
// wildcard=true means "*" (every value). step=N means every Nth value.
// values holds explicit matched values.
type cronField struct {
	wildcard bool
	step     int
	values   map[int]bool
}

// parseField parses a single cron field string.
func parseField(s string, min, max int) (*cronField, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty field")
	}

	cf := &cronField{values: make(map[int]bool)}

	if s == "*" {
		cf.wildcard = true
		return cf, nil
	}

	if strings.HasPrefix(s, "*/") {
		step, err := strconv.Atoi(strings.TrimPrefix(s, "*/"))
		if err != nil || step < 1 {
			return nil, fmt.Errorf("invalid step %q", s)
		}
		cf.wildcard = true
		cf.step = step
		return cf, nil
	}

	// Comma-separated values, each can be a number or a range (e.g. "1-5")
	parts := strings.Split(s, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			rangeParts := strings.SplitN(part, "-", 2)
			start, err := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid range start %q", part)
			}
			end, err := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid range end %q", part)
			}
			if start < min || end > max || start > end {
				return nil, fmt.Errorf("range %q out of bounds [%d,%d]", part, min, max)
			}
			for v := start; v <= end; v++ {
				cf.values[v] = true
			}
		} else {
			v, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid value %q", part)
			}
			// Allow dow=7 to mean Sunday, normalise to 0
			if v == 7 && max == 7 {
				v = 0
			}
			if v < min || v > max {
				return nil, fmt.Errorf("value %d out of bounds [%d,%d]", v, min, max)
			}
			cf.values[v] = true
		}
	}

	return cf, nil
}

// matchField checks if a value v matches a parsed cron field.
func matchField(f *cronField, v int) bool {
	if f.wildcard {
		if f.step > 0 {
			return v%f.step == 0
		}
		return true
	}
	return f.values[v]
}