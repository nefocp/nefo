---
title: Pemecahan masalah
description: Solusi untuk masalah paling umum dengan Nefo, dari panel yang tidak bisa dijangkau sampai kesalahan sertifikat dan situs web yang menampilkan 502 atau 503.
---

# Pemecahan masalah

Sebagian besar masalah dengan Nefo punya solusi singkat di server, sebagai root: `nefo info` menampilkan alamat panel yang terlupa, `nefo reset-password` mengatur kata sandi baru (dengan `--disable-2fa` perintah ini juga menonaktifkan masuk dua faktor), dan `journalctl -u nefo` menampilkan alasan panel tidak mau berjalan. Halaman ini juga membahas kesalahan sertifikat, situs web yang menampilkan 502 atau 503, dan pemulihan yang gagal.

Sebagian besar solusi membutuhkan akses SSH ke server sebagai root. Jika SSH juga tidak berfungsi, gunakan konsol web di panel kontrol penyedia Anda.

## Anda tidak bisa membuka panel {#you-can-t-open-the-panel}

### Anda lupa alamatnya {#you-forgot-the-address}

Jalankan ini di server:

```sh
nefo info
```

Keluarannya menampilkan URL lengkap, dengan port dan path rahasia: dengan domain panel jika Anda sudah memberinya domain, dan dengan alamat IP pertama server. Jika alamat itu alamat privat, tambahkan `--host` dan alamat IP publik Anda, seperti `nefo info --host 203.0.113.10`.

### Halaman menampilkan 404 {#the-page-shows-404}

Panel hanya merespons di bawah path rahasianya, seperti `https://203.0.113.10:28443/k3x9p2qa7m1z/`. Setiap alamat lain mengembalikan `404`, termasuk alamat tanpa path. Periksa path-nya dengan `nefo info`.

Pastikan juga alamat diawali `https://`. Dengan `http://`, panel menjawab dengan pesan kesalahan, bukan halaman masuk.

### Halaman sama sekali tidak termuat {#the-page-doesn-t-load-at-all}

Jika browser tidak bisa terhubung, atau menunggu lalu menyerah, berarti panel tidak berjalan atau ada yang memblokir koneksi.

1. **Periksa bahwa panel berjalan:**

   ```sh
   systemctl status nefo
   ```

   Jika statusnya bukan `active (running)`, lihat [Panel tidak mau berjalan](#the-panel-doesn-t-start).

2. **Periksa firewall cloud penyedia Anda.** Firewall itu harus mengizinkan port TCP panel. Temukan port-nya dengan `nefo info --port`.

3. **Periksa bahwa alamat IP Anda masih diizinkan.** Jika Anda membatasi port panel hanya untuk alamat IP Anda sendiri, di ufw atau di firewall cloud, dan alamat IP Anda sudah berubah (umum pada koneksi rumah dan seluler), izinkan alamat yang baru. Di ufw, dengan alamat baru Anda dan port panel:

   ```sh
   ufw allow from 198.51.100.7 to any port 28443 proto tcp
   ```

4. **Periksa apakah fail2ban memblokir Anda.** Jika Anda menginstal fail2ban dari panel, fail2ban memblokir sebuah alamat selama satu jam setelah 10 upaya masuk panel yang gagal dalam 10 menit (5 untuk SSH). Buka blokir alamat Anda:

   ```sh
   fail2ban-client set panel unbanip 198.51.100.7
   ```

   Untuk SSH, gunakan jail `sshd` alih-alih `panel`.

### "Terlalu banyak upaya yang gagal" {#too-many-failed-attempts}

Setelah 5 upaya masuk gagal dalam 15 menit, panel menolak upaya berikutnya dari alamat Anda selama 15 menit, dan menampilkan "Terlalu banyak upaya yang gagal. Coba lagi dalam … detik." Kode dua faktor yang salah juga dihitung. Tunggu sampai waktunya habis. Blokir ini disimpan di memori, jadi `systemctl restart nefo` juga menghapusnya.

### Panel tidak mau berjalan {#the-panel-doesn-t-start}

Baca alasannya di log:

```sh
journalctl -u nefo -n 50 --no-pager
```

| Isi log | Yang harus dilakukan |
|---|---|
| `address already in use` | Program lain memakai port panel. Hentikan program itu, atau ubah port di `/etc/nefo/config.json` (lihat [Baris perintah](./cli#the-configuration-file)). |
| Kesalahan tentang `/etc/nefo/config.json` | File konfigurasi berisi kesalahan, misalnya setelah Anda mengeditnya. Perbaiki baris yang disebutkan pesan tersebut. |
| `database schema version … is newer than this panel supports` | Versi program yang lebih lama berjalan dengan data dari versi yang lebih baru. Instal ulang rilis terbaru (lihat [Pembaruan](./update#if-an-update-fails)). |

Setelah diperbaiki, jalankan `systemctl restart nefo`.

## Anda lupa kata sandi {#you-forgot-the-password}

Atur kata sandi acak baru di server:

```sh
nefo reset-password
```

Perintah ini mencetak kata sandi baru dan mengeluarkan akun dari semua tempat. Masuk dengan kata sandi itu, lalu pilih kata sandi Anda sendiri di **Akun**.

## Ponsel 2FA hilang {#lost-your-2fa-phone}

Jika Anda tidak bisa lagi mendapatkan kode dari aplikasi autentikator, nonaktifkan autentikasi dua faktor dari server. Ini juga mengatur kata sandi baru:

```sh
nefo reset-password --disable-2fa
```

Masuk dengan kata sandi baru, lalu siapkan lagi autentikasi dua faktor di **Akun**, dengan ponsel baru Anda.

### Kode selalu "tidak valid" {#codes-are-always-not-valid}

Kode bergantung pada waktu. Panel menerima kode saat ini dan kode tepat sebelum dan sesudahnya, dan setiap kode hanya sekali. Jika setiap kode gagal, periksa bahwa jam server benar:

```sh
timedatectl
```

`System clock synchronized` seharusnya menyatakan `yes`. Periksa juga waktu di ponsel Anda.

## Browser memperingatkan tentang sertifikat {#the-browser-warns-about-the-certificate}

Sampai panel punya sertifikatnya sendiri, panel memakai sertifikat yang ditandatangani sendiri (self-signed), dan setiap browser memperingatkannya. Ini wajar. Beri panel domain dan sertifikat gratis agar peringatan hilang (lihat [Langkah pertama](./first-steps#give-the-panel-its-own-domain)).

Jika panel sudah punya sertifikat dan browser tetap memperingatkan:

- Anda membuka panel lewat alamat IP-nya. Sertifikat itu untuk domain: gunakan `https://panel.example.com:28443/k3x9p2qa7m1z/`.
- Sertifikat sudah kedaluwarsa karena tidak bisa diperpanjang. Periksa **Pengaturan** → **Sertifikat**, dan klik **Dapatkan sertifikat baru**. Penyebabnya sama seperti di bagian berikutnya.

## Permintaan sertifikat gagal {#a-certificate-request-fails}

Sebelum meminta ke Let's Encrypt, panel memeriksa bahwa setiap domain mengarah ke server ini: panel meletakkan file uji di server dan mengambilnya melalui domain lewat HTTP, di port 80. Jika gagal, panel tidak meminta apa pun dan mencantumkan setiap masalah di bawah "these names do not reach this server yet, so no certificate was requested" (nama-nama ini belum mengarah ke server ini, jadi tidak ada sertifikat yang diminta).

| Isi pesan | Penyebab dan solusi |
|---|---|
| `no DNS record. Add an A record that points to this server's IP address.` | Domain belum punya alamat. Tambahkan record A di penyedia DNS Anda. Record baru bisa butuh waktu sebelum berfungsi di semua tempat. |
| `no answer over HTTP on port 80 (…). Check the DNS record and the firewall.` | Port 80 tertutup, atau domain mengarah ke server lain. Buka port 80 di ufw dan di firewall cloud penyedia Anda, dan periksa bahwa server web berjalan. |
| `answered HTTP …, but not from this server. Check that the DNS record points here.` | Ada pihak lain yang menjawab untuk domain itu: record mengarah ke server lain, atau ada proxy atau CDN di depannya, seperti proxy Cloudflare (awan oranye). |

Setiap baris diawali domain dan alamat yang dituju domain itu. Jika alamat-alamat itu bukan alamat server Anda, perbaiki record DNS terlebih dahulu. Jika tercantum alamat IPv6 yang bukan milik server Anda, perbaiki atau hapus record AAAA.

**Di balik proxy Cloudflare:**

- Untuk situs web, atur record ke "DNS only" selama sertifikat diminta, atau centang **Validasi lewat DNS (Cloudflare)** di tab **SSL / HTTPS** situs web. Validasi DNS membutuhkan token API Cloudflare, yang Anda tambahkan di **Pengaturan** → **Cloudflare**. Nama wildcard (`*.example.com`) selalu membutuhkan validasi DNS.
- Untuk domain milik panel sendiri, record harus mengarah langsung ke server, tanpa proxy.

**Untuk domain milik panel sendiri,** tab **Sertifikat** menampilkan "Ini butuh server web" sampai Anda menginstalnya, karena server web yang menjawab Let's Encrypt di port 80. Instal dulu di halaman **Situs web**.

Jika pemeriksaan lolos tetapi Let's Encrypt tetap menolak, log tugas menampilkan alasannya. Let's Encrypt membatasi berapa banyak upaya gagal yang boleh Anda lakukan, jadi perbaiki penyebabnya sebelum mencoba lagi.

Jika perpanjangan otomatis gagal, panel menampilkan "Perpanjangan sertifikat untuk example.com gagal" dan mencoba lagi setiap 12 jam. Sertifikat tetap berfungsi sampai kedaluwarsa.

## Situs web menampilkan kesalahan {#a-website-shows-an-error}

| Apa yang dilihat pengunjung | Penyebab | Yang harus dilakukan |
|---|---|---|
| Teks "This website is temporarily unavailable." (HTTP 503) | Situs web ditangguhkan, secara manual atau karena tanggal kedaluwarsanya sudah lewat. | Buka tab **Kedaluwarsa** situs web. Hapus centang **Tangguhkan situs web ini**, dan kosongkan atau ubah **Tanggal kedaluwarsa**. Jika tanggalnya tetap di masa lalu, panel menangguhkan situs web lagi dalam satu jam. |
| Halaman "502 Bad Gateway" atau "503 Service Unavailable" dari server web | Program di balik situs web tidak menjawab: PHP-FPM untuk situs web PHP, atau aplikasi atau kontainer untuk situs web aplikasi. Nginx menampilkan 502, Apache dan OpenLiteSpeed mungkin menampilkan 503. | Untuk PHP, buka **Layanan** dan jalankan atau mulai ulang layanan PHP-FPM untuk versi PHP situs web, seperti `php8.3-fpm`. Untuk aplikasi, situs web menampilkan "Aplikasi berhenti": baca **Log aplikasi** di tab **Log**-nya. |
| Tidak ada jawaban untuk semua situs web | Server web berhenti. Halaman **Situs web** menyatakan bahwa server web "tidak berjalan". | Jalankan server web di halaman **Layanan**. |
| HTTP 429 | Batas laju situs web aktif (khusus Nginx), dan pengunjung mengirim terlalu banyak permintaan. | Naikkan batasnya, atau atur ke 0 untuk menonaktifkannya, di tab **Blokir akses** situs web. |
| HTTP 403 | Alamat pengunjung ada di daftar alamat yang diblokir situs web, atau firewall aplikasi web memblokir permintaan itu. | Periksa **Alamat IP yang diblokir** di tab **Blokir akses**, dan peristiwa firewall terbaru di tab **Firewall web (WAF)**. |

Untuk kesalahan lain, baca log kesalahan dan kesalahan PHP situs web di tab **Log**-nya.

## Unggahan atau pemulihan cadangan gagal {#a-backup-upload-or-restore-fails}

- **Pemulihan gagal dengan pesan "the passphrase does not unlock it".** Salinan itu dienkripsi dengan passphrase lain, bukan passphrase penyimpanan yang sekarang. Pulihkan lagi dan masukkan passphrase tersebut di **Passphrase enkripsi (opsional)**. Lihat [Mengenkripsi salinan](./backups#encrypt-the-copies).
- **Unggahan ke Google Drive gagal setelah seminggu.** OAuth consent screen masih berstatus **Testing**: ubah menjadi **In production** di Google Cloud console, lalu pilih **Ubah** → **Hubungkan lagi** di **Pencadangan** → **Penyimpanan jarak jauh**. Lihat [Google Drive](./backups#google-drive).
- **Cadangan situs web dipulihkan sebagai situs web statis.** Cadangan itu dibuat oleh versi sebelum 0.24.1, yang hanya mencatat versi PHP. Untuk situs web aplikasi atau reverse proxy, buat dulu situs webnya lagi dengan jenisnya, lalu pulihkan. Cadangan yang dibuat oleh 0.24.1 atau sesudahnya mengembalikan situs web beserta jenisnya. Lihat [Memulihkan situs web yang sudah dihapus](./backups#restore-a-website-that-was-deleted).

## Lokasi log {#where-the-logs-are}

| Log | Di server | Di panel |
|---|---|---|
| Panel | `journalctl -u nefo` | — |
| Log akses dan log kesalahan situs web | `/var/log/nginx/`, `/var/log/apache2/`, atau `/var/log/openlitespeed/`, sebagai `example.com.access.log` dan `example.com.error.log` | Situs web → (situs) → **Log** |
| Kesalahan PHP situs web | `/var/www/example.com/logs/php-error.log` | Situs web → (situs) → **Log** → **Kesalahan PHP** |
| Situs web aplikasi | `journalctl -u panel-app-example_com` | Situs web → (situs) → **Log** → **Log aplikasi** |
| Tugas cron | `/var/log/panel-cron/` | **Tugas cron** |
| Aktivitas masuk dan perubahan di panel | — | **Akun** → **Aktivitas terbaru** |

Saat Anda melaporkan masalah, sertakan keluaran `nefo version` dan baris log yang relevan. Hapus kata sandi dan path rahasia panel terlebih dahulu.
