---
title: Keamanan
description: Cara Nefo melindungi halaman masuknya sendiri, dan cara memakai pemeriksaan keamanan, firewall, fail2ban, pengaturan SSH, firewall aplikasi web, pemblokiran negara, penguncian file, dan pemindai malware.
---

# Keamanan

Nefo berjalan sebagai root, jadi Nefo melindungi halaman masuknya sendiri lebih dulu: alamat rahasia di port acak, HTTPS sejak pertama dijalankan, autentikasi dua faktor, batas upaya masuk, dan log aktivitas. Nefo juga mengelola firewall server (ufw), fail2ban, dan pengaturan SSH dengan pengaman agar Anda tidak terkunci, serta melindungi situs web dengan firewall aplikasi web, batas laju, alamat yang diblokir, dan pemindai malware.

Halaman ini membahas:

- perlindungan panel itu sendiri: alamat tersembunyi, HTTPS, batas upaya masuk, autentikasi dua faktor, sesi, dan log aktivitas;
- [pemeriksaan keamanan](./security-check), yang mencantumkan pengaturan yang membuat server terbuka dan tempat memperbaiki masing-masing;
- firewall server (ufw), fail2ban, dan pengaturan SSH, di halaman **Firewall & SSH**;
- keamanan situs web: firewall aplikasi web, batas laju, alamat yang diblokir, pemblokiran negara, penguncian file, dan pemindai malware.

Nefo masih beta dan belum pernah diaudit keamanannya oleh pihak luar. Lihat [kebijakan keamanan](../security-policy).

## Pemeriksaan keamanan {#security-check}

**Pemeriksaan keamanan**, di kelompok **Keamanan** pada menu kiri, menjalankan 22 pemeriksaan cepat pada server, seperti masuk SSH dengan kata sandi, firewall, pembaruan keamanan yang menunggu, basis data yang terbuka ke internet, port Docker, file yang tidak sengaja disajikan situs web, dan sertifikat. Setiap temuan menautkan ke halaman yang memperbaikinya. Pemeriksaan berjalan saat diminta dan sekali sehari. Lihat [Pemeriksaan keamanan](./security-check).

## Perlindungan panel itu sendiri {#the-panel-s-own-protection}

### Path masuk tersembunyi {#hidden-entrance-path}

Penginstal memilih port acak antara 20000 dan 49999 dan path rahasia berisi 12 huruf kecil dan angka. Panel hanya menjawab di bawah path itu, misalnya `https://203.0.113.10:28443/k3x9p2qa7m1z/`. Setiap URL lain di port itu mengembalikan `404`, jadi pemindai tidak bisa tahu bahwa ada panel yang berjalan di sana.

Jika Anda kehilangan alamatnya, jalankan ini di server:

```sh
nefo info
```

Perintah ini mencetak URL dengan alamat IP pertama server, dan juga dengan domain panel setelah panel punya domain (lihat di bawah). Jika server berada di balik alamat publik yang terpisah, jalankan `nefo info --host 203.0.113.10` untuk mencetak URL dengan alamat itu.

Port dan path disimpan sebagai `listen` dan `entrance` di `/etc/nefo/config.json`. Panel tidak punya pengaturan untuk mengubahnya. Untuk mengubah path secara manual, atur `entrance` menjadi 8 sampai 32 huruf kecil dan angka, lalu jalankan `systemctl restart nefo`.

### HTTPS sejak pertama dijalankan {#https-from-the-first-start}

Panel menyajikan HTTPS sejak pertama kali dijalankan, dengan TLS 1.2 atau lebih baru. Panel membuat sertifikat yang ditandatangani sendiri di `/var/lib/nefo/tls/`, jadi browser Anda memberi peringatan pada kali pertama. Untuk menghilangkan peringatan itu, dapatkan sertifikat Let's Encrypt gratis untuk domain yang mengarah ke server ini:

1. Instal server web dari **Situs web** jika belum. Let's Encrypt memeriksa domain di port 80, dan server web yang menjawabnya.
2. Arahkan record A domain ke server, tanpa proxy atau CDN.
3. Buka **Pengaturan** → **Sertifikat**.
4. Masukkan **Domain panel**, misalnya `panel.example.com`, lalu pilih **Dapatkan sertifikat**.
5. Buka panel di `https://panel.example.com:28443/k3x9p2qa7m1z/`, dengan port dan path Anda sendiri.

Sertifikat diperpanjang dengan sendirinya, seperti sertifikat situs web; perpanjangan yang gagal dicoba lagi setiap 12 jam. Untuk menghapusnya, pilih **Hapus**: panel kembali memakai sertifikat yang ditandatangani sendiri. Lihat [Sertifikat SSL](./ssl) untuk sertifikat secara umum.

### Batas upaya masuk {#sign-in-limits}

- 5 upaya gagal dalam 15 menit memblokir alamat tersebut selama 15 menit. Halaman masuk lalu menampilkan "Terlalu banyak upaya yang gagal. Coba lagi dalam … detik."
- Kata sandi yang salah dan kode dua faktor yang salah sama-sama dihitung. Begitu pula kata sandi saat ini yang salah di formulir kata sandi dan 2FA pada halaman **Akun**.
- Alamat IPv6 dihitung per jaringan /64, karena satu pengguna biasanya menguasai seluruh /64.
- Upaya masuk yang berhasil menghapus hitungan.
- Nama pengguna yang salah dan kata sandi yang salah memberi pesan yang sama dan memakan waktu yang sama, jadi halaman masuk tidak membocorkan nama pengguna mana yang ada.
- Panel menghitung alamat yang terhubung kepadanya dan mengabaikan `X-Forwarded-For`. Jika Anda memasang proxy di depan panel, semua pengunjung berbagi satu hitungan.

Kata sandi disimpan sebagai hash bcrypt, tidak pernah dalam teks biasa.

::: tip
Jika Anda terkunci, tunggu 15 menit. Hitungan disimpan di memori, jadi memulai ulang panel di server (`systemctl restart nefo`) juga menghapusnya.
:::

### Sesi dan keluar {#sessions-and-signing-out}

- Satu kali masuk berlaku 12 jam. Setelah itu Anda masuk lagi, meskipun Anda sedang aktif.
- Untuk keluar, buka menu akun di kanan atas dan pilih **Keluar**.
- Mengganti kata sandi mengeluarkan Anda dari semua perangkat. Begitu pula `nefo reset-password` di server.
- Cookie sesi tidak bisa dibaca skrip, hanya dikirim lewat HTTPS, dan hanya ke path milik panel sendiri.

### Header keamanan {#security-headers}

Setiap halaman dikirim dengan Content Security Policy ketat yang hanya mengizinkan skrip dari panel itu sendiri. Panel juga mengirim `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer` (yang menjaga path rahasia tidak masuk ke log situs web lain), dan `Permissions-Policy` yang menonaktifkan kamera, mikrofon, dan lokasi. Setiap perubahan membutuhkan header permintaan yang tidak bisa dikirim situs web lain, sehingga cross-site request forgery terblokir.

## Autentikasi dua faktor {#two-factor-authentication}

Autentikasi dua faktor (2FA) meminta kode 6 digit dari aplikasi autentikator setiap kali masuk, jadi kata sandi yang dicuri tidak cukup. Aplikasi apa pun yang mendukung kode berbasis waktu standar bisa dipakai, seperti Google Authenticator, Authy, atau 2FAS. Selama 2FA nonaktif, panel mencantumkan "Masuk dua faktor nonaktif" sebagai masalah serius.

### Mengaktifkan 2FA {#turn-on-2fa}

1. Buka **Akun** di menu kiri. Anda juga bisa membuka menu akun dan memilih **Kata sandi dan 2FA**.
2. Di bawah **Autentikasi dua faktor**, pilih **Siapkan**.
3. Pindai kode QR dengan aplikasi Anda, atau ketik **Kunci penyiapan** ke dalamnya. Aplikasi mencantumkan akun ini sebagai Nefo.
4. Masukkan kode 6 digit yang ditampilkan aplikasi, lalu pilih **Aktifkan**.

Mulai sekarang, halaman masuk meminta **Kode dua faktor** setelah kata sandi Anda.

Kode berganti setiap 30 detik. Panel juga menerima kode sebelumnya dan kode berikutnya, untuk berjaga-jaga jika jam sedikit berbeda. Setiap kode hanya berlaku sekali: jika kode ditolak, tunggu kode berikutnya.

### Menonaktifkan 2FA {#turn-off-2fa}

Di **Akun**, di bawah **Autentikasi dua faktor**, masukkan **Kata sandi** dan **Kode** yang berlaku saat ini, lalu pilih **Nonaktifkan**.

### Jika ponsel Anda hilang {#if-you-lose-your-phone}

Nefo tidak punya kode cadangan. Anda memulihkan akses lewat server: masuk lewat SSH sebagai root dan jalankan:

```sh
nefo reset-password --disable-2fa
```

Perintah ini mengatur kata sandi acak baru untuk akun `admin`, menonaktifkan 2FA, mengakhiri semua sesi, dan mencetak kata sandi baru. Masuk dengan kata sandi itu, ganti kata sandinya, lalu siapkan 2FA lagi. Lihat [Baris perintah](./cli).

::: warning
Tanpa ponsel Anda, akses root ke server adalah satu-satunya jalan kembali ke panel. Simpan kunci SSH atau kata sandi root Anda, atau akses ke konsol web penyedia Anda, dengan aman.
:::

## Mengganti kata sandi panel {#change-the-panel-password}

1. Buka **Akun** → **Ganti kata sandi**.
2. Masukkan **Kata sandi saat ini**, lalu **Kata sandi baru** dua kali (**Ulangi kata sandi baru**). Panjangnya harus 12 sampai 72 karakter.
3. Pilih **Ganti kata sandi**.

Anda dikeluarkan dari semua perangkat. Masuk lagi dengan kata sandi baru.

## Log aktivitas {#activity-log}

**Akun** → **Aktivitas terbaru** menampilkan 100 entri terbaru, dengan waktu, tindakan, pengguna, alamat IP, dan detailnya. Tampilkan **Semua** entri atau hanya **Kegagalan**, dan ketik di kotak filter untuk mencari. Bagian atas halaman menampilkan waktu masuk terakhir Anda dan berapa banyak upaya masuk gagal yang tercatat di log.

Log mencatat:

- masuk, keluar, dan upaya masuk yang gagal, beserta alasannya (nama pengguna atau kata sandi salah, atau kode 2FA salah);
- penggantian kata sandi, reset kata sandi dari baris perintah, serta 2FA yang diaktifkan atau dinonaktifkan;
- perubahan yang dibuat di panel: situs web, sertifikat, basis data, file, cadangan, tugas cron, layanan, aturan firewall, pengaturan dan kunci SSH, perangkat lunak, dan Docker;
- membuka dan menutup terminal, beserta berapa lama terminal terbuka;
- mulai ulang panel dan reboot server.

Perpanjangan sertifikat otomatis dicatat dengan pengguna `system`. Entri disimpan selama 90 hari. Setiap entri juga ditulis ke log layanan panel, yang bisa Anda baca dengan `journalctl -u nefo`. fail2ban membaca upaya masuk yang gagal dari sana.

## Firewall (ufw) {#firewall-ufw}

Nefo mengelola firewall server dengan ufw. Buka **Firewall & SSH** → **Firewall (ufw)**.

Jika halaman menyatakan ufw tidak terinstal, instal di server. Panel tidak menginstalnya:

```sh
apt install ufw
```

### Mengaktifkan firewall {#turn-on-the-firewall}

1. Pilih **Aktifkan firewall** dan konfirmasi.
2. Panel lebih dulu mengizinkan port SSH (komentar `SSH`) dan port panel (komentar `Nefo`). Jika server web terinstal, panel juga mengizinkan port 80 dan 443 untuk situs web (komentar `Websites`), dan 443/UDP jika OpenLiteSpeed melayani HTTP/3. Lalu panel mengaktifkan ufw. Panel menambahkan aturan-aturan ini meskipun Anda sudah menambahkan aturan untuk salah satu port tersebut selama firewall nonaktif, misalnya aturan yang dibatasi ke alamat Anda, jadi mengaktifkan firewall tidak pernah membuat Anda terkunci. Untuk mempertahankan aturan Anda saja, hapus aturan untuk semua orang sesudahnya (lihat akhir bagian [Apa yang ditolak firewall](#what-the-firewall-refuses)).

Anda bisa [menambahkan aturan](#add-a-rule) sebelum mengaktifkan firewall, misalnya untuk port yang dibutuhkan program lain di server. Selama firewall nonaktif, halaman menampilkan aturan-aturan itu di bawah **Aktifkan firewall** sebagai perintah ufw, seperti `ufw allow 8080/tcp`, di kolom **Aturan (perintah ufw)**, dengan setiap pasangan IPv4 dan IPv6 ditampilkan sekali. Aturan-aturan itu berlaku saat Anda mengaktifkan firewall. ufw hanya memberi nomor pada aturannya selama aktif, jadi untuk menghapus salah satunya, aktifkan firewall lebih dulu.

Semua port masuk lainnya tertutup kecuali ada aturan yang mengizinkannya. Jika Anda menginstal server web belakangan, dengan firewall aktif, Nefo membuka port 80 dan 443 pada saat itu.

Jika penyedia Anda punya firewall cloud (security group), buka juga port yang sama di sana. Nefo tidak bisa mengubahnya.

### Menambahkan aturan {#add-a-rule}

1. Pilih **Tambah aturan**.
2. Pilih **Izinkan** atau **Tolak**.
3. Di **Port, mis. 8080**, masukkan satu port, atau rentang seperti `6000:6010`.
4. Pilih **Protokol**: TCP, UDP, atau TCP + UDP. Rentang membutuhkan satu protokol: dengan TCP + UDP, rentang menjadi TCP.
5. Di **Dari**, masukkan alamat IP atau jaringan seperti `198.51.100.0/24`. Kosongkan untuk mengizinkan atau menolak semua orang.
6. Opsional, tambahkan **Komentar** hingga 60 karakter, tanpa tanda kutip.
7. Pilih **Tambah**.

Halaman menampilkan alamat IP Anda sendiri di atas daftar, yang membantu saat Anda mengizinkan port hanya untuk diri sendiri.

ufw memeriksa aturan dari atas ke bawah dan memakai aturan pertama yang cocok. Aturan **Tolak** dengan alamat di **Dari** diletakkan di atas daftar, jadi aturan itu tetap berlaku pada port yang diizinkan untuk semua orang oleh aturan di bawahnya. Aturan lain ditambahkan di akhir.

::: tip
Untuk menjauhkan sebuah alamat dari satu situs web saja, gunakan pengaturan **Blokir akses** milik situs web tersebut (lihat di bawah).
:::

Daftar ini juga menampilkan aturan yang Anda tambahkan dengan ufw di baris perintah. **Batasi** mengizinkan port tetapi, untuk sementara, menolak alamat yang membuka 6 koneksi dalam 30 detik (`ufw limit`). **Tolak langsung** menolak seperti **Tolak**, tetapi langsung memberi tahu pihak lain.

### Menghapus aturan {#delete-a-rule}

Pilih ikon tempat sampah di ujung baris aturan dan konfirmasi. Jika aturan berubah sejak Anda membuka halaman, panel menolak dan meminta Anda memuat ulang, jadi panel tidak pernah menghapus aturan yang salah.

### Apa yang ditolak firewall {#what-the-firewall-refuses}

Pengaman berikut mencegah Anda mengunci diri sendiri:

- Aturan yang mengizinkan port SSH atau port panel menampilkan **dilindungi** jika aturan itu adalah jalan masuk terakhir Anda dari alamat yang Anda pakai sekarang (ditampilkan di atas daftar). Aturan itu tidak bisa dihapus di sini. Jika ada aturan lain yang juga mengizinkan alamat Anda masuk, aturan itu bisa dihapus.
- Aturan **Tolak** untuk port SSH atau port panel dengan **Dari** kosong ditolak. Aturan **Tolak** untuk satu alamat atau jaringan diizinkan.

Agar hanya alamat Anda sendiri yang bisa menjangkau panel: tambahkan aturan **Izinkan** untuk port panel dengan alamat Anda di **Dari**, lalu hapus aturan yang mengizinkan port itu dari mana saja (beserta salinan IPv6-nya). Jika alamat Anda berubah nanti, gunakan konsol web penyedia Anda untuk menjalankan `ufw allow <port>/tcp`.

Aturan untuk akses basis data jarak jauh (port 3306 untuk MariaDB dan MySQL, port cluster untuk PostgreSQL, bawaannya 5432) ditambahkan dan dihapus dari halaman **Basis data**. Ubah aturan itu di sana. Lihat [Basis data](./databases#allow-remote-access) dan [PostgreSQL](./postgresql#postgresql-remote-access).

## Perlindungan brute-force (fail2ban) {#brute-force-protection-fail2ban}

fail2ban memblokir alamat yang terus gagal masuk, ke SSH dan ke panel.

### Menginstal fail2ban {#install-fail2ban}

1. Buka **Firewall & SSH** → **Perlindungan brute-force (fail2ban)**.
2. Pilih **Instal fail2ban**. Instalasi berjalan di latar belakang dengan log langsung.

Nefo menyiapkan dua jail di `/etc/fail2ban/jail.d/panel.conf`:

| Jail | Yang dipantau | Memblokir setelah | Lama blokir |
|---|---|---|---|
| `sshd` | upaya masuk SSH, termasuk SFTP | 5 kegagalan dalam 10 menit | 1 jam |
| `panel` | upaya masuk ke panel yang gagal, atau ditolak selama alamat sedang diblokir | 10 dalam 10 menit | 1 jam |

Alamat yang diblokir tidak bisa menjangkau port yang dilindungi jail tersebut. Batas upaya masuk milik panel sendiri (5 kegagalan dalam 15 menit) bertindak lebih dulu. Alamat yang terus mencoba selama diblokir kemudian diblokir oleh jail `panel` di firewall.

Panel menulis ulang file ini, beserta filternya di `/etc/fail2ban/filter.d/panel.conf`, jadi jangan edit keduanya secara manual. Saat Anda mengubah port SSH, jail `sshd` ikut menyesuaikan.

### Melihat dan membuka blokir alamat {#see-and-unblock-addresses}

Tabel mencantumkan setiap **Layanan yang dilindungi**, **Kegagalan terbaru**-nya, dan alamat yang **Diblokir sekarang**, beserta jumlah total blokir. Untuk membuka blokir sebuah alamat, pilih **Buka blokir** di sebelahnya.

Jika fail2ban terinstal tetapi berhenti, jalankan di halaman **Layanan**.

::: tip
Jika Anda memblokir diri sendiri dari SSH, masuk ke panel dari jaringan Anda saat ini dan pilih **Buka blokir**. Anda juga bisa menjalankan `fail2ban-client set sshd unbanip 203.0.113.10` dari **Terminal** panel.
:::

## Pengaturan SSH {#ssh-settings}

Buka **Firewall & SSH** → **SSH**.

| Pengaturan | Pilihan |
|---|---|
| **Port** | 1 sampai 65535 |
| **Login root** | **Hanya dengan kunci SSH (disarankan)**, **Dengan kunci atau kata sandi**, **Tidak diizinkan** |
| **Izinkan masuk dengan kata sandi** | Aktif atau nonaktif, untuk setiap akun |

Pilih **Simpan**. Panel menulis pengaturannya ke `/etc/ssh/sshd_config.d/00-panel.conf`, yang diutamakan di atas file pengaturan SSH lainnya. Panel juga menonaktifkan masuk keyboard-interactive. Sebelum memulai ulang SSH, panel menguji pengaturan baru dengan `sshd -t`. Jika pengujian gagal, file lama dikembalikan dan tidak ada yang berubah.

::: warning
Biarkan sesi SSH Anda saat ini tetap terbuka setelah menyimpan. Buka koneksi baru dan pastikan Anda bisa masuk sebelum menutup sesi yang lama.
:::

### Apa yang ditolak pengaturan SSH {#what-the-ssh-settings-refuse}

- Pengaturan yang membuat tidak ada lagi yang bisa masuk lewat SSH. Misalnya: menonaktifkan masuk dengan kata sandi selagi tidak ada akun yang punya kunci SSH; login root **Hanya dengan kunci SSH** selagi root tidak punya kunci; atau root **Tidak diizinkan** selagi tidak ada akun lain yang bisa masuk.
- Port yang sudah dipakai: port milik panel, 80, 443, atau port apa pun yang didengarkan program lain.
- Port baru jika SSH dijalankan oleh systemd socket activation. Halaman lalu menampilkan **diaktifkan oleh socket** dan kolom port dikunci. Ubah port di baris perintah.

Untuk memutuskan apakah seseorang masih bisa masuk, panel memeriksa root dan akun milik orang (UID 1000 ke atas) yang punya login shell: kunci mereka di `~/.ssh/authorized_keys` (atau file yang diatur `AuthorizedKeysFile`), kata sandi mereka, serta `AllowUsers`, `DenyUsers`, `AllowGroups`, dan `DenyGroups`. Panel tidak membaca blok `Match`. Halaman mencantumkan siapa yang bisa masuk sekarang, dan dengan cara apa (kunci atau kata sandi). Jika root tidak punya kunci, halaman juga menampilkan peringatan dengan tombol **Tambah kunci** (lihat [Kunci SSH](#ssh-keys)).

SSH tidak bisa dihentikan dari halaman **Layanan**, hanya dimulai ulang atau dimuat ulang.

::: warning
Dengan login root **Tidak diizinkan**, Anda membutuhkan akun lain yang bisa memakai `sudo`; tanpa itu Anda bisa masuk tetapi tidak bisa mengelola server lewat SSH. Nefo tidak membuat akun itu, dan tidak memeriksa apakah akun itu bisa memakai `sudo`.
:::

### Mengubah port SSH {#change-the-ssh-port}

1. Masukkan **Port** baru, pilih **Simpan**, lalu konfirmasi.
2. Jika firewall aktif, panel membuka port baru (komentar `SSH`) sebelum SSH pindah ke port itu. Jika fail2ban terinstal, jail `sshd`-nya juga pindah ke port baru.
3. Jika penyedia Anda punya firewall cloud, buka juga port baru di sana.
4. Uji koneksi baru, misalnya `ssh -p 2222 root@203.0.113.10`.
5. Aturan firewall untuk port lama tetap ada, dan tidak lagi dilindungi. Hapus di bawah **Firewall (ufw)** setelah port baru berfungsi.

Akun SFTP situs web Anda terhubung di port yang sama.

### Kunci SSH {#ssh-keys}

**Kunci SSH**, di bawah pengaturan SSH, mencantumkan kunci publik root dan akun milik orang (UID 1000 ke atas, dengan login shell), dikelompokkan per akun. Nefo membacanya dari `~/.ssh/authorized_keys`, atau dari file yang diatur `AuthorizedKeysFile`. Setiap kunci menampilkan jenis dan ukurannya (seperti `ed25519` atau `rsa 4096`), komentarnya, fingerprint SHA256-nya, dan opsinya jika ada, seperti `from="…"` atau `command="…"`. Kunci DSA, atau kunci RSA di bawah 2048 bit, ditandai **lemah**: OpenSSH versi saat ini menolaknya. Tambahkan kunci baru, lalu hapus kunci yang lemah.

Akun SFTP situs web Anda tidak dicantumkan: kuncinya ada di pengaturan SFTP masing-masing situs web.

#### Menambahkan kunci {#add-a-key}

1. Di komputer Anda, buat pasangan kunci jika belum punya. Cara ini sama di PowerShell pada Windows 10 dan yang lebih baru:
   ```sh
   ssh-keygen -t ed25519
   ```
   Kunci privat disimpan di `~/.ssh/id_ed25519` dan tetap di komputer Anda. Kunci publik disimpan di `~/.ssh/id_ed25519.pub`.
2. Tampilkan kunci publik dan salin seluruh barisnya, yang diawali `ssh-ed25519`:
   ```sh
   cat ~/.ssh/id_ed25519.pub
   ```
3. Di panel, di bawah **Kunci SSH**, pilih **Tambah kunci**.
4. Pilih **Akun**, tempelkan baris tadi ke **Kunci publik**, lalu pilih **Tambah kunci**.
5. Uji dari terminal baru, misalnya `ssh root@203.0.113.10`.

Nefo menambahkan kunci di akhir `~/.ssh/authorized_keys` milik akun tersebut. Jika folder atau file-nya tidak ada, Nefo membuatnya, dimiliki akun itu, dengan mode 700 dan 600. Nefo menolak:

- kunci privat, seperti yang diawali `-----BEGIN OPENSSH PRIVATE KEY-----`. Jika Anda tidak sengaja menempelkan kunci privat di suatu tempat, buat pasangan kunci baru;
- lebih dari satu baris. Di PuTTYgen, salin kotak **Public key for pasting into OpenSSH authorized_keys file**;
- kunci DSA, dan kunci RSA di bawah 2048 bit;
- kunci yang sudah dimiliki akun tersebut;
- file `authorized_keys` atau folder `.ssh` yang berupa tautan simbolik, atau bukan file atau folder biasa. Sebuah akun bisa mengarahkannya ke file lain di server, jadi Nefo tidak membaca maupun mengubahnya.

#### Menghapus kunci {#remove-a-key}

Pilih ikon tempat sampah di sebelah kunci dan konfirmasi. Konfirmasi menyebut kunci berdasarkan jenis, fingerprint, dan komentarnya. Jika dengan pengaturan saat ini tidak ada yang bisa masuk lewat SSH tanpa kunci ini, panel menolak: tambahkan kunci lain lebih dulu, atau izinkan masuk dengan kata sandi.

Kunci yang ditambahkan dan dihapus dicatat di log aktivitas, beserta akun dan fingerprint kuncinya.

### Menonaktifkan masuk dengan kata sandi {#turn-off-password-sign-in}

1. Di bawah [Kunci SSH](#ssh-keys), tambahkan kunci publik Anda untuk root, atau untuk akun yang Anda pakai untuk masuk.
2. Buka terminal baru dan masuk dengan kunci itu. Jika SSH meminta kata sandi, kunci itu tidak dipakai: periksa bahwa Anda menambahkannya ke akun yang Anda pakai untuk masuk.
3. Nonaktifkan **Izinkan masuk dengan kata sandi** dan pilih **Simpan**. Jika tidak ada yang punya kunci, panel menolak (lihat [Apa yang ditolak pengaturan SSH](#what-the-ssh-settings-refuse)).
4. Biarkan sesi Anda saat ini tetap terbuka, dan periksa koneksi baru sekali lagi sebelum menutupnya.

Akun SFTP situs web Anda tetap memakai cara masuknya sendiri, kata sandi atau kunci, meskipun masuk dengan kata sandi untuk SSH dinonaktifkan.

Selama root bisa masuk dengan kata sandi, panel mencantumkan "Root bisa masuk lewat SSH dengan kata sandi" sebagai masalah serius.

## Keamanan situs web {#website-security}

Setiap situs web punya pengaturan keamanannya sendiri. Buka **Situs web**, buka situs web dengan memilih namanya atau **Pengaturan**, lalu gunakan tab di bawah **Keamanan**.

### Firewall aplikasi web {#web-application-firewall}

Firewall aplikasi web (WAF) adalah ModSecurity dengan OWASP Core Rule Set, keduanya gratis dan sumber terbuka. WAF memeriksa setiap permintaan dan menghentikan SQL injection, cross-site scripting (XSS), file inclusion, pemindai yang dikenal, dan serangan umum lainnya. WAF bekerja dengan Nginx, Apache, dan OpenLiteSpeed.

1. Instal sekali: **Perangkat lunak** → **Firewall web (ModSecurity)** → **Instal**. Firewall tetap nonaktif di situs web sampai Anda mengaktifkannya. Jika distribusi tidak punya modul ModSecurity untuk server web, kartunya menyebutkan hal itu dan tidak menawarkan instalasi: Ubuntu 22.04 tidak punya modul untuk Nginx, sedangkan Apache dan OpenLiteSpeed berfungsi di sana, dan Nginx berfungsi mulai Ubuntu 24.04.
2. Buka **Situs web** → (situs) → **Firewall web (WAF)**.
3. Pilih mode:
   - **Nonaktif**
   - **Hanya deteksi: catat serangan, jangan blokir apa pun**
   - **Blokir serangan**: pengunjung mendapat HTTP 403.
4. Pilih **Simpan**.

Mulailah dengan mode hanya deteksi selama beberapa hari, dan periksa peristiwanya untuk alarm palsu sebelum Anda memblokir. Situs web yang menjalankan WordPress atau memakai preset rewrite WordPress atau Drupal otomatis mendapat pengecualian rule set untuk aplikasi tersebut.

**Peristiwa firewall terbaru** mencantumkan 50 permintaan terbaru yang ditandai aturan di situs web ini: waktu, alamat IP, permintaan, aturan yang cocok, dan apakah permintaan itu **diblokir** atau hanya **dicatat**. Peristiwa ini berasal dari `/var/log/panel-waf/audit.log`, yang dirotasi setiap hari dan disimpan selama 7 hari.

Hanya 13 MB pertama dari sebuah permintaan yang diperiksa, jadi unggahan besar tetap berfungsi. Jika Anda menghapus WAF di halaman **Perangkat lunak**, setiap situs web mempertahankan modenya, yang berlaku lagi saat Anda menginstal ulang.

### Batas laju dan alamat yang diblokir {#rate-limit-and-blocked-addresses}

Buka **Situs web** → (situs) → **Blokir akses**.

- **Batas laju (permintaan per detik per pengunjung, 0 = nonaktif)**: hanya menghitung permintaan PHP dan aplikasi, bukan gambar, CSS, atau skrip. Lonjakan hingga lima kali batas diizinkan; di atas itu, pengunjung mendapat HTTP 429. 10 cocok untuk sebagian besar situs web, dan nilai tertingginya 1000. Hanya Nginx yang punya pengaturan ini: di Apache dan OpenLiteSpeed pengaturan ini tidak ditampilkan.
- **Alamat IP yang diblokir**: satu alamat atau rentang IPv4 atau IPv6 per baris, seperti `203.0.113.7` atau `198.51.100.0/24`, hingga 200 per situs web. Pengunjung yang diblokir mendapat HTTP 403.

Pilih **Simpan**. Keduanya bekerja sendiri, tanpa firewall aplikasi web, dan di setiap jenis situs web, termasuk situs web reverse proxy. Tab yang sama punya **Perlindungan kata sandi**, yang dijelaskan di [Situs web](./websites#suspend-redirect-or-password-protect-a-website).

### Pemindaian malware {#malware-scan}

Pemindai mencari web shell, backdoor, kode tersembunyi, dan skrip sisipan di file situs web. Pemindai tidak mengubah apa pun sampai Anda mengarantina sebuah file.

1. Buka **Situs web** → (situs) → **Pemindaian malware**.
2. Pilih **Pindai sekarang**. Pemindaian berjalan di latar belakang dengan log langsung.

Pemindai membaca folder situs web, seperti `/var/www/example.com`. Pemindai melewati folder `.git` dan file di atas 5 MB, dan tidak mengikuti tautan simbolik. Pemindai memeriksa file PHP (juga kode PHP di file dengan nama lain), file JavaScript dan HTML, serta gambar.

| Temuan | Tingkat keparahan |
|---|---|
| Web shell yang dikenal | kritis |
| Menjalankan kode yang dikirim pengunjung | kritis |
| Memanggil fungsi yang namanya ditentukan pengunjung | kritis |
| Menjalankan perintah shell yang dikirim pengunjung | kritis |
| Menjalankan kode tersembunyi (terenkode) | tinggi |
| Mendekode data tersembunyi yang dikirim pengunjung | tinggi |
| Membuat fungsi dari masukan pengunjung | tinggi |
| Pengunggah yang membiarkan pengunjung memilih lokasi file | tinggi |
| Sisipan skrip yang disamarkan | tinggi |
| Kode PHP tersembunyi di dalam gambar | tinggi |
| Eksekusi kode lewat preg_replace /e | periksa |
| Data terenkode panjang di samping kode yang mendekodenya | periksa |
| Teks yang sangat disamarkan di samping kode yang menjalankannya | periksa |
| File PHP di folder unggahan | periksa |

Setiap temuan menampilkan file, baris, dan kutipan singkat. File PHP di folder unggahan yang hanya berisi komentar, seperti `index.php` "Silence is golden" milik WordPress, tidak dilaporkan.

Panel menyimpan pemindaian terakhir setiap situs web. Panel tidak memindai sesuai jadwal: mulai pemindaian saat Anda membutuhkannya.

### Karantina dan pemulihan {#quarantine-and-restore}

Pilih **Karantina** di sebelah sebuah temuan untuk memindahkan file keluar dari situs web, ke `/var/lib/panel-quarantine`, yang hanya bisa dibaca root. Panel mencatat pemilik dan izin file tersebut.

Di bawah **File yang dikarantina**:

- **Pulihkan** mengembalikan file ke tempat asalnya, dengan pemilik dan izinnya. Pemulihan ditolak jika sekarang sudah ada file di path itu, atau jika foldernya sudah tidak ada.
- **Hapus** menghapus file secara permanen, setelah Anda mengonfirmasi.

Saat Anda menghapus situs web, file-file situs itu yang dikarantina juga dihapus.

::: warning
Mengarantina file yang dibutuhkan situs web, seperti file plugin, bisa merusak situs web. Periksa kutipannya lebih dulu, dan pulihkan file itu jika ternyata alarm palsu.
:::

### Memindai dengan ClamAV {#scan-with-clamav}

ClamAV adalah mesin pemindai tambahan yang opsional.

1. Instal **Antivirus ClamAV** di halaman **Perangkat lunak**. Basis data virusnya diunduh di latar belakang selama beberapa menit setelah instalasi.
2. Di **Pemindaian malware**, centang **Pindai juga dengan ClamAV (lebih lambat, butuh memori bebas sekitar 1,4 GB)** dan pilih **Pindai sekarang**.

ClamAV membutuhkan memori bebas sekitar 1,4 GB saat memindai, jadi cocok untuk server dengan 2 GB atau lebih. Jika memori bebas lebih sedikit, panel melewati ClamAV dan menyebutkannya di laporan. ClamAV berjalan dengan prioritas CPU terendah dan melewati file di atas 25 MB. Temuannya ditandai tinggi.

### Pemblokiran negara dan penguncian file {#country-blocking-and-file-lock}

Setiap situs web juga punya **Pemblokiran negara**, yang menolak pengunjung dari negara yang Anda cantumkan atau hanya mengizinkan pengunjung dari negara tersebut, dan **Penguncian file**, yang membuat kode situs web hanya-baca bagi PHP-nya dan akun SFTP-nya, kecuali folder yang harus ditulisinya. Lihat [Pemblokiran negara dan penguncian file](./website-protection).

## Riwayat masuk SSH {#ssh-sign-in-history}

Daftar upaya masuk SSH dan percobaan yang gagal ada di halaman **Proses**. Lihat [Upaya masuk SSH](./system#ssh-sign-ins). Untuk mendapat pesan saat seseorang masuk lewat SSH atau ke panel, atau saat upaya masuk SSH terus gagal, lihat [Peringatan masuk](./mail-alerts#sign-in-alerts).
