package update

import (
	"crypto/ed25519"
	"encoding/base64"
)

// FeedBase is where the manifests are published: assets of the GitHub
// release "update-feed" of the public repository, replaced by each
// release (scripts/release/publish_update.py). GitHub serves them over
// HTTPS from its CDN, needs no change to the pilot server, and keeps
// working while the pilot is down; trust comes from the signature, not
// from the host.
const FeedBase = "https://github.com/himasifunej/himasif-space/releases/download/update-feed/"

// FeedURL is the manifest of channel: stable.json or rc.json.
func FeedURL(channel string) string { return FeedBase + channel + ".json" }

// publicKeys are the base64 ed25519 public keys a manifest may be signed
// with. The private key is the vault's space-update-signing; to rotate,
// add the new key here, ship a release signed with the old one, then
// sign with the new one and drop the old.
var publicKeys = []string{
	"hppeUgxTmEIeTEG0H8Rf3rbOwlODwjfSDLCUutbcThM=", // 869a5e520c539842, 2026-11-11
}

// PublicKeys returns the keys the app trusts.
func PublicKeys() []ed25519.PublicKey {
	var out []ed25519.PublicKey
	for _, k := range publicKeys {
		b, err := base64.StdEncoding.DecodeString(k)
		if err == nil && len(b) == ed25519.PublicKeySize {
			out = append(out, ed25519.PublicKey(b))
		}
	}
	return out
}
