---
title: Línea de comandos
description: Todos los comandos de nefo y sus opciones (info, reset-password, update, setup, unlock-files, decrypt-backup), el archivo de configuración y cómo administrar el servicio nefo con systemctl y journalctl.
---

# Línea de comandos

El programa `nefo`, en `/usr/local/bin/nefo`, ejecuta el panel y tiene comandos para el usuario root del servidor: `nefo info` muestra la dirección del panel, `nefo reset-password` define una contraseña nueva (y puede desactivar la 2FA), `nefo update` instala la última versión y `nefo setup` instala el software. Ejecuta todos los comandos como root.

Esta página enumera los comandos y luego muestra cómo administrar el servicio `nefo` y qué contiene el archivo de configuración.

## Comandos {#commands}

| Comando | Qué hace |
|---|---|
| `nefo info` | Muestra la URL del panel |
| `nefo reset-password` | Define una contraseña aleatoria nueva y cierra todas las sesiones |
| `nefo update` | Instala la última versión y reinicia el panel |
| `nefo setup` | Instala un servidor web, versiones de PHP, un servidor de bases de datos y extras |
| `nefo unlock-files` | Devuelve sus archivos a los sitios web bloqueados (antes de quitar el panel) |
| `nefo decrypt-backup` | Descifra una copia de seguridad que cifró el almacenamiento remoto |
| `nefo version` | Muestra la versión |
| `nefo init` | Crea el archivo de configuración y la primera cuenta de administrador |
| `nefo serve` | Ejecuta el panel (systemd lo hace por ti) |
| `nefo help` | Muestra la lista de comandos |

Ejecuta `nefo <command> -h` para ver las opciones de un comando. Las opciones funcionan con uno o dos guiones: `-port` y `--port` son lo mismo.

Todos los comandos, salvo `version`, `help` y `decrypt-backup`, leen el archivo de configuración `/etc/nefo/config.json`. Para usar otro archivo, agrega `--config /path/to/config.json`, o define la variable de entorno `NEFO_CONFIG`.

## nefo info {#nefo-info}

Muestra la URL del panel y dónde están sus archivos.

```sh
nefo info
```

```
URL:    https://panel.example.com:28443/k3x9p2qa7m1z/
URL:    https://203.0.113.10:28443/k3x9p2qa7m1z/
Config: /etc/nefo/config.json
Data:   /var/lib/nefo
```

La primera línea aparece cuando el panel tiene su propio dominio (**Configuración** → **Certificado**). La segunda usa la primera dirección IP del servidor. Si tu proveedor le da al servidor una dirección privada y otra pública aparte, indica la pública:

```sh
nefo info --host 203.0.113.10
```

| Opción | Qué hace |
|---|---|
| `--host` | El nombre de host o la dirección IP que se muestra en la URL |
| `--port` | Muestra solo el número de puerto, para scripts |
| `--config` | El archivo de configuración que se lee |

## nefo reset-password {#nefo-reset-password}

Define una contraseña aleatoria nueva para la cuenta, cierra su sesión en todos los dispositivos y muestra la contraseña. Úsalo cuando hayas olvidado la contraseña.

```sh
nefo reset-password
```

```
New password for admin: ••••••••••••••••••••
```

Si además perdiste el teléfono con tu aplicación de autenticación, agrega `--disable-2fa`. Desactiva la autenticación de dos factores de la cuenta:

```sh
nefo reset-password --disable-2fa
```

```
New password for admin: ••••••••••••••••••••
Two-factor authentication is off. Turn it on again after signing in.
```

| Opción | Qué hace |
|---|---|
| `--disable-2fa` | Desactiva además la autenticación de dos factores |
| `--user` | El nombre de la cuenta (predeterminado: `admin`, la única cuenta del panel) |
| `--config` | El archivo de configuración que se lee |

El panel sigue funcionando: no hace falta reiniciarlo. El restablecimiento se anota en el registro de actividad.

## nefo update {#nefo-update}

Consulta las versiones del proyecto en GitHub. Si hay una versión más reciente, la descarga, comprueba su firma y su suma de verificación, reemplaza el programa y reinicia el panel.

```sh
nefo update
```

Para ver solamente si hay una actualización disponible:

```sh
nefo update --check
```

```
Installed: v0.1.0
Latest:    v0.2.0
```

| Opción | Qué hace |
|---|---|
| `--check` | Solo muestra la versión instalada y la última |
| `--config` | El archivo de configuración que se lee |

Consulta [Actualizar](./update) para saber qué comprueba la actualización y qué hacer si falla.

## nefo setup {#nefo-setup}

Configura el servidor como la página de configuración inicial del panel (consulta [Primeros pasos](./first-steps#set-up-the-server)): instala un servidor web, versiones de PHP, un servidor de bases de datos y extras, uno tras otro. Las partes que ya están instaladas se omiten, así que puedes volver a ejecutarlo después de un fallo. Cuando termina, el panel ya no abre su página de configuración inicial después del inicio de sesión.

```sh
nefo setup --web nginx --php 8.3,8.4 --db mariadb --redis
```

```
This server: 1 CPU, 960 MB of RAM (250 MB in use), 19.5 GB of free disk.

What the setup installs, one after the other:
  1. Nginx and PHP 8.3  about 40 MB of RAM, 55 MB of disk
  2. PHP 8.4            about 25 MB of RAM, 50 MB of disk, from packages.sury.org
  3. MariaDB            about 228 MB of RAM, 300 MB of disk
  4. Redis              up to 64 MB of RAM, 10 MB of disk
In all: about 357 MB more RAM and 415 MB of disk.

Install? [y/N]
```

Cuando respondes `y`, muestra el registro de cada instalación. Si una falla, se detiene ahí, dice qué ya está instalado y termina con un error. Sin `--web`, `--php`, `--db` ni ningún extra, ofrece la selección recomendada para la RAM del servidor, igual que la página de configuración inicial.

Mientras el panel está en ejecución, `nefo setup` le pide al panel que haga las instalaciones, como una de sus tareas: esperan a que termine cualquier otra tarea, se ven en el panel y continúan aunque cierres la sesión SSH. Se comunica con el panel a través de un socket en la carpeta de datos que solo root puede abrir. Cuando el panel no está en ejecución, `nefo setup` hace las instalaciones por su cuenta.

| Opción | Qué hace |
|---|---|
| `--web` | `nginx`, `apache`, `openlitespeed` o `none`. El servidor web viene con la versión de PHP de tu distribución |
| `--php` | Versiones de PHP separadas por comas, como `8.3,8.4`, o `none` |
| `--db` | `mariadb`, `mysql` o `none`; `mariadb-11.4`, `mariadb-11.8` o `mariadb-12.3` para una serie LTS del repositorio de MariaDB, donde el servidor la ofrezca (consulta [Versiones de MariaDB](./databases#mariadb-versions)). `--mariadb` es lo mismo que `--db mariadb` |
| `--redis`, `--node`, `--python`, `--docker` | Instala además Redis, Node.js (la versión LTS más reciente), Python para sitios web de aplicaciones, o Docker |
| `--adminer`, `--waf` | Instala además Adminer o el firewall web. Ambos necesitan un servidor web |
| `--clamav` | Instala además ClamAV, en servidores con 2 GB de RAM o más |
| `--postgresql`, `--memcached` | Instala además PostgreSQL o Memcached, donde la página Software los ofrezca |
| `--yes`, `-y` | Instala sin preguntar. Es necesario cuando no hay una terminal conectada, como en un script |
| `--dry-run` | Solo muestra lo que se instalaría |
| `--skip` | No instala nada: el panel ya no abre su página de configuración inicial después del inicio de sesión |
| `--wait` | Cuánto esperar a que el panel se inicie, como `60s`. Lo usa el instalador |
| `--check` | Solo comprueba las opciones. El instalador lo usa antes de configurar nada |
| `--config` | El archivo de configuración que se lee |

El instalador acepta las mismas opciones, para configurar el servidor en la misma ejecución (consulta [Instalar](./install#set-the-server-up-in-the-same-run)).

## nefo unlock-files {#nefo-unlock-files}

Devuelve sus archivos a cada sitio web que tiene los archivos bloqueados (**Sitios web** → (sitio) → **Bloqueo de archivos**, consulta [Bloqueo por país y bloqueo de archivos](./website-protection#lock-a-website-s-files)), con exactamente el propietario, el grupo y los permisos que tenía cada uno antes del bloqueo, y quita los grupos de bloqueo de los sitios web. Ejecútalo como root, con el panel detenido, antes de quitar Nefo (consulta [Desinstalar](./uninstall#unlock-the-websites-files)).

```sh
systemctl stop nefo
nefo unlock-files
```

```
Unlocked the files of example.com
Unlocked the files of blog.example.com
```

Cuando ningún sitio web está bloqueado, muestra `No website's files are locked.`

| Opción | Qué hace |
|---|---|
| `--config` | El archivo de configuración que se lee |

## nefo decrypt-backup {#nefo-decrypt-backup}

Descifra una copia de seguridad que cifró un almacenamiento remoto (un archivo `.tar.gz.enc`), por ejemplo una que descargaste de Google Drive. Solo hace falta la frase de contraseña: funciona en cualquier servidor, también en uno donde el panel no está configurado, y no lee el archivo de configuración. Consulta [Restaurar en un servidor nuevo](./restore-new-server#decrypt-a-copy-by-hand).

```sh
nefo decrypt-backup example.com_20261126-030000.tar.gz.enc
```

```
Passphrase:
Decrypted to example.com_20261126-030000.tar.gz
SHA-256: 3f1c…
```

La frase de contraseña nunca se da en la línea de comandos, donde otros usuarios podrían verla. En una terminal se pide sin mostrarla; si no, es la primera línea de la entrada estándar o de `-passphrase-file`.

| Opción | Qué hace |
|---|---|
| `-o` | El archivo de salida. El predeterminado es el nombre de la entrada sin `.enc`. `-o -` escribe en la salida estándar, por ejemplo `nefo decrypt-backup -o - FILE.enc \| tar xz` |
| `-passphrase-file` | Lee la frase de contraseña de la primera línea de este archivo |
| `-force` | Reemplaza el archivo de salida si existe |

La salida se escribe en un archivo temporal y solo se le cambia el nombre cuando se descifró toda la copia, así que una frase de contraseña incorrecta o un archivo dañado no dejan nada (salvo con `-o -`, donde el final del archivo se comprueba al final). Un archivo que no está cifrado se copia tal cual, con un aviso. El SHA-256 que muestra es el que el panel registró para la copia de seguridad.

## nefo version {#nefo-version}

Muestra la versión y el commit a partir del cual se compiló. `nefo --version` y `nefo -v` hacen lo mismo.

```sh
nefo version
```

```
Nefo v0.1.0 (commit 1a2b3c4)
```

## nefo init {#nefo-init}

Crea el archivo de configuración y la primera cuenta de administrador. El instalador lo ejecuta por ti, así que normalmente no lo necesitas. Se niega a sobrescribir un archivo de configuración existente.

Elige un puerto aleatorio entre 20000 y 49999 y una ruta secreta aleatoria, activa HTTPS y crea la cuenta `admin` con una contraseña aleatoria. Si la carpeta de datos ya tiene cuentas, las conserva, solo escribe el archivo de configuración nuevo y no muestra ninguna contraseña.

| Opción | Qué hace |
|---|---|
| `--config` | Dónde escribir el archivo de configuración (predeterminado: `/etc/nefo/config.json`) |
| `--data` | La carpeta de datos (predeterminado: `/var/lib/nefo`) |
| `--listen` | La dirección en la que escuchar, como `:28443`, en lugar de un puerto aleatorio |
| `--host` | El nombre de host o la dirección IP que se muestra en la URL impresa |
| `--dev` | Modo de desarrollo: HTTP simple en `127.0.0.1:8080` con la ruta `dev`. Nunca lo uses en un servidor. |

## nefo serve {#nefo-serve}

Ejecuta el panel. El servicio `nefo` lo inicia con `nefo serve --config /etc/nefo/config.json`, así que no lo ejecutas tú. Una segunda copia fallaría, porque el servicio ya usa el puerto.

## Administrar el servicio nefo {#manage-the-nefo-service}

| Comando | Qué hace |
|---|---|
| `systemctl status nefo` | Muestra si el panel está en ejecución y sus últimas líneas de registro |
| `systemctl restart nefo` | Reinicia el panel |
| `systemctl stop nefo` | Detiene el panel |
| `systemctl start nefo` | Inicia el panel |
| `journalctl -u nefo` | Muestra el registro del panel |
| `journalctl -u nefo -f` | Sigue el registro a medida que llegan líneas nuevas |
| `journalctl -u nefo -n 100 --no-pager` | Muestra las últimas 100 líneas |
| `journalctl -u nefo --since "1 hour ago"` | Muestra el registro de la última hora |

Reiniciar el panel no detiene tus sitios web, bases de datos ni otros servicios. También puedes reiniciarlo desde **Configuración** → **Panel** → **Energía** → **Reiniciar el panel**.

Cuando el panel se detiene, primero deja terminar una tarea en curso, como la instalación de un software, durante un máximo de 75 segundos. Si el panel se detiene por un error, systemd lo vuelve a iniciar al cabo de 3 segundos.

El archivo del servicio es `/etc/systemd/system/nefo.service`:

```ini
[Unit]
Description=Nefo server control panel
Documentation=https://nefo.io
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/nefo serve --config /etc/nefo/config.json
Restart=on-failure
RestartSec=3
UMask=0077
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

El panel se ejecuta como root, porque administra todo el servidor.

## El archivo de configuración {#the-configuration-file}

Solo root puede leer `/etc/nefo/config.json`. Lo escribe el instalador, por ejemplo:

```json
{
  "listen": ":28443",
  "entrance": "k3x9p2qa7m1z",
  "data_dir": "/var/lib/nefo",
  "tls": {
    "enabled": true
  }
}
```

| Campo | Qué significa |
|---|---|
| `listen` | La dirección y el puerto en los que escucha el panel. `:28443` significa el puerto 28443 en todas las direcciones. |
| `entrance` | La ruta secreta. De 8 a 32 letras minúsculas o dígitos. Cualquier otra ruta responde `404`. |
| `data_dir` | La carpeta de datos: la base de datos del panel `nefo.db`, los certificados y las claves. |
| `tls.enabled` | HTTPS para el panel. Mantenlo activado. |
| `tls.cert`, `tls.key` | Rutas opcionales a tu propio certificado y clave para el panel. Tienen prioridad sobre el certificado de Let's Encrypt del panel y sobre el autofirmado. |
| `acme_directory` | Autoridad de certificación opcional para los certificados. Vacío significa Let's Encrypt. Define `https://acme-staging-v02.api.letsencrypt.org/directory` mientras haces pruebas: los navegadores no confían en los certificados de staging, pero los intentos fallidos no cuentan para los límites de Let's Encrypt. |
| `update_url` | Dirección opcional desde la que se descargan las actualizaciones. Vacío significa las versiones del proyecto en GitHub. Debe empezar por `https://`. |
| `dev` | Modo de desarrollo. Nunca lo actives en un servidor. |

Después de editar el archivo, reinicia el panel con `systemctl restart nefo`. Si el archivo tiene un error, el panel no se inicia: `journalctl -u nefo` muestra el motivo.

::: warning Cambiar el puerto
Si cambias el puerto en `listen`, abre primero el puerto nuevo, tanto en ufw (por ejemplo `ufw allow 31234/tcp`) como en el firewall en la nube de tu proveedor. Si no, no podrás acceder al panel después del reinicio.
:::
