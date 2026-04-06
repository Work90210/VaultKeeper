package backup

import "context"

type BackupRunner interface {
	Run(ctx context.Context) error
}
