package update

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testKey(t *testing.T, seed byte) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	priv := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
	return priv.Public().(ed25519.PublicKey), priv
}

func testManifest(channel, version string) Manifest {
	return Manifest{App: App, Channel: channel, Version: version, Released: "2026-11-11T05:00:00Z", Notes: "Perbaikan.",
		Windows: &Installer{URL: "https://github.com/himasifunej/himasif-space/releases/download/v" + version + "/space-setup.exe", SHA256: strings.Repeat("ab", 32), Size: 1234}}
}

func TestSignedManifestOpens(t *testing.T) {
	pub, priv := testKey(t, 1)
	env, err := Sign(testManifest(ChannelRC, "0.4.6-rc"), priv)
	if err != nil {
		t.Fatal(err)
	}
	m, err := Open(env, []ed25519.PublicKey{pub}, ChannelRC)
	if err != nil || m.Version != "0.4.6-rc" || m.Windows.Size != 1234 {
		t.Fatalf("open: %+v %v", m, err)
	}
	// A key rotated in later: the old one still listed opens it too.
	other, _ := testKey(t, 9)
	if _, err := Open(env, []ed25519.PublicKey{other, pub}, ChannelRC); err != nil {
		t.Fatal(err)
	}
}

func TestManifestSignatureRefused(t *testing.T) {
	pub, priv := testKey(t, 1)
	_, stranger := testKey(t, 2)
	m := testManifest(ChannelStable, "0.4.6")
	good, _ := Sign(m, priv)

	// Signed by another key.
	bad, _ := Sign(m, stranger)
	if _, err := Open(bad, []ed25519.PublicKey{pub}, ChannelStable); !errors.Is(err, ErrSignature) {
		t.Fatalf("other key: %v", err)
	}
	// The manifest changed after signing: another installer.
	var env Envelope
	_ = json.Unmarshal(good, &env)
	body, _ := base64.StdEncoding.DecodeString(env.Manifest)
	body = bytes.Replace(body, []byte("github.com"), []byte("example.com"), 1)
	env.Manifest = base64.StdEncoding.EncodeToString(body)
	tampered, _ := json.Marshal(env)
	if _, err := Open(tampered, []ed25519.PublicKey{pub}, ChannelStable); !errors.Is(err, ErrSignature) {
		t.Fatalf("tampered: %v", err)
	}
	// No keys, no trust.
	if _, err := Open(good, nil, ChannelStable); !errors.Is(err, ErrSignature) {
		t.Fatalf("no keys: %v", err)
	}
	// A signature over the bare manifest, without the context string.
	raw, _ := json.Marshal(m)
	env.Manifest = base64.StdEncoding.EncodeToString(raw)
	env.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, raw))
	noContext, _ := json.Marshal(env)
	if _, err := Open(noContext, []ed25519.PublicKey{pub}, ChannelStable); !errors.Is(err, ErrSignature) {
		t.Fatalf("no context: %v", err)
	}
	// Garbage.
	if _, err := Open([]byte("{"), []ed25519.PublicKey{pub}, ChannelStable); err == nil {
		t.Fatal("garbage opened")
	}
}

// A signed rc manifest served as the stable feed is refused: the channel
// is signed with it.
func TestManifestChannelIsSigned(t *testing.T) {
	pub, priv := testKey(t, 1)
	env, _ := Sign(testManifest(ChannelRC, "0.4.6-rc"), priv)
	if _, err := Open(env, []ed25519.PublicKey{pub}, ChannelStable); !errors.Is(err, ErrChannel) {
		t.Fatalf("rc as stable: %v", err)
	}
}

func TestManifestValidation(t *testing.T) {
	_, priv := testKey(t, 1)
	for name, change := range map[string]func(*Manifest){
		"http":        func(m *Manifest) { m.Windows.URL = "http://example.com/a.exe" },
		"userinfo":    func(m *Manifest) { m.Windows.URL = "https://u:p@example.com/a.exe" },
		"short hash":  func(m *Manifest) { m.Windows.SHA256 = "abcd" },
		"no size":     func(m *Manifest) { m.Windows.Size = 0 },
		"huge":        func(m *Manifest) { m.Windows.Size = MaxInstaller + 1 },
		"version":     func(m *Manifest) { m.Version = "terbaru" },
		"app":         func(m *Manifest) { m.App = "lain" },
		"channel":     func(m *Manifest) { m.Channel = "beta" },
		"no windows":  func(m *Manifest) { m.Windows = nil },
		"notes":       func(m *Manifest) { m.Notes = strings.Repeat("x", MaxNotes+1) },
		"released":    func(m *Manifest) { m.Released = "kemarin" },
		"notes http":  func(m *Manifest) { m.NotesURL = "http://example.com" },
		"empty build": func(m *Manifest) { m.Version = "0.4.6-" },
	} {
		m := testManifest(ChannelStable, "0.4.6")
		change(&m)
		if _, err := Sign(m, priv); err == nil {
			t.Errorf("%s: signed an invalid manifest", name)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	ordered := []string{"0.3.0-pilot.2", "0.3.0-pilot.11", "0.3.0", "0.4.4", "0.4.5-rc", "0.4.5-rc.1", "0.4.5-rc.2", "0.4.5-rc.10", "0.4.5", "0.4.10", "0.10.0", "1.0.0"}
	for i := range ordered {
		for j := range ordered {
			want := cmpInt(i, j)
			if got := Compare(ordered[i], ordered[j]); got != want {
				t.Errorf("Compare(%s, %s) = %d, want %d", ordered[i], ordered[j], got, want)
			}
		}
	}
	if Compare("v0.4.5", "0.4.5") != 0 || Compare("0.4.5+build.7", "0.4.5") != 0 {
		t.Error("prefix or build metadata changed the order")
	}
	for _, bad := range []string{"", "0.4", "0.4.5.1", "01.4.5", "0.4.x", "0.4.5-", "0.4.5-rc..1", "0.4.5-rc!"} {
		if ValidVersion(bad) {
			t.Errorf("%q accepted", bad)
		}
		if Compare(bad, "0.0.1") >= 0 {
			t.Errorf("invalid %q not older than any version", bad)
		}
	}
}

// Only a newer version is offered: the same one, an older one (a
// replayed manifest) and a manifest for an unknown running version never.
func TestOfferRefusesDowngrade(t *testing.T) {
	for _, c := range []struct {
		current, offered string
		want             bool
	}{
		{"0.4.5", "0.4.6", true},
		{"0.4.5-rc", "0.4.5", true},
		{"0.4.5-rc", "0.4.5-rc.2", true},
		{"0.4.5", "0.4.5", false},
		{"0.4.5", "0.4.5-rc", false},
		{"0.4.6", "0.4.5", false},
		{"0.4.10", "0.4.9", false},
		{"dev", "0.4.6", false},
	} {
		if got := Offer(c.current, testManifest(ChannelStable, c.offered)); got != c.want {
			t.Errorf("Offer(%s, %s) = %v", c.current, c.offered, got)
		}
	}
	// Uji coba reads both channels: the newest that is newer wins.
	m, ok := Newest("0.4.5-rc", testManifest(ChannelRC, "0.4.6-rc"), testManifest(ChannelStable, "0.4.5"))
	if !ok || m.Version != "0.4.6-rc" {
		t.Fatalf("newest: %v %v", m.Version, ok)
	}
	m, ok = Newest("0.4.6-rc", testManifest(ChannelRC, "0.4.6-rc"), testManifest(ChannelStable, "0.4.6"))
	if !ok || m.Version != "0.4.6" {
		t.Fatalf("stable after rc: %v %v", m.Version, ok)
	}
	if _, ok := Newest("0.4.7", testManifest(ChannelRC, "0.4.6-rc"), testManifest(ChannelStable, "0.4.6")); ok {
		t.Fatal("offered a downgrade")
	}
}

func TestDownloadChecksSHA256(t *testing.T) {
	payload := bytes.Repeat([]byte("space"), 5000)
	sum := sha256.Sum256(payload)
	var agent, cookie, query string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent, cookie, query = r.UserAgent(), r.Header.Get("Cookie"), r.URL.RawQuery
		if r.URL.Path == "/bad" {
			w.Write(bytes.ToUpper(payload))
			return
		}
		w.Write(payload)
	}))
	defer srv.Close()
	dir := t.TempDir()
	in := Installer{URL: srv.URL + "/ok", SHA256: hex.EncodeToString(sum[:]), Size: int64(len(payload))}
	var last int64
	path := filepath.Join(dir, "setup.exe")
	if err := Download(context.Background(), srv.Client(), in, path, "0.4.5", func(done, total int64) { last = done }); err != nil {
		t.Fatal(err)
	}
	if last != in.Size || VerifyFile(path, in) != nil {
		t.Fatalf("progress %d, verify %v", last, VerifyFile(path, in))
	}
	if agent != "Space-Updater/0.4.5" || cookie != "" || query != "" {
		t.Fatalf("request told more than the version: %q %q %q", agent, cookie, query)
	}
	// Changed on disk after the download: refused before it runs.
	os.WriteFile(path, bytes.ToUpper(payload), 0o644)
	if err := VerifyFile(path, in); !errors.Is(err, ErrChecksum) {
		t.Fatalf("changed file: %v", err)
	}
	// A server sending another file: nothing is left behind.
	bad := filepath.Join(dir, "bad.exe")
	in.URL = srv.URL + "/bad"
	if err := Download(context.Background(), srv.Client(), in, bad, "0.4.5", nil); !errors.Is(err, ErrChecksum) {
		t.Fatalf("bad download: %v", err)
	}
	if _, err := os.Stat(bad); err == nil {
		t.Fatal("bad file kept")
	}
	if _, err := os.Stat(bad + ".part"); err == nil {
		t.Fatal("partial file kept")
	}
	// Longer than the manifest says.
	in.URL, in.Size = srv.URL+"/ok", in.Size-10
	if err := Download(context.Background(), srv.Client(), in, bad, "0.4.5", nil); err == nil {
		t.Fatal("size not checked")
	}
}

// The app trusts at least one well-formed key, and reads its manifests
// from GitHub over https.
func TestBuiltInKeysAndFeed(t *testing.T) {
	if len(PublicKeys()) == 0 || len(PublicKeys()) != len(publicKeys) {
		t.Fatalf("keys %d of %d parse", len(PublicKeys()), len(publicKeys))
	}
	for _, ch := range []string{ChannelStable, ChannelRC} {
		if !isHTTPS(FeedURL(ch)) || !strings.HasSuffix(FeedURL(ch), "/"+ch+".json") {
			t.Errorf("feed %s", FeedURL(ch))
		}
	}
}

// A feed that answers 404 has nothing published: ErrNoFeed, which callers
// tell from a failed check; other statuses stay errors.
func TestFetchNoFeed(t *testing.T) {
	status := http.StatusNotFound
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
	defer srv.Close()
	_, err := Fetch(context.Background(), srv.Client(), srv.URL+"/stable.json", "0.4.5-rc2")
	if !errors.Is(err, ErrNoFeed) {
		t.Fatalf("404: %v", err)
	}
	status = http.StatusBadGateway
	_, err = Fetch(context.Background(), srv.Client(), srv.URL+"/stable.json", "0.4.5-rc2")
	if err == nil || errors.Is(err, ErrNoFeed) {
		t.Fatalf("502: %v", err)
	}
}
