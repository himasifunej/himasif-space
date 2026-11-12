package appui

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Onboarding guided by Ace (canvas db456de51efb, option A): a card of at
// most five steps, each skippable, while Ace speaks from the real island
// with a face of its own per step. Kadiv get a step to invite their
// division. The web, without an island, skips Ace's steps. Accounts that
// were onboarded before meet Ace once, in the dialog after an update
// (aceUpdateDialog), together with its "Yang baru".

// Onboarding steps, by key.
const (
	aceStepProfile = "profil"
	aceStepTeam    = "divisi"
	aceStepLook    = "ace-tampilan"
	aceStepWhen    = "ace-kapan"
	aceStepReach   = "kabar"
	aceStepDone    = "selesai"
)

// aceLine is what Ace says on a step, and its face.
type aceLine struct{ text, face string }

var aceLines = map[string]aceLine{
	aceStepProfile: {"Halo, aku Ace! Aku nyolek kamu kalau ada tugas, ping, atau rapat.", AceFaceHappy},
	aceStepTeam:    {"Kamu kadiv! Ajak timmu dulu yuk. Tempel NIM-nya, undangannya aku antar.", AceFaceHappy},
	aceStepLook:    {"Taruh aku di mana? Seret aku ke kiri, tengah, atau kanan. Aku nggak gigit.", AceFaceLook},
	aceStepWhen:    {"Aku nongol terus, atau pas ada kabar aja? Coba pencet tombol di kabar ini.", ""},
	aceStepReach:   {"Biar kabarku sampai walau Space ditutup, aku perlu izin Windows.", AceFaceWink},
	aceStepDone:    {"Ace siap menemani! Aku nunggu di atas ya.", AceFaceHappy},
}

// aceUpdateLine is Ace's line in the dialog after an update.
var aceUpdateLine = aceLine{"Psst, aku sekarang bisa dipindah dan ganti baju.", AceFaceWink}

// aceQuietDefault are the quiet hours offered: 22.00 to 06.00.
const aceQuietFrom, aceQuietTo = 22, 6

// aceWizardState is the onboarding card's progress.
type aceWizardState struct {
	step int
	// said is the step Ace last spoke for, so it speaks once per step;
	// spoken whether the island showed it.
	said   string
	spoken bool
	// openWA opens Bot WhatsApp when the card closes.
	openWA bool
	// invite is the kadiv step's NIMs, and its sending.
	invite                  string
	inviteBusy              bool
	inviteSent, inviteFails int
	inviteErr               string
	// update: the dialog after an update has spoken.
	updateSaid bool
}

// aceSteps are the account's steps: the kadiv's invitation step only for
// who may invite, Ace's steps only where there is an island.
func (a *App) aceSteps() []string {
	steps := []string{aceStepProfile}
	if a.aceCanInvite() {
		steps = append(steps, aceStepTeam)
	}
	if a.Ace != nil {
		steps = append(steps, aceStepLook, aceStepWhen)
	}
	return append(steps, aceStepReach, aceStepDone)
}

// aceInviteDivision is the division of the onboarding Space where the
// account may invite members, else id.Nil.
func (a *App) aceInviteDivision() id.ID {
	sp := a.onboardingSpace()
	if sp == nil || sp.ReadOnly {
		return id.Nil
	}
	for _, r := range sp.Roles {
		if slices.Contains(r.Capabilities, api.CapInvitationsManage) {
			return r.DivisionID
		}
	}
	return id.Nil
}

func (a *App) aceCanInvite() bool { return a.aceInviteDivision() != id.Nil }

// aceSpeak has Ace say step's line once; spoken records whether the island
// shows it, else the card does.
func (a *App) aceSpeak(key string, line aceLine) {
	st := &a.onb.ace
	if st.said == key {
		return
	}
	st.said, st.spoken = key, false
	if a.Ace != nil {
		st.spoken = a.Ace.Say(line.text, line.face)
	}
}

// aceWizard is the onboarding card, open while the welcome is due.
func (a *App) aceWizard(c *ui.Context) {
	t := c.Theme()
	w, h := c.Size()
	phone := w < mobileMaxWidth
	st := &a.onb.ace
	steps := a.aceSteps()
	st.step = min(max(st.step, 0), len(steps)-1)
	key := steps[st.step]
	line := aceLines[key]
	a.aceSpeak(key, line)
	open := true
	ui.DialogBase(c.Key("ace-wizard"), &open, func(backdrop, panel ui.Element) {
		dialogMotion(backdrop, panel)
		backdrop.Background(ui.RGBA(0, 0, 0, .45))
		if phone {
			backdrop.Justify(ui.End)
			panel.Width(w).MaxHeight(h-40).Radius(18, 18, 0, 0)
		} else {
			// Room above the card for Ace speaking from the island.
			backdrop.Justify(ui.Start).Padding(min(float32(120), h/6), 0, 0, 0)
			panel.Width(min(float32(520), w-48)).MaxHeight(h - 160).Radius(16)
		}
		panel.Column().Background(t.Surface).Border(1, t.Border).Transition(onbPop).Label("Kenalan dengan Space").Children(func() {
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Padding(16, 20, 0, 20).Children(func() {
				ui.Text(c, fmt.Sprintf("%d DARI %d · %s", st.step+1, len(steps), strings.ToUpper(aceStepTitle(key)))).FontSize(11).TextColor(faint(t)).Grow(1).MinWidth(0).MaxLines(1)
				ui.Row(c).Gap(4).Shrink(0).Children(func() {
					for i := range steps {
						dot := ui.Box(c.Key(i)).Size(16, 4).Radius(2).Background(t.Border)
						if i <= st.step {
							dot.Background(t.Accent)
						}
					}
				})
			})
			ui.Scroll(c.Key("ace-scroll")).Grow(1).MinHeight(0).FillWidth().Children(func() {
				ui.Column(c).FillWidth().Padding(10, 20, 16, 20).Gap(12).Children(func() {
					if !st.spoken {
						a.aceSaysInCard(c, line)
					}
					a.aceStepBody(c, key)
				})
			})
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Padding(12, 20, 16, 20).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
				a.aceStepFooter(c, key, st.step == len(steps)-1)
			})
		})
	})
	if !open {
		a.aceFinish(false)
	}
}

// aceSaysInCard is Ace's line in the card, where the island cannot say it.
func (a *App) aceSaysInCard(c *ui.Context, line aceLine) {
	t := c.Theme()
	ui.Row(c.Key("ace-says")).FillWidth().Gap(10).AlignItems(ui.Center).Padding(10, 12).Radius(12).Background(spaceMix(t.Accent, t.Surface, .9)).Children(func() {
		aceEyes(c, line.face, 1)
		ui.Text(c, line.text).FontSize(13).Grow(1).MinWidth(0)
	})
}

// aceEyes draws Ace's two capsules small, for the card: wide, happy
// (short), or winking.
func aceEyes(c *ui.Context, face string, scale float32) {
	t := c.Theme()
	ui.Row(c).Size(34*scale, 22*scale).Shrink(0).Radius(11 * scale).Background(t.Text).Gap(4 * scale).Center().Children(func() {
		for side := range 2 {
			h := 11 * scale
			switch {
			case face == AceFaceHappy:
				h = 5 * scale
			case face == AceFaceWink && side == 0:
				h = 2 * scale
			}
			ui.Box(c.Key(side)).Size(5*scale, h).Radius(2.5 * scale).Background(t.Surface)
		}
	})
}

func aceStepTitle(key string) string {
	return map[string]string{aceStepProfile: "Profil", aceStepTeam: "Siapkan divisi", aceStepLook: "Kenalan dengan Ace", aceStepWhen: "Kenalan dengan Ace", aceStepReach: "Kabar sampai", aceStepDone: "Selesai"}[key]
}

// aceStepBody is a step's title, note and controls.
func (a *App) aceStepBody(c *ui.Context, key string) {
	t := c.Theme()
	title := func(head, note string) {
		ui.Text(c, head).FontSize(19).FontWeight(600)
		muted(c, note).FontSize(13)
	}
	switch key {
	case aceStepProfile:
		first := firstName(a.user.DisplayName)
		if first == "" {
			first = "kamu"
		}
		title("Hai, "+first, "Periksa nama yang dilihat rekan di board, catatan, dan ping.")
		if a.onb.profileName == "" && !a.onb.profileBusy {
			a.onb.profileName = a.user.DisplayName
		}
		ui.TextInput(c.Key("ace-name"), &a.onb.profileName).Label("Nama yang dilihat rekan").Placeholder("Nama yang biasa dipanggil").FillWidth().Disabled(a.onb.profileBusy)
		if a.onb.profileErr != "" {
			ui.Text(c, a.onb.profileErr).FontSize(12).TextColor(t.Danger)
		}
		ui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			onbRoleChip(c, a.onboardRole())
			if sp := a.onboardingSpace(); sp != nil {
				muted(c, sp.Name).FontSize(13)
			}
			if p := a.period(); p != nil {
				muted(c, "· periode "+p.Label).FontSize(13)
			}
		})
		muted(c, "Divisi dan role diambil dari undanganmu.").FontSize(12)
	case aceStepTeam:
		a.aceInviteStep(c)
	case aceStepLook:
		title("Tampilan Ace", "Setiap pilihan langsung dipakai. Bawaan: tengah, ikut tema Space.")
		ui.Text(c, "Posisi · atau seret Ace langsung di atas layar").FontSize(12).TextColor(faint(t))
		sel := -1
		switch an := a.Ace.Anchor(); {
		case an <= .02:
			sel = 0
		case an >= .98:
			sel = 2
		case an > .48 && an < .52:
			sel = 1
		}
		if i := segmented(c, "Posisi Ace", []string{"Kiri", "Tengah", "Kanan"}, sel); i >= 0 && i != sel {
			a.Ace.SetAnchor([]float64{0, .5, 1}[i])
		}
		settingsApart(func() { a.Ace.Look(c) })
	case aceStepWhen:
		title("Kapan Ace muncul", "Bawaan: selalu tampil, tanpa jam tenang.")
		sel := 1
		if a.Ace.AlwaysOn() {
			sel = 0
		}
		if i := segmented(c, "Kapan Ace tampil", []string{"Selalu", "Saat ada kabar"}, sel); i >= 0 && i != sel {
			a.Ace.SetAlwaysOn(i == 0)
		}
		on, from, to := a.Ace.QuietHours()
		if !on {
			from, to = aceQuietFrom, aceQuietTo
		}
		quiet := on
		if aceSwitchRow(c, "ace-quiet", "Jam tenang", fmt.Sprintf("%02d.00–%02d.00 · Ace diam dan bel tidak berbunyi; kabar menunggu sampai jam tenang selesai.", from, to), &quiet) {
			a.Ace.SetQuietHours(quiet, from, to)
		}
		ui.Row(c.Key("ace-try")).FillWidth().Gap(10).AlignItems(ui.Center).Padding(10, 12).Radius(12).Border(1, t.Border).Children(func() {
			ui.Icon(c, spaceIcon("bell")).Size(16, 16).TextColor(t.TextMuted).Shrink(0)
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, "Kabar percobaan").FontSize(13).FontWeight(600)
				muted(c, "Muncul di Ace dengan tombol Buka, Tunda, dan Tutup.").FontSize(12)
			})
			if onbSecondary(c, "Coba kabar", "Coba kabar", true).Clicked() {
				a.Ace.Try()
			}
		})
	case aceStepReach:
		title("Supaya kabar sampai", "Satu klik meminta izin. Sisanya sudah menyala; matikan bila tidak perlu.")
		a.aceReachRows(c)
	case aceStepDone:
		title("Ace siap menemani", "Semua bisa diubah lagi di Pengaturan › Perangkat ini.")
		for _, line := range a.aceSummary() {
			ui.Row(c.Key(line)).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, spaceIcon("check")).Size(15, 15).TextColor(t.Success).Shrink(0)
				ui.Text(c, line).FontSize(13).Grow(1).MinWidth(0)
			})
		}
	}
}

// aceSwitchRow is a label, its note, and a switch for *on, as Pengaturan
// has them (a click on the label toggles too); it reports a change.
func aceSwitchRow(c *ui.Context, key, title, note string, on *bool) bool {
	changed := false
	ui.Column(c.Key(key)).FillWidth().Children(func() {
		settingsApart(func() { changed = SettingSwitch(c, title, note, on) })
	})
	return changed
}

// aceReachRows are the notification permission, WhatsApp, and on the
// desktop start-up and automatic updates.
func (a *App) aceReachRows(c *ui.Context) {
	t := c.Theme()
	st := &a.onb.ace
	row := func(key, title, note string, action func()) {
		ui.Row(c.Key(key)).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, title).FontSize(13).FontWeight(500)
				muted(c, note).FontSize(12)
			})
			if action != nil {
				action()
			}
		})
	}
	name := "Notifikasi Windows"
	if a.Ace == nil {
		name = "Notifikasi"
	}
	switch perm := a.noticePermission(); perm {
	case BrowserPermissionGranted:
		row("notif", name, "Diizinkan.", func() { ui.Icon(c, spaceIcon("check")).Size(18, 18).TextColor(t.Success) })
	case BrowserPermissionDefault, BrowserPermissionDenied:
		note := "Supaya pengingat dan ping muncul walau Space tertutup."
		if perm == BrowserPermissionDenied {
			note = a.noticeDeniedHint()
		}
		row("notif", name, note, func() {
			if onbPrimary(c, "Izinkan notifikasi", "Izinkan", true).Clicked() {
				a.doOnboardingStep(onbNotif)
			}
		})
	}
	if a.onb.data.WhatsAppLinked {
		row("wa", "WhatsApp", "Sudah tertaut.", func() { ui.Icon(c, spaceIcon("check")).Size(18, 18).TextColor(t.Success) })
	} else {
		note := "Opsional. Pengingat dan ping juga masuk ke WhatsApp-mu."
		if st.openWA {
			note = "Bot WhatsApp terbuka setelah langkah ini selesai."
		}
		on := st.openWA
		if aceSwitchRow(c, "wa", "Tautkan WhatsApp", note, &on) {
			st.openWA = on
		}
	}
	if a.Ace == nil {
		return
	}
	on := a.Ace.Autostart()
	if aceSwitchRow(c, "autostart", "Mulai bersama Windows", "Space jalan diam di tray; kabar tetap masuk walau jendelanya ditutup.", &on) {
		if err := a.Ace.SetAutostart(on); err != nil {
			a.addNotice("Mulai bersama Windows belum diubah: " + err.Error())
		}
	}
	auto := a.Ace.AutoUpdate()
	if aceSwitchRow(c, "autoupdate", "Pembaruan otomatis", "Versi baru diunduh di latar dan dipasang saat jendela Space tertutup.", &auto) {
		a.Ace.SetAutoUpdate(auto)
	}
}

// aceSummary is the last step's list of what was set.
func (a *App) aceSummary() []string {
	out := []string{"Profil: " + firstNonEmpty(strings.TrimSpace(a.onb.profileName), a.user.DisplayName)}
	if st := a.onb.ace; st.inviteSent > 0 {
		out = append(out, fmt.Sprintf("%d undangan terkirim", st.inviteSent))
	}
	if a.Ace != nil {
		where := "tengah"
		switch an := a.Ace.Anchor(); {
		case an <= .02:
			where = "kiri"
		case an >= .98:
			where = "kanan"
		case an < .48 || an > .52:
			where = "posisi pilihanmu"
		}
		when := "selalu tampil"
		if !a.Ace.AlwaysOn() {
			when = "muncul saat ada kabar"
		}
		line := "Ace: " + where + " · " + when
		if on, from, to := a.Ace.QuietHours(); on {
			line += fmt.Sprintf(" · jam tenang %02d–%02d", from, to)
		}
		out = append(out, line)
	}
	if a.noticePermission() == BrowserPermissionGranted {
		out = append(out, "Notifikasi diizinkan")
	}
	if a.onb.data.WhatsAppLinked {
		out = append(out, "WhatsApp tertaut")
	} else if a.onb.ace.openWA {
		out = append(out, "Bot WhatsApp dibuka setelah ini")
	}
	return out
}

// aceStepFooter is Lewati and the step's main button.
func (a *App) aceStepFooter(c *ui.Context, key string, last bool) {
	st := &a.onb.ace
	if last {
		if onbSecondary(c, "Ke Beranda", "Ke Beranda", false).Clicked() {
			a.aceFinish(false)
		}
		ui.Box(c).Grow(1)
		if onbPrimary(c, "Mulai tur · 1 menit", "Mulai tur · 1 menit", false).Clicked() {
			a.aceFinish(true)
		}
		return
	}
	if onbLink(c, "Lewati langkah ini", "Lewati").Clicked() {
		a.aceNext()
	}
	if onbLink(c, "Lewati semua langkah, pakai bawaan", "Lewati semua").Clicked() {
		a.aceFinish(false)
	}
	ui.Box(c).Grow(1)
	switch key {
	case aceStepProfile:
		if onbPrimary(c, "Simpan dan lanjut", "Lanjut", false).Disabled(a.onb.profileBusy).Clicked() {
			if strings.Join(strings.Fields(a.onb.profileName), " ") != a.user.DisplayName {
				a.saveProfile()
				if a.onb.profileErr != "" {
					return
				}
			} else {
				a.markOnboarding(api.OnboardProfile)
			}
			a.aceNext()
		}
	case aceStepTeam:
		nims, bad := aceNIMs(st.invite)
		label := "Lanjut"
		if len(nims) > 0 && st.inviteSent == 0 {
			label = fmt.Sprintf("Kirim %d undangan", len(nims))
		}
		if st.inviteBusy {
			label = "Mengirim…"
		}
		if onbPrimary(c, label, label, false).Disabled(st.inviteBusy || (len(nims) == 0 && len(bad) > 0)).Clicked() {
			if len(nims) > 0 && st.inviteSent == 0 {
				a.aceSendInvites(nims)
			} else {
				a.aceNext()
			}
		}
	default:
		if onbPrimary(c, "Lanjut", "Lanjut", false).Clicked() {
			a.aceNext()
		}
	}
}

func (a *App) aceNext() {
	a.onb.ace.step++
}

// aceFinish closes the onboarding: the welcome is done for the account,
// Ace introduced on this device, and what was asked for opens.
func (a *App) aceFinish(tour bool) {
	st := &a.onb.ace
	openWA := st.openWA
	if a.Ace != nil {
		a.Ace.Hush()
		a.Ace.SetIntroduced()
	}
	// A new account's onboarding covers what the version brought.
	if wn := a.WhatsNew; wn != nil {
		a.WhatsNew = nil
		if a.SeenWhatsNew != nil {
			a.SeenWhatsNew(wn.Version)
		}
	}
	a.onb.ace = aceWizardState{}
	a.dismissWelcome()
	switch {
	case tour:
		a.startTour()
	case openWA:
		a.doOnboardingStep("wa")
	}
}

// aceNIMs reads the kadiv's list: NIMs or emails, one a line or split by
// commas; good are the ones an invitation can go to, bad the rest.
func aceNIMs(text string) (good []api.InviteRequest, bad []string) {
	seen := map[string]bool{}
	for _, f := range strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\r' }) {
		target := inviteTarget(f)
		key := strings.ToLower(target.email)
		switch {
		case target.req == nil:
			bad = append(bad, f)
		case !seen[key]:
			seen[key] = true
			good = append(good, *target.req)
		}
	}
	return good, bad
}

// aceInviteStep is the kadiv's step: paste NIMs, send the invitations.
func (a *App) aceInviteStep(c *ui.Context) {
	t := c.Theme()
	st := &a.onb.ace
	name := "divisimu"
	if sp := a.onboardingSpace(); sp != nil {
		name = sp.Name
	}
	ui.Text(c, "Siapkan "+name).FontSize(19).FontWeight(600)
	muted(c, "Undang anggota lewat NIM; undangan dikirim ke email kampus mereka. Bisa juga nanti dari tab Anggota.").FontSize(13)
	ui.TextArea(c.Key("ace-invite"), &st.invite).Label("NIM anggota, satu per baris").FillWidth().Height(96).Disabled(st.inviteBusy || st.inviteSent > 0)
	nims, bad := aceNIMs(st.invite)
	switch {
	case st.inviteSent > 0 || st.inviteFails > 0:
		msg := fmt.Sprintf("%d undangan terkirim.", st.inviteSent)
		if st.inviteFails > 0 {
			msg += fmt.Sprintf(" %d belum terkirim: %s", st.inviteFails, st.inviteErr)
		}
		ui.Text(c, msg).FontSize(12).TextColor(t.TextMuted)
	case len(nims) > 0 || len(bad) > 0:
		msg := fmt.Sprintf("%d siap diundang", len(nims))
		if len(bad) > 0 {
			msg += fmt.Sprintf(" · %d bukan NIM atau email: %s", len(bad), strings.Join(bad[:min(len(bad), 3)], ", "))
		}
		ui.Text(c, msg).FontSize(12).TextColor(t.TextMuted)
	}
}

// aceSendInvites sends the invitations one after another, then reports.
func (a *App) aceSendInvites(reqs []api.InviteRequest) {
	svc, ok := a.workspaceService()
	did := a.aceInviteDivision()
	if !ok || did == id.Nil {
		return
	}
	st := &a.onb.ace
	gen, pid := a.gen, a.periodID
	st.inviteBusy, st.inviteErr = true, ""
	a.async(func() func() {
		sent, fails := 0, 0
		var last error
		for _, r := range reqs {
			if _, err := svc.Invite(context.Background(), pid, did, r); err != nil {
				fails++
				last = err
			} else {
				sent++
			}
		}
		return func() {
			if gen != a.gen {
				return
			}
			st := &a.onb.ace
			st.inviteBusy, st.inviteSent, st.inviteFails = false, sent, fails
			if last != nil {
				st.inviteErr = a.failed(last)
			}
			if fails == 0 {
				a.aceNext()
			}
		}
	})
}

// aceUpdateDue tells whether this device has yet to meet Ace, for an
// account that was onboarded before: the dialog after an update.
func (a *App) aceUpdateDue() bool {
	if a.Ace == nil || a.Ace.Introduced() || a.screen != screenShell || a.user.ID == id.Nil {
		return false
	}
	if a.isAdmin() || a.onb.off || (a.onb.err != "" && !a.onb.loaded) {
		return true
	}
	return a.onb.loaded && !a.onb.welcome && !a.onb.tour.open
}

// aceUpdateDialog is "Yang baru" with Ace's settings, once: the version's
// notes when it just updated, then "Ace sekarang bisa diatur".
func (a *App) aceUpdateDialog(c *ui.Context) {
	if !a.aceUpdateDue() {
		return
	}
	t := c.Theme()
	w, h := c.Size()
	st := &a.onb.ace
	if !st.updateSaid {
		st.updateSaid = true
		a.Ace.Say(aceUpdateLine.text, aceUpdateLine.face)
	}
	wn := a.WhatsNew
	open := true
	title := "Ace sekarang bisa diatur"
	if wn != nil {
		title = "Yang baru di " + wn.Version
	}
	ui.DialogBase(c.Key("ace-update"), &open, func(backdrop, panel ui.Element) {
		dialogMotion(backdrop, panel)
		backdrop.Background(ui.RGBA(0, 0, 0, .45)).Justify(ui.Start).Padding(min(float32(110), h/6), 0, 0, 0)
		panel.Width(min(float32(520), w-32)).MaxHeight(h-140).Radius(16).Column().Background(t.Surface).Border(1, t.Border).Label(title).Children(func() {
			ui.Scroll(c.Key("ace-update-scroll")).Grow(1).MinHeight(0).FillWidth().Children(func() {
				ui.Column(c).FillWidth().Padding(20, 20, 12, 20).Gap(12).Children(func() {
					ui.Text(c, title).FontSize(18).Bold()
					if wn != nil {
						ui.Column(c).FillWidth().Gap(8).Children(func() {
							for i, n := range wn.Notes {
								ui.Row(c.Key(i)).FillWidth().Gap(8).AlignItems(ui.Start).Children(func() {
									ui.Text(c, "•").FontSize(14).TextColor(t.Accent).Shrink(0)
									ui.Text(c, n).FontSize(14).Grow(1).MinWidth(0)
								})
							}
						})
					}
					ui.Column(c.Key("ace-card")).FillWidth().Gap(10).Padding(14).Radius(12).Background(spaceMix(t.Accent, t.Surface, .9)).Border(1, spaceMix(t.Accent, t.Surface, .6)).Children(func() {
						ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
							aceEyes(c, AceFaceHappy, 1)
							ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
								ui.Text(c, "Ace sekarang bisa diatur").FontSize(14).FontWeight(600)
								muted(c, "Posisi, gaya, dan jam tenang. Semuanya langsung dipakai.").FontSize(12)
							})
						})
						sel := -1
						switch an := a.Ace.Anchor(); {
						case an <= .02:
							sel = 0
						case an >= .98:
							sel = 2
						case an > .48 && an < .52:
							sel = 1
						}
						if i := segmented(c, "Posisi Ace", []string{"Kiri", "Tengah", "Kanan"}, sel); i >= 0 && i != sel {
							a.Ace.SetAnchor([]float64{0, .5, 1}[i])
						}
						settingsApart(func() { a.Ace.Look(c) })
						on, from, to := a.Ace.QuietHours()
						if !on {
							from, to = aceQuietFrom, aceQuietTo
						}
						quiet := on
						if aceSwitchRow(c, "ace-quiet", "Jam tenang", fmt.Sprintf("%02d.00–%02d.00 · Ace diam, kabar menunggu.", from, to), &quiet) {
							a.Ace.SetQuietHours(quiet, from, to)
						}
					})
				})
			})
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Padding(12, 20, 16, 20).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
				if onbLink(c, "Riwayat perubahan", "Riwayat perubahan").Clicked() {
					open = false
					a.chrome.settingsTab = "Tentang"
					a.navigate(PathPengaturan)
				}
				ui.Box(c).Grow(1)
				if onbSecondary(c, "Nanti", "Nanti", false).Clicked() {
					open = false
				}
				if onbPrimary(c, "Selesai", "Selesai", false).Clicked() {
					open = false
				}
			})
		})
	})
	if !open {
		a.Ace.Hush()
		a.Ace.SetIntroduced()
		st.updateSaid = false
		if wn != nil {
			a.WhatsNew = nil
			if a.SeenWhatsNew != nil {
				a.SeenWhatsNew(wn.Version)
			}
		}
	}
}
