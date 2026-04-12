package migration

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestGenerateAttestationPDF_BasicSigned(t *testing.T) {
	signer, err := LoadOrGenerate()
	if err != nil {
		t.Fatalf("LoadOrGenerate: %v", err)
	}
	ts := time.Date(2026, 4, 9, 14, 30, 0, 0, time.UTC)
	rec := Record{
		ID:              uuid.New(),
		CaseID:          uuid.New(),
		SourceSystem:    "RelativityOne",
		TotalItems:      2,
		MatchedItems:    2,
		MismatchedItems: 0,
		MigrationHash:   strings.Repeat("a", 64),
		ManifestHash:    strings.Repeat("b", 64),
		PerformedBy:     "tester",
		StartedAt:       ts,
		CompletedAt:     &ts,
		TSAToken:        []byte("fake-token"),
		TSAName:         "FakeTSA",
		TSATimestamp:    &ts,
	}
	in := CertificateInput{
		Record:        rec,
		CaseReference: "ICC-TEST-2026",
		CaseTitle:     "Test Case",
		InstanceVer:   "dev",
		GeneratedAt:   ts,
		PublicKeyB64:  signer.PublicKeyBase64(),
		Files: []CertificateFile{
			{Index: 1, Filename: "a.pdf", SourceHash: strings.Repeat("a", 64), ComputedHash: strings.Repeat("a", 64), Match: true},
			{Index: 2, Filename: "b.pdf", SourceHash: strings.Repeat("b", 64), ComputedHash: strings.Repeat("b", 64), Match: true},
		},
	}
	cert, err := GenerateAttestationPDF(in, signer)
	if err != nil {
		t.Fatalf("GenerateAttestationPDF: %v", err)
	}
	if len(cert.PDFBytes) == 0 {
		t.Error("PDFBytes is empty")
	}
	// PDF file signature starts with "%PDF-".
	if !strings.HasPrefix(string(cert.PDFBytes[:5]), "%PDF-") {
		t.Errorf("PDF header not present, got %q", string(cert.PDFBytes[:5]))
	}
	if len(cert.Signature) == 0 {
		t.Error("Signature missing")
	}
	// Signature must verify against the same signer.
	if !signer.Verify(cert.SignedBody, cert.Signature) {
		t.Error("signature verification failed")
	}
}

func TestGenerateAttestationPDF_FrenchLocale(t *testing.T) {
	signer, _ := LoadOrGenerate()
	ts := time.Now().UTC()
	in := CertificateInput{
		Record: Record{
			ID:            uuid.New(),
			CaseID:        uuid.New(),
			SourceSystem:  "RelativityOne",
			TotalItems:    1,
			MatchedItems:  1,
			MigrationHash: strings.Repeat("c", 64),
			ManifestHash:  strings.Repeat("c", 64),
			PerformedBy:   "testeur",
			StartedAt:     ts,
		},
		CaseReference: "ICC-FR-2026",
		CaseTitle:     "Dossier",
		Locale:        "fr",
		GeneratedAt:   ts,
		InstanceVer:   "dev",
		PublicKeyB64:  signer.PublicKeyBase64(),
	}
	cert, err := GenerateAttestationPDF(in, signer)
	if err != nil {
		t.Fatalf("GenerateAttestationPDF: %v", err)
	}
	// Canonical body must carry the French title so verifiers see it.
	if !strings.Contains(string(cert.SignedBody), "CERTIFICAT") {
		t.Errorf("French title missing from canonical body")
	}
}

func TestRequireConfiguredKey(t *testing.T) {
	t.Setenv("INSTANCE_ED25519_KEY", "")
	if err := RequireConfiguredKey(); err == nil {
		t.Error("want error when key not set")
	}
	key, err := GenerateKeyBase64()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("INSTANCE_ED25519_KEY", key)
	if err := RequireConfiguredKey(); err != nil {
		t.Errorf("unexpected error with key set: %v", err)
	}
	// And loading it back should succeed.
	signer, err := LoadOrGenerate()
	if err != nil {
		t.Fatalf("LoadOrGenerate with env: %v", err)
	}
	if len(signer.PublicKey()) == 0 {
		t.Error("public key empty")
	}
}

func TestLoadOrGenerate_InvalidEnv(t *testing.T) {
	t.Setenv("INSTANCE_ED25519_KEY", "not-base64!!")
	if _, err := LoadOrGenerate(); err == nil {
		t.Error("want error for invalid base64 env")
	}
	t.Setenv("INSTANCE_ED25519_KEY", "dGVzdA==") // valid base64, wrong length
	if _, err := LoadOrGenerate(); err == nil {
		t.Error("want error for wrong-length key")
	}
}

func TestMain(m *testing.M) {
	// Ensure nothing leaks from a user env into the test matrix.
	_ = os.Unsetenv("INSTANCE_ED25519_KEY")
	os.Exit(m.Run())
}
