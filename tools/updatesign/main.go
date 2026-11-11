// Command updatesign makes and checks the signed update manifests of the
// desktop app (internal/update, docs/operations/auto-update.md).
//
//	go run ./tools/updatesign keygen -out FILE
//	    writes a new private key (base64 seed) to FILE, mode 0600, and
//	    prints only its public key, for internal/update.PublicKeys.
//	go run ./tools/updatesign sign -channel rc -version 0.4.6-rc \
//	    -url https://... -file space-setup.exe -notes "…" [-notes-url …] -out rc.json
//	    reads the private key from SPACE_UPDATE_SIGNING_KEY (from the
//	    vault: enowx creds use space-update-signing -- …) or -key-file.
//	go run ./tools/updatesign verify -channel rc -in rc.json [-file space-setup.exe]
//	    checks the envelope against the keys built into the app, and the
//	    file against its SHA-256 and size.
//	go run ./tools/updatesign compare A B
//	    prints -1, 0 or 1 as version A is older, the same or newer.
//
// The private key is never printed.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/update"
)

func main() {
	if len(os.Args) < 2 {
		fail(errors.New("usage: updatesign keygen|sign|verify …"))
	}
	var err error
	switch os.Args[1] {
	case "keygen":
		err = keygen(os.Args[2:])
	case "sign":
		err = sign(os.Args[2:])
	case "verify":
		err = verify(os.Args[2:])
	case "compare":
		if len(os.Args) != 4 {
			err = errors.New("usage: updatesign compare A B")
			break
		}
		fmt.Println(update.Compare(os.Args[2], os.Args[3]))
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "updatesign:", err)
	os.Exit(1)
}

func keygen(args []string) error {
	fs := flag.NewFlagSet("keygen", flag.ExitOnError)
	out := fs.String("out", "", "file for the private key (must not exist)")
	fs.Parse(args)
	if *out == "" {
		return errors.New("-out is required")
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(*out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(base64.StdEncoding.EncodeToString(priv.Seed())); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	fmt.Printf("public key: %s\nkey id: %s\n", base64.StdEncoding.EncodeToString(pub), update.KeyID(pub))
	return nil
}

// privateKey reads the base64 seed from SPACE_UPDATE_SIGNING_KEY or file.
func privateKey(file string) (ed25519.PrivateKey, error) {
	raw := os.Getenv("SPACE_UPDATE_SIGNING_KEY")
	if file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		raw = string(data)
	}
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, errors.New("no valid signing key: set SPACE_UPDATE_SIGNING_KEY or -key-file")
	}
	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)
	for _, k := range update.PublicKeys() {
		if k.Equal(pub) {
			return priv, nil
		}
	}
	return nil, fmt.Errorf("signing key %s is not one the app trusts (internal/update.PublicKeys)", update.KeyID(pub))
}

func sign(args []string) error {
	fs := flag.NewFlagSet("sign", flag.ExitOnError)
	channel := fs.String("channel", "", "stable or rc")
	version := fs.String("version", "", "version, as the app reports it")
	url := fs.String("url", "", "https URL of the installer")
	file := fs.String("file", "", "the installer, to hash")
	notes := fs.String("notes", "", "short release notes")
	notesURL := fs.String("notes-url", "", "https URL of the full notes")
	keyFile := fs.String("key-file", "", "private key file instead of SPACE_UPDATE_SIGNING_KEY")
	out := fs.String("out", "", "envelope to write")
	fs.Parse(args)
	if *out == "" || *file == "" {
		return errors.New("-file and -out are required")
	}
	priv, err := privateKey(*keyFile)
	if err != nil {
		return err
	}
	sum, size, err := hashFile(*file)
	if err != nil {
		return err
	}
	m := update.Manifest{App: update.App, Channel: *channel, Version: *version, Released: time.Now().UTC().Format(time.RFC3339),
		Notes: *notes, NotesURL: *notesURL, Windows: &update.Installer{URL: *url, SHA256: sum, Size: size}}
	env, err := update.Sign(m, priv)
	if err != nil {
		return err
	}
	if err := os.WriteFile(*out, append(env, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("signed %s %s (key %s): %s, %d bytes, sha256 %s\n", m.Channel, m.Version, update.KeyID(priv.Public().(ed25519.PublicKey)), *out, size, sum)
	return nil
}

func verify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	channel := fs.String("channel", "", "stable or rc")
	in := fs.String("in", "", "envelope")
	file := fs.String("file", "", "installer to check (optional)")
	fs.Parse(args)
	data, err := os.ReadFile(*in)
	if err != nil {
		return err
	}
	m, err := update.Open(data, update.PublicKeys(), *channel)
	if err != nil {
		return err
	}
	if *file != "" {
		if err := update.VerifyFile(*file, *m.Windows); err != nil {
			return err
		}
	}
	fmt.Printf("ok: %s %s, %s (%d bytes)\n", m.Channel, m.Version, m.Windows.URL, m.Windows.Size)
	return nil
}

func hashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), n, err
}
