package appui

import (
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// A command saved with !setgif shows its media as "GIF" in the editor, not
// the server's media type.
func TestCommandEditorNamesGIFMedia(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.SetSize(1440, 900)
	d := h.api.divisions[0]
	s := testAuthorizedSpace(h.app, d)
	s.Roles[0].Capabilities = append(s.Roles[0].Capabilities, api.CapTasksOwn, api.CapWhatsAppManage)
	h.app.access.spaces[0] = s
	command := api.WACustomCommand{ID: id.New(), Trigger: "damn", AssetID: "ab12", MediaType: "gif", GroupIDs: []id.ID{}, Version: 1}
	h.app.whatsapp = whatsappUIState{user: h.app.user.ID, space: s.ID, period: s.PeriodID, division: d.ID, loaded: true, name: "Space", language: "id", commandsEnabled: true, mapSpace: s.ID,
		connection: &api.WAConnection{ID: id.New(), DivisionID: d.ID}, commands: []api.WACustomCommand{command}, command: &command}
	h.app.chrome.settingsTab = "Command"
	h.app.router.Replace(PathPengaturan)
	h.tt.Advance(500 * time.Millisecond)
	h.tt.Frame()
	h.want("Media tersimpan: GIF")
	h.dontWant("Media tersimpan: gif")
	for mediaType, label := range map[string]string{"sticker": "Stiker", "gif": "GIF", "image": "Gambar", "video": "Video"} {
		if got := waMediaLabel(mediaType); got != label {
			t.Errorf("waMediaLabel(%q) = %q, want %q", mediaType, got, label)
		}
	}
}
