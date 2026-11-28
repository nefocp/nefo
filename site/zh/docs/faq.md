---
title: 常见问题
description: 关于免费 Linux 服务器控制面板 Nefo 的常见问题简答：是否免费、许可证、支持的系统、内存、数据库、Docker、迁移网站、安全性、隐私、更新、卸载和语言。
---

# 常见问题

这里简要回答大家最常问的关于 Nefo 的问题。Nefo 是一款免费、开源的控制面板，管理一台 Linux 服务器。每个回答先给出简短的答案，再给出细节，并链接到详细介绍的页面。这些信息适用于 Nefo v0.24.1。

## Nefo 是什么？ {#what-is-nefo}

Nefo 是一款免费、开源的控制面板，用一条命令安装在你自己的 Linux 服务器（VPS）上，在浏览器中管理网站、SSL 证书、数据库、应用、文件、备份和安全。它是单个程序 `/usr/local/bin/nefo`，Web 界面内置其中，以 systemd 服务 `nefo` 运行。见 [Nefo 是什么？](./)。

## Nefo 免费吗？ {#is-nefo-free}

免费。Nefo 是遵循 GNU AGPLv3 的自由软件，所有功能对所有人免费，企业也一样。没有付费版本，没有锁定的功能，没有许可证密钥，也不需要注册账户。见 [关于 Nefo](../about#licence)。

## Nefo 可以运行在哪些系统上？ {#what-does-nefo-run-on}

Nefo 可以运行在 64 位 x86（amd64）和 64 位 ARM（arm64）上的 Ubuntu 22.04、24.04 和 26.04，以及 Debian 12 和 13 上，需要 root 权限和 systemd。安装程序已在每个系统的全新服务器上测试过，平台为 x86-64。arm64 版本还没有在真实的 ARM 硬件上测试过，所以请先在可以重建的服务器上试用。在其他系统上，除非你强制安装，安装程序会停止。见 [系统要求](./requirements)。

## Nefo 需要多少内存？ {#how-much-ram-does-nefo-need}

面板本身约占用 34 MB 内存（在测试服务器上更新到 v0.24.0 后测得），Nefo 为 1 GB 内存的 VPS 设计。你安装的软件会在此基础上增加占用，**软件** 页面会显示大约多少：Nginx 约 15 MB，每个 PHP 版本约 25 MB，MariaDB 为它的缓冲池加约 100 MB（在 1 GB 服务器上约 228 MB）。在 1 GB 服务器上，请添加 1 到 2 GB 交换空间。见 [系统要求](./requirements#memory-and-disk)。

## Nefo 支持 MySQL、PostgreSQL 和 Docker 吗？ {#does-nefo-support-mysql-postgresql-and-docker}

支持。Nefo 运行 MariaDB（发行版自带的版本，或 11.4、11.8、12.3 LTS）或 MySQL 8.4 LTS，每台服务器一个，并可连同数据库在两者之间切换。PostgreSQL（发行版自带的版本，或 PostgreSQL 项目提供的 15 到 19 版）可以在它旁边运行。Docker 运行容器、Compose 项目以及应用商店中的 Docker 应用。见 [数据库](./databases)、[PostgreSQL](./postgresql) 和 [Docker](./docker)。

## 可以使用哪些 Web 服务器？ {#which-web-servers-can-i-use}

Nginx、Apache 或 OpenLiteSpeed，每台服务器一个。安装 Web 服务器时选择：

- Nginx 最轻量。
- Apache 会读取一些 PHP 应用自带的 `.htaccess` 文件。
- OpenLiteSpeed 会读取 `.htaccess` 文件中的重写规则，并内置 LiteSpeed Cache。

三种 Web 服务器下，PHP 都在 PHP-FPM 中运行，每个网站一个池。网站也可以是 Node.js、Python 或 Java 应用，或者到另一个地址的反向代理。见 [网站](./websites)。

## 以后可以换成另一个 Web 服务器吗？ {#can-i-switch-to-another-web-server-later}

可以，但只能在没有网站的服务器上换。Web 服务器还在为网站提供服务时，面板会拒绝卸载它，因为每个网站的设置都是为这个 Web 服务器写的。先删除网站，在 **软件** 页面卸载 Web 服务器，然后安装另一个。请先备份你的网站。见 [软件](./software#web-server)。

## 能从 cPanel 或 aaPanel 迁移网站吗？ {#can-i-move-my-websites-from-another-panel}

能。Nefo 可以导入 cPanel 完整备份（网站、数据库及其用户和密码、证书和定时任务），也可以导入来自 aaPanel、HestiaCP、CyberPanel 或任何其他服务器的网站压缩包及其 SQL 转储。邮件、DNS 区域和 FTP 账户不会导入。见 [导入网站](./import)。

## Nefo 安全吗？ {#is-nefo-safe}

Nefo 的设计默认就是安全的，但它是以 root 运行的测试阶段软件，还没有经过外部安全审计。面板只在随机端口上的秘密路径下响应，从第一次启动起就使用 HTTPS，支持双因素登录，15 分钟内登录失败 5 次后锁定该地址，并记录每一项更改。发布版本用 Ed25519 签名，内置的 [安全检查](./security-check) 会列出让服务器暴露在外的设置。请先在可以重建的服务器上试用，并把备份保存在服务器之外。见 [安全](./security) 和 [安全策略](../security-policy)。

## Nefo 会向外发送数据吗？ {#does-nefo-send-data-anywhere}

没有遥测，没有使用统计，也没有许可证检查。面板只会主动联系 Let's Encrypt 来签发和续期证书，并且在正式发布版本中，每天下载一次带签名的应用商店目录。其他一切，例如检查更新、告警、远程备份，以及国家/地区拦截使用的国家数据库，都只在你要求或设置后才会发生。见 [隐私](./#privacy)。

## Nefo 能备份到 Google Drive 吗？ {#can-nefo-back-up-to-google-drive}

能，从 v0.24.0 起，使用你自己的 OAuth 客户端，它只能看到面板创建的文件。Nefo 还可以备份到 S3 兼容存储（Amazon S3、Cloudflare R2、Backblaze B2、Wasabi、MinIO）和 SFTP 服务器，可以用密码短语加密每个远程副本，新服务器也能在存储上找到备份并恢复它们。见 [备份](./backups#google-drive) 和 [在新服务器上恢复](./restore-new-server)。

## 如何更新 Nefo？ {#how-do-i-update-nefo}

打开 **设置** → **面板**，点击 **检查更新**，或以 root 身份运行 `nefo update`。更新在替换程序之前会检查发布版本的签名和校验和，然后重启面板；网站会继续运行。Nefo 从不自行更新。见 [更新](./update)。

## 如何卸载 Nefo？ {#how-do-i-uninstall-nefo}

Nefo 没有卸载命令：需要手动移除。先用 `nefo unlock-files` 解锁所有被锁定的网站，停止并移除 `nefo` 服务和 `/usr/local/bin/nefo`，关闭面板的端口，然后删除 `/etc/nefo` 和 `/var/lib/nefo`。除非你事先移除，否则网站和软件会继续运行。见 [卸载](./uninstall)。

## Nefo 能托管我的邮件吗？ {#can-nefo-host-my-e-mail}

不能。Nefo 没有邮件服务器：它不接收邮件，也不托管邮箱。网站和告警可以通过 Gmail、SendGrid、Mailgun 或 Amazon SES 等服务商的 SMTP 中继发送邮件，DNS 页面可以添加你的邮件服务商的 MX、SPF 和 DMARC 记录。邮箱请使用邮件服务商。见 [邮件与告警](./mail-alerts)。

## 能在一个面板中管理多台服务器吗？ {#can-i-manage-several-servers-from-one-panel}

不能。请在每台服务器上安装 Nefo，分别管理。不打算支持在一个面板中管理多台服务器：控制其他服务器的面板需要代理程序，还需要机器之间互相信任。见 [路线图](../roadmap#not-planned-and-why)。

## 能给客户单独的登录账户吗？ {#can-i-give-my-clients-their-own-login}

不能。每台服务器只有一个面板账户 `admin`。Nefo 没有为客户或代理商单独设置的账户。要让客户向他们的网站上传文件，请为该网站开一个 SFTP 账户，它被限制在网站的文件夹内。见 [网站](./websites#sftp-access)。

## 面板支持哪些语言？ {#which-languages-does-the-panel-speak}

从 v0.24.0 起支持七种：英语、越南语、简体中文、西班牙语、葡萄牙语、俄语和印度尼西亚语，包括应用商店的文字。面板跟随浏览器的语言，你可以在登录页面或账户菜单中更改。每种语言都是一个单独的文件，浏览器只在用到时才下载。本文档也提供同样的七种语言。

欢迎参与翻译。把 `web/src/locales/en.json` 复制为以你的语言代码命名的文件，翻译其中的值，然后在 `web/src/i18n.js` 中注册该语言。见 [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md)。

## 如何报告安全问题？ {#how-do-i-report-a-security-problem}

请私下报告，不要发在公开的 issue 中。在 [GitHub 仓库](https://github.com/nefocp/nefo) 中打开“Security”（安全）标签页，选择“Report a vulnerability”（报告漏洞）。请附上你的版本（`nefo version`）、操作系统和重现问题的步骤。见 [安全策略](../security-policy)，它与 [SECURITY.md](https://github.com/nefocp/nefo/blob/main/SECURITY.md) 一致。

## 有更新日志和路线图吗？ {#is-there-a-changelog-and-a-roadmap}

有。[更新日志](../changelog) 按从新到旧列出每个发布版本的变化，以及更新前需要了解的事项。[路线图](../roadmap) 列出 1.0 版本之前正在进行的工作、正在考虑的工作，以及不打算做的事及原因。要在安装之前自己检查发布版本的签名，见 [自行检查发布版本](../security-policy#check-a-release-yourself)。

## 如何帮助这个项目？ {#how-can-i-support-the-project}

- 报告 bug，附上重现步骤、操作系统和 `nefo version`。
- 把面板翻译成你的语言，或改进现有的翻译。
- 修复 issue 并提交 pull request（见 [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md)）。
- 在 [GitHub](https://github.com/nefocp/nefo) 上给项目点星，并告诉其他人。
