package server

import (
	"slices"
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Tahap 5c: the Space tab bar follows the account, not the device, and one
// account never reads another's.
func TestUIPreferencesFollowAccount(t *testing.T) {
	o := newOrg(t)
	kTok, _ := o.kadiv("mediatek", "km@himasif.test")
	mTok, _ := o.join(kTok, o.period.ID, "mediatek", "m@himasif.test", "Mediatek")

	var got api.UIPreferences
	if c := o.call("GET", "/api/me/ui-preferences", kTok, nil, &got); c != 200 || got.SpaceTabs == nil || len(got.SpaceTabs) != 0 || got.UpdatedAt != nil {
		t.Fatalf("empty preferences = %d %+v", c, got)
	}
	sid := id.New().String()
	want := api.UIPreferences{SpaceTabs: map[string]api.SpaceTabPreference{sid: {Order: []string{"board", "beranda", "agenda"}, Hidden: []string{"rapat"}}}}
	if c := o.call("PUT", "/api/me/ui-preferences", kTok, want, &got); c != 200 || got.UpdatedAt == nil {
		t.Fatalf("put = %d %+v", c, got)
	}
	got = api.UIPreferences{}
	if c := o.call("GET", "/api/me/ui-preferences", kTok, nil, &got); c != 200 || !slices.Equal(got.SpaceTabs[sid].Order, want.SpaceTabs[sid].Order) || !slices.Equal(got.SpaceTabs[sid].Hidden, []string{"rapat"}) {
		t.Fatalf("get after put = %d %+v", c, got)
	}
	var other api.UIPreferences
	if c := o.call("GET", "/api/me/ui-preferences", mTok, nil, &other); c != 200 || len(other.SpaceTabs) != 0 {
		t.Fatalf("another account sees %+v", other)
	}
	if o.audits("ui.preferences.set") != 1 {
		t.Fatal("no audit of the change")
	}

	for name, bad := range map[string]api.UIPreferences{
		"space id":  {SpaceTabs: map[string]api.SpaceTabPreference{"bukan-uuid": {}}},
		"tab key":   {SpaceTabs: map[string]api.SpaceTabPreference{sid: {Order: []string{"Board Tugas"}}}},
		"duplicate": {SpaceTabs: map[string]api.SpaceTabPreference{sid: {Hidden: []string{"ping", "ping"}}}},
	} {
		if c, _ := o.errCode("PUT", "/api/me/ui-preferences", kTok, bad); c != 400 {
			t.Errorf("%s accepted: %d", name, c)
		}
	}
	got = api.UIPreferences{}
	o.call("GET", "/api/me/ui-preferences", kTok, nil, &got)
	if !slices.Equal(got.SpaceTabs[sid].Order, want.SpaceTabs[sid].Order) {
		t.Fatalf("a refused put changed the preferences: %+v", got)
	}
}
