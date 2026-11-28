---
title: Software
description: Instala y desinstala el servidor web, PHP 5.6 a 8.6, el ionCube Loader, MariaDB o MySQL, PostgreSQL, Redis, Memcached, Adminer, phpMyAdmin, Node.js, Python, Java, Docker, el firewall web y ClamAV, y administra las extensiones de PHP y los ajustes de php.ini.
---

# Software

Nefo no instala nada hasta que tú lo eliges. La página **Software** instala y desinstala cada componente opcional, con la memoria aproximada que usa: el servidor web, PHP 5.6 a 8.6 y el ionCube Loader, MariaDB o MySQL, PostgreSQL, Redis, Memcached, Node.js, Python, Java, Docker, el firewall web y ClamAV. Se niega a quitar lo que algo todavía usa.

Esta página trata de lo que puedes instalar, cuándo se rechaza una desinstalación, qué conserva una desinstalación, y los ajustes de PHP, Node.js, Python, Java y Redis. Un servidor pequeño se mantiene ligero cuando solo lleva lo que usan sus sitios web.

Las instalaciones y desinstalaciones se ejecutan como tareas con un registro en vivo. El panel ejecuta una tarea a la vez. El software viene de los paquetes de tu distribución, salvo que esta página diga lo contrario.

## Qué puedes instalar {#what-you-can-install}

Cada tarjeta muestra aproximadamente cuánta memoria usa el elemento mientras se ejecuta. La parte superior de la página suma los elementos instalados.

| Sección | Elemento | Memoria, aproximadamente |
|---|---|---|
| **Nginx, Apache u OpenLiteSpeed** | Nginx | unos 15 MB |
| | Apache | unos 40 MB |
| | OpenLiteSpeed | unos 26 MB |
| **Versiones de PHP** | PHP 7.4, 8.0, 8.1, 8.2, 8.3, 8.4, 8.5 y 8.6 (y las más recientes en cuanto el repositorio de PHP las tenga), y en **Versiones antiguas** 5.6, 7.0, 7.1, 7.2 y 7.3 | unos 25 MB cada una |
| | ionCube Loader | ninguna propia: se carga en los workers de PHP |
| **Bases de datos y caché** | MariaDB (la de tu distribución, o 11.4, 11.8 o 12.3 LTS del repositorio de MariaDB), o en su lugar MySQL 8.4 LTS | MariaDB: su buffer pool más unos 100 MB (consulta [Bases de datos](./databases#how-mariadb-is-tuned)). MySQL: su buffer pool más unos 150 MB, o más unos 300 MB a partir de unos 3.5 GB de RAM (consulta [Bases de datos](./databases#how-mysql-is-tuned)) |
| | PostgreSQL | unos 30 a 40 MB en reposo, y crece hasta `shared_buffers` más unos 40 MB (consulta [PostgreSQL](./postgresql#how-postgresql-is-tuned)) |
| | Redis | hasta 64, 128 o 256 MB, según la RAM del servidor |
| | Memcached | hasta 64, 128 o 256 MB, según la RAM del servidor; unos pocos MB mientras está vacío |
| | Adminer (interfaz de bases de datos) | nada en reposo |
| | phpMyAdmin (interfaz de bases de datos) | nada en reposo |
| **Entornos de ejecución** | Node.js | nada en reposo |
| | Aplicaciones Python | nada en reposo |
| | Java (OpenJDK) | nada en reposo; el programa Java de un sitio web de aplicación usa lo que necesite su heap |
| **Contenedores** | Docker | unos 90 MB, más tus contenedores |
| **Seguridad** | Firewall web (ModSecurity) | unos 40 MB |
| | Antivirus ClamAV | unos 15 MB, y alrededor de 1.4 GB de memoria libre mientras analiza |
| **Herramientas** | Composer | nada, solo se ejecuta cuando se lo llama |
| | WP-CLI | nada, solo se ejecuta cuando se lo llama |

## Instalar un elemento {#install-an-item}

1. Ve a **Software**.
2. Selecciona **Instalar** en la tarjeta del elemento.
3. Sigue el registro. Cuando la tarea termina, la tarjeta muestra **Instalado** y su versión.

Algunas tarjetas ofrecen entonces un enlace: **Abrir Bases de datos**, **Abrir PostgreSQL**, **Abrir Redis**, **Abrir Memcached**, **Abrir Adminer**, **Abrir phpMyAdmin** o **Abrir Docker**.

## Desinstalar un elemento {#uninstall-an-item}

1. Selecciona **Desinstalar** en la tarjeta del elemento. Para una versión de PHP, está en el menú de la versión.
2. Lee lo que el cuadro de diálogo dice que hace la desinstalación.
3. En MariaDB, MySQL, PostgreSQL y Docker, puedes marcar la casilla para eliminar también sus datos. En ese caso debes escribir el nombre del elemento para confirmar.
4. Selecciona **Desinstalar**.

### Cuándo se rechaza una desinstalación {#when-an-uninstall-is-refused}

El panel nunca quita software que algo todavía usa. Cuando un elemento no se puede desinstalar, su tarjeta muestra el motivo en lugar de un botón que funcione.

| Elemento | Se rechaza mientras |
|---|---|
| Servidor web | Exista algún sitio web, en el panel o como archivo de configuración en la carpeta de sitios del servidor web |
| Una versión de PHP | Un sitio web la use, o sea la única versión de PHP instalada (la configuración del servidor web y Adminer necesitan una). Una versión con soporte (7.4 o más reciente) también se queda mientras sea la única junto a [versiones antiguas](#older-php-versions) |
| Composer | Un script de despliegue con Git ejecute `composer` |
| WP-CLI | Existan sitios web WordPress: las herramientas de WordPress lo ejecutan para ellos |
| MariaDB o MySQL | Contenga alguna base de datos que no sea una de sus propias bases de datos del sistema. El panel se lo pregunta al propio servidor, así que también cuentan las bases de datos creadas a mano o con otra herramienta. Si está detenido y su carpeta de datos se movió, el panel no puede comprobarlo y lo rechaza |
| PostgreSQL | PostgreSQL contenga alguna base de datos distinta de `postgres`, `template0` y `template1`, según el propio PostgreSQL, o esté detenido, porque entonces el panel no puede comprobarlo |
| Node.js | Haya sitios web de aplicaciones que usen el Node.js predeterminado del servidor, o el `node` de `/usr/local/bin` no lo haya instalado el panel |
| Una rama en **Versiones de Node.js** | Sea la predeterminada del servidor, o la haya elegido un sitio web de aplicación |
| Aplicaciones Python | Haya sitios web de aplicaciones que ejecuten Python |
| Java (OpenJDK) | Un sitio web de aplicación ejecute Java. Una sola versión de Java se rechaza mientras un sitio web de aplicación use esa versión |
| Docker | Haya sitios web que ejecuten aplicaciones Docker, existan proyectos de Compose, o Docker tenga contenedores, en ejecución o detenidos |

Redis, Memcached, Adminer, phpMyAdmin, el ionCube Loader, el firewall web y ClamAV siempre se pueden desinstalar.

### Qué conserva una desinstalación {#what-an-uninstall-keeps}

| Elemento | Se conserva | Se quita |
|---|---|---|
| Servidor web | Las carpetas de los sitios web en `/var/www`, los certificados, PHP | El servidor web, su configuración del panel, Adminer y las reglas de firewall de los puertos 80 y 443 |
| Versión de PHP | | La versión, sus extensiones y las bibliotecas que solo ellas necesitaban |
| MariaDB | La carpeta de datos `/var/lib/mysql`, salvo que marques **Eliminar también el directorio de datos (/var/lib/mysql)** y escribas `mariadb`. Si vuelves a instalar MariaDB, la usa | Los paquetes del servidor y del cliente, los ajustes de MariaDB del panel, y el repositorio de MariaDB con su clave si MariaDB vino de ahí |
| MySQL | La carpeta de datos `/var/lib/mysql`, salvo que marques **Eliminar también el directorio de datos (/var/lib/mysql)** y escribas `mysql`. Si vuelves a instalar MySQL, la usa | Los paquetes del servidor y del cliente, los ajustes de MySQL del panel, y el repositorio de Oracle con su clave |
| PostgreSQL | Los datos de `/var/lib/postgresql` y los ajustes del clúster de `/etc/postgresql`, salvo que marques **Eliminar también los datos y los ajustes (/var/lib/postgresql y /etc/postgresql)** y escribas `postgresql`. Los paquetes se quitan sin purgar, y si vuelves a instalar PostgreSQL se inicia el mismo clúster | Los paquetes de PostgreSQL, y el repositorio apt.postgresql.org con su clave (`/etc/apt/sources.list.d/panel-pgdg.list` y `/etc/apt/keyrings/panel-pgdg.gpg`) si los agregó el panel |
| Redis | | Redis, su extensión de PHP y todo lo guardado en Redis. PHP-FPM se reinicia |
| Memcached | | Memcached, su extensión de PHP y todo lo que tenía en caché. PHP-FPM se reinicia |
| Adminer | Tus bases de datos | Los archivos de Adminer y su sitio interno |
| phpMyAdmin | Tus bases de datos | Los archivos de phpMyAdmin, sus archivos temporales, su sitio interno y sus ajustes |
| Node.js | Las demás versiones de Node.js. También la carpeta de la predeterminada, si un sitio web de aplicación eligió esa versión | `node`, `npm` y `npx` en `/usr/local/bin`, y la versión a la que apuntan |
| Aplicaciones Python | El propio `python3`, que necesita el sistema, y los entornos virtuales de las carpetas de aplicaciones antiguas | Los paquetes venv y pip |
| Java (OpenJDK) | | Todas las versiones de Java instaladas, con los paquetes que solo ellas necesitaban |
| ionCube Loader | | El loader en todas las versiones de PHP y `/usr/local/lib/ioncube`. PHP-FPM se recarga, y los archivos PHP codificados dejan de funcionar |
| Docker | Las imágenes, los volúmenes y la caché de compilación en `/var/lib/docker`, salvo que marques **Eliminar también todas las imágenes, los volúmenes y la caché de compilación (/var/lib/docker)** y escribas `docker`. Si vuelves a instalar Docker, los encuentra | Docker y su plugin Compose |
| Firewall web | El registro del firewall | ModSecurity y las reglas de OWASP. El servidor web se reinicia |
| ClamAV | | ClamAV y sus bases de datos de virus |
| Composer | Los sitios web Laravel y Drupal, que solo lo necesitaban para instalarse | `/usr/local/bin/composer` y el paquete `composer` de la distribución |
| WP-CLI | | `/usr/local/lib/panel/wp-cli.phar`. Las herramientas de WordPress lo vuelven a instalar cuando lo necesitan |

::: danger
Eliminar los datos de MariaDB, MySQL, PostgreSQL o Docker no se puede deshacer. Asegúrate de tener una copia de seguridad de todo lo que todavía necesites.
:::

## El menú muestra lo que está instalado {#the-menu-shows-what-is-installed}

El menú de la izquierda oculta las páginas del software que no está instalado:

- **Docker** aparece cuando Docker está instalado.
- **WordPress** aparece cuando existe un sitio web WordPress.
- En la página **Bases de datos**, las pestañas **PostgreSQL**, **Redis** y **Memcached** aparecen cuando ese software está instalado, y **Consultas lentas** cuando MariaDB o MySQL está instalado.

**Sitios web**, **Bases de datos** y **Software** siempre se muestran, porque ofrecen las instalaciones. Una página que tienes abierta sigue visible.

## Servidor web {#web-server}

El panel ejecuta un solo servidor web a la vez y configura todos los sitios web para él:

- Nginx es el más ligero. PHP se ejecuta en PHP-FPM.
- Apache lee los archivos `.htaccess` que traen algunas aplicaciones PHP, y usa un poco más de memoria.
- OpenLiteSpeed lee las reglas de reescritura de los archivos `.htaccess` y trae LiteSpeed Cache integrado para WordPress. Viene del repositorio de paquetes propio de LiteSpeed.

Instalar un servidor web también instala la versión de PHP de tu distribución con PHP-FPM. Si el firewall ufw está activado, el panel abre los puertos 80 y 443, y 443/udp para HTTP/3 con OpenLiteSpeed.

El panel se niega a instalar un segundo servidor web mientras haya otro instalado, porque ambos necesitarían los puertos 80 y 443.

Para cambiar a otro servidor web:

1. Primero elimina todos los sitios web. Haz copias de seguridad si los vas a necesitar más adelante.
2. Selecciona **Desinstalar** en el servidor web actual y escribe su nombre (`nginx`, `apache` u `openlitespeed`) para confirmar.
3. Instala el otro servidor web.

Consulta [Sitios web](./websites) para configurar los sitios.

## Versiones de PHP {#php-versions}

Puedes instalar varias versiones de PHP una al lado de la otra y elegir una por sitio web en **Sitios web** → (sitio) → **PHP**.

- La versión que trae tu distribución viene de sus paquetes. Las demás versiones vienen del repositorio de Ondřej Surý `packages.sury.org/php`, tanto en Ubuntu como en Debian. El panel pregunta antes de agregar ese repositorio al servidor, y solo acepta su clave de firma con la huella `15058500 A0235D97 F5D10063 B188E2B6 95BD4743`.
- Cada versión se instala con estas extensiones: fpm, cli, mysql, curl, gd, mbstring, xml, zip, intl, bcmath y opcache. Desde PHP 8.5, OPcache forma parte del propio PHP, así que 8.5 y las versiones más recientes no tienen un paquete opcache aparte. Cuando Redis o Memcached está instalado, también se agrega la extensión redis o memcached.
- Antes de PHP 8.0, json es un paquete aparte y también se agrega. PHP 5.6 recibe además mcrypt, cuando el repositorio lo tiene.
- El panel cambia el pool de PHP-FPM predeterminado de la versión a workers bajo demanda, así que una versión que ningún sitio web usa casi no consume memoria.

Las versiones de la lista siguen al repositorio de PHP. Cuando `packages.sury.org/php` está en el servidor (se agrega con la primera versión de PHP que no es la de tu distribución), la página ofrece las versiones que el repositorio tiene para tu sistema, leídas de las listas de paquetes de apt. Una versión nueva de PHP, como PHP 8.7, aparece sola en cuanto el repositorio la publica y se actualizan las listas de paquetes (cada instalación ejecuta `apt-get update`, y también lo hace la tarea diaria de apt de tu sistema). Una versión que el repositorio no tiene no se ofrece. Antes de que se agregue el repositorio, la página ofrece las versiones que Nefo conoce, de la 7.4 a la 8.6. Las versiones instaladas, la versión de tu distribución y las **Versiones antiguas** siempre aparecen en la lista.

Selecciona **Administrar** en una versión instalada para abrir sus **Extensiones** y **Ajustes**.

### Versiones sin actualizaciones de seguridad {#end-of-life}

Cada versión de PHP recibe actualizaciones de seguridad de PHP hasta una fecha fija (consulta [php.net/supported-versions.php](https://www.php.net/supported-versions.php)): 8.1 hasta el 31 de diciembre de 2025, 8.2 hasta el 31 de diciembre de 2026, 8.3 hasta el 31 de diciembre de 2027, 8.4 hasta el 31 de diciembre de 2028, 8.5 hasta el 31 de diciembre de 2029 y 8.6 hasta el 31 de diciembre de 2030. 7.4 y 8.0 ya no reciben ninguna. Pasada esa fecha, una versión de `packages.sury.org` muestra la insignia **Fin de vida útil: sin actualizaciones de seguridad** y una línea con la fecha, en la página Software, en su ventana **Administrar** y en los menús de versión de PHP de los sitios web. El panel compara la fecha con la de hoy, así que la insignia aparece sola.

La versión de PHP propia de tu distribución (8.1 en Ubuntu 22.04, 8.3 en 24.04, 8.5 en 26.04, 8.2 en Debian 12, 8.4 en Debian 13) nunca muestra la insignia: el equipo de seguridad de la distribución la sigue parcheando mientras esa versión de la distribución tenga soporte.

Cuando puedas, traslada los sitios web fuera de una versión con la insignia. Todavía puedes instalarla sin una confirmación adicional; solo las [versiones antiguas](#older-php-versions) la piden.

### Versiones antiguas de PHP {#older-php-versions}

PHP 5.6, 7.0, 7.1, 7.2 y 7.3 están en **Versiones antiguas**, plegadas debajo de las demás versiones. Están ahí para los sitios web trasladados desde otro servidor (cPanel, aaPanel…) que todavía no funcionan con un PHP más reciente.

::: danger Fin de vida útil: sin actualizaciones de seguridad
PHP dejó de publicar actualizaciones de seguridad para estas versiones hace años: 5.6 en 2018, 7.3 en 2021 (consulta [php.net/eol.php](https://www.php.net/eol.php)). Sus vulnerabilidades conocidas nunca se corregirán, así que un sitio web que funciona con una de ellas es más fácil de atacar. Úsalas solo para mantener en funcionamiento un sitio web antiguo mientras lo actualizas, y luego trasládalo a una versión con soporte. No las uses para sitios web nuevos.
:::

- Cada una muestra la insignia **Fin de vida útil: sin actualizaciones de seguridad**, en la página Software y en los menús de versión de PHP de los sitios web.
- Al instalar una, se te pide primero que confirmes ese riesgo.
- Primero debe estar instalada una versión con soporte (7.4 o más reciente), y la última versión con soporte no se puede desinstalar mientras haya versiones antiguas instaladas. Así, una versión antigua nunca es el único PHP del servidor: la configuración del servidor web, Adminer, Composer y las herramientas del panel conservan una con soporte.
- La tienda de aplicaciones mantiene sus propias versiones mínimas: WordPress necesita PHP 7.4, y las demás aplicaciones más. Las herramientas de WordPress siguen administrando un sitio web WordPress con una versión antigua, pero allí no actualizan el propio WordPress, porque el WordPress actual necesita PHP 7.4. Siguen actualizando los plugins y los temas; WP-CLI omite las actualizaciones que necesitan un PHP más reciente.
- El pool de PHP-FPM del sitio web y los ajustes de php.ini son los mismos que en las versiones más recientes: todos los ajustes que escribe el panel existen en PHP 5.6.

`packages.sury.org/php` tiene PHP 5.6 a 7.3 para Ubuntu 22.04, 24.04 y 26.04 y Debian 12 y 13, en amd64 y arm64. La instalación también lo comprueba con apt, y se rechaza indicando el motivo si el repositorio no tiene la versión.

### Servidores que usaban el PPA ondrej/php {#ondrej-php-ppa}

Las versiones anteriores del panel tomaban las versiones adicionales de PHP en Ubuntu de `ppa:ondrej/php`. Ese PPA está congelado y ya no recibe actualizaciones: su mantenedor publica ahora los paquetes de Ubuntu en `packages.sury.org/php` (consulta el aviso en [launchpad.net/~ondrej/+archive/ubuntu/php](https://launchpad.net/~ondrej/+archive/ubuntu/php)).

Cuando el panel se inicia en un servidor Ubuntu que todavía tiene el PPA, cambia el servidor una sola vez, como describe el aviso del PPA:

1. Agrega `packages.sury.org/php` con su clave fijada (`/usr/share/keyrings/sury-php.gpg` y `/etc/apt/sources.list.d/sury-php.list`), salvo que ya exista una fuente para él.
2. Mueve el archivo de fuente del PPA desde `/etc/apt/sources.list.d`, y la clave del PPA `/etc/apt/trusted.gpg.d/ondrej-ubuntu-php.gpg` si existe, a `apt-backup` en la carpeta de datos del panel (`/var/lib/nefo/apt-backup`). Esa clave era de confianza para todos los repositorios y ya nada la necesita. Los demás PPA, incluso del mismo mantenedor, no se tocan.

No se instala, quita ni revierte a una versión anterior nada, y el panel no ejecuta `apt-get update` al iniciarse. La siguiente actualización (la búsqueda de actualizaciones del panel, una instalación o tu propio `apt update`) lee la fuente nueva, y los paquetes de PHP instalados reciben sus próximas actualizaciones de ella. El registro del panel indica lo que cambió. Si la clave no se puede descargar, el PPA se queda y el panel lo vuelve a intentar en el siguiente inicio. Una línea del PPA en el propio `/etc/apt/sources.list`, o en un archivo que también enumera otros repositorios, no se edita: el registro te pide que la reemplaces a mano.

### Extensiones {#extensions}

La pestaña **Extensiones** enumera las extensiones opcionales que tienen un paquete para esta versión. Selecciona **Instalar** o **Quitar**, y PHP-FPM se recarga.

Las extensiones opcionales son: apcu, gmp, igbinary, imagick, imap, ldap, memcached, mongodb, msgpack, pgsql, redis, soap, sqlite3, ssh2, tidy, xdebug, xsl y yaml.

::: warning
xdebug hace más lentos todos los sitios de esa versión de PHP. No lo dejes instalado en un servidor de producción.
:::

### Ajustes de php.ini {#php-ini-settings}

La pestaña **Ajustes** define valores de php.ini para todos los sitios web de esta versión de PHP. Deja un campo vacío para usar el valor predeterminado de PHP y luego selecciona **Guardar**.

| Ajuste | Predeterminado | Valores permitidos |
|---|---|---|
| `date.timezone` | UTC | El nombre de una zona horaria, como `Europe/Berlin` |
| `max_input_vars` | 1000 | de 100 a 100000 |
| `max_input_time` | 60 | de -1 a 3600 |
| `default_socket_timeout` | 60 | de 1 a 600 |
| `session.gc_maxlifetime` | 1440 | de 60 a 2592000 |
| `realpath_cache_size` | 4096K | Un número, opcionalmente seguido de K o M |
| `display_errors` | Off | On u Off |
| `expose_php` | Off | On u Off |
| `allow_url_fopen` | On | On u Off |
| `short_open_tag` | Off | On u Off |
| `opcache.enable` | 1 | 1 o 0 |
| `opcache.memory_consumption` | 128 | de 32 a 2048 |
| `opcache.max_accelerated_files` | 10000 | de 1000 a 1000000 |
| `opcache.validate_timestamps` | 1 | 1 o 0 |
| `opcache.revalidate_freq` | 2 | de 0 a 3600 |

Los valores se escriben en `/etc/php/<version>/fpm/conf.d/99-panel.ini` y se comprueban con `php-fpm -t`. Si PHP-FPM los rechaza, vuelve el archivo anterior y no cambia nada. Los límites de memoria, de tamaño de subida y de tiempo se definen por sitio web, en los ajustes del sitio web.

### ionCube Loader {#ioncube}

La tarjeta **ionCube Loader**, debajo de las versiones de PHP, ejecuta archivos PHP codificados con el ionCube Encoder. Lo necesitan algunos scripts PHP comerciales, como WHMCS y otro software de facturación o de licencias, y a menudo los sitios web trasladados desde cPanel.

1. Primero instala las versiones de PHP que usan tus sitios web.
2. En la tarjeta **ionCube Loader**, selecciona **Instalar**.

El panel descarga los loaders de ionCube (`downloads.ioncube.com`) por HTTPS. Instala una versión fijada, la 15.5.1, y comprueba la descarga con la suma de verificación SHA-256 que el panel tiene para ella, porque ionCube no publica sumas de verificación ni firmas. Una descarga que no coincide se rechaza y no cambia nada. Funciona en servidores amd64 y arm64.

Los loaders van a `/usr/local/lib/ioncube`, con la licencia de ionCube (`LICENSE.txt`). Para cada versión de PHP instalada que el loader admite, el panel escribe `00-ioncube.ini` en `/etc/php/<version>/fpm/conf.d` y `/etc/php/<version>/cli/conf.d`, para que el loader se cargue antes que OPcache, como exige ionCube. Comprueba que `php<version> -v` arranca con el loader y luego recarga PHP-FPM. Si PHP no arranca con él, el panel vuelve a quitar el archivo, para que esa versión siga funcionando sin el loader, y la tarea indica qué versión falló.

La tarjeta enumera cada versión de PHP instalada con lo que el loader hace en ella:

| Estado | Significado |
|---|---|
| **cargado** | El loader está cargado para PHP-FPM y la línea de comandos |
| **no cargado** | Existe un loader para esta versión, pero no está cargado. Selecciona **Cargar en todas las versiones** en la tarjeta |
| **ionCube no ofrece loader para esta versión** | PHP 8.0: ionCube nunca publicó un loader para ella |
| "todavía no disponible para PHP 8.6" | Una versión de PHP más reciente que los loaders que instala el panel (la 15.5.1 admite hasta PHP 8.5) |

La versión fijada tiene loaders para PHP 5.6, de 7.0 a 7.4 y de 8.1 a 8.5. Una versión de PHP que instales después también recibe el loader, si existe uno para ella. Desinstalar una versión de PHP quita su `00-ioncube.ini`.

**Desinstalar** quita el loader de todas las versiones de PHP, recarga PHP-FPM y elimina `/usr/local/lib/ioncube`. Los archivos PHP codificados dejan de funcionar.

Para comprobar un sitio web, pon un archivo como este en su carpeta y ábrelo, y luego elimina el archivo:

```php
<?php var_dump(extension_loaded('ionCube Loader'));
```

::: tip
El ionCube Loader es gratuito, pero no es de código abierto. La licencia de ionCube ([ioncube.com/legal/loaders/eula](https://www.ioncube.com/legal/loaders/eula)) permite usarlo sin costo. El panel lo descarga de ionCube cuando lo instalas y no lo distribuye.
:::

## Node.js {#node-js}

La tarjeta **Node.js** instala la versión LTS más reciente desde nodejs.org, comprobada con su suma de verificación SHA-256 publicada. Va a `/opt/node-v<version>-linux-<arch>`, y `node`, `npm` y `npx` se enlazan en `/usr/local/bin`. Este es el Node.js predeterminado del servidor, el que usan los sitios web de aplicaciones que no eligen una versión.

### Varias versiones de Node.js {#several-node-js-versions}

Cuando Node.js está instalado, la tarjeta **Versiones de Node.js** enumera las tres ramas LTS más recientes y cualquier rama instalada. Selecciona **Instalar** para agregar una rama junto a las demás. No cambia la predeterminada.

Elige una rama para un sitio web de aplicación en **Sitios web** → (sitio) → **Entorno de ejecución**. Una rama que usan sitios web de aplicaciones, y la predeterminada, no se pueden desinstalar desde esta tarjeta. Desinstala la predeterminada desde la tarjeta **Node.js**.

La lista viene de nodejs.org, así que el servidor necesita acceso a internet para mostrarla.

::: warning
Desinstalar el Node.js predeterminado también rompe las tareas cron y los programas en segundo plano que llaman a `node`.
:::

## Python {#python}

La tarjeta **Aplicaciones Python** instala `python3-venv` y `python3-pip`, para que los sitios web de aplicaciones puedan ejecutar Python (Django, Flask, FastAPI…). Cada sitio web de aplicación Python tiene su propio entorno virtual en `app/.venv`. Las herramientas de compilación como `build-essential` y `python3-dev` no se instalan, así que un paquete que no tiene un wheel listo y debe compilarse no se puede instalar hasta que las agregues con `apt`.

Desinstalar quita venv y pip. El propio `python3` se queda, porque lo necesita el sistema.

## Java {#java}

La tarjeta **Java (OpenJDK)** instala OpenJDK de tu distribución, para sitios web de aplicaciones escritas en Java, como una aplicación Spring Boot que se inicia con `java -jar`. Es el JDK sin interfaz gráfica (`openjdk-<version>-jdk-headless`): `java`, más `javac`, `jar` y las demás herramientas que necesita una compilación en un script de [despliegue con Git](./git-deploy), sin las partes de escritorio. Va a `/usr/lib/jvm/java-<version>-openjdk-<arch>` y recibe sus actualizaciones de seguridad con el resto del sistema.

**Instalar** en la tarjeta instala la versión de soporte a largo plazo (LTS) más reciente que tiene tu distribución. Las versiones disponibles dependen del sistema:

| Sistema | Versiones de Java |
|---|---|
| Ubuntu 22.04 | 17, 21, 25 |
| Ubuntu 24.04 | 17, 21, 25 |
| Ubuntu 26.04 | 17, 21, 25 |
| Debian 12 | 17 |
| Debian 13 | 21, 25 |

### Varias versiones de Java {#several-java-versions}

Cuando Java está instalado, la tarjeta **Versiones de Java** enumera las versiones LTS que ofrece tu distribución y cualquier versión instalada. Selecciona **Instalar** para agregar una versión junto a las demás. Una versión LTS más reciente aparece sola en cuanto la distribución la tiene.

Elige una versión para un sitio web de aplicación en **Sitios web** → (sitio) → **Entorno de ejecución** (consulta [Sitios web de aplicaciones Java](./websites#java-app-websites)). Una versión que usan sitios web de aplicaciones no se puede desinstalar, y la tarjeta muestra que la usan aplicaciones. **Desinstalar** en la tarjeta **Java (OpenJDK)** quita todas las versiones instaladas, y se rechaza mientras algún sitio web de aplicación ejecute Java.

Cada versión ocupa aproximadamente de 390 a 480 MB de disco, con los paquetes descargados. No se ejecuta nada mientras ningún sitio web la use.

## Redis {#redis}

Redis es una caché en memoria. WordPress puede usarla con un plugin de caché de objetos para Redis.

Cuando instalas Redis, el panel:

- instala `redis-server` y la extensión redis para cada versión de PHP instalada;
- limita la memoria de Redis según el servidor: 64 MB con menos de 1.5 GB de RAM, 128 MB desde 1.5 GB, 256 MB desde 3 GB;
- define la política `allkeys-lru`, para que Redis descarte las claves usadas menos recientemente cuando está lleno, como una caché;
- deja Redis escuchando en `127.0.0.1:6379`, accesible solo desde este servidor.

Para revisar Redis, ve a **Bases de datos** → **Redis**. Consulta [Bases de datos](./databases#redis-key-browser).

Antes de desinstalar Redis, desactiva el plugin de caché de tus sitios web. Siguen funcionando sin la caché, un poco más lentos.

## PostgreSQL {#postgresql}

La tarjeta **PostgreSQL** instala la versión de tu distribución u otra versión mayor del repositorio del proyecto PostgreSQL (apt.postgresql.org), según elijas en el cuadro de diálogo de instalación. El panel lo ajusta a la memoria del servidor en un archivo complementario de `conf.d`, hace que solo escuche en localhost y escribe un `pg_hba.conf` que pide contraseña a todos excepto al usuario del sistema `postgres` en el socket local. Sus bases de datos están en **Bases de datos** → **PostgreSQL**. Consulta [PostgreSQL](./postgresql) para las versiones, la actualización, el acceso remoto, las reglas de importación y las copias de seguridad.

Cuando viene de apt.postgresql.org, la tarjeta dice **De apt.postgresql.org**; si no, **De la distribución**. Al desinstalarlo también se quita ese repositorio, si lo agregó el panel.

Cuando Adminer está instalado, el panel agrega la extensión `pgsql` de PHP para la versión de PHP de Adminer, para que Adminer también abra bases de datos PostgreSQL. La extensión solo se agrega junto con PostgreSQL.

## Memcached {#memcached}

Memcached es una pequeña caché en memoria, para las sesiones de PHP y las cachés de aplicaciones (el controlador de caché `memcached` de Laravel, por ejemplo).

Cuando instalas Memcached, el panel:

- instala `memcached` y la extensión memcached para cada versión de PHP instalada, y agrega la extensión a las versiones de PHP que instales después;
- escribe `/etc/memcached.conf`: Memcached solo escucha en `127.0.0.1:11211`, por TCP, con UDP desactivado (`-U 0`), así que no se puede usar para amplificar ataques;
- limita su memoria según el servidor: 64 MB con menos de 1.5 GB de RAM, 128 MB desde 1.5 GB, 256 MB desde 3 GB, y permite 512 conexiones con menos de 1.5 GB de RAM, y 1024 a partir de ahí.

Cuando la memoria se llena, Memcached descarta los elementos usados menos recientemente. Para ver sus cifras, ve a **Bases de datos** → **Memcached**.

Antes de desinstalar Memcached, lleva a otro lugar las sesiones y la caché de tus sitios web: una aplicación cuyo controlador de caché es Memcached falla sin él.

## Servidor de bases de datos, Adminer y phpMyAdmin {#mariadb-and-adminer}

La tarjeta **Servidor de bases de datos** instala MariaDB o MySQL 8.4 LTS, ajustados a la memoria del servidor y accesibles solo desde este servidor. Un servidor tiene uno de los dos.

Mientras no hay ninguno instalado, la tarjeta ofrece la elección, con **MariaDB** seleccionado: es más ligero y es con lo que funcionan WordPress y la mayoría de las aplicaciones PHP.

- En **MariaDB** eliges la versión cuando el repositorio propio de MariaDB tiene una serie LTS más reciente que tu distribución: la de tu distribución (la predeterminada, como "10.11, la de la distribución"), o **11.4 LTS de MariaDB**, **11.8 LTS de MariaDB** o **12.3 LTS de MariaDB** (en Ubuntu 26.04 y Debian 13, solo 12.3). Cada una indica hasta cuándo la admite MariaDB. Consulta [Versiones de MariaDB](./databases#mariadb-versions).
- **MySQL 8.4 LTS** viene del repositorio de Oracle y se ofrece en Ubuntu 22.04, 24.04 y 26.04 y Debian 12 y 13, solo en servidores amd64. En los demás, la tarjeta indica por qué no se ofrece.

Cuando hay uno instalado, la tarjeta muestra cuál es, su versión y de dónde viene (**de los paquetes de la distribución**, **del repositorio de MariaDB** o **del repositorio de Oracle**), su estimación de memoria, el número de bases de datos, **Abrir Bases de datos** y **Desinstalar**. También ofrece:

- **Actualizar**, en MariaDB, cuando se ofrece una serie LTS más reciente: consulta [Actualizar MariaDB](./databases#upgrade-mariadb).
- El otro servidor como alternativa, **MySQL 8.4 LTS en su lugar** o **MariaDB en su lugar**, con un botón para cambiar: el panel traslada las bases de datos, sus usuarios y contraseñas, y vuelve atrás si algo falla. Consulta [Cambiar entre MariaDB y MySQL](./databases#switch-between-mariadb-and-mysql).
- **Volcados conservados**: las exportaciones que hizo una actualización o un cambio, con su carpeta, su tamaño y un botón para eliminarlas. Consulta [Volcados de actualizaciones y cambios](./databases#dumps-from-upgrades-and-switches).

Adminer y phpMyAdmin muestran tus bases de datos en el navegador y solo son accesibles a través del inicio de sesión del panel. Ambos necesitan un servidor web y PHP, y ambos funcionan con MariaDB y MySQL. Consulta [Bases de datos](./databases#phpmyadmin).

## Docker {#docker}

La tarjeta **Docker** instala `docker.io` de tu distribución, con su plugin Compose cuando hay uno disponible. Aparece entonces el elemento de menú **Docker**.

Los puertos que publican los contenedores Docker se saltan el firewall ufw. Por eso, el panel publica los puertos de los contenedores solo en 127.0.0.1, y llegas a ellos a través de un sitio web. Consulta [Docker](./docker).

## Firewall web y ClamAV {#web-firewall-and-clamav}

- **Firewall web (ModSecurity)** instala ModSecurity con el OWASP Core Rule Set. Necesita el servidor web. Después de instalarlo, actívalo por sitio web en **Sitios web** → (sitio) → **Firewall web (WAF)**. Cuando la distribución no tiene un módulo de ModSecurity para el servidor web, la tarjeta indica por qué y no ofrece la instalación: Ubuntu 22.04 no tiene ninguno para Nginx. Apache y OpenLiteSpeed funcionan allí, y Nginx funciona a partir de Ubuntu 24.04.
- **Antivirus ClamAV** es un motor adicional opcional para los análisis de malware. Sus bases de datos de virus se descargan justo después de la instalación, así que los análisis pueden usarlo unos minutos después. Es adecuado para servidores con 2 GB de RAM o más.

Consulta [Seguridad](./security) para ambos.

## Herramientas: Composer y WP-CLI {#tools}

El grupo **Herramientas** muestra dos herramientas de línea de comandos que el panel instala cuando algo las necesita. Cada tarjeta muestra si la herramienta está instalada, su versión, de dónde viene y qué la usa. **Actualizar** instala la última versión oficial de la herramienta. El panel la descarga por HTTPS con la suma de verificación que el proyecto publica junto a ella, y rechaza una descarga que no coincida: en ese caso, la herramienta instalada se queda como estaba. El archivo nuevo reemplaza al anterior en un solo paso.

### Composer {#composer}

Composer es el gestor de paquetes de PHP. Instalar Laravel o Drupal desde la tienda de aplicaciones lo instala, y los scripts de despliegue con Git y la terminal también pueden ejecutarlo.

- **Instalar** instala el paquete `composer` de tu distribución, que recibe sus actualizaciones de seguridad con el resto del sistema.
- **Actualizar** instala la última versión estable de getcomposer.org en `/usr/local/bin/composer`, comprobada con su suma de verificación SHA-256 publicada. Esa va primero en el `PATH`, así que `composer` en la terminal, los scripts de despliegue con Git y las instalaciones de Laravel y Drupal del propio panel la usan. El paquete de la distribución sigue instalado.
- **Desinstalar** quita ambos. Se rechaza mientras un script de despliegue con Git ejecute `composer`. Los sitios web Laravel y Drupal no lo impiden: solo necesitan Composer para instalarse, y siguen funcionando sin él. Volver a instalar uno de ellos trae de vuelta Composer.

### WP-CLI {#wp-cli}

WP-CLI ejecuta WordPress desde la línea de comandos. Las [herramientas de WordPress](./wordpress) lo usan para las actualizaciones, el modo de mantenimiento, las contraseñas y las copias de staging, y lo instalan la primera vez que lo necesitan.

- **Instalar** y **Actualizar** descargan la última versión desde GitHub en `/usr/local/lib/panel/wp-cli.phar`, comprobada con la suma de verificación SHA-512 que publica la versión.
- **Desinstalar** se rechaza mientras existan sitios web WordPress.
