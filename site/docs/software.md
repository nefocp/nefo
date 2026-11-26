---
title: Software
description: Install and uninstall the web server, PHP versions, MariaDB, Redis, Adminer, Node.js, Python, Docker, the web firewall and ClamAV, and manage PHP extensions and php.ini settings.
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
| **PHP versions** | PHP 7.4, 8.0, 8.1, 8.2, 8.3, 8.4 and 8.5 | about 25 MB each |
| **Databases and cache** | MariaDB | its buffer pool plus about 100 MB (see [Databases](./databases#how-mariadb-is-tuned)) |
| | Redis | up to 64, 128 or 256 MB, depending on the server's RAM |
| | Adminer (database UI) | none while idle |
| **Runtimes** | Node.js | none while idle |
| | Python apps | none while idle |
| **Containers** | Docker | about 90 MB, plus your containers |
| **Security** | Web firewall (ModSecurity) | about 40 MB |
| | ClamAV antivirus | about 15 MB, and about 1.4 GB of free memory while it scans |

## Install an item

1. Go to **Software**.
2. Select **Install** on the item's card.
3. Follow the log. When the task has finished, the card shows **Installed** and its version.

Some cards then offer a link: **Open Databases**, **Open Redis**, **Open Adminer** or **Open Docker**.

## Uninstall an item

1. Select **Uninstall** on the item's card. For a PHP version, it is in the version's menu.
2. Read what the dialog says the uninstall does.
3. For MariaDB and Docker, you can tick the box to delete their data too. You must then type the item's name to confirm.
4. Select **Uninstall**.

### When an uninstall is refused

The panel never removes software that something still uses. When an item can't be uninstalled, its card shows the reason instead of a working button.

| Item | Refused while |
|---|---|
| Web server | Any website exists, in the panel or as a configuration file in the web server's sites folder |
| A PHP version | A website uses it, or it is the only PHP version installed (the web server's setup and Adminer need one) |
| MariaDB | MariaDB holds any database other than its own system databases. The panel asks MariaDB itself, so databases made by hand or by another tool count too. If MariaDB is stopped and its data folder was moved, the panel can't check and refuses |
| Node.js | App websites use the server's default Node.js, or the `node` in `/usr/local/bin` wasn't installed by the panel |
| A line under **Node.js versions** | It is the server's default, or an app website chose it |
| Python apps | App websites run Python |
| Docker | Websites run Docker apps, Compose projects exist, or Docker has containers, running or stopped |

Redis, Adminer, the web firewall and ClamAV can always be uninstalled.

### What an uninstall keeps

| Item | Kept | Removed |
|---|---|---|
| Web server | Website folders in `/var/www`, certificates, PHP | The web server, its panel configuration, Adminer, and the firewall rules for ports 80 and 443 |
| PHP version | | The version, its extensions and the libraries only they needed |
| MariaDB | The data folder `/var/lib/mysql`, unless you tick **Also delete the data directory (/var/lib/mysql)** and type `mariadb`. Installing MariaDB again uses it | The server and client packages and the panel's MariaDB settings |
| Redis | | Redis, its PHP extension, and everything stored in Redis. PHP-FPM restarts |
| Adminer | Your databases | Adminer's files and its internal site |
| Node.js | Other Node.js versions. The default's folder too, if an app website chose that version | `node`, `npm` and `npx` in `/usr/local/bin`, and the release they point to |
| Python apps | `python3` itself, which the system needs, and the virtual environments in old app folders | The venv and pip packages |
| Docker | Images, volumes and build cache in `/var/lib/docker`, unless you tick **Also delete all images, volumes and build cache (/var/lib/docker)** and type `docker`. Installing Docker again finds them | Docker and its Compose plugin |
| Web firewall | The firewall's log | ModSecurity and the OWASP rules. The web server restarts |
| ClamAV | | ClamAV and its virus databases |

::: danger
Deleting the data of MariaDB or Docker can't be undone. Make sure you have a backup of anything you still need.
:::

## The menu shows what is installed

The left menu hides pages for software that isn't installed:

- **Docker** appears once Docker is installed.
- **WordPress** appears once a WordPress website exists.
- On the **Databases** page, the **Redis** tab appears once Redis is installed, and **Slow queries** once MariaDB is installed.

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

- The version your distribution ships comes from its packages. Other versions come from the `ondrej/php` repository on Ubuntu, or `packages.sury.org` on Debian. The panel asks before it adds that repository to the server.
- Each version is installed with these extensions: fpm, cli, mysql, curl, gd, mbstring, xml, zip, intl, bcmath and opcache. When Redis is installed, the redis extension is added too.
- The panel switches the version's default PHP-FPM pool to on-demand workers, so a version no website uses costs almost no memory.

Select **Manage** on an installed version to open its **Extensions** and **Settings**.

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

## MariaDB and Adminer

MariaDB is the database server, tuned to the server's memory and reachable from this server only. Adminer shows your databases in the browser and is reachable only through the panel's sign-in. Adminer needs a web server and PHP. See [Databases](./databases).

## Docker

The **Docker** card installs `docker.io` from your distribution, with its Compose plugin when one is available. The **Docker** menu item then appears.

Ports that Docker containers publish bypass the ufw firewall. The panel therefore publishes container ports on 127.0.0.1 only, and you reach them through a website. See [Docker](./docker).

## Web firewall and ClamAV

- **Web firewall (ModSecurity)** installs ModSecurity with the OWASP Core Rule Set. It needs the web server. After installing, turn it on per website in **Websites** → (site) → **Web firewall (WAF)**.
- **ClamAV antivirus** is an optional extra engine for malware scans. Its virus databases download right after the install, so scans can use it a few minutes later. It suits servers with 2 GB of RAM or more.

See [Security](./security) for both.
