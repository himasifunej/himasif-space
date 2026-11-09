// Space is the app of Space — Ruang Kerja HIMASIF: the Windows desktop
// app and, compiled to js/wasm, the web app at db.tiyoouw.app. Both draw
// the same native UI (package appui) and talk to the API server.
//
// From the root of the module:
//
//	go tool mygo dev ./app                     # the desktop app
//	go tool mygo dev -web ./app                # in a browser
//	go tool mygo build -platform web ./app     # the static site, in app/build/web
//	go tool mygo build -platform windows/amd64 ./app
package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/himasifunej/himasif-space/internal/appui"
	"github.com/himasifunej/himasif-space/internal/client"
)

// version is the app's version where the build does not stamp one, as in
// a browser or under go run; keep it as mygo.json's.
var version = "0.3.4"

// appVersion returns the version of the app.
func appVersion() string {
	if v := mygo.App.Version(); v != "" {
		return v
	}
	return version
}

func main() {
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	registerDeviceScheme()
	// Canvas A's IBM Plex; without it text keeps the system's font.
	if err := appui.RegisterFonts(); err != nil {
		log.Printf("fonts: %v", err)
	}
	api := client.New(apiURL())
	configureOffline(api)
	mygo.App.WhenReady(func() {
		// The window's Update applies the results of network calls on the
		// main thread; Start runs after win is set.
		var win *mygo.Window
		app := appui.New(api, newTokenStore(), func(fn func()) { win.Update(fn) })
		app.ServerURL = api.BaseURL
		app.ClientName = clientName
		app.Version = appVersion()
		app.PreferencesStore = newPreferencesStore()
		app.NoteDrafts = newNoteDraftStore()
		app.PendingFiles = newPendingFileStore()
		win = mygo.NewWindow(mygo.WindowOptions{
			Title:         "Space — Ruang Kerja HIMASIF",
			TitleBarStyle: mygo.TitleBarHidden,
			Width:         1440,
			Height:        900,
			MinWidth:      380,
			MinHeight:     640,
			StateKey:      "main",
			Content:       ui.View(app.View),
		})
		// Without an explicit window icon Windows can show the runtime's
		// default tile on the taskbar instead of the embedded Space logo.
		_ = win.SetIcon(applicationIcon)
		app.PickFile = func(maxBytes int64, done func(appui.SelectedFile, error)) { pickFile(win, maxBytes, done) }
		app.ExportFile = func(name, mediaType string, data []byte) (string, error) {
			return exportFile(win, name, mediaType, data)
		}
		win.OnClose(func(event *mygo.CloseEvent) {
			if err := app.FlushNoteDraft(); err != nil {
				event.PreventDefault()
				win.Update(func() {})
			}
		})
		attachDevice(win, app)
		app.Start()
		if link := startupAccountLink(); link != "" {
			_ = app.HandleAccountLink(link)
		}
		watchAccountLinks(func(raw string) {
			win.Update(func() { _ = app.HandleAccountLink(raw) })
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
