---
title: Security policy
description: How to report a vulnerability in Nefo, how releases are signed and how to check one yourself, and the current security status.
---

# Security policy

Nefo runs as root and manages a whole server, so every vulnerability report is treated as urgent. This page says how to report one, how releases are signed and how you can check one, and where Nefo stands today. It follows [SECURITY.md](https://github.com/nefocp/nefo/blob/main/SECURITY.md) in the repository.

## Report a vulnerability

Don't open a public issue, discussion or pull request. Report it privately on GitHub:

1. Open the [Nefo repository](https://github.com/nefocp/nefo) and go to the **Security** tab.
2. Choose **Report a vulnerability**, or use the [direct link](https://github.com/nefocp/nefo/security/advisories/new). Only the maintainers see your report.

Include:

- the affected version (`nefo version`) and the operating system;
- what an attacker can do, and what access they need first: none, a panel account, a shell account…;
- the steps to reproduce, or a proof of concept;
- whether you plan to publish your findings, and when.

### What happens next

| Step | Target |
|---|---|
| The project confirms that it received your report | within 3 working days |
| The project tells you whether it confirms the issue and how severe it is | within 7 days |
| The project releases a fix for a critical or high-severity issue | within 14 days of confirmation |
| Public disclosure | after the fix is released, or at most 90 days after your report, coordinated with you |

Every confirmed vulnerability gets a GitHub Security Advisory with a CVE. You are credited, unless you'd rather stay anonymous.

### Scope

In scope: the `nefo` program, its API and interface, the installer, and the configuration it creates on the server.

Out of scope:

- the browser's warning about the self-signed certificate the panel creates on its first start (this is expected);
- attacks that need root access to the server, or physical access to it;
- denial of service through traffic volume alone;
- vulnerabilities in third-party software the panel installs, such as Nginx, PHP or MariaDB. Report those to their own projects, but tell us if the panel configures them unsafely.

### Safe harbour

The project will not take legal action against you for security research that follows this policy: you test only on servers you own or have permission to test, you avoid privacy violations and data destruction, and you give the project reasonable time to fix the issue before you disclose it.

## Supported versions

Until version 1.0, only the latest release receives security fixes. Update from the panel (**Settings** → **Panel** → **Check for updates**) or with `nefo update` on the server. See [Update](./docs/update).

## How releases are signed

GitHub Actions builds each release from a version tag in the repository. A release contains:

- `nefo-linux-amd64` and `nefo-linux-arm64`, the programs;
- `install.sh`, the installer;
- `VERSION`;
- `checksums.txt`, the SHA-256 checksums of the two programs and the installer;
- `checksums.txt.sig`, an Ed25519 signature of `checksums.txt`;
- `appstore-catalog.json` and `appstore-catalog.json.sig`, the app store catalog with its own signature.

The project's public key is built into every Nefo program and copied into `install.sh`. It is also in the repository as [`internal/update/release.pub`](https://github.com/nefocp/nefo/blob/main/internal/update/release.pub):

```
-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEA2c6nGagX7yp8Y8YftuOqW2WVprKe6h0hv+3s9iB+KJA=
-----END PUBLIC KEY-----
```

Three things check signatures before they use a download:

- **The installer** checks the signature of `checksums.txt` and the checksum of the program. If either is wrong, it stops: "invalid signature: checksums.txt was not signed by the project. Do not install this file."
- **`nefo update`**, and **Install update** in the panel, do the same with the key built into the program you already run, then run the new program once before they replace `/usr/local/bin/nefo`.
- **The app store** fetches the catalog once a day and ignores it unless its signature is valid.

### What the signature does and doesn't prove

The signature shows that the files come from the project's release build and weren't changed afterwards, on GitHub or on the way to your server.

- **Updates** are checked with the key inside the Nefo you already have, so a changed release on GitHub can't get itself installed.
- **A first install** is different: the installer you download carries the key it checks with. If someone could change `install.sh` itself, they could change that key too. To rule this out, check `install.sh` yourself before you run it, as shown below.
- The signing key is kept as a secret of the GitHub repository, and releases are signed there by GitHub Actions. Someone who took over the project's GitHub account could sign a release.

## Check a release yourself

You need `curl`, `sha256sum` and OpenSSL 3 (on Ubuntu 22.04 and later, and on Debian 12 and later). This example checks the latest release on an amd64 server, as root; set `V` to another release tag to check that release.

1. Download the checksums, their signature and the installer:

   ```sh
   V=$(curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/VERSION)
   cd /tmp
   for f in checksums.txt checksums.txt.sig install.sh; do
     curl -fsSLO "https://github.com/nefocp/nefo/releases/download/$V/$f"
   done
   ```

2. Save the public key from this page:

   ```sh
   cat > nefo-release.pub <<'EOF'
   -----BEGIN PUBLIC KEY-----
   MCowBQYDK2VwAyEA2c6nGagX7yp8Y8YftuOqW2WVprKe6h0hv+3s9iB+KJA=
   -----END PUBLIC KEY-----
   EOF
   ```

3. Check the signature, then the installer's checksum:

   ```sh
   openssl pkeyutl -verify -pubin -inkey nefo-release.pub -rawin \
     -in checksums.txt -sigfile checksums.txt.sig
   grep " install.sh$" checksums.txt | sha256sum -c -
   ```

   The first command must print `Signature Verified Successfully`, the second `install.sh: OK`. If either fails, don't run the installer, and report it as above.

4. Run the installer you checked. It downloads the program for your CPU and checks it with the same key:

   ```sh
   NEFO_VERSION=$V bash install.sh
   ```

To check a program you downloaded yourself, such as `nefo-linux-amd64`, run `grep " nefo-linux-amd64$" checksums.txt | sha256sum -c -` after step 3.

## How Nefo protects the server

A short summary; [Security](./docs/security) has the details:

- The panel answers only under a secret path, on a random port. Every other address returns `404`.
- It uses HTTPS from its first start, with TLS 1.2 or newer.
- Sign-in supports two-factor authentication. 5 failed attempts in 15 minutes lock an address out for 15 minutes. Passwords are stored as bcrypt hashes.
- Sign-ins and changes are written to an activity log, kept for 90 days.
- Every page has a strict Content Security Policy, and every change needs a header that other websites can't send.
- The project's [rules for code](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md#security-rules-for-code) forbid building shell commands from input: a program gets each value as its own argument, and input is checked against an allow-list before it reaches a file path, a configuration file or a command. Commands you write yourself, such as cron jobs and deploy scripts, run in a shell as you wrote them.
- Firewall rules and SSH settings that would lock you out are refused.
- Each website runs as its own Linux user, and SFTP accounts are locked in the website's folder.

## Current status

Nefo is beta. Its first public release was v0.20.0, in November 2026; the [changelog](./changelog) lists the current one. No outside security audit has been done yet.

Try Nefo on a server you can rebuild before you rely on it for important websites, keep copies of your backups off the server, and keep Nefo up to date. A security review from outside the project is one of the steps before 1.0 (see the [roadmap](./roadmap)). If you review the code and find something, report it as described above.
