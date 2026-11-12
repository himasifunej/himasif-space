package appui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/guides"
)

// PathPanduan is the guide page.
const PathPanduan = "/panduan"

// GuidesAPI is the server's guide (/api/guides): Markdown that pengurus
// change on the server without a release. Without it, or offline, the app
// shows the copy built in.
type GuidesAPI interface {
	Guides(ctx context.Context) (api.GuideList, error)
}

type guideState struct {
	loading, loaded bool
	list            []api.Guide
	// builtIn tells the list is the app's own copy.
	builtIn bool
	query   string
	// role is the role whose topics the page lists; "" the account's.
	role string
	// open is the article the page shows; "" the topics.
	open string
	// panel is the "?" panel (a sheet on a phone), showing panelSlug.
	panel     bool
	panelSlug string
	md        markdown.Options
	mdReady   bool
}

func (a *App) guidesService() (GuidesAPI, bool) {
	if c, ok := a.api.(*client.Client); ok {
		return c, true
	}
	s, ok := a.api.(GuidesAPI)
	return s, ok
}

// ensureGuides loads the guide once per session: the server's, else the
// built-in copy.
func (a *App) ensureGuides() {
	st := &a.guide
	if st.loaded || st.loading {
		return
	}
	svc, ok := a.guidesService()
	if !ok {
		st.list, st.builtIn, st.loaded = guides.Embedded(), true, true
		return
	}
	gen := a.gen
	st.loading = true
	a.async(func() func() {
		list, err := svc.Guides(context.Background())
		return func() {
			if gen != a.gen {
				return
			}
			st := &a.guide
			st.loading, st.loaded = false, true
			if err != nil || len(list.Guides) == 0 {
				st.list, st.builtIn = guides.Embedded(), true
				return
			}
			st.list, st.builtIn = list.Guides, false
		}
	})
}

func (a *App) guideBySlug(slug string) (api.Guide, bool) {
	for _, g := range a.guide.list {
		if g.Slug == slug {
			return g, true
		}
	}
	return api.Guide{}, false
}

// guidePage is the "?" key of the page shown: board, catatan, ping,
// anggota, whatsapp, or beranda.
func (a *App) guidePage() string {
	switch a.router.Path() {
	case PathJobdesk:
		return "board"
	case PathSpaces, PathMedhum:
		switch a.chrome.tab {
		case "Board tugas", "Tinjauan":
			return "board"
		case "Catatan", "Rapat", "Agenda":
			return "catatan"
		case "Ping":
			return "ping"
		case "Anggota":
			return "anggota"
		case "Bot WhatsApp":
			return "whatsapp"
		}
	}
	return "beranda"
}

var guidePageNames = map[string]string{"board": "Board tugas", "catatan": "Catatan rapat", "ping": "Ping", "anggota": "Anggota", "whatsapp": "Bot WhatsApp", "beranda": "Beranda"}

// guidesForPage are the articles for page and the account's role, most
// relevant first.
func (a *App) guidesForPage(page string) []api.Guide {
	role := a.onboardRole()
	var out []api.Guide
	for _, g := range a.guide.list {
		if g.ForRole(role) && slices.Contains(g.Pages, page) {
			out = append(out, g)
		}
	}
	return out
}

// openGuidePanel opens the "?" panel on slug, or on the page's first
// article when slug is "".
func (a *App) openGuidePanel(slug string) {
	a.ensureGuides()
	if slug == "" {
		if gs := a.guidesForPage(a.guidePage()); len(gs) > 0 {
			slug = gs[0].Slug
		}
	}
	a.guide.panel, a.guide.panelSlug = true, slug
	a.chrome.appearance = false
}

// helpButton is the "?" of the top bar.
func (a *App) helpButton(c *ui.Context, size float32) {
	t := c.Theme()
	b := ui.ButtonBase(c.Key("help-button")).Label("Panduan untuk halaman ini").Tooltip("Panduan untuk halaman ini").Size(size, size).Shrink(0).Center().Radius(size/2).Border(1, t.Border).TextColor(t.TextMuted).Transition(ui.ElementTransition{Duration: 150 * time.Millisecond, Colors: true})
	if a.guide.panel {
		b.BorderColor(t.Accent).TextColor(t.Accent).Background(spaceMix(t.Accent, t.Surface, .9))
	} else if b.Hovered() {
		b.Background(t.SurfaceHover).TextColor(t.Text)
	}
	b.FocusRing(b.FocusVisible()).Children(func() { ui.Text(c, "?").FontSize(14).FontWeight(600) })
	onbPress(b)
	a.tourTarget("help", b)
	if b.Clicked() {
		if a.guide.panel {
			a.guide.panel = false
		} else {
			a.openGuidePanel("")
		}
	}
}

// guideMarkdown renders an article: Markdown, with "gambar" blocks drawn
// as small pictures of Space.
func (a *App) guideMarkdown(c *ui.Context, body string) {
	st := &a.guide
	if !st.mdReady {
		st.md.Code = map[string]func(c *ui.Context, b *markdown.CodeBlockView){
			"gambar": func(c *ui.Context, b *markdown.CodeBlockView) {
				guidePicture(c, strings.TrimSpace(b.Code()))
			},
		}
		st.md.OnLink = func(url string) {
			if slug, ok := strings.CutPrefix(url, "panduan:"); ok {
				a.guide.open, a.guide.panelSlug = slug, slug
			}
		}
		st.mdReady = true
	}
	markdown.View(c, body, &st.md).FillWidth().MinWidth(0)
}

// guideView is the Panduan page: topics by role with search, or one
// article.
func (a *App) guideView(c *ui.Context) {
	a.ensureGuides()
	st := &a.guide
	t := c.Theme()
	phone := isMobile(c)
	role := st.role
	if role == "" {
		role = a.onboardRole()
	}
	ui.Scroll(c.Key("panduan")).Fill().Children(func() {
		body := ui.Column(c).FillWidth().MinWidth(0).MaxWidth(pageMaxWidth).Padding(28).Gap(16)
		if phone {
			body.Padding(16).Gap(12)
		}
		body.Children(func() {
			if st.open != "" {
				if g, ok := a.guideBySlug(st.open); ok {
					a.guideArticle(c, g, role, phone)
					return
				}
				st.open = ""
			}
			head := ui.Column(c).FillWidth().Gap(12)
			if !phone {
				head.Row().AlignItems(ui.End)
			}
			head.Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
					ui.Text(c, "Panduan").FontSize(phoneSize(phone, 18, 22)).Bold()
					muted(c, "Topik untuk "+guideRoleName(role)+". Ganti role untuk melihat panduan peran lain.").FontSize(13)
				})
				roles := []string{api.GuideRoleMember, api.GuideRole45, api.GuideRoleKadiv}
				if i := segmented(c, "Role panduan", []string{"Anggota", "45", "Kadiv"}, slices.Index(roles, role)); i >= 0 {
					st.role = roles[i]
				}
			})
			ui.TextInput(c.Key("guide-search"), &st.query).Label("Cari panduan").Placeholder("Cari: undang, WhatsApp, timer, periode…").FillWidth().MaxWidth(560)
			list := a.guideMatches(role, st.query)
			if len(list) == 0 {
				pageEmpty(c, "search", "Tidak ada panduan yang cocok", "Coba kata lain, atau ganti role di atas.")
			}
			a.guideTopics(c, list, phone)
			ui.Row(c.Key("guide-tour")).FillWidth().MaxWidth(pageMaxWidth).Gap(12).Padding(12, 14).Radius(10).AlignItems(ui.Center).Background(spaceMix(t.Border, t.Surface, .55)).Children(func() {
				ui.Icon(c, spaceIcon("compass")).Size(16, 16).TextColor(t.TextMuted)
				ui.Text(c, "Tur 5 langkah bisa diulang kapan pun.").FontSize(13).Grow(1).MinWidth(0)
				if a.onboardingActive() && onbSecondary(c, "Ulangi tur", "Ulangi tur", true).Clicked() {
					a.startTour()
				}
			})
			if st.builtIn {
				muted(c, "Menampilkan panduan bawaan aplikasi; panduan terbaru dari server belum bisa diambil.").FontSize(11)
			}
		})
	})
}

func phoneSize(phone bool, small, large float32) float32 {
	if phone {
		return small
	}
	return large
}

// guideMatches are role's articles whose title, summary or text hold q.
func (a *App) guideMatches(role, q string) []api.Guide {
	q = strings.ToLower(strings.TrimSpace(q))
	var out []api.Guide
	for _, g := range a.guide.list {
		if !g.ForRole(role) {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(g.Title+" "+g.Summary+" "+g.Topic+" "+g.Body), q) {
			continue
		}
		out = append(out, g)
	}
	return out
}

// guideTopics lays the articles out by topic, as cards that come in one
// after another.
func (a *App) guideTopics(c *ui.Context, list []api.Guide, phone bool) {
	t := c.Theme()
	var topics []string
	by := map[string][]api.Guide{}
	for _, g := range list {
		if _, ok := by[g.Topic]; !ok {
			topics = append(topics, g.Topic)
		}
		by[g.Topic] = append(by[g.Topic], g)
	}
	cols := 3
	if phone {
		cols = 1
	}
	for row := 0; row*cols < len(topics); row++ {
		ui.Row(c.Key(fmt.Sprintf("topic-row-%d", row))).FillWidth().Gap(16).AlignItems(ui.Start).Children(func() {
			for col := 0; col < cols; col++ {
				i := row*cols + col
				if i >= len(topics) {
					ui.Box(c).Grow(1).Width(0)
					continue
				}
				topic := topics[i]
				enter := ui.ElementTransition{Animation: &ui.Animation{Duration: 260 * time.Millisecond, Ease: ui.EaseOut, Delay: time.Duration(i) * 45 * time.Millisecond}, Enter: &ui.Motion{Opacity: 0, Y: 10}}
				ui.Column(c.Key("topic-"+topic)).Grow(1).Width(0).MinWidth(0).Padding(14, 16).Gap(2).Radius(12).Background(t.Surface).Border(1, t.Border).Transition(enter).Children(func() {
					ui.Text(c, topic).FontSize(15).Bold().Padding(0, 0, 6, 0)
					for _, g := range by[topic] {
						if a.guideRow(c, g).Clicked() {
							a.guide.open = g.Slug
						}
					}
				})
			}
		})
	}
}

func (a *App) guideRow(c *ui.Context, g api.Guide) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c.Key("guide-"+g.Slug)).Label("Buka panduan "+g.Title).FillWidth().MinWidth(0).Row().Gap(10).Padding(8, 6).Radius(8).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).BorderColor(spaceMix(t.Border, t.Surface, .45)).Transition(ui.ElementTransition{Duration: 150 * time.Millisecond, Colors: true})
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		ui.Icon(c, spaceIcon("book")).Size(14, 14).Shrink(0).TextColor(t.TextMuted)
		ui.Text(c, g.Title).FontSize(13).Grow(1).MinWidth(0).MaxLines(2)
		ui.Text(c, fmt.Sprintf("%d mnt", max(1, g.Minutes))).FontSize(11).TextColor(faint(t)).Shrink(0)
	})
	return onbPress(b)
}

// guideArticle is one article on the page, with the way back and its
// neighbours of the same topic.
func (a *App) guideArticle(c *ui.Context, g api.Guide, role string, phone bool) {
	t := c.Theme()
	if onbLink(c, "Kembali ke semua panduan", "← Semua panduan").Clicked() {
		a.guide.open = ""
		return
	}
	ui.Column(c.Key("article-" + g.Slug)).FillWidth().MaxWidth(760).Gap(8).Transition(onbPop).Children(func() {
		ui.Text(c, fmt.Sprintf("Panduan · %s · %d mnt", g.Topic, max(1, g.Minutes))).FontSize(12).TextColor(faint(t))
		ui.Text(c, g.Title).FontSize(phoneSize(phone, 20, 24)).FontWeight(600)
		a.guideMarkdown(c, g.Body)
		var related []api.Guide
		for _, o := range a.guide.list {
			if o.Slug != g.Slug && o.Topic == g.Topic && o.ForRole(role) {
				related = append(related, o)
			}
		}
		if len(related) > 0 {
			ui.Text(c, "TERKAIT").FontSize(11).TextColor(faint(t)).Padding(12, 0, 0, 0)
			for _, o := range related {
				if a.guideRow(c, o).Clicked() {
					a.guide.open = o.Slug
				}
			}
		}
	})
}

// guidePanel is the "?" panel beside the page (desktop): it slides in.
func (a *App) guidePanel(c *ui.Context) {
	a.ensureGuides()
	t := c.Theme()
	ui.Column(c.Key("guide-panel")).Width(400).Shrink(0).FillHeight().MinHeight(0).Background(t.Surface).BorderWidth(0, 0, 0, 1).BorderColor(t.Border).Transition(onbSlide).Children(func() {
		a.guidePanelBody(c)
	})
}

// guideSheet is the "?" panel on a phone: a sheet from the bottom.
func (a *App) guideSheet(c *ui.Context) {
	a.ensureGuides()
	t := c.Theme()
	w, h := c.Size()
	ui.DialogBase(c.Key("guide-sheet"), &a.guide.panel, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, .45)).Justify(ui.End)
		panel.Width(w).Height(h*0.82).Radius(18, 18, 0, 0).Column().Background(t.Surface).Border(1, t.Border).
			Transition(ui.ElementTransition{Duration: 260 * time.Millisecond, Enter: &ui.Motion{Y: 60, Opacity: 0}}).Children(func() {
			a.guidePanelBody(c)
		})
	})
}

func (a *App) guidePanelBody(c *ui.Context) {
	t := c.Theme()
	page := a.guidePage()
	ui.Row(c).FillWidth().Padding(12, 16).Gap(8).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		ui.Icon(c, spaceIcon("book")).Size(16, 16).TextColor(t.Accent)
		ui.Text(c, "Panduan · "+guidePageNames[page]).FontSize(13).Bold().Grow(1).MinWidth(0).MaxLines(1)
		if spaceIconButton(c, "Buka di halaman Panduan", "panel").Clicked() {
			a.guide.open, a.guide.panel = a.guide.panelSlug, false
			a.navigate(PathPanduan)
		}
		if spaceIconButton(c, "Tutup panduan", "close").Clicked() {
			a.guide.panel = false
		}
	})
	ui.Scroll(c.Key("guide-panel-scroll")).Grow(1).MinHeight(0).FillWidth().Children(func() {
		ui.Column(c).FillWidth().MinWidth(0).Padding(16, 20).Gap(8).Children(func() {
			if !a.guide.loaded {
				muted(c, "Memuat panduan…")
				return
			}
			g, ok := a.guideBySlug(a.guide.panelSlug)
			if !ok {
				pageEmpty(c, "book", "Belum ada panduan untuk halaman ini", "Buka halaman Panduan untuk melihat semua topik.")
				return
			}
			ui.Column(c.Key("panel-" + g.Slug)).FillWidth().MinWidth(0).Gap(6).Transition(onbPop).Children(func() {
				ui.Text(c, fmt.Sprintf("%s · %d mnt", g.Topic, max(1, g.Minutes))).FontSize(11).TextColor(faint(t))
				ui.Text(c, g.Title).FontSize(17).FontWeight(600)
				a.guideMarkdown(c, g.Body)
			})
			others := a.guidesForPage(page)
			if len(others) > 1 {
				ui.Text(c, "TOPIK LAIN DI HALAMAN INI").FontSize(11).TextColor(faint(t)).Padding(12, 0, 0, 0)
				for _, o := range others {
					if o.Slug == g.Slug {
						continue
					}
					if a.guideRow(c, o).Clicked() {
						a.guide.panelSlug = o.Slug
					}
				}
			}
		})
	})
}

// guidePicture draws a small, schematic picture of a part of Space for an
// article: shapes in the theme's colors, the part meant marked in accent.
func guidePicture(c *ui.Context, name string) {
	t := c.Theme()
	line := func(w float32, strong bool) {
		col := spaceMix(t.Border, t.Surface, .3)
		if strong {
			col = t.Border
		}
		ui.Box(c).Width(w).Height(6).Radius(3).Background(col)
	}
	mini := func(fn func()) ui.Element {
		return ui.Column(c).Padding(8).Gap(6).Radius(8).Background(t.Surface).Border(1, spaceMix(t.Border, t.Surface, .4)).Children(fn)
	}
	mark := func(e ui.Element) { e.Border(2, t.Accent) }
	frame := ui.Column(c.Key("picture-"+name)).FillWidth().MaxWidth(520).Padding(12).Gap(8).Radius(10).Background(t.Background).Border(1, t.Border).Margin(4, 0)
	frame.Children(func() {
		switch name {
		case "board":
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				for i, col := range []string{"Belum mulai", "Dikerjakan", "Perlu tinjauan", "Selesai"} {
					ui.Column(c.Key(i)).Grow(1).Width(0).Gap(6).Children(func() {
						ui.Text(c, col).FontSize(10).TextColor(faint(t)).MaxLines(1)
						m := mini(func() { line(60, true); line(36, false) })
						if i == 1 {
							mark(m)
						}
					})
				}
			})
		case "catatan":
			mini(func() {
				line(140, true)
				for i := 1; i <= 2; i++ {
					ui.Row(c.Key(i)).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, fmt.Sprintf("%d.", i)).FontSize(10).TextColor(faint(t))
						line(90, false)
						guideTag(c, fmt.Sprintf("%d mnt", 10+i*5), false)
					})
				}
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(12, 12).Radius(3).Border(1.5, t.Border)
					line(80, false)
					guideTag(c, "PJ · Jum", true)
				})
			}).FillWidth()
		case "ping":
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				mini(func() {
					ui.Text(c, "Ping · Kadiv").FontSize(10).TextColor(faint(t))
					ui.Text(c, "Rapat rutin Senin 19.00 di sekret.").FontSize(12)
				}).Grow(1)
				ui.Column(c).Padding(8).Radius(8).Background(spaceMix(t.Success, t.Surface, .85)).Children(func() {
					ui.Text(c, "WhatsApp").FontSize(10).TextColor(faint(t))
					ui.Text(c, "!tugas").Font(FontMono).FontSize(12)
				})
			})
		case "anggota":
			mini(func() {
				for i, r := range [][2]string{{"Kadiv", "kadiv"}, {"45", "45"}, {"Anggota", "anggota"}} {
					ui.Row(c.Key(i)).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(18, 18).Radius(9).Background(spaceMix(t.Border, t.Surface, .2))
						line(80, true)
						ui.Box(c).Grow(1)
						onbRoleChip(c, r[1])
					})
				}
			}).FillWidth()
		case "whatsapp":
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				m := mini(func() {
					ui.Text(c, "Kode tautan").FontSize(10).TextColor(faint(t))
					ui.Text(c, "482 913").Font(FontMono).FontSize(16).FontWeight(500)
				})
				mark(m)
				ui.Icon(c, spaceIcon("arrow")).Size(16, 16).TextColor(t.TextMuted)
				ui.Column(c).Grow(1).Padding(8).Radius(8).Background(spaceMix(t.Success, t.Surface, .85)).Children(func() {
					ui.Text(c, "Bot divisi").FontSize(10).TextColor(faint(t))
					ui.Text(c, "Nomormu tertaut ke Space.").FontSize(12)
				})
			})
		case "beranda":
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				m := mini(func() {
					for i := 0; i < 5; i++ {
						line(48, i == 2)
					}
				}).Width(80).Shrink(0)
				mark(m)
				ui.Column(c).Grow(1).Gap(8).Children(func() {
					mini(func() { line(120, true); line(180, false) }).FillWidth()
					mini(func() { line(90, true); line(150, false) }).FillWidth()
				})
			})
		default: // tanya
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				mini(func() {
					ui.Row(c).FillWidth().Children(func() {
						line(80, true)
						ui.Box(c).Grow(1)
						ui.Box(c).Size(18, 18).Radius(9).Center().Border(1.5, t.Accent).Children(func() {
							ui.Text(c, "?").FontSize(10).TextColor(t.Accent)
						})
					})
					line(140, false)
					line(110, false)
				}).Grow(1)
				m := mini(func() { line(70, true); line(90, false); line(60, false) }).Width(110).Shrink(0)
				mark(m)
			})
		}
	})
}

func guideTag(c *ui.Context, text string, due bool) {
	t := c.Theme()
	bg, fg := spaceMix(t.Border, t.Surface, .5), t.TextMuted
	if due {
		bg, fg = spaceMix(t.Warning, t.Surface, .85), t.Warning
	}
	ui.Row(c).Padding(1, 6).Radius(5).Background(bg).Children(func() { ui.Text(c, text).FontSize(10).TextColor(fg) })
}
