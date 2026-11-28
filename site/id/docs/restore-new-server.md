---
title: Memulihkan di server baru
description: Temukan cadangan di penyimpanan S3, SFTP, atau Google Drive Anda dari server Nefo baru, pulihkan situs web yang belum ada beserta basis datanya, dan dekripsi salinan cadangan secara manual.
---

# Memulihkan di server baru

Jika sebuah server hilang, server baru dengan Nefo bisa menemukan cadangan di penyimpanan jarak jauh Anda (kompatibel dengan S3, SFTP, atau Google Drive) lewat **Cari cadangan**, lalu memulihkannya. Situs web yang belum ada di server baru dibuat lagi dari cadangannya, dengan domain, alias, versi PHP, file, dan basis datanya. Ini berfungsi sejak v0.24.0.

Halaman ini membahas cara menemukan cadangan, apa yang dibuat dan tidak dibuat oleh pemulihan, jenis cadangan lainnya, dan cara mendekripsi salinan secara manual. Untuk pencadangan secara umum, lihat [Pencadangan](./backups).

## Yang Anda butuhkan {#what-you-need}

- detail akses penyimpanan: bucket dan kuncinya, akun SFTP, atau Google OAuth client dari proyek Google Cloud yang sama;
- passphrase, jika salinannya terenkripsi;
- server dengan Nefo, server web, versi PHP yang dipakai situs web Anda, dan MariaDB atau MySQL (lihat [Langkah pertama](./first-steps#set-up-the-server)).

## Bagaimana cara memulihkan situs web di server baru? {#restore-on-another-server}

1. Instal Nefo di server baru, lalu server web, PHP, dan MariaDB (lihat [Langkah pertama](./first-steps)).
2. Buka **Pencadangan** → **Penyimpanan jarak jauh** dan siapkan penyimpanan yang sama: bucket dan folder yang sama, server dan folder SFTP yang sama, atau folder Google Drive yang sama. Untuk Google Drive, gunakan proyek Google Cloud yang sama (lihat [Google Drive](./backups#google-drive)). Mencentang **Enkripsi cadangan sebelum diunggah** dengan passphrase lama bersifat opsional: Anda juga bisa memasukkan passphrase di langkah 4.
3. Pilih **Cari cadangan** di baris penyimpanan. Panel menampilkan file cadangan di penyimpanan yang belum ada di daftar cadangannya, dan membaca bagian awal masing-masing (`manifest.json`-nya): apa yang dicadangkan, kapan dibuat, ukurannya, versi PHP, dan basis datanya. Sisanya tidak diunduh.
4. Jika ada cadangan terenkripsi yang tidak terbuka dengan passphrase penyimpanan, masukkan passphrase yang dipakai saat cadangan itu diunggah lalu pilih **Baca**.
5. Pilih **Tambahkan ke daftar** untuk satu cadangan, atau **Tambahkan semua ke daftar (…)**, dengan jumlah cadangan yang berhasil dibaca panel. Tidak ada yang berubah di server: cadangan itu kini ada di daftar **Pencadangan**, dan hanya tersimpan di penyimpanan.
6. Pulihkan masing-masing dengan **Pulihkan**, seperti di [Memulihkan cadangan](./backups#restore-a-backup). **Pulihkan** di daftar hasil pencarian menjalankan langkah 5 dan 6 sekaligus.

Cadangan yang tidak bisa dibaca panel tetap ada di daftar beserta alasannya, seperti passphrase yang salah, file yang rusak, atau manifes yang menyebut situs web lain daripada folder tempatnya berada. **Cari lagi** membaca penyimpanan sekali lagi.

Cadangan yang ditemukan di penyimpanan tidak punya checksum yang tercatat di daftar. Pemulihan pertamanya membaca seluruh arsip sekali sebelum mengubah apa pun, jadi file yang rusak ditolak, lalu mencatat checksum-nya saat itu.

## Memulihkan situs web yang tidak ada di sini {#restoring-a-website-that-doesn-t-exist-here}

Jika situs web dari sebuah cadangan tidak ada di server ini, dialog pemulihan memberi tahu hal itu, dan pemulihan membuatnya:

- dengan domain dan alias yang sama. Jika sebuah alias sudah dipakai situs web lain, situs web dibuat tanpa aliasnya;
- dengan versi PHP yang sama, atau versi terbaru yang terinstal di sini, dengan peringatan di log;
- ditandai sebagai WordPress (atau aplikasi lainnya) dengan aturan rewrite dan direktif server web miliknya sendiri, jika cadangan menyimpannya;
- dengan file-filenya, dimiliki oleh pengguna situs web yang baru;
- dengan basis data dan penggunanya. Cadangan yang dibuat oleh v0.24.0 dan sesudahnya menyimpan kata sandi setiap pengguna basis data, jadi konfigurasi situs web itu sendiri (`wp-config.php` dan sejenisnya) tetap berfungsi. Dengan cadangan yang lebih lama, pengguna mendapat kata sandi baru: WordPress mendapatkannya di `wp-config.php` secara otomatis; untuk perangkat lunak lain, salin dari **Basis data**.

Tidak ada di cadangan, jadi tidak dibuat lagi: sertifikat SSL (dapatkan yang baru di tab **SSL / HTTPS** situs web setelah DNS-nya mengarah ke server baru), tugas cron, akun SFTP, program latar belakang, pengalihan, proteksi kata sandi, dan pengaturan situs web lainnya.

Cadangan yang dibuat oleh Nefo 0.24.1 atau sesudahnya mencatat jenis setiap situs web: situs web aplikasi (Node.js, Python, Java) dibuat lagi dengan runtime, perintah mulai, port, dan batas memorinya, dan situs web reverse proxy dengan alamat dan opsinya. Instal dulu runtime aplikasinya di halaman **Perangkat lunak**; jika belum ada, pemulihan berhenti dan memberi tahu Anda. Cadangan yang dibuat oleh versi yang lebih lama hanya mencatat versi PHP, dan situs web seperti itu dibuat lagi sebagai situs web statis: buat dulu situs web itu dengan domain dan jenis yang sama (lihat [Situs web](./websites#create-a-website)), lalu pulihkan, dan pemulihan hanya mengganti file dan basis datanya.

Pemulihan tidak pernah mengganti situs web tanpa konfirmasi: Anda mengetik domainnya di dialog, baik situs web itu ada maupun tidak. Memulihkan di atas situs web yang sudah ada mengganti file dan basis datanya, seperti biasa.

::: tip
Cadangan situs web menyimpan kata sandi pengguna basis datanya, di samping file dan dump. Perlakukan cadangan sesensitif situs web itu sendiri. Mengenkripsi salinan jarak jauh membuatnya tidak terbaca di penyimpanan.
:::

## Jenis cadangan lainnya {#other-kinds-of-backups}

| Jenis | Di server baru |
|---|---|
| **Basis data** | Dibuat lagi jika belum ada, dengan kata sandi penggunanya dari cadangan jika cadangan menyimpannya |
| **Folder** | Dibuat lagi jika belum ada |
| **Aplikasi** | Instal dulu aplikasinya dari Toko aplikasi (image-nya diunduh lagi), lalu pulihkan |

## Mendekripsi salinan secara manual {#decrypt-a-copy-by-hand}

Anda juga bisa mengunduh sendiri file cadangan dari penyimpanan: dari konsol S3, dengan klien SFTP, atau dari Google Drive di browser. Salinan terenkripsi (`.tar.gz.enc`) hanya membutuhkan passphrase-nya. Dekripsi dengan program `nefo`, di server Linux mana pun yang sudah menginstalnya:

```sh
nefo decrypt-backup example.com_20261126-030000.tar.gz.enc
```

Program ini menanyakan passphrase lalu menulis `example.com_20261126-030000.tar.gz` di sebelahnya. Lihat [Baris perintah](./cli#nefo-decrypt-backup).

Arsip itu berisi file situs web di bawah `files/` dan dump basis data di bawah `databases/`. Ekstrak dengan `tar xzf`.

## Format salinan terenkripsi {#format-of-encrypted-copies}

Sebagai referensi, dan untuk mendekripsi salinan tanpa Nefo. Semua angka berformat big-endian.

| Offset | Ukuran | Isi |
|---|---|---|
| 0 | 7 | `NEFOENC` (ASCII) |
| 7 | 1 | Versi format: 1 |
| 8 | 1 | Penurunan kunci: 1 = scrypt |
| 9 | 1 | Biaya scrypt, log2(N): 16 |
| 10 | 1 | scrypt r: 8 |
| 11 | 1 | scrypt p: 1 |
| 12 | 1 | Cipher: 1 = AES-256-GCM |
| 13 | 1 | log2(ukuran potongan): 16, untuk 64 KiB |
| 14 | 2 | Nol |
| 16 | 32 | Salt, acak untuk setiap file |
| 48 | 12 | Pemeriksa kunci |
| 60 | 4 | CRC-32 (IEEE) dari byte 0 sampai 59 |

- scrypt(passphrase, salt, N, r, p) menghasilkan 44 byte. Byte 0–31 adalah kunci AES-256; byte 32–43 harus sama dengan pemeriksa kunci, jika tidak, passphrase-nya salah.
- Setelah header 64 byte datang potongan-potongannya. Arsip dipotong menjadi bagian sebesar ukuran potongan. Bagian terakhir lebih pendek, mungkin kosong, jadi selalu ada ukuran ÷ potongan + 1 bagian (dibulatkan ke bawah, ditambah satu).
- Setiap bagian disegel dengan AES-256-GCM, dengan header 64 byte sebagai data tambahan, dan nonce: nomor bagian mulai dari 0 (8 byte), 3 byte nol, lalu `01` untuk bagian terakhir dan `00` untuk yang lain. Bagian disimpan sebagai ciphertext diikuti tag 16 byte.
- Ukuran file adalah 64 + ukuran + 16 × (ukuran ÷ 65536 + 1).
