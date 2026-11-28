---
title: Docker
description: Jalankan kontainer dan proyek Compose, buka terminal di dalam kontainer, kelola image, dan tempatkan kontainer di situs web di balik reverse proxy.
---

# Docker

Nefo menginstal Docker dari distribusi Anda dan mengelola kontainer, proyek Compose, dan image di halaman **Docker**, dengan terminal di setiap kontainer. Nefo memublikasikan port kontainer hanya di 127.0.0.1, jadi Anda menjangkaunya lewat situs web, dan menolak proyek Compose yang akan melemahkan server, seperti kontainer privileged atau port yang terbuka ke internet.

Halaman ini membahas setiap tab, pemeriksaan keamanan yang dijalankan panel pada proyek Compose, bagaimana aplikasi Docker dari Toko aplikasi masuk ke dalamnya, dan cara membuat kontainer bisa diakses publik melalui situs web.

## Menginstal Docker {#install-docker}

Buka **Perangkat lunak** → **Docker** dan pilih **Instal**. Panel menginstal `docker.io` dari distribusi Anda, beserta plugin Compose-nya jika tersedia. Item menu **Docker** muncul setelah Docker terinstal.

Jika plugin Compose hilang di kemudian hari, panel menginstalnya saat pertama kali Anda menjalankan proyek Compose.

Untuk mencopot Docker, lihat [Perangkat lunak](./software#when-an-uninstall-is-refused). Pencopotan ditolak selama masih ada kontainer, proyek Compose, atau aplikasi Docker.

## Mengapa port tetap di 127.0.0.1 {#why-ports-stay-on-127-0-0-1}

Port yang dipublikasikan Docker melewati firewall ufw: kontainer yang dipublikasikan di semua alamat terbuka ke internet, apa pun aturan ufw-nya. Jadi panel:

- memublikasikan kontainer yang dijalankannya hanya di `127.0.0.1`;
- menolak proyek Compose yang memublikasikan port di alamat lain.

Untuk membuat kontainer bisa diakses publik, letakkan situs web di depannya. Lihat [Menempatkan kontainer di situs web](#put-a-container-on-a-website).

## Kontainer {#containers}

Tab **Kontainer** menampilkan setiap kontainer di server, berjalan atau tidak, beserta image, status, dan port-nya. Kontainer dari proyek Compose dan dari aplikasi Toko aplikasi juga muncul di sini.

Baris setiap kontainer punya tombol **Jalankan** atau **Hentikan**, dan tindakan lain ada di menunya:

| Tindakan | Fungsinya |
|---|---|
| **Jalankan** / **Hentikan** | Menjalankan atau menghentikan kontainer |
| **Terminal** | Membuka shell di dalam kontainer. Hanya untuk kontainer yang sedang berjalan |
| **Mulai ulang** | Memulai ulang kontainer |
| **Log** | Menampilkan 200 baris terakhir keluarannya, dengan stempel waktu |
| **Hapus** | Menghapus kontainer, bahkan saat sedang berjalan |

::: warning
**Hapus** menghilangkan data kontainer kecuali data itu disimpan di volume.
:::

### Menjalankan kontainer {#run-a-container}

1. Di tab **Kontainer**, pilih **Jalankan kontainer**.
2. Isi:
   - **Image**, seperti `nginx:alpine`;
   - **Nama kontainer**, seperti `my-app`;
   - **Port di server ini** (1024 sampai 65535) dan **Port di kontainer**, jika kontainer menyajikan sesuatu;
   - **Variabel lingkungan (satu NAME=value per baris)**, seperti `TZ=UTC`.
3. Pilih **Jalankan**.

Panel mengunduh image jika diperlukan dan menjalankan kontainer dengan kebijakan restart `unless-stopped`, jadi kontainer kembali berjalan setelah reboot. Port hanya dipublikasikan di `127.0.0.1`.

### Terminal di dalam kontainer {#terminal-in-a-container}

Pilih **Terminal** untuk membuka shell di dalam kontainer yang sedang berjalan. Panel memakai `bash` jika image memilikinya, dan `sh` jika tidak. Sesi berakhir saat Anda menutup jendela atau setelah 30 menit tanpa masukan.

## Proyek Compose {#compose-projects}

Proyek Compose adalah `compose.yaml` yang menjalankan beberapa kontainer bersama-sama. Panel menyimpan setiap proyek di `/var/lib/panel-compose/<name>/compose.yaml`, yang hanya bisa dibaca root, dan menjalankannya sebagai proyek Compose `panel-<name>`.

### Membuat proyek {#create-a-project}

1. Buka **Docker** → **Compose** dan pilih **Proyek baru**.
2. Masukkan **Nama proyek**: huruf kecil, angka, dan tanda hubung, paling banyak 40 karakter. Kontainer diberi nama `panel-<name>-<service>-1`.
3. Tulis file di kolom `compose.yaml`. **Sisipkan contoh** mengisi contoh kecil yang berfungsi.
4. Pilih **Buat dan jalankan**.

Panel memeriksa file, menyimpannya, dan menjalankan kontainer, sambil menarik image yang dibutuhkan. Path relatif di file, seperti `./data:/data`, berarti folder proyek.

Proyek minimal:

```yaml
services:
  web:
    image: traefik/whoami
    restart: unless-stopped
    ports:
      - "127.0.0.1:8081:80"
```

`compose.yaml` bisa berukuran hingga 64 KB.

### Mengedit proyek {#edit-a-project}

Buka menu sebuah proyek dan pilih **Edit**. Ubah file dan pilih **Simpan dan terapkan**. Panel memeriksa file baru, menyimpannya, dan membuat ulang kontainer yang konfigurasinya berubah. Kontainer dari layanan yang Anda hapus dari file juga dihapus.

### Tindakan proyek {#project-actions}

| Tindakan | Fungsinya |
|---|---|
| **Jalankan** | Memeriksa file dan menjalankan kontainer |
| **Hentikan** | Menghentikan kontainer |
| **Mulai ulang** | Memulai ulang kontainer |
| **Perbarui image** | Memeriksa file, menarik image yang lebih baru, dan membuat ulang kontainer |
| **Hapus kontainer** | Menghentikan dan menghapus kontainer. Volume dan `compose.yaml` dipertahankan |
| **Log** | 200 baris terakhir dari setiap kontainer |
| **Hapus** | Menghapus kontainer, jaringan, dan folder proyek beserta `compose.yaml`. Centang **Hapus juga volume-nya (datanya akan hilang)** untuk ikut menghapus volume. Image dipertahankan |

### Pemeriksaan keamanan {#safety-checks}

Sebelum menyimpan proyek, dan sebelum setiap kali proyek dijalankan dan diperbarui, panel meminta konfigurasi akhir dari Compose, dengan include, extends, dan variabel yang sudah di-resolve. Panel menolak proyek yang akan melemahkan server:

| Ditolak | Alasan |
|---|---|
| `privileged: true` | Kontainer privileged punya kendali penuh atas server |
| `network_mode`, `pid`, `ipc`, `userns_mode`, atau `cgroup` diatur ke `host` | Kontainer akan berbagi namespace milik server itu sendiri |
| `cap_add` apa pun | Hak istimewa kontainer tambahan |
| `devices` atau `device_cgroup_rules` apa pun | Perangkat host diberikan ke kontainer |
| `security_opt` selain `no-new-privileges` | Opsi seperti `seccomp=unconfined` atau `apparmor=unconfined` melemahkan isolasi kontainer |
| `sysctls` apa pun | Pengaturan kernel |
| Port tanpa `127.0.0.1` atau `::1` sebagai alamat host-nya, seperti `"8080:80"`, atau port tanpa nomor yang dipublikasikan | Docker akan mendengarkan di semua alamat, melewati firewall |
| Bind mount `/etc`, `/root`, `/boot`, `/proc`, `/sys`, `/dev`, atau apa pun di dalamnya | Folder sistem memberi kontainer kendali atas server |
| Bind mount socket Docker, `/var/run/docker.sock` atau `/run/docker.sock`, atau folder yang memuatnya, seperti `/`, `/var`, `/var/run`, atau `/run` | Socket memberi kontainer kendali atas Docker, dan dengan demikian atas server |
| Named volume yang opsi `device`-nya berupa path seperti itu, serta secret atau config yang dibaca dari path seperti itu | Folder yang sama, di-mount dengan cara lain |

Path diperiksa sebagaimana ditulis dan setelah mengikuti tautan simbolik. `/etc/localtime` dan `/etc/timezone` boleh di-mount hanya-baca (`:ro`), agar kontainer bisa memakai zona waktu server.

Jika pemeriksaan gagal, editor menunjukkan layanan dan pengaturan mana yang ditolak. Tidak ada yang disimpan atau dijalankan.

## Image {#images}

Tab **Image** menampilkan image lokal beserta ukuran, tanggal pembuatan, dan kontainer yang memakainya (**Dipakai oleh**).

- **Hapus** menghapus image. Jika ada kontainer yang memakainya, panel menyebutkan kontainer-kontainer itu dan bertanya lagi sebelum tetap menghapus image tersebut.
- **Hapus yang tidak terpakai** menghapus setiap image yang tidak dipakai kontainer mana pun. Image itu diunduh lagi saat dibutuhkan.

Tidak ada tombol pull terpisah. Image diunduh saat Anda menjalankan kontainer, menjalankan proyek Compose, atau memilih **Perbarui image**.

## Aplikasi Docker dari Toko aplikasi {#docker-apps-from-the-app-store}

[Toko aplikasi](./apps) menginstal aplikasi Docker seperti Ghost, Uptime Kuma, n8n, dan Gitea. Aplikasi ini berjalan sebagai proyek Compose yang ditulis panel dari resep setiap aplikasi:

- File dan datanya ada di `/var/lib/panel-apps/<name>`, dan proyek Compose-nya bernama `panelapp-<name>`. Aplikasi ini tidak muncul di tab **Compose**, tetapi kontainernya muncul di tab **Kontainer**.
- Aplikasi ini melewati pemeriksaan keamanan yang sama seperti proyek Anda sendiri.
- Setiap aplikasi Docker mendapat situs webnya sendiri. Aplikasi layanan seperti PostgreSQL, MongoDB, dan Ollama tidak punya situs web dan hanya mendengarkan di `127.0.0.1`.

Kelola aplikasi ini dari **Toko aplikasi** → **Terinstal**, tempat Anda menjalankan, menghentikan, memperbarui, dan mencadangkannya. Lihat [Toko aplikasi](./apps) untuk menghapusnya, dan [Pencadangan](./backups#app-backups) untuk cadangannya.

## Menempatkan kontainer di situs web {#put-a-container-on-a-website}

Situs web bisa meneruskan pengunjungnya ke port kontainer di `127.0.0.1`. Server web lalu bertindak sebagai reverse proxy, dan Anda bisa memberi situs sertifikat SSL.

1. Publikasikan port kontainer di `127.0.0.1`, misalnya `127.0.0.1:8081:80` di `compose.yaml`, atau **Port di server ini** `8081` di **Jalankan kontainer**.
2. Buka **Situs web** dan pilih **Tambah situs web**.
3. Masukkan **Domain**, seperti `app.example.com`.
4. Di **Jenis**, pilih **Aplikasi (Node.js, Python, Java…) di balik proxy**.
5. Di **Port yang didengarkan aplikasi (127.0.0.1)**, masukkan port yang sama, `8081`.
6. Biarkan **Perintah mulai (opsional)** kosong: Docker yang menjalankan aplikasinya.
7. Pilih **Buat situs web**.

Port harus antara 1024 dan 65535, dan bukan port panel, 3306 (MariaDB), 6379 (Redis), atau 7809 (pendengar webhook Git). Dua situs web aplikasi tidak bisa memakai port yang sama.

Lalu dapatkan sertifikat untuk situs itu, lihat [Sertifikat SSL](./ssl). Lihat [Situs web](./websites) untuk pengaturan lain situs tersebut.

Jenis **Reverse proxy ke alamat lain** juga bisa dipakai untuk kontainer: jenis ini meneruskan ke alamat mana pun, seperti `http://127.0.0.1:8081`, dengan opsi untuk header Host, WebSocket, dan batas waktu. Lihat [Situs web reverse proxy](./websites#reverse-proxy-websites).
