---
title: Basis data
description: Instal MariaDB atau MySQL, pilih versi MariaDB, tingkatkan versi MariaDB, beralih antara MariaDB dan MySQL, buat basis data, impor dan ekspor SQL, izinkan akses jarak jauh, baca log kueri lambat, serta gunakan Adminer, phpMyAdmin, dan penjelajah kunci Redis.
---

# Basis data

Nefo menjalankan satu server basis data yang kompatibel dengan MySQL di setiap server: MariaDB (dari distribusi Anda, atau 11.4, 11.8, atau 12.3 LTS dari MariaDB) atau MySQL 8.4 LTS, disetel untuk VPS kecil dan hanya bisa dijangkau dari server itu sendiri. Halaman **Basis data** membuat basis data dan pengguna, mengimpor dan mengekspor SQL, membuka satu basis data untuk alamat yang Anda pilih, dan menampilkan kueri lambat. PostgreSQL bisa berjalan di sampingnya.

Halaman ini membahas:

- memilih MariaDB atau MySQL dan versi MariaDB, menginstalnya, cara Nefo menyetelnya untuk server kecil, meningkatkan versi MariaDB, dan beralih dari yang satu ke yang lain;
- membuat basis data, menampilkan login-nya, dan mengganti kata sandi;
- impor dan ekspor SQL;
- akses jarak jauh untuk satu basis data dalam satu waktu;
- log kueri lambat;
- Adminer dan phpMyAdmin, antarmuka basis data di browser Anda;
- penjelajah kunci Redis, di tab **Redis** pada halaman yang sama, dan status Memcached.

PostgreSQL punya tab **PostgreSQL** sendiri di halaman ini dan halaman dokumentasi sendiri: lihat [PostgreSQL](./postgresql). MongoDB, dan PostgreSQL sebagai kontainer, tersedia di [Toko aplikasi](./apps) sebagai aplikasi Docker yang hanya mendengarkan di 127.0.0.1. Aplikasi Docker tersebut tidak muncul di halaman **Basis data**.

## Memilih MariaDB atau MySQL {#choose-mariadb-or-mysql}

Satu server menjalankan satu server basis data yang kompatibel dengan MySQL: MariaDB atau MySQL 8.4 LTS. Anda memilihnya saat menginstal, di halaman **Basis data**, di kartu **Server basis data** pada halaman **Perangkat lunak**, atau di penyiapan (**Siapkan server ini**). Selama salah satunya terinstal, panel menolak menginstal yang lain; nanti Anda bisa [beralih](#switch-between-mariadb-and-mysql) dari yang satu ke yang lain, beserta basis data Anda.

| | MariaDB (disarankan) | MySQL 8.4 LTS |
|---|---|---|
| Berasal dari | Paket distribusi Anda, atau seri LTS dari repositori milik MariaDB sendiri (lihat [Versi MariaDB](#mariadb-versions)) | Repositori Oracle, `repo.mysql.com` (jalur `mysql-8.4-lts`) |
| Memori, kira-kira | Buffer pool-nya ditambah sekitar 100 MB | Buffer pool-nya ditambah sekitar 150 MB, atau ditambah sekitar 300 MB jika performance schema aktif, yang diaktifkan panel mulai dari RAM sekitar 3,5 GB (jadi di server 4 GB) |
| Dukungan | Selama distribusi Anda mendukungnya; seri dari repositori MariaDB sampai tanggal akhir yang ditetapkan MariaDB untuk seri itu | Oracle mendukung 8.4 LTS hingga 2032 |
| Sistem | Setiap sistem yang didukung Nefo | Ubuntu 22.04, 24.04, dan 26.04 serta Debian 12 dan 13, hanya di amd64 (x86-64) |
| Cocok untuk | WordPress dan sebagian besar aplikasi PHP | Aplikasi yang membutuhkan MySQL itu sendiri, atau tim yang terbiasa dengannya |

Keduanya bekerja dengan semua hal di halaman ini, dengan pencadangan, salinan situs web, staging WordPress, dan aplikasi PHP dari toko aplikasi. Adminer dan phpMyAdmin bisa membuka keduanya. Di server arm64, atau sistem yang tidak punya paket dari Oracle, halaman **Perangkat lunak** hanya menawarkan MariaDB dan menyebutkan alasannya.

## Menginstal MariaDB {#install-mariadb}

1. Buka **Basis data**.
2. Pilih **Instal MariaDB (disarankan)**. Anda juga bisa menginstalnya di halaman **Perangkat lunak**: biarkan **MariaDB** terpilih di kartu **Server basis data** dan pilih **Instal MariaDB**.
3. Tunggu satu sampai dua menit. Log progres menampilkan setiap langkah.

Nefo menginstal paket `mariadb-server` dan `mariadb-client` dari distribusi Anda. Nefo lalu menulis pengaturannya, menjalankan MariaDB, dan menghapus pengguna anonim serta basis data `test` yang disertakan di beberapa versi.

Untuk menginstal seri yang lebih baru dari repositori milik MariaDB sendiri, lihat [Versi MariaDB](#mariadb-versions).

### Cara MariaDB disetel {#how-mariadb-is-tuned}

Nefo menulis pengaturannya ke `/etc/mysql/mariadb.conf.d/90-panel.cnf`. Pengaturan memori terbesar, InnoDB buffer pool, bergantung pada RAM server:

| RAM server | `innodb_buffer_pool_size` |
|---|---|
| Kurang dari 1,5 GB | 128M |
| 1,5 GB atau lebih | 256M |
| 3 GB atau lebih | 512M |
| 7 GB atau lebih | 1G |

Pengaturan lainnya sama di setiap server:

| Pengaturan | Nilai | Alasan |
|---|---|---|
| `bind-address` | `127.0.0.1` | MariaDB hanya menerima koneksi dari server ini |
| `skip-name-resolve` | aktif | Tidak ada pencarian DNS untuk setiap koneksi |
| `performance_schema` | `OFF` | Menghemat memori |
| `max_connections` | `100` | |
| `character-set-server` | `utf8mb4` | Unicode lengkap, termasuk emoji |
| `collation-server` | `utf8mb4_unicode_ci` | |

MariaDB lalu memakai memori sekitar ukuran buffer pool-nya ditambah 100 MB. Halaman **Perangkat lunak** menampilkan perkiraan ini.

::: warning
Jangan edit `90-panel.cnf`. Panel menulis ulang file itu.
:::

## Versi MariaDB mana yang bisa saya instal? {#mariadb-versions}

Secara bawaan, MariaDB berasal dari distribusi Anda. Perbaikan keamanannya lalu datang bersama pembaruan sistem dan tidak ada repositori lain yang ditambahkan. Sebagai gantinya, Anda bisa menginstal seri dukungan jangka panjang (LTS) dari repositori milik MariaDB sendiri, `dlm.mariadb.com`, jika Anda membutuhkan MariaDB yang lebih baru daripada milik distribusi Anda:

| Seri | Didukung MariaDB sampai | Ditawarkan di |
|---|---|---|
| 11.4 LTS | 29 Mei 2029 | Ubuntu 22.04 dan 24.04, Debian 12 |
| 11.8 LTS | 4 Juni 2028 | Ubuntu 22.04 dan 24.04, Debian 12 |
| 12.3 LTS | 12 Juni 2029 | Ubuntu 22.04, 24.04, dan 26.04, Debian 12 dan 13 |

Sebuah seri hanya ditawarkan jika lebih baru daripada MariaDB milik distribusi:

| Sistem | MariaDB milik distribusi | Seri yang ditawarkan |
|---|---|---|
| Ubuntu 22.04 | 10.6 | 11.4, 11.8, 12.3 |
| Ubuntu 24.04 | 10.11 | 11.4, 11.8, 12.3 |
| Debian 12 | 10.11 | 11.4, 11.8, 12.3 |
| Ubuntu 26.04 | 11.8 | 12.3 |
| Debian 13 | 11.8 | 12.3 |

MariaDB 12.3 tidak menginstal Galera (perangkat lunak cluster MariaDB, yang tidak dipakai server tunggal); 11.4 dan 11.8 ikut membawanya karena paketnya mengharuskannya.

Untuk menginstal sebuah seri:

1. Buka **Perangkat lunak**. Di kartu **Server basis data**, biarkan **MariaDB** terpilih.
2. Di bawahnya, pilih versinya, seperti **11.4 LTS dari MariaDB**. Versi milik distribusi Anda, seperti "10.11, milik distribusi", tetap menjadi bawaan.
3. Pilih **Instal MariaDB 11.4**.

Penyiapan (**Siapkan server ini**) menawarkan versi yang sama di bawah **MariaDB**, begitu pula `nefo setup --db mariadb-11.4` (lihat [Baris perintah](./cli#nefo-setup)).

Yang dilakukan panel untuk seri dari repositori MariaDB:

- Panel mengunduh kunci penandatanganan MariaDB dari `https://supplychain.mariadb.com/MariaDB-Server-GPG-KEY` (atau `keyserver.ubuntu.com`) dan hanya menerimanya jika fingerprint-nya persis `177F 4010 FE56 CA33 3630 0305 F165 6F24 C74C D1D8`, yaitu "MariaDB Signing Key" yang tercantum di dokumentasi MariaDB. Panel menyimpannya sebagai `/etc/apt/keyrings/panel-mariadb.gpg`, yang dipercaya hanya untuk repositori ini.
- Panel menulis `/etc/apt/sources.list.d/panel-mariadb.list` dengan satu baris hanya untuk seri itu, seperti `https://dlm.mariadb.com/repo/mariadb-server/11.4/repo/ubuntu noble main`. Server Anda lalu mendapat rilis minor 11.4 bersama pembaruan sistem, tidak pernah seri yang lebih baru dengan sendirinya.
- Panel memeriksa bahwa apt benar-benar akan menginstal paket MariaDB dari seri itu, lalu menginstal `mariadb-server` dan `mariadb-client`, disetel seperti MariaDB milik distribusi.

Mencopot MariaDB juga menghapus file repositori dan kuncinya.

Panel menolak:

- menginstal sebuah seri selagi MariaDB lain terinstal: gunakan **Tingkatkan versi** (lihat di bawah);
- menginstal MariaDB yang lebih lama daripada versi yang datanya ada di `/var/lib/mysql`, sisa instalasi sebelumnya, karena MariaDB itu tidak bisa membacanya. Instal versi tersebut atau yang lebih baru, atau pindahkan direktori data itu terlebih dahulu.

## Meningkatkan versi MariaDB {#upgrade-mariadb}

Anda bisa meningkatkan MariaDB yang terinstal ke seri LTS yang lebih baru dari repositori MariaDB, seperti 10.11 milik distribusi ke 11.4, atau 11.8 milik Debian 13 ke 12.3, dengan basis data Anda tetap di tempatnya.

1. Buka **Perangkat lunak**. Di kartu **Server basis data**, pilih **Tingkatkan versi**.
2. Pilih serinya. Dialog menampilkan apa yang akan terjadi, dan berapa ruang disk kosong yang dibutuhkan.
3. Pilih **Tingkatkan ke MariaDB 11.4** dan ikuti log-nya.

Peningkatan versi berjalan dengan urutan ini:

1. Setiap basis data diekspor, masing-masing satu file SQL yang di-gzip, ke folder baru di `/var/backups/panel/db-server/`. Panel menyimpan dump ini sampai Anda menghapusnya.
2. Paket seri baru diunduh, dan paket seri yang terinstal tetap disimpan, jadi kembali ke versi lama bisa dilakukan tanpa jaringan.
3. MariaDB menjadi hanya-baca, panel menghitung baris setiap tabel, lalu MariaDB berhenti. Direktori datanya disalin ke `/var/lib/mysql.before-11.4-<date>-<time>` dan `/etc/mysql` ke folder dump. Mulai dari sini, situs web tidak bisa memakai basis datanya sampai peningkatan versi selesai, biasanya beberapa menit.
4. Paket baru menggantikan paket lama dan menjalankan MariaDB di atas data yang sama, masih hanya-baca. `mariadb-upgrade` lalu memperbarui tabel sistemnya.
5. Panel memeriksa bahwa MariaDB melaporkan versi baru, bahwa setiap basis data punya tabel yang sama dengan jumlah baris, view, routine, trigger, dan event yang sama, dan bahwa setiap pengguna basis data milik panel bisa masuk dengan kata sandinya.
6. MariaDB menerima penulisan lagi, dan salinan direktori data dihapus.

Jika ada langkah dari langkah 4 dan seterusnya yang gagal, panel mengembalikan MariaDB yang lama: panel menghapus paket baru, memulihkan `/etc/mysql` dan direktori data yang disalin, menginstal lagi paket lama, dan menjalankan MariaDB. Log diakhiri dengan alasannya. Basis data Anda lalu kembali seperti sebelum peningkatan versi.

::: warning Tidak bisa kembali ke seri yang lebih lama
Setelah peningkatan versi, MariaDB sudah memperbarui file data dan tabel sistemnya, yang tidak bisa dibaca MariaDB yang lebih lama. Panel menolak penurunan versi, seperti 11.8 ke 11.4, dan menyebutkan alasannya. Untuk kembali, ekspor basis data, copot MariaDB beserta datanya, instal versi yang lebih lama, lalu impor basis datanya.
:::

Panel membutuhkan ruang disk kosong untuk dump, salinan direktori data, dan paket: kira-kira satu setengah kali direktori data ditambah 400 MB. Tanpa ruang itu, panel menolak memulai. Panel juga menolak selama MariaDB berhenti, atau jika konfigurasi server memindahkan direktori data dari `/var/lib/mysql`.

## Menginstal MySQL {#install-mysql}

1. Buka **Basis data** dan pilih **Instal MySQL 8.4 sebagai gantinya**. Di halaman **Perangkat lunak**, pilih **MySQL 8.4 LTS** di kartu **Server basis data** dan pilih **Instal MySQL 8.4 LTS**.
2. Tunggu dua sampai empat menit. Log progres menampilkan setiap langkah.

Nefo menambahkan sendiri repositori Oracle, alih-alih menginstal paket `mysql-apt-config` milik Oracle:

- Nefo mengunduh kunci penandatanganan dari `https://repo.mysql.com/RPM-GPG-KEY-mysql-2025` dan hanya menerimanya jika fingerprint-nya persis `BCA4 3417 C3B4 85DD 128E C6D4 B7B3 B788 A8D3 785C`, yaitu kunci MySQL Release Engineering yang tercantum di dokumentasi MySQL. Nefo menyimpan kunci itu sebagai `/etc/apt/keyrings/panel-mysql.gpg`.
- `/etc/apt/sources.list.d/panel-mysql.list` memercayai kunci itu hanya untuk repositori ini, dan hanya mencantumkan jalur `mysql-8.4-lts`.
- Oracle memperbarui kunci itu dari waktu ke waktu; salinan saat ini kedaluwarsa pada 23 Oktober 2027. Seminggu sekali panel mencari salinan terbaru dari kunci yang sama dan mengganti yang lama, sehingga `apt-get update` tetap berfungsi.

Lalu Nefo menginstal `mysql-community-server` dan `mysql-community-client` tanpa pertanyaan. Akun `root` MySQL tidak punya kata sandi: akun ini hanya masuk lewat socket lokal (`auth_socket`), sebagai root di server, dan panel tidak menyimpan kata sandi root. Terakhir, panel menghapus pengguna anonim dan basis data `test`.

### Cara MySQL disetel {#how-mysql-is-tuned}

Nefo menulis `/etc/mysql/mysql.conf.d/90-panel.cnf` sebelum paket pertama kali menjalankan MySQL, jadi MySQL tidak pernah berjalan dengan pengaturan bawaan Oracle di server Anda. Pengaturan memori bergantung pada RAM server:

| RAM server | `innodb_buffer_pool_size` | `max_connections` | `table_open_cache` | `temptable_max_ram` |
|---|---|---|---|---|
| Kurang dari 1,5 GB | 128M | 75 | 400 | 32M |
| 1,5 GB atau lebih | 256M | 100 | 600 | 64M |
| 3 GB atau lebih | 512M | 150 | 1000 | 128M |
| 7 GB atau lebih | 1024M | 200 | 2000 | 256M |

Pengaturan lainnya:

| Pengaturan | Nilai | Alasan |
|---|---|---|
| `bind-address` | `127.0.0.1` | MySQL hanya menerima koneksi dari server ini, sampai ada basis data yang mengizinkan akses jarak jauh |
| `mysqlx` | `OFF` | Tidak ada pendengar X Protocol di port 33060: PHP dan panel memakai protokol klasik |
| `disable_log_bin` | aktif | Tidak ada binary log yang memenuhi disk: server tidak punya replika |
| `performance_schema` | `OFF` di bawah RAM sekitar 3,5 GB, `ON` mulai dari situ (jadi di server 4 GB) | Memakan 100 sampai 200 MB sendiri |
| `skip-name-resolve`, `host_cache_size` | aktif, `0` | Tidak ada pencarian DNS untuk setiap koneksi |
| `innodb_log_buffer_size` | `16M` | Bawaan MySQL 8.4 adalah 64M |
| `character-set-server`, `collation-server` | `utf8mb4`, `utf8mb4_unicode_ci` | Sama seperti MariaDB, jadi dump bisa dipindahkan antara keduanya |

MySQL lalu memakai memori sekitar ukuran buffer pool-nya ditambah 150 MB, atau ditambah 300 MB jika performance schema aktif. Halaman **Perangkat lunak** menampilkan perkiraan ini; halaman **Layanan** menampilkan pemakaian sebenarnya.

Pengguna masuk dengan `caching_sha2_password`, bawaan MySQL 8.4, yang didukung `mysqli` dan PDO di PHP. `mysql_native_password` tetap nonaktif, seperti saat MySQL 8.4 dirilis: metode ini membuat hash dengan SHA-1 tanpa salt, dan MySQL 9 sudah menghapusnya. Jadi hash kata sandi lama dari server lain, seperti yang disimpan cPanel, tidak bisa dipakai ulang di MySQL: basis data yang dipindahkan membutuhkan kata sandinya, atau kata sandi baru.

Di MySQL, panel menjalankan SQL dengan perintah `mysql` alih-alih `mariadb`, mengekspor dengan `mysqldump`, dan layanannya bernama `mysql`.

::: warning
Jangan edit `90-panel.cnf`. Panel menulis ulang file itu.
:::

## Bisakah saya beralih antara MariaDB dan MySQL? {#switch-between-mariadb-and-mysql}

Bisa, mulai v0.24.0. Satu server menjalankan satu server basis data, tetapi Anda bisa mengganti MariaDB dengan MySQL 8.4 LTS, atau MySQL dengan MariaDB, beserta basis data, penggunanya, dan hak aksesnya.

1. Buka **Perangkat lunak**. Di kartu **Server basis data**, di bawah **MySQL 8.4 LTS sebagai gantinya** (atau **MariaDB sebagai gantinya**), pilih **Beralih ke MySQL 8.4 LTS** (atau **Beralih ke MariaDB**).
2. Baca dialognya. Dialog mencantumkan basis data dan akun yang ikut dipindahkan, dan cara setiap akun masuk sesudahnya.
3. Ketik `mysql` (atau `mariadb`) untuk konfirmasi, lalu pilih tombol peralihan.

Beralih kembali ke MariaDB menginstal MariaDB milik distribusi Anda. Tanpa basis data apa pun, tugas yang sama hanya mengganti server yang satu dengan yang lain.

### Yang dilakukan peralihan {#what-the-switch-does}

1. Panel mengunduh paket server baru, dan menyimpan paket server lama untuk kembali. Belum ada yang berubah bagi situs web Anda.
2. Server lama menjadi hanya-baca: situs web masih bisa membaca basis datanya, tetapi tidak bisa menulis. Panel membaca akun dan hak aksesnya, menghitung baris setiap tabel, dan mengekspor setiap basis data ke folder baru di `/var/backups/panel/db-server/`.
3. Server lama berhenti. Direktori datanya dipindahkan ke `/var/lib/mysql.mariadb-<date>-<time>` (atau `.mysql-…`) dan `/etc/mysql` disalin ke folder dump. Paketnya dihapus. Mulai dari sini, situs web tidak bisa memakai basis datanya sampai peralihan selesai.
4. Server baru diinstal seperti halaman **Perangkat lunak** menginstalnya, dan tetap hanya-baca. Akses jarak jauh dan log kueri lambat disiapkan lagi seperti sebelumnya.
5. Akun dibuat, setiap basis data diimpor, dan hak aksesnya diberikan lagi. Dump dari MySQL 8 disesuaikan untuk MariaDB seperti yang dilakukan impor situs web: collation `utf8mb4_0900_*`-nya menjadi milik MariaDB (`utf8mb4_unicode_520_ci`, collation khusus sebuah bahasa, atau `utf8mb4_bin`) dan pengaturan yang hanya ada di MySQL dilewati. Dump dari MariaDB 11 yang masuk ke MySQL mendapat collation MySQL sebagai pengganti collation `uca1400` milik MariaDB (`utf8mb4_uca1400_ai_ci` menjadi `utf8mb4_0900_ai_ci`).
6. Panel memeriksa bahwa setiap basis data punya tabel yang sama dengan jumlah baris, view, routine, trigger, dan event yang sama, dan bahwa setiap pengguna basis data milik panel bisa masuk dengan kata sandinya. Lalu server menerima penulisan lagi dan direktori data lama dihapus.

Jika ada langkah dari langkah 3 dan seterusnya yang gagal, panel menghapus server baru, memulihkan konfigurasi dan direktori data server lama, menginstal lagi paketnya, dan menjalankannya: Anda kembali ke keadaan semula. Dump tetap ada dalam kedua kasus.

Lamanya bergantung pada ukuran basis data Anda: ekspor, impor, dan penghitungan baris masing-masing membaca seluruh data satu kali.

### Akun dan kata sandi {#accounts-and-passwords}

MariaDB dan MySQL menyimpan kata sandi dengan cara berbeda, jadi tidak setiap kata sandi bisa ikut dipindahkan:

| Akun | Setelah peralihan |
|---|---|
| Pengguna basis data di halaman **Basis data** | Kata sandinya, yang diketahui panel, di `localhost` dan di setiap alamat yang diizinkan untuk akses jarak jauh |
| Akun yang Anda buat sendiri dengan kata sandi `mysql_native_password`, yang pindah ke MariaDB | Kata sandinya, dari hash-nya |
| Akun yang masuk lewat socket (`unix_socket` di MariaDB, `auth_socket` di MySQL) | Masuk lewat socket lagi |
| Akun lain yang Anda buat sendiri, seperti akun MariaDB yang pindah ke MySQL (MySQL 8.4 menonaktifkan `mysql_native_password`) atau akun MySQL yang pindah ke MariaDB (MariaDB tidak bisa membaca `caching_sha2_password`) | Dibuat dengan kata sandi acak baru dan dikunci. Log mencantumkan masing-masing akun. Atur kata sandinya sebagai root di server: `ALTER USER 'name'@'localhost' IDENTIFIED BY '…' ACCOUNT UNLOCK;` |

Hak aksesnya diberikan lagi berdasarkan `SHOW GRANTS` dari server lama. Hak akses yang tidak dimiliki server lain dilewati dan dicantumkan di log, begitu pula pemberian role dan proxy, yang tidak dibawa panel.

### Yang tetap berfungsi {#what-keeps-working}

- Situs web mempertahankan pengaturannya: situs web terhubung ke `localhost`, lewat socket yang sama di kedua server.
- phpMyAdmin dan Adminer membuka server baru.
- Pencadangan: cadangan baru memakai program ekspor server baru, dan cadangan yang dibuat sebelum peralihan bisa dipulihkan sesudahnya.
- Akses jarak jauh tetap terbuka untuk alamat yang sama.

### Kapan panel menolak {#when-the-panel-refuses}

Dialog mencantumkan apa yang harus diperbaiki lebih dulu, di bawah **Perbaiki ini terlebih dahulu:**, dan tombol peralihan tetap nonaktif:

| Alasan | Yang harus dilakukan |
|---|---|
| Panel tidak tahu kata sandi pengguna sebuah basis data (basis data yang dipindahkan hanya dengan hash kata sandinya) | Atur kata sandi baru untuk basis data itu di halaman **Basis data**, dan masukkan ke pengaturan situs web |
| Pindah ke MySQL: tabel yang memakai salah satu storage engine milik MariaDB sendiri (Aria dan lainnya), sequence, atau tabel system-versioned | Konversikan, misalnya dengan `ALTER TABLE … ENGINE=InnoDB`, atau hapus |
| Pindah ke MySQL: kolom `INET4`, `INET6`, atau `UUID` | Ubah, misalnya menjadi `VARCHAR` |
| Pindah ke MySQL: view, trigger, atau routine yang nama pengguna definer-nya lebih dari 32 karakter | Buat ulang dengan definer lain |
| Nama basis data yang diawali tanda hubung | Ganti nama basis data |
| Ruang disk kosong tidak cukup: dump dan data yang diimpor di samping direktori data lama membutuhkan kira-kira dua kali ukuran basis data Anda ditambah 700 MB | Kosongkan sebagian ruang; hapus dump lama |

Peralihan juga ditolak selama server berhenti, jika konfigurasi server memindahkan direktori data dari `/var/lib/mysql`, dan, untuk MySQL, di tempat MySQL tidak ditawarkan (arm64, atau sistem yang tidak punya paket dari Oracle).

Beberapa perbedaan baru terlihat saat data diimpor, seperti indeks yang ditandai `IGNORED` oleh MariaDB, yang tidak dikenal MySQL. Impor lalu gagal dan panel kembali ke server lama, dengan pesan kesalahannya di log.

## Dump dari peningkatan versi dan peralihan {#dumps-from-upgrades-and-switches}

Kartu **Server basis data** di halaman **Perangkat lunak** mencantumkan folder-foldernya di bawah **Dump yang disimpan**: kapan, apa (seperti "MariaDB 10.11.14 → MariaDB 11.4"), apakah hasilnya **Selesai**, **Dikembalikan**, atau **Gagal**, berapa basis data, ukurannya, dan foldernya. Setiap basis data berupa satu file, `001.sql.gz`, `002.sql.gz`, dan seterusnya, dengan `manifest.json` yang menyebut basis data setiap file, karena nama basis data bisa memuat karakter yang tidak bisa dipakai nama file. Folder ini hanya bisa dibaca root.

Untuk mengembalikan satu basis data, ekstrak file-nya dan impor di halaman **Basis data**. Untuk menghapus sebuah folder, pilih **Hapus dump ini** (ikon tempat sampah di sebelahnya). Panel menolak selama peningkatan versi atau peralihan sedang berjalan.

## Membuat basis data {#create-a-database}

1. Buka **Basis data** dan pilih **Tambah basis data**.
2. Masukkan nama di **Basis data**. Pengguna dengan nama yang sama akan dibuat, dan pengguna itu hanya bisa memakai basis data ini.
3. Kosongkan **Kata sandi** untuk mendapatkan kata sandi acak yang kuat, atau ketik kata sandi Anda sendiri.
4. Opsional: pilih situs di **Situs web**. Cadangan situs web itu lalu menyertakan basis data ini.
5. Pilih **Buat basis data**.

Sebuah kartu berjudul **Detail koneksi untuk** diikuti nama basis data lalu menampilkan **Basis data**, **Pengguna**, **Kata sandi**, dan **Host**. Host-nya selalu `localhost`.

Aturan untuk nama dan kata sandi:

- Nama diawali huruf dan hanya memakai `a-z`, `0-9`, dan `_`, paling banyak 32 karakter. Nama-nama ini dicadangkan: `mysql`, `information_schema`, `performance_schema`, `sys`, `test`, `root`, `mariadb`, `phpmyadmin`, `debian_sys_maint`. Panel juga menolak nama yang sudah dipakai basis data atau pengguna basis data di MariaDB.
- Kata sandi terdiri dari 12 sampai 64 karakter, berupa huruf, angka, dan `! # % * + , . : = ? @ ^ _ ~ -`. Kata sandi yang dibuat panel terdiri dari 24 karakter.

Basis data baru memakai `utf8mb4` dan `utf8mb4_unicode_ci`.

### Pengguna dan hak akses {#users-and-privileges}

Setiap basis data punya satu pengguna, `name@localhost`, dengan semua hak akses pada basis data itu dan tidak ada pada basis data lain. Jika akses jarak jauh aktif, pengguna itu juga mendapat salinan untuk setiap alamat yang diizinkan (lihat [Mengizinkan akses jarak jauh](#allow-remote-access)). Selain pengguna sementara yang menjalankan impor (lihat [Impor](#import)), panel tidak membuat pengguna tambahan atau mengubah hak akses. Jika Anda membutuhkannya, gunakan perintah `mariadb` (`mysql` di MySQL) di server sebagai root.

Daftar ini menampilkan basis data yang dibuat panel. Basis data yang dihapus di luar panel ditandai **Tidak ditemukan di MariaDB** (atau **Tidak ditemukan di MySQL**).

## Menampilkan login basis data {#show-a-database-s-login}

Pilih **Tampilkan login** di baris basis data. Kartu detail koneksi menampilkan nama, pengguna, kata sandi, dan host, masing-masing dengan tombol salin. Jika Adminer atau phpMyAdmin terinstal, kartu itu juga punya **Buka di Adminer** atau **Buka di phpMyAdmin**.

## Mengganti kata sandi basis data {#change-a-database-password}

1. Buka menu basis data dan pilih **Kata sandi baru**.
2. Biarkan **Buat kata sandi acak yang kuat**, atau pilih **Gunakan kata sandi saya sendiri** dan ketik kata sandinya. Kata sandi Anda sendiri mengikuti aturan yang sama seperti saat Anda [membuat basis data](#create-a-database).
3. Pilih **Kata sandi baru**.

Panel mengatur kata sandi baru dan menampilkannya di kartu detail koneksi. Jika basis data tertaut ke situs web WordPress yang dibuat panel, panel juga menulis kata sandi baru ke `wp-config.php` situs tersebut. Aplikasi lain tidak bisa terhubung sampai Anda memperbarui pengaturannya.

Jika basis data mengizinkan akses jarak jauh, pengguna jarak jauhnya juga mendapat kata sandi baru.

## Ekspor dan impor {#export-and-import}

### Ekspor {#export}

Buka menu basis data dan pilih **Ekspor (.sql)**. Browser Anda mengunduh file SQL biasa dengan nama seperti `shop_20261126-143000.sql`.

Dump dibuat dengan `mariadb-dump --single-transaction` (`mysqldump` di MySQL, juga dengan `--set-gtid-purged=OFF`), jadi situs web tetap berjalan selama ekspor. Dump menyertakan stored routine dan trigger.

### Impor {#import}

1. Buka menu basis data dan pilih **Impor**.
2. Pilih file di **File SQL (.sql atau .sql.gz)**. File `.sql.gz` didekompresi sambil diimpor.
3. Opsional: centang **Kosongkan basis data dulu (hapus semua tabelnya)**. Basis data lalu dihapus dan dibuat lagi sebelum impor, dan penggunanya tetap punya akses.
4. Pilih **Impor**.

Satu impor bisa berukuran hingga 4 GB.

Dump yang dibuat server lain, seperti cadangan dari sebelum [peralihan](#switch-between-mariadb-and-mysql), juga bisa diimpor: panel menulis ulang nama collation yang tidak dikenal server ini sambil file dialirkan masuk (`utf8mb4_0900_*` milik MySQL 8 di MariaDB, collation `uca1400` milik MariaDB 11 di MySQL). Di MySQL, panel juga membuang `NO_AUTO_CREATE_USER`, yang ditulis MariaDB ke SQL mode setiap trigger dan routine dan ditolak MySQL 8. Tidak ada hal lain di file yang berubah; dump yang memakai basis data lain tetap ditolak. Pemulihan cadangan basis data bekerja dengan cara yang sama.

Impor berjalan sebagai pengguna MariaDB sementara, `nefo-import-` diikuti nama basis data, dengan semua hak akses pada basis data ini dan tidak ada pada basis data lain. Pengguna ini juga boleh mengatur definer trigger, view, dan stored routine, jadi dump yang memuatnya, seperti ekspor dan cadangan milik panel sendiri, diimpor seperti sebelumnya. Panel menghapus pengguna ini saat impor selesai, juga jika impor gagal. Hanya jika file membuat trigger, view, routine, atau event tanpa klausa `DEFINER`, yang lalu berjalan sebagai pengguna ini, panel mempertahankannya: dalam keadaan terkunci, sehingga tidak ada yang bisa masuk dengannya, dan dengan hak akses hanya pada basis data ini. Menghapus basis data juga menghapus pengguna ini.

Di MySQL, nama pengguna dibatasi 32 karakter, jadi untuk nama basis data yang panjang, nama penggunanya dipendekkan dan ditambahi hash pendek. Definer diizinkan di sana dengan `SET_ANY_DEFINER` dan `ALLOW_NONEXISTENT_DEFINER` alih-alih `SET USER` milik MariaDB; tanpa `SYSTEM_USER`, pengguna tetap tidak bisa membuat objek berjalan sebagai root. Baris pertama dump dari versi MariaDB yang lebih baru, `/*M!999999\- enable the sandbox mode */`, yang ditolak klien MySQL, dilewati di MySQL.

::: warning
Jika file memakai basis data lain, misalnya dengan `USE other_db`, `CREATE DATABASE other_db`, atau `INSERT INTO other_db.posts`, MariaDB menolak pernyataan itu dan impor berhenti dengan pesan yang diawali "File SQL ini memakai basis data lain, other_db." Apa yang sudah dilakukan file sebelum baris itu tetap ada di basis data. Hapus baris `CREATE DATABASE` dan `USE` dari file, atau ekspor ulang basis data tanpa `--databases`, lalu impor lagi dengan **Kosongkan basis data dulu (hapus semua tabelnya)** tercentang. Baris `CREATE DATABASE` dan `USE` yang menyebut basis data yang sama ini tidak masalah.
:::

## Mengizinkan akses jarak jauh {#allow-remote-access}

Secara bawaan, MariaDB hanya menerima koneksi dari server itu sendiri. Anda bisa mengizinkan alamat tertentu terhubung ke satu basis data, misalnya alat pelaporan di komputer kantor Anda.

Sebelum mulai, instal dan aktifkan firewall ufw di halaman **Firewall & SSH**. Panel menolak akses jarak jauh selama ufw belum terinstal.

1. Buka menu basis data dan pilih **Akses jarak jauh**.
2. Di bawah **Alamat yang diizinkan**, masukkan sebuah alamat. Pilih **Tambah alamat** untuk menambah alamat lain.
3. Pilih **Simpan**.

Hubungkan dengan pengguna dan kata sandi basis data ke alamat IP publik server, port 3306.

### Alamat yang diizinkan {#allowed-addresses}

- Alamat IPv4 atau IPv6 tunggal, seperti `203.0.113.10` atau `2001:db8::1`.
- Rentang IPv4 dalam bentuk CIDR, seperti `198.51.100.0/24`, tidak lebih lebar dari `/16`.
- Rentang IPv6 tidak bisa, karena MariaDB tidak bisa mencocokkannya. Masukkan alamat IPv6 tunggal.
- Paling banyak 10 alamat per basis data.
- Alamat loopback, `0.0.0.0`, dan multicast ditolak.

### Apa yang diubah panel {#what-the-panel-changes}

| Di mana | Perubahan |
|---|---|
| Pengguna MariaDB | Untuk setiap alamat, pengguna `name@address` dengan kata sandi dan hak akses yang sama seperti pengguna lokal |
| Firewall (ufw) | Untuk setiap alamat, aturan yang mengizinkan port 3306/tcp hanya dari alamat itu, dengan komentar `MariaDB name` |
| Pengaturan MariaDB | `/etc/mysql/mariadb.conf.d/99-panel-remote.cnf` membuat MariaDB mendengarkan di semua antarmuka (`0.0.0.0`, atau `::` jika ada alamat IPv6 yang diizinkan). File ini hanya ada selama setidaknya satu basis data mengizinkan akses jarak jauh |

MySQL bekerja dengan cara yang sama: file-nya adalah `/etc/mysql/mysql.conf.d/99-panel-remote.cnf`, komentar aturannya `MySQL name`, dan pengguna jarak jauh dihapus lalu dibuat lagi saat berubah, karena MySQL tidak punya `CREATE OR REPLACE USER`. Di MySQL, pesan-pesan di bawah menyebut MySQL.

MariaDB dimulai ulang saat akses jarak jauh pertama kali diaktifkan, dan lagi saat akses itu dinonaktifkan untuk basis data terakhir. Situs web kehilangan basis datanya selama beberapa detik. Jika ada tugas lain yang sedang berjalan saat itu, dialog menampilkan "MariaDB harus dimulai ulang sebelum menerima koneksi jarak jauh." Pilih **Mulai ulang MariaDB** setelah tugas itu selesai.

Jika MariaDB tidak mau berjalan dengan file akses jarak jauh, panel menghapus file itu dan menjalankan MariaDB tanpanya, jadi situs web Anda tetap berfungsi.

::: warning
- Jika firewall nonaktif, port 3306 terbuka untuk semua orang. MariaDB tetap hanya menerima alamat yang tercantum.
- Koneksi tidak dienkripsi kecuali klien meminta TLS. Untuk data sensitif, gunakan tunnel SSH atau VPN.
- Cantumkan hanya alamat yang Anda kendalikan dan tidak berubah.
:::

Untuk menonaktifkan akses jarak jauh, buka lagi **Akses jarak jauh** dan pilih **Nonaktifkan**. Menghapus basis data juga menghapus pengguna jarak jauh dan aturan firewall-nya.

## Menemukan kueri lambat {#find-slow-queries}

MariaDB bisa mencatat setiap kueri yang berjalan lebih lama dari ambang batas. Ini membantu Anda menemukan apa yang memperlambat situs web.

1. Buka **Basis data** → **Kueri lambat**. Tab ini muncul setelah MariaDB terinstal.
2. Atur **Lebih lambat dari** ke jumlah detik, dari 0,1 sampai 60.
3. Aktifkan sakelar **Catat kueri lambat** dan pilih **Terapkan**.

MariaDB harus berjalan untuk mengubah pengaturan ini. Perubahan langsung berlaku, untuk koneksi yang dibuka setelahnya, dan tetap ada setelah dimulai ulang: panel menyimpannya di `/etc/mysql/mariadb.conf.d/99-panel-slowlog.cnf`, dan log ditulis ke `/var/log/mysql/mariadb-slow.log`. Di MySQL, pengaturannya ada di `/etc/mysql/mysql.conf.d/99-panel-slowlog.cnf` dan log-nya `/var/log/mysql/mysql-slow.log`.

Panel merotasi log agar tidak memenuhi disk, di MariaDB maupun di MySQL. Sekali satu jam panel memeriksanya, dan jika log lebih besar dari 50 MB, atau entri tertuanya sudah berumur 7 hari, panel mengganti nama log itu, meminta server membuka log baru (`FLUSH SLOW LOGS`, tanpa memulai ulang), dan memampatkan log lama di sebelahnya, sebagai `mariadb-slow.log-YYYYMMDD-HHMMSS.gz` (`mysql-slow.log-…` di MySQL). Panel menyimpan 4 yang terbaru dan menghapus yang lebih lama. Tab ini menampilkan berapa yang sedang disimpan. Tabel hanya membaca log saat ini.

Tabel menampilkan 200 entri terbaru, yang terbaru lebih dulu, dibaca dari 4 MB terakhir log:

| Kolom | Artinya |
|---|---|
| **Waktu** | Kapan kueri berjalan |
| **Pengguna dan host** | Siapa yang menjalankannya, seperti `shop@localhost` |
| **Basis data** | Basis data yang dipakainya |
| **Durasi** | Berapa lama prosesnya, dalam detik |
| **Baris diperiksa** | Berapa baris yang dibaca MariaDB |
| **Kueri** | Kuerinya. Pilih untuk melihat seluruhnya, hingga 2.000 karakter. Kueri yang lebih panjang ditandai **Dipendekkan** |

Pilih **Muat ulang** untuk membaca log lagi, dan **Kosongkan log** untuk mengosongkannya. **Kosongkan log** hanya mengosongkan log saat ini; log lama yang dimampatkan tetap ada sampai rotasi menghapusnya.

::: tip
Pencatatan menambah sedikit beban kerja server. Matikan sakelarnya dan pilih **Terapkan** setelah selesai.
:::

## Adminer {#adminer}

Adminer memungkinkan Anda menjelajah dan mengedit basis data di browser, seperti phpMyAdmin.

### Menginstal Adminer {#install-adminer}

1. Instal server web terlebih dahulu, di halaman **Perangkat lunak**. Adminer berjalan di server web itu dengan PHP.
2. Buka **Perangkat lunak** → **Adminer (antarmuka basis data)** dan pilih **Instal**.

Panel mengunduh Adminer 6.3.1 (build dengan semua driver Adminer) dan memeriksanya terhadap checksum SHA-256 yang dipatok sebelum menginstalnya. Adminer membuka basis data MariaDB atau MySQL, dan basis data PostgreSQL setelah PostgreSQL terinstal (lihat [PostgreSQL di Adminer](./postgresql#postgresql-in-adminer)). Driver SQLite-nya menolak setiap upaya masuk, karena SQLite tidak punya kata sandi.

### Membuka Adminer {#open-adminer}

1. Pilih **Buka di Adminer** di bagian atas halaman **Basis data**, atau di kartu detail koneksi sebuah basis data. **Perangkat lunak** → **Buka Adminer** melakukan hal yang sama.
2. Adminer terbuka di tab baru, di alamat panel Anda diikuti `adminer/`, misalnya `https://203.0.113.10:28443/k3x9p2qa7m1z/adminer/`.
3. Masuk ke Adminer dengan pengguna dan kata sandi basis data dari **Tampilkan login**.

### Cara Adminer dilindungi {#how-adminer-is-protected}

Adminer hanya bisa dijangkau melalui proses masuk panel:

- Adminer berjalan di situs internal server web Anda yang hanya mendengarkan di 127.0.0.1, di port acak antara 40000 dan 59999.
- Situs internal itu hanya menjawab permintaan yang membawa token rahasia. Permintaan lain mendapat `404`.
- Panel hanya menambahkan token ke permintaan dari sesi panel yang sudah masuk. Tanpa sesi panel, `adminer/` menjawab `401` dan permintaan tidak pernah sampai ke Adminer.
- Cookie sesi milik panel tidak pernah diteruskan ke Adminer.

Jadi Adminer tidak bisa dijangkau dari internet, dan program lain di server tidak bisa memakainya tanpa token.

Adminer bisa mengimpor file hingga 256 MB, dan satu permintaan bisa berjalan hingga 600 detik. Untuk dump yang lebih besar, gunakan **Impor** di halaman **Basis data**.

Mencopot Adminer menghapus file-filenya dan situs internalnya. Basis data Anda tidak disentuh. Mencopot server web juga menghapus Adminer.

## phpMyAdmin {#phpmyadmin}

phpMyAdmin adalah antarmuka basis data lainnya, untuk MariaDB dan MySQL. Instal yang Anda sukai, atau keduanya: keduanya tidak berbagi file.

### Menginstal phpMyAdmin {#install-phpmyadmin}

1. Instal server web terlebih dahulu, di halaman **Perangkat lunak**. phpMyAdmin berjalan di server web itu dengan PHP.
2. Buka **Perangkat lunak** → **phpMyAdmin (antarmuka basis data)** dan pilih **Instal**.

Panel mengunduh phpMyAdmin 5.2.3 (semua bahasa) dari `files.phpmyadmin.net` dan memeriksanya terhadap checksum SHA-256 yang dipatok sebelum mengekstraknya ke `/usr/share/nefo-phpmyadmin`. Panel tidak menyertakan skrip penyiapan, contoh, dan dokumentasi lokal (tautan bantuan membuka docs.phpmyadmin.net sebagai gantinya). phpMyAdmin menyimpan file sementaranya di `/var/lib/nefo-phpmyadmin/tmp`.

Panel menulis `config.inc.php` milik phpMyAdmin, dengan kunci enkripsi cookie acak:

- Anda masuk dengan pengguna dan kata sandi basis data, hanya ke basis data server ini. Anda tidak bisa memilih server lain, masuk tanpa kata sandi, atau masuk sebagai `root`.
- phpMyAdmin tidak memeriksa versi baru atau mengirim laporan kesalahan.
- Formulir penggantian kata sandinya disembunyikan. Ganti kata sandi basis data di halaman **Basis data**, yang juga memperbarui `wp-config.php` situs web WordPress.

### Membuka phpMyAdmin {#open-phpmyadmin}

1. Pilih **Buka di phpMyAdmin** di bagian atas halaman **Basis data**, di menu basis data, atau di kartu detail koneksi. **Perangkat lunak** → **Buka phpMyAdmin** melakukan hal yang sama.
2. phpMyAdmin terbuka di tab baru, di alamat panel Anda diikuti `phpmyadmin/`, misalnya `https://203.0.113.10:28443/k3x9p2qa7m1z/phpmyadmin/`.
3. Masuk dengan pengguna dan kata sandi basis data dari **Tampilkan login**.

phpMyAdmin dilindungi seperti Adminer (lihat [Cara Adminer dilindungi](#how-adminer-is-protected)), di situs internalnya sendiri dengan token rahasianya sendiri. Tanpa sesi panel, `phpmyadmin/` menjawab `401`, dan kunci API juga tidak bisa menjangkaunya. Hanya tiga titik masuk phpMyAdmin yang berjalan sebagai PHP dan hanya skrip, style, dan gambarnya yang disajikan sebagai file; hal lain menjawab `404`, termasuk skrip penyiapan. phpMyAdmin mengirim Content-Security-Policy-nya sendiri dan memeriksa token formulirnya sendiri.

Seperti Adminer, phpMyAdmin bisa mengimpor file hingga 256 MB, dan satu permintaan bisa berjalan hingga 600 detik. Mencopot phpMyAdmin menghapus file-filenya, situs internalnya, dan pengaturannya. Basis data Anda tidak disentuh. Mencopot server web juga menghapus phpMyAdmin.

## Penjelajah kunci Redis {#redis-key-browser}

Jika Redis terinstal, halaman **Basis data** punya tab **Redis**. Untuk menginstal Redis, buka **Perangkat lunak** → **Redis** → **Instal**. Lihat [Perangkat lunak](./software#redis) untuk cara Redis disiapkan.

Tab ini menampilkan versi Redis, berapa lama Redis sudah berjalan, **Memori terpakai** (beserta batas memorinya), dan jumlah **Kunci**. Jika tab ini menampilkan **Redis terinstal tetapi tidak berjalan.**, jalankan Redis di halaman **Layanan**.

### Mencari kunci {#find-keys}

1. Pilih basis data dari 0 sampai 15. Daftar ini menampilkan berapa kunci yang dimiliki masing-masing.
2. Masukkan pola. `*` cocok dengan apa saja dan `?` cocok dengan satu karakter, misalnya `user:*`.
3. Pilih **Cari**.

Pencarian menelusuri kunci dalam langkah sekitar 100. Jika masih ada lagi, pilih **Lanjutkan pencarian**.

### Melihat kunci {#view-a-key}

Pilih sebuah kunci untuk melihat **Jenis**, **Kedaluwarsa dalam**, dan **Ukuran**-nya, serta nilainya:

| Jenis | Yang ditampilkan panel |
|---|---|
| string | 64 KB pertama dari nilai |
| list, hash, set, sorted set | 200 item pertama. Setiap item dipotong pada 4 KB |
| stream dan jenis lain | Hanya jenis dan masa kedaluwarsa |

Karakter yang tidak bisa ditampilkan dalam nilai biner diganti.

### Menghapus kunci {#delete-keys}

- Untuk menghapus satu kunci, pilih kunci itu lalu **Hapus kunci**.
- Untuk menghapus setiap kunci di basis data yang dipilih, pilih **Kosongkan basis data 0** (tombol menampilkan nomor basis data), ketik nomornya untuk konfirmasi, lalu pilih **Hapus semua kunci**.

Situs web yang memakai Redis sebagai cache akan membangunnya kembali. Data yang hanya disimpan di Redis akan hilang.

Penjelajah ini tidak mengedit nilai atau waktu kedaluwarsa. Gunakan `redis-cli` di server untuk itu.

Jika Redis punya kata sandi (`requirepass` di `/etc/redis/redis.conf`), panel membacanya dari file itu dan masuk dengannya.

## Menghapus basis data {#delete-a-database}

1. Buka menu basis data dan pilih **Hapus**.
2. Ketik nama basis data untuk konfirmasi dan pilih **Hapus**.

::: danger
Ini menghapus basis data, semua tabelnya, dan penggunanya. Tindakan ini tidak bisa dibatalkan. Pengguna jarak jauh, aturan firewall, dan jadwal pencadangan otomatisnya ikut terhapus. Cadangan yang sudah dibuat tetap disimpan di halaman **Pencadangan**.
:::

Saat Anda menghapus situs web, basis datanya tetap disimpan kecuali Anda mencentang **Hapus juga basis datanya**: basis data itu tetap ada di sini, tidak tertaut ke situs web mana pun. Memulihkan cadangan situs web menautkannya lagi.

## Mencopot MariaDB {#uninstall-mariadb}

Copot MariaDB di halaman **Perangkat lunak**. Panel menolak selama MariaDB masih menyimpan basis data apa pun selain basis data sistemnya sendiri, termasuk basis data yang dibuat di luar panel. Lihat [Perangkat lunak](./software) untuk apa yang dipertahankan saat pencopotan.

MariaDB dari repositori MariaDB ikut dicopot beserta file repositori dan kuncinya.

MySQL dicopot dengan cara yang sama, di kartunya, dengan pemeriksaan yang sama. Panel juga menghapus repositori Oracle dan kuncinya. Direktori data `/var/lib/mysql` tetap ada kecuali Anda mencentang kotaknya dan mengetik `mysql`.

Untuk mengganti yang satu dengan yang lain sambil mempertahankan basis data Anda, lakukan [peralihan](#switch-between-mariadb-and-mysql).

## PostgreSQL {#postgresql}

PostgreSQL berjalan di samping MariaDB atau MySQL, di tab **PostgreSQL** sendiri pada halaman ini: versi milik distribusi Anda atau 15 sampai 19 dari proyek PostgreSQL, dengan basis data dan pemiliknya, impor dan ekspor, akses jarak jauh lewat TLS, pencadangan, dan peningkatan versi mayor. Lihat [PostgreSQL](./postgresql).

## Memcached {#memcached}

Jika Memcached terinstal (**Perangkat lunak** → **Memcached**), halaman **Basis data** punya tab **Memcached**. Tab ini menampilkan versi, berapa lama Memcached sudah berjalan, **Memori terpakai** beserta batasnya, jumlah **Item**, rasio hit, dan koneksi yang terbuka. Tab ini juga menampilkan apa yang dilaporkan Memcached sendiri: bahwa Memcached hanya mendengarkan di 127.0.0.1 dan UDP nonaktif.

**Mulai ulang** mengosongkan cache. Lihat [Perangkat lunak](./software#memcached) untuk cara Memcached disiapkan.
