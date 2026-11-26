---
title: System
description: Install operating system updates, turn on automatic security updates, add swap, set the time zone, clock synchronization and host name, turn on TCP BBR and free disk space.
---

# System

The **System** page, in the **Server** group of the left menu, looks after the server itself. It has five tabs: **Updates**, **Swap**, **Time & name**, **Network** and **Cleanup**. Every change on this page is recorded in the activity log (**Account** → **Recent activity**).

Long changes, such as installing updates or creating a swap file, run as a background task: the page shows their progress, and only one task runs at a time.

## Updates

The **Updates** tab lists the operating system's pending updates: each package, its installed and new version, and where the new version comes from (the suite, such as `noble-updates`). Updates from a security suite (`noble-security`, `bookworm-security`…) are marked **Security** and listed first. A package marked **new** is installed with an update, such as a new kernel.

### How the panel checks for updates

- The panel checks on its own: every hour it looks at its last check, and when that is older than 6 hours, it runs `apt-get update` and asks apt what an upgrade would install. It skips the check while another task runs.
- Opening the page never runs apt: it shows the last check, with its time.
- **Check now** checks at once, as a task.
- **Held back for now** lists updates that apt doesn't install yet: packages it keeps back because they would need other changes, and Ubuntu's phased updates, which reach servers over a few days.

When security updates are waiting, **Needs attention** on the dashboard and the bell show "N security updates are available".

### Install updates

- **Install all updates** runs `apt-get upgrade --with-new-pkgs`. It installs new packages that an update needs, such as a new kernel, and never removes a package.
- **Install security updates only** updates only the installed packages whose new version comes from a security suite (`apt-get install --only-upgrade` with their names).

Both refresh the package lists first, run without questions, and keep the configuration files you changed. The list is checked again when they are done.

::: warning Services restart for a moment
Updating Nginx, Apache, OpenLiteSpeed, PHP, MariaDB or Redis restarts that service, so websites may not answer for a few seconds. Updating Docker restarts it and its containers. The page names the services that the pending updates restart, and so does the confirmation. Install such updates at a quiet time.
:::

### Restart required

Some updates, such as a new kernel or a new C library, are only complete after the server restarts. The tab says "The server needs a restart to finish installing updates." when:

- `/var/run/reboot-required` exists (Ubuntu writes it, and lists the packages in `/var/run/reboot-required.pkgs`), or
- a newer kernel than the running one is installed in `/boot` (Debian doesn't always write the file).

**Restart in Settings** opens **Settings** → **Panel** → **Power**, where you can reboot the server. The bell also shows "The server needs a restart" until you do.

### Automatic security updates

The **Automatic security updates** switch lets the server install security updates by itself, every day:

- If `unattended-upgrades` isn't installed, the panel installs it first, as a task. Its helper that waits for the server to shut down uses about 20 MB of memory.
- The panel sets `APT::Periodic::Update-Package-Lists "1";` and `APT::Periodic::Unattended-Upgrade "1";` in `/etc/apt/apt.conf.d/20auto-upgrades`, keeping the file's other lines, and turns on apt's daily timers.
- Turning the switch off sets `APT::Periodic::Unattended-Upgrade "0";`.

Which updates are installed is decided by unattended-upgrades' own settings in `/etc/apt/apt.conf.d/50unattended-upgrades`: on Ubuntu and Debian, only security updates. It doesn't reboot the server unless you turn on `Unattended-Upgrade::Automatic-Reboot` there. If another file in `/etc/apt/apt.conf.d` overrides the setting, the panel says so and leaves the file as it was.

## Swap

Swap is disk space that the kernel uses when memory runs out. On a server with 1 or 2 GB of memory, 1 to 2 GB of swap keeps MariaDB and PHP from being stopped by the kernel when a busy moment needs more memory than the server has.

The **Swap** tab shows the memory, the swap in use, and each swap file or partition from `/proc/swaps`.

### Create a swap file

1. Choose the **Size**: 512 MB, 1 GB, 2 GB, 4 GB or 8 GB. Only sizes that leave 1 GB of disk free are offered.
2. Select **Create a swap file**.

The panel creates `/panel.swap`, readable only by root, with `fallocate`. Where the file system doesn't support that, or `swapon` refuses such a file, it writes the file with `dd` instead, which takes about a minute per few gigabytes. On btrfs, the file is first marked not copy-on-write. Then the panel runs `mkswap` and `swapon`, and adds `/panel.swap none swap sw 0 0` to `/etc/fstab`, so the swap is on again after a restart.

The panel refuses to create a swap file:

- when the disk doesn't have the file's size plus 1 GB free;
- when the server already has a swap file that the panel didn't create, such as `/swapfile` or `/swap.img`. The panel leaves it as it is. A swap partition doesn't stop it;
- on ZFS, where swap files don't work;
- in a container (OpenVZ, LXC, Docker), where the kernel doesn't allow `swapon`: "This server does not allow swap". Ask your provider for swap, or use a full virtual server.

If something fails, the panel removes the file again.

### Remove the swap file

**Remove the swap file** turns the swap off with `swapoff`, removes its line from `/etc/fstab` and deletes `/panel.swap`. What is in swap moves back into memory first, so the panel refuses when it wouldn't fit in the free memory.

### Swappiness

**Swappiness** (`vm.swappiness`, from 0 to 100) sets how readily the kernel moves memory to swap. The default is 60; 10 suits a server, which then uses swap only when memory is short. The panel writes the value to `/etc/sysctl.d/90-panel-swap.conf` and applies it at once.

## Time and host name {#time-and-host-name}

The **Time & name** tab shows the server's time, its time zone and whether the clock is synchronized.

### Time zone

Choose a time zone from the list (`timedatectl list-timezones`) and select **Save**. The panel sets it with `timedatectl set-timezone` and restarts cron, so cron jobs follow the new time zone. Other services that were already running, such as MariaDB, use the new time zone after they restart.

### Clock synchronization

**Synchronize the clock (NTP)** turns time synchronization on or off (`timedatectl set-ntp`). Keep it on: certificates and two-factor codes need a correct clock. It needs a time service such as `systemd-timesyncd`. Without one, the switch is greyed out: install it with `apt install systemd-timesyncd`.

### Host name

Enter the new **Host name** and select **Save**. A host name has letters, digits and hyphens, with dots between parts, at most 64 characters, and can't start or end with a hyphen: `web1` or `web1.example.com`.

The panel:

1. sets it with `hostnamectl set-hostname`;
2. points the `127.0.1.1` line of `/etc/hosts` at it (the full name and its first part), or adds that line;
3. on a server with cloud-init, writes `/etc/cloud/cloud.cfg.d/99-panel-hostname.cfg` with `preserve_hostname: true`, so cloud-init doesn't put the provider's name back at the next boot.

The host name doesn't change the panel's address or any certificate. To reboot the server from **Settings**, type the new name.

## Network: TCP BBR {#network-tcp-bbr}

The **Network** tab shows the TCP congestion control in use (usually `cubic`), the default queue (`net.core.default_qdisc`, usually `fq_codel`) and the algorithms the kernel has.

BBR is a congestion control from Google. It often makes websites and downloads faster over long or lossy connections, such as to visitors on another continent or on mobile networks. Turning on **TCP BBR**:

1. loads the `tcp_bbr` module, when the kernel has it as a module;
2. writes `/etc/modules-load.d/panel-bbr.conf` (`tcp_bbr`) and `/etc/sysctl.d/90-panel-bbr.conf` (`net.core.default_qdisc = fq` and `net.ipv4.tcp_congestion_control = bbr`);
3. runs `sysctl --system`.

New connections use BBR at once. The `fq` queue applies to the network cards after the next restart; BBR works well without it in the meantime.

The panel refuses when the running kernel has no BBR, and in a container, where the kernel settings can't be changed. It then keeps nothing. Turning BBR off deletes both files and goes back to `cubic` and `fq_codel`. When BBR was set up outside the panel, the switch is greyed out.

## Disk cleanup

The **Cleanup** tab shows how much space each cleanup frees. **Clean** asks you to confirm, runs the cleanup as a task, and then shows the space freed.

| Item | What is removed |
|---|---|
| **Downloaded packages** | Package files that apt keeps in `/var/cache/apt/archives` after installing (`apt-get clean`). |
| **System journal** | Old entries of the system journal, down to 100 MB (`journalctl --rotate`, then `journalctl --vacuum-size=100M`). |
| **Old logs** | Logs in `/var/log` that logrotate has already rotated: names ending in `.gz` or a number, such as `syslog.1` or `auth.log.2.gz`. |
| **Unused packages** | Packages that nothing needs any more, such as old kernels (`apt-get autoremove`). |
| **Docker** | Images that no container uses, and the build cache (`docker image prune --all`, `docker builder prune --all`). Shown only when Docker is installed. |

- **Old logs** keeps the web server logs in `/var/log/nginx`, `/var/log/apache2` and `/var/log/openlitespeed`, because the websites' traffic statistics read them. It only deletes files in root's own folders and never follows links.
- For **Unused packages**, the confirmation lists the packages. If apt would remove anything else by the time you confirm, nothing is removed: "The list of unused packages has changed".
- **Docker** never removes containers, stopped or not, or volumes, where containers keep their data.

## Activity log

These actions appear in **Account** → **Recent activity**: installed system updates, turned automatic security updates on or off, created or removed the swap file, changed swappiness, changed the time zone, turned clock synchronization on or off, changed the host name, turned TCP BBR on or off, and cleaned up disk space.
