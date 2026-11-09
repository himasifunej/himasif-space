package api

import (
	"encoding/json"
	"time"

	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// Block types of a note's content document (PRD NTE-01).
const (
	BlockHeading      = "heading"
	BlockParagraph    = "paragraph"
	BlockBulletedList = "bulleted_list"
	BlockNumberedList = "numbered_list"
	BlockChecklist    = "checklist"
	BlockTable        = "table"
	BlockDivider      = "divider"
	// Catatan rapat A (tahap C1).
	BlockCallout = "callout" // a highlighted run of text; Variant says which
	BlockToggle  = "toggle"  // a title over child blocks, one level deep
)

// Callout variants: a decision is the "Keputusan" block of meeting notes.
const (
	CalloutInfo     = "info"
	CalloutDecision = "decision"
)

// Mention kinds: a span may point at a person or a task.
const (
	MentionUser = "user"
	MentionTask = "task"
)

// knownBlockType reports whether this build knows a block type. Blocks of
// other types are kept as they came (NoteBlock.Raw).
func knownBlockType(t string) bool {
	switch t {
	case BlockHeading, BlockParagraph, BlockBulletedList, BlockNumberedList, BlockChecklist, BlockTable, BlockDivider, BlockCallout, BlockToggle:
		return true
	}
	return false
}

// Template kinds. meeting/schedule/free are built-in; custom is a saved
// division template (NTE-02, NTE-03, NTE-04).
const (
	TemplateMeeting  = "meeting"
	TemplateSchedule = "schedule"
	TemplateFree     = "free"
	TemplateCustom   = "custom"
)

// Targets a note may link to (NTE-05).
const (
	NoteLinkTask     = "task"
	NoteLinkReminder = "reminder"
)

// Attachment statuses (NTE-09).
const (
	AttachmentPending = "pending"
	AttachmentReady   = "ready"
)

// Span is a run of text with optional emphasis and a link (NTE-01), or a
// mention of a person or a task. A mention keeps its name in Text, so a
// client that does not know mentions still shows it.
type Span struct {
	Text    string   `json:"text"`
	Bold    bool     `json:"bold,omitempty"`
	Link    string   `json:"link,omitempty"`
	Mention *Mention `json:"mention,omitempty"`
}

// Mention points a span at a person (MentionUser) or a task (MentionTask).
type Mention struct {
	Kind string `json:"kind"`
	ID   id.ID  `json:"id"`
}

// NoteBlock is one block of a note document. Which fields are read depends
// on Type (see PRD NTE-01 and the API contract). The server rejects
// unknown types on write; a client keeps them (Raw) so that saving a
// document from a newer server loses nothing.
type NoteBlock struct {
	Type     string         `json:"type"`
	Level    int            `json:"level,omitempty"`    // heading: 1..3
	Text     string         `json:"text,omitempty"`     // heading, toggle title
	Spans    []Span         `json:"spans,omitempty"`    // paragraph, callout
	Items    []NoteListItem `json:"items,omitempty"`    // lists, checklist
	Columns  []string       `json:"columns,omitempty"`  // table header
	Rows     [][][]Span     `json:"rows,omitempty"`     // table cells of spans
	Variant  string         `json:"variant,omitempty"`  // callout: info, decision
	Minutes  int            `json:"minutes,omitempty"`  // heading: planned agenda minutes
	Children []NoteBlock    `json:"children,omitempty"` // toggle: one level deep
	// Raw is a block of a type this build does not know, exactly as it was
	// received; it is written back unchanged. Empty for known types.
	Raw json.RawMessage `json:"-"`
}

// Unsupported reports whether this build cannot show or edit the block.
func (b NoteBlock) Unsupported() bool { return !knownBlockType(b.Type) }

type noteBlockJSON NoteBlock

// UnmarshalJSON reads a known block into its fields and keeps any other
// block whole in Raw.
func (b *NoteBlock) UnmarshalJSON(data []byte) error {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return err
	}
	if !knownBlockType(head.Type) {
		*b = NoteBlock{Type: head.Type, Raw: append(json.RawMessage(nil), data...)}
		return nil
	}
	var v noteBlockJSON
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*b = NoteBlock(v)
	b.Raw = nil
	return nil
}

// MarshalJSON writes a kept unknown block back as it came.
func (b NoteBlock) MarshalJSON() ([]byte, error) {
	if !knownBlockType(b.Type) && len(b.Raw) > 0 {
		return b.Raw, nil
	}
	return json.Marshal(noteBlockJSON(b))
}

// NoteListItem is one item of a list or checklist block. Checked,
// AssigneeID, DueOn and TaskID apply to checklist items (an action item:
// its PJ, due date and the task made from it); Minutes to numbered list
// items (the planned length of an agenda item).
type NoteListItem struct {
	Spans      []Span         `json:"spans"`
	Checked    bool           `json:"checked,omitempty"`
	AssigneeID *id.ID         `json:"assigneeId,omitempty"`
	DueOn      *timeutil.Date `json:"dueOn,omitempty"`
	TaskID     *id.ID         `json:"taskId,omitempty"`
	Minutes    int            `json:"minutes,omitempty"`
}

// Note is a note as the caller sees it. Content is the block document. The
// list form omits Content, Comments and Attachments (see contract).
type Note struct {
	ID              id.ID            `json:"id"`
	SpaceID         id.ID            `json:"spaceId"`
	PeriodID        id.ID            `json:"periodId"`
	OwnerDivisionID id.ID            `json:"ownerDivisionId"`
	Title           string           `json:"title"`
	Content         []NoteBlock      `json:"content,omitempty"`
	TemplateKind    string           `json:"templateKind"`
	MeetingID       *id.ID           `json:"meetingId,omitempty"`
	Version         int64            `json:"version"`
	CreatedBy       PersonRef        `json:"createdBy"`
	CreatedAt       time.Time        `json:"createdAt"`
	UpdatedAt       time.Time        `json:"updatedAt"`
	Lock            *NoteLock        `json:"lock,omitempty"`
	Attachments     []NoteAttachment `json:"attachments,omitempty"`
	Comments        []NoteComment    `json:"comments,omitempty"`
	Permissions     NotePermissions  `json:"permissions"`
}

// NotePermissions tells the client what the caller may do. CanEdit is true
// when the caller may manage and holds (or may take) the editor lock.
type NotePermissions struct {
	UploadFiles        bool  `json:"uploadFiles"`
	MaxAttachmentBytes int64 `json:"maxAttachmentBytes"`
	Manage             bool  `json:"manage"`
	Comment            bool  `json:"comment"`
	CanEdit            bool  `json:"canEdit"`
}

// NoteLock is the single active editor of a note (NTE-06).
type NoteLock struct {
	Editor       PersonRef `json:"editor"`
	AcquiredAt   time.Time `json:"acquiredAt"`
	ExpiresAt    time.Time `json:"expiresAt"`
	HeldByCaller bool      `json:"heldByCaller"`
}

// NoteCreateRequest creates a note. One of TemplateID or Content is used;
// TemplateID copies the template's content (copy-on-create, NTE-09).
type NoteCreateRequest struct {
	Title        string      `json:"title"`
	TemplateID   *id.ID      `json:"templateId,omitempty"`
	TemplateKind string      `json:"templateKind,omitempty"`
	Content      []NoteBlock `json:"content,omitempty"`
}

// NoteWriteRequest replaces a note's title and content. ExpectedVersion
// guards against lost updates (AC-34).
type NoteWriteRequest struct {
	Title           string      `json:"title"`
	Content         []NoteBlock `json:"content"`
	ExpectedVersion int64       `json:"expectedVersion"`
}

// NoteLockRequest optionally forces a takeover of a live lock by a manager.
type NoteLockRequest struct {
	Force bool `json:"force,omitempty"`
}

// NoteDraft is one account-owned recovery copy. ID identifies the copy without
// exposing its authentication session. Other devices' copies survive a save.
// Stale is true when the note has advanced past BaseVersion.
type NoteDraft struct {
	ID             id.ID       `json:"id"`
	CurrentSession bool        `json:"currentSession"`
	Title          string      `json:"title"`
	Content        []NoteBlock `json:"content"`
	BaseVersion    int64       `json:"baseVersion"`
	SavedAt        time.Time   `json:"savedAt"`
	Stale          bool        `json:"stale"`
}

// NoteDraftRequest saves the caller's draft of a note.
type NoteDraftRequest struct {
	Title       string      `json:"title"`
	Content     []NoteBlock `json:"content"`
	BaseVersion int64       `json:"baseVersion"`
}

// NoteComment is one comment on a note (NTE-06).
type NoteComment struct {
	ID        id.ID     `json:"id"`
	Author    PersonRef `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

type NoteCommentRequest struct {
	Body string `json:"body"`
}

// NoteAttachment is attachment metadata (NTE-09). A pending attachment has
// no url yet.
type NoteAttachment struct {
	Stored      bool      `json:"stored"`
	SHA256      string    `json:"sha256,omitempty"`
	ID          id.ID     `json:"id"`
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Status      string    `json:"status"`
	ContentType string    `json:"contentType"`
	SizeBytes   int64     `json:"sizeBytes"`
	AddedBy     PersonRef `json:"addedBy"`
	CreatedAt   time.Time `json:"createdAt"`
}

type NoteAttachmentRequest struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
}

// NoteLink points from a note to a task or reminder it gave rise to
// (NTE-05). It is only a reference; it does not open the note (NTE-07).
type NoteLink struct {
	ID         id.ID     `json:"id"`
	TargetType string    `json:"targetType"`
	TargetID   id.ID     `json:"targetId"`
	CreatedBy  PersonRef `json:"createdBy"`
	CreatedAt  time.Time `json:"createdAt"`
}

type NoteLinkRequest struct {
	TargetType string `json:"targetType"`
	TargetID   id.ID  `json:"targetId"`
}

// NoteTemplate is a template available in a space: a built-in one or a
// saved division template (NTE-02).
type NoteTemplate struct {
	ID        id.ID       `json:"id"`
	SpaceID   *id.ID      `json:"spaceId"`
	Kind      string      `json:"kind"`
	Name      string      `json:"name"`
	Builtin   bool        `json:"builtin"`
	Content   []NoteBlock `json:"content"`
	CreatedAt *time.Time  `json:"createdAt,omitempty"`
	UpdatedAt *time.Time  `json:"updatedAt,omitempty"`
}

type NoteTemplateRequest struct {
	Name    string      `json:"name"`
	Content []NoteBlock `json:"content"`
}

// NoteEvent is one recorded change to a note (AC-33, AC-34).
type NoteEvent struct {
	ID        id.ID     `json:"id"`
	Actor     PersonRef `json:"actor"`
	Action    string    `json:"action"`
	Reason    string    `json:"reason,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	Before    any       `json:"before,omitempty"`
	After     any       `json:"after,omitempty"`
}
