---
title: Toolkit WordPress
description: Periksa dan perbarui situs WordPress Anda dengan cadangan basis data terlebih dahulu, dan gunakan mode pemeliharaan, mode debug, pembaruan otomatis, reset kata sandi, dan salinan staging.
---

# Toolkit WordPress

Toolkit WordPress di Nefo menampilkan versi dan pembaruan yang tertunda untuk setiap situs WordPress, dan memperbarui WordPress, plugin, dan tema dengan satu klik setelah mencadangkan basis data. Toolkit ini juga mengaktifkan atau menonaktifkan mode pemeliharaan, mode debug, dan pembaruan otomatis, mereset kata sandi administrator, dan membuat salinan staging yang bisa Anda dorong kembali ke situs live.

Toolkit ini bekerja untuk situs WordPress yang diinstal Nefo: dari [Toko aplikasi](./apps), dengan tombol **Instal WordPress** di situs web PHP yang kosong, atau sebagai salinan staging. WordPress yang Anda unggah sendiri tidak muncul. Item **WordPress** di menu kiri muncul setelah server punya situs WordPress.

Toolkit ini memakai wp-cli, yang diunduh Nefo saat pertama kali dipakai dari rilis terbarunya dan diperiksa terhadap checksum SHA-512 yang dipublikasikan bersama rilis itu. **Perangkat lunak** → **Alat** menampilkan versinya dan memperbaruinya (lihat [Perangkat lunak](./software#tools)). wp-cli berjalan sebagai pengguna Linux milik setiap situs dengan versi PHP situs tersebut, dan tidak memuat plugin atau tema, jadi plugin yang rusak tidak mengganggu.

## Menginstal WordPress {#install-wordpress}

Ada dua cara:

- **Toko aplikasi**: instal WordPress atau WooCommerce ke situs web PHP yang kosong. Lihat [Toko aplikasi](./apps).
- **Dari situs web PHP yang kosong**: buka tab **Ringkasan** situs web dan klik **Instal WordPress**. Isi **Judul situs**, **Nama pengguna admin** (hindari `admin`: itu nama pertama yang dicoba penyerang), **Email admin**, dan **Kata sandi admin**, atau kosongkan kata sandi untuk mendapatkan kata sandi yang kuat. Kata sandi yang Anda ketik harus terdiri dari 12 sampai 128 karakter, seperti di toko aplikasi. Klik **Instal WordPress**.

WordPress membutuhkan MariaDB atau MySQL: instal dulu di halaman **Basis data** (lihat [Basis data](./databases)). Aktifkan HTTPS sebelum menginstal, agar WordPress dimulai dengan alamat `https://`-nya; jika Anda mengaktifkannya nanti, Nefo mengalihkan WordPress untuk Anda. Simpan detail login saat ditampilkan: kata sandi tidak akan ditampilkan lagi.

## Daftar situs {#the-site-list}

Buka **WordPress** di menu kiri. Untuk setiap situs, tabel menampilkan:

| Kolom | Isinya |
|---|---|
| **Situs web** | Domain dan versi PHP. Klik untuk membuka tab **WordPress** situs web. |
| **Versi** | Versi WordPress, dengan "WordPress … tersedia" jika ada versi yang lebih baru, atau "Terbaru". |
| **Plugin dan tema** | Berapa jumlahnya dan berapa yang punya pembaruan. **Tampilkan … pembaruan yang tersedia** mencantumkannya. |
| **Mode** | **Pemeliharaan**, **Debug**, dan **Pembaruan otomatis** jika aktif. |

Nefo memeriksa setiap situs saat halaman dibuka. **Periksa lagi** di bagian atas memeriksa ulang semua situs; item yang sama di menu sebuah situs hanya memeriksa situs itu. Jika sebuah situs tidak bisa diperiksa, misalnya karena basis datanya mati, barisnya menampilkan "Tidak bisa memeriksa situs:" beserta alasannya.

Menu di ujung setiap baris berisi tindakan di bawah ini. Tindakan dan sakelar yang sama ada di tab **WordPress** setiap situs web, bersama **WP Admin**.

## Memperbarui WordPress, plugin, dan tema {#update-wordpress-plugins-and-themes}

1. Klik **Perbarui semua** di baris situs, atau di tab **WordPress** situs web.
2. Konfirmasi: "Cadangkan basis data example.com, lalu perbarui WordPress, semua plugin, dan semua tema?"

Pembaruan berjalan di latar belakang dengan log:

1. Nefo mengekspor basis data ke `/var/www/example.com/tmp/panel-wp-backup-YYYYMMDD-HHMMSS.sql`. Hanya pengguna situs yang bisa membaca file ini, dan file ini tidak bisa dijangkau dari web. Nefo menyimpan tiga file terbaru seperti ini. Jika pencadangan gagal, tidak ada yang diperbarui.
2. Nefo memperbarui WordPress dan basis datanya, lalu setiap plugin dan setiap tema.
3. Situs yang sedang dalam mode pemeliharaan tetap dalam mode itu.

Jika beberapa plugin atau tema tidak bisa diperbarui, log menyebutkan alasannya dan nama cadangan basis datanya. Pada situs web yang [file-nya dikunci](./website-protection#lock-a-website-s-files), toolkit membuka kunci file untuk pembaruan, mode pemeliharaan, mode debug, dan pembaruan otomatis, lalu menguncinya lagi sesudahnya. WordPress sendiri tidak bisa memperbarui apa pun selama file dikunci: perbarui dari sini. Untuk kembali, unduh cadangan itu dari **File** dan impor di halaman **Basis data**: lihat [Basis data](./databases). Untuk salinan lengkap file juga, gunakan [Pencadangan](./backups).

## Mode pemeliharaan {#maintenance-mode}

Pilih **Aktifkan mode pemeliharaan**, atau aktifkan sakelar **Mode pemeliharaan** di tab **WordPress** situs web. Pengunjung lalu melihat halaman pemeliharaan WordPress (HTTP 503). Mode ini tetap aktif sampai Anda menonaktifkannya di panel dengan **Nonaktifkan mode pemeliharaan**.

Mode pemeliharaan adalah halaman milik WordPress sendiri. Untuk menutup seluruh situs web di tingkat server web, tangguhkan situs web: lihat [Situs web](./websites).

## Mode debug {#debug-mode}

Pilih **Aktifkan mode debug**, atau aktifkan sakelar **Mode debug**. Nefo mengatur `WP_DEBUG` dan `WP_DEBUG_LOG` di `wp-config.php` dan membiarkan `WP_DEBUG_DISPLAY` nonaktif, jadi kesalahan ditulis ke log dan tidak pernah ditampilkan kepada pengunjung:

- kesalahan dari permintaan web masuk ke `/var/www/example.com/logs/php-error.log`: buka tab **Log** situs web dan pilih **Kesalahan PHP**;
- kesalahan dari eksekusi wp-cli masuk ke `/var/www/example.com/logs/wp-debug.log`: buka di **File**.

Nonaktifkan dengan **Nonaktifkan mode debug** setelah selesai.

## Pembaruan otomatis {#automatic-updates}

**Aktifkan pembaruan otomatis** membiarkan WordPress memperbarui semuanya sendiri: WordPress, termasuk versi mayor, setiap plugin, dan setiap tema.

**Nonaktifkan pembaruan otomatis** mengembalikan WordPress ke perilaku bawaannya: WordPress tetap menginstal rilis keamanan dan rilis minor WordPress itu sendiri, sedangkan plugin dan tema tidak diperbarui dengan sendirinya.

Pengaturan ini berlaku untuk plugin dan tema yang terinstal saat Anda mengaktifkannya. Untuk plugin dan tema yang Anda tambahkan kemudian, nonaktifkan lalu aktifkan lagi.

## Mereset kata sandi administrator {#reset-an-administrator-password}

1. Pilih **Reset kata sandi administrator**.
2. Pilih akun di bawah **Administrator**. Daftar ini menampilkan pengguna dengan peran administrator.
3. Pilih **Buat kata sandi kuat**, atau **Ketik kata sandi** dengan 12 sampai 128 karakter.
4. Klik **Atur kata sandi**.

Nefo menampilkan kata sandi baru sekali, dengan tombol **Masuk**. Akun tersebut dikeluarkan dari semua perangkat.

## Membuat salinan staging {#make-a-staging-copy}

Salinan staging adalah situs web kedua berisi salinan situs, tempat Anda bisa mencoba perubahan tanpa menyentuh situs live. Situs live tetap berjalan selama penyalinan.

1. Pilih **Kloning ke staging**.
2. Periksa **Domain situs staging**. Domainnya `staging.example.com` kecuali Anda mengubahnya.
3. Klik **Buat salinan staging**.
4. Arahkan DNS domain staging ke server ini, lalu aktifkan HTTPS di tab **SSL / HTTPS**-nya.

Salinan mendapatkan:

- situs web PHP baru dengan versi PHP yang sama, dan pengguna Linux sendiri;
- basis data baru dengan salinan kontennya;
- salinan file di `public`, dengan `wp-config.php` diarahkan ke basis data baru. Drop-in cache (`object-cache.php`, `advanced-cache.php`) dan tautan simbolik tidak disertakan;
- alamat situs diganti dengan alamat baru di semua tempat di basis data;
- permintaan kepada mesin pencari agar tidak mengindeksnya;
- aturan rewrite yang sama dengan situs live.

Situs staging dimulai di `http://`, tanpa sertifikat. Jika ada langkah yang gagal, Nefo menghapus semua yang dibuatnya untuk salinan itu.

Nefo menolak membuat salinan jika:

- MariaDB tidak berjalan;
- domain staging sudah dipakai situs web, atau foldernya sudah ada di `/var/www`;
- `wp-config.php` tidak ada di folder `public` situs;
- `DB_NAME` di `wp-config.php` bukan nilai biasa dalam tanda kutip, atau nama basis data memakai karakter selain huruf kecil, angka, dan `_`;
- basis data situs ada di server lain;
- situs merupakan jaringan multisite.

::: tip
Siapa pun bisa membuka situs staging setelah DNS-nya mengarah ke server. Agar tetap privat, lindungi dengan kata sandi di tab **Blokir akses**-nya: lihat [Situs web](./websites).
:::

## Mendorong salinan staging ke situs live {#push-the-staging-copy-to-the-live-site}

Jika perubahan yang Anda coba di salinan staging sudah siap, salin kembali ke situs live. Nefo mengingat dari situs mana setiap salinan staging dibuat. Salinan yang dibuat sebelum Nefo punya tombol ini, dan [hasil kloning](./websites#clone-a-website) situs web, tidak tertaut ke situs live dan tidak bisa didorong.

1. Di baris situs staging, pilih **Dorong ke live**, atau klik **Dorong ke live** di tab **WordPress**-nya. Daftar situs menampilkan "Salinan staging dari example.com" di bawah situs staging.
2. Pilih apa yang akan didorong:
   - **File dan basis data**;
   - **Hanya file (tema, plugin, unggahan)**: situs live mempertahankan pos, halaman, pesanan, dan pengaturannya;
   - **Hanya basis data (pos, halaman, pengaturan)**: situs live mempertahankan file-filenya.
3. Ketik domain situs live untuk konfirmasi, lalu klik **Dorong ke live**.

Pendorongan berjalan sebagai tugas dengan log sendiri:

1. Nefo mencadangkan file dan basis data situs live. Cadangannya ada di halaman [Pencadangan](./backups). Jika pencadangan gagal, tidak ada yang didorong.
2. Situs live menampilkan halaman pemeliharaannya. Jika sebelumnya sudah dalam mode pemeliharaan, situs tetap dalam mode itu setelah pendorongan.
3. **File**: file di folder `public` situs staging menggantikan file situs live, dan dimiliki pengguna Linux situs live. File yang tidak ada di salinan staging dihapus dari situs live; file dengan ukuran dan tanggal yang sama dibiarkan apa adanya. Situs live mempertahankan `wp-config.php`-nya sendiri, drop-in cache-nya (`object-cache.php`, `advanced-cache.php`), dan tautan simboliknya. Di `.user.ini` dan `wordfence-waf.php`, folder staging diganti dengan folder live.
4. **Basis data**: basis data live dikosongkan dan diisi salinan basis data staging. Nefo lalu membalik apa yang dilakukan salinan staging: alamat staging (dengan `http://` dan `https://`, juga dalam bentuk JSON) dan folder staging diganti dengan milik situs live di semua tempat, dan situs live mendapatkan kembali alamatnya sendiri dan pengaturan mesin pencarinya. Cache objek dikosongkan, dan WordPress membangun ulang aturan rewrite-nya pada kunjungan berikutnya.
5. Mode pemeliharaan dinonaktifkan, juga jika ada langkah yang gagal.

::: warning
Pendorongan menggantikan situs live. Apa yang ditambahkan ke situs live sejak salinan staging dibuat akan hilang: pesanan, komentar, dan pengguna baru jika Anda mendorong basis data, unggahan jika Anda mendorong file. Jika situs live tidak berfungsi setelah pendorongan, pulihkan cadangan dari langkah 1 di halaman **Pencadangan**.
:::

Nefo menolak mendorong jika:

- situs bukan salinan staging, atau situs live-nya sudah dihapus;
- Anda mendorong basis data dan MariaDB tidak berjalan, basis data live tidak ada di halaman **Basis data**, kedua situs memakai basis data yang sama, prefiks tabelnya (`$table_prefix`) berbeda, atau salah satu alasan di daftar atas berlaku untuk salah satu `wp-config.php`.

Pengaturan di `wp-config.php` tidak pernah didorong: ubah sendiri di situs live.

## Menghapus salinan staging {#delete-a-staging-copy}

Untuk menghapus salinan staging, hapus situs webnya dan centang **Hapus juga basis datanya**: lihat [Menghapus situs web](./websites#delete-a-website). Menghapus salinan staging tidak mengubah situs live-nya.

Menghapus situs live tidak menghapus salinan staging-nya, tetapi salinan itu tidak bisa lagi didorong: dialog penghapusan menampilkan "Ini adalah situs live dari salinan staging …".
