---
title: App store
description: Install PHP apps into a website, Docker apps behind their own website and database services, and manage, update, back up and remove them.
---

# App store

Nefo's App store installs 42 applications in one click: PHP apps such as WordPress, WooCommerce, Joomla, Nextcloud and Moodle into a website, Docker apps such as Ghost, n8n, Gitea, Grafana and Jellyfin behind a website of their own, and PostgreSQL, MongoDB and Ollama as services on 127.0.0.1. It checks the server's memory and disk first.

Nefo creates what each app needs, such as a database, a website, passwords and containers. This page covers the kinds of apps, the catalog, installing, and managing the apps you installed.

Open **App store** in the left menu. The **Catalog** tab lists the apps and the **Installed** tab the apps on this server. On the **Websites** page, **Install from the App store** leads here too.

## Three kinds of apps

| Kind | Where it goes | What it gets |
|---|---|---|
| **PHP app** | Into an empty PHP website that you created | Its own database, its files in the website, the right rewrite rules. It runs as the website's Linux user. |
| **Docker app** | Into a new website that Nefo creates for it | Containers that listen on `127.0.0.1` only, with the website in front of them. |
| **Service** | No website | Containers that listen on `127.0.0.1` only, for your own sites and apps. One of each per server. |

PHP apps and Docker apps need the web server installed: see [Websites](./websites). Docker apps and services also need Docker. Install it on the **Software** page; **Install Docker** in the app store opens that page. See [Docker](./docker).

## Which apps can I install? {#the-catalog}

At catalog version 2026112701, the app store offers 42 apps. They are grouped in categories: **All** shows every category with its apps, and a category button shows only its apps. The search looks at names, descriptions and category names, and ignores Vietnamese accents (typing "giam sat" finds the "Giám sát & thống kê" apps).

The minimum memory is the server's total RAM below which Nefo refuses the install (swap doesn't count).

| App | Kind | Category | What it is | Minimum RAM |
|---|---|---|---|---|
| WordPress | PHP app | Websites & content | Website and blog builder | 450 MB |
| Joomla | PHP app | Websites & content | CMS for websites and portals | 450 MB |
| Drupal | PHP app | Websites & content | CMS for structured content, built with Composer | 900 MB |
| Ghost | Docker app | Websites & content | Blogs and newsletters | 900 MB |
| Directus | Docker app | Websites & content | Headless CMS | 900 MB |
| Wiki.js | Docker app | Websites & content | Wiki | 900 MB |
| WooCommerce | PHP app | Shops & business | WordPress with WooCommerce: an online shop | 900 MB |
| OpenCart | PHP app | Shops & business | Shopping cart | 900 MB |
| Odoo | Docker app | Shops & business | ERP (Community edition) | 1800 MB |
| Stirling-PDF | Docker app | Productivity | PDF tools | 1800 MB |
| Memos | Docker app | Productivity | Quick Markdown notes on a timeline | 230 MB |
| linkding | Docker app | Productivity | Bookmark manager | 450 MB |
| Vikunja | Docker app | Productivity | To-do lists, Kanban boards and projects | 450 MB |
| Actual Budget | Docker app | Productivity | Personal budgeting | 450 MB |
| Kanboard | Docker app | Productivity | Kanban board for projects | 230 MB |
| FreshRSS | Docker app | Productivity | News and blog reader (RSS) | 450 MB |
| Mealie | Docker app | Productivity | Recipes, meal plans and shopping lists | 900 MB |
| Nextcloud | PHP app | Files & storage | Your own cloud for files, calendars and contacts | 900 MB |
| RustFS | Docker app | Files & storage | S3-compatible object storage | 900 MB |
| FileBrowser Quantum | Docker app | Files & storage | Files in the browser | 450 MB |
| Navidrome | Docker app | Media | Music streaming from your own collection | 450 MB |
| Jellyfin | Docker app | Media | Films, series and music from your server | 1800 MB |
| ntfy | Docker app | Communication | Push notifications over HTTP | 230 MB |
| Gotify | Docker app | Communication | Notification server with an Android app | 230 MB |
| listmonk | Docker app | Communication | Newsletters and mailing lists | 450 MB |
| Uptime Kuma | Docker app | Monitoring & analytics | Watches your sites and services | 450 MB |
| Umami | Docker app | Monitoring & analytics | Privacy-friendly website analytics | 900 MB |
| Grafana | Docker app | Monitoring & analytics | Dashboards and alerts | 900 MB |
| changedetection.io | Docker app | Monitoring & analytics | Watches web pages for changes | 900 MB |
| Healthchecks | Docker app | Monitoring & analytics | Alerts when cron jobs stop running | 450 MB |
| Vaultwarden | Docker app | Security & network | Password manager server for the Bitwarden apps | 450 MB |
| AdGuard Home | Docker app | Security & network | Ad blocking through your own DNS-over-HTTPS address | 450 MB |
| Open WebUI | Docker app | AI | Chat interface for Ollama or OpenAI-compatible APIs | 1800 MB |
| Ollama | Service | AI | Runs open language models on the CPU | 3584 MB |
| Laravel | PHP app | Developer tools & automation | A new Laravel project with its database | 450 MB |
| code-server | Docker app | Developer tools & automation | VS Code in the browser | 900 MB |
| IT-Tools | Docker app | Developer tools & automation | Developer tools in the browser | 230 MB |
| Gitea | Docker app | Developer tools & automation | Your own Git server | 450 MB |
| n8n | Docker app | Developer tools & automation | Workflow automation | 900 MB |
| PostgreSQL | Service | Databases | PostgreSQL database server | 900 MB |
| MongoDB | Service | Databases | MongoDB document database | 900 MB |
| Moodle | PHP app | Education | Learning platform | 900 MB |

The catalog is built into the panel. Nefo also checks once a day for a newer catalog published with the releases, and uses it only if its signature is valid. **Update the catalog** checks now. The header shows the catalog version and whether it is "built into the panel" or "downloaded, signature checked".

A downloaded catalog may contain apps that need a newer version of Nefo, because their recipe uses something this version doesn't have. Nefo leaves those apps out and keeps the rest of the catalog. If it already knew the app, it keeps the version it knows, so installed copies keep working. The header then says, for example, "2 apps of the downloaded catalog need a newer panel". Update Nefo to get them: see [Update](./update). Apps that a newer catalog adds show their names and descriptions in every language the catalog has.

Each card shows the app's version, kind, memory and disk needs. Its **Install** button opens the description, licence, needs and notes, and the install form. A card that the server can't run shows why, and **Details** instead. An installed service shows **Manage**. A "Source-available" badge marks a licence that is not OSI-approved.

## The checks before an install

When you open an app, Nefo compares its needs with the server. It refuses the install when:

- the server's total memory is below the app's minimum (on a VPS that is a container, its memory limit counts);
- the free disk space is less than the app's images and data plus 1 GB;
- the processor's architecture or features don't suit the app.

It warns, but lets you install, when the server has less memory than recommended, when less memory is free right now than the app may use, or when the memory limits of all installed apps together exceed the server's memory.

## Install a PHP app

1. Create an empty PHP website with a PHP version the app supports: see [Websites](./websites). The minimum PHP versions are WordPress 7.4, Nextcloud and Laravel 8.2, WooCommerce, Joomla, Drupal and Moodle 8.3, OpenCart 8.0. OpenCart supports PHP up to 8.3 and Moodle up to 8.4.
2. Make sure MariaDB (or MySQL) is installed and running: see [Databases](./databases).
3. In the app store, click the app's **Install** button.
4. Choose the **Website**. The list shows only empty PHP websites with a suitable PHP version. If there is none, use **Add a PHP website**.
5. For apps that ask for it, fill in **Site name**, **Administrator** (avoid `admin`: it is the first name attackers try), **Administrator e-mail** and **Administrator password**. Leave the password empty for a strong random one; a password you type needs 12 to 128 characters. WooCommerce also asks for the store country and currency.
6. Click **Install**. The install runs in the background and shows in the task list.

When it is done, Nefo shows the **Sign-in address**, the **Administrator** and the **Administrator password** once. Save them now.

The website's `public` folder must be empty; the panel's welcome page is fine. The website's files must not be [locked](./website-protection#lock-a-website-s-files). Drupal and Laravel are built with Composer, and Moodle needs the whole site folder too: their website must be new and empty, and the site folder becomes the project, with `public` as its web folder.

A PHP app gets:

- a new database and database user, linked to the website;
- the application's files, downloaded and checked against the checksum in the catalog;
- the app's rewrite rules, and higher PHP limits when the app needs them;
- a cron job run as the website's user for Moodle (every minute) and WooCommerce (every five minutes).

If the install fails, Nefo empties the website again and deletes the new database. WordPress sites also appear in the [WordPress toolkit](./wordpress).

## Install a Docker app

1. Point the app's domain to this server's IP address in your DNS, for example `app.example.com` to `203.0.113.10`.
2. In the app store, click the app's **Install** button.
3. Enter the **Domain of the new website** and, if you want, **Other domains**.
4. Some apps also ask for an **Administrator e-mail**. Use a real domain, not `.test` or `.local`.
5. Click **Install**.
6. When the install is done, turn on HTTPS on the new website's **SSL / HTTPS** tab: see [SSL certificates](./ssl). The app restarts with its `https://` address. Some apps need HTTPS to work fully, and the install form says so.

How you get the first administrator account depends on the app, and the install form tells you:

| How | Apps | What you do |
|---|---|---|
| Created by the panel | RustFS, code-server, AdGuard Home, Odoo, Umami, Grafana, ntfy, Gotify, Memos, linkding, Navidrome, Vikunja, Actual Budget, listmonk, Kanboard, FreshRSS, Jellyfin, Mealie | Open **Credentials** on the **Installed** tab. The user is `admin` (Mealie: `changeme@example.com`; Actual Budget has only a server password). |
| Your e-mail and a random password | Open WebUI, Directus, Wiki.js, Healthchecks | Open **Credentials**. |
| The first account registered | Ghost, Uptime Kuma, n8n, Gitea | Register right after the install: the first account becomes the administrator. |
| An invitation | Vaultwarden | Sign up on the app's page with the e-mail address the panel invited. |
| A one-time password | FileBrowser Quantum | Open **Credentials**; the app asks for a new password at the first sign-in. |
| A website password | Stirling-PDF, changedetection.io | The website asks for the user `admin` and the password in **Credentials**. |
| No sign-in | IT-Tools | Nothing to do. |

Notes on some of these apps:

- **Memos** and **Vikunja**: the panel creates the account and then closes sign-ups. You can open them again in the app's settings.
- **ntfy**: nobody can read or send messages without signing in. The `admin` account is part of the app's settings, so its password is the one under **Credentials**.
- **Mealie**: sign in with `changeme@example.com` and the password under **Credentials** (the panel replaced the default password), then set your own e-mail address in your profile.
- **Actual Budget** opens only over HTTPS: get a certificate for its website first.
- **Healthchecks** and **listmonk** send e-mail only through an SMTP service you set up in the app; this server doesn't send mail itself. Healthchecks' alerts to ntfy, Telegram, Slack or webhooks work without it.
- **Navidrome** and **Jellyfin**: see [Music and video folders](#music-and-video-folders).

A Docker app gets:

- a new website that forwards to the app. The website stays closed (503) until the install has finished and the app's default logins are locked;
- containers run as a Docker Compose project, with a memory limit and logs rotated at 10 MB;
- its files in `/var/lib/panel-apps/<name>`, readable only by root: the Compose file, the generated passwords and the containers' data;
- ports published on `127.0.0.1` only, never on the internet;
- images pinned to an exact version.

### Music and video folders

Navidrome and Jellyfin play files from a folder in their website, so that you can add files with the [file manager](./files) or SFTP:

| App | Folder | Seen by the app as |
|---|---|---|
| Navidrome | `/var/www/<domain>/music` | `/music` |
| Jellyfin | `/var/www/<domain>/media` | `/media` |

Nefo creates the folder at the install, owned by the website's Linux user, and gives it to the app read-only: the app can play the files but not change them. Navidrome scans the folder when it starts and then every hour. In Jellyfin, add a library and choose a folder under `/media`.

These files are part of the website, so a backup of the website includes them, and deleting the website deletes them. Converting video on the fly takes a lot of CPU on a small server: prefer files your devices can play as they are.

## Install a service

PostgreSQL, MongoDB and Ollama have no website. Click **Install**: there is nothing to fill in. Nefo picks a free port on `127.0.0.1` and creates the passwords. Apps on this server can reach the service; the internet can't. Each service can be installed once per server.

Open **Credentials** on the **Installed** tab for the address, the user, the password and the **Connection string**.

## The safety checks on Docker apps

A Docker app's recipe in the catalog is data, never a script. Nefo turns it into a Compose file and checks that file with the same rules as your own Compose projects, without exceptions:

- no privileged containers, no added capabilities and no host devices;
- no sharing of the server's network, process or other namespaces;
- no weakened security options and no kernel settings;
- ports published on `127.0.0.1` only;
- no mount of system folders or of the Docker socket;
- a folder in the website (Navidrome's music, Jellyfin's media) is mounted only into a container that runs as the website's own Linux user, and read-only;
- set-up steps that need a password send it in a request body or on the program's standard input, never on a command line.

See [Docker](./docker) for these rules. Passwords are generated for each install, and initial passwords are removed from the container settings after the first start. A downloaded catalog is used only if its signature is valid.

## Manage installed apps

The **Installed** tab lists every installed app. Filter with **All**, **With a website** or **Services**, or search. The columns are **App**, **Version**, **Status** and **Memory** (what the containers use now).

The status is one of **Running**, **Stopped**, **Partly running**, **No containers**, **Installing** or **Install failed**. A PHP app shows **In its website**, or **Website suspended**.

The menu of each row has these actions:

| Action | For | What it does |
|---|---|---|
| **Open** | Apps with a website | Opens the app's website. |
| **Credentials** | Docker apps and services | Shows the sign-in details. Every opening is written to the activity log. |
| **Logs** | Docker apps and services | The newest lines of the containers' logs. |
| **Start**, **Stop**, **Restart** | Docker apps and services | Starts, stops or restarts the containers. |
| **Update** | Docker apps and services, when the catalog has a newer version | See below. |
| **Back up now** | Docker apps and services | Backs up the app. The copy is on the **Backups** page: see [Backups](./backups). |
| **Website settings** | Apps with a website | Opens the website's settings. |
| **Uninstall** | Services | Removes the service: see below. |
| **Delete with its website** | Apps with a website | Opens the website, where you delete it. |

### Update an app

When the catalog has a newer version of a Docker app or service, the **Version** column shows it, for example "2.6.0 available". Click **Update** and confirm. Nefo:

1. backs up the app first. If the backup fails, nothing changes;
2. downloads the new images before it stops anything;
3. restarts the app with them and checks that it answers. If it doesn't, the previous version runs again.

Some apps refuse a one-click update when the major version changes: "This update (…) changes the major version and needs manual steps. See the app's upgrade guide." This applies to PostgreSQL, Wiki.js, Odoo, Grafana, Gotify, Vikunja, Healthchecks, listmonk, Jellyfin and Mealie, and to a minor version change of MongoDB.

PHP apps are not updated from here: "Update it from the app's own admin page." For WordPress, use the [WordPress toolkit](./wordpress).

An app installed before the catalog existed shows **Old install**. **Convert and update** moves its data into the app store's folder, runs it with Docker Compose and updates it, after a backup.

## Uninstall an app

- **A service:** click **Uninstall**, type its name and confirm. Its containers, data and passwords are deleted. Its backups are kept.
- **An app with a website:** delete the website: see [Delete a website](./websites#delete-a-website). For a Docker app this removes its containers, its folder in `/var/lib/panel-apps` and the images no other app uses. **Back up the website first**, ticked by default in the delete dialog, also backs up the app's data, as a backup of the kind **App**. For a PHP app, tick **Also delete its databases** to delete its database too.

::: danger
Uninstalling deletes the app's data for good. Back it up first with **Back up now** if you may need it again.
:::

## Make a second copy of an app

To make a second copy of a Docker app with its data, clone its website: **Websites** → (site) → **Overview** → **Clone…**. See [Clone a website](./websites#docker-apps). n8n, Odoo and apps installed before the App store can't be cloned. A PHP app is cloned with its website and database like any PHP website.
