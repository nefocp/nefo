---
title: Websites
description: Choose a web server, create PHP, static, app (Node.js, Python, Java) and reverse proxy websites, and manage their domains, PHP settings, access options, SFTP, traffic, logs, clones and deletion.
---

# Websites

A website in Nefo is a domain served by the server's one web server, Nginx, Apache or OpenLiteSpeed: a PHP website, a static website, an app (Node.js, Python or Java) behind a proxy, or a reverse proxy to another address. Each website gets its own Linux user, folder and optional SFTP account, and can be cloned, backed up or deleted from its settings.

This page covers the web server, creating a website, and every setting a website has: domains, PHP, rewrite rules, access options, SFTP, traffic statistics, logs, cloning and deletion. Certificates have their own page, [SSL certificates](./ssl), and so do [country blocking and the file lock](./website-protection).

## Which web server should I choose? {#choose-the-web-server}

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

Uninstalling keeps the website folders in `/var/www`, the certificates and the Let's Encrypt account. PHP stays installed. Adminer is removed because it needs the web server, and the firewall rules Nefo added for websites (ports 80 and 443) are deleted.

Nefo also refuses to install a web server while another one is installed on the server, even one you installed yourself: both need ports 80 and 443. The **Software** page then shows it as "installed, not used by the panel". Remove it first.

## Create a website

1. Point the domain's DNS A record to your server's IP address, for example `203.0.113.10`.
2. Open **Websites** and click **Add website**.
3. In **Domain**, type the name without `http://` or `www.`, for example `example.com`.
4. Leave **Also answer on www.example.com** ticked if the site should also answer on `www`. Nefo ticks it for you when the domain has two parts, such as `example.com`.
5. Choose the **Type** (see the table below).
6. Click **Create website**.

Nefo then shows "example.com is ready. Upload your files to /var/www/example.com/public." A welcome page answers until you replace it.

| Type | What the website does |
|---|---|
| **PHP 8.3** (one entry per installed PHP version) | Serves your files and runs `.php` files with that PHP version. |
| **Static (HTML only)** | Serves HTML, CSS, JavaScript and other files. No PHP. |
| **App (Node.js, Python, Java…) behind a proxy** | Forwards every request to a program that listens on a port of this server. |
| **Reverse proxy to another address** | Forwards every request to an address you give: another server, or a service on this server that Nefo doesn't run. See [Reverse proxy websites](#reverse-proxy-websites). |

A website can't change type later: a static site can't become a PHP site, an app site can't become a PHP or static site, and a reverse proxy website can't become another type either. Create a new website instead. To install WordPress or another application, see the [App store](./apps). Docker apps get their own website from there.

Nefo refuses a domain that is not valid (write international names in punycode, `xn--…`), a name another website already uses, and a domain whose folder already exists in `/var/www`.

### App websites (Node.js, Python, Java) {#app-websites-node-js-python}

For the **App (Node.js, Python, Java…) behind a proxy** type, the form asks two more things:

- **Port the app listens on (127.0.0.1)**: a port from 1024 to 65535. Nefo refuses the panel's own port, 3306 (MariaDB), 6379 (Redis), 7809 (the Git webhook listener) and a port another app website uses.
- **Start command (optional)**: one line of at most 1000 characters, such as `node server.js`. Nefo runs it in the website's `app` folder as the website's Linux user, with `PORT`, `HOST=127.0.0.1` and `NODE_ENV=production` set. It runs as the systemd service `panel-app-<user>.service` and restarts 3 seconds after it stops. Leave the command empty if the program is started another way, for example in Docker.

Put your code in `/var/www/example.com/app`, over [SFTP](#sftp-access) or with [Git deploy](./git-deploy), which deploys an app website into this folder. Then, in the website's settings:

1. On the **Runtime** tab, choose the **Runtime**: **Server default (node in /usr/local/bin)**, a Node.js version you installed on the **Software** page, Python (`app/.venv`), which needs Python installed on the **Software** page, or a Java version you installed on the **Software** page, shown as "Java 21 (OpenJDK 21.0.9)" for example. The same tab changes the **Start command (optional)**. Click **Save**.
2. On the **Overview** tab, under **Application program**, click **Install dependencies**. Nefo runs `npm ci` (or `npm install` without a `package-lock.json`) for a `package.json`. For Python it creates the virtual environment `app/.venv` and installs `requirements.txt` with pip. It runs as the website's user, for up to 20 minutes.
3. Click **Restart** (or **Start**) to run the app with its dependencies.

The **Application program** card appears when the website has a start command. It also has **Stop** and **App logs**. On the **Reverse proxy** tab you can change the port. The web server forwards requests to `127.0.0.1` on that port, with the visitor's address in `X-Real-IP` and `X-Forwarded-For`, and WebSocket connections work. To forward to another server, or to a service on this server that Nefo doesn't start, create a [reverse proxy website](#reverse-proxy-websites) instead.

The websites list has a **Java** filter once a Java app website exists, next to the filters for the other types.

#### Java app websites

For a Java program, set the **Start command (optional)** to the command that starts it, for example `java -jar app.jar`, and put `app.jar` in `/var/www/example.com/app`. Choose the Java version as the **Runtime**. Install Java first on the **Software** page (see [Java](./software#java)).

- The app's service gets that version's `JAVA_HOME` and its `bin` folder first in `PATH`, so `java` is that version. Besides `PORT` and `HOST=127.0.0.1`, which every app website gets, it gets `SERVER_PORT` (the same port) and `SERVER_ADDRESS=127.0.0.1`, which Spring Boot reads. A Spring Boot application then listens on the right port, on this server only, without settings of its own. Other frameworks read `PORT` or need it in their own setting.
- **Memory limit (MB)** appears for a Java runtime. It sets Java's maximum heap (`-Xmx`), from 32 to 65536 MB, through `JDK_JAVA_OPTIONS`, which every `java` the start command runs reads, a start script too. Leave it empty and Java takes up to a quarter of the server's memory. The process uses somewhat more memory than its heap. Java prints `NOTE: Picked up JDK_JAVA_OPTIONS: -Xmx512m` at the start of the **App logs**.
- [Git deploy](./git-deploy) scripts of the website run with the same `JAVA_HOME` and `PATH`, so `./mvnw package` or `./gradlew build` build with the website's Java version. The memory limit doesn't apply to them.
- **Install dependencies** isn't offered for a Java website: build the jar in your deploy script or before you upload it.
- A Java version that a website uses can't be uninstalled on the **Software** page. Choose another runtime first, or delete the website.

## Reverse proxy websites

A reverse proxy website forwards every request to an address you choose: a website or app on another server, or a service on this server that Nefo doesn't run, such as a program you started yourself or a container of your own Compose project. The website has its own domain, HTTPS certificate, access options and web firewall, like any other website. It works with Nginx, Apache and OpenLiteSpeed.

1. Open **Websites** and click **Add website**.
2. Type the **Domain** and choose the type **Reverse proxy to another address**.
3. In **Forward to**, type the address: `http://` or `https://`, a host name or an IP address, an optional port and an optional path, for example `http://198.51.100.7:8080` or `https://backend.example.com/app`.
4. Choose the **Host header sent**: this website's domain, or the upstream's host.
5. Click **Create website**.

Nefo shows "example.com was created. It forwards to … once its domain points at this server." The address is stored with its port, so `https://backend.example.com/app` shows as `https://backend.example.com:443/app`.

The request's path goes after the address's path: with `http://198.51.100.7:8080/app`, a visit to `https://example.com/shop?page=2` reaches `http://198.51.100.7:8080/app/shop?page=2`.

### The options

Change them on the website's **Reverse proxy** tab and click **Save**. A new reverse proxy website has WebSocket and the X-Forwarded-\* headers on, and a time limit of 60 seconds.

| Option | What it does |
|---|---|
| **Forward to** | The address, as above. |
| **Host header sent** | The website's domain sends the domain the visitor asked for: most apps behind a proxy expect it. **The upstream's host** sends the upstream's own name (with its port when it isn't 80 or 443), for a server that answers only to its own name. The upstream's own address in redirects (`Location`) is then changed to the website's. |
| **WebSocket** | Passes WebSocket connections on (chat, live updates, terminals). |
| **X-Forwarded-\* headers** | Tells the upstream the visitor's address and whether the request came over HTTPS: `X-Forwarded-For`, `X-Forwarded-Proto`, `X-Forwarded-Host` and `X-Real-IP`. When it is off, these headers are removed, also the ones a visitor sends. |
| **Time limit (seconds)** | How long the upstream may take to answer, from 5 to 3600 seconds. The connection itself must open within 30 seconds (or the time limit, if shorter). A slower upstream gets the visitor an error page: 504 with Nginx, 502 with Apache. |
| **Ignore the upstream's certificate (self-signed)** | Shown for an `https://` address. Accepts any certificate from the upstream. |

::: warning
**Ignore the upstream's certificate (self-signed)** keeps the connection to the upstream encrypted, but anyone who can intercept it could pose as the upstream. Use it only for an upstream on your own network with a self-signed certificate.
:::

Without that option, Nginx and Apache check an `https://` upstream's certificate against the certificate authorities of the server (`/etc/ssl/certs/ca-certificates.crt`), for the name in the Host header: the website's domain, or the upstream's host if you chose it. A self-signed or expired certificate gets the visitor an error page (502 with Nginx, 500 with Apache).

### What Nefo refuses

- an address that isn't `http://` or `https://` with a host name or an IP address, an optional port (1 to 65535) and path, for example one with a user name, a query (`?`), a fragment (`#`), spaces, `..` in the path, or the addresses `0.0.0.0` and `::`;
- a host name that can't be found ("The host name … cannot be found. Check it, or use its IP address.");
- the panel itself: the panel's port or `7809` (the Git webhook listener) on this server, whether you write `127.0.0.1`, `localhost`, one of the server's own IP addresses or a name that points at them;
- ports 80 and 443 on this server: the website would forward to the web server itself;
- a **Time limit (seconds)** under 5 or over 3600.

### Good to know

- With Nginx, a host name is looked up when requests come in, with the DNS servers of `/etc/resolv.conf`, so a name that doesn't resolve at boot never keeps Nginx from starting. These lookups don't read `/etc/hosts`: for a name that only `/etc/hosts` knows, use its IP address.
- HTTPS, the redirect to HTTPS and HSTS work as for other websites: see [SSL certificates](./ssl).
- **Suspend**, **Redirect**, password protection, blocked IP addresses, the rate limit, [country blocking](./website-protection#block-or-allow-countries) and the web firewall apply before a request reaches the upstream.
- The tabs for rewrite rules, the custom 404 page, hotlink protection and the file lock don't apply to a reverse proxy website and are hidden. Your own directives (the web server configuration tab) still apply.
- The website still has its folder `/var/www/example.com` and Linux user, for SFTP and backups, but it doesn't serve those files.

### With OpenLiteSpeed

OpenLiteSpeed forwards differently from Nginx and Apache. The **Reverse proxy** tab says so in a note, and **Ignore the upstream's certificate (self-signed)** isn't shown there:

- It never checks an `https://` upstream's certificate: a self-signed certificate works without the option.
- The **Time limit (seconds)** isn't applied: OpenLiteSpeed has no limit for a proxied answer.
- It doesn't send the visitor's address in `X-Forwarded-For`: it sends `X-Forwarded-Host`, and `X-Forwarded-Proto` for HTTPS requests. With **X-Forwarded-\* headers** off, Nefo removes `X-Forwarded-For` (and `X-Real-IP` is always removed); OpenLiteSpeed still sends `X-Forwarded-Host`.
- WebSocket connections reach the upstream without the address's path (`/live`, not `/app/live`), and don't work with an `https://` upstream.
- Paths, the upstream's Host header, the password and blocked addresses work as with the other web servers.

## Open a website's settings

Click the domain in the list, or **Settings** at the end of its row. The settings open in a panel with tabs in four groups. Some tabs show only for some websites:

| Group | Tabs |
|---|---|
| **Basics** | **Overview**, **Domains**, **PHP** (PHP sites), **Runtime** (app sites), **Reverse proxy** (reverse proxy sites) |
| **Security** | **SSL / HTTPS**, **Web firewall (WAF)**, **Block access**, **Country blocking**, **File lock** (not for Docker apps and reverse proxy sites), **Hotlink protection** (not for reverse proxy sites), **Malware scan** |
| **Configuration** | **Rewrite** (PHP and static sites), **Page cache** (PHP sites on Nginx and OpenLiteSpeed), **Nginx configuration** (named after your web server), **Redirect**, **Reverse proxy** (app sites: the port), **404 page** (not for reverse proxy sites) |
| **Operations** | **WordPress** (WordPress sites), **Git deploy** (not for Docker apps), **SFTP**, **Backups**, **Monitoring**, **Traffic**, **Logs**, **Expiry and suspension** |

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

Click **Save**. If PHP-FPM or the web server rejects the result, Nefo puts the previous settings back. To install other PHP versions or extensions, the ionCube Loader, or to change `php.ini`, see [Software](./software).

## Rewrite rules and your own directives

On the **Rewrite** tab, **Rewrite rules** offers ready-made rules for common applications: **Default (front controller: index.php or index.html)**, WordPress, Laravel, Symfony, CodeIgniter 4, Drupal, Joomla, OpenCart, Nextcloud, Moodle and phpBB for PHP sites, and Single-page app and Static site (clean URLs) for any site. The tab shows the rules of the preset you pick. On Nginx a preset replaces the default `location /` block. On Apache the site's own `.htaccess` file may replace the preset's front controller, and on OpenLiteSpeed the front controller runs after the site's `.htaccess` rules. On both, the preset's protective rules can't be undone by `.htaccess` files.

**Static site (clean URLs)** serves pages without their `.html` ending, as site generators such as VitePress (with `cleanUrls`) link to them: `/docs/install` serves `docs/install.html`, and `/docs/` serves `docs/index.html`. Other files are served as they are, and a page that doesn't exist still gets a 404.

The phpBB preset is for a phpBB you install yourself. It sends addresses that are not files to phpBB's `app.php`, and never serves phpBB's settings files (`config.php`, `common.php`), its `cache`, `files` (attachments), `store` (backups), `includes`, `phpbb` and `vendor` folders or `images/avatars/upload`: those answer 403.

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

The same tab blocks IP addresses and, on Nginx, limits requests per visitor: see [Security](./security#rate-limit-and-blocked-addresses). To keep out whole countries, see [Country blocking](./website-protection#block-or-allow-countries).

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

The account is locked in the website's folder. It sees only `/var/www/example.com`, at the same path as in the panel and in PHP, and it can't run commands or forward ports. The server's password policy may refuse a password you choose; Nefo then shows why. While the website's [files are locked](./website-protection#lock-a-website-s-files), the SFTP account can only change the writable folders.

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

## Web firewall, malware scan, country blocking, file lock and backups

- **Web firewall (WAF)**: ModSecurity with the OWASP Core Rule Set for this site: **Off**, **Detect only: log attacks, block nothing** or **Block attacks**. Install the firewall on the **Software** page first. See [Security](./security#web-application-firewall), which also covers the **Malware scan** tab.
- **Country blocking**: refuse visitors from some countries, or let in only some. **File lock**: make the website's code read-only for its PHP and its SFTP account, except the folders it must write to. See [Country blocking and file lock](./website-protection).
- **Backups**: **Back up now**, or schedule a backup **Every day at 03:00**. See [Backups](./backups).

## Clone a website

**Clone** copies a website to a new domain: to start a new website from an existing one, or to try changes on a copy. It works for PHP, static and reverse proxy websites, app websites (Node.js, Python, Java) and Docker apps from the [App store](./apps).

1. Open the website's **Overview** tab and click **Clone…** under **Clone this website**.
2. Type the **Domain of the copy**.
3. Leave **Copy the databases linked to this website** ticked to copy them too. The box isn't shown for WordPress, whose database is always copied, and for Docker apps, whose data is always copied.
4. Click **Clone**. The website keeps running during the copy, which runs as a task with a log. A Docker app stops for a moment while its data is copied.
5. Point the new domain's DNS to this server, then turn on HTTPS on its **SSL / HTTPS** tab. The copy starts on `http://`.

### PHP, static and reverse proxy websites

The copy gets:

- a new website of the same type (the same PHP version, static, or a reverse proxy to the same address with the same options) with its own Linux user, the same app, PHP settings, rewrite rules and own directives. In your directives, the old website's folder is replaced with the new one;
- a copy of the website's folder `/var/www/example.com`, owned by the new user, without `logs`, `tmp` and the Git deploy key (`.ssh`). Symbolic links are copied as links; a link to a place inside the old website's folder now leads to the same place in the new one. In `public/.user.ini` the old folder is replaced with the new one;
- for each linked database, MariaDB, MySQL or PostgreSQL, a new database and user of the same kind with a copy of its data, linked to the new website;
- the website's background programs, stopped (see below).

When the copy is ready, Nefo shows the connection details of the new MariaDB and MySQL databases (PostgreSQL ones are on the **Databases** page). The app's own configuration files (such as `.env` or `config.php`) still name the old database and user: change them to the new ones. Files that name the old folder, such as a cache of the app's settings, still do too.

### App websites

The copy of an app website gets, in addition:

- a port of its own, chosen by Nefo, which the app gets in `PORT`. An app that sets its own port must be changed to use the new one;
- the same **Runtime** and **Start command (optional)**; in the command, the old website's folder is replaced with the new one. The copy's service `panel-app-<user>.service` starts once the files and databases are copied;
- `node_modules` with its links (`node_modules/.bin`), and a Python virtual environment `app/.venv` that works in its new folder: the paths in its scripts, `pyvenv.cfg` and `.pth` files name the copy's folder. The dependencies aren't installed again;
- the website's background programs, with the same command, folder and variables (paths to the old folder replaced), but **stopped**: until the copy uses databases and queues of its own, they would work on the original's. Start them on the **Background programs** page.

The runtime must still be installed: a Node.js version, Python or Java removed from the **Software** page is refused before the copy starts.

### Docker apps

The copy of a Docker app's website is a new website with a new installation of the app:

- the same version and images as the original (also when the App store has a newer one), with its own containers, ports and the same memory limits;
- a copy of the app's data from `/var/lib/panel-apps/<name>`, made while the original is stopped: Nefo stops it, copies the data, and starts it again right away. Files of the original website's user belong to the new website's user in the copy;
- the same accounts and passwords as the original (the copied data was made with them), shown under **App store** → **Installed** as for any app;
- the app's address set to the new domain, for apps that take it from the panel (`url`, `DOMAIN` and similar settings of the app). If the app keeps its own address in its own settings, change it there.

Nefo checks the memory and disk the copy needs first (about the size of the app's data, plus 512 MB).

Nefo refuses to clone:

| App | Why |
|---|---|
| n8n | The copy would run the active workflows (schedules and triggers) a second time. Export the workflows and import them into a new n8n. |
| Odoo | The copy would run the scheduled actions and mail servers too: it would send the same e-mails, and fetch and delete the same mailbox's messages. Back it up, or install a new Odoo. |
| An app installed before the App store | Update it first (**App store** → **Installed**), then clone it. |

A copy of a monitoring app such as Uptime Kuma checks the same websites and sends the same notifications as the original: pause what you don't need in one of them.

### WordPress

A WordPress site is copied like a [staging copy](./wordpress#make-a-staging-copy): its database is always copied, `wp-config.php` is pointed at the copy and the site's address is replaced everywhere, so the copy works right away. Search engines are asked not to index it; if the copy is a website of its own, change that in WordPress under **Settings → Reading**. Unlike a staging copy, a clone can't be pushed back to the website it came from.

### Not copied

Aliases, the certificate, password protection, redirects and firewall settings, country blocking, the file lock, cron jobs, Git deploy settings, SFTP access, monitoring and backup schedules.

If a step fails, Nefo removes everything it created for the copy: the website, its Linux user, the new databases, the copied background programs and, for a Docker app, the new containers and their data. The original keeps running.

## Delete a website

1. Open the website's **Overview** tab.
2. Under **Delete this website**, click **Delete**.
3. The dialog lists what goes with the website under **Removed with it:** (its Docker app and data, SFTP account, cron jobs, background programs, monitoring, Git deploy settings, backup schedules).
4. Leave **Back up the website first** ticked (the default), or untick it.
5. Tick **Also delete its databases** to delete the linked databases too. Unticked (the default), they stay on the **Databases** page, linked to no website.
6. Type the domain to confirm and click **Back up and delete** (or **Delete** without the backup).

### The backup made first

With **Back up the website first**, Nefo makes a normal backup on this server first: the website's files and its MariaDB, MySQL and PostgreSQL databases, and for a Docker app its data as a second backup of the kind **App**. The website is deleted only if the backup succeeded. If it fails, the task stops with "the backup failed, so example.com was not deleted" and nothing is removed.

- The backup is named `example.com_before-delete_<date>-<time>.tar.gz` (and `<user>_before-delete_….tar.gz` for a Docker app's data), in `/var/backups/panel/site/example.com/`.
- Like every backup you make by hand, it is kept until you delete it on the **Backups** page. Schedules only delete their own backups, and the website's schedules are removed with it, so nothing deletes it.
- It stays on this server only: it isn't uploaded to remote storage. Upload it from the **Backups** page if you want a remote copy.
- The deletion runs as a task with a log. Its log names the backup.

::: warning
Without the backup, the website's files (and its databases, if you tick **Also delete its databases**) can't be brought back. The dialog says so when you untick **Back up the website first**.
:::

### Can I bring a deleted website back? {#bring-a-deleted-website-back}

Yes, with the backup made before the deletion (or any other backup of the website). On the **Backups** page, restore the `…_before-delete_…` backup and type the domain to confirm. The restore creates the website again with its domain, aliases, PHP version, rewrite rules, files and databases. A database that was deleted comes back with the password it had, which the restored files name; a database you kept is linked to the website again.

Not created again: the certificate (get a new one on the **SSL / HTTPS** tab), cron jobs, SFTP access, background programs, redirects and password protection. See [Restore on a new server](./restore-new-server#restoring-a-website-that-doesn-t-exist-here) for what the restore creates.

Backups made by Nefo 0.24.1 or later record the website's type, so an app website (Node.js, Python, Java) comes back with its runtime, start command, port and memory limit, and a reverse proxy website with its address and options; install the app's runtime on the **Software** page first. Backups made by older versions record only the PHP version, so such a website comes back as a static website: create it again with the same domain and type first, then restore. For a Docker app, install the app again on the same domain from the **App store**, then restore its backup of the kind **App**.

### What Nefo removes and keeps

Nefo removes:

- the website folder `/var/www/example.com` with all its files, including a Git repository and its deploy key
- the Linux user and group, and the SFTP access
- the web server configuration, the password file and your own directives
- the PHP-FPM pool, the app service or the Docker app with its containers, data and unused images
- the certificate, the page cache and the web server logs of the site
- the website's background programs, the cron jobs that run as its user, its backup schedules, its monitoring and its Git deploy settings
- the website's trash in **Files** and its files quarantined by the malware scanner
- the linked databases, if you ticked **Also delete its databases**

Nefo keeps the backups you already made: they stay on the **Backups** page.

WordPress staging copies: deleting a live site that has a staging copy keeps the copy, which can no longer be pushed to a live site; the dialog says so. Deleting a staging copy doesn't change its live site.

While a Git deploy of the website is running, Nefo refuses to delete it ("Another task is running on the server. Wait until it has finished."). With the backup, the task waits for a deploy that started during the backup. If a cleanup step fails, the website is removed from the panel anyway and Nefo lists the steps that failed under "Some cleanup steps failed:" (in the task's log when the backup was made first).

From the API: `DELETE /api/sites/{id}` with `{"confirm": "example.com", "delete_databases": false, "backup_first": true}`. With `backup_first` it answers 202 with the task. Without it, the website is deleted at once (200, with the warnings).
