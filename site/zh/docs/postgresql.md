---
title: PostgreSQL
description: 安装发行版自带的 PostgreSQL，或 PostgreSQL 项目提供的 15 到 19 版，创建数据库及其所有者，允许经 TLS 的远程访问，导入和导出，备份，以及升级到更新的主版本。
---

# PostgreSQL

Nefo 可以在 MariaDB 或 MySQL 旁边运行 PostgreSQL：发行版自带的版本，或者 PostgreSQL 项目自己的软件源（apt.postgresql.org）中 15 到 19 的某个主版本。**数据库** → **PostgreSQL** 标签页可以创建数据库及其所有者、导入和导出数据库、经 TLS 向指定地址开放某个数据库，并在先做转储之后把服务器升级到更新的主版本。

很多 Laravel、Django、Node.js 和 Odoo 应用更倾向于使用 PostgreSQL 这个数据库服务器。MariaDB（或 MySQL）和 PostgreSQL 可以同时运行。MariaDB 和 MySQL 见 [数据库](./databases)。

## 安装 PostgreSQL {#install-postgresql}

1. 前往 **软件** → **PostgreSQL**，点击 **安装**。
2. 在 **版本** 下选择要安装哪个 PostgreSQL：
   - 发行版自带的版本，例如“PostgreSQL 16，发行版自带的版本”，标有 **推荐**。它随系统一起更新；
   - apt.postgresql.org 的版本，例如“来自 apt.postgresql.org 的 PostgreSQL 18”，以及 PostgreSQL 项目软件源提供的其他主版本。应用需要比系统自带的更新（或更旧）的版本时，选择其中之一。
3. 点击 **安装**，等一两分钟。进度日志会显示每一步。

| 系统 | 发行版自带的版本 | 来自 apt.postgresql.org |
|---|---|---|
| Ubuntu 22.04 | 14 | 15、16、17、18、19 |
| Ubuntu 24.04 | 16 | 15、17、18、19 |
| Ubuntu 26.04 | 18 | 15、16、17、19 |
| Debian 12 | 15 | 16、17、18、19 |
| Debian 13 | 17 | 15、16、18、19 |

面板提供 PostgreSQL 项目仍在支持的主版本。在其他系统上，或者软件源没有对应软件包的处理器上，只提供发行版自带的版本，对话框会说明原因。

选择 apt.postgresql.org 的版本时，面板会：

- 从 `https://www.postgresql.org/media/keys/ACCC4CF8.asc` 下载软件源的签名密钥，除非其指纹是 `B97B 0AFC AA1A 47F0 44F2 44A0 7FCC 7D46 ACCC 4CF8`（“PostgreSQL Debian Repository”密钥），否则拒绝它；
- 把它写入 `/etc/apt/keyrings/panel-pgdg.gpg`，并添加 `/etc/apt/sources.list.d/panel-pgdg.list`，只对这个软件源信任该密钥；
- 按名称安装服务器软件包，例如 `postgresql-18`。它从不安装该软件源中的 `postgresql` 软件包，因为这个包总是拉入最新的主版本，系统更新时就会多出第二个服务器。

两种版本的安全更新都随系统更新一起提供（**系统** 页面，或 `apt upgrade`）。

之后，**软件** 卡片会显示 **来自发行版** 或 **来自 apt.postgresql.org**，**数据库** → **PostgreSQL** 的顶部也一样。

同一时间只运行一个 PostgreSQL 版本。面板会拒绝：

- 已安装一个版本时再安装另一个版本。请改为 [升级](#upgrade-postgresql) 它，或先卸载它；
- 服务器上还留有以前安装的某个版本的数据（在 `/var/lib/postgresql/<version>` 中）时，安装其他版本。请再次安装那个版本来使用这些数据，需要的话再升级，或者先删除这些数据。

apt 在 C 区域设置下运行，所以软件包创建的集簇会以 `SQL_ASCII` 存储文本。Nefo 随后会用 UTF-8（`C.UTF-8`）重新创建这个新的、仍然为空的集簇。安装之前就已存在的集簇永远不会被重新创建。

## PostgreSQL 如何调优 {#how-postgresql-is-tuned}

Nefo 把它的设置写入 `/etc/postgresql/<version>/main/conf.d/90-nefo.conf`。内存设置取决于服务器的内存：

| 服务器内存 | `shared_buffers` | `effective_cache_size` | `work_mem` | `maintenance_work_mem` | `max_connections` |
|---|---|---|---|---|---|
| 小于 768 MB | 64MB | 256MB | 2MB | 16MB | 30 |
| 768 MB 或以上 | 128MB | 512MB | 4MB | 32MB | 40 |
| 1.5 GB 或以上 | 256MB | 1024MB | 4MB | 64MB | 50 |
| 3 GB 或以上 | 512MB | 2048MB | 8MB | 128MB | 75 |
| 7 GB 或以上 | 1024MB | 4096MB | 16MB | 256MB | 100 |

`effective_cache_size` 约为内存的一半，因为同一台服务器还要运行 Web 服务器、PHP，可能还有 MariaDB。在每台服务器上，Nefo 还会设置 `listen_addresses = 'localhost'`、`password_encryption = 'scram-sha-256'`、`random_page_cost = 1.1`（针对 SSD 存储）和 `jit = off`。在容器 VPS（LXC）上，内存指的是容器的内存上限（见 [容器形式的 VPS](./first-steps#a-vps-that-is-a-container)）。

空闲且没有数据时，PostgreSQL 的几个后台进程大约需要 30 到 40 MB。随着缓存填满，它会增长到接近 `shared_buffers` 加约 40 MB，每个打开的连接再增加几 MB。**软件** 页面显示的是 `shared_buffers` 加 40 MB。

::: warning
不要编辑 `90-nefo.conf` 或 `pg_hba.conf`。再次安装 PostgreSQL 会重写它们。更改远程访问时，你在 `pg_hba.conf` 中自己写的行会保留：面板只改动它自己的行（见 [远程访问](#postgresql-remote-access)）。
:::

## 谁可以连接 {#who-can-connect}

Nefo 会写入 `/etc/postgresql/<version>/main/pg_hba.conf`：

```
local   all  postgres                peer
local   all  all                     scram-sha-256
host    all  all       127.0.0.1/32  scram-sha-256
host    all  all       ::1/128       scram-sha-256
```

- 只有 `postgres` 系统用户可以在本地套接字上不用密码登录。面板就是这样管理服务器的。
- 其他所有人都需要密码，无论是通过套接字，还是从这台服务器经 TCP 连接。与发行版的默认设置不同，其他 Linux 用户没有免密码（“peer”）登录，所以网站的 Linux 用户不能在没有密码的情况下打开同名的数据库角色。
- 其他机器无法连接，除非你为某个数据库允许它们的地址（见 [远程访问](#postgresql-remote-access)）。只要没有数据库允许远程访问，PostgreSQL 就只监听 localhost。

## 创建 PostgreSQL 数据库 {#create-a-postgresql-database}

1. 前往 **数据库** → **PostgreSQL**，点击 **添加数据库**。
2. 在 **数据库** 中输入名称。一个同名的角色拥有这个数据库。
3. **密码** 留空会得到一个高强度的随机密码，也可以自己输入。
4. 可选：在 **网站** 中选择一个网站。这样该网站的备份会包含这个数据库，连同数据库一起删除该网站时也会删除它。
5. 点击 **创建数据库**。

连接信息卡片会显示 **数据库**、**用户**、**密码**、**主机**（`localhost`）和 **端口**（`5432`）。例如，对于 Laravel 应用，设置 `DB_CONNECTION=pgsql`、`DB_HOST=127.0.0.1`、`DB_PORT=5432`，以及卡片上的数据库、用户和密码。

规则：

- 名称和密码遵循 [MariaDB 的规则](./databases#create-a-database)。PostgreSQL 自己的名称也会被拒绝：`postgres`、`template0`、`template1`、`public`、`all`，以及任何以 `pg_` 开头的名称。
- 同一个名称不能同时用于两个服务器：如果某个 MariaDB 数据库或用户叫 `shop`，PostgreSQL 数据库就不能叫这个名字，反过来也一样。PostgreSQL 已经用于某个数据库或角色的名称，面板也会拒绝。
- 该角色可以登录，不是超级用户，不能创建数据库或角色。其他角色都不能连接这个数据库，因为面板会撤销其他所有人的 `CONNECT` 权限。
- 新数据库使用 UTF-8 和 `C.UTF-8` 区域设置。

密码永远不会以明文形式到达 PostgreSQL：面板只发送它的 SCRAM-SHA-256 验证信息，所以密码也不会出现在 PostgreSQL 的日志中。

**显示登录信息**、**新密码** 和 **删除** 的用法与 MariaDB 相同。删除时还会关闭该数据库已打开的连接，并移除它的远程访问行和防火墙规则。在面板之外删除的数据库会标记为 **PostgreSQL 中不存在**。

## 其他服务器能连接 PostgreSQL 数据库吗？ {#postgresql-remote-access}

能，从 v0.24.0 起，按数据库逐个开放。默认情况下，PostgreSQL 只接受来自服务器本身的连接。你可以让指定的地址用某个数据库自己的用户（角色）和密码，经 TLS 连接这个数据库。

开始之前，在 **防火墙与 SSH** 页面安装 ufw 防火墙并开启它。没有安装 ufw 时，面板会拒绝开启远程访问。

1. 前往 **数据库** → **PostgreSQL**，打开该数据库的菜单，选择 **远程访问**。
2. 在 **允许的地址** 下输入一个地址。点击 **添加地址** 可以添加更多。
3. 点击 **保存**。

之后该行会显示“远程：1”（地址的数量），标签页顶部会显示“1 个对其他服务器开放”。

用 **显示登录信息** 中的用户和密码，经 TLS 连接服务器的公网 IP 地址，例如：

```sh
psql "host=203.0.113.10 port=5432 dbname=shop user=shop sslmode=require"
```

### 允许的地址 {#allowed-addresses}

- 单个 IPv4 或 IPv6 地址，例如 `198.51.100.7` 或 `2001:db8::1`。
- CIDR 形式的地址段：IPv4 不大于 `/16`（例如 `198.51.100.0/24`），IPv6 不大于 `/48`（例如 `2001:db8:5::/48`）。
- 每个数据库最多 10 个地址。
- 回环地址、链路本地地址（`fe80::`）、`0.0.0.0`、`::` 和组播地址会被拒绝。

### 面板会改动什么 {#what-the-panel-changes}

| 位置 | 改动 |
|---|---|
| `pg_hba.conf`（PostgreSQL 16 及更新版本） | 在末尾加一行 `include_if_exists nefo-remote.conf`，带一条注释。允许的地址写在 `/etc/postgresql/<version>/main/nefo-remote.conf` 中 |
| `pg_hba.conf`（PostgreSQL 14 和 15） | 在末尾 `# BEGIN Nefo remote access` 和 `# END Nefo remote access` 之间加一个块 |
| 这些行 | 每个地址一行，只针对这个数据库及其自己的角色，使用密码登录：`hostssl "shop" "shop" 198.51.100.7/32 scram-sha-256` |
| PostgreSQL 设置 | `/etc/postgresql/<version>/main/conf.d/95-nefo-remote.conf` 设置 `listen_addresses = '*'`。只有至少一个数据库允许远程访问时，这个文件才存在 |
| 防火墙（ufw） | 每个地址一条规则，只允许从该地址经 TCP 访问集簇的端口（默认 5432），注释为 `PostgreSQL name` |

面板从不改动 `pg_hba.conf` 中的其他行。你自己的行写在面板的行之前时，会先被读取：你为同一地址写的 `reject` 行优先。在 PostgreSQL 重新加载改动后的 `pg_hba.conf` 之前，面板会请 PostgreSQL 先检查它（`pg_hba_file_rules`）。如果 PostgreSQL 发现错误，旧文件会被放回去，什么都不改变。

如果集簇使用 5432 以外的端口，防火墙规则会使用那个端口，连接信息中也会显示它。

### 加密 {#encryption}

对话框会说明来自其他机器的连接是否加密：

- **来自其他机器的连接已加密**：这些行是 `hostssl`，所以 PostgreSQL 会拒绝来自这些地址、不使用 TLS 的连接。客户端需要 `sslmode=require`（或更严格的设置）。Debian 和 Ubuntu 创建集簇时会使用系统的自签名证书（`/etc/ssl/certs/ssl-cert-snakeoil.pem`）。如果集簇关闭了 TLS，面板会生成自己的自签名证书（`pg_hba.conf` 旁边的 `nefo-ssl.crt` 和 `nefo-ssl.key`），并在 `95-nefo-remote.conf` 中开启 TLS；关闭最后一个远程访问时，TLS 也会随之关闭。两种证书都是自签名的，所以客户端无法用它们验证服务器：`sslmode=require` 会加密，但 `verify-ca` 和 `verify-full` 会失败，除非你安装自己的证书。
- **来自其他机器的连接未加密**：PostgreSQL 没有 TLS，所以这些行是 `host`。任何敏感内容都请使用 SSH 隧道或 VPN。

### 重启 {#restarts}

新的 `listen_addresses` 需要重启才能生效；允许的地址和 TLS 只需要重新加载。所以，第一次开启远程访问时 PostgreSQL 会重启，为最后一个数据库关闭远程访问时也会再次重启。网站会有几秒钟用不了数据库，未完成的事务会被取消。之后添加或移除地址不会重启它。

如果当时有其他任务正在运行，对话框会显示“**PostgreSQL 需要重启后才能接受来自其他机器的连接。**”任务完成后，在对话框中点击 **重启**。

如果 PostgreSQL 带着 `95-nefo-remote.conf` 无法启动，面板会移除这个文件，不带它启动 PostgreSQL，这样你的网站可以继续工作。

::: warning
- 如果防火墙已关闭，端口就对所有人开放。PostgreSQL 仍然只允许列出的地址登录，而且必须有密码。
- 只列出你控制、而且不会变化的地址。
:::

要关闭远程访问，再次打开 **远程访问**，点击 **关闭**。删除数据库也会移除它的行和防火墙规则；如果它是最后一个开启远程访问的数据库，PostgreSQL 会重启，重新只监听 localhost。

## 导出和导入 PostgreSQL 数据库 {#export-and-import-a-postgresql-database}

**导出（.sql.gz）** 会下载一个 gzip 压缩的纯 SQL 转储，由 `pg_dump` 以数据库自己的角色、带 `--no-owner --no-privileges` 生成，所以可以导入任何数据库。

**导入** 接受最大 4 GB 的 `.sql` 或 `.sql.gz` 文件：

- `psql` 以数据库自己的角色运行，从不以 `postgres` 运行，它的密码放在只有 root 能读取的临时文件中。
- 文件在单个事务中运行，并带有 `ON_ERROR_STOP`：某条语句失败时，什么都不会导入，错误信息会指出所在的行。
- psql 自己的命令，即以反斜杠开头的行（例如 `\connect`、`\!` 或 `\copy`），会被拒绝：psql 以 root 运行，这些命令可能在服务器上运行程序或写入文件。面板以受限模式启动 psql（`\restrict` 加一个随机密钥），并去掉较新的 `pg_dump` 版本写入的 `\restrict` 和 `\unrestrict` 行。没有受限模式的 psql（早于 17.6、16.10、15.14 或 14.19）会拒绝所有导入：请先安装系统更新。
- 创建或更改角色、把对象交给其他角色或需要超级用户的语句会失败。请用 `pg_dump --no-owner --no-privileges` 重新导出这样的转储。
- 在较旧的服务器上，会跳过 `pg_dump` 17 及以后版本写入的 `SET transaction_timeout`。
- 导入之后，面板会重新设置角色的密码，以防文件更改了它。

勾选 **先清空数据库（删除其中的所有内容）** 后，导入前会删除数据库并重新创建，所有者仍是同一个角色。

## 把 PostgreSQL 升级到更新的版本 {#upgrade-postgresql}

你可以把服务器迁移到 apt.postgresql.org 中更新的主版本，例如从发行版自带的 15 升级到 17，并保留每个数据库、用户、密码和远程访问设置。

1. 前往 **数据库** → **PostgreSQL**，点击 **升级**。有更新的版本可用、且 PostgreSQL 正在运行时，才会显示这个按钮。
2. 选择 **新版本**。
3. 阅读 **升级过程**，然后勾选 **我了解升级期间网站和应用将无法使用数据库。**
4. 点击 **升级到 17**（你选择的版本），然后查看日志。

面板按顺序执行：

1. 检查新版本确实更新、只有一个集簇（`main`）、没有为旧版本安装扩展软件包（例如 `postgresql-15-postgis-3`），以及磁盘空间足够容纳两倍的数据再加 256 MB。
2. 用 `pg_dumpall` 把每个数据库保存到一个压缩文件 `/var/backups/nefo-postgresql/postgresql-<old>-before-<new>-<date>.sql.gz` 中，只有 root 能读取。此时什么都还没有改动：如果转储失败，升级就在这里停止。
3. 先把 `postgresql-<old>` 标记为手动安装，然后移除发行版的 `postgresql` 软件包，这个包只是指向某个版本（来自 apt.postgresql.org 时，它会指向最新版本并安装第二个服务器）。接着，如果还没有 apt.postgresql.org 软件源就添加它，并安装 `postgresql-<new>`。这个软件包创建的空集簇会被删除。如果 apt 顺带装上了其他 PostgreSQL 服务器，升级会停止并回滚。
4. 运行 `pg_upgradecluster -v <new> -m upgrade <old> main`。在此期间 PostgreSQL 会停止：小型服务器约一分钟，大型数据库会更久。`pg_upgrade` 会复制数据文件，所以旧集簇保持原样。
5. 把旧的 `pg_hba.conf`、面板的调优设置（`conf.d/90-nefo.conf`）和远程访问设置交给新集簇（从 16 版起，允许的地址移到 `nefo-remote.conf` 中），重启它，并检查它在同一个端口上运行、有同样的数据库，而且每个数据库都能响应。
6. 直到这时，才删除旧集簇并移除它的软件包（`postgresql-<old>` 和 `postgresql-client-<old>`）。

如果第 3 到第 5 步中的任何一步失败，面板会回滚：删除新集簇，移除新版本的软件包，放回旧集簇的设置并重新启动它。日志最后会写“PostgreSQL 15 runs again with its data as before”（PostgreSQL 15 已带着原来的数据重新运行）。你的数据库保持原样。一旦从 apt.postgresql.org 安装过软件包，这个软件源就会保留，因为 `postgresql-common` 和 `libpq5` 已经从它更新，否则就再也得不到更新；发行版的 `postgresql` 软件包不会被放回。

以下情况会拒绝升级，并在对话框中给出原因：

| 原因 | 怎么办 |
|---|---|
| 为旧版本安装了扩展软件包 | pg_upgrade 需要为新版本构建的相同扩展，面板不会安装它们。为新版本安装扩展后，用 `pg_upgradecluster` 手动升级，或先移除这些扩展 |
| 存在多个集簇 | 面板只管理 `main`。请手动升级 |
| 磁盘空间不足 | 释放提示中给出的空间 |

`pg_upgrade` 无法迁移的数据库（例如带有 `regproc` 列的表）会让第 4 步失败；面板随后会按上面的说明回滚。

### 保留的转储 {#the-dumps-kept}

无论成功与否，保存的文件都会留在 `/var/backups/nefo-postgresql` 中，**PostgreSQL** 标签页会在 **保留的转储** 下列出它：它是为哪次升级生成的（例如“PostgreSQL 15 → 17”）、生成时间、大小和路径。确认一切正常后，用 **删除此转储** 删除它。

要在全新的服务器上手动用它恢复所有内容：

```sh
zcat postgresql-15-before-17-20261127-101500.sql.gz | runuser -u postgres -- psql -X -d postgres
```

## Adminer 中的 PostgreSQL {#postgresql-in-adminer}

Adminer 需要 PHP 的 `pgsql` 扩展才能使用 PostgreSQL。面板会为运行 Adminer 的 PHP 版本安装这个扩展，而且只在与 PostgreSQL 同时存在时安装：已有 Adminer 时安装 PostgreSQL，或已有 PostgreSQL 时安装 Adminer。旧版面板安装的 Adminer（只支持 MySQL 的版本）会同时被替换。

数据库所在行的 **在 Adminer 中打开** 会打开 Adminer 的 PostgreSQL 登录页面，并填好服务器 `localhost`、用户和数据库。输入 **显示登录信息** 中的密码即可。

## 备份中的 PostgreSQL {#postgresql-in-backups}

网站备份包含该网站的 PostgreSQL 数据库，数据库备份可以包含一个 PostgreSQL 数据库，可手动备份，也可按计划备份。转储是以数据库角色生成的纯 `pg_dump` 输出，在压缩包中存为 `postgres/<name>.sql`，与 MariaDB 转储放在一起。

备份、导出和导入使用已安装版本（来自发行版或 apt.postgresql.org）的 `pg_dump` 和 `psql`，以及集簇的端口，即使端口不是 5432 也一样。

恢复时会清空每个数据库（删除后重新创建，所有者仍是同一个角色），并以该角色导入转储。备份之后被删除的数据库会重新创建；备份中有角色密码时（从 v0.24.0 起）使用备份中的密码，否则使用新密码。

## 卸载 PostgreSQL {#uninstall-postgresql}

在 **软件** 页面卸载 PostgreSQL。只要 PostgreSQL 中还有 `postgres`、`template0` 和 `template1` 以外的任何数据库（包括在面板之外创建的数据库），面板就会拒绝卸载。PostgreSQL 已停止时也会拒绝，因为那时无法检查。`/var/lib/postgresql` 中的数据和 `/etc/postgresql` 中的设置会保留，除非你勾选复选框并输入 `postgresql`。见 [软件](./software#postgresql)。

卸载还会移除 apt.postgresql.org 软件源及其密钥（如果是面板添加的），以及为 PostgreSQL 数据库留下的防火墙规则。如果 PHP 的 `pgsql` 扩展（为 Adminer 添加）仍需要 `libpq5`，面板会把 `libpq5` 恢复为发行版的版本，让它重新获得系统的安全更新。

应用商店中还有作为 Docker 服务运行在 127.0.0.1 上的 PostgreSQL，供需要自己的服务器的应用使用。它不会出现在 **数据库** 页面上：见 [应用商店](./apps#install-a-service)。
