---
title: Mencopot Nefo
description: Hapus Nefo dari server secara manual, dan tentukan dulu apa yang terjadi pada situs web dan perangkat lunak yang disiapkannya.
---

# Mencopot Nefo

Nefo tidak punya perintah untuk mencopot dirinya: Anda menghapusnya secara manual, sebagai root, dalam beberapa langkah. Buka kunci situs web yang terkunci dengan `nefo unlock-files`, hentikan dan hapus layanan `nefo` dan `/usr/local/bin/nefo`, tutup port panel, lalu hapus `/etc/nefo` dan `/var/lib/nefo`. Situs web, basis data, dan perangkat lunak Anda tetap berjalan kecuali Anda menghapusnya terlebih dahulu.

Halaman ini menunjukkan setiap langkahnya. Halaman ini juga menjelaskan apa yang tetap ada di server, karena menghapus panel tidak menghapus situs web dan perangkat lunak yang Anda siapkan dengannya.

## Apa yang tetap ada saat Anda menghapus Nefo {#what-stays-when-you-remove-nefo}

Langkah-langkah di halaman ini menghapus panel itu sendiri: programnya, layanannya, konfigurasinya, dan datanya. Semua yang Anda siapkan lewat panel tetap ada dan tetap berjalan:

- server web, PHP, MariaDB, Redis, Docker, dan komponen lain dari halaman **Perangkat lunak**;
- situs web Anda di `/var/www`, pengguna Linux-nya, dan basis datanya;
- tugas cron, program latar belakang, situs web aplikasi, dan kontainer Docker;
- cadangan yang disimpan di server di `/var/backups/panel`.

Tanpa panel, tidak ada yang memperpanjang sertifikat situs web Anda atau menjalankan pencadangan terjadwal. Sertifikat situs web disimpan di folder data Nefo, `/var/lib/nefo/certs`. Jika Anda menghapus folder itu selagi situs web masih memakai HTTPS, server web tidak bisa memuat sertifikatnya.

Jadi, tentukan dulu:

- **Untuk menghapus semuanya**, hapus situs web dan perangkat lunak Anda di panel terlebih dahulu (langkah 2), lalu hapus Nefo.
- **Agar situs web tetap berjalan** tanpa panel, lewati langkah 2, dan pertahankan `/var/lib/nefo/certs` saat Anda menghapus data (langkah 7). Sertifikat Let's Encrypt berlaku paling lama 90 hari, jadi siapkan cara lain untuk memperpanjangnya sebelum kedaluwarsa.

## 1. Simpan salinan yang Anda butuhkan {#_1-keep-copies-of-what-you-need}

1. Di halaman **Pencadangan**, buat cadangan baru untuk setiap situs web dan basis data yang ingin Anda simpan, dengan **Cadangkan sekarang**.
2. Salin cadangan ke luar server. Cadangan ada di `/var/backups/panel`, dan di penyimpanan jarak jauh Anda jika Anda sudah menyiapkannya.

## 2. Hapus situs web dan perangkat lunak di panel {#_2-remove-websites-and-software-in-the-panel}

Lakukan ini selagi panel masih berjalan. Panel menolak mencopot komponen selama masih ada yang memakainya, jadi kerjakan dengan urutan berikut:

1. **Situs web:** buka setiap situs web, dan di tab **Ringkasan** klik **Hapus**. Centang **Hapus juga basis datanya** jika Anda tidak membutuhkannya. Hapus centang **Cadangkan situs web dulu** jika Anda sudah menyimpan cadangan (langkah 1). Ini menghapus file situs web, pengguna Linux-nya, dan sertifikatnya.
2. **Basis data:** hapus basis data yang tersisa.
3. **Docker:** hapus kontainer dan proyek Compose yang Anda buat.
4. **Tugas cron** dan **Program latar belakang:** hapus yang Anda buat.
5. **Perangkat lunak:** klik **Copot** pada setiap komponen yang tidak Anda butuhkan lagi, seperti Docker, MariaDB, Redis, dan server web. Untuk MariaDB dan Docker, folder datanya (`/var/lib/mysql` untuk MariaDB, `/var/lib/docker` untuk Docker) dipertahankan kecuali Anda mencentang kotak untuk menghapusnya juga.

Versi PHP terakhir tetap ada, karena penyiapan server web membutuhkannya. Jika tidak membutuhkannya, hapus dengan apt setelah Anda menghapus Nefo.

## 3. Catat port panel {#_3-note-the-panel-s-port}

Anda membutuhkan port ini untuk menutupnya di firewall nanti:

```sh
nefo info --port
```

## 4. Buka kunci file situs web {#unlock-the-websites-files}

Jika Anda mengunci file beberapa situs web (**Situs web** → (situs) → **Penguncian file**, lihat [Pemblokiran negara dan penguncian file](./website-protection#lock-a-website-s-files)), file-filenya dimiliki root selama terkunci. Kembalikan file itu ke situs webnya sebelum Anda menghapus panel, kalau tidak, file tetap menjadi milik root:

```sh
systemctl stop nefo
nefo unlock-files
```

Perintah ini mencantumkan setiap situs web yang kuncinya dibuka:

```
Unlocked the files of example.com
Unlocked the files of blog.example.com
```

Setiap file dan folder mendapatkan kembali pemilik, grup, dan izin yang persis sama seperti saat dikunci. File yang dimasukkan root ke situs web sementara itu tetap milik root. Perintah ini juga menghapus grup kunci situs web (`panel-lock-<uid>`). Perintah ini membaca catatan di `/var/lib/nefo/filelock`, jadi jalankan sebelum Anda menghapus data panel (langkah 7). Menjalankannya lagi tidak melakukan apa-apa: perintah ini mencetak `No website's files are locked.`

## 5. Hentikan dan hapus layanan dan programnya {#_5-stop-and-remove-the-service-and-the-program}

```sh
systemctl disable --now nefo
rm /etc/systemd/system/nefo.service
systemctl daemon-reload
rm /usr/local/bin/nefo
```

## 6. Tutup port panel {#_6-close-the-panel-s-port}

Jika ufw aktif, hapus aturan untuk port panel. Penginstal menambahkannya dengan komentar `Nefo`.

1. Tampilkan daftar aturan beserta nomornya:

   ```sh
   ufw status numbered
   ```

2. Hapus aturan untuk port panel berdasarkan nomornya, misalnya:

   ```sh
   ufw delete 3
   ```

3. ufw biasanya punya aturan kedua untuk IPv6, yang ditandai `(v6)`. Nomornya berubah setelah setiap penghapusan, jadi tampilkan lagi daftar aturannya dan hapus juga aturan itu.

Jika Anda memakai firewalld, dengan port 28443 sebagai contoh:

```sh
firewall-cmd --permanent --remove-port=28443/tcp
firewall-cmd --reload
```

Hapus juga port itu dari firewall cloud penyedia Anda.

## 7. Hapus konfigurasi dan data {#_7-delete-the-configuration-and-the-data}

::: danger Ini tidak bisa dibatalkan
Ini menghapus akun panel, pengaturannya, daftar situs web, jadwal pencadangan, log aktivitas, sertifikat milik panel, dan sertifikat situs web Anda. Pastikan Anda sudah menyimpan yang Anda butuhkan (langkah 1).
:::

Untuk menghapus semuanya:

```sh
rm -rf /etc/nefo /var/lib/nefo
```

Jika situs web Anda tetap dipertahankan (lihat di atas), simpan sertifikatnya dan hapus sisanya:

```sh
rm -rf /etc/nefo
find /var/lib/nefo -mindepth 1 -maxdepth 1 ! -name certs -exec rm -rf {} +
```

## 8. Opsional: hapus yang ditinggalkan Nefo di tempat lain {#_8-optional-remove-what-nefo-left-elsewhere}

Selama Anda memakainya, Nefo juga membuat file di luar foldernya sendiri. Hapus hanya yang tidak Anda butuhkan lagi:

| Path | Apa itu |
|---|---|
| `/var/backups/panel` | Cadangan yang disimpan di server ini |
| `/etc/cron.d/panel` dan `/var/log/panel-cron` | Tugas cron yang dibuat di panel, beserta log-nya. Tugas-tugas itu tetap berjalan sampai Anda menghapus file tersebut. |
| `/var/lib/panel-trash` | File yang dihapus di pengelola file, disimpan selama 30 hari |
| `/var/lib/panel-quarantine` | File yang disisihkan oleh pemindai malware |
| `/var/lib/panel-geoip` | Basis data negara untuk pemblokiran negara |
| `/var/backups/nefo-postgresql` | Dump yang dibuat sebelum peningkatan versi PostgreSQL |
| `/var/lib/panel-acme` | Folder yang menjawab pemeriksaan Let's Encrypt. Hapus hanya jika tidak ada lagi server web yang memakainya. |
| `/etc/ssh/sshd_config.d/99-panel-sftp.conf` | Aturan SSH untuk akun situs web dan SFTP |
| `/etc/fail2ban/jail.d/panel.conf` dan `/etc/fail2ban/filter.d/panel.conf` | Aturan fail2ban untuk SSH dan panel, jika Anda menginstal fail2ban dari panel |
| `/etc/ssh/sshd_config.d/00-panel.conf` | Pengaturan SSH yang Anda simpan di panel |

::: warning Pengaturan SSH
Jika Anda mengubah port SSH atau menonaktifkan masuk dengan kata sandi di panel, pengaturan itu ada di `/etc/ssh/sshd_config.d/00-panel.conf`. Menghapus file ini mengubah cara Anda masuk lewat SSH setelah SSH dimulai ulang. Pertahankan file ini kecuali Anda yakin ingin kembali ke pengaturan bawaan, dan uji sesi SSH baru sebelum Anda menutup sesi yang sekarang.
:::
