package auth

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	h, err := HashPassword("kata-sandi-panjang")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("unexpected encoding %q", h)
	}
	if err := VerifyPassword(h, "kata-sandi-panjang"); err != nil {
		t.Fatalf("right password: %v", err)
	}
	if err := VerifyPassword(h, "kata-sandi-salah!"); !errors.Is(err, ErrMismatch) {
		t.Fatalf("wrong password: %v", err)
	}
	h2, _ := HashPassword("kata-sandi-panjang")
	if h == h2 {
		t.Fatal("two hashes of the same password share a salt")
	}
}

func TestVerifyRejectsMalformedHashes(t *testing.T) {
	for _, h := range []string{"", "plain", "$argon2i$v=19$m=1,t=1,p=1$AA$AA", "$argon2id$v=19$m=x$AA$AA"} {
		if err := VerifyPassword(h, "whatever-password"); err == nil {
			t.Errorf("%q verified", h)
		}
	}
}

func TestCheckPassword(t *testing.T) {
	if CheckPassword("pendek") == "" {
		t.Error("short password accepted")
	}
	if CheckPassword("          ") == "" {
		t.Error("blank password accepted")
	}
	if CheckPassword(strings.Repeat("a", MaxPasswordLength+1)) == "" {
		t.Error("overlong password accepted")
	}
	if msg := CheckPassword("cukup-panjang-123"); msg != "" {
		t.Errorf("good password rejected: %s", msg)
	}
}

func TestTokens(t *testing.T) {
	a, ha, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	b, _, _ := NewToken()
	if a == b {
		t.Fatal("tokens repeat")
	}
	if len(a) < 40 {
		t.Fatalf("token too short: %d", len(a))
	}
	if !bytes.Equal(ha, HashToken(a)) {
		t.Fatal("hash does not match the token")
	}
}
