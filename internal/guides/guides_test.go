package guides

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
)

func TestEmbeddedGuides(t *testing.T) {
	gs := Embedded()
	if len(gs) < 10 {
		t.Fatalf("%d guides", len(gs))
	}
	roles := map[string]int{}
	pages := map[string]bool{}
	for _, g := range gs {
		for _, r := range g.Roles {
			roles[r]++
		}
		for _, p := range g.Pages {
			pages[p] = true
		}
		// Pictures a guide draws must be known to the app.
		for _, part := range strings.Split(g.Body, "```gambar\n")[1:] {
			name, _, _ := strings.Cut(part, "\n")
			found := false
			for _, p := range Pictures {
				found = found || p == name
			}
			if !found {
				t.Errorf("%s: picture %q", g.Slug, name)
			}
		}
	}
	for _, r := range []string{api.GuideRoleMember, api.GuideRole45, api.GuideRoleKadiv} {
		if roles[r] < 3 {
			t.Errorf("role %s has %d guides", r, roles[r])
		}
	}
	// Every page with a "?" has an article.
	for _, p := range []string{"beranda", "board", "catatan", "ping", "anggota", "whatsapp"} {
		if !pages[p] {
			t.Errorf("no guide for page %s", p)
		}
	}
	for i := 1; i < len(gs); i++ {
		if gs[i-1].Order > gs[i].Order {
			t.Fatalf("not sorted at %s", gs[i].Slug)
		}
	}
}

func TestParseRejects(t *testing.T) {
	for name, src := range map[string]string{
		"no header":  "# Judul\n",
		"open":       "---\nslug: a\n",
		"bad slug":   "---\nslug: A B\ntitle: t\ntopic: x\nroles: anggota\n---\nisi",
		"bad role":   "---\nslug: a\ntitle: t\ntopic: x\nroles: admin\n---\nisi",
		"no roles":   "---\nslug: a\ntitle: t\ntopic: x\n---\nisi",
		"unknown":    "---\nslug: a\ntitle: t\ntopic: x\nroles: 45\nwarna: merah\n---\nisi",
		"bad number": "---\nslug: a\ntitle: t\ntopic: x\nroles: 45\nminutes: dua\n---\nisi",
	} {
		if _, err := Parse(src); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	g, err := Parse("---\r\nslug: a\r\ntitle: Judul\r\ntopic: T\r\nroles: anggota, kadiv\r\npages: board\r\nminutes: 2\r\n---\r\nIsi **tebal**.\r\n")
	if err != nil || g.Slug != "a" || len(g.Roles) != 2 || g.Minutes != 2 || g.Body != "Isi **tebal**.\n" || !g.ForRole("kadiv") {
		t.Fatalf("%+v %v", g, err)
	}
}

func TestLoadFolder(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("b.md", "---\nslug: b\ntitle: Kedua\ntopic: T\nroles: anggota\norder: 2\n---\nisi")
	write("a.md", "---\nslug: a\ntitle: Pertama\ntopic: T\nroles: anggota\norder: 1\n---\nisi")
	write("catatan.txt", "bukan panduan")
	l, err := Load(dir)
	if err != nil || l.Source != "berkas" || len(l.Guides) != 2 || l.Guides[0].Slug != "a" {
		t.Fatalf("%+v %v", l, err)
	}
	write("c.md", "---\nslug: a\ntitle: Ganda\ntopic: T\nroles: anggota\n---\nisi")
	if _, err := Load(dir); err == nil {
		t.Fatal("duplicate slug accepted")
	}
	if l, err := Load(""); err != nil || l.Source != "bawaan" {
		t.Fatalf("%+v %v", l, err)
	}
}
