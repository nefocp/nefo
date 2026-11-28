---
title: Perangkat lunak
description: Instal dan copot server web, PHP 5.6 sampai 8.6, ionCube Loader, MariaDB atau MySQL, PostgreSQL, Redis, Memcached, Adminer, phpMyAdmin, Node.js, Python, Java, Docker, firewall web, dan ClamAV, serta kelola ekstensi PHP dan pengaturan php.ini.
---

# Perangkat lunak

Nefo tidak menginstal apa pun sampai Anda memilihnya. Halaman **Perangkat lunak** menginstal dan mencopot setiap komponen opsional, dengan perkiraan memori yang dipakainya: server web, PHP 5.6 sampai 8.6 dan ionCube Loader, MariaDB atau MySQL, PostgreSQL, Redis, Memcached, Node.js, Python, Java, Docker, firewall web, dan ClamAV. Halaman ini menolak menghapus apa yang masih dipakai sesuatu.

Halaman dokumentasi ini membahas apa yang bisa Anda instal, kapan pencopotan ditolak, apa yang dipertahankan saat pencopotan, serta pengaturan PHP, Node.js, Python, Java, dan Redis. Server kecil tetap ringan jika hanya memuat apa yang dipakai situs webnya.

Instalasi dan pencopotan berjalan sebagai tugas dengan log langsung. Panel menjalankan satu tugas dalam satu waktu. Perangkat lunak berasal dari paket distribusi Anda kecuali halaman ini menyebutkan sebaliknya.

## Apa yang bisa Anda instal {#what-you-can-install}

Setiap kartu menampilkan perkiraan memori yang dipakai item saat berjalan. Bagian atas halaman menjumlahkan item yang terinstal.

| Bagian | Item | Memori, kira-kira |
|---|---|---|
| **Nginx, Apache, atau OpenLiteSpeed** | Nginx | sekitar 15 MB |
| | Apache | sekitar 40 MB |
| | OpenLiteSpeed | sekitar 26 MB |
| **Versi PHP** | PHP 7.4, 8.0, 8.1, 8.2, 8.3, 8.4, 8.5, dan 8.6 (serta versi yang lebih baru setelah repositori PHP menyediakannya), dan di bawah **Versi lama** 5.6, 7.0, 7.1, 7.2, dan 7.3 | sekitar 25 MB per versi |
| | ionCube Loader | tidak ada sendiri: loader dimuat ke dalam worker PHP |
| **Basis data dan cache** | MariaDB (milik distribusi Anda, atau 11.4, 11.8, atau 12.3 LTS dari repositori MariaDB), atau sebagai gantinya MySQL 8.4 LTS | MariaDB: buffer pool-nya ditambah sekitar 100 MB (lihat [Basis data](./databases#how-mariadb-is-tuned)). MySQL: buffer pool-nya ditambah sekitar 150 MB, atau ditambah sekitar 300 MB mulai dari RAM sekitar 3,5 GB (lihat [Basis data](./databases#how-mysql-is-tuned)) |
| | PostgreSQL | sekitar 30 sampai 40 MB saat menganggur, tumbuh hingga `shared_buffers` ditambah sekitar 40 MB (lihat [PostgreSQL](./postgresql#how-postgresql-is-tuned)) |
| | Redis | hingga 64, 128, atau 256 MB, bergantung pada RAM server |
| | Memcached | hingga 64, 128, atau 256 MB, bergantung pada RAM server; beberapa MB saat kosong |
| | Adminer (antarmuka basis data) | tidak ada saat menganggur |
| | phpMyAdmin (antarmuka basis data) | tidak ada saat menganggur |
| **Runtime** | Node.js | tidak ada saat menganggur |
| | Aplikasi Python | tidak ada saat menganggur |
| | Java (OpenJDK) | tidak ada saat menganggur; program Java sebuah situs web aplikasi memakai sebanyak yang dibutuhkan heap-nya |
| **Kontainer** | Docker | sekitar 90 MB, ditambah kontainer Anda |
| **Keamanan** | Firewall web (ModSecurity) | sekitar 40 MB |
| | Antivirus ClamAV | sekitar 15 MB, dan sekitar 1,4 GB memori bebas saat memindai |
| **Alat** | Composer | tidak ada, hanya berjalan saat dipanggil |
| | WP-CLI | tidak ada, hanya berjalan saat dipanggil |

## Menginstal item {#install-an-item}

1. Buka **Perangkat lunak**.
2. Pilih **Instal** di kartu item.
3. Ikuti log-nya. Setelah tugas selesai, kartu menampilkan **Terinstal** dan versinya.

Beberapa kartu lalu menawarkan tautan: **Buka Basis data**, **Buka PostgreSQL**, **Buka Redis**, **Buka Memcached**, **Buka Adminer**, **Buka phpMyAdmin**, atau **Buka Docker**.

## Mencopot item {#uninstall-an-item}

1. Pilih **Copot** di kartu item. Untuk versi PHP, tombol ini ada di menu versi tersebut.
2. Baca apa yang dikatakan dialog tentang hal yang dilakukan pencopotan.
3. Untuk MariaDB, MySQL, PostgreSQL, dan Docker, Anda bisa mencentang kotak untuk ikut menghapus datanya. Anda lalu harus mengetik nama item untuk konfirmasi.
4. Pilih **Copot**.

### Kapan pencopotan ditolak {#when-an-uninstall-is-refused}

Panel tidak pernah menghapus perangkat lunak yang masih dipakai sesuatu. Jika sebuah item tidak bisa dicopot, kartunya menampilkan alasannya alih-alih tombol yang berfungsi.

| Item | Ditolak selama |
|---|---|
| Server web | Ada situs web, di panel atau sebagai file konfigurasi di folder situs server web |
| Sebuah versi PHP | Ada situs web yang memakainya, atau versi itu satu-satunya versi PHP yang terinstal (penyiapan server web dan Adminer membutuhkan satu). Versi yang didukung (7.4 atau lebih baru) juga tetap ada selama versi itu satu-satunya di samping [versi lama](#older-php-versions) |
| Composer | Ada skrip deploy Git yang menjalankan `composer` |
| WP-CLI | Ada situs web WordPress: toolkit WordPress menjalankannya untuk situs-situs itu |
| MariaDB atau MySQL | Menyimpan basis data apa pun selain basis data sistemnya sendiri. Panel bertanya ke server itu sendiri, jadi basis data yang dibuat secara manual atau dengan alat lain juga dihitung. Jika server berhenti dan folder datanya dipindahkan, panel tidak bisa memeriksa dan menolak |
| PostgreSQL | PostgreSQL menyimpan basis data apa pun selain `postgres`, `template0`, dan `template1`, menurut PostgreSQL sendiri, atau PostgreSQL berhenti, sehingga panel tidak bisa memeriksa |
| Node.js | Ada situs web aplikasi yang memakai Node.js bawaan server, atau `node` di `/usr/local/bin` tidak diinstal oleh panel |
| Satu baris di bawah **Versi Node.js** | Baris itu adalah bawaan server, atau dipilih oleh situs web aplikasi |
| Aplikasi Python | Ada situs web aplikasi yang menjalankan Python |
| Java (OpenJDK) | Ada situs web aplikasi yang menjalankan Java. Satu versi Java ditolak selama ada situs web aplikasi yang memakai versi itu |
| Docker | Ada situs web yang menjalankan aplikasi Docker, ada proyek Compose, atau Docker punya kontainer, baik yang berjalan maupun berhenti |

Redis, Memcached, Adminer, phpMyAdmin, ionCube Loader, firewall web, dan ClamAV selalu bisa dicopot.

### Apa yang dipertahankan saat pencopotan {#what-an-uninstall-keeps}

| Item | Dipertahankan | Dihapus |
|---|---|---|
| Server web | Folder situs web di `/var/www`, sertifikat, PHP | Server web, konfigurasi panelnya, Adminer, dan aturan firewall untuk port 80 dan 443 |
| Versi PHP | | Versi itu, ekstensinya, dan library yang hanya dibutuhkan keduanya |
| MariaDB | Folder data `/var/lib/mysql`, kecuali Anda mencentang **Hapus juga direktori data (/var/lib/mysql)** dan mengetik `mariadb`. Menginstal MariaDB lagi akan memakainya | Paket server dan klien, pengaturan MariaDB dari panel, serta repositori MariaDB beserta kuncinya jika MariaDB berasal dari sana |
| MySQL | Folder data `/var/lib/mysql`, kecuali Anda mencentang **Hapus juga direktori data (/var/lib/mysql)** dan mengetik `mysql`. Menginstal MySQL lagi akan memakainya | Paket server dan klien, pengaturan MySQL dari panel, serta repositori Oracle beserta kuncinya |
| PostgreSQL | Data di `/var/lib/postgresql` dan pengaturan cluster di `/etc/postgresql`, kecuali Anda mencentang **Hapus juga data dan pengaturan (/var/lib/postgresql dan /etc/postgresql)** dan mengetik `postgresql`. Paket dihapus tanpa purge, dan menginstal PostgreSQL lagi akan menjalankan cluster yang sama | Paket PostgreSQL, serta repositori apt.postgresql.org beserta kuncinya (`/etc/apt/sources.list.d/panel-pgdg.list` dan `/etc/apt/keyrings/panel-pgdg.gpg`) jika panel yang menambahkannya |
| Redis | | Redis, ekstensi PHP-nya, dan semua yang tersimpan di Redis. PHP-FPM dimulai ulang |
| Memcached | | Memcached, ekstensi PHP-nya, dan semua yang di-cache. PHP-FPM dimulai ulang |
| Adminer | Basis data Anda | File Adminer dan situs internalnya |
| phpMyAdmin | Basis data Anda | File phpMyAdmin, file sementaranya, situs internalnya, dan pengaturannya |
| Node.js | Versi Node.js lain. Folder versi bawaan juga, jika ada situs web aplikasi yang memilih versi itu | `node`, `npm`, dan `npx` di `/usr/local/bin`, serta rilis yang dituju ketiganya |
| Aplikasi Python | `python3` itu sendiri, yang dibutuhkan sistem, dan lingkungan virtual di folder aplikasi lama | Paket venv dan pip |
| Java (OpenJDK) | | Setiap versi Java yang terinstal, beserta paket yang hanya dibutuhkan versi-versi itu |
| ionCube Loader | | Loader di setiap versi PHP dan `/usr/local/lib/ioncube`. PHP-FPM dimuat ulang, dan file PHP yang di-encode berhenti berfungsi |
| Docker | Image, volume, dan build cache di `/var/lib/docker`, kecuali Anda mencentang **Hapus juga semua image, volume, dan build cache (/var/lib/docker)** dan mengetik `docker`. Menginstal Docker lagi akan menemukannya | Docker dan plugin Compose-nya |
| Firewall web | Log firewall | ModSecurity dan aturan OWASP. Server web dimulai ulang |
| ClamAV | | ClamAV dan basis data virusnya |
| Composer | Situs web Laravel dan Drupal, yang hanya membutuhkannya untuk instalasi | `/usr/local/bin/composer` dan paket `composer` dari distribusi |
| WP-CLI | | `/usr/local/lib/panel/wp-cli.phar`. Toolkit WordPress menginstalnya lagi saat membutuhkannya |

::: danger
Menghapus data MariaDB, MySQL, PostgreSQL, atau Docker tidak bisa dibatalkan. Pastikan Anda punya cadangan dari apa pun yang masih Anda butuhkan.
:::

## Menu menampilkan apa yang terinstal {#the-menu-shows-what-is-installed}

Menu kiri menyembunyikan halaman untuk perangkat lunak yang belum terinstal:

- **Docker** muncul setelah Docker terinstal.
- **WordPress** muncul setelah ada situs web WordPress.
- Di halaman **Basis data**, tab **PostgreSQL**, **Redis**, dan **Memcached** muncul setelah perangkat lunak itu terinstal, dan **Kueri lambat** setelah MariaDB atau MySQL terinstal.

**Situs web**, **Basis data**, dan **Perangkat lunak** selalu tampil, karena halaman-halaman itu menawarkan instalasi. Halaman yang sedang Anda buka tetap terlihat.

## Server web {#web-server}

Panel menjalankan satu server web dalam satu waktu dan menyiapkan setiap situs web untuknya:

- Nginx adalah yang paling ringan. PHP berjalan di PHP-FPM.
- Apache membaca file `.htaccess` yang disertakan sebagian aplikasi PHP, dan memakai sedikit lebih banyak memori.
- OpenLiteSpeed membaca aturan rewrite di file `.htaccess` dan sudah dilengkapi LiteSpeed Cache untuk WordPress. OpenLiteSpeed berasal dari repositori paket milik LiteSpeed sendiri.

Menginstal server web juga menginstal versi PHP dari distribusi Anda beserta PHP-FPM. Jika firewall ufw aktif, panel membuka port 80 dan 443, serta 443/udp untuk HTTP/3 dengan OpenLiteSpeed.

Panel menolak menginstal server web kedua selama server web lain terinstal, karena keduanya akan membutuhkan port 80 dan 443.

Untuk beralih ke server web lain:

1. Hapus semua situs web terlebih dahulu. Cadangkan jika Anda membutuhkannya nanti.
2. Pilih **Copot** pada server web yang sekarang dan ketik namanya (`nginx`, `apache`, atau `openlitespeed`) untuk konfirmasi.
3. Instal server web yang lain.

Lihat [Situs web](./websites) untuk menyiapkan situs.

## Versi PHP {#php-versions}

Anda bisa menginstal beberapa versi PHP berdampingan dan memilih satu per situs web di **Situs web** → (situs) → **PHP**.

- Versi yang disertakan distribusi Anda berasal dari paketnya. Versi lain berasal dari repositori Ondřej Surý `packages.sury.org/php`, baik di Ubuntu maupun Debian. Panel bertanya sebelum menambahkan repositori itu ke server, dan hanya menerima kunci penandatanganannya dengan fingerprint `15058500 A0235D97 F5D10063 B188E2B6 95BD4743`.
- Setiap versi diinstal dengan ekstensi berikut: fpm, cli, mysql, curl, gd, mbstring, xml, zip, intl, bcmath, dan opcache. Mulai PHP 8.5, OPcache menjadi bagian dari PHP itu sendiri, jadi 8.5 dan yang lebih baru tidak punya paket opcache tersendiri. Jika Redis atau Memcached terinstal, ekstensi redis atau memcached juga ditambahkan.
- Sebelum PHP 8.0, json adalah paket tersendiri dan juga ditambahkan. PHP 5.6 juga mendapat mcrypt, jika repositori memilikinya.
- Panel mengubah pool PHP-FPM bawaan versi tersebut ke worker sesuai permintaan, jadi versi yang tidak dipakai situs web mana pun hampir tidak memakan memori.

Versi yang tercantum mengikuti repositori PHP. Setelah `packages.sury.org/php` ada di server (repositori ini ditambahkan bersama versi PHP pertama yang bukan milik distribusi Anda), halaman ini menawarkan versi yang disediakan repositori untuk sistem Anda, dibaca dari daftar paket apt. Versi PHP baru, seperti PHP 8.7, muncul dengan sendirinya setelah repositori memublikasikannya dan daftar paket diperbarui (setiap instalasi menjalankan `apt-get update`, begitu pula tugas apt harian sistem Anda). Versi yang tidak ada di repositori tidak ditawarkan. Sebelum repositori ditambahkan, halaman ini menawarkan versi yang dikenal Nefo, 7.4 sampai 8.6. Versi yang terinstal, versi milik distribusi Anda, dan **Versi lama** selalu tercantum.

Pilih **Kelola** pada versi yang terinstal untuk membuka **Ekstensi** dan **Pengaturan**-nya.

### Versi tanpa pembaruan keamanan {#end-of-life}

Setiap versi PHP mendapat pembaruan keamanan dari PHP sampai tanggal tertentu (lihat [php.net/supported-versions.php](https://www.php.net/supported-versions.php)): 8.1 sampai 31 Desember 2025, 8.2 sampai 31 Desember 2026, 8.3 sampai 31 Desember 2027, 8.4 sampai 31 Desember 2028, 8.5 sampai 31 Desember 2029, dan 8.6 sampai 31 Desember 2030. 7.4 dan 8.0 sudah tidak mendapatkannya lagi. Setelah tanggal itu, versi dari `packages.sury.org` menampilkan lencana **Akhir masa dukungan — tanpa pembaruan keamanan** dan satu baris berisi tanggalnya, di halaman Perangkat lunak, di jendela **Kelola**-nya, dan di menu versi PHP situs web. Panel membandingkan tanggal itu dengan hari ini, jadi lencana muncul dengan sendirinya.

Versi PHP milik distribusi Anda sendiri (8.1 di Ubuntu 22.04, 8.3 di 24.04, 8.5 di 26.04, 8.2 di Debian 12, 8.4 di Debian 13) tidak pernah menampilkan lencana itu: tim keamanan distribusi terus menambalnya selama rilis tersebut masih didukung.

Pindahkan situs web dari versi berlencana itu jika memungkinkan. Anda tetap bisa menginstalnya tanpa konfirmasi tambahan; hanya [versi lama](#older-php-versions) yang meminta konfirmasi.

### Versi PHP lama {#older-php-versions}

PHP 5.6, 7.0, 7.1, 7.2, dan 7.3 ada di bawah **Versi lama**, terlipat di bawah versi lainnya. Versi-versi ini tersedia untuk situs web yang dipindahkan dari server lain (cPanel, aaPanel…) yang belum bisa berjalan di PHP yang lebih baru.

::: danger Akhir masa dukungan: tanpa pembaruan keamanan
PHP sudah menghentikan pembaruan keamanan versi-versi ini bertahun-tahun lalu: 5.6 pada 2018, 7.3 pada 2021 (lihat [php.net/eol.php](https://www.php.net/eol.php)). Kerentanan yang sudah diketahui tidak akan pernah diperbaiki, jadi situs web yang berjalan di salah satunya lebih mudah dibobol. Gunakan hanya untuk menjaga situs web lama tetap berjalan selama Anda memperbaruinya, lalu pindahkan ke versi yang didukung. Jangan gunakan untuk situs web baru.
:::

- Setiap versi menampilkan lencana **Akhir masa dukungan — tanpa pembaruan keamanan**, di halaman Perangkat lunak dan di menu versi PHP situs web.
- Menginstal salah satunya meminta Anda mengonfirmasi risiko itu terlebih dahulu.
- Versi yang didukung (7.4 atau lebih baru) harus diinstal lebih dulu, dan versi didukung yang terakhir tidak bisa dicopot selama versi lama masih terinstal. Jadi versi lama tidak pernah menjadi satu-satunya PHP di server: penyiapan server web, Adminer, Composer, dan alat-alat panel tetap memakai versi yang didukung.
- Toko aplikasi punya versi minimumnya sendiri: WordPress membutuhkan PHP 7.4, dan aplikasi lain lebih tinggi. Toolkit WordPress tetap mengelola situs web WordPress di versi lama, tetapi tidak memperbarui WordPress itu sendiri di sana, karena WordPress saat ini membutuhkan PHP 7.4. Toolkit tetap memperbarui plugin dan tema; WP-CLI melewati pembaruan yang membutuhkan PHP yang lebih baru.
- Pool PHP-FPM situs web dan pengaturan php.ini sama seperti di versi yang lebih baru: setiap pengaturan yang ditulis panel ada di PHP 5.6.

`packages.sury.org/php` menyediakan PHP 5.6 sampai 7.3 untuk Ubuntu 22.04, 24.04, dan 26.04 serta Debian 12 dan 13, di amd64 dan arm64. Instalasi juga memeriksa dengan apt, dan menolak dengan alasan jika repositori tidak punya versi itu.

### Server yang dulu memakai PPA ondrej/php {#ondrej-php-ppa}

Versi panel sebelumnya mengambil versi PHP tambahan di Ubuntu dari `ppa:ondrej/php`. PPA itu dibekukan dan tidak lagi menerima pembaruan: pengelolanya sekarang memublikasikan paket Ubuntu di `packages.sury.org/php` (lihat pemberitahuan di [launchpad.net/~ondrej/+archive/ubuntu/php](https://launchpad.net/~ondrej/+archive/ubuntu/php)).

Saat panel berjalan di server Ubuntu yang masih punya PPA itu, panel memindahkan server sekali, dengan cara yang dijelaskan pemberitahuan PPA:

1. Panel menambahkan `packages.sury.org/php` dengan kuncinya yang dipatok (`/usr/share/keyrings/sury-php.gpg` dan `/etc/apt/sources.list.d/sury-php.list`), kecuali sumber untuknya sudah ada.
2. Panel memindahkan file sumber PPA dari `/etc/apt/sources.list.d`, dan kunci PPA `/etc/apt/trusted.gpg.d/ondrej-ubuntu-php.gpg` jika ada, ke `apt-backup` di folder data panel (`/var/lib/nefo/apt-backup`). Kunci itu dipercaya untuk setiap repositori dan tidak ada lagi yang membutuhkannya. PPA lain, bahkan dari pengelola yang sama, dibiarkan.

Tidak ada yang diinstal, dihapus, atau diturunkan versinya, dan panel tidak menjalankan `apt-get update` saat dijalankan. Pembaruan berikutnya (pemeriksaan pembaruan oleh panel, sebuah instalasi, atau `apt update` Anda sendiri) membaca sumber baru, dan paket PHP yang terinstal mendapat pembaruan berikutnya dari sana. Log panel menyebutkan apa yang diubahnya. Jika kunci tidak bisa diunduh, PPA tetap ada dan panel mencoba lagi saat dijalankan berikutnya. Baris PPA di `/etc/apt/sources.list` itu sendiri, atau di file yang juga mencantumkan repositori lain, tidak diedit: log meminta Anda menggantinya secara manual.

### Ekstensi {#extensions}

Tab **Ekstensi** mencantumkan ekstensi opsional yang punya paket untuk versi ini. Pilih **Instal** atau **Hapus**, dan PHP-FPM dimuat ulang.

Ekstensi opsionalnya adalah: apcu, gmp, igbinary, imagick, imap, ldap, memcached, mongodb, msgpack, pgsql, redis, soap, sqlite3, ssh2, tidy, xdebug, xsl, dan yaml.

::: warning
xdebug memperlambat setiap situs di versi PHP tersebut. Jangan biarkan terinstal di server produksi.
:::

### Pengaturan php.ini {#php-ini-settings}

Tab **Pengaturan** mengatur nilai php.ini untuk setiap situs web di versi PHP ini. Kosongkan kolom untuk memakai nilai bawaan PHP, lalu pilih **Simpan**.

| Pengaturan | Bawaan | Nilai yang diizinkan |
|---|---|---|
| `date.timezone` | UTC | Nama zona waktu seperti `Europe/Berlin` |
| `max_input_vars` | 1000 | 100 sampai 100000 |
| `max_input_time` | 60 | -1 sampai 3600 |
| `default_socket_timeout` | 60 | 1 sampai 600 |
| `session.gc_maxlifetime` | 1440 | 60 sampai 2592000 |
| `realpath_cache_size` | 4096K | Angka, opsional diikuti K atau M |
| `display_errors` | Off | On atau Off |
| `expose_php` | Off | On atau Off |
| `allow_url_fopen` | On | On atau Off |
| `short_open_tag` | Off | On atau Off |
| `opcache.enable` | 1 | 1 atau 0 |
| `opcache.memory_consumption` | 128 | 32 sampai 2048 |
| `opcache.max_accelerated_files` | 10000 | 1000 sampai 1000000 |
| `opcache.validate_timestamps` | 1 | 1 atau 0 |
| `opcache.revalidate_freq` | 2 | 0 sampai 3600 |

Nilai-nilai ini ditulis ke `/etc/php/<version>/fpm/conf.d/99-panel.ini` dan diperiksa dengan `php-fpm -t`. Jika PHP-FPM menolaknya, file sebelumnya dikembalikan dan tidak ada yang berubah. Batas memori, ukuran unggahan, dan waktu diatur per situs web, di pengaturan situs web.

### ionCube Loader {#ioncube}

Kartu **ionCube Loader**, di bawah versi PHP, menjalankan file PHP yang di-encode dengan ionCube Encoder. Sebagian skrip PHP komersial membutuhkannya, seperti WHMCS dan perangkat lunak penagihan atau lisensi lainnya, dan situs web yang dipindahkan dari cPanel sering membutuhkannya.

1. Instal dulu versi PHP yang dipakai situs web Anda.
2. Di kartu **ionCube Loader**, pilih **Instal**.

Panel mengunduh loader dari ionCube (`downloads.ioncube.com`) lewat HTTPS. Panel menginstal satu rilis yang dipatok, 15.5.1, dan memeriksa unduhan terhadap checksum SHA-256 yang dimiliki panel untuknya, karena ionCube tidak memublikasikan checksum atau tanda tangan. Unduhan yang tidak cocok ditolak dan tidak ada yang berubah. Ini berfungsi di server amd64 dan arm64.

Loader ditempatkan di `/usr/local/lib/ioncube`, bersama lisensi ionCube (`LICENSE.txt`). Untuk setiap versi PHP terinstal yang didukung loader, panel menulis `00-ioncube.ini` ke `/etc/php/<version>/fpm/conf.d` dan `/etc/php/<version>/cli/conf.d`, agar loader dimuat sebelum OPcache, sesuai syarat ionCube. Panel memeriksa bahwa `php<version> -v` berjalan dengan loader, lalu memuat ulang PHP-FPM. Jika PHP tidak mau berjalan dengan loader, panel mengeluarkan file itu lagi, sehingga versi tersebut tetap berfungsi tanpa loader, dan tugas menyebutkan versi mana yang gagal.

Kartu ini mencantumkan setiap versi PHP yang terinstal beserta apa yang dilakukan loader untuknya:

| Keadaan | Arti |
|---|---|
| **dimuat** | Loader dimuat untuk PHP-FPM dan baris perintah |
| **tidak dimuat** | Ada loader untuk versi ini, tetapi belum dimuat. Pilih **Muat ke setiap versi** di kartu |
| **ionCube tidak membuat loader untuk versi ini** | PHP 8.0: ionCube tidak pernah memublikasikan loader untuknya |
| "belum tersedia untuk PHP 8.6" | Versi PHP yang lebih baru daripada loader yang diinstal panel (15.5.1 mendukung sampai PHP 8.5) |

Rilis yang dipatok punya loader untuk PHP 5.6, 7.0 sampai 7.4, dan 8.1 sampai 8.5. Versi PHP yang Anda instal kemudian juga mendapat loader, jika ada loader untuknya. Mencopot versi PHP menghapus `00-ioncube.ini`-nya.

**Copot** mengeluarkan loader dari setiap versi PHP, memuat ulang PHP-FPM, dan menghapus `/usr/local/lib/ioncube`. File PHP yang di-encode lalu berhenti berfungsi.

Untuk memeriksa sebuah situs web, taruh file seperti ini di foldernya dan buka, lalu hapus file itu:

```php
<?php var_dump(extension_loaded('ionCube Loader'));
```

::: tip
ionCube Loader gratis dipakai tetapi bukan sumber terbuka. Lisensi ionCube ([ioncube.com/legal/loaders/eula](https://www.ioncube.com/legal/loaders/eula)) mengizinkan pemakaiannya tanpa biaya. Panel mengunduhnya dari ionCube saat Anda menginstalnya dan tidak menyertakannya.
:::

## Node.js {#node-js}

Kartu **Node.js** menginstal rilis LTS terbaru dari nodejs.org, yang diperiksa terhadap checksum SHA-256 yang dipublikasikan. Rilis itu dipasang ke `/opt/node-v<version>-linux-<arch>`, dan `node`, `npm`, serta `npx` ditautkan ke `/usr/local/bin`. Inilah Node.js bawaan server, yang dipakai situs web aplikasi yang tidak memilih versi.

### Beberapa versi Node.js {#several-node-js-versions}

Setelah Node.js terinstal, kartu **Versi Node.js** mencantumkan tiga jalur LTS terbaru dan jalur apa pun yang terinstal. Pilih **Instal** untuk menambahkan satu jalur di samping yang lain. Ini tidak mengubah versi bawaan.

Pilih jalur untuk situs web aplikasi di **Situs web** → (situs) → **Runtime**. Jalur yang dipakai situs web aplikasi, dan versi bawaan, tidak bisa dicopot dari kartu ini. Copot versi bawaan dari kartu **Node.js** sebagai gantinya.

Daftar ini berasal dari nodejs.org, jadi server membutuhkan akses internet untuk menampilkannya.

::: warning
Mencopot Node.js bawaan juga merusak tugas cron dan program latar belakang yang memanggil `node`.
:::

## Python {#python}

Kartu **Aplikasi Python** menginstal `python3-venv` dan `python3-pip`, agar situs web aplikasi bisa menjalankan Python (Django, Flask, FastAPI…). Setiap situs web aplikasi Python mendapat lingkungan virtualnya sendiri di `app/.venv`. Alat build seperti `build-essential` dan `python3-dev` tidak diinstal, jadi paket yang tidak punya wheel siap pakai dan harus dikompilasi akan gagal diinstal sampai Anda menambahkan alat-alat itu dengan `apt`.

Pencopotan menghapus venv dan pip. `python3` itu sendiri tetap ada, karena sistem membutuhkannya.

## Java {#java}

Kartu **Java (OpenJDK)** menginstal OpenJDK dari distribusi Anda, untuk situs web aplikasi yang ditulis dalam Java, seperti aplikasi Spring Boot yang dijalankan dengan `java -jar`. Yang diinstal adalah JDK headless (`openjdk-<version>-jdk-headless`): `java`, ditambah `javac`, `jar`, dan alat lain yang dibutuhkan build dalam skrip [deploy Git](./git-deploy), tanpa bagian desktop. JDK ini dipasang ke `/usr/lib/jvm/java-<version>-openjdk-<arch>` dan mendapat pembaruan keamanan bersama bagian sistem lainnya.

**Instal** di kartu menginstal versi dukungan jangka panjang (LTS) terbaru yang ada di distribusi Anda. Versi yang tersedia bergantung pada sistem:

| Sistem | Versi Java |
|---|---|
| Ubuntu 22.04 | 17, 21, 25 |
| Ubuntu 24.04 | 17, 21, 25 |
| Ubuntu 26.04 | 17, 21, 25 |
| Debian 12 | 17 |
| Debian 13 | 21, 25 |

### Beberapa versi Java {#several-java-versions}

Setelah Java terinstal, kartu **Versi Java** mencantumkan versi LTS yang ditawarkan distribusi Anda dan versi apa pun yang terinstal. Pilih **Instal** untuk menambahkan satu versi di samping yang lain. Versi LTS yang lebih baru muncul dengan sendirinya setelah distribusi menyediakannya.

Pilih versi untuk situs web aplikasi di **Situs web** → (situs) → **Runtime** (lihat [Situs web aplikasi Java](./websites#java-app-websites)). Versi yang dipakai situs web aplikasi tidak bisa dicopot, dan kartu menunjukkan bahwa ada aplikasi yang memakainya. **Copot** di kartu **Java (OpenJDK)** menghapus setiap versi yang terinstal, dan ditolak selama ada situs web aplikasi yang menjalankan Java.

Setiap versi memakan sekitar 390 sampai 480 MB disk, termasuk paket yang diunduh. Tidak ada yang berjalan selama tidak ada situs web yang memakainya.

## Redis {#redis}

Redis adalah cache di memori. WordPress bisa memakainya dengan plugin object cache Redis.

Saat Anda menginstal Redis, panel:

- menginstal `redis-server` dan ekstensi redis untuk setiap versi PHP yang terinstal;
- membatasi memori Redis sesuai server: 64 MB di bawah RAM 1,5 GB, 128 MB mulai dari 1,5 GB, 256 MB mulai dari 3 GB;
- mengatur kebijakan `allkeys-lru`, jadi Redis membuang kunci yang paling lama tidak dipakai saat penuh, seperti cache;
- membiarkan Redis mendengarkan di `127.0.0.1:6379`, yang hanya bisa dijangkau dari server ini.

Untuk melihat isi Redis, buka **Basis data** → **Redis**. Lihat [Basis data](./databases#redis-key-browser).

Sebelum Anda mencopot Redis, nonaktifkan plugin cache di situs web Anda. Situs tetap berfungsi tanpa cache, hanya sedikit lebih lambat.

## PostgreSQL {#postgresql}

Kartu **PostgreSQL** menginstal versi milik distribusi Anda, atau versi mayor lain dari repositori proyek PostgreSQL (apt.postgresql.org), sesuai pilihan Anda di dialog instalasi. Panel menyetelnya sesuai memori server dalam drop-in `conf.d`, membuatnya hanya mendengarkan di localhost, dan menulis `pg_hba.conf` yang meminta kata sandi dari semua pihak kecuali pengguna sistem `postgres` di socket lokal. Basis datanya ada di **Basis data** → **PostgreSQL**. Lihat [PostgreSQL](./postgresql) untuk versi, peningkatan versi, akses jarak jauh, aturan impor, dan pencadangan.

Jika berasal dari apt.postgresql.org, kartu menampilkan **Dari apt.postgresql.org**; jika tidak, **Dari distribusi**. Mencopotnya juga menghapus repositori itu, jika panel yang menambahkannya.

Jika Adminer terinstal, panel menambahkan ekstensi PHP `pgsql` untuk versi PHP Adminer, agar Adminer juga bisa membuka basis data PostgreSQL. Ekstensi ini hanya ditambahkan bersama PostgreSQL.

## Memcached {#memcached}

Memcached adalah cache kecil di memori, untuk sesi PHP dan cache aplikasi (misalnya driver cache `memcached` milik Laravel).

Saat Anda menginstal Memcached, panel:

- menginstal `memcached` dan ekstensi memcached untuk setiap versi PHP yang terinstal, dan menambahkan ekstensi itu ke versi PHP yang Anda instal kemudian;
- menulis `/etc/memcached.conf`: Memcached hanya mendengarkan di `127.0.0.1:11211`, lewat TCP, dengan UDP nonaktif (`-U 0`), jadi tidak bisa dipakai untuk memperkuat serangan;
- membatasi memorinya sesuai server: 64 MB di bawah RAM 1,5 GB, 128 MB mulai dari 1,5 GB, 256 MB mulai dari 3 GB, dan mengizinkan 512 koneksi di bawah RAM 1,5 GB, 1024 mulai dari situ.

Saat memori penuh, Memcached membuang item yang paling lama tidak dipakai. Untuk melihat angka-angkanya, buka **Basis data** → **Memcached**.

Sebelum Anda mencopot Memcached, arahkan sesi dan cache situs web Anda ke tempat lain: aplikasi yang driver cache-nya Memcached akan gagal tanpanya.

## Server basis data, Adminer, dan phpMyAdmin {#mariadb-and-adminer}

Kartu **Server basis data** menginstal MariaDB atau MySQL 8.4 LTS, yang disetel sesuai memori server dan hanya bisa dijangkau dari server ini. Satu server punya salah satunya.

Selama belum ada yang terinstal, kartu menawarkan pilihan, dengan **MariaDB** terpilih: MariaDB lebih ringan dan menjadi dasar WordPress serta sebagian besar aplikasi PHP.

- Di bawah **MariaDB**, Anda memilih versi jika repositori MariaDB sendiri punya seri LTS yang lebih baru daripada distribusi Anda: milik distribusi Anda (bawaan, misalnya "10.11, milik distribusi"), atau **11.4 LTS dari MariaDB**, **11.8 LTS dari MariaDB**, atau **12.3 LTS dari MariaDB** (di Ubuntu 26.04 dan Debian 13, hanya 12.3). Masing-masing menyebutkan sampai kapan MariaDB mendukungnya. Lihat [Versi MariaDB](./databases#mariadb-versions).
- **MySQL 8.4 LTS** berasal dari repositori Oracle dan ditawarkan di Ubuntu 22.04, 24.04, dan 26.04 serta Debian 12 dan 13, hanya di server amd64. Di tempat lain, kartu menyebutkan alasan MySQL tidak ditawarkan.

Setelah salah satunya terinstal, kartu menampilkan yang mana, versinya dan asalnya (**dari paket distribusi**, **dari repositori MariaDB**, atau **dari repositori Oracle**), perkiraan memorinya, jumlah basis data, **Buka Basis data**, dan **Copot**. Kartu juga menawarkan:

- **Tingkatkan versi**, pada MariaDB, jika ada seri LTS yang lebih baru: lihat [Meningkatkan versi MariaDB](./databases#upgrade-mariadb).
- Server yang lain sebagai alternatif, **MySQL 8.4 LTS sebagai gantinya** atau **MariaDB sebagai gantinya**, dengan tombol untuk beralih: panel memindahkan basis data, penggunanya, dan kata sandinya, lalu kembali ke keadaan semula jika ada yang gagal. Lihat [Beralih antara MariaDB dan MySQL](./databases#switch-between-mariadb-and-mysql).
- **Dump yang disimpan**: ekspor yang dibuat oleh peningkatan versi atau peralihan, dengan folder, ukuran, dan tombol untuk menghapusnya. Lihat [Dump dari peningkatan versi dan peralihan](./databases#dumps-from-upgrades-and-switches).

Adminer dan phpMyAdmin menampilkan basis data Anda di browser dan hanya bisa dijangkau melalui halaman masuk panel. Keduanya membutuhkan server web dan PHP, dan keduanya bekerja dengan MariaDB dan MySQL. Lihat [Basis data](./databases#phpmyadmin).

## Docker {#docker}

Kartu **Docker** menginstal `docker.io` dari distribusi Anda, beserta plugin Compose-nya jika tersedia. Item menu **Docker** lalu muncul.

Port yang dipublikasikan kontainer Docker melewati firewall ufw. Karena itu panel memublikasikan port kontainer hanya di 127.0.0.1, dan Anda menjangkaunya melalui situs web. Lihat [Docker](./docker).

## Firewall web dan ClamAV {#web-firewall-and-clamav}

- **Firewall web (ModSecurity)** menginstal ModSecurity dengan OWASP Core Rule Set. Fitur ini membutuhkan server web. Setelah instalasi, aktifkan per situs web di **Situs web** → (situs) → **Firewall web (WAF)**. Jika distribusi tidak punya modul ModSecurity untuk server web, kartu menyebutkan alasannya dan tidak menawarkan instalasi: Ubuntu 22.04 tidak punya modul untuk Nginx. Apache dan OpenLiteSpeed berfungsi di sana, dan Nginx berfungsi mulai Ubuntu 24.04.
- **Antivirus ClamAV** adalah mesin tambahan opsional untuk pemindaian malware. Basis data virusnya diunduh tepat setelah instalasi, jadi pemindaian bisa memakainya beberapa menit kemudian. ClamAV cocok untuk server dengan RAM 2 GB atau lebih.

Lihat [Keamanan](./security) untuk keduanya.

## Alat: Composer dan WP-CLI {#tools}

Kelompok **Alat** menampilkan dua alat baris perintah yang diinstal panel saat ada yang membutuhkannya. Setiap kartu menampilkan apakah alat itu terinstal, versinya, asalnya, dan apa yang memakainya. **Perbarui** menginstal rilis resmi terbaru alat tersebut. Panel mengunduhnya lewat HTTPS dengan checksum yang dipublikasikan proyek di sebelahnya, dan menolak unduhan yang tidak cocok: alat yang terinstal lalu tetap seperti semula. File baru mengganti file lama dalam satu langkah.

### Composer {#composer}

Composer adalah pengelola paket PHP. Menginstal Laravel atau Drupal dari Toko aplikasi juga menginstalnya, dan skrip deploy Git serta terminal juga bisa menjalankannya.

- **Instal** menginstal paket `composer` dari distribusi Anda, yang mendapat pembaruan keamanan bersama bagian sistem lainnya.
- **Perbarui** menginstal rilis stabil terbaru dari getcomposer.org ke `/usr/local/bin/composer`, yang diperiksa terhadap checksum SHA-256 yang dipublikasikan. Versi itu muncul lebih dulu di `PATH`, jadi `composer` di terminal, skrip deploy Git, dan instalasi Laravel dan Drupal oleh panel sendiri semuanya memakainya. Paket dari distribusi tetap terinstal.
- **Copot** menghapus keduanya. Pencopotan ditolak selama ada skrip deploy Git yang menjalankan `composer`. Situs web Laravel dan Drupal tidak menghalanginya: situs-situs itu hanya membutuhkan Composer untuk instalasi, dan tetap berfungsi tanpanya. Menginstal salah satunya lagi akan mengembalikan Composer.

### WP-CLI {#wp-cli}

WP-CLI menjalankan WordPress dari baris perintah. [Toolkit WordPress](./wordpress) memakainya untuk pembaruan, mode pemeliharaan, kata sandi, dan salinan staging, dan menginstalnya saat pertama kali membutuhkannya.

- **Instal** dan **Perbarui** mengunduh rilis terbaru dari GitHub ke `/usr/local/lib/panel/wp-cli.phar`, yang diperiksa terhadap checksum SHA-512 yang dipublikasikan rilis tersebut.
- **Copot** ditolak selama ada situs web WordPress.
