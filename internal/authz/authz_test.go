package authz

import (
	"slices"
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
)

func has(caps []string, c string) bool { return slices.Contains(caps, c) }

func TestBuiltInRoles(t *testing.T) {
	member := Binding{Role: api.RoleMember}.Capabilities()
	if has(member, api.CapTasksManage) || has(member, api.CapPingSend) || !has(member, api.CapTasksOwn) || !has(member, api.CapSpaceRead) {
		t.Errorf("member %v", member)
	}
	companion := Binding{Role: api.RoleCompanion}.Capabilities()
	if !has(companion, api.CapTasksManage) || !has(companion, api.CapResultsReview) || !has(companion, api.CapPingSend) {
		t.Errorf("companion lacks operations: %v", companion)
	}
	if has(companion, api.CapWhatsAppManage) || has(companion, api.CapInvitationsManage) {
		t.Errorf("companion manages WhatsApp or access by default: %v", companion)
	}
	kadiv := Binding{Role: api.RoleKadiv}.Capabilities()
	for _, c := range []string{api.CapWhatsAppManage, api.CapInvitationsManage, api.CapRolesManage, api.CapArchivesManage, api.Cap45Manage} {
		if !has(kadiv, c) {
			t.Errorf("kadiv lacks %s", c)
		}
	}
	if has(kadiv, api.CapPeriodsManage) || has(kadiv, api.CapKadivManage) {
		t.Errorf("kadiv manages periods or kadiv: %v", kadiv)
	}
}

func Test45NeverAdministers(t *testing.T) {
	// Even a custom role carrying administration (which the server refuses
	// to save) and a 45 appointment with stray capabilities add up to
	// operations only.
	b := Binding{Role: api.RoleCompanion, Acting45: true,
		CustomCaps: [][]string{{api.CapRolesManage, api.CapInvitationsManage, api.CapWhatsAppManage}},
		Caps45:     []string{api.CapTasksManage, api.CapArchivesManage, api.Cap45Manage}}
	caps := b.Capabilities()
	for _, c := range Administrative {
		if has(caps, c) {
			t.Errorf("45 holds %s: %v", c, caps)
		}
	}
	if !has(caps, api.CapWhatsAppManage) || !has(caps, api.CapTasksManage) {
		t.Errorf("45 lacks operations: %v", caps)
	}
	if r := b.Roles(); !slices.Equal(r, []string{api.RoleCompanion, api.Role45}) {
		t.Errorf("roles %v", r)
	}
	// Narrowed: only what the kadiv chose.
	narrow := Binding{Role: api.RoleMember, Acting45: true, Caps45: []string{api.CapPingSend}}.Capabilities()
	if !has(narrow, api.CapPingSend) || has(narrow, api.CapWhatsAppManage) || has(narrow, api.CapTasksManage) {
		t.Errorf("narrowed 45 %v", narrow)
	}
}

func TestCustomRolesAddOnlyOperations(t *testing.T) {
	caps := Binding{Role: api.RoleMember, CustomCaps: [][]string{{api.CapPingSend, api.CapMembersManage}}}.Capabilities()
	if !has(caps, api.CapPingSend) || has(caps, api.CapMembersManage) {
		t.Errorf("custom %v", caps)
	}
	for _, c := range Administrative {
		if ValidCustom(c) {
			t.Errorf("custom role may hold %s", c)
		}
	}
	if !Valid45(api.CapWhatsAppManage) || Valid45(api.CapTasksOwn) || Valid45(api.CapRolesManage) {
		t.Error("Valid45")
	}
}

func TestAdminAndReadOnly(t *testing.T) {
	if has(AdminCaps, api.Cap45Manage) || has(AdminCaps, api.CapTasksManage) || has(AdminCaps, api.CapNotesRead) {
		t.Errorf("admin %v", AdminCaps)
	}
	ro := ReadOnly(Binding{Role: api.RoleKadiv}.Capabilities())
	if !slices.Equal(ro, []string{api.CapMembersRead, api.CapNotesRead, api.CapSpaceRead}) {
		t.Errorf("read-only %v", ro)
	}
	if IsWrite(api.CapSpaceRead) || !IsWrite(api.CapInvitationsManage) {
		t.Error("IsWrite")
	}
	if n := Normalize(nil); n == nil || len(n) != 0 {
		t.Error("Normalize(nil)")
	}
}
