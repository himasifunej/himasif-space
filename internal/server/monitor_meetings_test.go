package server

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func (o *monitorOrg) meeting(token string, sid id.ID, req api.MeetingRequest) (int, api.Meeting) {
	o.t.Helper()
	var m api.Meeting
	c := o.call("POST", "/api/spaces/"+sid.String()+"/meetings", token, req, &m)
	return c, m
}

// MTG-06: Rapat BPH and Rapat pengurus are held in BPH's space; a Rapat
// pengurus may invite every kadiv; a kind stays when a client leaves it out.
func TestMeetingKinds(t *testing.T) {
	o := newMonitorOrg(t)
	if c := o.call("PUT", o.scope(o.period.ID, "bph")+"/kadiv", o.admin, api.SetKadivRequest{UserID: o.bphID}, nil); c != 200 {
		t.Fatalf("ketum kadiv BPH = %d", c)
	}
	bph := o.spaces["division-bph"]
	at := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Minute)
	req := api.MeetingRequest{Title: "Rapat BPH mingguan", StartsAt: at, Kind: api.MeetingBPH}
	c, m := o.meeting(o.bph, bph, req)
	if c != 201 || m.Kind != api.MeetingBPH {
		t.Fatalf("rapat BPH = %d %+v", c, m)
	}
	// Without a kind, an update keeps it.
	upd := api.MeetingRequest{Title: "Rapat BPH (diubah)", StartsAt: at, ExpectedVersion: m.Version}
	if c := o.call("PUT", "/api/meetings/"+m.ID.String(), o.bph, upd, &m); c != 200 || m.Kind != api.MeetingBPH {
		t.Fatalf("update = %d %+v", c, m)
	}
	// BPH kinds belong to BPH's space; a division meeting defaults to its kind.
	if c, _ := o.meeting(o.kadiv, o.spaces["division-mediatek"], api.MeetingRequest{Title: "x", StartsAt: at, Kind: api.MeetingBPH}); c != 400 {
		t.Fatalf("rapat BPH di Mediatek = %d", c)
	}
	if c, _ := o.meeting(o.bph, bph, api.MeetingRequest{Title: "x", StartsAt: at, Kind: "rahasia"}); c != 400 {
		t.Fatalf("unknown kind = %d", c)
	}
	c, div := o.meeting(o.kadiv, o.spaces["division-mediatek"], api.MeetingRequest{Title: "Rapat divisi", StartsAt: at})
	if c != 201 || div.Kind != api.MeetingDivision {
		t.Fatalf("rapat divisi = %d %+v", c, div)
	}
	// The Mediatek kadiv is not a member of BPH: invited to a Rapat
	// pengurus, refused for a Rapat BPH.
	kadivID := o.kadivID(t)
	if c, _ := o.meeting(o.bph, bph, api.MeetingRequest{Title: "x", StartsAt: at, Kind: api.MeetingBPH, Participants: []id.ID{kadivID}}); c != 400 {
		t.Fatalf("kadiv in rapat BPH = %d", c)
	}
	if c, p := o.meeting(o.bph, bph, api.MeetingRequest{Title: "Rapat pengurus", StartsAt: at, Kind: api.MeetingPengurus, Participants: []id.ID{kadivID}}); c != 201 || p.Kind != api.MeetingPengurus {
		t.Fatalf("rapat pengurus = %d %+v", c, p)
	}
	if c, _ := o.meeting(o.bph, bph, api.MeetingRequest{Title: "x", StartsAt: at, Kind: api.MeetingPengurus, Participants: []id.ID{o.memberID}}); c != 400 {
		t.Fatalf("a member in rapat pengurus = %d", c)
	}
	var cands api.List[api.PersonRef]
	o.call("GET", "/api/spaces/"+bph.String()+"/meeting-candidates", o.bph, nil, &cands)
	if !slices.ContainsFunc(cands.Items, func(p api.PersonRef) bool { return p.ID == kadivID }) {
		t.Fatalf("kadiv not offered in BPH: %+v", cands.Items)
	}
}

func (o *monitorOrg) kadivID(t *testing.T) id.ID {
	var uid id.ID
	if err := o.st.DB().QueryRow(context.Background(), `select user_id from memberships where period_id=$1 and division_id=$2 and role='kadiv' and removed_at is null`,
		o.period.ID, o.div["mediatek"].ID).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	return uid
}

// The timeline lists every division's meetings for monitors only; their
// notes open (MON-03), and both are logged.
func TestMonitorMeetingTimeline(t *testing.T) {
	o := newMonitorOrg(t)
	at := time.Now().Add(-24 * time.Hour).UTC().Truncate(time.Minute)
	nid := o.note.ID
	c, m := o.meeting(o.kadiv, o.spaces["division-mediatek"], api.MeetingRequest{Title: "Rapat divisi Mediatek", StartsAt: at, NoteID: &nid})
	if c != 201 {
		t.Fatalf("meeting = %d", c)
	}
	path := "/api/periods/" + o.period.ID.String() + "/monitor/meetings"
	if c := o.call("GET", path, o.bph, nil, nil); c != 403 {
		t.Fatalf("timeline without grant = %d", c)
	}
	if c := o.call("GET", path, o.kadiv, nil, nil); c != 403 {
		t.Fatalf("timeline for a kadiv = %d", c)
	}
	o.grant()
	var list api.List[api.MonitorMeeting]
	if c := o.call("GET", path, o.bph, nil, &list); c != 200 {
		t.Fatalf("timeline = %d", c)
	}
	i := slices.IndexFunc(list.Items, func(x api.MonitorMeeting) bool { return x.ID == m.ID })
	if i < 0 || list.Items[i].Kind != api.MeetingDivision || list.Items[i].NoteTitle != o.note.Title || list.Items[i].SpaceName == "" {
		t.Fatalf("timeline %+v", list.Items)
	}
	if c := o.call("GET", path+"?from=nope", o.bph, nil, nil); c != 400 {
		t.Fatalf("bad range = %d", c)
	}
	var got api.Meeting
	if c := o.call("GET", "/api/meetings/"+m.ID.String(), o.bph, nil, &got); c != 200 || got.CanManage {
		t.Fatalf("monitor meeting = %d %+v", c, got)
	}
	var note api.Note
	if c := o.call("GET", "/api/notes/"+nid.String(), o.bph, nil, &note); c != 200 || len(note.Content) == 0 {
		t.Fatalf("notulen = %d", c)
	}
	if o.monitorAudits("monitor.read", o.div["mediatek"].ID) < 2 {
		t.Fatal("notulen read not audited with its division")
	}
	var n int
	o.st.DB().QueryRow(context.Background(), `select count(*) from audit_events where action='monitor.read' and after->>'detail' = 'Membaca timeline rapat lintas divisi'`).Scan(&n)
	if n != 1 {
		t.Fatalf("timeline reads audited = %d", n)
	}
}
