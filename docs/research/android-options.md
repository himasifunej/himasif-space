# Pilihan Android untuk HIMASIF

Riset: 27 Oktober 2026. Konteks: backend Go, desktop MyGo dengan frontend web, akses web cadangan, Baileys di server, Android tahap berikutnya. Pemilik sedang belajar Flutter. Repo belum memiliki konvensi dokumentasi atau AGENTS.md yang ditemukan.

## Rekomendasi

**Dengan prioritas terbaru berupa aplikasi mobile lintas platform dan melanjutkan belajar Flutter, rekomendasi saya adalah Flutter untuk klien Android, lalu iOS bila diperlukan.** Capacitor tetap paling hemat duplikasi UI jika prioritas utamanya memakai ulang frontend web/MyGo. Ini penilaian kebutuhan proyek, bukan klaim Flutter selalu paling ringan. Backend dan aturan bisnis tetap di Go; mobile mengakses API yang sama. UI Flutter terpisah dari frontend web desktop.

| Pilihan | Dasar teknis | Implikasi untuk HIMASIF (penilaian) |
|---|---|---|
| Capacitor | Runtime mobile untuk aplikasi web, bisa ditambahkan pada proyek JavaScript yang ada; Android memakai WebView dan menyediakan jembatan Java/Kotlin. [Pengantar](https://capacitorjs.com/docs), [Android](https://capacitorjs.com/docs/android) | Paling sedikit duplikasi UI dengan desktop/web. Tetap perlu layout mobile, navigasi Android, penyimpanan sesi, dan integrasi notifikasi. |
| Flutter | Toolkit UI Dart lintas platform; hot reload saat pengembangan, kode mesin untuk release mobile, dengan engine/runtime sendiri. [Arsitektur](https://docs.flutter.dev/resources/architectural-overview) | Cocok untuk meneruskan belajar Flutter dan membuat UI mobile khusus. Frontend HTML/CSS/JS MyGo tidak langsung dipakai ulang sebagai widget Flutter. |
| Kotlin + Jetpack Compose | Toolkit UI native Android yang direkomendasikan Android, dengan API Kotlin dan akses platform. [Compose](https://developer.android.com/compose) | Cocok jika Android native menjadi fokus utama. Menambah jalur pembelajaran serta UI tersendiri bagi proyek ini. |

## Apa arti cepat dan ringan?

- **Cepat dibuat:** Capacitor unggul secara kontekstual karena frontend web memang sudah direncanakan. Flutter bisa lebih sesuai jika pengembang ingin menginvestasikan waktu belajar Dart/Flutter. Tidak ada dasar untuk menjanjikan waktu implementasi tertentu.
- **Ukuran unduhan/instalasi:** belum ada angka yang sah untuk aplikasi HIMASIF. Flutter menyertakan kode dan aset yang diperlukan; debug build bukan ukuran produksi, dan paket unggahan Play juga berbeda dari unduhan per perangkat. Ukur release yang benar sebelum membandingkan. [Pengukuran Flutter](https://docs.flutter.dev/perf/app-size)
- **RAM dan kelancaran:** WebView pada Capacitor dan engine Flutter memiliki karakter berbeda, tetapi dokumentasi tersebut tidak membuktikan pemenang untuk beban HIMASIF. Hindari klaim “pasti lebih ringan”. Uji layar tugas, notes panjang, daftar konten, dan notifikasi pada HP sasaran yang sama.
- **Baterai:** gunakan penjadwal server dan push untuk ping; jangan mengandalkan polling/JavaScript yang selalu berjalan. Android membatasi aktivitas latar belakang dan pembatasan bisa berbeda menurut produsen. [Batas background Android](https://developer.android.com/develop/background-work/background-tasks/bg-work-restrictions)

## Notifikasi dan batasnya

Capacitor memiliki plugin push resmi yang memakai Firebase Cloud Messaging di Android. Flutter didukung oleh Firebase Messaging. Keduanya tetap memerlukan integrasi token perangkat dan pengiriman push dari server. [Capacitor Push](https://capacitorjs.com/docs/apis/push-notifications), [FCM Flutter](https://firebase.google.com/docs/cloud-messaging/flutter/receive-messages)

Android 13+ membutuhkan izin notifikasi. Pada Flutter/FCM, aplikasi yang di-force-stop melalui pengaturan Android harus dibuka lagi sebelum pesan bekerja. Capacitor menjelaskan bahwa callback JavaScript untuk data-only push tidak dipanggil saat aplikasi dimatikan; pemrosesan itu memerlukan layanan native. Maka untuk reminder terlihat, rancang notification payload dan buka tugas lewat aksi notifikasi; jangan bergantung pada JavaScript background. Pengiriman tidak boleh dijanjikan selalu tepat waktu atau lolos semua pengaturan perangkat. [Izin Android](https://developer.android.com/develop/ui/compose/notifications/notification-permission), [FCM Flutter](https://firebase.google.com/docs/cloud-messaging/flutter/receive-messages), [batas plugin Capacitor](https://capacitorjs.com/docs/apis/push-notifications)

## Bukti riset dan pengalaman komunitas

Studi EASE 2023 membandingkan Flutter, React Native, Ionic, dan native Java pada sepuluh benchmark komputasi serta dua aplikasi. Flutter umumnya memiliki overhead waktu eksekusi dan energi terendah di antara framework yang diuji, tetapi hasil berubah menurut workload; contoh animasi menunjukkan pemenang berbeda. Studi tersebut bukan perbandingan versi terbaru Capacitor atau Compose dan tidak membuktikan framework terbaik untuk semua aplikasi. Karena itu Flutter memiliki dukungan empiris sebagai pilihan yang masuk akal, tetapi tidak dapat dijanjikan paling hemat RAM/baterai untuk HIMASIF. [Publikasi dan abstrak penulis](https://research.utwente.nl/en/publications/analyzing-the-resource-usage-overhead-of-mobile-app-development-f/), [paper](https://dspace.library.uu.nl/bitstream/handle/1874/431606/3593434.3593487.pdf?isAllowed=y&sequence=1)

Diskusi komunitas memperlihatkan dua pertimbangan yang berulang: Capacitor menarik untuk berbagi UI/keahlian web, sedangkan Flutter dipertimbangkan untuk pengalaman mobile dan navigasi. Ada pengalaman positif maupun masalah pada keduanya. Ini anekdot dengan bias pemilihan, bukan pengukuran atau konsensus teknis. [Diskusi task tracker Flutter/Capacitor](https://www.reddit.com/r/FlutterDev/comments/1eu5b9e), [diskusi navigasi pada repo Capacitor](https://github.com/ionic-team/capacitor/discussions/5695), [pengalaman Capacitor](https://www.reddit.com/r/webdev/comments/1cb2bj2)

Keputusan bisa dibuat dari bukti di atas tanpa mewajibkan prototype pembanding. Pengujian aplikasi saat implementasi tetap diperlukan, tetapi bukan syarat untuk memilih framework sekarang.

## Flutter tanpa Mac atau Xcode

**Android bisa dikembangkan, dijalankan, dan dibangun dari Windows atau Linux**, memakai Flutter SDK, Android SDK/tooling, serta perangkat Android atau emulator. Tidak memerlukan Xcode untuk target Android. [Setup Android resmi](https://docs.flutter.dev/platform-integration/android/setup)

**Target iOS memerlukan toolchain Xcode pada macOS** untuk build, simulator, dan penandatanganan sesuai alur distribusi. Menulis kode Dart di Windows/Linux tetap bisa, tetapi tidak menghilangkan kebutuhan lingkungan macOS untuk langkah iOS. [Setup iOS resmi](https://docs.flutter.dev/platform-integration/ios/setup)

Tidak harus memiliki Mac pribadi: mesin macOS jarak jauh atau CI macOS seperti Codemagic dapat menjalankan build iOS dengan Xcode. Codemagic mendokumentasikan workflow Flutter iOS dan mesin macOS. Ini memindahkan tempat build, bukan membuat Xcode bisa dijalankan di Windows/Linux; debugging interaktif dan pengujian perangkat iOS tetap perlu direncanakan. Akun, signing, dan distribusi Apple juga tetap berlaku. Tidak ada janji layanan CI gratis tanpa batas dalam rekomendasi ini. [Codemagic Flutter](https://docs.codemagic.io/yaml-quick-start/building-a-flutter-app/), [konfigurasi runner](https://docs.codemagic.io/yaml-basic-configuration/yaml-getting-started/)
