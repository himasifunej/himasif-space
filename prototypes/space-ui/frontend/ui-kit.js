// Shared fixture and visual helpers for UI exploration. No backend calls.
const paths = {
  home:'<path d="m3 10 9-7 9 7v10a1 1 0 0 1-1 1h-5v-7H9v7H4a1 1 0 0 1-1-1Z"/>',
  task:'<rect x="4" y="4" width="16" height="17" rx="3"/><path d="M9 3h6v4H9zM8 12l2 2 5-5M8 18h8"/>',
  grid:'<rect x="3" y="3" width="7" height="7" rx="2"/><rect x="14" y="3" width="7" height="7" rx="2"/><rect x="3" y="14" width="7" height="7" rx="2"/><rect x="14" y="14" width="7" height="7" rx="2"/>',
  note:'<path d="M14 3H5a1 1 0 0 0-1 1v16a1 1 0 0 0 1 1h14a1 1 0 0 0 1-1V9Z"/><path d="M14 3v6h6M8 13h8M8 17h5"/>',
  calendar:'<rect x="3" y="5" width="18" height="16" rx="3"/><path d="M16 3v4M8 3v4M3 11h18m-12 4h2m3 0h2m-7 3h2"/>',
  bell:'<path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9M10 21h4"/>',
  search:'<circle cx="10.5" cy="10.5" r="6.5"/><path d="m16 16 5 5"/>',
  plus:'<path d="M12 5v14M5 12h14"/>',
  chevron:'<path d="m9 5 7 7-7 7"/>',
  down:'<path d="m6 9 6 6 6-6"/>',
  arrow:'<path d="M4 12h16m-6-6 6 6-6 6"/>',
  users:'<circle cx="9" cy="8" r="3"/><path d="M3 21v-3a6 6 0 0 1 12 0v3m1-16a3 3 0 0 1 0 6m3 10v-3a6 6 0 0 0-3-5"/>',
  clock:'<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
  settings:'<path d="m9 3-1 3-3 1 1 3-2 2 2 2-1 3 3 1 1 3h6l1-3 3-1-1-3 2-2-2-2 1-3-3-1-1-3Z"/><circle cx="12" cy="12" r="3"/>',
  link:'<path d="m10 13 4-4m-5 7-2 2a4 4 0 0 1-6-6l5-5a4 4 0 0 1 6 0m0 10a4 4 0 0 0 6 0l5-5a4 4 0 0 0-6-6l-2 2" transform="translate(1 0) scale(.9)"/>',
  check:'<path d="m5 12 4 4L19 6"/>',
  more:'<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>',
  send:'<path d="m22 2-7 20-4-9L2 9Z M11 13 22 2"/>',
  lock:'<rect x="5" y="10" width="14" height="11" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3m-4 5v2"/>',
  archive:'<rect x="3" y="3" width="18" height="5" rx="1"/><path d="M5 8v12h14V8m-10 4h6"/>',
  board:'<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M9 4v16M15 4v16M5 8h2m4 0h2m4 0h2M5 12h2m4 0h2"/>',
  chat:'<path d="M21 12a8 8 0 0 1-8 8H4l-2 2V10a8 8 0 0 1 8-8h3a8 8 0 0 1 8 8Z"/><path d="M7 8h9M7 12h7"/>',
  spark:'<path d="m12 3 2.5 6.5L21 12l-6.5 2.5L12 21l-2.5-6.5L3 12l6.5-2.5Z"/>',
  filter:'<path d="M4 7h16M7 12h10M10 17h4"/><circle cx="8" cy="7" r="2" fill="currentColor" stroke="none"/><circle cx="15" cy="12" r="2" fill="currentColor" stroke="none"/>',
  globe:'<circle cx="12" cy="12" r="9"/><ellipse cx="12" cy="12" rx="4" ry="9"/><path d="M3 12h18"/>',
  logout:'<path d="M10 4H4v16h6m5-12 4 4-4 4m-7-4h11"/>',
  folder:'<path d="M3 6a2 2 0 0 1 2-2h5l2 3h7a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z"/>',
};
const icon=(n,cl='')=>`<svg class="icon ${cl}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${paths[n]||paths.grid}</svg>`;
const esc=s=>String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const mark=()=>'<img class="space-mark" src="./space.svg" alt="">';
const people={AJ:{name:'Ahimsa Jenar',short:'Ahimsa',color:'sand'},ZM:{name:'Zakiya Mirza',short:'Zakiya',color:'sage'},OA:{name:'Oktavia',short:'Oktavia',color:'rose'},AR:{name:'Akbar Rizky',short:'Akbar',color:'blue'},IM:{name:'Ilham Maulana',short:'Ilham',color:'lavender'},NY:{name:'Naura Yesha',short:'Naura',color:'sage'},RF:{name:'Raffy Putra',short:'Raffy',color:'blue'}};
const avatar=(id='AJ',size='')=>`<span class="avatar ${people[id]?.color||'sand'} ${size}" title="${people[id]?.name||id}">${id}</span>`;
const avatars=(ids=['AJ','ZM','OA','AR'])=>`<span class="avatar-stack">${ids.map(id=>avatar(id,'small')).join('')}<span class="avatar-more">+4</span></span>`;
const tasks=[
 {id:1,title:'Lesson Learned — Oktober',desc:'Desain carousel rangkuman pembelajaran bulan Oktober. Konsep dan caption sudah tersedia dari Humas.',category:'Desain',space:'Medhum',pj:'AJ',humas:'NY',date:'28 Okt',posting:'30 Oktober 2026',status:'Dikerjakan',color:'sand',label:'Besok',progress:65},
 {id:2,title:'Mingper 11 — carousel',desc:'Siapkan carousel Minggu Pertama edisi 11. Gunakan arahan warna dark blue sesuai konsep Humas.',category:'Desain',space:'Medhum',pj:'ZM',humas:'NY',date:'29 Okt',posting:'31 Oktober 2026',status:'Belum mulai',color:'blue',label:'2 hari lagi',progress:0},
 {id:3,title:'Pembaruan profil himasif.id',desc:'Perbarui informasi pengurus dan tautan program kerja pada halaman profil.',category:'Tekno',space:'Mediatek',pj:'OA',date:'30 Okt',status:'Dikerjakan',color:'sage',label:'3 hari lagi',progress:40},
 {id:4,title:'Video recap Hi-Cussion',desc:'Rangkai dokumentasi kegiatan menjadi video singkat untuk arsip dan publikasi.',category:'Video',space:'Mediatek',pj:'AR',date:'31 Okt',status:'Belum mulai',color:'rose',label:'4 hari lagi',progress:0},
 {id:5,title:'Sempro — Nadya',desc:'Tautan hasil desain telah dikirim ke grup Medhum. Pastikan kiriman sesuai dengan tugas ini.',category:'Desain',space:'Medhum',pj:'OA',humas:'IM',date:'27 Okt',posting:'29 Oktober 2026',status:'Perlu tinjauan',color:'sand',label:'Hari ini',progress:100},
 {id:6,title:'Template feed November',desc:'Siapkan template feed bulan November berdasarkan arahan visual yang disepakati.',category:'Desain',space:'Mediatek',pj:'AJ',date:'2 Nov',status:'Selesai',color:'sand',label:'Selesai',progress:100},
];
const pill=(label,color='')=>`<span class="pill ${color}">${esc(label)}</span>`;
const statusPill=t=>pill(t.status,t.status==='Selesai'?'sage':t.status==='Perlu tinjauan'?'rose':t.status==='Dikerjakan'?'sand':'muted');
const btn=(text,action,kind='secondary',ico='')=>`<button class="btn ${kind}" data-action="${action}">${ico?icon(ico):''}${text}</button>`;
const mini=(name,action,label='')=>`<button class="icon-button" data-action="${action}" aria-label="${label||name}" title="${label||name}">${icon(name)}</button>`;
function aceArt(){return `<svg viewBox="0 0 84 88" class="ace-art" aria-label="Ace"><ellipse cx="43" cy="81" rx="24" ry="4" fill="#000" opacity=".2"/><path d="M21 29 15 9q19-2 23 15M56 26Q61 7 73 12l-5 24" fill="#b8a586"/><path d="M16 44q0-25 26-25t26 25v23q-1 11-10 9l-8-4-9 5-10-5-9 4q-8 0-7-10Z" fill="#e0d0ae"/><path d="M19 43q4-20 23-20 17 0 23 16" fill="none" stroke="#eee3ca" stroke-width="2"/><ellipse cx="31" cy="47" rx="3" ry="5" fill="#3b382f"/><ellipse cx="52" cy="47" rx="3" ry="5" fill="#3b382f"/><path d="m38 57 4 3 4-3" fill="none" stroke="#766750" stroke-width="2" stroke-linecap="round"/><ellipse cx="25" cy="55" rx="5" ry="2.5" fill="#cb9b83" opacity=".55"/><ellipse cx="58" cy="55" rx="5" ry="2.5" fill="#cb9b83" opacity=".55"/><path d="m42 29 2 4 5 .5-4 3 1 5-4-2.5-4 2.5 1-5-4-3 5-.5Z" fill="#9b8968"/></svg>`;}
export { icon, esc, mark, people, avatar, avatars, tasks, pill, statusPill, btn, mini, aceArt };
