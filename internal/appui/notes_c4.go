package appui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// Stage C4: mentions and action items.
//
// "@" in a paragraph, callout or list item opens a picker of the note
// division's members and of the Space's tasks; the choice becomes a mention
// span whose text is the name ("@Sinta Notulis", "#T22 Poster"), so old
// clients and the public view still read it. Text with mentions stays
// editable inline: an edit keeps every mention whose text is still there
// and turns a mention whose text was changed into plain text.
//
// A checklist item is an action item: PJ (assigneeId), tenggat (dueOn) and
// "Jadikan tugas", which opens the task form filled with them and, when the
// task is made, stores its id in taskId (shown as the chip #T22) beside the
// note link that conversion already makes.

// noteMentionTarget is one entry of the "@" picker.
type noteMentionTarget struct {
	kind   string // api.MentionUser or api.MentionTask
	id     id.ID
	text   string // the span's text
	detail string
}

type noteMentionMenu struct {
	open        bool
	block, item int
	query       string
	index       int
}

// notePick is the open PJ or tenggat picker of an action item.
type notePick struct {
	block, item int
	kind        string // notePickPJ or notePickDue
	query, date string
}

const (
	notePickPJ  = "pj"
	notePickDue = "due"
)

type noteMember struct {
	id   id.ID
	name string
}

// noteInlineSpans: spans the inline input may edit: plain text and
// mentions. Bold and links are edited in the block tools so they are never
// lost.
func noteInlineSpans(spans []api.Span) bool {
	for _, s := range spans {
		if s.Bold || s.Link != "" || (s.Mention != nil && s.Text == "") {
			return false
		}
	}
	return true
}

// noteRespan maps an edited text back onto spans: the mentions of old
// whose text is still in value, in order, with plain text between.
func noteRespan(old []api.Span, value string) []api.Span {
	out := []api.Span{}
	rest := value
	for _, s := range old {
		if s.Mention == nil || s.Text == "" {
			continue
		}
		i := strings.Index(rest, s.Text)
		if i < 0 {
			continue
		}
		if i > 0 {
			out = append(out, api.Span{Text: rest[:i]})
		}
		out = append(out, api.Span{Text: s.Text, Mention: clonePointer(s.Mention)})
		rest = rest[i+len(s.Text):]
	}
	if rest != "" || len(out) == 0 {
		out = append(out, api.Span{Text: rest})
	}
	return out
}

// noteMentionQuery is what follows an "@" typed at the end of the text
// (at the start or after a space), when that is not already a mention.
func noteMentionQuery(spans []api.Span) (string, bool) {
	if len(spans) == 0 || spans[len(spans)-1].Mention != nil {
		return "", false
	}
	text := spans[len(spans)-1].Text
	at := strings.LastIndex(text, "@")
	if at < 0 || (at > 0 && !strings.ContainsAny(text[at-1:at], " \t\n")) {
		return "", false
	}
	q := text[at+1:]
	if strings.ContainsAny(q, "\n@") || utf8.RuneCountInString(q) > 30 || strings.Contains(q, "  ") {
		return "", false
	}
	return q, true
}

// mentionSpans returns the spans being edited at block/item (-1: the
// block's own spans).
func (e *noteDocumentEditor) mentionSpans(block, item int) *[]api.Span {
	if block < 0 || block >= len(e.blocks) {
		return nil
	}
	b := &e.blocks[block]
	if item >= 0 {
		if item < len(b.Items) {
			return &b.Items[item].Spans
		}
		return nil
	}
	if b.Type == api.BlockParagraph || b.Type == api.BlockCallout {
		return &b.Spans
	}
	return nil
}

// afterInlineEdit opens or closes the "@" picker after an edit of block/
// item.
func (e *noteDocumentEditor) afterInlineEdit(block, item int) {
	spans := e.mentionSpans(block, item)
	if e.Mentions == nil || spans == nil {
		return
	}
	q, ok := noteMentionQuery(*spans)
	if ok && strings.HasSuffix(q, " ") && len(e.Mentions(q)) == 0 {
		ok = false
	}
	if ok {
		index := 0
		if e.mention.open && e.mention.block == block && e.mention.item == item {
			index = e.mention.index
		}
		e.mention = noteMentionMenu{open: true, block: block, item: item, query: q, index: index}
		e.menu.open = false
	} else if e.mention.block == block && e.mention.item == item {
		e.mention = noteMentionMenu{}
	}
}

// applyMention puts the chosen target in place of "@query".
func (e *noteDocumentEditor) applyMention(t noteMentionTarget) {
	m := e.mention
	e.mention = noteMentionMenu{}
	spans := e.mentionSpans(m.block, m.item)
	if spans == nil || len(*spans) == 0 {
		return
	}
	last := &(*spans)[len(*spans)-1]
	at := strings.LastIndex(last.Text, "@")
	if last.Mention != nil || at < 0 {
		return
	}
	last.Text = last.Text[:at]
	if last.Text == "" {
		*spans = (*spans)[:len(*spans)-1]
	}
	*spans = append(*spans, api.Span{Text: t.text, Mention: &api.Mention{Kind: t.kind, ID: t.id}}, api.Span{Text: " "})
	e.changed = true
	e.focusOn(m.block, m.item)
}

// mentionKey handles the picker's keys; it reports whether k was used.
func (e *noteDocumentEditor) mentionKey(index, item int, k ui.TextKey) bool {
	if !e.mention.open || e.mention.block != index || e.mention.item != item || e.Mentions == nil {
		return false
	}
	targets := e.Mentions(e.mention.query)
	switch k.Key {
	case ui.KeyDown:
		e.mention.index = min(e.mention.index+1, max(len(targets)-1, 0))
		return true
	case ui.KeyUp:
		e.mention.index = max(e.mention.index-1, 0)
		return true
	case ui.KeyEscape:
		e.mention.open = false
		return true
	case ui.KeyEnter, ui.KeyTab:
		if len(targets) == 0 {
			e.mention.open = false
			return k.Key == ui.KeyTab
		}
		e.applyMention(targets[min(e.mention.index, len(targets)-1)])
		return true
	}
	return false
}

// mentionMenu is the "@" picker under the block.
func (e *noteDocumentEditor) mentionMenu(c *ui.Context, anchor ui.Element) {
	t := c.Theme()
	targets := e.Mentions(e.mention.query)
	e.mention.index = min(e.mention.index, max(len(targets)-1, 0))
	ui.PopoverBase(c.Key("mention-menu"), anchor, &e.mention.open, func(panel ui.Element) {
		popPanel(panel, 0)
		panel.Margin(4, 0, 0, 0).Width(280).MaxHeight(320).Padding(6).Gap(2).Radius(10).Background(t.Surface).Border(1, t.Border).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, .18)).Label("Sebut anggota atau tugas")
		ui.Text(c, "SEBUT ANGGOTA ATAU TUGAS").FontSize(10).TextColor(faint(t)).Padding(4, 8)
		if len(targets) == 0 {
			muted(c, "Tidak ada anggota atau tugas bernama “"+e.mention.query+"”").FontSize(12).Padding(8)
			return
		}
		for i, target := range targets {
			b := ui.ButtonBase(c.Key(target.kind+target.id.String())).Label("Sebut "+target.text).Row().Justify(ui.Start).FillWidth().Height(32).Padding(0, 8).Gap(8).AlignItems(ui.Center).Radius(6)
			if i == e.mention.index || b.Hovered() {
				b.Background(t.SurfaceHover)
			}
			b.Children(func() {
				if target.kind == api.MentionUser {
					avatar(c, strings.TrimPrefix(target.text, "@"), 20)
				} else {
					ui.Box(c).Size(20, 20).Center().Children(func() { ui.Icon(c, spaceIcon("task")).Size(14, 14).TextColor(t.TextMuted) })
				}
				ui.Text(c, target.text).FontSize(13).MaxLines(1).MinWidth(0).Shrink(1)
				ui.Box(c).Grow(1)
				if target.detail != "" {
					ui.Text(c, target.detail).FontSize(11).TextColor(faint(t)).MaxLines(1).Shrink(0)
				}
			})
			if b.Clicked() {
				e.applyMention(target)
			}
		}
	})
}

// actionChips: PJ, tenggat and the task of a checklist item. An empty PJ
// or tenggat shows for the notulis only on the item's hover (or selection
// on a phone), so a plain checklist stays plain.
func (e *noteDocumentEditor) actionChips(c *ui.Context, index, i int, it api.NoteListItem, shown bool) {
	t := c.Theme()
	editable := e.editable && e.Members != nil
	if it.AssigneeID == nil && it.DueOn == nil && it.TaskID == nil && !(e.editable && shown) {
		return
	}
	chip := func(key, label, tip string, set bool, color ui.Color, enabled bool) ui.Element {
		b := ui.ButtonBase(c.Key(key)).Label(label).Tooltip(tip).Row().Height(24).Padding(0, 8).Gap(5).AlignItems(ui.Center).Radius(12).Shrink(0).TextColor(faint(t)).Disabled(!enabled)
		if set {
			b.Background(spaceMix(color, t.Surface, .88)).TextColor(color)
		} else {
			b.Border(1, t.Border)
		}
		if enabled && b.Hovered() {
			b.Background(t.SurfaceHover)
		}
		b.FocusRing(b.FocusVisible())
		return b
	}
	ui.Row(c.Key("action-chips")).Shrink(0).Gap(4).AlignItems(ui.Center).Padding(2, 0, 0, 0).Children(func() {
		if it.AssigneeID != nil || (editable && shown) {
			name := ""
			if it.AssigneeID != nil && e.MemberName != nil {
				name = e.MemberName(*it.AssigneeID)
			}
			if it.AssigneeID != nil && name == "" {
				name = "PJ"
			}
			b := chip("pj-chip", fmt.Sprintf("PJ butir %d blok %d", i+1, index+1), "Penanggung jawab", it.AssigneeID != nil, t.Accent, editable)
			b.Children(func() {
				if it.AssigneeID != nil {
					avatar(c, name, 16)
					ui.Text(c, firstName(name)).FontSize(12).MaxLines(1)
				} else {
					ui.Icon(c, spaceIcon("user")).Size(12, 12)
					ui.Text(c, "PJ").FontSize(12)
				}
			})
			if editable && b.Clicked() {
				e.togglePick(index, i, notePickPJ)
			}
			if editable && e.pick.kind == notePickPJ && e.pick.block == index && e.pick.item == i {
				e.pjPicker(c, b, index, i)
			}
		}
		if it.DueOn != nil || (e.editable && shown) {
			color := t.Accent
			text := "Tenggat"
			if it.DueOn != nil {
				d := *it.DueOn
				text = fmt.Sprintf("%d %s", d.Day, noteMonths[d.Month-1])
				if e.Today != nil && !it.Checked && d.Before(e.Today()) {
					color = t.Danger
				}
			}
			b := chip("due-chip", fmt.Sprintf("Tenggat butir %d blok %d", i+1, index+1), "Tenggat action item", it.DueOn != nil, color, e.editable)
			b.Children(func() {
				ui.Icon(c, spaceIcon("calendar")).Size(12, 12)
				ui.Text(c, text).FontSize(12).MaxLines(1)
			})
			if e.editable && b.Clicked() {
				e.togglePick(index, i, notePickDue)
			}
			if e.editable && e.pick.kind == notePickDue && e.pick.block == index && e.pick.item == i {
				e.duePicker(c, b, index, i)
			}
		}
		switch {
		case it.TaskID != nil:
			label := "Tugas"
			if e.TaskLabel != nil {
				label = e.TaskLabel(*it.TaskID)
			}
			b := chip("task-chip", "Buka tugas "+label, "Buka tugas dari action item ini", true, t.Success, e.OpenTask != nil)
			b.Children(func() {
				ui.Icon(c, spaceIcon("task")).Size(12, 12)
				ui.Text(c, label).FontSize(12).Bold().MaxLines(1)
			})
			if b.Clicked() && e.OpenTask != nil {
				e.OpenTask(*it.TaskID)
			}
		case e.editable && e.ExtractTask && e.OnActionTask != nil && strings.TrimSpace(noteSpansText(it.Spans)) != "" && (shown || it.AssigneeID != nil):
			b := ui.ButtonBase(c.Key("make-task")).Label(fmt.Sprintf("Jadikan tugas butir %d blok %d", i+1, index+1)).Row().Height(24).Padding(0, 8).Gap(5).AlignItems(ui.Center).Radius(12).Shrink(0).TextColor(t.Accent)
			if b.Hovered() {
				b.Background(accentSoft(t))
			}
			b.FocusRing(b.FocusVisible()).Children(func() {
				ui.Icon(c, spaceIcon("plus")).Size(12, 12)
				ui.Text(c, "Jadikan tugas").FontSize(12).MaxLines(1)
			})
			if b.Clicked() {
				e.OnActionTask(index, i, it)
			}
		}
	})
}

func (e *noteDocumentEditor) togglePick(block, item int, kind string) {
	if e.pick.kind == kind && e.pick.block == block && e.pick.item == item {
		e.pick = notePick{}
		return
	}
	e.pick = notePick{block: block, item: item, kind: kind}
	e.selectBlock(block)
	e.item = item
}

func (e *noteDocumentEditor) actionItem(block, item int) *api.NoteListItem {
	if block < 0 || block >= len(e.blocks) || e.blocks[block].Type != api.BlockChecklist || item < 0 || item >= len(e.blocks[block].Items) {
		return nil
	}
	return &e.blocks[block].Items[item]
}

func (e *noteDocumentEditor) pjPicker(c *ui.Context, anchor ui.Element, block, item int) {
	t := c.Theme()
	open := true
	popover(c, anchor, &open, 260, func() {
		ui.Text(c, "Penanggung jawab").FontSize(12).Bold()
		ui.TextInput(c.Key("pj-query"), &e.pick.query).FillWidth().Label("Cari PJ").Placeholder("Cari anggota…")
		members := e.Members()
		q := strings.ToLower(strings.TrimSpace(e.pick.query))
		ui.Scroll(c).FillWidth().MaxHeight(220).Children(func() {
			ui.Column(c).FillWidth().MinWidth(0).Gap(2).Children(func() {
				if members == nil {
					muted(c, "Memuat anggota divisi…").FontSize(12)
					return
				}
				shown := 0
				for _, m := range members {
					if q != "" && !strings.Contains(strings.ToLower(m.name), q) {
						continue
					}
					shown++
					b := ui.ButtonBase(c.Key(m.id)).Label("Pilih PJ "+m.name).Row().Justify(ui.Start).FillWidth().Height(32).Padding(0, 6).Gap(8).AlignItems(ui.Center).Radius(6)
					if b.Hovered() {
						b.Background(t.SurfaceHover)
					}
					b.Children(func() {
						avatar(c, m.name, 20)
						ui.Text(c, m.name).FontSize(13).MaxLines(1).MinWidth(0)
					})
					if b.Clicked() {
						if it := e.actionItem(block, item); it != nil {
							uid := m.id
							it.AssigneeID, e.changed = &uid, true
						}
						e.pick = notePick{}
					}
				}
				if shown == 0 {
					muted(c, "Tidak ada anggota yang cocok.").FontSize(12)
				}
			})
		})
		if it := e.actionItem(block, item); it != nil && it.AssigneeID != nil {
			if spaceButton(c, "Hapus PJ", "close", false).FillWidth().Clicked() {
				it.AssigneeID, e.changed = nil, true
				e.pick = notePick{}
			}
		}
	})
	if !open {
		e.pick = notePick{}
	}
}

func (e *noteDocumentEditor) duePicker(c *ui.Context, anchor ui.Element, block, item int) {
	open := true
	set := func(d *timeutil.Date) {
		if it := e.actionItem(block, item); it != nil {
			it.DueOn, e.changed = d, true
		}
		e.pick = notePick{}
	}
	popover(c, anchor, &open, 260, func() {
		ui.Text(c, "Tenggat").FontSize(12).Bold()
		if e.Today != nil {
			today := e.Today()
			ui.Row(c).FillWidth().Wrap().Gap(5).Children(func() {
				for i, p := range []struct {
					label string
					days  int
				}{{"Hari ini", 0}, {"Besok", 1}, {"Lusa", 2}, {"Minggu depan", 7}} {
					if spaceButton(c.Key(i), p.label, "", false).Clicked() {
						d := today.AddDays(p.days)
						set(&d)
					}
				}
			})
		}
		ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
			ui.TextInput(c.Key("due-date"), &e.pick.date).Grow(1).MinWidth(0).Label("Tanggal tenggat").Placeholder("2026-11-20")
			if spaceButton(c, "Pakai tanggal", "check", true).Clicked() {
				if d, err := timeutil.ParseDate(strings.TrimSpace(e.pick.date)); err == nil {
					set(&d)
				}
			}
		})
		if it := e.actionItem(block, item); it != nil && it.DueOn != nil {
			if spaceButton(c, "Hapus tenggat", "close", false).FillWidth().Clicked() {
				set(nil)
			}
		}
	})
	if !open {
		e.pick = notePick{}
	}
}

// SetItemTask stores the task made from an action item. The item is found
// by its place and text, else by its text among the items without a task,
// since the notulis may have moved it while the task was being made.
func (e *noteDocumentEditor) SetItemTask(block, item int, text string, task id.ID) bool {
	if !e.editable || task == id.Nil {
		return false
	}
	match := func(it *api.NoteListItem) bool {
		return it != nil && it.TaskID == nil && noteSpansText(it.Spans) == text
	}
	it := e.actionItem(block, item)
	if !match(it) {
		it = nil
	search:
		for b := range e.blocks {
			for i := range e.blocks[b].Items {
				if cand := e.actionItem(b, i); match(cand) {
					it = cand
					break search
				}
			}
		}
	}
	if it == nil {
		return false
	}
	tid := task
	it.TaskID, e.changed = &tid, true
	return true
}
