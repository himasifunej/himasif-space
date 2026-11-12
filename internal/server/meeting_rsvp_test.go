package server

import (
	"strings"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Kehadiranmu (canvas 3747dbbac0d0 desain 5): a participant answers for
// themself, the organizer sees the answer and its reason, attendance stays
// the organizer's, and someone not invited cannot answer.
func TestMeetingRSVP(t *testing.T) {
	o := newTaskOrg(t)
	sid := o.spaces["division-mediatek"]
	var m api.Meeting
	req := api.MeetingRequest{Title: "Gladi LPJ", StartsAt: time.Now().Add(3 * time.Hour).Truncate(time.Minute), TimeZone: "Asia/Jakarta", Participants: []id.ID{o.memberID}, ReminderMinutes: []int{}}
	if c := o.call("POST", "/api/spaces/"+sid.String()+"/meetings", o.kadiv, req, &m); c != 201 {
		t.Fatalf("create = %d", c)
	}
	path := "/api/meetings/" + m.ID.String() + "/rsvp"
	var got api.Meeting
	if c := o.call("PUT", path, o.member, api.RSVPRequest{Status: api.RSVPNo, Note: "  Ada kuliah pengganti  "}, &got); c != 200 {
		t.Fatalf("rsvp = %d", c)
	}
	if p := got.Participants[0]; p.RSVP != api.RSVPNo || p.RSVPNote != "Ada kuliah pengganti" || p.AttendanceStatus != "unrecorded" {
		t.Fatalf("participant %+v", p)
	}
	if got.Version != m.Version {
		t.Fatal("an answer changed the meeting's version")
	}
	var seen api.Meeting
	o.call("GET", "/api/meetings/"+m.ID.String(), o.kadiv, nil, &seen)
	if seen.Participants[0].RSVPNote != "Ada kuliah pengganti" {
		t.Fatalf("organizer sees %+v", seen.Participants)
	}
	// Hadir drops the reason; "" withdraws.
	got = api.Meeting{}
	o.call("PUT", path, o.member, api.RSVPRequest{Status: api.RSVPYes, Note: "abaikan"}, &got)
	if p := got.Participants[0]; p.RSVP != api.RSVPYes || p.RSVPNote != "" {
		t.Fatalf("yes %+v", p)
	}
	o.wantStatus(400, "PUT", path, o.member, api.RSVPRequest{Status: "maybe"})
	o.wantStatus(400, "PUT", path, o.member, api.RSVPRequest{Status: api.RSVPLate, Note: strings.Repeat("x", 301)})
	// The organizer is not a participant here; the peer cannot read it.
	o.wantStatus(403, "PUT", path, o.kadiv, api.RSVPRequest{Status: api.RSVPYes})
	if c := o.call("PUT", path, o.peer, api.RSVPRequest{Status: api.RSVPYes}, nil); c != 404 && c != 403 {
		t.Fatalf("peer = %d", c)
	}
	o.wantStatus(403, "PUT", path, o.admin, api.RSVPRequest{Status: api.RSVPYes})
}
