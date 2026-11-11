package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// ErrNoFeed is a channel with no manifest published yet (HTTP 404): there
// is nothing to install from it, which is not a failed check.
var ErrNoFeed = errors.New("update: nothing published on this channel")

// UserAgent is all an update check tells about the device: the app and
// its version. No account, device ID, cookie or query string is sent.
func UserAgent(version string) string { return "Space-Updater/" + version }

// Fetch reads the envelope at url, bounded by MaxEnvelopeSize.
func Fetch(ctx context.Context, client *http.Client, url, version string) ([]byte, error) {
	body, err := get(ctx, client, url, version)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	data, err := io.ReadAll(io.LimitReader(body, MaxEnvelopeSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxEnvelopeSize {
		return nil, errors.New("update: manifest too large")
	}
	return data, nil
}

func get(ctx context.Context, client *http.Client, url, version string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent(version))
	req.Header.Set("Cache-Control", "no-cache")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode == http.StatusNotFound {
		res.Body.Close()
		return nil, fmt.Errorf("%w: %s", ErrNoFeed, url)
	}
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, fmt.Errorf("update: %s: HTTP %d", url, res.StatusCode)
	}
	return res.Body, nil
}

// Download saves the installer in to path, calling progress with the
// bytes so far, and checks its size and SHA-256: a file that does not
// match is removed and never left at path.
func Download(ctx context.Context, client *http.Client, in Installer, path, version string, progress func(done, total int64)) error {
	if err := in.Validate(); err != nil {
		return err
	}
	body, err := get(ctx, client, in.URL, version)
	if err != nil {
		return err
	}
	defer body.Close()
	part := path + ".part"
	f, err := os.Create(part)
	if err != nil {
		return err
	}
	sum := sha256.New()
	w := &progressWriter{total: in.Size, fn: progress}
	n, err := io.Copy(io.MultiWriter(f, sum, w), io.LimitReader(body, in.Size+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && n != in.Size {
		err = fmt.Errorf("update: downloaded %d bytes, want %d", n, in.Size)
	}
	if err == nil && hex.EncodeToString(sum.Sum(nil)) != in.SHA256 {
		err = ErrChecksum
	}
	if err != nil {
		os.Remove(part)
		return err
	}
	return os.Rename(part, path)
}

// ErrChecksum is a file whose SHA-256 is not the manifest's.
var ErrChecksum = errors.New("update: installer SHA-256 does not match the manifest")

// VerifyFile checks the file at path against in, as just before it runs:
// the file may have changed since it was downloaded.
func VerifyFile(path string, in Installer) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sum := sha256.New()
	n, err := io.Copy(sum, io.LimitReader(f, in.Size+1))
	if err != nil {
		return err
	}
	if n != in.Size || hex.EncodeToString(sum.Sum(nil)) != in.SHA256 {
		return ErrChecksum
	}
	return nil
}

type progressWriter struct {
	done, total int64
	fn          func(done, total int64)
	last        time.Time
}

func (w *progressWriter) Write(p []byte) (int, error) {
	w.done += int64(len(p))
	if w.fn != nil && (time.Since(w.last) > 100*time.Millisecond || w.done >= w.total) {
		w.last = time.Now()
		w.fn(w.done, w.total)
	}
	return len(p), nil
}
