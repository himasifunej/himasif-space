"""Export the five alternatives as standalone HTML and enowX Canvas designs.

Run after the preview is open in enowX Browser. Never touches production data.
"""
from pathlib import Path
import base64
import json
import os
import subprocess

ROOT = Path(__file__).resolve().parent
ARTIFACTS = ROOT / "artifacts"
PREVIEWS = ROOT / "frontend/previews"
DOWNLOADS = ROOT / "frontend/downloads"
PREVIEWS.mkdir(exist_ok=True)
DOWNLOADS.mkdir(exist_ok=True)
EXE = os.environ["ENOWX_BIN"]
CANVAS = "f82f4ea8b842"
VARIANTS = [
    ("A", "Ruang", "Workspace akrab ala enowX, ringkasan divisi dan agenda"),
    ("B", "Orbit", "Navigasi ringkas dan board empat kolom untuk kerja bersama"),
    ("C", "Alur", "Daftar tugas dan detail berdampingan untuk koordinasi Medhum"),
    ("D", "Fokus", "Prioritas harian, catatan cepat dan agenda yang tenang"),
    ("E", "Ritme", "Kalender mingguan dan agenda hari ini sebagai pusat kerja"),
]


def call(*args, body=None):
    return subprocess.run(
        [EXE, *args], input=body, text=True, capture_output=True, check=True
    ).stdout.strip()


css = (ROOT / "frontend/styles.css").read_text()
js = (ROOT / "frontend/app.js").read_text()
logo = "data:image/svg+xml;base64," + base64.b64encode(
    (ROOT / "frontend/space.svg").read_bytes()
).decode()
manifest_path = ARTIFACTS / "manifest.json"
previous = json.loads(manifest_path.read_text()) if manifest_path.exists() else {}
entries = []
for i, (key, name, description) in enumerate(VARIANTS, 1):
    script = js.replace('src="./space.svg"', f'src="{logo}"')
    script = script.replace(
        "const params=new URLSearchParams(location.search);",
        f"const params=new URLSearchParams(location.search);if(!params.has('variant'))params.set('variant','{key}');",
    )
    html = f'''<!doctype html><html lang="id"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1"><title>{key} — {name} · Space</title>
<link rel="icon" href="{logo}"><style>{css}</style></head><body>
<div id="app"></div><div id="overlay-root"></div><div id="toast" role="status" aria-live="polite"></div>
<div id="prototype-switcher"></div><script type="module">{script}</script></body></html>'''
    (ARTIFACTS / f"Space-{key}-{name}.html").write_text(html)
    (DOWNLOADS / f"Space-{key}-{name}.html").write_text(html)
    captured = json.loads(call(
        "browser", "eval",
        f"(() => {{ document.querySelector('[data-variant=\"{key}\"]').click(); return JSON.stringify({{html:document.getElementById('app').innerHTML,title:document.title}}); }})()",
    ))
    content = captured["html"].replace('src="./space.svg"', f'src="{logo}"')
    if not previous and i > 1:
        call("canvas", "new-design", CANVAS, "--title", f"{key} — {name}",
             "--description", description, "--page", "Space desktop", "--variant", key,
             "--size", "1440x1000")
    call("canvas", "design", CANVAS, str(i), "--title", f"{key} — {name}",
         "--description", description, "--page", "Space desktop", "--variant", key)
    call("canvas", "grid", CANVAS, "--frame", str(i), "--cols", "12", "--rows", "10",
         "--gap", "24", "--margin", "32")
    # Canvas contains the same rendered UI, with lightweight preview interactions.
    small_script = '''<script>document.addEventListener('click',function(e){
const t=e.target.closest('[data-task],[data-select],[data-action],[data-filter]');if(!t)return;
if(t.dataset.filter){t.parentElement.querySelectorAll('button').forEach(b=>b.classList.remove('active'));t.classList.add('active');return;}
if(t.dataset.select){document.querySelectorAll('.master-item').forEach(b=>b.classList.remove('selected'));t.classList.add('selected');return;}
if(t.dataset.task||t.dataset.action==='review'||t.dataset.action==='meeting'||t.dataset.action==='ping'){
const d=document.createElement('div');d.className='modal-backdrop';d.innerHTML='<section class="modal"><div class="modal-top"><h2>Detail di Space</h2><button class="btn secondary">Tutup</button></div><p>Eksplorasi ini menampilkan struktur desktop. Untuk mencoba tugas, notes, ping dan Ace secara lengkap, buka preview interaktif A–E pada port 5173.</p></section>';d.addEventListener('click',()=>d.remove());document.body.append(d);}
});</script>'''
    canvas_css = ".prototype-banner{display:none}.shell{height:100vh;min-height:0}.work-area,.sidebar,.icon-rail{height:100vh;min-height:0}"
    call("canvas", "write", CANVAS, "--frame", str(i), "--html", "-", body=f"<style>{css}{canvas_css}</style><div id=\"ui-{key}\"></div>")
    call("canvas", "patch", CANVAS, "--frame", str(i), "--element", f"ui-{key}", "--html", "-", body=f'<div id="ui-{key}">{content}</div>{small_script}')
    image_path = ARTIFACTS / f"Space-{key}-{name}.png"
    screenshot_result = call("canvas", "screenshot", CANVAS, str(image_path), "--frame", str(i))
    if "warning:" in screenshot_result.lower():
        print(screenshot_result, flush=True)
    (PREVIEWS / f"{key}.png").write_bytes(image_path.read_bytes())
    entries.append({
        "key": key, "name": name, "description": description,
        "canvas_design": i, "canvas_url": f"canvas://{CANVAS}/design-{i}",
        "preview_url": f"http://localhost:5173/?variant={key}",
        "download_url": f"http://localhost:5173/downloads/Space-{key}-{name}.html",
        "html": f"Space-{key}-{name}.html", "screenshot": f"Space-{key}-{name}.png",
    })
    print(f"Exported {key} — {name}", flush=True)
manifest_path.write_text(json.dumps({
    "canvas_id": CANVAS, "desktop_framework": "MyGo v0.7.1",
    "gallery_url": "http://localhost:5173/gallery.html",
    "status": "UI prototype; fixture data; no production mutations",
    "artifacts": entries,
}, ensure_ascii=False, indent=2) + "\n")
call("canvas", "goto", CANVAS, "1")
call("browser", "eval", "document.querySelector('[data-variant=\"A\"]').click()")
