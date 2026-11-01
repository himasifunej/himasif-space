package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/himasifunej/himasif-space/internal/api"
)

type waLanguageKey struct{}
type waPhrase struct{ id, en string }

var waPhrases = map[string]waPhrase{
	"content.review":          {"perlu tinjauan; pasangan belum dipastikan", "review required; the match is not confirmed"},
	"content.concept":         {"penyerahan konsep tercatat; publikasi tetap terpisah", "concept submission recorded; publication remains separate"},
	"content.evidence":        {"hasil tercatat; kecocokan bukti menunggu tinjauan kadiv", "result recorded; evidence matching awaits division manager review"},
	"content.kind.concept":    {"Konsep", "Concept"},
	"content.kind.drive":      {"Drive", "Drive"},
	"content.kind.correction": {"Koreksi pesan", "Message correction"},

	"help":                  {"Command Space:\n!tugas — tugas aktif di ruang ini\n!pending — bukti menunggu tinjauan\n!cek <ID tugas> — detail tugas\n!hari — tenggat hari ini\n!jadwal — rapat mendatang\n!done <ID tugas> <tautan/catatan hasil> — ajukan hasil untuk tinjauan\n!bantu <ID tugas> [pesan] — ajukan bantuan\n!summary — ringkasan tugas\n!rekap — rekap SOP sesuai izin\n!eval <quarter> — evaluasi quarter terkonfigurasi", "Space commands:\n!tugas — active tasks in this space\n!pending — submissions awaiting review\n!cek <task ID> — task details\n!hari — deadlines today\n!jadwal — upcoming meetings\n!done <task ID> <result URL/note> — submit a result for review\n!bantu <task ID> [message] — offer help\n!summary — task summary\n!rekap — SOP report within your access\n!eval <quarter> — a configured quarter report"},
	"link.success":          {"Identitas WhatsApp berhasil ditautkan. Hak akses tetap mengikuti keanggotaan akun Space saat command dijalankan.", "WhatsApp identity linked. Each command still checks your current Space account membership and permissions."},
	"tasks.empty":           {"Tidak ada tugas yang sesuai di ruang ini.", "No matching tasks in this space."},
	"tasks.limit":           {"Daftar dibatasi 30 tugas. Buka Space untuk seluruh daftar.", "Showing up to 30 tasks. Open Space for the full list."},
	"meetings.empty":        {"Tidak ada rapat mendatang di ruang ini.", "No upcoming meetings in this space."},
	"task.details":          {"%s\nPJ: %s\nTenggat: %s\nProgres: %s\nPencocokan: %s\nID %s", "%s\nAssignee: %s\nDeadline: %s\nProgress: %s\nResult review: %s\nID %s"},
	"task.done":             {"Tugas sudah selesai. Tidak ada temuan atau pelanggaran SOP yang disahkan oleh command ini.", "The task is complete. This command does not confirm any SOP finding or violation."},
	"task.submitted":        {"Hasil tersimpan sebagai penyerahan yang perlu tinjauan. Ini belum berarti bukti cocok, mutu disetujui, atau konten sudah dipublikasikan.", "The result was submitted for review. It does not yet mean the evidence matches, quality is approved, or content is published."},
	"help.submitted":        {"Pengajuan bantuan tersimpan dan menunggu keputusan pengelola tugas.", "Your offer to help was saved and awaits the task manager's decision."},
	"task.delegated":        {"Delegasi tersimpan ke akun %s; alasan dan riwayat perubahan dicatat.", "The task was assigned to %s; the reason and change history were recorded."},
	"sticker.saved":         {"Stiker tersimpan sebagai !%s. Cakupan grup dapat dikelola di pengaturan command.", "Sticker saved as !%s. Manage its group restrictions in command settings."},
	"tasks.summary":         {"Ringkasan tugas ruang ini\nAktif: %d\nHasil perlu tinjauan: %d\nSelesai: %d", "Task summary for this space\nActive: %d\nSubmissions awaiting review: %d\nComplete: %d"},
	"report.week":           {"Tujuh hari terakhir", "Last seven days"},
	"report.weekly":         {"Mingguan", "Weekly"},
	"report.title":          {"Rekap mingguan SOP", "Weekly SOP report"},
	"report.body":           {"Rekap SOP %s · %s\nRentang: %s — %s\nPerlu tinjauan: %d\nDisahkan: %d\nDibatalkan: %d\nKeputusan perlu diperiksa ulang: %d\nBukti belum cukup: %d\nPengesahan dan pencocokan hasil adalah tindakan terpisah. Buka Space untuk bukti dan riwayat.", "SOP report %s · %s\nRange: %s — %s\nAwaiting review: %d\nConfirmed: %d\nDismissed: %d\nDecisions requiring recheck: %d\nInsufficient evidence: %d\nConfirming a finding and reviewing a task result are separate actions. Open Space for evidence and history."},
	"followup":              {"Koordinasi H-4 atas nama %s\nKonten: %s\nPJ desain: %s @%s\nJadwal posting: %s\nKonsep: %s", "Coordination four days before posting on behalf of %s\nContent: %s\nDesign assignee: %s @%s\nPosting date: %s\nConcept: %s"},
	"content.receipt":       {"Pesan diperiksa: %d item baru diterapkan, %d item perlu tinjauan. Penyerahan desain belum berarti hasil cocok atau konten dipublikasikan.", "Message checked: %d new items applied; %d items need review. A design submission does not mean the result matches or the content is published."},
	"truncated":             {"… Daftar dibatasi. Buka Space untuk seluruh detail.", "… List shortened. Open Space for the full details."},
	"reminder":              {"Pengingat: %s", "Reminder: %s"},
	"overdue":               {"Tugas melewati tenggat: %s", "Task overdue: %s"},
	"meeting.created":       {"Rapat baru: %s\n%s", "New meeting: %s\n%s"},
	"meeting.changed":       {"Perubahan rapat: %s\n%s", "Meeting changed: %s\n%s"},
	"meeting.cancelled":     {"Rapat dibatalkan: %s\n%s", "Meeting cancelled: %s\n%s"},
	"notice.generic":        {"Pembaruan tugas: %s", "Task update: %s"},
	"notice.new":            {"Tugas baru: %s", "New task: %s"},
	"notice.assignment":     {"Penugasan atau tenggat berubah: %s", "Assignment or deadline changed: %s"},
	"notice.help":           {"Pengajuan bantuan: %s", "Offer to help: %s"},
	"notice.help_decision":  {"Keputusan pengajuan bantuan: %s", "Help decision: %s"},
	"notice.submission":     {"Hasil dikirim — perlu tinjauan: %s", "Result submitted — review required: %s"},
	"notice.review":         {"Pencocokan hasil diperbarui: %s", "Result review updated: %s"},
	"notice.medhum_new":     {"Tugas Medhum baru: %s", "New Medhum task: %s"},
	"notice.medhum_changed": {"Jadwal atau PJ Medhum berubah: %s", "Medhum schedule or assignee changed: %s"},
	"notice.concept":        {"Konsep Humas diserahkan: %s", "Humas concept submitted: %s"},
}

func waLanguage(ctx context.Context) string {
	language, _ := ctx.Value(waLanguageKey{}).(string)
	if language == "en" {
		return "en"
	}
	return "id"
}
func waText(ctx context.Context, key string, args ...any) string {
	return waTextLanguage(waLanguage(ctx), key, args...)
}
func waTextLanguage(language, key string, args ...any) string {
	phrase, exists := waPhrases[key]
	if !exists {
		return key
	}
	format := phrase.id
	if language == "en" {
		format = phrase.en
	}
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}
func waProgress(language, value string) string {
	labels := map[string]waPhrase{"todo": {"Belum mulai", "Not started"}, "doing": {"Sedang dikerjakan", "In progress"}, "submitted": {"Diserahkan", "Submitted"}, "done": {"Selesai", "Complete"}, "cancelled": {"Dibatalkan", "Cancelled"}}
	phrase, exists := labels[value]
	if !exists {
		return value
	}
	if language == "en" {
		return phrase.en
	}
	return phrase.id
}
func waMatch(language, value string) string {
	labels := map[string]waPhrase{"none": {"Belum ada hasil", "No submission"}, "pending": {"Perlu tinjauan", "Awaiting review"}, "verified": {"Cocok", "Verified match"}, "rejected": {"Belum cocok", "Not a match"}}
	phrase, exists := labels[value]
	if !exists {
		return value
	}
	if language == "en" {
		return phrase.en
	}
	return phrase.id
}
func waFailure(language string, failure *apiError) string {
	if language != "en" {
		return failure.message
	}
	if message, exists := waEnglishErrors[failure.message]; exists {
		return message
	}
	switch failure.code {
	case api.CodeForbidden:
		return "You do not have permission for this action. Open Space to check your current membership and access."
	case api.CodeNotFound:
		return "The requested record is unavailable within your current access."
	case api.CodePeriodClosed:
		return "This period is closed and can only be read."
	case api.CodeConflict:
		return "The request conflicts with the current record. Open Space and review the latest state before trying again."
	}
	return fmt.Sprintf("The request could not be completed (%s). Check the command syntax in !help and open Space for details.", failure.code)
}

var waNoticeKeys = map[string]string{"Tugas baru": "notice.new", "Penugasan atau tenggat berubah": "notice.assignment", "Pengajuan bantuan": "notice.help", "Keputusan pengajuan bantuan": "notice.help_decision", "Hasil dikirim — perlu tinjauan": "notice.submission", "Pencocokan hasil diperbarui": "notice.review", "Tugas Medhum baru": "notice.medhum_new", "Jadwal atau PJ Medhum berubah": "notice.medhum_changed", "Konsep Humas diserahkan": "notice.concept"}

func waTaskNoticeKey(title string) string {
	if key := waNoticeKeys[title]; key != "" {
		return key
	}
	return "notice.generic"
}
func waDeliveryText(language, key, title, body string, args []string) (string, string) {
	if _, exists := waPhrases[key]; !exists {
		return title, body
	}
	switch {
	case key == "reminder" || key == "overdue":
		if len(args) == 1 {
			return args[0], waTextLanguage(language, key, args[0])
		}
	case strings.HasPrefix(key, "notice."):
		if len(args) == 1 {
			return strings.TrimSuffix(waTextLanguage(language, key, ""), ": "), waTextLanguage(language, key, args[0])
		}
	case strings.HasPrefix(key, "meeting."):
		if len(args) == 2 {
			return args[0], waTextLanguage(language, key, args[0], args[1])
		}
	}
	return title, body
}
