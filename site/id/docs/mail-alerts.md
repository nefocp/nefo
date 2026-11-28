---
title: Email dan peringatan
description: Biarkan situs web Anda mengirim email lewat penyedia SMTP, dan terima peringatan lewat Telegram, email, Discord, Slack, atau webhook saat server membutuhkan perhatian atau seseorang masuk.
---

# Email dan peringatan

Nefo tidak punya server email. Situs web mengirim email lewat Postfix, yang bekerja sebagai relay khusus pengiriman ke penyedia SMTP Anda (Gmail, SendGrid, Amazon SES, dan lainnya). Peringatan sampai kepada Anda lewat Telegram, email, Discord, Slack, atau webhook bertanda tangan saat CPU, memori, atau disk tinggi, sebuah layanan berhenti, pencadangan gagal, situs web mati, atau seseorang masuk.

Halaman ini membahas keduanya: mengirim email dari situs web, dan peringatan. Untuk menyiapkan record DNS penyedia email (MX, SPF, DMARC), lihat [DNS (Cloudflare)](./dns#set-up-email-records).

## Sengaja tanpa server email {#no-mail-server-by-design}

Nefo tidak menyediakan kotak surat dan tidak menerima email. Server email lengkap memakan memori dan butuh perawatan terus-menerus, yang tidak cocok untuk VPS kecil. Simpan kotak surat Anda di penyedia email.

Untuk email keluar, Nefo menginstal Postfix sebagai relay khusus pengiriman. Relay ini hanya mendengarkan di server ini dan meneruskan setiap pesan ke penyedia Anda (Gmail, SendGrid, Amazon SES, dan lainnya), dengan masuk memakai akun yang Anda masukkan.

## Mengirim email dari situs web {#send-e-mail-from-websites}

Setelah relay disiapkan, fungsi `mail()` di PHP berfungsi, karena Postfix menyediakan `/usr/sbin/sendmail`, yang dipakai PHP. Formulir kontak, reset kata sandi, dan pemberitahuan WordPress lalu bisa mengirim email tanpa plugin apa pun. Tanpa relay, situs web tidak bisa mengirim email.

### Menyiapkan relay {#set-up-the-relay}

1. Buka **Pengaturan** → **Email**.
2. Pilih **Penyedia** Anda. Panel mengisi **Server SMTP**, **Port**, dan **Enkripsi** untuk penyedia itu.
3. Masukkan **Nama pengguna** dan **Kata sandi** akun penyedia Anda. Petunjuk di bawah **Penyedia** menyebutkan apa yang diharapkan penyedia tersebut.
4. Opsional, isi **Kirim semuanya sebagai (opsional)**, seperti `noreply@example.com`.
5. Pertama kali, pilih **Instal dan simpan**: panel menginstal Postfix dan mengonfigurasinya. Selanjutnya, tombolnya menjadi **Simpan**.

Penyiapan berjalan di latar belakang dengan log langsung. Setelah selesai, kirim pesan uji (lihat di bawah).

| Penyedia | Server SMTP | Nama pengguna dan kata sandi |
|---|---|---|
| Gmail / Google Workspace | `smtp.gmail.com` | Alamat Gmail Anda, dan sandi aplikasi (Google account → Security → 2-Step Verification → App passwords; di akun Google berbahasa Indonesia: Keamanan → Verifikasi 2 Langkah → Sandi aplikasi), bukan kata sandi biasa Anda |
| Microsoft 365 / Outlook | `smtp.office365.com` | Alamat Microsoft 365 Anda. Pengiriman SMTP harus diizinkan untuk kotak surat tersebut |
| SendGrid | `smtp.sendgrid.net` | `apikey` (sudah diisikan untuk Anda), dan kunci API SendGrid dengan izin Mail Send. Kirim dari pengirim yang sudah diverifikasi |
| Mailgun | `smtp.mailgun.org` | Kredensial SMTP domain Mailgun Anda |
| Amazon SES | `email-smtp.us-east-1.amazonaws.com` | Kredensial SMTP SES Anda. Ganti server ke endpoint wilayah Anda, dan kirim dari alamat atau domain yang sudah diverifikasi |
| Brevo | `smtp-relay.brevo.com` | Login SMTP dan kunci SMTP Brevo Anda |
| Server SMTP lain | Server penyedia Anda | Port 587 dengan STARTTLS, atau 465 dengan SSL/TLS. Port 25 diblokir oleh sebagian besar penyedia cloud |

Setiap preset memakai port 587 dengan STARTTLS.

### Kolom dan aturan {#fields-and-rules}

| Kolom | Apa yang dimasukkan |
|---|---|
| **Server SMTP** | Nama host penyedia, atau alamat IP |
| **Port** | 1 sampai 65535, biasanya 587 atau 465 |
| **Enkripsi** | **STARTTLS** (port 587), **SSL/TLS** (port 465), atau **Tidak ada (hanya server ini)** |
| **Nama pengguna**, **Kata sandi** | Keduanya atau tidak sama sekali, tanpa spasi. Kosongkan **Kata sandi** untuk mempertahankan yang tersimpan |
| **Kirim semuanya sebagai (opsional)** | Alamat email yang menggantikan pengirim setiap pesan |

- **Tidak ada (hanya server ini)** hanya diterima untuk relay di server ini sendiri (`localhost`, `127.0.0.1`, atau `::1`). Tanpa enkripsi, kata sandi akan melintasi internet dalam teks biasa.
- Dengan STARTTLS atau SSL/TLS, Postfix menolak mengirim jika koneksi tidak bisa dienkripsi.
- Gunakan **Kirim semuanya sebagai (opsional)** jika penyedia Anda hanya menerima alamat pengirim yang sudah diverifikasi. Opsi ini menggantikan pengirim setiap pesan dari setiap situs web.
- Kata sandi penyedia disimpan di `/etc/postfix/sasl_passwd`, yang hanya bisa dibaca root. Situs web tidak pernah melihatnya.

### Mengirim pesan uji {#send-a-test-message}

1. Di bawah formulir, masukkan alamat di **Kirim pesan uji ke…**.
2. Pilih **Kirim uji**.

Panel mengikuti pesan di log email dan menunggu hingga satu menit untuk jawaban penyedia. Hasilnya menyebutkan apakah pesan sudah terkirim, belum diterima (Postfix terus mencoba), atau ditolak penyedia, diikuti balasan penyedia. Jika penyedia tidak menjawab dalam satu menit, pesan tetap di antrean dan Postfix terus mencoba.

### Memeriksa relay {#check-the-relay}

Di bagian atas kartu:

- **Me-relay lewat …** berarti Postfix berjalan dan memakai penyedia Anda. **Postfix tidak berjalan** berarti Postfix berhenti.
- **… pesan menunggu** menghitung pesan yang masih ada di antrean. Postfix terus mencoba ulang sebuah pesan hingga 2 hari.
- **Masalah pengiriman terakhir:** menampilkan alasan terbaru sebuah pesan tidak bisa dikirim, seperti yang diberikan penyedia.

### Menghapus relay {#remove-the-relay}

Pilih **Hapus** dan konfirmasi. Panel mencopot Postfix dan melupakan pengaturannya. Situs web tidak bisa lagi mengirim email.

## Peringatan {#alerts}

Peringatan mengirimi Anda pesan lewat Telegram, email, Discord, Slack, atau webhook Anda sendiri, saat server membutuhkan perhatian. Buka **Pengaturan** → **Peringatan**. Setiap saluran bersifat opsional: siapkan sebanyak yang Anda mau, dan setiap peringatan dikirim ke semuanya sekaligus. Saluran yang gagal tidak menghentikan saluran lain.

Panel memeriksa server setiap menit. Setiap masalah dilaporkan sekali, lagi setiap 6 jam selama masih berlangsung, dan sekali lagi saat sudah teratasi. Setiap pesan diawali nama server dalam tanda kurung siku. Peringatan email punya subjek "Server alert:" diikuti nama server.

### Apa yang mengirim peringatan {#what-sends-an-alert}

| Peringatan | Dikirim saat | Bawaan |
|---|---|---|
| **CPU di atas (%)** | pemakaian CPU rata-rata selama 5 menit mencapai batas | 90 |
| **Memori di atas (%)** | pemakaian memori mencapai batas | 90 |
| **Disk di atas (%)** | disk (`/`) sudah sepenuh itu | 90 |
| **Sebuah layanan berhenti (Nginx, PHP, MariaDB…)** | layanan yang berjalan saat boot tidak sedang berjalan | aktif |
| **Pencadangan, pemulihan, atau perpanjangan sertifikat gagal** | pencadangan, pemulihan, perpanjangan sertifikat situs web, atau perpanjangan sertifikat panel gagal | aktif |
| Pemantauan situs web (tab **Pemantauan** setiap situs web) | situs web yang dipantau mati (2 pemeriksaan gagal berturut-turut), hidup lagi, atau menyajikan sertifikat yang berakhir dalam waktu kurang dari 14 hari | nonaktif |
| **Masuk lewat SSH** | seseorang masuk lewat SSH: setiap kali, atau hanya dari alamat baru | **Tanpa peringatan** |
| **Upaya SSH gagal dalam 10 menit** | sebanyak itu upaya masuk SSH gagal dalam 10 menit | 0 (nonaktif) |
| **Masuk ke panel** | seseorang masuk ke panel: setiap kali, atau hanya dari alamat baru | **Tanpa peringatan** |
| **Beri tahu saya saat pemeriksaan harian menemukan masalah baru** (di halaman **Pemeriksaan keamanan**) | [pemeriksaan keamanan](./security-check) harian menemukan masalah kritis atau peringatan baru | nonaktif |
| **Beri tahu saya saat file PHP berubah** (tab **Penguncian file** sebuah situs web) | pemeriksaan harian situs web yang terkunci menemukan file PHP yang diubah, ditambahkan, atau dihapus (lihat [Penguncian file](./website-protection#alerts-when-php-files-change)) | nonaktif |

- Atur batas ke 0 untuk menonaktifkannya. Batas lainnya harus antara 10 dan 100.
- Layanan yang dipantau adalah server web (Nginx, Apache, atau OpenLiteSpeed), PHP-FPM, MariaDB atau MySQL, PostgreSQL, Redis, Memcached, dan fail2ban, jika terinstal.
- Tidak ada yang diperiksa sampai setidaknya satu saluran disiapkan.
- Pemantauan situs web diaktifkan per situs web, dan pemeriksaannya berjalan bahkan tanpa saluran: tab **Pemantauan** menampilkan hasilnya. Peringatan situs mati diulang setiap 6 jam, seperti peringatan lainnya. Lihat [Situs web](./websites#monitoring).

### Peringatan saat masuk {#sign-in-alerts}

Terima pesan saat seseorang masuk ke server lewat SSH atau ke panel, agar aktivitas masuk yang bukan dari Anda tidak luput dari perhatian.

1. Buka **Pengaturan** → **Peringatan** dan siapkan setidaknya satu saluran.
2. Di bawah **Aktivitas masuk**, pilih untuk **Masuk lewat SSH** dan **Masuk ke panel**:
   - **Tanpa peringatan**;
   - **Setiap kali masuk**;
   - **Hanya dari alamat baru**: masuk dari alamat yang belum pernah dipakai akun itu untuk masuk dalam 180 hari terakhir.
3. Opsional: di **Upaya SSH gagal dalam 10 menit**, masukkan berapa upaya gagal yang memicu peringatan, dari 3 sampai 100000. 0 menonaktifkannya.
4. Klik **Simpan**.

Peringatan SSH menyebutkan akun, alamat, cara akun itu masuk, dan waktunya, misalnya:

```
[server1] 🔑 SSH sign-in: admin from 203.0.113.10 (new address) with a key, 2026-11-27 10:31:05 +07
```

Beberapa aktivitas masuk dalam menit yang sama dikirim dalam satu pesan. Serangkaian upaya gagal menyebutkan alamat yang paling banyak mencoba:

```
[server1] ⚠️ 57 failed SSH sign-in attempts in the last 10 minutes, from 198.51.100.7 (40), 203.0.113.5 (17).
```

Cara kerjanya:

- Tidak ada yang mengawasi log SSH terus-menerus. Sekali semenit, panel membaca apa yang ditulis layanan SSH (`ssh.service` atau `sshd.service`) ke jurnal sistem sejak pembacaan terakhir, dan mengingat di mana ia berhenti. Peringatan SSH tidak memakan sumber daya apa pun selama nonaktif.
- Saat Anda mengaktifkan peringatan SSH, peringatan dimulai sejak saat itu: aktivitas masuk sebelumnya tidak pernah dilaporkan. Alamat aktivitas masuk SSH 30 hari terakhir dianggap sudah dikenal.
- Untuk panel, alamat aktivitas masuk yang tercatat di log aktivitas dianggap sudah dikenal.
- Untuk IPv6, setiap alamat dalam /64 yang sama dihitung sebagai satu alamat, karena sebagian besar penyedia memberikan satu /64 untuk satu pelanggan. Paling banyak 500 alamat diingat untuk SSH dan 500 untuk panel.
- Jika panel berhenti lebih dari satu jam, saat berjalan lagi panel hanya membaca satu jam terakhir dari log SSH.
- Upaya gagal adalah kata sandi yang salah dan nama akun yang tidak dikenal. Jika masuk dengan kata sandi dinonaktifkan, kunci yang ditolak tidak dihitung. Paling banyak satu peringatan upaya beruntun dikirim per jam.
- Untuk memblokir alamat yang terus mencoba, instal fail2ban di halaman **Firewall & SSH**. Lihat [Perlindungan brute force](./security#brute-force-protection-fail2ban).

### Menerima peringatan lewat Telegram {#get-alerts-by-telegram}

1. Di Telegram, buka BotFather, kirim `/newbot`, dan ikuti pertanyaannya. BotFather menjawab dengan token bot: angka, titik dua, lalu deretan panjang huruf dan angka.
2. Buka bot baru Anda dan kirimi pesan apa saja. Bot tidak bisa menulis kepada Anda sebelum Anda menulis kepadanya.
3. Dapatkan chat ID Anda, misalnya dari userinfobot. Chat ID berupa angka.
4. Di **Pengaturan** → **Peringatan**, masukkan **Token bot** dan **ID chat**.
5. Pilih **Simpan**, lalu **Kirim uji coba**.

Untuk mengirim peringatan ke grup atau channel, tambahkan bot ke sana terlebih dahulu. Chat ID grup diawali tanda minus. Untuk channel publik, Anda juga bisa memasukkan `@name`-nya.

Setelah Anda menyimpan, token ditampilkan sebagai titik-titik. Biarkan titik-titik itu apa adanya untuk mempertahankan token yang tersimpan. Jika Anda mengosongkan **ID chat** lalu menyimpan, token juga dihapus.

### Menerima peringatan lewat email {#get-alerts-by-e-mail}

Email peringatan dikirim langsung dari panel melalui server SMTP, dengan pengaturannya sendiri, terpisah dari relay untuk situs web. Anda bisa memakai akun penyedia yang sama.

1. Di bawah **Email**, masukkan **Server SMTP** dan **Port** (bawaannya 587).
2. Masukkan **Nama pengguna** dan **Kata sandi** akun tersebut, jika server membutuhkan login.
3. Masukkan alamat **Dari** dan **Ke**.
4. Pilih **Simpan**, lalu **Kirim uji coba**.

Di port 465, koneksi dienkripsi sejak awal. Di port lain, panel memakai STARTTLS jika server menawarkannya. Jika server tidak menawarkannya dan nama pengguna sudah diatur, panel menolak mengirim, jadi kata sandi tidak pernah berpindah dalam teks biasa. Kata sandi yang tersimpan ditampilkan sebagai titik-titik, seperti token Telegram.

### Menerima peringatan di Discord {#get-alerts-in-discord}

1. Di Discord, buka pengaturan channel yang akan menerima peringatan, lalu **Integrations** → **Webhooks** → **New Webhook** (di Discord berbahasa Indonesia: Integrasi → Webhook → Webhook Baru).
2. Beri nama webhook jika mau, lalu pilih **Copy Webhook URL** (Salin URL Webhook).
3. Di **Pengaturan** → **Peringatan**, tempelkan ke **URL webhook** di bawah **Discord**.
4. Pilih **Simpan**, lalu **Kirim uji coba**.

URL-nya terlihat seperti `https://discord.com/api/webhooks/<id>/<token>`. Alamat di `discordapp.com` serta versi `ptb.` dan `canary.` juga bisa, begitu pula `?thread_id=` untuk memposting ke thread. Peringatan yang lebih panjang dari 2000 karakter, batas Discord, akan dipotong. Mention di dalam peringatan, seperti `@everyone`, tidak memberi notifikasi kepada siapa pun.

### Menerima peringatan di Slack {#get-alerts-in-slack}

1. Buka [api.slack.com/apps](https://api.slack.com/apps), pilih **Create New App** → **From scratch**, lalu pilih workspace Anda.
2. Buka **Incoming Webhooks**, aktifkan, lalu pilih **Add New Webhook to Workspace** dan pilih channel-nya.
3. Salin **Webhook URL**. URL ini diawali `https://hooks.slack.com/services/`.
4. Di **Pengaturan** → **Peringatan**, tempelkan ke **URL webhook** di bawah **Slack**, pilih **Simpan**, lalu **Kirim uji coba**.

Hanya incoming webhook yang diterima: workflow webhook Slack mengharapkan kolom lain.

### Mengirim peringatan ke webhook {#send-alerts-to-a-webhook}

Webhook memposting setiap peringatan sebagai JSON ke alamat pilihan Anda: skrip Anda sendiri, n8n, Home Assistant, atau aplikasi chat yang tidak tercantum di sini. Di bawah **Webhook**, masukkan **URL webhook**-nya dan, jika penerima memeriksa tanda tangan, **Rahasia untuk penandatanganan (opsional)** sepanjang 16 sampai 200 karakter tanpa spasi. Lalu pilih **Simpan** dan **Kirim uji coba**.

Setiap peringatan berupa `POST` dengan `Content-Type: application/json` dan body ini:

```json
{"server":"vps-1","text":"⚠️ The disk is 93% full.","time":"2026-11-26T08:15:00Z","kind":"alert"}
```

| Kolom | Nilai |
|---|---|
| `server` | Nama host server |
| `text` | Isi peringatan, tanpa nama server. Masalah yang sudah teratasi diawali "✅ Resolved:" |
| `time` | Kapan peringatan dikirim, dalam UTC, RFC 3339 |
| `kind` | `alert`, atau `test` untuk **Kirim uji coba** |

Setiap jawaban 2xx dihitung sebagai terkirim. Panel menunggu paling lama 10 detik, tidak mengikuti pengalihan, dan tidak menampilkan jawaban penerima, hanya kode statusnya.

#### Memverifikasi tanda tangan {#verify-the-signature}

Dengan rahasia, setiap permintaan punya header `X-Nefo-Signature: sha256=` diikuti HMAC-SHA256 dari body, dalam heksadesimal, dengan rahasia sebagai kuncinya. Hitung dari body persis seperti yang diterima, sebelum mem-parse JSON, dan bandingkan dalam waktu konstan. Dalam Python:

```python
import hashlib, hmac

def verify(secret: bytes, body: bytes, signature: str) -> bool:
    expected = "sha256=" + hmac.new(secret, body, hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected, signature)
```

Di Node.js, bandingkan `'sha256=' + crypto.createHmac('sha256', secret).update(rawBody).digest('hex')` dengan header memakai `crypto.timingSafeEqual`, setelah memeriksa bahwa panjang keduanya sama. Body yang ditandatangani menyertakan `time`: tolak permintaan yang umurnya lebih dari beberapa menit untuk mencegah replay.

#### Alamat yang tidak bisa dipakai webhook {#webhook-addresses}

- URL diawali `https://`. Alamat `http://` biasa hanya bisa dipakai setelah Anda mencentang **Izinkan alamat http:// (tidak terenkripsi, untuk penerima di jaringan Anda sendiri)**: tanpa enkripsi, peringatan dan tanda tangannya bisa dibaca di perjalanan.
- Tidak ada nama pengguna atau kata sandi di URL, tidak ada `#`, dan paling banyak 2048 karakter.
- Agar webhook tidak bisa menjangkau layanan milik server sendiri, panel tidak pernah terhubung ke alamat loopback (`127.0.0.0/8`, `::1`, `localhost`), alamat link-local (`169.254.0.0/16`, `fe80::/10`), layanan metadata cloud (`169.254.169.254`, `100.100.100.200`, `168.63.129.16`, `fd00:ec2::254`), `0.0.0.0/8`, atau alamat multicast. Nama host diperiksa lagi setiap kali peringatan dikirim, pada alamat hasil resolve-nya, jadi nama yang mengarah ke `127.0.0.1` juga ditolak.
- Jaringan privat (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `fd00::/8`) diizinkan, untuk penerima di rumah atau di dalam kontainer.

### URL webhook yang tersimpan {#saved-webhook-urls}

URL Discord, Slack, dan webhook mengandung token-nya, jadi setelah Anda menyimpan, URL itu ditampilkan sebagai titik-titik, seperti token Telegram, begitu pula rahasianya. Biarkan titik-titik itu apa adanya untuk mempertahankan nilai yang tersimpan. Kosongkan URL lalu simpan untuk berhenti memakai saluran itu. Mengosongkan URL webhook juga menghapus rahasianya dan izin http://.

### Mengirim peringatan uji {#send-a-test-alert}

**Kirim uji coba** mengirim pesan uji melalui setiap saluran yang tersimpan. Pengujian memakai pengaturan yang tersimpan, jadi pilih **Simpan** terlebih dahulu. Hasilnya menampilkan satu baris per saluran, **Telegram**, **Email**, **Discord**, **Slack**, atau **Webhook**, dengan **terkirim** atau pesan kesalahannya. Jika belum ada saluran yang disiapkan, panel menampilkan "Siapkan saluran terlebih dahulu, lalu simpan."

Untuk Discord, Slack, dan webhook, kesalahannya salah satu dari ini:

- **Tidak bisa menjangkau webhook** diikuti alasannya, misalnya `no such host` atau `connection refused`.
- **Webhook menolak pesan (HTTP 404…)**: penerima menjawab dengan status selain 2xx. Discord dan Slack menambahkan alasannya, seperti `Unknown Webhook` atau `invalid_token`. Pengalihan (3xx) juga dihitung sebagai penolakan.
- **Webhook tidak menjawab dalam 10 detik.**
- **… adalah alamat loopback, link-local, atau metadata cloud**: lihat [Alamat yang tidak bisa dipakai webhook](#webhook-addresses).

Jika peringatan sungguhan tidak bisa dikirim, log panel menyebutkannya dengan nama saluran, tidak pernah dengan URL-nya.

### Peringatan dan dasbor {#alerts-and-the-dashboard}

Daftar **Perlu perhatian** di **Dasbor** dan ikon lonceng di kanan atas hanya menampilkan masalah selama Anda masuk, dan keduanya memeriksa hal lain, seperti masuk dua faktor dan firewall. Peringatan menjangkau Anda saat Anda sedang tidak di depan panel. Lihat [Alat sistem](./system#dashboard).
