package reports

import "context"

type ReportGenerator interface {
	GeneratePDF(ctx context.Context, reportID string) ([]byte, error)
}
