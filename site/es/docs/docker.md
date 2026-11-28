---
title: Docker
description: Ejecuta contenedores y proyectos de Compose, abre una terminal en un contenedor, administra imágenes y pon un contenedor en un sitio web detrás de un proxy inverso.
---

# Docker

Nefo instala Docker de tu distribución y administra los contenedores, los proyectos de Compose y las imágenes en la página **Docker**, con una terminal en cada contenedor. Publica los puertos de los contenedores solo en 127.0.0.1, así que llegas a ellos a través de un sitio web, y rechaza los proyectos de Compose que debilitarían el servidor, como los contenedores privilegiados o los puertos abiertos a internet.

Esta página trata de cada pestaña, de las comprobaciones de seguridad que ejecuta el panel sobre los proyectos de Compose, de cómo encajan las aplicaciones Docker de la tienda de aplicaciones y de cómo hacer público un contenedor a través de un sitio web.

## Instalar Docker {#install-docker}

Ve a **Software** → **Docker** y selecciona **Instalar**. El panel instala `docker.io` de tu distribución, con su plugin Compose cuando hay uno disponible. El elemento de menú **Docker** aparece cuando Docker está instalado.

Si más adelante falta el plugin Compose, el panel lo instala la primera vez que inicias un proyecto de Compose.

Para desinstalar Docker, consulta [Software](./software#when-an-uninstall-is-refused). Se rechaza mientras existan contenedores, proyectos de Compose o aplicaciones Docker.

## ¿Por qué los puertos se quedan en 127.0.0.1? {#why-ports-stay-on-127-0-0-1}

Los puertos que publica Docker se saltan el firewall ufw: un contenedor publicado en todas las direcciones queda abierto a internet, diga lo que diga ufw. Por eso, el panel:

- publica los contenedores que inicia solo en `127.0.0.1`;
- rechaza los proyectos de Compose que publican un puerto en cualquier otra dirección.

Para hacer público un contenedor, pon un sitio web delante de él. Consulta [Poner un contenedor en un sitio web](#put-a-container-on-a-website).

## Contenedores {#containers}

La pestaña **Contenedores** enumera todos los contenedores del servidor, en ejecución o no, con su imagen, su estado y sus puertos. Aquí también aparecen los contenedores de los proyectos de Compose y de las aplicaciones de la tienda de aplicaciones.

La fila de cada contenedor tiene un botón **Iniciar** o **Detener**, y las demás acciones están en su menú:

| Acción | Qué hace |
|---|---|
| **Iniciar** / **Detener** | Inicia o detiene el contenedor |
| **Terminal** | Abre una shell dentro del contenedor. Solo para contenedores en ejecución |
| **Reiniciar** | Reinicia el contenedor |
| **Registros** | Muestra las últimas 200 líneas de su salida, con marcas de tiempo |
| **Eliminar** | Quita el contenedor, incluso mientras se ejecuta |

::: warning
**Eliminar** pierde los datos del contenedor, salvo que estén guardados en un volumen.
:::

### Ejecutar un contenedor {#run-a-container}

1. En la pestaña **Contenedores**, selecciona **Ejecutar un contenedor**.
2. Completa:
   - **Imagen**, como `nginx:alpine`;
   - **Nombre del contenedor**, como `my-app`;
   - **Puerto en este servidor** (de 1024 a 65535) y **Puerto en el contenedor**, si el contenedor sirve algo;
   - **Variables de entorno (una NAME=value por línea)**, como `TZ=UTC`.
3. Selecciona **Iniciar**.

El panel descarga la imagen si hace falta e inicia el contenedor con la política de reinicio `unless-stopped`, así que vuelve después de un reinicio. El puerto se publica solo en `127.0.0.1`.

### Terminal en un contenedor {#terminal-in-a-container}

Selecciona **Terminal** para abrir una shell dentro de un contenedor en ejecución. El panel usa `bash` cuando la imagen lo tiene, y `sh` en caso contrario. La sesión termina cuando cierras la ventana o tras 30 minutos sin actividad.

## Proyectos de Compose {#compose-projects}

Un proyecto de Compose es un `compose.yaml` que ejecuta varios contenedores juntos. El panel guarda cada proyecto en `/var/lib/panel-compose/<name>/compose.yaml`, que solo puede leer root, y lo ejecuta como el proyecto de Compose `panel-<name>`.

### Crear un proyecto {#create-a-project}

1. Ve a **Docker** → **Compose** y selecciona **Proyecto nuevo**.
2. Escribe un **Nombre del proyecto**: letras minúsculas, dígitos y guiones, hasta 40 caracteres. Los contenedores se llaman `panel-<name>-<service>-1`.
3. Escribe el archivo en el campo `compose.yaml`. **Insertar ejemplo** completa un pequeño ejemplo que funciona.
4. Selecciona **Crear e iniciar**.

El panel comprueba el archivo, lo guarda e inicia los contenedores, descargando las imágenes que necesita. Las rutas relativas del archivo, como `./data:/data`, se refieren a la carpeta del proyecto.

Un proyecto mínimo:

```yaml
services:
  web:
    image: traefik/whoami
    restart: unless-stopped
    ports:
      - "127.0.0.1:8081:80"
```

`compose.yaml` puede ocupar hasta 64 KB.

### Editar un proyecto {#edit-a-project}

Abre el menú de un proyecto y selecciona **Editar**. Cambia el archivo y selecciona **Guardar y aplicar**. El panel comprueba el archivo nuevo, lo guarda y vuelve a crear los contenedores cuya configuración cambió. También se quitan los contenedores de los servicios que quitaste del archivo.

### Acciones de un proyecto {#project-actions}

| Acción | Qué hace |
|---|---|
| **Iniciar** | Comprueba el archivo e inicia los contenedores |
| **Detener** | Detiene los contenedores |
| **Reiniciar** | Reinicia los contenedores |
| **Actualizar imágenes** | Comprueba el archivo, descarga imágenes más recientes y vuelve a crear los contenedores |
| **Quitar contenedores** | Detiene y quita los contenedores. Los volúmenes y `compose.yaml` se conservan |
| **Registros** | Las últimas 200 líneas de cada contenedor |
| **Eliminar** | Quita los contenedores, las redes y la carpeta del proyecto con `compose.yaml`. Marca **Eliminar también sus volúmenes (se pierden los datos que contienen)** para eliminar también los volúmenes. Las imágenes se conservan |

### Comprobaciones de seguridad {#safety-checks}

Antes de guardar un proyecto, y antes de cada inicio y actualización, el panel le pide a Compose la configuración final, con los include, extends y variables resueltos. Rechaza un proyecto que debilitaría el servidor:

| Se rechaza | Por qué |
|---|---|
| `privileged: true` | Un contenedor privilegiado tiene control total del servidor |
| `network_mode`, `pid`, `ipc`, `userns_mode` o `cgroup` definidos como `host` | El contenedor compartiría el propio espacio de nombres del servidor |
| Cualquier `cap_add` | Privilegios adicionales para el contenedor |
| Cualquier `devices` o `device_cgroup_rules` | Dispositivos del host entregados a un contenedor |
| `security_opt` distinto de `no-new-privileges` | Opciones como `seccomp=unconfined` o `apparmor=unconfined` debilitan el aislamiento del contenedor |
| Cualquier `sysctls` | Ajustes del kernel |
| Un puerto sin `127.0.0.1` o `::1` como dirección del host, como `"8080:80"`, o un puerto sin número publicado | Docker escucharía en todas las direcciones, saltándose el firewall |
| Un montaje bind de `/etc`, `/root`, `/boot`, `/proc`, `/sys`, `/dev`, o de cualquier cosa dentro de ellos | Las carpetas del sistema le dan a un contenedor el control del servidor |
| Un montaje bind del socket de Docker, `/var/run/docker.sock` o `/run/docker.sock`, o de una carpeta que lo contenga, como `/`, `/var`, `/var/run` o `/run` | El socket le da a un contenedor el control de Docker, y por tanto del servidor |
| Un volumen con nombre cuya opción `device` sea una de esas rutas, y los secrets o configs leídos desde una de esas rutas | Las mismas carpetas, montadas de otra forma |

Las rutas se comprueban tal como están escritas y después de seguir los enlaces simbólicos. `/etc/localtime` y `/etc/timezone` se pueden montar en solo lectura (`:ro`), para que un contenedor pueda usar la zona horaria del servidor.

Cuando una comprobación falla, el editor muestra qué servicio y qué ajuste se rechazaron. No se guarda ni se inicia nada.

## Imágenes {#images}

La pestaña **Imágenes** enumera las imágenes locales con su tamaño, su fecha de creación y los contenedores que las usan (**Usada por**).

- **Quitar** elimina una imagen. Si hay contenedores que la usan, el panel los nombra y vuelve a preguntar antes de quitar la imagen de todos modos.
- **Quitar las que no se usan** elimina todas las imágenes que ningún contenedor usa. Se vuelven a descargar cuando hacen falta.

No hay un botón aparte para descargar imágenes. Las imágenes se descargan cuando ejecutas un contenedor, inicias un proyecto de Compose o seleccionas **Actualizar imágenes**.

## Aplicaciones Docker de la tienda de aplicaciones {#docker-apps-from-the-app-store}

La [tienda de aplicaciones](./apps) instala aplicaciones Docker como Ghost, Uptime Kuma, n8n y Gitea. Se ejecutan como proyectos de Compose que el panel escribe a partir de la receta de cada aplicación:

- Sus archivos y datos están en `/var/lib/panel-apps/<name>`, y el proyecto de Compose se llama `panelapp-<name>`. No aparecen en la pestaña **Compose**, pero sus contenedores sí aparecen en la pestaña **Contenedores**.
- Pasan por las mismas comprobaciones de seguridad que tus propios proyectos.
- Cada aplicación Docker tiene su propio sitio web. Las aplicaciones de servicio como PostgreSQL, MongoDB y Ollama no tienen sitio web y solo escuchan en `127.0.0.1`.

Administra estas aplicaciones desde **Tienda de aplicaciones** → **Instaladas**, donde las inicias, detienes, actualizas y haces sus copias de seguridad. Consulta [Tienda de aplicaciones](./apps) para quitarlas, y [Copias de seguridad](./backups#app-backups) para sus copias de seguridad.

## Poner un contenedor en un sitio web {#put-a-container-on-a-website}

Un sitio web puede reenviar a sus visitantes al puerto de un contenedor en `127.0.0.1`. El servidor web actúa entonces como proxy inverso, y puedes darle al sitio un certificado SSL.

1. Publica el puerto del contenedor en `127.0.0.1`, por ejemplo `127.0.0.1:8081:80` en `compose.yaml`, o **Puerto en este servidor** `8081` en **Ejecutar un contenedor**.
2. Ve a **Sitios web** y selecciona **Agregar sitio web**.
3. Escribe el **Dominio**, como `app.example.com`.
4. En **Tipo**, elige **Aplicación (Node.js, Python, Java…) detrás de un proxy**.
5. En **Puerto en el que escucha la aplicación (127.0.0.1)**, escribe el mismo puerto, `8081`.
6. Deja vacío **Comando de inicio (opcional)**: Docker ejecuta la aplicación.
7. Selecciona **Crear sitio web**.

El puerto debe estar entre 1024 y 65535, y no ser el puerto del panel, 3306 (MariaDB), 6379 (Redis) ni 7809 (el receptor de webhooks de Git). Dos sitios web de aplicaciones no pueden usar el mismo puerto.

Después obtén un certificado para el sitio: consulta [Certificados SSL](./ssl). Consulta [Sitios web](./websites) para los demás ajustes del sitio.

El tipo **Proxy inverso a otra dirección** también funciona para un contenedor: reenvía a cualquier dirección, como `http://127.0.0.1:8081`, con opciones para el encabezado Host, WebSocket y un límite de tiempo. Consulta [Sitios web de proxy inverso](./websites#reverse-proxy-websites).
