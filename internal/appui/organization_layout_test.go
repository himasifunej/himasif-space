package appui

import (
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func assertAdministrationVisible(t *testing.T, h *harness, label string, width, height int) {
	t.Helper()
	r, ok := h.tt.Find(label)
	if !ok || r.W <= 0 || r.H <= 0 || r.X < 0 || r.Y < 0 || r.X+r.W > float32(width)+1 || r.Y+r.H > float32(height-28)+1 {
		t.Fatalf("%dx%d: %s clipped: %+v exists=%v", width, height, label, r, ok)
	}
}

func TestOrganizationFormsAndFooterRemainReachable(t *testing.T) {
	for _, size := range [][2]int{{1440, 900}, {880, 640}} {
		h, _ := organizationHarness(t)
		h.tt.SetSize(size[0], size[1])
		h.app.router.Replace(PathOrganisasi)
		h.app.beginPeriod()
		h.tt.Advance(400 * time.Millisecond)
		h.tt.Frame()
		h.tt.Scroll(float32(size[0]-35), float32(size[1]-130), 0, 10000)
		h.tt.Advance(time.Second)
		h.tt.Frame()
		assertAdministrationVisible(t, h, "Simpan periode baru", size[0], size[1])
		// The footer is below all scrollable administration content.
		if r, ok := h.tt.Find("Tersambung"); !ok || r.Y+r.H > float32(size[1])+1 {
			t.Fatal("administration moved the connection footer outside the window")
		}
		captureNativeUI(t, h, "organization-period-"+map[int]string{1440: "large", 880: "minimum"}[size[0]])
		h.app.organization.create = nil
		h.app.organization.tab = "Serah terima"
		h.app.organization.handoverDraft.remaining = "Tidak ada pekerjaan tersisa."
		h.tt.Frame()
		h.tt.Scroll(float32(size[0]-35), float32(size[1]-130), 0, 10000)
		h.tt.Advance(time.Second)
		h.tt.Frame()
		assertAdministrationVisible(t, h, "Simpan checklist serah terima", size[0], size[1])
		captureNativeUI(t, h, "organization-handover-"+map[int]string{1440: "large", 880: "minimum"}[size[0]])
	}
}

func TestOrganizationReviewRevealsTheFrozenKadivConfirmation(t *testing.T) {
	h, f := organizationHarness(t)
	h.tt.SetSize(880, 640)
	h.app.router.Replace(PathOrganisasi)
	h.app.organization.tab = "Kadiv"
	h.app.organization.kadivUser = f.users[1].ID
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	h.tt.Scroll(840, 510, 0, 10000)
	h.tt.Advance(time.Second)
	h.tt.Frame()
	assertAdministrationVisible(t, h, "Tinjau penetapan kadiv", 880, 640)
	h.click("Tinjau penetapan kadiv")
	h.tt.Advance(time.Second)
	h.tt.Frame()
	assertAdministrationVisible(t, h, "Konfirmasi kadiv", 880, 640)
	if len(f.kadivCalls) != 0 || h.app.organization.confirm == nil || h.app.organization.confirm.user != f.users[1].ID {
		t.Fatal("review either dispatched a mutation or changed the confirmed account")
	}
	captureNativeUI(t, h, "organization-kadiv-confirmation")
}

func TestArchiveGrantAndRevokeConfirmationRemainReachable(t *testing.T) {
	h, f, d := archiveHarness(t)
	h.tt.SetSize(880, 640)
	h.app.router.Replace(PathArchiveAccess)
	h.app.archives.user, h.app.archives.space = f.members[d][0].UserID, f.targets[d][0].SpaceID
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	h.tt.Scroll(840, 510, 0, 10000)
	h.tt.Advance(time.Second)
	h.tt.Frame()
	assertAdministrationVisible(t, h, "Berikan izin baca arsip", 880, 640)
	h.click("Berikan izin baca arsip")
	settleArchiveAdmin(t, h)
	h.tt.Frame()
	h.tt.Scroll(840, 510, 0, 10000)
	h.tt.Advance(time.Second)
	h.tt.Frame()
	assertAdministrationVisible(t, h, "Cabut izin arsip", 880, 640)
	h.click("Cabut izin arsip")
	h.tt.Advance(time.Second)
	h.tt.Frame()
	assertAdministrationVisible(t, h, "Konfirmasi cabut izin arsip", 880, 640)
	if h.app.archives.confirmGrant == id.Nil || len(f.intents) != 1 {
		t.Fatal("revoke happened before its confirmation")
	}
	captureNativeUI(t, h, "archive-confirmation-minimum")
	h.click("Konfirmasi cabut izin arsip")
	settleArchiveAdmin(t, h)
	if len(h.app.archives.grants) != 0 || len(f.intents) != 2 {
		t.Fatal("native archive confirmation did not revoke the acknowledged grant")
	}
	// Recipient privacy is covered by the server regression; the page offers
	// access management only to the exact original-kadiv/admin scope.
	h.app.access.spaces[0].Roles[0].Roles = []string{api.Role45}
	h.tt.Frame()
	if _, ok := h.tt.Find("Berikan izin baca arsip"); ok {
		t.Fatal("45 retained the archive write form")
	}
}
