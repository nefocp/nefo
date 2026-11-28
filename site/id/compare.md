---
title: Nefo dan panel lain
description: Apa yang membedakan Nefo dari panel kontrol yang dipakai sebagian besar pemilik VPS, dan kapan panel lain menjadi pilihan yang lebih baik.
---

# Nefo dan panel lain

Nefo berbeda dari sebagian besar panel kontrol hosting dalam beberapa hal: semua fiturnya gratis, tanpa kunci lisensi atau akun vendor; panelnya sendiri memakai memori sekitar 34 MB; halaman masuknya berada di port acak di balik path rahasia; dan Nefo sengaja tidak menyertakan server email, DNS, dan FTP serta akun reseller.

Halaman ini membandingkan Nefo dengan panel kontrol hosting yang paling sering diinstal orang di VPS. Halaman ini tidak menyebut nama panel-panel itu. Panel-panel tersebut berbeda satu sama lain, jadi kolom kanan menyebutkan berapa banyak di antaranya yang melakukan sesuatu: sebagian besar, banyak, sebagian, atau sedikit.

Perbandingan ini diperiksa terhadap situs web resmi panel-panel tersebut pada November 2026. Panel mengubah harga dan fiturnya, jadi periksa panel yang sedang Anda pertimbangkan sebelum memutuskan.

## Sekilas {#at-a-glance}

| | Nefo | Panel lain |
|---|---|---|
| Harga | Gratis. Semua fitur, tanpa edisi berbayar | Sebagian besar punya sesuatu yang harus dibayar: lisensi per server, biasanya dihargai menurut jumlah akun atau situs, atau edisi berbayar atau add-on |
| Kunci lisensi atau akun vendor | Tidak ada | Sebagian membutuhkannya sebelum bisa berjalan, dan banyak yang membutuhkannya untuk fitur berbayar |
| Data yang dikirim ke vendor | Tidak ada: tanpa telemetri, tanpa laporan instalasi, tanpa pemeriksaan lisensi | Bervariasi. Sebagian mengumpulkan statistik penggunaan secara bawaan, dan sebagian penginstal melaporkan server ke vendor |
| Memori | Panelnya sendiri memakai sekitar 34 MB (diukur di server uji yang menjalankan v0.24.0). Dibuat untuk VPS dengan RAM 1 GB | Sebagian besar meminta setidaknya 1 GB, dan banyak yang menyarankan 4 GB |
| Halaman masuk | Port acak dan path rahasia. Setiap alamat lain mengembalikan `404` | Sebagian besar memakai port tetap yang sudah dikenal luas. Sedikit yang memakai port acak atau path rahasia |
| Pembaruan panel | Ditandatangani dengan Ed25519 dan diperiksa sebelum diinstal. Hanya saat Anda memintanya | Sebagian mendokumentasikan paket yang ditandatangani; yang lain tidak menyebutkannya |
| Pengaman agar tidak terkunci | Menolak aturan firewall dan pengaturan SSH yang akan mengunci Anda di luar | Sedikit yang punya pengaman untuk firewall, seperti rollback berwaktu. Pengaman untuk pengaturan SSH jarang ada |
| Kode sumber | Seluruhnya, di bawah AGPLv3 | Sebagian besar tertutup atau hanya source-available. Sebagian bersumber terbuka |
| Server email dan DNS | Tidak disertakan, memang disengaja | Sebagian besar menginstal keduanya |
| Beberapa server dari satu panel | Tidak. Satu panel per server | Sedikit yang mengelola beberapa server dari satu panel, meskipun banyak yang bisa berbagi zona DNS antarserver |
| Akun reseller dan pelanggan | Tidak. Satu akun administrator | Sebagian besar punya akun reseller, dan hampir semua punya login untuk pelanggan atau pengguna |
| Sistem operasi | Ubuntu dan Debian, di x86-64 dan ARM64 | Sebagian besar hanya untuk Linux, dan sebagian besar berjalan di lebih banyak distribusi Linux daripada dua ini. Sedikit yang berjalan di Windows |
| Usia | Rilis publik pertama pada November 2026. Beta | Banyak yang sudah ada selama 20 tahun atau lebih |

## Apa yang dilakukan Nefo secara berbeda {#what-nefo-does-differently}

### Tetap kecil {#it-stays-small}

- Nefo adalah satu program, `/usr/local/bin/nefo`, sekitar 25 MB, dengan antarmuka web di dalamnya. Di server uji yang menjalankan v0.24.0, Nefo memakai memori sekitar 34 MB. File antarmuka dikirim dalam bentuk terkompresi gzip, dan masing-masing dari tujuh bahasanya baru diunduh saat dipakai.
- Nefo tidak menginstal apa pun selain itu sampai Anda memilihnya. Saat pertama kali masuk, halaman penyiapan menanyakan server web, versi PHP, server basis data, dan tambahan yang Anda inginkan, serta menampilkan perkiraan memori dan disk yang ditambahkan oleh pilihan Anda. `nefo setup` melakukan hal yang sama di baris perintah.
- Setiap komponen bisa dicopot lagi, dan menu menyembunyikan apa yang belum terinstal. Nefo menolak menghapus komponen yang masih dipakai situs web, basis data, atau kontainer.
- Tidak ada server email, DNS, atau FTP yang berjalan di latar belakang, karena Nefo tidak menyertakannya.

### Semua fitur gratis, dan tidak ada yang mengirim data ke luar {#every-feature-is-free-and-nothing-phones-home}

- Tidak ada edisi berbayar, tidak ada akun yang harus dibuat, dan tidak ada kunci lisensi.
- Tidak ada telemetri dan tidak ada statistik penggunaan. Atas inisiatifnya sendiri, panel hanya menghubungi Let's Encrypt, untuk menerbitkan dan memperpanjang sertifikat, dan rilis GitHub milik proyek sekali sehari, untuk katalog toko aplikasi yang ditandatangani. Lihat [Privasi](./docs/#privacy).
- Nefo tidak pernah memperbarui dirinya sendiri, dan hanya memeriksa versi baru saat Anda memintanya.

### Halaman masuk sulit ditemukan {#the-sign-in-is-hard-to-find}

- Penginstal memilih port acak dari 20000 sampai 49999 dan path rahasia berisi 12 huruf dan angka. Setiap alamat lain di port itu mengembalikan `404`.
- Panel memakai HTTPS sejak pertama kali dijalankan. Proses masuk mendukung autentikasi dua faktor, dan 5 percobaan gagal dalam 15 menit memblokir sebuah alamat selama 15 menit. Setiap proses masuk dan perubahan dicatat di log aktivitas.
- Kunci API untuk skrip bisa dibuat hanya-baca, dibatasi ke alamat yang Anda cantumkan, dan diatur agar kedaluwarsa. Bahkan kunci dengan akses penuh tidak bisa membuka terminal, alat basis data, atau pengaturan akun.

Lihat [Keamanan](./docs/security) dan [Kunci API](./docs/api).

### Menolak perubahan yang akan mengunci Anda di luar atau merusak sesuatu {#it-refuses-changes-that-would-lock-you-out-or-break-things}

- **Firewall:** aturan terakhir yang mengizinkan alamat Anda sendiri menjangkau SSH atau panel tidak bisa dihapus.
- **SSH:** pengaturan yang membuat tidak ada yang bisa masuk ditolak, seperti menonaktifkan masuk dengan kata sandi selagi tidak ada akun yang punya kunci.
- **Perangkat lunak:** server web, versi PHP, atau server basis data yang masih dipakai tidak bisa dicopot.
- **Docker:** proyek Compose diperiksa sebelum disimpan atau dijalankan. Kontainer privileged, port yang terbuka ke internet, dan mount folder sistem ditolak.

### Memeriksa servernya sendiri {#it-checks-its-own-server}

[Pemeriksaan keamanan](./docs/security-check) menjalankan 22 pemeriksaan cepat atas server, sesuai permintaan atau sekali sehari, misalnya masuk SSH dengan kata sandi, firewall, pembaruan keamanan yang menunggu, basis data yang terbuka ke internet, port Docker yang melewati ufw, dan file yang tidak sengaja disajikan sebuah situs web, lalu menautkan setiap temuan ke halaman untuk memperbaikinya. Pemeriksaan ini tidak mengirim apa pun ke luar server.

### Cadangan tetap ada meski servernya hilang {#backups-survive-the-server}

Cadangan disimpan di server itu sendiri dan di penyimpanan yang kompatibel dengan S3, server SFTP, atau Google Drive, dan bisa dienkripsi dengan passphrase jika Anda mau. Server baru menemukannya di penyimpanan dan memulihkan situs web beserta basis datanya. Lihat [Memulihkan di server baru](./docs/restore-new-server).

### Rilis ditandatangani {#releases-are-signed}

Daftar checksum di setiap rilis ditandatangani dengan kunci Ed25519 milik proyek. Penginstal dan `nefo update` memeriksa tanda tangan dan checksum sebelum menginstal apa pun. [Kebijakan keamanan](./security-policy#check-a-release-yourself) menunjukkan cara memeriksa sendiri sebuah rilis.

### Anda bisa memindahkan situs web ke Nefo {#you-can-move-websites-in}

Nefo mengimpor situs web dari cadangan panel lain: file, basis data beserta pengguna dan kata sandinya, sertifikat, dan tugas cron. Lihat [Mengimpor situs web](./docs/import) untuk format yang bisa dibacanya.

## Kapan panel lain lebih cocok untuk Anda {#when-another-panel-fits-you-better}

Nefo sengaja tidak menyertakan beberapa hal (lihat [peta jalan](./roadmap#not-planned-and-why) untuk alasannya). Pilih panel lain jika Anda membutuhkan salah satu dari ini:

- **Email di server yang sama.** Nefo tidak punya server email: tanpa kotak surat, tanpa webmail, tanpa filter spam. Situs web dan peringatan mengirim email lewat relay SMTP penyedia Anda. Sebagian besar panel lain menyertakan server email.
- **Server DNS sendiri.** Nefo tidak menjalankan name server. Nefo mengelola record DNS domain Anda di Cloudflare.
- **FTP.** Nefo tidak punya server FTP. Setiap situs web bisa punya akun SFTP, yang terkunci di folder situs web.
- **Banyak server dari satu tempat.** Setiap server menjalankan Nefo-nya sendiri, dan Anda masuk ke masing-masing secara terpisah.
- **Menjual hosting.** Nefo punya satu akun administrator per server. Nefo tidak punya reseller, login pelanggan, paket hosting, atau kuota.
- **Rekam jejak yang panjang, atau audit keamanan dari pihak luar.** Rilis publik pertama Nefo adalah pada November 2026. Nefo masih beta, dan belum ada audit keamanan dari pihak luar.
- **Windows, atau Linux selain Ubuntu dan Debian.** Nefo hanya berjalan di Ubuntu 22.04, 24.04, dan 26.04 serta Debian 12 dan 13.

Jika tidak satu pun dari hal ini penting bagi Anda, [instal Nefo](./docs/install) di server yang bisa Anda bangun ulang, dan simpan salinan cadangan Anda di luar server.
