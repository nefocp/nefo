---
title: Bloqueo por país y bloqueo de archivos
description: Bloquea o permite a los visitantes de un sitio web según su país, con los datos de países gratuitos de DB-IP, y bloquea el código de un sitio web en solo lectura para su PHP, con una alerta cuando cambian archivos PHP.
---

# Bloqueo por país y bloqueo de archivos

Desde la v0.24.0, Nefo tiene dos protecciones por sitio web. El **bloqueo por país** rechaza a los visitantes de los países que indicas, o deja entrar solo a los de esos países, con los datos de países gratuitos de DB-IP, en Nginx, Apache y OpenLiteSpeed. El bloqueo de archivos hace que el código del sitio web sea de solo lectura para su propio PHP y su cuenta SFTP, salvo las carpetas en las que debe escribir, como la de las subidas.

Las dos están en los ajustes del sitio web, en el grupo **Seguridad**: **Sitios web** → (sitio) → **Bloqueo por país** y **Bloqueo de archivos**. Mientras están desactivadas no cuestan nada: ninguna descarga, ningún proceso adicional, nada cargado en el servidor web. Para el firewall de aplicaciones web, los límites de solicitudes y el escáner de malware, consulta [Seguridad](./security#website-security).

## ¿Cómo bloqueo países en un sitio web? {#block-or-allow-countries}

1. Abre **Sitios web**, haz clic en el sitio web y abre la pestaña **Bloqueo por país**.
2. Elige **Bloquear los países de abajo** o **Dejar entrar solo a los países de abajo**.
3. Escribe el nombre de un país (en el idioma del panel) o su código de dos letras en el campo de búsqueda, y haz clic en él para agregarlo. Para quitar uno, haz clic en la × de su etiqueta.
4. Haz clic en **Guardar**.

La primera vez que un sitio web del servidor usa el bloqueo por país, Nefo descarga la base de datos de países de db-ip.com (unos 5 MB) antes de guardar, lo que tarda unos segundos. Para volver a desactivarlo, elige **Desactivado: entran todos los países** y haz clic en **Guardar**.

El bloqueo por país funciona en todos los tipos de sitio web, incluidos los sitios web de proxy inverso. Cada sitio web tiene su propio ajuste.

Los visitantes bloqueados reciben **403 Forbidden**. Para mostrar tu propia página, pon un archivo `403.html` en la carpeta `public` del sitio web y apunta a él la página de error 403 del servidor web en **Sitios web** → (sitio) → **Configuración de Nginx** (Nginx: `error_page 403 /403.html;`; Apache: `ErrorDocument 403 /403.html`, que también funciona en `.htaccess`; OpenLiteSpeed: un bloque `errorpage 403`). Los visitantes bloqueados sí pueden cargar el propio `/403.html`.

### Lo que nunca se bloquea {#what-is-never-blocked}

- Las comprobaciones de Let's Encrypt (`/.well-known/acme-challenge/`), para que los certificados se sigan renovando.
- El webhook de despliegue con Git (`/.panel/deploy/…`). El panel comprueba su firma.
- `/403.html`.
- Las direcciones privadas y otras direcciones reservadas (`10.0.0.0/8`, `192.168.0.0/16`, loopback, etc.).
- Las solicitudes del propio servidor, como cuando WordPress se llama a sí mismo para sus tareas programadas.

El propio panel no es un sitio web: tiene su propio puerto y nunca se bloquea por país. Para limitar quién puede llegar al panel, usa el firewall (consulta [Seguridad](./security#firewall-ufw)).

### Detrás de Cloudflare {#behind-cloudflare}

Cuando el registro DNS de un sitio web pasa por el proxy de Cloudflare, todas las solicitudes vienen de direcciones de Cloudflare.

- **Nginx y Apache**: para las solicitudes que vienen de direcciones de Cloudflare, Nefo consulta la dirección del visitante que Cloudflare envía en el encabezado `CF-Connecting-IP`. Las solicitudes que vienen de cualquier otro lugar se consultan por su propia dirección, así que un cliente no puede falsificar el encabezado. En Apache, el registro de acceso y el PHP del sitio web también ven entonces la dirección del visitante en las solicitudes que pasan por Cloudflare.
- **OpenLiteSpeed**: para las solicitudes de direcciones de Cloudflare, Nefo usa el país que Cloudflare envía en el encabezado `CF-IPCountry`. Activa **IP Geolocation** en Cloudflare (ajustes de **Network** de la zona). Sin eso, los visitantes que llegan a través de Cloudflare no se bloquean, y con **Dejar entrar solo a los países de abajo** se les deja entrar.

Nefo tiene una lista integrada de las direcciones de Cloudflare y la actualiza desde Cloudflare cada mes, junto con la base de datos de países.

### Límites según el servidor web {#limits-per-web-server}

| Servidor web | Cómo funciona | Límite |
|---|---|---|
| Nginx | El módulo `geo` integrado de Nginx, cargado solo mientras un sitio web usa el bloqueo por país, con solo los países que enumeran los sitios web | Ninguno. Memoria: de 2 a 5 MB para un país, unos 40 MB para listas grandes (Estados Unidos, Alemania y Japón juntos) |
| Apache | Apache comprueba uno por uno, en cada solicitud, los rangos de direcciones de los países de la lista | Como máximo 50 000 rangos de direcciones por sitio web, así que los países grandes se rechazan (solo Estados Unidos tiene unos 270 000). China (15 000 rangos) ocupa unos 2.5 MB |
| OpenLiteSpeed | La consulta GeoIP propia de OpenLiteSpeed en el archivo de base de datos de DB-IP | Ninguno. Menos de 1 MB |

Una lista puede tener hasta 100 países. En Apache, la pestaña muestra cuántos rangos de direcciones usa el sitio web respecto del límite. Si una lista es demasiado grande, Apache la rechaza y el mensaje nombra los países que no caben, el más grande primero, con su número de rangos. Déjalos fuera, o usa Nginx u OpenLiteSpeed, que admiten todos los países.

### Los datos de países {#the-country-data}

Los datos son la base de datos gratuita **IP to Country Lite** de DB-IP, IP Geolocation by [DB-IP](https://db-ip.com), con la licencia Creative Commons Attribution 4.0. Cubre IPv4 e IPv6.

- Nefo la descarga por HTTPS la primera vez que un sitio web usa el bloqueo por país.
- Antes de reemplazar la copia anterior, comprueba que el archivo es una base de datos completa: su formato, su tamaño y que enumera más de 200 países.
- La actualiza una vez al mes (DB-IP publica una edición nueva a principios de cada mes). La pestaña muestra la edición en uso.
- Mientras ningún sitio web usa el bloqueo por país, no se descarga nada y el servidor web no carga nada.

La base de datos se guarda en `/var/lib/panel-geoip/`. Todos los usuarios pueden leerla, porque la lee el servidor web: son datos públicos.

## ¿Cómo hago que los archivos de un sitio web sean de solo lectura? {#lock-a-website-s-files}

La pestaña **Bloqueo de archivos** hace que el código del sitio web sea de solo lectura para su PHP, su aplicación y su cuenta SFTP. Así, un plugin con una vulnerabilidad ya no puede cambiar los archivos del sitio ni agregar otros nuevos, salvo en las carpetas que permitas, como las de subidas y cachés. Otros paneles llaman a esto protección contra manipulación.

1. Abre **Sitios web**, haz clic en el sitio web y abre la pestaña **Bloqueo de archivos**.
2. Revisa la lista de **Carpetas con permiso de escritura**. En los sitios WordPress, Joomla, Laravel, Drupal, Nextcloud, OpenCart, Moodle, Symfony y CodeIgniter, el panel rellena las carpetas de la aplicación; **Usar las carpetas de WordPress** (o de la aplicación del sitio) las vuelve a poner. Escribe una carpeta por línea, relativa a la carpeta del sitio web, por ejemplo `public/wp-content/uploads`. `*` representa un nombre de carpeta: `public/sites/*/files`. `tmp` y `logs` siempre tienen permiso de escritura.
3. Para recibir un aviso si de todos modos cambian archivos PHP, marca **Avisarme cuando cambien archivos PHP** (consulta [más abajo](#alerts-when-php-files-change)).
4. Haz clic en **Bloquear los archivos**.

La pestaña muestra entonces **Bloqueados desde** y la fecha. Para cambiar más adelante las carpetas con permiso de escritura o la alerta, edítalas y haz clic en **Guardar**: los archivos se vuelven a bloquear con la lista nueva. **Desbloquear los archivos** se lo devuelve todo al usuario del sitio web, con exactamente el propietario, el grupo y los permisos que tenía cada archivo antes.

La pestaña **Bloqueo de archivos** no aparece en las aplicaciones Docker, cuyos archivos pertenecen a sus contenedores, ni en los sitios web de proxy inverso, que no sirven archivos.

Las carpetas predefinidas:

| Aplicación | Carpetas con permiso de escritura |
|---|---|
| WordPress | `public/wp-content/uploads`, `public/wp-content/cache`, `public/wp-content/upgrade` |
| Joomla | `public/images`, `public/media`, `public/cache`, `public/administrator/cache`, `public/administrator/logs`, `public/tmp` |
| Laravel | `storage`, `bootstrap/cache` |
| Drupal | `public/sites/*/files` |
| Nextcloud | `data`, `public/config` |
| OpenCart | `storage`, `public/image/cache`, `public/image/catalog` |
| Moodle | `moodledata` |
| Symfony | `var` |
| CodeIgniter | `writable` |

Una carpeta con permiso de escritura que todavía no existe se crea, para el usuario del sitio web, al bloquear los archivos, si existe la carpeta que la contiene. La pestaña enumera las que no encontró.

### Qué cambia mientras los archivos están bloqueados {#what-changes-while-the-files-are-locked}

- El PHP del sitio web, su aplicación y su cuenta SFTP no pueden cambiar, agregar, renombrar ni eliminar nada fuera de las carpetas con permiso de escritura, ni cambiar los permisos allí. Las subidas y las cachés siguen funcionando.
- WordPress no puede actualizarse a sí mismo, instalar ni actualizar plugins y temas desde su escritorio, ni reescribir `wp-config.php` o `.htaccess`. Actualízalo en su lugar desde la pestaña **WordPress** (consulta más abajo).
- El sitio sigue funcionando: el servidor web y PHP siguen leyendo todos los archivos que podían leer antes. Los archivos que solo podía leer el usuario del sitio web, como `wp-config.php` (modo `0600`), siguen sin poder leerse para el servidor web.

Cómo funciona: los archivos y las carpetas que pertenecían al usuario del sitio web pasan a pertenecer a root, sin permiso de escritura para nadie más. Los archivos que podía leer el grupo del sitio web conservan ese grupo. Los archivos que solo podía leer su propietario reciben el grupo de bloqueo propio del sitio web (`panel-lock-<uid>`, cuyo único miembro es el usuario del sitio web). Sin módulo del kernel ni `chattr`. Lo que cambió el bloqueo se anota en `/var/lib/nefo/filelock/`, así que desbloquear lo restaura exactamente, incluso después de un fallo del sistema.

### Operaciones del panel en un sitio web bloqueado {#panel-operations-on-a-locked-website}

| Operación | Mientras está bloqueado |
|---|---|
| Pestaña **WordPress**: **Actualizar todo**, modo de mantenimiento, modo de depuración, actualizaciones automáticas | El panel desbloquea los archivos, hace el cambio y los vuelve a bloquear |
| Pasar una copia de staging a este sitio web | Se desbloquea para pasarla y luego se vuelve a bloquear |
| **Despliegue con Git** (a mano o desde el webhook) | Se desbloquea para el despliegue y luego se vuelve a bloquear |
| Restaurar una copia de seguridad de este sitio web (o de una de sus carpetas) | Se desbloquea para la restauración y luego se vuelve a bloquear |
| Análisis de malware: poner en cuarentena o restaurar un archivo | Se desbloquea mientras tanto y luego se vuelve a bloquear |
| Instalar las dependencias de la aplicación (Node.js, Python) | Se desbloquea mientras tanto y luego se vuelve a bloquear |
| **Archivos** (el administrador de archivos): editar, subir, crear, renombrar, eliminar, extraer, cambiar permisos | Permitido dentro de las carpetas con permiso de escritura y en `tmp`, `logs`; rechazado en el resto, con un mensaje de que los archivos del sitio web están bloqueados |
| Instalar una aplicación (WordPress u otra) en el sitio web | Se rechaza: primero desbloquea los archivos |
| Importar un sitio web en este sitio web (vacío) | Se rechaza: primero desbloquea los archivos |
| Clonar el sitio web, hacer una copia de staging de él, copias de seguridad | Funcionan: solo leen los archivos, también los que solo podía leer el sitio web (como `wp-config.php`). La copia pertenece a su propio sitio web y no está bloqueada |

Después de pasar una copia, de un despliegue o de una restauración, los archivos nuevos quedan bajo el bloqueo como los demás; los archivos que llegan a una carpeta con permiso de escritura siguen siendo del sitio web. Cada operación que desbloquea los archivos escribe "Unlocking the files of example.com for this change" y "Locking the files of example.com again" en el registro de su tarea. Si falla el nuevo bloqueo, la tarea lo indica y el panel lo vuelve a intentar cada hora. Mientras se ejecuta una operación, la pestaña muestra que los archivos están desbloqueados para ella.

::: warning
El bloqueo detiene al propio usuario del sitio web, no a root. Todo lo que se ejecuta como root, como una sesión SSH como root o la terminal del panel, puede seguir cambiando los archivos. Antes de quitar el panel, ejecuta `nefo unlock-files` (consulta [Desinstalar](./uninstall#unlock-the-websites-files)); si no, los archivos de los sitios web bloqueados siguen siendo propiedad de root.
:::

### Alertas cuando cambian archivos PHP {#alerts-when-php-files-change}

Con **Avisarme cuando cambien archivos PHP**, el panel toma una instantánea (una suma de verificación SHA-256 de cada archivo PHP del sitio web, incluidas las carpetas con permiso de escritura) al bloquear los archivos, y una vez al día compara los archivos PHP con ella. Cuando se cambiaron, agregaron (un archivo PHP en `uploads` es una señal clásica de una intrusión) o quitaron archivos, envía una alerta por los canales definidos en **Configuración** → **Alertas**, y la pestaña enumera los archivos. Solo vuelve a avisar cuando cambia la lista de diferencias.

Si hiciste tú los cambios, haz clic en **Tomar una instantánea nueva**. Las operaciones que desbloquean y vuelven a bloquear los archivos, como una actualización de WordPress, toman una instantánea nueva por sí solas.

La comprobación diaria lee como máximo 200 000 archivos PHP o 2 GB por sitio web. La pestaña indica cuándo se detuvo en ese límite.
