//go:build js

package main

import (
	"github.com/himasifunej/himasif-space/internal/client"
	"syscall/js"
)

type webOffline struct{}

func configureOffline(c *client.Client) { c.Offline = client.NewOfflineRepository(webOffline{}) }
func (webOffline) Read(key string) (data []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = jsError(r)
		}
	}()
	v := js.Global().Get("localStorage").Call("getItem", "space.offline."+key)
	if v.IsNull() {
		return nil, client.ErrOfflineMissing
	}
	return []byte(v.String()), nil
}
func (webOffline) Write(key string, data []byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = jsError(r)
		}
	}()
	js.Global().Get("localStorage").Call("setItem", "space.offline."+key, string(data))
	return nil
}
func (webOffline) Delete(key string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = jsError(r)
		}
	}()
	js.Global().Get("localStorage").Call("removeItem", "space.offline."+key)
	return nil
}
