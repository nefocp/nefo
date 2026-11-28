---
title: 国家/地区拦截与文件锁定
description: 用 DB-IP 的免费国家数据，按国家/地区拦截或只允许访客访问网站；把网站的代码锁定为对其 PHP 只读，并在 PHP 文件变化时告警。
---

# 国家/地区拦截与文件锁定

从 v0.24.0 起，Nefo 为每个网站提供两项防护。**国家/地区拦截**用 DB-IP 的免费国家数据，拒绝来自你列出的国家/地区的访客，或者只放行这些国家/地区，支持 Nginx、Apache 和 OpenLiteSpeed。文件锁定让网站的代码对它自己的 PHP 和 SFTP 账户只读，只有它必须写入的文件夹（例如上传目录）除外。

两项都在网站设置的 **安全** 分组中：**网站** → 你的网站 → **国家/地区拦截** 和 **文件锁定**。关闭时两者都没有任何开销：不下载东西，不增加进程，不往 Web 服务器里加载任何内容。Web 应用防火墙、速率限制和恶意软件扫描器见 [安全](./security#website-security)。

## 如何在网站上拦截国家/地区？ {#block-or-allow-countries}

1. 打开 **网站**，点击该网站，打开 **国家/地区拦截** 标签页。
2. 选择 **拦截以下国家/地区** 或 **只允许以下国家/地区访问**。
3. 在搜索框中输入国家/地区的名称（用面板的语言）或它的两字母代码，点击它即可添加。点击标签上的 × 可以移除。
4. 点击 **保存**。

服务器上第一次有网站使用国家/地区拦截时，Nefo 会在保存之前从 db-ip.com 下载国家数据库（约 5 MB），需要几秒钟。要再次关闭，选择 **关闭：所有国家/地区都可访问**，然后点击 **保存**。

国家/地区拦截适用于所有类型的网站，包括反向代理网站。每个网站各有自己的设置。

被拦截的访客会看到 **403 Forbidden**。要显示你自己的页面，在网站的 `public` 文件夹中放一个 `403.html` 文件，并在 **网站** → 你的网站 → **Nginx 配置** 中把 Web 服务器的 403 错误页面指向它（Nginx：`error_page 403 /403.html;`；Apache：`ErrorDocument 403 /403.html`，也可以写在 `.htaccess` 中；OpenLiteSpeed：一个 `errorpage 403` 块）。被拦截的访客仍然可以加载 `/403.html` 本身。

### 永远不会被拦截的内容 {#what-is-never-blocked}

- Let's Encrypt 的验证请求（`/.well-known/acme-challenge/`），这样证书可以照常续期。
- Git 部署的 Webhook（`/.panel/deploy/…`）。面板会检查它的签名。
- `/403.html`。
- 私有地址和其他保留地址（`10.0.0.0/8`、`192.168.0.0/16`、回环地址等）。
- 服务器自己发出的请求，例如 WordPress 为执行计划任务而访问自己。

面板本身不是网站：它有自己的端口，永远不会被国家/地区拦截。要限制谁能访问面板，请使用防火墙（见 [安全](./security#firewall-ufw)）。

### 在 Cloudflare 后面 {#behind-cloudflare}

网站的 DNS 记录由 Cloudflare 代理时，所有请求都来自 Cloudflare 的地址。

- **Nginx 和 Apache**：对于来自 Cloudflare 地址的请求，Nefo 查询 Cloudflare 在 `CF-Connecting-IP` 请求头中发送的访客地址。来自其他地方的请求按它们自己的地址查询，所以客户端无法伪造这个请求头。在 Apache 上，经过 Cloudflare 的请求在网站的访问日志和 PHP 中也会显示访客的地址。
- **OpenLiteSpeed**：对于来自 Cloudflare 地址的请求，Nefo 使用 Cloudflare 在 `CF-IPCountry` 请求头中发送的国家。请在 Cloudflare 中开启 **IP Geolocation**（IP 地理位置，在区域的 **Network**（网络）设置中）。不开启时，经过 Cloudflare 的访客不会被拦截；选择 **只允许以下国家/地区访问** 时，他们会被放行。

Nefo 内置一份 Cloudflare 地址列表，每月和国家数据库一起从 Cloudflare 刷新。

### 各 Web 服务器的限制 {#limits-per-web-server}

| Web 服务器 | 工作方式 | 限制 |
|---|---|---|
| Nginx | Nginx 内置的 `geo` 模块，只在有网站使用国家/地区拦截时加载，只包含各网站列出的国家/地区 | 无。内存：一个国家约 2 到 5 MB，大列表（美国、德国和日本加在一起）约 40 MB |
| Apache | Apache 在每个请求中逐一检查所列国家/地区的地址段 | 每个网站最多 50,000 个地址段，所以大国会被拒绝（仅美国就有约 270,000 个）。中国（15,000 个地址段）约占 2.5 MB |
| OpenLiteSpeed | OpenLiteSpeed 自己的 GeoIP 查询，使用 DB-IP 的数据库文件 | 无。不到 1 MB |

一个列表最多可以有 100 个国家/地区。在 Apache 上，标签页会显示网站已用的地址段数量和上限。列表太大时，Apache 会拒绝它，提示中会列出放不下的国家/地区，从最大的开始，并给出它们的地址段数量。去掉这些国家/地区，或者改用能处理所有国家/地区的 Nginx 或 OpenLiteSpeed。

### 国家数据 {#the-country-data}

数据来自 DB-IP 的免费 **IP to Country Lite** 数据库，IP Geolocation by [DB-IP](https://db-ip.com)，采用 Creative Commons Attribution 4.0 许可证。它涵盖 IPv4 和 IPv6。

- 第一次有网站使用国家/地区拦截时，Nefo 通过 HTTPS 下载它。
- 替换旧副本之前，Nefo 会检查文件是否是完整的数据库：格式、大小，以及是否列出了 200 多个国家/地区。
- 每月刷新一次（DB-IP 在每月初发布新版本）。标签页会显示正在使用的版本。
- 没有网站使用国家/地区拦截时，什么都不下载，Web 服务器也不加载任何内容。

数据库保存在 `/var/lib/panel-geoip/`。因为 Web 服务器要读取它，所以所有用户都可以读取：它是公开数据。

## 如何让网站的文件变为只读？ {#lock-a-website-s-files}

**文件锁定** 标签页让网站的代码对它的 PHP、它的应用和它的 SFTP 账户只读。这样，即使某个插件有安全漏洞，也无法再修改网站的文件或添加新文件，你允许的文件夹（例如上传目录和缓存）除外。其他面板把这叫作防篡改。

1. 打开 **网站**，点击该网站，打开 **文件锁定** 标签页。
2. 检查 **可写文件夹** 下的列表。对于 WordPress、Joomla、Laravel、Drupal、Nextcloud、OpenCart、Moodle、Symfony 和 CodeIgniter 网站，面板会填好应用的文件夹；**使用 WordPress 的文件夹**（或该网站所用应用的文件夹）可以恢复这些预设。每行写一个文件夹，相对于网站文件夹，例如 `public/wp-content/uploads`。`*` 代表一个文件夹名：`public/sites/*/files`。`tmp` 和 `logs` 始终可写。
3. 如果想在 PHP 文件仍然发生变化时收到通知，勾选 **PHP 文件变化时告警**（见 [下文](#alerts-when-php-files-change)）。
4. 点击 **锁定文件**。

之后标签页会显示 **自……起已锁定**，中间是锁定的日期。以后要更改可写文件夹或告警，编辑后点击 **保存**：文件会按新列表重新锁定。**解锁文件** 把一切交还给网站用户，每个文件的属主、属组和权限都和锁定前完全一样。

Docker 应用的文件属于其容器，反向代理网站不提供文件，所以这两类网站没有 **文件锁定** 标签页。

预设的文件夹：

| 应用 | 可写文件夹 |
|---|---|
| WordPress | `public/wp-content/uploads`、`public/wp-content/cache`、`public/wp-content/upgrade` |
| Joomla | `public/images`、`public/media`、`public/cache`、`public/administrator/cache`、`public/administrator/logs`、`public/tmp` |
| Laravel | `storage`、`bootstrap/cache` |
| Drupal | `public/sites/*/files` |
| Nextcloud | `data`、`public/config` |
| OpenCart | `storage`、`public/image/cache`、`public/image/catalog` |
| Moodle | `moodledata` |
| Symfony | `var` |
| CodeIgniter | `writable` |

锁定文件时，如果某个可写文件夹还不存在、但它的上一级文件夹存在，面板会以网站用户的身份创建它。标签页会列出找不到的文件夹。

### 文件锁定期间有什么变化 {#what-changes-while-the-files-are-locked}

- 网站的 PHP、它的应用和它的 SFTP 账户不能在可写文件夹之外修改、添加、重命名或删除任何东西，也不能更改那里的权限。上传和缓存照常工作。
- WordPress 无法自我更新，无法在它的后台安装或更新插件和主题，也无法改写 `wp-config.php` 或 `.htaccess`。请改从 **WordPress** 标签页更新（见下文）。
- 网站照常运行：Web 服务器和 PHP 仍然能读取之前能读取的每个文件。只有网站用户能读取的文件，例如 `wp-config.php`（模式 `0600`），Web 服务器仍然读不到。

工作原理：原来属于网站用户的文件和文件夹改为属于 root，其他人都没有写权限。网站用户组可读的文件保留该用户组。只有属主可读的文件改用网站自己的锁定用户组（`panel-lock-<uid>`，唯一的成员是网站用户）。不需要内核模块，也不用 `chattr`。锁定所做的更改记录在 `/var/lib/nefo/filelock/` 中，所以即使发生崩溃，解锁也能准确恢复。

### 锁定网站上的面板操作 {#panel-operations-on-a-locked-website}

| 操作 | 锁定期间 |
|---|---|
| **WordPress** 标签页：**全部更新**、维护模式、调试模式、自动更新 | 面板解锁文件，完成更改后重新锁定 |
| 把预发布副本推送到这个网站 | 推送时解锁，完成后重新锁定 |
| **Git 部署**（手动或由 Webhook 触发） | 部署时解锁，完成后重新锁定 |
| 恢复这个网站（或其中某个文件夹）的备份 | 恢复时解锁，完成后重新锁定 |
| 恶意软件扫描：隔离或恢复文件 | 期间解锁，完成后重新锁定 |
| 安装应用的依赖（Node.js、Python） | 期间解锁，完成后重新锁定 |
| **文件**（文件管理）：编辑、上传、创建、重命名、删除、解压、更改权限 | 在可写文件夹和 `tmp`、`logs` 中允许；在其他地方拒绝，并提示网站的文件已锁定 |
| 向网站安装应用（WordPress 或其他应用） | 拒绝：请先解锁文件 |
| 把网站导入这个（空）网站 | 拒绝：请先解锁文件 |
| 克隆网站、创建它的预发布副本、备份 | 可以：这些操作只读取文件，包括只有网站自己能读取的文件（如 `wp-config.php`）。副本属于它自己的网站，不会被锁定 |

推送、部署或恢复之后，新文件和其他文件一样处于锁定之下；放进可写文件夹的文件仍归网站所有。每个需要解锁文件的操作都会在任务日志中写入“Unlocking the files of example.com for this change”（正在解锁 example.com 的文件）和“Locking the files of example.com again”（重新锁定 example.com 的文件）。如果重新锁定失败，任务会说明，面板每小时重试一次。操作运行期间，标签页会显示文件正为该操作解锁。

::: warning
锁定拦住的是网站自己的用户，而不是 root。以 root 身份运行的任何东西，例如 root 的 SSH 会话或面板的终端，仍然可以修改文件。移除面板之前，请运行 `nefo unlock-files`（见 [卸载](./uninstall#unlock-the-websites-files)），否则被锁定网站的文件仍归 root 所有。
:::

### PHP 文件变化时告警 {#alerts-when-php-files-change}

勾选 **PHP 文件变化时告警** 后，面板在锁定文件时拍摄一份快照（网站每个 PHP 文件的 SHA-256 校验和，包括可写文件夹中的文件），并每天把 PHP 文件与快照对比一次。有文件被修改、添加（`uploads` 中出现 PHP 文件是典型的入侵迹象）或删除时，它会通过 **设置** → **告警** 中设置的渠道发送告警，标签页会列出这些文件。只有差异列表变化时，它才会再次告警。

如果这些更改是你自己做的，点击 **重新拍摄快照**。会先解锁再重新锁定文件的操作（例如 WordPress 更新）会自己拍摄新快照。

每天的检查每个网站最多读取 200,000 个 PHP 文件或 2 GB。达到这个上限而停止时，标签页会说明。
