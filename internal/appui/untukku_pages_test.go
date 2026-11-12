package appui

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// untukPagesFake answers what Untukku's pages ask beyond the fixture's
// fake: the tasks, comments with their tags, whom to tag, an answer to a
// meeting, and finishing a reminder.
type untukPagesFake struct {
	*fakeAPI
	TasksAPI
	RemindersAPI
	app      *App
	meetings []api.Meeting
	people   []api.TaskMentionCandidate
	comments []api.TaskCommentRequest
	rsvps    []api.RSVPRequest
	done     []id.ID
}

func (f *untukPagesFake) MyMeetings(context.Context, id.ID) ([]api.Meeting, error) {
	return f.meetings, nil
}
func (f *untukPagesFake) MyTasks(context.Context, id.ID, api.TaskFilter) ([]api.Task, error) {
	return slices.Clone(f.app.tasks.items), nil
}
func (f *untukPagesFake) Reminders(context.Context) ([]api.Reminder, error) {
	return slices.Clone(f.app.reminders.items), nil
}
func (f *untukPagesFake) GetReminder(_ context.Context, rid id.ID) (api.Reminder, error) {
	return *f.app.reminderByID(rid), nil
}
func (f *untukPagesFake) ReminderRecipients(context.Context, id.ID) ([]api.ReminderShare, error) {
	return nil, nil
}
func (f *untukPagesFake) ReminderHistory(context.Context, id.ID) ([]api.ReminderAuditEntry, error) {
	return nil, nil
}
func (f *untukPagesFake) ReminderPreferences(context.Context) (api.ReminderPreferences, error) {
	return api.ReminderPreferences{}, nil
}
func (f *untukPagesFake) AddTaskComment(_ context.Context, tid id.ID, req api.TaskCommentRequest) (api.TaskComment, error) {
	f.comments = append(f.comments, req)
	return api.TaskComment{ID: id.New(), Body: req.Body}, nil
}
func (f *untukPagesFake) TaskMentionCandidates(context.Context, id.ID) ([]api.TaskMentionCandidate, error) {
	return f.people, nil
}
func (f *untukPagesFake) MeetingRSVP(_ context.Context, mid id.ID, req api.RSVPRequest) (api.Meeting, error) {
	f.rsvps = append(f.rsvps, req)
	for _, m := range f.meetings {
		if m.ID == mid {
			for i := range m.Participants {
				if m.Participants[i].UserID == f.app.user.ID {
					m.Participants[i].RSVP, m.Participants[i].RSVPNote = req.Status, req.Note
				}
			}
			return m, nil
		}
	}
	return api.Meeting{}, nil
}
func (f *untukPagesFake) CompleteReminder(_ context.Context, rid id.ID, req api.ReminderCompleteRequest) (api.Reminder, error) {
	f.done = append(f.done, rid)
	r := *f.app.reminderByID(rid)
	r.Completed = req.Completed
	return r, nil
}

// untukPages is untukFixture with a task to read and comment on, people to
// tag, and a meeting the user is invited to.
func untukPages(t *testing.T) (*harness, *untukPagesFake) {
	t.Helper()
	h, mf, _ := untukFixture(t)
	f := &untukPagesFake{fakeAPI: h.api, app: h.app, meetings: mf.meetings}
	aruna, nara := id.New(), id.New()
	f.people = []api.TaskMentionCandidate{
		{ID: id.New(), DisplayName: "Bima Ketua", Role: api.RoleKadiv, Division: "BPH"},
		{ID: aruna, DisplayName: "Aruna Desain", Role: api.RoleMember, Division: "Mediatek", OnTask: true},
		{ID: nara, DisplayName: "Nara Tekno", Role: api.RoleMember, Division: "Mediatek"},
	}
	task := &h.app.tasks.items[0]
	task.Description = "Caption feed slide 2–5 masih memakai tanggal lama."
	task.ExpectedOutput = "Caption final di dokumen konten."
	task.Category = "Konten"
	task.TimeZone = "Asia/Jakarta"
	task.Assignee = api.PersonRef{ID: h.app.user.ID, DisplayName: h.app.user.DisplayName}
	task.CreatedBy = api.PersonRef{ID: aruna, DisplayName: "Aruna Desain"}
	task.Participants = []api.PersonRef{{ID: nara, DisplayName: "Nara Tekno"}}
	task.Permissions = api.TaskPermissions{Progress: true, Comment: true}
	task.Comments = []api.TaskComment{{ID: id.New(), Author: task.CreatedBy, Body: "Tolong cek lagi tanggalnya ya, @Nara Tekno jadwal OR mundur.", Mentions: []api.PersonRef{{ID: nara, DisplayName: "Nara Tekno"}}, CreatedAt: h.tt.Now().Add(-48 * time.Hour)}}
	m := &f.meetings[0]
	m.Description = "Gladi presentasi LPJ semester ganjil."
	m.ReminderMinutes = []int{30, 5}
	m.Participants = []api.MeetingParticipant{{UserID: aruna, DisplayName: "Aruna Desain"}, {UserID: h.app.user.ID, DisplayName: h.app.user.DisplayName}, {UserID: nara, DisplayName: "Nara Tekno"}}
	h.app.untuk.meetings = f.meetings
	h.app.api = f
	h.tt.Frame()
	return h, f
}

// Detail tugas: typing @ lists the people, those on the task first; a
// name is put in, and the comment is sent with the people still tagged.
func TestUntukTaskMention(t *testing.T) {
	h, f := untukPages(t)
	tid := h.app.tasks.items[0].ID
	h.app.openUntukTask(tid, PathUntukku)
	h.tt.Advance(400 * time.Millisecond)
	h.settle()
	h.tt.Frame()
	for _, s := range []string{"Detail tugas", "Deskripsi", "Output yang diharapkan", "Bukti pengerjaan", "Komentar", "Ace memberi tahu Nara Tekno", "Ingatkan aku sebelum tenggat", "Buka di Space", "Orang yang kamu tandai dapat notifikasi dari Ace."} {
		h.want(s)
	}
	h.click("Tulis komentar")
	h.tt.Type("Sudah aku revisi @")
	h.tt.Frame()
	h.want("Tandai orang")
	texts := strings.Join(h.tt.Texts(), "|")
	if i, j := strings.Index(texts, "Tandai Aruna Desain"), strings.Index(texts, "Tandai Bima Ketua"); i < 0 || j < 0 || i > j {
		t.Fatalf("on the task not first: %s", texts)
	}
	h.want("di tugas ini")
	h.tt.Type("nar")
	h.tt.Frame()
	h.dontWant("Tandai Bima Ketua")
	h.click("Tandai Nara Tekno")
	h.tt.Frame()
	if h.app.tasks.comment != "Sudah aku revisi @Nara Tekno " {
		t.Fatalf("comment %q", h.app.tasks.comment)
	}
	shot(t, h, "untuk-task")
	h.tt.Type("cek ya")
	h.click("Kirim")
	h.settle()
	if len(f.comments) != 1 || len(f.comments[0].Mentions) != 1 || f.comments[0].Mentions[0] != f.people[2].ID || f.comments[0].Body != "Sudah aku revisi @Nara Tekno cek ya" {
		t.Fatalf("sent %+v", f.comments)
	}
	if h.app.tasks.comment != "" {
		t.Fatal("box kept the comment")
	}
}

func TestMentionQuery(t *testing.T) {
	for text, want := range map[string]string{"@": "", "halo @ar": "ar", "@Nara Tek": "Nara Tek"} {
		if q, _, ok := mentionQuery(text); !ok || q != want {
			t.Errorf("%q: %q %v", text, q, ok)
		}
	}
	for _, text := range []string{"email@kampus", "halo", "@Nara Tekno ", "@a\nb"} {
		if _, _, ok := mentionQuery(text); ok {
			t.Errorf("%q opened the list", text)
		}
	}
	picked := []api.TaskMentionCandidate{{ID: id.New(), DisplayName: "Aruna"}, {ID: id.New(), DisplayName: "Nara"}}
	if got := commentMentions("cek @Nara", picked); len(got) != 1 || got[0] != picked[1].ID {
		t.Fatalf("mentions %v", got)
	}
}

// Rapat: the facts, the agenda, who comes with the user as (kamu), and
// Kehadiranmu answered with a reason.
func TestUntukMeetingRSVP(t *testing.T) {
	h, f := untukPages(t)
	h.app.openUntukMeeting(f.meetings[0].ID)
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	for _, s := range []string{"Rapat Divisi", "Waktu", "Tempat", "Sekret", "30 menit dan 5 menit sebelumnya", "Agenda", "Kehadiranmu", "Peserta · 3", "(kamu)", "Catatan rapat"} {
		h.want(s)
	}
	h.click("Tidak bisa hadir")
	h.settle()
	h.tt.Frame()
	if len(f.rsvps) != 1 || f.rsvps[0].Status != api.RSVPNo {
		t.Fatalf("rsvp %+v", f.rsvps)
	}
	h.click("Alasan")
	h.tt.Type("Kuliah pengganti")
	h.click("Kirim alasan")
	h.settle()
	if len(f.rsvps) != 2 || f.rsvps[1].Note != "Kuliah pengganti" {
		t.Fatalf("reason %+v", f.rsvps)
	}
	shot(t, h, "untuk-rapat")
}

// Untukku: Selesai finishes a reminder, which stays struck through with
// Buka lagi, and Urungkan opens it again; the filter carries counts.
func TestUntukkuFinishReminderUndo(t *testing.T) {
	h, f := untukPages(t)
	h.want("Minggu ")
	h.want("Tugas, rapat, dan pengingatmu dari")
	h.click("Selesai Bayar kas")
	h.settle()
	h.tt.Frame()
	if len(f.done) != 1 {
		t.Fatal("reminder not finished")
	}
	h.want("Buka lagi")
	h.want("“Bayar kas” selesai")
	h.click("Urungkan")
	h.settle()
	h.tt.Frame()
	if len(f.done) != 2 || h.app.reminders.items[0].Completed {
		t.Fatal("Urungkan did not open it again")
	}
	h.want("Semua, 4")
}

// The pages next to the canvas, for review: SPACE_SHOTS.
func TestUntukPagesShots(t *testing.T) {
	if os.Getenv("SPACE_SHOTS") == "" {
		t.Skip("SPACE_SHOTS not set")
	}
	h, _ := untukPages(t)
	h.app.navigate(PathJobdesk)
	h.tt.Advance(400 * time.Millisecond)
	shot(t, h, "jobdesk")
	h.app.navigate(PathReminder)
	h.tt.Advance(400 * time.Millisecond)
	h.settle()
	h.tt.Advance(400 * time.Millisecond)
	shot(t, h, "reminder")
	h.app.reminders.prefsLoaded = true
	h.app.beginReminder()
	h.tt.Advance(400 * time.Millisecond)
	shot(t, h, "reminder-dialog")
	h.app.openReminderWhen(h.app.reminders.draft, false)
	h.tt.Advance(400 * time.Millisecond)
	shot(t, h, "reminder-when")
	h.app.when(reminderWhenKey).open = false
	h.app.reminders.draft.title = "Kumpulkan berkas LPJ ke Bendahara"
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	h.tt.Advance(400 * time.Millisecond)
	shot(t, h, "reminder-dialog")
}
