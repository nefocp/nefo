---
title: Situs web
description: Pilih server web, buat situs web PHP, statis, aplikasi (Node.js, Python, Java), dan reverse proxy, lalu kelola domain, pengaturan PHP, opsi akses, SFTP, lalu lintas, log, kloning, dan penghapusannya.
---

# Situs web

Situs web di Nefo adalah domain yang disajikan oleh satu-satunya server web di server, yaitu Nginx, Apache, atau OpenLiteSpeed: situs web PHP, situs web statis, aplikasi (Node.js, Python, atau Java) di balik proxy, atau reverse proxy ke alamat lain. Setiap situs web mendapat pengguna Linux, folder, dan akun SFTP opsional sendiri, dan bisa dikloning, dicadangkan, atau dihapus dari pengaturannya.

Halaman ini membahas server web, cara membuat situs web, dan setiap pengaturan yang dimiliki situs web: domain, PHP, aturan rewrite, opsi akses, SFTP, statistik lalu lintas, log, kloning, dan penghapusan. Sertifikat punya halamannya sendiri, [Sertifikat SSL](./ssl), begitu pula [pemblokiran negara dan penguncian file](./website-protection).

## Server web mana yang sebaiknya saya pilih? {#choose-the-web-server}

Satu server menjalankan satu server web dalam satu waktu, dan Nefo menyiapkan setiap situs web untuk server web tersebut. Sebelum Anda menambahkan situs web pertama, halaman **Situs web** meminta Anda memilih salah satu:

| Server web | Pilih jika | Memori saat menganggur |
|---|---|---|
| Nginx | Situs web Anda tidak membutuhkan file `.htaccess`. Ini pilihan yang paling ringan. | sekitar 15 MB |
| Apache | Situs Anda mengandalkan file `.htaccess`, misalnya plugin WordPress atau situs yang dipindahkan dari shared hosting. | sekitar 40 MB |
| OpenLiteSpeed | Anda menginginkan aturan rewrite dari file `.htaccess` dan plugin LiteSpeed Cache untuk WordPress. Server ini juga melayani HTTP/3. | sekitar 26 MB |

Untuk menginstalnya:

1. Buka **Situs web**.
2. Di bawah **Instal server web**, pilih Nginx, Apache, atau OpenLiteSpeed.
3. Klik **Instal Nginx** (nama tombolnya mengikuti server yang Anda pilih).

Nefo menginstal server web dan versi PHP dari distribusi Anda beserta PHP-FPM, lalu membuka port 80 dan 443 di firewall (ufw), ditambah 443/udp untuk HTTP/3 di OpenLiteSpeed. Ini butuh satu sampai tiga menit. OpenLiteSpeed berasal dari repositori paket milik LiteSpeed sendiri, dan konsol WebAdmin-nya tetap dimatikan.

Ketiga server web menjalankan PHP di pool PHP-FPM terpisah untuk setiap situs web. Hanya Nginx yang bisa membatasi permintaan per pengunjung; pengaturan lainnya berfungsi sama di ketiganya.

### Beralih ke server web lain {#switch-to-another-web-server}

Nefo tidak bisa memindahkan situs web yang sudah ada ke server web lain, karena konfigurasi setiap situs ditulis untuk server web yang terinstal. Untuk beralih:

1. Hapus setiap situs web (lihat [Menghapus situs web](#delete-a-website)). Cadangkan dulu jika Anda masih membutuhkannya: lihat [Pencadangan](./backups).
2. Buka **Perangkat lunak** dan cari kartu **Nginx, Apache, atau OpenLiteSpeed**.
3. Klik **Copot** pada server web yang sekarang, ketik namanya (`nginx`, `apache`, atau `openlitespeed`), lalu konfirmasi.
4. Klik **Instal** pada server web yang Anda inginkan.

Pencopotan mempertahankan folder situs web di `/var/www`, sertifikat, dan akun Let's Encrypt. PHP tetap terinstal. Adminer dihapus karena membutuhkan server web, dan aturan firewall yang ditambahkan Nefo untuk situs web (port 80 dan 443) dihapus.

Nefo juga menolak menginstal server web selama server web lain terinstal di server, bahkan yang Anda instal sendiri: keduanya membutuhkan port 80 dan 443. Halaman **Perangkat lunak** lalu menampilkannya sebagai "terinstal, tidak dipakai panel". Hapus server web itu terlebih dahulu.

## Membuat situs web {#create-a-website}

1. Arahkan record A DNS domain ke alamat IP server Anda, misalnya `203.0.113.10`.
2. Buka **Situs web** dan klik **Tambah situs web**.
3. Di **Domain**, ketik nama tanpa `http://` atau `www.`, misalnya `example.com`.
4. Biarkan **Juga merespons di www.example.com** tercentang jika situs juga harus merespons di `www`. Nefo mencentangnya untuk Anda jika domain terdiri dari dua bagian, seperti `example.com`.
5. Pilih **Jenis** (lihat tabel di bawah).
6. Klik **Buat situs web**.

Nefo lalu menampilkan "example.com sudah siap. Unggah file Anda ke /var/www/example.com/public." Halaman sambutan tampil sampai Anda menggantinya.

| Jenis | Apa yang dilakukan situs web |
|---|---|
| **PHP 8.3** (satu entri per versi PHP yang terinstal) | Menyajikan file Anda dan menjalankan file `.php` dengan versi PHP tersebut. |
| **Statis (hanya HTML)** | Menyajikan HTML, CSS, JavaScript, dan file lain. Tanpa PHP. |
| **Aplikasi (Node.js, Python, Java…) di balik proxy** | Meneruskan setiap permintaan ke program yang mendengarkan di sebuah port server ini. |
| **Reverse proxy ke alamat lain** | Meneruskan setiap permintaan ke alamat yang Anda berikan: server lain, atau layanan di server ini yang tidak dijalankan Nefo. Lihat [Situs web reverse proxy](#reverse-proxy-websites). |

Jenis situs web tidak bisa diubah nanti: situs statis tidak bisa menjadi situs PHP, situs aplikasi tidak bisa menjadi situs PHP atau statis, dan situs web reverse proxy juga tidak bisa menjadi jenis lain. Buat situs web baru sebagai gantinya. Untuk menginstal WordPress atau aplikasi lain, lihat [Toko aplikasi](./apps). Aplikasi Docker mendapat situs webnya sendiri dari sana.

Nefo menolak domain yang tidak valid (tulis nama internasional dalam punycode, `xn--…`), nama yang sudah dipakai situs web lain, dan domain yang foldernya sudah ada di `/var/www`.

### Situs web aplikasi (Node.js, Python, Java) {#app-websites-node-js-python}

Untuk jenis **Aplikasi (Node.js, Python, Java…) di balik proxy**, formulir menanyakan dua hal lagi:

- **Port yang didengarkan aplikasi (127.0.0.1)**: port dari 1024 sampai 65535. Nefo menolak port panel sendiri, 3306 (MariaDB), 6379 (Redis), 7809 (pendengar webhook Git), dan port yang dipakai situs web aplikasi lain.
- **Perintah mulai (opsional)**: satu baris berisi paling banyak 1000 karakter, seperti `node server.js`. Nefo menjalankannya di folder `app` situs web sebagai pengguna Linux situs web tersebut, dengan `PORT`, `HOST=127.0.0.1`, dan `NODE_ENV=production` sudah diatur. Perintah ini berjalan sebagai layanan systemd `panel-app-<user>.service` dan dimulai ulang 3 detik setelah berhenti. Kosongkan perintahnya jika program dijalankan dengan cara lain, misalnya di Docker.

Letakkan kode Anda di `/var/www/example.com/app`, lewat [SFTP](#sftp-access) atau dengan [deploy Git](./git-deploy), yang men-deploy situs web aplikasi ke folder ini. Lalu, di pengaturan situs web:

1. Di tab **Runtime**, pilih **Runtime**: **Bawaan server (node di /usr/local/bin)**, versi Node.js yang Anda instal di halaman **Perangkat lunak**, Python (`app/.venv`), yang membutuhkan Python terinstal di halaman **Perangkat lunak**, atau versi Java yang Anda instal di halaman **Perangkat lunak**, yang tampil misalnya sebagai "Java 21 (OpenJDK 21.0.9)". Tab yang sama juga mengubah **Perintah mulai (opsional)**. Klik **Simpan**.
2. Di tab **Ringkasan**, di bawah **Program aplikasi**, klik **Instal dependensi**. Nefo menjalankan `npm ci` (atau `npm install` jika tidak ada `package-lock.json`) untuk `package.json`. Untuk Python, Nefo membuat lingkungan virtual `app/.venv` dan menginstal `requirements.txt` dengan pip. Proses ini berjalan sebagai pengguna situs web, paling lama 20 menit.
3. Klik **Mulai ulang** (atau **Jalankan**) untuk menjalankan aplikasi dengan dependensinya.

Kartu **Program aplikasi** muncul jika situs web punya perintah mulai. Kartu ini juga punya **Hentikan** dan **Log aplikasi**. Di tab **Reverse proxy** Anda bisa mengubah port. Server web meneruskan permintaan ke `127.0.0.1` di port tersebut, dengan alamat pengunjung di `X-Real-IP` dan `X-Forwarded-For`, dan koneksi WebSocket berfungsi. Untuk meneruskan ke server lain, atau ke layanan di server ini yang tidak dijalankan Nefo, buat [situs web reverse proxy](#reverse-proxy-websites) sebagai gantinya.

Daftar situs web punya filter **Java** begitu ada situs web aplikasi Java, di samping filter untuk jenis lainnya.

#### Situs web aplikasi Java {#java-app-websites}

Untuk program Java, isi **Perintah mulai (opsional)** dengan perintah yang menjalankannya, misalnya `java -jar app.jar`, lalu letakkan `app.jar` di `/var/www/example.com/app`. Pilih versi Java sebagai **Runtime**. Instal Java terlebih dahulu di halaman **Perangkat lunak** (lihat [Java](./software#java)).

- Layanan aplikasi mendapat `JAVA_HOME` dari versi tersebut dan folder `bin`-nya di urutan pertama `PATH`, jadi `java` adalah versi itu. Selain `PORT` dan `HOST=127.0.0.1`, yang didapat setiap situs web aplikasi, layanan ini juga mendapat `SERVER_PORT` (port yang sama) dan `SERVER_ADDRESS=127.0.0.1`, yang dibaca Spring Boot. Aplikasi Spring Boot pun mendengarkan di port yang benar, hanya di server ini, tanpa pengaturan tambahan. Framework lain membaca `PORT` atau membutuhkannya di pengaturannya sendiri.
- **Batas memori (MB)** muncul untuk runtime Java. Pengaturan ini menetapkan heap maksimum Java (`-Xmx`), dari 32 sampai 65536 MB, lewat `JDK_JAVA_OPTIONS`, yang dibaca setiap `java` yang dijalankan perintah mulai, termasuk dari skrip mulai. Jika dikosongkan, Java memakai paling banyak seperempat memori server. Prosesnya memakai memori sedikit lebih banyak daripada heap-nya. Java mencetak `NOTE: Picked up JDK_JAVA_OPTIONS: -Xmx512m` di awal **Log aplikasi**.
- Skrip [deploy Git](./git-deploy) situs web berjalan dengan `JAVA_HOME` dan `PATH` yang sama, jadi `./mvnw package` atau `./gradlew build` melakukan build dengan versi Java situs web. Batas memori tidak berlaku untuk skrip ini.
- **Instal dependensi** tidak ditawarkan untuk situs web Java: build file jar di skrip deploy Anda atau sebelum Anda mengunggahnya.
- Versi Java yang dipakai sebuah situs web tidak bisa dicopot di halaman **Perangkat lunak**. Pilih runtime lain terlebih dahulu, atau hapus situs webnya.

## Situs web reverse proxy {#reverse-proxy-websites}

Situs web reverse proxy meneruskan setiap permintaan ke alamat yang Anda pilih: situs web atau aplikasi di server lain, atau layanan di server ini yang tidak dijalankan Nefo, seperti program yang Anda jalankan sendiri atau kontainer dari proyek Compose milik Anda. Situs web ini punya domain, sertifikat HTTPS, opsi akses, dan firewall web sendiri, seperti situs web lainnya. Fitur ini berfungsi dengan Nginx, Apache, dan OpenLiteSpeed.

1. Buka **Situs web** dan klik **Tambah situs web**.
2. Ketik **Domain** dan pilih jenis **Reverse proxy ke alamat lain**.
3. Di **Teruskan ke**, ketik alamatnya: `http://` atau `https://`, nama host atau alamat IP, port opsional, dan path opsional, misalnya `http://198.51.100.7:8080` atau `https://backend.example.com/app`.
4. Pilih **Header Host yang dikirim**: domain situs web ini, atau host upstream.
5. Klik **Buat situs web**.

Nefo menampilkan "example.com dibuat. Situs ini meneruskan ke … setelah domainnya mengarah ke server ini." Alamat disimpan bersama port-nya, jadi `https://backend.example.com/app` tampil sebagai `https://backend.example.com:443/app`.

Path permintaan ditempatkan setelah path alamat: dengan `http://198.51.100.7:8080/app`, kunjungan ke `https://example.com/shop?page=2` sampai ke `http://198.51.100.7:8080/app/shop?page=2`.

### Opsinya {#the-options}

Ubah opsi di tab **Reverse proxy** situs web lalu klik **Simpan**. Situs web reverse proxy yang baru punya WebSocket dan header X-Forwarded-\* dalam keadaan aktif, dengan batas waktu 60 detik.

| Opsi | Fungsinya |
|---|---|
| **Teruskan ke** | Alamatnya, seperti di atas. |
| **Header Host yang dikirim** | Domain situs web mengirim domain yang diminta pengunjung: sebagian besar aplikasi di balik proxy mengharapkannya. **Host upstream** mengirim nama upstream itu sendiri (dengan port-nya jika bukan 80 atau 443), untuk server yang hanya merespons namanya sendiri. Alamat upstream sendiri di pengalihan (`Location`) lalu diganti dengan alamat situs web. |
| **WebSocket** | Meneruskan koneksi WebSocket (chat, pembaruan langsung, terminal). |
| **Header X-Forwarded-\*** | Memberi tahu upstream alamat pengunjung dan apakah permintaan datang lewat HTTPS: `X-Forwarded-For`, `X-Forwarded-Proto`, `X-Forwarded-Host`, dan `X-Real-IP`. Jika nonaktif, header ini dihapus, termasuk yang dikirim pengunjung. |
| **Batas waktu (detik)** | Berapa lama upstream boleh menjawab, dari 5 sampai 3600 detik. Koneksinya sendiri harus terbuka dalam 30 detik (atau dalam batas waktu itu, jika lebih pendek). Upstream yang lebih lambat membuat pengunjung mendapat halaman kesalahan: 504 dengan Nginx, 502 dengan Apache. |
| **Abaikan sertifikat upstream (ditandatangani sendiri)** | Ditampilkan untuk alamat `https://`. Menerima sertifikat apa pun dari upstream. |

::: warning
**Abaikan sertifikat upstream (ditandatangani sendiri)** tetap mengenkripsi koneksi ke upstream, tetapi siapa pun yang bisa menyadapnya bisa menyamar sebagai upstream. Gunakan hanya untuk upstream di jaringan Anda sendiri yang memakai sertifikat yang ditandatangani sendiri.
:::

Tanpa opsi itu, Nginx dan Apache memeriksa sertifikat upstream `https://` terhadap otoritas sertifikat server (`/etc/ssl/certs/ca-certificates.crt`), untuk nama di header Host: domain situs web, atau host upstream jika Anda memilihnya. Sertifikat yang ditandatangani sendiri atau kedaluwarsa membuat pengunjung mendapat halaman kesalahan (502 dengan Nginx, 500 dengan Apache).

### Apa yang ditolak Nefo {#what-nefo-refuses}

- alamat yang bukan `http://` atau `https://` dengan nama host atau alamat IP, port (1 sampai 65535) dan path opsional, misalnya alamat dengan nama pengguna, kueri (`?`), fragmen (`#`), spasi, `..` di path, atau alamat `0.0.0.0` dan `::`;
- nama host yang tidak ditemukan ("Nama host … tidak ditemukan. Periksa lagi, atau gunakan alamat IP-nya.");
- panel itu sendiri: port panel atau `7809` (pendengar webhook Git) di server ini, baik Anda menulis `127.0.0.1`, `localhost`, salah satu alamat IP server sendiri, atau nama yang mengarah ke alamat tersebut;
- port 80 dan 443 di server ini: situs web akan meneruskan ke server web itu sendiri;
- **Batas waktu (detik)** di bawah 5 atau di atas 3600.

### Perlu diketahui {#good-to-know}

- Dengan Nginx, nama host dicari saat permintaan masuk, memakai server DNS dari `/etc/resolv.conf`, jadi nama yang tidak bisa di-resolve saat boot tidak pernah menghalangi Nginx berjalan. Pencarian ini tidak membaca `/etc/hosts`: untuk nama yang hanya dikenal `/etc/hosts`, gunakan alamat IP-nya.
- HTTPS, pengalihan ke HTTPS, dan HSTS berfungsi seperti pada situs web lain: lihat [Sertifikat SSL](./ssl).
- **Tangguhkan**, **Pengalihan**, perlindungan kata sandi, alamat IP yang diblokir, batas laju, [pemblokiran negara](./website-protection#block-or-allow-countries), dan firewall web berlaku sebelum permintaan sampai ke upstream.
- Tab untuk aturan rewrite, halaman 404 kustom, perlindungan hotlink, dan penguncian file tidak berlaku untuk situs web reverse proxy dan disembunyikan. Direktif Anda sendiri (tab konfigurasi server web) tetap berlaku.
- Situs web ini tetap punya folder `/var/www/example.com` dan pengguna Linux, untuk SFTP dan pencadangan, tetapi tidak menyajikan file-file tersebut.

### Dengan OpenLiteSpeed {#with-openlitespeed}

OpenLiteSpeed meneruskan permintaan dengan cara yang berbeda dari Nginx dan Apache. Tab **Reverse proxy** menyebutkannya dalam sebuah catatan, dan **Abaikan sertifikat upstream (ditandatangani sendiri)** tidak ditampilkan di sana:

- OpenLiteSpeed tidak pernah memeriksa sertifikat upstream `https://`: sertifikat yang ditandatangani sendiri berfungsi tanpa opsi itu.
- **Batas waktu (detik)** tidak diterapkan: OpenLiteSpeed tidak punya batas untuk jawaban yang diproksikan.
- OpenLiteSpeed tidak mengirim alamat pengunjung di `X-Forwarded-For`: yang dikirim adalah `X-Forwarded-Host`, dan `X-Forwarded-Proto` untuk permintaan HTTPS. Jika **Header X-Forwarded-\*** nonaktif, Nefo menghapus `X-Forwarded-For` (dan `X-Real-IP` selalu dihapus); OpenLiteSpeed tetap mengirim `X-Forwarded-Host`.
- Koneksi WebSocket sampai ke upstream tanpa path alamat (`/live`, bukan `/app/live`), dan tidak berfungsi dengan upstream `https://`.
- Path, header Host upstream, kata sandi, dan alamat yang diblokir berfungsi seperti di server web lain.

## Membuka pengaturan situs web {#open-a-website-s-settings}

Klik domain di daftar, atau **Pengaturan** di ujung barisnya. Pengaturan terbuka di sebuah panel dengan tab dalam empat kelompok. Beberapa tab hanya tampil untuk situs web tertentu:

| Kelompok | Tab |
|---|---|
| **Dasar** | **Ringkasan**, **Domain**, **PHP** (situs PHP), **Runtime** (situs aplikasi), **Reverse proxy** (situs reverse proxy) |
| **Keamanan** | **SSL / HTTPS**, **Firewall web (WAF)**, **Blokir akses**, **Pemblokiran negara**, **Penguncian file** (tidak untuk aplikasi Docker dan situs reverse proxy), **Perlindungan hotlink** (tidak untuk situs reverse proxy), **Pemindaian malware** |
| **Konfigurasi** | **Rewrite** (situs PHP dan statis), **Cache halaman** (situs PHP di Nginx dan OpenLiteSpeed), **Konfigurasi Nginx** (dinamai sesuai server web Anda), **Pengalihan**, **Reverse proxy** (situs aplikasi: port-nya), **Halaman 404** (tidak untuk situs reverse proxy) |
| **Operasional** | **WordPress** (situs WordPress), **Deploy Git** (tidak untuk aplikasi Docker), **SFTP**, **Pencadangan**, **Pemantauan**, **Lalu lintas**, **Log**, **Kedaluwarsa** |

Tab **Ringkasan** menampilkan **Folder** situs web (dengan **Buka di File**), **Pengguna Linux**-nya, domain, versi PHP atau runtime, server web, serta status HTTPS dan firewall. Klik **Tambah catatan** untuk menulis satu baris (paling banyak 200 karakter) yang tampil di bawah domain di daftar, seperti nama klien.

## Pengguna Linux dan folder situs web {#the-website-s-linux-user-and-folders}

Setiap situs web mendapat pengguna Linux sendiri, jadi satu situs yang diretas tidak bisa membaca atau mengubah situs lain. Namanya berasal dari domain: `example.com` menjadi `example_com`, dan `blog.example.com` menjadi `blog_example_com` (dengan `_2`, `_3`… jika nama sudah dipakai). Pengguna ini tidak punya kata sandi dan shell, dan tidak bisa masuk lewat SSH kecuali Anda mengaktifkan [SFTP](#sftp-access). PHP dan aplikasi situs web berjalan sebagai pengguna ini, dan PHP hanya bisa membuka file di dalam folder situs web.

| Path | Isinya |
|---|---|
| `/var/www/example.com/public` | Folder web (document root). File situs web Anda diletakkan di sini. |
| `/var/www/example.com/logs` | `php-error.log`, kesalahan PHP situs web. |
| `/var/www/example.com/tmp` | File sementara PHP, unggahan yang sedang berlangsung, dan sesi. |
| `/var/www/example.com/app` | Khusus situs web aplikasi: kode program. |
| `/var/log/nginx/example.com.access.log` dan `.error.log` | Log server web untuk situs ini (`/var/log/apache2` di Apache, `/var/log/openlitespeed` di OpenLiteSpeed). |
| `/var/lib/nefo/certs/example.com` | Sertifikat situs web. |
| `/var/cache/nginx/nefo/example.com` | Khusus Nginx: [cache halaman](#page-cache), jika aktif. |

File server web dan PHP-FPM yang ditulis Nefo diawali "Managed by Nefo. Do not edit". Nefo menulis ulang file-file itu, jadi lakukan perubahan Anda di panel.

## Domain dan alias {#domains-and-aliases}

Di tab **Domain**, **Nama domain tambahan** mencantumkan nama lain yang membuka situs web yang sama, satu per baris, seperti `www.example.com`. Satu situs web bisa punya paling banyak 20 nama. Setiap nama harus mengarah ke server ini dan hanya bisa dimiliki satu situs web.

Wildcard seperti `*.example.com` merespons untuk setiap subdomain. Sertifikatnya membutuhkan validasi DNS lewat Cloudflare: lihat [Sertifikat SSL](./ssl).

Jika DNS domain ada di Cloudflare, **Arahkan DNS ke sini…** di tab yang sama membuat atau memperbarui record A (dan AAAA) untuk semua nama ini, setelah pratinjau: lihat [DNS (Cloudflare)](./dns#point-a-website-to-this-server).

Jika situs web memakai HTTPS dan Anda mengubah namanya, Nefo menampilkan "Nama domain berubah. Dapatkan sertifikat baru agar HTTPS mencakup semuanya." Klik **Perpanjang sertifikat**.

## Versi dan pengaturan PHP {#php-version-and-settings}

Tab **PHP** pada situs web PHP punya pengaturan berikut:

| Pengaturan | Bawaan | Yang diizinkan |
|---|---|---|
| **Versi PHP** | versi yang dipilih saat pembuatan | versi PHP apa pun yang terinstal |
| **Batas memori (MB)** | 256 | 32 sampai 4096 |
| **Unggahan terbesar (MB)** | 64 | 1 sampai 4096 (batas ukuran permintaan server web mengikutinya) |
| **Waktu jalan skrip terlama (detik)** | 300 | 10 sampai 3600 |
| **Worker PHP** | 5 | 1 sampai 200 |
| **Mode worker** | **Jalankan sesuai permintaan (hemat memori)** | atau **Siapkan beberapa (lebih cepat untuk situs sibuk)** |

**Worker PHP** adalah jumlah permintaan yang ditangani PHP situs secara bersamaan. Setiap worker bisa memakai memori sampai batas memori, jadi jumlah worker × memori harus muat di RAM server.

Demi keamanan, Nefo menonaktifkan `exec`, `passthru`, `shell_exec`, `system`, `proc_open`, `popen`, dan `pcntl_exec` di setiap situs web, karena fungsi-fungsi itu memungkinkan situs yang diretas menjalankan program di server. Di bawah **Izinkan fungsi yang dinonaktifkan demi keamanan**, centang salah satunya hanya jika sebuah aplikasi benar-benar membutuhkannya.

Klik **Simpan**. Jika PHP-FPM atau server web menolak hasilnya, Nefo mengembalikan pengaturan sebelumnya. Untuk menginstal versi PHP atau ekstensi lain, ionCube Loader, atau untuk mengubah `php.ini`, lihat [Perangkat lunak](./software).

## Aturan rewrite dan direktif Anda sendiri {#rewrite-rules-and-your-own-directives}

Di tab **Rewrite**, **Aturan rewrite** menawarkan aturan siap pakai untuk aplikasi umum: **Bawaan (front controller: index.php atau index.html)**, WordPress, Laravel, Symfony, CodeIgniter 4, Drupal, Joomla, OpenCart, Nextcloud, Moodle, dan phpBB untuk situs PHP, serta Single-page app dan Static site (clean URLs) untuk situs apa pun. Tab ini menampilkan aturan dari preset yang Anda pilih. Di Nginx, preset menggantikan blok `location /` bawaan. Di Apache, file `.htaccess` milik situs bisa menggantikan front controller dari preset, dan di OpenLiteSpeed front controller berjalan setelah aturan `.htaccess` situs. Di keduanya, aturan pelindung dari preset tidak bisa dibatalkan oleh file `.htaccess`.

**Situs statis (URL bersih)** menyajikan halaman tanpa akhiran `.html`, sesuai cara generator situs seperti VitePress (dengan `cleanUrls`) menautkannya: `/docs/install` menyajikan `docs/install.html`, dan `/docs/` menyajikan `docs/index.html`. File lain disajikan apa adanya, dan halaman yang tidak ada tetap mendapat 404.

Preset phpBB ditujukan untuk phpBB yang Anda instal sendiri. Preset ini mengirim alamat yang bukan file ke `app.php` milik phpBB, dan tidak pernah menyajikan file pengaturan phpBB (`config.php`, `common.php`), folder `cache`, `files` (lampiran), `store` (cadangan), `includes`, `phpbb`, dan `vendor`, maupun `images/avatars/upload`: semuanya menjawab 403.

Tab berikutnya dinamai sesuai server web Anda, misalnya **Konfigurasi Nginx**. Di **Direktif Nginx** (atau **Direktif Apache**, **Direktif OpenLiteSpeed**) Anda bisa menambahkan direktif Anda sendiri, paling banyak 32 KB, dalam sintaks server web tersebut:

- Nginx menambahkannya di dalam blok `server` situs, sebelum aturan rewrite.
- Apache menambahkannya di akhir blok `VirtualHost` situs.
- OpenLiteSpeed menambahkannya di akhir blok `virtualHost` situs.

Server web memeriksa konfigurasi baru sebelum ada yang berubah. Jika server web menolaknya, Nefo menampilkan kesalahannya dan situs web tetap berjalan seperti sebelumnya. Klik **Tampilkan file konfigurasi lengkap** untuk melihat apa yang dihasilkan Nefo.

Tentang file `.htaccess`: Apache membacanya. OpenLiteSpeed hanya memakai aturan rewrite-nya, yang disalin Nefo ke konfigurasi situs dalam hitungan detik setelah ada perubahan. Nginx mengabaikannya.

## Cache halaman {#page-cache}

Di Nginx, tab **Cache halaman** pada situs web PHP mengaktifkan cache FastCGI Nginx untuk situs tersebut: Nginx menyimpan setiap halaman yang dibuat PHP dan menyajikannya lagi tanpa menjalankan PHP atau bertanya ke basis data. Aktifkan sakelar **Cache halaman**.

- Sebuah halaman disimpan selama 10 menit, halaman yang tidak ada (404) selama 1 menit. Jika PHP gagal atau berhenti merespons, Nginx menyajikan salinan terakhir yang masih dimilikinya.
- Nginx selalu bertanya ke PHP untuk permintaan selain GET dan HEAD (formulir), alamat yang mengandung `?`, pengunjung dengan cookie login atau keranjang (WordPress `wordpress_logged_in_`, `wp-postpass_`, `comment_author_`, WooCommerce `woocommerce_items_in_cart`, `wp_woocommerce_session_`, dan `PHPSESSID` milik PHP sendiri), serta halaman di bawah `/wp-admin/`, `/wp-login.php`, `/xmlrpc.php`, `/wp-json/`, `/wp-cron.php`, `/cart/`, `/checkout/`, dan `/my-account/`. Halaman yang ditandai PHP sebagai privat atau dikirim dengan cookie juga tidak disimpan.
- Setiap situs web punya cache sendiri hingga 256 MB di `/var/cache/nginx/nefo/example.com`. Halaman yang tidak diminta siapa pun dalam satu jam dibuang.

Setelah Anda mengubah halaman, klik **Kosongkan cache**: setiap pengunjung langsung mendapat versi baru. Jika tidak, pengunjung yang tidak masuk mungkin melihat halaman lama hingga 10 menit. Menonaktifkan cache, atau menghapus situs web, akan menghapus foldernya.

Untuk memeriksa bahwa cache berfungsi, buka halaman dua kali di jendela privat, atau jalankan `curl -sI https://example.com/` dua kali: header `X-Cache-Status` menyatakan `MISS`, lalu `HIT` (`BYPASS` untuk permintaan di atas). File statis tidak punya header itu: Nginx memang menyajikannya secara langsung.

Di OpenLiteSpeed, tab ini menyatakan bahwa LSCache, cache halaman milik OpenLiteSpeed sendiri, sudah tertanam: untuk WordPress, instal plugin LiteSpeed Cache; aplikasi lain mengaktifkannya dengan aturan cache di `.htaccess`. Apache tidak punya cache halaman di Nefo, dan tidak punya tab ini.

## Menangguhkan, mengalihkan, atau melindungi situs web dengan kata sandi {#suspend-redirect-or-password-protect-a-website}

**Menangguhkan.** Matikan sakelar di kolom **Berjalan** pada daftar, atau **Situs web berjalan** di tab **Ringkasan**. Pengunjung lalu mendapat HTTP 503 dengan "This website is temporarily unavailable." File dan pengaturan tetap ada, dan sertifikat tetap diperpanjang. Untuk menangguhkan beberapa situs, centang situs-situs itu, pilih **Tangguhkan** di bawah **Tindakan massal**, lalu klik **Terapkan**; **Jalankan lagi** mengaktifkannya kembali.

**Mengalihkan.** Di tab **Pengalihan**, centang **Alihkan setiap permintaan ke alamat lain** dan masukkan **Alamat**, seperti `https://example.org`. Ini pengalihan permanen (301) yang mempertahankan path: `/page` diarahkan ke alamat baru diikuti `/page`. Pengalihan hanya berfungsi selama situs web berjalan.

**Kata sandi.** Di tab **Blokir akses**, di bawah **Perlindungan kata sandi**, centang **Lindungi dengan kata sandi** lalu masukkan **Nama pengguna** dan **Kata sandi** sepanjang 8 sampai 72 karakter. Browser lalu memintanya sebelum menampilkan situs. Saat mengubah pengaturan ini nanti, kosongkan kata sandi untuk mempertahankan kata sandi yang sekarang. Validasi Let's Encrypt dan webhook Git tetap bisa menjangkau situs.

Tab yang sama memblokir alamat IP dan, di Nginx, membatasi permintaan per pengunjung: lihat [Keamanan](./security#rate-limit-and-blocked-addresses). Untuk menolak seluruh negara, lihat [Pemblokiran negara](./website-protection#block-or-allow-countries).

## Halaman 404 kustom dan perlindungan hotlink {#custom-404-page-and-hotlink-protection}

**Halaman 404.** Di tab **Halaman 404**, centang **Gunakan halaman 404 saya sendiri** dan unggah halaman Anda sebagai `public/404.html`. Halaman yang ditangani aplikasi Anda sendiri, seperti halaman WordPress atau Laravel, tetap memakai halaman 404-nya sendiri.

**Perlindungan hotlink.** Di tab **Perlindungan hotlink**, centang **Perlindungan hotlink untuk gambar dan media**. Situs web lain lalu mendapat HTTP 403 saat menyematkan file `jpg`, `jpeg`, `png`, `gif`, `webp`, `avif`, `svg`, `ico`, `bmp`, `mp4`, `webm`, `mp3`, `ogg`, dan `pdf` Anda. Situs Anda sendiri, mesin pencari (Google, Bing, DuckDuckGo, dan Yahoo), dan pengunjung yang membuka file secara langsung selalu mendapatkannya. Cantumkan paling banyak 20 situs lain di bawah **Situs lain yang boleh menampilkannya**, satu per baris.

## Tanggal kedaluwarsa {#expiry-date}

Di tab **Kedaluwarsa**, atur **Tanggal kedaluwarsa** dan klik **Simpan**. Situs web berfungsi sampai akhir hari tersebut (waktu server); setelah itu, Nefo menangguhkan situs secara otomatis. Nefo memeriksanya setiap jam. Klik **Tidak pernah** untuk menghapus tanggalnya. Dalam 7 hari terakhir, daftar menampilkan "kedaluwarsa" dan tanggalnya di bawah domain. Ini berguna untuk situs klien yang dibayar sampai tanggal tertentu.

Tab yang sama punya **Tangguhkan situs web ini** untuk langsung menangguhkan situs.

## Akses SFTP {#sftp-access}

SFTP memungkinkan Anda atau klien mengunggah file dengan FileZilla, WinSCP, atau aplikasi SFTP apa pun. File-file itu lalu dimiliki pengguna situs web, sebagaimana mestinya. Fitur ini membutuhkan server OpenSSH terinstal.

1. Buka tab **SFTP** situs web dan centang **Izinkan masuk SFTP untuk situs web ini**.
2. Pilih cara akun masuk:
   - **Buat kata sandi kuat yang baru**
   - **Pilih kata sandi (minimal 12 karakter)**, paling banyak 128 karakter
   - **Pertahankan kata sandi saat ini** (jika sudah diatur)
   - **Tanpa kata sandi: masuk hanya dengan kunci SSH**
3. Opsional: tempelkan **Kunci publik SSH (opsional)**, satu per baris seperti di `authorized_keys`, paling banyak 50. Opsi seperti `command=` dihapus.
4. Klik **Simpan**. Kata sandi yang dibuat hanya ditampilkan sekali: salin sekarang.
5. Hubungkan dengan **Host**, **Port**, **Nama pengguna**, dan **Folder** yang ditampilkan. Host adalah alamat yang Anda pakai untuk membuka panel, dan port adalah port SSH server. Tab ini juga menampilkan alamat lengkapnya, misalnya `sftp://example_com@panel.example.com/var/www/example.com`.

Akun ini terkunci di folder situs web. Akun hanya melihat `/var/www/example.com`, di path yang sama seperti di panel dan di PHP, dan tidak bisa menjalankan perintah atau meneruskan port. Kebijakan kata sandi server mungkin menolak kata sandi yang Anda pilih; Nefo lalu menampilkan alasannya. Selama [file situs web dikunci](./website-protection#lock-a-website-s-files), akun SFTP hanya bisa mengubah folder yang bisa ditulis.

Menonaktifkan SFTP akan menghapus kata sandi dan kunci serta mengakhiri sesi yang terbuka.

## Statistik lalu lintas {#traffic-statistics}

Tab **Lalu lintas** membaca log akses server web untuk situs web tersebut. Pilih **Periode**: **Hari ini**, **7 hari**, **14 hari**, atau **30 hari**. Tab ini menampilkan:

- **Permintaan**, **Pengunjung** (alamat IP yang berbeda), **Bandwidth**, **Tingkat kesalahan** (4xx dan 5xx), dan **Bot**
- permintaan dan pengunjung per jam atau per hari
- **Halaman teratas**, **Alamat IP teratas**, **Perujuk**, **Browser**, dan **Bot dan skrip**

Pemeriksaan sertifikat dan pemeriksaan [pemantauan](#monitoring) milik panel sendiri tidak dihitung. Server menyimpan log sekitar 14 hari, jadi lalu lintas yang lebih lama tidak diketahui. Untuk log yang sangat besar, hanya 5.000.000 baris terbaru yang dibaca. Kolom **Kunjungan, 7 hari** di daftar situs web berasal dari log yang sama.

## Log {#logs}

Tab **Log** menampilkan baris terbaru dari:

- **Log akses (Nginx)** dan **Log kesalahan (Nginx)** (dinamai sesuai server web Anda)
- **Kesalahan PHP**, untuk situs web PHP (`/var/www/example.com/logs/php-error.log`)
- **Log aplikasi**, untuk situs web aplikasi dan Docker

## Pemantauan {#monitoring}

Tab **Pemantauan** memeriksa setiap beberapa menit bahwa situs web merespons, dan mengirimi Anda peringatan jika tidak. Aktifkan **Pantau situs web ini**; pemeriksaan pertama langsung berjalan.

Panel meminta alamat publik situs web, dengan `https://` jika situs punya sertifikat (jika tidak, `http://`), lewat DNS seperti pengunjung biasa. Jadi record DNS yang salah, halaman kesalahan Cloudflare, atau sertifikat yang kedaluwarsa juga terdeteksi. Pengalihan ke domain milik situs web itu sendiri (seperti `http://` ke `https://`, atau ke `www.`) diikuti. Pengalihan ke domain lain dihitung sebagai jawaban.

| Pengaturan | Fungsinya | Bawaan |
|---|---|---|
| **Pemeriksaan** | seberapa sering: setiap menit, atau setiap 2, 5, atau 10 menit | setiap 5 menit |
| **Path** | halaman yang diminta, seperti `/health`; query string diizinkan | `/` |
| **Teks yang harus ada di halaman** | opsional; huruf besar dan kecil tidak berpengaruh | kosong |
| **Hitung jawaban 4xx, seperti 404, sebagai mati** | jawaban 4xx menggagalkan pemeriksaan | nonaktif |

Pemeriksaan gagal jika:

- tidak ada jawaban dalam 10 detik, domain tidak bisa di-resolve, koneksi ditolak, atau sertifikat sudah kedaluwarsa, tidak tepercaya, atau untuk domain lain
- jawabannya berupa kesalahan HTTP 500 atau lebih tinggi. Jawaban 4xx dihitung sebagai hidup, kecuali Anda menghitung jawaban 4xx sebagai mati
- teks yang harus ada di halaman tidak ditemukan di 256 KB pertama halaman

Situs web dianggap **Mati** setelah 2 pemeriksaan gagal berturut-turut, dan hidup lagi setelah satu pemeriksaan berhasil. Panel lalu mengirim peringatan ke setiap saluran di **Pengaturan** → **Peringatan** (lihat [Peringatan](./mail-alerts#alerts)):

- saat situs mati, beserta alasannya, seperti "example.com is down (HTTP 502, 2 checks in a row)", dan lagi setiap 6 jam selama situs tetap mati
- saat situs hidup lagi, beserta berapa lama situs mati
- saat sertifikat yang disajikannya berakhir dalam waktu kurang dari 14 hari, paling banyak sekali sehari

Tab ini menampilkan status dan pemeriksaan terakhir, uptime selama 24 jam dan 7 hari, waktu respons rata-rata, waktu respons 24 jam terakhir dalam bentuk grafik, serta kegagalan terakhir beserta alasannya. Waktu respons dihitung sampai header jawaban tiba, termasuk waktu koneksi. Di daftar situs web, situs web terpantau yang mati menampilkan **Mati** berwarna merah di kolom **Berjalan**, dan ikon lonceng mencantumkannya.

- Pemeriksaan mengirim user agent `Nefo-Monitor/1.0`. Pemeriksaan ini tidak dihitung di tab **Lalu lintas**.
- Hasilnya disimpan selama 7 hari. Menghapus situs web akan menghapusnya.
- Situs web yang ditangguhkan tidak diperiksa.
- Pemeriksaan berjalan di server ini. Pemeriksaan tidak bisa melaporkan bahwa seluruh server mati, atau masalah di jaringan antara pengunjung Anda dan server: untuk itu, tambahkan layanan pemantauan dari luar.

## Firewall web, pemindaian malware, pemblokiran negara, penguncian file, dan pencadangan {#web-firewall-malware-scan-country-blocking-file-lock-and-backups}

- **Firewall web (WAF)**: ModSecurity dengan OWASP Core Rule Set untuk situs ini: **Nonaktif**, **Hanya deteksi: catat serangan, jangan blokir apa pun**, atau **Blokir serangan**. Instal firewall di halaman **Perangkat lunak** terlebih dahulu. Lihat [Keamanan](./security#web-application-firewall), yang juga membahas tab **Pemindaian malware**.
- **Pemblokiran negara**: tolak pengunjung dari negara tertentu, atau hanya izinkan negara tertentu. **Penguncian file**: jadikan kode situs web hanya-baca bagi PHP dan akun SFTP-nya, kecuali folder yang harus ditulisinya. Lihat [Pemblokiran negara dan penguncian file](./website-protection).
- **Pencadangan**: **Cadangkan sekarang**, atau jadwalkan pencadangan **Setiap hari pukul 03:00**. Lihat [Pencadangan](./backups).

## Mengkloning situs web {#clone-a-website}

**Kloning** menyalin situs web ke domain baru: untuk memulai situs web baru dari situs yang sudah ada, atau untuk mencoba perubahan pada salinan. Fitur ini berfungsi untuk situs web PHP, statis, dan reverse proxy, situs web aplikasi (Node.js, Python, Java), dan aplikasi Docker dari [Toko aplikasi](./apps).

1. Buka tab **Ringkasan** situs web dan klik **Kloning…** di bawah **Kloning situs web ini**.
2. Ketik **Domain salinan**.
3. Biarkan **Salin basis data yang tertaut ke situs web ini** tercentang untuk ikut menyalinnya. Kotak ini tidak ditampilkan untuk WordPress, yang basis datanya selalu disalin, dan untuk aplikasi Docker, yang datanya selalu disalin.
4. Klik **Kloning**. Situs web tetap berjalan selama penyalinan, yang berjalan sebagai tugas dengan log. Aplikasi Docker berhenti sebentar selama datanya disalin.
5. Arahkan DNS domain baru ke server ini, lalu aktifkan HTTPS di tab **SSL / HTTPS**-nya. Salinan dimulai di `http://`.

### Situs web PHP, statis, dan reverse proxy {#php-static-and-reverse-proxy-websites}

Salinan mendapatkan:

- situs web baru dengan jenis yang sama (versi PHP yang sama, statis, atau reverse proxy ke alamat yang sama dengan opsi yang sama) dengan pengguna Linux sendiri, aplikasi yang sama, pengaturan PHP, aturan rewrite, dan direktif sendiri. Di direktif Anda, folder situs web lama diganti dengan folder yang baru;
- salinan folder situs web `/var/www/example.com`, yang dimiliki pengguna baru, tanpa `logs`, `tmp`, dan kunci deploy Git (`.ssh`). Tautan simbolik disalin sebagai tautan; tautan ke tempat di dalam folder situs web lama kini mengarah ke tempat yang sama di folder baru. Di `public/.user.ini` folder lama diganti dengan folder yang baru;
- untuk setiap basis data yang tertaut, baik MariaDB, MySQL, maupun PostgreSQL, basis data dan pengguna baru dari jenis yang sama dengan salinan datanya, yang tertaut ke situs web baru;
- program latar belakang situs web, dalam keadaan berhenti (lihat di bawah).

Setelah salinan siap, Nefo menampilkan detail koneksi basis data MariaDB dan MySQL yang baru (basis data PostgreSQL ada di halaman **Basis data**). File konfigurasi milik aplikasi (seperti `.env` atau `config.php`) masih menyebut basis data dan pengguna lama: ganti dengan yang baru. File yang menyebut folder lama, seperti cache pengaturan aplikasi, juga masih begitu.

### Situs web aplikasi {#app-websites}

Salinan situs web aplikasi juga mendapatkan:

- port sendiri, yang dipilih Nefo, yang diterima aplikasi di `PORT`. Aplikasi yang menetapkan port-nya sendiri harus diubah agar memakai port baru;
- **Runtime** dan **Perintah mulai (opsional)** yang sama; di perintah tersebut, folder situs web lama diganti dengan folder yang baru. Layanan salinan `panel-app-<user>.service` berjalan setelah file dan basis data selesai disalin;
- `node_modules` beserta tautannya (`node_modules/.bin`), dan lingkungan virtual Python `app/.venv` yang berfungsi di folder barunya: path di skripnya, `pyvenv.cfg`, dan file `.pth` menyebut folder salinan. Dependensi tidak diinstal ulang;
- program latar belakang situs web, dengan perintah, folder, dan variabel yang sama (path ke folder lama diganti), tetapi **dalam keadaan berhenti**: selama salinan belum memakai basis data dan antrean sendiri, program itu akan bekerja pada milik situs asli. Jalankan program tersebut di halaman **Program latar belakang**.

Runtime-nya harus tetap terinstal: versi Node.js, Python, atau Java yang sudah dihapus dari halaman **Perangkat lunak** ditolak sebelum penyalinan dimulai.

### Aplikasi Docker {#docker-apps}

Salinan situs web aplikasi Docker adalah situs web baru dengan instalasi baru aplikasi tersebut:

- versi dan image yang sama dengan aslinya (juga jika Toko aplikasi punya versi yang lebih baru), dengan kontainer dan port sendiri serta batas memori yang sama;
- salinan data aplikasi dari `/var/lib/panel-apps/<name>`, yang dibuat selama aslinya dihentikan: Nefo menghentikannya, menyalin data, lalu langsung menjalankannya lagi. Di salinan, file milik pengguna situs web asli menjadi milik pengguna situs web baru;
- akun dan kata sandi yang sama dengan aslinya (data yang disalin dibuat dengan akun itu), yang ditampilkan di **Toko aplikasi** → **Terinstal** seperti untuk aplikasi lain;
- alamat aplikasi diatur ke domain baru, untuk aplikasi yang mengambilnya dari panel (`url`, `DOMAIN`, dan pengaturan serupa milik aplikasi). Jika aplikasi menyimpan alamatnya sendiri di pengaturannya sendiri, ubah di sana.

Nefo terlebih dahulu memeriksa memori dan disk yang dibutuhkan salinan (sekitar ukuran data aplikasi, ditambah 512 MB).

Nefo menolak mengkloning:

| Aplikasi | Alasannya |
|---|---|
| n8n | Salinan akan menjalankan workflow aktif (jadwal dan pemicu) untuk kedua kalinya. Ekspor workflow-nya dan impor ke n8n yang baru. |
| Odoo | Salinan juga akan menjalankan tindakan terjadwal dan server email: salinan akan mengirim email yang sama, serta mengambil dan menghapus pesan dari kotak surat yang sama. Cadangkan, atau instal Odoo baru. |
| Aplikasi yang diinstal sebelum ada Toko aplikasi | Perbarui dulu (**Toko aplikasi** → **Terinstal**), lalu kloning. |

Salinan aplikasi pemantauan seperti Uptime Kuma memeriksa situs web yang sama dan mengirim notifikasi yang sama dengan aslinya: jeda yang tidak Anda butuhkan di salah satunya.

### WordPress {#wordpress}

Situs WordPress disalin seperti [salinan staging](./wordpress#make-a-staging-copy): basis datanya selalu disalin, `wp-config.php` diarahkan ke salinan, dan alamat situs diganti di semua tempat, jadi salinan langsung berfungsi. Mesin pencari diminta untuk tidak mengindeksnya; jika salinan itu menjadi situs web tersendiri, ubah hal itu di WordPress di bawah **Settings → Reading**. Tidak seperti salinan staging, hasil kloning tidak bisa didorong kembali ke situs web asalnya.

### Yang tidak disalin {#not-copied}

Alias, sertifikat, perlindungan kata sandi, pengalihan dan pengaturan firewall, pemblokiran negara, penguncian file, tugas cron, pengaturan deploy Git, akses SFTP, pemantauan, dan jadwal pencadangan.

Jika satu langkah gagal, Nefo menghapus semua yang dibuatnya untuk salinan itu: situs web, pengguna Linux-nya, basis data baru, program latar belakang yang disalin, dan, untuk aplikasi Docker, kontainer baru beserta datanya. Situs asli tetap berjalan.

## Menghapus situs web {#delete-a-website}

1. Buka tab **Ringkasan** situs web.
2. Di bawah **Hapus situs web ini**, klik **Hapus**.
3. Dialog mencantumkan apa saja yang ikut terhapus bersama situs web di bawah **Ikut dihapus:** (aplikasi Docker dan datanya, akun SFTP, tugas cron, program latar belakang, pemantauan, pengaturan deploy Git, jadwal pencadangan).
4. Biarkan **Cadangkan situs web dulu** tercentang (bawaan), atau hapus centangnya.
5. Centang **Hapus juga basis datanya** untuk ikut menghapus basis data yang tertaut. Jika tidak dicentang (bawaan), basis data tetap ada di halaman **Basis data**, tidak tertaut ke situs web mana pun.
6. Ketik domainnya untuk konfirmasi dan klik **Cadangkan dan hapus** (atau **Hapus** tanpa pencadangan).

### Cadangan yang dibuat lebih dulu {#the-backup-made-first}

Dengan **Cadangkan situs web dulu**, Nefo terlebih dahulu membuat cadangan biasa di server ini: file situs web serta basis data MariaDB, MySQL, dan PostgreSQL-nya, dan untuk aplikasi Docker, datanya sebagai cadangan kedua dengan jenis **Aplikasi**. Situs web hanya dihapus jika pencadangan berhasil. Jika gagal, tugas berhenti dengan "the backup failed, so example.com was not deleted" dan tidak ada yang dihapus.

- Cadangan diberi nama `example.com_before-delete_<date>-<time>.tar.gz` (dan `<user>_before-delete_….tar.gz` untuk data aplikasi Docker), di `/var/backups/panel/site/example.com/`.
- Seperti setiap cadangan yang Anda buat secara manual, cadangan ini disimpan sampai Anda menghapusnya di halaman **Pencadangan**. Jadwal hanya menghapus cadangannya sendiri, dan jadwal situs web ikut dihapus bersamanya, jadi tidak ada yang menghapus cadangan ini.
- Cadangan ini hanya ada di server ini: tidak diunggah ke penyimpanan jarak jauh. Unggah dari halaman **Pencadangan** jika Anda menginginkan salinan jarak jauh.
- Penghapusan berjalan sebagai tugas dengan log. Log-nya menyebut nama cadangan tersebut.

::: warning
Tanpa cadangan, file situs web (dan basis datanya, jika Anda mencentang **Hapus juga basis datanya**) tidak bisa dikembalikan. Dialog menyebutkan hal ini saat Anda menghapus centang **Cadangkan situs web dulu**.
:::

### Bisakah saya mengembalikan situs web yang sudah dihapus? {#bring-a-deleted-website-back}

Bisa, dengan cadangan yang dibuat sebelum penghapusan (atau cadangan lain dari situs web tersebut). Di halaman **Pencadangan**, pulihkan cadangan `…_before-delete_…` dan ketik domainnya untuk konfirmasi. Pemulihan membuat ulang situs web dengan domain, alias, versi PHP, aturan rewrite, file, dan basis datanya. Basis data yang sudah dihapus kembali dengan kata sandi yang dimilikinya dulu, yang disebut di file yang dipulihkan; basis data yang Anda pertahankan ditautkan lagi ke situs web.

Yang tidak dibuat ulang: sertifikat (dapatkan yang baru di tab **SSL / HTTPS**), tugas cron, akses SFTP, program latar belakang, pengalihan, dan perlindungan kata sandi. Lihat [Memulihkan di server baru](./restore-new-server#restoring-a-website-that-doesn-t-exist-here) untuk apa saja yang dibuat oleh pemulihan.

Cadangan yang dibuat oleh Nefo 0.24.1 atau sesudahnya mencatat jenis situs web, jadi situs web aplikasi (Node.js, Python, Java) kembali dengan runtime, perintah mulai, port, dan batas memorinya, serta situs web reverse proxy dengan alamat dan opsinya; instal dulu runtime aplikasinya di halaman **Perangkat lunak**. Cadangan yang dibuat oleh versi yang lebih lama hanya mencatat versi PHP, jadi situs web seperti itu kembali sebagai situs web statis: buat ulang dulu situs web itu dengan domain dan jenis yang sama, lalu pulihkan. Untuk aplikasi Docker, instal ulang aplikasinya di domain yang sama dari **Toko aplikasi**, lalu pulihkan cadangannya yang berjenis **Aplikasi**.

### Apa yang dihapus dan dipertahankan Nefo {#what-nefo-removes-and-keeps}

Nefo menghapus:

- folder situs web `/var/www/example.com` beserta semua filenya, termasuk repositori Git dan kunci deploy-nya
- pengguna dan grup Linux, serta akses SFTP
- konfigurasi server web, file kata sandi, dan direktif Anda sendiri
- pool PHP-FPM, layanan aplikasi, atau aplikasi Docker beserta kontainer, data, dan image yang tidak terpakai
- sertifikat, cache halaman, dan log server web situs tersebut
- program latar belakang situs web, tugas cron yang berjalan sebagai penggunanya, jadwal pencadangannya, pemantauannya, dan pengaturan deploy Git-nya
- tempat sampah situs web di **File** dan file-filenya yang dikarantina oleh pemindai malware
- basis data yang tertaut, jika Anda mencentang **Hapus juga basis datanya**

Nefo mempertahankan cadangan yang sudah Anda buat: cadangan itu tetap ada di halaman **Pencadangan**.

Salinan staging WordPress: menghapus situs live yang punya salinan staging tidak menghapus salinannya, tetapi salinan itu tidak bisa lagi didorong ke situs live; dialog menyebutkan hal ini. Menghapus salinan staging tidak mengubah situs live-nya.

Selama deploy Git situs web sedang berjalan, Nefo menolak menghapusnya ("Ada tugas lain yang sedang berjalan di server. Tunggu sampai selesai."). Dengan pencadangan, tugas menunggu deploy yang dimulai selama pencadangan. Jika satu langkah pembersihan gagal, situs web tetap dihapus dari panel dan Nefo mencantumkan langkah-langkah yang gagal di bawah "Beberapa langkah pembersihan gagal:" (di log tugas jika cadangan dibuat lebih dulu).

Dari API: `DELETE /api/sites/{id}` dengan `{"confirm": "example.com", "delete_databases": false, "backup_first": true}`. Dengan `backup_first`, API menjawab 202 beserta tugasnya. Tanpa itu, situs web langsung dihapus (200, beserta peringatannya).
