package server

import (
	"net/http"
	"slices"
	"strings"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// waPreviewKinds are the messages a preview can show.
var waPreviewKinds = []string{"reminder", "overdue", "ping", "reply", "weekly", "followup"}

// previewWA (Tahap 7d) renders a draft of the bot's settings (name,
// language, greeting, message texts) into exactly the text the bot would
// send, with example data, using the same functions as the real sends. It
// saves nothing and sends nothing; it needs the right to manage WhatsApp.
func (s *Server) previewWA(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.WAPreviewRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	if !slices.Contains(waPreviewKinds, req.Kind) {
		return badRequest("Jenis pratinjau tidak dikenal.")
	}
	draft := api.WAConnectionRequest{DisplayName: req.DisplayName, Language: req.Language, Greeting: req.Greeting, Templates: req.Templates}
	if strings.TrimSpace(draft.DisplayName) == "" {
		draft.DisplayName = "Bot"
	}
	if err := validateWA(&draft); err != nil {
		return err
	}
	var out api.WAPreview
	err := s.store.Tx(r.Context(), func(q store.Querier) error {
		sc, err := s.loadScope(r.Context(), q, r, p, store.LockShare)
		if err != nil {
			return err
		}
		if p.admin() {
			return errForbidden
		}
		if err := sc.require(api.CapWhatsAppManage); err != nil {
			return err
		}
		conn := api.WAConnection{DisplayName: draft.DisplayName, Greeting: draft.Greeting, Language: draft.Language, Templates: draft.Templates}
		out = api.WAPreview{Kind: req.Kind, Text: waPreviewText(conn, req.Kind, sc.division.Name), Example: true}
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// waPreviewText builds one message from example data the way the sender
// does: the localized phrase, then the template, name and greeting.
func waPreviewText(conn api.WAConnection, kind, division string) string {
	const task = "#T12 Poster open recruitment"
	switch kind {
	case "reminder", "overdue":
		title, body := waDeliveryText(conn.Language, kind, "", "", []string{task})
		return renderWAText(conn, "reminder", title, body)
	case "ping":
		return renderWAText(conn, "ping", "Ping · "+division, "Mohon perbarui progres tugas hari ini.")
	case "reply":
		body := "3 tugas berjalan:\n#T12 Poster open recruitment · Aruna · 10 Nov\n#T9 Video teaser · Raka · lewat 8 Nov\n#T14 Caption IG · Nara · 12 Nov"
		if conn.Language == "en" {
			body = "3 tasks in progress:\n#T12 Poster open recruitment · Aruna · 10 Nov\n#T9 Video teaser · Raka · overdue 8 Nov\n#T14 Caption IG · Nara · 12 Nov"
		}
		return renderWAText(conn, "reply", "", body)
	case "weekly":
		start := timeutil.Date{Year: 2026, Month: 11, Day: 2}
		report := api.SOPReport{DivisionName: division, Label: "Minggu ini", StartsOn: start, EndsOn: start.AddDays(6),
			Summary: api.SOPReportSummary{NeedsReview: 2, Confirmed: 1, Dismissed: 1}}
		return renderWAText(conn, "weekly", waTextLanguage(conn.Language, "report.title"), sopReportTextLanguage(report, conn.Language))
	default: // followup
		text := waTextLanguage(conn.Language, "followup", "Humas PJ", "Poster open recruitment", "Aruna Desain", "aruna", "2026-11-14", "https://drive.google.com/…")
		return renderWAText(conn, "followup", "Poster open recruitment", text)
	}
}
