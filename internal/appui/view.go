package appui

import (
	"github.com/egoist/mygo/ui"

	"github.com/himasifunej/himasif-space/internal/api"
)

// navItem is a page of the sidebar.
type navItem struct {
	path, label string
	adminOnly   bool
}

// nav lists the shell's pages, as PRD §19.1 orders them.
var nav = []navItem{
	{PathJobdesk, "Jobdesk saya", false},
	{PathReminder, "Reminder pribadi", false},
	{PathSpaces, "Space divisi", false},
	{PathMedhum, "Medhum", false},
	{PathEvaluasi, "Evaluasi SOP", false},
	{PathArsip, "Arsip", false},
	{PathPengaturan, "Pengaturan", false},
	{PathOrganisasi, "Pengaturan organisasi", true},
}

// isAdmin reports whether the user is the organization's admin account.
func (a *App) isAdmin() bool { return a.user.Kind == api.AccountOrganizationAdmin }

// View builds the window's interface.
func (a *App) View(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).TextColor(t.Text).Children(func() {
		switch a.screen {
		case screenStarting:
			a.starting(c)
		case screenOffline:
			a.offline(c)
		case screenShell:
			a.shell(c)
		default:
			a.loginView(c)
		}
	})
}

// muted builds a small secondary text.
func muted(c *ui.Context, s string) ui.Element {
	return ui.Text(c, s).FontSize(12).TextColor(c.Theme().TextMuted)
}

// brand builds the name of the app, text only until the logo is designed
// (PRD OPEN-01).
func brand(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Gap(2).Children(func() {
		ui.Text(c, "Space").FontSize(28).Bold()
		ui.Text(c, "Ruang Kerja HIMASIF").TextColor(t.TextMuted)
	})
}

// card builds a centered box of at most 380 DIPs wide.
func card(c *ui.Context, fn func()) {
	t := c.Theme()
	ui.Column(c).Fill().Center().Padding(24).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(380).Padding(28).Gap(16).Radius(12).
			Background(t.Surface).Border(1, t.Border).Children(fn)
	})
}

func (a *App) starting(c *ui.Context) {
	ui.Column(c).Fill().Center().Gap(12).Children(func() {
		ui.Text(c, "Space").FontSize(28).Bold()
		ui.Row(c).Gap(8).Children(func() {
			ui.Spinner(c).Label("Menyambungkan")
			muted(c, "Menyambungkan…")
		})
	})
}

func (a *App) offline(c *ui.Context) {
	t := c.Theme()
	card(c, func() {
		brand(c)
		ui.Text(c, "Tidak ada koneksi").Bold()
		ui.Text(c, a.startErr).TextColor(t.TextMuted)
		if ui.PrimaryButton(c, "Coba lagi").Disabled(a.inflight > 0).Clicked() {
			a.Start()
		}
		muted(c, "Server: "+a.ServerURL)
	})
}

func (a *App) loginView(c *ui.Context) {
	t := c.Theme()
	card(c, func() {
		brand(c)
		if a.notice != "" {
			ui.Text(c, a.notice).TextColor(t.Warning)
		}
		submit := false
		ui.Field(c, "Email", func() {
			if ui.TextInput(c.Key("email"), &a.email).Placeholder("nama@email.com").AutoFocus().Disabled(a.busy).Submitted() {
				submit = true
			}
		})
		ui.Field(c, "Kata sandi", func() {
			if ui.TextInput(c.Key("password"), &a.password).Password().Disabled(a.busy).Submitted() {
				submit = true
			}
		})
		if a.loginErr != "" {
			ui.Text(c, a.loginErr).TextColor(t.Danger)
		}
		if ui.PrimaryButton(c, "Masuk").Disabled(a.busy).Clicked() {
			submit = true
		}
		if submit {
			a.login()
		}
		ui.Column(c).Gap(4).Children(func() {
			muted(c, "Akun dibuat lewat undangan dari kadiv.")
			muted(c, "Server: "+a.ServerURL)
		})
	})
}

func (a *App) shell(c *ui.Context) {
	t := c.Theme()
	if a.lastPing.IsZero() {
		a.lastPing = c.Now()
	}
	if next := a.lastPing.Add(pingEvery); !c.Now().Before(next) {
		a.ping(c.Now())
	} else {
		c.After(next.Sub(c.Now()))
	}
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(248).Shrink(0).PaddingY(12).Gap(8).Background(t.Surface).
			BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Children(func() {
			a.periodPicker(c)
			page := a.router.Path()
			ui.Sidebar(c, &page, func() {
				for _, it := range nav {
					if it.adminOnly && !a.isAdmin() {
						continue
					}
					ui.SidebarItem(c.Key(it.path), it.path, nil, it.label)
				}
			}).Grow(1).Label("Navigasi").OnChange(func() {
				a.router.Push(page)
			})
			a.account(c)
		})
		a.router.View(c, func(r *ui.Route) {
			a.page(c, r)
		}).Grow(1).MinWidth(0)
	})
}

func (a *App) periodPicker(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).PaddingX(12).Gap(4).Children(func() {
		muted(c, "Periode")
		switch {
		case len(a.periods) > 0:
			labels := make([]string, len(a.periods))
			chosen := ""
			for i, p := range a.periods {
				labels[i] = periodLabel(p)
				if p.ID == a.periodID {
					chosen = labels[i]
				}
			}
			if ui.Select(c.Key("period"), &chosen, labels).FillWidth().Label("Periode").Changed() {
				for i, l := range labels {
					if l == chosen {
						a.periodID = a.periods[i].ID
					}
				}
			}
		case a.periodsLoading:
			muted(c, "Memuat periode…")
		case a.periodsErr != "":
			ui.Text(c, a.periodsErr).FontSize(12).TextColor(t.Danger)
			if ui.Button(c, "Muat ulang").Clicked() {
				a.loadPeriods()
			}
		default:
			muted(c, "Belum ada periode.")
		}
	})
}

// account builds the user and the connection, at the bottom of the
// sidebar.
func (a *App) account(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Padding(12, 12, 0, 12).Gap(2).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
		ui.Text(c, a.user.DisplayName).Bold().MaxLines(1)
		muted(c, a.user.Email).MaxLines(1)
		color, text := t.Success, "Tersambung"
		switch a.conn {
		case connOffline:
			color, text = t.Danger, "Tidak ada koneksi"
		case connConnecting:
			color, text = t.Warning, "Menyambungkan…"
		}
		ui.Row(c).Gap(6).PaddingY(6).Children(func() {
			ui.Box(c).Size(8, 8).Radius(4).Background(color)
			muted(c, text).Grow(1)
			if a.conn == connOffline && ui.Button(c, "Coba lagi").Clicked() {
				a.ping(c.Now())
			}
		})
	})
}
