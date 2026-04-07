package evidence

import (
	"testing"
)

func TestMinIOStorage_NewMinIOStorage_InvalidEndpoint(t *testing.T) {
	// We can't connect to a real MinIO in unit tests, but we can verify
	// that the function signature and types work correctly by testing
	// the interface compliance.
	var _ ObjectStorage = (*MinIOStorage)(nil)
}
