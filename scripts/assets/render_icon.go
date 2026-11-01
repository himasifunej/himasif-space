// Render the canonical vector identity through the same renderer as the app.
// Run from the repository root: go run ./scripts/assets/render_icon.go
package main

import (
	"github.com/egoist/mygo/ui"
	"image/png"
	"os"
)

func main() {
	data, err := os.ReadFile("internal/appui/assets/space.svg")
	if err != nil {
		panic(err)
	}
	vector, err := ui.ParseSVG(data)
	if err != nil {
		panic(err)
	}
	t := ui.NewTester(func(c *ui.Context) {
		theme := ui.LightTheme()
		theme.Background = ui.RGBA(0, 0, 0, 0)
		c.SetTheme(theme)
		ui.Image(c, vector).Size(1024, 1024).Fit(ui.Contain)
	}, 1024, 1024)
	f, err := os.Create("app/resources/icon.png")
	if err != nil {
		panic(err)
	}
	if err = png.Encode(f, t.Image()); err != nil {
		panic(err)
	}
	if err = f.Close(); err != nil {
		panic(err)
	}
}
