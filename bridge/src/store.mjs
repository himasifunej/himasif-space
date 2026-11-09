import { createCipheriv, createDecipheriv, createHash, randomBytes } from 'node:crypto';
import { mkdir, open, readFile, rename, rm, lstat } from 'node:fs/promises';
import { join } from 'node:path';
import { BufferJSON, initAuthCreds, proto } from '@whiskeysockets/baileys';

export class EncryptedStore {
  constructor(root, key, namespace) {
    if (!/^[0-9a-f]{64}$/i.test(key ?? '')) throw new Error('SPACE_WA_STATE_KEY must be 32 bytes in hex');
    this.root = root;
    this.key = Buffer.from(key, 'hex');
    this.namespace = namespace;
    this.pending = Promise.resolve();
  }
  async init() {
    await mkdir(this.root, { recursive: true, mode: 0o700 });
    const info = await lstat(this.root);
    if (!info.isDirectory() || info.isSymbolicLink()) throw new Error('state path must be a real directory');
    return this;
  }
  location(key) { return join(this.root, createHash('sha256').update(`${this.namespace}:${key}`).digest('hex') + '.enc'); }
  async read(key, fallback) {
    await this.pending;
    let data;
    try { data = await readFile(this.location(key)); } catch (error) { if (error.code === 'ENOENT') return fallback; throw error; }
    if (data.length < 29 || data[0] !== 1) throw new Error('invalid encrypted state');
    const decipher = createDecipheriv('aes-256-gcm', this.key, data.subarray(1, 13));
    decipher.setAAD(Buffer.from(`${this.namespace}:${key}`));
    decipher.setAuthTag(data.subarray(13, 29));
    const plain = Buffer.concat([decipher.update(data.subarray(29)), decipher.final()]);
    return JSON.parse(plain.toString('utf8'), BufferJSON.reviver);
  }
  write(key, value) {
    const plain = Buffer.from(JSON.stringify(value, BufferJSON.replacer));
    const iv = randomBytes(12);
    const cipher = createCipheriv('aes-256-gcm', this.key, iv);
    cipher.setAAD(Buffer.from(`${this.namespace}:${key}`));
    const encrypted = Buffer.concat([cipher.update(plain), cipher.final()]);
    const encoded = Buffer.concat([Buffer.from([1]), iv, cipher.getAuthTag(), encrypted]);
    const run = async () => {
      const target = this.location(key), temp = `${target}.${randomBytes(8).toString('hex')}.tmp`;
      try {
        const file = await open(temp, 'wx', 0o600);
        try { await file.writeFile(encoded); await file.sync(); } finally { await file.close(); }
        await rename(temp, target);
        const directory = await open(this.root, 'r');
        try { await directory.sync(); } finally { await directory.close(); }
      } finally { await rm(temp, { force: true }); }
    };
    const task = this.pending.then(run, run);
    this.pending = task.catch(() => {});
    return task;
  }
  async remove(key) { await this.pending; await rm(this.location(key), { force: true }); }
}

export async function authState(store) {
  const creds = await store.read('auth-creds', null) ?? initAuthCreds();
  const keys = await store.read('signal-keys', {});
  return {
    state: {
      creds,
      keys: {
        async get(type, ids) {
          const out = {};
          for (const id of ids) {
            let value = keys[type]?.[id];
            if (type === 'app-state-sync-key' && value) value = proto.Message.AppStateSyncKeyData.fromObject(value);
            out[id] = value;
          }
          return out;
        },
        async set(changes) {
          for (const [type, values] of Object.entries(changes)) {
            keys[type] ??= {};
            for (const [id, value] of Object.entries(values)) {
              if (value == null) delete keys[type][id]; else keys[type][id] = value;
            }
          }
          await store.write('signal-keys', keys);
        }
      }
    },
    async saveCreds(update) { Object.assign(creds, update); await store.write('auth-creds', creds); }
  };
}
