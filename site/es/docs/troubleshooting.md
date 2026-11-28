---
title: Solución de problemas
description: Soluciones para los problemas más comunes con Nefo, desde un panel al que no puedes acceder hasta errores de certificados y sitios web que muestran 502 o 503.
---

# Solución de problemas

La mayoría de los problemas con Nefo tienen una solución corta en el servidor, como root: `nefo info` muestra una dirección del panel que perdiste, `nefo reset-password` define una contraseña nueva (con `--disable-2fa` también desactiva la autenticación de dos factores) y `journalctl -u nefo` muestra por qué no se inicia el panel. Esta página también trata los errores de certificados, los sitios web que muestran 502 o 503 y las restauraciones que fallan.

La mayoría de las soluciones necesitan acceso SSH al servidor como root. Si SSH tampoco funciona, usa la consola web del panel de control de tu proveedor.

## No puedes abrir el panel {#you-can-t-open-the-panel}

### Olvidaste la dirección {#you-forgot-the-address}

Ejecuta esto en el servidor:

```sh
nefo info
```

La salida muestra la URL completa, con el puerto y la ruta secreta: con el dominio del panel si le diste uno, y con la primera dirección IP del servidor. Si esa dirección es privada, agrega `--host` y tu dirección IP pública, como `nefo info --host 203.0.113.10`.

### La página muestra 404 {#the-page-shows-404}

El panel solo responde bajo su ruta secreta, como `https://203.0.113.10:28443/k3x9p2qa7m1z/`. Cualquier otra dirección devuelve `404`, incluida la dirección sin la ruta. Comprueba la ruta con `nefo info`.

Asegúrate también de que la dirección empieza por `https://`. Con `http://`, el panel responde con un error en lugar de la página de inicio de sesión.

### La página no carga en absoluto {#the-page-doesn-t-load-at-all}

Cuando el navegador no puede conectarse, o espera y luego se rinde, el panel no está en ejecución o algo bloquea la conexión.

1. **Comprueba que el panel está en ejecución:**

   ```sh
   systemctl status nefo
   ```

   Si no dice `active (running)`, consulta [El panel no se inicia](#the-panel-doesn-t-start).

2. **Revisa el firewall en la nube de tu proveedor.** Debe permitir el puerto TCP del panel. Averigua el puerto con `nefo info --port`.

3. **Comprueba que tu dirección IP sigue permitida.** Si limitaste el puerto del panel a tu propia dirección IP, en ufw o en el firewall en la nube, y tu dirección IP cambió (algo común en conexiones domésticas y móviles), permite la nueva. En ufw, con tu dirección nueva y el puerto del panel:

   ```sh
   ufw allow from 198.51.100.7 to any port 28443 proto tcp
   ```

4. **Comprueba si fail2ban te bloqueó.** Si instalaste fail2ban desde el panel, bloquea una dirección durante una hora tras 10 inicios de sesión fallidos en el panel en 10 minutos (5 para SSH). Desbloquea tu dirección:

   ```sh
   fail2ban-client set panel unbanip 198.51.100.7
   ```

   Para SSH, usa el jail `sshd` en lugar de `panel`.

### "Demasiados intentos fallidos" {#too-many-failed-attempts}

Tras 5 inicios de sesión fallidos en 15 minutos, el panel rechaza los siguientes intentos desde tu dirección durante 15 minutos, y muestra "Demasiados intentos fallidos. Inténtalo de nuevo en … segundos." Los códigos de dos factores incorrectos también cuentan. Espera a que se cumpla el tiempo. El bloqueo se guarda en memoria, así que `systemctl restart nefo` también lo quita.

### El panel no se inicia {#the-panel-doesn-t-start}

Lee el motivo en el registro:

```sh
journalctl -u nefo -n 50 --no-pager
```

| El registro dice | Qué hacer |
|---|---|
| `address already in use` | Otro programa usa el puerto del panel. Detén ese programa, o cambia el puerto en `/etc/nefo/config.json` (consulta [Línea de comandos](./cli#the-configuration-file)). |
| Un error sobre `/etc/nefo/config.json` | El archivo de configuración tiene un error, por ejemplo después de editarlo. Corrige la línea que nombra el mensaje. |
| `database schema version … is newer than this panel supports` | Una versión más antigua del programa se ejecuta con datos de una más reciente. Vuelve a instalar la última versión (consulta [Actualizar](./update#if-an-update-fails)). |

Después de corregirlo, ejecuta `systemctl restart nefo`.

## Olvidaste la contraseña {#you-forgot-the-password}

Define una contraseña aleatoria nueva en el servidor:

```sh
nefo reset-password
```

Muestra la contraseña nueva y cierra la sesión de la cuenta en todas partes. Inicia sesión con ella y luego elige tu propia contraseña en **Cuenta**.

## Perdiste el teléfono de la 2FA {#lost-your-2fa-phone}

Si ya no puedes obtener códigos de tu aplicación de autenticación, desactiva la autenticación de dos factores desde el servidor. Esto también define una contraseña nueva:

```sh
nefo reset-password --disable-2fa
```

Inicia sesión con la contraseña nueva y luego vuelve a configurar la autenticación de dos factores en **Cuenta**, con tu teléfono nuevo.

### El código nunca es válido {#codes-are-always-not-valid}

El código depende de la hora. El panel acepta el código actual y los inmediatamente anterior y posterior, y cada código una sola vez. Si todos los códigos fallan con "Ese código no es válido", comprueba que el reloj del servidor es correcto:

```sh
timedatectl
```

`System clock synchronized` debería decir `yes`. Revisa también la hora de tu teléfono.

## El navegador avisa sobre el certificado {#the-browser-warns-about-the-certificate}

Hasta que el panel tiene su propio certificado, usa uno autofirmado, y todos los navegadores avisan sobre él. Es lo esperado. Dale al panel un dominio y un certificado gratuito para que el aviso desaparezca (consulta [Primeros pasos](./first-steps#give-the-panel-its-own-domain)).

Si el panel tiene un certificado y el navegador sigue avisando:

- Abriste el panel con su dirección IP. El certificado es para el dominio: usa `https://panel.example.com:28443/k3x9p2qa7m1z/`.
- El certificado caducó porque no se pudo renovar. Revisa **Configuración** → **Certificado** y haz clic en **Obtener un certificado nuevo**. Las causas son las mismas que en la sección siguiente.

## Falla una solicitud de certificado {#a-certificate-request-fails}

Antes de consultar a Let's Encrypt, el panel comprueba que cada dominio llega a este servidor: pone un archivo de prueba en el servidor y lo descarga a través del dominio por HTTP, en el puerto 80. Si eso falla, no solicita nada y enumera cada problema bajo "these names do not reach this server yet, so no certificate was requested", un mensaje en inglés que indica que esos nombres todavía no llegan a este servidor y que no se solicitó ningún certificado.

| El mensaje dice | Causa y solución |
|---|---|
| `no DNS record. Add an A record that points to this server's IP address.` | El dominio todavía no tiene dirección. Agrega el registro A en tu proveedor de DNS. Un registro nuevo puede tardar un rato en funcionar en todas partes. |
| `no answer over HTTP on port 80 (…). Check the DNS record and the firewall.` | El puerto 80 está cerrado, o el dominio apunta a otro servidor. Abre el puerto 80 en ufw y en el firewall en la nube de tu proveedor, y comprueba que el servidor web está en ejecución. |
| `answered HTTP …, but not from this server. Check that the DNS record points here.` | Otra cosa respondió por el dominio: el registro apunta a otro servidor, o hay un proxy o una CDN delante, como el proxy de Cloudflare (la nube naranja). |

Cada línea empieza con el dominio y las direcciones a las que apunta. Si esas no son las direcciones de tu servidor, corrige primero el registro DNS. Si aparece una dirección IPv6 que no es la de tu servidor, corrige o quita el registro AAAA.

**Detrás del proxy de Cloudflare:**

- Para un sitio web, define el registro como "DNS only" mientras se solicita el certificado, o marca **Validar mediante DNS (Cloudflare)** en la pestaña **SSL / HTTPS** del sitio web. La validación por DNS necesita un token de API de Cloudflare, que agregas en **Configuración** → **Cloudflare**. Los nombres comodín (`*.example.com`) siempre necesitan validación por DNS.
- Para el dominio propio del panel, el registro debe apuntar directamente al servidor, sin el proxy.

**Para el dominio propio del panel,** la pestaña **Certificado** dice "Esto necesita el servidor web" hasta que instales uno, porque el servidor web es el que responde a Let's Encrypt en el puerto 80. Instálalo primero en la página **Sitios web**.

Si la comprobación pasa pero Let's Encrypt sigue rechazándolo, el registro de la tarea muestra su motivo. Let's Encrypt limita cuántos intentos fallidos puedes hacer, así que corrige la causa antes de volver a intentarlo.

Cuando falla una renovación automática, el panel muestra "Falló la renovación del certificado de example.com" y vuelve a intentarlo cada 12 horas. El certificado sigue funcionando hasta que caduca.

## Un sitio web muestra un error {#a-website-shows-an-error}

| Lo que ven los visitantes | Causa | Qué hacer |
|---|---|---|
| El texto "This website is temporarily unavailable." (HTTP 503) | El sitio web está suspendido, a mano o porque pasó su fecha de vencimiento. | Abre la pestaña **Vencimiento y suspensión** del sitio web. Desmarca **Suspender este sitio web**, y borra o cambia la **Fecha de vencimiento**. Si la fecha sigue en el pasado, el panel vuelve a suspender el sitio web en menos de una hora. |
| Una página "502 Bad Gateway" o "503 Service Unavailable" del servidor web | El programa que está detrás del sitio web no responde: PHP-FPM en un sitio web PHP, o la aplicación o el contenedor en un sitio web de aplicación. Nginx muestra 502; Apache y OpenLiteSpeed pueden mostrar 503. | En PHP, abre **Servicios** e inicia o reinicia el servicio PHP-FPM de la versión de PHP del sitio web, como `php8.3-fpm`. En una aplicación, el sitio web muestra "Aplicación detenida": lee los **Registros de la aplicación** en su pestaña **Registros**. |
| Ningún sitio web responde | El servidor web está detenido. La página **Sitios web** dice que "no está en ejecución". | Inicia el servidor web en la página **Servicios**. |
| HTTP 429 | El límite de solicitudes del sitio web está activado (solo Nginx), y el visitante envió demasiadas solicitudes. | Sube el límite, o ponlo en 0 para desactivarlo, en la pestaña **Bloquear acceso** del sitio web. |
| HTTP 403 | La dirección del visitante está entre las direcciones bloqueadas del sitio web, o el firewall de aplicaciones web bloqueó la solicitud. | Revisa **Direcciones IP bloqueadas** en la pestaña **Bloquear acceso**, y los eventos recientes del firewall en la pestaña **Firewall web (WAF)**. |

Para otros errores, lee el registro de errores y los errores de PHP del sitio web en su pestaña **Registros**.

## Falla la subida o la restauración de una copia de seguridad {#a-backup-upload-or-restore-fails}

- **Una restauración falla con "the passphrase does not unlock it".** La copia se cifró con una frase de contraseña distinta de la actual del almacenamiento. Vuelve a restaurar y escribe esa frase en **Frase de contraseña de cifrado (opcional)**. Consulta [Cifrar las copias](./backups#encrypt-the-copies).
- **Las subidas a Google Drive fallan al cabo de una semana.** La pantalla de consentimiento de OAuth sigue en **Testing**: cámbiala a **In production** en la consola de Google Cloud, y luego elige **Cambiar** → **Volver a conectar** en **Copias de seguridad** → **Almacenamiento remoto**. Consulta [Google Drive](./backups#google-drive).
- **Una copia de seguridad de un sitio web se restaura como sitio web estático.** La copia se hizo con una versión anterior a la 0.24.1, que solo registra la versión de PHP. Para un sitio web de aplicación o de proxy inverso, primero vuelve a crear el sitio web con su tipo y luego restaura. Las copias hechas con la 0.24.1 o posterior traen de vuelta el sitio web con su tipo. Consulta [Restaurar un sitio web que se eliminó](./backups#restore-a-website-that-was-deleted).

## Dónde están los registros {#where-the-logs-are}

| Registro | En el servidor | En el panel |
|---|---|---|
| El panel | `journalctl -u nefo` | — |
| Los registros de acceso y de errores de un sitio web | `/var/log/nginx/`, `/var/log/apache2/` o `/var/log/openlitespeed/`, como `example.com.access.log` y `example.com.error.log` | Sitios web → (sitio) → **Registros** |
| Los errores de PHP de un sitio web | `/var/www/example.com/logs/php-error.log` | Sitios web → (sitio) → **Registros** → **Errores de PHP** |
| Un sitio web de aplicación | `journalctl -u panel-app-example_com` | Sitios web → (sitio) → **Registros** → **Registros de la aplicación** |
| Las tareas cron | `/var/log/panel-cron/` | **Tareas cron** |
| Los inicios de sesión y los cambios en el panel | — | **Cuenta** → **Actividad reciente** |

Cuando informes de un problema, incluye la salida de `nefo version` y las líneas relevantes del registro. Quita antes las contraseñas y la ruta secreta del panel.
