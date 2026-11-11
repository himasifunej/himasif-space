package appui

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/tools/quicknavgen"
)

// The settings index is generated from the settings pages: a page changed
// without go generate fails here instead of going missing from Ctrl K.
func TestQuicknavSettingsIndexUpToDate(t *testing.T) {
	wd, _ := os.Getwd()
	root, err := quicknavgen.Root(wd)
	if err != nil {
		t.Fatal(err)
	}
	want, err := quicknavgen.Generate(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("quicknav_settings_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("quicknav_settings_gen.go is stale: run go generate ./internal/appui")
	}
	keys := map[string]bool{}
	for _, g := range (&App{}).settingsGroups() {
		for _, e := range g.entries {
			keys[e.key] = true
		}
	}
	tabs := map[string]bool{}
	for _, it := range settingsSearchItems {
		if !keys[it.tab] {
			t.Errorf("item %q of unknown tab %q", it.title, it.tab)
		}
		tabs[it.tab] = true
	}
	for key := range keys {
		if !tabs[key] {
			t.Errorf("settings tab %q has no searchable item", key)
		}
	}
}

// Every page the router draws is in appPages, so quick navigation and the
// topbar know it.
func TestQuicknavPagesCoverRouter(t *testing.T) {
	fset := token.NewFileSet()
	consts := map[string]string{}
	var matched []string
	files, _ := filepath.Glob("*.go")
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.ValueSpec:
				for i, v := range n.Values {
					if lit, ok := v.(*ast.BasicLit); ok && strings.HasPrefix(n.Names[i].Name, "Path") {
						consts[n.Names[i].Name], _ = strconv.Unquote(lit.Value)
					}
				}
			case *ast.FuncDecl:
				if n.Name.Name != "page" {
					return true
				}
				ast.Inspect(n.Body, func(n ast.Node) bool {
					if call, ok := n.(*ast.CallExpr); ok {
						if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Match" && len(call.Args) == 1 {
							if id, ok := call.Args[0].(*ast.Ident); ok {
								matched = append(matched, id.Name)
							}
						}
					}
					return true
				})
			}
			return true
		})
	}
	if len(matched) < 8 {
		t.Fatalf("found only %v in page()", matched)
	}
	for _, name := range matched {
		path, ok := consts[name]
		if !ok {
			t.Errorf("no value for %s", name)
			continue
		}
		found := false
		for _, p := range appPages {
			found = found || p.path == path
		}
		if !found {
			t.Errorf("page %s (%s) is missing from appPages", name, path)
		}
	}
}

func TestQuicknavMatching(t *testing.T) {
	a := &App{}
	index := append(a.navSettings(), a.navActions()...)
	for _, p := range appPages {
		index = append(index, navItem{key: p.path, title: p.title, trail: "Halaman", words: p.words})
	}
	first := func(q string) navItem {
		t.Helper()
		got := navMatches(index, q)
		if len(got) == 0 {
			t.Fatalf("%q matched nothing", q)
		}
		return got[0]
	}
	// Ace is the Dynamic Island: its name finds the device page first.
	if it := first("Ace"); it.title != "Notifikasi dan Dynamic Island" || it.trail != "Pengaturan › Perangkat ini" {
		t.Errorf("Ace → %q · %q", it.title, it.trail)
	}
	for q, want := range map[string]string{
		"pengaturn":     "Pengaturan", // a typo
		"setting":       "Pengaturan", // English
		"tema gelap":    "Tema gelap",
		"maskot":        "Notifikasi dan Dynamic Island",
		"selalu tampil": "Selalu tampil",
		"volume":        "Volume bel",
		"logout":        "Keluar akun",
		"sandi":         "Kata sandi",
		"balasan grup":  "Balasan perintah di grup",
		"island":        "Notifikasi dan Dynamic Island",
	} {
		if it := first(q); it.title != want {
			t.Errorf("%q → %q (%s), want %q", q, it.title, it.trail, want)
		}
	}
	if got := navMatches(index, "zzqx"); len(got) != 0 {
		t.Errorf("nonsense matched %q", got[0].title)
	}
}

// Ctrl K opens the palette and closes it; typing "Ace" lists the setting
// with where it is, and Enter opens it.
func TestQuicknavPalette(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.Key(ui.Ctrl, ui.KeyK)
	if !h.app.chrome.searchOpen {
		t.Fatal("Ctrl K did not open")
	}
	h.want("Jobdesk saya")
	h.tt.Key(ui.Ctrl, ui.KeyK)
	if h.app.chrome.searchOpen {
		t.Fatal("Ctrl K did not close the open palette")
	}
	h.dontWant("Navigasi cepat")
	h.tt.Key(ui.Ctrl, ui.KeyK)
	h.tt.Type("Ace")
	h.want("Notifikasi dan Dynamic Island")
	h.want("Pengaturan › Perangkat")
	h.want("Selalu tampil")
	h.tt.Key(0, ui.KeyDown)
	if h.app.chrome.searchCursor != 1 {
		t.Fatalf("Down left the cursor at %d", h.app.chrome.searchCursor)
	}
	h.tt.Key(0, ui.KeyUp)
	h.tt.Key(0, ui.KeyEnter)
	h.settle()
	if h.app.router.Path() != PathPengaturan || h.app.chrome.settingsTab != "Perangkat" || h.app.chrome.searchOpen {
		t.Fatalf("Enter opened %s tab %q, open %v", h.app.router.Path(), h.app.chrome.settingsTab, h.app.chrome.searchOpen)
	}

	// A section deep in a tab: the advanced WhatsApp settings open.
	h.tt.Key(ui.Ctrl, ui.KeyK)
	h.tt.Type("balasan perintah")
	h.want("Balasan perintah di grup")
	h.want("Pengaturan › WhatsApp › Pengaturan lanjutan")
	h.click("Balasan perintah di grup — Pengaturan › WhatsApp › Pengaturan lanjutan")
	if h.app.chrome.settingsTab != "WhatsApp" || !h.app.whatsapp.advancedOpen {
		t.Fatalf("tab %q, advanced %v", h.app.chrome.settingsTab, h.app.whatsapp.advancedOpen)
	}
}

// Opening a section scrolls it into view and outlines it for a moment.
func TestQuicknavRevealsSection(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.Key(ui.Ctrl, ui.KeyK)
	h.tt.Type("keamanan akun")
	h.want("Pengaturan › Akun")
	h.tt.Key(0, ui.KeyEnter)
	h.settle()
	if h.app.chrome.settingsTab != "Akun" || revealing.title != "Keamanan akun" || !revealing.scrolled {
		t.Fatalf("tab %q, revealing %+v", h.app.chrome.settingsTab, revealing)
	}
	h.tt.Advance(3 * time.Second)
	h.tt.Frame()
	if revealing.title != "" {
		t.Fatalf("the outline stayed: %+v", revealing)
	}
}

// Release review: Enter in the search field while the results are closed
// shows them; it does not run a result the user cannot see. Izin arsip is
// listed only for who manages an archive.
func TestQuicknavEnterWhileClosedShowsResults(t *testing.T) {
	h := newChromeHarness(t, nil)
	path := h.app.router.Path()
	h.tt.Key(ui.Ctrl, ui.KeyK)
	h.tt.Type("pengaturan")
	h.tt.Frame()
	h.tt.Key(ui.Ctrl, ui.KeyK)
	h.tt.Frame()
	if h.app.chrome.searchOpen {
		t.Fatal("Ctrl K did not close")
	}
	h.tt.Key(0, ui.KeyEnter)
	h.tt.Frame()
	if !h.app.chrome.searchOpen || h.app.router.Path() != path {
		t.Fatalf("Enter while closed: open %v, path %s", h.app.chrome.searchOpen, h.app.router.Path())
	}
	for _, it := range h.app.navIndex() {
		if it.key == "page"+PathArchiveAccess && len(h.app.archiveManagedDivisions()) == 0 {
			t.Fatal("Izin arsip offered to who manages no archive")
		}
	}
}
