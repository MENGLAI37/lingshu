package scheduler

import (
	"time"

	"github.com/lingshu/lingshu/pkg/agent"
)

type ScheduleType string

const (
	ScheduleCron     ScheduleType = "cron"
	ScheduleInterval ScheduleType = "interval"
	ScheduleOnce     ScheduleType = "once"
)

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
	JobStatusPaused    JobStatus = "paused"
)

type JobType string

const (
	JobTypeToolCall    JobType = "tool_call"
	JobTypeWorkFlow    JobType = "workflow"
	JobTypeHealthCheck JobType = "health_check"
)

type ScheduledJob struct {
	ID          string
	Name        string
	Description string
	Type        JobType
	ScheduleType ScheduleType
	CronExpr    string
	Interval    time.Duration
	At          time.Time
	ToolName    string
	ToolParams  map[string]interface{}
	WorkFlowID  string
	WorkFlowParams map[string]interface{}
	Enabled     bool
	LastRun     time.Time
	LastStatus  JobStatus
	LastError   string
	NextRun     time.Time
}

type JobExecution struct {
	ID        string
	JobID     string
	Status    JobStatus
	StartTime time.Time
	EndTime   time.Time
	Error     string
	Result    interface{}
}

type Scheduler interface {
	AddJob(job *ScheduledJob) error
	RemoveJob(jobID string) error
	GetJob(jobID string) (*ScheduledJob, error)
	ListJobs() []*ScheduledJob
	Start() error
	Stop() error
	PauseJob(jobID string) error
	ResumeJob(jobID string) error
	RunJobNow(jobID string) (*JobExecution, error)
}

type SchedulerToolInterface interface {
	agent.ToolRegistry
	GetScheduler() Scheduler
}
