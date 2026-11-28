---
title: Pemblokiran negara dan penguncian file
description: Blokir atau izinkan pengunjung berdasarkan negara di sebuah situs web, dengan data negara gratis dari DB-IP, dan kunci kode situs web agar hanya-baca bagi PHP-nya, dengan peringatan saat file PHP berubah.
---

# Pemblokiran negara dan penguncian file

Sejak v0.24.0, Nefo punya dua perlindungan per situs web. **Pemblokiran negara** menolak pengunjung dari negara yang Anda cantumkan, atau hanya mengizinkan negara-negara itu, dengan data negara gratis dari DB-IP, di Nginx, Apache, dan OpenLiteSpeed. Penguncian file membuat kode situs web hanya-baca bagi PHP dan akun SFTP situs itu sendiri, kecuali folder yang harus ditulisinya, seperti unggahan.

Keduanya ada di pengaturan situs web, di kelompok **Keamanan**: **Situs web** → (situs) → **Pemblokiran negara** dan **Penguncian file**. Selama nonaktif, keduanya tidak membebani apa pun: tidak ada unduhan, tidak ada proses tambahan, tidak ada yang dimuat ke server web. Untuk firewall aplikasi web, batas laju, dan pemindai malware, lihat [Keamanan](./security#website-security).

## Bagaimana cara memblokir negara di sebuah situs web? {#block-or-allow-countries}

1. Buka **Situs web**, klik situs webnya, lalu buka tab **Pemblokiran negara**.
2. Pilih **Blokir negara di bawah ini** atau **Hanya izinkan negara di bawah ini**.
3. Ketik nama negara (dalam bahasa panel) atau kode dua hurufnya di kolom pencarian, lalu klik untuk menambahkannya. Hapus negara dengan tanda × pada chip-nya.
4. Klik **Simpan**.

Saat pemblokiran negara dipakai pertama kali oleh situs web mana pun di server, Nefo mengunduh basis data negara dari db-ip.com (sekitar 5 MB) sebelum menyimpan, yang butuh beberapa detik. Untuk menonaktifkannya lagi, pilih **Nonaktif: semua negara bisa masuk** dan klik **Simpan**.

Pemblokiran negara berfungsi di semua jenis situs web, termasuk situs web reverse proxy. Setiap situs web punya pengaturannya sendiri.

Pengunjung yang diblokir mendapat **403 Forbidden**. Untuk menampilkan halaman Anda sendiri, letakkan file `403.html` di folder `public` situs web dan arahkan halaman kesalahan 403 server web ke file itu di **Situs web** → (situs) → **Konfigurasi Nginx** (Nginx: `error_page 403 /403.html;`; Apache: `ErrorDocument 403 /403.html`, yang juga berfungsi di `.htaccess`; OpenLiteSpeed: blok `errorpage 403`). Pengunjung yang diblokir tetap boleh memuat `/403.html` itu sendiri.

### Apa yang tidak pernah diblokir {#what-is-never-blocked}

- Pemeriksaan Let's Encrypt (`/.well-known/acme-challenge/`), jadi sertifikat tetap diperpanjang.
- Webhook deploy Git (`/.panel/deploy/…`). Panel memeriksa tanda tangannya.
- `/403.html`.
- Alamat privat dan alamat lain yang dicadangkan (`10.0.0.0/8`, `192.168.0.0/16`, loopback, dan sebagainya).
- Permintaan dari server itu sendiri, seperti WordPress yang memanggil dirinya sendiri untuk tugas terjadwalnya.

Panel sendiri bukan situs web: panel punya port sendiri dan tidak pernah terkena pemblokiran negara. Untuk membatasi siapa yang bisa menjangkau panel, gunakan firewall (lihat [Keamanan](./security#firewall-ufw)).

### Di belakang Cloudflare {#behind-cloudflare}

Jika record DNS sebuah situs web diproksikan oleh Cloudflare, setiap permintaan datang dari alamat Cloudflare.

- **Nginx dan Apache**: untuk permintaan yang datang dari alamat Cloudflare, Nefo mencari alamat pengunjung yang dikirim Cloudflare di header `CF-Connecting-IP`. Permintaan dari tempat lain dicari berdasarkan alamatnya sendiri, jadi klien tidak bisa memalsukan header tersebut. Di Apache, log akses dan PHP situs web lalu juga melihat alamat pengunjung untuk permintaan yang lewat Cloudflare.
- **OpenLiteSpeed**: untuk permintaan dari alamat Cloudflare, Nefo memakai negara yang dikirim Cloudflare di header `CF-IPCountry`. Aktifkan **IP Geolocation** di Cloudflare (pengaturan **Network** pada zona). Tanpa itu, pengunjung yang datang lewat Cloudflare tidak diblokir, dan dengan **Hanya izinkan negara di bawah ini** mereka diizinkan masuk.

Nefo menyimpan daftar bawaan alamat Cloudflare dan memperbaruinya dari Cloudflare setiap bulan, bersama basis data negara.

### Batasan per server web {#limits-per-web-server}

| Server web | Cara kerjanya | Batasan |
|---|---|---|
| Nginx | Modul `geo` bawaan Nginx, yang hanya dimuat selama ada situs web yang memakai pemblokiran negara, dengan hanya negara-negara yang dicantumkan situs web | Tidak ada. Memori: sekitar 2 sampai 5 MB untuk satu negara, sekitar 40 MB untuk daftar besar (Amerika Serikat, Jerman, dan Jepang sekaligus) |
| Apache | Apache memeriksa rentang alamat negara yang dicantumkan satu per satu pada setiap permintaan | Paling banyak 50.000 rentang alamat per situs web, jadi negara besar ditolak (Amerika Serikat sendiri punya sekitar 270.000). Tiongkok (15.000 rentang) memakai sekitar 2,5 MB |
| OpenLiteSpeed | Pencarian GeoIP milik OpenLiteSpeed sendiri di file basis data DB-IP | Tidak ada. Di bawah 1 MB |

Satu daftar bisa berisi paling banyak 100 negara. Di Apache, tab ini menampilkan berapa rentang alamat yang dipakai situs web dari batas tersebut. Jika daftar terlalu besar, Apache menolaknya dan pesannya menyebut negara-negara yang tidak muat, dimulai dari yang terbesar, beserta jumlah rentangnya. Jangan sertakan negara tersebut, atau gunakan Nginx atau OpenLiteSpeed, yang bisa menangani semua negara.

### Data negara {#the-country-data}

Datanya adalah basis data gratis **IP to Country Lite** dari DB-IP, IP Geolocation by [DB-IP](https://db-ip.com), di bawah lisensi Creative Commons Attribution 4.0. Data ini mencakup IPv4 dan IPv6.

- Nefo mengunduhnya lewat HTTPS saat pertama kali ada situs web yang memakai pemblokiran negara.
- Sebelum mengganti salinan lama, Nefo memeriksa bahwa file tersebut adalah basis data yang lengkap: formatnya, ukurannya, dan bahwa isinya mencantumkan lebih dari 200 negara.
- Nefo memperbaruinya sebulan sekali (DB-IP menerbitkan edisi baru di awal setiap bulan). Tab ini menampilkan edisi yang sedang dipakai.
- Selama tidak ada situs web yang memakai pemblokiran negara, tidak ada yang diunduh dan server web tidak memuat apa pun.

Basis data disimpan di `/var/lib/panel-geoip/`. File ini bisa dibaca semua pengguna, karena server web membacanya: isinya data publik.

## Bagaimana cara membuat file situs web menjadi hanya-baca? {#lock-a-website-s-files}

Tab **Penguncian file** membuat kode situs web hanya-baca bagi PHP, aplikasi, dan akun SFTP situs tersebut. Plugin yang punya celah keamanan pun tidak bisa lagi mengubah file situs atau menambahkan file baru, kecuali di folder yang Anda izinkan, seperti unggahan dan cache. Panel lain menyebut fitur ini tamper protection.

1. Buka **Situs web**, klik situs webnya, lalu buka tab **Penguncian file**.
2. Periksa daftar di bawah **Folder yang bisa ditulis**. Untuk situs WordPress, Joomla, Laravel, Drupal, Nextcloud, OpenCart, Moodle, Symfony, dan CodeIgniter, panel mengisi folder milik aplikasi tersebut; **Gunakan folder milik WordPress** (atau milik aplikasi situs) mengembalikannya. Tulis satu folder per baris, relatif terhadap folder situs web, misalnya `public/wp-content/uploads`. `*` mewakili satu nama folder: `public/sites/*/files`. `tmp` dan `logs` selalu bisa ditulis.
3. Agar diberi tahu jika file PHP tetap berubah, centang **Beri tahu saya saat file PHP berubah** (lihat [di bawah](#alerts-when-php-files-change)).
4. Klik **Kunci file**.

Tab ini lalu menampilkan **Dikunci sejak** dan tanggalnya. Untuk mengubah folder yang bisa ditulis atau peringatannya nanti, edit lalu klik **Simpan**: file dikunci lagi dengan daftar yang baru. **Buka kunci file** mengembalikan semuanya kepada pengguna situs web, dengan pemilik, grup, dan izin yang persis sama seperti yang dimiliki setiap file sebelumnya.

Tab **Penguncian file** tidak ditampilkan untuk aplikasi Docker, yang file-nya milik kontainernya, maupun untuk situs web reverse proxy, yang tidak menyajikan file.

Folder bawaan:

| Aplikasi | Folder yang bisa ditulis |
|---|---|
| WordPress | `public/wp-content/uploads`, `public/wp-content/cache`, `public/wp-content/upgrade` |
| Joomla | `public/images`, `public/media`, `public/cache`, `public/administrator/cache`, `public/administrator/logs`, `public/tmp` |
| Laravel | `storage`, `bootstrap/cache` |
| Drupal | `public/sites/*/files` |
| Nextcloud | `data`, `public/config` |
| OpenCart | `storage`, `public/image/cache`, `public/image/catalog` |
| Moodle | `moodledata` |
| Symfony | `var` |
| CodeIgniter | `writable` |

Folder yang bisa ditulis tetapi belum ada akan dibuat, untuk pengguna situs web, saat Anda mengunci file, jika folder di atasnya sudah ada. Tab ini mencantumkan folder yang tidak ditemukannya.

### Apa yang berubah selama file dikunci {#what-changes-while-the-files-are-locked}

- PHP, aplikasi, dan akun SFTP situs web tidak bisa mengubah, menambah, mengganti nama, atau menghapus apa pun di luar folder yang bisa ditulis, dan juga tidak bisa mengubah izin di sana. Unggahan dan cache tetap berfungsi.
- WordPress tidak bisa memperbarui dirinya sendiri, menginstal atau memperbarui plugin dan tema dari dasbornya, atau menulis ulang `wp-config.php` maupun `.htaccess`. Perbarui dari tab **WordPress** sebagai gantinya (lihat di bawah).
- Situs tetap berfungsi: server web dan PHP tetap membaca setiap file yang bisa dibacanya sebelumnya. File yang hanya bisa dibaca pengguna situs web, seperti `wp-config.php` (mode `0600`), tetap tidak bisa dibaca server web.

Cara kerjanya: file dan folder yang dulu dimiliki pengguna situs web kini dimiliki root, tanpa izin tulis untuk siapa pun. File yang bisa dibaca grup situs web tetap memakai grup itu. File yang hanya bisa dibaca pemiliknya mendapat grup kunci milik situs web sendiri (`panel-lock-<uid>`, yang satu-satunya anggotanya adalah pengguna situs web). Tanpa modul kernel, tanpa `chattr`. Apa yang diubah oleh penguncian dicatat di `/var/lib/nefo/filelock/`, jadi pembukaan kunci memulihkannya dengan tepat, bahkan setelah crash.

### Operasi panel pada situs web yang dikunci {#panel-operations-on-a-locked-website}

| Operasi | Selama dikunci |
|---|---|
| Tab **WordPress**: **Perbarui semua**, mode pemeliharaan, mode debug, pembaruan otomatis | Panel membuka kunci file, membuat perubahan, lalu menguncinya lagi |
| Mendorong salinan staging ke situs web ini | Dibuka kuncinya untuk pendorongan, lalu dikunci lagi |
| **Deploy Git** (secara manual atau dari webhook) | Dibuka kuncinya untuk deploy, lalu dikunci lagi |
| Memulihkan cadangan situs web ini (atau folder di dalamnya) | Dibuka kuncinya untuk pemulihan, lalu dikunci lagi |
| Pemindaian malware: mengarantina atau memulihkan file | Dibuka kuncinya selama itu, lalu dikunci lagi |
| Menginstal dependensi aplikasi (Node.js, Python) | Dibuka kuncinya selama itu, lalu dikunci lagi |
| **File** (pengelola file): mengedit, mengunggah, membuat, mengganti nama, menghapus, mengekstrak, mengubah izin | Diizinkan di dalam folder yang bisa ditulis serta `tmp`, `logs`; ditolak di tempat lain, dengan pesan bahwa file situs web dikunci |
| Menginstal aplikasi (WordPress atau lainnya) ke situs web | Ditolak: buka kunci file terlebih dahulu |
| Mengimpor situs web ke situs web ini (yang kosong) | Ditolak: buka kunci file terlebih dahulu |
| Mengkloning situs web, membuat salinan staging-nya, pencadangan | Berfungsi: operasi ini hanya membaca file, termasuk yang hanya bisa dibaca situs web (seperti `wp-config.php`). Salinannya milik situs webnya sendiri dan tidak dikunci |

Setelah pendorongan, deploy, atau pemulihan, file baru ikut terkunci seperti file lainnya; file yang masuk ke folder yang bisa ditulis tetap milik situs web. Setiap operasi yang membuka kunci file menulis "Unlocking the files of example.com for this change" dan "Locking the files of example.com again" di log tugasnya. Jika penguncian ulang gagal, tugas menyebutkannya dan panel mencoba lagi setiap jam. Selama operasi berjalan, tab ini menunjukkan bahwa file sedang dibuka kuncinya untuk operasi tersebut.

::: warning
Penguncian menghalangi pengguna situs web itu sendiri, bukan root. Apa pun yang berjalan sebagai root, seperti sesi SSH sebagai root atau terminal panel, tetap bisa mengubah file. Sebelum Anda menghapus panel, jalankan `nefo unlock-files` (lihat [Mencopot Nefo](./uninstall#unlock-the-websites-files)), karena jika tidak, file situs web yang dikunci tetap dimiliki root.
:::

### Peringatan saat file PHP berubah {#alerts-when-php-files-change}

Dengan **Beri tahu saya saat file PHP berubah**, panel mengambil snapshot (checksum SHA-256 dari setiap file PHP situs web, termasuk di folder yang bisa ditulis) saat mengunci file, lalu sekali sehari membandingkan file PHP dengan snapshot itu. Jika ada file yang diubah, ditambah (file PHP di `uploads` adalah tanda klasik pembobolan), atau dihapus, panel mengirim peringatan lewat saluran yang diatur di **Pengaturan** → **Peringatan**, dan tab ini mencantumkan file-file tersebut. Panel hanya memberi peringatan lagi jika daftar perbedaannya berubah.

Jika perubahan itu Anda buat sendiri, klik **Ambil snapshot baru**. Operasi yang membuka lalu mengunci file lagi, seperti pembaruan WordPress, mengambil snapshot baru dengan sendirinya.

Pemeriksaan harian membaca paling banyak 200.000 file PHP atau 2 GB per situs web. Tab ini menyebutkan jika pemeriksaan berhenti di batas tersebut.
