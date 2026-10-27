package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"net/http"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/authz"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
)

// Navigation must never mutate the process-wide capability policies. No
// database is needed on the admin branch of spaceViews.
func TestAdminSpaceViewsPreserveCapabilities(t *testing.T) {
	original := slices.Clone(authz.AdminCaps)
	t.Cleanup(func() { authz.AdminCaps = original })
	s := &Server{}
	p := &principal{User: store.User{Kind: api.AccountOrganizationAdmin}}
	spaces := []store.Space{
		{ID: id.New(), Kind: api.SpaceShared, Code: "medhum", Divisions: []api.Division{{ID: id.New(), Slug: "humas"}, {ID: id.New(), Slug: "mediatek"}}},
		{ID: id.New(), Kind: api.SpaceDivision, Code: "division-mediatek", Divisions: []api.Division{{ID: id.New(), Slug: "mediatek"}}},
	}
	for _, status := range []string{api.PeriodActive, api.PeriodClosed, api.PeriodActive} {
		got, err := s.spaceViews(context.Background(), nil, p, api.Period{Status: status}, spaces)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(authz.AdminCaps, original) {
			t.Fatalf("%s space lookup changed admin policy: got %v, want %v", status, authz.AdminCaps, original)
		}
		want := authz.Normalize(original)
		if status == api.PeriodClosed {
			want = authz.ReadOnly(want)
		}
		if len(got) != 2 || !slices.Equal(got[1].Capabilities, want) {
			t.Fatalf("admin division access changed after a shared-space lookup: %+v, want capabilities %v", got, want)
		}
		for _, c := range got[0].Capabilities {
			if !slices.Contains(authz.Operational, c) {
				t.Fatalf("shared space exposes administrative capability %q", c)
			}
		}
	}
}

type accessHTTPResult struct {
	status int
	err    error
}

// accessCallAsync avoids testing.Fatal inside the request goroutine and bounds
// every request even if the lock orchestration fails.
func accessCallAsync(ctx context.Context, e *env, method, path, token string, body any) <-chan accessHTTPResult {
	done := make(chan accessHTTPResult, 1)
	go func() {
		data, err := json.Marshal(body)
		if err != nil {
			done <- accessHTTPResult{err: err}
			return
		}
		req, err := http.NewRequestWithContext(ctx, method, e.url+path, bytes.NewReader(data))
		if err != nil {
			done <- accessHTTPResult{err: err}
			return
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			done <- accessHTTPResult{err: err}
			return
		}
		response.Body.Close()
		done <- accessHTTPResult{status: response.StatusCode}
	}()
	return done
}

// waitAccessQueryBlocked waits for a real PostgreSQL lock wait, rather than
// guessing whether a goroutine reached a statement after a fixed sleep.
func waitAccessQueryBlocked(ctx context.Context, t *testing.T, st *store.Store, queryFragment, event string, done <-chan accessHTTPResult) {
	t.Helper()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case result := <-done:
			t.Fatalf("request completed before waiting on %q: status=%d err=%v", queryFragment, result.status, result.err)
		default:
		}
		var blocked bool
		err := st.DB().QueryRow(ctx, `select exists(select 1 from pg_stat_activity where datname=current_database()
   and pid<>pg_backend_pid() and wait_event_type='Lock' and query like $1 and ($2='' or wait_event=$2))`, "%"+queryFragment+"%", event).Scan(&blocked)
		if err != nil {
			t.Fatal(err)
		}
		if blocked {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for PostgreSQL lock on %q: %v", queryFragment, ctx.Err())
		case <-ticker.C:
		}
	}
}

// A removal paused after its checks must serialize with appointment. Without
// an exclusive period lock both requests can succeed and leave an active 45
// appointment whose membership has already been removed.
func TestMemberRemovalSerializes45Appointment(t *testing.T) {
	o := newOrg(t)
	kadiv, _ := o.kadiv("mediatek", "kadiv-race@himasif.test")
	_, member := o.join(kadiv, o.period.ID, "mediatek", "anggota-race@himasif.test", "Anggota")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	gate, err := regressionConnection(ctx, o.st)
	if err != nil {
		t.Fatal(err)
	}
	const gateID int64 = 0x53414343455353
	if _, err := gate.Exec(ctx, `select pg_advisory_lock($1)`, gateID); err != nil {
		gate.Close(context.Background())
		t.Fatal(err)
	}
	held := true
	unlock := func() {
		if held {
			// Cleanup must also work after the request/test deadline.
			releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer releaseCancel()
			_, err := gate.Exec(releaseCtx, `select pg_advisory_unlock($1)`, gateID)
			if err != nil {
				t.Errorf("release membership gate: %v", err)
			}
			held = false
		}
	}
	t.Cleanup(func() { unlock(); gate.Close(context.Background()) })
	// The fixture has its own temporary database. This trigger is an external
	// synchronization gate; no production hooks or artificial timing are used.
	statement := fmt.Sprintf(`create function gate_member_removal() returns trigger language plpgsql as $$
 begin
  if old.id='%s'::uuid and new.removed_at is not null then
   perform pg_advisory_xact_lock(%d);
  end if;
  return new;
 end $$;
 create trigger gate_member_removal before update on memberships for each row execute function gate_member_removal()`, member.Membership.ID, gateID)
	if _, err := o.st.DB().Exec(ctx, statement); err != nil {
		t.Fatal(err)
	}
	path := o.scope(o.period.ID, "mediatek")
	removal := accessCallAsync(ctx, o.env, "DELETE", path+"/members/"+member.User.ID.String(), o.admin, nil)
	waitAccessQueryBlocked(ctx, t, o.st, "update memberships set removed_at", "advisory", removal)
	appointment := accessCallAsync(ctx, o.env, "PUT", path+"/45", kadiv, api.Appoint45Request{UserID: member.User.ID})
	waitAccessQueryBlocked(ctx, t, o.st, "from periods where id = $1 for update", "", appointment)
	unlock()
	for _, request := range []struct {
		name string
		done <-chan accessHTTPResult
		want int
	}{
		{"membership removal", removal, http.StatusNoContent},
		{"45 appointment", appointment, http.StatusBadRequest},
	} {
		select {
		case result := <-request.done:
			if result.err != nil || result.status != request.want {
				t.Fatalf("%s: status=%d err=%v; want %d", request.name, result.status, result.err, request.want)
			}
		case <-ctx.Done():
			t.Fatalf("%s: %v", request.name, ctx.Err())
		}
	}
	var active45 int
	if err := o.st.DB().QueryRow(ctx, `select count(*) from acting45 where period_id=$1 and division_id=$2 and removed_at is null`, o.period.ID, o.div["mediatek"].ID).Scan(&active45); err != nil {
		t.Fatal(err)
	}
	if active45 != 0 {
		t.Fatalf("removed member retained %d active 45 appointment(s)", active45)
	}
}

// regressionConnection opens an independent connection to the fixture's
// temporary database without exposing the store's connection pool.
func regressionConnection(ctx context.Context, st *store.Store) (*pgx.Conn, error) {
	var name string
	if err := st.DB().QueryRow(ctx, "select current_database()").Scan(&name); err != nil {
		return nil, err
	}
	cfg, err := pgx.ParseConfig(os.Getenv("SPACE_TEST_DATABASE_URL"))
	if err != nil {
		return nil, err
	}
	cfg.Database = name
	return pgx.ConnectConfig(ctx, cfg)
}
