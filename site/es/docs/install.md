---
title: Instalar
description: Instala Nefo en un servidor Ubuntu o Debian con un solo comando y abre el panel por primera vez.
---

# Instalar

Para instalar Nefo, ejecuta un solo comando como root en un servidor Ubuntu 22.04, 24.04 o 26.04 o Debian 12 o 13 recién instalado: `curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | sudo bash`. El instalador comprueba la firma Ed25519 de la versión, instala el panel como el servicio de systemd `nefo` en unos segundos y muestra la dirección del panel y una contraseña de administrador de un solo uso.

Esta página muestra cómo instalar Nefo, qué hace el instalador y cómo abrir el panel por primera vez. Revisa primero los [requisitos](./requirements).

Nefo se instala en Ubuntu 22.04, 24.04 y 26.04 y Debian 12 y 13, en x86-64 o ARM64. El instalador se ha probado en servidores recién instalados con cada uno de estos sistemas, en x86-64. La versión ARM64 todavía no se ha probado en hardware ARM real.

## Ejecutar el instalador {#run-the-installer}

1. Conéctate a tu servidor por SSH.
2. Ejecuta:

   ```sh
   curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | sudo bash
   ```

   Si iniciaste sesión como root y `sudo` no está instalado (algo común en Debian), omítelo:

   ```sh
   curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | bash
   ```

3. Espera a que el instalador muestre la dirección, el nombre de usuario y la contraseña del panel. Normalmente tarda unos segundos.

::: tip Comandos en esta documentación
Todos los demás comandos de esta documentación se ejecutan como root en el servidor, por eso se muestran sin `sudo`.
:::

## Lo que muestra el instalador {#what-the-installer-prints}

El final de la salida se ve así:

```
==> Downloading nefo-linux-amd64 (latest)
==> Verifying the release signature
==> Verifying checksum
==> Creating the configuration and the admin account
==> Installing the systemd service
==> Opening port 28443/tcp in ufw

Nefo is ready.

  URL:      https://203.0.113.10:28443/k3x9p2qa7m1z/
  Username: admin
  Password: ••••••••••••••••••••

Save this password now: it is stored only as a hash and cannot be shown again.
After signing in, change it and turn on two-factor authentication.
The certificate is self-signed, so your browser will warn you the first time.

If your provider has a cloud firewall (security group), allow TCP port 28443 there too.
Manage the service with: systemctl status|restart nefo   Logs: journalctl -u nefo
```

Tu puerto, tu ruta y tu contraseña son distintos: se eligen al azar para cada servidor.

::: warning Guarda la contraseña ahora
La contraseña se muestra una sola vez. El panel solo guarda un hash de ella. Si la pierdes, define una nueva con `nefo reset-password` (consulta [Línea de comandos](./cli#nefo-reset-password)).
:::

La dirección usa la primera dirección IP del servidor. Si tu proveedor le da al servidor una dirección privada y otra pública aparte, reemplaza la dirección de la URL por la IP pública.

## Lo que hace el instalador {#what-the-installer-does}

1. **Comprueba el servidor.** Se detiene si no se ejecuta como root, si falta systemd, `curl` o `sha256sum`, si el sistema operativo o la CPU no son compatibles, o si Nefo ya está instalado (existe `/etc/nefo/config.json`).
2. **Descarga** el programa para tu CPU (`nefo-linux-amd64` o `nefo-linux-arm64`) y la lista de sumas de verificación desde las versiones del proyecto en GitHub.
3. **Comprueba la firma** de la lista de sumas de verificación con la clave Ed25519 del proyecto, que viene integrada en el instalador. Una versión sin una firma válida se rechaza con el aviso "Do not install this file.", es decir, que no instales ese archivo. Un instalador de una compilación de desarrollo no tiene clave, y avisa de que la firma no se comprueba.
4. **Comprueba la suma de verificación SHA-256** de la descarga. Si no coincide, la instalación se detiene.
5. **Instala** el programa como `/usr/local/bin/nefo` y crea `/etc/nefo` y `/var/lib/nefo`, que solo puede leer root.
6. **Crea la configuración** en `/etc/nefo/config.json`: un puerto aleatorio entre 20000 y 49999, una ruta secreta aleatoria de 12 caracteres y HTTPS activado. También crea la cuenta `admin` con una contraseña aleatoria de 20 caracteres.
7. **Instala el servicio de systemd** `nefo`, lo inicia y lo habilita al arrancar.
8. **Abre el puerto del panel** en ufw si ufw está activo, o en firewalld si es el que se está ejecutando. Si ninguno está activo, no modifica ningún firewall.

A menos que le pases [opciones de configuración inicial](#set-the-server-up-in-the-same-run), el instalador no instala nada más: ni servidor web, ni PHP, ni base de datos. El panel ofrece configurarlos la primera vez que inicias sesión (consulta [Primeros pasos](./first-steps#set-up-the-server)).

## Abrir el panel por primera vez {#open-the-panel-for-the-first-time}

1. Abre en tu navegador la URL del instalador, exactamente como se mostró. Empieza por `https://` y termina con la ruta secreta y una barra.
2. Tu navegador avisa de que la conexión no es privada o de que el certificado no es de confianza. Es lo esperado: el panel creó su propio certificado autofirmado en el primer arranque. Elige la opción para continuar al sitio. En la mayoría de los navegadores está detrás de un botón como "Advanced" (en un navegador en español, "Avanzado" o "Configuración avanzada").
3. Inicia sesión con el nombre de usuario `admin` y la contraseña del instalador.

Después sigue los [primeros pasos](./first-steps): cambia la contraseña, activa la autenticación de dos factores y dale al panel su propio dominio y un certificado gratuito, para que desaparezca el aviso del navegador.

### La ruta secreta {#the-secret-path}

El panel solo responde bajo su ruta secreta, como `/k3x9p2qa7m1z/`. Cualquier otra dirección en su puerto devuelve `404`, incluida la dirección sin ruta `https://203.0.113.10:28443/`. Los escáneres que prueban direcciones habituales de paneles no encuentran nada.

Mantén la ruta en privado, como una contraseña. Si la olvidas, ejecuta `nefo info` en el servidor (consulta [Línea de comandos](./cli#nefo-info)).

Si escribes `http://` en lugar de `https://`, el panel responde con un error en lugar de la página de inicio de sesión.

## Abrir el puerto en el firewall en la nube {#open-the-port-in-your-cloud-firewall}

Si tu proveedor tiene un firewall fuera del servidor (un grupo de seguridad o firewall en la nube), permite allí el puerto TCP del panel. El instalador no puede hacerlo por ti. Más adelante, cuando alojes sitios web, permite también los puertos TCP 80 y 443. Consulta [Requisitos](./requirements#firewalls).

## Variables de entorno {#environment-variables}

Puedes cambiar lo que hace el instalador con estas variables:

| Variable | Predeterminado | Qué hace |
|---|---|---|
| `NEFO_VERSION` | `latest` | Instala una versión concreta, como `v0.1.0`, en lugar de la última. |
| `NEFO_REPO` | `nefocp/nefo` | Descarga desde otro repositorio de GitHub (`owner/name`), como un fork. |
| `NEFO_FORCE` | sin definir | Defínela como `1` para probar un sistema operativo no compatible. La comprobación de la CPU se sigue aplicando. |
| `NEFO_SETUP` | sin definir | Opciones para `nefo setup`, para configurar el servidor en la misma ejecución. Consulta [más abajo](#set-the-server-up-in-the-same-run). |

Pon la variable antes de `bash`:

```sh
curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | NEFO_VERSION=v0.1.0 bash
```

Con `sudo`, escríbela así: `| sudo NEFO_VERSION=v0.1.0 bash`.

`NEFO_REPO` solo cambia desde dónde descarga el instalador. Las actualizaciones posteriores vienen del repositorio integrado en el programa, o de `update_url` en el archivo de configuración (consulta [Actualizar](./update#where-updates-come-from)).

## Configurar el servidor en la misma ejecución {#set-the-server-up-in-the-same-run}

El instalador también puede configurar el servidor, como la página de configuración inicial del panel: un servidor web, versiones de PHP, un servidor de bases de datos y extras. Pásale las opciones de [`nefo setup`](./cli#nefo-setup) después de `bash -s --`:

```sh
curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | sudo bash -s -- --web nginx --php 8.4 --db mariadb --redis
```

O ponlas en la variable `NEFO_SETUP`:

```sh
curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | sudo NEFO_SETUP="--web nginx --php 8.4 --db mariadb" bash
```

Qué cambia con las opciones de configuración inicial:

- El instalador comprueba las opciones justo después de la descarga, antes de configurar nada. Una opción incorrecta, como `--web caddy`, lo detiene, y puedes volver a ejecutarlo. Las opciones solo pueden contener letras, dígitos y `. , = _ -`.
- Una vez iniciado el panel, el instalador ejecuta `nefo setup` con tus opciones y `--yes`, y muestra su registro. Esto tarda unos minutos. El panel hace las instalaciones como una de sus tareas, así que continúan aunque tu sesión SSH termine: en ese caso, síguelas en el panel.
- La dirección, el nombre de usuario y la contraseña se muestran al final, después de la configuración.
- El panel ya no abre su página de configuración inicial cuando inicias sesión.

Si la configuración se detiene, por ejemplo porque falla una descarga, el panel queda instalado. El instalador igualmente muestra la dirección y la contraseña, luego dice que la configuración se detuvo y termina con un error. Corrige el problema y ejecuta `nefo setup` con las mismas opciones: las partes ya instaladas se omiten.

## Instalar de nuevo {#install-again}

El instalador se niega a ejecutarse cuando existe `/etc/nefo/config.json`, así que no puede sobrescribir un panel que funciona. Para empezar de cero, primero quita Nefo (consulta [Desinstalar](./uninstall)) y luego vuelve a ejecutar el instalador.
