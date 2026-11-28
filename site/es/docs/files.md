---
title: Archivos
description: Explora, edita, sube, descarga, busca, comprime y extrae archivos en el navegador, y restaura los elementos eliminados desde la papelera.
---

# Archivos

La página **Archivos** de Nefo es un administrador de archivos en el navegador: explora, edita archivos de texto de hasta 2 MB, sube hasta 4 GB de una vez, descarga, trae un archivo desde una URL, busca por nombre o por contenido, comprime y extrae, y restaura los elementos eliminados desde una papelera de 30 días. Dentro de la carpeta de un sitio web trabaja como el usuario del sitio web y no puede salir de la carpeta.

Esta página trata de cada una de estas funciones. El panel se ejecuta como root, así que también explica adónde puedes ir y qué se niega a cambiar.

## Orientarse {#find-your-way}

El administrador de archivos se abre en `/var/www`, donde cada sitio web tiene su carpeta, `/var/www/example.com`.

- Selecciona el nombre de una carpeta para abrirla. **Subir** va a la carpeta superior y **Actualizar** vuelve a leer la carpeta.
- Se puede hacer clic en cada parte de la ruta de arriba. El botón de la casa (**Sitios web**) te lleva de vuelta a `/var/www`.
- Para saltar a cualquier carpeta, selecciona el botón del lápiz (**Escribir una ruta**), escribe una ruta absoluta como `/etc/nginx` y selecciona **Ir**.
- La tabla muestra **Nombre**, **Tamaño**, **Permisos**, **Propietario** y **Modificado**. Una carpeta con más de 5 000 elementos solo muestra los primeros 5 000.

## Adónde puedes llegar {#what-you-can-reach}

Fuera de las carpetas de los sitios web trabajas como root en todo el sistema de archivos.

Dentro de la carpeta de un sitio web, `/var/www/example.com`, cada operación queda limitada a esa carpeta, y junto a la ruta aparece una insignia con el dominio:

- Las rutas no pueden salir con `..`, y los enlaces simbólicos no pueden apuntar fuera de la carpeta. Un sitio hackeado que coloca un enlace a `/etc` no puede engañar al panel para que toque archivos del sistema.
- Los archivos y carpetas nuevos pertenecen al usuario del sitio web, para que su PHP pueda usarlos.
- Se rechaza mover, comprimir o extraer entre la carpeta de un sitio web y cualquier otra carpeta. En su lugar, descarga el archivo y súbelo donde lo necesites.
- Mientras los [archivos del sitio web están bloqueados](./website-protection#lock-a-website-s-files), solo puedes cambiar sus carpetas con permiso de escritura y sus carpetas `tmp` y `logs`. En el resto de la carpeta del sitio web, el administrador de archivos se niega con un mensaje que indica que los archivos están bloqueados.

Estas carpetas no se pueden eliminar, renombrar ni cambiar de permisos:

- `/`, `/bin`, `/boot`, `/dev`, `/etc`, `/home`, `/lib`, `/lib64`, `/opt`, `/proc`, `/root`, `/run`, `/sbin`, `/srv`, `/sys`, `/tmp`, `/usr`, `/var`;
- `/var/www`, `/var/lib`, `/var/log`, `/var/lib/nefo` y `/etc/nefo`;
- la propia carpeta de cada sitio web, como `/var/www/example.com` (los archivos que contiene sí se pueden cambiar);
- la carpeta de la papelera, `/var/lib/panel-trash`, y las carpetas que la contienen.

## Editar un archivo {#edit-a-file}

1. Selecciona el nombre del archivo. Se abre en el editor.
2. Haz tus cambios. Ctrl+S guarda, y la tecla Tab inserta una tabulación.
3. Selecciona **Guardar** y luego **Cerrar**.

El editor abre archivos de texto de hasta 2 MB. Los archivos binarios y los más grandes se rechazan: descárgalos. Un archivo guardado conserva sus permisos y su propietario. Si cierras el editor con cambios sin guardar, el panel pregunta primero.

Para crear un archivo, selecciona **Archivo nuevo**, escribe un nombre en **Nombre del archivo nuevo** y selecciona **Aceptar**. Para crear una carpeta, selecciona **Carpeta nueva**. Los archivos nuevos reciben los permisos `0644` y las carpetas nuevas `0755`.

## Subir archivos {#upload-files}

1. Abre la carpeta a la que quieres subir.
2. Selecciona **Subir archivos** y elige uno o más archivos.
3. Si existe un archivo con el mismo nombre, el panel pregunta antes de reemplazarlo.

Una barra de progreso muestra la subida. Una subida puede ocupar hasta 4 GB en total. Los archivos subidos reciben los permisos `0644`. Dentro de la carpeta de un sitio web pertenecen al usuario del sitio web; en otros lugares, a root.

## Descargar un archivo {#download-a-file}

Abre el menú de un archivo y selecciona **Descargar**. Solo se pueden descargar archivos. Para descargar una carpeta, comprímela primero.

## Descargar desde una URL {#download-from-a-url}

El servidor puede traer un archivo de internet directamente a una carpeta, sin pasar por tu computadora.

1. Abre la carpeta de destino y selecciona **Descargar desde una URL**.
2. Escribe la **Dirección del archivo (URL)** y selecciona **Descargar**.
3. Un registro muestra el progreso. Puedes seleccionar **Ocultar (la descarga continúa)**.

Las reglas:

- Solo direcciones `http` y `https`, con 5 redirecciones como máximo.
- Archivos de hasta 2 GB.
- El panel nunca descarga desde la propia red del servidor: se rechazan las direcciones de loopback, privadas y de enlace local (como `127.0.0.1`, `10.x.x.x` y `169.254.169.254`), y las propias direcciones del servidor. Esto se vuelve a comprobar en cada redirección.
- El nombre del archivo viene del servidor o de la URL. Nunca se reemplaza un archivo existente: el panel agrega ` (1)`, ` (2)` y así sucesivamente.
- El archivo pertenece al propietario de la carpeta, con los permisos `0640`.

## Buscar {#search}

1. Abre la carpeta en la que quieres buscar.
2. Escribe en el cuadro de búsqueda (**Buscar en esta carpeta**) y presiona Enter.
3. En el cuadro de diálogo, ajusta **El nombre contiene**. Para buscar texto dentro de los archivos, marca **Buscar también dentro de los archivos** y completa **Texto en el archivo**.
4. Selecciona **Buscar**.

No se distinguen mayúsculas de minúsculas. Selecciona un resultado para abrir su carpeta. Un archivo de texto también se abre en el editor.

Unos límites evitan que una búsqueda acapare el servidor:

| Límite | Valor |
|---|---|
| Elementos revisados | 20 000 |
| Resultados | 200 |
| Tiempo | 20 segundos |
| Búsqueda de texto | Omite los archivos binarios y los de más de 2 MB |
| Longitud de la consulta | 200 caracteres |

Los enlaces simbólicos se enumeran pero nunca se siguen. Una búsqueda desde `/` omite `/proc`, `/sys` y `/dev`. Cuando se alcanza un límite, el panel lo indica: busca en una carpeta más pequeña o usa palabras más precisas.

## Comprimir y extraer {#compress-and-extract}

### Comprimir {#compress}

1. Marca los archivos y carpetas que quieras empaquetar.
2. Selecciona **Comprimir** en la barra de selección.
3. Escribe el **Nombre del archivo comprimido (.zip o .tar.gz)** y selecciona **Aceptar**.

El archivo comprimido se crea en la carpeta actual. Los enlaces simbólicos y los archivos especiales se omiten.

### Extraer {#extract}

1. Abre el menú de un archivo comprimido y selecciona **Extraer**.
2. El campo contiene la carpeta actual. Cámbiala para extraer en otro lugar y luego selecciona **Aceptar**.

El panel extrae archivos comprimidos `.zip`, `.tar.gz`, `.tgz` y `.tar`:

- Los archivos existentes con el mismo nombre se sobrescriben.
- Los enlaces simbólicos y los archivos especiales del archivo comprimido se omiten.
- Los archivos reciben los permisos `0644`, o `0755` cuando eran ejecutables. Pertenecen al usuario del sitio web dentro de la carpeta de un sitio web, y a root en otros lugares.
- Se rechazan los archivos comprimidos con más de 20 GB de contenido o más de 500 000 archivos.

Comprimir y extraer se ejecutan como tareas con un registro en vivo, así que continúan si sales de la página.

## Renombrar o mover {#rename-or-move}

Abre el menú de un elemento y selecciona **Renombrar**, luego escribe el nombre nuevo y selecciona **Aceptar**. El nombre nuevo se interpreta en relación con la carpeta actual, así que una ruta como `old/index.html` o `../index.html` mueve el elemento dentro del mismo sitio web o de la misma parte del servidor.

## Cambiar los permisos {#change-permissions}

Selecciona los permisos de un elemento en la columna **Permisos**, o abre su menú y selecciona **Permisos**. Escribe el valor nuevo en octal, por ejemplo `0644` para archivos o `0755` para carpetas, y selecciona **Aceptar**.

Los valores llegan hasta `0777`. Aquí no se pueden definir los bits setuid, setgid ni sticky.

La columna **Propietario** muestra el propietario y el grupo. El administrador de archivos no cambia propietarios. Para eso, usa la [terminal](./system#terminal), por ejemplo `chown -R example_com:example_com /var/www/example.com/public` con el propio usuario del sitio web.

## Eliminar archivos y la papelera {#delete-files-and-the-trash}

1. Marca los elementos, o abre el menú de uno, y selecciona **Eliminar**.
2. De forma predeterminada, los elementos van a la papelera: selecciona **Mover a la papelera**.
3. Para no usar la papelera, marca **Eliminar definitivamente (sin pasar por la papelera)** y selecciona **Eliminar definitivamente**.

::: danger
Los elementos eliminados definitivamente, incluidas las carpetas con todo su contenido, no se pueden recuperar.
:::

### Restaurar desde la papelera {#restore-from-the-trash}

1. Abre una carpeta del sitio web cuyos archivos quieres recuperar, o cualquier carpeta fuera de los sitios web para el resto del servidor.
2. Selecciona **Papelera**.
3. Selecciona **Restaurar** junto al elemento.

Cada sitio web tiene su propia papelera, y el resto del servidor comparte una. El elemento vuelve a la ruta de la que salió, con su propietario y sus permisos. Las carpetas superiores que falten se crean. Si ahora hay otra cosa en esa ruta, la restauración se rechaza: muévela antes a otro lugar.

En la papelera también puedes usar **Eliminar** para borrar un elemento para siempre, o seleccionar **Vaciar papelera**. La lista muestra los 1000 elementos eliminados más recientemente.

### Cuánto tiempo se conservan los elementos {#how-long-items-stay}

Los elementos se quedan en la papelera durante **30 días**. El panel lo comprueba cada hora y elimina para siempre los elementos más antiguos.

- La papelera se guarda en `/var/lib/panel-trash/<domain>`, o en `/var/lib/panel-trash/_server` para el resto del servidor. Solo root puede leerla, así que el usuario de un sitio web no puede llegar a sus archivos eliminados.
- Los elementos de la papelera siguen ocupando espacio en disco. Elimina definitivamente los archivos muy grandes.
- Un elemento de más de 1 GB que está en un disco distinto al de la papelera no se puede mover a ella. En su lugar, elimínalo definitivamente.
- Eliminar un sitio web también elimina su papelera.
