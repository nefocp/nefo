---
title: Backups
description: Back up websites, databases, folders and apps, on a schedule with retention, to this server, S3-compatible storage or an SFTP server, and restore them in one click.
---

# Backups

The **Backups** page makes and restores backups of websites, databases, folders and App store apps. This page covers what a backup contains, backing up by hand and on a schedule, remote storage, restoring, and alerts when a backup fails.

## What a backup contains

| Kind | Contains |
|---|---|
| **Website** | The files of the website's folder, `/var/www/example.com`, except its top-level `tmp` and `logs` folders, and an SQL dump of every database linked to the website |
| **Database** | An SQL dump of one database |
| **Folder** | Any folder of the server, such as `/etc/nginx` or `/opt/app`, with owners, permissions and symbolic links |
| **App** | A Docker app from the App store: its folder in `/var/lib/panel-apps`, its saved passwords, its containers' data and dumps of its databases (see [App backups](#app-backups)) |

Link a database to its website when you create it (see [Databases](./databases#create-a-database)), so the website's backup includes it. Website backups leave out symbolic links and special files.

Every backup is a `.tar.gz` archive with a `manifest.json`, the files under `files/` and the database dumps under `databases/`. Database dumps are made with `mariadb-dump --single-transaction`, so websites keep running.

Nefo doesn't encrypt backups. On this server they are readable by root only. Choose remote storage you trust, or encrypt the bucket or disk on the provider's side.

## Where backups are kept

Every backup is first saved on this server, in `/var/backups/panel`:

```
/var/backups/panel/site/example.com/example.com_20261126-030000.tar.gz
/var/backups/panel/database/shop/shop_20261126-030000.tar.gz
/var/backups/panel/folder/etc_nginx-1a2b3c4d/etc_nginx_20261126-030000.tar.gz
```

The folders have mode `0700` and the files `0600`. The panel also records a SHA-256 checksum of each archive. Local backups take disk space, so keep an eye on the disk and on how many backups your schedules keep.

To copy a backup to your computer, open its menu and select **Download**.

## Back up now

1. Go to **Backups** and select **Back up now**.
2. Choose **Website**, **Database**, **Folder** or **App**. **App** is there once an app from the App store is installed.
3. Pick the website, database or app in **Backup of**, or type a **Folder path** such as `/etc/nginx`. Folders backed up before are suggested.
4. If remote storage is set up, you can tick **Also upload to remote storage**.
5. Select **Back up now**.

The progress log shows each step. The panel runs one long task at a time, so wait for a running task to finish first.

For a website, you can also go to **Websites** → (site) → **Backups** and select **Back up now**.

### Folders that can't be backed up

The panel refuses these folders, any folder inside them, and any folder that contains one of them. Symbolic links are followed before the check.

- `/`, the whole system;
- `/proc`, `/sys`, `/dev` and `/run`;
- the panel's data, `/var/lib/nefo`, and its backups, `/var/backups/panel`;
- `/var/lib/panel-sftp`, `/var/lib/panel-acme` and `/var/lib/panel-quarantine`.

So `/var` and `/var/lib` can't be backed up as a whole, but `/var/www` or `/etc` can.

## Automatic backups

1. Go to **Backups** and select **Add schedule**.
2. Choose **Website**, **Database**, **Folder** or **App**, and the target.
3. In **When**, choose **Every day** or **Every week**. For a weekly backup, choose the **Day**.
4. Choose the **Hour**. It is the server's time, and the form shows its UTC offset.
5. Enter the **Number of backups to keep**, from 1 to 365.
6. Optional: tick **Also upload to remote storage**.
7. Select **Save schedule**.

For a website there is a shortcut: **Websites** → (site) → **Backups** → **Every day at 03:00**. It keeps 7 backups.

The **Automatic backups** tab lists the schedules with their last run, **Done** or **Failed**. Open **The last run failed** to read the error.

How schedules run:

- The panel checks every minute. If another task is running, the backup waits for the next minute.
- If the server was off at the planned hour, the backup runs once the panel is running again.
- A new schedule waits for its first planned hour. To have a backup right away, use **Back up now** too.

### Retention

After each scheduled backup, the panel deletes the oldest backups made by that schedule beyond the number to keep, on this server and in remote storage. Backups you made by hand are never deleted automatically.

Deleting a schedule stops it. Its existing backups stay. Deleting a website, a database or an app also deletes its schedules, and its backups stay on the **Backups** page.

## Remote storage

A copy on another provider protects you if this server is lost. Go to **Backups** → **Remote storage** and select **Set up**. You can set up S3-compatible storage and an SFTP server at the same time: backups are then uploaded to both.

Google Drive isn't available yet.

### S3-compatible storage

Choose **S3-compatible storage** and fill in:

| Field | What to enter |
|---|---|
| **Endpoint (https://…)** | The provider's S3 address, `https://` and a host only, no path. For example `https://s3.eu-central-1.amazonaws.com` |
| **Bucket** | The bucket's name |
| **Region** | The bucket's region. Empty means `us-east-1`. Cloudflare R2 uses `auto` |
| **Folder in the bucket (optional)** | A prefix such as `panel/` |
| **Access key ID** | |
| **Secret access key** | When you change the settings later, leave it empty to keep the current one |

This works with Amazon S3, Cloudflare R2, Backblaze B2, Wasabi and MinIO. Use an access key that may only use this bucket.

Select **Test and save**. The panel writes a small test file, reads it back and deletes it before it saves the settings.

One backup can be at most 5 GB in S3 storage, the limit of a single upload.

### SFTP server

Use another server you can sign in to over SFTP (SSH), such as a storage box or a second VPS, with an account that is only for backups.

Choose **SFTP server** and fill in:

| Field | What to enter |
|---|---|
| **Server** | A host name or IP address, such as `backup.example.com` |
| **Port** | Usually `22` |
| **Username** | The account on the backup server |
| **Sign in with** | **Password**, or **Private key** |
| **Private key (OpenSSH or PEM)** | The whole key, from `-----BEGIN` to `END-----`. Its public key must be in `~/.ssh/authorized_keys` on the backup server |
| **Passphrase (only for an encrypted key)** | |
| **Folder on the server (optional)** | Relative to the account's home folder, unless it starts with `/`. It is created if needed |

Select **Test and save**. The panel connects, writes, lists, reads and deletes a test file, then saves the settings. Once a key is saved, **Change** shows its public half under **Public key, for ~/.ssh/authorized_keys on the backup server**, so you can copy it again.

### Host key pinning

**Test and save** trusts the host key the SFTP server presents at that moment, and shows it under **Trusted host key**. Every later connection must present exactly that key, or the backup upload fails.

Before you rely on the storage, compare the fingerprint with the backup server's own key. On the backup server, run:

```sh
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
```

If the backup server was reinstalled or its keys changed on purpose, select **Change** and **Test and save** again to trust the new key. The panel then warns that the key was replaced.

::: warning
A changed host key can also mean that someone intercepts the connection. If you didn't expect the change, check the backup server before you trust the new key.
:::

### Uploading and removing storage

- A backup made without **Also upload to remote storage** can be uploaded later: open its menu and select **Upload**. The panel copies it to each storage that doesn't have it yet.
- The **Stored** column shows **This server** and a tag for each remote copy, S3 or SFTP.
- Removing the storage settings doesn't delete the backups already uploaded there.

## Restore a backup

1. Go to **Backups** and find the backup in the list.
2. Select **Restore** on its row.
3. Type the name the dialog asks for (the domain, database name, folder path or app name) and select **Restore**.

The panel first checks the archive's SHA-256 checksum, and refuses a damaged file before it changes anything. If the file is no longer on this server, it is downloaded from remote storage.

::: danger
A restore overwrites the current content. Changes made since the backup are lost.
:::

What a restore replaces:

| Kind | What happens |
|---|---|
| **Website** | All files in the website's folder are deleted, except `tmp` and `logs`, and the backup's files are put back with the website's user as owner. Each database in the backup is emptied and filled from its dump. A database that no longer exists is created again, linked to the website, with a new password. For WordPress, the current database password is written into `wp-config.php` |
| **Database** | The database is dropped, created again empty, and filled from the dump |
| **Folder** | The backup is unpacked next to the current content first. Only when that worked is the folder's content replaced, so a broken backup leaves the folder as it was. Files added since the backup are deleted. Owners, permissions and symbolic links come back. A missing folder is created |
| **App** | The app's containers stop, its folder and data are replaced, the containers start again and the database dumps are loaded. The passwords saved in the backup come back too |

The dumps of website and database backups are loaded like an [import](./databases#import) on the **Databases** page: as a temporary MariaDB user that can use only that database, so a dump never writes into another database.

A website backup can only be restored while the website exists with the same domain. If you deleted the website, create it again with the same domain first, then restore.

## App backups

Docker apps from the [App store](./apps) keep their data outside the website's folder, in `/var/lib/panel-apps/<name>`. They get backups of the kind **App**:

- Backing up a website that runs a Docker app also makes an app backup, with the same schedule. Retention deletes old app backups together with the website's.
- To back up one app, choose **App** in **Back up now** or **Add schedule** on the **Backups** page, or go to **App store** → **Installed** → **Back up now**. Service apps such as PostgreSQL have no website, so give them a schedule of their own.
- Before an app update, the panel makes a backup first. If that backup fails, the update doesn't start.

Depending on the app, the panel pauses its containers while it copies their files, or dumps its databases with their own tools first. Restore app backups on the **Backups** page, like any other. A restore needs the app to be installed. After you uninstall an app, its backups stay in the list with its name and logo: install the app again (an app with a website on the same domain), then restore.

## Delete a backup

Open the menu of a backup and select **Delete**. The backup is deleted from this server and from remote storage.

## Alerts when a backup fails

The panel can send a message by Telegram or e-mail when a backup or a restore fails. Go to **Settings** → **Alerts** and turn on **A backup, restore or certificate renewal fails**. See [Mail and alerts](./mail-alerts).

The **Automatic backups** tab also shows **Failed** for a schedule whose last run failed.
