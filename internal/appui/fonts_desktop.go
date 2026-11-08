//go:build !js

package appui

import (
	"embed"
	"errors"

	"github.com/egoist/mygo/ui"
)

//go:embed assets/fonts/*.ttf
var fontFiles embed.FS

// RegisterFonts adds the embedded Plex fonts to the system's text engine
// (DirectWrite, Core Text or Pango). Call it once, before the first window.
// Each file keeps its own names, so the engine sees one family per typeface
// with its weights and picks the 600 for bold. When it fails, text falls
// back to the system's interface font.
func RegisterFonts() error {
	var errs []error
	for _, f := range fontFaces {
		data, err := fontFiles.ReadFile("assets/fonts/" + f.file + ".ttf")
		if err == nil {
			err = ui.RegisterFont(data, "")
		}
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
