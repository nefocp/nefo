---
title: Peta jalan
description: Apa yang sedang dikerjakan sebelum Nefo 1.0, apa yang sedang dipertimbangkan, dan apa yang tidak direncanakan beserta alasannya.
---

# Peta jalan

Peta jalan Nefo tidak memuat tanggal: sebuah item dirilis saat sudah berfungsi. Sebelum versi 1.0, pekerjaan difokuskan untuk membuat apa yang sudah ada bisa diandalkan: pengujian di perangkat keras ARM64 sungguhan dan dengan layanan sungguhan, server uji yang berjalan dalam waktu lama, laporan dari pengguna luar, dan tinjauan keamanan dari pihak luar. Beberapa ide sedang dipertimbangkan, dan beberapa hal sengaja tidak direncanakan.

Halaman ini mencantumkan apa yang sedang dikerjakan, apa yang sedang dipertimbangkan, dan apa yang tidak akan dilakukan Nefo. Untuk melihat apa yang sudah dirilis, baca [catatan perubahan](./changelog).

## Sekarang: menuju 1.0 {#now-toward-1-0}

Nefo masih beta. Sebelum versi 1.0, pekerjaan lebih difokuskan untuk membuat apa yang sudah ada bisa diandalkan daripada menambah fitur baru:

- **Menguji penginstal di ARM64.** Penginstal, dengan dan tanpa penyiapan perangkat lunak, sudah diuji di server Ubuntu 22.04, 24.04, dan 26.04 serta Debian 12 dan 13 yang baru diinstal, di x86-64. Build ARM64 sudah dirilis tetapi belum diuji di perangkat keras ARM sungguhan.
- **Menguji dengan layanan sungguhan.** Pencadangan ke penyimpanan yang kompatibel dengan S3 (seperti AWS S3 dan Cloudflare R2) dan Google Drive, peringatan lewat Telegram, Discord, Slack, dan webhook, email melalui penyedia SMTP, dan API Cloudflare. Sebagian dari ini sejauh ini baru diuji dengan tiruan: Google Drive, misalnya, baru diuji dengan layanan Google yang disimulasikan.
- **Menjalankan server uji untuk waktu yang lama,** dengan situs web sungguhan, agar masalah yang baru muncul setelah berminggu-minggu (log yang terus membesar, pemakaian disk, memori) ditemukan sebelum Anda menemukannya.
- **Pengguna dari luar dan laporan bug mereka.** Setiap laporan dengan langkah-langkah untuk mereproduksi masalah sangat membantu. Lihat [Melaporkan bug](./about#get-help-and-report-bugs).
- **Tinjauan keamanan dari pihak luar.** Belum ada audit dari pihak luar. Sampai ada, perlakukan Nefo sebagai perangkat lunak beta yang berjalan sebagai root, dan lihat [kebijakan keamanan](./security-policy).

Sampai versi 1.0, hanya rilis terbaru yang menerima perbaikan keamanan.

## Sedang dipertimbangkan {#being-considered}

Ini adalah ide, bukan janji. Sebagian mungkin berubah, dan sebagian mungkin tidak pernah dirilis.

- **Pencadangan ke Google Drive tanpa OAuth client milik Anda sendiri.** Saat ini setiap server membutuhkan OAuth client yang Anda buat sekali di proyek Google Cloud (lihat [Google Drive](./docs/backups#google-drive)).
- **Alamat asli pengunjung di balik proxy Cloudflare,** di log situs web dan PHP di Nginx, sebagai pengaturan per situs web. Saat ini situs web Nginx di balik proxy melihat alamat Cloudflare di sana; pemblokiran negara sudah mencari alamat pengunjung.
- **Antarmuka panel yang lebih tenang.**

Panel tersedia dalam tujuh bahasa. Terjemahan ke bahasa lain sangat diterima: lihat [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md).

## Tidak direncanakan, dan alasannya {#not-planned-and-why}

Nefo dimaksudkan untuk tetap cukup ringan bagi VPS dengan RAM 1 GB, dan cukup kecil agar kodenya bisa diperiksa keamanannya. Setiap item di bawah ini akan menambah layanan yang berjalan terus-menerus, membuka port ke internet, atau menambah banyak kode yang berjalan sebagai root. Jadi item-item ini sengaja tidak disertakan, dan halaman ini menyebutkan apa yang ditawarkan Nefo sebagai gantinya.

| Tidak direncanakan | Alasan | Yang ditawarkan Nefo sebagai gantinya |
|---|---|---|
| Server email | Email membutuhkan beberapa layanan yang berjalan terus-menerus, penyaringan spam dan virus yang memakan banyak memori, dan perawatan terus-menerus atas reputasi server. Banyak penyedia juga memblokir port 25 keluar. | Relay SMTP melalui penyedia email Anda, untuk situs web dan peringatan. Gunakan penyedia email untuk kotak surat. Lihat [Email dan peringatan](./docs/mail-alerts). |
| Server DNS | Name server harus merespons dari internet setiap saat, dan sebaiknya dijalankan di lebih dari satu mesin. | Record DNS zona Cloudflare Anda, yang dikelola dari panel, dan sertifikat wildcard lewat DNS Cloudflare. Lihat [DNS (Cloudflare)](./docs/dns). |
| Server FTP | Satu layanan lagi dengan port terbuka dan akunnya sendiri. | Akun SFTP per situs web, yang terkunci di folder situs web, dengan kata sandi atau kunci SSH. Lihat [Akses SFTP](./docs/websites#sftp-access). |
| Mengelola beberapa server dari satu panel | Panel yang mengendalikan server lain membutuhkan agen dan hubungan saling percaya antarmesin. Pembobolan di satu server akan menjangkau semuanya. | Satu Nefo per server. Kunci API memungkinkan skrip Anda sendiri bekerja dengan setiap server. Lihat [Kunci API](./docs/api). |
| Akun reseller dan pelanggan hosting | Akun dengan batas, paket, dan kuotanya sendiri adalah produk yang berbeda, untuk menjual hosting. Nefo dibuat untuk orang yang mengelola server. | Satu akun administrator. Agar klien bisa mengunggah file, beri situs webnya akun SFTP. |

Jika Anda membutuhkan salah satu dari ini, panel lain mungkin lebih cocok untuk Anda: lihat [Nefo dan panel lain](./compare#when-another-panel-fits-you-better).
