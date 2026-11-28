---
title: DNS (Cloudflare)
description: Kelola record DNS zona Cloudflare Anda dari panel, arahkan nama-nama situs web ke server ini, tambahkan record MX, SPF, dan DMARC dari penyedia email Anda, dan kosongkan cache Cloudflare.
---

# DNS (Cloudflare)

Nefo tidak menjalankan server DNS. Jika DNS domain Anda di-hosting di Cloudflare, Nefo mengelola record-nya lewat API Cloudflare dengan token API: Nefo menambah, mengubah, dan menghapus record, mengarahkan nama-nama situs web ke server ini setelah pratinjau, menambahkan record MX, SPF, dan DMARC dari penyedia email Anda, dan mengosongkan cache Cloudflare.

Anda menyimpan token di **Pengaturan** → **Cloudflare**. Setelah itu:

- halaman **DNS**, di kelompok **Hosting** pada menu kiri, menampilkan zona Anda beserta record-nya, menambah, mengubah, dan menghapus record, serta mengosongkan cache Cloudflare;
- **Arahkan DNS ke sini…** di tab **Domain** sebuah situs web mengarahkan nama-nama situs web ke server ini dalam satu langkah;
- **Siapkan email** di halaman DNS menambahkan record MX, SPF, dan DMARC dari sebuah penyedia email;
- sertifikat wildcard divalidasi lewat token yang sama: lihat [Sertifikat SSL](./ssl#wildcard-certificates-through-cloudflare-dns).

Nefo hanya berkomunikasi dengan API Cloudflare, dan token tidak pernah muncul di log atau pesan panel.

## Token API {#the-api-token}

Satu token dipakai untuk semua ini. Token membutuhkan izin berikut, untuk zona (domain) yang Anda pakai di server ini:

| Izin | Kegunaannya |
|---|---|
| Zone → DNS → Edit | Menampilkan, menambah, mengubah, dan menghapus record, **Arahkan DNS ke sini…**, dan validasi DNS untuk sertifikat. |
| Zone → Zone → Read | Menampilkan zona beserta status dan paketnya. Tanpa izin ini, daftar zona bisa tetap kosong, atau Cloudflare menolak menampilkan zona. |
| Zone → Cache Purge → Purge | **Kosongkan cache** di halaman DNS. |

1. Di dasbor Cloudflare, buka **My Profile → API Tokens → Create Token**.
2. Mulai dari template **Edit zone DNS**. Template ini memberi Zone → DNS → Edit.
3. Klik **+ Add more** dan tambahkan Zone → Zone → Read serta Zone → Cache Purge → Purge.
4. Di bawah **Zone Resources**, sertakan zona yang Anda pakai di sini, atau semua zona di akun tersebut.
5. Buat token, salin, lalu simpan di Nefo: **Pengaturan** → **Cloudflare** → **Token API** → **Periksa dan simpan**.

Jika token tidak punya sebuah izin, Nefo menyebut izin itu, misalnya: "Cloudflare menolak: token API tidak memiliki izin Zone → DNS → Edit untuk zona ini." Edit token di Cloudflare (**My Profile → API Tokens**, lalu **Edit** di sebelah token), tambahkan izin untuk zona tersebut, dan simpan. Mengedit token tidak mengubah nilainya, jadi tidak ada yang perlu diubah di Nefo. Jika Anda menghapus token atau melakukan roll, simpan token baru di **Pengaturan** → **Cloudflare**.

## Halaman DNS {#the-dns-page}

Halaman ini menampilkan satu zona dalam satu waktu. Jika ada beberapa zona, pilih salah satu di daftar di bagian atas; statusnya (**Aktif**, atau **Tertunda** selama name server domain belum milik Cloudflare) dan paketnya ada di sebelahnya.

Tabel mencantumkan record zona: **Jenis**, **Nama** (`@` adalah zona itu sendiri, `www` adalah `www.example.com`), **Konten**, **Proxy**, dan **TTL**. Saring berdasarkan jenis, atau cari nama dan konten. Zona dengan lebih dari 5.000 record menampilkan 5.000 record pertamanya; gunakan dasbor Cloudflare untuk sisanya.

Nefo bisa menambah dan mengubah record A, AAAA, CNAME, TXT, MX, dan CAA. Jenis lain, seperti NS atau SRV, dicantumkan dengan "di Cloudflare": ubah record tersebut di dasbor Cloudflare.

### Menambah atau mengubah record {#add-or-change-a-record}

Klik **Tambah record**, atau **Edit** di baris sebuah record. Jenis record tidak bisa diubah: hapus record lalu tambahkan yang baru. Nefo memeriksa record sebelum mengirimnya ke Cloudflare:

| Jenis | Yang dimasukkan |
|---|---|
| A | Alamat IPv4, seperti `203.0.113.10`. |
| AAAA | Alamat IPv6, seperti `2001:db8::10`. |
| CNAME | Nama host yang menjadi tujuan alias nama ini, seperti `example.com`. Tidak bisa mengarah ke namanya sendiri. |
| TXT | Satu baris teks, hingga 2.048 karakter: teks biasa, yang dipecah Cloudflare menjadi string 255 karakter, atau `"quoted strings"` yang masing-masing paling panjang 255 karakter. Beri tanda kutip pada seluruh nilai atau tidak sama sekali. |
| MX | Nama host server email, seperti `mail.example.com`, dan **Prioritas** dari 0 sampai 65535 (yang lebih kecil dicoba lebih dulu). |
| CAA | **Tag**: `issue` atau `issuewild` dengan domain sebuah CA, seperti `letsencrypt.org` (`;` tidak mengizinkan CA mana pun), atau `iodef` dengan alamat `mailto:` atau `https://`. **Flag** bernilai 0, atau 128 untuk kritis. |

**TTL** bernilai **Otomatis** (pilihan Cloudflare) atau waktu tetap dari 1 menit sampai 1 hari.

### Diproksikan atau Hanya DNS {#proxied-or-dns-only}

Record A, AAAA, dan CNAME bisa **Diproksikan** (awan oranye Cloudflare): pengunjung lalu sampai ke Cloudflare, yang meneruskan permintaan web ke alamat tersebut dan menyembunyikannya. Record yang diproksikan selalu memakai TTL otomatis. **Hanya DNS** (awan abu-abu) menjawab dengan alamat itu sendiri. Jenis record lain selalu Hanya DNS.

Proxy Cloudflare hanya meneruskan HTTP dan HTTPS di port 80 dan 443 (dan beberapa port lain). Biarkan semua yang lain Hanya DNS: SSH, email, dan panel.

::: warning Biarkan nama panel Hanya DNS
Jika Anda membuka panel lewat nama seperti `panel.example.com`, biarkan record itu **Hanya DNS**. Proxy Cloudflare tidak meneruskan port panel, jadi dengan awan oranye panel tidak lagi terbuka lewat nama itu. Halaman DNS dan **Arahkan DNS ke sini…** memperingatkan Anda jika record itu diproksikan.
:::

## Mengarahkan situs web ke server ini {#point-a-website-to-this-server}

1. Buka **Situs web**, lalu tab **Domain** milik situs web tersebut.
2. Di bawah **DNS di Cloudflare**, klik **Arahkan DNS ke sini…**.
3. Periksa alamat dan pilihannya:
   - **Alamat IPv4 publik server ini**, dibaca dari antarmuka jaringan server;
   - **Publikasikan juga alamat IPv6 (AAAA)**, dicentang jika server punya alamat IPv6 publik;
   - **Diproksikan lewat Cloudflare (awan oranye)**.
4. Baca pratinjaunya: untuk setiap nama dan jenis, record **Sekarang** dan **Sesudah**, ditandai **baru**, **diubah**, **dihapus**, atau **tidak berubah**.
5. Klik **Terapkan**.

Nefo hanya mengubah record untuk nama-nama situs web, yaitu domainnya dan nama domain tambahannya; record nama lain tidak pernah disentuh. Untuk setiap nama:

- satu record A mengarah ke server ini: record A yang ada diperbarui, atau record baru dibuat, dan record A lain untuk nama itu dihapus;
- dengan IPv6, hal yang sama terjadi pada record AAAA-nya. Tanpa IPv6, record AAAA yang mengarah ke server lain dihapus, karena pengunjung IPv6 dan Let's Encrypt akan sampai ke server itu;
- record CNAME diganti dengan record A, kecuali jika mengarah ke nama lain milik situs web tersebut (`www` → `example.com` tetap seperti apa adanya);
- nama yang tidak berada di zona mana pun yang bisa dilihat token dibiarkan, dan pratinjau mencantumkannya.

Saat Anda mengklik **Terapkan**, Nefo menghitung ulang perubahannya. Jika record di Cloudflare berubah sejak pratinjau, tidak ada yang diubah dan Nefo menampilkan pratinjau baru: "Record di Cloudflare berubah sejak pratinjau."

Jika server berada di balik NAT dan hanya punya alamat IPv4 privat, Nefo menampilkan "Tidak ditemukan alamat IPv4 publik di antarmuka jaringan server ini". Ketik alamat publik yang menjangkau server. Nefo tidak menanyakannya ke layanan luar mana pun.

### Let's Encrypt di balik awan oranye {#let-s-encrypt-behind-the-orange-cloud}

Let's Encrypt memeriksa situs web lewat HTTP biasa. Dengan proxy aktif, pengaturan **Always Use HTTPS** Cloudflare (**SSL/TLS → Edge Certificates**) menjawab pemeriksaan itu dengan pengalihan sebelum sampai ke server, dan sertifikat gagal. Pilih salah satu:

- nonaktifkan **Always Use HTTPS** di Cloudflare, lalu aktifkan [Alihkan HTTP ke HTTPS](./ssl#redirect-http-to-https) di Nefo sebagai gantinya; atau
- centang **Validasi lewat DNS (Cloudflare)** di tab **SSL / HTTPS** situs web, dan biarkan pengaturan Cloudflare seperti apa adanya.

Setelah situs web punya sertifikat, atur mode enkripsi **SSL/TLS** Cloudflare ke **Full (strict)**. Dengan **Flexible**, Cloudflare menjangkau server lewat HTTP, dan **Alihkan HTTP ke HTTPS** mengirimnya kembali ke HTTPS dalam putaran tanpa akhir.

## Bagaimana cara menyiapkan record DNS untuk email saya? {#set-up-email-records}

Nefo tidak meng-hosting kotak surat (lihat [Email dan peringatan](./mail-alerts)). Jika email Anda di-hosting oleh Google Workspace, Microsoft 365, Zoho Mail, atau penyedia lain, atau diteruskan oleh Cloudflare Email Routing, Nefo bisa menambahkan record DNS yang dibutuhkan penyedia tersebut:

| Record | Fungsinya |
|---|---|
| MX | Menyatakan server mana yang menerima email untuk domain. |
| SPF (record TXT) | Menyatakan server mana yang boleh mengirim email untuk domain. |
| DMARC (record TXT di `_dmarc`) | Memberi tahu server penerima apa yang harus dilakukan dengan email yang gagal pemeriksaan SPF dan DKIM, dan ke mana laporan dikirim. |
| DKIM (record TXT atau CNAME) | Memublikasikan kunci yang dipakai penyedia Anda untuk menandatangani email. |

Tidak ada yang berubah sampai Anda melihat daftar perubahan dan menerapkannya.

1. Buka **DNS** dan pilih zona.
2. Klik **Siapkan email**.
3. Di bawah **Penyedia email**, pilih **Google Workspace**, **Microsoft 365**, **Zoho Mail**, **Cloudflare Email Routing**, atau **Penyedia lain**.
4. Opsional: di **Domain email**, masukkan sebuah nama di zona tersebut, seperti `mail` untuk `mail.example.com`. Kosongkan untuk zona itu sendiri.
5. Opsional: masukkan **Alamat laporan DMARC (opsional)**, dan tempelkan record DKIM jika Anda sudah memilikinya (lihat di bawah).
6. Klik **Tampilkan perubahan**. Tabel mencantumkan setiap record dengan nilai **Sekarang** dan **Sesudah**, serta apa yang terjadi padanya: baru, diubah, dihapus, tidak berubah, atau penyedia lain.
7. Klik tombol terapkan, seperti **Terapkan 4 perubahan**.

Jika Anda mengubah sebuah kolom setelah pratinjau, klik **Pratinjau lagi** sebelum menerapkan. Jika record di Cloudflare berubah sementara itu, Nefo menolak dan menampilkan pratinjau baru.

### Yang didapat setiap penyedia {#what-each-provider-gets}

| Penyedia | Record MX | SPF include |
|---|---|---|
| **Google Workspace** | `smtp.google.com`, prioritas 1 | `include:_spf.google.com` |
| **Microsoft 365** | `example-com.mail.protection.outlook.com` (domain Anda dengan tanda hubung sebagai pengganti titik), prioritas 0 | `include:spf.protection.outlook.com` |
| **Zoho Mail** | `mx.zoho.com` (10), `mx2.zoho.com` (20), `mx3.zoho.com` (50), dengan domain **Pusat data Zoho** yang dipilih: zoho.com, zoho.eu, zoho.in, atau zoho.com.au | `include:zohomail.com`, atau `zohomail.eu`, `zohomail.in`, `zohomail.com.au` |
| **Cloudflare Email Routing** | `route1.mx.cloudflare.net`, `route2.mx.cloudflare.net`, `route3.mx.cloudflare.net` | `include:_spf.mx.cloudflare.net` |
| **Penyedia lain** | Server yang Anda masukkan di bawah **Server email (MX)**, hingga 5, masing-masing dengan prioritas | Domain di **SPF include (opsional)** dan alamat di **Alamat pengirim (opsional)**; jika keduanya kosong, `mx` |

Beberapa detail:

- **Google Workspace**: jika zona sudah punya record MX Google yang lebih lama (`aspmx.l.google.com` dan lainnya), Nefo mempertahankannya dan tidak menambahkan apa pun.
- **Microsoft 365**: Microsoft memberi setiap domain host MX-nya sendiri. Bandingkan nilai di pratinjau dengan nilai yang ditampilkan Microsoft 365 admin center untuk domain tersebut. Jika berbeda, pilih **Penyedia lain** dan masukkan nilainya. Record MX yang sudah mengarah ke Microsoft 365 dipertahankan apa adanya.
- **Zoho Mail**: pilih pusat data yang alamatnya Anda pakai untuk masuk. Untuk pusat data Zoho lain, pilih **Penyedia lain** dan masukkan nilai yang ditampilkan Zoho.
- **Cloudflare Email Routing**: email baru masuk setelah Anda mengaktifkan Email Routing untuk zona tersebut di dasbor Cloudflare dan menambahkan alamat tujuan di sana. Record routing Cloudflare yang sudah ada mempertahankan prioritas yang diberikan Cloudflare.

### SPF: selalu satu record {#spf-always-one-record}

Sebuah domain harus punya tepat satu record SPF; dua record membuat pemeriksaan SPF gagal. Nefo tidak pernah menambahkan record kedua:

- Jika zona punya record SPF, Nefo mempertahankan semua isinya, menambahkan include penyedia, dan mempertahankan akhirannya (`-all`, `~all`, atau `?all`). Record tanpa akhiran mendapat `~all`.
- Jika zona punya beberapa record SPF, Nefo menggabungkannya menjadi satu: Nefo mengubah record pertama dan menghapus yang lain.
- Jika Anda menyiapkan relay SMTP di **Pengaturan** → **Email**, record SPF juga mengizinkan penyedia relay tersebut (misalnya `include:sendgrid.net` untuk SendGrid, `include:mailgun.org` untuk Mailgun, `include:amazonses.com` untuk Amazon SES), jadi email yang dikirim situs web Anda lolos pemeriksaan. Pratinjau menyebutkannya.
- Record SPF yang membutuhkan lebih dari 10 pencarian DNS (`include`, `a`, `mx`, `ptr`, `exists`, `redirect`) gagal di banyak server penerima. Pratinjau memperingatkan Anda; hapus include yang tidak lagi Anda pakai.

### Record MX dari penyedia lain {#mx-records-of-another-provider}

Record MX yang mengarah ke penyedia lain ditandai "penyedia lain". Email akan terkirim ke kedua penyedia, jadi record itu harus dihapus: centang **Ganti record MX yang ada** untuk menghapusnya. Nefo menolak menerapkan perubahan sampai Anda melakukannya, atau sampai Anda menghapus sendiri record tersebut.

::: warning
Email berhenti sampai ke penyedia lama begitu record MX-nya dihapus. Pindahkan kotak surat Anda lebih dulu.
:::

### DMARC {#dmarc}

Nefo menambahkan `v=DMARC1; p=none` di `_dmarc`, dengan `rua=mailto:` dan alamat Anda jika Anda memasukkan **Alamat laporan DMARC (opsional)**. Dengan `p=none`, tidak ada email yang ditolak: server penerima hanya mengirim laporan. Jika laporan menunjukkan semua email Anda lolos, Anda bisa memperketat kebijakannya (`p=quarantine`, lalu `p=reject`) dengan mengedit record di halaman DNS.

Jika zona sudah punya record DMARC, Nefo membiarkannya apa adanya.

### DKIM {#dkim}

Penyedia email yang membuat kunci DKIM, jadi Nefo tidak bisa membuatnya. Di bawah **Tanda tangan DKIM**, dialog menjelaskan tempat menemukannya di penyedia Anda:

- **Google Workspace**: di Google Admin console, buka Apps → Google Workspace → Gmail → Authenticate email, lalu buat record baru. Namanya `google._domainkey` dan jenisnya TXT.
- **Microsoft 365**: di Microsoft Defender portal, buka pengaturan DKIM domain tersebut. Microsoft memberi dua record CNAME, `selector1._domainkey` dan `selector2._domainkey`; dialog punya satu baris untuk masing-masing.
- **Zoho Mail**: di Zoho Mail Admin Console, buka pengaturan DKIM domain tersebut dan tambahkan selector. Masukkan nama selector diikuti `._domainkey` dan nilai TXT-nya.
- **Cloudflare Email Routing** tidak butuh record DKIM di sini: Cloudflare menambahkan yang dibutuhkannya saat Anda mengaktifkan Email Routing.
- **Penyedia lain**: minta record DKIM ke penyedia tersebut.

Tempelkan **Nama record** dan **Nilai**, pilih **Jenis** (TXT atau CNAME), lalu lihat pratinjau. Nama seperti `google._domainkey` berada di bawah domain email; nama lengkap juga diterima. Record DKIM dengan nama yang sama diganti. Biarkan nilainya kosong untuk menambahkan record DKIM nanti: jalankan **Siapkan email** lagi, dengan penyedia yang sama, setelah Anda memilikinya. Lalu selesaikan penyiapan di penyedia (Google: **Start authentication**; Microsoft dan Zoho: aktifkan atau verifikasi DKIM).

Menjalankan **Siapkan email** lagi aman: record yang sudah benar ditampilkan sebagai "tidak berubah".

Setiap record yang ditambahkan, diubah, atau dihapus Nefo ditulis ke log aktivitas, seperti perubahan DNS lainnya.

## Mengosongkan cache Cloudflare {#purge-cloudflare-s-cache}

Dengan proxy aktif, Cloudflare menyimpan salinan file Anda. Setelah Anda mengubah file tersebut, klik **Kosongkan cache** di halaman DNS:

- **Semuanya** mengosongkan cache Cloudflare untuk seluruh zona. Cloudflare lalu mengambil setiap file dari server lagi, jadi situs web bisa lebih lambat selama beberapa menit;
- **URL berikut** mengosongkan hingga 30 alamat zona sekaligus, satu per baris, seperti `https://example.com/style.css`.

Ini adalah cache Cloudflare, bukan [cache halaman](./websites#page-cache) milik situs web sendiri di server ini.

## Log aktivitas {#activity-log}

Setiap perubahan dicatat di log aktivitas (**Akun** → **Aktivitas terbaru**): "Menambahkan record DNS", "Mengubah record DNS", "Menghapus record DNS", dan "Mengosongkan cache Cloudflare", dengan jenis, nama, dan konten record. **Arahkan DNS ke sini…** mencatat setiap record yang diubahnya.

## Pesan {#messages}

| Pesan | Yang perlu dilakukan |
|---|---|
| `No Cloudflare API token is saved.` | Simpan token di **Pengaturan** → **Cloudflare**. |
| `Cloudflare refused: the API token lacks the permission …` | Tambahkan izin yang disebutkan ke token: lihat [Token API](#the-api-token). |
| `Cloudflare did not accept this token.` | Token salah, kedaluwarsa, atau sudah dihapus. Buat token baru. |
| `None of these names is in a Cloudflare zone the API token can see` | Tambahkan zona ke **Zone Resources** token, atau atur record di tempat DNS domain di-hosting. |
| `Cloudflare does not answer for example.com yet (pending).` | Ubah name server domain di registrar Anda ke dua name server yang ditampilkan Cloudflare. |
| `Cloudflare refused: …` | Pesan Cloudflare sendiri, misalnya saat record CNAME dan record A akan memakai nama yang sama. |
