package server

import (
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
)

func TestOnboardingProgressFollowsAccount(t *testing.T) {
	o := newOrg(t)
	kTok, _ := o.kadiv("mediatek", "km@himasif.test")
	mTok, _ := o.join(kTok, o.period.ID, "mediatek", "m@himasif.test", "Mediatek")

	var got api.Onboarding
	if c := o.call("GET", "/api/me/onboarding?periodId="+o.period.ID.String(), mTok, nil, &got); c != 200 || len(got.Marks) != 0 || got.WhatsAppLinked {
		t.Fatalf("new account = %d %+v", c, got)
	}
	if c := o.call("POST", "/api/me/onboarding", mTok, api.OnboardingMark{Key: api.OnboardWelcome}, &got); c != 200 || !got.Done(api.OnboardWelcome) {
		t.Fatalf("mark = %d %+v", c, got)
	}
	first := got.Marks[api.OnboardWelcome]
	o.call("POST", "/api/me/onboarding", mTok, api.OnboardingMark{Key: api.OnboardWelcome}, &got)
	if !got.Marks[api.OnboardWelcome].Equal(first) {
		t.Fatal("a second mark moved the first time")
	}
	o.call("POST", "/api/me/onboarding", mTok, api.OnboardingMark{Key: api.OnboardTour}, &got)
	o.call("POST", "/api/me/onboarding", mTok, api.OnboardingMark{Key: api.OnboardTour, Clear: true}, &got)
	if got.Done(api.OnboardTour) || !got.Done(api.OnboardWelcome) {
		t.Fatalf("clear = %+v", got)
	}
	var other api.Onboarding
	if o.call("GET", "/api/me/onboarding", kTok, nil, &other); len(other.Marks) != 0 {
		t.Fatalf("another account sees %+v", other)
	}
	if c, _ := o.errCode("POST", "/api/me/onboarding", mTok, api.OnboardingMark{Key: "bebas"}); c != 400 {
		t.Fatalf("unknown key = %d", c)
	}
	if c, _ := o.errCode("GET", "/api/me/onboarding?periodId=bukan", mTok, nil); c != 400 {
		t.Fatalf("bad period = %d", c)
	}

	// The profile: the name is tidied, checked, audited, and the step
	// recorded.
	var u api.User
	if c := o.call("PUT", "/api/me/profile", mTok, api.ProfileUpdate{DisplayName: "  Nadia   Putri "}, &u); c != 200 || u.DisplayName != "Nadia Putri" {
		t.Fatalf("profile = %d %+v", c, u)
	}
	o.call("GET", "/api/me/onboarding", mTok, nil, &got)
	if !got.Done(api.OnboardProfile) || o.audits("account.profile.update") != 1 {
		t.Fatalf("profile step %+v", got)
	}
	if c, _ := o.errCode("PUT", "/api/me/profile", mTok, api.ProfileUpdate{DisplayName: "N"}); c != 400 {
		t.Fatalf("short name = %d", c)
	}

	var gl api.GuideList
	if c := o.call("GET", "/api/guides", mTok, nil, &gl); c != 200 || gl.Source != "bawaan" || len(gl.Guides) < 10 {
		t.Fatalf("guides = %d %s %d", c, gl.Source, len(gl.Guides))
	}
}
