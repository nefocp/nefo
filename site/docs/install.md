---
title: Install
description: Install Nefo on an Ubuntu or Debian server with one command, then open the panel for the first time.
---

# Install

This page shows how to install Nefo, what the installer does, and how to open the panel for the first time. Check the [requirements](./requirements) first.

## Run the installer

1. Connect to your server over SSH.
2. Run:

   ```sh
   curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | sudo bash
   ```

   If you are signed in as root and `sudo` isn't installed (common on Debian), leave it out:

   ```sh
   curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | bash
   ```

3. Wait until the installer prints the panel's address, username and password. This usually takes a few seconds.

::: tip Commands in these docs
Every other command in these docs is run as root on the server, so it is shown without `sudo`.
:::

## What the installer prints

The end of the output looks like this:

```
==> Downloading nefo-linux-amd64 (latest)
==> Verifying the release signature
==> Verifying checksum
==> Creating the configuration and the admin account
==> Installing the systemd service
==> Opening port 28443/tcp in ufw

Nefo is ready.

  URL:      https://203.0.113.10:28443/k3x9p2qa7m1z/
  Username: admin
  Password: ••••••••••••••••••••

Save this password now: it is stored only as a hash and cannot be shown again.
After signing in, change it and turn on two-factor authentication.
The certificate is self-signed, so your browser will warn you the first time.

If your provider has a cloud firewall (security group), allow TCP port 28443 there too.
Manage the service with: systemctl status|restart nefo   Logs: journalctl -u nefo
```

Your port, path and password are different: they are chosen at random for each server.

::: warning Save the password now
The password is shown only once. The panel stores only a hash of it. If you lose it, set a new one with `nefo reset-password` (see [Command line](./cli#nefo-reset-password)).
:::

The address uses the first IP address of the server. If your provider gives the server a private address and a separate public one, replace the address in the URL with the public IP.

## What the installer does

1. **Checks the server.** It stops if it isn't run as root, if systemd, `curl` or `sha256sum` is missing, if the operating system or the CPU isn't supported, or if Nefo is already installed (`/etc/nefo/config.json` exists).
2. **Downloads** the program for your CPU (`nefo-linux-amd64` or `nefo-linux-arm64`) and the list of checksums from the project's GitHub releases.
3. **Checks the signature** of the checksum list with the project's Ed25519 key, which is built into the installer. A release without a valid signature is refused: "Do not install this file." An installer from a development build has no key, and says that the signature is not checked.
4. **Checks the SHA-256 checksum** of the download. A mismatch stops the installation.
5. **Installs** the program as `/usr/local/bin/nefo`, and creates `/etc/nefo` and `/var/lib/nefo`, readable by root only.
6. **Creates the configuration** in `/etc/nefo/config.json`: a random port from 20000 to 49999, a random 12-character secret path, and HTTPS turned on. It also creates the account `admin` with a random 20-character password.
7. **Installs the systemd service** `nefo`, starts it, and enables it at boot.
8. **Opens the panel's port** in ufw if ufw is active, or in firewalld if that is running. If neither is active, it changes no firewall.

Unless you give it [setup options](#set-the-server-up-in-the-same-run), the installer installs nothing else: no web server, PHP or database. The panel offers to set those up when you first sign in (see [First steps](./first-steps#set-up-the-server)).

## Open the panel for the first time

1. Open the URL from the installer in your browser, exactly as printed. It starts with `https://` and ends with the secret path and a slash.
2. Your browser warns that the connection isn't private or that the certificate isn't trusted. This is expected: the panel created its own self-signed certificate on first start. Choose the option to continue to the site. In most browsers it is behind a button such as "Advanced".
3. Sign in with the username `admin` and the password from the installer.

Then follow the [first steps](./first-steps): change the password, turn on two-factor authentication, and give the panel its own domain and a free certificate, so the browser warning goes away.

### The secret path

The panel answers only under its secret path, such as `/k3x9p2qa7m1z/`. Every other address on its port returns `404`, including the bare `https://203.0.113.10:28443/`. Scanners that try common panel addresses find nothing.

Keep the path private, like a password. If you forget it, run `nefo info` on the server (see [Command line](./cli#nefo-info)).

If you type `http://` instead of `https://`, the panel answers with an error instead of the sign-in page.

## Open the port in your cloud firewall

If your provider has a firewall outside the server (a security group or cloud firewall), allow the panel's TCP port there. The installer can't do this for you. Later, when you host websites, also allow TCP ports 80 and 443. See [Requirements](./requirements#firewalls).

## Environment variables

You can change what the installer does with these variables:

| Variable | Default | What it does |
|---|---|---|
| `NEFO_VERSION` | `latest` | Install a specific release, such as `v0.1.0`, instead of the latest one. |
| `NEFO_REPO` | `nefocp/nefo` | Download from another GitHub repository (`owner/name`), such as a fork. |
| `NEFO_FORCE` | not set | Set to `1` to try an operating system that isn't supported. The CPU check still applies. |
| `NEFO_SETUP` | not set | Options for `nefo setup`, to set the server up in the same run. See [below](#set-the-server-up-in-the-same-run). |

Put the variable before `bash`:

```sh
curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | NEFO_VERSION=v0.1.0 bash
```

With `sudo`, write it as `| sudo NEFO_VERSION=v0.1.0 bash`.

`NEFO_REPO` only changes where the installer downloads from. Later updates come from the repository built into the program, or from `update_url` in the configuration file (see [Update](./update#where-updates-come-from)).

## Set the server up in the same run

The installer can also set the server up, like the setup page of the panel: a web server, PHP versions, a database server and extras. Give it the options of [`nefo setup`](./cli#nefo-setup) after `bash -s --`:

```sh
curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | sudo bash -s -- --web nginx --php 8.4 --db mariadb --redis
```

Or put them in the `NEFO_SETUP` variable:

```sh
curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | sudo NEFO_SETUP="--web nginx --php 8.4 --db mariadb" bash
```

What changes with setup options:

- The installer checks the options right after the download, before it configures anything. A wrong option, such as `--web caddy`, stops it, and you can run it again. Options may only contain letters, digits and `. , = _ -`.
- Once the panel has started, the installer runs `nefo setup` with your options and `--yes`, and shows its log. This takes a few minutes. The panel does the installs as one of its tasks, so they go on even if your SSH session ends: follow them in the panel then.
- The address, username and password are printed at the end, after the setup.
- The panel no longer opens its setup page after you sign in.

If the setup stops, for example because a download fails, the panel stays installed. The installer still prints the address and password, then says that the setup stopped, and exits with an error. Fix the problem, then run `nefo setup` with the same options: the parts that are installed are skipped.

## Install again

The installer refuses to run when `/etc/nefo/config.json` exists, so it can't overwrite a working panel. To start over, remove Nefo first (see [Uninstall](./uninstall)), then run the installer again.
