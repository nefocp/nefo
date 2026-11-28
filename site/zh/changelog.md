---
title: 更新日志
description: Nefo 每个发布版本的变化（最新的在最前面），以及更新前需要了解的事项。
---

# 更新日志

Nefo 的每个发布版本，最新的在最前面，包括新增、变更和修复的内容，以及更新前需要了解的事项。每个版本都链接到它在 GitHub 上的页面，那里有 amd64 和 arm64 的程序、安装程序、校验和及其签名。如何更新，见 [更新](./docs/update)。

::: warning 更新后回退
v0.21.0、v0.22.0、v0.23.0 和 v0.24.0 第一次启动时都会修改面板数据库的格式。此后，旧版本会拒绝使用这些数据启动。如果你可能需要回退，请在更新之前复制 `/var/lib/nefo`，方法见 [更新](./docs/update#if-an-update-fails)。
:::

## v0.24.1 {#v0-24-1}

2026 年 11 月 28 日 · [GitHub 上的发布页面](https://github.com/nefocp/nefo/releases/tag/v0.24.1)

网站备份可以恢复应用网站和反向代理网站，安全检查更快，并修复了七种语言中的问题。

**变更**

- **网站备份会记录网站的类型**，所以在没有该网站的服务器上恢复时，应用网站（Node.js、Python、Java）会连同它的运行环境、启动命令、端口和内存上限一起恢复，反向代理网站会连同它的目标地址和选项一起恢复。请先在 **软件** 页面安装应用的运行环境。旧版本做的备份仍会恢复为静态网站。Docker 应用网站的备份会被拒绝，并给出要做的步骤：先从应用商店安装该应用，再恢复该应用的备份。见 [在新服务器上恢复](./docs/restore-new-server)。
- **安全检查会同时运行各项检查**，它询问的每个程序最多 5 秒，整个检查最多 25 秒。来不及完成的检查会标为 **结果不完整**，不会拖住其他检查。见 [安全检查](./docs/security-check)。

**修复**

- 在繁忙的服务器上，安全检查可能运行很久，最后以服务器错误结束。
- 在有内存上限的容器中，仪表盘把页面缓存和程序需要时内核会释放的内存算作已用，所以在一次备份或一次大文件下载之后，内存看起来几乎用满。交换空间的设置现在也显示容器的内存。
- 在添加了 PHP 软件源的服务器上，软件源最新的 PHP（Ubuntu 24.04 上是 8.6）被当成了发行版自带的 PHP 版本：**软件** 页面把两个版本的来源弄反了，之后安装的 Web 服务器也装上了这个版本，而不是发行版的版本。
- 较新的应用商店目录新增的应用，在应用商店页面以外没有图标。
- 在文字较长的语言中，账户菜单的语言列表会被截断（现在分两列），网站面板中的标签页名称会被截断（现在可以占两行），窄屏上的顶部栏可能折到第二行。
- 修正了界面中的部分中文、俄语和印度尼西亚语文字。

## v0.24.0 {#v0-24-0}

2026 年 11 月 27 日 · [GitHub 上的发布页面](https://github.com/nefocp/nefo/releases/tag/v0.24.0)

七种界面语言、备份到 Google Drive、在新服务器上恢复，以及安全检查。

**新增**

- **七种语言的界面**：在英语和越南语之外，新增中文、西班牙语、葡萄牙语、俄语和印度尼西亚语，包括应用商店的文字。每种语言都是一个单独的文件，浏览器只在用到时才下载；界面的文件现在以 gzip 压缩发送：主脚本从 478 KB 减到 163 KB。
- **备份到 Google Drive**，使用你自己的 OAuth 客户端，通过设备代码登录，只能访问面板创建的文件，上传中断后可以继续。见 [Google Drive](./docs/backups#google-drive)。
- **加密的远程副本**：每个存储（S3、SFTP、Google Drive）都可以用密码短语加密它收到的备份：scrypt 加 AES-256-GCM，分成编号的块，所以被截断或改动过的副本会被拒绝。`nefo decrypt-backup` 可以手动打开副本。见 [加密副本](./docs/backups#encrypt-the-copies)。
- **在新服务器上恢复**：**查找备份** 会列出存储上这个面板不知道的备份，恢复时会创建这里还没有的网站，包括它的域名、别名、PHP 版本、文件，以及数据库及其密码。恢复已删除网站的备份时，也会以同样的方式重新创建它。见 [在新服务器上恢复](./docs/restore-new-server)。
- **网站**：可以克隆 Node.js、Python 和 Docker 应用网站，以及带有 PostgreSQL 数据库的网站。删除网站时可以保留它的数据库，并且默认先做备份。新的网站类型“反向代理”可以转发到任何地址，支持 Nginx、Apache 和 OpenLiteSpeed。见 [网站](./docs/websites#reverse-proxy-websites)。
- **安全检查**：对服务器做 22 项快速检查（SSH、防火墙、更新、对互联网开放的数据库或 Redis、绕过 ufw 的 Docker 端口、可通过 HTTP 访问的文件、证书、PHP 设置……），每项都带修复链接，可随时运行或每天运行。见 [安全检查](./docs/security-check)。
- **登录告警**：SSH 登录（每次，或只在来自新地址时）、一连串失败的 SSH 登录，以及面板登录。见 [登录告警](./docs/mail-alerts#sign-in-alerts)。
- 每个网站的 **国家/地区拦截**，使用 DB-IP 的免费国家数据，只在有网站使用时才下载。它也适用于反向代理网站。见 [国家/地区拦截与文件锁定](./docs/website-protection)。
- 每个网站的 **文件锁定**：它的代码对它的 PHP 和 SFTP 账户变为只读，只有它必须写入的文件夹除外，还可以在 PHP 文件变化时每天告警。`nefo unlock-files` 会在你移除 Nefo 之前把文件交还给网站。
- **数据库**：来自 MariaDB 软件源的 MariaDB 11.4、11.8 和 12.3 LTS，以及先做转储、失败时可以回退的原地升级；在 MariaDB 和 MySQL 之间切换并迁移数据库；慢查询日志轮转。见 [数据库](./docs/databases#mariadb-versions)。
- **PostgreSQL**：PostgreSQL 项目提供的 15 到 19 版，按数据库开启的远程访问（TLS、scram-sha-256、每个地址一条防火墙规则），以及先做转储、可以回退的主版本升级。升级保留的转储会被列出，并带有删除按钮。见 [PostgreSQL](./docs/postgresql)。
- **DNS 页面上的邮件记录**：通过 Cloudflare，为 Google Workspace、Microsoft 365、Zoho Mail、Cloudflare Email Routing 或其他服务商设置 MX、一条合并的 SPF 记录和 DMARC。见 [DNS（Cloudflare）](./docs/dns#set-up-email-records)。
- **软件**：供应用网站使用的 Java（OpenJDK 17、21 和 25），每个 PHP 版本的 ionCube Loader，以及 PHP 8.6（PHP 软件源有了之后会自动提供）。见 [软件](./docs/software)。
- **应用商店**：新增 15 个应用，共 42 个：Grafana、ntfy、Gotify、Memos、linkding、Navidrome、Vikunja、Actual Budget、changedetection.io、Healthchecks、listmonk、Kanboard、FreshRSS、Jellyfin 和 Mealie。见 [应用商店](./docs/apps)。

**变更**

- 内存数字以容器自己的内存上限（LXC）为准，包括仪表盘、告警、配置页面和应用商店的检查。
- 修正了配置页面的磁盘数字；在从未下载过软件包列表的服务器上，会把 apt 的软件包列表计算在内。
- 较新的应用商店目录中如果有旧版面板无法运行的应用，只会略过这些应用。较新的目录新增的应用会在所有地方显示它们的名称。

**修复**

- PHP 8.5 无法安装，因为 OPcache 不再有单独的软件包。
- 在 Ubuntu 26.04 上，安装 PostgreSQL（或其他服务）时，如果 systemd 在 apt 之后重新执行自己，安装可能失败。现在面板会在 systemd 无法连接时重试 `systemctl`。
- 在发行版没有适用于该 Web 服务器的 ModSecurity 模块的地方（Ubuntu 22.04 加 Nginx），也会提供 Web 防火墙。现在它会显示为不提供，并说明原因。
- WordPress 页面为空时的文字在句子中间就断了。

## v0.23.0 {#v0-23-0}

2026 年 11 月 26 日 · [GitHub 上的发布页面](https://github.com/nefocp/nefo/releases/tag/v0.23.0)

API 密钥、网站导入、配置页面，以及更多数据库服务器和 PHP 版本。

**新增**

- **API 密钥**，供你自己的脚本使用，位于 **设置** → **API 密钥**：只读或完全访问、地址允许列表和到期日期。密钥通过 `Authorization: Bearer` 请求头发送，只以哈希形式保存；错误的密钥算作一次登录失败。任何密钥都无法打开终端、Adminer、phpMyAdmin、账户或密钥本身。只读密钥无法读取密码、文件、导出或备份下载。见 [API 密钥](./docs/api)。
- **导入网站**：从 cPanel 完整备份导入（网站、数据库及其用户和密码、证书和定时任务），或者从 aaPanel 或其他服务器制作的网站压缩包和 SQL 转储导入。恶意构造的压缩包会被拒绝，MySQL 8 的转储会为 MariaDB 调整，导入失败时会移除它创建的一切。见 [导入网站](./docs/import)。
- 第一次登录时的 **配置页面**：在一个页面上选择 Web 服务器、PHP 版本、数据库服务器和附加组件，查看它们大约需要多少内存和磁盘，并作为一个任务安装。`nefo setup` 在命令行上做同样的事，安装程序也接受同样的选项。不带选项时，安装程序的行为与以前相同。见 [入门步骤](./docs/first-steps#set-up-the-server)。
- **MySQL 8.4 LTS**，作为 MariaDB 的替代（每台服务器二选一），来自 Oracle 的软件源，并固定了其密钥。只在 amd64 服务器上提供。
- **phpMyAdmin**，只能通过面板的登录访问。
- **数据库** 页面上的 **PostgreSQL**：数据库、所有者、导入和导出，以及备份。
- **Memcached**，只在服务器本身上监听，不开启 UDP。
- **PHP 5.6 到 7.3**，用于从旧服务器迁来的网站。它们只在你确认后才会安装，并且永远不能成为服务器上唯一的 PHP 版本。
- **PHP 生命周期结束标记**：已过 PHP 官方安全支持期的版本，会在 **软件** 页面和使用它的网站上标出。发行版自带的 PHP 版本永远不会被标记，因为发行版会继续为它打补丁。见 [软件](./docs/software#end-of-life)。
- **软件** 页面上的 **Composer** 和 **WP-CLI**。

**变更**

- 在 Ubuntu 上，额外的 PHP 版本现在来自 `packages.sury.org`，因为 `ppa:ondrej/php` 不再获得更新。有这个 PPA 的服务器会在面板启动时切换一次：PPA 的源文件被移到 `/var/lib/nefo/apt-backup`，不安装也不移除任何东西，已安装的 PHP 软件包会在下一次 `apt-get update` 之后从新的软件源更新。

**修复**

- 以暂停状态创建的网站，现在也会以暂停状态保存。
- 面板为其他程序创建的文件夹，不再带有面板自己的严格权限（umask 077）。
- 在全新的服务器上，即使 apt 还没有下载软件包列表，也能识别发行版自带的 PHP 版本。
- 单独的 `@`（即域名本身）不再导致 DNS 页面上的消息出错。
- 崩溃的任务现在会把它的堆栈跟踪写入面板的日志。

## v0.22.0 {#v0-22-0}

2026 年 11 月 26 日 · [GitHub 上的发布页面](https://github.com/nefocp/nefo/releases/tag/v0.22.0)

网站监控、更多告警渠道和 Cloudflare DNS。

**新增**

- **网站监控**：每隔 1 到 10 分钟检查一次每个网站（状态码、可选的关键词和证书的到期时间）。连续两次检查失败后，网站算作宕机；网站宕机和恢复时你都会收到告警。面板会显示 24 小时和 7 天的可用率、响应时间和最近的失败。监控请求不计入流量统计。见 [监控](./docs/websites#monitoring)。
- **通过 Discord、Slack 和 Webhook 发送告警**。Webhook 用 HMAC-SHA256 签名。已保存的 URL 会被遮盖显示；发送告警时，会拒绝服务器本身的地址、链路本地地址和云元数据地址。见 [邮件与告警](./docs/mail-alerts)。
- **Cloudflare 区域的 DNS 页面**：列出、添加、编辑和删除记录，开启或关闭代理，清除缓存，并在预览之后把网站的域名指向这台服务器。见 [DNS（Cloudflare）](./docs/dns)。

## v0.21.0 {#v0-21-0}

2026 年 11 月 26 日 · [GitHub 上的发布页面](https://github.com/nefocp/nefo/releases/tag/v0.21.0)

系统工具、SSH 密钥和更多网站选项。

**新增**

- **系统页面**：操作系统更新（标出安全更新，可以安装所有更新或仅安装安全更新，自动安全更新，以及需要重启时的提醒）、交换文件和 swappiness、时区、时钟同步、主机名、TCP BBR 和磁盘清理。后台的更新检查会在面板启动后等待 30 分钟，并复用最近 6 小时内刷新过的软件包列表。见 [系统](./docs/os)。
- **防火墙与 SSH** 页面上的 **SSH 密钥**：列出、添加和移除。私钥、弱密钥和重复的密钥会被拒绝，最后一条进入的途径不能移除。
- **网站**：关闭 HTTPS 并删除证书，三种 Web 服务器都支持；HSTS，可选 `includeSubDomains`；Nginx 上每个 PHP 网站的页面缓存，带清除按钮；连同文件、设置和数据库克隆 PHP 网站或静态网站；在备份之后把 WordPress 预发布副本推送回正式网站。

**修复**

- 面板自己运行的 apt 永远不会重叠，遇到其他程序的 apt 锁时会等待，而不是失败。
- 在 `/etc/hosts` 被绑定挂载的容器中，主机名会原地写入。系统拒绝的主机名会给出清楚的错误。

## v0.20.2 {#v0-20-2}

2026 年 11 月 26 日 · [GitHub 上的发布页面](https://github.com/nefocp/nefo/releases/tag/v0.20.2)

**变更**

- Nginx 现在会压缩 CSS、JavaScript、SVG 和字体。Debian 和 Ubuntu 只为 HTML 开启压缩，所以这些文件之前是未压缩发送的。`nginx.conf` 中已有的设置保持不变。现有服务器会在更新后的面板启动时得到这项改动。

**修复**

- 每个发布版本现在都带有带签名的应用商店目录。之前，面板每天查找一次目录，并记录一个 HTTP 404。

## v0.20.1 {#v0-20-1}

2026 年 11 月 26 日 · [GitHub 上的发布页面](https://github.com/nefocp/nefo/releases/tag/v0.20.1)

**修复**

- 安装程序在每个受支持的系统上都会失败：它从 `/etc/os-release` 中读取的是系统的版本，而不是它自己的版本，并试图下载一个不存在的发布版本。修复后已在全新的 Ubuntu 24.04 和 Debian 12 服务器上测试过。

## v0.20.0 {#v0-20-0}

2026 年 11 月 26 日 · [GitHub 上的标签](https://github.com/nefocp/nefo/releases/tag/v0.20.0)

第一个公开版本。它的安装程序会失败（在 v0.20.1 中修复），GitHub 上也没有它的发布文件，只有标签。请安装更新的发布版本。

它包含：运行在 Nginx、Apache 或 OpenLiteSpeed 上、使用 PHP-FPM 的网站，Node.js 和 Python 应用，Let's Encrypt 证书，从 Git 部署和 WordPress 工具箱；带有 PHP 应用和 Docker 应用的应用商店；MariaDB、Redis、文件管理、Web 终端，以及备份到 S3 兼容存储和 SFTP；带防锁定保护的防火墙（ufw）、fail2ban 和 SSH 设置，Web 应用防火墙和恶意软件扫描器；带签名的发布版本和 `nefo update`；以及英语和越南语的界面。
