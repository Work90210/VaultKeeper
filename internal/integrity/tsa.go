package integrity

import (
	"bytes"
	"context"
	"crypto"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/digitorus/timestamp"
)

// TimestampAuthority issues and verifies RFC 3161 trusted timestamps.
type TimestampAuthority interface {
	IssueTimestamp(ctx context.Context, digest []byte) (token []byte, tsaName string, tsTime time.Time, err error)
	VerifyTimestamp(ctx context.Context, token []byte, digest []byte) error
}

// RFC3161Client implements TimestampAuthority using an RFC 3161 TSA.
type RFC3161Client struct {
	url           string
	httpClient    *http.Client
	maxRetries    int
	hashAlgorithm crypto.Hash // defaults to crypto.SHA256
}

// NewRFC3161Client creates a new RFC 3161 timestamp client.
func NewRFC3161Client(tsaURL string) *RFC3161Client {
	return &RFC3161Client{
		url: tsaURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		maxRetries:    3,
		hashAlgorithm: crypto.SHA256,
	}
}

func (c *RFC3161Client) IssueTimestamp(ctx context.Context, digest []byte) ([]byte, string, time.Time, error) {
	tsReq, err := buildTSARequest(c.hashAlgorithm, digest)
	if err != nil {
		return nil, "", time.Time{}, fmt.Errorf("create TSA request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < c.maxRetries; attempt++ {
		token, name, ts, err := c.doRequest(ctx, tsReq, digest)
		if err == nil {
			return token, name, ts, nil
		}
		lastErr = err
		if attempt < c.maxRetries-1 {
			select {
			case <-ctx.Done():
				return nil, "", time.Time{}, fmt.Errorf("TSA request cancelled: %w", ctx.Err())
			case <-time.After(time.Duration(attempt+1) * time.Second):
			}
		}
	}
	return nil, "", time.Time{}, fmt.Errorf("TSA request failed after %d retries: %w", c.maxRetries, lastErr)
}

// buildTSARequest creates an RFC 3161 timestamp request with the pre-computed
// hash directly. timestamp.CreateRequest() would double-hash since it reads
// and hashes the input.
func buildTSARequest(hashAlg crypto.Hash, digest []byte) ([]byte, error) {
	return (&timestamp.Request{
		HashAlgorithm: hashAlg,
		HashedMessage: digest,
		Certificates:  true,
	}).Marshal()
}

func (c *RFC3161Client) doRequest(ctx context.Context, tsReq []byte, digest []byte) ([]byte, string, time.Time, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(tsReq))
	if err != nil {
		return nil, "", time.Time{}, fmt.Errorf("create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/timestamp-query")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", time.Time{}, fmt.Errorf("send TSA request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", time.Time{}, fmt.Errorf("TSA returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", time.Time{}, fmt.Errorf("read TSA response: %w", err)
	}

	ts, err := timestamp.ParseResponse(body)
	if err != nil {
		return nil, "", time.Time{}, fmt.Errorf("parse TSA response: %w", err)
	}

	// Verify the response contains our digest
	if !bytes.Equal(ts.HashedMessage, digest) {
		return nil, "", time.Time{}, fmt.Errorf("TSA response digest mismatch")
	}

	tsaName := ""
	if len(ts.Certificates) > 0 {
		tsaName = ts.Certificates[0].Subject.CommonName
	}

	return body, tsaName, ts.Time, nil
}

func (c *RFC3161Client) VerifyTimestamp(_ context.Context, token []byte, digest []byte) error {
	ts, err := timestamp.ParseResponse(token)
	if err != nil {
		return fmt.Errorf("parse TSA token: %w", err)
	}

	if !bytes.Equal(ts.HashedMessage, digest) {
		return fmt.Errorf("TSA token digest mismatch")
	}

	return nil
}

// NoopTimestampAuthority is a no-op TSA for when timestamping is disabled.
type NoopTimestampAuthority struct{}

func (n *NoopTimestampAuthority) IssueTimestamp(_ context.Context, _ []byte) ([]byte, string, time.Time, error) {
	return nil, "", time.Time{}, nil
}

func (n *NoopTimestampAuthority) VerifyTimestamp(_ context.Context, _ []byte, _ []byte) error {
	return nil
}
