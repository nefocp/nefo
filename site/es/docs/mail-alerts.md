---
title: Correo y alertas
description: Permite que tus sitios web envíen correo a través de un proveedor SMTP, y recibe alertas por Telegram, correo electrónico, Discord, Slack o un webhook cuando el servidor requiere atención o alguien inicia sesión.
---

# Correo y alertas

Nefo no tiene servidor de correo. Los sitios web envían correo a través de Postfix, que funciona como relay de solo envío hacia tu proveedor SMTP (Gmail, SendGrid, Amazon SES y otros). Las alertas te llegan por Telegram, correo electrónico, Discord, Slack o un webhook firmado cuando la CPU, la memoria o el disco están altos, un servicio se detiene, una copia de seguridad falla, un sitio web está caído o alguien inicia sesión.

Esta página trata de las dos cosas: el envío de correo desde los sitios web y las alertas. Para configurar los registros DNS de un proveedor de correo (MX, SPF, DMARC), consulta [DNS (Cloudflare)](./dns#set-up-email-records).

## Sin servidor de correo, a propósito {#no-mail-server-by-design}

Nefo no aloja buzones ni recibe correo. Un servidor de correo completo consume memoria y necesita atención constante, lo que no encaja en un VPS pequeño. Mantén tus buzones en un proveedor de correo.

Para el correo saliente, Nefo instala Postfix como relay de solo envío. Solo escucha en este servidor y pasa cada mensaje a tu proveedor (Gmail, SendGrid, Amazon SES y otros), con la sesión iniciada con la cuenta que indiques.

## Enviar correo desde los sitios web {#send-e-mail-from-websites}

Cuando el relay está configurado, la función `mail()` de PHP funciona, porque Postfix proporciona `/usr/sbin/sendmail`, que es lo que usa PHP. Los formularios de contacto, los restablecimientos de contraseña y los avisos de WordPress envían entonces correo sin ningún plugin. Sin el relay, los sitios web no pueden enviar correo.

### Configurar el relay {#set-up-the-relay}

1. Ve a **Configuración** → **Correo**.
2. Elige tu **Proveedor**. El panel completa para él el **Servidor SMTP**, el **Puerto** y el **Cifrado**.
3. Escribe el **Nombre de usuario** y la **Contraseña** de tu cuenta del proveedor. La indicación bajo **Proveedor** dice lo que espera ese proveedor.
4. Si quieres, completa **Enviar todo como (opcional)**, por ejemplo con `noreply@example.com`.
5. La primera vez, selecciona **Instalar y guardar**: el panel instala Postfix y lo configura. Más adelante, el botón es **Guardar**.

La configuración se ejecuta en segundo plano con un registro en vivo. Cuando termine, envía un mensaje de prueba (consulta más abajo).

| Proveedor | Servidor SMTP | Nombre de usuario y contraseña |
|---|---|---|
| Gmail / Google Workspace | `smtp.gmail.com` | Tu dirección de Gmail, y una contraseña de aplicación (Google account → Security → 2-Step Verification → App passwords), no tu contraseña habitual |
| Microsoft 365 / Outlook | `smtp.office365.com` | Tu dirección de Microsoft 365. El envío por SMTP debe estar permitido para el buzón |
| SendGrid | `smtp.sendgrid.net` | `apikey` (se completa por ti), y una clave de API de SendGrid con el permiso Mail Send. Envía desde un remitente verificado |
| Mailgun | `smtp.mailgun.org` | Las credenciales SMTP de tu dominio de Mailgun |
| Amazon SES | `email-smtp.us-east-1.amazonaws.com` | Tus credenciales SMTP de SES. Cambia el servidor por el endpoint de tu región, y envía desde una dirección o un dominio verificados |
| Brevo | `smtp-relay.brevo.com` | Tu usuario SMTP y tu clave SMTP de Brevo |
| Otro servidor SMTP | El servidor de tu proveedor | Puerto 587 con STARTTLS, o 465 con SSL/TLS. La mayoría de los proveedores de nube bloquean el puerto 25 |

Todos los ajustes predefinidos usan el puerto 587 con STARTTLS.

### Campos y reglas {#fields-and-rules}

| Campo | Qué escribir |
|---|---|
| **Servidor SMTP** | El nombre de host del proveedor, o una dirección IP |
| **Puerto** | De 1 a 65535, normalmente 587 o 465 |
| **Cifrado** | **STARTTLS** (puerto 587), **SSL/TLS** (puerto 465), o **Ninguno (solo este servidor)** |
| **Nombre de usuario**, **Contraseña** | Ambos o ninguno, sin espacios. Deja **Contraseña** vacía para conservar la guardada |
| **Enviar todo como (opcional)** | Una dirección de correo que reemplaza al remitente de todos los mensajes |

- **Ninguno (solo este servidor)** solo se acepta para un relay en este mismo servidor (`localhost`, `127.0.0.1` o `::1`). Sin cifrado, la contraseña viajaría por internet en claro.
- Con STARTTLS o SSL/TLS, Postfix se niega a enviar si la conexión no se puede cifrar.
- Usa **Enviar todo como (opcional)** cuando tu proveedor solo acepte direcciones de remitente verificadas. Reemplaza al remitente de todos los mensajes de todos los sitios web.
- La contraseña del proveedor se guarda en `/etc/postfix/sasl_passwd`, que solo puede leer root. Los sitios web nunca la ven.

### Enviar un mensaje de prueba {#send-a-test-message}

1. Debajo del formulario, escribe una dirección en **Enviar un mensaje de prueba a…**.
2. Selecciona **Enviar prueba**.

El panel sigue el mensaje en el registro de correo y espera hasta un minuto la respuesta del proveedor. El resultado indica si el mensaje se envió, si todavía no se entregó (Postfix sigue intentándolo) o si el proveedor lo rechazó, seguido de la respuesta del proveedor. Si el proveedor no responde en un minuto, el mensaje se queda en la cola y Postfix sigue intentándolo.

### Revisar el relay {#check-the-relay}

En la parte superior de la tarjeta:

- **Retransmitiendo a través de …** significa que Postfix está en ejecución y usa tu proveedor. **Postfix no está en ejecución** significa que está detenido.
- **… mensajes en espera** cuenta los mensajes que siguen en la cola. Postfix sigue reintentando un mensaje durante un máximo de 2 días.
- **Último problema de entrega:** muestra el último motivo por el que no se pudo entregar un mensaje, tal como lo dio el proveedor.

### Quitar el relay {#remove-the-relay}

Selecciona **Quitar** y confirma. El panel desinstala Postfix y olvida los ajustes. Los sitios web ya no pueden enviar correo.

## Alertas {#alerts}

Las alertas te envían un mensaje por Telegram, correo electrónico, Discord, Slack o tu propio webhook cuando el servidor requiere atención. Ve a **Configuración** → **Alertas**. Todos los canales son opcionales: configura tantos como quieras, y cada alerta va a todos a la vez. Un canal que falla no detiene a los demás.

El panel revisa el servidor cada minuto. Cada problema se notifica una vez, de nuevo cada 6 horas mientras dure, y una vez más cuando se resuelve. Cada mensaje empieza con el nombre del servidor entre corchetes. Las alertas por correo llevan el asunto "Server alert:" seguido del nombre del servidor.

### Qué envía una alerta {#what-sends-an-alert}

| Alerta | Se envía cuando | Predeterminado |
|---|---|---|
| **CPU por encima de (%)** | el uso de CPU promediado en 5 minutos alcanza el límite | 90 |
| **Memoria por encima de (%)** | el uso de memoria alcanza el límite | 90 |
| **Disco por encima de (%)** | el disco (`/`) está así de lleno | 90 |
| **Se detiene un servicio (Nginx, PHP, MariaDB…)** | un servicio que se inicia al arrancar no está en ejecución | activado |
| **Falla una copia de seguridad, una restauración o la renovación de un certificado** | falla una copia de seguridad, una restauración, la renovación del certificado de un sitio web o la renovación del certificado del panel | activado |
| Monitoreo de sitios web (la pestaña **Monitoreo** de cada sitio web) | un sitio web monitoreado está caído (2 comprobaciones fallidas seguidas), vuelve a estar en línea o sirve un certificado que vence en menos de 14 días | desactivado |
| **Inicios de sesión SSH** | alguien inicia sesión por SSH: cada vez, o solo desde una dirección nueva | **Sin alerta** |
| **Intentos SSH fallidos en 10 minutos** | fallaron esa cantidad de inicios de sesión SSH en 10 minutos | 0 (desactivado) |
| **Inicios de sesión en el panel** | alguien inicia sesión en el panel: cada vez, o solo desde una dirección nueva | **Sin alerta** |
| **Avisarme cuando la revisión diaria encuentre un problema nuevo** (en la página **Revisión de seguridad**) | la [revisión de seguridad](./security-check) diaria encuentra un problema crítico o un aviso nuevo | desactivado |
| **Avisarme cuando cambien archivos PHP** (la pestaña **Bloqueo de archivos** de un sitio web) | la comprobación diaria de un sitio web bloqueado encuentra archivos PHP cambiados, agregados o eliminados (consulta [Bloqueo de archivos](./website-protection#alerts-when-php-files-change)) | desactivado |

- Define un límite en 0 para desactivarlo. Los demás límites deben estar entre 10 y 100.
- Los servicios vigilados son el servidor web (Nginx, Apache u OpenLiteSpeed), PHP-FPM, MariaDB o MySQL, PostgreSQL, Redis, Memcached y fail2ban, cuando están instalados.
- No se comprueba nada hasta que haya al menos un canal configurado.
- El monitoreo de sitios web se activa para cada sitio web, y sus comprobaciones se ejecutan aunque no haya ningún canal: la pestaña **Monitoreo** muestra los resultados. Las alertas de caída se repiten cada 6 horas, como las demás. Consulta [Sitios web](./websites#monitoring).

### Alertas de inicio de sesión {#sign-in-alerts}

Recibe un mensaje cuando alguien inicia sesión en el servidor por SSH o en el panel, para que no pase desapercibido un inicio de sesión que no hiciste tú.

1. Ve a **Configuración** → **Alertas** y configura al menos un canal.
2. En **Inicios de sesión**, elige para **Inicios de sesión SSH** y para **Inicios de sesión en el panel**:
   - **Sin alerta**;
   - **Cada inicio de sesión**;
   - **Solo desde una dirección nueva**: un inicio de sesión desde una dirección desde la que la cuenta no inició sesión en los últimos 180 días.
3. Opcional: en **Intentos SSH fallidos en 10 minutos**, escribe cuántos intentos fallidos envían una alerta, de 3 a 100000. 0 la desactiva.
4. Haz clic en **Guardar**.

Una alerta de SSH indica la cuenta, la dirección, cómo inició sesión la cuenta y la hora, por ejemplo:

```
[server1] 🔑 SSH sign-in: admin from 203.0.113.10 (new address) with a key, 2026-11-27 10:31:05 +07
```

Varios inicios de sesión en el mismo minuto llegan en un solo mensaje. Una ráfaga de intentos fallidos nombra las direcciones que más lo intentaron:

```
[server1] ⚠️ 57 failed SSH sign-in attempts in the last 10 minutes, from 198.51.100.7 (40), 203.0.113.5 (17).
```

Cómo funciona:

- Nada vigila el registro de SSH todo el tiempo. Una vez por minuto, el panel lee lo que el servicio SSH (`ssh.service` o `sshd.service`) escribió en el diario del sistema desde la última lectura, y recuerda dónde se detuvo. Las alertas de SSH no cuestan nada mientras están desactivadas.
- Cuando activas las alertas de SSH, empiezan en ese momento: los inicios de sesión anteriores nunca se notifican. Las direcciones de los inicios de sesión SSH de los últimos 30 días cuentan como conocidas.
- Para el panel, las direcciones de los inicios de sesión del registro de actividad cuentan como conocidas.
- En IPv6, todas las direcciones de una misma /64 cuentan como una sola dirección, ya que la mayoría de los proveedores dan una /64 a cada cliente. Se recuerdan hasta 500 direcciones para SSH y 500 para el panel.
- Si el panel estuvo detenido más de una hora, al volver a arrancar solo lee la última hora del registro de SSH.
- Los intentos fallidos son las contraseñas incorrectas y los nombres de cuenta desconocidos. Con el inicio de sesión con contraseña desactivado, las claves rechazadas no se cuentan. Se envía como máximo una alerta de ráfaga por hora.
- Para bloquear las direcciones que siguen intentándolo, instala fail2ban en la página **Firewall y SSH**. Consulta [Protección contra fuerza bruta](./security#brute-force-protection-fail2ban).

### Recibir alertas por Telegram {#get-alerts-by-telegram}

1. En Telegram, abre BotFather, envía `/newbot` y responde a sus preguntas. Te responde con el token del bot: dígitos, dos puntos y luego una cadena larga de letras y dígitos.
2. Abre tu bot nuevo y envíale cualquier mensaje. Un bot no puede escribirte antes de que tú le escribas.
3. Obtén tu ID de chat, por ejemplo con userinfobot. Es un número.
4. En **Configuración** → **Alertas**, escribe el **Token del bot** y el **ID de chat**.
5. Selecciona **Guardar** y luego **Enviar una prueba**.

Para enviar alertas a un grupo o un canal, agrega antes el bot a él. El ID de chat de un grupo empieza con un signo menos. Para un canal público, también puedes escribir su `@name`.

Después de guardar, el token se muestra como puntos. Deja los puntos como están para conservar el token guardado. Si vacías el **ID de chat** y guardas, el token también se quita.

### Recibir alertas por correo electrónico {#get-alerts-by-e-mail}

Los correos de alerta se envían directamente desde el panel a través de un servidor SMTP, con sus propios ajustes, separados del relay de los sitios web. Puedes usar la misma cuenta del proveedor.

1. En **Correo electrónico**, escribe el **Servidor SMTP** y el **Puerto** (587 de forma predeterminada).
2. Escribe el **Nombre de usuario** y la **Contraseña** de la cuenta, si el servidor necesita inicio de sesión.
3. Escribe las direcciones **De** y **Para**.
4. Selecciona **Guardar** y luego **Enviar una prueba**.

En el puerto 465, la conexión va cifrada desde el principio. En otros puertos, el panel usa STARTTLS cuando el servidor lo ofrece. Si el servidor no lo ofrece y hay un nombre de usuario definido, el panel se niega a enviar, para que la contraseña nunca viaje en claro. La contraseña guardada se muestra como puntos, como el token de Telegram.

### Recibir alertas en Discord {#get-alerts-in-discord}

1. En Discord, abre los ajustes del canal que debe recibir las alertas y luego **Integrations** → **Webhooks** → **New Webhook**.
2. Si quieres, ponle un nombre al webhook y luego selecciona **Copy Webhook URL**.
3. En **Configuración** → **Alertas**, pégala en **URL del webhook**, en **Discord**.
4. Selecciona **Guardar** y luego **Enviar una prueba**.

La URL tiene la forma `https://discord.com/api/webhooks/<id>/<token>`. También funcionan las direcciones en `discordapp.com` y las versiones `ptb.` y `canary.`, y también `?thread_id=` para publicar en un hilo. Las alertas de más de 2000 caracteres, el límite de Discord, se recortan. Las menciones en una alerta, como `@everyone`, no notifican a nadie.

### Recibir alertas en Slack {#get-alerts-in-slack}

1. Ve a [api.slack.com/apps](https://api.slack.com/apps), selecciona **Create New App** → **From scratch** y elige tu espacio de trabajo.
2. Abre **Incoming Webhooks**, actívalo, luego selecciona **Add New Webhook to Workspace** y elige el canal.
3. Copia la **Webhook URL**. Empieza por `https://hooks.slack.com/services/`.
4. En **Configuración** → **Alertas**, pégala en **URL del webhook**, en **Slack**, selecciona **Guardar** y luego **Enviar una prueba**.

Solo se aceptan los webhooks entrantes: los webhooks de flujos de trabajo de Slack esperan otros campos.

### Enviar alertas a un webhook {#send-alerts-to-a-webhook}

El webhook publica cada alerta como JSON en una dirección que elijas: tu propio script, n8n, Home Assistant, un chat que no aparece aquí. En **Webhook**, escribe su **URL del webhook** y, si el receptor comprueba firmas, un **Secreto para firmar (opcional)** de 16 a 200 caracteres sin espacios. Luego selecciona **Guardar** y **Enviar una prueba**.

Cada alerta es un `POST` con `Content-Type: application/json` y este cuerpo:

```json
{"server":"vps-1","text":"⚠️ The disk is 93% full.","time":"2026-11-26T08:15:00Z","kind":"alert"}
```

| Campo | Valor |
|---|---|
| `server` | El nombre de host del servidor |
| `text` | La alerta, sin el nombre del servidor. Los problemas resueltos empiezan con "✅ Resolved:" |
| `time` | Cuándo se envió, en UTC, RFC 3339 |
| `kind` | `alert`, o `test` para **Enviar una prueba** |

Cualquier respuesta 2xx cuenta como entregada. El panel espera 10 segundos como máximo, no sigue redirecciones y no muestra la respuesta del receptor, solo su código de estado.

#### Verificar la firma {#verify-the-signature}

Con un secreto, cada solicitud lleva el encabezado `X-Nefo-Signature: sha256=` seguido del HMAC-SHA256 del cuerpo, en hexadecimal, con el secreto como clave. Calcúlalo sobre el cuerpo exactamente como se recibió, antes de analizar el JSON, y compara en tiempo constante. En Python:

```python
import hashlib, hmac

def verify(secret: bytes, body: bytes, signature: str) -> bool:
    expected = "sha256=" + hmac.new(secret, body, hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected, signature)
```

En Node.js, compara `'sha256=' + crypto.createHmac('sha256', secret).update(rawBody).digest('hex')` con el encabezado usando `crypto.timingSafeEqual`, después de comprobar que ambos tienen la misma longitud. El cuerpo firmado incluye `time`: rechaza las solicitudes que tengan más de unos minutos de antigüedad para impedir los ataques de repetición.

#### Direcciones que un webhook no puede usar {#webhook-addresses}

- La URL empieza por `https://`. Una dirección `http://` simple solo funciona después de marcar **Permitir una dirección http:// (sin cifrar, para un receptor en tu propia red)**: sin cifrado, la alerta y su firma se pueden leer por el camino.
- Sin nombre de usuario ni contraseña en la URL, sin `#`, y con 2048 caracteres como máximo.
- Para que el webhook no pueda llegar a los propios servicios del servidor, el panel nunca se conecta a direcciones de loopback (`127.0.0.0/8`, `::1`, `localhost`), direcciones de enlace local (`169.254.0.0/16`, `fe80::/10`), servicios de metadatos de la nube (`169.254.169.254`, `100.100.100.200`, `168.63.129.16`, `fd00:ec2::254`), `0.0.0.0/8` ni direcciones de multidifusión. Un nombre de host se vuelve a comprobar cada vez que se envía una alerta, sobre las direcciones a las que resuelve, así que también se rechaza un nombre que apunte a `127.0.0.1`.
- Se permiten las redes privadas (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `fd00::/8`), para un receptor en casa o en un contenedor.

### URL de webhook guardadas {#saved-webhook-urls}

Las URL de Discord, de Slack y del webhook contienen su token, así que después de guardar se muestran como puntos, como el token de Telegram, y lo mismo el secreto. Deja los puntos como están para conservar el valor guardado. Vacía una URL y guarda para dejar de usar ese canal. Vaciar la URL del webhook también quita su secreto y el permiso para http://.

### Enviar una alerta de prueba {#send-a-test-alert}

**Enviar una prueba** envía un mensaje de prueba a través de todos los canales guardados. Usa los ajustes guardados, así que selecciona antes **Guardar**. El resultado muestra una línea por canal, **Telegram**, **Correo electrónico**, **Discord**, **Slack** o **Webhook**, con **enviado** o el error. Si no hay ningún canal configurado, el panel dice "Configura primero un canal y guarda."

En Discord, Slack y el webhook, el error es uno de estos:

- **No se puede llegar al webhook** seguido del motivo, por ejemplo `no such host` o `connection refused`.
- **El webhook rechazó el mensaje (HTTP 404…)**: el receptor respondió con algo distinto de un estado 2xx. Discord y Slack agregan su motivo, como `Unknown Webhook` o `invalid_token`. Una redirección (3xx) también cuenta como rechazo.
- **El webhook no respondió en 10 segundos.**
- **… es una dirección de loopback, de enlace local o de metadatos de la nube**: consulta [Direcciones que un webhook no puede usar](#webhook-addresses).

Cuando no se puede enviar una alerta real, el registro del panel lo indica con el nombre del canal, nunca con su URL.

### Las alertas y el panel principal {#alerts-and-the-dashboard}

La lista **Requiere atención** del **Panel principal** y la campana de arriba a la derecha solo muestran los problemas mientras tienes la sesión iniciada, y comprueban otras cosas, como la autenticación de dos factores y el firewall. Las alertas te llegan cuando no estás. Consulta [Herramientas del sistema](./system#dashboard).
