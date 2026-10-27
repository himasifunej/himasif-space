//go:build js

package main

import (
	"errors"
	"strings"
	"syscall/js"
)

// errNoStorage is the error of a browser without sessionStorage.
var errNoStorage = errors.New("sessionStorage tidak tersedia")

// clientName names this app to the server at login.
const clientName = "web"

// tokenKey is the key of the session token in sessionStorage.
const tokenKey = "space.session-token"

// apiURL returns the URL of the API server: the page's origin, as the
// server serves the app.
func apiURL() string {
	return strings.TrimRight(js.Global().Get("location").Get("origin").String(), "/")
}

// sessionTokens keeps the token in the tab's sessionStorage, which the
// browser forgets as the tab closes.
type sessionTokens struct{}

func newTokenStore() sessionTokens { return sessionTokens{} }

// storage returns window.sessionStorage, or undefined where the browser
// denies it, as with storage turned off.
func storage() (s js.Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			s, err = js.Undefined(), jsError(r)
		}
	}()
	return js.Global().Get("sessionStorage"), nil
}

// jsError turns what a JavaScript exception panicked with into an error.
func jsError(r any) error {
	if err, ok := r.(error); ok {
		return err
	}
	return errNoStorage
}

// call calls a method of sessionStorage, catching the exceptions it
// throws, as when the storage is full.
func call(method string, args ...any) (v js.Value, err error) {
	s, err := storage()
	if err != nil {
		return js.Undefined(), err
	}
	if s.IsUndefined() || s.IsNull() {
		return js.Undefined(), errNoStorage
	}
	defer func() {
		if r := recover(); r != nil {
			err = jsError(r)
		}
	}()
	return s.Call(method, args...), nil
}

func (sessionTokens) Load() (string, error) {
	v, err := call("getItem", tokenKey)
	if err != nil || v.IsNull() || v.IsUndefined() {
		return "", err
	}
	return v.String(), nil
}

func (sessionTokens) Save(token string) error {
	_, err := call("setItem", tokenKey, token)
	return err
}

func (sessionTokens) Clear() error {
	_, err := call("removeItem", tokenKey)
	return err
}
