package evidence

import (
	"errors"
	"testing"
)

// TestClassificationAccessMatrix covers every cell of the Sprint 9 access
// matrix: 6 roles × 5 effective states (public, restricted, confidential,
// ex_parte/prosecution, ex_parte/defence) = 30 combinations. The spec text
// mentions "24 combinations" counting ex_parte once; this test expands
// ex_parte into its two sides and asserts each separately so defence never
// sees prosecution's ex_parte items and vice versa.
func TestClassificationAccessMatrix(t *testing.T) {
	pros := ExPartePros
	def := ExParteDefence

	cases := []struct {
		name           string
		role           string
		classification string
		exParteSide    *string
		want           bool
	}{
		// --- public: all roles allowed ---
		{"public/investigator", RoleInvestigator, ClassificationPublic, nil, true},
		{"public/prosecutor", RoleProsecutor, ClassificationPublic, nil, true},
		{"public/defence", RoleDefence, ClassificationPublic, nil, true},
		{"public/judge", RoleJudge, ClassificationPublic, nil, true},
		{"public/observer", RoleObserver, ClassificationPublic, nil, true},
		{"public/victim_rep", RoleVictimRep, ClassificationPublic, nil, true},

		// --- restricted: all roles allowed (disclosure filter applied at repo) ---
		{"restricted/investigator", RoleInvestigator, ClassificationRestricted, nil, true},
		{"restricted/prosecutor", RoleProsecutor, ClassificationRestricted, nil, true},
		{"restricted/defence", RoleDefence, ClassificationRestricted, nil, true},
		{"restricted/judge", RoleJudge, ClassificationRestricted, nil, true},
		{"restricted/observer", RoleObserver, ClassificationRestricted, nil, true},
		{"restricted/victim_rep", RoleVictimRep, ClassificationRestricted, nil, true},

		// --- confidential: investigator, prosecutor, judge only ---
		{"confidential/investigator", RoleInvestigator, ClassificationConfidential, nil, true},
		{"confidential/prosecutor", RoleProsecutor, ClassificationConfidential, nil, true},
		{"confidential/defence", RoleDefence, ClassificationConfidential, nil, false},
		{"confidential/judge", RoleJudge, ClassificationConfidential, nil, true},
		{"confidential/observer", RoleObserver, ClassificationConfidential, nil, false},
		{"confidential/victim_rep", RoleVictimRep, ClassificationConfidential, nil, false},

		// --- ex_parte (prosecution) ---
		{"exparte_pros/investigator", RoleInvestigator, ClassificationExParte, &pros, true},
		{"exparte_pros/prosecutor", RoleProsecutor, ClassificationExParte, &pros, true},
		{"exparte_pros/defence", RoleDefence, ClassificationExParte, &pros, false},
		{"exparte_pros/judge", RoleJudge, ClassificationExParte, &pros, true},
		{"exparte_pros/observer", RoleObserver, ClassificationExParte, &pros, false},
		{"exparte_pros/victim_rep", RoleVictimRep, ClassificationExParte, &pros, false},

		// --- ex_parte (defence) ---
		{"exparte_def/investigator", RoleInvestigator, ClassificationExParte, &def, false},
		{"exparte_def/prosecutor", RoleProsecutor, ClassificationExParte, &def, false},
		{"exparte_def/defence", RoleDefence, ClassificationExParte, &def, true},
		{"exparte_def/judge", RoleJudge, ClassificationExParte, &def, true},
		{"exparte_def/observer", RoleObserver, ClassificationExParte, &def, false},
		{"exparte_def/victim_rep", RoleVictimRep, ClassificationExParte, &def, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CheckAccess(tc.role, tc.classification, tc.exParteSide, UserSideForRole(tc.role))
			if got != tc.want {
				t.Errorf("CheckAccess(%s, %s, %v) = %v, want %v",
					tc.role, tc.classification, tc.exParteSide, got, tc.want)
			}
		})
	}
}

// TestClassificationAccessEdgeCases covers malformed/unknown inputs.
func TestClassificationAccessEdgeCases(t *testing.T) {
	bogus := "neutral"
	t.Run("unknown classification", func(t *testing.T) {
		if CheckAccess(RoleJudge, "phantom", nil, "") {
			t.Error("unknown classification must deny")
		}
	})
	t.Run("unknown role on public", func(t *testing.T) {
		if CheckAccess("mystery_role", ClassificationPublic, nil, "") {
			t.Error("unknown role must deny even on public")
		}
	})
	t.Run("ex_parte without side", func(t *testing.T) {
		if CheckAccess(RoleProsecutor, ClassificationExParte, nil, ExPartePros) {
			t.Error("ex_parte with nil side must deny")
		}
	})
	t.Run("ex_parte with invalid side", func(t *testing.T) {
		if CheckAccess(RoleProsecutor, ClassificationExParte, &bogus, ExPartePros) {
			t.Error("ex_parte with unknown side must deny")
		}
	})
}

func TestValidateClassificationChange(t *testing.T) {
	pros := ExPartePros
	def := ExParteDefence
	empty := ""
	bogus := "aliens"

	type tc struct {
		name    string
		class   string
		side    *string
		wantErr bool
		field   string
	}
	tcs := []tc{
		{"public ok", ClassificationPublic, nil, false, ""},
		{"restricted ok", ClassificationRestricted, nil, false, ""},
		{"confidential ok", ClassificationConfidential, nil, false, ""},
		{"ex_parte prosecution ok", ClassificationExParte, &pros, false, ""},
		{"ex_parte defence ok", ClassificationExParte, &def, false, ""},

		{"unknown classification", "phantom", nil, true, "classification"},
		{"ex_parte nil side", ClassificationExParte, nil, true, "ex_parte_side"},
		{"ex_parte empty side", ClassificationExParte, &empty, true, "ex_parte_side"},
		{"ex_parte invalid side", ClassificationExParte, &bogus, true, "ex_parte_side"},
		{"public with side", ClassificationPublic, &pros, true, "ex_parte_side"},
		{"confidential with side", ClassificationConfidential, &def, true, "ex_parte_side"},
	}

	for _, c := range tcs {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateClassificationChange(c.class, c.side)
			if c.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				var ve *ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("expected *ValidationError, got %T", err)
				}
				if ve.Field != c.field {
					t.Errorf("field = %q, want %q", ve.Field, c.field)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestUserSideForRole(t *testing.T) {
	tests := []struct {
		role string
		want string
	}{
		{RoleInvestigator, ExPartePros},
		{RoleProsecutor, ExPartePros},
		{RoleDefence, ExParteDefence},
		{RoleJudge, ""},
		{RoleObserver, ""},
		{RoleVictimRep, ""},
		{"stranger", ""},
	}
	for _, tt := range tests {
		if got := UserSideForRole(tt.role); got != tt.want {
			t.Errorf("UserSideForRole(%q) = %q, want %q", tt.role, got, tt.want)
		}
	}
}
