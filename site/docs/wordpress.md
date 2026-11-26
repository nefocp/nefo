---
title: WordPress toolkit
description: Check and update your WordPress sites with a database backup first, and use maintenance mode, debug mode, automatic updates, password resets and staging copies.
---

# WordPress toolkit

The WordPress toolkit shows the version and pending updates of each WordPress site, updates them in one click after a database backup, and switches maintenance mode, debug mode and automatic updates. It also resets administrator passwords, makes staging copies and pushes them back to the live site.

It works for the WordPress sites that Nefo installed: from the [App store](./apps), with the **Install WordPress** button of an empty PHP website, or as a staging copy. A WordPress that you uploaded yourself doesn't appear. The **WordPress** item of the left menu shows up once the server has a WordPress site.

The toolkit uses wp-cli, which Nefo downloads on first use from its latest release and checks against the release's published SHA-512 checksum. **Software** → **Tools** shows its version and updates it (see [Software](./software#tools)). wp-cli runs as each site's own Linux user with the site's PHP version, and it doesn't load plugins or themes, so a broken plugin doesn't get in the way.

## Install WordPress

Two ways:

- **App store**: install WordPress or WooCommerce into an empty PHP website. See [App store](./apps).
- **From an empty PHP website**: open the website's **Overview** tab and click **Install WordPress**. Fill in **Site title**, **Admin user name** (avoid `admin`: it is the first name attackers try), **Admin e-mail** and **Admin password**, or leave the password empty to get a strong one. A password you type needs 12 to 128 characters, as in the app store. Click **Install WordPress**.

WordPress needs MariaDB: install it on the **Databases** page first (see [Databases](./databases)). Turn on HTTPS before you install, so WordPress starts with its `https://` address; if you turn it on later, Nefo switches WordPress over for you. Save the login details when they appear: the password is not shown again.

## The site list

Open **WordPress** in the left menu. For each site the table shows:

| Column | What it shows |
|---|---|
| **Website** | The domain and PHP version. Click it to open the website's **WordPress** tab. |
| **Version** | The WordPress version, with "WordPress … available" when a newer one exists, or "Up to date". |
| **Plugins and themes** | How many there are and how many have an update. **Show the … available updates** lists them. |
| **Modes** | **Maintenance**, **Debug** and **Auto-updates** when they are on. |

Nefo checks each site when the page opens. **Check again** at the top checks all sites again; the same item in a site's menu checks only that site. If a site can't be checked, for example because its database is down, the row says "Could not check the site:" and why.

The menu at the end of each row holds the actions below. The same actions and switches are on each website's **WordPress** tab, together with **WP Admin**.

## Update WordPress, plugins and themes

1. Click **Update all** in the site's row, or on the website's **WordPress** tab.
2. Confirm: "Back up the database of example.com, then update WordPress, all plugins and all themes?"

The update runs in the background with a log:

1. Nefo exports the database to `/var/www/example.com/tmp/panel-wp-backup-YYYYMMDD-HHMMSS.sql`. Only the site's user can read the file, and it is not reachable from the web. Nefo keeps the newest three of these files. If the backup fails, nothing is updated.
2. It updates WordPress and its database, then every plugin and every theme.
3. A site that was in maintenance mode stays in it.

If some plugins or themes can't be updated, the log says why and names the database backup. To go back, download the backup from **Files** and import it on the **Databases** page: see [Databases](./databases). For a full copy of the files too, use [Backups](./backups).

## Maintenance mode

Choose **Turn on maintenance mode**, or turn on the **Maintenance mode** switch on the website's **WordPress** tab. Visitors then see WordPress's maintenance page (HTTP 503). It stays on until you turn it off in the panel with **Turn off maintenance mode**.

Maintenance mode is WordPress's own page. To close the whole website at the web server instead, suspend it: see [Websites](./websites).

## Debug mode

Choose **Turn on debug mode**, or turn on the **Debug mode** switch. Nefo sets `WP_DEBUG` and `WP_DEBUG_LOG` in `wp-config.php` and keeps `WP_DEBUG_DISPLAY` off, so errors are written to the logs and never shown to visitors:

- errors of web requests go to `/var/www/example.com/logs/php-error.log`: open the website's **Logs** tab and choose **PHP errors**;
- errors of wp-cli runs go to `/var/www/example.com/logs/wp-debug.log`: open it in **Files**.

Turn it off with **Turn off debug mode** when you are done.

## Automatic updates

**Turn on automatic updates** lets WordPress update everything by itself: WordPress, including major versions, every plugin and every theme.

**Turn off automatic updates** puts WordPress back to its default: it still installs security and minor releases of WordPress itself, and plugins and themes don't update by themselves.

The setting applies to the plugins and themes installed when you turn it on. For plugins and themes you add later, turn it off and on again.

## Reset an administrator password

1. Choose **Reset an administrator password**.
2. Pick the account under **Administrator**. The list shows the users with the administrator role.
3. Choose **Generate a strong password**, or **Type a password** with 12 to 128 characters.
4. Click **Set password**.

Nefo shows the new password once, with a **Sign in** button. The account is signed out on all devices.

## Make a staging copy

A staging copy is a second website with a copy of the site, where you can try changes without touching the live site. The live site keeps running during the copy.

1. Choose **Clone to staging**.
2. Check the **Domain of the staging site**. It is `staging.example.com` unless you change it.
3. Click **Create staging copy**.
4. Point the staging domain's DNS to this server, then turn on HTTPS on its **SSL / HTTPS** tab.

The copy gets:

- a new PHP website with the same PHP version, and its own Linux user;
- a new database with a copy of the content;
- a copy of the files in `public`, with `wp-config.php` pointed at the new database. Cache drop-ins (`object-cache.php`, `advanced-cache.php`) and symbolic links are left out;
- the site's address replaced with the new one everywhere in the database;
- a request to search engines not to index it;
- the same rewrite rules as the live site.

The staging site starts on `http://`, without a certificate. If any step fails, Nefo removes everything it created for the copy.

Nefo refuses to make a copy when:

- MariaDB is not running;
- the staging domain is already used by a website, or its folder exists in `/var/www`;
- `wp-config.php` is not in the site's `public` folder;
- `DB_NAME` in `wp-config.php` is not a plain value in quotes, or the database name uses other characters than lowercase letters, digits and `_`;
- the site's database is on another server;
- the site is a multisite network.

::: tip
Anyone can open the staging site once its DNS points to the server. To keep it private, protect it with a password on its **Block access** tab: see [Websites](./websites).
:::

## Push the staging copy to the live site

When the changes you tried on the staging copy are ready, copy them back over the live site. Nefo remembers which site each staging copy was made from. Copies made before Nefo had this button, and [clones](./websites#clone-a-website) of a website, aren't linked to a live site and can't be pushed.

1. In the staging site's row, choose **Push to live**, or click **Push to live** on its **WordPress** tab. The site list shows "Staging copy of example.com" under the staging site.
2. Choose what to push:
   - **Files and database**;
   - **Files only (themes, plugins, uploads)**: the live site keeps its posts, pages, orders and settings;
   - **Database only (posts, pages, settings)**: the live site keeps its files.
3. Type the live site's domain to confirm, and click **Push to live**.

The push runs as a task with its own log:

1. Nefo backs up the live site's files and database. The backup is on the [Backups](./backups) page. If the backup fails, nothing is pushed.
2. The live site shows its maintenance page. If it was in maintenance mode before, it stays in it after the push.
3. **Files**: the files of the staging site's `public` folder replace those of the live site, and belong to the live site's Linux user. Files the staging copy doesn't have are deleted from the live site; files with the same size and date are left as they are. The live site keeps its own `wp-config.php`, its cache drop-ins (`object-cache.php`, `advanced-cache.php`) and its symbolic links. In `.user.ini` and `wordfence-waf.php`, the staging folder is replaced with the live one.
4. **Database**: the live database is emptied and gets a copy of the staging database. Nefo then reverses what the staging copy did: the staging address (with `http://` and `https://`, also as JSON) and the staging folder are replaced with the live ones everywhere, and the live site gets back its own address and its search engine setting. The object cache is emptied, and WordPress builds its rewrite rules again on the next visit.
5. Maintenance mode is turned off, also when a step failed.

::: warning
A push replaces the live site. What was added to the live site since the staging copy was made is lost: orders, comments and new users when you push the database, uploads when you push the files. If the live site doesn't work after a push, restore the backup from step 1 on the **Backups** page.
:::

Nefo refuses to push when:

- the site is not a staging copy, or its live site was deleted;
- you push the database and MariaDB is not running, the live database is not on the **Databases** page, both sites use the same database, their table prefixes (`$table_prefix`) differ, or a reason in the list above applies to either `wp-config.php`.

Settings in `wp-config.php` are never pushed: change them on the live site yourself.

## Delete a staging copy

To delete the staging copy, delete its website and leave **Also delete the databases linked to this website** ticked: see [Websites](./websites).
