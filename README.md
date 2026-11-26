# Nefo

**A free, open-source control panel for your Linux server. Every feature is free, and the panel doesn't phone home.**

> ⚠️ **Status: beta.** Nefo is feature-complete for a single server and in daily use on test servers, but it has not had an outside security audit yet. Keep backups, and try it on a server you can rebuild before you trust it with important sites.

Nefo is a single, lightweight binary that you install on your own VPS. It gives you a modern web UI to manage websites, databases, files and security, without per-account licence fees and without sending your data anywhere.

- **100% free and open source** (AGPLv3). No paid tier, no locked features.
- **Secure by default:** a hidden login URL, HTTPS from the first start, two-factor authentication, rate-limited logins and an audit log.
- **Lightweight:** one static Go binary with the UI embedded. Built for small VPSes with 1 GB of RAM, on x86-64 and ARM64.
- **Private:** no telemetry, no licence server, no account required.
- **Multilingual** from day one. English and Vietnamese are included, and more languages come from the community.

## Install

On a fresh **Ubuntu 22.04 / 24.04 / 26.04** or **Debian 12 / 13** server, as root:

```sh
curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | sudo bash
```

The installer checks the release's Ed25519 signature and the download's checksum, and installs `/usr/local/bin/nefo` and a systemd service. It then picks a random port and a secret URL path, and prints the address and a one-time admin password:

```
  URL:      https://203.0.113.10:28443/k3x9p2qa7m1z/
  Username: admin
  Password: ••••••••••••••••••••
```

Every other URL on that port returns `404`, so scanners can't even find the login page. The certificate is self-signed at first, so your browser will warn you once.

Useful commands on the server:

| Command | What it does |
|---|---|
| `nefo info` | Show the panel URL |
| `nefo reset-password` | Set a new random admin password and sign out all sessions |
| `nefo reset-password --disable-2fa` | The same, and also turn off 2FA (lost phone) |
| `systemctl restart nefo` | Restart the panel |
| `journalctl -u nefo` | Show the logs |

## Features and roadmap

- [x] Single binary with embedded UI, systemd service, one-line installer, self-update (`nefo update`); releases are checksummed and signed with Ed25519 (see [RELEASING.md](RELEASING.md))
- [x] Hidden entrance path, HTTPS from the first start, a free Let's Encrypt certificate for the panel itself, secure cookies, strict CSP
- [x] Sign-in with TOTP two-factor authentication, rate limiting and an audit log
- [x] Dashboard with live load and 7 days of history
- [x] **Websites:** Nginx, Apache (for `.htaccess`) or OpenLiteSpeed with PHP-FPM (LSPHP on OpenLiteSpeed) in one click, one web server per server, chosen at install; suspend, redirect or password-protect a site; PHP, static and app sites (Node.js with a version per site, Python with its own virtualenv, dependencies installed as the site's user); one Linux user per site, with optional **SFTP** locked in the site's folder (password or SSH keys); per-site PHP version, limits, pool size and allowed functions; free Let's Encrypt certificates with automatic renewal, wildcard certificates through Cloudflare DNS, or your own certificate; rewrite presets (WordPress, Laravel, Drupal, Joomla, single-page apps, static sites with clean URLs…) and your own directives, checked by the web server before they apply; custom 404 page, hotlink protection, an expiry date; traffic statistics from the access log; logs
- [x] **Deploy from Git:** GitHub, GitLab, Gitea or any Git server over HTTPS or SSH (a deploy key per site), on demand or on every push through a signed webhook, with a deploy script run as the site's user
- [x] **WordPress toolkit:** versions and pending updates of every WordPress site, one-click updates with a database backup first, maintenance mode, debug log, auto-updates, admin password reset, and a staging copy with its own database
- [x] **App store:** WordPress, Joomla, Nextcloud, Drupal and Laravel in one click, with their own database; Ghost, Uptime Kuma, n8n and Gitea in Docker, each behind its own website
- [x] **Databases:** MariaDB tuned for small servers, databases and users, SQL import and export, remote access per database (allow-listed addresses, firewall rules managed), a slow query log viewer, and Adminer reachable only through the panel's sign-in; a **Redis** key browser
- [x] **Web terminal:** a root shell in the browser
- [x] **Files:** file manager with editor, upload, download, download from a URL, search by name or content, compress and extract, and a trash with 30-day restore; inside a website's folder it works as the site's user and can't leave the folder, elsewhere it works as root
- [x] **Backups** of sites, databases and any folder, scheduled with retention, to this server, S3-compatible storage (S3, R2, B2, Wasabi, MinIO) and SFTP servers (host key pinned), with one-click restore
- [x] Cron jobs, services, **background programs** (kept running by systemd as a site's user, restarted after a crash), processes and listening ports, SSH sign-in history, firewall (ufw), fail2ban (SSH and panel), SSH settings with lock-out guards (the panel refuses settings with which nobody could sign in, and never deletes your last firewall rule for SSH or the panel)
- [x] **Website security:** a web application firewall (ModSecurity with the OWASP Core Rule Set, per site: off, detect or block, with its events), per-IP rate limits and blocked addresses, and a malware scanner (web shells, backdoors, hidden PHP) with quarantine, plus optional ClamAV
- [x] Software: **install and uninstall** every component (web server, PHP, MariaDB, Adminer, Redis, Node.js, Python, Docker, WAF, ClamAV), refused while websites, databases or containers still use it, data kept unless you confirm; the menu hides what is not installed; several PHP versions with their extensions and php.ini settings, Redis, several Node.js versions, Python, Docker: containers, **Compose projects** (checked: localhost-only ports, no privileged containers or system mounts), images, and a terminal in each container
- [x] Outgoing mail through an SMTP relay (Gmail, SendGrid, Mailgun, Amazon SES…), so websites can send e-mail
- [x] Alerts by Telegram and e-mail, with their own SMTP settings (load, disk, stopped services, failed backups and renewals)
- [x] English and Vietnamese UI
- [ ] Google Drive backups, multi-server management

## Development

You need **Go 1.26+** and **Node.js 20.19+**.

```sh
# 1. Create a local dev config (HTTP on 127.0.0.1:8080, entrance "dev") and an admin account
go run ./cmd/nefo init --dev --config .dev/config.json --data .dev

# 2. Start the backend
go run ./cmd/nefo serve --config .dev/config.json

# 3. In a second terminal, start the UI with hot reload
cd web
npm install
npm run dev            # open http://localhost:5173
```

The Vite dev server forwards `/api` to the backend. Live system statistics and website management only work on Linux (Debian or Ubuntu). On Windows and macOS the UI shows a notice instead.

When you test certificates on a server, add `"acme_directory": "https://acme-staging-v02.api.letsencrypt.org/directory"` to `config.json` and restart the panel. Staging certificates are not trusted by browsers, but failed attempts do not count against Let's Encrypt's rate limits.

To build a release binary with the UI embedded:

```sh
make web build         # → bin/nefo
make release           # → dist/nefo-linux-{amd64,arm64}, install.sh, checksums.txt
go test ./...          # run the tests
```

### Project layout

```
cmd/nefo/             CLI: serve, init, info, reset-password, update
internal/api/         HTTP API, entrance, CSRF protection, security headers
internal/auth/        bcrypt passwords, TOTP, session tokens, login rate limiting
internal/backup/      site, database and folder backups; S3 and SFTP storage
internal/config/      the panel's config file
internal/cron/        cron jobs
internal/daemons/     background programs as systemd units
internal/database/    MariaDB: databases, users, remote access, slow query log
internal/docker/      containers, Compose projects, images
internal/files/       file manager, download from a URL, search, trash
internal/gitdeploy/   deploy from Git, push webhooks
internal/hosting/     Nginx, Apache or OpenLiteSpeed, PHP, websites, SFTP, WAF and certificates
internal/jobs/        background tasks with a live log
internal/logstats/    traffic statistics from access logs
internal/mailrelay/   outgoing mail through Postfix and an SMTP relay
internal/malware/     malware scanner and quarantine
internal/monitor/     metrics history and alerts
internal/procs/       processes, listening ports, SSH sign-ins
internal/redisui/     minimal Redis client for the key browser
internal/run/         runs system programs, never through a shell
internal/security/    ufw, fail2ban, SSH settings
internal/services/    systemd services
internal/software/    PHP versions and extensions, Node.js, Python, Redis, Docker
internal/store/       SQLite database (pure Go, no CGO) with migrations
internal/system/      CPU, memory, disk and load from /proc
internal/terminal/    PTY for the web terminal
internal/update/      signed self-update
internal/validate/    allow-list validation of everything that reaches the system
internal/wptoolkit/   WordPress toolkit (wp-cli)
internal/web/         the built UI, embedded with go:embed
web/                  Vue 3 + Vite UI, translations in web/src/locales/
scripts/install.sh    installer
```

## Security

The panel has root access to your server, so security is the top priority. See [SECURITY.md](SECURITY.md) to report a vulnerability privately. Please do not open a public issue for one.

## License

Nefo is free software under the [GNU AGPLv3](LICENSE). You can use it for any purpose, including commercial use, for free.

If you offer a **modified** Nefo to other people over a network, the AGPL requires you to publish your changes. Hosting companies and agencies that want to rebrand Nefo (white-label), keep their changes private, or get an invoice and a support agreement can buy a [commercial licence](COMMERCIAL.md).

## Contributing

Contributions are welcome, and translations especially. Please read [CONTRIBUTING.md](CONTRIBUTING.md) first. Contributors are asked to sign the [CLA](CLA.md).
