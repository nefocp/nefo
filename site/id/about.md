---
title: Tentang Nefo
description: Apa itu Nefo, untuk siapa Nefo dibuat, prinsip dasarnya, status dan lisensinya, serta cara mendapatkan bantuan atau melaporkan bug.
---

# Tentang Nefo

Nefo adalah panel kontrol gratis dan sumber terbuka untuk satu server Linux. Anda menginstalnya di VPS Anda sendiri dengan satu perintah, lalu mengelola situs web, sertifikat, basis data, aplikasi, file, cadangan, dan keamanan dari browser, dalam salah satu dari tujuh bahasa. Nefo masih beta, berlisensi GNU AGPLv3, dan dibuat untuk VPS dengan RAM 1 GB.

Nefo adalah satu program, `/usr/local/bin/nefo`, dengan antarmuka web di dalamnya, dan berjalan sebagai layanan systemd `nefo`. Nefo berjalan di Ubuntu 22.04, 24.04, dan 26.04 serta Debian 12 dan 13, di x86-64 dan ARM64. Toko aplikasinya berisi 42 aplikasi, dan antarmukanya tersedia dalam bahasa Inggris, Vietnam, Tionghoa, Spanyol, Portugis, Rusia, dan Indonesia. Lihat [Apa itu Nefo?](./docs/) untuk semua yang bisa dilakukannya.

## Untuk siapa Nefo dibuat {#who-it-is-for}

Nefo dibuat untuk orang yang mengelola VPS sendiri: seseorang yang menyiapkan server pertamanya untuk blog, developer yang men-deploy aplikasi, atau freelancer maupun agensi yang mengurus situs web klien.

Setiap server punya satu akun panel, `admin`. Nefo tidak dibuat untuk menjual hosting: tidak ada akun reseller atau pelanggan. Lihat [Nefo dan panel lain](./compare) untuk mengetahui kapan panel lain lebih cocok untuk Anda.

## Prinsip {#principles}

- **Ringan.** Nefo dibuat untuk VPS dengan RAM 1 GB, dan panelnya sendiri memakai memori sekitar {{ $release.ram }} (diukur di server uji yang menjalankan v{{ $release.version }}). Nefo tidak menginstal apa pun sampai Anda memilihnya, setiap komponen bisa dicopot lagi, dan fitur yang akan membuat layanan besar terus berjalan, seperti server email, sengaja tidak disertakan.
- **Semua fitur gratis.** Tidak ada edisi berbayar dan tidak ada fitur yang dikunci. Anda tidak perlu membuat akun, dan tidak ada kunci lisensi.
- **Tidak ada yang mengirim data ke luar.** Tanpa telemetri, tanpa statistik penggunaan, tanpa pemeriksaan lisensi. Antarmuka tidak memuat apa pun dari situs lain. Atas inisiatifnya sendiri, panel hanya menghubungi Let's Encrypt untuk sertifikat dan rilis GitHub milik proyek untuk katalog toko aplikasi yang ditandatangani, dan hanya memeriksa pembaruan saat Anda memintanya. Lihat [Privasi](./docs/#privacy).
- **Jujur tentang keterbatasannya.** Dokumentasi menyebutkan apa yang tidak dilakukan Nefo, serta apa yang ditolaknya dan mengapa. [Peta jalan](./roadmap) mencantumkan apa yang tidak direncanakan sejelas apa yang direncanakan.

## Status {#status}

Nefo masih beta. Rilis publik pertama, v0.20.0, keluar pada 26 November 2026, dan rilis saat ini adalah v{{ $release.version }} ({{ $release.date }}). Lihat [catatan perubahan](./changelog).

- Nomor rilis diawali `0.`. Sampai versi 1.0, hanya rilis terbaru yang menerima perbaikan keamanan.
- Belum ada audit keamanan dari pihak luar. Lihat [kebijakan keamanan](./security-policy).
- Penginstal sudah diuji di server Ubuntu 22.04, 24.04, dan 26.04 serta Debian 12 dan 13 yang baru diinstal, di x86-64. ARM64 belum diuji di perangkat keras ARM sungguhan.

Coba Nefo di server yang bisa Anda bangun ulang sebelum mengandalkannya untuk situs web penting, dan simpan salinan cadangan Anda di luar server.

## Lisensi {#licence}

Nefo adalah perangkat lunak bebas di bawah [GNU AGPLv3](https://github.com/nefocp/nefo/blob/main/LICENSE). Anda bisa memakainya untuk tujuan apa pun, di server sebanyak yang Anda mau, serta membaca, mengubah, dan membagikan kodenya. Jika Anda menawarkan Nefo yang sudah dimodifikasi kepada orang lain lewat jaringan, AGPL mewajibkan Anda memublikasikan perubahan Anda.

Kode sumbernya ada di [GitHub](https://github.com/nefocp/nefo).

## Mendapatkan bantuan dan melaporkan bug {#get-help-and-report-bugs}

- **Baca dokumentasi terlebih dahulu.** [Pemecahan masalah](./docs/troubleshooting) membahas masalah yang umum, dan [Pertanyaan umum](./docs/faq) membahas pertanyaan yang umum.
- **Laporkan bug** di [GitHub Issues](https://github.com/nefocp/nefo/issues). Sertakan versi Anda (`nefo version`), sistem operasi Anda, langkah-langkah yang menyebabkan masalah, apa yang Anda harapkan, dan apa yang terjadi. Jika panel itu sendiri yang gagal, tambahkan bagian akhir log-nya: `journalctl -u nefo -n 50 --no-pager`.
- **Jangan sertakan rahasia** dalam apa pun yang Anda posting: path rahasia panel, kata sandi, kunci API, dan nama domain privat.
- **Laporkan masalah keamanan secara privat,** jangan pernah di issue publik. Lihat [kebijakan keamanan](./security-policy#report-a-vulnerability).

## Berkontribusi {#contribute}

- **Laporkan bug** seperti dijelaskan di atas. Laporan dari server sungguhan adalah hal yang paling dibutuhkan Nefo sebelum 1.0.
- **Terjemahkan panel.** Salin `web/src/locales/en.json` ke file yang dinamai sesuai kode bahasa Anda, terjemahkan nilainya (bukan kuncinya), dan daftarkan bahasanya di `web/src/i18n.js`.
- **Perbaiki issue dan kirim pull request.** Baca [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md) terlebih dahulu. Nefo berjalan sebagai root di server penggunanya, jadi setiap perubahan ditinjau dengan memperhatikan keamanan, dan aturan keamanan untuk kode di sana tidak bersifat opsional. Untuk perubahan yang lebih besar, buka issue terlebih dahulu untuk menyepakati pendekatannya.
- **Tandatangani CLA.** Kontributor menandatangani [Contributor Licence Agreement](https://github.com/nefocp/nefo/blob/main/CLA.md) sekali, saat membuka pull request pertama mereka. Anda tetap memegang hak cipta atas kontribusi Anda.
