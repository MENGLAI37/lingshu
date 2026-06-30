package alertd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/lingshu/lingshu/pkg/config"
	"github.com/lingshu/lingshu/pkg/logger"
)

const (
	DefaultHost = "0.0.0.0"
	DefaultPort = 8081
	DefaultReadTimeout = 10 * time.Second
	DefaultWriteTimeout = 10 * time.Second
	DefaultAlertPath     = "/api/v1/alerts"
	DefaultAlertManagerPath = "/api/v1/webhook/alertmanager"
	DefaultPagerDutyPath    = "/api/v1/webhook/pagerduty"
	DefaultHealthPath       = "/healthz"
)

type Server struct {
	httpServer *http.Server
	router     *http.ServeMux
	host       string
	port       int

	handlers     []AlertHandler
	handlersMu   sync.RWMutex

	alertQueue   chan *Alert
	workerCount  int
	stopCh       chan struct{}
	wg           sync.WaitGroup
	running      bool
	mu           sync.Mutex

	totalAlerts  int64
	processedAlerts int64
	failedAlerts int64
}

var (
	instance *Server
)

func Init(cfg *config.Config) (*Server, error) {
	host := DefaultHost
	port := DefaultPort

	if cfg != nil {
		if cfg.Server.Host != "" {
			host = cfg.Server.Host
		}
		if cfg.Server.Port > 0 {
			port = cfg.Server.Port
		}
	}

	s := &Server{
		host:       host,
		port:       port,
		router:     http.NewServeMux(),
		alertQueue: make(chan *Alert, 10000),
		workerCount: 4,
		stopCh:     make(chan struct{}),
	}

	s.registerRoutes()

	instance = s

	logger.Info("Alertd server initialized",
		"host", host,
		"port", port,
	)

	return s, nil
}

func Get() *Server {
	return instance
}

func (s *Server) registerRoutes() {
	s.router.HandleFunc(DefaultHealthPath, s.handleHealth)
	s.router.HandleFunc(DefaultAlertPath, s.handleGenericAlert)
	s.router.HandleFunc(DefaultAlertManagerPath, s.handleAlertManager)
	s.router.HandleFunc(DefaultPagerDutyPath, s.handlePagerDuty)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"service": "alertd",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleGenericAlert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	defer r.Body.Close()

	var webhook GenericAlertWebhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid JSON format")
		return
	}

	alert := s.convertGenericAlert(&webhook)
	s.enqueueAlert(alert)

	s.writeSuccess(w, "alert received", 1)
}

func (s *Server) handleAlertManager(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	defer r.Body.Close()

	var webhook AlertManagerWebhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid AlertManager webhook format")
		return
	}

	alerts := s.convertAlertManagerAlerts(&webhook)
	for i := range alerts {
		s.enqueueAlert(&alerts[i])
	}

	s.writeSuccess(w, "alerts received", len(alerts))
}

func (s *Server) handlePagerDuty(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	defer r.Body.Close()

	var webhook PagerDutyWebhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid PagerDuty webhook format")
		return
	}

	alert := s.convertPagerDutyAlert(&webhook)
	s.enqueueAlert(alert)

	s.writeSuccess(w, "alert received", 1)
}

func (s *Server) convertGenericAlert(webhook *GenericAlertWebhook) *Alert {
	alertID := webhook.ID
	if alertID == "" {
		alertID = uuid.New().String()
	}

	status := StatusFiring
	if webhook.Status == "resolved" {
		status = StatusResolved
	}

	severity := SeverityWarning
	switch webhook.Severity {
	case "critical":
		severity = SeverityCritical
	case "high":
		severity = SeverityHigh
	case "warning":
		severity = SeverityWarning
	case "info":
		severity = SeverityInfo
	}

	startsAt := time.Now()
	if webhook.StartsAt != "" {
		if t, err := time.Parse(time.RFC3339, webhook.StartsAt); err == nil {
			startsAt = t
		}
	}

	endsAt := time.Time{}
	if webhook.EndsAt != "" {
		if t, err := time.Parse(time.RFC3339, webhook.EndsAt); err == nil {
			endsAt = t
		}
	}

	labels := webhook.Labels
	if labels == nil {
		labels = map[string]string{}
	}

	annotations := webhook.Annotations
	if annotations == nil {
		annotations = map[string]string{}
	}

	return &Alert{
		ID:           alertID,
		Fingerprint:  alertID,
		Source:       SourceGeneric,
		Status:       status,
		Severity:     severity,
		Labels:       labels,
		Annotations:  annotations,
		StartsAt:     startsAt,
		EndsAt:       endsAt,
		GeneratorURL: webhook.GeneratorURL,
		Cluster:      webhook.Cluster,
		Namespace:    webhook.Namespace,
		ReceivedAt:   time.Now(),
	}
}

func (s *Server) convertAlertManagerAlerts(webhook *AlertManagerWebhook) []Alert {
	alerts := make([]Alert, 0, len(webhook.Alerts))

	for _, amAlert := range webhook.Alerts {
		alertID := amAlert.Fingerprint
		if alertID == "" {
			alertID = uuid.New().String()
		}

		status := StatusFiring
		if amAlert.Status == "resolved" {
			status = StatusResolved
		}

		labels := amAlert.Labels
		if labels == nil {
			labels = map[string]string{}
		}

		annotations := amAlert.Annotations
		if annotations == nil {
			annotations = map[string]string{}
		}

		severity := SeverityWarning
		if sev, ok := labels["severity"]; ok {
			switch sev {
			case "critical":
				severity = SeverityCritical
			case "high":
				severity = SeverityHigh
			case "warning":
				severity = SeverityWarning
			case "info":
				severity = SeverityInfo
			}
		}

		cluster := labels["cluster"]
		namespace := labels["namespace"]
		resourceName := labels["pod"]
		if resourceName == "" {
			resourceName = labels["deployment"]
		}
		resourceKind := ""
		if labels["pod"] != "" {
			resourceKind = "Pod"
		} else if labels["deployment"] != "" {
			resourceKind = "Deployment"
		}

		alert := Alert{
			ID:           alertID,
			Fingerprint:  amAlert.Fingerprint,
			Source:       SourceAlertManager,
			Status:       status,
			Severity:     severity,
			Labels:       labels,
			Annotations:  annotations,
			StartsAt:     amAlert.StartsAt,
			EndsAt:       amAlert.EndsAt,
			GeneratorURL: amAlert.GeneratorURL,
			Receiver:     webhook.Receiver,
			Cluster:      cluster,
			Namespace:    namespace,
			ResourceName: resourceName,
			ResourceKind: resourceKind,
			ReceivedAt:   time.Now(),
		}

		alerts = append(alerts, alert)
	}

	return alerts
}

func (s *Server) convertPagerDutyAlert(webhook *PagerDutyWebhook) *Alert {
	alertID := webhook.Data.ID
	if alertID == "" {
		alertID = uuid.New().String()
	}

	status := StatusFiring
	switch webhook.Data.Status {
	case "resolved":
		status = StatusResolved
	case "acknowledged":
		status = StatusFiring
	case "triggered":
		status = StatusFiring
	}

	severity := SeverityHigh
	switch webhook.Data.Urgency {
	case "high":
		severity = SeverityCritical
	case "low":
		severity = SeverityWarning
	}

	startsAt := time.Now()
	if webhook.Data.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, webhook.Data.CreatedAt); err == nil {
			startsAt = t
		}
	}

	labels := map[string]string{
		"pagerduty_incident_key": webhook.Data.IncidentKey,
		"pagerduty_service":      webhook.Data.Service.Summary,
		"pagerduty_service_id":   webhook.Data.Service.ID,
	}

	annotations := map[string]string{
		"summary":   webhook.Data.Summary,
		"pd_url":    webhook.Data.HTMLURL,
		"event_id":  webhook.EventID,
		"event_type": webhook.EventType,
	}

	return &Alert{
		ID:           alertID,
		Fingerprint:  alertID,
		Source:       SourcePagerDuty,
		Status:       status,
		Severity:     severity,
		Labels:       labels,
		Annotations:  annotations,
		StartsAt:     startsAt,
		GeneratorURL: webhook.Data.HTMLURL,
		ReceivedAt:   time.Now(),
	}
}

func (s *Server) enqueueAlert(alert *Alert) {
	if alert == nil {
		return
	}

	select {
	case s.alertQueue <- alert:
		logger.Debug("Alert enqueued",
			"id", alert.ID,
			"source", alert.Source,
			"severity", alert.Severity,
		)
	default:
		logger.Warn("Alert queue full, dropping alert",
			"id", alert.ID,
			"source", alert.Source,
			"queue_size", len(s.alertQueue),
		)
	}
}

func (s *Server) processAlerts(ctx context.Context) {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopCh:
			return
		case alert := <-s.alertQueue:
			s.processAlert(ctx, alert)
		}
	}
}

func (s *Server) processAlert(ctx context.Context, alert *Alert) {
	s.handlersMu.RLock()
	handlers := make([]AlertHandler, len(s.handlers))
	copy(handlers, s.handlers)
	s.handlersMu.RUnlock()

	for _, handler := range handlers {
		if err := handler(alert); err != nil {
			logger.Error("Alert handler failed",
				"error", err,
				"alert_id", alert.ID,
				"source", alert.Source,
			)
		}
	}
}

func (s *Server) RegisterHandler(handler AlertHandler) {
	s.handlersMu.Lock()
	defer s.handlersMu.Unlock()
	s.handlers = append(s.handlers, handler)
}

func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("alertd server already running")
	}

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  DefaultReadTimeout,
		WriteTimeout: DefaultWriteTimeout,
	}

	s.stopCh = make(chan struct{})
	s.running = true

	for i := 0; i < s.workerCount; i++ {
		s.wg.Add(1)
		go s.processAlerts(context.Background())
	}

	go func() {
		logger.Info("Alertd server starting", "addr", addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Alertd server error", "error", err)
		}
	}()

	return nil
}

func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	close(s.stopCh)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		logger.Warn("Alertd server shutdown error", "error", err)
	}

	s.wg.Wait()
	s.running = false

	logger.Info("Alertd server stopped")
	return nil
}

func (s *Server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *Server) GetAddr() string {
	return fmt.Sprintf("%s:%d", s.host, s.port)
}

func (s *Server) GetQueueSize() int {
	return len(s.alertQueue)
}

func (s *Server) writeSuccess(w http.ResponseWriter, message string, alertCount int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(WebhookResponse{
		Status:     "success",
		Message:    message,
		AlertCount: alertCount,
	})
}

func (s *Server) writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(WebhookResponse{
		Status:  "error",
		Message: message,
	})
}
