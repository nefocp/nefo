---
title: File
description: Jelajahi, edit, unggah, unduh, cari, kompres, dan ekstrak file di browser, serta pulihkan item yang terhapus dari tempat sampah.
---

# File

Halaman **File** di Nefo adalah pengelola file di browser: menjelajah, mengedit file teks hingga 2 MB, mengunggah hingga 4 GB sekaligus, mengunduh, mengambil file dari URL, mencari berdasarkan nama atau isi, mengompres dan mengekstrak, serta memulihkan item yang terhapus dari tempat sampah 30 hari. Di dalam folder situs web, pengelola file bekerja sebagai pengguna situs web itu dan tidak bisa keluar dari foldernya.

Halaman ini membahas semua hal tersebut. Panel berjalan sebagai root, jadi halaman ini juga menjelaskan ke mana saja Anda bisa pergi dan apa yang ditolak panel untuk diubah.

## Menemukan jalan {#find-your-way}

Pengelola file terbuka di `/var/www`, tempat setiap situs web punya foldernya, `/var/www/example.com`.

- Pilih nama folder untuk membukanya. **Naik** menuju folder induk dan **Muat ulang** membaca folder lagi.
- Setiap bagian path di bagian atas bisa diklik. Tombol rumah (**Situs web**) membawa Anda kembali ke `/var/www`.
- Untuk melompat ke folder mana pun, pilih tombol pensil (**Ketik path**), ketik path absolut seperti `/etc/nginx`, lalu pilih **Buka**.
- Tabel menampilkan **Nama**, **Ukuran**, **Izin**, **Pemilik**, dan **Diubah**. Folder dengan lebih dari 5.000 item hanya menampilkan 5.000 item pertama.

## Apa yang bisa Anda jangkau {#what-you-can-reach}

Di luar folder situs web, Anda bekerja sebagai root di seluruh sistem file.

Di dalam folder situs web, `/var/www/example.com`, setiap operasi dibatasi pada folder itu, dan lencana berisi domain muncul di sebelah path:

- Path tidak bisa naik keluar dengan `..`, dan tautan simbolik tidak bisa mengarah ke luar folder. Situs yang diretas dan menanam tautan ke `/etc` tidak bisa menipu panel agar menyentuh file sistem.
- File dan folder baru dimiliki pengguna situs web, jadi PHP situs itu bisa memakainya.
- Memindahkan, mengompres, atau mengekstrak antara folder situs web dan folder lain mana pun ditolak. Sebagai gantinya, unduh file tersebut dan unggah ke tempat yang Anda butuhkan.
- Selama [file situs web dikunci](./website-protection#lock-a-website-s-files), Anda hanya bisa mengubah folder yang bisa ditulis serta folder `tmp` dan `logs`-nya. Di bagian lain folder situs web, pengelola file menolak dengan pesan bahwa file sedang dikunci.

Folder-folder berikut tidak bisa dihapus, diganti namanya, atau diubah izinnya:

- `/`, `/bin`, `/boot`, `/dev`, `/etc`, `/home`, `/lib`, `/lib64`, `/opt`, `/proc`, `/root`, `/run`, `/sbin`, `/srv`, `/sys`, `/tmp`, `/usr`, `/var`;
- `/var/www`, `/var/lib`, `/var/log`, `/var/lib/nefo`, dan `/etc/nefo`;
- folder milik setiap situs web, seperti `/var/www/example.com` (file di dalamnya bisa diubah);
- folder tempat sampah, `/var/lib/panel-trash`, dan folder-folder yang memuatnya.

## Mengedit file {#edit-a-file}

1. Pilih nama file. File terbuka di editor.
2. Lakukan perubahan. Ctrl+S menyimpan, dan tombol Tab menyisipkan tab.
3. Pilih **Simpan**, lalu **Tutup**.

Editor membuka file teks hingga 2 MB. File biner dan file yang lebih besar ditolak: unduh saja file-file itu. File yang disimpan mempertahankan izin dan pemiliknya. Jika Anda menutup editor dengan perubahan yang belum disimpan, panel bertanya terlebih dahulu.

Untuk membuat file, pilih **File baru**, masukkan nama di **Nama file baru**, lalu pilih **OK**. Untuk membuat folder, pilih **Folder baru**. File baru mendapat izin `0644` dan folder baru `0755`.

## Mengunggah file {#upload-files}

1. Buka folder tujuan unggahan.
2. Pilih **Unggah** dan pilih satu atau beberapa file.
3. Jika sudah ada file dengan nama yang sama, panel bertanya sebelum menggantinya.

Bilah progres menampilkan unggahan. Satu unggahan bisa berukuran total hingga 4 GB. File yang diunggah mendapat izin `0644`. Di dalam folder situs web, file dimiliki pengguna situs web; di tempat lain, dimiliki root.

## Mengunduh file {#download-a-file}

Buka menu sebuah file dan pilih **Unduh**. Hanya file yang bisa diunduh. Untuk mengunduh folder, kompres dulu.

## Mengunduh dari URL {#download-from-a-url}

Server bisa mengambil file dari internet langsung ke sebuah folder, tanpa melewati komputer Anda.

1. Buka folder tujuan dan pilih **Unduh dari URL**.
2. Masukkan **Alamat file (URL)** dan pilih **Unduh**.
3. Sebuah log menampilkan progresnya. Anda bisa memilih **Sembunyikan (unduhan tetap berjalan)**.

Aturannya:

- Hanya alamat `http` dan `https`, dengan paling banyak 5 pengalihan.
- File hingga 2 GB.
- Panel tidak pernah mengunduh dari jaringan milik server sendiri: alamat loopback, privat, dan link-local (seperti `127.0.0.1`, `10.x.x.x`, dan `169.254.169.254`), serta alamat milik server sendiri, ditolak. Hal ini diperiksa lagi untuk setiap pengalihan.
- Nama file berasal dari server atau dari URL. File yang sudah ada tidak pernah diganti: panel menambahkan ` (1)`, ` (2)`, dan seterusnya.
- File dimiliki pemilik folder, dengan izin `0640`.

## Pencarian {#search}

1. Buka folder tempat pencarian.
2. Ketik di kotak pencarian (**Cari di folder ini**) dan tekan Enter.
3. Di dialog, sesuaikan **Nama mengandung**. Untuk mencari teks di dalam file, centang **Cari juga di dalam file** dan isi **Teks di dalam file**.
4. Pilih **Cari**.

Huruf besar dan kecil diperlakukan sama. Pilih sebuah hasil untuk membuka foldernya. File teks juga terbuka di editor.

Batasan mencegah satu pencarian membebani server:

| Batas | Nilai |
|---|---|
| Item yang diperiksa | 20.000 |
| Hasil | 200 |
| Waktu | 20 detik |
| Pencarian teks | Melewati file biner dan file di atas 2 MB |
| Panjang kueri | 200 karakter |

Tautan simbolik dicantumkan tetapi tidak pernah diikuti. Pencarian dari `/` melewati `/proc`, `/sys`, dan `/dev`. Jika batas tercapai, panel memberi tahu Anda: cari di folder yang lebih kecil atau gunakan kata yang lebih spesifik.

## Mengompres dan mengekstrak {#compress-and-extract}

### Mengompres {#compress}

1. Centang file dan folder yang akan dikemas.
2. Pilih **Kompres** di bilah pilihan.
3. Masukkan **Nama arsip (.zip atau .tar.gz)** dan pilih **OK**.

Arsip dibuat di folder saat ini. Tautan simbolik dan file khusus tidak disertakan.

### Mengekstrak {#extract}

1. Buka menu sebuah arsip dan pilih **Ekstrak**.
2. Kolom berisi folder saat ini. Ubah untuk mengekstrak ke tempat lain, lalu pilih **OK**.

Panel mengekstrak arsip `.zip`, `.tar.gz`, `.tgz`, dan `.tar`:

- File yang sudah ada dengan nama yang sama akan ditimpa.
- Tautan simbolik dan file khusus di arsip dilewati.
- File mendapat izin `0644`, atau `0755` jika sebelumnya bisa dieksekusi. File dimiliki pengguna situs web di dalam folder situs web, dan dimiliki root di tempat lain.
- Arsip dengan isi lebih dari 20 GB atau lebih dari 500.000 file ditolak.

Kompresi dan ekstraksi berjalan sebagai tugas dengan log langsung, jadi tetap berlanjut jika Anda meninggalkan halaman.

## Mengganti nama atau memindahkan {#rename-or-move}

Buka menu sebuah item dan pilih **Ganti nama**, lalu masukkan nama baru dan pilih **OK**. Nama baru dibaca relatif terhadap folder saat ini, jadi path seperti `old/index.html` atau `../index.html` memindahkan item di dalam situs web yang sama atau bagian server yang sama.

## Mengubah izin {#change-permissions}

Pilih izin sebuah item di kolom **Izin**, atau buka menunya dan pilih **Izin**. Masukkan nilai baru dalam oktal, misalnya `0644` untuk file atau `0755` untuk folder, lalu pilih **OK**.

Nilai bisa sampai `0777`. Bit setuid, setgid, dan sticky tidak bisa diatur di sini.

Kolom **Pemilik** menampilkan pemilik dan grup. Pengelola file tidak mengubah pemilik. Gunakan [terminal](./system#terminal) untuk itu, misalnya `chown -R example_com:example_com /var/www/example.com/public` dengan pengguna milik situs web.

## Menghapus file dan tempat sampah {#delete-files-and-the-trash}

1. Centang item-itemnya, atau buka menu salah satunya, lalu pilih **Hapus**.
2. Secara bawaan, item masuk ke tempat sampah: pilih **Pindahkan ke tempat sampah**.
3. Untuk melewati tempat sampah, centang **Hapus permanen (lewati tempat sampah)** dan pilih **Hapus permanen**.

::: danger
Item yang dihapus permanen, termasuk folder beserta seluruh isinya, tidak bisa dipulihkan.
:::

### Memulihkan dari tempat sampah {#restore-from-the-trash}

1. Buka folder situs web yang file-filenya ingin Anda kembalikan, atau folder mana pun di luar situs web untuk bagian server lainnya.
2. Pilih **Tempat sampah**.
3. Pilih **Pulihkan** di sebelah item.

Setiap situs web punya tempat sampah sendiri, dan bagian server lainnya berbagi satu tempat sampah. Item kembali ke path asalnya, dengan pemilik dan izinnya. Folder induk yang hilang akan dibuat. Jika sekarang ada hal lain di path itu, pemulihan ditolak: pindahkan dulu hal tersebut.

Di tempat sampah, Anda juga bisa memakai **Hapus** untuk menghapus satu item selamanya, atau memilih **Kosongkan tempat sampah**. Daftar ini menampilkan 1.000 item yang paling baru dihapus.

### Berapa lama item disimpan {#how-long-items-stay}

Item disimpan di tempat sampah selama **30 hari**. Panel memeriksa setiap jam dan menghapus item yang lebih lama secara permanen.

- Tempat sampah disimpan di `/var/lib/panel-trash/<domain>`, atau `/var/lib/panel-trash/_server` untuk bagian server lainnya. Hanya root yang bisa membacanya, jadi pengguna situs web tidak bisa menjangkau file-filenya yang sudah dihapus.
- Item di tempat sampah tetap memakan ruang disk. Hapus file yang sangat besar secara permanen.
- Item yang lebih besar dari 1 GB yang berada di disk berbeda dari tempat sampah tidak bisa dipindahkan ke sana. Hapus permanen saja.
- Menghapus situs web juga menghapus tempat sampahnya.
