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

# MyGo's canvas takes touches as a mouse: a finger dragging over a list
# presses the row under it and scrolls nothing. This turns a touch that moves
# past a few pixels into wheel scrolling of the container under where it
# started, locked to the axis it began along, with a fling after the finger
# lifts; the press is released off screen so nothing is clicked. A long press
# stays a press, and an element the app drags (a clock face, a resize edge)
# sets space_pan_hold while pressed so the touch keeps dragging it.
TOUCH_PAN = '''      ;(() => {
        const SLOP = 8, HOLD_MS = 450, SETTLE_MS = 40
        let t = null, fling = 0
        const wheel = (p, dx, dy) => p.el.dispatchEvent(new WheelEvent("wheel", { clientX: p.x0, clientY: p.y0, deltaX: dx, deltaY: dy, deltaMode: 0, bubbles: true, cancelable: true }))
        const stop = () => { cancelAnimationFrame(fling); fling = 0 }
        window.addEventListener("pointerdown", (e) => {
          if (!e.isTrusted || e.pointerType !== "touch" || !(e.target instanceof HTMLCanvasElement)) return
          if (t) { t.dead = true; return }
          const swallow = fling !== 0
          stop()
          window.space_pan_hold = false
          t = { id: e.pointerId, el: e.target, x0: e.clientX, y0: e.clientY, lx: e.clientX, ly: e.clientY, t0: e.timeStamp, lt: e.timeStamp, vx: 0, vy: 0, axis: "", pan: false, dead: false, swallow }
          if (swallow) e.stopImmediatePropagation()
        }, true)
        window.addEventListener("pointermove", (e) => {
          if (!e.isTrusted || !t || e.pointerId !== t.id) return
          if (t.swallow) { e.stopImmediatePropagation(); return }
          if (t.dead) return
          if (!t.pan) {
            const mx = e.clientX - t.x0, my = e.clientY - t.y0
            if (Math.hypot(mx, my) < SLOP || e.timeStamp - t.t0 < SETTLE_MS) return
            if (window.space_pan_hold || e.timeStamp - t.t0 > HOLD_MS) { t.dead = true; return }
            t.pan = true
            t.axis = Math.abs(mx) > Math.abs(my) ? "x" : "y"
            t.el.dispatchEvent(new PointerEvent("pointercancel", { pointerId: t.id, pointerType: "touch", clientX: -9999, clientY: -9999, bubbles: true }))
            t.lx = t.x0; t.ly = t.y0
          }
          e.stopImmediatePropagation()
          const dx = t.axis === "x" ? e.clientX - t.lx : 0, dy = t.axis === "y" ? e.clientY - t.ly : 0
          const dt = Math.max(1, e.timeStamp - t.lt)
          t.vx = 0.7 * (dx / dt) + 0.3 * t.vx; t.vy = 0.7 * (dy / dt) + 0.3 * t.vy
          t.lx = e.clientX; t.ly = e.clientY; t.lt = e.timeStamp
          if (dx || dy) wheel(t, -dx, -dy)
        }, true)
        const end = (e) => {
          if (!e.isTrusted || !t || e.pointerId !== t.id) return
          const was = t
          if (was.swallow || was.pan) e.stopImmediatePropagation()
          if (was.pan && e.type === "pointerup" && e.timeStamp - was.lt < 80) {
            let vx = was.vx, vy = was.vy, last = performance.now()
            const step = (now) => {
              const dt = Math.min(32, now - last); last = now
              wheel(was, -vx * dt, -vy * dt)
              const k = Math.pow(0.95, dt / 16); vx *= k; vy *= k
              fling = Math.hypot(vx, vy) > 0.02 ? requestAnimationFrame(step) : 0
            }
            if (Math.hypot(vx, vy) > 0.15) fling = requestAnimationFrame(step)
          }
          t = null
        }
        window.addEventListener("pointerup", end, true)
        window.addEventListener("pointercancel", end, true)
      })()
'''

LOADER = TOUCH_PAN + '''      const go = new Go()
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
