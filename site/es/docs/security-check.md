---
title: Revisión de seguridad
description: La revisión de seguridad de Nefo encuentra los ajustes que dejan un servidor expuesto (SSH, firewall, actualizaciones, bases de datos abiertas, puertos de Docker, archivos expuestos, certificados, PHP) y enlaza cada hallazgo con su corrección, cuando la inicias o a diario.
---

# Revisión de seguridad

La revisión de seguridad, nueva en v0.24.0, ejecuta 22 comprobaciones rápidas del servidor: SSH que acepta contraseñas, un firewall desactivado, actualizaciones de seguridad pendientes, una base de datos o Redis abiertos a internet, puertos de Docker que se saltan ufw, archivos de sitios web que cualquiera puede descargar, certificados a punto de caducar y ajustes de PHP. Cada hallazgo tiene una gravedad y un botón que lleva a la página que lo corrige.

Se ejecuta cuando la inicias y, de forma predeterminada, una vez al día. No envía nada fuera de tu servidor. Abre **Revisión de seguridad** en el grupo **Seguridad** del menú de la izquierda.

## Ejecutar la revisión {#run-the-check}

1. Haz clic en **Ejecutar la revisión**. Tarda unos segundos.
2. Lee los hallazgos en la pestaña **Por corregir**. Cada uno tiene una gravedad:
   - **Crítico**: corrígelo ahora. Por ejemplo, root puede iniciar sesión por SSH con contraseña, o un sitio web sirve un volcado de una base de datos.
   - **Aviso**: corrígelo pronto. Por ejemplo, el firewall está desactivado o hay actualizaciones de seguridad pendientes.
   - **Nota**: conviene saberlo, y a menudo está bien así. Por ejemplo, SSH escucha en el puerto 22.
3. Haz clic en el botón de un hallazgo, como **Ir a Firewall y SSH**, para abrir el lugar donde se corrige. Luego vuelve a ejecutar la revisión.

Los contadores de arriba muestran los problemas **Críticos**, los **Avisos** y las **Notas** que siguen abiertos, y en **Superadas**, cuántas comprobaciones no encontraron nada. Haz clic en **Superadas** al final de la página para ver cada comprobación que no encontró nada que corregir. Las comprobaciones que no se aplican al servidor (no hay Redis, no hay sitios web, no hay Docker…) aparecen en "No se aplican a este servidor".

## ¿Qué comprueba la revisión de seguridad? {#what-is-checked}

| Comprobación | Gravedad | Dónde se corrige |
|---|---|---|
| root puede iniciar sesión por SSH con contraseña | Crítico | **Firewall y SSH** → SSH |
| SSH acepta contraseñas para otras cuentas | Aviso | **Firewall y SSH** → SSH |
| SSH escucha en el puerto 22 | Nota | **Firewall y SSH** → SSH |
| El firewall (ufw) no está instalado o está desactivado | Aviso | **Firewall y SSH** |
| fail2ban no está instalado, no está en ejecución o no vigila SSH (no hay jail `sshd`) | Aviso | **Firewall y SSH** → fail2ban, o **Servicios** |
| Se accede al panel sin HTTPS | Crítico | `/etc/nefo/config.json` (consulta más abajo) |
| El panel usa su certificado autofirmado | Nota | **Configuración** |
| Una cuenta del panel no tiene autenticación de dos factores | Crítico | **Cuenta** |
| Hay actualizaciones de seguridad pendientes | Aviso | **Sistema** |
| La búsqueda de actualizaciones tiene más de una semana, o nunca se ejecutó | Nota | **Sistema** |
| Las actualizaciones de seguridad automáticas están desactivadas | Aviso | **Sistema** |
| Hace falta reiniciar para terminar las actualizaciones | Nota | **Configuración** |
| Una cuenta que no es root tiene el ID de usuario 0 | Crítico | **Terminal** |
| Una cuenta tiene la contraseña vacía | Crítico | **Terminal** (`passwd -l <account>`) |
| MariaDB, MySQL, PostgreSQL, Redis, Valkey, Memcached o MongoDB escucha en una dirección que no es 127.0.0.1 | Crítico cuando el firewall deja que cualquiera llegue al puerto; si no, Nota | **Bases de datos** |
| Redis no tiene contraseña | Crítico en una dirección pública que el firewall deja pasar, Aviso en otra dirección pública, Nota cuando solo escucha en este servidor | **Bases de datos** → Redis |
| Un contenedor de Docker publica puertos en todas las direcciones | Aviso | **Docker** |
| Un sitio web no tiene HTTPS | Aviso | **Sitios web** → (sitio) → **SSL / HTTPS** |
| Un sitio web tiene un certificado, pero no redirige a HTTPS | Nota | **Sitios web** → (sitio) → **SSL / HTTPS** |
| Un certificado caducó | Crítico | **Sitios web** → (sitio) → **SSL / HTTPS** |
| Un certificado caduca en 14 días o menos | Aviso | **Sitios web** → (sitio) → **SSL / HTTPS** |
| Un sitio web sirve archivos que deberían ser privados | Crítico para los secretos, Aviso para las copias y los archivos comprimidos | **Archivos** |
| Un sitio web tiene archivos o carpetas con escritura para todos | Aviso | **Archivos** |
| PHP muestra los errores a los visitantes (`display_errors`) | Aviso | **Software** → PHP → **Administrar** |
| PHP anuncia su versión (`expose_php`) | Nota | **Software** → PHP → **Administrar** |
| Hay actualizaciones pendientes del núcleo, los plugins o los temas de WordPress | Aviso | **Sitios web** → (sitio) → **WordPress** |
| El modo de depuración de WordPress (`WP_DEBUG`) está activado | Aviso | **Sitios web** → (sitio) → **WordPress** |

Algunos detalles:

- **Bases de datos en la red**: la revisión lee en qué direcciones escucha cada programa. Para una dirección pública, mira el firewall: si no hay firewall, o si una regla deja que cualquiera llegue al puerto, es crítico. Un puerto que el firewall solo abre a algunas direcciones (como el **Acceso remoto** de una base de datos), o que no abre en absoluto, es una nota. Una dirección de una red privada (10.x, 172.16–31.x, 192.168.x) es una nota.
- **Docker** escribe sus propias reglas de firewall por delante de las de ufw, así que ufw no bloquea un puerto que publica un contenedor. Publica el puerto en `127.0.0.1` (por ejemplo `127.0.0.1:8080:80`) cuando solo lo usa un sitio web de este servidor.
- **Archivos que sirve un sitio web**: la revisión lista el primer nivel de la carpeta `public` de cada sitio web y le pide al servidor web los archivos que nunca deberían ser públicos: archivos `.env`, carpetas `.git`, `.svn` y `.hg`, `.htpasswd`, volcados de bases de datos (`.sql`, `.sql.gz`, `.dump`, `.sqlite`…), copias de `wp-config.php`, copias de respaldo (`.bak`, `.old`, `.orig`, `.save`, `.swp`, nombres que terminan en `~`), `phpinfo.php` e `info.php`, y archivos comprimidos cuyo nombre parece una copia del sitio (con "backup", "www", "site", "wordpress" o el nombre del dominio). Un hallazgo significa que el servidor web respondió con el archivo. La configuración de Nginx de Nefo oculta los archivos que empiezan por punto, así que en Nginx un archivo `.env` se encuentra, pero no se informa. No se consulta a los sitios web suspendidos, a los que redirigen a otro lugar, a los que piden contraseña ni a las aplicaciones detrás de un proxy.
- **Archivos con escritura para todos**: la revisión recorre la carpeta del sitio web (`/var/www/<domain>`) sin seguir los enlaces simbólicos. Una carpeta con el sticky bit, como una carpeta `tmp` compartida, está bien. Usa 644 para los archivos y 755 para las carpetas.
- **PHP**: la revisión lee los archivos `php.ini` y `conf.d` de cada versión de PHP, y los pools de PHP-FPM, en busca de `display_errors` activado para un sitio web.
- Las actualizaciones de **WordPress** vienen de la última comprobación de la página **WordPress** (o de la pestaña **WordPress** de un sitio web). La revisión de seguridad nunca ejecuta wp-cli por su cuenta. Después de actualizar un sitio, la siguiente comprobación allí indica lo que queda.
- **El panel sin HTTPS**: Nefo activa HTTPS al instalarse. Si está desactivado, define `"enabled": true` en la parte `"tls"` de `/etc/nefo/config.json` y ejecuta `systemctl restart nefo`. No se informa de un panel que solo escucha en 127.0.0.1.

## ¿Cuánto tarda? {#limits}

La revisión lee los datos propios del panel y unos pocos archivos, y solo le pide al servidor web de este mismo servidor (127.0.0.1) archivos que existen en la carpeta de un sitio web. Cada comprobación tiene un límite, así que una ejecución sigue siendo breve en un servidor con muchos sitios web:

| Límite | Valor |
|---|---|
| Una ejecución que inicias | 25 segundos como máximo. Las comprobaciones que no terminaron para entonces aparecen como detenidas en su límite, y el resultado indica que es parcial |
| Cada programa al que consulta la revisión (fail2ban, Docker, apt…) | 5 segundos como máximo; un programa que no responde a tiempo se deja fuera |
| Archivos y carpetas recorridos en la carpeta de un sitio web | 20 000 (100 000 para todos los sitios web juntos) |
| Entradas leídas en la carpeta `public` de un sitio web | 5 000 |
| Solicitudes al servidor web de este servidor | 200 en total, 4 a la vez, 3 segundos cada una como máximo |

## Aceptar un hallazgo {#accept-a-finding}

Algunos hallazgos están bien en tu servidor: SSH en el puerto 22 detrás de fail2ban, o un puerto de base de datos que abriste a propósito para una dirección. Haz clic en **Aceptar** en ese hallazgo. Pasa a la pestaña **Aceptados** y deja de contar.

Un hallazgo aceptado vuelve cuando cambian sus datos, por ejemplo cuando otra cuenta recibe el ID de usuario 0, o cuando una base de datos empieza a escuchar en otra dirección. También vuelve si se corrigió y luego aparece de nuevo. Para que vuelva tú mismo, abre la pestaña **Aceptados** y haz clic en **Volver a mostrar**. Aceptar un hallazgo y volver a mostrarlo se anotan en el registro de actividad.

## Revisión diaria y alertas {#daily-check-and-alerts}

Con **Ejecutar todos los días** activado (lo predeterminado), el panel ejecuta la revisión una vez al día en segundo plano, durante su mantenimiento de cada hora, pero nunca en los primeros 10 minutos después de que arranca el panel. Así el panel principal siempre muestra un resultado reciente.

Activa **Avisarme cuando la revisión diaria encuentre un problema nuevo** (desactivado de forma predeterminada) para recibir un mensaje en tus canales de alerta (**Configuración** → **Alertas**) cuando la revisión diaria encuentre un problema crítico o un aviso que la revisión anterior no tenía. Las notas y los hallazgos aceptados nunca se envían. Consulta [Alertas](./mail-alerts#alerts).

## En el panel principal {#on-the-dashboard}

Cuando la última revisión encontró problemas críticos o avisos, **Requiere atención** en el panel principal y la campana muestran "Revisión de seguridad: 3 problemas que corregir" con un botón **Ver**. Cuando no hay ningún problema grave abierto, el mosaico **Riesgos de seguridad** muestra cuándo se ejecutó la revisión por última vez, y al hacer clic en él se abre la revisión de seguridad.

Para recibir avisos de los inicios de sesión en SSH y en el panel, consulta [Alertas de inicio de sesión](./mail-alerts#sign-in-alerts).
