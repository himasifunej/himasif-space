package main

import (
	"encoding/base64"
	"encoding/json"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"net/url"
)

func notificationLink(uid id.ID, d api.Delivery, action string) string {
	// Protocol URLs may be passed through the shell and command line. Keep
	// only navigation identities, never notification text or receipt metadata.
	target := struct {
		ID          id.ID  `json:"id"`
		SourceKind  string `json:"sourceKind"`
		SourceID    id.ID  `json:"sourceId"`
		RelatedKind string `json:"relatedKind,omitempty"`
		RelatedID   *id.ID `json:"relatedId,omitempty"`
		SpaceID     *id.ID `json:"spaceId,omitempty"`
	}{d.ID, d.SourceKind, d.SourceID, d.RelatedKind, d.RelatedID, d.SpaceID}
	data, _ := json.Marshal(target)
	v := url.Values{"account": {uid.String()}, "payload": {base64.RawURLEncoding.EncodeToString(data)}, "action": {action}}
	return "himasif-space://notification?" + v.Encode()
}
