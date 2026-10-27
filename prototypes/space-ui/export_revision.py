"""Portable exports for revision draft 03. Originals A–E remain unchanged.

The browser preview contains the complete interactions. Canvas is a visual
snapshot with small navigation interactions. Production uses the native MyGo
foundation; this disposable HTML is a design reference, not production code.
"""
from pathlib import Path
from urllib.parse import urlencode
import base64, json, os, re, subprocess, sys, time, zipfile

ROOT=Path(__file__).resolve().parent
ART=ROOT/'artifacts/refined-03'; DOWNLOAD=ROOT/'frontend/downloads/refined-03'; PREVIEW=ROOT/'frontend/previews/refined-03'
for p in [ART,DOWNLOAD,PREVIEW]:p.mkdir(parents=True,exist_ok=True)
SCREENS=[
 ('01','Beranda','home','expanded',{},'Navigasi tetap, logo divisi, dan akses ringkas ke pekerjaan bersama.'),
 ('02','Detail','board','collapsed',{'task':'1','dialog':'status'},'Panel kanan, dropdown status buatan Space, serta board yang tetap aktif.'),
 ('03','WhatsApp','settings','expanded',{'settings':'whatsapp'},'Koneksi, identitas pesan, dan grup tujuan bot divisi.'),
 ('04','Hubungkan','settings','expanded',{'settings':'whatsapp','dialog':'qr'},'Alur menghubungkan akun melalui QR dan perangkat tertaut.'),
 ('05','Perintah','settings','collapsed',{'settings':'commands','dialog':'command'},'Perintah kustom dan pilihan stiker, dengan perlindungan perintah inti.'),
 ('06','Terang','settings','expanded',{'settings':'general','theme':'light','accent':'sage'},'Logo divisi dapat disesuaikan; seluruh kontrol mengikuti tema terang.'),
]
CANVAS='a9bd699c07c9'
css=(ROOT/'frontend/styles.css').read_text()+'\n'+(ROOT/'frontend/refined.css').read_text()+'\n'+(ROOT/'frontend/refined-revision.css').read_text()
kit=re.sub(r'^export \{[^\n]+\};\s*$', '', (ROOT/'frontend/ui-kit.js').read_text(), flags=re.M)
js=re.sub(r'^import \{[^\n]+\} from [^\n]+;\s*$', '', (ROOT/'frontend/refined.js').read_text(), flags=re.M)
logo='data:image/svg+xml;base64,'+base64.b64encode((ROOT/'frontend/space.svg').read_bytes()).decode()
kit=kit.replace('src="./space.svg"',f'src="{logo}"')
assets={name:'data:image/png;base64,'+base64.b64encode((ROOT/f'frontend/assets/{name}.png').read_bytes()).decode() for name in ['mediatek','humas']}
for asset_name,data in assets.items():js=js.replace(f'./assets/{asset_name}.png',data)
entries=[]
for n,name,view,sidebar,extras,desc in SCREENS:
 params=dict(view=view,sidebar=sidebar,theme='dark',accent='sand',**{})
 params.update(extras)
 defaults='const query = new URLSearchParams(location.search);\nfor(const [k,v] of Object.entries('+json.dumps(params)+'))if(!query.has(k))query.set(k,v);'
 script=kit+'\n'+js.replace('const query = new URLSearchParams(location.search);',defaults)
 html=f'''<!doctype html><html lang="id"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="color-scheme" content="dark light"><title>Space — Draft 03 · {name}</title><link rel="icon" href="{logo}"><style>{css}</style></head><body><div id="refined-app"></div><div id="refined-popovers"></div><div id="refined-overlay"></div><div id="refined-toast" role="status" aria-live="polite"></div><script type="module">{script}</script></body></html>'''
 filename=f'Space-03-{n}-{name}.html'
 (ART/filename).write_text(html);(DOWNLOAD/filename).write_text(html)
 entries.append(dict(number=int(n),name=name,description=desc,query=params,html=filename,preview_url='http://localhost:5173/refined.html?'+urlencode(params),download_url='http://localhost:5173/downloads/refined-03/'+filename,**({'canvas_url' if '--canvas' in sys.argv else 'planned_canvas_url':f'canvas://{CANVAS}/design-{int(n)}'})))
manifest=dict(canvas_id=CANVAS,canvas_status='prepared',status='Disposable UI prototype; fixtures; edits reset on reload',gallery_url='http://localhost:5173/refined-gallery.html',screens=entries)
(ART/'manifest.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+'\n')
with zipfile.ZipFile(ROOT/'frontend/downloads/Space-Draft-03.zip','w',zipfile.ZIP_DEFLATED) as bundle:
 for e in entries:bundle.write(ART/e['html'],e['html'])
print('Exported six draft 03 interactive HTML artifacts.',flush=True)

if '--capture' not in sys.argv:sys.exit()
sys.path.insert(0,'/tmp')
from importlib.machinery import SourceFileLoader
CDP=SourceFileLoader('space_cdp','/tmp/space-cdp.py').load_module().CDP
browser=CDP()
browser.call('Network.enable');browser.call('Network.setCacheDisabled',cacheDisabled=True)
browser.call('Emulation.setDeviceMetricsOverride',width=1440,height=1000,deviceScaleFactor=2,mobile=False)
exe=os.environ['ENOWX_BIN']
def call(*args,body=None):
 return subprocess.run([exe,*args],input=body,text=True,capture_output=True,check=True,timeout=40).stdout.strip()
frames=call('canvas','designs',CANVAS).count('canvas://') if '--canvas' in sys.argv else 0
for e in entries:
 if '--frame=6' in sys.argv and e['number']!=6:continue
 browser.call('Page.navigate',url='about:blank')
 time.sleep(.12)
 browser.call('Page.navigate',url=e['preview_url'])
 for attempt in range(50):
  try:
   if browser.evaluate('!!window.SpaceRefined'):break
  except Exception:pass
  time.sleep(.1)
 time.sleep(.45)
 browser.evaluate('Promise.all([...document.images].map(i=>i.decode().catch(()=>{})))')
 capture=browser.evaluate('JSON.stringify({html:document.getElementById("refined-app").innerHTML,overlay:document.getElementById("refined-overlay").innerHTML,popovers:document.getElementById("refined-popovers").innerHTML,theme:document.documentElement.dataset.uiTheme,density:document.documentElement.dataset.uiDensity,style:document.documentElement.getAttribute("style"),width:innerWidth,documentWidth:document.documentElement.scrollWidth})')
 capture=json.loads(capture)
 if capture['width']!=capture['documentWidth']:raise RuntimeError(f'Page overflow: {capture}')
 n=e['number'];name=e['name'];image=ART/f'Space-03-{n:02d}-{name}.png'
 shot=browser.call('Page.captureScreenshot',format='png',captureBeyondViewport=False)
 image.write_bytes(base64.b64decode(shot['data']));(PREVIEW/f'{n:02d}.png').write_bytes(image.read_bytes())
 if '--canvas' in sys.argv and n>frames:
  call('canvas','new-design',CANVAS,'--title',name,'--description',e['description'],'--page',name,'--variant','Draft 03','--size','1440x1000')
 if '--canvas' in sys.argv:call('canvas','design',CANVAS,str(n),'--title',f'{n:02d} — {name}','--description',e['description'],'--page',name,'--variant','Draft 03')
 if '--canvas' in sys.argv:call('canvas','grid',CANVAS,'--frame',str(n),'--cols','12','--rows','10','--gap','24','--margin','32')
 # Keep Canvas interactions small. The standalone artifacts carry full drag/drop.
 small='''<script>const r=document.documentElement;r.dataset.uiTheme=THEME;r.dataset.uiDensity=DENSITY;r.setAttribute('style',STYLE);document.addEventListener('click',e=>{const b=e.target.closest('button,[data-ui-card]');if(!b)return;if(b.dataset.uiAction==='sidebar'){document.querySelector('.ui-app').classList.toggle('sidebar-collapsed');document.querySelector('.ui-sidebar').classList.toggle('is-collapsed');return;}if(b.dataset.uiAction==='close-modal'){const o=document.getElementById('refined-overlay');if(o)o.innerHTML='';return;}if(b.dataset.uiChoice){const el=document.querySelector('[data-ui-combo="'+b.dataset.uiChoice+'"] .ui-combo-label');if(el)el.textContent=b.dataset.uiValue;document.getElementById('refined-popovers').innerHTML='';return;}let n=({home:1,board:2,settings:3})[b.dataset.uiView];if(b.dataset.uiTask||b.dataset.uiCard)n=2;if(b.dataset.uiAction==='wa-show-qr'||b.dataset.uiAction==='wa-reconnect')n=4;if(b.dataset.uiSettingTab==='whatsapp')n=3;if(b.dataset.uiSettingTab==='commands')n=5;if(b.dataset.uiSettingTab==='general')n=6;if(n)location.hash='design-'+n;});</script>''' 
 small=small.replace('THEME',json.dumps(capture['theme'])).replace('DENSITY',json.dumps(capture['density'])).replace('STYLE',json.dumps(capture['style']))
 content='<div id="refined-app">'+capture['html']+'</div><div id="refined-popovers">'+capture['popovers']+'</div><div id="refined-overlay">'+capture['overlay']+'</div>'
 content=content.replace('src="./space.svg"',f'src="{logo}"')
 for asset_name,data in assets.items():content=content.replace(f'./assets/{asset_name}.png',data)
 if '--canvas' in sys.argv:call('canvas','write',CANVAS,'--frame',str(n),'--html','-',body=f'<style>{css}</style><div id="draft-two-screen"></div>')
 if '--canvas' in sys.argv:call('canvas','patch',CANVAS,'--frame',str(n),'--element','draft-two-screen','--html','-',body=f'<div id="draft-two-screen">{content}</div>{small}')
 (ART/f'Canvas-{n:02d}.html').write_text(f'<style>{css}</style>{content}{small}')
 print(f'Captured {n}: {name}',flush=True)
if '--canvas' in sys.argv:
 call('canvas','goto',CANVAS,'1')
 manifest['canvas_status']='published'
 (ART/'manifest.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+'\n')
