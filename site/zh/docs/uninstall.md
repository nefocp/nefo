---
title: 卸载
description: 手动从服务器上移除 Nefo，并事先决定它配置的网站和软件如何处理。
---

# 卸载

Nefo 没有卸载命令：你需要以 root 身份手动移除它，只要几步。先用 `nefo unlock-files` 解锁所有被锁定的网站，停止并移除 `nefo` 服务和 `/usr/local/bin/nefo`，关闭面板的端口，然后删除 `/etc/nefo` 和 `/var/lib/nefo`。除非你事先移除，否则网站、数据库和软件会继续运行。

本页逐步说明每一步。它还会说明哪些东西会留在服务器上，因为移除面板并不会移除你用它配置的网站和软件。

## 移除 Nefo 后哪些东西会保留 {#what-stays-when-you-remove-nefo}

本页的步骤移除的是面板本身：它的程序、服务、配置和数据。你通过面板配置的一切都会保留并继续运行：

- Web 服务器、PHP、MariaDB、Redis、Docker，以及 **软件** 页面中的其他组件；
- `/var/www` 中的网站、它们的 Linux 用户和数据库；
- 定时任务、后台程序、应用网站和 Docker 容器；
- 保存在服务器 `/var/backups/panel` 中的备份。

没有面板，就没有程序为你的网站续期证书，也没有程序运行计划备份。网站的证书保存在 Nefo 的数据文件夹 `/var/lib/nefo/certs` 中。如果网站还在使用 HTTPS 时删除了这个文件夹，Web 服务器就无法加载它们的证书。

所以请先决定：

- **要移除所有东西**，先在面板中移除网站和软件（第 2 步），再移除 Nefo。
- **要让网站在没有面板的情况下继续运行**，跳过第 2 步，删除数据时保留 `/var/lib/nefo/certs`（第 7 步）。Let's Encrypt 证书最长有效 90 天，所以请在它们过期前另外安排续期的方法。

## 1. 保留你需要的副本 {#_1-keep-copies-of-what-you-need}

1. 在 **备份** 页面，用 **立即备份** 为每个要保留的网站和数据库做一份新备份。
2. 把备份复制到服务器之外。它们在 `/var/backups/panel` 中；如果设置了远程存储，远程存储中也有。

## 2. 在面板中移除网站和软件 {#_2-remove-websites-and-software-in-the-panel}

请在面板仍在运行时完成。某个组件仍被使用时，面板会拒绝卸载它，所以请按以下顺序操作：

1. **网站：** 打开每个网站，在它的 **概览** 标签页上点击 **删除**。如果不需要它的数据库，勾选 **同时删除它的数据库**。如果已经保留了备份（第 1 步），取消勾选 **先备份网站**。这会删除网站的文件、它的 Linux 用户和它的证书。
2. **数据库：** 删除剩下的数据库。
3. **Docker：** 删除你创建的容器和 Compose 项目。
4. **定时任务** 和 **后台程序：** 删除你创建的那些。
5. **软件：** 在每个不再需要的组件上点击 **卸载**，例如 Docker、MariaDB、Redis 和 Web 服务器。对于 MariaDB 和 Docker，数据文件夹（MariaDB 是 `/var/lib/mysql`，Docker 是 `/var/lib/docker`）会保留，除非你勾选复选框把它也删除。

最后一个 PHP 版本会保留，因为 Web 服务器的配置需要它。如果你不需要它，在移除 Nefo 之后用 apt 删除它。

## 3. 记下面板的端口 {#_3-note-the-panel-s-port}

稍后在防火墙中关闭端口时需要它：

```sh
nefo info --port
```

## 4. 解锁网站的文件 {#unlock-the-websites-files}

如果你锁定了某些网站的文件（**网站** → 你的网站 → **文件锁定**，见 [国家/地区拦截与文件锁定](./website-protection#lock-a-website-s-files)），锁定期间它们的文件属于 root。在移除面板之前把文件交还给网站，否则它们会一直属于 root：

```sh
systemctl stop nefo
nefo unlock-files
```

命令会列出它解锁的每个网站：

```
Unlocked the files of example.com
Unlocked the files of blog.example.com
```

每个文件和文件夹都会准确恢复锁定时的属主、属组和权限。锁定期间 root 放进网站的文件仍属于 root。该命令还会移除网站的锁定用户组（`panel-lock-<uid>`）。它读取 `/var/lib/nefo/filelock` 中的记录，所以请在删除面板数据（第 7 步）之前运行它。再次运行不会做任何事：它会打印 `No website's files are locked.`

## 5. 停止并移除服务和程序 {#_5-stop-and-remove-the-service-and-the-program}

```sh
systemctl disable --now nefo
rm /etc/systemd/system/nefo.service
systemctl daemon-reload
rm /usr/local/bin/nefo
```

## 6. 关闭面板的端口 {#_6-close-the-panel-s-port}

如果 ufw 已开启，删除面板端口的规则。安装程序添加这些规则时带有注释 `Nefo`。

1. 列出带编号的规则：

   ```sh
   ufw status numbered
   ```

2. 按编号删除面板端口的规则，例如：

   ```sh
   ufw delete 3
   ```

3. ufw 通常还有一条 IPv6 的规则，标有 `(v6)`。每次删除后编号都会变化，所以请再列一次规则，把这条也删除。

如果你用的是 firewalld，以 28443 端口为例：

```sh
firewall-cmd --permanent --remove-port=28443/tcp
firewall-cmd --reload
```

同时也要从服务商的云防火墙中移除这个端口。

## 7. 删除配置和数据 {#_7-delete-the-configuration-and-the-data}

::: danger 此操作无法撤销
这会删除面板的账户、设置、网站列表、备份计划、活动日志、面板自己的证书和你网站的证书。请确认已经保留了你需要的东西（第 1 步）。
:::

要删除所有东西：

```sh
rm -rf /etc/nefo /var/lib/nefo
```

如果网站要保留（见上文），保留它们的证书，删除其余内容：

```sh
rm -rf /etc/nefo
find /var/lib/nefo -mindepth 1 -maxdepth 1 ! -name certs -exec rm -rf {} +
```

## 8. 可选：移除 Nefo 在其他地方留下的东西 {#_8-optional-remove-what-nefo-left-elsewhere}

在你使用期间，Nefo 还在它自己的文件夹之外创建了一些文件。只移除你不再需要的：

| 路径 | 是什么 |
|---|---|
| `/var/backups/panel` | 保存在这台服务器上的备份 |
| `/etc/cron.d/panel` 和 `/var/log/panel-cron` | 在面板中创建的定时任务及其日志。删除这个文件之前，任务会一直运行。 |
| `/var/lib/panel-trash` | 在文件管理中删除的文件，保留 30 天 |
| `/var/lib/panel-quarantine` | 被恶意软件扫描器移开的文件 |
| `/var/lib/panel-geoip` | 国家/地区拦截使用的国家数据库 |
| `/var/backups/nefo-postgresql` | PostgreSQL 升级前生成的转储 |
| `/var/lib/panel-acme` | 响应 Let's Encrypt 验证的文件夹。确认没有 Web 服务器再使用它时才删除。 |
| `/etc/ssh/sshd_config.d/99-panel-sftp.conf` | 网站账户和 SFTP 的 SSH 规则 |
| `/etc/fail2ban/jail.d/panel.conf` 和 `/etc/fail2ban/filter.d/panel.conf` | SSH 和面板的 fail2ban 规则（如果你从面板安装了 fail2ban） |
| `/etc/ssh/sshd_config.d/00-panel.conf` | 你在面板中保存的 SSH 设置 |

::: warning SSH 设置
如果你在面板中更改了 SSH 端口或关闭了密码登录，这些设置保存在 `/etc/ssh/sshd_config.d/00-panel.conf` 中。删除这个文件后，SSH 一旦重启，你通过 SSH 登录的方式就会改变。除非你确定要恢复默认设置，否则请保留这个文件；关闭当前会话之前，先测试一个新的 SSH 会话。
:::
