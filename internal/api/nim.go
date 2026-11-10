package api

import (
	"strings"
	"unicode/utf8"
)

// NIM lengths accepted for a student address. UNEJ NIMs are 12 digits
// (232410101090); the range leaves room for older or longer formats and
// matches the pattern of migration 0028.
const (
	nimMinLen = 8
	nimMaxLen = 20
)

// ValidNIM reports whether s is a NIM: digits only, 8 to 20 of them.
func ValidNIM(s string) bool {
	if len(s) < nimMinLen || len(s) > nimMaxLen {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// StudentEmail is the student address of a NIM.
func StudentEmail(nim string) string { return nim + "@" + StudentEmailDomain }

// StudentNIM is the NIM of a student address, "" for any other email.
func StudentNIM(email string) string {
	local, domain, ok := strings.Cut(strings.ToLower(strings.TrimSpace(email)), "@")
	if !ok || domain != StudentEmailDomain || !ValidNIM(local) {
		return ""
	}
	return local
}

// IsStudentEmail reports whether email is a student address.
func IsStudentEmail(email string) bool { return StudentNIM(email) != "" }

// MaskNIM keeps the first four and the last three digits:
// 232410101090 → 2324•••••090. Used where a NIM confirms an identity to
// someone who may not be its owner, such as the WhatsApp bot's reply.
func MaskNIM(nim string) string {
	n := utf8.RuneCountInString(nim)
	if n <= 7 {
		return nim
	}
	return nim[:4] + strings.Repeat("•", n-7) + nim[n-3:]
}

// MaskPhone shows a WhatsApp number with its middle hidden:
// 6281234564471 → +62 812-••••-4471. "" when s is not a number.
func MaskPhone(digits string) string {
	if len(digits) < 8 {
		return ""
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return ""
		}
	}
	cc, rest := digits[:2], digits[2:]
	if !strings.HasPrefix(digits, "62") {
		cc, rest = digits[:1], digits[1:]
	}
	if len(rest) <= 7 {
		return "+" + cc + " " + strings.Repeat("•", len(rest)-3) + rest[len(rest)-3:]
	}
	return "+" + cc + " " + rest[:3] + "-" + strings.Repeat("•", len(rest)-7) + "-" + rest[len(rest)-4:]
}
