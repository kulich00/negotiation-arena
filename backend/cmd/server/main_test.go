package main

import (
	"context"
	"log/slog"
	"testing"
)

func TestNewLoggerLevels(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		configured   string
		debugEnabled bool
		infoEnabled  bool
		warnEnabled  bool
	}{
		{configured: "debug", debugEnabled: true, infoEnabled: true, warnEnabled: true},
		{configured: "warn", warnEnabled: true},
		{configured: "warning", warnEnabled: true},
		{configured: "error"},
		{configured: "unknown", infoEnabled: true, warnEnabled: true},
	}

	for _, test := range tests {
		t.Run(test.configured, func(t *testing.T) {
			handler := newLogger(test.configured).Handler()
			if got := handler.Enabled(ctx, slog.LevelDebug); got != test.debugEnabled {
				t.Fatalf("debug enabled = %v, want %v", got, test.debugEnabled)
			}
			if got := handler.Enabled(ctx, slog.LevelInfo); got != test.infoEnabled {
				t.Fatalf("info enabled = %v, want %v", got, test.infoEnabled)
			}
			if got := handler.Enabled(ctx, slog.LevelWarn); got != test.warnEnabled {
				t.Fatalf("warn enabled = %v, want %v", got, test.warnEnabled)
			}
			if !handler.Enabled(ctx, slog.LevelError) {
				t.Fatal("error logging must always be enabled")
			}
		})
	}
}
