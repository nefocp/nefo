---
title: Mail and alerts
description: Let your websites send e-mail through an SMTP provider, and get alerts by Telegram, e-mail, Discord, Slack or a webhook when the server needs attention or someone signs in.
---

# Mail and alerts

Nefo has no mail server. Websites send e-mail through Postfix as a send-only relay to your SMTP provider (Gmail, SendGrid, Amazon SES and others). Alerts reach you by Telegram, e-mail, Discord, Slack or a signed webhook when the CPU, memory or disk is high, a service stops, a backup fails, a website is down, or someone signs in.

This page covers both: sending e-mail from websites, and alerts. To set up the DNS records of a mail provider (MX, SPF, DMARC), see [DNS (Cloudflare)](./dns#set-up-email-records).

## No mail server, by design

Nefo doesn't host mailboxes and doesn't receive e-mail. A full mail server costs memory and needs constant care, which doesn't fit a small VPS. Keep your mailboxes at an e-mail provider.

For outgoing mail, Nefo installs Postfix as a send-only relay. It listens on this server only and passes every message to your provider (Gmail, SendGrid, Amazon SES and others), signed in with the account you enter.

## Send e-mail from websites

Once the relay is set up, PHP's `mail()` function works, because Postfix provides `/usr/sbin/sendmail`, which PHP uses. Contact forms, password resets and WordPress notices then send e-mail without any plugin. Without the relay, websites can't send e-mail.

### Set up the relay

1. Go to **Settings** → **E-mail**.
2. Choose your **Provider**. The panel fills in the **SMTP server**, **Port** and **Encryption** for it.
3. Enter the **User name** and **Password** of your provider account. The hint under **Provider** says what that provider expects.
4. Optionally, fill in **Send everything as (optional)**, such as `noreply@example.com`.
5. The first time, select **Install and save**: the panel installs Postfix and configures it. Later, the button is **Save**.

The setup runs in the background with a live log. When it finishes, send a test message (see below).

| Provider | SMTP server | User name and password |
|---|---|---|
| Gmail / Google Workspace | `smtp.gmail.com` | Your Gmail address, and an app password (Google account → Security → 2-Step Verification → App passwords), not your normal password |
| Microsoft 365 / Outlook | `smtp.office365.com` | Your Microsoft 365 address. SMTP sending must be allowed for the mailbox |
| SendGrid | `smtp.sendgrid.net` | `apikey` (filled in for you), and a SendGrid API key with Mail Send permission. Send from a verified sender |
| Mailgun | `smtp.mailgun.org` | The SMTP credentials of your Mailgun domain |
| Amazon SES | `email-smtp.us-east-1.amazonaws.com` | Your SES SMTP credentials. Change the server to the endpoint of your region, and send from a verified address or domain |
| Brevo | `smtp-relay.brevo.com` | Your Brevo SMTP login and SMTP key |
| Other SMTP server | Your provider's server | Port 587 with STARTTLS, or 465 with SSL/TLS. Port 25 is blocked by most cloud providers |

Every preset uses port 587 with STARTTLS.

### Fields and rules

| Field | What to enter |
|---|---|
| **SMTP server** | The provider's host name, or an IP address |
| **Port** | 1 to 65535, usually 587 or 465 |
| **Encryption** | **STARTTLS** (port 587), **SSL/TLS** (port 465), or **None (this server only)** |
| **User name**, **Password** | Both or neither, without spaces. Leave **Password** empty to keep the saved one |
| **Send everything as (optional)** | An e-mail address that replaces the sender of every message |

- **None (this server only)** is accepted only for a relay on this server itself (`localhost`, `127.0.0.1` or `::1`). Without encryption, the password would cross the internet in clear.
- With STARTTLS or SSL/TLS, Postfix refuses to send if the connection can't be encrypted.
- Use **Send everything as (optional)** when your provider only accepts verified sender addresses. It replaces the sender of every message from every website.
- The provider's password is stored in `/etc/postfix/sasl_passwd`, which only root can read. Websites never see it.

### Send a test message

1. Below the form, enter an address in **Send a test message to…**.
2. Select **Send test**.

The panel follows the message in the mail log and waits up to a minute for the provider's answer. The result says whether the message was sent, is not delivered yet (Postfix keeps trying) or was refused by the provider, followed by the provider's reply. If the provider doesn't answer within a minute, the message stays in the queue and Postfix keeps trying.

### Check the relay

At the top of the card:

- **Relaying through …** means Postfix runs and uses your provider. **Postfix is not running** means it is stopped.
- **… messages waiting** counts messages still in the queue. Postfix keeps retrying a message for up to 2 days.
- **Last delivery problem:** shows the latest reason a message couldn't be delivered, as the provider gave it.

### Remove the relay

Select **Remove** and confirm. The panel uninstalls Postfix and forgets the settings. Websites can no longer send e-mail.

## Alerts

Alerts send you a message by Telegram, e-mail, Discord, Slack or your own webhook, when the server needs attention. Go to **Settings** → **Alerts**. Every channel is optional: set up as many as you like, and each alert goes to all of them at once. A channel that fails doesn't stop the others.

The panel checks the server every minute. Each problem is reported once, again every 6 hours while it lasts, and once more when it is solved. Each message starts with the server's name in square brackets. E-mail alerts have the subject "Server alert:" followed by the server's name.

### What sends an alert

| Alert | Sent when | Default |
|---|---|---|
| **CPU above (%)** | CPU use averaged over 5 minutes reaches the limit | 90 |
| **Memory above (%)** | memory use reaches the limit | 90 |
| **Disk above (%)** | the disk (`/`) is that full | 90 |
| **A service stops (Nginx, PHP, MariaDB…)** | a service that starts at boot is not running | on |
| **A backup, restore or certificate renewal fails** | a backup, a restore, a website certificate renewal or the panel certificate renewal fails | on |
| Website monitoring (each website's **Monitoring** tab) | a monitored website is down (2 failed checks in a row), is up again, or serves a certificate that ends in less than 14 days | off |
| **SSH sign-ins** | someone signs in over SSH: every time, or only from a new address | **No alert** |
| **Failed SSH attempts in 10 minutes** | that many SSH sign-ins failed within 10 minutes | 0 (off) |
| **Panel sign-ins** | someone signs in to the panel: every time, or only from a new address | **No alert** |
| **Alert me when the daily check finds a new problem** (on the **Security check** page) | the daily [security check](./security-check) finds a new critical problem or warning | off |
| **Alert me when PHP files change** (a website's **File lock** tab) | the daily check of a locked website finds PHP files changed, added or removed (see [File lock](./website-protection#alerts-when-php-files-change)) | off |

- Set a limit to 0 to turn it off. Other limits must be between 10 and 100.
- The services watched are the web server (Nginx, Apache or OpenLiteSpeed), PHP-FPM, MariaDB or MySQL, PostgreSQL, Redis, Memcached and fail2ban, when installed.
- Nothing is checked until at least one channel is set up.
- Website monitoring is turned on for each website, and its checks run even without a channel: the **Monitoring** tab shows the results. Down alerts repeat every 6 hours, like the others. See [Websites](./websites#monitoring).

### Sign-in alerts

Get a message when someone signs in to the server over SSH or to the panel, so a sign-in you didn't make doesn't go unnoticed.

1. Go to **Settings** → **Alerts** and set up at least one channel.
2. Under **Sign-ins**, choose for **SSH sign-ins** and **Panel sign-ins**:
   - **No alert**;
   - **Every sign-in**;
   - **Only from a new address**: a sign-in from an address the account has not signed in from in the last 180 days.
3. Optional: in **Failed SSH attempts in 10 minutes**, enter how many failed attempts send an alert, from 3 to 100000. 0 turns it off.
4. Click **Save**.

An SSH alert names the account, the address, how the account signed in and the time, for example:

```
[server1] 🔑 SSH sign-in: admin from 203.0.113.10 (new address) with a key, 2026-11-27 10:31:05 +07
```

Several sign-ins in the same minute come in one message. A burst of failed attempts names the addresses that tried the most:

```
[server1] ⚠️ 57 failed SSH sign-in attempts in the last 10 minutes, from 198.51.100.7 (40), 203.0.113.5 (17).
```

How it works:

- Nothing watches the SSH log all the time. Once a minute, the panel reads what the SSH service (`ssh.service` or `sshd.service`) wrote to the system journal since the last read, and remembers where it stopped. The SSH alerts cost nothing while they are off.
- When you turn the SSH alerts on, they start from that moment: earlier sign-ins are never reported. The addresses of the SSH sign-ins of the last 30 days count as known.
- For the panel, the addresses of the sign-ins in the activity log count as known.
- For IPv6, every address in the same /64 counts as one address, as most providers give one /64 to one customer. Up to 500 addresses are remembered for SSH and 500 for the panel.
- If the panel was stopped for more than an hour, it reads only the last hour of the SSH log when it starts again.
- Failed attempts are wrong passwords and unknown account names. With password sign-in off, refused keys are not counted. At most one burst alert is sent per hour.
- To block the addresses that keep trying, install fail2ban on the **Firewall & SSH** page. See [Brute-force protection](./security#brute-force-protection-fail2ban).

### Get alerts by Telegram

1. In Telegram, open BotFather, send `/newbot` and follow its questions. It answers with the bot's token: digits, a colon, then a long string of letters and digits.
2. Open your new bot and send it any message. A bot can't write to you before you write to it.
3. Get your chat ID, for example from userinfobot. It's a number.
4. In **Settings** → **Alerts**, enter the **Bot token** and the **Chat ID**.
5. Select **Save**, then **Send a test**.

To send alerts to a group or a channel, add the bot to it first. A group's chat ID starts with a minus sign. For a public channel, you can also enter its `@name`.

After you save, the token is shown as dots. Leave the dots as they are to keep the saved token. If you clear the **Chat ID** and save, the token is removed too.

### Get alerts by e-mail

Alert e-mails are sent directly from the panel through an SMTP server, with their own settings, separate from the relay for websites. You can use the same provider account.

1. Under **E-mail**, enter the **SMTP server** and **Port** (587 by default).
2. Enter the **User name** and **Password** of the account, if the server needs a sign-in.
3. Enter the **From** and **To** addresses.
4. Select **Save**, then **Send a test**.

On port 465 the connection is encrypted from the start. On other ports, the panel uses STARTTLS when the server offers it. If the server doesn't offer it and a user name is set, the panel refuses to send, so the password never travels in clear. The saved password is shown as dots, like the Telegram token.

### Get alerts in Discord {#get-alerts-in-discord}

1. In Discord, open the settings of the channel that should receive the alerts, then **Integrations** → **Webhooks** → **New Webhook**.
2. Give the webhook a name if you like, then select **Copy Webhook URL**.
3. In **Settings** → **Alerts**, paste it into **Webhook URL** under **Discord**.
4. Select **Save**, then **Send a test**.

The URL looks like `https://discord.com/api/webhooks/<id>/<token>`. Addresses on `discordapp.com` and the `ptb.` and `canary.` versions work too, and so does `?thread_id=` to post into a thread. Alerts longer than 2000 characters, Discord's limit, are cut. Mentions in an alert, such as `@everyone`, don't notify anyone.

### Get alerts in Slack {#get-alerts-in-slack}

1. Go to [api.slack.com/apps](https://api.slack.com/apps), select **Create New App** → **From scratch**, and choose your workspace.
2. Open **Incoming Webhooks**, turn it on, then select **Add New Webhook to Workspace** and choose the channel.
3. Copy the **Webhook URL**. It starts with `https://hooks.slack.com/services/`.
4. In **Settings** → **Alerts**, paste it into **Webhook URL** under **Slack**, select **Save**, then **Send a test**.

Only incoming webhooks are accepted: Slack's workflow webhooks expect other fields.

### Send alerts to a webhook {#send-alerts-to-a-webhook}

The webhook posts each alert as JSON to an address of your choice: your own script, n8n, Home Assistant, a chat that isn't listed here. Under **Webhook**, enter its **Webhook URL** and, if the receiver checks signatures, a **Secret for signing (optional)** of 16 to 200 characters without spaces. Then select **Save** and **Send a test**.

Each alert is a `POST` with `Content-Type: application/json` and this body:

```json
{"server":"vps-1","text":"⚠️ The disk is 93% full.","time":"2026-11-26T08:15:00Z","kind":"alert"}
```

| Field | Value |
|---|---|
| `server` | The server's host name |
| `text` | The alert, without the server's name. Solved problems start with "✅ Resolved:" |
| `time` | When it was sent, in UTC, RFC 3339 |
| `kind` | `alert`, or `test` for **Send a test** |

Any 2xx answer counts as delivered. The panel waits 10 seconds at most, doesn't follow redirects, and doesn't show the receiver's answer, only its status code.

#### Verify the signature {#verify-the-signature}

With a secret, each request has the header `X-Nefo-Signature: sha256=` followed by the HMAC-SHA256 of the body, in hexadecimal, with the secret as key. Compute it over the body exactly as received, before parsing the JSON, and compare in constant time. In Python:

```python
import hashlib, hmac

def verify(secret: bytes, body: bytes, signature: str) -> bool:
    expected = "sha256=" + hmac.new(secret, body, hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected, signature)
```

In Node.js, compare `'sha256=' + crypto.createHmac('sha256', secret).update(rawBody).digest('hex')` with the header using `crypto.timingSafeEqual`, after checking that both have the same length. The signed body includes `time`: refuse requests that are more than a few minutes old to stop replays.

#### Addresses a webhook can't use {#webhook-addresses}

- The URL starts with `https://`. A plain `http://` address works only after you tick **Allow an http:// address (not encrypted, for a receiver on your own network)**: without encryption, the alert and its signature can be read on the way.
- No user name or password in the URL, no `#`, and at most 2048 characters.
- So that the webhook can't reach the server's own services, the panel never connects to loopback addresses (`127.0.0.0/8`, `::1`, `localhost`), link-local addresses (`169.254.0.0/16`, `fe80::/10`), cloud metadata services (`169.254.169.254`, `100.100.100.200`, `168.63.129.16`, `fd00:ec2::254`), `0.0.0.0/8` or multicast addresses. A host name is checked again each time an alert is sent, on the addresses it resolves to, so a name that points to `127.0.0.1` is refused too.
- Private networks (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `fd00::/8`) are allowed, for a receiver at home or in a container.

### Saved webhook URLs {#saved-webhook-urls}

The Discord, Slack and webhook URLs contain their token, so after you save they're shown as dots, like the Telegram token, and so is the secret. Leave the dots as they are to keep the saved value. Clear a URL and save to stop using that channel. Clearing the webhook's URL also removes its secret and the http:// permission.

### Send a test alert

**Send a test** sends a test message through every channel that is saved. It uses the saved settings, so select **Save** first. The result shows one line per channel, **Telegram**, **E-mail**, **Discord**, **Slack** or **Webhook**, with **sent** or the error. If no channel is set up, the panel says "Set up a channel first, and save."

For Discord, Slack and the webhook, the error is one of these:

- **Cannot reach the webhook** followed by the reason, for example `no such host` or `connection refused`.
- **The webhook refused the message (HTTP 404…). Check its URL.**: the receiver answered with something other than a 2xx status. Discord and Slack add their reason, such as `Unknown Webhook` or `invalid_token`. A redirect (3xx) counts as a refusal too.
- **The webhook did not answer within 10 seconds.**
- **… is a loopback, link-local or cloud metadata address**: see [Addresses a webhook can't use](#webhook-addresses).

When a real alert can't be sent, the panel log says so with the channel's name, never its URL.

### Alerts and the dashboard

The **Needs attention** list on the **Dashboard** and the bell at the top right only show problems while you're signed in, and they check other things, such as two-factor sign-in and the firewall. Alerts reach you when you're away. See [System tools](./system#dashboard).
