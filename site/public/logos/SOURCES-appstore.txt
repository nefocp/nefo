# Logo sources

Every file in this folder was downloaded into its own new, empty folder, then passed through
`scripts/sanitize-svg.mjs`: an allow-list rebuild that keeps only `svg, g, path, circle, ellipse, rect, line,
polyline, polygon, defs, linearGradient, radialGradient, stop, clipPath, mask, use` (local `#id` references only)
and presentation attributes. `<script>`, `<style>` (simple `.class{}` rules are inlined first), `foreignObject`,
`image`, `a`, `text`, `title`, metadata, editor namespaces, event handlers, `style` attributes (converted to
attributes), and every external, `data:` or `javascript:` reference are removed. Root `width`/`height` are dropped
so the file scales to its box. A final check refuses any file where a forbidden token survives, and
`sheet/logos-sheet.png` shows every file rendered as an `<img>` on light and dark tiles.

Simple Icons files are single-colour; the brand colour from Simple Icons' own data (`hex`) is set as the root
`fill`. Devicon "original" files keep their own colours.

**Trademarks.** The licences below cover the SVG files. The logos are trademarks of their owners and are used here
only to identify each application in the app store (nominative use). Several owners publish brand rules (linked by
Simple Icons as "guidelines"); none of them forbids this kind of use, but do not alter the marks beyond resizing.

**Dark mode.** Ghost, Ollama, Umami, Vaultwarden, Open WebUI, code-server, MariaDB, Adminer and the OpenCart wordmark
are dark on transparent. Show every logo on a light tile (white, rounded) in both themes rather than on the card
background.

| file | source (pinned) | licence of the file | raw sha256 | sanitized bytes / sha256 | note |
|---|---|---|---|---|---|
| `adguard-home.svg` | [Simple Icons 16.37.0 adguard](https://cdn.jsdelivr.net/npm/simple-icons@16.37.0/icons/adguard.svg) | CC0-1.0 (Simple Icons) | `f84e09836ad10d2d…` | 353 / `f8a8cc86db364cea…` | fill #68BC71 |
| `adminer.svg` | [Simple Icons 16.37.0 adminer](https://cdn.jsdelivr.net/npm/simple-icons@16.37.0/icons/adminer.svg) | CC0-1.0 (Simple Icons) | `775a5bc84b17e50d…` | 2324 / `f0e073270ea55e16…` | fill #34567C |
| `apache.svg` | [Devicon 2.18.0 apache-original](https://cdn.jsdelivr.net/npm/devicon@2.18.0/icons/apache/apache-original.svg) | MIT (Devicon) | `843d8cb7cf9a54eb…` | 8424 / `00959ed8071484ea…` |  |
| `code-server.svg` | [coder/code-server@c094099 src/browser/media/favicon.svg](https://raw.githubusercontent.com/coder/code-server/c094099c1516e578cb3b0976175e4e75a552e6d1/src/browser/media/favicon.svg) | MIT (code-server) | `179196a5ec3f20e1…` | 580 / `06cb4147d012bbaa…` |  |
| `composer.svg` | [Devicon 2.18.0 composer-original](https://cdn.jsdelivr.net/npm/devicon@2.18.0/icons/composer/composer-original.svg) | MIT (Devicon) | `146892e50267d38c…` | 45733 / `0a29cdde720d65b5…` |  |
| `directus.svg` | [Simple Icons 16.37.0 directus](https://cdn.jsdelivr.net/npm/simple-icons@16.37.0/icons/directus.svg) | CC0-1.0 (Simple Icons) | `29bf6496a1f716a1…` | 1126 / `c5f48e06f4eaaf14…` | fill #263238 |
| `docker.svg` | [Devicon 2.18.0 docker-original](https://cdn.jsdelivr.net/npm/devicon@2.18.0/icons/docker/docker-original.svg) | MIT (Devicon) | `3fd830ea69431ed7…` | 4453 / `fce65873cec1f399…` |  |
| `drupal.svg` | [Devicon 2.18.0 drupal-original](https://cdn.jsdelivr.net/npm/devicon@2.18.0/icons/drupal/drupal-original.svg) | MIT (Devicon) | `1fdbb906efb26f93…` | 4054 / `56bb0f4379138f16…` |  |
| `filebrowser.svg` | [gtsteffaniak/filebrowser@1672af6 frontend/public/img/icons/favicon.svg](https://raw.githubusercontent.com/gtsteffaniak/filebrowser/1672af6a6ffc0ee59601e46e74e019f13e78365e/frontend/public/img/icons/favicon.svg) | Apache-2.0 (FileBrowser Quantum) | `04fc96d53dea42ca…` | 324 / `401184e036fa5f6f…` |  |
| `ghost.svg` | [Devicon 2.18.0 ghost-original](https://cdn.jsdelivr.net/npm/devicon@2.18.0/icons/ghost/ghost-original.svg) | MIT (Devicon) | `4ab9622f4a9c5d04…` | 5070 / `2f6229b76efef782…` |  |
| `gitea.svg` | [Devicon 2.18.0 gitea-original](https://cdn.jsdelivr.net/npm/devicon@2.18.0/icons/gitea/gitea-original.svg) | MIT (Devicon) | `42c350d2c90cee1c…` | 2988 / `03643def2b76b939…` |  |
| `it-tools.svg` | [CorentinTh/it-tools@d505845 public/safari-pinned-tab.svg](https://raw.githubusercontent.com/CorentinTh/it-tools/d505845f918e946ec300af7b36efc107e2f66e9e/public/safari-pinned-tab.svg) | GPL-3.0 (IT-Tools) | `d9e86b1375dc051f…` | 5401 / `987360b66fd34308…` |  |
| `joomla.svg` | [Simple Icons 16.37.0 joomla](https://cdn.jsdelivr.net/npm/simple-icons@16.37.0/icons/joomla.svg) | CC0-1.0 (Simple Icons) | `f6bca1d9fe97dadc…` | 1451 / `ddd6e3ad9f631d6c…` | fill #5091CD |
| `laravel.svg` | [Devicon 2.18.0 laravel-original](https://cdn.jsdelivr.net/npm/devicon@2.18.0/icons/laravel/laravel-original.svg) | MIT (Devicon) | `b293960cd642537d…` | 2339 / `fe7c5a2de1057bb1…` |  |
| `mariadb.svg` | [Devicon 2.18.0 mariadb-original](https://cdn.jsdelivr.net/npm/devicon@2.18.0/icons/mariadb/mariadb-original.svg) | MIT (Devicon) | `90f14c26b4784471…` | 1831 / `488ec85910d573c7…` |  |
| `minio.svg` | [Simple Icons 16.37.0 minio](https://cdn.jsdelivr.net/npm/simple-icons@16.37.0/icons/minio.svg) | CC0-1.0 (Simple Icons) | `2367d2631b710930…` | 694 / `f1c2f18741b063a8…` | fill #C72E49 |
| `mongodb.svg` | [Devicon 2.18.0 mongodb-original](https://cdn.jsdelivr.net/npm/devicon@2.18.0/icons/mongodb/mongodb-original.svg) | MIT (Devicon) | `d9f2bf7041264f47…` | 5612 / `dc0750ff1b346253…` |  |
| `moodle.svg` | [Devicon 2.18.0 moodle-original](https://cdn.jsdelivr.net/npm/devicon@2.18.0/icons/moodle/moodle-original.svg) | MIT (Devicon) | `b00498c0faff39f8…` | 3513 / `b2eac0a8dda7a0f9…` |  |
| `n8n.svg` | [Devicon 2.18.0 n8n-original](https://cdn.jsdelivr.net/npm/devicon@2.18.0/icons/n8n/n8n-original.svg) | MIT (Devicon) | `21e959a63ece5523…` | 1205 / `de75b726c5216d09…` |  |
| `nextcloud.svg` | [Simple Icons 16.37.0 nextcloud](https://cdn.jsdelivr.net/npm/simple-icons@16.37.0/icons/nextcloud.svg) | CC0-1.0 (Simple Icons) | `523a9295f2e3a8be…` | 839 / `af03854373ddab96…` | fill #0082C9 |
| `nginx.svg` | [Devicon 2.18.0 nginx-original](https://cdn.jsdelivr.net/npm/devicon@2.18.0/icons/nginx/nginx-original.svg) | MIT (Devicon) | `a28c7002fda6748b…` | 1095 / `e30de0ffeace8650…` |  |
| `nodejs.svg` | [Devicon 2.18.0 nodejs-original](https://cdn.jsdelivr.net/npm/devicon@2.18.0/icons/nodejs/nodejs-original.svg) | MIT (Devicon) | `3218687c5ea24a4d…` | 1806 / `c59864b712c508eb…` |  |
| `odoo.svg` | [Simple Icons 16.37.0 odoo](https://cdn.jsdelivr.net/npm/simple-icons@16.37.0/icons/odoo.svg) | CC0-1.0 (Simple Icons) | `2690d23a9f3c287f…` | 1367 / `3e3d73a53e36df3a…` | fill #714B67 |
| `ollama.svg` | [Devicon 2.18.0 ollama-original](https://cdn.jsdelivr.net/npm/devicon@2.18.0/icons/ollama/ollama-original.svg) | MIT (Devicon) | `3f0c9f16aa641707…` | 2361 / `cf57b3cf954d403e…` |  |
| `open-webui.svg` | [Lobe Icons (@lobehub/icons-static-svg 1.101.0) openwebui; the Open WebUI repo has only a PNG favicon](https://cdn.jsdelivr.net/npm/@lobehub/icons-static-svg@1.101.0/icons/openwebui.svg) | MIT (Lobe Icons) | `9cbf7a1105e51029…` | 406 / `37cc221599521e03…` | currentColor->000000 |
| `opencart.svg` | [Wikimedia Commons File:OpenCart_logo.svg (wordmark; the repo has only an icon font and PNGs, opencart.com is behind a challenge)](https://upload.wikimedia.org/wikipedia/commons/6/63/OpenCart_logo.svg) | Public domain (Commons: PD, author OpenCart) | `c7c50387cbc8eb75…` | 8594 / `2c722c594cb64176…` |  |
| `php.svg` | [Devicon 2.18.0 php-original](https://cdn.jsdelivr.net/npm/devicon@2.18.0/icons/php/php-original.svg) | MIT (Devicon) | `f22619a19c99a123…` | 4623 / `3f329463c9bc389b…` |  |
| `postgresql.svg` | [Devicon 2.18.0 postgresql-original](https://cdn.jsdelivr.net/npm/devicon@2.18.0/icons/postgresql/postgresql-original.svg) | MIT (Devicon) | `f220a436258ed014…` | 9179 / `351888466697a95e…` |  |
| `python.svg` | [Devicon 2.18.0 python-original](https://cdn.jsdelivr.net/npm/devicon@2.18.0/icons/python/python-original.svg) | MIT (Devicon) | `71493b4a732f0532…` | 2300 / `59f472ed8e4ee83b…` |  |
| `redis.svg` | [Devicon 2.18.0 redis-original](https://cdn.jsdelivr.net/npm/devicon@2.18.0/icons/redis/redis-original.svg) | MIT (Devicon) | `4707378b190878f3…` | 1729 / `3fa7a989fc84756c…` |  |
| `rustfs.svg` | [Simple Icons 16.37.0 rustfs](https://cdn.jsdelivr.net/npm/simple-icons@16.37.0/icons/rustfs.svg) | Simple Icons entry license "custom" (see SOURCES.md) | `d96219c14be44603…` | 1711 / `ddf8b7a9bbb1f8dd…` | fill #0196D0 |
| `stirling-pdf.svg` | [Stirling-Tools/Stirling-PDF@a805365 app/core/src/main/resources/static/favicon.svg](https://raw.githubusercontent.com/Stirling-Tools/Stirling-PDF/a8053654bc75792c75dd3e3c6f2c99165fbf12e4/app/core/src/main/resources/static/favicon.svg) | MIT (Stirling-PDF core; repo also has proprietary folders) | `2d7d074be4960bfd…` | 267 / `351c16bc6e80d876…` |  |
| `strapi.svg` | [Simple Icons 16.37.0 strapi](https://cdn.jsdelivr.net/npm/simple-icons@16.37.0/icons/strapi.svg) | CC0-1.0 (Simple Icons) | `7ace74e62d71799f…` | 597 / `e4cb232f45a2ea12…` | fill #4945FF |
| `umami.svg` | [Simple Icons 16.37.0 umami](https://cdn.jsdelivr.net/npm/simple-icons@16.37.0/icons/umami.svg) | MIT (Simple Icons entry license, Umami) | `aead19c8457dea1a…` | 437 / `19a1517b0eb4417a…` | fill #000000 |
| `uptime-kuma.svg` | [Simple Icons 16.37.0 uptimekuma](https://cdn.jsdelivr.net/npm/simple-icons@16.37.0/icons/uptimekuma.svg) | CC0-1.0 (Simple Icons) | `c851fba020f5c1b3…` | 547 / `cb1f3ae03992c154…` | fill #5CDD8B |
| `vaultwarden.svg` | [Simple Icons 16.37.0 vaultwarden](https://cdn.jsdelivr.net/npm/simple-icons@16.37.0/icons/vaultwarden.svg) | CC0-1.0 (Simple Icons) | `150f5e30aeea8d7b…` | 4724 / `8fd78b9fac3f0944…` | fill #000000 |
| `wg-easy.svg` | [Devicon 2.18.0 wireguard-original (WireGuard mark; wg-easy ships only PNG logos)](https://cdn.jsdelivr.net/npm/devicon@2.18.0/icons/wireguard/wireguard-original.svg) | MIT (Devicon); WireGuard is a trademark of Jason A. Donenfeld | `bb1b301c5b77dfc5…` | 2933 / `affc8c3ce8793fd0…` |  |
| `wikijs.svg` | [Simple Icons 16.37.0 wikidotjs](https://cdn.jsdelivr.net/npm/simple-icons@16.37.0/icons/wikidotjs.svg) | CC0-1.0 (Simple Icons) | `50362e5b575c054d…` | 4992 / `b941a4674b06ec75…` | fill #1976D2 |
| `woocommerce.svg` | [Devicon 2.18.0 woocommerce-original](https://cdn.jsdelivr.net/npm/devicon@2.18.0/icons/woocommerce/woocommerce-original.svg) | MIT (Devicon) | `7770286380dc0d2c…` | 1016 / `a8e022baf3aecdfc…` |  |
| `wordpress.svg` | [Simple Icons 16.37.0 wordpress (Devicon wordpress-original includes the wordmark)](https://cdn.jsdelivr.net/npm/simple-icons@16.37.0/icons/wordpress.svg) | CC0-1.0 (Simple Icons) | `8877151b8978c3c9…` | 1040 / `11e012af5adab3bc…` | fill #21759B |

Full raw-file checksums are in `../dl/fetch.tsv` (key, URL, note, licence, sha256, colour).

## Gaps

- **wg-easy** publishes only PNG logos; the WireGuard mark (Devicon) is used instead. Replace it if wg-easy ships an SVG.
- **OpenCart**: the repository has only an icon font and PNGs, and opencart.com sits behind a bot challenge, so the
  public-domain wordmark from Wikimedia Commons is used. It is wide (400×78); fit it with `object-fit: contain`.
- **Open WebUI**: the repository's `favicon.svg` only wraps a PNG; the Lobe Icons (MIT) vector is used.
- **RustFS**: Simple Icons marks it `custom` licence and points to the RustFS trademark policy
  (https://docs.rustfs.com/trademark/); check it before shipping, or fall back to a letter tile.
- **No logo found** (use the current letter tile): OpenLiteSpeed, ClamAV, ModSecurity, ufw, fail2ban, Postfix, wp-cli.
- `minio.svg` is kept only in case MinIO stays in the catalog (see RESEARCH.md: MinIO is replaced).
- `strapi.svg` is kept only in case Strapi stays (see RESEARCH.md: no official image).

## Batch 2 (2026-11-27)

Same process (own empty download folder, `scripts/sanitize-svg.mjs`, rendered check on light and dark tiles).
Memos, Kanboard and Healthchecks are dark on transparent too: they also need the light tile. The Gotify logo is
CC BY 4.0: credit "Gotify logo by the Gotify authors (github.com/gotify/logo), CC BY 4.0" where logos are credited.
MediaWiki was left out of the catalog for now: its logo is CC BY-SA 4.0 (Wikimedia Foundation) and no file under a
permissive licence exists; Docmost publishes no SVG logo.

| file | source (pinned) | licence of the file | raw sha256 | sanitized bytes / sha256 | note |
|---|---|---|---|---|---|
| `actual.svg` | [Simple Icons 16.37.0 actualbudget](https://cdn.jsdelivr.net/npm/simple-icons@16.37.0/icons/actualbudget.svg) | CC0-1.0 (Simple Icons) | `b232e27f2c195558…` | 576 / `47fc2cf3e5b9061b…` | fill #6B46C1 |
| `changedetection.svg` | [Simple Icons 16.37.0 changedetection](https://cdn.jsdelivr.net/npm/simple-icons@16.37.0/icons/changedetection.svg) | CC0-1.0 (Simple Icons) | `3b4bb4bc98e9dd7d…` | 2424 / `1422e5b46a54f7bf…` | fill #3056D3 |
| `freshrss.svg` | [Simple Icons 16.37.0 freshrss](https://cdn.jsdelivr.net/npm/simple-icons@16.37.0/icons/freshrss.svg) | CC0-1.0 (Simple Icons) | `b64c2585d258b259…` | 1236 / `864a64207bdbc2ea…` | fill #0062BE |
| `gotify.svg` | [gotify/logo@25c1d2c gotify-logo-small.svg](https://raw.githubusercontent.com/gotify/logo/25c1d2c08894fcb0ed39c36a2816316a161c0e57/gotify-logo-small.svg) | CC-BY-4.0 (gotify/logo) | `1b7d4e4f46d09950…` | 6012 / `2a024c3c0c347227…` | Illustrator <switch>/<foreignObject> wrapper removed before sanitizing (the plain drawing inside is kept) |
| `grafana.svg` | [Devicon 2.18.0 grafana-original](https://cdn.jsdelivr.net/npm/devicon@2.18.0/icons/grafana/grafana-original.svg) | MIT (Devicon) | `2dec200d4e56521b…` | 4958 / `1c286ea25155c6b2…` |  |
| `healthchecks.svg` | [healthchecks/healthchecks@v5.0.3 static/img/favicon.svg](https://raw.githubusercontent.com/healthchecks/healthchecks/v5.0.3/static/img/favicon.svg) | BSD-3-Clause (Healthchecks) | `f19ed549a08dfeb0…` | 430 / `19686da13c29e39c…` |  |
| `jellyfin.svg` | [Simple Icons 16.37.0 jellyfin](https://cdn.jsdelivr.net/npm/simple-icons@16.37.0/icons/jellyfin.svg) | CC0-1.0 (Simple Icons) | `244d6b1913518914…` | 412 / `a7ba9d187e1844e4…` | fill #00A4DC |
| `kanboard.svg` | [kanboard/kanboard@v1.2.58 assets/img/inkscape-optimized-icon.svg](https://raw.githubusercontent.com/kanboard/kanboard/v1.2.58/assets/img/inkscape-optimized-icon.svg) | MIT (Kanboard) | `5e343a13cb0df0bf…` | 1256 / `a43d8b49be5f9a14…` |  |
| `linkding.svg` | [sissbruecker/linkding@v1.47.0 bookmarks/static/logo.svg](https://raw.githubusercontent.com/sissbruecker/linkding/v1.47.0/bookmarks/static/logo.svg) | MIT (linkding) | `9a7a04afa8c933fc…` | 664 / `01a817c02f5f6fb2…` |  |
| `listmonk.svg` | [Simple Icons 16.37.0 listmonk](https://cdn.jsdelivr.net/npm/simple-icons@16.37.0/icons/listmonk.svg) | CC0-1.0 (Simple Icons) | `1a821e29ed200856…` | 300 / `19db275692445c0b…` | fill #0055D4 |
| `mealie.svg` | [Simple Icons 16.37.0 mealie](https://cdn.jsdelivr.net/npm/simple-icons@16.37.0/icons/mealie.svg) | CC0-1.0 (Simple Icons) | `eda5cc8fb29f0e6f…` | 429 / `0720fbb76e3c9f11…` | fill #E58325 |
| `memos.svg` | [usememos/memos@26.11.2 web/public/logo.svg](https://raw.githubusercontent.com/usememos/memos/26.11.2/web/public/logo.svg) | MIT (Memos) | `1f65578358a8b420…` | 1554 / `021a04f613b4f299…` |  |
| `navidrome.svg` | [navidrome/website@be4aff4 static/brand/navidrome-logo.svg](https://raw.githubusercontent.com/navidrome/website/be4aff40ebf7b9e2bfea93bb1ef5d48331820c56/static/brand/navidrome-logo.svg) | Apache-2.0 (navidrome/website) | `27ae71feedde5195…` | 3093 / `7046cf7ae06f9f83…` |  |
| `ntfy.svg` | [Simple Icons 16.37.0 ntfy](https://cdn.jsdelivr.net/npm/simple-icons@16.37.0/icons/ntfy.svg) | CC0-1.0 (Simple Icons) | `10b6e507eba94b0f…` | 520 / `2c655966b2ca3ab9…` | fill #317F6F |
| `vikunja.svg` | [Simple Icons 16.37.0 vikunja](https://cdn.jsdelivr.net/npm/simple-icons@16.37.0/icons/vikunja.svg) | CC0-1.0 (Simple Icons) | `d47e13dab0520551…` | 4419 / `e0c714731b1b6a37…` | fill #196AFF |
