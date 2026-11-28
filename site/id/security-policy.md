---
title: Kebijakan keamanan
description: Cara melaporkan kerentanan di Nefo, bagaimana rilis ditandatangani dan cara memeriksanya sendiri, serta status keamanan saat ini.
---

# Kebijakan keamanan

Nefo berjalan sebagai root dan mengelola seluruh server, jadi setiap laporan kerentanan diperlakukan sebagai hal mendesak. Halaman ini menjelaskan cara melaporkannya, bagaimana rilis ditandatangani dan cara Anda memeriksanya, serta posisi Nefo saat ini. Halaman ini mengikuti [SECURITY.md](https://github.com/nefocp/nefo/blob/main/SECURITY.md) di repositori.

## Melaporkan kerentanan {#report-a-vulnerability}

Jangan membuka issue, diskusi, atau pull request publik. Laporkan secara privat di GitHub:

1. Buka [repositori Nefo](https://github.com/nefocp/nefo) dan masuk ke tab **Security**.
2. Pilih **Report a vulnerability**, atau gunakan [tautan langsung](https://github.com/nefocp/nefo/security/advisories/new). Hanya pengelola proyek yang melihat laporan Anda.

Sertakan:

- versi yang terdampak (`nefo version`) dan sistem operasinya;
- apa yang bisa dilakukan penyerang, dan akses apa yang mereka butuhkan terlebih dahulu: tidak ada, akun panel, akun shell…;
- langkah-langkah untuk mereproduksi, atau proof of concept;
- apakah Anda berencana memublikasikan temuan Anda, dan kapan.

### Apa yang terjadi selanjutnya {#what-happens-next}

| Langkah | Target |
|---|---|
| Proyek mengonfirmasi bahwa laporan Anda sudah diterima | dalam 3 hari kerja |
| Proyek memberi tahu Anda apakah masalah itu terkonfirmasi dan seberapa parah | dalam 7 hari |
| Proyek merilis perbaikan untuk masalah yang kritis atau berkeparahan tinggi | dalam 14 hari setelah konfirmasi |
| Pengungkapan publik | setelah perbaikan dirilis, atau paling lambat 90 hari setelah laporan Anda, dikoordinasikan dengan Anda |

Setiap kerentanan yang dikonfirmasi mendapat GitHub Security Advisory dengan CVE. Nama Anda dicantumkan, kecuali Anda lebih suka tetap anonim.

### Cakupan {#scope}

Termasuk cakupan: program `nefo`, API dan antarmukanya, penginstal, dan konfigurasi yang dibuatnya di server.

Di luar cakupan:

- peringatan browser tentang sertifikat yang ditandatangani sendiri (self-signed) yang dibuat panel saat pertama kali dijalankan (ini wajar);
- serangan yang membutuhkan akses root ke server, atau akses fisik ke server;
- denial of service yang hanya mengandalkan volume lalu lintas;
- kerentanan di perangkat lunak pihak ketiga yang diinstal panel, seperti Nginx, PHP, atau MariaDB. Laporkan itu ke proyeknya masing-masing, tetapi beri tahu kami jika panel mengonfigurasinya dengan cara yang tidak aman.

### Perlindungan hukum {#safe-harbour}

Proyek tidak akan mengambil tindakan hukum terhadap Anda atas riset keamanan yang mengikuti kebijakan ini: Anda hanya menguji di server milik Anda atau yang Anda punya izin untuk mengujinya, Anda menghindari pelanggaran privasi dan perusakan data, dan Anda memberi proyek waktu yang wajar untuk memperbaiki masalah sebelum Anda mengungkapkannya.

## Versi yang didukung {#supported-versions}

Sampai versi 1.0, hanya rilis terbaru yang menerima perbaikan keamanan. Perbarui dari panel (**Pengaturan** → **Panel** → **Periksa pembaruan**) atau dengan `nefo update` di server. Lihat [Pembaruan](./docs/update).

## Bagaimana rilis ditandatangani {#how-releases-are-signed}

GitHub Actions membangun setiap rilis dari tag versi di repositori. Sebuah rilis berisi:

- `nefo-linux-amd64` dan `nefo-linux-arm64`, yaitu programnya;
- `install.sh`, yaitu penginstal;
- `VERSION`;
- `checksums.txt`, yaitu checksum SHA-256 kedua program dan penginstal;
- `checksums.txt.sig`, yaitu tanda tangan Ed25519 dari `checksums.txt`;
- `appstore-catalog.json` dan `appstore-catalog.json.sig`, yaitu katalog toko aplikasi beserta tanda tangannya sendiri.

Kunci publik proyek tertanam di setiap program Nefo dan disalin ke `install.sh`. Kunci ini juga ada di repositori sebagai [`internal/update/release.pub`](https://github.com/nefocp/nefo/blob/main/internal/update/release.pub):

```
-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEA2c6nGagX7yp8Y8YftuOqW2WVprKe6h0hv+3s9iB+KJA=
-----END PUBLIC KEY-----
```

Tiga hal memeriksa tanda tangan sebelum memakai sebuah unduhan:

- **Penginstal** memeriksa tanda tangan `checksums.txt` dan checksum program. Jika salah satunya tidak benar, penginstal berhenti: "invalid signature: checksums.txt was not signed by the project. Do not install this file." (tanda tangan tidak sah: checksums.txt tidak ditandatangani oleh proyek; jangan instal file ini).
- **`nefo update`**, dan **Instal pembaruan** di panel, melakukan hal yang sama dengan kunci yang tertanam di program yang sudah Anda jalankan, lalu menjalankan program baru sekali sebelum mengganti `/usr/local/bin/nefo`.
- **Toko aplikasi** mengambil katalog sekali sehari dan mengabaikannya kecuali tanda tangannya sah.

### Apa yang dibuktikan dan tidak dibuktikan tanda tangan {#what-the-signature-does-and-doesn-t-prove}

Tanda tangan menunjukkan bahwa file-file itu berasal dari build rilis proyek dan tidak diubah sesudahnya, baik di GitHub maupun dalam perjalanan ke server Anda.

- **Pembaruan** diperiksa dengan kunci di dalam Nefo yang sudah Anda miliki, jadi rilis yang diubah di GitHub tidak bisa membuat dirinya terinstal.
- **Instalasi pertama** berbeda: penginstal yang Anda unduh membawa kunci yang dipakainya untuk memeriksa. Jika seseorang bisa mengubah `install.sh` itu sendiri, mereka juga bisa mengubah kunci itu. Untuk menyingkirkan kemungkinan ini, periksa sendiri `install.sh` sebelum menjalankannya, seperti yang ditunjukkan di bawah.
- Kunci penandatanganan disimpan sebagai secret repositori GitHub, dan rilis ditandatangani di sana oleh GitHub Actions. Seseorang yang mengambil alih akun GitHub proyek bisa menandatangani rilis.

## Memeriksa rilis sendiri {#check-a-release-yourself}

Anda membutuhkan `curl`, `sha256sum`, dan OpenSSL 3 (di Ubuntu 22.04 dan yang lebih baru, serta di Debian 12 dan yang lebih baru). Contoh ini memeriksa rilis terbaru di server amd64, sebagai root; isi `V` dengan tag rilis lain untuk memeriksa rilis tersebut.

1. Unduh checksum, tanda tangannya, dan penginstal:

   ```sh
   V=$(curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/VERSION)
   cd /tmp
   for f in checksums.txt checksums.txt.sig install.sh; do
     curl -fsSLO "https://github.com/nefocp/nefo/releases/download/$V/$f"
   done
   ```

2. Simpan kunci publik dari halaman ini:

   ```sh
   cat > nefo-release.pub <<'EOF'
   -----BEGIN PUBLIC KEY-----
   MCowBQYDK2VwAyEA2c6nGagX7yp8Y8YftuOqW2WVprKe6h0hv+3s9iB+KJA=
   -----END PUBLIC KEY-----
   EOF
   ```

3. Periksa tanda tangan, lalu checksum penginstal:

   ```sh
   openssl pkeyutl -verify -pubin -inkey nefo-release.pub -rawin \
     -in checksums.txt -sigfile checksums.txt.sig
   grep " install.sh$" checksums.txt | sha256sum -c -
   ```

   Perintah pertama harus mencetak `Signature Verified Successfully`, perintah kedua `install.sh: OK`. Jika salah satunya gagal, jangan jalankan penginstal, dan laporkan seperti dijelaskan di atas.

4. Jalankan penginstal yang sudah Anda periksa. Penginstal mengunduh program untuk CPU Anda dan memeriksanya dengan kunci yang sama:

   ```sh
   NEFO_VERSION=$V bash install.sh
   ```

Untuk memeriksa program yang Anda unduh sendiri, seperti `nefo-linux-amd64`, jalankan `grep " nefo-linux-amd64$" checksums.txt | sha256sum -c -` setelah langkah 3.

## Bagaimana Nefo melindungi server {#how-nefo-protects-the-server}

Ringkasan singkat; [Keamanan](./docs/security) memuat detailnya:

- Panel hanya merespons di bawah path rahasia, di port acak. Setiap alamat lain mengembalikan `404`.
- Panel memakai HTTPS sejak pertama kali dijalankan, dengan TLS 1.2 atau lebih baru.
- Proses masuk mendukung autentikasi dua faktor. 5 percobaan gagal dalam 15 menit memblokir sebuah alamat selama 15 menit. Kata sandi disimpan sebagai hash bcrypt.
- Setiap proses masuk dan perubahan dicatat di log aktivitas, yang disimpan selama 90 hari.
- Setiap halaman punya Content Security Policy yang ketat, dan setiap perubahan membutuhkan header yang tidak bisa dikirim situs web lain.
- [Aturan kode](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md#security-rules-for-code) proyek melarang menyusun perintah shell dari masukan: sebuah program menerima setiap nilai sebagai argumen tersendiri, dan masukan diperiksa terhadap daftar yang diizinkan sebelum sampai ke path file, file konfigurasi, atau perintah. Perintah yang Anda tulis sendiri, seperti tugas cron dan skrip deploy, berjalan di shell persis seperti yang Anda tulis.
- Aturan firewall dan pengaturan SSH yang akan mengunci Anda di luar ditolak.
- Setiap situs web berjalan sebagai pengguna Linux sendiri, dan akun SFTP terkunci di folder situs web.

## Status saat ini {#current-status}

Nefo masih beta. Rilis publik pertamanya adalah v0.20.0, pada November 2026; [catatan perubahan](./changelog) mencantumkan rilis saat ini. Belum ada audit keamanan dari pihak luar.

Coba Nefo di server yang bisa Anda bangun ulang sebelum mengandalkannya untuk situs web penting, simpan salinan cadangan Anda di luar server, dan selalu perbarui Nefo. Tinjauan keamanan dari luar proyek adalah salah satu langkah sebelum 1.0 (lihat [peta jalan](./roadmap)). Jika Anda meninjau kode dan menemukan sesuatu, laporkan seperti dijelaskan di atas.
