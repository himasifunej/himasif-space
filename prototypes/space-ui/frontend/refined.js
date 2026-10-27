// Draft 03: sidebar A, header/board B, inline detail C, calendar E.
// In-memory UI exploration; no backend or production mutations.
import { icon, esc, mark, people, avatar, avatars, tasks, pill, aceArt } from './ui-kit.js';

const query = new URLSearchParams(location.search);
const columns = ['Belum mulai', 'Dikerjakan', 'Perlu tinjauan', 'Selesai'];
const accents = { sand: '#d7c3a0', sage: '#a8c2ac', blue: '#a2bbd2', lavender: '#b6a8d0', rose: '#d2a499' };
const ui = {
  view: ['home','board','calendar','notes','personal','reminders','members','settings'].includes(query.get('view')) ? query.get('view') : 'home',
  collapsed: query.get('sidebar') === 'collapsed' || (!query.has('sidebar') && innerWidth < 800),
  division: query.get('division')==='Humas'?'Humas':'Mediatek',
  role: ['45','member'].includes(query.get('role'))?query.get('role'):'kadiv',
  settingsTab: ['general','whatsapp','reminders','commands','activity'].includes(query.get('settings'))?query.get('settings'):'whatsapp',
  logos: {Mediatek:'./assets/mediatek.png',Humas:'./assets/humas.png'},
  botDirty:false,
  space: query.get('space')==='Medhum'?'Medhum':query.get('division')==='Humas'?'Humas':'Mediatek', filter: 'Semua', search: '',
  selected: Number(query.get('task')) || null,
  panel: query.get('panel') === 'appearance' ? 'appearance' : query.has('task') ? 'task' : null,
  theme: ['dark','light','system'].includes(query.get('theme')) ? query.get('theme') : 'dark',
  accent: accents[query.get('accent')] || accents.sand,
  density: query.get('density') === 'compact' ? 'compact' : 'comfortable',
  panelWidth: 420, online: navigator.onLine, ace: true, motion: true,
  tasks: tasks.map(t => ({...t, hasEvidence: t.id === 5})),
  comments: [], dragging: null, before: null, suppressClickUntil: 0,
  lastChange: 'Belum ada perubahan', privateShare: false,
};
function makeBot(scope){return {connected:true,name:'Ace · '+scope,accountLabel:'HIMASIF '+scope,accountType:'dedicated',style:'Santai',time:'06:00',groups:[scope+' 2025 / 2026',...(scope==='Humas'?['Medhum 2025 / 2026']:[])],h2:true,h1:true,h0:true,overdue:true,mention:true,digest:true,weekly:true,retried:false,commands:[{name:'!damn',description:'Stiker reaksi pilihan divisi',type:'Stiker',enabled:true},{name:'!semangat',description:'Semangat, teman-teman. Satu tugas lagi, kita bisa!',type:'Teks',enabled:true}]};}
ui.bots={Mediatek:makeBot('Mediatek'),Humas:makeBot('Humas')};
const $ = s => document.querySelector(s);
const iPaths = {
  whatsapp:'<path d="M21 11.5a9 9 0 0 1-13.4 7.9L3 21l1.6-4.6A9 9 0 1 1 21 11.5Z"/><path d="M8 7.5c.7 3.6 2.9 5.8 6.5 6.5l1-2-2-.9-.8 1c-1.4-.6-2.2-1.4-2.8-2.8l1-.8L10 6.5Z"/>',
  info:'<circle cx="12" cy="12" r="9"/><path d="M12 11v6m0-10v.1"/>',
  alert:'<path d="m10.3 4-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.7-3l-8-14a2 2 0 0 0-3.4 0Z"/><path d="M12 9v4m0 4v.1"/>',
  message:'<path d="M21 11.5a8.5 8.5 0 0 1-8.5 8.5H4l-2 2V11.5A8.5 8.5 0 0 1 10.5 3h2a8.5 8.5 0 0 1 8.5 8.5Z"/><path d="M7 9h10M7 13h6"/>',
  qr:'<path d="M3 3h6v6H3zm12 0h6v6h-6zM3 15h6v6H3zm12 0h2v2h-2zm4 4h2v2h-2zM12 3v3m0 5v3m-9-2h4m5 6v3m6-9h3m-3 3h3m-6 3v3"/>',
  refresh:'<path d="M20 7v5h-5M4 17v-5h5M6.1 6a7 7 0 0 1 11.5 1L20 12M4 12l2.4 5A7 7 0 0 0 18 18"/>',
  shield:'<path d="m12 3 8 3v6c0 5-8 9-8 9s-8-4-8-9V6Z"/><path d="m8 12 3 3 5-6"/>',
  terminal:'<rect x="3" y="4" width="18" height="16" rx="3"/><path d="m7 9 3 3-3 3m6 0h4"/>',
  upload:'<path d="M12 16V3m-5 5 5-5 5 5M4 16v4h16v-4"/>',
  sticker:'<path d="M4 3h16v11l-7 7H4Z"/><path d="M13 21v-7h7M8 8h.1m6 0h.1m-6 4c2 2 4 2 6 0"/>',
  edit:'<path d="m15 4 5 5M4 20l4-1L21 6l-5-5L3 14Z"/>',
  unlink:'<path d="m3 3 18 18M8 11l-3 3a4 4 0 0 0 6 6l3-3m2-4 3-3a4 4 0 0 0-6-6l-3 3M9 15l6-6"/>',
  panel: '<rect x="3" y="4" width="18" height="16" rx="3"/><path d="M9 4v16m-3-9 2 1-2 1"/>',
  palette: '<path d="M12 3a9 9 0 1 0 0 18h1a2 2 0 0 0 2-2c0-1-1-1.5-1-2.5s1-1.5 2-1.5h2a3 3 0 0 0 3-3 9 9 0 0 0-9-9Z"/><circle cx="7" cy="10" r=".7"/><circle cx="10" cy="7" r=".7"/><circle cx="15" cy="7" r=".7"/><circle cx="18" cy="10" r=".7"/>',
  sun: '<circle cx="12" cy="12" r="4"/><path d="M12 2v2m0 16v2M2 12h2m16 0h2M5 5l1.5 1.5m11 11L19 19M5 19l1.5-1.5m11-11L19 5"/>',
  moon: '<path d="M20.5 13A8.5 8.5 0 0 1 11 3.5 8.5 8.5 0 1 0 20.5 13Z"/>',
  monitor: '<rect x="3" y="4" width="18" height="13" rx="2"/><path d="M8 21h8m-4-4v4"/>',
  grip: '<circle cx="9" cy="5" r=".8"/><circle cx="15" cy="5" r=".8"/><circle cx="9" cy="12" r=".8"/><circle cx="15" cy="12" r=".8"/><circle cx="9" cy="19" r=".8"/><circle cx="15" cy="19" r=".8"/>',
  close: '<path d="m6 6 12 12M6 18 18 6"/>',
  wifi: '<path d="M2 8a16 16 0 0 1 20 0M5 12a11 11 0 0 1 14 0m-11 4a6 6 0 0 1 8 0"/><circle cx="12" cy="20" r=".8"/>',
  keyboard: '<rect x="2" y="5" width="20" height="14" rx="2"/><path d="M6 9h1m3 0h1m3 0h1m3 0h1M6 13h1m3 0h1m3 0h1m3 0h1M7 16h10"/>',
};
const glyph = n => iPaths[n] ? `<svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${iPaths[n]}</svg>` : icon(n);
const action = (label, name, type='ghost', symbol='') => `<button class="ui-button ${type}" data-ui-action="${name}">${symbol?glyph(symbol):''}${label}</button>`;
const ib = (symbol, name, label) => `<button class="ui-icon-button" data-ui-action="${name}" aria-label="${label}" title="${label}">${glyph(symbol)}</button>`;
const list = () => ui.tasks.filter(t => (ui.space !== 'Medhum' || t.space === 'Medhum') && (ui.view !== 'personal' || t.pj === 'AJ') && (ui.filter === 'Semua' || t.category === ui.filter));
const activeTask = () => ui.tasks.find(t => t.id === ui.selected);
const statusColor = s => ({'Dikerjakan':'sand','Perlu tinjauan':'rose','Selesai':'sage'}[s] || 'muted');
const badge = t => pill(t.status, statusColor(t.status));

function syncURL() {
  const url = new URL(location.href);
  url.searchParams.set('view', ui.view);
  if(ui.division==='Humas')url.searchParams.set('division','Humas');
  if(ui.space==='Medhum')url.searchParams.set('space','Medhum');else url.searchParams.delete('space');
  if(ui.view==='settings')url.searchParams.set('settings',ui.settingsTab);else url.searchParams.delete('settings');
  url.searchParams.set('theme', ui.theme);
  url.searchParams.set('sidebar', ui.collapsed ? 'collapsed' : 'expanded');
  if(ui.selected && ui.panel === 'task') url.searchParams.set('task', ui.selected); else url.searchParams.delete('task');
  if(ui.panel === 'appearance') url.searchParams.set('panel', 'appearance'); else url.searchParams.delete('panel');
  const accentName = Object.entries(accents).find(([,v]) => v === ui.accent)?.[0];
  if(accentName) url.searchParams.set('accent', accentName); else url.searchParams.delete('accent');
  try { history.replaceState(null, '', url); } catch {} // File and sandbox previews may restrict history.
}
function applyAppearance() {
  const root = document.documentElement;
  const theme = ui.theme === 'system' ? (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light') : ui.theme;
  root.dataset.uiTheme = theme;
  root.dataset.uiDensity = ui.density;
  root.dataset.uiMotion = ui.motion ? 'on' : 'off';
  root.style.setProperty('--accent', ui.accent);
  const rgb=ui.accent.replace('#','').match(/.{2}/g)?.map(v=>parseInt(v,16))||[215,195,160];
  const lightness=(rgb[0]*299+rgb[1]*587+rgb[2]*114)/1000;
  root.style.setProperty('--ui-accent-ink',lightness>145?'#24251f':'#ffffff');
  root.style.setProperty('--ui-panel-width', `${ui.panelWidth}px`);
}
function sidebar() {
  const own=ui.division,who=currentUser();
  return `<aside class="ui-sidebar ${ui.collapsed?'is-collapsed':''}"><div class="ui-brand-row"><button class="ui-brand" data-ui-view="home" aria-label="Space — beranda">${mark()}<span>space<span class="ui-brand-dot">.</span><small>RUANG KERJA HIMASIF</small></span></button></div><button class="ui-period" data-ui-action="period" aria-label="Pilih periode kepengurusan">${glyph('archive')}<span>2025 / 2026<small>Periode aktif</small></span>${glyph('down')}</button><div class="ui-sidebar-nav"><div class="ui-nav-label">PRIBADI</div><button class="ui-nav ${ui.view==='personal'?'active':''}" data-ui-view="personal" title="Jobdesk saya">${glyph('task')}<span>Jobdesk saya</span><b>${ui.tasks.filter(t=>t.pj===who).length}</b></button><button class="ui-nav ${ui.view==='reminders'?'active':''}" data-ui-view="reminders" title="Reminder pribadi">${glyph('bell')}<span>Reminder pribadi</span></button><div class="ui-nav-label ui-space-label">SPACE SAYA ${ib('plus','new-space','Tambah Space')}</div><button class="ui-space ${ui.space===own?'selected':''}" data-ui-space="${own}" title="Space ${own}">${spaceMark(own)}<span>${own}<small>${own==='Mediatek'?'Media dan Teknologi':'Hubungan Masyarakat'}</small></span><i class="live-dot"></i></button><div class="ui-space-links">${[['home','home','Beranda'],['board','board','Board tugas'],['calendar','calendar','Agenda'],['notes','note','Notes']].map(([v,s,l])=>`<button class="${ui.view===v&&ui.space===own?'active':''}" data-ui-view="${v}" data-ui-own-space="${own}" title="${l}">${glyph(s)}<span>${l}</span></button>`).join('')}</div><button class="ui-space ${ui.space==='Medhum'?'selected':''}" data-ui-space="Medhum" title="Space bersama Medhum">${spaceMark('Medhum')}<span>Medhum<small>Humas × Mediatek</small></span><b>3</b></button><button class="ui-nav ui-archive" data-ui-action="archive" title="Arsip periode">${glyph('archive')}<span>Arsip periode</span></button></div><div class="ui-sidebar-bottom">${canManage()?`<button class="ui-nav ${ui.view==='settings'?'active':''}" data-ui-view="settings" title="Pengaturan Space dan WhatsApp">${glyph('settings')}<span>Pengaturan Space</span></button>`:''}<button class="ui-nav ${ui.panel==='appearance'?'active':''}" data-ui-action="appearance" title="Penampilan · Ctrl ,">${glyph('palette')}<span>Penampilan</span><i class="ui-accent-dot"></i></button><button class="ui-nav ui-collapse-control" data-ui-action="sidebar" aria-expanded="${!ui.collapsed}" title="${ui.collapsed?'Buka':'Ciutkan'} sidebar · Ctrl B">${glyph('panel')}<span>${ui.collapsed?'Buka':'Ciutkan'} sidebar</span><kbd>Ctrl B</kbd></button><button class="ui-user" data-ui-action="profile" title="${people[who].name}">${avatar(who)}<span>${people[who].name}<small>${ui.role==='45'?'Role 45':ui.role==='member'?'Anggota':'Kadiv'} ${own}</small></span>${glyph('more')}</button></div></aside>`;
}

function header() {
  return `<header class="ui-header"><div class="ui-header-identity"><div><button class="ui-space-title" data-ui-action="space-menu" aria-haspopup="menu">${ui.space}${glyph('down')}</button><span>HIMASIF <i>·</i> 2025 / 2026</span></div></div><div class="ui-header-spacer"></div><div class="ui-search-wrap"><label class="ui-search" for="ui-search">${glyph('search')}<input id="ui-search" aria-label="Cari tugas, notes, atau anggota" value="${esc(ui.search)}" placeholder="Cari tugas, notes, atau anggota…" autocomplete="off"><kbd>Ctrl K</kbd></label><div id="ui-search-results" class="ui-search-results" hidden></div></div><div class="ui-header-tools">${avatars(['AJ','ZM','OA'])}${ib('bell','notifications','Notifikasi')}${action('Buat tugas','new-task','primary','plus')}</div></header>`;
}

function tabs() {
  return `<nav class="ui-workspace-tabs" aria-label="Tampilan Space">${[['home','home','Beranda'],['board','board','Board tugas'],['calendar','calendar','Agenda'],['notes','note','Notes'],['members','users','Anggota'],...(canManage()?[['settings','settings','Pengaturan']]:[])].map(([v,s,l])=>`<button class="${ui.view===v?'active':''}" data-ui-view="${v}" ${ui.view===v?'aria-current="page"':''}>${glyph(s)}${l}${v==='board'?`<span>${list().length}</span>`:''}</button>`).join('')}<span class="grow"></span><span class="ui-view-hint">${ui.view==='board'?'Seret kartu untuk memindahkan tugas':ui.view==='calendar'?'Deadline desain dan posting terpisah':ui.view==='settings'?'Koneksi dan preferensi divisi':'Ruang kita, tetap terhubung'}</span></nav>`;
}
function statusbar() {
  return `<footer class="ui-statusbar"><button class="ui-connection ${ui.online?'':'is-offline'}" data-ui-action="online" title="Klik untuk mencoba keadaan offline"><i></i>${ui.online?'Online':'Tidak ada koneksi'}</button><span class="ui-status-divider"></span><span class="ui-status-context">${glyph('grid')}${ui.space}<span>·</span>${list().length} tugas</span><span class="ui-status-divider ui-status-long"></span><span class="ui-status-sync">${glyph(ui.online?'check':'clock')}${ui.online?'Tersimpan di preview':'Di perangkat · belum tersinkron'}</span><span class="grow"></span><span class="ui-draft-label">Draft 03 · data contoh</span><button data-ui-action="focus-search" title="Fokus pencarian"><kbd>Ctrl K</kbd>Cari</button><button data-ui-action="sidebar" title="Buka/tutup sidebar"><kbd>Ctrl B</kbd>Sidebar</button><button class="ui-shortcut-help" data-ui-action="shortcuts" title="Semua shortcut">${glyph('keyboard')}</button><button class="ui-status-theme" data-ui-action="appearance" title="Penampilan">${glyph(ui.theme==='light'?'sun':ui.theme==='system'?'monitor':'moon')}</button></footer>`;
}
function taskRows(data=list()) {
  return `<div class="ui-task-list">${data.map(t=>`<button class="ui-task-row ${ui.selected===t.id&&ui.panel==='task'?'selected':''}" data-ui-task="${t.id}"><span class="ui-task-check ${t.status==='Selesai'?'checked':''}">${t.status==='Selesai'?glyph('check'):''}</span><span class="ui-task-row-main"><strong>${esc(t.title)}</strong><small>${pill(t.category,t.color)}${t.space==='Medhum'?'Medhum · ':''}${people[t.pj].short}</small></span>${badge(t)}<span class="ui-task-date">${glyph('calendar')}${t.date}</span>${avatar(t.pj,'small')}${glyph('chevron')}</button>`).join('')||'<p class="ui-empty">Belum ada tugas pada filter ini.</p>'}</div>`;
}
function section(title, more='') {
  return `<div class="ui-section-heading"><h2>${title}</h2>${more?`<button data-ui-view="${more}">Lihat semua${glyph('arrow')}</button>`:''}</div>`;
}
function filters() {
  return `<div class="ui-filter-bar"><div class="ui-filter-options">${['Semua','Desain','Tekno','Video'].map(f=>`<button class="${ui.filter===f?'active':''}" data-ui-filter="${f}">${f}</button>`).join('')}</div><span class="grow"></span><span class="ui-filter-count">${list().length} tugas</span>${ib('filter','filter','Filter pekerjaan')}${ib('more','board-options','Opsi board')}</div>`;
}
function agenda() {
  return `<section class="ui-agenda">${section('Agenda terdekat','calendar')}<button class="ui-agenda-meeting" data-ui-action="meeting"><span class="ui-date-box"><b>27</b><small>OKT</small></span><span><strong>Rapat divisi Mediatek</strong><small>${glyph('clock')}19.00 – 20.30 WIB</small></span>${glyph('chevron')}</button><p>Evaluasi Oktober dan persiapan konten November.</p><div class="ui-agenda-bottom">${avatars(['AJ','ZM','OA'])}${action('Ping peserta','ping-meeting','outline','send')}</div><div class="ui-agenda-followup"><span></span><div><strong>Follow-up Medhum</strong><small>Besok · 09.00 WIB</small></div>${glyph('chevron')}</div></section>`;
}
function aceCard() {
  if(!ui.ace) return '';
  return `<section class="ui-ace-card"><span class="ui-eyebrow">SEDIKIT PENGINGAT</span><div><div><h3>Satu langkah lagi, Him.</h3><p>Ada kiriman desain yang menunggu tinjauanmu.</p><button data-ui-task="5">Lihat kiriman${glyph('arrow')}</button></div>${aceArt()}</div><footer><i class="live-dot"></i>Ace menemanimu<span class="grow"></span>${ib('more','ace','Pengaturan Ace')}</footer></section>`;
}
function notesCards() {
  return `<section class="ui-home-notes">${section('Catatan bersama','notes')}<div>${[['note','Rapat divisi — Oktober','Evaluasi jobdesk, arah visual, dan rencana bulan depan.','AJ'],['calendar','Jadwal konten November','Kalender posting dan pembagian PJ Medhum.','NY']].map(([s,title,desc,who])=>`<button class="ui-note-card" data-ui-view="notes">${glyph(s)}<strong>${title}</strong><p>${desc}</p><small>${avatar(who,'tiny')}${people[who].short} · 2 jam lalu</small></button>`).join('')}</div></section>`;
}
function home() {
  const data=list();const counts=[data.filter(t=>t.status!=='Selesai').length,data.filter(t=>t.status==='Dikerjakan').length,data.filter(t=>t.status==='Perlu tinjauan').length,data.filter(t=>t.status==='Selesai').length];
  return `<div class="ui-home"><div class="ui-page-heading"><div><span class="ui-eyebrow">SELASA, 27 OKTOBER 2026</span><h1>Ruang ${ui.space} kita<span>.</span></h1><p>Biar setiap ide punya tempat, dan setiap tugas tetap dalam alurnya.</p></div><div class="ui-view-actions">${action('Buka board','view-board','outline','board')}${action('Lihat agenda','view-calendar','ghost','calendar')}</div></div><div class="ui-stat-strip">${[['task','Tugas aktif',counts[0],'di divisi'],['clock','Dikerjakan',counts[1],'sedang berjalan'],['send','Perlu tinjauan',counts[2],'kiriman desain'],['check','Selesai',counts[3],'pekerjaan tuntas']].map(([s,l,n,d])=>`<div><span>${glyph(s)}${l}</span><strong>${n}<small>${d}</small></strong></div>`).join('')}</div><div class="ui-home-grid"><div class="ui-home-primary"><div class="ui-review-banner"><span>${glyph('send')}</span><div><strong>Ada kiriman yang perlu kamu cocokkan</strong><p>Oktavia mengirim hasil desain Sempro — Nadya.</p></div><button data-ui-task="5">Tinjau kiriman${glyph('arrow')}</button></div><section>${section('Jobdesk divisi','board')}${filters()}${taskRows()}</section>${notesCards()}</div><aside class="ui-home-aside">${agenda()}${aceCard()}<section class="ui-activity">${section('Aktivitas terbaru')}<div>${avatar('OA','small')}<p><strong>Oktavia</strong> mengirim hasil desain<br><b>Sempro — Nadya</b><small>Perlu pencocokan · 24 menit lalu</small></p></div><div>${avatar('NY','small')}<p><strong>Naura</strong> membagikan konsep<br><b>Lesson Learned</b><small>Medhum · 1 jam lalu</small></p></div></section></aside></div></div>`;
}

function boardCard(t) {
  return `<article class="ui-kanban-card ${ui.panel==='task'&&ui.selected===t.id?'is-selected':''}" draggable="true" data-ui-card="${t.id}" tabindex="0" role="button" aria-label="${esc(t.title)}, ${t.status}. Enter membuka detail, menu untuk memindahkan.">
    <div class="ui-card-top"><span class="ui-drag-handle" title="Seret untuk memindahkan">${glyph('grip')}</span>${pill(t.category,t.color)}<span class="grow"></span><button class="ui-card-more" data-ui-move-menu="${t.id}" aria-label="Pindahkan ${esc(t.title)}" title="Pindahkan tugas">${glyph('more')}</button></div>
    <h3>${esc(t.title)}</h3><p>${esc(t.desc)}</p>
    ${t.status==='Dikerjakan'?`<div class="ui-card-progress"><span style="width:${t.progress}%"></span></div>`:''}
    ${t.status==='Perlu tinjauan'?`<div class="ui-card-evidence">${glyph(t.hasEvidence?'link':'send')}${t.hasEvidence?'Tautan Drive diterima':'Menunggu tinjauan'}</div>`:''}
    <div class="ui-card-meta"><span>${glyph('calendar')}${t.date}</span>${avatar(t.pj,'small')}</div>
    <footer>${glyph(t.space==='Medhum'?'link':'grid')}${t.space}${t.posting?`<span>Posting ${t.posting.split(' ').slice(0,2).join(' ')}</span>`:''}</footer>
  </article>`;
}
function board() {
  return `<div class="ui-board-view"><div class="ui-page-heading ui-board-heading"><div><span class="ui-eyebrow">${ui.space.toUpperCase()} · OKTOBER</span><h1>Board tugas<span>.</span></h1><p>Lihat alurnya. Bagi pekerjaannya. Selesaikan bersama.</p></div><div class="ui-board-people">${avatars()}<span>8 anggota</span></div></div>${filters()}<div class="ui-kanban" aria-label="Board tugas yang dapat dipindahkan">${columns.map((status,index)=>`<section class="ui-kanban-column" data-ui-column="${status}"><header><span class="ui-column-dot column-${index}"></span><h2>${status}</h2><b>${list().filter(t=>t.status===status).length}</b><span class="grow"></span><button class="ui-icon-button" data-ui-new-column="${status}" aria-label="Tambah tugas ${status}">${glyph('plus')}</button></header><div class="ui-column-cards" data-ui-drop="${status}">${list().filter(t=>t.status===status).map(boardCard).join('')}<div class="ui-empty-drop">Lepaskan tugas di sini</div></div><button class="ui-add-card" data-ui-new-column="${status}">${glyph('plus')}Tambah tugas</button></section>`).join('')}</div><div class="ui-board-tip">${glyph('grip')}Seret kartu untuk mengubah urutan atau status.<span>Atau gunakan menu ${glyph('more')} pada kartu.</span><span class="grow"></span>${ui.panel==='task'?'<span>Esc untuk menutup detail</span>':'<span>Klik tugas untuk membuka detail di kanan</span>'}</div></div>`;
}
function calendar() {
  const days=[['Sen','26'],['Sel','27'],['Rab','28'],['Kam','29'],['Jum','30'],['Sab','31'],['Min','1']];
  return `<div class="ui-calendar-view"><div class="ui-page-heading"><div><span class="ui-eyebrow">AGENDA BERSAMA</span><h1>Selaraskan minggu kita<span>.</span></h1><p>Ruang untuk deadline, rapat, dan semua yang perlu diingat.</p></div>${action('Tambah agenda','new-task','outline','plus')}</div><div class="ui-calendar-toolbar"><div><h2>26 Oktober – 1 November <span>2026</span></h2><small>Minggu ke-44 · WIB</small></div><div class="ui-calendar-legend"><span><i class="sand"></i>Desain</span><span><i class="sage"></i>Tekno & rapat</span><span><i class="rose"></i>Video</span></div><span class="grow"></span>${action('Hari ini','today','outline')}${ib('chevron','prev-week','Minggu sebelumnya')}${ib('chevron','next-week','Minggu berikutnya')}</div>
    <div class="ui-calendar-scroll"><div class="ui-week-calendar"><div class="ui-calendar-time"><header>WIB</header>${['06.00','09.00','12.00','15.00','18.00','21.00'].map(h=>`<span>${h}</span>`).join('')}</div>${days.map(([name,date],i)=>`<div class="ui-calendar-day ${i===1?'today':''}"><header><span>${name}</span><strong>${date}</strong></header><div class="ui-calendar-day-body">${i===0?'<div class="ui-calendar-empty-note">Siapkan konsep<br>November</div>':''}${i===1?`<button class="ui-cal-event sand" style="--slot:16px" data-ui-task="5"><small>DEADLINE DESAIN</small><strong>Sempro — Nadya</strong><span>Perlu tinjauan</span></button><button class="ui-cal-event sage" style="--slot:356px" data-ui-action="meeting"><small>19.00 – 20.30</small><strong>Rapat divisi</strong><span>8 peserta · Online</span></button>`:''}${i===2?`<button class="ui-cal-event sand" style="--slot:16px" data-ui-task="1"><small>DEADLINE DESAIN</small><strong>Lesson Learned</strong>${avatar('AJ','tiny')}</button><button class="ui-cal-event muted" style="--slot:104px" data-ui-action="meeting"><small>09.00 WIB</small><strong>Follow-up Medhum</strong></button>`:''}${i===3?`<button class="ui-cal-event blue" style="--slot:16px" data-ui-task="2"><small>DEADLINE DESAIN</small><strong>Mingper 11</strong>${avatar('ZM','tiny')}</button>`:''}${i===4?`<button class="ui-cal-event sage" style="--slot:190px" data-ui-task="3"><small>TEKNO</small><strong>Update profil web</strong>${avatar('OA','tiny')}</button><button class="ui-cal-event muted" style="--slot:356px" data-ui-task="1"><small>JADWAL POSTING</small><strong>Lesson Learned</strong></button>`:''}${i===5?`<button class="ui-cal-event rose" style="--slot:16px" data-ui-task="4"><small>VIDEO</small><strong>Recap Hi-Cussion</strong>${avatar('AR','tiny')}</button>`:''}</div></div>`).join('')}</div></div><div class="ui-calendar-footnote">${glyph('calendar')}Jadwal posting dan deadline desain ditampilkan terpisah. Klik kegiatan untuk melihat detail.</div></div>`;
}
function detailPanel() {
  const t=activeTask();if(!t)return '';
  return `<div class="ui-inspector-header"><span>${glyph('task')}Detail tugas</span><span class="grow"></span>${ib('chevron','previous-task','Tugas sebelumnya')}${ib('chevron','next-task','Tugas berikutnya')}${ib('close','close-panel','Tutup detail · Esc')}</div><div class="ui-task-detail"><div class="ui-detail-context">${glyph(t.space==='Medhum'?'link':'grid')}${t.space}<span>/</span>${t.category}<span class="grow"></span>${ib('more','task-options','Opsi tugas')}</div><h2>${esc(t.title)}</h2><div class="ui-detail-status"><label><span class="ui-column-dot column-${columns.indexOf(t.status)}"></span>${combo('ui-task-status',t.status,columns,'Status tugas')}</label><span>Diperbarui di preview</span></div><dl class="ui-detail-properties"><dt>Penanggung jawab</dt><dd>${avatar(t.pj,'tiny')}<span>${people[t.pj].name}</span>${pill('PJ utama','muted')}</dd><dt>${t.category==='Desain'?'Deadline desain':'Tenggat tugas'}</dt><dd>${glyph('calendar')}<strong>${t.date} 2026</strong>${pill(t.label,t.label==='Hari ini'?'rose':'sand')}</dd>${t.posting?`<dt>Jadwal posting</dt><dd>${glyph('globe')}${t.posting}</dd><dt>PJ Humas</dt><dd>${avatar(t.humas||'NY','tiny')}${people[t.humas||'NY'].name}</dd>`:''}<dt>Space</dt><dd>${glyph('link')}${t.space==='Medhum'?'Medhum · Humas × Mediatek':'Mediatek'}</dd></dl><section class="ui-detail-description"><h3>Hasil yang diharapkan</h3><p>${esc(t.desc)}</p><div class="ui-deliverable">${glyph('check')}${t.category==='Desain'?'File desain siap digunakan':t.category==='Video'?'Video hasil dokumentasi':'Perubahan web sesuai kebutuhan'}</div><div class="ui-deliverable">${glyph('check')}Tautan hasil atau dokumentasi pengerjaan</div></section>${t.space==='Medhum'?`<button class="ui-detail-attachment" data-ui-action="attachment"><span>${glyph('link')}</span><div><strong>${t.hasEvidence?'Hasil desain — Sempro Nadya':'Konsep & caption konten'}</strong><small>${t.hasEvidence?'Google Drive · Dikirim Oktavia':'Trello · Dibagikan Naura'}</small></div>${glyph('arrow')}</button>`:''}<div class="ui-detail-buttons">${t.status==='Perlu tinjauan'&&t.hasEvidence?action('Konfirmasi sesuai tugas','confirm-task','primary','check'):action('Ping PJ','ping','primary','send')}${action('Tawarkan bantuan','help','outline','users')}</div><section class="ui-detail-discussion"><h3>Diskusi<span>${1+ui.comments.filter(c=>c.task===t.id).length}</span></h3><div class="ui-comment">${avatar(t.humas||'AJ','small')}<div><strong>${people[t.humas||'AJ'].short}<small>10.55 WIB</small></strong><p>${t.space==='Medhum'?'Konsep sudah siap ya. Kalau ada yang perlu ditanyakan, kabari di sini!':'Kalau ada kendala atau butuh bantuan, tulis di sini ya.'}</p></div></div>${ui.comments.filter(c=>c.task===t.id).map(c=>`<div class="ui-comment">${avatar('AJ','small')}<div><strong>Ahimsa<small>Baru saja</small></strong><p>${esc(c.text)}</p></div></div>`).join('')}<div class="ui-comment-input">${avatar('AJ','small')}<input id="ui-comment" placeholder="Tulis komentar…" aria-label="Komentar tugas">${ib('send','comment','Kirim komentar')}</div></section><div class="ui-detail-source">${glyph('clock')}Setiap perubahan terlihat juga di jobdesk divisi dan PJ.</div></div>`;
}
function appearancePanel() {
  return `<div class="ui-inspector-header"><span>${glyph('palette')}Penampilan</span><span class="grow"></span>${ib('close','close-panel','Tutup penampilan · Esc')}</div><div class="ui-appearance"><span class="ui-eyebrow">RUANGMU, CARAMU</span><h2>Buat terasa milikmu<span>.</span></h2><p>Atur tampilan tanpa mengubah alur kerja Space.</p><section><h3>Tema</h3><div class="ui-theme-options">${[['dark','moon','Gelap'],['light','sun','Terang'],['system','monitor','Sistem']].map(([value,symbol,label])=>`<button class="${ui.theme===value?'active':''}" data-ui-theme="${value}" aria-pressed="${ui.theme===value}"><span class="ui-theme-preview ${value}"><i></i><i></i><i></i></span><span>${glyph(symbol)}${label}</span>${ui.theme===value?glyph('check'):''}</button>`).join('')}</div></section><section><div class="ui-setting-title"><h3>Warna aksen</h3><span>${ui.accent.toUpperCase()}</span></div><p class="ui-setting-help">Sentuhan warna untuk tombol, pilihan, dan detail kecil.</p><div class="ui-accent-options">${Object.entries(accents).map(([name,value])=>`<button style="--swatch:${value}" class="${ui.accent===value?'selected':''}" data-ui-accent="${value}" title="${name}" aria-label="Aksen ${name}">${ui.accent===value?glyph('check'):''}</button>`).join('')}<button class="ui-custom-color" data-ui-action="custom-color" aria-label="Warna aksen kustom">${glyph('plus')}</button></div></section><section><h3>Kepadatan tampilan</h3><div class="ui-setting-segment">${[['comfortable','Nyaman'],['compact','Ringkas']].map(([v,l])=>`<button class="${ui.density===v?'active':''}" data-ui-density="${v}">${l}</button>`).join('')}</div><p class="ui-setting-help">Ringkas menampilkan lebih banyak tugas dalam satu layar.</p></section><section><div class="ui-toggle-row"><div><h3>Sidebar ringkas</h3><p>Jadikan navigasi rail ikon.</p></div><button class="ui-toggle ${ui.collapsed?'on':''}" data-ui-action="sidebar" aria-label="Sidebar ringkas" aria-pressed="${ui.collapsed}"></button></div><label class="ui-panel-width-label" for="ui-panel-width">Lebar panel detail<span>${ui.panelWidth} px</span></label><input id="ui-panel-width" type="range" min="340" max="560" step="10" value="${ui.panelWidth}"></section><section><div class="ui-toggle-row"><div><h3>Animasi halus</h3><p>Transisi sidebar dan panel.</p></div><button class="ui-toggle ${ui.motion?'on':''}" data-ui-action="motion" aria-label="Animasi halus" aria-pressed="${ui.motion}"></button></div><div class="ui-toggle-row"><div><h3>Tampilkan Ace</h3><p>Teman kecil di ruang kerjamu.</p></div><button class="ui-toggle ${ui.ace?'on':''}" data-ui-action="ace" aria-label="Tampilkan Ace" aria-pressed="${ui.ace}"></button></div></section><button class="ui-reset-appearance" data-ui-action="reset-appearance">Kembalikan tampilan awal</button><div class="ui-appearance-note">Preferensi pada draft ini tersimpan selama preview. Beranda, board, dan agenda mengikuti tema yang sama.</div></div>`;
}

function notesView() {
  return `<div class="ui-notes-view"><div class="ui-page-heading"><div><span class="ui-eyebrow">CATATAN BERSAMA</span><h1>Ruang untuk mencatat<span>.</span></h1><p>Dari pembahasan menjadi keputusan, dari keputusan menjadi tindakan.</p></div>${action('Catatan baru','new-note','outline','plus')}</div><div class="ui-note-templates">${[['note','Notulensi rapat','Agenda, pembahasan, keputusan, tindak lanjut'],['calendar','Tabel jadwal','Kegiatan, tanggal, waktu, dan PJ'],['plus','Catatan bebas','Mulai dari ruang kosong']].map(([s,t,d])=>`<button data-ui-action="new-note">${glyph(s)}<strong>${t}</strong><span>${d}</span></button>`).join('')}</div><article class="ui-note-document"><span class="ui-eyebrow">NOTULENSI · ${ui.space.toUpperCase()}</span><h2 contenteditable="true" aria-label="Judul notulensi">Rapat divisi — Oktober</h2><div class="ui-note-meta">${avatar('AJ','tiny')}Ahimsa Jenar<span>·</span>27 Oktober 2026${pill('Pencatat','sand')}</div><h3>Agenda</h3><p contenteditable="true">Evaluasi jobdesk Oktober, persiapan konten November, dan arah visual divisi.</p><h3>Keputusan bersama</h3><ul contenteditable="true"><li>Konsep dan caption disiapkan sebelum pengerjaan desain.</li><li>Gunakan satu template visual yang konsisten untuk November.</li><li>Hasil pekerjaan dikirim melalui folder Drive yang sesuai.</li></ul><h3>Tindak lanjut</h3><div class="ui-note-followup">${glyph('task')}Siapkan panduan visual November${action('Jadikan tugas','new-task','outline','plus')}</div><div class="ui-note-followup">${glyph('calendar')}Susun kalender konten bersama Humas${action('Jadikan tugas','new-task','outline','plus')}</div></article></div>`;
}
function otherView() {
  if(ui.view==='personal')return `<div class="ui-personal-view"><div class="ui-page-heading"><div><span class="ui-eyebrow">PRIBADI</span><h1>Jobdesk saya<span>.</span></h1><p>Tugas dari ruang yang berbeda, dalam satu pandangan.</p></div></div>${taskRows(list())}</div>`;
  if(ui.view==='members')return `<div class="ui-personal-view"><div class="ui-page-heading"><div><span class="ui-eyebrow">ORANG-ORANG DI RUANG KITA</span><h1>Anggota ${ui.space}<span>.</span></h1><p>Desain, Tekno, dan Video. Tetap bergerak bersama.</p></div></div><div class="ui-member-grid">${Object.entries(people).filter(([id])=>!['NY','RF'].includes(id)).map(([id,p],i)=>`<article>${avatar(id)}<div><strong>${p.name}</strong><small>${i===0?'Kadiv Mediatek':i===2?'Pendamping Kadiv · Tekno':'Anggota · '+(i===1?'Desain':'Video')}</small></div></article>`).join('')}</div></div>`;
  return `<div class="ui-personal-view"><div class="ui-page-heading"><div><span class="ui-eyebrow">HANYA UNTUKMU</span><h1>Reminder pribadi<span>.</span></h1><p>Privat secara default. Kamu yang memilih siapa yang boleh melihat.</p></div>${action('Buat reminder','personal-reminder','outline','plus')}</div><article class="ui-personal-reminder">${glyph('bell')}<div><strong>Cek referensi desain November</strong><p>Hari ini · 20.30 WIB</p></div>${pill(ui.privateShare?'Dibagikan ke Raffy':'Privat',ui.privateShare?'sage':'muted')}</article><div class="ui-private-setting"><div><h3>Bagikan ke kadiv</h3><p>Akses diberikan kepada akun yang dipilih, bukan diwariskan ke penggantinya.</p></div><button class="ui-toggle ${ui.privateShare?'on':''}" data-ui-action="private-share" aria-label="Bagikan reminder pribadi" aria-pressed="${ui.privateShare}"></button></div></div>`;
}
function workspace() {
  return ui.view==='home'?home():ui.view==='board'?board():ui.view==='calendar'?calendar():ui.view==='notes'?notesView():ui.view==='settings'?settingsView():otherView();
}
function render() {
  closePopover();
  const oldMain=$('#ui-main-scroll'),oldBoard=$('.ui-kanban'),oldPanel=$('.ui-inspector');
  const scrollTop=oldMain?.scrollTop||0,scrollLeft=oldBoard?.scrollLeft||0,panelScroll=oldPanel?.scrollTop||0;
  applyAppearance();syncURL();
  document.title=`Space · ${ui.view==='home'?'Beranda':ui.view==='board'?'Board tugas':ui.view==='calendar'?'Agenda':ui.view==='notes'?'Notes':ui.view==='settings'?'Pengaturan Space':'Ruang kerja'} — Draft 03`;
  $('#refined-app').innerHTML=`<div class="ui-app ${ui.collapsed?'sidebar-collapsed':''}">${sidebar()}<div class="ui-workspace">${header()}${tabs()}<div class="ui-body ${ui.panel?'has-panel':''}" data-inspector="${ui.panel||'none'}"><main id="ui-main-scroll" class="ui-main-scroll">${!ui.online?'<div class="ui-offline-banner">Tidak ada koneksi · Tersimpan di perangkat, belum tersinkron ke cloud</div>':''}${workspace()}</main><aside class="ui-inspector ${ui.panel?'is-open':''}" aria-label="${ui.panel==='appearance'?'Pengaturan penampilan':'Detail tugas'}">${ui.panel?'<div class="ui-panel-resizer" title="Seret untuk mengubah lebar panel" aria-hidden="true"></div>':''}${ui.panel==='task'?detailPanel():ui.panel==='appearance'?appearancePanel():''}</aside></div></div></div>${statusbar()}`;
  if(ui.view==='settings'&&!canManageBot()){document.querySelectorAll('.wa-settings-content [data-ui-bot-toggle],.wa-settings-content [data-ui-group],.wa-settings-content [data-ui-command-toggle],.wa-settings-content [data-ui-combo^="wa-"],.wa-settings-content [data-ui-action^="wa-"]').forEach(e=>e.disabled=true);}
  $('#ui-main-scroll').scrollTop=scrollTop;
  if($('.ui-kanban'))$('.ui-kanban').scrollLeft=scrollLeft;
  if($('.ui-inspector'))$('.ui-inspector').scrollTop=panelScroll;
  window.__SPACE_REFINED_STATE__=structuredClone(ui);
  console.info('[Space refined]',window.__SPACE_REFINED_STATE__);
}
function view(name) {closePopover();ui.view=name;ui.filter='Semua';ui.search='';ui.panel=null;ui.selected=null;render();$('#ui-main-scroll').scrollTop=0;}
function openTask(id) {ui.selected=Number(id);ui.panel='task';ui.search='';render();$('.ui-inspector').scrollTop=0;}
function toast(text,kind='success',undo=null) {
  const el=$('#refined-toast');toastAction=undo;el.setAttribute('aria-live',kind==='error'?'assertive':'polite');
  el.innerHTML=`<span class="ui-snackbar-symbol ${kind}">${glyph(kind==='error'?'alert':kind==='info'?'info':'check')}</span><span class="ui-snackbar-message">${esc(text)}</span>${undo?'<button data-ui-action="toast-undo">Urungkan</button>':''}<button class="ui-snackbar-close" data-ui-action="close-toast" aria-label="Tutup notifikasi">${glyph('close')}</button>`;
  el.classList.add('visible');clearTimeout(toast.timer);toast.timer=setTimeout(()=>el.classList.remove('visible'),6000);
}

function closeModal() {closePopover();$('#refined-overlay').innerHTML='';}
function modal(title,body,footer='') {
  $('#refined-overlay').innerHTML=`<div class="ui-modal-backdrop" data-ui-action="close-modal"><section class="ui-modal" role="dialog" aria-modal="true" aria-label="${esc(title)}"><header><h2>${title}</h2>${ib('close','close-modal','Tutup dialog')}</header>${body}${footer?`<footer>${footer}</footer>`:''}</section></div>`;
  setTimeout(()=>$('.ui-modal input,.ui-modal textarea,.ui-modal button')?.focus(),10);
}
function newTask(status='Belum mulai') {
  ui.newTaskStatus=status;
  modal('Buat tugas',`<label class="ui-field">Judul tugas<input id="ui-new-title" placeholder="Apa yang perlu dikerjakan?"></label><label class="ui-field">Hasil yang diharapkan<textarea id="ui-new-description" placeholder="Jelaskan hasil yang akan diserahkan…"></textarea></label><div class="ui-field-grid"><label class="ui-field">Penanggung jawab${combo('ui-new-pj',currentUser(),Object.entries(people).filter(([id])=>!['NY','RF'].includes(id)).map(([id,p])=>({value:id,label:p.name})),'Penanggung jawab')}</label><label class="ui-field">Deadline${dateControl('ui-new-date','2026-10-30','Deadline tugas')}</label></div><label class="ui-field">Subdivisi${combo('ui-new-category','Desain',['Desain','Tekno','Video'],'Subdivisi')}</label><p class="ui-modal-help">Tugas akan tampil di board divisi dan jobdesk PJ. Reminder bawaan: H-2, H-1, dan hari deadline.</p>`,action('Batal','close-modal','outline')+action('Bagikan tugas','save-task','primary','plus'));
}
function moveTask(id,status,before=null) {
  const task=ui.tasks.find(t=>t.id===Number(id));if(!task||!columns.includes(status))return;
  const beforeMove=ui.tasks.map(t=>({...t}));const old=task.status;task.status=status;
  if(status==='Selesai')task.progress=100;else if(old==='Selesai')task.progress=0;
  ui.tasks=ui.tasks.filter(t=>t.id!==task.id);
  const index=before?ui.tasks.findIndex(t=>t.id===Number(before)):-1;
  if(index>=0)ui.tasks.splice(index,0,task);else ui.tasks.push(task);
  ui.lastChange=`${task.title}: ${status}`;
  render();toast(old===status?'Urutan tugas diperbarui.':`Dipindahkan ke ${status.toLowerCase()}.`,'success',()=>{ui.tasks=beforeMove;render();toast('Perpindahan tugas dibatalkan.','info');});
}
function moveMenu(id) {
  const t=ui.tasks.find(t=>t.id===Number(id));if(!t)return;
  modal('Pindahkan tugas',`<p class="ui-modal-help">${esc(t.title)}</p><div class="ui-move-options">${columns.map((c,i)=>`<button data-ui-move-id="${t.id}" data-ui-move-status="${c}"><span class="ui-column-dot column-${i}"></span>${c}${c===t.status?glyph('check'):''}</button>`).join('')}</div>`);
}
function ping(meeting=false) {
  modal(meeting?'Ping peserta rapat':'Ping terkait jobdesk',`<p class="ui-modal-help">${meeting?'Peserta rapat divisi Mediatek':esc(activeTask()?.title||'Pengingat untuk PJ')}</p><label class="ui-field">Pesan<textarea>${meeting?'Halo teman-teman, rapat divisi kita mulai pukul 19.00 WIB ya. Sampai bertemu!':'Halo, jangan lupa jobdesk dan deadline-nya ya. Kalau ada kendala, kabari di sini.'}</textarea></label><div class="ui-channel-options"><label><input type="checkbox" checked>Perangkat</label><label><input type="checkbox" checked>Grup WhatsApp</label></div>`,action('Batal','close-modal','outline')+action('Kirim ping','send-ping','primary','send'));
}
function searchResults() {
  const container=$('#ui-search-results');if(!container)return;
  const q=ui.search.toLowerCase();const matches=ui.tasks.filter(t=>(t.title+' '+people[t.pj].name).toLowerCase().includes(q));
  container.hidden=false;
  container.innerHTML=`<div class="ui-search-heading">${q?'HASIL PENCARIAN':'TUGAS TERKINI'}<span>${matches.length}</span></div>${matches.map(t=>`<button data-ui-task="${t.id}">${glyph('task')}<span><strong>${esc(t.title)}</strong><small>${t.space} · ${people[t.pj].short}</small></span>${glyph('chevron')}</button>`).join('')||'<p class="ui-empty">Belum ada hasil yang cocok.</p>'}`;
}
// Custom menus and pickers share the Space surfaces and keyboard behavior.
const choices = new Map();
let popoverAnchor = null, toastAction = null;
function combo(id,value,options,label='') {
  const normalized=options.map(o=>typeof o==='string'?{value:o,label:o}:o);choices.set(id,normalized);
  const selected=normalized.find(o=>o.value===value)||normalized[0];
  return `<div class="ui-combo"><input type="hidden" id="${id}" value="${esc(value)}"><button type="button" class="ui-combo-trigger" role="combobox" aria-label="${esc(label||id)}" aria-haspopup="listbox" aria-expanded="false" aria-controls="listbox-${id}" data-ui-combo="${id}"><span class="ui-combo-label">${esc(selected?.label||value)}</span>${glyph('down')}</button></div>`;
}
function closePopover(restore=false) {
  $('#refined-popovers').innerHTML='';
  if(popoverAnchor?.isConnected){popoverAnchor.setAttribute('aria-expanded','false');if(restore)popoverAnchor.focus();}
  popoverAnchor=null;
}
function showPopover(anchor,html,kind='menu',width=260) {
  closePopover();popoverAnchor=anchor;anchor.setAttribute('aria-expanded','true');
  const root=$('#refined-popovers');root.innerHTML=`<div class="ui-popover ui-popover-${kind}" style="width:${Math.min(width,innerWidth-24)}px">${html}</div>`;
  const el=root.firstElementChild,r=anchor.getBoundingClientRect();
  const left=Math.max(12,Math.min(r.left,innerWidth-el.offsetWidth-12));
  const bottom=innerHeight-42,room=bottom-r.bottom-8;
  el.style.left=`${left}px`;el.style.maxHeight=`${Math.max(140,innerHeight-70)}px`;
  el.style.top=`${room>Math.min(el.scrollHeight,280)?r.bottom+8:Math.max(12,r.top-el.offsetHeight-8)}px`;
  requestAnimationFrame(()=>{if(el.getBoundingClientRect().bottom>bottom)el.style.top=`${Math.max(12,bottom-el.offsetHeight)}px`;});
}
function openCombo(id,anchor) {
  if(popoverAnchor===anchor){closePopover();return;}
  const value=$('#'+id).value;
  showPopover(anchor,`<div class="ui-menu-caption">${esc(anchor.getAttribute('aria-label'))}</div><div role="listbox" id="listbox-${id}" aria-label="${esc(anchor.getAttribute('aria-label'))}">${choices.get(id).map(o=>`<button role="option" aria-selected="${o.value===value}" data-ui-choice="${id}" data-ui-value="${esc(o.value)}"><span>${esc(o.label)}${o.description?`<small>${esc(o.description)}</small>`:''}</span>${o.value===value?glyph('check'):''}</button>`).join('')}</div>`,'menu',Math.max(240,anchor.offsetWidth));
  const selected=$('.ui-popover [aria-selected="true"]')||$('.ui-popover [role=option]');selected?.focus();
}
function choose(id,value) {
  const input=$('#'+id);input.value=value;
  const trigger=input.parentElement.querySelector('[role=combobox]');trigger.querySelector('.ui-combo-label').textContent=choices.get(id).find(o=>o.value===value)?.label||value;
  closePopover();trigger.focus();
  if(id==='ui-task-status')moveTask(ui.selected,value);
  if(id==='ui-command-type'&&$('.wa-sticker-example'))$('.wa-sticker-example').hidden=value!=='Stiker';
  if(id.startsWith('wa-')){if(!canManageBot()){toast('Koneksi ini dikelola Humas.','info');return;}bot()[id.slice(3)]=value;ui.botDirty=true;render();}
}
function dateControl(id,value,label='Tanggal') {
  return `<div class="ui-date-control"><input id="${id}" type="hidden" value="${value}"><button type="button" data-ui-date="${id}" aria-label="${label}" aria-haspopup="dialog">${glyph('calendar')}<span>${new Date(value+'T12:00:00').toLocaleDateString('id-ID',{day:'numeric',month:'long',year:'numeric'})}</span>${glyph('down')}</button></div>`;
}
function datePicker(id,anchor,month) {
  const selected=$('#'+id).value;const current=month||selected.slice(0,7);const [year,m]=current.split('-').map(Number);const start=(new Date(year,m-1,1).getDay()+6)%7;const max=new Date(year,m,0).getDate();
  showPopover(anchor,`<div class="ui-picker-title"><strong>${new Date(year,m-1,1).toLocaleDateString('id-ID',{month:'long',year:'numeric'})}</strong><span class="grow"></span><button data-ui-month="${id}" data-offset="-1" data-month="${current}" aria-label="Bulan sebelumnya">${glyph('chevron')}</button><button data-ui-month="${id}" data-offset="1" data-month="${current}" aria-label="Bulan berikutnya">${glyph('chevron')}</button></div><div class="ui-picker-week">${['S','S','R','K','J','S','M'].map(d=>`<span>${d}</span>`).join('')}</div><div class="ui-picker-days">${Array.from({length:start},()=>'<i></i>').join('')}${Array.from({length:max},(_,i)=>{const value=`${year}-${String(m).padStart(2,'0')}-${String(i+1).padStart(2,'0')}`;return `<button class="${value===selected?'selected':''} ${value==='2026-10-27'?'today':''}" data-ui-pick-date="${id}" data-ui-value="${value}">${i+1}</button>`;}).join('')}</div><div class="ui-picker-foot">Waktu Indonesia Barat</div>`,'calendar',290);
}
function notice(kind,title,text) {
  return `<div class="ui-notice ${kind}" role="${kind==='error'?'alert':'note'}"><span>${glyph(kind==='error'?'alert':kind==='success'?'check':kind==='warning'?'alert':'info')}</span><div><strong>${title}</strong><p>${text}</p></div></div>`;
}
function confirmDialog(title,description,label,actionName,kind='warning') {
  modal('',`<div class="ui-alert-icon ${kind}">${glyph(kind==='danger'?'unlink':'alert')}</div><h2 class="ui-alert-title">${title}</h2><p class="ui-alert-description">${description}</p>`,action('Batal','close-modal','outline')+action(label,actionName,kind==='danger'?'danger':'primary'));
  $('.ui-modal').classList.add('ui-alert-dialog');$('.ui-modal').setAttribute('role','alertdialog');$('.ui-modal').setAttribute('aria-label',title);
}
function colorPopover(anchor) {
  showPopover(anchor,`<div class="ui-menu-caption">WARNA AKSEN SENDIRI</div><div class="ui-color-grid">${['#d7c3a0','#a8c2ac','#a2bbd2','#b6a8d0','#d2a499','#e1b86b','#7eaa9d','#7ea0c8','#a98fbf','#bb786a','#c8c4b6','#575e72'].map(c=>`<button style="--swatch:${c}" data-ui-color-swatch="${c}" aria-label="Pilih ${c}"></button>`).join('')}</div><label class="ui-color-entry"><span id="ui-color-sample" style="background:${ui.accent}"></span><input id="ui-accent-hex" value="${ui.accent}" aria-label="Warna aksen heksadesimal" maxlength="7" spellcheck="false"></label><p class="ui-picker-error" id="ui-color-error" hidden>Gunakan enam digit warna, misalnya #A8C2AC.</p><button class="ui-button primary" data-ui-action="apply-color">Gunakan warna</button>`,'color',260);
}
function spaceMenu(anchor) {
  if(popoverAnchor===anchor){closePopover();return;}
  showPopover(anchor,`<div class="ui-menu-caption">SPACE KAMU · 2025 / 2026</div>${[ui.division,'Medhum'].map(name=>`<button class="ui-space-menu-row" data-ui-space="${name}">${spaceMark(name)}<span><strong>${name}</strong><small>${name==='Medhum'?'Humas × Mediatek':'Space divisi'}</small></span>${ui.space===name?glyph('check'):''}</button>`).join('')}<div class="ui-menu-separator"></div><button class="ui-menu-link" data-ui-view="settings">${glyph('settings')}Pengaturan Space</button>`,'space',300);
}
function periodMenu(anchor) {
  if(popoverAnchor===anchor){closePopover();return;}
  showPopover(anchor,`<div class="ui-menu-caption">KEPENGURUSAN</div><button class="ui-period-menu"><span><strong>2025 / 2026</strong><small>Periode aktif</small></span>${glyph('check')}</button><button class="ui-period-menu" data-ui-action="archive"><span><strong>2024 / 2025</strong><small>Arsip · sesuai izin</small></span>${glyph('archive')}</button>`,'menu',270);
}

function spaceMark(name,size='') {
  if(name==='Medhum'&&ui.logos.Medhum)return `<span class="ui-division-mark ${size}"><img src="${ui.logos.Medhum}" alt="Logo Medhum"></span>`;
  return name==='Medhum'?`<span class="ui-division-mark paired ${size}"><img src="${ui.logos.Humas}" alt=""><img src="${ui.logos.Mediatek}" alt=""></span>`:`<span class="ui-division-mark ${size}"><img src="${ui.logos[name]||ui.logos.Mediatek}" alt="Logo ${esc(name)}"></span>`;
}
function currentUser(){return ui.division==='Humas'?'NY':'AJ';}
function botScope(){return ui.space==='Medhum'?'Humas':ui.division;}
function bot(){return ui.bots[botScope()];}
function canManage(){return ui.role!=='member';}
function canManageBot(){return canManage()&&(ui.space!=='Medhum'||ui.division==='Humas');}
function toggleRow(title,description,key,value,disabled=false) {
  return `<div class="wa-toggle-row"><div><strong>${title}</strong><p>${description}</p></div><button class="ui-toggle ${value?'on':''}" data-ui-bot-toggle="${key}" aria-label="${title}" aria-pressed="${value}" ${disabled?'disabled':''}></button></div>`;
}
function settingsView() {
  if(!canManage())return `<div class="ui-settings-page"><div class="ui-page-heading"><div><span class="ui-eyebrow">PENGATURAN SPACE</span><h1>Ruang ${ui.space}<span>.</span></h1></div></div>${notice('info','Dikelola kadiv dan role 45','Pengaturan koneksi WhatsApp dan identitas Space memerlukan wewenang pengelola. Kamu tetap dapat membuka jobdesk dan catatan divisi.')}${action('Kembali ke board','view-board','outline','board')}</div>`;
  const tabs=[['general','grid','Umum'],['whatsapp','whatsapp','WhatsApp'],['reminders','bell','Reminder & SOP'],['commands','terminal','Perintah bot'],['activity','clock','Aktivitas']];
  return `<div class="ui-settings-page"><div class="ui-page-heading"><div><span class="ui-eyebrow">${ui.space.toUpperCase()} · 2025 / 2026</span><h1>Pengaturan Space<span>.</span></h1><p>Identitas ruang, koneksi, dan pengingat yang menemani divisi.</p></div><span class="wa-role-badge">${glyph('shield')}${ui.role==='45'?'Role 45':'Kadiv '+ui.division}</span></div><nav class="wa-settings-tabs" aria-label="Bagian pengaturan">${tabs.map(([key,symbol,label])=>`<button class="${ui.settingsTab===key?'active':''}" data-ui-setting-tab="${key}">${glyph(symbol)}${label}</button>`).join('')}</nav>${ui.role==='45'?notice('info','Akses operasional role 45','Kamu dapat mengelola WhatsApp dan operasional divisi. Pengaturan role dan akses tetap dikelola kadiv asli.'):''}${ui.settingsTab==='general'?generalSettings():ui.settingsTab==='whatsapp'?whatsappSettings():ui.settingsTab==='reminders'?reminderSettings():ui.settingsTab==='commands'?commandSettings():activitySettings()}</div>`;
}
function generalSettings() {
  return `<div class="wa-settings-content"><section class="wa-card"><div class="wa-card-heading"><div><h2>Identitas Space</h2><p>Logo divisi menjadi identitas awal ruangmu.</p></div>${pill('Per divisi','muted')}</div><div class="wa-logo-editor">${spaceMark(ui.space,'large')}<div><strong>Logo ${ui.space}</strong><p>Logo bawaan dari HIMASIF. Kamu dapat menggantinya untuk Space ini.</p><div class="wa-inline-actions"><label class="ui-button outline" for="ui-logo-upload">${glyph('upload')}Unggah logo</label><input id="ui-logo-upload" type="file" accept="image/png,image/jpeg,image/webp,image/svg+xml" hidden>${action('Gunakan logo bawaan','reset-logo','ghost')}</div><small>PNG, JPG, WebP, atau SVG · maksimum 2 MB</small></div></div><div class="ui-field-grid"><label class="ui-field">Nama Space<input id="space-name" value="${ui.space}" readonly></label><label class="ui-field">Periode<input value="2025 / 2026" readonly></label></div></section><section class="wa-card"><div class="wa-card-heading"><div><h2>Ruang bersama Medhum</h2><p>Kerja sama dengan divisi lain tetap punya tempat sendiri.</p></div>${spaceMark('Medhum')}</div><div class="wa-shared-members">${spaceMark('Humas')}Humas${glyph('link')}${spaceMark('Mediatek')}Mediatek<span class="grow"></span>${pill('Ruang bersama','sage')}</div><p class="wa-help">Logo divisi asal tetap menjadi identitas anggota dan pekerjaan. Identitas ruang bersama dapat dikelola terpisah.</p></section></div>`;
}
function whatsappSettings() {
  const b=bot(),scope=botScope(),manage=canManageBot();const off=manage?'':'disabled';
  return `<div class="wa-settings-content">${ui.space==='Medhum'?notice('info','Medhum dilayani bot Humas','Bot Humas menangani grup Humas dan Medhum. Bot Mediatek tetap berada di grup Mediatek agar pengingat tidak berganda.'):''}<div class="wa-connection-grid"><section class="wa-card wa-connection-card"><div class="wa-card-heading"><div><span class="ui-eyebrow">AKUN TERHUBUNG</span><h2>WhatsApp ${scope}</h2><p>Satu koneksi untuk ruang dan grup divisi yang ditetapkan.</p></div><span class="wa-connection-pill ${b.connected?'connected':''}"><i></i>${b.connected?'Terhubung':'Belum terhubung'}</span></div>${b.connected?`<div class="wa-account">${spaceMark(scope,'large')}<div><strong>${esc(b.accountLabel)}</strong><span>+62 8•• •••• ${scope==='Humas'?'1802':'2401'}</span><small>${b.accountType==='dedicated'?'Nomor khusus divisi':'Akun WhatsApp kadiv'}</small></div><span class="grow"></span>${glyph('shield')}</div><div class="wa-connection-stats"><div><i class="live-dot"></i><span>Koneksi terakhir<strong>Baru saja · 14.30 WIB</strong></span></div><div>${glyph('check')}<span>Pengiriman terakhir<strong>Reminder diterima kanal</strong></span></div></div><div class="wa-connection-actions">${action('Ganti akun','wa-reconnect','outline','qr').replace('<button','<button '+off)}${action('Putuskan koneksi','wa-disconnect','ghost').replace('<button','<button '+off)}</div>`:`<div class="wa-disconnected"><span>${glyph('whatsapp')}</span><h3>Hubungkan WhatsApp divisi</h3><p>Gunakan nomor khusus atau akun kadiv. Hubungkan melalui perangkat tertaut di WhatsApp.</p>${action('Tampilkan QR','wa-show-qr','primary','qr').replace('<button','<button '+off)}</div>`}</section><aside class="wa-summary-card"><span class="ui-eyebrow">BOT ${scope.toUpperCase()}</span><h3>Pengingat tetap berjalan.</h3><p>Jadwal tersimpan di server, sehingga bot tetap aktif saat aplikasi ditutup.</p><dl><dt>Reminder tugas</dt><dd>H-2, H-1, hari deadline</dd><dt>Jam pengiriman</dt><dd>${b.time.replace(':','.')} WIB</dd><dt>Grup aktif</dt><dd>${b.groups.length} grup terhubung</dd></dl><button data-ui-setting-tab="activity">Lihat aktivitas pengiriman${glyph('arrow')}</button></aside></div><section class="wa-card"><div class="wa-card-heading"><div><h2>Identitas dalam pesan</h2><p>Atur cara bot menyapa anggota di grup.</p></div>${glyph('message')}</div><div class="wa-identity-grid"><div><label class="ui-field">Label bot<input id="wa-name" value="${esc(b.name)}" ${!manage?'readonly':''}></label><div class="ui-field-grid"><label class="ui-field">Jenis akun${combo('wa-accountType',b.accountType,[{value:'dedicated',label:'Nomor khusus divisi'},{value:'kadiv',label:'Akun kadiv'}],'Jenis akun WhatsApp')}</label><label class="ui-field">Gaya pesan${combo('wa-style',b.style,['Santai','Ringkas','Formal'],'Gaya pesan bot')}</label></div><p class="wa-help">Label muncul dalam isi pesan. Nama profil WhatsApp mengikuti akun yang terhubung.</p></div><div class="wa-message-preview"><div class="wa-preview-caption">PRATINJAU PESAN</div><div class="wa-message-bubble"><strong class="wa-message-name">${esc(b.name)}</strong><p>${b.style==='Formal'?'Selamat pagi. Berikut pengingat tugas divisi.':b.style==='Ringkas'?'Pengingat jobdesk hari ini.':'Pagi, teman-teman! Ada pengingat kecil nih.'}</p><p><b>Lesson Learned — Oktober</b><br>Deadline desain: 28 Oktober<br><span>@Ahimsa</span>, jangan lupa ya.</p><small>06.00${glyph('check')}</small></div></div></div></section><section class="wa-card"><div class="wa-card-heading"><div><h2>Grup tujuan</h2><p>Tentukan ke mana pengingat dan konfirmasi dikirim.</p></div>${action('Sinkronkan grup','wa-sync-groups','outline','refresh').replace('<button','<button '+off)}</div><div class="wa-group-list">${[scope+' 2025 / 2026',...(scope==='Humas'?['Medhum 2025 / 2026']:[])].map((name,i)=>`<button class="wa-group-row ${b.groups.includes(name)?'selected':''}" data-ui-group="${name}" ${off}><span class="wa-checkbox">${b.groups.includes(name)?glyph('check'):''}</span>${spaceMark(i===1?'Medhum':scope)}<span><strong>${name}</strong><small>${i===1?'Follow-up H-4, konsep dan bukti kiriman':'Reminder jobdesk, rapat, dan rekap mingguan'}</small></span><span class="grow"></span>${pill(i===1?'Bersama':'Divisi',i===1?'sage':'muted')}</button>`).join('')}</div>${scope==='Mediatek'?`<div class="wa-group-locked">${glyph('lock')}Grup Medhum menggunakan bot Humas.</div>`:''}</section>${manage?settingsSaveBar():notice('info','Dikelola divisi Humas','Koneksi ini hanya dapat diubah oleh kadiv atau role 45 di Humas.')}</div>`;
}
function settingsSaveBar(){return `<div class="wa-save-bar"><span>${glyph(ui.botDirty?'clock':'check')}${ui.botDirty?'Ada perubahan pada preview':'Pengaturan divisi tersimpan di preview'}</span><span class="grow"></span>${action('Pratinjau pesan','wa-preview','outline','message')}${action('Simpan perubahan','wa-save','primary','check')}</div>`;}
function reminderSettings() {
  const b=bot();return `<div class="wa-settings-content"><section class="wa-card"><div class="wa-card-heading"><div><h2>Reminder jobdesk</h2><p>Dihitung dari deadline tugas. Untuk desain, gunakan deadline desain.</p></div>${glyph('bell')}</div><div class="wa-reminder-chips">${[['h2','H-2','Dua hari sebelum'],['h1','H-1','Satu hari sebelum'],['h0','Hari deadline','Pada tanggal tugas']].map(([key,title,desc])=>`<button class="${b[key]?'selected':''}" data-ui-bot-toggle="${key}"><span class="wa-checkbox">${b[key]?glyph('check'):''}</span><strong>${title}</strong><small>${desc}</small></button>`).join('')}</div><div class="wa-schedule-row"><label class="ui-field">Jam pengiriman${combo('wa-time',b.time,['06:00','07:00','08:00','09:00','12:00','15:00','18:00'].map(v=>({value:v,label:v.replace(':','.')+' WIB'})),'Jam pengiriman')}</label><p>${glyph('globe')}Zona waktu Asia/Jakarta</p></div>${toggleRow('Pengingat tugas terlambat','Satu kali sehari sampai penyerahan tercatat atau tugas ditutup.','overdue',b.overdue)}${toggleRow('Tag penanggung jawab','Sebut PJ terkait pada pesan pengingat di grup.','mention',b.mention)}${toggleRow('Gabungkan pada jam yang sama','Beberapa tugas dalam satu slot dikirim sebagai satu ringkasan.','digest',b.digest)}</section><section class="wa-card"><div class="wa-card-heading"><div><h2>Rapat dan rekap</h2><p>Pengingat rapat mengikuti waktu kegiatan.</p></div>${glyph('calendar')}</div><div class="wa-meeting-reminders"><span>H-1</span><span>1 jam sebelum</span><span>15 menit sebelum</span></div>${toggleRow('Rekap mingguan divisi','Ringkasan pekerjaan dan temuan yang perlu ditinjau, ke grup divisi sendiri.','weekly',b.weekly)}</section><section class="wa-card wa-sop-card"><div class="wa-card-heading"><div><span class="ui-eyebrow">ALUR BERSAMA MEDHUM</span><h2>Aturan SOP tetap terjaga.</h2></div>${glyph('lock')}</div><div class="wa-sop-rules"><div><span>H-4</span><strong>Follow-up Medhum</strong><p>Bot Humas membantu PJ mengingatkan desain, mengacu jadwal posting.</p></div><div><span>H-2</span><strong>Deadline desain</strong><p>Diturunkan dari jadwal posting sekali, sesuai alur yang disepakati.</p></div></div><p class="wa-help">Pengecualian dan temuan tetap melalui tinjauan. Pesan terkirim bukan bukti seluruh SOP telah terpenuhi.</p></section>${settingsSaveBar()}</div>`;
}
function commandSettings() {
  return `<div class="wa-settings-content"><section class="wa-card"><div class="wa-card-heading"><div><h2>Perintah buatan divisi</h2><p>Sedikit ciri khas untuk ruang dan bot milik kalian.</p></div>${action('Tambah perintah','wa-new-command','primary','plus')}</div><div class="wa-command-list">${bot().commands.map((c,index)=>`<div><span class="wa-command-icon">${glyph(c.type==='Stiker'?'sticker':'message')}</span><div><code>${esc(c.name)}</code><p>${esc(c.description)} · ${c.type}</p></div><span class="grow"></span>${ib('edit','wa-edit-command-'+index,'Edit '+c.name)}<button class="ui-toggle ${c.enabled?'on':''}" data-ui-command-toggle="${index}" aria-label="Aktifkan ${esc(c.name)}" aria-pressed="${c.enabled}"></button></div>`).join('')}</div></section><section class="wa-card"><div class="wa-card-heading"><div><h2>Perintah operasional</h2><p>Perintah inti mengikuti wewenang pengirim dan aturan Space.</p></div>${glyph('lock')}</div><div class="wa-core-commands">${[['!jobdesk','Lihat tugas yang relevan'],['!done','Kirim penyelesaian untuk dicocokkan'],['!ping','Pengingat ke sasaran yang diizinkan']].map(([c,d])=>`<div><code>${c}</code><span>${d}</span>${glyph('lock')}</div>`).join('')}</div>${notice('info','Perintah inti dilindungi','Perintah kustom tidak dapat menggantikan aturan reminder, SOP, role, atau akses.')}</section>${settingsSaveBar()}</div>`;
}
function activitySettings() {
  return `<div class="wa-settings-content"><div class="wa-log-summary"><div><span>Diterima kanal</span><strong>24</strong></div><div><span>Dalam antrean</span><strong>2</strong></div><div><span>Perlu diperiksa</span><strong>${bot().retried?0:1}</strong></div></div><section class="wa-card"><div class="wa-card-heading"><div><h2>Aktivitas pengiriman</h2><p>Status pesan dibedakan dari status koneksi bot.</p></div>${pill('Hari ini','muted')}</div><div class="wa-log-list">${[['check','Reminder Lesson Learned',botScope()+' · 06.00 WIB','Diterima kanal','sage'],['clock','Rekap mingguan divisi','Dijadwalkan Jumat, 18.00 WIB','Dalam antrean','sand'],['alert','Ping rapat divisi',bot().retried?'Dijadwalkan ulang sekali · Baru saja':'Respons kanal belum tersedia · 13.55 WIB',bot().retried?'Dalam antrean':'Status tidak diketahui',bot().retried?'sand':'rose']].map(([s,title,desc,status,color])=>`<div><span class="wa-log-icon ${color}">${glyph(s)}</span><div><strong>${title}</strong><p>${desc}</p></div><span class="grow"></span>${pill(status,color)}</div>`).join('')}</div>${!bot().retried?notice('warning','Periksa pesan sebelum mengirim ulang','Status tidak diketahui dapat berarti pesan sudah diterima kanal. Pastikan tidak ada pesan ganda.'):notice('success','Satu percobaan ulang dijadwalkan','Kejadian tetap memakai identitas yang sama agar tidak menggandakan pengiriman.')}${!bot().retried?action('Tinjau percobaan ulang','wa-retry','outline','refresh'):''}</section></div>`;
}
function qrDialog() {
  const grid=Array.from({length:29},(_,y)=>Array.from({length:29},(_,x)=>{const finder=(ox,oy)=>x>=ox&&x<ox+7&&y>=oy&&y<oy+7;let on=false;for(const [ox,oy]of[[0,0],[22,0],[0,22]])if(finder(ox,oy)){const dx=x-ox,dy=y-oy;on=dx===0||dx===6||dy===0||dy===6||(dx>=2&&dx<=4&&dy>=2&&dy<=4);return on?`<rect x="${x+2}" y="${y+2}" width="1" height="1"/>`:'';}return((x*7+y*11+x*y)%5<2)?`<rect x="${x+2}" y="${y+2}" width="1" height="1"/>`:'';}).join('')).join('');
  modal('Hubungkan WhatsApp',`<div class="wa-qr-intro">${spaceMark(botScope())}<div><strong>Bot ${botScope()}</strong><p>Nomor khusus atau akun kadiv</p></div></div><div class="wa-qr"><svg viewBox="0 0 33 33" aria-label="Ilustrasi QR untuk preview">${grid}</svg></div><p class="wa-qr-caption">QR ilustrasi · tidak dapat digunakan untuk menghubungkan akun</p><ol class="wa-qr-steps"><li>Buka WhatsApp di ponsel.</li><li>Pilih <strong>Perangkat tertaut → Tautkan perangkat</strong>.</li><li>Pindai kode yang disediakan saat layanan terhubung.</li></ol><div class="wa-qr-refresh">${glyph('clock')}Contoh alur koneksi${action('Perbarui QR','wa-refresh-qr','ghost','refresh')}</div>`,action('Batal','close-modal','outline')+action('Simulasikan terhubung','wa-connect','primary','check'));
}
function commandDialog(index=null) {
  const c=index===null?{name:'!',description:'',type:'Teks'}:bot().commands[index];ui.editCommand=index;
  ui.commandMedia=c.media||null;modal(index===null?'Tambah perintah':'Edit perintah',`<label class="ui-field">Perintah<input id="wa-command-name" value="${esc(c.name)}" placeholder="!semangat"></label><label class="ui-field">Jenis balasan${combo('ui-command-type',c.type,['Teks','Stiker'],'Jenis balasan')}</label><label class="ui-field">Isi atau deskripsi balasan<textarea id="wa-command-description" placeholder="Tulis balasan untuk perintah ini…">${esc(c.description)}</textarea></label><div class="wa-sticker-example" ${c.type!=='Stiker'?'hidden':''}><span id="wa-sticker-preview">${aceArt()}</span><div><strong>Stiker balasan</strong><label class="ui-button outline" for="wa-sticker-upload">Pilih stiker</label><input type="file" id="wa-sticker-upload" accept="image/webp,image/png,image/gif" hidden></div></div><p id="wa-command-error" class="ui-field-error" hidden></p>`,action('Batal','close-modal','outline')+action('Simpan perintah','wa-save-command','primary','check'));
}

const actions={
  'custom-color':el=>colorPopover(el),
  'apply-color':()=>{const value=$('#ui-accent-hex').value.trim();if(!/^#[0-9a-f]{6}$/i.test(value)){$('#ui-color-error').hidden=false;return;}ui.accent=value;render();toast('Warna aksen diperbarui.');},
  'close-toast':()=>$('#refined-toast').classList.remove('visible'),
  'toast-undo':()=>{const undo=toastAction;$('#refined-toast').classList.remove('visible');undo?.();},
  'reset-logo':()=>{if(ui.space==='Medhum')delete ui.logos.Medhum;else ui.logos[ui.space]=ui.space==='Humas'?'./assets/humas.png':'./assets/mediatek.png';render();toast('Logo bawaan divisi dipulihkan.');},
  'wa-disconnect':()=>{if(!canManageBot())return;confirmDialog('Putuskan koneksi WhatsApp?',`Bot ${botScope()} akan berhenti menerima dan mengirim pesan sampai akun dihubungkan lagi. Pengingat perangkat tetap berjalan.`,'Putuskan koneksi','wa-disconnect-confirm','danger');},
  'wa-disconnect-confirm':()=>{if(!canManageBot())return;bot().connected=false;closeModal();render();toast('Koneksi diputuskan dalam preview.','info');},
  'wa-reconnect':()=>{if(!canManageBot())return;confirmDialog('Ganti akun WhatsApp?',`Hubungkan nomor baru untuk bot ${botScope()}. Pengiriman WhatsApp dijeda selama proses pergantian akun.`,'Lanjutkan ke QR','wa-reconnect-confirm');},
  'wa-reconnect-confirm':()=>{if(!canManageBot())return;bot().connected=false;closeModal();render();qrDialog();},
  'wa-show-qr':()=>{if(canManageBot())qrDialog();},
  'wa-refresh-qr':()=>{qrDialog();toast('Ilustrasi QR diperbarui.','info');},
  'wa-connect':()=>{if(!canManageBot())return;bot().connected=true;closeModal();render();toast('Simulasi: akun WhatsApp terhubung.');},
  'wa-save':()=>{if(!canManageBot())return;if(!bot().name.trim()){modal('Lengkapi label bot',notice('error','Label bot belum diisi','Isi nama yang akan muncul dalam pesan bot.'),action('Kembali','close-modal','primary'));return;}ui.botDirty=false;render();toast('Pengaturan WhatsApp disimpan untuk preview.');},
  'wa-sync-groups':()=>{if(canManageBot())toast('Simulasi: daftar grup sudah diperbarui.');},
  'wa-preview':()=>modal('Pratinjau pesan',`<div class="wa-preview-dialog"><div class="wa-message-bubble"><strong>${esc(bot().name)}</strong><p>${bot().style==='Formal'?'Selamat pagi. Berikut pengingat tugas divisi.':bot().style==='Ringkas'?'Pengingat jobdesk hari ini.':'Pagi, teman-teman! Ada pengingat kecil nih.'}</p><p><b>Lesson Learned — Oktober</b><br>Deadline desain: 28 Oktober<br><span>@Ahimsa</span>, jangan lupa ya.</p><small>06.00${glyph('check')}</small></div></div><p class="ui-modal-help">Pesan contoh mengikuti label dan gaya bot divisi.</p>`,action('Tutup','close-modal','outline')),
  'wa-new-command':()=>{if(canManageBot())commandDialog();},
  'wa-save-command':()=>{if(!canManageBot())return;const name=$('#wa-command-name').value.trim().toLowerCase(),description=$('#wa-command-description').value.trim(),error=$('#wa-command-error');let message='';if(!/^![a-z0-9_-]{1,24}$/.test(name))message='Awali dengan !, lalu gunakan huruf, angka, - atau _.';else if(['!jobdesk','!done','!ping','!reminder','!role','!akses'].includes(name))message='Perintah inti dilindungi. Pilih nama perintah lain.';else if(bot().commands.some((c,index)=>index!==ui.editCommand&&c.name===name))message='Nama perintah ini sudah digunakan.';else if(!description)message='Isi balasan atau deskripsi stiker terlebih dahulu.';if(message){error.textContent=message;error.hidden=false;return;}const value={name,description,type:$('#ui-command-type').value,enabled:true,media:ui.commandMedia||null};if(ui.editCommand===null)bot().commands.push(value);else bot().commands[ui.editCommand]=value;ui.botDirty=true;closeModal();render();toast('Perintah divisi diperbarui.');},
  'wa-retry':()=>confirmDialog('Jadwalkan satu percobaan ulang?','Pastikan pesan sebelumnya belum diterima. Percobaan ulang memakai kejadian yang sama agar pengiriman tidak berganda.','Jadwalkan ulang','wa-retry-confirm'),
  'wa-retry-confirm':()=>{if(!canManageBot())return;bot().retried=true;closeModal();render();toast('Satu percobaan ulang dijadwalkan pada preview.');},

  sidebar:()=>{ui.collapsed=!ui.collapsed;render();},
  appearance:()=>{ui.panel=ui.panel==='appearance'?null:'appearance';render();},
  'close-panel':()=>{ui.panel=null;ui.selected=null;render();},
  'focus-search':()=>{$('#ui-search').focus();$('#ui-search').select();searchResults();},
  'view-board':()=>view('board'),'view-calendar':()=>view('calendar'),
  'new-task':()=>newTask(),'close-modal':closeModal,
  'save-task':()=>{const title=$('#ui-new-title').value.trim();if(!title){$('#ui-new-title').focus();return;}const category=$('#ui-new-category').value;const date=new Date($('#ui-new-date').value+'T12:00:00');ui.tasks.push({id:Math.max(...ui.tasks.map(t=>t.id))+1,title,desc:$('#ui-new-description').value||'Tugas baru dari Space divisi.',category,pj:$('#ui-new-pj').value,date:Number.isNaN(date.valueOf())?'30 Okt':date.toLocaleDateString('id-ID',{day:'numeric',month:'short'}),status:ui.newTaskStatus||'Belum mulai',color:{Desain:'sand',Tekno:'sage',Video:'rose'}[category],space:ui.space,progress:0,label:'Terjadwal'});closeModal();render();toast('Tugas ditambahkan ke board dan jobdesk PJ dalam preview.');},
  ping:()=>ping(false),'ping-meeting':()=>ping(true),'send-ping':()=>{closeModal();toast('Simulasi: ping dikirim melalui kanal pilihan.');},
  'confirm-task':()=>{const t=activeTask();if(t)moveTask(t.id,'Selesai');toast('Kiriman dikonfirmasi sesuai tugas pada preview.');},
  'previous-task':()=>{const index=ui.tasks.findIndex(t=>t.id===ui.selected);openTask(ui.tasks[(index-1+ui.tasks.length)%ui.tasks.length].id);},
  'next-task':()=>{const index=ui.tasks.findIndex(t=>t.id===ui.selected);openTask(ui.tasks[(index+1)%ui.tasks.length].id);},
  'task-options':()=>moveMenu(ui.selected),help:()=>toast('Pengajuan bantuan dicatat pada preview; menunggu tinjauan kadiv.'),
  comment:()=>{const input=$('#ui-comment');if(!input?.value.trim())return;ui.comments.push({task:ui.selected,text:input.value.trim()});render();$('.ui-inspector').scrollTop=$('.ui-inspector').scrollHeight;},
  online:()=>{ui.online=!ui.online;render();},
  motion:()=>{ui.motion=!ui.motion;render();},ace:()=>{ui.ace=!ui.ace;render();},
  'reset-appearance':()=>{ui.theme='dark';ui.accent=accents.sand;ui.density='comfortable';ui.panelWidth=420;ui.collapsed=false;ui.motion=true;render();},
  'private-share':()=>{ui.privateShare=!ui.privateShare;render();},
  'period':el=>periodMenu(el),
  'space-menu':el=>spaceMenu(el),
  'notifications':()=>modal('Notifikasi',`<div class="ui-move-options"><button data-ui-task="5">${glyph('send')}Oktavia mengirim hasil Sempro — Nadya${glyph('chevron')}</button><button data-ui-action="meeting">${glyph('calendar')}Rapat divisi malam ini, 19.00 WIB${glyph('chevron')}</button></div>`),
  'meeting':()=>modal('Rapat divisi Mediatek',`<div class="ui-meeting-info">${pill('Malam ini','sage')}<h3>Evaluasi Oktober & rencana November</h3><p>${glyph('calendar')}Selasa, 27 Oktober 2026</p><p>${glyph('clock')}19.00 – 20.30 WIB</p><p>${glyph('globe')}Google Meet</p>${avatars()}<small>8 peserta · Reminder H-1, 1 jam, 15 menit</small></div>`,action('Buka notulensi','meeting-notes','outline','note')+action('Ping peserta','ping-meeting','primary','send')),
  'meeting-notes':()=>{closeModal();view('notes');},
  'shortcuts':()=>modal('Shortcut Space',`<div class="ui-shortcut-list">${[['Ctrl / ⌘ K','Fokus pencarian'],['Ctrl / ⌘ B','Buka / ciutkan sidebar'],['Ctrl / ⌘ ,','Pengaturan penampilan'],['Esc','Tutup detail atau dialog'],['Enter','Buka kartu yang difokuskan'],['Alt 1 / 2 / 3','Beranda / board / agenda']].map(([key,label])=>`<div><span>${label}</span><kbd>${key}</kbd></div>`).join('')}</div>`),
  'new-note':()=>{view('notes');toast('Pilih template dan edit judul/isi catatan pada preview.');},
  'personal-reminder':()=>modal('Reminder pribadi',`<label class="ui-field">Ingatkan tentang<input placeholder="Contoh: cek referensi desain"></label><div class="ui-field-grid"><label class="ui-field">Tanggal${dateControl('ui-personal-date','2026-10-27','Tanggal reminder')}</label><label class="ui-field">Jam${combo('ui-personal-time','20:30',['06:00','09:00','12:00','15:00','18:00','20:30','21:00'],'Jam reminder')}</label></div><p class="ui-modal-help">${glyph('lock')}Privat secara default.</p>`,action('Simpan reminder','save-reminder','primary','bell')),
  'save-reminder':()=>{closeModal();view('reminders');toast('Reminder disimpan selama preview.');},
  'profile':()=>modal('Akun kamu',`<div class="ui-profile-detail">${avatar('AJ')}<div><strong>Ahimsa Jenar</strong><p>Kadiv Mediatek · 2025 / 2026</p></div></div><p class="ui-modal-help">Space yang dapat diakses: Mediatek dan Medhum.</p>`),
  'archive':()=>toast('Arsip periode sebelumnya dibuka sesuai izin kadiv.'),
  'attachment':()=>toast('Tautan konsep dan hasil menggunakan data contoh.'),
  'new-space':()=>toast('Space dan keanggotaan dikelola melalui admin organisasi.'),
  'today':()=>toast('Menampilkan minggu 26 Oktober – 1 November 2026.'),
  'prev-week':()=>toast('Kalender memakai minggu contoh untuk penilaian UI.'),
  'next-week':()=>toast('Kalender memakai minggu contoh untuk penilaian UI.'),
  'filter':()=>{ui.filter=ui.filter==='Semua'?'Desain':'Semua';render();},
  'board-options':()=>toast('Seret kartu, atau buka menu tiga titik untuk memindahkan dengan keyboard.'),
};
document.addEventListener('click',event=>{
  const el=event.target.closest('button,[data-ui-card],[data-ui-action]');
  if(!event.target.closest('.ui-search-wrap'))$('#ui-search-results')?.setAttribute('hidden','');
  if(!event.target.closest('.ui-popover,[data-ui-combo],[data-ui-date],[data-ui-action=space-menu],[data-ui-action=period],[data-ui-action=custom-color]'))closePopover();
  if(!el)return;
  if(el.dataset.uiCombo){openCombo(el.dataset.uiCombo,el);return;}
  if(el.dataset.uiChoice){choose(el.dataset.uiChoice,el.dataset.uiValue);return;}
  if(el.dataset.uiDate){datePicker(el.dataset.uiDate,el);return;}
  if(el.dataset.uiMonth){const [y,m]=el.dataset.month.split('-').map(Number),date=new Date(y,m-1+Number(el.dataset.offset),1),month=date.getFullYear()+'-'+String(date.getMonth()+1).padStart(2,'0');datePicker(el.dataset.uiMonth,popoverAnchor,month);return;}
  if(el.dataset.uiPickDate){const input=$('#'+el.dataset.uiPickDate);input.value=el.dataset.uiValue;const button=input.nextElementSibling;button.querySelector('span').textContent=new Date(input.value+'T12:00:00').toLocaleDateString('id-ID',{day:'numeric',month:'long',year:'numeric'});closePopover();button.focus();return;}
  if(el.dataset.uiColorSwatch){$('#ui-accent-hex').value=el.dataset.uiColorSwatch;$('#ui-color-sample').style.background=el.dataset.uiColorSwatch;return;}
  if(el.dataset.uiSettingTab){closePopover();ui.settingsTab=el.dataset.uiSettingTab;render();return;}
  if(el.dataset.uiBotToggle){if(!canManageBot())return;const key=el.dataset.uiBotToggle;bot()[key]=!bot()[key];ui.botDirty=true;render();return;}
  if(el.dataset.uiGroup){if(!canManageBot())return;const groups=bot().groups;bot().groups=groups.includes(el.dataset.uiGroup)?groups.filter(g=>g!==el.dataset.uiGroup):[...groups,el.dataset.uiGroup];ui.botDirty=true;render();return;}
  if(el.dataset.uiCommandToggle){if(!canManageBot())return;const c=bot().commands[Number(el.dataset.uiCommandToggle)];c.enabled=!c.enabled;ui.botDirty=true;render();return;}
  if(el.dataset.uiAction?.startsWith('wa-edit-command-')){commandDialog(Number(el.dataset.uiAction.split('-').at(-1)));return;}
  if(el.classList.contains('ui-modal-backdrop')&&event.target!==el)return;
  if(el.dataset.uiView){if(el.dataset.uiOwnSpace)ui.space=el.dataset.uiOwnSpace;view(el.dataset.uiView);return;}
  if(el.dataset.uiSpace){const wasSettings=ui.view==='settings';ui.space=el.dataset.uiSpace;closeModal();view(wasSettings?'settings':'home');return;}
  if(el.dataset.uiTask){closeModal();openTask(el.dataset.uiTask);return;}
  if(el.dataset.uiMoveMenu){moveMenu(el.dataset.uiMoveMenu);return;}
  if(el.dataset.uiMoveStatus){closeModal();moveTask(el.dataset.uiMoveId,el.dataset.uiMoveStatus);return;}
  if(el.dataset.uiNewColumn){newTask(el.dataset.uiNewColumn);return;}
  if(el.dataset.uiCard){if(Date.now()>ui.suppressClickUntil)openTask(el.dataset.uiCard);return;}
  if(el.dataset.uiTheme){ui.theme=el.dataset.uiTheme;render();return;}
  if(el.dataset.uiAccent){ui.accent=el.dataset.uiAccent;render();return;}
  if(el.dataset.uiDensity){ui.density=el.dataset.uiDensity;render();return;}
  if(el.dataset.uiFilter){ui.filter=el.dataset.uiFilter;render();return;}
  if(el.dataset.uiAction)actions[el.dataset.uiAction]?.(el);
});
document.addEventListener('focusin',event=>{if(event.target.id==='ui-search')searchResults();});
document.addEventListener('input',event=>{
  if(event.target.id==='wa-name'&&canManageBot()){bot().name=event.target.value;ui.botDirty=true;document.querySelectorAll('.wa-message-name').forEach(e=>e.textContent=event.target.value);const save=$('.wa-save-bar>span');if(save)save.innerHTML=glyph('clock')+'Ada perubahan pada preview';}
  if(event.target.id==='ui-accent-hex'&&/^#[0-9a-f]{6}$/i.test(event.target.value))$('#ui-color-sample').style.background=event.target.value;
  if(event.target.id==='ui-search'){ui.search=event.target.value;searchResults();}
  if(event.target.id==='ui-custom-accent'){ui.accent=event.target.value;applyAppearance();$('.ui-setting-title span').textContent=ui.accent.toUpperCase();}
  if(event.target.id==='ui-panel-width'){ui.panelWidth=Number(event.target.value);applyAppearance();$('.ui-panel-width-label span').textContent=`${ui.panelWidth} px`;}
});
document.addEventListener('change',event=>{
  if(['ui-logo-upload','wa-sticker-upload'].includes(event.target.id)){
    const file=event.target.files?.[0];if(!file)return;if(file.size>2*1024*1024||!file.type.startsWith('image/')){modal('Gambar belum dapat digunakan',notice('error','Pilih gambar maksimal 2 MB','Gunakan PNG, JPG, WebP, SVG, atau format stiker yang didukung.'),action('Tutup','close-modal','primary'));return;}
    const target=event.target.id,reader=new FileReader();reader.onload=()=>{if(target==='ui-logo-upload'){ui.logos[ui.space]=reader.result;render();toast('Logo Space diperbarui dalam preview.');}else{ui.commandMedia=reader.result;$('#wa-sticker-preview').innerHTML=`<img src="${reader.result}" alt="Stiker pilihan">`;}};reader.readAsDataURL(file);return;
  }
if(event.target.id==='ui-task-status')moveTask(ui.selected,event.target.value);if(['ui-custom-accent','ui-panel-width'].includes(event.target.id))render();});
document.addEventListener('keydown',event=>{
  const editing=event.target.closest('input,textarea,select,[contenteditable="true"]');
  if((event.ctrlKey||event.metaKey)&&['k','b',','].includes(event.key.toLowerCase())){event.preventDefault();actions[event.key.toLowerCase()==='k'?'focus-search':event.key.toLowerCase()==='b'?'sidebar':'appearance']();return;}
  if(event.altKey&&['1','2','3'].includes(event.key)){event.preventDefault();view(['home','board','calendar'][Number(event.key)-1]);return;}
  if($('.ui-popover')&&['ArrowDown','ArrowUp','Home','End'].includes(event.key)){const opts=[...$('.ui-popover').querySelectorAll('[role=option]')];if(opts.length){event.preventDefault();const index=opts.indexOf(document.activeElement),next=event.key==='Home'?0:event.key==='End'?opts.length-1:(index+(event.key==='ArrowDown'?1:-1)+opts.length)%opts.length;opts[next].focus();return;}}
  if(event.target.matches('[data-ui-combo]')&&['ArrowDown','ArrowUp'].includes(event.key)){event.preventDefault();openCombo(event.target.dataset.uiCombo,event.target);return;}
  if(event.key==='Escape'&&$('.ui-popover')){event.preventDefault();closePopover(true);return;}
  if(event.key==='Escape'){if($('.ui-modal'))closeModal();else if($('#ui-search-results')&&!$('#ui-search-results').hidden){$('#ui-search-results').hidden=true;$('#ui-search').blur();}else actions['close-panel']();return;}
  if(event.key==='Enter'&&event.target.id==='ui-search'){const first=$('#ui-search-results [data-ui-task]');if(first){event.preventDefault();openTask(first.dataset.uiTask);}return;}
  if(event.key==='Enter'&&event.target.id==='ui-comment'){event.preventDefault();actions.comment();return;}
  if(!editing&&['Enter',' '].includes(event.key)&&event.target.matches('[data-ui-card]')){event.preventDefault();openTask(event.target.dataset.uiCard);}
  if(event.key==='?'&&!editing){event.preventDefault();actions.shortcuts();}
  if(event.key==='Tab'&&$('.ui-modal')){const focusable=[...$('.ui-modal').querySelectorAll('button,input,textarea,select')].filter(e=>!e.disabled&&e.type!=='hidden'&&e.getClientRects().length);const first=focusable[0],last=focusable.at(-1);if(event.shiftKey&&document.activeElement===first){event.preventDefault();last.focus();}else if(!event.shiftKey&&document.activeElement===last){event.preventDefault();first.focus();}}
});
function clearDrag(){document.querySelectorAll('.ui-drop-marker').forEach(e=>e.remove());document.querySelectorAll('.drag-over,.is-dragging').forEach(e=>e.classList.remove('drag-over','is-dragging'));}
document.addEventListener('dragstart',event=>{const card=event.target.closest('[data-ui-card]');if(!card)return;ui.dragging=Number(card.dataset.uiCard);event.dataTransfer.effectAllowed='move';event.dataTransfer.setData('text/plain',String(ui.dragging));requestAnimationFrame(()=>card.classList.add('is-dragging'));});
document.addEventListener('dragover',event=>{
  const zone=event.target.closest('[data-ui-drop]');if(!zone||ui.dragging===null)return;
  event.preventDefault();event.dataTransfer.dropEffect='move';
  document.querySelectorAll('.drag-over').forEach(e=>{if(e!==zone)e.classList.remove('drag-over');});zone.classList.add('drag-over');
  const cards=[...zone.querySelectorAll('[data-ui-card]')].filter(e=>Number(e.dataset.uiCard)!==ui.dragging);
  const before=cards.find(e=>event.clientY<e.getBoundingClientRect().top+e.getBoundingClientRect().height/2);
  ui.before=before?Number(before.dataset.uiCard):null;
  let marker=$('.ui-drop-marker');if(!marker){marker=document.createElement('div');marker.className='ui-drop-marker';}
  if(before)zone.insertBefore(marker,before);else zone.append(marker);
  const board=$('.ui-kanban');if(board){const r=board.getBoundingClientRect();if(event.clientX>r.right-50)board.scrollLeft+=14;if(event.clientX<r.left+50)board.scrollLeft-=14;}
});
document.addEventListener('drop',event=>{const zone=event.target.closest('[data-ui-drop]');if(!zone)return;event.preventDefault();const id=ui.dragging??Number(event.dataTransfer.getData('text/plain'));const before=ui.before;ui.dragging=null;ui.before=null;ui.suppressClickUntil=Date.now()+200;clearDrag();moveTask(id,zone.dataset.uiDrop,before);});
document.addEventListener('dragend',()=>{ui.dragging=null;ui.before=null;ui.suppressClickUntil=Date.now()+200;clearDrag();});
document.addEventListener('pointerdown',event=>{if(!event.target.closest('.ui-panel-resizer'))return;event.preventDefault();const right=$('.ui-body').getBoundingClientRect().right;document.body.classList.add('ui-resizing');const move=e=>{ui.panelWidth=Math.max(340,Math.min(560,right-e.clientX));applyAppearance();};const end=()=>{document.body.classList.remove('ui-resizing');document.removeEventListener('pointermove',move);document.removeEventListener('pointerup',end);};document.addEventListener('pointermove',move);document.addEventListener('pointerup',end);});
window.addEventListener('online',()=>{ui.online=true;render();});window.addEventListener('offline',()=>{ui.online=false;render();});
matchMedia('(prefers-color-scheme: dark)').addEventListener('change',()=>{if(ui.theme==='system')applyAppearance();});
window.SpaceRefined={state:ui,render,view,openTask,moveTask};
render();

// Shareable initial states for review artifacts.
if(query.get('dialog')==='qr'){bot().connected=false;render();qrDialog();}
if(query.get('dialog')==='command')commandDialog(0);
if(query.get('dialog')==='status')document.querySelector('[data-ui-combo=ui-task-status]')?.click();
