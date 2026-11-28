---
title: Security
description: How Nefo protects its own sign-in, and how to use the security check, the firewall, fail2ban, SSH settings, the web application firewall, country blocking, the file lock and the malware scanner.
---

# Security

Nefo runs as root, so it protects its own sign-in first: a secret address on a random port, HTTPS from the first start, two-factor authentication, sign-in limits and an activity log. It also manages the server's firewall (ufw), fail2ban and SSH settings with lock-out guards, and protects websites with a web application firewall, rate limits, blocked addresses and a malware scanner.

This page covers:

- the panel's own protection: the hidden address, HTTPS, sign-in limits, two-factor authentication, sessions and the activity log;
- the [security check](./security-check), which lists the settings that leave the server open and where to fix each one;
- the server's firewall (ufw), fail2ban and SSH settings, on the **Firewall & SSH** page;
- website security: the web application firewall, rate limits, blocked addresses, country blocking, the file lock and the malware scanner.

Nefo is beta and has not had an outside security audit yet. See the [security policy](../security-policy).

## Security check

**Security check**, in the **Security** group of the left menu, runs 22 quick checks of the server, such as SSH password sign-in, the firewall, waiting security updates, databases open to the internet, Docker ports, files a website serves by mistake and certificates. Each finding links to the page that fixes it. It runs on demand and once a day. See [Security check](./security-check).

## The panel's own protection

### Hidden entrance path

The installer picks a random port between 20000 and 49999 and a secret path of 12 lowercase letters and digits. The panel only answers under that path, for example `https://203.0.113.10:28443/k3x9p2qa7m1z/`. Every other URL on that port returns `404`, so a scanner can't tell that a panel runs there.

If you lose the address, run this on the server:

```sh
nefo info
```

It prints the URL with the server's first IP address, and also with the panel's domain once it has one (see below). If the server sits behind a separate public address, run `nefo info --host 203.0.113.10` to print the URL with it.

The port and the path are stored as `listen` and `entrance` in `/etc/nefo/config.json`. The panel has no setting to change them. To change the path by hand, set `entrance` to 8 to 32 lowercase letters and digits, then run `systemctl restart nefo`.

### HTTPS from the first start

The panel serves HTTPS from its first start, with TLS 1.2 or newer. It creates a self-signed certificate in `/var/lib/nefo/tls/`, so your browser warns you the first time. To get rid of the warning, get a free Let's Encrypt certificate for a domain that points to this server:

1. Install the web server from **Websites** if you haven't yet. Let's Encrypt checks the domain on port 80, and the web server answers it.
2. Point the domain's A record to the server, without a proxy or CDN.
3. Go to **Settings** → **Certificate**.
4. Enter the **Domain of the panel**, for example `panel.example.com`, and select **Get certificate**.
5. Open the panel at `https://panel.example.com:28443/k3x9p2qa7m1z/`, with your own port and path.

The certificate renews on its own, like the websites' certificates; a failed renewal is retried every 12 hours. To remove it, select **Delete**: the panel goes back to its self-signed certificate. See [SSL certificates](./ssl) for certificates in general.

### Sign-in limits

- 5 failed attempts within 15 minutes lock the address out for 15 minutes. The sign-in page then says "Too many failed attempts. Try again in … seconds."
- A wrong password and a wrong two-factor code both count. So does a wrong current password in the password and 2FA forms of the **Account** page.
- IPv6 addresses are counted per /64 network, because one user usually controls a whole /64.
- A successful sign-in clears the count.
- A wrong username and a wrong password give the same message and take the same time, so the sign-in page doesn't reveal which usernames exist.
- The panel counts the address that connects to it and ignores `X-Forwarded-For`. If you put a proxy in front of the panel, all visitors share one count.

Passwords are stored as bcrypt hashes, never in plain text.

::: tip
If you lock yourself out, wait 15 minutes. The count is kept in memory, so restarting the panel on the server (`systemctl restart nefo`) also clears it.
:::

### Sessions and signing out

- A sign-in lasts 12 hours. After that you sign in again, even if you were active.
- To sign out, open the account menu at the top right and select **Sign out**.
- Changing your password signs you out on every device. So does `nefo reset-password` on the server.
- The session cookie can't be read by scripts, is only sent over HTTPS and only to the panel's own path.

### Security headers

Every page is sent with a strict Content Security Policy that allows scripts only from the panel itself. The panel also sends `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer` (which keeps the secret path out of other websites' logs) and a `Permissions-Policy` that turns off the camera, microphone and location. Every change needs a request header that other websites can't send, which blocks cross-site request forgery.

## Two-factor authentication

Two-factor authentication (2FA) asks for a 6-digit code from an authenticator app at every sign-in, so a stolen password isn't enough. Any app that supports standard time-based codes works, such as Google Authenticator, Authy or 2FAS. While 2FA is off, the panel lists "Two-factor sign-in is off" as a serious problem.

### Turn on 2FA

1. Go to **Account** in the left menu. You can also open the account menu and select **Password and 2FA**.
2. Under **Two-factor authentication**, select **Set up**.
3. Scan the QR code with your app, or type the **Setup key** into it. The app lists the account as Nefo.
4. Enter the 6-digit code that the app shows and select **Turn on**.

From now on, the sign-in page asks for a **Two-factor code** after your password.

Codes change every 30 seconds. The panel also accepts the previous and the next code, in case the clocks differ a little. Each code works only once: if a code is refused, wait for the next one.

### Turn off 2FA

On **Account**, under **Two-factor authentication**, enter your **Password** and a current **Code**, then select **Turn off**.

### If you lose your phone

Nefo has no backup codes. You recover through the server: sign in over SSH as root and run:

```sh
nefo reset-password --disable-2fa
```

This sets a new random password for the `admin` account, turns off 2FA, signs out all sessions and prints the new password. Sign in with it, change the password and set up 2FA again. See [Command line](./cli).

::: warning
Without your phone, root access to the server is the only way back into the panel. Keep your SSH key or root password, or your provider's web console, safe.
:::

## Change the panel password

1. Go to **Account** → **Change password**.
2. Enter your **Current password**, then the **New password** twice (**Repeat new password**). It must be 12 to 72 characters long.
3. Select **Change password**.

You are signed out on every device. Sign in again with the new password.

## Activity log

**Account** → **Recent activity** shows the 100 newest entries, with the time, action, user, IP address and detail. Show **All** entries or only **Failures**, and type in the filter box to search. The top of the page shows your last sign-in and how many failed sign-ins the log holds.

The log records:

- sign-ins, sign-outs and failed sign-ins, with the reason (wrong username or password, or wrong 2FA code);
- password changes, password resets from the command line, and 2FA turned on or off;
- changes made in the panel: websites, certificates, databases, files, backups, cron jobs, services, firewall rules, SSH settings and keys, software and Docker;
- opening and closing the terminal, with how long it was open;
- panel restarts and server reboots.

Automatic certificate renewals are recorded with the user `system`. Entries are kept for 90 days. Each entry is also written to the panel's service log, which you can read with `journalctl -u nefo`. fail2ban reads failed sign-ins from there.

## Firewall (ufw) {#firewall-ufw}

Nefo manages the server's firewall with ufw. Go to **Firewall & SSH** → **Firewall (ufw)**.

If the page says ufw is not installed, install it on the server. The panel doesn't install it:

```sh
apt install ufw
```

### Turn on the firewall

1. Select **Turn on the firewall** and confirm.
2. The panel first allows the SSH port (comment `SSH`) and the panel's port (comment `Nefo`). When a web server is installed, it also allows ports 80 and 443 for websites (comment `Websites`), and 443/UDP when OpenLiteSpeed answers HTTP/3. Then it turns ufw on. It adds these rules even if you added a rule for one of these ports while the firewall was off, for example one limited to your address, so turning the firewall on never locks you out. To keep only your rule, delete the one for everyone afterwards (see the end of [What the firewall refuses](#what-the-firewall-refuses)).

You can [add rules](#add-a-rule) before you turn the firewall on, for example for a port that another program on the server needs. While the firewall is off, the page shows them below **Turn on the firewall** as ufw commands, such as `ufw allow 8080/tcp`, in the **Rule (ufw command)** column, with each IPv4 and IPv6 pair shown once. They take effect when you turn the firewall on. ufw numbers its rules only while it is on, so to delete one of them, turn the firewall on first.

All other incoming ports are closed unless a rule allows them. When you install the web server later, with the firewall on, Nefo opens ports 80 and 443 at that time.

If your provider has a cloud firewall (security group), open the same ports there too. Nefo can't change it.

### Add a rule

1. Select **Add rule**.
2. Choose **Allow** or **Deny**.
3. In **Port, e.g. 8080**, enter one port, or a range such as `6000:6010`.
4. Choose the **Protocol**: TCP, UDP, or TCP + UDP. A range needs one protocol: with TCP + UDP it becomes TCP.
5. In **From**, enter an IP address or a network such as `198.51.100.0/24`. Leave it empty to allow or deny everyone.
6. Optionally, add a **Comment** of up to 60 characters, without quotes.
7. Select **Add**.

The page shows your own IP address above the list, which helps when you allow a port only for yourself.

ufw checks the rules from top to bottom and uses the first one that matches. A **Deny** rule with an address in **From** is put at the top of the list, so it takes effect even on a port that a later rule allows for everyone. Other rules are added at the end.

::: tip
To keep an address away from one website only, use the website's **Block access** settings instead (see below).
:::

The list also shows rules you added with ufw on the command line. **Limit** allows a port but refuses, for a while, an address that opens 6 connections within 30 seconds (`ufw limit`). **Reject** refuses like **Deny**, but tells the other side at once.

### Delete a rule

Select the bin icon at the end of the rule's row and confirm. If the rules changed since you opened the page, the panel refuses and asks you to reload, so it never deletes the wrong rule.

### What the firewall refuses

These guards stop you from locking yourself out:

- A rule that allows the SSH port or the panel's port shows **protected** when it is your last way in from the address you use now (shown above the list). It can't be deleted here. When another rule also lets your address in, the rule can be deleted.
- A **Deny** rule for the SSH port or the panel's port with an empty **From** is refused. A **Deny** rule for one address or network is allowed.

To let only your own address reach the panel: add an **Allow** rule for the panel's port with your address in **From**, then delete the rule that allows the port from anywhere (and its IPv6 copy). If your address changes later, use your provider's web console to run `ufw allow <port>/tcp`.

Rules for remote database access (port 3306 for MariaDB and MySQL, the cluster's port, 5432 by default, for PostgreSQL) are added and removed from the **Databases** page. Change them there. See [Databases](./databases#allow-remote-access) and [PostgreSQL](./postgresql#postgresql-remote-access).

## Brute-force protection (fail2ban) {#brute-force-protection-fail2ban}

fail2ban blocks addresses that keep failing to sign in, to SSH and to the panel.

### Install fail2ban

1. Go to **Firewall & SSH** → **Brute-force protection (fail2ban)**.
2. Select **Install fail2ban**. The install runs in the background with a live log.

Nefo sets up two jails in `/etc/fail2ban/jail.d/panel.conf`:

| Jail | Watches | Blocks after | Block time |
|---|---|---|---|
| `sshd` | SSH sign-ins, SFTP included | 5 failures within 10 minutes | 1 hour |
| `panel` | sign-ins to the panel that fail, or are refused while the address is locked out | 10 within 10 minutes | 1 hour |

A blocked address can't reach the port that the jail protects. The panel's own sign-in limit (5 failures in 15 minutes) acts first. An address that keeps trying while it is locked out is then blocked by the `panel` jail in the firewall.

The panel rewrites this file, and its filter in `/etc/fail2ban/filter.d/panel.conf`, so don't edit them by hand. When you change the SSH port, the `sshd` jail follows it.

### See and unblock addresses

The table lists each **Protected service**, its **Recent failures** and the addresses **Blocked now**, with the number of blocks in total. To unblock an address, select **Unblock** next to it.

If fail2ban is installed but stopped, start it on the **Services** page.

::: tip
If you blocked yourself from SSH, sign in to the panel from your current network and select **Unblock**. You can also run `fail2ban-client set sshd unbanip 203.0.113.10` from the panel's **Terminal**.
:::

## SSH settings

Go to **Firewall & SSH** → **SSH**.

| Setting | Choices |
|---|---|
| **Port** | 1 to 65535 |
| **root sign-in** | **With an SSH key only (recommended)**, **With a key or a password**, **Not allowed** |
| **Allow sign-in with a password** | On or off, for every account |

Select **Save**. The panel writes its settings to `/etc/ssh/sshd_config.d/00-panel.conf`, which takes precedence over the other SSH settings files. It also turns off keyboard-interactive sign-in. Before it restarts SSH, it tests the new settings with `sshd -t`. If the test fails, the old file comes back and nothing changes.

::: warning
Keep your current SSH session open after you save. Open a new connection and check that you can sign in before you close the old one.
:::

### What the SSH settings refuse

- Settings with which nobody could sign in over SSH any more. For example: turning off password sign-in while no account has an SSH key; root sign-in **With an SSH key only** while root has no key; or root **Not allowed** while no other account can sign in.
- A port that is already taken: the panel's own port, 80, 443, or any port another program listens on.
- A new port when SSH is started by systemd socket activation. The page then shows **socket activated** and the port field is locked. Change the port on the command line.

To decide whether someone could still sign in, the panel looks at root and at the people's accounts (UID 1000 and up) with a login shell: their keys in `~/.ssh/authorized_keys` (or the files set by `AuthorizedKeysFile`), their passwords, and `AllowUsers`, `DenyUsers`, `AllowGroups` and `DenyGroups`. It doesn't read `Match` blocks. The page lists who can sign in now, and how (key or password). When root has no key, it also shows a warning with an **Add key** button (see [SSH keys](#ssh-keys)).

SSH can't be stopped from the **Services** page, only restarted or reloaded.

::: warning
With root sign-in **Not allowed**, you need another account that can use `sudo`, or you can sign in but not manage the server over SSH. Nefo doesn't create that account, and doesn't check that it can use `sudo`.
:::

### Change the SSH port

1. Enter the new **Port**, select **Save** and confirm.
2. If the firewall is on, the panel opens the new port (comment `SSH`) before SSH moves to it. If fail2ban is installed, its `sshd` jail moves to the new port too.
3. If your provider has a cloud firewall, open the new port there too.
4. Test a new connection, for example `ssh -p 2222 root@203.0.113.10`.
5. The firewall rule for the old port stays, and is no longer protected. Delete it under **Firewall (ufw)** once the new port works.

The SFTP accounts of your websites connect on the same port.

### SSH keys

**SSH keys**, below the SSH settings, lists the public keys of root and of the people's accounts (UID 1000 and up, with a login shell), grouped by account. Nefo reads them from `~/.ssh/authorized_keys`, or from the files set by `AuthorizedKeysFile`. Each key shows its type and size (such as `ed25519` or `rsa 4096`), its comment, its SHA256 fingerprint, and its options if it has any, such as `from="…"` or `command="…"`. A DSA key, or an RSA key under 2048 bits, is marked **weak**: current OpenSSH refuses them. Add a new key, then remove the weak one.

The SFTP accounts of your websites are not listed: their keys are in each website's SFTP settings.

#### Add a key

1. On your computer, make a key pair if you don't have one yet. This works the same in PowerShell on Windows 10 and later:
   ```sh
   ssh-keygen -t ed25519
   ```
   The private key goes to `~/.ssh/id_ed25519` and stays on your computer. The public key goes to `~/.ssh/id_ed25519.pub`.
2. Print the public key and copy the whole line, which starts with `ssh-ed25519`:
   ```sh
   cat ~/.ssh/id_ed25519.pub
   ```
3. In the panel, under **SSH keys**, select **Add key**.
4. Choose the **Account**, paste the line into **Public key** and select **Add key**.
5. Test it from a new terminal, for example `ssh root@203.0.113.10`.

Nefo adds the key at the end of the account's `~/.ssh/authorized_keys`. When the folder or the file is missing, it creates them, owned by the account, with modes 700 and 600. It refuses:

- a private key, such as one that starts with `-----BEGIN OPENSSH PRIVATE KEY-----`. If you pasted a private key somewhere by mistake, make a new key pair;
- more than one line. In PuTTYgen, copy the box **Public key for pasting into OpenSSH authorized_keys file**;
- DSA keys, and RSA keys under 2048 bits;
- a key the account already has;
- an `authorized_keys` file or `.ssh` folder that is a symbolic link, or not a regular file or folder. An account could point it at another file of the server, so Nefo neither reads nor changes it.

#### Remove a key

Select the bin icon next to the key and confirm. The confirmation names the key by its type, fingerprint and comment. If nobody could sign in over SSH without this key, with the current settings, the panel refuses: add another key first, or allow sign-in with a password.

Added and removed keys are recorded in the activity log, with the account and the key's fingerprint.

### Turn off password sign-in

1. Under [SSH keys](#ssh-keys), add your public key for root, or for the account you sign in with.
2. Open a new terminal and sign in with the key. If SSH asks for a password, the key isn't used: check that you added it to the account you sign in as.
3. Turn off **Allow sign-in with a password** and select **Save**. If nobody has a key, the panel refuses (see [What the SSH settings refuse](#what-the-ssh-settings-refuse)).
4. Keep your current session open, and check a new connection once more before you close it.

The SFTP accounts of your websites keep their own sign-in method, password or key, even with password sign-in off for SSH.

While root can sign in with a password, the panel lists "Root can sign in over SSH with a password" as a serious problem.

## Website security

Each website has its own security settings. Go to **Websites**, open the website by selecting its name or **Settings**, and use the tabs under **Security**.

### Web application firewall

The web application firewall (WAF) is ModSecurity with the OWASP Core Rule Set, both free and open source. It checks every request and stops SQL injection, cross-site scripting (XSS), file inclusion, known scanners and other common attacks. It works with Nginx, Apache and OpenLiteSpeed.

1. Install it once: **Software** → **Web firewall (ModSecurity)** → **Install**. Websites keep the firewall off until you turn it on. Where the distribution has no ModSecurity module for the web server, the card says so and offers no install: Ubuntu 22.04 has none for Nginx, while Apache and OpenLiteSpeed work there, and Nginx works from Ubuntu 24.04 on.
2. Go to **Websites** → (site) → **Web firewall (WAF)**.
3. Choose a mode:
   - **Off**
   - **Detect only: log attacks, block nothing**
   - **Block attacks**: the visitor gets HTTP 403.
4. Select **Save**.

Start with detect only for a few days, and check the events for false alarms before you block. Websites that run WordPress or use the WordPress or Drupal rewrite preset get the rule set's exceptions for those applications automatically.

**Recent firewall events** lists the 50 newest requests that the rules flagged on this website: the time, the IP address, the request, the rules that matched, and whether it was **blocked** or only **logged**. The events come from `/var/log/panel-waf/audit.log`, which is rotated daily and kept for 7 days.

Only the first 13 MB of a request are inspected, so large uploads still work. If you remove the WAF on the **Software** page, each website keeps its mode, which applies again when you reinstall.

### Rate limit and blocked addresses

Go to **Websites** → (site) → **Block access**.

- **Rate limit (requests per second per visitor, 0 = off)**: counts PHP and app requests only, not images, CSS or scripts. Bursts of up to five times the limit are allowed; above that, visitors get HTTP 429. 10 suits most websites, and the highest value is 1000. Only Nginx has this setting: on Apache and OpenLiteSpeed it isn't shown.
- **Blocked IP addresses**: one IPv4 or IPv6 address or range per line, such as `203.0.113.7` or `198.51.100.0/24`, up to 200 per website. Blocked visitors get HTTP 403.

Select **Save**. Both work on their own, without the web application firewall, and on every website type, reverse proxy websites included. The same tab has **Password protection**, described in [Websites](./websites#suspend-redirect-or-password-protect-a-website).

### Malware scan

The scanner looks for web shells, backdoors, hidden code and injected scripts in a website's files. It changes nothing until you quarantine a file.

1. Go to **Websites** → (site) → **Malware scan**.
2. Select **Scan now**. The scan runs in the background with a live log.

The scanner reads the website's folder, such as `/var/www/example.com`. It skips `.git` folders and files over 5 MB, and doesn't follow symbolic links. It checks PHP files (also PHP code in files with other names), JavaScript and HTML files, and images.

| Finding | Severity |
|---|---|
| Known web shell | critical |
| Runs code sent by the visitor | critical |
| Calls a function named by the visitor | critical |
| Runs shell commands sent by the visitor | critical |
| Runs hidden (encoded) code | high |
| Decodes hidden data sent by the visitor | high |
| Builds a function from the visitor's input | high |
| Uploader that lets the visitor choose where files go | high |
| Obfuscated script injection | high |
| PHP code hidden in an image | high |
| Code execution through preg_replace /e | check |
| Long encoded data next to code that decodes it | check |
| Heavily obfuscated text next to code that runs it | check |
| PHP file in an uploads folder | check |

Each finding shows the file, the line and a short excerpt. A PHP file in an uploads folder that holds only a comment, like WordPress's "Silence is golden" `index.php`, is not reported.

The panel keeps the latest scan of each website. It doesn't scan on a schedule: start a scan when you want one.

### Quarantine and restore

Select **Quarantine** next to a finding to move the file out of the website, into `/var/lib/panel-quarantine`, which only root can read. The panel records the file's owner and permissions.

Under **Quarantined files**:

- **Restore** puts the file back where it was, with its owner and permissions. It is refused if a file now exists at that path, or if the folder no longer exists.
- **Delete** removes the file for good, after you confirm.

When you delete a website, its quarantined files are deleted too.

::: warning
Quarantining a file that the website needs, such as a plugin file, can break the website. Check the excerpt first, and restore the file if it was a false alarm.
:::

### Scan with ClamAV

ClamAV is an optional, extra scan engine.

1. Install **ClamAV antivirus** on the **Software** page. Its virus databases download in the background for a few minutes after the install.
2. On **Malware scan**, tick **Also scan with ClamAV (slower, needs about 1.4 GB of free memory)** and select **Scan now**.

ClamAV needs about 1.4 GB of free memory while it scans, so it suits servers with 2 GB or more. If less memory is free, the panel skips ClamAV and says so in the report. ClamAV runs at the lowest CPU priority and skips files over 25 MB. Its findings are marked high.

### Country blocking and file lock

Each website also has **Country blocking**, which refuses visitors from the countries you list or lets in only those, and a **File lock**, which makes the website's code read-only for its PHP and its SFTP account except the folders it must write to. See [Country blocking and file lock](./website-protection).

## SSH sign-in history

The list of SSH sign-ins and failed attempts is on the **Processes** page. See [SSH sign-ins](./system#ssh-sign-ins). To get a message when someone signs in over SSH or to the panel, or when SSH sign-ins keep failing, see [Sign-in alerts](./mail-alerts#sign-in-alerts).
