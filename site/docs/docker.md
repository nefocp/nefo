---
title: Docker
description: Run containers and Compose projects, open a terminal in a container, manage images, and put a container on a website behind a reverse proxy.
---

# Docker

Nefo installs Docker from your distribution and manages containers, Compose projects and images on the **Docker** page, with a terminal in each container. It publishes container ports on 127.0.0.1 only, so you reach them through a website, and refuses Compose projects that would weaken the server, such as privileged containers or ports open to the internet.

This page covers each tab, the safety checks the panel runs on Compose projects, how the App store's Docker apps fit in, and how to make a container public through a website.

## Install Docker

Go to **Software** → **Docker** and select **Install**. The panel installs `docker.io` from your distribution, with its Compose plugin when one is available. The **Docker** menu item appears once Docker is installed.

If the Compose plugin is missing later, the panel installs it the first time you start a Compose project.

To uninstall Docker, see [Software](./software#when-an-uninstall-is-refused). It is refused while containers, Compose projects or Docker apps exist.

## Why ports stay on 127.0.0.1

Ports that Docker publishes bypass the ufw firewall: a container published on all addresses is open to the internet, whatever ufw says. So the panel:

- publishes the containers it starts on `127.0.0.1` only;
- refuses Compose projects that publish a port on any other address.

To make a container public, put a website in front of it. See [Put a container on a website](#put-a-container-on-a-website).

## Containers

The **Containers** tab lists every container on the server, running or not, with its image, state and ports. Containers of Compose projects and of App store apps appear here too.

Each container's row has a **Start** or **Stop** button, and the other actions are in its menu:

| Action | What it does |
|---|---|
| **Start** / **Stop** | Starts or stops the container |
| **Terminal** | Opens a shell inside the container. Only for running containers |
| **Restart** | Restarts the container |
| **Logs** | Shows the last 200 lines of its output, with timestamps |
| **Delete** | Removes the container, even while it runs |

::: warning
**Delete** loses the container's data unless it is stored in a volume.
:::

### Run a container

1. On the **Containers** tab, select **Run a container**.
2. Fill in:
   - **Image**, such as `nginx:alpine`;
   - **Container name**, such as `my-app`;
   - **Port on this server** (1024 to 65535) and **Port in the container**, if the container serves something;
   - **Environment variables (one NAME=value per line)**, such as `TZ=UTC`.
3. Select **Start**.

The panel downloads the image if needed and starts the container with the restart policy `unless-stopped`, so it comes back after a reboot. The port is published on `127.0.0.1` only.

### Terminal in a container

Select **Terminal** to open a shell inside a running container. The panel uses `bash` when the image has it, and `sh` otherwise. The session ends when you close the window or after 30 minutes without input.

## Compose projects

A Compose project is a `compose.yaml` that runs several containers together. The panel keeps each project in `/var/lib/panel-compose/<name>/compose.yaml`, readable by root only, and runs it as the Compose project `panel-<name>`.

### Create a project

1. Go to **Docker** → **Compose** and select **New project**.
2. Enter a **Project name**: lowercase letters, digits and dashes, up to 40 characters. Containers are named `panel-<name>-<service>-1`.
3. Write the file in the `compose.yaml` field. **Insert example** fills in a small working example.
4. Select **Create and start**.

The panel checks the file, saves it and starts the containers, pulling the images it needs. Relative paths in the file, such as `./data:/data`, mean the project's folder.

A minimal project:

```yaml
services:
  web:
    image: traefik/whoami
    restart: unless-stopped
    ports:
      - "127.0.0.1:8081:80"
```

`compose.yaml` can be up to 64 KB.

### Edit a project

Open the menu of a project and select **Edit**. Change the file and select **Save and apply**. The panel checks the new file, saves it, and recreates the containers whose configuration changed. Containers of services you removed from the file are removed too.

### Project actions

| Action | What it does |
|---|---|
| **Start** | Checks the file and starts the containers |
| **Stop** | Stops the containers |
| **Restart** | Restarts the containers |
| **Update images** | Checks the file, pulls newer images and recreates the containers |
| **Remove containers** | Stops and removes the containers. Volumes and `compose.yaml` are kept |
| **Logs** | The last 200 lines of each container |
| **Delete** | Removes the containers, networks and the project's folder with `compose.yaml`. Tick **Also delete its volumes (the data in them is lost)** to delete volumes too. Images are kept |

### Safety checks

Before it saves a project, and before every start and update, the panel asks Compose for the final configuration, with includes, extends and variables resolved. It refuses a project that would weaken the server:

| Refused | Why |
|---|---|
| `privileged: true` | A privileged container has full control of the server |
| `network_mode`, `pid`, `ipc`, `userns_mode` or `cgroup` set to `host` | The container would share the server's own namespace |
| Any `cap_add` | Extra container privileges |
| Any `devices` or `device_cgroup_rules` | Host devices given to a container |
| `security_opt` other than `no-new-privileges` | Options such as `seccomp=unconfined` or `apparmor=unconfined` weaken the container's confinement |
| Any `sysctls` | Kernel settings |
| A port without `127.0.0.1` or `::1` as its host address, such as `"8080:80"`, or a port with no published number | Docker would listen on every address, past the firewall |
| A bind mount of `/etc`, `/root`, `/boot`, `/proc`, `/sys`, `/dev`, or anything inside them | System folders give a container control of the server |
| A bind mount of the Docker socket, `/var/run/docker.sock` or `/run/docker.sock`, or of a folder that contains it, such as `/`, `/var`, `/var/run` or `/run` | The socket gives a container control of Docker, and so of the server |
| A named volume whose `device` option is such a path, and secrets or configs read from such a path | The same folders, mounted another way |

Paths are checked as written and after following symbolic links. `/etc/localtime` and `/etc/timezone` may be mounted read-only (`:ro`), so a container can use the server's time zone.

When a check fails, the editor shows which service and setting were refused. Nothing is saved or started.

## Images

The **Images** tab lists the local images with their size, creation date, and the containers that use them (**Used by**).

- **Remove** deletes an image. If containers use it, the panel names them and asks again before it removes the image anyway.
- **Remove unused** deletes every image that no container uses. They are downloaded again when needed.

There is no separate pull button. Images are downloaded when you run a container, start a Compose project, or select **Update images**.

## Docker apps from the App store

The [App store](./apps) installs Docker apps such as Ghost, Uptime Kuma, n8n and Gitea. They run as Compose projects that the panel writes from each app's recipe:

- Their files and data are in `/var/lib/panel-apps/<name>`, and the Compose project is called `panelapp-<name>`. They don't appear on the **Compose** tab, but their containers appear on the **Containers** tab.
- They go through the same safety checks as your own projects.
- Each Docker app gets its own website. Service apps such as PostgreSQL, MongoDB and Ollama have no website and listen on `127.0.0.1` only.

Manage these apps from **App store** → **Installed**, where you start, stop, update and back them up. See [App store](./apps) for removing them, and [Backups](./backups#app-backups) for their backups.

## Put a container on a website

A website can forward its visitors to a container's port on `127.0.0.1`. The web server then acts as a reverse proxy, and you can give the site an SSL certificate.

1. Publish the container's port on `127.0.0.1`, for example `127.0.0.1:8081:80` in `compose.yaml`, or **Port on this server** `8081` in **Run a container**.
2. Go to **Websites** and select **Add website**.
3. Enter the **Domain**, such as `app.example.com`.
4. In **Type**, choose **App (Node.js, Python, Java…) behind a proxy**.
5. In **Port the app listens on (127.0.0.1)**, enter the same port, `8081`.
6. Leave **Start command (optional)** empty: Docker runs the app.
7. Select **Create website**.

The port must be between 1024 and 65535, and not the panel's port, 3306 (MariaDB), 6379 (Redis) or 7809 (the Git webhook listener). Two app websites can't use the same port.

Then get a certificate for the site, see [SSL certificates](./ssl). See [Websites](./websites) for the site's other settings.

The **Reverse proxy to another address** type also works for a container: it forwards to any address, such as `http://127.0.0.1:8081`, with options for the Host header, WebSocket and a time limit. See [Reverse proxy websites](./websites#reverse-proxy-websites).
