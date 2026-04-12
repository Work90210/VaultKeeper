package evidence

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
)

func ptrTime(t time.Time) *time.Time { return &t }

func TestEffectiveRetention(t *testing.T) {
	base := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	later := base.Add(30 * 24 * time.Hour)

	tests := []struct {
		name       string
		itemUntil  *time.Time
		caseUntil  *time.Time
		want       *time.Time
	}{
		{"both nil", nil, nil, nil},
		{"item only", ptrTime(base), nil, ptrTime(base)},
		{"case only", nil, ptrTime(base), ptrTime(base)},
		{"item later", ptrTime(later), ptrTime(base), ptrTime(later)},
		{"case later", ptrTime(base), ptrTime(later), ptrTime(later)},
		{"equal", ptrTime(base), ptrTime(base), ptrTime(base)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EffectiveRetention(tc.itemUntil, tc.caseUntil)
			switch {
			case tc.want == nil && got != nil:
				t.Fatalf("want nil, got %v", got)
			case tc.want != nil && got == nil:
				t.Fatalf("want %v, got nil", *tc.want)
			case tc.want != nil && got != nil && !got.Equal(*tc.want):
				t.Fatalf("want %v, got %v", *tc.want, *got)
			}
		})
	}
}

func TestCheckRetention(t *testing.T) {
	now := time.Date(2030, 6, 1, 0, 0, 0, 0, time.UTC)
	past := now.Add(-1 * time.Hour)
	future := now.Add(24 * time.Hour)

	t.Run("no retention allows destruction", func(t *testing.T) {
		err := CheckRetention(EvidenceItem{}, nil, now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("past retention allows destruction", func(t *testing.T) {
		item := EvidenceItem{RetentionUntil: ptrTime(past)}
		if err := CheckRetention(item, nil, now); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("future retention blocks destruction", func(t *testing.T) {
		item := EvidenceItem{RetentionUntil: ptrTime(future)}
		err := CheckRetention(item, nil, now)
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrRetentionActive) {
			t.Errorf("expected ErrRetentionActive, got %v", err)
		}
	})

	t.Run("case retention in future blocks even when item retention is nil", func(t *testing.T) {
		err := CheckRetention(EvidenceItem{}, ptrTime(future), now)
		if !errors.Is(err, ErrRetentionActive) {
			t.Errorf("expected ErrRetentionActive, got %v", err)
		}
	})

	t.Run("item past but case future blocks (MAX rules)", func(t *testing.T) {
		item := EvidenceItem{RetentionUntil: ptrTime(past)}
		err := CheckRetention(item, ptrTime(future), now)
		if !errors.Is(err, ErrRetentionActive) {
			t.Errorf("expected ErrRetentionActive, got %v", err)
		}
	})
}

// fakeRetentionRepo is a minimal Repository that also satisfies
// RetentionRepository. We reuse mockRepo for everything else and just
// delegate FindExpiringRetention here.
type fakeRetentionRepo struct {
	*mockRepo
	expiring []ExpiringRetentionItem
	findErr  error
}

func (f *fakeRetentionRepo) FindExpiringRetention(_ context.Context, _ time.Time) ([]ExpiringRetentionItem, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.expiring, nil
}

type fakeRetentionNotifier struct {
	notified []ExpiringRetentionItem
	err      error
}

func (f *fakeRetentionNotifier) NotifyRetentionExpiring(_ context.Context, item ExpiringRetentionItem) error {
	if f.err != nil {
		return f.err
	}
	f.notified = append(f.notified, item)
	return nil
}

func TestService_NotifyExpiringRetention(t *testing.T) {
	ctx := context.Background()
	caseID := uuid.New()

	expiring := []ExpiringRetentionItem{
		{EvidenceID: uuid.New(), CaseID: caseID, EvidenceNumber: "CASE-00001", RetentionUntil: time.Now().Add(10 * 24 * time.Hour)},
		{EvidenceID: uuid.New(), CaseID: caseID, EvidenceNumber: "CASE-00002", RetentionUntil: time.Now().Add(25 * 24 * time.Hour)},
	}

	repo := &fakeRetentionRepo{mockRepo: newMockRepo(), expiring: expiring}
	notifier := &fakeRetentionNotifier{}

	svc := &Service{
		repo:   repo,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	svc.WithRetentionNotifier(notifier)

	count, err := svc.NotifyExpiringRetention(ctx, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
	if len(notifier.notified) != 2 {
		t.Errorf("notified = %d, want 2", len(notifier.notified))
	}
}

func TestService_NotifyExpiringRetention_FindError(t *testing.T) {
	repo := &fakeRetentionRepo{mockRepo: newMockRepo(), findErr: errors.New("db down")}
	svc := &Service{
		repo:   repo,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	_, err := svc.NotifyExpiringRetention(context.Background(), 30*24*time.Hour)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestService_NotifyExpiringRetention_RepoNotSupported(t *testing.T) {
	// Base mockRepo does not implement RetentionRepository.
	svc := &Service{
		repo:   newMockRepo(),
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	_, err := svc.NotifyExpiringRetention(context.Background(), 30*24*time.Hour)
	if err == nil {
		t.Fatal("expected error when repo does not implement RetentionRepository")
	}
}
