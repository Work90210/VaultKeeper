package custody

import (
	"context"
	"encoding/json"
	"fmt"
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
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("marshal custody detail: %w", err)
	}

	return l.repo.Insert(ctx, Event{
		CaseID:      caseID,
		EvidenceID:  uuid.Nil,
		Action:      action,
		ActorUserID: actorUserID,
		Detail:      string(detailJSON),
		Timestamp:   time.Now().UTC(),
	})
}
