---
title: Databases
description: Install MariaDB or MySQL, choose a MariaDB version, upgrade MariaDB, switch between MariaDB and MySQL, create databases, import and export SQL, allow remote access, read the slow query log, and use Adminer, phpMyAdmin and the Redis key browser.
---

# Databases

Nefo runs one MySQL-compatible database server per server: MariaDB (your distribution's, or 11.4, 11.8 or 12.3 LTS from MariaDB) or MySQL 8.4 LTS, tuned for a small VPS and reachable from the server only. The **Databases** page creates databases and users, imports and exports SQL, opens one database to chosen addresses, and shows slow queries. PostgreSQL can run next to it.

This page covers:

- choosing MariaDB or MySQL and a MariaDB version, installing it, how Nefo tunes it for a small server, upgrading MariaDB, and switching from one to the other;
- creating databases, showing their logins and changing passwords;
- SQL import and export;
- remote access for one database at a time;
- the slow query log;
- Adminer and phpMyAdmin, database UIs in your browser;
- the Redis key browser, on the **Redis** tab of the same page, and Memcached's status.

PostgreSQL has its own **PostgreSQL** tab on this page and its own docs page: see [PostgreSQL](./postgresql). MongoDB, and PostgreSQL as a container, are available in the [App store](./apps) as Docker apps that listen on 127.0.0.1 only. Those Docker apps don't appear on the **Databases** page.

## Choose MariaDB or MySQL

A server runs one MySQL-compatible database server: MariaDB or MySQL 8.4 LTS. You choose when you install it, on the **Databases** page, on the **Software** page's **Database server** card, or in the setup (**Set up this server**). While one is installed, the panel refuses to install the other; you can [switch](#switch-between-mariadb-and-mysql) from one to the other later, with your databases.

| | MariaDB (recommended) | MySQL 8.4 LTS |
|---|---|---|
| Comes from | Your distribution's packages, or an LTS series from MariaDB's own repository (see [MariaDB versions](#mariadb-versions)) | Oracle's repository, `repo.mysql.com` (the `mysql-8.4-lts` track) |
| Memory, roughly | Its buffer pool plus about 100 MB | Its buffer pool plus about 150 MB, or plus about 300 MB with the performance schema on, which the panel turns on from about 3.5 GB of RAM (so on 4 GB servers) |
| Support | As long as your distribution supports it; a series from MariaDB's repository until MariaDB's end date for it | Oracle supports 8.4 LTS until 2032 |
| Systems | Every system Nefo supports | Ubuntu 22.04, 24.04 and 26.04 and Debian 12 and 13, on amd64 (x86-64) only |
| Good for | WordPress and most PHP applications | Applications that need MySQL itself, or a team used to it |

Both work with everything on this page, with backups, website copies, WordPress staging and the app store's PHP applications. Adminer and phpMyAdmin open both. On an arm64 server, or a system Oracle has no packages for, the **Software** page offers MariaDB only and says why.

## Install MariaDB

1. Go to **Databases**.
2. Select **Install MariaDB (recommended)**. You can also install it on the **Software** page: keep **MariaDB** selected on the **Database server** card and select **Install MariaDB**.
3. Wait one to two minutes. The progress log shows each step.

Nefo installs the `mariadb-server` and `mariadb-client` packages from your distribution. It then writes its settings, starts MariaDB, and removes the anonymous users and the `test` database that some versions ship with.

To install a newer series from MariaDB's own repository instead, see [MariaDB versions](#mariadb-versions).

### How MariaDB is tuned

Nefo writes its settings to `/etc/mysql/mariadb.conf.d/90-panel.cnf`. The largest memory setting, the InnoDB buffer pool, depends on the server's RAM:

| Server RAM | `innodb_buffer_pool_size` |
|---|---|
| Less than 1.5 GB | 128M |
| 1.5 GB or more | 256M |
| 3 GB or more | 512M |
| 7 GB or more | 1G |

The other settings are the same on every server:

| Setting | Value | Why |
|---|---|---|
| `bind-address` | `127.0.0.1` | MariaDB only accepts connections from this server |
| `skip-name-resolve` | on | No DNS lookup for each connection |
| `performance_schema` | `OFF` | Saves memory |
| `max_connections` | `100` | |
| `character-set-server` | `utf8mb4` | Full Unicode, emoji included |
| `collation-server` | `utf8mb4_unicode_ci` | |

MariaDB then uses about the size of its buffer pool plus 100 MB. The **Software** page shows this estimate.

::: warning
Don't edit `90-panel.cnf`. The panel rewrites it.
:::

## Which MariaDB versions can I install? {#mariadb-versions}

MariaDB comes from your distribution by default. Its security fixes then arrive with the system's updates and no other repository is added. You can instead install a long-term support (LTS) series from MariaDB's own repository, `dlm.mariadb.com`, when you need a newer MariaDB than your distribution has:

| Series | MariaDB supports it until | Offered on |
|---|---|---|
| 11.4 LTS | 29 May 2029 | Ubuntu 22.04 and 24.04, Debian 12 |
| 11.8 LTS | 4 June 2028 | Ubuntu 22.04 and 24.04, Debian 12 |
| 12.3 LTS | 12 June 2029 | Ubuntu 22.04, 24.04 and 26.04, Debian 12 and 13 |

A series is offered only where it is newer than the distribution's own MariaDB:

| System | The distribution's MariaDB | Series offered |
|---|---|---|
| Ubuntu 22.04 | 10.6 | 11.4, 11.8, 12.3 |
| Ubuntu 24.04 | 10.11 | 11.4, 11.8, 12.3 |
| Debian 12 | 10.11 | 11.4, 11.8, 12.3 |
| Ubuntu 26.04 | 11.8 | 12.3 |
| Debian 13 | 11.8 | 12.3 |

MariaDB 12.3 doesn't install Galera (MariaDB's cluster software, which a single server doesn't use); 11.4 and 11.8 bring it along as their packages require.

To install a series:

1. Go to **Software**. On the **Database server** card, keep **MariaDB** selected.
2. Under it, choose the version, such as **11.4 LTS from MariaDB**. Your distribution's version, such as "10.11, the distribution's", stays the default.
3. Select **Install MariaDB 11.4**.

The setup (**Set up this server**) offers the same versions under **MariaDB**, and so does `nefo setup --db mariadb-11.4` (see [Command line](./cli#nefo-setup)).

What the panel does for a series from MariaDB's repository:

- It downloads MariaDB's signing key from `https://supplychain.mariadb.com/MariaDB-Server-GPG-KEY` (or `keyserver.ubuntu.com`) and accepts it only if its fingerprint is exactly `177F 4010 FE56 CA33 3630 0305 F165 6F24 C74C D1D8`, the "MariaDB Signing Key" that MariaDB's documentation lists. It saves it as `/etc/apt/keyrings/panel-mariadb.gpg`, trusted for this repository only.
- It writes `/etc/apt/sources.list.d/panel-mariadb.list` with one line for that series only, such as `https://dlm.mariadb.com/repo/mariadb-server/11.4/repo/ubuntu noble main`. Your server then gets 11.4's point releases with your system updates, never a newer series by itself.
- It checks that apt will really install MariaDB's packages of that series, then installs `mariadb-server` and `mariadb-client`, tuned like the distribution's MariaDB.

Uninstalling MariaDB also removes the repository file and the key.

The panel refuses:

- to install a series while another MariaDB is installed: use **Upgrade** instead (see below);
- to install a MariaDB older than the one whose data is in `/var/lib/mysql`, kept from an earlier install, because it couldn't read it. Install that version or a newer one, or move the data directory away first.

## Upgrade MariaDB

You can upgrade the installed MariaDB to a newer LTS series from MariaDB's repository, such as the distribution's 10.11 to 11.4, or Debian 13's 11.8 to 12.3, with your databases in place.

1. Go to **Software**. On the **Database server** card, select **Upgrade**.
2. Choose the series. The dialog shows what will happen, and how much free disk space it needs.
3. Select **Upgrade to MariaDB 11.4** and follow the log.

The upgrade runs in this order:

1. Every database is exported, one gzipped SQL file each, into a new folder of `/var/backups/panel/db-server/`. The panel keeps these dumps until you delete them.
2. The packages of the new series are downloaded, and those of the installed one are kept, so going back works without the network.
3. MariaDB becomes read-only, the panel counts the rows of every table, and MariaDB stops. Its data directory is copied to `/var/lib/mysql.before-11.4-<date>-<time>` and `/etc/mysql` into the dumps' folder. From here, websites can't use their databases until the upgrade ends, usually a few minutes.
4. The new packages replace the old ones and start MariaDB on the same data, still read-only. `mariadb-upgrade` then updates its system tables.
5. The panel checks that MariaDB reports the new version, that every database has the same tables with the same number of rows, views, routines, triggers and events, and that each database user of the panel can sign in with its password.
6. MariaDB accepts writes again, and the copy of the data directory is deleted.

If any step from 4 on fails, the panel puts the old MariaDB back: it removes the new packages, restores `/etc/mysql` and the copied data directory, installs the old packages again and starts MariaDB. The log ends with the reason. Your databases are then as they were before the upgrade.

::: warning No way back to an older series
After an upgrade, MariaDB has upgraded its data files and system tables, which an older MariaDB can't read. The panel refuses a downgrade, such as 11.8 to 11.4, and says why. To go back, export the databases, uninstall MariaDB with its data, install the older version and import them.
:::

The panel needs free disk space for the dumps, the copy of the data directory and the packages: roughly one and a half times the data directory plus 400 MB. It refuses to start without it. It also refuses while MariaDB is stopped, or when the server's configuration moves the data directory away from `/var/lib/mysql`.

## Install MySQL

1. Go to **Databases** and select **Install MySQL 8.4 instead**. On the **Software** page, choose **MySQL 8.4 LTS** on the **Database server** card and select **Install MySQL 8.4 LTS**.
2. Wait two to four minutes. The progress log shows each step.

Nefo adds Oracle's repository itself, instead of installing Oracle's `mysql-apt-config` package:

- It downloads the signing key from `https://repo.mysql.com/RPM-GPG-KEY-mysql-2025` and accepts it only if its fingerprint is exactly `BCA4 3417 C3B4 85DD 128E C6D4 B7B3 B788 A8D3 785C`, the MySQL Release Engineering key that MySQL's documentation lists. It saves the key as `/etc/apt/keyrings/panel-mysql.gpg`.
- `/etc/apt/sources.list.d/panel-mysql.list` trusts that key for this repository only, and lists only the `mysql-8.4-lts` track.
- Oracle renews the key from time to time; the current copy expires on 23 October 2027. Once a week the panel looks for a renewed copy of the same key and replaces the old one, so `apt-get update` keeps working.

Then it installs `mysql-community-server` and `mysql-community-client` without questions. MySQL's `root` account has no password: it signs in through the local socket only (`auth_socket`), as root on the server, and the panel stores no root password. Last, the panel removes any anonymous user and the `test` database.

### How MySQL is tuned

Nefo writes `/etc/mysql/mysql.conf.d/90-panel.cnf` before the package first starts MySQL, so MySQL never runs with Oracle's defaults on your server. The memory settings depend on the server's RAM:

| Server RAM | `innodb_buffer_pool_size` | `max_connections` | `table_open_cache` | `temptable_max_ram` |
|---|---|---|---|---|
| Less than 1.5 GB | 128M | 75 | 400 | 32M |
| 1.5 GB or more | 256M | 100 | 600 | 64M |
| 3 GB or more | 512M | 150 | 1000 | 128M |
| 7 GB or more | 1024M | 200 | 2000 | 256M |

The other settings:

| Setting | Value | Why |
|---|---|---|
| `bind-address` | `127.0.0.1` | MySQL only accepts connections from this server, until a database allows remote access |
| `mysqlx` | `OFF` | No X Protocol listener on port 33060: PHP and the panel use the classic protocol |
| `disable_log_bin` | on | No binary logs filling the disk: the server has no replicas |
| `performance_schema` | `OFF` below about 3.5 GB of RAM, `ON` from there (so on 4 GB servers) | It takes 100 to 200 MB by itself |
| `skip-name-resolve`, `host_cache_size` | on, `0` | No DNS lookup for each connection |
| `innodb_log_buffer_size` | `16M` | MySQL 8.4's default is 64M |
| `character-set-server`, `collation-server` | `utf8mb4`, `utf8mb4_unicode_ci` | The same as MariaDB, so dumps move between the two |

MySQL then uses about the size of its buffer pool plus 150 MB, or plus 300 MB with the performance schema on. The **Software** page shows this estimate; the **Services** page shows what it really uses.

Users sign in with `caching_sha2_password`, MySQL 8.4's default, which PHP's `mysqli` and PDO support. `mysql_native_password` stays turned off, as MySQL 8.4 ships it: it hashes with unsalted SHA-1, and MySQL 9 removed it. So an old password hash from another server, as cPanel keeps them, can't be reused on MySQL: a database brought over needs its password, or a new one.

On MySQL the panel runs SQL with the `mysql` command instead of `mariadb`, exports with `mysqldump`, and the service is called `mysql`.

::: warning
Don't edit `90-panel.cnf`. The panel rewrites it.
:::

## Can I switch between MariaDB and MySQL? {#switch-between-mariadb-and-mysql}

Yes, from v0.24.0. A server runs one database server, but you can replace MariaDB with MySQL 8.4 LTS, or MySQL with MariaDB, with your databases, their users and their privileges.

1. Go to **Software**. On the **Database server** card, under **MySQL 8.4 LTS instead** (or **MariaDB instead**), select **Switch to MySQL 8.4 LTS** (or **Switch to MariaDB**).
2. Read the dialog. It lists the databases and the accounts that go over, and how each account signs in afterwards.
3. Type `mysql` (or `mariadb`) to confirm, and select the switch button.

Switching back to MariaDB installs your distribution's MariaDB. Without any database, the same task only replaces one server with the other.

### What the switch does

1. The panel downloads the new server's packages, and keeps the old server's for going back. Nothing changes for your websites yet.
2. The old server becomes read-only: websites can still read their databases, but not write. The panel reads its accounts and privileges, counts the rows of every table and exports every database into a new folder of `/var/backups/panel/db-server/`.
3. The old server stops. Its data directory is moved to `/var/lib/mysql.mariadb-<date>-<time>` (or `.mysql-…`) and `/etc/mysql` is copied into the dumps' folder. Its packages are removed. From here, websites can't use their databases until the switch ends.
4. The new server is installed as the **Software** page installs it, and stays read-only. Remote access and the slow query log are set up again as they were.
5. The accounts are created, every database is imported, and the privileges are granted again. Dumps from MySQL 8 are adapted for MariaDB as the website import adapts them: its `utf8mb4_0900_*` collations become MariaDB's (`utf8mb4_unicode_520_ci`, a language's own collation, or `utf8mb4_bin`) and MySQL-only settings are left out. Dumps from MariaDB 11 going into MySQL get MySQL's collations for MariaDB's `uca1400` ones (`utf8mb4_uca1400_ai_ci` becomes `utf8mb4_0900_ai_ci`).
6. The panel checks that every database has the same tables with the same number of rows, views, routines, triggers and events, and that each database user of the panel signs in with its password. Then the server accepts writes again and the old data directory is deleted.

If any step from 3 on fails, the panel removes the new server, restores the old one's configuration and data directory, installs its packages again and starts it: you are back where you were. The dumps stay in either case.

How long it takes depends on the size of your databases: the export, the import and the row count each read all the data once.

### Accounts and passwords

MariaDB and MySQL store passwords differently, so not every password can go over:

| Account | After the switch |
|---|---|
| The user of a database on the **Databases** page | Its password, which the panel knows, on `localhost` and on each address allowed for remote access |
| An account you made yourself with a `mysql_native_password` password, going to MariaDB | Its password, from its hash |
| An account that signs in through the socket (`unix_socket` on MariaDB, `auth_socket` on MySQL) | Signs in through the socket again |
| Any other account you made yourself, such as a MariaDB account going to MySQL (MySQL 8.4 keeps `mysql_native_password` off) or a MySQL account going to MariaDB (MariaDB can't read `caching_sha2_password`) | Created with a new random password and locked. The log lists each one. Set a password as root on the server: `ALTER USER 'name'@'localhost' IDENTIFIED BY '…' ACCOUNT UNLOCK;` |

Their privileges are granted again from the old server's `SHOW GRANTS`. A privilege the other server doesn't have is left out and listed in the log, as are role and proxy grants, which the panel doesn't carry.

### What keeps working

- Websites keep their settings: they connect to `localhost`, through the same socket on both servers.
- phpMyAdmin and Adminer open the new server.
- Backups: new backups use the new server's export program, and a backup made before the switch can be restored after it.
- Remote access stays open for the same addresses.

### When the panel refuses

The dialog lists what must be fixed first, under **Fix these first:**, and the switch button stays disabled:

| Reason | What to do |
|---|---|
| The panel doesn't know the password of a database's user (a database brought over with only its password hash) | Set a new password for that database on the **Databases** page, and put it in the website's settings |
| Going to MySQL: tables that use one of MariaDB's own storage engines (Aria and others), sequences or system-versioned tables | Convert them, for example with `ALTER TABLE … ENGINE=InnoDB`, or drop them |
| Going to MySQL: `INET4`, `INET6` or `UUID` columns | Change them, for example to `VARCHAR` |
| Going to MySQL: views, triggers or routines whose definer's user name is longer than 32 characters | Create them again with another definer |
| A database name that starts with a dash | Rename the database |
| Not enough free disk space: the dumps and the imported data next to the old data directory need roughly twice your databases' size plus 700 MB | Free some space; delete old dumps |

The switch is also refused while the server is stopped, when the server's configuration moves the data directory away from `/var/lib/mysql`, and, for MySQL, where MySQL isn't offered (arm64, or a system Oracle has no packages for).

Some differences only show when the data is imported, such as an index MariaDB marks `IGNORED`, which MySQL doesn't know. The import then fails and the panel goes back to the old server, with the error in the log.

## Dumps from upgrades and switches

The **Database server** card on the **Software** page lists the folders under **Dumps kept**: when, what (such as "MariaDB 10.11.14 → MariaDB 11.4"), whether it was **Done**, **Went back** or **Failed**, how many databases, their size and the folder. Each database is one file, `001.sql.gz`, `002.sql.gz` and so on, with `manifest.json` naming the database of each file, because database names can hold characters a file name can't. The folder is readable by root only.

To bring one database back, unzip its file and import it on the **Databases** page. To delete a folder, select **Delete these dumps** (the bin next to it). The panel refuses while an upgrade or a switch runs.

## Create a database

1. Go to **Databases** and select **Add database**.
2. Enter a name in **Database**. A user with the same name is created, and that user can use only this database.
3. Leave **Password** empty to get a strong random password, or type your own.
4. Optional: choose a site in **Website**. A backup of that website then includes this database.
5. Select **Create database**.

A card titled **Connection details for** followed by the database name then shows **Database**, **User**, **Password** and **Host**. The host is always `localhost`.

The rules for names and passwords:

- Names start with a letter and use only `a-z`, `0-9` and `_`, at most 32 characters. These names are reserved: `mysql`, `information_schema`, `performance_schema`, `sys`, `test`, `root`, `mariadb`, `phpmyadmin`, `debian_sys_maint`. The panel also refuses a name that is already used by a database or a database user in MariaDB.
- Passwords have 12 to 64 characters, made of letters, digits and `! # % * + , . : = ? @ ^ _ ~ -`. A generated password has 24 characters.

New databases use `utf8mb4` and `utf8mb4_unicode_ci`.

### Users and privileges

Each database has one user, `name@localhost`, with all privileges on that database and none on others. With remote access on, that user also gets a copy for each allowed address (see [Allow remote access](#allow-remote-access)). Apart from the temporary user that runs imports (see [Import](#import)), the panel doesn't create extra users or change privileges. If you need that, use the `mariadb` command (`mysql` on MySQL) on the server as root.

The list shows the databases the panel created. A database that was removed outside the panel is marked **Not found in MariaDB** (or **Not found in MySQL**).

## Show a database's login

Select **Show login** on the database's row. The connection details card shows the name, user, password and host, each with a copy button. If Adminer or phpMyAdmin is installed, the card also has **Open in Adminer** or **Open in phpMyAdmin**.

## Change a database password

1. Open the menu of the database and select **New password**.
2. Keep **Generate a strong random password**, or choose **Use my own password** and type it. Your own password follows the same rules as when you [create a database](#create-a-database).
3. Select **New password**.

The panel sets the new password and shows it in the connection details card. If the database is linked to a WordPress website that the panel created, the panel also writes the new password into that site's `wp-config.php`. Other applications stop connecting until you update their settings.

If the database allows remote access, the remote users get the new password too.

## Export and import

### Export

Open the menu of the database and select **Export (.sql)**. Your browser downloads a plain SQL file named like `shop_20261126-143000.sql`.

The dump is made with `mariadb-dump --single-transaction` (`mysqldump` on MySQL, also with `--set-gtid-purged=OFF`), so the website keeps running during the export. It includes stored routines and triggers.

### Import

1. Open the menu of the database and select **Import**.
2. Choose a file in **SQL file (.sql or .sql.gz)**. A `.sql.gz` file is unpacked on the fly.
3. Optional: tick **Empty the database first (delete all of its tables)**. The database is then dropped and created again before the import, and its user keeps access.
4. Select **Import**.

One import can be up to 4 GB.

A dump made by the other server, such as a backup from before a [switch](#switch-between-mariadb-and-mysql), imports too: the panel rewrites the collation names that this server doesn't know while the file streams in (MySQL 8's `utf8mb4_0900_*` on MariaDB, MariaDB 11's `uca1400` ones on MySQL). On MySQL it also drops `NO_AUTO_CREATE_USER`, which MariaDB writes into the SQL mode of each trigger and routine and MySQL 8 refuses. Nothing else in the file changes; a dump that uses another database is still refused. Restoring a database backup works the same way.

The import runs as a temporary MariaDB user, `nefo-import-` followed by the database name, with all privileges on this database and none on others. It may also set the definer of triggers, views and stored routines, so dumps that contain them, such as the panel's own exports and backups, import as before. The panel deletes this user when the import ends, also when it fails. Only when the file created triggers, views, routines or events without a `DEFINER` clause, which then run as this user, does the panel keep it: locked, so nobody can log in with it, and with privileges on this database only. Deleting the database deletes it too.

On MySQL, user names stop at 32 characters, so for a long database name the user's name is shortened and a short hash added. The definers are allowed there with `SET_ANY_DEFINER` and `ALLOW_NONEXISTENT_DEFINER` instead of MariaDB's `SET USER`; without `SYSTEM_USER`, the user still can't make an object run as root. The first line of dumps from newer MariaDB versions, `/*M!999999\- enable the sandbox mode */`, which MySQL's client refuses, is skipped on MySQL.

::: warning
If the file uses another database, for example with `USE other_db`, `CREATE DATABASE other_db` or `INSERT INTO other_db.posts`, MariaDB refuses that statement and the import stops with a message that begins "The SQL file uses another database, other_db." What the file did before that line stays in the database. Remove the `CREATE DATABASE` and `USE` lines from the file, or export the database again without `--databases`, then import it again with **Empty the database first (delete all of its tables)** ticked. `CREATE DATABASE` and `USE` lines that name this same database are fine.
:::

## Allow remote access

By default MariaDB only accepts connections from the server itself. You can let chosen addresses connect to one database, for example a reporting tool on your office machine.

Before you start, install and turn on the ufw firewall on the **Firewall & SSH** page. The panel refuses remote access while ufw isn't installed.

1. Open the menu of the database and select **Remote access**.
2. Under **Allowed addresses**, enter an address. Select **Add an address** for more.
3. Select **Save**.

Connect with the database's user and password to the server's public IP address, port 3306.

### Allowed addresses

- Single IPv4 or IPv6 addresses, such as `203.0.113.10` or `2001:db8::1`.
- IPv4 ranges in CIDR form, such as `198.51.100.0/24`, no wider than `/16`.
- IPv6 ranges aren't possible, because MariaDB can't match them. Enter single IPv6 addresses.
- At most 10 addresses per database.
- Loopback, `0.0.0.0` and multicast addresses are refused.

### What the panel changes

| Where | Change |
|---|---|
| MariaDB users | For each address, a user `name@address` with the same password and the same privileges as the local user |
| Firewall (ufw) | For each address, a rule that allows port 3306/tcp from that address only, with the comment `MariaDB name` |
| MariaDB settings | `/etc/mysql/mariadb.conf.d/99-panel-remote.cnf` makes MariaDB listen on all interfaces (`0.0.0.0`, or `::` when an IPv6 address is allowed). The file exists only while at least one database allows remote access |

MySQL works the same way: the file is `/etc/mysql/mysql.conf.d/99-panel-remote.cnf`, the rules' comment is `MySQL name`, and a remote user is dropped and created again when it changes, as MySQL has no `CREATE OR REPLACE USER`. The messages below then say MySQL.

MariaDB restarts the first time remote access is turned on, and again when it is turned off for the last database. Websites lose their database for a few seconds. If another task is running at that moment, the dialog says "MariaDB has to restart before it accepts remote connections." Select **Restart MariaDB** when the task has finished.

If MariaDB doesn't start with the remote access file, the panel removes the file and starts MariaDB without it, so your websites keep working.

::: warning
- If the firewall is turned off, port 3306 is open to everyone. MariaDB still accepts only the listed addresses.
- Connections aren't encrypted unless the client asks for TLS. For sensitive data, use an SSH tunnel or a VPN.
- List only addresses you control that don't change.
:::

To turn remote access off, open **Remote access** again and select **Turn off**. Deleting a database also removes its remote users and its firewall rules.

## Find slow queries

MariaDB can log every query that takes longer than a threshold. This helps you find what slows a website down.

1. Go to **Databases** → **Slow queries**. The tab appears once MariaDB is installed.
2. Set **Slower than** to a number of seconds, from 0.1 to 60.
3. Turn on the **Log slow queries** switch and select **Apply**.

MariaDB must be running to change these settings. The change applies at once, to connections opened after it, and stays after a restart: the panel keeps it in `/etc/mysql/mariadb.conf.d/99-panel-slowlog.cnf`, and the log is written to `/var/log/mysql/mariadb-slow.log`. On MySQL, the settings are in `/etc/mysql/mysql.conf.d/99-panel-slowlog.cnf` and the log is `/var/log/mysql/mysql-slow.log`.

The panel rotates the log so that it can't fill the disk, on MariaDB and on MySQL. Once an hour it looks at it, and when the log is larger than 50 MB, or its oldest entry is 7 days old, the panel renames it, has the server open a new one (`FLUSH SLOW LOGS`, without a restart) and compresses the old one next to it, as `mariadb-slow.log-YYYYMMDD-HHMMSS.gz` (`mysql-slow.log-…` on MySQL). It keeps the 4 newest of those and deletes older ones. The tab shows how many it keeps now. The table only reads the current log.

The table shows the 200 newest entries, newest first, read from the last 4 MB of the log:

| Column | Meaning |
|---|---|
| **Time** | When the query ran |
| **User and host** | Who ran it, such as `shop@localhost` |
| **Database** | The database it used |
| **Duration** | How long it took, in seconds |
| **Rows examined** | How many rows MariaDB read |
| **Query** | The query. Select it to see all of it, up to 2,000 characters. Longer queries are marked **Shortened** |

Select **Refresh** to read the log again, and **Clear log** to empty it. **Clear log** empties the current log only; the compressed old logs stay until the rotation deletes them.

::: tip
Logging adds a little work for the server. Turn the switch off and select **Apply** when you are done.
:::

## Adminer

Adminer lets you browse and edit your databases in the browser, like phpMyAdmin.

### Install Adminer

1. Install a web server first, on the **Software** page. Adminer runs on it with PHP.
2. Go to **Software** → **Adminer (database UI)** and select **Install**.

The panel downloads Adminer 6.3.1 (the build with all of Adminer's drivers) and checks it against a pinned SHA-256 checksum before installing it. It opens MariaDB or MySQL databases, and PostgreSQL databases once PostgreSQL is installed (see [PostgreSQL in Adminer](./postgresql#postgresql-in-adminer)). Its SQLite driver refuses every sign-in, because SQLite has no password.

### Open Adminer

1. Select **Open in Adminer** at the top of the **Databases** page, or in a database's connection details card. **Software** → **Open Adminer** does the same.
2. Adminer opens in a new tab, at your panel address followed by `adminer/`, for example `https://203.0.113.10:28443/k3x9p2qa7m1z/adminer/`.
3. Sign in to Adminer with the database's user and password from **Show login**.

### How Adminer is protected

Adminer is reachable only through the panel's sign-in:

- It runs on an internal site of your web server that listens on 127.0.0.1 only, on a random port between 40000 and 59999.
- That internal site answers only requests that carry a secret token. Other requests get `404`.
- The panel adds the token only to requests from a signed-in panel session. Without a panel session, `adminer/` answers `401` and the request never reaches Adminer.
- The panel's own session cookie is never passed to Adminer.

So Adminer can't be reached from the internet, and other programs on the server can't use it without the token.

Adminer can import files of up to 256 MB, and a request can run for up to 600 seconds. For larger dumps, use **Import** on the **Databases** page.

Uninstalling Adminer removes its files and its internal site. Your databases aren't touched. Uninstalling the web server also removes Adminer.

## phpMyAdmin

phpMyAdmin is the other database UI, for MariaDB and MySQL. Install the one you prefer, or both: they don't share files.

### Install phpMyAdmin

1. Install a web server first, on the **Software** page. phpMyAdmin runs on it with PHP.
2. Go to **Software** → **phpMyAdmin (database UI)** and select **Install**.

The panel downloads phpMyAdmin 5.2.3 (all languages) from `files.phpmyadmin.net` and checks it against a pinned SHA-256 checksum before it unpacks it into `/usr/share/nefo-phpmyadmin`. It leaves out the setup script, the examples and the local documentation (the help links open docs.phpmyadmin.net instead). phpMyAdmin keeps its temporary files in `/var/lib/nefo-phpmyadmin/tmp`.

The panel writes phpMyAdmin's `config.inc.php`, with a random cookie encryption key:

- You sign in with a database's user and password, to this server's database only. You can't choose another server, sign in without a password, or sign in as `root`.
- phpMyAdmin doesn't check for new versions or send error reports.
- Its password change form is hidden. Change a database's password on the **Databases** page, which also updates a WordPress website's `wp-config.php`.

### Open phpMyAdmin

1. Select **Open in phpMyAdmin** at the top of the **Databases** page, in a database's menu, or in the connection details card. **Software** → **Open phpMyAdmin** does the same.
2. phpMyAdmin opens in a new tab, at your panel address followed by `phpmyadmin/`, for example `https://203.0.113.10:28443/k3x9p2qa7m1z/phpmyadmin/`.
3. Sign in with the database's user and password from **Show login**.

phpMyAdmin is protected like Adminer (see [How Adminer is protected](#how-adminer-is-protected)), on its own internal site with its own secret token. Without a panel session, `phpmyadmin/` answers `401`, and an API key can't reach it either. Only phpMyAdmin's three entry points run as PHP and only its scripts, styles and images are served as files; anything else answers `404`, the setup script included. phpMyAdmin sends its own Content-Security-Policy and checks its own form tokens.

Like Adminer, phpMyAdmin can import files of up to 256 MB, and a request can run for up to 600 seconds. Uninstalling phpMyAdmin removes its files, its internal site and its settings. Your databases aren't touched. Uninstalling the web server also removes phpMyAdmin.

## Redis key browser

When Redis is installed, the **Databases** page has a **Redis** tab. To install Redis, go to **Software** → **Redis** → **Install**. See [Software](./software#redis) for how it is set up.

The tab shows the Redis version, how long it has been running, **Memory used** (with the memory limit) and the number of **Keys**. If it says **Redis is installed but not running.**, start Redis on the **Services** page.

### Find keys

1. Choose a database from 0 to 15. The list shows how many keys each one has.
2. Enter a pattern. `*` matches anything and `?` matches one character, for example `user:*`.
3. Select **Search**.

The search goes through the keys in steps of about 100. If there are more, select **Continue searching**.

### View a key

Select a key to see its **Type**, **Expires in** and **Size**, and its value:

| Type | What the panel shows |
|---|---|
| string | The first 64 KB of the value |
| list, hash, set, sorted set | The first 200 items. Each item is cut at 4 KB |
| stream and other types | The type and expiry only |

Characters that can't be shown in binary values are replaced.

### Delete keys

- To delete one key, select it and then **Delete key**.
- To delete every key of the chosen database, select **Empty database 0** (the button shows the database's number), type the number to confirm, and select **Delete all keys**.

Websites that use Redis as a cache rebuild it. Data stored only in Redis is lost.

The browser doesn't edit values or expiry times. Use `redis-cli` on the server for that.

If Redis has a password (`requirepass` in `/etc/redis/redis.conf`), the panel reads it from that file and signs in with it.

## Delete a database

1. Open the menu of the database and select **Delete**.
2. Type the database's name to confirm and select **Delete**.

::: danger
This deletes the database, all of its tables and its user. It can't be undone. Its remote users, firewall rules and automatic backup schedules go with it. Backups already made are kept on the **Backups** page.
:::

When you delete a website, its databases are kept unless you tick **Also delete its databases**: they stay here, linked to no website. Restoring the website's backup links them again.

## Uninstall MariaDB

Uninstall MariaDB on the **Software** page. The panel refuses while MariaDB holds any database other than its own system databases, including databases created outside the panel. See [Software](./software) for what an uninstall keeps.

A MariaDB from MariaDB's repository goes with its repository file and key.

MySQL is uninstalled the same way, on its card, with the same check. The panel also removes Oracle's repository and its key. The data directory `/var/lib/mysql` stays unless you tick the box and type `mysql`.

To replace one with the other while keeping your databases, [switch](#switch-between-mariadb-and-mysql) instead.

## PostgreSQL

PostgreSQL runs next to MariaDB or MySQL, on its own **PostgreSQL** tab of this page: your distribution's version or 15 to 19 from the PostgreSQL project, with databases and owners, import and export, remote access over TLS, backups and major upgrades. See [PostgreSQL](./postgresql).

## Memcached

When Memcached is installed (**Software** → **Memcached**), the **Databases** page has a **Memcached** tab. It shows the version, how long it has been running, **Memory used** with the limit, the number of **Items**, the hit rate and the open connections. It also shows what Memcached itself reports: that it listens on 127.0.0.1 only and that UDP is off.

**Restart** empties the cache. See [Software](./software#memcached) for how it is set up.
