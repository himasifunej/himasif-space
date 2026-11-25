package appui

import (
	"context"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/id"
)

// sopMenuState says whether the SOP menus show (PRD EVA-11, owner's
// decision of 25 Nov 2026): Litbang has not joined the pilot, so Evaluasi
// SOP stays hidden until a division of the period has an active SOP policy,
// but it stays ready: whoever may configure a policy (the kadiv of Humas or
// Mediatek; the server decides) reaches it from Pengaturan → Evaluasi SOP,
// and turning a policy on brings the menus back without a new release. The server never runs the H-4 follow-up or the
// weekly SOP recap without an active policy either.
type sopMenuState struct {
	seq             uint64
	user, period    id.ID
	loading, loaded bool
	// enabled is, per division the caller may read SOP of, whether its
	// policy is on.
	enabled map[id.ID]bool
	// configure tells whether the caller may configure a policy somewhere.
	configure bool
}

// ensureSOPMenu loads, once per account and period, the SOP scopes and
// their policies: two small requests at most (Humas and Mediatek). A
// failure leaves the menus hidden until the period or account changes.
func (a *App) ensureSOPMenu() {
	if a.period() == nil {
		return
	}
	st := &a.sopMenu
	if st.user != a.user.ID || st.period != a.periodID {
		*st = sopMenuState{seq: st.seq + 1, user: a.user.ID, period: a.periodID}
	}
	if st.loaded || st.loading {
		return
	}
	cl := a.notificationClient()
	if cl == nil {
		return
	}
	st.loading = true
	gen, seq, pid := a.gen, st.seq, st.period
	a.async(func() func() {
		ctx := context.Background()
		scopes, err := cl.SOPScopes(ctx, pid)
		enabled := map[id.ID]bool{}
		configure := false
		for _, s := range scopes {
			configure = configure || s.CanConfigure
			if err != nil {
				break
			}
			policy, perr := cl.SOPPolicy(ctx, pid, s.DivisionID)
			enabled[s.DivisionID] = perr == nil && policy.Enabled
		}
		return func() {
			st := &a.sopMenu
			if gen != a.gen || seq != st.seq {
				return
			}
			st.loading, st.loaded = false, true
			if err != nil {
				return
			}
			st.enabled, st.configure = enabled, configure
		}
	})
}

// noteSOPPolicy records a division's policy as Evaluasi SOP just loaded or
// saved it, so turning a policy on or off shows or hides the menus at once.
func (a *App) noteSOPPolicy(division id.ID, enabled bool) {
	st := &a.sopMenu
	if st.user != a.user.ID || st.period != a.periodID || division == id.Nil {
		return
	}
	if st.enabled == nil {
		st.enabled = map[id.ID]bool{}
	}
	st.enabled[division] = enabled
}

// sopMenuVisible tells whether Evaluasi SOP shows in the sidebar, Lainnya,
// quick navigation and Medhum: only while a division of the period has an
// active SOP policy the caller may read.
func (a *App) sopMenuVisible() bool {
	a.ensureSOPMenu()
	for _, on := range a.sopMenu.enabled {
		if on {
			return true
		}
	}
	return false
}

// sopConfigurable tells whether Pengaturan lists Evaluasi SOP: the caller
// may configure the SOP policy of a division (SOPAccess.CanConfigure, today
// the kadiv of Humas or Mediatek).
func (a *App) sopConfigurable() bool {
	a.ensureSOPMenu()
	return a.sopMenu.configure
}

// sopSettings is Pengaturan → Evaluasi SOP: where the SOP menus are while
// they are hidden. The policy itself is edited on the Evaluasi SOP page.
func (a *App) sopSettings(c *ui.Context) {
	settingsBody(c, "Evaluasi SOP", "Evaluasi SOP Medhum belum dipakai: Litbang belum ikut pilot. Menunya disembunyikan sampai ada divisi yang menyalakan kebijakan SOP, tetapi tetap siap dinyalakan tanpa pembaruan aplikasi.", func() {
		on := 0
		for _, enabled := range a.sopMenu.enabled {
			if enabled {
				on++
			}
		}
		if on == 0 {
			pageNotice(c, "info", "Belum aktif", "Tidak ada kebijakan SOP yang aktif di periode ini. Follow-up H-4 dan rekap mingguan SOP tidak dikirim.")
		} else {
			pageNotice(c, "check", "Aktif", "Kebijakan SOP aktif di divisi yang kamu kelola; menu Evaluasi SOP tampil di sidebar.")
		}
		muted(c, "Untuk menyalakan: buka Evaluasi SOP, pilih divisi, isi versi dokumen SOP dan aturan yang dinilai, nyalakan Otomatisasi aktif, lalu simpan kebijakan dengan alasannya.")
		if spaceButton(c, "Buka Evaluasi SOP", "shield", false).Clicked() {
			a.navigate(PathEvaluasi)
		}
	})
}
