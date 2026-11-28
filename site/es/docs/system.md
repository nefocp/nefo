---
title: Herramientas del sistema
description: El panel principal, las tareas cron, los servicios, los programas en segundo plano, los procesos y los puertos, los inicios de sesión SSH, la terminal web, los registros, y cómo reiniciar el panel o el servidor.
---

# Herramientas del sistema

El **Panel principal** muestra en vivo la CPU, la memoria, el disco y la carga del servidor, con 7 días de historial, y enumera lo que requiere atención. Las herramientas de servidor de Nefo también administran las tareas cron, los servicios de systemd, los programas en segundo plano que deben seguir ejecutándose, los procesos y los puertos en escucha, los inicios de sesión SSH y una terminal de root en el navegador, y reinician el panel o el servidor.

Esta página trata de cada una de estas herramientas y de dónde encontrar los registros. La mayoría están en el grupo **Servidor** del menú de la izquierda. Las actualizaciones del sistema operativo, la swap, la zona horaria y el nombre de host, TCP BBR y la limpieza del disco están en la página [Sistema](./os).

## Panel principal {#dashboard}

El **Panel principal** se abre después de iniciar sesión.

- La parte superior de la página muestra el nombre del servidor, el sistema operativo, el tiempo en funcionamiento, el número de CPU y la memoria. En un VPS que es un contenedor (LXC), la memoria es el límite del contenedor, marcado como "límite del contenedor" (consulta [Un VPS que es un contenedor](./first-steps#a-vps-that-is-a-container)). Los botones de allí te permiten **Buscar actualizaciones**, abrir la **Terminal** y usar el menú **Energía**.
- Cuatro indicadores muestran en vivo la **Carga del sistema**, la CPU, la **Memoria** y el **Disco /**. Se actualizan cada 5 segundos. La carga del sistema es la carga media de 1 minuto dividida por el número de CPU.
- Unos mosaicos cuentan los sitios web, las bases de datos, las cuentas SFTP, las tareas cron, las copias de seguridad y los riesgos de seguridad. Selecciona un mosaico para abrir su página. Cuando no hay ningún problema grave abierto, el mosaico **Riesgos de seguridad** muestra cuándo se ejecutó por última vez la [revisión de seguridad](./security-check).
- **Historial de recursos** dibuja un gráfico de CPU, RAM, Disco, Carga o Red, en 1 hora, 24 horas o 7 días. Debajo del gráfico están los valores **Ahora**, **Promedio** y **Máximo**. Para la red, muestra las tasas de **Entrada** y **Salida** y los totales del intervalo.
- **Requiere atención** enumera los problemas que necesitan tu intervención, de más grave a menos grave.
- **Software instalado** muestra los componentes principales y su estado.

El panel anota la carga cada minuto y conserva 7 días de historial. Después de una instalación nueva, los gráficos aparecen al cabo de unos minutos. La parte inferior del menú de la izquierda también muestra el uso actual de CPU y RAM.

### ¿Qué comprueba "Requiere atención"? {#what-needs-attention-checks}

- La autenticación de dos factores está desactivada.
- root puede iniciar sesión por SSH con contraseña.
- El firewall está desactivado.
- El certificado de un sitio web no se pudo renovar, caducó o caduca en menos de 14 días.
- Un sitio web vence en menos de 7 días.
- Falló una copia de seguridad programada.
- El disco está lleno al 90% o más.
- Hay actualizaciones de seguridad del sistema operativo esperando (consulta [Actualizaciones](./os#updates)).
- El servidor necesita reiniciarse para terminar de instalar las actualizaciones.
- La última [revisión de seguridad](./security-check) encontró problemas críticos o avisos.
- Un sitio web monitoreado está caído (consulta [Monitoreo](./websites#monitoring)).

La campana de arriba a la derecha (**Notificaciones**) muestra la misma lista. Ambas solo se ven mientras tienes la sesión iniciada. Para recibir en cambio un mensaje por Telegram, correo electrónico, Discord, Slack o un webhook, configura las [alertas](./mail-alerts).

## Tareas cron {#cron-jobs}

Las tareas cron ejecutan comandos según una programación. Ve a **Tareas cron** en el menú de la izquierda.

### Agregar una tarea cron {#add-a-cron-job}

1. Selecciona **Agregar tarea cron**.
2. Escribe un **Nombre**, de hasta 100 caracteres.
3. En **Ejecutar como**, elige como quién se ejecuta: root, o el usuario de un sitio web (que aparece con su dominio). Ejecuta las tareas de un sitio web como su propio usuario, y usa root solo para las tareas del servidor.
4. Define la **Programación**. Elige una programación habitual (**Cada 5 minutos**, **Cada hora**, **Todos los días a las 03:00**, **Todos los domingos a las 04:00**, **Al iniciarse el servidor**) o escribe la tuya. Debajo del campo, el panel describe la programación con palabras.
5. Escribe el **Comando**: un comando de shell en una línea, de hasta 2000 caracteres, que se ejecuta con `/bin/sh`.
6. Deja **Activada** activado y selecciona **Guardar**.

Para el usuario de un sitio web con PHP, el enlace **Usar el comando cron de WordPress** completa el cron de WordPress, como `php8.3 /var/www/example.com/public/wp-cron.php`, cada 5 minutos.

### Sintaxis de la programación {#schedule-syntax}

Una programación tiene cinco campos, en hora del servidor:

| Campo | Valores |
|---|---|
| minuto | 0–59 |
| hora | 0–23 |
| día del mes | 1–31 |
| mes | 1–12 |
| día de la semana | 0–7 (0 y 7 son domingo) |

En cada campo, usa `*` para todos los valores, una lista como `1,15`, un rango como `1-5`, o un paso como `*/10`. También puedes usar uno de estos en lugar de los cinco campos: `@hourly`, `@daily` (o `@midnight`), `@weekly`, `@monthly`, `@yearly` (o `@annually`) y `@reboot`. El panel rechaza los números fuera de rango, como el minuto 60.

| Programación | Se ejecuta |
|---|---|
| `*/5 * * * *` | cada 5 minutos |
| `0 3 * * *` | todos los días a las 03:00 |
| `30 2 * * 1` | todos los lunes a las 02:30 |
| `0 0 1 * *` | a medianoche el día 1 de cada mes |

### Cómo se ejecutan las tareas cron {#how-cron-jobs-run}

- El panel escribe las tareas en `/etc/cron.d/panel`, así que el cron del sistema las ejecuta aunque el panel esté detenido. No edites ese archivo: el panel lo reescribe.
- La salida y los errores del comando se agregan al registro de la tarea, `/var/log/panel-cron/<id>.log`, que pertenece al usuario de la tarea.
- Un `%` en el comando funciona tal como lo escribes: el panel lo escapa para cron.
- Una tarea deja de ejecutarse cuando su usuario ya no existe, por ejemplo después de eliminar el sitio web.

### Ejecutar, revisar y cambiar una tarea {#run-check-and-change-a-job}

- El interruptor de la columna **Activada** activa o desactiva una tarea. Una tarea desactivada se queda en la lista.
- **Ejecutar ahora** ejecuta la tarea de inmediato, como su usuario, y muestra la salida en vivo. Se detiene al cabo de una hora.
- **Registros**, en el menú ⋯ de la fila, muestra el final del registro de la tarea. Los registros de más de 5 MB se recortan a su último 1 MB.
- **Editar** y **Eliminar** están en el mismo menú. Eliminar una tarea también elimina su registro.

Encima de la lista, muestra **Todas** las tareas o solo las **Activadas** o **Desactivadas**, y busca por nombre, comando o usuario.

## Servicios {#services}

**Servicios** enumera los servicios del sistema que administra Nefo, si están instalados:

| Servicio | Unidad de systemd |
|---|---|
| Nefo | `nefo` |
| Nginx, Apache u OpenLiteSpeed | `nginx`, `apache2`, `lshttpd` |
| PHP-FPM, uno por cada versión instalada | como `php8.3-fpm` |
| MariaDB o MySQL | `mariadb` o `mysql` |
| PostgreSQL | como `postgresql@16-main` |
| Redis | `redis-server` |
| Memcached | `memcached` |
| fail2ban | `fail2ban` |
| SSH | `ssh` |
| Cron | `cron` |
| Firewall (ufw) | `ufw` |

Los demás servicios, como Docker o Postfix, no aparecen. Cada fila muestra el estado (**En ejecución**, **Detenido**, **Fallido**…), cuánto tiempo lleva en ejecución el servicio, "no se inicia al arrancar" cuando corresponde, y su uso de memoria.

Las acciones dependen del servicio:

| Servicio | Acciones |
|---|---|
| Servidor web, PHP-FPM | **Iniciar**, **Detener**, **Reiniciar**, **Recargar** |
| Nefo | **Reiniciar** |
| SSH | **Reiniciar**, **Recargar** |
| Los demás | **Iniciar**, **Detener**, **Reiniciar** |

- **Detener** te pide confirmación, porque los sitios web que necesitan el servicio dejan de funcionar.
- Antes de reiniciar o recargar el servidor web, el panel prueba su configuración. Si la prueba falla, no se reinicia nada y se muestra el error.
- SSH no se puede detener desde el panel, así que no puedes quedarte fuera.
- Reiniciar Nefo recarga la página al cabo de unos segundos.
- **Registros** muestra las últimas 150 líneas del diario del servicio.

El panel no cambia si un servicio se inicia al arrancar.

## Programas en segundo plano {#background-programs}

Los programas en segundo plano son comandos que deben seguir ejecutándose, como un worker de colas. El panel ejecuta cada uno como un servicio de systemd, como el usuario de Linux de un sitio web, y lo vuelve a iniciar cuando se detiene. Primero necesitas al menos un sitio web.

### Agregar un programa {#add-a-program}

1. Ve a **Programas en 2.º plano** en el menú de la izquierda y selecciona **Agregar programa**.
2. Escribe un **Nombre**, de hasta 60 caracteres.
3. Elige el **Sitio web**. El programa se ejecuta como el usuario de Linux de este sitio web, nunca como root.
4. Opcionalmente, define la **Carpeta de trabajo**: una carpeta dentro de la carpeta del sitio web, como `app`. Déjala vacía para usar la propia carpeta del sitio web.
5. Escribe el **Comando**: un programa con sus argumentos, en una línea, de hasta 1000 caracteres.
6. Opcionalmente, agrega **Variables de entorno**: una `NAME=value` por línea, hasta 20. En el servidor, solo root puede leerlas.
7. Deja activado **Iniciar ahora y al iniciarse el servidor** y selecciona **Crear**.

Para un sitio web PHP, el enlace **Usar el worker de colas de Laravel** completa `php8.3 artisan queue:work --sleep=3 --tries=3 --max-time=3600`, con la versión de PHP del sitio web.

### Cómo se ejecutan los programas en segundo plano {#how-background-programs-run}

- Cada programa es un servicio de systemd llamado `panel-daemon-<id>.service`.
- Cuando termina, por cualquier motivo, systemd lo vuelve a iniciar al cabo de 3 segundos.
- Al arrancar, se inicia después de MariaDB y Redis.
- Se ejecuta con las protecciones de systemd: `/usr`, `/boot` y `/etc` son de solo lectura, `/home` y `/root` están ocultos, y tiene su propio `/tmp`.
- La carpeta de trabajo debe existir y quedarse dentro de la carpeta del sitio web, también a través de enlaces simbólicos.
- Su salida va al diario del sistema.

### Controlar un programa {#control-a-program}

Cada fila muestra el estado (en ejecución, reiniciándose, fallido, deteniéndose o detenido), cuántas veces se reinició y su uso de memoria.

- La fila muestra como botón **Iniciar** (cuando el programa está detenido) o **Reiniciar** (cuando se está ejecutando). Las demás acciones, incluida **Detener**, están en el menú ⋯ de la fila. **Detener** mantiene el programa detenido, también después de un reinicio, hasta que lo vuelvas a iniciar.
- **Registros** muestra las últimas 200 líneas que escribió el programa.
- **Editar** cambia el programa. **Guardar** lo reinicia con los ajustes nuevos.
- **Eliminar** lo detiene y lo quita.

Cuando eliminas un sitio web, sus programas en segundo plano también se detienen y se quitan.

## Procesos y puertos {#processes-and-ports}

**Procesos** muestra lo que se ejecuta en el servidor, en tres pestañas: **Procesos**, **Puertos** e **Inicios de sesión SSH**.

### Procesos {#processes}

La lista muestra los procesos más activos, hasta 300. Ordena por PID, % de CPU o memoria con los encabezados de las columnas, y filtra por nombre, usuario o PID. **Actualizar cada 5 segundos** mantiene la lista al día.

El % de CPU se mide durante los últimos segundos: 100% es un núcleo de CPU trabajando todo el tiempo.

Para detener un proceso, selecciona el botón de detener al final de su fila, y luego:

- **Detener** le pide al programa que termine y salga (señal TERM). Los programas bien hechos guardan antes sus datos.
- **Forzar detención** lo termina de inmediato (señal KILL). Úsalo solo cuando **Detener** no funcione: se pierden los datos sin guardar.

Un servicio que ejecuta systemd puede volver a ser iniciado por systemd. El panel no puede detener el proceso init del sistema (PID 1), el propio panel, el proceso que inició el panel ni los hilos del kernel. Cada detención se anota en el registro de actividad.

### Puertos {#ports}

**Puertos** enumera los puertos TCP y UDP en los que escuchan los programas, con la **Dirección**, el **Programa** y desde dónde se llega a él, en **Accesible desde**: **solo este servidor** o **la red**.

Un puerto abierto a la red todavía puede estar cerrado hacia el exterior por el firewall. Consulta [Firewall](./security#firewall-ufw).

## Inicios de sesión SSH {#ssh-sign-ins}

**Procesos** → **Inicios de sesión SSH** enumera los inicios de sesión SSH y los intentos fallidos de los últimos 30 días, de los más recientes a los más antiguos, hasta 200. Cada fila muestra la hora, el resultado (**sesión iniciada**, **contraseña incorrecta** o **usuario desconocido**), el usuario, la dirección IP y el método, como `publickey` o `password`.

Muestra **Todos**, **Solo correctos** o **Solo fallidos**. La lista se lee del diario del servicio SSH.

Muchos intentos fallidos suelen significar que alguien está intentando adivinar contraseñas. Desactiva el inicio de sesión con contraseña, o instala fail2ban para bloquear esas direcciones. Consulta [Seguridad](./security#brute-force-protection-fail2ban).

## Terminal {#terminal}

**Terminal** abre una shell de root (bash) en el servidor, en tu navegador.

::: warning
La terminal es como una sesión SSH como root: cada comando se ejecuta con todos los permisos, y un comando equivocado puede eliminar datos o detener el servidor. El panel no puede deshacer lo que ejecutes allí.
:::

- La sesión termina cuando sales de la página, o tras 30 minutos sin actividad. Selecciona **Volver a conectar** para abrir una nueva.
- Puede haber hasta 4 terminales abiertas a la vez, incluidas las terminales de contenedores.
- El registro de actividad anota cuándo se abrió y se cerró una terminal, y durante cuánto tiempo. No anota los comandos.

## Registros {#logs}

Nefo no tiene una página única de registros. Cada registro se muestra junto a lo que le corresponde:

| Registro | Dónde |
|---|---|
| El registro de acceso, el registro de errores, los errores de PHP y los registros de la aplicación de un sitio web | **Sitios web** → (sitio) → **Registros** |
| Los eventos del firewall de un sitio web | **Sitios web** → (sitio) → **Firewall web (WAF)** |
| La salida de una tarea cron | **Tareas cron** → **Registros** |
| El diario de un servicio | **Servicios** → **Registros** |
| La salida de un programa en segundo plano | **Programas en 2.º plano** → **Registros** |
| Los contenedores y los proyectos de Compose | **Docker** |
| Lo que se hizo en el panel | **Cuenta** → **Actividad reciente** |
| El registro del servicio del propio panel | `journalctl -u nefo` en el servidor |

Los registros de acceso, de errores y de PHP de un sitio web muestran sus últimas 300 líneas. Selecciona **Actualizar** en la ventana del registro para cargar las líneas nuevas.

## Reiniciar el panel o el servidor {#restart-the-panel-or-reboot-the-server}

Ve a **Configuración** → **Panel** → **Energía**. El **Panel principal** tiene las mismas acciones en su menú **Energía**.

- **Reiniciar el panel** solo reinicia el panel. Los sitios web, las bases de datos y los demás servicios siguen funcionando, y el panel vuelve en unos segundos.
- **Reiniciar el servidor** reinicia todo el servidor. Los sitios web y las bases de datos no funcionan hasta que vuelve, normalmente de uno a dos minutos. Escribe el nombre del servidor para confirmar.

En ambos casos, la página se recarga sola cuando el panel vuelve a responder. Ambos se rechazan mientras se ejecuta una tarea en segundo plano, como una instalación o una copia de seguridad: espera a que termine. Ambos se anotan en el registro de actividad.

En el servidor, también puedes reiniciar el panel con:

```sh
systemctl restart nefo
```
