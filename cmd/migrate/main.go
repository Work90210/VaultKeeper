// Command vaultkeeper-migrate is the CLI companion to Sprint 10's data
// migration tool. It supports two modes:
//
//  1. Offline dry-run — compute SHA-256 for each file in a manifest and
//     compare against the manifest's source hash. No DB or storage access.
//     Useful for operators preparing a migration to verify hashes before
//     committing to ingestion.
//
//  2. API-driven full run — POSTs the manifest to a running VaultKeeper
//     server, which performs verified ingestion, TSA stamping, and
//     certificate generation. The CLI streams progress and downloads the
//     final attestation PDF on completion.
//
// Subcommands:
//
//	vaultkeeper-migrate dry-run  --manifest <path> --files <dir>
//	vaultkeeper-migrate run      --case <id> --source-system <name> \
//	                             --manifest <path> --files <dir> \
//	                             --api <url> --token <jwt> [--concurrency N]
//	vaultkeeper-migrate certificate --migration <id> --api <url> --token <jwt> --output <path>
//	vaultkeeper-migrate genkey   (prints a base64 Ed25519 private key for INSTANCE_ED25519_KEY)
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/migration"
)

// cliContext returns a context that is cancelled when the operator
// presses Ctrl-C or the process receives SIGTERM. Long-running
// operations must use this so the ingestion pipeline can cancel cleanly
// instead of being killed mid-file.
func cliContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "dry-run":
		err = runDryRun(os.Args[2:])
	case "run":
		err = runFullMigration(os.Args[2:])
	case "certificate":
		err = runFetchCertificate(os.Args[2:])
	case "genkey":
		err = runGenKey()
	case "-h", "--help", "help":
		usage()
		return
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `vaultkeeper-migrate — Sprint 10 data migration CLI

USAGE:
    vaultkeeper-migrate dry-run --manifest <path> --files <dir> [--format csv|relativity]
    vaultkeeper-migrate run --case <uuid> --source-system <name> --manifest <path> --files <dir> --api <url> --token <jwt> [--concurrency N] [--halt-on-mismatch]
    vaultkeeper-migrate certificate --migration <uuid> --api <url> --token <jwt> --output <path>
    vaultkeeper-migrate genkey`)
}

func runDryRun(args []string) error {
	fs := flag.NewFlagSet("dry-run", flag.ExitOnError)
	manifest := fs.String("manifest", "", "path to manifest file")
	filesDir := fs.String("files", "", "root directory containing the files")
	format := fs.String("format", "csv", "manifest format (csv|relativity)")
	concurrency := fs.Int("concurrency", 4, "worker concurrency")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *manifest == "" || *filesDir == "" {
		return fmt.Errorf("dry-run requires --manifest and --files")
	}
	ctx, cancel := cliContext()
	defer cancel()

	mf, err := os.Open(*manifest) // #nosec G304 -- operator-supplied path
	if err != nil {
		return fmt.Errorf("open manifest: %w", err)
	}
	defer mf.Close()

	parser := migration.NewParser()
	entries, err := parser.Parse(ctx, mf, migration.ManifestFormat(*format))
	if err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}

	ingester := migration.NewIngester(nil, nil) // no writer needed for dry-run
	report, err := ingester.BatchIngest(ctx, migration.BatchRequest{
		CaseID:     uuid.New(),
		SourceRoot: *filesDir,
		UploadedBy: "dry-run",
		Entries:    entries,
		Options: migration.BatchOptions{
			Concurrency:    *concurrency,
			DryRun:         true,
			HaltOnMismatch: false,
		},
	}, uuid.New(), func(cur, total int, path string) {
		fmt.Fprintf(os.Stdout, "[%d/%d] %s\n", cur, total, path)
	})
	if err != nil {
		return err
	}
	fmt.Printf("\nDry-run complete: %d matched, %d mismatched, %d failed.\n",
		report.MatchedItems, report.MismatchedItems, len(report.Failed)-report.MismatchedItems)
	for _, f := range report.Failed {
		fmt.Printf("  FAIL %s: %s\n", f.FilePath, f.Reason)
	}
	if report.MismatchedItems > 0 {
		return fmt.Errorf("%d hash mismatches", report.MismatchedItems)
	}
	return nil
}

// runFullMigration drives the HTTP API. The server owns ingestion, TSA
// stamping, and certificate generation; the CLI only validates the
// manifest locally (fast-fail) and then hands the server a reference to
// the manifest file and the evidence root directory. This assumes the
// CLI host and the server share a filesystem (NAS / bind mount) — the
// common deployment shape for on-prem forensic workflows.
func runFullMigration(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	caseID := fs.String("case", "", "target case UUID")
	sourceSystem := fs.String("source-system", "", "source system name (e.g. RelativityOne)")
	manifest := fs.String("manifest", "", "manifest file path (server-visible)")
	filesDir := fs.String("files", "", "files directory (server-visible)")
	format := fs.String("format", "csv", "manifest format (csv|relativity)")
	apiURL := fs.String("api", "", "VaultKeeper API base URL")
	token := fs.String("token", "", "bearer token")
	concurrency := fs.Int("concurrency", 4, "worker concurrency")
	halt := fs.Bool("halt-on-mismatch", true, "halt migration on first hash mismatch")
	dryRun := fs.Bool("dry-run", false, "server-side dry run (hashes only, no ingestion)")
	resumeID := fs.String("resume-id", "", "resume an existing in-progress migration (UUID). Manifest and case must match.")
	if err := fs.Parse(args); err != nil {
		return err
	}
	// Ordered check so the error message is deterministic across runs.
	required := []struct{ name, value string }{
		{"case", *caseID},
		{"source-system", *sourceSystem},
		{"manifest", *manifest},
		{"files", *filesDir},
		{"api", *apiURL},
		{"token", *token},
	}
	for _, r := range required {
		if r.value == "" {
			return fmt.Errorf("--%s is required", r.name)
		}
	}
	if _, err := uuid.Parse(*caseID); err != nil {
		return fmt.Errorf("invalid --case uuid: %w", err)
	}
	ctx, cancel := cliContext()
	defer cancel()

	// Fast-fail: validate the manifest locally before making any HTTP
	// calls. If the CLI host can read the manifest, the server probably
	// can too — and if the manifest is malformed we want to know now, not
	// after the server round-trip.
	mf, err := os.Open(*manifest) // #nosec G304 -- operator-supplied path
	if err != nil {
		return fmt.Errorf("open manifest: %w", err)
	}
	defer mf.Close()
	parser := migration.NewParser()
	entries, err := parser.Parse(ctx, mf, migration.ManifestFormat(*format))
	if err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}
	fmt.Printf("Manifest OK: %d entries\n", len(entries))

	payload := manifestPayload{
		SourceSystem:   *sourceSystem,
		SourceRoot:     *filesDir,
		ManifestPath:   *manifest,
		ManifestFormat: *format,
		Concurrency:    *concurrency,
		HaltOnMismatch: *halt,
		DryRun:         *dryRun,
	}
	if *resumeID != "" {
		if _, err := uuid.Parse(*resumeID); err != nil {
			return fmt.Errorf("invalid --resume-id: %w", err)
		}
		payload.ResumeMigrationID = resumeID
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	url := fmt.Sprintf("%s/api/cases/%s/migrations", *apiURL, *caseID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+*token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 0} // long-running migrations; no client-side timeout
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", url, err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	var result struct {
		Migration map[string]any `json:"migration"`
		Processed int            `json:"processed"`
		Matched   int            `json:"matched"`
		Halted    bool           `json:"halted"`
		Error     string         `json:"error"`
	}
	if len(respBytes) > 0 {
		if err := json.Unmarshal(respBytes, &result); err != nil {
			// Non-fatal: the request itself may have succeeded (based
			// on HTTP status), but the response body is not in the
			// shape we expect. Usually this means the CLI and server
			// are on different versions. Log to stderr so operators
			// can spot the mismatch.
			fmt.Fprintf(os.Stderr, "warning: could not parse server response body: %v\n", err)
		}
	}

	switch {
	case resp.StatusCode == http.StatusCreated:
		fmt.Printf("Migration complete: %d files processed, %d matched.\n", result.Processed, result.Matched)
		if id, ok := result.Migration["ID"].(string); ok {
			fmt.Printf("Migration ID: %s\n", id)
			fmt.Printf("Next: vaultkeeper-migrate certificate --migration %s --api %s --token <TOKEN> --output ./cert.pdf\n", id, *apiURL)
		}
		return nil
	case resp.StatusCode == http.StatusConflict:
		fmt.Fprintf(os.Stderr, "Migration halted on hash mismatch: %s\n", result.Error)
		return fmt.Errorf("migration halted")
	default:
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(respBytes))
	}
}

type manifestPayload struct {
	SourceSystem      string  `json:"source_system"`
	SourceRoot        string  `json:"source_root"`
	ManifestPath      string  `json:"manifest_path"`
	ManifestFormat    string  `json:"manifest_format"`
	Concurrency       int     `json:"concurrency"`
	HaltOnMismatch    bool    `json:"halt_on_mismatch"`
	DryRun            bool    `json:"dry_run"`
	ResumeMigrationID *string `json:"resume_migration_id,omitempty"`
}


func runFetchCertificate(args []string) error {
	fs := flag.NewFlagSet("certificate", flag.ExitOnError)
	migrationID := fs.String("migration", "", "migration UUID")
	apiURL := fs.String("api", "", "VaultKeeper API base URL")
	token := fs.String("token", "", "bearer token")
	output := fs.String("output", "", "output PDF path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *migrationID == "" || *apiURL == "" || *token == "" || *output == "" {
		return fmt.Errorf("certificate requires --migration --api --token --output")
	}

	ctx, cancel := cliContext()
	defer cancel()

	url := fmt.Sprintf("%s/api/migrations/%s/certificate", *apiURL, *migrationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+*token)
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch certificate: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(b))
	}
	if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
		return err
	}
	f, err := os.Create(*output) // #nosec G304 -- operator-supplied path
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	fmt.Printf("Certificate written to %s\n", *output)
	return nil
}

func runGenKey() error {
	key, err := migration.GenerateKeyBase64()
	if err != nil {
		return err
	}
	fmt.Println(key)
	fmt.Fprintln(os.Stderr, "\nSet INSTANCE_ED25519_KEY to the value above in your VaultKeeper environment.")
	return nil
}
