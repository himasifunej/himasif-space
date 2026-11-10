package appui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
)

// Gaya C, stage G3: the notulis switches between Edit and Baca (the same
// document without handles, menus or inputs, the editor lock kept), and
// the editor keeps an undo history of the document.
//
// Undo works on the whole document: typing in one block within
// noteUndoQuiet of the previous change is one step, a structural change
// (a new, moved or removed block) is a step of its own. Ctrl+Z goes to the
// document before the text field sees it, so there is one history, and an
// undo is an ordinary change that autosave writes.

const (
	noteUndoQuiet = 1200 * time.Millisecond
	noteUndoMax   = 100
)

type noteUndoStep struct {
	blocks   []api.NoteBlock
	selected int
}

type noteUndoHistory struct {
	undo, redo []noteUndoStep
	// base is the document after the last change seen; the next step
	// starts from it.
	base     []api.NoteBlock
	baseSel  int
	baseSet  bool
	lastAt   time.Time
	group    string // what the open step is typing in ("" = none open)
	restored bool   // the change seen next is an undo or a redo itself
}

// noteUndoGroup names what a change touched, so typing on in one block
// stays one step: the block and its length of items, rows and children.
func noteUndoGroup(before, after []api.NoteBlock) string {
	if len(before) != len(after) {
		return ""
	}
	changed := -1
	for i := range after {
		b, a := before[i], after[i]
		if b.Type != a.Type || len(b.Items) != len(a.Items) || len(b.Rows) != len(a.Rows) || len(b.Children) != len(a.Children) || len(b.Columns) != len(a.Columns) {
			return ""
		}
		if noteBlockText(b) != noteBlockText(a) || b.Level != a.Level {
			if changed >= 0 {
				return ""
			}
			changed = i
		}
	}
	if changed < 0 {
		return "" // a check box, a PJ, a duration: a step of its own
	}
	return fmt.Sprint("block ", changed)
}

// track records a change of the document (called when the editor reports
// one, at now).
func (e *noteDocumentEditor) track(now time.Time) {
	h := &e.history
	if !h.baseSet {
		h.base, h.baseSel, h.baseSet = cloneNoteDocument(e.blocks), e.selected, true
		return
	}
	if h.restored {
		h.restored = false
	} else {
		group := noteUndoGroup(h.base, e.blocks)
		if group == "" || group != h.group || now.Sub(h.lastAt) > noteUndoQuiet {
			h.undo = append(h.undo, noteUndoStep{blocks: h.base, selected: h.baseSel})
			if len(h.undo) > noteUndoMax {
				h.undo = h.undo[len(h.undo)-noteUndoMax:]
			}
		}
		h.redo = nil
		h.group, h.lastAt = group, now
	}
	h.base, h.baseSel = cloneNoteDocument(e.blocks), e.selected
}

// undoKey takes Ctrl+Z, Ctrl+Shift+Z and Ctrl+Y from a block's text
// field (a field would undo only its own text), so the document has one
// history. It runs between frames.
func (e *noteDocumentEditor) undoKey(k ui.TextKey) bool {
	if !e.editable {
		return false
	}
	switch {
	case k.Key == ui.KeyZ && (k.Mods == ui.Ctrl || k.Mods == ui.Cmd):
		e.Undo()
		return true
	case k.Key == ui.KeyZ && (k.Mods == ui.Ctrl|ui.Shift || k.Mods == ui.Cmd|ui.Shift), k.Key == ui.KeyY && k.Mods == ui.Ctrl:
		e.Redo()
		return true
	}
	return false
}

// startHistory makes the current document the start of the history.
func (e *noteDocumentEditor) startHistory() {
	e.history = noteUndoHistory{base: cloneNoteDocument(e.blocks), baseSel: e.selected, baseSet: true}
}

func (e *noteDocumentEditor) CanUndo() bool { return e.editable && len(e.history.undo) > 0 }
func (e *noteDocumentEditor) CanRedo() bool { return e.editable && len(e.history.redo) > 0 }

// Undo puts back the document as it was before the last step.
func (e *noteDocumentEditor) Undo() bool {
	h := &e.history
	if !e.editable || len(h.undo) == 0 {
		return false
	}
	step := h.undo[len(h.undo)-1]
	h.undo = h.undo[:len(h.undo)-1]
	h.redo = append(h.redo, noteUndoStep{blocks: cloneNoteDocument(e.blocks), selected: e.selected})
	e.restore(step)
	return true
}

// Redo takes back an undo.
func (e *noteDocumentEditor) Redo() bool {
	h := &e.history
	if !e.editable || len(h.redo) == 0 {
		return false
	}
	step := h.redo[len(h.redo)-1]
	h.redo = h.redo[:len(h.redo)-1]
	h.undo = append(h.undo, noteUndoStep{blocks: cloneNoteDocument(e.blocks), selected: e.selected})
	e.restore(step)
	return true
}

func (e *noteDocumentEditor) restore(step noteUndoStep) {
	e.blocks = cloneNoteDocument(step.blocks)
	e.pages = map[int]*noteDocumentPage{}
	e.closed, e.menu, e.mention = nil, noteBlockMenu{}, noteMentionMenu{}
	e.selected = -1
	if len(e.blocks) > 0 {
		e.selectBlock(max(0, min(step.selected, len(e.blocks)-1)))
	}
	e.history.group, e.history.restored = "", true
	e.changed = true
}
