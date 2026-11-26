---
title: Mail and alerts
description: Let your websites send e-mail through an SMTP provider, and get alerts by Telegram or e-mail when the server needs attention.
---

# Mail and alerts

This page covers two things: how your websites send e-mail through an SMTP provider, and how Nefo alerts you by Telegram or e-mail when something on the server needs attention.

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

Alerts send you a message by Telegram, by e-mail, or both, when the server needs attention. Go to **Settings** → **Alerts**.

The panel checks the server every minute. Each problem is reported once, again every 6 hours while it lasts, and once more when it is solved. Each message starts with the server's name in brackets. E-mail alerts have the subject "Server alert:" followed by the server's name.

### What sends an alert

| Alert | Sent when | Default |
|---|---|---|
| **CPU above (%)** | CPU use averaged over 5 minutes reaches the limit | 90 |
| **Memory above (%)** | memory use reaches the limit | 90 |
| **Disk above (%)** | the disk (`/`) is that full | 90 |
| **A service stops (Nginx, PHP, MariaDB…)** | a service that starts at boot is not running | on |
| **A backup, restore or certificate renewal fails** | a backup, a restore, a website certificate renewal or the panel certificate renewal fails | on |

- Set a limit to 0 to turn it off. Other limits must be between 10 and 100.
- The services watched are the web server (Nginx, Apache or OpenLiteSpeed), PHP-FPM, MariaDB, Redis and fail2ban, when installed.
- Nothing is checked until at least one channel is set up.

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

### Send a test alert

**Send a test** sends a test message through every channel that is saved. It uses the saved settings, so select **Save** first. The result shows one line per channel, **Telegram** or **E-mail**, with **sent** or the error. If no channel is set up, the panel says "Set up Telegram or e-mail first, and save."

### Alerts and the dashboard

The **Needs attention** list on the **Dashboard** and the bell at the top right only show problems while you're signed in, and they check other things, such as two-factor sign-in and the firewall. Alerts reach you when you're away. See [System tools](./system#dashboard).
