// Package guides holds Space's in-app guide (Panduan): short articles in
// Markdown, one file each, with a header of metadata:
//
//	---
//	slug: board-tugas
//	title: Membaca dan memindah kartu tugas
//	topic: Tugas
//	summary: Satu kalimat untuk daftar topik.
//	roles: anggota, 45, kadiv
//	pages: board
//	minutes: 2
//	order: 20
//	---
//	Isi artikel dalam Markdown.
//
// The articles in content/ are built into the server and the app. A server
// with SPACE_GUIDES_DIR set serves the .md files of that folder instead, so
// the guide can be changed on the server without a release; the app shows
// the built-in copy only when the server has none (an older server, or
// offline).
//
// A fenced block of the language "gambar" draws a small illustration of
// Space in the app, named by its first line: board, catatan, ping,
// anggota, whatsapp, beranda, tanya.
package guides

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/himasifunej/himasif-space/internal/api"
)

//go:embed content/*.md
var content embed.FS

// Bounds of a guide folder: enough for a handbook, small enough to send
// in one answer.
const (
	maxGuides    = 80
	maxGuideSize = 32 << 10
)

var (
	slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	pageRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
)

// Pictures are the illustrations a "gambar" block may name.
var Pictures = []string{"board", "catatan", "ping", "anggota", "whatsapp", "beranda", "tanya"}

// Embedded returns the guides built in. It panics on a broken file: the
// tests read every one.
func Embedded() []api.Guide {
	out, err := read(content, "content")
	if err != nil {
		panic(err)
	}
	return out
}

// Load returns the guides of dir, or the built-in ones when dir is "".
// It is read on every call, so a changed file shows at once.
func Load(dir string) (api.GuideList, error) {
	if dir == "" {
		return api.GuideList{Guides: Embedded(), Source: "bawaan"}, nil
	}
	gs, err := read(os.DirFS(dir), ".")
	if err != nil {
		return api.GuideList{}, err
	}
	return api.GuideList{Guides: gs, Source: "berkas"}, nil
}

func read(fsys fs.FS, dir string) ([]api.Guide, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	var out []api.Guide
	seen := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if len(out) == maxGuides {
			return nil, fmt.Errorf("guides: more than %d guides", maxGuides)
		}
		raw, err := fs.ReadFile(fsys, path.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		if len(raw) > maxGuideSize {
			return nil, fmt.Errorf("guides: %s is larger than %d bytes", e.Name(), maxGuideSize)
		}
		g, err := Parse(string(raw))
		if err != nil {
			return nil, fmt.Errorf("guides: %s: %w", e.Name(), err)
		}
		if seen[g.Slug] {
			return nil, fmt.Errorf("guides: slug %q twice", g.Slug)
		}
		seen[g.Slug] = true
		out = append(out, g)
	}
	Sort(out)
	return out, nil
}

// Sort orders guides as the Panduan lists them: by order, then title.
func Sort(gs []api.Guide) {
	sort.SliceStable(gs, func(i, j int) bool {
		if gs[i].Order != gs[j].Order {
			return gs[i].Order < gs[j].Order
		}
		return gs[i].Title < gs[j].Title
	})
}

// Parse reads one guide file: its header, then its Markdown.
func Parse(src string) (api.Guide, error) {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	if !strings.HasPrefix(src, "---\n") {
		return api.Guide{}, fmt.Errorf("no header (--- on the first line)")
	}
	head, body, ok := strings.Cut(src[4:], "\n---\n")
	if !ok {
		return api.Guide{}, fmt.Errorf("header not closed by ---")
	}
	var g api.Guide
	for _, line := range strings.Split(head, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			return api.Guide{}, fmt.Errorf("header line %q has no colon", line)
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		switch k {
		case "slug":
			g.Slug = v
		case "title":
			g.Title = v
		case "topic":
			g.Topic = v
		case "summary":
			g.Summary = v
		case "roles":
			g.Roles = list(v)
		case "pages":
			g.Pages = list(v)
		case "minutes", "order":
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 || n > 9999 {
				return api.Guide{}, fmt.Errorf("%s %q is not a number", k, v)
			}
			if k == "minutes" {
				g.Minutes = n
			} else {
				g.Order = n
			}
		default:
			return api.Guide{}, fmt.Errorf("unknown header %q", k)
		}
	}
	g.Body = strings.TrimSpace(body) + "\n"
	return g, Valid(g)
}

// Valid checks a guide's metadata.
func Valid(g api.Guide) error {
	switch {
	case !slugRe.MatchString(g.Slug):
		return fmt.Errorf("slug %q: lowercase letters, digits and -", g.Slug)
	case g.Title == "" || len(g.Title) > 120:
		return fmt.Errorf("title: 1 to 120 characters")
	case g.Topic == "" || len(g.Topic) > 40:
		return fmt.Errorf("topic: 1 to 40 characters")
	case len(g.Roles) == 0:
		return fmt.Errorf("roles: at least one of anggota, 45, kadiv")
	}
	for _, r := range g.Roles {
		if r != api.GuideRoleMember && r != api.GuideRole45 && r != api.GuideRoleKadiv {
			return fmt.Errorf("role %q: anggota, 45 or kadiv", r)
		}
	}
	for _, p := range g.Pages {
		if !pageRe.MatchString(p) {
			return fmt.Errorf("page %q", p)
		}
	}
	return nil
}

func list(v string) []string {
	var out []string
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
