---
title: Hoja de ruta
description: En qué se está trabajando antes de Nefo 1.0, qué se está considerando, y qué no está previsto y por qué.
---

# Hoja de ruta

La hoja de ruta de Nefo no tiene fechas: una mejora se publica cuando funciona. Antes de la versión 1.0, el trabajo se centra en hacer fiable lo que ya existe: pruebas en hardware ARM64 real y con los servicios reales, un servidor de pruebas que funcione durante mucho tiempo, informes de usuarios externos y una revisión de seguridad externa. Algunas ideas se están considerando, y unas pocas cosas no están previstas, a propósito.

Esta página enumera en qué se está trabajando, qué se está considerando y qué no hará Nefo. Para ver lo que ya se publicó, lee el [registro de cambios](./changelog).

## Ahora: hacia la 1.0 {#now-toward-1-0}

Nefo está en beta. Antes de la versión 1.0, el trabajo se centra más en hacer fiable lo que ya existe que en funciones nuevas:

- **Probar el instalador en ARM64.** El instalador, con y sin la configuración inicial del software, se ha probado en servidores Ubuntu 22.04, 24.04 y 26.04 y Debian 12 y 13 recién instalados, en x86-64. Las compilaciones ARM64 se publican, pero todavía no se han probado en hardware ARM real.
- **Probar con los servicios reales.** Las copias de seguridad en almacenamiento compatible con S3 (como AWS S3 y Cloudflare R2) y en Google Drive, las alertas por Telegram, Discord, Slack y webhook, el correo a través de proveedores SMTP y la API de Cloudflare. Algunos de ellos, hasta ahora, solo se han probado con sustitutos: Google Drive, por ejemplo, solo con un servicio de Google simulado.
- **Mantener un servidor de pruebas durante mucho tiempo,** con sitios web reales, para que los problemas que tardan semanas en aparecer (registros que crecen, uso del disco, memoria) se encuentren antes de que los encuentres tú.
- **Usuarios externos y sus informes de errores.** Cada informe con los pasos para reproducir el error ayuda. Consulta [Informar de un error](./about#get-help-and-report-bugs).
- **Una revisión de seguridad externa.** Todavía no se ha hecho ninguna auditoría externa. Hasta entonces, trata Nefo como un software en beta que se ejecuta como root, y consulta la [política de seguridad](./security-policy).

Hasta la versión 1.0, solo la última versión recibe correcciones de seguridad.

## Lo que se está considerando {#being-considered}

Son ideas, no promesas. Algunas pueden cambiar, y algunas quizá nunca se publiquen.

- **Copias de seguridad en Google Drive sin un cliente OAuth propio.** Hoy cada servidor necesita un cliente OAuth que creas una sola vez en un proyecto de Google Cloud (consulta [Google Drive](./docs/backups#google-drive)).
- **La dirección real del visitante detrás del proxy de Cloudflare,** en los registros y en el PHP de un sitio web con Nginx, como un ajuste por sitio web. Hoy un sitio web con Nginx detrás del proxy ve ahí las direcciones de Cloudflare; el bloqueo por país ya consulta la dirección del visitante.
- **Una interfaz del panel más tranquila.**

El panel está en siete idiomas. Las traducciones a más idiomas son bienvenidas: consulta [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md).

## Lo que no está previsto, y por qué {#not-planned-and-why}

Nefo pretende seguir siendo lo bastante ligero para un VPS con 1 GB de RAM, y lo bastante pequeño para que se pueda revisar la seguridad de su código. Cada punto de abajo agregaría servicios que funcionan todo el tiempo, puertos abiertos a internet o una gran cantidad de código que se ejecuta como root. Por eso se dejan fuera a propósito, y esta página indica qué ofrece Nefo en su lugar.

| No está previsto | Por qué | Lo que ofrece Nefo en su lugar |
|---|---|---|
| Un servidor de correo | El correo necesita varios servicios funcionando todo el tiempo, filtros de spam y de virus que usan mucha memoria, y un cuidado constante de la reputación del servidor. Además, muchos proveedores bloquean el puerto 25 de salida. | Un relay SMTP a través de tu proveedor de correo, para los sitios web y las alertas. Usa un proveedor de correo para los buzones. Consulta [Correo y alertas](./docs/mail-alerts). |
| Un servidor DNS | Los servidores de nombres tienen que responder desde internet todo el tiempo, y lo mejor es que funcionen en más de una máquina. | Los registros DNS de tus zonas de Cloudflare, administrados desde el panel, y los certificados comodín mediante el DNS de Cloudflare. Consulta [DNS (Cloudflare)](./docs/dns). |
| Un servidor FTP | Otro servicio con sus propios puertos abiertos y sus propias cuentas. | Una cuenta SFTP por sitio web, limitada a la carpeta del sitio web, con contraseña o claves SSH. Consulta [Acceso SFTP](./docs/websites#sftp-access). |
| Administrar varios servidores desde un solo panel | Un panel que controla otros servidores necesita agentes y confianza entre máquinas. Una intrusión en uno llegaría a todos. | Un Nefo por servidor. Las claves de API permiten que tus propios scripts trabajen con cada servidor. Consulta [Claves de API](./docs/api). |
| Cuentas de revendedores y de clientes de hosting | Las cuentas con sus propios límites, planes y cuotas son otro producto, para vender hosting. Nefo es para la persona que administra el servidor. | Una sola cuenta de administrador. Para que un cliente suba archivos, dale a su sitio web una cuenta SFTP. |

Si necesitas una de estas cosas, puede que otro panel te convenga más: consulta [Nefo y otros paneles](./compare#when-another-panel-fits-you-better).
