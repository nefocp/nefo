---
title: 导入网站
description: 从 cPanel、aaPanel、HestiaCP、CyberPanel 或其他服务器迁移网站，连同它们的文件、数据库、证书和定时任务。
---

# 导入网站

Nefo 可以从 cPanel 完整备份导入网站，连同数据库及其用户、证书和定时任务；也可以从 aaPanel、HestiaCP、CyberPanel 或任何其他服务器制作的网站压缩包和 SQL 转储导入。它会先给出计划，在你确认之前不改动这台服务器上的任何东西；如果导入失败，它会移除自己创建的内容。

导入有两种方式：

- **cPanel 完整备份**：一个 cPanel 账户的所有网站，连同数据库及其用户、证书和定时任务。
- **网站压缩包和 SQL 转储**：一个网站的文件，打包为 `.tar.gz`、`.tgz` 或 `.zip` 压缩包，可以再加上 `.sql` 或 `.sql.gz` 转储。适用于 aaPanel、HestiaCP、CyberPanel 或任何其他服务器。

导入从不碰旧服务器。Nefo 先读取压缩包并显示它找到的内容；在你确认之前，这台服务器上不会有任何改动。

## 制作备份 {#make-the-backup}

### cPanel {#cpanel}

在 cPanel 中打开 **Files → Backup**（文件 → 备份），点击 **Download a Full Account Backup**（下载完整账户备份）。目标保持为 **Home Directory**（主目录），然后点击 **Generate Backup**（生成备份）。cPanel 会把 `backup-M.D.YYYY_HH-MM-SS_USER.tar.gz` 写入你的主文件夹，准备好后会给你发邮件。

如果有 WHM 的 root 权限，可以改为运行 `/scripts/pkgacct USER`：它会写出 `/home/cpmove-USER.tar.gz`。两个文件的结构相同，Nefo 都能读取，旧版 cPanel 的也可以。

部分备份（“Home Directory”或“MySQL Databases”下载）不是完整备份。对于这类备份，请使用 **网站压缩包和 SQL 转储**。

### aaPanel {#aapanel}

1. 打开 **Website**（网站），在网站所在的行点击 **Backup**（备份），然后点击 **Backup now**（立即备份）。aaPanel 会写出 `/www/backup/site/DOMAIN/web_DOMAIN_YYYYMMDD_HHMMSS_xxxxxx.tar.gz`（旧版本：`/www/backup/site/`）。
2. 打开 **Database**（数据库），在数据库所在的行点击 **Backup**，然后点击 **Backup now**。aaPanel 会在 `/www/backup/database/` 下写出 `db_NAME_YYYYMMDD_HHMMSS_mysql_data.sql.gz`。

网站压缩包的顶层是网站的文件夹；Nefo 导入其中的内容。Nefo 还会从网站的 `.user.ini` 中读取旧文件夹（`/www/wwwroot/DOMAIN`），并在 WordPress 的数据库中替换它。

### HestiaCP、CyberPanel 和其他服务器 {#hestiacp-cyberpanel-and-other-servers}

它们自己的备份会把每个网站打包成它们自己格式的压缩包（HestiaCP 用 zstd 压缩），所以请改为在旧服务器上通过 SSH 制作普通的压缩包：

```bash
# The website's files: the folder that holds index.php or index.html
tar czf /root/site.tar.gz -C /home/USER/web/example.com public_html
# Each database
mysqldump --single-transaction --routines --triggers DBNAME | gzip > /root/DBNAME.sql.gz
```

在 HestiaCP 上，文件夹是 `/home/USER/web/DOMAIN/public_html`，在 CyberPanel 上是 `/home/DOMAIN/public_html`，在普通服务器上通常是 `/var/www/html`。顶层是 `public_html` 文件夹的压缩包也可以：Nefo 导入其中的内容。

## 把压缩包放到这台服务器上 {#get-the-archive-onto-this-server}

以下任何一种方式：

- **SFTP 或 scp**，传到 root 的某个文件夹，例如 `scp backup.tar.gz root@NEW-SERVER:/root/`。
- **文件**：上传，或使用 **从 URL 下载**（见 [文件](./files)）。
- **网址**：给导入一个 `http` 或 `https` 地址。Nefo 会把文件下载到这台服务器磁盘上的 `/var/tmp/nefo-import`，导入后删除它。这台服务器自己网络中的地址会被拒绝。

Nefo 可以从任何位置读取压缩包，系统文件夹（`/etc`、`/proc`、`/sys`、`/dev`、`/run`、`/boot`）和它自己的数据除外。磁盘需要容纳网站的文件，以及约为转储两倍大小的空间；计划会显示需要多少、可用多少。

## 逐步导入 {#import-step-by-step}

1. 打开 **网站**，点击 **添加网站** 旁边的 **导入**。
2. 选择 **cPanel 完整备份** 或 **网站压缩包和 SQL 转储**。
3. 输入文件的路径（**浏览** 会列出这台服务器上的文件夹和压缩包）或它的网址。如果是网站压缩包，再添加 SQL 转储。
4. 点击 **分析**。Nefo 会完整读取一遍压缩包，不解压它；大的备份需要几分钟。**停止** 可以中止。
5. 查看计划，勾选要迁移的内容（见下文）。点击 **导入**。
6. 查看日志。导入完成之前，新网站保持关闭（错误 503）。**停止导入** 会移除到目前为止创建的内容。
7. 完成后，页面会列出新网站（带 **打开网站** 和 **管理**）、数据库，以及值得检查的事项。

离开页面后导入会继续；再次打开 **网站** → **导入** 可以查看。分析过的压缩包会被记住六个小时。

## 从 cPanel 导入哪些内容 {#what-is-imported-from-cpanel}

| 备份中 | 在 Nefo 上 |
|---|---|
| 主域名、每个附加域名和每个子域名 | 各自成为一个网站。cPanel 为附加域名创建的内部子域名（`addon.main.com`）会被略过。 |
| 停放域名（别名） | 主域名网站的别名；如果旧服务器上有 `www.`，也一并带上。 |
| 每个文档根目录（`public_html`、`public_html/blog`、`addon.com`……） | 网站的 `public` 文件夹。其中属于另一个导入域名的文件夹只会进入那个域名的网站。 |
| PHP 版本（`ea-php81`） | 这里安装了相同版本时使用它，否则使用这台服务器的默认版本。没有 PHP 文件的网站会成为静态网站。 |
| `mysql/DB.sql` | 一个同名数据库（如果这里的名称未被占用），否则为 `NAME_2`，并关联到使用它的网站。 |
| 数据库用户及其密码（`mysql.sql`） | 每个数据库对应一个用户：它的 WordPress `wp-config.php` 中写的那个用户，否则是一个有权访问它的用户。用户保留原来的密码：MariaDB 直接接受旧的密码哈希，或者 Nefo 使用 `wp-config.php` 中的密码。否则用户会得到新密码。 |
| WordPress | 通过它的 `wp-config.php` 识别。Nefo 会把网站标记为 WordPress，应用 WordPress 重写规则，在数据库名称、用户或密码变化时让 `wp-config.php` 指向新的值，并在数据库中把旧文件夹（`/home/USER/public_html`）替换为新文件夹。 |
| 证书（`apache_tls/`、`sslcerts/`、`ssl/`） | 勾选后作为你自己的证书安装：只有当它的私钥在备份中、今天仍然有效、并且覆盖网站的每个名称时才安装。否则请在导入后从 Let's Encrypt 获取证书。 |
| 定时任务（`cron/USER`） | 会显示出来，只有勾选后才安装：以网站自己的用户（从不以 root）运行，旧的文档根目录和 cPanel 的 PHP 程序会被替换为这台服务器上的。仍然提到旧主文件夹的命令会被标记出来。 |
| `.htaccess`、`.user.ini` | 旧文件夹被替换为新文件夹，cPanel 的 PHP 处理程序行会从 `.htaccess` 中移除。 |

计划会说明某个域名为什么不能导入，例如这里已有网站使用该域名。

## 哪些内容不会导入 {#what-is-not-imported}

- **邮件**：邮箱、转发器、自动回复和过滤器。Nefo 没有邮件服务器。关闭旧服务器之前，请把邮件迁移到邮件服务。网站发出的邮件见 [邮件与告警](./mail-alerts)。
- **DNS 区域**：Nefo 不运行 DNS 服务器。在管理域名 DNS 的地方把域名指向这台服务器，或使用 [DNS（Cloudflare）](./dns)。
- **FTP 账户**：这里的网站使用 SFTP，每个网站一个账户（网站的 **SFTP** 标签页）。
- 邮件列表、PostgreSQL 数据库、主文件夹中文档根目录之外的文件，以及 cPanel 自己的设置。

## 网站压缩包 {#website-archives}

- 压缩包顶层的单个文件夹（如 aaPanel 的备份）不会保留：它里面的内容会成为网站。含有 `public_html` 的主文件夹也可以。
- 网站可以是新网站（会询问它的域名、`www.` 和 PHP 版本），也可以是已有的空网站：`public` 文件夹中只有欢迎页面、而且文件没有被 [锁定](./website-protection#lock-a-website-s-files) 的 PHP 网站或静态网站。
- 每个 SQL 转储都会成为一个新数据库，带一个同名用户和一个新密码。WordPress 的 `wp-config.php` 会指向它所写的那个转储（或唯一的转储）。其他应用请把配置改为新的数据库、用户和密码（**数据库** 页面上的 **显示登录信息**）。

## 来自 MySQL 8 的转储 {#dumps-from-mysql-8}

MySQL 8（aaPanel 和很多 cPanel 服务器都运行它）制作的转储会使用 MariaDB 不认识的排序规则，例如 `utf8mb4_0900_ai_ci`，还有 MariaDB 拒绝的行。Nefo 会在数据流入时调整它们，不改动你的数据：

- MySQL 8 的排序规则会换成最接近的 MariaDB 排序规则（`utf8mb4_unicode_520_ci`，区分大小写的换成 `utf8mb4_bin`，或该语言自己的排序规则，例如 `utf8mb4_vietnamese_ci`）。
- 只有 MySQL 8 才有的设置，例如 `DEFAULT ENCRYPTION='N'` 和 `SRID`，会变成普通注释。
- `SET @@GLOBAL.GTID_PURGED` 及其前后的 `SQL_LOG_BIN` 行会被去掉。
- 针对转储自己数据库的 `CREATE DATABASE` 和 `USE` 会被去掉，这样它会进入新数据库。

数据中的文本永远不会被改动。日志会说明调整了多少行。

## Nefo 如何处理压缩包 {#how-nefo-treats-archives}

来自另一台服务器的压缩包里可能有任何东西，所以 Nefo 会检查每个条目：

- 含有 `..` 或绝对路径的名称、符号链接、硬链接、设备文件和 FIFO 会被略过，日志会说明有多少。
- 文件会去掉 setuid 和 setgid，以及属组和其他人的写权限。一切都属于网站自己的 Linux 用户。
- 文件只通过网站的文件夹写入：什么都不会落到它之外。
- 超过 8 GB 的文件会被略过；解压后远远大于自身大小的压缩包（压缩炸弹）会被拒绝。
- SQL 以一个临时数据库账户运行，它只能使用新数据库，从不以 root 运行。

如果导入失败或被停止，Nefo 会移除它创建的网站和数据库。无法移除的网站会保持暂停状态，并附有导入未完成的说明。

## 导入之后 {#after-the-import}

1. **改 DNS 之前先测试**：在你电脑的 `hosts` 文件中把域名指向这台服务器，然后打开网站。
2. **DNS**：把每个域名的 A（和 AAAA）记录指向这台服务器。
3. **HTTPS**：DNS 指向这里之后，在每个网站的 **SSL / HTTPS** 标签页上获取 Let's Encrypt 证书。导入的证书不会由 Nefo 续期。见 [SSL 证书](./ssl)。
4. **PHP**：在每个网站的 **PHP** 标签页上检查它的 PHP 版本和设置。
5. **定时任务**：在 **定时任务** 页面添加你没有导入的任务。
6. **邮件**：关闭旧服务器之前，确认邮件在别处已能正常工作。

## 出了问题怎么办 {#if-something-goes-wrong}

- **“不是 cPanel 完整备份”**：压缩包中没有 `userdata/main`。它是部分备份或普通压缩包：请选择 **网站压缩包和 SQL 转储**。
- **“使用了另一个数据库”**：转储会写入第二个数据库。请分别导出每个数据库，不要使用 `--databases` 或 `--all-databases`。
- **“可用磁盘空间不足”**：释放一些空间，或者一次导入更少的网站。
- **WordPress 网站显示“Error establishing a database connection”（建立数据库连接时出错）**：在 **文件** 中打开它的 `wp-config.php`，把 `DB_NAME`、`DB_USER` 和 `DB_PASSWORD` 与 **数据库** 页面上的 **显示登录信息** 进行比较。
