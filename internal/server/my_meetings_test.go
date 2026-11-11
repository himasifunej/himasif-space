package server

import (
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// /my-meetings lists the meetings of the period the caller attends, in
// the window asked, across Spaces; not others', not cancelled ones.
func TestMyMeetingsAcrossSpaces(t *testing.T) {
	o := newTaskOrg(t)
	sid := o.spaces["division-mediatek"]
	create := func(title string, at time.Time, people []id.ID) api.Meeting {
		t.Helper()
		var m api.Meeting
		req := api.MeetingRequest{Title: title, StartsAt: at, TimeZone: "Asia/Jakarta", Participants: people, ReminderMinutes: []int{}}
		if c := o.call("POST", "/api/spaces/"+sid.String()+"/meetings", o.kadiv, req, &m); c != 201 {
			t.Fatalf("create %s = %d", title, c)
		}
		return m
	}
	soon := time.Now().Add(3 * time.Hour).Truncate(time.Minute)
	mine := create("Rapat divisi", soon, []id.ID{o.memberID})
	create("Rapat inti tanpa anggota", soon, nil)
	far := create("Rapat bulan depan", time.Now().Add(20*24*time.Hour), []id.ID{o.memberID})
	gone := create("Rapat batal", soon, []id.ID{o.memberID})
	req := api.MeetingRequest{Title: gone.Title, StartsAt: gone.StartsAt, TimeZone: gone.TimeZone, Participants: []id.ID{o.memberID}, ReminderMinutes: []int{}, Cancelled: true, ExpectedVersion: gone.Version}
	if c := o.call("PUT", "/api/meetings/"+gone.ID.String(), o.kadiv, req, nil); c != 200 {
		t.Fatalf("cancel = %d", c)
	}
	var out api.List[api.Meeting]
	path := "/api/periods/" + o.period.ID.String() + "/my-meetings"
	if c := o.call("GET", path, o.member, nil, &out); c != 200 {
		t.Fatalf("my meetings = %d", c)
	}
	if len(out.Items) != 1 || out.Items[0].ID != mine.ID || len(out.Items[0].Participants) != 1 {
		t.Fatalf("member's meetings %+v", out.Items)
	}
	// A wider window brings next month's.
	q := "?from=" + time.Now().UTC().Format(time.RFC3339) + "&to=" + time.Now().Add(30*24*time.Hour).UTC().Format(time.RFC3339)
	out = api.List[api.Meeting]{}
	if c := o.call("GET", path+q, o.member, nil, &out); c != 200 || len(out.Items) != 2 || out.Items[1].ID != far.ID {
		t.Fatalf("wide window = %d %+v", c, out.Items)
	}
	// The organizer sees what they organize.
	out = api.List[api.Meeting]{}
	if c := o.call("GET", path, o.kadiv, nil, &out); c != 200 || len(out.Items) != 2 {
		t.Fatalf("organizer's = %d %d", c, len(out.Items))
	}
	if c := o.call("GET", path+"?from=x", o.member, nil, nil); c != 400 {
		t.Fatalf("bad from = %d", c)
	}
	if c := o.call("GET", path+"?from=2026-01-01T00:00:00Z&to=2026-06-01T00:00:00Z", o.member, nil, nil); c != 400 {
		t.Fatalf("too wide = %d", c)
	}
}
