package server

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/update"
)

// Card links (0.4.6): WhatsApp messages about a task or a meeting carry
// <SPACE_PUBLIC_URL>/t/<task id> or /r/<meeting id>. The link holds no
// sign-in, since messages are forwarded in groups: the web app opens the
// card for whoever signs in, and says so politely to who may not see it.

// cardLink is the public link to a task's or a meeting's card, or "" for
// anything else.
func (s *Server) cardLink(kind string, cid id.ID) string {
	base := strings.TrimRight(s.cfg.PublicURL, "/")
	if base == "" || cid == id.Nil {
		return ""
	}
	switch kind {
	case "task":
		return base + "/t/" + cid.String()
	case "meeting":
		return base + "/r/" + cid.String()
	}
	return ""
}

// openCard answers /t/{id} and /r/{id}: the web app's page of the card,
// which signs in first when needed and then returns to it.
func (s *Server) openCard(page string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cid, err := id.Parse(r.PathValue("id"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Robots-Tag", "noindex")
		http.Redirect(w, r, "/#/"+page+"/"+cid.String(), http.StatusFound)
	}
}

// installerCache keeps the newest Windows installer the update feeds name
// for a while, so the web's Unduh does not read GitHub each time.
type installerCache struct {
	mu      sync.Mutex
	url     string
	fetched time.Time
}

const installerCacheFor = 10 * time.Minute

// downloadWindows answers /unduh/windows: the installer of the newest
// stable version, or of the release candidate while none is stable, from
// the signed update feeds.
func (s *Server) downloadWindows(w http.ResponseWriter, r *http.Request) {
	c := &s.installer
	c.mu.Lock()
	link, fresh := c.url, time.Since(c.fetched) < installerCacheFor
	c.mu.Unlock()
	if !fresh || link == "" {
		link = newestInstaller(r.Context())
		if link != "" {
			c.mu.Lock()
			c.url, c.fetched = link, time.Now()
			c.mu.Unlock()
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	if link == "" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("Installer Space untuk Windows belum dapat diambil. Coba lagi sebentar lagi."))
		return
	}
	http.Redirect(w, r, link, http.StatusFound)
}

// newestInstaller is the Windows installer of the stable feed, else of the
// rc feed: only from manifests whose signature checks.
func newestInstaller(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 15 * time.Second}
	for _, ch := range []string{update.ChannelStable, update.ChannelRC} {
		data, err := update.Fetch(ctx, client, update.FeedURL(ch), "server")
		if err != nil {
			continue
		}
		m, err := update.Open(data, update.PublicKeys(), ch)
		if err == nil && m.Windows != nil && strings.HasPrefix(m.Windows.URL, "https://") {
			return m.Windows.URL
		}
	}
	return ""
}
