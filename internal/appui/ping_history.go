package appui

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
)

// PingHistoryAPI reads a Space's Riwayat ping. It is apart from DeliveryAPI
// so older services (and test fakes) without it still send pings.
type PingHistoryAPI interface {
	Pings(ctx context.Context, space id.ID, before time.Time, limit int) ([]api.PingRecord, error)
}

// pingHistoryLimit is how many pings the Ping tab shows.
const pingHistoryLimit = 50

type pingHistoryState struct {
	space   id.ID
	items   []api.PingRecord
	loading bool
	loaded  bool
	err     string
	// missing is set when the server has no history endpoint yet.
	missing bool
}

func (a *App) pingHistoryService() (PingHistoryAPI, bool) {
	if c, ok := a.api.(*client.Client); ok {
		return c.AccountSession(), true
	}
	s, ok := a.api.(PingHistoryAPI)
	return s, ok
}

// ensurePingHistory loads the selected Space's pings once, and again after
// a ping is sent (which clears loaded).
func (a *App) ensurePingHistory() {
	sid := a.selectedSpaceID()
	st := &a.pingHistory
	if st.space != sid {
		*st = pingHistoryState{space: sid}
	}
	if sid == id.Nil || st.loaded || st.loading || st.err != "" {
		return
	}
	svc, ok := a.pingHistoryService()
	if !ok {
		st.loaded, st.missing = true, true
		return
	}
	gen := a.gen
	st.loading = true
	a.async(func() func() {
		items, err := svc.Pings(context.Background(), sid, time.Time{}, pingHistoryLimit)
		return func() {
			if gen != a.gen || a.pingHistory.space != sid {
				return
			}
			st := &a.pingHistory
			st.loading, st.loaded = false, true
			var ce *client.Error
			switch {
			case errors.As(err, &ce) && (ce.Status == http.StatusNotFound || ce.Status == http.StatusMethodNotAllowed):
				st.missing = true
			case err != nil:
				st.loaded = false
				st.err = a.failed(err)
			default:
				st.items, st.missing = items, false
			}
		}
	})
}

func (a *App) pingHistoryView(c *ui.Context) {
	a.ensurePingHistory()
	t := c.Theme()
	st := &a.pingHistory
	switch {
	case st.err != "":
		ui.Text(c, "Riwayat ping belum dapat dimuat: "+st.err).FontSize(12).TextColor(t.Danger)
		if spaceButton(c, "Muat ulang riwayat ping", "refresh", false).Clicked() {
			st.err = ""
		}
		return
	case st.loading && !st.loaded:
		muted(c, "Memuat riwayat ping…")
		return
	case st.missing:
		pageEmpty(c, "clock", "Riwayat ping belum tersedia", "Server ini belum menyediakan riwayat ping. Status pengiriman WhatsApp ada di Pengaturan, Aktivitas WhatsApp.")
		return
	case len(st.items) == 0:
		pageEmpty(c, "clock", "Belum ada ping", "Ping yang dikirim di Space ini muncul di sini, dengan status tiap penerima.")
		return
	}
	ui.Column(c).FillWidth().MinWidth(0).Radius(12).Background(t.Surface).Border(1, t.Border).Clip().Children(func() {
		for i, p := range st.items {
			row := ui.Column(c.Key(p.ID)).FillWidth().MinWidth(0).Padding(12, 14).Gap(6)
			if i > 0 {
				row.BorderWidth(1, 0, 0, 0).BorderColor(t.Border)
			}
			row.Children(func() {
				ui.Text(c, p.Message).FontSize(13).FontWeight(600).MaxLines(3)
				muted(c, pingByLine(p)).FontSize(12).MaxLines(1)
				ui.Row(c).FillWidth().MinWidth(0).Gap(6).Wrap().Children(func() {
					for j, r := range p.Recipients {
						ui.Text(c.Key(j), r.Recipient.DisplayName+" · "+pingRecipientLabel(r)).FontSize(11).TextColor(pingRecipientColor(t, r)).Padding(3, 8).Radius(6).Background(t.Background).MaxLines(1)
					}
				})
			})
		}
	})
}

// pingByLine is "Demo Mediatek · 08 Nov 2026 · 09.15 WIB · tugas".
func pingByLine(p api.PingRecord) string {
	parts := []string{}
	if p.Sender != nil && p.Sender.DisplayName != "" {
		parts = append(parts, p.Sender.DisplayName)
	}
	parts = append(parts, taskTimestamp(p.SentAt, p.TimeZone))
	switch p.RelatedKind {
	case "task":
		parts = append(parts, "tugas")
	case "meeting":
		parts = append(parts, "rapat")
	}
	return strings.Join(parts, " · ")
}

// pingRecipientLabel is a recipient's channel and status, as "perangkat ·
// dilihat" or "WA · gagal".
func pingRecipientLabel(r api.PingRecipientStatus) string {
	channel := map[string]string{"device": "perangkat", "whatsapp": "WA"}[r.Channel]
	if channel == "" {
		channel = r.Channel
	}
	status := map[string]string{"queued": "antre", "accepted": "terkirim", "failed": "gagal", "unknown": "tidak diketahui", "cancelled": "dibatalkan"}[r.Status]
	if status == "" {
		status = r.Status
	}
	if r.SeenAt != nil {
		status = "dilihat"
	}
	return channel + " · " + status
}

func pingRecipientColor(t *ui.Theme, r api.PingRecipientStatus) ui.Color {
	switch {
	case r.SeenAt != nil:
		return t.Success
	case r.Status == "failed":
		return t.Danger
	}
	return t.TextMuted
}
