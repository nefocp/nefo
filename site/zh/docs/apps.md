---
title: 应用商店
description: 把 PHP 应用安装到网站中，在 Docker 应用前面配上它自己的网站，安装数据库服务，以及管理、更新、备份和移除它们。
---

# 应用商店

Nefo 的应用商店可以一键安装 42 个应用：WordPress、WooCommerce、Joomla、Nextcloud 和 Moodle 等 PHP 应用装进一个网站，Ghost、n8n、Gitea、Grafana 和 Jellyfin 等 Docker 应用运行在它们自己的网站后面，PostgreSQL、MongoDB 和 Ollama 则作为服务运行在 127.0.0.1 上。它会先检查服务器的内存和磁盘。

Nefo 会创建每个应用需要的东西，例如数据库、网站、密码和容器。本页介绍应用的类型、目录、安装，以及如何管理你安装的应用。

在左侧菜单中打开 **应用商店**。**目录** 标签页列出所有应用，**已安装** 标签页列出这台服务器上的应用。在 **网站** 页面上，**从应用商店安装** 也会带你来到这里。

## 三种应用 {#three-kinds-of-apps}

| 类型 | 安装到哪里 | 会得到什么 |
|---|---|---|
| **PHP 应用** | 你创建的一个空 PHP 网站中 | 它自己的数据库、网站中的文件、正确的重写规则。它以网站的 Linux 用户身份运行。 |
| **Docker 应用** | Nefo 为它创建的一个新网站中 | 只监听 `127.0.0.1` 的容器，前面是该网站。 |
| **服务** | 没有网站 | 只监听 `127.0.0.1` 的容器，供你自己的网站和应用使用。每台服务器每种服务一个。 |

PHP 应用和 Docker 应用需要先安装 Web 服务器：见 [网站](./websites)。Docker 应用和服务还需要 Docker。请在 **软件** 页面安装；应用商店中的 **安装 Docker** 会打开那个页面。见 [Docker](./docker)。

## 可以安装哪些应用？ {#the-catalog}

在目录版本 2026112701 中，应用商店提供 42 个应用。它们按类别分组：**全部** 显示每个类别及其应用，类别按钮只显示该类别的应用。搜索会查找名称、描述和类别名称，并忽略越南语的声调符号（输入“giam sat”可以找到“Giám sát & thống kê”类别的应用）。

最低内存是指服务器的总内存低于这个值时，Nefo 会拒绝安装（交换空间不计入）。

| 应用 | 类型 | 类别 | 是什么 | 最低内存 |
|---|---|---|---|---|
| WordPress | PHP 应用 | 网站与内容 | 网站和博客搭建工具 | 450 MB |
| Joomla | PHP 应用 | 网站与内容 | 用于网站和门户的 CMS | 450 MB |
| Drupal | PHP 应用 | 网站与内容 | 用于结构化内容的 CMS，用 Composer 构建 | 900 MB |
| Ghost | Docker 应用 | 网站与内容 | 博客和电子报 | 900 MB |
| Directus | Docker 应用 | 网站与内容 | 无头 CMS | 900 MB |
| Wiki.js | Docker 应用 | 网站与内容 | Wiki | 900 MB |
| WooCommerce | PHP 应用 | 商店与商务 | 带 WooCommerce 的 WordPress：网上商店 | 900 MB |
| OpenCart | PHP 应用 | 商店与商务 | 购物车 | 900 MB |
| Odoo | Docker 应用 | 商店与商务 | ERP（社区版） | 1800 MB |
| Stirling-PDF | Docker 应用 | 效率 | PDF 工具 | 1800 MB |
| Memos | Docker 应用 | 效率 | 时间线上的 Markdown 速记 | 230 MB |
| linkding | Docker 应用 | 效率 | 书签管理器 | 450 MB |
| Vikunja | Docker 应用 | 效率 | 待办清单、看板和项目 | 450 MB |
| Actual Budget | Docker 应用 | 效率 | 个人预算 | 450 MB |
| Kanboard | Docker 应用 | 效率 | 项目看板 | 230 MB |
| FreshRSS | Docker 应用 | 效率 | 新闻和博客阅读器（RSS） | 450 MB |
| Mealie | Docker 应用 | 效率 | 食谱、膳食计划和购物清单 | 900 MB |
| Nextcloud | PHP 应用 | 文件与存储 | 存放文件、日历和联系人的私有云 | 900 MB |
| RustFS | Docker 应用 | 文件与存储 | S3 兼容的对象存储 | 900 MB |
| FileBrowser Quantum | Docker 应用 | 文件与存储 | 浏览器中的文件管理 | 450 MB |
| Navidrome | Docker 应用 | 媒体 | 从你自己的收藏中串流音乐 | 450 MB |
| Jellyfin | Docker 应用 | 媒体 | 来自你服务器的电影、剧集和音乐 | 1800 MB |
| ntfy | Docker 应用 | 通讯 | 通过 HTTP 推送通知 | 230 MB |
| Gotify | Docker 应用 | 通讯 | 带 Android 应用的通知服务器 | 230 MB |
| listmonk | Docker 应用 | 通讯 | 电子报和邮件列表 | 450 MB |
| Uptime Kuma | Docker 应用 | 监控与分析 | 监视你的网站和服务 | 450 MB |
| Umami | Docker 应用 | 监控与分析 | 注重隐私的网站分析 | 900 MB |
| Grafana | Docker 应用 | 监控与分析 | 仪表盘和告警 | 900 MB |
| changedetection.io | Docker 应用 | 监控与分析 | 监视网页的变化 | 900 MB |
| Healthchecks | Docker 应用 | 监控与分析 | 定时任务停止运行时告警 | 450 MB |
| Vaultwarden | Docker 应用 | 安全与网络 | 供 Bitwarden 应用使用的密码管理服务器 | 450 MB |
| AdGuard Home | Docker 应用 | 安全与网络 | 通过你自己的 DNS-over-HTTPS 地址拦截广告 | 450 MB |
| Open WebUI | Docker 应用 | AI | 用于 Ollama 或 OpenAI 兼容 API 的聊天界面 | 1800 MB |
| Ollama | 服务 | AI | 在 CPU 上运行开放的语言模型 | 3584 MB |
| Laravel | PHP 应用 | 开发工具与自动化 | 一个带数据库的新 Laravel 项目 | 450 MB |
| code-server | Docker 应用 | 开发工具与自动化 | 浏览器中的 VS Code | 900 MB |
| IT-Tools | Docker 应用 | 开发工具与自动化 | 浏览器中的开发者工具 | 230 MB |
| Gitea | Docker 应用 | 开发工具与自动化 | 你自己的 Git 服务器 | 450 MB |
| n8n | Docker 应用 | 开发工具与自动化 | 工作流自动化 | 900 MB |
| PostgreSQL | 服务 | 数据库 | PostgreSQL 数据库服务器 | 900 MB |
| MongoDB | 服务 | 数据库 | MongoDB 文档数据库 | 900 MB |
| Moodle | PHP 应用 | 教育 | 学习平台 | 900 MB |

目录内置在面板中。Nefo 还会每天检查一次随发布版本一起发布的更新目录，并且只在其签名有效时使用它。**更新目录** 会立即检查。页头显示目录版本，以及它是“内置于面板”还是“已下载，签名已验证”。

下载的目录中可能有一些应用需要更新版本的 Nefo，因为它们的配方用到了这个版本没有的东西。Nefo 会略过这些应用，保留目录的其余部分。如果它之前已经知道某个应用，会保留它知道的版本，这样已安装的副本可以继续工作。这时页头会显示类似“下载的目录中有 2 个应用需要更新版本的面板”的提示。更新 Nefo 才能获得它们：见 [更新](./update)。更新的目录新增的应用，会以目录中所有语言显示它们的名称和描述。

每张卡片显示应用的版本、类型、内存和磁盘需求。它的 **安装** 按钮会打开描述、许可证、需求和说明，以及安装表单。服务器无法运行的应用，卡片会说明原因，并显示 **详情**。已安装的服务显示 **管理**。“源码可用”徽章表示该许可证未经 OSI 认可。

## 安装前的检查 {#the-checks-before-an-install}

打开一个应用时，Nefo 会把它的需求与服务器进行比较。以下情况它会拒绝安装：

- 服务器的总内存低于应用的最低要求（在容器形式的 VPS 上，以它的内存上限为准）；
- 可用磁盘空间少于应用的镜像和数据再加 1 GB；
- 处理器的架构或特性不适合该应用。

服务器内存少于推荐值、当前可用内存少于应用可能使用的内存，或者所有已安装应用的内存上限加起来超过服务器内存时，它会发出警告，但仍允许你安装。

## 安装 PHP 应用 {#install-a-php-app}

1. 创建一个空的 PHP 网站，使用该应用支持的 PHP 版本：见 [网站](./websites)。最低 PHP 版本为：WordPress 7.4，Nextcloud 和 Laravel 8.2，WooCommerce、Joomla、Drupal 和 Moodle 8.3，OpenCart 8.0。OpenCart 最高支持 PHP 8.3，Moodle 最高支持 8.4。
2. 确认 MariaDB（或 MySQL）已安装并正在运行：见 [数据库](./databases)。
3. 在应用商店中，点击该应用的 **安装** 按钮。
4. 选择 **网站**。列表只显示 PHP 版本合适的空 PHP 网站。如果没有，使用 **添加 PHP 网站**。
5. 对于需要这些信息的应用，填写 **网站名称**、**管理员**（避免用 `admin`：这是攻击者最先尝试的名字）、**管理员邮箱** 和 **管理员密码**。密码留空可以得到一个高强度的随机密码；你输入的密码需要 12 到 128 个字符。WooCommerce 还会询问商店所在的国家和货币。
6. 点击 **安装**。安装在后台运行，并显示在任务列表中。

完成后，Nefo 只显示一次 **登录地址**、**管理员** 和 **管理员密码**。请现在就保存。

网站的 `public` 文件夹必须为空；面板的欢迎页面没有关系。网站的文件不能被 [锁定](./website-protection#lock-a-website-s-files)。Drupal 和 Laravel 用 Composer 构建，Moodle 也需要整个网站文件夹：它们的网站必须是新建的空网站，网站文件夹会成为项目，`public` 作为它的 Web 文件夹。

PHP 应用会得到：

- 一个新的数据库和数据库用户，关联到该网站；
- 应用的文件，下载后会对照目录中的校验和进行检查；
- 应用的重写规则，以及应用需要时更高的 PHP 限制；
- 对于 Moodle（每分钟）和 WooCommerce（每五分钟），一个以网站用户身份运行的定时任务。

如果安装失败，Nefo 会再次清空网站，并删除新建的数据库。WordPress 网站也会出现在 [WordPress 工具箱](./wordpress) 中。

## 安装 Docker 应用 {#install-a-docker-app}

1. 在你的 DNS 中把应用的域名指向这台服务器的 IP 地址，例如把 `app.example.com` 指向 `203.0.113.10`。
2. 在应用商店中，点击该应用的 **安装** 按钮。
3. 输入 **新网站的域名**，如果需要，再输入 **其他域名**。
4. 有些应用还会要求 **管理员邮箱**。请使用真实的域名，而不是 `.test` 或 `.local`。
5. 点击 **安装**。
6. 安装完成后，在新网站的 **SSL / HTTPS** 标签页上开启 HTTPS：见 [SSL 证书](./ssl)。应用会以它的 `https://` 地址重启。有些应用需要 HTTPS 才能完整工作，安装表单会说明。

如何获得第一个管理员账户取决于应用，安装表单会告诉你：

| 方式 | 应用 | 你要做什么 |
|---|---|---|
| 由面板创建 | RustFS、code-server、AdGuard Home、Odoo、Umami、Grafana、ntfy、Gotify、Memos、linkding、Navidrome、Vikunja、Actual Budget、listmonk、Kanboard、FreshRSS、Jellyfin、Mealie | 在 **已安装** 标签页上打开 **凭据**。用户是 `admin`（Mealie 为 `changeme@example.com`；Actual Budget 只有一个服务器密码）。 |
| 你的邮箱和一个随机密码 | Open WebUI、Directus、Wiki.js、Healthchecks | 打开 **凭据**。 |
| 第一个注册的账户 | Ghost、Uptime Kuma、n8n、Gitea | 安装后立即注册：第一个账户会成为管理员。 |
| 邀请 | Vaultwarden | 在应用的页面上用面板发出邀请的邮箱地址注册。 |
| 一次性密码 | FileBrowser Quantum | 打开 **凭据**；第一次登录时应用会要求设置新密码。 |
| 网站密码 | Stirling-PDF、changedetection.io | 网站会要求输入用户 `admin` 和 **凭据** 中的密码。 |
| 不需要登录 | IT-Tools | 什么都不用做。 |

关于其中一些应用的说明：

- **Memos** 和 **Vikunja**：面板创建账户后会关闭注册。你可以在应用的设置中重新开放注册。
- **ntfy**：不登录的话，任何人都不能读取或发送消息。`admin` 账户是应用设置的一部分，所以它的密码就是 **凭据** 下的那个。
- **Mealie**：用 `changeme@example.com` 和 **凭据** 下的密码登录（面板已经替换了默认密码），然后在你的个人资料中设置你自己的邮箱地址。
- **Actual Budget** 只能通过 HTTPS 打开：请先为它的网站获取证书。
- **Healthchecks** 和 **listmonk** 只通过你在应用中设置的 SMTP 服务发送邮件；这台服务器自己不发送邮件。Healthchecks 发到 ntfy、Telegram、Slack 或 Webhook 的告警不需要它也能工作。
- **Navidrome** 和 **Jellyfin**：见 [音乐和视频文件夹](#music-and-video-folders)。

Docker 应用会得到：

- 一个转发到该应用的新网站。在安装完成、应用的默认登录信息被锁定之前，这个网站保持关闭（503）；
- 以 Docker Compose 项目运行的容器，带有内存上限，日志在 10 MB 时轮转；
- 它在 `/var/lib/panel-apps/<name>` 中的文件，只有 root 能读取：Compose 文件、生成的密码和容器的数据；
- 只在 `127.0.0.1` 上发布的端口，永远不在互联网上；
- 固定到确切版本的镜像。

### 音乐和视频文件夹 {#music-and-video-folders}

Navidrome 和 Jellyfin 播放的是它们网站中某个文件夹里的文件，这样你可以用 [文件管理](./files) 或 SFTP 添加文件：

| 应用 | 文件夹 | 应用看到的路径 |
|---|---|---|
| Navidrome | `/var/www/<domain>/music` | `/music` |
| Jellyfin | `/var/www/<domain>/media` | `/media` |

Nefo 在安装时创建这个文件夹，属于网站的 Linux 用户，并以只读方式交给应用：应用可以播放文件，但不能修改它们。Navidrome 在启动时扫描这个文件夹，之后每小时扫描一次。在 Jellyfin 中，添加一个媒体库，并选择 `/media` 下的一个文件夹。

这些文件是网站的一部分，所以网站的备份包含它们，删除网站也会删除它们。在小型服务器上实时转换视频会占用大量 CPU：最好使用你的设备能直接播放的文件。

## 安装服务 {#install-a-service}

PostgreSQL、MongoDB 和 Ollama 没有网站。点击 **安装**：没有需要填写的内容。Nefo 会在 `127.0.0.1` 上选一个空闲端口并创建密码。这台服务器上的应用可以访问该服务；互联网无法访问。每种服务每台服务器只能安装一次。

在 **已安装** 标签页上打开 **凭据**，可以查看地址、用户、密码和 **连接字符串**。

## Docker 应用的安全检查 {#the-safety-checks-on-docker-apps}

目录中 Docker 应用的配方是数据，从来不是脚本。Nefo 把它转换成 Compose 文件，并用与你自己的 Compose 项目相同的规则检查这个文件，没有例外：

- 没有特权容器，没有额外添加的权限，没有宿主机设备；
- 不共享服务器的网络、进程或其他命名空间；
- 没有削弱安全性的选项，没有内核设置；
- 端口只在 `127.0.0.1` 上发布；
- 不挂载系统文件夹或 Docker 套接字；
- 网站中的文件夹（Navidrome 的音乐、Jellyfin 的媒体）只挂载到以网站自己的 Linux 用户身份运行的容器中，并且是只读的；
- 需要密码的配置步骤会通过请求体或程序的标准输入发送密码，从不放在命令行上。

这些规则见 [Docker](./docker)。每次安装都会生成密码，初始密码会在第一次启动后从容器设置中删除。下载的目录只在签名有效时才会使用。

## 管理已安装的应用 {#manage-installed-apps}

**已安装** 标签页列出每个已安装的应用。可以用 **全部**、**带网站** 或 **服务** 筛选，也可以搜索。列有 **应用**、**版本**、**状态** 和 **内存**（容器当前占用的内存）。

状态是 **运行中**、**已停止**、**部分运行**、**无容器**、**正在安装** 或 **安装失败** 之一。PHP 应用显示 **在其网站中** 或 **网站已暂停**。

每行的菜单有以下操作：

| 操作 | 适用于 | 作用 |
|---|---|---|
| **打开** | 有网站的应用 | 打开应用的网站。 |
| **凭据** | Docker 应用和服务 | 显示登录信息。每次打开都会写入活动日志。 |
| **日志** | Docker 应用和服务 | 容器日志的最新几行。 |
| **启动**、**停止**、**重启** | Docker 应用和服务 | 启动、停止或重启容器。 |
| **更新** | 目录中有更新版本的 Docker 应用和服务 | 见下文。 |
| **立即备份** | Docker 应用和服务 | 备份应用。副本在 **备份** 页面上：见 [备份](./backups)。 |
| **网站设置** | 有网站的应用 | 打开网站的设置。 |
| **卸载** | 服务 | 移除服务：见下文。 |
| **连同网站一起删除** | 有网站的应用 | 打开网站，你在那里删除它。 |

### 更新应用 {#update-an-app}

目录中有 Docker 应用或服务的更新版本时，**版本** 列会显示它，例如“2.6.0 可用”。点击 **更新** 并确认。Nefo 会：

1. 先备份应用。如果备份失败，什么都不会改变；
2. 在停止任何东西之前，下载新的镜像；
3. 用新镜像重启应用，并检查它是否响应。如果没有响应，就重新运行上一个版本。

主版本变化时，有些应用会拒绝一键更新：“此更新（…）会更改主版本，需要手动操作。请查看该应用的升级指南。”这适用于 PostgreSQL、Wiki.js、Odoo、Grafana、Gotify、Vikunja、Healthchecks、listmonk、Jellyfin 和 Mealie，以及 MongoDB 的次版本变化。

PHP 应用不在这里更新：“请在应用自己的管理页面中更新。”WordPress 请使用 [WordPress 工具箱](./wordpress)。

在目录出现之前安装的应用会显示 **旧版安装**。**转换并更新** 会在备份之后，把它的数据移到应用商店的文件夹中，用 Docker Compose 运行它，并更新它。

## 卸载应用 {#uninstall-an-app}

- **服务：** 点击 **卸载**，输入它的名称并确认。它的容器、数据和密码会被删除。它的备份会保留。
- **有网站的应用：** 删除网站：见 [删除网站](./websites#delete-a-website)。对于 Docker 应用，这会移除它的容器、它在 `/var/lib/panel-apps` 中的文件夹，以及没有其他应用使用的镜像。删除对话框中默认勾选的 **先备份网站** 也会备份应用的数据，作为一份 **应用** 类型的备份。对于 PHP 应用，勾选 **同时删除它的数据库** 也会删除它的数据库。

::: danger
卸载会永久删除应用的数据。如果以后可能还需要，请先用 **立即备份** 备份它。
:::

## 制作应用的第二份副本 {#make-a-second-copy-of-an-app}

要连同数据制作一个 Docker 应用的第二份副本，请克隆它的网站：**网站** → 你的网站 → **概览** → **克隆…**。见 [克隆网站](./websites#docker-apps)。n8n、Odoo 以及在应用商店出现之前安装的应用无法克隆。PHP 应用像任何 PHP 网站一样，连同网站和数据库一起克隆。
