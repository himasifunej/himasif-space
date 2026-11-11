//go:build js

package main

import (
	"strings"
	"syscall/js"
	"time"

	"github.com/himasifunej/himasif-space/internal/appui"
)

// webPlatform is appui.WebPlatform in the browser.
type webPlatform struct{}

var _ appui.WebPlatform = webPlatform{}

// WindowsDesktop: a Windows computer, not a phone or a tablet (Windows
// phones are long gone; touch laptops are computers).
func (webPlatform) WindowsDesktop() bool {
	nav := js.Global().Get("navigator")
	ua := strings.ToLower(nav.Get("userAgent").String())
	if strings.Contains(ua, "mobile") || strings.Contains(ua, "android") || strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad") {
		return false
	}
	if data := nav.Get("userAgentData"); data.Truthy() {
		if data.Get("mobile").Truthy() {
			return false
		}
		if p := data.Get("platform"); p.Truthy() {
			return strings.EqualFold(p.String(), "Windows")
		}
	}
	return strings.Contains(ua, "windows nt")
}

func localStorage() (js.Value, bool) {
	var s js.Value
	ok := true
	func() {
		defer func() {
			if recover() != nil {
				ok = false
			}
		}()
		s = js.Global().Get("localStorage")
	}()
	return s, ok && s.Truthy()
}

func (webPlatform) Get(key string) string {
	s, ok := localStorage()
	if !ok {
		return ""
	}
	v := s.Call("getItem", key)
	if v.IsNull() || v.IsUndefined() {
		return ""
	}
	return v.String()
}

func (webPlatform) Set(key, value string) {
	if s, ok := localStorage(); ok {
		func() {
			defer func() { _ = recover() }() // full or blocked storage
			s.Call("setItem", key, value)
		}()
	}
}

// OpenApp opens link through a hidden frame, so the page stays when no app
// answers, and reports whether the browser lost the focus or the page hid
// within wait: the sign that the app opened. Browsers that ask first count
// once the person agrees within wait.
func (webPlatform) OpenApp(link string, wait time.Duration) <-chan bool {
	out := make(chan bool, 1)
	win, doc := js.Global(), js.Global().Get("document")
	left := false
	var onBlur, onHide js.Func
	onBlur = js.FuncOf(func(js.Value, []js.Value) any { left = true; return nil })
	onHide = js.FuncOf(func(js.Value, []js.Value) any {
		if doc.Get("visibilityState").String() == "hidden" {
			left = true
		}
		return nil
	})
	win.Call("addEventListener", "blur", onBlur)
	doc.Call("addEventListener", "visibilitychange", onHide)
	frame := doc.Call("createElement", "iframe")
	frame.Get("style").Set("display", "none")
	frame.Set("src", link)
	doc.Get("body").Call("appendChild", frame)
	var done js.Func
	done = js.FuncOf(func(js.Value, []js.Value) any {
		win.Call("removeEventListener", "blur", onBlur)
		doc.Call("removeEventListener", "visibilitychange", onHide)
		frame.Call("remove")
		onBlur.Release()
		onHide.Release()
		done.Release()
		out <- left
		return nil
	})
	win.Call("setTimeout", done, wait.Milliseconds())
	return out
}

// Go opens path of this origin in the tab: a download leaves the page as
// it is.
func (webPlatform) Go(path string) {
	js.Global().Get("location").Call("assign", path)
}
