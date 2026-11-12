package appui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
)

// Stage C3: Notion-like block interactions (canvas 8eef2f092aab, designs 1
// and 2). Each block has a "+" and a "⠇" handle in its gutter, shown on
// hover (on a phone: on the selected block). "+" adds a block below and
// opens the block menu; "⠇" drags the block to another place, and a click
// opens the block tools (move, delete, format, make a task or reminder).
// Typing "/" at the start of a text block opens the same menu, Notulen
// first. Enter makes the next block or item, Backspace on an empty one
// removes it, and the focus follows.
//
// Spike (MyGo v0.7.1): Hovered on a row, Drag/Drop[T]/DragOver, Focus and
// OnTextKey all work; there is no on-screen caret position nor keyboard
// height, so the menu sits under the block being typed in, and the phone's
// block bar sits at the bottom of the note (above the keyboard where the
// viewport shrinks for it, as in browsers).

// noteFocus is where the focus goes in the next frame: a block's input, or
// item of a list (-1: the block's own input).
type noteFocus struct {
	block, item int
	set         bool
}

type noteBlockMenu struct {
	open    bool
	block   int  // block the menu acts on
	replace bool // replace that block (its "/" text or the new empty line)
	query   string
	index   int // highlighted entry among the filtered ones
}

// noteBlockDrag is what a "⠇" handle carries; editor keeps a drop to the
// document it came from.
type noteBlockDrag struct {
	editor *noteDocumentEditor
	from   int
}

type noteMenuEntry struct {
	key, label, hint, icon, glyph, group, words string
	// short is the shortcut shown on the right of the menu: a Markdown
	// marker typed at the start of an empty line, or a "/" filter word.
	short string
	make  func(e *noteDocumentEditor) []api.NoteBlock
}

const (
	noteMenuNotulen = "Notulen"
	noteMenuBasic   = "Blok dasar"
)

func noteEmptyItem() api.NoteListItem { return api.NoteListItem{Spans: []api.Span{{}}} }

// noteMenuEntries: the Notulen group (meeting building blocks) above the
// basic blocks.
func noteMenuEntries() []noteMenuEntry {
	one := func(b api.NoteBlock) func(*noteDocumentEditor) []api.NoteBlock {
		return func(*noteDocumentEditor) []api.NoteBlock { return []api.NoteBlock{b} }
	}
	list := func(kind string) api.NoteBlock {
		return api.NoteBlock{Type: kind, Items: []api.NoteListItem{noteEmptyItem()}}
	}
	return []noteMenuEntry{
		{key: "action", short: "/tugas", label: "Action item", hint: "Checklist tugas hasil rapat", icon: "task", group: noteMenuNotulen, words: "tugas todo pj checklist", make: one(list(api.BlockChecklist))},
		{key: "decision", short: "/kep", label: "Keputusan", hint: "Kotak hijau untuk hasil rapat", icon: "check", group: noteMenuNotulen, words: "callout putusan hasil", make: one(api.NoteBlock{Type: api.BlockCallout, Variant: api.CalloutDecision, Spans: []api.Span{{}}})},
		{key: "attendance", short: "/hadir", label: "Kehadiran", hint: "Hadir, izin, absen peserta", icon: "users", group: noteMenuNotulen, words: "hadir izin absen peserta presensi", make: func(e *noteDocumentEditor) []api.NoteBlock {
			items := []api.NoteListItem{}
			if e.Attendance != nil {
				for _, line := range e.Attendance() {
					items = append(items, api.NoteListItem{Spans: []api.Span{{Text: line}}})
				}
			}
			if len(items) == 0 {
				items = append(items, noteEmptyItem())
			}
			return []api.NoteBlock{{Type: api.BlockHeading, Level: 3, Text: "Kehadiran"}, {Type: api.BlockBulletedList, Items: items}}
		}},
		{key: "text", label: "Teks", hint: "Paragraf biasa", glyph: "¶", group: noteMenuBasic, words: "paragraf tulisan", make: one(api.NoteBlock{Type: api.BlockParagraph, Spans: []api.Span{{}}})},
		{key: "h1", short: "#", label: "Judul 1", hint: "Judul terbesar", glyph: "H1", group: noteMenuBasic, words: "heading judul h1", make: one(api.NoteBlock{Type: api.BlockHeading, Level: 1})},
		{key: "h2", short: "##", label: "Judul 2", hint: "Judul bagian", glyph: "H2", group: noteMenuBasic, words: "heading judul bagian h2", make: one(api.NoteBlock{Type: api.BlockHeading, Level: 2})},
		{key: "h3", short: "###", label: "Judul 3", hint: "Subjudul", glyph: "H3", group: noteMenuBasic, words: "heading subjudul judul h3", make: one(api.NoteBlock{Type: api.BlockHeading, Level: 3})},
		{key: "bullet", short: "-", label: "Daftar poin", hint: "Daftar dengan titik", glyph: "•", group: noteMenuBasic, words: "list bullet poin", make: one(list(api.BlockBulletedList))},
		{key: "numbered", short: "1.", label: "Daftar bernomor", hint: "Agenda, urutan", glyph: "1.", group: noteMenuBasic, words: "list nomor agenda urutan", make: one(list(api.BlockNumberedList))},
		{key: "checklist", short: "[]", label: "Checklist", hint: "Daftar centang", glyph: "☐", group: noteMenuBasic, words: "todo centang", make: one(list(api.BlockChecklist))},
		{key: "toggle", label: "Toggle", hint: "Judul yang bisa dilipat", glyph: "▸", group: noteMenuBasic, words: "lipat buka tutup", make: one(api.NoteBlock{Type: api.BlockToggle, Children: []api.NoteBlock{{Type: api.BlockParagraph, Spans: []api.Span{{}}}}})},
		{key: "callout", label: "Callout", hint: "Catatan penting berlatar", glyph: "!", group: noteMenuBasic, words: "info penting sorot", make: one(api.NoteBlock{Type: api.BlockCallout, Variant: api.CalloutInfo, Spans: []api.Span{{}}})},
		{key: "table", label: "Tabel", hint: "Jadwal dalam baris dan kolom", glyph: "⊞", group: noteMenuBasic, words: "jadwal tabel kolom", make: func(*noteDocumentEditor) []api.NoteBlock {
			columns := []string{"Tanggal", "Jam", "Kegiatan", "PJ", "Keterangan"}
			return []api.NoteBlock{{Type: api.BlockTable, Columns: columns, Rows: [][][]api.Span{noteEmptyRow(len(columns))}}}
		}},
		{key: "divider", short: "---", label: "Pemisah", hint: "Garis antar bagian", glyph: "—", group: noteMenuBasic, words: "garis divider", make: one(api.NoteBlock{Type: api.BlockDivider})},
	}
}

// noteMenuMatches filters the menu by what follows "/".
func noteMenuMatches(query string) []noteMenuEntry {
	q := strings.ToLower(strings.TrimSpace(query))
	out := []noteMenuEntry{}
	for _, m := range noteMenuEntries() {
		if q == "" || strings.Contains(strings.ToLower(m.label+" "+m.words+" "+m.key), q) {
			out = append(out, m)
		}
	}
	return out
}

// noteSaveContent is the document as saved: a heading still being named
// (empty) goes as an empty paragraph, which the server accepts, so autosave
// is not refused while the notulis has only picked "Judul bagian".
func noteSaveContent(content []api.NoteBlock) []api.NoteBlock {
	for i, b := range content {
		if b.Type == api.BlockHeading && b.Text == "" {
			content[i] = api.NoteBlock{Type: api.BlockParagraph, Spans: []api.Span{{}}}
		}
	}
	return content
}

func (e *noteDocumentEditor) focusOn(block, item int) {
	if block < 0 || block >= len(e.blocks) {
		return
	}
	e.focus = noteFocus{block: block, item: item, set: true}
	e.blockPage = block / noteBlocksPerPage
	if item >= 0 {
		e.page(block).items = item / noteItemsPerPage
	}
}

// takeFocus reports (once) whether the input of block/item should take the
// focus in this frame.
func (e *noteDocumentEditor) takeFocus(block, item int) bool {
	if e.focus.set && e.focus.block == block && e.focus.item == item {
		e.focus.set = false
		return true
	}
	return false
}

// insertBlocks puts blocks after index (or replaces it), within the limit.
func (e *noteDocumentEditor) insertBlocks(index int, replace bool, blocks []api.NoteBlock) int {
	if !e.editable || len(blocks) == 0 {
		return -1
	}
	if replace && index >= 0 && index < len(e.blocks) {
		if len(e.blocks)-1+len(blocks) > api.NoteMaxBlocks {
			return -1
		}
		e.blocks = slices.Replace(e.blocks, index, index+1, blocks...)
	} else {
		if len(e.blocks)+len(blocks) > api.NoteMaxBlocks {
			return -1
		}
		index = min(max(index+1, 0), len(e.blocks))
		e.blocks = slices.Insert(e.blocks, index, blocks...)
	}
	e.pages = map[int]*noteDocumentPage{}
	e.closed = nil
	e.selectBlock(index)
	e.changed = true
	return index
}

// openMenuBelow is "+": an empty line under the block, with the menu on it.
func (e *noteDocumentEditor) openMenuBelow(index int) {
	at := e.insertBlocks(index, false, []api.NoteBlock{{Type: api.BlockParagraph, Spans: []api.Span{{}}}})
	if at < 0 {
		return
	}
	e.menu = noteBlockMenu{open: true, block: at, replace: true}
	e.focusOn(at, -1)
}

// applyMenu turns the menu's block into the chosen one (or adds it below).
func (e *noteDocumentEditor) applyMenu(m noteMenuEntry) {
	menu := e.menu
	e.menu = noteBlockMenu{}
	at := e.insertBlocks(menu.block, menu.replace, m.make(e))
	if at < 0 {
		return
	}
	switch b := e.blocks[at]; b.Type {
	case api.BlockBulletedList, api.BlockNumberedList, api.BlockChecklist:
		e.focusOn(at, len(b.Items)-1)
	case api.BlockDivider, api.BlockTable:
		// Nothing to type in: a fresh line follows, as in Notion.
		if b.Type == api.BlockDivider && at+1 >= len(e.blocks) {
			e.insertBlocks(at, false, []api.NoteBlock{{Type: api.BlockParagraph, Spans: []api.Span{{}}}})
			e.focusOn(at+1, -1)
		}
	default:
		if m.key == "attendance" && at+1 < len(e.blocks) {
			e.focusOn(at+1, 0)
		} else {
			e.focusOn(at, -1)
		}
	}
}

// moveBlockTo moves a block to where another one is (a drop on it).
func (e *noteDocumentEditor) moveBlockTo(from, to int) {
	if !e.editable || from == to || from < 0 || to < 0 || from >= len(e.blocks) || to >= len(e.blocks) {
		return
	}
	b := e.blocks[from]
	e.blocks = slices.Delete(e.blocks, from, from+1)
	e.blocks = slices.Insert(e.blocks, to, b)
	e.pages = map[int]*noteDocumentPage{}
	e.closed = nil
	e.selectBlock(to)
	e.changed = true
}

// textKey handles Enter, Backspace and the menu's keys in a block's text
// (item -1) or a list item. It runs between frames, with k's text current.
func (e *noteDocumentEditor) textKey(index, item int, k ui.TextKey) bool {
	if !e.editable || index >= len(e.blocks) {
		return false
	}
	if e.undoKey(k) || e.mentionKey(index, item, k) {
		return true
	}
	if e.menu.open && e.menu.block == index && item < 0 {
		matches := noteMenuMatches(e.menu.query)
		switch k.Key {
		case ui.KeyDown:
			e.menu.index = min(e.menu.index+1, max(len(matches)-1, 0))
			return true
		case ui.KeyUp:
			e.menu.index = max(e.menu.index-1, 0)
			return true
		case ui.KeyEscape:
			e.menu.open = false
			return true
		case ui.KeyEnter:
			if len(matches) > 0 {
				e.applyMenu(matches[min(e.menu.index, len(matches)-1)])
			} else {
				e.menu.open = false
			}
			return true
		}
	}
	empty := k.Text() == ""
	b := &e.blocks[index]
	switch {
	case k.Key == ui.KeyEnter && k.Mods == 0 && item >= 0:
		if empty && item == len(b.Items)-1 {
			// Enter on the last, empty item leaves the list.
			if len(b.Items) > 1 {
				b.Items = b.Items[:item]
			}
			at := e.insertBlocks(index, false, []api.NoteBlock{{Type: api.BlockParagraph, Spans: []api.Span{{}}}})
			e.focusOn(at, -1)
			return true
		}
		b.Items = slices.Insert(b.Items, item+1, noteEmptyItem())
		e.changed = true
		e.focusOn(index, item+1)
		return true
	case k.Key == ui.KeyEnter && k.Mods == 0 && (b.Type == api.BlockParagraph || b.Type == api.BlockHeading):
		// Shift+Enter breaks the line in a paragraph; Enter starts a block.
		at := e.insertBlocks(index, false, []api.NoteBlock{{Type: api.BlockParagraph, Spans: []api.Span{{}}}})
		e.focusOn(at, -1)
		return true
	case k.Key == ui.KeyBackspace && empty && item >= 0:
		if len(b.Items) > 1 {
			b.Items = slices.Delete(b.Items, item, item+1)
			e.changed = true
			e.focusOn(index, max(item-1, 0))
			return true
		}
		// The only item: the list becomes an empty line.
		e.blocks[index] = api.NoteBlock{Type: api.BlockParagraph, Spans: []api.Span{{}}}
		e.pages, e.changed = map[int]*noteDocumentPage{}, true
		e.focusOn(index, -1)
		return true
	case k.Key == ui.KeyBackspace && empty && item < 0 && index > 0 && (b.Type == api.BlockParagraph || b.Type == api.BlockHeading):
		e.blocks = slices.Delete(e.blocks, index, index+1)
		e.pages, e.changed, e.closed = map[int]*noteDocumentPage{}, true, nil
		e.menu = noteBlockMenu{}
		e.selectBlock(index - 1)
		if prev := e.blocks[index-1]; len(prev.Items) > 0 {
			e.focusOn(index-1, len(prev.Items)-1)
		} else {
			e.focusOn(index-1, -1)
		}
		return true
	}
	return false
}

// View draws the document: the blocks of the current page, each with its
// handles, and a line to add a block at the end.
func (e *noteDocumentEditor) View(c *ui.Context) {
	outer := ui.Column(c.Key(e)).FillWidth().MinWidth(0)
	outer.Children(func() { e.documentView(c, outer) })
}

func (e *noteDocumentEditor) documentView(c *ui.Context, outer ui.Element) {
	view := ui.Column(c.Key("doc")).FillWidth().MinWidth(0).Gap(2)
	if e.editable && !e.Compact {
		// The handles hang left of the text column, as in Notion: the
		// rows are 52 px wider than the column and start 52 px before it.
		if w := outer.Bounds().W; w > 0 {
			view.Width(w + 52)
		}
		view.Margin(0, 0, 0, -52)
	}
	view.Children(func() {
		if noteDocumentPager(c, "Blok", &e.blockPage, len(e.blocks), noteBlocksPerPage) && len(e.blocks) > 0 {
			e.selectBlock(e.blockPage * noteBlocksPerPage)
		}
		if len(e.blocks) == 0 {
			if e.editable {
				if spaceButton(c, "Mulai menulis", "plus", false).Clicked() {
					e.openMenuBelow(-1)
				}
			} else {
				muted(c, "Catatan belum memiliki isi.")
			}
			return
		}
		start, end := e.blockPage*noteBlocksPerPage, min((e.blockPage+1)*noteBlocksPerPage, len(e.blocks))
		for i := start; i < end && i < len(e.blocks); i++ {
			e.blockRow(c.Key(i), i)
		}
		if e.editable && end >= len(e.blocks) {
			t := c.Theme()
			b := ui.ButtonBase(c.Key("add-end")).Label("Tambah blok").Row().Justify(ui.Start).FillWidth().MinHeight(36).Padding(0, 8, 0, 52).Gap(8).AlignItems(ui.Center).Radius(6).TextColor(faint(t)).Disabled(len(e.blocks) >= api.NoteMaxBlocks)
			if b.Hovered() {
				b.Background(t.SurfaceHover)
			}
			b.FocusRing(b.FocusVisible()).Children(func() {
				ui.Icon(c, spaceIcon("plus")).Size(14, 14)
				ui.Text(c, "Klik untuk menambah blok, atau ketik / di baris kosong").FontSize(13).MaxLines(1).MinWidth(0)
			})
			if b.Clicked() {
				e.openMenuBelow(len(e.blocks) - 1)
			}
		}
	})
}

// noteDocumentPager pages long documents, lists and tables; it shows only
// when there is more than one page.
func noteDocumentPager(c *ui.Context, label string, page *int, length, size int) bool {
	last := max(0, (length-1)/size)
	*page = max(0, min(*page, last))
	if last == 0 {
		return false
	}
	changed := false
	noun := strings.Fields(label)[0]
	ui.Row(c).FillWidth().MinWidth(0).Wrap().Gap(6).AlignItems(ui.Center).Padding(4, 0).Children(func() {
		if spaceIconButton(c, label+" sebelumnya", "left").Disabled(*page == 0).Clicked() {
			*page--
			changed = true
		}
		muted(c, fmt.Sprintf("%s %d–%d dari %d", noun, *page*size+1, min((*page+1)*size, length), length)).FontSize(11)
		if spaceIconButton(c, label+" berikutnya", "right").Disabled(*page >= last).Clicked() {
			*page++
			changed = true
		}
	})
	return changed
}

// blockRow is one block with its gutter: "+" and "⠇" for editors, shown on
// hover or (phone) on the selected block; a drop target for dragged blocks.
func (e *noteDocumentEditor) blockRow(c *ui.Context, index int) {
	t := c.Theme()
	b := e.blocks[index]
	row := ui.Row(c).FillWidth().MinWidth(0).Gap(4).AlignItems(ui.Start).Radius(6)
	hovered := row.Hovered()
	if d, ok := ui.Drop[noteBlockDrag](row); ok && d.editor == e {
		e.moveBlockTo(d.from, index)
	}
	_, over := ui.DragOver[noteBlockDrag](row)
	if over {
		row.BorderWidth(2, 0, 0, 0).BorderColor(t.Accent)
	}
	shown := hovered || (e.selected == index && e.advanced)
	if e.Compact && !e.editable {
		shown = e.reveal == index+1
	}
	row.Children(func() {
		if e.editable && !e.Compact {
			// A phone has no hover and little width: no gutter; the block bar
			// adds, moves and opens the block tools instead.
			ui.Row(c.Key("gutter")).Width(48).Shrink(0).Gap(0).Padding(4, 0, 0, 0).Opacity(noteShown(shown)).Children(func() {
				plus := noteHandle(c.Key("plus"), fmt.Sprintf("Tambah blok di bawah blok %d", index+1), "plus").Disabled(len(e.blocks) >= api.NoteMaxBlocks)
				if plus.Clicked() {
					e.openMenuBelow(index)
				}
				grip := noteHandle(c.Key("grip"), fmt.Sprintf("Atur blok %d", index+1), "grip").Tooltip("Seret untuk memindah, klik untuk pilihan")
				grip.Drag(noteBlockDrag{editor: e, from: index})
				if grip.Clicked() {
					if e.selected == index {
						e.advanced = !e.advanced
					} else {
						e.selectBlock(index)
						e.advanced = true
					}
				}
			})
		}
		ui.Column(c.Key("content")).Grow(1).MinWidth(0).Gap(6).Padding(2, 0).Children(func() {
			anchor := e.blockContent(c, index, b, shown)
			if e.editable && e.menu.open && e.menu.block == index {
				e.blockMenu(c, anchor)
			}
			if e.editable && e.Mentions != nil && e.mention.open && e.mention.block == index {
				e.mentionMenu(c, anchor)
			}
			if e.editable && e.selected == index && e.advanced {
				if e.OnExtract != nil && b.Type != api.BlockDivider {
					e.extractView(c, noteBlockText(b), fmt.Sprintf("blok %d", index+1), true)
				}
				e.blockEditor(c)
			}
		})
		if !e.editable && e.OnExtract != nil && b.Type != api.BlockDivider {
			// Readers turn a whole block into a reminder (or a task): on
			// hover, or on a phone from the block's "⋯".
			ui.Row(c.Key("reader-tools")).Shrink(0).Gap(0).AlignItems(ui.Center).Children(func() {
				if e.Compact {
					if noteHandle(c.Key("more"), fmt.Sprintf("Pilihan blok %d", index+1), "more").Clicked() {
						if e.reveal == index+1 {
							e.reveal = 0
						} else {
							e.reveal = index + 1
						}
					}
				}
				// Lists and tables show their own per item or row on hover.
				whole := shown && (e.reveal == index+1 || (len(b.Items) == 0 && len(b.Rows) == 0))
				e.extractView(c, noteBlockText(b), fmt.Sprintf("blok %d", index+1), whole)
			})
		}
	})
}

func noteShown(shown bool) float32 {
	if shown {
		return 1
	}
	return 0
}

// noteHandle is a small gutter button (24 px; the row keeps 44 px tap
// height on a phone through its padding).
func noteHandle(c *ui.Context, label, icon string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Tooltip(label).Size(24, 28).Center().Radius(6).TextColor(faint(t))
	if b.Hovered() {
		b.Background(t.SurfaceHover).TextColor(t.Text)
	}
	b.FocusRing(b.FocusVisible())
	b.Children(func() { ui.Icon(c, spaceIcon(icon)).Size(16, 16) })
	return b
}

// blockContent draws the block itself and returns the element the "/" menu
// hangs under.
func (e *noteDocumentEditor) blockContent(c *ui.Context, index int, b api.NoteBlock, shown bool) ui.Element {
	t := c.Theme()
	box := ui.Column(c.Key("block")).FillWidth().MinWidth(0).Gap(6)
	box.Children(func() {
		switch b.Type {
		case api.BlockHeading:
			size := float32(28 - max(1, min(b.Level, 3))*4)
			if e.editable {
				in := ui.TextInput(c.Key("inline-heading"), &e.blocks[index].Text).Label(fmt.Sprintf("Judul bagian %d", index+1)).Placeholder("Judul bagian").FillWidth().FontSize(size).Bold().Background(ui.RGBA(0, 0, 0, 0)).BorderWidth(0).Padding(4, 0)
				in.OnTextKey(func(k ui.TextKey) bool { return e.textKey(index, -1, k) })
				if e.focusBlock == index+1 || e.takeFocus(index, -1) {
					in.Focus()
					e.focusBlock = 0
				}
				if in.Focused() {
					e.selectBlock(index)
				}
				if in.Changed() {
					e.changed = true
				}
			} else {
				ui.Text(c, b.Text).FontSize(size).Bold().FillWidth().MinWidth(0).Padding(4, 0)
			}
		case api.BlockParagraph:
			if e.editable && noteInlineSpans(b.Spans) {
				e.paragraphInput(c, index, fmt.Sprintf("Paragraf %d", index+1), "Ketik / untuk memilih blok", &e.blocks[index].Spans)
			} else {
				noteDocumentRichText(c, b.Spans, true)
			}
		case api.BlockCallout:
			accent := t.Accent
			if b.Variant == api.CalloutDecision {
				accent = t.Success
			}
			ui.Row(c).FillWidth().MinWidth(0).Gap(10).Padding(10, 12).AlignItems(ui.Start).Radius(8).Background(spaceMix(accent, t.Surface, .9)).Border(1, spaceMix(accent, t.Surface, .55)).Children(func() {
				icon := "info"
				if b.Variant == api.CalloutDecision {
					icon = "check"
				}
				ui.Icon(c, spaceIcon(icon)).Size(16, 16).TextColor(accent).Shrink(0)
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, noteCalloutLabel(b.Variant)).FontSize(11).Bold().TextColor(accent)
					if e.editable && noteInlineSpans(b.Spans) {
						e.paragraphInput(c, index, fmt.Sprintf("Isi blok %d", index+1), "Tulis "+strings.ToLower(noteCalloutLabel(b.Variant))+"…", &e.blocks[index].Spans)
					} else {
						noteDocumentRichText(c, b.Spans, true)
					}
				})
			})
		case api.BlockToggle:
			e.toggleView(c, index, b)
		case api.BlockBulletedList, api.BlockNumberedList, api.BlockChecklist:
			e.listView(c, index, b)
		case api.BlockTable:
			e.tableView(c, index, b)
		case api.BlockDivider:
			ui.Box(c).Height(1).FillWidth().Margin(10, 0).Background(t.Border)
		default:
			noteReadOnlyBlock(c, b)
		}
	})
	return box
}

// paragraphInput is a borderless text area as high as its text, editing
// plain spans. Typing "/" at the start opens the block menu; a typed
// Markdown marker ("# ", "- ", "1. ", "[] ") turns a paragraph into that
// block.
func (e *noteDocumentEditor) paragraphInput(c *ui.Context, index int, label, placeholder string, spans *[]api.Span) {
	value := noteSpansText(*spans)
	in := ui.TextAreaBase(c.Key("inline-paragraph"), &value).Lines(1, 40).Label(label).Placeholder(placeholder).FillWidth().MinWidth(0).FontSize(14).Background(ui.RGBA(0, 0, 0, 0)).BorderWidth(0).Padding(4, 0)
	in.OnTextKey(func(k ui.TextKey) bool { return e.textKey(index, -1, k) })
	if e.takeFocus(index, -1) {
		in.Focus()
	}
	if in.Focused() {
		e.selectBlock(index)
	}
	if !in.Changed() {
		return
	}
	e.changed = true
	paragraph := e.blocks[index].Type == api.BlockParagraph
	switch {
	case paragraph && (value == "# " || value == "## " || value == "### "):
		e.blocks[index] = api.NoteBlock{Type: api.BlockHeading, Level: len(value) - 1}
		e.focusOn(index, -1)
		return
	case paragraph && value == "---":
		e.blocks[index] = api.NoteBlock{Type: api.BlockDivider}
		at := e.insertBlocks(index, false, []api.NoteBlock{{Type: api.BlockParagraph, Spans: []api.Span{{}}}})
		e.focusOn(at, -1)
		return
	case paragraph && (value == "- " || value == "* " || value == "1. " || value == "[] " || value == "[ ] "):
		kind := api.BlockBulletedList
		if value == "1. " {
			kind = api.BlockNumberedList
		}
		if value == "[] " || value == "[ ] " {
			kind = api.BlockChecklist
		}
		e.blocks[index] = api.NoteBlock{Type: kind, Items: []api.NoteListItem{noteEmptyItem()}}
		e.focusOn(index, 0)
		return
	}
	*spans = noteRespan(*spans, value)
	e.afterInlineEdit(index, -1)
	if paragraph && notePlainSpans(*spans) && strings.HasPrefix(value, "/") && !strings.ContainsAny(value, "\n") && len([]rune(value)) <= 32 {
		e.menu = noteBlockMenu{open: true, block: index, replace: true, query: value[1:]}
	} else if e.menu.open && e.menu.block == index && e.menu.replace {
		e.menu.open = false
	}
}

// blockMenu is the "/" and "+" menu under the block: Notulen above the
// basic blocks, filtered by what follows "/". A phone gets a grid.
func (e *noteDocumentEditor) blockMenu(c *ui.Context, anchor ui.Element) {
	t := c.Theme()
	matches := noteMenuMatches(e.menu.query)
	e.menu.index = min(e.menu.index, max(len(matches)-1, 0))
	width := float32(260)
	if e.Compact {
		width = max(anchor.Bounds().W, 280)
	}
	ui.PopoverBase(c.Key("block-menu"), anchor, &e.menu.open, func(panel ui.Element) {
		popPanel(panel, 0)
		panel.Margin(4, 0, 0, 0).Width(width).MaxHeight(360).Padding(6).Gap(2).Radius(10).Background(t.Surface).Border(1, t.Border).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, .18)).Label("Pilih jenis blok")
		ui.Scroll(c).FillWidth().MaxHeight(348).Children(func() {
			ui.Column(c).FillWidth().MinWidth(0).Gap(2).Children(func() {
				if len(matches) == 0 {
					muted(c, "Tidak ada blok bernama “"+e.menu.query+"”").FontSize(12).Padding(8)
					return
				}
				group := ""
				var grid []noteMenuEntry
				flush := func() {
					if len(grid) == 0 {
						return
					}
					items := grid
					grid = nil
					ui.Row(c.Key("grid-" + items[0].key)).FillWidth().Wrap().Gap(6).Children(func() {
						for _, m := range items {
							e.menuTile(c, m)
						}
					})
				}
				for i, m := range matches {
					if m.group != group {
						flush()
						group = m.group
						ui.Text(c.Key("group-"+group), group).FontSize(11).TextColor(faint(t)).Padding(6, 8, 2, 8)
					}
					if e.Compact {
						grid = append(grid, m)
						continue
					}
					e.menuRow(c, m, i == e.menu.index)
				}
				flush()
			})
		})
	}).AttachTo(anchor, ui.AnchorBottomLeft, ui.AnchorTopLeft)
}

func noteMenuMark(c *ui.Context, m noteMenuEntry, size float32) {
	t := c.Theme()
	ui.Box(c).Size(size, size).Shrink(0).Center().Radius(6).Background(spaceMix(t.Surface, t.Background, .4)).Border(1, t.Border).Children(func() {
		if m.icon != "" {
			ui.Icon(c, spaceIcon(m.icon)).Size(size*.5, size*.5).TextColor(t.TextMuted)
		} else {
			ui.Text(c, m.glyph).FontSize(size * .38).Bold().TextColor(t.TextMuted)
		}
	})
}

func (e *noteDocumentEditor) menuRow(c *ui.Context, m noteMenuEntry, active bool) {
	t := c.Theme()
	// Gaya enowX: one compact line, the shortcut on the right; the hint
	// is the tooltip.
	b := ui.ButtonBase(c.Key(m.key)).Label(m.label).Tooltip(m.hint).Row().FillWidth().MinWidth(0).Height(30).Padding(0, 8).Gap(8).AlignItems(ui.Center).Radius(6)
	if active || b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		ui.Box(c).Width(20).Shrink(0).Center().Children(func() {
			if m.icon != "" {
				ui.Icon(c, spaceIcon(m.icon)).Size(15, 15).TextColor(t.TextMuted)
			} else {
				ui.Text(c, m.glyph).FontSize(12).Bold().TextColor(t.TextMuted)
			}
		})
		ui.Text(c, m.label).FontSize(13).MaxLines(1).Grow(1).MinWidth(0)
		if m.short != "" {
			ui.Text(c, m.short).Font(FontMono).FontSize(11).TextColor(faint(t)).Shrink(0)
		}
	})
	if b.Clicked() {
		e.applyMenu(m)
	}
}

func (e *noteDocumentEditor) menuTile(c *ui.Context, m noteMenuEntry) {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(m.key)).Label(m.label).Column().Width(84).MinHeight(72).Padding(8, 4).Gap(6).AlignItems(ui.Center).Radius(8).Border(1, t.Border)
	if m.group == noteMenuNotulen {
		b.Background(accentSoft(t))
	}
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		noteMenuMark(c, m, 32)
		ui.Text(c, m.label).FontSize(11).MaxLines(2).TextAlign(ui.Center)
	})
	if b.Clicked() {
		e.applyMenu(m)
	}
}

// toggleView: a title that folds its lines; the notulis edits the title and
// the plain text lines inside (one level deep, as the server allows).
func (e *noteDocumentEditor) toggleView(c *ui.Context, index int, b api.NoteBlock) {
	t := c.Theme()
	closed := e.closed[index]
	ui.Row(c).FillWidth().MinWidth(0).Gap(4).AlignItems(ui.Center).Children(func() {
		chev := "down"
		label := fmt.Sprintf("Tutup toggle %d", index+1)
		if closed {
			chev, label = "right", fmt.Sprintf("Buka toggle %d", index+1)
		}
		if noteHandle(c.Key("fold"), label, chev).Clicked() {
			if e.closed == nil {
				e.closed = map[int]bool{}
			}
			e.closed[index] = !closed
		}
		if e.editable {
			in := ui.TextInput(c.Key("toggle-title"), &e.blocks[index].Text).Label(fmt.Sprintf("Judul toggle %d", index+1)).Placeholder("Judul toggle").FillWidth().FontSize(14).Bold().Background(ui.RGBA(0, 0, 0, 0)).BorderWidth(0).Padding(4, 0)
			in.OnTextKey(func(k ui.TextKey) bool { return e.textKey(index, -1, k) })
			if e.takeFocus(index, -1) {
				in.Focus()
			}
			if in.Focused() {
				e.selectBlock(index)
			}
			if in.Changed() {
				e.changed = true
			}
		} else {
			title := b.Text
			if title == "" {
				title = "Toggle"
			}
			ui.Text(c, title).FontSize(14).Bold().Grow(1).MinWidth(0)
		}
	})
	if closed {
		return
	}
	ui.Column(c).FillWidth().MinWidth(0).Gap(4).Padding(0, 0, 0, 28).Children(func() {
		for i, child := range b.Children {
			ui.Column(c.Key(i)).FillWidth().MinWidth(0).Children(func() {
				if e.editable && child.Type == api.BlockParagraph && notePlainSpans(child.Spans) {
					value := noteSpansText(child.Spans)
					in := ui.TextAreaBase(c.Key("toggle-line"), &value).Lines(1, 20).Label(fmt.Sprintf("Isi toggle %d baris %d", index+1, i+1)).Placeholder("Isi toggle").FillWidth().MinWidth(0).FontSize(14).Background(ui.RGBA(0, 0, 0, 0)).BorderWidth(0).Padding(2, 0)
					in.OnTextKey(e.undoKey)
					if in.Focused() {
						e.selectBlock(index)
					}
					if in.Changed() {
						e.blocks[index].Children[i].Spans = []api.Span{{Text: value}}
						e.changed = true
					}
					return
				}
				noteReadOnlyBlock(c, child)
			})
		}
		if e.editable {
			add := ui.ButtonBase(c.Key("toggle-add")).Label(fmt.Sprintf("Tambah baris di toggle %d", index+1)).Row().Gap(6).AlignItems(ui.Center).Padding(4, 0).TextColor(faint(t))
			add.FocusRing(add.FocusVisible()).Children(func() {
				ui.Icon(c, spaceIcon("plus")).Size(12, 12)
				ui.Text(c, "Tambah baris").FontSize(12)
			})
			if add.Clicked() {
				e.blocks[index].Children = append(e.blocks[index].Children, api.NoteBlock{Type: api.BlockParagraph, Spans: []api.Span{{}}})
				e.changed = true
			}
		}
	})
}

// extractView: "Jadikan tugas/reminder" for a block, item or row, present
// for keyboards and screen readers but drawn only when shown (hover, the
// block tools, or a phone's "⋯").
func (e *noteDocumentEditor) extractView(c *ui.Context, text, source string, shown bool) {
	e.extractViewWith(c, text, source, shown, true)
}

// extractViewWith: task false leaves "Jadikan tugas" to the action item's
// own button, which fills the task's PJ and tenggat and keeps its id.
func (e *noteDocumentEditor) extractViewWith(c *ui.Context, text, source string, shown, task bool) {
	if e.OnExtract == nil || strings.TrimSpace(text) == "" {
		return
	}
	ui.Row(c.Key("extract")).Shrink(0).Gap(0).Opacity(noteShown(shown)).Children(func() {
		if task && e.ExtractTask && noteHandle(c.Key("task"), "Jadikan tugas dari "+source, "task").Clicked() {
			e.OnExtract(text, api.NoteLinkTask)
		}
		if noteHandle(c.Key("bell"), "Jadikan reminder dari "+source, "bell").Clicked() {
			e.OnExtract(text, api.NoteLinkReminder)
		}
	})
}

// listView: bullets, numbers or check boxes with an input per item; Enter
// makes the next item. Reminder buttons show on the item's hover.
func (e *noteDocumentEditor) listView(c *ui.Context, index int, b api.NoteBlock) {
	t := c.Theme()
	p := e.page(index)
	if noteDocumentPager(c, fmt.Sprintf("Butir blok %d", index+1), &p.items, len(b.Items), noteItemsPerPage) && e.selected == index {
		e.item, e.span, e.spanPage = p.items*noteItemsPerPage, 0, 0
	}
	start, end := p.items*noteItemsPerPage, min((p.items+1)*noteItemsPerPage, len(b.Items))
	for i := start; i < end; i++ {
		row := ui.Row(c.Key(i)).FillWidth().MinWidth(0).Gap(6).AlignItems(ui.Start)
		// An item whose PJ, tenggat or durasi picker is open keeps its
		// chips while the pointer goes over to the picker, which lies
		// outside the row: else the chip, and the picker on it, would go.
		picking := e.pick.kind != "" && e.pick.block == index && e.pick.item == i
		shown := row.Hovered() || picking || (e.Compact && (e.reveal == index+1 || e.editable && e.selected == index && e.item == i))
		row.Children(func() {
			switch b.Type {
			case api.BlockChecklist:
				checked := b.Items[i].Checked
				box := ui.ButtonBase(c.Key("check")).Label(fmt.Sprintf("Checklist butir %d blok %d", i+1, index+1)).Size(18, 18).Margin(6, 4, 0, 4).Shrink(0).Center().Radius(4).Border(1.5, t.TextMuted).Disabled(!e.editable)
				if checked {
					box.Background(t.Accent).BorderColor(t.Accent)
				}
				box.Children(func() {
					if checked {
						ui.Icon(c, spaceIcon("check")).Size(12, 12).TextColor(t.AccentText)
					}
				})
				if !e.editable {
					box.Label(fmt.Sprintf("Butir %d %s", i+1, map[bool]string{true: "selesai", false: "belum selesai"}[checked]))
				}
				if e.editable && box.Clicked() {
					e.toggleItem(index, i)
				}
			case api.BlockNumberedList:
				ui.Text(c, fmt.Sprintf("%d.", i+1)).Width(26).Padding(5, 0, 0, 4).TextColor(t.TextMuted).Shrink(0)
			default:
				ui.Text(c, "•").Width(26).Padding(5, 0, 0, 8).TextColor(t.TextMuted).Shrink(0)
			}
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				spans := b.Items[i].Spans
				if e.editable && noteInlineSpans(spans) {
					value := noteSpansText(spans)
					in := ui.TextInput(c.Key("inline-list-item"), &value).Label(fmt.Sprintf("Butir %d blok %d", i+1, index+1)).Placeholder("Tulis poin…").FillWidth().FontSize(14).Background(ui.RGBA(0, 0, 0, 0)).BorderWidth(0).Padding(4, 0)
					in.OnTextKey(func(k ui.TextKey) bool { return e.textKey(index, i, k) })
					if (e.focusBlock == index+1 && i == 0) || e.takeFocus(index, i) {
						in.Focus()
						e.focusBlock = 0
					}
					if in.Focused() {
						e.selectBlock(index)
						e.item = i
					}
					if in.Changed() {
						e.blocks[index].Items[i].Spans = noteRespan(spans, value)
						e.changed = true
						e.afterInlineEdit(index, i)
					}
				} else {
					ui.Column(c).FillWidth().MinWidth(0).Padding(4, 0).Children(func() { noteDocumentRichText(c, spans, true) })
				}
			})
			if e.editable && !noteInlineSpans(b.Items[i].Spans) && noteHandle(c.Key("rich"), fmt.Sprintf("Ubah butir %d blok %d", i+1, index+1), "edit").Clicked() {
				e.selectBlock(index)
				e.item, e.advanced = i, true
			}
			// A checklist item is an action item: PJ, tenggat and its task.
			action := b.Type == api.BlockChecklist
			if action {
				e.actionChips(c, index, i, b.Items[i], shown)
			}
			if b.Type == api.BlockNumberedList {
				e.minutesChip(c, index, i, b.Items[i], shown)
			}
			e.extractViewWith(c, noteSpansText(b.Items[i].Spans), fmt.Sprintf("butir %d blok %d", i+1, index+1), shown, !(action && e.editable && e.OnActionTask != nil))
		})
	}
}

// MobileBar is the phone's block bar under the document while editing:
// add (menu as a grid), heading, checklist, decision, and move up/down
// (dragging needs a pointer that hovers).
func (e *noteDocumentEditor) MobileBar(c *ui.Context) {
	if !e.editable {
		return
	}
	t := c.Theme()
	sel := e.selected
	valid := sel >= 0 && sel < len(e.blocks)
	ui.ScrollHorizontal(c.Key("note-block-bar")).FillWidth().Height(48).Children(func() {
		ui.Row(c).Height(48).Padding(0, 8).Gap(4).AlignItems(ui.Center).Children(func() {
			tool := func(key, label, icon, glyph string, disabled bool, do func()) {
				b := ui.ButtonBase(c.Key(key)).Label(label).Tooltip(label).Size(44, 40).Center().Radius(8).TextColor(t.Text).Disabled(disabled)
				if b.Hovered() {
					b.Background(t.SurfaceHover)
				}
				b.FocusRing(b.FocusVisible()).Children(func() {
					if icon != "" {
						ui.Icon(c, spaceIcon(icon)).Size(18, 18)
					} else {
						ui.Text(c, glyph).FontSize(14).Bold()
					}
				})
				if b.Clicked() {
					do()
				}
			}
			entry := func(key string) noteMenuEntry {
				for _, m := range noteMenuEntries() {
					if m.key == key {
						return m
					}
				}
				return noteMenuEntry{}
			}
			// The bar's blocks replace an empty line, else go below it.
			use := func(key string) func() {
				return func() {
					replace := valid && e.blocks[sel].Type == api.BlockParagraph && noteSpansText(e.blocks[sel].Spans) == ""
					e.menu = noteBlockMenu{block: sel, replace: replace}
					e.applyMenu(entry(key))
				}
			}
			tool("bar-add", "Tambah blok", "plus", "", len(e.blocks) >= api.NoteMaxBlocks, func() { e.openMenuBelow(sel) })
			tool("bar-h2", "Judul bagian", "", "H2", false, use("h2"))
			tool("bar-check", "Checklist", "", "☐", false, use("checklist"))
			tool("bar-decision", "Keputusan", "check", "", false, use("decision"))
			ui.Box(c).Width(1).Height(24).Background(t.Border).Margin(0, 4)
			tool("bar-up", "Naikkan blok", "", "↑", !valid || sel == 0, func() { e.moveBlock(-1) })
			tool("bar-down", "Turunkan blok", "", "↓", !valid || sel >= len(e.blocks)-1, func() { e.moveBlock(1) })
			tool("bar-tools", "Pilihan blok", "more", "", !valid, func() { e.advanced = !e.advanced })
			ui.Box(c).Width(1).Height(24).Background(t.Border).Margin(0, 4)
			tool("bar-undo", "Urungkan", "undo", "", !e.CanUndo(), func() { e.Undo() })
			tool("bar-redo", "Ulangi", "redo", "", !e.CanRedo(), func() { e.Redo() })
		})
	})
}

type noteHeading struct {
	index, level int
	text         string
}

// Headings are the document's top-level headings, for "Di halaman ini".
func (e *noteDocumentEditor) Headings() []noteHeading {
	out := []noteHeading{}
	for i, b := range e.blocks {
		if b.Type == api.BlockHeading && strings.TrimSpace(b.Text) != "" {
			out = append(out, noteHeading{index: i, level: max(1, min(b.Level, 3)), text: b.Text})
		}
	}
	return out
}

// Reveal shows the page holding a block and selects it (the notulis'
// caret goes into it).
func (e *noteDocumentEditor) Reveal(index int) {
	if index < 0 || index >= len(e.blocks) {
		return
	}
	e.selectBlock(index)
	if e.editable {
		e.focusOn(index, -1)
	}
}
