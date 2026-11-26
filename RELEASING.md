# Releasing

Releases are built by GitHub Actions when a tag such as `v1.2.0` is pushed (see `.github/workflows/release.yml`). Each release contains:

- `nefo-linux-amd64` and `nefo-linux-arm64`
- `install.sh`
- `VERSION`
- `checksums.txt` (SHA-256 of the files above)
- `checksums.txt.sig` (Ed25519 signature of `checksums.txt`)
- `appstore-catalog.json` and `appstore-catalog.json.sig`: the App store catalog (`internal/appstore/catalog.json`), signed on its own. Panels fetch it once a day from the latest release and use it only when its `catalog_version` is higher than the one built into them, so a catalog fix can ship without a new binary: raise `catalog_version`, then run `make release` and replace these two files on the latest release.

`install.sh` and `nefo update` refuse a download whose checksum does not match, and refuse a `checksums.txt` that is not signed by the project's key. So an attacker who can change files on GitHub, or in transit, still cannot get their own binary installed.

## One-time setup: the signing key

1. On an offline or otherwise trusted machine, create the key pair:

   ```sh
   openssl genpkey -algorithm ed25519 -out release.key
   openssl pkey -in release.key -pubout -out internal/update/release.pub
   ```

2. Commit `internal/update/release.pub`. It is built into the nefo binary, and the release build copies it into `install.sh`.
3. Add the content of `release.key` as the repository secret `RELEASE_SIGNING_KEY` (*Settings → Secrets and variables → Actions*).
4. Keep an offline backup of `release.key`, and never commit it.

While `internal/update/release.pub` is empty, builds are development builds: they do not check signatures, and the release workflow warns that the release is unsigned.

## Making a release

```sh
git tag -s v1.2.0 -m "v1.2.0"
git push origin v1.2.0
```

To build a release locally: `make release VERSION=v1.2.0 SIGNING_KEY=/path/to/release.key`.

`install.sh` passes its setup options (after `--`, or in `NEFO_SETUP`) to the `nefo setup` of the binary it downloads, after checking them with `nefo setup --check`. Releases before `nefo setup` existed refuse every option, so an installer run with options and an older `NEFO_VERSION` stops before configuring anything. When you change the installer, test it in fresh containers both without options (it must behave as before) and with them.

## If the key is lost or leaked

Create a new key pair, commit the new `release.pub`, and publish a release signed with the **old** key (if you still have it) that contains the new public key. Servers that install that release then trust the new key. If the old key leaked, announce it in a security advisory: installations must then be updated by hand (`install.sh` from a trusted copy).
