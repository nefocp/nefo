---
title: Desplegar desde Git
description: Despliega un sitio web desde GitHub, GitLab, Gitea o cualquier servidor Git, a mano o en cada push, con una clave de despliegue, un webhook firmado y un script de despliegue.
---

# Desplegar desde Git

Nefo despliega un sitio web desde un repositorio Git en GitHub, GitLab, Gitea, Forgejo o cualquier servidor Git, por HTTPS o por SSH con una clave de despliegue por sitio web. Despliega cuando haces clic en un botón o en cada push mediante un webhook firmado, y después ejecuta tu script de despliegue. Todo se ejecuta como el propio usuario de Linux del sitio web.

El despliegue con Git está en la pestaña **Despliegue con Git** de cada sitio web, excepto en los sitios web de aplicaciones Docker. En un sitio web cuyos [archivos están bloqueados](./website-protection#lock-a-website-s-files), el panel los desbloquea para el despliegue y después los vuelve a bloquear.

## Antes de empezar {#before-you-start}

- **La dirección del repositorio.** Nefo acepta tres formas:
  - `https://github.com/owner/repo.git` para un repositorio público;
  - `git@github.com:owner/repo.git` (la forma que muestra GitHub) para uno privado;
  - `ssh://git@git.example.com:2222/owner/repo.git` para un servidor SSH en otro puerto.

  Se rechaza `http://` simple, y también una contraseña o un token en la dirección, porque aparecería en los registros. Para un repositorio privado, usa la dirección SSH y la clave de despliegue del sitio web.
- **La carpeta.** Elige adónde van los archivos:
  - **La carpeta web** (`/var/www/example.com/public`): para un sitio web que es el propio repositorio, con `index.html` o `index.php` en el nivel superior.
  - **La carpeta del sitio** (`/var/www/example.com`): para proyectos con su propia carpeta `public`, como Laravel o Symfony. El sitio web muestra entonces la carpeta `public` del repositorio.

  Un sitio web de aplicación Node.js, Python o Java no tiene opción: los archivos van a **La carpeta de la aplicación** (`/var/www/example.com/app`), donde se ejecuta su programa. Pon `package.json` o `requirements.txt` en el nivel superior del repositorio.
- **Una carpeta vacía para el primer despliegue.** No cuentan la página de bienvenida del panel, ni las carpetas `logs`, `tmp` y `.ssh` de la carpeta del sitio, ni la carpeta `.venv` de una aplicación Python en la carpeta de la aplicación. Mueve antes los demás archivos a otro lugar, o despliega en la otra carpeta.
- **Git** no necesita estar instalado: Nefo lo instala en el primer despliegue si falta.

## Configurar el repositorio {#set-up-the-repository}

1. Abre **Sitios web** y luego la pestaña **Despliegue con Git** del sitio web.
2. Escribe la **Dirección del repositorio**.
3. Escribe la **Rama**, por ejemplo `main`.
4. En **Poner los archivos en**, elige **La carpeta web** o **La carpeta del sitio**. Un sitio web de aplicación muestra en su lugar **La carpeta de la aplicación**, sin nada que elegir.
5. Opcional: escribe un **Script de despliegue (opcional)** (consulta [El script de despliegue](#the-deploy-script)).
6. Haz clic en **Guardar**.

La primera vez, Nefo crea la clave de despliegue y el secreto del webhook del sitio web, y dice "Se guardaron los ajustes. Agrega al repositorio la clave de despliegue y el webhook de abajo y luego despliega."

## Agregar la clave de despliegue (repositorios privados) {#add-the-deploy-key-private-repositories}

Un repositorio HTTPS público no necesita clave. Para un repositorio privado, agrega al repositorio la **Clave pública** del sitio web, que aparece en **Clave de despliegue**, como clave de despliegue de solo lectura:

- **GitHub:** repositorio → Settings → Deploy keys → Add deploy key. Deja "Allow write access" desactivado.
- **GitLab:** proyecto → Settings → Repository → Deploy keys, sin acceso de escritura.
- **Gitea y Forgejo:** repositorio → Settings → Deploy keys.

Cada sitio web tiene su propia clave ed25519, creada como el usuario del sitio web en `/var/www/example.com/.ssh/panel_deploy`. **Clave nueva** la reemplaza: la clave anterior deja de funcionar de inmediato, así que agrega la nueva al repositorio y quita allí la anterior.

La primera conexión SSH acepta la clave de host del servidor Git y la recuerda en `/var/www/example.com/.ssh/known_hosts`. Si la clave del servidor cambia más adelante, los despliegues fallan con "The Git server's SSH host key is not the one seen before." Si el servidor realmente cambió su clave, elimina ese archivo y vuelve a desplegar.

## Desplegar {#deploy}

En **Desplegar**, haz clic en **Desplegar ahora**. Si cambiaste los ajustes, guárdalos primero. Un registro muestra cada paso:

1. Nefo descarga la rama desde el repositorio.
2. Los archivos con seguimiento toman la versión de la rama del repositorio.
3. Se ejecuta el script de despliegue, si escribiste uno.

Los archivos sin seguimiento se quedan, así que las subidas y los archivos que el repositorio ignora (como `.env`) se conservan en cada despliegue. Después del primer despliegue, Nefo quita su página de bienvenida cuando el repositorio ha puesto un sitio web en `public`.

::: warning
Los cambios que hagas en el servidor a archivos con seguimiento del repositorio se sobrescriben en el siguiente despliegue. Haz commit de ellos en el repositorio.
:::

Después de un despliegue, la pestaña muestra su estado (**Desplegado** o **Falló**), la hora, los primeros 12 caracteres del commit y su asunto. Una descarga puede tardar hasta 10 minutos, el script hasta 15 minutos y todo el despliegue hasta 30 minutos.

Cuando falla una descarga, el registro explica los motivos habituales:

| Mensaje | Qué hacer |
|---|---|
| `The repository refused the deploy key` | Agrega al repositorio la clave que muestra el panel. |
| `The repository is private or does not exist` | Comprueba la dirección. Para un repositorio privado, usa su dirección SSH y agrega la clave de despliegue. |
| `The repository has no branch …` | Comprueba la **Rama**. |
| `The Git server's SSH host key is not the one seen before` | Consulta la sección de la clave de despliegue, más arriba. |
| `The repository's host name was not found` | Comprueba la dirección. |
| `It took longer than 10 minutes` | Vuelve a intentarlo; revisa el tamaño del repositorio y la red. |

## El script de despliegue {#the-deploy-script}

El script se ejecuta después de cada despliegue, en la carpeta de despliegue, como el usuario del sitio web, durante 15 minutos como máximo. Puede tener hasta 2000 caracteres. Por ejemplo, para un sitio Laravel:

```sh
composer install --no-dev --optimize-autoloader
php artisan migrate --force
```

En un sitio web de aplicación, el script se ejecuta en la carpeta `app`, por ejemplo `npm ci` seguido de `npm run build`. Sus `node` y `npm` son los de la versión de Node.js elegida para el sitio web, y en un sitio web Python, `python` y `pip` son los de su entorno virtual (`app/.venv`). En un sitio web Java, `java`, `javac` y `JAVA_HOME` son los de la versión de Java elegida para él, así que `./mvnw package` o `./gradlew build` compilan con esa versión.

- Se ejecuta con `sh -e`: el script se detiene en el primer comando que falla, y el despliegue se marca como **Falló**. Para entonces, los archivos nuevos ya están en su lugar.
- `php` es la versión de PHP del sitio web, también para Composer.
- `DEPLOY_COMMIT` contiene el commit desplegado y `DEPLOY_BRANCH` la rama.
- `HOME` es la carpeta `tmp` del sitio web.
- El script no puede usar `sudo` ni reiniciar servicios del sistema. Para reiniciar el programa de un sitio web de aplicación después de un despliegue, haz clic en **Reiniciar** en la pestaña **Resumen** del sitio web.

## Desplegar en cada push {#deploy-on-every-push}

En **Desplegar en cada push**, Nefo muestra la **URL del webhook**, como `https://example.com/.panel/deploy/12`, y su **Secreto**. El webhook está en el propio dominio del sitio web, así que el servicio que aloja el repositorio nunca necesita llegar al puerto del panel. El servidor web lo pasa a un pequeño receptor del panel en `127.0.0.1:7809`.

::: warning
Sin HTTPS, la URL del webhook usa `http://`, y GitLab enviaría el secreto sin cifrar. Activa primero HTTPS: consulta [Certificados SSL](./ssl).
:::

Cada push a la rama elegida despliega el sitio web. Se ignoran los push a otras ramas, los push de etiquetas, los demás eventos y las eliminaciones de ramas.

### GitHub {#github}

1. Abre el repositorio → Settings → Webhooks → Add webhook.
2. Pega la **URL del webhook** como Payload URL y el **Secreto** como Secret.
3. Define el tipo de contenido como `application/json`.
4. Elige "Just the push event" y agrega el webhook.

GitHub envía una prueba (ping) de inmediato, y el panel la responde, así que la primera entrega aparece como correcta.

### GitLab {#gitlab}

1. Abre el proyecto → Settings → Webhooks.
2. Pega la **URL del webhook** como URL y el **Secreto** como Secret token.
3. Marca Push events y agrega el webhook.

### Gitea y Forgejo {#gitea-and-forgejo}

1. Abre el repositorio → Settings → Webhooks → Add webhook, y elige Gitea (o Forgejo).
2. Pega la **URL del webhook** como Target URL y el **Secreto** como Secret.
3. Define el tipo de contenido como `application/json`, el disparador en los eventos push, y agrega el webhook.

Gogs funciona de la misma forma: elige Gogs en el paso 1.

### Cómo comprueba el panel un webhook {#how-the-panel-checks-a-webhook}

- GitHub, Gitea, Forgejo y Gogs firman cada webhook con el secreto (HMAC-SHA256). GitLab envía el propio secreto. Un webhook con una firma incorrecta o ausente recibe HTTP 401 y se anota en el registro con la dirección del remitente.
- La protección con contraseña y el firewall web del sitio web no se aplican a la ruta del webhook, así que no necesitas hacer una excepción para ella.
- Nefo ejecuta una sola tarea a la vez. Si hay otra tarea en curso cuando llega un push, el despliegue espera y se vuelve a intentar cada 15 segundos durante un máximo de una hora. Varios push mientras tanto necesitan un solo despliegue: descarga el commit más reciente.
- **Secreto nuevo** reemplaza el secreto. Los push se rechazan hasta que pegues el secreto nuevo en el webhook del repositorio.

El servicio que aloja el repositorio muestra la respuesta del panel en su lista de entregas recientes: `deploying`, `queued: another task is running`, `ignored: not branch main`, o un error como `bad_signature`.

## El registro de despliegue {#the-deploy-log}

- La pestaña **Despliegue con Git** muestra el registro del despliegue actual o del último, también uno que haya iniciado un push.
- **Cuenta** → **Actividad reciente** enumera cada despliegue como "Se desplegó desde Git" o "Falló el despliegue con Git", con el sitio web, el commit y lo que lo inició (`manual` o un push desde el servicio que aloja el repositorio).

## Dejar de desplegar desde Git {#stop-deploying-from-git}

En **Dejar de desplegar desde Git**, haz clic en **Dejar de desplegar desde Git** y confirma. Nefo olvida los ajustes y elimina la clave de despliegue. Los archivos desplegados se quedan en la carpeta, junto con la carpeta `.git` del repositorio.
