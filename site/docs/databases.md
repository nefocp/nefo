---
title: Databases
description: Install MariaDB, create databases, import and export SQL, allow remote access, read the slow query log, and use Adminer and the Redis key browser.
---

# Databases

The **Databases** page manages MariaDB, the database server for WordPress and most PHP applications. This page covers:

- installing MariaDB and how Nefo tunes it for a small server;
- creating databases, showing their logins and changing passwords;
- SQL import and export;
- remote access for one database at a time;
- the slow query log;
- Adminer, the database UI in your browser;
- the Redis key browser, on the **Redis** tab of the same page.

Nefo manages MariaDB only. PostgreSQL and MongoDB are available in the [App store](./apps) as Docker apps that listen on 127.0.0.1 only. They don't appear on the **Databases** page.

## Install MariaDB

1. Go to **Databases**.
2. Select **Install now**. You can also install it from **Software** → **MariaDB** → **Install**.
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

Each database has one user, `name@localhost`, with all privileges on that database and none on others. With remote access on, that user also gets a copy for each allowed address (see [Allow remote access](#allow-remote-access)). Apart from the temporary user that runs imports (see [Import](#import)), the panel doesn't create extra users or change privileges. If you need that, use the `mariadb` command on the server as root.

The list shows the databases the panel created. A database that was removed outside the panel is marked **Not found in MariaDB**.

## Show a database's login

Select **Show login** on the database's row. The connection details card shows the name, user, password and host, each with a copy button. If Adminer is installed, the card also has **Open in Adminer**.

## Change a database password

1. Open the menu of the database and select **New password**.
2. Keep **Generate a strong random password**, or choose **Use my own password** and type it. Your own password follows the same rules as when you [create a database](#create-a-database).
3. Select **New password**.

The panel sets the new password and shows it in the connection details card. If the database is linked to a WordPress website that the panel created, the panel also writes the new password into that site's `wp-config.php`. Other applications stop connecting until you update their settings.

If the database allows remote access, the remote users get the new password too.

## Export and import

### Export

Open the menu of the database and select **Export (.sql)**. Your browser downloads a plain SQL file named like `shop_20261126-143000.sql`.

The dump is made with `mariadb-dump --single-transaction`, so the website keeps running during the export. It includes stored routines and triggers.

### Import

1. Open the menu of the database and select **Import**.
2. Choose a file in **SQL file (.sql or .sql.gz)**. A `.sql.gz` file is unpacked on the fly.
3. Optional: tick **Empty the database first (delete all of its tables)**. The database is then dropped and created again before the import, and its user keeps access.
4. Select **Import**.

One import can be up to 4 GB.

The import runs as a temporary MariaDB user, `nefo-import-` followed by the database name, with all privileges on this database and none on others. It may also set the definer of triggers, views and stored routines, so dumps that contain them, such as the panel's own exports and backups, import as before. The panel deletes this user when the import ends, also when it fails. Only when the file created triggers, views, routines or events without a `DEFINER` clause, which then run as this user, does the panel keep it: locked, so nobody can log in with it, and with privileges on this database only. Deleting the database deletes it too.

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

MariaDB must be running to change these settings. The change applies at once, to connections opened after it, and stays after a restart: the panel keeps it in `/etc/mysql/mariadb.conf.d/99-panel-slowlog.cnf`. The log is written to `/var/log/mysql/mariadb-slow.log`, which the MariaDB package's log rotation covers.

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

The panel downloads Adminer 6.3.1 (the MySQL build) and checks it against a pinned SHA-256 checksum before installing it.

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
