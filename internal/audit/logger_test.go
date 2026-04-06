package audit

import (
	"testing"
	"time"
)

func TestComputeHash_Deterministic(t *testing.T) {
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	h1 := computeHash("login", "user-1", `{"ip":"1.2.3.4"}`, "prev-hash", ts)
	h2 := computeHash("login", "user-1", `{"ip":"1.2.3.4"}`, "prev-hash", ts)

	if h1 != h2 {
		t.Errorf("hash should be deterministic, got %q and %q", h1, h2)
	}

	if len(h1) != 64 {
		t.Errorf("hash length = %d, want 64 (sha256 hex)", len(h1))
	}
}

func TestComputeHash_DifferentInputs(t *testing.T) {
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	h1 := computeHash("login", "user-1", "{}", "", ts)
	h2 := computeHash("logout", "user-1", "{}", "", ts)
	h3 := computeHash("login", "user-2", "{}", "", ts)
	h4 := computeHash("login", "user-1", "{}", "different-prev", ts)

	if h1 == h2 {
		t.Error("different action should produce different hash")
	}
	if h1 == h3 {
		t.Error("different actor should produce different hash")
	}
	if h1 == h4 {
		t.Error("different previous hash should produce different hash")
	}
}

func TestComputeHash_PreviousHashChains(t *testing.T) {
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	hash1 := computeHash("login", "user-1", "{}", "", ts)
	hash2 := computeHash("access_denied", "user-2", "{}", hash1, ts.Add(time.Second))

	if hash1 == hash2 {
		t.Error("chained hashes should differ")
	}
	if hash2 == "" {
		t.Error("chained hash should not be empty")
	}
}

func TestAuthEvent_Structure(t *testing.T) {
	event := AuthEvent{
		Action:      "access_denied",
		ActorUserID: "user-123",
		IPAddress:   "192.168.1.1",
		UserAgent:   "curl/8.0",
		Detail: map[string]string{
			"endpoint":      "/api/cases",
			"required_role": "case_admin",
			"actual_role":   "user",
		},
	}

	if event.Action != "access_denied" {
		t.Errorf("Action = %q", event.Action)
	}
	if event.Detail["endpoint"] != "/api/cases" {
		t.Errorf("Detail[endpoint] = %q", event.Detail["endpoint"])
	}
}
