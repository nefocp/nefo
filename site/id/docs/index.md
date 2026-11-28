---
title: Apa itu Nefo?
description: Nefo adalah panel kontrol gratis dan sumber terbuka yang Anda instal di server Linux Anda sendiri untuk menjalankan situs web, basis data, aplikasi, pencadangan, dan keamanan dari browser.
---

# Apa itu Nefo?

Nefo adalah panel kontrol gratis dan sumber terbuka (GNU AGPLv3) yang Anda instal di server Linux Anda sendiri dengan satu perintah. Dari browser, dalam salah satu dari tujuh bahasa, Anda mengelola situs web beserta sertifikat SSL-nya, basis data, aplikasi, file, cadangan, dan keamanan. Nefo adalah satu program, dibuat untuk VPS dengan RAM 1 GB, dan tidak mengirim telemetri.

Halaman ini menjelaskan apa yang dilakukan Nefo, untuk siapa Nefo dibuat, apa yang sengaja tidak disertakan, dan apa yang dikirimnya lewat jaringan.

## Panel kontrol dalam satu program {#a-control-panel-in-one-program}

Anda menginstal Nefo di VPS dengan satu perintah. Nefo adalah satu program, `/usr/local/bin/nefo`, dengan antarmuka web di dalamnya, dan berjalan sebagai layanan systemd `nefo`. Setelah itu Anda mengelola server dari browser: situs web, sertifikat, basis data, file, cadangan, perangkat lunak, dan keamanan.

- **Semua fitur gratis.** Tidak ada paket berbayar dan tidak ada biaya per situs atau per akun.
- **Tanpa akun dan tanpa server lisensi.** Tidak ada yang perlu didaftarkan atau diaktivasi.
- **Dibuat untuk server kecil.** Nefo dirancang untuk berjalan di VPS dengan RAM 1 GB, di x86-64 dan ARM64. Panelnya sendiri memakai sekitar 34 MB memori, diukur di server uji setelah pembaruan ke v0.24.0.
- **Tujuh bahasa antarmuka:** Inggris, Vietnam, Tionghoa, Spanyol, Portugis, Rusia, dan Indonesia. Anda memilih bahasa di halaman masuk atau di menu akun, dan browser hanya mengunduh bahasa yang Anda pakai.

Untuk memulai, periksa [persyaratan](./requirements), lalu [instal Nefo](./install) dan ikuti [langkah pertama](./first-steps).

## Untuk siapa Nefo dibuat {#who-it-is-for}

Nefo dibuat untuk orang yang mengelola VPS sendiri: seseorang yang menyiapkan server pertamanya untuk blog, developer yang men-deploy aplikasi, atau freelancer maupun agensi yang mengurus situs web klien.

Setiap server punya satu akun panel, `admin`. Nefo tidak punya login terpisah untuk pelanggan atau reseller.

## Apa yang bisa Anda lakukan dengannya {#what-you-can-do-with-it}

### Situs web {#websites}

- Jalankan situs web PHP, statis, dan aplikasi (Node.js, Python, atau Java) di Nginx, Apache, atau OpenLiteSpeed, serta situs web reverse proxy yang meneruskan ke alamat lain. Setiap situs web mendapat pengguna Linux sendiri, dan bisa juga akun SFTP yang terkunci di foldernya. Kloning situs web ke domain baru, dan hapus situs web dengan mencadangkannya lebih dulu. Lihat [Situs web](./websites).
- Aktifkan HTTPS dengan sertifikat Let's Encrypt gratis yang diperpanjang otomatis, sertifikat wildcard lewat DNS Cloudflare, atau sertifikat Anda sendiri. Lihat [Sertifikat SSL](./ssl).
- Blokir atau izinkan pengunjung berdasarkan negara, dan kunci kode situs web agar hanya bisa dibaca oleh PHP-nya. Lihat [Pemblokiran negara dan penguncian file](./website-protection).
- Deploy dari GitHub, GitLab, Gitea, atau server Git apa pun, sesuai permintaan atau pada setiap push. Lihat [Deploy dari Git](./git-deploy).
- Jaga situs WordPress tetap mutakhir, dengan cadangan basis data sebelum setiap pembaruan, mode pemeliharaan, dan salinan staging. Lihat [Toolkit WordPress](./wordpress).
- Instal 42 aplikasi dengan satu klik: aplikasi PHP seperti WordPress, Joomla, Nextcloud, dan Laravel, serta aplikasi Docker seperti Ghost, Uptime Kuma, n8n, Gitea, Grafana, dan Jellyfin. Lihat [Toko aplikasi](./apps).

### Data {#data}

- Jalankan MariaDB (versi distribusi Anda, atau 11.4, 11.8, atau 12.3 LTS) atau MySQL 8.4 yang disetel untuk server kecil, dengan basis data, pengguna, impor dan ekspor SQL, serta akses jarak jauh per basis data, dan beralih di antara keduanya beserta basis data Anda. Adminer dan phpMyAdmin berada di balik halaman masuk panel. Telusuri kunci Redis, dan tambahkan Memcached. Lihat [Basis data](./databases).
- Jalankan PostgreSQL, versi distribusi Anda atau versi 15 sampai 19 dari proyek PostgreSQL, dengan basis data dan pemiliknya sendiri, akses jarak jauh lewat TLS, dan peningkatan versi mayor. Lihat [PostgreSQL](./postgresql).
- Impor situs web dari cadangan lengkap cPanel, atau dari arsip situs dan dump SQL yang dibuat oleh aaPanel, HestiaCP, CyberPanel, atau server apa pun. Lihat [Mengimpor situs web](./import).
- Kelola file dengan editor, unggah dan unduh, arsip, dan tempat sampah yang menyimpan file terhapus selama 30 hari. Lihat [File](./files).
- Cadangkan situs web, basis data, folder, dan aplikasi sesuai jadwal, ke server ini, ke penyimpanan yang kompatibel dengan S3, ke server SFTP, atau ke Google Drive, dienkripsi dengan passphrase jika Anda mau, lalu pulihkan, juga di server baru. Lihat [Pencadangan](./backups) dan [Memulihkan di server baru](./restore-new-server).

### Server {#server}

- Instal dan copot setiap komponen (server web, versi PHP dan ionCube Loader, MariaDB atau MySQL, PostgreSQL, Redis, Node.js, Python, Java, Docker, dan lainnya). Nefo menolak menghapus komponen yang masih dipakai situs web, basis data, atau kontainer. Lihat [Perangkat lunak](./software).
- Jalankan kontainer Docker dan proyek Compose. Nefo memeriksa file Compose untuk pengaturan yang tidak aman, seperti kontainer privileged atau port yang terbuka ke internet. Lihat [Docker](./docker).
- Kendalikan firewall (ufw), fail2ban, pengaturan SSH, firewall aplikasi web per situs web, dan pemindai malware. Lihat [Keamanan](./security).
- Jalankan pemeriksaan keamanan: 22 pemeriksaan cepat atas server, masing-masing dengan tautan ke perbaikannya, sesuai permintaan atau setiap hari. Lihat [Pemeriksaan keamanan](./security-check).
- Gunakan terminal di browser, dan kelola layanan, proses, program latar belakang, serta tugas cron. Lihat [Alat sistem](./system).
- Pantau apakah setiap situs web merespons, lengkap dengan uptime dan waktu responsnya. Lihat [Pemantauan](./websites#monitoring).
- Biarkan situs web mengirim email lewat relay SMTP, dan terima peringatan lewat Telegram, email, Discord, Slack, atau webhook, juga saat seseorang masuk. Lihat [Email dan peringatan](./mail-alerts).
- Kelola record DNS zona Cloudflare Anda, arahkan domain situs web ke server ini, dan tambahkan record MX, SPF, dan DMARC dari penyedia email Anda. Lihat [DNS (Cloudflare)](./dns).

## Keamanan panel itu sendiri {#security-of-the-panel-itself}

Nefo berjalan sebagai root, jadi ia melindungi halaman masuknya sendiri:

- Panel hanya merespons di bawah path rahasia seperti `/k3x9p2qa7m1z/`. Setiap alamat lain di port-nya mengembalikan `404`.
- Panel memakai HTTPS sejak pertama kali dijalankan. Awalnya sertifikatnya ditandatangani sendiri (self-signed), dan Anda bisa menggantinya dengan sertifikat gratis untuk domain panel sendiri.
- Proses masuk mendukung autentikasi dua faktor. Setelah 5 kali gagal dalam 15 menit, sebuah alamat diblokir selama 15 menit.
- Setiap proses masuk dan perubahan dicatat di log aktivitas, yang menyimpan entri selama 90 hari.

## Apa yang tidak dilakukan Nefo {#what-nefo-doesn-t-do}

- **Tidak ada server email.** Nefo tidak menerima email dan tidak menyediakan kotak surat. Situs web dan peringatan mengirim email lewat relay SMTP di penyedia email. Gunakan penyedia email untuk kotak surat Anda.
- **Tidak ada server DNS atau FTP.** Nefo mengelola record DNS domain Anda di Cloudflare, dan setiap situs web bisa punya akun SFTP sebagai pengganti FTP.
- **Satu server per panel.** Setiap server menjalankan Nefo-nya sendiri. Mengelola beberapa server dari satu panel tidak direncanakan (lihat [peta jalan](../roadmap#not-planned-and-why)).
- **Tidak ada akun untuk pelanggan.** Hanya ada satu akun administrator per server.
- **Tidak ada akun email, DNS, atau FTP saat mengimpor.** Impor dari cPanel membawa situs web, basis data, sertifikat, dan tugas cron. Lihat [Apa yang tidak diimpor](./import#what-is-not-imported).
- **Hanya Ubuntu dan Debian.** Ubuntu 22.04, 24.04, dan 26.04, serta Debian 12 dan 13. Lihat [Persyaratan](./requirements).

## Status {#status}

Nefo masih perangkat lunak muda. Nomor rilisnya diawali `0.`, dan sampai versi 1.0 hanya rilis terbaru yang menerima perbaikan keamanan. Anda bisa melihat versi Anda di **Pengaturan** → **Panel**, atau dengan `nefo version` di server.

Penginstal sudah diuji di server Ubuntu 22.04, 24.04, dan 26.04 serta Debian 12 dan 13 yang baru diinstal, di x86-64. Versi ARM64 belum diuji di perangkat keras ARM sungguhan. Belum ada audit keamanan dari pihak luar.

Coba Nefo di server yang bisa Anda bangun ulang sebelum mengandalkannya untuk situs web penting, dan simpan salinan cadangan Anda di luar server. Laporkan masalah di [repositori GitHub](https://github.com/nefocp/nefo).

[Catatan perubahan](../changelog) mencantumkan apa yang berubah di setiap rilis, [peta jalan](../roadmap) mencantumkan apa yang sedang dikerjakan dan apa yang tidak direncanakan, dan [kebijakan keamanan](../security-policy) menjelaskan cara melaporkan kerentanan dan memeriksa sendiri sebuah rilis.

## Lisensi {#licence}

Nefo adalah perangkat lunak bebas di bawah GNU AGPLv3. Anda bisa memakainya untuk tujuan apa pun, termasuk penggunaan komersial. Jika Anda menawarkan Nefo yang sudah dimodifikasi kepada orang lain lewat jaringan, AGPL mewajibkan Anda memublikasikan perubahan Anda.

## Privasi {#privacy}

Nefo tidak punya telemetri, statistik penggunaan, atau pemeriksaan lisensi. Antarmuka web tidak memuat apa pun dari situs lain: font, ikon, dan skripnya sudah tertanam di dalam program.

Atas inisiatifnya sendiri, panel hanya terhubung ke server lain untuk tugas-tugas berikut:

| Kapan | Ke mana | Mengapa |
|---|---|---|
| Saat sertifikat jatuh tempo (diperiksa setiap jam) | Let's Encrypt | Meminta atau memperpanjang sertifikat situs web Anda dan sertifikat panel. Akun Let's Encrypt milik panel tidak punya alamat email. Kecuali Anda memakai validasi DNS, panel lebih dulu mengambil file uji dari domain Anda lewat HTTP, untuk memeriksa bahwa domain tersebut mengarah ke server ini. |
| Sekali sehari, pada build rilis | Rilis GitHub milik proyek | Mengunduh katalog toko aplikasi yang ditandatangani. Katalog tanpa tanda tangan yang sah dari proyek akan diabaikan. |

Semua hal lain hanya terjadi karena Anda memintanya:

- **Pembaruan:** Nefo memeriksa versi baru hanya saat Anda mengklik **Periksa pembaruan** atau menjalankan `nefo update`. Nefo tidak pernah memperbarui dirinya sendiri. Lihat [Pembaruan](./update).
- **Hal yang Anda siapkan:** peringatan (Telegram, server SMTP Anda, Discord, Slack, atau webhook Anda), pemantauan situs web (situs web Anda, lewat domainnya, seperti pengunjung), cadangan jarak jauh (penyimpanan S3 atau SFTP Anda, atau Google Drive), validasi DNS dan halaman DNS (API Cloudflare), relay email (penyedia email Anda), dan pemblokiran negara, yang mengunduh basis data negara DB-IP dan daftar alamat Cloudflare sebulan sekali selama ada situs web yang memakainya.
- **Perangkat lunak yang Anda instal:** paket berasal dari distribusi Anda lewat apt, dan beberapa komponen berasal dari proyeknya masing-masing, seperti Node.js dari nodejs.org, versi PHP tambahan dari repositori packages.sury.org, OpenLiteSpeed dari repositori LiteSpeed, versi MariaDB dan PostgreSQL dari repositori proyeknya jika Anda memilihnya, dan ionCube Loader dari ionCube. Sebagian perangkat lunak yang terinstal terhubung keluar dengan sendirinya: misalnya, ClamAV mengunduh basis data virusnya.

Pemeriksaan keamanan tidak mengirim apa pun ke luar server. Sebagian besar permintaan dari panel mengidentifikasi dirinya sebagai `Nefo/<version>`. Permintaan itu tidak menyertakan informasi tentang server atau situs web Anda selain yang dibutuhkan tugas tersebut, seperti nama domain dalam permintaan sertifikat.
