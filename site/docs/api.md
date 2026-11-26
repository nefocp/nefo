---
title: API keys
description: Script the panel with API keys, for CI deploys, backups and monitoring, without a browser session. What a key can and can't do, how to call the API, examples and errors.
---

# API keys

An API key lets a script use the panel without a browser: deploy a website from CI, start a backup from a cron job on another server, or read the state of the websites from a monitoring tool. The script sends the key with each request, to the same address and secret path as the panel.

Keys are on **Settings** → **API keys**.

## What a key can do

Each key has one of two kinds of access:

- **Read only**: `GET` and `HEAD` requests only. It can list websites, databases and backups, read job logs, statistics and the state of the server, but not change anything. It also can't read what would give it more power: database and app passwords, database exports, files (the file manager), backup downloads, Redis values and Git webhook secrets. Those requests, and any request other than `GET` or `HEAD`, get `403` with `api_key_read_only`.
- **Full access**: every request the panel's own pages make, in your name.

Even a full key can't:

- create, list or revoke API keys;
- change the password or two-factor sign-in, or sign out (everything under `/api/auth/`);
- open the web terminal or a container's terminal;
- use the database tools behind the panel's sign-in, Adminer and phpMyAdmin.

These answer `403` with `api_key_forbidden`: sign in to the panel for them. A key also can't sign in to get a session.

::: warning
A full key is as powerful as you are: it can write files, add cron jobs that run as root and install software. Keep it as secret as the root password. A read-only key is a secret too: it can read your websites' and the server's settings, logs and lists.
:::

## Create a key

1. Go to **Settings** → **API keys** and select **New key**.
2. Enter a **Name**, such as `deploy-bot`: up to 32 letters, digits, spaces, dots, hyphens and underscores. The name shows in the activity log.
3. Choose **Read only** or **Full access**.
4. Optional: under **Allowed addresses**, enter the IP addresses or networks the key may be used from, such as `203.0.113.7` or `198.51.100.0/24`, IPv4 or IPv6, separated by spaces or new lines. Leave it empty to allow any address.
5. Choose when it **Expires**: after 30, 90 or 365 days, or **Never**.
6. Select **New key**.

The panel shows the key once, with a **Copy** button: copy it now, you won't see it again. A key looks like `nefo_` followed by 43 random characters. The panel keeps only a SHA-256 hash of it.

The list shows each key's name and first characters (such as `nefo_Ab12cd…`), its access, the allowed addresses, when and from where it was last used (updated at most once a minute) and when it expires. You can have up to 20 keys. To revoke a key, select its trash icon and confirm: the next request with it fails.

## Call the API

Send the key in the `Authorization` header, as a Bearer token. The address is the panel's address with its secret path, then `api/` and the route:

```sh
export NEFO_KEY='nefo_…'
export NEFO='https://panel.example.com:PORT/<entrance>'

curl -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/sites"
```

- The key is only read from the `Authorization` header: never from the address (`?key=`) or a cookie.
- A request with a JSON body needs `Content-Type: application/json`. Unknown fields are refused.
- Requests with a key don't need the `X-Requested-With` header that the panel's own pages send.
- Use the panel over HTTPS with a real certificate (**Settings** → **Certificate**, see [Security](./security#https-from-the-first-start)). Avoid `curl -k`: it would send the key to whoever sits in between.

Long tasks, such as backups, deploys and certificates, run in the background. Their request answers `202` with a job:

```json
{ "id": "a1b2c3", "kind": "backup", "target": "example.com", "status": "running", "log": [], "started": "…" }
```

Ask for `GET /api/jobs/{id}` until its `status` is `succeeded` or `failed`; `log` holds its lines and `error` why it failed. The panel runs one task at a time: while another one runs, a new one answers `409` with `busy`.

## Examples

The routes are those that the panel's pages use. Replace `3` with the `id` of your website.

### List the websites

```sh
curl -s -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/sites"
```

The answer is `{"sites": [{"id": 3, "domain": "example.com", "ssl": true, …}]}`. A read-only key is enough.

### Back up a website

```sh
curl -s -X POST -H "Authorization: Bearer $NEFO_KEY" -H "Content-Type: application/json" \
  -d '{"kind": "site", "target": "example.com", "remote": true}' \
  "$NEFO/api/backups"
```

- `kind` is `site` (`target` is the domain), `database` (the database's name), `folder` (an absolute path) or `app` (an App store app's name).
- `remote: true` also uploads the backup to the remote storage, if it is set up (see [Backups](./backups#remote-storage)).

It answers `202` with the job. To wait for it:

```sh
JOB=$(curl -s -X POST -H "Authorization: Bearer $NEFO_KEY" -H "Content-Type: application/json" \
  -d '{"kind": "database", "target": "shop", "remote": false}' "$NEFO/api/backups" | jq -r .id)
while [ "$(curl -s -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/jobs/$JOB" | jq -r .status)" = running ]; do
  sleep 5
done
```

### Deploy from Git

Set up the repository on the website's **Git deploy** tab first (see [Deploy from Git](./git-deploy)). Then, from your CI after the tests pass:

```sh
curl -fsS -X POST -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/sites/3/git/deploy"
```

It needs no body and answers `202` with the job. In a GitHub Actions workflow, keep the key and the address in the repository's secrets:

```yaml
- name: Deploy
  run: curl -fsS -X POST -H "Authorization: Bearer ${{ secrets.NEFO_KEY }}" "${{ secrets.NEFO_URL }}/api/sites/3/git/deploy"
```

### Install a certificate

To get a free certificate from Let's Encrypt (see [SSL certificates](./ssl)):

```sh
curl -s -X POST -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/sites/3/ssl"
```

It answers `202` with the job. To validate through Cloudflare DNS instead, which wildcard names need, send `-H "Content-Type: application/json" -d '{"dns": true}'`.

To install your own certificate, such as one renewed on another server, send the certificate with its chain and the private key, in PEM, up to 256 KB in all. `jq` builds the JSON from the files:

```sh
jq -n --rawfile cert fullchain.pem --rawfile key privkey.pem '{cert: $cert, key: $key}' |
  curl -s -X POST -H "Authorization: Bearer $NEFO_KEY" -H "Content-Type: application/json" \
    --data-binary @- "$NEFO/api/sites/3/certificate"
```

It answers `200` with the website and a list of `warnings`, or `400` with the reason, such as a key that doesn't match the certificate (see [Use your own certificate](./ssl#use-your-own-certificate)).

## Errors

Errors are JSON, such as `{"error": "invalid_api_key"}`, some with a `detail`.

| Status | `error` | Meaning |
|---|---|---|
| 401 | `invalid_api_key` | The key is unknown, revoked or expired, or used from an address it doesn't allow. The answer doesn't say which; the activity log does. |
| 403 | `api_key_read_only` | A read-only key sent something else than `GET` or `HEAD`, or asked for a password, a file, an export or a backup download. |
| 403 | `api_key_forbidden` | Only a signed-in session can do this (see [What a key can do](#what-a-key-can-do)). |
| 429 | `too_many_attempts` | Too many failed attempts from this address. Wait `retry_after` seconds, also in the `Retry-After` header. |
| 400 | such as `bad_request` | The request is wrong; `detail` may say what. |
| 404 | `not_found` | The item doesn't exist. A path outside the secret path gets a bare 404. |
| 409 | `busy` | Another task is running. Try again when it has finished. |

## Failed attempts and fail2ban

A failed attempt with a key counts like a failed sign-in, on the same count (see [Sign-in limits](./security#sign-in-limits)): 5 failures within 15 minutes lock the address out for 15 minutes, for keys and for the sign-in page. A valid key doesn't reset the count.

Each failure is in **Account** → **Recent activity** as "Failed sign-in", with the reason: `unknown API key`, `API key expired` or `API key not allowed from this address`. fail2ban's panel jail reads them, so it bans an address that keeps guessing (see [Brute-force protection](./security#brute-force-protection-fail2ban)).

## The activity log

- Each change made with a key (every request except `GET` and `HEAD`) is logged as "Change through an API key", with the method and path, such as `POST /api/sites/3/git/deploy`.
- The user of these entries is your username and the key's name, such as `admin (key deploy-bot)`. So are the entries of what the request did, such as "Deployed from Git".
- Requests refused to a key are logged as "API key refused"; creating and revoking keys, as "Created an API key" and "Revoked an API key".
- Reads aren't logged. The list of keys shows when each was last used, and from where.

The key itself is never written to any log.

## Security advice

- **Use read-only keys where you can.** Monitoring and reports only need `GET`.
- **One key per script.** Its name shows in the log, and you can revoke it without breaking the others.
- **Allow only the addresses that need it**, such as your CI runners or your backup server, when they have fixed addresses.
- **Let keys expire, and rotate them**: create a new key, put it in the script, then revoke the old one.
- **Keep keys out of repositories and addresses.** Use your CI's secret store or a file readable only by its owner, and a variable in commands, so the key doesn't end up in the shell history.
- **Revoke a key at once if it may have leaked**, then check **Recent activity** for what it did.
- If the [firewall](./security#firewall-ufw) limits the panel's port to some addresses, allow the script's address there too.
