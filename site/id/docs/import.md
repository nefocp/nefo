---
title: Mengimpor situs web
description: Pindahkan situs web dari cPanel, aaPanel, HestiaCP, CyberPanel, atau server lain, beserta file, basis data, sertifikat, dan tugas cron-nya.
---

# Mengimpor situs web

Nefo mengimpor situs web dari cadangan lengkap cPanel, beserta basis data dan penggunanya, sertifikat, dan tugas cron, atau dari arsip situs web dan dump SQL yang dibuat oleh aaPanel, HestiaCP, CyberPanel, atau server lain mana pun. Nefo menampilkan rencana lebih dulu, tidak mengubah apa pun di server ini sampai Anda mengonfirmasi, dan menghapus apa yang sudah dibuatnya jika impor gagal.

Ada dua cara mengimpor:

- **Cadangan lengkap cPanel**: setiap situs web dari satu akun cPanel, beserta basis data dan penggunanya, sertifikat, dan tugas cron.
- **Arsip situs web dan dump SQL**: file satu situs web sebagai arsip `.tar.gz`, `.tgz`, atau `.zip`, dengan dump `.sql` atau `.sql.gz` opsional. Gunakan cara ini untuk aaPanel, HestiaCP, CyberPanel, atau server lain mana pun.

Impor tidak pernah menyentuh server lama. Nefo lebih dulu membaca arsip dan menampilkan apa yang ditemukannya; tidak ada yang berubah di server ini sampai Anda mengonfirmasi.

## Membuat cadangan {#make-the-backup}

### cPanel {#cpanel}

Di cPanel, buka **Files → Backup** dan klik **Download a Full Account Backup**. Biarkan **Home Directory** sebagai tujuan, lalu klik **Generate Backup**. cPanel menulis `backup-M.D.YYYY_HH-MM-SS_USER.tar.gz` ke folder home Anda dan mengirimi Anda email saat cadangan siap.

Dengan akses root ke WHM, jalankan `/scripts/pkgacct USER` sebagai gantinya: perintah ini menulis `/home/cpmove-USER.tar.gz`. Kedua file punya susunan yang sama dan Nefo membaca keduanya, juga dari versi cPanel yang lama.

Cadangan parsial (unduhan "Home Directory" atau "MySQL Databases") bukan cadangan lengkap. Untuk cadangan seperti itu, gunakan **Arsip situs web dan dump SQL**.

### aaPanel {#aapanel}

1. Buka **Website**, klik **Backup** di baris situs web, lalu **Backup now**. aaPanel menulis `/www/backup/site/DOMAIN/web_DOMAIN_YYYYMMDD_HHMMSS_xxxxxx.tar.gz` (versi lama: `/www/backup/site/`).
2. Buka **Database**, klik **Backup** di baris basis data, lalu **Backup now**. aaPanel menulis `db_NAME_YYYYMMDD_HHMMSS_mysql_data.sql.gz` di bawah `/www/backup/database/`.

Arsip situs memuat folder situs web di tingkat teratasnya; Nefo mengimpor isinya. Nefo juga membaca folder lama (`/www/wwwroot/DOMAIN`) dari `.user.ini` situs dan menggantinya di basis data WordPress.

### HestiaCP, CyberPanel, dan server lain {#hestiacp-cyberpanel-and-other-servers}

Cadangan bawaan panel-panel ini mengemas setiap situs web dalam arsip berformat khusus (HestiaCP mengompresnya dengan zstd), jadi buatlah arsip biasa sebagai gantinya, lewat SSH di server lama:

```bash
# The website's files: the folder that holds index.php or index.html
tar czf /root/site.tar.gz -C /home/USER/web/example.com public_html
# Each database
mysqldump --single-transaction --routines --triggers DBNAME | gzip > /root/DBNAME.sql.gz
```

Di HestiaCP foldernya adalah `/home/USER/web/DOMAIN/public_html`, di CyberPanel `/home/DOMAIN/public_html`, dan di server biasa umumnya `/var/www/html`. Arsip yang tingkat teratasnya berupa folder `public_html` juga bisa: Nefo mengimpor isinya.

## Memindahkan arsip ke server ini {#get-the-archive-onto-this-server}

Salah satu cara berikut:

- **SFTP atau scp** ke folder milik root, misalnya `scp backup.tar.gz root@NEW-SERVER:/root/`.
- **File**: unggah arsip, atau gunakan **Unduh dari URL** (lihat [File](./files)).
- **Alamat web**: berikan alamat `http` atau `https` kepada impor. Nefo mengunduh file ke `/var/tmp/nefo-import`, di disk server ini, dan menghapusnya setelah impor. Alamat dari jaringan milik server ini sendiri ditolak.

Nefo membaca arsip dari mana saja kecuali folder sistem (`/etc`, `/proc`, `/sys`, `/dev`, `/run`, `/boot`) dan datanya sendiri. Disk membutuhkan ruang untuk file situs web dan sekitar dua kali ukuran dump; rencana impor menunjukkan berapa yang dibutuhkan dan berapa yang tersedia.

## Impor langkah demi langkah {#import-step-by-step}

1. Buka **Situs web** dan klik **Impor** di sebelah **Tambah situs web**.
2. Pilih **Cadangan lengkap cPanel** atau **Arsip situs web dan dump SQL**.
3. Masukkan path file (**Telusuri** menampilkan folder dan arsip di server ini) atau alamat webnya. Untuk arsip situs web, tambahkan dump SQL-nya.
4. Klik **Analisis**. Nefo membaca seluruh arsip sekali, tanpa mengekstraknya; cadangan besar butuh beberapa menit. **Hentikan** menghentikannya.
5. Periksa rencananya dan centang apa yang ingin dipindahkan (lihat di bawah). Klik **Impor**.
6. Ikuti log-nya. Situs web baru tetap tertutup (kesalahan 503) sampai impor selesai. **Hentikan impor** menghapus apa yang sudah dibuat sejauh ini.
7. Setelah selesai, halaman ini menampilkan situs web baru dengan **Buka situs web** dan **Kelola**, basis data, dan hal-hal yang perlu diperiksa.

Jika Anda meninggalkan halaman, impor tetap berjalan; buka lagi **Situs web** → **Impor** untuk melihatnya. Arsip yang sudah dianalisis diingat selama enam jam.

## Apa yang diimpor dari cPanel {#what-is-imported-from-cpanel}

| Di cadangan | Di Nefo |
|---|---|
| Domain utama, setiap domain addon, dan setiap subdomain | Masing-masing satu situs web. Subdomain internal cPanel untuk domain addon (`addon.main.com`) tidak disertakan. |
| Parked domain (alias) | Alias dari situs web domain utama, beserta `www.`-nya jika server lama memilikinya. |
| Setiap document root (`public_html`, `public_html/blog`, `addon.com`…) | Folder `public` situs web. Folder milik domain lain yang diimpor di dalamnya hanya masuk ke situs web domain tersebut. |
| Versi PHP (`ea-php81`) | Versi yang sama jika terinstal di sini, jika tidak, versi bawaan server ini. Situs web tanpa file PHP menjadi situs web statis. |
| `mysql/DB.sql` | Basis data dengan nama yang sama jika nama itu masih tersedia di sini, jika tidak, `NAME_2`, yang tertaut ke situs web yang memakainya. |
| Pengguna basis data dan kata sandinya (`mysql.sql`) | Setiap basis data mendapat satu pengguna: pengguna yang disebut di `wp-config.php` WordPress-nya, jika tidak, pengguna yang punya akses ke basis data tersebut. Pengguna mempertahankan kata sandi lamanya: MariaDB menerima hash kata sandi lama apa adanya, atau Nefo memakai kata sandi dari `wp-config.php`. Jika tidak, pengguna mendapat kata sandi baru. |
| WordPress | Dikenali dari `wp-config.php`-nya. Nefo menandai situs web sebagai WordPress, menerapkan aturan rewrite WordPress, mengarahkan `wp-config.php` ke nama basis data, pengguna, atau kata sandi yang baru jika berubah, dan mengganti folder lama (`/home/USER/public_html`) dengan yang baru di basis data. |
| Sertifikat (`apache_tls/`, `sslcerts/`, `ssl/`) | Diinstal sebagai sertifikat Anda sendiri jika Anda mencentangnya: hanya jika kuncinya ada di cadangan, sertifikat masih berlaku hari ini, dan mencakup setiap nama situs web. Jika tidak, dapatkan sertifikat dari Let's Encrypt setelah impor. |
| Tugas cron (`cron/USER`) | Ditampilkan, dan hanya diinstal jika Anda mencentangnya: sebagai pengguna milik situs web (tidak pernah root), dengan document root lama dan program PHP cPanel diganti dengan milik server ini. Perintah yang masih menyebut folder home lama akan ditandai. |
| `.htaccess`, `.user.ini` | Folder lama diganti dengan yang baru, dan baris handler PHP cPanel dihapus dari `.htaccess`. |

Rencana impor menyebutkan alasan sebuah domain tidak bisa diimpor, misalnya jika sudah ada situs web di sini yang memakai domain tersebut.

## Apa yang tidak diimpor {#what-is-not-imported}

- **Email**: kotak surat, forwarder, autoresponder, dan filter. Nefo tidak punya server email. Pindahkan email ke layanan email sebelum Anda mematikan server lama. Untuk email yang dikirim situs web, lihat [Email dan peringatan](./mail-alerts).
- **Zona DNS**: Nefo tidak menjalankan server DNS. Arahkan domain ke server ini di tempat DNS-nya dikelola, atau dengan [DNS (Cloudflare)](./dns).
- **Akun FTP**: situs web di sini memakai SFTP, satu akun per situs web (tab **SFTP** situs web).
- Milis, basis data PostgreSQL, file di folder home di luar document root, dan pengaturan cPanel itu sendiri.

## Arsip situs web {#website-archives}

- Satu folder di tingkat teratas arsip (seperti di cadangan aaPanel) tidak dipertahankan: isinya yang menjadi situs web. Folder home yang berisi `public_html` juga bisa.
- Situs webnya bisa situs baru (domain, `www.`, dan versi PHP-nya akan ditanyakan) atau situs yang sudah ada dan kosong: situs web PHP atau statis yang folder `public`-nya hanya berisi halaman sambutan, dan yang file-nya tidak [dikunci](./website-protection#lock-a-website-s-files).
- Setiap dump SQL menjadi basis data baru dengan pengguna bernama sama dan kata sandi baru. `wp-config.php` WordPress diarahkan ke dump yang disebutkannya (atau ke satu-satunya dump). Untuk aplikasi lain, ubah konfigurasinya ke basis data, pengguna, dan kata sandi baru (**Tampilkan login** di halaman **Basis data**).

## Dump dari MySQL 8 {#dumps-from-mysql-8}

Dump yang dibuat MySQL 8, seperti yang dijalankan aaPanel dan banyak server cPanel, memakai collation yang tidak dikenal MariaDB, seperti `utf8mb4_0900_ai_ci`, dan baris yang ditolak MariaDB. Nefo menyesuaikannya saat dump dialirkan masuk, tanpa mengubah data Anda:

- Collation MySQL 8 menjadi collation MariaDB yang paling mendekati (`utf8mb4_unicode_520_ci`, `utf8mb4_bin` untuk yang peka huruf besar-kecil, atau collation bahasanya sendiri, seperti `utf8mb4_vietnamese_ci`).
- Pengaturan yang hanya dikenal MySQL 8, seperti `DEFAULT ENCRYPTION='N'` dan `SRID`, menjadi komentar biasa.
- `SET @@GLOBAL.GTID_PURGED` dan baris `SQL_LOG_BIN` di sekitarnya tidak disertakan.
- `CREATE DATABASE` dan `USE` untuk basis data milik dump itu sendiri tidak disertakan, sehingga dump masuk ke basis data baru.

Teks di dalam data Anda tidak pernah diubah. Log menyebutkan berapa baris yang disesuaikan.

## Bagaimana Nefo memperlakukan arsip {#how-nefo-treats-archives}

Arsip dari server lain bisa berisi apa saja, jadi Nefo memeriksa setiap entri:

- Nama dengan `..` atau path absolut, tautan simbolik, hard link, perangkat, dan FIFO tidak disertakan, dan log menyebutkan jumlahnya.
- File kehilangan setuid dan setgid, serta akses tulis untuk grup dan pihak lain. Semuanya dimiliki oleh pengguna Linux milik situs web.
- File hanya ditulis melalui folder situs web: tidak ada yang bisa mendarat di luarnya.
- File di atas 8 GB tidak disertakan; arsip yang hasil ekstraknya jauh lebih besar daripada ukurannya sendiri (bom arsip) ditolak.
- SQL dijalankan sebagai akun basis data sementara yang hanya boleh memakai basis data baru, tidak pernah sebagai root.

Jika impor gagal atau dihentikan, Nefo menghapus situs web dan basis data yang dibuatnya. Situs web yang tidak bisa dihapus tetap ditangguhkan, dengan catatan bahwa impor tidak selesai.

## Setelah impor {#after-the-import}

1. **Uji sebelum DNS**: arahkan domain ke server ini di file `hosts` komputer Anda dan buka situs webnya.
2. **DNS**: arahkan record A (dan AAAA) setiap domain ke server ini.
3. **HTTPS**: setelah DNS mengarah ke sini, dapatkan sertifikat Let's Encrypt di tab **SSL / HTTPS** setiap situs web. Sertifikat yang diimpor tidak diperpanjang oleh Nefo. Lihat [Sertifikat SSL](./ssl).
4. **PHP**: periksa versi dan pengaturan PHP setiap situs web di tab **PHP**-nya.
5. **Tugas cron**: tambahkan yang tidak Anda impor di halaman **Tugas cron**.
6. **Email**: pastikan email berfungsi di tempat lain sebelum Anda mematikan server lama.

## Jika terjadi masalah {#if-something-goes-wrong}

- **"bukan cadangan lengkap cPanel"**: arsip tidak punya `userdata/main`. Arsip itu cadangan parsial atau arsip biasa: pilih **Arsip situs web dan dump SQL**.
- **"memakai basis data lain"**: dump menulis ke basis data kedua. Ekspor setiap basis data secara terpisah, tanpa `--databases` atau `--all-databases`.
- **"ruang disk kosong tidak cukup"**: kosongkan sebagian ruang, atau impor lebih sedikit situs web sekaligus.
- **Situs WordPress menampilkan "Error establishing a database connection"**: buka `wp-config.php`-nya di **File** dan bandingkan `DB_NAME`, `DB_USER`, dan `DB_PASSWORD` dengan **Tampilkan login** di halaman **Basis data**.
