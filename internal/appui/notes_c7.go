package appui

import (
	"context"
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Stage C7: "Kirim ringkasan ke WhatsApp" in the Rapat tab of a note, for
// whoever may edit it. The button asks the server for the message (the
// decisions and action items of the saved note, with PJ, due date and a
// link) and shows it in a dialog with the group it goes to; sending is a
// second, deliberate tap. The bot of the division sends it.

// NoteSummaryAPI previews and sends the summary. Optional: without it the
// section is not shown.
type NoteSummaryAPI interface {
	NoteWASummary(context.Context, id.ID, api.NoteWASummaryRequest) (api.NoteWASummary, error)
}

func (a *App) noteSummaryService() (NoteSummaryAPI, bool) {
	if live, ok := a.api.(*client.Client); ok {
		return live.AccountSession(), true
	}
	s, ok := a.api.(NoteSummaryAPI)
	return s, ok
}

// noteSummaryState is the dialog of one note.
type noteSummaryState struct {
	open, busy bool
	note       id.ID
	preview    *api.NoteWASummary
	err        string
	toast      string
}

func (a *App) noteSummarySection(c *ui.Context) {
	n := a.notes.note
	if n == nil || !n.Permissions.Manage || a.offlineMode() {
		return
	}
	if _, ok := a.noteSummaryService(); !ok {
		return
	}
	st := &a.notes.summary
	if st.toast != "" {
		c.AddToast(ui.Toast{Title: st.toast, Type: "success", Timeout: 5 * time.Second})
		st.toast = ""
	}
	notePanelSection(c, "sec-wa-summary", "Ringkasan ke WhatsApp", func() {
		muted(c, "Keputusan dan action item (PJ + tenggat) dikirim bot ke grup WhatsApp divisi, dengan tautan catatan.").FontSize(12)
		if outlineButton(c.Key("wa-summary"), "Kirim ringkasan ke WhatsApp", "whatsapp", false).Height(30).Disabled(st.busy).Clicked() {
			a.loadNoteSummary(n.ID)
		}
	})
	if st.open && st.note == n.ID {
		a.noteSummaryDialog(c)
	}
}

func (a *App) loadNoteSummary(nid id.ID) {
	svc, ok := a.noteSummaryService()
	if !ok {
		return
	}
	st := &a.notes.summary
	*st = noteSummaryState{open: true, busy: true, note: nid}
	gen := a.gen
	a.async(func() func() {
		out, err := svc.NoteWASummary(context.Background(), nid, api.NoteWASummaryRequest{})
		return func() {
			st := &a.notes.summary
			if gen != a.gen || st.note != nid {
				return
			}
			st.busy = false
			if err != nil {
				st.err = a.failed(err)
				return
			}
			st.preview = &out
		}
	})
}

func (a *App) sendNoteSummary(nid id.ID, version int64) {
	svc, ok := a.noteSummaryService()
	st := &a.notes.summary
	if !ok || st.busy {
		return
	}
	st.busy, st.err = true, ""
	gen := a.gen
	a.async(func() func() {
		out, err := svc.NoteWASummary(context.Background(), nid, api.NoteWASummaryRequest{Send: true, ExpectedVersion: version})
		return func() {
			st := &a.notes.summary
			if gen != a.gen || st.note != nid {
				return
			}
			st.busy = false
			if err != nil {
				st.err = a.failed(err)
				return
			}
			st.open, st.preview = false, nil
			st.toast = "Ringkasan dikirim ke grup " + out.GroupName + "."
		}
	})
}

func (a *App) noteSummaryDialog(c *ui.Context) {
	t := c.Theme()
	st := &a.notes.summary
	ui.DialogBase(c.Key("wa-summary-dialog"), &st.open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.35))
		panel.Width(min(float32(480), mustWidth(c)-32)).Padding(20).Gap(12).Radius(14).Background(t.Surface).Border(1, t.Border).Label("Dialog ringkasan ke WhatsApp").Children(func() {
			ui.Text(c, "Kirim ringkasan ke WhatsApp?").FontSize(16).Bold()
			p := st.preview
			switch {
			case p == nil && st.busy:
				ui.Spinner(c).Label("Menyusun ringkasan…")
			case p == nil:
			default:
				if p.GroupName != "" {
					muted(c, "Ke grup "+p.GroupName+" · "+fmt.Sprintf("%d keputusan, %d action item", p.Decisions, p.Actions)).FontSize(12)
				}
				ui.Scroll(c).FillWidth().MaxHeight(300).Children(func() {
					ui.Text(c, p.Text).FontSize(13).LineHeight(1.5).Selectable().FillWidth().Padding(10, 12).Radius(8).Background(t.Background).Border(1, t.Border)
				})
				if a.notes.dirty {
					ui.Text(c, "Perubahan yang belum tersimpan tidak ikut terkirim. Simpan dulu bila perlu.").FontSize(12).TextColor(t.Warning)
				}
				if p.Problem != "" {
					ui.Text(c, p.Problem).FontSize(12).TextColor(t.Danger)
				} else if p.Warning != "" {
					ui.Text(c, p.Warning).FontSize(12).TextColor(t.Warning)
				}
			}
			if st.err != "" {
				ui.Text(c, st.err).FontSize(12).TextColor(t.Danger)
			}
			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
				if spaceButton(c.Key("wa-summary-cancel"), "Batal", "close", false).Clicked() {
					st.open = false
				}
				if p != nil && p.Problem == "" {
					label := "Kirim ke grup"
					if p.GroupName != "" {
						label = "Kirim ke " + p.GroupName
					}
					if primarySpaceButton(c.Key("wa-summary-send"), label, "send").Disabled(st.busy).Clicked() {
						a.sendNoteSummary(st.note, p.Version)
					}
				} else if st.err != "" && spaceButton(c.Key("wa-summary-retry"), "Coba lagi", "refresh", false).Disabled(st.busy).Clicked() {
					a.loadNoteSummary(st.note)
				}
			})
		})
	})
}
