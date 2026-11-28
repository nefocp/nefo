---
title: SSL certificates
description: Get free Let's Encrypt certificates, wildcard certificates through Cloudflare DNS, or install your own, and fix the usual failures.
---

# SSL certificates

Nefo gets a free Let's Encrypt certificate for each website in one click and renews it by itself. It can also get wildcard certificates through Cloudflare DNS, install a certificate you already have, redirect HTTP to HTTPS and send HSTS. Before each request it checks that every name reaches the server, so failed attempts don't count against Let's Encrypt's limits.

This page covers the requirements, the redirect to HTTPS and HSTS, wildcard certificates through Cloudflare DNS, installing a certificate you already have, turning HTTPS off again, and what to do when a certificate fails.

## Before you start

For a normal certificate, Let's Encrypt checks over plain HTTP that each name of the website reaches your server. Every name must pass: the domain and each of its [extra domain names](./websites#domains-and-aliases).

- **DNS:** each name has an A record that points to this server's IP address, for example `203.0.113.10`. If a name also has an AAAA (IPv6) record, it must point to this server too, or be removed.
- **Port 80:** the server answers on port 80 from the internet. Nefo opens ports 80 and 443 in ufw when it installs the web server. If your hosting provider has its own firewall, open port 80 there as well.
- **Proxy:** if your DNS is at Cloudflare with the proxy on (orange cloud), read [Cloudflare proxy](#cloudflare-proxy-orange-cloud) first.

By turning on HTTPS you accept the Let's Encrypt [Subscriber Agreement](https://letsencrypt.org/repository/).

## Get a free certificate

1. Open **Websites**, then the website's **SSL / HTTPS** tab. In the website list, **Turn on HTTPS** in the **SSL** column opens the same tab.
2. Under **Free certificate from Let's Encrypt**, leave **Validate through DNS (Cloudflare)** unticked.
3. Click **Get certificate**.

A log shows the steps: Nefo checks that every name reaches this server, asks Let's Encrypt for the certificate, validates each name and turns on HTTPS in the web server. It ends with "Done. The certificate is valid until" and the date.

When a site gets its first certificate, Nefo also switches a WordPress site it installed to its `https://` address, and restarts a Docker app with its `https://` address. The files are kept in `/var/lib/nefo/certs/example.com/` as `fullchain.pem` and `privkey.pem`.

### The check before the request

Before it asks Let's Encrypt, Nefo puts a test file on the server and fetches it through each name over HTTP. If a name fails, Nefo doesn't send the request at all, so failed attempts don't count against the Let's Encrypt limits. The log then says "these names do not reach this server yet, so no certificate was requested:" with one line per name:

| Message | What to fix |
|---|---|
| `blog.example.com: no DNS record. Add an A record that points to this server's IP address.` | Add the A record and wait until it is visible. |
| `example.com (203.0.113.10): no answer over HTTP on port 80 (…). Check the DNS record and the firewall.` | Open port 80 in your provider's firewall, and check that the web server runs. |
| `example.com (198.51.100.7) answered HTTP 404, but not from this server. Check that the DNS record points here.` | The name points to another server: an old IP address, an AAAA record elsewhere, or a proxy such as Cloudflare. |

The addresses in parentheses are what the name resolves to. Compare them with your server's IP address.

## Redirect HTTP to HTTPS

Once the site has a certificate, turn on **Redirect HTTP to HTTPS** on the **SSL / HTTPS** tab or on the **Overview** tab. Every `http://` address then answers with a permanent redirect (301) to `https://`. Let's Encrypt validation and Git webhooks still work over HTTP.

To make browsers use HTTPS even before the redirect, turn on [HSTS](#hsts) too.

## HSTS

HSTS (the `Strict-Transport-Security` header) tells browsers to open the site only over `https://`, even when someone types `http://` or follows an old link. Turn on **HSTS (Strict-Transport-Security)** on the **SSL / HTTPS** tab. It needs a certificate and **Redirect HTTP to HTTPS**; turning the redirect off turns HSTS off too.

Nefo then sends `Strict-Transport-Security: max-age=15552000` (180 days, about 6 months) with the site's HTTPS answers, on Nginx, Apache and OpenLiteSpeed. Tick **Include subdomains** to add `includeSubDomains`: browsers then also use HTTPS for every subdomain of the domain, so tick it only when all of them have HTTPS. Nefo never adds `preload`: the browsers' preload list is hard to leave, so submit a domain to it yourself, if at all.

::: warning
A browser that got the header refuses plain HTTP for the site for 6 months after its last visit, even if you turn HSTS or HTTPS off later. Turn HSTS on only for a site that keeps HTTPS.
:::

On Nginx the header is set for the site's whole `server` block. If your own directives (see [Websites](./websites#rewrite-rules-and-your-own-directives)) add a `location` with an `add_header` of its own, Nginx drops the site's headers in that location: repeat `add_header Strict-Transport-Security "max-age=15552000" always;` there.

To check it, run `curl -sI https://example.com/ | grep -i strict-transport`.

## Automatic renewal

Nefo checks the certificates when it starts and then every hour. It renews a Let's Encrypt certificate when a third of its lifetime is left: 30 days before the end for a 90-day certificate. You don't need to do anything, and a suspended website keeps renewing.

If a renewal fails, Nefo tries again every 12 hours. Meanwhile:

- the website list shows "Renewal failed" in the **SSL** column;
- the **SSL / HTTPS** tab shows "The last certificate attempt failed." with the error;
- an alert goes out if you set up alerts for failed renewals: see [Mail and alerts](./mail-alerts).

The panel's own certificate, from **Settings** → **Certificate** (see [Security](./security#https-from-the-first-start)), renews the same way: a third of its lifetime before the end, and again every 12 hours after a failure. A failed renewal is recorded in the activity log and sends the same alert.

Nefo registers its Let's Encrypt account without an e-mail address, so Let's Encrypt doesn't write to you about expiring certificates. Rely on the panel's alerts instead.

To renew by hand, for example after you fixed a problem, click **Renew certificate** on the **SSL / HTTPS** tab.

## Wildcard certificates through Cloudflare DNS

A wildcard name such as `*.example.com` can only be validated through DNS. Nefo does this through Cloudflare's API, so the domain's DNS must be hosted at Cloudflare. DNS validation also works for a site that is not reachable over HTTP yet, for example behind Cloudflare's proxy. For a certificate, Nefo only adds and removes `_acme-challenge` TXT records.

### Create a Cloudflare API token

The same token also serves the [DNS page](./dns). Its permissions are listed in one place, under [The API token](./dns#the-api-token): certificates need only Zone → DNS → Edit; the DNS page can also use Zone → Zone → Read and Zone → Cache Purge → Purge.

1. In the Cloudflare dashboard, open **My Profile → API Tokens → Create Token**.
2. Use the **Edit zone DNS** template. It gives the permission Zone → DNS → Edit.
3. Under the zone resources, include the zones (domains) you use on this server.
4. Create the token and copy it.

See Cloudflare's guide to [creating an API token](https://developers.cloudflare.com/fundamentals/api/get-started/create-token/).

### Save the token in Nefo

1. Open **Settings**, then the **Cloudflare** tab.
2. Paste the token into **API token** and click **Check and save**.

Nefo checks that Cloudflare accepts the token and that it is active, then shows "Token saved" with the token's last characters and the **Zones** it can use. Use **Replace the API token** to change it, or **Remove** to delete it.

### Get the certificate

1. For a wildcard, open the website's **Domains** tab, add `*.example.com` to **Extra domain names** and click **Save**.
2. On the **SSL / HTTPS** tab, tick **Validate through DNS (Cloudflare)**. For a site with a wildcard name it is ticked and can't be changed: "Required: this site has a wildcard name (*.domain)."
3. Click **Get certificate**.

Nefo adds the TXT record `_acme-challenge.example.com`, waits up to 3 minutes until Cloudflare's name servers answer with it, validates, then deletes the record. The site remembers the choice, so its renewals use DNS validation too.

The certificate covers the names; visitors still need DNS records that point the subdomains to this server.

## Use your own certificate

Install your own certificate when you bought one from another provider, or for a Cloudflare origin certificate.

1. Open the website's **SSL / HTTPS** tab and go to **Your own certificate**.
2. Paste **Certificate (PEM, with intermediates)**: the certificate followed by its chain. You can also choose a `.pem`, `.crt`, `.cer` or `.txt` file.
3. Paste **Private key (PEM, not encrypted)**, or choose a `.pem`, `.key` or `.txt` file.
4. Click **Install**.

Nefo checks the files before it changes anything, and refuses:

- a file that is not a PEM certificate, or larger than 64 KB;
- a private key protected by a passphrase. Remove it first, for example with `openssl pkey -in key.pem -out plain.pem`;
- a key that doesn't belong to the certificate;
- a certificate that is not valid yet, or has expired;
- a certificate that doesn't cover every name of the site. A wildcard name of the site must be listed in the certificate exactly.

It installs the certificate with a warning ("Installed, with warnings:") when browsers won't trust it on their own, which is expected for a Cloudflare origin certificate used behind Cloudflare's proxy, or when it expires within two weeks.

Nefo doesn't renew your own certificate. In its last two weeks, the **SSL / HTTPS** tab says "Nefo doesn't renew a certificate you installed. Install a new one before" and the date, and **Needs attention** on the dashboard lists the site. Nefo also reminds you once a day: the activity log records it as a failed renewal, and the alert for failed renewals goes out. Install a new certificate in time, or switch back to Let's Encrypt with **Get certificate**. The website list marks these sites with "own cert".

## Turn off HTTPS

To take a website back to plain HTTP, click **Turn off HTTPS** at the bottom of its **SSL / HTTPS** tab and confirm. Nefo then:

- serves the website over `http://` only: the HTTPS part of its web server configuration is removed;
- turns off **Redirect HTTP to HTTPS** and [HSTS](#hsts);
- deletes the certificate files in `/var/lib/nefo/certs/example.com/`, from Let's Encrypt or your own, so nothing renews or reminds you about them. The Let's Encrypt account stays for the other websites;
- switches a WordPress site it installed back to its `http://` address, and restarts a Docker app with its `http://` address.

Visitors who open an `https://` address of the site (from a bookmark, a search result or a link) then get a browser error, because the server no longer has a certificate for the name. If the site had HSTS, browsers that got the header keep refusing plain HTTP for up to 6 months, so for them the site doesn't open at all until it has HTTPS again. You can get a new certificate at any time with **Get certificate**.

To replace a certificate you don't need to turn HTTPS off: get a new Let's Encrypt certificate or install your own over it. Deleting a website deletes its certificate too: see [Websites](./websites). Uninstalling the web server keeps the certificates.

## When a certificate fails

The error appears on the website's **SSL / HTTPS** tab under "The last certificate attempt failed." and in the log of the attempt.

### DNS or port 80

See [the check before the request](#the-check-before-the-request). Most failures are a DNS record that doesn't point to this server yet, or port 80 closed in a provider firewall.

### Cloudflare proxy (orange cloud)

With Cloudflare's proxy on, the domain resolves to Cloudflare's addresses, and requests reach Cloudflare before your server. Cloudflare settings such as **Always Use HTTPS** (a redirect to HTTPS, under **SSL/TLS → Edge Certificates**) can then stop the check from reaching the server unchanged, and Nefo reports that the name "answered HTTP …, but not from this server". See also [Let's Encrypt behind the orange cloud](./dns#let-s-encrypt-behind-the-orange-cloud). You have three ways out:

- tick **Validate through DNS (Cloudflare)** and get the certificate through DNS. The proxy can stay on, and renewals work the same way;
- install a Cloudflare origin certificate as [your own certificate](#use-your-own-certificate);
- turn the proxy off (DNS only, grey cloud) for the name. Renewals also check over HTTP, so leave it off.

### CAA records

If your domain has CAA records in its DNS, one of them must allow `letsencrypt.org`. Otherwise Let's Encrypt refuses the certificate, and the log shows its error, which mentions CAA. See [Let's Encrypt and CAA](https://letsencrypt.org/docs/caa/).

### Rate limits

Let's Encrypt limits how many certificates and failed validations a domain and an account may have. When you hit a limit, the log shows Let's Encrypt's message with the time to wait. See [Let's Encrypt rate limits](https://letsencrypt.org/docs/rate-limits/).

::: tip
To test without using up the limits, add `"acme_directory": "https://acme-staging-v02.api.letsencrypt.org/directory"` to `/etc/nefo/config.json` and restart the panel with `systemctl restart nefo`. Staging certificates are not trusted by browsers. Remove the line and restart again to get real certificates.
:::

### DNS validation through Cloudflare

| Message | What to fix |
|---|---|
| `DNS validation needs a Cloudflare API token: add one in Settings → Cloudflare` | Save a token on the **Settings** page, **Cloudflare** tab. |
| `Cloudflare did not accept this token.` | The token is wrong, expired or not active. Create a new one. |
| `none of the Cloudflare zones this API token can use holds example.com` | Add the domain's zone to the token's zone resources in Cloudflare. |
| `the DNS record _acme-challenge.example.com did not appear on … in time` | Cloudflare's name servers were slow. Try again a few minutes later. |
