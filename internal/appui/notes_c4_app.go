package appui

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// The page side of G3 (Edit | Baca, undo), C4 (mentions, action items)
// and T1 hooks on the note editor.

// noteActionSource is the action item a "Jadikan tugas" started from; the
// task's id goes back into it once made.
type noteActionSource struct {
	block, item int
	text        string
}

// wireNoteEditor gives the editor what it asks the page for.
func (a *App) wireNoteEditor(c *ui.Context, e *noteDocumentEditor) {
	e.Mentions = a.noteMentionTargets
	e.Members = a.noteMembers
	e.MemberName = a.noteMemberName
	zone := a.noteViewZone()
	now := c.Now()
	e.Today = func() timeutil.Date { return timeutil.DateOf(now.In(noteZone(zone))) }
	e.TaskLabel = a.noteTaskLabel
	e.OpenTask = a.openNoteTask
	e.OnActionTask = nil
	if e.editable && !a.notesWriteBlocked() && !a.isAdmin() {
		e.OnActionTask = func(block, item int, it api.NoteListItem) {
			text := strings.TrimSpace(noteSpansText(it.Spans))
			a.beginNoteConversion(api.NoteLinkTask, text)
			cv := a.notes.conversion
			if cv == nil || cv.kind != api.NoteLinkTask || cv.text != text || cv.target != id.Nil {
				return
			}
			cv.title = text
			if it.AssigneeID != nil {
				cv.assignee = *it.AssigneeID
			}
			if it.DueOn != nil {
				cv.date = it.DueOn.String()
			}
			cv.source = &noteActionSource{block: block, item: item, text: noteSpansText(it.Spans)}
			a.openNotePanel(c, a.notePanelTab())
		}
	}
}

// noteMembers: the note division's members for the PJ and "@" pickers;
// nil while they load.
func (a *App) noteMembers() []noteMember {
	n := a.notes.note
	if n == nil {
		return nil
	}
	key := n.OwnerDivisionID
	if a.notes.membersFor != key || a.notes.membersPeriod != a.periodID {
		a.notes.membersFor, a.notes.membersPeriod = key, a.periodID
		a.notes.members, a.notes.membersErr = nil, ""
		a.loadNoteMembers()
	}
	if a.notes.members == nil {
		return nil
	}
	out := make([]noteMember, 0, len(a.notes.members))
	for _, m := range a.notes.members {
		if m.UserID != id.Nil && m.DisplayName != "" && !slices.ContainsFunc(out, func(o noteMember) bool { return o.id == m.UserID }) {
			out = append(out, noteMember{id: m.UserID, name: m.DisplayName})
		}
	}
	return out
}

func (a *App) loadNoteMembers() {
	service, ok := a.workspaceService()
	if !ok {
		a.notes.members, a.notes.membersErr = []api.Member{}, "Direktori anggota belum tersedia."
		return
	}
	gen, seq, period, division := a.gen, a.notes.seq, a.periodID, a.notes.membersFor
	a.async(func() func() {
		members, err := service.Members(context.Background(), period, division)
		return func() {
			if gen != a.gen || seq != a.notes.seq || a.notes.membersFor != division || a.notes.membersPeriod != period {
				return
			}
			if err != nil {
				if client.IsUnauthenticated(err) {
					a.failed(err)
					return
				}
				a.notes.members, a.notes.membersErr = []api.Member{}, a.failed(err)
				return
			}
			if members == nil {
				members = []api.Member{}
			}
			a.notes.members = members
		}
	})
}

func (a *App) noteMemberName(uid id.ID) string {
	for _, m := range a.noteMembers() {
		if m.id == uid {
			return m.name
		}
	}
	return ""
}

// noteTaskLabel: "#T22" in this Space, else "Tugas".
func (a *App) noteTaskLabel(tid id.ID) string {
	for _, t := range a.tasks.items {
		if t.ID == tid {
			if n := a.taskNumber(t); n > 0 {
				return fmt.Sprintf("#T%d", n)
			}
			break
		}
	}
	return "Tugas"
}

// noteMentionTargets: members whose name holds the query, then tasks of
// the Space by number or title (only once something is typed).
func (a *App) noteMentionTargets(query string) []noteMentionTarget {
	q := strings.ToLower(strings.TrimSpace(query))
	out := []noteMentionTarget{}
	for _, m := range a.noteMembers() {
		if q == "" || strings.Contains(strings.ToLower(m.name), q) {
			out = append(out, noteMentionTarget{kind: api.MentionUser, id: m.id, text: "@" + m.name, detail: "anggota"})
			if len(out) == 6 {
				break
			}
		}
	}
	if q == "" {
		return out
	}
	tasks := 0
	for _, t := range a.tasks.items {
		num := a.taskNumber(t)
		code := ""
		if num > 0 {
			code = fmt.Sprintf("#T%d", num)
		}
		if !strings.Contains(strings.ToLower(code+" "+t.Title), strings.TrimPrefix(q, "#")) {
			continue
		}
		title := t.Title
		if r := []rune(title); len(r) > 40 {
			title = string(r[:40]) + "…"
		}
		text := strings.TrimSpace(code + " " + title)
		out = append(out, noteMentionTarget{kind: api.MentionTask, id: t.ID, text: text, detail: "tugas"})
		if tasks++; tasks == 4 {
			break
		}
	}
	return out
}

// openNoteTask opens a task made from (or mentioned in) the note.
func (a *App) openNoteTask(tid id.ID) {
	for _, t := range a.tasks.items {
		if t.ID == tid {
			a.openCanonicalTask(t)
			return
		}
	}
	s, ok := a.taskService()
	if !ok {
		return
	}
	gen := a.gen
	a.async(func() func() {
		task, err := s.GetTask(context.Background(), tid)
		return func() {
			if gen != a.gen {
				return
			}
			if err != nil {
				a.notes.notice = "Tugas belum dapat dibuka: " + a.failed(err)
				return
			}
			if !a.openCanonicalTask(task) {
				a.notes.notice = "Tugas ada di Space yang tidak dapat kamu buka."
			}
		}
	})
}

// noteModeSwitch: Edit | Baca for the notulis. Baca hides handles, menus
// and inputs; the editor lock and autosave go on.
func (a *App) noteModeSwitch(c *ui.Context, phone bool) {
	t := c.Theme()
	ui.Row(c.Key("note-mode")).Height(28).Padding(2).Gap(2).Radius(8).Background(spaceMix(t.Surface, t.Background, .4)).Shrink(0).Children(func() {
		seg := func(key, label, icon string, on bool) ui.Element {
			b := ui.ButtonBase(c.Key(key)).Label(label).Tooltip(label).Row().Height(24).Padding(0, 8).Gap(5).AlignItems(ui.Center).Radius(6).TextColor(t.TextMuted)
			if on {
				b.Background(t.Surface).TextColor(t.Text).Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, .08))
			}
			b.FocusRing(b.FocusVisible()).Children(func() {
				ui.Icon(c, spaceIcon(icon)).Size(13, 13)
				if !phone {
					ui.Text(c, map[string]string{"mode-edit": "Edit", "mode-read": "Baca"}[key]).FontSize(12)
				}
			})
			return b
		}
		if seg("mode-edit", "Mode edit", "edit", !a.notes.reading).Clicked() {
			a.notes.reading = false
		}
		if seg("mode-read", "Mode baca", "eye", a.notes.reading).Clicked() {
			a.notes.reading = true
			if e := a.notes.editor; e != nil {
				e.menu, e.mention, e.pick, e.advanced = noteBlockMenu{}, noteMentionMenu{}, notePick{}, false
			}
		}
	})
}

// noteUndoButtons: Urungkan and Ulangi in the bar (Ctrl+Z, Ctrl+Shift+Z).
func (a *App) noteUndoButtons(c *ui.Context) {
	e := a.notes.editor
	if e == nil {
		return
	}
	if spaceIconButton(c.Key("note-undo"), "Urungkan (Ctrl+Z)", "undo").Disabled(!e.CanUndo()).Clicked() {
		e.Undo()
	}
	if spaceIconButton(c.Key("note-redo"), "Ulangi (Ctrl+Shift+Z)", "redo").Disabled(!e.CanRedo()).Clicked() {
		e.Redo()
	}
}

// noteEditing: the notulis is writing (holds the editor, in Edit mode).
func (a *App) noteEditing() bool {
	return a.notesCanEdit() && !a.notes.reading && a.notes.compare == nil
}

// noteUndoShortcuts: Ctrl+Z and Ctrl+Shift+Z (Ctrl+Y) reach the document
// before the focused text field, so there is one history.
func (a *App) noteUndoShortcuts(c *ui.Context) {
	e := a.notes.editor
	if e == nil || !a.noteEditing() {
		return
	}
	for _, m := range []ui.Modifiers{ui.Ctrl, ui.Cmd} {
		c.OnShortcut(m, ui.KeyZ, func() { e.Undo() })
		c.OnShortcut(m|ui.Shift, ui.KeyZ, func() { e.Redo() })
	}
	c.OnShortcut(ui.Ctrl, ui.KeyY, func() { e.Redo() })
}
