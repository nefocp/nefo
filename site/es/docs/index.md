---
title: ¿Qué es Nefo?
description: Nefo es un panel de control gratuito y de código abierto que instalas en tu propio servidor Linux para administrar desde el navegador sitios web, bases de datos, aplicaciones, copias de seguridad y seguridad.
---

# ¿Qué es Nefo?

Nefo es un panel de control gratuito y de código abierto (GNU AGPLv3) que instalas en tu propio servidor Linux con un solo comando. Desde el navegador, en uno de siete idiomas, administras sitios web y sus certificados SSL, bases de datos, aplicaciones, archivos, copias de seguridad y seguridad. Es un solo programa, hecho para un VPS con 1 GB de RAM, y no envía telemetría.

Esta página explica qué hace Nefo, para quién es, qué deja fuera a propósito y qué envía por la red.

## Un panel de control en un solo programa {#a-control-panel-in-one-program}

Instalas Nefo en un VPS con un solo comando. Es un único programa, `/usr/local/bin/nefo`, con la interfaz web integrada, y se ejecuta como el servicio de systemd `nefo`. Después administras el servidor desde el navegador: sitios web, certificados, bases de datos, archivos, copias de seguridad, software y seguridad.

- **Todas las funciones son gratuitas.** No hay plan de pago ni cuotas por sitio o por cuenta.
- **Sin cuenta y sin servidor de licencias.** No hay que registrar ni activar nada.
- **Hecho para servidores pequeños.** Nefo está diseñado para funcionar en un VPS con 1 GB de RAM, en x86-64 y ARM64. El panel en sí usa unos 34 MB de memoria, medidos en un servidor de prueba después de actualizar a la v0.24.0.
- **Siete idiomas de interfaz:** inglés, vietnamita, chino, español, portugués, ruso e indonesio. Eliges el idioma en la página de inicio de sesión o en el menú de la cuenta, y el navegador descarga solo el idioma que usas.

Para empezar, revisa los [requisitos](./requirements), luego [instala Nefo](./install) y sigue los [primeros pasos](./first-steps).

## Para quién es {#who-it-is-for}

Nefo es para quienes administran su propio VPS: alguien que monta su primer servidor para un blog, un desarrollador que despliega una aplicación, o un freelancer o una agencia que se ocupa de los sitios web de sus clientes.

Cada servidor tiene una sola cuenta del panel, `admin`. Nefo no tiene accesos separados para clientes ni revendedores.

## Qué puedes hacer con él {#what-you-can-do-with-it}

### Sitios web {#websites}

- Aloja sitios web PHP, estáticos y de aplicaciones (Node.js, Python o Java) en Nginx, Apache u OpenLiteSpeed, y sitios web de proxy inverso que reenvían a otra dirección. Cada sitio web tiene su propio usuario de Linux y, de forma opcional, una cuenta SFTP limitada a su carpeta. Clona un sitio web a un dominio nuevo y elimina uno con una copia de seguridad previa. Consulta [Sitios web](./websites).
- Activa HTTPS con certificados gratuitos de Let's Encrypt que se renuevan automáticamente, certificados comodín (wildcard) mediante el DNS de Cloudflare, o tu propio certificado. Consulta [Certificados SSL](./ssl).
- Bloquea o permite visitantes por país, y deja el código de un sitio web en solo lectura para su PHP. Consulta [Bloqueo por país y bloqueo de archivos](./website-protection).
- Despliega desde GitHub, GitLab, Gitea o cualquier servidor Git, cuando lo pidas o en cada push. Consulta [Desplegar desde Git](./git-deploy).
- Mantén actualizados los sitios WordPress, con una copia de seguridad de la base de datos antes de cada actualización, modo de mantenimiento y una copia de staging. Consulta [Herramientas de WordPress](./wordpress).
- Instala 42 aplicaciones con un clic: aplicaciones PHP como WordPress, Joomla, Nextcloud y Laravel, y aplicaciones Docker como Ghost, Uptime Kuma, n8n, Gitea, Grafana y Jellyfin. Consulta [Tienda de aplicaciones](./apps).

### Datos {#data}

- Usa MariaDB (la de tu distribución, o la 11.4, 11.8 o 12.3 LTS) o MySQL 8.4, ajustados para servidores pequeños, con bases de datos, usuarios, importación y exportación de SQL y acceso remoto por base de datos, y cambia de uno a otro con tus bases de datos. Adminer y phpMyAdmin quedan detrás del inicio de sesión del panel. Explora las claves de Redis y agrega Memcached. Consulta [Bases de datos](./databases).
- Usa PostgreSQL, la versión de tu distribución o de la 15 a la 19 del proyecto PostgreSQL, con sus propias bases de datos y propietarios, acceso remoto por TLS y actualizaciones de versión mayor. Consulta [PostgreSQL](./postgresql).
- Importa sitios web desde una copia de seguridad completa de cPanel, o desde un archivo comprimido del sitio y volcados SQL hechos por aaPanel, HestiaCP, CyberPanel o cualquier servidor. Consulta [Importar sitios web](./import).
- Administra archivos con un editor, subidas y descargas, archivos comprimidos y una papelera que conserva los archivos eliminados durante 30 días. Consulta [Archivos](./files).
- Haz copias de seguridad de sitios web, bases de datos, carpetas y aplicaciones según una programación, en este servidor, en un almacenamiento compatible con S3, en un servidor SFTP o en Google Drive, cifradas con una frase de contraseña si quieres, y restáuralas, también en un servidor nuevo. Consulta [Copias de seguridad](./backups) y [Restaurar en un servidor nuevo](./restore-new-server).

### Servidor {#server}

- Instala y desinstala cada componente (servidor web, versiones de PHP y el ionCube Loader, MariaDB o MySQL, PostgreSQL, Redis, Node.js, Python, Java, Docker y más). Nefo se niega a quitar uno que todavía usan sitios web, bases de datos o contenedores. Consulta [Software](./software).
- Ejecuta contenedores Docker y proyectos de Compose. Nefo revisa los archivos de Compose en busca de ajustes inseguros, como contenedores privilegiados o puertos abiertos a internet. Consulta [Docker](./docker).
- Controla el firewall (ufw), fail2ban, los ajustes de SSH, un firewall de aplicaciones web por sitio web y un escáner de malware. Consulta [Seguridad](./security).
- Ejecuta la revisión de seguridad: 22 comprobaciones rápidas del servidor, cada una con un enlace a la solución, cuando la pidas o a diario. Consulta [Revisión de seguridad](./security-check).
- Usa una terminal en el navegador y administra servicios, procesos, programas en segundo plano y tareas cron. Consulta [Herramientas del sistema](./system).
- Comprueba que cada sitio web responde, con su disponibilidad y sus tiempos de respuesta. Consulta [Monitoreo](./websites#monitoring).
- Permite que los sitios web envíen correo a través de un relay SMTP y recibe alertas por Telegram, correo electrónico, Discord, Slack o un webhook, también cuando alguien inicia sesión. Consulta [Correo y alertas](./mail-alerts).
- Administra los registros DNS de tus zonas de Cloudflare, apunta el dominio de un sitio web a este servidor y agrega los registros MX, SPF y DMARC de tu proveedor de correo. Consulta [DNS (Cloudflare)](./dns).

## Seguridad del propio panel {#security-of-the-panel-itself}

Nefo se ejecuta como root, así que protege su propio inicio de sesión:

- El panel solo responde bajo una ruta secreta como `/k3x9p2qa7m1z/`. Cualquier otra dirección en su puerto devuelve `404`.
- Usa HTTPS desde el primer arranque. Al principio el certificado es autofirmado, y puedes reemplazarlo por un certificado gratuito para el dominio propio del panel.
- El inicio de sesión admite autenticación de dos factores. Tras 5 intentos fallidos en 15 minutos, una dirección queda bloqueada durante 15 minutos.
- Los inicios de sesión y los cambios se anotan en un registro de actividad, que conserva las entradas durante 90 días.

## Lo que Nefo no hace {#what-nefo-doesn-t-do}

- **Sin servidor de correo.** Nefo no recibe correo ni aloja buzones. Los sitios web y las alertas envían correo a través de un relay SMTP de un proveedor de correo. Usa un proveedor de correo para tus buzones.
- **Sin servidor DNS ni FTP.** Nefo administra los registros DNS de tus dominios en Cloudflare, y cada sitio web puede tener una cuenta SFTP en lugar de FTP.
- **Un servidor por panel.** Cada servidor ejecuta su propio Nefo. No está previsto administrar varios servidores desde un mismo panel (consulta la [hoja de ruta](../roadmap#not-planned-and-why)).
- **Sin cuentas para clientes.** Hay una sola cuenta de administrador por servidor.
- **Sin cuentas de correo, DNS ni FTP al importar.** Una importación desde cPanel trae los sitios web, las bases de datos, los certificados y las tareas cron. Consulta [Lo que no se importa](./import#what-is-not-imported).
- **Solo Ubuntu y Debian.** Ubuntu 22.04, 24.04 y 26.04, y Debian 12 y 13. Consulta [Requisitos](./requirements).

## Estado {#status}

Nefo es un software joven. Los números de versión empiezan por `0.`, y hasta la versión 1.0 solo la última versión recibe correcciones de seguridad. Puedes ver tu versión en **Configuración** → **Panel**, o con `nefo version` en el servidor.

El instalador se ha probado en servidores Ubuntu 22.04, 24.04 y 26.04 y Debian 12 y 13 recién instalados, en x86-64. La versión ARM64 todavía no se ha probado en hardware ARM real. Todavía no se ha hecho ninguna auditoría de seguridad externa.

Prueba Nefo en un servidor que puedas reconstruir antes de confiarle sitios web importantes, y guarda copias de tus copias de seguridad fuera del servidor. Informa de los problemas en el [repositorio de GitHub](https://github.com/nefocp/nefo).

El [registro de cambios](../changelog) enumera lo que cambió en cada versión; la [hoja de ruta](../roadmap), en qué se está trabajando y qué no está previsto, y la [política de seguridad](../security-policy), cómo informar de una vulnerabilidad y cómo comprobar tú mismo una versión.

## Licencia {#licence}

Nefo es software libre bajo la licencia GNU AGPLv3. Puedes usarlo para cualquier fin, incluido el uso comercial. Si ofreces un Nefo modificado a otras personas a través de una red, la AGPL te obliga a publicar tus cambios.

## Privacidad {#privacy}

Nefo no tiene telemetría, ni estadísticas de uso, ni comprobación de licencia. La interfaz web no carga nada de otros sitios: sus fuentes, iconos y scripts están integrados en el programa.

Por su cuenta, el panel solo se conecta a otros servidores para estas tareas:

| Cuándo | Adónde | Para qué |
|---|---|---|
| Cuando toca un certificado (se comprueba cada hora) | Let's Encrypt | Solicitar o renovar los certificados de tus sitios web y del panel. La cuenta de Let's Encrypt del panel no tiene dirección de correo. Salvo que uses la validación por DNS, el panel primero descarga un archivo de prueba de tu dominio por HTTP, para comprobar que el dominio llega a este servidor. |
| Una vez al día, en las compilaciones de versión publicada | Las versiones del proyecto en GitHub | Descargar el catálogo firmado de la tienda de aplicaciones. Un catálogo sin una firma válida del proyecto se ignora. |

Todo lo demás ocurre solo porque tú lo pediste:

- **Actualizaciones:** Nefo busca una versión nueva solo cuando haces clic en **Buscar actualizaciones** o ejecutas `nefo update`. Nunca se actualiza solo. Consulta [Actualizar](./update).
- **Lo que configuras:** las alertas (Telegram, tu servidor SMTP, Discord, Slack o tu webhook), el monitoreo de sitios web (tus sitios web, por su dominio, como un visitante), las copias de seguridad remotas (tu almacenamiento S3 o SFTP, o Google Drive), la validación por DNS y la página DNS (la API de Cloudflare), el relay de correo (tu proveedor de correo) y el bloqueo por país, que descarga una vez al mes la base de datos de países de DB-IP y la lista de direcciones de Cloudflare mientras algún sitio web lo usa.
- **El software que instalas:** los paquetes vienen de tu distribución a través de apt, y algunos componentes vienen de sus propios proyectos, como Node.js de nodejs.org, las versiones adicionales de PHP del repositorio packages.sury.org, OpenLiteSpeed del repositorio de LiteSpeed, las versiones de MariaDB y PostgreSQL de los repositorios de sus proyectos cuando las eliges, y el ionCube Loader de ionCube. Parte del software instalado se conecta por su cuenta: por ejemplo, ClamAV descarga sus bases de datos de virus.

La revisión de seguridad no envía nada fuera del servidor. La mayoría de las solicitudes del panel se identifican como `Nefo/<version>`. No incluyen información sobre tu servidor ni tus sitios web más allá de lo que necesita la tarea, como los nombres de dominio en una solicitud de certificado.
