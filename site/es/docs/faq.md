---
title: Preguntas frecuentes
description: Respuestas breves a las preguntas habituales sobre Nefo, el panel de control gratuito para servidores Linux, sobre si es gratuito, su licencia, los sistemas compatibles, la memoria, las bases de datos, Docker, mover sitios web, la seguridad, la privacidad, las actualizaciones, la desinstalación y los idiomas.
---

# Preguntas frecuentes

Respuestas breves a las preguntas que más se hacen sobre Nefo, el panel de control gratuito y de código abierto para un servidor Linux. Cada respuesta empieza por lo esencial, luego da los detalles y enlaza a la página que los trata. Los datos corresponden a Nefo v0.24.1.

## ¿Qué es Nefo? {#what-is-nefo}

Nefo es un panel de control gratuito y de código abierto que instalas en tu propio servidor Linux (un VPS) con un solo comando, para administrar desde el navegador sitios web, certificados SSL, bases de datos, aplicaciones, archivos, copias de seguridad y seguridad. Es un solo programa, `/usr/local/bin/nefo`, con la interfaz web integrada, y se ejecuta como el servicio de systemd `nefo`. Consulta [¿Qué es Nefo?](./).

## ¿Nefo es gratuito? {#is-nefo-free}

Sí. Nefo es software libre bajo la GNU AGPLv3, y todas las funciones son gratuitas para todos, empresas incluidas. No hay edición de pago, ni funciones bloqueadas, ni clave de licencia, ni cuenta que crear. Consulta [Acerca de](../about#licence).

## ¿En qué funciona Nefo? {#what-does-nefo-run-on}

Nefo funciona en Ubuntu 22.04, 24.04 y 26.04 y Debian 12 y 13, en x86 de 64 bits (amd64) y ARM de 64 bits (arm64), con acceso root y systemd. El instalador se ha probado en servidores recién instalados con cada uno de estos sistemas en x86-64. La compilación arm64 todavía no se ha probado en hardware ARM real, así que pruébala primero en un servidor que puedas reconstruir. En otros sistemas, el instalador se detiene salvo que lo fuerces. Consulta [Requisitos](./requirements).

## ¿Cuánta RAM necesita Nefo? {#how-much-ram-does-nefo-need}

El panel en sí usa unos 34 MB de memoria, medidos en un servidor de prueba después de actualizar a la v0.24.0, y Nefo está hecho para un VPS con 1 GB de RAM. Lo que instalas se suma a eso, y la página **Software** muestra aproximadamente cuánto: Nginx unos 15 MB, cada versión de PHP unos 25 MB, MariaDB su buffer pool más unos 100 MB (unos 228 MB en un servidor de 1 GB). En un servidor de 1 GB, agrega de 1 a 2 GB de swap. Consulta [Requisitos](./requirements#memory-and-disk).

## ¿Nefo admite MySQL, PostgreSQL y Docker? {#does-nefo-support-mysql-postgresql-and-docker}

Sí. Nefo ejecuta MariaDB (la de tu distribución, o la 11.4, 11.8 o 12.3 LTS) o MySQL 8.4 LTS, uno por servidor, y puede cambiar de uno a otro con tus bases de datos. PostgreSQL (la versión de tu distribución, o de la 15 a la 19 del proyecto PostgreSQL) funciona a su lado. Docker ejecuta contenedores y proyectos de Compose, y las aplicaciones Docker de la tienda de aplicaciones. Consulta [Bases de datos](./databases), [PostgreSQL](./postgresql) y [Docker](./docker).

## ¿Qué servidores web puedo usar? {#which-web-servers-can-i-use}

Nginx, Apache u OpenLiteSpeed, uno por servidor. Lo eliges cuando instalas el servidor web:

- Nginx es el más ligero.
- Apache lee los archivos `.htaccess` que traen algunas aplicaciones PHP.
- OpenLiteSpeed lee las reglas de reescritura de los archivos `.htaccess` y trae LiteSpeed Cache integrado.

Con los tres, PHP se ejecuta en PHP-FPM, con un pool por sitio web. Los sitios web también pueden ser aplicaciones Node.js, Python o Java, o proxies inversos a otra dirección. Consulta [Sitios web](./websites).

## ¿Puedo cambiar a otro servidor web más adelante? {#can-i-switch-to-another-web-server-later}

Sí, pero solo en un servidor sin sitios web. El panel se niega a desinstalar un servidor web mientras todavía sirve sitios web, porque los ajustes de cada sitio web están escritos para ese servidor web. Elimina los sitios web, desinstala el servidor web en la página **Software** y luego instala el otro. Haz antes una copia de seguridad de tus sitios web. Consulta [Software](./software#web-server).

## ¿Puedo traer mis sitios web desde cPanel o aaPanel? {#can-i-move-my-websites-from-another-panel}

Sí. Nefo importa una copia de seguridad completa de cPanel (sitios web, bases de datos con sus usuarios y contraseñas, certificados y tareas cron), y un archivo comprimido de un sitio web con sus volcados SQL desde aaPanel, HestiaCP, CyberPanel o cualquier otro servidor. El correo, las zonas DNS y las cuentas FTP no se importan. Consulta [Importar sitios web](./import).

## ¿Nefo es seguro? {#is-nefo-safe}

Nefo está hecho para ser seguro de forma predeterminada, pero es software beta que se ejecuta como root y todavía no ha pasado una auditoría de seguridad externa. El panel solo responde bajo una ruta secreta en un puerto aleatorio, usa HTTPS desde el primer arranque, admite la autenticación de dos factores, bloquea una dirección tras 5 inicios de sesión fallidos en 15 minutos y registra cada cambio. Las versiones se firman con Ed25519, y la [revisión de seguridad](./security-check) integrada enumera los ajustes que dejan el servidor expuesto. Pruébalo primero en un servidor que puedas reconstruir, y guarda copias de seguridad fuera del servidor. Consulta [Seguridad](./security) y la [política de seguridad](../security-policy).

## ¿Nefo envía datos a algún sitio? {#does-nefo-send-data-anywhere}

Sin telemetría, sin estadísticas de uso y sin comprobación de licencia. Por su cuenta, el panel solo contacta con Let's Encrypt para emitir y renovar certificados y, en las compilaciones de versión publicada, descarga una vez al día el catálogo firmado de la tienda de aplicaciones. Todo lo demás, como la búsqueda de actualizaciones, las alertas, las copias de seguridad remotas y la base de datos de países del bloqueo por país, ocurre solo cuando lo pides o lo configuras. Consulta [Privacidad](./#privacy).

## ¿Nefo puede hacer copias de seguridad en Google Drive? {#can-nefo-back-up-to-google-drive}

Sí, desde la v0.24.0, con un cliente OAuth propio que solo puede ver los archivos que crea el panel. Nefo también hace copias de seguridad en almacenamiento compatible con S3 (Amazon S3, Cloudflare R2, Backblaze B2, Wasabi, MinIO) y en servidores SFTP, puede cifrar cada copia remota con una frase de contraseña, y un servidor nuevo puede encontrar las copias de seguridad en el almacenamiento y restaurarlas. Consulta [Copias de seguridad](./backups#google-drive) y [Restaurar en un servidor nuevo](./restore-new-server).

## ¿Cómo actualizo Nefo? {#how-do-i-update-nefo}

Abre **Configuración** → **Panel** y haz clic en **Buscar actualizaciones**, o ejecuta `nefo update` como root. La actualización comprueba la firma y la suma de verificación de la versión antes de reemplazar el programa, y luego reinicia el panel; los sitios web siguen funcionando. Nefo nunca se actualiza solo. Consulta [Actualizar](./update).

## ¿Cómo desinstalo Nefo? {#how-do-i-uninstall-nefo}

Nefo no tiene un comando de desinstalación: se quita a mano. Desbloquea los sitios web bloqueados con `nefo unlock-files`, detén y quita el servicio `nefo` y `/usr/local/bin/nefo`, cierra el puerto del panel y elimina `/etc/nefo` y `/var/lib/nefo`. Tus sitios web y tu software siguen funcionando, salvo que los quites antes. Consulta [Desinstalar](./uninstall).

## ¿Nefo puede alojar mi correo? {#can-nefo-host-my-e-mail}

No. Nefo no tiene servidor de correo: no recibe correo ni aloja buzones. Los sitios web y las alertas pueden enviar correo a través de un relay SMTP de un proveedor como Gmail, SendGrid, Mailgun o Amazon SES, y la página DNS puede agregar los registros MX, SPF y DMARC de tu proveedor de correo. Usa un proveedor de correo para tus buzones. Consulta [Correo y alertas](./mail-alerts).

## ¿Puedo administrar varios servidores desde un solo panel? {#can-i-manage-several-servers-from-one-panel}

No. Instala Nefo en cada servidor y administra cada uno por separado. No está previsto administrar varios servidores desde un mismo panel: un panel que controla otros servidores necesita agentes y confianza entre las máquinas. Consulta la [hoja de ruta](../roadmap#not-planned-and-why).

## ¿Puedo darles a mis clientes su propio acceso? {#can-i-give-my-clients-their-own-login}

No. Cada servidor tiene una sola cuenta del panel, `admin`. Nefo no tiene cuentas separadas para clientes ni revendedores. Para que un cliente suba archivos a su sitio web, dale a ese sitio web una cuenta SFTP, que queda limitada a la carpeta del sitio web. Consulta [Sitios web](./websites#sftp-access).

## ¿En qué idiomas está el panel? {#which-languages-does-the-panel-speak}

En siete, desde la v0.24.0: inglés, vietnamita, chino (simplificado), español, portugués, ruso e indonesio, incluidos los textos de la tienda de aplicaciones. El panel sigue el idioma de tu navegador, y puedes cambiarlo en la página de inicio de sesión o en el menú de la cuenta. Cada idioma es un archivo aparte que el navegador descarga solo cuando se usa. Esta documentación está en los mismos siete idiomas.

Las traducciones son bienvenidas. Copia `web/src/locales/en.json` a un archivo con el código de tu idioma, traduce los valores y registra el idioma en `web/src/i18n.js`. Consulta [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md).

## ¿Cómo informo de un problema de seguridad? {#how-do-i-report-a-security-problem}

Infórmalo en privado, no en un issue público. En el [repositorio de GitHub](https://github.com/nefocp/nefo), abre la pestaña "Security" y elige "Report a vulnerability". Incluye tu versión (`nefo version`), tu sistema operativo y los pasos para reproducir el problema. Consulta la [política de seguridad](../security-policy), que sigue [SECURITY.md](https://github.com/nefocp/nefo/blob/main/SECURITY.md).

## ¿Hay un registro de cambios y una hoja de ruta? {#is-there-a-changelog-and-a-roadmap}

Sí. El [registro de cambios](../changelog) enumera lo que cambió en cada versión, de la más reciente a la más antigua, y lo que conviene saber antes de actualizar. La [hoja de ruta](../roadmap) enumera en qué se está trabajando antes de la versión 1.0, qué se está considerando y qué no está previsto, y por qué. Para comprobar tú mismo la firma de una versión antes de instalarla, consulta [Comprobar tú mismo una versión](../security-policy#check-a-release-yourself).

## ¿Cómo puedo ayudar al proyecto? {#how-can-i-support-the-project}

- Informa de errores con los pasos para reproducirlos, tu sistema operativo y `nefo version`.
- Traduce el panel a tu idioma, o mejora una traducción.
- Corrige issues y envía pull requests (consulta [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md)).
- Dale una estrella al proyecto en [GitHub](https://github.com/nefocp/nefo) y cuéntaselo a otros.
