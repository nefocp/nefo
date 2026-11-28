---
title: Security check
description: Nefo's security check finds the settings that leave a server open (SSH, firewall, updates, open databases, Docker ports, exposed files, certificates, PHP) and links each finding to its fix, on demand or daily.
---

# Security check

The security check, new in v0.24.0, runs 22 quick checks of the server: SSH that accepts passwords, a firewall that is off, waiting security updates, a database or Redis open to the internet, Docker ports that bypass ufw, website files anyone can download, expiring certificates and PHP settings. Each finding has a severity and a button to the page that fixes it.

It runs when you start it, and once a day by default. It sends nothing outside your server. Open **Security check** in the **Security** group of the left menu.

## Run the check

1. Click **Run the check**. It takes a few seconds.
2. Read the findings on the **To fix** tab. Each has a severity:
   - **Critical**: fix it now. For example, root can sign in over SSH with a password, or a website serves a database dump.
   - **Warning**: fix it soon. For example, the firewall is off or security updates are waiting.
   - **Note**: worth knowing, often fine as it is. For example, SSH listens on port 22.
3. Click the button of a finding, such as **Go to Firewall & SSH**, to open the place that fixes it. Then run the check again.

The counters at the top show the **Critical** problems, **Warnings** and **Notes** still open, and how many checks **Passed**. Click **Passed** at the bottom of the page to see each check that found nothing to fix. Checks that don't apply to the server (no Redis, no websites, no Docker…) are listed under "Not applicable on this server".

## What does the security check look at? {#what-is-checked}

| Check | Severity | Where you fix it |
|---|---|---|
| root can sign in over SSH with a password | Critical | **Firewall & SSH** → SSH |
| SSH accepts passwords for other accounts | Warning | **Firewall & SSH** → SSH |
| SSH listens on port 22 | Note | **Firewall & SSH** → SSH |
| The firewall (ufw) is not installed, or off | Warning | **Firewall & SSH** |
| fail2ban is not installed, not running, or doesn't watch SSH (no `sshd` jail) | Warning | **Firewall & SSH** → fail2ban, or **Services** |
| The panel is reached without HTTPS | Critical | `/etc/nefo/config.json` (see below) |
| The panel uses its self-signed certificate | Note | **Settings** |
| A panel account has no two-factor sign-in | Critical | **Account** |
| Security updates are waiting | Warning | **System** |
| The update check is more than a week old, or never ran | Note | **System** |
| Automatic security updates are off | Warning | **System** |
| A restart is needed to finish updates | Note | **Settings** |
| An account other than root has user ID 0 | Critical | **Terminal** |
| An account has an empty password | Critical | **Terminal** (`passwd -l <account>`) |
| MariaDB, MySQL, PostgreSQL, Redis, Valkey, Memcached or MongoDB listens on an address other than 127.0.0.1 | Critical when the firewall lets everyone reach the port, otherwise Note | **Databases** |
| Redis has no password | Critical on a public address the firewall lets through, Warning on another public address, Note when it only listens on this server | **Databases** → Redis |
| A Docker container publishes ports on every address | Warning | **Docker** |
| A website has no HTTPS | Warning | **Websites** → (site) → **SSL / HTTPS** |
| A website has a certificate but doesn't redirect to HTTPS | Note | **Websites** → (site) → **SSL / HTTPS** |
| A certificate has expired | Critical | **Websites** → (site) → **SSL / HTTPS** |
| A certificate expires in 14 days or less | Warning | **Websites** → (site) → **SSL / HTTPS** |
| A website serves files that should be private | Critical for secrets, Warning for copies and archives | **Files** |
| A website has world-writable files or folders | Warning | **Files** |
| PHP shows errors to visitors (`display_errors`) | Warning | **Software** → PHP → **Manage** |
| PHP announces its version (`expose_php`) | Note | **Software** → PHP → **Manage** |
| WordPress core, plugin or theme updates are waiting | Warning | **Websites** → (site) → **WordPress** |
| WordPress debug mode (`WP_DEBUG`) is on | Warning | **Websites** → (site) → **WordPress** |

Some details:

- **Databases on the network**: the check reads which addresses each program listens on. For a public address it looks at the firewall: no firewall, or a rule that lets everyone reach the port, is critical. A port that the firewall only opens to some addresses (such as a database's **Remote access**), or doesn't open at all, is a note. An address of a private network (10.x, 172.16–31.x, 192.168.x) is a note.
- **Docker** writes its own firewall rules ahead of ufw's, so ufw doesn't block a port a container publishes. Publish the port on `127.0.0.1` (for example `127.0.0.1:8080:80`) when only a website on this server uses it.
- **Files a website serves**: the check lists the top of each website's `public` folder and asks the web server for the files that should never be public: `.env` files, `.git`, `.svn` and `.hg` folders, `.htpasswd`, database dumps (`.sql`, `.sql.gz`, `.dump`, `.sqlite`…), copies of `wp-config.php`, backup copies (`.bak`, `.old`, `.orig`, `.save`, `.swp`, names ending in `~`), `phpinfo.php` and `info.php`, and archives whose name looks like a copy of the site (with "backup", "www", "site", "wordpress" or the domain's name in it). A finding means the web server answered with the file. Nefo's Nginx configuration hides dot files, so on Nginx a `.env` file is found but not reported. Websites that are suspended, redirect elsewhere, ask for a password or are apps behind a proxy are not asked.
- **World-writable files**: the check walks the website's folder (`/var/www/<domain>`) without following symbolic links. A folder with the sticky bit, such as a shared `tmp`, is fine. Use 644 for files and 755 for folders.
- **PHP**: the check reads each PHP version's `php.ini` and `conf.d` files, and the PHP-FPM pools for `display_errors` turned on for one website.
- **WordPress** updates come from the last check on the **WordPress** page (or a website's **WordPress** tab). The security check never runs wp-cli itself. After you update a site, the next check there tells what is left.
- **The panel without HTTPS**: Nefo turns HTTPS on when it is installed. If it is off, set `"enabled": true` in the `"tls"` part of `/etc/nefo/config.json` and run `systemctl restart nefo`. A panel that only listens on 127.0.0.1 is not reported.

## How long does it take? {#limits}

The check reads the panel's own data and a few files, and asks only this server's own web server (127.0.0.1) for files that exist in a website's folder. Each check has a limit, so a run stays short on a server with many websites:

| Limit | Value |
|---|---|
| A run you start | 25 seconds at most. Checks not done by then are listed as stopped at their limit, and the result says it is partial |
| Each program the check asks (fail2ban, Docker, apt…) | 5 seconds at most; a program that doesn't answer in time is left out |
| Files and folders walked in one website's folder | 20,000 (100,000 for all websites together) |
| Entries read in one website's `public` folder | 5,000 |
| Requests to this server's web server | 200 in all, 4 at a time, 3 seconds each at most |

## Accept a finding

Some findings are fine on your server: SSH on port 22 behind fail2ban, or a database port you opened to one address on purpose. Click **Accept** on such a finding. It moves to the **Accepted** tab and no longer counts.

An accepted finding comes back when its facts change, for example when another account gets user ID 0, or a database starts listening on another address. It also comes back if it was fixed and then appears again. To bring it back yourself, open the **Accepted** tab and click **Show again**. Accepting and showing again are written to the activity log.

## Daily check and alerts

With **Run every day** on (the default), the panel runs the check once a day in the background, during its hourly maintenance, but never in the first 10 minutes after the panel starts. The dashboard then always shows a recent result.

Turn on **Alert me when the daily check finds a new problem** (off by default) to get a message on your alert channels (**Settings** → **Alerts**) when the daily check finds a critical problem or a warning that the previous check didn't have. Notes and accepted findings are never sent. See [Alerts](./mail-alerts#alerts).

## On the dashboard

When the last check found critical problems or warnings, **Needs attention** on the dashboard and the bell show "Security check: 3 problems to fix" with a **View** button. When no serious problem is open, the **Security risks** tile shows when the check last ran, and clicking it opens the security check.

To be told about sign-ins to SSH and the panel, see [Sign-in alerts](./mail-alerts#sign-in-alerts).
