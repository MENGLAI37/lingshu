package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/lingshu/lingshu/pkg/tools"
)

// ===========================================================================
// Mock Tool for Scheduler Testing
// ===========================================================================

type mockSchedulerTool struct {
	name      string
	riskLevel tools.ToolRiskLevel
	execCount int
}

func (t *mockSchedulerTool) Name() string                            { return t.name }
func (t *mockSchedulerTool) RiskLevel() tools.ToolRiskLevel           { return t.riskLevel }
func (t *mockSchedulerTool) Description() string                      { return "mock" }
func (t *mockSchedulerTool) ParameterSchema() map[string]interface{}   { return map[string]interface{}{} }
func (t *mockSchedulerTool) Execute(ctx context.Context, params map[string]any) (*tools.ToolResult, error) {
	t.execCount++
	return &tools.ToolResult{
		Success:   true,
		Message:   "mock exec",
		Timestamp: time.Now(),
		Duration:  "1ms",
		ToolName:  t.name,
		RiskLevel: t.riskLevel,
	}, nil
}

type mockSchedulerRegistry struct {
	tools map[string]tools.Tool
}

func (r *mockSchedulerRegistry) GetTool(name string) (tools.Tool, error) {
	t, ok := r.tools[name]
	if !ok {
		return nil, &toolError{name: name}
	}
	return t, nil
}

func (r *mockSchedulerRegistry) ListTools() []tools.Tool {
	result := make([]tools.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

func (r *mockSchedulerRegistry) RegisterTool(t tools.Tool) error {
	r.tools[t.Name()] = t
	return nil
}

type toolError struct{ name string }

func (e *toolError) Error() string { return "tool not found: " + e.name }

func newMockRegistryWith(name string, t tools.Tool) *mockSchedulerRegistry {
	r := &mockSchedulerRegistry{tools: map[string]tools.Tool{}}
	r.tools[name] = t
	return r
}

// ===========================================================================
// Scheduler Job CRUD Tests
// ===========================================================================

func TestAddJob_Success(t *testing.T) {
	registry := newMockRegistryWith("test", &mockSchedulerTool{name: "test"})
	s := NewDefaultScheduler(registry)

	job := &ScheduledJob{
		ID:       "job-1",
		Name:     "Test Job",
		Type:     JobTypeToolCall,
		ToolName: "test",
		Enabled:  true,
	}
	err := s.AddJob(job)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(s.ListJobs()) != 1 {
		t.Errorf("expected 1 job, got %d", len(s.ListJobs()))
	}
}

func TestAddJob_NilJob(t *testing.T) {
	registry := newMockRegistryWith("test", &mockSchedulerTool{name: "test"})
	s := NewDefaultScheduler(registry)

	err := s.AddJob(nil)
	if err == nil {
		t.Errorf("expected error for nil job")
	}
}

func TestAddJob_EmptyID(t *testing.T) {
	registry := newMockRegistryWith("test", &mockSchedulerTool{name: "test"})
	s := NewDefaultScheduler(registry)

	err := s.AddJob(&ScheduledJob{ID: ""})
	if err == nil {
		t.Errorf("expected error for empty ID")
	}
}

func TestAddJob_DuplicateID(t *testing.T) {
	registry := newMockRegistryWith("test", &mockSchedulerTool{name: "test"})
	s := NewDefaultScheduler(registry)

	_ = s.AddJob(&ScheduledJob{ID: "dup"})
	err := s.AddJob(&ScheduledJob{ID: "dup"})
	if err == nil {
		t.Errorf("expected error for duplicate ID")
	}
}

func TestGetJob(t *testing.T) {
	registry := newMockRegistryWith("test", &mockSchedulerTool{name: "test"})
	s := NewDefaultScheduler(registry)

	_ = s.AddJob(&ScheduledJob{ID: "job-1", Name: "Test"})

	job, err := s.GetJob("job-1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if job.Name != "Test" {
		t.Errorf("expected name 'Test', got '%s'", job.Name)
	}
}

func TestGetJob_NotFound(t *testing.T) {
	registry := newMockRegistryWith("test", &mockSchedulerTool{name: "test"})
	s := NewDefaultScheduler(registry)

	_, err := s.GetJob("nonexistent")
	if err == nil {
		t.Errorf("expected error for missing job")
	}
}

func TestRemoveJob(t *testing.T) {
	registry := newMockRegistryWith("test", &mockSchedulerTool{name: "test"})
	s := NewDefaultScheduler(registry)

	_ = s.AddJob(&ScheduledJob{ID: "job-1"})
	err := s.RemoveJob("job-1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(s.ListJobs()) != 0 {
		t.Errorf("expected 0 jobs after remove, got %d", len(s.ListJobs()))
	}
}

func TestRemoveJob_NotFound(t *testing.T) {
	registry := newMockRegistryWith("test", &mockSchedulerTool{name: "test"})
	s := NewDefaultScheduler(registry)

	err := s.RemoveJob("nonexistent")
	if err == nil {
		t.Errorf("expected error for removing non-existent job")
	}
}

func TestPauseResumeJob(t *testing.T) {
	registry := newMockRegistryWith("test", &mockSchedulerTool{name: "test"})
	s := NewDefaultScheduler(registry)

	_ = s.AddJob(&ScheduledJob{ID: "job-1", Enabled: true})

	err := s.PauseJob("job-1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	job, _ := s.GetJob("job-1")
	if job.Enabled {
		t.Errorf("expected job to be paused")
	}

	err = s.ResumeJob("job-1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	job, _ = s.GetJob("job-1")
	if !job.Enabled {
		t.Errorf("expected job to be resumed")
	}
}

func TestRunJobNow_ToolCall(t *testing.T) {
	mockTool := &mockSchedulerTool{name: "k8s_status", riskLevel: tools.RiskLevelL1}
	registry := newMockRegistryWith("k8s_status", mockTool)
	s := NewDefaultScheduler(registry)

	_ = s.AddJob(&ScheduledJob{
		ID:       "job-1",
		Type:     JobTypeToolCall,
		ToolName: "k8s_status",
		Enabled:  true,
	})

	exec, err := s.RunJobNow("job-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec.Status != JobStatusCompleted {
		t.Errorf("expected completed, got %s", exec.Status)
	}
	if mockTool.execCount != 1 {
		t.Errorf("expected 1 tool execution, got %d", mockTool.execCount)
	}
}

func TestRunJobNow_NotFound(t *testing.T) {
	registry := newMockRegistryWith("test", &mockSchedulerTool{name: "test"})
	s := NewDefaultScheduler(registry)

	_, err := s.RunJobNow("nonexistent")
	if err == nil {
		t.Errorf("expected error for missing job")
	}
}

func TestRunJobNow_HealthCheck(t *testing.T) {
	mockTool := &mockSchedulerTool{name: "k8s_status", riskLevel: tools.RiskLevelL0}
	registry := newMockRegistryWith("k8s_status", mockTool)
	s := NewDefaultScheduler(registry)

	_ = s.AddJob(&ScheduledJob{
		ID:   "hc-1",
		Type: JobTypeHealthCheck,
	})

	exec, err := s.RunJobNow("hc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec.Status != JobStatusCompleted {
		t.Errorf("expected completed, got %s", exec.Status)
	}
}

// ===========================================================================
// Cron Parsing Tests
// ===========================================================================

func TestParseCronField_Asterisk(t *testing.T) {
	values, err := parseCronField("*", 0, 59)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 60 {
		t.Errorf("expected 60 values, got %d", len(values))
	}
}

func TestParseCronField_Step(t *testing.T) {
	values, err := parseCronField("*/15", 0, 59)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 4 {
		t.Errorf("expected 4 values (0,15,30,45), got %d: %v", len(values), values)
	}
}

func TestParseCronField_SingleValue(t *testing.T) {
	values, err := parseCronField("30", 0, 59)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 1 || values[0] != 30 {
		t.Errorf("expected [30], got %v", values)
	}
}

func TestParseCronField_OutOfRange(t *testing.T) {
	_, err := parseCronField("70", 0, 59)
	if err == nil {
		t.Errorf("expected error for out-of-range value")
	}
}

func TestParseCronField_InvalidFormat(t *testing.T) {
	_, err := parseCronField("abc", 0, 59)
	if err == nil {
		t.Errorf("expected error for invalid format")
	}
}

func TestParseCronField_InvalidStep(t *testing.T) {
	_, err := parseCronField("*/0", 0, 59)
	if err == nil {
		t.Errorf("expected error for zero step")
	}
}

func TestParseCronFields_FullCron(t *testing.T) {
	fields := parseCronFields("*/5 * * * *")
	if len(fields) != 5 {
		t.Errorf("expected 5 fields, got %d: %v", len(fields), fields)
	}
	if fields[0] != "*/5" {
		t.Errorf("expected '*/5', got '%s'", fields[0])
	}
}

func TestMatchesField(t *testing.T) {
	if !matchesField(5, []int{1, 5, 10}) {
		t.Errorf("5 should match [1,5,10]")
	}
	if matchesField(7, []int{1, 5, 10}) {
		t.Errorf("7 should not match [1,5,10]")
	}
}

func TestFindNextTime_SimpleCron(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	next := findNextTime([]string{"30", "14", "*", "*", "*"}, now)

	if next.IsZero() {
		t.Fatalf("expected non-zero time")
	}
	if next.Hour() != 14 || next.Minute() != 30 {
		t.Errorf("expected 14:30, got %s", next.Format("15:04"))
	}
	if !next.After(now) {
		t.Errorf("expected future time, got %s (now=%s)", next, now)
	}
}

// ===========================================================================
// SchedulerToolImpl Tests
// ===========================================================================

func TestSchedulerTool_Interface(t *testing.T) {
	s := NewDefaultScheduler(newMockRegistryWith("test", &mockSchedulerTool{name: "test"}))
	tool := NewSchedulerTool(s)

	if tool.Name() != "k8s_scheduler" {
		t.Errorf("expected 'k8s_scheduler', got '%s'", tool.Name())
	}
	if tool.RiskLevel() != tools.RiskLevelL1 {
		t.Errorf("expected L1, got %s", tool.RiskLevel())
	}
	if tool.Description() == "" {
		t.Errorf("expected non-empty description")
	}
	if tool.ParameterSchema() == nil {
		t.Errorf("expected non-nil parameter schema")
	}
}

func TestSchedulerTool_Execute_List(t *testing.T) {
	s := NewDefaultScheduler(newMockRegistryWith("test", &mockSchedulerTool{name: "test"}))
	tool := NewSchedulerTool(s)

	_ = s.AddJob(&ScheduledJob{ID: "j1", Name: "Job 1"})
	_ = s.AddJob(&ScheduledJob{ID: "j2", Name: "Job 2"})

	result, err := tool.Execute(context.Background(), map[string]any{
		"action": "list",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success")
	}
}

func TestSchedulerTool_Execute_Remove(t *testing.T) {
	s := NewDefaultScheduler(newMockRegistryWith("test", &mockSchedulerTool{name: "test"}))
	tool := NewSchedulerTool(s)

	_ = s.AddJob(&ScheduledJob{ID: "j1"})

	result, err := tool.Execute(context.Background(), map[string]any{
		"action": "remove",
		"job_id": "j1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success")
	}
	if len(s.ListJobs()) != 0 {
		t.Errorf("expected 0 jobs after remove")
	}
}

func TestSchedulerTool_Execute_InvalidAction(t *testing.T) {
	s := NewDefaultScheduler(newMockRegistryWith("test", &mockSchedulerTool{name: "test"}))
	tool := NewSchedulerTool(s)

	_, err := tool.Execute(context.Background(), map[string]any{
		"action": "invalid",
	})
	if err == nil {
		t.Errorf("expected error for invalid action")
	}
}

func TestSchedulerTool_Execute_MissingAction(t *testing.T) {
	s := NewDefaultScheduler(newMockRegistryWith("test", &mockSchedulerTool{name: "test"}))
	tool := NewSchedulerTool(s)

	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Errorf("expected error for missing action")
	}
}

// ===========================================================================
// Schedule Type Tests
// ===========================================================================

func TestCalculateNextRun_Interval(t *testing.T) {
	registry := newMockRegistryWith("test", &mockSchedulerTool{name: "test"})
	s := NewDefaultScheduler(registry)

	job := &ScheduledJob{
		ID:           "interval-job",
		ScheduleType: ScheduleInterval,
		Interval:     5 * time.Minute,
	}
	next := s.calculateNextRun(job)
	if next.IsZero() {
		t.Errorf("expected non-zero next run for interval schedule")
	}
	expected := time.Now().Add(5 * time.Minute)
	if next.Sub(expected).Abs() > time.Second {
		t.Errorf("expected ~%s, got %s", expected, next)
	}
}

func TestCalculateNextRun_Once_Past(t *testing.T) {
	registry := newMockRegistryWith("test", &mockSchedulerTool{name: "test"})
	s := NewDefaultScheduler(registry)

	job := &ScheduledJob{
		ID:           "once-job",
		ScheduleType: ScheduleOnce,
		At:           time.Now().Add(-1 * time.Hour),
	}
	next := s.calculateNextRun(job)
	if !next.IsZero() {
		t.Errorf("expected zero time for past once schedule, got %s", next)
	}
}

func TestCalculateNextRun_Once_Future(t *testing.T) {
	registry := newMockRegistryWith("test", &mockSchedulerTool{name: "test"})
	s := NewDefaultScheduler(registry)

	future := time.Now().Add(10 * time.Minute)
	job := &ScheduledJob{
		ID:           "once-future",
		ScheduleType: ScheduleOnce,
		At:           future,
	}
	next := s.calculateNextRun(job)
	if next.IsZero() {
		t.Errorf("expected non-zero time for future once schedule")
	}
	if next.Sub(future).Abs() > time.Second {
		t.Errorf("expected %s, got %s", future, next)
	}
}
