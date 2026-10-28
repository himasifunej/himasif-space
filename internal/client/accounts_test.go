package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
)

// Contract tests cover the distinction between new-account proof acceptance,
// existing-account bearer acceptance, and the empty 202/204 responses.
func TestAccountEndpointContracts(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name, path, token string
		body              any
		status            int
		response          any
		call              func(*Client) error
	}{
		{"inspect", "/api/auth/invitations/inspect", "", api.InvitationTokenRequest{Token: "invitation"}, 200, api.InvitationPreview{Email: "person@example.test", AccountExists: true}, func(c *Client) error {
			out, err := c.InspectInvitation(ctx, api.InvitationTokenRequest{Token: "invitation"})
			if err == nil && (!out.AccountExists || out.Email != "person@example.test") {
				t.Error("preview not decoded")
			}
			return err
		}},
		{"email-proof", "/api/auth/invitations/verify", "", api.InvitationTokenRequest{Token: "invitation"}, 202, nil, func(c *Client) error {
			return c.VerifyInvitationEmail(ctx, api.InvitationTokenRequest{Token: "invitation"})
		}},
		{"accept-new", "/api/auth/invitations/accept", "", api.InvitationAcceptRequest{Token: "invitation", VerificationToken: "proof", DisplayName: "Person", Password: "new-password"}, 200, api.InvitationAcceptResponse{User: api.User{Email: "person@example.test"}}, func(c *Client) error {
			out, err := c.AcceptInvitation(ctx, api.InvitationAcceptRequest{Token: "invitation", VerificationToken: "proof", DisplayName: "Person", Password: "new-password"})
			if err == nil && out.User.Email != "person@example.test" {
				t.Error("accept response not decoded")
			}
			return err
		}},
		{"accept-existing", "/api/auth/invitations/accept", "session", api.InvitationAcceptRequest{Token: "invitation"}, 200, api.InvitationAcceptResponse{}, func(c *Client) error {
			_, err := c.AcceptInvitation(ctx, api.InvitationAcceptRequest{Token: "invitation"})
			return err
		}},
		{"recovery", "/api/auth/recovery", "", api.RecoveryRequest{Email: "unknown@example.test"}, 202, nil, func(c *Client) error {
			return c.RequestRecovery(ctx, api.RecoveryRequest{Email: "unknown@example.test"})
		}},
		{"reset", "/api/auth/recovery/reset", "", api.RecoveryResetRequest{Token: "recovery", Password: "new-password"}, 204, nil, func(c *Client) error {
			return c.ResetPassword(ctx, api.RecoveryResetRequest{Token: "recovery", Password: "new-password"})
		}},
		{"password", "/api/auth/password", "session", api.PasswordChangeRequest{CurrentPassword: "old-password", Password: "new-password"}, 204, nil, func(c *Client) error {
			return c.ChangePassword(ctx, api.PasswordChangeRequest{CurrentPassword: "old-password", Password: "new-password"})
		}},
		{"admin-proof", "/api/auth/admin-email", "session", api.AdminEmailRequest{CurrentPassword: "old-password", Email: "new@example.test"}, 202, nil, func(c *Client) error {
			return c.RequestAdminEmail(ctx, api.AdminEmailRequest{CurrentPassword: "old-password", Email: "new@example.test"})
		}},
		{"admin-handover", "/api/auth/admin-handover", "session", api.AdminHandoverRequest{CurrentPassword: "old-password", Password: "new-password", Email: "new@example.test", VerificationToken: "proof", Custodian: "Successor"}, 204, nil, func(c *Client) error {
			return c.AdminHandover(ctx, api.AdminHandoverRequest{CurrentPassword: "old-password", Password: "new-password", Email: "new@example.test", VerificationToken: "proof", Custodian: "Successor"})
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != tc.path {
					t.Errorf("unexpected method/path: %s %s", r.Method, r.URL.Path)
				}
				wantAuth := ""
				if tc.token != "" {
					wantAuth = "Bearer " + tc.token
				}
				if r.Header.Get("Authorization") != wantAuth {
					t.Error("wrong session header")
				}
				if r.Header.Get("Content-Type") != "application/json" {
					t.Error("missing JSON content type")
				}
				var got any
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Error(err)
				}
				body, _ := json.Marshal(tc.body)
				var want any
				_ = json.Unmarshal(body, &want)
				if !reflect.DeepEqual(got, want) {
					t.Error("request body differs from account contract")
				}
				if tc.response != nil {
					writeJSON(w, tc.status, tc.response)
				} else {
					w.WriteHeader(tc.status)
				}
			}))
			defer s.Close()
			c := New(s.URL)
			c.SetToken(tc.token)
			if err := tc.call(c); err != nil {
				t.Fatal(err)
			}
			if c.token() != tc.token {
				t.Error("account endpoint mutated the session locally")
			}
		})
	}
}

func TestAccountInvalidProofRemainsServerError(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiError(w, 410, api.CodeInvalidToken, "Tautan tidak valid atau kedaluwarsa.")
	}))
	defer s.Close()
	c := New(s.URL)
	if _, err := c.InspectInvitation(context.Background(), api.InvitationTokenRequest{Token: "expired"}); err == nil || IsUnauthenticated(err) || IsUnreachable(err) {
		t.Fatalf("invalid-proof error classification: %v", err)
	}
}

func TestAccountSessionPinsBearerBeforeAccountSwitch(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer first-account" {
			t.Error("queued operation used the next account's bearer")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer s.Close()
	c := New(s.URL)
	c.SetToken("first-account")
	queued := c.AccountSession()
	c.SetToken("second-account")
	if err := queued.ChangePassword(context.Background(), api.PasswordChangeRequest{CurrentPassword: "current-password", Password: "new-password"}); err != nil {
		t.Fatal(err)
	}
	if c.token() != "second-account" {
		t.Error("snapshot mutated the live session")
	}
}
