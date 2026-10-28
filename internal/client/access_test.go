package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Scope stays in the URL and empty configuration arrays remain JSON arrays;
// neither a shared Space nor an invitation silently changes division identity.
func TestAccessClientScopeAndReplacement(t *testing.T) {
	p, d, u := id.New(), id.New(), id.New()
	sid := id.New()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer session" {
			apiError(w, 401, api.CodeUnauthenticated, "Sesi berakhir")
			return
		}
		switch r.Method + " " + r.URL.Path {
		case "GET /api/periods/" + p.String() + "/spaces":
			writeJSON(w, 200, api.List[api.Space]{Items: []api.Space{{ID: sid, PeriodID: p, Kind: api.SpaceShared, Divisions: []api.Division{{ID: d}}, ReadOnly: true}}})
		case "PATCH " + divisionPath(p, d) + "/members/" + u.String():
			var raw map[string]json.RawMessage
			if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
				t.Error(err)
			}
			if string(raw["groups"]) != "[]" || string(raw["customRoleIds"]) != "[]" || string(raw["role"]) != "\"companion\"" {
				t.Errorf("replacement lost required values: %s %s %s", raw["groups"], raw["customRoleIds"], raw["role"])
			}
			writeJSON(w, 200, api.Member{UserID: u, DivisionID: d, PeriodID: p, Role: api.RoleCompanion})
		default:
			t.Errorf("unexpected scope/method %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	c := New(srv.URL)
	c.SetToken("session")
	spaces, err := c.Spaces(context.Background(), p)
	if err != nil || len(spaces) != 1 || spaces[0].ID != sid || !spaces[0].ReadOnly {
		t.Fatalf("spaces=%+v err=%v", spaces, err)
	}
	role := api.RoleCompanion
	member, err := c.UpdateMember(context.Background(), p, d, u, api.UpdateMemberRequest{Role: &role, Groups: []string{}, CustomRoleIDs: []id.ID{}})
	if err != nil || member.DivisionID != d || member.Role != role {
		t.Fatalf("member=%+v err=%v", member, err)
	}
	c.SetToken("")
	_, err = c.Spaces(context.Background(), p)
	if !IsUnauthenticated(err) {
		t.Fatalf("lost unauthorized error: %v", err)
	}
}

func TestAccessWritesPreserveRejection(t *testing.T) {
	p, d := id.New(), id.New()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiError(w, 403, api.CodeForbidden, "45 tidak boleh mengatur akses.")
	}))
	defer srv.Close()
	c := New(srv.URL)
	if _, err := c.Appoint45(context.Background(), p, d, api.Appoint45Request{UserID: id.New()}); err == nil {
		t.Fatal("appointment appeared successful after rejection")
	}
	if err := c.RevokeInvitation(context.Background(), p, d, id.New()); err == nil {
		t.Fatal("revocation appeared successful after rejection")
	}
}
