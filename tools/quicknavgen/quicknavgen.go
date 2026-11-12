// Package quicknavgen reads the settings pages' source and lists what each
// tab shows: its sections, fields, headings and buttons. The quick
// navigation palette (Ctrl K) searches that list, so a setting added to a
// page is found without anyone keeping a second list by hand.
//
// The tabs come from the switch on a.chrome.settingsTab in
// internal/appui/settings_nav.go; from each case it follows the calls into
// the methods and functions of the same package, and for the tab
// "Perangkat" into the device settings of app/ (desktop and web).
//
//	go generate ./internal/appui
package quicknavgen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Item is one thing a settings tab shows.
type Item struct {
	Tab      string // the settings tab's key, as "Perangkat"
	Title    string // as shown, without its state ("Suara bel", not "Suara bel: aktif")
	Section  string // the section it sits in, "" at the top of the page
	Kind     string // "section", "field", "heading" or "button"
	Platform string // "desktop" or "web" for device settings, "" for both
}

// pkg is a parsed package: its functions and its package-level maps.
type pkg struct {
	name  string
	funcs map[string][]*ast.FuncDecl // by name, methods and functions
	maps  map[string][]string        // string values of package-level map literals
}

func load(dir string, keep func(string) bool) (*pkg, error) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	p := &pkg{funcs: map[string][]*ast.FuncDecl{}, maps: map[string][]string{}}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || !keep(name) {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		p.name = f.Name.Name
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				p.funcs[d.Name.Name] = append(p.funcs[d.Name.Name], d)
			case *ast.GenDecl:
				for _, s := range d.Specs {
					vs, ok := s.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, v := range vs.Values {
						if lit, ok := v.(*ast.CompositeLit); ok {
							if _, isMap := lit.Type.(*ast.MapType); isMap {
								for _, el := range lit.Elts {
									if kv, ok := el.(*ast.KeyValueExpr); ok {
										if s, ok := str(kv.Value); ok {
											p.maps[vs.Names[i].Name] = append(p.maps[vs.Names[i].Name], s)
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return p, nil
}

// receiverType is "App" for func (a *App) and "" for a function.
func receiverType(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return ""
	}
	t := fd.Recv.List[0].Type
	if s, ok := t.(*ast.StarExpr); ok {
		t = s.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func (p *pkg) find(recv, name string) *ast.FuncDecl {
	for _, fd := range p.funcs[name] {
		if receiverType(fd) == recv {
			return fd
		}
	}
	return nil
}

func str(e ast.Expr) (string, bool) {
	if b, ok := e.(*ast.BasicLit); ok && b.Kind == token.STRING {
		s, err := strconv.Unquote(b.Value)
		return s, err == nil
	}
	return "", false
}

// sectionCalls name the section containers: the title argument and the
// function holding the section's contents.
var sectionCalls = map[string]int{"pageSection": 1, "settingsBody": 1, "settingsPageView": 1, "SettingSection": 1, "appui.SettingSection": 1}

// fieldCalls are labelled inputs: the label argument's index.
var fieldCalls = map[string]int{"ui.Field": 1, "SettingRow": 1, "appui.SettingRow": 1, "taskInput": 2, "taskInputHint": 2, "a.waField": 2, "a.waTemplateEditor": 2}

// buttonCalls are buttons: the label argument's index.
var buttonCalls = map[string]int{"spaceButton": 1, "primarySpaceButton": 1, "ui.Button": 1,
	"SettingSwitch": 1, "appui.SettingSwitch": 1, "SettingButton": 1, "appui.SettingButton": 1}

// sectionFuncs are methods drawing a section of their own that no
// pageSection names: their items sit in it.
var sectionFuncs = map[string]string{"waAdvanced": "Pengaturan lanjutan"}

// skipCalls are not settings: notices and empty or loading states.
var skipCalls = map[string]bool{"pageNotice": true, "pageEmpty": true, "pageUnavailable": true, "muted": true}

// skipFuncs draw states, not settings: waHero is the bot's status line;
// the ping composer's parts are its states, found by its heading; Kredit
// (Tentang › Dibuat oleh) holds credits, not settings.
var skipFuncs = map[string]bool{"waHero": true, "pingComposerBody": true, "pingComposerFooter": true, "pingSentView": true, "kreditSettings": true}

// generic labels say nothing on their own in a search: confirmations,
// closing, reloading, and the channels and sides of a comparison that
// share their names with whole tabs.
var generic = map[string]bool{"Batal": true, "Tutup": true, "Salin": true, "Selesai": true, "Coba lagi": true, "Muat ulang": true,
	"Divisi": true, "Pratinjau": true, "Putar": true, "Ganti": true, "Pakai bawaan": true, "Kirim percobaan": true, "Tutup pratinjau": true, "Perangkat": true, "WhatsApp": true, "Server": true, "Pesan": true}

type walker struct {
	p        *pkg
	tab      string
	platform string
	seen     map[*ast.FuncDecl]bool
	items    *[]Item
}

func callName(call *ast.CallExpr) string {
	switch f := call.Fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		if x, ok := f.X.(*ast.Ident); ok {
			return x.Name + "." + f.Sel.Name
		}
	}
	return ""
}

// labels returns the texts e stands for: a string literal, the values of a
// package map it indexes, or the literals a local variable is given in fn.
func (w *walker) labels(e ast.Expr, fn ast.Node) []string {
	if s, ok := str(e); ok {
		return []string{s}
	}
	switch e := e.(type) {
	case *ast.IndexExpr:
		if id, ok := e.X.(*ast.Ident); ok {
			return w.p.maps[id.Name]
		}
	case *ast.Ident:
		var out []string
		ast.Inspect(fn, func(n ast.Node) bool {
			if as, ok := n.(*ast.AssignStmt); ok && len(as.Lhs) == len(as.Rhs) {
				for i, l := range as.Lhs {
					if id, ok := l.(*ast.Ident); ok && id.Name == e.Name {
						if s, ok := str(as.Rhs[i]); ok {
							out = append(out, s)
						}
					}
				}
			}
			return true
		})
		return out
	}
	return nil
}

func (w *walker) add(kind, title, section string) {
	title = clean(title)
	if title == "" || generic[title] || strings.HasPrefix(title, "Konfirmasi ") || strings.HasPrefix(title, "Ya, ") || strings.HasSuffix(title, "?") || strings.Contains(title, "? ") {
		return
	}
	for _, it := range *w.items {
		if it.Tab == w.tab && it.Title == title && it.Platform == w.platform {
			return
		}
	}
	*w.items = append(*w.items, Item{Tab: w.tab, Title: title, Section: section, Kind: kind, Platform: w.platform})
}

// clean drops a label's state and hints: "Suara bel: aktif" is "Suara bel",
// "Volume bel · 0–100" is "Volume bel", "Pemicu, misalnya damn" is "Pemicu".
func clean(s string) string {
	for _, sep := range []string{": ", " · ", ", misalnya", " — "} {
		if i := strings.Index(s, sep); i > 0 {
			s = s[:i]
		}
	}
	s = strings.TrimSpace(strings.TrimRight(s, "…."))
	if len([]rune(s)) < 3 || strings.ContainsAny(s, "%{[") {
		return ""
	}
	return s
}

func (w *walker) walkFunc(fd *ast.FuncDecl, section string) {
	if fd == nil || fd.Body == nil || w.seen[fd] || skipFuncs[fd.Name.Name] {
		return
	}
	w.seen[fd] = true
	if s, ok := sectionFuncs[fd.Name.Name]; ok {
		w.add("section", s, s)
		section = s
	}
	w.walk(fd.Body, fd, receiverType(fd), section)
}

// walk visits n in source order; a section call's contents are walked with
// its title as the section.
func (w *walker) walk(n ast.Node, fn ast.Node, recv, section string) {
	ast.Inspect(n, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := callName(call)
		short := name
		if i := strings.LastIndex(name, "."); i >= 0 {
			short = name[i+1:]
		}
		if skipCalls[name] {
			return false
		}
		if i, ok := sectionCalls[name]; ok && len(call.Args) > i {
			inner := section
			for _, s := range w.labels(call.Args[i], fn) {
				w.add("section", s, clean(s))
				inner = clean(s)
			}
			for _, arg := range call.Args[i+1:] {
				w.walk(arg, fn, recv, inner)
			}
			return false
		}
		if i, ok := fieldCalls[name]; ok && len(call.Args) > i {
			for _, s := range w.labels(call.Args[i], fn) {
				w.add("field", s, section)
			}
		}
		if i, ok := buttonCalls[name]; ok && len(call.Args) > i {
			for _, s := range w.labels(call.Args[i], fn) {
				w.add("button", s, section)
			}
		}
		// ui.Text(c, "Tema").Bold() and .FontWeight(…) are headings.
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && (sel.Sel.Name == "Bold" || sel.Sel.Name == "FontWeight") {
			for x := sel.X; ; {
				inner, ok := x.(*ast.CallExpr)
				if !ok {
					break
				}
				if callName(inner) == "ui.Text" && len(inner.Args) > 1 {
					for _, s := range w.labels(inner.Args[1], fn) {
						w.add("heading", s, section)
					}
					break
				}
				s, ok := inner.Fun.(*ast.SelectorExpr)
				if !ok {
					break
				}
				x = s.X
			}
		}
		// Follow the package's own methods and functions.
		switch f := call.Fun.(type) {
		case *ast.Ident:
			w.walkFunc(w.p.find("", f.Name), section)
		case *ast.SelectorExpr:
			if x, ok := f.X.(*ast.Ident); ok && x.Name != "ui" && x.Name != "c" && x.Name != "t" {
				if fd := w.p.find(recv, short); fd != nil {
					w.walkFunc(fd, section)
				} else if fds := w.p.funcs[short]; len(fds) == 1 && fds[0].Recv != nil {
					w.walkFunc(fds[0], section)
				}
			}
		}
		return true
	})
}

// Root is the repository's root directory, as found from dir.
func Root(dir string) (string, error) {
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d, nil
		}
		if filepath.Dir(d) == d {
			return "", fmt.Errorf("quicknavgen: no go.mod above %s", dir)
		}
	}
}

// Harvest lists the items of every settings tab of the repository at root.
func Harvest(root string) ([]Item, error) {
	appui, err := load(filepath.Join(root, "internal", "appui"), func(string) bool { return true })
	if err != nil {
		return nil, err
	}
	desktop, err := load(filepath.Join(root, "app"), func(name string) bool {
		return !strings.HasSuffix(name, "_web.go") && !strings.HasSuffix(name, "_js.go")
	})
	if err != nil {
		return nil, err
	}
	settings := appui.find("App", "settings")
	if settings == nil {
		return nil, fmt.Errorf("quicknavgen: (*App).settings not found")
	}
	var sw *ast.SwitchStmt
	ast.Inspect(settings.Body, func(n ast.Node) bool {
		if s, ok := n.(*ast.SwitchStmt); ok && sw == nil {
			if sel, ok := s.Tag.(*ast.SelectorExpr); ok && sel.Sel.Name == "settingsTab" {
				sw = s
			}
		}
		return sw == nil
	})
	if sw == nil {
		return nil, fmt.Errorf("quicknavgen: the switch on settingsTab not found")
	}
	keys, err := settingsKeys(appui)
	if err != nil {
		return nil, err
	}
	var items []Item
	covered := map[string]bool{}
	var fallback *ast.CaseClause
	for _, s := range sw.Body.List {
		cc := s.(*ast.CaseClause)
		if cc.List == nil {
			fallback = cc
			continue
		}
		for _, e := range cc.List {
			key, _ := str(e)
			covered[key] = true
			harvestCase(appui, desktop, key, cc, settings, &items)
		}
	}
	if fallback != nil {
		for _, key := range keys {
			if !covered[key] {
				harvestCase(appui, desktop, key, fallback, settings, &items)
			}
		}
	}
	return items, nil
}

func harvestCase(appui, desktop *pkg, key string, cc *ast.CaseClause, settings *ast.FuncDecl, items *[]Item) {
	w := &walker{p: appui, tab: key, seen: map[*ast.FuncDecl]bool{}, items: items}
	device := false
	for _, st := range cc.Body {
		ast.Inspect(st, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "DeviceSettings" {
				device = true
			}
			return true
		})
		w.walk(st, settings, "App", "")
	}
	if device {
		d := &walker{p: desktop, tab: key, platform: "desktop", seen: map[*ast.FuncDecl]bool{}, items: items}
		d.walkFunc(desktop.find("desktopDevice", "settings"), "")
		web := &walker{p: appui, tab: key, platform: "web", seen: map[*ast.FuncDecl]bool{}, items: items}
		web.walkFunc(appui.find("BrowserNotifications", "View"), "")
	}
}

// settingsKeys are the tab keys of settingsGroups, the first string of
// each settingsEntry.
func settingsKeys(p *pkg) ([]string, error) {
	fd := p.find("App", "settingsGroups")
	if fd == nil {
		return nil, fmt.Errorf("quicknavgen: (*App).settingsGroups not found")
	}
	var keys []string
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || len(lit.Elts) != 4 {
			return true
		}
		if s, ok := str(lit.Elts[0]); ok {
			keys = append(keys, s)
		}
		return true
	})
	return keys, nil
}

// Generate returns the Go source of quicknav_settings_gen.go.
func Generate(root string) ([]byte, error) {
	items, err := Harvest(root)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.WriteString("// Code generated by tools/quicknavgen; DO NOT EDIT.\n// Run go generate ./internal/appui after changing a settings page.\n\npackage appui\n\n")
	b.WriteString("// settingsSearchItems is every section, field, heading and button of the\n// settings tabs, read from their source.\nvar settingsSearchItems = []settingsSearchItem{\n")
	for _, it := range items {
		fmt.Fprintf(&b, "\t{%q, %q, %q, %q, %q},\n", it.Tab, it.Title, it.Section, it.Kind, it.Platform)
	}
	b.WriteString("}\n")
	return format.Source(b.Bytes())
}
