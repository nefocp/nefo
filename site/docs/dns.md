---
title: DNS (Cloudflare)
description: Manage the DNS records of your Cloudflare zones from the panel, point a website's names to this server, add the MX, SPF and DMARC records of your mail provider, and purge Cloudflare's cache.
---

# DNS (Cloudflare)

Nefo doesn't run a DNS server. If your domain's DNS is hosted at Cloudflare, Nefo manages its records through Cloudflare's API with an API token: it adds, changes and deletes records, points a website's names at this server after a preview, adds the MX, SPF and DMARC records of your mail provider, and purges Cloudflare's cache.

You save the token in **Settings** → **Cloudflare**. Then:

- the **DNS** page, in the **Hosting** group of the left menu, lists your zones and their records, adds, changes and deletes records, and purges Cloudflare's cache;
- **Point DNS here…** on a website's **Domains** tab points the website's names to this server in one step;
- **Set up email** on the DNS page adds the MX, SPF and DMARC records of a mail provider;
- wildcard certificates are validated through the same token: see [SSL certificates](./ssl#wildcard-certificates-through-cloudflare-dns).

Nefo only talks to Cloudflare's API, and the token never appears in the panel's logs or messages.

## The API token

One token serves all of this. It needs these permissions, for the zones (domains) you use on this server:

| Permission | What it is for |
|---|---|
| Zone → DNS → Edit | Listing, adding, changing and deleting records, **Point DNS here…**, and DNS validation of certificates. |
| Zone → Zone → Read | Listing the zones with their status and plan. Without it, the zone list can stay empty, or Cloudflare refuses to list the zones. |
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

## How do I set up the DNS records for my e-mail? {#set-up-email-records}

Nefo doesn't host mailboxes (see [Mail and alerts](./mail-alerts)). If your mail is hosted by Google Workspace, Microsoft 365, Zoho Mail or another provider, or forwarded by Cloudflare Email Routing, Nefo can add the DNS records the provider needs:

| Record | What it does |
|---|---|
| MX | Says which servers receive mail for the domain. |
| SPF (a TXT record) | Says which servers may send mail for the domain. |
| DMARC (a TXT record at `_dmarc`) | Tells receiving servers what to do with mail that fails SPF and DKIM checks, and where to send reports. |
| DKIM (a TXT or CNAME record) | Publishes the key your provider signs mail with. |

Nothing changes until you have seen the list of changes and applied it.

1. Open **DNS** and choose the zone.
2. Click **Set up email**.
3. Under **Mail provider**, choose **Google Workspace**, **Microsoft 365**, **Zoho Mail**, **Cloudflare Email Routing** or **Other provider**.
4. Optional: in **Mail domain**, enter a name in the zone, such as `mail` for `mail.example.com`. Leave it empty for the zone itself.
5. Optional: enter a **DMARC report address (optional)**, and paste the DKIM record if you have it already (see below).
6. Click **Show the changes**. The table lists each record with its **Now** and **After** value and what happens to it: new, changed, removed, no change, or other provider.
7. Click the apply button, such as **Apply 4 changes**.

If you change a field after the preview, click **Preview again** before you apply. If the records at Cloudflare changed in the meantime, Nefo refuses and shows the new preview.

### What each provider gets

| Provider | MX records | SPF include |
|---|---|---|
| **Google Workspace** | `smtp.google.com`, priority 1 | `include:_spf.google.com` |
| **Microsoft 365** | `example-com.mail.protection.outlook.com` (your domain with dashes for dots), priority 0 | `include:spf.protection.outlook.com` |
| **Zoho Mail** | `mx.zoho.com` (10), `mx2.zoho.com` (20), `mx3.zoho.com` (50), with the domain of the chosen **Zoho data centre**: zoho.com, zoho.eu, zoho.in or zoho.com.au | `include:zohomail.com`, or `zohomail.eu`, `zohomail.in`, `zohomail.com.au` |
| **Cloudflare Email Routing** | `route1.mx.cloudflare.net`, `route2.mx.cloudflare.net`, `route3.mx.cloudflare.net` | `include:_spf.mx.cloudflare.net` |
| **Other provider** | The servers you enter under **Mail servers (MX)**, up to 5, each with a priority | The domain in **SPF include (optional)** and the addresses in **Sending addresses (optional)**; with neither, `mx` |

Some details:

- **Google Workspace**: if the zone already has Google's older MX records (`aspmx.l.google.com` and the others), Nefo keeps them and adds nothing.
- **Microsoft 365**: Microsoft gives each domain its own MX host. Compare the value in the preview with the one the Microsoft 365 admin center shows for the domain. If it is different, choose **Other provider** and enter it. An MX record that already points to Microsoft 365 is kept as it is.
- **Zoho Mail**: choose the data centre whose address you sign in at. For another Zoho data centre, choose **Other provider** and enter the values Zoho shows.
- **Cloudflare Email Routing**: mail arrives only after you turn on Email Routing for the zone in the Cloudflare dashboard and add a destination address there. Existing Cloudflare routing records keep the priorities Cloudflare gave them.

### SPF: always one record

A domain must have exactly one SPF record; two make the SPF check fail. Nefo never adds a second one:

- If the zone has an SPF record, Nefo keeps everything in it, adds the provider's include, and keeps its ending (`-all`, `~all` or `?all`). A record without an ending gets `~all`.
- If the zone has several SPF records, Nefo merges them into one: it changes the first and removes the others.
- If you set up an SMTP relay in **Settings** → **E-mail**, the SPF record also allows the relay's provider (for example `include:sendgrid.net` for SendGrid, `include:mailgun.org` for Mailgun, `include:amazonses.com` for Amazon SES), so mail your websites send passes the check. The preview says so.
- An SPF record that needs more than 10 DNS lookups (`include`, `a`, `mx`, `ptr`, `exists`, `redirect`) fails at many receivers. The preview warns you; remove the includes you no longer use.

### MX records of another provider

MX records that point to another provider are marked "other provider". Mail would go to both providers, so they have to go: tick **Replace the existing MX records** to remove them. Nefo refuses to apply the changes until you do, or until you remove those records yourself.

::: warning
Mail stops reaching the old provider as soon as its MX records are removed. Move your mailboxes first.
:::

### DMARC

Nefo adds `v=DMARC1; p=none` at `_dmarc`, with `rua=mailto:` and your address when you enter a **DMARC report address (optional)**. With `p=none`, no mail is rejected: receiving servers only send reports. When the reports show that all your mail passes, you can make the policy stricter (`p=quarantine`, then `p=reject`) by editing the record on the DNS page.

If the zone already has a DMARC record, Nefo leaves it as it is.

### DKIM

The mail provider makes the DKIM key, so Nefo can't create it. Under **DKIM signature**, the dialog says where to find it at your provider:

- **Google Workspace**: in the Google Admin console, open Apps → Google Workspace → Gmail → Authenticate email, and generate a new record. Its name is `google._domainkey` and its type TXT.
- **Microsoft 365**: in the Microsoft Defender portal, open the DKIM settings of the domain. Microsoft gives two CNAME records, `selector1._domainkey` and `selector2._domainkey`; the dialog has a row for each.
- **Zoho Mail**: in the Zoho Mail Admin Console, open the domain's DKIM settings and add a selector. Enter the selector's name followed by `._domainkey` and the TXT value.
- **Cloudflare Email Routing** needs no DKIM record here: Cloudflare adds what it needs when you turn Email Routing on.
- **Other provider**: ask the provider for its DKIM record.

Paste the **Record name** and **Value**, choose the **Type** (TXT or CNAME), and preview. A name such as `google._domainkey` is under the mail domain; a full name is also accepted. A DKIM record with the same name is replaced. Leave the value empty to add the DKIM record later: run **Set up email** again, with the same provider, when you have it. Then finish the setup at the provider (Google: **Start authentication**; Microsoft and Zoho: turn DKIM on or verify it).

Running **Set up email** again is safe: records that are already right are shown as "no change".

Every record Nefo adds, changes or removes is written to the activity log, like other DNS changes.

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
