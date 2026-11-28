---
title: Roadmap
description: What is being worked on before Nefo 1.0, what is being considered, and what is not planned and why.
---

# Roadmap

Nefo's roadmap has no dates: an item ships when it works. Before version 1.0, the work goes into making what exists dependable: testing on real ARM64 hardware and against the real services, a test server that runs for a long time, reports from outside users, and an outside security review. Some ideas are being considered, and a few things are not planned, on purpose.

This page lists what is being worked on, what is being considered, and what Nefo will not do. To see what has shipped, read the [changelog](./changelog).

## Now: toward 1.0 {#now-toward-1-0}

Nefo is beta. Before version 1.0, the work goes into making what exists dependable, more than into new features:

- **Test the installer on ARM64.** The installer, with and without the software setup, has been tested on fresh Ubuntu 22.04, 24.04 and 26.04 and Debian 12 and 13 servers on x86-64. ARM64 builds are released but haven't been tested on real ARM hardware yet.
- **Test against the real services.** Backups to S3-compatible storage (such as AWS S3 and Cloudflare R2) and Google Drive, alerts by Telegram, Discord, Slack and webhook, mail through SMTP providers, and the Cloudflare API. Some of these have so far been tested only against stand-ins: Google Drive, for example, only against a simulated Google service.
- **Run a test server for a long time,** with real websites, so that problems that take weeks to show up (growing logs, disk use, memory) are found before you find them.
- **Outside users and their bug reports.** Every report with steps to reproduce helps. See [Report a bug](./about#get-help-and-report-bugs).
- **An outside security review.** No outside audit has been done yet. Until one has, treat Nefo as beta software that runs as root, and see the [security policy](./security-policy).

Until version 1.0, only the latest release receives security fixes.

## Being considered {#being-considered}

These are ideas, not promises. Some may change, and some may never ship.

- **Google Drive backups without an OAuth client of your own.** Today each server needs an OAuth client that you create once in a Google Cloud project (see [Google Drive](./docs/backups#google-drive)).
- **The visitor's own address behind Cloudflare's proxy,** in a website's logs and PHP on Nginx, as a setting per website. Today an Nginx website behind the proxy sees Cloudflare's addresses there; country blocking already looks up the visitor's address.
- **A calmer panel interface.**

The panel speaks seven languages. Translations into more languages are welcome: see [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md).

## Not planned, and why {#not-planned-and-why}

Nefo is meant to stay light enough for a VPS with 1 GB of RAM, and small enough that its code can be checked for security. Each item below would add services that run all the time, open ports to the internet, or a large amount of code that runs as root. So they are left out on purpose, and this page says what Nefo offers instead.

| Not planned | Why | What Nefo offers instead |
|---|---|---|
| A mail server | Mail needs several services running all the time, spam and virus filtering that use a lot of memory, and constant care for the server's reputation. Many providers also block outgoing port 25. | An SMTP relay through your mail provider, for websites and alerts. Use a mail provider for mailboxes. See [Mail and alerts](./docs/mail-alerts). |
| A DNS server | Name servers have to answer from the internet all the time, and are best run on more than one machine. | The DNS records of your Cloudflare zones, managed from the panel, and wildcard certificates through Cloudflare DNS. See [DNS (Cloudflare)](./docs/dns). |
| An FTP server | Another service with its own open ports and its own accounts. | An SFTP account per website, locked in the website's folder, with a password or SSH keys. See [SFTP access](./docs/websites#sftp-access). |
| Managing several servers from one panel | A panel that controls other servers needs agents and trust between machines. A break-in at one would reach all of them. | One Nefo per server. API keys let your own scripts work with each server. See [API keys](./docs/api). |
| Reseller and hosting-customer accounts | Accounts with their own limits, plans and quotas are a different product, for selling hosting. Nefo is for the person who runs the server. | One administrator account. To let a client upload files, give their website an SFTP account. |

If you need one of these, another panel may suit you better: see [Nefo and other panels](./compare#when-another-panel-fits-you-better).
