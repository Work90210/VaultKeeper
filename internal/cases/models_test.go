package cases

import (
	"testing"

	"github.com/google/uuid"
)

func TestIsValidStatusTransition(t *testing.T) {
	tests := []struct {
		from, to string
		want     bool
	}{
		{StatusActive, StatusClosed, true},
		{StatusClosed, StatusArchived, true},
		{StatusActive, StatusArchived, false},
		{StatusArchived, StatusActive, false},
		{StatusClosed, StatusActive, false},
		{StatusArchived, StatusClosed, false},
		{"invalid", StatusClosed, false},
		{StatusActive, "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.from+"->"+tt.to, func(t *testing.T) {
			if got := IsValidStatusTransition(tt.from, tt.to); got != tt.want {
				t.Errorf("IsValidStatusTransition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestClampPagination(t *testing.T) {
	tests := []struct {
		name  string
		input Pagination
		want  Pagination
	}{
		{"zero limit defaults", Pagination{Limit: 0}, Pagination{Limit: DefaultPageLimit}},
		{"negative limit defaults", Pagination{Limit: -1}, Pagination{Limit: DefaultPageLimit}},
		{"over max capped", Pagination{Limit: 500}, Pagination{Limit: MaxPageLimit}},
		{"valid unchanged", Pagination{Limit: 25}, Pagination{Limit: 25}},
		{"exactly max unchanged", Pagination{Limit: 200}, Pagination{Limit: 200}},
		{"cursor preserved", Pagination{Limit: 10, Cursor: "abc"}, Pagination{Limit: 10, Cursor: "abc"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClampPagination(tt.input)
			if got.Limit != tt.want.Limit {
				t.Errorf("Limit = %d, want %d", got.Limit, tt.want.Limit)
			}
		})
	}
}

func TestValidCaseRoles(t *testing.T) {
	valid := []string{"investigator", "prosecutor", "defence", "judge", "observer", "victim_representative"}
	for _, r := range valid {
		if !ValidCaseRoles[r] {
			t.Errorf("expected %q to be valid", r)
		}
	}
	if ValidCaseRoles["admin"] {
		t.Error("admin should not be a valid case role")
	}
}

func TestEncodeCursor_Roundtrip(t *testing.T) {
	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	cursor := EncodeCursor(id)
	if cursor == "" {
		t.Fatal("expected non-empty cursor")
	}

	decoded, err := decodeCursor(cursor)
	if err != nil {
		t.Fatalf("decodeCursor error: %v", err)
	}
	if decoded != id {
		t.Errorf("roundtrip failed: got %s, want %s", decoded, id)
	}
}

func TestDecodeCursor_Invalid(t *testing.T) {
	if _, err := decodeCursor("!!!not-base64!!!"); err == nil {
		t.Error("expected error for invalid base64")
	}
	if _, err := decodeCursor("bm90LWEtdXVpZA"); err == nil {
		t.Error("expected error for invalid UUID")
	}
}
