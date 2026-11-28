---
title: Country blocking and file lock
description: Block or allow visitors by country on a website, with DB-IP's free country data, and lock a website's code read-only for its PHP, with an alert when PHP files change.
---

# Country blocking and file lock

Nefo has two protections per website, from v0.24.0. **Country blocking** refuses visitors from the countries you list, or lets in only those countries, with DB-IP's free country data, on Nginx, Apache and OpenLiteSpeed. The file lock makes the website's code read-only for its own PHP and SFTP account, except the folders it must write to, such as uploads.

Both are on the website's settings, in the **Security** group: **Websites** → (site) → **Country blocking** and **File lock**. Neither costs anything while it is off: no download, no extra process, nothing loaded into the web server. For the web application firewall, rate limits and the malware scanner, see [Security](./security#website-security).

## How do I block countries on a website? {#block-or-allow-countries}

1. Open **Websites**, click the website, and open the **Country blocking** tab.
2. Choose **Block the countries below** or **Let in only the countries below**.
3. Type a country's name (in the panel's language) or its two-letter code in the search field, and click it to add it. Remove one with the × on its chip.
4. Click **Save**.

The first time any website on the server uses country blocking, Nefo downloads the country database from db-ip.com (about 5 MB) before it saves, which takes a few seconds. To turn it off again, choose **Off: every country gets in** and click **Save**.

Country blocking works on every type of website, reverse proxy websites included. Each website has its own setting.

Blocked visitors get **403 Forbidden**. To show your own page instead, put a `403.html` file in the website's `public` folder and point the web server's 403 error page at it in **Websites** → (site) → **Nginx configuration** (Nginx: `error_page 403 /403.html;`; Apache: `ErrorDocument 403 /403.html`, which also works in `.htaccess`; OpenLiteSpeed: an `errorpage 403` block). Blocked visitors may load `/403.html` itself.

### What is never blocked

- Let's Encrypt's checks (`/.well-known/acme-challenge/`), so certificates keep renewing.
- The Git deploy webhook (`/.panel/deploy/…`). The panel checks its signature.
- `/403.html`.
- Private and other reserved addresses (`10.0.0.0/8`, `192.168.0.0/16`, loopback and so on).
- The server's own requests, such as WordPress calling itself for its scheduled tasks.

The panel itself is not a website: it has its own port and is never country-blocked. To limit who can reach the panel, use the firewall (see [Security](./security#firewall-ufw)).

### Behind Cloudflare

When a website's DNS record is proxied by Cloudflare, every request comes from Cloudflare's addresses.

- **Nginx and Apache**: for requests that come from Cloudflare's addresses, Nefo looks up the visitor's address that Cloudflare sends in the `CF-Connecting-IP` header. Requests from anywhere else are looked up by their own address, so a client can't fake the header. On Apache, the website's access log and PHP then also see the visitor's address for requests through Cloudflare.
- **OpenLiteSpeed**: for requests from Cloudflare's addresses, Nefo uses the country that Cloudflare sends in the `CF-IPCountry` header. Turn on **IP Geolocation** in Cloudflare (**Network** settings of the zone). Without it, visitors who come through Cloudflare are not blocked, and with **Let in only the countries below** they are let in.

Nefo keeps a built-in list of Cloudflare's addresses and refreshes it from Cloudflare each month, together with the country database.

### Limits per web server

| Web server | How it works | Limit |
|---|---|---|
| Nginx | Nginx's built-in `geo` module, loaded only while a website uses country blocking, with only the countries the websites list | None. Memory: about 2 to 5 MB for one country, about 40 MB for large lists (the United States, Germany and Japan together) |
| Apache | Apache checks the address ranges of the listed countries one by one on every request | At most 50,000 address ranges per website, so large countries are refused (the United States alone has about 270,000). China (15,000 ranges) takes about 2.5 MB |
| OpenLiteSpeed | OpenLiteSpeed's own GeoIP lookup in DB-IP's database file | None. Under 1 MB |

A list can hold up to 100 countries. On Apache the tab shows how many address ranges the website uses out of the limit. If a list is too large, Apache refuses it and the message names the countries that don't fit, the largest first, with their number of ranges. Leave them out, or use Nginx or OpenLiteSpeed, which handle every country.

### The country data

The data is DB-IP's free **IP to Country Lite** database, IP Geolocation by [DB-IP](https://db-ip.com), under the Creative Commons Attribution 4.0 licence. It covers IPv4 and IPv6.

- Nefo downloads it over HTTPS the first time a website uses country blocking.
- Before it replaces the old copy, it checks that the file is a complete database: its format, its size, and that it lists more than 200 countries.
- It refreshes it once a month (DB-IP publishes a new edition at the start of each month). The tab shows the edition in use.
- While no website uses country blocking, nothing is downloaded and the web server loads nothing.

The database is kept in `/var/lib/panel-geoip/`. It is readable by all users, since the web server reads it: it is public data.

## How do I make a website's files read-only? {#lock-a-website-s-files}

The **File lock** tab makes the website's code read-only for its PHP, its app and its SFTP account. A plugin with a security hole can then no longer change the site's files or add new ones, except in the folders you allow, such as uploads and caches. Other panels call this tamper protection.

1. Open **Websites**, click the website, and open the **File lock** tab.
2. Check the list under **Writable folders**. For WordPress, Joomla, Laravel, Drupal, Nextcloud, OpenCart, Moodle, Symfony and CodeIgniter sites, the panel fills in the application's folders; **Use the folders of WordPress** (or of the site's application) puts them back. Write one folder per line, relative to the website's folder, for example `public/wp-content/uploads`. `*` stands for one folder name: `public/sites/*/files`. `tmp` and `logs` always stay writable.
3. To be told when PHP files change anyway, tick **Alert me when PHP files change** (see [below](#alerts-when-php-files-change)).
4. Click **Lock the files**.

The tab then shows **Locked since** and the date. To change the writable folders or the alert later, edit them and click **Save**: the files are locked again with the new list. **Unlock the files** gives everything back to the website's user, with exactly the owner, group and permissions each file had before.

The **File lock** tab isn't shown for Docker apps, whose files belong to their containers, or for reverse proxy websites, which serve no files.

The preset folders:

| Application | Writable folders |
|---|---|
| WordPress | `public/wp-content/uploads`, `public/wp-content/cache`, `public/wp-content/upgrade` |
| Joomla | `public/images`, `public/media`, `public/cache`, `public/administrator/cache`, `public/administrator/logs`, `public/tmp` |
| Laravel | `storage`, `bootstrap/cache` |
| Drupal | `public/sites/*/files` |
| Nextcloud | `data`, `public/config` |
| OpenCart | `storage`, `public/image/cache`, `public/image/catalog` |
| Moodle | `moodledata` |
| Symfony | `var` |
| CodeIgniter | `writable` |

A writable folder that doesn't exist yet is created, for the website's user, when you lock the files, if the folder above it exists. The tab lists the ones it couldn't find.

### What changes while the files are locked

- The website's PHP, its app and its SFTP account can't change, add, rename or delete anything outside the writable folders, nor change permissions there. Uploads and caches keep working.
- WordPress can't update itself, install or update plugins and themes from its dashboard, or rewrite `wp-config.php` or `.htaccess`. Update it from the **WordPress** tab instead (see below).
- The site keeps working: the web server and PHP still read every file they could read before. Files only the website's user could read, such as `wp-config.php` (mode `0600`), stay unreadable for the web server.

How it works: the files and folders the website's user owned now belong to root, without write permission for anyone else. Files the website's group could read keep that group. Files only their owner could read get the website's own lock group (`panel-lock-<uid>`, whose only member is the website's user). No kernel module, no `chattr`. What the lock changed is recorded in `/var/lib/nefo/filelock/`, so unlocking restores it exactly, even after a crash.

### Panel operations on a locked website

| Operation | While locked |
|---|---|
| **WordPress** tab: **Update all**, maintenance mode, debug mode, automatic updates | The panel unlocks the files, makes the change and locks them again |
| Pushing a staging copy to this website | Unlocked for the push, then locked again |
| **Git deploy** (by hand or from the webhook) | Unlocked for the deploy, then locked again |
| Restoring a backup of this website (or of a folder in it) | Unlocked for the restore, then locked again |
| Malware scan: quarantining or restoring a file | Unlocked meanwhile, then locked again |
| Installing the app's dependencies (Node.js, Python) | Unlocked meanwhile, then locked again |
| **Files** (the file manager): editing, uploading, creating, renaming, deleting, extracting, changing permissions | Allowed inside the writable folders and `tmp`, `logs`; refused elsewhere, with a message that the website's files are locked |
| Installing an app (WordPress or another) into the website | Refused: unlock the files first |
| Importing a website into this (empty) website | Refused: unlock the files first |
| Cloning the website, a staging copy of it, backups | They work: they only read the files, also those only the website could read (such as `wp-config.php`). The copy belongs to its own website and is not locked |

After a push, a deploy or a restore, the new files are under the lock like the others; files that arrive in a writable folder stay the website's. Each operation that unlocks the files writes "Unlocking the files of example.com for this change" and "Locking the files of example.com again" in its task log. If locking again fails, the task says so and the panel tries again every hour. While an operation runs, the tab shows that the files are unlocked for it.

::: warning
The lock stops the website's own user, not root. Anything that runs as root, such as an SSH session as root or the panel's terminal, can still change the files. Before you remove the panel, run `nefo unlock-files` (see [Uninstall](./uninstall#unlock-the-websites-files)), or the locked websites' files stay owned by root.
:::

### Alerts when PHP files change

With **Alert me when PHP files change**, the panel takes a snapshot (a SHA-256 checksum of every PHP file of the website, writable folders included) when it locks the files, and once a day compares the PHP files with it. When files were changed, added (a PHP file in `uploads` is a classic sign of a break-in) or removed, it sends an alert through the channels set in **Settings** → **Alerts**, and the tab lists the files. It alerts again only when the list of differences changes.

If you made the changes yourself, click **Take a new snapshot**. Operations that unlock and lock the files again, such as a WordPress update, take a new snapshot themselves.

The daily check reads at most 200,000 PHP files or 2 GB per website. The tab says when it stopped at that limit.
