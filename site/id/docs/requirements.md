---
title: Persyaratan
description: Sistem operasi, jenis CPU, memori, port, dan pengaturan firewall yang dibutuhkan Nefo.
---

# Persyaratan

Nefo membutuhkan server 64-bit, x86-64 atau ARM64, yang menjalankan Ubuntu 22.04, 24.04, atau 26.04 atau Debian 12 atau 13, dengan akses root dan systemd. Nefo dibuat untuk VPS dengan RAM 1 GB: panelnya sendiri memakai sekitar 34 MB memori, diukur di server uji setelah pembaruan ke v0.24.0.

Halaman ini mencantumkan apa yang dibutuhkan server Anda sebelum Anda [menginstal Nefo](./install): sistem operasi, jenis CPU, memori dan disk, serta port jaringan.

## Sistem operasi {#operating-system}

Penginstal mendukung sistem berikut:

| Distribusi | Versi |
|---|---|
| Ubuntu | 22.04, 24.04, 26.04 |
| Debian | 12, 13 |

Penginstal sudah diuji di server baru dengan setiap sistem ini, di x86-64. Versi ARM64 belum diuji di perangkat keras ARM sungguhan.

Pada sistem lain, penginstal berhenti dan menyatakan bahwa sistem tersebut "is not supported yet" (belum didukung). Anda bisa mengatur `NEFO_FORCE=1` untuk tetap mencoba (lihat [Instalasi](./install#environment-variables)), tetapi pengelolaan situs web membutuhkan `apt-get`, `systemctl`, dan `useradd`. Tanpa itu, halaman **Situs web** menampilkan "Pengelolaan situs web memerlukan server Debian atau Ubuntu dengan systemd, jadi tidak tersedia di mesin ini."

Penginstal juga membutuhkan:

- akses root (masuk sebagai root, atau gunakan `sudo`);
- systemd;
- `curl` dan `sha256sum`;
- `openssl`, untuk memeriksa tanda tangan rilis.

Jika `curl` tidak ada, instal dulu dengan `apt install curl`.

## CPU {#cpu}

Nefo berjalan di x86 64-bit (`x86_64`, juga disebut amd64) dan ARM 64-bit (`aarch64`, juga disebut arm64). Pada CPU lain, penginstal berhenti dengan pesan "unsupported CPU architecture" (arsitektur CPU tidak didukung).

## Memori dan disk {#memory-and-disk}

Nefo dibuat untuk VPS kecil dengan RAM 1 GB. Panelnya sendiri adalah satu program berukuran sekitar 25 MB, dan memakai sekitar 34 MB memori (diukur di server uji setelah pembaruan ke v0.24.0). Anda hanya menginstal komponen yang Anda butuhkan. Halaman **Perangkat lunak** menunjukkan perkiraan memori yang dipakai setiap komponen. Misalnya, server web memakai sekitar 15 MB (Nginx), 26 MB (OpenLiteSpeed), atau 40 MB (Apache) saat menganggur.

Di VPS yang berupa kontainer (LXC, OpenVZ), Nefo menghitung batas memori milik kontainer itu sendiri, bukan memori host (lihat [VPS yang berupa kontainer](./first-steps#a-vps-that-is-a-container)).

Ada satu komponen yang butuh memori jauh lebih banyak: antivirus ClamAV yang opsional membutuhkan sekitar 1,4 GB memori bebas saat memindai, jadi cocok untuk server dengan 2 GB atau lebih.

Di server dengan memori 1 GB, tambahkan swap 1 sampai 2 GB, agar saat server sibuk kernel tidak menghentikan MariaDB atau PHP. Banyak image VPS tidak menyertakan swap. **Sistem** → **Swap** membuat file swap dengan satu klik (lihat [Swap](./os#swap)). Di dalam kontainer (OpenVZ, LXC), kernel tidak mengizinkan swap: tanyakan kepada penyedia Anda.

Ruang disk yang Anda butuhkan bergantung pada situs web, basis data, dan cadangan Anda. Cadangan yang disimpan di server berada di `/var/backups/panel`, di disk yang sama dengan situs web Anda. Dasbor menampilkan pemakaian disk, dan panel memperingatkan Anda saat disk sudah 90% penuh.

## Server baru {#a-fresh-server}

Instal Nefo di server yang baru diinstal. Penginstal tidak memeriksa apa lagi yang sudah terinstal, tetapi panel menyiapkan sendiri server web, PHP, MariaDB, firewall, dan SSH:

- Panel menjalankan satu server web (Nginx, Apache, atau OpenLiteSpeed). Panel menolak menginstal server web jika sudah ada server web lain di server, karena keduanya membutuhkan port 80 dan 443.
- Panel menulis konfigurasinya sendiri untuk perangkat lunak yang dikelolanya. Jangan instal Nefo berdampingan dengan panel kontrol lain.

Penginstal menolak dijalankan untuk kedua kalinya di server yang sama: jika `/etc/nefo/config.json` ada, penginstal berhenti dengan pesan "the panel is already installed" (panel sudah terinstal).

## Port {#ports}

| Port | Dipakai oleh | Kapan |
|---|---|---|
| Port TCP acak dari 20000 sampai 49999 | Panel | Sejak instalasi. Penginstal mencetaknya. |
| 80 dan 443 (TCP) | Server web | Setelah Anda menginstal server web dari panel |
| 443 (UDP) | HTTP/3 di OpenLiteSpeed | Jika OpenLiteSpeed adalah server webnya |
| Port SSH Anda (bawaannya 22) | SSH | Selalu |
| 3306 (TCP) | MariaDB atau MySQL | Hanya jika Anda mengizinkan akses jarak jauh ke basis data, dan hanya untuk alamat yang Anda cantumkan |
| 5432 (TCP) | PostgreSQL | Hanya jika Anda mengizinkan akses jarak jauh ke basis data PostgreSQL, dan hanya untuk alamat yang Anda cantumkan |

Beberapa layanan hanya mendengarkan di server itu sendiri (127.0.0.1), jadi Anda tidak perlu membukanya: penerima webhook push Git (port 7809, dijangkau lewat server web), port kontainer Docker, dan relay email lokal.

## Firewall {#firewalls}

### Firewall di server (ufw) {#the-firewall-on-the-server-ufw}

- Jika ufw aktif saat Anda menginstal, penginstal membuka port panel di ufw (atau di firewalld, jika firewalld yang berjalan).
- Saat Anda menginstal server web dari panel dan ufw aktif, panel membuka port 80 dan 443. Panel tidak mengelola firewalld: jika server memakainya, buka sendiri port-port ini.
- Halaman **Firewall & SSH** mengelola ufw. Jika ufw tidak terinstal, halaman ini menampilkan "ufw tidak terinstal di server ini". Instal dengan `apt install ufw`.

Saat Anda mengaktifkan firewall dari panel, panel tetap membuka SSH dan port panel, serta port 80 dan 443 jika server web sudah terinstal. Lihat [Keamanan](./security).

### Firewall cloud penyedia Anda {#your-provider-s-cloud-firewall}

Banyak penyedia punya firewall di luar server, yang sering disebut security group atau firewall cloud. Penginstal tidak bisa mengubahnya. Izinkan sendiri hal-hal berikut di sana:

1. Port SSH Anda.
2. Port TCP panel, yang dicetak penginstal di akhir.
3. Port TCP 80 dan 443, setelah Anda menjalankan situs web. Let's Encrypt memeriksa domain Anda lewat port 80, jadi sertifikat tidak bisa diterbitkan selama port itu tertutup.

## Nama domain {#a-domain-name}

Anda tidak membutuhkan domain untuk menginstal Nefo: Anda membuka panel lewat alamat IP server. Anda membutuhkan domain untuk setiap situs web, dan Anda bisa memberi panel domainnya sendiri dan sertifikat gratis (lihat [Langkah pertama](./first-steps#give-the-panel-its-own-domain)). Arahkan record A domain ke alamat IP server.
