// Package auth hashes passwords and makes session tokens.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
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

// ErrBusy is what the context variants return when no hashing slot frees
// up in time: too many logins at once. Callers answer 429.
var ErrBusy = errors.New("auth: password hashing is busy")

// Every Argon2id run takes argonMemory (19 MiB) for its duration, and the
// API unit is capped at 256 MB. hashGate bounds the runs in progress in the
// whole process, so a burst of logins queues instead of getting the process
// killed for its memory.
type hashGate struct {
	slots    chan struct{}
	maxQueue int64
	waiting  atomic.Int64
	inFlight atomic.Int64
	peak     atomic.Int64
}

// Defaults of SetHashLimits: two runs (38 MiB) at once, and 32 more waiting.
const (
	DefaultHashConcurrency = 2
	DefaultHashQueue       = 32
)

var (
	gateMu sync.Mutex
	gate   = newHashGate(DefaultHashConcurrency, DefaultHashQueue)
)

func newHashGate(concurrency, queue int) *hashGate {
	if concurrency < 1 {
		concurrency = 1
	}
	if queue < 0 {
		queue = 0
	}
	return &hashGate{slots: make(chan struct{}, concurrency), maxQueue: int64(queue)}
}

// SetHashLimits sets how many password hashes may run at once in this
// process, and how many callers of the context variants may wait for one.
// Call it at start, before serving.
func SetHashLimits(concurrency, queue int) {
	gateMu.Lock()
	defer gateMu.Unlock()
	gate = newHashGate(concurrency, queue)
}

func currentGate() *hashGate {
	gateMu.Lock()
	defer gateMu.Unlock()
	return gate
}

// HashStats reports the hashes running now and the most that ran at once
// since start (or the last SetHashLimits).
func HashStats() (inFlight, peak int) {
	g := currentGate()
	return int(g.inFlight.Load()), int(g.peak.Load())
}

// acquire takes a hashing slot. bounded callers give up with ErrBusy when
// the queue is full or ctx ends first; unbounded ones (seed, invitations,
// already limited per address) wait.
func (g *hashGate) acquire(ctx context.Context, bounded bool) (func(), error) {
	select {
	case g.slots <- struct{}{}:
	default:
		if n := g.waiting.Add(1); bounded && n > g.maxQueue {
			g.waiting.Add(-1)
			return nil, ErrBusy
		}
		select {
		case g.slots <- struct{}{}:
			g.waiting.Add(-1)
		case <-ctx.Done():
			g.waiting.Add(-1)
			return nil, ErrBusy
		}
	}
	n := g.inFlight.Add(1)
	for {
		p := g.peak.Load()
		if n <= p || g.peak.CompareAndSwap(p, n) {
			break
		}
	}
	return func() {
		g.inFlight.Add(-1)
		<-g.slots
	}, nil
}

func idKey(ctx context.Context, bounded bool, pw, salt []byte, passes, memory uint32, threads uint8, keyLen uint32) ([]byte, error) {
	release, err := currentGate().acquire(ctx, bounded)
	if err != nil {
		return nil, err
	}
	defer release()
	return argon2.IDKey(pw, salt, passes, memory, threads, keyLen), nil
}

// HashPassword returns the encoded Argon2id hash of pw with a new salt. It
// waits for a hashing slot as long as it takes.
func HashPassword(pw string) (string, error) {
	return hashPassword(context.Background(), false, pw)
}

// HashPasswordContext is HashPassword that gives up with ErrBusy when ctx
// ends or too many wait for a hashing slot.
func HashPasswordContext(ctx context.Context, pw string) (string, error) {
	return hashPassword(ctx, true, pw)
}

func hashPassword(ctx context.Context, bounded bool, pw string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := idKey(ctx, bounded, []byte(pw), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	if err != nil {
		return "", err
	}
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// VerifyPassword checks pw against an encoded hash, in constant time. It
// waits for a hashing slot as long as it takes.
func VerifyPassword(encoded, pw string) error {
	return verifyPassword(context.Background(), false, encoded, pw)
}

// VerifyPasswordContext is VerifyPassword that gives up with ErrBusy when
// ctx ends or too many wait for a hashing slot.
func VerifyPasswordContext(ctx context.Context, encoded, pw string) error {
	return verifyPassword(ctx, true, encoded, pw)
}

func verifyPassword(ctx context.Context, bounded bool, encoded, pw string) error {
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
	got, err := idKey(ctx, bounded, []byte(pw), salt, passes, memory, threads, uint32(len(want)))
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrMismatch
	}
	return nil
}

// dummyHash is verified against when an account does not exist, so that a
// login takes as long whether or not the email has an account.
// It is made at start, so a BurnTimeContext refused for a full gate never
// waits for a slot to make it.
var dummyHash = func() string {
	h, err := HashPassword("space-dummy-password")
	if err != nil {
		panic(err)
	}
	return h
}()

// BurnTime spends the time a password verification takes.
func BurnTime(pw string) { _ = VerifyPassword(dummyHash, pw) }

// BurnTimeContext is BurnTime that gives up with ErrBusy like
// VerifyPasswordContext, so a refused unknown email answers like a refused
// known one.
func BurnTimeContext(ctx context.Context, pw string) error {
	if err := VerifyPasswordContext(ctx, dummyHash, pw); errors.Is(err, ErrBusy) {
		return err
	}
	return nil
}

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
