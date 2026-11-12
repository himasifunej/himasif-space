import { createServer } from 'node:http';
import { timingSafeEqual } from 'node:crypto';
import { resolve } from 'node:path';
import pino from 'pino';
import { Connections } from './connections.mjs';
import { EncryptedStore } from './store.mjs';

const token = process.env.SPACE_WA_BRIDGE_TOKEN ?? '';
if (token.length < 32) throw new Error('SPACE_WA_BRIDGE_TOKEN must contain at least 32 characters');
const key = process.env.SPACE_WA_STATE_KEY ?? '';
const root = resolve(process.env.SPACE_WA_STATE_DIR ?? './state');
const registry = await new EncryptedStore(root, key, 'registry').init();
const ids = new Set(await registry.read('connection-ids', []));
const logger = pino({ level: 'silent', redact: ['creds', 'keys', 'qr', 'msg', 'message', 'auth', 'headers.authorization'] });
const connections = new Connections(root, key, logger);
await connections.restore(ids);
const authorize = (request) => { const actual = Buffer.from(request.headers.authorization ?? ''), expected = Buffer.from(`Bearer ${token}`); return actual.length === expected.length && timingSafeEqual(actual, expected); };
const readBody = async (request) => { const chunks = []; let size = 0; for await (const chunk of request) { size += chunk.length; if (size > 1024 * 1024) throw Object.assign(new Error('body too large'), { status: 413 }); chunks.push(chunk); } return size ? JSON.parse(Buffer.concat(chunks).toString('utf8')) : {}; };
const reply = (response, status, data) => { response.writeHead(status, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' }); response.end(JSON.stringify(data)); };
const server = createServer(async (request, response) => {
  if (!authorize(request)) return reply(response, 401, { error: 'unauthorized' });
  try {
    const url = new URL(request.url, 'http://bridge.internal');
    if (request.method === 'GET' && url.pathname === '/health') return reply(response, 200, { status: 'ready', connections: connections.connections.size });
    const match = /^\/connections\/([a-f0-9-]+)(?:\/(connect|disconnect|logout|groups|events|ack|send|jobs|sticker|gif))?(?:\/([a-f0-9-]+))?$/.exec(url.pathname);
    if (!match) return reply(response, 404, { error: 'not found' });
    const [, id, operation, jobId] = match;
    const c = await connections.get(id);
    if (!ids.has(id)) { ids.add(id); await registry.write('connection-ids', [...ids]); }
    const body = ['POST', 'PUT'].includes(request.method) ? await readBody(request) : {};
    let result;
    if (request.method === 'GET' && !operation) result = connections.status(c);
    else if (request.method === 'PUT' && !operation) result = await connections.configure(id, body);
    else if (request.method === 'POST' && operation === 'connect') result = await connections.connect(id);
    else if (request.method === 'POST' && operation === 'disconnect') result = await connections.disconnect(id);
    else if (request.method === 'POST' && operation === 'logout') result = await connections.disconnect(id, true);
    else if (request.method === 'GET' && operation === 'groups') result = await connections.groups(id);
    else if (request.method === 'GET' && operation === 'events') { const after = Number(url.searchParams.get('after') ?? 0); if (!Number.isSafeInteger(after) || after < 0) throw Object.assign(new Error('invalid cursor'), { status: 400 }); result = await connections.events(id, after); }
    else if (request.method === 'POST' && operation === 'ack') { if (!Number.isSafeInteger(body.through) || body.through < 0) throw Object.assign(new Error('invalid cursor'), { status: 400 }); await connections.acknowledge(id, body.through); result = { status: 'acknowledged' }; }
    else if (request.method === 'POST' && operation === 'send') result = await connections.send(id, body);
    else if (request.method === 'GET' && operation === 'jobs' && jobId) result = await connections.job(id, jobId);
    else if (request.method === 'POST' && operation === 'sticker') result = await connections.sticker(id, body.groupJid, body.messageId);
    else if (request.method === 'POST' && operation === 'gif') result = await connections.gif(id, body.groupJid, body.messageId);
    else return reply(response, 405, { error: 'method not allowed' });
    reply(response, 200, result);
  } catch (error) { reply(response, error.status ?? 500, { error: error.status ? error.message : 'bridge operation failed', attempted: error.attempted }); }
});
server.requestTimeout = 30_000;
server.headersTimeout = 10_000;
server.listen(Number(process.env.SPACE_WA_BRIDGE_PORT ?? 8789), process.env.SPACE_WA_BRIDGE_LISTEN ?? '127.0.0.1');
for (const signal of ['SIGINT', 'SIGTERM']) process.once(signal, () => { server.close(); connections.close().finally(() => process.exit(0)); });
