package appui

import (
	"regexp"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// The texts the bot sends are edited with words people know instead of
// the server's {placeholders}: "Sisipkan:" chips put [Nama pengirim] or
// [Tanggal] where the caret is, the words are coloured in the field, and
// saving turns them back into the placeholders. Templates saved with
// {placeholders} keep working: they open with the words, and a
// {placeholder} typed by hand is saved as it is.

// waToken is a value the bot fills in: the server's placeholder and the
// label people see.
type waToken struct{ token, label, hint string }

func (t waToken) word() string { return "[" + t.label + "]" }

// waTokens are the values of the messages' templates; [Isi pesan] must be
// in each.
var waTokens = []waToken{
	{"{body}", "Isi pesan", "Isi pengingat atau hasil perintah. Wajib ada."},
	{"{name}", "Nama bot", "Nama bot dari pengaturan ini."},
	{"{greeting}", "Sapaan", "Sapaan dari pengaturan ini."},
	{"{title}", "Judul", "Judul pesan, misalnya nama tugas."},
}

// waCommandTokens are the values of a custom command's response.
var waCommandTokens = []waToken{
	{"{sender}", "Nama pengirim", "Nama anggota yang mengetik perintah."},
	{"{date}", "Tanggal", "Tanggal hari ini, misalnya 11 Nov 2026."},
	{"{time}", "Jam", "Jam saat perintah diterima, misalnya 14.30."},
	{"{group}", "Nama grup", "Nama grup WhatsApp tempat perintah diketik."},
	{"{args}", "Teks setelah perintah", "Yang diketik setelah perintah: \"!halo besok\" berisi \"besok\"."},
	{"{name}", "Nama bot", "Nama bot divisi."},
}

// waBodyToken is the one value a template cannot leave out.
var waBodyToken = waTokens[0]

// waFriendly shows a template with words instead of placeholders.
func waFriendly(s string) string { return waFriendlyWith(waTokens, s) }

// waServerTemplate turns the words back into the server's placeholders.
func waServerTemplate(s string) string { return waServerWith(waTokens, s) }

func waFriendlyWith(tokens []waToken, s string) string {
	for _, t := range tokens {
		s = strings.ReplaceAll(s, t.token, t.word())
	}
	return s
}

var waWordPattern = regexp.MustCompile(`\[([^\[\]\n]{1,40})\]`)

// waServerWith turns words into placeholders, whatever their case: one
// typing [nama pengirim] by hand means the same. Other brackets stay.
func waServerWith(tokens []waToken, s string) string {
	return waWordPattern.ReplaceAllStringFunc(s, func(w string) string {
		label := strings.TrimSpace(w[1 : len(w)-1])
		for _, t := range tokens {
			if strings.EqualFold(t.label, label) {
				return t.token
			}
		}
		return w
	})
}

// waMissingBody reports whether a template, as typed, would hide the
// message's content: it is not empty and has no [Isi pesan].
func waMissingBody(s string) bool {
	return strings.TrimSpace(s) != "" && !strings.Contains(waServerTemplate(s), waBodyToken.token)
}

// waEditorState keeps a text field's selection while a chip takes the
// focus, and where to put the caret back.
type waEditorState struct {
	start, end int
	known      bool
	focused    bool
	caret      int
	refocus    bool
}

func (a *App) waEditor(key string) *waEditorState {
	st := &a.whatsapp
	if st.editors == nil {
		st.editors = map[string]*waEditorState{}
	}
	e := st.editors[key]
	if e == nil {
		e = &waEditorState{}
		st.editors[key] = e
	}
	return e
}

// waTemplateEditor is a text the bot sends: the field, coloured words,
// the "Sisipkan:" chips and, for a template, the warning when [Isi pesan]
// is missing. preview names the preview this field opens, bubble draws
// it; it opens while
// the field has the focus or is typed in, and stays until closed or until
// another field opens its own.
func (a *App) waTemplateEditor(c *ui.Context, key, label string, value *string, tokens []waToken, multiline bool, preview string, bubble func()) {
	t := c.Theme()
	st := &a.whatsapp
	ed := a.waEditor(key)
	ui.Column(c.Key(key)).FillWidth().MinWidth(0).Gap(6).Children(func() {
		var field ui.Element
		ui.Field(c, label, func() {
			if multiline {
				field = ui.TextArea(c.Key("text"), value).Height(84)
			} else {
				field = ui.TextInput(c.Key("text"), value)
			}
			field.Label(label).Placeholder("Kosongkan untuk teks bawaan").FillWidth()
		})
		// Colour the words the bot fills in, as chips in the text.
		var ranges []ui.TextRange
		runes := []rune(*value)
		for _, m := range waWordPattern.FindAllStringIndex(*value, -1) {
			w := (*value)[m[0]:m[1]]
			if waServerWith(tokens, w) == w {
				continue
			}
			start := len([]rune((*value)[:m[0]]))
			ranges = append(ranges, ui.TextRange{Start: start, End: start + len([]rune(w)), Color: t.Accent, Weight: 600})
		}
		if len(ranges) > 0 {
			field.TextRanges(ranges...)
		}
		if ed.refocus {
			ed.refocus = false
			field.Focus()
			field.SetTextSelection(ed.caret, ed.caret)
		}
		focused := field.Focused()
		if focused {
			ed.start, ed.end = field.TextSelection()
			ed.known = true
		}
		if preview != "" && (focused && !ed.focused || field.Changed()) {
			st.previewOpen = preview
		}
		ed.focused = focused
		ui.Row(c).Wrap().Gap(6).AlignItems(ui.Center).Children(func() {
			if len(tokens) > 0 {
				muted(c, "Sisipkan:").FontSize(12)
			}
			for _, tk := range tokens {
				b := spaceButton(c.Key("ins"+tk.token), tk.label, "plus", false).Label("Sisipkan "+tk.label+" ke "+label).Tooltip(tk.hint).Padding(2, 8).MinHeight(26)
				if b.Clicked() {
					s, e := len(runes), len(runes)
					if ed.known {
						s, e = min(ed.start, ed.end, len(runes)), min(max(ed.start, ed.end), len(runes))
					}
					word := []rune(tk.word())
					*value = string(runes[:s]) + string(word) + string(runes[e:])
					ed.caret, ed.refocus = s+len(word), true
					ed.start, ed.end, ed.known = ed.caret, ed.caret, true
					if preview != "" {
						st.previewOpen = preview
					}
					c.After(0)
				}
			}
			if preview != "" {
				ui.Box(c).Grow(1)
				a.waPreviewButton(c, preview)
			}
		})
		if len(tokens) > 0 && tokens[0] == waBodyToken && waMissingBody(*value) {
			ui.Row(c.Key("missing-body")).FillWidth().Wrap().Gap(8).AlignItems(ui.Center).Padding(8, 10).Radius(8).Background(t.SurfaceHover).Children(func() {
				ui.Icon(c, spaceIcon("alert")).Size(14, 14).TextColor(t.Danger)
				ui.Text(c, "[Isi pesan] belum ada: tanpa itu isi pengingat atau hasil perintah tidak terlihat.").FontSize(12).Grow(1).MinWidth(0)
				if spaceButton(c.Key("add-body"), "Tambahkan [Isi pesan]", "plus", false).Padding(2, 8).MinHeight(26).Clicked() {
					*value = strings.TrimRight(*value, "\n") + "\n" + waBodyToken.word()
					st.previewOpen = preview
				}
			})
		}
		if preview != "" && st.previewOpen == preview && bubble != nil {
			bubble()
		}
	})
}

// waCommandPreview shows a custom command's response filled with example
// values, as the bot would send it.
func waCommandPreview(c *ui.Context, response string, now time.Time) {
	text := waServerWith(waCommandTokens, response)
	if strings.TrimSpace(text) == "" {
		text = "(Respons kosong: hanya media yang dikirim.)"
	}
	months := [...]string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}
	text = strings.NewReplacer(
		"{sender}", "Aruna",
		"{date}", now.Format("2")+" "+months[now.Month()-1]+" "+now.Format("2006"),
		"{time}", now.Format("15.04"),
		"{group}", "Grup divisi",
		"{args}", "besok pagi",
		"{name}", "Bot divisi",
	).Replace(text)
	waBubble(c, text, "", false)
}
