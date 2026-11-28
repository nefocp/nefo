---
title: 命令行
description: 每个 nefo 命令及其选项（info、reset-password、update、setup、unlock-files、decrypt-backup）、配置文件，以及如何用 systemctl 和 journalctl 管理 nefo 服务。
---

# 命令行

位于 `/usr/local/bin/nefo` 的 `nefo` 程序负责运行面板，并为服务器的 root 用户提供命令：`nefo info` 显示面板的地址，`nefo reset-password` 设置新密码（还可以关闭 2FA），`nefo update` 安装最新的发布版本，`nefo setup` 安装软件。所有命令都请以 root 身份运行。

本页列出这些命令，然后介绍如何管理 `nefo` 服务，以及配置文件中有什么。

## 命令 {#commands}

| 命令 | 作用 |
|---|---|
| `nefo info` | 显示面板的 URL |
| `nefo reset-password` | 设置新的随机密码，并让所有会话退出登录 |
| `nefo update` | 安装最新的发布版本并重启面板 |
| `nefo setup` | 安装 Web 服务器、PHP 版本、数据库服务器和附加组件 |
| `nefo unlock-files` | 把被锁定网站的文件交还给它们（在移除面板之前） |
| `nefo decrypt-backup` | 解密远程存储加密过的备份副本 |
| `nefo version` | 显示版本 |
| `nefo init` | 创建配置文件和第一个 admin 账户 |
| `nefo serve` | 运行面板（systemd 会替你运行） |
| `nefo help` | 显示命令列表 |

运行 `nefo <command> -h` 可以查看某个命令的选项。选项用一个或两个短横线都可以：`-port` 和 `--port` 是一样的。

除 `version`、`help` 和 `decrypt-backup` 外，每个命令都会读取配置文件 `/etc/nefo/config.json`。要使用另一个文件，加上 `--config /path/to/config.json`，或设置 `NEFO_CONFIG` 环境变量。

## nefo info {#nefo-info}

显示面板的 URL 以及它的文件所在位置。

```sh
nefo info
```

```
URL:    https://panel.example.com:28443/k3x9p2qa7m1z/
URL:    https://203.0.113.10:28443/k3x9p2qa7m1z/
Config: /etc/nefo/config.json
Data:   /var/lib/nefo
```

面板有了自己的域名（**设置** → **证书**）后，才会出现第一行。第二行使用服务器的第一个 IP 地址。如果服务商给服务器分配了一个私有地址和一个单独的公网地址，请传入公网地址：

```sh
nefo info --host 203.0.113.10
```

| 选项 | 作用 |
|---|---|
| `--host` | 在 URL 中显示的主机名或 IP 地址 |
| `--port` | 只打印端口号，供脚本使用 |
| `--config` | 要读取的配置文件 |

## nefo reset-password {#nefo-reset-password}

为账户设置新的随机密码，让它在所有设备上退出登录，并打印密码。忘记密码时使用。

```sh
nefo reset-password
```

```
New password for admin: ••••••••••••••••••••
```

如果装有身份验证器应用的手机也丢了，加上 `--disable-2fa`。它会为该账户关闭双因素认证：

```sh
nefo reset-password --disable-2fa
```

```
New password for admin: ••••••••••••••••••••
Two-factor authentication is off. Turn it on again after signing in.
```

| 选项 | 作用 |
|---|---|
| `--disable-2fa` | 同时关闭双因素认证 |
| `--user` | 账户名（默认 `admin`，即面板唯一的账户） |
| `--config` | 要读取的配置文件 |

面板会继续运行：你不需要重启它。重置会写入活动日志。

## nefo update {#nefo-update}

检查项目的 GitHub 发布页面。如果有更新的版本，它会下载新版本，检查签名和校验和，替换程序并重启面板。

```sh
nefo update
```

只查看是否有可用的更新：

```sh
nefo update --check
```

```
Installed: v0.1.0
Latest:    v0.2.0
```

| 选项 | 作用 |
|---|---|
| `--check` | 只显示已安装的版本和最新版本 |
| `--config` | 要读取的配置文件 |

更新会检查什么、失败时怎么办，见 [更新](./update)。

## nefo setup {#nefo-setup}

像面板的配置页面一样配置服务器（见 [入门步骤](./first-steps#set-up-the-server)）：依次安装 Web 服务器、PHP 版本、数据库服务器和附加组件。已经安装的部分会被跳过，所以失败后可以再次运行。完成后，面板登录后不再打开配置页面。

```sh
nefo setup --web nginx --php 8.3,8.4 --db mariadb --redis
```

```
This server: 1 CPU, 960 MB of RAM (250 MB in use), 19.5 GB of free disk.

What the setup installs, one after the other:
  1. Nginx and PHP 8.3  about 40 MB of RAM, 55 MB of disk
  2. PHP 8.4            about 25 MB of RAM, 50 MB of disk, from packages.sury.org
  3. MariaDB            about 228 MB of RAM, 300 MB of disk
  4. Redis              up to 64 MB of RAM, 10 MB of disk
In all: about 357 MB more RAM and 415 MB of disk.

Install? [y/N]
```

你回答 `y` 后，它会显示每项安装的日志。如果某一项失败，它会在那里停止，说明哪些已经装好，并以错误退出。如果没有给出 `--web`、`--php`、`--db` 或任何附加组件，它会像配置页面一样，按服务器的内存给出推荐的选择。

面板运行时，`nefo setup` 会请面板作为它的一个任务来执行安装：这些安装会等待其他任务完成、显示在面板中，即使你关闭 SSH 会话也会继续。它通过数据文件夹中一个只有 root 能打开的套接字与面板通信。面板没有运行时，`nefo setup` 会自己执行安装。

| 选项 | 作用 |
|---|---|
| `--web` | `nginx`、`apache`、`openlitespeed` 或 `none`。Web 服务器会带上发行版自带的 PHP 版本 |
| `--php` | 用逗号分隔的 PHP 版本，例如 `8.3,8.4`，或 `none` |
| `--db` | `mariadb`、`mysql` 或 `none`；在服务器提供时，`mariadb-11.4`、`mariadb-11.8` 或 `mariadb-12.3` 表示 MariaDB 软件源中的 LTS 系列（见 [MariaDB 版本](./databases#mariadb-versions)）。`--mariadb` 等同于 `--db mariadb` |
| `--redis`、`--node`、`--python`、`--docker` | 同时安装 Redis、Node.js（最新的 LTS 版本）、用于应用网站的 Python，或 Docker |
| `--adminer`、`--waf` | 同时安装 Adminer 或 Web 防火墙。两者都需要 Web 服务器 |
| `--clamav` | 同时安装 ClamAV，仅限 2 GB 或以上内存的服务器 |
| `--postgresql`、`--memcached` | 同时安装 PostgreSQL 或 Memcached（在“软件”页面提供它们的系统上） |
| `--yes`、`-y` | 不询问直接安装。没有连接终端时（例如在脚本中）需要它 |
| `--dry-run` | 只显示会安装什么 |
| `--skip` | 什么都不安装：面板登录后不再打开配置页面 |
| `--wait` | 等待面板启动的时间，例如 `60s`。安装程序会使用它 |
| `--check` | 只检查选项。安装程序在配置任何东西之前使用它 |
| `--config` | 要读取的配置文件 |

安装程序接受同样的选项，可以在同一次运行中配置服务器（见 [安装](./install#set-the-server-up-in-the-same-run)）。

## nefo unlock-files {#nefo-unlock-files}

把文件交还给每个文件被锁定的网站（**网站** → 你的网站 → **文件锁定**，见 [国家/地区拦截与文件锁定](./website-protection#lock-a-website-s-files)），每个文件都准确恢复锁定前的属主、属组和权限，并移除网站的锁定用户组。请在移除 Nefo 之前，以 root 身份、在面板停止的状态下运行它（见 [卸载](./uninstall#unlock-the-websites-files)）。

```sh
systemctl stop nefo
nefo unlock-files
```

```
Unlocked the files of example.com
Unlocked the files of blog.example.com
```

没有网站被锁定时，它会打印 `No website's files are locked.`

| 选项 | 作用 |
|---|---|
| `--config` | 要读取的配置文件 |

## nefo decrypt-backup {#nefo-decrypt-backup}

解密远程存储加密过的备份副本（`.tar.gz.enc` 文件），例如你从 Google Drive 下载的副本。只需要密码短语：它可以在任何服务器上运行，包括没有配置面板的服务器，而且不读取配置文件。见 [在新服务器上恢复](./restore-new-server#decrypt-a-copy-by-hand)。

```sh
nefo decrypt-backup example.com_20261126-030000.tar.gz.enc
```

```
Passphrase:
Decrypted to example.com_20261126-030000.tar.gz
SHA-256: 3f1c…
```

密码短语从不在命令行上给出，因为其他用户可能看到它。在终端中，它会以不显示的方式询问；否则，它是标准输入或 `-passphrase-file` 的第一行。

| 选项 | 作用 |
|---|---|
| `-o` | 输出文件。默认为输入文件名去掉 `.enc`。`-o -` 写到标准输出，例如 `nefo decrypt-backup -o - FILE.enc \| tar xz` |
| `-passphrase-file` | 从这个文件的第一行读取密码短语 |
| `-force` | 如果输出文件已存在，替换它 |

输出先写入一个临时文件，只有整份副本解密成功后才重命名，所以密码短语错误或文件损坏时不会留下任何东西（`-o -` 除外，这时文件的末尾最后才检查）。没有加密的文件会原样复制，并附上一条说明。它打印的 SHA-256 就是面板为该备份记录的校验和。

## nefo version {#nefo-version}

显示版本以及构建它的提交。`nefo --version` 和 `nefo -v` 的作用相同。

```sh
nefo version
```

```
Nefo v0.1.0 (commit 1a2b3c4)
```

## nefo init {#nefo-init}

创建配置文件和第一个 admin 账户。安装程序会替你运行它，所以通常不需要它。它拒绝覆盖已有的配置文件。

它会选择 20000 到 49999 之间的随机端口和随机的秘密路径，开启 HTTPS，并创建带随机密码的账户 `admin`。如果数据文件夹中已有账户，它会保留这些账户，只写入新的配置文件，不打印密码。

| 选项 | 作用 |
|---|---|
| `--config` | 配置文件写到哪里（默认 `/etc/nefo/config.json`） |
| `--data` | 数据文件夹（默认 `/var/lib/nefo`） |
| `--listen` | 要监听的地址，例如 `:28443`，代替随机端口 |
| `--host` | 在打印的 URL 中显示的主机名或 IP 地址 |
| `--dev` | 开发模式：在 `127.0.0.1:8080` 上使用普通 HTTP，路径为 `dev`。切勿在服务器上使用。 |

## nefo serve {#nefo-serve}

运行面板。`nefo` 服务用 `nefo serve --config /etc/nefo/config.json` 启动它，所以你不需要自己运行。第二份副本会失败，因为服务已经占用了端口。

## 管理 nefo 服务 {#manage-the-nefo-service}

| 命令 | 作用 |
|---|---|
| `systemctl status nefo` | 显示面板是否在运行，以及最新的日志行 |
| `systemctl restart nefo` | 重启面板 |
| `systemctl stop nefo` | 停止面板 |
| `systemctl start nefo` | 启动面板 |
| `journalctl -u nefo` | 显示面板的日志 |
| `journalctl -u nefo -f` | 实时跟踪新的日志行 |
| `journalctl -u nefo -n 100 --no-pager` | 显示最后 100 行 |
| `journalctl -u nefo --since "1 hour ago"` | 显示最近一小时的日志 |

重启面板不会停止你的网站、数据库或其他服务。你也可以从 **设置** → **面板** → **电源** → **重启面板** 重启它。

面板停止时，会先让正在运行的任务（例如软件安装）完成，最多等 75 秒。如果面板因错误而停止，systemd 会在 3 秒后再次启动它。

服务文件是 `/etc/systemd/system/nefo.service`：

```ini
[Unit]
Description=Nefo server control panel
Documentation=https://nefo.io
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/nefo serve --config /etc/nefo/config.json
Restart=on-failure
RestartSec=3
UMask=0077
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

面板以 root 身份运行，因为它要管理整台服务器。

## 配置文件 {#the-configuration-file}

`/etc/nefo/config.json` 只有 root 能读取。安装程序会写入它，例如：

```json
{
  "listen": ":28443",
  "entrance": "k3x9p2qa7m1z",
  "data_dir": "/var/lib/nefo",
  "tls": {
    "enabled": true
  }
}
```

| 字段 | 含义 |
|---|---|
| `listen` | 面板监听的地址和端口。`:28443` 表示所有地址上的 28443 端口。 |
| `entrance` | 秘密路径。8 到 32 个小写字母或数字。其他所有路径都返回 `404`。 |
| `data_dir` | 数据文件夹：面板的数据库 `nefo.db`、证书和密钥。 |
| `tls.enabled` | 面板的 HTTPS。请保持开启。 |
| `tls.cert`、`tls.key` | 可选，面板使用的你自己的证书和私钥的路径。它们优先于面板的 Let's Encrypt 证书和自签名证书。 |
| `acme_directory` | 可选，签发证书的证书颁发机构。留空表示 Let's Encrypt。测试时可以设为 `https://acme-staging-v02.api.letsencrypt.org/directory`：测试环境（staging）的证书不受浏览器信任，但失败的尝试不计入 Let's Encrypt 的限制。 |
| `update_url` | 可选，下载更新的地址。留空表示项目的 GitHub 发布页面。它必须以 `https://` 开头。 |
| `dev` | 开发模式。切勿在服务器上设置。 |

编辑文件后，用 `systemctl restart nefo` 重启面板。如果文件有错误，面板不会启动：`journalctl -u nefo` 会显示原因。

::: warning 更改端口
如果更改了 `listen` 中的端口，请先在 ufw（例如 `ufw allow 31234/tcp`）和服务商的云防火墙中开放新端口。否则重启后你将无法访问面板。
:::
