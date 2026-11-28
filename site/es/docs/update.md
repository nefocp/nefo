---
title: Actualizar
description: Actualiza Nefo desde el panel o con nefo update, y conoce cómo se verifican las versiones y qué hacer si una actualización falla.
---

# Actualizar

Para actualizar Nefo, haz clic en **Buscar actualizaciones** en **Configuración** → **Panel**, o ejecuta `nefo update` como root. El panel descarga la última versión, comprueba su firma Ed25519 y su suma de verificación SHA-256, reemplaza `/usr/local/bin/nefo` en un solo paso y se reinicia. Los sitios web siguen funcionando. Nefo nunca se actualiza solo.

Esta página muestra cómo actualizar Nefo, cómo se comprueba cada versión antes de instalarla y qué hacer si una actualización falla. Hasta la versión 1.0, solo la última versión recibe correcciones de seguridad, así que mantén Nefo actualizado.

## Actualizar desde el panel {#update-from-the-panel}

1. Abre **Configuración** → **Panel**. La versión actual aparece junto al nombre Nefo.
2. Haz clic en **Buscar actualizaciones**. Si ya tienes la última versión, el panel dice "Esta es la versión más reciente."
3. Si existe una versión más reciente, el panel dice "La versión v0.2.0 está disponible." Haz clic en **Instalar actualización** y confirma.

El progreso se ve en la página. Cuando la versión nueva está lista, el panel se reinicia, y la página se recarga sola al cabo de unos 20 segundos. Tus sitios web siguen funcionando durante el reinicio.

También puedes actualizar desde el panel principal: haz clic en **Buscar actualizaciones** y luego en **Actualizar a v0.2.0**.

## Actualizar desde la línea de comandos {#update-from-the-command-line}

```sh
nefo update
```

```
Installed: v0.1.0
Latest:    v0.2.0
Release signature verified
Downloading nefo-linux-amd64 v0.2.0
Checked: Nefo v0.2.0 (commit 5e6f7a8)
Installed. The panel restarts now; reload the page in a few seconds.
```

Si ya tienes la última versión, el comando muestra las dos versiones y no cambia nada. Para solo comprobar, ejecuta `nefo update --check`.

## Qué ocurre durante una actualización {#what-happens-during-an-update}

1. El panel lee el número de la última versión en las versiones del proyecto en GitHub.
2. Descarga la lista de sumas de verificación y su firma, y comprueba la firma con la clave del proyecto, que viene integrada en el panel.
3. Descarga el programa nuevo para tu CPU y comprueba su suma de verificación SHA-256 con la lista.
4. Ejecuta el programa nuevo una vez, para asegurarse de que funciona en este servidor.
5. Reemplaza `/usr/local/bin/nefo` en un solo paso, así que nunca queda un programa escrito a medias.
6. Reinicia el servicio `nefo`. Si hay una tarea en curso, como la instalación de un software, el panel la deja terminar primero, durante un máximo de 75 segundos.

Cuando se inicia la versión nueva, actualiza la base de datos del panel a su nuevo formato. También pone al día con sus plantillas los archivos de configuración que administra (servidor web, pools de PHP-FPM). Si el servidor web rechaza los archivos nuevos, el panel vuelve a poner los anteriores.

## Cómo se verifican las versiones {#how-releases-are-verified}

Cada versión en GitHub contiene los programas para amd64 y arm64, `install.sh`, un archivo `VERSION`, `checksums.txt` con las sumas de verificación SHA-256 de los dos programas y del instalador, y `checksums.txt.sig`, una firma Ed25519 de `checksums.txt`.

Tanto el instalador como el actualizador rechazan un `checksums.txt` que no esté firmado con la clave del proyecto, y una descarga cuya suma de verificación no coincida. Durante una actualización, los errores dicen:

- "checksums.txt has no valid signature from the project: the update was not installed";
- "the download does not match checksums.txt: not installed".

Así, alguien que pudiera modificar los archivos de la versión en GitHub, o en el camino hasta tu servidor, no puede hacer que una actualización instale su propio programa: la actualización se comprueba con la clave integrada en el Nefo que ya ejecutas. Una primera instalación es distinta: el instalador que descargas lleva su propia copia de la clave. Además, la clave de firma se guarda como secreto del repositorio de GitHub, donde GitHub Actions firma cada versión, así que alguien que se apoderara del repositorio podría firmar una versión. Consulta [Lo que la firma prueba y lo que no](../security-policy#what-the-signature-does-and-doesn-t-prove).

Una compilación de desarrollo, hecha sin la clave del proyecto, no puede comprobar firmas. Solo comprueba la suma de verificación, y su registro dice "Development build: the release signature is not checked".

## Qué se conserva {#what-is-kept}

Una actualización solo reemplaza el programa, `/usr/local/bin/nefo`. Todo lo demás se mantiene:

- el archivo de configuración `/etc/nefo/config.json`, incluidos el puerto y la ruta secreta;
- la carpeta de datos `/var/lib/nefo`: tu cuenta y la autenticación de dos factores, los ajustes, los sitios web, las programaciones, los certificados y el registro de actividad;
- tus sitios web, bases de datos, archivos y copias de seguridad, y los servicios que los ejecutan.

Solo se reinicia el panel. Los sitios web, las bases de datos y los demás servicios siguen funcionando.

## Si una actualización falla {#if-an-update-fails}

**La actualización se detiene antes de instalar el programa nuevo.** Esto ocurre cuando falla la descarga, la firma o la suma de verificación no coinciden, o el programa nuevo no funciona en este servidor. No ha cambiado nada: la versión anterior sigue funcionando. El error aparece en el registro de la tarea en el panel, o en la terminal. Vuelve a intentarlo más tarde, e informa del error si continúa.

**La versión nueva no se inicia.** Revisa el servicio y su registro:

```sh
systemctl status nefo
journalctl -u nefo -n 50 --no-pager
```

Informa del problema en el [repositorio de GitHub](https://github.com/nefocp/nefo/issues) con esta salida. Para volver a la versión anterior, instala su programa a mano. Este ejemplo usa `v0.1.0` en un servidor amd64:

```sh
cd /tmp
curl -fsSLO https://github.com/nefocp/nefo/releases/download/v0.1.0/nefo-linux-amd64
curl -fsSLO https://github.com/nefocp/nefo/releases/download/v0.1.0/checksums.txt
grep " nefo-linux-amd64$" checksums.txt | sha256sum -c -
install -m 0755 nefo-linux-amd64 /usr/local/bin/nefo
systemctl restart nefo
```

Continúa solo si `sha256sum` muestra `OK`. En un servidor ARM, usa `nefo-linux-arm64`.

::: warning Volver a una versión anterior
Si la versión más reciente ya modificó la base de datos del panel, una versión anterior se niega a iniciarse con "database schema version … is newer than this panel supports". En ese caso, solo una versión más reciente funciona con esos datos. Antes de una actualización importante, puedes guardar una copia de la carpeta de datos: `systemctl stop nefo`, luego `cp -a /var/lib/nefo /root/nefo-data-copy`, luego `systemctl start nefo`.
:::

## Actualizaciones automáticas {#automatic-updates}

Nefo no se actualiza solo, ni busca actualizaciones por su cuenta. Solo las busca cuando haces clic en **Buscar actualizaciones** o ejecutas `nefo update`. Para enterarte de las versiones nuevas, sigue las versiones del [repositorio de GitHub](https://github.com/nefocp/nefo/releases). El [registro de cambios](../changelog) enumera lo que cambió en cada una.

Esto se refiere solo al propio panel. Los paquetes del sistema operativo son otro asunto: el panel los revisa cada pocas horas, los muestra en **Sistema** → **Actualizaciones** y puede instalar las actualizaciones de seguridad automáticamente. Consulta [Sistema](./os#updates).

## De dónde vienen las actualizaciones {#where-updates-come-from}

De forma predeterminada, las actualizaciones vienen de `https://github.com/nefocp/nefo/releases/latest/download`. El campo `update_url` en `/etc/nefo/config.json` puede apuntar a otra dirección que publique los mismos archivos. Debe empezar por `https://`; si no, la búsqueda de actualizaciones falla con "update_url en config.json debe ser una dirección https://." Déjalo sin definir salvo que sepas que lo necesitas.
