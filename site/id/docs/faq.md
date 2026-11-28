---
title: Pertanyaan umum
description: Jawaban singkat untuk pertanyaan umum tentang Nefo, panel kontrol gratis untuk server Linux, tentang apakah Nefo gratis, lisensinya, sistem yang didukung, memori, basis data, Docker, memindahkan situs web, keamanan, privasi, pembaruan, pencopotan, dan bahasa.
---

# Pertanyaan umum

Jawaban singkat untuk pertanyaan yang paling sering diajukan tentang Nefo, panel kontrol gratis dan sumber terbuka untuk satu server Linux. Setiap jawaban dimulai dengan jawaban singkatnya, lalu memberikan detail dan tautan ke halaman yang membahasnya. Fakta di sini berlaku untuk Nefo v0.24.1.

## Apa itu Nefo? {#what-is-nefo}

Nefo adalah panel kontrol gratis dan sumber terbuka yang Anda instal di server Linux Anda sendiri (VPS) dengan satu perintah, untuk mengelola situs web, sertifikat SSL, basis data, aplikasi, file, cadangan, dan keamanan dari browser. Nefo adalah satu program, `/usr/local/bin/nefo`, dengan antarmuka web di dalamnya, dan berjalan sebagai layanan systemd `nefo`. Lihat [Apa itu Nefo?](./).

## Apakah Nefo gratis? {#is-nefo-free}

Ya. Nefo adalah perangkat lunak bebas di bawah GNU AGPLv3, dan setiap fitur gratis untuk semua orang, termasuk bisnis. Tidak ada edisi berbayar, fitur yang dikunci, kunci lisensi, atau akun yang harus dibuat. Lihat [Tentang](../about#licence).

## Nefo berjalan di sistem apa? {#what-does-nefo-run-on}

Nefo berjalan di Ubuntu 22.04, 24.04, dan 26.04 serta Debian 12 dan 13, di x86 64-bit (amd64) dan ARM 64-bit (arm64), dengan akses root dan systemd. Penginstal sudah diuji di server yang baru diinstal dengan setiap sistem ini di x86-64. Build arm64 belum diuji di perangkat keras ARM sungguhan, jadi coba dulu di server yang bisa Anda bangun ulang. Di sistem lain, penginstal berhenti kecuali Anda memaksanya. Lihat [Persyaratan](./requirements).

## Berapa RAM yang dibutuhkan Nefo? {#how-much-ram-does-nefo-need}

Panel sendiri memakai sekitar 34 MB memori, diukur di server uji setelah pembaruan ke v0.24.0, dan Nefo dibuat untuk VPS dengan RAM 1 GB. Apa yang Anda instal menambah angka itu, dan halaman **Perangkat lunak** menampilkan perkiraannya: Nginx sekitar 15 MB, setiap versi PHP sekitar 25 MB, MariaDB sebesar buffer pool-nya ditambah sekitar 100 MB (sekitar 228 MB di server 1 GB). Di server 1 GB, tambahkan swap 1 sampai 2 GB. Lihat [Persyaratan](./requirements#memory-and-disk).

## Apakah Nefo mendukung MySQL, PostgreSQL, dan Docker? {#does-nefo-support-mysql-postgresql-and-docker}

Ya. Nefo menjalankan MariaDB (versi distribusi Anda, atau 11.4, 11.8, atau 12.3 LTS) atau MySQL 8.4 LTS, satu per server, dan bisa beralih di antara keduanya beserta basis data Anda. PostgreSQL (versi distribusi Anda, atau 15 sampai 19 dari proyek PostgreSQL) berjalan di sampingnya. Docker menjalankan kontainer dan proyek Compose, serta aplikasi Docker dari Toko aplikasi. Lihat [Basis data](./databases), [PostgreSQL](./postgresql), dan [Docker](./docker).

## Server web apa yang bisa saya pakai? {#which-web-servers-can-i-use}

Nginx, Apache, atau OpenLiteSpeed, satu per server. Anda memilihnya saat menginstal server web:

- Nginx adalah yang paling ringan.
- Apache membaca file `.htaccess` yang disertakan sebagian aplikasi PHP.
- OpenLiteSpeed membaca aturan rewrite di file `.htaccess` dan sudah dilengkapi LiteSpeed Cache.

PHP berjalan di PHP-FPM di ketiganya, dengan satu pool per situs web. Situs web juga bisa berupa aplikasi Node.js, Python, atau Java, atau reverse proxy ke alamat lain. Lihat [Situs web](./websites).

## Bisakah saya beralih ke server web lain nanti? {#can-i-switch-to-another-web-server-later}

Bisa, tetapi hanya di server tanpa situs web. Panel menolak mencopot server web selama server web itu masih melayani situs web, karena pengaturan setiap situs web ditulis untuk server web tersebut. Hapus situs-situs webnya, copot server web di halaman **Perangkat lunak**, lalu instal yang lain. Cadangkan situs web Anda terlebih dahulu. Lihat [Perangkat lunak](./software#web-server).

## Bisakah saya memindahkan situs web dari cPanel atau aaPanel? {#can-i-move-my-websites-from-another-panel}

Bisa. Nefo mengimpor cadangan lengkap cPanel (situs web, basis data beserta pengguna dan kata sandinya, sertifikat, dan tugas cron), serta arsip situs web beserta dump SQL-nya dari aaPanel, HestiaCP, CyberPanel, atau server lain mana pun. Email, zona DNS, dan akun FTP tidak diimpor. Lihat [Mengimpor situs web](./import).

## Apakah Nefo aman? {#is-nefo-safe}

Nefo dibuat agar aman sejak awal, tetapi Nefo adalah perangkat lunak beta yang berjalan sebagai root dan belum pernah diaudit keamanannya oleh pihak luar. Panel hanya merespons di bawah path rahasia pada port acak, memakai HTTPS sejak pertama kali dijalankan, mendukung masuk dua faktor, memblokir sebuah alamat setelah 5 upaya masuk gagal dalam 15 menit, dan mencatat setiap perubahan. Rilis ditandatangani dengan Ed25519, dan [pemeriksaan keamanan](./security-check) bawaan mencantumkan pengaturan yang membuat server terbuka. Coba dulu di server yang bisa Anda bangun ulang, dan simpan cadangan di luar server. Lihat [Keamanan](./security) dan [kebijakan keamanan](../security-policy).

## Apakah Nefo mengirim data ke mana pun? {#does-nefo-send-data-anywhere}

Tidak ada telemetri, statistik penggunaan, atau pemeriksaan lisensi. Atas inisiatifnya sendiri, panel hanya menghubungi Let's Encrypt untuk menerbitkan dan memperpanjang sertifikat, dan, pada build rilis, mengunduh katalog toko aplikasi yang ditandatangani sekali sehari. Semua hal lain, seperti pemeriksaan pembaruan, peringatan, cadangan jarak jauh, dan basis data negara untuk pemblokiran negara, hanya terjadi saat Anda memintanya atau menyiapkannya. Lihat [Privasi](./#privacy).

## Bisakah Nefo mencadangkan ke Google Drive? {#can-nefo-back-up-to-google-drive}

Bisa, sejak v0.24.0, dengan OAuth client milik Anda sendiri yang hanya bisa melihat file yang dibuat panel. Nefo juga mencadangkan ke penyimpanan yang kompatibel dengan S3 (Amazon S3, Cloudflare R2, Backblaze B2, Wasabi, MinIO) dan server SFTP, bisa mengenkripsi setiap salinan jarak jauh dengan passphrase, dan server baru bisa menemukan cadangan di penyimpanan lalu memulihkannya. Lihat [Pencadangan](./backups#google-drive) dan [Memulihkan di server baru](./restore-new-server).

## Bagaimana cara memperbarui Nefo? {#how-do-i-update-nefo}

Buka **Pengaturan** → **Panel** dan klik **Periksa pembaruan**, atau jalankan `nefo update` sebagai root. Pembaruan memeriksa tanda tangan dan checksum rilis sebelum mengganti program, lalu memulai ulang panel; situs web tetap berjalan. Nefo tidak pernah memperbarui dirinya sendiri. Lihat [Pembaruan](./update).

## Bagaimana cara mencopot Nefo? {#how-do-i-uninstall-nefo}

Nefo tidak punya perintah pencopotan: Anda menghapusnya secara manual. Buka kunci situs web yang terkunci dengan `nefo unlock-files`, hentikan dan hapus layanan `nefo` serta `/usr/local/bin/nefo`, tutup port panel, lalu hapus `/etc/nefo` dan `/var/lib/nefo`. Situs web dan perangkat lunak Anda tetap berjalan kecuali Anda menghapusnya terlebih dahulu. Lihat [Mencopot Nefo](./uninstall).

## Bisakah Nefo meng-host email saya? {#can-nefo-host-my-e-mail}

Tidak. Nefo tidak punya server email: Nefo tidak menerima email atau menyediakan kotak surat. Situs web dan peringatan bisa mengirim email lewat relay SMTP di penyedia seperti Gmail, SendGrid, Mailgun, atau Amazon SES, dan halaman DNS bisa menambahkan record MX, SPF, dan DMARC penyedia email Anda. Gunakan penyedia email untuk kotak surat Anda. Lihat [Email dan peringatan](./mail-alerts).

## Bisakah saya mengelola beberapa server dari satu panel? {#can-i-manage-several-servers-from-one-panel}

Tidak. Instal Nefo di setiap server dan kelola masing-masing secara terpisah. Mengelola beberapa server dari satu panel tidak direncanakan: panel yang mengendalikan server lain membutuhkan agen dan hubungan saling percaya antarmesin. Lihat [peta jalan](../roadmap#not-planned-and-why).

## Bisakah saya memberi klien login mereka sendiri? {#can-i-give-my-clients-their-own-login}

Tidak. Setiap server punya satu akun panel, `admin`. Nefo tidak punya akun terpisah untuk pelanggan atau reseller. Agar klien bisa mengunggah file ke situs webnya, beri situs web itu akun SFTP, yang terkunci di folder situs web. Lihat [Situs web](./websites#sftp-access).

## Panel tersedia dalam bahasa apa saja? {#which-languages-does-the-panel-speak}

Tujuh bahasa, sejak v0.24.0: Inggris, Vietnam, Tionghoa (sederhana), Spanyol, Portugis, Rusia, dan Indonesia, termasuk teks Toko aplikasi. Panel mengikuti bahasa browser Anda, dan Anda bisa mengubahnya di halaman masuk atau di menu akun. Setiap bahasa adalah file terpisah yang hanya diunduh browser saat bahasa itu dipakai. Dokumentasi ini tersedia dalam tujuh bahasa yang sama.

Terjemahan sangat diterima. Salin `web/src/locales/en.json` ke file yang dinamai dengan kode bahasa Anda, terjemahkan nilainya, lalu daftarkan bahasanya di `web/src/i18n.js`. Lihat [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md).

## Bagaimana cara melaporkan masalah keamanan? {#how-do-i-report-a-security-problem}

Laporkan secara privat, bukan di issue publik. Di [repositori GitHub](https://github.com/nefocp/nefo), buka tab "Security" dan pilih "Report a vulnerability". Sertakan versi Anda (`nefo version`), sistem operasi Anda, dan langkah-langkah untuk mereproduksi masalahnya. Lihat [kebijakan keamanan](../security-policy), yang mengikuti [SECURITY.md](https://github.com/nefocp/nefo/blob/main/SECURITY.md).

## Apakah ada catatan perubahan dan peta jalan? {#is-there-a-changelog-and-a-roadmap}

Ada. [Catatan perubahan](../changelog) mencantumkan apa yang berubah di setiap rilis, yang terbaru lebih dulu, dan apa yang perlu diketahui sebelum Anda memperbarui. [Peta jalan](../roadmap) mencantumkan apa yang sedang dikerjakan sebelum versi 1.0, apa yang sedang dipertimbangkan, dan apa yang tidak direncanakan beserta alasannya. Untuk memeriksa sendiri tanda tangan sebuah rilis sebelum Anda menginstalnya, lihat [Memeriksa rilis sendiri](../security-policy#check-a-release-yourself).

## Bagaimana saya bisa membantu proyek ini? {#how-can-i-support-the-project}

- Laporkan bug beserta langkah-langkah untuk mereproduksinya, sistem operasi Anda, dan `nefo version`.
- Terjemahkan panel ke bahasa Anda, atau perbaiki terjemahan yang ada.
- Perbaiki issue dan kirim pull request (lihat [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md)).
- Beri bintang proyek ini di [GitHub](https://github.com/nefocp/nefo) dan ceritakan kepada orang lain.
