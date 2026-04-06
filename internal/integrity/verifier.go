package integrity

import "context"

type HashVerifier interface {
	VerifyHash(ctx context.Context, algorithm string, expected string, actual string) error
}
