package appui

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
)

type organizationFixtureScope struct{ period, division id.ID }
type organizationKadivCall struct{ period, division, user id.ID }

// The fixture checks UI intent and publication timing. Server/Postgres tests
// remain responsible for authorization, transaction and storage guarantees.
type organizationFixture struct {
	*fakeAPI
	OrganizationAPI
	WorkspaceAPI
	orgMu                      sync.Mutex
	users                      []api.User
	handovers                  map[id.ID]api.Handover
	spaces                     map[id.ID][]api.Space
	members                    map[organizationFixtureScope][]api.Member
	serverPeriods              map[id.ID]api.Period
	usersCalls, writeCalls     int
	memberCalls                []organizationFixtureScope
	createRequests             []api.CreatePeriodRequest
	kadivCalls                 []organizationKadivCall
	usersErr, writeErr         error
	usersGate, writeGate       chan struct{}
	usersStarted, writeStarted chan struct{}
	memberGates                map[organizationFixtureScope]chan struct{}
	memberStarted              chan organizationFixtureScope
}

func signalOrganization(ch chan struct{}) {
	if ch != nil {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (f *organizationFixture) Periods(context.Context) ([]api.Period, error) {
	f.orgMu.Lock()
	defer f.orgMu.Unlock()
	periods := make([]api.Period, 0, len(f.serverPeriods))
	for _, p := range f.serverPeriods {
		periods = append(periods, p)
	}
	slices.SortFunc(periods, func(a, b api.Period) int {
		if a.StartsOn.Before(b.StartsOn) {
			return -1
		}
		if b.StartsOn.Before(a.StartsOn) {
			return 1
		}
		if a.ID.String() < b.ID.String() {
			return -1
		}
		if a.ID == b.ID {
			return 0
		}
		return 1
	})
	return periods, nil
}

func (f *organizationFixture) Users(context.Context) ([]api.User, error) {
	f.orgMu.Lock()
	f.usersCalls++
	users, err, gate, started := slices.Clone(f.users), f.usersErr, f.usersGate, f.usersStarted
	f.orgMu.Unlock()
	signalOrganization(started)
	if gate != nil {
		<-gate
	}
	return users, err
}

func (f *organizationFixture) GetHandover(_ context.Context, p id.ID) (api.Handover, error) {
	f.orgMu.Lock()
	defer f.orgMu.Unlock()
	if h, ok := f.handovers[p]; ok {
		return h, nil
	}
	return api.Handover{PeriodID: p}, nil
}

func (f *organizationFixture) waitOrganizationWrite() error {
	f.orgMu.Lock()
	f.writeCalls++
	err, gate, started := f.writeErr, f.writeGate, f.writeStarted
	f.orgMu.Unlock()
	signalOrganization(started)
	if gate != nil {
		<-gate
	}
	return err
}

func (f *organizationFixture) CreatePeriod(_ context.Context, req api.CreatePeriodRequest) (api.Period, error) {
	f.orgMu.Lock()
	f.createRequests = append(f.createRequests, req)
	f.orgMu.Unlock()
	if err := f.waitOrganizationWrite(); err != nil {
		return api.Period{}, err
	}
	p := api.Period{ID: id.New(), Label: req.Label, StartsOn: req.StartsOn, EndsOn: req.EndsOn, TimeZone: req.TimeZone, Status: api.PeriodPreparing}
	f.orgMu.Lock()
	f.serverPeriods[p.ID] = p
	f.orgMu.Unlock()
	return p, nil
}

func (f *organizationFixture) ActivatePeriod(_ context.Context, p id.ID) (api.Period, error) {
	return f.changeOrganizationPeriod(p, api.PeriodActive)
}

func (f *organizationFixture) ClosePeriod(_ context.Context, p id.ID) (api.Period, error) {
	return f.changeOrganizationPeriod(p, api.PeriodClosed)
}

func (f *organizationFixture) changeOrganizationPeriod(pid id.ID, status string) (api.Period, error) {
	if err := f.waitOrganizationWrite(); err != nil {
		return api.Period{}, err
	}
	f.orgMu.Lock()
	defer f.orgMu.Unlock()
	p := f.serverPeriods[pid]
	p.Status = status
	f.serverPeriods[pid] = p
	return p, nil
}

func (f *organizationFixture) SetKadiv(_ context.Context, p, d id.ID, req api.SetKadivRequest) (api.Member, error) {
	f.orgMu.Lock()
	f.kadivCalls = append(f.kadivCalls, organizationKadivCall{period: p, division: d, user: req.UserID})
	f.orgMu.Unlock()
	if err := f.waitOrganizationWrite(); err != nil {
		return api.Member{}, err
	}
	f.orgMu.Lock()
	defer f.orgMu.Unlock()
	key := organizationFixtureScope{period: p, division: d}
	var out api.Member
	for i := range f.members[key] {
		m := &f.members[key][i]
		if m.Role == api.RoleKadiv {
			m.Role, m.Roles = api.RoleMember, []string{api.RoleMember}
		}
		if m.UserID == req.UserID {
			m.Role, m.Roles = api.RoleKadiv, []string{api.RoleKadiv}
			out = *m
		}
	}
	return out, nil
}

func (f *organizationFixture) SaveHandover(_ context.Context, p id.ID, req api.HandoverRequest) (api.Handover, error) {
	if err := f.waitOrganizationWrite(); err != nil {
		return api.Handover{}, err
	}
	now := time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC)
	h := api.Handover{PeriodID: p, SuccessorPeriodID: req.SuccessorPeriodID, IncomingLeadership: req.IncomingLeadership, AdminCustodian: req.AdminCustodian, BotManagement: req.BotManagement, ArchivePlan: req.ArchivePlan, RemainingWork: req.RemainingWork, UpdatedAt: &now}
	f.orgMu.Lock()
	f.handovers[p] = h
	f.orgMu.Unlock()
	return h, nil
}

func (f *organizationFixture) Spaces(_ context.Context, p id.ID) ([]api.Space, error) {
	f.orgMu.Lock()
	defer f.orgMu.Unlock()
	spaces := slices.Clone(f.spaces[p])
	for i := range spaces {
		sp := &spaces[i]
		sp.PeriodStatus = f.serverPeriods[p].Status
		sp.ReadOnly = sp.PeriodStatus == api.PeriodClosed
		sp.Roles = slices.Clone(sp.Roles)
		for j := range sp.Roles {
			sp.Roles[j].PeriodStatus, sp.Roles[j].ReadOnly = sp.PeriodStatus, sp.ReadOnly
			if sp.ReadOnly {
				sp.Roles[j].Capabilities = []string{api.CapSpaceRead, api.CapMembersRead}
			}
		}
	}
	return spaces, nil
}

func (f *organizationFixture) Members(_ context.Context, p, d id.ID) ([]api.Member, error) {
	key := organizationFixtureScope{period: p, division: d}
	f.orgMu.Lock()
	f.memberCalls = append(f.memberCalls, key)
	members, gate, started := slices.Clone(f.members[key]), f.memberGates[key], f.memberStarted
	f.orgMu.Unlock()
	if started != nil {
		select {
		case started <- key:
		default:
		}
	}
	if gate != nil {
		<-gate
	}
	return members, nil
}

func (f *organizationFixture) ArchiveGrants(context.Context, id.ID, id.ID) ([]api.ArchiveGrant, error) {
	return nil, nil
}
func (f *organizationFixture) ArchiveTargets(context.Context, id.ID, id.ID) ([]api.ArchiveTarget, error) {
	return nil, nil
}

func settleOrganization(t *testing.T, h *harness) {
	t.Helper()
	for h.app.inflight > 0 {
		select {
		case apply := <-h.queue:
			apply()
		case <-time.After(5 * time.Second):
			t.Fatalf("%d organization calls did not settle", h.app.inflight)
		}
	}
}

func organizationHarness(t *testing.T) (*harness, *organizationFixture) {
	return newOrganizationHarness(t, api.AccountOrganizationAdmin)
}

func newOrganizationHarness(t *testing.T, kind string) (*harness, *organizationFixture) {
	f := &organizationFixture{fakeAPI: newFake(kind), handovers: map[id.ID]api.Handover{}, spaces: map[id.ID][]api.Space{}, members: map[organizationFixtureScope][]api.Member{}, serverPeriods: map[id.ID]api.Period{}, memberGates: map[organizationFixtureScope]chan struct{}{}}
	f.users = []api.User{{ID: id.New(), Kind: api.AccountPerson, DisplayName: "Ayu", Email: "ayu@example.test", EmailVerified: true}, {ID: id.New(), Kind: api.AccountPerson, DisplayName: "Bima", Email: "bima@example.test", EmailVerified: true}}
	for i := range f.periods {
		f.periods[i].TimeZone = "Asia/Jakarta"
		p := f.periods[i]
		f.serverPeriods[p.ID] = p
		for _, d := range f.divisions {
			caps := []string{api.CapSpaceRead, api.CapMembersRead, api.CapArchivesManage, api.CapKadivManage}
			if p.Status == api.PeriodClosed {
				caps = []string{api.CapSpaceRead, api.CapMembersRead}
			}
			f.spaces[p.ID] = append(f.spaces[p.ID], api.Space{ID: id.New(), PeriodID: p.ID, Name: d.Name, Kind: api.SpaceDivision, Divisions: []api.Division{d}, PeriodStatus: p.Status, ReadOnly: p.Status == api.PeriodClosed, Access: api.AccessAdmin, Roles: []api.Access{{DivisionID: d.ID, Roles: []string{api.RoleOrganizationAdmin}, Capabilities: caps, PeriodStatus: p.Status, ReadOnly: p.Status == api.PeriodClosed}}})
			key := organizationFixtureScope{period: p.ID, division: d.ID}
			for index, user := range f.users {
				role := api.RoleMember
				if index == 0 {
					role = api.RoleKadiv
				}
				f.members[key] = append(f.members[key], api.Member{ID: id.New(), UserID: user.ID, PeriodID: p.ID, DivisionID: d.ID, DisplayName: user.DisplayName, Role: role, Roles: []string{role}})
			}
		}
	}
	h := newHarness(t, f.fakeAPI, "")
	h.app.api, h.app.user, h.app.screen, h.app.conn = f, f.user, screenShell, connOnline
	h.app.periods, h.app.divisions = slices.Clone(f.periods), slices.Clone(f.divisions)
	h.app.periodID = f.periods[1].ID
	h.app.access.spaces = slices.Clone(f.spaces[h.app.periodID])
	h.app.ensureOrganization()
	settleOrganization(t, h)
	return h, f
}

func fillOrganizationPeriod(a *App) {
	a.beginPeriod()
	a.organization.create.label, a.organization.create.start, a.organization.create.end = "2027/2028", "2027-01-01", "2027-12-31"
}

func TestOrganizationPersonCannotLoadPrivateDirectoryOrDispatchAdminWrites(t *testing.T) {
	h, f := newOrganizationHarness(t, api.AccountPerson)
	// Displaying admin capabilities in cached metadata must not turn a personal
	// account (including a 45) into the shared organization administrator.
	h.app.organization.period, h.app.organization.loaded = h.app.periodID, true
	h.app.organization.create = &periodDraft{label: "Injected", start: "2027-01-01", end: "2027-12-31", zone: "Asia/Jakarta", divisions: map[id.ID]bool{f.divisions[0].ID: true}}
	h.app.organization.handoverDraft = &handoverDraft{remaining: "Injected"}
	h.app.loadOrganization()
	h.app.loadOrganizationMembers(f.divisions[0].ID)
	h.app.savePeriod()
	h.app.saveHandover()
	h.app.requestOrganizationAction("close", id.Nil)
	h.app.organization.confirm = &organizationConfirmation{action: "close", period: h.app.periodID}
	h.app.applyOrganizationConfirmation()
	settleOrganization(t, h)
	if f.usersCalls != 0 || len(f.memberCalls) != 0 || f.writeCalls != 0 || len(h.app.organization.users) != 0 {
		t.Fatal("personal account fetched the private directory or dispatched organization writes")
	}
}

func TestOrganizationCreateRequiresExplicitDivisionsAndServerAcknowledgement(t *testing.T) {
	h, f := organizationHarness(t)
	fillOrganizationPeriod(h.app)
	draft := h.app.organization.create
	for d := range draft.divisions {
		draft.divisions[d] = false
	}
	h.app.savePeriod()
	if f.writeCalls != 0 || h.app.organization.err == "" {
		t.Fatal("an empty UI selection silently used the backend's all-divisions default")
	}
	draft.divisions[f.divisions[0].ID] = true
	draft.zone = "Asia/Makassar"
	f.writeGate, f.writeStarted = make(chan struct{}), make(chan struct{}, 1)
	f.writeErr = &client.Error{Status: 409, Code: api.CodeConflict, Message: "Nama periode sudah dipakai"}
	oldPeriod, oldCount := h.app.periodID, len(h.app.periods)
	h.app.savePeriod()
	select {
	case <-f.writeStarted:
	case <-time.After(time.Second):
		t.Fatal("create was not dispatched")
	}
	if !h.app.organization.busy || len(h.app.periods) != oldCount || h.app.periodID != oldPeriod || h.app.organization.notice != "" {
		t.Fatal("unacknowledged create altered the period list or reported success")
	}
	close(f.writeGate)
	settleOrganization(t, h)
	if h.app.organization.create != draft || h.app.organization.err == "" || h.app.organization.notice != "" {
		t.Fatal("failed creation discarded its draft or claimed success")
	}
	f.writeErr, f.writeGate = nil, nil
	h.app.savePeriod()
	settleOrganization(t, h)
	if h.app.periodID == oldPeriod || len(h.app.periods) != oldCount+1 || h.app.period().Status != api.PeriodPreparing {
		t.Fatal("acknowledged create did not select the canonical preparing period")
	}
	req := f.createRequests[len(f.createRequests)-1]
	if req.StartsOn.String() != "2027-01-01" || req.EndsOn.String() != "2027-12-31" || req.TimeZone != "Asia/Makassar" || !slices.Equal(req.DivisionIDs, []id.ID{f.divisions[0].ID}) {
		t.Fatalf("creation lost date-only, zone or explicit division selection: %+v", req)
	}
}

func TestOrganizationClosedPeriodStillAllowsCreatingNextPeriod(t *testing.T) {
	h, f := organizationHarness(t)
	h.app.changePeriod(f.periods[0].ID)
	settleOrganization(t, h)
	h.app.ensureOrganization()
	settleOrganization(t, h)
	h.app.organization.handoverDraft.remaining = "Tidak ada pekerjaan tersisa"
	h.app.saveHandover()
	h.app.requestOrganizationAction("kadiv", f.users[1].ID)
	h.app.requestOrganizationAction("close", id.Nil)
	if f.writeCalls != 0 || h.app.organization.confirm != nil {
		t.Fatal("closed period allowed leadership, checklist or status writes")
	}
	fillOrganizationPeriod(h.app)
	h.app.savePeriod()
	settleOrganization(t, h)
	if f.writeCalls != 1 || h.app.period().Status != api.PeriodPreparing || f.serverPeriods[f.periods[0].ID].Status != api.PeriodClosed {
		t.Fatal("creating a successor was blocked or reopened the closed period")
	}
}

func TestOrganizationActivationWaitsAndDoesNotCloseAnotherActivePeriod(t *testing.T) {
	h, f := organizationHarness(t)
	previous := h.app.periodID
	h.app.changePeriod(f.periods[2].ID)
	settleOrganization(t, h)
	h.app.ensureOrganization()
	settleOrganization(t, h)
	f.writeGate, f.writeStarted = make(chan struct{}), make(chan struct{}, 1)
	h.app.requestOrganizationAction("activate", id.Nil)
	h.app.applyOrganizationConfirmation()
	select {
	case <-f.writeStarted:
	case <-time.After(time.Second):
		t.Fatal("activation was not dispatched")
	}
	if h.app.period().Status != api.PeriodPreparing || h.app.organization.notice != "" {
		t.Fatal("activation appeared saved before the server acknowledged it")
	}
	close(f.writeGate)
	settleOrganization(t, h)
	if h.app.period().Status != api.PeriodActive || f.serverPeriods[previous].Status != api.PeriodActive {
		t.Fatal("activation changed the wrong status or silently closed another active period")
	}
}

func TestOrganizationKadivConfirmationKeepsAccountAndScopeAndPreservesRejection(t *testing.T) {
	h, f := organizationHarness(t)
	p, d, selected := h.app.periodID, h.app.organization.division, f.users[1].ID
	h.app.requestOrganizationAction("kadiv", selected)
	if h.app.organization.confirm == nil {
		t.Fatal("verified active candidate could not be confirmed")
	}
	// Changing an open picker after confirmation cannot redirect its intent.
	h.app.organization.kadivUser = f.users[0].ID
	f.writeErr = &client.Error{Status: 409, Code: api.CodeConflict, Message: "Kadiv asli harus mencopot 45 terlebih dahulu."}
	h.app.applyOrganizationConfirmation()
	settleOrganization(t, h)
	if len(f.kadivCalls) != 1 || f.kadivCalls[0] != (organizationKadivCall{period: p, division: d, user: selected}) {
		t.Fatal("confirmation followed a changed picker instead of its approved target")
	}
	if h.app.organization.notice != "" || h.app.organization.err == "" || h.app.organization.members[0].Role != api.RoleKadiv || h.app.organization.confirm == nil {
		t.Fatal("45 conflict optimistically changed leadership or erased the actionable rejection")
	}
	f.writeErr = nil
	h.app.selectOrganizationDivision(f.divisions[1].ID)
	settleOrganization(t, h)
	h.app.applyOrganizationConfirmation()
	if f.writeCalls != 1 || h.app.organization.confirm != nil {
		t.Fatal("switching division reused an old leadership confirmation")
	}
}

func TestOrganizationHandoverFollowsFreshBaselineButKeepsUnsavedEdits(t *testing.T) {
	h, f := organizationHarness(t)
	p := h.app.periodID
	f.handovers[p] = api.Handover{PeriodID: p, IncomingLeadership: "Pengurus baru", RemainingWork: "Laporan terbaru"}
	h.app.loadOrganization()
	settleOrganization(t, h)
	if h.app.organization.handoverDraft.incoming != "Pengurus baru" || h.app.organization.handoverDraft.remaining != "Laporan terbaru" {
		t.Fatal("clean handover editor retained an old server baseline")
	}
	draft := h.app.organization.handoverDraft
	draft.remaining = "Catatan lokal yang belum disimpan"
	f.handovers[p] = api.Handover{PeriodID: p, IncomingLeadership: "Pengurus terkini", RemainingWork: "Versi perangkat lain"}
	h.app.loadOrganization()
	settleOrganization(t, h)
	if h.app.organization.handover.RemainingWork != "Versi perangkat lain" || h.app.organization.handoverDraft != draft || draft.remaining != "Catatan lokal yang belum disimpan" {
		t.Fatal("refresh overwrote unsaved edits or failed to retain the latest server snapshot")
	}
	f.writeErr = &client.Error{Status: 503, Code: api.CodeUnavailable, Message: "Server belum siap"}
	h.app.saveHandover()
	settleOrganization(t, h)
	if h.app.organization.handoverDraft != draft || draft.remaining != "Catatan lokal yang belum disimpan" || h.app.organization.notice != "" {
		t.Fatal("failed handover save lost the local draft or reported success")
	}
}

func TestOrganizationLateReadsAndWritesCannotSurviveAccountReset(t *testing.T) {
	for _, operation := range []string{"directory", "members", "create"} {
		t.Run(operation, func(t *testing.T) {
			h, f := organizationHarness(t)
			gate := make(chan struct{})
			switch operation {
			case "directory":
				f.usersGate, f.usersStarted = gate, make(chan struct{}, 1)
				h.app.loadOrganization()
				select {
				case <-f.usersStarted:
				case <-time.After(time.Second):
					t.Fatal("directory was not requested")
				}
			case "members":
				f.memberStarted = make(chan organizationFixtureScope, 1)
				f.memberGates[organizationFixtureScope{period: h.app.periodID, division: h.app.organization.division}] = gate
				h.app.loadOrganizationMembers(h.app.organization.division)
				select {
				case <-f.memberStarted:
				case <-time.After(time.Second):
					t.Fatal("members were not requested")
				}
			case "create":
				fillOrganizationPeriod(h.app)
				f.writeGate, f.writeStarted = gate, make(chan struct{}, 1)
				h.app.savePeriod()
				select {
				case <-f.writeStarted:
				case <-time.After(time.Second):
					t.Fatal("create was not requested")
				}
			}
			h.app.toLogin("Berpindah akun")
			close(gate)
			settleOrganization(t, h)
			if h.app.screen != screenLogin || len(h.app.organization.users) != 0 || len(h.app.organization.members) != 0 || h.app.organization.create != nil || h.app.organization.notice != "" || h.app.organization.loaded {
				t.Fatal("an old account's response repopulated private administration state")
			}
		})
	}
}

func TestOrganizationLateDivisionAndPeriodReadsStayInTheirOriginalScope(t *testing.T) {
	h, f := organizationHarness(t)
	oldDivision := h.app.organization.division
	oldKey := organizationFixtureScope{period: h.app.periodID, division: oldDivision}
	gate := make(chan struct{})
	f.memberGates[oldKey], f.memberStarted = gate, make(chan organizationFixtureScope, 2)
	h.app.loadOrganizationMembers(oldDivision)
	select {
	case <-f.memberStarted:
	case <-time.After(time.Second):
		t.Fatal("old member scope did not start")
	}
	h.app.selectOrganizationDivision(f.divisions[1].ID)
	close(gate)
	settleOrganization(t, h)
	if h.app.organization.division != f.divisions[1].ID || !h.app.organization.membersLoaded || h.app.organization.members[0].DivisionID != f.divisions[1].ID {
		t.Fatal("old division's members replaced the newly selected directory")
	}
	gate = make(chan struct{})
	f.usersGate, f.usersStarted = gate, make(chan struct{}, 2)
	h.app.loadOrganization()
	select {
	case <-f.usersStarted:
	case <-time.After(time.Second):
		t.Fatal("old period directory did not start")
	}
	newPeriod := f.periods[2].ID
	f.handovers[newPeriod] = api.Handover{PeriodID: newPeriod, RemainingWork: "Periode baru"}
	h.app.changePeriod(newPeriod)
	// A period switch invalidates pending results even before its page renders.
	close(gate)
	settleOrganization(t, h)
	if h.app.organization.handover.PeriodID == newPeriod || h.app.organization.period == newPeriod {
		t.Fatal("old read was accepted as a snapshot of the new period")
	}
	f.usersGate = nil
	h.app.ensureOrganization()
	settleOrganization(t, h)
	if h.app.organization.period != newPeriod || h.app.organization.handover.PeriodID != newPeriod || h.app.organization.handoverDraft.remaining != "Periode baru" {
		t.Fatal("new period did not obtain its own handover after ignoring the stale result")
	}
}

func TestOrganizationAwayAndBackInvalidatesPendingIntentAndClearsBusy(t *testing.T) {
	for _, operation := range []string{"directory", "create"} {
		t.Run(operation, func(t *testing.T) {
			h, f := organizationHarness(t)
			originalPeriod, count := h.app.periodID, len(h.app.periods)
			gate, started := make(chan struct{}), make(chan struct{}, 1)
			if operation == "directory" {
				f.usersGate, f.usersStarted = gate, started
				h.app.loadOrganization()
			} else {
				fillOrganizationPeriod(h.app)
				f.writeGate, f.writeStarted = gate, started
				h.app.savePeriod()
			}
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("pending operation did not start")
			}
			h.app.changePeriod(f.periods[2].ID)
			h.app.changePeriod(originalPeriod)
			close(gate)
			settleOrganization(t, h)
			if h.app.periodID != originalPeriod || len(h.app.periods) != count || h.app.organization.busy || h.app.organization.loading || h.app.organization.loaded || h.app.organization.create != nil || h.app.organization.notice != "" || len(h.app.organization.users) != 0 {
				t.Fatal("returning to a period revived its old request or left administration stuck busy")
			}
			f.usersGate, f.writeGate = nil, nil
			calls := f.usersCalls
			h.app.ensureOrganization()
			settleOrganization(t, h)
			if !h.app.organization.loaded || h.app.organization.period != originalPeriod || f.usersCalls != calls+1 {
				t.Fatal("returning to the period could not obtain a fresh administration snapshot")
			}
		})
	}
}

func TestOrganizationUnavailableEndpointDoesNotBecomeEmptyOrRetryEveryFrame(t *testing.T) {
	h, f := organizationHarness(t)
	f.usersErr = &client.Error{Status: 404, Code: api.CodeNotFound, Message: "Layanan administrasi tidak ditemukan"}
	h.app.resetOrganization()
	h.app.ensureOrganization()
	settleOrganization(t, h)
	for range 5 {
		h.app.ensureOrganization()
	}
	if f.usersCalls != 2 || h.app.organization.loaded || h.app.organization.err == "" || len(h.app.organization.users) != 0 {
		t.Fatal("missing endpoint became a confirmed empty directory or a render retry loop")
	}
}

func TestOrganizationRejectsInvalidOrInactiveLeadershipCandidates(t *testing.T) {
	h, f := organizationHarness(t)
	disabledAt := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	for _, candidate := range []api.User{
		{ID: id.New(), Kind: api.AccountPerson, EmailVerified: false},
		{ID: id.New(), Kind: api.AccountPerson, EmailVerified: true, DisabledAt: &disabledAt},
		{ID: id.New(), Kind: api.AccountOrganizationAdmin, EmailVerified: true},
	} {
		h.app.organization.users = append(h.app.organization.users, candidate)
		h.app.requestOrganizationAction("kadiv", candidate.ID)
		if h.app.organization.confirm != nil {
			t.Fatal("invalid, unverified or disabled candidate reached leadership confirmation")
		}
	}
	if f.writeCalls != 0 {
		t.Fatal("candidate filtering dispatched a mutation")
	}
	// A malformed date is rejected before dispatch, independently of whether
	// a backend would normalize it into a different day.
	fillOrganizationPeriod(h.app)
	h.app.organization.create.start = "2027-02-30"
	h.app.savePeriod()
	if f.writeCalls != 0 || h.app.organization.err == "" {
		t.Fatal("invalid calendar date reached the API")
	}
}

func TestOrganizationReloadObservesStatusChangesFromAnotherDevice(t *testing.T) {
	for _, change := range []struct{ from, to string }{{api.PeriodActive, api.PeriodClosed}, {api.PeriodPreparing, api.PeriodActive}} {
		t.Run(change.from+" to "+change.to, func(t *testing.T) {
			h, f := organizationHarness(t)
			if change.from == api.PeriodPreparing {
				h.app.changePeriod(f.periods[2].ID)
				settleOrganization(t, h)
				h.app.ensureOrganization()
				settleOrganization(t, h)
			}
			pid := h.app.periodID
			if h.app.period().Status != change.from {
				t.Fatalf("unexpected initial period status %q", h.app.period().Status)
			}
			f.orgMu.Lock()
			p := f.serverPeriods[pid]
			p.Status = change.to
			f.serverPeriods[pid] = p
			f.orgMu.Unlock()
			h.app.loadOrganization()
			settleOrganization(t, h)
			if h.app.period().Status != change.to {
				t.Fatalf("manual reload retained stale status %q after server changed it to %q", h.app.period().Status, change.to)
			}
			if change.to == api.PeriodClosed {
				if h.app.organizationPeriodWritable() {
					t.Fatal("a freshly closed period retained write controls")
				}
				for _, space := range h.app.access.spaces {
					if space.PeriodID == pid && !space.ReadOnly {
						t.Fatal("refresh left a closed period's Space marked writable")
					}
				}
			}
		})
	}
}

func TestOrganizationPeriodClosedRejectionChangesReadOnlyStateWithoutLosingDraft(t *testing.T) {
	h, f := organizationHarness(t)
	draft := h.app.organization.handoverDraft
	draft.remaining = "Draf laporan yang perlu dijaga"
	h.app.requestOrganizationAction("kadiv", f.users[1].ID)
	if h.app.organization.confirm == nil {
		t.Fatal("leadership change did not reach confirmation")
	}
	f.orgMu.Lock()
	p := f.serverPeriods[h.app.periodID]
	p.Status = api.PeriodClosed
	f.serverPeriods[p.ID] = p
	f.writeErr = &client.Error{Status: 409, Code: api.CodePeriodClosed, Message: "Periode sudah ditutup."}
	f.orgMu.Unlock()
	h.app.applyOrganizationConfirmation()
	settleOrganization(t, h)
	if h.app.period().Status != api.PeriodClosed || h.app.organizationPeriodWritable() || h.app.organization.confirm != nil {
		t.Fatal("period_closed rejection did not discard confirmation and mark the period read-only")
	}
	if h.app.organization.handoverDraft != draft || draft.remaining != "Draf laporan yang perlu dijaga" || h.app.organization.err == "" || h.app.organization.notice != "" {
		t.Fatal("period closure lost an unsaved checklist or claimed the rejected mutation succeeded")
	}
	for _, space := range h.app.access.spaces {
		if space.PeriodID == p.ID && !space.ReadOnly {
			t.Fatal("period_closed rejection did not refresh Space access")
		}
	}
	before := f.writeCalls
	h.app.saveHandover()
	h.app.requestOrganizationAction("kadiv", f.users[0].ID)
	h.app.applyOrganizationConfirmation()
	settleOrganization(t, h)
	if f.writeCalls != before {
		t.Fatal("read-only transition allowed another leadership or checklist mutation")
	}
}

func TestOrganizationReloadRefreshesExternalKadivChangeAndRejectsOlderMemberSnapshot(t *testing.T) {
	h, f := organizationHarness(t)
	key := organizationFixtureScope{period: h.app.periodID, division: h.app.organization.division}
	gate := make(chan struct{})
	f.memberGates[key], f.memberStarted = gate, make(chan organizationFixtureScope, 1)
	h.app.loadOrganizationMembers(key.division)
	select {
	case <-f.memberStarted:
	case <-time.After(time.Second):
		t.Fatal("old membership snapshot did not start")
	}
	// The old request has captured Ayu as kadiv. Another device changes the
	// leader before the manual reload; its new request must observe Bima.
	f.orgMu.Lock()
	delete(f.memberGates, key)
	for i := range f.members[key] {
		m := &f.members[key][i]
		m.Role, m.Roles = api.RoleMember, []string{api.RoleMember}
		if m.UserID == f.users[1].ID {
			m.Role, m.Roles = api.RoleKadiv, []string{api.RoleKadiv}
		}
	}
	f.orgMu.Unlock()
	h.app.loadOrganization()
	for !h.app.organization.membersLoaded {
		select {
		case apply := <-h.queue:
			apply()
		case <-time.After(5 * time.Second):
			close(gate)
			t.Fatal("manual reload did not obtain a fresh membership snapshot")
		}
	}
	assertCurrentLeader := func() {
		t.Helper()
		var leader id.ID
		for _, m := range h.app.organization.members {
			if m.Role == api.RoleKadiv {
				leader = m.UserID
			}
		}
		if leader != f.users[1].ID {
			t.Fatal("member snapshot did not retain the externally assigned kadiv")
		}
	}
	assertCurrentLeader()
	close(gate)
	settleOrganization(t, h)
	assertCurrentLeader()
}
