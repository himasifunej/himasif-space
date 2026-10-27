// Package auth hashes passwords and makes session tokens.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters: OWASP's recommended minimum (19 MiB, two passes, one
// lane), which keeps a login affordable on the small server Space runs on.
// They are written into each hash, so raising them later still verifies
// the hashes made before.
const (
	argonMemory  = 19 * 1024
	argonTime    = 2
	argonThreads = 1
	argonKeyLen  = 32
	saltLen      = 16
)

// MinPasswordLength and MaxPasswordLength bound passwords people choose.
// The maximum keeps hashing a single request cheap.
const (
	MinPasswordLength = 10
	MaxPasswordLength = 256
)

// ErrMismatch is what Verify returns for a wrong password.
var ErrMismatch = errors.New("auth: password does not match")

// CheckPassword returns a message for a password that may not be used, in
// Indonesian for the person choosing it, or "" when it may.
func CheckPassword(pw string) string {
	n := utf8.RuneCountInString(pw)
	switch {
	case n < MinPasswordLength:
		return fmt.Sprintf("Kata sandi minimal %d karakter.", MinPasswordLength)
	case len(pw) > MaxPasswordLength:
		return fmt.Sprintf("Kata sandi maksimal %d karakter.", MaxPasswordLength)
	case strings.TrimSpace(pw) == "":
		return "Kata sandi tidak boleh hanya spasi."
	}
	return ""
}

// HashPassword returns the encoded Argon2id hash of pw with a new salt.
func HashPassword(pw string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(pw), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// VerifyPassword checks pw against an encoded hash, in constant time.
func VerifyPassword(encoded, pw string) error {
	if len(pw) > MaxPasswordLength {
		return ErrMismatch
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return errors.New("auth: unknown password hash format")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return errors.New("auth: unsupported argon2 version")
	}
	var memory, passes uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &passes, &threads); err != nil {
		return fmt.Errorf("auth: bad argon2 parameters: %w", err)
	}
	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return fmt.Errorf("auth: bad salt: %w", err)
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil {
		return fmt.Errorf("auth: bad key: %w", err)
	}
	got := argon2.IDKey([]byte(pw), salt, passes, memory, threads, uint32(len(want)))
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrMismatch
	}
	return nil
}

// dummyHash is verified against when an account does not exist, so that a
// login takes as long whether or not the email has an account.
var dummyHash = func() string {
	h, err := HashPassword("space-dummy-password")
	if err != nil {
		panic(err)
	}
	return h
}()

// BurnTime spends the time a password verification takes.
func BurnTime(pw string) { _ = VerifyPassword(dummyHash, pw) }

// NewToken returns a new session token and the hash the server keeps of it.
// The server stores only the hash, so a leaked database does not give
// working tokens.
func NewToken() (token string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token), nil
}

// HashToken returns the hash the server keeps of a token.
func HashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}
