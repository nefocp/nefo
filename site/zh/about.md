---
title: 关于 Nefo
description: Nefo 是什么、适合谁、遵循的原则、当前状态和许可证，以及如何获得帮助或报告 bug。
---

# 关于 Nefo

Nefo 是一款免费、开源的控制面板，管理一台 Linux 服务器。你用一条命令把它安装在自己的 VPS 上，然后在浏览器中，用七种语言之一，管理网站、证书、数据库、应用、文件、备份和安全。它处于测试阶段（beta），采用 GNU AGPLv3 许可证，为 1 GB 内存的 VPS 设计。

它是单个程序 `/usr/local/bin/nefo`，Web 界面内置其中，以 systemd 服务 `nefo` 运行。它可以运行在 x86-64 和 ARM64 上的 Ubuntu 22.04、24.04 和 26.04，以及 Debian 12 和 13 上。它的应用商店有 42 个应用，界面支持英语、越南语、中文、西班牙语、葡萄牙语、俄语和印度尼西亚语。它能做的一切见 [Nefo 是什么？](./docs/)。

## 适合谁 {#who-it-is-for}

Nefo 适合自己运行 VPS 的人：为博客搭建第一台服务器的人、部署应用的开发者，或者照看客户网站的自由职业者和代理机构。

每台服务器只有一个面板账户 `admin`。Nefo 不是为出售主机服务而设计的：它没有代理商账户或客户账户。什么情况下其他面板更适合你，见 [Nefo 与其他面板](./compare)。

## 原则 {#principles}

- **轻量**。Nefo 为 1 GB 内存的 VPS 设计，面板本身约占用 {{ $release.ram }} 内存（在运行 v{{ $release.version }} 的测试服务器上测得）。在你选择之前，它什么都不安装，每个组件都可以再卸载；那些会让大型服务一直运行的功能（例如邮件服务器）被有意排除在外。
- **所有功能都免费**。没有付费版本，没有锁定的功能。不需要注册账户，也没有许可证密钥。
- **不会偷偷“回传”**。没有遥测，没有使用统计，没有许可证检查。界面不从其他网站加载任何内容。面板只会主动联系 Let's Encrypt 获取证书，以及项目的 GitHub 发布页面获取带签名的应用商店目录；它只在你要求时才检查更新。见 [隐私](./docs/#privacy)。
- **坦诚说明局限**。文档会说明 Nefo 不做什么、拒绝做什么以及原因。[路线图](./roadmap) 像列出计划中的事一样，坦白列出不打算做的事。

## 状态 {#status}

Nefo 处于测试阶段（beta）。第一个公开版本 v0.20.0 于 2026 年 11 月 26 日发布，当前版本是 v{{ $release.version }}（{{ $release.date }}）。见 [更新日志](./changelog)。

- 版本号以 `0.` 开头。在 1.0 版本之前，只有最新的发布版本会获得安全修复。
- 还没有做过外部安全审计。见 [安全策略](./security-policy)。
- 安装程序已在全新的 Ubuntu 22.04、24.04、26.04 和 Debian 12、13 服务器上测试过，平台为 x86-64。ARM64 还没有在真实的 ARM 硬件上测试过。

在把重要网站托付给 Nefo 之前，请先在可以重建的服务器上试用，并把备份副本保存在服务器之外。

## 许可证 {#licence}

Nefo 是遵循 [GNU AGPLv3](https://github.com/nefocp/nefo/blob/main/LICENSE) 的自由软件。你可以把它用于任何目的，在任意多台服务器上使用，阅读、修改和分享它的代码。如果你通过网络向其他人提供修改过的 Nefo，AGPL 要求你公开你的修改。

源代码在 [GitHub](https://github.com/nefocp/nefo) 上。

## 获取帮助和报告 bug {#get-help-and-report-bugs}

- **先阅读文档**。[故障排除](./docs/troubleshooting) 介绍常见问题的解决办法，[常见问题](./docs/faq) 回答常被问到的问题。
- 在 [GitHub Issues](https://github.com/nefocp/nefo/issues) 中 **报告 bug**。请附上你的版本（`nefo version`）、操作系统、导致问题的步骤、你预期的结果和实际发生的情况。如果是面板本身出错，请加上它日志的末尾：`journalctl -u nefo -n 50 --no-pager`。
- 你发布的任何内容中都 **不要包含机密信息**：面板的秘密路径、密码、API 密钥和不公开的域名。
- **私下报告安全问题**，永远不要发在公开的 issue 中。见 [安全策略](./security-policy#report-a-vulnerability)。

## 参与贡献 {#contribute}

- 按上面的方法 **报告 bug**。在 1.0 之前，Nefo 最需要的就是来自真实服务器的报告。
- **翻译面板**。把 `web/src/locales/en.json` 复制为以你的语言代码命名的文件，翻译其中的值（不是键），然后在 `web/src/i18n.js` 中注册该语言。
- **修复 issue 并提交 pull request**。请先阅读 [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md)。Nefo 以 root 身份运行在用户的服务器上，所以每项改动都会以安全为重点进行审查，那里列出的代码安全规则不是可选的。较大的改动请先开一个 issue，商定做法。
- **签署 CLA**。贡献者在提交第一个 pull request 时签署一次 [贡献者许可协议](https://github.com/nefocp/nefo/blob/main/CLA.md)。你保留你所贡献内容的版权。
