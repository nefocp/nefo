---
title: Seguridad
description: Cómo protege Nefo su propio inicio de sesión, y cómo usar la revisión de seguridad, el firewall, fail2ban, los ajustes de SSH, el firewall de aplicaciones web, el bloqueo por país, el bloqueo de archivos y el escáner de malware.
---

# Seguridad

Nefo se ejecuta como root, así que primero protege su propio inicio de sesión: una dirección secreta en un puerto aleatorio, HTTPS desde el primer arranque, autenticación de dos factores, límites de inicio de sesión y un registro de actividad. También administra el firewall del servidor (ufw), fail2ban y los ajustes de SSH con protecciones para que no te quedes fuera, y protege los sitios web con un firewall de aplicaciones web, límites de solicitudes, direcciones bloqueadas y un escáner de malware.

Esta página trata de:

- la protección propia del panel: la dirección oculta, HTTPS, los límites de inicio de sesión, la autenticación de dos factores, las sesiones y el registro de actividad;
- la [revisión de seguridad](./security-check), que enumera los ajustes que dejan el servidor expuesto y dónde corregir cada uno;
- el firewall del servidor (ufw), fail2ban y los ajustes de SSH, en la página **Firewall y SSH**;
- la seguridad de los sitios web: el firewall de aplicaciones web, los límites de solicitudes, las direcciones bloqueadas, el bloqueo por país, el bloqueo de archivos y el escáner de malware.

Nefo está en beta y todavía no ha pasado una auditoría de seguridad externa. Consulta la [política de seguridad](../security-policy).

## Revisión de seguridad {#security-check}

**Revisión de seguridad**, en el grupo **Seguridad** del menú de la izquierda, ejecuta 22 comprobaciones rápidas del servidor, como el inicio de sesión SSH con contraseña, el firewall, las actualizaciones de seguridad pendientes, las bases de datos abiertas a internet, los puertos de Docker, los archivos que un sitio web sirve por error y los certificados. Cada hallazgo enlaza con la página que lo corrige. Se ejecuta cuando la inicias y una vez al día. Consulta [Revisión de seguridad](./security-check).

## La protección propia del panel {#the-panel-s-own-protection}

### Ruta de entrada oculta {#hidden-entrance-path}

El instalador elige un puerto aleatorio entre 20000 y 49999 y una ruta secreta de 12 letras minúsculas y dígitos. El panel solo responde bajo esa ruta, por ejemplo `https://203.0.113.10:28443/k3x9p2qa7m1z/`. Cualquier otra URL en ese puerto devuelve `404`, así que un escáner no puede saber que allí funciona un panel.

Si pierdes la dirección, ejecuta esto en el servidor:

```sh
nefo info
```

Muestra la URL con la primera dirección IP del servidor, y también con el dominio del panel cuando tiene uno (consulta más abajo). Si el servidor está detrás de una dirección pública aparte, ejecuta `nefo info --host 203.0.113.10` para mostrar la URL con ella.

El puerto y la ruta se guardan como `listen` y `entrance` en `/etc/nefo/config.json`. El panel no tiene ningún ajuste para cambiarlos. Para cambiar la ruta a mano, define `entrance` con 8 a 32 letras minúsculas y dígitos, y luego ejecuta `systemctl restart nefo`.

### HTTPS desde el primer arranque {#https-from-the-first-start}

El panel sirve HTTPS desde su primer arranque, con TLS 1.2 o más reciente. Crea un certificado autofirmado en `/var/lib/nefo/tls/`, así que tu navegador te avisa la primera vez. Para quitar el aviso, obtén un certificado gratuito de Let's Encrypt para un dominio que apunte a este servidor:

1. Instala el servidor web desde **Sitios web** si todavía no lo hiciste. Let's Encrypt comprueba el dominio en el puerto 80, y el servidor web le responde.
2. Apunta el registro A del dominio al servidor, sin un proxy ni una CDN.
3. Ve a **Configuración** → **Certificado**.
4. Escribe el **Dominio del panel**, por ejemplo `panel.example.com`, y selecciona **Obtener certificado**.
5. Abre el panel en `https://panel.example.com:28443/k3x9p2qa7m1z/`, con tu propio puerto y tu propia ruta.

El certificado se renueva solo, como los certificados de los sitios web; una renovación fallida se vuelve a intentar cada 12 horas. Para quitarlo, selecciona **Eliminar**: el panel vuelve a su certificado autofirmado. Consulta [Certificados SSL](./ssl) para los certificados en general.

### Límites de inicio de sesión {#sign-in-limits}

- 5 intentos fallidos en 15 minutos bloquean la dirección durante 15 minutos. La página de inicio de sesión dice entonces "Demasiados intentos fallidos. Inténtalo de nuevo en … segundos."
- Cuentan tanto una contraseña incorrecta como un código de dos factores incorrecto. También cuenta una contraseña actual incorrecta en los formularios de contraseña y de 2FA de la página **Cuenta**.
- Las direcciones IPv6 se cuentan por red /64, porque un usuario suele controlar una /64 completa.
- Un inicio de sesión correcto pone el contador a cero.
- Un nombre de usuario incorrecto y una contraseña incorrecta dan el mismo mensaje y tardan lo mismo, así que la página de inicio de sesión no revela qué nombres de usuario existen.
- El panel cuenta la dirección que se conecta a él e ignora `X-Forwarded-For`. Si pones un proxy delante del panel, todos los visitantes comparten un mismo contador.

Las contraseñas se guardan como hashes bcrypt, nunca en texto simple.

::: tip
Si te quedas bloqueado, espera 15 minutos. El contador se guarda en memoria, así que reiniciar el panel en el servidor (`systemctl restart nefo`) también lo pone a cero.
:::

### Sesiones y cierre de sesión {#sessions-and-signing-out}

- Un inicio de sesión dura 12 horas. Después vuelves a iniciar sesión, aunque hayas estado activo.
- Para cerrar sesión, abre el menú de la cuenta arriba a la derecha y selecciona **Cerrar sesión**.
- Cambiar tu contraseña cierra tu sesión en todos los dispositivos. Lo mismo hace `nefo reset-password` en el servidor.
- Los scripts no pueden leer la cookie de sesión, que solo se envía por HTTPS y solo a la ruta propia del panel.

### Encabezados de seguridad {#security-headers}

Cada página se envía con una Content Security Policy estricta que solo permite scripts del propio panel. El panel también envía `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer` (que mantiene la ruta secreta fuera de los registros de otros sitios web) y una `Permissions-Policy` que desactiva la cámara, el micrófono y la ubicación. Cada cambio necesita un encabezado de solicitud que otros sitios web no pueden enviar, lo que bloquea la falsificación de solicitudes entre sitios.

## Autenticación de dos factores {#two-factor-authentication}

La autenticación de dos factores (2FA) pide un código de 6 dígitos de una aplicación de autenticación en cada inicio de sesión, así que una contraseña robada no basta. Funciona cualquier aplicación que admita los códigos estándar basados en el tiempo, como Google Authenticator, Authy o 2FAS. Mientras la 2FA está desactivada, el panel muestra "La autenticación de dos factores está desactivada" como un problema grave.

### Activar la 2FA {#turn-on-2fa}

1. Ve a **Cuenta** en el menú de la izquierda. También puedes abrir el menú de la cuenta y seleccionar **Contraseña y 2FA**.
2. En **Autenticación de dos factores**, selecciona **Configurar**.
3. Escanea el código QR con tu aplicación, o escribe en ella la **Clave de configuración**. La aplicación muestra la cuenta como Nefo.
4. Escribe el código de 6 dígitos que muestra la aplicación y selecciona **Activar**.

A partir de ahora, la página de inicio de sesión pide un **Código de dos factores** después de tu contraseña.

Los códigos cambian cada 30 segundos. El panel también acepta el código anterior y el siguiente, por si los relojes difieren un poco. Cada código funciona una sola vez: si se rechaza un código, espera al siguiente.

### Desactivar la 2FA {#turn-off-2fa}

En **Cuenta**, en **Autenticación de dos factores**, escribe tu **Contraseña** y un **Código** actual, y luego selecciona **Desactivar**.

### Si pierdes tu teléfono {#if-you-lose-your-phone}

Nefo no tiene códigos de respaldo. La recuperación se hace a través del servidor: inicia sesión por SSH como root y ejecuta:

```sh
nefo reset-password --disable-2fa
```

Esto define una contraseña aleatoria nueva para la cuenta `admin`, desactiva la 2FA, cierra todas las sesiones y muestra la contraseña nueva. Inicia sesión con ella, cambia la contraseña y vuelve a configurar la 2FA. Consulta [Línea de comandos](./cli).

::: warning
Sin tu teléfono, el acceso root al servidor es la única forma de volver a entrar en el panel. Guarda a salvo tu clave SSH o tu contraseña de root, o el acceso a la consola web de tu proveedor.
:::

## Cambiar la contraseña del panel {#change-the-panel-password}

1. Ve a **Cuenta** → **Cambiar contraseña**.
2. Escribe tu **Contraseña actual** y luego la **Contraseña nueva** dos veces (**Repite la contraseña nueva**). Debe tener de 12 a 72 caracteres.
3. Selecciona **Cambiar contraseña**.

Tu sesión se cierra en todos los dispositivos. Vuelve a iniciar sesión con la contraseña nueva.

## Registro de actividad {#activity-log}

**Cuenta** → **Actividad reciente** muestra las 100 entradas más recientes, con la hora, la acción, el usuario, la dirección IP y el detalle. Muestra **Todos** los registros o solo los **Fallos**, y escribe en el cuadro de filtro para buscar. La parte superior de la página muestra tu último inicio de sesión y cuántos inicios de sesión fallidos contiene el registro.

El registro anota:

- los inicios de sesión, los cierres de sesión y los inicios de sesión fallidos, con el motivo (nombre de usuario o contraseña incorrectos, o código 2FA incorrecto);
- los cambios de contraseña, los restablecimientos de contraseña desde la línea de comandos, y la activación o desactivación de la 2FA;
- los cambios hechos en el panel: sitios web, certificados, bases de datos, archivos, copias de seguridad, tareas cron, servicios, reglas de firewall, ajustes y claves de SSH, software y Docker;
- la apertura y el cierre de la terminal, con cuánto tiempo estuvo abierta;
- los reinicios del panel y del servidor.

Las renovaciones automáticas de certificados se anotan con el usuario `system`. Las entradas se conservan 90 días. Cada entrada también se escribe en el registro del servicio del panel, que puedes leer con `journalctl -u nefo`. fail2ban lee de ahí los inicios de sesión fallidos.

## Firewall (ufw) {#firewall-ufw}

Nefo administra el firewall del servidor con ufw. Ve a **Firewall y SSH** → **Firewall (ufw)**.

Si la página dice que ufw no está instalado, instálalo en el servidor. El panel no lo instala:

```sh
apt install ufw
```

### Activar el firewall {#turn-on-the-firewall}

1. Selecciona **Activar el firewall** y confirma.
2. El panel primero permite el puerto SSH (comentario `SSH`) y el puerto del panel (comentario `Nefo`). Cuando hay un servidor web instalado, también permite los puertos 80 y 443 para los sitios web (comentario `Websites`), y 443/UDP cuando OpenLiteSpeed responde por HTTP/3. Luego activa ufw. Agrega estas reglas aunque hayas agregado una regla para uno de estos puertos mientras el firewall estaba desactivado, por ejemplo una limitada a tu dirección, así que activar el firewall nunca te deja fuera. Para conservar solo tu regla, elimina después la que vale para todos (consulta el final de [Lo que rechaza el firewall](#what-the-firewall-refuses)).

Puedes [agregar reglas](#add-a-rule) antes de activar el firewall, por ejemplo para un puerto que necesita otro programa del servidor. Mientras el firewall está desactivado, la página las muestra debajo de **Activar el firewall** como comandos de ufw, como `ufw allow 8080/tcp`, en la columna **Regla (comando ufw)**, con cada par IPv4 e IPv6 mostrado una sola vez. Entran en vigor cuando activas el firewall. ufw solo numera sus reglas mientras está activado, así que para eliminar una de ellas, primero activa el firewall.

Todos los demás puertos de entrada quedan cerrados salvo que una regla los permita. Si instalas el servidor web más adelante, con el firewall activado, Nefo abre en ese momento los puertos 80 y 443.

Si tu proveedor tiene un firewall en la nube (grupo de seguridad), abre allí también los mismos puertos. Nefo no puede modificarlo.

### Agregar una regla {#add-a-rule}

1. Selecciona **Agregar regla**.
2. Elige **Permitir** o **Denegar**.
3. En **Puerto, p. ej. 8080**, escribe un puerto, o un rango como `6000:6010`.
4. Elige el **Protocolo**: TCP, UDP, o TCP + UDP. Un rango necesita un solo protocolo: con TCP + UDP se convierte en TCP.
5. En **Desde**, escribe una dirección IP o una red como `198.51.100.0/24`. Déjalo vacío para permitir o denegar a todos.
6. Si quieres, agrega un **Comentario** de hasta 60 caracteres, sin comillas.
7. Selecciona **Agregar**.

La página muestra tu propia dirección IP encima de la lista, lo que ayuda cuando permites un puerto solo para ti.

ufw revisa las reglas de arriba abajo y usa la primera que coincide. Una regla **Denegar** con una dirección en **Desde** se pone al principio de la lista, para que surta efecto incluso en un puerto que una regla posterior permite para todos. Las demás reglas se agregan al final.

::: tip
Para mantener una dirección alejada de un solo sitio web, usa en su lugar los ajustes de **Bloquear acceso** del sitio web (consulta más abajo).
:::

La lista también muestra las reglas que agregaste con ufw en la línea de comandos. **Limitar** permite un puerto pero rechaza, durante un tiempo, una dirección que abre 6 conexiones en 30 segundos (`ufw limit`). **Rechazar** rechaza como **Denegar**, pero se lo comunica de inmediato al otro lado.

### Eliminar una regla {#delete-a-rule}

Selecciona el icono de la papelera al final de la fila de la regla y confirma. Si las reglas cambiaron desde que abriste la página, el panel se niega y te pide que recargues, así que nunca elimina la regla equivocada.

### Lo que rechaza el firewall {#what-the-firewall-refuses}

Estas protecciones evitan que te quedes fuera:

- Una regla que permite el puerto SSH o el puerto del panel muestra **protegida** cuando es tu última vía de entrada desde la dirección que usas ahora (que aparece encima de la lista). No se puede eliminar aquí. Cuando otra regla también deja entrar a tu dirección, la regla se puede eliminar.
- Se rechaza una regla **Denegar** para el puerto SSH o el puerto del panel con **Desde** vacío. Se permite una regla **Denegar** para una dirección o una red concreta.

Para que solo tu propia dirección llegue al panel: agrega una regla **Permitir** para el puerto del panel con tu dirección en **Desde**, y luego elimina la regla que permite el puerto desde cualquier lugar (y su copia IPv6). Si tu dirección cambia más adelante, usa la consola web de tu proveedor para ejecutar `ufw allow <port>/tcp`.

Las reglas para el acceso remoto a bases de datos (el puerto 3306 para MariaDB y MySQL; el puerto del clúster, 5432 de forma predeterminada, para PostgreSQL) se agregan y se quitan desde la página **Bases de datos**. Cámbialas allí. Consulta [Bases de datos](./databases#allow-remote-access) y [PostgreSQL](./postgresql#postgresql-remote-access).

## Protección contra fuerza bruta (fail2ban) {#brute-force-protection-fail2ban}

fail2ban bloquea las direcciones que fallan una y otra vez al iniciar sesión, en SSH y en el panel.

### Instalar fail2ban {#install-fail2ban}

1. Ve a **Firewall y SSH** → **Protección contra fuerza bruta (fail2ban)**.
2. Selecciona **Instalar fail2ban**. La instalación se ejecuta en segundo plano con un registro en vivo.

Nefo configura dos jails en `/etc/fail2ban/jail.d/panel.conf`:

| Jail | Vigila | Bloquea tras | Tiempo de bloqueo |
|---|---|---|---|
| `sshd` | Los inicios de sesión SSH, incluido SFTP | 5 fallos en 10 minutos | 1 hora |
| `panel` | Los inicios de sesión en el panel que fallan, o que se rechazan mientras la dirección está bloqueada | 10 en 10 minutos | 1 hora |

Una dirección bloqueada no puede llegar al puerto que protege el jail. El propio límite de inicio de sesión del panel (5 fallos en 15 minutos) actúa primero. Una dirección que sigue intentándolo mientras está bloqueada queda entonces bloqueada en el firewall por el jail `panel`.

El panel reescribe este archivo, y su filtro en `/etc/fail2ban/filter.d/panel.conf`, así que no los edites a mano. Cuando cambias el puerto SSH, el jail `sshd` lo sigue.

### Ver y desbloquear direcciones {#see-and-unblock-addresses}

La tabla enumera cada **Servicio protegido**, sus **Fallos recientes** y las direcciones **Bloqueadas ahora**, con el número total de bloqueos. Para desbloquear una dirección, selecciona **Desbloquear** junto a ella.

Si fail2ban está instalado pero detenido, inícialo en la página **Servicios**.

::: tip
Si te bloqueaste el acceso por SSH, inicia sesión en el panel desde tu red actual y selecciona **Desbloquear**. También puedes ejecutar `fail2ban-client set sshd unbanip 203.0.113.10` desde la **Terminal** del panel.
:::

## Ajustes de SSH {#ssh-settings}

Ve a **Firewall y SSH** → **SSH**.

| Ajuste | Opciones |
|---|---|
| **Puerto** | de 1 a 65535 |
| **Inicio de sesión de root** | **Solo con una clave SSH (recomendado)**, **Con una clave o una contraseña**, **No permitido** |
| **Permitir el inicio de sesión con contraseña** | Activado o desactivado, para todas las cuentas |

Selecciona **Guardar**. El panel escribe sus ajustes en `/etc/ssh/sshd_config.d/00-panel.conf`, que tiene prioridad sobre los demás archivos de ajustes de SSH. También desactiva el inicio de sesión keyboard-interactive. Antes de reiniciar SSH, prueba los ajustes nuevos con `sshd -t`. Si la prueba falla, vuelve el archivo anterior y no cambia nada.

::: warning
Mantén abierta tu sesión SSH actual después de guardar. Abre una conexión nueva y comprueba que puedes iniciar sesión antes de cerrar la anterior.
:::

### Lo que rechazan los ajustes de SSH {#what-the-ssh-settings-refuse}

- Ajustes con los que ya nadie podría iniciar sesión por SSH. Por ejemplo: desactivar el inicio de sesión con contraseña mientras ninguna cuenta tiene una clave SSH; el inicio de sesión de root **Solo con una clave SSH** mientras root no tiene clave; o root en **No permitido** mientras ninguna otra cuenta puede iniciar sesión.
- Un puerto que ya está ocupado: el propio puerto del panel, 80, 443, o cualquier puerto en el que escuche otro programa.
- Un puerto nuevo cuando SSH se inicia mediante la activación por socket de systemd. La página muestra entonces **activado por socket** y el campo del puerto queda bloqueado. Cambia el puerto en la línea de comandos.

Para decidir si alguien todavía podría iniciar sesión, el panel revisa root y las cuentas de personas (UID 1000 o más) con una shell de inicio de sesión: sus claves en `~/.ssh/authorized_keys` (o en los archivos definidos por `AuthorizedKeysFile`), sus contraseñas, y `AllowUsers`, `DenyUsers`, `AllowGroups` y `DenyGroups`. No lee los bloques `Match`. La página enumera quién puede iniciar sesión ahora, y cómo (clave o contraseña). Cuando root no tiene clave, también muestra una advertencia con un botón **Agregar clave** (consulta [Claves SSH](#ssh-keys)).

SSH no se puede detener desde la página **Servicios**, solo reiniciar o recargar.

::: warning
Con el inicio de sesión de root en **No permitido**, necesitas otra cuenta que pueda usar `sudo`; si no, podrás iniciar sesión, pero no administrar el servidor por SSH. Nefo no crea esa cuenta, ni comprueba que pueda usar `sudo`.
:::

### Cambiar el puerto SSH {#change-the-ssh-port}

1. Escribe el **Puerto** nuevo, selecciona **Guardar** y confirma.
2. Si el firewall está activado, el panel abre el puerto nuevo (comentario `SSH`) antes de que SSH pase a usarlo. Si fail2ban está instalado, su jail `sshd` también pasa al puerto nuevo.
3. Si tu proveedor tiene un firewall en la nube, abre allí también el puerto nuevo.
4. Prueba una conexión nueva, por ejemplo `ssh -p 2222 root@203.0.113.10`.
5. La regla de firewall del puerto anterior se queda, y ya no está protegida. Elimínala en **Firewall (ufw)** cuando el puerto nuevo funcione.

Las cuentas SFTP de tus sitios web se conectan por el mismo puerto.

### Claves SSH {#ssh-keys}

**Claves SSH**, debajo de los ajustes de SSH, enumera las claves públicas de root y de las cuentas de personas (UID 1000 o más, con una shell de inicio de sesión), agrupadas por cuenta. Nefo las lee de `~/.ssh/authorized_keys`, o de los archivos definidos por `AuthorizedKeysFile`. Cada clave muestra su tipo y tamaño (como `ed25519` o `rsa 4096`), su comentario, su huella SHA256 y sus opciones si las tiene, como `from="…"` o `command="…"`. Una clave DSA, o una clave RSA de menos de 2048 bits, se marca como **débil**: las versiones actuales de OpenSSH las rechazan. Agrega una clave nueva y luego quita la débil.

Las cuentas SFTP de tus sitios web no aparecen: sus claves están en los ajustes de SFTP de cada sitio web.

#### Agregar una clave {#add-a-key}

1. En tu computadora, crea un par de claves si todavía no tienes uno. Esto funciona igual en PowerShell en Windows 10 y posteriores:
   ```sh
   ssh-keygen -t ed25519
   ```
   La clave privada va a `~/.ssh/id_ed25519` y se queda en tu computadora. La clave pública va a `~/.ssh/id_ed25519.pub`.
2. Muestra la clave pública y copia la línea completa, que empieza por `ssh-ed25519`:
   ```sh
   cat ~/.ssh/id_ed25519.pub
   ```
3. En el panel, en **Claves SSH**, selecciona **Agregar clave**.
4. Elige la **Cuenta**, pega la línea en **Clave pública** y selecciona **Agregar clave**.
5. Pruébala desde una terminal nueva, por ejemplo `ssh root@203.0.113.10`.

Nefo agrega la clave al final del `~/.ssh/authorized_keys` de la cuenta. Cuando falta la carpeta o el archivo, los crea, con la cuenta como propietaria y los modos 700 y 600. Rechaza:

- una clave privada, como una que empieza por `-----BEGIN OPENSSH PRIVATE KEY-----`. Si pegaste una clave privada en algún lugar por error, crea un par de claves nuevo;
- más de una línea. En PuTTYgen, copia el cuadro **Public key for pasting into OpenSSH authorized_keys file**;
- las claves DSA, y las claves RSA de menos de 2048 bits;
- una clave que la cuenta ya tiene;
- un archivo `authorized_keys` o una carpeta `.ssh` que sea un enlace simbólico, o que no sea un archivo o una carpeta normal. Una cuenta podría hacer que apunte a otro archivo del servidor, así que Nefo ni lo lee ni lo cambia.

#### Quitar una clave {#remove-a-key}

Selecciona el icono de la papelera junto a la clave y confirma. La confirmación nombra la clave por su tipo, su huella y su comentario. Si con los ajustes actuales nadie podría iniciar sesión por SSH sin esta clave, el panel se niega: agrega antes otra clave, o permite el inicio de sesión con contraseña.

Las claves agregadas y quitadas se anotan en el registro de actividad, con la cuenta y la huella de la clave.

### Desactivar el inicio de sesión con contraseña {#turn-off-password-sign-in}

1. En [Claves SSH](#ssh-keys), agrega tu clave pública para root, o para la cuenta con la que inicias sesión.
2. Abre una terminal nueva e inicia sesión con la clave. Si SSH pide una contraseña, la clave no se está usando: comprueba que la agregaste a la cuenta con la que inicias sesión.
3. Desactiva **Permitir el inicio de sesión con contraseña** y selecciona **Guardar**. Si nadie tiene una clave, el panel se niega (consulta [Lo que rechazan los ajustes de SSH](#what-the-ssh-settings-refuse)).
4. Mantén abierta tu sesión actual, y comprueba una vez más una conexión nueva antes de cerrarla.

Las cuentas SFTP de tus sitios web conservan su propio método de inicio de sesión, contraseña o clave, aunque el inicio de sesión con contraseña esté desactivado para SSH.

Mientras root pueda iniciar sesión con contraseña, el panel muestra "Root puede iniciar sesión por SSH con contraseña" como un problema grave.

## Seguridad de los sitios web {#website-security}

Cada sitio web tiene sus propios ajustes de seguridad. Ve a **Sitios web**, abre el sitio web seleccionando su nombre o **Ajustes**, y usa las pestañas del grupo **Seguridad**.

### Firewall de aplicaciones web {#web-application-firewall}

El firewall de aplicaciones web (WAF) es ModSecurity con el OWASP Core Rule Set, ambos gratuitos y de código abierto. Revisa cada solicitud y detiene la inyección SQL, el cross-site scripting (XSS), la inclusión de archivos, los escáneres conocidos y otros ataques comunes. Funciona con Nginx, Apache y OpenLiteSpeed.

1. Instálalo una vez: **Software** → **Firewall web (ModSecurity)** → **Instalar**. Los sitios web mantienen el firewall desactivado hasta que lo actives. Cuando la distribución no tiene un módulo de ModSecurity para el servidor web, la tarjeta lo indica y no ofrece la instalación: Ubuntu 22.04 no tiene ninguno para Nginx, aunque allí funcionan Apache y OpenLiteSpeed, y Nginx funciona a partir de Ubuntu 24.04.
2. Ve a **Sitios web** → (sitio) → **Firewall web (WAF)**.
3. Elige un modo:
   - **Desactivado**
   - **Solo detectar: registrar ataques, no bloquear nada**
   - **Bloquear ataques**: el visitante recibe HTTP 403.
4. Selecciona **Guardar**.

Empieza con solo detectar durante unos días, y revisa los eventos en busca de falsas alarmas antes de bloquear. Los sitios web que ejecutan WordPress o usan el preajuste de reescritura de WordPress o de Drupal reciben automáticamente las excepciones del conjunto de reglas para esas aplicaciones.

**Eventos recientes del firewall** enumera las 50 solicitudes más recientes que las reglas marcaron en este sitio web: la hora, la dirección IP, la solicitud, las reglas que coincidieron, y si se marcó como **bloqueada** o solo como **registrada**. Los eventos vienen de `/var/log/panel-waf/audit.log`, que se rota a diario y se conserva 7 días.

Solo se inspeccionan los primeros 13 MB de una solicitud, así que las subidas grandes siguen funcionando. Si quitas el WAF en la página **Software**, cada sitio web conserva su modo, que se vuelve a aplicar cuando lo reinstalas.

### Límite de solicitudes y direcciones bloqueadas {#rate-limit-and-blocked-addresses}

Ve a **Sitios web** → (sitio) → **Bloquear acceso**.

- **Límite de solicitudes (solicitudes por segundo por visitante, 0 = desactivado)**: solo cuenta las solicitudes de PHP y de aplicaciones, no las imágenes, el CSS ni los scripts. Se permiten ráfagas de hasta cinco veces el límite; por encima, los visitantes reciben HTTP 429. 10 es adecuado para la mayoría de los sitios web, y el valor más alto es 1000. Solo Nginx tiene este ajuste: en Apache y OpenLiteSpeed no aparece.
- **Direcciones IP bloqueadas**: una dirección o un rango IPv4 o IPv6 por línea, como `203.0.113.7` o `198.51.100.0/24`, hasta 200 por sitio web. Los visitantes bloqueados reciben HTTP 403.

Selecciona **Guardar**. Ambos funcionan por sí solos, sin el firewall de aplicaciones web, y en todos los tipos de sitio web, incluidos los sitios web de proxy inverso. La misma pestaña tiene **Protección con contraseña**, que se describe en [Sitios web](./websites#suspend-redirect-or-password-protect-a-website).

### Análisis de malware {#malware-scan}

El escáner busca web shells, puertas traseras, código oculto y scripts inyectados en los archivos de un sitio web. No cambia nada hasta que pones un archivo en cuarentena.

1. Ve a **Sitios web** → (sitio) → **Análisis de malware**.
2. Selecciona **Analizar ahora**. El análisis se ejecuta en segundo plano con un registro en vivo.

El escáner lee la carpeta del sitio web, como `/var/www/example.com`. Omite las carpetas `.git` y los archivos de más de 5 MB, y no sigue los enlaces simbólicos. Revisa los archivos PHP (también el código PHP en archivos con otros nombres), los archivos JavaScript y HTML, y las imágenes.

| Hallazgo | Gravedad |
|---|---|
| Web shell conocida | crítica |
| Ejecuta código enviado por el visitante | crítica |
| Llama a una función que nombra el visitante | crítica |
| Ejecuta comandos de shell enviados por el visitante | crítica |
| Ejecuta código oculto (codificado) | alta |
| Decodifica datos ocultos enviados por el visitante | alta |
| Construye una función a partir de la entrada del visitante | alta |
| Script de subida que deja al visitante elegir dónde van los archivos | alta |
| Inyección de script ofuscado | alta |
| Código PHP oculto en una imagen | alta |
| Ejecución de código mediante preg_replace /e | revisar |
| Datos codificados largos junto a código que los decodifica | revisar |
| Texto muy ofuscado junto a código que lo ejecuta | revisar |
| Archivo PHP en una carpeta de subidas | revisar |

Cada hallazgo muestra el archivo, la línea y un fragmento breve. No se informa de un archivo PHP en una carpeta de subidas que solo contiene un comentario, como el `index.php` "Silence is golden" de WordPress.

El panel conserva el último análisis de cada sitio web. No analiza de forma programada: inicia un análisis cuando quieras uno.

### Cuarentena y restauración {#quarantine-and-restore}

Selecciona **Poner en cuarentena** junto a un hallazgo para sacar el archivo del sitio web y moverlo a `/var/lib/panel-quarantine`, que solo puede leer root. El panel anota el propietario y los permisos del archivo.

En **Archivos en cuarentena**:

- **Restaurar** vuelve a poner el archivo donde estaba, con su propietario y sus permisos. Se rechaza si ahora existe un archivo en esa ruta, o si la carpeta ya no existe.
- **Eliminar** borra el archivo para siempre, después de que confirmes.

Cuando eliminas un sitio web, sus archivos en cuarentena también se eliminan.

::: warning
Poner en cuarentena un archivo que el sitio web necesita, como un archivo de un plugin, puede romper el sitio web. Revisa primero el fragmento, y restaura el archivo si era una falsa alarma.
:::

### Analizar con ClamAV {#scan-with-clamav}

ClamAV es un motor de análisis adicional y opcional.

1. Instala **Antivirus ClamAV** en la página **Software**. Sus bases de datos de virus se descargan en segundo plano durante unos minutos después de la instalación.
2. En **Análisis de malware**, marca **Analizar también con ClamAV (más lento, necesita unos 1.4 GB de memoria libre)** y selecciona **Analizar ahora**.

ClamAV necesita unos 1.4 GB de memoria libre mientras analiza, así que es adecuado para servidores con 2 GB o más. Si hay menos memoria libre, el panel omite ClamAV y lo indica en el informe. ClamAV se ejecuta con la prioridad de CPU más baja y omite los archivos de más de 25 MB. Sus hallazgos se marcan con gravedad alta.

### Bloqueo por país y bloqueo de archivos {#country-blocking-and-file-lock}

Cada sitio web también tiene el **Bloqueo por país**, que rechaza a los visitantes de los países que indiques o deja entrar solo a esos, y el **Bloqueo de archivos**, que deja el código del sitio web en solo lectura para su PHP y su cuenta SFTP, salvo las carpetas en las que tiene que escribir. Consulta [Bloqueo por país y bloqueo de archivos](./website-protection).

## Historial de inicios de sesión SSH {#ssh-sign-in-history}

La lista de inicios de sesión SSH y de intentos fallidos está en la página **Procesos**. Consulta [Inicios de sesión SSH](./system#ssh-sign-ins). Para recibir un mensaje cuando alguien inicia sesión por SSH o en el panel, o cuando los inicios de sesión SSH fallan una y otra vez, consulta [Alertas de inicio de sesión](./mail-alerts#sign-in-alerts).
