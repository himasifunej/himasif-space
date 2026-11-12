import { createHash, randomUUID } from 'node:crypto';
import { execFile } from 'node:child_process';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { promisify } from 'node:util';
import { downloadMediaMessage, normalizeMessageContent } from '@whiskeysockets/baileys';
const execute = promisify(execFile);
export const INPUT_LIMIT = 16 * 1024 * 1024;
export const STICKER_LIMIT = 1024 * 1024;
// A GIF is an mp4 WhatsApp plays muted and looping (gifPlayback). Space's
// server checks the same limits again before saving a command.
export const GIF_LIMIT = 4 * 1024 * 1024;
export const GIF_SECONDS = 10;
// An image has no length of its own; it becomes a still clip this long.
export const GIF_STILL_SECONDS = 3;
async function download(connection, message, logger) {
  const stream = await downloadMediaMessage(message, 'stream', {}, { logger, reuploadRequest: connection.socket.updateMediaMessage });
  const chunks = []; let size = 0;
  for await (const chunk of stream) {
    size += chunk.length;
    if (size > INPUT_LIMIT) { stream.destroy(); throw new Error('media exceeds 16 MiB'); }
    chunks.push(chunk);
  }
  return Buffer.concat(chunks);
}
// convert runs ffmpeg on input in a private directory and returns the output
// file. Arguments are fixed; nothing from a message reaches a shell.
async function convert(input, prefix, name, args) {
  const directory = await mkdtemp(join(tmpdir(), prefix));
  try {
    const source = join(directory, randomUUID() + '.input'), output = join(directory, name);
    await writeFile(source, input, { mode: 0o600 });
    await execute('ffmpeg', ['-hide_banner', '-loglevel', 'error', '-y', ...args(source), output], { timeout: 25_000, maxBuffer: 1024 * 1024 });
    const bytes = await readFile(output);
    return { bytes, output, directory };
  } catch (error) { await rm(directory, { recursive: true, force: true }); throw error; }
}
// convertSticker makes a 512×512 WebP. A still image skips the fps filter:
// ffmpeg 7 drops a single frame without a duration there and writes nothing.
export async function convertSticker(input, still) {
  const { bytes, directory } = await convert(input, 'space-sticker-', 'sticker.webp', (source) => ['-i', source, '-t', '6', '-vf', (still ? '' : 'fps=12,') + 'scale=512:512:force_original_aspect_ratio=decrease,pad=512:512:(ow-iw)/2:(oh-ih)/2:color=0x00000000,format=rgba', '-an', '-loop', '0', '-c:v', 'libwebp', '-quality', '75']);
  await rm(directory, { recursive: true, force: true });
  if (bytes.length === 0) throw new Error('sticker output is empty');
  if (bytes.length > STICKER_LIMIT) throw new Error('sticker output exceeds 1 MiB');
  return { bytes, mime: 'image/webp' };
}
// convertGif makes a short, silent H.264 mp4 at most 480 px wide, which every
// WhatsApp client plays as a GIF. still is true for an image or sticker.
export async function convertGif(input, still) {
  const { bytes, output, directory } = await convert(input, 'space-gif-', 'gif.mp4', (source) => [
    ...(still ? ['-loop', '1', '-t', String(GIF_STILL_SECONDS)] : []), '-i', source, '-t', String(GIF_SECONDS), '-an',
    '-vf', "fps=15,scale='trunc(min(480,iw)/2)*2':-2:flags=lanczos:out_range=tv,format=yuv420p",
    '-c:v', 'libx264', '-preset', 'veryfast', '-crf', '28', '-profile:v', 'baseline', '-pix_fmt', 'yuv420p', '-movflags', '+faststart']);
  try {
    if (bytes.length === 0 || bytes.length > GIF_LIMIT) throw new Error('gif output is empty or exceeds 4 MiB');
    const { stdout } = await execute('ffprobe', ['-v', 'error', '-show_entries', 'format=duration', '-of', 'csv=p=0', output], { timeout: 10_000 });
    const durationMs = Math.round(Number(stdout.trim()) * 1000);
    if (!Number.isFinite(durationMs) || durationMs <= 0 || durationMs > GIF_SECONDS * 1000 + 500) throw new Error('gif duration out of range');
    return { bytes, mime: 'video/mp4', durationMs };
  } finally { await rm(directory, { recursive: true, force: true }); }
}
async function keep(connection, result) {
  const assetId = createHash('sha256').update(result.bytes).digest('hex');
  await connection.store.write(`asset:${assetId}`, { bytes: result.bytes, mime: result.mime });
  return { assetId, mime: result.mime, size: result.bytes.length, ...(result.durationMs ? { durationMs: result.durationMs } : {}) };
}
// still is true for an image or a sticker, false for a video (a WhatsApp GIF
// is one); anything else has no media to convert.
function still(message) {
  const content = normalizeMessageContent(message.message) ?? {};
  if (!content.videoMessage && !content.imageMessage && !content.stickerMessage) throw Object.assign(new Error('not a GIF, video or image'), { status: 400 });
  return !content.videoMessage;
}
export async function makeSticker(connection, message, logger) {
  const image = still(message);
  return keep(connection, await convertSticker(await download(connection, message, logger), image));
}
export async function makeGif(connection, message, logger) {
  const image = still(message);
  return keep(connection, await convertGif(await download(connection, message, logger), image));
}
// outgoingContent is the Baileys message for a journaled send. A GIF is a
// video with gifPlayback; a sticker has no caption.
export function outgoingContent(payload, asset) {
  if (!payload.assetId) return { text: payload.text, mentions: payload.mentions };
  if (!asset || !['sticker', 'image', 'video', 'gif'].includes(payload.mediaType)) throw Object.assign(new Error('asset or media type unavailable'), { status: 400, attempted: false });
  if (payload.mediaType === 'sticker') return { sticker: asset.bytes };
  if (payload.mediaType === 'gif') {
    if (asset.mime !== 'video/mp4') throw Object.assign(new Error('GIF asset is not an mp4'), { status: 400, attempted: false });
    return { video: asset.bytes, gifPlayback: true, mimetype: 'video/mp4', caption: payload.text, mentions: payload.mentions };
  }
  return { [payload.mediaType]: asset.bytes, caption: payload.text, mentions: payload.mentions };
}
