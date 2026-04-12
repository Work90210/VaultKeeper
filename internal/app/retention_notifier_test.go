package app

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/evidence"
)

func TestLoggingRetentionNotifier_EmitsWarning(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	n := &LoggingRetentionNotifier{Logger: logger}

	item := evidence.ExpiringRetentionItem{
		EvidenceID:     uuid.New(),
		CaseID:         uuid.New(),
		EvidenceNumber: "ICC-01/04-01/07-00042",
		RetentionUntil: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
	}

	if err := n.NotifyRetentionExpiring(context.Background(), item); err != nil {
		t.Fatalf("want nil, got %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "retention expiring") {
		t.Errorf("log missing message: %s", out)
	}
	if !strings.Contains(out, item.EvidenceID.String()) {
		t.Errorf("log missing evidence_id: %s", out)
	}
	if !strings.Contains(out, "ICC-01/04-01/07-00042") {
		t.Errorf("log missing evidence_number: %s", out)
	}
}

func TestLoggingRetentionNotifier_NilReceiver(t *testing.T) {
	var n *LoggingRetentionNotifier
	if err := n.NotifyRetentionExpiring(context.Background(), evidence.ExpiringRetentionItem{}); err != nil {
		t.Errorf("nil receiver must be a no-op, got %v", err)
	}
}

func TestLoggingRetentionNotifier_NilLogger(t *testing.T) {
	n := &LoggingRetentionNotifier{Logger: nil}
	if err := n.NotifyRetentionExpiring(context.Background(), evidence.ExpiringRetentionItem{}); err != nil {
		t.Errorf("nil logger must be a no-op, got %v", err)
	}
}
