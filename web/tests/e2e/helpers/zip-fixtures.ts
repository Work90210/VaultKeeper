import { createHash } from 'node:crypto';
import { writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync } from 'node:fs';
import { execSync } from 'node:child_process';

/**
 * Computes the hex SHA-256 of a string.
 */
function sha256(text: string): string {
  return createHash('sha256').update(text).digest('hex');
}

/**
 * Builds a bulk-upload ZIP on disk and returns the absolute path.
 * The archive contains only a `_metadata.csv` and a handful of files
 * — no `manifest.csv`, so the server takes the bulk-upload path.
 */
export function buildBulkUploadZip(): string {
  const dir = mkdtempSync(join(tmpdir(), 'vk-e2e-bulk-'));
  const files: Record<string, string> = {
    'alpha.txt': 'Alpha content',
    'bravo.txt': 'Bravo content',
    '_metadata.csv':
      'filename,title,description,tags,classification\n' +
      'alpha.txt,Alpha File,Imported via E2E,e2e;bulk,confidential\n' +
      'bravo.txt,Bravo File,Imported via E2E,e2e;bulk,restricted\n',
  };
  for (const [name, content] of Object.entries(files)) {
    writeFileSync(join(dir, name), content);
  }
  const zipPath = join(dir, 'bulk.zip');
  execSync(
    `zip -q -j ${JSON.stringify(zipPath)} ` +
      Object.keys(files)
        .map((n) => JSON.stringify(join(dir, n)))
        .join(' '),
    { stdio: 'inherit' },
  );
  return zipPath;
}

/**
 * Builds a verified-migration ZIP with a `manifest.csv` at the root.
 * The server detects the manifest and routes to the hash-verified
 * migration path.
 */
export function buildVerifiedMigrationZip(): {
  zipPath: string;
  fileHashes: Record<string, string>;
} {
  const dir = mkdtempSync(join(tmpdir(), 'vk-e2e-verified-'));
  const payload: Record<string, string> = {
    'doc-a.pdf': 'Document A E2E content',
    'doc-b.txt': 'Document B E2E content',
  };
  const fileHashes: Record<string, string> = {};
  for (const [name, content] of Object.entries(payload)) {
    writeFileSync(join(dir, name), content);
    fileHashes[name] = sha256(content);
  }
  const manifest =
    'filename,sha256_hash,title,description,source,tags,classification\n' +
    `doc-a.pdf,${fileHashes['doc-a.pdf']},Doc A,E2E verified migration,RelativityOne,e2e;verified,confidential\n` +
    `doc-b.txt,${fileHashes['doc-b.txt']},Doc B,E2E verified migration,RelativityOne,e2e;verified,restricted\n`;
  writeFileSync(join(dir, 'manifest.csv'), manifest);

  const zipPath = join(dir, 'verified.zip');
  execSync(
    `zip -q -j ${JSON.stringify(zipPath)} ` +
      [...Object.keys(payload), 'manifest.csv']
        .map((n) => JSON.stringify(join(dir, n)))
        .join(' '),
    { stdio: 'inherit' },
  );
  return { zipPath, fileHashes };
}

/**
 * Builds a malformed archive (garbage bytes) to exercise the
 * ErrZipRejected path in the bulk/import handler.
 */
export function buildBadArchive(): string {
  const dir = mkdtempSync(join(tmpdir(), 'vk-e2e-bad-'));
  const p = join(dir, 'bad.zip');
  writeFileSync(p, 'not a real zip file');
  return p;
}
