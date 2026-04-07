package notifications

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// UserEmailResolver resolves a user's email address from their ID.
// This is optional; when nil, email delivery is skipped (in-app notifications
// still work). A concrete implementation would call the Keycloak Admin API.
type UserEmailResolver interface {
	GetUserEmail(ctx context.Context, userID string) (string, error)
}

// Service coordinates notification creation, routing, and retrieval.
type Service struct {
	repo          *Repository
	emailer       *EmailSender
	emailResolver UserEmailResolver
	logger        *slog.Logger
	adminUserIDs  []string
}

// NewService creates a notification Service.
// adminUserIDs is a static list of system-admin user IDs used for routing
// events that target administrators (e.g. integrity warnings, backup failures).
// emailResolver may be nil; when nil, email delivery is skipped.
func NewService(repo *Repository, emailer *EmailSender, emailResolver UserEmailResolver, logger *slog.Logger, adminUserIDs []string) *Service {
	ids := make([]string, len(adminUserIDs))
	copy(ids, adminUserIDs)
	return &Service{
		repo:          repo,
		emailer:       emailer,
		emailResolver: emailResolver,
		logger:        logger,
		adminUserIDs:  ids,
	}
}

// Notify determines recipients for an event, persists notifications, and
// enqueues email delivery.
func (s *Service) Notify(ctx context.Context, event NotificationEvent) error {
	recipients, err := s.resolveRecipients(ctx, event)
	if err != nil {
		return fmt.Errorf("resolve recipients: %w", err)
	}

	if len(recipients) == 0 {
		s.logger.Warn("no recipients for notification event",
			"type", event.Type, "case_id", event.CaseID)
		return nil
	}

	for _, userID := range recipients {
		uid, parseErr := uuid.Parse(userID)
		if parseErr != nil {
			s.logger.Error("invalid recipient user ID", "user_id", userID, "error", parseErr)
			continue
		}

		n := Notification{
			ID:        uuid.New(),
			UserID:    uid,
			Type:      event.Type,
			Title:     event.Title,
			Body:      event.Body,
			Read:      false,
			CreatedAt: time.Now().UTC(),
		}

		if event.CaseID != uuid.Nil {
			caseID := event.CaseID
			n.CaseID = &caseID
		}

		if err := s.repo.Create(ctx, n); err != nil {
			s.logger.Error("failed to create notification",
				"user_id", userID, "type", event.Type, "error", err)
			continue
		}

		// Fire-and-forget email; only attempt if we can resolve user emails.
		if s.emailResolver != nil {
			email, resolveErr := s.emailResolver.GetUserEmail(ctx, userID)
			if resolveErr != nil {
				s.logger.Warn("could not resolve email for user, skipping email delivery",
					"user_id", userID, "error", resolveErr)
			} else {
				_ = s.emailer.Send(ctx, email, event.Title, event.Body, event.Body)
			}
		}
	}

	return nil
}

// GetUserNotifications returns paginated notifications for a user.
func (s *Service) GetUserNotifications(ctx context.Context, userID string, limit int, cursor string) ([]Notification, int, error) {
	items, total, err := s.repo.ListByUser(ctx, userID, limit, cursor)
	if err != nil {
		return nil, 0, fmt.Errorf("list user notifications: %w", err)
	}
	return items, total, nil
}

// MarkRead marks a single notification as read.
func (s *Service) MarkRead(ctx context.Context, id uuid.UUID, userID string) error {
	if err := s.repo.MarkRead(ctx, id, userID); err != nil {
		return fmt.Errorf("mark read: %w", err)
	}
	return nil
}

// MarkAllRead marks all of a user's unread notifications as read.
func (s *Service) MarkAllRead(ctx context.Context, userID string) error {
	if err := s.repo.MarkAllRead(ctx, userID); err != nil {
		return fmt.Errorf("mark all read: %w", err)
	}
	return nil
}

// GetUnreadCount returns the number of unread notifications for a user.
func (s *Service) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	count, err := s.repo.GetUnreadCount(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("get unread count: %w", err)
	}
	return count, nil
}

// resolveRecipients determines who should receive a notification based on the
// event type.
func (s *Service) resolveRecipients(ctx context.Context, event NotificationEvent) ([]string, error) {
	switch event.Type {
	case EventEvidenceUploaded, EventLegalHoldChanged:
		return s.repo.GetCaseUserIDs(ctx, event.CaseID)

	case EventUserAddedToCase:
		if event.TargetUserID == "" {
			return nil, fmt.Errorf("user_added_to_case event missing TargetUserID")
		}
		return []string{event.TargetUserID}, nil

	case EventIntegrityWarning:
		// System admins plus case-specific users (admins of the case).
		caseUsers, err := s.repo.GetCaseUserIDs(ctx, event.CaseID)
		if err != nil {
			return nil, fmt.Errorf("get case users for integrity warning: %w", err)
		}
		return deduplicateStrings(append(s.adminUserIDs, caseUsers...)), nil

	case EventRetentionExpiring, EventBackupFailed:
		return s.adminUserIDs, nil

	default:
		s.logger.Warn("unknown notification event type", "type", event.Type)
		return nil, nil
	}
}

// deduplicateStrings returns a new slice with duplicates removed, preserving order.
func deduplicateStrings(ss []string) []string {
	seen := make(map[string]struct{}, len(ss))
	result := make([]string, 0, len(ss))
	for _, s := range ss {
		if _, exists := seen[s]; exists {
			continue
		}
		seen[s] = struct{}{}
		result = append(result, s)
	}
	return result
}
