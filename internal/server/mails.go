package server

import (
	"context"
	"fmt"
	"net/url"
	"time"

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

func (s *Server) link(page string, params url.Values) string {
	return s.cfg.PublicURL + page + "?" + params.Encode()
}

// send sends m, bounded in time. A failure is logged without the body,
// which holds a link that acts as a credential.
func (s *Server) send(ctx context.Context, m mail.Message) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := s.mailer.Send(ctx, m); err != nil {
		s.log.Error("mail: send", "to", m.To, "subject", m.Subject, "err", err)
		return errMailFailed
	}
	return nil
}

func wib(t time.Time) string {
	return t.In(time.FixedZone("WIB", 7*60*60)).Format("2 Jan 2006 15.04") + " WIB"
}

func invitationMail(s *Server, to, division, period, token string, expires time.Time) mail.Message {
	return mail.Message{
		To:      to,
		Subject: fmt.Sprintf("Undangan Space — %s %s", division, period),
		Text: fmt.Sprintf("Halo,\n\nKamu diundang bergabung ke divisi %s HIMASIF periode %s di Space.\n\n"+
			"Buka tautan ini untuk menerima undangan:\n%s\n\n"+
			"Undangan ini berlaku sampai %s dan hanya untuk alamat email %s. "+
			"Tautan boleh dibagikan lewat WhatsApp, tetapi akun baru tetap harus memverifikasi email ini.\n\n"+
			"Jika kamu tidak merasa diundang, abaikan email ini.\n\n— Space HIMASIF",
			division, period, s.link(pageInvitation, url.Values{"token": {token}}), wib(expires), to),
	}
}

func proofMail(s *Server, to, invitationToken, proof string) mail.Message {
	return mail.Message{
		To:      to,
		Subject: "Verifikasi email untuk akun Space",
		Text: fmt.Sprintf("Halo,\n\nGunakan tautan ini untuk memverifikasi email dan membuat akun Space:\n%s\n\n"+
			"Tautan berlaku satu jam dan hanya sekali pakai. Jika kamu tidak memintanya, abaikan email ini.\n\n— Space HIMASIF",
			s.link(pageInvitation, url.Values{"token": {invitationToken}, "verification": {proof}})),
	}
}

func recoveryMail(s *Server, to, token string) mail.Message {
	return mail.Message{
		To:      to,
		Subject: "Pemulihan akun Space",
		Text: fmt.Sprintf("Halo,\n\nAda permintaan mengatur ulang kata sandi akun Space untuk email ini. Buka tautan berikut:\n%s\n\n"+
			"Tautan berlaku satu jam dan hanya sekali pakai. Setelah kata sandi diganti, semua sesi yang masuk akan berakhir.\n"+
			"Jika kamu tidak memintanya, abaikan email ini; kata sandimu tidak berubah.\n\n— Space HIMASIF",
			s.link(pageRecovery, url.Values{"token": {token}})),
	}
}

func adminEmailMail(s *Server, to, proof string) mail.Message {
	return mail.Message{
		To:      to,
		Subject: "Verifikasi email pemulihan admin organisasi Space",
		Text: fmt.Sprintf("Halo,\n\nEmail ini diajukan sebagai email pemulihan akun admin organisasi HIMASIF di Space. "+
			"Gunakan tautan ini untuk menyelesaikan serah terima admin:\n%s\n\n"+
			"Tautan berlaku satu jam dan hanya sekali pakai. Jika kamu tidak terlibat serah terima admin, abaikan email ini.\n\n— Space HIMASIF",
			s.link(pageAdminHandover, url.Values{"verification": {proof}})),
	}
}
