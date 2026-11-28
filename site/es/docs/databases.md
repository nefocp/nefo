---
title: Bases de datos
description: Instala MariaDB o MySQL, elige una versión de MariaDB, actualiza MariaDB, cambia entre MariaDB y MySQL, crea bases de datos, importa y exporta SQL, permite el acceso remoto, lee el registro de consultas lentas y usa Adminer, phpMyAdmin y el explorador de claves de Redis.
---

# Bases de datos

Nefo ejecuta un servidor de bases de datos compatible con MySQL por servidor: MariaDB (la de tu distribución, o 11.4, 11.8 o 12.3 LTS de MariaDB) o MySQL 8.4 LTS, ajustado para un VPS pequeño y accesible solo desde el propio servidor. La página **Bases de datos** crea bases de datos y usuarios, importa y exporta SQL, abre una base de datos a direcciones elegidas y muestra las consultas lentas. PostgreSQL puede funcionar a su lado.

Esta página trata de:

- elegir MariaDB o MySQL y una versión de MariaDB, instalarlo, cómo lo ajusta Nefo para un servidor pequeño, actualizar MariaDB y cambiar de uno a otro;
- crear bases de datos, mostrar sus datos de acceso y cambiar contraseñas;
- la importación y exportación de SQL;
- el acceso remoto para una base de datos a la vez;
- el registro de consultas lentas;
- Adminer y phpMyAdmin, interfaces de bases de datos en tu navegador;
- el explorador de claves de Redis, en la pestaña **Redis** de la misma página, y el estado de Memcached.

PostgreSQL tiene su propia pestaña **PostgreSQL** en esta página y su propia página de documentación: consulta [PostgreSQL](./postgresql). MongoDB, y PostgreSQL como contenedor, están disponibles en la [Tienda de aplicaciones](./apps) como aplicaciones Docker que solo escuchan en 127.0.0.1. Esas aplicaciones Docker no aparecen en la página **Bases de datos**.

## Elegir MariaDB o MySQL {#choose-mariadb-or-mysql}

Un servidor ejecuta un solo servidor de bases de datos compatible con MySQL: MariaDB o MySQL 8.4 LTS. Lo eliges al instalarlo, en la página **Bases de datos**, en la tarjeta **Servidor de bases de datos** de la página **Software** o en la configuración inicial (**Configurar este servidor**). Mientras uno está instalado, el panel se niega a instalar el otro; más adelante puedes [cambiar](#switch-between-mariadb-and-mysql) de uno a otro, con tus bases de datos.

| | MariaDB (recomendado) | MySQL 8.4 LTS |
|---|---|---|
| Procedencia | Los paquetes de tu distribución, o una serie LTS del repositorio propio de MariaDB (consulta [Versiones de MariaDB](#mariadb-versions)) | El repositorio de Oracle, `repo.mysql.com` (la rama `mysql-8.4-lts`) |
| Memoria, aproximadamente | Su buffer pool más unos 100 MB | Su buffer pool más unos 150 MB, o más unos 300 MB con el performance schema activado, que el panel activa a partir de unos 3.5 GB de RAM (es decir, en los servidores de 4 GB) |
| Soporte | Mientras tu distribución le dé soporte; una serie del repositorio de MariaDB, hasta la fecha de fin que MariaDB fija para ella | Oracle da soporte a 8.4 LTS hasta 2032 |
| Sistemas | Todos los sistemas que admite Nefo | Ubuntu 22.04, 24.04 y 26.04 y Debian 12 y 13, solo en amd64 (x86-64) |
| Adecuado para | WordPress y la mayoría de las aplicaciones PHP | Aplicaciones que necesitan MySQL en sí, o un equipo acostumbrado a él |

Ambos funcionan con todo lo de esta página, con las copias de seguridad, las copias de sitios web, el staging de WordPress y las aplicaciones PHP de la tienda de aplicaciones. Adminer y phpMyAdmin abren los dos. En un servidor arm64, o en un sistema para el que Oracle no tiene paquetes, la página **Software** solo ofrece MariaDB e indica el motivo.

## Instalar MariaDB {#install-mariadb}

1. Ve a **Bases de datos**.
2. Selecciona **Instalar MariaDB (recomendado)**. También puedes instalarlo en la página **Software**: deja **MariaDB** seleccionado en la tarjeta **Servidor de bases de datos** y selecciona **Instalar MariaDB**.
3. Espera de uno a dos minutos. El registro de progreso muestra cada paso.

Nefo instala los paquetes `mariadb-server` y `mariadb-client` de tu distribución. Luego escribe sus ajustes, inicia MariaDB y quita los usuarios anónimos y la base de datos `test` que traen algunas versiones.

Para instalar en su lugar una serie más reciente del repositorio propio de MariaDB, consulta [Versiones de MariaDB](#mariadb-versions).

### Cómo se ajusta MariaDB {#how-mariadb-is-tuned}

Nefo escribe sus ajustes en `/etc/mysql/mariadb.conf.d/90-panel.cnf`. El ajuste de memoria más grande, el buffer pool de InnoDB, depende de la RAM del servidor:

| RAM del servidor | `innodb_buffer_pool_size` |
|---|---|
| Menos de 1.5 GB | 128M |
| 1.5 GB o más | 256M |
| 3 GB o más | 512M |
| 7 GB o más | 1G |

Los demás ajustes son iguales en todos los servidores:

| Ajuste | Valor | Por qué |
|---|---|---|
| `bind-address` | `127.0.0.1` | MariaDB solo acepta conexiones desde este servidor |
| `skip-name-resolve` | activado | Sin consulta DNS en cada conexión |
| `performance_schema` | `OFF` | Ahorra memoria |
| `max_connections` | `100` | |
| `character-set-server` | `utf8mb4` | Unicode completo, emoji incluidos |
| `collation-server` | `utf8mb4_unicode_ci` | |

MariaDB usa entonces aproximadamente el tamaño de su buffer pool más 100 MB. La página **Software** muestra esta estimación.

::: warning
No edites `90-panel.cnf`. El panel lo reescribe.
:::

## ¿Qué versiones de MariaDB puedo instalar? {#mariadb-versions}

De forma predeterminada, MariaDB viene de tu distribución. Así, sus correcciones de seguridad llegan con las actualizaciones del sistema y no se agrega ningún otro repositorio. En su lugar, puedes instalar una serie de soporte a largo plazo (LTS) del repositorio propio de MariaDB, `dlm.mariadb.com`, cuando necesites una MariaDB más reciente que la de tu distribución:

| Serie | MariaDB le da soporte hasta | Se ofrece en |
|---|---|---|
| 11.4 LTS | 29 de mayo de 2029 | Ubuntu 22.04 y 24.04, Debian 12 |
| 11.8 LTS | 4 de junio de 2028 | Ubuntu 22.04 y 24.04, Debian 12 |
| 12.3 LTS | 12 de junio de 2029 | Ubuntu 22.04, 24.04 y 26.04, Debian 12 y 13 |

Una serie solo se ofrece donde es más reciente que la MariaDB propia de la distribución:

| Sistema | MariaDB de la distribución | Series que se ofrecen |
|---|---|---|
| Ubuntu 22.04 | 10.6 | 11.4, 11.8, 12.3 |
| Ubuntu 24.04 | 10.11 | 11.4, 11.8, 12.3 |
| Debian 12 | 10.11 | 11.4, 11.8, 12.3 |
| Ubuntu 26.04 | 11.8 | 12.3 |
| Debian 13 | 11.8 | 12.3 |

MariaDB 12.3 no instala Galera (el software de clúster de MariaDB, que un servidor único no usa); 11.4 y 11.8 lo traen porque sus paquetes lo exigen.

Para instalar una serie:

1. Ve a **Software**. En la tarjeta **Servidor de bases de datos**, deja **MariaDB** seleccionado.
2. Debajo, elige la versión, como **11.4 LTS de MariaDB**. La versión de tu distribución, como "10.11, la de la distribución", sigue siendo la predeterminada.
3. Selecciona **Instalar MariaDB 11.4**.

La configuración inicial (**Configurar este servidor**) ofrece las mismas versiones en **MariaDB**, y también `nefo setup --db mariadb-11.4` (consulta [Línea de comandos](./cli#nefo-setup)).

Lo que hace el panel con una serie del repositorio de MariaDB:

- Descarga la clave de firma de MariaDB de `https://supplychain.mariadb.com/MariaDB-Server-GPG-KEY` (o de `keyserver.ubuntu.com`) y solo la acepta si su huella es exactamente `177F 4010 FE56 CA33 3630 0305 F165 6F24 C74C D1D8`, la "MariaDB Signing Key" que figura en la documentación de MariaDB. La guarda como `/etc/apt/keyrings/panel-mariadb.gpg`, de confianza solo para este repositorio.
- Escribe `/etc/apt/sources.list.d/panel-mariadb.list` con una sola línea, solo para esa serie, como `https://dlm.mariadb.com/repo/mariadb-server/11.4/repo/ubuntu noble main`. Tu servidor recibe entonces las versiones menores de 11.4 con las actualizaciones del sistema, nunca una serie más reciente por sí sola.
- Comprueba que apt va a instalar realmente los paquetes de MariaDB de esa serie y luego instala `mariadb-server` y `mariadb-client`, ajustados como la MariaDB de la distribución.

Desinstalar MariaDB también quita el archivo del repositorio y la clave.

El panel se niega:

- a instalar una serie mientras hay otra MariaDB instalada: usa **Actualizar** en su lugar (ver más abajo);
- a instalar una MariaDB más antigua que aquella cuyos datos están en `/var/lib/mysql`, conservados de una instalación anterior, porque no podría leerlos. Instala esa versión o una más reciente, o mueve antes el directorio de datos a otro lugar.

## Actualizar MariaDB {#upgrade-mariadb}

Puedes actualizar la MariaDB instalada a una serie LTS más reciente del repositorio de MariaDB, como la 10.11 de la distribución a 11.4, o la 11.8 de Debian 13 a 12.3, sin mover tus bases de datos.

1. Ve a **Software**. En la tarjeta **Servidor de bases de datos**, selecciona **Actualizar**.
2. Elige la serie. El cuadro de diálogo muestra lo que va a ocurrir y cuánto espacio libre en disco necesita.
3. Selecciona **Actualizar a MariaDB 11.4** y sigue el registro.

La actualización sigue este orden:

1. Se exporta cada base de datos, en un archivo SQL comprimido con gzip por base de datos, a una carpeta nueva de `/var/backups/panel/db-server/`. El panel conserva estos volcados hasta que los eliminas.
2. Se descargan los paquetes de la serie nueva y se conservan los de la instalada, para que la vuelta atrás funcione sin red.
3. MariaDB pasa a solo lectura, el panel cuenta las filas de cada tabla y MariaDB se detiene. Su directorio de datos se copia en `/var/lib/mysql.before-11.4-<date>-<time>` y `/etc/mysql`, en la carpeta de los volcados. A partir de aquí, los sitios web no pueden usar sus bases de datos hasta que termine la actualización, normalmente unos minutos.
4. Los paquetes nuevos reemplazan a los antiguos e inician MariaDB sobre los mismos datos, todavía en solo lectura. Luego `mariadb-upgrade` actualiza sus tablas del sistema.
5. El panel comprueba que MariaDB informa la versión nueva, que cada base de datos tiene las mismas tablas con el mismo número de filas, vistas, rutinas, triggers y eventos, y que cada usuario de base de datos del panel puede iniciar sesión con su contraseña.
6. MariaDB vuelve a aceptar escrituras y se elimina la copia del directorio de datos.

Si falla cualquier paso a partir del 4, el panel vuelve a poner la MariaDB anterior: quita los paquetes nuevos, restaura `/etc/mysql` y el directorio de datos copiado, vuelve a instalar los paquetes antiguos e inicia MariaDB. El registro termina con el motivo. Tus bases de datos quedan entonces como estaban antes de la actualización.

::: warning No se puede volver a una serie anterior
Después de una actualización, MariaDB ya actualizó sus archivos de datos y sus tablas del sistema, que una MariaDB más antigua no puede leer. El panel rechaza volver a una versión anterior, como de 11.8 a 11.4, e indica el motivo. Para volver atrás, exporta las bases de datos, desinstala MariaDB con sus datos, instala la versión más antigua e impórtalas.
:::

El panel necesita espacio libre en disco para los volcados, la copia del directorio de datos y los paquetes: aproximadamente una vez y media el directorio de datos más 400 MB. Sin ese espacio, se niega a empezar. También se niega mientras MariaDB está detenida, o cuando la configuración del servidor mueve el directorio de datos fuera de `/var/lib/mysql`.

## Instalar MySQL {#install-mysql}

1. Ve a **Bases de datos** y selecciona **Instalar MySQL 8.4 en su lugar**. En la página **Software**, elige **MySQL 8.4 LTS** en la tarjeta **Servidor de bases de datos** y selecciona **Instalar MySQL 8.4 LTS**.
2. Espera de dos a cuatro minutos. El registro de progreso muestra cada paso.

Nefo agrega por sí mismo el repositorio de Oracle, en lugar de instalar el paquete `mysql-apt-config` de Oracle:

- Descarga la clave de firma de `https://repo.mysql.com/RPM-GPG-KEY-mysql-2025` y solo la acepta si su huella es exactamente `BCA4 3417 C3B4 85DD 128E C6D4 B7B3 B788 A8D3 785C`, la clave de MySQL Release Engineering que figura en la documentación de MySQL. Guarda la clave como `/etc/apt/keyrings/panel-mysql.gpg`.
- `/etc/apt/sources.list.d/panel-mysql.list` confía en esa clave solo para este repositorio, y solo incluye la rama `mysql-8.4-lts`.
- Oracle renueva la clave de vez en cuando; la copia actual caduca el 23 de octubre de 2027. Una vez por semana, el panel busca una copia renovada de la misma clave y reemplaza la anterior, para que `apt-get update` siga funcionando.

Después instala `mysql-community-server` y `mysql-community-client` sin preguntas. La cuenta `root` de MySQL no tiene contraseña: inicia sesión solo a través del socket local (`auth_socket`), como root en el servidor, y el panel no guarda ninguna contraseña de root. Por último, el panel quita cualquier usuario anónimo y la base de datos `test`.

### Cómo se ajusta MySQL {#how-mysql-is-tuned}

Nefo escribe `/etc/mysql/mysql.conf.d/90-panel.cnf` antes de que el paquete inicie MySQL por primera vez, así que MySQL nunca se ejecuta con los valores predeterminados de Oracle en tu servidor. Los ajustes de memoria dependen de la RAM del servidor:

| RAM del servidor | `innodb_buffer_pool_size` | `max_connections` | `table_open_cache` | `temptable_max_ram` |
|---|---|---|---|---|
| Menos de 1.5 GB | 128M | 75 | 400 | 32M |
| 1.5 GB o más | 256M | 100 | 600 | 64M |
| 3 GB o más | 512M | 150 | 1000 | 128M |
| 7 GB o más | 1024M | 200 | 2000 | 256M |

Los demás ajustes:

| Ajuste | Valor | Por qué |
|---|---|---|
| `bind-address` | `127.0.0.1` | MySQL solo acepta conexiones desde este servidor, hasta que una base de datos permita el acceso remoto |
| `mysqlx` | `OFF` | Sin receptor del X Protocol en el puerto 33060: PHP y el panel usan el protocolo clásico |
| `disable_log_bin` | activado | Sin registros binarios que llenen el disco: el servidor no tiene réplicas |
| `performance_schema` | `OFF` por debajo de unos 3.5 GB de RAM, `ON` a partir de ahí (es decir, en los servidores de 4 GB) | Ocupa de 100 a 200 MB por sí solo |
| `skip-name-resolve`, `host_cache_size` | activado, `0` | Sin consulta DNS en cada conexión |
| `innodb_log_buffer_size` | `16M` | El valor predeterminado de MySQL 8.4 es 64M |
| `character-set-server`, `collation-server` | `utf8mb4`, `utf8mb4_unicode_ci` | Lo mismo que MariaDB, para que los volcados pasen de uno a otro |

MySQL usa entonces aproximadamente el tamaño de su buffer pool más 150 MB, o más 300 MB con el performance schema activado. La página **Software** muestra esta estimación; la página **Servicios** muestra lo que usa realmente.

Los usuarios inician sesión con `caching_sha2_password`, el método predeterminado de MySQL 8.4, que admiten `mysqli` y PDO de PHP. `mysql_native_password` sigue desactivado, tal como lo trae MySQL 8.4: calcula el hash con SHA-1 sin sal, y MySQL 9 lo eliminó. Por eso, un hash de contraseña antiguo de otro servidor, como los que guarda cPanel, no se puede reutilizar en MySQL: una base de datos traída de otro lugar necesita su contraseña, o una nueva.

En MySQL, el panel ejecuta el SQL con el comando `mysql` en lugar de `mariadb`, exporta con `mysqldump`, y el servicio se llama `mysql`.

::: warning
No edites `90-panel.cnf`. El panel lo reescribe.
:::

## ¿Puedo cambiar entre MariaDB y MySQL? {#switch-between-mariadb-and-mysql}

Sí, a partir de v0.24.0. Un servidor ejecuta un solo servidor de bases de datos, pero puedes reemplazar MariaDB por MySQL 8.4 LTS, o MySQL por MariaDB, con tus bases de datos, sus usuarios y sus privilegios.

1. Ve a **Software**. En la tarjeta **Servidor de bases de datos**, en **MySQL 8.4 LTS en su lugar** (o **MariaDB en su lugar**), selecciona **Cambiar a MySQL 8.4 LTS** (o **Cambiar a MariaDB**).
2. Lee el cuadro de diálogo. Enumera las bases de datos y las cuentas que pasan al otro servidor, y cómo inicia sesión cada cuenta después.
3. Escribe `mysql` (o `mariadb`) para confirmar y selecciona el botón del cambio.

Volver a MariaDB instala la MariaDB de tu distribución. Si no hay ninguna base de datos, la misma tarea solo reemplaza un servidor por el otro.

### Qué hace el cambio {#what-the-switch-does}

1. El panel descarga los paquetes del servidor nuevo y conserva los del antiguo para poder volver atrás. Todavía no cambia nada para tus sitios web.
2. El servidor antiguo pasa a solo lectura: los sitios web aún pueden leer sus bases de datos, pero no escribir. El panel lee sus cuentas y privilegios, cuenta las filas de cada tabla y exporta cada base de datos a una carpeta nueva de `/var/backups/panel/db-server/`.
3. El servidor antiguo se detiene. Su directorio de datos se mueve a `/var/lib/mysql.mariadb-<date>-<time>` (o `.mysql-…`) y `/etc/mysql` se copia en la carpeta de los volcados. Sus paquetes se quitan. A partir de aquí, los sitios web no pueden usar sus bases de datos hasta que termine el cambio.
4. El servidor nuevo se instala como lo instala la página **Software**, y queda en solo lectura. El acceso remoto y el registro de consultas lentas se vuelven a configurar como estaban.
5. Se crean las cuentas, se importa cada base de datos y se vuelven a conceder los privilegios. Los volcados de MySQL 8 se adaptan a MariaDB como los adapta la importación de sitios web: sus intercalaciones `utf8mb4_0900_*` pasan a ser las de MariaDB (`utf8mb4_unicode_520_ci`, la intercalación propia de un idioma o `utf8mb4_bin`) y se omiten los ajustes exclusivos de MySQL. Los volcados de MariaDB 11 que van a MySQL reciben las intercalaciones de MySQL en lugar de las `uca1400` de MariaDB (`utf8mb4_uca1400_ai_ci` pasa a ser `utf8mb4_0900_ai_ci`).
6. El panel comprueba que cada base de datos tiene las mismas tablas con el mismo número de filas, vistas, rutinas, triggers y eventos, y que cada usuario de base de datos del panel inicia sesión con su contraseña. Luego el servidor vuelve a aceptar escrituras y se elimina el directorio de datos antiguo.

Si falla cualquier paso a partir del 3, el panel quita el servidor nuevo, restaura la configuración y el directorio de datos del antiguo, vuelve a instalar sus paquetes y lo inicia: vuelves a estar donde estabas. Los volcados se conservan en ambos casos.

Cuánto tarda depende del tamaño de tus bases de datos: la exportación, la importación y el recuento de filas leen cada uno todos los datos una vez.

### Cuentas y contraseñas {#accounts-and-passwords}

MariaDB y MySQL guardan las contraseñas de forma distinta, así que no todas las contraseñas pueden pasar al otro servidor:

| Cuenta | Después del cambio |
|---|---|
| El usuario de una base de datos de la página **Bases de datos** | Su contraseña, que el panel conoce, en `localhost` y en cada dirección permitida para el acceso remoto |
| Una cuenta que creaste tú con una contraseña `mysql_native_password`, que va a MariaDB | Su contraseña, a partir de su hash |
| Una cuenta que inicia sesión a través del socket (`unix_socket` en MariaDB, `auth_socket` en MySQL) | Vuelve a iniciar sesión a través del socket |
| Cualquier otra cuenta que creaste tú, como una cuenta de MariaDB que va a MySQL (MySQL 8.4 mantiene desactivado `mysql_native_password`) o una cuenta de MySQL que va a MariaDB (MariaDB no puede leer `caching_sha2_password`) | Se crea con una contraseña aleatoria nueva y bloqueada. El registro enumera cada una. Define una contraseña como root en el servidor: `ALTER USER 'name'@'localhost' IDENTIFIED BY '…' ACCOUNT UNLOCK;` |

Sus privilegios se vuelven a conceder a partir del `SHOW GRANTS` del servidor antiguo. Un privilegio que el otro servidor no tiene se omite y se indica en el registro, igual que las concesiones de roles y de proxy, que el panel no traslada.

### Qué sigue funcionando {#what-keeps-working}

- Los sitios web conservan sus ajustes: se conectan a `localhost`, a través del mismo socket en ambos servidores.
- phpMyAdmin y Adminer abren el servidor nuevo.
- Copias de seguridad: las nuevas usan el programa de exportación del servidor nuevo, y una copia de seguridad hecha antes del cambio se puede restaurar después.
- El acceso remoto sigue abierto para las mismas direcciones.

### Cuándo se niega el panel {#when-the-panel-refuses}

El cuadro de diálogo enumera lo que hay que corregir antes, en **Corrige esto primero:**, y el botón del cambio queda desactivado:

| Motivo | Qué hacer |
|---|---|
| El panel no conoce la contraseña del usuario de una base de datos (una base de datos traída de otro lugar solo con el hash de su contraseña) | Define una contraseña nueva para esa base de datos en la página **Bases de datos** y ponla en los ajustes del sitio web |
| Hacia MySQL: tablas que usan uno de los motores de almacenamiento propios de MariaDB (Aria y otros), secuencias o tablas versionadas por el sistema | Conviértelas, por ejemplo con `ALTER TABLE … ENGINE=InnoDB`, o elimínalas |
| Hacia MySQL: columnas `INET4`, `INET6` o `UUID` | Cámbialas, por ejemplo a `VARCHAR` |
| Hacia MySQL: vistas, triggers o rutinas cuyo definer tiene un nombre de usuario de más de 32 caracteres | Vuelve a crearlos con otro definer |
| Un nombre de base de datos que empieza con un guion | Cambia el nombre de la base de datos |
| No hay suficiente espacio libre en disco: los volcados y los datos importados junto al directorio de datos antiguo necesitan aproximadamente el doble del tamaño de tus bases de datos más 700 MB | Libera espacio; elimina volcados antiguos |

El cambio también se rechaza mientras el servidor está detenido, cuando la configuración del servidor mueve el directorio de datos fuera de `/var/lib/mysql` y, en el caso de MySQL, donde MySQL no se ofrece (arm64, o un sistema para el que Oracle no tiene paquetes).

Algunas diferencias solo aparecen al importar los datos, como un índice que MariaDB marca como `IGNORED` y que MySQL no conoce. La importación falla entonces y el panel vuelve al servidor antiguo, con el error en el registro.

## Volcados de las actualizaciones y los cambios {#dumps-from-upgrades-and-switches}

La tarjeta **Servidor de bases de datos** de la página **Software** enumera las carpetas en **Volcados conservados**: cuándo, qué (como "MariaDB 10.11.14 → MariaDB 11.4"), si terminó como **Hecho**, **Revertido** o **Falló**, cuántas bases de datos, su tamaño y la carpeta. Cada base de datos es un archivo, `001.sql.gz`, `002.sql.gz` y así sucesivamente, con `manifest.json`, que indica la base de datos de cada archivo, porque los nombres de bases de datos pueden contener caracteres que un nombre de archivo no admite. Solo root puede leer la carpeta.

Para recuperar una base de datos, descomprime su archivo e impórtalo en la página **Bases de datos**. Para eliminar una carpeta, selecciona **Eliminar estos volcados** (la papelera que está a su lado). El panel se niega mientras haya una actualización o un cambio en curso.

## Crear una base de datos {#create-a-database}

1. Ve a **Bases de datos** y selecciona **Agregar base de datos**.
2. Escribe un nombre en **Base de datos**. Se crea un usuario con el mismo nombre, y ese usuario solo puede usar esta base de datos.
3. Deja **Contraseña** vacío para obtener una contraseña aleatoria segura, o escribe la tuya.
4. Opcional: elige un sitio en **Sitio web**. Una copia de seguridad de ese sitio web incluye entonces esta base de datos.
5. Selecciona **Crear base de datos**.

Una tarjeta titulada **Datos de conexión de** seguido del nombre de la base de datos muestra entonces **Base de datos**, **Usuario**, **Contraseña** y **Host**. El host siempre es `localhost`.

Las reglas para los nombres y las contraseñas:

- Los nombres empiezan con una letra y solo usan `a-z`, `0-9` y `_`, con 32 caracteres como máximo. Estos nombres están reservados: `mysql`, `information_schema`, `performance_schema`, `sys`, `test`, `root`, `mariadb`, `phpmyadmin`, `debian_sys_maint`. El panel también rechaza un nombre que ya usa una base de datos o un usuario de base de datos en MariaDB.
- Las contraseñas tienen de 12 a 64 caracteres, formados por letras, dígitos y `! # % * + , . : = ? @ ^ _ ~ -`. Una contraseña generada tiene 24 caracteres.

Las bases de datos nuevas usan `utf8mb4` y `utf8mb4_unicode_ci`.

### Usuarios y privilegios {#users-and-privileges}

Cada base de datos tiene un usuario, `name@localhost`, con todos los privilegios sobre esa base de datos y ninguno sobre las demás. Con el acceso remoto activado, ese usuario también recibe una copia para cada dirección permitida (consulta [Permitir el acceso remoto](#allow-remote-access)). Aparte del usuario temporal que ejecuta las importaciones (consulta [Importar](#import)), el panel no crea usuarios adicionales ni cambia privilegios. Si lo necesitas, usa el comando `mariadb` (`mysql` en MySQL) en el servidor como root.

La lista muestra las bases de datos que creó el panel. Una base de datos que se quitó fuera del panel se marca como **No se encontró en MariaDB** (o **No se encontró en MySQL**).

## Mostrar los datos de acceso de una base de datos {#show-a-database-s-login}

Selecciona **Mostrar acceso** en la fila de la base de datos. La tarjeta de datos de conexión muestra el nombre, el usuario, la contraseña y el host, cada uno con un botón para copiar. Si Adminer o phpMyAdmin están instalados, la tarjeta también tiene **Abrir en Adminer** o **Abrir en phpMyAdmin**.

## Cambiar la contraseña de una base de datos {#change-a-database-password}

1. Abre el menú de la base de datos y selecciona **Contraseña nueva**.
2. Deja **Generar una contraseña aleatoria segura**, o elige **Usar mi propia contraseña** y escríbela. Tu propia contraseña sigue las mismas reglas que al [crear una base de datos](#create-a-database).
3. Selecciona **Contraseña nueva**.

El panel define la contraseña nueva y la muestra en la tarjeta de datos de conexión. Si la base de datos está vinculada a un sitio web WordPress que creó el panel, el panel también escribe la contraseña nueva en el `wp-config.php` de ese sitio. Las demás aplicaciones dejan de conectarse hasta que actualices sus ajustes.

Si la base de datos permite el acceso remoto, los usuarios remotos también reciben la contraseña nueva.

## Exportar e importar {#export-and-import}

### Exportar {#export}

Abre el menú de la base de datos y selecciona **Exportar (.sql)**. Tu navegador descarga un archivo SQL simple con un nombre como `shop_20261126-143000.sql`.

El volcado se hace con `mariadb-dump --single-transaction` (`mysqldump` en MySQL, también con `--set-gtid-purged=OFF`), así que el sitio web sigue funcionando durante la exportación. Incluye las rutinas almacenadas y los triggers.

### Importar {#import}

1. Abre el menú de la base de datos y selecciona **Importar**.
2. Elige un archivo en **Archivo SQL (.sql o .sql.gz)**. Un archivo `.sql.gz` se descomprime sobre la marcha.
3. Opcional: marca **Vaciar primero la base de datos (eliminar todas sus tablas)**. La base de datos se elimina y se vuelve a crear antes de la importación, y su usuario conserva el acceso.
4. Selecciona **Importar**.

Una importación puede ocupar hasta 4 GB.

Un volcado hecho por el otro servidor, como una copia de seguridad de antes de un [cambio](#switch-between-mariadb-and-mysql), también se importa: el panel reescribe, mientras el archivo va entrando, los nombres de intercalación que este servidor no conoce (las `utf8mb4_0900_*` de MySQL 8 en MariaDB, las `uca1400` de MariaDB 11 en MySQL). En MySQL también quita `NO_AUTO_CREATE_USER`, que MariaDB escribe en el modo SQL de cada trigger y rutina y que MySQL 8 rechaza. Nada más cambia en el archivo; un volcado que usa otra base de datos se sigue rechazando. Restaurar una copia de seguridad de base de datos funciona de la misma forma.

La importación se ejecuta como un usuario temporal de MariaDB, `nefo-import-` seguido del nombre de la base de datos, con todos los privilegios sobre esta base de datos y ninguno sobre las demás. También puede asignar el definer de los triggers, las vistas y las rutinas almacenadas, así que los volcados que los contienen, como las propias exportaciones y copias de seguridad del panel, se importan como antes. El panel elimina este usuario cuando termina la importación, también cuando falla. Solo lo conserva cuando el archivo creó triggers, vistas, rutinas o eventos sin una cláusula `DEFINER`, que se ejecutan entonces como este usuario: bloqueado, para que nadie pueda iniciar sesión con él, y con privilegios solo sobre esta base de datos. Eliminar la base de datos también lo elimina.

En MySQL, los nombres de usuario se cortan a los 32 caracteres, así que con un nombre de base de datos largo el nombre del usuario se acorta y se le agrega un hash corto. Allí, la asignación de definers se permite con `SET_ANY_DEFINER` y `ALLOW_NONEXISTENT_DEFINER` en lugar del `SET USER` de MariaDB; sin `SYSTEM_USER`, el usuario sigue sin poder hacer que un objeto se ejecute como root. La primera línea de los volcados de las versiones más recientes de MariaDB, `/*M!999999\- enable the sandbox mode */`, que el cliente de MySQL rechaza, se omite en MySQL.

::: warning
Si el archivo usa otra base de datos, por ejemplo con `USE other_db`, `CREATE DATABASE other_db` o `INSERT INTO other_db.posts`, MariaDB rechaza esa sentencia y la importación se detiene con un mensaje que empieza por "El archivo SQL usa otra base de datos, other_db." Lo que el archivo hizo antes de esa línea se queda en la base de datos. Quita del archivo las líneas `CREATE DATABASE` y `USE`, o vuelve a exportar la base de datos sin `--databases`, y luego impórtala de nuevo con **Vaciar primero la base de datos (eliminar todas sus tablas)** marcada. Las líneas `CREATE DATABASE` y `USE` que nombran esta misma base de datos no son un problema.
:::

## Permitir el acceso remoto {#allow-remote-access}

De forma predeterminada, MariaDB solo acepta conexiones desde el propio servidor. Puedes permitir que direcciones elegidas se conecten a una base de datos, por ejemplo una herramienta de informes en la computadora de tu oficina.

Antes de empezar, instala y activa el firewall ufw en la página **Firewall y SSH**. El panel rechaza el acceso remoto mientras ufw no esté instalado.

1. Abre el menú de la base de datos y selecciona **Acceso remoto**.
2. En **Direcciones permitidas**, escribe una dirección. Selecciona **Agregar una dirección** para agregar más.
3. Selecciona **Guardar**.

Conéctate con el usuario y la contraseña de la base de datos a la dirección IP pública del servidor, puerto 3306.

### Direcciones permitidas {#allowed-addresses}

- Direcciones IPv4 o IPv6 individuales, como `203.0.113.10` o `2001:db8::1`.
- Rangos IPv4 en notación CIDR, como `198.51.100.0/24`, no más amplios que `/16`.
- No es posible usar rangos IPv6, porque MariaDB no puede compararlos. Escribe direcciones IPv6 individuales.
- Como máximo 10 direcciones por base de datos.
- Se rechazan las direcciones de loopback, `0.0.0.0` y las de multidifusión.

### Qué cambia el panel {#what-the-panel-changes}

| Dónde | Cambio |
|---|---|
| Usuarios de MariaDB | Para cada dirección, un usuario `name@address` con la misma contraseña y los mismos privilegios que el usuario local |
| Firewall (ufw) | Para cada dirección, una regla que permite el puerto 3306/tcp solo desde esa dirección, con el comentario `MariaDB name` |
| Ajustes de MariaDB | `/etc/mysql/mariadb.conf.d/99-panel-remote.cnf` hace que MariaDB escuche en todas las interfaces (`0.0.0.0`, o `::` cuando se permite una dirección IPv6). El archivo solo existe mientras al menos una base de datos permita el acceso remoto |

MySQL funciona de la misma forma: el archivo es `/etc/mysql/mysql.conf.d/99-panel-remote.cnf`, el comentario de las reglas es `MySQL name`, y un usuario remoto se elimina y se vuelve a crear cuando cambia, porque MySQL no tiene `CREATE OR REPLACE USER`. En ese caso, los mensajes de abajo dicen MySQL.

MariaDB se reinicia la primera vez que se activa el acceso remoto, y de nuevo cuando se desactiva para la última base de datos. Los sitios web pierden su base de datos durante unos segundos. Si en ese momento hay otra tarea en curso, el cuadro de diálogo dice "MariaDB tiene que reiniciarse antes de aceptar conexiones remotas." Selecciona **Reiniciar MariaDB** cuando la tarea haya terminado.

Si MariaDB no se inicia con el archivo de acceso remoto, el panel quita el archivo e inicia MariaDB sin él, para que tus sitios web sigan funcionando.

::: warning
- Si el firewall está desactivado, el puerto 3306 está abierto para todos. MariaDB sigue aceptando solo las direcciones de la lista.
- Las conexiones no van cifradas salvo que el cliente pida TLS. Para datos sensibles, usa un túnel SSH o una VPN.
- Incluye solo direcciones que controles y que no cambien.
:::

Para desactivar el acceso remoto, vuelve a abrir **Acceso remoto** y selecciona **Desactivar**. Eliminar una base de datos también quita sus usuarios remotos y sus reglas de firewall.

## Encontrar consultas lentas {#find-slow-queries}

MariaDB puede anotar en un registro cada consulta que tarda más de un umbral. Esto te ayuda a encontrar lo que hace lento un sitio web.

1. Ve a **Bases de datos** → **Consultas lentas**. La pestaña aparece cuando MariaDB está instalado.
2. Define **Más lentas que** en un número de segundos, de 0.1 a 60.
3. Activa el interruptor **Registrar consultas lentas** y selecciona **Aplicar**.

MariaDB debe estar en ejecución para cambiar estos ajustes. El cambio se aplica de inmediato, a las conexiones abiertas después de él, y se mantiene tras un reinicio: el panel lo guarda en `/etc/mysql/mariadb.conf.d/99-panel-slowlog.cnf`, y el registro se escribe en `/var/log/mysql/mariadb-slow.log`. En MySQL, los ajustes están en `/etc/mysql/mysql.conf.d/99-panel-slowlog.cnf` y el registro es `/var/log/mysql/mysql-slow.log`.

El panel rota el registro para que no pueda llenar el disco, en MariaDB y en MySQL. Una vez por hora lo revisa y, cuando el registro pasa de 50 MB o su entrada más antigua tiene 7 días, el panel le cambia el nombre, hace que el servidor abra uno nuevo (`FLUSH SLOW LOGS`, sin reiniciar) y comprime el antiguo a su lado, como `mariadb-slow.log-YYYYMMDD-HHMMSS.gz` (`mysql-slow.log-…` en MySQL). Conserva los 4 más recientes y elimina los más antiguos. La pestaña muestra cuántos conserva en ese momento. La tabla solo lee el registro actual.

La tabla muestra las 200 entradas más recientes, de la más nueva a la más antigua, leídas de los últimos 4 MB del registro:

| Columna | Significado |
|---|---|
| **Hora** | Cuándo se ejecutó la consulta |
| **Usuario y host** | Quién la ejecutó, como `shop@localhost` |
| **Base de datos** | La base de datos que usó |
| **Duración** | Cuánto tardó, en segundos |
| **Filas examinadas** | Cuántas filas leyó MariaDB |
| **Consulta** | La consulta. Selecciónala para verla completa, hasta 2000 caracteres. Las consultas más largas se marcan como **Acortada** |

Selecciona **Actualizar** para volver a leer el registro, y **Vaciar registro** para vaciarlo. **Vaciar registro** solo vacía el registro actual; los registros antiguos comprimidos se conservan hasta que la rotación los elimina.

::: tip
El registro agrega un poco de trabajo al servidor. Desactiva el interruptor y selecciona **Aplicar** cuando termines.
:::

## Adminer {#adminer}

Adminer te permite explorar y editar tus bases de datos en el navegador, como phpMyAdmin.

### Instalar Adminer {#install-adminer}

1. Instala primero un servidor web, en la página **Software**. Adminer se ejecuta en él con PHP.
2. Ve a **Software** → **Adminer (interfaz de bases de datos)** y selecciona **Instalar**.

El panel descarga Adminer 6.3.1 (la compilación con todos los controladores de Adminer) y lo comprueba con una suma de verificación SHA-256 fijada antes de instalarlo. Abre bases de datos MariaDB o MySQL, y bases de datos PostgreSQL cuando PostgreSQL está instalado (consulta [PostgreSQL en Adminer](./postgresql#postgresql-in-adminer)). Su controlador de SQLite rechaza todos los inicios de sesión, porque SQLite no tiene contraseña.

### Abrir Adminer {#open-adminer}

1. Selecciona **Abrir en Adminer** en la parte superior de la página **Bases de datos**, o en la tarjeta de datos de conexión de una base de datos. **Software** → **Abrir Adminer** hace lo mismo.
2. Adminer se abre en una pestaña nueva, en la dirección de tu panel seguida de `adminer/`, por ejemplo `https://203.0.113.10:28443/k3x9p2qa7m1z/adminer/`.
3. Inicia sesión en Adminer con el usuario y la contraseña de la base de datos que muestra **Mostrar acceso**.

### Cómo se protege Adminer {#how-adminer-is-protected}

Solo se puede llegar a Adminer a través del inicio de sesión del panel:

- Se ejecuta en un sitio interno de tu servidor web que solo escucha en 127.0.0.1, en un puerto aleatorio entre 40000 y 59999.
- Ese sitio interno solo responde a las solicitudes que llevan un token secreto. Las demás solicitudes reciben `404`.
- El panel solo agrega el token a las solicitudes de una sesión del panel iniciada. Sin una sesión del panel, `adminer/` responde `401` y la solicitud nunca llega a Adminer.
- La cookie de sesión propia del panel nunca se pasa a Adminer.

Así, no se puede llegar a Adminer desde internet, y otros programas del servidor no pueden usarlo sin el token.

Adminer puede importar archivos de hasta 256 MB, y una solicitud puede ejecutarse durante un máximo de 600 segundos. Para volcados más grandes, usa **Importar** en la página **Bases de datos**.

Desinstalar Adminer quita sus archivos y su sitio interno. Tus bases de datos no se tocan. Desinstalar el servidor web también quita Adminer.

## phpMyAdmin {#phpmyadmin}

phpMyAdmin es la otra interfaz de bases de datos, para MariaDB y MySQL. Instala la que prefieras, o ambas: no comparten archivos.

### Instalar phpMyAdmin {#install-phpmyadmin}

1. Instala primero un servidor web, en la página **Software**. phpMyAdmin se ejecuta en él con PHP.
2. Ve a **Software** → **phpMyAdmin (interfaz de bases de datos)** y selecciona **Instalar**.

El panel descarga phpMyAdmin 5.2.3 (todos los idiomas) desde `files.phpmyadmin.net` y lo comprueba con una suma de verificación SHA-256 fijada antes de descomprimirlo en `/usr/share/nefo-phpmyadmin`. Deja fuera el script de configuración, los ejemplos y la documentación local (los enlaces de ayuda abren en cambio docs.phpmyadmin.net). phpMyAdmin guarda sus archivos temporales en `/var/lib/nefo-phpmyadmin/tmp`.

El panel escribe el `config.inc.php` de phpMyAdmin, con una clave aleatoria de cifrado de cookies:

- Inicias sesión con el usuario y la contraseña de una base de datos, solo en el servidor de bases de datos de este servidor. No puedes elegir otro servidor, iniciar sesión sin contraseña ni iniciar sesión como `root`.
- phpMyAdmin no busca versiones nuevas ni envía informes de errores.
- Su formulario de cambio de contraseña está oculto. Cambia la contraseña de una base de datos en la página **Bases de datos**, que también actualiza el `wp-config.php` de un sitio web WordPress.

### Abrir phpMyAdmin {#open-phpmyadmin}

1. Selecciona **Abrir en phpMyAdmin** en la parte superior de la página **Bases de datos**, en el menú de una base de datos o en la tarjeta de datos de conexión. **Software** → **Abrir phpMyAdmin** hace lo mismo.
2. phpMyAdmin se abre en una pestaña nueva, en la dirección de tu panel seguida de `phpmyadmin/`, por ejemplo `https://203.0.113.10:28443/k3x9p2qa7m1z/phpmyadmin/`.
3. Inicia sesión con el usuario y la contraseña de la base de datos que muestra **Mostrar acceso**.

phpMyAdmin está protegido como Adminer (consulta [Cómo se protege Adminer](#how-adminer-is-protected)), en su propio sitio interno con su propio token secreto. Sin una sesión del panel, `phpmyadmin/` responde `401`, y una clave de API tampoco puede llegar a él. Solo los tres puntos de entrada de phpMyAdmin se ejecutan como PHP, y solo sus scripts, estilos e imágenes se sirven como archivos; todo lo demás responde `404`, incluido el script de configuración. phpMyAdmin envía su propia Content-Security-Policy y comprueba sus propios tokens de formulario.

Como Adminer, phpMyAdmin puede importar archivos de hasta 256 MB, y una solicitud puede ejecutarse durante un máximo de 600 segundos. Desinstalar phpMyAdmin quita sus archivos, su sitio interno y sus ajustes. Tus bases de datos no se tocan. Desinstalar el servidor web también quita phpMyAdmin.

## Explorador de claves de Redis {#redis-key-browser}

Cuando Redis está instalado, la página **Bases de datos** tiene una pestaña **Redis**. Para instalar Redis, ve a **Software** → **Redis** → **Instalar**. Consulta [Software](./software#redis) para ver cómo se configura.

La pestaña muestra la versión de Redis, cuánto tiempo lleva en ejecución, **Memoria usada** (con el límite de memoria) y el número de **Claves**. Si dice **Redis está instalado pero no está en ejecución.**, inicia Redis en la página **Servicios**.

### Buscar claves {#find-keys}

1. Elige una base de datos de 0 a 15. La lista muestra cuántas claves tiene cada una.
2. Escribe un patrón. `*` coincide con cualquier cosa y `?` con un carácter, por ejemplo `user:*`.
3. Selecciona **Buscar**.

La búsqueda recorre las claves en pasos de unas 100. Si hay más, selecciona **Seguir buscando**.

### Ver una clave {#view-a-key}

Selecciona una clave para ver su **Tipo**, **Caduca en** y **Tamaño**, y su valor:

| Tipo | Qué muestra el panel |
|---|---|
| string | Los primeros 64 KB del valor |
| list, hash, set, sorted set | Los primeros 200 elementos. Cada elemento se corta a los 4 KB |
| stream y otros tipos | Solo el tipo y la caducidad |

Los caracteres que no se pueden mostrar en los valores binarios se reemplazan.

### Eliminar claves {#delete-keys}

- Para eliminar una clave, selecciónala y luego **Eliminar clave**.
- Para eliminar todas las claves de la base de datos elegida, selecciona **Vaciar la base de datos 0** (el botón muestra el número de la base de datos), escribe el número para confirmar y selecciona **Eliminar todas las claves**.

Los sitios web que usan Redis como caché la reconstruyen. Los datos guardados solo en Redis se pierden.

El explorador no edita valores ni tiempos de caducidad. Para eso, usa `redis-cli` en el servidor.

Si Redis tiene una contraseña (`requirepass` en `/etc/redis/redis.conf`), el panel la lee de ese archivo e inicia sesión con ella.

## Eliminar una base de datos {#delete-a-database}

1. Abre el menú de la base de datos y selecciona **Eliminar**.
2. Escribe el nombre de la base de datos para confirmar y selecciona **Eliminar**.

::: danger
Esto elimina la base de datos, todas sus tablas y su usuario. No se puede deshacer. Con ella se van sus usuarios remotos, sus reglas de firewall y sus programaciones de copias de seguridad automáticas. Las copias de seguridad ya hechas se conservan en la página **Copias de seguridad**.
:::

Cuando eliminas un sitio web, sus bases de datos se conservan salvo que marques **Eliminar también sus bases de datos**: se quedan aquí, sin vínculo con ningún sitio web. Restaurar la copia de seguridad del sitio web las vuelve a vincular.

## Desinstalar MariaDB {#uninstall-mariadb}

Desinstala MariaDB en la página **Software**. El panel se niega mientras MariaDB contenga alguna base de datos que no sea una de sus propias bases de datos del sistema, incluidas las bases de datos creadas fuera del panel. Consulta [Software](./software) para saber qué conserva una desinstalación.

Una MariaDB del repositorio de MariaDB se va con su archivo de repositorio y su clave.

MySQL se desinstala de la misma forma, en su tarjeta, con la misma comprobación. El panel también quita el repositorio de Oracle y su clave. El directorio de datos `/var/lib/mysql` se conserva salvo que marques la casilla y escribas `mysql`.

Para reemplazar uno por otro conservando tus bases de datos, mejor [cámbialo](#switch-between-mariadb-and-mysql).

## PostgreSQL {#postgresql}

PostgreSQL funciona junto a MariaDB o MySQL, en su propia pestaña **PostgreSQL** de esta página: la versión de tu distribución o de la 15 a la 19 del proyecto PostgreSQL, con bases de datos y propietarios, importación y exportación, acceso remoto por TLS, copias de seguridad y actualizaciones de versión mayor. Consulta [PostgreSQL](./postgresql).

## Memcached {#memcached}

Cuando Memcached está instalado (**Software** → **Memcached**), la página **Bases de datos** tiene una pestaña **Memcached**. Muestra la versión, cuánto tiempo lleva en ejecución, **Memoria usada** con el límite, el número de **Elementos**, la tasa de aciertos y las conexiones abiertas. También muestra lo que informa el propio Memcached: que solo escucha en 127.0.0.1 y que UDP está desactivado.

**Reiniciar** vacía la caché. Consulta [Software](./software#memcached) para ver cómo se configura.
