package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// Compare literal wire contracts rather than marshaling the request DTO into
// the expectation: extra fields and omitted clearing values must be detected.
func TestOrganizationClientWritesKeepScopeAndStrictBodies(t *testing.T) {
	p, d, u, successor, space, grant := id.New(), id.New(), id.New(), id.New(), id.New(), id.New()
	ctx := context.Background()
	at := time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC)
	start := timeutil.Date{Year: 2027, Month: 1, Day: 1}
	end := timeutil.Date{Year: 2027, Month: 12, Day: 31}
	period := api.Period{ID: p, Label: "2027/2028", StartsOn: start, EndsOn: end, TimeZone: "Asia/Jakarta", Status: api.PeriodPreparing}
	active, closed := period, period
	active.Status, closed.Status = api.PeriodActive, api.PeriodClosed
	member := api.Member{UserID: u, PeriodID: p, DivisionID: d, Role: api.RoleKadiv}
	handover := api.HandoverRequest{SuccessorPeriodID: &successor, IncomingLeadership: "Kadiv baru", AdminCustodian: "Pengurus baru", BotManagement: "Nomor divisi tetap", ArchivePlan: "Baca sesuai izin", RemainingWork: "Selesaikan laporan"}
	handoverResult := api.Handover{PeriodID: p, SuccessorPeriodID: &successor, IncomingLeadership: handover.IncomingLeadership, AdminCustodian: handover.AdminCustodian, BotManagement: handover.BotManagement, ArchivePlan: handover.ArchivePlan, RemainingWork: handover.RemainingWork, UpdatedAt: &at}
	cleared := api.Handover{PeriodID: p, RemainingWork: "Tidak ada pekerjaan tersisa.", UpdatedAt: &at}
	archive := api.ArchiveGrant{ID: grant, PeriodID: p, DivisionID: d, UserID: u, SpaceID: space, CreatedAt: at}
	periodURL := "/api/periods/" + p.String()
	scopeURL := periodURL + "/divisions/" + d.String()
	tests := []struct {
		name, method, path, body string
		status                   int
		response                 any
		call                     func(*Client) (any, error)
	}{
		{"create period", "POST", "/api/periods", fmt.Sprintf(`{"label":"2027/2028","startsOn":"2027-01-01","endsOn":"2027-12-31","timeZone":"Asia/Jakarta","divisionIds":["%s"]}`, d), 201, period,
			func(c *Client) (any, error) {
				return c.CreatePeriod(ctx, api.CreatePeriodRequest{Label: period.Label, StartsOn: start, EndsOn: end, TimeZone: period.TimeZone, DivisionIDs: []id.ID{d}})
			}},
		{"activate period", "POST", periodURL + "/activate", "", 200, active,
			func(c *Client) (any, error) { return c.ActivatePeriod(ctx, p) }},
		{"close period", "POST", periodURL + "/close", "", 200, closed,
			func(c *Client) (any, error) { return c.ClosePeriod(ctx, p) }},
		{"assign kadiv", "PUT", scopeURL + "/kadiv", fmt.Sprintf(`{"userId":"%s"}`, u), 200, member,
			func(c *Client) (any, error) { return c.SetKadiv(ctx, p, d, api.SetKadivRequest{UserID: u}) }},
		{"save handover", "PUT", periodURL + "/handover", fmt.Sprintf(`{"successorPeriodId":"%s","incomingLeadership":"Kadiv baru","adminCustodian":"Pengurus baru","botManagement":"Nomor divisi tetap","archivePlan":"Baca sesuai izin","remainingWork":"Selesaikan laporan"}`, successor), 200, handoverResult,
			func(c *Client) (any, error) { return c.SaveHandover(ctx, p, handover) }},
		{"clear successor and checklist fields", "PUT", periodURL + "/handover", `{"successorPeriodId":null,"incomingLeadership":"","adminCustodian":"","botManagement":"","archivePlan":"","remainingWork":"Tidak ada pekerjaan tersisa."}`, 200, cleared,
			func(c *Client) (any, error) {
				return c.SaveHandover(ctx, p, api.HandoverRequest{RemainingWork: cleared.RemainingWork})
			}},
		{"grant one archive space", "POST", scopeURL + "/archives", fmt.Sprintf(`{"userId":"%s","spaceId":"%s"}`, u, space), 201, archive,
			func(c *Client) (any, error) {
				return c.GrantArchive(ctx, p, d, api.ArchiveRequest{UserID: u, SpaceID: space})
			}},
		{"revoke one archive grant", "DELETE", scopeURL + "/archives/" + grant.String(), "", 204, nil,
			func(c *Client) (any, error) { return nil, c.RevokeArchive(ctx, p, d, grant) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.method || r.URL.RequestURI() != tt.path || r.Header.Get("Authorization") != "Bearer admin-session" {
					t.Errorf("request = %s %s; authorization present=%v", r.Method, r.URL.RequestURI(), r.Header.Get("Authorization") != "")
				}
				raw, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if tt.body == "" {
					if len(raw) != 0 {
						t.Errorf("unexpected body: %s", raw)
					}
				} else {
					var actual, expected any
					if err := json.Unmarshal(raw, &actual); err != nil {
						t.Error(err)
					}
					if err := json.Unmarshal([]byte(tt.body), &expected); err != nil {
						t.Error(err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					if !reflect.DeepEqual(actual, expected) {
						t.Errorf("body = %s; want %s", raw, tt.body)
					}
				}
				if tt.status == http.StatusNoContent {
					w.WriteHeader(tt.status)
					return
				}
				writeJSON(w, tt.status, tt.response)
			}))
			defer srv.Close()
			c := New(srv.URL)
			c.SetToken("admin-session")
			got, err := tt.call(c)
			if err != nil || !reflect.DeepEqual(got, tt.response) {
				t.Fatalf("response = %+v, error = %v; want %+v", got, err, tt.response)
			}
		})
	}
}

func TestOrganizationClientReadsKeepAccountStateAndArchiveMetadata(t *testing.T) {
	p, d, u, space, grant, oldPeriod := id.New(), id.New(), id.New(), id.New(), id.New(), id.New()
	at := time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC)
	users := []api.User{{ID: u, Kind: api.AccountPerson, Email: "ayu@example.test", DisplayName: "Ayu", EmailVerified: true, DisabledAt: &at}}
	handover := api.Handover{PeriodID: p}
	grants := []api.ArchiveGrant{{ID: grant, PeriodID: p, DivisionID: d, UserID: u, SpaceID: space, CreatedAt: at}}
	targets := []api.ArchiveTarget{{SpaceID: space, PeriodID: oldPeriod, PeriodLabel: "2025/2026", SpaceName: "Medhum", SpaceKind: api.SpaceShared}}
	periodURL := "/api/periods/" + p.String()
	scopeURL := periodURL + "/divisions/" + d.String()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer granting-account" {
			t.Errorf("unexpected request method or bearer: %s", r.Method)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || len(body) != 0 {
			t.Errorf("unexpected read body: %s, %v", body, err)
		}
		switch r.URL.RequestURI() {
		case "/api/users":
			writeJSON(w, 200, api.List[api.User]{Items: users})
		case periodURL + "/handover":
			writeJSON(w, 200, handover)
		case scopeURL + "/archives":
			writeJSON(w, 200, api.List[api.ArchiveGrant]{Items: grants})
		case scopeURL + "/archives/targets":
			writeJSON(w, 200, api.List[api.ArchiveTarget]{Items: targets})
		default:
			t.Errorf("unexpected route: %s", r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c, ctx := New(srv.URL), context.Background()
	c.SetToken("granting-account")
	if got, err := c.Users(ctx); err != nil || !reflect.DeepEqual(got, users) {
		t.Fatalf("users lost account verification/disabled state: %+v, %v", got, err)
	}
	if got, err := c.GetHandover(ctx, p); err != nil || !reflect.DeepEqual(got, handover) || got.UpdatedAt != nil || got.SuccessorPeriodID != nil {
		t.Fatalf("unwritten handover became filled: %+v, %v", got, err)
	}
	if got, err := c.ArchiveGrants(ctx, p, d); err != nil || !reflect.DeepEqual(got, grants) {
		t.Fatalf("archive grants lost granting/target scope: %+v, %v", got, err)
	}
	if got, err := c.ArchiveTargets(ctx, p, d); err != nil || !reflect.DeepEqual(got, targets) {
		t.Fatalf("archive target metadata lost identity: %+v, %v", got, err)
	}
}

func TestOrganizationClientPreservesScopedRejections(t *testing.T) {
	p, d, u, grant := id.New(), id.New(), id.New(), id.New()
	ctx := context.Background()
	tests := []struct {
		name, code, message string
		status              int
		call                func(*Client) error
	}{
		{"closed period", api.CodePeriodClosed, "Periode sudah ditutup.", 409,
			func(c *Client) error { _, err := c.ClosePeriod(ctx, p); return err }},
		{"active 45 blocks reassignment", api.CodeConflict, "Kadiv asli harus mencopot 45 terlebih dahulu.", 409,
			func(c *Client) error { _, err := c.SetKadiv(ctx, p, d, api.SetKadivRequest{UserID: u}); return err }},
		{"handover forbidden", api.CodeForbidden, "Hanya admin organisasi.", 403,
			func(c *Client) error { _, err := c.SaveHandover(ctx, p, api.HandoverRequest{}); return err }},
		{"archive grant forbidden", api.CodeForbidden, "45 tidak boleh mengatur akses.", 403,
			func(c *Client) error { _, err := c.GrantArchive(ctx, p, d, api.ArchiveRequest{UserID: u}); return err }},
		{"archive revoke forbidden", api.CodeForbidden, "Scope pemberi berbeda.", 403,
			func(c *Client) error { return c.RevokeArchive(ctx, p, d, grant) }},
		{"targets forbidden", api.CodeForbidden, "Direktori arsip tidak tersedia.", 403,
			func(c *Client) error { _, err := c.ArchiveTargets(ctx, p, d); return err }},
		{"users session expired", api.CodeUnauthenticated, "Sesi berakhir.", 401,
			func(c *Client) error { _, err := c.Users(ctx); return err }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				apiError(w, tt.status, tt.code, tt.message)
			}))
			defer srv.Close()
			err := tt.call(New(srv.URL))
			var apiErr *Error
			if !errors.As(err, &apiErr) || apiErr.Status != tt.status || apiErr.Code != tt.code || apiErr.Message != tt.message {
				t.Fatalf("rejection changed or appeared successful: %+v", err)
			}
		})
	}
}
