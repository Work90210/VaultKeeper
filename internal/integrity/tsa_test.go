package integrity

import (
	"context"
	"crypto"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNoopTimestampAuthority_IssueTimestamp(t *testing.T) {
	noop := &NoopTimestampAuthority{}

	token, name, ts, err := noop.IssueTimestamp(context.Background(), []byte("digest"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != nil {
		t.Error("expected nil token")
	}
	if name != "" {
		t.Error("expected empty name")
	}
	if !ts.IsZero() {
		t.Error("expected zero time")
	}
}

func TestNoopTimestampAuthority_VerifyTimestamp(t *testing.T) {
	noop := &NoopTimestampAuthority{}

	err := noop.VerifyTimestamp(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRFC3161Client_Creation(t *testing.T) {
	client := NewRFC3161Client("http://example.com/tsa")
	if client.url != "http://example.com/tsa" {
		t.Errorf("url = %q", client.url)
	}
	if client.maxRetries != 3 {
		t.Errorf("maxRetries = %d", client.maxRetries)
	}
	if client.httpClient.Timeout != 10*time.Second {
		t.Errorf("timeout = %v", client.httpClient.Timeout)
	}
}

func TestRFC3161Client_IssueTimestamp_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewRFC3161Client(srv.URL)
	client.maxRetries = 1

	_, _, _, err := client.IssueTimestamp(context.Background(), make([]byte, 32))
	if err == nil {
		t.Fatal("expected error for server 500")
	}
}

func TestRFC3161Client_IssueTimestamp_InvalidURL(t *testing.T) {
	client := NewRFC3161Client("http://127.0.0.1:0/invalid")
	client.maxRetries = 1

	_, _, _, err := client.IssueTimestamp(context.Background(), make([]byte, 32))
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestRFC3161Client_IssueTimestamp_CancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewRFC3161Client(srv.URL)
	client.maxRetries = 3

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately so the retry delay select picks up ctx.Done()
	cancel()

	_, _, _, err := client.IssueTimestamp(ctx, make([]byte, 32))
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestRFC3161Client_IssueTimestamp_InvalidResponseBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/timestamp-reply")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not-valid-asn1"))
	}))
	defer srv.Close()

	client := NewRFC3161Client(srv.URL)
	client.maxRetries = 1

	_, _, _, err := client.IssueTimestamp(context.Background(), make([]byte, 32))
	if err == nil {
		t.Fatal("expected error for invalid ASN.1 response")
	}
}

func TestRFC3161Client_IssueTimestamp_RetryThenFail(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := NewRFC3161Client(srv.URL)
	client.maxRetries = 2

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, _, _, err := client.IssueTimestamp(ctx, make([]byte, 32))
	if err == nil {
		t.Fatal("expected error after retries exhausted")
	}
	if attempts < 2 {
		t.Errorf("expected at least 2 attempts, got %d", attempts)
	}
}

func TestRFC3161Client_VerifyTimestamp_InvalidToken(t *testing.T) {
	client := NewRFC3161Client("http://example.com")

	err := client.VerifyTimestamp(context.Background(), []byte("not-valid"), make([]byte, 32))
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestRFC3161Client_doRequest_BadURL(t *testing.T) {
	client := NewRFC3161Client("://bad-url")

	_, _, _, err := client.doRequest(context.Background(), []byte("req"), make([]byte, 32))
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestRFC3161Client_doRequest_ReadBodyError(t *testing.T) {
	// Server that returns 200 but closes connection before body is fully read
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100000") // Claim a large body
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("partial")) // Write less than Content-Length
		// Connection closes prematurely, causing io.ReadAll to error
	}))
	defer srv.Close()

	client := NewRFC3161Client(srv.URL)

	_, _, _, err := client.doRequest(context.Background(), []byte("req"), make([]byte, 32))
	if err == nil {
		t.Fatal("expected error for read body failure")
	}
}

func TestRFC3161Client_IssueTimestamp_MarshalError(t *testing.T) {
	client := NewRFC3161Client("http://localhost:0")
	client.maxRetries = 1
	// Use an invalid hash algorithm to trigger Marshal error
	client.hashAlgorithm = crypto.Hash(0)

	_, _, _, err := client.IssueTimestamp(context.Background(), make([]byte, 32))
	if err == nil {
		t.Fatal("expected error for invalid hash algorithm")
	}
	if !strings.Contains(err.Error(), "create TSA request") {
		t.Errorf("error = %q, want 'create TSA request' prefix", err.Error())
	}
}

func TestBuildTSARequest_Valid(t *testing.T) {
	req, err := buildTSARequest(crypto.SHA256, make([]byte, 32))
	if err != nil {
		t.Fatalf("buildTSARequest error: %v", err)
	}
	if len(req) == 0 {
		t.Error("expected non-empty request")
	}
}

func TestBuildTSARequest_InvalidHash(t *testing.T) {
	_, err := buildTSARequest(crypto.Hash(0), make([]byte, 32))
	if err == nil {
		t.Fatal("expected error for invalid hash algorithm")
	}
}
