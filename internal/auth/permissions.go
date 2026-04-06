package auth

import "context"

type PermissionChecker interface {
	HasPermission(ctx context.Context, subject string, permission string, resourceID string) (bool, error)
}
