package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/auth"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
)

func (s *Server) listInvitations(w http.ResponseWriter, r *http.Request, p *principal) error {
	return s.read(r, p, api.CapInvitationsManage, func(q store.Querier, sc *scope) error {
		is, err := s.store.Invitations(r.Context(), q, sc.period.ID, sc.division.ID)
		if err != nil {
			return err
		}
		now, out := s.now(), []api.Invitation{}
		for _, i := range is {
			out = append(out, i.API(now))
		}
		writeJSON(w, http.StatusOK, api.List[api.Invitation]{Items: out})
		return nil
	})
}

// allowMail limits the emails sent to one address, whoever asks.
func (s *Server) allowMail(email string) error {
	if !s.mailLimit.allow("mail:"+email, s.now()) {
		return errRateLimited
	}
	return nil
}

// allowPublic limits the requests of one address to public endpoints.
func (s *Server) allowPublic(r *http.Request) error {
	if !s.publicLimit.allow("ip:"+clientIP(r), s.now()) {
		return errRateLimited
	}
	return nil
}

// invite invites an email to the division as a member (ACC-02). The token
// goes only into the email; positions are given separately.
func (s *Server) invite(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.InviteRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	email, err := validEmail(req.Email)
	if err != nil {
		return err
	}
	var out api.Invitation
	err = s.write(r, p, api.CapInvitationsManage, store.LockShare, func(q store.Querier, sc *scope) error {
		ctx, now := r.Context(), s.now()
		u, err := s.store.UserByEmailQ(ctx, q, email)
		switch {
		case err == nil && u.Kind == api.AccountOrganizationAdmin:
			return badRequest("Akun admin organisasi tidak dapat menjadi anggota divisi; undang email pribadi.")
		case err == nil:
			if _, err := s.store.ActiveMembership(ctx, q, sc.period.ID, sc.division.ID, u.ID); err == nil {
				return conflict("Email ini sudah menjadi anggota divisi pada periode ini.")
			} else if !errors.Is(err, store.ErrNotFound) {
				return err
			}
		case !errors.Is(err, store.ErrNotFound):
			return err
		}
		if err := s.allowMail(email); err != nil {
			return err
		}
		token, hash, err := auth.NewToken()
		if err != nil {
			return err
		}
		inv := store.Invitation{ID: id.New(), PeriodID: sc.period.ID, DivisionID: sc.division.ID, Email: email,
			InvitedBy: p.User.ID, CreatedAt: now, SentAt: now, ExpiresAt: now.Add(invitationTTL)}
		if err := s.store.CreateInvitation(ctx, q, inv, hash); errors.Is(err, store.ErrConflict) {
			return conflict("Email ini sudah memiliki undangan yang belum diterima; kirim ulang undangan tersebut.")
		} else if err != nil {
			return err
		}
		out = inv.API(now)
		if err := s.record(r, q, p, sc.audit(store.AuditEvent{Action: "invitation.create", TargetType: "invitation", TargetID: &inv.ID, After: out})); err != nil {
			return err
		}
		// Last, so that nothing after it can fail and roll back an invitation
		// whose email is already sent.
		return s.send(ctx, invitationMail(s, email, sc.division.Name, sc.period.Label, token, inv.ExpiresAt))
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, out)
	return nil
}

// resendInvitation sends an invitation again with a new token, valid seven
// days from now; the previous token and email proofs stop working.
func (s *Server) resendInvitation(w http.ResponseWriter, r *http.Request, p *principal) error {
	iid, err := pathID(r, "invitationId")
	if err != nil {
		return err
	}
	var out api.Invitation
	err = s.write(r, p, api.CapInvitationsManage, store.LockShare, func(q store.Querier, sc *scope) error {
		ctx, now := r.Context(), s.now()
		inv, err := s.store.InvitationInScope(ctx, q, sc.period.ID, sc.division.ID, iid, store.LockUpdate)
		if err != nil {
			return notFound(err, errNotFound)
		}
		switch inv.Status(now) {
		case api.InvitationAccepted:
			return conflict("Undangan ini sudah diterima.")
		case api.InvitationRevoked:
			return conflict("Undangan ini sudah dicabut; buat undangan baru.")
		}
		if err := s.allowMail(inv.Email); err != nil {
			return err
		}
		token, hash, err := auth.NewToken()
		if err != nil {
			return err
		}
		before := inv.API(now)
		inv.SentAt, inv.ExpiresAt = now, now.Add(invitationTTL)
		if err := s.store.RotateInvitation(ctx, q, inv.ID, hash, inv.SentAt, inv.ExpiresAt); err != nil {
			return err
		}
		if err := s.store.UseInvitationTokens(ctx, q, inv.ID, now); err != nil {
			return err
		}
		out = inv.API(now)
		if err := s.record(r, q, p, sc.audit(store.AuditEvent{Action: "invitation.resend", TargetType: "invitation", TargetID: &inv.ID, Before: before, After: out})); err != nil {
			return err
		}
		return s.send(ctx, invitationMail(s, inv.Email, sc.division.Name, sc.period.Label, token, inv.ExpiresAt))
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) revokeInvitation(w http.ResponseWriter, r *http.Request, p *principal) error {
	iid, err := pathID(r, "invitationId")
	if err != nil {
		return err
	}
	err = s.write(r, p, api.CapInvitationsManage, store.LockShare, func(q store.Querier, sc *scope) error {
		ctx, now := r.Context(), s.now()
		inv, err := s.store.InvitationInScope(ctx, q, sc.period.ID, sc.division.ID, iid, store.LockUpdate)
		if err != nil {
			return notFound(err, errNotFound)
		}
		switch inv.Status(now) {
		case api.InvitationAccepted:
			return conflict("Undangan ini sudah diterima; keluarkan anggotanya bila perlu.")
		case api.InvitationRevoked:
			return nil
		}
		before := inv.API(now)
		if err := s.store.RevokeInvitation(ctx, q, inv.ID, p.User.ID, now); err != nil {
			return err
		}
		if err := s.store.UseInvitationTokens(ctx, q, inv.ID, now); err != nil {
			return err
		}
		return s.record(r, q, p, sc.audit(store.AuditEvent{Action: "invitation.revoke", TargetType: "invitation", TargetID: &inv.ID,
			Before: before, After: map[string]any{"status": api.InvitationRevoked}}))
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// usableInvitation returns the invitation of a token when it may still be
// accepted: pending, in a period not closed. Every other case is the same
// invalid token.
func (s *Server) usableInvitation(r *http.Request, q store.Querier, token, lock string) (store.Invitation, api.Period, error) {
	hash, err := tokenHash(token)
	if err != nil {
		return store.Invitation{}, api.Period{}, err
	}
	inv, err := s.store.InvitationByToken(r.Context(), q, hash, lock)
	if err != nil {
		return inv, api.Period{}, notFound(err, errInvalidToken)
	}
	if inv.Status(s.now()) != api.InvitationPending {
		return inv, api.Period{}, errInvalidToken
	}
	period, err := s.store.Period(r.Context(), q, inv.PeriodID, store.LockShare)
	if err != nil {
		return inv, period, err
	}
	if period.Status == api.PeriodClosed {
		return inv, period, errInvalidToken
	}
	return inv, period, nil
}

// inspectInvitation shows the holder of a link what it invites to, and
// whether to sign in or create an account.
func (s *Server) inspectInvitation(w http.ResponseWriter, r *http.Request) error {
	if err := s.allowPublic(r); err != nil {
		return err
	}
	var req api.InvitationTokenRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	q := s.store.DB()
	inv, period, err := s.usableInvitation(r, q, req.Token, store.LockNone)
	if err != nil {
		return err
	}
	div, err := s.store.PeriodDivision(r.Context(), q, inv.PeriodID, inv.DivisionID)
	if err != nil {
		return err
	}
	_, err = s.store.UserByEmail(r.Context(), inv.Email)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	writeJSON(w, http.StatusOK, api.InvitationPreview{Email: inv.Email, PeriodID: period.ID, PeriodLabel: period.Label,
		DivisionID: div.ID, DivisionName: div.Name, ExpiresAt: inv.ExpiresAt.UTC(), AccountExists: err == nil})
	return nil
}

// verifyInvitationEmail sends a proof, valid one hour, to the invited
// address. Holding the link, which may travel by WhatsApp, does not prove
// the email; opening the proof does (ACC-03).
func (s *Server) verifyInvitationEmail(w http.ResponseWriter, r *http.Request) error {
	if err := s.allowPublic(r); err != nil {
		return err
	}
	var req api.InvitationTokenRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	err := s.store.Tx(r.Context(), func(q store.Querier) error {
		ctx, now := r.Context(), s.now()
		inv, _, err := s.usableInvitation(r, q, req.Token, store.LockUpdate)
		if err != nil {
			return err
		}
		if _, err := s.store.UserByEmailQ(ctx, q, inv.Email); err == nil {
			return conflict("Email ini sudah memiliki akun Space; masuk lalu terima undangan.")
		} else if !errors.Is(err, store.ErrNotFound) {
			return err
		}
		if err := s.allowMail(inv.Email); err != nil {
			return err
		}
		proof, hash, err := auth.NewToken()
		if err != nil {
			return err
		}
		// Only the latest proof works.
		if err := s.store.UseInvitationTokens(ctx, q, inv.ID, now); err != nil {
			return err
		}
		if err := s.store.CreateAuthToken(ctx, q, store.AuthToken{Purpose: store.TokenInvitationEmail, Email: inv.Email,
			InvitationID: &inv.ID, ExpiresAt: now.Add(proofTTL)}, hash, now); err != nil {
			return err
		}
		if err := s.record(r, q, nil, store.AuditEvent{ActorKind: "system", Action: "invitation.email_proof.send", PeriodID: &inv.PeriodID,
			ScopeType: "division", ScopeID: &inv.DivisionID, TargetType: "invitation", TargetID: &inv.ID}); err != nil {
			return err
		}
		return s.send(ctx, proofMail(s, inv.Email, strings.TrimSpace(req.Token), proof))
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusAccepted)
	return nil
}

// acceptInvitation turns an invitation into a membership, once (ACC-03,
// ACC-04). Without a session it creates the account, with the email proof;
// with one, the account must be the personal account of the invited email.
func (s *Server) acceptInvitation(w http.ResponseWriter, r *http.Request) error {
	if err := s.allowPublic(r); err != nil {
		return err
	}
	var p *principal
	if r.Header.Get("Authorization") != "" {
		var err error
		if p, err = s.principal(r); err != nil {
			return err
		}
	}
	var req api.InvitationAcceptRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	var name, hash string
	if p != nil {
		if req.VerificationToken != "" || req.DisplayName != "" || req.Password != "" {
			return badRequest("Akun yang sudah masuk cukup mengirim token undangan.")
		}
		if p.admin() {
			return fail(http.StatusForbidden, api.CodeForbidden, "Akun admin organisasi tidak dapat menerima keanggotaan divisi.")
		}
	} else {
		var err error
		if name, err = text(req.DisplayName, "nama", 1, 120); err != nil {
			return err
		}
		if msg := auth.CheckPassword(req.Password); msg != "" {
			return badRequest(msg)
		}
		if req.VerificationToken == "" {
			return badRequest("Verifikasi email terlebih dahulu melalui tautan yang dikirim ke email undangan.")
		}
		if hash, err = auth.HashPassword(req.Password); err != nil {
			return err
		}
	}
	var out api.InvitationAcceptResponse
	err := s.store.Tx(r.Context(), func(q store.Querier) error {
		ctx, now := r.Context(), s.now()
		inv, _, err := s.usableInvitation(r, q, req.Token, store.LockUpdate)
		if err != nil {
			return err
		}
		var u store.User
		if p != nil {
			if u, err = s.lockedAccount(r, q, p); err != nil {
				return err
			}
			if u.Email != inv.Email || u.EmailVerifiedAt == nil {
				return fail(http.StatusForbidden, api.CodeForbidden, "Undangan ini untuk alamat email lain; masuk dengan akun email undangan.")
			}
		} else {
			proofHash, err := tokenHash(req.VerificationToken)
			if err != nil {
				return err
			}
			proof, err := s.store.UsableAuthToken(ctx, q, proofHash, store.TokenInvitationEmail, now)
			if err != nil {
				return notFound(err, errInvalidToken)
			}
			if proof.InvitationID == nil || *proof.InvitationID != inv.ID || proof.Email != inv.Email {
				return errInvalidToken
			}
			u, err = s.store.CreateUser(ctx, q, store.User{Kind: api.AccountPerson, Email: inv.Email, DisplayName: name,
				PasswordHash: hash, EmailVerifiedAt: &now})
			if errors.Is(err, store.ErrEmailTaken) {
				return conflict("Email ini sudah memiliki akun Space; masuk lalu terima undangan.")
			}
			if err != nil {
				return err
			}
			if err := s.record(r, q, nil, store.AuditEvent{ActorUserID: &u.ID, ActorKind: u.Kind, Action: "user.create",
				TargetType: "user", TargetID: &u.ID, After: map[string]any{"kind": u.Kind, "email": u.Email, "displayName": u.DisplayName, "invitationId": inv.ID}}); err != nil {
				return err
			}
		}
		if _, err := s.store.CreateMembership(ctx, q, inv.PeriodID, inv.DivisionID, u.ID, api.RoleMember, "invitation", &inv.ID, now); errors.Is(err, store.ErrConflict) {
			return conflict("Akun ini sudah menjadi anggota divisi pada periode tersebut.")
		} else if err != nil {
			return err
		}
		if err := s.store.MarkInvitationAccepted(ctx, q, inv.ID, u.ID, now); err != nil {
			return err
		}
		if err := s.store.UseInvitationTokens(ctx, q, inv.ID, now); err != nil {
			return err
		}
		m, err := s.store.ActiveMembership(ctx, q, inv.PeriodID, inv.DivisionID, u.ID)
		if err != nil {
			return err
		}
		out = api.InvitationAcceptResponse{User: u.API(), Membership: memberAPI(m, nil, false)}
		return s.record(r, q, nil, store.AuditEvent{ActorUserID: &u.ID, ActorKind: u.Kind, Action: "invitation.accept",
			PeriodID: &inv.PeriodID, ScopeType: "division", ScopeID: &inv.DivisionID, TargetType: "invitation", TargetID: &inv.ID,
			After: map[string]any{"membershipId": m.ID, "userId": u.ID, "role": m.Role}})
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}
