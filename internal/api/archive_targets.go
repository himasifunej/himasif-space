package api

import "github.com/himasifunej/himasif-space/internal/id"

// ArchiveTarget is the metadata needed to select an archive grant. Discovering
// a target grants no access to its contents, directories or private objects.
type ArchiveTarget struct {
	SpaceID     id.ID  `json:"spaceId"`
	PeriodID    id.ID  `json:"periodId"`
	PeriodLabel string `json:"periodLabel"`
	SpaceName   string `json:"spaceName"`
	SpaceKind   string `json:"spaceKind"`
}
