import { createHash, randomUUID } from 'node:crypto';
import { execFile } from 'node:child_process';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { promisify } from 'node:util';
import { downloadMediaMessage } from '@whiskeysockets/baileys';
const execute = promisify(execFile);
export async function makeSticker(connection, message, logger) {
  const stream = await downloadMediaMessage(message, 'stream', {}, { logger, reuploadRequest: connection.socket.updateMediaMessage });
  const chunks = []; let size = 0;
  for await (const chunk of stream) {
    size += chunk.length;
    if (size > 16 * 1024 * 1024) { stream.destroy(); throw new Error('media exceeds 16 MiB'); }
    chunks.push(chunk);
  }
  const directory = await mkdtemp(join(tmpdir(), 'space-sticker-'));
  try {
    const input = join(directory, randomUUID() + '.input'), output = join(directory, 'sticker.webp');
    await writeFile(input, Buffer.concat(chunks), { mode: 0o600 });
    await execute('ffmpeg', ['-hide_banner', '-loglevel', 'error', '-y', '-i', input, '-t', '6', '-vf', 'fps=12,scale=512:512:force_original_aspect_ratio=decrease,pad=512:512:(ow-iw)/2:(oh-ih)/2:color=0x00000000,format=rgba', '-an', '-loop', '0', '-c:v', 'libwebp', '-quality', '75', output], { timeout: 25_000, maxBuffer: 1024 * 1024 });
    const bytes = await readFile(output);
    if (bytes.length > 1024 * 1024) throw new Error('sticker output exceeds 1 MiB');
    const assetId = createHash('sha256').update(bytes).digest('hex');
    await connection.store.write(`asset:${assetId}`, { bytes, mime: 'image/webp' });
    return { assetId, mime: 'image/webp', size: bytes.length };
  } finally { await rm(directory, { recursive: true, force: true }); }
}
