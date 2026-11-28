---
title: PostgreSQL
description: Instala PostgreSQL desde tu distribución o las versiones 15 a 19 del proyecto PostgreSQL, crea bases de datos y propietarios, permite el acceso remoto por TLS, importa y exporta, haz copias de seguridad y actualiza a una versión mayor más reciente.
---

# PostgreSQL

Nefo ejecuta PostgreSQL junto a MariaDB o MySQL: la versión de tu distribución, o una versión mayor de la 15 a la 19 del repositorio propio del proyecto PostgreSQL (apt.postgresql.org). La pestaña **Bases de datos** → **PostgreSQL** crea bases de datos con sus propietarios, las importa y exporta, abre una base de datos a direcciones elegidas por TLS y actualiza el servidor a una versión mayor más reciente, con un volcado previo.

PostgreSQL es el servidor de bases de datos que prefieren muchas aplicaciones Laravel, Django, Node.js y Odoo. MariaDB (o MySQL) y PostgreSQL pueden funcionar uno al lado del otro. Para MariaDB y MySQL, consulta [Bases de datos](./databases).

## Instalar PostgreSQL {#install-postgresql}

1. Ve a **Software** → **PostgreSQL** y selecciona **Instalar**.
2. En **Versión**, elige qué PostgreSQL instalar:
   - la versión de tu distribución, como "PostgreSQL 16, la versión de la distribución", marcada como **Recomendada**. Se actualiza con el sistema;
   - una versión de apt.postgresql.org, como "PostgreSQL 18 desde apt.postgresql.org", y las demás versiones mayores que ofrece el repositorio del proyecto PostgreSQL. Elige una cuando una aplicación necesite una versión más reciente (o más antigua) que la que trae tu sistema.
3. Selecciona **Instalar** y espera uno o dos minutos. El registro de progreso muestra cada paso.

| Sistema | Versión de la distribución | Desde apt.postgresql.org |
|---|---|---|
| Ubuntu 22.04 | 14 | 15, 16, 17, 18, 19 |
| Ubuntu 24.04 | 16 | 15, 17, 18, 19 |
| Ubuntu 26.04 | 18 | 15, 16, 17, 19 |
| Debian 12 | 15 | 16, 17, 18, 19 |
| Debian 13 | 17 | 15, 16, 18, 19 |

El panel ofrece las versiones mayores a las que el proyecto PostgreSQL todavía da soporte. En otro sistema, o en un procesador para el que el repositorio no tiene paquetes, solo se ofrece la versión de la distribución y el cuadro de diálogo indica el motivo.

Cuando eliges una versión de apt.postgresql.org, el panel:

- descarga la clave de firma del repositorio de `https://www.postgresql.org/media/keys/ACCC4CF8.asc` y la rechaza salvo que su huella sea `B97B 0AFC AA1A 47F0 44F2 44A0 7FCC 7D46 ACCC 4CF8` (la clave "PostgreSQL Debian Repository");
- la escribe en `/etc/apt/keyrings/panel-pgdg.gpg` y agrega `/etc/apt/sources.list.d/panel-pgdg.list`, que confía en esa clave solo para este repositorio;
- instala el paquete del servidor por su nombre, como `postgresql-18`. Nunca instala el paquete `postgresql` de ese repositorio, porque siempre arrastra la versión mayor más reciente, y una actualización del sistema agregaría entonces un segundo servidor.

Las actualizaciones de seguridad de cualquiera de las dos versiones llegan con las actualizaciones del sistema (página **Sistema**, o `apt upgrade`).

La tarjeta de **Software** muestra entonces **De la distribución** o **De apt.postgresql.org**, igual que la parte superior de **Bases de datos** → **PostgreSQL**.

Solo se ejecuta una versión de PostgreSQL a la vez. El panel rechaza:

- otra versión mientras hay una instalada. En su lugar, [actualízala](#upgrade-postgresql), o desinstálala primero;
- una versión distinta de aquella cuyos datos siguen en el servidor de una instalación anterior (en `/var/lib/postgresql/<version>`). Vuelve a instalar esa versión para usar los datos y luego actualízala si quieres, o elimina primero los datos.

apt se ejecuta con la configuración regional C, así que el clúster que crea el paquete guarda el texto como `SQL_ASCII`. Nefo vuelve a crear entonces ese clúster nuevo, todavía vacío, con UTF-8 (`C.UTF-8`). Un clúster que existía antes de la instalación nunca se vuelve a crear.

## Cómo se ajusta PostgreSQL {#how-postgresql-is-tuned}

Nefo escribe sus ajustes en `/etc/postgresql/<version>/main/conf.d/90-nefo.conf`. Los ajustes de memoria dependen de la RAM del servidor:

| RAM del servidor | `shared_buffers` | `effective_cache_size` | `work_mem` | `maintenance_work_mem` | `max_connections` |
|---|---|---|---|---|---|
| Menos de 768 MB | 64MB | 256MB | 2MB | 16MB | 30 |
| 768 MB o más | 128MB | 512MB | 4MB | 32MB | 40 |
| 1.5 GB o más | 256MB | 1024MB | 4MB | 64MB | 50 |
| 3 GB o más | 512MB | 2048MB | 8MB | 128MB | 75 |
| 7 GB o más | 1024MB | 4096MB | 16MB | 256MB | 100 |

`effective_cache_size` es aproximadamente la mitad de la RAM, porque el mismo servidor también ejecuta el servidor web, PHP y tal vez MariaDB. En todos los servidores, Nefo también define `listen_addresses = 'localhost'`, `password_encryption = 'scram-sha-256'`, `random_page_cost = 1.1` (para almacenamiento SSD) y `jit = off`. En un VPS que es un contenedor (LXC), la RAM es el límite de memoria del contenedor (consulta [Un VPS que es un contenedor](./first-steps#a-vps-that-is-a-container)).

En reposo y vacío, PostgreSQL necesita aproximadamente de 30 a 40 MB para su puñado de procesos en segundo plano. A medida que se llena su caché, crece hacia `shared_buffers` más unos 40 MB, y cada conexión abierta agrega unos pocos MB. La página **Software** muestra `shared_buffers` más 40 MB.

::: warning
No edites `90-nefo.conf` ni `pg_hba.conf`. Instalar PostgreSQL de nuevo los reescribe. Tus propias líneas de `pg_hba.conf` se conservan cuando cambias el acceso remoto: el panel solo toca sus propias líneas (consulta [Acceso remoto](#postgresql-remote-access)).
:::

## Quién puede conectarse {#who-can-connect}

Nefo escribe `/etc/postgresql/<version>/main/pg_hba.conf`:

```
local   all  postgres                peer
local   all  all                     scram-sha-256
host    all  all       127.0.0.1/32  scram-sha-256
host    all  all       ::1/128       scram-sha-256
```

- Solo el usuario del sistema `postgres` inicia sesión sin contraseña, en el socket local. El panel administra el servidor de esa forma.
- Todos los demás necesitan una contraseña, en el socket o por TCP desde este servidor. A diferencia de lo predeterminado en la distribución, los demás usuarios de Linux no tienen inicio de sesión sin contraseña ("peer"), así que el usuario de Linux de un sitio web no puede abrir un rol de base de datos con el mismo nombre sin su contraseña.
- Otras máquinas no pueden conectarse, salvo que permitas su dirección para una base de datos (consulta [Acceso remoto](#postgresql-remote-access)). PostgreSQL solo escucha en localhost mientras ninguna base de datos permita el acceso remoto.

## Crear una base de datos PostgreSQL {#create-a-postgresql-database}

1. Ve a **Bases de datos** → **PostgreSQL** y selecciona **Agregar base de datos**.
2. Escribe un nombre en **Base de datos**. Un rol con el mismo nombre es el propietario de la base de datos.
3. Deja **Contraseña** vacío para obtener una contraseña aleatoria segura, o escribe la tuya.
4. Opcional: elige un sitio en **Sitio web**. Una copia de seguridad de ese sitio web incluye entonces esta base de datos, y eliminar el sitio web con sus bases de datos también la elimina.
5. Selecciona **Crear base de datos**.

La tarjeta de datos de conexión muestra **Base de datos**, **Usuario**, **Contraseña**, **Host** (`localhost`) y **Puerto** (`5432`). Para una aplicación Laravel, por ejemplo, define `DB_CONNECTION=pgsql`, `DB_HOST=127.0.0.1`, `DB_PORT=5432` y la base de datos, el usuario y la contraseña de la tarjeta.

Las reglas:

- Los nombres y las contraseñas siguen las [reglas de MariaDB](./databases#create-a-database). También se rechazan los nombres propios de PostgreSQL: `postgres`, `template0`, `template1`, `public`, `all` y cualquiera que empiece con `pg_`.
- Un mismo nombre no puede usarse en los dos servidores: si una base de datos o un usuario de MariaDB se llama `shop`, una base de datos PostgreSQL no puede llamarse así, y al revés. El panel también rechaza un nombre que PostgreSQL ya usa para una base de datos o un rol.
- El rol puede iniciar sesión, no es superusuario y no puede crear bases de datos ni roles. Ningún otro rol puede conectarse a la base de datos, porque el panel les revoca `CONNECT` a todos los demás.
- Las bases de datos nuevas usan UTF-8 con la configuración regional `C.UTF-8`.

La contraseña nunca llega a PostgreSQL en texto claro: el panel solo envía su verificador SCRAM-SHA-256, así que tampoco puede aparecer en el registro de PostgreSQL.

**Mostrar acceso**, **Contraseña nueva** y **Eliminar** funcionan como en MariaDB. Eliminar también cierra las conexiones abiertas de la base de datos y quita sus líneas de acceso remoto y sus reglas de firewall. Una base de datos quitada fuera del panel se marca como **No se encontró en PostgreSQL**.

## ¿Pueden otros servidores conectarse a una base de datos PostgreSQL? {#postgresql-remote-access}

Sí, a partir de v0.24.0, una base de datos a la vez. De forma predeterminada, PostgreSQL solo acepta conexiones desde el propio servidor. Puedes permitir que direcciones elegidas se conecten a una base de datos, con el usuario (rol) y la contraseña propios de esa base de datos, por TLS.

Antes de empezar, instala el firewall ufw en la página **Firewall y SSH** y actívalo. El panel rechaza el acceso remoto mientras ufw no esté instalado.

1. Ve a **Bases de datos** → **PostgreSQL**, abre el menú de la base de datos y selecciona **Acceso remoto**.
2. En **Direcciones permitidas**, escribe una dirección. Selecciona **Agregar una dirección** para agregar más.
3. Selecciona **Guardar**.

La fila muestra entonces "Remoto: 1" (el número de direcciones), y la parte superior de la pestaña muestra "1 abierta a otros servidores".

Conéctate a la dirección IP pública del servidor con el usuario y la contraseña que muestra **Mostrar acceso**, y con TLS, por ejemplo:

```sh
psql "host=203.0.113.10 port=5432 dbname=shop user=shop sslmode=require"
```

### Direcciones permitidas {#allowed-addresses}

- Direcciones IPv4 o IPv6 individuales, como `198.51.100.7` o `2001:db8::1`.
- Rangos en notación CIDR: IPv4 no más amplios que `/16` (como `198.51.100.0/24`), IPv6 no más amplios que `/48` (como `2001:db8:5::/48`).
- Como máximo 10 direcciones por base de datos.
- Se rechazan las direcciones de loopback, de enlace local (`fe80::`), `0.0.0.0`, `::` y las de multidifusión.

### Qué cambia el panel {#what-the-panel-changes}

| Dónde | Cambio |
|---|---|
| `pg_hba.conf` (PostgreSQL 16 y posteriores) | Una línea al final, `include_if_exists nefo-remote.conf`, con un comentario. Las direcciones permitidas están en `/etc/postgresql/<version>/main/nefo-remote.conf` |
| `pg_hba.conf` (PostgreSQL 14 y 15) | Un bloque al final entre `# BEGIN Nefo remote access` y `# END Nefo remote access` |
| Esas líneas | Una por dirección, solo para esta base de datos y su propio rol, con inicio de sesión por contraseña: `hostssl "shop" "shop" 198.51.100.7/32 scram-sha-256` |
| Ajustes de PostgreSQL | `/etc/postgresql/<version>/main/conf.d/95-nefo-remote.conf` define `listen_addresses = '*'`. Solo existe mientras al menos una base de datos permita el acceso remoto |
| Firewall (ufw) | Para cada dirección, una regla que permite el puerto del clúster (5432 de forma predeterminada) por TCP solo desde esa dirección, con el comentario `PostgreSQL name` |

El panel nunca cambia las demás líneas de `pg_hba.conf`. Cuando tus propias líneas van antes que las del panel, se leen primero: una línea `reject` tuya para la misma dirección tiene prioridad. Antes de que PostgreSQL recargue un `pg_hba.conf` modificado, el panel le pide a PostgreSQL que lo compruebe (`pg_hba_file_rules`). Si PostgreSQL encuentra un error, se vuelven a poner los archivos anteriores y no cambia nada.

Si el clúster usa un puerto distinto de 5432, las reglas del firewall usan ese puerto y los datos de conexión lo muestran.

### Cifrado {#encryption}

El cuadro de diálogo indica si las conexiones desde otras máquinas van cifradas:

- **Las conexiones de otras máquinas van cifradas**: las líneas son `hostssl`, así que PostgreSQL rechaza las conexiones desde esas direcciones que no usan TLS. Los clientes necesitan `sslmode=require` (o algo más estricto). Debian y Ubuntu crean los clústeres con el certificado autofirmado del sistema (`/etc/ssl/certs/ssl-cert-snakeoil.pem`). Si el clúster tiene TLS desactivado, el panel crea su propio certificado autofirmado (`nefo-ssl.crt` y `nefo-ssl.key` junto a `pg_hba.conf`) y activa TLS en `95-nefo-remote.conf`; TLS se vuelve a desactivar con el último acceso remoto. Ambos certificados son autofirmados, así que un cliente no puede verificar el servidor con ellos: `sslmode=require` cifra, pero `verify-ca` y `verify-full` fallan salvo que instales un certificado propio.
- **Las conexiones de otras máquinas no van cifradas**: PostgreSQL no tiene TLS, así que las líneas son `host`. Usa un túnel SSH o una VPN para todo lo que sea sensible.

### Reinicios {#restarts}

Un `listen_addresses` nuevo requiere un reinicio; las direcciones permitidas y TLS solo requieren una recarga. Por eso PostgreSQL se reinicia la primera vez que se activa el acceso remoto, y de nuevo cuando se desactiva para la última base de datos. Los sitios web pierden su base de datos durante unos segundos y las transacciones abiertas se cancelan. Agregar o quitar direcciones más adelante no lo reinicia.

Si en ese momento hay otra tarea en curso, el cuadro de diálogo dice **PostgreSQL tiene que reiniciarse antes de aceptar conexiones de otras máquinas.** Selecciona **Reiniciar** en el cuadro de diálogo cuando la tarea haya terminado.

Si PostgreSQL no se inicia con `95-nefo-remote.conf`, el panel quita ese archivo e inicia PostgreSQL sin él, para que tus sitios web sigan funcionando.

::: warning
- Si el firewall está desactivado, el puerto está abierto para todos. PostgreSQL sigue dejando iniciar sesión solo a las direcciones de la lista, y solo con la contraseña.
- Incluye solo direcciones que controles y que no cambien.
:::

Para desactivar el acceso remoto, vuelve a abrir **Acceso remoto** y selecciona **Desactivar**. Eliminar una base de datos también quita sus líneas y sus reglas de firewall; cuando era la última base de datos con acceso remoto, PostgreSQL se reinicia para volver a escuchar solo en localhost.

## Exportar e importar una base de datos PostgreSQL {#export-and-import-a-postgresql-database}

**Exportar (.sql.gz)** descarga un volcado SQL simple comprimido con gzip, hecho con `pg_dump` como el propio rol de la base de datos, con `--no-owner --no-privileges`, así que se importa en cualquier base de datos.

**Importar** acepta un archivo `.sql` o `.sql.gz` de hasta 4 GB:

- `psql` se ejecuta como el propio rol de la base de datos, nunca como `postgres`, con su contraseña en un archivo temporal que solo puede leer root.
- El archivo se ejecuta en una sola transacción con `ON_ERROR_STOP`: cuando falla una sentencia, no se importa nada y el error indica la línea.
- Se rechazan los comandos propios de psql, las líneas que empiezan con una barra invertida (como `\connect`, `\!` o `\copy`): psql se ejecuta como root, y esos comandos podrían ejecutar programas o escribir archivos en el servidor. El panel inicia psql en modo restringido (`\restrict` con una clave aleatoria) y quita las líneas `\restrict` y `\unrestrict` que escriben las versiones más recientes de `pg_dump`. Un psql sin modo restringido (anterior a 17.6, 16.10, 15.14 o 14.19) rechaza todas las importaciones: instala primero las actualizaciones del sistema.
- Fallan las sentencias que crean o cambian roles, que dan objetos a otros roles o que necesitan un superusuario. Vuelve a exportar ese volcado con `pg_dump --no-owner --no-privileges`.
- `SET transaction_timeout`, que escriben `pg_dump` 17 y posteriores, se omite en servidores más antiguos.
- Después, el panel vuelve a definir la contraseña del rol, por si el archivo la cambió.

Con **Vaciar primero la base de datos (eliminar todo su contenido)** marcada, la base de datos se elimina y se vuelve a crear antes de la importación, con el mismo rol como propietario.

## Actualizar PostgreSQL a una versión más reciente {#upgrade-postgresql}

Puedes pasar el servidor a una versión mayor más reciente de apt.postgresql.org, por ejemplo de la 15 de la distribución a la 17, conservando cada base de datos, usuario, contraseña y ajuste de acceso remoto.

1. Ve a **Bases de datos** → **PostgreSQL** y selecciona **Actualizar**. El botón aparece cuando se ofrece una versión más reciente y PostgreSQL está en ejecución.
2. Elige la **Versión nueva**.
3. Lee **Qué ocurre** y marca **Entiendo que los sitios web y las aplicaciones pierden su base de datos durante la actualización.**
4. Selecciona **Actualizar a 17** (la versión que elegiste) y sigue el registro.

Lo que hace el panel, en orden:

1. Comprueba que la versión nueva es más reciente, que hay un solo clúster (`main`), que no hay paquetes de extensiones instalados para la versión antigua (como `postgresql-15-postgis-3`) y que el disco tiene espacio para el doble de los datos más 256 MB.
2. Guarda todas las bases de datos con `pg_dumpall` en un solo archivo comprimido, `/var/backups/nefo-postgresql/postgresql-<old>-before-<new>-<date>.sql.gz`, que solo puede leer root. Todavía no ha cambiado nada: si el volcado falla, la actualización se detiene ahí.
3. Quita el paquete `postgresql` de la distribución, que solo apunta a una versión (desde apt.postgresql.org apuntaría a la más reciente e instalaría un segundo servidor), después de marcar `postgresql-<old>` como instalado a mano. Luego agrega el repositorio apt.postgresql.org si todavía no está e instala `postgresql-<new>`. Se elimina el clúster vacío que crea este paquete. Si apt trae consigo cualquier otro servidor PostgreSQL, la actualización se detiene y se revierte.
4. Ejecuta `pg_upgradecluster -v <new> -m upgrade <old> main`. Mientras tanto, PostgreSQL está detenido: alrededor de un minuto para un servidor pequeño, más para bases de datos grandes. `pg_upgrade` copia los archivos de datos, así que el clúster antiguo queda como estaba.
5. Le da al clúster nuevo el `pg_hba.conf` antiguo, el ajuste del panel (`conf.d/90-nefo.conf`) y los ajustes de acceso remoto (desde la versión 16, las direcciones permitidas pasan a `nefo-remote.conf`), lo reinicia y comprueba que se ejecuta en el mismo puerto con las mismas bases de datos, y que cada una responde.
6. Solo entonces elimina el clúster antiguo y quita sus paquetes (`postgresql-<old>` y `postgresql-client-<old>`).

Si falla cualquier paso del 3 al 5, el panel revierte: elimina el clúster nuevo, quita los paquetes de la versión nueva, vuelve a poner los ajustes del clúster antiguo y lo inicia de nuevo. El registro termina con "PostgreSQL 15 runs again with its data as before". Tus bases de datos quedan como estaban. El repositorio apt.postgresql.org se conserva una vez que se instalaron paquetes desde él, porque `postgresql-common` y `libpq5` se actualizaron desde él y, de lo contrario, ya no recibirían actualizaciones; el paquete `postgresql` de la distribución no se vuelve a poner.

La actualización se rechaza, con el motivo en el cuadro de diálogo, cuando:

| Motivo | Qué hacer |
|---|---|
| Hay paquetes de extensiones instalados para la versión antigua | pg_upgrade necesita las mismas extensiones compiladas para la versión nueva, y el panel no las instala. Actualiza a mano con `pg_upgradecluster` después de instalarlas para la versión nueva, o quita antes las extensiones |
| Existe más de un clúster | El panel solo administra `main`. Actualiza a mano |
| No hay suficiente espacio en disco | Libera la cantidad que indica el mensaje |

Una base de datos que `pg_upgrade` no puede trasladar, por ejemplo una tabla con una columna `regproc`, hace fallar el paso 4; el panel revierte entonces como se describe.

### Los volcados conservados {#the-dumps-kept}

El archivo guardado se queda en `/var/backups/nefo-postgresql` en cualquier caso, y la pestaña **PostgreSQL** lo enumera en **Volcados conservados**: para qué actualización se hizo (como "PostgreSQL 15 → 17"), cuándo, su tamaño y su ruta. Elimínalo con **Eliminar este volcado** cuando estés seguro de que todo funciona.

Para restaurarlo todo a mano en un servidor recién instalado:

```sh
zcat postgresql-15-before-17-20261127-101500.sql.gz | runuser -u postgres -- psql -X -d postgres
```

## PostgreSQL en Adminer {#postgresql-in-adminer}

Adminer necesita la extensión `pgsql` de PHP para PostgreSQL. El panel la instala para la versión de PHP que ejecuta Adminer, y solo junto con PostgreSQL: cuando instalas PostgreSQL con Adminer ya instalado, o Adminer con PostgreSQL ya instalado. Al mismo tiempo se reemplaza un Adminer que instaló una versión anterior del panel, la compilación solo para MySQL.

**Abrir en Adminer** en la fila de una base de datos abre el inicio de sesión de PostgreSQL de Adminer con el servidor `localhost`, el usuario y la base de datos ya completados. Escribe la contraseña que muestra **Mostrar acceso**.

## PostgreSQL en las copias de seguridad {#postgresql-in-backups}

Las copias de seguridad de un sitio web incluyen sus bases de datos PostgreSQL, y una copia de seguridad de base de datos puede contener una base de datos PostgreSQL, cuando la pidas o según una programación. El volcado es la salida simple de `pg_dump`, hecho como el rol de la base de datos, y se guarda como `postgres/<name>.sql` en el archivo comprimido, junto a los volcados de MariaDB.

Las copias de seguridad, las exportaciones y las importaciones usan el `pg_dump` y el `psql` de la versión instalada, de la distribución o de apt.postgresql.org, y el puerto del clúster, también cuando no es 5432.

Restaurar vacía cada base de datos (se elimina y se vuelve a crear, con el mismo rol como propietario) e importa el volcado como ese rol. Una base de datos eliminada después de la copia de seguridad se vuelve a crear, con la contraseña de su rol guardada en la copia cuando la copia la tiene (a partir de v0.24.0), o si no, con una contraseña nueva.

## Desinstalar PostgreSQL {#uninstall-postgresql}

Desinstala PostgreSQL en la página **Software**. El panel se niega mientras PostgreSQL contenga alguna base de datos distinta de `postgres`, `template0` y `template1`, incluidas las bases de datos creadas fuera del panel. También se niega mientras PostgreSQL esté detenido, porque entonces no puede comprobarlo. Los datos de `/var/lib/postgresql` y los ajustes de `/etc/postgresql` se conservan salvo que marques la casilla y escribas `postgresql`. Consulta [Software](./software#postgresql).

Desinstalar también quita el repositorio apt.postgresql.org y su clave, si los agregó el panel, y las reglas de firewall que queden para bases de datos PostgreSQL. Cuando la extensión `pgsql` de PHP (agregada para Adminer) todavía necesita `libpq5`, el panel vuelve a poner `libpq5` en la versión de la distribución, para que reciba de nuevo las actualizaciones de seguridad de tu sistema.

La Tienda de aplicaciones también tiene PostgreSQL como servicio Docker en 127.0.0.1, para las aplicaciones que quieren su propio servidor. Ese no aparece en la página **Bases de datos**: consulta [Tienda de aplicaciones](./apps#install-a-service).
