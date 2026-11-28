---
title: Requisitos
description: Los sistemas operativos, tipos de CPU, memoria, puertos y ajustes de firewall que necesita Nefo.
---

# Requisitos

Nefo necesita un servidor de 64 bits, x86-64 o ARM64, con Ubuntu 22.04, 24.04 o 26.04 o Debian 12 o 13, acceso root y systemd. Está hecho para un VPS con 1 GB de RAM: el panel en sí usa unos 34 MB de memoria, medidos en un servidor de prueba después de actualizar a la v0.24.0.

Esta página enumera lo que necesita tu servidor antes de [instalar Nefo](./install): el sistema operativo, el tipo de CPU, la memoria y el disco, y los puertos de red.

## Sistema operativo {#operating-system}

El instalador admite estos sistemas:

| Distribución | Versiones |
|---|---|
| Ubuntu | 22.04, 24.04, 26.04 |
| Debian | 12, 13 |

El instalador se ha probado en servidores recién instalados con cada uno de estos sistemas, en x86-64. La versión ARM64 todavía no se ha probado en hardware ARM real.

En cualquier otro sistema, el instalador se detiene con el aviso de que el sistema todavía no es compatible: "is not supported yet". Puedes definir `NEFO_FORCE=1` para intentarlo de todos modos (consulta [Instalar](./install#environment-variables)), pero la gestión de sitios web necesita `apt-get`, `systemctl` y `useradd`. Sin ellos, la página **Sitios web** muestra "La gestión de sitios web necesita un servidor Debian o Ubuntu con systemd, así que no está disponible en esta máquina."

El instalador también necesita:

- acceso root (inicia sesión como root o usa `sudo`);
- systemd;
- `curl` y `sha256sum`;
- `openssl`, para comprobar la firma de una versión.

Si falta `curl`, instálalo primero con `apt install curl`.

## CPU {#cpu}

Nefo funciona en x86 de 64 bits (`x86_64`, también llamado amd64) y en ARM de 64 bits (`aarch64`, también llamado arm64). En cualquier otra CPU, el instalador se detiene con "unsupported CPU architecture" (la arquitectura de la CPU no es compatible).

## Memoria y disco {#memory-and-disk}

Nefo está hecho para VPS pequeños con 1 GB de RAM. El panel en sí es un solo programa de unos 25 MB y usa unos 34 MB de memoria (medidos en un servidor de prueba después de actualizar a la v0.24.0). Solo instalas los componentes que necesitas. La página **Software** muestra aproximadamente cuánta memoria usa cada componente. Por ejemplo, los servidores web usan unos 15 MB (Nginx), 26 MB (OpenLiteSpeed) o 40 MB (Apache) en reposo.

En un VPS que es un contenedor (LXC, OpenVZ), Nefo cuenta el límite de memoria del propio contenedor, no la memoria del host (consulta [Un VPS que es un contenedor](./first-steps#a-vps-that-is-a-container)).

Un componente necesita mucha más memoria: el antivirus opcional ClamAV necesita alrededor de 1.4 GB de memoria libre mientras analiza, así que es adecuado para servidores con 2 GB o más.

En un servidor con 1 GB de memoria, agrega de 1 a 2 GB de swap, para que un momento de mucha carga no haga que el kernel detenga MariaDB o PHP. Muchas imágenes de VPS vienen sin swap. **Sistema** → **Swap** crea un archivo de swap con un clic (consulta [Swap](./os#swap)). En un contenedor (OpenVZ, LXC), el kernel no permite swap: consulta a tu proveedor.

El espacio en disco que necesitas depende de tus sitios web, bases de datos y copias de seguridad. Las copias de seguridad que se guardan en el servidor se almacenan en `/var/backups/panel`, en el mismo disco que tus sitios web. El panel principal muestra el uso del disco, y el panel te avisa cuando el disco está lleno al 90%.

## Un servidor recién instalado {#a-fresh-server}

Instala Nefo en un servidor recién instalado. El instalador no comprueba qué más hay instalado, pero el propio panel configura el servidor web, PHP, MariaDB, el firewall y SSH:

- Ejecuta un solo servidor web (Nginx, Apache u OpenLiteSpeed). Se niega a instalar uno mientras haya otro servidor web en el servidor, porque ambos necesitan los puertos 80 y 443.
- Escribe su propia configuración para el software que administra. No instales Nefo junto a otro panel de control.

El instalador se niega a ejecutarse por segunda vez en el mismo servidor: cuando existe `/etc/nefo/config.json`, se detiene con "the panel is already installed" (el panel ya está instalado).

## Puertos {#ports}

| Puerto | Lo usa | Cuándo |
|---|---|---|
| Un puerto TCP aleatorio entre 20000 y 49999 | El panel | Desde la instalación. El instalador lo muestra. |
| 80 y 443 (TCP) | El servidor web | Cuando instalas un servidor web desde el panel |
| 443 (UDP) | HTTP/3 en OpenLiteSpeed | Cuando OpenLiteSpeed es el servidor web |
| Tu puerto SSH (22 de forma predeterminada) | SSH | Siempre |
| 3306 (TCP) | MariaDB o MySQL | Solo si permites el acceso remoto a una base de datos, y solo para las direcciones que indiques |
| 5432 (TCP) | PostgreSQL | Solo si permites el acceso remoto a una base de datos PostgreSQL, y solo para las direcciones que indiques |

Algunos servicios solo escuchan en el propio servidor (127.0.0.1), así que no los abres: el receptor de los webhooks de push de Git (puerto 7809, al que se llega a través del servidor web), los puertos de los contenedores Docker y el relay de correo local.

## Firewalls {#firewalls}

### El firewall del servidor (ufw) {#the-firewall-on-the-server-ufw}

- Si ufw está activo cuando instalas, el instalador abre en él el puerto del panel (o en firewalld, si es el que se está ejecutando).
- Cuando instalas un servidor web desde el panel y ufw está activo, el panel abre los puertos 80 y 443. El panel no administra firewalld: si el servidor lo usa, abre tú mismo estos puertos.
- La página **Firewall y SSH** administra ufw. Si ufw no está instalado, la página dice "ufw no está instalado en este servidor." Instálalo con `apt install ufw`.

Cuando activas el firewall desde el panel, este mantiene abiertos SSH y el puerto del panel, y los puertos 80 y 443 cuando hay un servidor web instalado. Consulta [Seguridad](./security).

### El firewall en la nube de tu proveedor {#your-provider-s-cloud-firewall}

Muchos proveedores tienen un firewall fuera del servidor, a menudo llamado grupo de seguridad o firewall en la nube. El instalador no puede modificarlo. Permite allí tú mismo lo siguiente:

1. Tu puerto SSH.
2. El puerto TCP del panel, que el instalador muestra al final.
3. Los puertos TCP 80 y 443, cuando alojes sitios web. Let's Encrypt comprueba tus dominios por el puerto 80, así que no se pueden emitir certificados mientras esté cerrado.

## Un nombre de dominio {#a-domain-name}

No necesitas un dominio para instalar Nefo: abres el panel con la dirección IP del servidor. Necesitas uno para cada sitio web, y puedes darle al panel su propio dominio y un certificado gratuito (consulta [Primeros pasos](./first-steps#give-the-panel-its-own-domain)). Apunta el registro A del dominio a la dirección IP del servidor.
