---
title: Certificados SSL
description: Obtén certificados gratuitos de Let's Encrypt, certificados comodín mediante el DNS de Cloudflare, o instala los tuyos, y soluciona los fallos habituales.
---

# Certificados SSL

Nefo obtiene con un clic un certificado gratuito de Let's Encrypt para cada sitio web y lo renueva por su cuenta. También puede obtener certificados comodín mediante el DNS de Cloudflare, instalar un certificado que ya tienes, redirigir HTTP a HTTPS y enviar HSTS. Antes de cada solicitud comprueba que cada nombre llega al servidor, así que los intentos fallidos no cuentan para los límites de Let's Encrypt.

Esta página trata de los requisitos, la redirección a HTTPS y HSTS, los certificados comodín mediante el DNS de Cloudflare, la instalación de un certificado que ya tienes, cómo volver a desactivar HTTPS y qué hacer cuando un certificado falla.

## Antes de empezar {#before-you-start}

Para un certificado normal, Let's Encrypt comprueba por HTTP simple que cada nombre del sitio web llega a tu servidor. Todos los nombres deben pasar la comprobación: el dominio y cada uno de sus [nombres de dominio adicionales](./websites#domains-and-aliases).

- **DNS:** cada nombre tiene un registro A que apunta a la dirección IP de este servidor, por ejemplo `203.0.113.10`. Si un nombre también tiene un registro AAAA (IPv6), debe apuntar también a este servidor o eliminarse.
- **Puerto 80:** el servidor responde en el puerto 80 desde internet. Nefo abre los puertos 80 y 443 en ufw cuando instala el servidor web. Si tu proveedor de hosting tiene su propio firewall, abre también allí el puerto 80.
- **Proxy:** si tu DNS está en Cloudflare con el proxy activado (nube naranja), lee primero [Proxy de Cloudflare](#cloudflare-proxy-orange-cloud).

Al activar HTTPS aceptas el [Acuerdo de suscriptor](https://letsencrypt.org/repository/) de Let's Encrypt.

## Obtener un certificado gratuito {#get-a-free-certificate}

1. Abre **Sitios web** y luego la pestaña **SSL / HTTPS** del sitio web. En la lista de sitios web, **Activar HTTPS**, en la columna **SSL**, abre la misma pestaña.
2. En **Certificado gratuito de Let's Encrypt**, deja desmarcada **Validar mediante DNS (Cloudflare)**.
3. Haz clic en **Obtener certificado**.

Un registro muestra los pasos: Nefo comprueba que cada nombre llega a este servidor, le pide el certificado a Let's Encrypt, valida cada nombre y activa HTTPS en el servidor web. Termina con "Done. The certificate is valid until" y la fecha.

Cuando un sitio obtiene su primer certificado, Nefo también cambia a su dirección `https://` un sitio WordPress que haya instalado, y reinicia una aplicación Docker con su dirección `https://`. Los archivos se guardan en `/var/lib/nefo/certs/example.com/` como `fullchain.pem` y `privkey.pem`.

### La comprobación antes de la solicitud {#the-check-before-the-request}

Antes de consultar a Let's Encrypt, Nefo pone un archivo de prueba en el servidor y lo descarga a través de cada nombre por HTTP. Si un nombre falla, Nefo no envía la solicitud, así que los intentos fallidos no cuentan para los límites de Let's Encrypt. El registro dice entonces "these names do not reach this server yet, so no certificate was requested:", es decir, que esos nombres todavía no llegan a este servidor y que no se solicitó ningún certificado, con una línea por nombre:

| Mensaje | Qué corregir |
|---|---|
| `blog.example.com: no DNS record. Add an A record that points to this server's IP address.` | Agrega el registro A y espera a que sea visible. |
| `example.com (203.0.113.10): no answer over HTTP on port 80 (…). Check the DNS record and the firewall.` | Abre el puerto 80 en el firewall de tu proveedor y comprueba que el servidor web está en ejecución. |
| `example.com (198.51.100.7) answered HTTP 404, but not from this server. Check that the DNS record points here.` | El nombre apunta a otro servidor: una dirección IP antigua, un registro AAAA en otro lugar o un proxy como Cloudflare. |

Las direcciones entre paréntesis son las que resuelve el nombre. Compáralas con la dirección IP de tu servidor.

## Redirigir HTTP a HTTPS {#redirect-http-to-https}

Cuando el sitio tenga un certificado, activa **Redirigir HTTP a HTTPS** en la pestaña **SSL / HTTPS** o en la pestaña **Resumen**. Todas las direcciones `http://` responden entonces con una redirección permanente (301) a `https://`. La validación de Let's Encrypt y los webhooks de Git siguen funcionando por HTTP.

Para que los navegadores usen HTTPS incluso antes de la redirección, activa también [HSTS](#hsts).

## HSTS {#hsts}

HSTS (el encabezado `Strict-Transport-Security`) indica a los navegadores que abran el sitio solo por `https://`, incluso cuando alguien escribe `http://` o sigue un enlace antiguo. Activa **HSTS (Strict-Transport-Security)** en la pestaña **SSL / HTTPS**. Necesita un certificado y **Redirigir HTTP a HTTPS**; desactivar la redirección también desactiva HSTS.

Nefo envía entonces `Strict-Transport-Security: max-age=15552000` (180 días, unos 6 meses) con las respuestas HTTPS del sitio, en Nginx, Apache y OpenLiteSpeed. Marca **Incluir subdominios** para agregar `includeSubDomains`: los navegadores usan entonces HTTPS también para todos los subdominios del dominio, así que márcala solo cuando todos tengan HTTPS. Nefo nunca agrega `preload`: es difícil salir de la lista de precarga de los navegadores, así que, si acaso, envía tú mismo un dominio a ella.

::: warning
Un navegador que recibió el encabezado rechaza HTTP simple para el sitio durante 6 meses después de su última visita, aunque más adelante desactives HSTS o HTTPS. Activa HSTS solo para un sitio que vaya a mantener HTTPS.
:::

En Nginx, el encabezado se define para todo el bloque `server` del sitio. Si tus propias directivas (consulta [Sitios web](./websites#rewrite-rules-and-your-own-directives)) agregan un `location` con su propio `add_header`, Nginx descarta los encabezados del sitio en ese location: repite allí `add_header Strict-Transport-Security "max-age=15552000" always;`.

Para comprobarlo, ejecuta `curl -sI https://example.com/ | grep -i strict-transport`.

## Renovación automática {#automatic-renewal}

Nefo comprueba los certificados al iniciarse y luego cada hora. Renueva un certificado de Let's Encrypt cuando le queda un tercio de su vida útil: 30 días antes del final para un certificado de 90 días. No necesitas hacer nada, y un sitio web suspendido se sigue renovando.

Si una renovación falla, Nefo vuelve a intentarlo cada 12 horas. Mientras tanto:

- la lista de sitios web muestra "Falló la renovación" en la columna **SSL**;
- la pestaña **SSL / HTTPS** muestra "Falló el último intento de obtener un certificado." con el error;
- se envía una alerta si configuraste alertas para las renovaciones fallidas: consulta [Correo y alertas](./mail-alerts).

El propio certificado del panel, de **Configuración** → **Certificado** (consulta [Seguridad](./security#https-from-the-first-start)), se renueva de la misma forma: cuando le queda un tercio de su vida útil, y de nuevo cada 12 horas después de un fallo. Una renovación fallida se anota en el registro de actividad y envía la misma alerta.

Nefo registra su cuenta de Let's Encrypt sin dirección de correo, así que Let's Encrypt no te escribe sobre los certificados que van a caducar. Confía en cambio en las alertas del panel.

Para renovar a mano, por ejemplo después de corregir un problema, haz clic en **Renovar certificado** en la pestaña **SSL / HTTPS**.

## Certificados comodín mediante el DNS de Cloudflare {#wildcard-certificates-through-cloudflare-dns}

Un nombre comodín como `*.example.com` solo se puede validar mediante DNS. Nefo lo hace a través de la API de Cloudflare, así que el DNS del dominio debe estar alojado en Cloudflare. La validación por DNS también funciona para un sitio que todavía no es accesible por HTTP, por ejemplo detrás del proxy de Cloudflare. Para un certificado, Nefo solo agrega y quita registros TXT `_acme-challenge`.

### Crear un token de API de Cloudflare {#create-a-cloudflare-api-token}

El mismo token sirve también para la [página DNS](./dns). Sus permisos están enumerados en un solo lugar, en [El token de API](./dns#the-api-token): los certificados solo necesitan Zone → DNS → Edit; la página DNS también puede usar Zone → Zone → Read y Zone → Cache Purge → Purge.

1. En el panel de Cloudflare, abre **My Profile → API Tokens → Create Token**.
2. Usa la plantilla **Edit zone DNS**. Da el permiso Zone → DNS → Edit.
3. En los recursos de zona, incluye las zonas (dominios) que usas en este servidor.
4. Crea el token y cópialo.

Consulta la guía de Cloudflare para [crear un token de API](https://developers.cloudflare.com/fundamentals/api/get-started/create-token/).

### Guardar el token en Nefo {#save-the-token-in-nefo}

1. Abre **Configuración** y luego la pestaña **Cloudflare**.
2. Pega el token en **Token de API** y haz clic en **Comprobar y guardar**.

Nefo comprueba que Cloudflare acepta el token y que está activo, y luego muestra "Token guardado" con los últimos caracteres del token y las **Zonas** que puede usar. Usa **Reemplazar el token de API** para cambiarlo, o **Quitar** para eliminarlo.

### Obtener el certificado {#get-the-certificate}

1. Para un comodín, abre la pestaña **Dominios** del sitio web, agrega `*.example.com` a **Nombres de dominio adicionales** y haz clic en **Guardar**.
2. En la pestaña **SSL / HTTPS**, marca **Validar mediante DNS (Cloudflare)**. En un sitio con un nombre comodín está marcada y no se puede cambiar: "Obligatorio: este sitio tiene un nombre comodín (*.domain)."
3. Haz clic en **Obtener certificado**.

Nefo agrega el registro TXT `_acme-challenge.example.com`, espera hasta 3 minutos a que los servidores de nombres de Cloudflare respondan con él, valida y luego elimina el registro. El sitio recuerda la elección, así que sus renovaciones también usan la validación por DNS.

El certificado cubre los nombres; los visitantes siguen necesitando registros DNS que apunten los subdominios a este servidor.

## Usar tu propio certificado {#use-your-own-certificate}

Instala tu propio certificado cuando compraste uno a otro proveedor, o para un certificado de origen de Cloudflare.

1. Abre la pestaña **SSL / HTTPS** del sitio web y ve a **Tu propio certificado**.
2. Pega **Certificado (PEM, con intermedios)**: el certificado seguido de su cadena. También puedes elegir un archivo `.pem`, `.crt`, `.cer` o `.txt`.
3. Pega **Clave privada (PEM, sin cifrar)**, o elige un archivo `.pem`, `.key` o `.txt`.
4. Haz clic en **Instalar**.

Nefo comprueba los archivos antes de cambiar nada, y rechaza:

- un archivo que no es un certificado PEM, o que ocupa más de 64 KB;
- una clave privada protegida con una frase de contraseña. Quítala primero, por ejemplo con `openssl pkey -in key.pem -out plain.pem`;
- una clave que no corresponde al certificado;
- un certificado que todavía no es válido, o que ya caducó;
- un certificado que no cubre todos los nombres del sitio. Un nombre comodín del sitio debe figurar exactamente en el certificado.

Instala el certificado con una advertencia ("Instalado, con advertencias:") cuando los navegadores no van a confiar en él por sí solos, lo que es de esperar en un certificado de origen de Cloudflare usado detrás del proxy de Cloudflare, o cuando caduca en menos de dos semanas.

Nefo no renueva tu propio certificado. En sus dos últimas semanas, la pestaña **SSL / HTTPS** dice "Nefo no renueva un certificado que instalaste tú. Instala uno nuevo antes del" y la fecha, y **Requiere atención**, en el panel principal, enumera el sitio. Nefo también te lo recuerda una vez al día: el registro de actividad lo anota como una renovación fallida, y se envía la alerta de renovaciones fallidas. Instala un certificado nuevo a tiempo, o vuelve a Let's Encrypt con **Obtener certificado**. La lista de sitios web marca estos sitios con "cert. propio".

## Desactivar HTTPS {#turn-off-https}

Para devolver un sitio web a HTTP simple, haz clic en **Desactivar HTTPS** al final de su pestaña **SSL / HTTPS** y confirma. Nefo entonces:

- sirve el sitio web solo por `http://`: se quita la parte HTTPS de su configuración del servidor web;
- desactiva **Redirigir HTTP a HTTPS** y [HSTS](#hsts);
- elimina los archivos del certificado en `/var/lib/nefo/certs/example.com/`, sea de Let's Encrypt o tuyo, para que nada los renueve ni te los recuerde. La cuenta de Let's Encrypt se queda para los demás sitios web;
- devuelve a su dirección `http://` un sitio WordPress que haya instalado, y reinicia una aplicación Docker con su dirección `http://`.

Los visitantes que abren una dirección `https://` del sitio (desde un marcador, un resultado de búsqueda o un enlace) reciben entonces un error del navegador, porque el servidor ya no tiene un certificado para el nombre. Si el sitio tenía HSTS, los navegadores que recibieron el encabezado siguen rechazando HTTP simple durante 6 meses como máximo, así que para ellos el sitio no se abre en absoluto hasta que vuelva a tener HTTPS. Puedes obtener un certificado nuevo en cualquier momento con **Obtener certificado**.

Para reemplazar un certificado no necesitas desactivar HTTPS: obtén un certificado nuevo de Let's Encrypt o instala el tuyo encima. Eliminar un sitio web también elimina su certificado: consulta [Sitios web](./websites). Desinstalar el servidor web conserva los certificados.

## Cuando un certificado falla {#when-a-certificate-fails}

El error aparece en la pestaña **SSL / HTTPS** del sitio web, en "Falló el último intento de obtener un certificado.", y en el registro del intento.

### DNS o puerto 80 {#dns-or-port-80}

Consulta [la comprobación antes de la solicitud](#the-check-before-the-request). La mayoría de los fallos se deben a un registro DNS que todavía no apunta a este servidor, o al puerto 80 cerrado en el firewall de un proveedor.

### Proxy de Cloudflare (nube naranja) {#cloudflare-proxy-orange-cloud}

Con el proxy de Cloudflare activado, el dominio resuelve a las direcciones de Cloudflare, y las solicitudes llegan a Cloudflare antes que a tu servidor. Ajustes de Cloudflare como **Always Use HTTPS** (una redirección a HTTPS, en **SSL/TLS → Edge Certificates**) pueden entonces impedir que la comprobación llegue al servidor sin cambios, y Nefo informa de que el nombre "answered HTTP …, but not from this server", es decir, que la respuesta no vino de este servidor. Consulta también [Let's Encrypt detrás de la nube naranja](./dns#let-s-encrypt-behind-the-orange-cloud). Tienes tres salidas:

- marca **Validar mediante DNS (Cloudflare)** y obtén el certificado mediante DNS. El proxy puede seguir activado, y las renovaciones funcionan de la misma forma;
- instala un certificado de origen de Cloudflare como [tu propio certificado](#use-your-own-certificate);
- desactiva el proxy (solo DNS, nube gris) para el nombre. Las renovaciones también comprueban por HTTP, así que déjalo desactivado.

### Registros CAA {#caa-records}

Si tu dominio tiene registros CAA en su DNS, uno de ellos debe permitir `letsencrypt.org`. Si no, Let's Encrypt rechaza el certificado, y el registro muestra su error, que menciona CAA. Consulta [Let's Encrypt y CAA](https://letsencrypt.org/docs/caa/).

### Límites de solicitudes {#rate-limits}

Let's Encrypt limita cuántos certificados y validaciones fallidas pueden tener un dominio y una cuenta. Cuando alcanzas un límite, el registro muestra el mensaje de Let's Encrypt con el tiempo de espera. Consulta [Límites de solicitudes de Let's Encrypt](https://letsencrypt.org/docs/rate-limits/).

::: tip
Para hacer pruebas sin agotar los límites, agrega `"acme_directory": "https://acme-staging-v02.api.letsencrypt.org/directory"` a `/etc/nefo/config.json` y reinicia el panel con `systemctl restart nefo`. Los navegadores no confían en los certificados de staging. Quita la línea y vuelve a reiniciar para obtener certificados reales.
:::

### Validación por DNS mediante Cloudflare {#dns-validation-through-cloudflare}

| Mensaje | Qué corregir |
|---|---|
| `DNS validation needs a Cloudflare API token: add one in Settings → Cloudflare` | Guarda un token en la página **Configuración**, pestaña **Cloudflare**. |
| `Cloudflare did not accept this token.` | El token es incorrecto, caducó o no está activo. Crea uno nuevo. |
| `none of the Cloudflare zones this API token can use holds example.com` | Agrega la zona del dominio a los recursos de zona del token en Cloudflare. |
| `the DNS record _acme-challenge.example.com did not appear on … in time` | Los servidores de nombres de Cloudflare fueron lentos. Vuelve a intentarlo unos minutos más tarde. |
