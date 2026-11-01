//go:build !windows && !js

package main

import (
	"errors"
	"github.com/egoist/mygo"
	"github.com/himasifunej/himasif-space/internal/api"
)

func registerNotifications() error   { return nil }
func setDeviceAutostart(bool) error  { return errors.New("Autostart ini tersedia pada Windows.") }
func notificationSoundAllowed() bool { return true }
func showDeviceNotification(d api.Delivery, link string) error {
	n := mygo.NewNotification(mygo.NotificationOptions{ID: d.ID.String(), Title: d.Title, Body: d.Body, Silent: true, Group: "Space"})
	n.OnClick(func() { mygo.App.WhenReady(func() { _ = mygo.Shell.OpenExternal(link) }) })
	return n.Show()
}

func clearDeviceNotifications(string) { mygo.ClearNotifications() }
