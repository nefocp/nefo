---
title: Backups
description: Back up websites, databases, folders and apps on a schedule with retention, to this server, S3-compatible storage, an SFTP server or Google Drive, encrypted if you like, and restore them in one click, also on a new server.
---

# Backups

Nefo backs up websites, databases, folders and App store apps as `.tar.gz` archives, by hand or on a daily or weekly schedule with retention. It keeps each backup on the server and can upload a copy to S3-compatible storage, an SFTP server or Google Drive, encrypted with a passphrase if you like. A restore needs only the backup and its name typed to confirm, also on a new server.

This page covers what a backup contains, backing up by hand and on a schedule, remote storage and its encryption, restoring, and alerts when a backup fails. To restore on another server, see [Restore on a new server](./restore-new-server).

## What a backup contains

| Kind | Contains |
|---|---|
| **Website** | The files of the website's folder, `/var/www/example.com`, except its top-level `tmp` and `logs` folders, and an SQL dump of every database linked to the website |
| **Database** | An SQL dump of one database |
| **Folder** | Any folder of the server, such as `/etc/nginx` or `/opt/app`, with owners, permissions and symbolic links |
| **App** | A Docker app from the App store: its folder in `/var/lib/panel-apps`, its saved passwords, its containers' data and dumps of its databases (see [App backups](#app-backups)) |

Link a database to its website when you create it (see [Databases](./databases#create-a-database)), so the website's backup includes it. Website backups leave out symbolic links and special files.

Every backup is a `.tar.gz` archive with a `manifest.json`, the files under `files/` and the database dumps under `databases/`. Database dumps are made with `mariadb-dump --single-transaction`, so websites keep running.

The `manifest.json` of a website backup also records what the website needs to be created again on another server: its aliases, PHP version, rewrite rules, its own web server directives, and the passwords of its databases' users.

Backups on this server are not encrypted. They are readable by root only. Each remote storage can encrypt its copies with a passphrase before they leave the server (see [Encrypt the copies](#encrypt-the-copies)).

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

The backup Nefo makes before it deletes a website (**Back up the website first**) counts as one made by hand: it is never deleted automatically. Its name ends in `_before-delete_<date>-<time>.tar.gz`.

Deleting a schedule stops it. Its existing backups stay. Deleting a website, a database or an app also deletes its schedules, and its backups stay on the **Backups** page.

## Remote storage

A copy on another provider protects you if this server is lost. Go to **Backups** → **Remote storage** and select **Set up** (or **Add storage** when one is set up already). You can set up S3-compatible storage, an SFTP server and Google Drive at the same time: backups are then uploaded to each of them.

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

### Can Nefo back up to Google Drive? {#google-drive}

Yes, from v0.24.0. Backups go to a folder in your Google Drive. The panel asks only for access to the files it creates itself (Google's `drive.file` permission): it can't see or change anything else in your Drive. A folder you create by hand in Drive is invisible to the panel, even with the right name.

Google Drive needs an OAuth client of your own, in a Google Cloud project. It costs nothing and you create it once.

Google Drive was tested against a simulated Google service only, not against a real Google account.

#### Create the OAuth client

The names in bold in these steps are Google's, not the panel's.

1. Open the [Google Cloud console](https://console.cloud.google.com/) and create a project, or pick one.
2. Under **APIs and services** → **Library**, enable the **Google Drive API**.
3. Set up the **OAuth consent screen** with the user type **External**, then set its publishing status to **In production**.
4. Under **Credentials**, create an **OAuth client ID** of the type **TVs and Limited Input devices**.
5. Keep the client ID (it ends in `.apps.googleusercontent.com`) and the client secret.

::: warning
While the consent screen's publishing status is **Testing**, Google ends the panel's access after 7 days, and uploads then fail. Set it to **In production**.
:::

#### Connect

1. In **Remote storage**, select **Set up** or **Add storage**, and choose **Google Drive** in **Type**. **How to create the OAuth client** repeats the steps above.
2. Enter the **Client ID** and the **Client secret**.
3. Optional: change the **Folder in Google Drive**. The default is `Nefo backups/<the server's host name>`. Separate folders with `/`; missing folders are created.
4. Optional: tick **Encrypt backups before upload** (see [Encrypt the copies](#encrypt-the-copies)).
5. Select **Connect to Google Drive**. The panel shows an address, `google.com/device`, and a code such as `ABCD-EFGH`.
6. On any device (your phone works), open that address, sign in with the Google account that should hold the backups, enter the code and allow access.
7. The panel waits while you do this, and shows how long the code stays valid. Once Google answers, the panel creates the folder, writes, lists, reads and deletes a small test file there, and saves the settings.

The **Remote storage** list then shows the folder, the Google account and a link, **Open in Google Drive**.

You can leave the page while the panel waits: open **Backups** → **Remote storage** again to see the code.

#### Change or reconnect

Select **Change** on the Google Drive row:

- To change the folder or the encryption, edit them and select **Test and save**. No new sign-in is needed. A new folder is created if needed. Backups uploaded before stay in the old folder, and the panel no longer finds them there for a restore.
- To use another Google account or another OAuth client, or after the access was revoked, select **Connect again** and enter the new code. Leave the **Client secret** empty to keep the saved one of the same client.

**Test and save** also mends a folder that was deleted or moved to the Drive trash: it creates the folder again.

Keep the same Google Cloud project. Google gives each project access only to the files that project created, so with an OAuth client of another project the panel can't see the backups it uploaded before.

#### How uploads work

- Each backup is uploaded in pieces of 8 MB. If the connection breaks or Google is busy, the panel asks Google how much arrived and goes on from there. It retries for a few minutes before it gives up.
- Before an upload, the panel checks the free space of the Google account and refuses at once when the backup doesn't fit.
- Downloads for a restore also go on from where they stopped.
- The folder in Drive has the same sub-folders as `/var/backups/panel`: `site/example.com/…`, `database/shop/…`.

#### Disconnect

**Delete** on the Google Drive row forgets the settings and tells Google that the panel gives up its access. Backups already uploaded stay in your Google Drive.

#### When something goes wrong

| Message | What to do |
|---|---|
| Google Drive access was revoked or has expired | Select **Change** → **Connect again**. If it happens about 7 days after connecting, set the consent screen's publishing status to **In production** first. |
| Google Drive is full | Free up space in the Google account or buy more storage, then upload again with **Upload**. |
| The Google Drive folder … was deleted / is in the Drive trash | Restore the folder from the Drive trash (its backups come back with it), or select **Change** → **Test and save** to create a new folder. |
| Google refused the OAuth client | Check the client ID and secret, and that the client's type is **TVs and Limited Input devices**. |
| The code expired before it was entered | Select **Connect to Google Drive** again for a new code. |

### Encrypt the copies

Each storage (S3, SFTP and Google Drive) can encrypt the backups it receives. The copy on the storage is then unreadable without the passphrase, for the provider and for anyone who gets into the storage account. The copy on this server stays a normal `.tar.gz`.

1. In the storage's form, tick **Encrypt backups before upload**.
2. Enter a **Passphrase** of at least 12 characters, and the same in **Repeat the passphrase**. A sentence of several words works well.
3. Select **Test and save** (or **Connect to Google Drive**). The test file is encrypted too.

::: danger
Without the passphrase, the encrypted backups can't be restored: not by you, not by the panel, not by anyone. Keep it outside this server, for example in a password manager. If this server is lost, the passphrase is all you need, together with the storage, to get the backups back.
:::

What changes:

- The encrypted copy is named like the backup with `.enc` added: `example.com_20261126-030000.tar.gz.enc`.
- On the **Backups** list, the storage's tag in the **Stored** column shows a lock instead of a cloud.
- The storage's row in **Remote storage** shows **Encrypted**.
- The passphrase is never shown again. To keep it when you change other settings, leave **Passphrase** empty.

Changing the passphrase:

- A new passphrase applies to new uploads only. Copies already uploaded keep the passphrase they were encrypted with; they aren't encrypted again.
- This server remembers the earlier passphrases (up to 10), so restores here keep working.
- On another server, you need the passphrase that was in use when the copy was uploaded.
- If you turn encryption off, copies already uploaded stay encrypted, and this server keeps their passphrase to restore them.

How it works: the panel encrypts the archive on this server, while it uploads it, with AES-256-GCM. The key comes from the passphrase through scrypt, with a new random salt for every file, so two copies of the same backup look different. The archive is sealed in pieces of 64 KB, each one numbered and the last one marked, so a copy that was changed, cut short or put together from pieces of other copies is refused. No key is stored on the server apart from the passphrase in the panel's settings. The file format is described in [Restore on a new server](./restore-new-server#format-of-encrypted-copies).

Encrypting costs little: deriving the key takes about a quarter of a second and 64 MB of memory for a moment, once per upload, and the copy is larger by a 64-byte header and 16 bytes per 64 KB. For S3 storage, the panel reads the backup twice, because S3 needs the checksum of the encrypted copy before the upload starts.

### Uploading and removing storage

- A backup made without **Also upload to remote storage** can be uploaded later: open its menu and select **Upload**. The panel copies it to each storage that doesn't have it yet, encrypted for the storages that encrypt.
- The **Stored** column shows **This server** and a tag for each remote copy: S3, SFTP or Google Drive, with a lock for an encrypted copy.
- Removing the storage settings doesn't delete the backups already uploaded there.
- **Find backups** on a storage's row lists the backups on it that are not in the list, for example on a new server: see [Restore on a new server](./restore-new-server).

## Restore a backup

1. Go to **Backups** and find the backup in the list.
2. Select **Restore** on its row.
3. Type the name the dialog asks for (the domain, database name, folder path or app name) and select **Restore**.

The panel first checks the archive's SHA-256 checksum, and refuses a damaged file before it changes anything. If the file is no longer on this server, it is downloaded from remote storage.

If the copy is encrypted, the panel decrypts it while it downloads it, with the storage's passphrase (or an earlier one this server remembers). The SHA-256 check then runs on the decrypted archive, as for any other backup. When a backup is only in remote storage and its copy is encrypted, the **Restore** dialog has a field **Encryption passphrase (optional)**. Leave it empty to use the storage's passphrase. Enter the passphrase that was in use when the copy was uploaded if the storage was set up again since, for example after you removed it and added it with a new passphrase.

The restore stops before anything changes, with a message in the log, when:

- the passphrase doesn't unlock the copy ("this copy is encrypted and the passphrase does not unlock it");
- the copy was changed or damaged ("the encrypted copy is damaged or was changed");
- the copy is incomplete ("the encrypted copy is incomplete").

If there are copies on several storages, the panel tries the next one.

::: danger
A restore overwrites the current content. Changes made since the backup are lost.
:::

What a restore replaces:

| Kind | What happens |
|---|---|
| **Website** | All files in the website's folder are deleted, except `tmp` and `logs`, and the backup's files are put back with the website's user as owner. Each database in the backup is emptied and filled from its dump. A database that no longer exists is created again, linked to the website, with its user's password from the backup (backups made before v0.24.0 don't have it: the database then gets a new password). A database that is no longer linked to a website is linked to the restored one. For WordPress, the current database password is written into `wp-config.php` |
| **Database** | The database is dropped, created again empty, and filled from the dump |
| **Folder** | The backup is unpacked next to the current content first. Only when that worked is the folder's content replaced, so a broken backup leaves the folder as it was. Files added since the backup are deleted. Owners, permissions and symbolic links come back. A missing folder is created |
| **App** | The app's containers stop, its folder and data are replaced, the containers start again and the database dumps are loaded. The passwords saved in the backup come back too |

The dumps of website and database backups are loaded like an [import](./databases#import) on the **Databases** page: as a temporary MariaDB or MySQL user that can use only that database, so a dump never writes into another database.

### Restore a website that was deleted

If the website of a backup no longer exists on this server, for example because you deleted it, the restore creates it again from the backup: the same domain and aliases, PHP version, rewrite rules, files and databases. The restore dialog says so first. What a backup doesn't hold, such as the certificate, cron jobs and SFTP access, isn't created. See [Restore on a new server](./restore-new-server#restoring-a-website-that-doesn-t-exist-here) for the details.

Backups made by Nefo 0.24.1 or later record each website's type, so a website that no longer exists comes back as the same kind: an app website (Node.js, Python, Java) with its runtime, start command, port and memory limit, and a reverse proxy website with its address and options. If the app's runtime isn't installed, the restore stops and asks you to install it on the **Software** page first. Backups made by older versions record only the PHP version: such a website comes back as a static website, so create it again with the same domain and type first, then restore. For a Docker app, install the app again on the same domain from the [App store](./apps), then restore its backup of the kind **App**, and the website's backup if the website's folder held files of its own, such as Navidrome's music.

## Restore on another server

A new server with Nefo can find the backups on your remote storage with **Find backups**, add them to its list, and restore them, websites included. You need the storage's access details, and the passphrase if the copies are encrypted. See [Restore on a new server](./restore-new-server).

## App backups

Docker apps from the [App store](./apps) keep their data outside the website's folder, in `/var/lib/panel-apps/<name>`. They get backups of the kind **App**:

- Backing up a website that runs a Docker app also makes an app backup, with the same schedule. Retention deletes old app backups together with the website's.
- To back up one app, choose **App** in **Back up now** or **Add schedule** on the **Backups** page, or go to **App store** → **Installed** → **Back up now**. Service apps such as PostgreSQL have no website, so give them a schedule of their own.
- Before an app update, the panel makes a backup first. If that backup fails, the update doesn't start.

Depending on the app, the panel pauses its containers while it copies their files, or dumps its databases with their own tools first. Restore app backups on the **Backups** page, like any other. A restore needs the app to be installed. After you uninstall an app, its backups stay in the list with its name and logo: install the app again (an app with a website on the same domain), then restore.

## Delete a backup

Open the menu of a backup and select **Delete**. The backup is deleted from this server and from remote storage.

## Alerts when a backup fails

The panel can send an alert by Telegram, e-mail, Discord, Slack or a webhook when a backup or a restore fails. Go to **Settings** → **Alerts** and turn on **A backup, restore or certificate renewal fails**. See [Mail and alerts](./mail-alerts).

The **Automatic backups** tab also shows **Failed** for a schedule whose last run failed.
