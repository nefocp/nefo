---
title: Kunci API
description: Otomatiskan panel dengan kunci API, untuk deploy dari CI, pencadangan, dan pemantauan, tanpa sesi browser. Apa yang bisa dan tidak bisa dilakukan sebuah kunci, cara memanggil API, contoh, dan kesalahan.
---

# Kunci API

Kunci API memungkinkan sebuah skrip memakai panel Nefo tanpa browser, misalnya untuk men-deploy situs web dari CI, memulai pencadangan dari tugas cron di server lain, atau membaca keadaan situs web dari alat pemantauan. Sebuah kunci bersifat hanya baca atau punya akses penuh, bisa dibatasi ke alamat yang Anda cantumkan, dan bisa kedaluwarsa.

Skrip mengirim kunci bersama setiap permintaan, sebagai token Bearer, ke alamat dan path rahasia yang sama dengan panel.

Kunci ada di **Pengaturan** → **Kunci API**.

## Apa yang bisa dilakukan sebuah kunci {#what-a-key-can-do}

Setiap kunci punya salah satu dari dua jenis akses:

- **Hanya baca**: hanya permintaan `GET` dan `HEAD`. Kunci ini bisa menampilkan daftar situs web, basis data, dan cadangan, membaca log tugas, statistik, dan keadaan server, tetapi tidak bisa mengubah apa pun. Kunci ini juga tidak bisa membaca hal yang akan memberinya kuasa lebih: kata sandi basis data dan aplikasi, ekspor basis data, file (pengelola file), unduhan cadangan, nilai Redis, dan rahasia webhook Git. Permintaan itu, dan permintaan apa pun selain `GET` atau `HEAD`, mendapat `403` dengan `api_key_read_only`.
- **Akses penuh**: setiap permintaan yang dibuat oleh halaman-halaman panel sendiri, atas nama Anda.

Bahkan kunci dengan akses penuh tidak bisa:

- membuat, menampilkan daftar, atau mencabut kunci API;
- mengganti kata sandi atau masuk dua faktor, atau keluar (semua yang ada di bawah `/api/auth/`);
- membuka terminal web atau terminal sebuah kontainer;
- memakai alat basis data di balik halaman masuk panel, Adminer dan phpMyAdmin.

Permintaan ini dijawab `403` dengan `api_key_forbidden`: masuklah ke panel untuk melakukannya. Sebuah kunci juga tidak bisa masuk untuk mendapatkan sesi.

::: warning
Kunci dengan akses penuh sama kuatnya dengan Anda: kunci itu bisa menulis file, menambahkan tugas cron yang berjalan sebagai root, dan menginstal perangkat lunak. Jaga kerahasiaannya seperti kata sandi root. Kunci hanya baca juga rahasia: kunci itu bisa membaca pengaturan, log, dan daftar situs web Anda dan server.
:::

## Membuat kunci {#create-a-key}

1. Buka **Pengaturan** → **Kunci API** dan pilih **Kunci baru**.
2. Masukkan **Nama**, seperti `deploy-bot`: hingga 32 huruf, angka, spasi, titik, tanda hubung, dan garis bawah. Nama ini tampil di log aktivitas.
3. Pilih **Hanya baca** atau **Akses penuh**.
4. Opsional: di bawah **Alamat yang diizinkan (opsional)**, masukkan alamat IP atau jaringan tempat kunci boleh dipakai, seperti `203.0.113.7` atau `198.51.100.0/24`, IPv4 atau IPv6, dipisahkan spasi atau baris baru. Kosongkan untuk mengizinkan alamat mana pun.
5. Pilih kapan kunci **Kedaluwarsa**: setelah 30, 90, atau 365 hari, atau **Tidak pernah**.
6. Pilih **Kunci baru**.

Panel menampilkan kunci sekali saja, dengan tombol **Salin**: salin sekarang, karena Anda tidak akan melihatnya lagi. Kunci berbentuk `nefo_` diikuti 43 karakter acak. Panel hanya menyimpan hash SHA-256-nya.

Daftar menampilkan nama setiap kunci dan karakter pertamanya (seperti `nefo_Ab12cd…`), aksesnya, alamat yang diizinkan, kapan dan dari mana kunci terakhir dipakai (diperbarui paling sering sekali semenit), dan kapan kunci kedaluwarsa. Anda bisa punya hingga 20 kunci. Untuk mencabut kunci, pilih ikon tempat sampahnya dan konfirmasi: permintaan berikutnya dengan kunci itu gagal.

## Memanggil API {#call-the-api}

Kirim kunci di header `Authorization`, sebagai token Bearer. Alamatnya adalah alamat panel dengan path rahasianya, lalu `api/` dan rutenya:

```sh
export NEFO_KEY='nefo_…'
export NEFO='https://panel.example.com:PORT/<entrance>'

curl -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/sites"
```

- Kunci hanya dibaca dari header `Authorization`: tidak pernah dari alamat (`?key=`) atau cookie.
- Permintaan dengan body JSON membutuhkan `Content-Type: application/json`. Kolom yang tidak dikenal ditolak.
- Permintaan dengan kunci tidak membutuhkan header `X-Requested-With` yang dikirim halaman-halaman panel sendiri.
- Gunakan panel lewat HTTPS dengan sertifikat sungguhan (**Pengaturan** → **Sertifikat**, lihat [Keamanan](./security#https-from-the-first-start)). Hindari `curl -k`: perintah itu akan mengirim kunci ke siapa pun yang berada di tengah jalur.

Tugas panjang, seperti pencadangan, deploy, dan sertifikat, berjalan di latar belakang. Permintaannya dijawab `202` dengan sebuah tugas:

```json
{ "id": "a1b2c3", "kind": "backup", "target": "example.com", "status": "running", "log": [], "started": "…" }
```

Minta `GET /api/jobs/{id}` sampai `status`-nya `succeeded` atau `failed`; `log` berisi baris-barisnya dan `error` alasan kegagalannya. Panel menjalankan satu tugas dalam satu waktu: selama tugas lain berjalan, tugas baru dijawab `409` dengan `busy`.

## Contoh {#examples}

Rutenya adalah rute yang dipakai halaman-halaman panel. Ganti `3` dengan `id` situs web Anda.

### Menampilkan daftar situs web {#list-the-websites}

```sh
curl -s -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/sites"
```

Jawabannya `{"sites": [{"id": 3, "domain": "example.com", "ssl": true, …}]}`. Kunci hanya baca sudah cukup.

### Mencadangkan situs web {#back-up-a-website}

```sh
curl -s -X POST -H "Authorization: Bearer $NEFO_KEY" -H "Content-Type: application/json" \
  -d '{"kind": "site", "target": "example.com", "remote": true}' \
  "$NEFO/api/backups"
```

- `kind` bernilai `site` (`target` adalah domainnya), `database` (nama basis data), `folder` (path absolut), atau `app` (nama aplikasi dari Toko aplikasi).
- `remote: true` juga mengunggah cadangan ke penyimpanan jarak jauh, jika sudah disiapkan (lihat [Pencadangan](./backups#remote-storage)).

Permintaan ini dijawab `202` dengan tugasnya. Untuk menunggunya:

```sh
JOB=$(curl -s -X POST -H "Authorization: Bearer $NEFO_KEY" -H "Content-Type: application/json" \
  -d '{"kind": "database", "target": "shop", "remote": false}' "$NEFO/api/backups" | jq -r .id)
while [ "$(curl -s -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/jobs/$JOB" | jq -r .status)" = running ]; do
  sleep 5
done
```

### Deploy dari Git {#deploy-from-git}

Siapkan repositori di tab **Deploy Git** situs web lebih dulu (lihat [Deploy dari Git](./git-deploy)). Lalu, dari CI Anda setelah pengujian lolos:

```sh
curl -fsS -X POST -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/sites/3/git/deploy"
```

Permintaan ini tidak butuh body dan dijawab `202` dengan tugasnya. Di workflow GitHub Actions, simpan kunci dan alamatnya di secrets repositori:

```yaml
- name: Deploy
  run: curl -fsS -X POST -H "Authorization: Bearer ${{ secrets.NEFO_KEY }}" "${{ secrets.NEFO_URL }}/api/sites/3/git/deploy"
```

### Memasang sertifikat {#install-a-certificate}

Untuk mendapatkan sertifikat gratis dari Let's Encrypt (lihat [Sertifikat SSL](./ssl)):

```sh
curl -s -X POST -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/sites/3/ssl"
```

Permintaan ini dijawab `202` dengan tugasnya. Untuk memvalidasi lewat DNS Cloudflare sebagai gantinya, yang dibutuhkan nama wildcard, kirim `-H "Content-Type: application/json" -d '{"dns": true}'`.

Untuk menginstal sertifikat Anda sendiri, misalnya yang diperpanjang di server lain, kirim sertifikat dengan rantainya dan kunci privatnya, dalam format PEM, total hingga 256 KB. `jq` menyusun JSON dari file-file tersebut:

```sh
jq -n --rawfile cert fullchain.pem --rawfile key privkey.pem '{cert: $cert, key: $key}' |
  curl -s -X POST -H "Authorization: Bearer $NEFO_KEY" -H "Content-Type: application/json" \
    --data-binary @- "$NEFO/api/sites/3/certificate"
```

Permintaan ini dijawab `200` dengan situs web dan daftar `warnings`, atau `400` dengan alasannya, seperti kunci yang tidak cocok dengan sertifikat (lihat [Memakai sertifikat Anda sendiri](./ssl#use-your-own-certificate)).

## Kesalahan {#errors}

Kesalahan berbentuk JSON, seperti `{"error": "invalid_api_key"}`, sebagian dengan `detail`.

| Status | `error` | Arti |
|---|---|---|
| 401 | `invalid_api_key` | Kunci tidak dikenal, sudah dicabut atau kedaluwarsa, atau dipakai dari alamat yang tidak diizinkannya. Jawabannya tidak menyebut yang mana; log aktivitas menyebutkannya. |
| 403 | `api_key_read_only` | Kunci hanya baca mengirim permintaan selain `GET` atau `HEAD`, atau meminta kata sandi, file, ekspor, atau unduhan cadangan. |
| 403 | `api_key_forbidden` | Hanya sesi yang sudah masuk yang bisa melakukan ini (lihat [Apa yang bisa dilakukan sebuah kunci](#what-a-key-can-do)). |
| 429 | `too_many_attempts` | Terlalu banyak upaya gagal dari alamat ini. Tunggu `retry_after` detik, yang juga ada di header `Retry-After`. |
| 400 | misalnya `bad_request` | Permintaannya salah; `detail` mungkin menyebutkan apa yang salah. |
| 404 | `not_found` | Item tidak ada. Path di luar path rahasia mendapat 404 polos. |
| 409 | `busy` | Ada tugas lain yang sedang berjalan. Coba lagi setelah tugas itu selesai. |

## Upaya gagal dan fail2ban {#failed-attempts-and-fail2ban}

Upaya gagal dengan kunci dihitung seperti upaya masuk yang gagal, pada hitungan yang sama (lihat [Batas upaya masuk](./security#sign-in-limits)): 5 kegagalan dalam 15 menit memblokir alamat tersebut selama 15 menit, untuk kunci dan untuk halaman masuk. Kunci yang valid tidak mengatur ulang hitungan.

Setiap kegagalan tercatat di **Akun** → **Aktivitas terbaru** sebagai "Gagal masuk", dengan alasannya: `unknown API key`, `API key expired`, atau `API key not allowed from this address`. Jail panel milik fail2ban membacanya, jadi fail2ban memblokir alamat yang terus menebak (lihat [Perlindungan brute-force](./security#brute-force-protection-fail2ban)).

## Log aktivitas {#the-activity-log}

- Setiap perubahan yang dibuat dengan kunci (setiap permintaan kecuali `GET` dan `HEAD`) dicatat sebagai "Perubahan lewat kunci API", dengan metode dan path-nya, seperti `POST /api/sites/3/git/deploy`.
- Pengguna pada entri ini adalah nama pengguna Anda dan nama kuncinya, seperti `admin (key deploy-bot)`. Begitu pula entri untuk apa yang dilakukan permintaan itu, seperti "Men-deploy dari Git".
- Permintaan yang ditolak untuk sebuah kunci dicatat sebagai "Kunci API ditolak"; pembuatan dan pencabutan kunci sebagai "Membuat kunci API" dan "Mencabut kunci API".
- Pembacaan tidak dicatat. Daftar kunci menampilkan kapan setiap kunci terakhir dipakai, dan dari mana.

Kunci itu sendiri tidak pernah ditulis ke log mana pun.

## Saran keamanan {#security-advice}

- **Pakai kunci hanya baca jika memungkinkan.** Pemantauan dan laporan hanya butuh `GET`.
- **Satu kunci per skrip.** Namanya tampil di log, dan Anda bisa mencabutnya tanpa merusak yang lain.
- **Izinkan hanya alamat yang membutuhkannya**, seperti runner CI atau server cadangan Anda, jika alamatnya tetap.
- **Biarkan kunci kedaluwarsa, dan ganti secara berkala**: buat kunci baru, pasang di skrip, lalu cabut yang lama.
- **Jauhkan kunci dari repositori dan alamat.** Gunakan penyimpanan rahasia CI Anda atau file yang hanya bisa dibaca pemiliknya, dan variabel di perintah, agar kunci tidak tersimpan di riwayat shell.
- **Segera cabut kunci jika mungkin sudah bocor**, lalu periksa **Aktivitas terbaru** untuk melihat apa yang dilakukannya.
- Jika [firewall](./security#firewall-ufw) membatasi port panel ke alamat tertentu, izinkan juga alamat skrip di sana.
