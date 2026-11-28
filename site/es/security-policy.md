---
title: Política de seguridad
description: Cómo informar de una vulnerabilidad en Nefo, cómo se firman las versiones y cómo comprobar una tú mismo, y el estado actual de la seguridad.
---

# Política de seguridad

Nefo se ejecuta como root y administra todo un servidor, así que cada informe de vulnerabilidad se trata como urgente. Esta página explica cómo informar de una, cómo se firman las versiones y cómo puedes comprobar una, y en qué punto está Nefo hoy. Sigue el archivo [SECURITY.md](https://github.com/nefocp/nefo/blob/main/SECURITY.md) del repositorio.

## Informar de una vulnerabilidad {#report-a-vulnerability}

No abras una incidencia, una discusión ni un pull request públicos. Infórmala de forma privada en GitHub:

1. Abre el [repositorio de Nefo](https://github.com/nefocp/nefo) y ve a la pestaña **Security**.
2. Elige **Report a vulnerability**, o usa el [enlace directo](https://github.com/nefocp/nefo/security/advisories/new). Solo los mantenedores ven tu informe.

Incluye:

- la versión afectada (`nefo version`) y el sistema operativo;
- lo que puede hacer un atacante, y qué acceso necesita antes: ninguno, una cuenta del panel, una cuenta con shell…;
- los pasos para reproducirla, o una prueba de concepto;
- si piensas publicar tus hallazgos, y cuándo.

### Qué ocurre después {#what-happens-next}

| Paso | Plazo |
|---|---|
| El proyecto confirma que recibió tu informe | en 3 días hábiles |
| El proyecto te dice si confirma el problema y qué gravedad tiene | en 7 días |
| El proyecto publica una corrección para un problema crítico o de gravedad alta | en 14 días desde la confirmación |
| Divulgación pública | después de publicar la corrección, o como máximo 90 días después de tu informe, coordinada contigo |

Cada vulnerabilidad confirmada recibe un GitHub Security Advisory con un CVE. Se te da el crédito, salvo que prefieras quedar en el anonimato.

### Alcance {#scope}

Dentro del alcance: el programa `nefo`, su API y su interfaz, el instalador y la configuración que crea en el servidor.

Fuera del alcance:

- el aviso del navegador sobre el certificado autofirmado que crea el panel en su primer arranque (es lo esperado);
- los ataques que necesitan acceso root al servidor, o acceso físico a él;
- la denegación de servicio solo por volumen de tráfico;
- las vulnerabilidades en software de terceros que instala el panel, como Nginx, PHP o MariaDB. Infórmalas a sus propios proyectos, pero avísanos si el panel los configura de forma insegura.

### Puerto seguro {#safe-harbour}

El proyecto no emprenderá acciones legales contra ti por una investigación de seguridad que siga esta política: solo haces pruebas en servidores que son tuyos o en los que tienes permiso para probar, evitas violar la privacidad y destruir datos, y le das al proyecto un tiempo razonable para corregir el problema antes de divulgarlo.

## Versiones con soporte {#supported-versions}

Hasta la versión 1.0, solo la última versión recibe correcciones de seguridad. Actualiza desde el panel (**Configuración** → **Panel** → **Buscar actualizaciones**) o con `nefo update` en el servidor. Consulta [Actualizar](./docs/update).

## Cómo se firman las versiones {#how-releases-are-signed}

GitHub Actions compila cada versión a partir de una etiqueta de versión del repositorio. Una versión contiene:

- `nefo-linux-amd64` y `nefo-linux-arm64`, los programas;
- `install.sh`, el instalador;
- `VERSION`;
- `checksums.txt`, las sumas de verificación SHA-256 de los dos programas y del instalador;
- `checksums.txt.sig`, una firma Ed25519 de `checksums.txt`;
- `appstore-catalog.json` y `appstore-catalog.json.sig`, el catálogo de la tienda de aplicaciones con su propia firma.

La clave pública del proyecto viene integrada en cada programa Nefo y copiada en `install.sh`. También está en el repositorio como [`internal/update/release.pub`](https://github.com/nefocp/nefo/blob/main/internal/update/release.pub):

```
-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEA2c6nGagX7yp8Y8YftuOqW2WVprKe6h0hv+3s9iB+KJA=
-----END PUBLIC KEY-----
```

Tres componentes comprueban las firmas antes de usar una descarga:

- **El instalador** comprueba la firma de `checksums.txt` y la suma de verificación del programa. Si alguna es incorrecta, se detiene con el mensaje "invalid signature: checksums.txt was not signed by the project. Do not install this file.", es decir, que la firma no es válida porque el proyecto no firmó la lista de sumas de verificación, y que no instales el archivo.
- **`nefo update`**, y **Instalar actualización** en el panel, hacen lo mismo con la clave integrada en el programa que ya ejecutas, y luego ejecutan el programa nuevo una vez antes de reemplazar `/usr/local/bin/nefo`.
- **La tienda de aplicaciones** descarga el catálogo una vez al día y lo ignora salvo que su firma sea válida.

### Lo que la firma prueba y lo que no {#what-the-signature-does-and-doesn-t-prove}

La firma demuestra que los archivos vienen de la compilación de versiones del proyecto y que no se modificaron después, en GitHub ni en el camino hasta tu servidor.

- **Las actualizaciones** se comprueban con la clave que lleva el Nefo que ya tienes, así que una versión modificada en GitHub no puede conseguir que se la instale.
- **Una primera instalación** es distinta: el instalador que descargas lleva la clave con la que comprueba. Si alguien pudiera modificar el propio `install.sh`, también podría cambiar esa clave. Para descartarlo, comprueba tú mismo `install.sh` antes de ejecutarlo, como se muestra más abajo.
- La clave de firma se guarda como secreto del repositorio de GitHub, y las versiones las firma allí GitHub Actions. Alguien que se apoderara de la cuenta de GitHub del proyecto podría firmar una versión.

## Comprobar tú mismo una versión {#check-a-release-yourself}

Necesitas `curl`, `sha256sum` y OpenSSL 3 (en Ubuntu 22.04 y posteriores, y en Debian 12 y posteriores). Este ejemplo comprueba la última versión en un servidor amd64, como root; define `V` con otra etiqueta de versión para comprobar esa versión.

1. Descarga las sumas de verificación, su firma y el instalador:

   ```sh
   V=$(curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/VERSION)
   cd /tmp
   for f in checksums.txt checksums.txt.sig install.sh; do
     curl -fsSLO "https://github.com/nefocp/nefo/releases/download/$V/$f"
   done
   ```

2. Guarda la clave pública de esta página:

   ```sh
   cat > nefo-release.pub <<'EOF'
   -----BEGIN PUBLIC KEY-----
   MCowBQYDK2VwAyEA2c6nGagX7yp8Y8YftuOqW2WVprKe6h0hv+3s9iB+KJA=
   -----END PUBLIC KEY-----
   EOF
   ```

3. Comprueba la firma y luego la suma de verificación del instalador:

   ```sh
   openssl pkeyutl -verify -pubin -inkey nefo-release.pub -rawin \
     -in checksums.txt -sigfile checksums.txt.sig
   grep " install.sh$" checksums.txt | sha256sum -c -
   ```

   El primer comando debe mostrar `Signature Verified Successfully`, y el segundo `install.sh: OK`. Si alguno falla, no ejecutes el instalador, e infórmalo como se indica más arriba.

4. Ejecuta el instalador que comprobaste. Descarga el programa para tu CPU y lo comprueba con la misma clave:

   ```sh
   NEFO_VERSION=$V bash install.sh
   ```

Para comprobar un programa que descargaste tú mismo, como `nefo-linux-amd64`, ejecuta `grep " nefo-linux-amd64$" checksums.txt | sha256sum -c -` después del paso 3.

## Cómo protege Nefo el servidor {#how-nefo-protects-the-server}

Un breve resumen; [Seguridad](./docs/security) tiene los detalles:

- El panel solo responde bajo una ruta secreta, en un puerto aleatorio. Cualquier otra dirección devuelve `404`.
- Usa HTTPS desde su primer arranque, con TLS 1.2 o más reciente.
- El inicio de sesión admite autenticación de dos factores. 5 intentos fallidos en 15 minutos bloquean una dirección durante 15 minutos. Las contraseñas se guardan como hashes bcrypt.
- Los inicios de sesión y los cambios se anotan en un registro de actividad, que se conserva 90 días.
- Cada página tiene una Content Security Policy estricta, y cada cambio necesita un encabezado que otros sitios web no pueden enviar.
- Las [reglas para el código](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md#security-rules-for-code) del proyecto prohíben construir comandos de shell a partir de la entrada: un programa recibe cada valor como un argumento propio, y la entrada se comprueba con una lista de permitidos antes de que llegue a una ruta de archivo, a un archivo de configuración o a un comando. Los comandos que escribes tú mismo, como las tareas cron y los scripts de despliegue, se ejecutan en un shell tal como los escribiste.
- Se rechazan las reglas de firewall y los ajustes de SSH que te dejarían fuera.
- Cada sitio web se ejecuta como su propio usuario de Linux, y las cuentas SFTP quedan limitadas a la carpeta del sitio web.

## Estado actual {#current-status}

Nefo está en beta. Su primera versión pública fue la v0.20.0, en noviembre de 2026; el [registro de cambios](./changelog) indica la actual. Todavía no se ha hecho ninguna auditoría de seguridad externa.

Prueba Nefo en un servidor que puedas reconstruir antes de confiarle sitios web importantes, guarda copias de tus copias de seguridad fuera del servidor y mantén Nefo actualizado. Una revisión de seguridad externa al proyecto es uno de los pasos previos a la 1.0 (consulta la [hoja de ruta](./roadmap)). Si revisas el código y encuentras algo, infórmalo como se describe más arriba.
