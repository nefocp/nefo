---
title: Pembaruan
description: Perbarui Nefo dari panel atau dengan nefo update, dan pelajari cara rilis diverifikasi serta apa yang harus dilakukan jika pembaruan gagal.
---

# Pembaruan

Untuk memperbarui Nefo, klik **Periksa pembaruan** di **Pengaturan** → **Panel**, atau jalankan `nefo update` sebagai root. Panel mengunduh rilis terbaru, memeriksa tanda tangan Ed25519 dan checksum SHA-256-nya, mengganti `/usr/local/bin/nefo` dalam satu langkah, lalu memulai ulang. Situs web tetap berjalan. Nefo tidak pernah memperbarui dirinya sendiri.

Halaman ini menunjukkan cara memperbarui Nefo, bagaimana setiap rilis diperiksa sebelum diinstal, dan apa yang harus dilakukan jika pembaruan gagal. Sampai versi 1.0, hanya rilis terbaru yang menerima perbaikan keamanan, jadi selalu perbarui Nefo.

## Memperbarui dari panel {#update-from-the-panel}

1. Buka **Pengaturan** → **Panel**. Versi saat ini ditampilkan di sebelah nama Nefo.
2. Klik **Periksa pembaruan**. Jika Anda sudah memakai rilis terbaru, panel menampilkan "Ini versi terbaru."
3. Jika ada rilis yang lebih baru, panel menampilkan "Versi v0.2.0 tersedia." Klik **Instal pembaruan** dan konfirmasi.

Progresnya tampil di halaman. Setelah versi baru terpasang, panel dimulai ulang, dan halaman memuat ulang dengan sendirinya setelah sekitar 20 detik. Situs web Anda tetap berjalan selama panel dimulai ulang.

Anda juga bisa memperbarui dari dasbor: klik **Periksa pembaruan**, lalu **Perbarui ke v0.2.0**.

## Memperbarui dari baris perintah {#update-from-the-command-line}

```sh
nefo update
```

```
Installed: v0.1.0
Latest:    v0.2.0
Release signature verified
Downloading nefo-linux-amd64 v0.2.0
Checked: Nefo v0.2.0 (commit 5e6f7a8)
Installed. The panel restarts now; reload the page in a few seconds.
```

Jika Anda sudah memakai rilis terbaru, perintah ini mencetak kedua versi dan tidak mengubah apa pun. Untuk sekadar memeriksa, jalankan `nefo update --check`.

## Apa yang terjadi saat pembaruan {#what-happens-during-an-update}

1. Panel membaca nomor versi terbaru dari rilis GitHub milik proyek.
2. Panel mengunduh daftar checksum beserta tanda tangannya, lalu memeriksa tanda tangan itu dengan kunci proyek, yang tertanam di dalam panel.
3. Panel mengunduh program baru untuk CPU Anda dan memeriksa checksum SHA-256-nya terhadap daftar tersebut.
4. Panel menjalankan program baru sekali, untuk memastikan program itu berfungsi di server ini.
5. Panel mengganti `/usr/local/bin/nefo` dalam satu langkah, jadi tidak pernah ada program yang baru setengah tertulis.
6. Panel memulai ulang layanan `nefo`. Jika ada tugas yang sedang berjalan, seperti instalasi perangkat lunak, panel membiarkannya selesai lebih dulu, paling lama 75 detik.

Saat versi baru berjalan, versi itu memperbarui basis data panel ke format barunya. Versi baru juga menyesuaikan file konfigurasi yang dikelolanya (server web, pool PHP-FPM) dengan templatnya. Jika server web menolak file baru, panel mengembalikan file yang lama.

## Bagaimana rilis diverifikasi {#how-releases-are-verified}

Setiap rilis di GitHub berisi program untuk amd64 dan arm64, `install.sh`, file `VERSION`, `checksums.txt` dengan checksum SHA-256 kedua program dan penginstal, dan `checksums.txt.sig`, yaitu tanda tangan Ed25519 dari `checksums.txt`.

Baik penginstal maupun pembaru menolak `checksums.txt` yang tidak ditandatangani dengan kunci proyek, dan unduhan yang checksum-nya tidak cocok. Saat pembaruan, pesan kesalahannya berbunyi:

- "checksums.txt has no valid signature from the project: the update was not installed" (checksums.txt tidak punya tanda tangan sah dari proyek: pembaruan tidak diinstal);
- "the download does not match checksums.txt: not installed" (unduhan tidak cocok dengan checksums.txt: tidak diinstal).

Jadi seseorang yang bisa mengubah file rilis di GitHub, atau di sepanjang jalur menuju server Anda, tidak bisa membuat programnya sendiri terinstal lewat pembaruan: pembaruan diperiksa dengan kunci yang tertanam di Nefo yang sudah Anda jalankan. Instalasi pertama berbeda: penginstal yang Anda unduh membawa salinan kuncinya sendiri. Selain itu, kunci penandatanganan disimpan sebagai secret repositori GitHub, tempat GitHub Actions menandatangani setiap rilis, jadi seseorang yang mengambil alih repositori bisa menandatangani rilis. Lihat [Apa yang dibuktikan dan tidak dibuktikan tanda tangan](../security-policy#what-the-signature-does-and-doesn-t-prove).

Build pengembangan, yang dibuat tanpa kunci proyek, tidak bisa memeriksa tanda tangan. Build ini hanya memeriksa checksum, dan log-nya menampilkan "Development build: the release signature is not checked" (Build pengembangan: tanda tangan rilis tidak diperiksa).

## Apa yang dipertahankan {#what-is-kept}

Pembaruan hanya mengganti program, `/usr/local/bin/nefo`. Semua hal lain tetap:

- file konfigurasi `/etc/nefo/config.json`, termasuk port dan path rahasia;
- folder data `/var/lib/nefo`: akun Anda dan autentikasi dua faktor, pengaturan, situs web, jadwal, sertifikat, dan log aktivitas;
- situs web, basis data, file, dan cadangan Anda, serta layanan yang menjalankannya.

Hanya panel yang dimulai ulang. Situs web, basis data, dan layanan lain tetap berjalan.

## Jika pembaruan gagal {#if-an-update-fails}

**Pembaruan berhenti sebelum program baru diinstal.** Ini terjadi jika unduhan gagal, tanda tangan atau checksum tidak cocok, atau program baru tidak bisa berjalan di server ini. Tidak ada yang berubah: versi lama tetap berjalan. Pesan kesalahannya tampil di log tugas panel, atau di terminal. Coba lagi nanti, dan laporkan kesalahan itu jika terus terjadi.

**Versi baru tidak mau berjalan.** Periksa layanan dan log-nya:

```sh
systemctl status nefo
journalctl -u nefo -n 50 --no-pager
```

Laporkan masalahnya di [repositori GitHub](https://github.com/nefocp/nefo/issues) beserta keluaran ini. Untuk kembali ke rilis sebelumnya, instal programnya secara manual. Contoh ini memakai `v0.1.0` di server amd64:

```sh
cd /tmp
curl -fsSLO https://github.com/nefocp/nefo/releases/download/v0.1.0/nefo-linux-amd64
curl -fsSLO https://github.com/nefocp/nefo/releases/download/v0.1.0/checksums.txt
grep " nefo-linux-amd64$" checksums.txt | sha256sum -c -
install -m 0755 nefo-linux-amd64 /usr/local/bin/nefo
systemctl restart nefo
```

Lanjutkan hanya jika `sha256sum` mencetak `OK`. Di server ARM, gunakan `nefo-linux-arm64`.

::: warning Kembali ke versi yang lebih lama
Jika versi yang lebih baru sudah mengubah basis data panel, versi yang lebih lama menolak berjalan dengan pesan "database schema version … is newer than this panel supports" (versi skema basis data … lebih baru daripada yang didukung panel ini). Dalam hal itu, hanya versi yang lebih baru yang bisa bekerja dengan data tersebut. Sebelum pembaruan penting, Anda bisa menyimpan salinan folder data: `systemctl stop nefo`, lalu `cp -a /var/lib/nefo /root/nefo-data-copy`, lalu `systemctl start nefo`.
:::

## Pembaruan otomatis {#automatic-updates}

Nefo tidak memperbarui dirinya sendiri, dan tidak memeriksa pembaruan atas inisiatifnya sendiri. Nefo hanya memeriksa saat Anda mengklik **Periksa pembaruan** atau menjalankan `nefo update`. Untuk mengetahui rilis baru, pantau rilis di [repositori GitHub](https://github.com/nefocp/nefo/releases). [Catatan perubahan](../changelog) mencantumkan apa yang berubah di setiap rilis.

Ini hanya berlaku untuk panel itu sendiri. Paket sistem operasi adalah urusan terpisah: panel memeriksanya setiap beberapa jam, menampilkannya di **Sistem** → **Pembaruan**, dan bisa menginstal pembaruan keamanan secara otomatis. Lihat [Sistem](./os#updates).

## Sumber pembaruan {#where-updates-come-from}

Secara bawaan, pembaruan berasal dari `https://github.com/nefocp/nefo/releases/latest/download`. Kolom `update_url` di `/etc/nefo/config.json` bisa diarahkan ke alamat lain yang memublikasikan file yang sama. Alamat itu harus diawali `https://`; jika tidak, pemeriksaan pembaruan gagal dengan pesan "update_url di config.json harus berupa alamat https://." Biarkan kosong kecuali Anda tahu bahwa Anda membutuhkannya.
