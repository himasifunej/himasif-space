// Space is the app of Space — Ruang Kerja HIMASIF: the Windows desktop
// app and, compiled to js/wasm, the web app at app.himasif.id. Both draw
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
var version = "0.1.0"

// appVersion returns the version of the app.
func appVersion() string {
	if v := mygo.App.Version(); v != "" {
		return v
	}
	return version
}

func main() {
	api := client.New(apiURL())
	mygo.App.WhenReady(func() {
		// The window's Update applies the results of network calls on the
		// main thread; Start runs after win is set.
		var win *mygo.Window
		app := appui.New(api, newTokenStore(), func(fn func()) { win.Update(fn) })
		app.ServerURL = api.BaseURL
		app.ClientName = clientName
		app.Version = appVersion()
		app.PreferencesStore = newPreferencesStore()
		win = mygo.NewWindow(mygo.WindowOptions{
			Title:     "Space — Ruang Kerja HIMASIF",
			Width:     1440,
			Height:    900,
			MinWidth:  880,
			MinHeight: 640,
			StateKey:  "main",
			Content:   ui.View(app.View),
		})
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
