package search

// Coverage tests for the local classification access matrix that
// filters search hits before they leave the server.

import (
	"testing"
)

func TestUserSideForRole(t *testing.T) {
	cases := []struct {
		role, want string
	}{
		{"investigator", "prosecution"},
		{"prosecutor", "prosecution"},
		{"defence", "defence"},
		{"judge", ""},
		{"observer", ""},
		{"victim_representative", ""},
		{"", ""},
		{"alien", ""},
	}
	for _, tc := range cases {
		if got := userSideForRole(tc.role); got != tc.want {
			t.Errorf("userSideForRole(%q) = %q, want %q", tc.role, got, tc.want)
		}
	}
}

func TestIsKnownRole(t *testing.T) {
	for _, r := range []string{"investigator", "prosecutor", "defence", "judge", "observer", "victim_representative"} {
		if !isKnownRole(r) {
			t.Errorf("isKnownRole(%q) = false, want true", r)
		}
	}
	for _, r := range []string{"", "unknown", "ADMIN", "Defence"} {
		if isKnownRole(r) {
			t.Errorf("isKnownRole(%q) = true, want false", r)
		}
	}
}

func TestCheckAccess_Matrix(t *testing.T) {
	pros := "prosecution"
	def := "defence"
	cases := []struct {
		name     string
		role     string
		class    string
		side     *string
		want     bool
	}{
		// public / restricted: any known role allowed
		{"public/judge", "judge", "public", nil, true},
		{"public/unknown", "alien", "public", nil, false},
		{"restricted/observer", "observer", "restricted", nil, true},
		// confidential
		{"confidential/investigator", "investigator", "confidential", nil, true},
		{"confidential/prosecutor", "prosecutor", "confidential", nil, true},
		{"confidential/judge", "judge", "confidential", nil, true},
		{"confidential/defence", "defence", "confidential", nil, false},
		{"confidential/observer", "observer", "confidential", nil, false},
		// ex_parte prosecution
		{"exparte_pros/investigator", "investigator", "ex_parte", &pros, true},
		{"exparte_pros/prosecutor", "prosecutor", "ex_parte", &pros, true},
		{"exparte_pros/defence", "defence", "ex_parte", &pros, false},
		{"exparte_pros/judge", "judge", "ex_parte", &pros, true},
		// ex_parte defence
		{"exparte_def/defence", "defence", "ex_parte", &def, true},
		{"exparte_def/prosecutor", "prosecutor", "ex_parte", &def, false},
		{"exparte_def/judge", "judge", "ex_parte", &def, true},
		// ex_parte with nil side
		{"exparte_nilside", "judge", "ex_parte", nil, false},
		// ex_parte with unknown side value (falls through)
		{"exparte_unknownside", "prosecutor", "ex_parte", strPtr("phantom"), false},
		// unknown classification
		{"phantom_class", "judge", "top_secret", nil, false},
	}
	for _, tc := range cases {
		got := checkAccess(tc.role, tc.class, tc.side, userSideForRole(tc.role))
		if got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func strPtr(s string) *string { return &s }

func TestFilterHitsByAccess(t *testing.T) {
	caseIDAccessible := "case-1"
	caseIDNoRole := "case-2"
	pros := "prosecution"
	input := EvidenceSearchResult{
		Hits: []EvidenceSearchHit{
			{EvidenceID: "1", CaseID: caseIDAccessible, Classification: "restricted"},
			{EvidenceID: "2", CaseID: caseIDNoRole, Classification: "restricted"},
			{EvidenceID: "3", CaseID: caseIDAccessible, Classification: "confidential"},
			{EvidenceID: "4", CaseID: caseIDAccessible, Classification: "ex_parte", ExParteSide: &pros},
			{EvidenceID: "5", CaseID: caseIDAccessible, Classification: ""},
		},
		Query:            "test",
		ProcessingTimeMs: 42,
	}
	roles := map[string]string{
		caseIDAccessible: "investigator",
		// caseIDNoRole deliberately missing
	}
	out := filterHitsByAccess(input, roles)
	// Hits 1, 3, 4, 5 (investigator sees all, default=restricted) — 4 total.
	// Hit 2 dropped (no role).
	if len(out.Hits) != 4 {
		t.Errorf("len = %d, want 4", len(out.Hits))
	}
	if out.TotalHits != 4 {
		t.Errorf("TotalHits = %d, want 4", out.TotalHits)
	}
	if out.Query != input.Query {
		t.Errorf("query mismatch")
	}
}
