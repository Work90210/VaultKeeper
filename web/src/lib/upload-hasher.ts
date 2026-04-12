import { createSHA256 } from 'hash-wasm';

export type HashProgress = (bytesHashed: number, total: number) => void;

/**
 * Compute a streaming SHA-256 hash of a File in the browser.
 * Processes in 8 MiB chunks to avoid OOM on large files (up to 5 GB).
 *
 * @returns 64-character lowercase hex digest
 */
export async function hashFileStreaming(
  file: File,
  onProgress: HashProgress,
  signal?: AbortSignal,
): Promise<string> {
  const hasher = await createSHA256();
  hasher.init();

  const chunkSize = 8 * 1024 * 1024; // 8 MiB
  let offset = 0;

  while (offset < file.size) {
    if (signal?.aborted) {
      throw new DOMException('Hashing aborted', 'AbortError');
    }

    const end = Math.min(offset + chunkSize, file.size);
    const slice = file.slice(offset, end);
    const buf = new Uint8Array(await slice.arrayBuffer());
    hasher.update(buf);
    offset = end;
    onProgress(offset, file.size);
  }

  return hasher.digest('hex');
}
