---
title: Instalasi
description: Instal Nefo di server Ubuntu atau Debian dengan satu perintah, lalu buka panel untuk pertama kalinya.
---

# Instalasi

Untuk menginstal Nefo, jalankan satu perintah sebagai root di server Ubuntu 22.04, 24.04, atau 26.04 atau Debian 12 atau 13 yang baru diinstal: `curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | sudo bash`. Penginstal memeriksa tanda tangan Ed25519 rilis, menginstal panel sebagai layanan systemd `nefo` dalam beberapa detik, lalu mencetak alamat panel dan kata sandi admin sekali pakai.

Halaman ini menunjukkan cara menginstal Nefo, apa yang dilakukan penginstal, dan cara membuka panel untuk pertama kalinya. Periksa [persyaratan](./requirements) terlebih dahulu.

Nefo bisa diinstal di Ubuntu 22.04, 24.04, dan 26.04 serta Debian 12 dan 13, di x86-64 atau ARM64. Penginstal sudah diuji di server baru dengan setiap sistem ini, di x86-64. Versi ARM64 belum diuji di perangkat keras ARM sungguhan.

## Jalankan penginstal {#run-the-installer}

1. Hubungkan ke server Anda lewat SSH.
2. Jalankan:

   ```sh
   curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | sudo bash
   ```

   Jika Anda masuk sebagai root dan `sudo` tidak terinstal (umum di Debian), jalankan tanpanya:

   ```sh
   curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | bash
   ```

3. Tunggu sampai penginstal mencetak alamat panel, nama pengguna, dan kata sandi. Biasanya ini hanya butuh beberapa detik.

::: tip Perintah dalam dokumentasi ini
Semua perintah lain dalam dokumentasi ini dijalankan sebagai root di server, jadi ditampilkan tanpa `sudo`.
:::

## Apa yang dicetak penginstal {#what-the-installer-prints}

Akhir keluarannya terlihat seperti ini:

```
==> Downloading nefo-linux-amd64 (latest)
==> Verifying the release signature
==> Verifying checksum
==> Creating the configuration and the admin account
==> Installing the systemd service
==> Opening port 28443/tcp in ufw

Nefo is ready.

  URL:      https://203.0.113.10:28443/k3x9p2qa7m1z/
  Username: admin
  Password: ••••••••••••••••••••

Save this password now: it is stored only as a hash and cannot be shown again.
After signing in, change it and turn on two-factor authentication.
The certificate is self-signed, so your browser will warn you the first time.

If your provider has a cloud firewall (security group), allow TCP port 28443 there too.
Manage the service with: systemctl status|restart nefo   Logs: journalctl -u nefo
```

Port, path, dan kata sandi Anda berbeda: semuanya dipilih secara acak untuk setiap server.

::: warning Simpan kata sandinya sekarang
Kata sandi hanya ditampilkan sekali. Panel hanya menyimpan hash-nya. Jika Anda kehilangannya, atur kata sandi baru dengan `nefo reset-password` (lihat [Baris perintah](./cli#nefo-reset-password)).
:::

Alamat tersebut memakai alamat IP pertama server. Jika penyedia Anda memberi server alamat privat dan alamat publik yang terpisah, ganti alamat di URL dengan IP publiknya.

## Apa yang dilakukan penginstal {#what-the-installer-does}

1. **Memeriksa server.** Penginstal berhenti jika tidak dijalankan sebagai root, jika systemd, `curl`, atau `sha256sum` tidak ada, jika sistem operasi atau CPU tidak didukung, atau jika Nefo sudah terinstal (`/etc/nefo/config.json` ada).
2. **Mengunduh** program untuk CPU Anda (`nefo-linux-amd64` atau `nefo-linux-arm64`) dan daftar checksum dari rilis GitHub milik proyek.
3. **Memeriksa tanda tangan** daftar checksum dengan kunci Ed25519 milik proyek, yang tertanam di dalam penginstal. Rilis tanpa tanda tangan yang sah ditolak: "Do not install this file." (Jangan instal file ini.) Penginstal dari build pengembangan tidak punya kunci, dan menyatakan bahwa tanda tangan tidak diperiksa.
4. **Memeriksa checksum SHA-256** unduhan. Jika tidak cocok, instalasi dihentikan.
5. **Menginstal** program sebagai `/usr/local/bin/nefo`, serta membuat `/etc/nefo` dan `/var/lib/nefo`, yang hanya bisa dibaca oleh root.
6. **Membuat konfigurasi** di `/etc/nefo/config.json`: port acak dari 20000 sampai 49999, path rahasia acak sepanjang 12 karakter, dan HTTPS yang aktif. Penginstal juga membuat akun `admin` dengan kata sandi acak sepanjang 20 karakter.
7. **Menginstal layanan systemd** `nefo`, menjalankannya, dan mengaktifkannya saat boot.
8. **Membuka port panel** di ufw jika ufw aktif, atau di firewalld jika firewalld berjalan. Jika keduanya tidak aktif, penginstal tidak mengubah firewall apa pun.

Kecuali Anda memberinya [opsi penyiapan](#set-the-server-up-in-the-same-run), penginstal tidak menginstal apa pun selain itu: tidak ada server web, PHP, atau basis data. Panel menawarkan untuk menyiapkannya saat Anda pertama kali masuk (lihat [Langkah pertama](./first-steps#set-up-the-server)).

## Buka panel untuk pertama kalinya {#open-the-panel-for-the-first-time}

1. Buka URL dari penginstal di browser Anda, persis seperti yang dicetak. URL itu diawali `https://` dan diakhiri path rahasia dan garis miring.
2. Browser Anda memperingatkan bahwa koneksi tidak privat atau sertifikat tidak tepercaya. Ini wajar: panel membuat sertifikat sendiri yang ditandatangani sendiri saat pertama kali dijalankan. Pilih opsi untuk melanjutkan ke situs. Di sebagian besar browser, opsi ini ada di balik tombol seperti "Advanced" (Lanjutan).
3. Masuk dengan nama pengguna `admin` dan kata sandi dari penginstal.

Lalu ikuti [langkah pertama](./first-steps): ganti kata sandi, aktifkan autentikasi dua faktor, dan beri panel domainnya sendiri serta sertifikat gratis, agar peringatan browser hilang.

### Path rahasia {#the-secret-path}

Panel hanya merespons di bawah path rahasianya, seperti `/k3x9p2qa7m1z/`. Setiap alamat lain di port-nya mengembalikan `404`, termasuk `https://203.0.113.10:28443/` saja tanpa path. Pemindai yang mencoba alamat panel yang umum tidak menemukan apa-apa.

Rahasiakan path ini, seperti kata sandi. Jika Anda lupa, jalankan `nefo info` di server (lihat [Baris perintah](./cli#nefo-info)).

Jika Anda mengetik `http://` alih-alih `https://`, panel menjawab dengan pesan kesalahan, bukan halaman masuk.

## Buka port di firewall cloud Anda {#open-the-port-in-your-cloud-firewall}

Jika penyedia Anda punya firewall di luar server (security group atau firewall cloud), izinkan port TCP panel di sana. Penginstal tidak bisa melakukannya untuk Anda. Nanti, saat Anda menjalankan situs web, izinkan juga port TCP 80 dan 443. Lihat [Persyaratan](./requirements#firewalls).

## Variabel lingkungan {#environment-variables}

Anda bisa mengubah apa yang dilakukan penginstal dengan variabel berikut:

| Variabel | Bawaan | Fungsinya |
|---|---|---|
| `NEFO_VERSION` | `latest` | Menginstal rilis tertentu, seperti `v0.1.0`, alih-alih rilis terbaru. |
| `NEFO_REPO` | `nefocp/nefo` | Mengunduh dari repositori GitHub lain (`owner/name`), seperti fork. |
| `NEFO_FORCE` | tidak diatur | Atur ke `1` untuk mencoba sistem operasi yang tidak didukung. Pemeriksaan CPU tetap berlaku. |
| `NEFO_SETUP` | tidak diatur | Opsi untuk `nefo setup`, untuk menyiapkan server dalam proses yang sama. Lihat [di bawah](#set-the-server-up-in-the-same-run). |

Letakkan variabel sebelum `bash`:

```sh
curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | NEFO_VERSION=v0.1.0 bash
```

Dengan `sudo`, tuliskan sebagai `| sudo NEFO_VERSION=v0.1.0 bash`.

`NEFO_REPO` hanya mengubah sumber unduhan penginstal. Pembaruan berikutnya berasal dari repositori yang tertanam di program, atau dari `update_url` di file konfigurasi (lihat [Pembaruan](./update#where-updates-come-from)).

## Siapkan server dalam proses yang sama {#set-the-server-up-in-the-same-run}

Penginstal juga bisa menyiapkan server, seperti halaman penyiapan di panel: server web, versi PHP, server basis data, dan tambahan. Berikan opsi [`nefo setup`](./cli#nefo-setup) setelah `bash -s --`:

```sh
curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | sudo bash -s -- --web nginx --php 8.4 --db mariadb --redis
```

Atau letakkan di variabel `NEFO_SETUP`:

```sh
curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | sudo NEFO_SETUP="--web nginx --php 8.4 --db mariadb" bash
```

Yang berubah jika ada opsi penyiapan:

- Penginstal memeriksa opsi tepat setelah unduhan, sebelum mengonfigurasi apa pun. Opsi yang salah, seperti `--web caddy`, menghentikannya, dan Anda bisa menjalankannya lagi. Opsi hanya boleh berisi huruf, angka, dan `. , = _ -`.
- Setelah panel berjalan, penginstal menjalankan `nefo setup` dengan opsi Anda dan `--yes`, lalu menampilkan log-nya. Ini butuh beberapa menit. Panel melakukan instalasi sebagai salah satu tugasnya, jadi instalasi tetap berlanjut meskipun sesi SSH Anda berakhir: dalam hal itu, ikuti prosesnya di panel.
- Alamat, nama pengguna, dan kata sandi dicetak di akhir, setelah penyiapan.
- Panel tidak lagi membuka halaman penyiapannya setelah Anda masuk.

Jika penyiapan berhenti, misalnya karena unduhan gagal, panel tetap terinstal. Penginstal tetap mencetak alamat dan kata sandi, lalu menyatakan bahwa penyiapan berhenti, dan keluar dengan kesalahan. Perbaiki masalahnya, lalu jalankan `nefo setup` dengan opsi yang sama: bagian yang sudah terinstal akan dilewati.

## Instal ulang {#install-again}

Penginstal menolak berjalan jika `/etc/nefo/config.json` ada, jadi penginstal tidak bisa menimpa panel yang sedang berjalan. Untuk memulai dari awal, hapus Nefo terlebih dahulu (lihat [Mencopot Nefo](./uninstall)), lalu jalankan penginstal lagi.
