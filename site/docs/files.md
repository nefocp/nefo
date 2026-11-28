---
title: Files
description: Browse, edit, upload, download, search, compress and extract files in the browser, and restore deleted items from the trash.
---

# Files

Nefo's **Files** page is a file manager in the browser: browse, edit text files of up to 2 MB, upload up to 4 GB at a time, download, fetch a file from a URL, search by name or content, compress and extract, and restore deleted items from a 30-day trash. Inside a website's folder it works as the website's user and can't leave the folder.

This page covers each of these. The panel runs as root, so it also explains where you can go and what it refuses to change.

## Find your way

The file manager opens at `/var/www`, where each website has its folder, `/var/www/example.com`.

- Select a folder's name to open it. **Up** goes to the parent folder and **Refresh** reads the folder again.
- Each part of the path at the top is clickable. The house button (**Websites**) takes you back to `/var/www`.
- To jump to any folder, select the pencil button (**Type a path**), type an absolute path such as `/etc/nginx`, and select **Go**.
- The table shows **Name**, **Size**, **Permissions**, **Owner** and **Modified**. A folder with more than 5,000 items shows only the first 5,000.

## What you can reach

Outside website folders you work as root on the whole file system.

Inside a website's folder, `/var/www/example.com`, every operation is confined to that folder, and a badge with the domain appears next to the path:

- Paths can't climb out with `..`, and symbolic links can't point outside the folder. A hacked site that plants a link to `/etc` can't trick the panel into touching system files.
- New files and folders belong to the website's user, so its PHP can use them.
- Moving, compressing or extracting between a website's folder and any other folder is refused. Download the file and upload it where you need it instead.
- While the website's [files are locked](./website-protection#lock-a-website-s-files), you can change only its writable folders and its `tmp` and `logs` folders. Elsewhere in the website's folder, the file manager refuses with a message that the files are locked.

These folders can't be deleted, renamed or have their permissions changed:

- `/`, `/bin`, `/boot`, `/dev`, `/etc`, `/home`, `/lib`, `/lib64`, `/opt`, `/proc`, `/root`, `/run`, `/sbin`, `/srv`, `/sys`, `/tmp`, `/usr`, `/var`;
- `/var/www`, `/var/lib`, `/var/log`, `/var/lib/nefo` and `/etc/nefo`;
- each website's own folder, such as `/var/www/example.com` (the files inside it can be changed);
- the trash folder, `/var/lib/panel-trash`, and the folders that hold it.

## Edit a file

1. Select the file's name. It opens in the editor.
2. Make your changes. Ctrl+S saves, and the Tab key inserts a tab.
3. Select **Save**, then **Close**.

The editor opens text files of up to 2 MB. Binary files and larger files are refused: download them instead. A saved file keeps its permissions and owner. If you close the editor with unsaved changes, the panel asks first.

To create a file, select **New file**, enter a name in **Name of the new file** and select **OK**. To create a folder, select **New folder**. New files get permissions `0644` and new folders `0755`.

## Upload files

1. Open the folder you want to upload into.
2. Select **Upload** and choose one or more files.
3. If a file with the same name exists, the panel asks before it replaces it.

A progress bar shows the upload. One upload can be up to 4 GB in total. Uploaded files get permissions `0644`. Inside a website folder they belong to the website's user, elsewhere to root.

## Download a file

Open the menu of a file and select **Download**. Only files can be downloaded. To download a folder, compress it first.

## Download from a URL

The server can fetch a file from the internet straight into a folder, without going through your computer.

1. Open the target folder and select **Download from URL**.
2. Enter the **File address (URL)** and select **Download**.
3. A log shows the progress. You can select **Hide (the download goes on)**.

The rules:

- Only `http` and `https` addresses, with at most 5 redirects.
- Files of up to 2 GB.
- The panel never downloads from the server's own network: loopback, private and link-local addresses (such as `127.0.0.1`, `10.x.x.x` and `169.254.169.254`), and the server's own addresses, are refused. This is checked again for every redirect.
- The file name comes from the server or from the URL. An existing file is never replaced: the panel adds ` (1)`, ` (2)` and so on.
- The file belongs to the folder's owner, with permissions `0640`.

## Search

1. Open the folder to search in.
2. Type in the search box (**Search this folder**) and press Enter.
3. In the dialog, adjust **Name contains**. To search text inside files, tick **Also search inside files** and fill in **Text in the file**.
4. Select **Search**.

Upper and lower case are treated the same. Select a result to open its folder. A text file also opens in the editor.

Limits keep one search from tying up the server:

| Limit | Value |
|---|---|
| Items looked at | 20,000 |
| Results | 200 |
| Time | 20 seconds |
| Text search | Skips binary files and files over 2 MB |
| Query length | 200 characters |

Symbolic links are listed but never followed. A search from `/` skips `/proc`, `/sys` and `/dev`. When a limit is reached, the panel says so: search a smaller folder or use more precise words.

## Compress and extract

### Compress

1. Tick the files and folders to pack.
2. Select **Compress** in the selection bar.
3. Enter the **Archive name (.zip or .tar.gz)** and select **OK**.

The archive is created in the current folder. Symbolic links and special files are left out.

### Extract

1. Open the menu of an archive and select **Extract**.
2. The field holds the current folder. Change it to extract somewhere else, then select **OK**.

The panel extracts `.zip`, `.tar.gz`, `.tgz` and `.tar` archives:

- Existing files with the same name are overwritten.
- Symbolic links and special files in the archive are skipped.
- Files get permissions `0644`, or `0755` when they were executable. They belong to the website's user inside a website folder, and to root elsewhere.
- Archives with more than 20 GB of content or more than 500,000 files are refused.

Compressing and extracting run as tasks with a live log, so they go on if you leave the page.

## Rename or move

Open the menu of an item and select **Rename**, then enter the new name and select **OK**. The new name is read relative to the current folder, so a path such as `old/index.html` or `../index.html` moves the item within the same website or the same part of the server.

## Change permissions

Select the permissions of an item in the **Permissions** column, or open its menu and select **Permissions**. Enter the new value in octal, for example `0644` for files or `0755` for folders, and select **OK**.

Values go up to `0777`. Setuid, setgid and sticky bits can't be set here.

The **Owner** column shows the owner and group. The file manager doesn't change owners. Use the [terminal](./system#terminal) for that, for example `chown -R example_com:example_com /var/www/example.com/public` with the website's own user.

## Delete files and the trash

1. Tick the items, or open the menu of one, and select **Delete**.
2. By default the items go to the trash: select **Move to trash**.
3. To skip the trash, tick **Delete permanently (skip the trash)** and select **Delete permanently**.

::: danger
Permanently deleted items, including folders with all their content, can't be recovered.
:::

### Restore from the trash

1. Open a folder of the website whose files you want back, or any folder outside the websites for the rest of the server.
2. Select **Trash**.
3. Select **Restore** next to the item.

Each website has its own trash, and the rest of the server shares one. The item goes back to the path it came from, with its owner and permissions. Missing parent folders are created. If something else is at that path now, restore is refused: move it away first.

In the trash you can also **Delete** one item for good, or select **Empty trash**. The list shows the 1,000 most recently deleted items.

### How long items stay

Items stay in the trash for **30 days**. The panel checks every hour and removes older items for good.

- The trash is kept in `/var/lib/panel-trash/<domain>`, or `/var/lib/panel-trash/_server` for the rest of the server. Only root can read it, so a website's user can't reach its deleted files.
- Items in the trash still take disk space. Delete very large files permanently.
- An item larger than 1 GB that sits on another disk than the trash can't be moved there. Delete it permanently instead.
- Deleting a website also deletes its trash.
