package custody

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type CustodyLogger interface {
	Record(ctx context.Context, event Event) error
}

type Logger struct {
	repo *PGRepository
}

func NewLogger(repo *PGRepository) *Logger {
	return &Logger{repo: repo}
}

func (l *Logger) Record(ctx context.Context, event Event) error {
	return l.repo.Insert(ctx, event)
}

func (l *Logger) RecordCaseEvent(ctx context.Context, caseID uuid.UUID, action string, actorUserID string, detail map[string]string) error {
	// json.Marshal on map[string]string is infallible in the Go standard library;
	// the error return is intentionally not checked here.
	detailJSON, _ := json.Marshal(detail)

	return l.repo.Insert(ctx, Event{
		CaseID:      caseID,
		EvidenceID:  uuid.Nil,
		Action:      action,
		ActorUserID: actorUserID,
		Detail:      string(detailJSON),
		Timestamp:   time.Now().UTC(),
	})
}
