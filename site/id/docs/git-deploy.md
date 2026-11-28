---
title: Deploy dari Git
description: Deploy situs web dari GitHub, GitLab, Gitea, atau server Git apa pun, secara manual atau pada setiap push, dengan kunci deploy, webhook bertanda tangan, dan skrip deploy.
---

# Deploy dari Git

Nefo men-deploy situs web dari repositori Git di GitHub, GitLab, Gitea, Forgejo, atau server Git apa pun, lewat HTTPS atau SSH dengan kunci deploy per situs web. Nefo men-deploy saat Anda mengklik tombol, atau pada setiap push lewat webhook bertanda tangan, lalu menjalankan skrip deploy Anda. Semuanya berjalan sebagai pengguna Linux milik situs web.

Deploy Git ada di tab **Deploy Git** setiap situs web, kecuali situs web aplikasi Docker. Pada situs web yang [file-nya dikunci](./website-protection#lock-a-website-s-files), panel membuka kunci file untuk deploy lalu menguncinya lagi sesudahnya.

## Sebelum mulai {#before-you-start}

- **Alamat repositori.** Nefo menerima tiga bentuk:
  - `https://github.com/owner/repo.git` untuk repositori publik;
  - `git@github.com:owner/repo.git` (bentuk yang ditampilkan GitHub) untuk repositori privat;
  - `ssh://git@git.example.com:2222/owner/repo.git` untuk server SSH di port lain.

  `http://` biasa ditolak, begitu pula kata sandi atau token di dalam alamat, karena akan terlihat di log. Untuk repositori privat, gunakan alamat SSH dan kunci deploy situs web.
- **Foldernya.** Pilih tempat file diletakkan:
  - **Folder web** (`/var/www/example.com/public`): untuk situs web yang merupakan repositori itu sendiri, dengan `index.html` atau `index.php` di tingkat teratas.
  - **Folder situs** (`/var/www/example.com`): untuk proyek yang punya folder `public` sendiri, seperti Laravel atau Symfony. Situs web lalu menampilkan folder `public` dari repositori.

  Situs web aplikasi Node.js, Python, atau Java tidak punya pilihan: file masuk ke **Folder aplikasi** (`/var/www/example.com/app`), tempat programnya berjalan. Letakkan `package.json` atau `requirements.txt` di tingkat teratas repositori.
- **Folder kosong untuk deploy pertama.** Halaman sambutan dari panel tidak dihitung, begitu pula folder `logs`, `tmp`, dan `.ssh` di folder situs, maupun folder `.venv` aplikasi Python di folder aplikasi. Pindahkan file lain terlebih dahulu, atau deploy ke folder yang satunya.
- **Git** tidak perlu diinstal: Nefo menginstalnya pada deploy pertama jika belum ada.

## Menyiapkan repositori {#set-up-the-repository}

1. Buka **Situs web**, lalu tab **Deploy Git** situs web.
2. Masukkan **Alamat repositori**.
3. Masukkan **Branch**, misalnya `main`.
4. Di bawah **Letakkan file di**, pilih **Folder web** atau **Folder situs**. Situs web aplikasi menampilkan **Folder aplikasi** sebagai gantinya, tanpa pilihan.
5. Opsional: tulis **Skrip deploy (opsional)** (lihat [Skrip deploy](#the-deploy-script)).
6. Klik **Simpan**.

Pertama kali, Nefo membuat kunci deploy dan rahasia webhook untuk situs web, lalu menampilkan "Pengaturan disimpan. Tambahkan kunci deploy dan webhook di bawah ke repositori, lalu deploy."

## Menambahkan kunci deploy (repositori privat) {#add-the-deploy-key-private-repositories}

Repositori HTTPS publik tidak membutuhkan kunci. Untuk repositori privat, tambahkan **Kunci publik** situs web, yang ditampilkan di bawah **Kunci deploy**, ke repositori sebagai kunci deploy hanya-baca:

- **GitHub:** repositori → Settings → Deploy keys → Add deploy key. Biarkan "Allow write access" tidak aktif.
- **GitLab:** proyek → Settings → Repository → Deploy keys, tanpa akses tulis.
- **Gitea dan Forgejo:** repositori → Settings → Deploy keys.

Setiap situs web punya kunci ed25519 sendiri, yang dibuat sebagai pengguna situs web di `/var/www/example.com/.ssh/panel_deploy`. **Kunci baru** menggantinya: kunci lama langsung berhenti berfungsi, jadi tambahkan kunci baru ke repositori dan hapus kunci lama di sana.

Koneksi SSH pertama menerima host key server Git dan mengingatnya di `/var/www/example.com/.ssh/known_hosts`. Jika kunci server berubah nanti, deploy gagal dengan pesan "The Git server's SSH host key is not the one seen before." Jika server memang mengganti kuncinya, hapus file itu dan deploy lagi.

## Deploy {#deploy}

Di bawah **Deploy**, klik **Deploy sekarang**. Jika Anda mengubah pengaturan, simpan dulu. Sebuah log menampilkan setiap langkah:

1. Nefo mengambil branch dari repositori.
2. File yang dilacak mengikuti versi branch di repositori.
3. Skrip deploy berjalan, jika Anda menulisnya.

File yang tidak dilacak tetap ada, jadi unggahan dan file yang diabaikan repositori (seperti `.env`) dipertahankan pada setiap deploy. Setelah deploy pertama, Nefo menghapus halaman sambutannya jika repositori sudah meletakkan situs web di `public`.

::: warning
Perubahan yang Anda buat di server pada file yang dilacak repositori akan ditimpa pada deploy berikutnya. Commit perubahan itu ke repositori sebagai gantinya.
:::

Setelah deploy, tab ini menampilkan statusnya (**Berhasil di-deploy** atau **Gagal**), waktunya, 12 karakter pertama commit, dan subjeknya. Pengambilan bisa memakan waktu hingga 10 menit, skrip hingga 15 menit, dan seluruh deploy hingga 30 menit.

Jika pengambilan gagal, log menjelaskan alasan yang umum:

| Pesan | Yang harus dilakukan |
|---|---|
| `The repository refused the deploy key` | Tambahkan kunci yang ditampilkan di panel ke repositori. |
| `The repository is private or does not exist` | Periksa alamatnya. Untuk repositori privat, gunakan alamat SSH-nya dan tambahkan kunci deploy. |
| `The repository has no branch …` | Periksa **Branch**. |
| `The Git server's SSH host key is not the one seen before` | Lihat bagian kunci deploy di atas. |
| `The repository's host name was not found` | Periksa alamatnya. |
| `It took longer than 10 minutes` | Coba lagi; periksa ukuran repositori dan jaringan. |

## Skrip deploy {#the-deploy-script}

Skrip berjalan setelah setiap deploy, di folder deploy, sebagai pengguna situs web, paling lama 15 menit. Skrip boleh berisi hingga 2000 karakter. Misalnya, untuk situs Laravel:

```sh
composer install --no-dev --optimize-autoloader
php artisan migrate --force
```

Di situs web aplikasi, skrip berjalan di folder `app`, misalnya `npm ci` diikuti `npm run build`. `node` dan `npm`-nya adalah versi Node.js yang dipilih untuk situs web, dan di situs web Python, `python` dan `pip` adalah milik lingkungan virtualnya (`app/.venv`). Di situs web Java, `java`, `javac`, dan `JAVA_HOME` adalah milik versi Java yang dipilih untuk situs itu, jadi `./mvnw package` atau `./gradlew build` melakukan build dengan versi tersebut.

- Skrip berjalan dengan `sh -e`: skrip berhenti pada perintah pertama yang gagal, dan deploy ditandai **Gagal**. Saat itu file baru sudah terpasang.
- `php` adalah versi PHP situs web, juga untuk Composer.
- `DEPLOY_COMMIT` berisi commit yang di-deploy dan `DEPLOY_BRANCH` berisi branch-nya.
- `HOME` adalah folder `tmp` situs web.
- Skrip tidak bisa memakai `sudo` atau memulai ulang layanan sistem. Untuk memulai ulang program situs web aplikasi setelah deploy, klik **Mulai ulang** di tab **Ringkasan** situs web.

## Deploy pada setiap push {#deploy-on-every-push}

Di bawah **Deploy saat push**, Nefo menampilkan **URL webhook**, seperti `https://example.com/.panel/deploy/12`, dan **Rahasia**-nya. Webhook berada di domain milik situs web, jadi host repositori tidak pernah perlu menjangkau port panel. Server web meneruskannya ke pendengar kecil milik panel di `127.0.0.1:7809`.

::: warning
Tanpa HTTPS, URL webhook memakai `http://`, dan GitLab akan mengirim rahasia tanpa enkripsi. Aktifkan HTTPS terlebih dahulu: lihat [Sertifikat SSL](./ssl).
:::

Setiap push ke branch yang dipilih men-deploy situs web. Push ke branch lain, push tag, event lain, dan penghapusan branch diabaikan.

### GitHub {#github}

1. Buka repositori → Settings → Webhooks → Add webhook.
2. Tempelkan **URL webhook** sebagai payload URL dan **Rahasia** sebagai secret.
3. Atur content type ke `application/json`.
4. Pilih "Just the push event" dan tambahkan webhook.

GitHub langsung mengirim uji coba (ping), dan panel menjawabnya, jadi pengiriman pertama tampil sebagai berhasil.

### GitLab {#gitlab}

1. Buka proyek → Settings → Webhooks.
2. Tempelkan **URL webhook** sebagai URL dan **Rahasia** sebagai secret token.
3. Centang Push events dan tambahkan webhook.

### Gitea dan Forgejo {#gitea-and-forgejo}

1. Buka repositori → Settings → Webhooks → Add webhook, lalu pilih Gitea (atau Forgejo).
2. Tempelkan **URL webhook** sebagai target URL dan **Rahasia** sebagai secret.
3. Atur content type ke `application/json`, picu pada push events, dan tambahkan webhook.

Gogs bekerja dengan cara yang sama: pilih Gogs di langkah 1.

### Bagaimana panel memeriksa webhook {#how-the-panel-checks-a-webhook}

- GitHub, Gitea, Forgejo, dan Gogs menandatangani setiap webhook dengan rahasia (HMAC-SHA256). GitLab mengirim rahasia itu sendiri. Webhook dengan tanda tangan yang salah atau tidak ada mendapat HTTP 401 dan dicatat beserta alamat pengirimnya.
- Perlindungan kata sandi dan firewall web situs tidak berlaku untuk path webhook, jadi Anda tidak perlu membuat pengecualian untuknya.
- Nefo menjalankan satu tugas dalam satu waktu. Jika ada tugas lain yang sedang berjalan saat push tiba, deploy menunggu dan dicoba lagi setiap 15 detik hingga satu jam. Beberapa push dalam waktu tersebut cukup dengan satu deploy: deploy itu mengambil commit terbaru.
- **Rahasia baru** mengganti rahasia. Push ditolak sampai Anda menempelkan rahasia baru ke webhook repositori.

Host repositori menampilkan jawaban panel di daftar pengiriman terbarunya: `deploying`, `queued: another task is running`, `ignored: not branch main`, atau kesalahan seperti `bad_signature`.

## Log deploy {#the-deploy-log}

- Tab **Deploy Git** menampilkan log deploy yang sedang berjalan atau yang terakhir, termasuk deploy yang dimulai oleh push.
- **Akun** → **Aktivitas terbaru** mencantumkan setiap deploy sebagai "Men-deploy dari Git" atau "Deploy Git gagal", beserta situs web, commit, dan pemicunya (`manual` atau push dari host repositori).

## Berhenti men-deploy dari Git {#stop-deploying-from-git}

Di bawah **Hentikan deploy dari Git**, klik **Hentikan deploy dari Git** dan konfirmasi. Nefo melupakan pengaturannya dan menghapus kunci deploy. File yang sudah di-deploy tetap ada di folder, bersama folder `.git` repositori.
