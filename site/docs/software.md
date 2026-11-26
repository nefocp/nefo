---
title: Software
description: Install and uninstall the web server, PHP versions, MariaDB or MySQL, PostgreSQL, Redis, Memcached, Adminer, phpMyAdmin, Node.js, Python, Docker, the web firewall and ClamAV, and manage PHP extensions and php.ini settings.
---

# Software

The **Software** page lists everything optional the server can run, with **Install** and **Uninstall** for each item. A small server stays light when it only carries what its websites use. This page covers what you can install, when an uninstall is refused, what an uninstall keeps, and the settings of PHP, Node.js, Python and Redis.

Installs and uninstalls run as tasks with a live log. The panel runs one task at a time. Software comes from your distribution's packages unless this page says otherwise.

## What you can install

Each card shows roughly how much memory the item uses while it runs. The top of the page adds up the installed items.

| Section | Item | Memory, roughly |
|---|---|---|
| **Nginx, Apache or OpenLiteSpeed** | Nginx | about 15 MB |
| | Apache | about 40 MB |
| | OpenLiteSpeed | about 26 MB |
| **PHP versions** | PHP 7.4, 8.0, 8.1, 8.2, 8.3, 8.4 and 8.5, and under **Older versions** 5.6, 7.0, 7.1, 7.2 and 7.3 | about 25 MB each |
| **Databases and cache** | MariaDB, or instead MySQL 8.4 LTS | MariaDB: its buffer pool plus about 100 MB (see [Databases](./databases#how-mariadb-is-tuned)). MySQL: its buffer pool plus about 150 MB, or 300 MB from about 4 GB of RAM (see [Databases](./databases#how-mysql-is-tuned)) |
| | PostgreSQL | about 30 to 40 MB while idle, growing to `shared_buffers` plus about 40 MB (see [Databases](./databases#how-postgresql-is-tuned)) |
| | Redis | up to 64, 128 or 256 MB, depending on the server's RAM |
| | Memcached | up to 64, 128 or 256 MB, depending on the server's RAM; a few MB while empty |
| | Adminer (database UI) | none while idle |
| | phpMyAdmin (database UI) | none while idle |
| **Runtimes** | Node.js | none while idle |
| | Python apps | none while idle |
| **Containers** | Docker | about 90 MB, plus your containers |
| **Security** | Web firewall (ModSecurity) | about 40 MB |
| | ClamAV antivirus | about 15 MB, and about 1.4 GB of free memory while it scans |
| **Tools** | Composer | none, it only runs when called |
| | WP-CLI | none, it only runs when called |

## Install an item

1. Go to **Software**.
2. Select **Install** on the item's card.
3. Follow the log. When the task has finished, the card shows **Installed** and its version.

Some cards then offer a link: **Open Databases**, **Open PostgreSQL**, **Open Redis**, **Open Memcached**, **Open Adminer**, **Open phpMyAdmin** or **Open Docker**.

## Uninstall an item

1. Select **Uninstall** on the item's card. For a PHP version, it is in the version's menu.
2. Read what the dialog says the uninstall does.
3. For MariaDB, MySQL, PostgreSQL and Docker, you can tick the box to delete their data too. You must then type the item's name to confirm.
4. Select **Uninstall**.

### When an uninstall is refused

The panel never removes software that something still uses. When an item can't be uninstalled, its card shows the reason instead of a working button.

| Item | Refused while |
|---|---|
| Web server | Any website exists, in the panel or as a configuration file in the web server's sites folder |
| A PHP version | A website uses it, or it is the only PHP version installed (the web server's setup and Adminer need one). A supported version (7.4 or newer) also stays while it is the only one next to [older versions](#older-php-versions) |
| Composer | A Git deploy script runs `composer` |
| WP-CLI | WordPress websites exist: the WordPress toolkit runs it for them |
| MariaDB or MySQL | It holds any database other than its own system databases. The panel asks the server itself, so databases made by hand or by another tool count too. If it is stopped and its data folder was moved, the panel can't check and refuses |
| PostgreSQL | PostgreSQL holds any database other than `postgres`, `template0` and `template1`, as PostgreSQL itself says, or it is stopped, so the panel can't check |
| Node.js | App websites use the server's default Node.js, or the `node` in `/usr/local/bin` wasn't installed by the panel |
| A line under **Node.js versions** | It is the server's default, or an app website chose it |
| Python apps | App websites run Python |
| Docker | Websites run Docker apps, Compose projects exist, or Docker has containers, running or stopped |

Redis, Memcached, Adminer, phpMyAdmin, the web firewall and ClamAV can always be uninstalled.

### What an uninstall keeps

| Item | Kept | Removed |
|---|---|---|
| Web server | Website folders in `/var/www`, certificates, PHP | The web server, its panel configuration, Adminer, and the firewall rules for ports 80 and 443 |
| PHP version | | The version, its extensions and the libraries only they needed |
| MariaDB | The data folder `/var/lib/mysql`, unless you tick **Also delete the data directory (/var/lib/mysql)** and type `mariadb`. Installing MariaDB again uses it | The server and client packages and the panel's MariaDB settings |
| MySQL | The data folder `/var/lib/mysql`, unless you tick **Also delete the data directory (/var/lib/mysql)** and type `mysql`. Installing MySQL again uses it | The server and client packages, the panel's MySQL settings, and Oracle's repository with its key |
| PostgreSQL | The data in `/var/lib/postgresql` and the cluster settings in `/etc/postgresql`, unless you tick **Also delete the data and the settings (/var/lib/postgresql and /etc/postgresql)** and type `postgresql`. The packages are removed without purging, and installing PostgreSQL again starts the same cluster | The PostgreSQL packages |
| Redis | | Redis, its PHP extension, and everything stored in Redis. PHP-FPM restarts |
| Memcached | | Memcached, its PHP extension, and everything cached. PHP-FPM restarts |
| Adminer | Your databases | Adminer's files and its internal site |
| phpMyAdmin | Your databases | phpMyAdmin's files, its temporary files, its internal site and its settings |
| Node.js | Other Node.js versions. The default's folder too, if an app website chose that version | `node`, `npm` and `npx` in `/usr/local/bin`, and the release they point to |
| Python apps | `python3` itself, which the system needs, and the virtual environments in old app folders | The venv and pip packages |
| Docker | Images, volumes and build cache in `/var/lib/docker`, unless you tick **Also delete all images, volumes and build cache (/var/lib/docker)** and type `docker`. Installing Docker again finds them | Docker and its Compose plugin |
| Web firewall | The firewall's log | ModSecurity and the OWASP rules. The web server restarts |
| ClamAV | | ClamAV and its virus databases |
| Composer | Laravel and Drupal websites, which only needed it to install | `/usr/local/bin/composer` and the distribution's `composer` package |
| WP-CLI | | `/usr/local/lib/panel/wp-cli.phar`. The WordPress toolkit installs it again when it needs it |

::: danger
Deleting the data of MariaDB, MySQL, PostgreSQL or Docker can't be undone. Make sure you have a backup of anything you still need.
:::

## The menu shows what is installed

The left menu hides pages for software that isn't installed:

- **Docker** appears once Docker is installed.
- **WordPress** appears once a WordPress website exists.
- On the **Databases** page, the **PostgreSQL**, **Redis** and **Memcached** tabs appear once that software is installed, and **Slow queries** once MariaDB or MySQL is installed.

**Websites**, **Databases** and **Software** always show, because they offer the installs. A page you have open stays visible.

## Web server

The panel runs one web server at a time and sets every website up for it:

- Nginx is the lightest. PHP runs in PHP-FPM.
- Apache reads the `.htaccess` files some PHP applications come with, and uses a little more memory.
- OpenLiteSpeed reads the rewrite rules in `.htaccess` files and has LiteSpeed Cache built in for WordPress. It comes from LiteSpeed's own package repository.

Installing a web server also installs your distribution's PHP version with PHP-FPM. If the ufw firewall is on, the panel opens ports 80 and 443, and 443/udp for HTTP/3 with OpenLiteSpeed.

The panel refuses to install a second web server while another one is installed, because both would need ports 80 and 443.

To switch to another web server:

1. Delete all websites first. Back them up if you need them later.
2. Select **Uninstall** on the current web server and type its name (`nginx`, `apache` or `openlitespeed`) to confirm.
3. Install the other web server.

See [Websites](./websites) for setting up sites.

## PHP versions

You can install several PHP versions side by side and choose one per website in **Websites** → (site) → **PHP**.

- The version your distribution ships comes from its packages. Other versions come from Ondřej Surý's repository `packages.sury.org/php`, on Ubuntu and Debian alike. The panel asks before it adds that repository to the server, and only accepts its signing key with the fingerprint `15058500 A0235D97 F5D10063 B188E2B6 95BD4743`.
- Each version is installed with these extensions: fpm, cli, mysql, curl, gd, mbstring, xml, zip, intl, bcmath and opcache. When Redis or Memcached is installed, the redis or memcached extension is added too.
- Before PHP 8.0, json is a package of its own and is added too. PHP 5.6 also gets mcrypt, where the repository has it.
- The panel switches the version's default PHP-FPM pool to on-demand workers, so a version no website uses costs almost no memory.

Select **Manage** on an installed version to open its **Extensions** and **Settings**.

### Versions without security updates {#end-of-life}

Each PHP version gets security updates from PHP until a set date (see [php.net/supported-versions.php](https://www.php.net/supported-versions.php)): 8.1 until 31 December 2025, 8.2 until 31 December 2026, 8.3 until 31 December 2027, 8.4 until 31 December 2028 and 8.5 until 31 December 2029. 7.4 and 8.0 have none any more. After that date, a version from `packages.sury.org` shows the badge **End of life — no security updates** and a line with the date, on the Software page, in its **Manage** window and in the websites' PHP version menus. The panel compares the date with today, so the badge appears by itself.

Your distribution's own PHP version (8.1 on Ubuntu 22.04, 8.3 on 24.04, 8.5 on 26.04, 8.2 on Debian 12, 8.4 on Debian 13) never shows the badge: the distribution's security team keeps patching it for as long as the release is supported.

Move websites off a version with the badge when you can. You can still install it without an extra confirmation; only the [older versions](#older-php-versions) ask.

### Older PHP versions {#older-php-versions}

PHP 5.6, 7.0, 7.1, 7.2 and 7.3 are under **Older versions**, folded below the other versions. They are there for websites moved from another server (cPanel, aaPanel…) that don't run on a newer PHP yet.

::: danger End of life: no security updates
PHP stopped the security updates of these versions years ago: 5.6 in 2018, 7.3 in 2021 (see [php.net/eol.php](https://www.php.net/eol.php)). Their known vulnerabilities will never be fixed, so a website running on one is easier to break into. Use them only to keep an old website running while you update it, then move it to a supported version. Don't use them for new websites.
:::

- Each one shows the badge **End of life — no security updates**, on the Software page and in the websites' PHP version menus.
- Installing one asks you to confirm that risk first.
- A supported version (7.4 or newer) must be installed first, and the last supported version can't be uninstalled while older ones are installed. So an old version is never the server's only PHP: the web server's setup, Adminer, Composer and the panel's tools keep a supported one.
- The App store keeps its own minimum versions: WordPress needs PHP 7.4, and the other apps more. The WordPress toolkit still manages a WordPress website on an old version, but it doesn't update WordPress itself there, because the current WordPress needs PHP 7.4. It still updates plugins and themes; WP-CLI skips the updates that need a newer PHP.
- The website's PHP-FPM pool and the php.ini settings are the same as on newer versions: every setting the panel writes exists in PHP 5.6.

`packages.sury.org/php` has PHP 5.6 to 7.3 for Ubuntu 22.04, 24.04 and 26.04 and Debian 12 and 13, on amd64 and arm64. The install also checks with apt, and refuses with a reason if the repository doesn't have the version.

### Servers that used the ondrej/php PPA {#ondrej-php-ppa}

Earlier versions of the panel took the extra PHP versions on Ubuntu from `ppa:ondrej/php`. That PPA is frozen and no longer receives updates: its maintainer now publishes the Ubuntu packages at `packages.sury.org/php` (see the notice on [launchpad.net/~ondrej/+archive/ubuntu/php](https://launchpad.net/~ondrej/+archive/ubuntu/php)).

When the panel starts on an Ubuntu server that still has the PPA, it switches the server over once, the way the PPA's notice describes:

1. It adds `packages.sury.org/php` with its pinned key (`/usr/share/keyrings/sury-php.gpg` and `/etc/apt/sources.list.d/sury-php.list`), unless a source for it exists already.
2. It moves the PPA's source file from `/etc/apt/sources.list.d`, and the PPA's key `/etc/apt/trusted.gpg.d/ondrej-ubuntu-php.gpg` if there is one, to `apt-backup` in the panel's data folder (`/var/lib/nefo/apt-backup`). That key was trusted for every repository and nothing needs it any more. Other PPAs, even of the same maintainer, are left alone.

Nothing is installed, removed or downgraded, and the panel doesn't run `apt-get update` at start-up. The next update (the panel's update check, an install, or your own `apt update`) reads the new source, and the installed PHP packages get their next updates from it. The panel log says what it changed. If the key can't be downloaded, the PPA stays and the panel tries again at the next start. A PPA line in `/etc/apt/sources.list` itself, or in a file that also lists other repositories, is not edited: the log asks you to replace it by hand.

### Extensions

The **Extensions** tab lists optional extensions that have a package for this version. Select **Install** or **Remove**, and PHP-FPM reloads.

The optional extensions are: apcu, gmp, igbinary, imagick, imap, ldap, memcached, mongodb, msgpack, pgsql, redis, soap, sqlite3, ssh2, tidy, xdebug, xsl and yaml.

::: warning
xdebug slows every site on that PHP version down. Don't leave it installed on a production server.
:::

### php.ini settings

The **Settings** tab sets php.ini values for every website on this PHP version. Leave a field empty to use PHP's default, then select **Save**.

| Setting | Default | Allowed values |
|---|---|---|
| `date.timezone` | UTC | A time zone name such as `Europe/Berlin` |
| `max_input_vars` | 1000 | 100 to 100000 |
| `max_input_time` | 60 | -1 to 3600 |
| `default_socket_timeout` | 60 | 1 to 600 |
| `session.gc_maxlifetime` | 1440 | 60 to 2592000 |
| `realpath_cache_size` | 4096K | A number, optionally followed by K or M |
| `display_errors` | Off | On or Off |
| `expose_php` | Off | On or Off |
| `allow_url_fopen` | On | On or Off |
| `short_open_tag` | Off | On or Off |
| `opcache.enable` | 1 | 1 or 0 |
| `opcache.memory_consumption` | 128 | 32 to 2048 |
| `opcache.max_accelerated_files` | 10000 | 1000 to 1000000 |
| `opcache.validate_timestamps` | 1 | 1 or 0 |
| `opcache.revalidate_freq` | 2 | 0 to 3600 |

The values are written to `/etc/php/<version>/fpm/conf.d/99-panel.ini` and checked with `php-fpm -t`. If PHP-FPM rejects them, the previous file comes back and nothing changes. Memory, upload size and time limits are set per website, in the website's settings.

## Node.js

The **Node.js** card installs the newest LTS release from nodejs.org, checked against its published SHA-256 checksum. It goes to `/opt/node-v<version>-linux-<arch>`, and `node`, `npm` and `npx` are linked into `/usr/local/bin`. This is the server's default Node.js, used by app websites that don't choose a version.

### Several Node.js versions

Once Node.js is installed, the **Node.js versions** card lists the three newest LTS lines and any line installed. Select **Install** to add a line next to the others. It doesn't change the default.

Choose a line for an app website in **Websites** → (site) → **Runtime**. A line that app websites use, and the default, can't be uninstalled from this card. Uninstall the default from the **Node.js** card instead.

The list comes from nodejs.org, so the server needs internet access to show it.

::: warning
Uninstalling the default Node.js also breaks cron jobs and background programs that call `node`.
:::

## Python

The **Python apps** card installs `python3-venv` and `python3-pip`, so app websites can run Python (Django, Flask, FastAPI…). Each Python app website gets its own virtual environment in `app/.venv`. Build tools such as `build-essential` and `python3-dev` are not installed, so a package that has no ready-made wheel and must be compiled fails to install until you add them with `apt`.

Uninstalling removes venv and pip. `python3` itself stays, because the system needs it.

## Redis

Redis is an in-memory cache. WordPress can use it with a Redis object cache plugin.

When you install Redis, the panel:

- installs `redis-server` and the redis extension for every installed PHP version;
- limits Redis's memory to suit the server: 64 MB under 1.5 GB of RAM, 128 MB from 1.5 GB, 256 MB from 3 GB;
- sets the policy `allkeys-lru`, so Redis drops the least recently used keys when it is full, like a cache;
- leaves Redis listening on `127.0.0.1:6379`, reachable from this server only.

To look into Redis, go to **Databases** → **Redis**. See [Databases](./databases#redis-key-browser).

Before you uninstall Redis, turn off the cache plugin of your websites. They keep working without the cache, a little slower.

## PostgreSQL

The **PostgreSQL** card installs your distribution's `postgresql` package (16 on Ubuntu 24.04, 15 on Debian 12, 17 on Debian 13, 14 on Ubuntu 22.04). The panel tunes it for the server's memory in a `conf.d` drop-in, lets it listen on localhost only, and writes a `pg_hba.conf` that asks everyone for a password except the `postgres` system user on the local socket. Its databases are on **Databases** → **PostgreSQL**; see [Databases](./databases#postgresql) for the settings, the import rules and backups.

When Adminer is installed, the panel adds PHP's `pgsql` extension for Adminer's PHP version, so Adminer opens PostgreSQL databases too. The extension is only added together with PostgreSQL.

## Memcached

Memcached is a small in-memory cache, for PHP sessions and application caches (Laravel's `memcached` cache driver, for example).

When you install Memcached, the panel:

- installs `memcached` and the memcached extension for every installed PHP version, and adds the extension to PHP versions you install later;
- writes `/etc/memcached.conf`: Memcached listens on `127.0.0.1:11211` only, over TCP, with UDP off (`-U 0`), so it can't be used to amplify attacks;
- limits its memory to suit the server: 64 MB under 1.5 GB of RAM, 128 MB from 1.5 GB, 256 MB from 3 GB, and allows 512 connections under 1.5 GB of RAM, 1024 from there.

When the memory is full, Memcached drops the least recently used items. To see its numbers, go to **Databases** → **Memcached**.

Before you uninstall Memcached, point your websites' sessions and cache elsewhere: an application whose cache driver is Memcached fails without it.

## MariaDB and Adminer

The **Database server** card installs MariaDB or MySQL 8.4 LTS, tuned to the server's memory and reachable from this server only. A server has one of them:

- While neither is installed, the card offers the choice, with **MariaDB** selected: it is lighter and is what WordPress and most PHP applications run on. **MySQL 8.4 LTS** comes from Oracle's repository and is offered on Ubuntu 22.04, 24.04 and 26.04 and Debian 12 and 13, on amd64 servers only. Elsewhere the card says why it isn't offered.
- Once one is installed, the card shows which one, its version, its memory estimate and **Uninstall**. The other can't be installed until it is uninstalled.

Switching from one to the other isn't automatic: export the databases, uninstall, install the other and import them again. See [Choose MariaDB or MySQL](./databases#choose-mariadb-or-mysql).

Adminer and phpMyAdmin show your databases in the browser and are reachable only through the panel's sign-in. Both need a web server and PHP, and both work with MariaDB and MySQL. See [Databases](./databases#phpmyadmin).

## Docker

The **Docker** card installs `docker.io` from your distribution, with its Compose plugin when one is available. The **Docker** menu item then appears.

Ports that Docker containers publish bypass the ufw firewall. The panel therefore publishes container ports on 127.0.0.1 only, and you reach them through a website. See [Docker](./docker).

## Web firewall and ClamAV

- **Web firewall (ModSecurity)** installs ModSecurity with the OWASP Core Rule Set. It needs the web server. After installing, turn it on per website in **Websites** → (site) → **Web firewall (WAF)**.
- **ClamAV antivirus** is an optional extra engine for malware scans. Its virus databases download right after the install, so scans can use it a few minutes later. It suits servers with 2 GB of RAM or more.

See [Security](./security) for both.

## Tools: Composer and WP-CLI {#tools}

The **Tools** group shows two command-line tools that the panel installs when something needs them. Each card shows whether the tool is installed, its version, where it comes from and what uses it. **Update** installs the tool's latest official release. The panel downloads it over HTTPS with the checksum the project publishes next to it, and refuses a download that doesn't match: the installed tool then stays as it was. The new file replaces the old one in a single step.

### Composer

Composer is the PHP package manager. Installing Laravel or Drupal from the App store installs it, and Git deploy scripts and the terminal can run it too.

- **Install** installs your distribution's `composer` package, which gets its security updates with the rest of the system.
- **Update** installs the latest stable release from getcomposer.org into `/usr/local/bin/composer`, checked against its published SHA-256 checksum. That one comes first in `PATH`, so `composer` in the terminal, Git deploy scripts and the panel's own installs of Laravel and Drupal all use it. The distribution's package stays installed.
- **Uninstall** removes both. It is refused while a Git deploy script runs `composer`. Laravel and Drupal websites don't stop it: they only need Composer to install, and keep working without it. Installing one of them again brings Composer back.

### WP-CLI

WP-CLI runs WordPress from the command line. The [WordPress toolkit](./wordpress) uses it for updates, maintenance mode, passwords and staging copies, and installs it the first time it needs it.

- **Install** and **Update** download the latest release from GitHub to `/usr/local/lib/panel/wp-cli.phar`, checked against the SHA-512 checksum the release publishes.
- **Uninstall** is refused while WordPress websites exist.
