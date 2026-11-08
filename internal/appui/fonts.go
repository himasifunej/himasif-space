package appui

// Canvas A sets the interface in IBM Plex Sans and numbers, dates and key
// badges in IBM Plex Mono. Both are embedded (assets/fonts, SIL OFL 1.1,
// unmodified IBM releases) so that Windows, Linux and browsers draw the same
// letters: the desktop app registers the TTFs, the web app the WOFF2s, and
// the theme names the family. Only the weights the canvas uses are shipped:
// sans 400/500/600, mono 400/500; bold text takes the 600.
const (
	FontSans = "IBM Plex Sans"
	FontMono = "IBM Plex Mono"
)

type fontFace struct {
	file, family string
	// weights is the CSS font-weight range the face answers in browsers.
	weights string
}

var fontFaces = []fontFace{
	{"IBMPlexSans-Regular", FontSans, "100 449"},
	{"IBMPlexSans-Medium", FontSans, "450 549"},
	{"IBMPlexSans-SemiBold", FontSans, "550 900"},
	{"IBMPlexMono-Regular", FontMono, "100 449"},
	{"IBMPlexMono-Medium", FontMono, "450 900"},
}
