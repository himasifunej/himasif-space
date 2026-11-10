import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, readFile, readdir, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { EncryptedStore } from '../src/store.mjs';
import { Connections } from '../src/connections.mjs';
const key = 'ab'.repeat(32), cid = '11111111-1111-4111-8111-111111111111';
const logger = { info(){}, error(){}, warn(){}, debug(){}, trace(){}, child(){ return this; } };
async function temporary(t) { const root=await mkdtemp(join(tmpdir(),'space-bridge-test-'));t.after(()=>rm(root,{recursive:true,force:true}));return root; }

test('encrypted state survives restart and authenticates namespace/content', async t => {
 const root=await temporary(t), store=await new EncryptedStore(root,key,cid).init();
 await store.write('credential',{secret:'never-plaintext',bytes:Buffer.from([1,2,3])});
 const raw=await readFile(store.location('credential'));
 assert.equal(raw.includes(Buffer.from('never-plaintext')),false);
 const reopened=await new EncryptedStore(root,key,cid).init();
 assert.deepEqual((await reopened.read('credential')).bytes,Buffer.from([1,2,3]));
 raw[raw.length-1]^=1;await writeFile(store.location('credential'),raw);
 await assert.rejects(reopened.read('credential'));
 assert.equal((await readdir(root)).filter(name=>name.endsWith('.tmp')).length,0);
});

test('job UUID deduplicates sends and a restart never retries an ambiguous job',async t=>{
 const root=await temporary(t), connections=new Connections(root,key,logger), c=await connections.get(cid);
 await connections.configure(cid,{allowedGroups:['12345@g.us']});
 c.status='connected';let attempts=0;
 c.socket={async sendMessage(jid,content,options){attempts++;return {key:{id:options.messageId}};},end(){}};
 const request={id:'22222222-2222-4222-8222-222222222222',groupJid:'12345@g.us',text:'fixture only'};
 const first=await connections.send(cid,request);await connections.send(cid,request);
 assert.equal(first.status,'accepted');assert.equal(attempts,1);
 await assert.rejects(connections.send(cid,{...request,text:'changed'}),error=>error.status===409);
 c.jobs[request.id].status='sending';await c.store.write('jobs',c.jobs);
 const restarted=new Connections(root,key,logger);const old=await restarted.job(cid,request.id);
 assert.equal(old.status,'unknown');assert.equal((await restarted.send(cid,request)).status,'unknown');assert.equal(attempts,1);
 await connections.close();await restarted.close();
});

test('unknown destination and disconnected jobs are rejected without attempting transport',async t=>{
 const connections=new Connections(await temporary(t),key,logger);
 const request={id:'33333333-3333-4333-8333-333333333333',groupJid:'12345@g.us',text:'fixture'};
 await assert.rejects(connections.send(cid,request),e=>e.status===403);
 await connections.configure(cid,{allowedGroups:['12345@g.us']});
 await assert.rejects(connections.send(cid,request),e=>e.status===503 && e.attempted===false);
 assert.equal(await connections.job(cid,request.id),null);
 await connections.close();
});

test('a direct message is forwarded only when it is a link code, and only its sender can get a reply',async t=>{
 const connections=new Connections(await temporary(t),key,logger), c=await connections.get(cid);
 const dm=(text,id)=>({key:{remoteJid:'6281234567890@s.whatsapp.net',id,fromMe:false},messageTimestamp:1762800000,message:{conversation:text}});
 assert.equal(await connections.direct(c,dm('halo bot, ada tugas apa?','A1')),false);
 assert.equal(c.events.items.length,0);
 assert.equal(await connections.direct(c,dm('SPACE-abcdefghijklmnopqrstuvwx','A2')),true);
 assert.equal(c.events.items.length,1);
 const event=c.events.items[0];
 assert.equal(event.groupJid,'');assert.equal(event.senderJid,'6281234567890@s.whatsapp.net');assert.equal(event.text,'SPACE-abcdefghijklmnopqrstuvwx');
 c.status='connected';const sent=[];
 c.socket={async sendMessage(jid,content,options){sent.push([jid,content]);return {key:{id:options.messageId}};},end(){}};
 const reply={id:'33333333-3333-4333-8333-333333333333',toJid:'6281234567890@s.whatsapp.net',text:'Tertaut.'};
 assert.equal((await connections.send(cid,reply)).status,'accepted');
 assert.deepEqual(sent,[['6281234567890@s.whatsapp.net',{text:'Tertaut.',mentions:[]}]]);
 await assert.rejects(connections.send(cid,{...reply,id:'44444444-4444-4444-8444-444444444444',toJid:'6289999999999@s.whatsapp.net'}),error=>error.status===403);
 await assert.rejects(connections.send(cid,{...reply,id:'55555555-5555-4555-8555-555555555555',mentions:['6281234567890@s.whatsapp.net']}),error=>error.status===400);
 c.dmPeers['6281234567890@s.whatsapp.net']=Date.now()-1;
 await assert.rejects(connections.send(cid,{...reply,id:'66666666-6666-4666-8666-666666666666'}),error=>error.status===403);
 await connections.close();
});
