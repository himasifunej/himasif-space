// Draft 02: sidebar A, header/board B, inline detail C, calendar E.
// In-memory UI exploration; no backend or production mutations.
import { icon, esc, mark, people, avatar, avatars, tasks, pill, aceArt } from './ui-kit.js';

const query = new URLSearchParams(location.search);
const columns = ['Belum mulai', 'Dikerjakan', 'Perlu tinjauan', 'Selesai'];
const accents = { sand: '#d7c3a0', sage: '#a8c2ac', blue: '#a2bbd2', lavender: '#b6a8d0', rose: '#d2a499' };
const ui = {
  view: ['home','board','calendar','notes','personal','reminders','members'].includes(query.get('view')) ? query.get('view') : 'home',
  collapsed: query.get('sidebar') === 'collapsed' || (!query.has('sidebar') && innerWidth < 800),
  space: 'Mediatek', filter: 'Semua', search: '',
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
const $ = s => document.querySelector(s);
const iPaths = {
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
  return `<aside class="ui-sidebar ${ui.collapsed?'is-collapsed':''}">
    <div class="ui-brand-row"><button class="ui-brand" data-ui-view="home" title="Space — beranda">${mark()}<span>space<span class="ui-brand-dot">.</span><small>RUANG KERJA HIMASIF</small></span></button>${ib('panel','sidebar','Ciutkan atau buka sidebar · Ctrl B')}</div>
    <button class="ui-period" data-ui-action="period" title="Periode 2025/2026">${glyph('archive')}<span>2025 / 2026<small>Periode aktif</small></span>${glyph('down')}</button>
    <div class="ui-nav-label">PRIBADI</div>
    <button class="ui-nav ${ui.view==='personal'?'active':''}" data-ui-view="personal" title="Jobdesk saya">${glyph('task')}<span>Jobdesk saya</span><b>${ui.tasks.filter(t=>t.pj==='AJ').length}</b></button>
    <button class="ui-nav ${ui.view==='reminders'?'active':''}" data-ui-view="reminders" title="Reminder pribadi">${glyph('bell')}<span>Reminder pribadi</span></button>
    <div class="ui-nav-label ui-space-label">SPACE SAYA ${ib('plus','new-space','Tambah Space')}</div>
    <button class="ui-space ${ui.space==='Mediatek'?'selected':''}" data-ui-space="Mediatek" title="Space Mediatek"><span class="ui-space-symbol sand">M</span><span>Mediatek<small>Media dan Teknologi</small></span><i class="live-dot"></i></button>
    <div class="ui-space-links">${[['home','home','Beranda'],['board','board','Board tugas'],['calendar','calendar','Agenda'],['notes','note','Notes']].map(([view,symbol,label])=>`<button class="${ui.view===view&&ui.space==='Mediatek'?'active':''}" data-ui-view="${view}" title="${label}">${glyph(symbol)}<span>${label}</span></button>`).join('')}</div>
    <button class="ui-space ${ui.space==='Medhum'?'selected':''}" data-ui-space="Medhum" title="Space bersama Medhum"><span class="ui-space-symbol sage">${glyph('link')}</span><span>Medhum<small>Humas × Mediatek</small></span><b>${ui.tasks.filter(t=>t.space==='Medhum').length}</b></button>
    <button class="ui-nav ui-archive" data-ui-action="archive" title="Arsip periode">${glyph('archive')}<span>Arsip periode</span></button>
    <div class="ui-sidebar-bottom"><button class="ui-nav ${ui.panel==='appearance'?'active':''}" data-ui-action="appearance" title="Penampilan · Ctrl ,">${glyph('palette')}<span>Penampilan</span><i class="ui-accent-dot"></i></button><button class="ui-user" data-ui-action="profile" title="Ahimsa Jenar · Kadiv Mediatek">${avatar('AJ')}<span>Ahimsa Jenar<small>Kadiv Mediatek</small></span>${glyph('more')}</button></div>
  </aside>`;
}
function header() {
  return `<header class="ui-header"><div class="ui-header-identity">${ib('panel','sidebar',`${ui.collapsed?'Buka':'Ciutkan'} sidebar · Ctrl B`)}<div><button class="ui-space-title" data-ui-action="space-menu">${ui.space}${glyph('down')}</button><span>HIMASIF <i>·</i> 2025 / 2026</span></div></div>
    <div class="ui-header-spacer"></div><div class="ui-search-wrap"><label class="ui-search" for="ui-search">${glyph('search')}<input id="ui-search" aria-label="Cari tugas, notes, atau anggota" value="${esc(ui.search)}" placeholder="Cari tugas, notes, atau anggota…" autocomplete="off"><kbd>Ctrl K</kbd></label><div id="ui-search-results" class="ui-search-results" hidden></div></div>
    <div class="ui-header-tools">${avatars(['AJ','ZM','OA'])}${ib('bell','notifications','Notifikasi')}<button class="ui-icon-button ui-appearance-button" data-ui-action="appearance" aria-label="Ubah penampilan" title="Penampilan · Ctrl ,">${glyph('palette')}</button>${action('Buat tugas','new-task','primary','plus')}</div></header>`;
}
function tabs() {
  return `<nav class="ui-workspace-tabs" aria-label="Tampilan Space">${[['home','home','Beranda'],['board','board','Board tugas'],['calendar','calendar','Agenda'],['notes','note','Notes'],['members','users','Anggota']].map(([v,s,l])=>`<button class="${ui.view===v?'active':''}" data-ui-view="${v}" ${ui.view===v?'aria-current="page"':''}>${glyph(s)}${l}${v==='board'?`<span>${list().length}</span>`:''}</button>`).join('')}<span class="grow"></span><span class="ui-view-hint">${ui.view==='board'?'Seret kartu untuk memindahkan tugas':ui.view==='calendar'?'Deadline desain dan posting terpisah':'Ruang kita, tetap terhubung'}</span></nav>`;
}
function statusbar() {
  return `<footer class="ui-statusbar"><button class="ui-connection ${ui.online?'':'is-offline'}" data-ui-action="online" title="Klik untuk mencoba keadaan offline"><i></i>${ui.online?'Online':'Tidak ada koneksi'}</button><span class="ui-status-divider"></span><span class="ui-status-context">${glyph('grid')}${ui.space}<span>·</span>${list().length} tugas</span><span class="ui-status-divider ui-status-long"></span><span class="ui-status-sync">${glyph(ui.online?'check':'clock')}${ui.online?'Tersimpan di preview':'Di perangkat · belum tersinkron'}</span><span class="grow"></span><span class="ui-draft-label">Draft 02 · data contoh</span><button data-ui-action="focus-search" title="Fokus pencarian"><kbd>Ctrl K</kbd>Cari</button><button data-ui-action="sidebar" title="Buka/tutup sidebar"><kbd>Ctrl B</kbd>Sidebar</button><button class="ui-shortcut-help" data-ui-action="shortcuts" title="Semua shortcut">${glyph('keyboard')}</button><button class="ui-status-theme" data-ui-action="appearance" title="Penampilan">${glyph(ui.theme==='light'?'sun':ui.theme==='system'?'monitor':'moon')}</button></footer>`;
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
  return `<div class="ui-inspector-header"><span>${glyph('task')}Detail tugas</span><span class="grow"></span>${ib('chevron','previous-task','Tugas sebelumnya')}${ib('chevron','next-task','Tugas berikutnya')}${ib('close','close-panel','Tutup detail · Esc')}</div><div class="ui-task-detail"><div class="ui-detail-context">${glyph(t.space==='Medhum'?'link':'grid')}${t.space}<span>/</span>${t.category}<span class="grow"></span>${ib('more','task-options','Opsi tugas')}</div><h2>${esc(t.title)}</h2><div class="ui-detail-status"><label><span class="ui-column-dot column-${columns.indexOf(t.status)}"></span><select id="ui-task-status" aria-label="Status tugas">${columns.map(c=>`<option ${t.status===c?'selected':''}>${c}</option>`).join('')}</select>${glyph('down')}</label><span>Diperbarui di preview</span></div><dl class="ui-detail-properties"><dt>Penanggung jawab</dt><dd>${avatar(t.pj,'tiny')}<span>${people[t.pj].name}</span>${pill('PJ utama','muted')}</dd><dt>${t.category==='Desain'?'Deadline desain':'Tenggat tugas'}</dt><dd>${glyph('calendar')}<strong>${t.date} 2026</strong>${pill(t.label,t.label==='Hari ini'?'rose':'sand')}</dd>${t.posting?`<dt>Jadwal posting</dt><dd>${glyph('globe')}${t.posting}</dd><dt>PJ Humas</dt><dd>${avatar(t.humas||'NY','tiny')}${people[t.humas||'NY'].name}</dd>`:''}<dt>Space</dt><dd>${glyph('link')}${t.space==='Medhum'?'Medhum · Humas × Mediatek':'Mediatek'}</dd></dl><section class="ui-detail-description"><h3>Hasil yang diharapkan</h3><p>${esc(t.desc)}</p><div class="ui-deliverable">${glyph('check')}${t.category==='Desain'?'File desain siap digunakan':t.category==='Video'?'Video hasil dokumentasi':'Perubahan web sesuai kebutuhan'}</div><div class="ui-deliverable">${glyph('check')}Tautan hasil atau dokumentasi pengerjaan</div></section>${t.space==='Medhum'?`<button class="ui-detail-attachment" data-ui-action="attachment"><span>${glyph('link')}</span><div><strong>${t.hasEvidence?'Hasil desain — Sempro Nadya':'Konsep & caption konten'}</strong><small>${t.hasEvidence?'Google Drive · Dikirim Oktavia':'Trello · Dibagikan Naura'}</small></div>${glyph('arrow')}</button>`:''}<div class="ui-detail-buttons">${t.status==='Perlu tinjauan'&&t.hasEvidence?action('Konfirmasi sesuai tugas','confirm-task','primary','check'):action('Ping PJ','ping','primary','send')}${action('Tawarkan bantuan','help','outline','users')}</div><section class="ui-detail-discussion"><h3>Diskusi<span>${1+ui.comments.filter(c=>c.task===t.id).length}</span></h3><div class="ui-comment">${avatar(t.humas||'AJ','small')}<div><strong>${people[t.humas||'AJ'].short}<small>10.55 WIB</small></strong><p>${t.space==='Medhum'?'Konsep sudah siap ya. Kalau ada yang perlu ditanyakan, kabari di sini!':'Kalau ada kendala atau butuh bantuan, tulis di sini ya.'}</p></div></div>${ui.comments.filter(c=>c.task===t.id).map(c=>`<div class="ui-comment">${avatar('AJ','small')}<div><strong>Ahimsa<small>Baru saja</small></strong><p>${esc(c.text)}</p></div></div>`).join('')}<div class="ui-comment-input">${avatar('AJ','small')}<input id="ui-comment" placeholder="Tulis komentar…" aria-label="Komentar tugas">${ib('send','comment','Kirim komentar')}</div></section><div class="ui-detail-source">${glyph('clock')}Setiap perubahan terlihat juga di jobdesk divisi dan PJ.</div></div>`;
}
function appearancePanel() {
  return `<div class="ui-inspector-header"><span>${glyph('palette')}Penampilan</span><span class="grow"></span>${ib('close','close-panel','Tutup penampilan · Esc')}</div><div class="ui-appearance"><span class="ui-eyebrow">RUANGMU, CARAMU</span><h2>Buat terasa milikmu<span>.</span></h2><p>Atur tampilan tanpa mengubah alur kerja Space.</p><section><h3>Tema</h3><div class="ui-theme-options">${[['dark','moon','Gelap'],['light','sun','Terang'],['system','monitor','Sistem']].map(([value,symbol,label])=>`<button class="${ui.theme===value?'active':''}" data-ui-theme="${value}" aria-pressed="${ui.theme===value}"><span class="ui-theme-preview ${value}"><i></i><i></i><i></i></span><span>${glyph(symbol)}${label}</span>${ui.theme===value?glyph('check'):''}</button>`).join('')}</div></section><section><div class="ui-setting-title"><h3>Warna aksen</h3><span>${ui.accent.toUpperCase()}</span></div><p class="ui-setting-help">Sentuhan warna untuk tombol, pilihan, dan detail kecil.</p><div class="ui-accent-options">${Object.entries(accents).map(([name,value])=>`<button style="--swatch:${value}" class="${ui.accent===value?'selected':''}" data-ui-accent="${value}" title="${name}" aria-label="Aksen ${name}">${ui.accent===value?glyph('check'):''}</button>`).join('')}<label class="ui-custom-color" title="Pilih warna sendiri">${glyph('plus')}<input id="ui-custom-accent" type="color" value="${ui.accent}" aria-label="Warna aksen kustom"></label></div></section><section><h3>Kepadatan tampilan</h3><div class="ui-setting-segment">${[['comfortable','Nyaman'],['compact','Ringkas']].map(([v,l])=>`<button class="${ui.density===v?'active':''}" data-ui-density="${v}">${l}</button>`).join('')}</div><p class="ui-setting-help">Ringkas menampilkan lebih banyak tugas dalam satu layar.</p></section><section><div class="ui-toggle-row"><div><h3>Sidebar ringkas</h3><p>Jadikan navigasi rail ikon.</p></div><button class="ui-toggle ${ui.collapsed?'on':''}" data-ui-action="sidebar" aria-label="Sidebar ringkas" aria-pressed="${ui.collapsed}"></button></div><label class="ui-panel-width-label" for="ui-panel-width">Lebar panel detail<span>${ui.panelWidth} px</span></label><input id="ui-panel-width" type="range" min="340" max="560" step="10" value="${ui.panelWidth}"></section><section><div class="ui-toggle-row"><div><h3>Animasi halus</h3><p>Transisi sidebar dan panel.</p></div><button class="ui-toggle ${ui.motion?'on':''}" data-ui-action="motion" aria-label="Animasi halus" aria-pressed="${ui.motion}"></button></div><div class="ui-toggle-row"><div><h3>Tampilkan Ace</h3><p>Teman kecil di ruang kerjamu.</p></div><button class="ui-toggle ${ui.ace?'on':''}" data-ui-action="ace" aria-label="Tampilkan Ace" aria-pressed="${ui.ace}"></button></div></section><button class="ui-reset-appearance" data-ui-action="reset-appearance">Kembalikan tampilan awal</button><div class="ui-appearance-note">Preferensi pada draft ini tersimpan selama preview. Beranda, board, dan agenda mengikuti tema yang sama.</div></div>`;
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
  return ui.view==='home'?home():ui.view==='board'?board():ui.view==='calendar'?calendar():ui.view==='notes'?notesView():otherView();
}
function render() {
  const oldMain=$('#ui-main-scroll'),oldBoard=$('.ui-kanban'),oldPanel=$('.ui-inspector');
  const scrollTop=oldMain?.scrollTop||0,scrollLeft=oldBoard?.scrollLeft||0,panelScroll=oldPanel?.scrollTop||0;
  applyAppearance();syncURL();
  document.title=`Space · ${ui.view==='home'?'Beranda':ui.view==='board'?'Board tugas':ui.view==='calendar'?'Agenda':ui.view==='notes'?'Notes':'Ruang kerja'} — Draft 02`;
  $('#refined-app').innerHTML=`<div class="ui-app ${ui.collapsed?'sidebar-collapsed':''}">${sidebar()}<div class="ui-workspace">${header()}${tabs()}<div class="ui-body ${ui.panel?'has-panel':''}" data-inspector="${ui.panel||'none'}"><main id="ui-main-scroll" class="ui-main-scroll">${!ui.online?'<div class="ui-offline-banner">Tidak ada koneksi · Tersimpan di perangkat, belum tersinkron ke cloud</div>':''}${workspace()}</main><aside class="ui-inspector ${ui.panel?'is-open':''}" aria-label="${ui.panel==='appearance'?'Pengaturan penampilan':'Detail tugas'}">${ui.panel?'<div class="ui-panel-resizer" title="Seret untuk mengubah lebar panel" aria-hidden="true"></div>':''}${ui.panel==='task'?detailPanel():ui.panel==='appearance'?appearancePanel():''}</aside></div></div></div>${statusbar()}`;
  $('#ui-main-scroll').scrollTop=scrollTop;
  if($('.ui-kanban'))$('.ui-kanban').scrollLeft=scrollLeft;
  if($('.ui-inspector'))$('.ui-inspector').scrollTop=panelScroll;
  window.__SPACE_REFINED_STATE__=structuredClone(ui);
  console.info('[Space refined]',window.__SPACE_REFINED_STATE__);
}
function view(name) {ui.view=name;ui.filter='Semua';ui.search='';ui.panel=null;ui.selected=null;render();$('#ui-main-scroll').scrollTop=0;}
function openTask(id) {ui.selected=Number(id);ui.panel='task';ui.search='';render();$('.ui-inspector').scrollTop=0;}
function toast(text) {const el=$('#refined-toast');el.textContent=text;el.classList.add('visible');clearTimeout(toast.timer);toast.timer=setTimeout(()=>el.classList.remove('visible'),3400);}
function closeModal() {$('#refined-overlay').innerHTML='';}
function modal(title,body,footer='') {
  $('#refined-overlay').innerHTML=`<div class="ui-modal-backdrop" data-ui-action="close-modal"><section class="ui-modal" role="dialog" aria-modal="true" aria-label="${esc(title)}"><header><h2>${title}</h2>${ib('close','close-modal','Tutup dialog')}</header>${body}${footer?`<footer>${footer}</footer>`:''}</section></div>`;
  setTimeout(()=>$('.ui-modal input,.ui-modal textarea,.ui-modal button')?.focus(),10);
}
function newTask(status='Belum mulai') {
  ui.newTaskStatus=status;
  modal('Buat tugas',`<label class="ui-field">Judul tugas<input id="ui-new-title" placeholder="Apa yang perlu dikerjakan?"></label><label class="ui-field">Hasil yang diharapkan<textarea id="ui-new-description" placeholder="Jelaskan hasil yang akan diserahkan…"></textarea></label><div class="ui-field-grid"><label class="ui-field">Penanggung jawab<select id="ui-new-pj">${Object.entries(people).filter(([id])=>!['NY','RF'].includes(id)).map(([id,p])=>`<option value="${id}">${p.name}</option>`).join('')}</select></label><label class="ui-field">Deadline<input id="ui-new-date" type="date" value="2026-10-30"></label></div><label class="ui-field">Subdivisi<select id="ui-new-category"><option>Desain</option><option>Tekno</option><option>Video</option></select></label><p class="ui-modal-help">Tugas akan tampil di board divisi dan jobdesk PJ. Reminder bawaan: H-2, H-1, dan hari deadline.</p>`,action('Batal','close-modal','outline')+action('Bagikan tugas','save-task','primary','plus'));
}
function moveTask(id,status,before=null) {
  const task=ui.tasks.find(t=>t.id===Number(id));if(!task||!columns.includes(status))return;
  const old=task.status;task.status=status;
  if(status==='Selesai')task.progress=100;else if(old==='Selesai')task.progress=0;
  ui.tasks=ui.tasks.filter(t=>t.id!==task.id);
  const index=before?ui.tasks.findIndex(t=>t.id===Number(before)):-1;
  if(index>=0)ui.tasks.splice(index,0,task);else ui.tasks.push(task);
  ui.lastChange=`${task.title}: ${status}`;
  render();toast(old===status?'Urutan tugas diperbarui di preview.':`Dipindahkan ke ${status.toLowerCase()}.`);
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
const actions={
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
  'period':()=>modal('Periode kepengurusan',`<div class="ui-period-choice"><strong>2025 / 2026</strong>${pill('Aktif','sage')}</div><div class="ui-period-choice"><span>2024 / 2025<small>Arsip · hanya-baca sesuai izin</small></span>${glyph('archive')}</div>`),
  'space-menu':()=>modal('Pilih Space',`<div class="ui-move-options"><button data-ui-space="Mediatek">${glyph('grid')}Mediatek<span class="grow"></span>Media dan Teknologi</button><button data-ui-space="Medhum">${glyph('link')}Medhum<span class="grow"></span>Humas × Mediatek</button></div>`),
  'notifications':()=>modal('Notifikasi',`<div class="ui-move-options"><button data-ui-task="5">${glyph('send')}Oktavia mengirim hasil Sempro — Nadya${glyph('chevron')}</button><button data-ui-action="meeting">${glyph('calendar')}Rapat divisi malam ini, 19.00 WIB${glyph('chevron')}</button></div>`),
  'meeting':()=>modal('Rapat divisi Mediatek',`<div class="ui-meeting-info">${pill('Malam ini','sage')}<h3>Evaluasi Oktober & rencana November</h3><p>${glyph('calendar')}Selasa, 27 Oktober 2026</p><p>${glyph('clock')}19.00 – 20.30 WIB</p><p>${glyph('globe')}Google Meet</p>${avatars()}<small>8 peserta · Reminder H-1, 1 jam, 15 menit</small></div>`,action('Buka notulensi','meeting-notes','outline','note')+action('Ping peserta','ping-meeting','primary','send')),
  'meeting-notes':()=>{closeModal();view('notes');},
  'shortcuts':()=>modal('Shortcut Space',`<div class="ui-shortcut-list">${[['Ctrl / ⌘ K','Fokus pencarian'],['Ctrl / ⌘ B','Buka / ciutkan sidebar'],['Ctrl / ⌘ ,','Pengaturan penampilan'],['Esc','Tutup detail atau dialog'],['Enter','Buka kartu yang difokuskan'],['Alt 1 / 2 / 3','Beranda / board / agenda']].map(([key,label])=>`<div><span>${label}</span><kbd>${key}</kbd></div>`).join('')}</div>`),
  'new-note':()=>{view('notes');toast('Pilih template dan edit judul/isi catatan pada preview.');},
  'personal-reminder':()=>modal('Reminder pribadi',`<label class="ui-field">Ingatkan tentang<input placeholder="Contoh: cek referensi desain"></label><div class="ui-field-grid"><label class="ui-field">Tanggal<input type="date" value="2026-10-27"></label><label class="ui-field">Jam<input type="time" value="20:30"></label></div><p class="ui-modal-help">${glyph('lock')}Privat secara default.</p>`,action('Simpan reminder','save-reminder','primary','bell')),
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
  if(!el)return;
  if(el.classList.contains('ui-modal-backdrop')&&event.target!==el)return;
  if(el.dataset.uiView){view(el.dataset.uiView);return;}
  if(el.dataset.uiSpace){ui.space=el.dataset.uiSpace;closeModal();view('home');return;}
  if(el.dataset.uiTask){closeModal();openTask(el.dataset.uiTask);return;}
  if(el.dataset.uiMoveMenu){moveMenu(el.dataset.uiMoveMenu);return;}
  if(el.dataset.uiMoveStatus){closeModal();moveTask(el.dataset.uiMoveId,el.dataset.uiMoveStatus);return;}
  if(el.dataset.uiNewColumn){newTask(el.dataset.uiNewColumn);return;}
  if(el.dataset.uiCard){if(Date.now()>ui.suppressClickUntil)openTask(el.dataset.uiCard);return;}
  if(el.dataset.uiTheme){ui.theme=el.dataset.uiTheme;render();return;}
  if(el.dataset.uiAccent){ui.accent=el.dataset.uiAccent;render();return;}
  if(el.dataset.uiDensity){ui.density=el.dataset.uiDensity;render();return;}
  if(el.dataset.uiFilter){ui.filter=el.dataset.uiFilter;render();return;}
  if(el.dataset.uiAction)actions[el.dataset.uiAction]?.();
});
document.addEventListener('focusin',event=>{if(event.target.id==='ui-search')searchResults();});
document.addEventListener('input',event=>{
  if(event.target.id==='ui-search'){ui.search=event.target.value;searchResults();}
  if(event.target.id==='ui-custom-accent'){ui.accent=event.target.value;applyAppearance();$('.ui-setting-title span').textContent=ui.accent.toUpperCase();}
  if(event.target.id==='ui-panel-width'){ui.panelWidth=Number(event.target.value);applyAppearance();$('.ui-panel-width-label span').textContent=`${ui.panelWidth} px`;}
});
document.addEventListener('change',event=>{if(event.target.id==='ui-task-status')moveTask(ui.selected,event.target.value);if(['ui-custom-accent','ui-panel-width'].includes(event.target.id))render();});
document.addEventListener('keydown',event=>{
  const editing=event.target.closest('input,textarea,select,[contenteditable="true"]');
  if((event.ctrlKey||event.metaKey)&&['k','b',','].includes(event.key.toLowerCase())){event.preventDefault();actions[event.key.toLowerCase()==='k'?'focus-search':event.key.toLowerCase()==='b'?'sidebar':'appearance']();return;}
  if(event.altKey&&['1','2','3'].includes(event.key)){event.preventDefault();view(['home','board','calendar'][Number(event.key)-1]);return;}
  if(event.key==='Escape'){if($('.ui-modal'))closeModal();else if($('#ui-search-results')&&!$('#ui-search-results').hidden){$('#ui-search-results').hidden=true;$('#ui-search').blur();}else actions['close-panel']();return;}
  if(event.key==='Enter'&&event.target.id==='ui-search'){const first=$('#ui-search-results [data-ui-task]');if(first){event.preventDefault();openTask(first.dataset.uiTask);}return;}
  if(event.key==='Enter'&&event.target.id==='ui-comment'){event.preventDefault();actions.comment();return;}
  if(!editing&&['Enter',' '].includes(event.key)&&event.target.matches('[data-ui-card]')){event.preventDefault();openTask(event.target.dataset.uiCard);}
  if(event.key==='?'&&!editing){event.preventDefault();actions.shortcuts();}
  if(event.key==='Tab'&&$('.ui-modal')){const focusable=[...$('.ui-modal').querySelectorAll('button,input,textarea,select')].filter(e=>!e.disabled);const first=focusable[0],last=focusable.at(-1);if(event.shiftKey&&document.activeElement===first){event.preventDefault();last.focus();}else if(!event.shiftKey&&document.activeElement===last){event.preventDefault();first.focus();}}
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
