package server

import (
	"net/http"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/store"
)

// listArchiveTargets lets a grant manager choose a closed-period space even
// without a membership in that old period. It reveals only selector metadata;
// the normal Space/content endpoints continue to check their own read access.
func (s *Server) listArchiveTargets(w http.ResponseWriter, r *http.Request, p *principal) error {
	return s.read(r, p, api.CapArchivesManage, func(q store.Querier, sc *scope) error {
		targets, err := s.store.ArchiveTargets(r.Context(), q, sc.division.ID)
		if err != nil {
			return err
		}
		writeJSON(w, http.StatusOK, api.List[api.ArchiveTarget]{Items: nonNil(targets)})
		return nil
	})
}
