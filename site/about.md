---
title: About Nefo
description: What Nefo is, who it is for, the principles it is built on, its status and licence, and how to get help or report a bug.
---

# About Nefo

Nefo is a free, open-source control panel for one Linux server. You install it on your own VPS with one command, then manage websites, certificates, databases, apps, files, backups and security from your browser, in one of seven languages. It is beta, licensed under the GNU AGPLv3, and built for a VPS with 1 GB of RAM.

It is one program, `/usr/local/bin/nefo`, with the web interface built in, and it runs as the systemd service `nefo`. It runs on Ubuntu 22.04, 24.04 and 26.04 and Debian 12 and 13, on x86-64 and ARM64. Its App store has 42 apps, and the interface speaks English, Vietnamese, Chinese, Spanish, Portuguese, Russian and Indonesian. See [What is Nefo?](./docs/) for everything it does.

## Who it is for

Nefo is for people who run their own VPS: someone setting up a first server for a blog, a developer deploying an app, or a freelancer or agency looking after client websites.

Each server has one panel account, `admin`. Nefo is not made for selling hosting: it has no reseller or customer accounts. See [Nefo and other panels](./compare) for when another panel suits you better.

## Principles

- **Light.** Nefo is built for a VPS with 1 GB of RAM, and the panel itself uses about {{ $release.ram }} of memory (measured on a test server running v{{ $release.version }}). It installs nothing until you choose, every component can be uninstalled again, and features that would keep large services running all the time, such as a mail server, are left out.
- **Every feature is free.** There is no paid edition and no locked feature. You don't create an account, and there is no licence key.
- **Nothing phones home.** No telemetry, no usage statistics, no licence check. The interface loads nothing from other sites. On its own, the panel contacts only Let's Encrypt for certificates and the project's GitHub releases for the signed app store catalog, and it checks for updates only when you ask. See [Privacy](./docs/#privacy).
- **Honest about its limits.** The docs say what Nefo doesn't do, and what it refuses to do and why. The [roadmap](./roadmap) lists what is not planned as plainly as what is.

## Status

Nefo is beta. The first public release, v0.20.0, came out on 26 November 2026, and the current release is v{{ $release.version }} ({{ $release.date }}). See the [changelog](./changelog).

- Release numbers start with `0.`. Until version 1.0, only the latest release receives security fixes.
- No outside security audit has been done yet. See the [security policy](./security-policy).
- The installer has been tested on fresh Ubuntu 22.04, 24.04 and 26.04 and Debian 12 and 13 servers on x86-64. ARM64 hasn't been tested on real ARM hardware yet.

Try Nefo on a server you can rebuild before you rely on it for important websites, and keep copies of your backups off the server.

## Licence

Nefo is free software under the [GNU AGPLv3](https://github.com/nefocp/nefo/blob/main/LICENSE). You can use it for any purpose, on as many servers as you like, and read, change and share its code. If you offer a modified Nefo to other people over a network, the AGPL requires you to publish your changes.

The source code is on [GitHub](https://github.com/nefocp/nefo).

## Get help and report bugs

- **Read the docs first.** [Troubleshooting](./docs/troubleshooting) covers the common problems, and the [FAQ](./docs/faq) the common questions.
- **Report a bug** in [GitHub Issues](https://github.com/nefocp/nefo/issues). Include your version (`nefo version`), your operating system, the steps that lead to the problem, what you expected and what happened. If the panel itself fails, add the end of its log: `journalctl -u nefo -n 50 --no-pager`.
- **Leave secrets out** of anything you post: the panel's secret path, passwords, API keys and private domain names.
- **Report a security problem privately,** never in a public issue. See the [security policy](./security-policy#report-a-vulnerability).

## Contribute

- **Report bugs** as described above. Reports from real servers are what Nefo needs most before 1.0.
- **Translate the panel.** Copy `web/src/locales/en.json` to a file named after your language code, translate the values (not the keys), and register the language in `web/src/i18n.js`.
- **Fix issues and send pull requests.** Read [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md) first. Nefo runs as root on its users' servers, so every change is reviewed with security in mind, and the security rules for code there are not optional. For a larger change, open an issue first to agree on the approach.
- **Sign the CLA.** Contributors sign the [Contributor Licence Agreement](https://github.com/nefocp/nefo/blob/main/CLA.md) once, when they open their first pull request. You keep the copyright in your contributions.
