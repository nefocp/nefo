---
title: Herramientas de WordPress
description: Revisa y actualiza tus sitios WordPress con una copia de seguridad previa de la base de datos, y usa el modo de mantenimiento, el modo de depuración, las actualizaciones automáticas, el restablecimiento de contraseñas y las copias de staging.
---

# Herramientas de WordPress

Las herramientas de WordPress de Nefo muestran la versión y las actualizaciones pendientes de cada sitio WordPress, y actualizan WordPress, los plugins y los temas con un clic después de una copia de seguridad de la base de datos. También activan o desactivan el modo de mantenimiento, el modo de depuración y las actualizaciones automáticas, restablecen las contraseñas de los administradores y hacen una copia de staging que puedes pasar de vuelta al sitio en producción.

Funcionan con los sitios WordPress que instaló Nefo: desde la [tienda de aplicaciones](./apps), con el botón **Instalar WordPress** de un sitio web PHP vacío, o como copia de staging. Un WordPress que subiste tú mismo no aparece. El elemento **WordPress** del menú de la izquierda aparece cuando el servidor tiene un sitio WordPress.

Las herramientas usan wp-cli, que Nefo descarga en el primer uso desde su última versión y comprueba con la suma de verificación SHA-512 publicada con esa versión. **Software** → **Herramientas** muestra su versión y lo actualiza (consulta [Software](./software#tools)). wp-cli se ejecuta como el propio usuario de Linux de cada sitio, con la versión de PHP del sitio, y no carga plugins ni temas, así que un plugin roto no estorba.

## Instalar WordPress {#install-wordpress}

Dos formas:

- **Tienda de aplicaciones**: instala WordPress o WooCommerce en un sitio web PHP vacío. Consulta [Tienda de aplicaciones](./apps).
- **Desde un sitio web PHP vacío**: abre la pestaña **Resumen** del sitio web y haz clic en **Instalar WordPress**. Completa **Título del sitio**, **Nombre de usuario del administrador** (evita `admin`: es el primer nombre que prueban los atacantes), **Correo del administrador** y **Contraseña del administrador**, o deja la contraseña vacía para obtener una segura. Una contraseña que escribas necesita de 12 a 128 caracteres, como en la tienda de aplicaciones. Haz clic en **Instalar WordPress**.

WordPress necesita MariaDB o MySQL: instálalo primero en la página **Bases de datos** (consulta [Bases de datos](./databases)). Activa HTTPS antes de instalar, para que WordPress empiece con su dirección `https://`; si lo activas después, Nefo cambia WordPress por ti. Guarda los datos de acceso cuando aparezcan: la contraseña no se vuelve a mostrar.

## La lista de sitios {#the-site-list}

Abre **WordPress** en el menú de la izquierda. Para cada sitio, la tabla muestra:

| Columna | Qué muestra |
|---|---|
| **Sitio web** | El dominio y la versión de PHP. Haz clic para abrir la pestaña **WordPress** del sitio web. |
| **Versión** | La versión de WordPress, con "WordPress … disponible" cuando existe una más reciente, o "Actualizado". |
| **Plugins y temas** | Cuántos hay y cuántos tienen una actualización. **Mostrar las … actualizaciones disponibles** los enumera. |
| **Modos** | **Mantenimiento**, **Depuración** y **Actualizaciones automáticas** cuando están activados. |

Nefo revisa cada sitio cuando se abre la página. **Volver a comprobar**, arriba, vuelve a revisar todos los sitios; el mismo elemento en el menú de un sitio revisa solo ese sitio. Si un sitio no se puede revisar, por ejemplo porque su base de datos no funciona, la fila dice "No se pudo comprobar el sitio:" y el motivo.

El menú al final de cada fila contiene las acciones de abajo. Las mismas acciones e interruptores están en la pestaña **WordPress** de cada sitio web, junto con **WP Admin**.

## Actualizar WordPress, plugins y temas {#update-wordpress-plugins-and-themes}

1. Haz clic en **Actualizar todo** en la fila del sitio, o en la pestaña **WordPress** del sitio web.
2. Confirma: "¿Hacer una copia de seguridad de la base de datos de example.com y luego actualizar WordPress, todos los plugins y todos los temas?"

La actualización se ejecuta en segundo plano con un registro:

1. Nefo exporta la base de datos a `/var/www/example.com/tmp/panel-wp-backup-YYYYMMDD-HHMMSS.sql`. Solo el usuario del sitio puede leer el archivo, y no es accesible desde la web. Nefo conserva los tres más recientes de estos archivos. Si la copia de seguridad falla, no se actualiza nada.
2. Actualiza WordPress y su base de datos, y luego cada plugin y cada tema.
3. Un sitio que estaba en modo de mantenimiento sigue en él.

Si algunos plugins o temas no se pueden actualizar, el registro dice por qué y nombra la copia de seguridad de la base de datos. En un sitio web cuyos [archivos están bloqueados](./website-protection#lock-a-website-s-files), las herramientas los desbloquean para la actualización, el modo de mantenimiento, el modo de depuración y las actualizaciones automáticas, y después los vuelven a bloquear. El propio WordPress no puede actualizar nada mientras los archivos están bloqueados: actualízalo aquí. Para volver atrás, descarga la copia de seguridad desde **Archivos** e impórtala en la página **Bases de datos**: consulta [Bases de datos](./databases). Para tener también una copia completa de los archivos, usa [Copias de seguridad](./backups).

## Modo de mantenimiento {#maintenance-mode}

Elige **Activar el modo de mantenimiento**, o activa el interruptor **Modo de mantenimiento** en la pestaña **WordPress** del sitio web. Los visitantes ven entonces la página de mantenimiento de WordPress (HTTP 503). Sigue activado hasta que lo desactives en el panel con **Desactivar el modo de mantenimiento**.

El modo de mantenimiento es la propia página de WordPress. Para cerrar en cambio todo el sitio web desde el servidor web, suspéndelo: consulta [Sitios web](./websites).

## Modo de depuración {#debug-mode}

Elige **Activar el modo de depuración**, o activa el interruptor **Modo de depuración**. Nefo define `WP_DEBUG` y `WP_DEBUG_LOG` en `wp-config.php` y mantiene `WP_DEBUG_DISPLAY` desactivado, así que los errores se escriben en los registros y nunca se muestran a los visitantes:

- los errores de las solicitudes web van a `/var/www/example.com/logs/php-error.log`: abre la pestaña **Registros** del sitio web y elige **Errores de PHP**;
- los errores de las ejecuciones de wp-cli van a `/var/www/example.com/logs/wp-debug.log`: ábrelo en **Archivos**.

Desactívalo con **Desactivar el modo de depuración** cuando termines.

## Actualizaciones automáticas {#automatic-updates}

**Activar las actualizaciones automáticas** permite que WordPress lo actualice todo por su cuenta: WordPress, incluidas las versiones principales, cada plugin y cada tema.

**Desactivar las actualizaciones automáticas** devuelve WordPress a su comportamiento predeterminado: sigue instalando las versiones de seguridad y menores del propio WordPress, y los plugins y temas no se actualizan solos.

El ajuste se aplica a los plugins y temas instalados en el momento en que lo activas. Para los plugins y temas que agregues después, desactívalo y vuelve a activarlo.

## Restablecer la contraseña de un administrador {#reset-an-administrator-password}

1. Elige **Restablecer la contraseña de un administrador**.
2. Elige la cuenta en **Administrador**. La lista muestra los usuarios con el rol de administrador.
3. Elige **Generar una contraseña segura**, o **Escribir una contraseña** de 12 a 128 caracteres.
4. Haz clic en **Definir contraseña**.

Nefo muestra la contraseña nueva una sola vez, con un botón **Iniciar sesión**. La sesión de la cuenta se cierra en todos los dispositivos.

## Hacer una copia de staging {#make-a-staging-copy}

Una copia de staging es un segundo sitio web con una copia del sitio, donde puedes probar cambios sin tocar el sitio en producción. El sitio en producción sigue funcionando durante la copia.

1. Elige **Clonar a staging**.
2. Revisa el **Dominio del sitio de staging**. Es `staging.example.com` salvo que lo cambies.
3. Haz clic en **Crear copia de staging**.
4. Apunta el DNS del dominio de staging a este servidor y luego activa HTTPS en su pestaña **SSL / HTTPS**.

La copia recibe:

- un sitio web PHP nuevo con la misma versión de PHP, y su propio usuario de Linux;
- una base de datos nueva con una copia del contenido;
- una copia de los archivos de `public`, con `wp-config.php` apuntando a la base de datos nueva. Se omiten los drop-ins de caché (`object-cache.php`, `advanced-cache.php`) y los enlaces simbólicos;
- la dirección del sitio reemplazada por la nueva en toda la base de datos;
- una petición a los buscadores para que no la indexen;
- las mismas reglas de reescritura que el sitio en producción.

El sitio de staging empieza en `http://`, sin certificado. Si falla algún paso, Nefo quita todo lo que creó para la copia.

Nefo se niega a hacer una copia cuando:

- MariaDB no está en ejecución;
- el dominio de staging ya lo usa un sitio web, o su carpeta existe en `/var/www`;
- `wp-config.php` no está en la carpeta `public` del sitio;
- `DB_NAME` en `wp-config.php` no es un valor simple entre comillas, o el nombre de la base de datos usa caracteres distintos de letras minúsculas, dígitos y `_`;
- la base de datos del sitio está en otro servidor;
- el sitio es una red multisitio.

::: tip
Cualquiera puede abrir el sitio de staging en cuanto su DNS apunta al servidor. Para mantenerlo privado, protégelo con una contraseña en su pestaña **Bloquear acceso**: consulta [Sitios web](./websites).
:::

## Pasar la copia de staging al sitio en producción {#push-the-staging-copy-to-the-live-site}

Cuando los cambios que probaste en la copia de staging estén listos, cópialos de vuelta sobre el sitio en producción. Nefo recuerda de qué sitio se hizo cada copia de staging. Las copias hechas antes de que Nefo tuviera este botón, y los [clones](./websites#clone-a-website) de un sitio web, no están vinculados a un sitio en producción y no se pueden pasar.

1. En la fila del sitio de staging, elige **Pasar a producción**, o haz clic en **Pasar a producción** en su pestaña **WordPress**. La lista de sitios muestra "Copia de staging de example.com" debajo del sitio de staging.
2. Elige qué pasar:
   - **Archivos y base de datos**;
   - **Solo archivos (temas, plugins, subidas)**: el sitio en producción conserva sus entradas, páginas, pedidos y ajustes;
   - **Solo base de datos (entradas, páginas, ajustes)**: el sitio en producción conserva sus archivos.
3. Escribe el dominio del sitio en producción para confirmar y haz clic en **Pasar a producción**.

El paso a producción se ejecuta como una tarea con su propio registro:

1. Nefo hace una copia de seguridad de los archivos y la base de datos del sitio en producción. La copia de seguridad está en la página [Copias de seguridad](./backups). Si la copia de seguridad falla, no se pasa nada.
2. El sitio en producción muestra su página de mantenimiento. Si ya estaba en modo de mantenimiento, sigue en él después del paso.
3. **Archivos**: los archivos de la carpeta `public` del sitio de staging reemplazan a los del sitio en producción, y pertenecen al usuario de Linux del sitio en producción. Los archivos que la copia de staging no tiene se eliminan del sitio en producción; los archivos con el mismo tamaño y la misma fecha se dejan como están. El sitio en producción conserva su propio `wp-config.php`, sus drop-ins de caché (`object-cache.php`, `advanced-cache.php`) y sus enlaces simbólicos. En `.user.ini` y `wordfence-waf.php`, la carpeta de staging se reemplaza por la de producción.
4. **Base de datos**: la base de datos en producción se vacía y recibe una copia de la base de datos de staging. Nefo deshace entonces lo que hizo la copia de staging: la dirección de staging (con `http://` y `https://`, también como JSON) y la carpeta de staging se reemplazan por las de producción en todas partes, y el sitio en producción recupera su propia dirección y su ajuste para los buscadores. La caché de objetos se vacía, y WordPress vuelve a generar sus reglas de reescritura en la siguiente visita.
5. El modo de mantenimiento se desactiva, también cuando falló un paso.

::: warning
Un paso a producción reemplaza el sitio en producción. Lo que se haya agregado al sitio en producción desde que se hizo la copia de staging se pierde: los pedidos, comentarios y usuarios nuevos cuando pasas la base de datos, y las subidas cuando pasas los archivos. Si el sitio en producción no funciona después de un paso, restaura la copia de seguridad del paso 1 en la página **Copias de seguridad**.
:::

Nefo se niega a pasar a producción cuando:

- el sitio no es una copia de staging, o su sitio en producción se eliminó;
- pasas la base de datos y MariaDB no está en ejecución, la base de datos en producción no está en la página **Bases de datos**, ambos sitios usan la misma base de datos, sus prefijos de tabla (`$table_prefix`) son distintos, o uno de los motivos de la lista de arriba se aplica a cualquiera de los dos `wp-config.php`.

Los ajustes de `wp-config.php` nunca se pasan: cámbialos tú mismo en el sitio en producción.

## Eliminar una copia de staging {#delete-a-staging-copy}

Para eliminar la copia de staging, elimina su sitio web y marca **Eliminar también sus bases de datos**: consulta [Eliminar un sitio web](./websites#delete-a-website). Eliminar una copia de staging no cambia su sitio en producción.

Eliminar el sitio en producción conserva su copia de staging, pero la copia ya no se puede pasar: el cuadro de diálogo de eliminación dice "Este es el sitio en producción de la copia de staging …".
