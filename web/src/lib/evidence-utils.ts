/** Shared evidence utility functions and constants. */

export function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  const size = bytes / Math.pow(1024, i);
  return `${size.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

export function mimeLabel(mimeType: string): string {
  if (mimeType.startsWith('image/')) return 'Image';
  if (mimeType.startsWith('video/')) return 'Video';
  if (mimeType.startsWith('audio/')) return 'Audio';
  if (mimeType.includes('pdf')) return 'PDF';
  if (mimeType.includes('word') || mimeType.includes('document'))
    return 'Document';
  if (mimeType.includes('spreadsheet') || mimeType.includes('excel'))
    return 'Spreadsheet';
  if (mimeType.includes('zip') || mimeType.includes('archive'))
    return 'Archive';
  return 'File';
}

export function mimeIcon(mimeType: string): string {
  if (mimeType.startsWith('image/')) return '\u{1F5BC}';
  if (mimeType.startsWith('video/')) return '\u{1F3AC}';
  if (mimeType.startsWith('audio/')) return '\u{1F3B5}';
  if (mimeType.includes('pdf')) return '\u{1F4C4}';
  if (mimeType.includes('spreadsheet') || mimeType.includes('excel'))
    return '\u{1F4CA}';
  return '\u{1F4CE}';
}

export const CLASSIFICATION_STYLES: Record<
  string,
  { color: string; bg: string }
> = {
  public: { color: 'var(--status-active)', bg: 'var(--status-active-bg)' },
  restricted: { color: 'var(--status-closed)', bg: 'var(--status-closed-bg)' },
  confidential: { color: 'var(--status-hold)', bg: 'var(--status-hold-bg)' },
  ex_parte: {
    color: 'var(--status-archived)',
    bg: 'var(--status-archived-bg)',
  },
};
