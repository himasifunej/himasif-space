package appui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/emergency"
	"github.com/himasifunej/himasif-space/internal/emergency/sheet"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/qr"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// The emergency sheet (note "Space — Darurat via Google Sheets", canvas
// dcb4562af51c option B and design 5). Prepared while the server is up: the
// sheet's edit and publish links, kept on this device. When the server is
// down, the team edits the Google Sheet and this device reads its published
// copy every 3 minutes, keeping the newest by the sheet's clock: the
// progress it shows for those tasks comes from the sheet, and is changed
// there, not here. After recovery the sheet is frozen, its final copy read,
// and the changes go through the existing reconciliation against a fresh
// server snapshot. Space never writes to Google.

// sheetConfig is what was prepared: the links, and the last test.
type sheetConfig struct {
	Publish  string    `json:"publish,omitempty"` // …/pub?output=xlsx
	Edit     string    `json:"edit,omitempty"`    // …/edit?usp=sharing
	Tested   time.Time `json:"tested,omitzero"`
	TestNote string    `json:"testNote,omitempty"`
	// Reminded is when Ace last asked to prepare the sheet.
	Reminded time.Time `json:"reminded,omitzero"`
}

// sheetSession is one outage: from the first copy read to Akhiri.
type sheetSession struct {
	Active bool            `json:"active"`
	Since  time.Time       `json:"since,omitzero"`
	Filled time.Time       `json:"filled,omitzero"` // this device filled the sheet
	Follow sheet.Follow    `json:"follow"`
	Feed   []sheetFeed     `json:"feed,omitempty"` // newest first
	Stale  int             `json:"stale,omitempty"`
	Frozen time.Time       `json:"frozen,omitzero"` // Sheet dibekukan, as told
	Closed map[string]bool `json:"closed,omitempty"`
	// Ace says each once per session.
	SaidDown bool `json:"saidDown,omitempty"`
	SaidUp   bool `json:"saidUp,omitempty"`
}

// sheetFeed is a line of Perubahan dari sheet.
type sheetFeed struct {
	At     time.Time `json:"at"`
	Kind   string    `json:"kind"` // applied, issue, held, manual, error
	Title  string    `json:"title"`
	Detail string    `json:"detail,omitempty"`
}

const sheetFeedMax = 60

type sheetUIState struct {
	user        id.ID
	loaded      bool
	cfg         sheetConfig
	sess        sheetSession
	publishIn   string
	editIn      string
	editing     bool
	err, notice string
	busy        bool // a test, an export, a reconciliation
	fetching    bool
	failures    int
	lastErr     string
	lastFetch   time.Time
	nextAt      time.Time
	downSince   time.Time
	phase       int // the tab chosen; -1 follows the state
	copied      time.Time
	qr          *ui.Bitmap
	qrFor       string
	fill        sheetFillState
	recon       sheetReconState
	legacy      bool
	// forceDown stands in for an unreachable server in tests.
	forceDown bool
	// http fetches the sheet; nil: a client with a timeout.
	http *http.Client
	loc  *time.Location
}

type sheetFillState struct {
	open    bool
	spaces  map[id.ID]bool
	tasks   []sheet.Task
	loading bool
	err     string
}

type sheetReconState struct {
	prepared bool
	manual   []sheetManual
	count    int
}

// sheetManual is a sheet row the reconciliation cannot take: why, and the
// task to open.
type sheetManual struct {
	TaskID id.ID
	Title  string
	Reason string
}

func (a *App) sheetStore() (load func(string) ([]byte, error), save func(string, []byte) error) {
	c := a.offlineClient()
	if c == nil {
		return nil, nil
	}
	return c.DeviceRecord, c.SaveDeviceRecord
}

// sheetEnsure loads the sheet's records for the signed-in account.
func (a *App) sheetEnsure() *sheetUIState {
	st := &a.sheet
	if st.loaded && st.user == a.user.ID {
		return st
	}
	keepHTTP, keepDown := st.http, st.forceDown
	*st = sheetUIState{user: a.user.ID, loaded: true, phase: -1, http: keepHTTP, forceDown: keepDown}
	if load, _ := a.sheetStore(); load != nil {
		if b, err := load("darurat-config"); err == nil {
			_ = json.Unmarshal(b, &st.cfg)
		}
		if b, err := load("darurat-session"); err == nil {
			_ = json.Unmarshal(b, &st.sess)
		}
	}
	st.publishIn, st.editIn = st.cfg.Publish, st.cfg.Edit
	return st
}

func (a *App) sheetSave() {
	_, save := a.sheetStore()
	if save == nil {
		return
	}
	st := &a.sheet
	if b, err := json.Marshal(st.cfg); err == nil {
		if err = save("darurat-config", b); err != nil {
			st.err = "Pengaturan sheet belum tersimpan di perangkat: " + err.Error()
		}
	}
	if b, err := json.Marshal(st.sess); err == nil {
		if err = save("darurat-session", b); err != nil {
			st.err = "Data sheet belum tersimpan di perangkat: " + err.Error()
		}
	}
}

// sheetZone is the sheet's zone: the period's, as the template asks the
// sheet to be set.
func (a *App) sheetZone() *time.Location {
	if a.sheet.loc != nil {
		return a.sheet.loc
	}
	zone := "Asia/Jakarta"
	if p := a.period(); p != nil && p.TimeZone != "" {
		zone = p.TimeZone
	}
	loc, err := timeutil.LoadZone(zone)
	if err != nil {
		return time.FixedZone("WIB", 7*3600)
	}
	return loc
}

func (a *App) serverDown() bool { return a.sheet.forceDown || a.offlineMode() }

// sheetLinked: links saved on this device.
func (st *sheetUIState) linked() bool { return st.cfg.Publish != "" && st.cfg.Edit != "" }

func (st *sheetUIState) feed(kind, title, detail string, at time.Time) {
	st.sess.Feed = append([]sheetFeed{{At: at, Kind: kind, Title: title, Detail: detail}}, st.sess.Feed...)
	if len(st.sess.Feed) > sheetFeedMax {
		st.sess.Feed = st.sess.Feed[:sheetFeedMax]
	}
}

// TickEmergencySheet runs the sheet's clock: noticing an outage, reading
// the sheet when it is due, Ace's lines. The desktop calls it every 15
// seconds from the tray as well, so it works with the window hidden.
func (a *App) TickEmergencySheet(now time.Time) {
	if a.screen != screenShell || a.user.ID == id.Nil || a.isAdmin() {
		return
	}
	st := a.sheetEnsure()
	down := a.serverDown()
	if !down {
		st.downSince = time.Time{}
	} else if st.downSince.IsZero() {
		st.downSince = now
	}
	if !st.linked() {
		if st.cfg.Reminded.IsZero() {
			// A day's grace before Ace first asks, after an install or
			// the onboarding.
			st.cfg.Reminded = now.Add(-6 * 24 * time.Hour)
		}
		if !down && a.sheetManager() && now.Sub(st.cfg.Reminded) > 7*24*time.Hour && a.Ace != nil {
			st.cfg.Reminded = now
			a.sheetSave()
			a.sheetSay("setup", "Sheet darurat belum disiapkan", "Persiapan sekali, ±5 menit, selagi server hidup.", "open", "Siapkan", []NoticeAction{{Label: "Nanti", Link: sheetLink("later")}})
		}
		return
	}
	// Two minutes unreachable is an outage: the first copy starts the
	// session, Ace offers the sheet once.
	if down && now.Sub(st.downSince) >= 2*time.Minute && !st.sess.SaidDown {
		if !st.sess.Active {
			st.sess = sheetSession{Active: true, Since: st.downSince}
		}
		st.sess.SaidDown = true
		st.sess.SaidUp = false
		a.sheetSave()
		a.sheetSay("down", "Server Space mati", "Buka sheet darurat? Tim mencatat progres di sana sampai server pulih.", "open-sheet", "Buka sheet", []NoticeAction{{Label: "Salin link", Link: sheetLink("copy-link")}})
	}
	if !st.sess.Active {
		return
	}
	if !down && !st.sess.SaidUp {
		st.sess.SaidUp = true
		st.sess.SaidDown = false
		a.sheetSave()
		a.sheetSay("up", "Server pulih", "Bekukan sheet dulu (link edit jadi Pelihat), lalu rekonsiliasi di Pusat darurat.", "open", "Buka Pusat darurat", nil)
	}
	if !st.fetching && !now.Before(st.nextAt) {
		a.sheetFetch(now)
	}
}

// sheetManager: someone who manages tasks in a Space, who would prepare
// and fill the sheet.
func (a *App) sheetManager() bool {
	for _, s := range a.access.spaces {
		if spaceCan(s, api.CapTasksManage) {
			return true
		}
	}
	return false
}

// sheetFetch reads the published sheet once.
func (a *App) sheetFetch(now time.Time) {
	st := &a.sheet
	if st.fetching || st.cfg.Publish == "" {
		return
	}
	st.fetching = true
	gen, uid, url, hc, loc := a.gen, a.user.ID, st.cfg.Publish, st.http, a.sheetZone()
	a.async(func() func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		snap, err := sheet.Fetch(ctx, hc, url, loc)
		cancel()
		return func() {
			if gen != a.gen || uid != a.user.ID {
				return
			}
			a.sheetApply(snap, err, time.Now())
		}
	})
}

// sheetApply takes a fetch's result on the UI thread.
func (a *App) sheetApply(snap sheet.Snapshot, err error, now time.Time) {
	st := &a.sheet
	st.fetching = false
	st.lastFetch = now
	if err != nil {
		st.failures++
		msg := err.Error()
		if msg != st.lastErr {
			st.feed("error", msg, "Data di perangkat tidak berubah. Dicoba lagi otomatis.", now)
		}
		st.lastErr = msg
		st.nextAt = now.Add(sheet.NextDelay(st.failures, err, rand.Float64()))
		a.sheetSave()
		return
	}
	st.failures, st.lastErr = 0, ""
	st.nextAt = now.Add(sheet.NextDelay(0, nil, rand.Float64()))
	if !st.sess.Active {
		st.sess = sheetSession{Active: true, Since: now}
	}
	before := len(st.sess.Follow.Issues)
	out := st.sess.Follow.Offer(snap)
	switch out.Kind {
	case sheet.OutStale:
		// Google served a copy older than one already read: set aside.
		st.sess.Stale++
	case sheet.OutHeld:
		st.feed("held", fmt.Sprintf("%d baris berubah sekaligus", out.Held), "Belum diterapkan. Periksa sheet, lalu terapkan atau abaikan salinan ini.", now)
		a.sheetSay("held", fmt.Sprintf("%d baris berubah sekaligus", out.Held), "Periksa sheet dulu; perangkat ini menahan salinan itu.", "open", "Periksa", nil)
	case sheet.OutApplied:
		a.sheetApplied(out, before, now)
	}
	a.sheetSave()
}

func (a *App) sheetApplied(out sheet.Outcome, issuesBefore int, now time.Time) {
	st := &a.sheet
	for _, ch := range out.Changes {
		detail := fmt.Sprintf("Baris %d", ch.Line)
		if r, ok := st.sess.Follow.Row(ch.TaskID); ok && r.ReportedBy != "" {
			detail += " · dilaporkan oleh " + r.ReportedBy
		}
		title := ch.Title + " · " + strings.ToLower(sheet.ProgressWord(ch.From)) + " → " + strings.ToLower(sheet.ProgressWord(ch.To))
		if ch.From == ch.To {
			title = ch.Title + " · catatan"
		}
		st.feed("applied", title, detail, now)
	}
	if out.Gone > 0 {
		st.feed("manual", fmt.Sprintf("%d baris tidak ada lagi di sheet", out.Gone), "Tugasnya tetap di Space dengan isi terakhir dari sheet.", now)
	}
	issues := st.sess.Follow.Issues
	if len(issues) > issuesBefore {
		first := issues[0]
		st.feed("issue", fmt.Sprintf("%d baris ditolak", len(issues)), fmt.Sprintf("Mis. baris %d: %s. Perbaiki di sheet; baris lain tetap diterapkan.", first.Line, first.Reason), now)
	}
	a.sheetOverlayTasks()
	if len(out.Changes) > 0 || len(issues) > issuesBefore {
		body := fmt.Sprintf("%d perubahan diterapkan di perangkat", len(out.Changes))
		if n := len(issues); n > 0 {
			body += fmt.Sprintf(" · %d baris ditolak", n)
		}
		a.sheetSay("read", sheet.Lag(st.sess.Follow.Clock, now), body, "open", "Lihat", nil)
	}
}

// sheetOverlayTasks shows the sheet's progress on the loaded tasks.
func (a *App) sheetOverlayTasks() {
	if !a.tasks.loaded {
		return
	}
	a.tasks.items = a.overlaySheetTasks(a.tasks.items)
	a.projectTasks()
}

// overlaySheetTasks puts the sheet's progress on tasks it changed.
func (a *App) overlaySheetTasks(items []api.Task) []api.Task {
	st := &a.sheet
	if !st.loaded || !st.sess.Active {
		return items
	}
	for i := range items {
		if r, ok := st.sess.Follow.Row(items[i].ID); ok && r.Progress != r.BaseProgress && r.Progress != "" {
			items[i].Progress = r.Progress
		}
	}
	return items
}

// sheetLocked: the task's progress is the sheet's while the session lasts.
func (a *App) sheetLocked(tid id.ID) bool {
	st := &a.sheet
	if !st.loaded || !st.sess.Active {
		return false
	}
	_, ok := st.sess.Follow.Row(tid)
	return ok
}

// sheetLockNotice says why a progress did not change here.
func (a *App) sheetLockNotice() {
	a.tasks.notice = "Progres tugas ini diatur dari sheet darurat selama mode darurat. Ubah di sheet; Space membacanya tiap 3 menit."
}

// sheetFromSheet reports a task whose shown progress comes from the sheet.
func (a *App) sheetFromSheet(tid id.ID) (sheet.Row, bool) {
	st := &a.sheet
	if !st.loaded || !st.sess.Active {
		return sheet.Row{}, false
	}
	r, ok := st.sess.Follow.Row(tid)
	return r, ok && r.Progress != r.BaseProgress
}

// sheetLink is an island action's link (app/emergency_links_desktop.go).
func sheetLink(action string) string { return "himasif-space://darurat?action=" + action }

// NoticeEmergency is the kind of Ace's emergency lines.
const NoticeEmergency = "darurat"

// sheetSay is Ace's line on the island (and in the OS notification when
// the island is off): one at a time per kind.
func (a *App) sheetSay(kind, title, body, open, openLabel string, actions []NoticeAction) {
	if a.Notifier == nil || a.Ace == nil {
		return
	}
	_ = a.Notifier.Present(Notice{Kind: NoticeEmergency, Title: title, Body: body, Tag: "darurat-" + kind, Link: sheetLink(open), OpenLabel: openLabel, Actions: actions, AccountID: a.user.ID.String()})
}

// EmergencyAction answers an island action: the URL to open in the
// browser, the text to copy, and whether to show Pusat darurat.
func (a *App) EmergencyAction(action string) (open, copyText string, show bool) {
	st := a.sheetEnsure()
	switch action {
	case "open-sheet":
		return st.cfg.Edit, "", st.cfg.Edit == ""
	case "copy-link":
		if st.cfg.Edit == "" {
			return "", "", true
		}
		st.copied = time.Now()
		a.sheetSay("copied", "Link sheet disalin", "Tempel di grup pengurus, atau kirim lewat WhatsApp.", "whatsapp", "WhatsApp", nil)
		return "", st.cfg.Edit, false
	case "whatsapp":
		if st.cfg.Edit == "" {
			return "", "", true
		}
		return sheet.WhatsAppURL(sheet.ShareText(st.cfg.Edit)), "", false
	case "later":
		return "", "", false
	}
	a.OpenEmergencyCenter()
	return "", "", true
}

// OpenEmergencyCenter shows Pusat darurat.
func (a *App) OpenEmergencyCenter() {
	if a.screen != screenShell {
		return
	}
	a.navigate(PathPengaturan)
	a.chrome.settingsTab = "Darurat"
	a.chrome.settingsDetail = true
}

// sheetPhase is the tab Pusat darurat shows: 0 Persiapan, 1 Saat
// gangguan, 2 Setelah pulih.
func (a *App) sheetPhase() int {
	st := &a.sheet
	if st.phase >= 0 {
		return st.phase
	}
	switch {
	case st.sess.Active && !a.serverDown():
		return 2
	case st.sess.Active || a.serverDown():
		return 1
	}
	return 0
}

// sheetSaveLinks checks pasted links, reads the sheet once, and keeps them.
func (a *App) sheetSaveLinks() {
	st := &a.sheet
	pub, err := sheet.PublishURL(st.publishIn)
	if err != nil {
		st.err = err.Error()
		return
	}
	edit, err := sheet.EditURL(st.editIn)
	if err != nil {
		st.err = err.Error()
		return
	}
	st.err, st.notice, st.busy = "", "", true
	gen, uid, hc, loc := a.gen, a.user.ID, st.http, a.sheetZone()
	a.async(func() func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		snap, err := sheet.Fetch(ctx, hc, pub, loc)
		cancel()
		return func() {
			if gen != a.gen || uid != a.user.ID {
				return
			}
			st := &a.sheet
			st.busy = false
			if err != nil {
				st.err = "Link belum bisa dipakai: " + err.Error() + ". Periksa Publikasikan ke web: Seluruh dokumen, Microsoft Excel (.xlsx)."
				return
			}
			now := time.Now()
			st.cfg.Publish, st.cfg.Edit = pub, edit
			st.publishIn, st.editIn = pub, edit
			st.cfg.Tested = now
			st.cfg.TestNote = fmt.Sprintf("Terbaca: %d baris tugas, %s.", len(snap.Rows), sheet.Lag(snap.Clock, now))
			if now.Sub(snap.Clock) > 10*time.Minute {
				st.cfg.TestNote += " Jam sheet tertinggal: pastikan Penghitungan \"setiap menit\" dan publikasi ulang otomatis."
			}
			st.editing = false
			st.notice = "Link tersimpan di perangkat ini."
			a.sheetSave()
		}
	})
}

// sheetForgetLinks removes the links from this device.
func (a *App) sheetForgetLinks() {
	st := &a.sheet
	st.cfg = sheetConfig{Reminded: st.cfg.Reminded}
	st.publishIn, st.editIn, st.editing = "", "", true
	st.notice = "Link dihapus dari perangkat ini."
	a.sheetSave()
}

// sheetQR is the QR of the edit link, drawn once per link.
func (a *App) sheetQR() *ui.Bitmap {
	st := &a.sheet
	if st.cfg.Edit == "" {
		return nil
	}
	if st.qrFor != st.cfg.Edit {
		st.qr, st.qrFor = nil, st.cfg.Edit
		if code, err := qr.Encode([]byte(st.cfg.Edit)); err == nil {
			st.qr = ui.NewBitmap(code.Image(6, 4))
		}
	}
	return st.qr
}

// sheetTemplate saves the empty workbook to upload to Google Drive.
func (a *App) sheetTemplate() {
	if a.ExportFile == nil {
		return
	}
	b, err := sheet.Workbook(nil, time.Now().In(a.sheetZone()))
	if err != nil {
		a.sheet.err = err.Error()
		return
	}
	a.sheetExport("space-darurat-template.xlsx", b)
}

func (a *App) sheetExport(name string, data []byte) {
	save := a.ExportFile
	st := &a.sheet
	st.busy = true
	gen := a.gen
	a.async(func() func() {
		path, err := save(name, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
		return func() {
			if gen != a.gen {
				return
			}
			st := &a.sheet
			st.busy = false
			if err != nil {
				st.err = err.Error()
				return
			}
			if path != "" {
				st.notice = "Tersimpan: " + path
			}
		}
	})
}

// sheetLoadFill gathers the chosen Spaces' active tasks from the device
// cache (the server may be down), for the sheet.
func (a *App) sheetLoadFill() {
	st := &a.sheet
	service, ok := a.taskService()
	if !ok || st.fill.loading {
		return
	}
	var spaces []api.Space
	for _, s := range a.access.spaces {
		if st.fill.spaces[s.ID] {
			spaces = append(spaces, s)
		}
	}
	st.fill.loading, st.fill.err, st.fill.tasks = true, "", nil
	gen, uid := a.gen, a.user.ID
	a.async(func() func() {
		var out []sheet.Task
		var errs []error
		seen := map[id.ID]bool{}
		for _, s := range spaces {
			items, err := service.SpaceTasks(context.Background(), s.ID, api.TaskFilter{})
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %s", s.Name, err))
				continue
			}
			for _, t := range items {
				if seen[t.ID] || t.Progress == api.TaskDone || t.Progress == api.TaskCancelled {
					continue
				}
				seen[t.ID] = true
				out = append(out, sheet.Task{ID: t.ID, Version: t.Version, Progress: t.Progress, Title: t.Title, Assignee: t.Assignee.DisplayName, Space: s.Name, Deadline: taskDate(t.DeadlineDate, t.DeadlineAt, t.TimeZone)})
			}
		}
		return func() {
			if gen != a.gen || uid != a.user.ID {
				return
			}
			st := &a.sheet
			st.fill.loading = false
			st.fill.tasks = out
			if len(errs) > 0 {
				st.fill.err = "Sebagian tugas belum ada di cache perangkat: " + errors.Join(errs...).Error()
			}
		}
	})
}

// sheetFilled marks that this device filled the sheet: the session starts.
func (a *App) sheetFilled(now time.Time) {
	st := &a.sheet
	if !st.sess.Active {
		st.sess = sheetSession{Active: true, Since: now, SaidDown: true}
	}
	st.sess.Filled = now
	st.feed("manual", fmt.Sprintf("Sheet diisi dari cache · %d tugas", len(st.fill.tasks)), "Tempel di A1 tab Tugas, atau impor .xlsx.", now)
	st.nextAt = now
	a.sheetSave()
}

// sheetReconcilePlan pairs the sheet's changed rows with a fresh server
// snapshot: a row whose task the server still has at the version the sheet
// was filled from goes to the reconciliation; any other is for a person.
func sheetReconcilePlan(rows []sheet.Row, snapshot map[id.ID]emergency.SnapshotRow, clockText string) ([]api.EmergencyChange, []sheetManual) {
	var changes []api.EmergencyChange
	var manual []sheetManual
	rows = slices.Clone(rows)
	slices.SortFunc(rows, func(x, y sheet.Row) int { return x.Line - y.Line })
	for _, r := range rows {
		s, ok := snapshot[r.TaskID]
		switch {
		case !ok:
			manual = append(manual, sheetManual{TaskID: r.TaskID, Title: r.Title, Reason: "Tugas tidak ada di snapshot server (dipindah, dihapus, atau di luar akses)."})
			continue
		case s.Version != r.Version:
			manual = append(manual, sheetManual{TaskID: r.TaskID, Title: r.Title, Reason: fmt.Sprintf("Tugas berubah di server sejak sheet diisi (versi %d → %d). Periksa dan perbarui di tugasnya.", r.Version, s.Version)})
			continue
		}
		at := r.ReportedAt
		if at == "" {
			at = "Sheet darurat " + clockText
		}
		by := r.ReportedBy
		if by == "" {
			by = "Tidak dicatat di sheet"
		}
		note := r.Note
		if note == "" {
			note = "Dari sheet darurat."
		}
		changes = append(changes, api.EmergencyChange{TaskID: r.TaskID, SnapshotID: s.SnapshotID, RowID: s.RowID, Progress: r.Progress, ReportedAt: at, ReportedBy: by, Note: note})
	}
	return changes, manual
}

// sheetPrepareReconcile makes a server snapshot of each Space the changed
// tasks are in, pairs the rows, and opens the existing preview.
func (a *App) sheetPrepareReconcile() {
	st := &a.sheet
	cl := a.notificationClient()
	service, ok := a.taskService()
	if cl == nil || !ok || st.busy || a.serverDown() {
		return
	}
	rows := st.sess.Follow.Changed()
	noID := st.sess.Follow.NoID
	clock := st.sess.Follow.Clock.Format("2006-01-02 15.04")
	st.busy, st.err = true, ""
	gen, uid := a.gen, a.user.ID
	a.async(func() func() {
		bySpace := map[id.ID]bool{}
		var missing []sheetManual
		var known []sheet.Row
		for _, r := range rows {
			t, err := service.GetTask(context.Background(), r.TaskID)
			if err != nil || len(t.SpaceIDs) == 0 {
				missing = append(missing, sheetManual{TaskID: r.TaskID, Title: r.Title, Reason: "Tugas tidak dapat dibuka dengan akses sekarang."})
				continue
			}
			bySpace[t.SpaceIDs[0]] = true
			known = append(known, r)
		}
		snapshot := map[id.ID]emergency.SnapshotRow{}
		var err error
		for sid := range bySpace {
			var snap api.EmergencySnapshot
			snap, err = cl.CreateEmergencySnapshot(context.Background(), sid, api.EmergencySnapshotRequest{IncludeCompleted: true})
			var csv []byte
			if err == nil {
				csv, err = cl.EmergencySnapshotCSV(context.Background(), snap.ID)
			}
			var srows []emergency.SnapshotRow
			if err == nil {
				srows, err = emergency.ParseSnapshotRows(csv)
			}
			if err != nil {
				break
			}
			for _, s := range srows {
				snapshot[s.TaskID] = s
			}
		}
		changes, manual := sheetReconcilePlan(known, snapshot, clock)
		manual = append(missing, manual...)
		for _, r := range noID {
			manual = append(manual, sheetManual{Title: r.Title, Reason: fmt.Sprintf("Baris %d tanpa ID: tugas yang diketik di sheet. Buat di Space bila memang perlu.", r.Line)})
		}
		return func() {
			if gen != a.gen || uid != a.user.ID {
				return
			}
			st := &a.sheet
			st.busy = false
			if err != nil {
				st.err = "Snapshot server belum dapat dibuat: " + a.failed(err)
				return
			}
			st.recon = sheetReconState{prepared: true, manual: manual, count: len(changes)}
			if len(changes) > 100 {
				st.recon.manual = append(st.recon.manual, sheetManual{Reason: fmt.Sprintf("%d baris di atas 100 pertama menunggu putaran berikutnya.", len(changes)-100)})
				changes = changes[:100]
			}
			e := &a.emergency
			if e.user != a.user.ID || e.period != a.periodID {
				*e = emergencyUIState{seq: e.seq + 1, user: a.user.ID, period: a.periodID, space: a.selectedSpaceID()}
			}
			e.changes, e.results, e.selected, e.confirmed = changes, nil, map[id.ID]bool{}, false
			e.fileName = "Sheet darurat (" + clock + ")"
			if len(changes) > 0 {
				a.reconcileEmergency("preview")
			}
		}
	})
}

// sheetEnd closes the session on this device: the overlay and locks go,
// the tasks reload from the server.
func (a *App) sheetEnd() {
	st := &a.sheet
	st.sess = sheetSession{}
	st.recon = sheetReconState{}
	st.phase, st.failures, st.lastErr, st.nextAt = -1, 0, "", time.Time{}
	st.notice = "Mode darurat selesai di perangkat ini."
	a.sheetSave()
	a.tasks.loaded = false
}
