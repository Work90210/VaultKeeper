package integrity

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"
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
	// trustedRoots is the CA pool used to verify the TSA signing certificate
	// chain. When nil, VerifyTimestamp logs a warning and skips chain
	// verification (insecure). Production deployments MUST populate this
	// via WithTrustedRoots.
	trustedRoots *x509.CertPool
}

// WithTrustedRoots configures the CA certificate pool used to verify the
// TSA signing certificate chain. Without this, VerifyTimestamp only checks
// the CMS signature consistency but cannot detect a forged token signed by
// an attacker-controlled certificate.
func (c *RFC3161Client) WithTrustedRoots(roots *x509.CertPool) *RFC3161Client {
	c.trustedRoots = roots
	return c
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

	const maxTSAResponseBytes = 64 * 1024 // 64 KiB
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTSAResponseBytes+1))
	if err != nil {
		return nil, "", time.Time{}, fmt.Errorf("read TSA response: %w", err)
	}
	if len(body) > maxTSAResponseBytes {
		return nil, "", time.Time{}, fmt.Errorf("TSA response exceeds %d bytes", maxTSAResponseBytes)
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

	// Always verify the chain: verifyChain returns an error when trustedRoots is nil,
	// making this fail-closed. Removing the nil guard matches the behaviour of
	// VerifyTimestamp, which also refuses to skip chain verification.
	if err := c.verifyChain(ts); err != nil {
		return nil, "", time.Time{}, fmt.Errorf("TSA response chain verification failed: %w", err)
	}

	return body, tsaName, ts.Time, nil
}

// verifyChain verifies that the TSA signing certificate in the timestamp
// response chains to a configured trusted root CA. Returns an error when
// trustedRoots is nil so that callers always fail closed when roots are absent.
func (c *RFC3161Client) verifyChain(ts *timestamp.Timestamp) error {
	if c.trustedRoots == nil {
		return fmt.Errorf("TSA trusted roots not configured — cannot verify timestamp certificate chain")
	}
	if len(ts.Certificates) == 0 {
		return fmt.Errorf("TSA token contains no signing certificates — cannot verify chain of trust")
	}
	// The first certificate is the TSA signer; remaining certs form
	// the intermediate chain.
	signerCert := ts.Certificates[0]
	intermediates := x509.NewCertPool()
	for _, cert := range ts.Certificates[1:] {
		intermediates.AddCert(cert)
	}
	_, err := signerCert.Verify(x509.VerifyOptions{
		Roots:         c.trustedRoots,
		Intermediates: intermediates,
		// TSA certificates use the timestamping EKU.
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageTimeStamping},
		// Verify against the token's own time so that historical
		// tokens remain valid after the TSA cert expires.
		CurrentTime: ts.Time,
	})
	if err != nil {
		return fmt.Errorf("TSA certificate chain verification failed: %w", err)
	}
	return nil
}

func (c *RFC3161Client) VerifyTimestamp(_ context.Context, token []byte, digest []byte) error {
	// ParseResponse internally calls pkcs7.Parse + p7.Verify(), which
	// validates that the CMS signature is mathematically correct (the data
	// was signed by the embedded certificate's private key). However,
	// p7.Verify does NOT verify that the signing certificate chains to a
	// trusted root — an attacker can self-sign a certificate, create a
	// valid CMS structure with any timestamp they choose, and p7.Verify
	// will pass. We must additionally verify the cert chain against our
	// trusted root CA pool to prevent forged timestamps.
	ts, err := timestamp.ParseResponse(token)
	if err != nil {
		return fmt.Errorf("parse TSA token: %w", err)
	}

	if !bytes.Equal(ts.HashedMessage, digest) {
		return fmt.Errorf("TSA token digest mismatch")
	}

	// Verify the TSA signing certificate chains to a trusted root.
	if c.trustedRoots != nil {
		if err := c.verifyChain(ts); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("TSA trusted roots not configured — cannot verify timestamp certificate chain")
	}

	return nil
}

// NoopTimestampAuthority is a no-op TSA for when timestamping is disabled.
type NoopTimestampAuthority struct{}

func (n *NoopTimestampAuthority) IssueTimestamp(_ context.Context, _ []byte) ([]byte, string, time.Time, error) {
	slog.Warn("NoopTimestampAuthority: IssueTimestamp called — timestamps are NOT being issued; ensure TIMESTAMP_AUTHORITY_URL is configured in production")
	return nil, "", time.Time{}, nil
}

func (n *NoopTimestampAuthority) VerifyTimestamp(_ context.Context, token []byte, _ []byte) error {
	if len(token) > 0 {
		return fmt.Errorf("timestamp verification unavailable: timestamping is disabled but a token was provided")
	}
	return nil
}
