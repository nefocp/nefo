---
title: Importar sitios web
description: Trae sitios web desde cPanel, aaPanel, HestiaCP, CyberPanel u otro servidor, con sus archivos, bases de datos, certificados y tareas cron.
---

# Importar sitios web

Nefo importa sitios web desde una copia de seguridad completa de cPanel, con sus bases de datos y usuarios, certificados y tareas cron, o desde un archivo comprimido del sitio web y volcados SQL hechos por aaPanel, HestiaCP, CyberPanel o cualquier otro servidor. Primero muestra un plan, no cambia nada en este servidor hasta que confirmas y, si la importación falla, quita lo que creó.

Hay dos formas de importar:

- **Copia de seguridad completa de cPanel**: todos los sitios web de una cuenta de cPanel, con sus bases de datos y sus usuarios, certificados y tareas cron.
- **Archivo comprimido del sitio web y volcados SQL**: los archivos de un sitio web como archivo comprimido `.tar.gz`, `.tgz` o `.zip`, con volcados `.sql` o `.sql.gz` opcionales. Úsalo para aaPanel, HestiaCP, CyberPanel o cualquier otro servidor.

La importación nunca toca el servidor anterior. Nefo primero lee el archivo comprimido y muestra lo que encontró; nada cambia en este servidor hasta que confirmas.

## Hacer la copia de seguridad {#make-the-backup}

### cPanel {#cpanel}

En cPanel, abre **Files → Backup** y haz clic en **Download a Full Account Backup**. Deja **Home Directory** como destino y haz clic en **Generate Backup**. cPanel escribe `backup-M.D.YYYY_HH-MM-SS_USER.tar.gz` en tu carpeta personal y te envía un correo cuando está lista.

Con acceso root a WHM, ejecuta en su lugar `/scripts/pkgacct USER`: escribe `/home/cpmove-USER.tar.gz`. Ambos archivos tienen la misma estructura y Nefo lee los dos, también de versiones antiguas de cPanel.

Una copia de seguridad parcial (las descargas "Home Directory" o "MySQL Databases") no es una copia de seguridad completa. Para esas, usa **Archivo comprimido del sitio web y volcados SQL**.

### aaPanel {#aapanel}

1. Abre **Website**, haz clic en **Backup** en la fila del sitio web y luego en **Backup now**. aaPanel escribe `/www/backup/site/DOMAIN/web_DOMAIN_YYYYMMDD_HHMMSS_xxxxxx.tar.gz` (versiones antiguas: `/www/backup/site/`).
2. Abre **Database**, haz clic en **Backup** en la fila de la base de datos y luego en **Backup now**. aaPanel escribe `db_NAME_YYYYMMDD_HHMMSS_mysql_data.sql.gz` en `/www/backup/database/`.

El archivo comprimido del sitio tiene la carpeta del sitio web en su nivel superior; Nefo importa lo que hay dentro. Nefo también lee la carpeta anterior (`/www/wwwroot/DOMAIN`) en el `.user.ini` del sitio y la reemplaza en la base de datos de WordPress.

### HestiaCP, CyberPanel y otros servidores {#hestiacp-cyberpanel-and-other-servers}

Sus propias copias de seguridad empaquetan cada sitio web en archivos de formato propio (HestiaCP los comprime con zstd), así que haz en su lugar un archivo comprimido simple, por SSH en el servidor anterior:

```bash
# The website's files: the folder that holds index.php or index.html
tar czf /root/site.tar.gz -C /home/USER/web/example.com public_html
# Each database
mysqldump --single-transaction --routines --triggers DBNAME | gzip > /root/DBNAME.sql.gz
```

En HestiaCP la carpeta es `/home/USER/web/DOMAIN/public_html`, en CyberPanel `/home/DOMAIN/public_html`, y en un servidor sin panel normalmente `/var/www/html`. También funciona un archivo comprimido cuyo nivel superior es una carpeta `public_html`: Nefo importa lo que hay dentro.

## Llevar el archivo comprimido a este servidor {#get-the-archive-onto-this-server}

Cualquiera de estas opciones:

- **SFTP o scp** a una carpeta de root, por ejemplo `scp backup.tar.gz root@NEW-SERVER:/root/`.
- **Archivos**: súbelo, o usa **Descargar desde una URL** (consulta [Archivos](./files)).
- **Una dirección web**: dale a la importación una dirección `http` o `https`. Nefo descarga el archivo en `/var/tmp/nefo-import`, en el disco de este servidor, y lo quita después de la importación. Se rechazan las direcciones de las propias redes de este servidor.

Nefo lee archivos comprimidos desde cualquier lugar excepto las carpetas del sistema (`/etc`, `/proc`, `/sys`, `/dev`, `/run`, `/boot`) y sus propios datos. El disco necesita espacio para los archivos de los sitios web y aproximadamente el doble del tamaño de los volcados; el plan muestra lo que se necesita y lo que está libre.

## Importar paso a paso {#import-step-by-step}

1. Abre **Sitios web** y haz clic en **Importar**, junto a **Agregar sitio web**.
2. Elige **Copia de seguridad completa de cPanel** o **Archivo comprimido del sitio web y volcados SQL**.
3. Escribe la ruta del archivo (**Examinar** enumera las carpetas y los archivos comprimidos de este servidor) o su dirección web. Para un archivo comprimido de sitio web, agrega los volcados SQL.
4. Haz clic en **Analizar**. Nefo lee todo el archivo comprimido una vez, sin descomprimirlo; las copias de seguridad grandes tardan unos minutos. **Detener** lo detiene.
5. Revisa el plan y marca lo que quieres traer (ver más abajo). Haz clic en **Importar**.
6. Sigue el registro. Los sitios web nuevos permanecen cerrados (error 503) hasta que termina la importación. **Detener la importación** quita lo que se haya creado hasta el momento.
7. Cuando termina, la página enumera los sitios web nuevos con **Abrir el sitio web** y **Administrar**, las bases de datos y lo que conviene revisar.

Si sales de la página, la importación continúa; vuelve a abrir **Sitios web** → **Importar** para verla. Un archivo comprimido analizado se recuerda durante seis horas.

## Qué se importa desde cPanel {#what-is-imported-from-cpanel}

| En la copia de seguridad | En Nefo |
|---|---|
| El dominio principal, cada dominio adicional y cada subdominio | Un sitio web por cada uno. Se omite el subdominio interno de cPanel de un dominio adicional (`addon.main.com`). |
| Los dominios estacionados (alias) | Alias del sitio web del dominio principal, con su `www.` cuando el servidor anterior lo tenía. |
| Cada raíz de documentos (`public_html`, `public_html/blog`, `addon.com`…) | La carpeta `public` del sitio web. Una carpeta de otro dominio importado que esté dentro va solo al sitio web de ese dominio. |
| La versión de PHP (`ea-php81`) | La misma versión cuando está instalada aquí; si no, la predeterminada de este servidor. Un sitio web sin archivos PHP se convierte en un sitio web estático. |
| `mysql/DB.sql` | Una base de datos con el mismo nombre cuando está libre aquí; si no, `NAME_2`, vinculada al sitio web que la usa. |
| Los usuarios de bases de datos y sus contraseñas (`mysql.sql`) | Cada base de datos recibe un usuario: el que nombra su `wp-config.php` de WordPress o, si no, uno con acceso a ella. El usuario conserva su contraseña anterior: MariaDB acepta el hash de la contraseña anterior tal cual, o Nefo usa la contraseña de `wp-config.php`. Si no, el usuario recibe una contraseña nueva. |
| WordPress | Se detecta por su `wp-config.php`. Nefo marca el sitio web como WordPress, aplica las reglas de reescritura de WordPress, apunta `wp-config.php` al nuevo nombre de la base de datos, usuario o contraseña cuando cambiaron, y reemplaza la carpeta anterior (`/home/USER/public_html`) por la nueva en la base de datos. |
| Los certificados (`apache_tls/`, `sslcerts/`, `ssl/`) | Se instalan como tu propio certificado cuando lo marcas: solo cuando su clave está en la copia de seguridad, es válido hoy y cubre todos los nombres del sitio web. Si no, obtén uno de Let's Encrypt después de la importación. |
| Las tareas cron (`cron/USER`) | Se muestran, y solo se instalan cuando las marcas: como el propio usuario del sitio web (nunca root), con las raíces de documentos anteriores y los programas PHP de cPanel reemplazados por los de este servidor. Se marca un comando que todavía nombra la carpeta personal anterior. |
| `.htaccess`, `.user.ini` | La carpeta anterior se reemplaza por la nueva, y las líneas del gestor de PHP de cPanel salen de `.htaccess`. |

El plan indica por qué un dominio no se puede importar, por ejemplo cuando un sitio web de aquí ya tiene ese dominio.

## Lo que no se importa {#what-is-not-imported}

- **Correo**: buzones, reenvíos, respuestas automáticas y filtros. Nefo no tiene servidor de correo. Traslada el correo a un servicio de correo antes de apagar el servidor anterior. Para el correo que envían los sitios web, consulta [Correo y alertas](./mail-alerts).
- **Zonas DNS**: Nefo no ejecuta ningún servidor DNS. Apunta los dominios a este servidor donde se administre su DNS, o con [DNS (Cloudflare)](./dns).
- **Cuentas FTP**: aquí los sitios web usan SFTP, una cuenta por sitio web (la pestaña **SFTP** del sitio web).
- Las listas de correo, las bases de datos PostgreSQL, los archivos de la carpeta personal que están fuera de las raíces de documentos y los ajustes propios de cPanel.

## Archivos comprimidos de sitios web {#website-archives}

- No se conserva una carpeta única en el nivel superior del archivo comprimido (como en las copias de seguridad de aaPanel): lo que hay dentro se convierte en el sitio web. También funciona una carpeta personal que contiene `public_html`.
- El sitio web puede ser uno nuevo (se te piden su dominio, `www.` y versión de PHP) o uno existente vacío: un sitio web PHP o estático cuya carpeta `public` no contiene nada más que su página de bienvenida, y cuyos archivos no están [bloqueados](./website-protection#lock-a-website-s-files).
- Cada volcado SQL se convierte en una base de datos nueva con un usuario del mismo nombre y una contraseña nueva. Un `wp-config.php` de WordPress se apunta al volcado que nombra (o al único que haya). Para otras aplicaciones, cambia su configuración a la nueva base de datos, usuario y contraseña (**Mostrar acceso** en la página **Bases de datos**).

## Volcados de MySQL 8 {#dumps-from-mysql-8}

Los volcados hechos con MySQL 8, que usan aaPanel y muchos servidores cPanel, usan intercalaciones (collations) que MariaDB no conoce, como `utf8mb4_0900_ai_ci`, y líneas que MariaDB rechaza. Nefo los adapta mientras entran, sin cambiar tus datos:

- Las intercalaciones de MySQL 8 se convierten en la más parecida de MariaDB (`utf8mb4_unicode_520_ci`, `utf8mb4_bin` para las que distinguen mayúsculas y minúsculas, o la propia del idioma, como `utf8mb4_vietnamese_ci`).
- Los ajustes que solo conoce MySQL 8, como `DEFAULT ENCRYPTION='N'` y `SRID`, se convierten en simples comentarios.
- Se omiten `SET @@GLOBAL.GTID_PURGED` y las líneas `SQL_LOG_BIN` que lo rodean.
- Se omiten `CREATE DATABASE` y `USE` de la propia base de datos del volcado, para que vaya a la base de datos nueva.

El texto dentro de tus datos nunca se cambia. El registro indica cuántas líneas se adaptaron.

## Cómo trata Nefo los archivos comprimidos {#how-nefo-treats-archives}

Un archivo comprimido de otro servidor puede contener cualquier cosa, así que Nefo comprueba cada entrada:

- Se omiten los nombres con `..` o una ruta absoluta, los enlaces simbólicos, los enlaces duros, los dispositivos y los FIFO, y el registro indica cuántos.
- Los archivos pierden setuid y setgid, y el permiso de escritura para el grupo y los demás. Todo pertenece al propio usuario de Linux del sitio web.
- Los archivos se escriben solo a través de la carpeta del sitio web: nada puede acabar fuera de ella.
- Se omite un archivo de más de 8 GB; se rechaza un archivo comprimido que al descomprimirse ocupa mucho más que su propio tamaño (una bomba de descompresión).
- El SQL se ejecuta con una cuenta de base de datos temporal que solo puede usar la base de datos nueva, nunca como root.

Si la importación falla o se detiene, Nefo quita los sitios web y las bases de datos que creó. Un sitio web que no pudo quitar queda suspendido, con una nota que dice que la importación no terminó.

## Después de la importación {#after-the-import}

1. **Prueba antes de cambiar el DNS**: apunta los dominios a este servidor en el archivo `hosts` de tu computadora y abre los sitios web.
2. **DNS**: apunta el registro A (y AAAA) de cada dominio a este servidor.
3. **HTTPS**: cuando el DNS apunte aquí, obtén certificados de Let's Encrypt en la pestaña **SSL / HTTPS** de cada sitio web. Nefo no renueva un certificado importado. Consulta [Certificados SSL](./ssl).
4. **PHP**: revisa la versión y los ajustes de PHP de cada sitio web en su pestaña **PHP**.
5. **Tareas cron**: agrega las que no importaste en la página **Tareas cron**.
6. **Correo**: asegúrate de que el correo funciona en otro lugar antes de apagar el servidor anterior.

## Si algo sale mal {#if-something-goes-wrong}

- **"not a cPanel full backup"** en el registro: el archivo comprimido no tiene `userdata/main`. Es una copia de seguridad parcial o un archivo comprimido simple: elige **Archivo comprimido del sitio web y volcados SQL**.
- **"uses another database"**: el volcado escribe en una segunda base de datos. Exporta cada base de datos por separado, sin `--databases` ni `--all-databases`.
- **"not enough free disk space"**: no hay suficiente espacio libre en el disco. Libera espacio, o importa menos sitios web a la vez.
- **Un sitio WordPress muestra "Error establishing a database connection"**: abre su `wp-config.php` en **Archivos** y compara `DB_NAME`, `DB_USER` y `DB_PASSWORD` con **Mostrar acceso** en la página **Bases de datos**.
