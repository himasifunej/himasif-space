package client

import (
	"context"
	"net/http"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func notePath(n id.ID) string { return "/api/notes/" + n.String() }

func (c *Client) SpaceNotes(ctx context.Context, s id.ID) ([]api.Note, error) {
	var out api.List[api.Note]
	err := c.do(ctx, http.MethodGet, "/api/spaces/"+s.String()+"/notes", nil, &out)
	return out.Items, err
}

func (c *Client) GetNote(ctx context.Context, n id.ID) (api.Note, error) {
	var out api.Note
	err := c.do(ctx, http.MethodGet, notePath(n), nil, &out)
	return out, err
}

func (c *Client) CreateNote(ctx context.Context, s id.ID, req api.NoteCreateRequest) (api.Note, error) {
	var out api.Note
	err := c.do(ctx, http.MethodPost, "/api/spaces/"+s.String()+"/notes", req, &out)
	return out, err
}

func (c *Client) UpdateNote(ctx context.Context, n id.ID, req api.NoteWriteRequest) (api.Note, error) {
	var out api.Note
	err := c.do(ctx, http.MethodPut, notePath(n), req, &out)
	return out, err
}

func (c *Client) DeleteNote(ctx context.Context, n id.ID) error {
	return c.do(ctx, http.MethodDelete, notePath(n), nil, nil)
}

func (c *Client) NoteHistory(ctx context.Context, n id.ID) ([]api.NoteEvent, error) {
	var out api.List[api.NoteEvent]
	err := c.do(ctx, http.MethodGet, notePath(n)+"/history", nil, &out)
	return out.Items, err
}

// AcquireNoteLock also renews a lock already held by the caller.
func (c *Client) AcquireNoteLock(ctx context.Context, n id.ID) (api.NoteLock, error) {
	var out api.NoteLock
	err := c.do(ctx, http.MethodPost, notePath(n)+"/lock", nil, &out)
	return out, err
}

func (c *Client) TakeoverNoteLock(ctx context.Context, n id.ID, req api.NoteLockRequest) (api.NoteLock, error) {
	var out api.NoteLock
	err := c.do(ctx, http.MethodPost, notePath(n)+"/lock/takeover", req, &out)
	return out, err
}

func (c *Client) ReleaseNoteLock(ctx context.Context, n id.ID) error {
	return c.do(ctx, http.MethodDelete, notePath(n)+"/lock", nil, nil)
}

func (c *Client) SaveNoteDraft(ctx context.Context, n id.ID, req api.NoteDraftRequest) (api.NoteDraft, error) {
	var out api.NoteDraft
	err := c.do(ctx, http.MethodPut, notePath(n)+"/draft", req, &out)
	return out, err
}

func (c *Client) GetNoteDraft(ctx context.Context, n id.ID) (api.NoteDraft, error) {
	var out api.NoteDraft
	err := c.do(ctx, http.MethodGet, notePath(n)+"/draft", nil, &out)
	return out, err
}

func (c *Client) DeleteNoteDraft(ctx context.Context, n id.ID) error {
	return c.do(ctx, http.MethodDelete, notePath(n)+"/draft", nil, nil)
}

// NoteDrafts lists the caller's recoverable drafts without exposing session IDs.
func (c *Client) NoteDrafts(ctx context.Context, n id.ID) ([]api.NoteDraft, error) {
	var out api.List[api.NoteDraft]
	err := c.do(ctx, http.MethodGet, notePath(n)+"/drafts", nil, &out)
	return out.Items, err
}

func (c *Client) DeleteNoteRecoveryDraft(ctx context.Context, n, draft id.ID) error {
	return c.do(ctx, http.MethodDelete, notePath(n)+"/drafts/"+draft.String(), nil, nil)
}

func (c *Client) AddNoteComment(ctx context.Context, n id.ID, req api.NoteCommentRequest) (api.NoteComment, error) {
	var out api.NoteComment
	err := c.do(ctx, http.MethodPost, notePath(n)+"/comments", req, &out)
	return out, err
}

func (c *Client) AddNoteAttachment(ctx context.Context, n id.ID, req api.NoteAttachmentRequest) (api.NoteAttachment, error) {
	var out api.NoteAttachment
	err := c.do(ctx, http.MethodPost, notePath(n)+"/attachments", req, &out)
	return out, err
}

func (c *Client) DeleteNoteAttachment(ctx context.Context, n, attachment id.ID) error {
	return c.do(ctx, http.MethodDelete, notePath(n)+"/attachments/"+attachment.String(), nil, nil)
}

func (c *Client) NoteLinks(ctx context.Context, n id.ID) ([]api.NoteLink, error) {
	var out api.List[api.NoteLink]
	err := c.do(ctx, http.MethodGet, notePath(n)+"/links", nil, &out)
	return out.Items, err
}

func (c *Client) AddNoteLink(ctx context.Context, n id.ID, req api.NoteLinkRequest) (api.NoteLink, error) {
	var out api.NoteLink
	err := c.do(ctx, http.MethodPost, notePath(n)+"/links", req, &out)
	return out, err
}

func (c *Client) DeleteNoteLink(ctx context.Context, n, link id.ID) error {
	return c.do(ctx, http.MethodDelete, notePath(n)+"/links/"+link.String(), nil, nil)
}

func (c *Client) NoteTemplates(ctx context.Context, s id.ID) ([]api.NoteTemplate, error) {
	var out api.List[api.NoteTemplate]
	err := c.do(ctx, http.MethodGet, "/api/spaces/"+s.String()+"/note-templates", nil, &out)
	return out.Items, err
}

func (c *Client) CreateNoteTemplate(ctx context.Context, s id.ID, req api.NoteTemplateRequest) (api.NoteTemplate, error) {
	var out api.NoteTemplate
	err := c.do(ctx, http.MethodPost, "/api/spaces/"+s.String()+"/note-templates", req, &out)
	return out, err
}

func (c *Client) UpdateNoteTemplate(ctx context.Context, template id.ID, req api.NoteTemplateRequest) (api.NoteTemplate, error) {
	var out api.NoteTemplate
	err := c.do(ctx, http.MethodPut, "/api/note-templates/"+template.String(), req, &out)
	return out, err
}

func (c *Client) DeleteNoteTemplate(ctx context.Context, template id.ID) error {
	return c.do(ctx, http.MethodDelete, "/api/note-templates/"+template.String(), nil, nil)
}
