package sheet

import (
	"time"

	"github.com/himasifunej/himasif-space/internal/id"
)

// MassChange is how many rows may change between two copies before Space
// holds the newer one for a person to check: a sheet emptied or pasted
// over by mistake, or by someone with the link.
const MassChange = 30

// Follow is what the device has taken from the sheet: the newest copy
// applied, by its clock. Google serves older copies after newer ones (the
// spike saw a 28-minute-old copy 30 seconds after a fresh one), so a copy
// whose clock is not newer is set aside. A row gone from the sheet stays
// as it was last seen: nothing is deleted in Space.
type Follow struct {
	Clock     time.Time       `json:"clock"`
	ClockText string          `json:"clockText,omitempty"`
	Rows      map[string]Row  `json:"rows"`
	Missing   map[string]bool `json:"missing,omitempty"`
	NoID      []Row           `json:"noId,omitempty"`
	Issues    []Issue         `json:"issues,omitempty"`
	// Held is a newer copy that changes too much at once.
	Held *Snapshot `json:"held,omitempty"`
}

// Outcome is what offering a copy did.
type Outcome struct {
	Kind    string   // OutStale, OutHeld, OutApplied
	Changes []Change // rows whose progress or note changed
	Gone    int      // rows no longer in the sheet
	Held    int      // rows changed, when held
}

const (
	OutStale   = "stale"
	OutHeld    = "held"
	OutApplied = "applied"
)

// Change is a row's change between copies.
type Change struct {
	TaskID   id.ID
	Line     int
	Title    string
	From, To string // progress
	Note     bool   // its note changed
}

// Offer takes a copy if it is newer than the last applied one.
func (f *Follow) Offer(s Snapshot) Outcome {
	if !f.Clock.IsZero() && !s.Clock.After(f.Clock) {
		return Outcome{Kind: OutStale}
	}
	changes, gone := f.diff(s)
	if len(changes)+gone > MassChange {
		held := s
		f.Held = &held
		return Outcome{Kind: OutHeld, Held: len(changes) + gone}
	}
	f.Held = nil
	f.apply(s)
	return Outcome{Kind: OutApplied, Changes: changes, Gone: gone}
}

// AcceptHeld applies the held copy after a person checked it.
func (f *Follow) AcceptHeld() Outcome {
	if f.Held == nil {
		return Outcome{Kind: OutStale}
	}
	s := *f.Held
	changes, gone := f.diff(s)
	f.Held = nil
	if !f.Clock.IsZero() && !s.Clock.After(f.Clock) {
		return Outcome{Kind: OutStale}
	}
	f.apply(s)
	return Outcome{Kind: OutApplied, Changes: changes, Gone: gone}
}

func (f *Follow) diff(s Snapshot) (changes []Change, gone int) {
	present := map[string]bool{}
	for _, r := range s.Rows {
		key := r.TaskID.String()
		present[key] = true
		from, note := r.BaseProgress, ""
		if prev, ok := f.Rows[key]; ok {
			from, note = prev.Progress, prev.Note
		}
		if r.Progress != from || r.Note != note {
			changes = append(changes, Change{TaskID: r.TaskID, Line: r.Line, Title: r.Title, From: from, To: r.Progress, Note: r.Note != note})
		}
	}
	for key := range f.Rows {
		if !present[key] && !f.Missing[key] {
			gone++
		}
	}
	return changes, gone
}

func (f *Follow) apply(s Snapshot) {
	if f.Rows == nil {
		f.Rows = map[string]Row{}
	}
	present := map[string]bool{}
	for _, r := range s.Rows {
		f.Rows[r.TaskID.String()] = r
		present[r.TaskID.String()] = true
	}
	f.Missing = map[string]bool{}
	for key := range f.Rows {
		if !present[key] {
			f.Missing[key] = true
		}
	}
	f.Clock, f.ClockText = s.Clock, s.ClockText
	f.NoID, f.Issues = s.NoID, s.Issues
}

// Row is the row of a task as last applied.
func (f *Follow) Row(tid id.ID) (Row, bool) {
	r, ok := f.Rows[tid.String()]
	return r, ok
}

// Changed are the applied rows the team changed, for reconciliation.
func (f *Follow) Changed() []Row {
	var out []Row
	for _, r := range f.Rows {
		if r.Changed() {
			out = append(out, r)
		}
	}
	return out
}
