---
title: What is Nefo?
description: Nefo is a free, open-source control panel that you install on your own Linux server to run websites, databases, apps and backups.
---

# What is Nefo?

Nefo is a control panel for your own Linux server. This page explains what it does, who it is for, what it deliberately leaves out, and what it sends over the network.

## A control panel in one program

You install Nefo on a VPS with one command. It is a single program, `/usr/local/bin/nefo`, with the web interface built in, and it runs as the systemd service `nefo`. You then manage the server from your browser: websites, certificates, databases, files, backups, software and security.

- **Every feature is free.** There is no paid tier and no per-site or per-account fee.
- **No account and no licence server.** Nothing has to be registered or activated.
- **Built for small servers.** Nefo is designed to run on a VPS with 1 GB of RAM, on x86-64 and ARM64.
- **English and Vietnamese** interface. You pick the language on the sign-in page or in the account menu.

To get started, check the [requirements](./requirements), then [install Nefo](./install) and follow the [first steps](./first-steps).

## Who it is for

Nefo is for people who run their own VPS: someone setting up a first server for a blog, a developer deploying an app, or a freelancer or agency looking after client websites.

Each server has one panel account, `admin`. Nefo has no separate logins for customers or resellers.

## What you can do with it

### Websites

- Host PHP, static and app websites (Node.js or Python) on Nginx, Apache or OpenLiteSpeed. Each website gets its own Linux user, and optionally an SFTP account locked in its folder. See [Websites](./websites).
- Turn on HTTPS with free Let's Encrypt certificates that renew automatically, wildcard certificates through Cloudflare DNS, or your own certificate. See [SSL certificates](./ssl).
- Deploy from GitHub, GitLab, Gitea or any Git server, on demand or on every push. See [Deploy from Git](./git-deploy).
- Keep WordPress sites up to date, with a database backup before each update, maintenance mode and a staging copy. See [WordPress toolkit](./wordpress).
- Install applications in one click: PHP applications such as WordPress, Joomla, Nextcloud and Laravel, and Docker applications such as Ghost, Uptime Kuma, n8n and Gitea. See [App store](./apps).

### Data

- Run MariaDB tuned for small servers, with databases, users, SQL import and export, remote access per database, and Adminer behind the panel's sign-in. Browse Redis keys. See [Databases](./databases).
- Manage files with an editor, upload and download, archives and a trash that keeps deleted files for 30 days. See [Files](./files).
- Back up websites, databases and folders on a schedule, to this server, to S3-compatible storage or to an SFTP server, and restore them in one click. See [Backups](./backups).

### Server

- Install and uninstall each component (web server, PHP versions, MariaDB, Redis, Node.js, Python, Docker and more). Nefo refuses to remove one that websites, databases or containers still use. See [Software](./software).
- Run Docker containers and Compose projects. Nefo checks Compose files for unsafe settings, such as privileged containers or ports open to the internet. See [Docker](./docker).
- Control the firewall (ufw), fail2ban, SSH settings, a web application firewall per website and a malware scanner. See [Security](./security).
- Use a terminal in the browser, and manage services, processes, background programs and cron jobs. See [System tools](./system).
- Let websites send e-mail through an SMTP relay, and get alerts by Telegram or e-mail. See [Mail and alerts](./mail-alerts).

## Security of the panel itself

Nefo runs as root, so it protects its own sign-in:

- The panel answers only under a secret path such as `/k3x9p2qa7m1z/`. Every other address on its port returns `404`.
- It uses HTTPS from the first start. The certificate is self-signed at first, and you can replace it with a free certificate for the panel's own domain.
- Sign-in supports two-factor authentication. After 5 failed attempts in 15 minutes, an address is locked out for 15 minutes.
- Sign-ins and changes are written to an audit log, which keeps entries for 90 days.

## What Nefo doesn't do

- **No mail server.** Nefo doesn't receive e-mail or host mailboxes. Websites and alerts send mail through an SMTP relay at a mail provider. Use a mail provider for your mailboxes.
- **One server per panel.** Each server runs its own Nefo. Managing several servers from one panel isn't available yet.
- **No Google Drive backups yet.** Backups go to this server, S3-compatible storage or SFTP.
- **No accounts for customers.** There is one administrator account per server.
- **No import from other panels.** Move websites by hand: create the website, upload its files and import its database.
- **Only Ubuntu and Debian.** See [Requirements](./requirements).

## Status

Nefo is young software. Release numbers start with `0.`, and until version 1.0 only the latest release receives security fixes. You can see your version in **Settings → Panel**, or with `nefo version` on the server.

Try Nefo on a server you can rebuild before you rely on it for important websites, and keep copies of your backups off the server. Report problems in the [GitHub repository](https://github.com/nefocp/nefo).

## Licence

Nefo is free software under the GNU AGPLv3. You can use it for any purpose, including commercial use. If you offer a modified Nefo to other people over a network, the AGPL requires you to publish your changes.

Companies that want to rebrand Nefo (white-label) or keep their changes private can buy a commercial licence: see [COMMERCIAL.md](https://github.com/nefocp/nefo/blob/main/COMMERCIAL.md).

## Privacy

Nefo has no telemetry, no usage statistics and no licence check. The web interface loads nothing from other sites: its fonts, icons and scripts are built into the program.

On its own, the panel connects to other servers only for these tasks:

| When | Where | Why |
|---|---|---|
| A certificate is due (checked every hour) | Let's Encrypt | Request or renew the certificates of your websites and of the panel. The panel's Let's Encrypt account has no e-mail address. Unless you use DNS validation, the panel first fetches a test file from your domain over HTTP, to check that the domain reaches this server. |
| Once a day, in release builds | The project's GitHub releases | Download the signed app store catalog. A catalog without a valid signature from the project is ignored. |

Everything else happens only because you asked for it:

- **Updates:** Nefo checks for a new version only when you click **Check for updates** or run `nefo update`. It never updates itself. See [Update](./update).
- **Things you set up:** alerts (Telegram or your SMTP server), remote backups (your S3 or SFTP storage), DNS validation (the Cloudflare API) and the mail relay (your mail provider).
- **Software you install:** packages come from your distribution through apt, and some components come from their own projects, such as Node.js from nodejs.org, extra PHP versions from the ondrej/php repository, and OpenLiteSpeed from LiteSpeed's repository. Some installed software connects out on its own: for example, ClamAV downloads its virus databases.

Most requests from the panel identify themselves as `Nefo/<version>`. They don't include information about your server or your websites beyond what the task needs, such as the domain names in a certificate request.
