---
title: Import websites
description: Bring websites over from cPanel, aaPanel, HestiaCP, CyberPanel or another server, with their files, databases, certificates and cron jobs.
---

# Import websites

Nefo imports websites from another server in two ways:

- **cPanel full backup**: every website of a cPanel account, with its databases and their users, certificates and cron jobs.
- **Website archive and SQL dumps**: the files of one website as a `.tar.gz`, `.tgz` or `.zip` archive, with optional `.sql` or `.sql.gz` dumps. Use it for aaPanel, HestiaCP, CyberPanel or any other server.

The import never touches the old server. Nefo first reads the archive and shows what it found; nothing changes on this server until you confirm.

## Make the backup

### cPanel

In cPanel, open **Files → Backup** and click **Download a Full Account Backup**. Keep **Home Directory** as the destination, then click **Generate Backup**. cPanel writes `backup-M.D.YYYY_HH-MM-SS_USER.tar.gz` into your home folder and e-mails you when it is ready.

With root access to WHM, run `/scripts/pkgacct USER` instead: it writes `/home/cpmove-USER.tar.gz`. Both files have the same layout and Nefo reads both, also from old cPanel versions.

A partial backup ("Home Directory" or "MySQL Databases" downloads) is not a full backup. For those, use **Website archive and SQL dumps**.

### aaPanel

1. Open **Website**, click **Backup** in the website's row, then **Backup now**. aaPanel writes `/www/backup/site/DOMAIN/web_DOMAIN_YYYYMMDD_HHMMSS_xxxxxx.tar.gz` (older versions: `/www/backup/site/`).
2. Open **Database**, click **Backup** in the database's row, then **Backup now**. aaPanel writes `db_NAME_YYYYMMDD_HHMMSS_mysql_data.sql.gz` under `/www/backup/database/`.

The site archive has the website's folder at its top; Nefo imports what is inside it. Nefo also reads the old folder (`/www/wwwroot/DOMAIN`) from the site's `.user.ini` and replaces it in WordPress's database.

### HestiaCP, CyberPanel and other servers

Their own backups pack each website in archives of their own (HestiaCP compresses them with zstd), so make a plain archive instead, over SSH on the old server:

```bash
# The website's files: the folder that holds index.php or index.html
tar czf /root/site.tar.gz -C /home/USER/web/example.com public_html
# Each database
mysqldump --single-transaction --routines --triggers DBNAME | gzip > /root/DBNAME.sql.gz
```

On HestiaCP the folder is `/home/USER/web/DOMAIN/public_html`, on CyberPanel `/home/DOMAIN/public_html`, on a plain server usually `/var/www/html`. An archive whose top is a `public_html` folder works as well: Nefo imports what is inside it.

## Get the archive onto this server

Any of these:

- **SFTP or scp** to a folder of root, for example `scp backup.tar.gz root@NEW-SERVER:/root/`.
- **Files**: upload it, or use **Download from URL** (see [Files](./files)).
- **A web address**: give the import an `http` or `https` address. Nefo downloads the file into `/var/tmp/nefo-import`, on this server's disk, and removes it after the import. Addresses of this server's own networks are refused.

Nefo reads archives from anywhere except system folders (`/etc`, `/proc`, `/sys`, `/dev`, `/run`, `/boot`) and its own data. The disk needs room for the websites' files and about twice the size of the dumps; the plan shows what is needed and what is free.

## Import step by step

1. Open **Websites** and click **Import** next to **Add website**.
2. Choose **cPanel full backup** or **Website archive and SQL dumps**.
3. Enter the path of the file (**Browse** lists the folders and archives of this server) or its web address. For a website archive, add the SQL dumps.
4. Click **Analyse**. Nefo reads the whole archive once, without unpacking it; large backups take a few minutes. **Stop** stops it.
5. Look through the plan and tick what to bring over (see below). Click **Import**.
6. Follow the log. The new websites stay closed (error 503) until the import is done. **Stop the import** removes what was created so far.
7. When it is done, the page lists the new websites with **Open website** and **Manage**, the databases, and what is worth a look.

If you leave the page, the import goes on; open **Websites → Import** again to see it. An analysed archive is remembered for six hours.

## What is imported from cPanel

| In the backup | On Nefo |
|---|---|
| The main domain, each addon domain and each subdomain | One website each. cPanel's internal subdomain of an addon domain (`addon.main.com`) is left out. |
| Parked domains (aliases) | Aliases of the main domain's website, with their `www.` when the old server had it. |
| Each document root (`public_html`, `public_html/blog`, `addon.com`…) | The website's `public` folder. A folder of another imported domain inside it goes to that domain's website only. |
| The PHP version (`ea-php81`) | The same version when it is installed here, else this server's default. A website without PHP files becomes a static website. |
| `mysql/DB.sql` | A database of the same name when it is free here, else `NAME_2`, linked to the website that uses it. |
| Database users and their passwords (`mysql.sql`) | Each database gets one user: the one its WordPress `wp-config.php` names, else one with access to it. The user keeps its old password: MariaDB takes the old password hash as it is, or Nefo uses the password from `wp-config.php`. Otherwise the user gets a new password. |
| WordPress | Found by its `wp-config.php`. Nefo marks the website as WordPress, applies the WordPress rewrite rules, points `wp-config.php` at the new database name, user or password when they changed, and replaces the old folder (`/home/USER/public_html`) with the new one in the database. |
| Certificates (`apache_tls/`, `sslcerts/`, `ssl/`) | Installed as your own certificate when you tick it: only when its key is in the backup, it is valid today and covers every name of the website. Otherwise get one from Let's Encrypt after the import. |
| Cron jobs (`cron/USER`) | Shown, and installed only when you tick them: as the website's own user (never root), with the old document roots and cPanel's PHP programs replaced by this server's. A command that still names the old home folder is marked. |
| `.htaccess`, `.user.ini` | The old folder is replaced by the new one, and cPanel's PHP handler lines leave `.htaccess`. |

The plan says why a domain can't be imported, for example when a website here already has the domain.

## What is not imported

- **Mail**: mailboxes, forwarders, autoresponders and filters. Nefo has no mail server. Move mail to a mail service before you turn the old server off. For mail that the websites send, see [Mail and alerts](./mail-alerts).
- **DNS zones**: Nefo runs no DNS server. Point the domains at this server where their DNS is managed, or with [DNS (Cloudflare)](./dns).
- **FTP accounts**: websites here use SFTP, one account per website (the website's **SFTP** tab).
- Mailing lists, PostgreSQL databases, the files of the home folder outside the document roots, and cPanel's own settings.

## Website archives

- One folder at the top of the archive (as in aaPanel's backups) is not kept: what is inside it becomes the website. A home folder with `public_html` in it works as well.
- The website can be a new one (its domain, `www.` and PHP version are asked) or an existing empty one: a PHP or static website whose `public` folder holds nothing but its placeholder page.
- Each SQL dump becomes a new database with a user of the same name and a new password. A WordPress `wp-config.php` is pointed at the dump it names (or at the only one). For other applications, change their configuration to the new database, user and password (**Show login** on the **Databases** page).

## Dumps from MySQL 8

Dumps made by MySQL 8, as aaPanel and many cPanel servers run it, use collations MariaDB doesn't know, such as `utf8mb4_0900_ai_ci`, and lines MariaDB refuses. Nefo adapts them while they stream in, without changing your data:

- MySQL 8 collations become the closest MariaDB one (`utf8mb4_unicode_520_ci`, `utf8mb4_bin` for the case-sensitive ones, or the language's own, such as `utf8mb4_vietnamese_ci`).
- Settings only MySQL 8 knows, such as `DEFAULT ENCRYPTION='N'` and `SRID`, become plain comments.
- `SET @@GLOBAL.GTID_PURGED` and the `SQL_LOG_BIN` lines around it are left out.
- `CREATE DATABASE` and `USE` for the dump's own database are left out, so it goes into the new database.

Text inside your data is never changed. The log says how many lines were adapted.

## How Nefo treats archives

An archive from another server may hold anything, so Nefo checks every entry:

- Names with `..` or an absolute path, symbolic links, hard links, devices and FIFOs are left out, and the log says how many.
- Files lose setuid and setgid, and write access for the group and others. Everything belongs to the website's own Linux user.
- Files are written through the website's folder only: nothing can land outside it.
- A file over 8 GB is left out; an archive that unpacks to far more than its own size (an archive bomb) is refused.
- SQL runs as a temporary database account that may only use the new database, never as root.

If the import fails or is stopped, Nefo removes the websites and databases it created. A website it could not remove stays suspended, with a note saying the import did not finish.

## After the import

1. **Test before DNS**: point the domains at this server in your computer's `hosts` file and open the websites.
2. **DNS**: point each domain's A (and AAAA) record at this server.
3. **HTTPS**: once DNS points here, get Let's Encrypt certificates on each website's **SSL / HTTPS** tab. An imported certificate is not renewed by Nefo. See [SSL certificates](./ssl).
4. **PHP**: check each website's PHP version and settings on its **PHP** tab.
5. **Cron jobs**: add the ones you didn't import on the **Cron jobs** page.
6. **Mail**: make sure mail works elsewhere before you turn the old server off.

## If something goes wrong

- **"not a cPanel full backup"**: the archive has no `userdata/main`. It is a partial backup or a plain archive: choose **Website archive and SQL dumps**.
- **"uses another database"**: the dump writes into a second database. Export each database on its own, without `--databases` or `--all-databases`.
- **"not enough free disk space"**: free some space, or import fewer websites at a time.
- **A WordPress site shows "Error establishing a database connection"**: open its `wp-config.php` in **Files** and compare `DB_NAME`, `DB_USER` and `DB_PASSWORD` with **Show login** on the **Databases** page.
