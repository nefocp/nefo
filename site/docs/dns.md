---
title: DNS (Cloudflare)
description: Manage the DNS records of your Cloudflare zones from the panel, point a website's names to this server, and purge Cloudflare's cache.
---

# DNS (Cloudflare)

Nefo doesn't run a DNS server. If your domain's DNS is hosted at Cloudflare, Nefo manages it through Cloudflare's API with the token you save in **Settings** → **Cloudflare**:

- the **DNS** page, in the **Hosting** group of the left menu, lists your zones and their records, adds, changes and deletes records, and purges Cloudflare's cache;
- **Point DNS here…** on a website's **Domains** tab points the website's names to this server in one step;
- wildcard certificates are validated through the same token: see [SSL certificates](./ssl#wildcard-certificates-through-cloudflare-dns).

Nefo only talks to Cloudflare's API, and the token never appears in the panel's logs or messages.

## The API token

One token serves all of this. It needs these permissions, for the zones (domains) you use on this server:

| Permission | What it is for |
|---|---|
| Zone → DNS → Edit | Listing, adding, changing and deleting records, **Point DNS here…**, and DNS validation of certificates. |
| Zone → Zone → Read | Listing the zones with their status and plan. Add it if the zone list stays empty or Cloudflare refuses it. |
| Zone → Cache Purge → Purge | **Purge cache** on the DNS page. |

1. In the Cloudflare dashboard, open **My Profile → API Tokens → Create Token**.
2. Start from the **Edit zone DNS** template. It gives Zone → DNS → Edit.
3. Click **+ Add more** and add Zone → Zone → Read and Zone → Cache Purge → Purge.
4. Under **Zone Resources**, include the zones you use here, or all zones of the account.
5. Create the token, copy it, and save it in Nefo: **Settings** → **Cloudflare** → **API token** → **Check and save**.

When the token lacks a permission, Nefo names it, for example: "Cloudflare refused: the API token lacks the permission Zone → DNS → Edit for this zone." Edit the token in Cloudflare (**My Profile → API Tokens**, then **Edit** next to the token), add the permission for the zone, and save. Editing a token keeps its value, so there is nothing to change in Nefo. If you deleted or rolled the token, save the new one in **Settings** → **Cloudflare**.

## The DNS page

The page shows one zone at a time. With several zones, choose one in the list at the top; its status (**Active**, or **Pending** while the domain's name servers are not Cloudflare's yet) and plan stand next to it.

The table lists the zone's records: **Type**, **Name** (`@` is the zone itself, `www` is `www.example.com`), **Content**, **Proxy** and **TTL**. Filter it by type, or search names and content. A zone with more than 5,000 records shows its first 5,000; use Cloudflare's dashboard for the others.

Nefo can add and change A, AAAA, CNAME, TXT, MX and CAA records. Other types, such as NS or SRV, are listed with "in Cloudflare": change them in Cloudflare's dashboard.

### Add or change a record

Click **Add record**, or **Edit** on a record's row. A record's type can't change: delete it and add a new one. Nefo checks the record before it sends it to Cloudflare:

| Type | What to enter |
|---|---|
| A | An IPv4 address, such as `203.0.113.10`. |
| AAAA | An IPv6 address, such as `2001:db8::10`. |
| CNAME | The host name this name is an alias of, such as `example.com`. It can't point to its own name. |
| TXT | One line of text, up to 2,048 characters: plain text, which Cloudflare splits into strings of 255 characters, or `"quoted strings"` of up to 255 characters each. Quote the whole value or none of it. |
| MX | The mail server's host name, such as `mail.example.com`, and a **Priority** from 0 to 65535 (lower is tried first). |
| CAA | A **Tag**: `issue` or `issuewild` with a CA's domain, such as `letsencrypt.org` (`;` allows no CA), or `iodef` with a `mailto:` or `https://` address. **Flags** are 0, or 128 for critical. |

**TTL** is **Auto** (Cloudflare's choice) or a fixed time from 1 minute to 1 day.

### Proxied or DNS only

A, AAAA and CNAME records can be **Proxied** (Cloudflare's orange cloud): visitors then reach Cloudflare, which passes web requests on to the address and hides it. Proxied records always have the automatic TTL. **DNS only** (grey cloud) answers with the address itself. Other record types are always DNS only.

Cloudflare's proxy only passes HTTP and HTTPS on ports 80 and 443 (and a few others). Keep everything else DNS only: SSH, mail, and the panel.

::: warning Keep the panel's name DNS only
If you open the panel through a name such as `panel.example.com`, keep that record **DNS only**. Cloudflare's proxy doesn't pass the panel's port, so with the orange cloud the panel no longer opens through that name. The DNS page and **Point DNS here…** warn you when the record is proxied.
:::

## Point a website to this server

1. Open **Websites**, then the website's **Domains** tab.
2. Under **DNS at Cloudflare**, click **Point DNS here…**.
3. Check the addresses and the choices:
   - **This server's public IPv4 address**, read from the server's network interfaces;
   - **Also publish an IPv6 address (AAAA)**, ticked when the server has a public IPv6 address;
   - **Proxied through Cloudflare (orange cloud)**.
4. Read the preview: for each name and type, the record **Now** and **After**, marked **new**, **changed**, **removed** or **no change**.
5. Click **Apply**.

Nefo only changes the records of the website's names, its domain and extra domain names; records of other names are never touched. For each name:

- one A record points to this server: an existing A record is updated, or a new one is created, and other A records of the name are removed;
- with IPv6, the same happens to its AAAA records. Without it, AAAA records that point to another server are removed, since IPv6 visitors and Let's Encrypt would reach that server instead;
- a CNAME record is replaced by the A record, unless it points to another name of the website (`www` → `example.com` stays as it is);
- names that are in no zone the token can see are left alone, and the preview lists them.

When you click **Apply**, Nefo works the changes out again. If the records changed at Cloudflare since the preview, nothing is changed and Nefo shows the new preview: "The records at Cloudflare changed since the preview."

If the server is behind NAT and has only a private IPv4 address, Nefo says "No public IPv4 address was found on this server's network interfaces". Type the public address that reaches the server. Nefo asks no outside service for it.

### Let's Encrypt behind the orange cloud

Let's Encrypt checks a website over plain HTTP. With the proxy on, Cloudflare's **Always Use HTTPS** setting (**SSL/TLS → Edge Certificates**) answers that check with a redirect before it reaches the server, and the certificate fails. Either:

- turn **Always Use HTTPS** off in Cloudflare, and turn on [Redirect HTTP to HTTPS](./ssl#redirect-http-to-https) in Nefo instead; or
- tick **Validate through DNS (Cloudflare)** on the website's **SSL / HTTPS** tab, and leave Cloudflare's settings as they are.

Once the website has a certificate, set Cloudflare's **SSL/TLS** encryption mode to **Full (strict)**. With **Flexible**, Cloudflare reaches the server over HTTP, and **Redirect HTTP to HTTPS** sends it back to HTTPS in a loop.

## Purge Cloudflare's cache

With the proxy on, Cloudflare keeps copies of your files. After you change them, click **Purge cache** on the DNS page:

- **Everything** empties Cloudflare's cache for the whole zone. Cloudflare then fetches every file from the server again, so the websites can be slower for a few minutes;
- **These URLs** purges up to 30 addresses of the zone at a time, one per line, such as `https://example.com/style.css`.

This is Cloudflare's cache, not the website's own [page cache](./websites#page-cache) on this server.

## Activity log

Every change is recorded in the activity log (**Account** → **Recent activity**): "Added a DNS record", "Changed a DNS record", "Deleted a DNS record" and "Purged Cloudflare's cache", with the record's type, name and content. **Point DNS here…** records each record it changes.

## Messages

| Message | What to do |
|---|---|
| `No Cloudflare API token is saved.` | Save a token in **Settings** → **Cloudflare**. |
| `Cloudflare refused: the API token lacks the permission …` | Add the permission it names to the token: see [The API token](#the-api-token). |
| `Cloudflare did not accept this token.` | The token is wrong, expired or deleted. Create a new one. |
| `None of these names is in a Cloudflare zone the API token can see` | Add the zone to the token's **Zone Resources**, or set the records where the domain's DNS is hosted. |
| `Cloudflare does not answer for example.com yet (pending).` | Change the domain's name servers at your registrar to the two Cloudflare shows. |
| `Cloudflare refused: …` | Cloudflare's own message, for example when a CNAME and an A record would share a name. |
