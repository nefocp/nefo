---
title: Nefo 是什么？
description: Nefo 是一款免费、开源的控制面板，安装在你自己的 Linux 服务器上，在浏览器中管理网站、数据库、应用、备份和安全。
---

# Nefo 是什么？

Nefo 是一款免费、开源的控制面板（GNU AGPLv3），用一条命令就能安装在你自己的 Linux 服务器上。你可以在浏览器中，用七种语言之一，管理网站及其 SSL 证书、数据库、应用、文件、备份和安全。它是单个程序，为 1 GB 内存的 VPS 设计，不发送任何遥测数据。

本页介绍 Nefo 能做什么、适合谁、有意不做什么，以及它会通过网络发送什么。

## 一个程序就是一个控制面板 {#a-control-panel-in-one-program}

你用一条命令在 VPS 上安装 Nefo。它是单个程序 `/usr/local/bin/nefo`，Web 界面内置其中，以 systemd 服务 `nefo` 运行。之后你就可以在浏览器中管理服务器：网站、证书、数据库、文件、备份、软件和安全。

- **所有功能都免费**。没有付费版本，也不按网站或账户收费。
- **不需要注册账户，没有许可证服务器**。不需要注册或激活任何东西。
- **为小型服务器设计**。Nefo 设计为在 1 GB 内存的 VPS 上运行，支持 x86-64 和 ARM64。面板本身约占用 34 MB 内存，这是在测试服务器上更新到 v0.24.0 后测得的。
- **七种界面语言**：英语、越南语、中文、西班牙语、葡萄牙语、俄语和印度尼西亚语。你可以在登录页面或账户菜单中选择语言，浏览器只下载你使用的语言。

要开始使用，请先查看 [系统要求](./requirements)，然后 [安装 Nefo](./install)，再按照 [入门步骤](./first-steps) 操作。

## 适合谁 {#who-it-is-for}

Nefo 适合自己运行 VPS 的人：为博客搭建第一台服务器的人、部署应用的开发者，或者照看客户网站的自由职业者和代理机构。

每台服务器只有一个面板账户 `admin`。Nefo 没有为客户或代理商单独设置的登录账户。

## 你可以用它做什么 {#what-you-can-do-with-it}

### 网站 {#websites}

- 在 Nginx、Apache 或 OpenLiteSpeed 上托管 PHP 网站、静态网站和应用网站（Node.js、Python 或 Java），以及转发到另一个地址的反向代理网站。每个网站都有自己的 Linux 用户，还可以有一个限制在网站文件夹内的 SFTP 账户。可以把网站克隆到新域名，删除网站时先做备份。见 [网站](./websites)。
- 用自动续期的免费 Let's Encrypt 证书开启 HTTPS，通过 Cloudflare DNS 获取通配符证书，或使用你自己的证书。见 [SSL 证书](./ssl)。
- 按国家/地区拦截或放行访客，并把网站的代码锁定为对其 PHP 只读。见 [国家/地区拦截与文件锁定](./website-protection)。
- 从 GitHub、GitLab、Gitea 或任何 Git 服务器部署，可以手动部署，也可以每次推送时部署。见 [从 Git 部署](./git-deploy)。
- 让 WordPress 网站保持最新，每次更新前先备份数据库，还有维护模式和预发布副本。见 [WordPress 工具箱](./wordpress)。
- 一键安装 42 个应用：WordPress、Joomla、Nextcloud 和 Laravel 等 PHP 应用，以及 Ghost、Uptime Kuma、n8n、Gitea、Grafana 和 Jellyfin 等 Docker 应用。见 [应用商店](./apps)。

### 数据 {#data}

- 运行为小型服务器调优的 MariaDB（发行版自带的版本，或 11.4、11.8、12.3 LTS）或 MySQL 8.4，管理数据库、用户、SQL 导入和导出以及按数据库开启的远程访问，并可连同数据库在两者之间切换。Adminer 和 phpMyAdmin 位于面板的登录之后。浏览 Redis 键，添加 Memcached。见 [数据库](./databases)。
- 运行 PostgreSQL，可以是发行版自带的版本，也可以是 PostgreSQL 项目提供的 15 到 19 版，有它自己的数据库和所有者、经 TLS 的远程访问和主版本升级。见 [PostgreSQL](./postgresql)。
- 从 cPanel 完整备份导入网站，或从 aaPanel、HestiaCP、CyberPanel 或任何服务器制作的网站压缩包和 SQL 转储导入。见 [导入网站](./import)。
- 管理文件：编辑器、上传和下载、压缩包，以及把删除的文件保留 30 天的回收站。见 [文件](./files)。
- 按计划备份网站、数据库、文件夹和应用，保存到这台服务器、S3 兼容存储、SFTP 服务器或 Google Drive，可以用密码短语加密，并能恢复它们，在新服务器上也可以。见 [备份](./backups) 和 [在新服务器上恢复](./restore-new-server)。

### 服务器 {#server}

- 安装和卸载每个组件（Web 服务器、PHP 版本和 ionCube Loader、MariaDB 或 MySQL、PostgreSQL、Redis、Node.js、Python、Java、Docker 等）。仍有网站、数据库或容器在使用的组件，Nefo 会拒绝移除。见 [软件](./software)。
- 运行 Docker 容器和 Compose 项目。Nefo 会检查 Compose 文件中的不安全设置，例如特权容器或对互联网开放的端口。见 [Docker](./docker)。
- 控制防火墙（ufw）、fail2ban、SSH 设置、每个网站的 Web 应用防火墙和恶意软件扫描器。见 [安全](./security)。
- 运行安全检查：22 项快速的服务器检查，每项都链接到修复的位置，可随时运行或每天运行。见 [安全检查](./security-check)。
- 使用浏览器中的终端，管理服务、进程、后台程序和定时任务。见 [系统工具](./system)。
- 监控每个网站是否响应，以及它的可用率和响应时间。见 [监控](./websites#monitoring)。
- 让网站通过 SMTP 中继发送邮件，并通过 Telegram、邮件、Discord、Slack 或 Webhook 接收告警，包括有人登录时。见 [邮件与告警](./mail-alerts)。
- 管理 Cloudflare 区域的 DNS 记录，把网站的域名指向这台服务器，并添加你的邮件服务商的 MX、SPF 和 DMARC 记录。见 [DNS（Cloudflare）](./dns)。

## 面板自身的安全 {#security-of-the-panel-itself}

Nefo 以 root 身份运行，所以它会保护自己的登录：

- 面板只在秘密路径（例如 `/k3x9p2qa7m1z/`）下响应。它端口上的其他所有地址都返回 `404`。
- 从第一次启动起就使用 HTTPS。证书一开始是自签名的，你可以换成面板自己域名的免费证书。
- 登录支持双因素认证。15 分钟内失败 5 次后，该地址会被锁定 15 分钟。
- 登录和更改都会写入活动日志，条目保留 90 天。

## Nefo 不做什么 {#what-nefo-doesn-t-do}

- **没有邮件服务器**。Nefo 不接收邮件，也不托管邮箱。网站和告警通过邮件服务商的 SMTP 中继发送邮件。邮箱请使用邮件服务商。
- **没有 DNS 或 FTP 服务器**。Nefo 管理你的域名在 Cloudflare 的 DNS 记录，每个网站可以用 SFTP 账户代替 FTP。
- **一个面板管理一台服务器**。每台服务器运行自己的 Nefo。不打算支持在一个面板中管理多台服务器（见 [路线图](../roadmap#not-planned-and-why)）。
- **没有客户账户**。每台服务器只有一个管理员账户。
- **导入时不包括邮件、DNS 或 FTP 账户**。从 cPanel 导入会带来网站、数据库、证书和定时任务。见 [哪些内容不会导入](./import#what-is-not-imported)。
- **只支持 Ubuntu 和 Debian**。Ubuntu 22.04、24.04 和 26.04，以及 Debian 12 和 13。见 [系统要求](./requirements)。

## 状态 {#status}

Nefo 还是一款年轻的软件。版本号以 `0.` 开头，在 1.0 版本之前，只有最新的发布版本会获得安全修复。你可以在 **设置** → **面板** 中，或在服务器上用 `nefo version` 查看你的版本。

安装程序已在全新的 Ubuntu 22.04、24.04、26.04 和 Debian 12、13 服务器上测试过，平台为 x86-64。ARM64 版本还没有在真实的 ARM 硬件上测试过。还没有做过外部安全审计。

在把重要网站托付给 Nefo 之前，请先在可以重建的服务器上试用，并把备份副本保存在服务器之外。请在 [GitHub 仓库](https://github.com/nefocp/nefo) 中报告问题。

[更新日志](../changelog) 列出每个发布版本的变化，[路线图](../roadmap) 列出正在进行和不打算做的工作，[安全策略](../security-policy) 说明如何报告漏洞以及如何自行检查发布版本。

## 许可证 {#licence}

Nefo 是遵循 GNU AGPLv3 的自由软件。你可以把它用于任何目的，包括商业用途。如果你通过网络向其他人提供修改过的 Nefo，AGPL 要求你公开你的修改。

## 隐私 {#privacy}

Nefo 没有遥测、没有使用统计，也没有许可证检查。Web 界面不从其他网站加载任何内容：它的字体、图标和脚本都内置在程序中。

面板只会为以下任务主动连接其他服务器：

| 何时 | 连接到 | 原因 |
|---|---|---|
| 证书到期时（每小时检查一次） | Let's Encrypt | 申请或续期网站和面板的证书。面板的 Let's Encrypt 账户没有邮箱地址。除非你使用 DNS 验证，面板会先通过 HTTP 从你的域名获取一个测试文件，检查该域名能否到达这台服务器。 |
| 每天一次（正式发布版本） | 项目的 GitHub 发布页面 | 下载带签名的应用商店目录。没有项目有效签名的目录会被忽略。 |

其他一切都只在你要求时才会发生：

- **更新**：只有你点击 **检查更新** 或运行 `nefo update` 时，Nefo 才会检查新版本。它从不自行更新。见 [更新](./update)。
- **你设置的功能**：告警（Telegram、你的 SMTP 服务器、Discord、Slack 或你的 Webhook）、网站监控（像访客一样按域名访问你的网站）、远程备份（你的 S3 或 SFTP 存储，或 Google Drive）、DNS 验证和 DNS 页面（Cloudflare API）、邮件中继（你的邮件服务商），以及国家/地区拦截：有网站使用它时，每月下载一次 DB-IP 的国家数据库和 Cloudflare 的地址列表。
- **你安装的软件**：软件包通过 apt 来自你的发行版，有些组件来自它们自己的项目，例如来自 nodejs.org 的 Node.js、来自 packages.sury.org 软件源的额外 PHP 版本、来自 LiteSpeed 软件源的 OpenLiteSpeed、你选择时来自各自项目软件源的 MariaDB 和 PostgreSQL 版本，以及来自 ionCube 的 ionCube Loader。有些已安装的软件会自己连接外部：例如，ClamAV 会下载它的病毒库。

安全检查不会向服务器之外发送任何东西。面板发出的大多数请求会标明自己是 `Nefo/<version>`。除了任务需要的信息（例如证书申请中的域名），这些请求不包含关于你的服务器或网站的任何信息。
