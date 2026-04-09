package witnesses

import "testing"

func TestClampPagination(t *testing.T) {
	tests := []struct {
		name  string
		input Pagination
		want  int
	}{
		{"zero defaults", Pagination{Limit: 0}, DefaultPageLimit},
		{"negative defaults", Pagination{Limit: -1}, DefaultPageLimit},
		{"over max capped", Pagination{Limit: 500}, MaxPageLimit},
		{"valid unchanged", Pagination{Limit: 25}, 25},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClampPagination(tt.input)
			if got.Limit != tt.want {
				t.Errorf("got %d, want %d", got.Limit, tt.want)
			}
		})
	}
}

func TestValidProtectionStatuses(t *testing.T) {
	valid := []string{"standard", "protected", "high_risk"}
	for _, s := range valid {
		if !ValidProtectionStatuses[s] {
			t.Errorf("%q should be valid", s)
		}
	}
	if ValidProtectionStatuses["invalid"] {
		t.Error("invalid should not be valid")
	}
}
