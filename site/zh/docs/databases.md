---
title: 数据库
description: 安装 MariaDB 或 MySQL，选择 MariaDB 版本，升级 MariaDB，在 MariaDB 和 MySQL 之间切换，创建数据库，导入和导出 SQL，允许远程访问，查看慢查询日志，以及使用 Adminer、phpMyAdmin 和 Redis 键浏览器。
---

# 数据库

Nefo 每台服务器运行一个与 MySQL 兼容的数据库服务器：MariaDB（发行版自带的版本，或来自 MariaDB 的 11.4、11.8、12.3 LTS）或 MySQL 8.4 LTS，为小型 VPS 调优，只能从服务器本身访问。**数据库** 页面可以创建数据库和用户、导入和导出 SQL、向指定地址开放某个数据库，并显示慢查询。PostgreSQL 可以在它旁边运行。

本页介绍：

- 选择 MariaDB 或 MySQL 以及 MariaDB 的版本、安装它、Nefo 如何为小型服务器调优它、升级 MariaDB，以及在两者之间切换；
- 创建数据库、显示登录信息和修改密码；
- SQL 导入和导出；
- 按数据库逐个开放的远程访问；
- 慢查询日志；
- Adminer 和 phpMyAdmin，浏览器中的数据库界面；
- 同一页面 **Redis** 标签页上的 Redis 键浏览器，以及 Memcached 的状态。

PostgreSQL 在这个页面上有它自己的 **PostgreSQL** 标签页，也有它自己的文档页面：见 [PostgreSQL](./postgresql)。MongoDB 以及容器形式的 PostgreSQL 可以在 [应用商店](./apps) 中作为只监听 127.0.0.1 的 Docker 应用安装。这些 Docker 应用不会出现在 **数据库** 页面上。

## 选择 MariaDB 还是 MySQL {#choose-mariadb-or-mysql}

一台服务器运行一个与 MySQL 兼容的数据库服务器：MariaDB 或 MySQL 8.4 LTS。你在安装时选择，可以在 **数据库** 页面、**软件** 页面的 **数据库服务器** 卡片上，或在配置页面（**配置这台服务器**）中安装。已安装其中一个时，面板会拒绝安装另一个；以后你可以连同数据库从一个 [切换](#switch-between-mariadb-and-mysql) 到另一个。

| | MariaDB（推荐） | MySQL 8.4 LTS |
|---|---|---|
| 来源 | 发行版的软件包，或 MariaDB 自己软件源中的 LTS 系列（见 [MariaDB 版本](#mariadb-versions)） | Oracle 的软件源 `repo.mysql.com`（`mysql-8.4-lts` 分支） |
| 内存（大约） | 它的缓冲池加约 100 MB | 它的缓冲池加约 150 MB；开启 performance schema 时为缓冲池加约 300 MB，面板从约 3.5 GB 内存起开启它（所以 4 GB 服务器上是开启的） |
| 支持期 | 只要你的发行版还支持它；来自 MariaDB 软件源的系列，支持到 MariaDB 为它设定的结束日期 | Oracle 支持 8.4 LTS 到 2032 年 |
| 系统 | Nefo 支持的所有系统 | Ubuntu 22.04、24.04、26.04 和 Debian 12、13，仅限 amd64（x86-64） |
| 适合 | WordPress 和大多数 PHP 应用 | 需要 MySQL 本身的应用，或习惯 MySQL 的团队 |

两者都适用于本页的所有功能，以及备份、网站副本、WordPress 预发布副本和应用商店中的 PHP 应用。Adminer 和 phpMyAdmin 两者都能打开。在 arm64 服务器上，或在 Oracle 没有提供软件包的系统上，**软件** 页面只提供 MariaDB，并说明原因。

## 安装 MariaDB {#install-mariadb}

1. 前往 **数据库**。
2. 点击 **安装 MariaDB（推荐）**。也可以在 **软件** 页面安装：在 **数据库服务器** 卡片上保持选中 **MariaDB**，然后点击 **安装 MariaDB**。
3. 等待一到两分钟。进度日志会显示每一步。

Nefo 会从发行版安装 `mariadb-server` 和 `mariadb-client` 软件包。然后它写入自己的设置，启动 MariaDB，并删除有些版本自带的匿名用户和 `test` 数据库。

要改为安装 MariaDB 自己软件源中更新的系列，见 [MariaDB 版本](#mariadb-versions)。

### MariaDB 如何调优 {#how-mariadb-is-tuned}

Nefo 把它的设置写入 `/etc/mysql/mariadb.conf.d/90-panel.cnf`。最大的内存设置，即 InnoDB 缓冲池，取决于服务器的内存：

| 服务器内存 | `innodb_buffer_pool_size` |
|---|---|
| 小于 1.5 GB | 128M |
| 1.5 GB 或以上 | 256M |
| 3 GB 或以上 | 512M |
| 7 GB 或以上 | 1G |

其他设置在每台服务器上都相同：

| 设置 | 值 | 原因 |
|---|---|---|
| `bind-address` | `127.0.0.1` | MariaDB 只接受来自这台服务器的连接 |
| `skip-name-resolve` | 开启 | 每个连接都不做 DNS 查询 |
| `performance_schema` | `OFF` | 节省内存 |
| `max_connections` | `100` | |
| `character-set-server` | `utf8mb4` | 完整的 Unicode，包括表情符号 |
| `collation-server` | `utf8mb4_unicode_ci` | |

这样 MariaDB 大约占用它的缓冲池大小加 100 MB。**软件** 页面会显示这个估算值。

::: warning
不要编辑 `90-panel.cnf`。面板会重写它。
:::

## 可以安装哪些 MariaDB 版本？ {#mariadb-versions}

默认情况下，MariaDB 来自你的发行版。这样它的安全修复会随系统更新一起到来，也不需要添加其他软件源。当你需要比发行版更新的 MariaDB 时，可以改为安装 MariaDB 自己的软件源 `dlm.mariadb.com` 中的长期支持（LTS）系列：

| 系列 | MariaDB 支持到 | 提供于 |
|---|---|---|
| 11.4 LTS | 2029 年 5 月 29 日 | Ubuntu 22.04 和 24.04、Debian 12 |
| 11.8 LTS | 2028 年 6 月 4 日 | Ubuntu 22.04 和 24.04、Debian 12 |
| 12.3 LTS | 2029 年 6 月 12 日 | Ubuntu 22.04、24.04 和 26.04、Debian 12 和 13 |

只有比发行版自带的 MariaDB 更新时，才会提供某个系列：

| 系统 | 发行版自带的 MariaDB | 提供的系列 |
|---|---|---|
| Ubuntu 22.04 | 10.6 | 11.4、11.8、12.3 |
| Ubuntu 24.04 | 10.11 | 11.4、11.8、12.3 |
| Debian 12 | 10.11 | 11.4、11.8、12.3 |
| Ubuntu 26.04 | 11.8 | 12.3 |
| Debian 13 | 11.8 | 12.3 |

MariaDB 12.3 不会安装 Galera（MariaDB 的集群软件，单台服务器用不到）；11.4 和 11.8 会按照它们软件包的要求一并带上它。

要安装某个系列：

1. 前往 **软件**。在 **数据库服务器** 卡片上，保持选中 **MariaDB**。
2. 在它下面选择版本，例如 **来自 MariaDB 的 11.4 LTS**。发行版自带的版本（例如“10.11，发行版自带”）仍是默认值。
3. 点击 **安装 MariaDB 11.4**。

配置页面（**配置这台服务器**）的 **MariaDB** 下提供同样的版本，`nefo setup --db mariadb-11.4` 也一样（见 [命令行](./cli#nefo-setup)）。

对于来自 MariaDB 软件源的系列，面板会：

- 从 `https://supplychain.mariadb.com/MariaDB-Server-GPG-KEY`（或 `keyserver.ubuntu.com`）下载 MariaDB 的签名密钥，只有它的指纹正好是 `177F 4010 FE56 CA33 3630 0305 F165 6F24 C74C D1D8`（MariaDB 文档中列出的“MariaDB Signing Key”）时才接受它。它把密钥保存为 `/etc/apt/keyrings/panel-mariadb.gpg`，只对这个软件源受信任。
- 写入 `/etc/apt/sources.list.d/panel-mariadb.list`，其中只有该系列的一行，例如 `https://dlm.mariadb.com/repo/mariadb-server/11.4/repo/ubuntu noble main`。这样你的服务器会随系统更新获得 11.4 的小版本，但永远不会自己升到更新的系列。
- 检查 apt 确实会安装 MariaDB 该系列的软件包，然后安装 `mariadb-server` 和 `mariadb-client`，调优方式与发行版的 MariaDB 相同。

卸载 MariaDB 也会移除这个软件源文件和密钥。

面板会拒绝：

- 在已安装另一个 MariaDB 时安装某个系列：请改用 **升级**（见下文）；
- 安装比 `/var/lib/mysql` 中保留的旧数据（来自之前的安装）所属版本更旧的 MariaDB，因为它无法读取这些数据。请安装那个版本或更新的版本，或者先把数据目录移走。

## 升级 MariaDB {#upgrade-mariadb}

你可以把已安装的 MariaDB 升级到 MariaDB 软件源中更新的 LTS 系列，例如从发行版的 10.11 升级到 11.4，或从 Debian 13 的 11.8 升级到 12.3，数据库保持原位。

1. 前往 **软件**。在 **数据库服务器** 卡片上，点击 **升级**。
2. 选择系列。对话框会显示将要发生什么，以及需要多少可用磁盘空间。
3. 点击 **升级到 MariaDB 11.4**，然后查看日志。

升级按以下顺序进行：

1. 每个数据库都会导出为一个 gzip 压缩的 SQL 文件，放到 `/var/backups/panel/db-server/` 中的一个新文件夹里。面板会保留这些转储，直到你删除它们。
2. 下载新系列的软件包，同时保留已安装系列的软件包，这样不联网也能回退。
3. MariaDB 变为只读，面板统计每个表的行数，然后 MariaDB 停止。它的数据目录被复制到 `/var/lib/mysql.before-11.4-<date>-<time>`，`/etc/mysql` 被复制到转储所在的文件夹中。从这里开始，网站无法使用它们的数据库，直到升级结束，通常需要几分钟。
4. 新软件包替换旧软件包，并在同样的数据上启动 MariaDB，仍为只读。然后 `mariadb-upgrade` 更新它的系统表。
5. 面板检查 MariaDB 报告的是新版本、每个数据库都有相同的表且行数相同、视图、存储例程、触发器和事件也相同，以及面板的每个数据库用户都能用自己的密码登录。
6. MariaDB 重新接受写入，数据目录的副本被删除。

如果第 4 步及之后的任何一步失败，面板会放回旧的 MariaDB：移除新软件包，恢复 `/etc/mysql` 和复制的数据目录，重新安装旧软件包并启动 MariaDB。日志最后会写明原因。这时你的数据库和升级之前一样。

::: warning 无法退回旧系列
升级之后，MariaDB 已经升级了它的数据文件和系统表，旧版 MariaDB 无法读取它们。面板会拒绝降级（例如从 11.8 降到 11.4），并说明原因。要退回，请导出数据库，连同数据卸载 MariaDB，安装旧版本，然后导入它们。
:::

面板需要为转储、数据目录的副本和软件包准备可用磁盘空间：大约是数据目录的一倍半再加 400 MB。空间不够时它会拒绝开始。MariaDB 已停止时，或者服务器的配置把数据目录从 `/var/lib/mysql` 移走时，它也会拒绝。

## 安装 MySQL {#install-mysql}

1. 前往 **数据库**，点击 **改为安装 MySQL 8.4**。在 **软件** 页面上，在 **数据库服务器** 卡片上选择 **MySQL 8.4 LTS**，然后点击 **安装 MySQL 8.4 LTS**。
2. 等待两到四分钟。进度日志会显示每一步。

Nefo 自己添加 Oracle 的软件源，而不是安装 Oracle 的 `mysql-apt-config` 软件包：

- 它从 `https://repo.mysql.com/RPM-GPG-KEY-mysql-2025` 下载签名密钥，只有它的指纹正好是 `BCA4 3417 C3B4 85DD 128E C6D4 B7B3 B788 A8D3 785C`（MySQL 文档中列出的 MySQL Release Engineering 密钥）时才接受它。它把密钥保存为 `/etc/apt/keyrings/panel-mysql.gpg`。
- `/etc/apt/sources.list.d/panel-mysql.list` 只对这个软件源信任该密钥，并且只列出 `mysql-8.4-lts` 分支。
- Oracle 会不时更新这个密钥；当前这份在 2027 年 10 月 23 日过期。面板每周查找一次同一密钥的更新副本并替换旧的，这样 `apt-get update` 可以继续工作。

然后它在不询问任何问题的情况下安装 `mysql-community-server` 和 `mysql-community-client`。MySQL 的 `root` 账户没有密码：它只能以服务器上的 root 身份，通过本地套接字登录（`auth_socket`），面板不保存任何 root 密码。最后，面板会删除匿名用户和 `test` 数据库（如果有）。

### MySQL 如何调优 {#how-mysql-is-tuned}

Nefo 在软件包第一次启动 MySQL 之前就写入 `/etc/mysql/mysql.conf.d/90-panel.cnf`，所以 MySQL 在你的服务器上从不以 Oracle 的默认设置运行。内存设置取决于服务器的内存：

| 服务器内存 | `innodb_buffer_pool_size` | `max_connections` | `table_open_cache` | `temptable_max_ram` |
|---|---|---|---|---|
| 小于 1.5 GB | 128M | 75 | 400 | 32M |
| 1.5 GB 或以上 | 256M | 100 | 600 | 64M |
| 3 GB 或以上 | 512M | 150 | 1000 | 128M |
| 7 GB 或以上 | 1024M | 200 | 2000 | 256M |

其他设置：

| 设置 | 值 | 原因 |
|---|---|---|
| `bind-address` | `127.0.0.1` | 在某个数据库允许远程访问之前，MySQL 只接受来自这台服务器的连接 |
| `mysqlx` | `OFF` | 不在 33060 端口上运行 X Protocol 监听：PHP 和面板使用经典协议 |
| `disable_log_bin` | 开启 | 没有填满磁盘的二进制日志：服务器没有副本 |
| `performance_schema` | 内存约 3.5 GB 以下为 `OFF`，从那里起为 `ON`（所以 4 GB 服务器上是开启的） | 它自己就要占用 100 到 200 MB |
| `skip-name-resolve`、`host_cache_size` | 开启、`0` | 每个连接都不做 DNS 查询 |
| `innodb_log_buffer_size` | `16M` | MySQL 8.4 的默认值是 64M |
| `character-set-server`、`collation-server` | `utf8mb4`、`utf8mb4_unicode_ci` | 与 MariaDB 相同，所以转储可以在两者之间迁移 |

这样 MySQL 大约占用它的缓冲池大小加 150 MB，开启 performance schema 时加 300 MB。**软件** 页面会显示这个估算值；**服务** 页面显示它实际占用的内存。

用户使用 MySQL 8.4 的默认方式 `caching_sha2_password` 登录，PHP 的 `mysqli` 和 PDO 都支持它。`mysql_native_password` 保持关闭，与 MySQL 8.4 出厂时一样：它使用不加盐的 SHA-1 哈希，MySQL 9 已经移除了它。所以来自另一台服务器的旧密码哈希（cPanel 就是这样保存的）不能在 MySQL 上重用：迁移过来的数据库需要它的密码，或者一个新密码。

在 MySQL 上，面板用 `mysql` 命令而不是 `mariadb` 运行 SQL，用 `mysqldump` 导出，服务名为 `mysql`。

::: warning
不要编辑 `90-panel.cnf`。面板会重写它。
:::

## 能在 MariaDB 和 MySQL 之间切换吗？ {#switch-between-mariadb-and-mysql}

能，从 v0.24.0 起。一台服务器只运行一个数据库服务器，但你可以把 MariaDB 换成 MySQL 8.4 LTS，或把 MySQL 换成 MariaDB，连同你的数据库、它们的用户和权限。

1. 前往 **软件**。在 **数据库服务器** 卡片上，在 **改用 MySQL 8.4 LTS**（或 **改用 MariaDB**）下，点击 **切换到 MySQL 8.4 LTS**（或 **切换到 MariaDB**）。
2. 阅读对话框。它会列出要迁移的数据库和账户，以及每个账户之后如何登录。
3. 输入 `mysql`（或 `mariadb`）进行确认，然后点击切换按钮。

切换回 MariaDB 会安装发行版自带的 MariaDB。如果没有任何数据库，同样的任务只是把一个服务器换成另一个。

### 切换会做什么 {#what-the-switch-does}

1. 面板下载新服务器的软件包，并保留旧服务器的软件包以便回退。这时你的网站还没有任何变化。
2. 旧服务器变为只读：网站仍然可以读取数据库，但不能写入。面板读取它的账户和权限，统计每个表的行数，并把每个数据库导出到 `/var/backups/panel/db-server/` 中的一个新文件夹里。
3. 旧服务器停止。它的数据目录被移到 `/var/lib/mysql.mariadb-<date>-<time>`（或 `.mysql-…`），`/etc/mysql` 被复制到转储所在的文件夹中。它的软件包被移除。从这里开始，网站无法使用它们的数据库，直到切换结束。
4. 新服务器像 **软件** 页面安装它时那样安装，并保持只读。远程访问和慢查询日志按原样重新设置。
5. 创建账户，导入每个数据库，并重新授予权限。来自 MySQL 8 的转储会像导入网站时那样为 MariaDB 调整：它的 `utf8mb4_0900_*` 排序规则会换成 MariaDB 的（`utf8mb4_unicode_520_ci`、某种语言自己的排序规则，或 `utf8mb4_bin`），只有 MySQL 才有的设置会被去掉。从 MariaDB 11 迁到 MySQL 的转储，MariaDB 的 `uca1400` 排序规则会换成 MySQL 的（`utf8mb4_uca1400_ai_ci` 变为 `utf8mb4_0900_ai_ci`）。
6. 面板检查每个数据库都有相同的表且行数相同、视图、存储例程、触发器和事件也相同，以及面板的每个数据库用户都能用自己的密码登录。然后服务器重新接受写入，旧的数据目录被删除。

如果第 3 步及之后的任何一步失败，面板会移除新服务器，恢复旧服务器的配置和数据目录，重新安装它的软件包并启动它：你回到原来的状态。两种情况下转储都会保留。

需要多长时间取决于数据库的大小：导出、导入和行数统计各自都要把所有数据读一遍。

### 账户和密码 {#accounts-and-passwords}

MariaDB 和 MySQL 保存密码的方式不同，所以并不是每个密码都能迁移过去：

| 账户 | 切换之后 |
|---|---|
| **数据库** 页面上某个数据库的用户 | 保留它的密码（面板知道这个密码），在 `localhost` 和每个允许远程访问的地址上都是 |
| 你自己创建的、使用 `mysql_native_password` 密码的账户，迁到 MariaDB | 根据它的哈希保留密码 |
| 通过套接字登录的账户（MariaDB 上为 `unix_socket`，MySQL 上为 `auth_socket`） | 仍然通过套接字登录 |
| 你自己创建的其他任何账户，例如迁到 MySQL 的 MariaDB 账户（MySQL 8.4 关闭了 `mysql_native_password`），或迁到 MariaDB 的 MySQL 账户（MariaDB 无法读取 `caching_sha2_password`） | 用新的随机密码创建，并被锁定。日志会列出每个这样的账户。请在服务器上以 root 身份设置密码：`ALTER USER 'name'@'localhost' IDENTIFIED BY '…' ACCOUNT UNLOCK;` |

它们的权限根据旧服务器的 `SHOW GRANTS` 重新授予。另一个服务器没有的权限会被略过并列在日志中，角色和代理授权也一样，面板不会迁移它们。

### 哪些功能继续工作 {#what-keeps-working}

- 网站保留它们的设置：它们连接到 `localhost`，两种服务器使用同一个套接字。
- phpMyAdmin 和 Adminer 会打开新服务器。
- 备份：新的备份使用新服务器的导出程序，切换前创建的备份在切换后也可以恢复。
- 远程访问对同样的地址保持开放。

### 面板什么时候会拒绝 {#when-the-panel-refuses}

对话框会在 **请先解决以下问题：** 下列出必须先修复的问题，切换按钮保持禁用：

| 原因 | 怎么办 |
|---|---|
| 面板不知道某个数据库用户的密码（迁移过来时只带了密码哈希的数据库） | 在 **数据库** 页面为该数据库设置新密码，并把它写进网站的设置 |
| 迁到 MySQL：有表使用 MariaDB 自己的存储引擎（Aria 等）、序列或系统版本表 | 转换它们，例如用 `ALTER TABLE … ENGINE=InnoDB`，或者删除它们 |
| 迁到 MySQL：有 `INET4`、`INET6` 或 `UUID` 列 | 修改它们，例如改为 `VARCHAR` |
| 迁到 MySQL：视图、触发器或存储例程的定义者（definer）用户名超过 32 个字符 | 用另一个定义者重新创建它们 |
| 数据库名称以短横线开头 | 重命名该数据库 |
| 可用磁盘空间不足：转储和旧数据目录旁边导入的数据大约需要数据库大小的两倍再加 700 MB | 释放一些空间；删除旧的转储 |

服务器已停止时、服务器的配置把数据目录从 `/var/lib/mysql` 移走时，以及（迁到 MySQL 时）在不提供 MySQL 的地方（arm64，或 Oracle 没有提供软件包的系统），切换也会被拒绝。

有些差异只在导入数据时才会显现，例如 MariaDB 标为 `IGNORED` 的索引，MySQL 不认识它。这时导入会失败，面板会回到旧服务器，并在日志中给出错误。

## 升级和切换生成的转储 {#dumps-from-upgrades-and-switches}

**软件** 页面上的 **数据库服务器** 卡片会在 **保留的转储** 下列出这些文件夹：时间、内容（例如“MariaDB 10.11.14 → MariaDB 11.4”）、结果是 **完成**、**已回退** 还是 **失败**、数据库数量、它们的大小以及文件夹。每个数据库一个文件，`001.sql.gz`、`002.sql.gz` 等，`manifest.json` 写明每个文件对应的数据库，因为数据库名称中可能有文件名不能使用的字符。这个文件夹只有 root 能读取。

要找回某个数据库，解压它的文件，并在 **数据库** 页面导入。要删除一个文件夹，点击 **删除这些转储**（它旁边的垃圾桶）。升级或切换正在运行时，面板会拒绝。

## 创建数据库 {#create-a-database}

1. 前往 **数据库**，点击 **添加数据库**。
2. 在 **数据库** 中输入名称。会创建一个同名的用户，该用户只能使用这个数据库。
3. **密码** 留空会得到一个高强度的随机密码，也可以自己输入。
4. 可选：在 **网站** 中选择一个网站。这样该网站的备份会包含这个数据库。
5. 点击 **创建数据库**。

随后会出现一张标题为 **……的连接信息**（前面是数据库名称）的卡片，显示 **数据库**、**用户**、**密码** 和 **主机**。主机总是 `localhost`。

名称和密码的规则：

- 名称以字母开头，只能使用 `a-z`、`0-9` 和 `_`，最多 32 个字符。以下名称是保留的：`mysql`、`information_schema`、`performance_schema`、`sys`、`test`、`root`、`mariadb`、`phpmyadmin`、`debian_sys_maint`。MariaDB 中已被某个数据库或数据库用户使用的名称，面板也会拒绝。
- 密码为 12 到 64 个字符，由字母、数字和 `! # % * + , . : = ? @ ^ _ ~ -` 组成。生成的密码有 24 个字符。

新数据库使用 `utf8mb4` 和 `utf8mb4_unicode_ci`。

### 用户和权限 {#users-and-privileges}

每个数据库有一个用户 `name@localhost`，对该数据库拥有所有权限，对其他数据库没有任何权限。开启远程访问后，该用户还会为每个允许的地址得到一份副本（见 [允许远程访问](#allow-remote-access)）。除了运行导入的临时用户（见 [导入](#import)），面板不会创建额外的用户，也不会更改权限。如果你需要，请在服务器上以 root 身份使用 `mariadb` 命令（MySQL 上为 `mysql`）。

列表显示面板创建的数据库。在面板之外删除的数据库会标记为 **MariaDB 中不存在**（或 **MySQL 中不存在**）。

## 显示数据库的登录信息 {#show-a-database-s-login}

在数据库所在的行点击 **显示登录信息**。连接信息卡片会显示名称、用户、密码和主机，每项都带复制按钮。如果安装了 Adminer 或 phpMyAdmin，卡片上还会有 **在 Adminer 中打开** 或 **在 phpMyAdmin 中打开**。

## 修改数据库密码 {#change-a-database-password}

1. 打开数据库的菜单，选择 **新密码**。
2. 保持 **生成强随机密码**，或选择 **使用我自己的密码** 并输入它。你自己的密码遵循与 [创建数据库](#create-a-database) 时相同的规则。
3. 点击 **新密码**。

面板会设置新密码，并在连接信息卡片中显示它。如果该数据库关联到面板创建的 WordPress 网站，面板还会把新密码写入该网站的 `wp-config.php`。其他应用在你更新它们的设置之前将无法连接。

如果该数据库允许远程访问，远程用户也会得到新密码。

## 导出和导入 {#export-and-import}

### 导出 {#export}

打开数据库的菜单，选择 **导出（.sql）**。浏览器会下载一个纯 SQL 文件，名称类似 `shop_20261126-143000.sql`。

转储用 `mariadb-dump --single-transaction` 生成（MySQL 上为 `mysqldump`，同时带 `--set-gtid-purged=OFF`），所以导出期间网站会继续运行。它包含存储例程和触发器。

### 导入 {#import}

1. 打开数据库的菜单，选择 **导入**。
2. 在 **SQL 文件（.sql 或 .sql.gz）** 中选择一个文件。`.sql.gz` 文件会在导入时实时解压。
3. 可选：勾选 **先清空数据库（删除其中的所有表）**。这样导入之前会删除数据库并重新创建，它的用户仍保留访问权限。
4. 点击 **导入**。

一次导入最大 4 GB。

另一种服务器制作的转储（例如 [切换](#switch-between-mariadb-and-mysql) 之前的备份）也可以导入：面板会在文件流入时改写这台服务器不认识的排序规则名称（MariaDB 上的 MySQL 8 `utf8mb4_0900_*`，MySQL 上的 MariaDB 11 `uca1400`）。在 MySQL 上，它还会去掉 `NO_AUTO_CREATE_USER`，这是 MariaDB 写进每个触发器和存储例程 SQL 模式中的值，MySQL 8 会拒绝它。文件中的其他内容都不改变；使用另一个数据库的转储仍然会被拒绝。恢复数据库备份也是同样的处理。

导入以一个临时 MariaDB 用户运行，名称为 `nefo-import-` 后接数据库名称，对该数据库拥有所有权限，对其他数据库没有任何权限。它还可以设置触发器、视图和存储例程的定义者，所以包含这些内容的转储（例如面板自己的导出和备份）可以像以前一样导入。导入结束时，面板会删除这个用户，失败时也一样。只有当文件创建了不带 `DEFINER` 子句的触发器、视图、存储例程或事件（它们会以这个用户的身份运行）时，面板才会保留它：处于锁定状态，任何人都不能用它登录，并且只对这个数据库有权限。删除数据库时也会删除它。

在 MySQL 上，用户名最长 32 个字符，所以数据库名称较长时，用户名会被缩短，并加上一个短哈希。在那里，定义者通过 `SET_ANY_DEFINER` 和 `ALLOW_NONEXISTENT_DEFINER` 允许，而不是 MariaDB 的 `SET USER`；没有 `SYSTEM_USER`，这个用户仍然无法让对象以 root 身份运行。较新的 MariaDB 版本生成的转储第一行 `/*M!999999\- enable the sandbox mode */` 会被 MySQL 的客户端拒绝，在 MySQL 上会被跳过。

::: warning
如果文件使用了另一个数据库，例如通过 `USE other_db`、`CREATE DATABASE other_db` 或 `INSERT INTO other_db.posts`，MariaDB 会拒绝那条语句，导入会停止，并给出以“SQL 文件使用了另一个数据库 other_db。”开头的提示。文件在那一行之前所做的事会留在数据库中。请从文件中删除 `CREATE DATABASE` 和 `USE` 行，或者不带 `--databases` 重新导出数据库，然后勾选 **先清空数据库（删除其中的所有表）** 重新导入。指向这同一个数据库的 `CREATE DATABASE` 和 `USE` 行没有问题。
:::

## 允许远程访问 {#allow-remote-access}

默认情况下，MariaDB 只接受来自服务器本身的连接。你可以让指定的地址连接某个数据库，例如办公室电脑上的报表工具。

开始之前，在 **防火墙与 SSH** 页面安装并开启 ufw 防火墙。没有安装 ufw 时，面板会拒绝开启远程访问。

1. 打开数据库的菜单，选择 **远程访问**。
2. 在 **允许的地址** 下输入一个地址。点击 **添加地址** 可以添加更多。
3. 点击 **保存**。

用该数据库的用户和密码连接服务器的公网 IP 地址，端口 3306。

### 允许的地址 {#allowed-addresses}

- 单个 IPv4 或 IPv6 地址，例如 `203.0.113.10` 或 `2001:db8::1`。
- CIDR 形式的 IPv4 地址段，例如 `198.51.100.0/24`，不大于 `/16`。
- 不能使用 IPv6 地址段，因为 MariaDB 无法匹配它们。请输入单个 IPv6 地址。
- 每个数据库最多 10 个地址。
- 回环地址、`0.0.0.0` 和组播地址会被拒绝。

### 面板会改动什么 {#what-the-panel-changes}

| 位置 | 改动 |
|---|---|
| MariaDB 用户 | 每个地址一个用户 `name@address`，密码和权限与本地用户相同 |
| 防火墙（ufw） | 每个地址一条规则，只允许从该地址访问 3306/tcp 端口，注释为 `MariaDB name` |
| MariaDB 设置 | `/etc/mysql/mariadb.conf.d/99-panel-remote.cnf` 让 MariaDB 监听所有接口（`0.0.0.0`，允许 IPv6 地址时为 `::`）。只有至少一个数据库允许远程访问时，这个文件才存在 |

MySQL 的工作方式相同：文件为 `/etc/mysql/mysql.conf.d/99-panel-remote.cnf`，规则的注释为 `MySQL name`，远程用户变化时会先删除再重新创建，因为 MySQL 没有 `CREATE OR REPLACE USER`。下面的提示中相应地会写 MySQL。

第一次开启远程访问时 MariaDB 会重启，为最后一个数据库关闭远程访问时也会再次重启。网站会有几秒钟用不了数据库。如果当时有其他任务正在运行，对话框会显示“MariaDB 需要重启后才能接受远程连接。”任务完成后，点击 **重启 MariaDB**。

如果 MariaDB 带着远程访问文件无法启动，面板会移除这个文件，不带它启动 MariaDB，这样你的网站可以继续工作。

::: warning
- 如果防火墙已关闭，3306 端口就对所有人开放。MariaDB 仍然只接受列出的地址。
- 除非客户端要求 TLS，否则连接不加密。对于敏感数据，请使用 SSH 隧道或 VPN。
- 只列出你控制、而且不会变化的地址。
:::

要关闭远程访问，再次打开 **远程访问**，点击 **关闭**。删除数据库也会移除它的远程用户和防火墙规则。

## 查找慢查询 {#find-slow-queries}

MariaDB 可以记录每条耗时超过阈值的查询。这能帮你找到拖慢网站的原因。

1. 前往 **数据库** → **慢查询**。安装 MariaDB 后才会出现这个标签页。
2. 把 **慢于** 设为一个秒数，0.1 到 60。
3. 打开 **记录慢查询** 开关，然后点击 **应用**。

MariaDB 必须在运行才能更改这些设置。更改会立即生效，作用于之后打开的连接，并且重启后仍然保留：面板把它保存在 `/etc/mysql/mariadb.conf.d/99-panel-slowlog.cnf` 中，日志写入 `/var/log/mysql/mariadb-slow.log`。在 MySQL 上，设置在 `/etc/mysql/mysql.conf.d/99-panel-slowlog.cnf` 中，日志为 `/var/log/mysql/mysql-slow.log`。

面板会轮转这个日志，让它不会填满磁盘，MariaDB 和 MySQL 都一样。它每小时查看一次；当日志大于 50 MB，或者最早的条目已有 7 天时，面板会重命名它，让服务器打开一个新日志（`FLUSH SLOW LOGS`，不需要重启），并把旧日志压缩后放在旁边，名为 `mariadb-slow.log-YYYYMMDD-HHMMSS.gz`（MySQL 上为 `mysql-slow.log-…`）。它保留最新的 4 个，删除更早的。标签页会显示现在保留了多少个。表格只读取当前的日志。

表格显示最新的 200 条记录，从新到旧，读取自日志的最后 4 MB：

| 列 | 含义 |
|---|---|
| **时间** | 查询运行的时间 |
| **用户和主机** | 谁运行了它，例如 `shop@localhost` |
| **数据库** | 它使用的数据库 |
| **耗时** | 它花了多长时间，单位为秒 |
| **检查的行数** | MariaDB 读取了多少行 |
| **查询** | 查询语句。点击它可以查看完整内容，最多 2,000 个字符。更长的查询会标为 **已截断** |

点击 **刷新** 重新读取日志，点击 **清空日志** 清空它。**清空日志** 只清空当前日志；压缩过的旧日志会保留，直到轮转删除它们。

::: tip
记录日志会给服务器增加一点工作量。完成后请关闭开关并点击 **应用**。
:::

## Adminer {#adminer}

Adminer 让你在浏览器中浏览和编辑数据库，与 phpMyAdmin 类似。

### 安装 Adminer {#install-adminer}

1. 先在 **软件** 页面安装 Web 服务器。Adminer 用 PHP 在它上面运行。
2. 前往 **软件** → **Adminer（数据库界面）**，点击 **安装**。

面板会下载 Adminer 6.3.1（包含 Adminer 所有驱动的版本），并在安装前对照固定的 SHA-256 校验和进行检查。它可以打开 MariaDB 或 MySQL 数据库，安装 PostgreSQL 之后也能打开 PostgreSQL 数据库（见 [Adminer 中的 PostgreSQL](./postgresql#postgresql-in-adminer)）。它的 SQLite 驱动会拒绝所有登录，因为 SQLite 没有密码。

### 打开 Adminer {#open-adminer}

1. 点击 **数据库** 页面顶部或某个数据库连接信息卡片中的 **在 Adminer 中打开**。**软件** → **打开 Adminer** 的作用相同。
2. Adminer 会在新标签页中打开，地址是你的面板地址后接 `adminer/`，例如 `https://203.0.113.10:28443/k3x9p2qa7m1z/adminer/`。
3. 用 **显示登录信息** 中数据库的用户和密码登录 Adminer。

### Adminer 如何受到保护 {#how-adminer-is-protected}

Adminer 只能通过面板的登录访问：

- 它运行在 Web 服务器的一个内部网站上，这个网站只监听 127.0.0.1，端口是 40000 到 59999 之间的一个随机端口。
- 这个内部网站只响应带有秘密令牌的请求。其他请求得到 `404`。
- 面板只给来自已登录面板会话的请求加上令牌。没有面板会话时，`adminer/` 返回 `401`，请求永远到不了 Adminer。
- 面板自己的会话 Cookie 从不传给 Adminer。

所以 Adminer 无法从互联网访问，服务器上的其他程序没有令牌也无法使用它。

Adminer 可以导入最大 256 MB 的文件，一个请求最多可以运行 600 秒。更大的转储请使用 **数据库** 页面上的 **导入**。

卸载 Adminer 会移除它的文件和它的内部网站。你的数据库不会被动。卸载 Web 服务器也会移除 Adminer。

## phpMyAdmin {#phpmyadmin}

phpMyAdmin 是另一个数据库界面，适用于 MariaDB 和 MySQL。安装你喜欢的那个，或者两个都装：它们不共用文件。

### 安装 phpMyAdmin {#install-phpmyadmin}

1. 先在 **软件** 页面安装 Web 服务器。phpMyAdmin 用 PHP 在它上面运行。
2. 前往 **软件** → **phpMyAdmin（数据库界面）**，点击 **安装**。

面板会从 `files.phpmyadmin.net` 下载 phpMyAdmin 5.2.3（包含所有语言），对照固定的 SHA-256 校验和检查后，解压到 `/usr/share/nefo-phpmyadmin`。它会去掉配置脚本、示例和本地文档（帮助链接改为打开 docs.phpmyadmin.net）。phpMyAdmin 把它的临时文件放在 `/var/lib/nefo-phpmyadmin/tmp`。

面板会写入 phpMyAdmin 的 `config.inc.php`，带有一个随机的 Cookie 加密密钥：

- 你用某个数据库的用户和密码登录，只能登录这台服务器的数据库。你不能选择其他服务器、不能不用密码登录，也不能以 `root` 登录。
- phpMyAdmin 不检查新版本，也不发送错误报告。
- 它的修改密码表单被隐藏了。请在 **数据库** 页面修改数据库的密码，这样还会更新 WordPress 网站的 `wp-config.php`。

### 打开 phpMyAdmin {#open-phpmyadmin}

1. 点击 **数据库** 页面顶部、某个数据库的菜单或连接信息卡片中的 **在 phpMyAdmin 中打开**。**软件** → **打开 phpMyAdmin** 的作用相同。
2. phpMyAdmin 会在新标签页中打开，地址是你的面板地址后接 `phpmyadmin/`，例如 `https://203.0.113.10:28443/k3x9p2qa7m1z/phpmyadmin/`。
3. 用 **显示登录信息** 中数据库的用户和密码登录。

phpMyAdmin 的保护方式与 Adminer 相同（见 [Adminer 如何受到保护](#how-adminer-is-protected)），有它自己的内部网站和秘密令牌。没有面板会话时，`phpmyadmin/` 返回 `401`，API 密钥也无法访问它。只有 phpMyAdmin 的三个入口点作为 PHP 运行，只有它的脚本、样式和图片作为文件提供；其他任何东西都返回 `404`，包括配置脚本。phpMyAdmin 会发送它自己的 Content-Security-Policy，并检查它自己的表单令牌。

与 Adminer 一样，phpMyAdmin 可以导入最大 256 MB 的文件，一个请求最多可以运行 600 秒。卸载 phpMyAdmin 会移除它的文件、它的内部网站和它的设置。你的数据库不会被动。卸载 Web 服务器也会移除 phpMyAdmin。

## Redis 键浏览器 {#redis-key-browser}

安装 Redis 后，**数据库** 页面会有一个 **Redis** 标签页。要安装 Redis，前往 **软件** → **Redis** → **安装**。它如何设置见 [软件](./software#redis)。

标签页显示 Redis 版本、它已运行多长时间、**已用内存**（以及内存上限）和 **键** 的数量。如果显示 **Redis 已安装，但未运行。**，请在 **服务** 页面启动 Redis。

### 查找键 {#find-keys}

1. 选择 0 到 15 中的一个数据库。列表会显示每个数据库有多少个键。
2. 输入一个模式。`*` 匹配任何内容，`?` 匹配一个字符，例如 `user:*`。
3. 点击 **搜索**。

搜索每次大约查看 100 个键。如果还有更多，点击 **继续搜索**。

### 查看键 {#view-a-key}

点击一个键，可以查看它的 **类型**、**过期时间** 和 **大小**，以及它的值：

| 类型 | 面板显示什么 |
|---|---|
| string | 值的前 64 KB |
| list、hash、set、sorted set | 前 200 项。每项截取到 4 KB |
| stream 和其他类型 | 只显示类型和过期时间 |

二进制值中无法显示的字符会被替换。

### 删除键 {#delete-keys}

- 要删除一个键，点击它，然后点击 **删除键**。
- 要删除所选数据库中的每个键，点击 **清空数据库 0**（按钮会显示数据库的编号），输入编号进行确认，然后点击 **删除所有键**。

把 Redis 用作缓存的网站会重建缓存。只保存在 Redis 中的数据会丢失。

浏览器不能编辑值或过期时间。请在服务器上使用 `redis-cli`。

如果 Redis 有密码（`/etc/redis/redis.conf` 中的 `requirepass`），面板会从那个文件读取密码，并用它登录。

## 删除数据库 {#delete-a-database}

1. 打开数据库的菜单，选择 **删除**。
2. 输入数据库的名称进行确认，然后点击 **删除**。

::: danger
这会删除数据库、它的所有表和它的用户。此操作无法撤销。它的远程用户、防火墙规则和自动备份计划也会一起删除。已经做好的备份会保留在 **备份** 页面上。
:::

删除网站时，它的数据库会保留，除非你勾选 **同时删除它的数据库**：它们留在这里，不关联任何网站。恢复该网站的备份会重新关联它们。

## 卸载 MariaDB {#uninstall-mariadb}

在 **软件** 页面卸载 MariaDB。只要 MariaDB 中还有它自己的系统数据库以外的任何数据库（包括在面板之外创建的数据库），面板就会拒绝。卸载会保留什么见 [软件](./software)。

来自 MariaDB 软件源的 MariaDB 会连同它的软件源文件和密钥一起移除。

MySQL 也以同样的方式卸载，在它的卡片上，并进行同样的检查。面板还会移除 Oracle 的软件源及其密钥。数据目录 `/var/lib/mysql` 会保留，除非你勾选复选框并输入 `mysql`。

如果想把一个换成另一个，同时保留你的数据库，请改为 [切换](#switch-between-mariadb-and-mysql)。

## PostgreSQL {#postgresql}

PostgreSQL 在 MariaDB 或 MySQL 旁边运行，位于本页它自己的 **PostgreSQL** 标签页上：发行版自带的版本，或 PostgreSQL 项目提供的 15 到 19 版，带有数据库和所有者、导入和导出、经 TLS 的远程访问、备份和主版本升级。见 [PostgreSQL](./postgresql)。

## Memcached {#memcached}

安装 Memcached（**软件** → **Memcached**）后，**数据库** 页面会有一个 **Memcached** 标签页。它显示版本、已运行多长时间、**已用内存** 和上限、**条目** 数量、命中率和打开的连接数。它还会显示 Memcached 自己报告的情况：它只监听 127.0.0.1，并且 UDP 已关闭。

**重启** 会清空缓存。它如何设置见 [软件](./software#memcached)。
