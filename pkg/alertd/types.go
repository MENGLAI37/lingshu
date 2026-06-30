package alertd

import (
	"time"
)

type AlertSeverity string

const (
	SeverityCritical AlertSeverity = "critical"
	SeverityHigh     AlertSeverity = "high"
	SeverityWarning  AlertSeverity = "warning"
	SeverityInfo     AlertSeverity = "info"
)

type AlertStatus string

const (
	StatusFiring   AlertStatus = "firing"
	StatusResolved AlertStatus = "resolved"
)

type AlertSource string

const (
	SourceAlertManager AlertSource = "alertmanager"
	SourcePagerDuty    AlertSource = "pagerduty"
	SourceGeneric      AlertSource = "generic"
)

type Alert struct {
	ID            string            `json:"id"`
	Fingerprint   string            `json:"fingerprint"`
	Source        AlertSource       `json:"source"`
	Status        AlertStatus       `json:"status"`
	Severity      AlertSeverity     `json:"severity"`
	Labels        map[string]string `json:"labels"`
	Annotations   map[string]string `json:"annotations"`
	StartsAt      time.Time         `json:"starts_at"`
	EndsAt        time.Time         `json:"ends_at,omitempty"`
	GeneratorURL  string            `json:"generator_url,omitempty"`
	Receiver      string            `json:"receiver,omitempty"`
	Cluster       string            `json:"cluster,omitempty"`
	Namespace     string            `json:"namespace,omitempty"`
	ResourceName  string            `json:"resource_name,omitempty"`
	ResourceKind  string            `json:"resource_kind,omitempty"`
	ReceivedAt    time.Time         `json:"received_at"`
}

type AlertManagerWebhook struct {
	Version           string            `json:"version"`
	GroupKey          string            `json:"groupKey"`
	Status            string            `json:"status"`
	Receiver          string            `json:"receiver"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string            `json:"externalURL"`
	Alerts            []struct {
		Status       string            `json:"status"`
		Labels       map[string]string `json:"labels"`
		Annotations  map[string]string `json:"annotations"`
		StartsAt     time.Time         `json:"startsAt"`
		EndsAt       time.Time         `json:"endsAt"`
		GeneratorURL string            `json:"generatorURL"`
		Fingerprint  string            `json:"fingerprint"`
	} `json:"alerts"`
}

type PagerDutyWebhook struct {
	EventID    string `json:"event_id"`
	EventType  string `json:"event_type"`
	DataVersion string `json:"data_version"`
	Data       struct {
		ID          string `json:"id"`
		Type        string `json:"type"`
		Summary     string `json:"summary"`
		Self        string `json:"self"`
		HTMLURL     string `json:"html_url"`
		IncidentKey string `json:"incident_key"`
		Service     struct {
			ID      string `json:"id"`
			Type    string `json:"type"`
			Summary string `json:"summary"`
			Self    string `json:"self"`
			HTMLURL string `json:"html_url"`
		} `json:"service"`
		Urgency string `json:"urgency"`
		Status  string `json:"status"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	} `json:"data"`
}

type GenericAlertWebhook struct {
	ID           string            `json:"id"`
	Status       string            `json:"status"`
	Severity     string            `json:"severity"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     string            `json:"starts_at"`
	EndsAt       string            `json:"ends_at,omitempty"`
	GeneratorURL string            `json:"generator_url,omitempty"`
	Cluster      string            `json:"cluster,omitempty"`
	Namespace    string            `json:"namespace,omitempty"`
}

type AlertHandler func(alert *Alert) error

type WebhookResponse struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	AlertCount int    `json:"alert_count,omitempty"`
}
