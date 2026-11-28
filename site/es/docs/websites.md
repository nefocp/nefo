---
title: Sitios web
description: Elige un servidor web, crea sitios web PHP, estáticos, de aplicaciones (Node.js, Python, Java) y de proxy inverso, y administra sus dominios, ajustes de PHP, opciones de acceso, SFTP, tráfico, registros, clones y eliminación.
---

# Sitios web

Un sitio web en Nefo es un dominio que sirve el único servidor web del servidor, Nginx, Apache u OpenLiteSpeed: un sitio web PHP, un sitio web estático, una aplicación (Node.js, Python o Java) detrás de un proxy o un proxy inverso a otra dirección. Cada sitio web tiene su propio usuario de Linux, su carpeta y una cuenta SFTP opcional, y se puede clonar, copiar o eliminar desde sus ajustes.

Esta página trata del servidor web, de cómo crear un sitio web y de todos los ajustes que tiene un sitio web: dominios, PHP, reglas de reescritura, opciones de acceso, SFTP, estadísticas de tráfico, registros, clonación y eliminación. Los certificados tienen su propia página, [Certificados SSL](./ssl), igual que [el bloqueo por país y el bloqueo de archivos](./website-protection).

## ¿Qué servidor web elijo? {#choose-the-web-server}

Un servidor ejecuta un solo servidor web a la vez, y Nefo configura todos los sitios web para él. Antes de que agregues el primer sitio web, la página **Sitios web** te pide que elijas uno:

| Servidor web | Elígelo cuando | Memoria en reposo |
|---|---|---|
| Nginx | Tus sitios web no necesitan archivos `.htaccess`. Es la opción más ligera. | unos 15 MB |
| Apache | Tus sitios dependen de archivos `.htaccess`, por ejemplo plugins de WordPress o sitios trasladados desde un hosting compartido. | unos 40 MB |
| OpenLiteSpeed | Quieres las reglas de reescritura de los archivos `.htaccess` y el plugin LiteSpeed Cache para WordPress. También responde por HTTP/3. | unos 26 MB |

Para instalarlo:

1. Abre **Sitios web**.
2. En **Instalar el servidor web**, selecciona Nginx, Apache u OpenLiteSpeed.
3. Haz clic en **Instalar Nginx** (el botón lleva el nombre del servidor que seleccionaste).

Nefo instala el servidor web y la versión de PHP de tu distribución con PHP-FPM, y luego abre los puertos 80 y 443 en el firewall (ufw), además de 443/udp para HTTP/3 en OpenLiteSpeed. Esto tarda de uno a tres minutos. OpenLiteSpeed viene del repositorio de paquetes propio de LiteSpeed, y su consola WebAdmin queda desactivada.

Los tres servidores web ejecutan PHP en un pool de PHP-FPM separado para cada sitio web. Solo Nginx puede limitar las solicitudes por visitante; los demás ajustes funcionan igual en los tres.

### Cambiar a otro servidor web {#switch-to-another-web-server}

Nefo no puede trasladar los sitios web existentes a otro servidor web, porque la configuración de cada sitio está escrita para el que está instalado. Para cambiar:

1. Elimina todos los sitios web (consulta [Eliminar un sitio web](#delete-a-website)). Antes, haz copias de seguridad si los necesitas: consulta [Copias de seguridad](./backups).
2. Abre **Software** y busca la tarjeta **Nginx, Apache u OpenLiteSpeed**.
3. Haz clic en **Desinstalar** en el servidor web actual, escribe su nombre (`nginx`, `apache` u `openlitespeed`) y confirma.
4. Haz clic en **Instalar** en el servidor web que quieras.

La desinstalación conserva las carpetas de los sitios web en `/var/www`, los certificados y la cuenta de Let's Encrypt. PHP sigue instalado. Adminer se quita porque necesita el servidor web, y se eliminan las reglas de firewall que Nefo agregó para los sitios web (puertos 80 y 443).

Nefo también se niega a instalar un servidor web mientras haya otro instalado en el servidor, incluso uno que instalaste tú mismo: ambos necesitan los puertos 80 y 443. En ese caso, la página **Software** lo muestra como "instalado, el panel no lo usa". Quítalo primero.

## Crear un sitio web {#create-a-website}

1. Apunta el registro A del DNS del dominio a la dirección IP de tu servidor, por ejemplo `203.0.113.10`.
2. Abre **Sitios web** y haz clic en **Agregar sitio web**.
3. En **Dominio**, escribe el nombre sin `http://` ni `www.`, por ejemplo `example.com`.
4. Deja marcada **Responder también en www.example.com** si el sitio también debe responder en `www`. Nefo la marca por ti cuando el dominio tiene dos partes, como `example.com`.
5. Elige el **Tipo** (consulta la tabla de abajo).
6. Haz clic en **Crear sitio web**.

Nefo muestra entonces "example.com está listo. Sube tus archivos a /var/www/example.com/public." Una página de bienvenida responde hasta que la reemplaces.

| Tipo | Qué hace el sitio web |
|---|---|
| **PHP 8.3** (una entrada por cada versión de PHP instalada) | Sirve tus archivos y ejecuta los archivos `.php` con esa versión de PHP. |
| **Estático (solo HTML)** | Sirve HTML, CSS, JavaScript y otros archivos. Sin PHP. |
| **Aplicación (Node.js, Python, Java…) detrás de un proxy** | Reenvía cada solicitud a un programa que escucha en un puerto de este servidor. |
| **Proxy inverso a otra dirección** | Reenvía cada solicitud a una dirección que indicas: otro servidor, o un servicio de este servidor que Nefo no ejecuta. Consulta [Sitios web de proxy inverso](#reverse-proxy-websites). |

Un sitio web no puede cambiar de tipo más adelante: un sitio estático no puede convertirse en un sitio PHP, un sitio de aplicación no puede convertirse en un sitio PHP ni estático, y un sitio web de proxy inverso tampoco puede pasar a otro tipo. En su lugar, crea un sitio web nuevo. Para instalar WordPress u otra aplicación, consulta la [Tienda de aplicaciones](./apps). Las aplicaciones Docker obtienen su propio sitio web desde allí.

Nefo rechaza un dominio no válido (escribe los nombres internacionales en punycode, `xn--…`), un nombre que ya usa otro sitio web y un dominio cuya carpeta ya existe en `/var/www`.

### Sitios web de aplicaciones (Node.js, Python, Java) {#app-websites-node-js-python}

Para el tipo **Aplicación (Node.js, Python, Java…) detrás de un proxy**, el formulario pide dos cosas más:

- **Puerto en el que escucha la aplicación (127.0.0.1)**: un puerto entre 1024 y 65535. Nefo rechaza el propio puerto del panel, 3306 (MariaDB), 6379 (Redis), 7809 (el receptor de webhooks de Git) y un puerto que use otro sitio web de aplicación.
- **Comando de inicio (opcional)**: una línea de 1000 caracteres como máximo, como `node server.js`. Nefo lo ejecuta en la carpeta `app` del sitio web como el usuario de Linux del sitio, con `PORT`, `HOST=127.0.0.1` y `NODE_ENV=production` definidos. Se ejecuta como el servicio de systemd `panel-app-<user>.service` y se reinicia 3 segundos después de detenerse. Deja el comando vacío si el programa se inicia de otra forma, por ejemplo en Docker.

Pon tu código en `/var/www/example.com/app`, por [SFTP](#sftp-access) o con el [despliegue con Git](./git-deploy), que despliega un sitio web de aplicación en esta carpeta. Después, en los ajustes del sitio web:

1. En la pestaña **Entorno de ejecución**, elige el **Entorno de ejecución**: **Predeterminado del servidor (node en /usr/local/bin)**, una versión de Node.js que instalaste en la página **Software**, Python (`app/.venv`), que necesita Python instalado en la página **Software**, o una versión de Java que instalaste en la página **Software**, que aparece por ejemplo como "Java 21 (OpenJDK 21.0.9)". En la misma pestaña se cambia el **Comando de inicio (opcional)**. Haz clic en **Guardar**.
2. En la pestaña **Resumen**, en **Programa de la aplicación**, haz clic en **Instalar dependencias**. Nefo ejecuta `npm ci` (o `npm install` si no hay un `package-lock.json`) para un `package.json`. Para Python crea el entorno virtual `app/.venv` e instala `requirements.txt` con pip. Se ejecuta como el usuario del sitio web, durante 20 minutos como máximo.
3. Haz clic en **Reiniciar** (o en **Iniciar**) para ejecutar la aplicación con sus dependencias.

La tarjeta **Programa de la aplicación** aparece cuando el sitio web tiene un comando de inicio. También tiene **Detener** y **Registros de la aplicación**. En la pestaña **Proxy inverso** puedes cambiar el puerto. El servidor web reenvía las solicitudes a `127.0.0.1` en ese puerto, con la dirección del visitante en `X-Real-IP` y `X-Forwarded-For`, y las conexiones WebSocket funcionan. Para reenviar a otro servidor, o a un servicio de este servidor que Nefo no inicia, crea en su lugar un [sitio web de proxy inverso](#reverse-proxy-websites).

La lista de sitios web tiene un filtro **Java** en cuanto existe un sitio web de aplicación Java, junto a los filtros de los demás tipos.

#### Sitios web de aplicaciones Java {#java-app-websites}

Para un programa Java, define el **Comando de inicio (opcional)** con el comando que lo inicia, por ejemplo `java -jar app.jar`, y pon `app.jar` en `/var/www/example.com/app`. Elige la versión de Java como **Entorno de ejecución**. Primero instala Java en la página **Software** (consulta [Java](./software#java)).

- El servicio de la aplicación recibe el `JAVA_HOME` de esa versión y su carpeta `bin` al principio de `PATH`, así que `java` es esa versión. Además de `PORT` y `HOST=127.0.0.1`, que recibe todo sitio web de aplicación, recibe `SERVER_PORT` (el mismo puerto) y `SERVER_ADDRESS=127.0.0.1`, que lee Spring Boot. Así, una aplicación Spring Boot escucha en el puerto correcto, solo en este servidor, sin ajustes propios. Otros frameworks leen `PORT` o lo necesitan en un ajuste propio.
- **Límite de memoria (MB)** aparece para un entorno de ejecución Java. Define el heap máximo de Java (`-Xmx`), de 32 a 65536 MB, mediante `JDK_JAVA_OPTIONS`, que lee cada `java` que ejecuta el comando de inicio, también desde un script de inicio. Si lo dejas vacío, Java usa hasta una cuarta parte de la memoria del servidor. El proceso usa algo más de memoria que su heap. Java escribe `NOTE: Picked up JDK_JAVA_OPTIONS: -Xmx512m` al principio de los **Registros de la aplicación**.
- Los scripts de [despliegue con Git](./git-deploy) del sitio web se ejecutan con los mismos `JAVA_HOME` y `PATH`, así que `./mvnw package` o `./gradlew build` compilan con la versión de Java del sitio web. El límite de memoria no se les aplica.
- **Instalar dependencias** no se ofrece para un sitio web Java: compila el jar en tu script de despliegue o antes de subirlo.
- Una versión de Java que usa un sitio web no se puede desinstalar en la página **Software**. Primero elige otro entorno de ejecución o elimina el sitio web.

## Sitios web de proxy inverso {#reverse-proxy-websites}

Un sitio web de proxy inverso reenvía cada solicitud a una dirección que eliges: un sitio web o una aplicación en otro servidor, o un servicio de este servidor que Nefo no ejecuta, como un programa que iniciaste tú mismo o un contenedor de tu propio proyecto Compose. El sitio web tiene su propio dominio, certificado HTTPS, opciones de acceso y firewall web, como cualquier otro sitio web. Funciona con Nginx, Apache y OpenLiteSpeed.

1. Abre **Sitios web** y haz clic en **Agregar sitio web**.
2. Escribe el **Dominio** y elige el tipo **Proxy inverso a otra dirección**.
3. En **Reenviar a**, escribe la dirección: `http://` o `https://`, un nombre de host o una dirección IP, un puerto opcional y una ruta opcional, por ejemplo `http://198.51.100.7:8080` o `https://backend.example.com/app`.
4. Elige el **Encabezado Host enviado**: el dominio de este sitio web o el host del destino.
5. Haz clic en **Crear sitio web**.

Nefo muestra "Se creó example.com. Reenvía a … cuando su dominio apunte a este servidor." La dirección se guarda con su puerto, así que `https://backend.example.com/app` aparece como `https://backend.example.com:443/app`.

La ruta de la solicitud va después de la ruta de la dirección: con `http://198.51.100.7:8080/app`, una visita a `https://example.com/shop?page=2` llega a `http://198.51.100.7:8080/app/shop?page=2`.

### Las opciones {#the-options}

Cámbialas en la pestaña **Proxy inverso** del sitio web y haz clic en **Guardar**. Un sitio web de proxy inverso nuevo tiene activados WebSocket y los encabezados X-Forwarded-\*, y un límite de tiempo de 60 segundos.

| Opción | Qué hace |
|---|---|
| **Reenviar a** | La dirección, como se indicó arriba. |
| **Encabezado Host enviado** | El dominio del sitio web envía el dominio que pidió el visitante: lo esperan la mayoría de las aplicaciones detrás de un proxy. **El host del destino** envía el nombre propio del destino (con su puerto si no es 80 ni 443), para un servidor que solo responde a su propio nombre. La dirección propia del destino en las redirecciones (`Location`) se cambia entonces por la del sitio web. |
| **WebSocket** | Deja pasar las conexiones WebSocket (chat, actualizaciones en vivo, terminales). |
| **Encabezados X-Forwarded-\*** | Indica al destino la dirección del visitante y si la solicitud llegó por HTTPS: `X-Forwarded-For`, `X-Forwarded-Proto`, `X-Forwarded-Host` y `X-Real-IP`. Si se desactiva, estos encabezados se quitan, también los que envía un visitante. |
| **Límite de tiempo (segundos)** | Cuánto puede tardar el destino en responder, de 5 a 3600 segundos. La conexión en sí debe abrirse en 30 segundos (o en el límite de tiempo, si es menor). Con un destino más lento, el visitante recibe una página de error: 504 con Nginx, 502 con Apache. |
| **Ignorar el certificado del destino (autofirmado)** | Aparece con una dirección `https://`. Acepta cualquier certificado del destino. |

::: warning
**Ignorar el certificado del destino (autofirmado)** mantiene cifrada la conexión con el destino, pero cualquiera que pueda interceptarla podría hacerse pasar por el destino. Úsalo solo para un destino de tu propia red con un certificado autofirmado.
:::

Sin esa opción, Nginx y Apache comprueban el certificado de un destino `https://` con las autoridades de certificación del servidor (`/etc/ssl/certs/ca-certificates.crt`), para el nombre del encabezado Host: el dominio del sitio web, o el host del destino si lo elegiste. Con un certificado autofirmado o vencido, el visitante recibe una página de error (502 con Nginx, 500 con Apache).

### Lo que Nefo rechaza {#what-nefo-refuses}

- una dirección que no sea `http://` o `https://` con un nombre de host o una dirección IP, y un puerto (de 1 a 65535) y una ruta opcionales, por ejemplo una con un nombre de usuario, una consulta (`?`), un fragmento (`#`), espacios, `..` en la ruta, o las direcciones `0.0.0.0` y `::`;
- un nombre de host que no se encuentra ("No se encuentra el nombre de host …. Revísalo o usa su dirección IP.");
- el propio panel: el puerto del panel o el `7809` (el receptor de webhooks de Git) de este servidor, tanto si escribes `127.0.0.1`, `localhost`, una de las direcciones IP del propio servidor o un nombre que apunta a ellas;
- los puertos 80 y 443 de este servidor: el sitio web reenviaría al propio servidor web;
- un **Límite de tiempo (segundos)** menor que 5 o mayor que 3600.

### Bueno saberlo {#good-to-know}

- Con Nginx, un nombre de host se resuelve cuando llegan las solicitudes, con los servidores DNS de `/etc/resolv.conf`, así que un nombre que no se resuelve al arrancar nunca impide que Nginx se inicie. Estas consultas no leen `/etc/hosts`: para un nombre que solo conoce `/etc/hosts`, usa su dirección IP.
- HTTPS, la redirección a HTTPS y HSTS funcionan como en los demás sitios web: consulta [Certificados SSL](./ssl).
- **Suspender**, **Redirección**, la protección con contraseña, las direcciones IP bloqueadas, el límite de solicitudes, el [bloqueo por país](./website-protection#block-or-allow-countries) y el firewall web se aplican antes de que una solicitud llegue al destino.
- Las pestañas de reglas de reescritura, página 404 personalizada, protección contra hotlinking y bloqueo de archivos no se aplican a un sitio web de proxy inverso y están ocultas. Tus propias directivas (la pestaña de configuración del servidor web) sí se aplican.
- El sitio web sigue teniendo su carpeta `/var/www/example.com` y su usuario de Linux, para SFTP y las copias de seguridad, pero no sirve esos archivos.

### Con OpenLiteSpeed {#with-openlitespeed}

OpenLiteSpeed reenvía de otra forma que Nginx y Apache. La pestaña **Proxy inverso** lo indica en una nota, y **Ignorar el certificado del destino (autofirmado)** no aparece allí:

- Nunca comprueba el certificado de un destino `https://`: un certificado autofirmado funciona sin la opción.
- El **Límite de tiempo (segundos)** no se aplica: OpenLiteSpeed no tiene límite para una respuesta a través del proxy.
- No envía la dirección del visitante en `X-Forwarded-For`: envía `X-Forwarded-Host`, y `X-Forwarded-Proto` en las solicitudes HTTPS. Con los **Encabezados X-Forwarded-\*** desactivados, Nefo quita `X-Forwarded-For` (y `X-Real-IP` se quita siempre); OpenLiteSpeed sigue enviando `X-Forwarded-Host`.
- Las conexiones WebSocket llegan al destino sin la ruta de la dirección (`/live`, no `/app/live`), y no funcionan con un destino `https://`.
- Las rutas, el encabezado Host del destino, la contraseña y las direcciones bloqueadas funcionan como con los demás servidores web.

## Abrir los ajustes de un sitio web {#open-a-website-s-settings}

Haz clic en el dominio en la lista, o en **Ajustes** al final de su fila. Los ajustes se abren en un panel lateral con pestañas en cuatro grupos. Algunas pestañas solo aparecen en algunos sitios web:

| Grupo | Pestañas |
|---|---|
| **Básico** | **Resumen**, **Dominios**, **PHP** (sitios PHP), **Entorno de ejecución** (sitios de aplicación), **Proxy inverso** (sitios de proxy inverso) |
| **Seguridad** | **SSL / HTTPS**, **Firewall web (WAF)**, **Bloquear acceso**, **Bloqueo por país**, **Bloqueo de archivos** (no en aplicaciones Docker ni en sitios de proxy inverso), **Protección contra hotlinking** (no en sitios de proxy inverso), **Análisis de malware** |
| **Configuración** | **Reescritura** (sitios PHP y estáticos), **Caché de páginas** (sitios PHP en Nginx y OpenLiteSpeed), **Configuración de Nginx** (lleva el nombre de tu servidor web), **Redirección**, **Proxy inverso** (sitios de aplicación: el puerto), **Página 404** (no en sitios de proxy inverso) |
| **Operaciones** | **WordPress** (sitios WordPress), **Despliegue con Git** (no en aplicaciones Docker), **SFTP**, **Copias de seguridad**, **Monitoreo**, **Tráfico**, **Registros**, **Vencimiento y suspensión** |

La pestaña **Resumen** muestra la **Carpeta** del sitio web (con **Abrir en Archivos**), su **Usuario de Linux**, los dominios, la versión de PHP o el entorno de ejecución, el servidor web, y el estado de HTTPS y del firewall. Haz clic en **Agregar una nota** para escribir una línea (de hasta 200 caracteres) que aparece bajo el dominio en la lista, como el nombre de un cliente.

## El usuario de Linux y las carpetas del sitio web {#the-website-s-linux-user-and-folders}

Cada sitio web tiene su propio usuario de Linux, así que un sitio hackeado no puede leer ni modificar los demás. El nombre sale del dominio: `example.com` se convierte en `example_com`, y `blog.example.com` en `blog_example_com` (con `_2`, `_3`… si el nombre ya está en uso). El usuario no tiene contraseña ni shell, y no puede iniciar sesión por SSH a menos que actives [SFTP](#sftp-access). El PHP del sitio web y su aplicación se ejecutan como este usuario, y PHP solo puede abrir archivos dentro de la carpeta del sitio web.

| Ruta | Qué contiene |
|---|---|
| `/var/www/example.com/public` | La carpeta web (raíz de documentos). Aquí van los archivos de tu sitio web. |
| `/var/www/example.com/logs` | `php-error.log`, los errores de PHP del sitio web. |
| `/var/www/example.com/tmp` | Los archivos temporales de PHP, las subidas en curso y las sesiones. |
| `/var/www/example.com/app` | Solo sitios web de aplicaciones: el código del programa. |
| `/var/log/nginx/example.com.access.log` y `.error.log` | Los registros del servidor web para el sitio (`/var/log/apache2` en Apache, `/var/log/openlitespeed` en OpenLiteSpeed). |
| `/var/lib/nefo/certs/example.com` | El certificado del sitio web. |
| `/var/cache/nginx/nefo/example.com` | Solo Nginx: la [caché de páginas](#page-cache), cuando está activada. |

Los archivos del servidor web y de PHP-FPM que escribe Nefo empiezan por "Managed by Nefo. Do not edit". Nefo los reescribe, así que haz tus cambios en el panel.

## Dominios y alias {#domains-and-aliases}

En la pestaña **Dominios**, **Nombres de dominio adicionales** enumera los demás nombres que abren el mismo sitio web, uno por línea, como `www.example.com`. Un sitio web puede tener hasta 20. Cada nombre debe apuntar a este servidor y solo puede pertenecer a un sitio web.

Un comodín como `*.example.com` responde para todos los subdominios. Su certificado necesita validación por DNS a través de Cloudflare: consulta [Certificados SSL](./ssl).

Si el DNS del dominio está en Cloudflare, **Apuntar el DNS aquí…**, en la misma pestaña, crea o actualiza los registros A (y AAAA) de todos estos nombres, después de una vista previa: consulta [DNS (Cloudflare)](./dns#point-a-website-to-this-server).

Si el sitio web tiene HTTPS y cambias sus nombres, Nefo dice "Los nombres de dominio cambiaron. Obtén un certificado nuevo para que HTTPS los cubra todos." Haz clic en **Renovar certificado**.

## Versión y ajustes de PHP {#php-version-and-settings}

La pestaña **PHP** de un sitio web PHP tiene estos ajustes:

| Ajuste | Predeterminado | Permitido |
|---|---|---|
| **Versión de PHP** | la versión elegida al crearlo | cualquier versión de PHP instalada |
| **Límite de memoria (MB)** | 256 | de 32 a 4096 |
| **Subida máxima (MB)** | 64 | de 1 a 4096 (el límite de tamaño de solicitud del servidor web lo sigue) |
| **Ejecución máxima de un script (segundos)** | 300 | de 10 a 3600 |
| **Workers de PHP** | 5 | de 1 a 200 |
| **Modo de los workers** | **Iniciar bajo demanda (ahorra memoria)** | o **Mantener algunos listos (más rápido para sitios con mucho tráfico)** |

**Workers de PHP** indica cuántas solicitudes atiende a la vez el PHP del sitio. Cada worker puede usar hasta el límite de memoria, así que workers × memoria debería caber en la RAM del servidor.

Por seguridad, Nefo desactiva `exec`, `passthru`, `shell_exec`, `system`, `proc_open`, `popen` y `pcntl_exec` en todos los sitios web, porque permiten que un sitio hackeado ejecute programas en el servidor. En **Permitir funciones desactivadas por seguridad**, marca una solo si una aplicación realmente la necesita.

Haz clic en **Guardar**. Si PHP-FPM o el servidor web rechazan el resultado, Nefo vuelve a poner los ajustes anteriores. Para instalar otras versiones de PHP o extensiones, o el ionCube Loader, o para cambiar `php.ini`, consulta [Software](./software).

## Reglas de reescritura y tus propias directivas {#rewrite-rules-and-your-own-directives}

En la pestaña **Reescritura**, **Reglas de reescritura** ofrece reglas listas para aplicaciones comunes: **Predeterminado (controlador frontal: index.php o index.html)**, WordPress, Laravel, Symfony, CodeIgniter 4, Drupal, Joomla, OpenCart, Nextcloud, Moodle y phpBB para sitios PHP, y Aplicación de una sola página y Sitio estático (URL limpias) para cualquier sitio. La pestaña muestra las reglas del preajuste que elijas. En Nginx, un preajuste reemplaza el bloque `location /` predeterminado. En Apache, el propio archivo `.htaccess` del sitio puede reemplazar el controlador frontal del preajuste, y en OpenLiteSpeed el controlador frontal se ejecuta después de las reglas del `.htaccess` del sitio. En ambos, los archivos `.htaccess` no pueden anular las reglas de protección del preajuste.

**Sitio estático (URL limpias)** sirve las páginas sin su terminación `.html`, tal como las enlazan los generadores de sitios como VitePress (con `cleanUrls`): `/docs/install` sirve `docs/install.html`, y `/docs/` sirve `docs/index.html`. Los demás archivos se sirven tal cual, y una página que no existe sigue recibiendo un 404.

El preajuste de phpBB es para un phpBB que instalas tú mismo. Envía a `app.php` de phpBB las direcciones que no son archivos, y nunca sirve los archivos de ajustes de phpBB (`config.php`, `common.php`), sus carpetas `cache`, `files` (adjuntos), `store` (copias de seguridad), `includes`, `phpbb` y `vendor` ni `images/avatars/upload`: esas responden 403.

La siguiente pestaña lleva el nombre de tu servidor web, por ejemplo **Configuración de Nginx**. En **Directivas de Nginx** (o **Directivas de Apache**, **Directivas de OpenLiteSpeed**) puedes agregar tus propias directivas, hasta 32 KB, con la sintaxis del servidor web:

- Nginx las agrega dentro del bloque `server` del sitio, antes de las reglas de reescritura.
- Apache las agrega al final del bloque `VirtualHost` del sitio.
- OpenLiteSpeed las agrega al final del bloque `virtualHost` del sitio.

El servidor web comprueba la configuración nueva antes de que cambie nada. Si la rechaza, Nefo muestra el error y el sitio web sigue funcionando como antes. Haz clic en **Mostrar el archivo de configuración completo** para ver lo que genera Nefo.

Sobre los archivos `.htaccess`: Apache los lee. OpenLiteSpeed solo usa sus reglas de reescritura, que Nefo copia en la configuración del sitio a los pocos segundos de un cambio. Nginx los ignora.

## Caché de páginas {#page-cache}

En Nginx, la pestaña **Caché de páginas** de un sitio web PHP activa para él la caché FastCGI de Nginx: Nginx guarda cada página que genera PHP y la vuelve a servir sin ejecutar PHP ni consultar la base de datos. Activa el interruptor **Caché de páginas**.

- Una página se guarda 10 minutos, y una página inexistente (404), 1 minuto. Si PHP falla o deja de responder, Nginx sirve la última copia que todavía tiene.
- Nginx consulta a PHP cada vez en las solicitudes que no son GET ni HEAD (formularios), en las direcciones con un `?`, para los visitantes con una cookie de inicio de sesión o de carrito (WordPress `wordpress_logged_in_`, `wp-postpass_`, `comment_author_`, WooCommerce `woocommerce_items_in_cart`, `wp_woocommerce_session_`, y la propia `PHPSESSID` de PHP), y en las páginas bajo `/wp-admin/`, `/wp-login.php`, `/xmlrpc.php`, `/wp-json/`, `/wp-cron.php`, `/cart/`, `/checkout/` y `/my-account/`. Tampoco se guardan las páginas que PHP marca como privadas o envía con una cookie.
- Cada sitio web tiene su propia caché de hasta 256 MB en `/var/cache/nginx/nefo/example.com`. Una página que nadie pidió en una hora se descarta.

Después de cambiar una página, haz clic en **Vaciar caché**: todos los visitantes reciben la versión nueva de inmediato. Si no, los visitantes que no iniciaron sesión pueden ver la página anterior durante 10 minutos como máximo. Desactivar la caché, o eliminar el sitio web, elimina su carpeta.

Para comprobar que funciona, abre una página dos veces en una ventana privada, o ejecuta `curl -sI https://example.com/` dos veces: el encabezado `X-Cache-Status` dice `MISS` y luego `HIT` (`BYPASS` para las solicitudes de arriba). Los archivos estáticos no tienen ese encabezado: Nginx los sirve directamente de todos modos.

En OpenLiteSpeed, la pestaña indica que LSCache, su propia caché de páginas, viene integrada: para WordPress instala el plugin LiteSpeed Cache; las demás aplicaciones la activan con reglas de caché en `.htaccess`. Apache no tiene caché de páginas en Nefo, ni pestaña.

## Suspender, redirigir o proteger con contraseña un sitio web {#suspend-redirect-or-password-protect-a-website}

**Suspender.** Desactiva el interruptor de la columna **En ejecución** de la lista, o **Sitio web en ejecución** en la pestaña **Resumen**. Los visitantes reciben entonces HTTP 503 con "This website is temporarily unavailable." Los archivos y los ajustes se conservan, y el certificado se sigue renovando. Para suspender varios sitios, márcalos, elige **Suspender** en **Acción masiva** y haz clic en **Aplicar**; **Reactivar** los vuelve a activar.

**Redirigir.** En la pestaña **Redirección**, marca **Redirigir todas las solicitudes a otra dirección** y escribe la **Dirección**, como `https://example.org`. Es una redirección permanente (301) que conserva la ruta: `/page` va a la dirección nueva seguida de `/page`. Una redirección solo funciona mientras el sitio web está en ejecución.

**Contraseña.** En la pestaña **Bloquear acceso**, en **Protección con contraseña**, marca **Proteger con contraseña** y escribe un **Nombre de usuario** y una **Contraseña** de 8 a 72 caracteres. El navegador los pide entonces antes de mostrar el sitio. Más adelante, deja la contraseña vacía para conservar la actual. La validación de Let's Encrypt y los webhooks de Git siguen llegando al sitio.

La misma pestaña bloquea direcciones IP y, en Nginx, limita las solicitudes por visitante: consulta [Seguridad](./security#rate-limit-and-blocked-addresses). Para dejar fuera países enteros, consulta [Bloqueo por país](./website-protection#block-or-allow-countries).

## Página 404 personalizada y protección contra hotlinking {#custom-404-page-and-hotlink-protection}

**Página 404.** En la pestaña **Página 404**, marca **Usar mi propia página 404** y sube tu página como `public/404.html`. Las páginas que tu aplicación gestiona por sí misma, como las de WordPress o Laravel, conservan su propia página 404.

**Protección contra hotlinking.** En la pestaña **Protección contra hotlinking**, marca **Protección contra hotlinking para imágenes y multimedia**. Los demás sitios web reciben entonces HTTP 403 cuando insertan tus archivos `jpg`, `jpeg`, `png`, `gif`, `webp`, `avif`, `svg`, `ico`, `bmp`, `mp4`, `webm`, `mp3`, `ogg` y `pdf`. Tu propio sitio, los buscadores (Google, Bing, DuckDuckGo y Yahoo) y los visitantes que abren un archivo directamente siempre lo reciben. Enumera hasta otros 20 sitios en **Otros sitios que pueden mostrarlos**, uno por línea.

## Fecha de vencimiento {#expiry-date}

En la pestaña **Vencimiento y suspensión**, define una **Fecha de vencimiento** y haz clic en **Guardar**. El sitio web funciona hasta el final de ese día (hora del servidor); después, Nefo suspende el sitio automáticamente. Lo comprueba cada hora. Haz clic en **Nunca** para quitar la fecha. En los últimos 7 días, la lista muestra "vence el" y la fecha bajo el dominio. Es útil para sitios de clientes pagados hasta una fecha.

La misma pestaña tiene **Suspender este sitio web** para suspender el sitio ahora.

## Acceso SFTP {#sftp-access}

SFTP te permite a ti o a un cliente subir archivos con FileZilla, WinSCP o cualquier aplicación SFTP. Así, los archivos pertenecen al usuario del sitio web, como debe ser. Necesita el servidor OpenSSH instalado.

1. Abre la pestaña **SFTP** del sitio web y marca **Permitir el inicio de sesión SFTP para este sitio web**.
2. Elige cómo inicia sesión la cuenta:
   - **Generar una contraseña segura nueva**
   - **Elegir una contraseña (al menos 12 caracteres)**, de hasta 128 caracteres
   - **Conservar la contraseña actual** (cuando hay una definida)
   - **Sin contraseña: iniciar sesión solo con claves SSH**
3. Opcional: pega **Claves públicas SSH (opcional)**, una por línea como en `authorized_keys`, hasta 50. Las opciones como `command=` se quitan.
4. Haz clic en **Guardar**. Una contraseña generada se muestra una sola vez: cópiala ahora.
5. Conéctate con el **Host**, el **Puerto**, el **Nombre de usuario** y la **Carpeta** que se muestran. El host es la dirección con la que abriste el panel, y el puerto es el puerto SSH del servidor. La pestaña también muestra la dirección completa, por ejemplo `sftp://example_com@panel.example.com/var/www/example.com`.

La cuenta queda limitada a la carpeta del sitio web. Solo ve `/var/www/example.com`, en la misma ruta que en el panel y en PHP, y no puede ejecutar comandos ni reenviar puertos. La política de contraseñas del servidor puede rechazar una contraseña que elijas; en ese caso, Nefo muestra el motivo. Mientras los [archivos del sitio web están bloqueados](./website-protection#lock-a-website-s-files), la cuenta SFTP solo puede cambiar las carpetas con permiso de escritura.

Desactivar SFTP quita la contraseña y las claves, y cierra las sesiones abiertas.

## Estadísticas de tráfico {#traffic-statistics}

La pestaña **Tráfico** lee los registros de acceso del servidor web para el sitio web. Elige el **Periodo**: **Hoy**, **7 días**, **14 días** o **30 días**. Muestra:

- **Solicitudes**, **Visitantes** (direcciones IP distintas), **Ancho de banda**, **Tasa de errores** (4xx y 5xx) y **Bots**
- las solicitudes y los visitantes por hora o por día
- **Páginas más visitadas**, **Direcciones IP principales**, **Sitios de referencia**, **Navegadores** y **Bots y scripts**

No se cuentan las comprobaciones de certificados ni las comprobaciones de [monitoreo](#monitoring) del propio panel. El servidor conserva unos 14 días de registros, así que no se conoce el tráfico más antiguo. En registros muy grandes, solo se leen las 5 000 000 líneas más recientes. La columna **Visitas, 7 días** de la lista de sitios web sale de los mismos registros.

## Registros {#logs}

La pestaña **Registros** muestra las líneas más recientes de:

- **Registro de acceso (Nginx)** y **Registro de errores (Nginx)** (llevan el nombre de tu servidor web)
- **Errores de PHP**, para sitios web PHP (`/var/www/example.com/logs/php-error.log`)
- **Registros de la aplicación**, para sitios web de aplicaciones y Docker

## Monitoreo {#monitoring}

La pestaña **Monitoreo** comprueba cada pocos minutos que el sitio web responde, y te envía una alerta cuando no lo hace. Activa **Monitorear este sitio web**; la primera comprobación se ejecuta de inmediato.

El panel solicita la dirección pública del sitio web, con `https://` cuando tiene un certificado (si no, `http://`), a través del DNS como cualquier visitante. Así también se detectan un registro DNS incorrecto, una página de error de Cloudflare o un certificado vencido. Se siguen las redirecciones a los propios dominios del sitio web (como de `http://` a `https://`, o a `www.`). Una redirección a otro dominio cuenta como respuesta.

| Ajuste | Qué hace | Predeterminado |
|---|---|---|
| **Comprobar** | con qué frecuencia: cada minuto, o cada 2, 5 o 10 minutos | cada 5 minutos |
| **Ruta** | la página que se solicita, como `/health`; se permite una cadena de consulta | `/` |
| **Texto que debe contener la página** | opcional; no distingue mayúsculas de minúsculas | vacío |
| **Contar las respuestas 4xx, como 404, como caída** | una respuesta 4xx hace fallar la comprobación | desactivado |

Una comprobación falla cuando:

- no llega ninguna respuesta en 10 segundos, el dominio no se resuelve, la conexión se rechaza, o el certificado venció, no es de confianza o es de otro dominio
- la respuesta es un error HTTP 500 o superior. Una respuesta 4xx cuenta como en línea, salvo que cuentes las respuestas 4xx como caída
- el texto que debe contener la página no está en los primeros 256 KB de la página

El sitio web pasa a **Caído** tras 2 comprobaciones fallidas seguidas, y vuelve a estar en línea tras una comprobación correcta. El panel envía entonces una alerta a todos los canales de **Configuración** → **Alertas** (consulta [Alertas](./mail-alerts#alerts)):

- cuando se cae, con el motivo, como "example.com is down (HTTP 502, 2 checks in a row)", y de nuevo cada 6 horas mientras siga caído
- cuando vuelve a estar en línea, con cuánto tiempo estuvo caído
- cuando el certificado que sirve vence en menos de 14 días, como máximo una vez al día

La pestaña muestra el estado y la última comprobación, la disponibilidad en 24 horas y en 7 días, el tiempo de respuesta medio, los tiempos de respuesta de las últimas 24 horas en un gráfico, y los últimos fallos con sus motivos. El tiempo de respuesta se mide hasta que llegan los encabezados de la respuesta, incluida la conexión. En la lista de sitios web, un sitio web monitoreado que está caído muestra **Caído** en rojo en su columna **En ejecución**, y la campana lo enumera.

- Las comprobaciones envían el agente de usuario `Nefo-Monitor/1.0`. No se cuentan en la pestaña **Tráfico**.
- Los resultados se conservan 7 días. Eliminar el sitio web los elimina.
- Un sitio web suspendido no se comprueba.
- Las comprobaciones se ejecutan en este servidor. No pueden avisar de que todo el servidor está caído, ni de un problema en la red entre tus visitantes y el servidor: para eso, agrega un servicio de monitoreo externo.

## Firewall web, análisis de malware, bloqueo por país, bloqueo de archivos y copias de seguridad {#web-firewall-malware-scan-country-blocking-file-lock-and-backups}

- **Firewall web (WAF)**: ModSecurity con el OWASP Core Rule Set para este sitio: **Desactivado**, **Solo detectar: registrar ataques, no bloquear nada** o **Bloquear ataques**. Primero instala el firewall en la página **Software**. Consulta [Seguridad](./security#web-application-firewall), que también trata la pestaña **Análisis de malware**.
- **Bloqueo por país**: rechaza a los visitantes de algunos países, o deja entrar solo a los de algunos. **Bloqueo de archivos**: hace que el código del sitio web sea de solo lectura para su PHP y su cuenta SFTP, salvo las carpetas en las que debe escribir. Consulta [Bloqueo por país y bloqueo de archivos](./website-protection).
- **Copias de seguridad**: **Hacer copia ahora**, o programa una copia de seguridad **Todos los días a las 03:00**. Consulta [Copias de seguridad](./backups).

## Clonar un sitio web {#clone-a-website}

**Clonar** copia un sitio web a un dominio nuevo: para empezar un sitio web nuevo a partir de uno existente, o para probar cambios en una copia. Funciona con sitios web PHP, estáticos y de proxy inverso, con sitios web de aplicaciones (Node.js, Python, Java) y con aplicaciones Docker de la [Tienda de aplicaciones](./apps).

1. Abre la pestaña **Resumen** del sitio web y haz clic en **Clonar…** en **Clonar este sitio web**.
2. Escribe el **Dominio de la copia**.
3. Deja marcada **Copiar las bases de datos vinculadas a este sitio web** para copiarlas también. La casilla no aparece en WordPress, cuya base de datos siempre se copia, ni en las aplicaciones Docker, cuyos datos siempre se copian.
4. Haz clic en **Clonar**. El sitio web sigue funcionando durante la copia, que se ejecuta como una tarea con registro. Una aplicación Docker se detiene un momento mientras se copian sus datos.
5. Apunta el DNS del dominio nuevo a este servidor y luego activa HTTPS en su pestaña **SSL / HTTPS**. La copia empieza en `http://`.

### Sitios web PHP, estáticos y de proxy inverso {#php-static-and-reverse-proxy-websites}

La copia recibe:

- un sitio web nuevo del mismo tipo (la misma versión de PHP, estático, o un proxy inverso a la misma dirección con las mismas opciones) con su propio usuario de Linux, la misma aplicación, los mismos ajustes de PHP, reglas de reescritura y directivas propias. En tus directivas, la carpeta del sitio web anterior se reemplaza por la nueva;
- una copia de la carpeta del sitio web `/var/www/example.com`, propiedad del usuario nuevo, sin `logs`, `tmp` ni la clave de despliegue de Git (`.ssh`). Los enlaces simbólicos se copian como enlaces; un enlace a un lugar dentro de la carpeta del sitio web anterior lleva ahora al mismo lugar en la nueva. En `public/.user.ini` la carpeta anterior se reemplaza por la nueva;
- por cada base de datos vinculada, MariaDB, MySQL o PostgreSQL, una base de datos y un usuario nuevos del mismo tipo con una copia de sus datos, vinculados al sitio web nuevo;
- los programas en segundo plano del sitio web, detenidos (consulta más abajo).

Cuando la copia está lista, Nefo muestra los datos de conexión de las bases de datos MariaDB y MySQL nuevas (las de PostgreSQL están en la página **Bases de datos**). Los archivos de configuración propios de la aplicación (como `.env` o `config.php`) siguen nombrando la base de datos y el usuario anteriores: cámbialos por los nuevos. Los archivos que nombran la carpeta anterior, como una caché de los ajustes de la aplicación, también siguen haciéndolo.

### Sitios web de aplicaciones {#app-websites}

La copia de un sitio web de aplicación recibe, además:

- un puerto propio, elegido por Nefo, que la aplicación recibe en `PORT`. Una aplicación que define su propio puerto debe cambiarse para usar el nuevo;
- el mismo **Entorno de ejecución** y el mismo **Comando de inicio (opcional)**; en el comando, la carpeta del sitio web anterior se reemplaza por la nueva. El servicio `panel-app-<user>.service` de la copia se inicia cuando ya se copiaron los archivos y las bases de datos;
- `node_modules` con sus enlaces (`node_modules/.bin`), y un entorno virtual de Python `app/.venv` que funciona en su carpeta nueva: las rutas de sus scripts y de los archivos `pyvenv.cfg` y `.pth` nombran la carpeta de la copia. Las dependencias no se vuelven a instalar;
- los programas en segundo plano del sitio web, con el mismo comando, carpeta y variables (con las rutas a la carpeta anterior reemplazadas), pero **detenidos**: hasta que la copia use bases de datos y colas propias, trabajarían sobre las del original. Inícialos en la página **Programas en segundo plano**.

El entorno de ejecución tiene que seguir instalado: si quitaste de la página **Software** una versión de Node.js, Python o Java, la copia se rechaza antes de empezar.

### Aplicaciones Docker {#docker-apps}

La copia del sitio web de una aplicación Docker es un sitio web nuevo con una instalación nueva de la aplicación:

- la misma versión e imágenes que el original (también cuando la Tienda de aplicaciones tiene una más reciente), con sus propios contenedores y puertos, y los mismos límites de memoria;
- una copia de los datos de la aplicación desde `/var/lib/panel-apps/<name>`, hecha con el original detenido: Nefo lo detiene, copia los datos y lo vuelve a iniciar enseguida. En la copia, los archivos del usuario del sitio web original pertenecen al usuario del sitio web nuevo;
- las mismas cuentas y contraseñas que el original (los datos copiados se crearon con ellas), que aparecen en **Tienda de aplicaciones** → **Instaladas** como en cualquier aplicación;
- la dirección de la aplicación cambiada al dominio nuevo, en las aplicaciones que la toman del panel (`url`, `DOMAIN` y ajustes similares de la aplicación). Si la aplicación guarda su dirección en sus propios ajustes, cámbiala allí.

Primero, Nefo comprueba la memoria y el disco que necesita la copia (más o menos el tamaño de los datos de la aplicación, más 512 MB).

Nefo se niega a clonar:

| Aplicación | Por qué |
|---|---|
| n8n | La copia ejecutaría por segunda vez los flujos de trabajo activos (programaciones y disparadores). Exporta los flujos de trabajo e impórtalos en un n8n nuevo. |
| Odoo | La copia también ejecutaría las acciones programadas y los servidores de correo: enviaría los mismos correos, y descargaría y eliminaría los mensajes del mismo buzón. Haz una copia de seguridad o instala un Odoo nuevo. |
| Una aplicación instalada antes de la Tienda de aplicaciones | Primero actualízala (**Tienda de aplicaciones** → **Instaladas**) y luego clónala. |

Una copia de una aplicación de monitoreo como Uptime Kuma comprueba los mismos sitios web y envía las mismas notificaciones que el original: pausa en uno de los dos lo que no necesites.

### WordPress {#wordpress}

Un sitio WordPress se copia como una [copia de staging](./wordpress#make-a-staging-copy): su base de datos siempre se copia, `wp-config.php` apunta a la copia y la dirección del sitio se reemplaza en todas partes, así que la copia funciona de inmediato. Se pide a los buscadores que no la indexen; si la copia es un sitio web independiente, cámbialo en WordPress en **Settings → Reading**. A diferencia de una copia de staging, un clon no se puede pasar de vuelta al sitio web del que salió.

### Lo que no se copia {#not-copied}

Los alias, el certificado, la protección con contraseña, las redirecciones y los ajustes del firewall, el bloqueo por país, el bloqueo de archivos, las tareas cron, los ajustes de despliegue con Git, el acceso SFTP, el monitoreo y las programaciones de copias de seguridad.

Si un paso falla, Nefo quita todo lo que creó para la copia: el sitio web, su usuario de Linux, las bases de datos nuevas, los programas en segundo plano copiados y, en una aplicación Docker, los contenedores nuevos y sus datos. El original sigue funcionando.

## Eliminar un sitio web {#delete-a-website}

1. Abre la pestaña **Resumen** del sitio web.
2. En **Eliminar este sitio web**, haz clic en **Eliminar**.
3. El cuadro de diálogo enumera en **También se elimina:** lo que se va con el sitio web (su aplicación Docker y sus datos, la cuenta SFTP, las tareas cron, los programas en segundo plano, el monitoreo, los ajustes de despliegue con Git y las programaciones de copias de seguridad).
4. Deja marcada **Hacer antes una copia de seguridad del sitio web** (la opción predeterminada), o desmárcala.
5. Marca **Eliminar también sus bases de datos** para eliminar también las bases de datos vinculadas. Si no la marcas (lo predeterminado), se quedan en la página **Bases de datos**, sin vincular a ningún sitio web.
6. Escribe el dominio para confirmar y haz clic en **Hacer copia y eliminar** (o en **Eliminar**, sin la copia de seguridad).

### La copia de seguridad previa {#the-backup-made-first}

Con **Hacer antes una copia de seguridad del sitio web**, Nefo hace primero una copia de seguridad normal en este servidor: los archivos del sitio web y sus bases de datos MariaDB, MySQL y PostgreSQL, y en una aplicación Docker sus datos, como una segunda copia de seguridad del tipo **Aplicación**. El sitio web solo se elimina si la copia de seguridad salió bien. Si falla, la tarea se detiene con "the backup failed, so example.com was not deleted" y no se quita nada.

- La copia de seguridad se llama `example.com_before-delete_<date>-<time>.tar.gz` (y `<user>_before-delete_….tar.gz` para los datos de una aplicación Docker), en `/var/backups/panel/site/example.com/`.
- Como toda copia de seguridad que haces a mano, se conserva hasta que la eliminas en la página **Copias de seguridad**. Las programaciones solo eliminan sus propias copias, y las programaciones del sitio web se quitan con él, así que nada la elimina.
- Se queda solo en este servidor: no se sube al almacenamiento remoto. Súbela desde la página **Copias de seguridad** si quieres una copia remota.
- La eliminación se ejecuta como una tarea con registro. Su registro nombra la copia de seguridad.

::: warning
Sin la copia de seguridad, los archivos del sitio web (y sus bases de datos, si marcas **Eliminar también sus bases de datos**) no se pueden recuperar. El cuadro de diálogo lo advierte cuando desmarcas **Hacer antes una copia de seguridad del sitio web**.
:::

### ¿Puedo recuperar un sitio web eliminado? {#bring-a-deleted-website-back}

Sí, con la copia de seguridad hecha antes de la eliminación (o con cualquier otra copia de seguridad del sitio web). En la página **Copias de seguridad**, restaura la copia `…_before-delete_…` y escribe el dominio para confirmar. La restauración vuelve a crear el sitio web con su dominio, sus alias, su versión de PHP, sus reglas de reescritura, sus archivos y sus bases de datos. Una base de datos que se eliminó vuelve con la contraseña que tenía, que nombran los archivos restaurados; una base de datos que conservaste se vuelve a vincular al sitio web.

No se vuelven a crear: el certificado (obtén uno nuevo en la pestaña **SSL / HTTPS**), las tareas cron, el acceso SFTP, los programas en segundo plano, las redirecciones y la protección con contraseña. Consulta [Restaurar en un servidor nuevo](./restore-new-server#restoring-a-website-that-doesn-t-exist-here) para ver lo que crea la restauración.

Las copias de seguridad hechas con Nefo 0.24.1 o posterior registran el tipo del sitio web, así que un sitio web de aplicación (Node.js, Python, Java) vuelve con su entorno de ejecución, su comando de inicio, su puerto y su límite de memoria, y un sitio web de proxy inverso, con su dirección y sus opciones; primero instala el entorno de ejecución de la aplicación en la página **Software**. Las copias hechas con versiones anteriores solo registran la versión de PHP, así que un sitio web así vuelve como sitio web estático: primero vuelve a crearlo con el mismo dominio y el mismo tipo, y luego restaura. En una aplicación Docker, vuelve a instalar la aplicación en el mismo dominio desde la **Tienda de aplicaciones** y luego restaura su copia de seguridad del tipo **Aplicación**.

### Lo que Nefo quita y lo que conserva {#what-nefo-removes-and-keeps}

Nefo quita:

- la carpeta del sitio web `/var/www/example.com` con todos sus archivos, incluidos un repositorio Git y su clave de despliegue
- el usuario y el grupo de Linux, y el acceso SFTP
- la configuración del servidor web, el archivo de contraseñas y tus propias directivas
- el pool de PHP-FPM, el servicio de la aplicación o la aplicación Docker con sus contenedores, datos e imágenes sin usar
- el certificado, la caché de páginas y los registros del servidor web del sitio
- los programas en segundo plano del sitio web, las tareas cron que se ejecutan como su usuario, sus programaciones de copias de seguridad, su monitoreo y sus ajustes de despliegue con Git
- la papelera del sitio web en **Archivos** y sus archivos puestos en cuarentena por el escáner de malware
- las bases de datos vinculadas, si marcaste **Eliminar también sus bases de datos**

Nefo conserva las copias de seguridad que ya hiciste: se quedan en la página **Copias de seguridad**.

Copias de staging de WordPress: eliminar un sitio en producción que tiene una copia de staging conserva la copia, que ya no se puede pasar a un sitio en producción; el cuadro de diálogo lo indica. Eliminar una copia de staging no cambia su sitio en producción.

Mientras se ejecuta un despliegue con Git del sitio web, Nefo se niega a eliminarlo ("Hay otra tarea en curso en el servidor. Espera a que termine."). Con la copia de seguridad, la tarea espera a que termine un despliegue que empezó durante la copia. Si falla un paso de limpieza, el sitio web se quita del panel de todos modos y Nefo enumera los pasos que fallaron en "Fallaron algunos pasos de limpieza:" (en el registro de la tarea, cuando se hizo antes la copia de seguridad).

Desde la API: `DELETE /api/sites/{id}` con `{"confirm": "example.com", "delete_databases": false, "backup_first": true}`. Con `backup_first` responde 202 con la tarea. Sin él, el sitio web se elimina de inmediato (200, con las advertencias).
