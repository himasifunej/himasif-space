package appui

import (
	_ "embed"
	"encoding/json"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Kredit, option B of canvas dec9ac46ca25 (the owner's pick): reached from
// Tentang's "Dibuat oleh" row, the owner's mark large at the left, the
// details at the right, and the contributors as a list.

// contributorsJSON is the repository's contributors.json (format 1,
// docs/contributors.md), copied here at build time so Kredit shows offline.
// TestContributorsEmbedded keeps the copy the same as the root file.
//
//go:embed contributors.json
var contributorsJSON []byte

// Contributor is an entry of contributors.json. Fields this app does not
// know are ignored, as format 1 asks of its readers.
type Contributor struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Alias       string   `json:"alias"`
	Role        string   `json:"role"`
	Kind        []string `json:"kind"`
	Affiliation string   `json:"affiliation"`
	Period      string   `json:"period"`
	GitHub      string   `json:"github"`
	Links       []struct {
		Label string `json:"label"`
		URL   string `json:"url"`
	} `json:"links"`
}

// contributorsFile is contributors.json.
type contributorsFile struct {
	Format  int `json:"format"`
	Project struct {
		Name       string `json:"name"`
		By         string `json:"by"`
		Repository string `json:"repository"`
	} `json:"project"`
	Contributors []Contributor `json:"contributors"`
}

// parseContributors reads contributors.json; a format other than 1, or a
// file it cannot read, gives no list rather than a wrong one.
func parseContributors(data []byte) contributorsFile {
	var f contributorsFile
	if err := json.Unmarshal(data, &f); err != nil || f.Format != 1 {
		return contributorsFile{}
	}
	people := f.Contributors[:0]
	for _, p := range f.Contributors {
		if strings.TrimSpace(p.Name) != "" {
			people = append(people, p)
		}
	}
	f.Contributors = people
	return f
}

// Contributors is the list bundled with the app, in the file's order.
var Contributors = parseContributors(contributorsJSON)

// The owner's line and links on Kredit.
const (
	kreditOwner      = "Ahimsa Jenar Bramsaifstyo Kusuma - Tiyoouw | MEDIATEK 25/26"
	kreditSite       = "https://tiyoouw.com"
	kreditGitHub     = "https://github.com/Tiyouw"
	kreditRepository = "https://github.com/himasifunej/himasif-space"
)

// kreditState is the page's: its mark, which plays its intro each time
// the page opens.
type kreditState struct {
	mark tiyoouwMark
}

// openKredit shows Kredit, its mark playing from the start.
func (a *App) openKredit() {
	a.chrome.settingsTab = "Kredit"
	a.kredit = kreditState{}
}

// madeByRow is Tentang's "Dibuat oleh": the whole row opens Kredit.
func (a *App) madeByRow(c *ui.Context) {
	if !settingsRowShown("Dibuat oleh", "Tiyoouw dan kontributor") {
		return
	}
	t := c.Theme()
	row := ui.ButtonBase(c.Key("row-Dibuat oleh")).Label("Dibuat oleh: Tiyoouw dan kontributor").Row().FillWidth().MinWidth(0).Padding(14, 16).Gap(16).AlignItems(ui.Center).Radius(0, 0, 12, 12).Transition(motionState)
	if !settingsPage.first {
		row.BorderWidth(1, 0, 0, 0).BorderColor(t.Border)
	}
	settingsPage.first = false
	if row.Hovered() || row.Pressed() {
		row.Background(t.SurfacePressed)
	}
	row.FocusRing(row.FocusVisible())
	if row.Clicked() {
		a.openKredit()
	}
	row.Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, "Dibuat oleh").FontSize(14).TextColor(t.TextMuted)
		})
		ui.Row(c).Shrink(0).Gap(10).AlignItems(ui.Center).TextColor(faint(t)).Children(func() {
			ui.Text(c, "Tiyoouw dan kontributor").FontSize(13).MaxLines(1)
			ui.Icon(c, spaceIcon("chevron")).Size(16, 16)
		})
	})
}

// kreditLink is a row of a list that opens url: an icon, a title and what
// it is, and the outward arrow.
func kreditLink(c *ui.Context, icon, title, kind, url string, mono bool) {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(url)).Label(title+", "+kind).Tooltip(url).Row().FillWidth().MinWidth(0).Gap(12).AlignItems(ui.Center).Padding(10, 8).Radius(8).Transition(motionState)
	if b.Hovered() || b.Pressed() {
		b.Background(t.SurfaceHover)
	}
	b.FocusRing(b.FocusVisible())
	if b.Clicked() {
		c.OpenURL(url)
	}
	b.Children(func() {
		ui.Icon(c, spaceIcon(icon)).Size(16, 16).Shrink(0).TextColor(t.TextMuted)
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			name := ui.Text(c, title).FontSize(14).FontWeight(500).MaxLines(1)
			if mono {
				name.Font(FontMono).FontSize(13)
			}
			muted(c, kind)
		})
		ui.Icon(c, spaceIcon("external")).Size(14, 14).Shrink(0).TextColor(faint(t))
	})
}

// kreditList draws rows with a line between them and above and below.
func kreditList(c *ui.Context, key string, n int, row func(i int)) {
	t := c.Theme()
	ui.Column(c.Key(key)).FillWidth().MinWidth(0).BorderWidth(1, 0, 1, 0).BorderColor(t.Border).Children(func() {
		for i := range n {
			r := ui.Column(c.Key(i)).FillWidth().MinWidth(0)
			if i > 0 {
				r.BorderWidth(1, 0, 0, 0).BorderColor(t.Border)
			}
			r.Children(func() { row(i) })
		}
	})
}

// contributorInitials are up to two letters of a name, for its badge.
func contributorInitials(name string) string {
	var out []rune
	for _, w := range strings.Fields(name) {
		for _, r := range w {
			out = append(out, r)
			break
		}
		if len(out) == 2 {
			break
		}
	}
	return strings.ToUpper(string(out))
}

// contributorRow is one person of contributors.json.
func contributorRow(c *ui.Context, p Contributor) {
	t := c.Theme()
	ui.Row(c).FillWidth().MinWidth(0).Gap(12).AlignItems(ui.Center).Padding(10, 8).Children(func() {
		ui.Column(c).Size(32, 32).Shrink(0).Radius(8).Background(t.SurfaceHover).Center().Children(func() {
			ui.Text(c, contributorInitials(p.Name)).FontSize(12).FontWeight(600).TextColor(t.TextMuted)
		})
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			name := p.Name
			if p.Alias != "" {
				name += " (" + p.Alias + ")"
			}
			ui.Text(c, name).FontSize(14).FontWeight(500).MaxLines(2)
			line := p.Role
			if p.Affiliation != "" {
				line += " · " + p.Affiliation
			}
			muted(c, line).MaxLines(2)
		})
		if p.GitHub != "" {
			url := "https://github.com/" + p.GitHub
			b := ui.ButtonBase(c.Key("gh")).Label("GitHub "+p.GitHub).Tooltip(url).Shrink(0).Padding(4, 8).Radius(6).TextColor(faint(t)).Transition(motionState)
			if b.Hovered() {
				b.Background(t.SurfaceHover).TextColor(t.Text)
			}
			b.FocusRing(b.FocusVisible())
			if b.Clicked() {
				c.OpenURL(url)
			}
			b.Children(func() { ui.Text(c, "github.com/"+p.GitHub).Font(FontMono).FontSize(12).MaxLines(1) })
		}
	})
}

// kreditSettings is the Kredit page.
func (a *App) kreditSettings(c *ui.Context) {
	t := c.Theme()
	w, _ := c.Size()
	stacked := w < 1100
	ui.Scroll(c.Key("kredit-page")).Fill().Label("Kredit").Children(func() {
		page := ui.Column(c).FillWidth().MinWidth(0).MaxWidth(1088).Padding(28, 40, 64, 40).Gap(28)
		if w < 600 {
			page.Padding(20, 16, 48, 16).Gap(24)
		}
		page.Children(func() {
			ui.Row(c.Key("crumb")).FillWidth().MinWidth(0).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
				back := ui.ButtonBase(c.Key("back")).Label("Kembali ke Tentang").Row().Gap(6).AlignItems(ui.Center).Height(36).Padding(0, 12, 0, 8).Margin(0, 0, 0, -8).Radius(8).TextColor(t.TextMuted).Transition(motionState)
				if back.Hovered() {
					back.Background(t.SurfaceHover).TextColor(t.Text)
				}
				back.FocusRing(back.FocusVisible())
				if back.Clicked() {
					a.chrome.settingsTab = "Tentang"
				}
				back.Children(func() {
					ui.Icon(c, spaceIcon("left")).Size(16, 16)
					ui.Text(c, "Kembali ke Tentang").FontSize(13)
				})
				ui.Text(c, "Tentang Space › Dibuat oleh").FontSize(12).TextColor(faint(t)).MaxLines(1)
			})
			var split ui.Element
			if stacked {
				split = ui.Column(c.Key("split")).FillWidth().MinWidth(0).Gap(24)
			} else {
				split = ui.Row(c.Key("split")).FillWidth().MinWidth(0).Gap(32).AlignItems(ui.Start)
			}
			split.Children(func() {
				stage := ui.Column(c.Key("stage")).Height(360).Shrink(0).Radius(12).Background(t.Surface).Border(1, t.Border).Center()
				if stacked {
					stage.FillWidth()
				} else {
					stage.Width(360)
				}
				stage.Children(func() {
					markView(c.Key("mark"), &a.kredit.mark, 232, "Logo Tiyoouw, klik untuk memutar ulang")
				})
				ui.Column(c.Key("side")).Grow(1).MinWidth(0).Gap(20).Children(func() {
					ui.Column(c).FillWidth().MinWidth(0).Gap(6).Children(func() {
						ui.Text(c, "Dirancang dan dibangun oleh").FontSize(12).TextColor(faint(t))
						ui.Text(c, kreditOwner).FontSize(18).FontWeight(600).Selectable()
					})
					links := [][4]string{{"globe", "tiyoouw.com", "Situs web", kreditSite}, {"github", "github.com/Tiyouw", "GitHub", kreditGitHub}}
					kreditList(c, "owner-links", len(links), func(i int) {
						l := links[i]
						kreditLink(c, l[0], l[1], l[2], l[3], i == 1)
					})
					ui.Text(c, "Space by HIMASIF ASTRA 25/26").FontSize(12).TextColor(faint(t))
					ui.Column(c.Key("contributors")).FillWidth().MinWidth(0).Gap(8).Children(func() {
						ui.Column(c).FillWidth().MinWidth(0).Gap(2).Children(func() {
							ui.Text(c, "Kontributor").FontSize(15).Bold()
							muted(c, "Dari contributors.json, terpasang bersama aplikasi").FontSize(13)
						})
						people := Contributors.Contributors
						if len(people) == 0 {
							muted(c, "Daftar kontributor tidak tersedia di versi ini.").FontSize(13)
						} else {
							kreditList(c, "people", len(people), func(i int) { contributorRow(c, people[i]) })
						}
						repo := Contributors.Project.Repository
						if repo == "" {
							repo = kreditRepository
						}
						guide := strings.TrimSuffix(repo, "/") + "/blob/main/CONTRIBUTING.md"
						ui.Row(c.Key("contribute")).FillWidth().MinWidth(0).Gap(8).AlignItems(ui.Center).Wrap().Margin(4, 0, 0, 0).Children(func() {
							ui.Icon(c, spaceIcon("note")).Size(14, 14).Shrink(0).TextColor(t.TextMuted)
							muted(c, "Ingin ikut berkontribusi? Lihat").FontSize(13)
							l := ui.ButtonBase(c.Key("guide")).Label("CONTRIBUTING.md di GitHub").Tooltip(guide)
							l.FocusRing(l.FocusVisible())
							if l.Clicked() {
								c.OpenURL(guide)
							}
							line := t.Accent.Alpha(.4)
							if l.Hovered() {
								line = t.Accent
							}
							l.Children(func() {
								ui.Text(c, "CONTRIBUTING.md di GitHub").FontSize(13).BorderWidth(0, 0, 1, 0).BorderColor(line)
							})
						})
					})
				})
			})
		})
	})
}
