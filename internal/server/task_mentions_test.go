package server

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// @Sebutan di komentar (canvas 3747dbbac0d0 desain 4): typing @ lists the
// people who can see the task; the tagged get a task notification (the
// bell) and a device delivery (Ace), never WhatsApp; someone who cannot see
// the task cannot be tagged, and the author is not told about themself.
func TestTaskCommentMentions(t *testing.T) {
	o := newTaskOrg(t)
	task := o.create(false)
	var cands api.List[api.TaskMentionCandidate]
	if c := o.call("GET", taskPath(task)+"/mention-candidates", o.member, nil, &cands); c != 200 {
		t.Fatalf("candidates = %d", c)
	}
	ids, kadivID := []id.ID{}, id.Nil
	for _, c := range cands.Items {
		ids = append(ids, c.ID)
		if c.Role == api.RoleKadiv {
			kadivID = c.ID
		}
	}
	if !slices.Contains(ids, kadivID) || slices.Contains(ids, o.memberID) || slices.Contains(ids, o.peerID) {
		t.Fatalf("candidates %+v (kadiv %s member %s peer %s)", cands.Items, kadivID, o.memberID, o.peerID)
	}
	k := cands.Items[slices.Index(ids, kadivID)]
	if !k.OnTask || k.Role != api.RoleKadiv || k.Division == "" {
		t.Fatalf("kadiv candidate %+v", k)
	}
	// The peer of Humas cannot read the task: neither list nor tag.
	o.wantStatus(403, "GET", taskPath(task)+"/mention-candidates", o.peer, nil)
	o.wantStatus(400, "POST", taskPath(task)+"/comments", o.member, api.TaskCommentRequest{Body: "@Anggota Humas lihat ini", Mentions: []id.ID{o.peerID}})

	var comment api.TaskComment
	body := "@Kadiv mediatek tolong cek tanggalnya ya"
	if c := o.call("POST", taskPath(task)+"/comments", o.member, api.TaskCommentRequest{Body: body, Mentions: []id.ID{kadivID, kadivID, o.memberID}}, &comment); c != 201 {
		t.Fatalf("comment = %d", c)
	}
	if len(comment.Mentions) != 1 || comment.Mentions[0].ID != kadivID {
		t.Fatalf("mentions %+v", comment.Mentions)
	}
	got := o.get(task, o.kadiv)
	if n := len(got.Comments); n != 1 || len(got.Comments[0].Mentions) != 1 || got.Comments[0].Mentions[0].ID != kadivID {
		t.Fatalf("stored comments %+v", got.Comments)
	}
	var ns api.List[api.Notification]
	o.call("GET", "/api/notifications", o.kadiv, nil, &ns)
	i := slices.IndexFunc(ns.Items, func(n api.Notification) bool { return n.Title == mentionTitle })
	if i < 0 || ns.Items[i].TaskID != task.ID || !strings.Contains(ns.Items[i].Body, "tolong cek") || !strings.HasPrefix(ns.Items[i].Body, "PJ Mediatek di") {
		t.Fatalf("kadiv notifications %+v", ns.Items)
	}
	o.call("GET", "/api/notifications", o.member, nil, &ns)
	if slices.ContainsFunc(ns.Items, func(n api.Notification) bool { return n.Title == mentionTitle }) {
		t.Fatal("author told about their own tag")
	}
	var channels []string
	rows, err := o.st.DB().Query(context.Background(), `select d.channel from delivery_recipients d join delivery_events e on e.id=d.event_id where e.title=$1 and d.recipient_id=$2`, mentionTitle, kadivID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var ch string
		if err := rows.Scan(&ch); err != nil {
			t.Fatal(err)
		}
		channels = append(channels, ch)
	}
	if rows.Err() != nil || !slices.Equal(channels, []string{"device"}) {
		t.Fatalf("mention channels %v %v", channels, rows.Err())
	}
	// A comment with no tag tells no one.
	before := len(ns.Items)
	o.call("POST", taskPath(task)+"/comments", o.kadiv, api.TaskCommentRequest{Body: "Siap"}, nil)
	o.call("GET", "/api/notifications", o.member, nil, &ns)
	if len(ns.Items) != before {
		t.Fatal("untagged comment notified")
	}
	tooMany := make([]id.ID, maxCommentMentions+1)
	o.wantStatus(400, "POST", taskPath(task)+"/comments", o.member, api.TaskCommentRequest{Body: "x", Mentions: tooMany})
}

func TestMentionBody(t *testing.T) {
	long := strings.Repeat("a", 300)
	got := mentionBody("Aruna", "Revisi caption", "baris satu\n\nbaris  dua "+long)
	if !strings.HasPrefix(got, "Aruna di “Revisi caption”: baris satu baris dua ") || !strings.HasSuffix(got, "…") {
		t.Fatalf("mentionBody = %q", got)
	}
}
