---
title: First steps
description: What to do after installing Nefo, from securing your account to your first website and backups.
---

# First steps

After you [install Nefo](./install), work through these steps in order. Each one is short and links to a page with the details.

## Sign in

1. Open the URL that the installer printed, such as `https://203.0.113.10:28443/k3x9p2qa7m1z/`.
2. Accept the browser's certificate warning. It goes away once the panel has its own certificate (see [below](#give-the-panel-its-own-domain)).
3. Enter **Username** `admin` and the password from the installer, then click **Sign in**.

If you lost the URL or the password, run `nefo info` or `nefo reset-password` on the server (see [Command line](./cli)).

## Set up the server

On a new server, the panel opens **Set up this server** right after you sign in. On one page, you choose what the server needs, and the panel installs it all in one go, one part after the other.

1. Choose a **Web server**: **Nginx** (recommended), **Apache** for websites that need `.htaccess` files, **OpenLiteSpeed**, or **None** for a server without websites, such as one that only runs Docker apps.
2. Tick one or more **PHP versions**. The web server always comes with your distribution's PHP version as well. Other versions come from the `packages.sury.org/php` repository, on Ubuntu and Debian, which the panel adds to the server. Old versions past their end of life are not offered here: install them from the **Software** page if an old website needs one.
3. Choose the **Database server**: **MariaDB** (recommended), **MySQL** where the Software page offers it, or **None**.
4. Tick the **Extras** you want: Redis, Node.js, Python, Docker, Adminer and the web firewall. ClamAV is offered on servers with 2 GB of RAM or more, and PostgreSQL and Memcached where the Software page offers them.
5. Check the summary at the bottom. It shows the server's CPUs, RAM and free disk, and roughly how much RAM and disk your choice adds. If the choice may not fit, a warning says so, but you can still install.
6. Click **Install**.

The page starts with a choice that suits the server's RAM: Nginx, the newest PHP version and MariaDB, plus Redis from about 1.5 GB of RAM. Docker and the other extras are never ticked for you.

The log shows each part as it installs, and the whole setup is one task. If a part fails, the setup stops there and tells you what is installed already. Fix the problem, then click **Install** again: the parts that are installed are skipped.

To set the server up yourself instead, click **Skip for now**. You can run the setup again at any time from **Software** → **Run the setup again**, or with [`nefo setup`](./cli#nefo-setup) on the server. You can also [set the server up while installing Nefo](./install#set-the-server-up-in-the-same-run).

The panel stops opening the setup after sign-in once it has finished or was skipped, and on a server that already has a web server or a website.

## Change the password

Replace the password from the installer with one you keep in a password manager.

1. In the left menu, open **Account**. You can also open it from the account menu at the top right: **Password and 2FA**.
2. Under **Change password**, fill in **Current password**, **New password** and **Repeat new password**. The new password needs 12 to 72 characters.
3. Click **Change password**.

The panel signs you out on every device. Sign in again with the new password.

## Turn on two-factor authentication

With two-factor authentication, signing in needs your password and a 6-digit code from an authenticator app, such as Google Authenticator, Authy or 2FAS. Until you turn it on, the dashboard and the notifications menu show "Two-factor sign-in is off".

1. Open **Account**.
2. Under **Two-factor authentication**, click **Set up**.
3. Scan the QR code with your authenticator app, or type in the **Setup key**.
4. Enter the 6-digit code that the app shows, and click **Turn on**.

From now on, the sign-in page asks for a **Two-factor code** after your password.

::: warning Keep a way back in
If you lose your phone, you can only turn two-factor authentication off from the server, with `nefo reset-password --disable-2fa` (see [Troubleshooting](./troubleshooting#lost-your-2fa-phone)). Keep your SSH access working, or save the setup key somewhere safe.
:::

## Install a web server and PHP

Websites need a web server. Nefo runs one per server. If the [setup](#set-up-the-server) installed one, skip this step.

1. Open **Websites**. On a new server it shows **Install the web server**.
2. Choose a web server:
   - **Nginx**: fast and light, the best choice unless your websites need `.htaccess` files.
   - **Apache**: reads `.htaccess` files, which many WordPress plugins and sites from shared hosting rely on. It uses a little more memory.
   - **OpenLiteSpeed**: reads the rewrite rules in `.htaccess` files and has LiteSpeed Cache built in. It comes from LiteSpeed's own package repository.
3. Click the install button, such as **Install Nginx**.

The panel installs the web server and your distribution's PHP version, and opens ports 80 and 443 if the firewall (ufw) is on. This takes one to three minutes, and the progress shows on the page.

To switch to another web server later, you must delete all websites first. Add more PHP versions, MariaDB and other components on the **Software** page. See [Software](./software).

## Give the panel its own domain

With its own domain, the panel gets a free Let's Encrypt certificate, and the browser warning goes away. You need a web server first, because it answers Let's Encrypt on port 80.

1. At your DNS provider, create an A record for a name such as `panel.example.com`, pointing to the server's IP address. It must point straight to the server, without a proxy or CDN in front (with Cloudflare, use "DNS only").
2. Make sure port 80 is open, also in your provider's cloud firewall.
3. In the panel, open **Settings → Certificate**.
4. In **Domain of the panel**, enter `panel.example.com`, and click **Get certificate**.

The panel first checks that the domain reaches this server, then asks Let's Encrypt for the certificate. It uses the new certificate within a minute, and shows the new address, such as `https://panel.example.com:28443/k3x9p2qa7m1z/`. Use that address from now on: the panel keeps the same port and secret path. With the IP address, the browser warns again, because the certificate is for the domain.

The certificate renews automatically. To go back to the self-signed certificate, click **Delete** on the same page.

If the request fails, see [Troubleshooting](./troubleshooting#a-certificate-request-fails).

## Create your first website

1. Point the domain's A record, such as `example.com`, to the server's IP address.
2. Open **Websites** and click **Add website**.
3. Enter the **Domain** without `http://` or `www.`. Keep **Also answer on www.example.com** ticked if you want both names.
4. Choose the **Type**: a PHP version, **Static (HTML only)**, or an app (Node.js or Python).
5. Click **Create website**.
6. Click **Turn on HTTPS** next to the website to get a free certificate.

Upload your files to `/var/www/example.com/public`, with the **Files** page or over SFTP. To install WordPress or another application instead, use the **App store**. See [Websites](./websites), [SSL certificates](./ssl) and [App store](./apps).

## Set up backups

1. Open **Backups**.
2. Under **Automatic backups**, click **Add schedule**.
3. Choose what to back up (a website, a database or a folder), how often, and the **Number of backups to keep**.
4. Click **Save schedule**.

A website backup contains its files and the databases linked to it. Backups are kept on the server in `/var/backups/panel`. Add **Remote storage** (S3-compatible or SFTP) so a copy survives if the server is lost. See [Backups](./backups).

## Turn on the firewall

If the dashboard says "The firewall is off", every port that a program opens is reachable from the internet.

1. Open **Firewall & SSH**.
2. Click **Turn on the firewall**.

The panel keeps SSH and the panel's own port open. If the page says that ufw is not installed, install it first with `apt install ufw`. See [Security](./security).
