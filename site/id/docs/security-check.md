---
title: Pemeriksaan keamanan
description: Pemeriksaan keamanan Nefo menemukan pengaturan yang membuat server terbuka (SSH, firewall, pembaruan, basis data yang terbuka, port Docker, file yang terekspos, sertifikat, PHP) dan menautkan setiap temuan ke perbaikannya, saat diminta atau setiap hari.
---

# Pemeriksaan keamanan

Pemeriksaan keamanan, baru di v0.24.0, menjalankan 22 pemeriksaan cepat pada server: SSH yang menerima kata sandi, firewall yang nonaktif, pembaruan keamanan yang menunggu, basis data atau Redis yang terbuka ke internet, port Docker yang melewati ufw, file situs web yang bisa diunduh siapa saja, sertifikat yang akan kedaluwarsa, dan pengaturan PHP. Setiap temuan punya tingkat keparahan dan tombol ke halaman yang memperbaikinya.

Pemeriksaan berjalan saat Anda memulainya, dan secara bawaan sekali sehari. Pemeriksaan ini tidak mengirim apa pun ke luar server Anda. Buka **Pemeriksaan keamanan** di kelompok **Keamanan** pada menu kiri.

## Menjalankan pemeriksaan {#run-the-check}

1. Klik **Jalankan pemeriksaan**. Pemeriksaan butuh beberapa detik.
2. Baca temuan di tab **Perlu diperbaiki**. Setiap temuan punya tingkat keparahan:
   - **Kritis**: perbaiki sekarang. Misalnya, root bisa masuk lewat SSH dengan kata sandi, atau sebuah situs web menyajikan dump basis data.
   - **Peringatan**: segera perbaiki. Misalnya, firewall nonaktif atau ada pembaruan keamanan yang menunggu.
   - **Catatan**: patut diketahui, sering kali tidak masalah dibiarkan. Misalnya, SSH mendengarkan di port 22.
3. Klik tombol pada sebuah temuan, seperti **Buka Firewall & SSH**, untuk membuka tempat memperbaikinya. Lalu jalankan pemeriksaan lagi.

Penghitung di bagian atas menampilkan masalah **Kritis**, **Peringatan**, dan **Catatan** yang masih terbuka, serta berapa pemeriksaan yang **Lolos**. Klik **Lolos** di bagian bawah halaman untuk melihat setiap pemeriksaan yang tidak menemukan apa pun untuk diperbaiki. Pemeriksaan yang tidak berlaku untuk server (tidak ada Redis, tidak ada situs web, tidak ada Docker…) dicantumkan di bawah "Tidak berlaku di server ini".

## Apa saja yang diperiksa pemeriksaan keamanan? {#what-is-checked}

| Pemeriksaan | Tingkat keparahan | Tempat memperbaikinya |
|---|---|---|
| root bisa masuk lewat SSH dengan kata sandi | Kritis | **Firewall & SSH** → SSH |
| SSH menerima kata sandi untuk akun lain | Peringatan | **Firewall & SSH** → SSH |
| SSH mendengarkan di port 22 | Catatan | **Firewall & SSH** → SSH |
| Firewall (ufw) tidak terinstal, atau nonaktif | Peringatan | **Firewall & SSH** |
| fail2ban tidak terinstal, tidak berjalan, atau tidak mengawasi SSH (tidak ada jail `sshd`) | Peringatan | **Firewall & SSH** → fail2ban, atau **Layanan** |
| Panel diakses tanpa HTTPS | Kritis | `/etc/nefo/config.json` (lihat di bawah) |
| Panel memakai sertifikatnya yang ditandatangani sendiri | Catatan | **Pengaturan** |
| Sebuah akun panel tidak memakai masuk dua faktor | Kritis | **Akun** |
| Pembaruan keamanan menunggu | Peringatan | **Sistem** |
| Pemeriksaan pembaruan terakhir sudah lebih dari seminggu, atau belum pernah berjalan | Catatan | **Sistem** |
| Pembaruan keamanan otomatis nonaktif | Peringatan | **Sistem** |
| Server perlu dimulai ulang untuk menyelesaikan pembaruan | Catatan | **Pengaturan** |
| Akun selain root punya user ID 0 | Kritis | **Terminal** |
| Sebuah akun punya kata sandi kosong | Kritis | **Terminal** (`passwd -l <account>`) |
| MariaDB, MySQL, PostgreSQL, Redis, Valkey, Memcached, atau MongoDB mendengarkan di alamat selain 127.0.0.1 | Kritis jika firewall membiarkan semua orang menjangkau port-nya, selain itu Catatan | **Basis data** |
| Redis tidak punya kata sandi | Kritis di alamat publik yang dilewatkan firewall, Peringatan di alamat publik lain, Catatan jika hanya mendengarkan di server ini | **Basis data** → Redis |
| Sebuah kontainer Docker memublikasikan port di semua alamat | Peringatan | **Docker** |
| Sebuah situs web tidak punya HTTPS | Peringatan | **Situs web** → (situs) → **SSL / HTTPS** |
| Sebuah situs web punya sertifikat tetapi tidak mengalihkan ke HTTPS | Catatan | **Situs web** → (situs) → **SSL / HTTPS** |
| Sebuah sertifikat sudah kedaluwarsa | Kritis | **Situs web** → (situs) → **SSL / HTTPS** |
| Sebuah sertifikat kedaluwarsa dalam 14 hari atau kurang | Peringatan | **Situs web** → (situs) → **SSL / HTTPS** |
| Sebuah situs web menyajikan file yang seharusnya privat | Kritis untuk rahasia, Peringatan untuk salinan dan arsip | **File** |
| Sebuah situs web punya file atau folder yang bisa ditulis siapa saja | Peringatan | **File** |
| PHP menampilkan kesalahan ke pengunjung (`display_errors`) | Peringatan | **Perangkat lunak** → PHP → **Kelola** |
| PHP mengumumkan versinya (`expose_php`) | Catatan | **Perangkat lunak** → PHP → **Kelola** |
| Pembaruan inti, plugin, atau tema WordPress menunggu | Peringatan | **Situs web** → (situs) → **WordPress** |
| Mode debug WordPress (`WP_DEBUG`) aktif | Peringatan | **Situs web** → (situs) → **WordPress** |

Beberapa detail:

- **Basis data di jaringan**: pemeriksaan membaca alamat mana saja yang didengarkan setiap program. Untuk alamat publik, pemeriksaan melihat firewall: tanpa firewall, atau dengan aturan yang membiarkan semua orang menjangkau port-nya, temuan ini kritis. Port yang oleh firewall hanya dibuka untuk alamat tertentu (seperti **Akses jarak jauh** sebuah basis data), atau tidak dibuka sama sekali, menjadi catatan. Alamat jaringan privat (10.x, 172.16–31.x, 192.168.x) menjadi catatan.
- **Docker** menulis aturan firewall-nya sendiri di depan aturan ufw, jadi ufw tidak memblokir port yang dipublikasikan sebuah kontainer. Publikasikan port di `127.0.0.1` (misalnya `127.0.0.1:8080:80`) jika hanya situs web di server ini yang memakainya.
- **File yang disajikan situs web**: pemeriksaan mendaftar isi tingkat teratas folder `public` setiap situs web dan meminta ke server web file-file yang tidak boleh publik: file `.env`, folder `.git`, `.svn`, dan `.hg`, `.htpasswd`, dump basis data (`.sql`, `.sql.gz`, `.dump`, `.sqlite`…), salinan `wp-config.php`, salinan cadangan (`.bak`, `.old`, `.orig`, `.save`, `.swp`, nama yang berakhiran `~`), `phpinfo.php` dan `info.php`, serta arsip yang namanya tampak seperti salinan situs (mengandung "backup", "www", "site", "wordpress", atau nama domainnya). Temuan berarti server web menjawab dengan file tersebut. Konfigurasi Nginx milik Nefo menyembunyikan file titik (dot file), jadi di Nginx file `.env` ditemukan tetapi tidak dilaporkan. Situs web yang ditangguhkan, dialihkan ke tempat lain, meminta kata sandi, atau berupa aplikasi di balik proxy tidak ditanyakan.
- **File yang bisa ditulis siapa saja**: pemeriksaan menelusuri folder situs web (`/var/www/<domain>`) tanpa mengikuti tautan simbolik. Folder dengan sticky bit, seperti `tmp` bersama, tidak masalah. Gunakan 644 untuk file dan 755 untuk folder.
- **PHP**: pemeriksaan membaca file `php.ini` dan `conf.d` setiap versi PHP, serta pool PHP-FPM yang mengaktifkan `display_errors` untuk satu situs web.
- **WordPress**: pembaruan diambil dari pemeriksaan terakhir di halaman **WordPress** (atau tab **WordPress** sebuah situs web). Pemeriksaan keamanan tidak pernah menjalankan wp-cli sendiri. Setelah Anda memperbarui sebuah situs, pemeriksaan berikutnya di sana menunjukkan apa yang tersisa.
- **Panel tanpa HTTPS**: Nefo mengaktifkan HTTPS saat diinstal. Jika HTTPS nonaktif, atur `"enabled": true` di bagian `"tls"` pada `/etc/nefo/config.json` lalu jalankan `systemctl restart nefo`. Panel yang hanya mendengarkan di 127.0.0.1 tidak dilaporkan.

## Berapa lama pemeriksaannya? {#limits}

Pemeriksaan membaca data milik panel sendiri dan beberapa file, dan hanya meminta ke server web milik server ini sendiri (127.0.0.1) file yang ada di folder situs web. Setiap pemeriksaan punya batas, jadi satu kali jalan tetap singkat di server dengan banyak situs web:

| Batas | Nilai |
|---|---|
| Satu kali jalan yang Anda mulai | Paling lama 25 detik. Pemeriksaan yang belum selesai saat itu dicantumkan sebagai dihentikan di batasnya, dan hasilnya disebut sebagian |
| Setiap program yang ditanya pemeriksaan (fail2ban, Docker, apt…) | Paling lama 5 detik; program yang tidak menjawab tepat waktu dilewati |
| File dan folder yang ditelusuri di folder satu situs web | 20.000 (100.000 untuk semua situs web bersama-sama) |
| Entri yang dibaca di folder `public` satu situs web | 5.000 |
| Permintaan ke server web milik server ini | Total 200, 4 sekaligus, masing-masing paling lama 3 detik |

## Menerima temuan {#accept-a-finding}

Beberapa temuan tidak masalah di server Anda: SSH di port 22 yang dijaga fail2ban, atau port basis data yang sengaja Anda buka untuk satu alamat. Klik **Terima** pada temuan seperti itu. Temuan pindah ke tab **Diterima** dan tidak lagi dihitung.

Temuan yang diterima muncul kembali saat faktanya berubah, misalnya saat akun lain mendapat user ID 0, atau sebuah basis data mulai mendengarkan di alamat lain. Temuan itu juga muncul kembali jika sudah diperbaiki lalu muncul lagi. Untuk memunculkannya sendiri, buka tab **Diterima** dan klik **Tampilkan lagi**. Menerima dan menampilkan lagi dicatat di log aktivitas.

## Pemeriksaan harian dan peringatan {#daily-check-and-alerts}

Dengan **Jalankan setiap hari** aktif (bawaan), panel menjalankan pemeriksaan sekali sehari di latar belakang, saat pemeliharaan per jamnya, tetapi tidak pernah dalam 10 menit pertama setelah panel dijalankan. Dengan begitu, dasbor selalu menampilkan hasil terbaru.

Aktifkan **Beri tahu saya saat pemeriksaan harian menemukan masalah baru** (bawaannya nonaktif) untuk mendapat pesan di saluran peringatan Anda (**Pengaturan** → **Peringatan**) saat pemeriksaan harian menemukan masalah kritis atau peringatan yang tidak ada pada pemeriksaan sebelumnya. Catatan dan temuan yang diterima tidak pernah dikirim. Lihat [Peringatan](./mail-alerts#alerts).

## Di dasbor {#on-the-dashboard}

Jika pemeriksaan terakhir menemukan masalah kritis atau peringatan, **Perlu perhatian** di dasbor dan lonceng menampilkan "Pemeriksaan keamanan: 3 masalah perlu diperbaiki" dengan tombol **Lihat**. Jika tidak ada masalah serius yang terbuka, ubin **Risiko keamanan** menampilkan kapan pemeriksaan terakhir berjalan, dan mengkliknya membuka pemeriksaan keamanan.

Untuk diberi tahu tentang aktivitas masuk ke SSH dan panel, lihat [Peringatan masuk](./mail-alerts#sign-in-alerts).
