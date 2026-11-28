---
title: FAQ
description: Short answers to common questions about Nefo, the free control panel for Linux servers, about whether it is free, its licence, supported systems, memory, databases, Docker, moving websites, safety, privacy, updates, uninstalling and languages.
---

# FAQ

Short answers to the questions people ask most often about Nefo, the free, open-source control panel for one Linux server. Each answer starts with the short answer, then gives the details and links to the page that covers them. The facts are for Nefo v0.24.1.

## What is Nefo?

Nefo is a free, open-source control panel that you install on your own Linux server (a VPS) with one command, to manage websites, SSL certificates, databases, apps, files, backups and security from your browser. It is one program, `/usr/local/bin/nefo`, with the web interface built in, and it runs as the systemd service `nefo`. See [What is Nefo?](./).

## Is Nefo free?

Yes. Nefo is free software under the GNU AGPLv3, and every feature is free for everyone, businesses included. There is no paid edition, no locked feature, no licence key and no account to create. See [About](../about#licence).

## What does Nefo run on?

Nefo runs on Ubuntu 22.04, 24.04 and 26.04 and Debian 12 and 13, on 64-bit x86 (amd64) and 64-bit ARM (arm64), with root access and systemd. The installer has been tested on fresh servers with each of these systems on x86-64. The arm64 build hasn't been tested on real ARM hardware yet, so try it on a server you can rebuild first. On other systems the installer stops unless you force it. See [Requirements](./requirements).

## How much RAM does Nefo need?

The panel itself uses about 34 MB of memory, measured on a test server after the update to v0.24.0, and Nefo is built for a VPS with 1 GB of RAM. What you install adds to that, and the **Software** page shows roughly how much: Nginx about 15 MB, each PHP version about 25 MB, MariaDB its buffer pool plus about 100 MB (about 228 MB on a 1 GB server). On a 1 GB server, add 1 to 2 GB of swap. See [Requirements](./requirements#memory-and-disk).

## Does Nefo support MySQL, PostgreSQL and Docker?

Yes. Nefo runs MariaDB (your distribution's, or 11.4, 11.8 or 12.3 LTS) or MySQL 8.4 LTS, one per server, and can switch between the two with your databases. PostgreSQL (your distribution's version, or 15 to 19 from the PostgreSQL project) runs next to it. Docker runs containers and Compose projects, and the App store's Docker apps. See [Databases](./databases), [PostgreSQL](./postgresql) and [Docker](./docker).

## Which web servers can I use?

Nginx, Apache or OpenLiteSpeed, one per server. You choose when you install the web server:

- Nginx is the lightest.
- Apache reads the `.htaccess` files that some PHP applications come with.
- OpenLiteSpeed reads the rewrite rules in `.htaccess` files and has LiteSpeed Cache built in.

PHP runs in PHP-FPM with all three, one pool per website. Websites can also be Node.js, Python or Java apps, or reverse proxies to another address. See [Websites](./websites).

## Can I switch to another web server later?

Yes, but only on a server without websites. The panel refuses to uninstall a web server while it still serves websites, because each website's settings are written for that web server. Delete the websites, uninstall the web server on the **Software** page, then install the other one. Back up your websites first. See [Software](./software#web-server).

## Can I move my websites from cPanel or aaPanel? {#can-i-move-my-websites-from-another-panel}

Yes. Nefo imports a cPanel full backup (websites, databases with their users and passwords, certificates and cron jobs), and a website archive with its SQL dumps from aaPanel, HestiaCP, CyberPanel or any other server. Mail, DNS zones and FTP accounts are not imported. See [Import websites](./import).

## Is Nefo safe?

Nefo is built to be safe by default, but it is beta software that runs as root and has not had an outside security audit yet. The panel answers only under a secret path on a random port, uses HTTPS from the first start, supports two-factor sign-in, locks an address out after 5 failed sign-ins in 15 minutes, and logs every change. Releases are signed with Ed25519, and the built-in [security check](./security-check) lists the settings that leave the server open. Try it on a server you can rebuild first, and keep backups off the server. See [Security](./security) and the [security policy](../security-policy).

## Does Nefo send data anywhere?

No telemetry, no usage statistics and no licence check. On its own, the panel only contacts Let's Encrypt to issue and renew certificates, and, in release builds, downloads the signed app store catalog once a day. Everything else, such as update checks, alerts, remote backups and the country database for country blocking, happens only when you ask for it or set it up. See [Privacy](./#privacy).

## Can Nefo back up to Google Drive?

Yes, from v0.24.0, with an OAuth client of your own that can see only the files the panel creates. Nefo also backs up to S3-compatible storage (Amazon S3, Cloudflare R2, Backblaze B2, Wasabi, MinIO) and SFTP servers, can encrypt every remote copy with a passphrase, and a new server can find the backups on the storage and restore them. See [Backups](./backups#google-drive) and [Restore on a new server](./restore-new-server).

## How do I update Nefo?

Open **Settings** → **Panel** and click **Check for updates**, or run `nefo update` as root. The update checks the release's signature and checksum before it replaces the program, then restarts the panel; websites keep running. Nefo never updates itself. See [Update](./update).

## How do I uninstall Nefo?

Nefo has no uninstall command: you remove it by hand. Unlock any locked websites with `nefo unlock-files`, stop and remove the `nefo` service and `/usr/local/bin/nefo`, close the panel's port and delete `/etc/nefo` and `/var/lib/nefo`. Your websites and software keep running unless you remove them first. See [Uninstall](./uninstall).

## Can Nefo host my e-mail?

No. Nefo has no mail server: it doesn't receive e-mail or host mailboxes. Websites and alerts can send e-mail through an SMTP relay at a provider such as Gmail, SendGrid, Mailgun or Amazon SES, and the DNS page can add the MX, SPF and DMARC records of your mail provider. Use a mail provider for your mailboxes. See [Mail and alerts](./mail-alerts).

## Can I manage several servers from one panel?

No. Install Nefo on each server and manage each one separately. Managing several servers from one panel is not planned: a panel that controls other servers needs agents and trust between machines. See the [roadmap](../roadmap#not-planned-and-why).

## Can I give my clients their own login?

No. Each server has one panel account, `admin`. Nefo has no separate accounts for customers or resellers. To let a client upload files to their website, give that website an SFTP account, which is locked in the website's folder. See [Websites](./websites#sftp-access).

## Which languages does the panel speak?

Seven, from v0.24.0: English, Vietnamese, Chinese (simplified), Spanish, Portuguese, Russian and Indonesian, App store texts included. The panel follows your browser's language, and you can change it on the sign-in page or in the account menu. Each language is a separate file that the browser downloads only when it is used. These docs are in the same seven languages.

Translations are welcome. Copy `web/src/locales/en.json` to a file named after your language code, translate the values, and register the language in `web/src/i18n.js`. See [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md).

## How do I report a security problem?

Report it privately, not in a public issue. In the [GitHub repository](https://github.com/nefocp/nefo), open the "Security" tab and choose "Report a vulnerability". Include your version (`nefo version`), your operating system, and the steps to reproduce the problem. See the [security policy](../security-policy), which follows [SECURITY.md](https://github.com/nefocp/nefo/blob/main/SECURITY.md).

## Is there a changelog and a roadmap?

Yes. The [changelog](../changelog) lists what changed in each release, newest first, and what to know before you update. The [roadmap](../roadmap) lists what is being worked on before version 1.0, what is being considered, and what is not planned and why. To check a release's signature yourself before you install it, see [Check a release yourself](../security-policy#check-a-release-yourself).

## How can I help the project? {#how-can-i-support-the-project}

- Report bugs with steps to reproduce, your operating system and `nefo version`.
- Translate the panel into your language, or improve a translation.
- Fix issues and send pull requests (see [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md)).
- Star the project on [GitHub](https://github.com/nefocp/nefo) and tell others about it.
