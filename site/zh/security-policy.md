---
title: 安全策略
description: 如何报告 Nefo 的漏洞，发布版本如何签名以及如何自己检查，以及当前的安全状态。
---

# 安全策略

Nefo 以 root 身份运行并管理整台服务器，所以每一份漏洞报告都会被当作紧急事项处理。本页说明如何报告漏洞、发布版本如何签名以及你如何检查，还有 Nefo 目前的状况。它与仓库中的 [SECURITY.md](https://github.com/nefocp/nefo/blob/main/SECURITY.md) 一致。

## 报告漏洞 {#report-a-vulnerability}

不要开公开的 issue、讨论或 pull request。请在 GitHub 上私下报告：

1. 打开 [Nefo 仓库](https://github.com/nefocp/nefo)，进入 **Security**（安全）标签页。
2. 选择 **Report a vulnerability**（报告漏洞），或使用 [直接链接](https://github.com/nefocp/nefo/security/advisories/new)。只有维护者能看到你的报告。

请附上：

- 受影响的版本（`nefo version`）和操作系统；
- 攻击者能做什么，以及他们事先需要什么权限：不需要、一个面板账户、一个 shell 账户……；
- 重现步骤，或概念验证；
- 你是否打算公开你的发现，以及何时公开。

### 之后会发生什么 {#what-happens-next}

| 步骤 | 目标时间 |
|---|---|
| 项目确认收到你的报告 | 3 个工作日内 |
| 项目告诉你是否确认这个问题，以及它的严重程度 | 7 天内 |
| 项目为严重或高危问题发布修复 | 确认后 14 天内 |
| 公开披露 | 修复发布之后，或最迟在你报告后 90 天，与你协调进行 |

每个确认的漏洞都会有一份带 CVE 的 GitHub 安全公告。你会被致谢，除非你希望保持匿名。

### 范围 {#scope}

范围内：`nefo` 程序、它的 API 和界面、安装程序，以及它在服务器上创建的配置。

范围外：

- 浏览器对面板第一次启动时创建的自签名证书发出的警告（这是正常的）；
- 需要服务器 root 权限或物理接触才能进行的攻击；
- 仅靠流量规模造成的拒绝服务；
- 面板安装的第三方软件（例如 Nginx、PHP 或 MariaDB）中的漏洞。请向这些项目自己报告，但如果面板对它们的配置不安全，请告诉我们。

### 安全港 {#safe-harbour}

只要你的安全研究遵循本策略，项目就不会对你采取法律行动：你只在自己拥有或获得测试许可的服务器上测试，避免侵犯隐私和破坏数据，并在披露之前给项目合理的时间修复问题。

## 受支持的版本 {#supported-versions}

在 1.0 版本之前，只有最新的发布版本会获得安全修复。请从面板（**设置** → **面板** → **检查更新**）或在服务器上用 `nefo update` 更新。见 [更新](./docs/update)。

## 发布版本如何签名 {#how-releases-are-signed}

GitHub Actions 根据仓库中的版本标签构建每个发布版本。一个发布版本包含：

- `nefo-linux-amd64` 和 `nefo-linux-arm64`，即程序；
- `install.sh`，即安装程序；
- `VERSION`；
- `checksums.txt`，两个程序和安装程序的 SHA-256 校验和；
- `checksums.txt.sig`，`checksums.txt` 的 Ed25519 签名；
- `appstore-catalog.json` 和 `appstore-catalog.json.sig`，应用商店目录及其单独的签名。

项目的公钥内置在每个 Nefo 程序中，并复制到了 `install.sh` 中。它也以 [`internal/update/release.pub`](https://github.com/nefocp/nefo/blob/main/internal/update/release.pub) 的形式放在仓库中：

```
-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEA2c6nGagX7yp8Y8YftuOqW2WVprKe6h0hv+3s9iB+KJA=
-----END PUBLIC KEY-----
```

有三个地方会在使用下载的文件之前检查签名：

- **安装程序** 检查 `checksums.txt` 的签名和程序的校验和。任何一项不对，它都会停止：“invalid signature: checksums.txt was not signed by the project. Do not install this file.”（签名无效：checksums.txt 不是由项目签名的。不要安装此文件。）
- **`nefo update`** 以及面板中的 **安装更新** 也会这样检查，使用的是你正在运行的程序中内置的密钥，然后在替换 `/usr/local/bin/nefo` 之前先运行一次新程序。
- **应用商店** 每天获取一次目录，除非签名有效，否则忽略它。

### 签名能证明什么，不能证明什么 {#what-the-signature-does-and-doesn-t-prove}

签名表明这些文件来自项目的发布构建，并且之后没有被改动过，无论是在 GitHub 上，还是在传到你服务器的途中。

- **更新** 用你已有的 Nefo 中的密钥检查，所以 GitHub 上被改动过的发布版本无法让自己被安装。
- **首次安装** 则不同：你下载的安装程序带着它用来检查的密钥。如果有人能改动 `install.sh` 本身，也就能改动那个密钥。要排除这种可能，请在运行 `install.sh` 之前按下面的方法自己检查它。
- 签名密钥作为 GitHub 仓库的机密保存，发布版本由 GitHub Actions 在那里签名。接管了项目 GitHub 账户的人可以为发布版本签名。

## 自行检查发布版本 {#check-a-release-yourself}

你需要 `curl`、`sha256sum` 和 OpenSSL 3（Ubuntu 22.04 及以后、Debian 12 及以后都有）。下面的例子以 root 身份在 amd64 服务器上检查最新的发布版本；把 `V` 设为另一个发布标签，就可以检查那个版本。

1. 下载校验和、它的签名和安装程序：

   ```sh
   V=$(curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/VERSION)
   cd /tmp
   for f in checksums.txt checksums.txt.sig install.sh; do
     curl -fsSLO "https://github.com/nefocp/nefo/releases/download/$V/$f"
   done
   ```

2. 保存本页上的公钥：

   ```sh
   cat > nefo-release.pub <<'EOF'
   -----BEGIN PUBLIC KEY-----
   MCowBQYDK2VwAyEA2c6nGagX7yp8Y8YftuOqW2WVprKe6h0hv+3s9iB+KJA=
   -----END PUBLIC KEY-----
   EOF
   ```

3. 检查签名，然后检查安装程序的校验和：

   ```sh
   openssl pkeyutl -verify -pubin -inkey nefo-release.pub -rawin \
     -in checksums.txt -sigfile checksums.txt.sig
   grep " install.sh$" checksums.txt | sha256sum -c -
   ```

   第一条命令必须打印 `Signature Verified Successfully`，第二条必须打印 `install.sh: OK`。任何一条失败，都不要运行安装程序，并按上面的方法报告。

4. 运行你检查过的安装程序。它会下载适合你 CPU 的程序，并用同一个密钥检查它：

   ```sh
   NEFO_VERSION=$V bash install.sh
   ```

要检查你自己下载的程序（例如 `nefo-linux-amd64`），在第 3 步之后运行 `grep " nefo-linux-amd64$" checksums.txt | sha256sum -c -`。

## Nefo 如何保护服务器 {#how-nefo-protects-the-server}

简要总结；详情见 [安全](./docs/security)：

- 面板只在随机端口上的秘密路径下响应。其他所有地址都返回 `404`。
- 它从第一次启动起就使用 HTTPS，TLS 1.2 或更新版本。
- 登录支持双因素认证。15 分钟内失败 5 次会让该地址被锁定 15 分钟。密码以 bcrypt 哈希值保存。
- 登录和更改都会写入活动日志，保留 90 天。
- 每个页面都有严格的内容安全策略，每项更改都需要一个其他网站无法发送的请求头。
- 项目的 [代码规则](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md#security-rules-for-code) 禁止用输入拼接 shell 命令：程序把每个值作为单独的参数接收，输入在用于文件路径、配置文件或命令之前，都要对照允许列表进行检查。你自己写的命令（例如定时任务和部署脚本）会按你写的样子在 shell 中运行。
- 会把你锁在外面的防火墙规则和 SSH 设置会被拒绝。
- 每个网站以自己的 Linux 用户运行，SFTP 账户被限制在网站的文件夹内。

## 当前状态 {#current-status}

Nefo 处于测试阶段（beta）。它的第一个公开版本是 2026 年 11 月发布的 v0.20.0；当前版本见 [更新日志](./changelog)。还没有做过外部安全审计。

在把重要网站托付给 Nefo 之前，请先在可以重建的服务器上试用，把备份副本保存在服务器之外，并让 Nefo 保持最新。来自项目之外的安全审查是 1.0 之前的步骤之一（见 [路线图](./roadmap)）。如果你审查了代码并发现了问题，请按上面的方法报告。
