package client

import (
	"context"
	"net/http"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// CreateNotePublicLink makes a note public, or replaces its link. The URL
// in the answer is the only time the token is sent.
func (c *Client) CreateNotePublicLink(ctx context.Context, nid id.ID) (api.NotePublicLinkCreated, error) {
	var out api.NotePublicLinkCreated
	err := c.do(ctx, http.MethodPost, "/api/notes/"+nid.String()+"/public-link", nil, &out)
	return out, err
}

// RevokeNotePublicLink makes a note readable by its division only again.
func (c *Client) RevokeNotePublicLink(ctx context.Context, nid id.ID) error {
	return c.do(ctx, http.MethodDelete, "/api/notes/"+nid.String()+"/public-link", nil, nil)
}

// SetNoteTags replaces a note's tags and returns them as saved.
func (c *Client) SetNoteTags(ctx context.Context, nid id.ID, tags []string) ([]string, error) {
	var out api.NoteTagsRequest
	err := c.do(ctx, http.MethodPut, "/api/notes/"+nid.String()+"/tags", api.NoteTagsRequest{Tags: tags}, &out)
	return out.Tags, err
}
