package api

import (
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"time"
)

type MedhumContent struct {
	HumasPJName        string         `json:"humasPjName"`
	DesignPJName       string         `json:"designPjName"`
	ID                 id.ID          `json:"id"`
	PeriodID           id.ID          `json:"periodId"`
	SpaceID            id.ID          `json:"spaceId"`
	HumasDivisionID    id.ID          `json:"humasDivisionId"`
	DesignDivisionID   id.ID          `json:"designDivisionId"`
	Title              string         `json:"title"`
	PostingDate        *timeutil.Date `json:"postingDate,omitempty"`
	PostingAt          *time.Time     `json:"postingAt,omitempty"`
	DesignDate         *timeutil.Date `json:"designDate,omitempty"`
	DesignAt           *time.Time     `json:"designAt,omitempty"`
	HumasPJID          id.ID          `json:"humasPjId"`
	DesignPJID         *id.ID         `json:"designPjId,omitempty"`
	HumasTaskID        *id.ID         `json:"humasTaskId,omitempty"`
	DesignTaskID       *id.ID         `json:"designTaskId,omitempty"`
	ConceptURL         string         `json:"conceptUrl"`
	Caption            string         `json:"caption"`
	TrelloCardID       string         `json:"trelloCardId"`
	ConceptSubmittedAt *time.Time     `json:"conceptSubmittedAt,omitempty"`
	PublishedAt        *time.Time     `json:"publishedAt,omitempty"`
	Version            int64          `json:"version"`
	CreatedBy          id.ID          `json:"createdBy"`
	CreatedAt          time.Time      `json:"createdAt"`
	UpdatedAt          time.Time      `json:"updatedAt"`
	CanEditCalendar    bool           `json:"canEditCalendar"`
	CanAssignDesign    bool           `json:"canAssignDesign"`
	// DesignAcc is the shared ACC of the design task's latest submission
	// (PRD MED-13): who, from which side and when. Absent until approved
	// and on servers before 1.0.0-rc.2.
	DesignAcc *MedhumAcc `json:"designAcc,omitempty"`
}

// MedhumAcc records one approval of a Medhum design.
type MedhumAcc struct {
	By   PersonRef `json:"by"`
	Side string    `json:"side"`
	At   time.Time `json:"at"`
}
type MedhumCreateRequest struct {
	Title       string         `json:"title"`
	PostingDate *timeutil.Date `json:"postingDate,omitempty"`
	PostingAt   *time.Time     `json:"postingAt,omitempty"`
	HumasPJID   id.ID          `json:"humasPjId"`
	ConceptURL  string         `json:"conceptUrl"`
	Caption     string         `json:"caption"`
}
type MedhumUpdateRequest struct {
	ExpectedVersion int64          `json:"expectedVersion"`
	Title           *string        `json:"title,omitempty"`
	PostingDate     *timeutil.Date `json:"postingDate,omitempty"`
	PostingAt       *time.Time     `json:"postingAt,omitempty"`
	HumasPJID       *id.ID         `json:"humasPjId,omitempty"`
	DesignPJID      *id.ID         `json:"designPjId,omitempty"`
	ConceptURL      *string        `json:"conceptUrl,omitempty"`
	Caption         *string        `json:"caption,omitempty"`
	TrelloCardID    *string        `json:"trelloCardId,omitempty"`
	PublishedAt     *time.Time     `json:"publishedAt,omitempty"`
	Reason          string         `json:"reason"`
}
