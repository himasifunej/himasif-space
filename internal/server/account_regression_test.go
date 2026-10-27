package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/auth"
	"github.com/himasifunej/himasif-space/internal/store"
)

// Hold the user row while login reads and verifies the old credential, then
// atomically rotate credentials/revoke sessions before allowing it to proceed.
// This is the same serialization boundary used by recovery and handover.
func TestLoginRechecksAccountAtSessionCreation(t *testing.T) {
	for _, change := range []string{"password", "email", "disabled"} {
		t.Run(change, func(t *testing.T) {
			o := newOrg(t)
			_, acc := o.join(o.admin, o.period.ID, "humas", "race-login@himasif.test", "Pemilik akun")
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			conn, err := regressionConnection(ctx, o.st)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close(context.Background())
			tx, err := conn.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			if _, err := o.st.UserForUpdate(ctx, tx, acc.User.ID); err != nil {
				t.Fatal(err)
			}
			request := api.LoginRequest{Email: acc.User.Email, Password: "kata-sandi-anggota", Client: "web"}
			done := accessCallAsync(ctx, o.env, "POST", "/api/auth/login", "", request)
			// Original code waits in the INSERT's FK check. Correct code waits for
			// the account lock before checking the credential for session creation.
			waitAccessQueryBlocked(ctx, t, o.st, "", "", done)
			now := time.Now()
			wantStatus := http.StatusUnauthorized
			switch change {
			case "password":
				hash, err := auth.HashPassword("kata-sandi-sesudah-pemulihan")
				if err != nil {
					t.Fatal(err)
				}
				if err := o.st.SetPassword(ctx, tx, acc.User.ID, hash, now); err != nil {
					t.Fatal(err)
				}
			case "email":
				if err := o.st.SetVerifiedEmail(ctx, tx, acc.User.ID, "email-baru@himasif.test", now); err != nil {
					t.Fatal(err)
				}
			case "disabled":
				if _, err := o.st.SetDisabled(ctx, tx, acc.User.ID, true, now); err != nil {
					t.Fatal(err)
				}
				wantStatus = http.StatusForbidden
			}
			if _, err := o.st.RevokeUserSessions(ctx, tx, acc.User.ID, nil, now); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			select {
			case result := <-done:
				if result.err != nil || result.status != wantStatus {
					t.Fatalf("login after %s rotation: status=%d err=%v; want %d", change, result.status, result.err, wantStatus)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			var active int
			if err := o.st.DB().QueryRow(ctx, "select count(*) from sessions where user_id=$1 and revoked_at is null", acc.User.ID).Scan(&active); err != nil {
				t.Fatal(err)
			}
			if active != 0 {
				t.Fatalf("old credential created %d session(s) after account rotation", active)
			}
		})
	}
}

// Model a request authenticated just before another request completes admin
// handover. The handler must not trust that stale middleware principal to
// overwrite the new custodian's password or issue new email proofs.
func TestSensitiveAccountActionsRejectStalePrincipal(t *testing.T) {
	o := newOrg(t)
	capture := httptest.NewRequest("GET", "/api/me", nil)
	capture.Header.Set("Authorization", "Bearer "+o.admin)
	old, err := o.srv.principal(capture)
	if err != nil {
		t.Fatal(err)
	}
	if status := o.call("POST", "/api/auth/admin-handover", o.admin, api.AdminHandoverRequest{CurrentPassword: adminPassword, Password: "kata-sandi-pengelola-baru", Custodian: "Pengelola baru"}, nil); status != 204 {
		t.Fatalf("handover=%d", status)
	}
	cases := []struct {
		name   string
		body   any
		action func(http.ResponseWriter, *http.Request, *principal) error
	}{
		{"password", api.PasswordChangeRequest{CurrentPassword: adminPassword, Password: "kata-sandi-dari-permintaan-lama"}, o.srv.changePassword},
		{"admin-handover", api.AdminHandoverRequest{CurrentPassword: adminPassword, Password: "kata-sandi-diambil-lagi", Custodian: "Pengelola lama"}, o.srv.adminHandover},
		{"admin-email", api.AdminEmailRequest{CurrentPassword: adminPassword, Email: "bukti-lama@himasif.test"}, o.srv.requestAdminEmail},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.body)
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest("POST", "/api/auth/"+tc.name, bytes.NewReader(data))
			rec := httptest.NewRecorder()
			err = tc.action(rec, req, old)
			if err == nil {
				t.Fatal("stale authenticated request changed the account after handover")
			}
			o.srv.answerError(rec, req, old, err)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("stale request=%d %s, want401", rec.Code, rec.Body.String())
			}
		})
	}
	if o.mail.count("bukti-lama@himasif.test") != 0 {
		t.Fatal("revoked admin session sent an email proof")
	}
	if status, _, _ := o.login(adminEmail, "kata-sandi-pengelola-baru"); status != http.StatusOK {
		t.Fatalf("new custodian's password no longer works: %d", status)
	}
}

func TestJSONRejectsTrailingClosingDelimiter(t *testing.T) {
	for _, body := range []string{`{"email":"someone@himasif.test"}}`, `{"email":"someone@himasif.test"}]`} {
		var req api.RecoveryRequest
		if err := readJSON(httptest.NewRequest("POST", "/", strings.NewReader(body)), &req); err == nil {
			t.Errorf("accepted malformed body %q", body)
		}
		rec := httptest.NewRecorder()
		if decode(rec, httptest.NewRequest("POST", "/", strings.NewReader(body)), &req) {
			t.Errorf("login decoder accepted malformed body %q", body)
		}
	}
}

// Two previously issued proofs can exist, for example from concurrent requests
// before an upgrade. Reset must serialize on the account before locking either
// proof; otherwise each reset can hold the token needed by the other.
func TestRecoveryResetsSerializeBeforeTokenLocks(t *testing.T) {
	o := newOrg(t)
	_, acc := o.join(o.admin, o.period.ID, "humas", "double-reset@himasif.test", "Pemilik akun")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var tokens []string
	for range 2 {
		raw, hash, err := auth.NewToken()
		if err != nil {
			t.Fatal(err)
		}
		if err := o.st.CreateAuthToken(ctx, o.st.DB(), store.AuthToken{Purpose: store.TokenRecovery, UserID: &acc.User.ID, Email: acc.User.Email, ExpiresAt: time.Now().Add(time.Hour)}, hash, time.Now()); err != nil {
			t.Fatal(err)
		}
		tokens = append(tokens, raw)
	}
	conn, err := regressionConnection(ctx, o.st)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err := o.st.UserForUpdate(ctx, tx, acc.User.ID); err != nil {
		t.Fatal(err)
	}
	first := accessCallAsync(ctx, o.env, "POST", "/api/auth/recovery/reset", "", api.RecoveryResetRequest{Token: tokens[0], Password: "kata-sandi-pemulihan-satu"})
	second := accessCallAsync(ctx, o.env, "POST", "/api/auth/recovery/reset", "", api.RecoveryResetRequest{Token: tokens[1], Password: "kata-sandi-pemulihan-dua"})
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		var count int
		if err := o.st.DB().QueryRow(ctx, `select count(*) from pg_stat_activity where datname=current_database() and wait_event_type='Lock' and query like '%from users where id = $1 for update%'`).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count == 2 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("both resets did not wait for the account lock: ", ctx.Err())
		case <-ticker.C:
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	counts := map[int]int{}
	for _, done := range []<-chan accessHTTPResult{first, second} {
		select {
		case result := <-done:
			if result.err != nil {
				t.Fatal(result.err)
			}
			counts[result.status]++
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	if counts[204] != 1 || counts[410] != 1 {
		t.Fatalf("concurrent reset responses=%v, want one204 and one410", counts)
	}
}
