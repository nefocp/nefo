---
title: 安装
description: 用一条命令在 Ubuntu 或 Debian 服务器上安装 Nefo，然后第一次打开面板。
---

# 安装

要安装 Nefo，在全新的 Ubuntu 22.04、24.04 或 26.04，或 Debian 12 或 13 服务器上以 root 身份运行一条命令：`curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | sudo bash`。安装程序会检查发布版本的 Ed25519 签名，在几秒钟内把面板安装为 systemd 服务 `nefo`，并打印出面板的地址和一次性的 admin 密码。

本页介绍如何安装 Nefo、安装程序做了什么，以及如何第一次打开面板。请先查看 [系统要求](./requirements)。

Nefo 可以安装在 x86-64 或 ARM64 上的 Ubuntu 22.04、24.04 和 26.04，以及 Debian 12 和 13 上。安装程序已在每个系统的全新服务器上测试过，平台为 x86-64。ARM64 版本还没有在真实的 ARM 硬件上测试过。

## 运行安装程序 {#run-the-installer}

1. 通过 SSH 连接到服务器。
2. 运行：

   ```sh
   curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | sudo bash
   ```

   如果你以 root 登录，并且没有安装 `sudo`（在 Debian 上很常见），去掉它即可：

   ```sh
   curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | bash
   ```

3. 等待安装程序打印出面板的地址、用户名和密码。通常只需几秒钟。

::: tip 本文档中的命令
本文档中的其他所有命令都在服务器上以 root 身份运行，所以显示时不带 `sudo`。
:::

## 安装程序打印的内容 {#what-the-installer-prints}

输出的结尾类似这样：

```
==> Downloading nefo-linux-amd64 (latest)
==> Verifying the release signature
==> Verifying checksum
==> Creating the configuration and the admin account
==> Installing the systemd service
==> Opening port 28443/tcp in ufw

Nefo is ready.

  URL:      https://203.0.113.10:28443/k3x9p2qa7m1z/
  Username: admin
  Password: ••••••••••••••••••••

Save this password now: it is stored only as a hash and cannot be shown again.
After signing in, change it and turn on two-factor authentication.
The certificate is self-signed, so your browser will warn you the first time.

If your provider has a cloud firewall (security group), allow TCP port 28443 there too.
Manage the service with: systemctl status|restart nefo   Logs: journalctl -u nefo
```

你的端口、路径和密码会不同：每台服务器都是随机选择的。

::: warning 现在就保存密码
密码只显示一次。面板只保存它的哈希值。如果丢失了密码，用 `nefo reset-password` 设置新密码（见 [命令行](./cli#nefo-reset-password)）。
:::

地址使用服务器的第一个 IP 地址。如果服务商给服务器分配了一个私有地址和一个单独的公网地址，请把 URL 中的地址换成公网 IP。

## 安装程序做了什么 {#what-the-installer-does}

1. **检查服务器。** 如果不是以 root 运行，缺少 systemd、`curl` 或 `sha256sum`，操作系统或 CPU 不受支持，或者 Nefo 已经安装（`/etc/nefo/config.json` 存在），它会停止。
2. **下载** 适合你 CPU 的程序（`nefo-linux-amd64` 或 `nefo-linux-arm64`），以及项目 GitHub 发布页面上的校验和列表。
3. **检查校验和列表的签名**，使用内置在安装程序中的项目 Ed25519 密钥。没有有效签名的发布版本会被拒绝：“Do not install this file.”（不要安装此文件。）开发版本的安装程序没有密钥，它会说明不检查签名。
4. **检查下载文件的 SHA-256 校验和。** 不匹配时安装会停止。
5. **安装** 程序为 `/usr/local/bin/nefo`，并创建只有 root 能读取的 `/etc/nefo` 和 `/var/lib/nefo`。
6. **创建配置** `/etc/nefo/config.json`：20000 到 49999 之间的随机端口、随机的 12 个字符的秘密路径，并开启 HTTPS。它还会创建账户 `admin`，带一个随机的 20 个字符的密码。
7. **安装 systemd 服务** `nefo`，启动它，并设置为开机启动。
8. **开放面板的端口**：ufw 处于启用状态时在 ufw 中开放，firewalld 正在运行时在 firewalld 中开放。两者都没有启用时，它不改动任何防火墙。

除非你给出 [配置选项](#set-the-server-up-in-the-same-run)，安装程序不会再安装其他东西：没有 Web 服务器、PHP 或数据库。你第一次登录时，面板会提出帮你配置这些（见 [入门步骤](./first-steps#set-up-the-server)）。

## 第一次打开面板 {#open-the-panel-for-the-first-time}

1. 在浏览器中打开安装程序给出的 URL，完全按打印的内容输入。它以 `https://` 开头，以秘密路径和斜杠结尾。
2. 浏览器会警告连接不是私密连接，或证书不受信任。这是正常的：面板在第一次启动时创建了自己的自签名证书。选择继续访问该网站的选项。在大多数浏览器中，它藏在“Advanced”（高级）之类的按钮后面。
3. 用用户名 `admin` 和安装程序给出的密码登录。

然后按照 [入门步骤](./first-steps) 操作：修改密码，开启双因素认证，并给面板一个它自己的域名和免费证书，这样浏览器的警告就会消失。

### 秘密路径 {#the-secret-path}

面板只在它的秘密路径下响应，例如 `/k3x9p2qa7m1z/`。它端口上的其他所有地址都返回 `404`，包括不带路径的 `https://203.0.113.10:28443/`。尝试常见面板地址的扫描器什么也找不到。

请像密码一样保密这个路径。如果忘了，在服务器上运行 `nefo info`（见 [命令行](./cli#nefo-info)）。

如果你输入的是 `http://` 而不是 `https://`，面板会返回错误，而不是登录页面。

## 在云防火墙中开放端口 {#open-the-port-in-your-cloud-firewall}

如果服务商在服务器之外还有防火墙（安全组或云防火墙），请在那里放行面板的 TCP 端口。安装程序无法替你完成。以后托管网站时，还要放行 TCP 80 和 443 端口。见 [系统要求](./requirements#firewalls)。

## 环境变量 {#environment-variables}

你可以用以下变量改变安装程序的行为：

| 变量 | 默认值 | 作用 |
|---|---|---|
| `NEFO_VERSION` | `latest` | 安装指定的发布版本（例如 `v0.1.0`），而不是最新版本。 |
| `NEFO_REPO` | `nefocp/nefo` | 从另一个 GitHub 仓库（`owner/name`）下载，例如一个分支（fork）。 |
| `NEFO_FORCE` | 未设置 | 设为 `1` 可以尝试不受支持的操作系统。CPU 检查仍然有效。 |
| `NEFO_SETUP` | 未设置 | `nefo setup` 的选项，用于在同一次运行中配置服务器。见 [下文](#set-the-server-up-in-the-same-run)。 |

把变量放在 `bash` 前面：

```sh
curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | NEFO_VERSION=v0.1.0 bash
```

使用 `sudo` 时，写成 `| sudo NEFO_VERSION=v0.1.0 bash`。

`NEFO_REPO` 只改变安装程序从哪里下载。以后的更新来自程序中内置的仓库，或配置文件中的 `update_url`（见 [更新](./update#where-updates-come-from)）。

## 在同一次运行中配置服务器 {#set-the-server-up-in-the-same-run}

安装程序也可以像面板的配置页面那样配置服务器：Web 服务器、PHP 版本、数据库服务器和附加组件。在 `bash -s --` 之后给出 [`nefo setup`](./cli#nefo-setup) 的选项：

```sh
curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | sudo bash -s -- --web nginx --php 8.4 --db mariadb --redis
```

或者把它们放进 `NEFO_SETUP` 变量：

```sh
curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | sudo NEFO_SETUP="--web nginx --php 8.4 --db mariadb" bash
```

使用配置选项时的变化：

- 安装程序在下载之后、配置任何东西之前立即检查选项。错误的选项（例如 `--web caddy`）会让它停止，你可以重新运行。选项只能包含字母、数字和 `. , = _ -`。
- 面板启动后，安装程序会用你的选项加 `--yes` 运行 `nefo setup`，并显示它的日志。这需要几分钟。面板把这些安装作为它的一个任务来执行，所以即使 SSH 会话断开，安装也会继续：这时请在面板中查看进度。
- 地址、用户名和密码在配置完成后、最后才打印出来。
- 登录后，面板不再打开它的配置页面。

如果配置中途停止，例如因为某个下载失败，面板仍然保持安装状态。安装程序仍会打印地址和密码，然后说明配置已停止，并以错误退出。解决问题后，用相同的选项运行 `nefo setup`：已经安装的部分会被跳过。

## 重新安装 {#install-again}

`/etc/nefo/config.json` 存在时，安装程序会拒绝运行，所以它不会覆盖正在工作的面板。要从头开始，请先移除 Nefo（见 [卸载](./uninstall)），然后重新运行安装程序。
