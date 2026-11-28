---
title: PostgreSQL
description: Install PostgreSQL from your distribution or versions 15 to 19 from the PostgreSQL project, create databases and owners, allow remote access over TLS, import and export, back up, and upgrade to a newer major version.
---

# PostgreSQL

Nefo runs PostgreSQL next to MariaDB or MySQL: your distribution's version, or a major version from 15 to 19 from the PostgreSQL project's own repository (apt.postgresql.org). The **Databases** → **PostgreSQL** tab creates databases with their owners, imports and exports them, opens one database to chosen addresses over TLS, and upgrades the server to a newer major version with a dump first.

PostgreSQL is the database server many Laravel, Django, Node.js and Odoo applications prefer. MariaDB (or MySQL) and PostgreSQL can run side by side. For MariaDB and MySQL, see [Databases](./databases).

## Install PostgreSQL

1. Go to **Software** → **PostgreSQL** and select **Install**.
2. Under **Version**, choose which PostgreSQL to install:
   - your distribution's version, such as "PostgreSQL 16, the distribution's version", marked **Recommended**. It is updated with the system;
   - a version from apt.postgresql.org, such as "PostgreSQL 18 from apt.postgresql.org", and the other major versions the PostgreSQL project's repository offers. Choose one when an application needs a newer (or an older) version than your system ships.
3. Select **Install** and wait a minute or two. The progress log shows each step.

| System | The distribution's version | From apt.postgresql.org |
|---|---|---|
| Ubuntu 22.04 | 14 | 15, 16, 17, 18, 19 |
| Ubuntu 24.04 | 16 | 15, 17, 18, 19 |
| Ubuntu 26.04 | 18 | 15, 16, 17, 19 |
| Debian 12 | 15 | 16, 17, 18, 19 |
| Debian 13 | 17 | 15, 16, 18, 19 |

The panel offers the major versions the PostgreSQL project still supports. On another system, or on a processor the repository has no packages for, only the distribution's version is offered and the dialog says why.

When you choose a version from apt.postgresql.org, the panel:

- downloads the repository's signing key from `https://www.postgresql.org/media/keys/ACCC4CF8.asc` and refuses it unless its fingerprint is `B97B 0AFC AA1A 47F0 44F2 44A0 7FCC 7D46 ACCC 4CF8` (the "PostgreSQL Debian Repository" key);
- writes it to `/etc/apt/keyrings/panel-pgdg.gpg` and adds `/etc/apt/sources.list.d/panel-pgdg.list`, which trusts that key for this repository only;
- installs the server package by its name, such as `postgresql-18`. It never installs the `postgresql` package from that repository, because it always pulls in the newest major version, and a system update would then add a second server.

Security updates for either version come with the system updates (**System** page, or `apt upgrade`).

The **Software** card then shows **From the distribution** or **From apt.postgresql.org**, and so does the top of **Databases** → **PostgreSQL**.

Only one PostgreSQL version runs at a time. The panel refuses:

- another version while one is installed. [Upgrade](#upgrade-postgresql) it instead, or uninstall it first;
- a version other than the one whose data is still on the server from an earlier install (in `/var/lib/postgresql/<version>`). Install that version again to use the data, then upgrade it if you want, or delete the data first.

apt runs in the C locale, so the cluster the package creates stores text as `SQL_ASCII`. Nefo then creates that new, still empty cluster again with UTF-8 (`C.UTF-8`). A cluster that existed before the install is never recreated.

## How PostgreSQL is tuned

Nefo writes its settings to `/etc/postgresql/<version>/main/conf.d/90-nefo.conf`. The memory settings depend on the server's RAM:

| Server RAM | `shared_buffers` | `effective_cache_size` | `work_mem` | `maintenance_work_mem` | `max_connections` |
|---|---|---|---|---|---|
| Less than 768 MB | 64MB | 256MB | 2MB | 16MB | 30 |
| 768 MB or more | 128MB | 512MB | 4MB | 32MB | 40 |
| 1.5 GB or more | 256MB | 1024MB | 4MB | 64MB | 50 |
| 3 GB or more | 512MB | 2048MB | 8MB | 128MB | 75 |
| 7 GB or more | 1024MB | 4096MB | 16MB | 256MB | 100 |

`effective_cache_size` is about half the RAM, because the same server also runs the web server, PHP and maybe MariaDB. On every server Nefo also sets `listen_addresses = 'localhost'`, `password_encryption = 'scram-sha-256'`, `random_page_cost = 1.1` (for SSD storage) and `jit = off`. On a container VPS (LXC), the RAM is the container's memory limit (see [A VPS that is a container](./first-steps#a-vps-that-is-a-container)).

Idle and empty, PostgreSQL needs roughly 30 to 40 MB for its handful of background processes. As its cache fills, it grows towards `shared_buffers` plus about 40 MB, and each open connection adds a few MB. The **Software** page shows `shared_buffers` plus 40 MB.

::: warning
Don't edit `90-nefo.conf` or `pg_hba.conf`. Installing PostgreSQL again rewrites them. Your own lines in `pg_hba.conf` are kept when you change remote access: the panel only touches its own lines (see [Remote access](#postgresql-remote-access)).
:::

## Who can connect

Nefo writes `/etc/postgresql/<version>/main/pg_hba.conf`:

```
local   all  postgres                peer
local   all  all                     scram-sha-256
host    all  all       127.0.0.1/32  scram-sha-256
host    all  all       ::1/128       scram-sha-256
```

- Only the `postgres` system user signs in without a password, on the local socket. The panel manages the server that way.
- Everyone else needs a password, on the socket or over TCP from this server. Unlike the distribution's default, other Linux users get no password-less ("peer") sign-in, so a website's Linux user can't open a database role of the same name without its password.
- Other machines can't connect, unless you allow their address for one database (see [Remote access](#postgresql-remote-access)). PostgreSQL listens on localhost only while no database allows remote access.

## Create a PostgreSQL database

1. Go to **Databases** → **PostgreSQL** and select **Add database**.
2. Enter a name in **Database**. A role with the same name owns the database.
3. Leave **Password** empty to get a strong random password, or type your own.
4. Optional: choose a site in **Website**. A backup of that website then includes this database, and deleting the website with its databases deletes it too.
5. Select **Create database**.

The connection details card shows **Database**, **User**, **Password**, **Host** (`localhost`) and **Port** (`5432`). For a Laravel app, for example, set `DB_CONNECTION=pgsql`, `DB_HOST=127.0.0.1`, `DB_PORT=5432` and the database, user and password from the card.

The rules:

- Names and passwords follow the [MariaDB rules](./databases#create-a-database). PostgreSQL's own names are refused too: `postgres`, `template0`, `template1`, `public`, `all` and anything starting with `pg_`.
- One name can't be used by both servers: if a MariaDB database or user is called `shop`, a PostgreSQL database can't be, and the other way round. The panel also refuses a name PostgreSQL already uses for a database or a role.
- The role can sign in, isn't a superuser, and can't create databases or roles. No other role may connect to the database, because the panel revokes `CONNECT` from everyone else.
- New databases use UTF-8 with the `C.UTF-8` locale.

The password never reaches PostgreSQL in clear text: the panel sends only its SCRAM-SHA-256 verifier, so it can't show up in PostgreSQL's log either.

**Show login**, **New password** and **Delete** work as for MariaDB. Deleting also closes the database's open connections, and removes its remote access lines and firewall rules. A database removed outside the panel is marked **Not found in PostgreSQL**.

## Can other servers connect to a PostgreSQL database? {#postgresql-remote-access}

Yes, from v0.24.0, one database at a time. By default PostgreSQL only accepts connections from the server itself. You can let chosen addresses connect to one database, with that database's own user (role) and password, over TLS.

Before you start, install the ufw firewall on the **Firewall & SSH** page, and turn it on. The panel refuses remote access while ufw isn't installed.

1. Go to **Databases** → **PostgreSQL**, open the menu of the database and select **Remote access**.
2. Under **Allowed addresses**, enter an address. Select **Add an address** for more.
3. Select **Save**.

The row then shows "Remote: 1" (the number of addresses), and the top of the tab shows "1 open to other servers".

Connect to the server's public IP address with the user and password from **Show login**, and with TLS, for example:

```sh
psql "host=203.0.113.10 port=5432 dbname=shop user=shop sslmode=require"
```

### Allowed addresses

- Single IPv4 or IPv6 addresses, such as `198.51.100.7` or `2001:db8::1`.
- Ranges in CIDR form: IPv4 no wider than `/16` (such as `198.51.100.0/24`), IPv6 no wider than `/48` (such as `2001:db8:5::/48`).
- At most 10 addresses per database.
- Loopback, link-local (`fe80::`), `0.0.0.0`, `::` and multicast addresses are refused.

### What the panel changes

| Where | Change |
|---|---|
| `pg_hba.conf` (PostgreSQL 16 and newer) | One line at the end, `include_if_exists nefo-remote.conf`, with a comment. The allowed addresses are in `/etc/postgresql/<version>/main/nefo-remote.conf` |
| `pg_hba.conf` (PostgreSQL 14 and 15) | A block at the end between `# BEGIN Nefo remote access` and `# END Nefo remote access` |
| Those lines | One per address, for this database and its own role only, with password sign-in: `hostssl "shop" "shop" 198.51.100.7/32 scram-sha-256` |
| PostgreSQL settings | `/etc/postgresql/<version>/main/conf.d/95-nefo-remote.conf` sets `listen_addresses = '*'`. It exists only while at least one database allows remote access |
| Firewall (ufw) | For each address, a rule that allows the cluster's port (5432 by default) over TCP from that address only, with the comment `PostgreSQL name` |

The panel never changes the other lines of `pg_hba.conf`. When your own lines come before the panel's, they are read first: a `reject` line of yours for the same address wins. Before PostgreSQL reloads a changed `pg_hba.conf`, the panel asks PostgreSQL to check it (`pg_hba_file_rules`). If PostgreSQL finds an error, the old files are put back and nothing changes.

If the cluster uses another port than 5432, the firewall rules use that port, and the connection details show it.

### Encryption

The dialog says whether connections from other machines are encrypted:

- **Connections from other machines are encrypted**: the lines are `hostssl`, so PostgreSQL refuses connections from those addresses that don't use TLS. Clients need `sslmode=require` (or stricter). Debian and Ubuntu create clusters with the system's self-signed certificate (`/etc/ssl/certs/ssl-cert-snakeoil.pem`). If the cluster has TLS turned off, the panel makes its own self-signed certificate (`nefo-ssl.crt` and `nefo-ssl.key` next to `pg_hba.conf`) and turns TLS on in `95-nefo-remote.conf`; TLS goes off again with the last remote access. Both certificates are self-signed, so a client can't verify the server with them: `sslmode=require` encrypts, but `verify-ca` and `verify-full` fail unless you install a certificate of your own.
- **Connections from other machines are not encrypted**: PostgreSQL has no TLS, so the lines are `host`. Use an SSH tunnel or a VPN for anything sensitive.

### Restarts

A new `listen_addresses` takes a restart; the allowed addresses and TLS only take a reload. So PostgreSQL restarts the first time remote access is turned on, and again when it is turned off for the last database. Websites lose their database for a few seconds, and open transactions are cancelled. Adding or removing addresses later doesn't restart it.

If another task is running at that moment, the dialog says **PostgreSQL has to restart before it accepts connections from other machines.** Select **Restart** in the dialog when the task has finished.

If PostgreSQL doesn't start with `95-nefo-remote.conf`, the panel removes that file and starts PostgreSQL without it, so your websites keep working.

::: warning
- If the firewall is turned off, the port is open to everyone. PostgreSQL still lets only the listed addresses sign in, and only with the password.
- List only addresses you control that don't change.
:::

To turn remote access off, open **Remote access** again and select **Turn off**. Deleting a database also removes its lines and its firewall rules; when it was the last database with remote access, PostgreSQL restarts to listen on localhost only again.

## Export and import a PostgreSQL database

**Export (.sql.gz)** downloads a gzipped plain SQL dump made with `pg_dump` as the database's own role, with `--no-owner --no-privileges`, so it imports into any database.

**Import** takes a `.sql` or `.sql.gz` file of up to 4 GB:

- `psql` runs as the database's own role, never as `postgres`, with its password in a temporary file only root can read.
- The file runs in a single transaction with `ON_ERROR_STOP`: when a statement fails, nothing is imported and the error names the line.
- psql's own commands, the lines that start with a backslash (such as `\connect`, `\!` or `\copy`), are refused: psql runs as root, and those commands could run programs or write files on the server. The panel starts psql in restricted mode (`\restrict` with a random key) and takes out the `\restrict` and `\unrestrict` lines newer `pg_dump` versions write. A psql without restricted mode (older than 17.6, 16.10, 15.14 or 14.19) refuses every import: install the system updates first.
- Statements that create or change roles, give objects to other roles or need a superuser fail. Export such a dump again with `pg_dump --no-owner --no-privileges`.
- `SET transaction_timeout`, which `pg_dump` 17 and later write, is skipped on older servers.
- Afterwards the panel sets the role's password again, in case the file changed it.

With **Empty the database first (delete everything in it)** ticked, the database is dropped and created again before the import, owned by the same role.

## Upgrade PostgreSQL to a newer version {#upgrade-postgresql}

You can move the server to a newer major version from apt.postgresql.org, for example from the distribution's 15 to 17, keeping every database, user, password and remote access setting.

1. Go to **Databases** → **PostgreSQL** and select **Upgrade**. The button shows when a newer version is offered and PostgreSQL runs.
2. Choose the **New version**.
3. Read **What happens**, then tick **I understand that websites and applications lose their database during the upgrade.**
4. Select **Upgrade to 17** (the version you chose) and follow the log.

What the panel does, in order:

1. Checks that the new version is newer, that there is a single cluster (`main`), that no extension packages are installed for the old version (such as `postgresql-15-postgis-3`), and that the disk has room for twice the data plus 256 MB.
2. Saves every database with `pg_dumpall` to one compressed file, `/var/backups/nefo-postgresql/postgresql-<old>-before-<new>-<date>.sql.gz`, readable by root only. Nothing has changed yet: if the dump fails, the upgrade stops there.
3. Removes the distribution's `postgresql` package, which only points to a version (from apt.postgresql.org it would point to the newest one and install a second server), after marking `postgresql-<old>` as installed by hand. Then it adds the apt.postgresql.org repository if it isn't there yet and installs `postgresql-<new>`. The empty cluster this package creates is dropped. If apt brings any other PostgreSQL server along, the upgrade stops and rolls back.
4. Runs `pg_upgradecluster -v <new> -m upgrade <old> main`. PostgreSQL is stopped meanwhile: about a minute for a small server, longer for large databases. `pg_upgrade` copies the data files, so the old cluster stays as it was.
5. Gives the new cluster the old `pg_hba.conf`, the panel's tuning (`conf.d/90-nefo.conf`) and the remote access settings (from version 16 the allowed addresses move to `nefo-remote.conf`), restarts it, and checks that it runs on the same port with the same databases, each answering.
6. Only then drops the old cluster and removes its packages (`postgresql-<old>` and `postgresql-client-<old>`).

If any step from 3 to 5 fails, the panel rolls back: it drops the new cluster, removes the new version's packages, puts back the old cluster's settings and starts it again. The log ends with "PostgreSQL 15 runs again with its data as before". Your databases are as they were. The apt.postgresql.org repository stays once packages were installed from it, because `postgresql-common` and `libpq5` were updated from it and would otherwise get no more updates; the distribution's `postgresql` package isn't put back.

The upgrade is refused, with the reason in the dialog, when:

| Reason | What to do |
|---|---|
| Extension packages are installed for the old version | pg_upgrade needs the same extensions built for the new version, which the panel doesn't install. Upgrade by hand with `pg_upgradecluster` after installing them for the new version, or remove the extensions first |
| More than one cluster exists | The panel only manages `main`. Upgrade by hand |
| Not enough disk space | Free the amount the message gives |

A database that `pg_upgrade` can't carry over, for example a table with a `regproc` column, makes step 4 fail; the panel then rolls back as described.

### The dumps kept

The saved file stays in `/var/backups/nefo-postgresql` either way, and the **PostgreSQL** tab lists it under **Dumps kept**: which upgrade it was made for (such as "PostgreSQL 15 → 17"), when, its size and its path. Delete it with **Delete this dump** once you are sure everything works.

To restore everything from it on a fresh server by hand:

```sh
zcat postgresql-15-before-17-20261127-101500.sql.gz | runuser -u postgres -- psql -X -d postgres
```

## PostgreSQL in Adminer

Adminer needs PHP's `pgsql` extension for PostgreSQL. The panel installs it for the PHP version that runs Adminer, and only together with PostgreSQL: when you install PostgreSQL while Adminer is there, or Adminer while PostgreSQL is there. An Adminer that an earlier panel version installed, the MySQL-only build, is replaced at the same time.

**Open in Adminer** on a database's row opens Adminer's PostgreSQL sign-in with the server `localhost`, the user and the database filled in. Type the password from **Show login**.

## PostgreSQL in backups

Website backups include the website's PostgreSQL databases, and a database backup can hold one PostgreSQL database, on demand or on a schedule. The dump is plain `pg_dump` output, made as the database's role, stored as `postgres/<name>.sql` in the archive next to the MariaDB dumps.

Backups, exports and imports use the `pg_dump` and `psql` of the installed version, from the distribution or from apt.postgresql.org, and the cluster's port, also when it isn't 5432.

Restoring empties each database (it is dropped and created again, owned by the same role) and imports the dump as that role. A database deleted since the backup is created again, with its role's password from the backup when the backup has it (from v0.24.0), otherwise with a new password.

## Uninstall PostgreSQL

Uninstall PostgreSQL on the **Software** page. The panel refuses while PostgreSQL holds any database other than `postgres`, `template0` and `template1`, including databases created outside the panel. It also refuses while PostgreSQL is stopped, because it can't check then. The data in `/var/lib/postgresql` and the settings in `/etc/postgresql` stay unless you tick the box and type `postgresql`. See [Software](./software#postgresql).

Uninstalling also removes the apt.postgresql.org repository and its key, if the panel added them, and any firewall rules left for PostgreSQL databases. When PHP's `pgsql` extension (added for Adminer) still needs `libpq5`, the panel puts `libpq5` back to the distribution's version, so it gets your system's security updates again.

The App store also has PostgreSQL as a Docker service on 127.0.0.1, for apps that want their own server. That one doesn't appear on the **Databases** page: see [App store](./apps#install-a-service).
