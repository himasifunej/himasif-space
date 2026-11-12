package appui

//go:generate go run ../../tools/quicknavgen/cmd

import (
	"runtime"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Quick navigation (Ctrl K) reaches every page, the Spaces and their tabs,
// the common actions, and every section, field, heading and button of the
// settings. The settings come from quicknav_settings_gen.go, which
// tools/quicknavgen reads from the settings pages' source; the pages from
// appPages, which the router's page switch is checked against.

// appPage is one page of the router.
type appPage struct {
	path, title, icon string
	// words are other ways to ask for the page.
	words string
	// palette says whether quick navigation lists it.
	palette bool
}

// appPages are the router's pages: the topbar titles them, quick
// navigation lists them.
var appPages = []appPage{
	{PathUntukku, "Untukku", "home", "agenda saya hari ini minggu ini beranda pribadi", true},
	{PathJobdesk, "Jobdesk saya", "task", "tugas saya", true},
	{PathReminder, "Reminder pribadi", "bell", "pengingat pribadi", true},
	{PathSpaces, "Space saya", "grid", "divisi ruang kerja", true},
	{PathMedhum, "Medhum", "humas", "media humas konten posting", false},
	{PathEvaluasi, "Evaluasi SOP", "check", "sop temuan penilaian", true},
	{PathArsip, "Arsip periode", "archive", "arsip periode lalu", true},
	{PathArchiveAccess, "Izin arsip", "shield", "akses arsip", true},
	{PathOrganisasi, "Organisasi", "shield", "admin periode kadiv serah terima", true},
	{PathPanduan, "Panduan", "book", "bantuan tutorial", true},
	{PathPengaturan, "Pengaturan", "settings", "setelan", true},
	{PathLainnya, "Lainnya", "more", "menu", false},
	// A card's page, from a link: not one to open from the palette.
	{PathTaskCard, "Tugas", "task", "", false},
	{PathMeetingCard, "Rapat", "calendar", "", false},
	// Untukku's details, from one of its rows: not from the palette.
	{PathUntukTask, "Detail tugas", "task", "", false},
	{PathUntukRapat, "Rapat", "calendar", "", false},
}

// pageTitle is the title of the page at path, "" for a Space's.
func pageTitle(path string) string {
	for _, p := range appPages {
		if p.path == path && p.path != PathSpaces && p.path != PathMedhum {
			return p.title
		}
	}
	return ""
}

// settingsSearchItem is one thing a settings tab shows, as
// tools/quicknavgen found it.
type settingsSearchItem struct {
	tab, title, section, kind, platform string
}

// navItem is one result of quick navigation.
type navItem struct {
	key          string
	title, trail string
	icon, group  string
	words        string
	rank         int // ties go to the lower rank: pages, Spaces, actions, settings
	open         func(c *ui.Context)
}

const (
	rankPage = iota
	rankSpace
	rankAction
	rankSettings
	rankSetting
	rankNote
	rankTab
	rankButton
)

// navIndex lists everything quick navigation can open for this account.
func (a *App) navIndex() []navItem {
	var out []navItem
	admin := a.isAdmin()
	for _, p := range appPages {
		if !p.palette || (p.path == PathOrganisasi && !admin) || (p.path == PathPanduan && admin) || (p.path == PathArchiveAccess && len(a.archiveManagedDivisions()) == 0) {
			continue
		}
		path := p.path
		out = append(out, navItem{key: "page" + path, title: p.title, trail: "Halaman", icon: p.icon, group: "Halaman", words: p.words, rank: rankPage, open: func(*ui.Context) {
			if path == PathSpaces {
				a.selectSpace(id.Nil)
			}
			a.navigate(path)
		}})
	}
	for i := range a.access.spaces {
		sp := a.access.spaces[i]
		out = append(out, navItem{key: "space" + sp.ID.String(), title: sp.Name, trail: "Space saya", icon: "grid", group: "Space", words: "space divisi", rank: rankSpace, open: func(*ui.Context) {
			a.selectSpace(sp.ID)
			a.navigate(PathSpaces)
		}})
		for _, tab := range a.spaceTabsAll(&sp) {
			title := tab.title
			out = append(out, navItem{key: "tab" + sp.ID.String() + tab.key, title: title, trail: "Space › " + sp.Name, icon: "board", group: "Tab", words: tab.key, rank: rankTab, open: func(*ui.Context) {
				a.selectSpace(sp.ID)
				a.navigate(PathSpaces)
				a.chrome.tab = title
			}})
		}
	}
	if admin {
		for _, tab := range organizationTabs {
			name := tab.name
			out = append(out, navItem{key: "org" + name, title: name, trail: "Organisasi", icon: tab.icon, group: "Tab", rank: rankTab, open: func(*ui.Context) {
				a.cancelOrganizationConfirmation()
				a.organization.tab = name
				a.navigate(PathOrganisasi)
			}})
		}
	}
	out = append(out, a.navActions()...)
	out = append(out, a.navSettings()...)
	for _, n := range a.notes.items {
		n := n
		out = append(out, navItem{key: "note" + n.ID.String(), title: n.Title, trail: "Catatan · " + a.waSpaceName(n.SpaceID), icon: "note", group: "Catatan", words: "catatan notes", rank: rankNote, open: func(*ui.Context) {
			a.selectSpace(n.SpaceID)
			a.navigate(PathSpaces)
			a.chrome.tab = "Catatan"
			a.openNote(n.ID)
		}})
	}
	return out
}

// navActions are the buttons quick navigation presses itself.
func (a *App) navActions() []navItem {
	action := func(key, title, icon, words string, open func(c *ui.Context)) navItem {
		return navItem{key: "act" + key, title: title, trail: "Aksi", icon: icon, group: "Aksi", words: words, rank: rankAction, open: open}
	}
	var out []navItem
	if !a.isAdmin() {
		out = append(out, action("task", "Buat tugas", "plus", "tugas baru tambah jobdesk", func(*ui.Context) { a.startTaskFromAnywhere() }))
	}
	if sp := a.pingSpace(); sp != nil {
		sid := sp.ID
		out = append(out, action("ping", "Kirim ping", "send", "ping colek ingatkan anggota", func(*ui.Context) {
			a.selectSpace(sid)
			a.chrome.pingOpen = true
		}))
	}
	if a.onboardingSpace() != nil {
		out = append(out, action("tour", "Mulai tur Space", "compass", "tur panduan pengenalan onboarding", func(*ui.Context) { a.startTour() }))
	}
	out = append(out, action("appearance", "Buka panel tampilan", "palette", "tema warna aksen kepadatan", func(*ui.Context) { a.chrome.appearance = true }))
	for _, mode := range []struct{ key, title, icon string }{{"dark", "Tema gelap", "moon"}, {"light", "Tema terang", "sun"}, {"system", "Tema ikuti sistem", "monitor"}} {
		mode := mode
		out = append(out, action("theme"+mode.key, mode.title, mode.icon, "tampilan mode "+mode.key, func(c *ui.Context) {
			a.chrome.preferences.Theme = mode.key
			a.savePreferences(c)
		}))
	}
	sidebar := "Ciutkan sidebar"
	if a.chrome.preferences.SidebarCollapsed {
		sidebar = "Buka sidebar"
	}
	out = append(out, action("sidebar", sidebar, "panel", "menu samping navigasi", func(c *ui.Context) {
		a.chrome.preferences.SidebarCollapsed = !a.chrome.preferences.SidebarCollapsed
		a.savePreferences(c)
	}))
	out = append(out, action("reload", "Muat ulang Space", "refresh", "segarkan refresh akses", func(*ui.Context) { a.loadSpaces() }))
	out = append(out, action("logout", "Keluar akun", "logout", "logout sign out", func(*ui.Context) { a.logout() }))
	return out
}

// pingSpace is the Space a ping goes to: the selected one, or the first
// the account may ping in.
func (a *App) pingSpace() *api.Space {
	if sp := a.selectedSpace(); sp != nil && canPing(sp) {
		return sp
	}
	for i := range a.access.spaces {
		if canPing(&a.access.spaces[i]) {
			return &a.access.spaces[i]
		}
	}
	return nil
}

// navSettings are the settings tabs and everything on them.
func (a *App) navSettings() []navItem {
	platform := "desktop"
	if runtime.GOOS == "js" {
		platform = "web"
	}
	entries := map[string]settingsEntry{}
	var out []navItem
	for _, g := range a.settingsGroups() {
		for _, e := range g.entries {
			e := e
			entries[e.key] = e
			out = append(out, navItem{key: "set" + e.key, title: e.label, trail: "Pengaturan › " + g.title, icon: e.icon, group: "Pengaturan", words: e.key + " " + e.detail, rank: rankSettings, open: func(*ui.Context) {
				a.openSettings(e.key, "")
			}})
		}
	}
	seen := map[string]bool{}
	for _, it := range settingsSearchItems {
		e, ok := entries[it.tab]
		if !ok || (it.platform != "" && it.platform != platform) || seen[it.tab+"\x00"+it.title] || it.title == e.label {
			continue
		}
		seen[it.tab+"\x00"+it.title] = true
		trail := "Pengaturan › " + it.tab
		if it.section != "" && it.section != it.title {
			trail += " › " + it.section
		}
		rank := rankSetting
		if it.kind == "button" {
			rank = rankButton
		}
		reveal := it.section
		if it.kind == "section" {
			reveal = it.title
		}
		tab := it.tab
		out = append(out, navItem{key: "set" + it.tab + "/" + it.title, title: it.title, trail: trail, icon: e.icon, group: "Pengaturan", words: e.label + " " + e.detail, rank: rank, open: func(*ui.Context) {
			a.openSettings(tab, reveal)
		}})
	}
	return out
}

// openSettings opens a settings tab, scrolled to its section reveal.
func (a *App) openSettings(tab, reveal string) {
	a.chrome.settingsTab = tab
	settingsPage.query = "" // a search on the device page would hide the section
	a.chrome.settingsDetail = true
	if reveal == "Pengaturan lanjutan" {
		a.whatsapp.advancedOpen = true
	}
	revealing.title, revealing.asked, revealing.until, revealing.scrolled = reveal, time.Now(), time.Time{}, false
	a.navigate(PathPengaturan)
}

// revealing is the section quick navigation opened: scrolled into view
// once, then outlined for a moment. Only the UI thread touches it.
var revealing struct {
	title        string
	asked, until time.Time
	scrolled     bool
}

// RevealSection scrolls section, titled title, into view and outlines it
// when quick navigation opened it. Section containers outside this
// package, as the desktop's device settings, call it too.
func RevealSection(c *ui.Context, section ui.Element, title string) {
	if revealing.title == "" || revealing.title != title {
		return
	}
	now := c.Now()
	if !revealing.scrolled && time.Since(revealing.asked) > 5*time.Second {
		revealing.title = "" // the section never showed: its page changed
		return
	}
	if revealing.until.IsZero() {
		revealing.until = now.Add(2 * time.Second)
	}
	if !revealing.scrolled {
		section.ScrollIntoView()
		revealing.scrolled = true
	}
	if now.Before(revealing.until) {
		section.Border(2, c.Theme().Accent)
		c.After(revealing.until.Sub(now))
		return
	}
	revealing.title = ""
}

// --- Matching ---

// navSynonyms are words people use for the same thing, Indonesian and
// English: a word of the query matches its group's other words too.
var navSynonyms = [][]string{
	{"pengaturan", "setting", "settings", "setelan", "atur", "konfigurasi", "preferensi", "opsi"},
	{"ace", "island", "dynamic", "maskot", "mascot", "karakter"},
	{"notifikasi", "notif", "pemberitahuan", "notification", "toast", "bel", "lonceng"},
	{"suara", "sound", "audio", "bunyi", "volume"},
	{"tampilan", "tema", "theme", "warna", "appearance", "gelap", "terang", "dark", "light", "aksen"},
	{"akun", "profil", "account", "profile", "pengguna"},
	{"sandi", "password", "keamanan", "security"},
	{"keluar", "logout", "signout"},
	{"whatsapp", "wa", "bot", "grup", "group"},
	{"perintah", "command", "cmd", "pemicu"},
	{"template", "teks", "balasan", "pesan", "format"},
	{"tugas", "task", "jobdesk", "todo", "pekerjaan", "kerjaan"},
	{"pengingat", "reminder", "ingatkan", "alarm", "jadwal"},
	{"rapat", "meeting", "pertemuan", "notulensi"},
	{"catatan", "notes", "note", "notulen", "dokumen"},
	{"anggota", "member", "pengurus", "tim"},
	{"arsip", "archive", "riwayat", "history"},
	{"sinkronisasi", "sync", "offline", "antrean", "sinkron"},
	{"darurat", "emergency", "csv", "spreadsheet", "cadangan", "backup", "snapshot"},
	{"status", "layanan", "service", "kesehatan", "health"},
	{"perangkat", "device", "windows", "desktop", "komputer", "laptop"},
	{"island", "dynamic"},
	{"ping", "colek", "senggol"},
	{"buat", "tambah", "baru", "new", "create"},
	{"hapus", "delete", "buang", "remove"},
	{"panduan", "bantuan", "help", "tutorial", "tur", "tour"},
	{"evaluasi", "sop", "penilaian"},
	{"organisasi", "kadiv", "periode", "admin"},
	{"sidebar", "samping", "menu"},
}

// navWords splits s into lower-case words of letters and digits.
func navWords(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// navAlternatives are the words a query word stands for: itself, and the
// words of every synonym group one of whose words it begins.
func navAlternatives(q string) (own string, others []string) {
	if len(q) >= 2 {
		for _, group := range navSynonyms {
			if slices.ContainsFunc(group, func(w string) bool { return strings.HasPrefix(w, q) }) {
				for _, w := range group {
					if w != q && !slices.Contains(others, w) {
						others = append(others, w)
					}
				}
			}
		}
	}
	return q, others
}

// wordScore is how well query word q matches word w: whole, as its start,
// inside it, with one typo, or as its letters in order.
func wordScore(q, w string) int {
	switch {
	case q == w:
		return 100
	case strings.HasPrefix(w, q):
		return 80
	case len(q) >= 3 && strings.Contains(w, q):
		return 50
	case len(q) >= 4 && (typos(q, w) <= 1 || len(w) > len(q) && typos(q, w[:len(q)]) <= 1):
		return 40
	case len(q) >= 3 && q[0] == w[0] && subsequence(q, w):
		return 20
	}
	return 0
}

// typos is the edit distance of a and b, as bytes.
func typos(a, b string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

func subsequence(q, w string) bool {
	i := 0
	for j := 0; j < len(w) && i < len(q); j++ {
		if w[j] == q[i] {
			i++
		}
	}
	return i == len(q)
}

// navScore is how well query matches item, 0 when a word of the query
// matches none of the item's words. The title counts twice the trail and
// the hidden words.
func navScore(query []string, whole string, item navItem) int {
	title := navWords(item.title)
	rest := append(navWords(item.trail), navWords(item.words)...)
	total := 0
	for _, q := range query {
		own, others := navAlternatives(q)
		best := 0
		for _, w := range title {
			best = max(best, 2*wordScore(own, w))
			for _, o := range others {
				best = max(best, 2*wordScore(o, w)*7/10)
			}
		}
		for _, w := range rest {
			best = max(best, wordScore(own, w))
			for _, o := range others {
				best = max(best, wordScore(o, w)*7/10)
			}
		}
		if best == 0 {
			return 0
		}
		total += best
	}
	if strings.HasPrefix(strings.ToLower(item.title), whole) {
		total += 60
	}
	return total
}

// navMatches returns the items matching query, best first.
func navMatches(items []navItem, query string) []navItem {
	whole := strings.ToLower(strings.TrimSpace(query))
	words := navWords(whole)
	if len(words) == 0 {
		return nil
	}
	type scored struct {
		item  navItem
		score int
	}
	var found []scored
	for _, it := range items {
		if s := navScore(words, whole, it); s > 0 {
			found = append(found, scored{it, s})
		}
	}
	slices.SortStableFunc(found, func(x, y scored) int {
		if x.score != y.score {
			return y.score - x.score
		}
		return x.item.rank - y.item.rank
	})
	out := make([]navItem, len(found))
	for i, f := range found {
		out[i] = f.item
	}
	return out
}

// --- The palette ---

// navLimit is how many results the palette lists.
const navLimit = 30

// toggleSearch is Ctrl K: it opens the palette, and closes it when open.
func (a *App) toggleSearch() (opened bool) {
	if a.chrome.searchOpen {
		a.chrome.searchOpen = false
		return false
	}
	a.chrome.searchOpen = true
	a.chrome.searchCursor = 0
	a.chrome.membersOpen = false
	a.chrome.notificationsOpen = false
	a.chrome.profileOpen = false
	return true
}

func (a *App) searchResults(c *ui.Context) {
	t := c.Theme()
	q := strings.TrimSpace(a.chrome.search)
	var items []navItem
	if q == "" {
		for _, it := range a.navIndex() {
			if it.rank == rankPage || it.rank == rankSpace {
				items = append(items, it)
			}
		}
	} else {
		items = navMatches(a.navIndex(), q)
	}
	if q != a.chrome.searchQuery {
		a.chrome.searchQuery, a.chrome.searchCursor = q, 0
	}
	shown := items[:min(len(items), navLimit)]
	moved := false
	if n := len(shown); n > 0 {
		if c.Shortcut(0, ui.KeyDown) {
			a.chrome.searchCursor = (a.chrome.searchCursor + 1) % n
			moved = true
		}
		if c.Shortcut(0, ui.KeyUp) {
			a.chrome.searchCursor = (a.chrome.searchCursor + n - 1) % n
			moved = true
		}
		a.chrome.searchCursor = min(a.chrome.searchCursor, n-1)
	}
	submit := a.chrome.searchSubmit
	a.chrome.searchSubmit = false
	ui.Row(c).FillWidth().Padding(2, 6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "Navigasi cepat").Bold().Grow(1)
		muted(c, "↑↓ pilih · Enter buka").FontSize(11)
	})
	ui.Scroll(c.Key("quicknav-results")).FillWidth().MaxHeight(340).Label("Hasil navigasi cepat").Children(func() {
		ui.Column(c).FillWidth().Gap(2).Children(func() {
			for i, it := range shown {
				on := i == a.chrome.searchCursor
				b := ui.ButtonBase(c.Key(it.key)).Label(it.title+" — "+it.trail).FillWidth().Row().Gap(10).Padding(7, 8).Radius(8).AlignItems(ui.Center)
				if on || b.Hovered() {
					b.Background(t.SurfaceHover)
				}
				if on && moved {
					b.ScrollIntoView()
				}
				b.FocusRing(b.FocusVisible()).Children(func() {
					ui.Icon(c, spaceIcon(it.icon)).Size(16, 16).TextColor(t.TextMuted).Shrink(0)
					ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
						ui.Text(c, it.title).MaxLines(1)
						muted(c, it.trail).FontSize(11).MaxLines(1)
					})
					muted(c, it.group).FontSize(10).Shrink(0)
				})
				if b.Clicked() || on && submit {
					it.open(c)
					a.chrome.searchOpen = false
					a.chrome.search = ""
				}
			}
		})
	})
	tasks := a.taskSearchResults(c, strings.ToLower(q))
	if len(shown) == 0 && tasks == 0 {
		muted(c, "Tidak ada yang cocok. Coba kata lain, misalnya \"tema\" atau \"island\".").Padding(6)
	}
	if q == "" {
		muted(c, "Ketik untuk mencari halaman, Space, aksi, catatan, tugas, dan setiap pengaturan.").Padding(6)
	}
}
