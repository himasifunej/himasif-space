package server

import (
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
)

// An expired editor lock is not sent: a reader sees nobody writing, and
// may take the editor without force (as the takeover rules already allow).
func TestNoteExpiredLockIsNotSent(t *testing.T) {
	o := newOrg(t)
	kTok, _ := o.kadiv("mediatek", "km@himasif.test")
	aTok, _ := o.join(kTok, o.period.ID, "mediatek", "a@himasif.test", "A")
	bTok, _ := o.join(kTok, o.period.ID, "mediatek", "b@himasif.test", "B")
	sid := o.spaceID(aTok, "division-mediatek")
	var note api.Note
	o.call("POST", "/api/spaces/"+sid.String()+"/notes", aTok, api.NoteCreateRequest{Title: "N", Content: []api.NoteBlock{para("awal")}}, &note)
	nid := note.ID.String()
	base := o.srv.now()
	o.srv.now = func() time.Time { return base }
	o.call("POST", "/api/notes/"+nid+"/lock", aTok, nil, nil)

	var seen api.Note
	o.call("GET", "/api/notes/"+nid, bTok, nil, &seen)
	if seen.Lock == nil || seen.Lock.Editor.DisplayName != "A" || seen.Permissions.CanEdit {
		t.Fatalf("live lock as B sees it: %+v %+v", seen.Lock, seen.Permissions)
	}

	o.srv.now = func() time.Time { return base.Add(noteLockTTL + time.Minute) }
	for name, tok := range map[string]string{"B": bTok, "A": aTok} {
		seen = api.Note{}
		if c := o.call("GET", "/api/notes/"+nid, tok, nil, &seen); c != 200 {
			t.Fatalf("%s read = %d", name, c)
		}
		if seen.Lock != nil || !seen.Permissions.CanEdit {
			t.Fatalf("expired lock as %s sees it: %+v %+v", name, seen.Lock, seen.Permissions)
		}
	}
	var list api.List[api.Note]
	o.call("GET", "/api/spaces/"+sid.String()+"/notes", bTok, nil, &list)
	for _, n := range list.Items {
		if n.Lock != nil {
			t.Fatalf("list sends an expired lock: %+v", n.Lock)
		}
	}
	var lock api.NoteLock
	if c := o.call("POST", "/api/notes/"+nid+"/lock", bTok, nil, &lock); c != 200 || !lock.HeldByCaller {
		t.Fatalf("B takes the editor = %d %+v", c, lock)
	}
}
