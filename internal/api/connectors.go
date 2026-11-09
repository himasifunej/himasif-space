package api

import (
	"encoding/json"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"time"
)

type SheetsConfig struct {
	MirrorStage   string         `json:"mirrorStage,omitempty"`
	SpreadsheetID string         `json:"spreadsheetId"`
	Tab           string         `json:"tab"`
	HeaderRow     int            `json:"headerRow"`
	KeyColumn     int            `json:"keyColumn"`
	TitleColumn   int            `json:"titleColumn"`
	PostingColumn int            `json:"postingColumn"`
	HumasColumn   int            `json:"humasColumn"`
	DesignColumn  int            `json:"designColumn"`
	TrelloColumn  int            `json:"trelloColumn"`
	MirrorColumns map[string]int `json:"mirrorColumns,omitempty"`
}
type TrelloConfig struct {
	CardIDs []string `json:"cardIds"`
}
type ConnectorSourceItem struct {
	Key          string         `json:"key"`
	Fingerprint  string         `json:"fingerprint"`
	Row          int            `json:"row"`
	Title        string         `json:"title"`
	PostingDate  *timeutil.Date `json:"postingDate,omitempty"`
	HumasName    string         `json:"humasName"`
	DesignName   string         `json:"designName"`
	TrelloCardID string         `json:"trelloCardId"`
	ConceptURL   string         `json:"conceptUrl"`
	Caption      string         `json:"caption"`
	Issue        string         `json:"issue,omitempty"`
}
type Connector struct {
	ID              id.ID           `json:"id"`
	PeriodID        id.ID           `json:"periodId"`
	DivisionID      id.ID           `json:"divisionId"`
	SpaceID         id.ID           `json:"spaceId"`
	Kind            string          `json:"kind"`
	Name            string          `json:"name"`
	Config          json.RawMessage `json:"config"`
	Enabled         bool            `json:"enabled"`
	Mode            string          `json:"mode"`
	IntervalSeconds int             `json:"intervalSeconds"`
	NextRunAt       time.Time       `json:"nextRunAt"`
	LastSuccessAt   *time.Time      `json:"lastSuccessAt,omitempty"`
	LastError       string          `json:"lastError"`
	Version         int64           `json:"version"`
	CreatedBy       id.ID           `json:"createdBy"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}
type ConnectorRequest struct {
	Kind            string          `json:"kind"`
	Name            string          `json:"name"`
	SpaceID         id.ID           `json:"spaceId"`
	Config          json.RawMessage `json:"config"`
	Enabled         bool            `json:"enabled"`
	Mode            string          `json:"mode"`
	IntervalSeconds int             `json:"intervalSeconds"`
	ExpectedVersion int64           `json:"expectedVersion"`
}
type ConnectorItem struct {
	ID          id.ID               `json:"id"`
	ConnectorID id.ID               `json:"connectorId"`
	Source      ConnectorSourceItem `json:"source"`
	State       string              `json:"state"`
	Issue       string              `json:"issue"`
	ContentID   *id.ID              `json:"contentId,omitempty"`
	FirstSeenAt time.Time           `json:"firstSeenAt"`
	LastSeenAt  time.Time           `json:"lastSeenAt"`
}
type ConnectorRun struct {
	ID           id.ID      `json:"id"`
	ConnectorID  id.ID      `json:"connectorId"`
	StartedAt    time.Time  `json:"startedAt"`
	FinishedAt   *time.Time `json:"finishedAt,omitempty"`
	Status       string     `json:"status"`
	SnapshotHash string     `json:"snapshotHash"`
	ItemCount    int        `json:"itemCount"`
	Error        string     `json:"error"`
}
type ConnectorDecision struct {
	FollowUpdates       bool   `json:"followUpdates"`
	Action              string `json:"action"`
	ContentID           *id.ID `json:"contentId,omitempty"`
	ExpectedFingerprint string `json:"expectedFingerprint"`
	HumasPJID           *id.ID `json:"humasPjId,omitempty"`
	Reason              string `json:"reason"`
}
type ConnectorPersonMapping struct {
	SourceName string `json:"sourceName"`
	DivisionID id.ID  `json:"divisionId"`
	UserID     id.ID  `json:"userId"`
}
