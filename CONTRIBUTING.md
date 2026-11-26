# Contributing to Nefo

Thank you for helping! Nefo has root access to the servers it runs on, so we review every change with security in mind. Please read this page before you open a pull request.

## Ways to help

- **Translate** the UI. Translations live in `web/src/locales/`, and `en.json` is the source. Copy it to `<language-code>.json`, translate the values (not the keys), and register the language in `web/src/i18n.js`.
- **Report bugs** with steps to reproduce, your OS version and `nefo version`.
- **Fix issues** marked `good first issue` or `help wanted`.
- **Report security problems privately.** See [SECURITY.md](SECURITY.md) and never use a public issue for them.

For larger changes, open an issue first so we can agree on the approach before you spend time on it.

## Development setup

See the Development section of the [README](README.md). Before you push, run:

```sh
go vet ./... && go test ./...
cd web && npm run build
```

## Security rules for code

These rules are not optional; pull requests that break them will not be merged.

1. **Never build shell commands from strings.** Run programs with `exec.Command(name, arg1, arg2, …)` and pass each value as its own argument. Never use `sh -c` with user input.
2. **Validate every input against an allow-list** in `internal/validate` before it reaches a file path, a config file or a command. Add a validator (with tests, including attack strings) if none fits.
3. **Write config files from templates with escaped values.** Never concatenate user input into Nginx, PHP or systemd configuration.
4. **Every new API route that changes something** goes through `requireAuth`, and is written to the audit log if it matters for security.
5. **No new dependencies without discussion.** Every dependency runs as root on our users' servers.
6. **Never log secrets:** no passwords, tokens, 2FA secrets or the entrance path.

## UI rules

- Put every piece of text the user can see into `web/src/locales/en.json` and use it with `t('key')`. Never hard-code text in components.
- Never use `v-html`.
- The production build must not use `eval` or `new Function`, because the panel's Content Security Policy forbids them.

## Pull requests

- Keep each pull request focused on one change, and add tests for new behaviour.
- Describe what changed and why, and how you tested it.
- The first time you contribute, the CLA Assistant bot will ask you to sign the [Contributor Licence Agreement](CLA.md). It lets the project stay free under the AGPLv3 while also offering a [commercial licence](COMMERCIAL.md) that funds development.
