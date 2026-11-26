---
title: System tools
description: The dashboard, cron jobs, services, background programs, processes and ports, SSH sign-ins, the web terminal, logs, and restarting the panel or the server.
---

# System tools

This page covers the server tools in Nefo: the dashboard, cron jobs, services, background programs, processes and listening ports, SSH sign-ins, the web terminal, where to find logs, and how to restart the panel or reboot the server. Most of them are in the **Server** group of the left menu. Operating system updates, swap, the time zone and host name, TCP BBR and disk cleanup are on the [System](./os) page.

## Dashboard {#dashboard}

The **Dashboard** opens after you sign in.

- The top of the page shows the server's name, operating system, uptime, CPU count and memory. Buttons there let you **Check for updates**, open the **Terminal**, and use the **Power** menu.
- Four gauges show the live **System load**, CPU, **Memory** and **Disk /**. They refresh every 5 seconds. System load is the 1-minute load average divided by the number of CPUs.
- Tiles count websites, databases, SFTP accounts, cron jobs, backups and security risks. Select a tile to open its page.
- **Resource history** draws a chart of CPU, RAM, Disk, Load or Network, over 1 hour, 24 hours or 7 days. Under the chart are the **Now**, **Average** and **Highest** values. For the network, it shows the **In** and **Out** rates and the totals for the time range.
- **Needs attention** lists problems that need you, worst first.
- **Installed software** shows the main components and their state.

The panel records the load every minute and keeps 7 days of history. After a fresh install, the charts appear after a few minutes. The bottom of the left menu also shows the current CPU and RAM use.

### What "Needs attention" checks

- Two-factor sign-in is off.
- Root can sign in over SSH with a password.
- The firewall is off.
- A website's certificate couldn't be renewed, has expired, or expires within 14 days.
- A website expires within 7 days.
- A scheduled backup failed.
- The disk is 90% full or more.
- Security updates of the operating system are waiting (see [Updates](./os#updates)).
- The server needs a restart to finish installing updates.

The bell at the top right (**Notifications**) shows the same list. Both are only visible while you're signed in. To get a message by Telegram or e-mail instead, set up [alerts](./mail-alerts).

## Cron jobs

Cron jobs run commands on a schedule. Go to **Cron jobs** in the left menu.

### Add a cron job

1. Select **Add cron job**.
2. Enter a **Name**, up to 100 characters.
3. Choose who to **Run as**: root, or a website's user (shown with its domain). Run a website's tasks as its own user, and use root only for server tasks.
4. Set the **Schedule**. Pick a common schedule (**Every 5 minutes**, **Every hour**, **Every day at 03:00**, **Every Sunday at 04:00**, **When the server starts**) or type your own. Under the field, the panel describes the schedule in words.
5. Enter the **Command**: a shell command on one line, up to 2000 characters, run with `/bin/sh`.
6. Leave **Enabled** on, and select **Save**.

For a website's user with PHP, the link **Use the WordPress cron command** fills in WordPress's cron, such as `php8.3 /var/www/example.com/public/wp-cron.php`, every 5 minutes.

### Schedule syntax

A schedule has five fields, in server time:

| Field | Values |
|---|---|
| minute | 0–59 |
| hour | 0–23 |
| day of month | 1–31 |
| month | 1–12 |
| day of week | 0–7 (0 and 7 are Sunday) |

In each field, use `*` for every value, a list such as `1,15`, a range such as `1-5`, or a step such as `*/10`. You can also use one of these instead of five fields: `@hourly`, `@daily` (or `@midnight`), `@weekly`, `@monthly`, `@yearly` (or `@annually`) and `@reboot`. The panel refuses numbers out of range, such as minute 60.

| Schedule | Runs |
|---|---|
| `*/5 * * * *` | every 5 minutes |
| `0 3 * * *` | every day at 03:00 |
| `30 2 * * 1` | every Monday at 02:30 |
| `0 0 1 * *` | at midnight on the 1st of every month |

### How cron jobs run

- The panel writes the jobs to `/etc/cron.d/panel`, so the system's cron runs them even when the panel is stopped. Don't edit that file: the panel rewrites it.
- The command's output and errors are added to the job's log, `/var/log/panel-cron/<id>.log`, which belongs to the job's user.
- A `%` in the command works as typed: the panel escapes it for cron.
- A job stops running when its user no longer exists, for example after you delete the website.

### Run, check and change a job

- The switch in the **On** column turns a job on or off. A job that is off stays in the list.
- **Run now** runs the job at once, as its user, and shows the output live. It stops after one hour.
- **Logs**, in the ⋯ menu of the row, shows the end of the job's log. Logs over 5 MB are cut to their last 1 MB.
- **Edit** and **Delete** are in the same menu. Deleting a job also deletes its log.

Above the list, show **All** jobs or only those **On** or **Off**, and search by name, command or user.

## Services

**Services** lists the system services that Nefo manages, if they are installed:

| Service | systemd unit |
|---|---|
| Nefo | `nefo` |
| Nginx, Apache or OpenLiteSpeed | `nginx`, `apache2`, `lshttpd` |
| PHP-FPM, one per installed version | such as `php8.3-fpm` |
| MariaDB | `mariadb` |
| Redis | `redis-server` |
| fail2ban | `fail2ban` |
| SSH | `ssh` |
| Cron | `cron` |
| Firewall (ufw) | `ufw` |

Other services, such as Docker or Postfix, are not listed. Each row shows the state (**Running**, **Stopped**, **Failed**…), how long the service has been running, "does not start at boot" when that applies, and its memory use.

The actions depend on the service:

| Service | Actions |
|---|---|
| Web server, PHP-FPM | **Start**, **Stop**, **Restart**, **Reload** |
| Nefo | **Restart** |
| SSH | **Restart**, **Reload** |
| The others | **Start**, **Stop**, **Restart** |

- **Stop** asks you to confirm, because websites that need the service stop working.
- Before it restarts or reloads the web server, the panel tests its configuration. If the test fails, nothing is restarted and the error is shown.
- SSH can't be stopped from the panel, so you can't lock yourself out.
- Restarting Nefo reloads the page after a few seconds.
- **Logs** shows the last 150 lines of the service's journal.

The panel doesn't change whether a service starts at boot.

## Background programs

Background programs are commands that must keep running, such as a queue worker. The panel runs each one as a systemd service, as a website's Linux user, and starts it again when it stops. You need at least one website first.

### Add a program

1. Go to **Background programs** and select **Add program**.
2. Enter a **Name**, up to 60 characters.
3. Choose the **Website**. The program runs as this website's Linux user, never as root.
4. Optionally, set the **Working folder**: a folder inside the website's folder, such as `app`. Leave it empty for the website's folder itself.
5. Enter the **Command**: one program with its arguments, on one line, up to 1000 characters.
6. Optionally, add **Environment variables**: one `NAME=value` per line, up to 20. On the server, only root can read them.
7. Leave **Start now and when the server starts** on, and select **Create**.

For a PHP website, the link **Use the Laravel queue worker** fills in `php8.3 artisan queue:work --sleep=3 --tries=3 --max-time=3600`, with the website's PHP version.

### How background programs run

- Each program is a systemd service named `panel-daemon-<id>.service`.
- When it exits, for any reason, systemd starts it again after 3 seconds.
- At boot, it starts after MariaDB and Redis.
- It runs with systemd's protections: `/usr`, `/boot` and `/etc` are read-only, `/home` and `/root` are hidden, and it has its own `/tmp`.
- The working folder must exist and stay inside the website's folder, also through symbolic links.
- Its output goes to the system journal.

### Control a program

Each row shows the state (running, restarting, failed, stopping or stopped), how many times it restarted, and its memory use.

- The row shows **Start** (when the program is stopped) or **Restart** (when it runs) as a button. The other actions, **Stop** included, are in the row's ⋯ menu. **Stop** keeps the program stopped, also after a reboot, until you start it again.
- **Logs** shows the last 200 lines the program wrote.
- **Edit** changes the program. **Save** restarts it with the new settings.
- **Delete** stops and removes it.

When you delete a website, its background programs are stopped and removed too.

## Processes and ports

**Processes** shows what runs on the server, in three tabs: **Processes**, **Ports** and **SSH sign-ins**.

### Processes

The list shows the busiest processes, up to 300. Sort by PID, CPU % or memory with the column headings, and filter by name, user or PID. **Refresh every 5 seconds** keeps the list up to date.

CPU % is measured over the last few seconds: 100% is one CPU core working all the time.

To stop a process, select the stop button at the end of its row, then:

- **Stop** asks the program to finish and exit (signal TERM). Well-made programs save their data first.
- **Force stop** ends it at once (signal KILL). Use it only when **Stop** doesn't work: unsaved data is lost.

A service run by systemd may be started again by systemd. The panel can't stop the system's init process (PID 1), the panel itself, the process that started the panel, or kernel threads. Every stop is recorded in the activity log.

### Ports

**Ports** lists the TCP and UDP ports that programs listen on, with the **Address**, the **Program** and where it is **Reachable from**: **this server only**, or **the network**.

A port open to the network can still be closed to the outside by the firewall. See [Firewall](./security#firewall-ufw).

## SSH sign-ins {#ssh-sign-ins}

**Processes** → **SSH sign-ins** lists the SSH sign-ins and failed attempts of the past 30 days, newest first, up to 200. Each row shows the time, the result (**signed in**, **wrong password** or **unknown user**), the user, the IP address and the method, such as `publickey` or `password`.

Show **All**, **Successful only** or **Failed only**. The list is read from the SSH service's journal.

Many failed attempts usually mean someone is guessing passwords. Turn off password sign-in, or install fail2ban to block those addresses. See [Security](./security#brute-force-protection-fail2ban).

## Terminal

**Terminal** opens a root shell (bash) on the server, in your browser.

::: warning
The terminal is like an SSH session as root: every command runs with full rights, and a wrong command can delete data or stop the server. The panel can't undo what you run there.
:::

- The session ends when you leave the page, or after 30 minutes without input. Select **Reconnect** to open a new one.
- Up to 4 terminals can be open at once, container terminals included.
- The activity log records when a terminal was opened and closed, and for how long. It doesn't record the commands.

## Logs

Nefo has no single log page. Each log is shown next to what it belongs to:

| Log | Where |
|---|---|
| A website's access log, error log, PHP errors and app logs | **Websites** → (site) → **Logs** |
| A website's firewall events | **Websites** → (site) → **Web firewall (WAF)** |
| A cron job's output | **Cron jobs** → **Logs** |
| A service's journal | **Services** → **Logs** |
| A background program's output | **Background programs** → **Logs** |
| Containers and Compose projects | **Docker** |
| What was done in the panel | **Account** → **Recent activity** |
| The panel's own service log | `journalctl -u nefo` on the server |

A website's access, error and PHP logs show their last 300 lines. Select **Refresh** in the log window to load new lines.

## Restart the panel or reboot the server

Go to **Settings** → **Panel** → **Power**. The **Dashboard** has the same actions in its **Power** menu.

- **Restart the panel** restarts only the panel. Websites, databases and other services keep running, and the panel is back in a few seconds.
- **Reboot the server** restarts the whole server. Websites and databases are down until it is back, usually one to two minutes. Type the server's name to confirm.

In both cases the page reloads by itself when the panel answers again. Both are refused while a background task runs, such as an install or a backup: wait until it has finished. Both are recorded in the activity log.

On the server, you can also restart the panel with:

```sh
systemctl restart nefo
```
