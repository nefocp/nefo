---
title: Tienda de aplicaciones
description: Instala aplicaciones PHP en un sitio web, aplicaciones Docker detrás de su propio sitio web y servicios de bases de datos, y adminístralas, actualízalas, haz copias de seguridad y quítalas.
---

# Tienda de aplicaciones

La tienda de aplicaciones de Nefo instala 42 aplicaciones con un clic: aplicaciones PHP como WordPress, WooCommerce, Joomla, Nextcloud y Moodle en un sitio web, aplicaciones Docker como Ghost, n8n, Gitea, Grafana y Jellyfin detrás de su propio sitio web, y PostgreSQL, MongoDB y Ollama como servicios en 127.0.0.1. Antes comprueba la memoria y el disco del servidor.

Nefo crea lo que necesita cada aplicación, como una base de datos, un sitio web, contraseñas y contenedores. Esta página trata de los tipos de aplicaciones, el catálogo, la instalación y la administración de las aplicaciones que instalaste.

Abre **Tienda de aplicaciones** en el menú de la izquierda. La pestaña **Catálogo** enumera las aplicaciones y la pestaña **Instaladas**, las aplicaciones de este servidor. En la página **Sitios web**, **Instalar desde la tienda de aplicaciones** también lleva aquí.

## Tres tipos de aplicaciones {#three-kinds-of-apps}

| Tipo | Dónde va | Qué recibe |
|---|---|---|
| **Aplicación PHP** | En un sitio web PHP vacío que creaste | Su propia base de datos, sus archivos en el sitio web y las reglas de reescritura adecuadas. Se ejecuta como el usuario de Linux del sitio web. |
| **Aplicación Docker** | En un sitio web nuevo que Nefo crea para ella | Contenedores que solo escuchan en `127.0.0.1`, con el sitio web delante. |
| **Servicio** | Sin sitio web | Contenedores que solo escuchan en `127.0.0.1`, para tus propios sitios y aplicaciones. Uno de cada uno por servidor. |

Las aplicaciones PHP y las aplicaciones Docker necesitan el servidor web instalado: consulta [Sitios web](./websites). Las aplicaciones Docker y los servicios también necesitan Docker. Instálalo en la página **Software**; **Instalar Docker**, en la tienda de aplicaciones, abre esa página. Consulta [Docker](./docker).

## ¿Qué aplicaciones puedo instalar? {#the-catalog}

En la versión 2026112701 del catálogo, la tienda de aplicaciones ofrece 42 aplicaciones. Están agrupadas en categorías: **Todas** muestra cada categoría con sus aplicaciones, y el botón de una categoría muestra solo las suyas. La búsqueda mira los nombres, las descripciones y los nombres de las categorías, e ignora los acentos vietnamitas (si escribes "giam sat", encuentra las aplicaciones de "Giám sát & thống kê").

La memoria mínima es la RAM total del servidor por debajo de la cual Nefo rechaza la instalación (la swap no cuenta).

| Aplicación | Tipo | Categoría | Qué es | RAM mínima |
|---|---|---|---|---|
| WordPress | Aplicación PHP | Sitios web y contenido | Creador de sitios web y blogs | 450 MB |
| Joomla | Aplicación PHP | Sitios web y contenido | CMS para sitios web y portales | 450 MB |
| Drupal | Aplicación PHP | Sitios web y contenido | CMS para contenido estructurado, compilado con Composer | 900 MB |
| Ghost | Aplicación Docker | Sitios web y contenido | Blogs y boletines | 900 MB |
| Directus | Aplicación Docker | Sitios web y contenido | CMS headless | 900 MB |
| Wiki.js | Aplicación Docker | Sitios web y contenido | Wiki | 900 MB |
| WooCommerce | Aplicación PHP | Tiendas y negocios | WordPress con WooCommerce: una tienda en línea | 900 MB |
| OpenCart | Aplicación PHP | Tiendas y negocios | Carrito de compras | 900 MB |
| Odoo | Aplicación Docker | Tiendas y negocios | ERP (edición Community) | 1800 MB |
| Stirling-PDF | Aplicación Docker | Productividad | Herramientas PDF | 1800 MB |
| Memos | Aplicación Docker | Productividad | Notas rápidas en Markdown en una línea de tiempo | 230 MB |
| linkding | Aplicación Docker | Productividad | Gestor de marcadores | 450 MB |
| Vikunja | Aplicación Docker | Productividad | Listas de tareas, tableros Kanban y proyectos | 450 MB |
| Actual Budget | Aplicación Docker | Productividad | Presupuesto personal | 450 MB |
| Kanboard | Aplicación Docker | Productividad | Tablero Kanban para proyectos | 230 MB |
| FreshRSS | Aplicación Docker | Productividad | Lector de noticias y blogs (RSS) | 450 MB |
| Mealie | Aplicación Docker | Productividad | Recetas, planes de comidas y listas de compras | 900 MB |
| Nextcloud | Aplicación PHP | Archivos y almacenamiento | Tu propia nube para archivos, calendarios y contactos | 900 MB |
| RustFS | Aplicación Docker | Archivos y almacenamiento | Almacenamiento de objetos compatible con S3 | 900 MB |
| FileBrowser Quantum | Aplicación Docker | Archivos y almacenamiento | Archivos en el navegador | 450 MB |
| Navidrome | Aplicación Docker | Multimedia | Streaming de música de tu propia colección | 450 MB |
| Jellyfin | Aplicación Docker | Multimedia | Películas, series y música desde tu servidor | 1800 MB |
| ntfy | Aplicación Docker | Comunicación | Notificaciones push por HTTP | 230 MB |
| Gotify | Aplicación Docker | Comunicación | Servidor de notificaciones con una aplicación para Android | 230 MB |
| listmonk | Aplicación Docker | Comunicación | Boletines y listas de correo | 450 MB |
| Uptime Kuma | Aplicación Docker | Monitoreo y analítica | Vigila tus sitios y servicios | 450 MB |
| Umami | Aplicación Docker | Monitoreo y analítica | Analítica web respetuosa con la privacidad | 900 MB |
| Grafana | Aplicación Docker | Monitoreo y analítica | Tableros y alertas | 900 MB |
| changedetection.io | Aplicación Docker | Monitoreo y analítica | Vigila los cambios en páginas web | 900 MB |
| Healthchecks | Aplicación Docker | Monitoreo y analítica | Alertas cuando las tareas cron dejan de ejecutarse | 450 MB |
| Vaultwarden | Aplicación Docker | Seguridad y red | Servidor de gestor de contraseñas para las aplicaciones de Bitwarden | 450 MB |
| AdGuard Home | Aplicación Docker | Seguridad y red | Bloqueo de anuncios mediante tu propia dirección DNS-over-HTTPS | 450 MB |
| Open WebUI | Aplicación Docker | IA | Interfaz de chat para Ollama o API compatibles con OpenAI | 1800 MB |
| Ollama | Servicio | IA | Ejecuta modelos de lenguaje abiertos en la CPU | 3584 MB |
| Laravel | Aplicación PHP | Herramientas de desarrollo y automatización | Un proyecto Laravel nuevo con su base de datos | 450 MB |
| code-server | Aplicación Docker | Herramientas de desarrollo y automatización | VS Code en el navegador | 900 MB |
| IT-Tools | Aplicación Docker | Herramientas de desarrollo y automatización | Herramientas para desarrolladores en el navegador | 230 MB |
| Gitea | Aplicación Docker | Herramientas de desarrollo y automatización | Tu propio servidor Git | 450 MB |
| n8n | Aplicación Docker | Herramientas de desarrollo y automatización | Automatización de flujos de trabajo | 900 MB |
| PostgreSQL | Servicio | Bases de datos | Servidor de bases de datos PostgreSQL | 900 MB |
| MongoDB | Servicio | Bases de datos | Base de datos de documentos MongoDB | 900 MB |
| Moodle | Aplicación PHP | Educación | Plataforma de aprendizaje | 900 MB |

El catálogo viene integrado en el panel. Nefo también busca una vez al día un catálogo más reciente publicado con las versiones, y solo lo usa si su firma es válida. **Actualizar el catálogo** lo busca ahora. El encabezado muestra la versión del catálogo y si está "integrado en el panel" o "descargado, firma comprobada".

Un catálogo descargado puede contener aplicaciones que necesitan una versión más reciente de Nefo, porque su receta usa algo que esta versión no tiene. Nefo deja fuera esas aplicaciones y conserva el resto del catálogo. Si ya conocía la aplicación, conserva la versión que conoce, así que las copias instaladas siguen funcionando. El encabezado dice entonces, por ejemplo, "2 aplicaciones del catálogo descargado necesitan un panel más reciente". Actualiza Nefo para obtenerlas: consulta [Actualizar](./update). Las aplicaciones que agrega un catálogo más reciente muestran su nombre y su descripción en todos los idiomas que tiene el catálogo.

Cada tarjeta muestra la versión, el tipo y las necesidades de memoria y disco de la aplicación. Su botón **Instalar** abre la descripción, la licencia, las necesidades y las notas, y el formulario de instalación. Una tarjeta que el servidor no puede ejecutar muestra el motivo, y **Detalles** en su lugar. Un servicio instalado muestra **Administrar**. Una insignia "Código disponible" marca una licencia que no está aprobada por la OSI.

## Las comprobaciones antes de instalar {#the-checks-before-an-install}

Cuando abres una aplicación, Nefo compara sus necesidades con el servidor. Rechaza la instalación cuando:

- la memoria total del servidor está por debajo del mínimo de la aplicación (en un VPS que es un contenedor, cuenta su límite de memoria);
- el espacio libre en disco es menor que las imágenes y los datos de la aplicación más 1 GB;
- la arquitectura o las funciones del procesador no son adecuadas para la aplicación.

Avisa, pero te deja instalar, cuando el servidor tiene menos memoria de la recomendada, cuando en este momento hay menos memoria libre de la que puede usar la aplicación, o cuando los límites de memoria de todas las aplicaciones instaladas juntas superan la memoria del servidor.

## Instalar una aplicación PHP {#install-a-php-app}

1. Crea un sitio web PHP vacío con una versión de PHP que admita la aplicación: consulta [Sitios web](./websites). Las versiones mínimas de PHP son 7.4 para WordPress; 8.2 para Nextcloud y Laravel; 8.3 para WooCommerce, Joomla, Drupal y Moodle, y 8.0 para OpenCart. OpenCart admite PHP hasta la 8.3, y Moodle hasta la 8.4.
2. Asegúrate de que MariaDB (o MySQL) está instalado y en ejecución: consulta [Bases de datos](./databases).
3. En la tienda de aplicaciones, haz clic en el botón **Instalar** de la aplicación.
4. Elige el **Sitio web**. La lista muestra solo los sitios web PHP vacíos con una versión de PHP adecuada. Si no hay ninguno, usa **Agregar un sitio web PHP**.
5. En las aplicaciones que lo piden, completa **Nombre del sitio**, **Administrador** (evita `admin`: es el primer nombre que prueban los atacantes), **Correo del administrador** y **Contraseña del administrador**. Deja la contraseña vacía para obtener una aleatoria segura; una contraseña que escribas necesita de 12 a 128 caracteres. WooCommerce también pide el país y la moneda de la tienda.
6. Haz clic en **Instalar**. La instalación se ejecuta en segundo plano y aparece en la lista de tareas.

Cuando termina, Nefo muestra una sola vez la **Dirección de inicio de sesión**, el **Administrador** y la **Contraseña del administrador**. Guárdalos ahora.

La carpeta `public` del sitio web debe estar vacía; la página de bienvenida del panel no importa. Los archivos del sitio web no deben estar [bloqueados](./website-protection#lock-a-website-s-files). Drupal y Laravel se compilan con Composer, y Moodle también necesita toda la carpeta del sitio: su sitio web debe ser nuevo y estar vacío, y la carpeta del sitio se convierte en el proyecto, con `public` como carpeta web.

Una aplicación PHP recibe:

- una base de datos y un usuario de base de datos nuevos, vinculados al sitio web;
- los archivos de la aplicación, descargados y comprobados con la suma de verificación del catálogo;
- las reglas de reescritura de la aplicación, y límites de PHP más altos cuando la aplicación los necesita;
- una tarea cron que se ejecuta como el usuario del sitio web para Moodle (cada minuto) y WooCommerce (cada cinco minutos).

Si la instalación falla, Nefo vuelve a vaciar el sitio web y elimina la base de datos nueva. Los sitios WordPress también aparecen en las [herramientas de WordPress](./wordpress).

## Instalar una aplicación Docker {#install-a-docker-app}

1. Apunta en tu DNS el dominio de la aplicación a la dirección IP de este servidor, por ejemplo `app.example.com` a `203.0.113.10`.
2. En la tienda de aplicaciones, haz clic en el botón **Instalar** de la aplicación.
3. Escribe el **Dominio del sitio web nuevo** y, si quieres, **Otros dominios**.
4. Algunas aplicaciones también piden un **Correo del administrador**. Usa un dominio real, no `.test` ni `.local`.
5. Haz clic en **Instalar**.
6. Cuando termine la instalación, activa HTTPS en la pestaña **SSL / HTTPS** del sitio web nuevo: consulta [Certificados SSL](./ssl). La aplicación se reinicia con su dirección `https://`. Algunas aplicaciones necesitan HTTPS para funcionar por completo, y el formulario de instalación lo indica.

La forma de obtener la primera cuenta de administrador depende de la aplicación, y el formulario de instalación te lo dice:

| Cómo | Aplicaciones | Qué haces |
|---|---|---|
| La crea el panel | RustFS, code-server, AdGuard Home, Odoo, Umami, Grafana, ntfy, Gotify, Memos, linkding, Navidrome, Vikunja, Actual Budget, listmonk, Kanboard, FreshRSS, Jellyfin, Mealie | Abre **Credenciales** en la pestaña **Instaladas**. El usuario es `admin` (Mealie: `changeme@example.com`; Actual Budget solo tiene una contraseña de servidor). |
| Tu correo y una contraseña aleatoria | Open WebUI, Directus, Wiki.js, Healthchecks | Abre **Credenciales**. |
| La primera cuenta registrada | Ghost, Uptime Kuma, n8n, Gitea | Regístrate justo después de la instalación: la primera cuenta se convierte en administrador. |
| Una invitación | Vaultwarden | Regístrate en la página de la aplicación con la dirección de correo que invitó el panel. |
| Una contraseña de un solo uso | FileBrowser Quantum | Abre **Credenciales**; la aplicación pide una contraseña nueva en el primer inicio de sesión. |
| Una contraseña del sitio web | Stirling-PDF, changedetection.io | El sitio web pide el usuario `admin` y la contraseña que aparece en **Credenciales**. |
| Sin inicio de sesión | IT-Tools | No hay nada que hacer. |

Notas sobre algunas de estas aplicaciones:

- **Memos** y **Vikunja**: el panel crea la cuenta y luego cierra los registros. Puedes volver a abrirlos en los ajustes de la aplicación.
- **ntfy**: nadie puede leer ni enviar mensajes sin iniciar sesión. La cuenta `admin` forma parte de los ajustes de la aplicación, así que su contraseña es la que aparece en **Credenciales**.
- **Mealie**: inicia sesión con `changeme@example.com` y la contraseña de **Credenciales** (el panel reemplazó la contraseña predeterminada), y luego pon tu propia dirección de correo en tu perfil.
- **Actual Budget** solo se abre por HTTPS: primero obtén un certificado para su sitio web.
- **Healthchecks** y **listmonk** solo envían correo a través de un servicio SMTP que configures en la aplicación; este servidor no envía correo por sí mismo. Las alertas de Healthchecks a ntfy, Telegram, Slack o webhooks funcionan sin él.
- **Navidrome** y **Jellyfin**: consulta [Carpetas de música y video](#music-and-video-folders).

Una aplicación Docker recibe:

- un sitio web nuevo que reenvía a la aplicación. El sitio web permanece cerrado (503) hasta que termina la instalación y se bloquean los accesos predeterminados de la aplicación;
- contenedores que se ejecutan como un proyecto de Docker Compose, con un límite de memoria y registros rotados a los 10 MB;
- sus archivos en `/var/lib/panel-apps/<name>`, que solo puede leer root: el archivo de Compose, las contraseñas generadas y los datos de los contenedores;
- puertos publicados solo en `127.0.0.1`, nunca en internet;
- imágenes fijadas a una versión exacta.

### Carpetas de música y video {#music-and-video-folders}

Navidrome y Jellyfin reproducen archivos de una carpeta de su sitio web, para que puedas agregar archivos con el [administrador de archivos](./files) o por SFTP:

| Aplicación | Carpeta | La aplicación la ve como |
|---|---|---|
| Navidrome | `/var/www/<domain>/music` | `/music` |
| Jellyfin | `/var/www/<domain>/media` | `/media` |

Nefo crea la carpeta durante la instalación, con el usuario de Linux del sitio web como propietario, y se la da a la aplicación en solo lectura: la aplicación puede reproducir los archivos, pero no cambiarlos. Navidrome analiza la carpeta al arrancar y luego cada hora. En Jellyfin, agrega una biblioteca y elige una carpeta dentro de `/media`.

Estos archivos forman parte del sitio web, así que una copia de seguridad del sitio web los incluye, y eliminar el sitio web los elimina. Convertir video sobre la marcha consume mucha CPU en un servidor pequeño: prefiere archivos que tus dispositivos puedan reproducir tal cual.

## Instalar un servicio {#install-a-service}

PostgreSQL, MongoDB y Ollama no tienen sitio web. Haz clic en **Instalar**: no hay nada que completar. Nefo elige un puerto libre en `127.0.0.1` y crea las contraseñas. Las aplicaciones de este servidor pueden llegar al servicio; internet no. Cada servicio se puede instalar una vez por servidor.

Abre **Credenciales** en la pestaña **Instaladas** para ver la dirección, el usuario, la contraseña y la **Cadena de conexión**.

## Las comprobaciones de seguridad de las aplicaciones Docker {#the-safety-checks-on-docker-apps}

La receta de una aplicación Docker en el catálogo son datos, nunca un script. Nefo la convierte en un archivo de Compose y comprueba ese archivo con las mismas reglas que tus propios proyectos de Compose, sin excepciones:

- sin contenedores privilegiados, sin capacidades agregadas y sin dispositivos del host;
- sin compartir la red, los procesos ni otros espacios de nombres del servidor;
- sin opciones de seguridad debilitadas y sin ajustes del kernel;
- puertos publicados solo en `127.0.0.1`;
- sin montar carpetas del sistema ni el socket de Docker;
- una carpeta del sitio web (la música de Navidrome, los medios de Jellyfin) solo se monta en un contenedor que se ejecuta como el propio usuario de Linux del sitio web, y en solo lectura;
- los pasos de configuración que necesitan una contraseña la envían en el cuerpo de una solicitud o por la entrada estándar del programa, nunca en una línea de comandos.

Consulta [Docker](./docker) para estas reglas. Las contraseñas se generan para cada instalación, y las contraseñas iniciales se quitan de los ajustes del contenedor después del primer arranque. Un catálogo descargado solo se usa si su firma es válida.

## Administrar las aplicaciones instaladas {#manage-installed-apps}

La pestaña **Instaladas** enumera todas las aplicaciones instaladas. Filtra con **Todas**, **Con sitio web** o **Servicios**, o busca. Las columnas son **Aplicación**, **Versión**, **Estado** y **Memoria** (lo que usan los contenedores ahora).

El estado es uno de estos: **En ejecución**, **Detenida**, **En ejecución parcial**, **Sin contenedores**, **Instalando** o **Falló la instalación**. Una aplicación PHP muestra **En su sitio web**, o **Sitio web suspendido**.

El menú de cada fila tiene estas acciones:

| Acción | Para | Qué hace |
|---|---|---|
| **Abrir** | Aplicaciones con sitio web | Abre el sitio web de la aplicación. |
| **Credenciales** | Aplicaciones Docker y servicios | Muestra los datos de inicio de sesión. Cada apertura se anota en el registro de actividad. |
| **Registros** | Aplicaciones Docker y servicios | Las líneas más recientes de los registros de los contenedores. |
| **Iniciar**, **Detener**, **Reiniciar** | Aplicaciones Docker y servicios | Inicia, detiene o reinicia los contenedores. |
| **Actualizar** | Aplicaciones Docker y servicios, cuando el catálogo tiene una versión más reciente | Consulta más abajo. |
| **Hacer copia ahora** | Aplicaciones Docker y servicios | Hace una copia de seguridad de la aplicación. La copia está en la página **Copias de seguridad**: consulta [Copias de seguridad](./backups). |
| **Ajustes del sitio web** | Aplicaciones con sitio web | Abre los ajustes del sitio web. |
| **Desinstalar** | Servicios | Quita el servicio: consulta más abajo. |
| **Eliminar con su sitio web** | Aplicaciones con sitio web | Abre el sitio web, donde lo eliminas. |

### Actualizar una aplicación {#update-an-app}

Cuando el catálogo tiene una versión más reciente de una aplicación Docker o de un servicio, la columna **Versión** la muestra, por ejemplo "2.6.0 disponible". Haz clic en **Actualizar** y confirma. Nefo:

1. primero hace una copia de seguridad de la aplicación. Si la copia de seguridad falla, no cambia nada;
2. descarga las imágenes nuevas antes de detener nada;
3. reinicia la aplicación con ellas y comprueba que responde. Si no responde, vuelve a ejecutarse la versión anterior.

Algunas aplicaciones rechazan la actualización con un clic cuando cambia la versión principal: "Esta actualización (…) cambia la versión principal y requiere pasos manuales. Consulta la guía de actualización de la aplicación." Esto se aplica a PostgreSQL, Wiki.js, Odoo, Grafana, Gotify, Vikunja, Healthchecks, listmonk, Jellyfin y Mealie, y a un cambio de versión menor de MongoDB.

Las aplicaciones PHP no se actualizan desde aquí: "Actualízala desde la propia página de administración de la aplicación." Para WordPress, usa las [herramientas de WordPress](./wordpress).

Una aplicación instalada antes de que existiera el catálogo muestra **Instalación antigua**. **Convertir y actualizar** mueve sus datos a la carpeta de la tienda de aplicaciones, la ejecuta con Docker Compose y la actualiza, después de una copia de seguridad.

## Desinstalar una aplicación {#uninstall-an-app}

- **Un servicio:** haz clic en **Desinstalar**, escribe su nombre y confirma. Se eliminan sus contenedores, datos y contraseñas. Sus copias de seguridad se conservan.
- **Una aplicación con sitio web:** elimina el sitio web: consulta [Eliminar un sitio web](./websites#delete-a-website). En una aplicación Docker, esto quita sus contenedores, su carpeta en `/var/lib/panel-apps` y las imágenes que ninguna otra aplicación usa. **Hacer antes una copia de seguridad del sitio web**, marcada de forma predeterminada en el cuadro de diálogo de eliminación, también hace una copia de seguridad de los datos de la aplicación, del tipo **Aplicación**. En una aplicación PHP, marca **Eliminar también sus bases de datos** para eliminar también su base de datos.

::: danger
Desinstalar elimina para siempre los datos de la aplicación. Haz antes una copia de seguridad con **Hacer copia ahora** si es posible que vuelvas a necesitarlos.
:::

## Hacer una segunda copia de una aplicación {#make-a-second-copy-of-an-app}

Para hacer una segunda copia de una aplicación Docker con sus datos, clona su sitio web: **Sitios web** → (sitio) → **Resumen** → **Clonar…**. Consulta [Clonar un sitio web](./websites#docker-apps). n8n, Odoo y las aplicaciones instaladas antes de la tienda de aplicaciones no se pueden clonar. Una aplicación PHP se clona con su sitio web y su base de datos, como cualquier sitio web PHP.
