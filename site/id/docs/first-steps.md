---
title: Langkah pertama
description: Apa yang perlu dilakukan setelah menginstal Nefo, dari halaman penyiapan dan mengamankan akun Anda sampai situs web pertama, pencadangan, dan pemeriksaan keamanan.
---

# Langkah pertama

Setelah Anda [menginstal Nefo](./install), masuklah dengan alamat dan kata sandi yang dicetak penginstal, pilih perangkat lunak di halaman penyiapan, ganti kata sandi, aktifkan autentikasi dua faktor, beri panel domainnya sendiri, lalu buat situs web pertama Anda, siapkan pencadangan, dan jalankan pemeriksaan keamanan. Setiap langkah singkat.

Kerjakan langkah-langkah ini secara berurutan. Setiap langkah menautkan ke halaman dengan penjelasan lengkap.

## Masuk {#sign-in}

1. Buka URL yang dicetak penginstal, seperti `https://203.0.113.10:28443/k3x9p2qa7m1z/`.
2. Terima peringatan sertifikat dari browser. Peringatan ini hilang setelah panel punya sertifikat sendiri (lihat [di bawah](#give-the-panel-its-own-domain)).
3. Masukkan **Nama pengguna** `admin` dan kata sandi dari penginstal, lalu klik **Masuk**.

Jika Anda kehilangan URL atau kata sandi, jalankan `nefo info` atau `nefo reset-password` di server (lihat [Baris perintah](./cli)).

## Siapkan server {#set-up-the-server}

Di server baru, panel membuka **Siapkan server ini** tepat setelah Anda masuk. Di satu halaman, Anda memilih apa yang dibutuhkan server, lalu panel menginstal semuanya sekaligus, satu bagian demi satu bagian.

1. Pilih **Server web**: **Nginx** (disarankan), **Apache** untuk situs web yang membutuhkan file `.htaccess`, **OpenLiteSpeed**, atau **Tidak ada** untuk server tanpa situs web, seperti server yang hanya menjalankan aplikasi Docker.
2. Centang satu atau beberapa **Versi PHP**. Server web selalu disertai versi PHP dari distribusi Anda. Versi lain berasal dari repositori `packages.sury.org/php`, di Ubuntu dan Debian, yang ditambahkan panel ke server. Versi lama yang sudah melewati akhir masa dukungannya tidak ditawarkan di sini: instal dari halaman **Perangkat lunak** jika situs web lama membutuhkannya.
3. Pilih **Server basis data**: **MariaDB** (disarankan), **MySQL** jika halaman Perangkat lunak menawarkannya, atau **Tidak ada**. Di bawah **MariaDB** Anda bisa memilih versinya: versi distribusi Anda (bawaan) atau seri LTS dari repositori milik MariaDB sendiri, seperti **11.4 LTS dari MariaDB**, jika lebih baru daripada versi distribusi Anda. Lihat [Versi MariaDB](./databases#mariadb-versions).
4. Centang **Tambahan** yang Anda inginkan: Redis, Node.js, Python, Docker, Adminer, dan firewall web. ClamAV ditawarkan di server dengan RAM 2 GB atau lebih, sedangkan PostgreSQL dan Memcached ditawarkan jika halaman Perangkat lunak menawarkannya.
5. Periksa ringkasan di bagian bawah. Ringkasan ini menampilkan CPU, RAM, dan ruang disk kosong server, serta perkiraan RAM dan disk yang ditambahkan oleh pilihan Anda. Jika pilihan itu mungkin tidak muat, sebuah peringatan memberi tahu Anda, tetapi Anda tetap bisa menginstal.
6. Klik **Instal**.

Halaman ini dimulai dengan pilihan yang sesuai dengan RAM server: Nginx, versi PHP terbaru, dan MariaDB, ditambah Redis mulai dari RAM sekitar 1,5 GB. Docker dan tambahan lainnya tidak pernah dicentang untuk Anda.

Angka disk menghitung ruang yang dipakai paket setelah terinstal, salinan paket unduhan yang disimpan apt di `/var/cache/apt/archives` (**Paket yang diunduh** di bawah **Pembersihan** di halaman **Sistem** menghapusnya), file data pertama server basis data, dan basis data virus ClamAV. Di server yang daftar paketnya belum pernah diunduh, angka itu juga menghitung daftar tersebut, dan ringkasan menyebutkan berapa bagiannya: sekitar 450 MB di Ubuntu 22.04, 300 MB di 24.04, 250 MB di 26.04, 155 MB di Debian 12, dan 175 MB di Debian 13.

Log menampilkan setiap bagian saat diinstal, dan seluruh penyiapan adalah satu tugas. Jika satu bagian gagal, penyiapan berhenti di sana dan memberi tahu apa yang sudah terinstal. Perbaiki masalahnya, lalu klik **Instal** lagi: bagian yang sudah terinstal akan dilewati.

Untuk menyiapkan server sendiri, klik **Lewati dulu**. Anda bisa menjalankan penyiapan lagi kapan saja dari **Perangkat lunak** → **Jalankan penyiapan lagi**, atau dengan [`nefo setup`](./cli#nefo-setup) di server. Anda juga bisa [menyiapkan server sambil menginstal Nefo](./install#set-the-server-up-in-the-same-run).

Panel berhenti membuka penyiapan setelah masuk begitu penyiapan selesai atau dilewati, dan di server yang sudah punya server web atau situs web.

### VPS yang berupa kontainer {#a-vps-that-is-a-container}

Sebagian VPS adalah kontainer (LXC, OpenVZ), dan `/proc/meminfo`-nya bisa menampilkan memori seluruh host, bukan memori Anda. Dalam hal itu Nefo membaca batas milik kontainer itu sendiri (cgroup memori) dan memakainya di mana-mana: dasbor, grafik riwayat dan peringatan, angka RAM di penyiapan ini, pemeriksaan memori Toko aplikasi, dan pengaturan yang dipilihnya untuk MariaDB, MySQL, PostgreSQL, Redis, dan Memcached. Saat itu terjadi, dasbor menampilkan "batas kontainer" di sebelah RAM. ClamAV ditawarkan mulai dari batas 2 GB.

## Ganti kata sandi {#change-the-password}

Ganti kata sandi dari penginstal dengan kata sandi yang Anda simpan di pengelola kata sandi.

1. Di menu kiri, buka **Akun**. Anda juga bisa membukanya dari menu akun di kanan atas: **Kata sandi dan 2FA**.
2. Di bawah **Ganti kata sandi**, isi **Kata sandi saat ini**, **Kata sandi baru**, dan **Ulangi kata sandi baru**. Kata sandi baru harus terdiri dari 12 sampai 72 karakter.
3. Klik **Ganti kata sandi**.

Panel mengeluarkan Anda dari semua perangkat. Masuk lagi dengan kata sandi baru.

## Aktifkan autentikasi dua faktor {#turn-on-two-factor-authentication}

Dengan autentikasi dua faktor, proses masuk membutuhkan kata sandi Anda dan kode 6 digit dari aplikasi autentikator, seperti Google Authenticator, Authy, atau 2FAS. Selama fitur ini belum aktif, dasbor dan menu notifikasi menampilkan "Masuk dua faktor nonaktif".

1. Buka **Akun**.
2. Di bawah **Autentikasi dua faktor**, klik **Siapkan**.
3. Pindai kode QR dengan aplikasi autentikator Anda, atau ketik **Kunci penyiapan**.
4. Masukkan kode 6 digit yang ditampilkan aplikasi, lalu klik **Aktifkan**.

Mulai sekarang, halaman masuk meminta **Kode dua faktor** setelah kata sandi Anda.

::: warning Sisakan jalan untuk masuk kembali
Jika ponsel Anda hilang, autentikasi dua faktor hanya bisa dinonaktifkan dari server, dengan `nefo reset-password --disable-2fa` (lihat [Pemecahan masalah](./troubleshooting#lost-your-2fa-phone)). Pastikan akses SSH Anda tetap berfungsi, atau simpan kunci penyiapan di tempat yang aman.
:::

## Instal server web dan PHP {#install-a-web-server-and-php}

Situs web membutuhkan server web. Nefo menjalankan satu server web per server. Jika [penyiapan](#set-up-the-server) sudah menginstalnya, lewati langkah ini.

1. Buka **Situs web**. Di server baru, halaman ini menampilkan **Instal server web**.
2. Pilih server web:
   - **Nginx**: cepat dan ringan, pilihan terbaik kecuali situs web Anda membutuhkan file `.htaccess`.
   - **Apache**: membaca file `.htaccess`, yang diandalkan banyak plugin WordPress dan situs dari shared hosting. Apache memakai sedikit lebih banyak memori.
   - **OpenLiteSpeed**: membaca aturan rewrite di file `.htaccess` dan sudah dilengkapi LiteSpeed Cache. OpenLiteSpeed berasal dari repositori paket milik LiteSpeed sendiri.
3. Klik tombol instal, seperti **Instal Nginx**.

Panel menginstal server web dan versi PHP dari distribusi Anda, lalu membuka port 80 dan 443 jika firewall (ufw) aktif. Ini butuh satu sampai tiga menit, dan progresnya tampil di halaman.

Untuk beralih ke server web lain nanti, Anda harus menghapus semua situs web terlebih dahulu. Tambahkan versi PHP lain, MariaDB, dan komponen lain di halaman **Perangkat lunak**. Lihat [Perangkat lunak](./software).

## Beri panel domainnya sendiri {#give-the-panel-its-own-domain}

Dengan domainnya sendiri, panel mendapat sertifikat Let's Encrypt gratis, dan peringatan browser hilang. Anda membutuhkan server web terlebih dahulu, karena server web yang menjawab Let's Encrypt di port 80.

1. Di penyedia DNS Anda, buat record A untuk nama seperti `panel.example.com`, yang mengarah ke alamat IP server. Record ini harus mengarah langsung ke server, tanpa proxy atau CDN di depannya (di Cloudflare, gunakan "DNS only").
2. Pastikan port 80 terbuka, juga di firewall cloud penyedia Anda.
3. Di panel, buka **Pengaturan** → **Sertifikat**.
4. Di **Domain panel**, masukkan `panel.example.com`, lalu klik **Dapatkan sertifikat**.

Panel lebih dulu memeriksa bahwa domain mengarah ke server ini, lalu meminta sertifikat dari Let's Encrypt. Panel mulai memakai sertifikat baru dalam satu menit, dan menampilkan alamat baru, seperti `https://panel.example.com:28443/k3x9p2qa7m1z/`. Gunakan alamat itu mulai sekarang: panel tetap memakai port dan path rahasia yang sama. Jika memakai alamat IP, browser akan memperingatkan lagi, karena sertifikatnya untuk domain.

Sertifikat diperpanjang secara otomatis. Untuk kembali ke sertifikat yang ditandatangani sendiri, klik **Hapus** di halaman yang sama.

Jika permintaan gagal, lihat [Pemecahan masalah](./troubleshooting#a-certificate-request-fails).

## Buat situs web pertama Anda {#create-your-first-website}

1. Arahkan record A domain, seperti `example.com`, ke alamat IP server.
2. Buka **Situs web** dan klik **Tambah situs web**.
3. Masukkan **Domain** tanpa `http://` atau `www.`. Biarkan **Juga merespons di www.example.com** tercentang jika Anda ingin kedua nama.
4. Pilih **Jenis**: versi PHP, **Statis (hanya HTML)**, aplikasi (Node.js, Python, atau Java), atau reverse proxy ke alamat lain.
5. Klik **Buat situs web**.
6. Klik **Aktifkan HTTPS** di sebelah situs web untuk mendapatkan sertifikat gratis.

Unggah file Anda ke `/var/www/example.com/public`, dengan halaman **File** atau lewat SFTP. Untuk menginstal WordPress atau aplikasi lain, gunakan **Toko aplikasi**. Lihat [Situs web](./websites), [Sertifikat SSL](./ssl), dan [Toko aplikasi](./apps).

## Siapkan pencadangan {#set-up-backups}

1. Buka **Pencadangan**.
2. Di bawah **Cadangan otomatis**, klik **Tambah jadwal**.
3. Pilih apa yang akan dicadangkan (situs web, basis data, atau folder), seberapa sering, dan **Jumlah cadangan yang disimpan**.
4. Klik **Simpan jadwal**.

Cadangan situs web berisi file-filenya dan basis data yang tertaut dengannya. Cadangan disimpan di server di `/var/backups/panel`. Tambahkan **Penyimpanan jarak jauh** (kompatibel dengan S3, SFTP, atau Google Drive, dienkripsi jika Anda mau) agar ada salinan yang tetap selamat jika server hilang. Lihat [Pencadangan](./backups).

## Aktifkan firewall {#turn-on-the-firewall}

Jika dasbor menampilkan "Firewall nonaktif", setiap port yang dibuka sebuah program bisa dijangkau dari internet.

1. Buka **Firewall & SSH**.
2. Klik **Aktifkan firewall**.

Panel tetap membuka SSH dan port panel itu sendiri. Jika halaman ini menyatakan bahwa ufw tidak terinstal, instal dulu dengan `apt install ufw`. Lihat [Keamanan](./security).

## Jalankan pemeriksaan keamanan {#run-the-security-check}

1. Buka **Pemeriksaan keamanan**, di kelompok **Keamanan** pada menu kiri.
2. Klik **Jalankan pemeriksaan**.
3. Perbaiki yang tercantum di **Perlu diperbaiki**, mulai dari **Kritis**: setiap temuan punya tombol ke halaman yang memperbaikinya.

Setelah itu pemeriksaan berjalan sendiri sekali sehari, dan dasbor menampilkan temuannya. Lihat [Pemeriksaan keamanan](./security-check).
