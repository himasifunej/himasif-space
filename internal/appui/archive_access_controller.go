package appui

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
)

type archiveAdminUIState struct {
	revealRevoke                                bool
	seq                                         uint64
	period, division, user, space, confirmGrant id.ID
	loaded, loading, busy                       bool
	err, notice, targetsErr                     string
	targetsLoaded                               bool
	access                                      api.Access
	members                                     []api.Member
	grants                                      []api.ArchiveGrant
	targets                                     []api.ArchiveTarget
	userOpen, spaceOpen                         bool
}

func (a *App) resetArchiveAdmin() {
	a.archives = archiveAdminUIState{seq: a.archives.seq + 1}
}
func (a *App) archiveManagedDivisions() []api.Division {
	var ds []api.Division
	if a.screen != screenShell || a.period() == nil || a.period().Status == api.PeriodClosed {
		return ds
	}
	seen := map[id.ID]bool{}
	for _, s := range a.access.spaces {
		if s.PeriodID != a.periodID || s.ReadOnly {
			continue
		}
		for _, r := range s.Roles {
			if r.ReadOnly || r.PeriodStatus == api.PeriodClosed || slices.Contains(r.Roles, api.Role45) || !slices.Contains(r.Capabilities, api.CapArchivesManage) {
				continue
			}
			for _, d := range s.Divisions {
				if d.ID == r.DivisionID && !seen[d.ID] {
					seen[d.ID] = true
					ds = append(ds, d)
				}
			}
		}
	}
	return ds
}
func (a *App) archiveDivisionAllowed(d id.ID) bool {
	return slices.ContainsFunc(a.archiveManagedDivisions(), func(div api.Division) bool { return div.ID == d })
}
func (a *App) ensureArchiveAdmin() {
	if a.archives.period != a.periodID {
		a.resetArchiveAdmin()
		a.archives.period = a.periodID
	}
	if a.archives.division != id.Nil && !a.archiveDivisionAllowed(a.archives.division) {
		a.resetArchiveAdmin()
		a.archives.period = a.periodID
	}
	if a.archives.division == id.Nil && !a.access.loading {
		if ds := a.archiveManagedDivisions(); len(ds) > 0 {
			a.selectArchiveDivision(ds[0].ID)
		}
	}
}
func (a *App) selectArchiveDivision(d id.ID) {
	if a.archives.busy || !a.archiveDivisionAllowed(d) {
		return
	}
	a.resetArchiveAdmin()
	a.archives.period, a.archives.division = a.periodID, d
	a.loadArchiveAdmin()
}
func (a *App) loadArchiveAdmin() {
	service, ok := a.organizationService()
	workspace, hasWorkspace := a.workspaceService()
	if !ok || !hasWorkspace {
		a.archives.err = "Layanan izin arsip belum tersedia."
		return
	}
	d, p := a.archives.division, a.periodID
	if a.archives.loading || a.archives.busy || p != a.archives.period || !a.archiveDivisionAllowed(d) {
		return
	}
	gen, seq := a.gen, a.archives.seq
	a.archives.loading, a.archives.err, a.archives.targetsErr = true, "", ""
	a.async(func() func() {
		access, err := workspace.DivisionAccess(context.Background(), p, d)
		var members []api.Member
		var grants []api.ArchiveGrant
		var targets []api.ArchiveTarget
		var targetErr error
		if err == nil && (access.ReadOnly || access.PeriodStatus == api.PeriodClosed) {
			err = &client.Error{Status: http.StatusConflict, Code: api.CodePeriodClosed, Message: "Periode telah ditutup dan hanya dapat dibaca."}
		} else if err == nil && (slices.Contains(access.Roles, api.Role45) || !slices.Contains(access.Capabilities, api.CapArchivesManage)) {
			err = &client.Error{Status: http.StatusForbidden, Code: api.CodeForbidden, Message: "Kewenangan mengelola arsip tidak lagi tersedia."}
		}
		if err == nil {
			members, err = workspace.Members(context.Background(), p, d)
		}
		if err == nil {
			grants, err = service.ArchiveGrants(context.Background(), p, d)
		}
		if err == nil {
			targets, targetErr = service.ArchiveTargets(context.Background(), p, d)
		}
		return func() {
			if gen != a.gen || seq != a.archives.seq || p != a.periodID || d != a.archives.division {
				return
			}
			a.archives.loading = false
			if err != nil {
				a.archiveAdminFailure(err)
				return
			}
			if targetErr != nil && client.IsUnauthenticated(targetErr) {
				a.failed(targetErr)
				return
			}
			var denied *client.Error
			if errors.As(targetErr, &denied) && (denied.Status == http.StatusForbidden || denied.Code == api.CodePeriodClosed) {
				a.archiveAdminFailure(targetErr)
				return
			}
			a.conn = connOnline
			a.archives.access, a.archives.members, a.archives.grants = access, members, grants
			a.archives.loaded = true
			a.archives.targets, a.archives.targetsLoaded = targets, targetErr == nil
			if targetErr != nil {
				a.archives.targetsErr = a.failed(targetErr)
			}
			if !slices.ContainsFunc(members, func(m api.Member) bool { return m.UserID == a.archives.user }) {
				a.archives.user = id.Nil
			}
			if !slices.ContainsFunc(targets, func(t api.ArchiveTarget) bool { return t.SpaceID == a.archives.space }) {
				a.archives.space = id.Nil
			}
		}
	})
}
func (a *App) archiveWriteBlocked() bool {
	r := a.archives.access
	return a.screen != screenShell || a.conn != connOnline || a.archives.busy || a.archives.loading || !a.archives.loaded || a.access.loading || a.archives.period != a.periodID || !a.archiveDivisionAllowed(a.archives.division) || r.ReadOnly || slices.Contains(r.Roles, api.Role45) || !slices.Contains(r.Capabilities, api.CapArchivesManage)
}
func (a *App) archiveAdminFailure(err error) {
	if client.IsUnauthenticated(err) {
		a.failed(err)
		return
	}
	var e *client.Error
	if errors.As(err, &e) && e.Code == api.CodePeriodClosed {
		a.markPeriodClosed(a.archives.period)
	}
	if errors.As(err, &e) && (e.Status == http.StatusForbidden || e.Code == api.CodePeriodClosed) {
		p, d := a.archives.period, a.archives.division
		a.resetArchiveAdmin()
		a.archives.period, a.archives.division = p, d
	}
	a.archives.err = a.failed(err)
}
func (a *App) writeArchiveAdmin(notice string, work func(OrganizationAPI, id.ID, id.ID) error) {
	s, ok := a.organizationService()
	if !ok || a.archiveWriteBlocked() {
		return
	}
	gen, seq, p, d := a.gen, a.archives.seq, a.archives.period, a.archives.division
	a.archives.busy, a.archives.err, a.archives.notice = true, "", ""
	a.async(func() func() {
		err := work(s, p, d)
		return func() {
			if gen != a.gen || seq != a.archives.seq || p != a.periodID || d != a.archives.division {
				return
			}
			a.archives.busy = false
			if err != nil {
				a.archiveAdminFailure(err)
				return
			}
			a.conn = connOnline
			a.archives.confirmGrant = id.Nil
			a.archives.notice = notice
			a.loadArchiveAdmin()
		}
	})
}
func (a *App) grantArchive() {
	if a.archiveWriteBlocked() || !a.archives.targetsLoaded {
		return
	}
	u, s := a.archives.user, a.archives.space
	if !slices.ContainsFunc(a.archives.members, func(m api.Member) bool { return m.UserID == u }) || !slices.ContainsFunc(a.archives.targets, func(t api.ArchiveTarget) bool { return t.SpaceID == s }) {
		a.archives.err = "Pilih anggota aktif dan Space arsip yang tersedia."
		return
	}
	if slices.ContainsFunc(a.archives.grants, func(g api.ArchiveGrant) bool { return g.UserID == u && g.SpaceID == s }) {
		a.archives.err = "Anggota ini sudah diberi akses ke Space tersebut."
		return
	}
	req := api.ArchiveRequest{UserID: u, SpaceID: s}
	a.writeArchiveAdmin("Izin baca arsip tersimpan untuk anggota dan Space yang dipilih.", func(service OrganizationAPI, p, d id.ID) error {
		_, err := service.GrantArchive(context.Background(), p, d, req)
		return err
	})
}
func (a *App) requestArchiveRevoke(gid id.ID) {
	if !a.archiveWriteBlocked() && slices.ContainsFunc(a.archives.grants, func(g api.ArchiveGrant) bool {
		return g.ID == gid && g.PeriodID == a.archives.period && g.DivisionID == a.archives.division
	}) {
		a.archives.confirmGrant = gid
		a.archives.revealRevoke = true
	}
}
func (a *App) revokeArchive() {
	gid := a.archives.confirmGrant
	if gid == id.Nil || !slices.ContainsFunc(a.archives.grants, func(g api.ArchiveGrant) bool {
		return g.ID == gid && g.PeriodID == a.archives.period && g.DivisionID == a.archives.division
	}) {
		return
	}
	a.writeArchiveAdmin("Pemberian izin arsip dicabut. Hak dari keanggotaan lama tetap terpisah.", func(s OrganizationAPI, p, d id.ID) error { return s.RevokeArchive(context.Background(), p, d, gid) })
}
func (a *App) archiveUserName(uid id.ID) string {
	for _, u := range a.archives.members {
		if u.UserID == uid {
			return u.DisplayName
		}
	}
	return "Akun di luar anggota aktif (" + uid.String() + ")"
}
func (a *App) archiveSpaceName(sid id.ID) string {
	for _, s := range a.archives.targets {
		if s.SpaceID == sid {
			return s.SpaceName + " · " + s.PeriodLabel
		}
	}
	return "Space arsip (" + sid.String() + ")"
}
