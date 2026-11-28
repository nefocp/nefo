---
title: What is Nefo?
description: Nefo is a free, open-source control panel that you install on your own Linux server to run websites, databases, apps, backups and security from your browser.
---

# What is Nefo?

Nefo is a free, open-source control panel (GNU AGPLv3) that you install on your own Linux server with one command. From the browser, in one of seven languages, you manage websites and their SSL certificates, databases, apps, files, backups and security. It is one program, built for a VPS with 1 GB of RAM, and it sends no telemetry.

This page explains what Nefo does, who it is for, what it deliberately leaves out, and what it sends over the network.

## A control panel in one program

You install Nefo on a VPS with one command. It is a single program, `/usr/local/bin/nefo`, with the web interface built in, and it runs as the systemd service `nefo`. You then manage the server from your browser: websites, certificates, databases, files, backups, software and security.

- **Every feature is free.** There is no paid tier and no per-site or per-account fee.
- **No account and no licence server.** Nothing has to be registered or activated.
- **Built for small servers.** Nefo is designed to run on a VPS with 1 GB of RAM, on x86-64 and ARM64. The panel itself uses about 34 MB of memory, measured on a test server after the update to v0.24.0.
- **Seven interface languages:** English, Vietnamese, Chinese, Spanish, Portuguese, Russian and Indonesian. You pick the language on the sign-in page or in the account menu, and the browser downloads only the language you use.

To get started, check the [requirements](./requirements), then [install Nefo](./install) and follow the [first steps](./first-steps).

## Who it is for

Nefo is for people who run their own VPS: someone setting up a first server for a blog, a developer deploying an app, or a freelancer or agency looking after client websites.

Each server has one panel account, `admin`. Nefo has no separate logins for customers or resellers.

## What you can do with it

### Websites

- Host PHP, static and app websites (Node.js, Python or Java) on Nginx, Apache or OpenLiteSpeed, and reverse proxy websites that forward to another address. Each website gets its own Linux user, and optionally an SFTP account locked in its folder. Clone a website to a new domain, and delete one with a backup first. See [Websites](./websites).
- Turn on HTTPS with free Let's Encrypt certificates that renew automatically, wildcard certificates through Cloudflare DNS, or your own certificate. See [SSL certificates](./ssl).
- Block or allow visitors by country, and lock a website's code read-only for its PHP. See [Country blocking and file lock](./website-protection).
- Deploy from GitHub, GitLab, Gitea or any Git server, on demand or on every push. See [Deploy from Git](./git-deploy).
- Keep WordPress sites up to date, with a database backup before each update, maintenance mode and a staging copy. See [WordPress toolkit](./wordpress).
- Install 42 applications in one click: PHP applications such as WordPress, Joomla, Nextcloud and Laravel, and Docker applications such as Ghost, Uptime Kuma, n8n, Gitea, Grafana and Jellyfin. See [App store](./apps).

### Data

- Run MariaDB (your distribution's, or 11.4, 11.8 or 12.3 LTS) or MySQL 8.4 tuned for small servers, with databases, users, SQL import and export and remote access per database, and switch between the two with your databases. Adminer and phpMyAdmin sit behind the panel's sign-in. Browse Redis keys, and add Memcached. See [Databases](./databases).
- Run PostgreSQL, your distribution's version or 15 to 19 from the PostgreSQL project, with its own databases and owners, remote access over TLS and major upgrades. See [PostgreSQL](./postgresql).
- Import websites from a cPanel full backup, or from a site archive and SQL dumps made by aaPanel, HestiaCP, CyberPanel or any server. See [Import websites](./import).
- Manage files with an editor, upload and download, archives and a trash that keeps deleted files for 30 days. See [Files](./files).
- Back up websites, databases, folders and apps on a schedule, to this server, S3-compatible storage, an SFTP server or Google Drive, encrypted with a passphrase if you like, and restore them, also on a new server. See [Backups](./backups) and [Restore on a new server](./restore-new-server).

### Server

- Install and uninstall each component (web server, PHP versions and the ionCube Loader, MariaDB or MySQL, PostgreSQL, Redis, Node.js, Python, Java, Docker and more). Nefo refuses to remove one that websites, databases or containers still use. See [Software](./software).
- Run Docker containers and Compose projects. Nefo checks Compose files for unsafe settings, such as privileged containers or ports open to the internet. See [Docker](./docker).
- Control the firewall (ufw), fail2ban, SSH settings, a web application firewall per website and a malware scanner. See [Security](./security).
- Run the security check: 22 quick checks of the server, each with a link to the fix, on demand or daily. See [Security check](./security-check).
- Use a terminal in the browser, and manage services, processes, background programs and cron jobs. See [System tools](./system).
- Watch that each website answers, with its uptime and response times. See [Monitoring](./websites#monitoring).
- Let websites send e-mail through an SMTP relay, and get alerts by Telegram, e-mail, Discord, Slack or a webhook, also when someone signs in. See [Mail and alerts](./mail-alerts).
- Manage the DNS records of your Cloudflare zones, point a website's domain at this server, and add the MX, SPF and DMARC records of your mail provider. See [DNS (Cloudflare)](./dns).

## Security of the panel itself

Nefo runs as root, so it protects its own sign-in:

- The panel answers only under a secret path such as `/k3x9p2qa7m1z/`. Every other address on its port returns `404`.
- It uses HTTPS from the first start. The certificate is self-signed at first, and you can replace it with a free certificate for the panel's own domain.
- Sign-in supports two-factor authentication. After 5 failed attempts in 15 minutes, an address is locked out for 15 minutes.
- Sign-ins and changes are written to an activity log, which keeps entries for 90 days.

## What Nefo doesn't do

- **No mail server.** Nefo doesn't receive e-mail or host mailboxes. Websites and alerts send mail through an SMTP relay at a mail provider. Use a mail provider for your mailboxes.
- **No DNS or FTP server.** Nefo manages the DNS records of your domains at Cloudflare, and each website can have an SFTP account instead of FTP.
- **One server per panel.** Each server runs its own Nefo. Managing several servers from one panel is not planned (see the [roadmap](../roadmap#not-planned-and-why)).
- **No accounts for customers.** There is one administrator account per server.
- **No mail, DNS or FTP accounts when importing.** An import from cPanel brings the websites, databases, certificates and cron jobs. See [What is not imported](./import#what-is-not-imported).
- **Only Ubuntu and Debian.** Ubuntu 22.04, 24.04 and 26.04, and Debian 12 and 13. See [Requirements](./requirements).

## Status

Nefo is young software. Release numbers start with `0.`, and until version 1.0 only the latest release receives security fixes. You can see your version in **Settings** → **Panel**, or with `nefo version` on the server.

The installer has been tested on fresh Ubuntu 22.04, 24.04 and 26.04 and Debian 12 and 13 servers, on x86-64. The ARM64 version hasn't been tested on real ARM hardware yet. No outside security audit has been done yet.

Try Nefo on a server you can rebuild before you rely on it for important websites, and keep copies of your backups off the server. Report problems in the [GitHub repository](https://github.com/nefocp/nefo).

The [changelog](../changelog) lists what changed in each release, the [roadmap](../roadmap) what is being worked on and what is not planned, and the [security policy](../security-policy) how to report a vulnerability and check a release yourself.

## Licence

Nefo is free software under the GNU AGPLv3. You can use it for any purpose, including commercial use. If you offer a modified Nefo to other people over a network, the AGPL requires you to publish your changes.

## Privacy

Nefo has no telemetry, no usage statistics and no licence check. The web interface loads nothing from other sites: its fonts, icons and scripts are built into the program.

On its own, the panel connects to other servers only for these tasks:

| When | Where | Why |
|---|---|---|
| A certificate is due (checked every hour) | Let's Encrypt | Request or renew the certificates of your websites and of the panel. The panel's Let's Encrypt account has no e-mail address. Unless you use DNS validation, the panel first fetches a test file from your domain over HTTP, to check that the domain reaches this server. |
| Once a day, in release builds | The project's GitHub releases | Download the signed app store catalog. A catalog without a valid signature from the project is ignored. |

Everything else happens only because you asked for it:

- **Updates:** Nefo checks for a new version only when you click **Check for updates** or run `nefo update`. It never updates itself. See [Update](./update).
- **Things you set up:** alerts (Telegram, your SMTP server, Discord, Slack or your webhook), website monitoring (your websites, by their domain, like a visitor), remote backups (your S3 or SFTP storage, or Google Drive), DNS validation and the DNS page (the Cloudflare API), the mail relay (your mail provider), and country blocking, which downloads DB-IP's country database and Cloudflare's address list once a month while a website uses it.
- **Software you install:** packages come from your distribution through apt, and some components come from their own projects, such as Node.js from nodejs.org, extra PHP versions from the packages.sury.org repository, OpenLiteSpeed from LiteSpeed's repository, MariaDB and PostgreSQL versions from their projects' repositories when you choose them, and the ionCube Loader from ionCube. Some installed software connects out on its own: for example, ClamAV downloads its virus databases.

The security check sends nothing outside the server. Most requests from the panel identify themselves as `Nefo/<version>`. They don't include information about your server or your websites beyond what the task needs, such as the domain names in a certificate request.
