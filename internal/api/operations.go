package api

import (
	"github.com/himasifunej/himasif-space/internal/id"
	"time"
)

type WorkerHealth struct {
	Name                   string     `json:"name"`
	State                  string     `json:"state"`
	ActiveInstances        int        `json:"activeInstances"`
	LastProgressAt         *time.Time `json:"lastProgressAt,omitempty"`
	LastSuccessAt          *time.Time `json:"lastSuccessAt,omitempty"`
	LastFailureAt          *time.Time `json:"lastFailureAt,omitempty"`
	ExpectedSilenceSeconds int        `json:"expectedSilenceSeconds"`
}
type SystemOperations struct {
	Build       BuildInfo      `json:"build"`
	Status      string         `json:"status"`
	Version     string         `json:"version"`
	RunMode     string         `json:"runMode"`
	GeneratedAt time.Time      `json:"generatedAt"`
	Workers     []WorkerHealth `json:"workers"`
}
type QueueHealth struct {
	Suppressed  int        `json:"suppressed"`
	Queued      int        `json:"queued"`
	Due         int        `json:"due"`
	Sending     int        `json:"sending"`
	Unknown     int        `json:"unknown"`
	Failed      int        `json:"failed"`
	Accepted    int        `json:"accepted"`
	Delivered   int        `json:"delivered"`
	Read        int        `json:"read"`
	Cancelled   int        `json:"cancelled"`
	OldestDueAt *time.Time `json:"oldestDueAt,omitempty"`
}
type ConnectorHealth struct {
	ID            id.ID      `json:"id"`
	Name          string     `json:"name"`
	Kind          string     `json:"kind"`
	Enabled       bool       `json:"enabled"`
	State         string     `json:"state"`
	LastSuccessAt *time.Time `json:"lastSuccessAt,omitempty"`
	NextRunAt     time.Time  `json:"nextRunAt"`
	ReviewItems   int        `json:"reviewItems"`
}
type WhatsAppHealth struct {
	LastPolledAt  *time.Time  `json:"lastPolledAt,omitempty"`
	ID            id.ID       `json:"id"`
	Name          string      `json:"name"`
	Enabled       bool        `json:"enabled"`
	State         string      `json:"state"`
	LastEventAt   *time.Time  `json:"lastEventAt,omitempty"`
	HasError      bool        `json:"hasError"`
	MappedGroups  int         `json:"mappedGroups"`
	InboundQueued int         `json:"inboundQueued"`
	Queue         QueueHealth `json:"queue"`
}
type DivisionOperations struct {
	PeriodID            id.ID             `json:"periodId"`
	DivisionID          id.ID             `json:"divisionId"`
	DivisionName        string            `json:"divisionName"`
	GeneratedAt         time.Time         `json:"generatedAt"`
	CanReadSources      bool              `json:"canReadSources"`
	CanReadWhatsApp     bool              `json:"canReadWhatsApp"`
	MemberNotifications *QueueHealth      `json:"memberNotifications,omitempty"`
	Connectors          []ConnectorHealth `json:"connectors"`
	WhatsApp            *WhatsAppHealth   `json:"whatsApp,omitempty"`
}
