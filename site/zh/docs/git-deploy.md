---
title: 从 Git 部署
description: 从 GitHub、GitLab、Gitea 或任何 Git 服务器部署网站，可以手动部署，也可以每次推送时部署，使用部署密钥、带签名的 Webhook 和部署脚本。
---

# 从 Git 部署

Nefo 可以从 GitHub、GitLab、Gitea、Forgejo 或任何 Git 服务器上的 Git 仓库部署网站，通过 HTTPS，或通过 SSH 加每个网站一个的部署密钥。你点击按钮时部署，或者通过带签名的 Webhook 在每次推送时部署，然后运行你的部署脚本。一切都以网站自己的 Linux 用户身份运行。

Git 部署位于每个网站的 **Git 部署** 标签页上，Docker 应用网站除外。对于 [文件被锁定](./website-protection#lock-a-website-s-files) 的网站，面板会在部署时解锁文件，部署完成后重新锁定。

## 开始之前 {#before-you-start}

- **仓库地址。** Nefo 接受三种形式：
  - `https://github.com/owner/repo.git`，用于公开仓库；
  - `git@github.com:owner/repo.git`（GitHub 显示的形式），用于私有仓库；
  - `ssh://git@git.example.com:2222/owner/repo.git`，用于其他端口上的 SSH 服务器。

  不接受普通的 `http://`，也不接受地址中包含密码或令牌，因为它们会出现在日志中。私有仓库请使用 SSH 地址和网站的部署密钥。
- **文件夹。** 选择文件放在哪里：
  - **Web 文件夹**（`/var/www/example.com/public`）：适用于仓库本身就是网站、顶层有 `index.html` 或 `index.php` 的情况。
  - **网站文件夹**（`/var/www/example.com`）：适用于有自己 `public` 文件夹的项目，例如 Laravel 或 Symfony。这时网站显示的是仓库的 `public` 文件夹。

  Node.js、Python 或 Java 应用网站没有选择：文件放进 **应用文件夹**（`/var/www/example.com/app`），它的程序在那里运行。请把 `package.json` 或 `requirements.txt` 放在仓库的顶层。
- **第一次部署需要空文件夹。** 面板的欢迎页面不算，网站文件夹中的 `logs`、`tmp` 和 `.ssh` 文件夹不算，应用文件夹中 Python 应用的 `.venv` 文件夹也不算。请先把其他文件移走，或者部署到另一个文件夹。
- **Git** 不需要事先安装：如果缺少，Nefo 会在第一次部署时安装它。

## 设置仓库 {#set-up-the-repository}

1. 打开 **网站**，然后打开该网站的 **Git 部署** 标签页。
2. 输入 **仓库地址**。
3. 输入 **分支**，例如 `main`。
4. 在 **文件放在** 下，选择 **Web 文件夹** 或 **网站文件夹**。应用网站则显示 **应用文件夹**，没有可选项。
5. 可选：编写 **部署脚本（可选）**（见 [部署脚本](#the-deploy-script)）。
6. 点击 **保存**。

第一次保存时，Nefo 会创建该网站的部署密钥和 Webhook 密钥，并提示“设置已保存。请把下方的部署密钥和 Webhook 添加到仓库，然后部署。”

## 添加部署密钥（私有仓库） {#add-the-deploy-key-private-repositories}

公开的 HTTPS 仓库不需要密钥。对于私有仓库，把 **部署密钥** 下显示的网站 **公钥** 作为只读部署密钥添加到仓库：

- **GitHub：** 仓库 → Settings → Deploy keys → Add deploy key。保持“Allow write access”（允许写入）关闭。
- **GitLab：** 项目 → Settings → Repository → Deploy keys，不给写入权限。
- **Gitea 和 Forgejo：** 仓库 → Settings → Deploy keys。

每个网站都有自己的 ed25519 密钥，以网站用户的身份创建在 `/var/www/example.com/.ssh/panel_deploy` 中。**新密钥** 会替换它：旧密钥立即失效，所以请把新密钥添加到仓库，并在那里删除旧密钥。

第一次 SSH 连接会接受 Git 服务器的主机密钥，并记录在 `/var/www/example.com/.ssh/known_hosts` 中。如果之后服务器的密钥变了，部署会失败，并提示“The Git server's SSH host key is not the one seen before.”（Git 服务器的 SSH 主机密钥与之前见到的不同。）如果服务器确实更换了密钥，删除那个文件，再部署一次。

## 部署 {#deploy}

在 **部署** 下，点击 **立即部署**。如果更改了设置，请先保存。日志会显示每一步：

1. Nefo 从仓库获取该分支。
2. 被跟踪的文件更新为仓库中该分支的版本。
3. 运行部署脚本（如果你写了）。

未被跟踪的文件会保留，所以上传的文件和仓库忽略的文件（例如 `.env`）在每次部署时都会保留。第一次部署之后，如果仓库已经在 `public` 中放入了网站，Nefo 会删除它的欢迎页面。

::: warning
你在服务器上对仓库跟踪的文件所做的更改，会在下一次部署时被覆盖。请把这些更改提交到仓库。
:::

部署之后，标签页会显示它的状态（**已部署** 或 **失败**）、时间、提交的前 12 个字符及其标题。获取最多可用 10 分钟，脚本最多 15 分钟，整个部署最多 30 分钟。

获取失败时，日志会解释常见的原因：

| 提示 | 怎么办 |
|---|---|
| `The repository refused the deploy key` | 把面板中显示的密钥添加到仓库。 |
| `The repository is private or does not exist` | 检查地址。私有仓库请使用它的 SSH 地址，并添加部署密钥。 |
| `The repository has no branch …` | 检查 **分支**。 |
| `The Git server's SSH host key is not the one seen before` | 见上面的部署密钥一节。 |
| `The repository's host name was not found` | 检查地址。 |
| `It took longer than 10 minutes` | 重试；检查仓库的大小和网络。 |

## 部署脚本 {#the-deploy-script}

脚本在每次部署之后运行，在部署文件夹中，以网站用户的身份运行，最多 15 分钟。它最多可以有 2000 个字符。例如，对于 Laravel 网站：

```sh
composer install --no-dev --optimize-autoloader
php artisan migrate --force
```

在应用网站上，脚本在 `app` 文件夹中运行，例如先 `npm ci` 再 `npm run build`。它的 `node` 和 `npm` 是为该网站选择的 Node.js 版本；在 Python 网站上，`python` 和 `pip` 是它的虚拟环境（`app/.venv`）中的版本。在 Java 网站上，`java`、`javac` 和 `JAVA_HOME` 是为它选择的 Java 版本，所以 `./mvnw package` 或 `./gradlew build` 会用这个版本构建。

- 它用 `sh -e` 运行：脚本在第一条失败的命令处停止，部署标记为 **失败**。此时新文件已经就位。
- `php` 是网站的 PHP 版本，Composer 也用它。
- `DEPLOY_COMMIT` 是部署的提交，`DEPLOY_BRANCH` 是分支。
- `HOME` 是网站的 `tmp` 文件夹。
- 脚本不能使用 `sudo`，也不能重启系统服务。要在部署后重启应用网站的程序，在网站的 **概览** 标签页上点击 **重启**。

## 每次推送时部署 {#deploy-on-every-push}

在 **推送时部署** 下，Nefo 会显示 **Webhook 地址**（例如 `https://example.com/.panel/deploy/12`）及其 **密钥**。Webhook 位于网站自己的域名上，所以仓库托管方永远不需要访问面板的端口。Web 服务器把它转交给面板在 `127.0.0.1:7809` 上的一个小型监听程序。

::: warning
没有 HTTPS 时，Webhook 地址使用 `http://`，GitLab 会以未加密的方式发送密钥。请先开启 HTTPS：见 [SSL 证书](./ssl)。
:::

每次推送到所选分支都会部署网站。推送到其他分支、推送标签、其他事件以及删除分支都会被忽略。

### GitHub {#github}

1. 打开仓库 → Settings → Webhooks → Add webhook。
2. 把 **Webhook 地址** 粘贴为 payload URL，把 **密钥** 粘贴为 secret。
3. 把 content type 设为 `application/json`。
4. 选择“Just the push event”（仅推送事件），然后添加 Webhook。

GitHub 会立即发送一次测试（ping），面板会响应它，所以第一次投递会显示成功。

### GitLab {#gitlab}

1. 打开项目 → Settings → Webhooks。
2. 把 **Webhook 地址** 粘贴为 URL，把 **密钥** 粘贴为 secret token。
3. 勾选 Push events，然后添加 Webhook。

### Gitea 和 Forgejo {#gitea-and-forgejo}

1. 打开仓库 → Settings → Webhooks → Add webhook，然后选择 Gitea（或 Forgejo）。
2. 把 **Webhook 地址** 粘贴为 target URL，把 **密钥** 粘贴为 secret。
3. 把 content type 设为 `application/json`，选择在 push 事件时触发，然后添加 Webhook。

Gogs 的做法相同：在第 1 步中选择 Gogs。

### 面板如何检查 Webhook {#how-the-panel-checks-a-webhook}

- GitHub、Gitea、Forgejo 和 Gogs 会用密钥为每个 Webhook 签名（HMAC-SHA256）。GitLab 则直接发送密钥本身。签名错误或缺失的 Webhook 会得到 HTTP 401，并连同发送方的地址一起记录下来。
- 网站的密码保护和 Web 防火墙不作用于 Webhook 路径，所以你不需要为它设置例外。
- Nefo 一次只运行一个任务。如果推送到达时有另一个任务正在运行，部署会等待，每 15 秒重试一次，最多一小时。期间的多次推送只需要一次部署：它会获取最新的提交。
- **新 Webhook 密钥** 会替换 Webhook 密钥。在你把新密钥粘贴到仓库的 Webhook 之前，推送都会被拒绝。

仓库托管方会在最近投递的列表中显示面板的响应：`deploying`、`queued: another task is running`、`ignored: not branch main`，或 `bad_signature` 之类的错误。

## 部署日志 {#the-deploy-log}

- **Git 部署** 标签页显示当前或上一次部署的日志，包括由推送启动的部署。
- **账户** → **最近活动** 会把每次部署列为“从 Git 部署了网站”或“Git 部署失败”，并附上网站、提交以及触发方式（`manual` 或来自仓库托管方的推送）。

## 停止从 Git 部署 {#stop-deploying-from-git}

在 **停止从 Git 部署** 下，点击 **停止从 Git 部署** 并确认。Nefo 会忘掉这些设置，并删除部署密钥。已部署的文件会和仓库的 `.git` 文件夹一起留在文件夹中。
