# Sprint 20: Mobile Evidence Capture (Flutter)

**Phase:** 4 — Enterprise & Scale
**Duration:** Weeks 39-40
**Goal:** Build a Flutter mobile app for field investigators to capture photo/video/audio evidence in the field, with offline-first sync to VaultKeeper when connectivity is available.

---

## Prerequisites

- Phase 3 complete (external API, API key auth)
- API endpoints for evidence upload operational

---

## Task Type

- [x] Mobile (Flutter/Dart)
- [x] Backend (Go — minor API enhancements)

---

## Implementation Steps

### Step 1: Flutter Project Setup

**Project structure:**
```
mobile/
├── lib/
│   ├── main.dart
│   ├── app/
│   │   ├── app.dart
│   │   └── routes.dart
│   ├── features/
│   │   ├── auth/
│   │   │   ├── login_screen.dart
│   │   │   └── auth_service.dart
│   │   ├── capture/
│   │   │   ├── camera_screen.dart
│   │   │   ├── audio_recorder_screen.dart
│   │   │   ├── capture_service.dart
│   │   │   └── metadata_form.dart
│   │   ├── evidence/
│   │   │   ├── evidence_list_screen.dart
│   │   │   ├── evidence_detail_screen.dart
│   │   │   └── evidence_repository.dart
│   │   ├── sync/
│   │   │   ├── sync_service.dart
│   │   │   ├── sync_queue.dart
│   │   │   └── sync_status_widget.dart
│   │   └── cases/
│   │       ├── case_list_screen.dart
│   │       └── case_repository.dart
│   ├── core/
│   │   ├── api/
│   │   │   └── api_client.dart
│   │   ├── database/
│   │   │   └── local_db.dart
│   │   ├── crypto/
│   │   │   ├── hashing.dart
│   │   │   └── encryption.dart
│   │   └── models/
│   │       └── evidence_item.dart
│   └── shared/
│       ├── widgets/
│       └── utils/
├── test/
├── pubspec.yaml
├── android/
└── ios/
```

**Key dependencies:**
- `camera` — photo/video capture
- `record` — audio recording
- `sqflite` / `drift` — local SQLite database
- `connectivity_plus` — network status
- `geolocator` — GPS coordinates
- `crypto` — SHA-256 hashing
- `encrypt` — AES-256-GCM local encryption
- `exif` — EXIF metadata reading

### Step 2: Offline-First Evidence Capture

**Capture flow:**
1. Investigator opens camera/audio recorder
2. Capture photo, video, or audio
3. Auto-tag with GPS coordinates and timestamp
4. Compute SHA-256 hash immediately on device
5. Encrypt file locally (AES-256-GCM with device key)
6. Store in local SQLite database + local file storage
7. Add to sync queue

**Local evidence model:**
```dart
class LocalEvidence {
  String id;
  String caseId;
  String filePath;       // local encrypted file
  String sha256Hash;     // computed on capture
  String mimeType;
  int fileSize;
  double? latitude;
  double? longitude;
  DateTime capturedAt;
  String? title;
  String? description;
  List<String> tags;
  SyncStatus syncStatus; // pending, syncing, synced, failed
  DateTime? syncedAt;
}
```

**Offline storage:**
- Local SQLite for metadata
- Local file system for encrypted evidence files
- Max local storage: configurable (default 10GB)
- Storage usage indicator in UI

**Tests:**
- Photo capture → stored locally with hash
- Video capture → stored locally with hash
- Audio recording → stored locally with hash
- GPS tagged correctly
- Timestamp accurate
- SHA-256 computed on device
- Local encryption working
- SQLite storage persists across app restarts
- Storage usage tracked correctly

### Step 3: Sync Service

**Deliverable:** Background sync when connectivity available.

**Sync flow:**
1. Monitor network connectivity
2. When online → process sync queue (oldest first)
3. For each item:
   a. Decrypt local file
   b. Upload to VaultKeeper API (tus protocol for resumable upload)
   c. Server computes its own SHA-256
   d. Server confirms hash matches client-computed hash
   e. If match → mark synced, optionally delete local copy
   f. If mismatch → flag as error, keep local copy, alert user
4. Sync progress visible in UI

**Conflict resolution:**
- Client hash === server hash → success
- Client hash !== server hash → sync error (file corrupted in transit)
- Network interruption → tus resumes from last chunk
- Server rejected (auth, permission) → show error, keep in queue

**Background sync:**
- Runs as background service (Android WorkManager, iOS Background Tasks)
- Respects battery level (pause below 15%)
- Respects connection type (option: WiFi only for large files)
- Retry with exponential backoff on failure

**Tests:**
- Online → sync completes, evidence appears on server
- Offline → queued, syncs when online
- Network interruption mid-upload → resumes
- Hash mismatch → flagged, not synced
- Large file (500MB video) → chunked upload with progress
- Multiple items in queue → processed sequentially
- Background sync triggers when connectivity changes
- WiFi-only mode → skips sync on mobile data

### Step 4: Mobile Authentication

**Deliverable:** Keycloak OIDC login + API key fallback.

**Auth options:**
1. **OIDC login:** Full Keycloak flow (when server reachable)
   - PKCE flow via system browser
   - Token stored in secure keychain (iOS) / Keystore (Android)
   - Auto-refresh
2. **API key:** Pre-configured key for field use (when Keycloak unreachable)
   - Key entered once, stored in secure keychain
   - Scoped to specific cases

**Offline auth:**
- Cached JWT valid for up to 8 hours (refresh token lifetime)
- API key works offline (no server validation needed for local capture)
- Capture allowed offline regardless of auth method
- Sync requires valid auth

### Step 5: Mobile UI

**Screens:**
- **Login** — Keycloak OIDC or API key input
- **Dashboard** — Case list, sync status, storage usage
- **Case Detail** — Evidence list for case (local + synced)
- **Camera** — Photo/video capture with GPS overlay
- **Audio Recorder** — Audio recording with timer
- **Metadata Form** — Title, description, tags for captured evidence
- **Sync Queue** — List of pending uploads with status
- **Settings** — Server URL, sync preferences, storage management

**UX priorities:**
- One-tap capture (minimize steps between opening app and capturing evidence)
- Offline indicator always visible
- Sync status always visible
- Large touch targets (field conditions, gloves)
- High contrast mode for outdoor use

### Step 6: Backend API Enhancements

**New/modified endpoints for mobile:**
```
POST /api/evidence/mobile-upload    → Upload with client-provided hash for verification
GET  /api/sync/status               → Check sync status for evidence IDs
POST /api/evidence/batch-metadata   → Update metadata for multiple items (post-sync)
```

**Client hash verification:**
- Mobile client sends `X-Client-Hash: sha256:abc123...` header
- Server computes its own hash
- If match → 201 with hash verification confirmation
- If mismatch → 409 with both hashes for debugging

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `mobile/lib/features/capture/*` | Create | Camera, audio, capture service |
| `mobile/lib/features/sync/*` | Create | Sync queue, sync service |
| `mobile/lib/features/auth/*` | Create | OIDC + API key auth |
| `mobile/lib/core/crypto/*` | Create | Local hashing + encryption |
| `mobile/lib/core/database/*` | Create | SQLite local storage |
| `mobile/test/*` | Create | Full test suite |
| `internal/evidence/handler.go` | Modify | Mobile upload with hash verification |

---

## Definition of Done

- [ ] Photo/video/audio capture works on iOS + Android
- [ ] GPS + timestamp auto-tagged on capture
- [ ] SHA-256 computed on device immediately
- [ ] Evidence encrypted locally
- [ ] Offline capture works without server connectivity
- [ ] Sync uploads to VaultKeeper when online
- [ ] tus resumable upload for large files
- [ ] Hash verification: client hash must match server hash
- [ ] Background sync respects battery + connectivity
- [ ] OIDC + API key auth working
- [ ] 100% test coverage on crypto + sync logic

---

## Security Checklist

- [ ] Evidence encrypted at rest on device (AES-256-GCM)
- [ ] Encryption key stored in secure keychain/keystore
- [ ] JWT tokens stored in secure keychain/keystore
- [ ] API keys stored in secure keychain/keystore
- [ ] No evidence data in app logs
- [ ] Device wipe → encrypted data irrecoverable
- [ ] HTTPS only for sync (certificate pinning optional)
- [ ] Hash computed before encryption (proves original content)
- [ ] Failed sync doesn't delete local evidence

---

## Test Coverage Requirements (100% Target)

Every line of code introduced in Sprint 20 must be covered by automated tests. CI blocks merge if coverage drops below 100% for new code.

### Unit Tests

- **`mobile/lib/features/capture/capture_service.dart`** — photo capture: creates LocalEvidence with correct mimeType (image/jpeg); GPS coordinates populated when location permission granted; GPS null when permission denied (capture still succeeds); SHA-256 hash computed and matches manual calculation; file size recorded accurately
- **`mobile/lib/features/capture/capture_service.dart`** — video capture: creates LocalEvidence with correct mimeType (video/mp4); duration recorded; hash computed on full file; GPS tagged at capture start
- **`mobile/lib/features/capture/capture_service.dart`** — audio recording: creates LocalEvidence with correct mimeType (audio/aac); duration recorded; hash computed on completed recording
- **`mobile/lib/core/crypto/hashing.dart`** — SHA-256: known input produces expected hash; empty file produces correct empty-file hash; large file (500MB) produces hash without OOM; hash is deterministic (same file always same hash)
- **`mobile/lib/core/crypto/encryption.dart`** — AES-256-GCM encrypt: ciphertext differs from plaintext; ciphertext length is plaintext length + overhead; different keys produce different ciphertext
- **`mobile/lib/core/crypto/encryption.dart`** — AES-256-GCM decrypt: correct key returns original bytes; wrong key throws error; tampered ciphertext throws authentication error
- **`mobile/lib/core/database/local_db.dart`** — CRUD operations: insert LocalEvidence, read by ID, read all by case, update sync status, delete after sync; database persists across simulated app restart
- **`mobile/lib/features/sync/sync_queue.dart`** — enqueue: adds item with status "pending"; queue ordered by capturedAt (oldest first); duplicate evidence ID not re-added
- **`mobile/lib/features/sync/sync_service.dart`** — sync success flow: decrypts local file, uploads via tus, server confirms hash match, marks "synced", optionally deletes local copy
- **`mobile/lib/features/sync/sync_service.dart`** — sync hash mismatch: server returns 409, item marked "failed" with error details, local copy preserved
- **`mobile/lib/features/sync/sync_service.dart`** — sync network failure: item remains "pending", retry with exponential backoff, respects max retry count
- **`mobile/lib/features/sync/sync_service.dart`** — background sync: triggers on connectivity change (offline to online); pauses below 15% battery; WiFi-only mode skips sync on mobile data
- **`mobile/lib/features/sync/sync_service.dart`** — tus resumable upload: interrupted upload resumes from last chunk on next attempt; completed chunks not re-uploaded
- **`mobile/lib/features/auth/auth_service.dart`** — OIDC login: PKCE flow initiated correctly; tokens stored in secure keychain; auto-refresh before expiry
- **`mobile/lib/features/auth/auth_service.dart`** — API key auth: key stored in secure keychain; key used in X-API-Key header; offline capture allowed without server validation
- **Storage management** — storage usage tracked correctly; warning at 80% of configured limit; capture blocked at 100% with user-friendly message
- **Backend `POST /api/evidence/mobile-upload`** — client hash header parsed; server hash computed; match returns 201 with confirmation; mismatch returns 409 with both hashes

### Integration Tests (testcontainers)

- **Mobile upload end-to-end** — capture evidence on device (simulated), sync to VaultKeeper API, verify evidence item created in database with matching hash, file stored in MinIO
- **Hash verification round-trip** — capture photo, compute SHA-256 on device, upload with X-Client-Hash header, verify server computes identical hash and returns 201
- **Hash mismatch detection** — upload file with intentionally wrong X-Client-Hash header, verify server returns 409 with both hashes for debugging
- **Tus resumable upload** — start uploading 100MB file, interrupt at 50%, resume upload, verify complete file received with correct hash
- **Batch metadata update** — sync 5 evidence items, then send batch metadata update (titles, descriptions, tags), verify all 5 items updated in database
- **Sync status endpoint** — sync 3 items, call GET /api/sync/status with their IDs, verify all 3 show status "received" with correct hashes
- **OIDC + API key fallback** — authenticate with OIDC token, upload succeeds; authenticate with API key, upload succeeds; no auth returns 401

### E2E Automated Tests (Playwright)

Note: Mobile E2E tests use Flutter integration testing framework rather than Playwright. The following scenarios are automated via `flutter_test` and `integration_test`.

- **Photo capture flow** — open app, select case, tap camera button, capture photo, verify metadata form appears with GPS coordinates and timestamp pre-filled; save; verify evidence appears in local evidence list with "Pending sync" badge
- **Video capture flow** — tap video button, record for 10 seconds, stop; verify file saved locally with duration, GPS, and hash; verify "Pending sync" badge
- **Audio recording flow** — tap audio button, record for 30 seconds, stop; verify file saved locally with duration and hash; verify "Pending sync" badge
- **Offline capture** — enable airplane mode, capture photo, verify success with "Offline" indicator visible; verify evidence stored locally
- **Sync when online** — disable airplane mode after capturing 3 items offline; verify sync starts automatically; verify all 3 items transition from "Pending" to "Synced" with checkmark
- **Sync progress visibility** — during sync of a large video file, verify progress bar shows upload percentage; verify sync queue screen shows all pending items with individual status
- **Hash verification on sync** — after sync completes, verify the evidence detail on the server shows "Hash verified: client and server match"
- **GPS tagging accuracy** — capture evidence at a known location, verify GPS coordinates in metadata are within 50 meters of actual position
- **Storage usage indicator** — capture multiple items, verify dashboard shows updated storage usage; verify warning appears when approaching limit

---

## Manual E2E Testing Checklist

1. [ ] **Action:** Open the VaultKeeper mobile app on an Android device, log in via Keycloak OIDC, select a case, and capture a photo of a physical document
   **Expected:** Camera opens with GPS overlay showing current coordinates; photo captured; metadata form appears with GPS, timestamp, and suggested title; after saving, evidence appears in local list with "Pending sync" badge
   **Verify:** Check local SQLite database for the evidence entry; verify SHA-256 hash is stored; verify GPS coordinates match device's actual location (within 50m); verify file exists in local encrypted storage

2. [ ] **Action:** Record a 2-minute video of a location using the mobile app
   **Expected:** Video recording starts with timer and GPS overlay; recording stops when tapping stop button; video saved locally with hash computed; metadata form appears
   **Verify:** Verify video plays back correctly from local storage; verify file size is reasonable for 2 minutes of video; verify SHA-256 hash is present in metadata; verify GPS coordinates tagged

3. [ ] **Action:** Record a 5-minute audio interview using the audio recorder
   **Expected:** Audio recording starts with timer display; recording stops when tapping stop; audio saved locally with hash; metadata form appears for title and description
   **Verify:** Play back the audio from local storage; verify duration matches approximately 5 minutes; verify hash is computed and stored

4. [ ] **Action:** Enable airplane mode on the device, then capture 3 evidence items (1 photo, 1 video, 1 audio)
   **Expected:** All 3 captures succeed; "Offline" indicator visible throughout; all items appear in local evidence list with "Pending sync" badges; no error messages about connectivity
   **Verify:** All 3 items in local SQLite database with syncStatus "pending"; all files exist in local encrypted storage; hashes computed for all 3

5. [ ] **Action:** Disable airplane mode and observe the sync behavior
   **Expected:** Sync starts automatically within 30 seconds of connectivity; sync queue shows all 3 items being processed sequentially (oldest first); each item transitions from "Syncing" to "Synced" with progress indication
   **Verify:** After all 3 items sync, check VaultKeeper web UI; verify all 3 evidence items appear in the case with correct metadata, GPS, timestamps, and "Hash verified" status

6. [ ] **Action:** Start uploading a large video file (500MB+), then walk into an area with no connectivity mid-upload
   **Expected:** Upload pauses when connectivity drops; "Offline" indicator appears; when connectivity returns, upload resumes from where it left off (tus resumable); upload completes successfully
   **Verify:** Check server-side evidence item; verify file is complete and hash matches; verify no duplicate uploads occurred; verify tus upload offset was preserved

7. [ ] **Action:** After syncing evidence, check the custody chain on the VaultKeeper web UI for one of the mobile-captured items
   **Expected:** Custody chain shows: (1) "Captured on mobile device" with device info, GPS, and timestamp; (2) "Synced to server" with hash verification result; (3) hash matches between client-computed and server-computed values
   **Verify:** Custody chain entries include device identifier (not device secrets); GPS coordinates match those in the evidence metadata; timestamps are in correct chronological order

8. [ ] **Action:** Attempt to capture evidence when device storage is nearly full (simulate by filling storage to within 100MB of the configured limit)
   **Expected:** Storage usage indicator shows warning (e.g., "Storage 95% full"); capture is allowed but warns user; when storage is 100% full, capture is blocked with message "Insufficient storage - sync or delete local evidence"
   **Verify:** Warning threshold triggers at correct percentage; blocking threshold prevents new captures; existing evidence is not affected; sync is still possible to free up space

9. [ ] **Action:** Log in using an API key (instead of OIDC) on a device without reliable internet for Keycloak authentication
   **Expected:** API key entered once and stored securely; app shows cases scoped to the API key; capture works immediately; sync uses the API key for authentication
   **Verify:** Evidence uploads use X-API-Key header; API key is not visible in app logs; key persists across app restarts; synced evidence shows API key ID (not value) in custody chain

10. [ ] **Action:** Capture evidence, sync it, then verify the hash on the server matches the hash computed on the device
    **Expected:** Evidence detail on server shows both client hash and server hash; both are identical SHA-256 values; "Integrity: Verified" badge displayed
    **Verify:** Copy both hash values and compare manually; they must be identical; if they differ, the system should have rejected the upload with a 409 error (which it did not, so they match)
