#!/usr/bin/env python3
"""Builds the emergency Google Sheets template (spike only, not app code).

Tabs and columns follow the note "Space — Darurat via Google Sheets".
Identity columns are hidden and locked; every row is [DUMMY] data.
Needs openpyxl: python3 -m pip install openpyxl
"""
import sys
import uuid

from openpyxl import Workbook
from openpyxl.styles import Font, PatternFill, Protection
from openpyxl.worksheet.datavalidation import DataValidation

OUT = sys.argv[1] if len(sys.argv) > 1 else "space-darurat-template.xlsx"
TEMPLATE_VERSION = "1"

ID_FILL = PatternFill("solid", fgColor="D9D9D9")
HEAD = Font(bold=True)
UNLOCKED = Protection(locked=False)


def sheet(wb, title, ids, visible, rows, enums):
    """ids: hidden locked columns; visible: editable/display columns."""
    ws = wb.create_sheet(title)
    header = ids + visible
    ws.append(header)
    for row in rows:
        ws.append(row)
    for col, name in enumerate(header, start=1):
        letter = ws.cell(row=1, column=col).column_letter
        ws.cell(row=1, column=col).font = HEAD
        if col <= len(ids):
            ws.column_dimensions[letter].hidden = True
            for r in range(1, len(rows) + 2):
                ws.cell(row=r, column=col).fill = ID_FILL
        else:
            ws.column_dimensions[letter].width = max(14, len(name) + 4)
            # Everything visible stays editable, including blank rows below.
            for r in range(2, 201):
                c = ws.cell(row=r, column=col)
                c.protection = UNLOCKED
                c.number_format = "@"  # plain text: no locale date parsing
        if name in enums:
            dv = DataValidation(type="list", formula1='"%s"' % ",".join(enums[name]), allow_blank=True)
            dv.error = "Pilih dari daftar"
            dv.add(f"{letter}2:{letter}200")
            ws.add_data_validation(dv)
    ws.freeze_panes = ws.cell(row=2, column=len(ids) + 1)
    ws.protection.sheet = True
    ws.protection.formatColumns = False  # allow resizing
    return ws


def uid():
    return str(uuid.uuid4())


wb = Workbook()
meta = wb.active
meta.title = "Meta"
for row in [
    ("template_version", TEMPLATE_VERSION),
    ("jam_sheet", "=NOW()"),
    ("jam_sheet_teks", '=TEXT(NOW(),"yyyy-mm-dd hh:mm:ss")'),
    ("zona_waktu", "Asia/Jakarta (atur di File → Setelan)"),
    ("petunjuk", "Jangan ubah tab ini, nama tab atau kolom abu-abu. Isi 'Dilaporkan oleh' untuk tiap perubahan."),
    ("data", "[DUMMY] Semua baris adalah data contoh untuk spike."),
]:
    meta.append(row)
meta.column_dimensions["A"].width = 20
meta.column_dimensions["B"].width = 70
meta["A1"].font = HEAD
meta.protection.sheet = True

tasks = [
    ("[DUMMY] Desain poster acara", "Anggota A", "Divisi 1", "2026-11-20", "todo"),
    ("[DUMMY] Rekap kas bulanan", "Anggota B", "Divisi 2", "2026-11-18", "doing"),
    ("[DUMMY] Proposal sponsor", "Anggota C", "Divisi 1", "2026-11-25", "todo"),
    ("[DUMMY] Daftar hadir rapat", "Anggota D", "Divisi 3", "2026-11-16", "doing"),
    ("[DUMMY] Caption unggahan", "Anggota E", "Divisi 2", "2026-11-17T19:00:00+07:00", "todo"),
    ("[DUMMY] Laporan kegiatan", "Anggota F", "Divisi 3", "2026-11-30", "todo"),
]
sheet(
    wb,
    "Tugas",
    ["space_row_id", "task_id", "task_version", "base_progress"],
    ["Judul", "PJ", "Divisi", "Deadline", "Progres", "Dilaporkan oleh", "Waktu laporan", "Catatan"],
    [[uid(), uid(), 3, p, t, pj, d, dl, p, "", "", ""] for t, pj, d, dl, p in tasks],
    {"Progres": ["todo", "doing", "done"]},
)
sheet(
    wb,
    "Rapat",
    ["meeting_id", "meeting_version"],
    ["Judul", "Mulai", "Selesai", "Status", "Catatan"],
    [
        [uid(), 1, "[DUMMY] Rapat koordinasi", "2026-11-19T15:00:00+07:00", "2026-11-19T16:00:00+07:00", "jadi", ""],
        [uid(), 2, "[DUMMY] Rapat evaluasi", "2026-11-21T19:30:00+07:00", "2026-11-21T21:00:00+07:00", "jadi", ""],
    ],
    {"Status": ["jadi", "batal"]},
)
sheet(
    wb,
    "Reminder",
    ["reminder_id", "base_at"],
    ["Judul", "Waktu", "Status"],
    [
        [uid(), "2026-11-18T08:00:00+07:00", "[DUMMY] Kumpulkan laporan", "2026-11-18T08:00:00+07:00", "aktif"],
        [uid(), "2026-11-20T12:00:00+07:00", "[DUMMY] Cek publikasi", "2026-11-20T12:00:00+07:00", "aktif"],
    ],
    {"Status": ["aktif", "selesai"]},
)
wb.save(OUT)
print(OUT)
