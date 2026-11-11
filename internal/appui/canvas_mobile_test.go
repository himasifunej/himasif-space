package appui

import (
	"fmt"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// On a phone the month grid's cells are too small for strips and
// "+n lainnya" (it was clipped at the cell's bottom): a busy day shows dots,
// and every item stays reachable in the list below the grid.
func TestPhoneAgendaMonthCellShowsDotsNotOverflowText(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.SetSize(390, 844)
	testAuthorizedSpace(h.app, h.api.divisions[0])
	h.app.router.Push(PathSpaces)
	h.app.chrome.tab = "Agenda"
	h.tt.Frame()
	loc := h.app.agendaZone()
	today := timeutil.DateOf(time.Now().In(loc))
	var items []api.Task
	for i := range 5 {
		d := today
		items = append(items, api.Task{ID: id.New(), Title: fmt.Sprintf("Tugas padat %d", i+1), Progress: api.TaskTodo, TimeZone: loc.String(), DeadlineDate: &d})
	}
	h.app.tasks.items, h.app.tasks.loaded = items, true
	h.app.meetings.loaded = true
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	h.dontWant("+2 lainnya")
	h.want("Tugas padat 5")
	captureNativeUI(t, h, "phone-agenda-month")
}

// The custom roles card used text buttons "Edit role <nama>" that ran past
// a phone's right edge; they are icon buttons now and stay on screen, in
// the Role kustom dialog (pilot 0.4.4).
func TestPhoneCustomRoleActionsStayOnScreen(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.SetSize(390, 844)
	d := h.api.divisions[0]
	testAuthorizedSpace(h.app, d)
	caps := []string{api.CapSpaceRead, api.CapMembersRead, api.CapMembersManage, api.CapRolesManage}
	roles := []string{api.RoleKadiv}
	h.app.access.spaces[0].Roles = []api.Access{{DivisionID: d.ID, Roles: roles, Capabilities: caps}}
	name := "Demo — pengelola tugas"
	h.app.access.members = memberManagementState{division: d.ID, loaded: true, access: api.Access{DivisionID: d.ID, Roles: roles, Capabilities: caps}, roles: []api.CustomRole{{ID: id.New(), DivisionID: d.ID, Name: name, Capabilities: []string{api.CapTasksManage}}}}
	h.app.router.Push(PathSpaces)
	h.app.chrome.tab = "Anggota"
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	h.click("Role kustom")
	h.tt.Frame()
	for _, label := range []string{"Edit role " + name, "Hapus role " + name} {
		h.tt.Scroll(200, 500, 0, 0)
		r, ok := h.tt.Find(label)
		for tries := 0; ok && (r.Y < 60 || r.Y+r.H > 760) && tries < 20; tries++ {
			h.tt.Scroll(200, 500, 0, 200)
			h.tt.Advance(200 * time.Millisecond)
			h.tt.Frame()
			r, ok = h.tt.Find(label)
		}
		if !ok || r.X < 0 || r.X+r.W > 391 {
			t.Fatalf("%s missing or past the phone's edge: %+v exists=%v", label, r, ok)
		}
	}
	// The old row squeezed the name into three lines beside a wide button.
	if r, ok := h.tt.Find(name); !ok || r.H > 30 {
		t.Fatalf("role name squeezed: %+v exists=%v", r, ok)
	}
	captureNativeUI(t, h, "phone-members-roles")
}
