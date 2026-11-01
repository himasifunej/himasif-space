import { createHash } from 'node:crypto';
import makeWASocket, { DisconnectReason, jidNormalizedUser, normalizeMessageContent } from '@whiskeysockets/baileys';
import QRCode from 'qrcode';
import { EncryptedStore, authState } from './store.mjs';
import { makeSticker } from './media.mjs';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const GROUP = /^\d+(?:-\d+)?@g\.us$/;
const USER = /^[0-9a-zA-Z._:-]+@(s\.whatsapp\.net|lid)$/;
const hash = (value) => createHash('sha256').update(JSON.stringify(value)).digest('hex');
const messageText = (message) => {
  const content = normalizeMessageContent(message) ?? {};
  return content.conversation ?? content.extendedTextMessage?.text ?? content.imageMessage?.caption ?? content.videoMessage?.caption ?? '';
};
export class Connections {
  constructor(root, key, logger) { this.root = root; this.key = key; this.logger = logger; this.connections = new Map(); this.loading = new Map(); }
  async get(id) {
    if (!UUID.test(id)) throw Object.assign(new Error('invalid connection ID'), { status: 400 });
    if (this.connections.has(id)) return this.connections.get(id);
    if (this.loading.has(id)) return this.loading.get(id);
    const loading = (async () => {
      const store = await new EncryptedStore(this.root, this.key, id).init();
      const config = await store.read('configuration', { enabled: false, allowedGroups: [] });
      const events = await store.read('events', { sequence: 0, items: [] });
      const jobs = await store.read('jobs', {});
      for (const job of Object.values(jobs)) if (job.status === 'sending') job.status = 'unknown';
      const c = { id, store, config, events, jobs, socket: null, generation: 0, status: 'disconnected', updatedAt: new Date().toISOString(), qr: '', qrExpiresAt: null, attempts: 0, reconnect: null, pending: Promise.resolve() };
      this.connections.set(id, c); return c;
    })();
    this.loading.set(id, loading);
    try { return await loading; } finally { this.loading.delete(id); }
  }
  status(c) { return { id: c.id, status: c.status, accountJid: c.socket?.user?.id ? jidNormalizedUser(c.socket.user.id) : '', qr: c.qr, qrExpiresAt: c.qrExpiresAt, updatedAt: c.updatedAt, lastError: c.lastError ?? '', reconnectAt: c.reconnectAt ?? null }; }
  async emit(c, event) {
    const task = async () => {
      
      c.events.items.push({ ...event, sequence: ++c.events.sequence, receivedAt: new Date().toISOString() });
      await c.store.write('events', c.events);
      if (c.events.items.length >= 10_000 && c.config.enabled) { c.config.enabled = false; c.status = 'backpressure'; c.lastError = 'Inbound queue is full; waiting for backend acknowledgement.'; c.socket?.end(new Error('inbound backpressure')); }
    };
    const pending = c.pending.then(task, task); c.pending = pending.catch(() => {}); return pending;
  }
  async configure(id, config) {
    const c = await this.get(id);
    if (!Array.isArray(config.allowedGroups) || config.allowedGroups.some((jid) => !GROUP.test(jid)) || config.allowedGroups.length > 20) throw Object.assign(new Error('invalid allowed groups'), { status: 400 });
    c.config.allowedGroups = [...new Set(config.allowedGroups)];
    await c.store.write('configuration', c.config);
    return this.status(c);
  }
  async connect(id) {
    const c = await this.get(id);
    if (c.socket && ['connected', 'connecting', 'pairing'].includes(c.status)) return this.status(c);
    if (c.reconnectAt && Date.now() < Date.parse(c.reconnectAt)) throw Object.assign(new Error('reconnect cooldown is active'), { status: 409 });
    c.config.enabled = true; await c.store.write('configuration', c.config);
    this.launch(c).catch(() => { c.status = 'failed'; c.lastError = 'Connection startup failed; inspect protected service logs.'; });
    return this.status(c);
  }
  async launch(c) {
    clearTimeout(c.reconnect); c.reconnect = null;
    const generation = ++c.generation;
    c.socket?.end(new Error('Replacing local socket'));
    c.socket = null;
    c.status = 'connecting'; c.qr = ''; c.updatedAt = new Date().toISOString();
    const auth = await authState(c.store);
    if (c.generation !== generation || !c.config.enabled) return;
    const socket = makeWASocket({ auth: auth.state, logger: this.logger, markOnlineOnConnect: false, syncFullHistory: false, printQRInTerminal: false, generateHighQualityLinkPreview: false,
      getMessage: async (key) => (await c.store.read(`message:${key.remoteJid}:${key.id}`, null))?.message
    });
    c.socket = socket;
    socket.ev.on('creds.update', (update) => {
      if (c.generation !== generation) return;
      auth.saveCreds(update).catch(() => { if (c.generation !== generation) return; c.config.enabled = false; c.status = 'storage_failed'; c.lastError = 'Auth state was not persisted.'; socket.end(new Error('auth persistence failed')); });
    });
    socket.ev.on('connection.update', (update) => {
      (async () => {
        if (c.generation !== generation) return;
        c.updatedAt = new Date().toISOString();
        if (update.qr) { c.qr = await QRCode.toDataURL(update.qr, { width: 280, margin: 2 }); c.qrExpiresAt = new Date(Date.now() + 60_000).toISOString(); c.status = 'pairing'; }
        if (update.connection === 'open') { c.status = 'connected'; c.qr = ''; c.qrExpiresAt = null; c.attempts = 0; c.reconnectAt = null; c.lastError = ''; }
        if (update.connection === 'close') {
          c.socket = null; c.qr = ''; c.qrExpiresAt = null;
          const code = update.lastDisconnect?.error?.output?.statusCode;
          if ([DisconnectReason.loggedOut, DisconnectReason.badSession, DisconnectReason.connectionReplaced, 403].includes(code)) { c.status = 'needs_pairing'; c.config.enabled = false; await c.store.write('configuration', c.config); }
          else if (c.config.enabled) {
            c.status = 'reconnecting'; const wait = Math.min(300_000, 30_000 * 2 ** Math.min(c.attempts++, 4)); c.reconnectAt = new Date(Date.now() + wait).toISOString();
            c.reconnect = setTimeout(() => this.launch(c).catch(() => { c.status = 'failed'; }), wait); c.reconnect.unref();
          } else c.status = 'disconnected';
        }
        await this.emit(c, { type: 'connection', status: c.status, accountJid: socket.user?.id ? jidNormalizedUser(socket.user.id) : '', occurredAt: c.updatedAt });
      })().catch(() => { c.lastError = 'Connection event could not be persisted.'; });
    });
    socket.ev.on('messages.upsert', ({ messages, type }) => {
      if (c.generation !== generation) return;
      (async () => {
        for (const message of messages) {
          if (c.generation !== generation) return;
          const group = message.key.remoteJid;
          if (!GROUP.test(group ?? '') || !c.config.allowedGroups.includes(group)) continue;
          await c.store.write(`message:${group}:${message.key.id}`, message);
          if (message.key.fromMe || type !== 'notify') continue;
          const body = normalizeMessageContent(message.message) ?? {};
          const protocol = body.protocolMessage;
          const kind = protocol?.editedMessage ? 'edit' : protocol?.type === 0 ? 'revoke' : 'message';
          const targetId = protocol?.key?.id ?? message.key.id;
          const text = messageText(protocol?.editedMessage ?? message.message).slice(0, 32_000);
          const sender = jidNormalizedUser(message.key.participant ?? message.participant ?? '');
          const senderAlt = message.key.participantAlt ? jidNormalizedUser(message.key.participantAlt) : '';
          const context = body.extendedTextMessage?.contextInfo ?? body.imageMessage?.contextInfo ?? body.videoMessage?.contextInfo ?? {};
          if (context.stanzaId && context.quotedMessage) await c.store.write(`message:${group}:${context.stanzaId}`, { key: { remoteJid: group, id: context.stanzaId, participant: context.participant }, message: context.quotedMessage });
          const media = body.imageMessage ?? body.videoMessage ?? body.stickerMessage;
          const stamp = Number(message.messageTimestamp?.toString?.() ?? message.messageTimestamp);
          const occurredAt = Number.isFinite(stamp) && stamp > 0 ? new Date(stamp * 1000).toISOString() : null;
          await this.emit(c, { type: kind, groupJid: group, senderJid: sender, senderAltJid: senderAlt, messageId: targetId, envelopeId: message.key.id, revision: hash([kind, text, protocol?.timestampMs?.toString?.() ?? '', targetId]), occurredAt, text,
            quotedMessageId: context.stanzaId ?? '', mentionedJids: context.mentionedJid ?? [], media: media ? { available: true, type: body.videoMessage ? 'video' : body.stickerMessage ? 'sticker' : 'image', mime: media.mimetype ?? '', size: Number(media.fileLength?.toString?.() ?? media.fileLength ?? 0) } : null });
        }
      })().catch(() => { c.status = 'storage_failed'; c.lastError = 'Inbound event was not persisted; pairing was not reset.'; });
    });
    socket.ev.on('messages.update', (updates) => {
      if (c.generation !== generation) return;
      (async () => { let changed = false;
        for (const { key, update } of updates) {
          if (!key.fromMe || update.status == null || update.status < 2) continue;
          const job = Object.values(c.jobs).find((value) => value.messageId === key.id);
          if (!job) continue;
          const next = update.status >= 4 ? 'read' : update.status >= 3 ? 'delivered' : 'accepted';
          const rank = { sending: 0, unknown: 0, accepted: 1, delivered: 2, read: 3 };
          if ((rank[next] ?? 0) > (rank[job.status] ?? 0)) job.status = next;
          const now = new Date().toISOString(); job.acceptedAt ??= now;
          if (next === 'delivered' || next === 'read') job.deliveredAt ??= now;
          if (next === 'read') job.readAt ??= now;
          job.updatedAt = now; changed = true;
        }
        if (changed) await c.store.write('jobs', c.jobs);
      })().catch(() => { c.lastError = 'Delivery receipt was not persisted.'; });
    });
  }
  async disconnect(id, logout = false) {
    const c = await this.get(id);
    if (logout && !c.socket) throw Object.assign(new Error('Connect before logging out; remote session removal is not confirmed while offline.'), { status: 409, attempted: false });
    c.config.enabled = false; ++c.generation;
    clearTimeout(c.reconnect); c.reconnect = null; c.reconnectAt = null;
    await c.store.write('configuration', c.config);
    if (logout && c.socket) await c.socket.logout(); else c.socket?.end(new Error('Disconnected by Space manager'));
    c.socket = null; c.status = logout ? 'needs_pairing' : 'disconnected'; c.qr = ''; c.qrExpiresAt = null;
    if (logout) { await c.store.remove('auth-creds'); await c.store.remove('signal-keys'); }
    return this.status(c);
  }
  async groups(id) {
    const c = await this.get(id); if (c.status !== 'connected' || !c.socket) throw Object.assign(new Error('not connected'), { status: 503 });
    const groups = await c.socket.groupFetchAllParticipating();
    return Object.values(groups).map(({ id, subject }) => ({ jid: id, name: subject }));
  }
  async events(id, after) { const c = await this.get(id); await c.pending; return c.events.items.filter((event) => event.sequence > after).slice(0, 100); }
  async acknowledge(id, through) { const c = await this.get(id); await c.pending; if (through > c.events.sequence) throw Object.assign(new Error('cursor exceeds available events'), { status: 400 }); c.events.items = c.events.items.filter((event) => event.sequence > through); await c.store.write('events', c.events); }
  async send(id, body) {
    const c = await this.get(id);
    if (!UUID.test(body.id ?? '') || !GROUP.test(body.groupJid ?? '') || !c.config.allowedGroups.includes(body.groupJid)) throw Object.assign(new Error('destination is not configured'), { status: 403 });
    const payload = { groupJid: body.groupJid, text: String(body.text ?? ''), mentions: [...new Set(body.mentions ?? [])], assetId: body.assetId ?? '', mediaType: body.mediaType ?? '' };
    if (payload.text.length > 30_000 || payload.mentions.length > 500 || payload.mentions.some((jid) => !USER.test(jid)) || payload.assetId && !/^[0-9a-f]{64}$/.test(payload.assetId)) throw Object.assign(new Error('invalid outgoing content'), { status: 400 });
    const fingerprint = hash(payload), previous = c.jobs[body.id];
    if (previous) { if (previous.fingerprint !== fingerprint) throw Object.assign(new Error('job ID reused for different content'), { status: 409 }); return previous; }
    if (!c.socket || c.status !== 'connected') throw Object.assign(new Error('not connected; send not attempted'), { status: 503, attempted: false });
    let content = { text: payload.text, mentions: payload.mentions };
    if (payload.assetId) {
      const asset = await c.store.read(`asset:${payload.assetId}`, null);
      if (!asset || !['sticker', 'image', 'video'].includes(payload.mediaType)) throw Object.assign(new Error('asset or media type unavailable'), { status: 400, attempted: false });
      content = { [payload.mediaType]: asset.bytes, ...(payload.mediaType === 'sticker' ? {} : { caption: payload.text, mentions: payload.mentions }) };
    }
    const claimed = c.jobs[body.id];
    if (claimed) { if (claimed.fingerprint !== fingerprint) throw Object.assign(new Error('job ID reused for different content'), { status: 409 }); return claimed; }
    const job = { id: body.id, fingerprint, messageId: '3EB0' + createHash('sha256').update(`${id}:${body.id}`).digest('hex').slice(0, 20).toUpperCase(), status: 'sending', updatedAt: new Date().toISOString() };
    c.jobs[body.id] = job; await c.store.write('jobs', c.jobs);
    try {
      const result = await c.socket.sendMessage(payload.groupJid, content, { messageId: job.messageId });
      job.messageId = result.key.id;
      if (!['accepted', 'delivered', 'read'].includes(job.status)) job.status = 'accepted';
      job.updatedAt = new Date().toISOString(); job.acceptedAt ??= job.updatedAt;
      await c.store.write('jobs', c.jobs);
    } catch { if (!['accepted', 'delivered', 'read'].includes(job.status)) job.status = 'unknown'; job.updatedAt = new Date().toISOString(); await c.store.write('jobs', c.jobs); }
    return job;
  }
  async job(id, jobId) { const c = await this.get(id); return c.jobs[jobId] ?? null; }
  async sticker(id, group, messageId) {
    const c = await this.get(id);
    if (!c.config.allowedGroups.includes(group) || c.status !== 'connected') throw Object.assign(new Error('group or connection unavailable'), { status: 403 });
    const message = await c.store.read(`message:${group}:${messageId}`, null); if (!message) throw Object.assign(new Error('media message unavailable'), { status: 404 });
    return makeSticker(c, message, this.logger);
  }
  async restore(ids) { for (const id of ids) { const c = await this.get(id); if (c.config.enabled) this.launch(c).catch(() => { c.status = 'failed'; }); } }
  async close() { for (const c of this.connections.values()) { ++c.generation; clearTimeout(c.reconnect); c.socket?.end(new Error('Service shutdown')); await c.pending; await c.store.pending; } }
}
