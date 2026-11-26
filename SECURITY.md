# Security Policy

Nefo runs as root and manages entire servers. We treat every vulnerability report as urgent and are grateful to everyone who reports one responsibly.

## Reporting a vulnerability

**Please do not open a public issue, discussion or pull request.**

Report it privately:

- **GitHub:** go to the repository's **Security** tab and choose **Report a vulnerability** ([direct link](https://github.com/nefocp/nefo/security/advisories/new)). This is the preferred way: only the maintainers see your report.

Please include:

- the affected version (`nefo version`) and operating system
- what an attacker can do, and what access they need first (none, a panel account, a shell account…)
- steps to reproduce, or a proof of concept
- whether you plan to publish your findings, and when

## What happens next

| Step | Target |
|---|---|
| We confirm we received your report | within 3 working days |
| We tell you whether we confirm the issue and how severe it is | within 7 days |
| We release a fix for a critical or high-severity issue | within 14 days of confirmation |
| Public disclosure | after the fix is released, or at most 90 days after your report, coordinated with you |

We publish a GitHub Security Advisory with a CVE for every confirmed vulnerability, and credit you unless you would rather stay anonymous.

## Supported versions

Until version 1.0, only the **latest release** receives security fixes. Update from the panel (**Settings** → **Panel** → **Check for updates**) or with `nefo update` on the server. Both check the release's Ed25519 signature and checksum before they replace the binary.

## Scope

In scope: the `nefo` binary, its API and UI, the installer, and the configuration it creates on the server.

Out of scope:

- the browser warning about the self-signed certificate that the panel creates on first start (this is expected)
- attacks that require root access to the server or physical access to it
- denial of service through traffic volume alone
- vulnerabilities in third-party software the panel installs (Nginx, PHP, MariaDB…). Report those upstream, but tell us if the panel configures them unsafely.

## Safe harbour

We will not take legal action against you for security research that follows this policy. That means you test only on servers you own or have permission to test, avoid privacy violations and data destruction, and give us reasonable time to fix the issue before you disclose it.
