# Kontrak tugas/jobdesk persisten — irisan Sesi 2A

Acuan PRD SPC01–06, TSK01–09, AC03–05; akun/akses memakai backend1B–1C. DTO tetap di internal/api/tasks.go. Ini pekerjaan tugas, bukan scheduler2C, konektor/SOP4, editor notes2B atau notifikasi OS3.

## Endpoint

- GET `/api/spaces/{spaceId}/tasks`: `api.List[api.Task]`, hanya space berizin saat request. Query opsional `assigneeId`, `progress`, `dueBefore` (YYYY-MM-DD pada zona periode).
- GET `/api/periods/{periodId}/tasks/mine`: daftar canonical tasks tempat pemanggil PJ/pelaksana tambahan, masih berizin membaca. Filter sama.
- POST `/api/spaces/{spaceId}/tasks`: api.TaskWriteRequest,201Task. OwnerDivisionID wajib, actor memiliki tasks.manage pada divisi pemilik. Space asal divisi selalu terhubung; jika membuat dari shared space, hubungkan shared tersebut. SpaceIDs hanya tambahan shared space periode sama yang memuat owner divisi. Tugas internal tidak terbuka ke shared otomatis.
- GET `/api/tasks/{taskId}`:200Task, pemeriksaan akses terkini.
- PATCH `/api/tasks/{taskId}`:TaskWriteRequest,200Task, ExpectedVersion wajib. OwnerDivisionID tidak dapat dipindahkan. Satu PJ aktif dari divisi pemilik. Perubahan PJ/deadline wajib Reason; perubahan ditulis ke riwayat/audit dan notifikasi pihak terkait dalam transaksi sama.
- POST `/api/tasks/{taskId}/move`:TaskMoveRequest,200Task. todo/doing/submitted/done/cancelled adalah progres; matching none/pending/verified/rejected terpisah. cancelled dan pembukaan kembali hasil terverifikasi hanya pengelola dengan alasan. Anggota hanya tugas miliknya (PJ/helper diterima), pengelola memakai scope owner divisi. Role Humas pada Medhum tidak mengelola tugas Mediatek. Urutan disimpan server; BeforeID bila ada harus readable, periode dan lane sama. Tidak boleh merusak tugas lain saat request ditolak. Rank internal, tidak diekspos DTO.
- POST `/api/tasks/{taskId}/comments`:TaskCommentRequest,201TaskComment, actor memiliki comments.write lewat salah satu space tugas yang berizin; arsip/admin administrasi bukan izin kerja.
- POST `/api/tasks/{taskId}/help`:TaskHelpCreateRequest,201TaskHelpRequest; satu pending per user/task; pembaca berizin help.request dapat menawarkan diri. PJ/helper yang sudah diterima tidak mengajukan ulang.
- POST `/api/tasks/{taskId}/help/{helpId}/decide`:TaskHelpDecisionRequest,200TaskHelpRequest; pengelola divisi pemilik; diterima menambah peserta, tidak mengganti PJ. Pastikan penerima masih boleh melihat tugas dan akun aktif.
- POST `/api/tasks/{taskId}/evidence`:TaskEvidenceRequest,201TaskEvidence. URL http/https atau catatan hasil wajib; tidak fetch URL server-side. Pemilik/pelaksana/pengelola yang berwenang. Progres submitted, matching pending; server mencatat waktu penerimaan UTC. Tidak ada status revisi desain.
- POST `/api/tasks/{taskId}/evidence/{evidenceId}/review`:TaskEvidenceReviewRequest,200TaskEvidence; results.review pada owner division. Cocok -> done/verified; tidak cocok -> progres sebelumnya (todo/doing),rejected. Alasan wajib. Bukti/waktu awal tidak dihapus, resubmit membuat bukti baru. Move done tidak boleh melewati pending/rejected matching; submitted tanpa bukti ditolak. UI drag ke Review membuka formulir bukti bila belum ada.
- GET `/api/tasks/{taskId}/history`:List[TaskEvent], pembaca tugas saja.
- GET `/api/notifications`:List[Notification] akun sendiri, filter ulang current task access sebelum mengungkap title/body. POST `/api/notifications/{notificationId}/read`:204 hanya penerima yang masih dapat membaca task.

## Invarian

Satu task ID untuk pribadi/divisi/Medhum. Metadata PJ/deadline/progres canonical, tidak disalin. Admin organisasi hanya administrasi, tidak mendapat tasks.manage atau hak privat otomatis. Periode closed hanya baca, arsip task hanya jika grant space yang tepat masih efektif. Hak tidak melekat karena pernah menjadi PJ/helper; setelah semua akses space dicabut, tugas tidak dapat dibaca/ditulis lewat ID/notifikasi/mine/history. Hak tambahan helper tidak mengelola PJ/deadline.

Gunakan period lock sebelum task row lock supaya seragam dengan close/membership/45. ExpectedVersion integer naik pada mutasi task/progres/bukti/anggota tambahan; konflik409, tidak silent overwrite. Rank maintenance tidak mengubah metadata/versi tugas lain. Member fields dari DTO hanya ID/nama, tanpa email. TaskWrite date-only dan waktu UTC eksklusif; tepat satu deadlineDate/deadlineAt, postingDate/postingAt opsional eksklusif. Jangan menganggap tanggal kalender punya jam. Deadline dan jadwal posting berbeda; belum ada auto aturan konten Medhum/H-2 dalam irisan CRUD ini, tidak dikurangi lagi oleh UI.

Notifikasi assignment, PJ/deadline, bantuan, bukti dibuat sebagai record in-app dalam transaksi; belum menjanjikan scheduler, push/suara OS/WA. Seluruh mutasi + audit dalam transaksi; ID UUIDv7 dan timestamp UTC. Migrasi baru0003 saja, tidak mengubah0001/0002. DB pengujian terisolasi, Mailer palsu; tidak menyentuh Reo, sheet, email nyata atau deploy produksi.
