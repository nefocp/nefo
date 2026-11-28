---
title: Catatan perubahan
description: Apa yang berubah di setiap rilis Nefo, yang terbaru lebih dulu, dan apa yang perlu diketahui sebelum Anda memperbarui.
---

# Catatan perubahan

Setiap rilis Nefo, yang terbaru lebih dulu, dengan apa yang baru, apa yang berubah, dan apa yang diperbaiki, serta apa yang perlu diketahui sebelum Anda memperbarui. Setiap rilis menautkan ke halamannya di GitHub, yang berisi program untuk amd64 dan arm64, penginstal, checksum, dan tanda tangannya. Untuk memperbarui, lihat [Pembaruan](./docs/update).

::: warning Kembali setelah pembaruan
v0.21.0, v0.22.0, v0.23.0, dan v0.24.0 masing-masing mengubah format basis data panel saat pertama kali dijalankan. Setelah itu, versi yang lebih lama menolak berjalan dengan data tersebut. Jika Anda mungkin ingin kembali, salin `/var/lib/nefo` sebelum memperbarui, seperti yang ditunjukkan di [Pembaruan](./docs/update#if-an-update-fails).
:::

## v0.24.1 {#v0-24-1}

28 November 2026 · [Rilis di GitHub](https://github.com/nefocp/nefo/releases/tag/v0.24.1)

Cadangan situs web yang memulihkan situs aplikasi dan situs reverse proxy, pemeriksaan keamanan yang lebih cepat, dan perbaikan untuk ketujuh bahasa.

**Diubah**

- **Cadangan situs web mencatat jenis situsnya,** sehingga saat dipulihkan di server yang belum punya situs itu, situs aplikasi (Node.js, Python, Java) kembali dengan runtime, perintah mulai, port, dan batas memorinya, dan situs reverse proxy kembali dengan alamat dan opsinya. Instal dulu runtime aplikasinya di halaman **Perangkat lunak**. Cadangan yang dibuat versi lama tetap kembali sebagai situs statis. Cadangan situs aplikasi Docker ditolak, beserta langkah yang perlu dilakukan: instal dulu aplikasinya dari Toko aplikasi, lalu pulihkan cadangan aplikasinya. Lihat [Memulihkan di server baru](./docs/restore-new-server).
- **Pemeriksaan keamanan menjalankan pemeriksaannya bersamaan,** paling lama 5 detik untuk setiap program yang ditanyainya dan 25 detik untuk seluruh pemeriksaan. Pemeriksaan yang tidak selesai tepat waktu ditandai **hasil sebagian**, alih-alih menahan pemeriksaan lainnya. Lihat [Pemeriksaan keamanan](./docs/security-check).

**Diperbaiki**

- Di server yang sibuk, pemeriksaan keamanan bisa berjalan lama lalu berakhir dengan galat server.
- Di kontainer dengan batas memori, dasbor menghitung page cache dan memori yang dibebaskan kernel saat program membutuhkannya sebagai terpakai, sehingga memori tampak hampir penuh setelah pencadangan atau unduhan besar. Pengaturan swap kini juga menampilkan memori kontainer.
- Di server yang memakai repositori PHP, PHP terbaru dari repositori (8.6 di Ubuntu 24.04) dianggap sebagai versi PHP bawaan distribusi: halaman **Perangkat lunak** tertukar menampilkan asal kedua versi, dan server web yang diinstal sesudahnya mendapat versi itu, bukan versi distribusi.
- Aplikasi yang ditambahkan katalog Toko aplikasi yang lebih baru tidak punya logo di luar halaman Toko aplikasi.
- Pada bahasa dengan teks yang lebih panjang, menu akun memotong daftar bahasa (kini dua kolom), tab di panel situs web memotong namanya (kini bisa dua baris), dan bilah atas bisa turun ke baris kedua di layar sempit.
- Beberapa teks antarmuka berbahasa Tionghoa, Rusia, dan Indonesia diperbaiki.

## v0.24.0 {#v0-24-0}

27 November 2026 · [Rilis di GitHub](https://github.com/nefocp/nefo/releases/tag/v0.24.0)

Tujuh bahasa antarmuka, pencadangan ke Google Drive, pemulihan di server baru, dan pemeriksaan keamanan.

**Baru**

- **Antarmuka dalam tujuh bahasa:** Tionghoa, Spanyol, Portugis, Rusia, dan Indonesia di samping Inggris dan Vietnam, termasuk teks Toko aplikasi. Setiap bahasa adalah file terpisah yang baru diunduh browser saat bahasa itu dipakai, dan file antarmuka kini dikirim dalam bentuk terkompresi gzip: skrip utama turun dari 478 KB menjadi 163 KB.
- **Pencadangan ke Google Drive,** dengan OAuth client milik Anda sendiri, masuk dengan kode perangkat, akses hanya ke file yang dibuat panel, dan unggahan yang berlanjut setelah terputus. Lihat [Google Drive](./docs/backups#google-drive).
- **Salinan jarak jauh terenkripsi:** setiap penyimpanan (S3, SFTP, Google Drive) bisa mengenkripsi cadangan yang diterimanya dengan passphrase: scrypt dan AES-256-GCM dalam potongan bernomor, sehingga salinan yang terpotong atau diubah ditolak. `nefo decrypt-backup` membuka salinan secara manual. Lihat [Mengenkripsi salinan](./docs/backups#encrypt-the-copies).
- **Pemulihan di server baru:** **Cari cadangan** menampilkan cadangan di penyimpanan yang tidak dikenal panel ini, dan pemulihan membuat situs web yang belum ada, beserta domain, alias, versi PHP, file, dan basis data dengan kata sandinya. Memulihkan cadangan situs web yang sudah dihapus membuatnya kembali dengan cara yang sama. Lihat [Memulihkan di server baru](./docs/restore-new-server).
- **Situs web:** kloning situs web aplikasi Node.js, Python, dan Docker, serta situs web dengan basis data PostgreSQL. Menghapus situs web bisa mempertahankan basis datanya, dan secara bawaan membuat cadangan terlebih dahulu. Jenis situs web baru, reverse proxy, meneruskan ke alamat mana pun, di Nginx, Apache, dan OpenLiteSpeed. Lihat [Situs web](./docs/websites#reverse-proxy-websites).
- **Pemeriksaan keamanan:** 22 pemeriksaan cepat atas server (SSH, firewall, pembaruan, basis data atau Redis yang terbuka ke internet, port Docker yang melewati ufw, file yang bisa dijangkau lewat HTTP, sertifikat, pengaturan PHP…), masing-masing dengan tautan ke perbaikannya, sesuai permintaan atau setiap hari. Lihat [Pemeriksaan keamanan](./docs/security-check).
- **Peringatan saat masuk** untuk masuk lewat SSH (setiap kali, atau hanya dari alamat baru), rentetan upaya masuk SSH yang gagal, dan masuk ke panel. Lihat [Peringatan saat masuk](./docs/mail-alerts#sign-in-alerts).
- **Pemblokiran negara** per situs web, dengan data negara gratis dari DB-IP, yang baru diunduh setelah ada situs web yang memakainya. Fitur ini juga berfungsi di situs web reverse proxy. Lihat [Pemblokiran negara dan penguncian file](./docs/website-protection).
- **Penguncian file** per situs web: kodenya menjadi hanya-baca bagi PHP dan akun SFTP-nya, kecuali folder yang harus ditulisinya, dengan peringatan harian opsional saat file PHP berubah. `nefo unlock-files` mengembalikan file seperti semula sebelum Anda menghapus Nefo.
- **Basis data:** MariaDB 11.4, 11.8, dan 12.3 LTS dari repositori MariaDB, dan peningkatan versi di tempat dengan dump terlebih dahulu dan jalan kembali jika gagal; peralihan antara MariaDB dan MySQL yang ikut membawa basis datanya; rotasi log kueri lambat. Lihat [Basis data](./docs/databases#mariadb-versions).
- **PostgreSQL:** versi 15 sampai 19 dari proyek PostgreSQL, akses jarak jauh per basis data (TLS, scram-sha-256, aturan firewall per alamat), dan peningkatan versi mayor dengan dump terlebih dahulu dan jalan kembali. Dump yang disimpan oleh peningkatan versi ditampilkan dalam daftar, dengan tombol hapus. Lihat [PostgreSQL](./docs/postgresql).
- **Record email di halaman DNS:** MX, satu record SPF gabungan, dan DMARC untuk Google Workspace, Microsoft 365, Zoho Mail, Cloudflare Email Routing, atau penyedia lain, lewat Cloudflare. Lihat [DNS (Cloudflare)](./docs/dns#set-up-email-records).
- **Perangkat lunak:** Java (OpenJDK 17, 21, dan 25) untuk situs web aplikasi, ionCube Loader untuk setiap versi PHP, dan PHP 8.6, yang ditawarkan dengan sendirinya setelah repositori PHP menyediakannya. Lihat [Perangkat lunak](./docs/software).
- **Toko aplikasi:** 15 aplikasi baru, total 42: Grafana, ntfy, Gotify, Memos, linkding, Navidrome, Vikunja, Actual Budget, changedetection.io, Healthchecks, listmonk, Kanboard, FreshRSS, Jellyfin, dan Mealie. Lihat [Toko aplikasi](./docs/apps).

**Diubah**

- Angka memori mengikuti batas memori kontainer itu sendiri (LXC), di dasbor, di peringatan, di penyiapan, dan di pemeriksaan Toko aplikasi.
- Angka disk di halaman penyiapan diperbaiki, dan kini ikut menghitung daftar paket apt di server yang belum pernah mengunduhnya.
- Katalog Toko aplikasi yang lebih baru, yang berisi aplikasi yang tidak bisa dijalankan panel versi lama, kini hanya melewatkan aplikasi-aplikasi itu. Aplikasi yang ditambahkan katalog yang lebih baru menampilkan namanya di mana-mana.

**Diperbaiki**

- PHP 8.5 tidak bisa diinstal, karena OPcache tidak lagi punya paket sendiri.
- Di Ubuntu 26.04, menginstal PostgreSQL (atau layanan lain) bisa gagal saat systemd menjalankan ulang dirinya sendiri setelah apt. Panel kini mencoba `systemctl` lagi selama systemd belum bisa dihubungi.
- Firewall web ditawarkan di distribusi yang tidak punya modul ModSecurity untuk server webnya (Ubuntu 22.04 dengan Nginx). Kini firewall web ditampilkan sebagai tidak ditawarkan, beserta alasannya.
- Teks halaman WordPress yang masih kosong terpotong di tengah kalimat.

## v0.23.0 {#v0-23-0}

26 November 2026 · [Rilis di GitHub](https://github.com/nefocp/nefo/releases/tag/v0.23.0)

Kunci API, impor situs web, halaman penyiapan, serta lebih banyak server basis data dan versi PHP.

**Baru**

- **Kunci API** untuk skrip Anda sendiri, di **Pengaturan** → **Kunci API**: hanya-baca atau akses penuh, daftar alamat yang diizinkan, dan tanggal kedaluwarsa. Kunci dikirim di header `Authorization: Bearer` dan hanya disimpan sebagai hash, dan kunci yang salah dihitung sebagai upaya masuk yang gagal. Tidak ada kunci yang bisa membuka terminal, Adminer, phpMyAdmin, akun, atau kunci-kunci itu sendiri. Kunci hanya-baca tidak bisa membaca kata sandi, file, ekspor, atau unduhan cadangan. Lihat [Kunci API](./docs/api).
- **Impor situs web** dari cadangan lengkap cPanel (situs web, basis data beserta pengguna dan kata sandinya, sertifikat, dan tugas cron), atau dari arsip situs dan dump SQL yang dibuat oleh aaPanel atau server lain. Arsip berbahaya ditolak, dump MySQL 8 disesuaikan untuk MariaDB, dan impor yang gagal menghapus semua yang sudah dibuatnya. Lihat [Mengimpor situs web](./docs/import).
- **Halaman penyiapan** saat pertama kali masuk: pilih server web, versi PHP, server basis data, dan tambahan di satu halaman, lihat perkiraan memori dan disk yang dibutuhkan, lalu instal semuanya sebagai satu tugas. `nefo setup` melakukan hal yang sama di baris perintah, dan penginstal menerima opsi yang sama. Tanpa opsi, penginstal berperilaku seperti sebelumnya. Lihat [Langkah pertama](./docs/first-steps#set-up-the-server).
- **MySQL 8.4 LTS** sebagai alternatif MariaDB (salah satunya per server), dari repositori Oracle dengan kuncinya yang dipatok. MySQL hanya ditawarkan di server amd64.
- **phpMyAdmin**, yang hanya bisa dijangkau melalui halaman masuk panel.
- **PostgreSQL** di halaman **Basis data**: basis data, pemilik, impor dan ekspor, serta pencadangan.
- **Memcached**, yang hanya mendengarkan di server itu sendiri, tanpa UDP.
- **PHP 5.6 sampai 7.3**, untuk situs web yang dipindahkan dari server lama. Versi-versi ini hanya diinstal setelah Anda mengonfirmasi, dan tidak pernah bisa menjadi satu-satunya versi PHP di server.
- **Tanda akhir masa dukungan untuk PHP:** versi yang sudah melewati akhir dukungan keamanan PHP ditandai di halaman **Perangkat lunak** dan di situs webnya. Versi PHP milik distribusi Anda sendiri tidak pernah ditandai, karena distribusi terus menambalnya. Lihat [Perangkat lunak](./docs/software#end-of-life).
- **Composer** dan **WP-CLI** di halaman **Perangkat lunak**.

**Diubah**

- Di Ubuntu, versi PHP tambahan sekarang berasal dari `packages.sury.org`, karena `ppa:ondrej/php` tidak lagi menerima pembaruan. Server yang punya PPA itu beralih sekali, saat panel dijalankan: file sumber PPA dipindahkan ke `/var/lib/nefo/apt-backup`, tidak ada yang diinstal atau dihapus, dan paket PHP yang terinstal diperbarui dari sumber baru setelah `apt-get update` berikutnya.

**Diperbaiki**

- Situs web yang dibuat dalam keadaan ditangguhkan sekarang juga disimpan sebagai ditangguhkan.
- Folder yang dibuat panel untuk program lain tidak lagi mendapat izin ketat milik panel sendiri (umask 077).
- Versi PHP milik distribusi dikenali di server baru, sebelum apt mengunduh daftar paketnya.
- `@` saja (domain itu sendiri) tidak lagi merusak pesan di halaman DNS.
- Tugas yang crash sekarang menulis stack trace-nya ke log panel.

## v0.22.0 {#v0-22-0}

26 November 2026 · [Rilis di GitHub](https://github.com/nefocp/nefo/releases/tag/v0.22.0)

Pemantauan situs web, lebih banyak saluran peringatan, dan DNS Cloudflare.

**Baru**

- **Pemantauan situs web:** setiap situs web diperiksa setiap 1 sampai 10 menit (kode status, kata kunci opsional, dan masa berlaku sertifikat). Situs web dianggap mati setelah dua pemeriksaan gagal, dan Anda mendapat peringatan saat situs mati dan saat hidup kembali. Panel menampilkan uptime selama 24 jam dan 7 hari, waktu respons, dan kegagalan terbaru. Permintaan pemantauan tidak dihitung dalam statistik lalu lintas. Lihat [Pemantauan](./docs/websites#monitoring).
- **Peringatan lewat Discord, Slack, dan webhook.** Webhook ditandatangani dengan HMAC-SHA256. URL yang tersimpan disamarkan, dan alamat di server itu sendiri, alamat link-local, dan alamat metadata cloud ditolak saat peringatan dikirim. Lihat [Email dan peringatan](./docs/mail-alerts).
- **Halaman DNS untuk zona Cloudflare:** tampilkan, tambah, edit, dan hapus record, aktifkan atau nonaktifkan proxy, kosongkan cache, dan arahkan domain situs web ke server ini dengan pratinjau terlebih dahulu. Lihat [DNS (Cloudflare)](./docs/dns).

## v0.21.0 {#v0-21-0}

26 November 2026 · [Rilis di GitHub](https://github.com/nefocp/nefo/releases/tag/v0.21.0)

Alat sistem, kunci SSH, dan lebih banyak opsi situs web.

**Baru**

- **Halaman Sistem:** pembaruan sistem operasi (pembaruan keamanan ditandai, instal semua atau hanya pembaruan keamanan, pembaruan keamanan otomatis, dan pemberitahuan saat server perlu dimulai ulang), file swap dan swappiness, zona waktu, sinkronisasi jam, nama host, TCP BBR, dan pembersihan disk. Pemeriksaan pembaruan di latar belakang menunggu 30 menit setelah panel dijalankan, dan memakai ulang daftar paket yang disegarkan dalam 6 jam terakhir. Lihat [Sistem](./docs/os).
- **Kunci SSH** di halaman **Firewall & SSH**: tampilkan, tambah, dan hapus. Kunci privat, kunci lemah, dan duplikat ditolak, dan jalan masuk terakhir tidak bisa dihapus.
- **Situs web:** nonaktifkan HTTPS dan hapus sertifikat, di ketiga server web; HSTS, dengan `includeSubDomains` sebagai opsi; cache halaman per situs web PHP di Nginx, dengan tombol pengosongan; kloning situs web PHP atau statis beserta file, pengaturan, dan basis datanya; dorong salinan staging WordPress kembali ke situs web live, setelah pencadangan.

**Diperbaiki**

- Proses apt milik panel sendiri tidak pernah tumpang-tindih, dan panel menunggu kunci apt milik program lain alih-alih gagal.
- Di kontainer yang `/etc/hosts`-nya di-bind-mount, nama host ditulis langsung di file itu. Nama host yang ditolak sistem memberi pesan kesalahan yang jelas.

## v0.20.2 {#v0-20-2}

26 November 2026 · [Rilis di GitHub](https://github.com/nefocp/nefo/releases/tag/v0.20.2)

**Diubah**

- Nginx sekarang mengompres CSS, JavaScript, SVG, dan font. Debian dan Ubuntu hanya mengaktifkan kompresi untuk HTML, jadi file-file ini dulu dikirim tanpa kompresi. Pengaturan yang sudah ada di `nginx.conf` dibiarkan apa adanya. Server yang sudah ada mendapat perubahan ini saat panel yang diperbarui dijalankan.

**Diperbaiki**

- Setiap rilis sekarang membawa katalog toko aplikasi yang ditandatangani. Sebelumnya, panel mencarinya sekali sehari dan mencatat HTTP 404 di log.

## v0.20.1 {#v0-20-1}

26 November 2026 · [Rilis di GitHub](https://github.com/nefocp/nefo/releases/tag/v0.20.1)

**Diperbaiki**

- Penginstal gagal di setiap sistem yang didukung: penginstal membaca versi sistem dari `/etc/os-release` alih-alih versinya sendiri, dan mencoba mengunduh rilis yang tidak ada. Setelah perbaikan, penginstal diuji di server Ubuntu 24.04 dan Debian 12 yang baru diinstal.

## v0.20.0 {#v0-20-0}

26 November 2026 · [Tag di GitHub](https://github.com/nefocp/nefo/releases/tag/v0.20.0)

Rilis publik pertama. Penginstalnya gagal (diperbaiki di v0.20.1), dan GitHub tidak punya file rilis untuknya, hanya tag. Instal rilis yang lebih baru.

Rilis ini berisi situs web di Nginx, Apache, atau OpenLiteSpeed dengan PHP-FPM, aplikasi Node.js dan Python, sertifikat Let's Encrypt, deploy dari Git, dan toolkit WordPress; toko aplikasi dengan aplikasi PHP dan Docker; MariaDB, Redis, pengelola file, terminal web, dan pencadangan ke penyimpanan yang kompatibel dengan S3 dan SFTP; firewall (ufw), fail2ban, dan pengaturan SSH dengan pengaman agar tidak terkunci, firewall aplikasi web, dan pemindai malware; rilis yang ditandatangani dan `nefo update`; serta antarmuka dalam bahasa Inggris dan Vietnam.
