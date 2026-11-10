package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/mail"
)

// Lifetimes of the tokens sent by email. Invitations last seven days from
// each sending (ADR 0001, OPEN-10); proofs and recovery one hour.
const (
	invitationTTL = 7 * 24 * time.Hour
	proofTTL      = time.Hour
	recoveryTTL   = time.Hour
)

// Pages of the app that email links open. The router of the app keeps its
// page in the URL's fragment, so the token never reaches a server log.
const (
	pageInvitation    = "/#/undangan"
	pageRecovery      = "/#/pemulihan"
	pageAdminHandover = "/#/organisasi/serah-terima-admin"
)

var errMailThrottled = fail(http.StatusTooManyRequests, api.CodeRateLimited, "Terlalu banyak email terkirim dalam semenit terakhir. Coba lagi sebentar lagi.")

// errMailRejected: the mail provider refused the address for good. In the
// Amazon SES sandbox this is also what an unverified recipient gets.
func errMailRejected(to string) error {
	msg := "Server email menolak alamat " + to + ". Periksa alamatnya, lalu coba lagi."
	if api.IsStudentEmail(to) {
		msg = "Server email menolak alamat " + to + ". Periksa NIM-nya, lalu coba lagi."
	}
	return fail(http.StatusUnprocessableEntity, api.CodeBadRequest, msg)
}

func (s *Server) link(page string, params url.Values) string {
	return s.cfg.PublicURL + page + "?" + params.Encode()
}

// send sends m, bounded in time. A failure is logged without the body,
// which holds a link that acts as a credential.
func (s *Server) send(ctx context.Context, m mail.Message) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err := s.mailer.Send(ctx, m)
	if err == nil {
		return nil
	}
	s.log.Error("mail: send", "to", m.To, "subject", m.Subject, "err", err)
	var rejected *mail.RejectedError
	switch {
	case errors.Is(err, mail.ErrThrottled):
		return errMailThrottled
	case errors.As(err, &rejected):
		return errMailRejected(m.To)
	}
	return errMailFailed
}

func wib(t time.Time) string {
	return t.In(time.FixedZone("WIB", 7*60*60)).Format("2 Jan 2006 15.04") + " WIB"
}

func invitationMail(s *Server, to, division, period, token string, expires time.Time) mail.Message {
	return mail.Letter{
		To:         to,
		Subject:    fmt.Sprintf("Undangan Space — %s %s", division, period),
		Preheader:  fmt.Sprintf("Bergabung ke divisi %s HIMASIF periode %s.", division, period),
		Heading:    fmt.Sprintf("Kamu diundang ke divisi %s", division),
		Paragraphs: []string{fmt.Sprintf("Kamu diundang bergabung ke divisi %s HIMASIF periode %s di Space, ruang kerja pengurus untuk tugas, rapat dan catatan.", division, period)},
		Button:     &mail.Button{Label: "Terima undangan", URL: s.link(pageInvitation, url.Values{"token": {token}})},
		Notes: []string{
			fmt.Sprintf("Undangan ini berlaku sampai %s dan hanya untuk alamat email %s. Tautannya boleh dibagikan lewat WhatsApp, tetapi akun baru tetap harus memverifikasi email ini.", wib(expires), to),
			"Jika kamu tidak merasa diundang, abaikan email ini.",
		},
	}.Message()
}

func proofMail(s *Server, to, invitationToken, proof string) mail.Message {
	return mail.Letter{
		To:         to,
		Subject:    "Verifikasi email untuk akun Space",
		Preheader:  "Satu langkah lagi untuk membuat akun Space.",
		Heading:    "Verifikasi email",
		Paragraphs: []string{"Gunakan tombol ini untuk memverifikasi email dan membuat akun Space."},
		Button:     &mail.Button{Label: "Verifikasi email", URL: s.link(pageInvitation, url.Values{"token": {invitationToken}, "verification": {proof}})},
		Notes:      []string{"Tautan berlaku satu jam dan hanya sekali pakai. Jika kamu tidak memintanya, abaikan email ini."},
	}.Message()
}

func recoveryMail(s *Server, to, token string) mail.Message {
	return mail.Letter{
		To:         to,
		Subject:    "Pemulihan akun Space",
		Preheader:  "Atur ulang kata sandi akun Space.",
		Heading:    "Atur ulang kata sandi",
		Paragraphs: []string{"Ada permintaan mengatur ulang kata sandi akun Space untuk email ini."},
		Button:     &mail.Button{Label: "Atur ulang kata sandi", URL: s.link(pageRecovery, url.Values{"token": {token}})},
		Notes: []string{
			"Tautan berlaku satu jam dan hanya sekali pakai. Setelah kata sandi diganti, semua sesi yang masuk akan berakhir.",
			"Jika kamu tidak memintanya, abaikan email ini; kata sandimu tidak berubah.",
		},
	}.Message()
}

func adminEmailMail(s *Server, to, proof string) mail.Message {
	return mail.Letter{
		To:         to,
		Subject:    "Verifikasi email pemulihan admin organisasi Space",
		Preheader:  "Selesaikan serah terima admin organisasi.",
		Heading:    "Verifikasi email pemulihan admin",
		Paragraphs: []string{"Email ini diajukan sebagai email pemulihan akun admin organisasi HIMASIF di Space. Gunakan tombol ini untuk menyelesaikan serah terima admin."},
		Button:     &mail.Button{Label: "Verifikasi email", URL: s.link(pageAdminHandover, url.Values{"verification": {proof}})},
		Notes:      []string{"Tautan berlaku satu jam dan hanya sekali pakai. Jika kamu tidak terlibat serah terima admin, abaikan email ini."},
	}.Message()
}
