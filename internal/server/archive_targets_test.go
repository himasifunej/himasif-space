package server

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// A new kadiv must be able to select an old Space without already possessing
// its contents. Selector discovery and archive read permission are separate.
func TestArchiveTargetsDiscoverMetadataWithoutGrantingReadAccess(t *testing.T) {
	o := newOrg(t)
	oldKadiv, _ := o.kadiv("mediatek", "old-archive-kadiv@himasif.test")
	var private api.Reminder
	if code := o.call("POST", "/api/reminders", oldKadiv, dateReq("Private old reminder", "2026-11-10"), &private); code != http.StatusCreated {
		t.Fatalf("create private reminder = %d", code)
	}
	var oldSpaces api.List[api.Space]
	if code := o.call("GET", "/api/periods/"+o.period.ID.String()+"/spaces", o.admin, nil, &oldSpaces); code != http.StatusOK {
		t.Fatalf("old spaces = %d", code)
	}
	if code := o.call("POST", "/api/periods/"+o.period.ID.String()+"/close", o.admin, nil, nil); code != http.StatusOK {
		t.Fatalf("close old period = %d", code)
	}
	var current api.Period
	if code := o.call("POST", "/api/periods", o.admin, api.CreatePeriodRequest{
		Label: "2026/2027", StartsOn: o.period.EndsOn.AddDays(1), EndsOn: o.period.EndsOn.AddDays(365),
	}, &current); code != http.StatusCreated {
		t.Fatalf("create new period = %d", code)
	}
	if code := o.call("POST", "/api/periods/"+current.ID.String()+"/activate", o.admin, nil, nil); code != http.StatusOK {
		t.Fatalf("activate new period = %d", code)
	}
	// Another preparing period must not enter the selector either.
	if code := o.call("POST", "/api/periods", o.admin, api.CreatePeriodRequest{
		Label: "2027/2028", StartsOn: current.EndsOn.AddDays(1), EndsOn: current.EndsOn.AddDays(365),
	}, nil); code != http.StatusCreated {
		t.Fatalf("create future period = %d", code)
	}
	kadiv, person := o.join(o.admin, current.ID, "mediatek", "new-archive-kadiv@himasif.test", "New Kadiv")
	currentScope := o.scope(current.ID, "mediatek")
	if code := o.call("PUT", currentScope+"/kadiv", o.admin, api.SetKadivRequest{UserID: person.User.ID}, nil); code != http.StatusOK {
		t.Fatalf("set new kadiv = %d", code)
	}
	assertNoOldAccess := func() {
		t.Helper()
		var visible api.List[api.Space]
		if code := o.call("GET", "/api/periods/"+o.period.ID.String()+"/spaces", kadiv, nil, &visible); code != http.StatusOK || len(visible.Items) != 0 {
			t.Fatalf("discovery granted old spaces: code=%d spaces=%+v", code, visible.Items)
		}
		if code, _ := o.errCode("GET", o.scope(o.period.ID, "mediatek")+"/members", kadiv, nil); code != http.StatusForbidden {
			t.Fatalf("discovery opened old directory = %d", code)
		}
		if code, _ := o.errCode("GET", "/api/reminders/"+private.ID.String(), kadiv, nil); code != http.StatusNotFound {
			t.Fatalf("discovery opened private reminder = %d", code)
		}
	}
	assertNoOldAccess()
	var targets api.List[api.ArchiveTarget]
	if code := o.call("GET", currentScope+"/archives/targets", kadiv, nil, &targets); code != http.StatusOK {
		t.Fatalf("discover without old membership = %d", code)
	}
	allowed := map[id.ID]api.Space{}
	for _, space := range oldSpaces.Items {
		if slices.ContainsFunc(space.Divisions, func(d api.Division) bool { return d.ID == o.div["mediatek"].ID }) {
			allowed[space.ID] = space
		}
	}
	if len(allowed) != 2 || len(targets.Items) != len(allowed) {
		t.Fatalf("selector = %+v, expected Mediatek and Medhum only", targets.Items)
	}
	seen := map[id.ID]bool{}
	for _, target := range targets.Items {
		space, ok := allowed[target.SpaceID]
		if !ok || seen[target.SpaceID] || target.PeriodID != o.period.ID || target.PeriodLabel != o.period.Label || target.SpaceName != space.Name || target.SpaceKind != space.Kind {
			t.Fatalf("unrelated, open-period, duplicate or invalid target: %+v", target)
		}
		seen[target.SpaceID] = true
	}
	// Keep this endpoint's public shape deliberately narrower than api.Space.
	var raw struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if code := o.call("GET", currentScope+"/archives/targets", kadiv, nil, &raw); code != http.StatusOK {
		t.Fatalf("raw selector = %d", code)
	}
	for _, target := range raw.Items {
		if len(target) != 5 {
			t.Fatalf("selector exposed more than grant-selection metadata: %v", target)
		}
		for _, field := range []string{"spaceId", "periodId", "periodLabel", "spaceName", "spaceKind"} {
			if _, ok := target[field]; !ok {
				t.Fatalf("selector field missing: %s", field)
			}
		}
	}
	assertNoOldAccess()
	for sid := range allowed {
		for _, suffix := range []string{"", "/tasks"} {
			if code, _ := o.errCode("GET", "/api/spaces/"+sid.String()+suffix, kadiv, nil); code != http.StatusForbidden {
				t.Fatalf("selector opened old space%s = %d", suffix, code)
			}
		}
	}
	var grants api.List[api.ArchiveGrant]
	if code := o.call("GET", currentScope+"/archives", kadiv, nil, &grants); code != http.StatusOK || len(grants.Items) != 0 {
		t.Fatalf("discovery created a grant: code=%d grants=%+v", code, grants.Items)
	}
	// The metadata is usable by the existing grant endpoint, which remains the
	// separate explicit action that opens just the selected Space.
	memberToken, member := o.join(kadiv, current.ID, "mediatek", "archive-reader@himasif.test", "Reader")
	target := targets.Items[0]
	if code := o.call("POST", currentScope+"/archives", kadiv, api.ArchiveRequest{UserID: member.User.ID, SpaceID: target.SpaceID}, nil); code != http.StatusCreated {
		t.Fatalf("grant discovered target = %d", code)
	}
	var opened api.Space
	if code := o.call("GET", "/api/spaces/"+target.SpaceID.String(), memberToken, nil, &opened); code != http.StatusOK || !opened.ReadOnly || opened.Access != api.AccessArchive {
		t.Fatalf("read explicitly granted target = %d %+v", code, opened)
	}
	if code, _ := o.errCode("GET", "/api/reminders/"+private.ID.String(), memberToken, nil); code != http.StatusNotFound {
		t.Fatalf("explicit archive grant opened private reminder = %d", code)
	}
}

func TestArchiveTargetsRequireGrantManagementInTheExactOpenScope(t *testing.T) {
	o := newOrg(t)
	kadiv, _ := o.kadiv("mediatek", "archive-manager@himasif.test")
	memberToken, member := o.join(kadiv, o.period.ID, "mediatek", "archive-member@himasif.test", "Member")
	scope := o.scope(o.period.ID, "mediatek")
	path := scope + "/archives/targets"
	for _, token := range []string{kadiv, o.admin} {
		var targets api.List[api.ArchiveTarget]
		if code := o.call("GET", path, token, nil, &targets); code != http.StatusOK || targets.Items == nil || len(targets.Items) != 0 {
			t.Fatalf("manager's empty selector = %d %+v", code, targets.Items)
		}
	}
	assertDenied := func(token string) {
		t.Helper()
		if code, reason := o.errCode("GET", path, token, nil); code != http.StatusForbidden || reason != api.CodeForbidden {
			t.Fatalf("non-manager selector = %d %s", code, reason)
		}
	}
	assertDenied(memberToken)
	role := api.RoleCompanion
	if code := o.call("PATCH", scope+"/members/"+member.User.ID.String(), kadiv, api.UpdateMemberRequest{Role: &role, Groups: []string{}, CustomRoleIDs: []id.ID{}}, nil); code != http.StatusOK {
		t.Fatalf("make companion = %d", code)
	}
	assertDenied(memberToken)
	if code := o.call("PUT", scope+"/45", kadiv, api.Appoint45Request{UserID: member.User.ID}, nil); code != http.StatusCreated {
		t.Fatalf("make 45 = %d", code)
	}
	assertDenied(memberToken)
	if code, _ := o.errCode("GET", o.scope(o.period.ID, "humas")+"/archives/targets", kadiv, nil); code != http.StatusForbidden {
		t.Fatalf("cross-division selector = %d", code)
	}
	if code, _ := o.errCode("GET", path, "", nil); code != http.StatusUnauthorized {
		t.Fatalf("anonymous selector = %d", code)
	}
	if code := o.call("POST", "/api/periods/"+o.period.ID.String()+"/close", o.admin, nil, nil); code != http.StatusOK {
		t.Fatalf("close manager scope = %d", code)
	}
	for _, token := range []string{kadiv, o.admin} {
		if code, reason := o.errCode("GET", path, token, nil); code != http.StatusConflict || reason != api.CodePeriodClosed {
			t.Fatalf("closed manager scope = %d %s", code, reason)
		}
	}
}
