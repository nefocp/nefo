---
title: PostgreSQL
description: Instal PostgreSQL dari distribusi Anda atau versi 15 sampai 19 dari proyek PostgreSQL, buat basis data dan pemiliknya, izinkan akses jarak jauh lewat TLS, impor dan ekspor, cadangkan, dan tingkatkan ke versi mayor yang lebih baru.
---

# PostgreSQL

Nefo menjalankan PostgreSQL di samping MariaDB atau MySQL: versi milik distribusi Anda, atau versi mayor 15 sampai 19 dari repositori milik proyek PostgreSQL sendiri (apt.postgresql.org). Tab **Basis data** → **PostgreSQL** membuat basis data beserta pemiliknya, mengimpor dan mengekspornya, membuka satu basis data untuk alamat yang Anda pilih lewat TLS, dan meningkatkan server ke versi mayor yang lebih baru dengan membuat dump terlebih dahulu.

PostgreSQL adalah server basis data yang disukai banyak aplikasi Laravel, Django, Node.js, dan Odoo. MariaDB (atau MySQL) dan PostgreSQL bisa berjalan berdampingan. Untuk MariaDB dan MySQL, lihat [Basis data](./databases).

## Menginstal PostgreSQL {#install-postgresql}

1. Buka **Perangkat lunak** → **PostgreSQL** dan pilih **Instal**.
2. Di bawah **Versi**, pilih PostgreSQL yang akan diinstal:
   - versi milik distribusi Anda, seperti "PostgreSQL 16, versi distribusi", yang ditandai **Disarankan**. Versi ini diperbarui bersama sistem;
   - versi dari apt.postgresql.org, seperti "PostgreSQL 18 dari apt.postgresql.org", dan versi mayor lain yang ditawarkan repositori proyek PostgreSQL. Pilih salah satunya jika aplikasi membutuhkan versi yang lebih baru (atau lebih lama) daripada yang disertakan sistem Anda.
3. Pilih **Instal** dan tunggu satu atau dua menit. Log progres menampilkan setiap langkah.

| Sistem | Versi milik distribusi | Dari apt.postgresql.org |
|---|---|---|
| Ubuntu 22.04 | 14 | 15, 16, 17, 18, 19 |
| Ubuntu 24.04 | 16 | 15, 17, 18, 19 |
| Ubuntu 26.04 | 18 | 15, 16, 17, 19 |
| Debian 12 | 15 | 16, 17, 18, 19 |
| Debian 13 | 17 | 15, 16, 18, 19 |

Panel menawarkan versi mayor yang masih didukung proyek PostgreSQL. Di sistem lain, atau di prosesor yang tidak punya paket di repositori itu, hanya versi milik distribusi yang ditawarkan dan dialog menyebutkan alasannya.

Jika Anda memilih versi dari apt.postgresql.org, panel:

- mengunduh kunci penandatanganan repositori dari `https://www.postgresql.org/media/keys/ACCC4CF8.asc` dan menolaknya kecuali fingerprint-nya `B97B 0AFC AA1A 47F0 44F2 44A0 7FCC 7D46 ACCC 4CF8` (kunci "PostgreSQL Debian Repository");
- menulisnya ke `/etc/apt/keyrings/panel-pgdg.gpg` dan menambahkan `/etc/apt/sources.list.d/panel-pgdg.list`, yang memercayai kunci itu hanya untuk repositori ini;
- menginstal paket server berdasarkan namanya, seperti `postgresql-18`. Panel tidak pernah menginstal paket `postgresql` dari repositori itu, karena paket itu selalu menarik versi mayor terbaru, dan pembaruan sistem lalu akan menambahkan server kedua.

Pembaruan keamanan untuk kedua jenis versi datang bersama pembaruan sistem (halaman **Sistem**, atau `apt upgrade`).

Kartu di **Perangkat lunak** lalu menampilkan **Dari distribusi** atau **Dari apt.postgresql.org**, begitu pula bagian atas **Basis data** → **PostgreSQL**.

Hanya satu versi PostgreSQL yang berjalan dalam satu waktu. Panel menolak:

- versi lain selagi satu versi terinstal. [Tingkatkan versinya](#upgrade-postgresql) saja, atau copot terlebih dahulu;
- versi selain versi yang datanya masih ada di server dari instalasi sebelumnya (di `/var/lib/postgresql/<version>`). Instal lagi versi itu untuk memakai datanya, lalu tingkatkan versinya jika Anda mau, atau hapus data itu terlebih dahulu.

apt berjalan dengan locale C, jadi cluster yang dibuat paket menyimpan teks sebagai `SQL_ASCII`. Nefo lalu membuat ulang cluster baru yang masih kosong itu dengan UTF-8 (`C.UTF-8`). Cluster yang sudah ada sebelum instalasi tidak pernah dibuat ulang.

## Cara PostgreSQL disetel {#how-postgresql-is-tuned}

Nefo menulis pengaturannya ke `/etc/postgresql/<version>/main/conf.d/90-nefo.conf`. Pengaturan memori bergantung pada RAM server:

| RAM server | `shared_buffers` | `effective_cache_size` | `work_mem` | `maintenance_work_mem` | `max_connections` |
|---|---|---|---|---|---|
| Kurang dari 768 MB | 64MB | 256MB | 2MB | 16MB | 30 |
| 768 MB atau lebih | 128MB | 512MB | 4MB | 32MB | 40 |
| 1,5 GB atau lebih | 256MB | 1024MB | 4MB | 64MB | 50 |
| 3 GB atau lebih | 512MB | 2048MB | 8MB | 128MB | 75 |
| 7 GB atau lebih | 1024MB | 4096MB | 16MB | 256MB | 100 |

`effective_cache_size` sekitar setengah RAM, karena server yang sama juga menjalankan server web, PHP, dan mungkin MariaDB. Di setiap server, Nefo juga mengatur `listen_addresses = 'localhost'`, `password_encryption = 'scram-sha-256'`, `random_page_cost = 1.1` (untuk penyimpanan SSD), dan `jit = off`. Di VPS berupa kontainer (LXC), RAM yang dihitung adalah batas memori kontainer (lihat [VPS yang berupa kontainer](./first-steps#a-vps-that-is-a-container)).

Dalam keadaan menganggur dan kosong, PostgreSQL membutuhkan sekitar 30 sampai 40 MB untuk segelintir proses latar belakangnya. Saat cache-nya terisi, pemakaiannya tumbuh mendekati `shared_buffers` ditambah sekitar 40 MB, dan setiap koneksi terbuka menambah beberapa MB. Halaman **Perangkat lunak** menampilkan `shared_buffers` ditambah 40 MB.

::: warning
Jangan edit `90-nefo.conf` atau `pg_hba.conf`. Menginstal ulang PostgreSQL akan menulis ulang keduanya. Baris Anda sendiri di `pg_hba.conf` tetap dipertahankan saat Anda mengubah akses jarak jauh: panel hanya menyentuh barisnya sendiri (lihat [Akses jarak jauh](#postgresql-remote-access)).
:::

## Siapa yang bisa terhubung {#who-can-connect}

Nefo menulis `/etc/postgresql/<version>/main/pg_hba.conf`:

```
local   all  postgres                peer
local   all  all                     scram-sha-256
host    all  all       127.0.0.1/32  scram-sha-256
host    all  all       ::1/128       scram-sha-256
```

- Hanya pengguna sistem `postgres` yang masuk tanpa kata sandi, lewat socket lokal. Panel mengelola server dengan cara itu.
- Semua pihak lain membutuhkan kata sandi, lewat socket atau lewat TCP dari server ini. Tidak seperti bawaan distribusi, pengguna Linux lain tidak mendapat login tanpa kata sandi ("peer"), jadi pengguna Linux sebuah situs web tidak bisa membuka role basis data bernama sama tanpa kata sandinya.
- Mesin lain tidak bisa terhubung, kecuali Anda mengizinkan alamatnya untuk satu basis data (lihat [Akses jarak jauh](#postgresql-remote-access)). PostgreSQL hanya mendengarkan di localhost selama tidak ada basis data yang mengizinkan akses jarak jauh.

## Membuat basis data PostgreSQL {#create-a-postgresql-database}

1. Buka **Basis data** → **PostgreSQL** dan pilih **Tambah basis data**.
2. Masukkan nama di **Basis data**. Role dengan nama yang sama menjadi pemilik basis data.
3. Kosongkan **Kata sandi** untuk mendapatkan kata sandi acak yang kuat, atau ketik kata sandi Anda sendiri.
4. Opsional: pilih situs di **Situs web**. Cadangan situs web itu lalu menyertakan basis data ini, dan menghapus situs web beserta basis datanya juga menghapus basis data ini.
5. Pilih **Buat basis data**.

Kartu detail koneksi menampilkan **Basis data**, **Pengguna**, **Kata sandi**, **Host** (`localhost`), dan **Port** (`5432`). Untuk aplikasi Laravel, misalnya, atur `DB_CONNECTION=pgsql`, `DB_HOST=127.0.0.1`, `DB_PORT=5432`, serta basis data, pengguna, dan kata sandi dari kartu tersebut.

Aturannya:

- Nama dan kata sandi mengikuti [aturan MariaDB](./databases#create-a-database). Nama milik PostgreSQL sendiri juga ditolak: `postgres`, `template0`, `template1`, `public`, `all`, dan apa pun yang diawali `pg_`.
- Satu nama tidak bisa dipakai oleh kedua server: jika basis data atau pengguna MariaDB bernama `shop`, basis data PostgreSQL tidak bisa memakai nama itu, dan sebaliknya. Panel juga menolak nama yang sudah dipakai PostgreSQL untuk basis data atau role.
- Role bisa masuk, bukan superuser, dan tidak bisa membuat basis data atau role. Tidak ada role lain yang boleh terhubung ke basis data, karena panel mencabut `CONNECT` dari semua pihak lain.
- Basis data baru memakai UTF-8 dengan locale `C.UTF-8`.

Kata sandi tidak pernah sampai ke PostgreSQL dalam bentuk teks biasa: panel hanya mengirim verifier SCRAM-SHA-256-nya, jadi kata sandi juga tidak bisa muncul di log PostgreSQL.

**Tampilkan login**, **Kata sandi baru**, dan **Hapus** bekerja seperti untuk MariaDB. Penghapusan juga menutup koneksi basis data yang terbuka, dan menghapus baris akses jarak jauh serta aturan firewall-nya. Basis data yang dihapus di luar panel ditandai **Tidak ditemukan di PostgreSQL**.

## Bisakah server lain terhubung ke basis data PostgreSQL? {#postgresql-remote-access}

Bisa, mulai v0.24.0, satu basis data dalam satu waktu. Secara bawaan, PostgreSQL hanya menerima koneksi dari server itu sendiri. Anda bisa mengizinkan alamat tertentu terhubung ke satu basis data, dengan pengguna (role) dan kata sandi milik basis data itu sendiri, lewat TLS.

Sebelum mulai, instal firewall ufw di halaman **Firewall & SSH**, lalu aktifkan. Panel menolak akses jarak jauh selama ufw belum terinstal.

1. Buka **Basis data** → **PostgreSQL**, buka menu basis data, dan pilih **Akses jarak jauh**.
2. Di bawah **Alamat yang diizinkan**, masukkan sebuah alamat. Pilih **Tambah alamat** untuk menambah alamat lain.
3. Pilih **Simpan**.

Baris basis data lalu menampilkan "Jarak jauh: 1" (jumlah alamat), dan bagian atas tab menampilkan "1 terbuka untuk server lain".

Hubungkan ke alamat IP publik server dengan pengguna dan kata sandi dari **Tampilkan login**, dan dengan TLS, misalnya:

```sh
psql "host=203.0.113.10 port=5432 dbname=shop user=shop sslmode=require"
```

### Alamat yang diizinkan {#allowed-addresses}

- Alamat IPv4 atau IPv6 tunggal, seperti `198.51.100.7` atau `2001:db8::1`.
- Rentang dalam bentuk CIDR: IPv4 tidak lebih lebar dari `/16` (seperti `198.51.100.0/24`), IPv6 tidak lebih lebar dari `/48` (seperti `2001:db8:5::/48`).
- Paling banyak 10 alamat per basis data.
- Alamat loopback, link-local (`fe80::`), `0.0.0.0`, `::`, dan multicast ditolak.

### Apa yang diubah panel {#what-the-panel-changes}

| Di mana | Perubahan |
|---|---|
| `pg_hba.conf` (PostgreSQL 16 dan yang lebih baru) | Satu baris di akhir, `include_if_exists nefo-remote.conf`, dengan komentar. Alamat yang diizinkan ada di `/etc/postgresql/<version>/main/nefo-remote.conf` |
| `pg_hba.conf` (PostgreSQL 14 dan 15) | Sebuah blok di akhir antara `# BEGIN Nefo remote access` dan `# END Nefo remote access` |
| Baris-baris itu | Satu per alamat, hanya untuk basis data ini dan role-nya sendiri, dengan masuk memakai kata sandi: `hostssl "shop" "shop" 198.51.100.7/32 scram-sha-256` |
| Pengaturan PostgreSQL | `/etc/postgresql/<version>/main/conf.d/95-nefo-remote.conf` mengatur `listen_addresses = '*'`. File ini hanya ada selama setidaknya satu basis data mengizinkan akses jarak jauh |
| Firewall (ufw) | Untuk setiap alamat, aturan yang mengizinkan port cluster (bawaannya 5432) lewat TCP hanya dari alamat itu, dengan komentar `PostgreSQL name` |

Panel tidak pernah mengubah baris lain di `pg_hba.conf`. Jika baris Anda sendiri ada sebelum baris panel, baris Anda dibaca lebih dulu: baris `reject` milik Anda untuk alamat yang sama yang berlaku. Sebelum PostgreSQL memuat ulang `pg_hba.conf` yang berubah, panel meminta PostgreSQL memeriksanya (`pg_hba_file_rules`). Jika PostgreSQL menemukan kesalahan, file lama dikembalikan dan tidak ada yang berubah.

Jika cluster memakai port selain 5432, aturan firewall memakai port itu, dan detail koneksi menampilkannya.

### Enkripsi {#encryption}

Dialog menyebutkan apakah koneksi dari mesin lain dienkripsi:

- **Koneksi dari mesin lain terenkripsi**: barisnya `hostssl`, jadi PostgreSQL menolak koneksi dari alamat-alamat itu yang tidak memakai TLS. Klien membutuhkan `sslmode=require` (atau yang lebih ketat). Debian dan Ubuntu membuat cluster dengan sertifikat sistem yang ditandatangani sendiri (`/etc/ssl/certs/ssl-cert-snakeoil.pem`). Jika TLS cluster dinonaktifkan, panel membuat sertifikatnya sendiri yang ditandatangani sendiri (`nefo-ssl.crt` dan `nefo-ssl.key` di sebelah `pg_hba.conf`) dan mengaktifkan TLS di `95-nefo-remote.conf`; TLS nonaktif lagi bersama akses jarak jauh yang terakhir. Kedua sertifikat ditandatangani sendiri, jadi klien tidak bisa memverifikasi server dengannya: `sslmode=require` mengenkripsi, tetapi `verify-ca` dan `verify-full` gagal kecuali Anda memasang sertifikat Anda sendiri.
- **Koneksi dari mesin lain tidak terenkripsi**: PostgreSQL tidak punya TLS, jadi barisnya `host`. Gunakan tunnel SSH atau VPN untuk apa pun yang sensitif.

### Memulai ulang {#restarts}

`listen_addresses` yang baru butuh memulai ulang; alamat yang diizinkan dan TLS cukup dimuat ulang. Jadi PostgreSQL dimulai ulang saat akses jarak jauh pertama kali diaktifkan, dan lagi saat akses itu dinonaktifkan untuk basis data terakhir. Situs web kehilangan basis datanya selama beberapa detik, dan transaksi yang terbuka dibatalkan. Menambah atau menghapus alamat sesudahnya tidak memulai ulang PostgreSQL.

Jika ada tugas lain yang sedang berjalan saat itu, dialog menampilkan **PostgreSQL harus dimulai ulang sebelum menerima koneksi dari mesin lain.** Pilih **Mulai ulang** di dialog setelah tugas itu selesai.

Jika PostgreSQL tidak mau berjalan dengan `95-nefo-remote.conf`, panel menghapus file itu dan menjalankan PostgreSQL tanpanya, jadi situs web Anda tetap berfungsi.

::: warning
- Jika firewall nonaktif, port terbuka untuk semua orang. PostgreSQL tetap hanya mengizinkan alamat yang tercantum masuk, dan hanya dengan kata sandi.
- Cantumkan hanya alamat yang Anda kendalikan dan tidak berubah.
:::

Untuk menonaktifkan akses jarak jauh, buka lagi **Akses jarak jauh** dan pilih **Nonaktifkan**. Menghapus basis data juga menghapus baris dan aturan firewall-nya; jika itu basis data terakhir dengan akses jarak jauh, PostgreSQL dimulai ulang agar kembali hanya mendengarkan di localhost.

## Mengekspor dan mengimpor basis data PostgreSQL {#export-and-import-a-postgresql-database}

**Ekspor (.sql.gz)** mengunduh dump SQL biasa yang di-gzip, dibuat dengan `pg_dump` sebagai role milik basis data itu sendiri, dengan `--no-owner --no-privileges`, sehingga bisa diimpor ke basis data mana pun.

**Impor** menerima file `.sql` atau `.sql.gz` hingga 4 GB:

- `psql` berjalan sebagai role milik basis data itu sendiri, tidak pernah sebagai `postgres`, dengan kata sandinya di file sementara yang hanya bisa dibaca root.
- File dijalankan dalam satu transaksi dengan `ON_ERROR_STOP`: jika satu pernyataan gagal, tidak ada yang diimpor dan pesan kesalahan menyebut barisnya.
- Perintah milik psql sendiri, yaitu baris yang diawali garis miring terbalik (seperti `\connect`, `\!`, atau `\copy`), ditolak: psql berjalan sebagai root, dan perintah-perintah itu bisa menjalankan program atau menulis file di server. Panel menjalankan psql dalam mode terbatas (`\restrict` dengan kunci acak) dan membuang baris `\restrict` dan `\unrestrict` yang ditulis versi `pg_dump` yang lebih baru. psql tanpa mode terbatas (lebih lama dari 17.6, 16.10, 15.14, atau 14.19) menolak setiap impor: instal pembaruan sistem terlebih dahulu.
- Pernyataan yang membuat atau mengubah role, memberikan objek ke role lain, atau membutuhkan superuser akan gagal. Ekspor ulang dump seperti itu dengan `pg_dump --no-owner --no-privileges`.
- `SET transaction_timeout`, yang ditulis `pg_dump` 17 ke atas, dilewati di server yang lebih lama.
- Sesudahnya, panel mengatur ulang kata sandi role, untuk berjaga-jaga jika file mengubahnya.

Dengan **Kosongkan basis data dulu (hapus semua isinya)** tercentang, basis data dihapus dan dibuat lagi sebelum impor, dengan pemilik role yang sama.

## Meningkatkan PostgreSQL ke versi yang lebih baru {#upgrade-postgresql}

Anda bisa memindahkan server ke versi mayor yang lebih baru dari apt.postgresql.org, misalnya dari 15 milik distribusi ke 17, dengan mempertahankan setiap basis data, pengguna, kata sandi, dan pengaturan akses jarak jauh.

1. Buka **Basis data** → **PostgreSQL** dan pilih **Tingkatkan versi**. Tombol ini muncul jika ada versi yang lebih baru yang ditawarkan dan PostgreSQL sedang berjalan.
2. Pilih **Versi baru**.
3. Baca **Apa yang terjadi**, lalu centang **Saya mengerti bahwa situs web dan aplikasi kehilangan basis datanya selama peningkatan versi.**
4. Pilih **Tingkatkan ke 17** (versi yang Anda pilih) dan ikuti log-nya.

Yang dilakukan panel, berurutan:

1. Memeriksa bahwa versi baru memang lebih baru, bahwa hanya ada satu cluster (`main`), bahwa tidak ada paket ekstensi yang terinstal untuk versi lama (seperti `postgresql-15-postgis-3`), dan bahwa disk punya ruang untuk dua kali data ditambah 256 MB.
2. Menyimpan setiap basis data dengan `pg_dumpall` ke satu file terkompresi, `/var/backups/nefo-postgresql/postgresql-<old>-before-<new>-<date>.sql.gz`, yang hanya bisa dibaca root. Belum ada yang berubah: jika dump gagal, peningkatan versi berhenti di situ.
3. Menghapus paket `postgresql` milik distribusi, yang hanya menunjuk ke sebuah versi (dari apt.postgresql.org paket itu akan menunjuk ke versi terbaru dan menginstal server kedua), setelah menandai `postgresql-<old>` sebagai diinstal secara manual. Lalu panel menambahkan repositori apt.postgresql.org jika belum ada dan menginstal `postgresql-<new>`. Cluster kosong yang dibuat paket ini dihapus. Jika apt ikut membawa server PostgreSQL lain, peningkatan versi berhenti dan dikembalikan.
4. Menjalankan `pg_upgradecluster -v <new> -m upgrade <old> main`. Selama itu PostgreSQL dihentikan: sekitar satu menit untuk server kecil, lebih lama untuk basis data besar. `pg_upgrade` menyalin file data, jadi cluster lama tetap seperti semula.
5. Memberi cluster baru `pg_hba.conf` yang lama, penyetelan panel (`conf.d/90-nefo.conf`), dan pengaturan akses jarak jauh (mulai versi 16 alamat yang diizinkan pindah ke `nefo-remote.conf`), memulai ulang cluster itu, dan memeriksa bahwa cluster berjalan di port yang sama dengan basis data yang sama, yang masing-masing merespons.
6. Baru setelah itu menghapus cluster lama dan mencopot paketnya (`postgresql-<old>` dan `postgresql-client-<old>`).

Jika ada langkah dari 3 sampai 5 yang gagal, panel mengembalikan keadaan: panel menghapus cluster baru, mencopot paket versi baru, mengembalikan pengaturan cluster lama, dan menjalankannya lagi. Log diakhiri dengan "PostgreSQL 15 runs again with its data as before". Basis data Anda tetap seperti semula. Repositori apt.postgresql.org tetap ada setelah paket diinstal darinya, karena `postgresql-common` dan `libpq5` diperbarui dari repositori itu dan kalau tidak, keduanya tidak akan mendapat pembaruan lagi; paket `postgresql` milik distribusi tidak dikembalikan.

Peningkatan versi ditolak, dengan alasannya di dialog, jika:

| Alasan | Yang harus dilakukan |
|---|---|
| Ada paket ekstensi yang terinstal untuk versi lama | pg_upgrade membutuhkan ekstensi yang sama yang dibuat untuk versi baru, dan panel tidak menginstalnya. Tingkatkan versi secara manual dengan `pg_upgradecluster` setelah menginstal ekstensi itu untuk versi baru, atau hapus ekstensinya terlebih dahulu |
| Ada lebih dari satu cluster | Panel hanya mengelola `main`. Tingkatkan versi secara manual |
| Ruang disk tidak cukup | Kosongkan ruang sebesar yang disebutkan pesannya |

Basis data yang tidak bisa dibawa `pg_upgrade`, misalnya tabel dengan kolom `regproc`, membuat langkah 4 gagal; panel lalu mengembalikan keadaan seperti dijelaskan di atas.

### Dump yang disimpan {#the-dumps-kept}

File yang disimpan tetap ada di `/var/backups/nefo-postgresql` dalam kedua kasus, dan tab **PostgreSQL** mencantumkannya di bawah **Dump yang disimpan**: untuk peningkatan versi mana file itu dibuat (seperti "PostgreSQL 15 → 17"), kapan, ukurannya, dan path-nya. Hapus dengan **Hapus dump ini** setelah Anda yakin semuanya berfungsi.

Untuk memulihkan semuanya dari file itu di server baru secara manual:

```sh
zcat postgresql-15-before-17-20261127-101500.sql.gz | runuser -u postgres -- psql -X -d postgres
```

## PostgreSQL di Adminer {#postgresql-in-adminer}

Adminer membutuhkan ekstensi PHP `pgsql` untuk PostgreSQL. Panel menginstalnya untuk versi PHP yang menjalankan Adminer, dan hanya bersama PostgreSQL: saat Anda menginstal PostgreSQL selagi Adminer ada, atau Adminer selagi PostgreSQL ada. Adminer yang diinstal versi panel sebelumnya, yaitu build khusus MySQL, diganti pada saat yang sama.

**Buka di Adminer** di baris basis data membuka halaman masuk PostgreSQL Adminer dengan server `localhost`, pengguna, dan basis data sudah terisi. Ketik kata sandi dari **Tampilkan login**.

## PostgreSQL dalam cadangan {#postgresql-in-backups}

Cadangan situs web menyertakan basis data PostgreSQL milik situs web tersebut, dan cadangan basis data bisa memuat satu basis data PostgreSQL, sesuai permintaan atau sesuai jadwal. Dump-nya berupa keluaran `pg_dump` biasa, dibuat sebagai role basis data, disimpan sebagai `postgres/<name>.sql` di arsip di samping dump MariaDB.

Pencadangan, ekspor, dan impor memakai `pg_dump` dan `psql` dari versi yang terinstal, dari distribusi atau dari apt.postgresql.org, dan port cluster, juga jika port itu bukan 5432.

Pemulihan mengosongkan setiap basis data (basis data dihapus dan dibuat lagi, dengan pemilik role yang sama) dan mengimpor dump sebagai role tersebut. Basis data yang dihapus sejak pencadangan dibuat lagi, dengan kata sandi role-nya dari cadangan jika cadangan memuatnya (mulai v0.24.0), dan jika tidak, dengan kata sandi baru.

## Mencopot PostgreSQL {#uninstall-postgresql}

Copot PostgreSQL di halaman **Perangkat lunak**. Panel menolak selama PostgreSQL masih menyimpan basis data apa pun selain `postgres`, `template0`, dan `template1`, termasuk basis data yang dibuat di luar panel. Panel juga menolak selama PostgreSQL berhenti, karena saat itu panel tidak bisa memeriksanya. Data di `/var/lib/postgresql` dan pengaturan di `/etc/postgresql` tetap ada kecuali Anda mencentang kotaknya dan mengetik `postgresql`. Lihat [Perangkat lunak](./software#postgresql).

Pencopotan juga menghapus repositori apt.postgresql.org dan kuncinya, jika panel yang menambahkannya, serta aturan firewall yang tersisa untuk basis data PostgreSQL. Jika ekstensi PHP `pgsql` (yang ditambahkan untuk Adminer) masih membutuhkan `libpq5`, panel mengembalikan `libpq5` ke versi milik distribusi, sehingga paket itu kembali mendapat pembaruan keamanan sistem Anda.

Toko aplikasi juga punya PostgreSQL sebagai layanan Docker di 127.0.0.1, untuk aplikasi yang menginginkan servernya sendiri. Yang itu tidak muncul di halaman **Basis data**: lihat [Toko aplikasi](./apps#install-a-service).
