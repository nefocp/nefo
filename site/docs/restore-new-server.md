---
title: Restore on a new server
description: Find the backups on your S3, SFTP or Google Drive storage from a new Nefo server, restore websites that aren't there yet with their databases, and decrypt a backup copy by hand.
---

# Restore on a new server

If a server is lost, a new server with Nefo can find the backups on your remote storage (S3-compatible, SFTP or Google Drive) with **Find backups**, and restore them. A website that doesn't exist on the new server is created again from its backup, with its domain, aliases, PHP version, files and databases. This works from v0.24.0.

This page covers finding the backups, what a restore creates and what it doesn't, other kinds of backups, and decrypting a copy by hand. For backups in general, see [Backups](./backups).

## What you need

- the access details of the storage: the bucket and its keys, the SFTP account, or a Google OAuth client of the same Google Cloud project;
- the passphrase, if the copies are encrypted;
- a server with Nefo, a web server, the PHP versions your websites use, and MariaDB or MySQL (see [First steps](./first-steps#set-up-the-server)).

## How do I restore a website on a new server? {#restore-on-another-server}

1. Install Nefo on the new server, then the web server, PHP and MariaDB (see [First steps](./first-steps)).
2. Go to **Backups** → **Remote storage** and set up the same storage: the same bucket and folder, SFTP server and folder, or Google Drive folder. For Google Drive, use the same Google Cloud project (see [Google Drive](./backups#google-drive)). Ticking **Encrypt backups before upload** with the old passphrase is optional: you can also enter the passphrase in step 4.
3. Select **Find backups** on the storage's row. The panel lists the backup files on the storage that are not in its backup list, and reads the start of each one (its `manifest.json`): what it backs up, when it was made, its size, the PHP version and the databases. It doesn't download the rest.
4. If some encrypted backups don't open with the storage's passphrase, enter the passphrase they were uploaded with and select **Read them**.
5. Select **Add to the list** for a backup, or **Add all to the list (…)**, with the number of backups the panel could read. Nothing changes on the server: the backups are now on the **Backups** list, kept only on the storage.
6. Restore each one with **Restore**, as in [Restore a backup](./backups#restore-a-backup). **Restore** in the found list does steps 5 and 6 in one go.

A backup that the panel can't read stays in the list with the reason, such as a wrong passphrase, a damaged file, or a manifest that names another website than the folder it's in. **Look again** reads the storage again.

Backups found on a storage have no checksum recorded in the list. Their first restore reads the whole archive once before it changes anything, so a damaged file is refused, and records the checksum then.

## Restoring a website that doesn't exist here

When the website of a backup doesn't exist on this server, the restore dialog says so, and the restore creates it:

- with the same domain and aliases. If an alias is taken by another website, the website is created without its aliases;
- with the same PHP version, or the newest one installed here, with a warning in the log;
- marked as WordPress (or the other application) with its rewrite rules and its own web server directives, when the backup has them;
- with its files, owned by the new website's user;
- with its databases and their users. Backups made by v0.24.0 and later keep each database user's password, so the website's own configuration (`wp-config.php` and the like) keeps working. With an older backup, the user gets a new password: WordPress gets it in `wp-config.php` automatically; for other software, copy it from **Databases**.

Not in a backup, so not created again: the SSL certificate (get a new one on the website's **SSL / HTTPS** tab once its DNS points to the new server), cron jobs, SFTP accounts, background programs, redirects, password protection and the website's other settings.

Backups made by Nefo 0.24.1 or later record each website's type: an app website (Node.js, Python, Java) is created again with its runtime, start command, port and memory limit, and a reverse proxy website with its address and options. Install the app's runtime on the **Software** page first; the restore stops and says so if it's missing. Backups made by older versions record only the PHP version, and such a website is created again as a static website: create it first with the same domain and type (see [Websites](./websites#create-a-website)), then restore, and the restore only replaces its files and databases.

A restore never replaces a website without the confirmation: you type its domain in the dialog, whether it exists or not. Restoring over a website that exists replaces its files and databases, as always.

::: tip
A website backup holds the passwords of its databases' users, next to the files and the dumps. Treat backups as sensitive as the website itself. Encrypting the remote copies keeps them unreadable on the storage.
:::

## Other kinds of backups

| Kind | On a new server |
|---|---|
| **Database** | Created again if missing, with its user's password from the backup when the backup has it |
| **Folder** | Created again if missing |
| **App** | Install the app from the App store first (its images are downloaded again), then restore |

## Decrypt a copy by hand

You can also download a backup file from the storage yourself: from the S3 console, with an SFTP client, or from Google Drive in a browser. An encrypted copy (`.tar.gz.enc`) needs only its passphrase. Decrypt it with the `nefo` program, on any Linux server where it is installed:

```sh
nefo decrypt-backup example.com_20261126-030000.tar.gz.enc
```

It asks for the passphrase and writes `example.com_20261126-030000.tar.gz` next to it. See [Command line](./cli#nefo-decrypt-backup).

The archive has the website's files under `files/` and the database dumps under `databases/`. Unpack it with `tar xzf`.

## Format of encrypted copies

For reference, and to decrypt a copy without Nefo. All numbers are big-endian.

| Offset | Size | Content |
|---|---|---|
| 0 | 7 | `NEFOENC` (ASCII) |
| 7 | 1 | Format version: 1 |
| 8 | 1 | Key derivation: 1 = scrypt |
| 9 | 1 | scrypt cost, log2(N): 16 |
| 10 | 1 | scrypt r: 8 |
| 11 | 1 | scrypt p: 1 |
| 12 | 1 | Cipher: 1 = AES-256-GCM |
| 13 | 1 | log2(chunk size): 16, for 64 KiB |
| 14 | 2 | Zero |
| 16 | 32 | Salt, random for each file |
| 48 | 12 | Key check |
| 60 | 4 | CRC-32 (IEEE) of bytes 0 to 59 |

- scrypt(passphrase, salt, N, r, p) gives 44 bytes. Bytes 0–31 are the AES-256 key; bytes 32–43 must equal the key check, or the passphrase is wrong.
- After the 64-byte header come the chunks. The archive is cut into pieces of the chunk size. The last piece is shorter, possibly empty, so there are always size ÷ chunk + 1 pieces (rounded down, plus one).
- Each piece is sealed with AES-256-GCM, with the 64-byte header as additional data, and the nonce: the piece's number from 0 (8 bytes), 3 zero bytes, then `01` for the last piece and `00` for the others. It is stored as the ciphertext followed by the 16-byte tag.
- The file's size is 64 + size + 16 × (size ÷ 65536 + 1).
