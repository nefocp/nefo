---
title: Uninstall
description: Remove Nefo from a server by hand, and decide first what happens to the websites and software it set up.
---

# Uninstall

Nefo has no uninstall command: you remove it by hand, as root, in a few steps. Unlock any locked websites with `nefo unlock-files`, stop and remove the `nefo` service and `/usr/local/bin/nefo`, close the panel's port, and delete `/etc/nefo` and `/var/lib/nefo`. Your websites, databases and software keep running unless you remove them first.

This page shows each step. It also explains what stays on the server, because removing the panel doesn't remove the websites and software you set up with it.

## What stays when you remove Nefo

The steps on this page remove the panel itself: its program, its service, its configuration and its data. Everything you set up through the panel stays and keeps running:

- the web server, PHP, MariaDB, Redis, Docker and the other components from the **Software** page;
- your websites in `/var/www`, their Linux users and their databases;
- cron jobs, background programs, app websites and Docker containers;
- backups kept on the server in `/var/backups/panel`.

Without the panel, nothing renews your websites' certificates or runs your scheduled backups. The websites' certificates are stored in Nefo's data folder, `/var/lib/nefo/certs`. If you delete that folder while websites still use HTTPS, the web server can't load their certificates.

So decide first:

- **To remove everything**, remove your websites and software in the panel first (step 2), then remove Nefo.
- **To keep the websites running** without the panel, skip step 2, and keep `/var/lib/nefo/certs` when you delete the data (step 7). Let's Encrypt certificates last at most 90 days, so set up another way to renew them before they expire.

## 1. Keep copies of what you need

1. On the **Backups** page, make a fresh backup of each website and database you want to keep, with **Back up now**.
2. Copy the backups off the server. They are in `/var/backups/panel`, and in your remote storage if you set it up.

## 2. Remove websites and software in the panel

Do this while the panel still runs. The panel refuses to uninstall a component while something still uses it, so work in this order:

1. **Websites:** open each website, and on its **Overview** tab click **Delete**. Tick **Also delete its databases** if you don't need them. Untick **Back up the website first** if you already kept a backup (step 1). This deletes the website's files, its Linux user and its certificate.
2. **Databases:** delete the databases that are left.
3. **Docker:** delete the containers and Compose projects you created.
4. **Cron jobs** and **Background programs:** delete the ones you created.
5. **Software:** click **Uninstall** on each component you no longer need, such as Docker, MariaDB, Redis and the web server. For MariaDB and Docker, the data folder (`/var/lib/mysql` for MariaDB, `/var/lib/docker` for Docker) is kept unless you tick the box to delete it too.

The last PHP version stays, because the web server setup needs one. If you don't need it, remove it with apt after you have removed Nefo.

## 3. Note the panel's port

You need the port to close it in the firewall later:

```sh
nefo info --port
```

## 4. Unlock the websites' files {#unlock-the-websites-files}

If you locked the files of some websites (**Websites** → (site) → **File lock**, see [Country blocking and file lock](./website-protection#lock-a-website-s-files)), their files belong to root while they are locked. Give them back to the websites before you remove the panel, or they stay root's:

```sh
systemctl stop nefo
nefo unlock-files
```

The command lists each website it unlocked:

```
Unlocked the files of example.com
Unlocked the files of blog.example.com
```

Every file and folder gets back exactly the owner, group and permissions it had when it was locked. Files that root put into a website meanwhile stay root's. The command also removes the websites' lock groups (`panel-lock-<uid>`). It reads the records in `/var/lib/nefo/filelock`, so run it before you delete the panel's data (step 7). Running it again does nothing: it prints `No website's files are locked.`

## 5. Stop and remove the service and the program

```sh
systemctl disable --now nefo
rm /etc/systemd/system/nefo.service
systemctl daemon-reload
rm /usr/local/bin/nefo
```

## 6. Close the panel's port

If ufw is on, delete the rules for the panel's port. The installer added them with the comment `Nefo`.

1. List the rules with their numbers:

   ```sh
   ufw status numbered
   ```

2. Delete the rule for the panel's port by its number, for example:

   ```sh
   ufw delete 3
   ```

3. ufw usually has a second rule for IPv6, marked `(v6)`. The numbers change after each deletion, so list the rules again and delete that one too.

If you use firewalld instead, with port 28443 as the example:

```sh
firewall-cmd --permanent --remove-port=28443/tcp
firewall-cmd --reload
```

Also remove the port from your provider's cloud firewall.

## 7. Delete the configuration and the data

::: danger This can't be undone
This deletes the panel's account, its settings, the list of websites, the backup schedules, the activity log, the panel's own certificate and the certificates of your websites. Make sure you kept what you need (step 1).
:::

To delete everything:

```sh
rm -rf /etc/nefo /var/lib/nefo
```

If your websites stay (see above), keep their certificates and delete the rest:

```sh
rm -rf /etc/nefo
find /var/lib/nefo -mindepth 1 -maxdepth 1 ! -name certs -exec rm -rf {} +
```

## 8. Optional: remove what Nefo left elsewhere

While you used it, Nefo also created files outside its own folders. Remove only what you no longer need:

| Path | What it is |
|---|---|
| `/var/backups/panel` | Backups kept on this server |
| `/etc/cron.d/panel` and `/var/log/panel-cron` | Cron jobs created in the panel, and their logs. The jobs keep running until you delete the file. |
| `/var/lib/panel-trash` | Files deleted in the file manager, kept for 30 days |
| `/var/lib/panel-quarantine` | Files moved aside by the malware scanner |
| `/var/lib/panel-geoip` | The country database for country blocking |
| `/var/backups/nefo-postgresql` | The dumps made before a PostgreSQL upgrade |
| `/var/lib/panel-acme` | The folder that answers Let's Encrypt's checks. Remove it only once no web server uses it. |
| `/etc/ssh/sshd_config.d/99-panel-sftp.conf` | SSH rules for the websites' accounts and SFTP |
| `/etc/fail2ban/jail.d/panel.conf` and `/etc/fail2ban/filter.d/panel.conf` | fail2ban rules for SSH and the panel, if you installed fail2ban from the panel |
| `/etc/ssh/sshd_config.d/00-panel.conf` | The SSH settings you saved in the panel |

::: warning SSH settings
If you changed the SSH port or turned off password sign-in in the panel, those settings are in `/etc/ssh/sshd_config.d/00-panel.conf`. Deleting the file changes how you sign in over SSH once SSH restarts. Keep the file unless you know you want the default settings back, and test a new SSH session before you close the current one.
:::
