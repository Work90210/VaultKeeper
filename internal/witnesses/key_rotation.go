package witnesses

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
)

// KeyRotationProgress tracks the state of a key rotation job.
type KeyRotationProgress struct {
	Total     int64
	Processed int64
	Failed    int64
	Running   bool
}

// KeyRotationJob rotates witness encryption keys from old to new.
type KeyRotationJob struct {
	repo      Repository
	oldEnc    *Encryptor
	newEnc    *Encryptor
	custody   CustodyRecorder
	logger    *slog.Logger
	progress  atomic.Pointer[KeyRotationProgress]
}

// NewKeyRotationJob creates a new key rotation job.
func NewKeyRotationJob(repo Repository, oldEnc, newEnc *Encryptor, custody CustodyRecorder, logger *slog.Logger) *KeyRotationJob {
	job := &KeyRotationJob{
		repo:    repo,
		oldEnc:  oldEnc,
		newEnc:  newEnc,
		custody: custody,
		logger:  logger,
	}
	job.progress.Store(&KeyRotationProgress{})
	return job
}

// Progress returns the current rotation progress.
func (j *KeyRotationJob) Progress() KeyRotationProgress {
	return *j.progress.Load()
}

// Run executes the key rotation, re-encrypting all witnesses.
// It is resumable: if interrupted, calling Run again will process remaining witnesses.
func (j *KeyRotationJob) Run(ctx context.Context) error {
	witnesses, err := j.repo.FindAll(ctx)
	if err != nil {
		return fmt.Errorf("load witnesses for rotation: %w", err)
	}

	total := int64(len(witnesses))
	j.progress.Store(&KeyRotationProgress{
		Total:   total,
		Running: true,
	})

	var processed, failed int64

	for _, w := range witnesses {
		select {
		case <-ctx.Done():
			j.progress.Store(&KeyRotationProgress{
				Total:     total,
				Processed: processed,
				Failed:    failed,
				Running:   false,
			})
			return ctx.Err()
		default:
		}

		// Check if already using new key version (all non-nil fields must be on new version)
		if allFieldsOnVersion(w, j.newEnc.CurrentVersion()) {
			processed++
			j.updateProgress(total, processed, failed, true)
			continue
		}

		if err := j.rotateWitness(ctx, w); err != nil {
			j.logger.Error("failed to rotate witness key",
				"witness_id", w.ID, "error", err)
			failed++
			j.updateProgress(total, processed, failed, true)
			continue
		}

		processed++
		j.updateProgress(total, processed, failed, true)
	}

	j.updateProgress(total, processed, failed, false)
	return nil
}

func (j *KeyRotationJob) rotateWitness(ctx context.Context, w Witness) error {
	witnessID := w.ID.String()

	// Decrypt with old key
	var fullNameEnc, contactInfoEnc, locationEnc []byte

	if w.FullNameEncrypted != nil {
		plaintext, err := j.oldEnc.Decrypt(w.FullNameEncrypted, witnessID, "full_name")
		if err != nil {
			return fmt.Errorf("decrypt full name: %w", err)
		}
		enc, err := j.newEnc.Encrypt(plaintext, witnessID, "full_name")
		if err != nil {
			// unreachable: newEnc.Encrypt cannot fail with a correctly initialised
			// Encryptor (see encryption.go unreachable annotations).
			return fmt.Errorf("re-encrypt full name: %w", err)
		}
		fullNameEnc = enc
	}

	if w.ContactInfoEncrypted != nil {
		plaintext, err := j.oldEnc.Decrypt(w.ContactInfoEncrypted, witnessID, "contact_info")
		if err != nil {
			return fmt.Errorf("decrypt contact info: %w", err)
		}
		enc, err := j.newEnc.Encrypt(plaintext, witnessID, "contact_info")
		if err != nil {
			// unreachable: same reasoning as full_name re-encrypt above.
			return fmt.Errorf("re-encrypt contact info: %w", err)
		}
		contactInfoEnc = enc
	}

	if w.LocationEncrypted != nil {
		plaintext, err := j.oldEnc.Decrypt(w.LocationEncrypted, witnessID, "location")
		if err != nil {
			return fmt.Errorf("decrypt location: %w", err)
		}
		enc, err := j.newEnc.Encrypt(plaintext, witnessID, "location")
		if err != nil {
			// unreachable: same reasoning as full_name re-encrypt above.
			return fmt.Errorf("re-encrypt location: %w", err)
		}
		locationEnc = enc
	}

	// Atomic per-witness update
	if err := j.repo.UpdateEncryptedFields(ctx, w.ID, fullNameEnc, contactInfoEnc, locationEnc); err != nil {
		return fmt.Errorf("update encrypted fields: %w", err)
	}

	// Log rotation event (no identity content)
	if j.custody != nil {
		j.custody.RecordCaseEvent(ctx, w.CaseID, "witness_key_rotated", "system", map[string]string{
			"witness_id": w.ID.String(),
		})
	}

	return nil
}

// allFieldsOnVersion returns true if every non-nil encrypted field is on the given key version.
func allFieldsOnVersion(w Witness, version byte) bool {
	fields := [][]byte{w.FullNameEncrypted, w.ContactInfoEncrypted, w.LocationEncrypted}
	for _, f := range fields {
		if len(f) > 0 && f[0] != version {
			return false
		}
	}
	// At least one field must exist to consider it "already rotated"
	hasAny := len(w.FullNameEncrypted) > 0 || len(w.ContactInfoEncrypted) > 0 || len(w.LocationEncrypted) > 0
	return hasAny
}

func (j *KeyRotationJob) updateProgress(total, processed, failed int64, running bool) {
	j.progress.Store(&KeyRotationProgress{
		Total:     total,
		Processed: processed,
		Failed:    failed,
		Running:   running,
	})
}
