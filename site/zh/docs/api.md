---
title: API 密钥
description: 用 API 密钥编写脚本操作面板，用于 CI 部署、备份和监控，无需浏览器会话。密钥能做什么、不能做什么，如何调用 API，示例和错误。
---

# API 密钥

API 密钥让脚本无需浏览器就能使用 Nefo 面板，例如从 CI 部署网站、从另一台服务器的定时任务启动备份，或从监控工具读取网站的状态。密钥可以是只读的或完全访问的，可以限制为只能从你列出的地址使用，也可以设置到期。

脚本在每个请求中以 Bearer 令牌的形式发送密钥，请求的地址和秘密路径与面板相同。

密钥位于 **设置** → **API 密钥**。

## 密钥能做什么 {#what-a-key-can-do}

每个密钥有以下两种访问权限之一：

- **只读**：只能发 `GET` 和 `HEAD` 请求。它可以列出网站、数据库和备份，读取任务日志、统计数据和服务器状态，但不能更改任何东西。它也不能读取会让它获得更多权力的内容：数据库和应用的密码、数据库导出、文件（文件管理）、备份下载、Redis 的值以及 Git Webhook 密钥。这些请求，以及 `GET` 或 `HEAD` 以外的任何请求，都会得到带 `api_key_read_only` 的 `403`。
- **完全访问**：可以以你的名义发出面板自己的页面会发出的每个请求。

即使是完全访问的密钥也不能：

- 创建、列出或吊销 API 密钥；
- 修改密码或双因素登录，或退出登录（`/api/auth/` 下的一切）；
- 打开 Web 终端或容器的终端；
- 使用位于面板登录之后的数据库工具 Adminer 和 phpMyAdmin。

这些请求会得到带 `api_key_forbidden` 的 `403`：请登录面板来完成它们。密钥也不能用来登录以获取会话。

::: warning
完全访问的密钥和你本人一样强大：它可以写文件、添加以 root 运行的定时任务、安装软件。请像 root 密码一样保密它。只读密钥同样是机密：它可以读取你的网站和服务器的设置、日志和列表。
:::

## 创建密钥 {#create-a-key}

1. 前往 **设置** → **API 密钥**，点击 **新建密钥**。
2. 输入 **名称**，例如 `deploy-bot`：最多 32 个字母、数字、空格、点、连字符和下划线。名称会显示在活动日志中。
3. 选择 **只读** 或 **完全访问**。
4. 可选：在 **允许的地址（可选）** 下，输入可以使用该密钥的 IP 地址或网段，例如 `203.0.113.7` 或 `198.51.100.0/24`，IPv4 或 IPv6 均可，用空格或换行分隔。留空表示允许任何地址。
5. 选择何时 **到期**：30、90 或 365 天后，或 **永不**。
6. 点击 **新建密钥**。

面板只显示一次密钥，旁边有 **复制** 按钮：请立即复制，之后就看不到了。密钥的样子是 `nefo_` 后接 43 个随机字符。面板只保存它的 SHA-256 哈希值。

列表显示每个密钥的名称和开头几个字符（例如 `nefo_Ab12cd…`）、访问权限、允许的地址、上次使用的时间和来源（最多每分钟更新一次），以及到期时间。最多可以有 20 个密钥。要吊销密钥，点击它的垃圾桶图标并确认：之后使用它的下一个请求就会失败。

## 调用 API {#call-the-api}

在 `Authorization` 请求头中以 Bearer 令牌的形式发送密钥。地址是面板地址加秘密路径，再加 `api/` 和路由：

```sh
export NEFO_KEY='nefo_…'
export NEFO='https://panel.example.com:PORT/<entrance>'

curl -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/sites"
```

- 只从 `Authorization` 请求头读取密钥：从不从地址（`?key=`）或 Cookie 中读取。
- 带 JSON 请求体的请求需要 `Content-Type: application/json`。未知字段会被拒绝。
- 使用密钥的请求不需要面板自己的页面发送的 `X-Requested-With` 请求头。
- 请通过 HTTPS 和正式证书使用面板（**设置** → **证书**，见 [安全](./security#https-from-the-first-start)）。避免使用 `curl -k`：它会把密钥发给任何拦在中间的人。

长时间的任务（例如备份、部署和证书）在后台运行。它们的请求会返回 `202` 和一个任务：

```json
{ "id": "a1b2c3", "kind": "backup", "target": "example.com", "status": "running", "log": [], "started": "…" }
```

反复请求 `GET /api/jobs/{id}`，直到它的 `status` 变为 `succeeded` 或 `failed`；`log` 包含它的日志行，`error` 说明失败原因。面板一次只运行一个任务：另一个任务运行期间，新任务会返回带 `busy` 的 `409`。

## 示例 {#examples}

路由就是面板页面使用的路由。把 `3` 换成你网站的 `id`。

### 列出网站 {#list-the-websites}

```sh
curl -s -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/sites"
```

返回 `{"sites": [{"id": 3, "domain": "example.com", "ssl": true, …}]}`。只读密钥就够了。

### 备份网站 {#back-up-a-website}

```sh
curl -s -X POST -H "Authorization: Bearer $NEFO_KEY" -H "Content-Type: application/json" \
  -d '{"kind": "site", "target": "example.com", "remote": true}' \
  "$NEFO/api/backups"
```

- `kind` 可以是 `site`（`target` 为域名）、`database`（数据库名称）、`folder`（绝对路径）或 `app`（应用商店应用的名称）。
- `remote: true` 表示同时把备份上传到远程存储（如果已设置，见 [备份](./backups#remote-storage)）。

它返回 `202` 和任务。要等待它完成：

```sh
JOB=$(curl -s -X POST -H "Authorization: Bearer $NEFO_KEY" -H "Content-Type: application/json" \
  -d '{"kind": "database", "target": "shop", "remote": false}' "$NEFO/api/backups" | jq -r .id)
while [ "$(curl -s -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/jobs/$JOB" | jq -r .status)" = running ]; do
  sleep 5
done
```

### 从 Git 部署 {#deploy-from-git}

先在网站的 **Git 部署** 标签页上设置仓库（见 [从 Git 部署](./git-deploy)）。然后，在测试通过后从你的 CI 中运行：

```sh
curl -fsS -X POST -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/sites/3/git/deploy"
```

它不需要请求体，返回 `202` 和任务。在 GitHub Actions 工作流中，把密钥和地址保存在仓库的机密中：

```yaml
- name: Deploy
  run: curl -fsS -X POST -H "Authorization: Bearer ${{ secrets.NEFO_KEY }}" "${{ secrets.NEFO_URL }}/api/sites/3/git/deploy"
```

### 安装证书 {#install-a-certificate}

从 Let's Encrypt 获取免费证书（见 [SSL 证书](./ssl)）：

```sh
curl -s -X POST -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/sites/3/ssl"
```

它返回 `202` 和任务。如果要改为通过 Cloudflare DNS 验证（通配符名称需要这样做），发送 `-H "Content-Type: application/json" -d '{"dns": true}'`。

要安装你自己的证书（例如在另一台服务器上续期的证书），发送 PEM 格式的证书及其证书链和私钥，总共最多 256 KB。`jq` 可以根据文件生成 JSON：

```sh
jq -n --rawfile cert fullchain.pem --rawfile key privkey.pem '{cert: $cert, key: $key}' |
  curl -s -X POST -H "Authorization: Bearer $NEFO_KEY" -H "Content-Type: application/json" \
    --data-binary @- "$NEFO/api/sites/3/certificate"
```

它返回 `200`，附带网站和一个 `warnings` 列表；或者返回 `400` 和原因，例如私钥与证书不匹配（见 [使用你自己的证书](./ssl#use-your-own-certificate)）。

## 错误 {#errors}

错误是 JSON 格式，例如 `{"error": "invalid_api_key"}`，有些带有 `detail`。

| 状态码 | `error` | 含义 |
|---|---|---|
| 401 | `invalid_api_key` | 密钥未知、已吊销或已过期，或者从它不允许的地址使用。响应不会说明是哪一种；活动日志会说明。 |
| 403 | `api_key_read_only` | 只读密钥发送了 `GET` 或 `HEAD` 以外的请求，或者请求了密码、文件、导出或备份下载。 |
| 403 | `api_key_forbidden` | 只有已登录的会话才能做这件事（见 [密钥能做什么](#what-a-key-can-do)）。 |
| 429 | `too_many_attempts` | 这个地址失败的尝试太多。等待 `retry_after` 秒，`Retry-After` 请求头中也有这个值。 |
| 400 | 例如 `bad_request` | 请求有误；`detail` 可能会说明原因。 |
| 404 | `not_found` | 该项目不存在。秘密路径之外的路径得到的是不带内容的 404。 |
| 409 | `busy` | 另一个任务正在运行。等它完成后再试。 |

## 失败的尝试和 fail2ban {#failed-attempts-and-fail2ban}

使用密钥的失败尝试与登录失败同样计数，并计入同一个计数（见 [登录限制](./security#sign-in-limits)）：15 分钟内失败 5 次，该地址会被锁定 15 分钟，密钥和登录页面都一样。有效的密钥不会重置计数。

每次失败都会以“登录失败”记录在 **账户** → **最近活动** 中，并附上原因：`unknown API key`、`API key expired` 或 `API key not allowed from this address`。fail2ban 的面板 jail 会读取这些记录，所以它会封禁不停猜测的地址（见 [防暴力破解](./security#brute-force-protection-fail2ban)）。

## 活动日志 {#the-activity-log}

- 用密钥做的每项更改（`GET` 和 `HEAD` 以外的每个请求）都会记录为“通过 API 密钥更改”，并附上方法和路径，例如 `POST /api/sites/3/git/deploy`。
- 这些条目的用户是你的用户名加上密钥的名称，例如 `admin (key deploy-bot)`。请求所做事情的条目也是如此，例如“从 Git 部署了网站”。
- 对密钥拒绝的请求记录为“API 密钥被拒绝”；创建和吊销密钥记录为“创建了 API 密钥”和“吊销了 API 密钥”。
- 读取操作不记录。密钥列表会显示每个密钥上次使用的时间和来源。

密钥本身永远不会写入任何日志。

## 安全建议 {#security-advice}

- **尽量使用只读密钥。** 监控和报告只需要 `GET`。
- **每个脚本一个密钥。** 它的名称会显示在日志中，而且你可以吊销它而不影响其他脚本。
- **只允许需要的地址**，例如你的 CI 运行器或备份服务器（如果它们有固定地址）。
- **让密钥到期，并定期轮换**：创建新密钥，放进脚本，然后吊销旧密钥。
- **不要把密钥放进仓库和地址中。** 使用 CI 的机密存储或只有属主能读取的文件，并在命令中使用变量，这样密钥不会出现在 shell 历史记录中。
- **密钥可能泄露时立即吊销它**，然后在 **最近活动** 中检查它做了什么。
- 如果 [防火墙](./security#firewall-ufw) 把面板端口限制为只对部分地址开放，也要在那里放行脚本的地址。
