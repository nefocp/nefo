---
title: Sistema
description: Instala las actualizaciones del sistema operativo, activa las actualizaciones de seguridad automáticas, agrega swap, define la zona horaria, la sincronización del reloj y el nombre de host, activa TCP BBR y libera espacio en disco.
---

# Sistema

La página **Sistema** instala las actualizaciones del sistema operativo, con las de seguridad marcadas y comprobadas cada pocas horas, activa las actualizaciones de seguridad automáticas, crea un archivo de swap, define la zona horaria, la sincronización del reloj y el nombre de host, activa TCP BBR y libera espacio en disco. Cada cambio de esta página se anota en el registro de actividad.

La página está en el grupo **Servidor** del menú de la izquierda. Tiene cinco pestañas: **Actualizaciones**, **Swap**, **Hora y nombre**, **Red** y **Limpieza**. El registro de actividad está en **Cuenta** → **Actividad reciente**.

Los cambios largos, como instalar actualizaciones o crear un archivo de swap, se ejecutan como una tarea en segundo plano: la página muestra su progreso, y solo se ejecuta una tarea a la vez.

## Actualizaciones {#updates}

La pestaña **Actualizaciones** enumera las actualizaciones pendientes del sistema operativo: cada paquete, su versión instalada y la nueva, y de dónde viene la versión nueva (la suite, como `noble-updates`). Las actualizaciones de una suite de seguridad (`noble-security`, `bookworm-security`…) se marcan como **Seguridad** y aparecen primero. Un paquete marcado como **nuevo** se instala con una actualización, como un kernel nuevo.

### Cómo busca el panel las actualizaciones {#how-the-panel-checks-for-updates}

- El panel las busca por su cuenta: cada hora revisa su última comprobación, y cuando tiene más de 6 horas, ejecuta `apt-get update` y le pregunta a apt qué instalaría una actualización. Omite la comprobación mientras se ejecuta otra tarea, y durante los primeros 30 minutos después de que se inicia el panel.
- Abrir la página nunca ejecuta apt: muestra la última comprobación, con su hora.
- **Comprobar ahora** comprueba de inmediato, como una tarea.
- **Retenidas por ahora** enumera las actualizaciones que apt todavía no instala: los paquetes que retiene porque necesitarían otros cambios, y las actualizaciones por fases de Ubuntu, que llegan a los servidores a lo largo de unos días.

Cuando hay actualizaciones de seguridad esperando, **Requiere atención**, en el panel principal, y la campana muestran "Hay N actualizaciones de seguridad disponibles".

### Instalar actualizaciones {#install-updates}

- **Instalar todas las actualizaciones** ejecuta `apt-get upgrade --with-new-pkgs`. Instala los paquetes nuevos que necesita una actualización, como un kernel nuevo, y nunca quita un paquete.
- **Instalar solo las actualizaciones de seguridad** solo actualiza los paquetes instalados cuya versión nueva viene de una suite de seguridad (`apt-get install --only-upgrade` con sus nombres).

Ambas actualizan primero las listas de paquetes, se ejecutan sin preguntas y conservan los archivos de configuración que modificaste. Cuando terminan, la lista se vuelve a comprobar.

::: warning Los servicios se reinician un momento
Actualizar Nginx, Apache, OpenLiteSpeed, PHP, MariaDB o Redis reinicia ese servicio, así que los sitios web pueden dejar de responder durante unos segundos. Actualizar Docker lo reinicia a él y a sus contenedores. La página nombra los servicios que reinician las actualizaciones pendientes, y también lo hace la confirmación. Instala esas actualizaciones en un momento tranquilo.
:::

### Reinicio necesario {#restart-required}

Algunas actualizaciones, como un kernel nuevo o una biblioteca de C nueva, solo se completan después de reiniciar el servidor. La pestaña dice "El servidor necesita reiniciarse para terminar de instalar las actualizaciones." cuando:

- existe `/var/run/reboot-required` (Ubuntu lo escribe, y enumera los paquetes en `/var/run/reboot-required.pkgs`), o
- hay instalado en `/boot` un kernel más reciente que el que está en ejecución (Debian no siempre escribe el archivo).

**Reiniciar en Configuración** abre **Configuración** → **Panel** → **Energía**, donde puedes reiniciar el servidor. La campana también muestra "El servidor necesita reiniciarse" hasta que lo hagas.

### Actualizaciones de seguridad automáticas {#automatic-security-updates}

El interruptor **Actualizaciones de seguridad automáticas** permite que el servidor instale por su cuenta las actualizaciones de seguridad, todos los días:

- Si `unattended-upgrades` no está instalado, el panel lo instala primero, como una tarea. Su ayudante que espera al apagado del servidor usa unos 20 MB de memoria.
- El panel define `APT::Periodic::Update-Package-Lists "1";` y `APT::Periodic::Unattended-Upgrade "1";` en `/etc/apt/apt.conf.d/20auto-upgrades`, conservando las demás líneas del archivo, y activa los temporizadores diarios de apt.
- Desactivar el interruptor define `APT::Periodic::Unattended-Upgrade "0";`.

Qué actualizaciones se instalan lo deciden los propios ajustes de unattended-upgrades en `/etc/apt/apt.conf.d/50unattended-upgrades`: en Ubuntu y Debian, solo las actualizaciones de seguridad. No reinicia el servidor salvo que actives allí `Unattended-Upgrade::Automatic-Reboot`. Si otro archivo de `/etc/apt/apt.conf.d` sobrescribe el ajuste, el panel lo indica y deja el archivo como estaba.

## Swap {#swap}

La swap es espacio en disco que el kernel usa cuando se acaba la memoria. En un servidor con 1 o 2 GB de memoria, de 1 a 2 GB de swap evitan que el kernel detenga MariaDB y PHP cuando un momento de mucha carga necesita más memoria de la que tiene el servidor.

La pestaña **Swap** muestra la memoria, la swap en uso y cada archivo o partición de swap de `/proc/swaps`.

### Crear un archivo de swap {#create-a-swap-file}

1. Elige el **Tamaño**: 512 MB, 1 GB, 2 GB, 4 GB u 8 GB. Solo se ofrecen los tamaños que dejan 1 GB de disco libre.
2. Selecciona **Crear un archivo de swap**.

El panel crea `/panel.swap`, que solo puede leer root, con `fallocate`. Donde el sistema de archivos no lo admite, o `swapon` rechaza ese archivo, escribe el archivo con `dd`, lo que tarda alrededor de un minuto por cada pocos gigabytes. En btrfs, el archivo se marca primero como no copy-on-write. Luego el panel ejecuta `mkswap` y `swapon`, y agrega `/panel.swap none swap sw 0 0` a `/etc/fstab`, para que la swap vuelva a estar activa después de un reinicio.

El panel se niega a crear un archivo de swap:

- cuando el disco no tiene libre el tamaño del archivo más 1 GB;
- cuando el servidor ya tiene un archivo de swap que no creó el panel, como `/swapfile` o `/swap.img`. El panel lo deja como está. Una partición de swap no lo impide;
- en ZFS, donde los archivos de swap no funcionan;
- en un contenedor (OpenVZ, LXC, Docker), donde el kernel no permite `swapon`: "Este servidor no permite swap". Pide swap a tu proveedor, o usa un servidor virtual completo.

Si algo falla, el panel vuelve a quitar el archivo.

### Quitar el archivo de swap {#remove-the-swap-file}

**Quitar el archivo de swap** desactiva la swap con `swapoff`, quita su línea de `/etc/fstab` y elimina `/panel.swap`. Lo que está en la swap vuelve primero a la memoria, así que el panel se niega cuando no cabría en la memoria libre.

### Swappiness {#swappiness}

**Swappiness** (`vm.swappiness`, de 0 a 100) define con qué facilidad el kernel mueve memoria a la swap. El valor predeterminado es 60; 10 es adecuado para un servidor, que así solo usa la swap cuando falta memoria. El panel escribe el valor en `/etc/sysctl.d/90-panel-swap.conf` y lo aplica de inmediato.

## Hora y nombre de host {#time-and-host-name}

La pestaña **Hora y nombre** muestra la hora del servidor, su zona horaria y si el reloj está sincronizado.

### Zona horaria {#time-zone}

Elige una zona horaria de la lista (`timedatectl list-timezones`) y selecciona **Guardar**. El panel la define con `timedatectl set-timezone` y reinicia cron, para que las tareas cron sigan la zona horaria nueva. Otros servicios que ya estaban en ejecución, como MariaDB, usan la zona horaria nueva después de reiniciarse.

### Sincronización del reloj {#clock-synchronization}

**Sincronizar el reloj (NTP)** activa o desactiva la sincronización de la hora (`timedatectl set-ntp`). Mantenla activada: los certificados y los códigos de dos factores necesitan un reloj correcto. Necesita un servicio de hora como `systemd-timesyncd`. Sin uno, el interruptor aparece en gris: instálalo con `apt install systemd-timesyncd`.

### Nombre de host {#host-name}

Escribe el **Nombre de host** nuevo y selecciona **Guardar**. Un nombre de host tiene letras, dígitos y guiones, con puntos entre las partes, 64 caracteres como máximo, y no puede empezar ni terminar con un guion: `web1` o `web1.example.com`.

El panel:

1. lo define con `hostnamectl set-hostname`;
2. apunta a él la línea `127.0.1.1` de `/etc/hosts` (el nombre completo y su primera parte), o agrega esa línea;
3. en un servidor con cloud-init, escribe `/etc/cloud/cloud.cfg.d/99-panel-hostname.cfg` con `preserve_hostname: true`, para que cloud-init no vuelva a poner el nombre del proveedor en el siguiente arranque.

El nombre de host no cambia la dirección del panel ni ningún certificado. Cuando reinicias el servidor desde **Configuración** → **Panel** → **Energía**, confirmas escribiendo el nombre del servidor: después de un cambio, escribe el nuevo.

## Red: TCP BBR {#network-tcp-bbr}

La pestaña **Red** muestra el control de congestión TCP en uso (normalmente `cubic`), la cola predeterminada (`net.core.default_qdisc`, normalmente `fq_codel`) y los algoritmos que tiene el kernel.

BBR es un control de congestión de Google. A menudo hace más rápidos los sitios web y las descargas en conexiones largas o con pérdidas, como hacia visitantes de otro continente o en redes móviles. Activar **TCP BBR**:

1. carga el módulo `tcp_bbr`, cuando el kernel lo tiene como módulo;
2. escribe `/etc/modules-load.d/panel-bbr.conf` (`tcp_bbr`) y `/etc/sysctl.d/90-panel-bbr.conf` (`net.core.default_qdisc = fq` y `net.ipv4.tcp_congestion_control = bbr`);
3. ejecuta `sysctl --system`.

Las conexiones nuevas usan BBR de inmediato. La cola `fq` se aplica a las tarjetas de red después del siguiente reinicio; mientras tanto, BBR funciona bien sin ella.

El panel se niega cuando el kernel en ejecución no tiene BBR, y en un contenedor, donde no se pueden cambiar los ajustes del kernel. En ese caso no guarda nada. Desactivar BBR elimina ambos archivos y vuelve a `cubic` y `fq_codel`. Cuando BBR se configuró fuera del panel, el interruptor aparece en gris.

## Limpieza del disco {#disk-cleanup}

La pestaña **Limpieza** muestra cuánto espacio libera cada limpieza. **Limpiar** te pide confirmación, ejecuta la limpieza como una tarea y luego muestra el espacio liberado.

| Elemento | Qué se quita |
|---|---|
| **Paquetes descargados** | Los archivos de paquetes que apt guarda en `/var/cache/apt/archives` después de instalar (`apt-get clean`). |
| **Diario del sistema** | Las entradas antiguas del diario del sistema, hasta dejarlo en 100 MB (`journalctl --rotate`, luego `journalctl --vacuum-size=100M`). |
| **Registros antiguos** | Los registros de `/var/log` que logrotate ya rotó: los nombres que terminan en `.gz` o en un número, como `syslog.1` o `auth.log.2.gz`. |
| **Paquetes sin usar** | Los paquetes que ya nada necesita, como los kernels antiguos (`apt-get autoremove`). |
| **Docker** | Las imágenes que ningún contenedor usa, y la caché de compilación (`docker image prune --all`, `docker builder prune --all`). Solo aparece cuando Docker está instalado. |

- **Registros antiguos** conserva los registros del servidor web en `/var/log/nginx`, `/var/log/apache2` y `/var/log/openlitespeed`, porque las estadísticas de tráfico de los sitios web los leen. Solo elimina archivos de las carpetas propias de root y nunca sigue enlaces.
- En **Paquetes sin usar**, la confirmación enumera los paquetes. Si para cuando confirmas apt fuera a quitar algo más, no se quita nada: "La lista de paquetes sin usar cambió".
- **Docker** nunca quita contenedores, detenidos o no, ni volúmenes, donde los contenedores guardan sus datos.

## Registro de actividad {#activity-log}

Estas acciones aparecen en **Cuenta** → **Actividad reciente**: actualizaciones del sistema instaladas, activación o desactivación de las actualizaciones de seguridad automáticas, creación o eliminación del archivo de swap, cambio de swappiness, cambio de zona horaria, activación o desactivación de la sincronización del reloj, cambio del nombre de host, activación o desactivación de TCP BBR, y limpieza del espacio en disco.
