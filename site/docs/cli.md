---
title: Command line
description: Every nefo command and its options (info, reset-password, update, setup, unlock-files, decrypt-backup), the configuration file, and how to manage the nefo service with systemctl and journalctl.
---

# Command line

The `nefo` program at `/usr/local/bin/nefo` runs the panel and has commands for the server's root user: `nefo info` shows the panel's address, `nefo reset-password` sets a new password (and can turn off 2FA), `nefo update` installs the latest release, and `nefo setup` installs the software. Run every command as root.

This page lists the commands, then shows how to manage the `nefo` service and what is in the configuration file.

## Commands

| Command | What it does |
|---|---|
| `nefo info` | Show the panel's URL |
| `nefo reset-password` | Set a new random password and sign out all sessions |
| `nefo update` | Install the latest release and restart the panel |
| `nefo setup` | Install a web server, PHP versions, a database server and extras |
| `nefo unlock-files` | Give the locked websites their files back (before you remove the panel) |
| `nefo decrypt-backup` | Decrypt a backup copy that remote storage encrypted |
| `nefo version` | Show the version |
| `nefo init` | Create the configuration file and the first admin account |
| `nefo serve` | Run the panel (systemd does this for you) |
| `nefo help` | Show the list of commands |

Run `nefo <command> -h` to see the options of a command. Options work with one dash or two: `-port` and `--port` are the same.

Every command except `version`, `help` and `decrypt-backup` reads the configuration file `/etc/nefo/config.json`. To use another file, add `--config /path/to/config.json`, or set the `NEFO_CONFIG` environment variable.

## nefo info

Shows the panel's URL and where its files are.

```sh
nefo info
```

```
URL:    https://panel.example.com:28443/k3x9p2qa7m1z/
URL:    https://203.0.113.10:28443/k3x9p2qa7m1z/
Config: /etc/nefo/config.json
Data:   /var/lib/nefo
```

The first line appears once the panel has its own domain (**Settings** → **Certificate**). The second uses the server's first IP address. If your provider gives the server a private address and a separate public one, pass the public one:

```sh
nefo info --host 203.0.113.10
```

| Option | What it does |
|---|---|
| `--host` | The host name or IP address to show in the URL |
| `--port` | Print only the port number, for scripts |
| `--config` | The configuration file to read |

## nefo reset-password

Sets a new random password for the account, signs it out on every device, and prints the password. Use it when you forgot the password.

```sh
nefo reset-password
```

```
New password for admin: ••••••••••••••••••••
```

If you also lost the phone with your authenticator app, add `--disable-2fa`. It turns off two-factor authentication for the account:

```sh
nefo reset-password --disable-2fa
```

```
New password for admin: ••••••••••••••••••••
Two-factor authentication is off. Turn it on again after signing in.
```

| Option | What it does |
|---|---|
| `--disable-2fa` | Also turn off two-factor authentication |
| `--user` | The account name (default `admin`, the panel's only account) |
| `--config` | The configuration file to read |

The panel keeps running: you don't need to restart it. The reset is written to the activity log.

## nefo update

Checks the project's GitHub releases. If there is a newer version, it downloads it, checks its signature and checksum, replaces the program and restarts the panel.

```sh
nefo update
```

To only see whether an update is available:

```sh
nefo update --check
```

```
Installed: v0.1.0
Latest:    v0.2.0
```

| Option | What it does |
|---|---|
| `--check` | Only show the installed and the latest version |
| `--config` | The configuration file to read |

See [Update](./update) for what the update checks and what to do if it fails.

## nefo setup

Sets the server up like the panel's setup page (see [First steps](./first-steps#set-up-the-server)): it installs a web server, PHP versions, a database server and extras, one after the other. Parts that are installed already are skipped, so you can run it again after a failure. Once it has finished, the panel no longer opens its setup page after sign-in.

```sh
nefo setup --web nginx --php 8.3,8.4 --db mariadb --redis
```

```
This server: 1 CPU, 960 MB of RAM (250 MB in use), 19.5 GB of free disk.

What the setup installs, one after the other:
  1. Nginx and PHP 8.3  about 40 MB of RAM, 55 MB of disk
  2. PHP 8.4            about 25 MB of RAM, 50 MB of disk, from packages.sury.org
  3. MariaDB            about 228 MB of RAM, 300 MB of disk
  4. Redis              up to 64 MB of RAM, 10 MB of disk
In all: about 357 MB more RAM and 415 MB of disk.

Install? [y/N]
```

After you answer `y`, it shows the log of each install. If one fails, it stops there, says what is installed already, and exits with an error. Without `--web`, `--php`, `--db` or an extra, it offers the recommended choice for the server's RAM, as the setup page does.

While the panel runs, `nefo setup` asks the panel to do the installs, as one of its tasks: they wait for any other task, show in the panel, and go on even if you close the SSH session. It talks to the panel through a socket in the data folder that only root can open. When the panel isn't running, `nefo setup` does the installs itself.

| Option | What it does |
|---|---|
| `--web` | `nginx`, `apache`, `openlitespeed` or `none`. The web server comes with your distribution's PHP version |
| `--php` | PHP versions separated by commas, such as `8.3,8.4`, or `none` |
| `--db` | `mariadb`, `mysql` or `none`; `mariadb-11.4`, `mariadb-11.8` or `mariadb-12.3` for an LTS series from MariaDB's repository, where the server offers it (see [MariaDB versions](./databases#mariadb-versions)). `--mariadb` is the same as `--db mariadb` |
| `--redis`, `--node`, `--python`, `--docker` | Also install Redis, Node.js (the newest LTS release), Python for app websites, or Docker |
| `--adminer`, `--waf` | Also install Adminer or the web firewall. Both need a web server |
| `--clamav` | Also install ClamAV, on servers with 2 GB of RAM or more |
| `--postgresql`, `--memcached` | Also install PostgreSQL or Memcached, where the Software page offers them |
| `--yes`, `-y` | Install without asking. Needed when no terminal is attached, as in a script |
| `--dry-run` | Only show what would be installed |
| `--skip` | Install nothing: the panel no longer opens its setup page after sign-in |
| `--wait` | How long to wait for the panel to start, such as `60s`. The installer uses it |
| `--check` | Only check the options. The installer uses it before it configures anything |
| `--config` | The configuration file to read |

The installer accepts the same options, to set the server up in the same run (see [Install](./install#set-the-server-up-in-the-same-run)).

## nefo unlock-files

Gives every website whose files are locked (**Websites** → (site) → **File lock**, see [Country blocking and file lock](./website-protection#lock-a-website-s-files)) its files back, with exactly the owner, group and permissions each had before the lock, and removes the websites' lock groups. Run it as root, with the panel stopped, before you remove Nefo (see [Uninstall](./uninstall#unlock-the-websites-files)).

```sh
systemctl stop nefo
nefo unlock-files
```

```
Unlocked the files of example.com
Unlocked the files of blog.example.com
```

When no website is locked, it prints `No website's files are locked.`

| Option | What it does |
|---|---|
| `--config` | The configuration file to read |

## nefo decrypt-backup

Decrypts a backup copy that a remote storage encrypted (a `.tar.gz.enc` file), for example one you downloaded from Google Drive. Only the passphrase is needed: it works on any server, also one where the panel is not set up, and it doesn't read the configuration file. See [Restore on a new server](./restore-new-server#decrypt-a-copy-by-hand).

```sh
nefo decrypt-backup example.com_20261126-030000.tar.gz.enc
```

```
Passphrase:
Decrypted to example.com_20261126-030000.tar.gz
SHA-256: 3f1c…
```

The passphrase is never given on the command line, where other users could see it. In a terminal it is asked for without being shown; otherwise it is the first line of standard input or of `-passphrase-file`.

| Option | What it does |
|---|---|
| `-o` | The output file. The default is the input's name without `.enc`. `-o -` writes to standard output, for example `nefo decrypt-backup -o - FILE.enc \| tar xz` |
| `-passphrase-file` | Read the passphrase from the first line of this file |
| `-force` | Replace the output file if it exists |

The output is written to a temporary file and renamed only when the whole copy decrypted, so a wrong passphrase or a damaged file leaves nothing behind (except with `-o -`, where the end of the file is checked last). A file that isn't encrypted is copied as it is, with a note. The SHA-256 it prints is the one the panel recorded for the backup.

## nefo version

Shows the version and the commit it was built from. `nefo --version` and `nefo -v` do the same.

```sh
nefo version
```

```
Nefo v0.1.0 (commit 1a2b3c4)
```

## nefo init

Creates the configuration file and the first admin account. The installer runs it for you, so you normally don't need it. It refuses to overwrite an existing configuration file.

It chooses a random port from 20000 to 49999 and a random secret path, turns on HTTPS, and creates the account `admin` with a random password. If the data folder already has accounts, it keeps them, writes only the new configuration file, and doesn't print a password.

| Option | What it does |
|---|---|
| `--config` | Where to write the configuration file (default `/etc/nefo/config.json`) |
| `--data` | The data folder (default `/var/lib/nefo`) |
| `--listen` | The address to listen on, such as `:28443`, instead of a random port |
| `--host` | The host name or IP address to show in the printed URL |
| `--dev` | Development mode: plain HTTP on `127.0.0.1:8080` with the path `dev`. Never use it on a server. |

## nefo serve

Runs the panel. The `nefo` service starts it with `nefo serve --config /etc/nefo/config.json`, so you don't run it yourself. A second copy would fail, because the service already uses the port.

## Manage the nefo service

| Command | What it does |
|---|---|
| `systemctl status nefo` | Show whether the panel is running, and its latest log lines |
| `systemctl restart nefo` | Restart the panel |
| `systemctl stop nefo` | Stop the panel |
| `systemctl start nefo` | Start the panel |
| `journalctl -u nefo` | Show the panel's log |
| `journalctl -u nefo -f` | Follow the log as new lines arrive |
| `journalctl -u nefo -n 100 --no-pager` | Show the last 100 lines |
| `journalctl -u nefo --since "1 hour ago"` | Show the log of the last hour |

Restarting the panel doesn't stop your websites, databases or other services. You can also restart it from **Settings** → **Panel** → **Power** → **Restart the panel**.

When the panel stops, it lets a running task, such as a software installation, finish first, for up to 75 seconds. If the panel stops because of an error, systemd starts it again after 3 seconds.

The service file is `/etc/systemd/system/nefo.service`:

```ini
[Unit]
Description=Nefo server control panel
Documentation=https://nefo.io
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/nefo serve --config /etc/nefo/config.json
Restart=on-failure
RestartSec=3
UMask=0077
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

The panel runs as root, because it manages the whole server.

## The configuration file

`/etc/nefo/config.json` is readable by root only. The installer writes it, for example:

```json
{
  "listen": ":28443",
  "entrance": "k3x9p2qa7m1z",
  "data_dir": "/var/lib/nefo",
  "tls": {
    "enabled": true
  }
}
```

| Field | What it means |
|---|---|
| `listen` | The address and port the panel listens on. `:28443` means port 28443 on every address. |
| `entrance` | The secret path. 8 to 32 lowercase letters or digits. Every other path answers `404`. |
| `data_dir` | The data folder: the panel's database `nefo.db`, certificates and keys. |
| `tls.enabled` | HTTPS for the panel. Keep it on. |
| `tls.cert`, `tls.key` | Optional paths to your own certificate and key for the panel. They take priority over the panel's Let's Encrypt certificate and the self-signed one. |
| `acme_directory` | Optional certificate authority for certificates. Empty means Let's Encrypt. Set `https://acme-staging-v02.api.letsencrypt.org/directory` while testing: staging certificates aren't trusted by browsers, but failed attempts don't count against Let's Encrypt's limits. |
| `update_url` | Optional address that updates are downloaded from. Empty means the project's GitHub releases. It must start with `https://`. |
| `dev` | Development mode. Never set it on a server. |

After you edit the file, restart the panel with `systemctl restart nefo`. If the file has an error, the panel doesn't start: `journalctl -u nefo` shows the reason.

::: warning Changing the port
If you change the port in `listen`, open the new port first, both in ufw (for example `ufw allow 31234/tcp`) and in your provider's cloud firewall. Otherwise you can't reach the panel after the restart.
:::
