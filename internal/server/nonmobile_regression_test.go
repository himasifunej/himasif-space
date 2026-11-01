package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/config"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/seed"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/jackc/pgx/v5"
)

func TestOfflineTaskReceiptRejectsReusedIntentAndRechecksAuthority(t *testing.T) {
	o := newTaskOrg(t)
	task := o.create(false)
	req := api.OfflineMutation{ID: id.New(), Kind: "task.progress", TargetID: task.ID, BaseProgress: api.TaskTodo, Progress: api.TaskDoing}
	var first, again api.OfflineResult
	if c := o.call("POST", "/api/sync", o.member, req, &first); c != 200 {
		t.Fatalf("sync %d", c)
	}
	if c := o.call("POST", "/api/sync", o.member, req, &again); c != 200 || first.Task.Version != again.Task.Version {
		t.Fatalf("retry changed state %d %+v %+v", c, first, again)
	}
	req.Progress = api.TaskTodo
	if c := o.call("POST", "/api/sync", o.member, req, nil); c != 409 {
		t.Fatalf("reused UUID accepted: %d", c)
	}
	req.Progress = api.TaskDoing
	if c := o.call("POST", "/api/sync", o.peer, req, nil); c != 404 && c != 403 {
		t.Fatalf("other division accessed receipt: %d", c)
	}
}

func TestAttachmentRetryPrivacyAndDownloadChecksum(t *testing.T) {
	o := newTaskOrg(t)
	o.srv.cfg.AttachmentDir = t.TempDir()
	if err := os.Chmod(o.srv.cfg.AttachmentDir, 0700); err != nil {
		t.Fatal(err)
	}
	o.srv.cfg.AttachmentMaxBytes = 1024
	var note api.Note
	if c := o.call("POST", "/api/spaces/"+o.spaces["division-mediatek"].String()+"/notes", o.member, api.NoteCreateRequest{Title: "Private", Content: []api.NoteBlock{para("text")}}, &note); c != 201 {
		t.Fatal(c)
	}
	aid := id.New()
	path := "/api/notes/" + note.ID.String() + "/attachments/" + aid.String() + "/file?name=fixture.txt"
	var attachment api.NoteAttachment
	if c := o.call("PUT", path, o.member, "private bytes", &attachment); c != 200 && c != 201 {
		t.Fatalf("upload %d", c)
	}
	if c := o.call("PUT", path, o.member, "private bytes", nil); c != 200 && c != 201 {
		t.Fatalf("retry %d", c)
	}
	if c := o.call("PUT", path, o.member, "different bytes", nil); c != 409 {
		t.Fatalf("UUID reuse %d", c)
	}
	if c := o.call("GET", path, o.peer, nil, nil); c != 404 {
		t.Fatalf("private file leaked %d", c)
	}
	req, _ := http.NewRequest("GET", o.url+path, nil)
	req.Header.Set("Authorization", "Bearer "+o.member)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || string(data) != "private bytes" || attachment.SHA256 == "" || resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("download %d %+v %q", resp.StatusCode, attachment, data)
	}
}

func TestEmergencySnapshotAndOperationsRespectDivisionPrivacy(t *testing.T) {
	o := newTaskOrg(t)
	o.create(false)
	sid := o.spaces["division-mediatek"].String()
	var snapshot api.EmergencySnapshot
	if c := o.call("POST", "/api/spaces/"+sid+"/emergency-snapshots", o.member, api.EmergencySnapshotRequest{}, &snapshot); c != 201 {
		t.Fatalf("snapshot %d", c)
	}
	if snapshot.RowCount != 1 {
		t.Fatalf("snapshot %+v", snapshot)
	}
	path := "/api/emergency-snapshots/" + snapshot.ID.String() + "/csv"
	if c := o.call("GET", path, o.peer, nil, nil); c != 404 && c != 403 {
		t.Fatalf("CSV leaked %d", c)
	}
	ops := o.scope(o.period.ID, "mediatek") + "/operations"
	var view api.DivisionOperations
	if c := o.call("GET", ops, o.kadiv, nil, &view); c != 200 || !view.CanReadSources {
		t.Fatalf("operations %d %+v", c, view)
	}
	if c := o.call("GET", ops, o.peer, nil, nil); c != 403 && c != 404 {
		t.Fatalf("operations leaked %d", c)
	}
	if c := o.call("GET", ops, o.admin, nil, nil); c != 403 {
		t.Fatalf("admin work scope leaked %d", c)
	}
}

func TestRestoreActivationPlanLeaseCutoffAndRetainedReceipts(t *testing.T) {
	base := os.Getenv("SPACE_TEST_DATABASE_URL")
	if base == "" {
		t.Skip("SPACE_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	name := "space_restore_test_" + strings.ReplaceAll(id.New().String(), "-", "")
	if _, err = admin.Exec(ctx, "create database "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "drop database "+pgx.Identifier{name}.Sanitize()+" with (force)")
	uri, _ := url.Parse(base)
	uri.Path = "/" + name
	st, err := store.Open(ctx, uri.String())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err = st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = seed.Run(ctx, st, seed.Options{AdminEmail: adminEmail, AdminPassword: adminPassword}); err != nil {
		t.Fatal(err)
	}
	var period, division, space, user id.ID
	if err = st.DB().QueryRow(ctx, `select s.period_id,sd.division_id,s.id,u.id from spaces s join space_divisions sd on sd.space_id=s.id cross join users u where s.kind='division' and u.kind='organization_admin' order by s.code limit 1`).Scan(&period, &division, &space, &user); err != nil {
		t.Fatal(err)
	}
	connection, group, event, job := id.New(), id.New(), id.New(), id.New()
	future := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
	if _, err = st.DB().Exec(ctx, `insert into wa_connections(id,period_id,division_id,display_name,enabled,created_by,updated_at) values($1,$2,$3,'fixture',true,$4,now())`, connection, period, division, user); err != nil {
		t.Fatal(err)
	}
	if _, err = st.DB().Exec(ctx, `insert into wa_groups(id,connection_id,period_id,space_id,jid,name) values($1,$2,$3,$4,'12345@g.us','fixture')`, group, connection, period, space); err != nil {
		t.Fatal(err)
	}
	if _, err = st.DB().Exec(ctx, `insert into delivery_events(id,event_key,source_kind,source_id,source_version,space_id,title,time_zone,scheduled_at,created_at) values($1,'fixture-future','ping',$1,1,$2,'fixture','Asia/Jakarta',$3,now())`, event, space, future); err != nil {
		t.Fatal(err)
	}
	if _, err = st.DB().Exec(ctx, `insert into delivery_recipients(id,event_id,recipient_id,channel,status,available_at,accepted_at) values($1,$2,$3,'device','accepted',$4,now())`, id.New(), event, user, future); err != nil {
		t.Fatal(err)
	}
	if _, err = st.DB().Exec(ctx, `insert into wa_outbox(id,event_key,connection_id,group_id,source_kind,source_id,payload,status,available_at,created_at,updated_at) values($1,'fixture-ambiguous',$2,$3,'delivery',$4,'{}','unknown',$5,now(),now())`, job, connection, group, event, future); err != nil {
		t.Fatal(err)
	}
	before, err := st.QuarantineRestore(ctx, name, "synthetic restore drill")
	if err != nil {
		t.Fatal(err)
	}
	cutoff := time.Now().UTC().Truncate(time.Microsecond)
	preview := func(hash string) (store.ActivationPlan, error) {
		return st.ActivateRestore(ctx, name, before.RestoreID.String(), "test operator", "fixture review", cutoff, hash)
	}
	plan, err := preview("")
	if err != nil || plan.Applied || plan.Hash == "" {
		t.Fatalf("preview %+v %v", plan, err)
	}
	if err = st.VerifyRunMode(ctx, config.RunLive); err == nil {
		t.Fatal("preview activated restore")
	}
	if _, err = preview("bad hash"); err == nil {
		t.Fatal("incorrect plan accepted")
	}
	err = st.WithRuntimeLease(ctx, config.RunIsolatedRestore, func(context.Context) error {
		if _, e := preview(""); e == nil {
			t.Error("activation allowed serving process")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	applied, err := preview(plan.Hash)
	if err != nil || !applied.Applied {
		t.Fatalf("apply %+v %v", applied, err)
	}
	control, err := st.RuntimeControl(ctx, st.DB(), store.LockNone)
	if err != nil || control.ActivationID == nil || !control.OutboundNotBefore.Equal(cutoff) {
		t.Fatalf("cutoff %+v %v", control, err)
	}
	var status string
	var suppressed *time.Time
	if err = st.DB().QueryRow(ctx, `select status,suppressed_at from wa_outbox where id=$1`, job).Scan(&status, &suppressed); err != nil || status != "unknown" || suppressed == nil {
		t.Fatalf("ambiguous receipt changed: %s %v %v", status, suppressed, err)
	}
	if err = st.DB().QueryRow(ctx, `select d.status,e.suppressed_at from delivery_recipients d join delivery_events e on e.id=d.event_id where e.id=$1`, event).Scan(&status, &suppressed); err != nil || status != "accepted" || suppressed == nil {
		t.Fatalf("accepted receipt changed: %s %v %v", status, suppressed, err)
	}
	srv := New(config.Config{Env: "development"}, st, &captureMailer{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err = st.Tx(ctx, func(q store.Querier) error {
		if e := srv.enqueue(ctx, q, "fixture-future", "ping", event, 1, &space, "fresh epoch", "", "Asia/Jakarta", future, []id.ID{user}, []string{"device"}); e != nil {
			return e
		}
		return srv.enqueue(ctx, q, "old-slot", "ping", event, 1, &space, "must not backfill", "", "Asia/Jakarta", cutoff.Add(-time.Minute), []id.ID{user}, []string{"device"})
	}); err != nil {
		t.Fatal(err)
	}
	var eventCount int
	if err = st.DB().QueryRow(ctx, `select count(*) from delivery_events`).Scan(&eventCount); err != nil || eventCount != 2 {
		t.Fatalf("future event not regenerated or old slot backfilled: %d %v", eventCount, err)
	}
	// Inspect durable audit, rather than merely the function return value.
	var audit []byte
	if err = st.DB().QueryRow(ctx, `select after from audit_events where action='runtime.restore.activate'`).Scan(&audit); err != nil {
		t.Fatal(err)
	}
	var receipt store.ActivationPlan
	if err = json.Unmarshal(audit, &receipt); err != nil || !receipt.Applied {
		t.Fatalf("missing activation receipt: %s %v", audit, err)
	}
}

func TestMeetingSchedulerDeduplicatesAndCancellationStopsEvents(t *testing.T) {
	o := newTaskOrg(t)
	now := time.Now().UTC()
	o.srv.now = func() time.Time { return now }
	req := api.MeetingRequest{Title: "Rapat fixture", StartsAt: now.Add(2 * time.Hour), EndsAt: func() *time.Time { v := now.Add(3 * time.Hour); return &v }(), TimeZone: "Asia/Jakarta", Participants: []id.ID{o.memberID}, ReminderMinutes: []int{60}}
	var meeting api.Meeting
	if c := o.call("POST", "/api/spaces/"+o.spaces["division-mediatek"].String()+"/meetings", o.kadiv, req, &meeting); c != 201 {
		t.Fatalf("meeting %d", c)
	}
	for i := 0; i < 2; i++ {
		if err := o.srv.schedule(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := o.st.DB().QueryRow(context.Background(), `select count(*) from delivery_events where source_kind='meeting' and source_id=$1`, meeting.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("one scheduled slot expected: %d %v", count, err)
	}
	if c := o.call("GET", "/api/meetings/"+meeting.ID.String(), o.peer, nil, nil); c != 404 && c != 403 {
		t.Fatalf("meeting leaked %d", c)
	}
	req.ExpectedVersion = meeting.Version
	req.Cancelled = true
	if c := o.call("PUT", "/api/meetings/"+meeting.ID.String(), o.kadiv, req, &meeting); c != 200 {
		t.Fatalf("cancel %d", c)
	}
	if err := o.srv.schedule(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := o.st.DB().QueryRow(context.Background(), `select count(*) from delivery_events where source_kind='meeting' and source_id=$1 and source_version=1 and cancelled_at is null`, meeting.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("stale events active: %d %v", count, err)
	}
}

func TestMedhumH2AndSeparateDivisionAuthority(t *testing.T) {
	o := newTaskOrg(t)
	_, humas := o.join(o.admin, o.period.ID, "humas", "calendar-pj@himasif.test", "Humas PJ")
	hk, _ := o.org.kadiv("humas", "calendar-kadiv@himasif.test")
	var content api.MedhumContent
	request := api.MedhumCreateRequest{Title: "Calendar fixture", PostingDate: taskTestDate(t, "2026-11-10"), HumasPJID: humas.User.ID}
	path := "/api/spaces/" + o.spaces["medhum"].String() + "/contents"
	if c := o.call("POST", path, hk, request, &content); c != 201 {
		t.Fatalf("create content %d", c)
	}
	if content.DesignDate == nil || content.DesignDate.String() != "2026-11-08" || content.DesignPJID != nil {
		t.Fatalf("deadline/PJ invented: %+v", content)
	}
	if c := o.call("POST", path, o.member, request, nil); c != 403 {
		t.Fatalf("design member edited Humas calendar %d", c)
	}
}
