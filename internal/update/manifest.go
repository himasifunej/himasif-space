// Package update is what the desktop app needs to update itself: the
// signed manifest of a version (which installer, its SHA-256 and size),
// the comparison of versions, and a download checked against the
// manifest. It has no UI and no platform code; app/ checks, asks and
// installs (docs/operations/auto-update.md).
//
// A manifest is published as an envelope: the manifest's JSON as bytes,
// and an ed25519 signature over a context string and those bytes. The app
// holds the public keys (PublicKeys) and accepts only a manifest that one
// of them signed, of the channel it asked for, and of a version newer
// than its own: an old manifest, replayed, offers nothing.
package update

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// Channels: Stabil, the default, and Uji coba (release candidates).
const (
	ChannelStable = "stable"
	ChannelRC     = "rc"
)

// App names Space in its manifests, so that a manifest of another app
// signed with the same key is refused.
const App = "space"

// signedContext goes before the manifest's bytes in what is signed, so
// that a signature made for something else is not a manifest's.
const signedContext = "space-update-manifest-v1\n"

// Limits of a manifest and of what it names.
const (
	MaxEnvelopeSize = 64 << 10
	MaxInstaller    = 512 << 20
	MaxNotes        = 600
)

// Manifest is a version of Space on a channel.
type Manifest struct {
	App     string `json:"app"`
	Channel string `json:"channel"`
	Version string `json:"version"`
	// Released is when it was published (RFC 3339).
	Released string `json:"released"`
	// Notes is a short summary, NotesURL the full release notes.
	Notes    string `json:"notes"`
	NotesURL string `json:"notesUrl,omitempty"`
	// Windows is the NSIS installer for Windows x64.
	Windows *Installer `json:"windows,omitempty"`
}

// Installer is a file to download: an https URL, its SHA-256 (hex) and
// size in bytes.
type Installer struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// Envelope is a manifest as published: its JSON, and the signature.
type Envelope struct {
	Format    int    `json:"format"`
	KeyID     string `json:"keyId"`
	Manifest  string `json:"manifest"`  // base64 of the manifest's JSON
	Signature string `json:"signature"` // base64 of the ed25519 signature
}

// KeyID names a public key: the first 8 bytes of it, in hex.
func KeyID(pub ed25519.PublicKey) string { return hex.EncodeToString(pub[:8]) }

// Sign makes the envelope of m with the private key.
func Sign(m Manifest, priv ed25519.PrivateKey) ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	sig := ed25519.Sign(priv, append([]byte(signedContext), body...))
	return json.MarshalIndent(Envelope{
		Format:    1,
		KeyID:     KeyID(priv.Public().(ed25519.PublicKey)),
		Manifest:  base64.StdEncoding.EncodeToString(body),
		Signature: base64.StdEncoding.EncodeToString(sig),
	}, "", "  ")
}

// Errors of Open.
var (
	ErrSignature = errors.New("update: manifest signature is not valid")
	ErrChannel   = errors.New("update: manifest is of another channel")
)

// Open checks the envelope data against keys and returns its manifest,
// which must be of channel.
func Open(data []byte, keys []ed25519.PublicKey, channel string) (Manifest, error) {
	var m Manifest
	if len(data) > MaxEnvelopeSize {
		return m, errors.New("update: manifest too large")
	}
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return m, fmt.Errorf("update: manifest: %w", err)
	}
	if env.Format != 1 {
		return m, fmt.Errorf("update: unknown manifest format %d", env.Format)
	}
	body, err1 := base64.StdEncoding.DecodeString(env.Manifest)
	sig, err2 := base64.StdEncoding.DecodeString(env.Signature)
	if err1 != nil || err2 != nil || len(sig) != ed25519.SignatureSize {
		return m, ErrSignature
	}
	signed := append([]byte(signedContext), body...)
	ok := false
	for _, k := range keys {
		if len(k) == ed25519.PublicKeySize && ed25519.Verify(k, signed, sig) {
			ok = true
			break
		}
	}
	if !ok {
		return m, ErrSignature
	}
	// Only now is the body trusted enough to read.
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("update: manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return Manifest{}, err
	}
	if m.Channel != channel {
		return Manifest{}, ErrChannel
	}
	return m, nil
}

// Validate checks what a manifest must hold.
func (m Manifest) Validate() error {
	switch {
	case m.App != App:
		return fmt.Errorf("update: manifest of app %q", m.App)
	case m.Channel != ChannelStable && m.Channel != ChannelRC:
		return fmt.Errorf("update: unknown channel %q", m.Channel)
	case !ValidVersion(m.Version):
		return fmt.Errorf("update: invalid version %q", m.Version)
	case utf8.RuneCountInString(m.Notes) > MaxNotes:
		return errors.New("update: notes too long")
	}
	if _, err := time.Parse(time.RFC3339, m.Released); err != nil {
		return fmt.Errorf("update: invalid release time %q", m.Released)
	}
	if m.NotesURL != "" && !isHTTPS(m.NotesURL) {
		return errors.New("update: notes URL must be https")
	}
	if m.Windows == nil {
		return errors.New("update: manifest has no installer")
	}
	return m.Windows.Validate()
}

// Validate checks an installer's URL, hash and size.
func (in Installer) Validate() error {
	if !isHTTPS(in.URL) {
		return errors.New("update: installer URL must be https")
	}
	if b, err := hex.DecodeString(in.SHA256); err != nil || len(b) != 32 {
		return errors.New("update: installer SHA-256 must be 64 hex digits")
	}
	if in.Size <= 0 || in.Size > MaxInstaller {
		return fmt.Errorf("update: invalid installer size %d", in.Size)
	}
	return nil
}

func isHTTPS(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil
}

// Offer reports whether m is an update for the app at version current: a
// newer version, never the same or an older one.
func Offer(current string, m Manifest) bool {
	return ValidVersion(current) && Compare(m.Version, current) > 0
}

// Newest returns the newest of the manifests that Offer accepts for
// current, and whether there is one. A Uji coba user reads both channels:
// a stable release newer than their release candidate is offered too.
func Newest(current string, ms ...Manifest) (Manifest, bool) {
	var best Manifest
	found := false
	for _, m := range ms {
		if Offer(current, m) && (!found || Compare(m.Version, best.Version) > 0) {
			best, found = m, true
		}
	}
	return best, found
}
