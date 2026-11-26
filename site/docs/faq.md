---
title: FAQ
description: Short answers to common questions about Nefo, its licence, privacy, supported systems and features.
---

# FAQ

Short answers to the questions people ask most often. Each answer links to the page with the details.

## Is Nefo really free?

Yes. Every feature is free for everyone, including businesses, and there is no paid tier. Nefo is free software under the GNU AGPLv3.

A commercial licence exists for companies that want to rebrand Nefo (white-label), keep their own changes to it private, or embed it in a proprietary product. It doesn't unlock any feature. See [COMMERCIAL.md](https://github.com/nefocp/nefo/blob/main/COMMERCIAL.md).

## Does Nefo send data anywhere?

No telemetry, no usage statistics and no licence check. On its own, the panel only contacts Let's Encrypt to issue and renew certificates, and, in release builds, downloads the signed app store catalog once a day. Everything else, such as update checks, alerts and remote backups, happens only when you ask for it or set it up. See [Privacy](./#privacy).

## Which operating systems does it support?

Ubuntu 22.04, 24.04 and 26.04, and Debian 12 and 13. The installer refuses other systems. See [Requirements](./requirements).

## Does it run on ARM servers?

Yes. Nefo runs on 64-bit ARM (arm64) as well as 64-bit x86 (amd64). The installer picks the right program for your CPU.

## Which web servers can I use?

Nginx, Apache or OpenLiteSpeed, one per server. You choose when you install the web server:

- Nginx is the lightest.
- Apache reads the `.htaccess` files that some PHP applications come with.
- OpenLiteSpeed reads the rewrite rules in `.htaccess` files and has LiteSpeed Cache built in.

PHP runs in PHP-FPM with all three. See [Websites](./websites).

## Can I switch to another web server later?

Yes, but only on a server without websites. The panel refuses to uninstall a web server while it still serves websites, because each website's settings are written for that web server. Delete the websites, uninstall the web server on the **Software** page, then install the other one. Back up your websites first. See [Software](./software).

## Can Nefo host my e-mail?

No. Nefo has no mail server: it doesn't receive e-mail or host mailboxes. Websites and alerts can send e-mail through an SMTP relay at a provider such as Gmail, SendGrid, Mailgun or Amazon SES. Use a mail provider for your mailboxes. See [Mail and alerts](./mail-alerts).

## Can I manage several servers from one panel?

Not yet. Install Nefo on each server and manage each one separately. Managing several servers from one panel is on the roadmap.

## Can I move my websites from another panel?

There is no automatic import from another panel. Move each website by hand: create the website in Nefo, upload its files (with the file manager, from a URL or over SFTP), create its database and import the SQL dump. See [Websites](./websites), [Files](./files) and [Databases](./databases).

## Can I give my clients their own login?

No. Each server has one panel account, `admin`. Nefo has no separate accounts for customers or resellers. To let a client upload files to their website, give that website an SFTP account, which is locked in the website's folder. See [Websites](./websites).

## Which languages does the panel speak?

English and Vietnamese. The panel follows your browser's language, and you can change it on the sign-in page or in the account menu.

Translations are welcome. Copy `web/src/locales/en.json` to a file named after your language code, translate the values, and register the language in `web/src/i18n.js`. See [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md).

## How do I report a security problem?

Report it privately, not in a public issue. In the [GitHub repository](https://github.com/nefocp/nefo), open the "Security" tab and choose "Report a vulnerability". Include your version (`nefo version`), your operating system, and the steps to reproduce the problem. See [SECURITY.md](https://github.com/nefocp/nefo/blob/main/SECURITY.md).

## How can I support the project?

- Report bugs with steps to reproduce, your operating system and `nefo version`.
- Translate the panel into your language, or improve a translation.
- Fix issues and send pull requests (see [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md)).
- Star the project on [GitHub](https://github.com/nefocp/nefo) and tell others about it.
- Buy a commercial licence for your company (see [COMMERCIAL.md](https://github.com/nefocp/nefo/blob/main/COMMERCIAL.md)).
