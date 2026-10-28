package appui

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
)

const (
	noteBlocksPerPage = 8
	noteItemsPerPage  = 8
	noteRowsPerPage   = 8
	noteColsPerPage   = 4
	noteSpansPerPage  = 8
)

type noteDocumentPage struct{ items, rows, columns int }

// noteDocumentEditor owns a working document, never the canonical API value.
// Locks, saving, account isolation and recovery are the containing page's job.
type noteDocumentEditor struct {
	OnExtract func(text string, target string)
	blocks    []api.NoteBlock
	editable  bool
	changed   bool
	selected  int
	blockPage int
	item      int
	row       int
	column    int
	span      int
	spanPage  int
	addOpen   bool
	pages     map[int]*noteDocumentPage
}

func newNoteDocumentEditor(content []api.NoteBlock) *noteDocumentEditor {
	e := &noteDocumentEditor{blocks: cloneNoteDocument(content), selected: -1, pages: map[int]*noteDocumentPage{}}
	if len(content) > 0 {
		e.selected = 0
	}
	return e
}

func cloneNoteDocument(content []api.NoteBlock) []api.NoteBlock {
	out := slices.Clone(content)
	for i := range out {
		out[i].Spans = slices.Clone(content[i].Spans)
		out[i].Items = slices.Clone(content[i].Items)
		for j := range out[i].Items {
			out[i].Items[j].Spans = slices.Clone(content[i].Items[j].Spans)
		}
		out[i].Columns = slices.Clone(content[i].Columns)
		out[i].Rows = slices.Clone(content[i].Rows)
		for j := range out[i].Rows {
			out[i].Rows[j] = slices.Clone(content[i].Rows[j])
			for k := range out[i].Rows[j] {
				out[i].Rows[j][k] = slices.Clone(content[i].Rows[j][k])
			}
		}
	}
	return out
}

func (e *noteDocumentEditor) Content() []api.NoteBlock { return cloneNoteDocument(e.blocks) }
func (e *noteDocumentEditor) SetEditable(value bool) {
	e.editable = value
	if !value {
		e.addOpen = false
	}
}
func (e *noteDocumentEditor) TakeChanged() bool {
	changed := e.changed
	e.changed = false
	return changed
}

func noteSpansText(spans []api.Span) string {
	var text strings.Builder
	for _, span := range spans {
		text.WriteString(span.Text)
	}
	return text.String()
}
func noteRowText(row [][]api.Span) string {
	cells := make([]string, len(row))
	for i, cell := range row {
		cells[i] = noteSpansText(cell)
	}
	return strings.Join(cells, " | ")
}
func noteBlockText(b api.NoteBlock) string {
	switch b.Type {
	case api.BlockHeading:
		return b.Text
	case api.BlockParagraph:
		return noteSpansText(b.Spans)
	case api.BlockBulletedList, api.BlockNumberedList, api.BlockChecklist:
		lines := make([]string, len(b.Items))
		for i, item := range b.Items {
			lines[i] = noteSpansText(item.Spans)
		}
		return strings.Join(lines, "\n")
	case api.BlockTable:
		lines := make([]string, len(b.Rows))
		for i, row := range b.Rows {
			lines[i] = noteRowText(row)
		}
		return strings.Join(lines, "\n")
	}
	return ""
}

func noteBlockLabel(kind string) string {
	return map[string]string{api.BlockHeading: "Judul bagian", api.BlockParagraph: "Paragraf", api.BlockBulletedList: "Daftar poin", api.BlockNumberedList: "Daftar bernomor", api.BlockChecklist: "Checklist", api.BlockTable: "Tabel", api.BlockDivider: "Pemisah"}[kind]
}
func (e *noteDocumentEditor) page(index int) *noteDocumentPage {
	if p := e.pages[index]; p != nil {
		return p
	}
	p := &noteDocumentPage{}
	e.pages[index] = p
	return p
}
func (e *noteDocumentEditor) selectBlock(index int) {
	if index < 0 || index >= len(e.blocks) {
		return
	}
	e.selected, e.blockPage, e.span, e.spanPage = index, index/noteBlocksPerPage, 0, 0
	p := e.page(index)
	e.item, e.row, e.column = p.items*noteItemsPerPage, p.rows*noteRowsPerPage, p.columns*noteColsPerPage
}
func (e *noteDocumentEditor) currentBlock() *api.NoteBlock {
	if e.selected < 0 || e.selected >= len(e.blocks) {
		return nil
	}
	return &e.blocks[e.selected]
}
func (e *noteDocumentEditor) addBlock(kind string) {
	if !e.editable || len(e.blocks) >= api.NoteMaxBlocks || noteBlockLabel(kind) == "" {
		return
	}
	b := api.NoteBlock{Type: kind}
	switch kind {
	case api.BlockHeading:
		b.Level, b.Text = 2, "Judul bagian"
	case api.BlockParagraph:
		b.Spans = []api.Span{{}}
	case api.BlockBulletedList, api.BlockNumberedList, api.BlockChecklist:
		b.Items = []api.NoteListItem{{Spans: []api.Span{{}}}}
	case api.BlockTable:
		b.Columns = []string{"Tanggal", "Jam", "Kegiatan", "PJ", "Keterangan"}
		b.Rows = [][][]api.Span{noteEmptyRow(len(b.Columns))}
	}
	index := len(e.blocks)
	if e.selected >= 0 && e.selected < len(e.blocks) {
		index = e.selected + 1
	}
	e.blocks = slices.Insert(e.blocks, index, b)
	e.pages = map[int]*noteDocumentPage{}
	e.selectBlock(index)
	e.addOpen, e.changed = false, true
}
func (e *noteDocumentEditor) deleteBlock() {
	if !e.editable || e.currentBlock() == nil {
		return
	}
	e.blocks = slices.Delete(e.blocks, e.selected, e.selected+1)
	e.pages = map[int]*noteDocumentPage{}
	if len(e.blocks) == 0 {
		e.selected, e.blockPage = -1, 0
	} else {
		e.selectBlock(min(e.selected, len(e.blocks)-1))
	}
	e.changed = true
}
func (e *noteDocumentEditor) moveBlock(delta int) {
	to := e.selected + delta
	if !e.editable || delta == 0 || e.currentBlock() == nil || to < 0 || to >= len(e.blocks) {
		return
	}
	e.blocks[e.selected], e.blocks[to] = e.blocks[to], e.blocks[e.selected]
	e.pages = map[int]*noteDocumentPage{}
	e.selectBlock(to)
	e.changed = true
}
func noteEmptyRow(columns int) [][]api.Span {
	row := make([][]api.Span, columns)
	for i := range row {
		row[i] = []api.Span{{}}
	}
	return row
}
func (e *noteDocumentEditor) toggleItem(block, item int) {
	if !e.editable || block < 0 || block >= len(e.blocks) || e.blocks[block].Type != api.BlockChecklist || item < 0 || item >= len(e.blocks[block].Items) {
		return
	}
	e.blocks[block].Items[item].Checked = !e.blocks[block].Items[item].Checked
	e.changed = true
}
func (e *noteDocumentEditor) addItem() {
	b := e.currentBlock()
	if !e.editable || b == nil || (b.Type != api.BlockBulletedList && b.Type != api.BlockNumberedList && b.Type != api.BlockChecklist) {
		return
	}
	b.Items = append(b.Items, api.NoteListItem{Spans: []api.Span{{}}})
	e.item, e.span, e.spanPage = len(b.Items)-1, 0, 0
	e.page(e.selected).items = e.item / noteItemsPerPage
	e.changed = true
}
func (e *noteDocumentEditor) deleteItem() {
	b := e.currentBlock()
	if !e.editable || b == nil || len(b.Items) <= 1 || e.item < 0 || e.item >= len(b.Items) {
		return
	}
	b.Items = slices.Delete(b.Items, e.item, e.item+1)
	e.item = min(e.item, len(b.Items)-1)
	e.page(e.selected).items = e.item / noteItemsPerPage
	e.span, e.spanPage, e.changed = 0, 0, true
}
func (e *noteDocumentEditor) moveItem(delta int) {
	b := e.currentBlock()
	to := e.item + delta
	if !e.editable || delta == 0 || b == nil || e.item < 0 || e.item >= len(b.Items) || to < 0 || to >= len(b.Items) {
		return
	}
	b.Items[e.item], b.Items[to] = b.Items[to], b.Items[e.item]
	e.item = to
	e.page(e.selected).items = to / noteItemsPerPage
	e.changed = true
}
func (e *noteDocumentEditor) addRow() {
	b := e.currentBlock()
	if !e.editable || b == nil || b.Type != api.BlockTable || len(b.Columns) == 0 || len(b.Rows) >= api.NoteMaxTableRows {
		return
	}
	b.Rows = append(b.Rows, noteEmptyRow(len(b.Columns)))
	e.row, e.span, e.spanPage = len(b.Rows)-1, 0, 0
	e.page(e.selected).rows = e.row / noteRowsPerPage
	e.changed = true
}
func (e *noteDocumentEditor) deleteRow() {
	b := e.currentBlock()
	if !e.editable || b == nil || b.Type != api.BlockTable || e.row < 0 || e.row >= len(b.Rows) {
		return
	}
	b.Rows = slices.Delete(b.Rows, e.row, e.row+1)
	e.row = max(0, min(e.row, len(b.Rows)-1))
	e.page(e.selected).rows = e.row / noteRowsPerPage
	e.span, e.spanPage, e.changed = 0, 0, true
}
func (e *noteDocumentEditor) moveRow(delta int) {
	b := e.currentBlock()
	to := e.row + delta
	if !e.editable || delta == 0 || b == nil || b.Type != api.BlockTable || e.row < 0 || e.row >= len(b.Rows) || to < 0 || to >= len(b.Rows) {
		return
	}
	b.Rows[e.row], b.Rows[to] = b.Rows[to], b.Rows[e.row]
	e.row = to
	e.page(e.selected).rows = to / noteRowsPerPage
	e.changed = true
}
func (e *noteDocumentEditor) addColumn() {
	b := e.currentBlock()
	if !e.editable || b == nil || b.Type != api.BlockTable || len(b.Columns) >= api.NoteMaxTableCols {
		return
	}
	b.Columns = append(b.Columns, "Kolom baru")
	for i := range b.Rows {
		b.Rows[i] = append(b.Rows[i], []api.Span{{}})
	}
	e.column, e.span, e.spanPage = len(b.Columns)-1, 0, 0
	e.page(e.selected).columns = e.column / noteColsPerPage
	e.changed = true
}
func (e *noteDocumentEditor) deleteColumn() {
	b := e.currentBlock()
	if !e.editable || b == nil || b.Type != api.BlockTable || len(b.Columns) <= 1 || e.column < 0 || e.column >= len(b.Columns) {
		return
	}
	b.Columns = slices.Delete(b.Columns, e.column, e.column+1)
	for i := range b.Rows {
		if e.column < len(b.Rows[i]) {
			b.Rows[i] = slices.Delete(b.Rows[i], e.column, e.column+1)
		}
	}
	e.column = min(e.column, len(b.Columns)-1)
	e.page(e.selected).columns = e.column / noteColsPerPage
	e.span, e.spanPage, e.changed = 0, 0, true
}
func (e *noteDocumentEditor) spans() *[]api.Span {
	b := e.currentBlock()
	if b == nil {
		return nil
	}
	switch b.Type {
	case api.BlockParagraph:
		return &b.Spans
	case api.BlockBulletedList, api.BlockNumberedList, api.BlockChecklist:
		if e.item >= 0 && e.item < len(b.Items) {
			return &b.Items[e.item].Spans
		}
	case api.BlockTable:
		if e.row >= 0 && e.row < len(b.Rows) && e.column >= 0 && e.column < len(b.Rows[e.row]) {
			return &b.Rows[e.row][e.column]
		}
	}
	return nil
}
func (e *noteDocumentEditor) addSpan() {
	spans := e.spans()
	if !e.editable || spans == nil {
		return
	}
	*spans = append(*spans, api.Span{})
	e.span = len(*spans) - 1
	e.spanPage = e.span / noteSpansPerPage
	e.changed = true
}
func (e *noteDocumentEditor) deleteSpan() {
	spans := e.spans()
	if !e.editable || spans == nil || e.span < 0 || e.span >= len(*spans) {
		return
	}
	*spans = slices.Delete(*spans, e.span, e.span+1)
	e.span = max(0, min(e.span, len(*spans)-1))
	e.spanPage = e.span / noteSpansPerPage
	e.changed = true
}

func noteDocumentPager(c *ui.Context, label string, page *int, length, size int) bool {
	last := max(0, (length-1)/size)
	*page = max(0, min(*page, last))
	changed := false
	ui.Row(c).FillWidth().MinWidth(0).Wrap().Gap(6).AlignItems(ui.Center).Children(func() {
		muted(c, fmt.Sprintf("%s %d–%d dari %d", label, min(*page*size+1, length), min((*page+1)*size, length), length)).FontSize(11)
		if last > 0 {
			if spaceIconButton(c, label+" sebelumnya", "arrow").Disabled(*page == 0).Clicked() {
				*page--
				changed = true
			}
			if spaceIconButton(c, label+" berikutnya", "arrow").Disabled(*page >= last).Clicked() {
				*page++
				changed = true
			}
		}
	})
	return changed
}

func (e *noteDocumentEditor) View(c *ui.Context) {
	ui.Column(c.Key(e)).FillWidth().MinWidth(0).Gap(14).Children(func() {
		ui.Row(c).FillWidth().MinWidth(0).Wrap().Gap(6).Children(func() {
			if e.editable {
				b := spaceButton(c, "Tambah blok", "plus", true).Disabled(len(e.blocks) >= api.NoteMaxBlocks)
				if b.Clicked() {
					e.addOpen = !e.addOpen
				}
				popover(c, b, &e.addOpen, 235, func() {
					ui.Text(c, "Pilih jenis blok").Bold()
					for _, kind := range []string{api.BlockHeading, api.BlockParagraph, api.BlockBulletedList, api.BlockNumberedList, api.BlockChecklist, api.BlockTable, api.BlockDivider} {
						if spaceButton(c.Key(kind), noteBlockLabel(kind), "plus", false).FillWidth().Clicked() {
							e.addBlock(kind)
						}
					}
				})
			} else {
				muted(c, "Isi catatan · hanya baca").FontSize(12)
			}
		})
		if noteDocumentPager(c, "Blok", &e.blockPage, len(e.blocks), noteBlocksPerPage) && len(e.blocks) > 0 {
			e.selectBlock(e.blockPage * noteBlocksPerPage)
		}
		if len(e.blocks) == 0 {
			muted(c, "Catatan belum memiliki isi.")
			return
		}
		start, end := e.blockPage*noteBlocksPerPage, min((e.blockPage+1)*noteBlocksPerPage, len(e.blocks))
		for i := start; i < end && i < len(e.blocks); i++ {
			e.blockView(c.Key(i), i)
		}
	})
}

func (e *noteDocumentEditor) blockView(c *ui.Context, index int) {
	b := e.blocks[index]
	border := c.Theme().Border
	if e.editable && e.selected == index {
		border = c.Theme().Accent
	}
	ui.Column(c).FillWidth().MinWidth(0).Gap(12).Padding(14).Radius(9).Background(c.Theme().Surface).Border(1, border).Children(func() {
		ui.Row(c).FillWidth().MinWidth(0).Gap(6).AlignItems(ui.Center).Children(func() {
			label := noteBlockLabel(b.Type)
			if label == "" {
				label = "Blok yang belum didukung"
			}
			muted(c, fmt.Sprintf("%d · %s", index+1, label)).FontSize(11).Grow(1).MinWidth(0)
			if e.editable && spaceIconButton(c, fmt.Sprintf("Ubah blok %d", index+1), "edit").Clicked() {
				e.selectBlock(index)
			}
		})
		switch b.Type {
		case api.BlockHeading:
			ui.Text(c, b.Text).FontSize(float32(28 - max(1, min(b.Level, 3))*4)).Bold().FillWidth().MinWidth(0)
		case api.BlockParagraph:
			noteDocumentRichText(c, b.Spans, true)
		case api.BlockBulletedList, api.BlockNumberedList, api.BlockChecklist:
			e.listView(c, index, b)
		case api.BlockTable:
			e.tableView(c, index, b)
		case api.BlockDivider:
			ui.Box(c).Height(1).FillWidth().Background(c.Theme().Border)
		default:
			muted(c, "Isi blok dipertahankan. Jenis ini belum dapat diedit di aplikasi ini.")
		}
		if e.OnExtract != nil && b.Type != api.BlockDivider {
			e.extractView(c, noteBlockText(b), fmt.Sprintf("blok %d", index+1))
		}
		if e.editable && e.selected == index {
			e.blockEditor(c)
		}
	})
}

func noteDocumentRichText(c *ui.Context, spans []api.Span, links bool) {
	ui.RichText(c).FillWidth().MinWidth(0).FontSize(14).Children(func() {
		for index, span := range spans {
			var text ui.Element
			u, err := url.Parse(span.Link)
			validLink := err == nil && u.Host != "" && u.User == nil && (u.Scheme == "http" || u.Scheme == "https")
			if links && validLink {
				text = ui.Link(c.Key(index), span.Text, span.Link)
			} else {
				text = ui.Text(c.Key(index), span.Text)
				if validLink {
					text.TextColor(c.Theme().Accent).Underline()
				}
			}
			if span.Bold {
				text.Bold()
			}
		}
	})
}

func (e *noteDocumentEditor) extractView(c *ui.Context, text, source string) {
	if e.OnExtract == nil || strings.TrimSpace(text) == "" {
		return
	}
	ui.Row(c).FillWidth().MinWidth(0).Wrap().Gap(6).Children(func() {
		if spaceIconButton(c, "Jadikan tugas dari "+source, "task").Clicked() {
			e.OnExtract(text, api.NoteLinkTask)
		}
		if spaceIconButton(c, "Jadikan reminder dari "+source, "bell").Clicked() {
			e.OnExtract(text, api.NoteLinkReminder)
		}
	})
}

func (e *noteDocumentEditor) listView(c *ui.Context, index int, b api.NoteBlock) {
	p := e.page(index)
	if noteDocumentPager(c, fmt.Sprintf("Butir blok %d", index+1), &p.items, len(b.Items), noteItemsPerPage) && e.selected == index {
		e.item, e.span, e.spanPage = p.items*noteItemsPerPage, 0, 0
	}
	start, end := p.items*noteItemsPerPage, min((p.items+1)*noteItemsPerPage, len(b.Items))
	for i := start; i < end; i++ {
		ui.Column(c.Key(i)).FillWidth().MinWidth(0).Gap(4).Children(func() {
			ui.Row(c).FillWidth().MinWidth(0).Gap(8).AlignItems(ui.Start).Children(func() {
				switch b.Type {
				case api.BlockChecklist:
					if e.editable {
						icon := "plus"
						if b.Items[i].Checked {
							icon = "check"
						}
						if spaceIconButton(c, fmt.Sprintf("Checklist butir %d blok %d", i+1, index+1), icon).Clicked() {
							e.toggleItem(index, i)
						}
					} else {
						mark := "□"
						if b.Items[i].Checked {
							mark = "✓"
						}
						ui.Text(c, mark).Width(22).TextColor(c.Theme().TextMuted)
					}
				case api.BlockNumberedList:
					ui.Text(c, fmt.Sprintf("%d.", i+1)).Width(28).TextColor(c.Theme().TextMuted)
				default:
					ui.Text(c, "•").Width(22).TextColor(c.Theme().TextMuted)
				}
				ui.Column(c).Grow(1).MinWidth(0).Children(func() { noteDocumentRichText(c, b.Items[i].Spans, true) })
				if e.editable && spaceIconButton(c, fmt.Sprintf("Ubah butir %d blok %d", i+1, index+1), "edit").Clicked() {
					e.selectBlock(index)
					e.item = i
				}
			})
			e.extractView(c, noteSpansText(b.Items[i].Spans), fmt.Sprintf("butir %d blok %d", i+1, index+1))
		})
	}
}

func (e *noteDocumentEditor) tableView(c *ui.Context, index int, b api.NoteBlock) {
	p := e.page(index)
	if noteDocumentPager(c, fmt.Sprintf("Baris blok %d", index+1), &p.rows, len(b.Rows), noteRowsPerPage) && e.selected == index {
		e.row, e.span, e.spanPage = p.rows*noteRowsPerPage, 0, 0
	}
	if noteDocumentPager(c, fmt.Sprintf("Kolom blok %d", index+1), &p.columns, len(b.Columns), noteColsPerPage) && e.selected == index {
		e.column, e.span, e.spanPage = p.columns*noteColsPerPage, 0, 0
	}
	rStart, rEnd := p.rows*noteRowsPerPage, min((p.rows+1)*noteRowsPerPage, len(b.Rows))
	cStart, cEnd := p.columns*noteColsPerPage, min((p.columns+1)*noteColsPerPage, len(b.Columns))
	ui.ScrollHorizontal(c).FillWidth().Height(float32(36 + (rEnd-rStart)*64)).Children(func() {
		ui.Column(c).Gap(1).Background(c.Theme().Border).Children(func() {
			ui.Row(c).Gap(1).Children(func() {
				for column := cStart; column < cEnd; column++ {
					ui.Text(c.Key(column), b.Columns[column]).Width(150).Height(34).Padding(8).FontSize(12).Bold().Background(c.Theme().SurfaceHover)
				}
			})
			for row := rStart; row < rEnd; row++ {
				ui.Row(c.Key(row)).Gap(1).Children(func() {
					for column := cStart; column < cEnd; column++ {
						if column >= len(b.Rows[row]) {
							continue
						}
						cell := ui.ButtonBase(c.Key(column)).Column().AlignItems(ui.Stretch).Label(fmt.Sprintf("Sel %d,%d blok %d", row+1, column+1, index+1))
						cell.Width(150).Height(63).Padding(8).Background(c.Theme().Background).Clip().Children(func() { noteDocumentRichText(c, b.Rows[row][column], false) })
						if cell.Clicked() {
							e.selectBlock(index)
							e.row, e.column = row, column
						}
					}
					if e.OnExtract != nil {
						ui.Column(c).Padding(4).Children(func() { e.extractView(c, noteRowText(b.Rows[row]), fmt.Sprintf("baris %d blok %d", row+1, index+1)) })
					}
				})
			}
		})
	})
	if !e.editable && e.selected == index && e.row >= 0 && e.row < len(b.Rows) && e.column >= 0 && e.column < len(b.Rows[e.row]) {
		muted(c, fmt.Sprintf("Isi sel · baris %d, kolom %d", e.row+1, e.column+1)).FontSize(11)
		noteDocumentRichText(c, b.Rows[e.row][e.column], true)
	}
}

func (e *noteDocumentEditor) blockEditor(c *ui.Context) {
	b := e.currentBlock()
	if !e.editable || b == nil {
		return
	}
	ui.Column(c).FillWidth().MinWidth(0).Gap(10).Padding(12).Radius(8).Background(c.Theme().Background).Children(func() {
		ui.Row(c).FillWidth().MinWidth(0).Wrap().Gap(5).Children(func() {
			if spaceIconButton(c, "Naikkan blok", "arrow").Disabled(e.selected == 0).Clicked() {
				e.moveBlock(-1)
			}
			if spaceIconButton(c, "Turunkan blok", "arrow").Disabled(e.selected >= len(e.blocks)-1).Clicked() {
				e.moveBlock(1)
			}
			if spaceButton(c, "Hapus blok", "close", false).Clicked() {
				e.deleteBlock()
			}
		})
		// A reorder/delete can change which block is selected in this frame.
		b = e.currentBlock()
		if b == nil {
			return
		}
		switch b.Type {
		case api.BlockHeading:
			if ui.TextInput(c.Key("heading"), &b.Text).FillWidth().Label("Teks judul bagian").Changed() {
				e.changed = true
			}
			ui.Row(c).FillWidth().Wrap().Gap(6).Children(func() {
				for level := 1; level <= 3; level++ {
					if spaceButton(c.Key(level), fmt.Sprintf("H%d", level), "", b.Level == level).Clicked() {
						b.Level, e.changed = level, true
					}
				}
			})
		case api.BlockBulletedList, api.BlockNumberedList, api.BlockChecklist:
			muted(c, fmt.Sprintf("Butir terpilih: %d", e.item+1)).FontSize(11)
			ui.Row(c).FillWidth().Wrap().Gap(5).Children(func() {
				if spaceButton(c, "Tambah butir", "plus", false).Clicked() {
					e.addItem()
				}
				if spaceIconButton(c, "Naikkan butir", "arrow").Disabled(e.item <= 0).Clicked() {
					e.moveItem(-1)
				}
				if spaceIconButton(c, "Turunkan butir", "arrow").Disabled(e.item >= len(b.Items)-1).Clicked() {
					e.moveItem(1)
				}
				if spaceButton(c, "Hapus butir", "close", false).Disabled(len(b.Items) <= 1).Clicked() {
					e.deleteItem()
				}
			})
		case api.BlockTable:
			muted(c, fmt.Sprintf("Sel terpilih: baris %d, kolom %d", e.row+1, e.column+1)).FontSize(11)
			if e.column >= 0 && e.column < len(b.Columns) {
				ui.Field(c, "Judul kolom", func() {
					if ui.TextInput(c.Key(fmt.Sprintf("column-%d", e.column)), &b.Columns[e.column]).FillWidth().Label("Judul kolom").Changed() {
						e.changed = true
					}
				})
			}
			ui.Row(c).FillWidth().Wrap().Gap(5).Children(func() {
				if spaceButton(c, "Tambah baris", "plus", false).Disabled(len(b.Rows) >= api.NoteMaxTableRows).Clicked() {
					e.addRow()
				}
				if spaceButton(c, "Hapus baris", "close", false).Disabled(len(b.Rows) == 0).Clicked() {
					e.deleteRow()
				}
				if spaceIconButton(c, "Naikkan baris", "arrow").Disabled(e.row <= 0).Clicked() {
					e.moveRow(-1)
				}
				if spaceIconButton(c, "Turunkan baris", "arrow").Disabled(e.row >= len(b.Rows)-1).Clicked() {
					e.moveRow(1)
				}
				if spaceButton(c, "Tambah kolom", "plus", false).Disabled(len(b.Columns) >= api.NoteMaxTableCols).Clicked() {
					e.addColumn()
				}
				if spaceButton(c, "Hapus kolom", "close", false).Disabled(len(b.Columns) <= 1).Clicked() {
					e.deleteColumn()
				}
			})
		}
		if spans := e.spans(); spans != nil {
			e.spansEditor(c.Key(fmt.Sprintf("spans-%d-%d-%d", e.item, e.row, e.column)), spans)
		}
	})
}

func (e *noteDocumentEditor) spansEditor(c *ui.Context, spans *[]api.Span) {
	if !e.editable {
		return
	}
	muted(c, "Setiap bagian teks mempertahankan format tebal dan tautannya sendiri.").FontSize(11)
	if noteDocumentPager(c, "Bagian teks", &e.spanPage, len(*spans), noteSpansPerPage) {
		e.span = e.spanPage * noteSpansPerPage
	}
	ui.Row(c).FillWidth().Wrap().Gap(5).Children(func() {
		for i := e.spanPage * noteSpansPerPage; i < min((e.spanPage+1)*noteSpansPerPage, len(*spans)); i++ {
			if spaceButton(c.Key(i), fmt.Sprintf("Bagian %d", i+1), "", e.span == i).Clicked() {
				e.span = i
			}
		}
		if spaceButton(c, "Tambah bagian teks", "plus", false).Clicked() {
			e.addSpan()
		}
	})
	if e.span < 0 || e.span >= len(*spans) {
		return
	}
	span := &(*spans)[e.span]
	ui.Field(c, "Teks bagian", func() {
		if ui.TextArea(c.Key(e.span), &span.Text).FillWidth().Height(84).Label("Teks bagian").Changed() {
			e.changed = true
		}
	})
	ui.Row(c).FillWidth().Wrap().Gap(5).Children(func() {
		if spaceButton(c, "Tebal", "edit", span.Bold).Clicked() {
			span.Bold, e.changed = !span.Bold, true
		}
		if spaceButton(c, "Hapus bagian teks", "close", false).Clicked() {
			e.deleteSpan()
		}
	})
	ui.Field(c, "Tautan bagian", func() {
		if ui.TextInput(c.Key(fmt.Sprintf("link-%d", e.span)), &span.Link).FillWidth().Label("Tautan bagian").Placeholder("https://… atau kosong").Changed() {
			e.changed = true
		}
	})
}
