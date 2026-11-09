package integrations

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type DriveLink struct {
	ResourceID  string     `json:"resourceId"`
	URL         string     `json:"url"`
	ResourceKey string     `json:"resourceKey,omitempty"`
	Accessible  bool       `json:"accessible"`
	Name        string     `json:"name"`
	MIME        string     `json:"mime"`
	CheckedAt   *time.Time `json:"checkedAt,omitempty"`
	Issue       string     `json:"issue"`
}

var httpsLinks = regexp.MustCompile(`https://[^\s<>"()]+`)

func ParseDriveLink(raw string) (DriveLink, error) {
	out := DriveLink{}
	u, err := url.Parse(strings.TrimRight(raw, ".,;!*_`"))
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || (u.Host != "drive.google.com" && u.Host != "docs.google.com") {
		return out, errors.New("gunakan tautan Google Drive HTTPS")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) >= 3 && parts[1] == "d" {
		out.ResourceID = parts[2]
	}
	if len(parts) >= 3 && parts[0] == "drive" && parts[1] == "folders" {
		out.ResourceID = parts[2]
	}
	if len(parts) >= 4 && parts[0] == "drive" && parts[1] == "u" {
		for i, part := range parts {
			if part == "folders" && i+1 < len(parts) {
				out.ResourceID = parts[i+1]
			}
		}
	}
	if u.Path == "/open" || u.Path == "/uc" {
		out.ResourceID = u.Query().Get("id")
	}
	if !resourceID.MatchString(out.ResourceID) {
		return out, errors.New("identitas file/folder Drive tidak valid")
	}
	out.ResourceKey = u.Query().Get("resourcekey")
	if out.ResourceKey != "" && !resourceID.MatchString(out.ResourceKey) {
		return out, errors.New("resource key Drive tidak valid")
	}
	query := url.Values{"id": {out.ResourceID}}
	if out.ResourceKey != "" {
		query.Set("resourcekey", out.ResourceKey)
	}
	out.URL = "https://drive.google.com/open?" + query.Encode()
	return out, nil
}
func DriveLinks(body string) []DriveLink {
	out := []DriveLink{}
	seen := map[string]bool{}
	for _, raw := range httpsLinks.FindAllString(body, -1) {
		link, err := ParseDriveLink(raw)
		if err != nil || seen[link.ResourceID] {
			continue
		}
		seen[link.ResourceID] = true
		out = append(out, link)
		if len(out) == 10 {
			break
		}
	}
	return out
}

// ProbeDrive reads metadata only, at a fixed Google API host. A readable folder
// proves only access to that folder, never correctness or quality of its files.
func (s *Sources) ProbeDrive(ctx context.Context, link DriveLink) DriveLink {
	now := time.Now().UTC()
	link.CheckedAt = &now
	link.Accessible = false
	token, err := s.googleScopedToken(ctx, "https://www.googleapis.com/auth/drive.metadata.readonly", "SPACE_GOOGLE_DRIVE_ACCESS_TOKEN")
	if err != nil {
		link.Issue = "Akses metadata Drive belum dapat diperiksa; perlu tinjauan manusia."
		return link
	}
	endpoint := "https://www.googleapis.com/drive/v3/files/" + url.PathEscape(link.ResourceID) + "?" + url.Values{"supportsAllDrives": {"true"}, "fields": {"id,name,mimeType,trashed"}}.Encode()
	request, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		link.Issue = "Permintaan metadata Drive tidak valid."
		return link
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	if link.ResourceKey != "" {
		request.Header.Set("X-Goog-Drive-Resource-Keys", link.ResourceID+"/"+link.ResourceKey)
	}
	response, err := s.HTTP.Do(request)
	if err != nil {
		link.Issue = "Metadata Drive tidak dapat dihubungi; akses belum terkonfirmasi."
		return link
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		link.Issue = "File/folder belum dapat dibaca oleh akun integrasi Drive."
		return link
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		link.Issue = "Respons metadata Drive tidak lengkap."
		return link
	}
	var file struct {
		ID, Name, MimeType string
		Trashed            bool
	}
	if json.Unmarshal(data, &file) != nil || file.ID != link.ResourceID || file.Trashed || file.MimeType == "application/vnd.google-apps.shortcut" {
		link.Issue = "File terhapus, berupa shortcut, atau metadata tidak valid; perlu tinjauan."
		return link
	}
	link.Accessible = true
	link.Name = file.Name
	link.MIME = file.MimeType
	link.Issue = ""
	return link
}
