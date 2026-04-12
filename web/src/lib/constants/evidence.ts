/**
 * Evidence-related constants mirrored from the Go backend.
 *
 * These MUST be kept in sync with the following Go declarations until a
 * runtime /api/config endpoint exposes them dynamically:
 *
 *   internal/evidence/destruction.go → MinDestructionAuthorityLength
 *   internal/evidence/models.go      → MaxTagLength, MaxTagCount
 *
 * When you change one of the Go constants, update this file too.
 */

/**
 * Minimum length (characters) for the destruction authority string in the
 * audited destruction workflow. The server rejects anything shorter.
 */
export const MIN_DESTRUCTION_AUTHORITY_LENGTH = 10;

/**
 * Maximum length (characters) of a single tag. Matches the server-side
 * `MaxTagLength` constant.
 */
export const MAX_TAG_LENGTH = 100;

/**
 * Maximum number of tags on a single evidence item. Matches the server-side
 * `MaxTagCount` constant.
 */
export const MAX_TAGS_PER_EVIDENCE = 50;
