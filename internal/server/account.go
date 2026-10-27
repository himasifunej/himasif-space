package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/auth"
	"github.com/himasifunej/himasif-space/internal/store"
)

// requestRecovery sends a recovery link to an account's email. It answers
// the same at once whether or not the email has an account; the lookup and
// the sending happen after the answer, so their time tells nothing either.
func (s *Server) requestRecovery(w http.ResponseWriter, r *http.Request) error {
	if err := s.allowPublic(r); err != nil {
		return err
	}
	var req api.RecoveryRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	email, err := validEmail(req.Email)
	if err != nil {
		return err
	}
	if err := s.allowMail(email); err != nil {
		return err
	}
	rid := requestID(r.Context())
	s.bg.Add(1)
	go func() {
		defer s.bg.Done()
		ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), requestIDKey, rid), time.Minute)
		defer cancel()
		if err := s.sendRecovery(ctx, r, email); err != nil {
			s.log.Error("recovery", "request_id", rid, "err", err)
		}
	}()
	w.WriteHeader(http.StatusAccepted)
	return nil
}

func (s *Server) sendRecovery(ctx context.Context, r *http.Request, email string) error {
	u, err := s.store.UserByEmail(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if u.DisabledAt != nil {
		return nil
	}
	r = r.WithContext(ctx)
	return s.store.Tx(ctx, func(q store.Querier) error {
		now := s.now()
		token, hash, err := auth.NewToken()
		if err != nil {
			return err
		}
		if err := s.store.UseUserTokens(ctx, q, u.ID, store.TokenRecovery, now); err != nil {
			return err
		}
		if err := s.store.CreateAuthToken(ctx, q, store.AuthToken{Purpose: store.TokenRecovery, Email: u.Email, UserID: &u.ID,
			ExpiresAt: now.Add(recoveryTTL)}, hash, now); err != nil {
			return err
		}
		if err := s.record(r, q, nil, store.AuditEvent{ActorKind: "system", Action: "auth.recovery.send", TargetType: "user", TargetID: &u.ID}); err != nil {
			return err
		}
		return s.send(ctx, recoveryMail(s, u.Email, token))
	})
}

// resetPassword sets a new password with a recovery token, ending every
// session and every other recovery token. It does not enable a disabled
// account.
func (s *Server) resetPassword(w http.ResponseWriter, r *http.Request) error {
	if err := s.allowPublic(r); err != nil {
		return err
	}
	var req api.RecoveryResetRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	th, err := tokenHash(req.Token)
	if err != nil {
		return err
	}
	if msg := auth.CheckPassword(req.Password); msg != "" {
		return badRequest(msg)
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return err
	}
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		ctx, now := r.Context(), s.now()
		t, err := s.store.UsableAuthToken(ctx, q, th, store.TokenRecovery, now)
		if err != nil {
			return notFound(err, errInvalidToken)
		}
		u, err := s.store.UserForUpdate(ctx, q, *t.UserID)
		if err != nil {
			return err
		}
		if u.Email != t.Email {
			return errInvalidToken // the account's email changed since
		}
		if err := s.store.SetPassword(ctx, q, u.ID, hash, now); err != nil {
			return err
		}
		if err := s.store.UseUserTokens(ctx, q, u.ID, store.TokenRecovery, now); err != nil {
			return err
		}
		n, err := s.store.RevokeUserSessions(ctx, q, u.ID, nil, now)
		if err != nil {
			return err
		}
		return s.record(r, q, nil, store.AuditEvent{ActorUserID: &u.ID, ActorKind: u.Kind, Action: "auth.recovery.reset",
			TargetType: "user", TargetID: &u.ID, After: map[string]any{"sessionsRevoked": n}})
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// checkCurrentPassword verifies the caller's password, with the limit of
// login attempts.
func (s *Server) checkCurrentPassword(r *http.Request, p *principal, pw string) error {
	if !s.loginLimit.allow("password:"+p.User.ID.String(), s.now()) {
		return errRateLimited
	}
	if pw == "" {
		return badRequest("Isi kata sandi saat ini.")
	}
	if err := auth.VerifyPassword(p.User.PasswordHash, pw); err != nil {
		if !errors.Is(err, auth.ErrMismatch) {
			return err
		}
		return fail(http.StatusForbidden, api.CodeForbidden, "Kata sandi saat ini salah.")
	}
	return nil
}

// changePassword changes the caller's password and ends their other
// sessions.
func (s *Server) changePassword(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.PasswordChangeRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	if msg := auth.CheckPassword(req.Password); msg != "" {
		return badRequest(msg)
	}
	if err := s.checkCurrentPassword(r, p, req.CurrentPassword); err != nil {
		return err
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return err
	}
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		now := s.now()
		if err := s.store.SetPassword(r.Context(), q, p.User.ID, hash, now); err != nil {
			return err
		}
		n, err := s.store.RevokeUserSessions(r.Context(), q, p.User.ID, &p.Session.ID, now)
		if err != nil {
			return err
		}
		return s.record(r, q, p, store.AuditEvent{Action: "auth.password.change", TargetType: "user", TargetID: &p.User.ID,
			After: map[string]any{"otherSessionsRevoked": n}})
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// requestAdminEmail sends a proof to the new recovery email of the
// organization admin, the first step of its handover.
func (s *Server) requestAdminEmail(w http.ResponseWriter, r *http.Request, p *principal) error {
	if !p.admin() {
		return errForbidden
	}
	var req api.AdminEmailRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	email, err := validEmail(req.Email)
	if err != nil {
		return err
	}
	if err := s.checkCurrentPassword(r, p, req.CurrentPassword); err != nil {
		return err
	}
	if email == p.User.Email {
		return badRequest("Email ini sudah menjadi email admin organisasi.")
	}
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		ctx, now := r.Context(), s.now()
		if _, err := s.store.UserByEmailQ(ctx, q, email); err == nil {
			return conflict("Email ini sudah dipakai akun lain.")
		} else if !errors.Is(err, store.ErrNotFound) {
			return err
		}
		if err := s.allowMail(email); err != nil {
			return err
		}
		proof, hash, err := auth.NewToken()
		if err != nil {
			return err
		}
		if err := s.store.UseUserTokens(ctx, q, p.User.ID, store.TokenAdminEmail, now); err != nil {
			return err
		}
		if err := s.store.CreateAuthToken(ctx, q, store.AuthToken{Purpose: store.TokenAdminEmail, Email: email, UserID: &p.User.ID,
			ExpiresAt: now.Add(proofTTL)}, hash, now); err != nil {
			return err
		}
		if err := s.record(r, q, p, store.AuditEvent{Action: "admin.email_proof.send", TargetType: "user", TargetID: &p.User.ID,
			After: map[string]any{"email": email}}); err != nil {
			return err
		}
		return s.send(ctx, adminEmailMail(s, email, proof))
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusAccepted)
	return nil
}

// adminHandover hands the organization admin account to its next
// custodian: a new password, optionally a new proved email, and the end of
// every session of the account, this one included (ACC-12, ACC-13).
func (s *Server) adminHandover(w http.ResponseWriter, r *http.Request, p *principal) error {
	if !p.admin() {
		return errForbidden
	}
	var req api.AdminHandoverRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	custodian, err := text(req.Custodian, "nama pengelola admin yang baru", 1, 200)
	if err != nil {
		return err
	}
	if msg := auth.CheckPassword(req.Password); msg != "" {
		return badRequest(msg)
	}
	if req.Password == req.CurrentPassword {
		return badRequest("Kata sandi baru harus berbeda dari kata sandi saat ini.")
	}
	email := ""
	if req.Email != "" {
		if email, err = validEmail(req.Email); err != nil {
			return err
		}
		if req.VerificationToken == "" {
			return badRequest("Verifikasi email baru terlebih dahulu.")
		}
	} else if req.VerificationToken != "" {
		return badRequest("Isi email yang diverifikasi.")
	}
	if err := s.checkCurrentPassword(r, p, req.CurrentPassword); err != nil {
		return err
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return err
	}
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		ctx, now := r.Context(), s.now()
		u, err := s.store.UserForUpdate(ctx, q, p.User.ID)
		if err != nil {
			return err
		}
		after := map[string]any{"email": u.Email, "custodian": custodian}
		if email != "" {
			th, err := tokenHash(req.VerificationToken)
			if err != nil {
				return err
			}
			t, err := s.store.UsableAuthToken(ctx, q, th, store.TokenAdminEmail, now)
			if err != nil {
				return notFound(err, errInvalidToken)
			}
			if t.UserID == nil || *t.UserID != u.ID || t.Email != email {
				return errInvalidToken
			}
			if err := s.store.SetVerifiedEmail(ctx, q, u.ID, email, now); errors.Is(err, store.ErrEmailTaken) {
				return conflict("Email ini sudah dipakai akun lain.")
			} else if err != nil {
				return err
			}
			after["email"] = email
		}
		if err := s.store.SetPassword(ctx, q, u.ID, hash, now); err != nil {
			return err
		}
		for _, purpose := range []string{store.TokenAdminEmail, store.TokenRecovery} {
			if err := s.store.UseUserTokens(ctx, q, u.ID, purpose, now); err != nil {
				return err
			}
		}
		n, err := s.store.RevokeUserSessions(ctx, q, u.ID, nil, now)
		if err != nil {
			return err
		}
		after["sessionsRevoked"] = n
		return s.record(r, q, p, store.AuditEvent{Action: "admin.handover", TargetType: "user", TargetID: &u.ID,
			Before: map[string]any{"email": u.Email}, After: after})
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// listPeople lists personal accounts for the admin, as to choose a kadiv.
func (s *Server) listPeople(w http.ResponseWriter, r *http.Request, p *principal) error {
	if !p.admin() {
		return errForbidden
	}
	us, err := s.store.People(r.Context())
	if err != nil {
		return err
	}
	out := []api.User{}
	for _, u := range us {
		out = append(out, u.API())
	}
	writeJSON(w, http.StatusOK, api.List[api.User]{Items: out})
	return nil
}

func (s *Server) disableUser(w http.ResponseWriter, r *http.Request, p *principal) error {
	return s.setDisabled(w, r, p, true)
}

func (s *Server) enableUser(w http.ResponseWriter, r *http.Request, p *principal) error {
	return s.setDisabled(w, r, p, false)
}

// setDisabled disables a personal account, ending its sessions, or enables
// it again. Memberships are kept; a disabled account just cannot sign in.
func (s *Server) setDisabled(w http.ResponseWriter, r *http.Request, p *principal, disabled bool) error {
	if !p.admin() {
		return errForbidden
	}
	uid, err := pathID(r, "userId")
	if err != nil {
		return err
	}
	var out store.User
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		ctx, now := r.Context(), s.now()
		u, err := s.store.UserForUpdate(ctx, q, uid)
		if err != nil {
			return notFound(err, errNotFound)
		}
		if u.Kind != api.AccountPerson {
			return badRequest("Hanya akun pribadi yang dapat dinonaktifkan.")
		}
		if (u.DisabledAt != nil) == disabled {
			out = u
			return nil
		}
		if out, err = s.store.SetDisabled(ctx, q, uid, disabled, now); err != nil {
			return err
		}
		action, after := "user.enable", map[string]any{"disabled": false}
		if disabled {
			n, err := s.store.RevokeUserSessions(ctx, q, uid, nil, now)
			if err != nil {
				return err
			}
			action, after = "user.disable", map[string]any{"disabled": true, "sessionsRevoked": n}
		}
		return s.record(r, q, p, store.AuditEvent{Action: action, TargetType: "user", TargetID: &uid,
			Before: map[string]any{"disabled": u.DisabledAt != nil}, After: after})
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out.API())
	return nil
}
