package appui

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
)

// AccountAPI is optional so a service without account-management endpoints can
// still run the shell. Account actions never report a save without its response.
type AccountAPI interface {
	InspectInvitation(context.Context, api.InvitationTokenRequest) (api.InvitationPreview, error)
	VerifyInvitationEmail(context.Context, api.InvitationTokenRequest) error
	AcceptInvitation(context.Context, api.InvitationAcceptRequest) (api.InvitationAcceptResponse, error)
	RequestRecovery(context.Context, api.RecoveryRequest) error
	ResetPassword(context.Context, api.RecoveryResetRequest) error
	ChangePassword(context.Context, api.PasswordChangeRequest) error
	RequestAdminEmail(context.Context, api.AdminEmailRequest) error
	AdminHandover(context.Context, api.AdminHandoverRequest) error
}

var _ AccountAPI = (*client.Client)(nil)

// Proofs and credentials live only here, never in preferences or token storage.
// seq changes whenever the flow is replaced; gen additionally binds requests to
// the current session. A late response cannot populate another account's form.
type accountFlowState struct {
	seq                                           uint64
	mode                                          string
	active, busy, awaitingLogin                   bool
	token, verification                           string
	preview                                       *api.InvitationPreview
	err, message                                  string
	link, email, name, password, confirm          string
	owner                                         id.ID
	currentPassword, newPassword, confirmPassword string
	adminEmail, custodian                         string
	handoverConfirm                               bool
}

type accountLink struct{ mode, token, verification string }

var errAccountLink = errors.New("Tautan akun tidak valid. Gunakan tautan undangan, pemulihan, atau serah terima dari email Space.")

// parseAccountLink accepts only recognized fragment routes and their exact
// parameters. It neither navigates to the supplied host nor includes raw input
// in errors, even for malformed or expired proof links.
func parseAccountLink(raw string) (accountLink, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 16384 {
		return accountLink{}, errAccountLink
	}
	fragment := ""
	if strings.HasPrefix(raw, "#") {
		fragment = strings.TrimPrefix(raw, "#")
	} else {
		u, err := url.Parse(raw)
		if err != nil || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.RawQuery != "" {
			return accountLink{}, errAccountLink
		}
		fragment = u.Fragment
	}
	u, err := url.Parse(fragment)
	if err != nil || u.Scheme != "" || u.Host != "" || u.Fragment != "" {
		return accountLink{}, errAccountLink
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return accountLink{}, errAccountLink
	}
	valid := func(key string, required bool) bool {
		vs := q[key]
		if len(vs) == 0 {
			return !required
		}
		if len(vs) != 1 || vs[0] == "" || len(vs[0]) > 4096 {
			return false
		}
		for _, r := range vs[0] {
			if unicode.IsSpace(r) || unicode.IsControl(r) {
				return false
			}
		}
		return true
	}
	switch u.Path {
	case "/undangan":
		if !valid("token", true) || !valid("verification", false) {
			return accountLink{}, errAccountLink
		}
		for k := range q {
			if k != "token" && k != "verification" {
				return accountLink{}, errAccountLink
			}
		}
		return accountLink{mode: "invite", token: q.Get("token"), verification: q.Get("verification")}, nil
	case "/pemulihan":
		if len(q) != 1 || !valid("token", true) {
			return accountLink{}, errAccountLink
		}
		return accountLink{mode: "reset", token: q.Get("token")}, nil
	case "/organisasi/serah-terima-admin":
		if len(q) != 1 || !valid("verification", true) {
			return accountLink{}, errAccountLink
		}
		return accountLink{mode: "admin", verification: q.Get("verification")}, nil
	}
	return accountLink{}, errAccountLink
}

// HandleAccountLink consumes a link into transient native state. Web callers
// remove the secret fragment from history before invoking this method.
func (a *App) HandleAccountLink(raw string) error {
	link, err := parseAccountLink(raw)
	if err != nil {
		a.openAccountLink()
		a.account.err = err.Error()
		return err
	}
	a.clearAccountFlow()
	a.account.mode, a.account.token, a.account.verification = link.mode, link.token, link.verification
	a.account.active = true
	return nil
}

func (a *App) clearAccountFlow() {
	seq := a.account.seq + 1
	a.account = accountFlowState{seq: seq}
}

// accountSessionEnded forgets credentials immediately on expiration/switch,
// while preserving a deliberate link for the invitation/admin login step.
func (a *App) accountSessionEnded() {
	a.account.seq++
	a.account.busy = false
	a.account.owner = id.ID{}
	a.account.password, a.account.confirm = "", ""
	a.account.currentPassword, a.account.newPassword, a.account.confirmPassword = "", "", ""
	a.account.name, a.account.email, a.account.link = "", "", ""
	a.account.adminEmail, a.account.custodian = "", ""
	a.account.handoverConfirm = false
}

func (a *App) openRecovery() {
	a.clearAccountFlow()
	a.account.mode, a.account.active, a.account.email = "recovery", true, a.email
}

// openAccountLink is also available in the desktop login, without OS deep links.
func (a *App) openAccountLink() {
	a.clearAccountFlow()
	a.account.mode, a.account.active = "paste", true
}

func (a *App) runAccount(work func(context.Context, AccountAPI) func()) {
	if a.account.busy {
		return
	}
	service, ok := a.api.(AccountAPI)
	if !ok {
		a.account.err = "Layanan akun belum tersedia pada server ini."
		return
	}
	// Pin authentication before leaving the view thread, not when the queued
	// goroutine eventually runs after a logout/account switch.
	if live, ok := a.api.(*client.Client); ok {
		service = live.AccountSession()
	}
	a.account.busy, a.account.err, a.account.message = true, "", ""
	seq, gen := a.account.seq, a.gen
	a.async(func() func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		apply := work(ctx, service)
		return func() {
			if a.account.seq != seq || a.gen != gen {
				return
			}
			a.account.busy = false
			if apply != nil {
				apply()
			}
		}
	})
}

func (a *App) inspectInvitation() {
	req := api.InvitationTokenRequest{Token: a.account.token}
	a.runAccount(func(ctx context.Context, service AccountAPI) func() {
		preview, err := service.InspectInvitation(ctx, req)
		return func() {
			if err != nil {
				a.account.err = errorText(err)
				return
			}
			a.account.preview = &preview
		}
	})
}

func (a *App) verifyInvitation() {
	req := api.InvitationTokenRequest{Token: a.account.token}
	a.runAccount(func(ctx context.Context, service AccountAPI) func() {
		err := service.VerifyInvitationEmail(ctx, req)
		return func() {
			if err != nil {
				a.account.err = errorText(err)
				return
			}
			a.account.message = "Permintaan verifikasi diterima. Buka tautan verifikasi dari email undanganmu, lalu tempel di Space bila memakai desktop."
		}
	})
}

func (a *App) invitationLogin() {
	if a.account.preview == nil {
		return
	}
	a.email = a.account.preview.Email
	a.pauseAccountForLogin("Masuk dengan email undangan untuk menerima keanggotaan.")
}

func (a *App) pauseAccountForLogin(notice string) {
	// Preserve the deliberate account link, never credentials belonging to the
	// previous user. A saved/offline session must be forgotten as well.
	a.account.seq++
	a.account.busy, a.account.active, a.account.awaitingLogin = false, false, true
	a.account.password, a.account.confirm, a.password = "", "", ""
	a.account.currentPassword, a.account.newPassword, a.account.confirmPassword = "", "", ""
	a.account.handoverConfirm = false
	a.toLogin(notice)
	write := a.tokenWrite()
	a.async(func() func() { write(""); return nil })
}

// resumeAccountFlow is called after enterShell. An invitation survives only the
// deliberate invitation login; credential forms never follow another account.
func (a *App) resumeAccountFlow() {
	a.account.password, a.account.confirm = "", ""
	a.account.currentPassword, a.account.newPassword, a.account.confirmPassword = "", "", ""
	a.account.handoverConfirm = false
	if a.account.awaitingLogin {
		a.account.seq++
		a.account.busy, a.account.active, a.account.awaitingLogin = false, true, false
		if a.account.mode == "admin" {
			return
		}
		if a.account.preview != nil && !strings.EqualFold(a.user.Email, a.account.preview.Email) {
			a.account.err = "Akun ini berbeda dari email undangan. Masuk dengan akun yang diundang."
		}
		return
	}
	// A startup link can be pending while a saved session is being confirmed.
	if a.account.active {
		a.account.busy = false
		a.account.seq++
		return
	}
	a.clearAccountFlow()
}

func (a *App) acceptInvitation() {
	p := a.account.preview
	if p == nil {
		return
	}
	req := api.InvitationAcceptRequest{Token: a.account.token}
	newAccount := !p.AccountExists
	if newAccount {
		if a.screen != screenLogin {
			a.account.err = "Keluar dari akun saat ini sebelum membuat akun undangan."
			return
		}
		if strings.TrimSpace(a.account.name) == "" {
			a.account.err = "Isi nama lengkapmu."
			return
		}
		if a.account.verification == "" {
			a.account.err = "Buka tautan verifikasi email terlebih dahulu."
			return
		}
		if !a.validateAccountPassword(a.account.password, a.account.confirm) {
			return
		}
		req.VerificationToken, req.DisplayName, req.Password = a.account.verification, strings.TrimSpace(a.account.name), a.account.password
	} else if a.screen != screenShell || a.user.Kind != api.AccountPerson || !a.user.EmailVerified || !strings.EqualFold(a.user.Email, p.Email) {
		a.account.err = "Masuk dengan akun pribadi terverifikasi yang diundang."
		return
	}
	email := p.Email
	a.runAccount(func(ctx context.Context, service AccountAPI) func() {
		_, err := service.AcceptInvitation(ctx, req)
		return func() {
			a.account.password, a.account.confirm = "", ""
			if err != nil {
				a.account.err = errorText(err)
				return
			}
			if newAccount {
				// Acceptance returns no session. Use the real login endpoint.
				a.email, a.password = email, req.Password
				a.clearAccountFlow()
				a.login()
				return
			}
			a.clearAccountFlow()
			a.account.message, a.account.mode, a.account.active = "Undangan diterima. Keanggotaanmu sudah tersimpan di server.", "success", true
			a.loadPeriods()
		}
	})
}

func (a *App) validateAccountPassword(password, confirm string) bool {
	if len([]rune(password)) < 10 {
		a.account.err = "Gunakan kata sandi minimal 10 karakter."
		return false
	}
	if password != confirm {
		a.account.err = "Konfirmasi kata sandi belum sama."
		return false
	}
	return true
}

func (a *App) requestRecovery() {
	email := strings.TrimSpace(a.account.email)
	if email == "" {
		a.account.err = "Isi alamat email akunmu."
		return
	}
	a.runAccount(func(ctx context.Context, service AccountAPI) func() {
		err := service.RequestRecovery(ctx, api.RecoveryRequest{Email: email})
		return func() {
			if err != nil {
				a.account.err = errorText(err)
				return
			}
			// Exactly the same message regardless of the email's registration.
			a.account.message = "Jika alamat tersebut memiliki akun aktif, tautan pemulihan akan dikirim. Periksa email dan folder spam. Tautan berlaku satu jam."
		}
	})
}

func (a *App) resetAccountPassword() {
	if !a.validateAccountPassword(a.account.password, a.account.confirm) {
		return
	}
	req := api.RecoveryResetRequest{Token: a.account.token, Password: a.account.password}
	a.runAccount(func(ctx context.Context, service AccountAPI) func() {
		err := service.ResetPassword(ctx, req)
		return func() {
			a.account.password, a.account.confirm = "", ""
			if err != nil {
				a.account.err = errorText(err)
				return
			}
			a.endAccountSession("Kata sandi berhasil dipulihkan. Semua sesi lama berakhir. Silakan masuk lagi.")
		}
	})
}

func (a *App) endAccountSession(notice string) {
	a.clearAccountFlow()
	a.password = ""
	a.toLogin(notice)
	write := a.tokenWrite()
	a.async(func() func() { write(""); return nil })
}

// accountFlowView returns false when the usual startup/login/shell should draw.
func (a *App) accountFlowView(c *ui.Context) bool {
	if !a.account.active || a.screen == screenStarting || a.busy {
		return false
	}
	if a.account.mode == "invite" && a.account.preview == nil && !a.account.busy && a.account.err == "" {
		a.inspectInvitation()
	}
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(32).AlignItems(ui.Center).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(530).Padding(28).Gap(16).Radius(12).Background(c.Theme().Surface).Border(1, c.Theme().Border).Children(func() {
				brand(c)
				switch a.account.mode {
				case "paste":
					a.accountPasteView(c)
				case "invite":
					a.invitationView(c)
				case "recovery":
					ui.Text(c, "Pulihkan akun").FontSize(21).Bold()
					muted(c, "Masukkan email akun. Respons yang sama diberikan untuk menjaga privasi pemilik email.")
					accountInput(c, "Email pemulihan", &a.account.email, false, a.account.busy)
					if spaceButton(c, "Kirim tautan pemulihan", "mail", false).Disabled(a.account.busy).Clicked() {
						a.requestRecovery()
					}
				case "reset":
					ui.Text(c, "Kata sandi baru").FontSize(21).Bold()
					muted(c, "Tautan hanya dapat dipakai sekali. Setelah berhasil, seluruh sesi akun akan berakhir.")
					accountInput(c, "Kata sandi baru", &a.account.password, true, a.account.busy)
					accountInput(c, "Ulangi kata sandi baru", &a.account.confirm, true, a.account.busy)
					if spaceButton(c, "Simpan kata sandi baru", "lock", false).Disabled(a.account.busy).Clicked() {
						a.resetAccountPassword()
					}
				case "admin":
					a.adminProofView(c)
				case "success":
					ui.Text(c, "Keanggotaan tersimpan").FontSize(21).Bold()
				}
				if a.account.busy {
					muted(c, "Menghubungi server…")
				}
				accountMessages(c, a.account.err, a.account.message)
				if spaceButton(c, "Kembali", "arrow", false).Clicked() {
					a.clearAccountFlow()
				}
				muted(c, "Server: "+a.ServerURL)
			})
		})
	})
	return true
}

func accountInput(c *ui.Context, label string, value *string, password, disabled bool) {
	ui.Field(c, label, func() {
		field := ui.TextInput(c.Key(label), value).Disabled(disabled)
		if password {
			field.Password()
		}
	})
}

func accountMessages(c *ui.Context, err, message string) {
	if err != "" {
		ui.Text(c, err).TextColor(c.Theme().Danger)
	}
	if message != "" {
		ui.Text(c, message).TextColor(c.Theme().Accent)
	}
}

func (a *App) accountPasteView(c *ui.Context) {
	ui.Text(c, "Buka tautan akun").FontSize(21).Bold()
	muted(c, "Tempel tautan undangan, verifikasi email, pemulihan, atau serah terima admin dari email Space. Tautan hanya digunakan selama alur ini.")
	accountInput(c, "Tautan dari email", &a.account.link, false, a.account.busy)
	if spaceButton(c, "Buka tautan", "link", false).Clicked() {
		raw := a.account.link
		a.account.link = ""
		if err := a.HandleAccountLink(raw); err != nil {
			a.account.err = err.Error()
		}
	}
}

func (a *App) invitationView(c *ui.Context) {
	ui.Text(c, "Undangan ke Space").FontSize(21).Bold()
	p := a.account.preview
	if p == nil {
		muted(c, "Memeriksa undangan dan alamat email…")
		if a.account.err != "" && spaceButton(c, "Periksa lagi", "refresh", false).Disabled(a.account.busy).Clicked() {
			a.inspectInvitation()
		}
		return
	}
	info(c, [][2]string{{"Email", p.Email}, {"Divisi", p.DivisionName}, {"Periode", p.PeriodLabel}, {"Berlaku sampai", p.ExpiresAt.Local().Format("02 Jan 2006 15:04 MST")}})
	if p.AccountExists {
		if a.screen != screenShell || !strings.EqualFold(a.user.Email, p.Email) || a.user.Kind != api.AccountPerson {
			muted(c, "Email ini sudah memiliki akun. Masuk dahulu, lalu konfirmasikan undangan ini.")
			if spaceButton(c, "Masuk dengan email undangan", "user", false).Disabled(a.account.busy).Clicked() {
				a.invitationLogin()
			}
		} else if spaceButton(c, "Terima undangan", "check", false).Disabled(a.account.busy).Clicked() {
			a.acceptInvitation()
		}
		return
	}
	if a.screen != screenLogin {
		muted(c, "Undangan ini untuk akun baru. Akhiri sesi saat ini sebelum melanjutkan.")
		if spaceButton(c, "Lanjut sebagai penerima undangan", "user", false).Clicked() {
			a.invitationLogin()
			a.account.awaitingLogin, a.account.active = false, true
		}
		return
	}
	if a.account.verification == "" {
		muted(c, "Verifikasi kepemilikan email sebelum membuat akun. Tautan undangan saja belum menjadi bukti email.")
		if spaceButton(c, "Kirim verifikasi email", "mail", false).Disabled(a.account.busy).Clicked() {
			a.verifyInvitation()
		}
		// Pasting replaces this flow, including its old proof and credentials.
		if spaceButton(c, "Tempel tautan verifikasi", "link", false).Disabled(a.account.busy).Clicked() {
			a.openAccountLink()
		}
		return
	}
	muted(c, "Bukti email diterima dari tautan. Server akan memeriksa keabsahannya saat akun dibuat.")
	accountInput(c, "Nama lengkap", &a.account.name, false, a.account.busy)
	accountInput(c, "Kata sandi akun baru", &a.account.password, true, a.account.busy)
	accountInput(c, "Ulangi kata sandi akun baru", &a.account.confirm, true, a.account.busy)
	if spaceButton(c, "Buat akun dan terima undangan", "check", false).Disabled(a.account.busy).Clicked() {
		a.acceptInvitation()
	}
}

func (a *App) adminProofView(c *ui.Context) {
	ui.Text(c, "Verifikasi email admin").FontSize(21).Bold()
	if a.screen != screenShell {
		muted(c, "Masuk dengan akun admin organisasi untuk melanjutkan serah terima. Bukti email disimpan hanya selama alur ini.")
		if spaceButton(c, "Masuk sebagai admin", "shield", false).Clicked() {
			a.email = ""
			a.pauseAccountForLogin("Masuk dengan akun admin organisasi untuk melanjutkan serah terima.")
		}
		return
	}
	if !a.isAdmin() {
		muted(c, "Tautan ini hanya dapat digunakan oleh akun admin organisasi.")
		if spaceButton(c, "Ganti ke akun admin", "shield", false).Clicked() {
			a.email = ""
			a.pauseAccountForLogin("Masuk dengan akun admin organisasi untuk melanjutkan serah terima.")
		}
		return
	}
	muted(c, "Bukti email siap diperiksa. Isi kembali email baru dan kredensial pada formulir serah terima.")
	if spaceButton(c, "Ke pengaturan akun admin", "settings", false).Clicked() {
		a.account.active, a.account.owner = false, a.user.ID
		a.chrome.settingsTab = "Akun"
		a.router.Push(PathPengaturan)
	}
}

// accountCredentialsView renders the authenticated account operations inside the
// existing settings page, using the same custom controls as the rest of Space.
func (a *App) accountCredentialsView(c *ui.Context) {
	if a.screen != screenShell {
		return
	}
	if a.account.owner != a.user.ID {
		proof := ""
		if a.account.mode == "admin" && a.isAdmin() {
			proof = a.account.verification
		}
		a.clearAccountFlow()
		a.account.owner, a.account.verification = a.user.ID, proof
	}
	pageSection(c, "Keamanan akun", "Perubahan kata sandi mengakhiri sesi di perangkat lain; sesi ini tetap aktif.", func() {
		accountInput(c, "Kata sandi saat ini", &a.account.currentPassword, true, a.account.busy)
		accountInput(c, "Kata sandi pengganti", &a.account.newPassword, true, a.account.busy)
		accountInput(c, "Ulangi kata sandi pengganti", &a.account.confirmPassword, true, a.account.busy)
		if spaceButton(c, "Ubah kata sandi", "lock", false).Disabled(a.account.busy).Clicked() {
			a.changeAccountPassword()
		}
	})
	if a.isAdmin() {
		pageSection(c, "Serah terima akun admin", "Akun bersama tetap satu. Kata sandi baru wajib; email pemulihan baru memerlukan bukti email. Semua sesi berakhir setelah serah terima.", func() {
			accountInput(c, "Pengelola baru", &a.account.custodian, false, a.account.busy)
			accountInput(c, "Email pemulihan baru (opsional)", &a.account.adminEmail, false, a.account.busy)
			muted(c, "Gunakan kata sandi saat ini dan kata sandi pengganti pada bagian keamanan di atas.")
			if a.account.verification != "" {
				muted(c, "Bukti email dimuat dari tautan. Keabsahan dan kesesuaian alamat tetap diperiksa server.")
			}
			if spaceButton(c, "Kirim bukti email admin", "mail", false).Disabled(a.account.busy).Clicked() {
				a.requestAdminEmail()
			}
			if spaceButton(c, "Tempel tautan bukti email", "link", false).Disabled(a.account.busy).Clicked() {
				a.openAccountLink()
			}
			if !a.account.handoverConfirm {
				if spaceButton(c, "Tinjau serah terima", "shield", false).Disabled(a.account.busy).Clicked() {
					a.prepareAdminHandover()
				}
			} else {
				pageNotice(c, "alert", "Konfirmasi serah terima", "Pengelola: "+a.account.custodian+". Semua perangkat, termasuk perangkat ini, akan keluar. Pastikan pengelola baru sudah menerima kredensial melalui jalur yang aman.")
				if spaceButton(c, "Konfirmasi dan akhiri semua sesi", "check", false).Disabled(a.account.busy).Clicked() {
					a.submitAdminHandover()
				}
				if spaceButton(c, "Batal serah terima", "close", false).Disabled(a.account.busy).Clicked() {
					a.account.handoverConfirm = false
				}
			}
		})
	}
	if a.account.busy {
		muted(c, "Menyimpan ke server…")
	}
	accountMessages(c, a.account.err, a.account.message)
}

func (a *App) changeAccountPassword() {
	if a.account.currentPassword == "" {
		a.account.err = "Isi kata sandi saat ini."
		return
	}
	if !a.validateAccountPassword(a.account.newPassword, a.account.confirmPassword) {
		return
	}
	req := api.PasswordChangeRequest{CurrentPassword: a.account.currentPassword, Password: a.account.newPassword}
	a.runAccount(func(ctx context.Context, s AccountAPI) func() {
		err := s.ChangePassword(ctx, req)
		return func() {
			a.account.currentPassword, a.account.newPassword, a.account.confirmPassword = "", "", ""
			if err != nil {
				a.account.err = errorText(err)
				return
			}
			a.account.message = "Kata sandi berhasil diubah. Sesi pada perangkat lain telah berakhir."
		}
	})
}

func (a *App) requestAdminEmail() {
	if !a.isAdmin() {
		return
	}
	if a.account.currentPassword == "" || strings.TrimSpace(a.account.adminEmail) == "" {
		a.account.err = "Isi kata sandi saat ini dan email pemulihan baru."
		return
	}
	req := api.AdminEmailRequest{CurrentPassword: a.account.currentPassword, Email: strings.TrimSpace(a.account.adminEmail)}
	a.runAccount(func(ctx context.Context, s AccountAPI) func() {
		err := s.RequestAdminEmail(ctx, req)
		return func() {
			a.account.currentPassword = ""
			if err != nil {
				a.account.err = errorText(err)
				return
			}
			a.account.verification = ""
			a.account.message = "Permintaan bukti email diterima. Buka tautan di email baru sebelum mengonfirmasi serah terima."
		}
	})
}

func (a *App) prepareAdminHandover() bool {
	if !a.isAdmin() {
		return false
	}
	if strings.TrimSpace(a.account.custodian) == "" || a.account.currentPassword == "" {
		a.account.err = "Isi pengelola baru dan kata sandi saat ini."
		return false
	}
	if !a.validateAccountPassword(a.account.newPassword, a.account.confirmPassword) {
		return false
	}
	if strings.TrimSpace(a.account.adminEmail) != "" && a.account.verification == "" {
		a.account.err = "Buka tautan bukti untuk email pemulihan baru terlebih dahulu."
		return false
	}
	a.account.handoverConfirm, a.account.err = true, ""
	return true
}

func (a *App) submitAdminHandover() {
	if !a.account.handoverConfirm || !a.prepareAdminHandover() {
		return
	}
	req := api.AdminHandoverRequest{CurrentPassword: a.account.currentPassword, Password: a.account.newPassword, Email: strings.TrimSpace(a.account.adminEmail), VerificationToken: a.account.verification, Custodian: strings.TrimSpace(a.account.custodian)}
	if req.Email == "" {
		req.VerificationToken = ""
	}
	a.runAccount(func(ctx context.Context, s AccountAPI) func() {
		err := s.AdminHandover(ctx, req)
		return func() {
			a.account.currentPassword, a.account.newPassword, a.account.confirmPassword = "", "", ""
			a.account.handoverConfirm = false
			if err != nil {
				a.account.err = errorText(err)
				return
			}
			a.endAccountSession("Serah terima admin berhasil. Semua sesi telah berakhir. Pengelola baru dapat masuk dengan kredensial yang baru.")
		}
	})
}
