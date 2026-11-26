---
title: Databases
description: Install MariaDB or MySQL, create databases, import and export SQL, allow remote access, read the slow query log, use Adminer, phpMyAdmin and the Redis key browser, and manage PostgreSQL databases.
---

# Databases

The **Databases** page manages the database server for WordPress and most PHP applications: MariaDB, or MySQL 8.4 if you choose it. This page covers:

- choosing MariaDB or MySQL, installing it and how Nefo tunes it for a small server;
- creating databases, showing their logins and changing passwords;
- SQL import and export;
- remote access for one database at a time;
- the slow query log;
- Adminer and phpMyAdmin, database UIs in your browser;
- the Redis key browser, on the **Redis** tab of the same page;
- PostgreSQL, on its own **PostgreSQL** tab, and Memcached's status.

PostgreSQL can run next to MariaDB, installed from your distribution (see [PostgreSQL](#postgresql)). MongoDB, and PostgreSQL as a container, are available in the [App store](./apps) as Docker apps that listen on 127.0.0.1 only. Those Docker apps don't appear on the **Databases** page.

## Choose MariaDB or MySQL

A server runs one MySQL-compatible database server: MariaDB or MySQL 8.4 LTS. You choose when you install it, on the **Databases** page or on the **Software** page's **Database server** card. While one is installed, the panel refuses to install the other.

| | MariaDB (recommended) | MySQL 8.4 LTS |
|---|---|---|
| Comes from | Your distribution's packages | Oracle's repository, `repo.mysql.com` (the `mysql-8.4-lts` track) |
| Memory, roughly | Its buffer pool plus about 100 MB | Its buffer pool plus about 150 MB; plus about 300 MB from about 4 GB of RAM, where the performance schema is on |
| Support | As long as your distribution supports it | Oracle supports 8.4 LTS until 2032 |
| Systems | Every system Nefo supports | Ubuntu 22.04, 24.04 and 26.04 and Debian 12 and 13, on amd64 (x86-64) only |
| Good for | WordPress and most PHP applications | Applications that need MySQL itself, or a team used to it |

Both work with everything on this page, with backups, website copies, WordPress staging and the app store's PHP applications. Adminer and phpMyAdmin open both. On an arm64 server, or a system Oracle has no packages for, the **Software** page offers MariaDB only and says why.

::: warning Switching later
The panel doesn't switch a server from one to the other. To switch, export every database (or make a backup), delete the databases, uninstall the current server, install the other one, then create the databases again and import the dumps. The two can't read each other's data directory: the panel refuses to install one over the other's data kept in `/var/lib/mysql`, so move or delete it first. A dump moves between the two as long as it uses nothing the other lacks; MariaDB 11's `utf8mb4_uca1400_ai_ci` collation, for example, doesn't exist in MySQL.
:::

## Install MariaDB

1. Go to **Databases**.
2. Select **Install MariaDB (recommended)**. You can also install it on the **Software** page: keep **MariaDB** selected on the **Database server** card and select **Install MariaDB**.
3. Wait one to two minutes. The progress log shows each step.

Nefo installs the `mariadb-server` and `mariadb-client` packages from your distribution. It then writes its settings, starts MariaDB, and removes the anonymous users and the `test` database that some versions ship with.

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
| `performance_schema` | `OFF` below about 3.5 GB of RAM, `ON` above | It takes 100 to 200 MB by itself |
| `skip-name-resolve`, `host_cache_size` | on, `0` | No DNS lookup for each connection |
| `innodb_log_buffer_size` | `16M` | MySQL 8.4's default is 64M |
| `character-set-server`, `collation-server` | `utf8mb4`, `utf8mb4_unicode_ci` | The same as MariaDB, so dumps move between the two |

MySQL then uses about the size of its buffer pool plus 150 MB, or plus 300 MB with the performance schema on. The **Software** page shows this estimate; the **Services** page shows what it really uses.

Users sign in with `caching_sha2_password`, MySQL 8.4's default, which PHP's `mysqli` and PDO support. `mysql_native_password` stays turned off, as MySQL 8.4 ships it: it hashes with unsalted SHA-1, and MySQL 9 removed it. So an old password hash from another server, as cPanel keeps them, can't be reused on MySQL: a database brought over needs its password, or a new one.

On MySQL the panel runs SQL with the `mysql` command instead of `mariadb`, exports with `mysqldump`, and the service is called `mysql`.

::: warning
Don't edit `90-panel.cnf`. The panel rewrites it.
:::

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

The import runs as a temporary MariaDB user, `nefo-import-` followed by the database name, with all privileges on this database and none on others. It may also set the definer of triggers, views and stored routines, so dumps that contain them, such as the panel's own exports and backups, import as before. The panel deletes this user when the import ends, also when it fails. Only when the file created triggers, views, routines or events without a `DEFINER` clause, which then run as this user, does the panel keep it: locked, so nobody can log in with it, and with privileges on this database only. Deleting the database deletes it too.

On MySQL, user names stop at 32 characters, so for a long database name the user's name is shortened and a short hash added. The definers are allowed there with `SET_ANY_DEFINER` and `ALLOW_NONEXISTENT_DEFINER` instead of MariaDB's `SET USER`; without `SYSTEM_USER`, the user still can't make an object run as root. The first line of dumps from newer MariaDB versions, `/*M!999999\- enable the sandbox mode */`, which MySQL's client refuses, is skipped on MySQL.

::: warning
If the file uses another database, for example with `USE other_db`, `CREATE DATABASE other_db` or `INSERT INTO other_db.posts`, MariaDB refuses that statement and the import stops with "The SQL file uses another database, other_db." What the file did before that line stays in the database. Remove the `CREATE DATABASE` and `USE` lines from the file, or export the database again without `--databases`, then import it again with **Empty the database first (delete all of its tables)** ticked. `CREATE DATABASE` and `USE` lines that name this same database are fine.
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

MariaDB restarts the first time remote access is turned on, and again when it is turned off for the last database. Websites lose their database for a few seconds. If another task is running at that moment, the dialog says **MariaDB has to restart before it accepts remote connections.** Select **Restart MariaDB** when the task has finished.

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

MariaDB must be running to change these settings. The change applies at once, to connections opened after it, and stays after a restart: the panel keeps it in `/etc/mysql/mariadb.conf.d/99-panel-slowlog.cnf`. The log is written to `/var/log/mysql/mariadb-slow.log`, which the MariaDB package's log rotation covers. On MySQL, the settings are in `/etc/mysql/mysql.conf.d/99-panel-slowlog.cnf` and the log is `/var/log/mysql/mysql-slow.log`; empty it with **Clear log** when it grows.

The table shows the 200 newest entries, newest first, read from the last 4 MB of the log:

| Column | Meaning |
|---|---|
| **Time** | When the query ran |
| **User and host** | Who ran it, such as `shop@localhost` |
| **Database** | The database it used |
| **Duration** | How long it took, in seconds |
| **Rows examined** | How many rows MariaDB read |
| **Query** | The query. Select it to see all of it, up to 2,000 characters. Longer queries are marked **Shortened** |

Select **Refresh** to read the log again, and **Clear log** to empty it.

::: tip
Logging adds a little work for the server. Turn the switch off and select **Apply** when you are done.
:::

## Adminer

Adminer lets you browse and edit your databases in the browser, like phpMyAdmin.

### Install Adminer

1. Install a web server first, on the **Software** page. Adminer runs on it with PHP.
2. Go to **Software** → **Adminer (database UI)** and select **Install**.

The panel downloads Adminer 6.3.1 (the build with all of Adminer's drivers) and checks it against a pinned SHA-256 checksum before installing it. It opens MariaDB or MySQL databases, and PostgreSQL databases once PostgreSQL is installed (see [PostgreSQL in Adminer](#postgresql-in-adminer)). Its SQLite driver refuses every sign-in, because SQLite has no password.

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

## Uninstall MariaDB

Uninstall MariaDB on the **Software** page. The panel refuses while MariaDB holds any database other than its own system databases, including databases created outside the panel. See [Software](./software) for what an uninstall keeps.

MySQL is uninstalled the same way, on its card, with the same check. The panel also removes Oracle's repository and its key. The data directory `/var/lib/mysql` stays unless you tick the box and type `mysql`.

## PostgreSQL

Nefo can also run PostgreSQL, the database server many Laravel, Django, Node.js and Odoo applications prefer. It is installed from your distribution and managed on the **PostgreSQL** tab of the **Databases** page. MariaDB and PostgreSQL can run side by side.

### Install PostgreSQL

1. Go to **Software** → **PostgreSQL** and select **Install**.
2. Wait a minute or two. The progress log shows each step.

Nefo installs the distribution's `postgresql` package, so you get the version your system ships: 14 on Ubuntu 22.04, 16 on Ubuntu 24.04, 15 on Debian 12 and 17 on Debian 13. Newer versions from the PostgreSQL project's own repository (PGDG) aren't offered yet.

apt runs in the C locale, so the cluster the package creates stores text as `SQL_ASCII`. Nefo then creates that new, still empty cluster again with UTF-8 (`C.UTF-8`). A cluster that existed before the install is never recreated.

### How PostgreSQL is tuned

Nefo writes its settings to `/etc/postgresql/<version>/main/conf.d/90-nefo.conf`. The memory settings depend on the server's RAM:

| Server RAM | `shared_buffers` | `effective_cache_size` | `work_mem` | `maintenance_work_mem` | `max_connections` |
|---|---|---|---|---|---|
| Less than 768 MB | 64MB | 256MB | 2MB | 16MB | 30 |
| 768 MB or more | 128MB | 512MB | 4MB | 32MB | 40 |
| 1.5 GB or more | 256MB | 1024MB | 4MB | 64MB | 50 |
| 3 GB or more | 512MB | 2048MB | 8MB | 128MB | 75 |
| 7 GB or more | 1024MB | 4096MB | 16MB | 256MB | 100 |

`effective_cache_size` is about half the RAM, because the same server also runs the web server, PHP and maybe MariaDB. On every server Nefo also sets `listen_addresses = 'localhost'`, `password_encryption = 'scram-sha-256'`, `random_page_cost = 1.1` (for SSD storage) and `jit = off`.

Idle and empty, PostgreSQL needs roughly 30 to 40 MB for its handful of background processes. As its cache fills, it grows towards `shared_buffers` plus about 40 MB, and each open connection adds a few MB. The **Software** page shows `shared_buffers` plus 40 MB.

::: warning
Don't edit `90-nefo.conf` or `pg_hba.conf`. Installing PostgreSQL again rewrites them.
:::

### Who can connect

Nefo writes `/etc/postgresql/<version>/main/pg_hba.conf`:

```
local   all  postgres                peer
local   all  all                     scram-sha-256
host    all  all       127.0.0.1/32  scram-sha-256
host    all  all       ::1/128       scram-sha-256
```

- Only the `postgres` system user signs in without a password, on the local socket. The panel manages the server that way.
- Everyone else needs a password, on the socket or over TCP from this server. Unlike the distribution's default, other Linux users get no password-less ("peer") sign-in, so a website's Linux user can't open a database role of the same name without its password.
- Other machines can't connect: PostgreSQL listens on localhost only. Remote access per database, as MariaDB has it, isn't available for PostgreSQL yet.

### Create a PostgreSQL database

1. Go to **Databases** → **PostgreSQL** and select **Add database**.
2. Enter a name in **Database**. A role with the same name owns the database.
3. Leave **Password** empty to get a strong random password, or type your own.
4. Optional: choose a site in **Website**. A backup of that website then includes this database, and deleting the website with its databases deletes it too.
5. Select **Create database**.

The connection details card shows **Database**, **User**, **Password**, **Host** (`localhost`) and **Port** (`5432`). For a Laravel app, for example, set `DB_CONNECTION=pgsql`, `DB_HOST=127.0.0.1`, `DB_PORT=5432` and the database, user and password from the card.

The rules:

- Names and passwords follow the [MariaDB rules](#create-a-database). PostgreSQL's own names are refused too: `postgres`, `template0`, `template1`, `public`, `all` and anything starting with `pg_`.
- One name can't be used by both servers: if a MariaDB database or user is called `shop`, a PostgreSQL database can't be, and the other way round. The panel also refuses a name PostgreSQL already uses for a database or a role.
- The role can sign in, isn't a superuser, and can't create databases or roles. No other role may connect to the database, because the panel revokes `CONNECT` from everyone else.
- New databases use UTF-8 with the `C.UTF-8` locale.

The password never reaches PostgreSQL in clear text: the panel sends only its SCRAM-SHA-256 verifier, so it can't show up in PostgreSQL's log either.

**Show login**, **New password** and **Delete** work as for MariaDB. Deleting also closes the database's open connections. A database removed outside the panel is marked **Not found in PostgreSQL**.

### Export and import a PostgreSQL database

**Export (.sql.gz)** downloads a gzipped plain SQL dump made with `pg_dump` as the database's own role, with `--no-owner --no-privileges`, so it imports into any database.

**Import** takes a `.sql` or `.sql.gz` file of up to 4 GB:

- `psql` runs as the database's own role, never as `postgres`, with its password in a temporary file only root can read.
- The file runs in a single transaction with `ON_ERROR_STOP`: when a statement fails, nothing is imported and the error names the line.
- psql's own commands, the lines that start with a backslash (such as `\connect`, `\!` or `\copy`), are refused: psql runs as root, and those commands could run programs or write files on the server. The panel starts psql in restricted mode (`\restrict` with a random key) and takes out the `\restrict` and `\unrestrict` lines newer `pg_dump` versions write. A psql without restricted mode (older than 17.6, 16.10, 15.14 or 14.19) refuses every import: install the system updates first.
- Statements that create or change roles, give objects to other roles or need a superuser fail. Export such a dump again with `pg_dump --no-owner --no-privileges`.
- `SET transaction_timeout`, which `pg_dump` 17 and later write, is skipped on older servers.
- Afterwards the panel sets the role's password again, in case the file changed it.

With **Empty the database first (delete everything in it)** ticked, the database is dropped and created again before the import, owned by the same role.

### PostgreSQL in Adminer

Adminer needs PHP's `pgsql` extension for PostgreSQL. The panel installs it for the PHP version that runs Adminer, and only together with PostgreSQL: when you install PostgreSQL while Adminer is there, or Adminer while PostgreSQL is there. An Adminer that an earlier panel version installed, the MySQL-only build, is replaced at the same time.

**Open in Adminer** on a database's row opens Adminer's PostgreSQL sign-in with the server `localhost`, the user and the database filled in. Type the password from **Show login**.

### PostgreSQL in backups

Website backups include the website's PostgreSQL databases, and a database backup can hold one PostgreSQL database, on demand or on a schedule. The dump is plain `pg_dump` output, made as the database's role, stored as `postgres/<name>.sql` in the archive next to the MariaDB dumps.

Restoring empties each database (it is dropped and created again, owned by the same role) and imports the dump as that role. A database deleted since the backup is created again, with a new password.

### Uninstall PostgreSQL

Uninstall PostgreSQL on the **Software** page. The panel refuses while PostgreSQL holds any database other than `postgres`, `template0` and `template1`, including databases created outside the panel. It also refuses while PostgreSQL is stopped, because it can't check then. The data in `/var/lib/postgresql` and the settings in `/etc/postgresql` stay unless you tick the box and type `postgresql`. See [Software](./software#postgresql).

## Memcached

When Memcached is installed (**Software** → **Memcached**), the **Databases** page has a **Memcached** tab. It shows the version, how long it has been running, **Memory used** with the limit, the number of **Items**, the hit rate and the open connections. It also shows what Memcached itself reports: that it listens on 127.0.0.1 only and that UDP is off.

**Restart** empties the cache. See [Software](./software#memcached) for how it is set up.
