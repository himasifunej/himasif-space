package client

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"net/http"
	"strings"
	"sync"
	"time"
)

// OfflineStorage stores opaque device records atomically. Credentials never
// enter it; namespaces are server/account IDs and bearer hashes are aliases.
var ErrOfflineMissing = errors.New("cache perangkat belum tersedia")

type OfflineStorage interface {
	Read(string) ([]byte, error)
	Write(string, []byte) error
	Delete(string) error
}
type PendingMutation struct {
	Mutation  api.OfflineMutation `json:"mutation"`
	CreatedAt time.Time           `json:"createdAt"`
	State     string              `json:"state"`
	Error     string              `json:"error,omitempty"`
	Remote    json.RawMessage     `json:"remote,omitempty"`
}
type offlineCache struct {
	Entries   map[string]json.RawMessage `json:"entries"`
	UpdatedAt time.Time                  `json:"updatedAt"`
}
type OfflineRepository struct {
	mu          sync.Mutex
	storage     OfflineStorage
	unavailable bool
	syncing     bool
	lastSync    map[string]time.Time
	lastLoaded  map[string]bool
	aliases     map[string]string
	epochs      map[string]uint64
	err         string
}

func NewOfflineRepository(storage OfflineStorage) *OfflineRepository {
	return &OfflineRepository{storage: storage, lastSync: map[string]time.Time{}, lastLoaded: map[string]bool{}, aliases: map[string]string{}, epochs: map[string]uint64{}}
}
func offlineHash(s string) string { v := sha256.Sum256([]byte(s)); return hex.EncodeToString(v[:]) }
func (c *Client) offlineNamespace() string {
	if c.Offline == nil || c.token() == "" {
		return ""
	}
	key := "alias-" + offlineHash(c.BaseURL+":"+c.token())
	c.Offline.mu.Lock()
	cached := c.Offline.aliases[key]
	c.Offline.mu.Unlock()
	if cached != "" {
		return cached
	}
	b, err := c.Offline.storage.Read(key)
	if err != nil {
		return ""
	}
	uid, err := id.Parse(string(b))
	if err != nil {
		return ""
	}
	ns := offlineHash(c.BaseURL) + "-" + uid.String()
	c.Offline.mu.Lock()
	c.Offline.aliases[key] = ns
	c.Offline.mu.Unlock()
	return ns
}
func (c *Client) offlineIdentify(user api.User) {
	if c.Offline == nil || c.token() == "" || user.ID == id.Nil {
		return
	}
	_ = c.Offline.storage.Write("alias-"+offlineHash(c.BaseURL+":"+c.token()), []byte(user.ID.String()))
	data, err := json.Marshal(user)
	if err == nil {
		c.cacheOffline("/api/me", data)
	}
}
func offlineReadable(path string) bool {
	if strings.Contains(path, "/lock") || strings.Contains(path, "/draft") || strings.Contains(path, "/history") || strings.Contains(path, "/invitations") || strings.Contains(path, "/users") {
		return false
	}
	if strings.Contains(path, "/whatsapp") || strings.Contains(path, "/wa-") || strings.Contains(path, "/sop") || strings.Contains(path, "/connectors") {
		return false
	}
	return path == "/api/me" || path == "/api/deliveries" || path == "/api/periods" || path == "/api/divisions" || strings.HasPrefix(path, "/api/spaces/") || strings.HasPrefix(path, "/api/tasks/") || strings.HasPrefix(path, "/api/meetings/") || strings.Contains(path, "/spaces") || strings.Contains(path, "/tasks") || strings.HasPrefix(path, "/api/notes/") || strings.HasPrefix(path, "/api/reminders")
}
func (c *Client) offlineEpoch() uint64 {
	ns := c.offlineNamespace()
	if c.Offline == nil || ns == "" {
		return 0
	}
	c.Offline.mu.Lock()
	defer c.Offline.mu.Unlock()
	return c.Offline.epochs[ns]
}
func (c *Client) cacheOffline(path string, data []byte) {
	c.cacheOfflineAt(path, data, c.offlineEpoch())
}
func (c *Client) cacheOfflineAt(path string, data []byte, epoch uint64) {
	ns := c.offlineNamespace()
	if ns == "" || !offlineReadable(path) || len(data) > 4<<20 {
		return
	}
	repo := c.Offline
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.epochs[ns] != epoch {
		return
	}
	cache := offlineCache{Entries: map[string]json.RawMessage{}}
	if b, err := repo.storage.Read("cache-" + ns); err == nil {
		_ = json.Unmarshal(b, &cache)
	}
	if cache.Entries == nil {
		cache.Entries = map[string]json.RawMessage{}
	}
	if previous, ok := cache.Entries[path]; ok && string(previous) != string(data) {
		if strings.HasSuffix(path, "/spaces") {
			for key := range cache.Entries {
				if key != "/api/me" && key != "/api/periods" && key != "/api/divisions" && !strings.HasSuffix(key, "/spaces") {
					delete(cache.Entries, key)
				}
			}
		}
		if strings.Contains(path, "/tasks") || strings.Contains(path, "/meetings") || strings.HasPrefix(path, "/api/reminders") {
			delete(cache.Entries, "/api/deliveries")
		}
	}
	cache.Entries[path] = append(json.RawMessage(nil), data...)
	cache.UpdatedAt = time.Now().UTC()
	b, err := json.Marshal(cache)
	if err == nil && len(b) <= 32<<20 {
		err = repo.storage.Write("cache-"+ns, b)
	} else if err == nil {
		err = errors.New("Cache perangkat mencapai batas 32 MiB.")
	}
	if err != nil {
		repo.err = err.Error()
	} else {
		repo.lastSync[ns] = cache.UpdatedAt
		repo.lastLoaded[ns] = true
		b, _ := json.Marshal(cache.UpdatedAt)
		_ = repo.storage.Write("sync-"+ns, b)
	}
}
func (c *Client) readOffline(method, path string, out any, cause error) error {
	if c.Offline == nil || method != http.MethodGet || !offlineReadable(path) {
		return &UnreachableError{Err: cause}
	}
	ns := c.offlineNamespace()
	if ns == "" {
		return &UnreachableError{Err: cause}
	}
	repo := c.Offline
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.unavailable = true
	b, err := repo.storage.Read("cache-" + ns)
	if err != nil {
		return &UnreachableError{Err: cause}
	}
	var cache offlineCache
	if json.Unmarshal(b, &cache) != nil {
		return &UnreachableError{Err: cause}
	}
	data, ok := cache.Entries[path]
	if !ok {
		return &UnreachableError{Err: cause}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}
func (c *Client) clearOfflineAccess() {
	if c.Offline == nil {
		return
	}
	ns := c.offlineNamespace()
	if ns == "" {
		return
	}
	// Draf/outbox remain owner-private; cached official data is evicted on
	// any explicit authorization denial rather than falling back to stale data.
	c.Offline.mu.Lock()
	defer c.Offline.mu.Unlock()
	if c.Offline.epochs == nil {
		c.Offline.epochs = map[string]uint64{}
	}
	c.Offline.epochs[ns]++
	_ = c.Offline.storage.Delete("cache-" + ns)
}
func (c *Client) OfflineStatus() (bool, bool, time.Time, string) {
	if c.Offline == nil {
		return false, false, time.Time{}, ""
	}
	ns := c.offlineNamespace()
	r := c.Offline
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.lastLoaded[ns] {
		r.lastLoaded[ns] = true
		if b, err := r.storage.Read("sync-" + ns); err == nil {
			var last time.Time
			if json.Unmarshal(b, &last) == nil {
				r.lastSync[ns] = last
			}
		}
	}
	return r.unavailable, r.syncing, r.lastSync[ns], r.err
}
func (c *Client) PendingOffline() ([]PendingMutation, error) {
	ns := c.offlineNamespace()
	if ns == "" {
		return nil, nil
	}
	r := c.Offline
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.readPending(ns)
}
func (r *OfflineRepository) readPending(ns string) ([]PendingMutation, error) {
	b, err := r.storage.Read("queue-" + ns)
	if errors.Is(err, ErrOfflineMissing) {
		return []PendingMutation{}, nil
	}
	if err != nil {
		return nil, err
	}
	var out []PendingMutation
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}
func (r *OfflineRepository) savePending(ns string, p []PendingMutation) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if len(b) > 16<<20 {
		return errors.New("Antrean offline mencapai batas 16 MiB.")
	}
	return r.storage.Write("queue-"+ns, b)
}
func (c *Client) QueueOffline(m api.OfflineMutation) error {
	ns := c.offlineNamespace()
	if ns == "" {
		return errors.New("Belum ada cache akun yang terkonfirmasi pada perangkat ini.")
	}
	r := c.Offline
	r.mu.Lock()
	defer r.mu.Unlock()
	pending, err := r.readPending(ns)
	if err != nil {
		return err
	}
	if len(pending) >= 200 {
		return errors.New("Antrean perangkat penuh; sinkronkan perubahan terlebih dahulu.")
	}
	if m.ID == id.Nil {
		m.ID = id.New()
	}
	// Coalescing edits of one target retains the first server baseline, while
	// a transmitted/conflicting immutable intent always gets a new identity.
	for i := range pending {
		old := &pending[i]
		if old.State == "local" && old.Mutation.Kind == m.Kind && old.Mutation.TargetID == m.TargetID {
			m.BaseProgress = old.Mutation.BaseProgress
			m.BaseTitle = old.Mutation.BaseTitle
			m.BaseContent = old.Mutation.BaseContent
			old.Mutation = m
			return r.savePending(ns, pending)
		}
	}
	pending = append(pending, PendingMutation{Mutation: m, CreatedAt: time.Now().UTC(), State: "local"})
	return r.savePending(ns, pending)
}
func (c *Client) SyncPending(ctx context.Context) error {
	ns := c.offlineNamespace()
	if ns == "" {
		return nil
	}
	r := c.Offline
	r.mu.Lock()
	if r.syncing {
		r.mu.Unlock()
		return nil
	}
	r.syncing = true
	r.mu.Unlock()
	defer func() { r.mu.Lock(); r.syncing = false; r.mu.Unlock() }()
	pending, err := c.PendingOffline()
	if err != nil {
		return err
	}
	blocked := map[string]bool{}
	for _, entry := range pending {
		targetKey := entry.Mutation.Kind + ":" + entry.Mutation.TargetID.String()
		if blocked[targetKey] {
			continue
		}
		if entry.State == "conflict" || entry.State == "rejected" {
			blocked[targetKey] = true
			continue
		}
		r.mu.Lock()
		all, e := r.readPending(ns)
		if e != nil {
			r.mu.Unlock()
			return e
		}
		for i := range all {
			if all[i].Mutation.ID == entry.Mutation.ID {
				all[i].State = "sending"
			}
		}
		e = r.savePending(ns, all)
		r.mu.Unlock()
		if e != nil {
			return e
		}
		result, sendErr := c.SyncOffline(ctx, entry.Mutation)
		r.mu.Lock()
		all, e = r.readPending(ns)
		if e != nil {
			r.mu.Unlock()
			return e
		}
		for i := range all {
			if all[i].Mutation.ID != entry.Mutation.ID {
				continue
			}
			if sendErr == nil {
				all = append(all[:i], all[i+1:]...)
			} else {
				var ae *Error
				if errors.As(sendErr, &ae) && (ae.Status == 409 || ae.Status == 403 || ae.Status == 404 || ae.Code == api.CodePeriodClosed) {
					all[i].State = "rejected"
					if ae.Status == 409 && ae.Code != api.CodePeriodClosed {
						all[i].State = "conflict"
					}
					all[i].Error = ae.Message
					all[i].Remote, _ = json.Marshal(ae.Data)
				} else {
					all[i].State = "sending"
					all[i].Error = sendErr.Error()
				}
			}
			break
		}
		e = r.savePending(ns, all)
		if sendErr == nil {
			r.lastSync[ns] = time.Now().UTC()
			r.lastLoaded[ns] = true
			b, _ := json.Marshal(r.lastSync[ns])
			_ = r.storage.Write("sync-"+ns, b)
			r.unavailable = false
		}
		r.mu.Unlock()
		if e != nil {
			return e
		}
		if sendErr != nil {
			blocked[targetKey] = true
			var ae *Error
			if !errors.As(sendErr, &ae) || ae.Status == 401 || ae.Status >= 500 {
				return sendErr
			}
			continue
		}
		// Store canonical responses only after the receipt is durable remotely.
		if result.Task != nil {
			b, _ := json.Marshal(result.Task)
			c.cacheOffline("/api/tasks/"+result.Task.ID.String(), b)
		}
		if result.Note != nil {
			b, _ := json.Marshal(result.Note)
			c.cacheOffline("/api/notes/"+result.Note.ID.String(), b)
		}
	}
	return nil
}
func (c *Client) ResolveOffline(mid id.ID, useLocal bool) error {
	ns := c.offlineNamespace()
	if ns == "" {
		return errors.New("Akun belum tersedia.")
	}
	r := c.Offline
	r.mu.Lock()
	defer r.mu.Unlock()
	all, err := r.readPending(ns)
	if err != nil {
		return err
	}
	for i := range all {
		p := &all[i]
		if p.Mutation.ID != mid {
			continue
		}
		if !useLocal {
			all = append(all[:i], all[i+1:]...)
			return r.savePending(ns, all)
		}
		if p.State != "conflict" {
			return errors.New("Perubahan ini ditolak server; draf tetap disimpan. Periksa izin/periode sebelum mencoba lagi.")
		}
		switch p.Mutation.Kind {
		case "task.progress":
			var t api.Task
			if json.Unmarshal(p.Remote, &t) != nil || t.ID == id.Nil {
				return errors.New("Versi server tidak tersedia.")
			}
			p.Mutation.BaseProgress = t.Progress
		case "note.content":
			var n api.Note
			if json.Unmarshal(p.Remote, &n) != nil || n.ID == id.Nil {
				return errors.New("Versi server tidak tersedia.")
			}
			p.Mutation.BaseTitle = n.Title
			p.Mutation.BaseContent = n.Content
		}
		p.Mutation.ID = id.New()
		p.State = "local"
		p.Error = ""
		p.Remote = nil
		return r.savePending(ns, all)
	}
	return fmt.Errorf("perubahan tidak ditemukan")
}

// RecoverPending can requeue a rejected intent only with a fresh identity;
// server permissions are still authoritative. It never edits the old receipt.
func (c *Client) RecoverPending(mid id.ID) error {
	ns := c.offlineNamespace()
	if ns == "" {
		return errors.New("Akun belum tersedia.")
	}
	r := c.Offline
	r.mu.Lock()
	defer r.mu.Unlock()
	all, err := r.readPending(ns)
	if err != nil {
		return err
	}
	for i := range all {
		if all[i].Mutation.ID == mid {
			if all[i].State != "rejected" {
				return errors.New("Gunakan penyelesaian konflik untuk perubahan ini.")
			}
			all[i].Mutation.ID = id.New()
			all[i].State = "local"
			all[i].Error = ""
			all[i].Remote = nil
			return r.savePending(ns, all)
		}
	}
	return errors.New("Perubahan tidak ditemukan.")
}

// DeviceRecord reads a small record this device keeps for the signed-in
// account beside its offline cache (the emergency sheet's links and what
// was read from it). name holds letters and dashes only.
func (c *Client) DeviceRecord(name string) ([]byte, error) {
	if c.Offline == nil {
		return nil, ErrOfflineMissing
	}
	ns := c.offlineNamespace()
	if ns == "" {
		return nil, ErrOfflineMissing
	}
	return c.Offline.storage.Read("rec-" + name + "-" + ns)
}

// SaveDeviceRecord keeps a record DeviceRecord reads.
func (c *Client) SaveDeviceRecord(name string, data []byte) error {
	if c.Offline == nil {
		return ErrOfflineMissing
	}
	ns := c.offlineNamespace()
	if ns == "" {
		return ErrOfflineMissing
	}
	return c.Offline.storage.Write("rec-"+name+"-"+ns, data)
}
