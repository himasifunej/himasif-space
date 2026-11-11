package appui

import (
	"slices"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func meetingAt(title string, start time.Time, end *time.Time) api.Meeting {
	return api.Meeting{ID: id.New(), Title: title, StartsAt: start, EndsAt: end, TimeZone: "Asia/Jakarta", Version: 1}
}

func TestMeetingSectionsGroupByDayInTheMeetingZone(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Date(2026, 11, 11, 16, 20, 0, 0, loc)
	morning := meetingAt("Pagi tadi", time.Date(2026, 11, 11, 8, 0, 0, 0, loc), nil)
	evening := meetingAt("Koordinasi", time.Date(2026, 11, 11, 19, 30, 0, 0, loc), nil)
	soon := meetingAt("Briefing", time.Date(2026, 11, 14, 9, 0, 0, 0, loc), nil)
	later := meetingAt("Muslub lanjutan", time.Date(2026, 11, 20, 13, 0, 0, 0, loc), nil)
	old := meetingAt("Muslub", time.Date(2026, 11, 3, 13, 0, 0, 0, loc), nil)
	recent := meetingAt("Final Rabulus", time.Date(2026, 11, 5, 19, 0, 0, 0, loc), nil)
	got := meetingSections([]api.Meeting{later, old, evening, recent, soon, morning}, now)
	titles := func(s meetingSection) []string {
		out := []string{}
		for _, m := range s.items {
			out = append(out, m.Title)
		}
		return out
	}
	if len(got) != 3 || got[0].group != meetingToday || got[1].group != meetingUpcoming || got[2].group != meetingPast {
		t.Fatalf("sections %+v", got)
	}
	if !slices.Equal(titles(got[0]), []string{"Pagi tadi", "Koordinasi"}) {
		t.Fatalf("today %q: a meeting that passed this morning stays under Hari ini", titles(got[0]))
	}
	if !slices.Equal(titles(got[1]), []string{"Briefing", "Muslub lanjutan"}) {
		t.Fatalf("upcoming %q, want soonest first", titles(got[1]))
	}
	if !slices.Equal(titles(got[2]), []string{"Final Rabulus", "Muslub"}) {
		t.Fatalf("past %q, want newest first", titles(got[2]))
	}
	if def := meetingDefault([]api.Meeting{old, soon, evening}, now); def != evening.ID {
		t.Fatal("the detail should open on today's meeting first")
	}
	// A meeting at 23.30 WIB is still "today" in Jakarta though it is the
	// next day in UTC.
	late := meetingAt("Malam", time.Date(2026, 11, 11, 23, 30, 0, 0, loc), nil)
	if g := meetingGroupOf(late, now.UTC()); g != meetingToday {
		t.Fatalf("group %v, want Hari ini by the meeting's zone", g)
	}
}

func TestMeetingWhenSaysAMissingEndInsteadOfSelesai(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	start := time.Date(2026, 11, 11, 19, 30, 0, 0, loc)
	end := start.Add(time.Hour)
	if got := meetingWhen(meetingAt("A", start, &end)); got != "Rab 11 Nov · 19.30–20.30 WIB" {
		t.Fatalf("when %q", got)
	}
	if got := meetingWhen(meetingAt("A", start, nil)); got != "Rab 11 Nov · 19.30 WIB" {
		t.Fatalf("when without end %q", got)
	}
	next := time.Date(2026, 11, 12, 1, 0, 0, 0, loc)
	if got := meetingWhen(meetingAt("A", start, &next)); got != "Rab 11 Nov · 19.30 – Kam 12 Nov 01.00 WIB" {
		t.Fatalf("when across midnight %q", got)
	}
	now := time.Date(2026, 11, 11, 16, 20, 0, 0, loc)
	if s, k := meetingStatus(meetingAt("A", start, &end), now); s != "Hari ini · mulai 19.30 · 3 jam 10 menit lagi" || k != statusSoon {
		t.Fatalf("status %q", s)
	}
	if s, _ := meetingStatus(meetingAt("A", start.Add(-5*time.Hour), nil), now); s != "Hari ini · mulai 14.30 · jam selesai belum diisi" {
		t.Fatalf("status of a started meeting without end %q", s)
	}
	cancelled := meetingAt("A", start, nil)
	cancelled.Cancelled = true
	if s, k := meetingStatus(cancelled, now); s != "Dibatalkan · Rab 11 Nov" || k != statusCancelled {
		t.Fatalf("status cancelled %q", s)
	}
	if s, _ := meetingStatus(meetingAt("A", start.AddDate(0, 0, -6), nil), now); s != "Sudah lewat · Kam 5 Nov" {
		t.Fatalf("status past %q", s)
	}
}

func TestMeetingRowMetaAndReminders(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Date(2026, 11, 11, 16, 20, 0, 0, loc)
	past := meetingAt("Final Rabulus", time.Date(2026, 11, 5, 19, 0, 0, 0, loc), nil)
	past.Location = "Cak Kebo"
	past.Participants = []api.MeetingParticipant{{UserID: id.New(), AttendanceStatus: "unrecorded"}, {UserID: id.New()}}
	if meta, warn := meetingRowMeta(past, now); meta != "19.00  Cak Kebo" || warn != "Kehadiran belum dicatat" {
		t.Fatalf("meta %q warn %q", meta, warn)
	}
	past.Participants[0].AttendanceStatus = "present"
	if meta, warn := meetingRowMeta(past, now); meta != "19.00  Cak Kebo  1/2 hadir" || warn != "" {
		t.Fatalf("meta %q warn %q", meta, warn)
	}
	online := meetingAt("Koordinasi", time.Date(2026, 11, 11, 19, 30, 0, 0, loc), nil)
	online.JoinURL = "https://meet.example/abc"
	online.Participants = past.Participants[:1]
	online.Participants[0].AttendanceStatus = ""
	if meta, _ := meetingRowMeta(online, now); meta != "19.30  Daring  1 peserta" {
		t.Fatalf("meta %q", meta)
	}
	if got := meetingReminders([]int{15, 1440, 60}); got != "H-1, 1 jam, dan 15 menit sebelum" {
		t.Fatalf("reminders %q", got)
	}
	if got := meetingReminders(nil); got != "Tanpa pengingat" {
		t.Fatalf("reminders %q", got)
	}
	if _, ok := meetingArrivalDefault(online, now); ok {
		t.Fatal("no arrival can be filled in before the meeting starts")
	}
	req := meetingRequestFrom(past)
	if req.ExpectedVersion != past.Version || len(req.Participants) != 2 || req.Title != past.Title {
		t.Fatalf("request %+v", req)
	}
}

func TestMeetingsPageListsGroupsAndOpensAttendance(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.SetSize(1440, 900)
	sp := testAuthorizedSpace(h.app, h.api.divisions[0])
	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 23, 0, 0, 0, loc)
	end := today.Add(30 * time.Minute)
	upcoming := meetingAt("Briefing liputan Dies Natalis", today.AddDate(0, 0, 3), nil)
	upcoming.Location = "Sekretariat HIMASIF"
	past := meetingAt("[DUMMY Showcase] Koordinasi demo Medhum", today.AddDate(0, 0, -6), nil)
	past.Location = "Ruang demo HIMASIF"
	past.OrganizerID, past.CanManage = h.app.user.ID, true
	past.Participants = []api.MeetingParticipant{{UserID: id.New(), DisplayName: "Aruna Desain", AttendanceStatus: "present"}, {UserID: id.New(), DisplayName: "Nara Tekno", AttendanceStatus: "unrecorded"}}
	coord := meetingAt("Rapat koordinasi mingguan", today, &end)
	coord.JoinURL = "https://meet.example/abc"
	coord.ReminderMinutes = []int{1440, 60, 15}
	coord.Description = "Evaluasi konten minggu lalu dan bagi tugas Dies Natalis."
	h.app.meetings = meetingsState{space: sp.ID, loaded: true, items: []api.Meeting{past, upcoming, coord}}
	h.app.router.Push(PathSpaces)
	h.app.chrome.tab = "Rapat"
	h.tt.Frame()
	for _, s := range []string{"HARI INI", "AKAN DATANG", "SUDAH LEWAT", "Rapat koordinasi mingguan", "Waktu", "Pengingat", "H-1, 1 jam, dan 15 menit sebelum", "Ringkasan", "TUJUAN DAN AGENDA"} {
		h.want(s)
	}
	h.dontWant("— selesai")
	h.tt.Advance(300 * time.Millisecond)
	h.tt.Frame()
	captureNativeUI(t, h, "rapat-a-desktop")
	h.click("[DUMMY Showcase] Koordinasi demo Medhum")
	h.tt.Frame()
	h.want("Jam selesai belum diisi")
	h.want("Sudah lewat")
	h.click("Kehadiran · 1/2")
	h.tt.Frame()
	h.want("1 hadir · 0 izin · 0 tidak hadir · 1 belum dicatat")
	h.want("Kehadiran Nara Tekno")
	captureNativeUI(t, h, "rapat-a-attendance")

	h.tt.SetSize(390, 844)
	h.app.meetings.mobileDetail = false
	h.tt.Frame()
	h.want("Briefing liputan Dies Natalis")
	h.dontWant("TUJUAN DAN AGENDA")
	captureNativeUI(t, h, "rapat-a-phone-list")
	h.click("Briefing liputan Dies Natalis")
	h.tt.Frame()
	h.want("Kembali ke daftar rapat")
	h.want("TUJUAN DAN AGENDA")
	captureNativeUI(t, h, "rapat-a-phone-detail")
	h.click("Kembali ke daftar rapat")
	h.tt.Frame()
	h.want("SUDAH LEWAT")
}

func TestMeetingDetailShowsActionsAndTheSharedTimer(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.SetSize(1440, 900)
	sp := testAuthorizedSpace(h.app, h.api.divisions[0])
	start := time.Now().Add(-20 * time.Minute)
	m := meetingAt("Rapat koordinasi mingguan", start, nil)
	nid := id.New()
	m.NoteID, m.CanManage, m.CanRunTimer, m.OrganizerID = &nid, true, true, h.app.user.ID
	m.Participants = []api.MeetingParticipant{{UserID: id.New(), DisplayName: "Aruna Desain"}}
	h.app.meetings = meetingsState{space: sp.ID, loaded: true, items: []api.Meeting{m}, notes: []api.Note{{ID: nid, Title: "Notulensi · Rapat koordinasi mingguan"}}}
	h.app.router.Push(PathSpaces)
	h.app.chrome.tab = "Rapat"
	h.tt.Frame()
	for _, s := range []string{"Mulai rapat", "Buka notulen", "Ubah"} {
		h.want(s)
	}
	h.app.meetings.items[0].Timer = &api.MeetingTimer{StartedAt: start.Add(5 * time.Minute), DoneMs: []int64{}, UpdatedBy: api.PersonRef{DisplayName: "Demo Mediatek"}}
	h.tt.Advance(300 * time.Millisecond)
	h.tt.Frame()
	h.want("Jeda")
	h.want("Hentikan")
	h.dontWant("Mulai rapat")
	h.click("Notulen")
	h.tt.Frame()
	h.want("Notulensi · Rapat koordinasi mingguan")
	captureNativeUI(t, h, "rapat-a-live")
}
