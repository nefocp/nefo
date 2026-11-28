---
title: DNS（Cloudflare）
description: 在面板中管理 Cloudflare 区域的 DNS 记录，把网站的名称指向这台服务器，添加你的邮件服务商的 MX、SPF 和 DMARC 记录，以及清除 Cloudflare 缓存。
---

# DNS（Cloudflare）

Nefo 不运行 DNS 服务器。如果你的域名 DNS 托管在 Cloudflare，Nefo 可以用 API 令牌通过 Cloudflare 的 API 管理它的记录：添加、修改和删除记录，在预览之后把网站的名称指向这台服务器，添加你的邮件服务商的 MX、SPF 和 DMARC 记录，以及清除 Cloudflare 的缓存。

令牌保存在 **设置** → **Cloudflare** 中。之后：

- 左侧菜单 **托管** 分组中的 **DNS** 页面会列出你的区域及其记录，可以添加、修改和删除记录，以及清除 Cloudflare 的缓存；
- 网站 **域名** 标签页上的 **将 DNS 指向此处…** 一步就能把网站的名称指向这台服务器；
- DNS 页面上的 **设置邮件** 会添加邮件服务商的 MX、SPF 和 DMARC 记录；
- 通配符证书也通过同一个令牌验证：见 [SSL 证书](./ssl#wildcard-certificates-through-cloudflare-dns)。

Nefo 只与 Cloudflare 的 API 通信，令牌永远不会出现在面板的日志或消息中。

## API 令牌 {#the-api-token}

一个令牌就能完成所有这些。对于你在这台服务器上使用的区域（域名），它需要以下权限：

| 权限 | 用途 |
|---|---|
| Zone → DNS → Edit | 列出、添加、修改和删除记录，**将 DNS 指向此处…**，以及证书的 DNS 验证。 |
| Zone → Zone → Read | 列出区域及其状态和套餐。没有这个权限时，区域列表可能一直为空，或者 Cloudflare 拒绝列出区域。 |
| Zone → Cache Purge → Purge | DNS 页面上的 **清除缓存**。 |

1. 在 Cloudflare 控制台中，打开 **My Profile → API Tokens → Create Token**（我的个人资料 → API 令牌 → 创建令牌）。
2. 从 **Edit zone DNS**（编辑区域 DNS）模板开始。它提供 Zone → DNS → Edit。
3. 点击 **+ Add more**（+ 添加更多），添加 Zone → Zone → Read 和 Zone → Cache Purge → Purge。
4. 在 **Zone Resources**（区域资源）下，包含你在这里使用的区域，或者账户的所有区域。
5. 创建令牌，复制它，然后保存到 Nefo 中：**设置** → **Cloudflare** → **API 令牌** → **检查并保存**。

令牌缺少某个权限时，Nefo 会指出是哪个，例如：“Cloudflare 拒绝了请求：API 令牌缺少此区域的 Zone → DNS → Edit 权限。”在 Cloudflare 中编辑令牌（**My Profile → API Tokens**，然后点击令牌旁边的 **Edit**（编辑）），为该区域添加权限并保存。编辑令牌不会改变它的值，所以 Nefo 中不需要改动任何东西。如果你删除或轮换了令牌，请在 **设置** → **Cloudflare** 中保存新令牌。

## DNS 页面 {#the-dns-page}

页面一次显示一个区域。有多个区域时，在顶部的列表中选择一个；它的状态（**已激活**，或者在域名的域名服务器还不是 Cloudflare 的时为 **待处理**）和套餐显示在旁边。

表格列出区域的记录：**类型**、**名称**（`@` 是区域本身，`www` 是 `www.example.com`）、**内容**、**代理** 和 **TTL**。可以按类型筛选，或搜索名称和内容。记录超过 5,000 条的区域只显示前 5,000 条；其余的请使用 Cloudflare 控制台。

Nefo 可以添加和修改 A、AAAA、CNAME、TXT、MX 和 CAA 记录。其他类型（例如 NS 或 SRV）会列出并标为“在 Cloudflare 中”：请在 Cloudflare 控制台中修改它们。

### 添加或修改记录 {#add-or-change-a-record}

点击 **添加记录**，或在记录所在的行点击 **编辑**。记录的类型不能更改：请删除它再添加一条新记录。Nefo 会在发送到 Cloudflare 之前检查记录：

| 类型 | 输入什么 |
|---|---|
| A | 一个 IPv4 地址，例如 `203.0.113.10`。 |
| AAAA | 一个 IPv6 地址，例如 `2001:db8::10`。 |
| CNAME | 这个名称作为其别名的主机名，例如 `example.com`。它不能指向自己的名称。 |
| TXT | 一行文本，最多 2,048 个字符：普通文本（Cloudflare 会把它拆分为 255 个字符的字符串），或每个最多 255 个字符的 `"quoted strings"`。要么整个值都加引号，要么都不加。 |
| MX | 邮件服务器的主机名，例如 `mail.example.com`，以及 0 到 65535 的 **优先级**（数值小的先尝试）。 |
| CAA | 一个 **标签**：`issue` 或 `issuewild` 加上 CA 的域名，例如 `letsencrypt.org`（`;` 表示不允许任何 CA），或 `iodef` 加上 `mailto:` 或 `https://` 地址。**标志** 为 0，或表示关键的 128。 |

**TTL** 为 **自动**（由 Cloudflare 选择）或 1 分钟到 1 天之间的固定时间。

### 已代理或仅 DNS {#proxied-or-dns-only}

A、AAAA 和 CNAME 记录可以设为 **已代理**（Cloudflare 的橙色云朵）：访客先到达 Cloudflare，由它把 Web 请求转发到该地址，并隐藏这个地址。已代理的记录总是使用自动 TTL。**仅 DNS**（灰色云朵）直接返回地址本身。其他记录类型始终是仅 DNS。

Cloudflare 的代理只转发 80 和 443 端口（以及少数其他端口）上的 HTTP 和 HTTPS。其他一切都请保持仅 DNS：SSH、邮件和面板。

::: warning 面板的名称请保持仅 DNS
如果你通过 `panel.example.com` 这样的名称打开面板，请让这条记录保持 **仅 DNS**。Cloudflare 的代理不转发面板的端口，所以开启橙色云朵后，就无法再通过这个名称打开面板。记录被代理时，DNS 页面和 **将 DNS 指向此处…** 都会提醒你。
:::

## 把网站指向这台服务器 {#point-a-website-to-this-server}

1. 打开 **网站**，然后打开该网站的 **域名** 标签页。
2. 在 **Cloudflare 上的 DNS** 下，点击 **将 DNS 指向此处…**。
3. 检查地址和选项：
   - **本服务器的公网 IPv4 地址**，从服务器的网络接口读取；
   - **同时发布 IPv6 地址（AAAA）**，服务器有公网 IPv6 地址时会勾选；
   - **通过 Cloudflare 代理（橙色云朵）**。
4. 阅读预览：每个名称和类型的记录在 **当前** 和 **之后** 的值，标为 **新增**、**已更改**、**已删除** 或 **无变化**。
5. 点击 **应用**。

Nefo 只更改网站名称（它的域名和额外域名）的记录；其他名称的记录永远不会被碰。对于每个名称：

- 一条 A 记录指向这台服务器：更新已有的 A 记录或新建一条，该名称的其他 A 记录会被删除；
- 选择 IPv6 时，它的 AAAA 记录也会这样处理。不选择时，指向其他服务器的 AAAA 记录会被删除，因为 IPv6 访客和 Let's Encrypt 会到达那台服务器；
- CNAME 记录会被替换为 A 记录，除非它指向网站的另一个名称（`www` → `example.com` 保持不变）；
- 不在令牌能看到的任何区域中的名称不会被动，预览会列出它们。

你点击 **应用** 时，Nefo 会重新计算更改。如果预览之后 Cloudflare 上的记录发生了变化，就什么都不改，并显示新的预览：“预览之后 Cloudflare 上的记录已发生变化。”

如果服务器位于 NAT 之后，只有私有 IPv4 地址，Nefo 会提示“在这台服务器的网络接口上找不到公网 IPv4 地址”。请输入能到达服务器的公网地址。Nefo 不会向任何外部服务查询它。

### 橙色云朵后面的 Let's Encrypt {#let-s-encrypt-behind-the-orange-cloud}

Let's Encrypt 通过普通 HTTP 检查网站。开启代理时，Cloudflare 的 **Always Use HTTPS**（始终使用 HTTPS）设置（**SSL/TLS → Edge Certificates**（SSL/TLS → 边缘证书））会在检查请求到达服务器之前就用重定向回应它，于是证书申请失败。你可以：

- 在 Cloudflare 中关闭 **Always Use HTTPS**，改为在 Nefo 中开启 [将 HTTP 重定向到 HTTPS](./ssl#redirect-http-to-https)；或者
- 在网站的 **SSL / HTTPS** 标签页上勾选 **通过 Cloudflare DNS 验证**，Cloudflare 的设置保持不变。

网站有了证书后，把 Cloudflare 的 **SSL/TLS** 加密模式设为 **Full (strict)**（完全（严格））。如果用 **Flexible**（灵活），Cloudflare 会通过 HTTP 访问服务器，而 **将 HTTP 重定向到 HTTPS** 又把它送回 HTTPS，形成循环。

## 如何为邮件设置 DNS 记录？ {#set-up-email-records}

Nefo 不托管邮箱（见 [邮件与告警](./mail-alerts)）。如果你的邮件由 Google Workspace、Microsoft 365、Zoho Mail 或其他服务商托管，或由 Cloudflare Email Routing 转发，Nefo 可以添加服务商需要的 DNS 记录：

| 记录 | 作用 |
|---|---|
| MX | 说明哪些服务器接收该域名的邮件。 |
| SPF（一条 TXT 记录） | 说明哪些服务器可以代表该域名发送邮件。 |
| DMARC（`_dmarc` 处的一条 TXT 记录） | 告诉收信服务器如何处理未通过 SPF 和 DKIM 检查的邮件，以及把报告发到哪里。 |
| DKIM（一条 TXT 或 CNAME 记录） | 公布你的服务商用来给邮件签名的密钥。 |

在你看过更改列表并应用之前，什么都不会改变。

1. 打开 **DNS**，选择区域。
2. 点击 **设置邮件**。
3. 在 **邮件服务商** 下，选择 **Google Workspace**、**Microsoft 365**、**Zoho Mail**、**Cloudflare Email Routing** 或 **其他服务商**。
4. 可选：在 **邮件域名** 中输入区域中的一个名称，例如 `mail.example.com` 就输入 `mail`。留空表示区域本身。
5. 可选：输入 **DMARC 报告地址（可选）**；如果已经有 DKIM 记录，也可以粘贴进来（见下文）。
6. 点击 **显示更改**。表格列出每条记录的 **当前** 和 **之后** 的值，以及它会怎样：新增、已更改、已删除、无变化或其他服务商。
7. 点击应用按钮，例如 **应用 4 项更改**。

如果在预览后更改了某个字段，请在应用之前点击 **重新预览**。如果在此期间 Cloudflare 上的记录发生了变化，Nefo 会拒绝，并显示新的预览。

### 每个服务商会得到什么 {#what-each-provider-gets}

| 服务商 | MX 记录 | SPF include |
|---|---|---|
| **Google Workspace** | `smtp.google.com`，优先级 1 | `include:_spf.google.com` |
| **Microsoft 365** | `example-com.mail.protection.outlook.com`（你的域名，点换成短横线），优先级 0 | `include:spf.protection.outlook.com` |
| **Zoho Mail** | `mx.zoho.com`（10）、`mx2.zoho.com`（20）、`mx3.zoho.com`（50），使用所选 **Zoho 数据中心** 的域名：zoho.com、zoho.eu、zoho.in 或 zoho.com.au | `include:zohomail.com`，或 `zohomail.eu`、`zohomail.in`、`zohomail.com.au` |
| **Cloudflare Email Routing** | `route1.mx.cloudflare.net`、`route2.mx.cloudflare.net`、`route3.mx.cloudflare.net` | `include:_spf.mx.cloudflare.net` |
| **其他服务商** | 你在 **邮件服务器（MX）** 下输入的服务器，最多 5 个，每个带一个优先级 | **SPF include（可选）** 中的域名和 **发信地址（可选）** 中的地址；两者都没有时为 `mx` |

一些细节：

- **Google Workspace**：如果区域中已经有 Google 较旧的 MX 记录（`aspmx.l.google.com` 等），Nefo 会保留它们，不再添加任何东西。
- **Microsoft 365**：Microsoft 为每个域名提供自己的 MX 主机。请把预览中的值与 Microsoft 365 管理中心为该域名显示的值进行比较。如果不同，选择 **其他服务商** 并输入它。已经指向 Microsoft 365 的 MX 记录保持不变。
- **Zoho Mail**：选择你登录地址所在的数据中心。如果是其他 Zoho 数据中心，选择 **其他服务商**，并输入 Zoho 显示的值。
- **Cloudflare Email Routing**：只有在 Cloudflare 控制台中为该区域开启 Email Routing 并添加目标地址之后，邮件才会到达。已有的 Cloudflare 路由记录保留 Cloudflare 给它们的优先级。

### SPF：始终只有一条记录 {#spf-always-one-record}

一个域名必须正好有一条 SPF 记录；有两条会让 SPF 检查失败。Nefo 从不添加第二条：

- 如果区域中已有 SPF 记录，Nefo 会保留其中的所有内容，加上服务商的 include，并保留它的结尾（`-all`、`~all` 或 `?all`）。没有结尾的记录会加上 `~all`。
- 如果区域中有多条 SPF 记录，Nefo 会把它们合并为一条：修改第一条，删除其他的。
- 如果你在 **设置** → **邮件** 中设置了 SMTP 中继，SPF 记录还会允许中继的服务商（例如 SendGrid 的 `include:sendgrid.net`、Mailgun 的 `include:mailgun.org`、Amazon SES 的 `include:amazonses.com`），这样网站发出的邮件能通过检查。预览会说明这一点。
- 需要超过 10 次 DNS 查询（`include`、`a`、`mx`、`ptr`、`exists`、`redirect`）的 SPF 记录在很多收信方会失败。预览会提醒你；请删除不再使用的 include。

### 其他服务商的 MX 记录 {#mx-records-of-another-provider}

指向其他服务商的 MX 记录会标为“其他服务商”。邮件会同时发往两个服务商，所以它们必须去掉：勾选 **替换现有的 MX 记录** 来删除它们。在你勾选或自己删除这些记录之前，Nefo 会拒绝应用更改。

::: warning
一旦删除旧服务商的 MX 记录，邮件就不再发往旧服务商。请先迁移你的邮箱。
:::

### DMARC {#dmarc}

Nefo 在 `_dmarc` 处添加 `v=DMARC1; p=none`；如果你输入了 **DMARC 报告地址（可选）**，还会加上 `rua=mailto:` 和你的地址。使用 `p=none` 时，不会拒收任何邮件：收信服务器只发送报告。当报告显示你的所有邮件都能通过时，你可以在 DNS 页面上编辑这条记录，让策略更严格（`p=quarantine`，然后 `p=reject`）。

如果区域中已有 DMARC 记录，Nefo 会保持它不变。

### DKIM {#dkim}

DKIM 密钥由邮件服务商生成，所以 Nefo 无法创建它。在 **DKIM 签名** 下，对话框会说明在服务商那里去哪里找到它：

- **Google Workspace**：在 Google Admin console 中，打开 Apps → Google Workspace → Gmail → Authenticate email，生成一条新记录。它的名称是 `google._domainkey`，类型为 TXT。
- **Microsoft 365**：在 Microsoft Defender portal 中，打开该域名的 DKIM 设置。Microsoft 会给出两条 CNAME 记录，`selector1._domainkey` 和 `selector2._domainkey`；对话框中每条都有一行。
- **Zoho Mail**：在 Zoho Mail Admin Console 中，打开该域名的 DKIM 设置并添加一个选择器。输入选择器的名称后接 `._domainkey`，以及 TXT 值。
- **Cloudflare Email Routing** 不需要在这里添加 DKIM 记录：开启 Email Routing 时，Cloudflare 会添加它需要的内容。
- **其他服务商**：请向服务商索取它的 DKIM 记录。

粘贴 **记录名称** 和 **值**，选择 **类型**（TXT 或 CNAME），然后预览。`google._domainkey` 这样的名称位于邮件域名之下；也接受完整的名称。同名的 DKIM 记录会被替换。值留空可以稍后再添加 DKIM 记录：拿到后，用同一个服务商再次运行 **设置邮件**。然后在服务商那里完成设置（Google：**Start authentication**（开始身份验证）；Microsoft 和 Zoho：开启或验证 DKIM）。

再次运行 **设置邮件** 是安全的：已经正确的记录会显示为“无变化”。

Nefo 添加、修改或删除的每条记录都会写入活动日志，与其他 DNS 更改一样。

## 清除 Cloudflare 缓存 {#purge-cloudflare-s-cache}

开启代理时，Cloudflare 会保存你文件的副本。修改文件后，在 DNS 页面上点击 **清除缓存**：

- **全部** 清空 Cloudflare 对整个区域的缓存。之后 Cloudflare 会重新从服务器获取每个文件，所以网站可能会慢几分钟；
- **这些 URL** 一次清除区域中最多 30 个地址，每行一个，例如 `https://example.com/style.css`。

这是 Cloudflare 的缓存，不是这台服务器上网站自己的 [页面缓存](./websites#page-cache)。

## 活动日志 {#activity-log}

每项更改都会记录在活动日志（**账户** → **最近活动**）中：“添加了 DNS 记录”“修改了 DNS 记录”“删除了 DNS 记录”和“清除了 Cloudflare 缓存”，并附上记录的类型、名称和内容。**将 DNS 指向此处…** 会记录它更改的每条记录。

## 提示信息 {#messages}

| 提示 | 怎么办 |
|---|---|
| `No Cloudflare API token is saved.` | 在 **设置** → **Cloudflare** 中保存令牌。 |
| `Cloudflare refused: the API token lacks the permission …` | 把它指出的权限添加到令牌中：见 [API 令牌](#the-api-token)。 |
| `Cloudflare did not accept this token.` | 令牌错误、已过期或已删除。创建一个新令牌。 |
| `None of these names is in a Cloudflare zone the API token can see` | 把该区域添加到令牌的 **Zone Resources** 中，或者在托管该域名 DNS 的地方设置记录。 |
| `Cloudflare does not answer for example.com yet (pending).` | 在你的域名注册商处，把域名的域名服务器改为 Cloudflare 显示的那两个。 |
| `Cloudflare refused: …` | Cloudflare 自己的消息，例如 CNAME 记录和 A 记录要共用同一个名称时。 |
