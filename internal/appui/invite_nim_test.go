package appui

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// The invite field: a NIM becomes the student address shown before sending;
// another email is allowed with a warning; anything else is not sent.
func TestInviteTarget(t *testing.T) {
	got := inviteTarget(" 232410101090 ")
	if got.req == nil || got.req.NIM != "232410101090" || got.req.Email != "" || got.email != "232410101090@mail.unej.ac.id" || got.warning != "" {
		t.Fatalf("NIM %+v", got)
	}
	got = inviteTarget("232410101090@mail.unej.ac.id")
	if got.req == nil || got.req.Email != "232410101090@mail.unej.ac.id" || got.warning != "" {
		t.Fatalf("student email %+v", got)
	}
	got = inviteTarget("Dosen@Unej.ac.id")
	if got.req == nil || got.req.Email != "dosen@unej.ac.id" || !strings.Contains(got.warning, "Bukan email mahasiswa") {
		t.Fatalf("other email %+v", got)
	}
	for _, bad := range []string{"2324", "23241010109O", "nama@", "nama@unej"} {
		if got := inviteTarget(bad); got.req != nil || got.problem == "" {
			t.Errorf("%q = %+v", bad, got)
		}
	}
	if got := inviteTarget(""); got.req != nil || got.problem != "" {
		t.Fatalf("empty %+v", got)
	}
}

// Two native screenshots for the handoff (SPACE_UI_CAPTURE_DIR): the
// members page with NIMs, the invite preview and a bounced invitation; and
// the member's own WhatsApp panel while a code waits to be sent.
func TestNIMAndWALinkNativeCapture(t *testing.T) {
	if os.Getenv("SPACE_UI_CAPTURE_DIR") == "" {
		t.Skip("set SPACE_UI_CAPTURE_DIR for native screenshots")
	}
	h := newChromeHarness(t, nil)
	h.app.chrome.preferences.Theme = "light"
	h.tt.SetSize(1440, 1180)
	d := h.api.divisions[0]
	sp := testAuthorizedSpace(h.app, d)
	caps := []string{api.CapSpaceRead, api.CapMembersRead, api.CapMembersManage, api.CapInvitationsManage, api.CapTasksOwn}
	roles := []string{api.RoleKadiv}
	h.app.access.spaces[0].Roles = []api.Access{{DivisionID: d.ID, Roles: roles, Capabilities: caps}}
	yes, no := true, false
	members := []api.Member{
		{UserID: h.app.user.ID, DisplayName: "Demo Mediatek", Role: api.RoleKadiv, WhatsAppLinked: &yes},
		{UserID: id.New(), DisplayName: "Aruna Desain", Role: api.RoleMember, Groups: []string{api.GroupDesain}, WhatsAppLinked: &yes, NIM: "232410101090"},
		{UserID: id.New(), DisplayName: "Nara Tekno", Role: api.RoleMember, Groups: []string{api.GroupTekno}, WhatsAppLinked: &no, NIM: "232410101112"},
		{UserID: id.New(), DisplayName: "Raka Video", Role: api.RoleMember, Groups: []string{api.GroupVideo}, WhatsAppLinked: &yes, NIM: "222410101045"},
	}
	bounced := time.Now().Add(-time.Hour)
	h.app.access.members = memberManagementState{division: d.ID, loaded: true, access: api.Access{DivisionID: d.ID, Roles: roles, Capabilities: caps}, members: members,
		inviteOpen: true, email: "232410101131",
		invitations: []api.Invitation{
			{ID: id.New(), Email: "232410101127@mail.unej.ac.id", NIM: "232410101127", Status: api.InvitationPending, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(7 * 24 * time.Hour)},
			{ID: id.New(), Email: "232410101921@mail.unej.ac.id", NIM: "232410101921", Status: api.InvitationPending, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
				BouncedAt: &bounced, BounceReason: "Email tidak sampai: alamat tidak ada atau menolak email."},
		}}
	h.app.tasks = tasksUIState{scope: sp.ID, loaded: true}
	h.app.router.Push(PathSpaces)
	h.app.chrome.tab = "Anggota"
	shot := func(name string) {
		h.tt.Frame()
		h.tt.Advance(400 * time.Millisecond)
		h.tt.Frame()
		captureNativeUI(t, h, name)
	}
	shot("anggota-nim-undang")

	h.tt.SetSize(1280, 820)
	h.app.chrome.tab = "Bot WhatsApp"
	h.tt.Frame()
	st := &h.app.whatsapp
	st.loaded, st.loading, st.division = true, false, d.ID
	st.linkStatus, st.linkStatusFor = &api.WALinkStatus{BotReady: true}, d.ID
	st.link = &api.WALinkChallenge{Code: "SPACE-7mQ2vXnL0aRk4pTz9sYe1bWc", ExpiresAt: time.Now().Add(15 * time.Minute), DeepLink: "https://wa.me/6281111111111?text=SPACE-7mQ2vXnL0aRk4pTz9sYe1bWc"}
	st.linkPollNext = time.Now().Add(time.Hour)
	shot("wa-tautkan-menunggu")
}
