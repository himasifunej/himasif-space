package appui

import (
	"errors"
	"testing"
)

type routeNotifier struct {
	calls int
	err   error
}

func (n *routeNotifier) Permission() string   { return BrowserPermissionGranted }
func (n *routeNotifier) RequestPermission()   {}
func (n *routeNotifier) Present(Notice) error { n.calls++; return n.err }
func (n *routeNotifier) DeniedHint() string   { return "hint" }

func TestMultiNotifierReplacementFallback(t *testing.T) {
	for _, tc := range []struct {
		name          string
		replace, fail bool
		want          int
	}{{"replace", true, false, 0}, {"fallback", true, true, 1}, {"accompany", false, false, 1}, {"accompany failure", false, true, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			base, extra := &routeNotifier{}, &routeNotifier{}
			if tc.fail {
				extra.err = errors.New("unavailable")
			}
			m := MultiNotifier{Base: base, Extra: []NoticePresenter{extra}, ReplaceBase: tc.replace}
			if err := m.Present(Notice{}); err != nil {
				t.Fatal(err)
			}
			if base.calls != tc.want || extra.calls != 1 {
				t.Fatalf("base %d extra %d", base.calls, extra.calls)
			}
			if m.Permission() != BrowserPermissionGranted || m.DeniedHint() != "hint" {
				t.Fatal("base permission/hint lost")
			}
		})
	}
	base := &routeNotifier{err: errors.New("os failed")}
	if (MultiNotifier{Base: base, ReplaceBase: true}).Present(Notice{}) == nil {
		t.Fatal("OS error lost")
	}
}
