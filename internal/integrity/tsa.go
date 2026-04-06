package integrity

import "context"

type TimestampAuthority interface {
	IssueTimestamp(ctx context.Context, digest []byte) (token []byte, err error)
	VerifyTimestamp(ctx context.Context, token []byte) error
}
