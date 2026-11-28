---
title: Registro de cambios
description: Lo que cambió en cada versión de Nefo, de la más reciente a la más antigua, y lo que conviene saber antes de actualizar.
---

# Registro de cambios

Todas las versiones de Nefo, de la más reciente a la más antigua, con lo nuevo, lo que cambió y lo que se corrigió, y lo que conviene saber antes de actualizar. Cada una enlaza a su página en GitHub, que tiene los programas para amd64 y arm64, el instalador, las sumas de verificación y su firma. Para actualizar, consulta [Actualizar](./docs/update).

::: warning Volver atrás después de una actualización
La v0.21.0, la v0.22.0, la v0.23.0 y la v0.24.0 cambian cada una el formato de la base de datos del panel la primera vez que se inician. Después, una versión anterior se niega a iniciarse con esos datos. Si es posible que quieras volver atrás, copia `/var/lib/nefo` antes de actualizar, como muestra [Actualizar](./docs/update#if-an-update-fails).
:::

## v0.24.1 {#v0-24-1}

28 de noviembre de 2026 · [Versión en GitHub](https://github.com/nefocp/nefo/releases/tag/v0.24.1)

Copias de seguridad de sitios web que recuperan los sitios de aplicación y de proxy inverso, una revisión de seguridad más rápida y correcciones para los siete idiomas.

**Cambios**

- **La copia de seguridad de un sitio web guarda el tipo del sitio,** así que, al restaurarla en un servidor que no tiene ese sitio, un sitio de aplicación (Node.js, Python, Java) vuelve con su entorno de ejecución, su comando de inicio, su puerto y su límite de memoria, y un sitio de proxy inverso vuelve con su dirección y sus opciones. Instala antes el entorno de ejecución de la aplicación en la página **Software**. Las copias hechas con versiones anteriores siguen volviendo como sitios estáticos. La copia de un sitio de aplicación Docker se rechaza, con los pasos a seguir: instala primero la aplicación desde la tienda de aplicaciones y luego restaura la copia de seguridad de la aplicación. Consulta [Restaurar en un servidor nuevo](./docs/restore-new-server).
- **La revisión de seguridad ejecuta sus comprobaciones a la vez,** con un máximo de 5 segundos para cada programa al que consulta y 25 segundos para toda la revisión. Una comprobación que no termina a tiempo se marca como **resultado parcial** en lugar de frenar a las demás. Consulta [Revisión de seguridad](./docs/security-check).

**Correcciones**

- En un servidor con mucha carga, la revisión de seguridad podía tardar mucho y terminar con un error del servidor.
- En un contenedor con límite de memoria, el panel principal contaba como usada la caché de páginas y la memoria que el kernel libera cuando los programas la necesitan, así que la memoria parecía casi llena después de una copia de seguridad o de una descarga grande. Los ajustes de swap ahora también muestran la memoria del contenedor.
- En un servidor con el repositorio de PHP, el PHP más reciente del repositorio (8.6 en Ubuntu 24.04) se tomaba por la versión de PHP propia de la distribución: la página **Software** confundía de dónde venía cada una, y un servidor web instalado después recibía esa versión en lugar de la de la distribución.
- Las aplicaciones que agrega un catálogo más reciente de la tienda de aplicaciones no tenían logotipo fuera de la página de la tienda.
- En los idiomas con textos más largos, el menú de la cuenta cortaba la lista de idiomas (ahora tiene dos columnas), las pestañas del panel de un sitio web cortaban sus nombres (ahora ocupan dos líneas) y la barra superior podía pasar a una segunda línea en una pantalla estrecha.
- Se corrigieron algunos textos de la interfaz en chino, ruso e indonesio.

## v0.24.0 {#v0-24-0}

27 de noviembre de 2026 · [Versión en GitHub](https://github.com/nefocp/nefo/releases/tag/v0.24.0)

Siete idiomas de interfaz, copias de seguridad en Google Drive, restauración en un servidor nuevo y una revisión de seguridad.

**Novedades**

- **La interfaz en siete idiomas:** chino, español, portugués, ruso e indonesio junto al inglés y el vietnamita, incluidos los textos de la tienda de aplicaciones. Cada idioma es un archivo aparte que el navegador descarga solo cuando se usa, y los archivos de la interfaz se envían ahora comprimidos con gzip: el script principal pasa de 478 KB a 163 KB.
- **Copias de seguridad en Google Drive,** con un cliente OAuth propio, un inicio de sesión con un código de dispositivo, acceso solo a los archivos que crea el panel y subidas que se reanudan tras una interrupción. Consulta [Google Drive](./docs/backups#google-drive).
- **Copias remotas cifradas:** cada almacenamiento (S3, SFTP, Google Drive) puede cifrar con una frase de contraseña las copias de seguridad que recibe: scrypt y AES-256-GCM en bloques numerados, así que una copia cortada o modificada se rechaza. `nefo decrypt-backup` abre una copia a mano. Consulta [Cifrar las copias](./docs/backups#encrypt-the-copies).
- **Restauración en un servidor nuevo:** **Buscar copias de seguridad** enumera las copias de seguridad de un almacenamiento que este panel no conoce, y una restauración crea un sitio web que todavía no existe, con su dominio, sus alias, su versión de PHP, sus archivos y sus bases de datos con sus contraseñas. Restaurar la copia de seguridad de un sitio web eliminado lo vuelve a crear de la misma forma. Consulta [Restaurar en un servidor nuevo](./docs/restore-new-server).
- **Sitios web:** clonar sitios web de aplicaciones Node.js, Python y Docker, y sitios web con bases de datos PostgreSQL. Al eliminar un sitio web se pueden conservar sus bases de datos, y de forma predeterminada se hace antes una copia de seguridad. Un tipo de sitio web nuevo, el proxy inverso, reenvía a cualquier dirección, en Nginx, Apache y OpenLiteSpeed. Consulta [Sitios web](./docs/websites#reverse-proxy-websites).
- **Revisión de seguridad:** 22 comprobaciones rápidas del servidor (SSH, firewall, actualizaciones, bases de datos o Redis abiertos a internet, puertos de Docker que se saltan ufw, archivos accesibles por HTTP, certificados, ajustes de PHP…), cada una con un enlace a la solución, cuando la pidas o a diario. Consulta [Revisión de seguridad](./docs/security-check).
- **Alertas de inicio de sesión** para los inicios de sesión SSH (todos, o solo desde una dirección nueva), las rachas de inicios de sesión SSH fallidos y los inicios de sesión en el panel. Consulta [Alertas de inicio de sesión](./docs/mail-alerts#sign-in-alerts).
- **Bloqueo por país** por sitio web, con los datos de países gratuitos de DB-IP, que se descargan solo cuando un sitio web lo usa. También funciona en sitios web de proxy inverso. Consulta [Bloqueo por país y bloqueo de archivos](./docs/website-protection).
- **Un bloqueo de archivos** por sitio web: su código pasa a ser de solo lectura para su PHP y su cuenta SFTP, salvo las carpetas en las que debe escribir, con una alerta diaria opcional cuando cambian archivos PHP. `nefo unlock-files` devuelve los archivos antes de quitar Nefo.
- **Bases de datos:** MariaDB 11.4, 11.8 y 12.3 LTS del repositorio de MariaDB, y una actualización en el mismo lugar con un volcado previo y una vuelta atrás si falla; un cambio entre MariaDB y MySQL que traslada las bases de datos; rotación del registro de consultas lentas. Consulta [Bases de datos](./docs/databases#mariadb-versions).
- **PostgreSQL:** versiones de la 15 a la 19 del proyecto PostgreSQL, acceso remoto por base de datos (TLS, scram-sha-256, una regla de firewall por dirección) y una actualización de versión mayor con un volcado previo y una vuelta atrás. Los volcados que conservan las actualizaciones se enumeran, con un botón para eliminarlos. Consulta [PostgreSQL](./docs/postgresql).
- **Registros de correo en la página DNS:** MX, un único registro SPF combinado y DMARC para Google Workspace, Microsoft 365, Zoho Mail, Cloudflare Email Routing u otro proveedor, a través de Cloudflare. Consulta [DNS (Cloudflare)](./docs/dns#set-up-email-records).
- **Software:** Java (OpenJDK 17, 21 y 25) para los sitios web de aplicaciones, el ionCube Loader para cada versión de PHP, y PHP 8.6, que se ofrece por sí solo en cuanto el repositorio de PHP lo tenga. Consulta [Software](./docs/software).
- **Tienda de aplicaciones:** 15 aplicaciones más, 42 en total: Grafana, ntfy, Gotify, Memos, linkding, Navidrome, Vikunja, Actual Budget, changedetection.io, Healthchecks, listmonk, Kanboard, FreshRSS, Jellyfin y Mealie. Consulta [Tienda de aplicaciones](./docs/apps).

**Cambios**

- Las cifras de memoria siguen el límite de memoria propio de un contenedor (LXC), en el panel principal, en las alertas, en la configuración inicial y en las comprobaciones de la tienda de aplicaciones.
- Se corrigieron las cifras de disco de la página de configuración inicial, que ahora cuentan las listas de paquetes de apt en un servidor que nunca las descargó.
- Un catálogo de la tienda de aplicaciones más reciente con aplicaciones que un panel más antiguo no puede ejecutar deja fuera solo esas aplicaciones. Las aplicaciones que agrega un catálogo más reciente muestran su nombre en todas partes.

**Correcciones**

- No se podía instalar PHP 8.5, porque OPcache ya no tiene un paquete propio.
- En Ubuntu 26.04, la instalación de PostgreSQL (u otro servicio) podía fallar mientras systemd se volvía a ejecutar a sí mismo después de apt. Ahora el panel reintenta `systemctl` mientras no se puede contactar con systemd.
- El firewall web se ofrecía donde la distribución no tiene un módulo ModSecurity para el servidor web (Ubuntu 22.04 con Nginx). Ahora se muestra como no disponible, con el motivo.
- El texto de la página vacía de WordPress terminaba a mitad de una frase.

## v0.23.0 {#v0-23-0}

26 de noviembre de 2026 · [Versión en GitHub](https://github.com/nefocp/nefo/releases/tag/v0.23.0)

Claves de API, importación de sitios web, una página de configuración inicial, y más servidores de bases de datos y versiones de PHP.

**Novedades**

- **Claves de API** para tus propios scripts, en **Configuración** → **Claves de API**: de solo lectura o de acceso completo, con una lista de direcciones permitidas y una fecha de caducidad. Una clave se envía en un encabezado `Authorization: Bearer` y solo se guarda como hash, y una clave incorrecta cuenta como un inicio de sesión fallido. Ninguna clave puede abrir la terminal, Adminer, phpMyAdmin, la cuenta ni las propias claves. Una clave de solo lectura no puede leer contraseñas, archivos, exportaciones ni descargas de copias de seguridad. Consulta [Claves de API](./docs/api).
- **Importar sitios web** desde una copia de seguridad completa de cPanel (sitios web, bases de datos con sus usuarios y contraseñas, certificados y tareas cron), o desde un archivo comprimido del sitio y volcados SQL hechos por aaPanel u otro servidor. Se rechazan los archivos comprimidos hostiles, los volcados de MySQL 8 se adaptan para MariaDB, y una importación fallida quita todo lo que creó. Consulta [Importar sitios web](./docs/import).
- **Una página de configuración inicial** en el primer inicio de sesión: elige en una sola página el servidor web, las versiones de PHP, el servidor de bases de datos y los extras, mira aproximadamente cuánta memoria y cuánto disco necesitan, e instálalos como una sola tarea. `nefo setup` hace lo mismo en la línea de comandos, y el instalador acepta las mismas opciones. Sin opciones, el instalador se comporta como antes. Consulta [Primeros pasos](./docs/first-steps#set-up-the-server).
- **MySQL 8.4 LTS** como alternativa a MariaDB (uno u otro por servidor), desde el repositorio de Oracle con su clave fijada. Solo se ofrece en servidores amd64.
- **phpMyAdmin**, accesible solo a través del inicio de sesión del panel.
- **PostgreSQL** en la página **Bases de datos**: bases de datos, propietarios, importación y exportación, y copias de seguridad.
- **Memcached**, que solo escucha en el propio servidor, sin UDP.
- **PHP 5.6 a 7.3**, para sitios web trasladados desde servidores más antiguos. Solo se instalan después de que lo confirmes, y nunca pueden ser la única versión de PHP del servidor.
- **Marcas de fin de vida útil para PHP:** una versión que ya no tiene el soporte de seguridad de PHP se marca en la página **Software** y en sus sitios web. La versión de PHP propia de tu distribución nunca se marca, porque la distribución la sigue parcheando. Consulta [Software](./docs/software#end-of-life).
- **Composer** y **WP-CLI** en la página **Software**.

**Cambios**

- En Ubuntu, las versiones adicionales de PHP vienen ahora de `packages.sury.org`, porque `ppa:ondrej/php` ya no recibe actualizaciones. Un servidor que tiene el PPA cambia una sola vez, cuando se inicia el panel: los archivos de fuente del PPA se mueven a `/var/lib/nefo/apt-backup`, no se instala ni se quita nada, y los paquetes de PHP instalados se actualizan desde la fuente nueva después del siguiente `apt-get update`.

**Correcciones**

- Un sitio web creado como suspendido ahora también se guarda como suspendido.
- Las carpetas que crea el panel para otros programas ya no reciben los permisos estrictos propios del panel (umask 077).
- La versión de PHP propia de la distribución se reconoce en un servidor recién instalado, antes de que apt haya descargado sus listas de paquetes.
- Un `@` solo (el propio dominio) ya no rompe un mensaje en la página DNS.
- Una tarea que se bloquea ahora escribe su traza de pila en el registro del panel.

## v0.22.0 {#v0-22-0}

26 de noviembre de 2026 · [Versión en GitHub](https://github.com/nefocp/nefo/releases/tag/v0.22.0)

Monitoreo de sitios web, más canales de alertas y DNS de Cloudflare.

**Novedades**

- **Monitoreo de sitios web:** cada sitio web se comprueba cada 1 a 10 minutos (el código de estado, una palabra clave opcional y la caducidad del certificado). Un sitio web cuenta como caído tras dos comprobaciones fallidas, y recibes una alerta cuando se cae y cuando vuelve. El panel muestra la disponibilidad en 24 horas y en 7 días, los tiempos de respuesta y los fallos recientes. Las solicitudes de monitoreo no se cuentan en las estadísticas de tráfico. Consulta [Monitoreo](./docs/websites#monitoring).
- **Alertas por Discord, Slack y webhook.** Los webhooks se firman con HMAC-SHA256. Las URL guardadas se ocultan, y al enviar una alerta se rechazan las direcciones del propio servidor, las de enlace local y las de metadatos de la nube. Consulta [Correo y alertas](./docs/mail-alerts).
- **Una página DNS para las zonas de Cloudflare:** enumera, agrega, edita y elimina registros, activa o desactiva el proxy, vacía la caché y apunta el dominio de un sitio web a este servidor con una vista previa antes. Consulta [DNS (Cloudflare)](./docs/dns).

## v0.21.0 {#v0-21-0}

26 de noviembre de 2026 · [Versión en GitHub](https://github.com/nefocp/nefo/releases/tag/v0.21.0)

Herramientas del sistema, claves SSH y más opciones para los sitios web.

**Novedades**

- **Una página Sistema:** actualizaciones del sistema operativo (con las de seguridad marcadas, instalar todas o solo las de seguridad, actualizaciones de seguridad automáticas y un aviso cuando hace falta reiniciar), un archivo de swap y swappiness, la zona horaria, la sincronización del reloj, el nombre de host, TCP BBR y la limpieza del disco. La búsqueda de actualizaciones en segundo plano espera 30 minutos después de que se inicia el panel, y reutiliza las listas de paquetes actualizadas en las últimas 6 horas. Consulta [Sistema](./docs/os).
- **Claves SSH** en la página **Firewall y SSH**: enumerar, agregar y quitar. Se rechazan las claves privadas, las claves débiles y los duplicados, y no se puede quitar la última vía de entrada.
- **Sitios web:** desactivar HTTPS y eliminar el certificado, en los tres servidores web; HSTS, con `includeSubDomains` como opción; una caché de páginas por sitio web PHP en Nginx, con un botón para vaciarla; clonar un sitio web PHP o estático con sus archivos, ajustes y bases de datos; pasar una copia de staging de WordPress al sitio web en producción, después de una copia de seguridad.

**Correcciones**

- Las ejecuciones de apt del propio panel nunca se solapan, y espera al bloqueo de apt de otro programa en lugar de fallar.
- En un contenedor donde `/etc/hosts` está montado con bind, el nombre de host se escribe en el propio archivo. Un nombre de host que el sistema rechaza da un error claro.

## v0.20.2 {#v0-20-2}

26 de noviembre de 2026 · [Versión en GitHub](https://github.com/nefocp/nefo/releases/tag/v0.20.2)

**Cambios**

- Nginx ahora comprime CSS, JavaScript, SVG y fuentes. Debian y Ubuntu activan la compresión solo para HTML, así que estos archivos se enviaban sin comprimir. Los ajustes que ya tiene `nginx.conf` se dejan como están. Los servidores existentes reciben el cambio cuando se inicia el panel actualizado.

**Correcciones**

- Cada versión trae ahora el catálogo firmado de la tienda de aplicaciones. Antes, los paneles lo buscaban una vez al día y anotaban en el registro un HTTP 404.

## v0.20.1 {#v0-20-1}

26 de noviembre de 2026 · [Versión en GitHub](https://github.com/nefocp/nefo/releases/tag/v0.20.1)

**Correcciones**

- El instalador fallaba en todos los sistemas compatibles: leía la versión del sistema de `/etc/os-release` en lugar de la suya propia, e intentaba descargar una versión que no existe. Después de la corrección se probó en servidores Ubuntu 24.04 y Debian 12 recién instalados.

## v0.20.0 {#v0-20-0}

26 de noviembre de 2026 · [Etiqueta en GitHub](https://github.com/nefocp/nefo/releases/tag/v0.20.0)

La primera versión pública. Su instalador fallaba (corregido en la v0.20.1), y GitHub no tiene archivos de versión para ella, solo la etiqueta. Instala una versión más reciente.

Incluía sitios web en Nginx, Apache u OpenLiteSpeed con PHP-FPM, aplicaciones Node.js y Python, certificados de Let's Encrypt, despliegues desde Git y las herramientas de WordPress; la tienda de aplicaciones con aplicaciones PHP y Docker; MariaDB, Redis, el administrador de archivos, la terminal web y las copias de seguridad en almacenamiento compatible con S3 y SFTP; el firewall (ufw), fail2ban y los ajustes de SSH con protecciones contra quedarse fuera, el firewall de aplicaciones web y el escáner de malware; las versiones firmadas y `nefo update`; y la interfaz en inglés y vietnamita.
