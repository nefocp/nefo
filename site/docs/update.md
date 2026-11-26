---
title: Update
description: Update Nefo from the panel or with nefo update, and learn how releases are verified and what to do if an update fails.
---

# Update

This page shows how to update Nefo, how each release is checked before it is installed, and what to do if an update fails. Until version 1.0, only the latest release receives security fixes, so keep Nefo up to date.

## Update from the panel

1. Open **Settings → Panel**. The current version is shown next to the name Nefo.
2. Click **Check for updates**. If you already have the latest release, the panel says "This is the latest version."
3. If a newer release exists, the panel says "Version v0.2.0 is available." Click **Install update** and confirm.

The progress shows on the page. When the new version is in place, the panel restarts, and the page reloads by itself after about 20 seconds. Your websites keep running during the restart.

You can also update from the dashboard: click **Check for updates**, then **Update to v0.2.0**.

## Update from the command line

```sh
nefo update
```

```
Installed: v0.1.0
Latest:    v0.2.0
Release signature verified
Downloading nefo-linux-amd64 v0.2.0
Checked: Nefo v0.2.0 (commit 5e6f7a8)
Installed. The panel restarts now; reload the page in a few seconds.
```

If you already have the latest release, the command prints the two versions and changes nothing. To only check, run `nefo update --check`.

## What happens during an update

1. The panel reads the latest version number from the project's GitHub releases.
2. It downloads the list of checksums and its signature, and checks the signature with the project's key, which is built into the panel.
3. It downloads the new program for your CPU and checks its SHA-256 checksum against the list.
4. It runs the new program once, to make sure it works on this server.
5. It replaces `/usr/local/bin/nefo` in one step, so there is never a half-written program.
6. It restarts the `nefo` service. If a task such as a software installation is running, the panel lets it finish first, for up to 75 seconds.

When the new version starts, it updates the panel's database to its new format. It also brings the configuration files it manages (web server, PHP-FPM pools) up to date with its templates. If the web server rejects the new files, the panel puts the old ones back.

## How releases are verified

Each release on GitHub contains the programs for amd64 and arm64, `install.sh`, a `VERSION` file, `checksums.txt` with the SHA-256 checksums of the files, and `checksums.txt.sig`, an Ed25519 signature of `checksums.txt`.

Both the installer and the updater refuse a `checksums.txt` that isn't signed with the project's key, and a download whose checksum doesn't match. During an update, the errors read:

- "checksums.txt has no valid signature from the project: the update was not installed";
- "the download does not match checksums.txt: not installed".

So someone who could change files on GitHub, or along the way to your server, still can't get their own program installed.

A development build, made without the project's key, can't check signatures. It checks only the checksum, and its log says "Development build: the release signature is not checked".

## What is kept

An update replaces only the program, `/usr/local/bin/nefo`. Everything else stays:

- the configuration file `/etc/nefo/config.json`, including the port and the secret path;
- the data folder `/var/lib/nefo`: your account and two-factor authentication, settings, websites, schedules, certificates and the audit log;
- your websites, databases, files and backups, and the services that run them.

Only the panel restarts. Websites, databases and other services keep running.

## If an update fails

**The update stops before the new program is installed.** This happens when the download fails, the signature or checksum doesn't match, or the new program doesn't run on this server. Nothing has changed: the old version keeps running. The error shows in the panel's task log, or in the terminal. Try again later, and report the error if it continues.

**The new version doesn't start.** Check the service and its log:

```sh
systemctl status nefo
journalctl -u nefo -n 50 --no-pager
```

Report the problem in the [GitHub repository](https://github.com/nefocp/nefo/issues) with this output. To go back to the previous release, install its program by hand. This example uses `v0.1.0` on an amd64 server:

```sh
cd /tmp
curl -fsSLO https://github.com/nefocp/nefo/releases/download/v0.1.0/nefo-linux-amd64
curl -fsSLO https://github.com/nefocp/nefo/releases/download/v0.1.0/checksums.txt
grep " nefo-linux-amd64$" checksums.txt | sha256sum -c -
install -m 0755 nefo-linux-amd64 /usr/local/bin/nefo
systemctl restart nefo
```

Go on only if `sha256sum` prints `OK`. On an ARM server, use `nefo-linux-arm64`.

::: warning Going back to an older version
If the newer version already changed the panel's database, an older version refuses to start with "database schema version … is newer than this panel supports". Then only a newer version works with that data. Before an important update, you can keep a copy of the data folder: `systemctl stop nefo`, then `cp -a /var/lib/nefo /root/nefo-data-copy`, then `systemctl start nefo`.
:::

## Automatic updates

Nefo doesn't update itself, and it doesn't check for updates on its own. It checks only when you click **Check for updates** or run `nefo update`. To hear about new releases, watch the releases of the [GitHub repository](https://github.com/nefocp/nefo/releases).

## Where updates come from

By default, updates come from `https://github.com/nefocp/nefo/releases/latest/download`. The `update_url` field in `/etc/nefo/config.json` can point to another address that publishes the same files. It must start with `https://`, otherwise checking for updates fails with "update_url in config.json must be an https:// address." Leave it unset unless you know you need it.
