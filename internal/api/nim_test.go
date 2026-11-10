package api

import "testing"

func TestNIMHelpers(t *testing.T) {
	for _, s := range []string{"232410101090", "12345678", "12345678901234567890"} {
		if !ValidNIM(s) {
			t.Errorf("ValidNIM(%q) = false", s)
		}
	}
	for _, s := range []string{"", "1234567", "2324101010a0", "123456789012345678901", " 232410101090"} {
		if ValidNIM(s) {
			t.Errorf("ValidNIM(%q) = true", s)
		}
	}
	if StudentEmail("232410101090") != "232410101090@mail.unej.ac.id" {
		t.Fatal(StudentEmail("232410101090"))
	}
	for email, want := range map[string]string{
		"232410101090@mail.unej.ac.id":  "232410101090",
		" 232410101090@MAIL.unej.ac.id": "232410101090",
		"232410101090@unej.ac.id":       "",
		"aruna@mail.unej.ac.id":         "",
		"232410101090@gmail.com":        "",
	} {
		if got := StudentNIM(email); got != want {
			t.Errorf("StudentNIM(%q) = %q, want %q", email, got, want)
		}
	}
	if got := MaskNIM("232410101090"); got != "2324•••••090" {
		t.Errorf("MaskNIM = %q", got)
	}
	for in, want := range map[string]string{"6281234564471": "+62 812-••••-4471", "6281234567": "+62 812-•-4567", "15551234567": "+1 555-•••-4567", "abc": ""} {
		if got := MaskPhone(in); got != want {
			t.Errorf("MaskPhone(%q) = %q, want %q", in, got, want)
		}
	}
}
