package appui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// Medhum sub-tabs, as on canvas A.
const (
	medhumTabList    = "list"
	medhumTabCal     = "cal"
	medhumTabInbox   = "inbox"
	medhumTabSources = "src"
	medhumTabPolicy  = "policy"
)

// medhumSpaceTab is the Space tab holding the Medhum content workspace.
const medhumSpaceTab = "Konten"

// medhumStage is where a content item is in the Humas–Mediatek flow.
type medhumStage int

const (
	medhumConcept medhumStage = iota
	medhumDesign
	medhumReview
	medhumReady
	medhumPosted
)

var medhumStageNames = [...]string{"Konsep Humas", "Desain Mediatek", "Bukti perlu tinjauan", "Siap posting", "Terposting"}

func medhumStageColor(t *ui.Theme, s medhumStage) ui.Color {
	switch s {
	case medhumDesign:
		return ui.Hex("#a2bbd2")
	case medhumReview:
		return t.Warning
	case medhumReady:
		return t.Accent
	case medhumPosted:
		return t.Success
	}
	return ui.Hex("#8f8878")
}

// medhumStageOf derives the stage from the content and the Drive evidence
// in the WhatsApp inbox: published wins, then the concept, then evidence.
func (a *App) medhumStageOf(item api.MedhumContent) (medhumStage, string) {
	if item.PublishedAt != nil {
		return medhumPosted, "Terposting"
	}
	evidence, pending := false, false
	for _, w := range a.waContent.items {
		if w.Kind != "drive" {
			continue
		}
		mine := w.ContentID != nil && *w.ContentID == item.ID
		switch {
		case mine && (w.State == "applied" || w.State == "corrected"):
			evidence = true
		case w.State == "pending" && (mine || slices.Contains(w.Candidates, item.ID)):
			pending = true
		}
	}
	switch {
	case item.ConceptSubmittedAt == nil:
		return medhumConcept, "—"
	case evidence:
		return medhumReady, "Ada bukti"
	case pending:
		return medhumReview, "Perlu tinjauan"
	}
	return medhumDesign, "Belum ada"
}

func (a *App) medhumZone() *time.Location {
	zone := ""
	if p := a.period(); p != nil {
		zone = p.TimeZone
	}
	loc, err := timeutil.LoadZone(zone)
	if err != nil || loc == nil {
		return time.UTC
	}
	return loc
}

// medhumDay is the calendar day of a date-only or timed deadline.
func medhumDay(date *timeutil.Date, at *time.Time, loc *time.Location) (time.Time, bool) {
	if date != nil {
		return time.Date(date.Year, date.Month, date.Day, 0, 0, 0, 0, loc), true
	}
	if at != nil {
		l := at.In(loc)
		return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, loc), true
	}
	return time.Time{}, false
}

// medhumShort writes "Sel, 10 Nov · 09.00", or "· tanpa jam" for dates.
func medhumShort(date *timeutil.Date, at *time.Time, loc *time.Location) string {
	day, ok := medhumDay(date, at, loc)
	if !ok {
		return "—"
	}
	s := fmt.Sprintf("%s, %d %s", shortDays[day.Weekday()], day.Day(), monthName(day.Month())[:3])
	if at != nil {
		return s + " · " + at.In(loc).Format("15.04")
	}
	return s
}

func (a *App) medhumSorted() []api.MedhumContent {
	loc := a.medhumZone()
	items := slices.Clone(a.medhum.items)
	key := func(x api.MedhumContent) time.Time {
		if x.PostingAt != nil {
			return *x.PostingAt
		}
		if d, ok := medhumDay(x.PostingDate, nil, loc); ok {
			return d.Add(24*time.Hour - time.Minute)
		}
		return time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	slices.SortStableFunc(items, func(x, y api.MedhumContent) int { return key(x).Compare(key(y)) })
	return items
}

// medhumPage is the Medhum workspace: a header with ping and new content,
// the stage counters and five sub-tabs. Phones scroll it as one page.
func (a *App) medhumPage(c *ui.Context) {
	a.ensureMedhum()
	a.ensureWAContent()
	phone := isMobile(c)
	ui.Scroll(c.Key("medhum-scroll")).Fill().Children(func() {
		body := ui.Column(c).FillWidth().MinWidth(0).MaxWidth(1280).Padding(26).Gap(18)
		if phone {
			body.Padding(16).Gap(14)
		}
		body.Children(func() {
			a.medhumHeader(c, phone)
			a.medhumCounters(c, phone)
			a.medhumTabs(c, phone)
			if a.medhum.err != "" && a.medhum.draft == nil {
				pageNotice(c, "alert", "Permintaan belum selesai", a.medhum.err)
			}
			switch a.medhum.tab {
			case medhumTabCal:
				a.medhumCalendar(c, phone)
			case medhumTabInbox:
				a.waContentBody(c)
			case medhumTabSources:
				a.connectorsBody(c)
			case medhumTabPolicy:
				a.medhumPolicyPanel(c)
			default:
				a.medhumList(c)
			}
		})
	})
	a.medhumDialog(c)
}

func (a *App) medhumHeader(c *ui.Context, phone bool) {
	head := ui.Column(c.Key("medhum-head")).FillWidth().MinWidth(0).Gap(12)
	if !phone {
		head.Row().AlignItems(ui.Start)
	}
	head.Children(func() {
		titles := ui.Column(c).MinWidth(0).Gap(6)
		if !phone {
			titles.Grow(1)
		}
		titles.Children(func() {
			ui.Text(c, "Medhum").FontSize(26).Bold()
			muted(c, "Ruang bersama Humas–Mediatek. Tanggal Humas adalah jadwal posting; deadline desain otomatis H-2 sekali.")
		})
		ui.Row(c).Gap(8).Wrap().Shrink(0).Children(func() {
			if canPing(a.selectedSpace()) && spaceButton(c, "Ping Medhum", "bell", a.chrome.pingOpen).Border(1, c.Theme().Border).Clicked() {
				a.chrome.pingOpen = true
			}
			if primarySpaceButton(c, "Konten baru", "plus").Disabled(len(a.medhum.humans) == 0 || a.medhum.busy || a.offlineMode()).Clicked() {
				a.beginMedhum(nil)
			}
		})
	})
}

func (a *App) medhumCounters(c *ui.Context, phone bool) {
	t := c.Theme()
	loc := a.medhumZone()
	now := c.Now().In(loc)
	var counts [5]int
	for _, item := range a.medhum.items {
		s, _ := a.medhumStageOf(item)
		if s == medhumPosted {
			if p := item.PublishedAt.In(loc); p.Year() != now.Year() || p.Month() != now.Month() {
				continue
			}
		}
		counts[s]++
	}
	tiles := []struct {
		label string
		stage medhumStage
	}{{"Konsep belum masuk", medhumConcept}, {"Desain dikerjakan", medhumDesign}, {"Bukti perlu tinjauan", medhumReview}, {"Siap posting", medhumReady}, {"Terposting bulan ini", medhumPosted}}
	tile := func(label string, stage medhumStage) {
		box := ui.Column(c.Key(label)).MinWidth(0).Padding(14).Gap(4).Radius(12).Background(t.Surface).Border(1, t.Border)
		if phone {
			box.Width(150).Shrink(0)
		}
		box.Children(func() {
			ui.Text(c, fmt.Sprint(counts[stage])).FontSize(24).Bold()
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(8, 8).Radius(4).Background(medhumStageColor(t, stage)).Shrink(0)
				l := muted(c, label).MinWidth(0)
				if phone {
					l.MaxLines(2)
				} else {
					l.MaxLines(1)
				}
			})
		})
	}
	if phone {
		ui.ScrollHorizontal(c.Key("medhum-kpis")).FillWidth().Children(func() {
			ui.Row(c).Gap(10).Children(func() {
				for _, k := range tiles {
					tile(k.label, k.stage)
				}
			})
		})
		return
	}
	ui.Grid(c.Key("medhum-kpis")).FillWidth().Columns(5).Gap(12).Children(func() {
		for _, k := range tiles {
			tile(k.label, k.stage)
		}
	})
}

func (a *App) medhumTabs(c *ui.Context, phone bool) {
	t := c.Theme()
	pending := 0
	for _, w := range a.waContent.items {
		if w.State == "pending" {
			pending++
		}
	}
	inbox := "Inbox broadcast"
	if pending > 0 {
		inbox = fmt.Sprintf("Inbox broadcast · %d", pending)
	}
	tabs := []struct{ key, label, icon string }{
		{medhumTabList, "Daftar konten", "task"},
		{medhumTabCal, "Kalender posting", "calendar"},
		{medhumTabInbox, inbox, "whatsapp"},
		{medhumTabSources, "Sumber dan sinkron", "link"},
		{medhumTabPolicy, "Aturan jam", "clock"},
	}
	row := func() {
		ui.Row(c).Gap(4).Padding(4).Radius(10).Background(t.Surface).Border(1, t.Border).Children(func() {
			for _, tab := range tabs {
				on := a.medhum.tab == tab.key || a.medhum.tab == "" && tab.key == medhumTabList
				b := spaceButton(c.Key(tab.key), tab.label, tab.icon, on).Label(tab.label)
				if on {
					b.Background(t.SurfacePressed).TextColor(t.Text)
				}
				if b.Clicked() {
					a.medhum.tab = tab.key
				}
			}
		})
	}
	if phone {
		ui.ScrollHorizontal(c.Key("medhum-tabs")).FillWidth().Children(row)
		return
	}
	ui.Row(c.Key("medhum-tabs")).FillWidth().Wrap().Children(row)
}

func medhumChip(c *ui.Context, stage medhumStage) {
	t := c.Theme()
	color := medhumStageColor(t, stage)
	ui.Row(c).Gap(6).Padding(3, 8).Radius(999).Background(t.Background).Border(1, t.Border).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Box(c).Size(7, 7).Radius(4).Background(color)
		ui.Text(c, medhumStageNames[stage]).FontSize(11).MaxLines(1)
	})
}

func (a *App) medhumList(c *ui.Context) {
	t := c.Theme()
	loc := a.medhumZone()
	ui.Row(c.Key("medhum-list-tools")).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.TextInput(c, &a.medhum.filter).Label("Cari konten").Placeholder("Cari judul konten atau PJ…").Grow(1).MinWidth(0)
		if spaceIconButton(c, "Muat ulang konten", "refresh").Disabled(a.medhum.loading || a.medhum.busy).Clicked() {
			a.medhum.err = ""
			a.loadMedhum()
			a.waContent.loaded = false
			a.waContent.err = ""
		}
	})
	q := strings.ToLower(strings.TrimSpace(a.medhum.filter))
	var items []api.MedhumContent
	for _, item := range a.medhumSorted() {
		if q != "" && !strings.Contains(strings.ToLower(item.Title+" "+item.HumasPJName+" "+item.DesignPJName), q) {
			continue
		}
		items = append(items, item)
	}
	if a.medhum.loading && !a.medhum.loaded {
		muted(c, "Memuat konten Medhum…")
	}
	if a.medhum.loaded && len(items) == 0 {
		if q != "" {
			pageEmpty(c, "search", "Tidak ada yang cocok", "Ubah kata pencarian.")
		} else {
			pageEmpty(c, "calendar", "Belum ada konten", "Tambahkan konten atau ambil dari sumber kalender Humas.")
		}
	}
	if len(items) == 0 {
		return
	}
	frame := ui.Column(c.Key("medhum-table")).FillWidth().MinWidth(0).Radius(12).Background(t.Surface).Border(1, t.Border)
	wide := frame.Bounds().W >= 900
	frame.Children(func() {
		designName := func(item api.MedhumContent) string {
			if item.DesignPJID == nil {
				return "Belum ditetapkan"
			}
			return item.DesignPJName
		}
		if wide {
			ui.Row(c.Key("head")).FillWidth().Padding(10, 16).Gap(12).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				for _, h := range []struct {
					label string
					width float32
				}{{"Posting", 130}, {"Konten", 0}, {"Tahap", 160}, {"PJ Humas", 120}, {"PJ desain", 120}, {"Deadline desain", 130}, {"Bukti", 100}} {
					cell := muted(c, h.label).FontSize(11).Bold()
					if h.width == 0 {
						cell.Grow(1).MinWidth(0)
					} else {
						cell.Width(h.width).Shrink(0)
					}
				}
			})
		}
		for i, item := range items {
			item := item
			stage, evidence := a.medhumStageOf(item)
			b := ui.ButtonBase(c.Key(item.ID)).Label("Buka konten "+item.Title).FillWidth().MinWidth(0).Padding(12, 16).Gap(12)
			if i > 0 {
				b.BorderWidth(1, 0, 0, 0).BorderColor(t.Border)
			}
			if b.Hovered() {
				b.Background(t.SurfaceHover)
			}
			if wide {
				b.Row().AlignItems(ui.Center)
			} else {
				b.Column().Gap(6)
			}
			b.FocusRing(b.FocusVisible()).Children(func() {
				if wide {
					ui.Text(c, medhumShort(item.PostingDate, item.PostingAt, loc)).FontSize(12).Width(130).Shrink(0).MaxLines(1)
					ui.Text(c, item.Title).Bold().Grow(1).MinWidth(0).MaxLines(2)
					ui.Row(c).Width(160).Shrink(0).Children(func() { medhumChip(c, stage) })
					ui.Text(c, item.HumasPJName).FontSize(12).Width(120).Shrink(0).MaxLines(1)
					ui.Text(c, designName(item)).FontSize(12).Width(120).Shrink(0).MaxLines(1).TextColor(t.TextMuted)
					ui.Text(c, medhumShort(item.DesignDate, item.DesignAt, loc)).FontSize(12).Width(130).Shrink(0).MaxLines(1)
					ev := ui.Text(c, evidence).FontSize(12).Width(100).Shrink(0).MaxLines(1).TextColor(t.TextMuted)
					if stage == medhumReview {
						ev.TextColor(t.Warning)
					}
					return
				}
				ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Start).Children(func() {
					ui.Text(c, item.Title).Bold().Grow(1).MinWidth(0).MaxLines(2)
					medhumChip(c, stage)
				})
				muted(c, "Posting "+medhumShort(item.PostingDate, item.PostingAt, loc)+" · desain "+medhumShort(item.DesignDate, item.DesignAt, loc))
				muted(c, "Humas: "+item.HumasPJName+" · Desain: "+designName(item)).MaxLines(1)
			})
			if b.Clicked() {
				a.beginMedhum(&item)
			}
		}
	})
	muted(c, "Tahap: konsep Humas · desain Mediatek · follow-up H-4 · publikasi. Klik baris untuk detail.")
}

type medhumCalEntry struct {
	kind string // post, design, follow
	item api.MedhumContent
}

func (a *App) medhumCalendar(c *ui.Context, phone bool) {
	t := c.Theme()
	loc := a.medhumZone()
	now := c.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	if a.medhum.month.IsZero() {
		a.medhum.month = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	}
	if a.medhum.day.IsZero() {
		a.medhum.day = today
	}
	month := a.medhum.month
	entries := map[string][]medhumCalEntry{}
	add := func(day time.Time, kind string, item api.MedhumContent) {
		k := day.Format(time.DateOnly)
		entries[k] = append(entries[k], medhumCalEntry{kind, item})
	}
	for _, item := range a.medhumSorted() {
		if d, ok := medhumDay(item.PostingDate, item.PostingAt, loc); ok {
			add(d, "post", item)
			if item.PublishedAt == nil {
				add(d.AddDate(0, 0, -4), "follow", item)
			}
		}
		if d, ok := medhumDay(item.DesignDate, item.DesignAt, loc); ok {
			add(d, "design", item)
		}
	}
	kindColor := map[string]ui.Color{"post": t.Accent, "design": ui.Hex("#c98a7a"), "follow": ui.Hex("#a2bbd2")}
	kindName := map[string]string{"post": "Posting", "design": "Desain H-2", "follow": "Follow-up H-4"}
	ui.Column(c.Key("medhum-cal")).FillWidth().MinWidth(0).Padding(16).Gap(14).Radius(11).Background(t.Surface).Border(1, t.Border).Children(func() {
		head := ui.Column(c).FillWidth().Gap(10)
		if !phone {
			head.Row().AlignItems(ui.Center)
		}
		head.Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Grow(1).Children(func() {
				if spaceIconButton(c, "Bulan sebelumnya", "left").Clicked() {
					a.medhum.month = month.AddDate(0, -1, 0)
				}
				ui.Text(c, monthName(month.Month())+" "+fmt.Sprint(month.Year())).FontSize(18).Bold().Padding(0, 4)
				if spaceIconButton(c, "Bulan berikutnya", "right").Clicked() {
					a.medhum.month = month.AddDate(0, 1, 0)
				}
				if spaceButton(c, "Hari ini", "", false).MinHeight(30).Padding(4, 10).Clicked() {
					a.medhum.month = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
					a.medhum.day = today
				}
			})
			ui.Row(c).Wrap().Gap(12).Children(func() {
				for _, k := range []string{"post", "design", "follow"} {
					ui.Row(c.Key(k)).Gap(5).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(8, 8).Radius(2).Background(kindColor[k])
						muted(c, map[string]string{"post": "Jadwal posting", "design": "Deadline desain H-2", "follow": "Follow-up H-4"}[k]).FontSize(11)
					})
				}
			})
		})
		start := month.AddDate(0, 0, -((int(month.Weekday()) + 6) % 7))
		ui.Grid(c.Key("grid")).FillWidth().Columns(7).Gap(4).Children(func() {
			for _, name := range []string{"Sen", "Sel", "Rab", "Kam", "Jum", "Sab", "Min"} {
				muted(c.Key("h"+name), name).FontSize(11).Padding(2, 4)
			}
			for i := 0; i < 42; i++ {
				day := start.AddDate(0, 0, i)
				if i >= 35 && day.Month() != month.Month() {
					break
				}
				list := entries[day.Format(time.DateOnly)]
				cell := ui.ButtonBase(c.Key(i)).Label(fmt.Sprintf("%d %s, %d agenda", day.Day(), monthName(day.Month()), len(list))).Column().AlignItems(ui.Stretch).Justify(ui.Start).MinWidth(0).Gap(3).Padding(6).Radius(8).Border(1, t.Border)
				if phone {
					cell.MinHeight(52)
				} else {
					cell.MinHeight(104)
				}
				if day.Month() != month.Month() {
					cell.Opacity(.45)
				}
				if day.Equal(a.medhum.day) {
					cell.BorderColor(t.Accent)
				}
				if cell.Hovered() {
					cell.Background(t.SurfaceHover)
				}
				cell.FocusRing(cell.FocusVisible()).Children(func() {
					n := ui.Text(c, fmt.Sprint(day.Day())).FontSize(12)
					if day.Equal(today) {
						n.Bold().TextColor(t.Accent)
					}
					if phone {
						ui.Row(c).Gap(3).Wrap().Children(func() {
							for j, e := range list {
								if j < 4 {
									ui.Box(c.Key(j)).Size(6, 6).Radius(3).Background(kindColor[e.kind])
								}
							}
						})
						return
					}
					for j, e := range list {
						if j == 3 {
							muted(c, fmt.Sprintf("+%d lagi", len(list)-3)).FontSize(10)
							break
						}
						ui.Row(c.Key(j)).FillWidth().MinWidth(0).Gap(4).Padding(2, 4).Radius(4).Background(t.Background).AlignItems(ui.Center).Children(func() {
							ui.Box(c).Size(3, 12).Radius(2).Background(kindColor[e.kind]).Shrink(0)
							ui.Text(c, e.item.Title).FontSize(10).MaxLines(1).Grow(1).MinWidth(0)
						})
					}
				})
				if cell.Clicked() {
					a.medhum.day = day
				}
			}
		})
	})
	day := a.medhum.day
	list := entries[day.Format(time.DateOnly)]
	pageSection(c.Key("medhum-day"), whenText(day.Format(time.DateOnly), ""), "Agenda konten pada hari ini.", func() {
		if len(list) == 0 {
			muted(c, "Tidak ada posting, deadline desain, atau follow-up.")
		}
		for j, e := range list {
			e := e
			b := ui.ButtonBase(c.Key(j)).Label("Buka konten "+e.item.Title).FillWidth().Row().Gap(10).Padding(10).Radius(8).Border(1, t.Border).AlignItems(ui.Center)
			if b.Hovered() {
				b.Background(t.SurfaceHover)
			}
			b.FocusRing(b.FocusVisible()).Children(func() {
				ui.Box(c).Size(4, 28).Radius(2).Background(kindColor[e.kind]).Shrink(0)
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, e.item.Title).Bold().MaxLines(1)
					muted(c, kindName[e.kind]+" · Humas "+e.item.HumasPJName).MaxLines(1)
				})
				stage, _ := a.medhumStageOf(e.item)
				medhumChip(c, stage)
			})
			if b.Clicked() {
				item := e.item
				a.beginMedhum(&item)
			}
		}
	})
}

// clockModalField edits HH:MM with the analog clock dialog used by every
// time input. Empty means not set.
func (a *App) clockModalField(c *ui.Context, key, label string, value *string, disabled bool) bool {
	t := c.Theme()
	st := a.when(key)
	before := *value
	text := "Belum ditetapkan"
	if at, err := time.Parse("15:04", *value); err == nil {
		text = at.Format("15.04")
	}
	b := ui.ButtonBase(c.Key(key)).Label(label+": "+text).Row().Gap(8).AlignItems(ui.Center).MinHeight(38).Padding(0, 12).Radius(9).Background(t.Background).Border(1, t.Border).Disabled(disabled)
	if st.open || b.Hovered() && !disabled {
		b.BorderColor(t.Accent)
	}
	if disabled {
		b.Opacity(.6)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		ui.Icon(c, spaceIcon("clock")).Size(16, 16).TextColor(t.TextMuted)
		ui.Text(c, text).MaxLines(1)
	})
	if b.Clicked() && !disabled {
		st.open, st.minutes = true, false
	}
	a.whenDialog(c, key, label, st, func(bool) {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() { analogClock(c, key, value, &st.minutes) })
	}, func() {
		if spaceButton(c, "Kosongkan", "close", false).Clicked() {
			*value = ""
		}
	})
	return *value != before
}

func (a *App) medhumPolicyPanel(c *ui.Context) {
	t := c.Theme()
	sp := a.selectedSpace()
	if sp == nil {
		return
	}
	canPost, canDesign := false, false
	for _, d := range sp.Divisions {
		if d.Slug == "humas" {
			canPost = a.spacePermissionsForDivision(d.ID, api.CapTasksManage)
		}
		if d.Slug == "mediatek" {
			canDesign = a.spacePermissionsForDivision(d.ID, api.CapTasksManage)
		}
	}
	off := a.offlineMode() || a.medhum.busy
	pageSection(c.Key("medhum-policy"), "Aturan jam Medhum", "Dipakai saat sumber hanya punya tanggal tanpa jam.", func() {
		rule := func(label, detail string, right func()) {
			r := ui.Column(c.Key(label)).FillWidth().Gap(8).Padding(12, 0).BorderWidth(0, 0, 1, 0).BorderColor(t.Border)
			if !isMobile(c) {
				r.Row().AlignItems(ui.Center)
			}
			r.Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
					ui.Text(c, label).Bold()
					muted(c, detail)
				})
				right()
			})
		}
		rule("Jam posting bawaan bila kosong", "Diatur Humas. Kosong: aturan berbasis jam tidak dinilai otomatis.", func() {
			if a.clockModalField(c, "medhum-posting-clock", "Jam posting bawaan", &a.medhum.postingClock, !canPost || off) {
				a.medhum.policyDirty = true
			}
		})
		rule("Deadline desain Mediatek", "H-2 dari jadwal posting (SOP 7.11) · dihitung sekali, tidak dihitung ulang. Jamnya diatur Mediatek.", func() {
			if a.clockModalField(c, "medhum-design-clock", "Batas penyerahan desain", &a.medhum.designClock, !canDesign || off) {
				a.medhum.policyDirty = true
			}
		})
		rule("Batas konsep Humas", "H-7 dari jadwal posting (SOP 7.5).", func() { muted(c, "H-7") })
		rule("Follow-up bot Humas", "H-4 dari jadwal posting (SOP 7.14). Jam follow-up diatur pada kebijakan Evaluasi SOP Humas.", func() {
			if spaceButton(c, "Buka Evaluasi SOP", "shield", false).Clicked() {
				a.navigate(PathEvaluasi)
			}
		})
		muted(c, "Humas mengubah jam posting, Mediatek mengubah deadline desain. Tanggal asli dari sumber tetap disimpan tanpa jam; riwayat perubahan disimpan.")
		ui.Row(c).Gap(8).Children(func() {
			if primarySpaceButton(c, "Simpan aturan jam", "check").Disabled(off || !a.medhum.policyDirty || !canPost && !canDesign).Clicked() {
				a.saveMedhumPolicy(canPost, canDesign)
			}
		})
	})
}

// medhumDialog shows a content item and its form. Desktop: a centered
// modal; phones: a full-height sheet.
func (a *App) medhumDialog(c *ui.Context) {
	d := a.medhum.draft
	if d == nil {
		return
	}
	t := c.Theme()
	w, h := c.Size()
	phone := w < mobileMaxWidth
	loc := a.medhumZone()
	var item *api.MedhumContent
	for i := range a.medhum.items {
		if a.medhum.items[i].ID == d.id && d.id != id.Nil {
			item = &a.medhum.items[i]
		}
	}
	title := "Konten baru"
	if item != nil {
		title = item.Title
	}
	open := true
	ui.DialogBase(c.Key("medhum-dialog"), &open, func(backdrop, panel ui.Element) {
		dialogMotion(backdrop, panel)
		backdrop.Background(ui.RGBA(0, 0, 0, .45))
		if phone {
			backdrop.Justify(ui.End)
			panel.Width(w).MaxHeight(h-24).Radius(18, 18, 0, 0)
		} else {
			panel.Width(min(float32(720), w-48)).MaxHeight(h - 64).Radius(16)
		}
		panel.Column().Background(t.Surface).Border(1, t.Border).Children(func() {
			ui.Row(c).FillWidth().Padding(14, 18).Gap(10).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				ui.Text(c, title).FontSize(17).Bold().Grow(1).MinWidth(0).MaxLines(1)
				if item != nil {
					stage, _ := a.medhumStageOf(*item)
					medhumChip(c, stage)
				}
				if spaceIconButton(c, "Tutup konten", "close").Clicked() {
					open = false
				}
			})
			ui.Scroll(c.Key("medhum-dialog-scroll")).Grow(1).MinHeight(0).FillWidth().Children(func() {
				ui.Column(c).FillWidth().Padding(18).Gap(14).Children(func() {
					if a.medhum.err != "" {
						pageNotice(c, "alert", "Belum tersimpan", a.medhum.err)
					}
					if item != nil {
						a.medhumFacts(c, *item, loc, &open)
					}
					a.medhumFields(c, d)
				})
			})
			ui.Row(c).FillWidth().Padding(12, 18).Gap(8).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
				ui.Box(c).Grow(1)
				if spaceButton(c, "Batal", "", false).Disabled(a.medhum.busy).Clicked() {
					open = false
				}
				editable := d.calendar || d.assignment
				if primarySpaceButton(c, "Simpan konten", "check").Disabled(!editable || a.medhum.busy || a.offlineMode()).Clicked() {
					a.medhum.err = ""
					a.saveMedhum()
				}
			})
		})
	})
	if !open && !a.medhum.busy {
		a.medhum.draft = nil
		a.medhum.err = ""
	}
}

func (a *App) medhumFacts(c *ui.Context, item api.MedhumContent, loc *time.Location, open *bool) {
	t := c.Theme()
	_, evidence := a.medhumStageOf(item)
	design := item.DesignPJName
	if item.DesignPJID == nil {
		design = "Belum ditetapkan kadiv Mediatek"
	}
	facts := [][2]string{
		{"Jadwal posting", medhumShort(item.PostingDate, item.PostingAt, loc)},
		{"Deadline desain", medhumShort(item.DesignDate, item.DesignAt, loc)},
		{"PJ Humas", item.HumasPJName},
		{"PJ desain", design},
		{"Bukti desain", evidence},
	}
	if item.ConceptSubmittedAt != nil {
		facts = append(facts, [2]string{"Konsep diserahkan", taskTimestamp(*item.ConceptSubmittedAt, loc.String())})
	}
	if item.PublishedAt != nil {
		facts = append(facts, [2]string{"Publikasi", taskTimestamp(*item.PublishedAt, loc.String())})
	}
	cols := 2
	if isMobile(c) {
		cols = 1
	}
	ui.Grid(c.Key("medhum-facts")).FillWidth().Columns(cols).Gap(10).Padding(14).Radius(10).Background(t.Background).Border(1, t.Border).Children(func() {
		for _, f := range facts {
			ui.Column(c.Key(f[0])).MinWidth(0).Gap(2).Children(func() {
				muted(c, f[0]).FontSize(11)
				ui.Text(c, f[1]).FontSize(13).MaxLines(2)
			})
		}
	})
	ui.Row(c).Wrap().Gap(8).Children(func() {
		if item.HumasTaskID != nil && spaceButton(c, "Buka tugas Humas", "task", false).Border(1, t.Border).Clicked() {
			*open = false
			a.OpenNotification(api.Delivery{SourceKind: "task", SourceID: *item.HumasTaskID, SpaceID: &item.SpaceID})
		}
		if item.DesignTaskID != nil && spaceButton(c, "Buka tugas desain", "task", false).Border(1, t.Border).Clicked() {
			*open = false
			a.OpenNotification(api.Delivery{SourceKind: "task", SourceID: *item.DesignTaskID, SpaceID: &item.SpaceID})
		}
	})
}

func (a *App) medhumFields(c *ui.Context, d *medhumDraft) {
	t := c.Theme()
	busy := a.medhum.busy
	ui.Column(c.Key("medhum-calendar-fields")).FillWidth().Gap(12).Disabled(!d.calendar || busy).Children(func() {
		if !d.calendar {
			muted(c, "Jadwal, konsep dan PJ Humas diubah oleh Humas.")
		}
		taskInput(c, "medhum-title", "Judul konten *", &d.title)
		a.whenField(c, "medhum-posting", "Jadwal posting", &d.date, &d.clock, true)
		muted(c, "Tanpa jam bila sumber hanya punya tanggal. Deadline desain H-2 dihitung sekali dari tanggal ini.").FontSize(11)
		taskInput(c, "medhum-concept", "Tautan konsep", &d.concept)
		taskLongInput(c, "medhum-caption", "Caption", &d.caption)
		taskInput(c, "medhum-trello", "Kartu Trello · opsional", &d.trello)
		fieldLabel(c, "PJ Humas", true)
		ui.Row(c).Wrap().Gap(6).Children(func() {
			for _, person := range a.medhum.humans {
				person := person
				if spaceButton(c.Key(person.UserID), person.DisplayName, "user", person.UserID == d.humas).Border(1, t.Border).Clicked() {
					d.humas = person.UserID
				}
			}
			if len(a.medhum.humans) == 0 {
				muted(c, "Daftar anggota Humas tidak tersedia untuk akunmu.")
			}
		})
	})
	if d.assignment {
		fieldLabel(c, "PJ desain · ditetapkan kadiv Mediatek", false)
		ui.Row(c.Key("medhum-design")).Wrap().Gap(6).Disabled(busy).Children(func() {
			for _, person := range a.medhum.designers {
				person := person
				if spaceButton(c.Key(person.UserID), person.DisplayName, "user", person.UserID == d.design).Border(1, t.Border).Clicked() {
					d.design = person.UserID
				}
			}
		})
	}
	if d.id != id.Nil && (d.calendar || d.assignment) {
		taskInput(c, "medhum-reason", "Alasan perubahan", &d.reason)
	}
}
