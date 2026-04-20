package federation

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

type mockCustodyRecorder struct {
	events []recordedEvent
}

type recordedEvent struct {
	CaseID      uuid.UUID
	Action      string
	ActorUserID string
	Detail      map[string]string
}

func (m *mockCustodyRecorder) RecordCaseEvent(_ context.Context, caseID uuid.UUID, action string, actorUserID string, detail map[string]string) error {
	m.events = append(m.events, recordedEvent{
		CaseID:      caseID,
		Action:      action,
		ActorUserID: actorUserID,
		Detail:      detail,
	})
	return nil
}

func TestRecordDisclosureBridgeEvent(t *testing.T) {
	recorder := &mockCustodyRecorder{}
	caseID := uuid.New()
	exchangeID := uuid.New()
	ev1, ev2 := uuid.New(), uuid.New()

	err := RecordDisclosureBridgeEvent(context.Background(), recorder, DisclosureBridgeInput{
		CaseID:               caseID,
		ExchangeID:           exchangeID,
		ManifestHash:         "mhash",
		RecipientInstanceID:  "recipient-inst",
		RecipientFingerprint: "sha256:fp",
		DisclosedEvidenceIDs: []uuid.UUID{ev1, ev2},
		ScopeHash:            "shash",
		MerkleRoot:           "mroot",
		ScopeCardinality:     2,
		ActorUserID:          "user-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(recorder.events))
	}
	e := recorder.events[0]
	if e.Action != ActionDisclosedToInstance {
		t.Errorf("action = %q, want %q", e.Action, ActionDisclosedToInstance)
	}
	if e.CaseID != caseID {
		t.Error("case_id mismatch")
	}
	if e.Detail["exchange_id"] != exchangeID.String() {
		t.Error("exchange_id mismatch")
	}
	if e.Detail["scope_cardinality"] != "2" {
		t.Errorf("scope_cardinality = %q", e.Detail["scope_cardinality"])
	}

	var ids []uuid.UUID
	if err := json.Unmarshal([]byte(e.Detail["disclosed_evidence_ids"]), &ids); err != nil {
		t.Fatalf("unmarshal evidence IDs: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 evidence IDs, got %d", len(ids))
	}
}

func TestRecordImportBridgeEvent(t *testing.T) {
	recorder := &mockCustodyRecorder{}
	caseID := uuid.New()
	exchangeID := uuid.New()
	ev1 := uuid.New()

	err := RecordImportBridgeEvent(context.Background(), recorder, ImportBridgeInput{
		CaseID:                caseID,
		ExchangeID:            exchangeID,
		ManifestHash:          "mhash",
		SenderInstanceID:      "sender-inst",
		SenderFingerprint:     "sha256:sfp",
		SenderCustodyHead:     "chead",
		SenderBridgeEventHash: "bhash",
		ImportedEvidenceIDs:   []uuid.UUID{ev1},
		ActorUserID:           "user-2",
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(recorder.events))
	}
	e := recorder.events[0]
	if e.Action != ActionImportedFromInstance {
		t.Errorf("action = %q, want %q", e.Action, ActionImportedFromInstance)
	}
	if e.Detail["sender_bridge_event_hash"] != "bhash" {
		t.Error("sender_bridge_event_hash mismatch")
	}
	if e.Detail["sender_custody_head"] != "chead" {
		t.Error("sender_custody_head mismatch")
	}
}
