package appui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/emergency"
	"github.com/himasifunej/himasif-space/internal/emergency/sheet"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/workspaceui"
)

// sheetCopy is a published copy as Google serves it: Meta's clock as a
// serial day, Tugas with the template's columns.
func sheetCopy(t *testing.T, serial string, rows ...[]string) []byte {
	t.Helper()
	head := []sheet.Cell{}
	for _, h := range []string{"Judul", "PJ", "Space", "Tenggat", "Progres", "Dilaporkan oleh", "Waktu laporan", "Catatan", "task_id", "task_version", "base_progress"} {
		head = append(head, sheet.Cell{Text: h})
	}
	tab := sheet.WriteTab{Name: sheet.TabTasks, Rows: [][]sheet.Cell{head}, Hidden: []int{8, 9, 10}}
	for _, r := range rows {
		row := []sheet.Cell{}
		for _, v := range r {
			row = append(row, sheet.Cell{Text: v})
		}
		tab.Rows = append(tab.Rows, row)
	}
	b, err := sheet.WriteXLSX([]sheet.WriteTab{{Name: sheet.TabMeta, Rows: [][]sheet.Cell{{{Text: sheet.ClockKey}, {Text: serial}}}}, tab})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func sheetRowCells(tid id.ID, title, progress, base string) []string {
	return []string{title, "Anggota A", "Divisi 1", "20 Nov 2026", progress, "Raka", "13.38", "", tid.String(), "3", base}
}

// googleFake serves the queued responses in turn, as Google's caches do.
type googleFake struct {
	mu    sync.Mutex
	queue []func(http.ResponseWriter)
	hits  int
}

func (g *googleFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.hits++
	if len(g.queue) == 0 {
		w.WriteHeader(500)
		return
	}
	next := g.queue[0]
	g.queue = g.queue[1:]
	next(w)
}

func (g *googleFake) push(fns ...func(http.ResponseWriter)) {
	g.mu.Lock()
	g.queue = append(g.queue, fns...)
	g.mu.Unlock()
}

func xlsxResponse(b []byte) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Write(b)
	}
}

// sheetHarness is a signed-in desktop with the sheet linked to a fake
// Google, the server unreachable.
func sheetHarness(t *testing.T) (*harness, *googleFake, api.Space) {
	t.Helper()
	h := newChromeHarness(t, nil)
	h.tt.SetSize(1440, 900)
	s := testAuthorizedSpace(h.app, h.api.divisions[0])
	s.Capabilities = append(s.Capabilities, api.CapTasksManage)
	h.app.access.spaces[0] = s
	g := &googleFake{}
	srv := httptest.NewServer(g)
	t.Cleanup(srv.Close)
	st := h.app.sheetEnsure()
	st.http = srv.Client()
	st.cfg = sheetConfig{Publish: srv.URL + "/pub?output=xlsx", Edit: "https://docs.google.com/spreadsheets/d/1HIJxxxxxxxxxxxxxxxxxxxxxxx/edit?usp=sharing", Tested: time.Date(2026, 11, 4, 10, 12, 0, 0, time.Local), TestNote: "Terbaca: 6 baris tugas, data sheet per 10.12."}
	st.forceDown = true
	st.loc = time.FixedZone("WIB", 7*3600)
	return h, g, s
}

func TestEmergencySheetKeepsNewestCopyAndSurvivesBadResponses(t *testing.T) {
	h, g, _ := sheetHarness(t)
	n := &fakeNotifier{perm: BrowserPermissionGranted}
	h.app.Notifier, h.app.Ace = n, &fakeAce{}
	a := id.New()
	g.push(
		xlsxResponse(sheetCopy(t, "46338.36", sheetRowCells(a, "Poster", "Dikerjakan", "todo"))),
		// 28 minutes older, served next by another cache.
		xlsxResponse(sheetCopy(t, "46338.34", sheetRowCells(a, "Poster", "Belum mulai", "todo"))),
		func(w http.ResponseWriter) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<!doctype html><title>Login</title>"))
		},
		func(w http.ResponseWriter) {
			w.Header().Set("Retry-After", "1500")
			w.WriteHeader(429)
		},
	)
	st := &h.app.sheet
	fetch := func() time.Time {
		now := time.Now()
		h.app.sheetFetch(now)
		h.settle()
		return now
	}
	fetch()
	if r, ok := st.sess.Follow.Row(a); !ok || r.Progress != "doing" || !st.sess.Active {
		t.Fatalf("first copy: %+v %v", r, st.sess.Active)
	}
	if len(n.shown) != 1 || n.shown[0].Kind != NoticeEmergency || !strings.Contains(n.shown[0].Body, "1 perubahan") {
		t.Fatalf("Ace after reading: %+v", n.shown)
	}
	fetch()
	if r, _ := st.sess.Follow.Row(a); r.Progress != "doing" || st.sess.Stale != 1 {
		t.Fatalf("older copy undid the newer: %+v stale %d", r, st.sess.Stale)
	}
	fetch()
	if r, _ := st.sess.Follow.Row(a); r.Progress != "doing" || st.failures != 1 || !strings.Contains(st.lastErr, "halaman") {
		t.Fatalf("HTML wiped or not reported: %+v %d %q", r, st.failures, st.lastErr)
	}
	at := fetch()
	if r, _ := st.sess.Follow.Row(a); r.Progress != "doing" || st.failures != 2 || st.nextAt.Sub(at) < 25*time.Minute {
		t.Fatalf("429: %+v failures %d next in %v", r, st.failures, st.nextAt.Sub(at))
	}
	if len(n.shown) != 1 {
		t.Fatalf("Ace spoke about a stale or failed fetch: %+v", n.shown)
	}
}

func TestEmergencySheetLocksProgressAndShowsTheSheets(t *testing.T) {
	h, g, s := sheetHarness(t)
	a, b := id.New(), id.New()
	g.push(xlsxResponse(sheetCopy(t, "46338.36", sheetRowCells(a, "Poster", "Selesai", "doing"))))
	h.app.sheetFetch(time.Now())
	h.settle()
	perm := api.TaskPermissions{Manage: true, Progress: true}
	items := []api.Task{{ID: a, Title: "Poster", Progress: api.TaskDoing, SpaceIDs: []id.ID{s.ID}, Permissions: perm}, {ID: b, Title: "Kas", Progress: api.TaskTodo, SpaceIDs: []id.ID{s.ID}, Permissions: perm}}
	h.app.tasks.items = h.app.overlaySheetTasks(items)
	h.app.tasks.loaded = true
	if h.app.tasks.items[0].Progress != api.TaskDone || h.app.tasks.items[1].Progress != api.TaskTodo {
		t.Fatalf("overlay: %+v", h.app.tasks.items)
	}
	if !h.app.sheetLocked(a) || h.app.sheetLocked(b) {
		t.Fatal("lock follows the sheet's rows")
	}
	h.app.moveTask(workspaceui.MoveRequest{TaskID: a, Destination: workspaceui.Todo})
	if !strings.Contains(h.app.tasks.notice, "sheet darurat") || h.app.tasks.items[0].Progress != api.TaskDone {
		t.Fatalf("moved a locked task: %q %+v", h.app.tasks.notice, h.app.tasks.items[0])
	}
	h.app.tasks.notice = ""
	h.app.queueProgress(h.app.tasks.items[0], api.TaskTodo)
	if !strings.Contains(h.app.tasks.notice, "sheet darurat") {
		t.Fatal("queued a locked task's progress")
	}
	if got := h.app.cancellableTasks([]id.ID{a, b}); len(got) != 1 || got[0] != b {
		t.Fatalf("cancellable %v", got)
	}
	// Akhiri: locks go.
	h.app.sheetEnd()
	if h.app.sheetLocked(a) {
		t.Fatal("still locked after Akhiri")
	}
}

func TestEmergencySheetAceSaysOutageOnceAndRecovery(t *testing.T) {
	h, _, _ := sheetHarness(t)
	n := &fakeNotifier{perm: BrowserPermissionGranted}
	h.app.Notifier, h.app.Ace = n, &fakeAce{}
	st := &h.app.sheet
	st.nextAt = time.Now().Add(time.Hour) // no fetch in this test
	now := time.Now()
	h.app.TickEmergencySheet(now)
	h.app.TickEmergencySheet(now.Add(time.Minute))
	if len(n.shown) != 0 {
		t.Fatalf("spoke before two minutes: %+v", n.shown)
	}
	h.app.TickEmergencySheet(now.Add(2 * time.Minute))
	h.app.TickEmergencySheet(now.Add(3 * time.Minute))
	if len(n.shown) != 1 || n.shown[0].Title != "Server Space mati" || n.shown[0].OpenLabel != "Buka sheet" || n.shown[0].Link != sheetLink("open-sheet") || len(n.shown[0].Actions) != 1 || n.shown[0].Actions[0].Link != sheetLink("copy-link") {
		t.Fatalf("outage: %+v", n.shown)
	}
	if !st.sess.Active {
		t.Fatal("outage did not start the session")
	}
	open, copyText, show := h.app.EmergencyAction("copy-link")
	if copyText != st.cfg.Edit || open != "" || show || n.shown[len(n.shown)-1].Title != "Link sheet disalin" {
		t.Fatalf("copy-link: %q %q %v %+v", open, copyText, show, n.shown)
	}
	if open, _, _ := h.app.EmergencyAction("whatsapp"); !strings.HasPrefix(open, "https://wa.me/?text=") {
		t.Fatalf("whatsapp: %q", open)
	}
	if open, _, _ := h.app.EmergencyAction("open-sheet"); open != st.cfg.Edit {
		t.Fatalf("open-sheet: %q", open)
	}
	st.forceDown = false
	h.app.TickEmergencySheet(now.Add(10 * time.Minute))
	h.app.TickEmergencySheet(now.Add(11 * time.Minute))
	last := n.shown[len(n.shown)-1]
	if last.Title != "Server pulih" || strings.Count(titles(n.shown), "Server pulih") != 1 {
		t.Fatalf("recovery: %+v", n.shown)
	}
	if h.app.sheetPhase() != 2 {
		t.Fatalf("phase %d", h.app.sheetPhase())
	}
}

func titles(ns []Notice) string {
	var s []string
	for _, n := range ns {
		s = append(s, n.Title)
	}
	return strings.Join(s, "|")
}

func TestSheetReconcilePlanPairsOnlyUnchangedTasks(t *testing.T) {
	same, moved, gone := id.New(), id.New(), id.New()
	snap := map[id.ID]emergency.SnapshotRow{
		same:  {SnapshotID: id.New(), RowID: id.New(), TaskID: same, Version: 3, Progress: "todo"},
		moved: {SnapshotID: id.New(), RowID: id.New(), TaskID: moved, Version: 5, Progress: "todo"},
	}
	rows := []sheet.Row{
		{Line: 4, TaskID: moved, Version: 3, BaseProgress: "todo", Progress: "doing", Title: "Kas"},
		{Line: 2, TaskID: same, Version: 3, BaseProgress: "todo", Progress: "doing", Title: "Poster", ReportedBy: "Raka"},
		{Line: 5, TaskID: gone, Version: 1, BaseProgress: "todo", Progress: "doing", Title: "Lama"},
	}
	changes, manual := sheetReconcilePlan(rows, snap, "2026-11-12 13.34")
	if len(changes) != 1 || changes[0].TaskID != same || changes[0].RowID != snap[same].RowID || changes[0].SnapshotID != snap[same].SnapshotID || changes[0].ReportedBy != "Raka" || changes[0].ReportedAt != "Sheet darurat 2026-11-12 13.34" || changes[0].Progress != "doing" {
		t.Fatalf("changes %+v", changes)
	}
	if len(manual) != 2 || !strings.Contains(manual[0].Reason, "versi 3 → 5") || !strings.Contains(manual[1].Reason, "tidak ada di snapshot") {
		t.Fatalf("manual %+v", manual)
	}
	// Without a reporter, the server still needs one: said honestly.
	rows[1].ReportedBy = ""
	changes, _ = sheetReconcilePlan(rows, snap, "x")
	if changes[0].ReportedBy != "Tidak dicatat di sheet" {
		t.Fatalf("reporter %q", changes[0].ReportedBy)
	}
}

// TestEmergencyCenterScreens draws Pusat darurat in its three phases and
// the banner, captured with SPACE_UI_CAPTURE_DIR.
func TestEmergencyCenterScreens(t *testing.T) {
	h, g, _ := sheetHarness(t)
	h.app.ExportFile = func(string, string, []byte) (string, error) { return "", nil }
	rows := [][]string{
		sheetRowCells(id.New(), "[DUMMY] Desain poster acara", "Dikerjakan", "todo"),
		sheetRowCells(id.New(), "[DUMMY] Rekap kas bulanan", "Selesai", "doing"),
		{"[DUMMY] Tugas baru dari sheet", "", "", "", "Belum mulai"},
		sheetRowCells(id.New(), "[DUMMY] Proposal sponsor", "Hampir", "todo"),
	}
	g.push(xlsxResponse(sheetCopy(t, "46338.565", rows...)))
	h.app.sheetFetch(time.Now())
	h.settle()
	h.app.OpenEmergencyCenter()
	h.app.sheet.nextAt = time.Now().Add(time.Hour)
	h.tt.Advance(500 * time.Millisecond)
	h.tt.Frame()
	for _, want := range []string{"Pusat darurat", "Buka sheet", "Perubahan dari sheet", "Bagikan sheet ke tim", "Salin link", "Bagikan ke WhatsApp", "Isi sheet dari cache", "Mode darurat"} {
		h.want(want)
	}
	captureNativeUI(t, h, "darurat-saat-gangguan")
	h.click("Salin link")
	h.want("Tersalin, tempel di grup")
	h.click("Persiapan")
	h.want("Kesiapan")
	captureNativeUI(t, h, "darurat-persiapan")
	h.app.sheet.editing = true
	h.tt.Frame()
	h.want("Link publish (dibaca Space)")
	captureNativeUI(t, h, "darurat-persiapan-link")
	h.app.sheet.editing = false
	h.app.sheet.forceDown = false
	h.click("Setelah pulih")
	h.want("1 · Bekukan sheet")
	h.click("Sudah dibekukan")
	h.want("Versi final")
	captureNativeUI(t, h, "darurat-setelah-pulih")
}
