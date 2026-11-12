package appui

import (
	_ "embed"
	"encoding/json"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// changelogJSON is every version's notes, newest first: the app shows
// them offline (Pengaturan › Tentang), and publish_update.py signs the
// version's into its update manifest, the one source of both.
//
//go:embed changelog.json
var changelogJSON []byte

// ReleaseNotes is what a version brought, in a few short lines.
type ReleaseNotes struct {
	Version string   `json:"version"`
	Date    string   `json:"date"`
	Title   string   `json:"title"`
	Notes   []string `json:"notes"`
}

// Changelog is the notes of every version, newest first.
var Changelog = func() []ReleaseNotes {
	var out []ReleaseNotes
	if err := json.Unmarshal(changelogJSON, &out); err != nil {
		panic("appui: changelog.json: " + err.Error())
	}
	return out
}()

// ChangelogFor returns the notes of version, a release candidate's being
// its release's (0.4.5-rc2 is 0.4.5).
func ChangelogFor(version string) (ReleaseNotes, bool) {
	base, _, _ := strings.Cut(strings.TrimPrefix(version, "v"), "-")
	for _, r := range Changelog {
		if r.Version == base {
			return r, true
		}
	}
	return ReleaseNotes{}, false
}

// NoteLines splits a manifest's notes into its lines, dropping bullets:
// "• a\n• b" (or "- a") is ["a", "b"]. A note of one sentence, as older
// manifests have, is one line.
func NoteLines(notes string) []string {
	var out []string
	for _, line := range strings.Split(notes, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimSpace(strings.TrimLeft(line, "•-*"))
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// FormatNotes is lines as a manifest's notes: one bullet a line.
func FormatNotes(lines []string) string {
	var b strings.Builder
	for i, l := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("• " + l)
	}
	return b.String()
}

// WhatsNew is the dialog shown once after Space updated itself to Version.
type WhatsNew struct {
	Version string
	Notes   []string
}

// whatsNewDialog shows what the version just installed brought, once:
// "Mengerti" closes it for good (SeenWhatsNew).
func (a *App) whatsNewDialog(c *ui.Context) {
	wn := a.WhatsNew
	if wn == nil || a.screen != screenShell || len(wn.Notes) == 0 {
		return
	}
	// A device yet to meet Ace shows the notes in Ace's dialog, and a new
	// account's onboarding covers them: one dialog, never two.
	if (a.Ace != nil && !a.Ace.Introduced()) || a.onb.welcome {
		return
	}
	t := c.Theme()
	open := true
	ui.DialogBase(c.Key("whats-new"), &open, func(backdrop, panel ui.Element) {
		dialogMotion(backdrop, panel)
		backdrop.Background(ui.RGBA(0, 0, 0, .45))
		panel.Width(440).MaxWidth(440).Padding(22).Gap(12).Radius(14).Background(t.Surface).Border(1, t.Border).Label("Yang baru di Space " + wn.Version).Children(func() {
			ui.Text(c, "Yang baru di "+wn.Version).FontSize(18).Bold()
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				for i, n := range wn.Notes {
					ui.Row(c.Key(i)).FillWidth().Gap(8).AlignItems(ui.Start).Children(func() {
						ui.Text(c, "•").FontSize(14).TextColor(t.Accent).Shrink(0)
						ui.Text(c, n).FontSize(14).Grow(1).MinWidth(0)
					})
				}
			})
			ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
				if primarySpaceButton(c, "Mengerti", "check").Clicked() {
					open = false
				}
			})
		})
	})
	if !open {
		a.WhatsNew = nil
		if a.SeenWhatsNew != nil {
			a.SeenWhatsNew(wn.Version)
		}
	}
}

// changelogSettings is Pengaturan › Tentang: the version, and the notes
// of every version, bundled with the app.
func (a *App) changelogSettings(c *ui.Context) {
	settingsBody(c, "Tentang Space", "Versi terpasang dan riwayat perubahan. Tersedia juga tanpa koneksi.", func() {
		t := c.Theme()
		if a.Version != "" {
			muted(c, "Space "+a.Version).FontSize(13)
		}
		ui.Text(c, "Riwayat perubahan").FontSize(16).Bold().Padding(8, 0, 0, 0)
		for _, r := range Changelog {
			ui.Column(c.Key("v"+r.Version)).FillWidth().Gap(6).Padding(12, 0).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Wrap().Children(func() {
					ui.Text(c, r.Version).Font(FontMono).FontSize(14).Bold()
					if d, err := time.Parse("2006-01-02", r.Date); err == nil {
						muted(c, d.Format("2")+" "+noteMonths[d.Month()-1]+" "+d.Format("2006")).FontSize(12)
					}
					ui.Text(c, r.Title).FontSize(13).TextColor(t.TextMuted).Grow(1).MinWidth(0)
				})
				for i, n := range r.Notes {
					ui.Row(c.Key(i)).FillWidth().Gap(8).AlignItems(ui.Start).Children(func() {
						ui.Text(c, "•").FontSize(13).TextColor(t.Accent).Shrink(0)
						ui.Text(c, n).FontSize(13).Grow(1).MinWidth(0)
					})
				}
			})
		}
	})
}
