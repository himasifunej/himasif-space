package appui

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// OrganizationAPI adds administration to clients without granting it to older
// services. Server authorization remains authoritative for every operation.
type OrganizationAPI interface {
	Periods(context.Context) ([]api.Period, error)
	CreatePeriod(context.Context, api.CreatePeriodRequest) (api.Period, error)
	ActivatePeriod(context.Context, id.ID) (api.Period, error)
	ClosePeriod(context.Context, id.ID) (api.Period, error)
	SetKadiv(context.Context, id.ID, id.ID, api.SetKadivRequest) (api.Member, error)
	Users(context.Context) ([]api.User, error)
	GetHandover(context.Context, id.ID) (api.Handover, error)
	SaveHandover(context.Context, id.ID, api.HandoverRequest) (api.Handover, error)
	ArchiveGrants(context.Context, id.ID, id.ID) ([]api.ArchiveGrant, error)
	ArchiveTargets(context.Context, id.ID, id.ID) ([]api.ArchiveTarget, error)
	GrantArchive(context.Context, id.ID, id.ID, api.ArchiveRequest) (api.ArchiveGrant, error)
	RevokeArchive(context.Context, id.ID, id.ID, id.ID) error
}

var _ OrganizationAPI = (*client.Client)(nil)

type periodDraft struct {
	label, start, end, zone string
	divisions               map[id.ID]bool
}
type handoverDraft struct {
	successor                                    id.ID
	incoming, custodian, bot, archive, remaining string
	source                                       api.Handover
}

func newHandoverDraft(h api.Handover) *handoverDraft {
	d := &handoverDraft{incoming: h.IncomingLeadership, custodian: h.AdminCustodian, bot: h.BotManagement, archive: h.ArchivePlan, remaining: h.RemainingWork, source: h}
	if h.SuccessorPeriodID != nil {
		d.successor = *h.SuccessorPeriodID
	}
	return d
}
func (d *handoverDraft) unchanged() bool {
	if d == nil {
		return false
	}
	old := newHandoverDraft(d.source)
	return d.successor == old.successor && d.incoming == old.incoming && d.custodian == old.custodian && d.bot == old.bot && d.archive == old.archive && d.remaining == old.remaining
}
func (d *handoverDraft) request() api.HandoverRequest {
	r := api.HandoverRequest{IncomingLeadership: d.incoming, AdminCustodian: d.custodian, BotManagement: d.bot, ArchivePlan: d.archive, RemainingWork: d.remaining}
	if d.successor != id.Nil {
		v := d.successor
		r.SuccessorPeriodID = &v
	}
	return r
}

type organizationConfirmation struct {
	action                 string
	period, division, user id.ID
	reveal                 bool
}
type organizationUIState struct {
	seq, memberSeq                uint64
	period                        id.ID
	tab                           string
	loaded, loading, busy         bool
	err, notice                   string
	users                         []api.User
	handover                      api.Handover
	handoverDraft                 *handoverDraft
	create                        *periodDraft
	division, kadivUser           id.ID
	members                       []api.Member
	membersLoaded, membersLoading bool
	membersErr                    string
	userOpen, successorOpen       bool
	confirm                       *organizationConfirmation
}

func (a *App) organizationService() (OrganizationAPI, bool) {
	if c, ok := a.api.(*client.Client); ok {
		return c.AccountSession(), true
	}
	s, ok := a.api.(OrganizationAPI)
	return s, ok
}
func (a *App) resetOrganization() {
	a.organization = organizationUIState{seq: a.organization.seq + 1, tab: "Periode"}
}
func (a *App) ensureOrganization() {
	if !a.isAdmin() {
		return
	}
	if a.organization.period != a.periodID {
		tab := a.organization.tab
		a.resetOrganization()
		a.organization.period = a.periodID
		if tab != "" {
			a.organization.tab = tab
		}
	}
	if a.organization.tab == "" {
		a.organization.tab = "Periode"
	}
	if a.organization.period != id.Nil && !a.organization.loaded && !a.organization.loading && a.organization.err == "" {
		a.loadOrganization()
	}
	if a.organization.division == id.Nil && !a.access.loading {
		if ds := a.organizationDivisions(); len(ds) > 0 {
			a.selectOrganizationDivision(ds[0].ID)
		}
	}
}
func (a *App) loadOrganization() {
	if !a.isAdmin() || a.screen != screenShell || a.organization.loading || a.organization.busy || a.periodID == id.Nil {
		return
	}
	s, ok := a.organizationService()
	if !ok {
		a.organization.err = "Layanan administrasi belum tersedia."
		return
	}
	gen, seq, p := a.gen, a.organization.seq, a.periodID
	a.organization.loading, a.organization.err = true, ""
	// A complete refresh replaces leadership data too. Invalidate an older
	// directory request so it cannot overwrite the refresh with a stale kadiv.
	a.organization.memberSeq++
	a.organization.membersLoaded, a.organization.membersLoading = false, false
	a.organization.membersErr = ""
	a.async(func() func() {
		periods, metadataErr := s.Periods(context.Background())
		err := metadataErr
		var users []api.User
		if err == nil {
			users, err = s.Users(context.Background())
		}
		var handover api.Handover
		if err == nil {
			handover, err = s.GetHandover(context.Background(), p)
		}
		return func() {
			if gen != a.gen || seq != a.organization.seq || p != a.periodID {
				return
			}
			a.organization.loading = false
			if metadataErr == nil {
				a.periods = periods
				if !a.organizationPeriodWritable() {
					a.organization.confirm = nil
				}
				a.loadSpaces()
			}
			if err != nil {
				a.organizationFailure(err)
				return
			}
			a.conn = connOnline
			a.organization.users, a.organization.handover, a.organization.loaded = users, handover, true
			if a.organization.handoverDraft == nil || a.organization.handoverDraft.unchanged() {
				a.organization.handoverDraft = newHandoverDraft(handover)
			}
			if d := a.organization.division; d != id.Nil {
				a.loadOrganizationMembers(d)
			}
		}
	})
}
func (a *App) organizationDivisions() []api.Division {
	if !a.isAdmin() {
		return nil
	}
	var out []api.Division
	seen := map[id.ID]bool{}
	for _, s := range a.access.spaces {
		if s.PeriodID != a.periodID {
			continue
		}
		for _, d := range s.Divisions {
			if !seen[d.ID] {
				seen[d.ID] = true
				out = append(out, d)
			}
		}
	}
	return out
}
func (a *App) selectOrganizationDivision(d id.ID) {
	if a.organization.busy || !slices.ContainsFunc(a.organizationDivisions(), func(div api.Division) bool { return div.ID == d }) {
		return
	}
	a.organization.memberSeq++
	a.organization.division, a.organization.kadivUser = d, id.Nil
	a.organization.members, a.organization.membersLoaded, a.organization.membersLoading = nil, false, false
	a.organization.membersErr = ""
	a.organization.confirm = nil
	a.loadOrganizationMembers(d)
}
func (a *App) loadOrganizationMembers(d id.ID) {
	s, ok := a.workspaceService()
	if !ok {
		a.organization.membersErr = "Direktori anggota belum tersedia."
		return
	}
	if !a.isAdmin() || a.organization.membersLoading || a.organization.busy || d != a.organization.division || !slices.ContainsFunc(a.organizationDivisions(), func(v api.Division) bool { return v.ID == d }) {
		return
	}
	a.organization.memberSeq++
	gen, seq, ms, p := a.gen, a.organization.seq, a.organization.memberSeq, a.periodID
	a.organization.membersLoading, a.organization.membersErr = true, ""
	a.async(func() func() {
		members, err := s.Members(context.Background(), p, d)
		return func() {
			if gen != a.gen || seq != a.organization.seq || ms != a.organization.memberSeq || p != a.periodID || d != a.organization.division {
				return
			}
			a.organization.membersLoading = false
			if err != nil {
				if client.IsUnauthenticated(err) {
					a.failed(err)
					return
				}
				a.organization.members, a.organization.membersLoaded = nil, false
				a.organization.membersErr = a.failed(err)
				return
			}
			a.conn = connOnline
			a.organization.members, a.organization.membersLoaded = members, true
		}
	})
}
func (a *App) organizationWriteBlocked() bool {
	return a.screen != screenShell || !a.isAdmin() || a.conn != connOnline || a.organization.busy || a.organization.loading || a.periodsLoading || a.periodsErr != "" || a.organization.period != a.periodID
}
func (a *App) organizationPeriodWritable() bool {
	p := a.period()
	return p != nil && p.Status != api.PeriodClosed && a.organization.period == p.ID
}
func (a *App) organizationFailure(err error) {
	if client.IsUnauthenticated(err) {
		a.failed(err)
		return
	}
	var e *client.Error
	if errors.As(err, &e) && e.Code == api.CodePeriodClosed {
		a.markPeriodClosed(a.periodID)
		a.organization.confirm = nil
	}
	if errors.As(err, &e) && e.Status == http.StatusForbidden {
		a.resetOrganization()
		a.organization.period = a.periodID
	}
	a.organization.err = a.failed(err)
}

func (a *App) markPeriodClosed(pid id.ID) {
	for i := range a.periods {
		if a.periods[i].ID == pid {
			a.periods[i].Status = api.PeriodClosed
			break
		}
	}
	if pid == a.periodID {
		a.loadSpaces()
	}
}
func (a *App) writeOrganization(notice string, work func(OrganizationAPI) (func(), error)) {
	s, ok := a.organizationService()
	if !ok || a.organizationWriteBlocked() {
		return
	}
	gen, seq, p := a.gen, a.organization.seq, a.periodID
	a.organization.busy, a.organization.err, a.organization.notice = true, "", ""
	a.async(func() func() {
		after, err := work(s)
		return func() {
			if gen != a.gen || seq != a.organization.seq || p != a.periodID {
				return
			}
			a.organization.busy = false
			if err != nil {
				a.organizationFailure(err)
				return
			}
			a.conn = connOnline
			if after != nil {
				after()
			}
			a.organization.notice = notice
		}
	})
}
func (a *App) beginPeriod() {
	if a.organizationWriteBlocked() || a.divisionsLoading || a.divisionsErr != "" {
		return
	}
	d := &periodDraft{zone: timeutil.DefaultZoneName, divisions: map[id.ID]bool{}}
	for _, div := range a.divisions {
		d.divisions[div.ID] = true
	}
	a.organization.create = d
	a.organization.confirm = nil
}
func (a *App) savePeriod() {
	d := a.organization.create
	if d == nil || a.organizationWriteBlocked() || a.divisionsLoading || a.divisionsErr != "" {
		return
	}
	start, err := timeutil.ParseDate(strings.TrimSpace(d.start))
	if err != nil {
		a.organization.err = "Tanggal mulai harus YYYY-MM-DD dan valid."
		return
	}
	end, err := timeutil.ParseDate(strings.TrimSpace(d.end))
	if err != nil || end.Before(start) {
		a.organization.err = "Tanggal selesai harus valid dan tidak sebelum tanggal mulai."
		return
	}
	zone := strings.TrimSpace(d.zone)
	if _, err := timeutil.LoadZone(zone); err != nil {
		a.organization.err = "Zona waktu tidak valid. Gunakan nama seperti Asia/Jakarta."
		return
	}
	label := strings.TrimSpace(d.label)
	if label == "" {
		a.organization.err = "Nama periode wajib diisi."
		return
	}
	var ds []id.ID
	for _, div := range a.divisions {
		if d.divisions[div.ID] {
			ds = append(ds, div.ID)
		}
	}
	if len(ds) == 0 {
		a.organization.err = "Pilih setidaknya satu divisi untuk periode baru."
		return
	}
	req := api.CreatePeriodRequest{Label: label, StartsOn: start, EndsOn: end, TimeZone: zone, DivisionIDs: ds}
	a.writeOrganization("Periode baru tersimpan dalam persiapan. Pengurus dan pekerjaan tidak disalin otomatis.", func(s OrganizationAPI) (func(), error) {
		p, err := s.CreatePeriod(context.Background(), req)
		return func() {
			a.updatePeriod(p)
			a.changePeriod(p.ID)
			a.resetOrganization()
			a.organization.period = p.ID
			a.loadOrganization()
		}, err
	})
}
func (a *App) updatePeriod(p api.Period) {
	for i := range a.periods {
		if a.periods[i].ID == p.ID {
			a.periods[i] = p
			return
		}
	}
	a.periods = append(a.periods, p)
}
func (a *App) saveHandover() {
	d := a.organization.handoverDraft
	if d == nil || !a.organization.loaded || !a.organizationPeriodWritable() || a.organizationWriteBlocked() {
		return
	}
	if strings.TrimSpace(d.remaining) == "" {
		a.organization.err = "Isi pekerjaan tersisa, atau jelaskan bahwa tidak ada pekerjaan tersisa."
		return
	}
	if d.successor != id.Nil && !slices.ContainsFunc(a.periods, func(p api.Period) bool {
		return p.ID == d.successor && p.ID != a.periodID && p.Status != api.PeriodClosed
	}) {
		a.organization.err = "Pilih periode penerus yang belum ditutup."
		return
	}
	req, p := d.request(), a.periodID
	a.writeOrganization("Checklist serah terima tersimpan. Periode dan tugas tidak diubah otomatis.", func(s OrganizationAPI) (func(), error) {
		h, err := s.SaveHandover(context.Background(), p, req)
		return func() { a.organization.handover = h; a.organization.handoverDraft = newHandoverDraft(h) }, err
	})
}
func (a *App) requestOrganizationAction(action string, user id.ID) {
	if a.organizationWriteBlocked() || !a.organizationPeriodWritable() {
		return
	}
	p := a.period()
	switch action {
	case "activate":
		if p.Status != api.PeriodPreparing {
			return
		}
	case "close":
	case "kadiv":
		if !a.organization.loaded || !a.organization.membersLoaded || a.organization.membersLoading {
			return
		}
		if !slices.ContainsFunc(a.organization.users, func(u api.User) bool {
			return u.ID == user && u.Kind == api.AccountPerson && u.EmailVerified && u.DisabledAt == nil
		}) {
			return
		}
		if !slices.ContainsFunc(a.organizationDivisions(), func(d api.Division) bool { return d.ID == a.organization.division }) {
			return
		}
	default:
		return
	}
	a.organization.confirm = &organizationConfirmation{action: action, period: a.periodID, division: a.organization.division, user: user, reveal: true}
}
func (a *App) cancelOrganizationConfirmation() {
	if !a.organization.busy {
		a.organization.confirm = nil
	}
}
func (a *App) applyOrganizationConfirmation() {
	c := a.organization.confirm
	if c == nil || a.organizationWriteBlocked() || !a.organizationPeriodWritable() || c.period != a.periodID {
		return
	}
	confirm := *c
	switch confirm.action {
	case "activate", "close":
		if confirm.action == "activate" && a.period().Status != api.PeriodPreparing {
			return
		}
		a.writeOrganization("Status periode tersimpan di server.", func(s OrganizationAPI) (func(), error) {
			var p api.Period
			var err error
			if confirm.action == "activate" {
				p, err = s.ActivatePeriod(context.Background(), confirm.period)
			} else {
				p, err = s.ClosePeriod(context.Background(), confirm.period)
			}
			return func() { a.updatePeriod(p); a.organization.confirm = nil; a.loadSpaces(); a.loadOrganization() }, err
		})
	case "kadiv":
		if confirm.division != a.organization.division || !a.organization.loaded || !a.organization.membersLoaded || a.organization.membersLoading {
			return
		}
		if !slices.ContainsFunc(a.organization.users, func(u api.User) bool {
			return u.ID == confirm.user && u.Kind == api.AccountPerson && u.EmailVerified && u.DisabledAt == nil
		}) {
			return
		}
		a.writeOrganization("Kadiv tersimpan. Akses privat tidak diwariskan kepada penggantinya.", func(s OrganizationAPI) (func(), error) {
			_, err := s.SetKadiv(context.Background(), confirm.period, confirm.division, api.SetKadivRequest{UserID: confirm.user})
			return func() {
				a.organization.confirm = nil
				a.organization.kadivUser = id.Nil
				a.loadOrganizationMembers(confirm.division)
			}, err
		})
	}
}
func (a *App) organizationUserName(uid id.ID) string {
	for _, u := range a.organization.users {
		if u.ID == uid {
			return u.DisplayName + " · " + u.Email
		}
	}
	return "Akun tidak lagi tersedia"
}
func (a *App) organizationDivisionName(did id.ID) string {
	for _, d := range a.divisions {
		if d.ID == did {
			return d.Name
		}
	}
	return "Divisi tidak tersedia"
}
func (a *App) organizationPeriodName(pid id.ID) string {
	for _, p := range a.periods {
		if p.ID == pid {
			return p.Label
		}
	}
	return "Periode tidak tersedia"
}
