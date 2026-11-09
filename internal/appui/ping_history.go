package appui

import (
	"context"
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
	Pings(ctx context.Context, space id.ID, before time.Time, limit int, filter string) ([]api.PingRecord, error)
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
	// Tahap 7c: the filter ("", "unseen", "failed"), the page shown with
	// each page's before= cursor, whether an older page exists, and the
	// ping whose "Ping ulang" waits for confirmation.
	filter        string
	page          int
	cursors       []time.Time
	hasMore       bool
	resendConfirm id.ID
	resending     bool
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
	_ = svc
	a.pingLoad()
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
