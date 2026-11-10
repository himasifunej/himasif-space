package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
)

// builtinTemplateKind reports whether kind is a built-in template a note
// may be created as.
func builtinTemplateKind(kind string) bool {
	switch kind {
	case api.TemplateMeeting, api.TemplateSchedule, api.TemplateEvaluation, api.TemplateBrief:
		return true
	}
	return false
}

func bullets() api.NoteBlock {
	return api.NoteBlock{Type: api.BlockBulletedList, Items: []api.NoteListItem{{Spans: []api.Span{}}}}
}

func checklist() api.NoteBlock {
	return api.NoteBlock{Type: api.BlockChecklist, Items: []api.NoteListItem{{Spans: []api.Span{}}}}
}

// meetingAgendaMinutes is the planned length of an agenda item the
// template proposes; the writer changes it with the ⏱ chip.
const meetingAgendaMinutes = 10

// meetingTemplateContent is Notulen rapat divisi: Agenda (numbered items
// with minutes, for the meeting timer), Pembahasan, Keputusan and Action
// item (a checklist, whose items take a PJ and a deadline and become
// tasks). agenda, when given, are the meeting's own items.
func meetingTemplateContent(agenda []string) []api.NoteBlock {
	if len(agenda) == 0 {
		agenda = []string{"", ""}
	}
	items := make([]api.NoteListItem, 0, len(agenda))
	for _, a := range agenda {
		spans := []api.Span{}
		if a != "" {
			spans = []api.Span{{Text: a}}
		}
		items = append(items, api.NoteListItem{Spans: spans, Minutes: meetingAgendaMinutes})
	}
	return []api.NoteBlock{
		heading(2, "Agenda"), {Type: api.BlockNumberedList, Items: items},
		heading(2, "Pembahasan"), emptyPara(),
		heading(2, "Keputusan"), bullets(),
		heading(2, "Action item"), checklist(),
	}
}

// meetingForNote loads a scheduled meeting a new note may be filled from:
// in the same space, not cancelled, and without a note yet.
func (s *Server) meetingForNote(ctx context.Context, q store.Querier, mid, sid id.ID) (api.Meeting, error) {
	m, err := s.store.Meeting(ctx, q, mid, " for update")
	if err != nil {
		return m, notFound(err, badRequest("Rapat tidak ditemukan."))
	}
	switch {
	case m.SpaceID != sid:
		return m, badRequest("Rapat itu bukan milik ruang ini.")
	case m.Cancelled:
		return m, badRequest("Rapat itu dibatalkan.")
	case m.NoteID != nil:
		return m, fail(409, api.CodeConflict, "Rapat itu sudah punya catatan.")
	}
	return m, nil
}

// meetingNoteTitle is "Notulen <meeting> — 17 Nov 2026".
func meetingNoteTitle(m api.Meeting) string {
	loc, err := time.LoadLocation(m.TimeZone)
	if err != nil {
		loc = time.UTC
	}
	t := m.StartsAt.In(loc)
	title := fmt.Sprintf("Notulen %s — %d %s %d", m.Title, t.Day(), publicMonths[t.Month()-1][:3], t.Year())
	if r := []rune(title); len(r) > 200 {
		title = string(r[:200])
	}
	return title
}

// meetingNoteContent fills the meeting template from the meeting: a line
// with when and where, and its description's lines as the agenda.
func meetingNoteContent(m api.Meeting) []api.NoteBlock {
	var agenda []string
	for _, line := range strings.Split(m.Description, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "-*•0123456789.)"))
		if line != "" && len(agenda) < 12 {
			agenda = append(agenda, line)
		}
	}
	loc, err := time.LoadLocation(m.TimeZone)
	if err != nil {
		loc = time.UTC
	}
	when := publicDate(m.StartsAt.In(loc))
	if m.Location != "" {
		when += " · " + m.Location
	}
	info := api.NoteBlock{Type: api.BlockParagraph, Spans: []api.Span{{Text: "Rapat: ", Bold: true}, {Text: when}}}
	return append([]api.NoteBlock{info}, meetingTemplateContent(agenda)...)
}
