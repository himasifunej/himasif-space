package api

import "github.com/himasifunej/himasif-space/internal/id"

// Stage C7: a meeting note's decisions and action items (PJ and due date)
// sent to the division's WhatsApp group by the bot, with a link to the
// note. The editor sees the message before sending it.

// NoteWASummaryRequest asks for the summary (Send false: the preview) or
// sends it. ExpectedVersion is the note version the preview showed.
type NoteWASummaryRequest struct {
	Send            bool  `json:"send"`
	ExpectedVersion int64 `json:"expectedVersion"`
}

// NoteWASummary is the message and where it goes. Problem says why it
// cannot be sent (no group, nothing to summarize); Warning is worth
// knowing but does not stop it (the bot is offline: it goes when back).
type NoteWASummary struct {
	Text      string `json:"text"`
	GroupName string `json:"groupName"`
	Decisions int    `json:"decisions"`
	Actions   int    `json:"actions"`
	Problem   string `json:"problem,omitempty"`
	Warning   string `json:"warning,omitempty"`
	Version   int64  `json:"version"`
	Queued    bool   `json:"queued"`
	OutboxID  *id.ID `json:"outboxId,omitempty"`
}
