---
title: Sertifikat SSL
description: Dapatkan sertifikat Let's Encrypt gratis, sertifikat wildcard lewat DNS Cloudflare, atau instal sertifikat Anda sendiri, dan atasi kegagalan yang umum.
---

# Sertifikat SSL

Nefo mendapatkan sertifikat Let's Encrypt gratis untuk setiap situs web dengan satu klik dan memperpanjangnya sendiri. Nefo juga bisa mendapatkan sertifikat wildcard lewat DNS Cloudflare, menginstal sertifikat yang sudah Anda miliki, mengalihkan HTTP ke HTTPS, dan mengirim HSTS. Sebelum setiap permintaan, Nefo memeriksa bahwa setiap nama mengarah ke server, jadi upaya yang gagal tidak dihitung dalam batas Let's Encrypt.

Halaman ini membahas persyaratannya, pengalihan ke HTTPS dan HSTS, sertifikat wildcard lewat DNS Cloudflare, pemasangan sertifikat yang sudah Anda miliki, menonaktifkan HTTPS lagi, dan apa yang harus dilakukan jika sertifikat gagal.

## Sebelum mulai {#before-you-start}

Untuk sertifikat biasa, Let's Encrypt memeriksa lewat HTTP biasa bahwa setiap nama situs web mengarah ke server Anda. Setiap nama harus lolos: domainnya dan setiap [nama domain tambahannya](./websites#domains-and-aliases).

- **DNS:** setiap nama punya record A yang mengarah ke alamat IP server ini, misalnya `203.0.113.10`. Jika sebuah nama juga punya record AAAA (IPv6), record itu juga harus mengarah ke server ini, atau dihapus.
- **Port 80:** server menjawab di port 80 dari internet. Nefo membuka port 80 dan 443 di ufw saat menginstal server web. Jika penyedia hosting Anda punya firewall sendiri, buka port 80 di sana juga.
- **Proxy:** jika DNS Anda ada di Cloudflare dengan proxy aktif (awan oranye), baca [Proxy Cloudflare](#cloudflare-proxy-orange-cloud) lebih dulu.

Dengan mengaktifkan HTTPS, Anda menyetujui [Perjanjian Pelanggan](https://letsencrypt.org/repository/) (Subscriber Agreement) Let's Encrypt.

## Mendapatkan sertifikat gratis {#get-a-free-certificate}

1. Buka **Situs web**, lalu tab **SSL / HTTPS** milik situs web tersebut. Di daftar situs web, **Aktifkan HTTPS** di kolom **SSL** membuka tab yang sama.
2. Di bawah **Sertifikat gratis dari Let's Encrypt**, biarkan **Validasi lewat DNS (Cloudflare)** tidak dicentang.
3. Klik **Dapatkan sertifikat**.

Sebuah log menampilkan langkah-langkahnya: Nefo memeriksa bahwa setiap nama mengarah ke server ini, meminta sertifikat ke Let's Encrypt, memvalidasi setiap nama, dan mengaktifkan HTTPS di server web. Log diakhiri dengan "Done. The certificate is valid until" (Selesai. Sertifikat berlaku sampai) dan tanggalnya.

Saat sebuah situs mendapat sertifikat pertamanya, Nefo juga mengalihkan situs WordPress yang diinstalnya ke alamat `https://`-nya, dan memulai ulang aplikasi Docker dengan alamat `https://`-nya. File-filenya disimpan di `/var/lib/nefo/certs/example.com/` sebagai `fullchain.pem` dan `privkey.pem`.

### Pemeriksaan sebelum permintaan {#the-check-before-the-request}

Sebelum meminta ke Let's Encrypt, Nefo meletakkan file uji di server dan mengambilnya lewat setiap nama melalui HTTP. Jika sebuah nama gagal, Nefo sama sekali tidak mengirim permintaan, jadi upaya yang gagal tidak dihitung dalam batas Let's Encrypt. Log lalu menampilkan "these names do not reach this server yet, so no certificate was requested:" (nama-nama ini belum mengarah ke server ini, jadi tidak ada sertifikat yang diminta:) dengan satu baris per nama:

| Pesan | Yang perlu diperbaiki |
|---|---|
| `blog.example.com: no DNS record. Add an A record that points to this server's IP address.` | Tambahkan record A dan tunggu sampai record itu terlihat. |
| `example.com (203.0.113.10): no answer over HTTP on port 80 (…). Check the DNS record and the firewall.` | Buka port 80 di firewall penyedia Anda, dan periksa bahwa server web berjalan. |
| `example.com (198.51.100.7) answered HTTP 404, but not from this server. Check that the DNS record points here.` | Nama itu mengarah ke server lain: alamat IP lama, record AAAA ke tempat lain, atau proxy seperti Cloudflare. |

Alamat dalam tanda kurung adalah hasil resolusi nama tersebut. Bandingkan dengan alamat IP server Anda.

## Mengalihkan HTTP ke HTTPS {#redirect-http-to-https}

Setelah situs punya sertifikat, aktifkan **Alihkan HTTP ke HTTPS** di tab **SSL / HTTPS** atau di tab **Ringkasan**. Setiap alamat `http://` lalu menjawab dengan pengalihan permanen (301) ke `https://`. Validasi Let's Encrypt dan webhook Git tetap berfungsi lewat HTTP.

Agar browser memakai HTTPS bahkan sebelum pengalihan, aktifkan juga [HSTS](#hsts).

## HSTS {#hsts}

HSTS (header `Strict-Transport-Security`) memberi tahu browser agar membuka situs hanya lewat `https://`, bahkan saat seseorang mengetik `http://` atau mengikuti tautan lama. Aktifkan **HSTS (Strict-Transport-Security)** di tab **SSL / HTTPS**. Fitur ini membutuhkan sertifikat dan **Alihkan HTTP ke HTTPS**; menonaktifkan pengalihan juga menonaktifkan HSTS.

Nefo lalu mengirim `Strict-Transport-Security: max-age=15552000` (180 hari, sekitar 6 bulan) bersama jawaban HTTPS situs, di Nginx, Apache, dan OpenLiteSpeed. Centang **Sertakan subdomain** untuk menambahkan `includeSubDomains`: browser lalu juga memakai HTTPS untuk setiap subdomain dari domain tersebut, jadi centang hanya jika semua subdomain punya HTTPS. Nefo tidak pernah menambahkan `preload`: daftar preload browser sulit ditinggalkan, jadi daftarkan sendiri domain ke daftar itu, jika memang perlu.

::: warning
Browser yang sudah menerima header ini menolak HTTP biasa untuk situs tersebut selama 6 bulan setelah kunjungan terakhirnya, meskipun nanti Anda menonaktifkan HSTS atau HTTPS. Aktifkan HSTS hanya untuk situs yang akan terus memakai HTTPS.
:::

Di Nginx, header diatur untuk seluruh blok `server` situs. Jika direktif Anda sendiri (lihat [Situs web](./websites#rewrite-rules-and-your-own-directives)) menambahkan `location` dengan `add_header` miliknya sendiri, Nginx membuang header situs di location tersebut: ulangi `add_header Strict-Transport-Security "max-age=15552000" always;` di sana.

Untuk memeriksanya, jalankan `curl -sI https://example.com/ | grep -i strict-transport`.

## Perpanjangan otomatis {#automatic-renewal}

Nefo memeriksa sertifikat saat dijalankan, lalu setiap jam. Nefo memperpanjang sertifikat Let's Encrypt saat masa berlakunya tinggal sepertiga: 30 hari sebelum berakhir untuk sertifikat 90 hari. Anda tidak perlu melakukan apa pun, dan situs web yang ditangguhkan tetap diperpanjang.

Jika perpanjangan gagal, Nefo mencoba lagi setiap 12 jam. Sementara itu:

- daftar situs web menampilkan "Perpanjangan gagal" di kolom **SSL**;
- tab **SSL / HTTPS** menampilkan "Upaya terakhir mendapatkan sertifikat gagal." beserta kesalahannya;
- peringatan dikirim jika Anda menyiapkan peringatan untuk perpanjangan yang gagal: lihat [Email dan peringatan](./mail-alerts).

Sertifikat milik panel sendiri, dari **Pengaturan** → **Sertifikat** (lihat [Keamanan](./security#https-from-the-first-start)), diperpanjang dengan cara yang sama: saat masa berlakunya tinggal sepertiga, dan lagi setiap 12 jam setelah kegagalan. Perpanjangan yang gagal dicatat di log aktivitas dan mengirim peringatan yang sama.

Nefo mendaftarkan akun Let's Encrypt-nya tanpa alamat email, jadi Let's Encrypt tidak mengirimi Anda email tentang sertifikat yang akan kedaluwarsa. Andalkan peringatan panel sebagai gantinya.

Untuk memperpanjang secara manual, misalnya setelah Anda memperbaiki masalah, klik **Perpanjang sertifikat** di tab **SSL / HTTPS**.

## Sertifikat wildcard lewat DNS Cloudflare {#wildcard-certificates-through-cloudflare-dns}

Nama wildcard seperti `*.example.com` hanya bisa divalidasi lewat DNS. Nefo melakukannya lewat API Cloudflare, jadi DNS domain harus di-hosting di Cloudflare. Validasi DNS juga berfungsi untuk situs yang belum bisa dijangkau lewat HTTP, misalnya di balik proxy Cloudflare. Untuk sertifikat, Nefo hanya menambah dan menghapus record TXT `_acme-challenge`.

### Membuat token API Cloudflare {#create-a-cloudflare-api-token}

Token yang sama juga dipakai [halaman DNS](./dns). Izinnya dicantumkan di satu tempat, di bawah [Token API](./dns#the-api-token): sertifikat hanya butuh Zone → DNS → Edit; halaman DNS juga bisa memakai Zone → Zone → Read dan Zone → Cache Purge → Purge.

1. Di dasbor Cloudflare, buka **My Profile → API Tokens → Create Token**.
2. Gunakan template **Edit zone DNS**. Template ini memberi izin Zone → DNS → Edit.
3. Di bawah sumber daya zona (zone resources), sertakan zona (domain) yang Anda pakai di server ini.
4. Buat token dan salin.

Lihat panduan Cloudflare untuk [membuat token API](https://developers.cloudflare.com/fundamentals/api/get-started/create-token/).

### Menyimpan token di Nefo {#save-the-token-in-nefo}

1. Buka **Pengaturan**, lalu tab **Cloudflare**.
2. Tempelkan token ke **Token API** dan klik **Periksa dan simpan**.

Nefo memeriksa bahwa Cloudflare menerima token itu dan token itu aktif, lalu menampilkan "Token disimpan" dengan karakter terakhir token dan **Zona** yang bisa dipakainya. Gunakan **Ganti token API** untuk menggantinya, atau **Hapus** untuk menghapusnya.

### Mendapatkan sertifikat {#get-the-certificate}

1. Untuk wildcard, buka tab **Domain** situs web, tambahkan `*.example.com` ke **Nama domain tambahan**, lalu klik **Simpan**.
2. Di tab **SSL / HTTPS**, centang **Validasi lewat DNS (Cloudflare)**. Untuk situs dengan nama wildcard, opsi ini sudah dicentang dan tidak bisa diubah: "Wajib: situs ini memakai nama wildcard (*.domain)."
3. Klik **Dapatkan sertifikat**.

Nefo menambahkan record TXT `_acme-challenge.example.com`, menunggu hingga 3 menit sampai name server Cloudflare menjawab dengan record itu, memvalidasi, lalu menghapus record tersebut. Situs mengingat pilihan ini, jadi perpanjangannya juga memakai validasi DNS.

Sertifikat mencakup nama-namanya; pengunjung tetap membutuhkan record DNS yang mengarahkan subdomain ke server ini.

## Memakai sertifikat Anda sendiri {#use-your-own-certificate}

Instal sertifikat Anda sendiri jika Anda membelinya dari penyedia lain, atau untuk sertifikat origin Cloudflare.

1. Buka tab **SSL / HTTPS** situs web dan pergi ke **Sertifikat Anda sendiri**.
2. Tempelkan **Sertifikat (PEM, dengan intermediate)**: sertifikat diikuti rantainya. Anda juga bisa memilih file `.pem`, `.crt`, `.cer`, atau `.txt`.
3. Tempelkan **Kunci privat (PEM, tidak terenkripsi)**, atau pilih file `.pem`, `.key`, atau `.txt`.
4. Klik **Instal**.

Nefo memeriksa file-file itu sebelum mengubah apa pun, dan menolak:

- file yang bukan sertifikat PEM, atau lebih besar dari 64 KB;
- kunci privat yang dilindungi passphrase. Hapus passphrase-nya lebih dulu, misalnya dengan `openssl pkey -in key.pem -out plain.pem`;
- kunci yang bukan milik sertifikat tersebut;
- sertifikat yang belum berlaku, atau sudah kedaluwarsa;
- sertifikat yang tidak mencakup setiap nama situs. Nama wildcard situs harus tercantum persis di sertifikat.

Nefo menginstal sertifikat dengan peringatan ("Terinstal, dengan peringatan:") jika browser tidak akan memercayainya dengan sendirinya, yang wajar untuk sertifikat origin Cloudflare yang dipakai di balik proxy Cloudflare, atau jika sertifikat kedaluwarsa dalam dua minggu.

Nefo tidak memperpanjang sertifikat Anda sendiri. Dalam dua minggu terakhirnya, tab **SSL / HTTPS** menampilkan "Nefo tidak memperpanjang sertifikat yang Anda instal. Instal yang baru sebelum" dan tanggalnya, dan **Perlu perhatian** di dasbor mencantumkan situs tersebut. Nefo juga mengingatkan Anda sekali sehari: log aktivitas mencatatnya sebagai perpanjangan yang gagal, dan peringatan untuk perpanjangan yang gagal dikirim. Instal sertifikat baru tepat waktu, atau beralih kembali ke Let's Encrypt dengan **Dapatkan sertifikat**. Daftar situs web menandai situs-situs ini dengan "sert. sendiri".

## Menonaktifkan HTTPS {#turn-off-https}

Untuk mengembalikan situs web ke HTTP biasa, klik **Nonaktifkan HTTPS** di bagian bawah tab **SSL / HTTPS** situs tersebut dan konfirmasi. Nefo lalu:

- menyajikan situs web hanya lewat `http://`: bagian HTTPS dari konfigurasi server webnya dihapus;
- menonaktifkan **Alihkan HTTP ke HTTPS** dan [HSTS](#hsts);
- menghapus file sertifikat di `/var/lib/nefo/certs/example.com/`, baik dari Let's Encrypt maupun milik Anda sendiri, jadi tidak ada yang diperpanjang atau diingatkan lagi. Akun Let's Encrypt tetap ada untuk situs web lain;
- mengalihkan situs WordPress yang diinstalnya kembali ke alamat `http://`-nya, dan memulai ulang aplikasi Docker dengan alamat `http://`-nya.

Pengunjung yang membuka alamat `https://` situs tersebut (dari bookmark, hasil pencarian, atau tautan) lalu mendapat kesalahan browser, karena server tidak lagi punya sertifikat untuk nama itu. Jika situs memakai HSTS, browser yang sudah menerima header-nya tetap menolak HTTP biasa hingga 6 bulan, jadi bagi mereka situs sama sekali tidak terbuka sampai punya HTTPS lagi. Anda bisa mendapatkan sertifikat baru kapan saja dengan **Dapatkan sertifikat**.

Untuk mengganti sertifikat, Anda tidak perlu menonaktifkan HTTPS: dapatkan sertifikat Let's Encrypt baru atau instal sertifikat Anda sendiri di atasnya. Menghapus situs web juga menghapus sertifikatnya: lihat [Situs web](./websites). Mencopot server web tetap menyimpan sertifikat.

## Jika sertifikat gagal {#when-a-certificate-fails}

Kesalahan muncul di tab **SSL / HTTPS** situs web di bawah "Upaya terakhir mendapatkan sertifikat gagal." dan di log upaya tersebut.

### DNS atau port 80 {#dns-or-port-80}

Lihat [pemeriksaan sebelum permintaan](#the-check-before-the-request). Sebagian besar kegagalan disebabkan record DNS yang belum mengarah ke server ini, atau port 80 yang tertutup di firewall penyedia.

### Proxy Cloudflare (awan oranye) {#cloudflare-proxy-orange-cloud}

Dengan proxy Cloudflare aktif, domain di-resolve ke alamat Cloudflare, dan permintaan sampai ke Cloudflare sebelum ke server Anda. Pengaturan Cloudflare seperti **Always Use HTTPS** (pengalihan ke HTTPS, di bawah **SSL/TLS → Edge Certificates**) lalu bisa membuat pemeriksaan tidak sampai ke server dalam keadaan utuh, dan Nefo melaporkan bahwa nama itu "answered HTTP …, but not from this server" (menjawab HTTP …, tetapi bukan dari server ini). Lihat juga [Let's Encrypt di balik awan oranye](./dns#let-s-encrypt-behind-the-orange-cloud). Ada tiga jalan keluar:

- centang **Validasi lewat DNS (Cloudflare)** dan dapatkan sertifikat lewat DNS. Proxy boleh tetap aktif, dan perpanjangan berjalan dengan cara yang sama;
- instal sertifikat origin Cloudflare sebagai [sertifikat Anda sendiri](#use-your-own-certificate);
- nonaktifkan proxy (DNS only, awan abu-abu) untuk nama tersebut. Perpanjangan juga memeriksa lewat HTTP, jadi biarkan proxy tetap nonaktif.

### Record CAA {#caa-records}

Jika domain Anda punya record CAA di DNS-nya, salah satunya harus mengizinkan `letsencrypt.org`. Jika tidak, Let's Encrypt menolak sertifikat, dan log menampilkan kesalahannya, yang menyebut CAA. Lihat [Let's Encrypt dan CAA](https://letsencrypt.org/docs/caa/).

### Batas laju {#rate-limits}

Let's Encrypt membatasi berapa banyak sertifikat dan validasi gagal yang boleh dimiliki satu domain dan satu akun. Saat Anda mencapai batas, log menampilkan pesan Let's Encrypt beserta waktu tunggunya. Lihat [batas laju Let's Encrypt](https://letsencrypt.org/docs/rate-limits/).

::: tip
Untuk menguji tanpa menghabiskan batas, tambahkan `"acme_directory": "https://acme-staging-v02.api.letsencrypt.org/directory"` ke `/etc/nefo/config.json` dan mulai ulang panel dengan `systemctl restart nefo`. Sertifikat staging tidak dipercaya browser. Hapus baris itu dan mulai ulang lagi untuk mendapatkan sertifikat sungguhan.
:::

### Validasi DNS lewat Cloudflare {#dns-validation-through-cloudflare}

| Pesan | Yang perlu diperbaiki |
|---|---|
| `DNS validation needs a Cloudflare API token: add one in Settings → Cloudflare` | Simpan token di halaman **Pengaturan**, tab **Cloudflare**. |
| `Cloudflare did not accept this token.` | Token salah, kedaluwarsa, atau tidak aktif. Buat token baru. |
| `none of the Cloudflare zones this API token can use holds example.com` | Tambahkan zona domain ke sumber daya zona (zone resources) token di Cloudflare. |
| `the DNS record _acme-challenge.example.com did not appear on … in time` | Name server Cloudflare lambat. Coba lagi beberapa menit kemudian. |
