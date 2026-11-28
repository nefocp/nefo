---
title: Sistem
description: Instal pembaruan sistem operasi, aktifkan pembaruan keamanan otomatis, tambahkan swap, atur zona waktu, sinkronisasi jam, dan nama host, aktifkan TCP BBR, dan kosongkan ruang disk.
---

# Sistem

Halaman **Sistem** menginstal pembaruan sistem operasi, dengan pembaruan keamanan yang ditandai dan diperiksa setiap beberapa jam, mengaktifkan pembaruan keamanan otomatis, membuat file swap, mengatur zona waktu, sinkronisasi jam, dan nama host, mengaktifkan TCP BBR, dan mengosongkan ruang disk. Setiap perubahan di halaman ini dicatat di log aktivitas.

Halaman ini ada di kelompok **Server** pada menu kiri. Halaman ini punya lima tab: **Pembaruan**, **Swap**, **Waktu & nama**, **Jaringan**, dan **Pembersihan**. Log aktivitas ada di **Akun** → **Aktivitas terbaru**.

Perubahan yang lama, seperti menginstal pembaruan atau membuat file swap, berjalan sebagai tugas latar belakang: halaman menampilkan progresnya, dan hanya satu tugas yang berjalan dalam satu waktu.

## Pembaruan {#updates}

Tab **Pembaruan** mencantumkan pembaruan sistem operasi yang tertunda: setiap paket, versi terinstal dan versi barunya, serta asal versi baru itu (suite-nya, seperti `noble-updates`). Pembaruan dari suite keamanan (`noble-security`, `bookworm-security`…) ditandai **Keamanan** dan dicantumkan lebih dulu. Paket yang ditandai **baru** diinstal bersama sebuah pembaruan, seperti kernel baru.

### Cara panel memeriksa pembaruan {#how-the-panel-checks-for-updates}

- Panel memeriksa dengan sendirinya: setiap jam panel melihat pemeriksaan terakhirnya, dan jika sudah lebih dari 6 jam, panel menjalankan `apt-get update` dan bertanya ke apt apa yang akan diinstal oleh upgrade. Panel melewati pemeriksaan selama ada tugas lain yang berjalan, dan dalam 30 menit pertama setelah panel dijalankan.
- Membuka halaman tidak pernah menjalankan apt: halaman menampilkan pemeriksaan terakhir, beserta waktunya.
- **Periksa sekarang** langsung memeriksa, sebagai tugas.
- **Ditahan untuk sementara** mencantumkan pembaruan yang belum diinstal apt: paket yang ditahan karena membutuhkan perubahan lain, dan phased update Ubuntu, yang sampai ke server secara bertahap selama beberapa hari.

Jika ada pembaruan keamanan yang menunggu, **Perlu perhatian** di dasbor dan ikon lonceng menampilkan "N pembaruan keamanan tersedia".

### Menginstal pembaruan {#install-updates}

- **Instal semua pembaruan** menjalankan `apt-get upgrade --with-new-pkgs`. Perintah ini menginstal paket baru yang dibutuhkan sebuah pembaruan, seperti kernel baru, dan tidak pernah menghapus paket.
- **Instal pembaruan keamanan saja** hanya memperbarui paket terinstal yang versi barunya berasal dari suite keamanan (`apt-get install --only-upgrade` dengan nama-nama paketnya).

Keduanya menyegarkan daftar paket terlebih dahulu, berjalan tanpa pertanyaan, dan mempertahankan file konfigurasi yang sudah Anda ubah. Daftar diperiksa lagi setelah selesai.

::: warning Layanan dimulai ulang sebentar
Memperbarui Nginx, Apache, OpenLiteSpeed, PHP, MariaDB, atau Redis akan memulai ulang layanan itu, jadi situs web mungkin tidak merespons selama beberapa detik. Memperbarui Docker memulai ulang Docker dan kontainernya. Halaman menyebutkan layanan yang akan dimulai ulang oleh pembaruan yang tertunda, begitu pula konfirmasinya. Instal pembaruan seperti itu pada waktu yang sepi.
:::

### Perlu dimulai ulang {#restart-required}

Sebagian pembaruan, seperti kernel baru atau library C baru, baru lengkap setelah server dimulai ulang. Tab ini menampilkan "Server perlu dimulai ulang untuk menyelesaikan instalasi pembaruan." jika:

- `/var/run/reboot-required` ada (Ubuntu menulisnya, dan mencantumkan paketnya di `/var/run/reboot-required.pkgs`), atau
- kernel yang lebih baru daripada yang sedang berjalan terinstal di `/boot` (Debian tidak selalu menulis file itu).

**Mulai ulang di Pengaturan** membuka **Pengaturan** → **Panel** → **Daya**, tempat Anda bisa me-reboot server. Ikon lonceng juga menampilkan "Server perlu dimulai ulang" sampai Anda melakukannya.

### Pembaruan keamanan otomatis {#automatic-security-updates}

Sakelar **Pembaruan keamanan otomatis** membuat server menginstal pembaruan keamanan dengan sendirinya, setiap hari:

- Jika `unattended-upgrades` belum terinstal, panel menginstalnya terlebih dahulu, sebagai tugas. Helper-nya yang menunggu server dimatikan memakai sekitar 20 MB memori.
- Panel mengatur `APT::Periodic::Update-Package-Lists "1";` dan `APT::Periodic::Unattended-Upgrade "1";` di `/etc/apt/apt.conf.d/20auto-upgrades`, dengan mempertahankan baris lain di file itu, dan mengaktifkan timer harian apt.
- Mematikan sakelar mengatur `APT::Periodic::Unattended-Upgrade "0";`.

Pembaruan mana yang diinstal ditentukan oleh pengaturan unattended-upgrades sendiri di `/etc/apt/apt.conf.d/50unattended-upgrades`: di Ubuntu dan Debian, hanya pembaruan keamanan. Server tidak di-reboot kecuali Anda mengaktifkan `Unattended-Upgrade::Automatic-Reboot` di sana. Jika file lain di `/etc/apt/apt.conf.d` menimpa pengaturan itu, panel menyebutkannya dan membiarkan file seperti apa adanya.

## Swap {#swap}

Swap adalah ruang disk yang dipakai kernel saat memori habis. Di server dengan memori 1 atau 2 GB, swap 1 sampai 2 GB mencegah MariaDB dan PHP dihentikan oleh kernel saat server sibuk dan butuh memori lebih banyak daripada yang dimiliki server.

Tab **Swap** menampilkan memori, swap yang terpakai, dan setiap file atau partisi swap dari `/proc/swaps`.

### Membuat file swap {#create-a-swap-file}

1. Pilih **Ukuran**: 512 MB, 1 GB, 2 GB, 4 GB, atau 8 GB. Hanya ukuran yang menyisakan 1 GB disk kosong yang ditawarkan.
2. Pilih **Buat file swap**.

Panel membuat `/panel.swap`, yang hanya bisa dibaca root, dengan `fallocate`. Jika sistem file tidak mendukungnya, atau `swapon` menolak file seperti itu, panel menulis file dengan `dd` sebagai gantinya, yang butuh sekitar satu menit per beberapa gigabyte. Di btrfs, file lebih dulu ditandai bukan copy-on-write. Lalu panel menjalankan `mkswap` dan `swapon`, serta menambahkan `/panel.swap none swap sw 0 0` ke `/etc/fstab`, agar swap aktif lagi setelah server dimulai ulang.

Panel menolak membuat file swap:

- jika disk tidak punya ruang kosong sebesar ukuran file ditambah 1 GB;
- jika server sudah punya file swap yang tidak dibuat panel, seperti `/swapfile` atau `/swap.img`. Panel membiarkannya. Partisi swap tidak menghalanginya;
- di ZFS, tempat file swap tidak berfungsi;
- di dalam kontainer (OpenVZ, LXC, Docker), tempat kernel tidak mengizinkan `swapon`: "Server ini tidak mengizinkan swap". Minta swap kepada penyedia Anda, atau gunakan server virtual penuh.

Jika ada yang gagal, panel menghapus file itu lagi.

### Menghapus file swap {#remove-the-swap-file}

**Hapus file swap** mematikan swap dengan `swapoff`, menghapus barisnya dari `/etc/fstab`, dan menghapus `/panel.swap`. Isi swap dipindahkan kembali ke memori terlebih dahulu, jadi panel menolak jika isinya tidak muat di memori bebas.

### Swappiness {#swappiness}

**Swappiness** (`vm.swappiness`, dari 0 sampai 100) mengatur seberapa mudah kernel memindahkan memori ke swap. Bawaannya 60; 10 cocok untuk server, yang lalu hanya memakai swap saat memori kurang. Panel menulis nilainya ke `/etc/sysctl.d/90-panel-swap.conf` dan langsung menerapkannya.

## Waktu dan nama host {#time-and-host-name}

Tab **Waktu & nama** menampilkan waktu server, zona waktunya, dan apakah jam sudah tersinkronisasi.

### Zona waktu {#time-zone}

Pilih zona waktu dari daftar (`timedatectl list-timezones`) dan pilih **Simpan**. Panel mengaturnya dengan `timedatectl set-timezone` dan memulai ulang cron, jadi tugas cron mengikuti zona waktu baru. Layanan lain yang sudah berjalan, seperti MariaDB, memakai zona waktu baru setelah dimulai ulang.

### Sinkronisasi jam {#clock-synchronization}

**Sinkronkan jam (NTP)** mengaktifkan atau menonaktifkan sinkronisasi waktu (`timedatectl set-ntp`). Biarkan aktif: sertifikat dan kode dua faktor membutuhkan jam yang benar. Fitur ini membutuhkan layanan waktu seperti `systemd-timesyncd`. Tanpa layanan itu, sakelar berwarna abu-abu: instal dengan `apt install systemd-timesyncd`.

### Nama host {#host-name}

Masukkan **Nama host** baru dan pilih **Simpan**. Nama host terdiri dari huruf, angka, dan tanda hubung, dengan titik di antara bagian-bagiannya, paling banyak 64 karakter, dan tidak boleh diawali atau diakhiri tanda hubung: `web1` atau `web1.example.com`.

Panel:

1. mengaturnya dengan `hostnamectl set-hostname`;
2. mengarahkan baris `127.0.1.1` di `/etc/hosts` ke nama itu (nama lengkap dan bagian pertamanya), atau menambahkan baris itu;
3. di server dengan cloud-init, menulis `/etc/cloud/cloud.cfg.d/99-panel-hostname.cfg` dengan `preserve_hostname: true`, agar cloud-init tidak mengembalikan nama dari penyedia pada boot berikutnya.

Nama host tidak mengubah alamat panel atau sertifikat apa pun. Saat Anda me-reboot server dari **Pengaturan** → **Panel** → **Daya**, Anda mengonfirmasi dengan mengetik nama server: setelah perubahan, ketik nama yang baru.

## Jaringan: TCP BBR {#network-tcp-bbr}

Tab **Jaringan** menampilkan TCP congestion control yang dipakai (biasanya `cubic`), antrean bawaan (`net.core.default_qdisc`, biasanya `fq_codel`), dan algoritme yang dimiliki kernel.

BBR adalah congestion control dari Google. BBR sering membuat situs web dan unduhan lebih cepat pada koneksi yang jauh atau sering kehilangan paket, seperti ke pengunjung di benua lain atau di jaringan seluler. Mengaktifkan **TCP BBR**:

1. memuat modul `tcp_bbr`, jika kernel memilikinya sebagai modul;
2. menulis `/etc/modules-load.d/panel-bbr.conf` (`tcp_bbr`) dan `/etc/sysctl.d/90-panel-bbr.conf` (`net.core.default_qdisc = fq` dan `net.ipv4.tcp_congestion_control = bbr`);
3. menjalankan `sysctl --system`.

Koneksi baru langsung memakai BBR. Antrean `fq` berlaku untuk kartu jaringan setelah server dimulai ulang berikutnya; sementara itu BBR tetap bekerja baik tanpanya.

Panel menolak jika kernel yang berjalan tidak punya BBR, dan di dalam kontainer, tempat pengaturan kernel tidak bisa diubah. Dalam hal itu panel tidak menyimpan apa pun. Menonaktifkan BBR menghapus kedua file dan kembali ke `cubic` dan `fq_codel`. Jika BBR disiapkan di luar panel, sakelarnya berwarna abu-abu.

## Pembersihan disk {#disk-cleanup}

Tab **Pembersihan** menampilkan berapa banyak ruang yang dikosongkan setiap pembersihan. **Bersihkan** meminta Anda mengonfirmasi, menjalankan pembersihan sebagai tugas, lalu menampilkan ruang yang dikosongkan.

| Item | Apa yang dihapus |
|---|---|
| **Paket yang diunduh** | File paket yang disimpan apt di `/var/cache/apt/archives` setelah instalasi (`apt-get clean`). |
| **Jurnal sistem** | Entri lama jurnal sistem, hingga tersisa 100 MB (`journalctl --rotate`, lalu `journalctl --vacuum-size=100M`). |
| **Log lama** | Log di `/var/log` yang sudah dirotasi logrotate: nama yang berakhiran `.gz` atau angka, seperti `syslog.1` atau `auth.log.2.gz`. |
| **Paket yang tidak terpakai** | Paket yang tidak dibutuhkan apa pun lagi, seperti kernel lama (`apt-get autoremove`). |
| **Docker** | Image yang tidak dipakai kontainer mana pun, dan build cache (`docker image prune --all`, `docker builder prune --all`). Hanya ditampilkan jika Docker terinstal. |

- **Log lama** mempertahankan log server web di `/var/log/nginx`, `/var/log/apache2`, dan `/var/log/openlitespeed`, karena statistik lalu lintas situs web membacanya. Pembersihan ini hanya menghapus file di folder milik root dan tidak pernah mengikuti tautan.
- Untuk **Paket yang tidak terpakai**, konfirmasi mencantumkan paket-paketnya. Jika apt akan menghapus hal lain saat Anda mengonfirmasi, tidak ada yang dihapus: "Daftar paket yang tidak terpakai telah berubah".
- **Docker** tidak pernah menghapus kontainer, baik yang berhenti maupun tidak, atau volume, tempat kontainer menyimpan datanya.

## Log aktivitas {#activity-log}

Tindakan-tindakan ini muncul di **Akun** → **Aktivitas terbaru**: menginstal pembaruan sistem, mengaktifkan atau menonaktifkan pembaruan keamanan otomatis, membuat atau menghapus file swap, mengubah swappiness, mengubah zona waktu, mengaktifkan atau menonaktifkan sinkronisasi jam, mengubah nama host, mengaktifkan atau menonaktifkan TCP BBR, dan membersihkan ruang disk.
