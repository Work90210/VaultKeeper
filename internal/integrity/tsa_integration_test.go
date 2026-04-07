package integrity

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/digitorus/timestamp"
)

// testTSAPolicyOID is a dummy OID used for test TSA responses.
var testTSAPolicyOID = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 99999, 1}

// buildMockTSAServer creates a test HTTP server that returns valid RFC 3161 responses.
func buildMockTSAServer(t *testing.T, includeCerts bool) (*httptest.Server, *x509.Certificate) {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "VaultKeeper Test TSA",
			Organization: []string{"VaultKeeper Test"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create CA cert: %v", err)
	}
	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		t.Fatalf("parse CA cert: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		tsReq, err := timestamp.ParseRequest(body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Build a Timestamp to create a response.
		ts := &timestamp.Timestamp{
			HashAlgorithm:    tsReq.HashAlgorithm,
			HashedMessage:    tsReq.HashedMessage,
			Time:             time.Now(),
			Accuracy:         time.Second,
			SerialNumber:     big.NewInt(time.Now().UnixNano()),
			Policy:           testTSAPolicyOID,
			AddTSACertificate: includeCerts,
		}
		if includeCerts {
			ts.Certificates = []*x509.Certificate{caCert}
		}

		tsToken, err := ts.CreateResponse(caCert, caKey)
		if err != nil {
			t.Logf("CreateResponse error: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/timestamp-reply")
		w.WriteHeader(http.StatusOK)
		w.Write(tsToken)
	}))

	return srv, caCert
}

func TestIntegration_IssueTimestamp_HappyPath(t *testing.T) {
	srv, _ := buildMockTSAServer(t, true)
	defer srv.Close()

	client := NewRFC3161Client(srv.URL)
	ctx := context.Background()

	data := []byte("evidence file content for timestamping")
	digest := sha256.Sum256(data)

	token, tsaName, tsTime, err := client.IssueTimestamp(ctx, digest[:])
	if err != nil {
		t.Fatalf("IssueTimestamp: %v", err)
	}

	if len(token) == 0 {
		t.Error("expected non-empty token")
	}
	if tsaName == "" {
		t.Error("expected non-empty TSA name")
	}
	if tsaName != "VaultKeeper Test TSA" {
		t.Errorf("tsa_name = %q, want %q", tsaName, "VaultKeeper Test TSA")
	}
	if tsTime.IsZero() {
		t.Error("expected non-zero timestamp")
	}
	t.Logf("TSA name: %s, time: %v, token length: %d", tsaName, tsTime, len(token))
}

func TestIntegration_VerifyTimestamp_Valid(t *testing.T) {
	srv, _ := buildMockTSAServer(t, true)
	defer srv.Close()

	client := NewRFC3161Client(srv.URL)
	ctx := context.Background()

	data := []byte("verify this content")
	digest := sha256.Sum256(data)

	token, _, _, err := client.IssueTimestamp(ctx, digest[:])
	if err != nil {
		t.Fatalf("IssueTimestamp: %v", err)
	}

	err = client.VerifyTimestamp(ctx, token, digest[:])
	if err != nil {
		t.Fatalf("VerifyTimestamp with correct digest: %v", err)
	}
}

func TestIntegration_VerifyTimestamp_DigestMismatch(t *testing.T) {
	srv, _ := buildMockTSAServer(t, true)
	defer srv.Close()

	client := NewRFC3161Client(srv.URL)
	ctx := context.Background()

	data := []byte("original content")
	digest := sha256.Sum256(data)

	token, _, _, err := client.IssueTimestamp(ctx, digest[:])
	if err != nil {
		t.Fatalf("IssueTimestamp: %v", err)
	}

	wrongDigest := sha256.Sum256([]byte("tampered content"))
	err = client.VerifyTimestamp(ctx, token, wrongDigest[:])
	if err == nil {
		t.Fatal("expected error for digest mismatch")
	}
	if err.Error() != "TSA token digest mismatch" {
		t.Errorf("error = %q, want %q", err.Error(), "TSA token digest mismatch")
	}
}

func TestIntegration_IssueTimestamp_NoCertificates(t *testing.T) {
	srv, _ := buildMockTSAServer(t, false)
	defer srv.Close()

	client := NewRFC3161Client(srv.URL)
	ctx := context.Background()

	digest := sha256.Sum256([]byte("test"))
	token, tsaName, tsTime, err := client.IssueTimestamp(ctx, digest[:])
	if err != nil {
		t.Fatalf("IssueTimestamp: %v", err)
	}
	if len(token) == 0 {
		t.Error("expected non-empty token")
	}
	if tsaName != "" {
		t.Errorf("expected empty tsaName when no certs, got %q", tsaName)
	}
	if tsTime.IsZero() {
		t.Error("expected non-zero time")
	}
}

func TestIntegration_IssueTimestamp_RetrySuccess(t *testing.T) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Retry TSA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}

	attempt := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		tsReq, err := timestamp.ParseRequest(body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ts := &timestamp.Timestamp{
			HashAlgorithm:    crypto.SHA256,
			HashedMessage:    tsReq.HashedMessage,
			Time:             time.Now(),
			SerialNumber:     big.NewInt(time.Now().UnixNano()),
			Policy:           testTSAPolicyOID,
			Certificates:     []*x509.Certificate{caCert},
			AddTSACertificate: true,
		}
		tsToken, err := ts.CreateResponse(caCert, caKey)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/timestamp-reply")
		w.WriteHeader(http.StatusOK)
		w.Write(tsToken)
	}))
	defer srv.Close()

	client := NewRFC3161Client(srv.URL)
	client.maxRetries = 3
	ctx := context.Background()

	digest := sha256.Sum256([]byte("retry content"))
	token, tsaName, _, err := client.IssueTimestamp(ctx, digest[:])
	if err != nil {
		t.Fatalf("IssueTimestamp after retry: %v", err)
	}
	if len(token) == 0 {
		t.Error("expected non-empty token")
	}
	if tsaName != "Retry TSA" {
		t.Errorf("tsaName = %q, want %q", tsaName, "Retry TSA")
	}
	if attempt < 2 {
		t.Errorf("expected at least 2 attempts, got %d", attempt)
	}
}

func TestIntegration_IssueTimestamp_DigestMismatch(t *testing.T) {
	// Server that returns a valid TSA response but with a different digest
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Mismatch TSA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, err = timestamp.ParseRequest(body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Return a response with a DIFFERENT digest than what was requested
		wrongDigest := sha256.Sum256([]byte("wrong content"))
		ts := &timestamp.Timestamp{
			HashAlgorithm: crypto.SHA256,
			HashedMessage: wrongDigest[:],
			Time:          time.Now(),
			SerialNumber:  big.NewInt(time.Now().UnixNano()),
			Policy:        testTSAPolicyOID,
			Certificates:  []*x509.Certificate{caCert},
			AddTSACertificate: true,
		}
		tsToken, err := ts.CreateResponse(caCert, caKey)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/timestamp-reply")
		w.WriteHeader(http.StatusOK)
		w.Write(tsToken)
	}))
	defer srv.Close()

	client := NewRFC3161Client(srv.URL)
	client.maxRetries = 1

	realDigest := sha256.Sum256([]byte("real content"))
	_, _, _, err = client.IssueTimestamp(context.Background(), realDigest[:])
	if err == nil {
		t.Fatal("expected error for digest mismatch")
	}
	if !strings.Contains(err.Error(), "digest mismatch") {
		t.Errorf("error = %q, want digest mismatch", err.Error())
	}
}
