---
title: Requirements
description: The operating systems, CPU types, memory, ports and firewall settings that Nefo needs.
---

# Requirements

This page lists what your server needs before you [install Nefo](./install): the operating system, the CPU type, memory and disk, and the network ports.

## Operating system

The installer supports these systems:

| Distribution | Versions |
|---|---|
| Ubuntu | 22.04, 24.04, 26.04 |
| Debian | 12, 13 |

On any other system the installer stops and says that the system "is not supported yet". You can set `NEFO_FORCE=1` to try anyway (see [Install](./install#environment-variables)), but website management needs `apt-get`, `systemctl` and `useradd`. Without them the **Websites** page shows "Website management needs a Debian or Ubuntu server with systemd, so it is not available on this machine."

The installer also needs:

- root access (sign in as root, or use `sudo`);
- systemd;
- `curl` and `sha256sum`;
- `openssl`, to check the signature of a release.

If `curl` is missing, install it first with `apt install curl`.

## CPU

Nefo runs on 64-bit x86 (`x86_64`, also called amd64) and 64-bit ARM (`aarch64`, also called arm64). On any other CPU the installer stops with "unsupported CPU architecture".

## Memory and disk

Nefo is built for small VPSes with 1 GB of RAM. The panel itself is one program, and you install only the components you need. The **Software** page shows roughly how much memory each component uses. For example, the web servers use about 15 MB (Nginx), 26 MB (OpenLiteSpeed) or 40 MB (Apache) while idle.

One component needs much more memory: the optional ClamAV antivirus needs about 1.4 GB of free memory while it scans, so it suits servers with 2 GB or more.

On a server with 1 GB of memory, add 1 to 2 GB of swap, so that a busy moment doesn't make the kernel stop MariaDB or PHP. Many VPS images come without swap. **System** → **Swap** creates a swap file in one click (see [Swap](./os#swap)). In a container (OpenVZ, LXC), the kernel doesn't allow swap: ask your provider.

The disk space you need depends on your websites, databases and backups. Backups kept on the server are stored in `/var/backups/panel`, on the same disk as your websites. The dashboard shows disk use, and the panel warns you when the disk is 90% full.

## A fresh server

Install Nefo on a freshly installed server. The installer doesn't check what else is installed, but the panel sets up the web server, PHP, MariaDB, the firewall and SSH itself:

- It runs one web server (Nginx, Apache or OpenLiteSpeed). It refuses to install one while another web server is already on the server, because both need ports 80 and 443.
- It writes its own configuration for the software it manages. Don't install Nefo next to another control panel.

The installer refuses to run a second time on the same server: when `/etc/nefo/config.json` exists, it stops with "the panel is already installed".

## Ports

| Port | Used by | When |
|---|---|---|
| A random TCP port from 20000 to 49999 | The panel | From the installation. The installer prints it. |
| 80 and 443 (TCP) | The web server | Once you install a web server from the panel |
| 443 (UDP) | HTTP/3 on OpenLiteSpeed | When OpenLiteSpeed is the web server |
| Your SSH port (22 by default) | SSH | Always |
| 3306 (TCP) | MariaDB | Only if you allow remote access to a database, and only for the addresses you list |

Some services listen on the server itself only (127.0.0.1), so you don't open them: the receiver for Git push webhooks (port 7809, reached through the web server), the ports of Docker containers, and the local mail relay.

## Firewalls

### The firewall on the server (ufw)

- If ufw is active when you install, the installer opens the panel's port in it (or in firewalld, if that is running instead).
- When you install a web server from the panel and ufw is active, the panel opens ports 80 and 443. The panel doesn't manage firewalld: if the server uses it, open these ports yourself.
- The **Firewall & SSH** page manages ufw. If ufw isn't installed, the page says "ufw is not installed on this server". Install it with `apt install ufw`.

When you turn on the firewall from the panel, it keeps SSH and the panel's port open, and ports 80 and 443 when a web server is installed. See [Security](./security).

### Your provider's cloud firewall

Many providers have a firewall outside the server, often called a security group or a cloud firewall. The installer can't change it. Allow these there yourself:

1. Your SSH port.
2. The panel's TCP port, which the installer prints at the end.
3. TCP ports 80 and 443, once you host websites. Let's Encrypt checks your domains over port 80, so certificates can't be issued while it is closed.

## A domain name

You don't need a domain to install Nefo: you open the panel by the server's IP address. You need one for each website, and you can give the panel its own domain and a free certificate (see [First steps](./first-steps#give-the-panel-its-own-domain)). Point the domain's A record to the server's IP address.
