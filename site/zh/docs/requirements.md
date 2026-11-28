---
title: 系统要求
description: Nefo 需要的操作系统、CPU 类型、内存、端口和防火墙设置。
---

# 系统要求

Nefo 需要一台 64 位服务器（x86-64 或 ARM64），运行 Ubuntu 22.04、24.04 或 26.04，或 Debian 12 或 13，并且有 root 权限和 systemd。它为 1 GB 内存的 VPS 设计：面板本身约占用 34 MB 内存，这是在测试服务器上更新到 v0.24.0 后测得的。

本页列出 [安装 Nefo](./install) 之前服务器需要满足的条件：操作系统、CPU 类型、内存和磁盘，以及网络端口。

## 操作系统 {#operating-system}

安装程序支持以下系统：

| 发行版 | 版本 |
|---|---|
| Ubuntu | 22.04、24.04、26.04 |
| Debian | 12、13 |

安装程序已在每个系统的全新服务器上测试过，平台为 x86-64。ARM64 版本还没有在真实的 ARM 硬件上测试过。

在其他系统上，安装程序会停止，并提示该系统“is not supported yet”（尚不支持）。你可以设置 `NEFO_FORCE=1` 强行尝试（见 [安装](./install#environment-variables)），但网站管理需要 `apt-get`、`systemctl` 和 `useradd`。没有它们时，**网站** 页面会显示“网站管理需要带 systemd 的 Debian 或 Ubuntu 服务器，因此在这台机器上不可用。”

安装程序还需要：

- root 权限（以 root 登录，或使用 `sudo`）；
- systemd；
- `curl` 和 `sha256sum`；
- `openssl`，用于检查发布版本的签名。

如果没有 `curl`，先用 `apt install curl` 安装它。

## CPU {#cpu}

Nefo 可以运行在 64 位 x86（`x86_64`，也叫 amd64）和 64 位 ARM（`aarch64`，也叫 arm64）上。在其他 CPU 上，安装程序会停止，并提示“unsupported CPU architecture”（不支持的 CPU 架构）。

## 内存和磁盘 {#memory-and-disk}

Nefo 为 1 GB 内存的小型 VPS 设计。面板本身是一个约 25 MB 的程序，约占用 34 MB 内存（在测试服务器上更新到 v0.24.0 后测得）。你只需安装自己需要的组件。**软件** 页面会显示每个组件大约占用多少内存。例如，Web 服务器空闲时约占用 15 MB（Nginx）、26 MB（OpenLiteSpeed）或 40 MB（Apache）。

在容器形式的 VPS（LXC、OpenVZ）上，Nefo 计算的是容器自己的内存上限，而不是宿主机的内存（见 [容器形式的 VPS](./first-steps#a-vps-that-is-a-container)）。

有一个组件需要多得多的内存：可选的 ClamAV 杀毒软件扫描时需要约 1.4 GB 可用内存，所以适合 2 GB 或以上内存的服务器。

在 1 GB 内存的服务器上，请添加 1 到 2 GB 交换空间，以免繁忙时内核停掉 MariaDB 或 PHP。很多 VPS 镜像没有交换空间。**系统** → **交换空间** 一键即可创建交换文件（见 [交换空间](./os#swap)）。在容器（OpenVZ、LXC）中，内核不允许使用交换空间：请询问你的服务商。

需要多少磁盘空间取决于你的网站、数据库和备份。保存在服务器上的备份存放在 `/var/backups/panel`，与网站在同一块磁盘上。仪表盘会显示磁盘用量，磁盘用到 90% 时面板会发出警告。

## 全新的服务器 {#a-fresh-server}

请在全新安装的服务器上安装 Nefo。安装程序不会检查服务器上还装了什么，但面板会自己配置 Web 服务器、PHP、MariaDB、防火墙和 SSH：

- 它只运行一个 Web 服务器（Nginx、Apache 或 OpenLiteSpeed）。服务器上已有另一个 Web 服务器时，它会拒绝安装，因为两者都需要 80 和 443 端口。
- 它会为自己管理的软件写入自己的配置。不要在另一个控制面板旁边安装 Nefo。

安装程序拒绝在同一台服务器上运行第二次：`/etc/nefo/config.json` 存在时，它会停止，并提示“the panel is already installed”（面板已安装）。

## 端口 {#ports}

| 端口 | 使用者 | 何时使用 |
|---|---|---|
| 20000 到 49999 之间的一个随机 TCP 端口 | 面板 | 从安装时起。安装程序会打印出来。 |
| 80 和 443（TCP） | Web 服务器 | 从面板安装 Web 服务器之后 |
| 443（UDP） | OpenLiteSpeed 上的 HTTP/3 | Web 服务器是 OpenLiteSpeed 时 |
| 你的 SSH 端口（默认 22） | SSH | 始终 |
| 3306（TCP） | MariaDB 或 MySQL | 只在你允许远程访问某个数据库时，并且只对你列出的地址开放 |
| 5432（TCP） | PostgreSQL | 只在你允许远程访问某个 PostgreSQL 数据库时，并且只对你列出的地址开放 |

有些服务只监听服务器本身（127.0.0.1），所以不需要开放它们：Git 推送 Webhook 的接收程序（7809 端口，经由 Web 服务器访问）、Docker 容器的端口，以及本地邮件中继。

## 防火墙 {#firewalls}

### 服务器上的防火墙（ufw） {#the-firewall-on-the-server-ufw}

- 如果安装时 ufw 处于启用状态，安装程序会在其中开放面板的端口（如果运行的是 firewalld，则在 firewalld 中开放）。
- 从面板安装 Web 服务器时，如果 ufw 处于启用状态，面板会开放 80 和 443 端口。面板不管理 firewalld：如果服务器使用它，请自己开放这些端口。
- **防火墙与 SSH** 页面管理 ufw。如果没有安装 ufw，页面会显示“这台服务器上未安装 ufw。”用 `apt install ufw` 安装它。

从面板开启防火墙时，它会保持 SSH 和面板端口开放；安装了 Web 服务器时，还会保持 80 和 443 端口开放。见 [安全](./security)。

### 服务商的云防火墙 {#your-provider-s-cloud-firewall}

很多服务商在服务器之外还有一层防火墙，通常叫安全组或云防火墙。安装程序无法更改它。请自己在那里放行：

1. 你的 SSH 端口。
2. 面板的 TCP 端口，安装程序会在最后打印出来。
3. TCP 80 和 443 端口，从你开始托管网站起。Let's Encrypt 通过 80 端口验证你的域名，所以这个端口关闭时无法签发证书。

## 域名 {#a-domain-name}

安装 Nefo 不需要域名：你可以通过服务器的 IP 地址打开面板。每个网站都需要一个域名；你也可以给面板一个它自己的域名和免费证书（见 [入门步骤](./first-steps#give-the-panel-its-own-domain)）。把域名的 A 记录指向服务器的 IP 地址。
