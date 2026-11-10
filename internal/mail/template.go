package mail

import (
	"bytes"
	"html/template"
	"strings"
)

// Letter is the content of one of Space's emails, rendered both as plain
// text and as simple HTML in Space's light theme. The final design comes
// from the email canvas; only this file changes then.
type Letter struct {
	To      string
	Subject string
	// Preheader is the line inbox lists show after the subject.
	Preheader string
	Heading   string
	// Paragraphs come before the button, Notes after it, in smaller type.
	Paragraphs []string
	Button     *Button
	Notes      []string
}

// Button is the one action of a letter. Its URL is also written out in
// full, for clients that strip links and for copying.
type Button struct {
	Label string
	URL   string
}

// Message renders the letter.
func (l Letter) Message() Message {
	var text strings.Builder
	text.WriteString("Halo,\n\n")
	for _, p := range l.Paragraphs {
		text.WriteString(p + "\n\n")
	}
	if l.Button != nil {
		text.WriteString(l.Button.Label + ":\n" + l.Button.URL + "\n\n")
	}
	for _, n := range l.Notes {
		text.WriteString(n + "\n\n")
	}
	text.WriteString("— Space HIMASIF")
	var html bytes.Buffer
	if err := letterHTML.Execute(&html, l); err != nil {
		// The template is fixed and its data plain strings: only a bug
		// gets here. Plain text alone still delivers the message.
		return Message{To: l.To, Subject: l.Subject, Text: text.String()}
	}
	return Message{To: l.To, Subject: l.Subject, Text: text.String(), HTML: html.String()}
}

// Colors of Space's light theme (appui appearance panel).
var letterHTML = template.Must(template.New("letter").Parse(`<!doctype html>
<html lang="id">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="color-scheme" content="light">
<title>{{.Subject}}</title>
</head>
<body style="margin:0;padding:0;background:#f6f3ec;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#1b1a16;">
<div style="display:none;max-height:0;overflow:hidden;">{{.Preheader}}</div>
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f6f3ec;">
<tr><td align="center" style="padding:32px 16px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:520px;">
<tr><td style="padding:0 4px 16px 4px;font-size:15px;font-weight:700;letter-spacing:.2px;">Space <span style="color:#8a8270;font-weight:500;">HIMASIF</span></td></tr>
<tr><td style="background:#fffdf8;border:1px solid #e0d8c8;border-radius:12px;padding:28px 28px 24px 28px;">
<h1 style="margin:0 0 16px 0;font-size:20px;line-height:1.3;font-weight:700;">{{.Heading}}</h1>
{{range .Paragraphs}}<p style="margin:0 0 14px 0;font-size:15px;line-height:1.55;">{{.}}</p>
{{end}}{{with .Button}}<table role="presentation" cellpadding="0" cellspacing="0" style="margin:22px 0 18px 0;"><tr><td style="background:#1b1a16;border-radius:8px;">
<a href="{{.URL}}" style="display:inline-block;padding:12px 22px;font-size:15px;font-weight:600;color:#fffdf8;text-decoration:none;">{{.Label}}</a>
</td></tr></table>
<p style="margin:0 0 18px 0;font-size:12px;line-height:1.5;color:#6b6558;word-break:break-all;">Atau buka: <a href="{{.URL}}" style="color:#6b6558;">{{.URL}}</a></p>
{{end}}{{range .Notes}}<p style="margin:0 0 10px 0;font-size:13px;line-height:1.5;color:#6b6558;">{{.}}</p>
{{end}}</td></tr>
<tr><td style="padding:16px 4px 0 4px;font-size:12px;line-height:1.5;color:#8a8270;">Email otomatis dari Space, ruang kerja pengurus HIMASIF. Balasan ke alamat ini tidak dibaca.</td></tr>
</table>
</td></tr>
</table>
</body>
</html>
`))
