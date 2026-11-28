---
title: Nefo y otros paneles
description: En qué se diferencia Nefo de los paneles de control que usan la mayoría de los dueños de un VPS, y cuándo otro panel es la mejor opción.
---

# Nefo y otros paneles

Nefo se diferencia de la mayoría de los paneles de control de hosting en algunos puntos: todas las funciones son gratuitas, sin clave de licencia ni cuenta con el proveedor; el panel en sí usa unos 34 MB de memoria; su inicio de sesión está en un puerto aleatorio, detrás de una ruta secreta; y deja fuera a propósito los servidores de correo, DNS y FTP y las cuentas de revendedores.

Esta página compara Nefo con los paneles de control de hosting que más se instalan en un VPS. No los nombra. Son distintos entre sí, así que la columna de la derecha indica cuántos de ellos hacen algo: la mayoría, muchos, algunos o pocos.

La comparación se comprobó con los sitios web de los propios paneles en noviembre de 2026. Los paneles cambian sus precios y sus funciones, así que revisa el que estés considerando antes de decidir.

## De un vistazo {#at-a-glance}

| | Nefo | Otros paneles |
|---|---|---|
| Precio | Gratuito. Todas las funciones, sin edición de pago | La mayoría tienen algo que pagar: una licencia por servidor, normalmente con precio según el número de cuentas o de sitios, o una edición de pago o complementos |
| Clave de licencia o cuenta con el proveedor | Ninguna | Algunos necesitan una para funcionar, y muchos la necesitan para sus funciones de pago |
| Datos enviados al proveedor | Ninguno: sin telemetría, sin informe de instalación, sin comprobación de licencia | Varía. Algunos recopilan estadísticas de uso de forma predeterminada, y algunos instaladores informan del servidor al proveedor |
| Memoria | El panel en sí usa unos 34 MB (medidos en un servidor de prueba con la v0.24.0). Hecho para un VPS con 1 GB de RAM | La mayoría piden al menos 1 GB, y muchos recomiendan 4 GB |
| Página de inicio de sesión | Un puerto aleatorio y una ruta secreta. Cualquier otra dirección devuelve `404` | La mayoría usan un puerto fijo y conocido. Unos pocos usan un puerto aleatorio o una ruta secreta |
| Actualizaciones del panel | Firmadas con Ed25519 y comprobadas antes de instalarse. Solo cuando lo pides | Algunos documentan paquetes firmados; otros no dicen nada |
| Protecciones contra quedarse fuera | Rechaza las reglas de firewall y los ajustes de SSH que te dejarían fuera | Pocos tienen una para el firewall, como una reversión temporizada. Las protecciones para los ajustes de SSH son raras |
| Código fuente | Todo, bajo la AGPLv3 | La mayoría son cerrados o solo de código disponible. Algunos son de código abierto |
| Servidores de correo y DNS | No incluidos, a propósito | La mayoría instalan ambos |
| Varios servidores desde un solo panel | No. Un panel por servidor | Pocos administran varios servidores desde un solo panel, aunque muchos pueden compartir zonas DNS entre servidores |
| Cuentas de revendedores y de clientes | No. Una sola cuenta de administrador | La mayoría tienen cuentas de revendedores, y casi todos tienen accesos para clientes o usuarios |
| Sistemas operativos | Ubuntu y Debian, en x86-64 y ARM64 | La mayoría son solo para Linux, y la mayoría funcionan en más distribuciones de Linux que estas dos. Pocos funcionan en Windows |
| Antigüedad | Primera versión pública en noviembre de 2026. Beta | Muchos existen desde hace 20 años o más |

## Lo que Nefo hace de otra forma {#what-nefo-does-differently}

### Se mantiene pequeño {#it-stays-small}

- Nefo es un solo programa, `/usr/local/bin/nefo`, de unos 25 MB, con la interfaz web integrada. En un servidor de prueba con la v0.24.0 usa unos 34 MB de memoria. Los archivos de la interfaz se envían comprimidos con gzip, y cada uno de sus siete idiomas se descarga solo cuando se usa.
- No instala nada más hasta que lo eliges. En el primer inicio de sesión, una página de configuración inicial pregunta qué servidor web, versiones de PHP, servidor de bases de datos y extras quieres, y muestra aproximadamente cuánta memoria y cuánto disco agrega tu elección. `nefo setup` hace lo mismo en la línea de comandos.
- Cada componente se puede volver a desinstalar, y el menú oculta lo que no está instalado. Nefo se niega a quitar un componente que todavía usan sitios web, bases de datos o contenedores.
- No funciona en segundo plano ningún servidor de correo, DNS ni FTP, porque Nefo no incluye ninguno.

### Todas las funciones son gratuitas, y nada se comunica a tus espaldas {#every-feature-is-free-and-nothing-phones-home}

- No hay edición de pago, ni cuenta que crear, ni clave de licencia.
- No hay telemetría ni estadísticas de uso. Por su cuenta, el panel solo se comunica con Let's Encrypt, para emitir y renovar certificados, y con las versiones del proyecto en GitHub una vez al día, para el catálogo firmado de la tienda de aplicaciones. Consulta [Privacidad](./docs/#privacy).
- Nunca se actualiza solo, y solo busca una versión nueva cuando se lo pides.

### El inicio de sesión es difícil de encontrar {#the-sign-in-is-hard-to-find}

- El instalador elige un puerto aleatorio entre 20000 y 49999 y una ruta secreta de 12 letras y dígitos. Cualquier otra dirección en ese puerto devuelve `404`.
- El panel usa HTTPS desde el primer arranque. El inicio de sesión admite autenticación de dos factores, y 5 intentos fallidos en 15 minutos bloquean una dirección durante 15 minutos. Los inicios de sesión y los cambios se anotan en un registro de actividad.
- Las claves de API para scripts pueden ser de solo lectura, limitarse a las direcciones que indiques y caducar. Ni siquiera una clave de acceso completo puede abrir la terminal, las herramientas de bases de datos ni los ajustes de la cuenta.

Consulta [Seguridad](./docs/security) y [Claves de API](./docs/api).

### Rechaza los cambios que te dejarían fuera o romperían algo {#it-refuses-changes-that-would-lock-you-out-or-break-things}

- **Firewall:** no se puede eliminar la última regla que deja llegar tu propia dirección a SSH o al panel.
- **SSH:** se rechazan los ajustes con los que nadie podría iniciar sesión, como desactivar el inicio de sesión con contraseña mientras ninguna cuenta tiene una clave.
- **Software:** no se puede desinstalar un servidor web, una versión de PHP o un servidor de bases de datos que todavía se usa.
- **Docker:** los proyectos de Compose se comprueban antes de guardarse o iniciarse. Se rechazan los contenedores privilegiados, los puertos abiertos a internet y los montajes de carpetas del sistema.

### Revisa su propio servidor {#it-checks-its-own-server}

La [revisión de seguridad](./docs/security-check) ejecuta 22 comprobaciones rápidas del servidor cuando la pides o una vez al día, como el inicio de sesión SSH con contraseña, el firewall, las actualizaciones de seguridad pendientes, las bases de datos abiertas a internet, los puertos de Docker que se saltan ufw y los archivos que un sitio web sirve por error, y enlaza cada hallazgo a la página donde se corrige. No envía nada fuera del servidor.

### Las copias de seguridad sobreviven al servidor {#backups-survive-the-server}

Las copias de seguridad van al propio servidor y a un almacenamiento compatible con S3, un servidor SFTP o Google Drive, cifradas con una frase de contraseña si quieres. Un servidor nuevo las encuentra en el almacenamiento y restaura los sitios web, con sus bases de datos. Consulta [Restaurar en un servidor nuevo](./docs/restore-new-server).

### Las versiones están firmadas {#releases-are-signed}

La lista de sumas de verificación de cada versión está firmada con la clave Ed25519 del proyecto. El instalador y `nefo update` comprueban la firma y la suma de verificación antes de instalar nada. La [política de seguridad](./security-policy#check-a-release-yourself) muestra cómo comprobar tú mismo una versión.

### Puedes traer tus sitios web {#you-can-move-websites-in}

Nefo importa sitios web desde las copias de seguridad de otros paneles: archivos, bases de datos con sus usuarios y contraseñas, certificados y tareas cron. Consulta [Importar sitios web](./docs/import) para ver los formatos que lee.

## Cuándo te conviene más otro panel {#when-another-panel-fits-you-better}

Nefo deja fuera algunas cosas a propósito (consulta en la [hoja de ruta](./roadmap#not-planned-and-why) por qué). Elige otro panel si necesitas alguna de estas:

- **Correo en el mismo servidor.** Nefo no tiene servidor de correo: ni buzones, ni webmail, ni filtro de spam. Los sitios web y las alertas envían correo a través del relay SMTP de tu proveedor. La mayoría de los demás paneles incluyen un servidor de correo.
- **Tu propio servidor DNS.** Nefo no ejecuta servidores de nombres. Administra los registros DNS de tus dominios en Cloudflare.
- **FTP.** Nefo no tiene servidor FTP. Cada sitio web puede tener una cuenta SFTP, limitada a la carpeta del sitio web.
- **Muchos servidores desde un solo lugar.** Cada servidor ejecuta su propio Nefo, y en cada uno inicias sesión por separado.
- **Vender hosting.** Nefo tiene una sola cuenta de administrador por servidor. No tiene revendedores, ni accesos para clientes, ni planes de hosting, ni cuotas.
- **Una larga trayectoria, o una auditoría de seguridad externa.** La primera versión pública de Nefo salió en noviembre de 2026. Está en beta, y todavía no se ha hecho ninguna auditoría de seguridad externa.
- **Windows, o un Linux distinto de Ubuntu y Debian.** Nefo solo funciona en Ubuntu 22.04, 24.04 y 26.04 y Debian 12 y 13.

Si nada de esto te importa, [instala Nefo](./docs/install) en un servidor que puedas reconstruir, y guarda una copia de tus copias de seguridad fuera del servidor.
