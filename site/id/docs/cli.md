---
title: Baris perintah
description: Setiap perintah nefo beserta opsinya (info, reset-password, update, setup, unlock-files, decrypt-backup), file konfigurasi, dan cara mengelola layanan nefo dengan systemctl dan journalctl.
---

# Baris perintah

Program `nefo` di `/usr/local/bin/nefo` menjalankan panel dan punya beberapa perintah untuk pengguna root server: `nefo info` menampilkan alamat panel, `nefo reset-password` mengatur kata sandi baru (dan bisa menonaktifkan 2FA), `nefo update` menginstal rilis terbaru, dan `nefo setup` menginstal perangkat lunak. Jalankan setiap perintah sebagai root.

Halaman ini mencantumkan perintah-perintah itu, lalu menunjukkan cara mengelola layanan `nefo` dan isi file konfigurasinya.

## Perintah {#commands}

| Perintah | Fungsinya |
|---|---|
| `nefo info` | Menampilkan URL panel |
| `nefo reset-password` | Mengatur kata sandi acak baru dan mengakhiri semua sesi |
| `nefo update` | Menginstal rilis terbaru dan memulai ulang panel |
| `nefo setup` | Menginstal server web, versi PHP, server basis data, dan tambahan |
| `nefo unlock-files` | Mengembalikan file situs web yang terkunci (sebelum Anda menghapus panel) |
| `nefo decrypt-backup` | Mendekripsi salinan cadangan yang dienkripsi penyimpanan jarak jauh |
| `nefo version` | Menampilkan versi |
| `nefo init` | Membuat file konfigurasi dan akun admin pertama |
| `nefo serve` | Menjalankan panel (systemd melakukannya untuk Anda) |
| `nefo help` | Menampilkan daftar perintah |

Jalankan `nefo <command> -h` untuk melihat opsi sebuah perintah. Opsi bisa ditulis dengan satu atau dua tanda hubung: `-port` dan `--port` sama saja.

Setiap perintah kecuali `version`, `help`, dan `decrypt-backup` membaca file konfigurasi `/etc/nefo/config.json`. Untuk memakai file lain, tambahkan `--config /path/to/config.json`, atau atur variabel lingkungan `NEFO_CONFIG`.

## nefo info {#nefo-info}

Menampilkan URL panel dan lokasi file-filenya.

```sh
nefo info
```

```
URL:    https://panel.example.com:28443/k3x9p2qa7m1z/
URL:    https://203.0.113.10:28443/k3x9p2qa7m1z/
Config: /etc/nefo/config.json
Data:   /var/lib/nefo
```

Baris pertama muncul setelah panel punya domainnya sendiri (**Pengaturan** → **Sertifikat**). Baris kedua memakai alamat IP pertama server. Jika penyedia Anda memberi server alamat privat dan alamat publik yang terpisah, berikan alamat publiknya:

```sh
nefo info --host 203.0.113.10
```

| Opsi | Fungsinya |
|---|---|
| `--host` | Nama host atau alamat IP yang ditampilkan di URL |
| `--port` | Hanya mencetak nomor port, untuk skrip |
| `--config` | File konfigurasi yang dibaca |

## nefo reset-password {#nefo-reset-password}

Mengatur kata sandi acak baru untuk akun, mengeluarkan akun itu dari semua perangkat, dan mencetak kata sandinya. Gunakan perintah ini jika Anda lupa kata sandi.

```sh
nefo reset-password
```

```
New password for admin: ••••••••••••••••••••
```

Jika ponsel dengan aplikasi autentikator Anda juga hilang, tambahkan `--disable-2fa`. Opsi ini menonaktifkan autentikasi dua faktor untuk akun tersebut:

```sh
nefo reset-password --disable-2fa
```

```
New password for admin: ••••••••••••••••••••
Two-factor authentication is off. Turn it on again after signing in.
```

| Opsi | Fungsinya |
|---|---|
| `--disable-2fa` | Juga menonaktifkan autentikasi dua faktor |
| `--user` | Nama akun (bawaan `admin`, satu-satunya akun panel) |
| `--config` | File konfigurasi yang dibaca |

Panel tetap berjalan: Anda tidak perlu memulai ulang panel. Reset ini dicatat di log aktivitas.

## nefo update {#nefo-update}

Memeriksa rilis GitHub milik proyek. Jika ada versi yang lebih baru, perintah ini mengunduhnya, memeriksa tanda tangan dan checksum-nya, mengganti program, dan memulai ulang panel.

```sh
nefo update
```

Untuk sekadar melihat apakah ada pembaruan:

```sh
nefo update --check
```

```
Installed: v0.1.0
Latest:    v0.2.0
```

| Opsi | Fungsinya |
|---|---|
| `--check` | Hanya menampilkan versi yang terinstal dan versi terbaru |
| `--config` | File konfigurasi yang dibaca |

Lihat [Pembaruan](./update) untuk mengetahui apa yang diperiksa saat pembaruan dan apa yang harus dilakukan jika pembaruan gagal.

## nefo setup {#nefo-setup}

Menyiapkan server seperti halaman penyiapan di panel (lihat [Langkah pertama](./first-steps#set-up-the-server)): perintah ini menginstal server web, versi PHP, server basis data, dan tambahan, satu per satu. Bagian yang sudah terinstal dilewati, jadi Anda bisa menjalankannya lagi setelah kegagalan. Setelah selesai, panel tidak lagi membuka halaman penyiapannya setelah masuk.

```sh
nefo setup --web nginx --php 8.3,8.4 --db mariadb --redis
```

```
This server: 1 CPU, 960 MB of RAM (250 MB in use), 19.5 GB of free disk.

What the setup installs, one after the other:
  1. Nginx and PHP 8.3  about 40 MB of RAM, 55 MB of disk
  2. PHP 8.4            about 25 MB of RAM, 50 MB of disk, from packages.sury.org
  3. MariaDB            about 228 MB of RAM, 300 MB of disk
  4. Redis              up to 64 MB of RAM, 10 MB of disk
In all: about 357 MB more RAM and 415 MB of disk.

Install? [y/N]
```

Setelah Anda menjawab `y`, perintah ini menampilkan log setiap instalasi. Jika salah satu gagal, perintah berhenti di sana, memberi tahu apa yang sudah terinstal, dan keluar dengan kesalahan. Tanpa `--web`, `--php`, `--db`, atau tambahan apa pun, perintah ini menawarkan pilihan yang disarankan untuk RAM server, seperti halaman penyiapan.

Selama panel berjalan, `nefo setup` meminta panel melakukan instalasi sebagai salah satu tugasnya: instalasi menunggu tugas lain selesai, tampil di panel, dan tetap berlanjut meskipun Anda menutup sesi SSH. Perintah ini berkomunikasi dengan panel lewat socket di folder data yang hanya bisa dibuka root. Jika panel tidak berjalan, `nefo setup` melakukan instalasi sendiri.

| Opsi | Fungsinya |
|---|---|
| `--web` | `nginx`, `apache`, `openlitespeed`, atau `none`. Server web disertai versi PHP dari distribusi Anda |
| `--php` | Versi PHP yang dipisahkan koma, seperti `8.3,8.4`, atau `none` |
| `--db` | `mariadb`, `mysql`, atau `none`; `mariadb-11.4`, `mariadb-11.8`, atau `mariadb-12.3` untuk seri LTS dari repositori MariaDB, jika server menawarkannya (lihat [Versi MariaDB](./databases#mariadb-versions)). `--mariadb` sama dengan `--db mariadb` |
| `--redis`, `--node`, `--python`, `--docker` | Juga menginstal Redis, Node.js (rilis LTS terbaru), Python untuk situs web aplikasi, atau Docker |
| `--adminer`, `--waf` | Juga menginstal Adminer atau firewall web. Keduanya membutuhkan server web |
| `--clamav` | Juga menginstal ClamAV, di server dengan RAM 2 GB atau lebih |
| `--postgresql`, `--memcached` | Juga menginstal PostgreSQL atau Memcached, jika halaman Perangkat lunak menawarkannya |
| `--yes`, `-y` | Menginstal tanpa bertanya. Diperlukan jika tidak ada terminal yang terhubung, seperti dalam skrip |
| `--dry-run` | Hanya menampilkan apa yang akan diinstal |
| `--skip` | Tidak menginstal apa pun: panel tidak lagi membuka halaman penyiapannya setelah masuk |
| `--wait` | Berapa lama menunggu panel berjalan, seperti `60s`. Dipakai oleh penginstal |
| `--check` | Hanya memeriksa opsi. Dipakai penginstal sebelum mengonfigurasi apa pun |
| `--config` | File konfigurasi yang dibaca |

Penginstal menerima opsi yang sama, untuk menyiapkan server dalam proses yang sama (lihat [Instalasi](./install#set-the-server-up-in-the-same-run)).

## nefo unlock-files {#nefo-unlock-files}

Mengembalikan file setiap situs web yang file-nya dikunci (**Situs web** → (situs) → **Penguncian file**, lihat [Pemblokiran negara dan penguncian file](./website-protection#lock-a-website-s-files)), persis dengan pemilik, grup, dan izin yang dimiliki setiap file sebelum dikunci, lalu menghapus grup kunci situs web tersebut. Jalankan sebagai root, dengan panel dihentikan, sebelum Anda menghapus Nefo (lihat [Mencopot Nefo](./uninstall#unlock-the-websites-files)).

```sh
systemctl stop nefo
nefo unlock-files
```

```
Unlocked the files of example.com
Unlocked the files of blog.example.com
```

Jika tidak ada situs web yang terkunci, perintah ini mencetak `No website's files are locked.`

| Opsi | Fungsinya |
|---|---|
| `--config` | File konfigurasi yang dibaca |

## nefo decrypt-backup {#nefo-decrypt-backup}

Mendekripsi salinan cadangan yang dienkripsi oleh penyimpanan jarak jauh (file `.tar.gz.enc`), misalnya salinan yang Anda unduh dari Google Drive. Hanya passphrase yang dibutuhkan: perintah ini bisa dipakai di server mana pun, juga di server tempat panel belum disiapkan, dan tidak membaca file konfigurasi. Lihat [Memulihkan di server baru](./restore-new-server#decrypt-a-copy-by-hand).

```sh
nefo decrypt-backup example.com_20261126-030000.tar.gz.enc
```

```
Passphrase:
Decrypted to example.com_20261126-030000.tar.gz
SHA-256: 3f1c…
```

Passphrase tidak pernah diberikan di baris perintah, tempat pengguna lain bisa melihatnya. Di terminal, passphrase diminta tanpa ditampilkan; jika tidak, passphrase diambil dari baris pertama masukan standar atau dari `-passphrase-file`.

| Opsi | Fungsinya |
|---|---|
| `-o` | File keluaran. Bawaannya nama file masukan tanpa `.enc`. `-o -` menulis ke keluaran standar, misalnya `nefo decrypt-backup -o - FILE.enc \| tar xz` |
| `-passphrase-file` | Membaca passphrase dari baris pertama file ini |
| `-force` | Mengganti file keluaran jika sudah ada |

Keluaran ditulis ke file sementara dan baru diganti namanya setelah seluruh salinan berhasil didekripsi, jadi passphrase yang salah atau file yang rusak tidak meninggalkan apa pun (kecuali dengan `-o -`, tempat akhir file diperiksa paling akhir). File yang tidak terenkripsi disalin apa adanya, disertai catatan. SHA-256 yang dicetak adalah nilai yang dicatat panel untuk cadangan tersebut.

## nefo version {#nefo-version}

Menampilkan versi dan commit yang menjadi dasar build-nya. `nefo --version` dan `nefo -v` melakukan hal yang sama.

```sh
nefo version
```

```
Nefo v0.1.0 (commit 1a2b3c4)
```

## nefo init {#nefo-init}

Membuat file konfigurasi dan akun admin pertama. Penginstal menjalankannya untuk Anda, jadi biasanya Anda tidak membutuhkannya. Perintah ini menolak menimpa file konfigurasi yang sudah ada.

Perintah ini memilih port acak dari 20000 sampai 49999 dan path rahasia acak, mengaktifkan HTTPS, dan membuat akun `admin` dengan kata sandi acak. Jika folder data sudah berisi akun, perintah ini mempertahankannya, hanya menulis file konfigurasi baru, dan tidak mencetak kata sandi.

| Opsi | Fungsinya |
|---|---|
| `--config` | Lokasi penulisan file konfigurasi (bawaan `/etc/nefo/config.json`) |
| `--data` | Folder data (bawaan `/var/lib/nefo`) |
| `--listen` | Alamat yang didengarkan, seperti `:28443`, alih-alih port acak |
| `--host` | Nama host atau alamat IP yang ditampilkan di URL yang dicetak |
| `--dev` | Mode pengembangan: HTTP biasa di `127.0.0.1:8080` dengan path `dev`. Jangan pernah memakainya di server. |

## nefo serve {#nefo-serve}

Menjalankan panel. Layanan `nefo` menjalankannya dengan `nefo serve --config /etc/nefo/config.json`, jadi Anda tidak menjalankannya sendiri. Salinan kedua akan gagal, karena port-nya sudah dipakai layanan.

## Mengelola layanan nefo {#manage-the-nefo-service}

| Perintah | Fungsinya |
|---|---|
| `systemctl status nefo` | Menampilkan apakah panel berjalan, beserta baris log terbarunya |
| `systemctl restart nefo` | Memulai ulang panel |
| `systemctl stop nefo` | Menghentikan panel |
| `systemctl start nefo` | Menjalankan panel |
| `journalctl -u nefo` | Menampilkan log panel |
| `journalctl -u nefo -f` | Mengikuti log saat baris baru masuk |
| `journalctl -u nefo -n 100 --no-pager` | Menampilkan 100 baris terakhir |
| `journalctl -u nefo --since "1 hour ago"` | Menampilkan log satu jam terakhir |

Memulai ulang panel tidak menghentikan situs web, basis data, atau layanan lain Anda. Anda juga bisa memulai ulang panel dari **Pengaturan** → **Panel** → **Daya** → **Mulai ulang panel**.

Saat panel berhenti, panel membiarkan tugas yang sedang berjalan, seperti instalasi perangkat lunak, selesai lebih dulu, paling lama 75 detik. Jika panel berhenti karena kesalahan, systemd menjalankannya lagi setelah 3 detik.

File layanannya adalah `/etc/systemd/system/nefo.service`:

```ini
[Unit]
Description=Nefo server control panel
Documentation=https://nefo.io
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/nefo serve --config /etc/nefo/config.json
Restart=on-failure
RestartSec=3
UMask=0077
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

Panel berjalan sebagai root, karena panel mengelola seluruh server.

## File konfigurasi {#the-configuration-file}

`/etc/nefo/config.json` hanya bisa dibaca oleh root. Penginstal menulisnya, misalnya:

```json
{
  "listen": ":28443",
  "entrance": "k3x9p2qa7m1z",
  "data_dir": "/var/lib/nefo",
  "tls": {
    "enabled": true
  }
}
```

| Kolom | Artinya |
|---|---|
| `listen` | Alamat dan port yang didengarkan panel. `:28443` berarti port 28443 di semua alamat. |
| `entrance` | Path rahasia. 8 sampai 32 huruf kecil atau angka. Setiap path lain menjawab `404`. |
| `data_dir` | Folder data: basis data panel `nefo.db`, sertifikat, dan kunci. |
| `tls.enabled` | HTTPS untuk panel. Biarkan aktif. |
| `tls.cert`, `tls.key` | Path opsional ke sertifikat dan kunci Anda sendiri untuk panel. Keduanya diprioritaskan di atas sertifikat Let's Encrypt panel dan sertifikat yang ditandatangani sendiri. |
| `acme_directory` | Otoritas sertifikat opsional untuk sertifikat. Kosong berarti Let's Encrypt. Atur `https://acme-staging-v02.api.letsencrypt.org/directory` saat menguji: sertifikat staging tidak dipercaya browser, tetapi percobaan yang gagal tidak dihitung dalam batas Let's Encrypt. |
| `update_url` | Alamat opsional tempat pembaruan diunduh. Kosong berarti rilis GitHub milik proyek. Harus diawali `https://`. |
| `dev` | Mode pengembangan. Jangan pernah mengaturnya di server. |

Setelah Anda mengedit file ini, mulai ulang panel dengan `systemctl restart nefo`. Jika file berisi kesalahan, panel tidak berjalan: `journalctl -u nefo` menampilkan alasannya.

::: warning Mengubah port
Jika Anda mengubah port di `listen`, buka port baru terlebih dahulu, baik di ufw (misalnya `ufw allow 31234/tcp`) maupun di firewall cloud penyedia Anda. Jika tidak, Anda tidak bisa menjangkau panel setelah dimulai ulang.
:::
