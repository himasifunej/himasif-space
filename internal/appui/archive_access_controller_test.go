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

type archiveIntent struct {
	action           string
	period, division id.ID
	grant            id.ID
	request          api.ArchiveRequest
}

// These fixtures exercise UI authority, selected identities, acknowledgement
// timing and stale responses. Server tests exercise the actual grant policy.
type archiveFixture struct {
	*fakeAPI
	OrganizationAPI
	WorkspaceAPI
	mu                      sync.Mutex
	access                  map[id.ID]api.Access
	members                 map[id.ID][]api.Member
	grants                  map[id.ID][]api.ArchiveGrant
	targets                 map[id.ID][]api.ArchiveTarget
	accessErr, targetErr    error
	writeErr                error
	readGate, readStarted   chan struct{}
	writeGate, writeStarted chan struct{}
	accessCalls             int
	intents                 []archiveIntent
}

func (f *archiveFixture) Spaces(context.Context, id.ID) ([]api.Space, error) { return nil, nil }

func (f *archiveFixture) DivisionAccess(_ context.Context, _, d id.ID) (api.Access, error) {
	f.mu.Lock()
	f.accessCalls++
	access, err := f.access[d], f.accessErr
	access.Roles = slices.Clone(access.Roles)
	access.Capabilities = slices.Clone(access.Capabilities)
	gate, started := f.readGate, f.readStarted
	f.mu.Unlock()
	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	if gate != nil {
		<-gate
	}
	return access, err
}

func (f *archiveFixture) Members(_ context.Context, _, d id.ID) ([]api.Member, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.members[d]), nil
}

func (f *archiveFixture) ArchiveGrants(_ context.Context, _, d id.ID) ([]api.ArchiveGrant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.grants[d]), nil
}

func (f *archiveFixture) ArchiveTargets(_ context.Context, _, d id.ID) ([]api.ArchiveTarget, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.targets[d]), f.targetErr
}

func (f *archiveFixture) waitWrite(intent archiveIntent) error {
	f.mu.Lock()
	f.intents = append(f.intents, intent)
	gate, started, err := f.writeGate, f.writeStarted, f.writeErr
	f.mu.Unlock()
	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	if gate != nil {
		<-gate
	}
	return err
}

func (f *archiveFixture) GrantArchive(_ context.Context, p, d id.ID, req api.ArchiveRequest) (api.ArchiveGrant, error) {
	if err := f.waitWrite(archiveIntent{action: "grant", period: p, division: d, request: req}); err != nil {
		return api.ArchiveGrant{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	g := api.ArchiveGrant{ID: id.New(), PeriodID: p, DivisionID: d, UserID: req.UserID, SpaceID: req.SpaceID}
	f.grants[d] = append(f.grants[d], g)
	return g, nil
}

func (f *archiveFixture) RevokeArchive(_ context.Context, p, d, gid id.ID) error {
	if err := f.waitWrite(archiveIntent{action: "revoke", period: p, division: d, grant: gid}); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.grants[d] = slices.DeleteFunc(f.grants[d], func(g api.ArchiveGrant) bool { return g.ID == gid })
	return nil
}

func settleArchiveAdmin(t *testing.T, h *harness) {
	t.Helper()
	for h.app.inflight > 0 {
		select {
		case apply := <-h.queue:
			apply()
		case <-time.After(5 * time.Second):
			t.Fatalf("%d archive requests did not settle", h.app.inflight)
		}
	}
}

func (f *archiveFixture) Periods(context.Context) ([]api.Period, error) {
	return slices.Clone(f.periods), nil
}

func archiveHarness(t *testing.T) (*harness, *archiveFixture, id.ID) {
	f := &archiveFixture{
		fakeAPI: newFake(api.AccountPerson), access: map[id.ID]api.Access{},
		members: map[id.ID][]api.Member{}, grants: map[id.ID][]api.ArchiveGrant{}, targets: map[id.ID][]api.ArchiveTarget{},
	}
	h := newHarness(t, f.fakeAPI, "")
	h.app.api, h.app.user, h.app.screen, h.app.conn = f, f.user, screenShell, connOnline
	h.app.periodID, h.app.periods = f.periods[1].ID, f.periods
	d := f.divisions[0]
	access := api.Access{DivisionID: d.ID, Roles: []string{api.RoleKadiv}, Capabilities: []string{api.CapSpaceRead, api.CapMembersRead, api.CapArchivesManage}, PeriodStatus: api.PeriodActive}
	f.access[d.ID] = access
	f.members[d.ID] = []api.Member{{UserID: id.New(), PeriodID: h.app.periodID, DivisionID: d.ID, DisplayName: "Anggota aktif"}}
	f.targets[d.ID] = []api.ArchiveTarget{{SpaceID: id.New(), PeriodID: f.periods[0].ID, PeriodLabel: f.periods[0].Label, SpaceName: d.Name, SpaceKind: api.SpaceDivision}}
	h.app.access.spaces = []api.Space{{ID: id.New(), PeriodID: h.app.periodID, Name: d.Name, Kind: api.SpaceDivision, Divisions: []api.Division{d}, Roles: []api.Access{access}}}
	h.app.ensureArchiveAdmin()
	settleArchiveAdmin(t, h)
	if !h.app.archives.loaded || !h.app.archives.targetsLoaded {
		t.Fatal("archive fixture did not load")
	}
	return h, f, d.ID
}

func TestArchiveManagerCannotDispatchAs45OrOrdinaryMembers(t *testing.T) {
	for _, role := range []string{api.Role45, api.RoleMember, api.RoleCompanion} {
		t.Run(role, func(t *testing.T) {
			h, f, d := archiveHarness(t)
			h.app.archives.user, h.app.archives.space = f.members[d][0].UserID, f.targets[d][0].SpaceID
			g := api.ArchiveGrant{ID: id.New(), PeriodID: h.app.periodID, DivisionID: d, UserID: h.app.archives.user, SpaceID: h.app.archives.space}
			h.app.archives.grants = []api.ArchiveGrant{g}
			r := &h.app.access.spaces[0].Roles[0]
			r.Roles = []string{role}
			if role == api.Role45 {
				// Even a combined/cached binding with an administrative capability
				// cannot bypass the absolute role-45 restriction.
				r.Roles = []string{api.RoleCompanion, api.Role45}
			} else {
				r.Capabilities = []string{api.CapSpaceRead, api.CapMembersRead}
			}
			h.app.grantArchive()
			h.app.requestArchiveRevoke(g.ID)
			h.app.archives.confirmGrant = g.ID // an obsolete confirmation is insufficient
			h.app.revokeArchive()
			settleArchiveAdmin(t, h)
			if len(f.intents) != 0 || len(h.app.archiveManagedDivisions()) != 0 {
				t.Fatal("non-manager dispatched an archive write or received a management scope")
			}
		})
	}
}

func TestArchiveFreshDivisionAccessOverridesCachedManagement(t *testing.T) {
	h, f, d := archiveHarness(t)
	h.app.archives.user, h.app.archives.space = f.members[d][0].UserID, f.targets[d][0].SpaceID
	f.access[d] = api.Access{DivisionID: d, Roles: []string{api.RoleMember}, Capabilities: []string{api.CapSpaceRead, api.CapMembersRead}}
	h.app.loadArchiveAdmin()
	settleArchiveAdmin(t, h)
	h.app.grantArchive()
	if h.app.archives.loaded || len(h.app.archives.members) != 0 || len(h.app.archives.grants) != 0 || len(h.app.archives.targets) != 0 || h.app.archives.err == "" || len(f.intents) != 0 {
		t.Fatal("revoked server authority left a usable management snapshot")
	}
	if !h.app.archiveDivisionAllowed(d) {
		t.Fatal("test did not retain the cached Space capability it was meant to challenge")
	}
}

func TestArchiveManagementDoesNotSpreadAcrossMedhumDivisions(t *testing.T) {
	h, f, med := archiveHarness(t)
	hum := f.divisions[1]
	s := &h.app.access.spaces[0]
	s.Kind, s.Divisions = api.SpaceShared, []api.Division{f.divisions[0], hum}
	s.Roles = append(s.Roles, api.Access{DivisionID: hum.ID, Roles: []string{api.RoleMember}, Capabilities: []string{api.CapSpaceRead, api.CapMembersRead}})
	if !h.app.archiveDivisionAllowed(med) || h.app.archiveDivisionAllowed(hum.ID) {
		t.Fatal("one Medhum binding granted management of the other division")
	}
	h.app.selectArchiveDivision(hum.ID)
	if h.app.archives.division != med || f.accessCalls != 1 {
		t.Fatal("unauthorized division started a management request")
	}
}

func TestArchiveUnavailableServiceIsNotConfirmedEmptyOrAnAutomaticRetry(t *testing.T) {
	h, f, d := archiveHarness(t)
	h.app.api = f.fakeAPI // an older server supplies no management service
	h.app.resetArchiveAdmin()
	h.app.archives.period, h.app.archives.division = h.app.periodID, d
	for i := 0; i < 3; i++ {
		h.app.loadArchiveAdmin()
		h.app.ensureArchiveAdmin()
	}
	if h.app.archives.loaded || h.app.archives.loading || h.app.archives.targetsLoaded || h.app.archives.err == "" || h.app.inflight != 0 {
		t.Fatal("missing service became an empty success or a request loop")
	}
}

func TestArchiveGrantRequiresOneListedMemberAndTargetAndRejectsDuplicates(t *testing.T) {
	h, f, d := archiveHarness(t)
	u, s := f.members[d][0].UserID, f.targets[d][0].SpaceID
	for _, pair := range [][2]id.ID{{id.New(), s}, {u, id.New()}, {id.Nil, s}, {u, id.Nil}} {
		h.app.archives.user, h.app.archives.space = pair[0], pair[1]
		h.app.grantArchive()
	}
	if len(f.intents) != 0 || h.app.archives.err == "" {
		t.Fatal("arbitrary account/Space identities were accepted")
	}
	h.app.archives.user, h.app.archives.space = u, s
	h.app.grantArchive()
	settleArchiveAdmin(t, h)
	if len(f.intents) != 1 || f.intents[0].request.UserID != u || f.intents[0].request.SpaceID != s || f.intents[0].division != d || f.intents[0].period != h.app.periodID || len(h.app.archives.grants) != 1 {
		t.Fatal("explicit grant did not preserve the selected member, Space and scope")
	}
	h.app.grantArchive()
	if len(f.intents) != 1 || h.app.archives.err == "" {
		t.Fatal("duplicate grant was dispatched")
	}
}

func TestArchiveGrantWaitsForServerAndFailureDoesNotClaimSuccess(t *testing.T) {
	h, f, d := archiveHarness(t)
	h.app.archives.user, h.app.archives.space = f.members[d][0].UserID, f.targets[d][0].SpaceID
	f.writeGate, f.writeStarted = make(chan struct{}), make(chan struct{}, 1)
	f.writeErr = &client.Error{Status: 409, Code: api.CodeConflict, Message: "Izin sudah berubah"}
	h.app.grantArchive()
	select {
	case <-f.writeStarted:
	case <-time.After(time.Second):
		t.Fatal("grant not dispatched")
	}
	if !h.app.archives.busy || h.app.archives.notice != "" || len(h.app.archives.grants) != 0 {
		t.Fatal("pending grant was presented as saved")
	}
	close(f.writeGate)
	settleArchiveAdmin(t, h)
	if h.app.archives.busy || h.app.archives.notice != "" || len(h.app.archives.grants) != 0 || h.app.archives.err == "" || h.app.archives.user == id.Nil || h.app.archives.space == id.Nil {
		t.Fatal("failed grant was claimed successful or lost the selected pair")
	}
}

func TestArchiveRevokeRequiresConfirmationAndKnownScope(t *testing.T) {
	h, f, d := archiveHarness(t)
	known := api.ArchiveGrant{ID: id.New(), PeriodID: h.app.periodID, DivisionID: d, UserID: f.members[d][0].UserID, SpaceID: f.targets[d][0].SpaceID}
	foreign := known
	foreign.ID, foreign.DivisionID = id.New(), id.New()
	f.grants[d], h.app.archives.grants = []api.ArchiveGrant{known, foreign}, []api.ArchiveGrant{known, foreign}
	h.app.revokeArchive()
	h.app.requestArchiveRevoke(id.New())
	h.app.requestArchiveRevoke(foreign.ID)
	h.app.archives.confirmGrant = foreign.ID
	h.app.revokeArchive()
	if len(f.intents) != 0 {
		t.Fatal("unconfirmed, unknown or foreign-scope grant was revoked")
	}
	h.app.archives.confirmGrant = id.Nil
	h.app.requestArchiveRevoke(known.ID)
	if h.app.archives.confirmGrant != known.ID {
		t.Fatal("known grant did not open confirmation")
	}
	f.writeGate, f.writeStarted = make(chan struct{}), make(chan struct{}, 1)
	h.app.revokeArchive()
	select {
	case <-f.writeStarted:
	case <-time.After(time.Second):
		t.Fatal("revoke not dispatched")
	}
	if len(h.app.archives.grants) != 2 || h.app.archives.notice != "" {
		t.Fatal("unacknowledged revoke changed the displayed grant list")
	}
	close(f.writeGate)
	settleArchiveAdmin(t, h)
	if len(f.intents) != 1 || f.intents[0].grant != known.ID || f.intents[0].action != "revoke" || slices.ContainsFunc(h.app.archives.grants, func(g api.ArchiveGrant) bool { return g.ID == known.ID }) || h.app.archives.confirmGrant != id.Nil {
		t.Fatal("acknowledged revoke did not refresh only the selected grant")
	}
}

func TestArchiveMissingTargetSelectorStillAllowsRevokingExistingGrants(t *testing.T) {
	h, f, d := archiveHarness(t)
	g := api.ArchiveGrant{ID: id.New(), PeriodID: h.app.periodID, DivisionID: d, UserID: f.members[d][0].UserID, SpaceID: f.targets[d][0].SpaceID}
	f.grants[d] = []api.ArchiveGrant{g}
	f.targetErr = &client.Error{Status: 404, Code: api.CodeNotFound, Message: "Selector belum tersedia"}
	h.app.loadArchiveAdmin()
	settleArchiveAdmin(t, h)
	if !h.app.archives.loaded || h.app.archives.targetsLoaded || h.app.archives.targetsErr == "" || len(h.app.archives.grants) != 1 || h.app.archiveWriteBlocked() {
		t.Fatal("missing selector disabled existing-grant administration")
	}
	h.app.archives.user, h.app.archives.space = g.UserID, g.SpaceID
	h.app.grantArchive()
	if len(f.intents) != 0 {
		t.Fatal("grant used targets from an unsuccessful selector request")
	}
	h.app.requestArchiveRevoke(g.ID)
	h.app.revokeArchive()
	settleArchiveAdmin(t, h)
	if len(f.intents) != 1 || f.intents[0].action != "revoke" || len(h.app.archives.grants) != 0 {
		t.Fatal("existing grant could not be revoked without target discovery")
	}
}

func TestArchiveRepliesCannotSurviveScopeOrAccountChanges(t *testing.T) {
	for _, reset := range []string{"period", "division", "account", "access"} {
		t.Run(reset, func(t *testing.T) {
			h, f, original := archiveHarness(t)
			f.readGate, f.readStarted = make(chan struct{}), make(chan struct{}, 1)
			h.app.loadArchiveAdmin()
			select {
			case <-f.readStarted:
			case <-time.After(time.Second):
				t.Fatal("reload not started")
			}
			switch reset {
			case "period":
				h.app.changePeriod(f.periods[2].ID)
			case "division":
				d := f.divisions[1]
				access := api.Access{DivisionID: d.ID, Roles: []string{api.RoleKadiv}, Capabilities: []string{api.CapArchivesManage}}
				f.access[d.ID] = access
				h.app.access.spaces = append(h.app.access.spaces, api.Space{ID: id.New(), PeriodID: h.app.periodID, Divisions: []api.Division{d}, Roles: []api.Access{access}})
				h.app.selectArchiveDivision(d.ID)
			case "account":
				h.app.toLogin("")
			case "access":
				h.app.resetAccess()
			}
			close(f.readGate)
			settleArchiveAdmin(t, h)
			if reset == "division" {
				if h.app.archives.division == original || len(h.app.archives.members) != 0 || len(h.app.archives.targets) != 0 {
					t.Fatal("old division populated a new division's management state")
				}
			} else if h.app.archives.loaded || len(h.app.archives.members) != 0 || len(h.app.archives.targets) != 0 || len(h.app.archives.grants) != 0 {
				t.Fatal("old request repopulated a reset management snapshot")
			}
		})
	}
}

func TestArchiveAccessResetInvalidatesPendingWriteAcknowledgement(t *testing.T) {
	h, f, d := archiveHarness(t)
	h.app.archives.user, h.app.archives.space = f.members[d][0].UserID, f.targets[d][0].SpaceID
	f.writeGate, f.writeStarted = make(chan struct{}), make(chan struct{}, 1)
	h.app.grantArchive()
	select {
	case <-f.writeStarted:
	case <-time.After(time.Second):
		t.Fatal("grant not started")
	}
	h.app.resetAccess()
	close(f.writeGate)
	settleArchiveAdmin(t, h)
	if h.app.archives.notice != "" || h.app.archives.loaded || len(h.app.archives.grants) != 0 || h.app.archives.busy {
		t.Fatal("an earlier intent published success or private state after access reset")
	}
}
