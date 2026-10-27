package server

import (
	"context"
	"net/http"
	"regexp"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

var linkParam = regexp.MustCompile(`[?&](token|verification)=([A-Za-z0-9_-]+)`)

// param returns a parameter of the link in an email.
func param(t *testing.T, text, name string) string {
	t.Helper()
	for _, m := range linkParam.FindAllStringSubmatch(text, -1) {
		if m[1] == name {
			return m[2]
		}
	}
	t.Fatalf("no %s in email %q", name, text)
	return ""
}

// org is the seeded organization, seen by the admin.
type org struct {
	*env
	admin  string
	period api.Period
	div    map[string]api.Division
}

func newOrg(t *testing.T) *org {
	e := newEnv(t, "")
	code, r, bad := e.login(adminEmail, adminPassword)
	if code != 200 {
		t.Fatalf("admin login %d %+v", code, bad)
	}
	o := &org{env: e, admin: r.Token, div: map[string]api.Division{}}
	var ps api.List[api.Period]
	e.call("GET", "/api/periods", o.admin, nil, &ps)
	o.period = ps.Items[0]
	var ds api.List[api.Division]
	e.call("GET", "/api/divisions", o.admin, nil, &ds)
	for _, d := range ds.Items {
		o.div[d.Slug] = d
	}
	return o
}

func (o *org) scope(pid id.ID, slug string) string {
	return "/api/periods/" + pid.String() + "/divisions/" + o.div[slug].ID.String()
}

// join invites email to a division of a period as inviter and accepts it
// with a new account, returning the new account's session.
func (o *org) join(inviter string, pid id.ID, slug, email, name string) (string, api.InvitationAcceptResponse) {
	o.t.Helper()
	var inv api.Invitation
	if c := o.call("POST", o.scope(pid, slug)+"/invitations", inviter, api.InviteRequest{Email: email}, &inv); c != 201 {
		o.t.Fatalf("invite %s = %d", email, c)
	}
	token := param(o.t, o.mail.last(o.t, email).Text, "token")
	if c := o.call("POST", "/api/auth/invitations/verify", "", api.InvitationTokenRequest{Token: token}, nil); c != 202 {
		o.t.Fatalf("verify = %d", c)
	}
	proof := param(o.t, o.mail.last(o.t, email).Text, "verification")
	var acc api.InvitationAcceptResponse
	if c := o.call("POST", "/api/auth/invitations/accept", "", api.InvitationAcceptRequest{Token: token, VerificationToken: proof,
		DisplayName: name, Password: "kata-sandi-anggota"}, &acc); c != 200 {
		o.t.Fatalf("accept = %d", c)
	}
	code, r, _ := o.login(email, "kata-sandi-anggota")
	if code != 200 {
		o.t.Fatalf("login %s = %d", email, code)
	}
	return r.Token, acc
}

// kadiv makes a new account kadiv of a division in the seeded period.
func (o *org) kadiv(slug, email string) (string, id.ID) {
	o.t.Helper()
	tok, acc := o.join(o.admin, o.period.ID, slug, email, "Kadiv "+slug)
	var m api.Member
	if c := o.call("PUT", o.scope(o.period.ID, slug)+"/kadiv", o.admin, api.SetKadivRequest{UserID: acc.User.ID}, &m); c != 200 || m.Role != api.RoleKadiv {
		o.t.Fatalf("set kadiv = %d %+v", c, m)
	}
	return tok, acc.User.ID
}

func (o *org) errCode(method, path, token string, body any) (int, string) {
	o.t.Helper()
	var er api.ErrorResponse
	c := o.call(method, path, token, body, &er)
	return c, er.Error.Code
}

func (o *org) audits(action string) int {
	var n int
	if err := o.st.DB().QueryRow(context.Background(), `select count(*) from audit_events where action = $1`, action).Scan(&n); err != nil {
		o.t.Fatal(err)
	}
	return n
}

// AC-01, AC-02: a new account from an invitation; an existing account
// joins a new period and stays one account.
func TestInvitationNewAndExistingAccount(t *testing.T) {
	o := newOrg(t)
	kTok, _ := o.kadiv("mediatek", "kadiv@himasif.test")

	var inv api.Invitation
	if c := o.call("POST", o.scope(o.period.ID, "mediatek")+"/invitations", kTok, api.InviteRequest{Email: " Anggota@Himasif.Test "}, &inv); c != 201 {
		t.Fatalf("invite = %d", c)
	}
	if inv.Email != "anggota@himasif.test" || inv.Status != api.InvitationPending || inv.ExpiresAt.Sub(inv.CreatedAt) != invitationTTL {
		t.Fatalf("invitation %+v", inv)
	}
	if c, code := o.errCode("POST", o.scope(o.period.ID, "mediatek")+"/invitations", kTok, api.InviteRequest{Email: "anggota@himasif.test"}); c != 409 || code != api.CodeConflict {
		t.Fatalf("duplicate invite = %d %s", c, code)
	}
	token := param(t, o.mail.last(t, "anggota@himasif.test").Text, "token")

	var pre api.InvitationPreview
	if c := o.call("POST", "/api/auth/invitations/inspect", "", api.InvitationTokenRequest{Token: token}, &pre); c != 200 || pre.AccountExists || pre.DivisionName != "Mediatek" {
		t.Fatalf("inspect = %d %+v", c, pre)
	}
	// The link alone does not prove the email.
	if c, code := o.errCode("POST", "/api/auth/invitations/accept", "", api.InvitationAcceptRequest{Token: token, DisplayName: "A", Password: "kata-sandi-anggota"}); c != 400 {
		t.Fatalf("accept without proof = %d %s", c, code)
	}
	o.call("POST", "/api/auth/invitations/verify", "", api.InvitationTokenRequest{Token: token}, nil)
	proof := param(t, o.mail.last(t, "anggota@himasif.test").Text, "verification")
	var acc api.InvitationAcceptResponse
	if c := o.call("POST", "/api/auth/invitations/accept", "", api.InvitationAcceptRequest{Token: token, VerificationToken: proof, DisplayName: "Anggota", Password: "kata-sandi-anggota"}, &acc); c != 200 {
		t.Fatalf("accept = %d", c)
	}
	if !acc.User.EmailVerified || acc.Membership.Role != api.RoleMember || acc.Membership.DivisionID != o.div["mediatek"].ID {
		t.Fatalf("accepted %+v", acc)
	}
	// Once only.
	if c, code := o.errCode("POST", "/api/auth/invitations/accept", "", api.InvitationAcceptRequest{Token: token, VerificationToken: proof, DisplayName: "B", Password: "kata-sandi-anggota"}); c != 410 || code != api.CodeInvalidToken {
		t.Fatalf("second accept = %d %s", c, code)
	}

	// A new period; the same person is invited again and signs in.
	var p2 api.Period
	if c := o.call("POST", "/api/periods", o.admin, api.CreatePeriodRequest{Label: "2026/2027", StartsOn: o.period.EndsOn.AddDays(1), EndsOn: o.period.EndsOn.AddDays(365)}, &p2); c != 201 || p2.Status != api.PeriodPreparing {
		t.Fatalf("create period = %d %+v", c, p2)
	}
	o.call("POST", o.scope(p2.ID, "humas")+"/invitations", o.admin, api.InviteRequest{Email: "anggota@himasif.test"}, nil)
	token2 := param(t, o.mail.last(t, "anggota@himasif.test").Text, "token")
	o.call("POST", "/api/auth/invitations/inspect", "", api.InvitationTokenRequest{Token: token2}, &pre)
	if !pre.AccountExists {
		t.Fatal("existing account not reported")
	}
	// Another account may not take it.
	other, _ := o.join(kTok, o.period.ID, "mediatek", "lain@himasif.test", "Lain")
	if c, _ := o.errCode("POST", "/api/auth/invitations/accept", other, api.InvitationAcceptRequest{Token: token2}); c != 403 {
		t.Fatalf("accept by another account = %d", c)
	}
	_, r, _ := o.login("anggota@himasif.test", "kata-sandi-anggota")
	var acc2 api.InvitationAcceptResponse
	if c := o.call("POST", "/api/auth/invitations/accept", r.Token, api.InvitationAcceptRequest{Token: token2}, &acc2); c != 200 {
		t.Fatalf("existing accept = %d", c)
	}
	if acc2.User.ID != acc.User.ID || acc2.Membership.PeriodID != p2.ID {
		t.Fatalf("not the same account: %+v", acc2)
	}
	// Roles are per period: member of Mediatek then, of Humas now.
	var sp1, sp2 api.List[api.Space]
	o.call("GET", "/api/periods/"+o.period.ID.String()+"/spaces", r.Token, nil, &sp1)
	o.call("GET", "/api/periods/"+p2.ID.String()+"/spaces", r.Token, nil, &sp2)
	codes := func(l api.List[api.Space]) (out []string) {
		for _, s := range l.Items {
			out = append(out, s.Code)
		}
		slices.Sort(out)
		return
	}
	if got := codes(sp1); !slices.Equal(got, []string{"division-mediatek", "medhum"}) {
		t.Errorf("spaces of the first period %v", got)
	}
	if got := codes(sp2); !slices.Equal(got, []string{"division-humas", "medhum"}) {
		t.Errorf("spaces of the new period %v", got)
	}
}

func TestInvitationResendRevokeAndMailFailure(t *testing.T) {
	o := newOrg(t)
	path := o.scope(o.period.ID, "humas") + "/invitations"
	var inv api.Invitation
	o.call("POST", path, o.admin, api.InviteRequest{Email: "x@himasif.test"}, &inv)
	old := param(t, o.mail.last(t, "x@himasif.test").Text, "token")
	o.call("POST", "/api/auth/invitations/verify", "", api.InvitationTokenRequest{Token: old}, nil)
	oldProof := param(t, o.mail.last(t, "x@himasif.test").Text, "verification")

	if c := o.call("POST", path+"/"+inv.ID.String()+"/resend", o.admin, nil, &inv); c != 200 {
		t.Fatalf("resend = %d", c)
	}
	fresh := param(t, o.mail.last(t, "x@himasif.test").Text, "token")
	if c, _ := o.errCode("POST", "/api/auth/invitations/inspect", "", api.InvitationTokenRequest{Token: old}); c != 410 {
		t.Fatalf("old token after resend = %d", c)
	}
	if c, _ := o.errCode("POST", "/api/auth/invitations/accept", "", api.InvitationAcceptRequest{Token: fresh, VerificationToken: oldProof, DisplayName: "X", Password: "kata-sandi-anggota"}); c != 410 {
		t.Fatalf("old proof after resend = %d", c)
	}
	if c := o.call("DELETE", path+"/"+inv.ID.String(), o.admin, nil, nil); c != 204 {
		t.Fatalf("revoke = %d", c)
	}
	if c, _ := o.errCode("POST", "/api/auth/invitations/verify", "", api.InvitationTokenRequest{Token: fresh}); c != 410 {
		t.Fatalf("revoked token = %d", c)
	}
	var list api.List[api.Invitation]
	o.call("GET", path, o.admin, nil, &list)
	if len(list.Items) != 1 || list.Items[0].Status != api.InvitationRevoked {
		t.Fatalf("list %+v", list)
	}

	// A failed email rolls the invitation back.
	o.mail.fail = true
	if c, code := o.errCode("POST", path, o.admin, api.InviteRequest{Email: "y@himasif.test"}); c != 503 || code != api.CodeUnavailable {
		t.Fatalf("invite with failing mail = %d %s", c, code)
	}
	o.mail.fail = false
	if c := o.call("POST", path, o.admin, api.InviteRequest{Email: "y@himasif.test"}, nil); c != 201 {
		t.Fatalf("invite after failure = %d", c)
	}
	// The admin account cannot be invited.
	if c, _ := o.errCode("POST", path, o.admin, api.InviteRequest{Email: adminEmail}); c != 400 {
		t.Fatalf("invite admin = %d", c)
	}
}

// AC-03 (access part), ACC-05, SPC-02: members open their division's space
// and Medhum, not other divisions'.
func TestSpacesAndDirectories(t *testing.T) {
	o := newOrg(t)
	kTok, _ := o.kadiv("mediatek", "km@himasif.test")
	mTok, _ := o.join(kTok, o.period.ID, "mediatek", "m@himasif.test", "Mediatek")
	hTok, _ := o.join(o.admin, o.period.ID, "humas", "h@himasif.test", "Humas")

	var sps api.List[api.Space]
	o.call("GET", "/api/periods/"+o.period.ID.String()+"/spaces", mTok, nil, &sps)
	var medhum, mediatek api.Space
	for _, s := range sps.Items {
		switch s.Code {
		case "medhum":
			medhum = s
		case "division-mediatek":
			mediatek = s
		default:
			t.Errorf("member sees %s", s.Code)
		}
	}
	if medhum.Kind != api.SpaceShared || len(medhum.Divisions) != 2 || len(medhum.Roles) != 1 || medhum.Access != api.AccessMember {
		t.Fatalf("medhum %+v", medhum)
	}
	if !slices.Contains(mediatek.Capabilities, api.CapTasksOwn) || slices.Contains(mediatek.Capabilities, api.CapTasksManage) {
		t.Fatalf("member capabilities %v", mediatek.Capabilities)
	}
	// The kadiv's Medhum holds operations only.
	var kmedhum api.Space
	o.call("GET", "/api/spaces/"+medhum.ID.String(), kTok, nil, &kmedhum)
	if slices.Contains(kmedhum.Capabilities, api.CapInvitationsManage) || !slices.Contains(kmedhum.Capabilities, api.CapWhatsAppManage) {
		t.Fatalf("kadiv medhum %v", kmedhum.Capabilities)
	}
	if c, _ := o.errCode("GET", "/api/spaces/"+mediatek.ID.String(), hTok, nil); c != 403 {
		t.Fatalf("humas opens mediatek = %d", c)
	}
	if c, _ := o.errCode("GET", o.scope(o.period.ID, "mediatek")+"/members", hTok, nil); c != 403 {
		t.Fatalf("humas reads mediatek directory = %d", c)
	}
	var ms api.List[api.Member]
	if c := o.call("GET", o.scope(o.period.ID, "mediatek")+"/members", mTok, nil, &ms); c != 200 || len(ms.Items) != 2 {
		t.Fatalf("members = %d %+v", c, ms)
	}
	for _, path := range []string{"/invitations", "/archives"} {
		if c, _ := o.errCode("GET", o.scope(o.period.ID, "mediatek")+path, mTok, nil); c != 403 {
			t.Errorf("member GET %s = %d", path, c)
		}
	}
	var acc api.Access
	o.call("GET", o.scope(o.period.ID, "humas")+"/access", mTok, nil, &acc)
	if len(acc.Roles) != 0 || len(acc.Capabilities) != 0 {
		t.Fatalf("access outside = %+v", acc)
	}
	// Groups only for Mediatek, and they grant nothing.
	role := api.RoleMember
	var m api.Member
	uid := ms.Items[slices.IndexFunc(ms.Items, func(x api.Member) bool { return x.Role == api.RoleMember })].UserID
	if c := o.call("PATCH", o.scope(o.period.ID, "mediatek")+"/members/"+uid.String(), kTok,
		api.UpdateMemberRequest{Role: &role, Groups: []string{"desain", "video"}, CustomRoleIDs: []id.ID{}}, &m); c != 200 || len(m.Groups) != 2 || slices.Contains(m.Capabilities, api.CapTasksManage) {
		t.Fatalf("groups = %d %+v", c, m)
	}
}

// AC-06, AC-07, AC-08: the 45 operates and manages WhatsApp, never roles or
// access; one at a time; removal takes the extra rights at once.
func Test45(t *testing.T) {
	o := newOrg(t)
	kTok, _ := o.kadiv("mediatek", "km@himasif.test")
	aTok, a := o.join(kTok, o.period.ID, "mediatek", "a@himasif.test", "A")
	_, b := o.join(kTok, o.period.ID, "mediatek", "b@himasif.test", "B")
	sc := o.scope(o.period.ID, "mediatek")

	if c, _ := o.errCode("PUT", sc+"/45", o.admin, api.Appoint45Request{UserID: a.User.ID}); c != 403 {
		t.Fatalf("admin appoints 45 = %d", c)
	}
	// Two at once: exactly one wins.
	var wg sync.WaitGroup
	codes := make([]int, 2)
	for i, u := range []id.ID{a.User.ID, b.User.ID} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i] = o.call("PUT", sc+"/45", kTok, api.Appoint45Request{UserID: u}, nil)
		}()
	}
	wg.Wait()
	slices.Sort(codes)
	if !slices.Equal(codes, []int{201, 409}) {
		t.Fatalf("concurrent appointments %v", codes)
	}
	o.call("DELETE", sc+"/45", kTok, nil, nil)
	var ap api.Appointment45
	if c := o.call("PUT", sc+"/45", kTok, api.Appoint45Request{UserID: a.User.ID}, &ap); c != 201 || !slices.Contains(ap.Capabilities, api.CapWhatsAppManage) {
		t.Fatalf("appoint = %d %+v", c, ap)
	}
	var acc api.Access
	o.call("GET", sc+"/access", aTok, nil, &acc)
	if !slices.Equal(acc.Roles, []string{api.RoleMember, api.Role45}) || !slices.Contains(acc.Capabilities, api.CapWhatsAppManage) || slices.Contains(acc.Capabilities, api.CapRolesManage) {
		t.Fatalf("45 access %+v", acc)
	}
	denied := o.audits("authz.denied")
	for _, tc := range []struct {
		method, path string
		body         any
	}{
		{"POST", sc + "/roles", api.RoleRequest{Name: "X", Capabilities: []string{api.CapPingSend}}},
		{"POST", sc + "/invitations", api.InviteRequest{Email: "z@himasif.test"}},
		{"POST", sc + "/archives", api.ArchiveRequest{UserID: b.User.ID, SpaceID: id.New()}},
		{"PUT", sc + "/45", api.Appoint45Request{UserID: b.User.ID}},
		{"DELETE", sc + "/45", nil},
		{"DELETE", sc + "/members/" + b.User.ID.String(), nil},
	} {
		if c, code := o.errCode(tc.method, tc.path, aTok, tc.body); c != 403 || code != api.CodeForbidden {
			t.Errorf("45 %s %s = %d %s", tc.method, tc.path, c, code)
		}
	}
	if n := o.audits("authz.denied"); n != denied+6 {
		t.Errorf("denied attempts audited %d, want %d", n-denied, 6)
	}
	// Kadiv cannot be replaced, nor the 45 removed as a member, while active.
	if c, _ := o.errCode("PUT", sc+"/kadiv", o.admin, api.SetKadivRequest{UserID: b.User.ID}); c != 409 {
		t.Fatalf("kadiv change with active 45 = %d", c)
	}
	if c, _ := o.errCode("DELETE", sc+"/members/"+a.User.ID.String(), kTok, nil); c != 409 {
		t.Fatalf("remove active 45 = %d", c)
	}
	if c := o.call("DELETE", sc+"/45", kTok, nil, nil); c != 204 {
		t.Fatalf("remove 45 = %d", c)
	}
	o.call("GET", sc+"/access", aTok, nil, &acc)
	if !slices.Equal(acc.Roles, []string{api.RoleMember}) || slices.Contains(acc.Capabilities, api.CapWhatsAppManage) || !slices.Contains(acc.Capabilities, api.CapSpaceRead) {
		t.Fatalf("after removal %+v", acc)
	}
	// Narrowed appointment.
	if c := o.call("PUT", sc+"/45", kTok, api.Appoint45Request{UserID: b.User.ID, Capabilities: []string{api.CapPingSend}}, &ap); c != 201 || !slices.Equal(ap.Capabilities, []string{api.CapPingSend}) {
		t.Fatalf("narrow = %d %+v", c, ap)
	}
	if c, _ := o.errCode("PUT", sc+"/45", kTok, api.Appoint45Request{UserID: a.User.ID, Capabilities: []string{api.CapRolesManage}}); c != 400 {
		t.Fatalf("45 with administration = %d", c)
	}
}

func TestCustomRoles(t *testing.T) {
	o := newOrg(t)
	kTok, _ := o.kadiv("mediatek", "km@himasif.test")
	mTok, m := o.join(kTok, o.period.ID, "mediatek", "m@himasif.test", "M")
	sc := o.scope(o.period.ID, "mediatek")
	if c, _ := o.errCode("POST", sc+"/roles", kTok, api.RoleRequest{Name: "Bos", Capabilities: []string{api.CapMembersManage}}); c != 400 {
		t.Fatalf("administrative custom role = %d", c)
	}
	var role api.CustomRole
	if c := o.call("POST", sc+"/roles", kTok, api.RoleRequest{Name: "Koordinator", Capabilities: []string{api.CapPingSend, api.CapTasksManage}}, &role); c != 201 {
		t.Fatalf("create role = %d", c)
	}
	if c, _ := o.errCode("POST", sc+"/roles", kTok, api.RoleRequest{Name: "koordinator"}); c != 409 {
		t.Fatalf("duplicate name = %d", c)
	}
	r := api.RoleMember
	o.call("PATCH", sc+"/members/"+m.User.ID.String(), kTok, api.UpdateMemberRequest{Role: &r, Groups: []string{}, CustomRoleIDs: []id.ID{role.ID}}, nil)
	var acc api.Access
	o.call("GET", sc+"/access", mTok, nil, &acc)
	if !slices.Contains(acc.Capabilities, api.CapPingSend) || !slices.Equal(acc.CustomRoleIDs, []id.ID{role.ID}) {
		t.Fatalf("with custom role %+v", acc)
	}
	// A role of another division is refused.
	var other api.CustomRole
	o.call("POST", o.scope(o.period.ID, "humas")+"/roles", o.admin, api.RoleRequest{Name: "H", Capabilities: []string{}}, &other)
	if c, _ := o.errCode("PATCH", sc+"/members/"+m.User.ID.String(), kTok, api.UpdateMemberRequest{Role: &r, Groups: []string{}, CustomRoleIDs: []id.ID{other.ID}}); c != 400 {
		t.Fatalf("foreign role = %d", c)
	}
	// Missing fields are refused: the body replaces the configuration.
	if c, _ := o.errCode("PATCH", sc+"/members/"+m.User.ID.String(), kTok, `{"role":"member"}`); c != 400 {
		t.Fatalf("partial patch = %d", c)
	}
	if c := o.call("DELETE", sc+"/roles/"+role.ID.String(), kTok, nil, nil); c != 204 {
		t.Fatalf("delete role = %d", c)
	}
	o.call("GET", sc+"/access", mTok, nil, &acc)
	if slices.Contains(acc.Capabilities, api.CapPingSend) || len(acc.CustomRoleIDs) != 0 {
		t.Fatalf("after delete %+v", acc)
	}
}

// AC-10, PER-03–05: closed periods are read-only; old members keep reading
// their spaces; new members need a grant, of one space.
func TestArchivesAndClosedPeriod(t *testing.T) {
	o := newOrg(t)
	_, old := o.kadiv("mediatek", "lama@himasif.test")
	_, oldPw, _ := o.login("lama@himasif.test", "kata-sandi-anggota")
	oldTok := oldPw.Token

	var p2 api.Period
	o.call("POST", "/api/periods", o.admin, api.CreatePeriodRequest{Label: "2026/2027", StartsOn: o.period.EndsOn.AddDays(1), EndsOn: o.period.EndsOn.AddDays(365)}, &p2)
	if c := o.call("POST", "/api/periods/"+p2.ID.String()+"/activate", o.admin, nil, nil); c != 200 {
		t.Fatalf("activate = %d", c)
	}
	// Handover before closing; then close.
	h := api.HandoverRequest{SuccessorPeriodID: &p2.ID, RemainingWork: "Tidak ada tugas tersisa; semua konten Oktober sudah diposting."}
	var ho api.Handover
	if c := o.call("PUT", "/api/periods/"+o.period.ID.String()+"/handover", o.admin, h, &ho); c != 200 || ho.UpdatedAt == nil {
		t.Fatalf("handover = %d %+v", c, ho)
	}
	if c := o.call("POST", "/api/periods/"+o.period.ID.String()+"/close", o.admin, nil, nil); c != 200 {
		t.Fatalf("close = %d", c)
	}
	if c, code := o.errCode("PUT", "/api/periods/"+o.period.ID.String()+"/handover", o.admin, h); c != 409 || code != api.CodePeriodClosed {
		t.Fatalf("handover after close = %d %s", c, code)
	}
	if c, code := o.errCode("POST", o.scope(o.period.ID, "mediatek")+"/invitations", oldTok, api.InviteRequest{Email: "q@himasif.test"}); c != 409 || code != api.CodePeriodClosed {
		t.Fatalf("write in closed period = %d %s", c, code)
	}
	// The old kadiv reads the old spaces, read-only.
	var oldSpaces api.List[api.Space]
	o.call("GET", "/api/periods/"+o.period.ID.String()+"/spaces", oldTok, nil, &oldSpaces)
	var oldMediatek api.Space
	for _, s := range oldSpaces.Items {
		if !s.ReadOnly || slices.Contains(s.Capabilities, api.CapTasksManage) {
			t.Errorf("closed space writable %+v", s)
		}
		if s.Code == "division-mediatek" {
			oldMediatek = s
		}
	}
	if oldMediatek.ID == id.Nil {
		t.Fatal("old kadiv lost the old space")
	}
	// No automatic access to the new period.
	var none api.List[api.Space]
	o.call("GET", "/api/periods/"+p2.ID.String()+"/spaces", oldTok, nil, &none)
	if len(none.Items) != 0 {
		t.Fatalf("old kadiv sees new spaces %+v", none)
	}

	// The new period: a new kadiv, a new member.
	newK, _ := o.join(o.admin, p2.ID, "mediatek", "baru@himasif.test", "Kadiv Baru")
	_, nk, _ := o.login("baru@himasif.test", "kata-sandi-anggota")
	var nkUser api.User
	o.call("GET", "/api/me", newK, nil, &nkUser)
	o.call("PUT", o.scope(p2.ID, "mediatek")+"/kadiv", o.admin, api.SetKadivRequest{UserID: nkUser.ID}, nil)
	memTok, mem := o.join(nk.Token, p2.ID, "mediatek", "anggota@himasif.test", "Anggota")

	if c, _ := o.errCode("GET", "/api/spaces/"+oldMediatek.ID.String(), memTok, nil); c != 403 {
		t.Fatalf("archive without grant = %d", c)
	}
	var g api.ArchiveGrant
	if c := o.call("POST", o.scope(p2.ID, "mediatek")+"/archives", nk.Token, api.ArchiveRequest{UserID: mem.User.ID, SpaceID: oldMediatek.ID}, &g); c != 201 {
		t.Fatalf("grant = %d", c)
	}
	var sp api.Space
	if c := o.call("GET", "/api/spaces/"+oldMediatek.ID.String(), memTok, nil, &sp); c != 200 || sp.Access != api.AccessArchive || !sp.ReadOnly || slices.Contains(sp.Capabilities, api.CapMembersRead) {
		t.Fatalf("archive space = %d %+v", c, sp)
	}
	// One space only, and not the old directory.
	var visible api.List[api.Space]
	o.call("GET", "/api/periods/"+o.period.ID.String()+"/spaces", memTok, nil, &visible)
	if len(visible.Items) != 1 {
		t.Fatalf("grant opened %d spaces", len(visible.Items))
	}
	if c, _ := o.errCode("GET", o.scope(o.period.ID, "mediatek")+"/members", memTok, nil); c != 403 {
		t.Fatalf("archive opens directory = %d", c)
	}
	// Humas's kadiv may not grant a Mediatek space.
	if c, _ := o.errCode("POST", o.scope(p2.ID, "humas")+"/archives", o.admin, api.ArchiveRequest{UserID: mem.User.ID, SpaceID: oldMediatek.ID}); c != 403 && c != 400 {
		t.Fatalf("cross-division grant = %d", c)
	}
	if c := o.call("DELETE", o.scope(p2.ID, "mediatek")+"/archives/"+g.ID.String(), nk.Token, nil, nil); c != 204 {
		t.Fatalf("revoke grant = %d", c)
	}
	if c, _ := o.errCode("GET", "/api/spaces/"+oldMediatek.ID.String(), memTok, nil); c != 403 {
		t.Fatalf("after revoke = %d", c)
	}
	_ = old
}

func TestRecoveryAndAdminHandover(t *testing.T) {
	o := newOrg(t)
	tok, acc := o.join(o.admin, o.period.ID, "humas", "lupa@himasif.test", "Lupa")
	email := acc.User.Email

	// Same answer for unknown addresses.
	if c := o.call("POST", "/api/auth/recovery", "", api.RecoveryRequest{Email: "tidak-ada@himasif.test"}, nil); c != 202 {
		t.Fatalf("unknown = %d", c)
	}
	before := o.mail.count(email)
	if c := o.call("POST", "/api/auth/recovery", "", api.RecoveryRequest{Email: email}, nil); c != 202 {
		t.Fatalf("recovery = %d", c)
	}
	o.srv.Wait()
	if o.mail.count(email) != before+1 || o.mail.count("tidak-ada@himasif.test") != 0 {
		t.Fatal("recovery emails")
	}
	rt := param(t, o.mail.last(t, email).Text, "token")
	if c := o.call("POST", "/api/auth/recovery/reset", "", api.RecoveryResetRequest{Token: rt, Password: "kata-sandi-baru-sekali"}, nil); c != 204 {
		t.Fatalf("reset = %d", c)
	}
	if c, _ := o.errCode("GET", "/api/me", tok, nil); c != 401 {
		t.Fatalf("old session after reset = %d", c)
	}
	if c, _ := o.errCode("POST", "/api/auth/recovery/reset", "", api.RecoveryResetRequest{Token: rt, Password: "kata-sandi-lain-lagi"}); c != 410 {
		t.Fatalf("reuse = %d", c)
	}
	if c, _, _ := o.login(email, "kata-sandi-baru-sekali"); c != 200 {
		t.Fatalf("login after reset = %d", c)
	}

	// Admin handover: prove the new email, then rotate; every session ends.
	if c := o.call("POST", "/api/auth/admin-email", o.admin, api.AdminEmailRequest{CurrentPassword: adminPassword, Email: "admin-baru@himasif.test"}, nil); c != 202 {
		t.Fatalf("admin email = %d", c)
	}
	proof := param(t, o.mail.last(t, "admin-baru@himasif.test").Text, "verification")
	_, second, _ := o.login(adminEmail, adminPassword)
	if c := o.call("POST", "/api/auth/admin-handover", o.admin, api.AdminHandoverRequest{CurrentPassword: adminPassword,
		Password: "kata-sandi-admin-baru", Email: "admin-baru@himasif.test", VerificationToken: proof, Custodian: "Sekretaris 2026/2027"}, nil); c != 204 {
		t.Fatalf("handover = %d", c)
	}
	for _, s := range []string{o.admin, second.Token} {
		if c, _ := o.errCode("GET", "/api/me", s, nil); c != 401 {
			t.Errorf("admin session survives = %d", c)
		}
	}
	if c, _, _ := o.login("admin-baru@himasif.test", "kata-sandi-admin-baru"); c != 200 {
		t.Fatalf("new admin login = %d", c)
	}
	if o.audits("admin.handover") != 1 {
		t.Fatal("handover not audited")
	}
}

func TestDisableEndsSessions(t *testing.T) {
	o := newOrg(t)
	tok, acc := o.join(o.admin, o.period.ID, "humas", "nonaktif@himasif.test", "N")
	if c := o.call("POST", "/api/users/"+acc.User.ID.String()+"/disable", o.admin, nil, nil); c != 200 {
		t.Fatalf("disable = %d", c)
	}
	if c, _ := o.errCode("GET", "/api/me", tok, nil); c != 401 {
		t.Fatalf("session after disable = %d", c)
	}
	if c, _, _ := o.login("nonaktif@himasif.test", "kata-sandi-anggota"); c != 403 {
		t.Fatalf("login disabled = %d", c)
	}
	// Recovery does not enable it.
	o.call("POST", "/api/auth/recovery", "", api.RecoveryRequest{Email: "nonaktif@himasif.test"}, nil)
	o.srv.Wait()
	time.Sleep(10 * time.Millisecond)
	if n := o.mail.count("nonaktif@himasif.test"); n != 2 { // invitation + proof only
		t.Fatalf("emails to a disabled account %d", n)
	}
	if c, _ := o.errCode("POST", "/api/users/"+acc.User.ID.String()+"/disable", tok, nil); c != 401 && c != 403 {
		t.Fatalf("non-admin disable = %d", c)
	}
	_ = http.StatusOK
}
