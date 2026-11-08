package appui

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
)

// SpaceTabPreference is one account's tab bar for one Space: the order of
// the tabs, by key, and the ones it hides. Keys it does not know are kept,
// so a tab the account may not open now keeps its place.
type SpaceTabPreference struct {
	Order  []string `json:"order,omitempty"`
	Hidden []string `json:"hidden,omitempty"`
}

// SpaceTabLayouts are the tab bars saved on this device, by "user/space".
type SpaceTabLayouts map[string]SpaceTabPreference

// spaceTabDef is a tab of the Space tab bar. Title is what a.chrome.tab
// holds, as other pages switch tabs by title.
type spaceTabDef struct{ key, title, icon string }

// spaceTabCatalog is canvas A's tab bar, in its default order.
var spaceTabCatalog = []spaceTabDef{
	{"beranda", "Beranda", "beranda"},
	{"board", "Board tugas", "board"},
	{"agenda", "Agenda", "agenda"},
	{"catatan", "Catatan", "catatan"},
	{"rapat", "Rapat", "rapat"},
	{"tinjauan", "Tinjauan", "tinjauan"},
	{"ping", "Ping", "ping"},
	{"anggota", "Anggota", "anggota"},
	{"bot", "Bot WhatsApp", "bot"},
}

// spaceTabKonten is the Medhum content page, first in the Medhum space.
var spaceTabKonten = spaceTabDef{"konten", medhumSpaceTab, "konten"}

// spaceTabAliases maps tab titles older entry points still use.
var spaceTabAliases = map[string]string{"Notes": "Catatan", "Pengaturan": "Bot WhatsApp"}

// spaceTabIcons are drawn from canvas A's ICON map, stroked as its svg.i.
var spaceTabIcons = func() map[string]*ui.SVG {
	paths := map[string]string{
		"beranda":  `<path d="M3 11 12 4l9 7v9H3Z"/>`,
		"board":    `<rect x="3" y="4" width="5" height="16" rx="1"/><rect x="10" y="4" width="5" height="10" rx="1"/><rect x="17" y="4" width="4" height="13" rx="1"/>`,
		"agenda":   `<rect x="3" y="5" width="18" height="16" rx="2"/><path d="M3 10h18M8 3v4M16 3v4"/>`,
		"catatan":  `<path d="M6 3h9l4 4v14H6Z"/><path d="M9 12h6M9 16h6"/>`,
		"rapat":    `<circle cx="9" cy="8" r="3"/><path d="M3 20c0-3 3-5 6-5s6 2 6 5"/><circle cx="17" cy="9" r="2.5"/><path d="M15 15c3 0 6 1.5 6 4"/>`,
		"bot":      `<path d="M4 20l1.5-4A8 8 0 1 1 9 19.5Z"/>`,
		"tinjauan": `<path d="M9 11l3 3 8-8"/><circle cx="10" cy="12" r="8"/>`,
		"anggota":  `<circle cx="9" cy="8" r="3"/><path d="M3 20c0-3 3-5 6-5s6 2 6 5"/><path d="M16 4a3 3 0 0 1 0 6M21 20c0-2.5-1.5-4.3-4-5"/>`,
		"ping":     `<path d="M3 11v2a1 1 0 0 0 1 1h2l5 4V6L6 10H4a1 1 0 0 0-1 1Z"/><path d="M15 9a4 4 0 0 1 0 6M18 6a8 8 0 0 1 0 12"/>`,
		"konten":   `<rect x="3" y="5" width="18" height="16" rx="2"/><path d="M3 10h18M8 3v4M16 3v4M8 15h3"/>`,
		"grip":     `<circle cx="9" cy="6" r="1"/><circle cx="15" cy="6" r="1"/><circle cx="9" cy="12" r="1"/><circle cx="15" cy="12" r="1"/><circle cx="9" cy="18" r="1"/><circle cx="15" cy="18" r="1"/>`,
		"atur":     `<path d="M4 6h10M4 12h16M4 18h7"/><circle cx="18" cy="6" r="2"/><circle cx="15" cy="18" r="2"/>`,
		"check":    `<path d="m5 12 4 4L19 6"/>`,
	}
	icons := make(map[string]*ui.SVG, len(paths))
	for name, path := range paths {
		icons[name] = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">` + path + `</svg>`))
	}
	return icons
}()

// spaceTabDrag is the value a tab drags to another to reorder them.
type spaceTabDrag struct{ key string }

type spaceTabsUIState struct {
	menuOpen bool
	// reviewFilter is the Tinjauan page's filter: a task match status.
	reviewFilter string
}

// spaceTabFixed reports whether a tab cannot be hidden: the space's home.
func spaceTabFixed(key string) bool { return key == "beranda" || key == "konten" }

// spaceTabAllowed reports whether the account may open the tab in sp.
func spaceTabAllowed(sp *api.Space, key string) bool {
	if sp == nil {
		return key == "beranda"
	}
	switch key {
	case "konten":
		return sp.Code == "medhum"
	case "tinjauan":
		return !sp.ReadOnly && slices.Contains(sp.Capabilities, api.CapResultsReview)
	case "ping":
		return canPing(sp)
	case "bot":
		return slices.ContainsFunc(sp.Roles, func(r api.Access) bool { return slices.Contains(r.Capabilities, api.CapTasksOwn) })
	}
	return true
}

func (a *App) spaceTabPreferenceKey(sp *api.Space) string {
	if sp == nil {
		return ""
	}
	return a.user.ID.String() + "/" + sp.ID.String()
}

// spaceTabPreference returns the account's saved tab bar for sp.
func (a *App) spaceTabPreference(sp *api.Space) SpaceTabPreference {
	if a.chrome.preferences.SpaceTabs == nil {
		return SpaceTabPreference{}
	}
	return (*a.chrome.preferences.SpaceTabs)[a.spaceTabPreferenceKey(sp)]
}

// spaceTabsAll returns the tabs the account may open in sp, in its order,
// hidden ones too.
func (a *App) spaceTabsAll(sp *api.Space) []spaceTabDef {
	catalog := spaceTabCatalog
	if sp != nil && sp.Code == "medhum" {
		catalog = append([]spaceTabDef{spaceTabKonten}, spaceTabCatalog...)
	}
	allowed := make([]spaceTabDef, 0, len(catalog))
	for _, tab := range catalog {
		if spaceTabAllowed(sp, tab.key) {
			allowed = append(allowed, tab)
		}
	}
	return orderSpaceTabs(allowed, a.spaceTabPreference(sp).Order)
}

// orderSpaceTabs puts tabs in the saved order; tabs it lacks keep their
// default place among the others, after the saved tab before them.
func orderSpaceTabs(tabs []spaceTabDef, order []string) []spaceTabDef {
	if len(order) == 0 {
		return tabs
	}
	rank := map[string]int{}
	for _, key := range order {
		if _, ok := rank[key]; !ok {
			rank[key] = len(rank)
		}
	}
	out := make([]spaceTabDef, 0, len(tabs))
	var unranked []spaceTabDef
	for _, tab := range tabs {
		if _, ok := rank[tab.key]; ok {
			out = append(out, tab)
		} else {
			unranked = append(unranked, tab)
		}
	}
	slices.SortStableFunc(out, func(x, y spaceTabDef) int { return rank[x.key] - rank[y.key] })
	// A tab new to the saved order goes after the tab before it by default.
	for _, tab := range unranked {
		at := 0
		for i, def := range tabs {
			if def.key == tab.key {
				for j := i - 1; j >= 0; j-- {
					if k := slices.IndexFunc(out, func(t spaceTabDef) bool { return t.key == tabs[j].key }); k >= 0 {
						at = k + 1
						break
					}
				}
				break
			}
		}
		out = slices.Insert(out, at, tab)
	}
	return out
}

// spaceTabsVisible returns the tabs the bar shows in sp.
func (a *App) spaceTabsVisible(sp *api.Space) []spaceTabDef {
	hidden := a.spaceTabPreference(sp).Hidden
	all := a.spaceTabsAll(sp)
	out := all[:0:0]
	for _, tab := range all {
		if spaceTabFixed(tab.key) || !slices.Contains(hidden, tab.key) {
			out = append(out, tab)
		}
	}
	return out
}

// moveSpaceTabKey moves from onto to: after it when it came from before
// it, as dragging right does, before it otherwise.
func moveSpaceTabKey(order []string, from, to string) []string {
	i, j := slices.Index(order, from), slices.Index(order, to)
	if i < 0 || j < 0 || i == j {
		return order
	}
	out := slices.Delete(slices.Clone(order), i, i+1)
	j = slices.Index(out, to)
	if i < slices.Index(order, to) {
		j++
	}
	return slices.Insert(out, j, from)
}

// setSpaceTabPreference stores p for sp and saves the preferences.
func (a *App) setSpaceTabPreference(c *ui.Context, sp *api.Space, p SpaceTabPreference) {
	key := a.spaceTabPreferenceKey(sp)
	if key == "" {
		return
	}
	tabs := SpaceTabLayouts{}
	if old := a.chrome.preferences.SpaceTabs; old != nil {
		for k, v := range *old {
			tabs[k] = v
		}
	}
	tabs[key] = p
	a.chrome.preferences.SpaceTabs = &tabs
	a.savePreferences(c)
}

// moveSpaceTab reorders the bar of sp by a drop of from onto to.
func (a *App) moveSpaceTab(c *ui.Context, sp *api.Space, from, to string) {
	if from == to {
		return
	}
	p := a.spaceTabPreference(sp)
	var order []string
	for _, tab := range a.spaceTabsAll(sp) {
		order = append(order, tab.key)
	}
	moved := moveSpaceTabKey(order, from, to)
	if slices.Equal(moved, order) {
		return
	}
	// Keep the place of tabs saved earlier that the account cannot open now.
	for _, key := range p.Order {
		if !slices.Contains(moved, key) {
			moved = append(moved, key)
		}
	}
	p.Order = moved
	a.setSpaceTabPreference(c, sp, p)
	if a.chrome.preferencesErr == "" {
		c.AddToast(ui.Toast{Title: "Urutan tab disimpan", Type: "success", Timeout: 3 * time.Second})
	}
}

// setSpaceTabHidden hides or shows a tab of sp.
func (a *App) setSpaceTabHidden(c *ui.Context, sp *api.Space, key string, hidden bool) {
	if spaceTabFixed(key) {
		return
	}
	p := a.spaceTabPreference(sp)
	p.Hidden = slices.DeleteFunc(slices.Clone(p.Hidden), func(k string) bool { return k == key })
	if hidden {
		p.Hidden = append(p.Hidden, key)
	}
	a.setSpaceTabPreference(c, sp, p)
}

// resolveSpaceTab maps old tab titles and leaves a tab the account may not
// open for the space's home.
func (a *App) resolveSpaceTab(sp *api.Space) {
	if title, ok := spaceTabAliases[a.chrome.tab]; ok {
		a.chrome.tab = title
	}
	for _, tab := range append([]spaceTabDef{spaceTabKonten}, spaceTabCatalog...) {
		if tab.title == a.chrome.tab {
			if !spaceTabAllowed(sp, tab.key) {
				a.chrome.tab = "Beranda"
				if sp != nil {
					a.chrome.tab = defaultSpaceTab(*sp)
				}
			}
			return
		}
	}
}

// spaceTabBar is canvas A's tab row: text and icon tabs, an accent ink
// under the open one that slides to the next, tabs that drag to reorder,
// and Atur tab at the end.
func (a *App) spaceTabBar(c *ui.Context, sp *api.Space) {
	t := c.Theme()
	mobile := isMobile(c)
	pad := float32(24)
	if mobile {
		pad = 12
	}
	all := a.spaceTabsAll(sp)
	tabs := a.spaceTabsVisible(sp)
	ui.Row(c.Key("space-tab-bar")).FillWidth().Height(46).Shrink(0).Background(t.Background).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).AlignItems(ui.Center).Children(func() {
		ui.ScrollHorizontal(c.Key("space-tabs")).Grow(1).MinWidth(0).FillHeight().Label("Tab space").Children(func() {
			list := ui.Row(c.Key("space-tab-list")).FillHeight().Padding(0, pad).Gap(2).AlignItems(ui.End)
			list.Children(func() {
				buttons := make(map[string]ui.Element, len(tabs))
				for i, tab := range tabs {
					buttons[tab.title] = a.spaceTabButton(c, sp, tabs, i)
				}
				active, ok := buttons[a.chrome.tab]
				if !ok {
					return
				}
				ab, lb := active.Bounds(), list.Bounds()
				if ab.W <= 0 || lb.W <= 0 {
					c.Invalidate()
					return
				}
				x := list.Animate("ink-x", ab.X-lb.X, 280*time.Millisecond)
				w := list.Animate("ink-w", ab.W, 280*time.Millisecond)
				ui.Box(c.Key("space-tab-ink")).Absolute().Left(x).Bottom(0).Width(w).Height(2).Radius(2).Background(t.Accent)
			})
		})
		a.spaceTabMenu(c, sp, all, mobile)
	})
}

func (a *App) spaceTabButton(c *ui.Context, sp *api.Space, tabs []spaceTabDef, index int) ui.Element {
	t := c.Theme()
	tab := tabs[index]
	on := a.chrome.tab == tab.title
	b := ui.ButtonBase(c.Key("space-tab-"+tab.key)).Label(tab.title).Row().AlignItems(ui.Center).Gap(6).Height(38).Padding(0, 10).Radius(8, 8, 0, 0).Shrink(0).TextColor(t.TextMuted)
	hovered := b.Hovered()
	if on {
		b.TextColor(t.Text)
	}
	if hovered {
		b.Background(t.SurfaceHover).TextColor(t.Text)
	}
	b.FocusRing(b.FocusVisible())
	// Dragging starts only once the press moves a few DIPs, so a click
	// stays a click. On touch the web loader turns a quick swipe into
	// scrolling and leaves a long press to drag.
	b.Drag(spaceTabDrag{tab.key})
	dragging := b.Dragging()
	if dragging {
		b.Opacity(.4)
	}
	marker := 0
	if v, ok := ui.DragOver[spaceTabDrag](b); ok && v.key != tab.key {
		marker = -1
		if from := slices.IndexFunc(tabs, func(d spaceTabDef) bool { return d.key == v.key }); from >= 0 && from < index {
			marker = 1
		}
	}
	if v, ok := ui.Drop[spaceTabDrag](b); ok && v.key != tab.key {
		a.moveSpaceTab(c, sp, v.key, tab.key)
	}
	b.Children(func() {
		grip := float32(0)
		if hovered || dragging {
			grip = 1
		}
		ui.Icon(c, spaceTabIcons["grip"]).Size(12, 12).Shrink(0).Opacity(grip).TextColor(t.TextMuted)
		ui.Icon(c, spaceTabIcons[tab.icon]).Size(16, 16).Shrink(0)
		ui.Text(c, tab.title).MaxLines(1)
		if marker != 0 {
			m := ui.Box(c.Key("drop-marker")).Absolute().Top(8).Bottom(8).Width(2).Radius(1).Background(t.Accent)
			if marker < 0 {
				m.Left(0)
			} else {
				m.Right(0)
			}
		}
	})
	if b.Clicked() {
		a.chrome.tab = tab.title
	}
	return b
}

// spaceTabMenu is Atur tab: a popover that shows or hides each tab.
func (a *App) spaceTabMenu(c *ui.Context, sp *api.Space, tabs []spaceTabDef, mobile bool) {
	t := c.Theme()
	st := &a.spaceTabs
	b := ui.ButtonBase(c.Key("space-tab-menu")).Label("Atur tab").Tooltip("Atur tab").Row().AlignItems(ui.Center).Gap(8).Height(34).Padding(0, 12).Margin(0, 16, 0, 4).Radius(8).Shrink(0).TextColor(t.TextMuted)
	if mobile {
		b.Width(34).Padding(0).Center().Margin(0, 8, 0, 0)
	}
	if b.Hovered() || st.menuOpen {
		b.Background(t.SurfaceHover).TextColor(t.Text)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		ui.Icon(c, spaceTabIcons["atur"]).Size(16, 16).Shrink(0)
		if !mobile {
			ui.Text(c, "Atur tab").MaxLines(1)
		}
	})
	if b.Clicked() {
		st.menuOpen = !st.menuOpen
	}
	popover(c, b, &st.menuOpen, 260, func() {
		muted(c, "Tampilkan tab · geser tab untuk mengubah urutan").FontSize(12).Padding(0, 4, 4, 4)
		hidden := a.spaceTabPreference(sp).Hidden
		for _, tab := range tabs {
			fixed := spaceTabFixed(tab.key)
			shown := fixed || !slices.Contains(hidden, tab.key)
			row := ui.CheckboxBase(c.Key("space-tab-show-"+tab.key), &shown).Label("Tampilkan "+tab.title).FillWidth().Row().AlignItems(ui.Center).Gap(10).Padding(7, 8).Radius(7).Disabled(fixed)
			if row.Hovered() && !fixed {
				row.Background(t.SurfaceHover)
			}
			row.FocusRing(row.FocusVisible()).Children(func() {
				ui.Icon(c, spaceTabIcons[tab.icon]).Size(16, 16).Shrink(0).TextColor(t.TextMuted)
				ui.Text(c, tab.title).Grow(1).MinWidth(0).MaxLines(1)
				box := ui.Box(c).Size(16, 16).Shrink(0).Radius(4).Border(1, t.Border).Center()
				if shown {
					box.Background(t.Accent).BorderColor(t.Accent)
				}
				if fixed {
					box.Opacity(.5)
				}
				box.Children(func() {
					if shown {
						ui.Icon(c, spaceTabIcons["check"]).Size(12, 12).TextColor(t.Background)
					}
				})
			})
			if row.Changed() && !fixed {
				a.setSpaceTabHidden(c, sp, tab.key, !shown)
			}
		}
		muted(c, "Urutan dan tab yang tampil disimpan untuk akunmu di perangkat ini. Beranda selalu tampil.").FontSize(11).Padding(6, 4, 0, 4)
	})
}

// spaceReviewPage is Tinjauan: results handed in on this Space's tasks, to
// match with their task. Matching itself, with its required reason, stays
// in the task's detail.
func (a *App) spaceReviewPage(c *ui.Context) {
	a.ensureTasks(false)
	t := c.Theme()
	st := &a.spaceTabs
	filter := st.reviewFilter
	if filter == "" {
		filter = api.MatchPending
	}
	counts := map[string]int{}
	var shown []api.Task
	for _, task := range a.tasks.items {
		if len(task.Evidence) == 0 || task.Progress == api.TaskCancelled {
			continue
		}
		counts[task.MatchStatus]++
		if task.MatchStatus == filter {
			shown = append(shown, task)
		}
	}
	ui.Scroll(c.Key("space-review")).Fill().Children(func() {
		body := ui.Column(c).FillWidth().MinWidth(0).MaxWidth(1280).Padding(26).Gap(18)
		if isMobile(c) {
			body.Padding(16).Gap(14)
		}
		wide := body.Bounds().W >= 760
		body.Children(func() {
			head := ui.Column(c).FillWidth().MinWidth(0).Gap(12)
			if wide {
				head.Row().AlignItems(ui.Center)
			}
			head.Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
					ui.Text(c, "Tinjauan hasil").FontSize(20).Bold()
					muted(c, "Cocokkan bukti yang masuk ke tugasnya. Ini bukan revisi desain: hanya memastikan bukti milik tugas yang benar.")
				})
				ui.Row(c).Gap(2).Padding(3).Radius(10).Border(1, t.Border).Shrink(0).Children(func() {
					for _, f := range []struct{ key, label string }{{api.MatchPending, "Perlu tinjauan"}, {api.MatchVerified, "Cocok"}, {api.MatchRejected, "Tidak cocok"}} {
						label := f.label
						if f.key == api.MatchPending {
							label += " · " + strconv.Itoa(counts[f.key])
						}
						b := ui.ButtonBase(c.Key("review-"+f.key)).Label(label).Padding(6, 12).Radius(7).TextColor(t.TextMuted)
						if filter == f.key {
							b.Background(t.Accent).TextColor(t.Background)
						} else if b.Hovered() {
							b.Background(t.SurfaceHover).TextColor(t.Text)
						}
						b.FocusRing(b.FocusVisible()).Children(func() { ui.Text(c, label).FontSize(12).MaxLines(1) })
						if b.Clicked() {
							st.reviewFilter = f.key
						}
					}
				})
			})
			a.taskFeedback(c)
			if a.tasks.err != "" {
				pageNotice(c, "alert", "Tugas belum dapat dimuat", a.tasks.err)
				return
			}
			if !a.tasks.loaded {
				muted(c, "Memuat tugas…")
				return
			}
			if len(shown) == 0 {
				empty := map[string][2]string{
					api.MatchPending:  {"Tidak ada bukti yang perlu ditinjau", "Hasil yang diserahkan lewat Space atau WhatsApp muncul di sini sampai dicocokkan."},
					api.MatchVerified: {"Belum ada bukti yang cocok", "Bukti yang sudah dicocokkan dengan tugasnya muncul di sini."},
					api.MatchRejected: {"Belum ada bukti yang tidak cocok", "Bukti yang ditandai bukan milik tugasnya muncul di sini."},
				}[filter]
				pageEmpty(c, "check", empty[0], empty[1])
				return
			}
			ui.Column(c).FillWidth().MinWidth(0).Radius(12).Background(t.Surface).Border(1, t.Border).Clip().Children(func() {
				for i, task := range shown {
					a.spaceReviewRow(c, task, i > 0, wide)
				}
			})
		})
	})
}

func (a *App) spaceReviewRow(c *ui.Context, task api.Task, divider, wide bool) {
	t := c.Theme()
	e := task.Evidence[len(task.Evidence)-1]
	row := ui.Column(c.Key(task.ID)).FillWidth().MinWidth(0).Padding(16).Gap(12)
	if wide {
		row.Row().AlignItems(ui.Start)
	}
	if divider {
		row.BorderWidth(1, 0, 0, 0).BorderColor(t.Border)
	}
	row.Children(func() {
		ui.Row(c).Grow(1).MinWidth(0).Gap(12).AlignItems(ui.Start).Children(func() {
			ui.Box(c).Size(24, 24).Shrink(0).Radius(12).Background(t.SurfaceHover).Center().Children(func() {
				ui.Text(c, reviewInitials(e.SubmittedBy.DisplayName)).FontSize(9).Bold()
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				ui.Text(c, task.Title).Bold().MaxLines(2)
				muted(c, "Dikirim "+e.SubmittedBy.DisplayName+" · "+taskTimestamp(e.SubmittedAt, task.TimeZone)).FontSize(12)
				proof := strings.TrimSpace(strings.TrimSpace(e.Note + " " + e.URL))
				if proof != "" {
					ui.Text(c, proof).FontSize(12).MaxLines(2).Padding(8, 10).Radius(7).Background(t.Background).FillWidth()
				}
				if e.ReviewReason != "" {
					muted(c, "Alasan: "+e.ReviewReason).FontSize(12)
				}
			})
		})
		ui.Row(c).Gap(8).Shrink(0).Children(func() {
			if e.URL != "" {
				ui.Link(c, "Buka tautan", e.URL).FontSize(12).TextColor(t.Accent).Padding(8, 4)
			}
			label := "Buka tugas"
			if task.MatchStatus == api.MatchPending && task.Permissions.Review {
				label = "Cocokkan di tugas"
			}
			if spaceButton(c.Key("open"), label, "arrow", task.MatchStatus == api.MatchPending).Clicked() {
				a.openCanonicalTask(task)
			}
		})
	})
}

func reviewInitials(name string) string {
	out := ""
	for _, part := range strings.Fields(name) {
		out += strings.ToUpper(string([]rune(part)[0]))
		if len([]rune(out)) == 2 {
			break
		}
	}
	if out == "" {
		return "?"
	}
	return out
}

// spacePingPage is the Ping tab: the ping composer of the header dialog,
// as a page. The server keeps no ping history for the app to list yet.
func (a *App) spacePingPage(c *ui.Context) {
	settingsBody(c, "Ping", "Ingatkan anggota divisi lewat perangkat atau WhatsApp.", func() {
		pageSection(c, "Kirim ping", "", func() { a.pingComposer(c) })
		pageSection(c, "Riwayat ping", "", func() {
			pageEmpty(c, "clock", "Riwayat ping belum tersedia", "Space belum menyimpan daftar ping yang sudah dikirim. Status pengiriman WhatsApp ada di Pengaturan, Aktivitas WhatsApp.")
		})
	})
}
