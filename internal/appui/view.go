package appui

import (
	"github.com/egoist/mygo/ui"

	"github.com/himasifunej/himasif-space/internal/api"
)

// isAdmin reports whether the user is the organization's admin account.
func (a *App) isAdmin() bool { return a.user.Kind == api.AccountOrganizationAdmin }

// View builds the window's interface.
func (a *App) View(c *ui.Context) {
	a.loadPreferences()
	dark := a.chrome.preferences.Theme == "dark"
	if a.chrome.preferences.Theme == "system" {
		dark = c.Theme().Dark
	}
	c.SetTheme(spaceTheme(dark, a.chrome.preferences.Accent, a.chrome.preferences.Compact))
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).TextColor(t.Text).Children(func() {
		if a.accountFlowView(c) {
			return
		}
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

// brand draws the approved Space identity.
func brand(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Image(c, spaceBrand()).Size(40, 40).Fit(ui.Contain)
		ui.Column(c).Gap(2).Children(func() {
			ui.Text(c, "Space").FontSize(28).Bold()
			ui.Text(c, "Ruang Kerja HIMASIF").TextColor(t.TextMuted)
		})
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
		if spaceButton(c, "Lupa kata sandi", "lock", false).Disabled(a.busy).Clicked() {
			a.openRecovery()
		}
		if spaceButton(c, "Buka tautan undangan", "users", false).Disabled(a.busy).Clicked() {
			a.openAccountLink()
		}
		ui.Column(c).Gap(4).Children(func() {
			muted(c, "Akun dibuat lewat undangan dari kadiv.")
			muted(c, "Server: "+a.ServerURL)
		})
	})
}
