---
title: Toko aplikasi
description: Instal aplikasi PHP ke situs web, aplikasi Docker di balik situs webnya sendiri, dan layanan basis data, serta kelola, perbarui, cadangkan, dan hapus aplikasi tersebut.
---

# Toko aplikasi

Toko aplikasi Nefo menginstal 42 aplikasi dengan satu klik: aplikasi PHP seperti WordPress, WooCommerce, Joomla, Nextcloud, dan Moodle ke dalam situs web, aplikasi Docker seperti Ghost, n8n, Gitea, Grafana, dan Jellyfin di balik situs webnya sendiri, serta PostgreSQL, MongoDB, dan Ollama sebagai layanan di 127.0.0.1. Toko aplikasi memeriksa memori dan disk server terlebih dahulu.

Nefo membuat apa yang dibutuhkan setiap aplikasi, seperti basis data, situs web, kata sandi, dan kontainer. Halaman ini membahas jenis-jenis aplikasi, katalog, instalasi, dan pengelolaan aplikasi yang sudah Anda instal.

Buka **Toko aplikasi** di menu kiri. Tab **Katalog** menampilkan daftar aplikasi dan tab **Terinstal** menampilkan aplikasi di server ini. Di halaman **Situs web**, **Instal dari Toko aplikasi** juga mengarah ke sini.

## Tiga jenis aplikasi {#three-kinds-of-apps}

| Jenis | Di mana dipasang | Apa yang didapat |
|---|---|---|
| **Aplikasi PHP** | Ke situs web PHP kosong yang Anda buat | Basis data sendiri, file-filenya di situs web, aturan rewrite yang tepat. Berjalan sebagai pengguna Linux situs web. |
| **Aplikasi Docker** | Ke situs web baru yang dibuat Nefo untuknya | Kontainer yang hanya mendengarkan di `127.0.0.1`, dengan situs web di depannya. |
| **Layanan** | Tanpa situs web | Kontainer yang hanya mendengarkan di `127.0.0.1`, untuk situs dan aplikasi Anda sendiri. Masing-masing satu per server. |

Aplikasi PHP dan aplikasi Docker membutuhkan server web yang terinstal: lihat [Situs web](./websites). Aplikasi Docker dan layanan juga membutuhkan Docker. Instal Docker di halaman **Perangkat lunak**; **Instal Docker** di toko aplikasi membuka halaman itu. Lihat [Docker](./docker).

## Aplikasi apa saja yang bisa saya instal? {#the-catalog}

Pada versi katalog 2026112701, toko aplikasi menawarkan 42 aplikasi. Aplikasi dikelompokkan dalam kategori: **Semua** menampilkan setiap kategori beserta aplikasinya, dan tombol kategori hanya menampilkan aplikasi kategori itu. Pencarian melihat nama, deskripsi, dan nama kategori, serta mengabaikan diakritik bahasa Vietnam (mengetik "giam sat" menemukan aplikasi "Giám sát & thống kê").

Memori minimum adalah total RAM server yang di bawahnya Nefo menolak instalasi (swap tidak dihitung).

| Aplikasi | Jenis | Kategori | Apa itu | RAM minimum |
|---|---|---|---|---|
| WordPress | Aplikasi PHP | Situs web & konten | Pembuat situs web dan blog | 450 MB |
| Joomla | Aplikasi PHP | Situs web & konten | CMS untuk situs web dan portal | 450 MB |
| Drupal | Aplikasi PHP | Situs web & konten | CMS untuk konten terstruktur, dibangun dengan Composer | 900 MB |
| Ghost | Aplikasi Docker | Situs web & konten | Blog dan newsletter | 900 MB |
| Directus | Aplikasi Docker | Situs web & konten | Headless CMS | 900 MB |
| Wiki.js | Aplikasi Docker | Situs web & konten | Wiki | 900 MB |
| WooCommerce | Aplikasi PHP | Toko & bisnis | WordPress dengan WooCommerce: toko online | 900 MB |
| OpenCart | Aplikasi PHP | Toko & bisnis | Keranjang belanja | 900 MB |
| Odoo | Aplikasi Docker | Toko & bisnis | ERP (edisi Community) | 1800 MB |
| Stirling-PDF | Aplikasi Docker | Produktivitas | Alat PDF | 1800 MB |
| Memos | Aplikasi Docker | Produktivitas | Catatan Markdown singkat di linimasa | 230 MB |
| linkding | Aplikasi Docker | Produktivitas | Pengelola bookmark | 450 MB |
| Vikunja | Aplikasi Docker | Produktivitas | Daftar tugas, papan Kanban, dan proyek | 450 MB |
| Actual Budget | Aplikasi Docker | Produktivitas | Penganggaran pribadi | 450 MB |
| Kanboard | Aplikasi Docker | Produktivitas | Papan Kanban untuk proyek | 230 MB |
| FreshRSS | Aplikasi Docker | Produktivitas | Pembaca berita dan blog (RSS) | 450 MB |
| Mealie | Aplikasi Docker | Produktivitas | Resep, rencana menu, dan daftar belanja | 900 MB |
| Nextcloud | Aplikasi PHP | File & penyimpanan | Cloud Anda sendiri untuk file, kalender, dan kontak | 900 MB |
| RustFS | Aplikasi Docker | File & penyimpanan | Penyimpanan objek yang kompatibel dengan S3 | 900 MB |
| FileBrowser Quantum | Aplikasi Docker | File & penyimpanan | File di browser | 450 MB |
| Navidrome | Aplikasi Docker | Media | Streaming musik dari koleksi Anda sendiri | 450 MB |
| Jellyfin | Aplikasi Docker | Media | Film, serial, dan musik dari server Anda | 1800 MB |
| ntfy | Aplikasi Docker | Komunikasi | Notifikasi push lewat HTTP | 230 MB |
| Gotify | Aplikasi Docker | Komunikasi | Server notifikasi dengan aplikasi Android | 230 MB |
| listmonk | Aplikasi Docker | Komunikasi | Newsletter dan milis | 450 MB |
| Uptime Kuma | Aplikasi Docker | Pemantauan & analitik | Memantau situs dan layanan Anda | 450 MB |
| Umami | Aplikasi Docker | Pemantauan & analitik | Analitik situs web yang ramah privasi | 900 MB |
| Grafana | Aplikasi Docker | Pemantauan & analitik | Dasbor dan peringatan | 900 MB |
| changedetection.io | Aplikasi Docker | Pemantauan & analitik | Memantau perubahan di halaman web | 900 MB |
| Healthchecks | Aplikasi Docker | Pemantauan & analitik | Peringatan saat tugas cron berhenti berjalan | 450 MB |
| Vaultwarden | Aplikasi Docker | Keamanan & jaringan | Server pengelola kata sandi untuk aplikasi Bitwarden | 450 MB |
| AdGuard Home | Aplikasi Docker | Keamanan & jaringan | Pemblokiran iklan lewat alamat DNS-over-HTTPS Anda sendiri | 450 MB |
| Open WebUI | Aplikasi Docker | AI | Antarmuka chat untuk Ollama atau API yang kompatibel dengan OpenAI | 1800 MB |
| Ollama | Layanan | AI | Menjalankan model bahasa terbuka di CPU | 3584 MB |
| Laravel | Aplikasi PHP | Alat pengembang & otomatisasi | Proyek Laravel baru dengan basis datanya | 450 MB |
| code-server | Aplikasi Docker | Alat pengembang & otomatisasi | VS Code di browser | 900 MB |
| IT-Tools | Aplikasi Docker | Alat pengembang & otomatisasi | Alat pengembang di browser | 230 MB |
| Gitea | Aplikasi Docker | Alat pengembang & otomatisasi | Server Git Anda sendiri | 450 MB |
| n8n | Aplikasi Docker | Alat pengembang & otomatisasi | Otomatisasi alur kerja | 900 MB |
| PostgreSQL | Layanan | Basis data | Server basis data PostgreSQL | 900 MB |
| MongoDB | Layanan | Basis data | Basis data dokumen MongoDB | 900 MB |
| Moodle | Aplikasi PHP | Pendidikan | Platform pembelajaran | 900 MB |

Katalog tertanam di dalam panel. Nefo juga memeriksa sekali sehari apakah ada katalog yang lebih baru yang dipublikasikan bersama rilis, dan hanya memakainya jika tanda tangannya sah. **Perbarui katalog** langsung memeriksanya. Bagian atas menampilkan versi katalog dan apakah katalog itu "bawaan panel" atau "diunduh, tanda tangan diperiksa".

Katalog yang diunduh bisa berisi aplikasi yang membutuhkan versi Nefo yang lebih baru, karena resepnya memakai sesuatu yang tidak ada di versi ini. Nefo meninggalkan aplikasi-aplikasi itu dan tetap memakai sisa katalog. Jika Nefo sudah mengenal aplikasi itu, Nefo mempertahankan versi yang dikenalnya, jadi salinan yang terinstal tetap berfungsi. Bagian atas lalu menampilkan, misalnya, "2 aplikasi dari katalog yang diunduh memerlukan panel yang lebih baru". Perbarui Nefo untuk mendapatkannya: lihat [Pembaruan](./update). Aplikasi yang ditambahkan katalog yang lebih baru menampilkan nama dan deskripsinya dalam setiap bahasa yang ada di katalog.

Setiap kartu menampilkan versi aplikasi, jenis, serta kebutuhan memori dan disk. Tombol **Instal**-nya membuka deskripsi, lisensi, kebutuhan, catatan, dan formulir instalasi. Kartu aplikasi yang tidak bisa dijalankan server menampilkan alasannya, dan **Detail** sebagai gantinya. Layanan yang sudah terinstal menampilkan **Kelola**. Lencana "Source-available" menandai lisensi yang tidak disetujui OSI.

## Pemeriksaan sebelum instalasi {#the-checks-before-an-install}

Saat Anda membuka sebuah aplikasi, Nefo membandingkan kebutuhannya dengan server. Nefo menolak instalasi jika:

- total memori server di bawah minimum aplikasi (di VPS yang berupa kontainer, batas memorinya yang dihitung);
- ruang disk kosong kurang dari image dan data aplikasi ditambah 1 GB;
- arsitektur atau fitur prosesor tidak cocok dengan aplikasi.

Nefo memperingatkan, tetapi tetap mengizinkan instalasi, jika server punya memori lebih sedikit dari yang disarankan, jika memori yang bebas saat ini lebih sedikit daripada yang mungkin dipakai aplikasi, atau jika batas memori semua aplikasi terinstal jika dijumlahkan melebihi memori server.

## Menginstal aplikasi PHP {#install-a-php-app}

1. Buat situs web PHP kosong dengan versi PHP yang didukung aplikasi: lihat [Situs web](./websites). Versi PHP minimumnya adalah WordPress 7.4, Nextcloud dan Laravel 8.2, WooCommerce, Joomla, Drupal, dan Moodle 8.3, OpenCart 8.0. OpenCart mendukung PHP hingga 8.3 dan Moodle hingga 8.4.
2. Pastikan MariaDB (atau MySQL) terinstal dan berjalan: lihat [Basis data](./databases).
3. Di toko aplikasi, klik tombol **Instal** aplikasi tersebut.
4. Pilih **Situs web**. Daftar ini hanya menampilkan situs web PHP kosong dengan versi PHP yang sesuai. Jika tidak ada, gunakan **Tambah situs web PHP**.
5. Untuk aplikasi yang memintanya, isi **Nama situs**, **Administrator** (hindari `admin`: itu nama pertama yang dicoba penyerang), **Email administrator**, dan **Kata sandi administrator**. Kosongkan kata sandi untuk mendapatkan kata sandi acak yang kuat; kata sandi yang Anda ketik harus terdiri dari 12 sampai 128 karakter. WooCommerce juga menanyakan negara dan mata uang toko.
6. Klik **Instal**. Instalasi berjalan di latar belakang dan tampil di daftar tugas.

Setelah selesai, Nefo menampilkan **Alamat masuk**, **Administrator**, dan **Kata sandi administrator** sekali saja. Simpan sekarang.

Folder `public` situs web harus kosong; halaman sambutan panel tidak masalah. File situs web tidak boleh [dikunci](./website-protection#lock-a-website-s-files). Drupal dan Laravel dibangun dengan Composer, dan Moodle juga membutuhkan seluruh folder situs: situs webnya harus baru dan kosong, dan folder situs menjadi proyeknya, dengan `public` sebagai folder webnya.

Aplikasi PHP mendapatkan:

- basis data dan pengguna basis data baru, yang tertaut ke situs web;
- file aplikasi, yang diunduh dan diperiksa terhadap checksum di katalog;
- aturan rewrite aplikasi, dan batas PHP yang lebih tinggi jika aplikasi membutuhkannya;
- tugas cron yang dijalankan sebagai pengguna situs web untuk Moodle (setiap menit) dan WooCommerce (setiap lima menit).

Jika instalasi gagal, Nefo mengosongkan situs web lagi dan menghapus basis data baru. Situs WordPress juga muncul di [Toolkit WordPress](./wordpress).

## Menginstal aplikasi Docker {#install-a-docker-app}

1. Arahkan domain aplikasi ke alamat IP server ini di DNS Anda, misalnya `app.example.com` ke `203.0.113.10`.
2. Di toko aplikasi, klik tombol **Instal** aplikasi tersebut.
3. Masukkan **Domain situs web baru** dan, jika mau, **Domain lain**.
4. Beberapa aplikasi juga meminta **Email administrator**. Gunakan domain sungguhan, bukan `.test` atau `.local`.
5. Klik **Instal**.
6. Setelah instalasi selesai, aktifkan HTTPS di tab **SSL / HTTPS** situs web baru: lihat [Sertifikat SSL](./ssl). Aplikasi dimulai ulang dengan alamat `https://`-nya. Beberapa aplikasi membutuhkan HTTPS agar berfungsi penuh, dan formulir instalasi menyebutkannya.

Cara mendapatkan akun administrator pertama bergantung pada aplikasinya, dan formulir instalasi memberi tahu Anda:

| Cara | Aplikasi | Yang Anda lakukan |
|---|---|---|
| Dibuat oleh panel | RustFS, code-server, AdGuard Home, Odoo, Umami, Grafana, ntfy, Gotify, Memos, linkding, Navidrome, Vikunja, Actual Budget, listmonk, Kanboard, FreshRSS, Jellyfin, Mealie | Buka **Kredensial** di tab **Terinstal**. Penggunanya `admin` (Mealie: `changeme@example.com`; Actual Budget hanya punya kata sandi server). |
| Email Anda dan kata sandi acak | Open WebUI, Directus, Wiki.js, Healthchecks | Buka **Kredensial**. |
| Akun pertama yang terdaftar | Ghost, Uptime Kuma, n8n, Gitea | Daftar tepat setelah instalasi: akun pertama menjadi administrator. |
| Undangan | Vaultwarden | Daftar di halaman aplikasi dengan alamat email yang diundang panel. |
| Kata sandi sekali pakai | FileBrowser Quantum | Buka **Kredensial**; aplikasi meminta kata sandi baru saat pertama kali masuk. |
| Kata sandi situs web | Stirling-PDF, changedetection.io | Situs web meminta pengguna `admin` dan kata sandi yang ada di **Kredensial**. |
| Tanpa masuk | IT-Tools | Tidak ada yang perlu dilakukan. |

Catatan untuk sebagian aplikasi ini:

- **Memos** dan **Vikunja**: panel membuat akun lalu menutup pendaftaran. Anda bisa membukanya lagi di pengaturan aplikasi.
- **ntfy**: tidak ada yang bisa membaca atau mengirim pesan tanpa masuk. Akun `admin` adalah bagian dari pengaturan aplikasi, jadi kata sandinya adalah yang ada di **Kredensial**.
- **Mealie**: masuk dengan `changeme@example.com` dan kata sandi di **Kredensial** (panel sudah mengganti kata sandi bawaannya), lalu atur alamat email Anda sendiri di profil.
- **Actual Budget** hanya terbuka lewat HTTPS: dapatkan dulu sertifikat untuk situs webnya.
- **Healthchecks** dan **listmonk** hanya mengirim email lewat layanan SMTP yang Anda siapkan di aplikasi; server ini sendiri tidak mengirim email. Peringatan Healthchecks ke ntfy, Telegram, Slack, atau webhook berfungsi tanpanya.
- **Navidrome** dan **Jellyfin**: lihat [Folder musik dan video](#music-and-video-folders).

Aplikasi Docker mendapatkan:

- situs web baru yang meneruskan ke aplikasi. Situs web tetap tertutup (503) sampai instalasi selesai dan login bawaan aplikasi dikunci;
- kontainer yang berjalan sebagai proyek Docker Compose, dengan batas memori dan log yang dirotasi pada 10 MB;
- file-filenya di `/var/lib/panel-apps/<name>`, yang hanya bisa dibaca root: file Compose, kata sandi yang dibuat, dan data kontainer;
- port yang hanya dipublikasikan di `127.0.0.1`, tidak pernah di internet;
- image yang dipatok ke versi yang persis.

### Folder musik dan video {#music-and-video-folders}

Navidrome dan Jellyfin memutar file dari sebuah folder di situs webnya, agar Anda bisa menambahkan file dengan [pengelola file](./files) atau SFTP:

| Aplikasi | Folder | Dilihat aplikasi sebagai |
|---|---|---|
| Navidrome | `/var/www/<domain>/music` | `/music` |
| Jellyfin | `/var/www/<domain>/media` | `/media` |

Nefo membuat folder itu saat instalasi, dimiliki oleh pengguna Linux situs web, dan memberikannya ke aplikasi sebagai hanya-baca: aplikasi bisa memutar file tetapi tidak bisa mengubahnya. Navidrome memindai folder saat dijalankan lalu setiap jam. Di Jellyfin, tambahkan library dan pilih folder di bawah `/media`.

File-file ini bagian dari situs web, jadi cadangan situs web menyertakannya, dan menghapus situs web juga menghapusnya. Mengonversi video secara langsung memakan banyak CPU di server kecil: utamakan file yang bisa diputar perangkat Anda apa adanya.

## Menginstal layanan {#install-a-service}

PostgreSQL, MongoDB, dan Ollama tidak punya situs web. Klik **Instal**: tidak ada yang perlu diisi. Nefo memilih port bebas di `127.0.0.1` dan membuat kata sandinya. Aplikasi di server ini bisa menjangkau layanan; internet tidak bisa. Setiap layanan bisa diinstal sekali per server.

Buka **Kredensial** di tab **Terinstal** untuk alamat, pengguna, kata sandi, dan **String koneksi**.

## Pemeriksaan keamanan pada aplikasi Docker {#the-safety-checks-on-docker-apps}

Resep aplikasi Docker di katalog berupa data, tidak pernah berupa skrip. Nefo mengubahnya menjadi file Compose dan memeriksa file itu dengan aturan yang sama seperti proyek Compose Anda sendiri, tanpa pengecualian:

- tanpa kontainer privileged, tanpa capability tambahan, dan tanpa perangkat host;
- tanpa berbagi namespace jaringan, proses, atau namespace lain milik server;
- tanpa opsi keamanan yang dilemahkan dan tanpa pengaturan kernel;
- port hanya dipublikasikan di `127.0.0.1`;
- tanpa mount folder sistem atau socket Docker;
- folder di situs web (musik Navidrome, media Jellyfin) hanya di-mount ke kontainer yang berjalan sebagai pengguna Linux situs web itu sendiri, dan sebagai hanya-baca;
- langkah penyiapan yang membutuhkan kata sandi mengirimkannya di badan permintaan atau di input standar program, tidak pernah di baris perintah.

Lihat [Docker](./docker) untuk aturan-aturan ini. Kata sandi dibuat untuk setiap instalasi, dan kata sandi awal dihapus dari pengaturan kontainer setelah kontainer pertama kali berjalan. Katalog yang diunduh hanya dipakai jika tanda tangannya sah.

## Mengelola aplikasi terinstal {#manage-installed-apps}

Tab **Terinstal** menampilkan setiap aplikasi yang terinstal. Saring dengan **Semua**, **Dengan situs web**, atau **Layanan**, atau cari. Kolomnya adalah **Aplikasi**, **Versi**, **Status**, dan **Memori** (yang dipakai kontainer saat ini).

Statusnya salah satu dari **Berjalan**, **Berhenti**, **Sebagian berjalan**, **Tanpa kontainer**, **Sedang diinstal**, atau **Instalasi gagal**. Aplikasi PHP menampilkan **Di situs webnya**, atau **Situs web ditangguhkan**.

Menu setiap baris punya tindakan berikut:

| Tindakan | Untuk | Fungsinya |
|---|---|---|
| **Buka** | Aplikasi dengan situs web | Membuka situs web aplikasi. |
| **Kredensial** | Aplikasi Docker dan layanan | Menampilkan detail masuk. Setiap kali dibuka dicatat di log aktivitas. |
| **Log** | Aplikasi Docker dan layanan | Baris terbaru dari log kontainer. |
| **Jalankan**, **Hentikan**, **Mulai ulang** | Aplikasi Docker dan layanan | Menjalankan, menghentikan, atau memulai ulang kontainer. |
| **Perbarui** | Aplikasi Docker dan layanan, jika katalog punya versi yang lebih baru | Lihat di bawah. |
| **Cadangkan sekarang** | Aplikasi Docker dan layanan | Mencadangkan aplikasi. Salinannya ada di halaman **Pencadangan**: lihat [Pencadangan](./backups). |
| **Pengaturan situs web** | Aplikasi dengan situs web | Membuka pengaturan situs web. |
| **Copot** | Layanan | Menghapus layanan: lihat di bawah. |
| **Hapus bersama situs webnya** | Aplikasi dengan situs web | Membuka situs web, tempat Anda menghapusnya. |

### Memperbarui aplikasi {#update-an-app}

Jika katalog punya versi yang lebih baru dari sebuah aplikasi Docker atau layanan, kolom **Versi** menampilkannya, misalnya "2.6.0 tersedia". Klik **Perbarui** dan konfirmasi. Nefo:

1. mencadangkan aplikasi terlebih dahulu. Jika pencadangan gagal, tidak ada yang berubah;
2. mengunduh image baru sebelum menghentikan apa pun;
3. memulai ulang aplikasi dengan image tersebut dan memeriksa bahwa aplikasi merespons. Jika tidak, versi sebelumnya dijalankan lagi.

Beberapa aplikasi menolak pembaruan satu klik jika versi mayornya berubah: "Pembaruan ini (…) mengubah versi mayor dan memerlukan langkah manual. Lihat panduan upgrade aplikasi itu." Ini berlaku untuk PostgreSQL, Wiki.js, Odoo, Grafana, Gotify, Vikunja, Healthchecks, listmonk, Jellyfin, dan Mealie, serta untuk perubahan versi minor MongoDB.

Aplikasi PHP tidak diperbarui dari sini: "Perbarui dari halaman admin aplikasi itu sendiri." Untuk WordPress, gunakan [Toolkit WordPress](./wordpress).

Aplikasi yang diinstal sebelum katalog ada menampilkan **Instalasi lama**. **Konversi dan perbarui** memindahkan datanya ke folder toko aplikasi, menjalankannya dengan Docker Compose, dan memperbaruinya, setelah pencadangan.

## Mencopot aplikasi {#uninstall-an-app}

- **Layanan:** klik **Copot**, ketik namanya, dan konfirmasi. Kontainer, data, dan kata sandinya dihapus. Cadangannya dipertahankan.
- **Aplikasi dengan situs web:** hapus situs webnya: lihat [Menghapus situs web](./websites#delete-a-website). Untuk aplikasi Docker, ini menghapus kontainernya, foldernya di `/var/lib/panel-apps`, dan image yang tidak dipakai aplikasi lain. **Cadangkan situs web dulu**, yang tercentang secara bawaan di dialog penghapusan, juga mencadangkan data aplikasi, sebagai cadangan berjenis **Aplikasi**. Untuk aplikasi PHP, centang **Hapus juga basis datanya** untuk ikut menghapus basis datanya.

::: danger
Mencopot aplikasi menghapus datanya secara permanen. Cadangkan dulu dengan **Cadangkan sekarang** jika Anda mungkin membutuhkannya lagi.
:::

## Membuat salinan kedua sebuah aplikasi {#make-a-second-copy-of-an-app}

Untuk membuat salinan kedua sebuah aplikasi Docker beserta datanya, kloning situs webnya: **Situs web** → (situs) → **Ringkasan** → **Kloning…**. Lihat [Mengkloning situs web](./websites#docker-apps). n8n, Odoo, dan aplikasi yang diinstal sebelum ada Toko aplikasi tidak bisa dikloning. Aplikasi PHP dikloning bersama situs web dan basis datanya seperti situs web PHP lainnya.
