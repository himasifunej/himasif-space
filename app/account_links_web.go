//go:build js

package main

import (
	"strings"
	"syscall/js"
)

var accountHashListener js.Func
var accountHashListening bool

// consumeAccountHash removes the complete query from an account-link fragment
// before handing the secret to Go. The proof never enters persistent storage.
func consumeAccountHash() (raw string) {
	defer func() {
		// A restricted browser can deny history access; do not crash or log its URL.
		if recover() != nil {
			raw = ""
		}
	}()
	loc := js.Global().Get("location")
	hash := loc.Get("hash").String()
	route := strings.SplitN(strings.TrimPrefix(hash, "#"), "?", 2)[0]
	switch route {
	case "/undangan", "/pemulihan", "/organisasi/serah-terima-admin":
		if !strings.Contains(hash, "?") {
			return ""
		}
		js.Global().Get("history").Call("replaceState", js.Null(), "", loc.Get("pathname").String()+loc.Get("search").String()+"#"+route)
		return hash
	}
	return ""
}

func startupAccountLink() string { return consumeAccountHash() }

func watchAccountLinks(handle func(string)) {
	if accountHashListening {
		js.Global().Call("removeEventListener", "hashchange", accountHashListener)
		accountHashListener.Release()
	}
	accountHashListener = js.FuncOf(func(js.Value, []js.Value) any {
		if raw := consumeAccountHash(); raw != "" {
			handle(raw)
		}
		return nil
	})
	accountHashListening = true
	js.Global().Call("addEventListener", "hashchange", accountHashListener)
}
