---
title: Pencadangan
description: Cadangkan situs web, basis data, folder, dan aplikasi sesuai jadwal dengan retensi, ke server ini, penyimpanan yang kompatibel dengan S3, server SFTP, atau Google Drive, terenkripsi jika Anda mau, lalu pulihkan dengan satu klik, juga di server baru.
---

# Pencadangan

Nefo mencadangkan situs web, basis data, folder, dan aplikasi dari Toko aplikasi sebagai arsip `.tar.gz`, secara manual atau sesuai jadwal harian atau mingguan dengan retensi. Setiap cadangan disimpan di server, dan salinannya bisa diunggah ke penyimpanan yang kompatibel dengan S3, server SFTP, atau Google Drive, terenkripsi dengan passphrase jika Anda mau. Untuk memulihkan, Anda hanya butuh cadangannya dan mengetik namanya sebagai konfirmasi, juga di server baru.

Halaman ini membahas isi cadangan, pencadangan manual dan terjadwal, penyimpanan jarak jauh dan enkripsinya, pemulihan, serta peringatan saat pencadangan gagal. Untuk memulihkan di server lain, lihat [Memulihkan di server baru](./restore-new-server).

## Isi cadangan {#what-a-backup-contains}

| Jenis | Isi |
|---|---|
| **Situs web** | File di folder situs web, `/var/www/example.com`, kecuali folder `tmp` dan `logs` di tingkat teratasnya, serta dump SQL dari setiap basis data yang tertaut ke situs web |
| **Basis data** | Dump SQL dari satu basis data |
| **Folder** | Folder mana pun di server, seperti `/etc/nginx` atau `/opt/app`, beserta pemilik, izin, dan tautan simbolik |
| **Aplikasi** | Aplikasi Docker dari Toko aplikasi: foldernya di `/var/lib/panel-apps`, kata sandinya yang tersimpan, data kontainernya, dan dump basis datanya (lihat [Cadangan aplikasi](#app-backups)) |

Tautkan basis data ke situs webnya saat Anda membuatnya (lihat [Basis data](./databases#create-a-database)), agar cadangan situs web menyertakannya. Cadangan situs web tidak menyertakan tautan simbolik dan file khusus.

Setiap cadangan adalah arsip `.tar.gz` berisi `manifest.json`, file-file di bawah `files/`, dan dump basis data di bawah `databases/`. Dump basis data dibuat dengan `mariadb-dump --single-transaction`, jadi situs web tetap berjalan.

`manifest.json` dari cadangan situs web juga mencatat apa yang dibutuhkan untuk membuat situs web itu lagi di server lain: alias, versi PHP, aturan rewrite, direktif server web miliknya sendiri, dan kata sandi pengguna basis datanya.

Cadangan di server ini tidak dienkripsi. Cadangan hanya bisa dibaca root. Setiap penyimpanan jarak jauh bisa mengenkripsi salinannya dengan passphrase sebelum salinan itu meninggalkan server (lihat [Mengenkripsi salinan](#encrypt-the-copies)).

## Tempat cadangan disimpan {#where-backups-are-kept}

Setiap cadangan disimpan lebih dulu di server ini, di `/var/backups/panel`:

```
/var/backups/panel/site/example.com/example.com_20261126-030000.tar.gz
/var/backups/panel/database/shop/shop_20261126-030000.tar.gz
/var/backups/panel/folder/etc_nginx-1a2b3c4d/etc_nginx_20261126-030000.tar.gz
```

Folder-foldernya punya mode `0700` dan file-filenya `0600`. Panel juga mencatat checksum SHA-256 dari setiap arsip. Cadangan lokal memakan ruang disk, jadi pantau disk dan berapa banyak cadangan yang disimpan jadwal Anda.

Untuk menyalin cadangan ke komputer Anda, buka menunya dan pilih **Unduh**.

## Mencadangkan sekarang {#back-up-now}

1. Buka **Pencadangan** dan pilih **Cadangkan sekarang**.
2. Pilih **Situs web**, **Basis data**, **Folder**, atau **Aplikasi**. **Aplikasi** muncul setelah ada aplikasi dari Toko aplikasi yang terinstal.
3. Pilih situs web, basis data, atau aplikasi di **Cadangan dari**, atau ketik **Path folder** seperti `/etc/nginx`. Folder yang pernah dicadangkan akan disarankan.
4. Jika penyimpanan jarak jauh sudah disiapkan, Anda bisa mencentang **Unggah juga ke penyimpanan jarak jauh**.
5. Pilih **Cadangkan sekarang**.

Log progres menampilkan setiap langkah. Panel menjalankan satu tugas panjang dalam satu waktu, jadi tunggu tugas yang sedang berjalan selesai lebih dulu.

Untuk situs web, Anda juga bisa membuka **Situs web** → (situs) → **Pencadangan** lalu memilih **Cadangkan sekarang**.

### Folder yang tidak bisa dicadangkan {#folders-that-can-t-be-backed-up}

Panel menolak folder-folder ini, folder apa pun di dalamnya, dan folder apa pun yang memuat salah satunya. Tautan simbolik diikuti sebelum pemeriksaan.

- `/`, seluruh sistem;
- `/proc`, `/sys`, `/dev`, dan `/run`;
- data panel, `/var/lib/nefo`, dan cadangannya, `/var/backups/panel`;
- `/var/lib/panel-sftp`, `/var/lib/panel-acme`, dan `/var/lib/panel-quarantine`.

Jadi `/var` dan `/var/lib` tidak bisa dicadangkan secara keseluruhan, tetapi `/var/www` atau `/etc` bisa.

## Pencadangan otomatis {#automatic-backups}

1. Buka **Pencadangan** dan pilih **Tambah jadwal**.
2. Pilih **Situs web**, **Basis data**, **Folder**, atau **Aplikasi**, serta targetnya.
3. Di **Kapan**, pilih **Setiap hari** atau **Setiap minggu**. Untuk pencadangan mingguan, pilih **Hari**.
4. Pilih **Jam**. Ini waktu server, dan formulir menampilkan selisih UTC-nya.
5. Masukkan **Jumlah cadangan yang disimpan**, dari 1 sampai 365.
6. Opsional: centang **Unggah juga ke penyimpanan jarak jauh**.
7. Pilih **Simpan jadwal**.

Untuk situs web ada jalan pintas: **Situs web** → (situs) → **Pencadangan** → **Setiap hari pukul 03:00**. Jalan pintas ini menyimpan 7 cadangan.

Tab **Cadangan otomatis** menampilkan jadwal beserta eksekusi terakhirnya, **Selesai** atau **Gagal**. Buka **Eksekusi terakhir gagal** untuk membaca kesalahannya.

Cara jadwal berjalan:

- Panel memeriksa setiap menit. Jika ada tugas lain yang sedang berjalan, pencadangan menunggu menit berikutnya.
- Jika server mati pada jam yang direncanakan, pencadangan berjalan setelah panel berjalan lagi.
- Jadwal baru menunggu jam pertama yang direncanakan. Untuk langsung mendapatkan cadangan, gunakan juga **Cadangkan sekarang**.

### Retensi {#retention}

Setelah setiap pencadangan terjadwal, panel menghapus cadangan tertua buatan jadwal itu yang melebihi jumlah yang disimpan, di server ini dan di penyimpanan jarak jauh. Cadangan yang Anda buat secara manual tidak pernah dihapus otomatis.

Cadangan yang dibuat Nefo sebelum menghapus situs web (**Cadangkan situs web dulu**) dihitung sebagai cadangan manual: cadangan ini tidak pernah dihapus otomatis. Namanya berakhiran `_before-delete_<date>-<time>.tar.gz`.

Menghapus jadwal akan menghentikannya. Cadangan yang sudah ada tetap tersimpan. Menghapus situs web, basis data, atau aplikasi juga menghapus jadwalnya, dan cadangannya tetap ada di halaman **Pencadangan**.

## Penyimpanan jarak jauh {#remote-storage}

Salinan di penyedia lain melindungi Anda jika server ini hilang. Buka **Pencadangan** → **Penyimpanan jarak jauh** dan pilih **Siapkan** (atau **Tambah penyimpanan** jika sudah ada satu penyimpanan). Anda bisa menyiapkan penyimpanan yang kompatibel dengan S3, server SFTP, dan Google Drive sekaligus: cadangan lalu diunggah ke masing-masing.

### Penyimpanan yang kompatibel dengan S3 {#s3-compatible-storage}

Pilih **Penyimpanan yang kompatibel dengan S3** dan isi:

| Kolom | Apa yang dimasukkan |
|---|---|
| **Endpoint (https://…)** | Alamat S3 penyedia, `https://` dan host saja, tanpa path. Misalnya `https://s3.eu-central-1.amazonaws.com` |
| **Bucket** | Nama bucket |
| **Wilayah** | Wilayah bucket. Kosong berarti `us-east-1`. Cloudflare R2 memakai `auto` |
| **Folder di bucket (opsional)** | Prefiks seperti `panel/` |
| **Access key ID** | |
| **Secret access key** | Saat Anda mengubah pengaturan nanti, kosongkan untuk mempertahankan yang sekarang |

Ini berfungsi dengan Amazon S3, Cloudflare R2, Backblaze B2, Wasabi, dan MinIO. Gunakan access key yang hanya boleh memakai bucket ini.

Pilih **Uji dan simpan**. Panel menulis file uji kecil, membacanya kembali, dan menghapusnya sebelum menyimpan pengaturan.

Satu cadangan bisa paling besar 5 GB di penyimpanan S3, yaitu batas satu kali unggahan.

### Server SFTP {#sftp-server}

Gunakan server lain yang bisa Anda masuki lewat SFTP (SSH), seperti storage box atau VPS kedua, dengan akun yang khusus untuk cadangan.

Pilih **Server SFTP** dan isi:

| Kolom | Apa yang dimasukkan |
|---|---|
| **Server** | Nama host atau alamat IP, seperti `backup.example.com` |
| **Port** | Biasanya `22` |
| **Nama pengguna** | Akun di server cadangan |
| **Masuk dengan** | **Kata sandi**, atau **Kunci privat** |
| **Kunci privat (OpenSSH atau PEM)** | Seluruh kunci, dari `-----BEGIN` sampai `END-----`. Kunci publiknya harus ada di `~/.ssh/authorized_keys` di server cadangan |
| **Passphrase (hanya untuk kunci terenkripsi)** | |
| **Folder di server (opsional)** | Relatif terhadap folder home akun, kecuali diawali `/`. Folder dibuat jika diperlukan |

Pilih **Uji dan simpan**. Panel terhubung, menulis, menampilkan daftar, membaca, dan menghapus file uji, lalu menyimpan pengaturan. Setelah kunci disimpan, **Ubah** menampilkan bagian publiknya di bawah **Kunci publik, untuk ~/.ssh/authorized_keys di server cadangan**, jadi Anda bisa menyalinnya lagi.

### Penyematan host key {#host-key-pinning}

**Uji dan simpan** memercayai host key yang ditunjukkan server SFTP saat itu, dan menampilkannya di bawah **Host key tepercaya**. Setiap koneksi berikutnya harus menunjukkan kunci yang persis sama, atau unggahan cadangan gagal.

Sebelum Anda mengandalkan penyimpanan ini, bandingkan fingerprint-nya dengan kunci milik server cadangan itu sendiri. Di server cadangan, jalankan:

```sh
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
```

Jika server cadangan diinstal ulang atau kuncinya sengaja diganti, pilih **Ubah** lalu **Uji dan simpan** lagi untuk memercayai kunci baru. Panel lalu memperingatkan bahwa kuncinya sudah diganti.

::: warning
Host key yang berubah juga bisa berarti ada seseorang yang menyadap koneksi. Jika Anda tidak menduga perubahan itu, periksa server cadangan sebelum memercayai kunci baru.
:::

### Bisakah Nefo mencadangkan ke Google Drive? {#google-drive}

Bisa, sejak v0.24.0. Cadangan masuk ke sebuah folder di Google Drive Anda. Panel hanya meminta akses ke file yang dibuatnya sendiri (izin `drive.file` dari Google): panel tidak bisa melihat atau mengubah apa pun yang lain di Drive Anda. Folder yang Anda buat sendiri di Drive tidak terlihat oleh panel, meskipun namanya benar.

Google Drive membutuhkan OAuth client milik Anda sendiri, di sebuah proyek Google Cloud. Ini gratis dan cukup dibuat sekali.

Google Drive baru diuji terhadap layanan Google tiruan, belum terhadap akun Google sungguhan.

#### Membuat OAuth client {#create-the-oauth-client}

Nama yang dicetak tebal di langkah-langkah ini adalah milik Google, bukan milik panel.

1. Buka [Google Cloud console](https://console.cloud.google.com/) dan buat proyek, atau pilih yang sudah ada.
2. Di **APIs and services** → **Library**, aktifkan **Google Drive API**.
3. Siapkan **OAuth consent screen** dengan jenis pengguna **External**, lalu ubah status publikasinya menjadi **In production**.
4. Di **Credentials**, buat **OAuth client ID** berjenis **TVs and Limited Input devices**.
5. Simpan client ID (berakhiran `.apps.googleusercontent.com`) dan client secret-nya.

::: warning
Selama status publikasi layar persetujuan masih **Testing**, Google mengakhiri akses panel setelah 7 hari, dan unggahan lalu gagal. Ubah menjadi **In production**.
:::

#### Menghubungkan {#connect}

1. Di **Penyimpanan jarak jauh**, pilih **Siapkan** atau **Tambah penyimpanan**, lalu pilih **Google Drive** di **Jenis**. **Cara membuat OAuth client** mengulang langkah-langkah di atas.
2. Masukkan **Client ID** dan **Client secret**.
3. Opsional: ubah **Folder di Google Drive**. Bawaannya `Nefo backups/<the server's host name>`. Pisahkan folder dengan `/`; folder yang belum ada akan dibuat.
4. Opsional: centang **Enkripsi cadangan sebelum diunggah** (lihat [Mengenkripsi salinan](#encrypt-the-copies)).
5. Pilih **Hubungkan ke Google Drive**. Panel menampilkan sebuah alamat, `google.com/device`, dan kode seperti `ABCD-EFGH`.
6. Di perangkat apa pun (ponsel Anda juga bisa), buka alamat itu, masuk dengan akun Google yang akan menyimpan cadangan, masukkan kodenya, dan izinkan akses.
7. Panel menunggu selama Anda melakukannya, dan menampilkan berapa lama kode itu masih berlaku. Begitu Google menjawab, panel membuat folder, menulis, menampilkan daftar, membaca, dan menghapus file uji kecil di sana, lalu menyimpan pengaturan.

Daftar **Penyimpanan jarak jauh** lalu menampilkan folder, akun Google, dan tautan **Buka di Google Drive**.

Anda boleh meninggalkan halaman selama panel menunggu: buka lagi **Pencadangan** → **Penyimpanan jarak jauh** untuk melihat kodenya.

#### Mengubah atau menghubungkan ulang {#change-or-reconnect}

Pilih **Ubah** di baris Google Drive:

- Untuk mengubah folder atau enkripsi, edit keduanya lalu pilih **Uji dan simpan**. Tidak perlu masuk lagi. Folder baru dibuat jika diperlukan. Cadangan yang diunggah sebelumnya tetap di folder lama, dan panel tidak lagi menemukannya di sana untuk pemulihan.
- Untuk memakai akun Google lain atau OAuth client lain, atau setelah akses dicabut, pilih **Hubungkan lagi** dan masukkan kode baru. Kosongkan **Client secret** untuk mempertahankan secret yang tersimpan dari client yang sama.

**Uji dan simpan** juga memperbaiki folder yang dihapus atau dipindahkan ke tempat sampah Drive: folder itu dibuat lagi.

Tetap gunakan proyek Google Cloud yang sama. Google memberi setiap proyek akses hanya ke file yang dibuat proyek itu, jadi dengan OAuth client dari proyek lain panel tidak bisa melihat cadangan yang diunggahnya sebelumnya.

#### Cara kerja unggahan {#how-uploads-work}

- Setiap cadangan diunggah dalam potongan 8 MB. Jika koneksi terputus atau Google sedang sibuk, panel menanyakan ke Google berapa banyak yang sudah sampai lalu melanjutkan dari situ. Panel mencoba lagi selama beberapa menit sebelum menyerah.
- Sebelum mengunggah, panel memeriksa ruang kosong akun Google dan langsung menolak jika cadangan tidak muat.
- Unduhan untuk pemulihan juga dilanjutkan dari titik terakhirnya.
- Folder di Drive punya subfolder yang sama dengan `/var/backups/panel`: `site/example.com/…`, `database/shop/…`.

#### Memutuskan koneksi {#disconnect}

**Hapus** di baris Google Drive melupakan pengaturan dan memberi tahu Google bahwa panel melepaskan aksesnya. Cadangan yang sudah diunggah tetap ada di Google Drive Anda.

#### Jika terjadi masalah {#when-something-goes-wrong}

| Pesan | Yang perlu dilakukan |
|---|---|
| Akses Google Drive dicabut atau sudah kedaluwarsa | Pilih **Ubah** → **Hubungkan lagi**. Jika ini terjadi sekitar 7 hari setelah terhubung, ubah dulu status publikasi layar persetujuan menjadi **In production**. |
| Google Drive penuh | Kosongkan ruang di akun Google atau beli penyimpanan tambahan, lalu unggah lagi dengan **Unggah**. |
| Folder Google Drive … telah dihapus / ada di tempat sampah Drive | Pulihkan folder dari tempat sampah Drive (cadangannya ikut kembali), atau pilih **Ubah** → **Uji dan simpan** untuk membuat folder baru. |
| Google menolak klien OAuth | Periksa client ID dan secret, dan pastikan jenis client itu **TVs and Limited Input devices**. |
| Kode kedaluwarsa sebelum dimasukkan | Pilih **Hubungkan ke Google Drive** lagi untuk mendapatkan kode baru. |

### Mengenkripsi salinan {#encrypt-the-copies}

Setiap penyimpanan (S3, SFTP, dan Google Drive) bisa mengenkripsi cadangan yang diterimanya. Salinan di penyimpanan lalu tidak bisa dibaca tanpa passphrase, baik oleh penyedia maupun oleh siapa pun yang berhasil masuk ke akun penyimpanan. Salinan di server ini tetap `.tar.gz` biasa.

1. Di formulir penyimpanan, centang **Enkripsi cadangan sebelum diunggah**.
2. Masukkan **Passphrase** minimal 12 karakter, dan yang sama di **Ulangi passphrase**. Kalimat yang terdiri dari beberapa kata cocok untuk ini.
3. Pilih **Uji dan simpan** (atau **Hubungkan ke Google Drive**). File uji juga dienkripsi.

::: danger
Tanpa passphrase, cadangan terenkripsi tidak bisa dipulihkan: tidak oleh Anda, tidak oleh panel, tidak oleh siapa pun. Simpan passphrase di luar server ini, misalnya di pengelola kata sandi. Jika server ini hilang, passphrase itulah satu-satunya yang Anda butuhkan, bersama penyimpanannya, untuk mendapatkan kembali cadangan.
:::

Yang berubah:

- Salinan terenkripsi diberi nama seperti cadangannya dengan tambahan `.enc`: `example.com_20261126-030000.tar.gz.enc`.
- Di daftar **Pencadangan**, tag penyimpanan di kolom **Disimpan** menampilkan gembok, bukan awan.
- Baris penyimpanan di **Penyimpanan jarak jauh** menampilkan **Terenkripsi**.
- Passphrase tidak pernah ditampilkan lagi. Untuk mempertahankannya saat mengubah pengaturan lain, kosongkan **Passphrase**.

Mengganti passphrase:

- Passphrase baru hanya berlaku untuk unggahan baru. Salinan yang sudah diunggah tetap memakai passphrase yang dipakai saat dienkripsi; salinan itu tidak dienkripsi ulang.
- Server ini mengingat passphrase sebelumnya (sampai 10), jadi pemulihan di sini tetap berfungsi.
- Di server lain, Anda membutuhkan passphrase yang berlaku saat salinan itu diunggah.
- Jika Anda menonaktifkan enkripsi, salinan yang sudah diunggah tetap terenkripsi, dan server ini menyimpan passphrase-nya untuk memulihkannya.

Cara kerjanya: panel mengenkripsi arsip di server ini, sambil mengunggahnya, dengan AES-256-GCM. Kuncinya diturunkan dari passphrase lewat scrypt, dengan salt acak baru untuk setiap file, jadi dua salinan dari cadangan yang sama terlihat berbeda. Arsip disegel dalam potongan 64 KB, masing-masing bernomor dan potongan terakhir ditandai, jadi salinan yang diubah, terpotong, atau disusun dari potongan salinan lain akan ditolak. Tidak ada kunci yang disimpan di server selain passphrase di pengaturan panel. Format filenya dijelaskan di [Memulihkan di server baru](./restore-new-server#format-of-encrypted-copies).

Enkripsi hanya butuh sedikit sumber daya: menurunkan kunci memakan sekitar seperempat detik dan 64 MB memori sesaat, sekali per unggahan, dan salinannya lebih besar sebanyak header 64 byte dan 16 byte per 64 KB. Untuk penyimpanan S3, panel membaca cadangan dua kali, karena S3 membutuhkan checksum salinan terenkripsi sebelum unggahan dimulai.

### Mengunggah dan menghapus penyimpanan {#uploading-and-removing-storage}

- Cadangan yang dibuat tanpa **Unggah juga ke penyimpanan jarak jauh** bisa diunggah nanti: buka menunya dan pilih **Unggah**. Panel menyalinnya ke setiap penyimpanan yang belum memilikinya, terenkripsi untuk penyimpanan yang mengenkripsi.
- Kolom **Disimpan** menampilkan **Server ini** dan tag untuk setiap salinan jarak jauh: S3, SFTP, atau Google Drive, dengan gembok untuk salinan terenkripsi.
- Menghapus pengaturan penyimpanan tidak menghapus cadangan yang sudah diunggah ke sana.
- **Cari cadangan** di baris sebuah penyimpanan menampilkan cadangan di penyimpanan itu yang belum ada di daftar, misalnya di server baru: lihat [Memulihkan di server baru](./restore-new-server).

## Memulihkan cadangan {#restore-a-backup}

1. Buka **Pencadangan** dan cari cadangan di daftar.
2. Pilih **Pulihkan** di barisnya.
3. Ketik nama yang diminta dialog (domain, nama basis data, path folder, atau nama aplikasi) dan pilih **Pulihkan**.

Panel lebih dulu memeriksa checksum SHA-256 arsip, dan menolak file yang rusak sebelum mengubah apa pun. Jika file tidak ada lagi di server ini, file itu diunduh dari penyimpanan jarak jauh.

Jika salinannya terenkripsi, panel mendekripsinya sambil mengunduhnya, dengan passphrase penyimpanan (atau passphrase sebelumnya yang masih diingat server ini). Pemeriksaan SHA-256 lalu berjalan pada arsip yang sudah didekripsi, seperti cadangan lainnya. Jika cadangan hanya ada di penyimpanan jarak jauh dan salinannya terenkripsi, dialog **Pulihkan** punya kolom **Passphrase enkripsi (opsional)**. Kosongkan untuk memakai passphrase penyimpanan. Masukkan passphrase yang berlaku saat salinan diunggah jika penyimpanan itu sudah disiapkan ulang sejak saat itu, misalnya setelah Anda menghapusnya lalu menambahkannya dengan passphrase baru.

Pemulihan berhenti sebelum ada yang berubah, dengan pesan di log, jika:

- passphrase tidak bisa membuka salinan ("this copy is encrypted and the passphrase does not unlock it");
- salinan diubah atau rusak ("the encrypted copy is damaged or was changed");
- salinan tidak lengkap ("the encrypted copy is incomplete").

Jika ada salinan di beberapa penyimpanan, panel mencoba salinan berikutnya.

::: danger
Pemulihan menimpa isi yang sekarang. Perubahan yang dibuat sejak pencadangan akan hilang.
:::

Apa yang diganti oleh pemulihan:

| Jenis | Apa yang terjadi |
|---|---|
| **Situs web** | Semua file di folder situs web dihapus, kecuali `tmp` dan `logs`, lalu file dari cadangan dikembalikan dengan pengguna situs web sebagai pemiliknya. Setiap basis data di cadangan dikosongkan dan diisi dari dump-nya. Basis data yang tidak ada lagi dibuat ulang, tertaut ke situs web, dengan kata sandi penggunanya dari cadangan (cadangan yang dibuat sebelum v0.24.0 tidak menyimpannya: basis data itu lalu mendapat kata sandi baru). Basis data yang tidak lagi tertaut ke situs web akan ditautkan ke situs web yang dipulihkan. Untuk WordPress, kata sandi basis data saat ini ditulis ke `wp-config.php` |
| **Basis data** | Basis data dihapus, dibuat ulang dalam keadaan kosong, dan diisi dari dump |
| **Folder** | Cadangan diekstrak lebih dulu di samping isi yang sekarang. Hanya jika itu berhasil, isi folder diganti, jadi cadangan yang rusak membiarkan folder seperti semula. File yang ditambahkan sejak pencadangan dihapus. Pemilik, izin, dan tautan simbolik dikembalikan. Folder yang tidak ada akan dibuat |
| **Aplikasi** | Kontainer aplikasi berhenti, folder dan datanya diganti, kontainer berjalan lagi, dan dump basis data dimuat. Kata sandi yang tersimpan di cadangan juga dikembalikan |

Dump dari cadangan situs web dan basis data dimuat seperti [impor](./databases#import) di halaman **Basis data**: sebagai pengguna MariaDB atau MySQL sementara yang hanya bisa memakai basis data tersebut, jadi dump tidak pernah menulis ke basis data lain.

### Memulihkan situs web yang sudah dihapus {#restore-a-website-that-was-deleted}

Jika situs web dari sebuah cadangan tidak ada lagi di server ini, misalnya karena Anda menghapusnya, pemulihan membuatnya lagi dari cadangan: domain dan alias, versi PHP, aturan rewrite, file, dan basis data yang sama. Dialog pemulihan memberi tahu hal ini lebih dulu. Apa yang tidak disimpan cadangan, seperti sertifikat, tugas cron, dan akses SFTP, tidak dibuat. Lihat [Memulihkan di server baru](./restore-new-server#restoring-a-website-that-doesn-t-exist-here) untuk detailnya.

Cadangan yang dibuat oleh Nefo 0.24.1 atau sesudahnya mencatat jenis setiap situs web, jadi situs web yang sudah tidak ada kembali dengan jenis yang sama: situs web aplikasi (Node.js, Python, Java) dengan runtime, perintah mulai, port, dan batas memorinya, serta situs web reverse proxy dengan alamat dan opsinya. Jika runtime aplikasinya belum terinstal, pemulihan berhenti dan meminta Anda menginstalnya dulu di halaman **Perangkat lunak**. Cadangan yang dibuat oleh versi yang lebih lama hanya mencatat versi PHP: situs web seperti itu kembali sebagai situs web statis, jadi buat dulu situs web itu lagi dengan domain dan jenis yang sama, lalu pulihkan. Untuk aplikasi Docker, instal lagi aplikasinya di domain yang sama dari [Toko aplikasi](./apps), lalu pulihkan cadangannya yang berjenis **Aplikasi**, serta cadangan situs webnya jika folder situs web menyimpan file miliknya sendiri, seperti musik Navidrome.

## Memulihkan di server lain {#restore-on-another-server}

Server baru dengan Nefo bisa menemukan cadangan di penyimpanan jarak jauh Anda lewat **Cari cadangan**, menambahkannya ke daftarnya, dan memulihkannya, termasuk situs web. Anda membutuhkan detail akses penyimpanan, dan passphrase jika salinannya terenkripsi. Lihat [Memulihkan di server baru](./restore-new-server).

## Cadangan aplikasi {#app-backups}

Aplikasi Docker dari [Toko aplikasi](./apps) menyimpan datanya di luar folder situs web, di `/var/lib/panel-apps/<name>`. Aplikasi ini mendapat cadangan berjenis **Aplikasi**:

- Mencadangkan situs web yang menjalankan aplikasi Docker juga membuat cadangan aplikasi, dengan jadwal yang sama. Retensi menghapus cadangan aplikasi lama bersama cadangan situs webnya.
- Untuk mencadangkan satu aplikasi, pilih **Aplikasi** di **Cadangkan sekarang** atau **Tambah jadwal** di halaman **Pencadangan**, atau buka **Toko aplikasi** → **Terinstal** → **Cadangkan sekarang**. Aplikasi layanan seperti PostgreSQL tidak punya situs web, jadi beri jadwal tersendiri.
- Sebelum pembaruan aplikasi, panel membuat cadangan lebih dulu. Jika pencadangan itu gagal, pembaruan tidak dimulai.

Bergantung pada aplikasinya, panel menjeda kontainer selama menyalin file-filenya, atau lebih dulu membuat dump basis datanya dengan alat milik aplikasi itu sendiri. Pulihkan cadangan aplikasi di halaman **Pencadangan**, seperti cadangan lain. Pemulihan membutuhkan aplikasi yang terinstal. Setelah Anda mencopot aplikasi, cadangannya tetap ada di daftar dengan nama dan logonya: instal lagi aplikasinya (aplikasi dengan situs web di domain yang sama), lalu pulihkan.

## Menghapus cadangan {#delete-a-backup}

Buka menu sebuah cadangan dan pilih **Hapus**. Cadangan dihapus dari server ini dan dari penyimpanan jarak jauh.

## Peringatan saat pencadangan gagal {#alerts-when-a-backup-fails}

Panel bisa mengirim peringatan lewat Telegram, email, Discord, Slack, atau webhook saat pencadangan atau pemulihan gagal. Buka **Pengaturan** → **Peringatan** dan aktifkan **Pencadangan, pemulihan, atau perpanjangan sertifikat gagal**. Lihat [Email dan peringatan](./mail-alerts).

Tab **Cadangan otomatis** juga menampilkan **Gagal** untuk jadwal yang eksekusi terakhirnya gagal.
