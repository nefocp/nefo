---
title: Alat sistem
description: Dasbor, tugas cron, layanan, program latar belakang, proses dan port, upaya masuk SSH, terminal web, log, serta memulai ulang panel atau me-reboot server.
---

# Alat sistem

**Dasbor** menampilkan CPU, memori, disk, dan beban server secara langsung dengan riwayat 7 hari, serta mencantumkan hal yang perlu perhatian. Alat server Nefo juga mengelola tugas cron, layanan systemd, program latar belakang yang harus terus berjalan, proses dan port yang mendengarkan, upaya masuk SSH, dan terminal root di browser, serta memulai ulang panel atau me-reboot server.

Halaman ini membahas setiap alat tersebut, dan tempat menemukan log. Sebagian besar alat ini ada di kelompok **Server** pada menu kiri. Pembaruan sistem operasi, swap, zona waktu dan nama host, TCP BBR, dan pembersihan disk ada di halaman [Sistem](./os).

## Dasbor {#dashboard}

**Dasbor** terbuka setelah Anda masuk.

- Bagian atas halaman menampilkan nama server, sistem operasi, uptime, jumlah CPU, dan memori. Di VPS yang berupa kontainer (LXC), memori yang ditampilkan adalah batas kontainer, dengan tanda "batas kontainer" (lihat [VPS yang berupa kontainer](./first-steps#a-vps-that-is-a-container)). Tombol-tombol di sana memungkinkan Anda **Periksa pembaruan**, membuka **Terminal**, dan memakai menu **Daya**.
- Empat pengukur menampilkan **Beban sistem**, CPU, **Memori**, dan **Disk /** secara langsung. Pengukur diperbarui setiap 5 detik. Beban sistem adalah rata-rata beban 1 menit dibagi jumlah CPU.
- Ubin menghitung situs web, basis data, akun SFTP, tugas cron, cadangan, dan risiko keamanan. Pilih sebuah ubin untuk membuka halamannya. Jika tidak ada masalah serius yang terbuka, ubin **Risiko keamanan** menampilkan kapan [pemeriksaan keamanan](./security-check) terakhir dijalankan.
- **Riwayat sumber daya** menggambar grafik CPU, RAM, Disk, Load, atau Network, selama 1 jam, 24 jam, atau 7 hari. Di bawah grafik ada nilai **Sekarang**, **Rata-rata**, dan **Tertinggi**. Untuk jaringan, grafik menampilkan laju **Masuk** dan **Keluar** serta totalnya untuk rentang waktu tersebut.
- **Perlu perhatian** mencantumkan masalah yang memerlukan tindakan Anda, yang paling parah lebih dulu.
- **Perangkat lunak terinstal** menampilkan komponen utama dan statusnya.

Panel mencatat beban setiap menit dan menyimpan riwayat 7 hari. Setelah instalasi baru, grafik muncul setelah beberapa menit. Bagian bawah menu kiri juga menampilkan pemakaian CPU dan RAM saat ini.

### Apa yang diperiksa "Perlu perhatian" {#what-needs-attention-checks}

- Masuk dua faktor nonaktif.
- Root bisa masuk lewat SSH dengan kata sandi.
- Firewall nonaktif.
- Sertifikat situs web tidak bisa diperpanjang, sudah kedaluwarsa, atau kedaluwarsa dalam 14 hari.
- Situs web kedaluwarsa dalam 7 hari.
- Pencadangan terjadwal gagal.
- Disk sudah 90% penuh atau lebih.
- Ada pembaruan keamanan sistem operasi yang menunggu (lihat [Pembaruan](./os#updates)).
- Server perlu dimulai ulang untuk menyelesaikan instalasi pembaruan.
- [Pemeriksaan keamanan](./security-check) terakhir menemukan masalah kritis atau peringatan.
- Situs web yang dipantau sedang mati (lihat [Pemantauan](./websites#monitoring)).

Ikon lonceng di kanan atas (**Notifikasi**) menampilkan daftar yang sama. Keduanya hanya terlihat selama Anda masuk. Untuk menerima pesan lewat Telegram, email, Discord, Slack, atau webhook, siapkan [peringatan](./mail-alerts).

## Tugas cron {#cron-jobs}

Tugas cron menjalankan perintah sesuai jadwal. Buka **Tugas cron** di menu kiri.

### Menambahkan tugas cron {#add-a-cron-job}

1. Pilih **Tambah tugas cron**.
2. Masukkan **Nama**, hingga 100 karakter.
3. Pilih **Jalankan sebagai**: root, atau pengguna situs web (ditampilkan bersama domainnya). Jalankan tugas situs web sebagai pengguna situs itu sendiri, dan gunakan root hanya untuk tugas server.
4. Atur **Jadwal**. Pilih jadwal umum (**Setiap 5 menit**, **Setiap jam**, **Setiap hari pukul 03:00**, **Setiap Minggu pukul 04:00**, **Saat server dijalankan**) atau ketik jadwal Anda sendiri. Di bawah kolom, panel menjelaskan jadwal itu dengan kata-kata.
5. Masukkan **Perintah**: perintah shell dalam satu baris, hingga 2000 karakter, dijalankan dengan `/bin/sh`.
6. Biarkan **Aktif** menyala, lalu pilih **Simpan**.

Untuk pengguna situs web dengan PHP, tautan **Gunakan perintah cron WordPress** mengisi cron WordPress, seperti `php8.3 /var/www/example.com/public/wp-cron.php`, setiap 5 menit.

### Sintaks jadwal {#schedule-syntax}

Jadwal punya lima kolom, dalam waktu server:

| Kolom | Nilai |
|---|---|
| menit | 0–59 |
| jam | 0–23 |
| tanggal | 1–31 |
| bulan | 1–12 |
| hari dalam minggu | 0–7 (0 dan 7 adalah Minggu) |

Di setiap kolom, gunakan `*` untuk semua nilai, daftar seperti `1,15`, rentang seperti `1-5`, atau langkah seperti `*/10`. Anda juga bisa memakai salah satu dari ini sebagai pengganti lima kolom: `@hourly`, `@daily` (atau `@midnight`), `@weekly`, `@monthly`, `@yearly` (atau `@annually`), dan `@reboot`. Panel menolak angka di luar rentang, seperti menit 60.

| Jadwal | Berjalan |
|---|---|
| `*/5 * * * *` | setiap 5 menit |
| `0 3 * * *` | setiap hari pukul 03:00 |
| `30 2 * * 1` | setiap Senin pukul 02:30 |
| `0 0 1 * *` | pada tengah malam tanggal 1 setiap bulan |

### Cara tugas cron berjalan {#how-cron-jobs-run}

- Panel menulis tugas-tugas ke `/etc/cron.d/panel`, jadi cron sistem menjalankannya bahkan saat panel berhenti. Jangan edit file itu: panel menulisnya ulang.
- Keluaran dan kesalahan perintah ditambahkan ke log tugas, `/var/log/panel-cron/<id>.log`, yang dimiliki pengguna tugas tersebut.
- `%` di perintah berfungsi sebagaimana diketik: panel meng-escape-nya untuk cron.
- Tugas berhenti berjalan jika penggunanya sudah tidak ada, misalnya setelah Anda menghapus situs web.

### Menjalankan, memeriksa, dan mengubah tugas {#run-check-and-change-a-job}

- Sakelar di kolom **Aktif** mengaktifkan atau menonaktifkan tugas. Tugas yang nonaktif tetap ada di daftar.
- **Jalankan sekarang** langsung menjalankan tugas, sebagai penggunanya, dan menampilkan keluarannya secara langsung. Proses ini berhenti setelah satu jam.
- **Log**, di menu ⋯ pada baris, menampilkan bagian akhir log tugas. Log di atas 5 MB dipangkas menjadi 1 MB terakhirnya.
- **Edit** dan **Hapus** ada di menu yang sama. Menghapus tugas juga menghapus log-nya.

Di atas daftar, tampilkan **Semua** tugas atau hanya yang **Aktif** atau **Nonaktif**, dan cari berdasarkan nama, perintah, atau pengguna.

## Layanan {#services}

**Layanan** mencantumkan layanan sistem yang dikelola Nefo, jika terinstal:

| Layanan | Unit systemd |
|---|---|
| Nefo | `nefo` |
| Nginx, Apache, atau OpenLiteSpeed | `nginx`, `apache2`, `lshttpd` |
| PHP-FPM, satu per versi yang terinstal | seperti `php8.3-fpm` |
| MariaDB atau MySQL | `mariadb` atau `mysql` |
| PostgreSQL | seperti `postgresql@16-main` |
| Redis | `redis-server` |
| Memcached | `memcached` |
| fail2ban | `fail2ban` |
| SSH | `ssh` |
| Cron | `cron` |
| Firewall (ufw) | `ufw` |

Layanan lain, seperti Docker atau Postfix, tidak dicantumkan. Setiap baris menampilkan status (**Berjalan**, **Berhenti**, **Gagal**…), berapa lama layanan sudah berjalan, "tidak berjalan saat boot" jika berlaku, dan pemakaian memorinya.

Tindakannya bergantung pada layanan:

| Layanan | Tindakan |
|---|---|
| Server web, PHP-FPM | **Jalankan**, **Hentikan**, **Mulai ulang**, **Muat ulang** |
| Nefo | **Mulai ulang** |
| SSH | **Mulai ulang**, **Muat ulang** |
| Lainnya | **Jalankan**, **Hentikan**, **Mulai ulang** |

- **Hentikan** meminta Anda mengonfirmasi, karena situs web yang membutuhkan layanan itu akan berhenti berfungsi.
- Sebelum memulai ulang atau memuat ulang server web, panel menguji konfigurasinya. Jika pengujian gagal, tidak ada yang dimulai ulang dan kesalahannya ditampilkan.
- SSH tidak bisa dihentikan dari panel, jadi Anda tidak bisa mengunci diri sendiri di luar.
- Memulai ulang Nefo memuat ulang halaman setelah beberapa detik.
- **Log** menampilkan 150 baris terakhir jurnal layanan.

Panel tidak mengubah apakah sebuah layanan berjalan saat boot.

## Program latar belakang {#background-programs}

Program latar belakang adalah perintah yang harus terus berjalan, seperti queue worker. Panel menjalankan setiap program sebagai layanan systemd, sebagai pengguna Linux milik situs web, dan menjalankannya lagi saat program berhenti. Anda membutuhkan setidaknya satu situs web terlebih dahulu.

### Menambahkan program {#add-a-program}

1. Buka **Program latar belakang** dan pilih **Tambah program**.
2. Masukkan **Nama**, hingga 60 karakter.
3. Pilih **Situs web**. Program berjalan sebagai pengguna Linux situs web ini, tidak pernah sebagai root.
4. Opsional, atur **Folder kerja**: folder di dalam folder situs web, seperti `app`. Kosongkan untuk memakai folder situs web itu sendiri.
5. Masukkan **Perintah**: satu program beserta argumennya, dalam satu baris, hingga 1000 karakter.
6. Opsional, tambahkan **Variabel lingkungan**: satu `NAME=value` per baris, hingga 20. Di server, hanya root yang bisa membacanya.
7. Biarkan **Jalankan sekarang dan saat server dijalankan** aktif, lalu pilih **Buat**.

Untuk situs web PHP, tautan **Gunakan queue worker Laravel** mengisi `php8.3 artisan queue:work --sleep=3 --tries=3 --max-time=3600`, dengan versi PHP situs web.

### Cara program latar belakang berjalan {#how-background-programs-run}

- Setiap program adalah layanan systemd bernama `panel-daemon-<id>.service`.
- Saat program keluar, karena alasan apa pun, systemd menjalankannya lagi setelah 3 detik.
- Saat boot, program dijalankan setelah MariaDB dan Redis.
- Program berjalan dengan perlindungan systemd: `/usr`, `/boot`, dan `/etc` hanya-baca, `/home` dan `/root` disembunyikan, dan program punya `/tmp` sendiri.
- Folder kerja harus ada dan tetap berada di dalam folder situs web, juga melalui tautan simbolik.
- Keluarannya masuk ke jurnal sistem.

### Mengendalikan program {#control-a-program}

Setiap baris menampilkan status (berjalan, memulai ulang, gagal, sedang berhenti, atau berhenti), berapa kali program dimulai ulang, dan pemakaian memorinya.

- Baris menampilkan **Jalankan** (saat program berhenti) atau **Mulai ulang** (saat program berjalan) sebagai tombol. Tindakan lainnya, termasuk **Hentikan**, ada di menu ⋯ baris tersebut. **Hentikan** membuat program tetap berhenti, juga setelah reboot, sampai Anda menjalankannya lagi.
- **Log** menampilkan 200 baris terakhir yang ditulis program.
- **Edit** mengubah program. **Simpan** memulai ulang program dengan pengaturan baru.
- **Hapus** menghentikan dan menghapusnya.

Saat Anda menghapus situs web, program latar belakangnya juga dihentikan dan dihapus.

## Proses dan port {#processes-and-ports}

**Proses** menampilkan apa yang berjalan di server, dalam tiga tab: **Proses**, **Port**, dan **Upaya masuk SSH**.

### Proses {#processes}

Daftar menampilkan proses tersibuk, hingga 300. Urutkan berdasarkan PID, CPU %, atau memori dengan judul kolom, dan saring berdasarkan nama, pengguna, atau PID. **Muat ulang setiap 5 detik** membuat daftar tetap mutakhir.

CPU % diukur selama beberapa detik terakhir: 100% berarti satu inti CPU bekerja terus-menerus.

Untuk menghentikan proses, pilih tombol hentikan di ujung barisnya, lalu:

- **Hentikan** meminta program menyelesaikan pekerjaannya dan keluar (sinyal TERM). Program yang dibuat dengan baik menyimpan datanya terlebih dahulu.
- **Paksa berhenti** langsung mengakhirinya (sinyal KILL). Gunakan hanya jika **Hentikan** tidak berhasil: data yang belum disimpan akan hilang.

Layanan yang dijalankan systemd mungkin dijalankan lagi oleh systemd. Panel tidak bisa menghentikan proses init sistem (PID 1), panel itu sendiri, proses yang menjalankan panel, atau thread kernel. Setiap penghentian dicatat di log aktivitas.

### Port {#ports}

**Port** mencantumkan port TCP dan UDP yang didengarkan program, beserta **Alamat**, **Program**, dan dari mana port itu **Dapat dijangkau dari**: **hanya server ini**, atau **jaringan**.

Port yang terbuka ke jaringan tetap bisa ditutup dari luar oleh firewall. Lihat [Firewall](./security#firewall-ufw).

## Upaya masuk SSH {#ssh-sign-ins}

**Proses** → **Upaya masuk SSH** mencantumkan aktivitas masuk SSH dan upaya yang gagal dalam 30 hari terakhir, yang terbaru lebih dulu, hingga 200. Setiap baris menampilkan waktu, hasil (**berhasil masuk**, **kata sandi salah**, atau **pengguna tidak dikenal**), pengguna, alamat IP, dan metodenya, seperti `publickey` atau `password`.

Tampilkan **Semua**, **Hanya yang berhasil**, atau **Hanya yang gagal**. Daftar ini dibaca dari jurnal layanan SSH.

Banyak upaya gagal biasanya berarti ada seseorang yang menebak-nebak kata sandi. Nonaktifkan masuk dengan kata sandi, atau instal fail2ban untuk memblokir alamat-alamat itu. Lihat [Keamanan](./security#brute-force-protection-fail2ban).

## Terminal {#terminal}

**Terminal** membuka shell root (bash) di server, di browser Anda.

::: warning
Terminal sama seperti sesi SSH sebagai root: setiap perintah berjalan dengan hak penuh, dan perintah yang salah bisa menghapus data atau menghentikan server. Panel tidak bisa membatalkan apa yang Anda jalankan di sana.
:::

- Sesi berakhir saat Anda meninggalkan halaman, atau setelah 30 menit tanpa masukan. Pilih **Hubungkan ulang** untuk membuka sesi baru.
- Paling banyak 4 terminal bisa terbuka sekaligus, termasuk terminal kontainer.
- Log aktivitas mencatat kapan terminal dibuka dan ditutup, dan berapa lama. Log tidak mencatat perintahnya.

## Log {#logs}

Nefo tidak punya satu halaman log tunggal. Setiap log ditampilkan di samping hal yang dimilikinya:

| Log | Di mana |
|---|---|
| Log akses, log kesalahan, kesalahan PHP, dan log aplikasi situs web | **Situs web** → (situs) → **Log** |
| Peristiwa firewall situs web | **Situs web** → (situs) → **Firewall web (WAF)** |
| Keluaran tugas cron | **Tugas cron** → **Log** |
| Jurnal layanan | **Layanan** → **Log** |
| Keluaran program latar belakang | **Program latar belakang** → **Log** |
| Kontainer dan proyek Compose | **Docker** |
| Apa yang dilakukan di panel | **Akun** → **Aktivitas terbaru** |
| Log layanan milik panel sendiri | `journalctl -u nefo` di server |

Log akses, kesalahan, dan PHP situs web menampilkan 300 baris terakhirnya. Pilih **Muat ulang** di jendela log untuk memuat baris baru.

## Memulai ulang panel atau me-reboot server {#restart-the-panel-or-reboot-the-server}

Buka **Pengaturan** → **Panel** → **Daya**. **Dasbor** punya tindakan yang sama di menu **Daya**-nya.

- **Mulai ulang panel** hanya memulai ulang panel. Situs web, basis data, dan layanan lain tetap berjalan, dan panel kembali dalam beberapa detik.
- **Reboot server** memulai ulang seluruh server. Situs web dan basis data mati sampai server kembali, biasanya satu sampai dua menit. Ketik nama server untuk konfirmasi.

Dalam kedua kasus, halaman memuat ulang dengan sendirinya saat panel merespons lagi. Keduanya ditolak selama ada tugas latar belakang yang berjalan, seperti instalasi atau pencadangan: tunggu sampai selesai. Keduanya dicatat di log aktivitas.

Di server, Anda juga bisa memulai ulang panel dengan:

```sh
systemctl restart nefo
```
