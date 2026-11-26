---
title: Troubleshooting
description: Fixes for the most common problems with Nefo, from a panel you can't reach to certificate errors and websites that show 502 or 503.
---

# Troubleshooting

This page covers the problems people run into most often, with the fix for each. Most fixes need SSH access to the server as root. If SSH doesn't work either, use the web console in your provider's control panel.

## You can't open the panel

### You forgot the address

Run this on the server:

```sh
nefo info
```

The output shows the full URL, with the port and the secret path: with the panel's domain if you gave it one, and with the server's first IP address. If that address is a private one, add `--host` and your public IP address, such as `nefo info --host 203.0.113.10`.

### The page shows 404

The panel answers only under its secret path, such as `https://203.0.113.10:28443/k3x9p2qa7m1z/`. Every other address returns `404`, including the address without the path. Check the path with `nefo info`.

Also make sure the address starts with `https://`. With `http://`, the panel answers with an error instead of the sign-in page.

### The page doesn't load at all

When the browser can't connect, or waits and then gives up, the panel isn't running or something blocks the connection.

1. **Check that the panel runs:**

   ```sh
   systemctl status nefo
   ```

   If it isn't `active (running)`, see [The panel doesn't start](#the-panel-doesn-t-start).

2. **Check your provider's cloud firewall.** It must allow the panel's TCP port. Find the port with `nefo info --port`.

3. **Check that your IP address is still allowed.** If you limited the panel's port to your own IP address, in ufw or in the cloud firewall, and your IP address has changed (common on home and mobile connections), allow the new one. In ufw, with your new address and the panel's port:

   ```sh
   ufw allow from 198.51.100.7 to any port 28443 proto tcp
   ```

4. **Check whether fail2ban blocked you.** If you installed fail2ban from the panel, it blocks an address for one hour after 10 failed panel sign-ins within 10 minutes (5 for SSH). Unblock your address:

   ```sh
   fail2ban-client set panel unbanip 198.51.100.7
   ```

   For SSH, use the jail `sshd` instead of `panel`.

### "Too many failed attempts"

After 5 failed sign-ins in 15 minutes, the panel refuses further attempts from your address for 15 minutes, and shows "Too many failed attempts. Try again in … seconds." Wrong two-factor codes count too. Wait until the time is up. The lock is kept in memory, so `systemctl restart nefo` also clears it.

### The panel doesn't start

Read the reason in the log:

```sh
journalctl -u nefo -n 50 --no-pager
```

| The log says | What to do |
|---|---|
| `address already in use` | Another program uses the panel's port. Stop that program, or change the port in `/etc/nefo/config.json` (see [Command line](./cli#the-configuration-file)). |
| An error about `/etc/nefo/config.json` | The configuration file has a mistake, for example after you edited it. Fix the line that the message names. |
| `database schema version … is newer than this panel supports` | An older version of the program runs with data from a newer one. Install the latest release again (see [Update](./update#if-an-update-fails)). |

After the fix, run `systemctl restart nefo`.

## You forgot the password

Set a new random password on the server:

```sh
nefo reset-password
```

It prints the new password and signs the account out everywhere. Sign in with it, then choose your own password in **Account**.

## Lost your 2FA phone

If you can't get codes from your authenticator app any more, turn two-factor authentication off from the server. This also sets a new password:

```sh
nefo reset-password --disable-2fa
```

Sign in with the new password, then set up two-factor authentication again in **Account**, with your new phone.

### Codes are always "not valid"

The code depends on the time. The panel accepts the current code and the ones right before and after it, and each code only once. If every code fails, check that the server's clock is right:

```sh
timedatectl
```

`System clock synchronized` should say `yes`. Also check the time on your phone.

## The browser warns about the certificate

Until the panel has its own certificate, it uses a self-signed one, and every browser warns about it. This is expected. Give the panel a domain and a free certificate to make the warning go away (see [First steps](./first-steps#give-the-panel-its-own-domain)).

If the panel has a certificate and the browser still warns:

- You opened the panel by its IP address. The certificate is for the domain: use `https://panel.example.com:28443/k3x9p2qa7m1z/`.
- The certificate has expired because it couldn't be renewed. Check **Settings → Certificate**, and click **Get a new certificate**. The causes are the same as in the next section.

## A certificate request fails

Before it asks Let's Encrypt, the panel checks that each domain reaches this server: it puts a test file on the server and fetches it through the domain over HTTP, on port 80. If that fails, it requests nothing and lists each problem under "these names do not reach this server yet, so no certificate was requested".

| The message says | Cause and fix |
|---|---|
| `no DNS record. Add an A record that points to this server's IP address.` | The domain has no address yet. Add the A record at your DNS provider. A new record can take a while to work everywhere. |
| `no answer over HTTP on port 80 (…). Check the DNS record and the firewall.` | Port 80 is closed, or the domain points to another server. Open port 80 in ufw and in your provider's cloud firewall, and check that the web server runs. |
| `answered HTTP …, but not from this server. Check that the DNS record points here.` | Something else answered for the domain: the record points to another server, or a proxy or CDN sits in front, such as Cloudflare's proxy (the orange cloud). |

Each line starts with the domain and the addresses it points to. If those aren't your server's addresses, fix the DNS record first. If an IPv6 address is listed that isn't your server's, fix or remove the AAAA record.

**Behind Cloudflare's proxy:**

- For a website, either set the record to "DNS only" while the certificate is requested, or tick **Validate through DNS (Cloudflare)** on the website's **SSL / HTTPS** tab. DNS validation needs a Cloudflare API token, which you add in **Settings → Cloudflare**. Wildcard names (`*.example.com`) always need DNS validation.
- For the panel's own domain, the record must point straight at the server, without the proxy.

**For the panel's own domain,** the **Certificate** tab says "This needs the web server" until you install one, because the web server answers Let's Encrypt on port 80. Install it on the **Websites** page first.

If the check passes but Let's Encrypt still refuses, the task log shows its reason. Let's Encrypt limits how many failed attempts you can make, so fix the cause before you try again.

When an automatic renewal fails, the panel shows "Certificate renewal failed for example.com" and tries again every 12 hours. The certificate keeps working until it expires.

## A website shows an error

| What visitors see | Cause | What to do |
|---|---|---|
| The text "This website is temporarily unavailable." (HTTP 503) | The website is suspended, by hand or because its expiry date has passed. | Open the website's **Expiry and suspension** tab. Untick **Suspend this website**, and clear or change the **Expiry date**. If the date stays in the past, the panel suspends the website again within the hour. |
| A "502 Bad Gateway" or "503 Service Unavailable" page from the web server | The program behind the website doesn't answer: PHP-FPM for a PHP website, or the app or container for an app website. Nginx shows 502, Apache and OpenLiteSpeed may show 503. | For PHP, open **Services** and start or restart the PHP-FPM service of the website's PHP version, such as `php8.3-fpm`. For an app, the website shows "App stopped": read the **App logs** on its **Logs** tab. |
| No answer for every website | The web server is stopped. The **Websites** page says that it "is not running". | Start the web server on the **Services** page. |
| HTTP 429 | The website's rate limit is on (Nginx only), and the visitor sent too many requests. | Raise the limit, or set it to 0 to turn it off, on the website's **Block access** tab. |
| HTTP 403 | The visitor's address is in the website's blocked addresses, or the web application firewall blocked the request. | Check **Blocked IP addresses** on the **Block access** tab, and the recent firewall events on the **Web firewall (WAF)** tab. |

For other errors, read the website's error log and PHP errors on its **Logs** tab.

## Where the logs are

| Log | On the server | In the panel |
|---|---|---|
| The panel | `journalctl -u nefo` | — |
| A website's access and error logs | `/var/log/nginx/`, `/var/log/apache2/` or `/var/log/openlitespeed/`, as `example.com.access.log` and `example.com.error.log` | Websites → (site) → **Logs** |
| PHP errors of a website | `/var/www/example.com/logs/php-error.log` | Websites → (site) → **Logs** → **PHP errors** |
| An app website | `journalctl -u panel-app-example_com` | Websites → (site) → **Logs** → **App logs** |
| Cron jobs | `/var/log/panel-cron/` | **Cron jobs** |
| Sign-ins and changes in the panel | — | **Account** → **Recent activity** |

When you report a problem, include the output of `nefo version` and the relevant log lines. Remove passwords and the panel's secret path first.
