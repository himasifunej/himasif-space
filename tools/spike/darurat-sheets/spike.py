#!/usr/bin/env python3
"""Spike: how a Google Sheet "published to the web" behaves for Space's
emergency mode. Not app code. Python 3.9+, standard library only.

  python3 spike.py --pub 'https://docs.google.com/spreadsheets/d/e/2PACX-.../pub?output=xlsx' \
      --log run.jsonl --duration 3600
  python3 spike.py --summarize run.jsonl

Each cycle (every --interval seconds plus up to --jitter seconds):
  - fetches the whole document as xlsx: status, content type, headers,
    whether it parses, its tabs, hidden/locked ID columns, validations;
  - reads the Meta tab's sheet clock (=NOW()) and compares it with now: the lag;
  - tries a conditional GET when Google sends an ETag/Last-Modified;
  - fetches every tab as CSV by gid (gids read from pubhtml, or --gids);
  - logs what changed since the last cycle, and the delay of an owner's edit
    when the edited cell holds its time ("edit 14:05" or "edit 14:05:30").
429, 5xx and HTML responses are logged and back the interval off (max 15 min).
"""
import argparse
import csv
import datetime as dt
import hashlib
import io
import json
import random
import re
import statistics
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
import zipfile
import xml.etree.ElementTree as ET
from zoneinfo import ZoneInfo

NS = {"m": "http://schemas.openxmlformats.org/spreadsheetml/2006/main",
      "r": "http://schemas.openxmlformats.org/officeDocument/2006/relationships",
      "pr": "http://schemas.openxmlformats.org/package/2006/relationships"}
ID_COLUMNS = {"Tugas": ["space_row_id", "task_id", "task_version", "base_progress"],
              "Rapat": ["meeting_id", "meeting_version"],
              "Reminder": ["reminder_id", "base_at"]}
UA = "Mozilla/5.0 (X11; Linux x86_64) SpaceDaruratSpike/1"
KEEP_HEADERS = ["content-type", "content-length", "cache-control", "expires", "last-modified",
                "etag", "age", "date", "x-frame-options", "retry-after"]
EDIT_MARK = re.compile(r"edit\s*(\d{1,2})[:.](\d{2})(?:[:.](\d{2}))?", re.I)


def fetch(url, headers=None, timeout=60):
    req = urllib.request.Request(url, headers={"User-Agent": UA, **(headers or {})})
    start = time.monotonic()
    out = {"url": url}
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            body = resp.read()
            out.update(status=resp.status, final_host=urllib.parse.urlparse(resp.geturl()).hostname,
                       headers={k: v for k, v in resp.headers.items() if k.lower() in KEEP_HEADERS})
    except urllib.error.HTTPError as e:
        body = e.read() or b""
        out.update(status=e.code, headers={k: v for k, v in e.headers.items() if k.lower() in KEEP_HEADERS})
    except Exception as e:  # network errors are data for this spike
        body = b""
        out.update(status=0, error=f"{type(e).__name__}: {e}")
    out["ms"] = round((time.monotonic() - start) * 1000)
    out["bytes"] = len(body)
    out["is_html"] = body.lstrip()[:15].lower().startswith((b"<!doctype html", b"<html"))
    return out, body


def col_index(ref):
    n = 0
    for ch in re.match(r"[A-Z]+", ref).group(0):
        n = n * 26 + ord(ch) - 64
    return n


def parse_xlsx(body):
    """Tabs as {name: {"rows": [[...]], "hidden_cols": [...], "protected": bool, "validations": n}}."""
    z = zipfile.ZipFile(io.BytesIO(body))
    shared = []
    if "xl/sharedStrings.xml" in z.namelist():
        for si in ET.fromstring(z.read("xl/sharedStrings.xml")).findall("m:si", NS):
            shared.append("".join(t.text or "" for t in si.iter("{%s}t" % NS["m"])))
    rels = {r.get("Id"): r.get("Target") for r in ET.fromstring(z.read("xl/_rels/workbook.xml.rels")).findall("pr:Relationship", NS)}
    tabs = {}
    for s in ET.fromstring(z.read("xl/workbook.xml")).find("m:sheets", NS):
        target = rels[s.get("{%s}id" % NS["r"])].lstrip("/")
        path = target if target.startswith("xl/") else "xl/" + target
        root = ET.fromstring(z.read(path))
        hidden = []
        for c in root.iter("{%s}col" % NS["m"]):
            if c.get("hidden") in ("1", "true"):
                hidden.extend(range(int(c.get("min")), int(c.get("max")) + 1))
        rows = []
        for row in root.iter("{%s}row" % NS["m"]):
            cells = {}
            for c in row.findall("m:c", NS):
                t, v = c.get("t"), c.find("m:v", NS)
                if t == "s" and v is not None:
                    val = shared[int(v.text)]
                elif t == "inlineStr":
                    val = "".join(x.text or "" for x in c.iter("{%s}t" % NS["m"]))
                else:
                    val = v.text if v is not None else ""
                cells[col_index(c.get("r"))] = val
            if any(v not in (None, "") for v in cells.values()):
                width = max(cells)
                rows.append([cells.get(i, "") for i in range(1, width + 1)])
        dvs = root.find("m:dataValidations", NS)
        tabs[s.get("name")] = {"rows": rows, "hidden_cols": hidden,
                               "protected": root.find("m:sheetProtection", NS) is not None,
                               "validations": len(dvs) if dvs is not None else 0}
    return tabs


def sheet_clock(value, tz):
    """Meta!B (=NOW()) as an aware datetime: an Excel serial, or text."""
    if value in (None, ""):
        return None
    try:
        serial = float(value)
        return (dt.datetime(1899, 12, 30) + dt.timedelta(days=serial)).replace(tzinfo=tz)
    except ValueError:
        pass
    for fmt in ("%Y-%m-%d %H:%M:%S", "%d/%m/%Y %H:%M:%S", "%d/%m/%Y %H.%M.%S", "%m/%d/%Y %H:%M:%S", "%d-%m-%Y %H:%M:%S"):
        try:
            return dt.datetime.strptime(value.strip(), fmt).replace(tzinfo=tz)
        except ValueError:
            continue
    return None


def meta_clock(rows, tz):
    for r in rows:
        if r and r[0] in ("jam_sheet", "jam_sheet_teks") and len(r) > 1:
            c = sheet_clock(r[1], tz)
            if c:
                return c, r[1]
    return None, None


def edit_delay(value, fetched, tz):
    m = EDIT_MARK.search(value or "")
    if not m:
        return None
    h, mi, s = int(m.group(1)), int(m.group(2)), int(m.group(3) or 0)
    local = fetched.astimezone(tz)
    edited = local.replace(hour=h, minute=mi, second=s, microsecond=0)
    if edited > local:
        edited -= dt.timedelta(days=1)
    return round((local - edited).total_seconds())


def find_gids(base):
    info, body = fetch(base + "/pubhtml")
    text = body.decode("utf-8", "replace")
    gids = {}
    for pat in (r"switchToSheet\('(\d+)'\)\"?>([^<]+)<", r'id="sheet-button-(\d+)"[^>]*>\s*<a[^>]*>([^<]+)<',
                r'"gid":"?(\d+)"?,"name":"([^"]+)"'):
        for gid, name in re.findall(pat, text):
            gids.setdefault(name.strip(), gid)
    return info, gids


def diff(prev, tabs):
    changes = []
    for name, tab in tabs.items():
        old = prev.get(name, {}).get("rows", [])
        for ri, row in enumerate(tab["rows"]):
            orow = old[ri] if ri < len(old) else []
            for ci in range(max(len(row), len(orow))):
                a = orow[ci] if ci < len(orow) else ""
                b = row[ci] if ci < len(row) else ""
                if a != b:
                    changes.append({"tab": name, "row": ri + 1, "col": ci + 1, "old": a, "new": b})
        if len(old) > len(tab["rows"]):
            changes.append({"tab": name, "rows_removed": len(old) - len(tab["rows"])})
    return changes


def run(args):
    tz = ZoneInfo(args.tz)
    m = re.match(r"(https://docs\.google\.com/spreadsheets/d/e/[^/]+)", args.pub)
    if not m:
        sys.exit("--pub must be a published link: https://docs.google.com/spreadsheets/d/e/2PACX-.../pub?...")
    base = m.group(1)
    log = open(args.log, "a", buffering=1)

    def emit(rec):
        rec["at"] = dt.datetime.now(dt.timezone.utc).isoformat(timespec="seconds")
        log.write(json.dumps(rec, ensure_ascii=False) + "\n")
        return rec

    gids = dict(kv.split("=", 1) for kv in args.gids.split(",")) if args.gids else {}
    if not gids:
        info, gids = find_gids(base)
        emit({"kind": "pubhtml", **info, "gids": gids})
        print(f"pubhtml {info['status']} gids={gids}")
    end = time.time() + args.duration
    interval, prev, prev_clock, cycle = args.interval, {}, None, 0
    while True:
        cycle += 1
        now = dt.datetime.now(dt.timezone.utc)
        info, body = fetch(base + "/pub?output=xlsx")
        rec = {"kind": "xlsx", "cycle": cycle, **info}
        bad = info["status"] in (0, 429) or info["status"] >= 500 or info["is_html"]
        if info["status"] == 200 and body[:2] == b"PK":
            try:
                tabs = parse_xlsx(body)
                rec["parse"] = "ok"
                rec["tabs"] = {n: {"rows": len(t["rows"]), "hidden_cols": t["hidden_cols"], "protected": t["protected"],
                                   "validations": t["validations"],
                                   "id_header_present": all(c in (t["rows"][0] if t["rows"] else []) for c in ID_COLUMNS.get(n, []))}
                               for n, t in tabs.items()}
                rec["missing_tabs"] = [n for n in ["Meta", "Tugas", "Rapat", "Reminder"] if n not in tabs]
                rec["hash"] = hashlib.sha256(json.dumps({n: t["rows"] for n, t in tabs.items()}).encode()).hexdigest()[:16]
                clock, raw = meta_clock(tabs.get("Meta", {}).get("rows", []), tz)
                rec["meta_clock_raw"] = raw
                if clock:
                    rec["meta_clock"] = clock.isoformat()
                    rec["lag_s"] = round((now - clock).total_seconds())
                    rec["clock_changed"] = prev_clock is not None and clock != prev_clock
                    prev_clock = clock
                if prev:
                    ch = diff(prev, tabs)
                    data_ch = [c for c in ch if c.get("tab") != "Meta"]
                    if data_ch:
                        rec["changes"] = data_ch[:30]
                        delays = [d for c in data_ch if (d := edit_delay(c.get("new"), now, tz)) is not None]
                        if delays:
                            rec["edit_delay_s"] = min(delays)
                prev = tabs
            except Exception as e:
                rec["parse"] = f"failed: {type(e).__name__}: {e}"
        elif info["status"] == 200:
            rec["parse"] = "not xlsx"
        etag, lm = info.get("headers", {}).get("ETag"), info.get("headers", {}).get("Last-Modified")
        if info["status"] == 200 and (etag or lm):
            h = {"If-None-Match": etag} if etag else {"If-Modified-Since": lm}
            ci, _ = fetch(base + "/pub?output=xlsx", h)
            rec["conditional_status"] = ci["status"]
        emit(rec)
        line = f"{now.astimezone(tz):%H:%M:%S} #{cycle} xlsx {info['status']} {info.get('headers', {}).get('Content-Type', '')} {info['bytes']}B parse={rec.get('parse')}"
        if "lag_s" in rec:
            line += f" clock={rec['meta_clock_raw']} lag={rec['lag_s']}s{' (republished)' if rec.get('clock_changed') else ''}"
        if rec.get("changes"):
            line += f" CHANGED {len(rec['changes'])} cells"
        if "edit_delay_s" in rec:
            line += f" edit_delay={rec['edit_delay_s']}s"
        if info["is_html"]:
            line += " HTML!"
        print(line, flush=True)

        if gids and cycle % args.csv_every == 1 % args.csv_every:
            for name, gid in gids.items():
                ci, cbody = fetch(f"{base}/pub?gid={gid}&single=true&output=csv")
                crec = {"kind": "csv", "cycle": cycle, "tab": name, "gid": gid, **ci}
                if ci["status"] == 200 and not ci["is_html"]:
                    rows = list(csv.reader(io.StringIO(cbody.decode("utf-8-sig", "replace"))))
                    crec["rows"] = len(rows)
                    crec["header"] = rows[0] if rows else []
                    crec["id_header_present"] = all(c in crec["header"] for c in ID_COLUMNS.get(name, []))
                    if name == "Meta":
                        clock, raw = meta_clock(rows, tz)
                        crec["meta_clock_raw"] = raw
                        if clock:
                            crec["lag_s"] = round((dt.datetime.now(dt.timezone.utc) - clock).total_seconds())
                bad = bad or ci["status"] == 429 or ci["is_html"]
                emit(crec)
                print(f"   csv {name} gid={gid} {ci['status']} {ci.get('headers', {}).get('Content-Type', '')} rows={crec.get('rows')} ids={crec.get('id_header_present')} lag={crec.get('lag_s')}", flush=True)

        interval = min(interval * 2, 900) if bad else args.interval
        wait = interval + random.uniform(0, args.jitter)
        if time.time() + wait > end:
            break
        time.sleep(wait)
    print(f"done; log in {args.log}. Summary: python3 {sys.argv[0]} --summarize {args.log}")


def summarize(path):
    recs = [json.loads(l) for l in open(path) if l.strip()]
    x = [r for r in recs if r["kind"] == "xlsx"]
    c = [r for r in recs if r["kind"] == "csv"]
    print(f"xlsx fetches: {len(x)}; status: {dict((s, sum(1 for r in x if r['status'] == s)) for s in {r['status'] for r in x})}")
    print(f"  parsed ok: {sum(1 for r in x if r.get('parse') == 'ok')}; HTML responses: {sum(1 for r in x if r['is_html'])}; 429: {sum(1 for r in x if r['status'] == 429)}")
    if x:
        print(f"  content types: {sorted({r.get('headers', {}).get('Content-Type', '') for r in x})}")
        print(f"  cache headers (last): { {k: v for k, v in x[-1].get('headers', {}).items() if k.lower() in ('cache-control', 'expires', 'etag', 'last-modified', 'age')} }")
        print(f"  conditional GET statuses: {sorted({r.get('conditional_status') for r in x if 'conditional_status' in r})}")
        print(f"  fetch ms median: {statistics.median(r['ms'] for r in x)}")
    ok = [r for r in x if r.get("parse") == "ok"]
    if ok:
        print(f"  tabs (last): {json.dumps(ok[-1]['tabs'], ensure_ascii=False)}")
        print(f"  missing tabs (last): {ok[-1].get('missing_tabs')}")
    lags = [r["lag_s"] for r in x if "lag_s" in r]
    if lags:
        print(f"  Meta clock lag s: min {min(lags)} median {statistics.median(lags)} max {max(lags)} (n={len(lags)})")
    else:
        print("  Meta clock: not readable in xlsx (see meta_clock_raw)")
    rep = [dt.datetime.fromisoformat(r["meta_clock"]) for r in x if r.get("clock_changed")]
    if len(rep) > 1:
        gaps = [round((b - a).total_seconds()) for a, b in zip(rep, rep[1:])]
        print(f"  republish (clock moved) {len(rep)} times; gaps between clock values s: {gaps}")
    else:
        print(f"  clock moved {len(rep)} times (recalculation alone may not republish)")
    for r in x:
        if r.get("changes"):
            print(f"  change seen at {r['at']} (#{r['cycle']}): {len(r['changes'])} cells, edit_delay_s={r.get('edit_delay_s')}, e.g. {r['changes'][0]}")
    for tab in sorted({r["tab"] for r in c}):
        rs = [r for r in c if r["tab"] == tab]
        st = dict((s, sum(1 for r in rs if r['status'] == s)) for s in {r['status'] for r in rs})
        l = [r["lag_s"] for r in rs if "lag_s" in r]
        print(f"csv {tab} gid={rs[0]['gid']}: status {st}; HTML {sum(1 for r in rs if r['is_html'])}; ids in header {rs[-1].get('id_header_present')}; header {rs[-1].get('header')}" + (f"; lag s min {min(l)} max {max(l)}" if l else ""))


if __name__ == "__main__":
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--pub", help="published link (any .../d/e/2PACX-.../pub... form)")
    p.add_argument("--log", default="darurat-spike.jsonl")
    p.add_argument("--interval", type=float, default=180)
    p.add_argument("--jitter", type=float, default=60)
    p.add_argument("--duration", type=float, default=3600)
    p.add_argument("--csv-every", type=int, default=1, help="fetch per-tab CSV every N cycles")
    p.add_argument("--gids", help="Meta=0,Tugas=123,... when pubhtml can't be read")
    p.add_argument("--tz", default="Asia/Jakarta", help="the spreadsheet's time zone (File → Settings)")
    p.add_argument("--summarize", metavar="LOG")
    a = p.parse_args()
    if a.summarize:
        summarize(a.summarize)
    elif a.pub:
        run(a)
    else:
        p.error("--pub or --summarize")
