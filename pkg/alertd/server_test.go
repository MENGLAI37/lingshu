package alertd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lingshu/lingshu/pkg/logger"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()

	s := &Server{
		host:        "127.0.0.1",
		port:        0,
		router:      http.NewServeMux(),
		alertQueue:  make(chan *Alert, 1000),
		workerCount: 1,
		stopCh:      make(chan struct{}),
	}

	s.registerRoutes()

	t.Cleanup(func() {
		_ = s.Stop()
	})

	return s
}

func TestHealthEndpoint(t *testing.T) {
	logger.Init("debug", "text")
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "ok", response["status"])
	assert.Equal(t, "alertd", response["service"])
}

func TestHealthEndpointWrongMethod(t *testing.T) {
	logger.Init("debug", "text")
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestGenericAlertWebhook(t *testing.T) {
	logger.Init("debug", "text")
	s := newTestServer(t)

	webhook := GenericAlertWebhook{
		ID:       "test-alert-001",
		Status:   "firing",
		Severity: "critical",
		Labels: map[string]string{
			"alertname": "TestAlert",
			"instance":  "localhost",
		},
		Annotations: map[string]string{
			"summary": "Test alert summary",
		},
		StartsAt:  time.Now().Format(time.RFC3339),
		Cluster:   "test-cluster",
		Namespace: "test-ns",
	}

	body, err := json.Marshal(webhook)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response WebhookResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", response.Status)
	assert.Equal(t, 1, response.AlertCount)

	assert.Equal(t, 1, s.GetQueueSize())
}

func TestGenericAlertWebhookInvalidJSON(t *testing.T) {
	logger.Init("debug", "text")
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAlertManagerWebhook(t *testing.T) {
	logger.Init("debug", "text")
	s := newTestServer(t)

	webhook := AlertManagerWebhook{
		Version:  "4",
		GroupKey: "test-group",
		Status:   "firing",
		Receiver: "test-receiver",
		GroupLabels: map[string]string{
			"alertname": "TestAlert",
		},
		CommonLabels: map[string]string{
			"cluster": "test-cluster",
		},
		CommonAnnotations: map[string]string{
			"summary": "Common summary",
		},
		ExternalURL: "http://alertmanager.example.com",
		Alerts: []struct {
			Status       string            `json:"status"`
			Labels       map[string]string `json:"labels"`
			Annotations  map[string]string `json:"annotations"`
			StartsAt     time.Time         `json:"startsAt"`
			EndsAt       time.Time         `json:"endsAt"`
			GeneratorURL string            `json:"generatorURL"`
			Fingerprint  string            `json:"fingerprint"`
		}{
			{
				Status: "firing",
				Labels: map[string]string{
					"alertname": "HighCPU",
					"severity":  "critical",
					"cluster":   "test-cluster",
					"namespace": "test-ns",
					"pod":       "test-pod-1",
				},
				Annotations: map[string]string{
					"summary": "High CPU usage",
				},
				StartsAt:     time.Now(),
				GeneratorURL: "http://prometheus.example.com",
				Fingerprint:  "fingerprint-1",
			},
			{
				Status: "firing",
				Labels: map[string]string{
					"alertname":  "HighMemory",
					"severity":   "warning",
					"cluster":    "test-cluster",
					"namespace":  "test-ns",
					"deployment": "test-deployment",
				},
				Annotations: map[string]string{
					"summary": "High memory usage",
				},
				StartsAt:     time.Now(),
				GeneratorURL: "http://prometheus.example.com",
				Fingerprint:  "fingerprint-2",
			},
		},
	}

	body, err := json.Marshal(webhook)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/alertmanager", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response WebhookResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", response.Status)
	assert.Equal(t, 2, response.AlertCount)

	assert.Equal(t, 2, s.GetQueueSize())
}

func TestPagerDutyWebhook(t *testing.T) {
	logger.Init("debug", "text")
	s := newTestServer(t)

	webhook := PagerDutyWebhook{
		EventID:     "pd-event-001",
		EventType:   "incident.triggered",
		DataVersion: "3",
		Data: struct {
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
			Urgency   string `json:"urgency"`
			Status    string `json:"status"`
			CreatedAt string `json:"created_at"`
			UpdatedAt string `json:"updated_at"`
		}{
			ID:          "pd-incident-001",
			Type:        "incident",
			Summary:     "Test incident",
			Self:        "https://api.pagerduty.com/incidents/pd-incident-001",
			HTMLURL:     "https://pagerduty.com/incidents/pd-incident-001",
			IncidentKey: "test-incident-key",
			Urgency:     "high",
			Status:      "triggered",
			CreatedAt:   time.Now().Format(time.RFC3339),
		},
	}

	webhook.Data.Service.ID = "service-001"
	webhook.Data.Service.Type = "service"
	webhook.Data.Service.Summary = "Test Service"
	webhook.Data.Service.Self = "https://api.pagerduty.com/services/service-001"
	webhook.Data.Service.HTMLURL = "https://pagerduty.com/services/service-001"

	body, err := json.Marshal(webhook)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/pagerduty", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response WebhookResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", response.Status)
	assert.Equal(t, 1, response.AlertCount)

	assert.Equal(t, 1, s.GetQueueSize())
}

func TestConvertGenericAlert(t *testing.T) {
	logger.Init("debug", "text")
	s := newTestServer(t)

	webhook := &GenericAlertWebhook{
		ID:       "test-id",
		Status:   "resolved",
		Severity: "info",
		Cluster:  "test-cluster",
	}

	alert := s.convertGenericAlert(webhook)

	assert.Equal(t, "test-id", alert.ID)
	assert.Equal(t, SourceGeneric, alert.Source)
	assert.Equal(t, StatusResolved, alert.Status)
	assert.Equal(t, SeverityInfo, alert.Severity)
	assert.Equal(t, "test-cluster", alert.Cluster)
	assert.False(t, alert.ReceivedAt.IsZero())
}

func TestConvertGenericAlertDefaults(t *testing.T) {
	logger.Init("debug", "text")
	s := newTestServer(t)

	webhook := &GenericAlertWebhook{}

	alert := s.convertGenericAlert(webhook)

	assert.NotEmpty(t, alert.ID)
	assert.Equal(t, StatusFiring, alert.Status)
	assert.Equal(t, SeverityWarning, alert.Severity)
	assert.NotNil(t, alert.Labels)
	assert.NotNil(t, alert.Annotations)
}

func TestConvertAlertManagerAlerts(t *testing.T) {
	logger.Init("debug", "text")
	s := newTestServer(t)

	webhook := &AlertManagerWebhook{
		Receiver: "test-receiver",
		Alerts: []struct {
			Status       string            `json:"status"`
			Labels       map[string]string `json:"labels"`
			Annotations  map[string]string `json:"annotations"`
			StartsAt     time.Time         `json:"startsAt"`
			EndsAt       time.Time         `json:"endsAt"`
			GeneratorURL string            `json:"generatorURL"`
			Fingerprint  string            `json:"fingerprint"`
		}{
			{
				Status: "firing",
				Labels: map[string]string{
					"severity":  "critical",
					"cluster":   "test-cluster",
					"namespace": "test-ns",
					"pod":       "my-pod",
				},
				Fingerprint: "fp1",
			},
		},
	}

	alerts := s.convertAlertManagerAlerts(webhook)

	require.Len(t, alerts, 1)
	alert := alerts[0]

	assert.Equal(t, SourceAlertManager, alert.Source)
	assert.Equal(t, StatusFiring, alert.Status)
	assert.Equal(t, SeverityCritical, alert.Severity)
	assert.Equal(t, "test-cluster", alert.Cluster)
	assert.Equal(t, "test-ns", alert.Namespace)
	assert.Equal(t, "my-pod", alert.ResourceName)
	assert.Equal(t, "Pod", alert.ResourceKind)
	assert.Equal(t, "test-receiver", alert.Receiver)
}

func TestConvertPagerDutyAlert(t *testing.T) {
	logger.Init("debug", "text")
	s := newTestServer(t)

	webhook := &PagerDutyWebhook{
		EventID:   "evt-001",
		EventType: "incident.triggered",
		Data: struct {
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
			Urgency   string `json:"urgency"`
			Status    string `json:"status"`
			CreatedAt string `json:"created_at"`
			UpdatedAt string `json:"updated_at"`
		}{
			ID:          "inc-001",
			Summary:     "Test PagerDuty incident",
			HTMLURL:     "https://pagerduty.example.com/incidents/inc-001",
			IncidentKey: "inc-key-001",
			Urgency:     "high",
			Status:      "triggered",
		},
	}

	webhook.Data.Service.ID = "svc-001"
	webhook.Data.Service.Summary = "Test Service"

	alert := s.convertPagerDutyAlert(webhook)

	assert.Equal(t, "inc-001", alert.ID)
	assert.Equal(t, SourcePagerDuty, alert.Source)
	assert.Equal(t, StatusFiring, alert.Status)
	assert.Equal(t, SeverityCritical, alert.Severity)
	assert.Equal(t, "Test PagerDuty incident", alert.Annotations["summary"])
	assert.Equal(t, "inc-key-001", alert.Labels["pagerduty_incident_key"])
}

func TestRegisterHandler(t *testing.T) {
	logger.Init("debug", "text")
	s := newTestServer(t)

	var processed int64

	handler := func(alert *Alert) error {
		atomic.AddInt64(&processed, 1)
		return nil
	}

	s.RegisterHandler(handler)

	s.stopCh = make(chan struct{})
	s.running = true
	s.wg.Add(1)
	go s.processAlerts(context.TODO())

	alert := &Alert{
		ID:       "test-alert",
		Source:   SourceGeneric,
		Status:   StatusFiring,
		Severity: SeverityWarning,
	}

	s.enqueueAlert(alert)

	time.Sleep(100 * time.Millisecond)

	close(s.stopCh)
	s.wg.Wait()
	s.running = false

	assert.Equal(t, int64(1), atomic.LoadInt64(&processed))
}

func TestEnqueueNilAlert(t *testing.T) {
	logger.Init("debug", "text")
	s := newTestServer(t)

	initialSize := s.GetQueueSize()
	s.enqueueAlert(nil)

	assert.Equal(t, initialSize, s.GetQueueSize())
}

func TestWrongMethodEndpoints(t *testing.T) {
	logger.Init("debug", "text")
	s := newTestServer(t)

	endpoints := []string{
		"/api/v1/alerts",
		"/api/v1/webhook/alertmanager",
		"/api/v1/webhook/pagerduty",
	}

	for _, endpoint := range endpoints {
		req := httptest.NewRequest(http.MethodGet, endpoint, nil)
		w := httptest.NewRecorder()

		s.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code,
			"endpoint %s should return MethodNotAllowed for GET", endpoint)
	}
}
