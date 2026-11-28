---
title: Primeros pasos
description: Qué hacer después de instalar Nefo, desde la página de configuración inicial y la protección de tu cuenta hasta tu primer sitio web, las copias de seguridad y la revisión de seguridad.
---

# Primeros pasos

Después de [instalar Nefo](./install), inicia sesión con la dirección y la contraseña que mostró el instalador, elige el software en la página de configuración inicial, cambia la contraseña, activa la autenticación de dos factores, dale al panel su propio dominio, y luego crea tu primer sitio web, configura las copias de seguridad y ejecuta la revisión de seguridad. Cada paso es breve.

Sigue estos pasos en orden. Cada uno enlaza a una página con los detalles.

## Iniciar sesión {#sign-in}

1. Abre la URL que mostró el instalador, como `https://203.0.113.10:28443/k3x9p2qa7m1z/`.
2. Acepta el aviso de certificado del navegador. Desaparece cuando el panel tiene su propio certificado (consulta [más abajo](#give-the-panel-its-own-domain)).
3. Escribe `admin` en **Nombre de usuario** y la contraseña del instalador, y haz clic en **Iniciar sesión**.

Si perdiste la URL o la contraseña, ejecuta `nefo info` o `nefo reset-password` en el servidor (consulta [Línea de comandos](./cli)).

## Configurar el servidor {#set-up-the-server}

En un servidor nuevo, el panel abre **Configurar este servidor** justo después de que inicias sesión. En una sola página eliges lo que necesita el servidor, y el panel lo instala todo de una vez, una parte tras otra.

1. Elige un **Servidor web**: **Nginx** (recomendado), **Apache** para sitios web que necesitan archivos `.htaccess`, **OpenLiteSpeed**, o **Ninguno** para un servidor sin sitios web, como uno que solo ejecuta aplicaciones Docker.
2. Marca una o más **Versiones de PHP**. El servidor web siempre viene además con la versión de PHP de tu distribución. Las demás versiones vienen del repositorio `packages.sury.org/php`, en Ubuntu y Debian, que el panel agrega al servidor. Las versiones antiguas que ya llegaron al final de su vida útil no se ofrecen aquí: instálalas desde la página **Software** si un sitio web antiguo necesita una.
3. Elige el **Servidor de bases de datos**: **MariaDB** (recomendado), **MySQL** donde la página Software lo ofrezca, o **Ninguno**. En **MariaDB** puedes elegir su versión: la de tu distribución (la predeterminada) o una serie LTS del repositorio propio de MariaDB, como **11.4 LTS de MariaDB**, cuando es más reciente que la de tu distribución. Consulta [Versiones de MariaDB](./databases#mariadb-versions).
4. Marca los **Extras** que quieras: Redis, Node.js, Python, Docker, Adminer y el firewall web. ClamAV se ofrece en servidores con 2 GB de RAM o más, y PostgreSQL y Memcached donde la página Software los ofrezca.
5. Revisa el resumen de abajo. Muestra las CPU, la RAM y el disco libre del servidor, y aproximadamente cuánta RAM y cuánto disco agrega tu elección. Si la elección puede no caber, un aviso lo indica, pero igualmente puedes instalar.
6. Haz clic en **Instalar**.

La página empieza con una selección adecuada para la RAM del servidor: Nginx, la versión más reciente de PHP y MariaDB, más Redis a partir de unos 1.5 GB de RAM. Docker y los demás extras nunca se marcan por ti.

La cifra de disco cuenta lo que ocupan los paquetes una vez instalados, las copias de los paquetes descargados que apt guarda en `/var/cache/apt/archives` (**Paquetes descargados**, en **Limpieza** de la página **Sistema**, las elimina), los primeros archivos de datos del servidor de bases de datos y las bases de datos de virus de ClamAV. En un servidor cuyas listas de paquetes nunca se descargaron, cuenta también esas listas, y el resumen indica qué parte de la cifra son: unos 450 MB en Ubuntu 22.04, 300 MB en 24.04, 250 MB en 26.04, 155 MB en Debian 12 y 175 MB en Debian 13.

El registro muestra cada parte mientras se instala, y toda la configuración es una sola tarea. Si una parte falla, la configuración se detiene ahí y te dice qué ya está instalado. Corrige el problema y vuelve a hacer clic en **Instalar**: las partes ya instaladas se omiten.

Para configurar el servidor tú mismo, haz clic en **Omitir por ahora**. Puedes volver a ejecutar la configuración en cualquier momento desde **Software** → **Volver a ejecutar la configuración**, o con [`nefo setup`](./cli#nefo-setup) en el servidor. También puedes [configurar el servidor al instalar Nefo](./install#set-the-server-up-in-the-same-run).

El panel deja de abrir la configuración después del inicio de sesión cuando esta terminó o se omitió, y en un servidor que ya tiene un servidor web o un sitio web.

### Un VPS que es un contenedor {#a-vps-that-is-a-container}

Algunos VPS son contenedores (LXC, OpenVZ), y su `/proc/meminfo` puede mostrar la memoria de todo el host en lugar de la tuya. En ese caso, Nefo lee el límite propio del contenedor (el cgroup de memoria) y lo usa en todas partes: el panel principal, los gráficos del historial y las alertas, las cifras de RAM de esta configuración, las comprobaciones de memoria de la tienda de aplicaciones y los ajustes que elige para MariaDB, MySQL, PostgreSQL, Redis y Memcached. Cuando lo hace, el panel principal muestra "límite del contenedor" junto a la RAM. ClamAV se ofrece a partir de 2 GB de ese límite.

## Cambiar la contraseña {#change-the-password}

Reemplaza la contraseña del instalador por una que guardes en un gestor de contraseñas.

1. En el menú de la izquierda, abre **Cuenta**. También puedes abrirla desde el menú de la cuenta, arriba a la derecha: **Contraseña y 2FA**.
2. En **Cambiar contraseña**, completa **Contraseña actual**, **Contraseña nueva** y **Repite la contraseña nueva**. La contraseña nueva necesita de 12 a 72 caracteres.
3. Haz clic en **Cambiar contraseña**.

El panel cierra tu sesión en todos los dispositivos. Vuelve a iniciar sesión con la contraseña nueva.

## Activar la autenticación de dos factores {#turn-on-two-factor-authentication}

Con la autenticación de dos factores, para iniciar sesión necesitas tu contraseña y un código de 6 dígitos de una aplicación de autenticación, como Google Authenticator, Authy o 2FAS. Mientras no la actives, el panel principal y el menú de notificaciones muestran "La autenticación de dos factores está desactivada".

1. Abre **Cuenta**.
2. En **Autenticación de dos factores**, haz clic en **Configurar**.
3. Escanea el código QR con tu aplicación de autenticación, o escribe la **Clave de configuración**.
4. Escribe el código de 6 dígitos que muestra la aplicación y haz clic en **Activar**.

A partir de ahora, la página de inicio de sesión pide un **Código de dos factores** después de tu contraseña.

::: warning Guarda una forma de volver a entrar
Si pierdes tu teléfono, solo puedes desactivar la autenticación de dos factores desde el servidor, con `nefo reset-password --disable-2fa` (consulta [Solución de problemas](./troubleshooting#lost-your-2fa-phone)). Mantén tu acceso SSH en funcionamiento, o guarda la clave de configuración en un lugar seguro.
:::

## Instalar un servidor web y PHP {#install-a-web-server-and-php}

Los sitios web necesitan un servidor web. Nefo ejecuta uno por servidor. Si la [configuración inicial](#set-up-the-server) ya instaló uno, omite este paso.

1. Abre **Sitios web**. En un servidor nuevo muestra **Instalar el servidor web**.
2. Elige un servidor web:
   - **Nginx**: rápido y ligero, la mejor opción salvo que tus sitios web necesiten archivos `.htaccess`.
   - **Apache**: lee los archivos `.htaccess`, de los que dependen muchos plugins de WordPress y sitios que vienen de un hosting compartido. Usa un poco más de memoria.
   - **OpenLiteSpeed**: lee las reglas de reescritura de los archivos `.htaccess` y trae LiteSpeed Cache integrado. Viene del repositorio de paquetes propio de LiteSpeed.
3. Haz clic en el botón de instalación, como **Instalar Nginx**.

El panel instala el servidor web y la versión de PHP de tu distribución, y abre los puertos 80 y 443 si el firewall (ufw) está activado. Esto tarda de uno a tres minutos, y el progreso se ve en la página.

Para cambiar a otro servidor web más adelante, primero debes eliminar todos los sitios web. Agrega más versiones de PHP, MariaDB y otros componentes en la página **Software**. Consulta [Software](./software).

## Darle al panel su propio dominio {#give-the-panel-its-own-domain}

Con su propio dominio, el panel obtiene un certificado gratuito de Let's Encrypt y el aviso del navegador desaparece. Primero necesitas un servidor web, porque es el que responde a Let's Encrypt en el puerto 80.

1. En tu proveedor de DNS, crea un registro A para un nombre como `panel.example.com` que apunte a la dirección IP del servidor. Debe apuntar directamente al servidor, sin un proxy ni una CDN delante (con Cloudflare, usa "DNS only").
2. Asegúrate de que el puerto 80 esté abierto, también en el firewall en la nube de tu proveedor.
3. En el panel, abre **Configuración** → **Certificado**.
4. En **Dominio del panel**, escribe `panel.example.com` y haz clic en **Obtener certificado**.

El panel primero comprueba que el dominio llega a este servidor y luego le pide el certificado a Let's Encrypt. Empieza a usar el certificado nuevo en menos de un minuto y muestra la dirección nueva, como `https://panel.example.com:28443/k3x9p2qa7m1z/`. Usa esa dirección a partir de ahora: el panel mantiene el mismo puerto y la misma ruta secreta. Con la dirección IP, el navegador vuelve a avisar, porque el certificado es para el dominio.

El certificado se renueva automáticamente. Para volver al certificado autofirmado, haz clic en **Eliminar** en la misma página.

Si la solicitud falla, consulta [Solución de problemas](./troubleshooting#a-certificate-request-fails).

## Crear tu primer sitio web {#create-your-first-website}

1. Apunta el registro A del dominio, como `example.com`, a la dirección IP del servidor.
2. Abre **Sitios web** y haz clic en **Agregar sitio web**.
3. Escribe el **Dominio** sin `http://` ni `www.`. Deja marcada **Responder también en www.example.com** si quieres ambos nombres.
4. Elige el **Tipo**: una versión de PHP, **Estático (solo HTML)**, una aplicación (Node.js, Python o Java) o un proxy inverso a otra dirección.
5. Haz clic en **Crear sitio web**.
6. Haz clic en **Activar HTTPS** junto al sitio web para obtener un certificado gratuito.

Sube tus archivos a `/var/www/example.com/public`, con la página **Archivos** o por SFTP. Para instalar WordPress u otra aplicación, usa en su lugar la **Tienda de aplicaciones**. Consulta [Sitios web](./websites), [Certificados SSL](./ssl) y [Tienda de aplicaciones](./apps).

## Configurar las copias de seguridad {#set-up-backups}

1. Abre **Copias de seguridad**.
2. En **Copias de seguridad automáticas**, haz clic en **Agregar programación**.
3. Elige de qué hacer la copia de seguridad (un sitio web, una base de datos o una carpeta), con qué frecuencia, y el **Número de copias que se conservan**.
4. Haz clic en **Guardar programación**.

Una copia de seguridad de un sitio web contiene sus archivos y las bases de datos vinculadas a él. Las copias de seguridad se guardan en el servidor, en `/var/backups/panel`. Agrega un **Almacenamiento remoto** (compatible con S3, SFTP o Google Drive, cifrado si quieres) para que una copia sobreviva si se pierde el servidor. Consulta [Copias de seguridad](./backups).

## Activar el firewall {#turn-on-the-firewall}

Si el panel principal dice "El firewall está desactivado", cualquier puerto que abra un programa es accesible desde internet.

1. Abre **Firewall y SSH**.
2. Haz clic en **Activar el firewall**.

El panel mantiene abiertos SSH y el propio puerto del panel. Si la página dice que ufw no está instalado, instálalo primero con `apt install ufw`. Consulta [Seguridad](./security).

## Ejecutar la revisión de seguridad {#run-the-security-check}

1. Abre **Revisión de seguridad**, en el grupo **Seguridad** del menú de la izquierda.
2. Haz clic en **Ejecutar la revisión**.
3. Corrige lo que aparece en **Por corregir**, empezando por los **Críticos**: cada hallazgo tiene un botón que lleva a la página donde se corrige.

Después, la revisión se ejecuta sola una vez al día, y el panel principal muestra lo que encontró. Consulta [Revisión de seguridad](./security-check).
