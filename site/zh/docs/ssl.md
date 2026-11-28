---
title: SSL 证书
description: 获取免费的 Let's Encrypt 证书，通过 Cloudflare DNS 获取通配符证书，或安装你自己的证书，以及解决常见的失败。
---

# SSL 证书

Nefo 一键就能为每个网站获取免费的 Let's Encrypt 证书，并自动续期。它还可以通过 Cloudflare DNS 获取通配符证书、安装你已有的证书、把 HTTP 重定向到 HTTPS，以及发送 HSTS。每次申请之前，它都会检查每个名称能否到达服务器，这样失败的尝试不会计入 Let's Encrypt 的限制。

本页介绍前提条件、重定向到 HTTPS 和 HSTS、通过 Cloudflare DNS 获取通配符证书、安装你已有的证书、重新关闭 HTTPS，以及证书失败时该怎么办。

## 开始之前 {#before-you-start}

对于普通证书，Let's Encrypt 会通过普通 HTTP 检查网站的每个名称能否到达你的服务器。每个名称都必须通过：域名及其每个 [额外域名](./websites#domains-and-aliases)。

- **DNS：** 每个名称都有一条指向这台服务器 IP 地址（例如 `203.0.113.10`）的 A 记录。如果某个名称还有 AAAA（IPv6）记录，它也必须指向这台服务器，否则请删除它。
- **80 端口：** 服务器在 80 端口上响应来自互联网的请求。Nefo 安装 Web 服务器时会在 ufw 中开放 80 和 443 端口。如果你的主机服务商有自己的防火墙，也要在那里开放 80 端口。
- **代理：** 如果你的 DNS 在 Cloudflare，并且开启了代理（橙色云朵），请先阅读 [Cloudflare 代理](#cloudflare-proxy-orange-cloud)。

开启 HTTPS 即表示你接受 Let's Encrypt 的 [订户协议](https://letsencrypt.org/repository/)。

## 获取免费证书 {#get-a-free-certificate}

1. 打开 **网站**，然后打开该网站的 **SSL / HTTPS** 标签页。在网站列表中，**SSL** 列中的 **开启 HTTPS** 也会打开这个标签页。
2. 在 **Let's Encrypt 免费证书** 下，保持 **通过 Cloudflare DNS 验证** 未勾选。
3. 点击 **获取证书**。

日志会显示每一步：Nefo 检查每个名称能否到达这台服务器，向 Let's Encrypt 申请证书，验证每个名称，并在 Web 服务器中开启 HTTPS。最后会显示“Done. The certificate is valid until”（完成。证书有效期至）和日期。

网站获得第一张证书时，Nefo 还会把它安装的 WordPress 网站切换到 `https://` 地址，并用 `https://` 地址重启 Docker 应用。文件以 `fullchain.pem` 和 `privkey.pem` 保存在 `/var/lib/nefo/certs/example.com/` 中。

### 申请前的检查 {#the-check-before-the-request}

在向 Let's Encrypt 申请之前，Nefo 会在服务器上放一个测试文件，并通过每个名称经 HTTP 获取它。如果某个名称失败，Nefo 根本不会发送申请，所以失败的尝试不会计入 Let's Encrypt 的限制。日志会显示“these names do not reach this server yet, so no certificate was requested:”（这些名称还无法到达这台服务器，因此没有申请证书：），每个名称一行：

| 提示 | 需要修复什么 |
|---|---|
| `blog.example.com: no DNS record. Add an A record that points to this server's IP address.` | 添加 A 记录，并等到它生效。 |
| `example.com (203.0.113.10): no answer over HTTP on port 80 (…). Check the DNS record and the firewall.` | 在服务商的防火墙中开放 80 端口，并检查 Web 服务器是否在运行。 |
| `example.com (198.51.100.7) answered HTTP 404, but not from this server. Check that the DNS record points here.` | 该名称指向了另一台服务器：旧的 IP 地址、指向别处的 AAAA 记录，或者 Cloudflare 之类的代理。 |

括号中的地址是该名称解析到的地址。把它们与你服务器的 IP 地址进行比较。

## 把 HTTP 重定向到 HTTPS {#redirect-http-to-https}

网站有了证书后，在 **SSL / HTTPS** 标签页或 **概览** 标签页上开启 **将 HTTP 重定向到 HTTPS**。这样每个 `http://` 地址都会以永久重定向（301）跳转到 `https://`。Let's Encrypt 验证和 Git Webhook 仍然可以通过 HTTP 工作。

要让浏览器在重定向之前就使用 HTTPS，请同时开启 [HSTS](#hsts)。

## HSTS {#hsts}

HSTS（`Strict-Transport-Security` 响应头）告诉浏览器只通过 `https://` 打开网站，即使有人输入 `http://` 或点击旧链接也是如此。在 **SSL / HTTPS** 标签页上开启 **HSTS（Strict-Transport-Security）**。它需要证书和 **将 HTTP 重定向到 HTTPS**；关闭重定向也会关闭 HSTS。

之后，Nefo 会在网站的 HTTPS 响应中发送 `Strict-Transport-Security: max-age=15552000`（180 天，约 6 个月），适用于 Nginx、Apache 和 OpenLiteSpeed。勾选 **包含子域名** 会加上 `includeSubDomains`：浏览器随后也会对该域名的每个子域名使用 HTTPS，所以只有在所有子域名都有 HTTPS 时才勾选。Nefo 从不添加 `preload`：浏览器的预加载列表很难退出，如果真要提交，请自己把域名提交上去。

::: warning
收到这个响应头的浏览器，在最后一次访问后的 6 个月内都会拒绝通过普通 HTTP 访问该网站，即使你之后关闭了 HSTS 或 HTTPS 也一样。只对会一直保持 HTTPS 的网站开启 HSTS。
:::

在 Nginx 上，响应头设置在网站的整个 `server` 块中。如果你自己的指令（见 [网站](./websites#rewrite-rules-and-your-own-directives)）添加了一个带有自己 `add_header` 的 `location`，Nginx 会在那个 location 中丢弃网站的响应头：请在那里重复写一遍 `add_header Strict-Transport-Security "max-age=15552000" always;`。

要检查它，运行 `curl -sI https://example.com/ | grep -i strict-transport`。

## 自动续期 {#automatic-renewal}

Nefo 在启动时检查证书，之后每小时检查一次。Let's Encrypt 证书的剩余有效期只剩三分之一时，它就会续期：对于 90 天的证书，就是到期前 30 天。你什么都不用做，已暂停的网站也会继续续期。

如果续期失败，Nefo 每 12 小时重试一次。在此期间：

- 网站列表的 **SSL** 列中会显示“续期失败”；
- **SSL / HTTPS** 标签页会显示“上次获取证书失败。”和错误；
- 如果你为续期失败设置了告警，会发出告警：见 [邮件与告警](./mail-alerts)。

面板自己的证书，即 **设置** → **证书** 中的证书（见 [安全](./security#https-from-the-first-start)），也以同样的方式续期：在剩余三分之一有效期时续期，失败后每 12 小时重试。续期失败会记录在活动日志中，并发出同样的告警。

Nefo 注册 Let's Encrypt 账户时不带邮箱地址，所以 Let's Encrypt 不会给你写信提醒证书即将过期。请改为依靠面板的告警。

要手动续期（例如在你修复了某个问题之后），在 **SSL / HTTPS** 标签页上点击 **续期证书**。

## 通过 Cloudflare DNS 获取通配符证书 {#wildcard-certificates-through-cloudflare-dns}

像 `*.example.com` 这样的通配符名称只能通过 DNS 验证。Nefo 通过 Cloudflare 的 API 来完成，所以该域名的 DNS 必须托管在 Cloudflare。DNS 验证也适用于还无法通过 HTTP 访问的网站，例如在 Cloudflare 代理后面的网站。为了证书，Nefo 只会添加和删除 `_acme-challenge` TXT 记录。

### 创建 Cloudflare API 令牌 {#create-a-cloudflare-api-token}

同一个令牌也用于 [DNS 页面](./dns)。它的权限集中列在一处，即 [API 令牌](./dns#the-api-token)：证书只需要 Zone → DNS → Edit；DNS 页面还可以使用 Zone → Zone → Read 和 Zone → Cache Purge → Purge。

1. 在 Cloudflare 控制台中，打开 **My Profile → API Tokens → Create Token**（我的个人资料 → API 令牌 → 创建令牌）。
2. 使用 **Edit zone DNS**（编辑区域 DNS）模板。它会给出 Zone → DNS → Edit 权限。
3. 在区域资源下，包含你在这台服务器上使用的区域（域名）。
4. 创建令牌并复制它。

见 Cloudflare 的 [创建 API 令牌](https://developers.cloudflare.com/fundamentals/api/get-started/create-token/) 指南。

### 在 Nefo 中保存令牌 {#save-the-token-in-nefo}

1. 打开 **设置**，然后打开 **Cloudflare** 标签页。
2. 把令牌粘贴到 **API 令牌** 中，点击 **检查并保存**。

Nefo 会检查 Cloudflare 是否接受这个令牌、令牌是否处于激活状态，然后显示“令牌已保存”以及令牌的最后几个字符和它能使用的 **区域**。用 **更换 API 令牌** 更换它，或用 **移除** 删除它。

### 获取证书 {#get-the-certificate}

1. 对于通配符，打开网站的 **域名** 标签页，把 `*.example.com` 添加到 **额外域名** 中，然后点击 **保存**。
2. 在 **SSL / HTTPS** 标签页上，勾选 **通过 Cloudflare DNS 验证**。对于有通配符名称的网站，它已被勾选且不能更改：“必须使用：此网站有通配符域名（*.domain）。”
3. 点击 **获取证书**。

Nefo 会添加 TXT 记录 `_acme-challenge.example.com`，最多等 3 分钟，直到 Cloudflare 的域名服务器返回它，然后完成验证并删除这条记录。网站会记住这个选择，所以续期也会使用 DNS 验证。

证书覆盖了这些名称；但访客仍然需要把子域名指向这台服务器的 DNS 记录。

## 使用你自己的证书 {#use-your-own-certificate}

如果你从其他服务商购买了证书，或者要使用 Cloudflare 源服务器证书，请安装你自己的证书。

1. 打开网站的 **SSL / HTTPS** 标签页，转到 **你自己的证书**。
2. 粘贴 **证书（PEM，含中间证书）**：证书后接它的证书链。也可以选择 `.pem`、`.crt`、`.cer` 或 `.txt` 文件。
3. 粘贴 **私钥（PEM，未加密）**，或选择 `.pem`、`.key` 或 `.txt` 文件。
4. 点击 **安装**。

Nefo 会在改动任何东西之前检查这些文件，并拒绝：

- 不是 PEM 证书或大于 64 KB 的文件；
- 有密码短语保护的私钥。请先去掉密码短语，例如用 `openssl pkey -in key.pem -out plain.pem`；
- 不属于该证书的私钥；
- 尚未生效或已经过期的证书；
- 没有覆盖网站每个名称的证书。网站的通配符名称必须在证书中原样列出。

当浏览器不会直接信任它时（在 Cloudflare 代理后面使用 Cloudflare 源服务器证书时这是正常的），或者它在两周内过期时，它会安装证书并给出警告（“已安装，但有警告：”）。

Nefo 不会续期你自己的证书。在它的最后两周，**SSL / HTTPS** 标签页会显示“Nefo 不会续期你自己安装的证书。请在……之前安装新证书”和日期，仪表盘上的 **需要注意** 也会列出该网站。Nefo 还会每天提醒你一次：活动日志把它记录为一次续期失败，并发出续期失败的告警。请及时安装新证书，或用 **获取证书** 改回 Let's Encrypt。网站列表会用“自有证书”标记这些网站。

## 关闭 HTTPS {#turn-off-https}

要让网站回到普通 HTTP，点击它的 **SSL / HTTPS** 标签页底部的 **关闭 HTTPS** 并确认。然后 Nefo 会：

- 只通过 `http://` 提供网站：Web 服务器配置中的 HTTPS 部分被移除；
- 关闭 **将 HTTP 重定向到 HTTPS** 和 [HSTS](#hsts)；
- 删除 `/var/lib/nefo/certs/example.com/` 中的证书文件，无论来自 Let's Encrypt 还是你自己，这样就不会再有任何续期或提醒。Let's Encrypt 账户会为其他网站保留；
- 把它安装的 WordPress 网站切换回 `http://` 地址，并用 `http://` 地址重启 Docker 应用。

之后，打开该网站 `https://` 地址（来自书签、搜索结果或链接）的访客会看到浏览器错误，因为服务器已经没有该名称的证书了。如果网站开启过 HSTS，收到过这个响应头的浏览器会在最多 6 个月内继续拒绝普通 HTTP，所以对他们来说，在网站重新有 HTTPS 之前根本打不开。你随时可以用 **获取证书** 获取新证书。

要替换证书，不需要先关闭 HTTPS：直接获取新的 Let's Encrypt 证书，或在原证书之上安装你自己的证书。删除网站也会删除它的证书：见 [网站](./websites)。卸载 Web 服务器会保留证书。

## 证书失败时 {#when-a-certificate-fails}

错误会显示在网站的 **SSL / HTTPS** 标签页上的“上次获取证书失败。”下面，以及该次尝试的日志中。

### DNS 或 80 端口 {#dns-or-port-80}

见 [申请前的检查](#the-check-before-the-request)。大多数失败是因为 DNS 记录还没有指向这台服务器，或者服务商的防火墙关闭了 80 端口。

### Cloudflare 代理（橙色云朵） {#cloudflare-proxy-orange-cloud}

开启 Cloudflare 代理后，域名会解析到 Cloudflare 的地址，请求会先到达 Cloudflare，再到你的服务器。这时，**Always Use HTTPS**（始终使用 HTTPS，即重定向到 HTTPS，位于 **SSL/TLS → Edge Certificates**（SSL/TLS → 边缘证书）下）之类的 Cloudflare 设置可能会让检查请求无法原样到达服务器，Nefo 会报告该名称“answered HTTP …, but not from this server”（返回了 HTTP …，但不是来自这台服务器）。另见 [橙色云朵后面的 Let's Encrypt](./dns#let-s-encrypt-behind-the-orange-cloud)。你有三种解决办法：

- 勾选 **通过 Cloudflare DNS 验证**，通过 DNS 获取证书。代理可以保持开启，续期也以同样方式进行；
- 把 Cloudflare 源服务器证书作为 [你自己的证书](#use-your-own-certificate) 安装；
- 为该名称关闭代理（仅 DNS，灰色云朵）。续期也会通过 HTTP 检查，所以请保持关闭。

### CAA 记录 {#caa-records}

如果你的域名在 DNS 中有 CAA 记录，其中必须有一条允许 `letsencrypt.org`。否则 Let's Encrypt 会拒绝签发证书，日志会显示它的错误，其中会提到 CAA。见 [Let's Encrypt 和 CAA](https://letsencrypt.org/docs/caa/)。

### 速率限制 {#rate-limits}

Let's Encrypt 会限制一个域名和一个账户可以拥有的证书数量和验证失败次数。达到限制时，日志会显示 Let's Encrypt 的消息以及需要等待的时间。见 [Let's Encrypt 速率限制](https://letsencrypt.org/docs/rate-limits/)。

::: tip
如果想测试而不用掉限制次数，在 `/etc/nefo/config.json` 中添加 `"acme_directory": "https://acme-staging-v02.api.letsencrypt.org/directory"`，然后用 `systemctl restart nefo` 重启面板。测试环境（staging）的证书不受浏览器信任。删除这一行并再次重启，就能获取正式证书。
:::

### 通过 Cloudflare 的 DNS 验证 {#dns-validation-through-cloudflare}

| 提示 | 需要修复什么 |
|---|---|
| `DNS validation needs a Cloudflare API token: add one in Settings → Cloudflare` | 在 **设置** 页面的 **Cloudflare** 标签页上保存令牌。 |
| `Cloudflare did not accept this token.` | 令牌错误、已过期或未激活。创建一个新令牌。 |
| `none of the Cloudflare zones this API token can use holds example.com` | 在 Cloudflare 中把该域名的区域添加到令牌的区域资源中。 |
| `the DNS record _acme-challenge.example.com did not appear on … in time` | Cloudflare 的域名服务器太慢了。几分钟后再试。 |
