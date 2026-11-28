---
title: Nefo and other panels
description: How Nefo differs from the control panels most VPS owners use, and when another panel is the better choice.
---

# Nefo and other panels

Nefo differs from most hosting control panels in a few ways: every feature is free, with no licence key or vendor account; the panel itself uses about 34 MB of memory; its sign-in sits on a random port behind a secret path; and it leaves out mail, DNS and FTP servers and reseller accounts on purpose.

This page compares Nefo with the hosting control panels people most often install on a VPS. It doesn't name them. They differ from one another, so the right-hand column says how many of them do something: most, many, some or few.

The comparison was checked against the panels' own websites in November 2026. Panels change their prices and features, so check the one you are considering before you decide.

## At a glance

| | Nefo | Other panels |
|---|---|---|
| Price | Free. Every feature, no paid edition | Most have something to pay for: a licence per server, usually priced by the number of accounts or sites, or a paid edition or add-ons |
| Licence key or vendor account | None | Some need one before they run, and many need one for their paid features |
| Data sent to the vendor | None: no telemetry, no install report, no licence check | Varies. Some collect usage statistics by default, and some installers report the server to the vendor |
| Memory | The panel itself uses about 34 MB (measured on a test server running v0.24.0). Built for a VPS with 1 GB of RAM | Most ask for at least 1 GB, and many recommend 4 GB |
| Sign-in page | A random port and a secret path. Every other address returns `404` | Most use a fixed, well-known port. A few use a random port or a secret path |
| Updates of the panel | Signed with Ed25519 and checked before they install. Only when you ask | Some document signed packages; others don't say |
| Lock-out guards | Refuses firewall rules and SSH settings that would lock you out | Few have one for the firewall, such as a timed rollback. Guards for SSH settings are rare |
| Source code | All of it, under the AGPLv3 | Most are closed or only source-available. Some are open source |
| Mail and DNS servers | Not included, by design | Most install both |
| Several servers from one panel | No. One panel per server | Few manage several servers from one panel, though many can share DNS zones between servers |
| Reseller and customer accounts | No. One administrator account | Most have reseller accounts, and nearly all have logins for customers or users |
| Operating systems | Ubuntu and Debian, on x86-64 and ARM64 | Most are Linux only, and most run on more Linux distributions than these two. Few run on Windows |
| Age | First public release in November 2026. Beta | Many have been around for 20 years or more |

## What Nefo does differently

### It stays small

- Nefo is one program, `/usr/local/bin/nefo`, about 25 MB, with the web interface built in. On a test server running v0.24.0 it uses about 34 MB of memory. The interface's files are sent gzip-compressed, and each of its seven languages is downloaded only when it is used.
- It installs nothing else until you choose. At the first sign-in, a setup page asks which web server, PHP versions, database server and extras you want, and shows roughly how much memory and disk your choice adds. `nefo setup` does the same on the command line.
- Every component can be uninstalled again, and the menu hides what isn't installed. Nefo refuses to remove a component that websites, databases or containers still use.
- No mail, DNS or FTP server runs in the background, because Nefo doesn't include one.

### Every feature is free, and nothing phones home

- There is no paid edition, no account to create and no licence key.
- There is no telemetry and no usage statistics. On its own, the panel contacts only Let's Encrypt, to issue and renew certificates, and the project's GitHub releases once a day, for the signed app store catalog. See [Privacy](./docs/#privacy).
- It never updates itself, and it checks for a new version only when you ask.

### The sign-in is hard to find

- The installer picks a random port from 20000 to 49999 and a secret path of 12 letters and digits. Every other address on that port returns `404`.
- The panel uses HTTPS from the first start. Sign-in supports two-factor authentication, and 5 failed attempts in 15 minutes lock an address out for 15 minutes. Sign-ins and changes go to an activity log.
- API keys for scripts can be read-only, limited to the addresses you list, and set to expire. Even a full key can't open the terminal, the database tools or the account settings.

See [Security](./docs/security) and [API keys](./docs/api).

### It refuses changes that would lock you out or break things

- **Firewall:** the last rule that lets your own address reach SSH or the panel can't be deleted.
- **SSH:** settings with which nobody could sign in are refused, such as turning off password sign-in while no account has a key.
- **Software:** a web server, PHP version or database server that is still in use can't be uninstalled.
- **Docker:** Compose projects are checked before they are saved or started. Privileged containers, ports open to the internet and mounts of system folders are refused.

### It checks its own server

The [security check](./docs/security-check) runs 22 quick checks of the server on demand or once a day, such as SSH password sign-in, the firewall, waiting security updates, databases open to the internet, Docker ports that bypass ufw and files a website serves by mistake, and links each finding to the page that fixes it. It sends nothing outside the server.

### Backups survive the server

Backups go to the server itself and to S3-compatible storage, an SFTP server or Google Drive, encrypted with a passphrase if you like. A new server finds them on the storage and restores the websites, with their databases. See [Restore on a new server](./docs/restore-new-server).

### Releases are signed

The list of checksums in each release is signed with the project's Ed25519 key. The installer and `nefo update` check the signature and the checksum before they install anything. The [security policy](./security-policy#check-a-release-yourself) shows how to check a release yourself.

### You can move websites in

Nefo imports websites from the backups of other panels: files, databases with their users and passwords, certificates and cron jobs. See [Import websites](./docs/import) for the formats it reads.

## When another panel fits you better

Nefo leaves some things out on purpose (see the [roadmap](./roadmap#not-planned-and-why) for why). Choose another panel if you need one of these:

- **Mail on the same server.** Nefo has no mail server: no mailboxes, no webmail, no spam filter. Websites and alerts send mail through your provider's SMTP relay. Most other panels include a mail server.
- **Your own DNS server.** Nefo doesn't run name servers. It manages the DNS records of your domains at Cloudflare.
- **FTP.** Nefo has no FTP server. Each website can have an SFTP account, locked in the website's folder.
- **Many servers from one place.** Each server runs its own Nefo, and you sign in to each one separately.
- **Selling hosting.** Nefo has one administrator account per server. It has no resellers, no customer logins, no hosting plans and no quotas.
- **A long track record, or an outside security audit.** Nefo's first public release was in November 2026. It is beta, and no outside security audit has been done yet.
- **Windows, or a Linux other than Ubuntu and Debian.** Nefo runs on Ubuntu 22.04, 24.04 and 26.04 and Debian 12 and 13 only.

If none of these matters to you, [install Nefo](./docs/install) on a server you can rebuild, and keep a copy of your backups off the server.
