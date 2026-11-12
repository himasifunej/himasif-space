import test from 'node:test';
import assert from 'node:assert/strict';
import { execFile } from 'node:child_process';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { promisify } from 'node:util';
import { Connections } from '../src/connections.mjs';
import { GIF_LIMIT, GIF_SECONDS, STICKER_LIMIT, convertGif, convertSticker, outgoingContent } from '../src/media.mjs';
const execute = promisify(execFile);
const key = 'ab'.repeat(32), cid = '11111111-1111-4111-8111-111111111111';
const logger = { info(){}, error(){}, warn(){}, debug(){}, trace(){}, child(){ return this; } };
async function temporary(t) { const root=await mkdtemp(join(tmpdir(),'space-bridge-test-'));t.after(()=>rm(root,{recursive:true,force:true}));return root; }
// fixture renders test media with ffmpeg's own generators, as WhatsApp would hand it over.
async function fixture(t, name, args) { const path=join(await temporary(t),name);await execute('ffmpeg',['-hide_banner','-loglevel','error','-y',...args,path]);return readFile(path); }
async function probe(t, bytes, name) {
 const path=join(await temporary(t),name);await writeFile(path,bytes);
 const { stdout }=await execute('ffprobe',['-v','error','-show_entries','stream=codec_type,codec_name,width,height,pix_fmt:format=duration','-of','json',path]);
 return JSON.parse(stdout);
}

test('a long video with sound becomes a silent mp4 GIF of at most ten seconds',async t=>{
 const input=await fixture(t,'clip.mp4',['-f','lavfi','-i','testsrc=size=1280x720:rate=30','-f','lavfi','-i','sine','-t','15','-c:v','libx264','-c:a','aac']);
 const gif=await convertGif(input,false);
 assert.equal(gif.mime,'video/mp4');
 assert.ok(gif.bytes.length>0 && gif.bytes.length<=GIF_LIMIT);
 assert.ok(gif.durationMs>9_000 && gif.durationMs<=GIF_SECONDS*1000+500,`duration ${gif.durationMs}`);
 const info=await probe(t,gif.bytes,'gif.mp4');
 assert.deepEqual(info.streams.map(s=>s.codec_type),['video']);
 const [video]=info.streams;
 assert.equal(video.codec_name,'h264');assert.equal(video.pix_fmt,'yuv420p');
 assert.equal(video.width,480);assert.equal(video.height%2,0);
});

test('an image becomes a short still GIF; a small one keeps its size, rounded to even',async t=>{
 const input=await fixture(t,'photo.jpg',['-f','lavfi','-i','testsrc=size=301x201','-frames:v','1']);
 const gif=await convertGif(input,true);
 assert.ok(gif.durationMs>=2_500 && gif.durationMs<=3_500,`duration ${gif.durationMs}`);
 const [video]=(await probe(t,gif.bytes,'gif.mp4')).streams;
 assert.equal(video.width,300);assert.equal(video.height,200);assert.equal(video.pix_fmt,'yuv420p');
});

test('a WhatsApp GIF (an mp4 already) converts again within the limits',async t=>{
 const input=await fixture(t,'gif.mp4',['-f','lavfi','-i','testsrc=size=320x240:rate=10','-t','2','-c:v','libx264','-pix_fmt','yuv420p']);
 const gif=await convertGif(input,false);
 assert.ok(gif.durationMs>=1_500 && gif.durationMs<=2_500,`duration ${gif.durationMs}`);
});

test('media ffmpeg cannot read is refused, not stored',async()=>{
 await assert.rejects(convertGif(Buffer.from('bukan video'),false));
 await assert.rejects(convertSticker(Buffer.from('bukan gambar'),true));
});

test('a sticker from an image is a 512×512 WebP of at most 1 MiB',async t=>{
 const input=await fixture(t,'photo.png',['-f','lavfi','-i','testsrc=size=800x400','-frames:v','1']);
 const sticker=await convertSticker(input,true);
 assert.equal(sticker.mime,'image/webp');assert.ok(sticker.bytes.length<=STICKER_LIMIT);
 assert.equal(sticker.bytes.subarray(0,4).toString(),'RIFF');assert.equal(sticker.bytes.subarray(8,12).toString(),'WEBP');
 const [image]=(await probe(t,sticker.bytes,'sticker.webp')).streams;
 assert.equal(image.width,512);assert.equal(image.height,512);
});

test('a sticker from a video is an animated 512×512 WebP of at most 1 MiB',async t=>{
 const input=await fixture(t,'clip.mp4',['-f','lavfi','-i','testsrc=size=640x360:rate=30','-t','10','-c:v','libx264']);
 const sticker=await convertSticker(input,false);
 assert.ok(sticker.bytes.length>0 && sticker.bytes.length<=STICKER_LIMIT);
 assert.ok(sticker.bytes.includes(Buffer.from('ANIM')),'not animated');
});

test('a GIF is sent as a gif-playback video with its caption; a sticker without one',()=>{
 const payload={text:'damn',mentions:['6281234567890@s.whatsapp.net'],assetId:'a'.repeat(64)};
 const bytes=Buffer.from([1,2,3]);
 assert.deepEqual(outgoingContent({...payload,mediaType:'gif'},{bytes,mime:'video/mp4'}),{video:bytes,gifPlayback:true,mimetype:'video/mp4',caption:'damn',mentions:payload.mentions});
 assert.deepEqual(outgoingContent({...payload,mediaType:'sticker'},{bytes,mime:'image/webp'}),{sticker:bytes});
 assert.deepEqual(outgoingContent({...payload,mediaType:'image'},{bytes,mime:'image/jpeg'}),{image:bytes,caption:'damn',mentions:payload.mentions});
 assert.deepEqual(outgoingContent({text:'halo',mentions:[],assetId:''},null),{text:'halo',mentions:[]});
 assert.throws(()=>outgoingContent({...payload,mediaType:'gif'},{bytes,mime:'image/webp'}),e=>e.status===400 && e.attempted===false);
 assert.throws(()=>outgoingContent({...payload,mediaType:'document'},{bytes,mime:'video/mp4'}),e=>e.status===400);
 assert.throws(()=>outgoingContent({...payload,mediaType:'gif'},null),e=>e.status===400);
});

test('a journaled GIF send reaches WhatsApp once as gifPlayback',async t=>{
 const connections=new Connections(await temporary(t),key,logger), c=await connections.get(cid);
 await connections.configure(cid,{allowedGroups:['12345@g.us']});
 const bytes=Buffer.from('mp4 fixture'), assetId='b'.repeat(64);
 await c.store.write(`asset:${assetId}`,{bytes,mime:'video/mp4'});
 c.status='connected';const sent=[];
 c.socket={async sendMessage(jid,content,options){sent.push([jid,content]);return {key:{id:options.messageId}};},end(){}};
 const request={id:'77777777-7777-4777-8777-777777777777',groupJid:'12345@g.us',text:'',assetId,mediaType:'gif'};
 assert.equal((await connections.send(cid,request)).status,'accepted');
 await connections.send(cid,request);
 assert.equal(sent.length,1);
 assert.equal(sent[0][0],'12345@g.us');assert.equal(sent[0][1].gifPlayback,true);assert.equal(sent[0][1].mimetype,'video/mp4');
 assert.deepEqual(Buffer.from(sent[0][1].video),bytes);
 // A sticker asset cannot be sent as a GIF; nothing is journaled or attempted.
 const sticker='c'.repeat(64);await c.store.write(`asset:${sticker}`,{bytes,mime:'image/webp'});
 await assert.rejects(connections.send(cid,{...request,id:'88888888-8888-4888-8888-888888888888',assetId:sticker}),e=>e.status===400 && e.attempted===false);
 assert.equal(await connections.job(cid,'88888888-8888-4888-8888-888888888888'),null);
 assert.equal(sent.length,1);
 await connections.close();
});

test('GIF conversion reads only a stored message of an allowed group, and only media',async t=>{
 const connections=new Connections(await temporary(t),key,logger), c=await connections.get(cid);
 await connections.configure(cid,{allowedGroups:['12345@g.us']});
 c.status='connected';
 await assert.rejects(connections.gif(cid,'99999@g.us','M1'),e=>e.status===403);
 await assert.rejects(connections.gif(cid,'12345@g.us','M1'),e=>e.status===404);
 await c.store.write('message:12345@g.us:M2',{key:{remoteJid:'12345@g.us',id:'M2'},message:{conversation:'!setgif damn'}});
 await assert.rejects(connections.gif(cid,'12345@g.us','M2'),e=>e.status===400);
 await connections.close();
});
