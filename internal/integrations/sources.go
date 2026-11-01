// Package integrations contains bounded, fixed-host source adapters. Secret
// material belongs to server configuration and never to connector API DTOs.
package integrations

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var resourceID = regexp.MustCompile(`^[A-Za-z0-9_-]{8,160}$`)

type SheetsConfig = api.SheetsConfig
type SourceItem = api.ConnectorSourceItem
type TrelloCard struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"desc"`
	URL         string `json:"url"`
	Closed      bool   `json:"closed"`
}
type Sources struct {
	HTTP   *http.Client
	mu     sync.Mutex
	tokens map[string]googleAuth
}

func New() *Sources {
	return &Sources{HTTP: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect sumber tidak diizinkan") }}}
}
func Hash(v any) string {
	b, _ := json.Marshal(v)
	s := sha256.Sum256(b)
	return fmt.Sprintf("%x", s[:])
}
func SheetID(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Host != "docs.google.com" {
			return "", errors.New("gunakan link Google Sheets HTTPS")
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) < 3 || parts[0] != "spreadsheets" || parts[1] != "d" {
			return "", errors.New("link spreadsheet tidak valid")
		}
		raw = parts[2]
	}
	if !resourceID.MatchString(raw) {
		return "", errors.New("identitas spreadsheet tidak valid")
	}
	return raw, nil
}
func ValidateSheets(c *SheetsConfig) error {
	if c.MirrorStage == "" {
		c.MirrorStage = "design"
	}
	if c.MirrorStage != "design" && c.MirrorStage != "humas" {
		return errors.New("pilih tahap desain atau Humas untuk mirror")
	}
	sid, err := SheetID(c.SpreadsheetID)
	if err != nil {
		return err
	}
	c.SpreadsheetID = sid
	if c.Tab == "" || len(c.Tab) > 100 || strings.ContainsAny(c.Tab, "\x00\r\n") {
		return errors.New("nama tab tidak valid")
	}
	if c.HeaderRow < 1 || c.HeaderRow > 1000 {
		return errors.New("baris header harus 1–1000")
	}
	used := map[int]bool{}
	for _, col := range []int{c.TitleColumn, c.PostingColumn, c.HumasColumn} {
		if col < 1 || col > 702 {
			return errors.New("petakan kolom judul, posting dan PJ Humas (1–702)")
		}
	}
	for _, col := range []int{c.KeyColumn, c.TitleColumn, c.PostingColumn, c.HumasColumn, c.DesignColumn, c.TrelloColumn} {
		if col < 0 || col > 702 {
			return errors.New("kolom di luar rentang")
		}
		if col > 0 {
			if used[col] {
				return errors.New("pemetaan kolom sumber tidak boleh tumpang tindih")
			}
			used[col] = true
		}
	}
	for field, col := range c.MirrorColumns {
		if field != "progress" && field != "evidenceUrl" && field != "matchStatus" {
			return errors.New("hanya progres, tautan bukti dan pencocokan yang dapat dimirror")
		}
		if col < 1 || col > 702 || used[col] {
			return errors.New("kolom mirror harus terpisah dari kolom sumber dan mirror lain")
		}
		used[col] = true
	}
	return nil
}
func (s *Sources) request(ctx context.Context, method, endpoint, token string, body any, out any) error {
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(data))
	if err != nil {
		return errors.New("alamat sumber tidak valid")
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return errors.New("sumber tidak dapat dihubungi; kredensial tidak dicatat pada error")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("sumber menjawab HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, (8<<20)+1))
	if err != nil {
		return errors.New("respons sumber terputus")
	}
	if len(b) > 8<<20 {
		return errors.New("respons sumber melebihi 8 MiB")
	}
	if out == nil {
		return nil
	}
	if json.Unmarshal(b, out) != nil {
		return errors.New("format respons sumber tidak valid")
	}
	return nil
}

type googleAuth struct {
	token   string
	expires time.Time
}

func (s *Sources) googleToken(ctx context.Context) (string, error) {
	return s.googleScopedToken(ctx, "https://www.googleapis.com/auth/spreadsheets", "SPACE_GOOGLE_ACCESS_TOKEN")
}
func (s *Sources) googleScopedToken(ctx context.Context, scope, env string) (string, error) {
	if token := os.Getenv(env); token != "" {
		return token, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if auth := s.tokens[scope]; auth.token != "" && time.Now().Before(auth.expires) {
		return auth.token, nil
	}
	path := os.Getenv("SPACE_GOOGLE_SERVICE_ACCOUNT")
	if path == "" {
		return "", errors.New("kredensial Google belum dikonfigurasi pada server")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", errors.New("file kredensial Google tidak dapat dibaca")
	}
	var cred struct {
		ClientEmail  string `json:"client_email"`
		PrivateKey   string `json:"private_key"`
		PrivateKeyID string `json:"private_key_id"`
	}
	if json.Unmarshal(data, &cred) != nil || cred.ClientEmail == "" {
		return "", errors.New("format kredensial Google tidak valid")
	}
	block, _ := pem.Decode([]byte(cred.PrivateKey))
	if block == nil {
		return "", errors.New("kunci Google tidak valid")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", errors.New("format kunci Google tidak didukung")
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return "", errors.New("kunci Google harus RSA")
	}
	enc := func(v any) string { b, _ := json.Marshal(v); return base64.RawURLEncoding.EncodeToString(b) }
	now := time.Now().Unix()
	unsigned := enc(map[string]any{"alg": "RS256", "typ": "JWT", "kid": cred.PrivateKeyID}) + "." + enc(map[string]any{"iss": cred.ClientEmail, "scope": scope, "aud": "https://oauth2.googleapis.com/token", "iat": now, "exp": now + 3600})
	sum := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", errors.New("gagal menandatangani akses Google")
	}
	values := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(values.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return "", errors.New("layanan akses Google tidak dapat dihubungi")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("otorisasi Google HTTP %d", resp.StatusCode)
	}
	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result) != nil || result.AccessToken == "" {
		return "", errors.New("respons akses Google tidak valid")
	}
	if s.tokens == nil {
		s.tokens = map[string]googleAuth{}
	}
	s.tokens[scope] = googleAuth{token: result.AccessToken, expires: time.Now().Add(time.Duration(max(0, result.ExpiresIn-120)) * time.Second)}
	return result.AccessToken, nil
}
func (s *Sources) SheetValues(ctx context.Context, c SheetsConfig, formulas bool) ([][]any, error) {
	if err := ValidateSheets(&c); err != nil {
		return nil, err
	}
	token, err := s.googleToken(ctx)
	if err != nil {
		return nil, err
	}
	render := "UNFORMATTED_VALUE"
	if formulas {
		render = "FORMULA"
	}
	a1 := "'" + strings.ReplaceAll(c.Tab, "'", "''") + "'!A1:ZZ10000"
	endpoint := "https://sheets.googleapis.com/v4/spreadsheets/" + url.PathEscape(c.SpreadsheetID) + "/values/" + url.PathEscape(a1) + "?valueRenderOption=" + render + "&dateTimeRenderOption=SERIAL_NUMBER"
	var out struct {
		Values [][]any `json:"values"`
	}
	if err := s.request(ctx, "GET", endpoint, token, nil, &out); err != nil {
		return nil, err
	}
	return out.Values, nil
}
func ParseRows(c SheetsConfig, rows [][]any) ([]SourceItem, error) {
	if len(rows) <= c.HeaderRow {
		return nil, errors.New("sumber kosong/tidak lengkap; data baik terakhir dipertahankan")
	}
	out := []SourceItem{}
	keys := map[string]bool{}
	for index, row := range rows {
		if index < c.HeaderRow {
			continue
		}
		cell := func(col int) string {
			if col < 1 || col > len(row) || row[col-1] == nil {
				return ""
			}
			return strings.TrimSpace(fmt.Sprint(row[col-1]))
		}
		title, dateRaw := cell(c.TitleColumn), cell(c.PostingColumn)
		if title == "" && dateRaw == "" {
			continue
		}
		if dateRaw == "" {
			continue
		} // month/section labels are not tasks
		for _, v := range row {
			if str, ok := v.(string); ok && strings.HasPrefix(str, "#") {
				return nil, errors.New("sumber mengandung error sel; tidak menerapkan snapshot parsial")
			}
		}
		item := SourceItem{Row: index + 1, Title: title, HumasName: cell(c.HumasColumn), DesignName: cell(c.DesignColumn), TrelloCardID: cell(c.TrelloColumn)}
		var date timeutil.Date
		var err error
		if n, e := strconv.ParseFloat(dateRaw, 64); e == nil && n > 1 && n < 3000000 {
			if n != float64(int64(n)) {
				item.Issue = "Tanggal sumber memiliki jam; pilih pemetaan waktu eksplisit sebelum menerapkan."
			}
			date = timeutil.DateOf(time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(n)))
		} else {
			date, err = timeutil.ParseDate(dateRaw)
			if err != nil {
				for _, format := range []string{"02/01/2006", "2/1/2006", "02-01-2006", "2 January 2006"} {
					if t, e := time.Parse(format, dateRaw); e == nil {
						date = timeutil.DateOf(t)
						err = nil
						break
					}
				}
			}
		}
		if err != nil {
			item.Issue = "Tanggal posting tidak dikenali. Tinjau tanpa menebak jam atau format."
		} else {
			item.PostingDate = &date
		}
		if item.Title == "" || item.HumasName == "" {
			item.Issue = "Judul/PJ Humas belum lengkap."
		}
		item.Key = cell(c.KeyColumn)
		if item.Key == "" && c.KeyColumn > 0 {
			item.Issue = "Identitas stabil sumber belum diisi."
		}
		if item.Key == "" {
			item.Key = "derived:" + Hash([]string{strings.ToLower(strings.Join(strings.Fields(title), " ")), dateRaw})
		}
		if keys[item.Key] {
			return nil, errors.New("identitas sumber berulang; perbaiki pemetaan sebelum impor")
		}
		keys[item.Key] = true
		item.Fingerprint = Hash(struct{ Title, Date, Humas, Design, Trello string }{title, dateRaw, item.HumasName, item.DesignName, item.TrelloCardID})
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil, errors.New("tidak ada baris kalender valid; tidak menghapus data aplikasi")
	}
	return out, nil
}
func (s *Sources) Trello(ctx context.Context, cardID string) (TrelloCard, error) {
	var out TrelloCard
	if strings.HasPrefix(cardID, "https://trello.com/c/") {
		u, err := url.Parse(cardID)
		if err != nil {
			return out, err
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) < 2 {
			return out, errors.New("tautan kartu tidak valid")
		}
		cardID = parts[1]
	}
	if !resourceID.MatchString(cardID) {
		return out, errors.New("identitas kartu Trello tidak valid")
	}
	key, token := os.Getenv("SPACE_TRELLO_KEY"), os.Getenv("SPACE_TRELLO_TOKEN")
	if key == "" || token == "" {
		return out, errors.New("kredensial Trello belum dikonfigurasi pada server")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.trello.com/1/cards/"+url.PathEscape(cardID)+"?fields=id,name,desc,url,closed", nil)
	if err != nil {
		return out, errors.New("permintaan Trello tidak valid")
	}
	req.Header.Set("Authorization", `OAuth oauth_consumer_key="`+url.QueryEscape(key)+`", oauth_token="`+url.QueryEscape(token)+`"`)
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return out, errors.New("Trello tidak dapat dihubungi")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return out, fmt.Errorf("Trello HTTP %d", resp.StatusCode)
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out) != nil || out.ID == "" {
		return out, errors.New("respons kartu Trello tidak valid")
	}
	return out, nil
}
