package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lingshu/lingshu/pkg/agent"
	"github.com/lingshu/lingshu/pkg/tools"
)

type DefaultScheduler struct {
	jobs       map[string]*ScheduledJob
	toolRegistry agent.ToolRegistry
	mu         sync.RWMutex
	running    bool
	ticker     *time.Ticker
	cancel     context.CancelFunc
}

func NewDefaultScheduler(toolRegistry agent.ToolRegistry) *DefaultScheduler {
	return &DefaultScheduler{
		jobs:       map[string]*ScheduledJob{},
		toolRegistry: toolRegistry,
		running:    false,
	}
}

func (s *DefaultScheduler) AddJob(job *ScheduledJob) error {
	if job == nil || job.ID == "" {
		return fmt.Errorf("job must have an ID")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.jobs[job.ID]; ok {
		return fmt.Errorf("job %s already exists", job.ID)
	}

	job.NextRun = s.calculateNextRun(job)
	s.jobs[job.ID] = job

	return nil
}

func (s *DefaultScheduler) RemoveJob(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.jobs[jobID]; !ok {
		return fmt.Errorf("job %s not found", jobID)
	}

	delete(s.jobs, jobID)
	return nil
}

func (s *DefaultScheduler) GetJob(jobID string) (*ScheduledJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, ok := s.jobs[jobID]
	if !ok {
		return nil, fmt.Errorf("job %s not found", jobID)
	}

	return job, nil
}

func (s *DefaultScheduler) ListJobs() []*ScheduledJob {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := []*ScheduledJob{}
	for _, job := range s.jobs {
		result = append(result, job)
	}

	return result
}

func (s *DefaultScheduler) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("scheduler is already running")
	}

	s.running = true
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	s.ticker = time.NewTicker(30 * time.Second)
	s.mu.Unlock()

	go s.run(ctx)
	return nil
}

func (s *DefaultScheduler) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("scheduler is not running")
	}

	s.running = false
	s.cancel()
	if s.ticker != nil {
		s.ticker.Stop()
	}

	return nil
}

func (s *DefaultScheduler) PauseJob(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[jobID]
	if !ok {
		return fmt.Errorf("job %s not found", jobID)
	}

	job.Enabled = false
	return nil
}

func (s *DefaultScheduler) ResumeJob(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[jobID]
	if !ok {
		return fmt.Errorf("job %s not found", jobID)
	}

	job.Enabled = true
	job.NextRun = s.calculateNextRun(job)
	return nil
}

func (s *DefaultScheduler) RunJobNow(jobID string) (*JobExecution, error) {
	s.mu.RLock()
	job, ok := s.jobs[jobID]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("job %s not found", jobID)
	}

	return s.executeJob(job), nil
}

func (s *DefaultScheduler) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.ticker.C:
			s.checkAndRunJobs()
		}
	}
}

func (s *DefaultScheduler) checkAndRunJobs() {
	s.mu.RLock()
	jobs := make([]*ScheduledJob, 0, len(s.jobs))
	for _, job := range s.jobs {
		if job.Enabled && !job.NextRun.IsZero() && time.Now().After(job.NextRun) {
			jobs = append(jobs, job)
		}
	}
	s.mu.RUnlock()

	for _, job := range jobs {
		go func(j *ScheduledJob) {
			result := s.executeJob(j)
			s.mu.Lock()
			j.LastRun = time.Now()
			j.LastStatus = result.Status
			j.LastError = result.Error
			j.NextRun = s.calculateNextRun(j)
			s.mu.Unlock()
		}(job)
	}
}

func (s *DefaultScheduler) executeJob(job *ScheduledJob) *JobExecution {
	exec := &JobExecution{
		ID:        fmt.Sprintf("exec_%d", time.Now().UnixNano()),
		JobID:     job.ID,
		StartTime: time.Now(),
		Status:    JobStatusRunning,
	}

	defer func() {
		exec.EndTime = time.Now()
	}()

	switch job.Type {
	case JobTypeToolCall:
		err := s.executeToolCall(job)
		if err != nil {
			exec.Status = JobStatusFailed
			exec.Error = err.Error()
		} else {
			exec.Status = JobStatusCompleted
		}
	case JobTypeWorkFlow:
		err := s.executeWorkFlow(job)
		if err != nil {
			exec.Status = JobStatusFailed
			exec.Error = err.Error()
		} else {
			exec.Status = JobStatusCompleted
		}
	case JobTypeHealthCheck:
		err := s.executeHealthCheck(job)
		if err != nil {
			exec.Status = JobStatusFailed
			exec.Error = err.Error()
		} else {
			exec.Status = JobStatusCompleted
		}
	default:
		exec.Status = JobStatusFailed
		exec.Error = fmt.Sprintf("unknown job type %s", job.Type)
	}

	return exec
}

func (s *DefaultScheduler) executeToolCall(job *ScheduledJob) error {
	tool, err := s.toolRegistry.GetTool(job.ToolName)
	if err != nil {
		return fmt.Errorf("tool %s not found: %w", job.ToolName, err)
	}

	_, err = tool.Execute(context.Background(), job.ToolParams)
	return err
}

func (s *DefaultScheduler) executeWorkFlow(job *ScheduledJob) error {
	return fmt.Errorf("workflow execution not implemented in scheduler")
}

func (s *DefaultScheduler) executeHealthCheck(job *ScheduledJob) error {
	tool, err := s.toolRegistry.GetTool("k8s_status")
	if err != nil {
		return fmt.Errorf("status tool not found: %w", err)
	}

	_, err = tool.Execute(context.Background(), map[string]any{})
	return err
}

func (s *DefaultScheduler) calculateNextRun(job *ScheduledJob) time.Time {
	now := time.Now()

	switch job.ScheduleType {
	case ScheduleInterval:
		if job.LastRun.IsZero() {
			return now.Add(job.Interval)
		}
		return job.LastRun.Add(job.Interval)
	case ScheduleOnce:
		if job.At.After(now) {
			return job.At
		}
		return time.Time{}
	case ScheduleCron:
		return s.parseCronAndGetNext(job.CronExpr, now)
	default:
		return time.Time{}
	}
}

func (s *DefaultScheduler) parseCronAndGetNext(cronExpr string, now time.Time) time.Time {
	fields := parseCronFields(cronExpr)
	if len(fields) != 5 {
		return time.Time{}
	}

	return findNextTime(fields, now)
}

func parseCronFields(expr string) []string {
	result := []string{}
	current := ""
	for _, ch := range expr {
		if ch == ' ' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func findNextTime(fields []string, now time.Time) time.Time {
	minute, err := parseCronField(fields[0], 0, 59)
	if err != nil {
		return time.Time{}
	}
	hour, err := parseCronField(fields[1], 0, 23)
	if err != nil {
		return time.Time{}
	}
	day, err := parseCronField(fields[2], 1, 31)
	if err != nil {
		return time.Time{}
	}
	month, err := parseCronField(fields[3], 1, 12)
	if err != nil {
		return time.Time{}
	}
	weekday, err := parseCronField(fields[4], 0, 6)
	if err != nil {
		return time.Time{}
	}

	for i := 0; i < 366; i++ {
		candidate := now.Add(time.Duration(i) * 24 * time.Hour)
		candidate = time.Date(candidate.Year(), candidate.Month(), candidate.Day(), 0, 0, 0, 0, candidate.Location())

		if !matchesField(int(candidate.Month()), month) {
			continue
		}
		if !matchesField(candidate.Day(), day) {
			continue
		}
		if !matchesField(int(candidate.Weekday()), weekday) {
			continue
		}

		for h := 0; h < 24; h++ {
			if !matchesField(h, hour) {
				continue
			}

			for m := 0; m < 60; m++ {
				if !matchesField(m, minute) {
					continue
				}

				target := candidate.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute)
				if target.After(now) {
					return target
				}
			}
		}
	}

	return time.Time{}
}

func parseCronField(field string, min, max int) ([]int, error) {
	if field == "*" {
		result := []int{}
		for i := min; i <= max; i++ {
			result = append(result, i)
		}
		return result, nil
	}

	if len(field) > 2 && field[:2] == "*/" {
		step := 0
		fmt.Sscanf(field[2:], "%d", &step)
		if step <= 0 {
			return nil, fmt.Errorf("invalid step value")
		}
		result := []int{}
		for i := min; i <= max; i += step {
			result = append(result, i)
		}
		return result, nil
	}

	var value int
	n, _ := fmt.Sscanf(field, "%d", &value)
	if n == 1 {
		if value < min || value > max {
			return nil, fmt.Errorf("value out of range")
		}
		return []int{value}, nil
	}

	return nil, fmt.Errorf("invalid field format")
}

func matchesField(value int, allowed []int) bool {
	for _, v := range allowed {
		if v == value {
			return true
		}
	}
	return false
}

type SchedulerToolImpl struct {
	scheduler *DefaultScheduler
}

func NewSchedulerTool(scheduler *DefaultScheduler) *SchedulerToolImpl {
	return &SchedulerToolImpl{scheduler: scheduler}
}

func (t *SchedulerToolImpl) Name() string {
	return "k8s_scheduler"
}

func (t *SchedulerToolImpl) RiskLevel() tools.ToolRiskLevel {
	return tools.RiskLevelL1
}

func (t *SchedulerToolImpl) Description() string {
	return "Manage scheduled jobs for periodic health checks and maintenance"
}

func (t *SchedulerToolImpl) ParameterSchema() map[string]interface{} {
	return map[string]interface{}{
		"action": map[string]interface{}{
			"type":        "string",
			"enum":        []string{"list", "add", "remove", "run_now", "pause", "resume"},
			"description": "Action to perform",
		},
		"job_id": map[string]interface{}{
			"type":        "string",
			"description": "Job ID for remove/run/pause/resume actions",
		},
		"job": map[string]interface{}{
			"type":        "object",
			"description": "Job definition for add action",
		},
	}
}

func (t *SchedulerToolImpl) Execute(ctx context.Context, params map[string]any) (*tools.ToolResult, error) {
	action, ok := params["action"].(string)
	if !ok {
		return nil, fmt.Errorf("action is required")
	}

	switch action {
	case "list":
		jobs := t.scheduler.ListJobs()
		return &tools.ToolResult{
			Data:    jobs,
			Message: fmt.Sprintf("Found %d scheduled jobs", len(jobs)),
		}, nil
	case "add":
		jobData, ok := params["job"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("job definition is required")
		}

		job := &ScheduledJob{
			ID:   fmt.Sprintf("%v", jobData["id"]),
			Name: fmt.Sprintf("%v", jobData["name"]),
			Type: JobType(fmt.Sprintf("%v", jobData["type"])),
		}

		if cron, ok := jobData["cron"].(string); ok {
			job.ScheduleType = ScheduleCron
			job.CronExpr = cron
		} else if interval, ok := jobData["interval"].(float64); ok {
			job.ScheduleType = ScheduleInterval
			job.Interval = time.Duration(interval) * time.Minute
		}

		if params, ok := jobData["params"].(map[string]interface{}); ok {
			job.ToolParams = params
		}

		if toolName, ok := jobData["tool_name"].(string); ok {
			job.ToolName = toolName
		}

		job.Enabled = true
		err := t.scheduler.AddJob(job)
		if err != nil {
			return nil, err
		}
		return &tools.ToolResult{
			Data:    job,
			Message: fmt.Sprintf("Job %s added successfully", job.ID),
		}, nil
	case "remove":
		jobID, ok := params["job_id"].(string)
		if !ok {
			return nil, fmt.Errorf("job_id is required")
		}
		err := t.scheduler.RemoveJob(jobID)
		if err != nil {
			return nil, err
		}
		return &tools.ToolResult{
			Message: fmt.Sprintf("Job %s removed", jobID),
		}, nil
	case "run_now":
		jobID, ok := params["job_id"].(string)
		if !ok {
			return nil, fmt.Errorf("job_id is required")
		}
		exec, err := t.scheduler.RunJobNow(jobID)
		if err != nil {
			return nil, err
		}
		return &tools.ToolResult{
			Data:    exec,
			Message: fmt.Sprintf("Job %s executed. Status: %s", jobID, exec.Status),
		}, nil
	case "pause":
		jobID, ok := params["job_id"].(string)
		if !ok {
			return nil, fmt.Errorf("job_id is required")
		}
		err := t.scheduler.PauseJob(jobID)
		if err != nil {
			return nil, err
		}
		return &tools.ToolResult{
			Message: fmt.Sprintf("Job %s paused", jobID),
		}, nil
	case "resume":
		jobID, ok := params["job_id"].(string)
		if !ok {
			return nil, fmt.Errorf("job_id is required")
		}
		err := t.scheduler.ResumeJob(jobID)
		if err != nil {
			return nil, err
		}
		return &tools.ToolResult{
			Message: fmt.Sprintf("Job %s resumed", jobID),
		}, nil
	default:
		return nil, fmt.Errorf("unknown action %s", action)
	}
}

func (t *SchedulerToolImpl) GetScheduler() Scheduler {
	return t.scheduler
}

func (t *SchedulerToolImpl) GetTool(name string) (tools.Tool, error) {
	return nil, fmt.Errorf("not implemented")
}

func (t *SchedulerToolImpl) ListTools() []tools.Tool {
	return nil
}

func (t *SchedulerToolImpl) RegisterTool(tool tools.Tool) error {
	return fmt.Errorf("not implemented")
}