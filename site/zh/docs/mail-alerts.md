---
title: 邮件与告警
description: 让网站通过 SMTP 服务商发送邮件，并在服务器需要注意或有人登录时，通过 Telegram、邮件、Discord、Slack 或 Webhook 接收告警。
---

# 邮件与告警

Nefo 没有邮件服务器。网站通过 Postfix 发送邮件，Postfix 作为只发不收的中继，把邮件转给你的 SMTP 服务商（Gmail、SendGrid、Amazon SES 等）。当 CPU、内存或磁盘占用过高、某个服务停止、备份失败、网站宕机或有人登录时，告警会通过 Telegram、邮件、Discord、Slack 或带签名的 Webhook 通知你。

本页介绍这两部分：从网站发送邮件，以及告警。要设置邮件服务商的 DNS 记录（MX、SPF、DMARC），见 [DNS（Cloudflare）](./dns#set-up-email-records)。

## 有意不提供邮件服务器 {#no-mail-server-by-design}

Nefo 不托管邮箱，也不接收邮件。完整的邮件服务器要占用内存，还需要持续维护，这不适合小型 VPS。请把邮箱放在邮件服务商那里。

对于外发邮件，Nefo 会把 Postfix 安装为只发不收的中继。它只监听这台服务器，并用你输入的账户登录你的服务商（Gmail、SendGrid、Amazon SES 等），把每封邮件转交给它。

## 从网站发送邮件 {#send-e-mail-from-websites}

设置好中继后，PHP 的 `mail()` 函数就能工作，因为 Postfix 提供了 PHP 使用的 `/usr/sbin/sendmail`。这样，联系表单、密码重置和 WordPress 通知不需要任何插件就能发送邮件。没有中继时，网站无法发送邮件。

### 设置中继 {#set-up-the-relay}

1. 前往 **设置** → **邮件**。
2. 选择你的 **服务商**。面板会为它填好 **SMTP 服务器**、**端口** 和 **加密**。
3. 输入你在服务商处账户的 **用户名** 和 **密码**。**服务商** 下方的提示会说明该服务商需要什么。
4. 可选：填写 **统一发件人（可选）**，例如 `noreply@example.com`。
5. 第一次请点击 **安装并保存**：面板会安装并配置 Postfix。之后，这个按钮变为 **保存**。

设置在后台运行，并带有实时日志。完成后，发送一封测试邮件（见下文）。

| 服务商 | SMTP 服务器 | 用户名和密码 |
|---|---|---|
| Gmail / Google Workspace | `smtp.gmail.com` | 你的 Gmail 地址，以及一个应用专用密码（Google 账号 → 安全性 → 两步验证 → 应用专用密码），而不是你的普通密码 |
| Microsoft 365 / Outlook | `smtp.office365.com` | 你的 Microsoft 365 地址。该邮箱必须允许 SMTP 发信 |
| SendGrid | `smtp.sendgrid.net` | `apikey`（已为你填好），以及一个带 Mail Send 权限的 SendGrid API 密钥。请用已验证的发件人发信 |
| Mailgun | `smtp.mailgun.org` | 你的 Mailgun 域名的 SMTP 凭据 |
| Amazon SES | `email-smtp.us-east-1.amazonaws.com` | 你的 SES SMTP 凭据。把服务器改为你所在区域的端点，并用已验证的地址或域名发信 |
| Brevo | `smtp-relay.brevo.com` | 你的 Brevo SMTP 登录名和 SMTP 密钥 |
| 其他 SMTP 服务器 | 你的服务商的服务器 | 587 端口加 STARTTLS，或 465 端口加 SSL/TLS。大多数云服务商会封锁 25 端口 |

每个预设都使用 587 端口加 STARTTLS。

### 字段和规则 {#fields-and-rules}

| 字段 | 输入什么 |
|---|---|
| **SMTP 服务器** | 服务商的主机名，或一个 IP 地址 |
| **端口** | 1 到 65535，通常是 587 或 465 |
| **加密** | **STARTTLS**（587 端口）、**SSL/TLS**（465 端口），或 **无（仅限本服务器）** |
| **用户名**、**密码** | 两者都填或都不填，不能有空格。**密码** 留空表示保留已保存的密码 |
| **统一发件人（可选）** | 一个邮箱地址，用来替换每封邮件的发件人 |

- 只有中继就在这台服务器上（`localhost`、`127.0.0.1` 或 `::1`）时，才接受 **无（仅限本服务器）**。不加密的话，密码会以明文在互联网上传输。
- 使用 STARTTLS 或 SSL/TLS 时，如果连接无法加密，Postfix 会拒绝发送。
- 当你的服务商只接受已验证的发件人地址时，请使用 **统一发件人（可选）**。它会替换每个网站发出的每封邮件的发件人。
- 服务商的密码保存在 `/etc/postfix/sasl_passwd` 中，只有 root 能读取。网站永远看不到它。

### 发送测试邮件 {#send-a-test-message}

1. 在表单下方的 **发送测试邮件至…** 中输入一个地址。
2. 点击 **发送测试**。

面板会在邮件日志中跟踪这封邮件，最多等一分钟，等待服务商的响应。结果会说明邮件是已发送、尚未投递（Postfix 会继续尝试），还是被服务商拒收，后面附上服务商的回复。如果服务商在一分钟内没有响应，邮件会留在队列中，Postfix 会继续尝试。

### 检查中继 {#check-the-relay}

在卡片顶部：

- **正在通过 … 中继** 表示 Postfix 正在运行并使用你的服务商。**Postfix 未运行** 表示它已停止。
- **… 封邮件等待发送** 统计仍在队列中的邮件。Postfix 会对一封邮件持续重试最多 2 天。
- **上次投递问题：** 显示最近一次邮件无法投递的原因，内容是服务商给出的。

### 移除中继 {#remove-the-relay}

点击 **移除** 并确认。面板会卸载 Postfix 并清除这些设置。网站将无法再发送邮件。

## 告警 {#alerts}

服务器需要注意时，告警会通过 Telegram、邮件、Discord、Slack 或你自己的 Webhook 给你发消息。前往 **设置** → **告警**。每个渠道都是可选的：想设置多少个都可以，每条告警会同时发往所有渠道。某个渠道失败不会影响其他渠道。

面板每分钟检查一次服务器。每个问题报告一次，问题持续期间每 6 小时再报告一次，解决后再报告一次。每条消息都以方括号中的服务器名称开头。邮件告警的主题是“Server alert:”（服务器告警：）后接服务器名称。

### 哪些情况会发送告警 {#what-sends-an-alert}

| 告警 | 何时发送 | 默认值 |
|---|---|---|
| **CPU 高于此百分比** | 5 分钟平均 CPU 占用达到上限 | 90 |
| **内存高于此百分比** | 内存占用达到上限 | 90 |
| **磁盘高于此百分比** | 磁盘（`/`）用到这个比例 | 90 |
| **服务停止（Nginx、PHP、MariaDB…）** | 某个开机启动的服务没有在运行 | 开启 |
| **备份、恢复或证书续期失败** | 备份、恢复、网站证书续期或面板证书续期失败 | 开启 |
| 网站监控（每个网站的 **监控** 标签页） | 受监控的网站宕机（连续 2 次检查失败）、恢复正常，或提供的证书在 14 天内到期 | 关闭 |
| **SSH 登录** | 有人通过 SSH 登录：每次都发，或者只在来自新地址时发 | **不告警** |
| **10 分钟内 SSH 登录失败次数** | 10 分钟内 SSH 登录失败达到这个次数 | 0（关闭） |
| **面板登录** | 有人登录面板：每次都发，或者只在来自新地址时发 | **不告警** |
| **每日检查发现新问题时告警**（在 **安全检查** 页面上） | 每日 [安全检查](./security-check) 发现新的严重问题或警告 | 关闭 |
| **PHP 文件变化时告警**（网站的 **文件锁定** 标签页） | 对锁定网站的每日检查发现 PHP 文件被修改、添加或删除（见 [文件锁定](./website-protection#alerts-when-php-files-change)） | 关闭 |

- 把上限设为 0 即关闭它。其他上限必须在 10 到 100 之间。
- 监视的服务包括 Web 服务器（Nginx、Apache 或 OpenLiteSpeed）、PHP-FPM、MariaDB 或 MySQL、PostgreSQL、Redis、Memcached 和 fail2ban（如果已安装）。
- 在设置至少一个渠道之前，不会检查任何东西。
- 网站监控需要为每个网站单独开启，即使没有渠道，它的检查也会运行：**监控** 标签页会显示结果。宕机告警和其他告警一样每 6 小时重复一次。见 [网站](./websites#monitoring)。

### 登录告警 {#sign-in-alerts}

有人通过 SSH 登录服务器或登录面板时收到消息，这样不是你本人的登录就不会被忽略。

1. 前往 **设置** → **告警**，设置至少一个渠道。
2. 在 **登录** 下，为 **SSH 登录** 和 **面板登录** 选择：
   - **不告警**；
   - **每次登录**；
   - **仅来自新地址时**：从该账户最近 180 天内没有登录过的地址登录。
3. 可选：在 **10 分钟内 SSH 登录失败次数** 中，输入失败多少次时发送告警，范围为 3 到 100000。0 表示关闭。
4. 点击 **保存**。

SSH 告警会写明账户、地址、账户的登录方式和时间，例如：

```
[server1] 🔑 SSH sign-in: admin from 203.0.113.10 (new address) with a key, 2026-11-27 10:31:05 +07
```

同一分钟内的多次登录会合并在一条消息中。一连串失败的尝试会列出尝试次数最多的地址：

```
[server1] ⚠️ 57 failed SSH sign-in attempts in the last 10 minutes, from 198.51.100.7 (40), 203.0.113.5 (17).
```

工作原理：

- 没有程序一直盯着 SSH 日志。面板每分钟读取一次 SSH 服务（`ssh.service` 或 `sshd.service`）自上次读取以来写入系统日志（journal）的内容，并记住读到哪里。SSH 告警关闭时没有任何开销。
- 开启 SSH 告警后，它们从那一刻开始：之前的登录永远不会被报告。最近 30 天内 SSH 登录过的地址算作已知地址。
- 对于面板，活动日志中登录过的地址算作已知地址。
- 对于 IPv6，同一个 /64 中的所有地址算作一个地址，因为大多数服务商给每个客户分配一个 /64。SSH 和面板各最多记住 500 个地址。
- 如果面板停止超过一小时，它重新启动时只读取 SSH 日志的最近一小时。
- 失败的尝试指密码错误和未知的账户名。关闭密码登录时，被拒绝的密钥不计入。每小时最多发送一条连续失败告警。
- 要封禁不停尝试的地址，请在 **防火墙与 SSH** 页面安装 fail2ban。见 [防暴力破解](./security#brute-force-protection-fail2ban)。

### 通过 Telegram 接收告警 {#get-alerts-by-telegram}

1. 在 Telegram 中打开 BotFather，发送 `/newbot` 并回答它的问题。它会回复机器人的令牌：一串数字、一个冒号，然后是一长串字母和数字。
2. 打开你的新机器人，给它发送任意一条消息。在你给机器人写信之前，它不能给你写信。
3. 获取你的聊天 ID，例如通过 userinfobot。它是一个数字。
4. 在 **设置** → **告警** 中，输入 **机器人令牌** 和 **聊天 ID**。
5. 点击 **保存**，然后点击 **发送测试**。

要把告警发到群组或频道，请先把机器人加进去。群组的聊天 ID 以减号开头。对于公开频道，也可以输入它的 `@name`。

保存后，令牌会显示为圆点。保持圆点不动即可保留已保存的令牌。如果清空 **聊天 ID** 并保存，令牌也会被删除。

### 通过邮件接收告警 {#get-alerts-by-e-mail}

告警邮件由面板直接通过 SMTP 服务器发送，有它自己的设置，与网站的中继分开。你可以使用同一个服务商账户。

1. 在 **邮件** 下，输入 **SMTP 服务器** 和 **端口**（默认 587）。
2. 如果服务器需要登录，输入账户的 **用户名** 和 **密码**。
3. 输入 **发件人** 和 **收件人** 地址。
4. 点击 **保存**，然后点击 **发送测试**。

在 465 端口上，连接从一开始就是加密的。在其他端口上，服务器提供 STARTTLS 时面板会使用它。如果服务器不提供 STARTTLS，而又设置了用户名，面板会拒绝发送，这样密码永远不会以明文传输。已保存的密码会像 Telegram 令牌一样显示为圆点。

### 在 Discord 中接收告警 {#get-alerts-in-discord}

1. 在 Discord 中，打开要接收告警的频道的设置，然后进入 **Integrations**（整合）→ **Webhooks** → **New Webhook**（新 Webhook）。
2. 可以给 Webhook 起个名字，然后点击 **Copy Webhook URL**（复制 Webhook URL）。
3. 在 **设置** → **告警** 中，把它粘贴到 **Discord** 下的 **Webhook 地址** 中。
4. 点击 **保存**，然后点击 **发送测试**。

URL 的样子是 `https://discord.com/api/webhooks/<id>/<token>`。`discordapp.com` 上的地址以及 `ptb.` 和 `canary.` 版本也可以，加上 `?thread_id=` 还可以发到某个子区。超过 Discord 上限 2000 个字符的告警会被截断。告警中的提及（例如 `@everyone`）不会通知任何人。

### 在 Slack 中接收告警 {#get-alerts-in-slack}

1. 前往 [api.slack.com/apps](https://api.slack.com/apps)，点击 **Create New App**（创建新应用）→ **From scratch**（从头开始），并选择你的工作区。
2. 打开 **Incoming Webhooks**（传入 Webhook），开启它，然后点击 **Add New Webhook to Workspace**（向工作区添加新 Webhook）并选择频道。
3. 复制 **Webhook URL**。它以 `https://hooks.slack.com/services/` 开头。
4. 在 **设置** → **告警** 中，把它粘贴到 **Slack** 下的 **Webhook 地址** 中，点击 **保存**，然后点击 **发送测试**。

只接受传入 Webhook：Slack 的工作流 Webhook 需要其他字段。

### 把告警发送到 Webhook {#send-alerts-to-a-webhook}

Webhook 会把每条告警以 JSON 格式发到你选择的地址：你自己的脚本、n8n、Home Assistant，或这里没有列出的聊天工具。在 **Webhook** 下，输入它的 **Webhook 地址**；如果接收端会检查签名，再输入 16 到 200 个字符、不含空格的 **签名密钥（可选）**。然后点击 **保存** 和 **发送测试**。

每条告警都是一个带 `Content-Type: application/json` 的 `POST`，请求体如下：

```json
{"server":"vps-1","text":"⚠️ The disk is 93% full.","time":"2026-11-26T08:15:00Z","kind":"alert"}
```

| 字段 | 值 |
|---|---|
| `server` | 服务器的主机名 |
| `text` | 告警内容，不含服务器名称。已解决的问题以“✅ Resolved:”（已解决：）开头 |
| `time` | 发送时间，UTC，RFC 3339 格式 |
| `kind` | `alert`，或 **发送测试** 时为 `test` |

任何 2xx 响应都算作已送达。面板最多等 10 秒，不跟随重定向，也不显示接收端的响应内容，只显示它的状态码。

#### 验证签名 {#verify-the-signature}

设置了密钥时，每个请求都带有请求头 `X-Nefo-Signature: sha256=`，后接以密钥为键、对请求体计算的 HMAC-SHA256，十六进制表示。请在解析 JSON 之前，对收到的原始请求体计算它，并用恒定时间比较。在 Python 中：

```python
import hashlib, hmac

def verify(secret: bytes, body: bytes, signature: str) -> bool:
    expected = "sha256=" + hmac.new(secret, body, hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected, signature)
```

在 Node.js 中，先确认两者长度相同，再用 `crypto.timingSafeEqual` 把 `'sha256=' + crypto.createHmac('sha256', secret).update(rawBody).digest('hex')` 与请求头进行比较。签名的请求体包含 `time`：拒绝超过几分钟的请求，以防重放。

#### Webhook 不能使用的地址 {#webhook-addresses}

- URL 以 `https://` 开头。只有勾选 **允许 http:// 地址（不加密，适用于你自己网络中的接收端）** 之后，普通的 `http://` 地址才可用：不加密的话，告警及其签名在传输途中可能被读取。
- URL 中不能有用户名或密码，不能有 `#`，最多 2048 个字符。
- 为了让 Webhook 无法访问服务器自己的服务，面板从不连接回环地址（`127.0.0.0/8`、`::1`、`localhost`）、链路本地地址（`169.254.0.0/16`、`fe80::/10`）、云元数据服务（`169.254.169.254`、`100.100.100.200`、`168.63.129.16`、`fd00:ec2::254`）、`0.0.0.0/8` 或组播地址。每次发送告警时都会对主机名解析出的地址重新检查，所以指向 `127.0.0.1` 的名称也会被拒绝。
- 允许私有网络（`10.0.0.0/8`、`172.16.0.0/12`、`192.168.0.0/16`、`fd00::/8`），供家中或容器中的接收端使用。

### 已保存的 Webhook 地址 {#saved-webhook-urls}

Discord、Slack 和 Webhook 的 URL 中包含它们的令牌，所以保存后会像 Telegram 令牌一样显示为圆点，签名密钥也一样。保持圆点不动即可保留已保存的值。清空某个 URL 并保存，就会停用该渠道。清空 Webhook 的 URL 也会删除它的签名密钥和 http:// 许可。

### 发送测试告警 {#send-a-test-alert}

**发送测试** 会通过每个已保存的渠道发送一条测试消息。它使用已保存的设置，所以请先点击 **保存**。结果每个渠道一行，即 **Telegram**、**邮件**、**Discord**、**Slack** 或 **Webhook**，显示 **已发送** 或错误。如果没有设置任何渠道，面板会提示“请先设置一个渠道并保存。”

对于 Discord、Slack 和 Webhook，错误是以下之一：

- **无法连接 Webhook**，后接原因，例如 `no such host` 或 `connection refused`。
- **Webhook 拒绝了该消息（HTTP 404…）**：接收端返回了 2xx 以外的状态码。Discord 和 Slack 会附上它们的原因，例如 `Unknown Webhook` 或 `invalid_token`。重定向（3xx）也算作拒绝。
- **Webhook 在 10 秒内没有响应。**
- **… 是回环地址、链路本地地址或云元数据地址**：见 [Webhook 不能使用的地址](#webhook-addresses)。

真正的告警无法发送时，面板日志会说明，并写出渠道的名称，但从不写出它的 URL。

### 告警和仪表盘 {#alerts-and-the-dashboard}

**仪表盘** 上的 **需要注意** 列表和右上角的铃铛只在你登录时显示问题，而且它们检查的还有其他事项，例如双因素登录和防火墙。告警则在你不在时也能通知到你。见 [系统工具](./system#dashboard)。
