package appui

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// pingAHarness opens a Space whose account may ping, with three other
// members loaded, and a fake that records the pings sent.
func pingAHarness(t *testing.T) (*harness, *pingPagesAPI, []api.PersonRef) {
	t.Helper()
	h, sp := newSpaceTabsHarness(t, &fakePreferences{})
	f := &pingPagesAPI{fakeAPI: h.api, pages: [][]api.PingRecord{nil, nil}}
	h.app.api = f
	people := []api.PersonRef{{ID: id.New(), DisplayName: "Rani Pratiwi"}, {ID: id.New(), DisplayName: "Sekar Ayu"}, {ID: id.New(), DisplayName: "Dimas Aditya"}}
	h.app.meetings = meetingsState{space: sp.ID, loaded: true, members: append(slices.Clone(people), api.PersonRef{ID: h.app.user.ID, DisplayName: "Saya Sendiri"})}
	h.tt.Frame()
	return h, f, people
}

// Option A: no tabs or internal text; tick a person, Kirim; the app is the
// channel, the empty message leaves the server's default; then the summary.
func TestPingDialogPickThenSend(t *testing.T) {
	h, f, people := pingAHarness(t)
	h.app.chrome.pingOpen = true
	h.tt.Frame()
	h.tt.Frame()
	for _, want := range []string{"Kirim ping", "Semua", "Juga ke WhatsApp", "Rani Pratiwi", "Semua menerima di aplikasi Space."} {
		h.want(want)
	}
	for _, gone := range []string{"Umum", "Perangkat", "bridge", "adapter", "Kirim ping sekarang", "Saya Sendiri"} {
		if slices.ContainsFunc(h.tt.Texts(), func(s string) bool { return strings.Contains(s, gone) }) {
			t.Errorf("text %q shown in %q", gone, h.tt.Texts())
		}
	}
	h.click("Rani Pratiwi")
	h.tt.Frame()
	h.want("Kirim ke 1 orang")
	h.click("Kirim ke 1 orang")
	h.settle()
	if len(f.sent) != 1 {
		t.Fatalf("sent %d pings", len(f.sent))
	}
	r := f.sent[0]
	if !slices.Equal(r.Recipients, []id.ID{people[0].ID}) || !slices.Equal(r.Channels, []string{"device"}) || r.Message != "" || r.SourceKind != "" || r.SourceID != nil {
		t.Fatalf("request %+v", r)
	}
	h.want("Ping terkirim ke 1 orang")
	h.click("Selesai")
	h.tt.Frame()
	if h.app.chrome.pingOpen || len(h.app.notificationSettingsState.recipients) != 0 {
		t.Fatalf("dialog open %v, recipients %v", h.app.chrome.pingOpen, h.app.notificationSettingsState.recipients)
	}
}

// Semua chooses everyone but oneself; the WhatsApp switch adds the
// channel and is remembered for the next Space.
func TestPingDialogGroupAndWhatsApp(t *testing.T) {
	h, f, people := pingAHarness(t)
	h.app.chrome.pingOpen = true
	h.tt.Frame()
	h.tt.Frame()
	h.click("Semua 3")
	h.tt.Frame()
	h.click("Juga ke WhatsApp")
	h.tt.Frame()
	h.click("Kirim ke 3 orang")
	h.settle()
	if len(f.sent) != 1 || len(f.sent[0].Recipients) != len(people) || !slices.Equal(f.sent[0].Channels, []string{"device", "whatsapp"}) {
		t.Fatalf("sent %+v", f.sent)
	}
	if slices.Contains(f.sent[0].Recipients, h.app.user.ID) {
		t.Fatal("pinged oneself")
	}
	h.app.resetPingState(h.app.selectedSpace(), id.Nil)
	if !h.app.notificationSettingsState.whatsapp || !h.app.notificationSettingsState.device {
		t.Fatal("WhatsApp choice not remembered")
	}
}

// A failed send keeps the choice and says so in words, with Coba kirim lagi.
func TestPingDialogFailureKeepsChoice(t *testing.T) {
	h, _, people := pingAHarness(t)
	h.app.conn = connOnline
	h.app.chrome.pingOpen = true
	h.tt.Frame()
	h.tt.Frame()
	h.app.notificationSettingsState.recipients = []id.ID{people[1].ID}
	h.app.pingUI.failed = true
	h.app.notificationSettingsState.err = "Server tidak dapat dihubungi."
	h.tt.Frame()
	h.want("Ping belum terkirim.")
	h.want("Coba kirim lagi")
	if len(h.app.notificationSettingsState.recipients) != 1 {
		t.Fatal("choice lost")
	}
}

// The server takes 1–500 recipients: groups stop at 500 and Kirim waits.
func TestPingRecipientLimit(t *testing.T) {
	var st notificationSettingsState
	g := pingGroup{key: "all"}
	for range 600 {
		g.members = append(g.members, id.New())
	}
	togglePingGroup(&st, g)
	if len(st.recipients) != pingMaxRecipients {
		t.Fatalf("%d recipients", len(st.recipients))
	}
	a := &App{conn: connOnline}
	st.pingKind = "general"
	if !a.pingCanSend(&st) {
		t.Fatal("500 recipients refused")
	}
	st.recipients = append(st.recipients, id.New())
	if a.pingCanSend(&st) {
		t.Fatal("501 recipients allowed")
	}
	st.recipients = nil
	if a.pingCanSend(&st) {
		t.Fatal("no recipients allowed")
	}
	st.recipients, st.pingKind = []id.ID{id.New()}, "task"
	if a.pingCanSend(&st) {
		t.Fatal("task ping without a task allowed")
	}
}

// Option C: "Ingatkan Rani" sends at once about the task to its PJ, only
// for who manages it, and not for oneself or a finished task.
func TestRemindTaskInline(t *testing.T) {
	h, f, people := pingAHarness(t)
	task := api.Task{ID: id.New(), Title: "Poster open recruitment", Progress: api.TaskDoing, Assignee: people[0], Permissions: api.TaskPermissions{Manage: true}}
	if !h.app.taskRemindable(task) {
		t.Fatal("task not remindable")
	}
	h.app.remindTask(task)
	if label, ready := h.app.pingRemindLabel(task.ID, "Rani", time.Now()); label != "Mengirim…" || ready {
		t.Fatalf("while sending %q %v", label, ready)
	}
	h.settle()
	if len(f.sent) != 1 {
		t.Fatalf("sent %d", len(f.sent))
	}
	r := f.sent[0]
	if r.SourceKind != "task" || r.SourceID == nil || *r.SourceID != task.ID || !slices.Equal(r.Recipients, []id.ID{people[0].ID}) || r.Message != "Mohon cek tugas: Poster open recruitment" {
		t.Fatalf("request %+v", r)
	}
	if label, ready := h.app.pingRemindLabel(task.ID, "Rani", time.Now()); !strings.HasPrefix(label, "Diingatkan ") || ready {
		t.Fatalf("after sending %q %v", label, ready)
	}
	if _, ready := h.app.pingRemindLabel(task.ID, "Rani", time.Now().Add(pingRemindGap)); !ready {
		t.Fatal("still blocked after a minute")
	}
	for name, tk := range map[string]api.Task{
		"no manage": {ID: id.New(), Progress: api.TaskDoing, Assignee: people[0]},
		"done":      {ID: id.New(), Progress: api.TaskDone, Assignee: people[0], Permissions: api.TaskPermissions{Manage: true}},
		"self":      {ID: id.New(), Progress: api.TaskDoing, Assignee: api.PersonRef{ID: h.app.user.ID}, Permissions: api.TaskPermissions{Manage: true}},
		"no PJ":     {ID: id.New(), Progress: api.TaskDoing, Permissions: api.TaskPermissions{Manage: true}},
	} {
		if h.app.taskRemindable(tk) {
			t.Errorf("%s: remindable", name)
		}
	}
}

// "Ingatkan peserta" pings the participants still in the Space, without
// oneself, about the meeting; not for a cancelled or finished meeting.
func TestRemindMeetingInline(t *testing.T) {
	h, f, people := pingAHarness(t)
	gone := id.New()
	m := api.Meeting{ID: id.New(), Title: "Rapat Divisi", StartsAt: time.Now().Add(time.Hour), TimeZone: "Asia/Jakarta", CanManage: true,
		Participants: []api.MeetingParticipant{{UserID: people[1].ID}, {UserID: people[2].ID}, {UserID: h.app.user.ID}, {UserID: gone}}}
	if !h.app.meetingRemindable(m, time.Now()) {
		t.Fatal("meeting not remindable")
	}
	h.app.remindMeeting(m)
	h.settle()
	if len(f.sent) != 1 {
		t.Fatalf("sent %d", len(f.sent))
	}
	r := f.sent[0]
	if r.SourceKind != "meeting" || *r.SourceID != m.ID || !slices.Equal(r.Recipients, []id.ID{people[1].ID, people[2].ID}) || !strings.HasPrefix(r.Message, "Pengingat rapat: Rapat Divisi, ") {
		t.Fatalf("request %+v", r)
	}
	cancelled, over, notMine := m, m, m
	cancelled.Cancelled = true
	over.StartsAt = time.Now().Add(-3 * time.Hour)
	notMine.CanManage = false
	for name, x := range map[string]api.Meeting{"cancelled": cancelled, "over": over, "not managed": notMine} {
		if h.app.meetingRemindable(x, time.Now()) {
			t.Errorf("%s: remindable", name)
		}
	}
}

// The header's name box: type a name, Enter adds it, Kirim sends and
// closes the box with a toast.
func TestPingNameBox(t *testing.T) {
	h, f, people := pingAHarness(t)
	h.click("Ping")
	h.tt.Frame()
	if !h.app.pingUI.boxOpen || h.app.chrome.pingOpen {
		t.Fatalf("box %v dialog %v", h.app.pingUI.boxOpen, h.app.chrome.pingOpen)
	}
	h.tt.Frame()
	h.tt.Type("sek")
	h.tt.Frame()
	h.tt.Key(0, ui.KeyEnter)
	h.tt.Frame()
	if !slices.Equal(h.app.notificationSettingsState.recipients, []id.ID{people[1].ID}) {
		t.Fatalf("recipients %v; texts %q", h.app.notificationSettingsState.recipients, h.tt.Texts())
	}
	h.click("Kirim ke 1 orang")
	h.settle()
	h.tt.Frame()
	if len(f.sent) != 1 || h.app.pingUI.boxOpen {
		t.Fatalf("sent %d, box open %v", len(f.sent), h.app.pingUI.boxOpen)
	}
	h.want("Ping terkirim ke 1 orang.")
}
