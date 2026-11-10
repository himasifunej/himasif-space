package api

import "time"

// Onboarding marks: the steps of a new account's first days that the
// server records once, by key. The checklist's other steps are derived
// (WhatsAppLinked).
const (
	// OnboardWelcome is set once the welcome after joining was seen.
	OnboardWelcome = "welcome"
	// OnboardProfile is set when the account saved its profile.
	OnboardProfile = "profile"
	// OnboardTasks is set the first time the account opened a task board.
	OnboardTasks = "tasks"
	// OnboardTour is set when the tour was finished or skipped.
	OnboardTour = "tour"
	// OnboardChecklistHidden hides "Langkah pertama" from Beranda.
	OnboardChecklistHidden = "checklistHidden"
	// OnboardHandover is set when a kadiv finished the handover guide.
	OnboardHandover = "handover"
	// OnboardWAHello is set by the server once it queued the test message
	// to a member's newly linked WhatsApp; clients cannot set it.
	OnboardWAHello = "waHello"
)

// OnboardingKeys are the marks a client may set or clear.
var OnboardingKeys = []string{OnboardWelcome, OnboardProfile, OnboardTasks, OnboardTour, OnboardChecklistHidden, OnboardHandover}

// Onboarding is the caller's onboarding progress: the marks recorded, by
// key, and what the server derives. Accounts that existed before
// onboarding (migration 0030) start with welcome, tour and the checklist
// already done.
type Onboarding struct {
	Marks map[string]time.Time `json:"marks"`
	// WhatsAppLinked tells whether the caller linked a WhatsApp number in
	// the period asked for, or in any period when none was.
	WhatsAppLinked bool       `json:"whatsappLinked"`
	UpdatedAt      *time.Time `json:"updatedAt,omitempty"`
}

// Done reports whether mark key is set.
func (o Onboarding) Done(key string) bool {
	_, ok := o.Marks[key]
	return ok
}

// OnboardingMark sets, or with Clear removes, one mark.
type OnboardingMark struct {
	Key   string `json:"key"`
	Clear bool   `json:"clear,omitempty"`
}

// ProfileUpdate changes the caller's own profile: for now the name others
// see on boards, notes and pings.
type ProfileUpdate struct {
	DisplayName string `json:"displayName"`
}

// Guide roles, as guides list them.
const (
	GuideRoleMember = "anggota"
	GuideRole45     = "45"
	GuideRoleKadiv  = "kadiv"
)

// Guide is one article of the in-app guide (Panduan): Markdown with
// metadata. Pages names the pages whose "?" opens it: beranda, board,
// catatan, ping, anggota, whatsapp.
type Guide struct {
	Slug    string   `json:"slug"`
	Title   string   `json:"title"`
	Topic   string   `json:"topic"`
	Summary string   `json:"summary,omitempty"`
	Roles   []string `json:"roles"`
	Pages   []string `json:"pages,omitempty"`
	Minutes int      `json:"minutes"`
	Order   int      `json:"order"`
	Body    string   `json:"body"`
}

// ForRole reports whether the guide is for role (a Guide role).
func (g Guide) ForRole(role string) bool {
	for _, r := range g.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// GuideList is the guide as the server has it now.
type GuideList struct {
	Guides []Guide `json:"guides"`
	// Source is "bawaan" for the guides built into the server, "berkas"
	// when they come from SPACE_GUIDES_DIR.
	Source string `json:"source"`
}
