package evidence

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Platform constants (must match CHECK constraint in migration 021).
const (
	PlatformX         = "x"
	PlatformFacebook  = "facebook"
	PlatformInstagram = "instagram"
	PlatformYouTube   = "youtube"
	PlatformTelegram  = "telegram"
	PlatformTikTok    = "tiktok"
	PlatformWhatsApp  = "whatsapp"
	PlatformSignal    = "signal"
	PlatformReddit    = "reddit"
	PlatformWeb       = "web"
	PlatformOther     = "other"
)

var validPlatforms = map[string]bool{
	PlatformX: true, PlatformFacebook: true, PlatformInstagram: true,
	PlatformYouTube: true, PlatformTelegram: true, PlatformTikTok: true,
	PlatformWhatsApp: true, PlatformSignal: true, PlatformReddit: true,
	PlatformWeb: true, PlatformOther: true,
}

// CaptureMethod constants.
const (
	CaptureMethodScreenshot      = "screenshot"
	CaptureMethodScreenRecording = "screen_recording"
	CaptureMethodWebArchive      = "web_archive"
	CaptureMethodAPIExport       = "api_export"
	CaptureMethodManualDownload  = "manual_download"
	CaptureMethodBrowserSave     = "browser_save"
	CaptureMethodForensicTool    = "forensic_tool"
	CaptureMethodOther           = "other"
)

var validCaptureMethods = map[string]bool{
	CaptureMethodScreenshot: true, CaptureMethodScreenRecording: true,
	CaptureMethodWebArchive: true, CaptureMethodAPIExport: true,
	CaptureMethodManualDownload: true, CaptureMethodBrowserSave: true,
	CaptureMethodForensicTool: true, CaptureMethodOther: true,
}

// AvailabilityStatus constants.
const (
	AvailabilityAccessible       = "accessible"
	AvailabilityDeleted          = "deleted"
	AvailabilityGeoBlocked       = "geo_blocked"
	AvailabilityLoginRequired    = "login_required"
	AvailabilityAccountSuspended = "account_suspended"
	AvailabilityRemoved          = "removed"
	AvailabilityUnavailable      = "unavailable"
	AvailabilityUnknown          = "unknown"
)

var validAvailabilityStatuses = map[string]bool{
	AvailabilityAccessible: true, AvailabilityDeleted: true,
	AvailabilityGeoBlocked: true, AvailabilityLoginRequired: true,
	AvailabilityAccountSuspended: true, AvailabilityRemoved: true,
	AvailabilityUnavailable: true, AvailabilityUnknown: true,
}

// VerificationStatus constants.
const (
	VerificationUnverified        = "unverified"
	VerificationPartiallyVerified = "partially_verified"
	VerificationVerified          = "verified"
	VerificationDisputed          = "disputed"
)

var validVerificationStatuses = map[string]bool{
	VerificationUnverified: true, VerificationPartiallyVerified: true,
	VerificationVerified: true, VerificationDisputed: true,
}

// GeoSource constants.
const (
	GeoSourceEXIF             = "exif"
	GeoSourcePlatformMetadata = "platform_metadata"
	GeoSourceManualEntry      = "manual_entry"
	GeoSourceDerived          = "derived"
	GeoSourceUnknown          = "unknown"
)

var validGeoSources = map[string]bool{
	GeoSourceEXIF: true, GeoSourcePlatformMetadata: true,
	GeoSourceManualEntry: true, GeoSourceDerived: true,
	GeoSourceUnknown: true,
}

// PlatformContentType constants.
const (
	ContentTypePost       = "post"
	ContentTypeProfile    = "profile"
	ContentTypeVideo      = "video"
	ContentTypeImage      = "image"
	ContentTypeComment    = "comment"
	ContentTypeStory      = "story"
	ContentTypeLivestream = "livestream"
	ContentTypeChannel    = "channel"
	ContentTypePage       = "page"
	ContentTypeOther      = "other"
)

var validPlatformContentTypes = map[string]bool{
	ContentTypePost: true, ContentTypeProfile: true, ContentTypeVideo: true,
	ContentTypeImage: true, ContentTypeComment: true, ContentTypeStory: true,
	ContentTypeLivestream: true, ContentTypeChannel: true, ContentTypePage: true,
	ContentTypeOther: true,
}

// Allowed keys in network_context JSONB.
var allowedNetworkContextKeys = map[string]bool{
	"vpn_used": true, "tor_used": true, "proxy_used": true,
	"capture_ip_region": true, "notes": true,
}

// Roles that can see collector identity fields.
var collectorVisibleRoles = map[string]bool{
	"investigator": true, "prosecutor": true, "judge": true,
}

// Roles that can see network context fields.
var networkContextVisibleRoles = map[string]bool{
	"investigator": true, "prosecutor": true,
}

// EvidenceCaptureMetadata is the domain model for Berkeley Protocol capture provenance.
type EvidenceCaptureMetadata struct {
	ID         uuid.UUID `json:"id"`
	EvidenceID uuid.UUID `json:"evidence_id"`

	// Source identification
	SourceURL           *string `json:"source_url,omitempty"`
	CanonicalURL        *string `json:"canonical_url,omitempty"`
	Platform            *string `json:"platform,omitempty"`
	PlatformContentType *string `json:"platform_content_type,omitempty"`

	// Capture context
	CaptureMethod        string     `json:"capture_method"`
	CaptureTimestamp     time.Time  `json:"capture_timestamp"`
	PublicationTimestamp  *time.Time `json:"publication_timestamp,omitempty"`

	// Collector identity — gated by role in response serialization
	CollectorUserID                *uuid.UUID `json:"collector_user_id,omitempty"`
	CollectorDisplayNameEncrypted  []byte     `json:"-"`
	CollectorDisplayName           *string    `json:"collector_display_name,omitempty"`

	// Content creator
	CreatorAccountHandle     *string `json:"creator_account_handle,omitempty"`
	CreatorAccountDisplayName *string `json:"creator_account_display_name,omitempty"`
	CreatorAccountURL        *string `json:"creator_account_url,omitempty"`
	CreatorAccountID         *string `json:"creator_account_id,omitempty"`

	// Content metadata
	ContentDescription *string `json:"content_description,omitempty"`
	ContentLanguage    *string `json:"content_language,omitempty"`

	// Geolocation
	GeoLatitude  *float64 `json:"geo_latitude,omitempty"`
	GeoLongitude *float64 `json:"geo_longitude,omitempty"`
	GeoPlaceName *string  `json:"geo_place_name,omitempty"`
	GeoSource    *string  `json:"geo_source,omitempty"`

	// Availability
	AvailabilityStatus *string `json:"availability_status,omitempty"`
	WasLive            *bool   `json:"was_live,omitempty"`
	WasDeleted         *bool   `json:"was_deleted,omitempty"`

	// Capture environment
	CaptureToolName    *string `json:"capture_tool_name,omitempty"`
	CaptureToolVersion *string `json:"capture_tool_version,omitempty"`
	BrowserName        *string `json:"browser_name,omitempty"`
	BrowserVersion     *string `json:"browser_version,omitempty"`
	BrowserUserAgent   *string `json:"browser_user_agent,omitempty"`

	// Network context — gated by role in response serialization
	NetworkContext map[string]any `json:"network_context,omitempty"`

	// Preservation & verification
	PreservationNotes  *string `json:"preservation_notes,omitempty"`
	VerificationStatus string  `json:"verification_status"`
	VerificationNotes  *string `json:"verification_notes,omitempty"`

	MetadataSchemaVersion int       `json:"metadata_schema_version"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// RedactForRole removes sensitive fields based on caller's case role.
func (m EvidenceCaptureMetadata) RedactForRole(role string) EvidenceCaptureMetadata {
	if !collectorVisibleRoles[role] {
		m.CollectorUserID = nil
		m.CollectorDisplayName = nil
		m.CollectorDisplayNameEncrypted = nil
	}
	if !networkContextVisibleRoles[role] {
		m.NetworkContext = nil
		m.BrowserUserAgent = nil
	}
	return m
}

// CaptureMetadataInput is the validated input for upserting capture metadata.
type CaptureMetadataInput struct {
	SourceURL           *string `json:"source_url"`
	CanonicalURL        *string `json:"canonical_url"`
	Platform            *string `json:"platform"`
	PlatformContentType *string `json:"platform_content_type"`

	CaptureMethod       string  `json:"capture_method"`
	CaptureTimestamp    string  `json:"capture_timestamp"`
	PublicationTimestamp *string `json:"publication_timestamp"`

	CollectorUserID     *string `json:"collector_user_id"`
	CollectorDisplayName *string `json:"collector_display_name"`

	CreatorAccountHandle      *string `json:"creator_account_handle"`
	CreatorAccountDisplayName *string `json:"creator_account_display_name"`
	CreatorAccountURL         *string `json:"creator_account_url"`
	CreatorAccountID          *string `json:"creator_account_id"`

	ContentDescription *string `json:"content_description"`
	ContentLanguage    *string `json:"content_language"`

	GeoLatitude  *float64 `json:"geo_latitude"`
	GeoLongitude *float64 `json:"geo_longitude"`
	GeoPlaceName *string  `json:"geo_place_name"`
	GeoSource    *string  `json:"geo_source"`

	AvailabilityStatus *string `json:"availability_status"`
	WasLive            *bool   `json:"was_live"`
	WasDeleted         *bool   `json:"was_deleted"`

	CaptureToolName    *string `json:"capture_tool_name"`
	CaptureToolVersion *string `json:"capture_tool_version"`
	BrowserName        *string `json:"browser_name"`
	BrowserVersion     *string `json:"browser_version"`
	BrowserUserAgent   *string `json:"browser_user_agent"`

	NetworkContext map[string]any `json:"network_context"`

	PreservationNotes  *string `json:"preservation_notes"`
	VerificationStatus *string `json:"verification_status"`
	VerificationNotes  *string `json:"verification_notes"`
}

// CaptureMetadataWarning is a non-fatal validation issue.
type CaptureMetadataWarning struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidateCaptureMetadataInput validates the input and returns errors and warnings.
func ValidateCaptureMetadataInput(input CaptureMetadataInput) ([]CaptureMetadataWarning, error) {
	var warnings []CaptureMetadataWarning

	// Required: capture_method
	if input.CaptureMethod == "" {
		return nil, &ValidationError{Field: "capture_method", Message: "capture method is required"}
	}
	if !validCaptureMethods[input.CaptureMethod] {
		return nil, &ValidationError{Field: "capture_method", Message: fmt.Sprintf("invalid capture method: %s", input.CaptureMethod)}
	}

	// Required: capture_timestamp
	if input.CaptureTimestamp == "" {
		return nil, &ValidationError{Field: "capture_timestamp", Message: "capture timestamp is required"}
	}
	captureTS, err := time.Parse(time.RFC3339, input.CaptureTimestamp)
	if err != nil {
		return nil, &ValidationError{Field: "capture_timestamp", Message: "capture timestamp must be RFC 3339 format"}
	}

	// publication_timestamp vs capture_timestamp: warn if pub > capture
	if input.PublicationTimestamp != nil && *input.PublicationTimestamp != "" {
		pubTS, err := time.Parse(time.RFC3339, *input.PublicationTimestamp)
		if err != nil {
			return nil, &ValidationError{Field: "publication_timestamp", Message: "publication timestamp must be RFC 3339 format"}
		}
		if pubTS.After(captureTS) {
			warnings = append(warnings, CaptureMetadataWarning{
				Field:   "publication_timestamp",
				Message: "publication timestamp is after capture timestamp",
			})
		}
	}

	// URL scheme validation
	if err := validateHTTPURL(input.SourceURL, "source_url"); err != nil {
		return nil, err
	}
	if err := validateHTTPURL(input.CanonicalURL, "canonical_url"); err != nil {
		return nil, err
	}
	if err := validateHTTPURL(input.CreatorAccountURL, "creator_account_url"); err != nil {
		return nil, err
	}

	// Platform validation
	if input.Platform != nil && *input.Platform != "" && !validPlatforms[*input.Platform] {
		return nil, &ValidationError{Field: "platform", Message: fmt.Sprintf("invalid platform: %s", *input.Platform)}
	}

	// Platform content type validation
	if input.PlatformContentType != nil && *input.PlatformContentType != "" && !validPlatformContentTypes[*input.PlatformContentType] {
		return nil, &ValidationError{Field: "platform_content_type", Message: fmt.Sprintf("invalid platform content type: %s", *input.PlatformContentType)}
	}

	// Geo pair constraint
	hasLat := input.GeoLatitude != nil
	hasLon := input.GeoLongitude != nil
	if hasLat != hasLon {
		return nil, &ValidationError{Field: "geo_latitude", Message: "geo_latitude and geo_longitude must both be provided or both omitted"}
	}

	// Geo source validation
	if input.GeoSource != nil && *input.GeoSource != "" && !validGeoSources[*input.GeoSource] {
		return nil, &ValidationError{Field: "geo_source", Message: fmt.Sprintf("invalid geo source: %s", *input.GeoSource)}
	}

	// Availability status validation
	if input.AvailabilityStatus != nil && *input.AvailabilityStatus != "" && !validAvailabilityStatuses[*input.AvailabilityStatus] {
		return nil, &ValidationError{Field: "availability_status", Message: fmt.Sprintf("invalid availability status: %s", *input.AvailabilityStatus)}
	}

	// Verification status validation
	if input.VerificationStatus != nil && *input.VerificationStatus != "" && !validVerificationStatuses[*input.VerificationStatus] {
		return nil, &ValidationError{Field: "verification_status", Message: fmt.Sprintf("invalid verification status: %s", *input.VerificationStatus)}
	}

	// Content language: basic BCP 47 / ISO 639-1 check (2-3 letter code, optional subtag)
	if input.ContentLanguage != nil && *input.ContentLanguage != "" {
		if !isValidLanguageCode(*input.ContentLanguage) {
			return nil, &ValidationError{Field: "content_language", Message: "invalid language code (expected BCP 47 format, e.g. 'en', 'fr', 'zh-Hans')"}
		}
	}

	// Network context: validate allowed keys and enforce max size
	if input.NetworkContext != nil {
		for key := range input.NetworkContext {
			if !allowedNetworkContextKeys[key] {
				return nil, &ValidationError{Field: "network_context", Message: fmt.Sprintf("unknown network context key: %s", key)}
			}
		}
		// Enforce max serialized size (4KB) to prevent oversized JSONB payloads
		if encoded, err := json.Marshal(input.NetworkContext); err == nil && len(encoded) > 4096 {
			return nil, &ValidationError{Field: "network_context", Message: "network context exceeds maximum size (4KB)"}
		}
	}

	// Max length on free-text fields (prevent oversized row storage)
	const maxTextFieldLen = 10000
	const maxShortFieldLen = 500
	textFields := map[string]*string{
		"content_description": input.ContentDescription,
		"preservation_notes":  input.PreservationNotes,
		"verification_notes":  input.VerificationNotes,
	}
	for field, ptr := range textFields {
		if ptr != nil && len(*ptr) > maxTextFieldLen {
			return nil, &ValidationError{Field: field, Message: fmt.Sprintf("%s exceeds maximum length (%d characters)", field, maxTextFieldLen)}
		}
	}
	shortFields := map[string]*string{
		"creator_account_handle":       input.CreatorAccountHandle,
		"creator_account_display_name": input.CreatorAccountDisplayName,
		"creator_account_id":           input.CreatorAccountID,
		"capture_tool_name":            input.CaptureToolName,
		"capture_tool_version":         input.CaptureToolVersion,
		"browser_name":                 input.BrowserName,
		"browser_version":              input.BrowserVersion,
		"browser_user_agent":           input.BrowserUserAgent,
		"geo_place_name":               input.GeoPlaceName,
	}
	for field, ptr := range shortFields {
		if ptr != nil && len(*ptr) > maxShortFieldLen {
			return nil, &ValidationError{Field: field, Message: fmt.Sprintf("%s exceeds maximum length (%d characters)", field, maxShortFieldLen)}
		}
	}

	// Collector user ID validation
	if input.CollectorUserID != nil && *input.CollectorUserID != "" {
		if _, err := uuid.Parse(*input.CollectorUserID); err != nil {
			return nil, &ValidationError{Field: "collector_user_id", Message: "invalid UUID for collector_user_id"}
		}
	}

	return warnings, nil
}

// validateHTTPURL ensures a URL pointer, if non-nil and non-empty, uses http or https scheme.
func validateHTTPURL(u *string, field string) error {
	if u == nil || *u == "" {
		return nil
	}
	parsed, err := url.Parse(*u)
	if err != nil {
		return &ValidationError{Field: field, Message: "invalid URL"}
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return &ValidationError{Field: field, Message: "URL must use http or https scheme"}
	}
	return nil
}

// isValidLanguageCode performs a basic BCP 47 check: 2-3 letter primary subtag,
// optional additional subtags separated by hyphens.
func isValidLanguageCode(code string) bool {
	if len(code) < 2 || len(code) > 35 {
		return false
	}
	parts := strings.Split(code, "-")
	primary := parts[0]
	if len(primary) < 2 || len(primary) > 3 {
		return false
	}
	for _, c := range primary {
		if c < 'a' || c > 'z' {
			if c < 'A' || c > 'Z' {
				return false
			}
		}
	}
	return true
}
