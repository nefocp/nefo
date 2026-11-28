---
title: Copias de seguridad
description: Haz copias de seguridad de sitios web, bases de datos, carpetas y aplicaciones según una programación con retención, en este servidor, en un almacenamiento compatible con S3, en un servidor SFTP o en Google Drive, cifradas si quieres, y restáuralas con un clic, también en un servidor nuevo.
---

# Copias de seguridad

Nefo hace copias de seguridad de sitios web, bases de datos, carpetas y aplicaciones de la tienda de aplicaciones como archivos comprimidos `.tar.gz`, a mano o según una programación diaria o semanal con retención. Guarda cada copia de seguridad en el servidor y puede subir una copia a un almacenamiento compatible con S3, a un servidor SFTP o a Google Drive, cifrada con una frase de contraseña si quieres. Para restaurar solo hacen falta la copia de seguridad y escribir su nombre para confirmar, también en un servidor nuevo.

Esta página trata de lo que contiene una copia de seguridad, las copias a mano y programadas, el almacenamiento remoto y su cifrado, la restauración y las alertas cuando falla una copia de seguridad. Para restaurar en otro servidor, consulta [Restaurar en un servidor nuevo](./restore-new-server).

## Qué contiene una copia de seguridad {#what-a-backup-contains}

| Tipo | Contiene |
|---|---|
| **Sitio web** | Los archivos de la carpeta del sitio web, `/var/www/example.com`, salvo sus carpetas `tmp` y `logs` de nivel superior, y un volcado SQL de cada base de datos vinculada al sitio web |
| **Base de datos** | Un volcado SQL de una base de datos |
| **Carpeta** | Cualquier carpeta del servidor, como `/etc/nginx` o `/opt/app`, con propietarios, permisos y enlaces simbólicos |
| **Aplicación** | Una aplicación Docker de la tienda de aplicaciones: su carpeta en `/var/lib/panel-apps`, sus contraseñas guardadas, los datos de sus contenedores y los volcados de sus bases de datos (consulta [Copias de seguridad de aplicaciones](#app-backups)) |

Vincula una base de datos a su sitio web al crearla (consulta [Bases de datos](./databases#create-a-database)), para que la copia de seguridad del sitio web la incluya. Las copias de seguridad de sitios web omiten los enlaces simbólicos y los archivos especiales.

Cada copia de seguridad es un archivo comprimido `.tar.gz` con un `manifest.json`, los archivos en `files/` y los volcados de las bases de datos en `databases/`. Los volcados se hacen con `mariadb-dump --single-transaction`, así que los sitios web siguen funcionando.

El `manifest.json` de la copia de seguridad de un sitio web también anota lo que el sitio necesita para volver a crearse en otro servidor: sus alias, la versión de PHP, las reglas de reescritura, sus propias directivas del servidor web y las contraseñas de los usuarios de sus bases de datos.

Las copias de seguridad en este servidor no están cifradas. Solo root puede leerlas. Cada almacenamiento remoto puede cifrar sus copias con una frase de contraseña antes de que salgan del servidor (consulta [Cifrar las copias](#encrypt-the-copies)).

## Dónde se guardan las copias de seguridad {#where-backups-are-kept}

Cada copia de seguridad se guarda primero en este servidor, en `/var/backups/panel`:

```
/var/backups/panel/site/example.com/example.com_20261126-030000.tar.gz
/var/backups/panel/database/shop/shop_20261126-030000.tar.gz
/var/backups/panel/folder/etc_nginx-1a2b3c4d/etc_nginx_20261126-030000.tar.gz
```

Las carpetas tienen el modo `0700` y los archivos `0600`. El panel también anota una suma de verificación SHA-256 de cada archivo comprimido. Las copias de seguridad locales ocupan espacio en disco, así que vigila el disco y cuántas copias conservan tus programaciones.

Para copiar una copia de seguridad a tu computadora, abre su menú y selecciona **Descargar**.

## Hacer una copia ahora {#back-up-now}

1. Ve a **Copias de seguridad** y selecciona **Hacer copia ahora**.
2. Elige **Sitio web**, **Base de datos**, **Carpeta** o **Aplicación**. **Aplicación** aparece cuando hay instalada una aplicación de la tienda de aplicaciones.
3. Elige el sitio web, la base de datos o la aplicación en **Copia de seguridad de**, o escribe una **Ruta de la carpeta** como `/etc/nginx`. Se sugieren las carpetas de las que ya se hicieron copias de seguridad.
4. Si hay un almacenamiento remoto configurado, puedes marcar **Subir también al almacenamiento remoto**.
5. Selecciona **Hacer copia ahora**.

El registro de progreso muestra cada paso. El panel ejecuta una sola tarea larga a la vez, así que espera primero a que termine la tarea en curso.

Para un sitio web, también puedes ir a **Sitios web** → (sitio) → **Copias de seguridad** y seleccionar **Hacer copia ahora**.

### Carpetas de las que no se puede hacer copia de seguridad {#folders-that-can-t-be-backed-up}

El panel rechaza estas carpetas, cualquier carpeta dentro de ellas y cualquier carpeta que contenga alguna de ellas. Los enlaces simbólicos se siguen antes de la comprobación.

- `/`, todo el sistema;
- `/proc`, `/sys`, `/dev` y `/run`;
- los datos del panel, `/var/lib/nefo`, y sus copias de seguridad, `/var/backups/panel`;
- `/var/lib/panel-sftp`, `/var/lib/panel-acme` y `/var/lib/panel-quarantine`.

Así, no se puede hacer copia de seguridad de `/var` ni de `/var/lib` completos, pero sí de `/var/www` o de `/etc`.

## Copias de seguridad automáticas {#automatic-backups}

1. Ve a **Copias de seguridad** y selecciona **Agregar programación**.
2. Elige **Sitio web**, **Base de datos**, **Carpeta** o **Aplicación**, y el destino.
3. En **Cuándo**, elige **Todos los días** o **Todas las semanas**. Para una copia semanal, elige el **Día**.
4. Elige la **Hora**. Es la hora del servidor, y el formulario muestra su desfase respecto de UTC.
5. Escribe el **Número de copias que se conservan**, de 1 a 365.
6. Opcional: marca **Subir también al almacenamiento remoto**.
7. Selecciona **Guardar programación**.

Para un sitio web hay un atajo: **Sitios web** → (sitio) → **Copias de seguridad** → **Todos los días a las 03:00**. Conserva 7 copias de seguridad.

La pestaña **Copias de seguridad automáticas** muestra las programaciones con su última ejecución, **Hecha** o **Falló**. Abre **La última ejecución falló** para leer el error.

Cómo se ejecutan las programaciones:

- El panel comprueba cada minuto. Si hay otra tarea en curso, la copia de seguridad espera al minuto siguiente.
- Si el servidor estaba apagado a la hora prevista, la copia de seguridad se hace en cuanto el panel vuelve a funcionar.
- Una programación nueva espera a su primera hora prevista. Para tener una copia de seguridad de inmediato, usa también **Hacer copia ahora**.

### Retención {#retention}

Después de cada copia de seguridad programada, el panel elimina las copias más antiguas de esa programación que superen el número que se conserva, en este servidor y en el almacenamiento remoto. Las copias de seguridad que hiciste a mano nunca se eliminan automáticamente.

La copia de seguridad que hace Nefo antes de eliminar un sitio web (**Hacer antes una copia de seguridad del sitio web**) cuenta como hecha a mano: nunca se elimina automáticamente. Su nombre termina en `_before-delete_<date>-<time>.tar.gz`.

Eliminar una programación la detiene. Sus copias de seguridad existentes se quedan. Eliminar un sitio web, una base de datos o una aplicación también elimina sus programaciones, y sus copias de seguridad se quedan en la página **Copias de seguridad**.

## Almacenamiento remoto {#remote-storage}

Una copia en otro proveedor te protege si se pierde este servidor. Ve a **Copias de seguridad** → **Almacenamiento remoto** y selecciona **Configurar** (o **Agregar almacenamiento** si ya hay uno configurado). Puedes configurar un almacenamiento compatible con S3, un servidor SFTP y Google Drive al mismo tiempo: las copias de seguridad se suben entonces a cada uno de ellos.

### Almacenamiento compatible con S3 {#s3-compatible-storage}

Elige **Almacenamiento compatible con S3** y completa:

| Campo | Qué escribir |
|---|---|
| **Endpoint (https://…)** | La dirección S3 del proveedor, `https://` y solo un host, sin ruta. Por ejemplo `https://s3.eu-central-1.amazonaws.com` |
| **Bucket** | El nombre del bucket |
| **Región** | La región del bucket. Vacío significa `us-east-1`. Cloudflare R2 usa `auto` |
| **Carpeta en el bucket (opcional)** | Un prefijo como `panel/` |
| **ID de clave de acceso** | |
| **Clave de acceso secreta** | Cuando cambies los ajustes más adelante, déjala vacía para conservar la actual |

Funciona con Amazon S3, Cloudflare R2, Backblaze B2, Wasabi y MinIO. Usa una clave de acceso que solo pueda usar este bucket.

Selecciona **Probar y guardar**. El panel escribe un pequeño archivo de prueba, lo vuelve a leer y lo elimina antes de guardar los ajustes.

En un almacenamiento S3, una copia de seguridad puede ocupar como máximo 5 GB, el límite de una sola subida.

### Servidor SFTP {#sftp-server}

Usa otro servidor en el que puedas iniciar sesión por SFTP (SSH), como un storage box o un segundo VPS, con una cuenta que sea solo para copias de seguridad.

Elige **Servidor SFTP** y completa:

| Campo | Qué escribir |
|---|---|
| **Servidor** | Un nombre de host o una dirección IP, como `backup.example.com` |
| **Puerto** | Normalmente `22` |
| **Nombre de usuario** | La cuenta en el servidor de copias de seguridad |
| **Iniciar sesión con** | **Contraseña** o **Clave privada** |
| **Clave privada (OpenSSH o PEM)** | La clave completa, desde `-----BEGIN` hasta `END-----`. Su clave pública debe estar en `~/.ssh/authorized_keys` en el servidor de copias de seguridad |
| **Frase de contraseña (solo para una clave cifrada)** | |
| **Carpeta en el servidor (opcional)** | Relativa a la carpeta personal de la cuenta, salvo que empiece por `/`. Se crea si hace falta |

Selecciona **Probar y guardar**. El panel se conecta, escribe, enumera, lee y elimina un archivo de prueba, y luego guarda los ajustes. Una vez guardada una clave, **Cambiar** muestra su mitad pública en **Clave pública, para ~/.ssh/authorized_keys en el servidor de copias de seguridad**, para que puedas volver a copiarla.

### Fijación de la clave de host {#host-key-pinning}

**Probar y guardar** confía en la clave de host que presenta el servidor SFTP en ese momento, y la muestra en **Clave de host de confianza**. Cada conexión posterior debe presentar exactamente esa clave, o la subida de la copia de seguridad falla.

Antes de confiar en el almacenamiento, compara la huella con la propia clave del servidor de copias de seguridad. En el servidor de copias de seguridad, ejecuta:

```sh
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
```

Si el servidor de copias de seguridad se reinstaló o sus claves cambiaron a propósito, selecciona **Cambiar** y de nuevo **Probar y guardar** para confiar en la clave nueva. El panel avisa entonces de que la clave se reemplazó.

::: warning
Una clave de host cambiada también puede significar que alguien está interceptando la conexión. Si no esperabas el cambio, revisa el servidor de copias de seguridad antes de confiar en la clave nueva.
:::

### ¿Puede Nefo hacer copias de seguridad en Google Drive? {#google-drive}

Sí, desde v0.24.0. Las copias de seguridad van a una carpeta de tu Google Drive. El panel solo pide acceso a los archivos que crea él mismo (el permiso `drive.file` de Google): no puede ver ni cambiar nada más de tu Drive. Una carpeta que crees a mano en Drive es invisible para el panel, aunque tenga el nombre correcto.

Google Drive necesita un cliente OAuth propio, en un proyecto de Google Cloud. No cuesta nada y se crea una sola vez.

Google Drive se probó solo contra un servicio de Google simulado, no contra una cuenta de Google real.

#### Crear el cliente OAuth {#create-the-oauth-client}

Los nombres en negrita de estos pasos son de Google, no del panel.

1. Abre la [consola de Google Cloud](https://console.cloud.google.com/) y crea un proyecto, o elige uno.
2. En **APIs and services** → **Library**, habilita la **Google Drive API**.
3. Configura la **OAuth consent screen** con el tipo de usuario **External** y luego cambia su estado de publicación a **In production**.
4. En **Credentials**, crea un **OAuth client ID** del tipo **TVs and Limited Input devices**.
5. Guarda el ID de cliente (termina en `.apps.googleusercontent.com`) y el secreto de cliente.

::: warning
Mientras el estado de publicación de la pantalla de consentimiento sea **Testing**, Google retira el acceso del panel a los 7 días, y entonces las subidas fallan. Cámbialo a **In production**.
:::

#### Conectar {#connect}

1. En **Almacenamiento remoto**, selecciona **Configurar** o **Agregar almacenamiento**, y elige **Google Drive** en **Tipo**. **Cómo crear el cliente OAuth** repite los pasos anteriores.
2. Escribe el **ID de cliente** y el **Secreto de cliente**.
3. Opcional: cambia la **Carpeta en Google Drive**. La predeterminada es `Nefo backups/<the server's host name>` (el nombre de host del servidor). Separa las carpetas con `/`; las carpetas que faltan se crean.
4. Opcional: marca **Cifrar las copias de seguridad antes de subirlas** (consulta [Cifrar las copias](#encrypt-the-copies)).
5. Selecciona **Conectar con Google Drive**. El panel muestra una dirección, `google.com/device`, y un código como `ABCD-EFGH`.
6. En cualquier dispositivo (tu teléfono sirve), abre esa dirección, inicia sesión con la cuenta de Google que debe guardar las copias de seguridad, escribe el código y permite el acceso.
7. El panel espera mientras lo haces, y muestra cuánto tiempo sigue siendo válido el código. En cuanto Google responde, el panel crea la carpeta, escribe, enumera, lee y elimina allí un pequeño archivo de prueba, y guarda los ajustes.

La lista de **Almacenamiento remoto** muestra entonces la carpeta, la cuenta de Google y un enlace, **Abrir en Google Drive**.

Puedes salir de la página mientras el panel espera: vuelve a abrir **Copias de seguridad** → **Almacenamiento remoto** para ver el código.

#### Cambiar o volver a conectar {#change-or-reconnect}

Selecciona **Cambiar** en la fila de Google Drive:

- Para cambiar la carpeta o el cifrado, edítalos y selecciona **Probar y guardar**. No hace falta volver a iniciar sesión. Si hace falta, se crea una carpeta nueva. Las copias de seguridad subidas antes se quedan en la carpeta anterior, y el panel ya no las encuentra allí para una restauración.
- Para usar otra cuenta de Google u otro cliente OAuth, o después de que se revocara el acceso, selecciona **Volver a conectar** y escribe el código nuevo. Deja vacío el **Secreto de cliente** para conservar el guardado del mismo cliente.

**Probar y guardar** también repara una carpeta que se eliminó o se movió a la papelera de Drive: vuelve a crear la carpeta.

Conserva el mismo proyecto de Google Cloud. Google da a cada proyecto acceso solo a los archivos que creó ese proyecto, así que con un cliente OAuth de otro proyecto el panel no puede ver las copias de seguridad que subió antes.

#### Cómo funcionan las subidas {#how-uploads-work}

- Cada copia de seguridad se sube en trozos de 8 MB. Si se corta la conexión o Google está ocupado, el panel pregunta a Google cuánto llegó y sigue desde ahí. Lo reintenta durante unos minutos antes de rendirse.
- Antes de una subida, el panel comprueba el espacio libre de la cuenta de Google y rechaza la subida de inmediato si la copia de seguridad no cabe.
- Las descargas para una restauración también siguen desde donde se detuvieron.
- La carpeta en Drive tiene las mismas subcarpetas que `/var/backups/panel`: `site/example.com/…`, `database/shop/…`.

#### Desconectar {#disconnect}

**Eliminar** en la fila de Google Drive olvida los ajustes y avisa a Google de que el panel renuncia a su acceso. Las copias de seguridad ya subidas se quedan en tu Google Drive.

#### Cuando algo sale mal {#when-something-goes-wrong}

| Mensaje | Qué hacer |
|---|---|
| El acceso a Google Drive se revocó o caducó | Selecciona **Cambiar** → **Volver a conectar**. Si ocurre unos 7 días después de conectar, cambia primero el estado de publicación de la pantalla de consentimiento a **In production**. |
| Google Drive está lleno | Libera espacio en la cuenta de Google o compra más almacenamiento, y vuelve a subir con **Subir**. |
| La carpeta de Google Drive … se eliminó / está en la papelera de Drive | Restaura la carpeta desde la papelera de Drive (sus copias de seguridad vuelven con ella), o selecciona **Cambiar** → **Probar y guardar** para crear una carpeta nueva. |
| Google rechazó el cliente OAuth | Revisa el ID de cliente y el secreto, y que el tipo del cliente sea **TVs and Limited Input devices**. |
| El código caducó antes de que lo escribieras | Vuelve a seleccionar **Conectar con Google Drive** para obtener un código nuevo. |

### Cifrar las copias {#encrypt-the-copies}

Cada almacenamiento (S3, SFTP y Google Drive) puede cifrar las copias de seguridad que recibe. La copia del almacenamiento es entonces ilegible sin la frase de contraseña, para el proveedor y para cualquiera que entre en la cuenta del almacenamiento. La copia de este servidor sigue siendo un `.tar.gz` normal.

1. En el formulario del almacenamiento, marca **Cifrar las copias de seguridad antes de subirlas**.
2. Escribe una **Frase de contraseña** de al menos 12 caracteres, y la misma en **Repite la frase de contraseña**. Una frase de varias palabras funciona bien.
3. Selecciona **Probar y guardar** (o **Conectar con Google Drive**). El archivo de prueba también se cifra.

::: danger
Sin la frase de contraseña, las copias de seguridad cifradas no se pueden restaurar: ni tú, ni el panel, ni nadie. Guárdala fuera de este servidor, por ejemplo en un gestor de contraseñas. Si se pierde este servidor, la frase de contraseña es todo lo que necesitas, junto con el almacenamiento, para recuperar las copias de seguridad.
:::

Qué cambia:

- La copia cifrada se llama como la copia de seguridad, con `.enc` agregado: `example.com_20261126-030000.tar.gz.enc`.
- En la lista de **Copias de seguridad**, la etiqueta del almacenamiento en la columna **Guardada en** muestra un candado en lugar de una nube.
- La fila del almacenamiento en **Almacenamiento remoto** muestra **Cifrado**.
- La frase de contraseña no se vuelve a mostrar nunca. Para conservarla cuando cambies otros ajustes, deja vacía la **Frase de contraseña**.

Cambiar la frase de contraseña:

- Una frase de contraseña nueva solo se aplica a las subidas nuevas. Las copias ya subidas conservan la frase con la que se cifraron; no se vuelven a cifrar.
- Este servidor recuerda las frases anteriores (hasta 10), así que las restauraciones aquí siguen funcionando.
- En otro servidor, necesitas la frase de contraseña que estaba en uso cuando se subió la copia.
- Si desactivas el cifrado, las copias ya subidas siguen cifradas, y este servidor conserva su frase de contraseña para restaurarlas.

Cómo funciona: el panel cifra el archivo comprimido en este servidor, mientras lo sube, con AES-256-GCM. La clave se deriva de la frase de contraseña mediante scrypt, con una sal aleatoria nueva para cada archivo, así que dos copias de la misma copia de seguridad se ven distintas. El archivo comprimido se sella en trozos de 64 KB, cada uno numerado y el último marcado, así que se rechaza una copia que se modificó, se cortó o se armó con trozos de otras copias. No se guarda ninguna clave en el servidor, aparte de la frase de contraseña en los ajustes del panel. El formato del archivo se describe en [Restaurar en un servidor nuevo](./restore-new-server#format-of-encrypted-copies).

Cifrar cuesta poco: derivar la clave tarda alrededor de un cuarto de segundo y usa 64 MB de memoria durante un momento, una vez por subida, y la copia crece en un encabezado de 64 bytes y 16 bytes por cada 64 KB. En un almacenamiento S3, el panel lee la copia de seguridad dos veces, porque S3 necesita la suma de verificación de la copia cifrada antes de que empiece la subida.

### Subir y quitar el almacenamiento {#uploading-and-removing-storage}

- Una copia de seguridad hecha sin **Subir también al almacenamiento remoto** se puede subir más tarde: abre su menú y selecciona **Subir**. El panel la copia a cada almacenamiento que todavía no la tenga, cifrada para los almacenamientos que cifran.
- La columna **Guardada en** muestra **Este servidor** y una etiqueta por cada copia remota: S3, SFTP o Google Drive, con un candado si la copia está cifrada.
- Quitar los ajustes del almacenamiento no elimina las copias de seguridad que ya se subieron allí.
- **Buscar copias de seguridad** en la fila de un almacenamiento enumera las copias de seguridad que hay en él y no están en la lista, por ejemplo en un servidor nuevo: consulta [Restaurar en un servidor nuevo](./restore-new-server).

## Restaurar una copia de seguridad {#restore-a-backup}

1. Ve a **Copias de seguridad** y busca la copia de seguridad en la lista.
2. Selecciona **Restaurar** en su fila.
3. Escribe el nombre que pide el cuadro de diálogo (el dominio, el nombre de la base de datos, la ruta de la carpeta o el nombre de la aplicación) y selecciona **Restaurar**.

El panel primero comprueba la suma de verificación SHA-256 del archivo comprimido, y rechaza un archivo dañado antes de cambiar nada. Si el archivo ya no está en este servidor, se descarga desde el almacenamiento remoto.

Si la copia está cifrada, el panel la descifra mientras la descarga, con la frase de contraseña del almacenamiento (o una anterior que este servidor recuerde). La comprobación SHA-256 se hace después sobre el archivo comprimido descifrado, como con cualquier otra copia de seguridad. Cuando una copia de seguridad solo está en el almacenamiento remoto y su copia está cifrada, el cuadro de diálogo **Restaurar** tiene un campo **Frase de contraseña de cifrado (opcional)**. Déjalo vacío para usar la frase de contraseña del almacenamiento. Escribe la frase que estaba en uso cuando se subió la copia si desde entonces el almacenamiento se volvió a configurar, por ejemplo después de quitarlo y agregarlo con una frase de contraseña nueva.

La restauración se detiene antes de cambiar nada, con un mensaje en el registro, cuando:

- la frase de contraseña no abre la copia ("this copy is encrypted and the passphrase does not unlock it");
- la copia se modificó o está dañada ("the encrypted copy is damaged or was changed");
- la copia está incompleta ("the encrypted copy is incomplete").

Si hay copias en varios almacenamientos, el panel prueba con el siguiente.

::: danger
Una restauración sobrescribe el contenido actual. Los cambios hechos desde la copia de seguridad se pierden.
:::

Qué reemplaza una restauración:

| Tipo | Qué ocurre |
|---|---|
| **Sitio web** | Se eliminan todos los archivos de la carpeta del sitio web, salvo `tmp` y `logs`, y los archivos de la copia de seguridad se vuelven a poner con el usuario del sitio web como propietario. Cada base de datos de la copia de seguridad se vacía y se llena con su volcado. Una base de datos que ya no existe se vuelve a crear, vinculada al sitio web, con la contraseña de su usuario tomada de la copia de seguridad (las copias hechas antes de v0.24.0 no la tienen: la base de datos recibe entonces una contraseña nueva). Una base de datos que ya no está vinculada a ningún sitio web se vincula al sitio restaurado. En WordPress, la contraseña actual de la base de datos se escribe en `wp-config.php` |
| **Base de datos** | La base de datos se elimina, se vuelve a crear vacía y se llena con el volcado |
| **Carpeta** | Primero, la copia de seguridad se descomprime junto al contenido actual. Solo cuando eso funcionó se reemplaza el contenido de la carpeta, así que una copia de seguridad dañada deja la carpeta como estaba. Los archivos agregados desde la copia de seguridad se eliminan. Vuelven los propietarios, los permisos y los enlaces simbólicos. Una carpeta que falta se crea |
| **Aplicación** | Los contenedores de la aplicación se detienen, su carpeta y sus datos se reemplazan, los contenedores se vuelven a iniciar y se cargan los volcados de las bases de datos. También vuelven las contraseñas guardadas en la copia de seguridad |

Los volcados de las copias de seguridad de sitios web y bases de datos se cargan como una [importación](./databases#import) en la página **Bases de datos**: como un usuario temporal de MariaDB o MySQL que solo puede usar esa base de datos, así que un volcado nunca escribe en otra base de datos.

### Restaurar un sitio web que se eliminó {#restore-a-website-that-was-deleted}

Si el sitio web de una copia de seguridad ya no existe en este servidor, por ejemplo porque lo eliminaste, la restauración lo vuelve a crear a partir de la copia: el mismo dominio y los mismos alias, la versión de PHP, las reglas de reescritura, los archivos y las bases de datos. El cuadro de diálogo de restauración lo avisa antes. Lo que una copia de seguridad no contiene, como el certificado, las tareas cron y el acceso SFTP, no se crea. Consulta los detalles en [Restaurar en un servidor nuevo](./restore-new-server#restoring-a-website-that-doesn-t-exist-here).

Las copias de seguridad hechas con Nefo 0.24.1 o posterior registran el tipo de cada sitio web, así que un sitio web que ya no existe vuelve con el mismo tipo: un sitio web de aplicación (Node.js, Python, Java), con su entorno de ejecución, su comando de inicio, su puerto y su límite de memoria, y un sitio web de proxy inverso, con su dirección y sus opciones. Si el entorno de ejecución de la aplicación no está instalado, la restauración se detiene y te pide que primero lo instales en la página **Software**. Las copias hechas con versiones anteriores solo registran la versión de PHP: un sitio web así vuelve como sitio web estático, de modo que primero vuelve a crearlo con el mismo dominio y el mismo tipo, y luego restaura. Para una aplicación Docker, vuelve a instalar la aplicación en el mismo dominio desde la [tienda de aplicaciones](./apps), luego restaura su copia de seguridad del tipo **Aplicación**, y también la copia del sitio web si la carpeta del sitio tenía archivos propios, como la música de Navidrome.

## Restaurar en otro servidor {#restore-on-another-server}

Un servidor nuevo con Nefo puede encontrar las copias de seguridad de tu almacenamiento remoto con **Buscar copias de seguridad**, agregarlas a su lista y restaurarlas, sitios web incluidos. Necesitas los datos de acceso del almacenamiento, y la frase de contraseña si las copias están cifradas. Consulta [Restaurar en un servidor nuevo](./restore-new-server).

## Copias de seguridad de aplicaciones {#app-backups}

Las aplicaciones Docker de la [tienda de aplicaciones](./apps) guardan sus datos fuera de la carpeta del sitio web, en `/var/lib/panel-apps/<name>`. Reciben copias de seguridad del tipo **Aplicación**:

- Hacer una copia de seguridad de un sitio web que ejecuta una aplicación Docker también hace una copia de seguridad de la aplicación, con la misma programación. La retención elimina las copias antiguas de la aplicación junto con las del sitio web.
- Para hacer una copia de seguridad de una sola aplicación, elige **Aplicación** en **Hacer copia ahora** o en **Agregar programación** de la página **Copias de seguridad**, o ve a **Tienda de aplicaciones** → **Instaladas** → **Hacer copia ahora**. Las aplicaciones de servicio, como PostgreSQL, no tienen sitio web, así que dales su propia programación.
- Antes de actualizar una aplicación, el panel hace primero una copia de seguridad. Si esa copia falla, la actualización no empieza.

Según la aplicación, el panel pausa sus contenedores mientras copia sus archivos, o primero vuelca sus bases de datos con sus propias herramientas. Restaura las copias de seguridad de aplicaciones en la página **Copias de seguridad**, como cualquier otra. Una restauración necesita que la aplicación esté instalada. Después de desinstalar una aplicación, sus copias de seguridad se quedan en la lista con su nombre y su logotipo: vuelve a instalar la aplicación (si tiene sitio web, en el mismo dominio) y luego restaura.

## Eliminar una copia de seguridad {#delete-a-backup}

Abre el menú de una copia de seguridad y selecciona **Eliminar**. La copia de seguridad se elimina de este servidor y del almacenamiento remoto.

## Alertas cuando falla una copia de seguridad {#alerts-when-a-backup-fails}

El panel puede enviar una alerta por Telegram, correo electrónico, Discord, Slack o un webhook cuando falla una copia de seguridad o una restauración. Ve a **Configuración** → **Alertas** y activa **Falla una copia de seguridad, una restauración o la renovación de un certificado**. Consulta [Correo y alertas](./mail-alerts).

La pestaña **Copias de seguridad automáticas** también muestra **Falló** para una programación cuya última ejecución falló.
