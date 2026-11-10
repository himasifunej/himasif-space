package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// meetingNoteFixture: a member's meeting note with a decision and an
// action item, and the kadiv's meeting that points at it.
func meetingNoteFixture(t *testing.T, o *taskOrg) (api.Note, api.Meeting) {
	t.Helper()
	sid := o.spaces["division-mediatek"]
	due := timeutil.Date{Year: 2026, Month: 11, Day: 12}
	member := o.memberID
	content := []api.NoteBlock{
		{Type: api.BlockNumberedList, Items: []api.NoteListItem{{Spans: []api.Span{{Text: "Evaluasi"}}, Minutes: 10}}},
		{Type: api.BlockCallout, Variant: api.CalloutDecision, Spans: []api.Span{{Text: "Open recruitment dibuka 15 Nov"}}},
		{Type: api.BlockChecklist, Items: []api.NoteListItem{
			{Spans: []api.Span{{Text: "Poster oprec"}}, AssigneeID: &member, DueOn: &due},
			{Spans: []api.Span{{Text: "Catatan biasa tanpa PJ"}}},
		}},
		{Type: api.BlockToggle, Text: "Lain-lain", Children: []api.NoteBlock{{Type: api.BlockCallout, Variant: api.CalloutDecision, Spans: []api.Span{{Text: "Rapat berikutnya Jumat"}}}}},
	}
	var note api.Note
	if c := o.call("POST", "/api/spaces/"+sid.String()+"/notes", o.member, api.NoteCreateRequest{Title: "Rapat mingguan", Content: content}, &note); c != 201 {
		t.Fatalf("note = %d", c)
	}
	req := api.MeetingRequest{Title: "Rapat mingguan", Location: "Sekre", StartsAt: time.Now().UTC().Add(time.Hour).Truncate(time.Second), TimeZone: "Asia/Jakarta", NoteID: &note.ID}
	var m api.Meeting
	if c := o.call("POST", "/api/spaces/"+sid.String()+"/meetings", o.kadiv, req, &m); c != 201 {
		t.Fatalf("meeting = %d", c)
	}
	return note, m
}

// T2: the timer is on the meeting, on the server. The note's editor (a
// member) and the meeting's manager (the kadiv) steer the same timer;
// every action carries the version it saw, so a stale tap is refused; the
// timer never touches the meeting's own version.
func TestMeetingTimerShared(t *testing.T) {
	o := newTaskOrg(t)
	_, m := meetingNoteFixture(t, o)
	path := "/api/meetings/" + m.ID.String()
	var seen api.Meeting
	if c := o.call("GET", path, o.member, nil, &seen); c != 200 || !seen.CanRunTimer || seen.Timer != nil || seen.TimerVersion != 0 {
		t.Fatalf("member read %d canRun=%v timer=%v v=%d", c, seen.CanRunTimer, seen.Timer, seen.TimerVersion)
	}
	act := func(token, action string, version int64, want int) api.Meeting {
		t.Helper()
		var out api.Meeting
		if c := o.call("POST", path+"/timer", token, api.MeetingTimerRequest{Action: action, ExpectedVersion: version}, &out); c != want {
			t.Fatalf("%s v%d = %d, want %d", action, version, c, want)
		}
		return out
	}
	started := act(o.member, api.TimerStart, 0, 200)
	if started.Timer == nil || started.TimerVersion != 1 || started.Timer.UpdatedBy.DisplayName != "PJ Mediatek" || started.Timer.ServerNow.IsZero() {
		t.Fatalf("start %+v v%d", started.Timer, started.TimerVersion)
	}
	// The kadiv saw version 0: refused, the timer unchanged.
	act(o.kadiv, api.TimerPause, 0, 409)
	act(o.kadiv, api.TimerStart, 1, 409) // already running
	paused := act(o.kadiv, api.TimerPause, 1, 200)
	if paused.Timer.PausedAt == nil || paused.Timer.UpdatedBy.DisplayName == "PJ Mediatek" {
		t.Fatalf("pause %+v", paused.Timer)
	}
	act(o.member, api.TimerPause, 2, 409) // already paused
	act(o.member, api.TimerResume, 2, 200)
	next := act(o.member, api.TimerNext, 3, 200)
	if next.Timer.Agenda != 1 || len(next.Timer.DoneMs) != 1 || next.Timer.PausedAt != nil {
		t.Fatalf("next %+v", next.Timer)
	}
	ext := act(o.kadiv, api.TimerExtend, 4, 200)
	if ext.Timer.ExtraMs != api.TimerExtendStep.Milliseconds() {
		t.Fatalf("extend %+v", ext.Timer)
	}
	// Everyone who reads the meeting sees it, in the list too.
	var list api.List[api.Meeting]
	o.call("GET", "/api/spaces/"+o.spaces["division-mediatek"].String()+"/meetings", o.kadiv, nil, &list)
	if len(list.Items) != 1 || list.Items[0].Timer == nil || list.Items[0].TimerVersion != 5 || list.Items[0].Version != m.Version {
		t.Fatalf("list %+v", list.Items)
	}
	stopped := act(o.member, api.TimerStop, 5, 200)
	if stopped.Timer != nil || stopped.TimerVersion != 6 {
		t.Fatalf("stop %+v v%d", stopped.Timer, stopped.TimerVersion)
	}
	act(o.member, "lompat", 6, 400)
	// Another division cannot see it.
	if c := o.call("POST", path+"/timer", o.peer, api.MeetingTimerRequest{Action: api.TimerStart, ExpectedVersion: 6}, nil); c != 404 && c != 403 {
		t.Fatalf("peer start = %d", c)
	}

	// A meeting without a note: only its manager runs the timer.
	req := api.MeetingRequest{Title: "Tanpa catatan", StartsAt: time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second), TimeZone: "Asia/Jakarta"}
	var bare api.Meeting
	if c := o.call("POST", "/api/spaces/"+o.spaces["division-mediatek"].String()+"/meetings", o.kadiv, req, &bare); c != 201 {
		t.Fatalf("bare = %d", c)
	}
	o.call("GET", "/api/meetings/"+bare.ID.String(), o.member, nil, &seen)
	if seen.CanRunTimer {
		t.Fatal("member may run a timer of a meeting without note")
	}
	if c := o.call("POST", "/api/meetings/"+bare.ID.String()+"/timer", o.member, api.MeetingTimerRequest{Action: api.TimerStart}, nil); c != 403 {
		t.Fatalf("member start bare = %d", c)
	}
	if c := o.call("POST", "/api/meetings/"+bare.ID.String()+"/timer", o.kadiv, api.MeetingTimerRequest{Action: api.TimerStart}, nil); c != 200 {
		t.Fatalf("kadiv start bare = %d", c)
	}
}

// C7: the summary is previewed, then sent to the division's group through
// the bot: decisions, action items with PJ and due date, a link. Without a
// group it says so; a stale preview or a double tap is refused.
func TestNoteWASummary(t *testing.T) {
	o := newTaskOrg(t)
	ctx := context.Background()
	note, _ := meetingNoteFixture(t, o)
	path := "/api/notes/" + note.ID.String() + "/wa-summary"
	var pv api.NoteWASummary
	if c := o.call("POST", path, o.member, api.NoteWASummaryRequest{}, &pv); c != 200 || !strings.Contains(pv.Problem, "belum punya grup") {
		t.Fatalf("preview without group %d %+v", c, pv)
	}
	if c := o.call("POST", path, o.member, api.NoteWASummaryRequest{Send: true, ExpectedVersion: note.Version}, nil); c != 409 {
		t.Fatalf("send without group = %d", c)
	}
	cid, gid := id.New(), id.New()
	if _, err := o.st.DB().Exec(ctx, `insert into wa_connections(id,period_id,division_id,display_name,greeting,enabled,status,account_jid,created_by,updated_at) values($1,$2,$3,'Space Mediatek','HELLOOOO',true,'connected','6281111111111@s.whatsapp.net',$4,now())`,
		cid, o.period.ID, o.div["mediatek"].ID, o.memberID); err != nil {
		t.Fatal(err)
	}
	if _, err := o.st.DB().Exec(ctx, `insert into wa_groups(id,connection_id,space_id,period_id,jid,name,commands_enabled) values($1,$2,$3,$4,'120363000000000009@g.us','Grup Mediatek',true)`,
		gid, cid, o.spaces["division-mediatek"], o.period.ID); err != nil {
		t.Fatal(err)
	}
	pv = api.NoteWASummary{}
	if c := o.call("POST", path, o.member, api.NoteWASummaryRequest{}, &pv); c != 200 || pv.Problem != "" || pv.GroupName != "Grup Mediatek" || pv.Decisions != 2 || pv.Actions != 1 {
		t.Fatalf("preview %d %+v", c, pv)
	}
	for _, want := range []string{"*Ringkasan rapat: Rapat mingguan*", "· Sekre", "*Keputusan*\n1. Open recruitment dibuka 15 Nov\n2. Rapat berikutnya Jumat", "*Action item*\n• Poster oprec — PJ: PJ Mediatek · tenggat 12 Nov", "Catatan lengkap: "} {
		if !strings.Contains(pv.Text, want) {
			t.Fatalf("text lacks %q:\n%s", want, pv.Text)
		}
	}
	if strings.Contains(pv.Text, "HELLOOOO") || strings.Contains(pv.Text, "tanpa PJ") {
		t.Fatalf("text:\n%s", pv.Text)
	}
	if c := o.call("POST", path, o.peer, api.NoteWASummaryRequest{}, nil); c != 404 {
		t.Fatalf("peer preview = %d", c)
	}
	if c := o.call("POST", path, o.member, api.NoteWASummaryRequest{Send: true, ExpectedVersion: pv.Version - 1}, nil); c != 409 {
		t.Fatalf("stale send = %d", c)
	}
	var sent api.NoteWASummary
	if c := o.call("POST", path, o.member, api.NoteWASummaryRequest{Send: true, ExpectedVersion: pv.Version}, &sent); c != 200 || !sent.Queued || sent.OutboxID == nil {
		t.Fatalf("send %d %+v", c, sent)
	}
	var kind string
	var group id.ID
	var payload []byte
	if err := o.st.DB().QueryRow(ctx, `select source_kind,group_id,payload from wa_outbox where id=$1`, *sent.OutboxID).Scan(&kind, &group, &payload); err != nil || kind != "note_summary" || group != gid || !strings.Contains(string(payload), "Poster oprec") {
		t.Fatalf("outbox %v %s %v %s", err, kind, group, payload)
	}
	if c := o.call("POST", path, o.member, api.NoteWASummaryRequest{Send: true, ExpectedVersion: pv.Version}, nil); c != 409 {
		t.Fatalf("double send = %d", c)
	}
}
