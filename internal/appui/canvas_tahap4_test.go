package appui

import (
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

func TestAgendaMonthDaysAreWholeWeeksFromMonday(t *testing.T) {
	days := agendaMonthDays(2026, time.November)
	// November 2026 starts on a Sunday and ends on a Monday.
	if len(days) != 42 || days[0] != (timeutil.Date{Year: 2026, Month: time.October, Day: 26}) || days[len(days)-1] != (timeutil.Date{Year: 2026, Month: time.December, Day: 6}) {
		t.Fatalf("grid = %v … %v (%d days)", days[0], days[len(days)-1], len(days))
	}
	if got := agendaWeekStart(timeutil.Date{Year: 2026, Month: time.November, Day: 8}, 1); got != (timeutil.Date{Year: 2026, Month: time.November, Day: 9}) {
		t.Fatalf("next week starts %v", got)
	}
}

func TestAgendaEventsJoinDeadlinesPostingAndMeetings(t *testing.T) {
	now := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	at := time.Date(2026, 11, 7, 19, 0, 0, 0, time.UTC)
	d5 := timeutil.Date{Year: 2026, Month: time.November, Day: 5}
	d9 := timeutil.Date{Year: 2026, Month: time.November, Day: 9}
	tasks := []api.Task{
		{ID: id.New(), Title: "Poster", Progress: api.TaskTodo, TimeZone: "UTC", DeadlineDate: &d5, PostingDate: &d9},
		{ID: id.New(), Title: "Batal", Progress: api.TaskCancelled, TimeZone: "UTC", DeadlineDate: &d5},
	}
	meetings := []api.Meeting{{ID: id.New(), Title: "Rapat koordinasi", StartsAt: at}, {ID: id.New(), Title: "Dibatalkan", StartsAt: at, Cancelled: true}}
	ev := agendaEvents(tasks, meetings, time.UTC, now)
	if len(ev) != 3 {
		t.Fatalf("events = %+v", ev)
	}
	if ev[0].kind != agendaDeadline || !ev[0].late || ev[1].kind != agendaMeeting || ev[1].meeting == id.Nil || ev[2].kind != agendaPosting || ev[2].day != d9 {
		t.Fatalf("order or kinds wrong: %+v", ev)
	}
}

func TestMemberRoleBadgeAndLoad(t *testing.T) {
	for _, tc := range []struct {
		m    api.Member
		want string
	}{
		{api.Member{Role: api.RoleKadiv}, "Kadiv"},
		{api.Member{Role: api.RoleMember, Roles: []string{api.RoleMember, api.Role45}}, "45"},
		{api.Member{Role: api.RoleCompanion}, "Pendamping"},
		{api.Member{Role: api.RoleMember}, "Anggota"},
	} {
		if got := memberRoleBadge(tc.m); got != tc.want {
			t.Errorf("badge(%v) = %q; want %q", tc.m.Roles, got, tc.want)
		}
	}
	a := &App{}
	uid := id.New()
	a.tasks.loaded = true
	a.tasks.items = []api.Task{
		{Assignee: api.PersonRef{ID: uid}, Progress: api.TaskTodo},
		{Assignee: api.PersonRef{ID: uid}, Progress: api.TaskDoing},
		{Assignee: api.PersonRef{ID: uid}, Progress: api.TaskDone},
		{Assignee: api.PersonRef{ID: id.New()}, Progress: api.TaskTodo},
	}
	if n, ok := a.memberLoad(uid); !ok || n != 2 {
		t.Fatalf("load = %d %v; want 2", n, ok)
	}
}

// The Anggota tab is a table with Ping and Ubah role per row; the old
// per-member actions open from Ubah role for a kadiv.
func TestMembersTableOpensOldActionsFromUbahRole(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.SetSize(1440, 900)
	d := h.api.divisions[0]
	testAuthorizedSpace(h.app, d)
	caps := []string{api.CapSpaceRead, api.CapMembersRead, api.CapMembersManage, api.CapInvitationsManage, api.Cap45Manage}
	roles := []string{api.RoleKadiv}
	h.app.access.spaces[0].Roles = []api.Access{{DivisionID: d.ID, Roles: roles, Capabilities: caps}}
	h.app.access.members = memberManagementState{division: d.ID, loaded: true, access: api.Access{DivisionID: d.ID, Roles: roles, Capabilities: caps}, members: []api.Member{{UserID: id.New(), DisplayName: "Aruna Desain", Role: api.RoleMember, Groups: []string{api.GroupDesain}}}, invitations: []api.Invitation{{ID: id.New(), Email: "rina@student.unej.ac.id", Status: api.InvitationPending, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(7 * 24 * time.Hour)}}}
	h.app.router.Push(PathSpaces)
	h.app.chrome.tab = "Anggota"
	h.tt.Frame()
	for _, s := range []string{"Anggota dan role", "Aruna Desain", "Undangan tertunda", "rina@student.unej.ac.id", "Kewenangan role", "Undang anggota"} {
		h.want(s)
	}
	h.dontWant("Hapus anggota Aruna Desain")
	h.click("Ubah role Aruna Desain")
	h.tt.Frame()
	h.want("Hapus anggota Aruna Desain")
	h.want("Tunjuk 45 Aruna Desain")
}

func TestArchiveShowsHandoverOnlyForAdmin(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.SetSize(1440, 900)
	h.app.router.Push(PathArsip)
	h.tt.Frame()
	h.want("Arsip periode")
	h.dontWant("Siapkan serah terima")
}
