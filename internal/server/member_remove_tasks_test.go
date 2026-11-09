package server

import (
	"context"
	"net/http"
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Stage A3: a member who is still the PJ of an active task in any Space of
// the division (its own Space or a shared one, as Medhum) cannot be
// removed. The server says how many tasks and where, both before the
// removal (active-tasks) and in the refusal.
func TestRemoveMemberBlockedByActiveTasks(t *testing.T) {
	o := newTaskOrg(t)
	ctx := context.Background()
	scope := o.scope(o.period.ID, "mediatek")
	member := func(uid id.ID) string { return scope + "/members/" + uid.String() }
	activeTasks := func(uid id.ID) api.MemberActiveTasks {
		t.Helper()
		var out api.MemberActiveTasks
		if c := o.call("GET", member(uid)+"/active-tasks", o.kadiv, nil, &out); c != http.StatusOK {
			t.Fatalf("active-tasks = %d", c)
		}
		return out
	}
	refusal := func(uid id.ID) api.MemberActiveTasks {
		t.Helper()
		var er struct {
			Error struct {
				Code    string                `json:"code"`
				Message string                `json:"message"`
				Data    api.MemberActiveTasks `json:"data"`
			} `json:"error"`
		}
		if c := o.call("DELETE", member(uid), o.kadiv, nil, &er); c != http.StatusConflict || er.Error.Code != api.CodeMemberHasTasks {
			t.Fatalf("remove = %d %+v", c, er)
		}
		if er.Error.Message == "" {
			t.Fatal("refusal without a message")
		}
		t.Logf("refusal: %s", er.Error.Message)
		return er.Error.Data
	}

	// No tasks at all: the active-tasks answer is empty and removal works.
	freeTok, free := o.join(o.kadiv, o.period.ID, "mediatek", "bebas@himasif.test", "Anggota Bebas")
	_ = freeTok
	if got := activeTasks(free.User.ID); got.Count != 0 || len(got.Spaces) != 0 {
		t.Fatalf("free member's tasks %+v", got)
	}
	o.wantStatus(http.StatusNoContent, "DELETE", member(free.User.ID), o.kadiv, nil)

	// An active task in the division's own Space.
	own := o.create(false) // PJ: o.memberID, in division-mediatek
	got := activeTasks(o.memberID)
	if got.Count != 1 || len(got.Spaces) != 1 || got.Spaces[0].SpaceID != o.spaces["division-mediatek"] || got.Spaces[0].Count != 1 || got.Spaces[0].Name == "" {
		t.Fatalf("own space tasks %+v", got)
	}
	if data := refusal(o.memberID); data.Count != 1 {
		t.Fatalf("refusal data %+v", data)
	}

	// Only done and cancelled tasks left there, but one active in Medhum,
	// a Space the division shares: still refused, and Medhum is named.
	if _, err := o.st.DB().Exec(ctx, `update tasks set progress='done' where id=$1`, own.ID); err != nil {
		t.Fatal(err)
	}
	cancelled := o.create(false)
	if _, err := o.st.DB().Exec(ctx, `update tasks set progress='cancelled' where id=$1`, cancelled.ID); err != nil {
		t.Fatal(err)
	}
	shared := o.create(true) // in medhum
	// It is one task; Medhum is among its Spaces (a task made in Medhum
	// also shows on its owner's board).
	inMedhum := func(d api.MemberActiveTasks) bool {
		for _, sp := range d.Spaces {
			if sp.SpaceID == o.spaces["medhum"] && sp.Count == 1 && sp.Name == "Medhum" {
				return true
			}
		}
		return false
	}
	got = activeTasks(o.memberID)
	if got.Count != 1 || !inMedhum(got) {
		t.Fatalf("medhum tasks %+v", got)
	}
	if data := refusal(o.memberID); data.Count != 1 || !inMedhum(data) {
		t.Fatalf("refusal data %+v", data)
	}

	// The gap of a board-only count: a task that is only in Medhum, not on
	// Mediatek's own board, still keeps the member.
	if _, err := o.st.DB().Exec(ctx, `delete from task_spaces where task_id=$1 and space_id=$2`, shared.ID, o.spaces["division-mediatek"]); err != nil {
		t.Fatal(err)
	}
	got = activeTasks(o.memberID)
	if got.Count != 1 || len(got.Spaces) != 1 || got.Spaces[0].SpaceID != o.spaces["medhum"] {
		t.Fatalf("medhum-only tasks %+v", got)
	}
	refusal(o.memberID)

	// A submitted task (waiting for review) is still active.
	if _, err := o.st.DB().Exec(ctx, `update tasks set progress='submitted' where id=$1`, shared.ID); err != nil {
		t.Fatal(err)
	}
	refusal(o.memberID)

	// Once the Medhum task is done too, the member can be removed.
	if _, err := o.st.DB().Exec(ctx, `update tasks set progress='done' where id=$1`, shared.ID); err != nil {
		t.Fatal(err)
	}
	if got := activeTasks(o.memberID); got.Count != 0 {
		t.Fatalf("after done %+v", got)
	}
	o.wantStatus(http.StatusNoContent, "DELETE", member(o.memberID), o.kadiv, nil)

	// Another division's tasks never count: Humas' PJ is not Mediatek's.
	var hs api.List[api.Member]
	o.call("GET", o.scope(o.period.ID, "humas")+"/members", o.admin, nil, &hs)
	if len(hs.Items) == 0 {
		t.Fatal("no humas member")
	}

	// Only a manager asks.
	if c, _ := o.errCode("GET", member(o.peerID)+"/active-tasks", o.peer, nil); c != http.StatusForbidden {
		t.Fatalf("peer asks = %d", c)
	}
}

func TestMemberTaskPlaces(t *testing.T) {
	a, b := id.New(), id.New()
	for _, tc := range []struct {
		tasks api.MemberActiveTasks
		want  string
	}{
		{api.MemberActiveTasks{Count: 3, Spaces: []api.MemberTaskSpace{{SpaceID: a, Name: "Mediatek", Count: 2}, {SpaceID: b, Name: "Medhum", Count: 1}}}, " (Mediatek 2, Medhum 1)"},
		{api.MemberActiveTasks{Count: 1, Spaces: []api.MemberTaskSpace{{SpaceID: b, Name: "Medhum", Count: 1}, {SpaceID: a, Name: "Mediatek", Count: 1}}}, " di Medhum dan Mediatek"},
		{api.MemberActiveTasks{Count: 2}, ""},
	} {
		if got := tc.tasks.Places(); got != tc.want {
			t.Errorf("%+v: %q, want %q", tc.tasks, got, tc.want)
		}
	}
}
