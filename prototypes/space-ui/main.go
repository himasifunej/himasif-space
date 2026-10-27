// Space UI is a disposable, isolated MyGo UI exploration, not the production app.
package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/egoist/mygo"
)

//go:embed all:frontend
var files embed.FS

func main() {
	variant := flag.String("variant", "A", "UI alternative A, B, C, D or E")
	refined := flag.Bool("refined", false, "open the combined draft 02")
	preview := flag.Bool("preview", false, "serve the same embedded frontend in a browser")
	addr := flag.String("addr", "127.0.0.1:5173", "browser preview address")
	flag.Parse()
	web, err := fs.Sub(files, "frontend")
	if err != nil {
		log.Fatal(err)
	}
	if *preview {
		log.Printf("Space UI preview: http://%s/?variant=%s", *addr, *variant)
		log.Fatal(http.ListenAndServe(*addr, http.FileServer(http.FS(web))))
		return
	}
	key := strings.ToUpper(*variant)
	if len(key) != 1 || key[0] < 'A' || key[0] > 'E' {
		key = "A"
	}
	mygo.SetFrontend(web)
	page := "/?variant=" + key
	hasVariant := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "variant" {
			hasVariant = true
		}
	})
	if *refined || !hasVariant {
		page = "/refined.html"
	}
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:           "Space — UI exploration",
			URL:             page,
			Width:           1440,
			Height:          960,
			MinWidth:        780,
			MinHeight:       640,
			BackgroundColor: "#151411",
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
