package appui

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func TestWATokenWords(t *testing.T) {
	// Words typed by hand, in any case, are the placeholders; other
	// brackets and placeholders typed as before stay as they are.
	in := "[nama pengirim] siap! [Tanggal] {sender} [catatan] {body}"
	if got := waServerWith(waCommandTokens, in); got != "{sender} siap! {date} {sender} [catatan] {body}" {
		t.Fatalf("server %q", got)
	}
	if got := waFriendlyWith(waCommandTokens, "Siap {sender}, {date} {time}"); got != "Siap [Nama pengirim], [Tanggal] [Jam]" {
		t.Fatalf("friendly %q", got)
	}
	if !waMissingBody("SIAP!") || waMissingBody("") || waMissingBody("[isi pesan]\nSIAP!") || waMissingBody("{body}") {
		t.Fatal("waMissingBody")
	}
}

// A template saved by an earlier version opens with words, not {body}.
func TestWALegacyTemplateLoads(t *testing.T) {
	h := waHarness(t, 1440, 1400, "connected", true, []bool{true})
	st := &h.app.whatsapp
	conn := *st.connection
	conn.Templates = map[string]string{"reply": "{body}\nSIAP!"}
	h.app.finishWALoad(h.app.gen, st.seq, &conn, st.session, nil, nil, nil, nil, "", nil)
	if got := st.templates[2]; got != "[Isi pesan]\nSIAP!" {
		t.Fatalf("reply shown as %q", got)
	}
	if got := h.app.waTemplatesForServer()["reply"]; got != "{body}\nSIAP!" {
		t.Fatalf("reply saved as %q", got)
	}
}

// The preview opens by itself when a field is typed in, stays when the
// field loses the focus, moves to the next field typed in, and closes
// with Tutup pratinjau. Chips insert their word where the caret is.
func TestWATemplateEditor(t *testing.T) {
	h := waHarness(t, 1440, 1600, "connected", true, []bool{true})
	h.click("Pengaturan lanjutan")
	st := &h.app.whatsapp
	if st.previewOpen != "" {
		t.Fatalf("preview open before typing: %q", st.previewOpen)
	}
	h.want("Sisipkan:")
	if err := h.tt.Click("Balasan perintah di grup"); err != nil {
		t.Fatal(err)
	}
	h.tt.Type("SIAP!")
	h.tt.Frame()
	if st.templates[2] != "SIAP!" || st.previewOpen != "reply|tpl-reply" {
		t.Fatalf("typed %q, preview %q", st.templates[2], st.previewOpen)
	}
	h.want("Contoh di grup WhatsApp")
	h.want("[Isi pesan] belum ada: tanpa itu isi pengingat atau hasil perintah tidak terlihat.")

	// The caret sits after "SIAP"; the chip puts the body there.
	h.tt.Key(0, ui.KeyLeft)
	h.click("Sisipkan Isi pesan ke Balasan perintah di grup")
	h.tt.Frame()
	if st.templates[2] != "SIAP[Isi pesan]!" {
		t.Fatalf("chip inserted as %q", st.templates[2])
	}
	h.dontWant("[Isi pesan] belum ada: tanpa itu isi pengingat atau hasil perintah tidak terlihat.")
	if got := h.app.waTemplatesForServer()["reply"]; got != "SIAP{body}!" {
		t.Fatalf("saved as %q", got)
	}

	// Another field typed in takes the preview over.
	if err := h.tt.Click("Pengingat tenggat"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	if st.previewOpen != "reminder|tpl-reminder" {
		t.Fatalf("preview %q after focusing another field", st.previewOpen)
	}
	h.click("Tutup pratinjau")
	if st.previewOpen != "" {
		t.Fatalf("Tutup pratinjau left %q", st.previewOpen)
	}

	// A template without [Isi pesan] is not saved.
	st.templates[3] = "Laporan minggu ini"
	h.app.saveWAConfig()
	if st.err == "" || st.busy || st.previewOpen != "weekly|tpl-weekly" {
		t.Fatalf("saved without body: err %q busy %v preview %q", st.err, st.busy, st.previewOpen)
	}
}

// A custom command's response is written with chips and previewed with
// example values.
func TestWACommandResponseEditor(t *testing.T) {
	h := waHarness(t, 1440, 1400, "connected", true, []bool{true})
	st := &h.app.whatsapp
	st.connection.Groups = []api.WAGroup{{ID: id.New(), Name: "Mediatek 2025/2026"}}
	st.command = &api.WACustomCommand{ID: id.New(), GroupIDs: []id.ID{}}
	st.commandText = waFriendlyWith(waCommandTokens, "Halo {sender}")
	h.app.chrome.settingsTab = "Command"
	h.app.navigate(PathPengaturan)
	h.tt.Frame()
	h.want("Nama perintah")
	h.want("Balasan bot")
	if err := h.tt.Click("Balasan bot"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	h.click("Sisipkan Tanggal ke Balasan bot")
	h.tt.Frame()
	if st.commandText != "Halo [Nama pengirim][Tanggal]" {
		t.Fatalf("response %q", st.commandText)
	}
	if got := waServerWith(waCommandTokens, st.commandText); got != "Halo {sender}{date}" {
		t.Fatalf("saved as %q", got)
	}
	h.want("Contoh di grup WhatsApp")
}
