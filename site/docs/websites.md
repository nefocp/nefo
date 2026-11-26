---
title: Websites
description: Choose a web server, create PHP, static and app websites, and manage their domains, PHP settings, access options, SFTP, traffic and logs.
---

# Websites

This page covers the web server Nefo runs, how to create a website, and every setting a website has: domains, PHP, rewrite rules, access options, SFTP, traffic statistics, logs and deletion. Certificates have their own page: [SSL certificates](./ssl).

## Choose the web server

A server runs one web server at a time, and Nefo sets every website up for it. Before you add the first website, the **Websites** page asks you to pick one:

| Web server | Choose it when | Memory when idle |
|---|---|---|
| Nginx | Your websites don't need `.htaccess` files. It is the lightest choice. | about 15 MB |
| Apache | Your sites rely on `.htaccess` files, for example WordPress plugins or sites moved from shared hosting. | about 40 MB |
| OpenLiteSpeed | You want the rewrite rules of `.htaccess` files and the LiteSpeed Cache plugin for WordPress. It also answers HTTP/3. | about 26 MB |

To install it:

1. Open **Websites**.
2. Under **Install the web server**, select Nginx, Apache or OpenLiteSpeed.
3. Click **Install Nginx** (the button names the server you selected).

Nefo installs the web server and your distribution's PHP version with PHP-FPM, then opens ports 80 and 443 in the firewall (ufw), plus 443/udp for HTTP/3 on OpenLiteSpeed. This takes one to three minutes. OpenLiteSpeed comes from LiteSpeed's own package repository, and its WebAdmin console stays switched off.

All three web servers run PHP in a separate PHP-FPM pool for each website. Only Nginx can limit requests per visitor; the other settings work the same on all three.

### Switch to another web server

Nefo can't move existing websites to another web server, because each site's configuration is written for the installed one. To switch:

1. Delete every website (see [Delete a website](#delete-a-website)). Back them up first if you need them: see [Backups](./backups).
2. Open **Software** and find the **Nginx, Apache or OpenLiteSpeed** card.
3. Click **Uninstall** on the current web server, type its name (`nginx`, `apache` or `openlitespeed`) and confirm.
4. Click **Install** on the web server you want.

Uninstalling keeps the website folders in `/var/www`, the certificates and the Let's Encrypt account. PHP stays installed. Adminer is removed because it needs the web server, and the firewall rules Nefo added for websites are closed.

Nefo also refuses to install a web server while another one is installed on the server, even one you installed yourself: both need ports 80 and 443. The **Software** page then shows it as "installed, not used by the panel". Remove it first.

## Create a website

1. Point the domain's DNS A record to your server's IP address, for example `203.0.113.10`.
2. Open **Websites** and click **Add website**.
3. In **Domain**, type the name without `http://` or `www.`, for example `example.com`.
4. Leave **Also answer on www.example.com** ticked if the site should also answer on `www`. Nefo ticks it for you when the domain has two parts, such as `example.com`.
5. Choose the **Type** (see the table below).
6. Click **Create website**.

Nefo then shows "example.com is ready. Upload your files to /var/www/example.com/public." A placeholder page answers until you replace it.

| Type | What the website does |
|---|---|
| **PHP 8.3** (one entry per installed PHP version) | Serves your files and runs `.php` files with that PHP version. |
| **Static (HTML only)** | Serves HTML, CSS, JavaScript and other files. No PHP. |
| **App (Node.js, Python…) behind a proxy** | Forwards every request to a program that listens on a port of this server. |

A website can't change type later: a static site can't become a PHP site, and an app site can't become a PHP or static site, or the other way round. Create a new website instead. To install WordPress or another application, see the [App store](./apps). Docker apps get their own website from there.

Nefo refuses a domain that is not valid (write international names in punycode, `xn--…`), a name another website already uses, and a domain whose folder already exists in `/var/www`.

### App websites (Node.js, Python)

For the **App (Node.js, Python…) behind a proxy** type, the form asks two more things:

- **Port the app listens on (127.0.0.1)**: a port from 1024 to 65535. Nefo refuses the panel's own port, 3306 (MariaDB), 6379 (Redis), 7809 (the Git webhook listener) and a port another app website uses.
- **Start command (optional)**: one line of at most 1000 characters, such as `node server.js`. Nefo runs it in the website's `app` folder as the website's Linux user, with `PORT`, `HOST=127.0.0.1` and `NODE_ENV=production` set. It runs as the systemd service `panel-app-<user>.service` and restarts 3 seconds after it stops. Leave the command empty if the program is started another way, for example in Docker.

Put your code in `/var/www/example.com/app`, over [SFTP](#sftp-access) or with [Git deploy](./git-deploy), which deploys an app website into this folder. Then, in the website's settings:

1. On the **Runtime** tab, choose the **Runtime**: **Server default (node in /usr/local/bin)**, a Node.js version you installed on the **Software** page, or **Python (app/.venv)**, which needs Python installed on the **Software** page. The same tab changes the **Start command (optional)**. Click **Save**.
2. On the **Overview** tab, under **Application program**, click **Install dependencies**. Nefo runs `npm ci` (or `npm install` without a `package-lock.json`) for a `package.json`. For Python it creates the virtual environment `app/.venv` and installs `requirements.txt` with pip. It runs as the website's user, for up to 20 minutes.
3. Click **Restart** (or **Start**) to run the app with its dependencies.

The **Application program** card appears when the website has a start command. It also has **Stop** and **App logs**. On the **Reverse proxy** tab you can change the port. The web server forwards requests to `127.0.0.1` on that port, with the visitor's address in `X-Real-IP` and `X-Forwarded-For`, and WebSocket connections work. Nefo only forwards to a port on this server; it doesn't proxy to another server.

## Open a website's settings

Click the domain in the list, or **Settings** at the end of its row. The settings open in a panel with tabs in four groups:

| Group | Tabs |
|---|---|
| **Basics** | **Overview**, **Domains**, **PHP** (PHP sites), **Runtime** (app sites) |
| **Security** | **SSL / HTTPS**, **Web firewall (WAF)**, **Block access**, **Hotlink protection**, **Malware scan** |
| **Configuration** | **Rewrite** (PHP and static sites), **Page cache** (PHP sites on Nginx and OpenLiteSpeed), **Nginx configuration** (named after your web server), **Redirect**, **Reverse proxy** (app sites), **404 page** |
| **Operations** | **WordPress** (WordPress sites), **Git deploy**, **SFTP**, **Backups**, **Monitoring**, **Traffic**, **Logs**, **Expiry and suspension** |

The **Overview** tab shows the website's **Folder** (with **Open in Files**), its **Linux user**, domains, PHP version or runtime, web server, HTTPS and firewall state. Click **Add a note** to write one line (up to 200 characters) that shows under the domain in the list, such as a client name.

## The website's Linux user and folders

Each website gets its own Linux user, so one hacked site can't read or change the others. The name comes from the domain: `example.com` becomes `example_com`, and `blog.example.com` becomes `blog_example_com` (with `_2`, `_3`… if the name is taken). The user has no password and no shell, and it can't sign in over SSH unless you turn on [SFTP](#sftp-access). The website's PHP and its app run as this user, and PHP can only open files inside the website's folder.

| Path | What it holds |
|---|---|
| `/var/www/example.com/public` | The web folder (document root). Your website's files go here. |
| `/var/www/example.com/logs` | `php-error.log`, the website's PHP errors. |
| `/var/www/example.com/tmp` | PHP's temporary files, uploads in progress and sessions. |
| `/var/www/example.com/app` | App websites only: the program's code. |
| `/var/log/nginx/example.com.access.log` and `.error.log` | The web server's logs for the site (`/var/log/apache2` on Apache, `/var/log/openlitespeed` on OpenLiteSpeed). |
| `/var/lib/nefo/certs/example.com` | The website's certificate. |
| `/var/cache/nginx/nefo/example.com` | Nginx only: the [page cache](#page-cache), when it is on. |

The web server and PHP-FPM files that Nefo writes start with "Managed by Nefo. Do not edit". Nefo rewrites them, so make your changes in the panel instead.

## Domains and aliases

On the **Domains** tab, **Extra domain names** lists the other names that open the same website, one per line, such as `www.example.com`. A website can have up to 20. Each name must point to this server and can belong to only one website.

A wildcard such as `*.example.com` answers for every subdomain. Its certificate needs DNS validation through Cloudflare: see [SSL certificates](./ssl).

If the domain's DNS is at Cloudflare, **Point DNS here…** on the same tab creates or updates the A (and AAAA) records of all these names, after a preview: see [DNS (Cloudflare)](./dns#point-a-website-to-this-server).

If the website has HTTPS and you change its names, Nefo says "The domain names changed. Get a new certificate so HTTPS covers all of them." Click **Renew certificate**.

## PHP version and settings

The **PHP** tab of a PHP website has these settings:

| Setting | Default | Allowed |
|---|---|---|
| **PHP version** | the version chosen at creation | any installed PHP version |
| **Memory limit (MB)** | 256 | 32 to 4096 |
| **Largest upload (MB)** | 64 | 1 to 4096 (the web server's request size limit follows it) |
| **Longest script run (seconds)** | 300 | 10 to 3600 |
| **PHP workers** | 5 | 1 to 200 |
| **Worker mode** | **Start on demand (saves memory)** | or **Keep some ready (faster for busy sites)** |

**PHP workers** is how many requests the site's PHP handles at the same time. Each worker can use up to the memory limit, so workers × memory should fit in the server's RAM.

For safety, Nefo disables `exec`, `passthru`, `shell_exec`, `system`, `proc_open`, `popen` and `pcntl_exec` on every website, because they let a hacked site run programs on the server. Under **Allow functions that are disabled for safety**, tick one only if an application really needs it.

Click **Save**. If PHP-FPM or the web server rejects the result, Nefo puts the previous settings back. To install other PHP versions, their extensions or change `php.ini`, see [Software](./software).

## Rewrite rules and your own directives

On the **Rewrite** tab, **Rewrite rules** offers ready-made rules for common applications: **Default (front controller: index.php or index.html)**, WordPress, Laravel, Symfony, CodeIgniter 4, Drupal, Joomla, OpenCart, Nextcloud and Moodle for PHP sites, and Single-page app and Static site (clean URLs) for any site. The tab shows the rules of the preset you pick. On Nginx a preset replaces the default `location /` block. On Apache the site's own `.htaccess` file may replace the preset's front controller, and on OpenLiteSpeed the front controller runs after the site's `.htaccess` rules. On both, the preset's protective rules can't be undone by `.htaccess` files.

**Static site (clean URLs)** serves pages without their `.html` ending, as site generators such as VitePress (with `cleanUrls`) link to them: `/docs/install` serves `docs/install.html`, and `/docs/` serves `docs/index.html`. Other files are served as they are, and a page that doesn't exist still gets a 404.

The next tab is named after your web server, for example **Nginx configuration**. In **Nginx directives** (or **Apache directives**, **OpenLiteSpeed directives**) you can add your own directives, up to 32 KB, in the web server's syntax:

- Nginx adds them inside the site's `server` block, before the rewrite rules.
- Apache adds them at the end of the site's `VirtualHost` block.
- OpenLiteSpeed adds them at the end of the site's `virtualHost` block.

The web server checks the new configuration before anything changes. If it refuses, Nefo shows the error and the website keeps working as before. Click **Show the full configuration file** to see what Nefo generates.

About `.htaccess` files: Apache reads them. OpenLiteSpeed uses only their rewrite rules, which Nefo copies into the site's configuration within seconds of a change. Nginx ignores them.

## Page cache

On Nginx, the **Page cache** tab of a PHP website turns on Nginx's FastCGI cache for it: Nginx keeps each page that PHP makes and serves it again without running PHP or asking the database. Turn on the **Page cache** switch.

- A page is kept for 10 minutes, a missing page (404) for 1 minute. If PHP fails or stops answering, Nginx serves the last copy it still has.
- Nginx asks PHP every time for requests other than GET and HEAD (forms), addresses with a `?`, visitors with a sign-in or cart cookie (WordPress `wordpress_logged_in_`, `wp-postpass_`, `comment_author_`, WooCommerce `woocommerce_items_in_cart`, `wp_woocommerce_session_`, and PHP's own `PHPSESSID`), and the pages under `/wp-admin/`, `/wp-login.php`, `/xmlrpc.php`, `/wp-json/`, `/wp-cron.php`, `/cart/`, `/checkout/` and `/my-account/`. Pages that PHP marks as private or sends with a cookie are not kept either.
- Each website has its own cache of up to 256 MB in `/var/cache/nginx/nefo/example.com`. A page nobody asked for in an hour is dropped.

After you change a page, click **Purge cache**: every visitor gets the new version at once. Otherwise visitors who are not signed in may see the old page for up to 10 minutes. Turning the cache off, or deleting the website, deletes its folder.

To check that it works, open a page twice in a private window, or run `curl -sI https://example.com/` twice: the `X-Cache-Status` header says `MISS`, then `HIT` (`BYPASS` for the requests above). Static files have no such header: Nginx serves them directly anyway.

On OpenLiteSpeed, the tab says that LSCache, its own page cache, is built in: for WordPress install the LiteSpeed Cache plugin, other applications turn it on with cache rules in `.htaccess`. Apache has no page cache in Nefo, and no tab.

## Suspend, redirect or password-protect a website

**Suspend.** Turn off the switch in the **Running** column of the list, or **Website running** on the **Overview** tab. Visitors then get HTTP 503 with "This website is temporarily unavailable." The files and settings stay, and the certificate keeps renewing. To suspend several sites, tick them, choose **Suspend** under **Bulk action** and click **Apply**; **Run again** turns them back on.

**Redirect.** On the **Redirect** tab, tick **Redirect every request to another address** and enter the **Address**, such as `https://example.org`. It is a permanent redirect (301) that keeps the path: `/page` goes to the new address followed by `/page`. A redirect only works while the website runs.

**Password.** On the **Block access** tab, under **Password protection**, tick **Protect with a password** and enter a **User name** and a **Password** of 8 to 72 characters. The browser then asks for them before it shows the site. Leave the password empty later to keep the current one. Let's Encrypt validation and Git webhooks still reach the site.

The same tab blocks IP addresses and, on Nginx, limits requests per visitor: see [Security](./security).

## Custom 404 page and hotlink protection

**404 page.** On the **404 page** tab, tick **Use my own 404 page** and upload your page as `public/404.html`. Pages that your application handles itself, such as WordPress or Laravel pages, keep their own 404 page.

**Hotlink protection.** On the **Hotlink protection** tab, tick **Hotlink protection for images and media**. Other websites then get HTTP 403 when they embed your `jpg`, `jpeg`, `png`, `gif`, `webp`, `avif`, `svg`, `ico`, `bmp`, `mp4`, `webm`, `mp3`, `ogg` and `pdf` files. Your own site, the search engines (Google, Bing, DuckDuckGo and Yahoo) and visitors who open a file directly always get it. List up to 20 other sites under **Other sites allowed to show them**, one per line.

## Expiry date

On the **Expiry and suspension** tab, set an **Expiry date** and click **Save**. The website works until the end of that day (server time); after it, Nefo suspends the site automatically. It checks every hour. Click **Never** to remove the date. In the last 7 days, the list shows "expires" and the date under the domain. This is handy for client sites paid until a date.

The same tab has **Suspend this website** to suspend the site now.

## SFTP access

SFTP lets you or a client upload files with FileZilla, WinSCP or any SFTP app. The files then belong to the website's user, as they should. It needs the OpenSSH server installed.

1. Open the website's **SFTP** tab and tick **Allow SFTP sign-in for this website**.
2. Choose how the account signs in:
   - **Generate a new strong password**
   - **Choose a password (at least 12 characters)**, up to 128 characters
   - **Keep the current password** (when one is set)
   - **No password: sign in with SSH keys only**
3. Optional: paste **SSH public keys (optional)**, one per line as in `authorized_keys`, up to 50. Options such as `command=` are removed.
4. Click **Save**. A generated password is shown once: copy it now.
5. Connect with the **Host**, **Port**, **Username** and **Folder** shown. The host is the address you opened the panel with, and the port is the server's SSH port. The tab also shows the whole address, for example `sftp://example_com@panel.example.com/var/www/example.com`.

The account is locked in the website's folder. It sees only `/var/www/example.com`, at the same path as in the panel and in PHP, and it can't run commands or forward ports. The server's password policy may refuse a password you choose; Nefo then shows why.

Turning SFTP off removes the password and the keys and ends open sessions.

## Traffic statistics

The **Traffic** tab reads the web server's access logs for the website. Choose the **Period**: **Today**, **7 days**, **14 days** or **30 days**. It shows:

- **Requests**, **Visitors** (different IP addresses), **Bandwidth**, **Error rate** (4xx and 5xx) and **Bots**
- requests and visitors per hour or per day
- **Top pages**, **Top IP addresses**, **Referrers**, **Browsers** and **Bots and scripts**

Certificate checks and the panel's own [monitoring](#monitoring) checks are not counted. The server keeps about 14 days of logs, so older traffic is not known. For very large logs, only the newest 5,000,000 lines are read. The **Visits, 7 days** column of the website list comes from the same logs.

## Logs

The **Logs** tab shows the newest lines of:

- **Access log (Nginx)** and **Error log (Nginx)** (named after your web server)
- **PHP errors**, for PHP websites (`/var/www/example.com/logs/php-error.log`)
- **App logs**, for app and Docker websites

## Monitoring

The **Monitoring** tab checks every few minutes that the website answers, and sends you an alert when it doesn't. Turn on **Monitor this website**; the first check runs at once.

The panel asks for the website's public address, with `https://` when it has a certificate (otherwise `http://`), through DNS like any visitor. So a wrong DNS record, a Cloudflare error page or an expired certificate shows up too. Redirects to the website's own domains (such as `http://` to `https://`, or to `www.`) are followed. A redirect to another domain counts as an answer.

| Setting | What it does | Default |
|---|---|---|
| **Check** | how often: every minute, or every 2, 5 or 10 minutes | every 5 minutes |
| **Path** | the page to ask for, such as `/health`; a query string is allowed | `/` |
| **Text the page must contain** | optional; upper and lower case don't matter | empty |
| **Count 4xx answers, such as 404, as down** | a 4xx answer fails the check | off |

A check fails when:

- no answer comes within 10 seconds, the domain doesn't resolve, the connection is refused, or the certificate has expired, isn't trusted or is for another domain
- the answer is an HTTP error 500 or higher. A 4xx answer counts as up, unless you count 4xx answers as down
- the text the page must contain isn't in the first 256 KB of the page

The website is **Down** after 2 failed checks in a row, and up again after one successful check. The panel then sends an alert to every channel in **Settings** → **Alerts** (see [Alerts](./mail-alerts#alerts)):

- when it goes down, with the reason, such as "example.com is down (HTTP 502, 2 checks in a row)", and again every 6 hours while it stays down
- when it's up again, with how long it was down
- when the certificate it serves ends in less than 14 days, at most once a day

The tab shows the state and the last check, the uptime over 24 hours and 7 days, the average response time, the response times of the last 24 hours as a chart, and the last failures with their reasons. The response time runs until the answer's headers arrive, connecting included. In the website list, a monitored website that is down shows **Down** in red in its **Running** column, and the bell lists it.

- The checks send the user agent `Nefo-Monitor/1.0`. They aren't counted in the **Traffic** tab.
- The results are kept for 7 days. Deleting the website deletes them.
- A suspended website isn't checked.
- The checks run on this server. They can't report that the whole server is down, or a problem on the network between your visitors and the server: for that, add an outside monitoring service.

## Web firewall, malware scan and backups

- **Web firewall (WAF)**: ModSecurity with the OWASP Core Rule Set for this site: **Off**, **Detect only: log attacks, block nothing** or **Block attacks**. Install the firewall on the **Software** page first. See [Security](./security), which also covers the **Malware scan** tab.
- **Backups**: **Back up now**, or schedule a backup **Every day at 03:00**. See [Backups](./backups).

## Clone a website

**Clone** copies a PHP or static website to a new domain: to start a new website from an existing one, or to try changes on a copy.

1. Open the website's **Overview** tab and click **Clone…** under **Clone this website**.
2. Type the **Domain of the copy**.
3. Leave **Copy the databases linked to this website** ticked to copy them too.
4. Click **Clone**. The website keeps running during the copy, which runs as a task with a log.
5. Point the new domain's DNS to this server, then turn on HTTPS on its **SSL / HTTPS** tab. The copy starts on `http://`.

The copy gets:

- a new website of the same type (the same PHP version, or static) with its own Linux user, the same app, PHP settings, rewrite rules and own directives. In your directives, the old website's folder is replaced with the new one;
- a copy of the website's folder `/var/www/example.com`, owned by the new user, without `logs`, `tmp` and the Git deploy key (`.ssh`). Symbolic links are left out, and in `public/.user.ini` the old folder is replaced with the new one;
- for each linked database, a new database and user with a copy of its data, linked to the new website.

When the copy is ready, Nefo shows the connection details of the new databases. The app's own configuration files (such as `.env` or `config.php`) still name the old database and user: change them to the new ones. Files that name the old folder, such as a cache of the app's settings, still do too.

A WordPress site is copied like a [staging copy](./wordpress#make-a-staging-copy): its database is always copied, `wp-config.php` is pointed at the copy and the site's address is replaced everywhere, so the copy works right away. Search engines are asked not to index it; if the copy is a website of its own, change that in WordPress under **Settings → Reading**. Unlike a staging copy, a clone can't be pushed back to the website it came from.

Not copied: aliases, the certificate, password protection, redirects and firewall settings, cron jobs, background programs, Git deploy settings, SFTP access and backup schedules.

Nefo can't clone app websites (Node.js, Python or Docker) yet. If a step fails, Nefo removes everything it created for the copy.

## Delete a website

1. Open the website's **Overview** tab.
2. Under **Delete this website**, click **Delete**.
3. Leave **Also delete the databases linked to this website** ticked to delete them too, or untick it to keep them.
4. Type the domain to confirm and click **Delete**.

::: danger
Deleting a website can't be undone. Make a backup first if you may need the files again.
:::

Nefo removes:

- the website folder `/var/www/example.com` with all its files, including a Git repository and its deploy key
- the Linux user and group, and the SFTP access
- the web server configuration, the password file and your own directives
- the PHP-FPM pool, the app service or the Docker app with its containers, data and unused images
- the certificate, the page cache and the web server logs of the site
- the website's background programs, the cron jobs that run as its user, its backup schedules and its Git deploy settings
- the website's trash in **Files** and its files quarantined by the malware scanner
- the linked databases, if you left the box ticked

Nefo keeps the backups you already made: they stay on the **Backups** page. To restore one, create a website with the same domain first.

While a Git deploy of the website is running, Nefo refuses to delete it ("Another task is running on the server. Wait until it has finished."). If a cleanup step fails, the website is removed from the panel anyway and Nefo lists the steps that failed under "Some cleanup steps failed:".
