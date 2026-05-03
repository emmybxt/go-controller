package gocontroller

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TaskFunc is a function executed by the scheduler.
type TaskFunc func(ctx context.Context) error

// Task represents a scheduled task.
type Task struct {
	Name     string
	Schedule string
	Func     TaskFunc
	LastRun  time.Time
	NextRun  time.Time
	Enabled  bool
}

// Scheduler manages scheduled tasks.
type Scheduler struct {
	mu      sync.Mutex
	tasks   []*Task
	running bool
	stopCh  chan struct{}
	doneCh  chan struct{}
}

// NewScheduler creates a new task scheduler.
func NewScheduler() *Scheduler {
	return &Scheduler{
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

// AddTask registers a task with a cron-like schedule.
func (s *Scheduler) AddTask(name, schedule string, fn TaskFunc) error {
	if _, err := parseCron(schedule); err != nil {
		return fmt.Errorf("invalid schedule %q: %w", schedule, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	task := &Task{
		Name:     name,
		Schedule: schedule,
		Func:     fn,
		Enabled:  true,
	}
	s.tasks = append(s.tasks, task)
	task.NextRun = s.nextRun(task)

	return nil
}

// RemoveTask removes a task by name.
func (s *Scheduler) RemoveTask(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, task := range s.tasks {
		if task.Name == name {
			s.tasks = append(s.tasks[:i], s.tasks[i+1:]...)
			return true
		}
	}
	return false
}

// Start begins executing scheduled tasks.
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
			close(s.doneCh)
			return
		case <-s.stopCh:
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
			close(s.doneCh)
			return
		case <-ticker.C:
			s.runDueTasks(ctx)
		}
	}
}

// Stop stops the scheduler.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	close(s.stopCh)
	<-s.doneCh

	s.stopCh = make(chan struct{})
	s.doneCh = make(chan struct{})
}

// Tasks returns all registered tasks.
func (s *Scheduler) Tasks() []*Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]*Task, len(s.tasks))
	for i, task := range s.tasks {
		t := *task
		result[i] = &t
	}
	return result
}

func (s *Scheduler) runDueTasks(ctx context.Context) {
	s.mu.Lock()
	now := time.Now()
	var due []*Task
	for _, task := range s.tasks {
		if task.Enabled && !task.NextRun.IsZero() && now.After(task.NextRun) {
			due = append(due, task)
		}
	}
	s.mu.Unlock()

	for _, task := range due {
		go s.executeTask(ctx, task)
	}
}

func (s *Scheduler) executeTask(ctx context.Context, task *Task) {
	err := task.Func(ctx)

	s.mu.Lock()
	task.LastRun = time.Now()
	task.NextRun = s.nextRun(task)
	s.mu.Unlock()

	if err != nil {
		fmt.Printf("[gocontroller] task %s failed: %v\n", task.Name, err)
	}
}

func (s *Scheduler) nextRun(task *Task) time.Time {
	fields, err := parseCron(task.Schedule)
	if err != nil {
		return time.Time{}
	}

	now := time.Now()
	for i := 0; i < 366*24*60; i++ {
		candidate := now.Add(time.Duration(i) * time.Minute)
		if matchesCron(candidate, fields) {
			return candidate
		}
	}
	return time.Time{}
}

type cronFields struct {
	minutes  map[int]bool
	hours    map[int]bool
	days     map[int]bool
	months   map[int]bool
	weekdays map[int]bool
}

func parseCron(expr string) (cronFields, error) {
	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return cronFields{}, fmt.Errorf("expected 5 fields, got %d", len(parts))
	}

	minutes, err := parseField(parts[0], 0, 59)
	if err != nil {
		return cronFields{}, fmt.Errorf("minutes: %w", err)
	}
	hours, err := parseField(parts[1], 0, 23)
	if err != nil {
		return cronFields{}, fmt.Errorf("hours: %w", err)
	}
	days, err := parseField(parts[2], 1, 31)
	if err != nil {
		return cronFields{}, fmt.Errorf("days: %w", err)
	}
	months, err := parseField(parts[3], 1, 12)
	if err != nil {
		return cronFields{}, fmt.Errorf("months: %w", err)
	}
	weekdays, err := parseField(parts[4], 0, 6)
	if err != nil {
		return cronFields{}, fmt.Errorf("weekdays: %w", err)
	}

	return cronFields{
		minutes:  minutes,
		hours:    hours,
		days:     days,
		months:   months,
		weekdays: weekdays,
	}, nil
}

func parseField(field string, min, max int) (map[int]bool, error) {
	result := make(map[int]bool)

	for _, part := range strings.Split(field, ",") {
		if part == "*" {
			for i := min; i <= max; i++ {
				result[i] = true
			}
			continue
		}

		if strings.Contains(part, "/") {
			parts := strings.Split(part, "/")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid range: %s", part)
			}
			start := min
			if parts[0] != "*" {
				var err error
				start, err = strconv.Atoi(parts[0])
				if err != nil {
					return nil, err
				}
			}
			if start < min || start > max {
				return nil, fmt.Errorf("value %d outside range %d-%d", start, min, max)
			}
			step, err := strconv.Atoi(parts[1])
			if err != nil {
				return nil, err
			}
			if step <= 0 {
				return nil, fmt.Errorf("step must be > 0")
			}
			for i := start; i <= max; i += step {
				result[i] = true
			}
			continue
		}

		if strings.Contains(part, "-") {
			parts := strings.Split(part, "-")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid range: %s", part)
			}
			start, err := strconv.Atoi(parts[0])
			if err != nil {
				return nil, err
			}
			end, err := strconv.Atoi(parts[1])
			if err != nil {
				return nil, err
			}
			if start < min || start > max || end < min || end > max || start > end {
				return nil, fmt.Errorf("range %d-%d outside range %d-%d", start, end, min, max)
			}
			for i := start; i <= end; i++ {
				result[i] = true
			}
			continue
		}

		val, err := strconv.Atoi(part)
		if err != nil {
			return nil, err
		}
		if val < min || val > max {
			return nil, fmt.Errorf("value %d outside range %d-%d", val, min, max)
		}
		result[val] = true
	}

	return result, nil
}

func matchesCron(t time.Time, fields cronFields) bool {
	if !fields.minutes[t.Minute()] {
		return false
	}
	if !fields.hours[t.Hour()] {
		return false
	}
	if !fields.days[t.Day()] {
		return false
	}
	if !fields.months[int(t.Month())] {
		return false
	}
	if !fields.weekdays[int(t.Weekday())] {
		return false
	}
	return true
}

func (t *Task) String() string {
	status := "enabled"
	if !t.Enabled {
		status = "disabled"
	}
	return fmt.Sprintf("Task{name: %s, schedule: %s, status: %s, next: %s}",
		t.Name, t.Schedule, status, t.NextRun.Format(time.RFC3339))
}

// TaskList returns a formatted string of all tasks.
func (s *Scheduler) TaskList() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	var lines []string
	for _, task := range s.tasks {
		lines = append(lines, task.String())
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// SchedulerMiddleware adds the scheduler to app context and manages lifecycle.
func SchedulerMiddleware(scheduler *Scheduler) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			ctx.Set("gocontroller.scheduler", scheduler)
			return next(ctx)
		}
	}
}

// GetScheduler retrieves the scheduler from context.
func GetScheduler(ctx *Context) *Scheduler {
	v, ok := ctx.Get("gocontroller.scheduler")
	if !ok {
		return nil
	}
	s, _ := v.(*Scheduler)
	return s
}
