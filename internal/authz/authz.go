// Package authz computes what a role may do. It is pure: the server loads
// memberships, custom roles and the 45 appointment, and asks here which
// capabilities they add up to. The rules follow PRD §4.2 and ACC-06–11.
package authz

import (
	"slices"

	"github.com/himasifunej/himasif-space/internal/api"
)

// Operational are the capabilities of work in a division.
var Operational = []string{
	api.CapSpaceRead, api.CapMembersRead, api.CapTasksOwn, api.CapCommentsWrite, api.CapHelpRequest,
	api.CapNotesRead, api.CapTasksManage, api.CapResultsReview, api.CapPingSend, api.CapWhatsAppManage,
}

// Administrative are the capabilities of managing roles and access.
var Administrative = []string{
	api.CapInvitationsManage, api.CapMembersManage, api.CapRolesManage, api.CapArchivesManage,
	api.Cap45Manage, api.CapPeriodsManage, api.CapKadivManage,
}

// reading are the capabilities still effective in a closed period.
var reading = []string{api.CapSpaceRead, api.CapMembersRead, api.CapNotesRead}

var memberCaps = []string{api.CapSpaceRead, api.CapMembersRead, api.CapTasksOwn, api.CapCommentsWrite, api.CapHelpRequest, api.CapNotesRead}

// Default45 are the capabilities a 45 gets beyond a member's when the
// kadiv narrows nothing: the operational ones of a kadiv.
var Default45 = []string{api.CapTasksManage, api.CapResultsReview, api.CapPingSend, api.CapWhatsAppManage}

// AdminCaps are the organization admin's capabilities in any division: the
// administration of it, no work and no private data, and not the 45, which
// only the original kadiv appoints.
var AdminCaps = []string{
	api.CapSpaceRead, api.CapMembersRead,
	api.CapInvitationsManage, api.CapMembersManage, api.CapRolesManage, api.CapArchivesManage,
	api.CapPeriodsManage, api.CapKadivManage,
}

// ArchiveCaps are what an archive grant opens of a space.
var ArchiveCaps = []string{api.CapSpaceRead, api.CapNotesRead}

// RoleCaps returns the capabilities of a built-in membership role.
func RoleCaps(role string) []string {
	switch role {
	case api.RoleMember:
		return slices.Clone(memberCaps)
	case api.RoleCompanion:
		// Delegation, deadlines, review and ping; not WhatsApp by default.
		return append(slices.Clone(memberCaps), api.CapTasksManage, api.CapResultsReview, api.CapPingSend)
	case api.RoleKadiv:
		return append(slices.Clone(Operational),
			api.CapInvitationsManage, api.CapMembersManage, api.CapRolesManage, api.CapArchivesManage, api.Cap45Manage)
	}
	return nil
}

// Binding is one membership as the rules see it.
type Binding struct {
	Role       string     // member, companion or kadiv
	CustomCaps [][]string // the capabilities of each custom role held
	Acting45   bool       // the member is the division's 45
	Caps45     []string   // the capabilities of that appointment
}

// Roles returns the built-in roles of b, its own first.
func (b Binding) Roles() []string {
	r := []string{b.Role}
	if b.Acting45 {
		r = append(r, api.Role45)
	}
	return r
}

// Capabilities returns what b may do, sorted. Custom roles add only
// operational capabilities, and the ban of a 45 on administration is
// applied after every role is combined, so no combination lifts it.
func (b Binding) Capabilities() []string {
	caps := RoleCaps(b.Role)
	for _, c := range b.CustomCaps {
		caps = append(caps, onlyOf(c, Operational)...)
	}
	if b.Acting45 {
		caps = append(caps, onlyOf(b.Caps45, Default45)...)
		caps = slices.DeleteFunc(caps, func(c string) bool { return slices.Contains(Administrative, c) })
	}
	return Normalize(caps)
}

// ReadOnly keeps only the capabilities that read.
func ReadOnly(caps []string) []string { return onlyOf(caps, reading) }

// IsWrite reports whether a capability changes something.
func IsWrite(c string) bool { return !slices.Contains(reading, c) }

// ValidCustom reports whether a custom role may hold c.
func ValidCustom(c string) bool { return slices.Contains(Operational, c) }

// Valid45 reports whether a 45 appointment may hold c.
func Valid45(c string) bool { return slices.Contains(Default45, c) }

// Normalize sorts caps and removes duplicates, into a non-nil slice.
func Normalize(caps []string) []string {
	out := slices.Clone(caps)
	slices.Sort(out)
	out = slices.Compact(out)
	if out == nil {
		out = []string{}
	}
	return out
}

// Union returns the capabilities of all the sets, sorted.
func Union(sets ...[]string) []string {
	var all []string
	for _, s := range sets {
		all = append(all, s...)
	}
	return Normalize(all)
}

func onlyOf(caps, allowed []string) []string {
	var out []string
	for _, c := range caps {
		if slices.Contains(allowed, c) {
			out = append(out, c)
		}
	}
	return out
}
