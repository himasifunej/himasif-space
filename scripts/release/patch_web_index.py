#!/usr/bin/env python3
"""Harden the page that `mygo build -platform web` writes.

MyGo generates index.html without a hook to customize it. The WebAssembly is
large, so a dropped connection on a phone used to leave "The app did not
load: TypeError: Failed to fetch". This retries the download a few times,
shows progress text in Indonesian, and lets the page fit phones with notches.

Usage: patch_web_index.py app/build/web/index.html
"""
import re
import sys

LOADER = '''      const go = new Go()
      const space_wasm = %(wasm)s
      const space_status = (text) => {
        let p = document.querySelector("p.space-load")
        if (!p) { p = document.createElement("p"); p.className = "space-load"; p.style.cssText = "position:fixed;left:0;right:0;bottom:24px;text-align:center;font:13px system-ui;color:#a39c8e"; document.body.appendChild(p) }
        p.textContent = text
      }
      const space_load = async () => {
        for (let attempt = 1; ; attempt++) {
          try {
            return await WebAssembly.instantiateStreaming(fetch(space_wasm), go.importObject)
          } catch (e) {
            if (attempt >= 4) throw e
            space_status("Koneksi terputus, mencoba lagi (" + attempt + "/3)…")
            await new Promise((r) => setTimeout(r, 1500 * attempt))
          }
        }
      }
      space_load()
        .then((r) => { document.querySelector("p.space-load")?.remove(); go.run(r.instance) })
        .catch((e) => {
          const p = document.createElement("p")
          p.className = "error"
          p.textContent = "Space belum dapat dimuat. Periksa koneksi lalu muat ulang halaman. (" + e + ")"
          document.body.replaceChildren(p)
        })
'''


def main(path):
    html = open(path, encoding="utf-8").read()
    if "space_load" in html:
        return
    m = re.search(r'      const go = new Go\(\)\n      WebAssembly\.instantiateStreaming\(fetch\(("[^"]+\.wasm")\), go\.importObject\)\n(?:.*\n)*?        \}\)\n', html)
    if not m:
        sys.exit("patch_web_index: loader not found; MyGo changed its page")
    html = html[:m.start()] + LOADER % {"wasm": m.group(1)} + html[m.end():]
    html = html.replace('content="width=device-width, initial-scale=1"', 'content="width=device-width, initial-scale=1, viewport-fit=cover, interactive-widget=resizes-content"')
    html = html.replace("<title>", '<meta name="theme-color" content="#141310">\n    <title>', 1)
    open(path, "w", encoding="utf-8").write(html)


if __name__ == "__main__":
    main(sys.argv[1])
