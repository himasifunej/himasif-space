package server

import (
	"errors"
	"net/http"
	"slices"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/authz"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
)

// read runs fn with the scope of r when the caller holds c there.
func (s *Server) read(r *http.Request, p *principal, c string, fn func(q store.Querier, sc *scope) error) error {
	q := s.store.DB()
	sc, err := s.loadScope(r.Context(), q, r, p, store.LockNone)
	if err != nil {
		return err
	}
	if err := sc.require(c); err != nil {
		return err
	}
	return fn(q, sc)
}

// write runs fn in a transaction with the scope of r, loaded in it, when
// the caller holds c there. lock is how the period is read: LockShare, or
// LockUpdate for changes that must not interleave with any other write of
// the period, such as who is kadiv.
func (s *Server) write(r *http.Request, p *principal, c, lock string, fn func(q store.Querier, sc *scope) error) error {
	return s.store.Tx(r.Context(), func(q store.Querier) error {
		sc, err := s.loadScope(r.Context(), q, r, p, lock)
		if err != nil {
			return err
		}
		if err := sc.require(c); err != nil {
			return err
		}
		return fn(q, sc)
	})
}

func (sc *scope) audit(e store.AuditEvent) store.AuditEvent {
	e.PeriodID = &sc.period.ID
	e.ScopeType, e.ScopeID = "division", &sc.division.ID
	return e
}

// member returns the active membership of a user in the scope, as Member.
func (s *Server) member(r *http.Request, q store.Querier, sc *scope, uid id.ID) (api.Member, error) {
	m, err := s.store.ActiveMembership(r.Context(), q, sc.period.ID, sc.division.ID, uid)
	if err != nil {
		return api.Member{}, err
	}
	roles, err := s.store.CustomRoles(r.Context(), q, sc.period.ID, sc.division.ID)
	if err != nil {
		return api.Member{}, err
	}
	return memberAPI(m, rolesByID(roles), sc.closed()), nil
}

// listMembers lists the division's directory: its members and the admin
// may read it; an archive grant of a space does not open it.
func (s *Server) listMembers(w http.ResponseWriter, r *http.Request, p *principal) error {
	return s.read(r, p, api.CapMembersRead, func(q store.Querier, sc *scope) error {
		ms, err := s.store.DivisionMemberships(r.Context(), q, sc.period.ID, sc.division.ID)
		if err != nil {
			return err
		}
		roles, err := s.store.CustomRoles(r.Context(), q, sc.period.ID, sc.division.ID)
		if err != nil {
			return err
		}
		byID := rolesByID(roles)
		out := []api.Member{}
		for _, m := range ms {
			out = append(out, memberAPI(m, byID, sc.closed()))
		}
		writeJSON(w, http.StatusOK, api.List[api.Member]{Items: out})
		return nil
	})
}

// setKadiv makes a verified personal account the division's kadiv, which
// only the admin may do. The previous kadiv stays a member; nothing private
// of theirs passes on. It is refused while a 45 is active.
func (s *Server) setKadiv(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.SetKadivRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	var out api.Member
	err := s.write(r, p, api.CapKadivManage, store.LockUpdate, func(q store.Querier, sc *scope) error {
		ctx, now := r.Context(), s.now()
		u, err := s.store.UserForUpdate(ctx, q, req.UserID)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return err
		}
		if err != nil || u.Kind != api.AccountPerson || u.EmailVerifiedAt == nil || u.DisabledAt != nil {
			return badRequest("Pilih akun pribadi aktif yang emailnya sudah terverifikasi.")
		}
		if _, err := s.store.Active45(ctx, q, sc.period.ID, sc.division.ID); err == nil {
			return conflict("Divisi ini masih memiliki 45 aktif; kadiv asli harus mencopotnya terlebih dahulu.")
		} else if !errors.Is(err, store.ErrNotFound) {
			return err
		}
		var prev *id.ID
		old, err := s.store.Kadiv(ctx, q, sc.period.ID, sc.division.ID)
		switch {
		case err == nil && old.UserID == u.ID:
			out, err = s.member(r, q, sc, u.ID)
			return err
		case err == nil:
			prev = &old.UserID
			if err := s.store.SetMembershipRole(ctx, q, old.ID, api.RoleMember, now); err != nil {
				return err
			}
		case !errors.Is(err, store.ErrNotFound):
			return err
		}
		m, err := s.store.ActiveMembership(ctx, q, sc.period.ID, sc.division.ID, u.ID)
		switch {
		case err == nil:
			err = s.store.SetMembershipRole(ctx, q, m.ID, api.RoleKadiv, now)
		case errors.Is(err, store.ErrNotFound):
			_, err = s.store.CreateMembership(ctx, q, sc.period.ID, sc.division.ID, u.ID, api.RoleKadiv, "kadiv_assignment", nil, now)
		}
		if errors.Is(err, store.ErrConflict) {
			return conflict("Kadiv divisi ini baru saja berubah; muat ulang lalu coba lagi.")
		}
		if err != nil {
			return err
		}
		if out, err = s.member(r, q, sc, u.ID); err != nil {
			return err
		}
		return s.record(r, q, p, sc.audit(store.AuditEvent{Action: "division.kadiv.set", TargetType: "user", TargetID: &u.ID,
			Before: map[string]any{"kadivUserId": prev}, After: map[string]any{"kadivUserId": u.ID}}))
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// updateMember replaces a member's role (member or companion), groups and
// custom roles. Kadiv and 45 are not given here.
func (s *Server) updateMember(w http.ResponseWriter, r *http.Request, p *principal) error {
	uid, err := pathID(r, "userId")
	if err != nil {
		return err
	}
	var req api.UpdateMemberRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	if req.Role == nil || req.Groups == nil || req.CustomRoleIDs == nil {
		return badRequest("Isi role, groups dan customRoleIds.")
	}
	if *req.Role != api.RoleMember && *req.Role != api.RoleCompanion {
		return badRequest("Role harus member atau companion; kadiv ditetapkan admin dan 45 ditunjuk kadiv.")
	}
	groups := slices.Clone(req.Groups)
	for _, g := range groups {
		if g != api.GroupTekno && g != api.GroupDesain && g != api.GroupVideo {
			return badRequest("Pengelompokan harus tekno, desain atau video.")
		}
	}
	groups = authz.Normalize(groups)
	roleIDs := slices.Clone(req.CustomRoleIDs)
	slices.SortFunc(roleIDs, func(a, b id.ID) int { return slices.Compare(a[:], b[:]) })
	roleIDs = slices.Compact(roleIDs)
	var out api.Member
	err = s.write(r, p, api.CapMembersManage, store.LockShare, func(q store.Querier, sc *scope) error {
		ctx := r.Context()
		if len(groups) > 0 && sc.division.Slug != "mediatek" {
			return badRequest("Pengelompokan tekno/desain/video hanya untuk Mediatek.")
		}
		before, err := s.member(r, q, sc, uid)
		if err != nil {
			return notFound(err, errNotFound)
		}
		if before.Role == api.RoleKadiv {
			return conflict("Jabatan kadiv diubah melalui penetapan kadiv oleh admin.")
		}
		roles, err := s.store.CustomRoles(ctx, q, sc.period.ID, sc.division.ID)
		if err != nil {
			return err
		}
		byID := rolesByID(roles)
		for _, rid := range roleIDs {
			if _, ok := byID[rid]; !ok {
				return badRequest("Ada role kustom yang bukan milik divisi ini.")
			}
		}
		if err := s.store.ConfigureMembership(ctx, q, before.ID, *req.Role, groups, roleIDs, s.now()); err != nil {
			return err
		}
		if out, err = s.member(r, q, sc, uid); err != nil {
			return err
		}
		return s.record(r, q, p, sc.audit(store.AuditEvent{Action: "member.update", TargetType: "membership", TargetID: &out.ID,
			Before: memberConfig(before), After: memberConfig(out)}))
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func memberConfig(m api.Member) map[string]any {
	return map[string]any{"userId": m.UserID, "role": m.Role, "groups": m.Groups, "customRoleIds": m.CustomRoleIDs, "capabilities": m.Capabilities}
}

// removeMember ends a membership. The kadiv and an active 45 are removed
// from their position first.
func (s *Server) removeMember(w http.ResponseWriter, r *http.Request, p *principal) error {
	uid, err := pathID(r, "userId")
	if err != nil {
		return err
	}
	err = s.write(r, p, api.CapMembersManage, store.LockShare, func(q store.Querier, sc *scope) error {
		m, err := s.store.ActiveMembership(r.Context(), q, sc.period.ID, sc.division.ID, uid)
		if err != nil {
			return notFound(err, errNotFound)
		}
		if m.Role == api.RoleKadiv {
			return conflict("Kadiv tidak dapat dikeluarkan; tetapkan kadiv lain terlebih dahulu.")
		}
		if m.Acting45 != nil {
			return conflict("Anggota ini masih 45; kadiv asli harus mencopotnya terlebih dahulu.")
		}
		if err := s.store.RemoveMembership(r.Context(), q, m.ID, s.now()); err != nil {
			return err
		}
		return s.record(r, q, p, sc.audit(store.AuditEvent{Action: "member.remove", TargetType: "membership", TargetID: &m.ID,
			Before: map[string]any{"userId": m.UserID, "role": m.Role, "groups": m.Groups, "customRoleIds": m.CustomRoleIDs}}))
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) listRoles(w http.ResponseWriter, r *http.Request, p *principal) error {
	return s.read(r, p, api.CapMembersRead, func(q store.Querier, sc *scope) error {
		rs, err := s.store.CustomRoles(r.Context(), q, sc.period.ID, sc.division.ID)
		if err != nil {
			return err
		}
		writeJSON(w, http.StatusOK, api.List[api.CustomRole]{Items: nonNil(rs)})
		return nil
	})
}

func readRole(r *http.Request) (string, []string, error) {
	var req api.RoleRequest
	if err := readJSON(r, &req); err != nil {
		return "", nil, err
	}
	name, err := text(req.Name, "nama role", 1, 60)
	if err != nil {
		return "", nil, err
	}
	for _, c := range req.Capabilities {
		if !authz.ValidCustom(c) {
			return "", nil, badRequest("Role kustom hanya boleh berisi kemampuan operasional.")
		}
	}
	return name, authz.Normalize(req.Capabilities), nil
}

// saveRole adds (roleId absent) or changes a custom role.
func (s *Server) saveRole(w http.ResponseWriter, r *http.Request, p *principal) error {
	name, caps, err := readRole(r)
	if err != nil {
		return err
	}
	update := r.PathValue("roleId") != ""
	var rid id.ID
	if update {
		if rid, err = pathID(r, "roleId"); err != nil {
			return err
		}
	}
	var out api.CustomRole
	err = s.write(r, p, api.CapRolesManage, store.LockShare, func(q store.Querier, sc *scope) error {
		ctx := r.Context()
		var before any
		action := "role.create"
		out = api.CustomRole{ID: id.New(), PeriodID: sc.period.ID, DivisionID: sc.division.ID, Name: name, Capabilities: caps}
		if update {
			old, err := s.store.CustomRole(ctx, q, sc.period.ID, sc.division.ID, rid)
			if err != nil {
				return notFound(err, errNotFound)
			}
			before, out.ID, action = old, old.ID, "role.update"
		}
		if err := s.store.SaveCustomRole(ctx, q, out, s.now()); errors.Is(err, store.ErrConflict) {
			return conflict("Nama role sudah dipakai di divisi ini.")
		} else if err != nil {
			return err
		}
		return s.record(r, q, p, sc.audit(store.AuditEvent{Action: action, TargetType: "custom_role", TargetID: &out.ID, Before: before, After: out}))
	})
	if err != nil {
		return err
	}
	status := http.StatusCreated
	if update {
		status = http.StatusOK
	}
	writeJSON(w, status, out)
	return nil
}

func (s *Server) deleteRole(w http.ResponseWriter, r *http.Request, p *principal) error {
	rid, err := pathID(r, "roleId")
	if err != nil {
		return err
	}
	err = s.write(r, p, api.CapRolesManage, store.LockShare, func(q store.Querier, sc *scope) error {
		old, err := s.store.CustomRole(r.Context(), q, sc.period.ID, sc.division.ID, rid)
		if err != nil {
			return notFound(err, errNotFound)
		}
		held, err := s.store.DeleteCustomRole(r.Context(), q, rid)
		if err != nil {
			return err
		}
		return s.record(r, q, p, sc.audit(store.AuditEvent{Action: "role.delete", TargetType: "custom_role", TargetID: &rid,
			Before: map[string]any{"role": old, "membershipIds": nonNil(held)}}))
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// appoint45 appoints the division's 45: only the original kadiv may, at
// most one is active, and it lasts until the kadiv removes it (ACC-08).
func (s *Server) appoint45(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.Appoint45Request
	if err := readJSON(r, &req); err != nil {
		return err
	}
	caps := authz.Normalize(authz.Default45)
	if req.Capabilities != nil {
		if len(req.Capabilities) == 0 {
			return badRequest("Pilih minimal satu kemampuan 45, atau kosongkan untuk bawaan.")
		}
		for _, c := range req.Capabilities {
			if !authz.Valid45(c) {
				return badRequest("Kemampuan 45 hanya tasks.manage, results.review, ping.send dan whatsapp.manage.")
			}
		}
		caps = authz.Normalize(req.Capabilities)
	}
	var out api.Appointment45
	err := s.write(r, p, api.Cap45Manage, store.LockShare, func(q store.Querier, sc *scope) error {
		m, err := s.store.ActiveMembership(r.Context(), q, sc.period.ID, sc.division.ID, req.UserID)
		if err != nil {
			return notFound(err, badRequest("Pilih anggota aktif divisi ini."))
		}
		if m.Role == api.RoleKadiv {
			return badRequest("Kadiv tidak dapat menjadi 45 di divisinya sendiri.")
		}
		a := store.Appointment45{ID: id.New(), PeriodID: sc.period.ID, DivisionID: sc.division.ID, UserID: m.UserID,
			Capabilities: caps, AppointedBy: p.User.ID, AppointedAt: s.now()}
		if err := s.store.Appoint45(r.Context(), q, a); errors.Is(err, store.ErrConflict) {
			return conflict("Divisi ini sudah memiliki 45 aktif.")
		} else if err != nil {
			return err
		}
		out = a.API()
		return s.record(r, q, p, sc.audit(store.AuditEvent{Action: "division.45.appoint", TargetType: "acting45", TargetID: &a.ID, After: out}))
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, out)
	return nil
}

// remove45 ends the appointment; the membership and its other roles stay,
// and the added capabilities are gone on the next request (AC-08).
func (s *Server) remove45(w http.ResponseWriter, r *http.Request, p *principal) error {
	err := s.write(r, p, api.Cap45Manage, store.LockShare, func(q store.Querier, sc *scope) error {
		a, err := s.store.Active45(r.Context(), q, sc.period.ID, sc.division.ID)
		if err != nil {
			return notFound(err, errNotFound)
		}
		if err := s.store.Remove45(r.Context(), q, a.ID, p.User.ID, s.now()); err != nil {
			return err
		}
		return s.record(r, q, p, sc.audit(store.AuditEvent{Action: "division.45.remove", TargetType: "acting45", TargetID: &a.ID, Before: a.API()}))
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) listArchives(w http.ResponseWriter, r *http.Request, p *principal) error {
	return s.read(r, p, api.CapArchivesManage, func(q store.Querier, sc *scope) error {
		gs, err := s.store.ArchiveGrants(r.Context(), q, sc.period.ID, sc.division.ID)
		if err != nil {
			return err
		}
		writeJSON(w, http.StatusOK, api.List[api.ArchiveGrant]{Items: nonNil(gs)})
		return nil
	})
}

// grantArchive lets a member of the scope read one space of a closed
// period that includes the scope's division (PER-04). It opens neither the
// division's directory nor anything private.
func (s *Server) grantArchive(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.ArchiveRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	var out api.ArchiveGrant
	err := s.write(r, p, api.CapArchivesManage, store.LockShare, func(q store.Querier, sc *scope) error {
		ctx := r.Context()
		sp, err := s.store.SpaceByID(ctx, q, req.SpaceID)
		if err != nil {
			return notFound(err, badRequest("Space arsip tidak ditemukan."))
		}
		src, err := s.store.Period(ctx, q, sp.PeriodID, store.LockShare)
		if err != nil {
			return err
		}
		if src.Status != api.PeriodClosed {
			return badRequest("Akses arsip hanya untuk Space periode yang sudah ditutup.")
		}
		if !sp.HasDivision(sc.division.ID) {
			return errForbidden
		}
		if _, err := s.store.ActiveMembership(ctx, q, sc.period.ID, sc.division.ID, req.UserID); err != nil {
			return notFound(err, badRequest("Akses arsip hanya untuk anggota aktif divisi ini."))
		}
		out = api.ArchiveGrant{ID: id.New(), PeriodID: sc.period.ID, DivisionID: sc.division.ID, UserID: req.UserID,
			SpaceID: sp.ID, GrantedBy: p.User.ID, CreatedAt: s.now().UTC()}
		if err := s.store.CreateArchiveGrant(ctx, q, out); errors.Is(err, store.ErrConflict) {
			return conflict("Anggota ini sudah memiliki akses arsip Space tersebut.")
		} else if err != nil {
			return err
		}
		return s.record(r, q, p, sc.audit(store.AuditEvent{Action: "archive.grant", TargetType: "archive_grant", TargetID: &out.ID, After: out}))
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, out)
	return nil
}

func (s *Server) revokeArchive(w http.ResponseWriter, r *http.Request, p *principal) error {
	gid, err := pathID(r, "grantId")
	if err != nil {
		return err
	}
	err = s.write(r, p, api.CapArchivesManage, store.LockShare, func(q store.Querier, sc *scope) error {
		g, err := s.store.ArchiveGrant(r.Context(), q, sc.period.ID, sc.division.ID, gid)
		if err != nil {
			return notFound(err, errNotFound)
		}
		if err := s.store.RevokeArchiveGrant(r.Context(), q, gid, p.User.ID, s.now()); err != nil {
			return err
		}
		return s.record(r, q, p, sc.audit(store.AuditEvent{Action: "archive.revoke", TargetType: "archive_grant", TargetID: &gid, Before: g}))
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
