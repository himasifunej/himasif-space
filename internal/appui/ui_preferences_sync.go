package appui

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
)

// UIPreferencesAPI keeps the Space tab bars with the account (Tahap 5c), so
// they follow it to every device. It is apart from the other services so an
// older server or test fake without it keeps the bars on the device.
type UIPreferencesAPI interface {
	UIPreferences(context.Context) (api.UIPreferences, error)
	SetUIPreferences(context.Context, api.UIPreferences) (api.UIPreferences, error)
}

// uiPreferencesSync is the account copy of the tab bars. The device copy
// (preferences) stays the one the tab bar reads: the account copy replaces
// it once per sign-in, and every change on the device is sent back.
type uiPreferencesSync struct {
	user    id.ID
	loading bool
	loaded  bool
	// off is set when the server has no such endpoint, or for the
	// organization admin; the bars then stay on the device only.
	off bool
	// pending is a change made before the account copy arrived; it is sent
	// once the copy is in, over it.
	pending bool
	err     string
}

func (a *App) uiPreferencesService() (UIPreferencesAPI, bool) {
	if c, ok := a.api.(*client.Client); ok {
		return c.AccountSession(), true
	}
	s, ok := a.api.(UIPreferencesAPI)
	return s, ok
}

// ensureUIPreferences fetches the account's tab bars once per account.
func (a *App) ensureUIPreferences() {
	st := &a.uiPrefs
	if a.user.ID == id.Nil {
		return
	}
	if st.user != a.user.ID {
		*st = uiPreferencesSync{user: a.user.ID}
	}
	if st.loaded || st.loading || st.off || st.err != "" {
		return
	}
	svc, ok := a.uiPreferencesService()
	if !ok {
		st.off = true
		return
	}
	gen, uid := a.gen, a.user.ID
	st.loading = true
	a.async(func() func() {
		prefs, err := svc.UIPreferences(context.Background())
		return func() {
			if gen != a.gen || a.uiPrefs.user != uid {
				return
			}
			st := &a.uiPrefs
			st.loading = false
			if uiPreferencesMissing(err) {
				st.off = true
				return
			}
			if err != nil {
				st.err = a.failed(err)
				return
			}
			st.loaded = true
			if st.pending || len(prefs.SpaceTabs) == 0 {
				// A change made meanwhile wins; an account with nothing
				// saved yet takes this device's bars (once).
				st.pending = false
				if len(a.deviceSpaceTabs(uid)) > 0 {
					a.pushUIPreferences()
				}
				return
			}
			a.applyAccountSpaceTabs(uid, prefs.SpaceTabs)
		}
	})
}

func uiPreferencesMissing(err error) bool {
	var ce *client.Error
	return errors.As(err, &ce) && (ce.Status == http.StatusNotFound || ce.Status == http.StatusMethodNotAllowed || ce.Status == http.StatusForbidden)
}

// deviceSpaceTabs returns this device's bars of an account, by Space ID.
func (a *App) deviceSpaceTabs(uid id.ID) map[string]api.SpaceTabPreference {
	out := map[string]api.SpaceTabPreference{}
	if a.chrome.preferences.SpaceTabs == nil {
		return out
	}
	prefix := uid.String() + "/"
	for k, v := range *a.chrome.preferences.SpaceTabs {
		if sid, ok := strings.CutPrefix(k, prefix); ok {
			out[sid] = api.SpaceTabPreference{Order: v.Order, Hidden: v.Hidden}
		}
	}
	return out
}

// applyAccountSpaceTabs makes the account's bars this device's, keeping
// other accounts' bars on the device as they are.
func (a *App) applyAccountSpaceTabs(uid id.ID, tabs map[string]api.SpaceTabPreference) {
	layouts := SpaceTabLayouts{}
	prefix := uid.String() + "/"
	if old := a.chrome.preferences.SpaceTabs; old != nil {
		for k, v := range *old {
			if !strings.HasPrefix(k, prefix) {
				layouts[k] = v
			}
		}
	}
	for sid, v := range tabs {
		layouts[prefix+sid] = SpaceTabPreference{Order: v.Order, Hidden: v.Hidden}
	}
	a.chrome.preferences.SpaceTabs = &layouts
	a.chrome.preferences = NormalizePreferences(a.chrome.preferences)
	if a.PreferencesStore != nil {
		if err := a.PreferencesStore.Save(a.chrome.preferences); err != nil {
			a.chrome.preferencesErr = "Urutan tab akunmu dipakai untuk sesi ini, tetapi belum tersimpan di perangkat."
		}
	}
}

// spaceTabsChanged sends the account's bars after a change on this device.
func (a *App) spaceTabsChanged() {
	st := &a.uiPrefs
	if st.off || a.user.ID == id.Nil || st.user != a.user.ID {
		return
	}
	if !st.loaded {
		st.pending = true
		return
	}
	a.pushUIPreferences()
}

func (a *App) pushUIPreferences() {
	svc, ok := a.uiPreferencesService()
	if !ok {
		return
	}
	gen, uid := a.gen, a.user.ID
	req := api.UIPreferences{SpaceTabs: a.deviceSpaceTabs(uid)}
	a.async(func() func() {
		_, err := svc.SetUIPreferences(context.Background(), req)
		return func() {
			if gen != a.gen || a.uiPrefs.user != uid {
				return
			}
			if uiPreferencesMissing(err) {
				a.uiPrefs.off = true
				return
			}
			if err != nil {
				// The bar stays saved on the device; the next change
				// sends the whole set again.
				a.uiPrefs.err = a.failed(err)
				return
			}
			a.uiPrefs.err = ""
		}
	})
}
