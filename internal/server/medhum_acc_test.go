package server

import (
	"strings"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// medhumDesign makes a content whose design task belongs to pic and
// returns the content and that task.
func (o *taskOrg) medhumDesign(humasKadiv string, humasPJ, pic id.ID, title string) (api.MedhumContent, api.Task) {
	o.t.Helper()
	var c api.MedhumContent
	path := "/api/spaces/" + o.spaces["medhum"].String() + "/contents"
	req := api.MedhumCreateRequest{Title: title, PostingDate: taskTestDate(o.t, "2026-12-10"), HumasPJID: humasPJ}
	if code := o.call("POST", path, humasKadiv, req, &c); code != 201 {
		o.t.Fatalf("create content = %d", code)
	}
	if code := o.call("PATCH", "/api/contents/"+c.ID.String(), o.kadiv, api.MedhumUpdateRequest{ExpectedVersion: c.Version, DesignPJID: &pic}, &c); code != 200 || c.DesignTaskID == nil {
		o.t.Fatalf("assign design PIC = %d %+v", code, c)
	}
	return c, o.get(api.Task{ID: *c.DesignTaskID}, o.kadiv)
}

func (o *taskOrg) submit(task api.Task, token string) api.TaskEvidence {
	o.t.Helper()
	task = o.get(task, token)
	var e api.TaskEvidence
	if code := o.call("POST", taskPath(task)+"/evidence", token, api.TaskEvidenceRequest{URL: "https://drive.example.invalid/desain", ExpectedVersion: task.Version}, &e); code != 201 && code != 200 {
		o.t.Fatalf("submit = %d", code)
	}
	return e
}

func (o *taskOrg) review(task api.Task, e api.TaskEvidence, token string, matched bool, reason string) (int, api.TaskEvidence) {
	o.t.Helper()
	task = o.get(task, o.kadiv)
	var out api.TaskEvidence
	code := o.call("POST", taskPath(task)+"/evidence/"+e.ID.String()+"/review", token, api.TaskEvidenceReviewRequest{Matched: matched, Reason: reason, ExpectedVersion: task.Version}, &out)
	return code, out
}

func (o *taskOrg) notified(token, title string) bool {
	o.t.Helper()
	var ns api.List[api.Notification]
	if code := o.call("GET", "/api/notifications", token, nil, &ns); code != 200 {
		o.t.Fatalf("notifications = %d", code)
	}
	for _, n := range ns.Items {
		if strings.Contains(n.Title, title) {
			return true
		}
	}
	return false
}

// PRD MED-10–15 (register R-07): one ACC from the PJ Humas, a Humas
// manager or a Mediatek manager; never from the PIC; recorded with its
// side; posting only after it; both sides hear of each step.
func TestMedhumSharedACC(t *testing.T) {
	o := newTaskOrg(t)
	hk, _ := o.org.kadiv("humas", "acc-humas-kadiv@himasif.test")
	pjTok, pj := o.join(o.admin, o.period.ID, "humas", "acc-pj-humas@himasif.test", "PJ Humas Uji")
	content, design := o.medhumDesign(hk, pj.User.ID, o.memberID, "Poster uji ACC")

	// The PJ Humas may review only this design task; others may not.
	if got := o.get(design, pjTok).Permissions; !got.Review || got.ReviewSide != "humas" || got.SelfReview {
		t.Fatalf("PJ Humas permissions = %+v", got)
	}
	if got := o.get(design, o.member).Permissions; got.Review {
		t.Fatalf("PIC may review own design: %+v", got)
	}
	if got := o.get(design, o.kadiv).Permissions; !got.Review || got.ReviewSide != "mediatek" {
		t.Fatalf("Mediatek kadiv permissions = %+v", got)
	}

	// Posting waits for the ACC.
	now := time.Now().UTC().Add(-time.Minute)
	if code := o.call("PATCH", "/api/contents/"+content.ID.String(), hk, api.MedhumUpdateRequest{ExpectedVersion: 2, PublishedAt: &now}, nil); code != 409 {
		t.Fatalf("posted before ACC = %d", code)
	}

	e := o.submit(design, o.member)
	if !o.notified(pjTok, "Hasil dikirim") {
		t.Fatal("PJ Humas not told of the submission")
	}
	if code, _ := o.review(design, e, o.member, true, ""); code != 403 {
		t.Fatalf("PIC ACC = %d", code)
	}
	if code, _ := o.review(design, e, o.peer, true, ""); code != 403 {
		t.Fatalf("plain Humas member ACC = %d", code)
	}
	if code, _ := o.review(design, e, pjTok, false, ""); code != 400 {
		t.Fatalf("revision without a note = %d", code)
	}
	code, acc := o.review(design, e, pjTok, true, "")
	if code != 200 || acc.ReviewSide != "humas" || acc.MatchStatus != api.MatchVerified {
		t.Fatalf("PJ Humas ACC = %d %+v", code, acc)
	}
	if !o.notified(o.member, "Desain di-ACC · Humas") {
		t.Fatal("PIC not told of the ACC")
	}
	if c := o.call("GET", "/api/contents/"+content.ID.String(), hk, nil, &content); c != 200 || content.DesignAcc == nil || content.DesignAcc.By.ID != pj.User.ID || content.DesignAcc.Side != "humas" {
		t.Fatalf("content ACC trail = %d %+v", c, content.DesignAcc)
	}
	if o.audits("medhum.design.acc") != 1 {
		t.Fatal("ACC not audited")
	}
	if code := o.call("PATCH", "/api/contents/"+content.ID.String(), hk, api.MedhumUpdateRequest{ExpectedVersion: content.Version, PublishedAt: &now}, nil); code != 200 {
		t.Fatalf("posting after ACC = %d", code)
	}
	if !o.notified(o.member, "Konten sudah diposting") {
		t.Fatal("PIC not told of the posting")
	}

	// A Mediatek kadiv who is the PIC cannot approve their own design; a
	// Humas kadiv asks for a revision, with a note.
	_, own := o.medhumDesign(hk, pj.User.ID, mediatekKadiv(o), "Poster kadiv")
	e = o.submit(own, o.kadiv)
	if code, _ := o.review(own, e, o.kadiv, true, ""); code != 403 {
		t.Fatalf("kadiv PIC self ACC = %d", code)
	}
	code, rev := o.review(own, e, hk, false, "Logo HIMASIF belum ada")
	if code != 200 || rev.ReviewSide != "humas" || rev.MatchStatus != api.MatchRejected {
		t.Fatalf("Humas kadiv revision = %d %+v", code, rev)
	}
	if !o.notified(o.kadiv, "Desain perlu perbaikan · Humas") {
		t.Fatal("PIC not told of the revision")
	}

	// Other Mediatek work stays closed to Humas (MED-15).
	plain := o.create(true)
	e = o.submit(plain, o.member)
	if code, _ := o.review(plain, e, hk, true, ""); code != 403 {
		t.Fatalf("Humas kadiv reviewed a non-Medhum Mediatek task = %d", code)
	}
	if got := o.get(plain, o.kadiv).Permissions; got.ReviewSide != "" {
		t.Fatalf("review side on a plain task: %+v", got)
	}
}

func mediatekKadiv(o *taskOrg) id.ID {
	o.t.Helper()
	var me api.User
	if c := o.call("GET", "/api/me", o.kadiv, nil, &me); c != 200 {
		o.t.Fatalf("me = %d", c)
	}
	return me.ID
}
