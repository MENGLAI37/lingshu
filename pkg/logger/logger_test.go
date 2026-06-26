package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestInit(t *testing.T) {
	Init("debug", "json")
	if defaultLogger == nil {
		t.Error("Expected defaultLogger to be initialized")
	}
}

func TestWithTraceID(t *testing.T) {
	ctx := context.Background()
	ctx = WithTraceID(ctx, "trace-123")

	traceID, ok := ctx.Value(TraceIDKey).(string)
	if !ok || traceID != "trace-123" {
		t.Errorf("Expected trace_id trace-123, got %v", traceID)
	}
}

func TestWithSessionID(t *testing.T) {
	ctx := context.Background()
	ctx = WithSessionID(ctx, "session-456")

	sessionID, ok := ctx.Value(SessionIDKey).(string)
	if !ok || sessionID != "session-456" {
		t.Errorf("Expected session_id session-456, got %v", sessionID)
	}
}

func TestFromContext(t *testing.T) {
	Init("debug", "json")
	ctx := context.Background()
	ctx = WithTraceID(ctx, "trace-123")
	ctx = WithSessionID(ctx, "session-456")

	logger := FromContext(ctx)
	if logger == nil {
		t.Error("Expected logger from context")
	}
}

func TestJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	testLogger := slog.New(handler)

	testLogger.Info("test message", "key", "value")

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if result["msg"] != "test message" {
		t.Errorf("Expected msg 'test message', got %v", result["msg"])
	}
	if result["key"] != "value" {
		t.Errorf("Expected key 'value', got %v", result["key"])
	}
}
