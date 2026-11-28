---
title: DNS (Cloudflare)
description: Administra los registros DNS de tus zonas de Cloudflare desde el panel, apunta los nombres de un sitio web a este servidor, agrega los registros MX, SPF y DMARC de tu proveedor de correo y vacía la caché de Cloudflare.
---

# DNS (Cloudflare)

Nefo no ejecuta un servidor DNS. Si el DNS de tu dominio está alojado en Cloudflare, Nefo administra sus registros a través de la API de Cloudflare con un token de API: agrega, cambia y elimina registros, apunta los nombres de un sitio web a este servidor después de una vista previa, agrega los registros MX, SPF y DMARC de tu proveedor de correo y vacía la caché de Cloudflare.

El token se guarda en **Configuración** → **Cloudflare**. Después:

- la página **DNS**, en el grupo **Alojamiento** del menú de la izquierda, enumera tus zonas y sus registros, agrega, cambia y elimina registros, y vacía la caché de Cloudflare;
- **Apuntar el DNS aquí…**, en la pestaña **Dominios** de un sitio web, apunta los nombres del sitio web a este servidor en un solo paso;
- **Configurar el correo**, en la página DNS, agrega los registros MX, SPF y DMARC de un proveedor de correo;
- los certificados comodín se validan con el mismo token: consulta [Certificados SSL](./ssl#wildcard-certificates-through-cloudflare-dns).

Nefo solo se comunica con la API de Cloudflare, y el token nunca aparece en los registros ni en los mensajes del panel.

## El token de API {#the-api-token}

Un solo token sirve para todo esto. Necesita estos permisos, para las zonas (dominios) que usas en este servidor:

| Permiso | Para qué sirve |
|---|---|
| Zone → DNS → Edit | Enumerar, agregar, cambiar y eliminar registros, **Apuntar el DNS aquí…** y la validación por DNS de los certificados. |
| Zone → Zone → Read | Enumerar las zonas con su estado y su plan. Sin él, la lista de zonas puede quedar vacía, o Cloudflare se niega a enumerar las zonas. |
| Zone → Cache Purge → Purge | **Vaciar caché** en la página DNS. |

1. En el panel de Cloudflare, abre **My Profile → API Tokens → Create Token**.
2. Parte de la plantilla **Edit zone DNS**. Da Zone → DNS → Edit.
3. Haz clic en **+ Add more** y agrega Zone → Zone → Read y Zone → Cache Purge → Purge.
4. En **Zone Resources**, incluye las zonas que usas aquí, o todas las zonas de la cuenta.
5. Crea el token, cópialo y guárdalo en Nefo: **Configuración** → **Cloudflare** → **Token de API** → **Comprobar y guardar**.

Cuando al token le falta un permiso, Nefo lo nombra, por ejemplo: "Cloudflare lo rechazó: al token de API le falta el permiso Zone → DNS → Edit para esta zona." Edita el token en Cloudflare (**My Profile → API Tokens**, luego **Edit** junto al token), agrega el permiso para la zona y guarda. Editar un token conserva su valor, así que no hay nada que cambiar en Nefo. Si eliminaste o rotaste el token, guarda el nuevo en **Configuración** → **Cloudflare**.

## La página DNS {#the-dns-page}

La página muestra una zona a la vez. Con varias zonas, elige una en la lista de arriba; junto a ella aparecen su estado (**Activa**, o **Pendiente** mientras los servidores de nombres del dominio todavía no son los de Cloudflare) y su plan.

La tabla enumera los registros de la zona: **Tipo**, **Nombre** (`@` es la propia zona, `www` es `www.example.com`), **Contenido**, **Proxy** y **TTL**. Fíltrala por tipo, o busca en los nombres y el contenido. Una zona con más de 5 000 registros muestra los primeros 5 000; usa el panel de Cloudflare para los demás.

Nefo puede agregar y cambiar registros A, AAAA, CNAME, TXT, MX y CAA. Los demás tipos, como NS o SRV, aparecen con "en Cloudflare": cámbialos en el panel de Cloudflare.

### Agregar o cambiar un registro {#add-or-change-a-record}

Haz clic en **Agregar registro**, o en **Editar** en la fila de un registro. El tipo de un registro no puede cambiar: elimínalo y agrega uno nuevo. Nefo comprueba el registro antes de enviarlo a Cloudflare:

| Tipo | Qué escribir |
|---|---|
| A | Una dirección IPv4, como `203.0.113.10`. |
| AAAA | Una dirección IPv6, como `2001:db8::10`. |
| CNAME | El nombre de host del que este nombre es un alias, como `example.com`. No puede apuntar a su propio nombre. |
| TXT | Una línea de texto, de hasta 2048 caracteres: texto simple, que Cloudflare divide en cadenas de 255 caracteres, o `"quoted strings"` (cadenas entre comillas) de hasta 255 caracteres cada una. Pon entre comillas todo el valor o nada de él. |
| MX | El nombre de host del servidor de correo, como `mail.example.com`, y una **Prioridad** de 0 a 65535 (el más bajo se prueba primero). |
| CAA | Una **Etiqueta**: `issue` o `issuewild` con el dominio de una CA, como `letsencrypt.org` (`;` no permite ninguna CA), o `iodef` con una dirección `mailto:` o `https://`. Los **Indicadores** son 0, o 128 para crítico. |

El **TTL** es **Automático** (lo elige Cloudflare) o un tiempo fijo de 1 minuto a 1 día.

### Con proxy o solo DNS {#proxied-or-dns-only}

Los registros A, AAAA y CNAME pueden ir **Con proxy** (la nube naranja de Cloudflare): los visitantes llegan entonces a Cloudflare, que pasa las solicitudes web a la dirección y la oculta. Los registros con proxy siempre tienen el TTL automático. **Solo DNS** (nube gris) responde con la propia dirección. Los demás tipos de registro siempre son solo DNS.

El proxy de Cloudflare solo deja pasar HTTP y HTTPS en los puertos 80 y 443 (y algunos más). Deja todo lo demás en solo DNS: SSH, el correo y el panel.

::: warning Deja el nombre del panel en solo DNS
Si abres el panel con un nombre como `panel.example.com`, deja ese registro en **Solo DNS**. El proxy de Cloudflare no deja pasar el puerto del panel, así que con la nube naranja el panel ya no se abre con ese nombre. La página DNS y **Apuntar el DNS aquí…** te avisan cuando el registro tiene el proxy activado.
:::

## Apuntar un sitio web a este servidor {#point-a-website-to-this-server}

1. Abre **Sitios web** y luego la pestaña **Dominios** del sitio web.
2. En **DNS en Cloudflare**, haz clic en **Apuntar el DNS aquí…**.
3. Revisa las direcciones y las opciones:
   - **La dirección IPv4 pública de este servidor**, leída de las interfaces de red del servidor;
   - **Publicar también una dirección IPv6 (AAAA)**, marcada cuando el servidor tiene una dirección IPv6 pública;
   - **Con el proxy de Cloudflare (nube naranja)**.
4. Lee la vista previa: para cada nombre y tipo, el registro **Ahora** y **Después**, marcado como **nuevo**, **cambiado**, **quitado** o **sin cambios**.
5. Haz clic en **Aplicar**.

Nefo solo cambia los registros de los nombres del sitio web, su dominio y sus nombres de dominio adicionales; los registros de otros nombres nunca se tocan. Para cada nombre:

- un registro A apunta a este servidor: se actualiza un registro A existente o se crea uno nuevo, y se quitan los demás registros A del nombre;
- con IPv6, lo mismo ocurre con sus registros AAAA. Sin IPv6, se quitan los registros AAAA que apuntan a otro servidor, porque los visitantes por IPv6 y Let's Encrypt llegarían a ese servidor;
- un registro CNAME se reemplaza por el registro A, salvo que apunte a otro nombre del sitio web (`www` → `example.com` se queda como está);
- los nombres que no están en ninguna zona que el token pueda ver no se tocan, y la vista previa los enumera.

Cuando haces clic en **Aplicar**, Nefo vuelve a calcular los cambios. Si los registros cambiaron en Cloudflare desde la vista previa, no se cambia nada y Nefo muestra la vista previa nueva: "Los registros de Cloudflare cambiaron desde la vista previa."

Si el servidor está detrás de NAT y solo tiene una dirección IPv4 privada, Nefo dice "No se encontró ninguna dirección IPv4 pública en las interfaces de red de este servidor". Escribe la dirección pública que llega al servidor. Nefo no se la pregunta a ningún servicio externo.

### Let's Encrypt detrás de la nube naranja {#let-s-encrypt-behind-the-orange-cloud}

Let's Encrypt comprueba un sitio web por HTTP simple. Con el proxy activado, el ajuste **Always Use HTTPS** de Cloudflare (**SSL/TLS → Edge Certificates**) responde a esa comprobación con una redirección antes de que llegue al servidor, y el certificado falla. Puedes:

- desactivar **Always Use HTTPS** en Cloudflare y activar en su lugar [Redirigir HTTP a HTTPS](./ssl#redirect-http-to-https) en Nefo; o
- marcar **Validar mediante DNS (Cloudflare)** en la pestaña **SSL / HTTPS** del sitio web y dejar los ajustes de Cloudflare como están.

Cuando el sitio web tenga un certificado, pon el modo de cifrado **SSL/TLS** de Cloudflare en **Full (strict)**. Con **Flexible**, Cloudflare llega al servidor por HTTP, y **Redirigir HTTP a HTTPS** lo devuelve a HTTPS en un bucle.

## ¿Cómo configuro los registros DNS de mi correo? {#set-up-email-records}

Nefo no aloja buzones (consulta [Correo y alertas](./mail-alerts)). Si tu correo lo aloja Google Workspace, Microsoft 365, Zoho Mail u otro proveedor, o lo reenvía Cloudflare Email Routing, Nefo puede agregar los registros DNS que necesita el proveedor:

| Registro | Qué hace |
|---|---|
| MX | Indica qué servidores reciben el correo del dominio. |
| SPF (un registro TXT) | Indica qué servidores pueden enviar correo en nombre del dominio. |
| DMARC (un registro TXT en `_dmarc`) | Indica a los servidores que reciben qué hacer con el correo que no pasa las comprobaciones SPF y DKIM, y adónde enviar los informes. |
| DKIM (un registro TXT o CNAME) | Publica la clave con la que tu proveedor firma el correo. |

No cambia nada hasta que hayas visto la lista de cambios y la hayas aplicado.

1. Abre **DNS** y elige la zona.
2. Haz clic en **Configurar el correo**.
3. En **Proveedor de correo**, elige **Google Workspace**, **Microsoft 365**, **Zoho Mail**, **Cloudflare Email Routing** u **Otro proveedor**.
4. Opcional: en **Dominio de correo**, escribe un nombre de la zona, como `mail` para `mail.example.com`. Déjalo vacío para la propia zona.
5. Opcional: escribe una **Dirección para informes DMARC (opcional)** y pega el registro DKIM si ya lo tienes (consulta más abajo).
6. Haz clic en **Mostrar los cambios**. La tabla enumera cada registro con su valor **Ahora** y **Después** y lo que le ocurre: nuevo, cambiado, quitado, sin cambios u otro proveedor.
7. Haz clic en el botón de aplicar, como **Aplicar 4 cambios**.

Si cambias un campo después de la vista previa, haz clic en **Volver a previsualizar** antes de aplicar. Si mientras tanto cambiaron los registros en Cloudflare, Nefo se niega y muestra la vista previa nueva.

### Lo que recibe cada proveedor {#what-each-provider-gets}

| Proveedor | Registros MX | Include de SPF |
|---|---|---|
| **Google Workspace** | `smtp.google.com`, prioridad 1 | `include:_spf.google.com` |
| **Microsoft 365** | `example-com.mail.protection.outlook.com` (tu dominio con guiones en lugar de puntos), prioridad 0 | `include:spf.protection.outlook.com` |
| **Zoho Mail** | `mx.zoho.com` (10), `mx2.zoho.com` (20), `mx3.zoho.com` (50), con el dominio del **Centro de datos de Zoho** elegido: zoho.com, zoho.eu, zoho.in o zoho.com.au | `include:zohomail.com`, o `zohomail.eu`, `zohomail.in`, `zohomail.com.au` |
| **Cloudflare Email Routing** | `route1.mx.cloudflare.net`, `route2.mx.cloudflare.net`, `route3.mx.cloudflare.net` | `include:_spf.mx.cloudflare.net` |
| **Otro proveedor** | Los servidores que escribas en **Servidores de correo (MX)**, hasta 5, cada uno con una prioridad | El dominio de **Include de SPF (opcional)** y las direcciones de **Direcciones de envío (opcional)**; si no hay ninguno de los dos, `mx` |

Algunos detalles:

- **Google Workspace**: si la zona ya tiene los registros MX antiguos de Google (`aspmx.l.google.com` y los demás), Nefo los conserva y no agrega nada.
- **Microsoft 365**: Microsoft da a cada dominio su propio host MX. Compara el valor de la vista previa con el que muestra el centro de administración de Microsoft 365 para el dominio. Si es distinto, elige **Otro proveedor** y escríbelo. Un registro MX que ya apunta a Microsoft 365 se conserva como está.
- **Zoho Mail**: elige el centro de datos en cuya dirección inicias sesión. Para otro centro de datos de Zoho, elige **Otro proveedor** y escribe los valores que muestra Zoho.
- **Cloudflare Email Routing**: el correo solo llega después de que activas Email Routing para la zona en el panel de Cloudflare y agregas allí una dirección de destino. Los registros de enrutamiento de Cloudflare que ya existen conservan las prioridades que les dio Cloudflare.

### SPF: siempre un solo registro {#spf-always-one-record}

Un dominio debe tener exactamente un registro SPF; con dos, la comprobación SPF falla. Nefo nunca agrega un segundo:

- Si la zona tiene un registro SPF, Nefo conserva todo lo que contiene, agrega el include del proveedor y conserva su terminación (`-all`, `~all` o `?all`). Un registro sin terminación recibe `~all`.
- Si la zona tiene varios registros SPF, Nefo los fusiona en uno: cambia el primero y quita los demás.
- Si configuraste un relay SMTP en **Configuración** → **Correo**, el registro SPF también permite al proveedor del relay (por ejemplo `include:sendgrid.net` para SendGrid, `include:mailgun.org` para Mailgun, `include:amazonses.com` para Amazon SES), para que el correo que envían tus sitios web pase la comprobación. La vista previa lo indica.
- Un registro SPF que necesita más de 10 consultas DNS (`include`, `a`, `mx`, `ptr`, `exists`, `redirect`) falla en muchos servidores de destino. La vista previa te avisa; quita los includes que ya no usas.

### Registros MX de otro proveedor {#mx-records-of-another-provider}

Los registros MX que apuntan a otro proveedor se marcan como "otro proveedor". El correo iría a los dos proveedores, así que tienen que desaparecer: marca **Reemplazar los registros MX existentes** para quitarlos. Nefo se niega a aplicar los cambios hasta que lo hagas, o hasta que quites tú mismo esos registros.

::: warning
El correo deja de llegar al proveedor anterior en cuanto se quitan sus registros MX. Traslada primero tus buzones.
:::

### DMARC {#dmarc}

Nefo agrega `v=DMARC1; p=none` en `_dmarc`, con `rua=mailto:` y tu dirección cuando escribes una **Dirección para informes DMARC (opcional)**. Con `p=none` no se rechaza ningún correo: los servidores que lo reciben solo envían informes. Cuando los informes muestren que todo tu correo pasa las comprobaciones, puedes endurecer la política (`p=quarantine` y luego `p=reject`) editando el registro en la página DNS.

Si la zona ya tiene un registro DMARC, Nefo lo deja como está.

### DKIM {#dkim}

El proveedor de correo genera la clave DKIM, así que Nefo no puede crearla. En **Firma DKIM**, el cuadro de diálogo indica dónde encontrarla en tu proveedor:

- **Google Workspace**: en la consola de administración de Google, abre Apps → Google Workspace → Gmail → Authenticate email y genera un registro nuevo. Su nombre es `google._domainkey` y su tipo, TXT.
- **Microsoft 365**: en el portal de Microsoft Defender, abre los ajustes de DKIM del dominio. Microsoft da dos registros CNAME, `selector1._domainkey` y `selector2._domainkey`; el cuadro de diálogo tiene una fila para cada uno.
- **Zoho Mail**: en la Zoho Mail Admin Console, abre los ajustes de DKIM del dominio y agrega un selector. Escribe el nombre del selector seguido de `._domainkey` y el valor TXT.
- **Cloudflare Email Routing** no necesita aquí ningún registro DKIM: Cloudflare agrega lo que necesita cuando activas Email Routing.
- **Otro proveedor**: pide al proveedor su registro DKIM.

Pega el **Nombre del registro** y el **Valor**, elige el **Tipo** (TXT o CNAME) y previsualiza. Un nombre como `google._domainkey` va bajo el dominio de correo; también se acepta un nombre completo. Un registro DKIM con el mismo nombre se reemplaza. Deja el valor vacío para agregar el registro DKIM más tarde: vuelve a usar **Configurar el correo**, con el mismo proveedor, cuando lo tengas. Luego termina la configuración en el proveedor (Google: **Start authentication**; Microsoft y Zoho: activa DKIM o verifícalo).

Volver a usar **Configurar el correo** no tiene riesgo: los registros que ya están bien aparecen como "sin cambios".

Cada registro que Nefo agrega, cambia o quita se anota en el registro de actividad, como los demás cambios de DNS.

## Vaciar la caché de Cloudflare {#purge-cloudflare-s-cache}

Con el proxy activado, Cloudflare guarda copias de tus archivos. Después de cambiarlos, haz clic en **Vaciar caché** en la página DNS:

- **Todo** vacía la caché de Cloudflare para toda la zona. Cloudflare vuelve entonces a descargar todos los archivos del servidor, así que los sitios web pueden ir más lentos durante unos minutos;
- **Estas URL** vacía hasta 30 direcciones de la zona a la vez, una por línea, como `https://example.com/style.css`.

Esta es la caché de Cloudflare, no la propia [caché de páginas](./websites#page-cache) del sitio web en este servidor.

## Registro de actividad {#activity-log}

Cada cambio se anota en el registro de actividad (**Cuenta** → **Actividad reciente**): "Se agregó un registro DNS", "Se cambió un registro DNS", "Se eliminó un registro DNS" y "Se vació la caché de Cloudflare", con el tipo, el nombre y el contenido del registro. **Apuntar el DNS aquí…** anota cada registro que cambia.

## Mensajes {#messages}

| Mensaje | Qué hacer |
|---|---|
| `No Cloudflare API token is saved.` | Guarda un token en **Configuración** → **Cloudflare**. |
| `Cloudflare refused: the API token lacks the permission …` | Agrega al token el permiso que nombra: consulta [El token de API](#the-api-token). |
| `Cloudflare did not accept this token.` | El token es incorrecto, caducó o se eliminó. Crea uno nuevo. |
| `None of these names is in a Cloudflare zone the API token can see` | Agrega la zona a los **Zone Resources** del token, o define los registros allí donde esté alojado el DNS del dominio. |
| `Cloudflare does not answer for example.com yet (pending).` | En tu registrador, cambia los servidores de nombres del dominio por los dos que muestra Cloudflare. |
| `Cloudflare refused: …` | El propio mensaje de Cloudflare, por ejemplo cuando un CNAME y un registro A compartirían un nombre. |
