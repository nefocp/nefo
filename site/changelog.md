---
title: Changelog
description: What changed in each Nefo release, newest first, and what to know before you update.
---

# Changelog

Every Nefo release, newest first, with what is new, what changed and what was fixed, and what to know before you update. Each one links to its page on GitHub, which has the programs for amd64 and arm64, the installer, the checksums and their signature. To update, see [Update](./docs/update).

::: warning Going back after an update
v0.21.0, v0.22.0, v0.23.0 and v0.24.0 each change the format of the panel's database the first time they start. After that, an older version refuses to start with that data. If you may want to go back, copy `/var/lib/nefo` before you update, as [Update](./docs/update#if-an-update-fails) shows.
:::

## v0.24.1

28 November 2026 · [Release on GitHub](https://github.com/nefocp/nefo/releases/tag/v0.24.1)

Website backups that bring app and reverse proxy websites back, a faster security check, and fixes for the seven languages.

**Changed**

- **A website backup records the website's type,** so restoring it on a server that doesn't have the website brings back an app website (Node.js, Python, Java) with its runtime, start command, port and memory limit, and a reverse proxy website with its address and options. Install the app's runtime on the **Software** page first. Backups made by older versions still come back as static websites. The backup of a Docker app website is refused, with the steps to follow: install the app from the App store first, then restore the app's backup. See [Restore on a new server](./docs/restore-new-server).
- **The security check runs its checks side by side,** with at most 5 seconds for each program it asks and 25 seconds for the whole check. A check that can't finish in time is marked **partial result** instead of holding up the others. See [Security check](./docs/security-check).

**Fixed**

- On a busy server, the security check could take a long time and end with a server error.
- In a container with a memory limit, the dashboard counted the page cache and the memory the kernel frees when programs need it as used, so memory looked almost full after a backup or a big download. The swap settings now show the container's memory too.
- On a server with the PHP repository, the repository's newest PHP (8.6 on Ubuntu 24.04) was taken for the distribution's own PHP version: the **Software** page mixed up where the two came from, and a web server installed afterwards got that version instead of the distribution's.
- Apps that a newer App store catalog adds had no logo outside the App store page.
- In the longer languages, the account menu cut off the list of languages (it now has two columns), the tabs of a website's panel cut off their names (they now take two lines), and the top bar could wrap onto a second line on a narrow screen.
- Some Chinese, Russian and Indonesian texts of the interface were corrected.

## v0.24.0

27 November 2026 · [Release on GitHub](https://github.com/nefocp/nefo/releases/tag/v0.24.0)

Seven interface languages, backups to Google Drive, restore on a new server, and a security check.

**New**

- **The interface in seven languages:** Chinese, Spanish, Portuguese, Russian and Indonesian next to English and Vietnamese, App store texts included. Each language is a separate file that the browser downloads only when it is used, and the interface's files are now sent gzip-compressed: the main script goes from 478 KB to 163 KB.
- **Backups to Google Drive,** with an OAuth client of your own, a sign-in with a device code, access only to the files the panel creates, and uploads that resume after a break. See [Google Drive](./docs/backups#google-drive).
- **Encrypted remote copies:** each storage (S3, SFTP, Google Drive) can encrypt the backups it receives with a passphrase: scrypt and AES-256-GCM in numbered chunks, so a cut or changed copy is refused. `nefo decrypt-backup` opens a copy by hand. See [Encrypt the copies](./docs/backups#encrypt-the-copies).
- **Restore on a new server:** **Find backups** lists the backups on a storage that this panel doesn't know, and a restore creates a website that isn't there yet, with its domain, aliases, PHP version, files and databases with their passwords. Restoring the backup of a deleted website creates it again the same way. See [Restore on a new server](./docs/restore-new-server).
- **Websites:** clone Node.js, Python and Docker app websites, and websites with PostgreSQL databases. Deleting a website can keep its databases, and makes a backup first by default. A new website type, the reverse proxy, forwards to any address, on Nginx, Apache and OpenLiteSpeed. See [Websites](./docs/websites#reverse-proxy-websites).
- **Security check:** 22 quick checks of the server (SSH, firewall, updates, databases or Redis open to the internet, Docker ports that bypass ufw, files reachable over HTTP, certificates, PHP settings…), each with a link to the fix, on demand or daily. See [Security check](./docs/security-check).
- **Sign-in alerts** for SSH sign-ins (every one, or only from a new address), bursts of failed SSH sign-ins, and panel sign-ins. See [Sign-in alerts](./docs/mail-alerts#sign-in-alerts).
- **Country blocking** per website, with DB-IP's free country data, downloaded only once a website uses it. It works on reverse proxy websites too. See [Country blocking and file lock](./docs/website-protection).
- **A file lock** per website: its code becomes read-only for its PHP and SFTP account except the folders it must write to, with an optional daily alert when PHP files change. `nefo unlock-files` gives the files back before you remove Nefo.
- **Databases:** MariaDB 11.4, 11.8 and 12.3 LTS from MariaDB's repository, and an in-place upgrade with a dump first and a way back if it fails; a switch between MariaDB and MySQL that carries the databases over; rotation of the slow query log. See [Databases](./docs/databases#mariadb-versions).
- **PostgreSQL:** versions 15 to 19 from the PostgreSQL project, remote access per database (TLS, scram-sha-256, a firewall rule per address), and a major upgrade with a dump first and a way back. The dumps that upgrades keep are listed, with a delete button. See [PostgreSQL](./docs/postgresql).
- **Email records on the DNS page:** MX, one merged SPF record and DMARC for Google Workspace, Microsoft 365, Zoho Mail, Cloudflare Email Routing or another provider, through Cloudflare. See [DNS (Cloudflare)](./docs/dns#set-up-email-records).
- **Software:** Java (OpenJDK 17, 21 and 25) for app websites, the ionCube Loader for each PHP version, and PHP 8.6, offered by itself once the PHP repository has it. See [Software](./docs/software).
- **App store:** 15 more apps, 42 in all: Grafana, ntfy, Gotify, Memos, linkding, Navidrome, Vikunja, Actual Budget, changedetection.io, Healthchecks, listmonk, Kanboard, FreshRSS, Jellyfin and Mealie. See [App store](./docs/apps).

**Changed**

- Memory figures follow a container's own memory limit (LXC), on the dashboard, in alerts, in the setup and in the App store's checks.
- The setup page's disk figures were corrected, and count apt's package lists on a server that never downloaded them.
- A newer App store catalog with apps that an older panel can't run leaves out just those apps. Apps that a newer catalog adds show their names everywhere.

**Fixed**

- PHP 8.5 could not be installed, because OPcache no longer has a package of its own.
- On Ubuntu 26.04, installing PostgreSQL (or another service) could fail while systemd re-executed itself after apt. The panel now retries `systemctl` while systemd can't be reached.
- The web firewall was offered where the distribution has no ModSecurity module for the web server (Ubuntu 22.04 with Nginx). It is now shown as not offered, with the reason.
- The empty WordPress page's text ended in the middle of a sentence.

## v0.23.0

26 November 2026 · [Release on GitHub](https://github.com/nefocp/nefo/releases/tag/v0.23.0)

API keys, website import, a setup page, and more database servers and PHP versions.

**New**

- **API keys** for your own scripts, on **Settings** → **API keys**: read-only or full access, an allow-list of addresses and an expiry date. A key is sent in an `Authorization: Bearer` header and stored only as a hash, and a wrong key counts as a failed sign-in. No key can open the terminal, Adminer, phpMyAdmin, the account or the keys themselves. A read-only key can't read passwords, files, exports or backup downloads. See [API keys](./docs/api).
- **Import websites** from a cPanel full backup (websites, databases with their users and passwords, certificates and cron jobs), or from a site archive and SQL dumps made by aaPanel or another server. Hostile archives are refused, MySQL 8 dumps are adapted for MariaDB, and a failed import removes everything it created. See [Import websites](./docs/import).
- **A setup page** at the first sign-in: choose the web server, PHP versions, database server and extras on one page, see roughly how much memory and disk they need, and install them as one task. `nefo setup` does the same on the command line, and the installer accepts the same options. Without options, the installer behaves as before. See [First steps](./docs/first-steps#set-up-the-server).
- **MySQL 8.4 LTS** as an alternative to MariaDB (one or the other per server), from Oracle's repository with its key pinned. It is offered on amd64 servers only.
- **phpMyAdmin**, reachable only through the panel's sign-in.
- **PostgreSQL** on the **Databases** page: databases, owners, import and export, and backups.
- **Memcached**, listening on the server itself only, without UDP.
- **PHP 5.6 to 7.3**, for websites moved from older servers. They install only after you confirm, and can never be the server's only PHP version.
- **End-of-life marks for PHP:** a version past the end of PHP's security support is marked on the **Software** page and on its websites. Your distribution's own PHP version is never marked, because the distribution keeps patching it. See [Software](./docs/software#end-of-life).
- **Composer** and **WP-CLI** on the **Software** page.

**Changed**

- On Ubuntu, the extra PHP versions now come from `packages.sury.org`, because `ppa:ondrej/php` no longer receives updates. A server that has the PPA switches once, when the panel starts: the PPA's source files are moved to `/var/lib/nefo/apt-backup`, nothing is installed or removed, and the installed PHP packages update from the new source after the next `apt-get update`.

**Fixed**

- A website created as suspended is now also saved as suspended.
- Folders that the panel creates for other programs no longer get the panel's own strict permissions (umask 077).
- The distribution's own PHP version is recognised on a fresh server, before apt has downloaded its package lists.
- A bare `@` (the domain itself) no longer breaks a message on the DNS page.
- A task that crashes now writes its stack trace to the panel's log.

## v0.22.0

26 November 2026 · [Release on GitHub](https://github.com/nefocp/nefo/releases/tag/v0.22.0)

Website monitoring, more alert channels and Cloudflare DNS.

**New**

- **Website monitoring:** each website is checked every 1 to 10 minutes (the status code, an optional keyword and the certificate's expiry). A website counts as down after two failed checks, and you get an alert when it goes down and when it is back. The panel shows uptime over 24 hours and 7 days, response times and recent failures. Monitoring requests are left out of the traffic statistics. See [Monitoring](./docs/websites#monitoring).
- **Alerts by Discord, Slack and webhook.** Webhooks are signed with HMAC-SHA256. Saved URLs are masked, and addresses on the server itself, link-local addresses and cloud metadata addresses are refused when an alert is sent. See [Mail and alerts](./docs/mail-alerts).
- **A DNS page for Cloudflare zones:** list, add, edit and delete records, turn the proxy on or off, purge the cache, and point a website's domain at this server with a preview first. See [DNS (Cloudflare)](./docs/dns).

## v0.21.0

26 November 2026 · [Release on GitHub](https://github.com/nefocp/nefo/releases/tag/v0.21.0)

System tools, SSH keys and more website options.

**New**

- **A System page:** operating system updates (security updates marked, install all or security updates only, automatic security updates, and a notice when a restart is needed), a swap file and swappiness, the time zone, clock synchronisation, the host name, TCP BBR and disk cleanup. The background check for updates waits 30 minutes after the panel starts, and reuses package lists refreshed in the last 6 hours. See [System](./docs/os).
- **SSH keys** on the **Firewall & SSH** page: list, add and remove. Private keys, weak keys and duplicates are refused, and the last way in can't be removed.
- **Websites:** turn HTTPS off and delete the certificate, on all three web servers; HSTS, with `includeSubDomains` as an option; a page cache per PHP website on Nginx, with a purge button; clone a PHP or static website with its files, settings and databases; push a WordPress staging copy back to the live website, after a backup.

**Fixed**

- The panel's own apt runs never overlap, and it waits for another program's apt lock instead of failing.
- In a container where `/etc/hosts` is bind-mounted, the host name is written in place. A host name that the system refuses gives a clear error.

## v0.20.2

26 November 2026 · [Release on GitHub](https://github.com/nefocp/nefo/releases/tag/v0.20.2)

**Changed**

- Nginx now compresses CSS, JavaScript, SVG and fonts. Debian and Ubuntu turn compression on for HTML only, so these files were sent uncompressed. Settings that `nginx.conf` already has are left as they are. Existing servers get the change when the updated panel starts.

**Fixed**

- Each release now carries the signed app store catalog. Before, panels looked for it once a day and logged an HTTP 404.

## v0.20.1

26 November 2026 · [Release on GitHub](https://github.com/nefocp/nefo/releases/tag/v0.20.1)

**Fixed**

- The installer failed on every supported system: it read the system's version from `/etc/os-release` in place of its own, and tried to download a release that doesn't exist. It was tested on fresh Ubuntu 24.04 and Debian 12 servers after the fix.

## v0.20.0

26 November 2026 · [Tag on GitHub](https://github.com/nefocp/nefo/releases/tag/v0.20.0)

The first public release. Its installer failed (fixed in v0.20.1), and GitHub has no release files for it, only the tag. Install a newer release.

It contained websites on Nginx, Apache or OpenLiteSpeed with PHP-FPM, Node.js and Python apps, Let's Encrypt certificates, deploys from Git and the WordPress toolkit; the app store with PHP and Docker apps; MariaDB, Redis, the file manager, the web terminal and backups to S3-compatible storage and SFTP; the firewall (ufw), fail2ban and SSH settings with lock-out guards, the web application firewall and the malware scanner; signed releases and `nefo update`; and the interface in English and Vietnamese.
