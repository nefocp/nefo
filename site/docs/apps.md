---
title: App store
description: Install PHP apps into a website, Docker apps behind their own website and database services, and manage, update, back up and remove them.
---

# App store

The app store installs ready-made applications in one click. Nefo creates what each one needs, such as a database, a website, passwords and containers, and checks the server's memory and disk first. This page covers the kinds of apps, the catalog, installing, and managing the apps you installed.

Open **App store** in the left menu. The **Catalog** tab lists the apps and the **Installed** tab the apps on this server. On the **Websites** page, **Install from the App store** leads here too.

## Three kinds of apps

| Kind | Where it goes | What it gets |
|---|---|---|
| **PHP app** | Into an empty PHP website that you created | Its own database, its files in the website, the right rewrite rules. It runs as the website's Linux user. |
| **Docker app** | Into a new website that Nefo creates for it | Containers that listen on `127.0.0.1` only, with the website in front of them. |
| **Service** | No website | Containers that listen on `127.0.0.1` only, for your own sites and apps. One of each per server. |

PHP apps and Docker apps need the web server installed: see [Websites](./websites). Docker apps and services also need Docker. Install it on the **Software** page; **Install Docker** in the app store opens that page. See [Docker](./docker).

## The catalog

At catalog version 2026112501, the app store offers these apps. The minimum memory is the server's total RAM below which Nefo refuses the install (swap doesn't count).

| App | Kind | What it is | Minimum RAM |
|---|---|---|---|
| WordPress | PHP app | Website and blog builder | 450 MB |
| WooCommerce | PHP app | WordPress with WooCommerce: an online shop | 900 MB |
| Joomla | PHP app | CMS for websites and portals | 450 MB |
| Nextcloud | PHP app | Your own cloud for files, calendars and contacts | 900 MB |
| Drupal | PHP app | CMS for structured content, built with Composer | 900 MB |
| Laravel | PHP app | A new Laravel project with its database | 450 MB |
| OpenCart | PHP app | Shopping cart | 900 MB |
| Moodle | PHP app | Learning platform | 900 MB |
| Ghost | Docker app | Blogs and newsletters | 900 MB |
| Uptime Kuma | Docker app | Watches your sites and services | 450 MB |
| n8n | Docker app | Workflow automation | 900 MB |
| Gitea | Docker app | Your own Git server | 450 MB |
| Open WebUI | Docker app | Chat interface for Ollama or OpenAI-compatible APIs | 1800 MB |
| Vaultwarden | Docker app | Password manager server for the Bitwarden apps | 450 MB |
| RustFS | Docker app | S3-compatible object storage | 900 MB |
| FileBrowser Quantum | Docker app | Files in the browser | 450 MB |
| code-server | Docker app | VS Code in the browser | 900 MB |
| Stirling-PDF | Docker app | PDF tools | 1800 MB |
| IT-Tools | Docker app | Developer tools in the browser | 230 MB |
| Directus | Docker app | Headless CMS | 900 MB |
| Wiki.js | Docker app | Wiki | 900 MB |
| Umami | Docker app | Privacy-friendly website analytics | 900 MB |
| AdGuard Home | Docker app | Ad blocking through your own DNS-over-HTTPS address | 450 MB |
| Odoo | Docker app | ERP (Community edition) | 1800 MB |
| PostgreSQL | Service | PostgreSQL database server | 900 MB |
| MongoDB | Service | MongoDB document database | 900 MB |
| Ollama | Service | Runs open language models on the CPU | 3584 MB |

The catalog is built into the panel. Nefo also checks once a day for a newer catalog published with the releases, and uses it only if its signature is valid. **Update the catalog** checks now. The header shows the catalog version and whether it is "built into the panel" or "downloaded, signature checked".

Each card shows the app's version, kind, memory and disk needs. Its **Install** button opens the description, licence, needs and notes, and the install form. A card that the server can't run shows why, and **Details** instead. An installed service shows **Manage**. A "Source-available" badge marks a licence that is not OSI-approved.

## The checks before an install

When you open an app, Nefo compares its needs with the server. It refuses the install when:

- the server's total memory is below the app's minimum;
- the free disk space is less than the app's images and data plus 1 GB;
- the processor's architecture or features don't suit the app.

It warns, but lets you install, when the server has less memory than recommended, when less memory is free right now than the app may use, or when the memory limits of all installed apps together exceed the server's memory.

## Install a PHP app

1. Create an empty PHP website with a PHP version the app supports: see [Websites](./websites). The minimum PHP versions are WordPress 7.4, Nextcloud and Laravel 8.2, WooCommerce, Joomla, Drupal and Moodle 8.3, OpenCart 8.0. OpenCart supports PHP up to 8.3 and Moodle up to 8.4.
2. Make sure MariaDB is installed and running: see [Databases](./databases).
3. In the app store, click the app's **Install** button.
4. Choose the **Website**. The list shows only empty PHP websites with a suitable PHP version. If there is none, use **Add a PHP website**.
5. For apps that ask for it, fill in **Site name**, **Administrator** (avoid `admin`: it is the first name attackers try), **Administrator e-mail** and **Administrator password**. Leave the password empty for a strong random one; a password you type needs 12 to 128 characters. WooCommerce also asks for the store country and currency.
6. Click **Install**. The install runs in the background and shows in the task list.

When it is done, Nefo shows the **Sign-in address**, the **Administrator** and the **Administrator password** once. Save them now.

The website's `public` folder must be empty; the panel's placeholder page is fine. Drupal and Laravel are built with Composer, and Moodle needs the whole site folder too: their website must be new and empty, and the site folder becomes the project, with `public` as its web folder.

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
| Created by the panel | RustFS, code-server, AdGuard Home, Odoo, Umami | Open **Credentials** on the **Installed** tab. |
| Your e-mail and a random password | Open WebUI, Directus, Wiki.js | Open **Credentials**. |
| The first account registered | Ghost, Uptime Kuma, n8n, Gitea | Register right after the install: the first account becomes the administrator. |
| An invitation | Vaultwarden | Sign up on the app's page with the e-mail address the panel invited. |
| A one-time password | FileBrowser Quantum | Open **Credentials**; the app asks for a new password at the first sign-in. |
| A website password | Stirling-PDF | The website asks for the user `admin` and the password in **Credentials**. |
| No sign-in | IT-Tools | Nothing to do. |

A Docker app gets:

- a new website that forwards to the app. The website stays closed (503) until the install has finished and the app's default logins are locked;
- containers run as a Docker Compose project, with a memory limit and logs rotated at 10 MB;
- its files in `/var/lib/panel-apps/<name>`, readable only by root: the Compose file, the generated passwords and the containers' data;
- ports published on `127.0.0.1` only, never on the internet;
- images pinned to an exact version.

## Install a service

PostgreSQL, MongoDB and Ollama have no website. Click **Install**: there is nothing to fill in. Nefo picks a free port on `127.0.0.1` and creates the passwords. Apps on this server can reach the service; the internet can't. Each service can be installed once per server.

Open **Credentials** on the **Installed** tab for the address, the user, the password and the **Connection string**.

## The safety checks on Docker apps

A Docker app's recipe in the catalog is data, never a script. Nefo turns it into a Compose file and checks that file with the same rules as your own Compose projects, without exceptions:

- no privileged containers, no added capabilities and no host devices;
- no sharing of the server's network, process or other namespaces;
- no weakened security options and no kernel settings;
- ports published on `127.0.0.1` only;
- no mount of system folders or of the Docker socket.

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

Some apps refuse a one-click update when the major version changes: "This update (…) changes the major version and needs manual steps. See the app's upgrade guide." This applies to PostgreSQL, Wiki.js and Odoo, and to a minor version change of MongoDB.

PHP apps are not updated from here: "Update it from the app's own admin page." For WordPress, use the [WordPress toolkit](./wordpress).

An app installed before the catalog existed shows **Old install**. **Convert and update** moves its data into the app store's folder, runs it with Docker Compose and updates it, after a backup.

## Uninstall an app

- **A service:** click **Uninstall**, type its name and confirm. Its containers, data and passwords are deleted. Its backups are kept.
- **An app with a website:** delete the website: see [Websites](./websites). For a Docker app this removes its containers, its folder in `/var/lib/panel-apps` and the images no other app uses. For a PHP app, leave **Also delete the databases linked to this website** ticked to delete its database too.

::: danger
Uninstalling deletes the app's data for good. Back it up first with **Back up now** if you may need it again.
:::
