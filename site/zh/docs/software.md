---
title: 软件
description: 安装和卸载 Web 服务器、PHP 5.6 到 8.6、ionCube Loader、MariaDB 或 MySQL、PostgreSQL、Redis、Memcached、Adminer、phpMyAdmin、Node.js、Python、Java、Docker、Web 防火墙和 ClamAV，以及管理 PHP 扩展和 php.ini 设置。
---

# 软件

在你选择之前，Nefo 什么都不安装。**软件** 页面可以安装和卸载每个可选组件，并显示它大约占用多少内存：Web 服务器、PHP 5.6 到 8.6 和 ionCube Loader、MariaDB 或 MySQL、PostgreSQL、Redis、Memcached、Node.js、Python、Java、Docker、Web 防火墙和 ClamAV。仍有东西在使用的组件，它会拒绝移除。

本页介绍可以安装什么、卸载什么时候会被拒绝、卸载会保留什么，以及 PHP、Node.js、Python、Java 和 Redis 的设置。小型服务器只装网站用得到的东西，就能保持轻量。

安装和卸载作为带实时日志的任务运行。面板一次只运行一个任务。除非本页另有说明，软件都来自你的发行版的软件包。

## 可以安装什么 {#what-you-can-install}

每张卡片显示该项运行时大约占用多少内存。页面顶部会把已安装的项加起来。

| 分组 | 项 | 内存（大约） |
|---|---|---|
| **Nginx、Apache 或 OpenLiteSpeed** | Nginx | 约 15 MB |
| | Apache | 约 40 MB |
| | OpenLiteSpeed | 约 26 MB |
| **PHP 版本** | PHP 7.4、8.0、8.1、8.2、8.3、8.4、8.5 和 8.6（PHP 软件源有更新的版本后也会出现），以及 **旧版本** 下的 5.6、7.0、7.1、7.2 和 7.3 | 每个约 25 MB |
| | ionCube Loader | 本身不占用：它被加载到 PHP 工作进程中 |
| **数据库与缓存** | MariaDB（发行版自带的版本，或 MariaDB 软件源中的 11.4、11.8 或 12.3 LTS），或者改用 MySQL 8.4 LTS | MariaDB：它的缓冲池加约 100 MB（见 [数据库](./databases#how-mariadb-is-tuned)）。MySQL：它的缓冲池加约 150 MB，内存约 3.5 GB 起则为缓冲池加约 300 MB（见 [数据库](./databases#how-mysql-is-tuned)） |
| | PostgreSQL | 空闲时约 30 到 40 MB，增长到 `shared_buffers` 加约 40 MB（见 [PostgreSQL](./postgresql#how-postgresql-is-tuned)） |
| | Redis | 最多 64、128 或 256 MB，取决于服务器的内存 |
| | Memcached | 最多 64、128 或 256 MB，取决于服务器的内存；为空时只有几 MB |
| | Adminer（数据库界面） | 空闲时不占用 |
| | phpMyAdmin（数据库界面） | 空闲时不占用 |
| **运行环境** | Node.js | 空闲时不占用 |
| | Python 应用 | 空闲时不占用 |
| | Java（OpenJDK） | 空闲时不占用；应用网站的 Java 程序占用它的堆所需的内存 |
| **容器** | Docker | 约 90 MB，加上你的容器 |
| **安全** | Web 防火墙（ModSecurity） | 约 40 MB |
| | ClamAV 杀毒软件 | 约 15 MB，扫描时需要约 1.4 GB 可用内存 |
| **工具** | Composer | 不占用，只在调用时运行 |
| | WP-CLI | 不占用，只在调用时运行 |

## 安装一项 {#install-an-item}

1. 前往 **软件**。
2. 在该项的卡片上点击 **安装**。
3. 查看日志。任务完成后，卡片会显示 **已安装** 及其版本。

有些卡片随后会提供一个链接：**打开数据库**、**打开 PostgreSQL**、**打开 Redis**、**打开 Memcached**、**打开 Adminer**、**打开 phpMyAdmin** 或 **打开 Docker**。

## 卸载一项 {#uninstall-an-item}

1. 在该项的卡片上点击 **卸载**。对于 PHP 版本，它在该版本的菜单中。
2. 阅读对话框中关于卸载会做什么的说明。
3. 对于 MariaDB、MySQL、PostgreSQL 和 Docker，可以勾选复选框同时删除它们的数据。这时你必须输入该项的名称进行确认。
4. 点击 **卸载**。

### 卸载什么时候会被拒绝 {#when-an-uninstall-is-refused}

面板从不移除仍有东西在使用的软件。某项无法卸载时，它的卡片会显示原因，而不是一个可用的按钮。

| 项 | 以下情况会被拒绝 |
|---|---|
| Web 服务器 | 存在任何网站，无论是在面板中，还是作为 Web 服务器网站文件夹中的配置文件 |
| 某个 PHP 版本 | 有网站在使用它，或者它是唯一安装的 PHP 版本（Web 服务器的配置和 Adminer 需要一个）。如果某个受支持的版本（7.4 或更新）是 [旧版本](#older-php-versions) 之外唯一的一个，它也会保留 |
| Composer | 某个 Git 部署脚本运行了 `composer` |
| WP-CLI | 存在 WordPress 网站：WordPress 工具箱要为它们运行它 |
| MariaDB 或 MySQL | 其中有它自己的系统数据库以外的任何数据库。面板直接询问服务器本身，所以手动创建或由其他工具创建的数据库也算。如果它已停止，而且它的数据文件夹被移动了，面板无法检查，就会拒绝 |
| PostgreSQL | PostgreSQL 自己报告其中有 `postgres`、`template0` 和 `template1` 以外的任何数据库，或者它已停止，面板无法检查 |
| Node.js | 有应用网站使用服务器的默认 Node.js，或者 `/usr/local/bin` 中的 `node` 不是面板安装的 |
| **Node.js 版本** 下的某一行 | 它是服务器的默认版本，或者有应用网站选择了它 |
| Python 应用 | 有应用网站运行 Python |
| Java（OpenJDK） | 有应用网站运行 Java。有应用网站使用某个 Java 版本时，单独卸载该版本会被拒绝 |
| Docker | 有网站运行 Docker 应用、存在 Compose 项目，或者 Docker 中有容器（无论是否在运行） |

Redis、Memcached、Adminer、phpMyAdmin、ionCube Loader、Web 防火墙和 ClamAV 随时都可以卸载。

### 卸载会保留什么 {#what-an-uninstall-keeps}

| 项 | 保留 | 移除 |
|---|---|---|
| Web 服务器 | `/var/www` 中的网站文件夹、证书、PHP | Web 服务器、它的面板配置、Adminer，以及 80 和 443 端口的防火墙规则 |
| PHP 版本 | | 该版本、它的扩展，以及只有它们需要的库 |
| MariaDB | 数据文件夹 `/var/lib/mysql`，除非你勾选 **同时删除数据目录（/var/lib/mysql）** 并输入 `mariadb`。再次安装 MariaDB 时会使用它 | 服务器和客户端软件包、面板的 MariaDB 设置，以及 MariaDB 的软件源和密钥（如果 MariaDB 来自那里） |
| MySQL | 数据文件夹 `/var/lib/mysql`，除非你勾选 **同时删除数据目录（/var/lib/mysql）** 并输入 `mysql`。再次安装 MySQL 时会使用它 | 服务器和客户端软件包、面板的 MySQL 设置，以及 Oracle 的软件源和密钥 |
| PostgreSQL | `/var/lib/postgresql` 中的数据和 `/etc/postgresql` 中的集簇设置，除非你勾选 **同时删除数据和设置（/var/lib/postgresql 和 /etc/postgresql）** 并输入 `postgresql`。软件包移除时不清除配置，再次安装 PostgreSQL 会启动同一个集簇 | PostgreSQL 软件包，以及 apt.postgresql.org 软件源和它的密钥（`/etc/apt/sources.list.d/panel-pgdg.list` 和 `/etc/apt/keyrings/panel-pgdg.gpg`），如果是面板添加的 |
| Redis | | Redis、它的 PHP 扩展，以及存储在 Redis 中的一切。PHP-FPM 会重启 |
| Memcached | | Memcached、它的 PHP 扩展，以及缓存的一切。PHP-FPM 会重启 |
| Adminer | 你的数据库 | Adminer 的文件和它的内部网站 |
| phpMyAdmin | 你的数据库 | phpMyAdmin 的文件、临时文件、内部网站和设置 |
| Node.js | 其他 Node.js 版本。如果有应用网站选择了默认版本，默认版本的文件夹也保留 | `/usr/local/bin` 中的 `node`、`npm` 和 `npx`，以及它们指向的版本 |
| Python 应用 | 系统需要的 `python3` 本身，以及旧应用文件夹中的虚拟环境 | venv 和 pip 软件包 |
| Java（OpenJDK） | | 每个已安装的 Java 版本，以及只有它们需要的软件包 |
| ionCube Loader | | 每个 PHP 版本中的加载器，以及 `/usr/local/lib/ioncube`。PHP-FPM 会重新加载，编码过的 PHP 文件将无法运行 |
| Docker | `/var/lib/docker` 中的镜像、卷和构建缓存，除非你勾选 **同时删除所有镜像、卷和构建缓存（/var/lib/docker）** 并输入 `docker`。再次安装 Docker 时会找到它们 | Docker 及其 Compose 插件 |
| Web 防火墙 | 防火墙的日志 | ModSecurity 和 OWASP 规则。Web 服务器会重启 |
| ClamAV | | ClamAV 及其病毒库 |
| Composer | Laravel 和 Drupal 网站（它们只在安装时需要 Composer） | `/usr/local/bin/composer` 和发行版的 `composer` 软件包 |
| WP-CLI | | `/usr/local/lib/panel/wp-cli.phar`。WordPress 工具箱需要时会重新安装它 |

::: danger
删除 MariaDB、MySQL、PostgreSQL 或 Docker 的数据无法撤销。请确认你对仍需要的东西都有备份。
:::

## 菜单只显示已安装的内容 {#the-menu-shows-what-is-installed}

左侧菜单会隐藏未安装软件的页面：

- 安装 Docker 后，才会出现 **Docker**。
- 存在 WordPress 网站后，才会出现 **WordPress**。
- 在 **数据库** 页面上，安装了相应的软件后，才会出现 **PostgreSQL**、**Redis** 和 **Memcached** 标签页；安装了 MariaDB 或 MySQL 后，才会出现 **慢查询**。

**网站**、**数据库** 和 **软件** 始终显示，因为它们提供安装入口。你正在打开的页面会保持可见。

## Web 服务器 {#web-server}

面板同一时间只运行一个 Web 服务器，并为它配置每个网站：

- Nginx 最轻量。PHP 在 PHP-FPM 中运行。
- Apache 会读取一些 PHP 应用自带的 `.htaccess` 文件，占用的内存稍多。
- OpenLiteSpeed 会读取 `.htaccess` 文件中的重写规则，并为 WordPress 内置 LiteSpeed Cache。它来自 LiteSpeed 自己的软件源。

安装 Web 服务器时，还会安装发行版自带的 PHP 版本和 PHP-FPM。如果 ufw 防火墙已开启，面板会开放 80 和 443 端口，使用 OpenLiteSpeed 时还会为 HTTP/3 开放 443/udp。

已安装一个 Web 服务器时，面板会拒绝安装第二个，因为两者都需要 80 和 443 端口。

要换成另一个 Web 服务器：

1. 先删除所有网站。如果以后还需要，请先备份。
2. 在当前 Web 服务器上点击 **卸载**，并输入它的名称（`nginx`、`apache` 或 `openlitespeed`）进行确认。
3. 安装另一个 Web 服务器。

网站的设置见 [网站](./websites)。

## PHP 版本 {#php-versions}

你可以并排安装多个 PHP 版本，并在 **网站** → 你的网站 → **PHP** 中为每个网站选择一个。

- 发行版自带的版本来自发行版的软件包。其他版本来自 Ondřej Surý 的软件源 `packages.sury.org/php`，Ubuntu 和 Debian 都一样。面板在把这个软件源添加到服务器之前会先询问，并且只接受指纹为 `15058500 A0235D97 F5D10063 B188E2B6 95BD4743` 的签名密钥。
- 每个版本都会安装以下扩展：fpm、cli、mysql、curl、gd、mbstring、xml、zip、intl、bcmath 和 opcache。从 PHP 8.5 起，OPcache 是 PHP 本身的一部分，所以 8.5 及更新版本没有单独的 opcache 软件包。安装了 Redis 或 Memcached 时，还会加上 redis 或 memcached 扩展。
- PHP 8.0 之前，json 是一个单独的软件包，也会加上。PHP 5.6 还会在软件源有 mcrypt 时加上它。
- 面板把该版本默认的 PHP-FPM 池改为按需启动的工作进程，所以没有网站使用的版本几乎不占内存。

列出的版本以 PHP 软件源为准。服务器上有了 `packages.sury.org/php` 之后（安装第一个非发行版自带的 PHP 版本时会添加它），页面会提供该软件源为你的系统提供的版本，这些版本从 apt 的软件包列表中读取。新的 PHP 版本（例如 PHP 8.7）在软件源发布、并且软件包列表刷新之后就会自动出现（每次安装都会运行 `apt-get update`，系统每天的 apt 任务也会运行）。软件源中没有的版本不会提供。在添加软件源之前，页面提供 Nefo 知道的版本，7.4 到 8.6。已安装的版本、发行版自带的版本和 **旧版本** 始终会列出。

在已安装的版本上点击 **管理**，可以打开它的 **扩展** 和 **设置**。

### 没有安全更新的版本 {#end-of-life}

每个 PHP 版本都会获得 PHP 官方的安全更新，直到一个固定日期（见 [php.net/supported-versions.php](https://www.php.net/supported-versions.php)）：8.1 到 2025 年 12 月 31 日，8.2 到 2026 年 12 月 31 日，8.3 到 2027 年 12 月 31 日，8.4 到 2028 年 12 月 31 日，8.5 到 2029 年 12 月 31 日，8.6 到 2030 年 12 月 31 日。7.4 和 8.0 已经没有安全更新了。过了那个日期，来自 `packages.sury.org` 的版本会在“软件”页面、它的 **管理** 窗口以及网站的 PHP 版本菜单中显示徽章 **生命周期结束——没有安全更新** 和一行带日期的说明。面板会把日期与今天比较，所以徽章会自动出现。

发行版自己的 PHP 版本（Ubuntu 22.04 上的 8.1、24.04 上的 8.3、26.04 上的 8.5、Debian 12 上的 8.2、Debian 13 上的 8.4）永远不会显示这个徽章：只要该发行版本仍受支持，发行版的安全团队就会继续为它打补丁。

条件允许时，请把网站从带有这个徽章的版本上迁走。你仍然可以安装它，不需要额外确认；只有 [旧版本](#older-php-versions) 会要求确认。

### 旧版 PHP {#older-php-versions}

PHP 5.6、7.0、7.1、7.2 和 7.3 位于 **旧版本** 下，折叠在其他版本下方。它们是为从另一台服务器（cPanel、aaPanel……）迁移过来、还不能在较新 PHP 上运行的网站准备的。

::: danger 生命周期结束：没有安全更新
PHP 多年前就停止了这些版本的安全更新：5.6 在 2018 年，7.3 在 2021 年（见 [php.net/eol.php](https://www.php.net/eol.php)）。它们已知的漏洞永远不会被修复，所以运行在这些版本上的网站更容易被入侵。只在更新旧网站期间用它们让网站继续运行，然后把网站迁到受支持的版本。不要把它们用于新网站。
:::

- 每个旧版本都会在“软件”页面和网站的 PHP 版本菜单中显示徽章 **生命周期结束——没有安全更新**。
- 安装旧版本时，会先请你确认这个风险。
- 必须先安装一个受支持的版本（7.4 或更新），而且只要还装着旧版本，最后一个受支持的版本就不能卸载。所以旧版本永远不会是服务器上唯一的 PHP：Web 服务器的配置、Adminer、Composer 和面板的工具都会保留一个受支持的版本。
- 应用商店有它自己的最低版本：WordPress 需要 PHP 7.4，其他应用要求更高。WordPress 工具箱仍然可以管理运行在旧版本上的 WordPress 网站，但不会在那里更新 WordPress 本身，因为当前的 WordPress 需要 PHP 7.4。它仍会更新插件和主题；WP-CLI 会跳过需要更新版本 PHP 的更新。
- 网站的 PHP-FPM 池和 php.ini 设置与较新版本相同：面板写入的每项设置在 PHP 5.6 中都存在。

`packages.sury.org/php` 为 Ubuntu 22.04、24.04、26.04 和 Debian 12、13 提供 PHP 5.6 到 7.3，适用于 amd64 和 arm64。安装时还会向 apt 确认；如果软件源没有该版本，会拒绝并说明原因。

### 使用过 ondrej/php PPA 的服务器 {#ondrej-php-ppa}

早期版本的面板在 Ubuntu 上从 `ppa:ondrej/php` 获取额外的 PHP 版本。这个 PPA 已经冻结，不再接收更新：它的维护者现在在 `packages.sury.org/php` 发布 Ubuntu 软件包（见 [launchpad.net/~ondrej/+archive/ubuntu/php](https://launchpad.net/~ondrej/+archive/ubuntu/php) 上的通知）。

当面板在仍有这个 PPA 的 Ubuntu 服务器上启动时，它会按照 PPA 通知中描述的方式，把服务器切换过去一次：

1. 它添加带固定密钥的 `packages.sury.org/php`（`/usr/share/keyrings/sury-php.gpg` 和 `/etc/apt/sources.list.d/sury-php.list`），除非已经有它的软件源。
2. 它把 PPA 在 `/etc/apt/sources.list.d` 中的源文件，以及 PPA 的密钥 `/etc/apt/trusted.gpg.d/ondrej-ubuntu-php.gpg`（如果有），移到面板数据文件夹中的 `apt-backup`（`/var/lib/nefo/apt-backup`）。那个密钥对所有软件源都受信任，而且已经没有东西需要它。其他 PPA，即使是同一个维护者的，也不会被动。

不会安装、移除或降级任何东西，面板启动时也不会运行 `apt-get update`。下一次更新（面板的更新检查、一次安装，或你自己运行 `apt update`）会读取新的软件源，已安装的 PHP 软件包会从那里获得之后的更新。面板日志会说明它改了什么。如果无法下载密钥，PPA 会保留，面板会在下次启动时再试。`/etc/apt/sources.list` 本身中的 PPA 行，或者同时列有其他软件源的文件中的 PPA 行，不会被编辑：日志会请你手动替换。

### 扩展 {#extensions}

**扩展** 标签页列出有该版本软件包的可选扩展。点击 **安装** 或 **移除**，PHP-FPM 会重新加载。

可选扩展有：apcu、gmp、igbinary、imagick、imap、ldap、memcached、mongodb、msgpack、pgsql、redis、soap、sqlite3、ssh2、tidy、xdebug、xsl 和 yaml。

::: warning
xdebug 会让该 PHP 版本上的每个网站变慢。不要在生产服务器上一直装着它。
:::

### php.ini 设置 {#php-ini-settings}

**设置** 标签页为该 PHP 版本上的每个网站设置 php.ini 的值。字段留空表示使用 PHP 的默认值，然后点击 **保存**。

| 设置 | 默认值 | 允许的值 |
|---|---|---|
| `date.timezone` | UTC | 时区名称，例如 `Europe/Berlin` |
| `max_input_vars` | 1000 | 100 到 100000 |
| `max_input_time` | 60 | -1 到 3600 |
| `default_socket_timeout` | 60 | 1 到 600 |
| `session.gc_maxlifetime` | 1440 | 60 到 2592000 |
| `realpath_cache_size` | 4096K | 一个数字，后面可以加 K 或 M |
| `display_errors` | Off | On 或 Off |
| `expose_php` | Off | On 或 Off |
| `allow_url_fopen` | On | On 或 Off |
| `short_open_tag` | Off | On 或 Off |
| `opcache.enable` | 1 | 1 或 0 |
| `opcache.memory_consumption` | 128 | 32 到 2048 |
| `opcache.max_accelerated_files` | 10000 | 1000 到 1000000 |
| `opcache.validate_timestamps` | 1 | 1 或 0 |
| `opcache.revalidate_freq` | 2 | 0 到 3600 |

这些值写入 `/etc/php/<version>/fpm/conf.d/99-panel.ini`，并用 `php-fpm -t` 检查。如果 PHP-FPM 拒绝它们，之前的文件会被放回去，什么都不改变。内存、上传大小和时间限制是在网站的设置中按网站设置的。

### ionCube Loader {#ioncube}

PHP 版本下方的 **ionCube Loader** 卡片用于运行用 ionCube Encoder 编码的 PHP 文件。有些商业 PHP 脚本需要它，例如 WHMCS 和其他计费或授权软件，从 cPanel 迁移来的网站也常常需要。

1. 先安装网站使用的 PHP 版本。
2. 在 **ionCube Loader** 卡片上，点击 **安装**。

面板通过 HTTPS 从 ionCube（`downloads.ioncube.com`）下载加载器。它安装一个固定的版本 15.5.1，并对照面板为它保存的 SHA-256 校验和检查下载，因为 ionCube 不发布校验和或签名。不匹配的下载会被拒绝，什么都不改变。它适用于 amd64 和 arm64 服务器。

加载器放在 `/usr/local/lib/ioncube` 中，附带 ionCube 的许可证（`LICENSE.txt`）。对于加载器支持的每个已安装 PHP 版本，面板会在 `/etc/php/<version>/fpm/conf.d` 和 `/etc/php/<version>/cli/conf.d` 中写入 `00-ioncube.ini`，让加载器按 ionCube 的要求在 OPcache 之前加载。它会检查 `php<version> -v` 能否带着加载器启动，然后重新加载 PHP-FPM。如果 PHP 带着它无法启动，面板会再把这个文件拿掉，让该版本在没有加载器的情况下继续工作，任务会说明是哪个版本失败了。

卡片列出每个已安装的 PHP 版本，以及加载器对它的状态：

| 状态 | 含义 |
|---|---|
| **已加载** | 加载器已为 PHP-FPM 和命令行加载 |
| **未加载** | 存在适用于该版本的加载器，但没有加载。在卡片上点击 **加载到所有版本** |
| **ionCube 没有为此版本提供加载器** | PHP 8.0：ionCube 从未为它发布加载器 |
| “尚不支持 PHP 8.6” | 比面板安装的加载器更新的 PHP 版本（15.5.1 最高支持 PHP 8.5） |

固定的版本有适用于 PHP 5.6、7.0 到 7.4 以及 8.1 到 8.5 的加载器。之后安装的 PHP 版本如果有对应的加载器，也会得到它。卸载某个 PHP 版本会移除它的 `00-ioncube.ini`。

**卸载** 会从每个 PHP 版本中拿掉加载器，重新加载 PHP-FPM，并删除 `/usr/local/lib/ioncube`。之后编码过的 PHP 文件将无法运行。

要检查某个网站，在它的文件夹中放一个这样的文件并打开它，然后删除这个文件：

```php
<?php var_dump(extension_loaded('ionCube Loader'));
```

::: tip
ionCube Loader 可以免费使用，但不是开源软件。ionCube 的许可证（[ioncube.com/legal/loaders/eula](https://www.ioncube.com/legal/loaders/eula)）允许免费使用它。面板在你安装时从 ionCube 下载它，并不随面板分发。
:::

## Node.js {#node-js}

**Node.js** 卡片会从 nodejs.org 安装最新的 LTS 版本，并对照它公布的 SHA-256 校验和进行检查。它安装到 `/opt/node-v<version>-linux-<arch>`，`node`、`npm` 和 `npx` 会链接到 `/usr/local/bin`。这是服务器的默认 Node.js，没有选择版本的应用网站会使用它。

### 多个 Node.js 版本 {#several-node-js-versions}

安装 Node.js 后，**Node.js 版本** 卡片会列出最新的三个 LTS 系列，以及任何已安装的系列。点击 **安装** 可以在其他系列旁边再添加一个。这不会改变默认版本。

在 **网站** → 你的网站 → **运行环境** 中为应用网站选择一个系列。有应用网站使用的系列和默认版本不能从这张卡片卸载。默认版本请改从 **Node.js** 卡片卸载。

这个列表来自 nodejs.org，所以服务器需要能访问互联网才能显示它。

::: warning
卸载默认的 Node.js 也会让调用 `node` 的定时任务和后台程序无法工作。
:::

## Python {#python}

**Python 应用** 卡片会安装 `python3-venv` 和 `python3-pip`，让应用网站可以运行 Python（Django、Flask、FastAPI……）。每个 Python 应用网站在 `app/.venv` 中有自己的虚拟环境。不会安装 `build-essential` 和 `python3-dev` 这类构建工具，所以没有现成 wheel、必须编译的软件包会安装失败，直到你用 `apt` 添加这些工具。

卸载会移除 venv 和 pip。`python3` 本身会保留，因为系统需要它。

## Java {#java}

**Java（OpenJDK）** 卡片会从你的发行版安装 OpenJDK，供用 Java 编写的应用网站使用，例如用 `java -jar` 启动的 Spring Boot 应用。它是无头 JDK（`openjdk-<version>-jdk-headless`）：`java`，加上 [Git 部署](./git-deploy) 脚本中构建所需的 `javac`、`jar` 和其他工具，不含桌面部分。它安装到 `/usr/lib/jvm/java-<version>-openjdk-<arch>`，并随系统其他部分一起获得安全更新。

卡片上的 **安装** 会安装发行版中最新的长期支持（LTS）版本。有哪些版本取决于系统：

| 系统 | Java 版本 |
|---|---|
| Ubuntu 22.04 | 17、21、25 |
| Ubuntu 24.04 | 17、21、25 |
| Ubuntu 26.04 | 17、21、25 |
| Debian 12 | 17 |
| Debian 13 | 21、25 |

### 多个 Java 版本 {#several-java-versions}

安装 Java 后，**Java 版本** 卡片会列出发行版提供的 LTS 版本，以及任何已安装的版本。点击 **安装** 可以在其他版本旁边再添加一个。发行版有了更新的 LTS 版本后，它会自动出现。

在 **网站** → 你的网站 → **运行环境** 中为应用网站选择一个版本（见 [Java 应用网站](./websites#java-app-websites)）。有应用网站使用的版本不能卸载，卡片会显示有应用在使用它。**Java（OpenJDK）** 卡片上的 **卸载** 会移除每个已安装的版本，只要有应用网站运行 Java 就会被拒绝。

每个版本连同下载的软件包，大约占用 390 到 480 MB 磁盘空间。没有网站使用它时，什么都不运行。

## Redis {#redis}

Redis 是一个内存缓存。WordPress 可以通过 Redis 对象缓存插件使用它。

安装 Redis 时，面板会：

- 为每个已安装的 PHP 版本安装 `redis-server` 和 redis 扩展；
- 按服务器限制 Redis 的内存：内存低于 1.5 GB 时为 64 MB，1.5 GB 起为 128 MB，3 GB 起为 256 MB；
- 设置策略 `allkeys-lru`，让 Redis 在满了时像缓存一样丢弃最近最少使用的键；
- 让 Redis 监听 `127.0.0.1:6379`，只能从这台服务器访问。

要查看 Redis 中的内容，前往 **数据库** → **Redis**。见 [数据库](./databases#redis-key-browser)。

卸载 Redis 之前，请先关闭网站的缓存插件。没有缓存它们也能继续工作，只是稍慢一些。

## PostgreSQL {#postgresql}

**PostgreSQL** 卡片会安装发行版自带的版本，或者 PostgreSQL 项目软件源（apt.postgresql.org）中的另一个主版本，由你在安装对话框中选择。面板会在一个 `conf.d` 配置片段中按服务器的内存调优它，让它只监听 localhost，并写入一个 `pg_hba.conf`，除了本地套接字上的 `postgres` 系统用户，其他所有人都需要密码。它的数据库在 **数据库** → **PostgreSQL** 上。版本、升级、远程访问、导入规则和备份见 [PostgreSQL](./postgresql)。

来自 apt.postgresql.org 时，卡片显示 **来自 apt.postgresql.org**；否则显示 **来自发行版**。卸载它也会移除那个软件源（如果是面板添加的）。

安装了 Adminer 时，面板会为 Adminer 使用的 PHP 版本添加 PHP 的 `pgsql` 扩展，这样 Adminer 也能打开 PostgreSQL 数据库。这个扩展只会和 PostgreSQL 一起添加。

## Memcached {#memcached}

Memcached 是一个小型内存缓存，用于 PHP 会话和应用缓存（例如 Laravel 的 `memcached` 缓存驱动）。

安装 Memcached 时，面板会：

- 为每个已安装的 PHP 版本安装 `memcached` 和 memcached 扩展，并为之后安装的 PHP 版本添加这个扩展；
- 写入 `/etc/memcached.conf`：Memcached 只通过 TCP 监听 `127.0.0.1:11211`，关闭 UDP（`-U 0`），这样它不能被用来放大攻击；
- 按服务器限制它的内存：内存低于 1.5 GB 时为 64 MB，1.5 GB 起为 128 MB，3 GB 起为 256 MB；内存低于 1.5 GB 时允许 512 个连接，1.5 GB 起允许 1024 个。

内存满了时，Memcached 会丢弃最近最少使用的条目。要查看它的数据，前往 **数据库** → **Memcached**。

卸载 Memcached 之前，请把网站的会话和缓存改到别处：缓存驱动为 Memcached 的应用没有它就会出错。

## 数据库服务器、Adminer 和 phpMyAdmin {#mariadb-and-adminer}

**数据库服务器** 卡片会安装 MariaDB 或 MySQL 8.4 LTS，按服务器的内存调优，只能从这台服务器访问。一台服务器只装其中一个。

两者都没有安装时，卡片会提供选择，默认选中 **MariaDB**：它更轻量，WordPress 和大多数 PHP 应用都运行在它上面。

- 在 MariaDB 自己的软件源有比发行版更新的 LTS 系列时，可以在 **MariaDB** 下选择版本：发行版自带的版本（默认，例如“10.11，发行版自带”），或 **来自 MariaDB 的 11.4 LTS**、**来自 MariaDB 的 11.8 LTS** 或 **来自 MariaDB 的 12.3 LTS**（在 Ubuntu 26.04 和 Debian 13 上只有 12.3）。每个选项都会说明 MariaDB 支持它到什么时候。见 [MariaDB 版本](./databases#mariadb-versions)。
- **MySQL 8.4 LTS** 来自 Oracle 的软件源，在 Ubuntu 22.04、24.04、26.04 和 Debian 12、13 上提供，仅限 amd64 服务器。在其他地方，卡片会说明为什么不提供。

安装其中一个后，卡片会显示是哪一个、它的版本和来源（**来自发行版的软件包**、**来自 MariaDB 官方软件源** 或 **来自 Oracle 的软件源**）、它的内存估算、数据库的数量、**打开数据库** 和 **卸载**。它还提供：

- **升级**（MariaDB 上，在有更新的 LTS 系列时）：见 [升级 MariaDB](./databases#upgrade-mariadb)。
- 另一个服务器作为替代，即 **改用 MySQL 8.4 LTS** 或 **改用 MariaDB**，并带有切换按钮：面板会把数据库、它们的用户和密码迁移过去，任何一步失败都会回退。见 [在 MariaDB 和 MySQL 之间切换](./databases#switch-between-mariadb-and-mysql)。
- **保留的转储**：升级或切换时生成的导出，带有它们的文件夹、大小和删除按钮。见 [升级和切换生成的转储](./databases#dumps-from-upgrades-and-switches)。

Adminer 和 phpMyAdmin 在浏览器中显示你的数据库，只能通过面板的登录访问。两者都需要 Web 服务器和 PHP，都适用于 MariaDB 和 MySQL。见 [数据库](./databases#phpmyadmin)。

## Docker {#docker}

**Docker** 卡片会从你的发行版安装 `docker.io`，有 Compose 插件时一并安装。之后会出现 **Docker** 菜单项。

Docker 容器发布的端口会绕过 ufw 防火墙。所以面板只在 127.0.0.1 上发布容器端口，你要通过网站访问它们。见 [Docker](./docker)。

## Web 防火墙和 ClamAV {#web-firewall-and-clamav}

- **Web 防火墙（ModSecurity）** 会安装 ModSecurity 和 OWASP Core Rule Set。它需要 Web 服务器。安装后，在 **网站** → 你的网站 → **Web 防火墙（WAF）** 中按网站开启它。如果发行版没有适用于该 Web 服务器的 ModSecurity 模块，卡片会说明原因，并且不提供安装：Ubuntu 22.04 上没有 Nginx 的模块。Apache 和 OpenLiteSpeed 在那里可以使用，从 Ubuntu 24.04 起 Nginx 也可以。
- **ClamAV 杀毒软件** 是恶意软件扫描的可选附加引擎。它的病毒库在安装后立即开始下载，所以几分钟后扫描就能用上它。它适合 2 GB 或以上内存的服务器。

两者见 [安全](./security)。

## 工具：Composer 和 WP-CLI {#tools}

**工具** 分组显示两个命令行工具，面板会在有东西需要它们时安装。每张卡片显示该工具是否已安装、它的版本、来源以及谁在使用它。**更新** 会安装该工具最新的官方版本。面板通过 HTTPS 下载它，并对照项目在旁边公布的校验和，拒绝不匹配的下载：这时已安装的工具保持原样。新文件一步替换旧文件。

### Composer {#composer}

Composer 是 PHP 的包管理器。从应用商店安装 Laravel 或 Drupal 时会安装它，Git 部署脚本和终端也可以运行它。

- **安装** 会安装发行版的 `composer` 软件包，它随系统其他部分一起获得安全更新。
- **更新** 会把 getcomposer.org 上最新的稳定版本安装到 `/usr/local/bin/composer`，并对照它公布的 SHA-256 校验和进行检查。它在 `PATH` 中排在前面，所以终端中的 `composer`、Git 部署脚本以及面板自己安装 Laravel 和 Drupal 时都会使用它。发行版的软件包仍然保留。
- **卸载** 会移除两者。只要有 Git 部署脚本运行 `composer`，就会被拒绝。Laravel 和 Drupal 网站不会妨碍卸载：它们只在安装时需要 Composer，没有它也能继续工作。再次安装其中之一时会重新装上 Composer。

### WP-CLI {#wp-cli}

WP-CLI 从命令行运行 WordPress。[WordPress 工具箱](./wordpress) 用它进行更新、维护模式、密码和预发布副本，并在第一次需要时安装它。

- **安装** 和 **更新** 会从 GitHub 下载最新版本到 `/usr/local/lib/panel/wp-cli.phar`，并对照该版本公布的 SHA-512 校验和进行检查。
- 只要存在 WordPress 网站，**卸载** 就会被拒绝。
