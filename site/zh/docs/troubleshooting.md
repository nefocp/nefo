---
title: 故障排除
description: Nefo 最常见问题的解决办法，从打不开面板，到证书错误，再到显示 502 或 503 的网站。
---

# 故障排除

Nefo 的大多数问题都可以在服务器上以 root 身份快速解决：`nefo info` 显示丢失的面板地址，`nefo reset-password` 设置新密码（加上 `--disable-2fa` 还会关闭双因素登录），`journalctl -u nefo` 显示面板无法启动的原因。本页还介绍证书错误、显示 502 或 503 的网站，以及恢复失败。

大多数解决办法都需要以 root 身份通过 SSH 访问服务器。如果 SSH 也用不了，请使用服务商控制面板中的 Web 控制台。

## 打不开面板 {#you-can-t-open-the-panel}

### 忘了地址 {#you-forgot-the-address}

在服务器上运行：

```sh
nefo info
```

输出会显示完整的 URL，包括端口和秘密路径：如果你给面板设置了域名，会使用面板的域名，同时也会使用服务器的第一个 IP 地址。如果那个地址是私有地址，加上 `--host` 和你的公网 IP 地址，例如 `nefo info --host 203.0.113.10`。

### 页面显示 404 {#the-page-shows-404}

面板只在它的秘密路径下响应，例如 `https://203.0.113.10:28443/k3x9p2qa7m1z/`。其他所有地址都返回 `404`，包括不带路径的地址。用 `nefo info` 检查路径。

还要确认地址以 `https://` 开头。用 `http://` 时，面板会返回错误，而不是登录页面。

### 页面完全打不开 {#the-page-doesn-t-load-at-all}

浏览器无法连接，或者一直等待然后放弃时，说明面板没有运行，或者有东西拦住了连接。

1. **检查面板是否在运行：**

   ```sh
   systemctl status nefo
   ```

   如果状态不是 `active (running)`，见 [面板无法启动](#the-panel-doesn-t-start)。

2. **检查服务商的云防火墙。** 它必须放行面板的 TCP 端口。用 `nefo info --port` 查看端口。

3. **检查你的 IP 地址是否仍被允许。** 如果你在 ufw 或云防火墙中把面板端口限制为只对你自己的 IP 地址开放，而你的 IP 地址变了（家庭和移动网络中很常见），请放行新地址。在 ufw 中，使用你的新地址和面板的端口：

   ```sh
   ufw allow from 198.51.100.7 to any port 28443 proto tcp
   ```

4. **检查是否被 fail2ban 封禁。** 如果你从面板安装了 fail2ban，10 分钟内面板登录失败 10 次（SSH 为 5 次）后，它会封禁该地址一小时。解封你的地址：

   ```sh
   fail2ban-client set panel unbanip 198.51.100.7
   ```

   对于 SSH，请使用 jail `sshd`，而不是 `panel`。

### “失败次数过多” {#too-many-failed-attempts}

15 分钟内登录失败 5 次后，面板会在 15 分钟内拒绝来自你的地址的后续尝试，并显示“失败次数过多。请 … 秒后重试。”错误的双因素验证码也会计入。请等时间过去。锁定记录保存在内存中，所以 `systemctl restart nefo` 也会清除它。

### 面板无法启动 {#the-panel-doesn-t-start}

在日志中查看原因：

```sh
journalctl -u nefo -n 50 --no-pager
```

| 日志中写着 | 怎么办 |
|---|---|
| `address already in use` | 另一个程序占用了面板的端口。停止那个程序，或在 `/etc/nefo/config.json` 中更改端口（见 [命令行](./cli#the-configuration-file)）。 |
| 关于 `/etc/nefo/config.json` 的错误 | 配置文件有错误，例如在你编辑过之后。修正提示中指出的那一行。 |
| `database schema version … is newer than this panel supports` | 较旧版本的程序在使用较新版本的数据。重新安装最新的发布版本（见 [更新](./update#if-an-update-fails)）。 |

修复后，运行 `systemctl restart nefo`。

## 忘了密码 {#you-forgot-the-password}

在服务器上设置一个新的随机密码：

```sh
nefo reset-password
```

它会打印新密码，并让该账户在所有地方退出登录。用新密码登录，然后在 **账户** 中设置你自己的密码。

## 丢了双因素认证的手机 {#lost-your-2fa-phone}

如果再也无法从身份验证器应用获取验证码，请在服务器上关闭双因素认证。这也会设置一个新密码：

```sh
nefo reset-password --disable-2fa
```

用新密码登录，然后用新手机在 **账户** 中重新设置双因素认证。

### 验证码总是“无效” {#codes-are-always-not-valid}

验证码取决于时间。面板接受当前的验证码以及它前后紧邻的验证码，每个验证码只能用一次。如果所有验证码都失败，请检查服务器的时钟是否正确：

```sh
timedatectl
```

`System clock synchronized` 应该是 `yes`。同时检查手机上的时间。

## 浏览器警告证书有问题 {#the-browser-warns-about-the-certificate}

在面板有自己的证书之前，它使用自签名证书，所有浏览器都会对此发出警告。这是正常的。给面板一个域名和免费证书，警告就会消失（见 [入门步骤](./first-steps#give-the-panel-its-own-domain)）。

如果面板已有证书，浏览器仍然警告：

- 你是用 IP 地址打开面板的。证书是为域名签发的：请使用 `https://panel.example.com:28443/k3x9p2qa7m1z/`。
- 证书因无法续期而过期了。检查 **设置** → **证书**，并点击 **获取新证书**。原因与下一节相同。

## 证书申请失败 {#a-certificate-request-fails}

在向 Let's Encrypt 申请之前，面板会检查每个域名能否到达这台服务器：它在服务器上放一个测试文件，再通过该域名经 HTTP 在 80 端口上获取它。如果失败，它什么也不申请，并在“these names do not reach this server yet, so no certificate was requested”（这些名称还无法到达这台服务器，因此没有申请证书）下列出每个问题。

| 提示中写着 | 原因和解决办法 |
|---|---|
| `no DNS record. Add an A record that points to this server's IP address.` | 该域名还没有地址。在你的 DNS 服务商处添加 A 记录。新记录可能需要一段时间才能在所有地方生效。 |
| `no answer over HTTP on port 80 (…). Check the DNS record and the firewall.` | 80 端口已关闭，或者该域名指向另一台服务器。在 ufw 和服务商的云防火墙中开放 80 端口，并检查 Web 服务器是否在运行。 |
| `answered HTTP …, but not from this server. Check that the DNS record points here.` | 有别的东西为该域名作了响应：记录指向另一台服务器，或者前面有代理或 CDN，例如 Cloudflare 的代理（橙色云朵）。 |

每一行都以域名及其指向的地址开头。如果那些不是你服务器的地址，请先修正 DNS 记录。如果列出了不属于你服务器的 IPv6 地址，请修正或删除 AAAA 记录。

**在 Cloudflare 代理后面：**

- 对于网站，要么在申请证书期间把记录设为“DNS only”（仅 DNS），要么在网站的 **SSL / HTTPS** 标签页上勾选 **通过 Cloudflare DNS 验证**。DNS 验证需要一个 Cloudflare API 令牌，在 **设置** → **Cloudflare** 中添加。通配符名称（`*.example.com`）总是需要 DNS 验证。
- 对于面板自己的域名，记录必须直接指向服务器，不经过代理。

**对于面板自己的域名**，在你安装 Web 服务器之前，**证书** 标签页会显示“需要 Web 服务器”，因为由 Web 服务器在 80 端口上响应 Let's Encrypt。请先在 **网站** 页面安装它。

如果检查通过，但 Let's Encrypt 仍然拒绝，任务日志会显示它的原因。Let's Encrypt 会限制失败尝试的次数，所以请先解决原因再重试。

自动续期失败时，面板会显示“example.com 的证书续期失败”，并每 12 小时重试一次。证书在过期之前仍然有效。

## 网站显示错误 {#a-website-shows-an-error}

| 访客看到的 | 原因 | 怎么办 |
|---|---|---|
| 文字“This website is temporarily unavailable.”（此网站暂时无法访问，HTTP 503） | 网站被暂停了，可能是手动暂停，也可能是因为到期日期已过。 | 打开网站的 **到期与暂停** 标签页。取消勾选 **暂停此网站**，并清除或更改 **到期日期**。如果日期仍在过去，面板会在一小时内再次暂停该网站。 |
| Web 服务器的“502 Bad Gateway”或“503 Service Unavailable”页面 | 网站背后的程序没有响应：PHP 网站是 PHP-FPM，应用网站是应用或容器。Nginx 显示 502，Apache 和 OpenLiteSpeed 可能显示 503。 | 对于 PHP，打开 **服务**，启动或重启该网站 PHP 版本的 PHP-FPM 服务，例如 `php8.3-fpm`。对于应用，网站会显示“应用已停止”：在它的 **日志** 标签页上查看 **应用日志**。 |
| 所有网站都没有响应 | Web 服务器已停止。**网站** 页面会显示它“未在运行”。 | 在 **服务** 页面启动 Web 服务器。 |
| HTTP 429 | 网站的速率限制已开启（仅 Nginx），访客发送的请求太多。 | 在网站的 **阻止访问** 标签页上提高限制，或设为 0 关闭它。 |
| HTTP 403 | 访客的地址在网站的封禁地址中，或者 Web 应用防火墙拦截了请求。 | 检查 **阻止访问** 标签页上的 **封禁的 IP 地址**，以及 **Web 防火墙（WAF）** 标签页上最近的防火墙事件。 |

其他错误请在网站的 **日志** 标签页上查看它的错误日志和 PHP 错误。

## 备份上传或恢复失败 {#a-backup-upload-or-restore-fails}

- **恢复失败，提示“the passphrase does not unlock it”（密码短语无法解锁）。** 这份副本是用与存储当前密码短语不同的另一个密码短语加密的。重新恢复，并在 **加密密码短语（可选）** 中输入那个密码短语。见 [加密副本](./backups#encrypt-the-copies)。
- **Google Drive 上传在一周后失败。** OAuth consent screen（OAuth 同意屏幕）仍处于 **Testing**（测试）状态：在 Google Cloud console 中把它设为 **In production**（正式发布），然后在 **备份** → **远程存储** 下点击 **更改** → **重新连接**。见 [Google Drive](./backups#google-drive)。
- **网站备份恢复成了静态网站。** 这份备份是 0.24.1 之前的版本创建的，只记录了 PHP 版本。对于应用网站或反向代理网站，请先按它的类型重新创建网站，然后再恢复。0.24.1 及以后版本创建的备份会以网站原来的类型恢复它。见 [恢复已删除的网站](./backups#restore-a-website-that-was-deleted)。

## 日志在哪里 {#where-the-logs-are}

| 日志 | 在服务器上 | 在面板中 |
|---|---|---|
| 面板 | `journalctl -u nefo` | — |
| 网站的访问日志和错误日志 | `/var/log/nginx/`、`/var/log/apache2/` 或 `/var/log/openlitespeed/`，文件名为 `example.com.access.log` 和 `example.com.error.log` | 网站 → 你的网站 → **日志** |
| 网站的 PHP 错误 | `/var/www/example.com/logs/php-error.log` | 网站 → 你的网站 → **日志** → **PHP 错误** |
| 应用网站 | `journalctl -u panel-app-example_com` | 网站 → 你的网站 → **日志** → **应用日志** |
| 定时任务 | `/var/log/panel-cron/` | **定时任务** |
| 面板中的登录和更改 | — | **账户** → **最近活动** |

报告问题时，请附上 `nefo version` 的输出和相关的日志行。请先去掉密码和面板的秘密路径。
