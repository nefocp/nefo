---
title: 更新
description: 从面板或用 nefo update 更新 Nefo，了解发布版本如何验证，以及更新失败时该怎么办。
---

# 更新

要更新 Nefo，在 **设置** → **面板** 上点击 **检查更新**，或以 root 身份运行 `nefo update`。面板会下载最新的发布版本，检查它的 Ed25519 签名和 SHA-256 校验和，一步替换 `/usr/local/bin/nefo` 并重启。网站会继续运行。Nefo 从不自行更新。

本页介绍如何更新 Nefo、每个发布版本在安装前如何检查，以及更新失败时该怎么办。在 1.0 版本之前，只有最新的发布版本会获得安全修复，所以请让 Nefo 保持最新。

## 从面板更新 {#update-from-the-panel}

1. 打开 **设置** → **面板**。当前版本显示在 Nefo 名称旁边。
2. 点击 **检查更新**。如果已经是最新的发布版本，面板会显示“已是最新版本。”
3. 如果有更新的发布版本，面板会显示“版本 v0.2.0 可用。”点击 **安装更新** 并确认。

页面上会显示进度。新版本就位后，面板会重启，大约 20 秒后页面会自动重新加载。重启期间网站会继续运行。

你也可以从仪表盘更新：点击 **检查更新**，然后点击 **更新到 v0.2.0**。

## 从命令行更新 {#update-from-the-command-line}

```sh
nefo update
```

```
Installed: v0.1.0
Latest:    v0.2.0
Release signature verified
Downloading nefo-linux-amd64 v0.2.0
Checked: Nefo v0.2.0 (commit 5e6f7a8)
Installed. The panel restarts now; reload the page in a few seconds.
```

如果已经是最新的发布版本，命令会打印两个版本号，不做任何改动。只想检查，运行 `nefo update --check`。

## 更新时会发生什么 {#what-happens-during-an-update}

1. 面板从项目的 GitHub 发布页面读取最新的版本号。
2. 它下载校验和列表及其签名，并用项目的密钥检查签名，这个密钥内置在面板中。
3. 它下载适合你 CPU 的新程序，并对照列表检查它的 SHA-256 校验和。
4. 它运行一次新程序，确认它能在这台服务器上工作。
5. 它一步替换 `/usr/local/bin/nefo`，所以永远不会出现写了一半的程序。
6. 它重启 `nefo` 服务。如果有任务（例如软件安装）正在运行，面板会先让它完成，最多等 75 秒。

新版本启动时，会把面板的数据库更新到新格式。它还会按自己的模板更新它管理的配置文件（Web 服务器、PHP-FPM 池）。如果 Web 服务器拒绝新文件，面板会把旧文件放回去。

## 发布版本如何验证 {#how-releases-are-verified}

GitHub 上的每个发布版本包含 amd64 和 arm64 的程序、`install.sh`、一个 `VERSION` 文件、包含两个程序和安装程序 SHA-256 校验和的 `checksums.txt`，以及 `checksums.txt.sig`，即 `checksums.txt` 的 Ed25519 签名。

安装程序和更新程序都会拒绝没有用项目密钥签名的 `checksums.txt`，也会拒绝校验和不匹配的下载。更新时的错误信息是：

- “checksums.txt has no valid signature from the project: the update was not installed”（checksums.txt 没有项目的有效签名：未安装更新）；
- “the download does not match checksums.txt: not installed”（下载的文件与 checksums.txt 不符：未安装）。

所以，即使有人能修改 GitHub 上的发布文件，或在传到你服务器的途中修改它们，也无法让更新安装他们自己的程序：更新是用你正在运行的 Nefo 中内置的密钥检查的。首次安装则不同：你下载的安装程序带有它自己的一份密钥。另外，签名密钥作为 GitHub 仓库的机密保存，由 GitHub Actions 为每个发布版本签名，所以接管了仓库的人可以为发布版本签名。见 [签名能证明什么，不能证明什么](../security-policy#what-the-signature-does-and-doesn-t-prove)。

没有项目密钥的开发版本无法检查签名。它只检查校验和，日志中会写“Development build: the release signature is not checked”（开发版本：不检查发布签名）。

## 哪些内容会保留 {#what-is-kept}

更新只替换程序 `/usr/local/bin/nefo`。其他一切都保留：

- 配置文件 `/etc/nefo/config.json`，包括端口和秘密路径；
- 数据文件夹 `/var/lib/nefo`：你的账户和双因素认证、设置、网站、计划、证书和活动日志；
- 你的网站、数据库、文件和备份，以及运行它们的服务。

只有面板会重启。网站、数据库和其他服务会继续运行。

## 更新失败怎么办 {#if-an-update-fails}

**更新在安装新程序之前停止。** 下载失败、签名或校验和不匹配，或新程序无法在这台服务器上运行时，就会这样。此时什么都没有改变：旧版本继续运行。错误会显示在面板的任务日志中，或终端中。稍后再试；如果问题持续，请报告这个错误。

**新版本无法启动。** 检查服务及其日志：

```sh
systemctl status nefo
journalctl -u nefo -n 50 --no-pager
```

请在 [GitHub 仓库](https://github.com/nefocp/nefo/issues) 中报告问题，并附上这些输出。要回到上一个发布版本，请手动安装它的程序。下面的例子在 amd64 服务器上使用 `v0.1.0`：

```sh
cd /tmp
curl -fsSLO https://github.com/nefocp/nefo/releases/download/v0.1.0/nefo-linux-amd64
curl -fsSLO https://github.com/nefocp/nefo/releases/download/v0.1.0/checksums.txt
grep " nefo-linux-amd64$" checksums.txt | sha256sum -c -
install -m 0755 nefo-linux-amd64 /usr/local/bin/nefo
systemctl restart nefo
```

只有 `sha256sum` 打印出 `OK` 时才继续。在 ARM 服务器上，请使用 `nefo-linux-arm64`。

::: warning 回到旧版本
如果新版本已经更改了面板的数据库，旧版本会拒绝启动，并提示“database schema version … is newer than this panel supports”（数据库架构版本……比此面板支持的版本更新）。这时只有更新的版本才能使用这些数据。在重要的更新之前，你可以保留一份数据文件夹的副本：先 `systemctl stop nefo`，再 `cp -a /var/lib/nefo /root/nefo-data-copy`，然后 `systemctl start nefo`。
:::

## 自动更新 {#automatic-updates}

Nefo 不会自行更新，也不会自己检查更新。只有你点击 **检查更新** 或运行 `nefo update` 时，它才会检查。要获知新的发布版本，请关注 [GitHub 仓库](https://github.com/nefocp/nefo/releases) 的发布。[更新日志](../changelog) 列出了每个版本的变化。

这里说的只是面板本身。操作系统的软件包是另一回事：面板每隔几小时检查一次，在 **系统** → **更新** 中显示，并且可以自动安装安全更新。见 [系统](./os#updates)。

## 更新从哪里来 {#where-updates-come-from}

默认情况下，更新来自 `https://github.com/nefocp/nefo/releases/latest/download`。`/etc/nefo/config.json` 中的 `update_url` 字段可以指向另一个发布相同文件的地址。它必须以 `https://` 开头，否则检查更新会失败，并提示“config.json 中的 update_url 必须是 https:// 地址。”除非你确定需要，否则不要设置它。
