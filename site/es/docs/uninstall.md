---
title: Desinstalar
description: Quita Nefo de un servidor a mano, y decide primero qué pasa con los sitios web y el software que configuró.
---

# Desinstalar

Nefo no tiene un comando de desinstalación: lo quitas a mano, como root, en unos pocos pasos. Desbloquea los sitios web bloqueados con `nefo unlock-files`, detén y quita el servicio `nefo` y `/usr/local/bin/nefo`, cierra el puerto del panel y elimina `/etc/nefo` y `/var/lib/nefo`. Tus sitios web, bases de datos y software siguen funcionando, salvo que los quites antes.

Esta página muestra cada paso. También explica qué queda en el servidor, porque quitar el panel no quita los sitios web ni el software que configuraste con él.

## Qué queda cuando quitas Nefo {#what-stays-when-you-remove-nefo}

Los pasos de esta página quitan el propio panel: su programa, su servicio, su configuración y sus datos. Todo lo que configuraste a través del panel se queda y sigue funcionando:

- el servidor web, PHP, MariaDB, Redis, Docker y los demás componentes de la página **Software**;
- tus sitios web en `/var/www`, sus usuarios de Linux y sus bases de datos;
- las tareas cron, los programas en segundo plano, los sitios web de aplicaciones y los contenedores Docker;
- las copias de seguridad guardadas en el servidor, en `/var/backups/panel`.

Sin el panel, nada renueva los certificados de tus sitios web ni ejecuta tus copias de seguridad programadas. Los certificados de los sitios web se guardan en la carpeta de datos de Nefo, `/var/lib/nefo/certs`. Si eliminas esa carpeta mientras los sitios web todavía usan HTTPS, el servidor web no puede cargar sus certificados.

Así que decide primero:

- **Para quitarlo todo**, quita primero tus sitios web y tu software en el panel (paso 2) y luego quita Nefo.
- **Para mantener los sitios web funcionando** sin el panel, omite el paso 2 y conserva `/var/lib/nefo/certs` cuando elimines los datos (paso 7). Los certificados de Let's Encrypt duran como máximo 90 días, así que configura otra forma de renovarlos antes de que caduquen.

## 1. Guarda copias de lo que necesites {#_1-keep-copies-of-what-you-need}

1. En la página **Copias de seguridad**, haz una copia de seguridad reciente de cada sitio web y base de datos que quieras conservar, con **Hacer copia ahora**.
2. Copia las copias de seguridad fuera del servidor. Están en `/var/backups/panel`, y en tu almacenamiento remoto si lo configuraste.

## 2. Quita los sitios web y el software en el panel {#_2-remove-websites-and-software-in-the-panel}

Hazlo mientras el panel sigue en funcionamiento. El panel se niega a desinstalar un componente mientras algo todavía lo usa, así que sigue este orden:

1. **Sitios web:** abre cada sitio web y, en su pestaña **Resumen**, haz clic en **Eliminar**. Marca **Eliminar también sus bases de datos** si no las necesitas. Desmarca **Hacer antes una copia de seguridad del sitio web** si ya guardaste una copia (paso 1). Esto elimina los archivos del sitio web, su usuario de Linux y su certificado.
2. **Bases de datos:** elimina las bases de datos que queden.
3. **Docker:** elimina los contenedores y los proyectos de Compose que creaste.
4. **Tareas cron** y **Programas en 2.º plano:** elimina los que creaste.
5. **Software:** haz clic en **Desinstalar** en cada componente que ya no necesites, como Docker, MariaDB, Redis y el servidor web. En MariaDB y Docker, la carpeta de datos (`/var/lib/mysql` para MariaDB, `/var/lib/docker` para Docker) se conserva, salvo que marques la casilla para eliminarla también.

La última versión de PHP se queda, porque la configuración del servidor web necesita una. Si no la necesitas, quítala con apt después de haber quitado Nefo.

## 3. Anota el puerto del panel {#_3-note-the-panel-s-port}

Necesitas el puerto para cerrarlo después en el firewall:

```sh
nefo info --port
```

## 4. Desbloquea los archivos de los sitios web {#unlock-the-websites-files}

Si bloqueaste los archivos de algunos sitios web (**Sitios web** → (sitio) → **Bloqueo de archivos**, consulta [Bloqueo por país y bloqueo de archivos](./website-protection#lock-a-website-s-files)), sus archivos pertenecen a root mientras están bloqueados. Devuélveselos a los sitios web antes de quitar el panel, o seguirán siendo de root:

```sh
systemctl stop nefo
nefo unlock-files
```

El comando enumera cada sitio web que desbloqueó:

```
Unlocked the files of example.com
Unlocked the files of blog.example.com
```

Cada archivo y cada carpeta recuperan exactamente el propietario, el grupo y los permisos que tenían cuando se bloquearon. Los archivos que root puso en un sitio web mientras tanto siguen siendo de root. El comando también quita los grupos de bloqueo de los sitios web (`panel-lock-<uid>`). Lee los registros de `/var/lib/nefo/filelock`, así que ejecútalo antes de eliminar los datos del panel (paso 7). Ejecutarlo de nuevo no hace nada: muestra `No website's files are locked.`

## 5. Detén y quita el servicio y el programa {#_5-stop-and-remove-the-service-and-the-program}

```sh
systemctl disable --now nefo
rm /etc/systemd/system/nefo.service
systemctl daemon-reload
rm /usr/local/bin/nefo
```

## 6. Cierra el puerto del panel {#_6-close-the-panel-s-port}

Si ufw está activado, elimina las reglas del puerto del panel. El instalador las agregó con el comentario `Nefo`.

1. Muestra las reglas con sus números:

   ```sh
   ufw status numbered
   ```

2. Elimina la regla del puerto del panel por su número, por ejemplo:

   ```sh
   ufw delete 3
   ```

3. ufw suele tener una segunda regla para IPv6, marcada con `(v6)`. Los números cambian después de cada eliminación, así que vuelve a mostrar las reglas y elimina también esa.

Si usas firewalld, con el puerto 28443 como ejemplo:

```sh
firewall-cmd --permanent --remove-port=28443/tcp
firewall-cmd --reload
```

Quita también el puerto del firewall en la nube de tu proveedor.

## 7. Elimina la configuración y los datos {#_7-delete-the-configuration-and-the-data}

::: danger Esto no se puede deshacer
Esto elimina la cuenta del panel, sus ajustes, la lista de sitios web, las programaciones de copias de seguridad, el registro de actividad, el certificado propio del panel y los certificados de tus sitios web. Asegúrate de haber guardado lo que necesitas (paso 1).
:::

Para eliminarlo todo:

```sh
rm -rf /etc/nefo /var/lib/nefo
```

Si tus sitios web se quedan (consulta más arriba), conserva sus certificados y elimina el resto:

```sh
rm -rf /etc/nefo
find /var/lib/nefo -mindepth 1 -maxdepth 1 ! -name certs -exec rm -rf {} +
```

## 8. Opcional: quita lo que Nefo dejó en otros lugares {#_8-optional-remove-what-nefo-left-elsewhere}

Mientras lo usabas, Nefo también creó archivos fuera de sus propias carpetas. Quita solo lo que ya no necesites:

| Ruta | Qué es |
|---|---|
| `/var/backups/panel` | Las copias de seguridad guardadas en este servidor |
| `/etc/cron.d/panel` y `/var/log/panel-cron` | Las tareas cron creadas en el panel y sus registros. Las tareas siguen ejecutándose hasta que eliminas el archivo. |
| `/var/lib/panel-trash` | Los archivos eliminados en el administrador de archivos, que se conservan durante 30 días |
| `/var/lib/panel-quarantine` | Los archivos que el escáner de malware apartó |
| `/var/lib/panel-geoip` | La base de datos de países del bloqueo por país |
| `/var/backups/nefo-postgresql` | Los volcados hechos antes de una actualización de PostgreSQL |
| `/var/lib/panel-acme` | La carpeta que responde a las comprobaciones de Let's Encrypt. Quítala solo cuando ningún servidor web la use. |
| `/etc/ssh/sshd_config.d/99-panel-sftp.conf` | Las reglas de SSH para las cuentas de los sitios web y SFTP |
| `/etc/fail2ban/jail.d/panel.conf` y `/etc/fail2ban/filter.d/panel.conf` | Las reglas de fail2ban para SSH y el panel, si instalaste fail2ban desde el panel |
| `/etc/ssh/sshd_config.d/00-panel.conf` | Los ajustes de SSH que guardaste en el panel |

::: warning Ajustes de SSH
Si cambiaste el puerto SSH o desactivaste el inicio de sesión con contraseña en el panel, esos ajustes están en `/etc/ssh/sshd_config.d/00-panel.conf`. Eliminar el archivo cambia la forma en que inicias sesión por SSH cuando SSH se reinicie. Conserva el archivo salvo que sepas que quieres volver a los ajustes predeterminados, y prueba una sesión SSH nueva antes de cerrar la actual.
:::
