---
title: Deploy from Git
description: Deploy a website from GitHub, GitLab, Gitea or any Git server, by hand or on every push, with a deploy key, a signed webhook and a deploy script.
---

# Deploy from Git

Nefo can deploy a website from a Git repository on GitHub, GitLab, Gitea, Forgejo or any Git server, over HTTPS or SSH. It deploys when you click a button, or on every push through a signed webhook, and it can run a deploy script afterwards. Everything runs as the website's own Linux user.

Git deploy is on the **Git deploy** tab of each website, except Docker app websites.

## Before you start

- **The repository address.** Nefo accepts three forms:
  - `https://github.com/owner/repo.git` for a public repository;
  - `git@github.com:owner/repo.git` (the form GitHub shows) for a private one;
  - `ssh://git@git.example.com:2222/owner/repo.git` for an SSH server on another port.

  Plain `http://` is refused, and so is a password or token in the address, because it would show in logs. For a private repository, use the SSH address and the website's deploy key.
- **The folder.** Choose where the files go:
  - **The web folder** (`/var/www/example.com/public`): for a website that is the repository itself, with `index.html` or `index.php` at the top.
  - **The site folder** (`/var/www/example.com`): for projects with their own `public` folder, such as Laravel or Symfony. The website then shows the repository's `public` folder.

  A Node.js or Python app website has no choice: the files go into **The app folder** (`/var/www/example.com/app`), where its program runs. Put `package.json` or `requirements.txt` at the top of the repository.
- **An empty folder for the first deploy.** The panel's placeholder page doesn't count, nor do the `logs`, `tmp` and `.ssh` folders in the site folder, nor a Python app's `.venv` folder in the app folder. Move other files away first, or deploy into the other folder.
- **Git** doesn't need to be installed: Nefo installs it on the first deploy if it is missing.

## Set up the repository

1. Open **Websites**, then the website's **Git deploy** tab.
2. Enter the **Repository address**.
3. Enter the **Branch**, for example `main`.
4. Under **Put the files in**, choose **The web folder** or **The site folder**. An app website shows **The app folder** instead, with nothing to choose.
5. Optional: write a **Deploy script (optional)** (see [The deploy script](#the-deploy-script)).
6. Click **Save**.

The first time, Nefo creates the website's deploy key and webhook secret and says "Settings saved. Add the deploy key and the webhook below to the repository, then deploy."

## Add the deploy key (private repositories)

A public HTTPS repository needs no key. For a private repository, add the website's **Public key**, shown under **Deploy key**, to the repository as a read-only deploy key:

- **GitHub:** repository → Settings → Deploy keys → Add deploy key. Leave "Allow write access" off.
- **GitLab:** project → Settings → Repository → Deploy keys, without write access.
- **Gitea and Forgejo:** repository → Settings → Deploy keys.

Each website has its own ed25519 key, created as the website's user in `/var/www/example.com/.ssh/panel_deploy`. **New key** replaces it: the old key stops working at once, so add the new one to the repository and remove the old one there.

The first SSH connection accepts the Git server's host key and remembers it in `/var/www/example.com/.ssh/known_hosts`. If the server's key changes later, deploys fail with "The Git server's SSH host key is not the one seen before." If the server really changed its key, delete that file and deploy again.

## Deploy

Under **Deploy**, click **Deploy now**. If you changed the settings, save them first. A log shows each step:

1. Nefo fetches the branch from the repository.
2. The tracked files take the repository's version of the branch.
3. The deploy script runs, if you wrote one.

Untracked files stay, so uploads and files that the repository ignores (such as `.env`) are kept on each deploy. After the first deploy, Nefo removes its placeholder page when the repository has put a website in `public`.

::: warning
Changes you make on the server to files that the repository tracks are overwritten on the next deploy. Commit them to the repository instead.
:::

After a deploy, the tab shows its status (**Deployed** or **Failed**), the time, the first 12 characters of the commit and its subject. A fetch may take up to 10 minutes, the script up to 15 minutes and the whole deploy up to 30 minutes.

When a fetch fails, the log explains the usual reasons:

| Message | What to do |
|---|---|
| The repository refused the deploy key | Add the key shown in the panel to the repository. |
| The repository is private or does not exist | Check the address. For a private repository, use its SSH address and add the deploy key. |
| The repository has no branch … | Check the **Branch**. |
| The Git server's SSH host key is not the one seen before | See the deploy key section above. |
| The repository's host name was not found | Check the address. |
| It took longer than 10 minutes | Try again; check the repository's size and the network. |

## The deploy script

The script runs after each deploy, in the deploy folder, as the website's user, for at most 15 minutes. It can have up to 2000 characters. For example, for a Laravel site:

```sh
composer install --no-dev --optimize-autoloader
php artisan migrate --force
```

On an app website the script runs in the `app` folder, for example `npm ci` followed by `npm run build`. Its `node` and `npm` are the Node.js version chosen for the website, and on a Python website `python` and `pip` are those of its virtual environment (`app/.venv`).

- It runs with `sh -e`: the script stops at the first command that fails, and the deploy is marked **Failed**. The new files are in place by then.
- `php` is the website's PHP version, also for Composer.
- `DEPLOY_COMMIT` holds the deployed commit and `DEPLOY_BRANCH` the branch.
- `HOME` is the website's `tmp` folder.
- The script can't use `sudo` or restart system services. To restart an app website's program after a deploy, click **Restart** on the website's **Overview** tab.

## Deploy on every push

Under **Deploy on push**, Nefo shows the **Webhook URL**, such as `https://example.com/.panel/deploy/12`, and its **Secret**. The webhook is on the website's own domain, so the repository host never needs to reach the panel's port. The web server passes it to a small listener of the panel on `127.0.0.1:7809`.

::: warning
Without HTTPS, the webhook URL uses `http://`, and GitLab would send the secret unencrypted. Turn on HTTPS first: see [SSL certificates](./ssl).
:::

Each push to the chosen branch deploys the website. Pushes to other branches, tag pushes, other events and branch deletions are ignored.

### GitHub

1. Open the repository → Settings → Webhooks → Add webhook.
2. Paste the **Webhook URL** as the payload URL and the **Secret** as the secret.
3. Set the content type to `application/json`.
4. Choose "Just the push event" and add the webhook.

GitHub sends a test (ping) right away, and the panel answers it, so the first delivery shows as successful.

### GitLab

1. Open the project → Settings → Webhooks.
2. Paste the **Webhook URL** as the URL and the **Secret** as the secret token.
3. Tick Push events and add the webhook.

### Gitea and Forgejo

1. Open the repository → Settings → Webhooks → Add webhook, and pick Gitea (or Forgejo).
2. Paste the **Webhook URL** as the target URL and the **Secret** as the secret.
3. Set the content type to `application/json`, trigger on push events, and add the webhook.

### How the panel checks a webhook

- GitHub, Gitea, Forgejo and Gogs sign each webhook with the secret (HMAC-SHA256). GitLab sends the secret itself. A webhook with a wrong or missing signature gets HTTP 401 and is logged with the sender's address.
- The website's password protection and web firewall don't apply to the webhook path, so you don't need to make an exception for it.
- Nefo runs one task at a time. If another task is running when a push arrives, the deploy waits and is tried again every 15 seconds for up to an hour. Several pushes in the meantime need one deploy: it fetches the newest commit.
- **New secret** replaces the secret. Pushes are refused until you paste the new secret into the repository's webhook.

The repository host shows the panel's answer in its list of recent deliveries: `deploying`, `queued: another task is running`, `ignored: not branch main`, or an error such as `bad_signature`.

## The deploy log

- The **Git deploy** tab shows the log of the current or last deploy, also one that a push started.
- **Account → Recent activity** lists every deploy as "Deployed from Git" or "Git deploy failed", with the website, the commit and what started it (`manual` or a push from the repository host).

## Stop deploying from Git

Under **Stop deploying from Git**, click **Stop deploying from Git** and confirm. Nefo forgets the settings and deletes the deploy key. The deployed files stay in the folder, together with the repository's `.git` folder.
