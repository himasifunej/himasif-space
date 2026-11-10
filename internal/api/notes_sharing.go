package api

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// NotePublicLink is a note's live public link, without its token.
type NotePublicLink struct {
	CreatedAt    time.Time  `json:"createdAt"`
	CreatedBy    PersonRef  `json:"createdBy"`
	ViewCount    int        `json:"viewCount"`
	LastViewedAt *time.Time `json:"lastViewedAt,omitempty"`
}

// NotePublicLinkCreated answers making a public link: the only time the
// token, inside URL, is sent.
type NotePublicLinkCreated struct {
	URL  string         `json:"url"`
	Link NotePublicLink `json:"link"`
}

// NoteTagsRequest replaces a note's tags.
type NoteTagsRequest struct {
	Tags []string `json:"tags"`
}

// Bounds of tags.
const (
	MaxNoteTags   = 10
	MaxNoteTagLen = 32
)

// NormalizeNoteTags tidies tags (lowercase, spaces become "-", "#" and
// duplicates dropped) and checks their bounds.
func NormalizeNoteTags(tags []string) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(t), "#")))
		t = strings.Join(strings.Fields(t), "-")
		if t == "" || seen[t] {
			continue
		}
		if utf8.RuneCountInString(t) > MaxNoteTagLen {
			return nil, fmt.Errorf("Tag paling banyak %d huruf.", MaxNoteTagLen)
		}
		for _, r := range t {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
				return nil, fmt.Errorf("Tag hanya boleh huruf, angka, - dan _.")
			}
		}
		seen[t] = true
		out = append(out, t)
	}
	if len(out) > MaxNoteTags {
		return nil, fmt.Errorf("Paling banyak %d tag per catatan.", MaxNoteTags)
	}
	return out, nil
}

// PublicNote is what a public link shows: a whitelist built on the server.
// It holds no user ID, email, NIM, WhatsApp number, role, comment,
// attachment or Space link: mentions are their display text only.
type PublicNote struct {
	Title    string      `json:"title"`
	Division string      `json:"division"`
	Tags     []string    `json:"tags"`
	Content  []NoteBlock `json:"content"`
	Version  int64       `json:"version"`
	// UpdatedAt changes with every autosave, so readers see the latest.
	UpdatedAt time.Time      `json:"updatedAt"`
	Meeting   *PublicMeeting `json:"meeting,omitempty"`
}

// PublicMeeting is the meeting of a public note: when and where only.
type PublicMeeting struct {
	Title    string     `json:"title"`
	StartsAt time.Time  `json:"startsAt"`
	EndsAt   *time.Time `json:"endsAt,omitempty"`
	TimeZone string     `json:"timeZone"`
	Location string     `json:"location,omitempty"`
}
