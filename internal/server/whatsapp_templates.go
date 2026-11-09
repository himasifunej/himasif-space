package server

import (
	"github.com/himasifunej/himasif-space/internal/api"
	"strings"
)

func renderWAText(c api.WAConnection, kind, title, body string) string {
	template := c.Templates[kind]
	if template == "" {
		template = "{body}"
	}
	// Body is mandatory even for configurations written by an earlier version;
	// a reply must never hide whether the requested action actually succeeded.
	if !strings.Contains(template, "{body}") {
		template += "\n{body}"
	}
	rendered := strings.NewReplacer("{body}", body, "{title}", title, "{name}", c.DisplayName, "{greeting}", c.Greeting).Replace(template)
	if c.Greeting != "" && !strings.Contains(template, "{greeting}") {
		rendered = c.Greeting + "\n" + rendered
	}
	if c.DisplayName != "" && !strings.Contains(template, "{name}") {
		rendered = c.DisplayName + "\n" + rendered
	}
	runes := []rune(rendered)
	if len(runes) > 14000 {
		return string(runes[:14000]) + "\n" + waTextLanguage(c.Language, "truncated")
	}
	return rendered
}
