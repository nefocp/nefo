---
title: 系统
description: 安装操作系统更新，开启自动安全更新，添加交换空间，设置时区、时钟同步和主机名，开启 TCP BBR，以及释放磁盘空间。
---

# 系统

**系统** 页面可以安装操作系统更新（安全更新会单独标出，每隔几小时检查一次）、开启自动安全更新、创建交换文件、设置时区、时钟同步和主机名、开启 TCP BBR，以及释放磁盘空间。这里的每项更改都会记录在活动日志中。

该页面位于左侧菜单的 **服务器** 分组中。它有五个标签页：**更新**、**交换空间**、**时间与名称**、**网络** 和 **清理**。活动日志位于 **账户** → **最近活动**。

耗时较长的更改（例如安装更新或创建交换文件）作为后台任务运行：页面会显示它们的进度，同一时间只运行一个任务。

## 更新 {#updates}

**更新** 标签页列出操作系统待安装的更新：每个软件包、它的已安装版本和新版本，以及新版本的来源（套件，例如 `noble-updates`）。来自安全套件（`noble-security`、`bookworm-security`……）的更新会标为 **安全**，并排在最前面。标为 **新** 的软件包会随更新一起安装，例如新内核。

### 面板如何检查更新 {#how-the-panel-checks-for-updates}

- 面板会自己检查：它每小时查看一次上次检查的时间，如果已超过 6 小时，就运行 `apt-get update`，并询问 apt 升级会安装什么。有其他任务运行时，以及面板启动后的前 30 分钟内，它会跳过检查。
- 打开页面从不运行 apt：页面显示的是上次检查的结果及其时间。
- **立即检查** 会立即以任务的形式检查。
- **暂时保留不更新** 列出 apt 暂时不安装的更新：因为需要其他改动而被它保留的软件包，以及 Ubuntu 的分阶段更新（会在几天内逐步推送到服务器）。

有安全更新等待安装时，仪表盘上的 **需要注意** 和铃铛会显示“有 N 个安全更新可用”。

### 安装更新 {#install-updates}

- **安装所有更新** 运行 `apt-get upgrade --with-new-pkgs`。它会安装更新所需的新软件包（例如新内核），并且从不移除软件包。
- **仅安装安全更新** 只更新新版本来自安全套件的已安装软件包（用 `apt-get install --only-upgrade` 加上它们的名称）。

两者都会先刷新软件包列表，运行时不询问任何问题，并保留你修改过的配置文件。完成后会重新检查列表。

::: warning 服务会短暂重启
更新 Nginx、Apache、OpenLiteSpeed、PHP、MariaDB 或 Redis 会重启该服务，所以网站可能有几秒钟没有响应。更新 Docker 会重启它和它的容器。页面会列出待安装的更新会重启哪些服务，确认对话框中也会列出。请在访问量少的时候安装这类更新。
:::

### 需要重启 {#restart-required}

有些更新（例如新内核或新的 C 库）只有在服务器重启后才算完成。以下情况下，标签页会显示“服务器需要重启以完成更新的安装。”：

- `/var/run/reboot-required` 存在（Ubuntu 会写入它，并在 `/var/run/reboot-required.pkgs` 中列出软件包），或者
- `/boot` 中安装了比正在运行的内核更新的内核（Debian 并不总是写入那个文件）。

**在设置中重启** 会打开 **设置** → **面板** → **电源**，你可以在那里重启服务器。在你重启之前，铃铛也会显示“服务器需要重启”。

### 自动安全更新 {#automatic-security-updates}

**自动安全更新** 开关让服务器每天自己安装安全更新：

- 如果没有安装 `unattended-upgrades`，面板会先以任务的形式安装它。它用来等待服务器关机的辅助程序约占用 20 MB 内存。
- 面板会在 `/etc/apt/apt.conf.d/20auto-upgrades` 中设置 `APT::Periodic::Update-Package-Lists "1";` 和 `APT::Periodic::Unattended-Upgrade "1";`，保留文件中的其他行，并开启 apt 的每日定时器。
- 关闭开关会设置 `APT::Periodic::Unattended-Upgrade "0";`。

安装哪些更新由 unattended-upgrades 自己在 `/etc/apt/apt.conf.d/50unattended-upgrades` 中的设置决定：在 Ubuntu 和 Debian 上只安装安全更新。除非你在那里开启 `Unattended-Upgrade::Automatic-Reboot`，否则它不会重启服务器。如果 `/etc/apt/apt.conf.d` 中的另一个文件覆盖了这个设置，面板会说明，并保持该文件不变。

## 交换空间 {#swap}

交换空间是内核在内存不够时使用的磁盘空间。在 1 或 2 GB 内存的服务器上，1 到 2 GB 的交换空间可以避免繁忙时需要的内存超过服务器所有时，MariaDB 和 PHP 被内核停掉。

**交换空间** 标签页显示内存、已使用的交换空间，以及 `/proc/swaps` 中的每个交换文件或交换分区。

### 创建交换文件 {#create-a-swap-file}

1. 选择 **大小**：512 MB、1 GB、2 GB、4 GB 或 8 GB。只提供能让磁盘至少剩下 1 GB 可用空间的大小。
2. 点击 **创建交换文件**。

面板用 `fallocate` 创建只有 root 能读取的 `/panel.swap`。如果文件系统不支持这种方式，或者 `swapon` 拒绝这样的文件，它会改用 `dd` 写入文件，每几 GB 大约需要一分钟。在 btrfs 上，文件会先被标记为不使用写时复制。然后面板运行 `mkswap` 和 `swapon`，并在 `/etc/fstab` 中添加 `/panel.swap none swap sw 0 0`，这样重启后交换空间会再次启用。

以下情况面板会拒绝创建交换文件：

- 磁盘可用空间不足文件大小加 1 GB；
- 服务器已有一个不是面板创建的交换文件，例如 `/swapfile` 或 `/swap.img`。面板会保持它不变。交换分区不会妨碍创建；
- 在 ZFS 上，交换文件无法工作；
- 在容器（OpenVZ、LXC、Docker）中，内核不允许 `swapon`：“此服务器不允许使用交换空间”。请向服务商要交换空间，或使用完整的虚拟服务器。

如果有任何步骤失败，面板会再次删除这个文件。

### 移除交换文件 {#remove-the-swap-file}

**移除交换文件** 会用 `swapoff` 关闭交换空间，从 `/etc/fstab` 中删除它的行，并删除 `/panel.swap`。交换空间中的内容会先移回内存，所以如果可用内存放不下，面板会拒绝。

### Swappiness {#swappiness}

**Swappiness**（`vm.swappiness`，0 到 100）决定内核把内存移到交换空间的积极程度。默认值是 60；服务器适合用 10，这样只有内存不足时才使用交换空间。面板把这个值写入 `/etc/sysctl.d/90-panel-swap.conf`，并立即应用。

## 时间和主机名 {#time-and-host-name}

**时间与名称** 标签页显示服务器的时间、时区，以及时钟是否已同步。

### 时区 {#time-zone}

从列表（`timedatectl list-timezones`）中选择时区，然后点击 **保存**。面板用 `timedatectl set-timezone` 设置时区，并重启 cron，这样定时任务会按新时区运行。其他已经在运行的服务（例如 MariaDB）会在重启后使用新时区。

### 时钟同步 {#clock-synchronization}

**同步时钟（NTP）** 开启或关闭时间同步（`timedatectl set-ntp`）。请保持开启：证书和双因素验证码都需要正确的时钟。它需要一个时间服务，例如 `systemd-timesyncd`。没有时间服务时，开关会显示为灰色：请用 `apt install systemd-timesyncd` 安装。

### 主机名 {#host-name}

输入新的 **主机名**，然后点击 **保存**。主机名由字母、数字和连字符组成，各部分之间用点分隔，最多 64 个字符，不能以连字符开头或结尾：`web1` 或 `web1.example.com`。

面板会：

1. 用 `hostnamectl set-hostname` 设置它；
2. 让 `/etc/hosts` 中的 `127.0.1.1` 行指向它（完整名称及其第一部分），没有这一行时就添加；
3. 在有 cloud-init 的服务器上，写入带有 `preserve_hostname: true` 的 `/etc/cloud/cloud.cfg.d/99-panel-hostname.cfg`，这样 cloud-init 不会在下次启动时改回服务商给的名称。

主机名不会改变面板的地址或任何证书。从 **设置** → **面板** → **电源** 重启服务器时，你需要输入服务器的名称来确认：更改主机名之后，请输入新的名称。

## 网络：TCP BBR {#network-tcp-bbr}

**网络** 标签页显示正在使用的 TCP 拥塞控制算法（通常是 `cubic`）、默认队列（`net.core.default_qdisc`，通常是 `fq_codel`），以及内核支持的算法。

BBR 是 Google 的一种拥塞控制算法。在距离远或丢包多的连接上（例如访问者在另一个大洲或使用移动网络），它常常能让网站和下载更快。开启 **TCP BBR** 会：

1. 加载 `tcp_bbr` 模块（如果内核把它作为模块提供）；
2. 写入 `/etc/modules-load.d/panel-bbr.conf`（`tcp_bbr`）和 `/etc/sysctl.d/90-panel-bbr.conf`（`net.core.default_qdisc = fq` 和 `net.ipv4.tcp_congestion_control = bbr`）；
3. 运行 `sysctl --system`。

新连接会立即使用 BBR。`fq` 队列在下次重启后才作用于网卡；在此期间，没有它 BBR 也能很好地工作。

正在运行的内核不支持 BBR 时，以及在不能更改内核设置的容器中，面板会拒绝开启，并且不保留任何改动。关闭 BBR 会删除这两个文件，并恢复为 `cubic` 和 `fq_codel`。如果 BBR 是在面板之外设置的，开关会显示为灰色。

## 磁盘清理 {#disk-cleanup}

**清理** 标签页显示每项清理能释放多少空间。**清理** 会请你确认，以任务的形式运行清理，然后显示释放的空间。

| 项目 | 删除什么 |
|---|---|
| **已下载的软件包** | apt 安装后保留在 `/var/cache/apt/archives` 中的软件包文件（`apt-get clean`）。 |
| **系统日志** | 系统日志（journal）中的旧条目，减到 100 MB（先 `journalctl --rotate`，再 `journalctl --vacuum-size=100M`）。 |
| **旧日志** | `/var/log` 中已被 logrotate 轮转过的日志：名称以 `.gz` 或数字结尾，例如 `syslog.1` 或 `auth.log.2.gz`。 |
| **不再需要的软件包** | 已经没有任何东西需要的软件包，例如旧内核（`apt-get autoremove`）。 |
| **Docker** | 没有容器使用的镜像，以及构建缓存（`docker image prune --all`、`docker builder prune --all`）。只在安装了 Docker 时显示。 |

- **旧日志** 会保留 `/var/log/nginx`、`/var/log/apache2` 和 `/var/log/openlitespeed` 中的 Web 服务器日志，因为网站的流量统计要读取它们。它只删除 root 自己文件夹中的文件，从不跟随链接。
- 对于 **不再需要的软件包**，确认对话框会列出这些软件包。如果在你确认时 apt 还要移除别的东西，就什么都不移除：“不再需要的软件包列表已发生变化”。
- **Docker** 从不移除容器（无论是否已停止），也不移除容器保存数据的卷。

## 活动日志 {#activity-log}

以下操作会出现在 **账户** → **最近活动** 中：安装了系统更新、开启或关闭了自动安全更新、创建或移除了交换文件、更改了 swappiness、更改了时区、开启或关闭了时钟同步、更改了主机名、开启或关闭了 TCP BBR，以及清理了磁盘空间。
