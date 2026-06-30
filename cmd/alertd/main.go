package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/lingshu/lingshu/pkg/alertd"
	"github.com/lingshu/lingshu/pkg/config"
	"github.com/lingshu/lingshu/pkg/logger"
)

func main() {
	logger.Init("info", "json")

	cfg, err := config.Load("")
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	server, err := alertd.Init(cfg)
	if err != nil {
		logger.Error("Failed to initialize alertd server", "error", err)
		os.Exit(1)
	}

	server.RegisterHandler(func(alert *alertd.Alert) error {
		logger.Info("Received alert",
			"id", alert.ID,
			"source", alert.Source,
			"severity", alert.Severity,
			"status", alert.Status,
			"cluster", alert.Cluster,
			"namespace", alert.Namespace,
		)
		return nil
	})

	if err := server.Start(); err != nil {
		logger.Error("Failed to start alertd server", "error", err)
		os.Exit(1)
	}

	fmt.Println("lingshu alertd - Alert Webhook Server")
	fmt.Printf("Listening on %s\n", server.GetAddr())
	fmt.Printf("Health check: http://%s/healthz\n", server.GetAddr())
	fmt.Printf("AlertManager webhook: http://%s/api/v1/webhook/alertmanager\n", server.GetAddr())
	fmt.Printf("PagerDuty webhook: http://%s/api/v1/webhook/pagerduty\n", server.GetAddr())
	fmt.Printf("Generic webhook: http://%s/api/v1/alerts\n", server.GetAddr())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Info("Received signal, shutting down", "signal", sig.String())
	case <-ctx.Done():
	}

	if err := server.Stop(); err != nil {
		logger.Error("Error stopping alertd server", "error", err)
	}

	logger.Info("Alertd server stopped gracefully")
}
