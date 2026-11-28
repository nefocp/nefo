---
title: Acerca de Nefo
description: Qué es Nefo, para quién es, los principios en los que se basa, su estado y su licencia, y cómo obtener ayuda o informar de un error.
---

# Acerca de Nefo

Nefo es un panel de control gratuito y de código abierto para un servidor Linux. Lo instalas en tu propio VPS con un solo comando y luego administras desde el navegador, en uno de siete idiomas, los sitios web, los certificados, las bases de datos, las aplicaciones, los archivos, las copias de seguridad y la seguridad. Está en beta, tiene licencia GNU AGPLv3 y está hecho para un VPS con 1 GB de RAM.

Es un único programa, `/usr/local/bin/nefo`, con la interfaz web integrada, y se ejecuta como el servicio de systemd `nefo`. Funciona en Ubuntu 22.04, 24.04 y 26.04 y en Debian 12 y 13, en x86-64 y ARM64. Su tienda de aplicaciones tiene 42 aplicaciones, y la interfaz está en inglés, vietnamita, chino, español, portugués, ruso e indonesio. Consulta [¿Qué es Nefo?](./docs/) para ver todo lo que hace.

## Para quién es {#who-it-is-for}

Nefo es para quienes administran su propio VPS: alguien que monta su primer servidor para un blog, un desarrollador que despliega una aplicación, o un freelancer o una agencia que se ocupa de los sitios web de sus clientes.

Cada servidor tiene una sola cuenta del panel, `admin`. Nefo no está hecho para vender hosting: no tiene cuentas de revendedores ni de clientes. Consulta [Nefo y otros paneles](./compare) para saber cuándo te conviene más otro panel.

## Principios {#principles}

- **Ligero.** Nefo está hecho para un VPS con 1 GB de RAM, y el propio panel usa unos {{ $release.ram }} de memoria (medido en un servidor de pruebas con la v{{ $release.version }}). No instala nada hasta que lo eliges, cada componente se puede volver a desinstalar, y se dejan fuera las funciones que mantendrían grandes servicios funcionando todo el tiempo, como un servidor de correo.
- **Todas las funciones son gratuitas.** No hay edición de pago ni funciones bloqueadas. No creas ninguna cuenta y no hay clave de licencia.
- **Nada se comunica a tus espaldas.** Sin telemetría, sin estadísticas de uso, sin comprobación de licencia. La interfaz no carga nada de otros sitios. Por su cuenta, el panel solo se comunica con Let's Encrypt para los certificados y con las versiones del proyecto en GitHub para el catálogo firmado de la tienda de aplicaciones, y solo busca actualizaciones cuando se lo pides. Consulta [Privacidad](./docs/#privacy).
- **Honesto sobre sus límites.** La documentación dice lo que Nefo no hace, y lo que se niega a hacer y por qué. La [hoja de ruta](./roadmap) enumera lo que no está previsto con la misma claridad que lo que sí.

## Estado {#status}

Nefo está en beta. La primera versión pública, la v0.20.0, salió el 26 de noviembre de 2026, y la versión actual es la v{{ $release.version }} ({{ $release.date }}). Consulta el [registro de cambios](./changelog).

- Los números de versión empiezan por `0.`. Hasta la versión 1.0, solo la última versión recibe correcciones de seguridad.
- Todavía no se ha hecho ninguna auditoría de seguridad externa. Consulta la [política de seguridad](./security-policy).
- El instalador se ha probado en servidores Ubuntu 22.04, 24.04 y 26.04 y Debian 12 y 13 recién instalados, en x86-64. ARM64 todavía no se ha probado en hardware ARM real.

Prueba Nefo en un servidor que puedas reconstruir antes de confiarle sitios web importantes, y guarda copias de tus copias de seguridad fuera del servidor.

## Licencia {#licence}

Nefo es software libre bajo la licencia [GNU AGPLv3](https://github.com/nefocp/nefo/blob/main/LICENSE). Puedes usarlo para cualquier fin, en tantos servidores como quieras, y leer, modificar y compartir su código. Si ofreces un Nefo modificado a otras personas a través de una red, la AGPL te obliga a publicar tus cambios.

El código fuente está en [GitHub](https://github.com/nefocp/nefo).

## Obtener ayuda e informar de errores {#get-help-and-report-bugs}

- **Lee primero la documentación.** [Solución de problemas](./docs/troubleshooting) trata los problemas habituales, y las [preguntas frecuentes](./docs/faq), las dudas habituales.
- **Informa de un error** en [GitHub Issues](https://github.com/nefocp/nefo/issues). Incluye tu versión (`nefo version`), tu sistema operativo, los pasos que llevan al problema, lo que esperabas y lo que ocurrió. Si falla el propio panel, agrega el final de su registro: `journalctl -u nefo -n 50 --no-pager`.
- **No incluyas secretos** en nada de lo que publiques: la ruta secreta del panel, contraseñas, claves de API y nombres de dominio privados.
- **Informa de un problema de seguridad de forma privada,** nunca en una incidencia pública. Consulta la [política de seguridad](./security-policy#report-a-vulnerability).

## Contribuir {#contribute}

- **Informa de errores** como se describe más arriba. Los informes desde servidores reales son lo que Nefo más necesita antes de la 1.0.
- **Traduce el panel.** Copia `web/src/locales/en.json` a un archivo con el nombre del código de tu idioma, traduce los valores (no las claves) y registra el idioma en `web/src/i18n.js`.
- **Corrige incidencias y envía pull requests.** Lee antes [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md). Nefo se ejecuta como root en los servidores de sus usuarios, así que cada cambio se revisa pensando en la seguridad, y las reglas de seguridad para el código que figuran allí no son opcionales. Para un cambio grande, abre antes una incidencia para acordar el enfoque.
- **Firma el CLA.** Los colaboradores firman el [Acuerdo de licencia de colaborador](https://github.com/nefocp/nefo/blob/main/CLA.md) una sola vez, cuando abren su primer pull request. Conservas los derechos de autor de tus contribuciones.
