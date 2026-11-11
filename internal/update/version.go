package update

import (
	"strconv"
	"strings"
)

// version is MAJOR.MINOR.PATCH with an optional pre-release after "-",
// as semantic versioning: "0.4.5", "0.4.5-rc", "0.4.5-rc.2",
// "0.3.0-pilot.11". Build metadata after "+" is ignored.
type version struct {
	core [3]int
	pre  []string
}

func parseVersion(s string) (version, bool) {
	var v version
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	core, pre, hasPre := strings.Cut(s, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, ok := number(p)
		if !ok {
			return v, false
		}
		v.core[i] = n
	}
	if hasPre {
		if pre == "" {
			return v, false
		}
		v.pre = strings.Split(pre, ".")
		for _, id := range v.pre {
			if id == "" || strings.Trim(id, "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-") != "" {
				return v, false
			}
		}
	}
	return v, true
}

// number parses a numeric identifier: digits, no leading zero.
func number(s string) (int, bool) {
	if s == "" || len(s) > 9 || (len(s) > 1 && s[0] == '0') {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	return n, err == nil && n >= 0
}

// ValidVersion reports whether s is a version Compare understands.
func ValidVersion(s string) bool {
	_, ok := parseVersion(s)
	return ok
}

// Compare returns -1, 0 or 1 as a is older than, the same as or newer
// than b. A pre-release is older than its release (0.4.5-rc < 0.4.5);
// pre-releases compare by their dot-separated identifiers, numbers as
// numbers and below words. An invalid version is older than any valid
// one.
func Compare(a, b string) int {
	va, oka := parseVersion(a)
	vb, okb := parseVersion(b)
	switch {
	case !oka && !okb:
		return 0
	case !oka:
		return -1
	case !okb:
		return 1
	}
	for i := range va.core {
		if c := cmpInt(va.core[i], vb.core[i]); c != 0 {
			return c
		}
	}
	switch {
	case len(va.pre) == 0 && len(vb.pre) == 0:
		return 0
	case len(va.pre) == 0:
		return 1
	case len(vb.pre) == 0:
		return -1
	}
	for i := 0; i < len(va.pre) && i < len(vb.pre); i++ {
		x, y := va.pre[i], vb.pre[i]
		nx, xNum := number(x)
		ny, yNum := number(y)
		var c int
		switch {
		case xNum && yNum:
			c = cmpInt(nx, ny)
		case xNum:
			c = -1
		case yNum:
			c = 1
		default:
			c = strings.Compare(x, y)
		}
		if c != 0 {
			return c
		}
	}
	return cmpInt(len(va.pre), len(vb.pre))
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
