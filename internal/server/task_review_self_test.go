package server

import (
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Owner's decision (11 Nov 2026): a match needs no reason, a mismatch
// does; a kadiv (or an admin) may approve their own submission, recorded
// as self-approved; any other reviewer waits for someone else.
func TestTaskEvidenceSelfApproval(t *testing.T) {
	o := newTaskOrg(t)
	// The PJ is also a reviewer, as 4.5 of the division.
	if c := o.call("PUT", o.scope(o.period.ID, "mediatek")+"/45", o.kadiv, api.Appoint45Request{UserID: o.memberID}, nil); c != 201 {
		t.Fatalf("appoint 4.5 = %d", c)
	}
	task := o.create(false)
	path := taskPath(task)
	task = o.get(task, o.member)
	if !task.Permissions.Review || task.Permissions.SelfReview {
		t.Fatalf("4.5 permissions %+v", task.Permissions)
	}
	if k := o.get(task, o.kadiv); !k.Permissions.Review || !k.Permissions.SelfReview {
		t.Fatalf("kadiv permissions %+v", k.Permissions)
	}

	// The 4.5 reviews another's submission, never their own.
	var own api.TaskEvidence
	if c := o.call("POST", path+"/evidence", o.member, api.TaskEvidenceRequest{URL: "https://drive.google.com/file/d/a", ExpectedVersion: task.Version}, &own); c != 201 {
		t.Fatalf("submit = %d", c)
	}
	task = o.get(task, o.member)
	o.wantStatus(403, "POST", path+"/evidence/"+own.ID.String()+"/review", o.member, api.TaskEvidenceReviewRequest{Matched: true, ExpectedVersion: task.Version})
	o.wantStatus(400, "POST", path+"/evidence/"+own.ID.String()+"/review", o.kadiv, api.TaskEvidenceReviewRequest{Matched: false, ExpectedVersion: task.Version})
	// The kadiv approves it in one click, without a reason.
	if c := o.call("POST", path+"/evidence/"+own.ID.String()+"/review", o.kadiv, api.TaskEvidenceReviewRequest{Matched: true, ExpectedVersion: task.Version}, &own); c != 200 || own.MatchStatus != api.MatchVerified || own.ReviewReason != "" {
		t.Fatalf("approve = %d %+v", c, own)
	}

	// The kadiv's own submission: approved by themselves, and said so.
	task = o.create(false)
	path = taskPath(task)
	var mine api.TaskEvidence
	if c := o.call("POST", path+"/evidence", o.kadiv, api.TaskEvidenceRequest{Note: "Sudah diposting", ExpectedVersion: task.Version}, &mine); c != 201 {
		t.Fatalf("kadiv submit = %d", c)
	}
	task = o.get(task, o.kadiv)
	if c := o.call("POST", path+"/evidence/"+mine.ID.String()+"/review", o.kadiv, api.TaskEvidenceReviewRequest{Matched: true, ExpectedVersion: task.Version}, &mine); c != 200 || mine.ReviewReason != api.SelfApprovedReason {
		t.Fatalf("self approve = %d %+v", c, mine)
	}
	var history api.List[api.TaskEvent]
	o.call("GET", path+"/history", o.kadiv, nil, &history)
	last := history.Items[len(history.Items)-1]
	for _, e := range history.Items {
		if e.CreatedAt.After(last.CreatedAt) {
			last = e
		}
	}
	if last.Action != "task.evidence.self_approve" || last.Reason != api.SelfApprovedReason {
		t.Fatalf("history %+v", history.Items)
	}
	var audited int
	if err := o.st.DB().QueryRow(t.Context(), `select count(*) from audit_events where target_type='task' and target_id=$1 and action='task.evidence.self_approve'`, task.ID).Scan(&audited); err != nil || audited != 1 {
		t.Fatalf("audit %d %v", audited, err)
	}
}

// A submission may be files alone, or files with a link: the files are
// the uploader's own, of this task, given to one submission only.
func TestTaskEvidenceFiles(t *testing.T) {
	o := newTaskOrg(t)
	withAttachmentDir(t, o)
	task := o.create(false)
	path := taskPath(task)
	upload := func(token, name, body string) id.ID {
		t.Helper()
		aid := id.New()
		if c := o.call("PUT", taskAttachmentURL(task, aid, name), token, body, nil); c != 201 {
			t.Fatalf("upload %s = %d", name, c)
		}
		return aid
	}
	proof := upload(o.member, "bukti.txt", "hasil akhir")
	others := upload(o.kadiv, "kadiv.txt", "punya kadiv")
	task = o.get(task, o.member)
	o.wantStatus(409, "POST", path+"/evidence", o.member, api.TaskEvidenceRequest{AttachmentIDs: []id.ID{others}, ExpectedVersion: task.Version})
	o.wantStatus(409, "POST", path+"/evidence", o.member, api.TaskEvidenceRequest{AttachmentIDs: []id.ID{id.New()}, ExpectedVersion: task.Version})
	var e api.TaskEvidence
	if c := o.call("POST", path+"/evidence", o.member, api.TaskEvidenceRequest{AttachmentIDs: []id.ID{proof}, ExpectedVersion: task.Version}, &e); c != 201 || len(e.Files) != 1 || e.Files[0].ID != proof || e.URL != "" || e.Note != "" {
		t.Fatalf("files only = %d %+v", c, e)
	}
	task = o.get(task, o.kadiv)
	if len(task.Evidence) != 1 || len(task.Evidence[0].Files) != 1 || task.Evidence[0].Files[0].Name != "bukti.txt" {
		t.Fatalf("evidence %+v", task.Evidence)
	}
	// Rejected, then sent again: the old files stay with the old submission.
	if c := o.call("POST", path+"/evidence/"+e.ID.String()+"/review", o.kadiv, api.TaskEvidenceReviewRequest{Matched: false, Reason: "Berkas salah", ExpectedVersion: task.Version}, nil); c != 200 {
		t.Fatalf("reject = %d", c)
	}
	task = o.get(task, o.member)
	o.wantStatus(409, "POST", path+"/evidence", o.member, api.TaskEvidenceRequest{AttachmentIDs: []id.ID{proof}, ExpectedVersion: task.Version})
	second := upload(o.member, "bukti-2.txt", "hasil revisi")
	task = o.get(task, o.member)
	if c := o.call("POST", path+"/evidence", o.member, api.TaskEvidenceRequest{URL: "https://drive.google.com/file/d/b", AttachmentIDs: []id.ID{second}, ExpectedVersion: task.Version}, &e); c != 201 || len(e.Files) != 1 || e.URL == "" {
		t.Fatalf("link and file = %d %+v", c, e)
	}
}
