package appui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"github.com/himasifunej/himasif-space/internal/workspaceui"
)

// Detail tugas (canvas 3747dbbac0d0, design 4), opened from a task row of
// Untukku or Jobdesk saya: its progress, what it asks, the proof of the
// work, comments where typing @ tags people (the owner's ask, 12 Nov: the
// tagged hear it from Ace and the bell), and on the side its facts with
// "Ingatkan aku sebelum tenggat".

// PathUntukTask and PathUntukRapat are Untukku's detail pages.
const (
	PathUntukTask  = "/untukku/tugas"
	PathUntukRapat = "/untukku/rapat"
)

// mentionAPI lists whom a comment on a task can tag.
type mentionAPI interface {
	TaskMentionCandidates(context.Context, id.ID) ([]api.TaskMentionCandidate, error)
}

func (a *App) mentionService() (mentionAPI, bool) {
	if c, ok := a.api.(*client.Client); ok {
		return c.AccountSession(), true
	}
	s, ok := a.api.(mentionAPI)
	return s, ok
}

// mentionState is the comment box's @ list: the task's people, read once
// per task, those tagged so far, the row the keyboard is on, and the text
// the list was closed for with Esc.
type mentionState struct {
	task          id.ID
	loading, done bool
	err           string
	people        []api.TaskMentionCandidate
	picked        []api.TaskMentionCandidate
	active        int
	dismissed     string
	focus         bool
	// gen renews the input after a name is put in, so the caret goes to
	// the end of the text.
	gen int
}

// maxMentionRows is how many names the @ list shows at once.
const maxMentionRows = 6

// openUntukTask shows a task's Detail tugas, its back link leading to from.
func (a *App) openUntukTask(tid id.ID, from string) {
	a.untuk.task, a.untuk.from = tid, from
	a.navigate(PathUntukTask)
}

// mentionQuery is what follows the @ being typed at the end of text: the
// name so far, and where the @ is. A space after a whole name, a new line
// or a long run of words ends it.
func mentionQuery(text string) (string, int, bool) {
	at := strings.LastIndexByte(text, '@')
	if at < 0 || at > 0 && !unicode.IsSpace(rune(text[at-1])) {
		return "", 0, false
	}
	q := text[at+1:]
	if len(q) > 40 || strings.HasSuffix(q, " ") && q != "" || strings.Count(q, " ") > 2 {
		return "", 0, false
	}
	for _, r := range q {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune(" .'-[]", r) {
			return "", 0, false
		}
	}
	return q, at, true
}

// mentionMatches are the people whose name holds q: those on the task
// first, then by name.
func mentionMatches(people []api.TaskMentionCandidate, q string) []api.TaskMentionCandidate {
	q = strings.ToLower(strings.TrimSpace(q))
	var out []api.TaskMentionCandidate
	for _, p := range people {
		if q == "" || strings.Contains(strings.ToLower(p.DisplayName), q) {
			out = append(out, p)
		}
	}
	slices.SortStableFunc(out, func(x, y api.TaskMentionCandidate) int {
		switch {
		case x.OnTask && !y.OnTask:
			return -1
		case !x.OnTask && y.OnTask:
			return 1
		}
		return 0
	})
	if len(out) > maxMentionRows {
		out = out[:maxMentionRows]
	}
	return out
}

// commentMentions are the people tagged whose @name is still in body.
func commentMentions(body string, picked []api.TaskMentionCandidate) []id.ID {
	var out []id.ID
	for _, p := range picked {
		if strings.Contains(body, "@"+p.DisplayName) && !slices.Contains(out, p.ID) {
			out = append(out, p.ID)
		}
	}
	return out
}

// roleLabel is the @ list's "Staf · Mediatek".
func roleLabel(p api.TaskMentionCandidate) string {
	role := map[string]string{api.RoleKadiv: "Kadiv", api.RoleCompanion: "Pendamping", api.RoleMember: "Staf"}[p.Role]
	if role == "" {
		role = "Anggota"
	}
	if p.Division == "" {
		return role
	}
	return role + " · " + p.Division
}

func (a *App) loadMentionPeople(tid id.ID) {
	st := &a.untuk.mention
	if st.task != tid {
		*st = mentionState{task: tid}
	}
	if st.loading || st.done {
		return
	}
	svc, ok := a.mentionService()
	if !ok {
		st.done = true
		return
	}
	gen := a.gen
	st.loading = true
	a.async(func() func() {
		people, err := svc.TaskMentionCandidates(context.Background(), tid)
		return func() {
			if gen != a.gen || a.untuk.mention.task != tid {
				return
			}
			st := &a.untuk.mention
			st.loading, st.done = false, true
			if err != nil {
				st.err = a.failed(err)
				return
			}
			st.people = people
		}
	})
}

func (a *App) untukTaskView(c *ui.Context) {
	a.ensureTasks(true)
	t := a.taskByID(a.untuk.task)
	back := "Untukku"
	if a.untuk.from == PathJobdesk {
		back = "Jobdesk saya"
	}
	phone := isMobile(c)
	th := c.Theme()
	ui.Scroll(c.Key("untuk-task")).Fill().Children(func() {
		ui.Column(c).FillWidth().MaxWidth(1000).Padding(20, phoneSize(phone, 16, 32), 40, phoneSize(phone, 16, 32)).Gap(18).Children(func() {
			if backLink(c, back).Clicked() {
				a.navigate(firstNonEmpty(a.untuk.from, PathUntukku))
			}
			if t == nil {
				if a.tasks.loading || !a.tasks.loaded {
					muted(c, "Memuat tugas…")
				} else {
					pageNotice(c, "lock", "Tugas tidak tersedia", "Tugas ini tidak lagi ada di daftarmu, atau kamu tidak lagi dapat membukanya.")
				}
				return
			}
			a.inspectTask(*t)
			task := *t
			ui.Column(c.Key("dt-h")).FillWidth().Gap(4).Children(func() {
				ui.Row(c.Key("crumb")).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
					if s, i, ok := a.taskSpace(task); ok {
						spaceTile(c, s, i)
						muted(c, s.Name).FontSize(12)
					}
					ui.Text(c, a.taskCode(task)).Font(FontMono).FontSize(12).TextColor(th.TextMuted)
					if task.Category != "" {
						muted(c, "·").FontSize(12)
						muted(c, task.Category).FontSize(12)
					}
				})
				ui.Text(c, task.Title).FontSize(22).FontWeight(600)
			})
			a.untukProgress(c, task)
			if a.tasks.errToast != "" {
				ui.Text(c, a.tasks.errToast).FontSize(12).TextColor(th.Danger)
			}
			grid := ui.Row(c.Key("dt-grid")).FillWidth().Gap(24).AlignItems(ui.Start)
			if phone {
				grid.Column()
			}
			grid.Children(func() {
				ui.Column(c.Key("dt-main")).Grow(1).MinWidth(0).FillWidth().Gap(18).Children(func() {
					detailBox(c, "desc", func() {
						boxTitle(c, "Deskripsi")
						desc := strings.TrimSpace(task.Description)
						if desc == "" {
							muted(c, "Belum ada deskripsi.").FontSize(13)
						} else {
							ui.Text(c, desc).FontSize(13)
						}
						if out := strings.TrimSpace(task.ExpectedOutput); out != "" {
							boxTitle(c, "Output yang diharapkan").Padding(6, 0, 0, 0)
							ui.Text(c, out).FontSize(13)
						}
					})
					detailBox(c, "proof", func() { a.untukProof(c, task) })
					detailBox(c, "comments", func() { a.untukComments(c, task) })
				})
				aside := detailBox(c, "facts", func() { a.untukTaskFacts(c, task) })
				if !phone {
					aside.Width(340).Shrink(0)
				}
			})
		})
	})
}

// untukProgress is Belum mulai / Dikerjakan / Selesai. Selesai goes
// through the proof, as everywhere in Space: it opens Kirim bukti, and the
// task leaves Untukku once handed in.
func (a *App) untukProgress(c *ui.Context, t api.Task) {
	selected := map[string]int{api.TaskTodo: 0, api.TaskDoing: 1, api.TaskSubmitted: 2, api.TaskDone: 2}[t.Progress]
	if t.Progress == api.TaskCancelled {
		muted(c, "Tugas ini dibatalkan.").FontSize(13)
		return
	}
	can := (t.Permissions.Progress || t.Permissions.Manage) && !a.taskWriteBlocked()
	ui.Row(c.Key("dt-prog")).Disabled(!can).Children(func() {
		i := quietSeg(c, "Progres", []string{"Belum mulai", "Dikerjakan", "Selesai"}, nil, selected)
		if i < 0 || i == selected {
			return
		}
		if i == 2 {
			a.tasks.evidenceOpen = true
			a.toast("Kirim bukti pengerjaan; tugas selesai setelah dicocokkan", nil)
			return
		}
		lane := []workspaceui.Lane{workspaceui.Todo, workspaceui.Doing}[i]
		old := taskLane(t)
		tid := t.ID
		a.moveTask(workspaceui.MoveRequest{TaskID: tid, Destination: lane})
		a.toast("Progres: "+[]string{"Belum mulai", "Dikerjakan"}[i], func() {
			a.moveTask(workspaceui.MoveRequest{TaskID: tid, Destination: old})
		})
	})
}

// untukProof is Bukti pengerjaan: what was handed in, then the place to
// hand in more.
func (a *App) untukProof(c *ui.Context, t api.Task) {
	th := c.Theme()
	boxTitle(c, "Bukti pengerjaan")
	for _, e := range t.Evidence {
		ui.Column(c.Key(e.ID)).FillWidth().Gap(4).Padding(8, 10).Radius(8).Border(1, th.Border).Children(func() {
			if e.URL != "" {
				ui.Link(c, e.URL, e.URL).FontSize(13).TextColor(th.Text).MaxLines(1)
			}
			if e.Note != "" {
				ui.Text(c, e.Note).FontSize(13)
			}
			for _, f := range e.Files {
				a.evidenceFileRow(c, t, f, nil)
			}
			muted(c, matchLabel(e.MatchStatus)+" · "+e.SubmittedBy.DisplayName+" · "+taskTimestamp(e.SubmittedAt, t.TimeZone)).FontSize(11)
		})
	}
	canSubmit := (t.Permissions.Progress || t.Permissions.Manage) && t.Progress != api.TaskCancelled && t.Progress != api.TaskDone && t.MatchStatus != api.MatchPending && !a.offlineMode()
	if a.tasks.evidenceOpen && canSubmit {
		a.taskEvidenceForm(c, t)
		return
	}
	if !canSubmit {
		if len(t.Evidence) == 0 {
			muted(c, "Belum ada bukti.").FontSize(13)
		}
		return
	}
	drop := ui.ButtonBase(c.Key("drop")).Label("Kirim bukti pengerjaan").Column().FillWidth().Gap(6).AlignItems(ui.Center).Padding(16).Radius(10).Border(1, spaceMix(t2c(th), th.Border, .4))
	if drop.Hovered() {
		drop.Background(th.SurfaceHover)
	}
	drop.FocusRing(drop.FocusVisible()).Children(func() {
		ui.Icon(c, spaceIcon("upload")).Size(18, 18).TextColor(th.TextMuted)
		ui.Row(c).Gap(4).Children(func() {
			muted(c, "Kirim berkas atau tautan, lalu").FontSize(13)
			ui.Text(c, "serahkan hasil").FontSize(13).FontWeight(500).TextColor(th.Accent)
		})
		ui.Text(c, "PDF, gambar, atau tautan · maks. 20 MB").FontSize(11).TextColor(faint(th))
	})
	if drop.Clicked() {
		a.tasks.evidenceOpen = true
	}
}

// t2c is the dashed outline's color of the drop area.
func t2c(th *ui.Theme) ui.Color { return th.TextMuted }

// untukComments are the task's comments, then the box with its @ list.
func (a *App) untukComments(c *ui.Context, t api.Task) {
	th := c.Theme()
	boxTitle(c, "Komentar")
	ui.Column(c.Key("cm-list")).FillWidth().Gap(12).Children(func() {
		if len(t.Comments) == 0 {
			muted(c, "Belum ada komentar.").FontSize(13)
		}
		for _, cm := range t.Comments {
			ui.Row(c.Key(cm.ID)).FillWidth().Gap(10).AlignItems(ui.Start).Children(func() {
				avatar(c, cm.Author.DisplayName, 26)
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.End).Children(func() {
						ui.Text(c, cm.Author.DisplayName).FontSize(13).FontWeight(500)
						ui.Text(c, untukStamp(cm.CreatedAt, t.TimeZone)).Font(FontMono).FontSize(11).TextColor(faint(th))
					})
					commentBody(c, cm)
					if len(cm.Mentions) > 0 {
						names := make([]string, len(cm.Mentions))
						for i, m := range cm.Mentions {
							names[i] = m.DisplayName
						}
						ui.Row(c.Key("tagged")).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Icon(c, spaceIcon("bell")).Size(12, 12).TextColor(faint(th))
							ui.Text(c, "Ace memberi tahu "+strings.Join(names, ", ")).FontSize(11).TextColor(faint(th))
						})
					}
				})
			})
		}
	})
	if !t.Permissions.Comment {
		return
	}
	a.commentComposer(c, t, "Tulis komentar… ketik @ untuk menandai orang")
}

// commentComposer is the comment box with its @ list, shared by Untukku's
// Detail tugas and the board's "Komentar dan aktivitas": typing @ lists the
// task's people, Enter or Tab picks one, Esc closes the list.
func (a *App) commentComposer(c *ui.Context, t api.Task, placeholder string) {
	th := c.Theme()
	a.loadMentionPeople(t.ID)
	st := &a.untuk.mention
	q, at, typing := mentionQuery(a.tasks.comment)
	open := typing && a.tasks.comment != st.dismissed
	matches := mentionMatches(st.people, q)
	if st.active >= len(matches) {
		st.active = 0
	}
	if open {
		ui.Column(c.Key("cm-mention")).Label("Tandai orang").FillWidth().MaxWidth(320).Padding(4).Radius(10).Background(spaceMix(th.Surface, th.Accent, .06)).Border(1, spaceMix(th.Accent, th.Border, .3)).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, .35)).Children(func() {
			head := "Tandai orang"
			if q != "" {
				head = "Cocok dengan “" + q + "”"
			}
			ui.Text(c, head).FontSize(11).TextColor(faint(th)).Padding(6, 8, 4, 8)
			switch {
			case st.loading:
				muted(c, "Memuat anggota…").FontSize(12).Padding(10, 8)
			case st.err != "":
				ui.Text(c, st.err).FontSize(12).TextColor(th.Danger).Padding(10, 8)
			case len(matches) == 0:
				muted(c, "Tidak ada anggota bernama itu.").FontSize(12).Padding(10, 8)
			}
			for i, p := range matches {
				o := ui.ButtonBase(c.Key(p.ID)).Label("Tandai "+p.DisplayName).Row().FillWidth().Height(40).Gap(10).AlignItems(ui.Center).Padding(0, 8).Radius(7)
				if i == st.active || o.Hovered() {
					o.Background(th.SurfaceHover)
				}
				o.Children(func() {
					avatar(c, p.DisplayName, 24)
					ui.Column(c).Grow(1).MinWidth(0).Children(func() {
						ui.Text(c, p.DisplayName).FontSize(13).FontWeight(500).MaxLines(1)
						ui.Text(c, roleLabel(p)).FontSize(11).TextColor(faint(th)).MaxLines(1)
					})
					if p.OnTask {
						ui.Text(c, "di tugas ini").FontSize(11).TextColor(faint(th)).Shrink(0)
					}
				})
				if o.Clicked() {
					a.chooseMention(p, at)
				}
			}
		})
	}
	ui.Row(c.Key("cm-form")).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		in := ui.TextInput(c.Key(fmt.Sprint("task-comment-", st.gen)), &a.tasks.comment).Label("Tulis komentar").Placeholder(placeholder).Grow(1).MinWidth(0).Disabled(a.taskWriteBlocked())
		if st.focus {
			in.Focus()
			st.focus = false
		}
		switch {
		case open && len(matches) > 0 && in.Shortcut(0, ui.KeyDown):
			st.active = (st.active + 1) % len(matches)
		case open && len(matches) > 0 && in.Shortcut(0, ui.KeyUp):
			st.active = (st.active - 1 + len(matches)) % len(matches)
		case open && in.Shortcut(0, ui.KeyEscape):
			st.dismissed = a.tasks.comment
		case open && len(matches) > 0 && (in.Submitted() || in.Shortcut(0, ui.KeyTab)):
			a.chooseMention(matches[st.active], at)
		case in.Submitted():
			a.sendComment(t)
		}
		if primarySpaceButton(c, "Kirim", "send").Shrink(0).Disabled(a.taskWriteBlocked() || strings.TrimSpace(a.tasks.comment) == "").Clicked() {
			a.sendComment(t)
		}
	})
	ui.Text(c, "Orang yang kamu tandai dapat notifikasi dari Ace.").FontSize(12).TextColor(faint(th))
}

// chooseMention puts "@Name " where the @ being typed is.
func (a *App) chooseMention(p api.TaskMentionCandidate, at int) {
	st := &a.untuk.mention
	a.tasks.comment = a.tasks.comment[:at] + "@" + p.DisplayName + " "
	if !slices.ContainsFunc(st.picked, func(x api.TaskMentionCandidate) bool { return x.ID == p.ID }) {
		st.picked = append(st.picked, p)
	}
	st.active, st.focus = 0, true
	st.gen++
}

// sendComment posts the comment with the people still tagged in it.
func (a *App) sendComment(t api.Task) {
	body := strings.TrimSpace(a.tasks.comment)
	if body == "" {
		return
	}
	st := &a.untuk.mention
	mentions := commentMentions(body, st.picked)
	var names []string
	for _, p := range st.picked {
		if slices.Contains(mentions, p.ID) {
			names = append(names, p.DisplayName)
		}
	}
	tid := t.ID
	a.mutateTask("Komentar", func(s TasksAPI) error {
		_, err := s.AddTaskComment(context.Background(), tid, api.TaskCommentRequest{Body: body, Mentions: mentions})
		return err
	}, func() {
		a.tasks.comment = ""
		a.untuk.mention.picked, a.untuk.mention.dismissed = nil, ""
		if len(names) > 0 {
			a.toast("Ace memberi tahu "+strings.Join(names, ", "), nil)
		}
	})
}

// commentBody is a comment with its @names in the accent.
func commentBody(c *ui.Context, cm api.TaskComment) {
	th := c.Theme()
	if len(cm.Mentions) == 0 {
		ui.Text(c, cm.Body).FontSize(13)
		return
	}
	var spans []ui.Span
	rest := cm.Body
	for rest != "" {
		at, who := -1, ""
		for _, m := range cm.Mentions {
			if i := strings.Index(rest, "@"+m.DisplayName); i >= 0 && (at < 0 || i < at) {
				at, who = i, "@"+m.DisplayName
			}
		}
		if at < 0 {
			spans = append(spans, ui.Span{Text: rest})
			break
		}
		if at > 0 {
			spans = append(spans, ui.Span{Text: rest[:at]})
		}
		spans = append(spans, ui.Span{Text: who, Weight: 500, Color: th.Accent, Background: accentSoft(th), Radius: 4})
		rest = rest[at+len(who):]
	}
	ui.RichText(c, spans...).FontSize(13).FillWidth().MinWidth(0)
}

// untukStamp is a comment's "Sel 14.02".
func untukStamp(at time.Time, zone string) string {
	loc, _ := timeutil.LoadZone(zone)
	if loc == nil {
		loc = wib()
	}
	at = at.In(loc)
	return untukWeekdays[at.Weekday()][:3] + " " + at.Format("15.04")
}

// untukTaskFacts is the side: deadline, PJ, who gave it, who else is on
// it, the zone; then Ingatkan aku sebelum tenggat and Buka di Space.
func (a *App) untukTaskFacts(c *ui.Context, t api.Task) {
	th := c.Theme()
	now := c.Now()
	late := taskLate(t, now) && t.Progress != api.TaskDone
	factRow(c, "due", "Tenggat", func() {
		due := "Tanpa tenggat"
		if t.DeadlineAt != nil || t.DeadlineDate != nil {
			at := taskDue(t)
			if t.DeadlineAt == nil {
				loc, _ := timeutil.LoadZone(t.TimeZone)
				if loc == nil {
					loc = wib()
				}
				at = t.DeadlineDate.At(23, 59, loc)
			}
			loc, _ := timeutil.LoadZone(t.TimeZone)
			if loc == nil {
				loc = wib()
			}
			at = at.In(loc)
			due = fmt.Sprintf("%s, %d %s · %s", shortDays[at.Weekday()], at.Day(), untukMonths[at.Month()-1], at.Format("15.04"))
			if late {
				due += " · lewat"
			}
		}
		d := ui.Text(c, due).Font(FontMono).FontSize(13)
		if late {
			d.TextColor(th.Danger)
		}
	})
	factRow(c, "pj", "PJ", func() { personLine(c, "pj", firstNonEmpty(t.Assignee.DisplayName, "—"), "") })
	factRow(c, "by", "Pemberi", func() { personLine(c, "by", firstNonEmpty(t.CreatedBy.DisplayName, "—"), "") })
	factRow(c, "people", "Peserta", func() {
		if len(t.Participants) == 0 {
			ui.Text(c, "—").FontSize(13).TextColor(faint(th))
		}
		for _, p := range t.Participants {
			personLine(c, p.ID, p.DisplayName, "")
		}
	})
	factRow(c, "zone", "Zona", func() {
		ui.Text(c, zoneLabel(t.TimeZone)).FontSize(13)
	})
	ui.Column(c.Key("dt-side-acts")).FillWidth().Gap(8).Padding(12, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(th.Border).Children(func() {
		_, hasReminders := a.reminderService()
		can := hasReminders && !a.isAdmin() && (t.DeadlineAt != nil || t.DeadlineDate != nil) && !late && t.Progress != api.TaskDone
		if spaceButton(c, "Ingatkan aku sebelum tenggat", "bell", false).FillWidth().Border(1, th.Border).TextColor(th.Text).Disabled(!can || a.reminderCreateBlocked()).Clicked() {
			a.remindBeforeDeadline(t)
		}
		if spaceButton(c, "Buka di Space", "chat", false).FillWidth().TextColor(th.Text).Clicked() {
			a.OpenNotification(api.Delivery{SourceKind: "task", SourceID: t.ID})
		}
	})
}

// zoneLabel is "WIB · Asia/Jakarta".
func zoneLabel(zone string) string {
	abbr := map[string]string{"Asia/Jakarta": "WIB", "Asia/Pontianak": "WIB", "Asia/Makassar": "WITA", "Asia/Jayapura": "WIT"}[zone]
	if zone == "" {
		zone = timeutil.DefaultZoneName
		abbr = "WIB"
	}
	if abbr == "" {
		return zone
	}
	return abbr + " · " + zone
}

// remindBeforeDeadline makes a personal reminder an hour before the
// deadline, or on its morning for a deadline without a time.
func (a *App) remindBeforeDeadline(t api.Task) {
	svc, ok := a.reminderService()
	if !ok || a.reminderCreateBlocked() {
		return
	}
	req := api.ReminderRequest{Title: "Tenggat: " + t.Title, TimeZone: timeutil.DefaultZoneName}
	if len([]rune(req.Title)) > 200 {
		req.Title = string([]rune(req.Title)[:199]) + "…"
	}
	what := "1 jam sebelum tenggat"
	switch {
	case t.DeadlineAt != nil:
		at := t.DeadlineAt.Add(-time.Hour)
		req.DueKind, req.DueAt = api.DueInstant, &at
	case t.DeadlineDate != nil:
		d := *t.DeadlineDate
		req.DueKind, req.DueOn = api.DueDate, &d
		what = "pagi hari tenggat, 06.00"
	default:
		return
	}
	gen := a.gen
	a.reminders.busy = true
	a.async(func() func() {
		r, err := svc.CreateReminder(context.Background(), req)
		return func() {
			if gen != a.gen {
				return
			}
			a.reminders.busy = false
			if err != nil {
				a.toast("Pengingat belum dibuat: "+a.failed(err), nil)
				return
			}
			a.replaceReminder(r)
			rid := r.ID
			a.toast("Pengingat dibuat: "+what+" · lihat di Reminder pribadi", func() {
				a.reminders.selected = rid
				a.reminders.deleteConfirm = true
				a.deleteReminder()
			})
		}
	})
}
