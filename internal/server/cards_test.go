package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/himasifunej/himasif-space/internal/config"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/whatsapp"
)

// A card's link is the public web's /t/ or /r/, without any sign-in; the
// server sends the browser on to the app's page of it.
func TestCardLinksRedirectWithoutSession(t *testing.T) {
	s := &Server{cfg: config.Config{PublicURL: "https://space.example/"}}
	tid := id.New()
	if got := s.cardLink("task", tid); got != "https://space.example/t/"+tid.String() {
		t.Fatalf("task link %q", got)
	}
	if got := s.cardLink("meeting", tid); got != "https://space.example/r/"+tid.String() {
		t.Fatalf("meeting link %q", got)
	}
	if s.cardLink("reminder", tid) != "" || s.cardLink("task", id.Nil) != "" {
		t.Fatal("link for something that is no card")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /t/{id}", s.openCard("tugas"))
	mux.HandleFunc("GET /r/{id}", s.openCard("rapat"))
	for path, want := range map[string]string{"/t/" + tid.String(): "/#/tugas/" + tid.String(), "/r/" + tid.String(): "/#/rapat/" + tid.String()} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != http.StatusFound || w.Header().Get("Location") != want || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s: %d %q", path, w.Code, w.Header().Get("Location"))
		}
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/t/bukan-id", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("bad id: %d", w.Code)
	}
}

// A task's WhatsApp message to the group links its card.
func TestWhatsAppTaskNoticeLinksCard(t *testing.T) {
	o := newTaskOrg(t)
	ctx := context.Background()
	cid, gid := id.New(), id.New()
	if _, err := o.st.DB().Exec(ctx, `insert into wa_connections(id,period_id,division_id,display_name,enabled,status,account_jid,created_by,updated_at) values($1,$2,$3,'Space Mediatek',true,'connected','6281111111111@s.whatsapp.net',$4,now())`,
		cid, o.period.ID, o.div["mediatek"].ID, o.memberID); err != nil {
		t.Fatal(err)
	}
	if _, err := o.st.DB().Exec(ctx, `insert into wa_groups(id,connection_id,space_id,period_id,jid,name,commands_enabled) values($1,$2,$3,$4,'120363000000000009@g.us','Grup Mediatek',true)`,
		gid, cid, o.spaces["division-mediatek"], o.period.ID); err != nil {
		t.Fatal(err)
	}
	task := o.create(false)
	rows, err := o.st.DB().Query(ctx, `select w.payload from wa_outbox w join delivery_events e on e.id=w.source_id where w.source_kind='delivery' and e.related_kind='task' and e.related_id=$1`, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	link := o.srv.cardLink("task", task.ID)
	found := 0
	for rows.Next() {
		var raw []byte
		var r whatsapp.SendRequest
		if err := rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		_ = json.Unmarshal(raw, &r)
		if !strings.Contains(r.Text, "Buka di Space: "+link) {
			t.Fatalf("message without the card's link: %q", r.Text)
		}
		found++
	}
	if found == 0 || !strings.HasPrefix(link, "http") {
		t.Fatalf("no WhatsApp message for the new task (link %q)", link)
	}
}
