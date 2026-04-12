package evidence

import (
	"fmt"
	"strings"
)

// Case role constants used by the classification access matrix. These mirror
// the values in cases.ValidCaseRoles but are re-declared here to avoid a
// package cycle (the evidence package must not depend on cases).
const (
	RoleInvestigator = "investigator"
	RoleProsecutor   = "prosecutor"
	RoleDefence      = "defence"
	RoleJudge        = "judge"
	RoleObserver     = "observer"
	RoleVictimRep    = "victim_representative"
)

// Ex parte side constants.
const (
	ExPartePros    = "prosecution"
	ExParteDefence = "defence"
)

// validExParteSides is the set of allowed ex_parte_side values.
// Unexported so other packages cannot mutate its membership. Use
// IsValidExParteSide for cross-package lookups.
var validExParteSides = map[string]bool{
	ExPartePros:    true,
	ExParteDefence: true,
}

// IsValidExParteSide reports whether s is a recognised ex_parte side
// value (prosecution, defence).
func IsValidExParteSide(s string) bool {
	return validExParteSides[s]
}

// CheckAccess returns true if a user with the given role may access an
// evidence item of the given classification. When classification is
// "ex_parte", exParteSide must be non-nil and userSide ("prosecution" or
// "defence") is used to decide visibility for defence/prosecutor roles.
//
// The matrix (spec Sprint 9 Step 1):
//
//	classification          | invest | pros | def | judge | obs | victim
//	public                  |   x    |  x   |  x  |   x   |  x  |   x
//	restricted              |   x    |  x   |  x  |   x   |  x  |   x
//	confidential            |   x    |  x   |  -  |   x   |  -  |   -
//	ex_parte(prosecution)   |   x    |  x   |  -  |   x   |  -  |   -
//	ex_parte(defence)       |   -    |  -   |  x  |   x   |  -  |   -
//
// Disclosure-gating for defence on "restricted" items is enforced separately
// at the repository layer (via the disclosures join); CheckAccess only
// evaluates the classification/role/side dimensions.
func CheckAccess(role, classification string, exParteSide *string, userSide string) bool {
	switch classification {
	case ClassificationPublic, ClassificationRestricted:
		// All case roles may access.
		return isKnownRole(role)

	case ClassificationConfidential:
		switch role {
		case RoleInvestigator, RoleProsecutor, RoleJudge:
			return true
		default:
			return false
		}

	case ClassificationExParte:
		if exParteSide == nil {
			// Ex parte without a side is a data-integrity error; deny.
			return false
		}
		side := *exParteSide
		// Judges always see both sides.
		if role == RoleJudge {
			return side == ExPartePros || side == ExParteDefence
		}
		switch side {
		case ExPartePros:
			// Prosecution side: investigator + prosecutor only.
			return role == RoleInvestigator || role == RoleProsecutor
		case ExParteDefence:
			// Defence side: defence only.
			return role == RoleDefence
		default:
			return false
		}
	}

	return false
}

// isKnownRole reports whether a role is one of the six documented case roles.
func isKnownRole(role string) bool {
	switch role {
	case RoleInvestigator, RoleProsecutor, RoleDefence,
		RoleJudge, RoleObserver, RoleVictimRep:
		return true
	}
	return false
}

// ValidateClassificationChange ensures a new classification value is valid
// and that ex_parte_side is provided exactly when required.
//
// Rules:
//   - newClass must be one of the four known values.
//   - If newClass == ex_parte, exParteSide must be non-nil and in
//     {prosecution, defence}.
//   - If newClass != ex_parte, exParteSide must be nil (or a nil-pointer to
//     an empty string) — a side on a non-ex_parte item is a contradiction.
func ValidateClassificationChange(newClass string, exParteSide *string) error {
	if !validClassifications[newClass] {
		return &ValidationError{Field: "classification", Message: "invalid classification"}
	}

	if newClass == ClassificationExParte {
		if exParteSide == nil || strings.TrimSpace(*exParteSide) == "" {
			return &ValidationError{
				Field:   "ex_parte_side",
				Message: "ex_parte classification requires a side (prosecution or defence)",
			}
		}
		if !validExParteSides[*exParteSide] {
			return &ValidationError{
				Field:   "ex_parte_side",
				Message: fmt.Sprintf("invalid ex_parte_side %q; must be prosecution or defence", *exParteSide),
			}
		}
		return nil
	}

	// Non-ex_parte classifications must not carry a side.
	if exParteSide != nil && strings.TrimSpace(*exParteSide) != "" {
		return &ValidationError{
			Field:   "ex_parte_side",
			Message: "ex_parte_side is only permitted when classification is ex_parte",
		}
	}
	return nil
}

// UserSideForRole maps a case role to the "side" it represents for ex_parte
// visibility. Roles with no side (observer, victim_representative, judge)
// return "".
func UserSideForRole(role string) string {
	switch role {
	case RoleInvestigator, RoleProsecutor:
		return ExPartePros
	case RoleDefence:
		return ExParteDefence
	default:
		return ""
	}
}

// buildClassificationAccessSQL returns a WHERE fragment enforcing the
// classification access matrix for `role`. The fragment assumes the
// evidence alias is `e`.
//
// The returned fragment mirrors CheckAccess so read and write code paths
// stay aligned:
//
//	e.classification IN ('public','restricted')
//	OR (e.classification = 'confidential' AND <role is investigator|prosecutor|judge>)
//	OR (e.classification = 'ex_parte' AND (
//	      <role is judge>
//	      OR (e.ex_parte_side = 'prosecution' AND <role is investigator|prosecutor>)
//	      OR (e.ex_parte_side = 'defence'     AND <role is defence>)
//	    ))
//
// Unknown roles get a deny-all fragment (1=0) so no accidental disclosure.
//
// SQL injection note: every interpolated value is a compile-time
// classification or side constant — never user input. The `role` parameter
// controls branching only; it is never written into the SQL string. This
// is safe but fragile: do NOT extend this function to interpolate any
// runtime value without switching to parameterised placeholders.
func buildClassificationAccessSQL(role string) string {
	if !isKnownRole(role) {
		return "1=0"
	}

	seesConfidential := role == RoleInvestigator || role == RoleProsecutor || role == RoleJudge
	seesPros := role == RoleInvestigator || role == RoleProsecutor || role == RoleJudge
	seesDef := role == RoleDefence || role == RoleJudge

	var parts []string
	parts = append(parts, fmt.Sprintf("e.classification IN ('%s','%s')", ClassificationPublic, ClassificationRestricted))
	if seesConfidential {
		parts = append(parts, fmt.Sprintf("e.classification = '%s'", ClassificationConfidential))
	}

	var exParteSideClauses []string
	if seesPros {
		exParteSideClauses = append(exParteSideClauses, fmt.Sprintf("e.ex_parte_side = '%s'", ExPartePros))
	}
	if seesDef {
		exParteSideClauses = append(exParteSideClauses, fmt.Sprintf("e.ex_parte_side = '%s'", ExParteDefence))
	}
	if len(exParteSideClauses) > 0 {
		parts = append(parts,
			fmt.Sprintf("(e.classification = '%s' AND (%s))",
				ClassificationExParte,
				strings.Join(exParteSideClauses, " OR "),
			),
		)
	}

	return "(" + strings.Join(parts, " OR ") + ")"
}
