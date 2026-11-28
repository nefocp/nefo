---
title: Claves de API
description: Automatiza el panel con claves de API, para despliegues desde CI, copias de seguridad y monitoreo, sin una sesión del navegador. Qué puede y qué no puede hacer una clave, cómo llamar a la API, ejemplos y errores.
---

# Claves de API

Una clave de API permite que un script use el panel de Nefo sin un navegador, por ejemplo para desplegar un sitio web desde CI, iniciar una copia de seguridad desde una tarea cron en otro servidor o leer el estado de los sitios web desde una herramienta de monitoreo. Una clave es de solo lectura o tiene acceso completo, se puede limitar a las direcciones que indiques y puede caducar.

El script envía la clave con cada solicitud, como token Bearer, a la misma dirección y ruta secreta que el panel.

Las claves están en **Configuración** → **Claves de API**.

## Qué puede hacer una clave {#what-a-key-can-do}

Cada clave tiene uno de dos tipos de acceso:

- **Solo lectura**: solo solicitudes `GET` y `HEAD`. Puede enumerar sitios web, bases de datos y copias de seguridad, y leer los registros de las tareas, las estadísticas y el estado del servidor, pero no cambiar nada. Tampoco puede leer lo que le daría más poder: las contraseñas de las bases de datos y de las aplicaciones, las exportaciones de bases de datos, los archivos (el administrador de archivos), las descargas de copias de seguridad, los valores de Redis y los secretos de los webhooks de Git. Esas solicitudes, y cualquier solicitud que no sea `GET` o `HEAD`, reciben `403` con `api_key_read_only`.
- **Acceso completo**: todas las solicitudes que hacen las propias páginas del panel, en tu nombre.

Ni siquiera una clave de acceso completo puede:

- crear, enumerar ni revocar claves de API;
- cambiar la contraseña ni la autenticación de dos factores, ni cerrar sesión (todo lo que está bajo `/api/auth/`);
- abrir la terminal web ni la terminal de un contenedor;
- usar las herramientas de bases de datos que están detrás del inicio de sesión del panel, Adminer y phpMyAdmin.

Estas responden `403` con `api_key_forbidden`: para ellas, inicia sesión en el panel. Una clave tampoco puede iniciar sesión para obtener una sesión.

::: warning
Una clave de acceso completo es tan poderosa como tú: puede escribir archivos, agregar tareas cron que se ejecutan como root e instalar software. Mantenla tan en secreto como la contraseña de root. Una clave de solo lectura también es un secreto: puede leer los ajustes, los registros y las listas de tus sitios web y del servidor.
:::

## Crear una clave {#create-a-key}

1. Ve a **Configuración** → **Claves de API** y selecciona **Clave nueva**.
2. Escribe un **Nombre**, como `deploy-bot`: hasta 32 letras, dígitos, espacios, puntos, guiones y guiones bajos. El nombre aparece en el registro de actividad.
3. Elige **Solo lectura** o **Acceso completo**.
4. Opcional: en **Direcciones permitidas (opcional)**, escribe las direcciones IP o las redes desde las que se puede usar la clave, como `203.0.113.7` o `198.51.100.0/24`, IPv4 o IPv6, separadas por espacios o saltos de línea. Déjalo vacío para permitir cualquier dirección.
5. Elige cuándo **Caduca**: a los 30, 90 o 365 días, o **Nunca**.
6. Selecciona **Clave nueva**.

El panel muestra la clave una sola vez, con un botón **Copiar**: cópiala ahora, no la volverás a ver. Una clave tiene la forma `nefo_` seguida de 43 caracteres aleatorios. El panel solo guarda un hash SHA-256 de ella.

La lista muestra el nombre y los primeros caracteres de cada clave (como `nefo_Ab12cd…`), su acceso, las direcciones permitidas, cuándo y desde dónde se usó por última vez (se actualiza como máximo una vez por minuto) y cuándo caduca. Puedes tener hasta 20 claves. Para revocar una clave, selecciona su icono de la papelera y confirma: la siguiente solicitud con ella falla.

## Llamar a la API {#call-the-api}

Envía la clave en el encabezado `Authorization`, como token Bearer. La dirección es la del panel con su ruta secreta, luego `api/` y la ruta:

```sh
export NEFO_KEY='nefo_…'
export NEFO='https://panel.example.com:PORT/<entrance>'

curl -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/sites"
```

- La clave solo se lee del encabezado `Authorization`: nunca de la dirección (`?key=`) ni de una cookie.
- Una solicitud con un cuerpo JSON necesita `Content-Type: application/json`. Los campos desconocidos se rechazan.
- Las solicitudes con una clave no necesitan el encabezado `X-Requested-With` que envían las propias páginas del panel.
- Usa el panel por HTTPS con un certificado real (**Configuración** → **Certificado**, consulta [Seguridad](./security#https-from-the-first-start)). Evita `curl -k`: enviaría la clave a quien estuviera en medio.

Las tareas largas, como las copias de seguridad, los despliegues y los certificados, se ejecutan en segundo plano. Su solicitud responde `202` con una tarea:

```json
{ "id": "a1b2c3", "kind": "backup", "target": "example.com", "status": "running", "log": [], "started": "…" }
```

Consulta `GET /api/jobs/{id}` hasta que su `status` sea `succeeded` o `failed`; `log` contiene sus líneas y `error`, el motivo del fallo. El panel ejecuta una tarea a la vez: mientras se ejecuta otra, una nueva responde `409` con `busy`.

## Ejemplos {#examples}

Las rutas son las que usan las páginas del panel. Reemplaza `3` por el `id` de tu sitio web.

### Enumerar los sitios web {#list-the-websites}

```sh
curl -s -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/sites"
```

La respuesta es `{"sites": [{"id": 3, "domain": "example.com", "ssl": true, …}]}`. Basta con una clave de solo lectura.

### Hacer una copia de seguridad de un sitio web {#back-up-a-website}

```sh
curl -s -X POST -H "Authorization: Bearer $NEFO_KEY" -H "Content-Type: application/json" \
  -d '{"kind": "site", "target": "example.com", "remote": true}' \
  "$NEFO/api/backups"
```

- `kind` es `site` (`target` es el dominio), `database` (el nombre de la base de datos), `folder` (una ruta absoluta) o `app` (el nombre de una aplicación de la tienda de aplicaciones).
- `remote: true` también sube la copia de seguridad al almacenamiento remoto, si está configurado (consulta [Copias de seguridad](./backups#remote-storage)).

Responde `202` con la tarea. Para esperarla:

```sh
JOB=$(curl -s -X POST -H "Authorization: Bearer $NEFO_KEY" -H "Content-Type: application/json" \
  -d '{"kind": "database", "target": "shop", "remote": false}' "$NEFO/api/backups" | jq -r .id)
while [ "$(curl -s -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/jobs/$JOB" | jq -r .status)" = running ]; do
  sleep 5
done
```

### Desplegar desde Git {#deploy-from-git}

Configura primero el repositorio en la pestaña **Despliegue con Git** del sitio web (consulta [Desplegar desde Git](./git-deploy)). Luego, desde tu CI, después de que pasen las pruebas:

```sh
curl -fsS -X POST -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/sites/3/git/deploy"
```

No necesita cuerpo y responde `202` con la tarea. En un workflow de GitHub Actions, guarda la clave y la dirección en los secretos del repositorio:

```yaml
- name: Deploy
  run: curl -fsS -X POST -H "Authorization: Bearer ${{ secrets.NEFO_KEY }}" "${{ secrets.NEFO_URL }}/api/sites/3/git/deploy"
```

### Instalar un certificado {#install-a-certificate}

Para obtener un certificado gratuito de Let's Encrypt (consulta [Certificados SSL](./ssl)):

```sh
curl -s -X POST -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/sites/3/ssl"
```

Responde `202` con la tarea. Para validar en cambio mediante el DNS de Cloudflare, que necesitan los nombres comodín, envía `-H "Content-Type: application/json" -d '{"dns": true}'`.

Para instalar tu propio certificado, como uno renovado en otro servidor, envía el certificado con su cadena y la clave privada, en PEM, hasta 256 KB en total. `jq` construye el JSON a partir de los archivos:

```sh
jq -n --rawfile cert fullchain.pem --rawfile key privkey.pem '{cert: $cert, key: $key}' |
  curl -s -X POST -H "Authorization: Bearer $NEFO_KEY" -H "Content-Type: application/json" \
    --data-binary @- "$NEFO/api/sites/3/certificate"
```

Responde `200` con el sitio web y una lista de `warnings`, o `400` con el motivo, como una clave que no corresponde al certificado (consulta [Usar tu propio certificado](./ssl#use-your-own-certificate)).

## Errores {#errors}

Los errores son JSON, como `{"error": "invalid_api_key"}`, algunos con un `detail`.

| Estado | `error` | Significado |
|---|---|---|
| 401 | `invalid_api_key` | La clave es desconocida, se revocó o caducó, o se usó desde una dirección que no permite. La respuesta no dice cuál de estos casos es; el registro de actividad sí. |
| 403 | `api_key_read_only` | Una clave de solo lectura envió una solicitud distinta de `GET` o `HEAD`, o pidió una contraseña, un archivo, una exportación o la descarga de una copia de seguridad. |
| 403 | `api_key_forbidden` | Solo una sesión iniciada puede hacer esto (consulta [Qué puede hacer una clave](#what-a-key-can-do)). |
| 429 | `too_many_attempts` | Demasiados intentos fallidos desde esta dirección. Espera `retry_after` segundos, que también figuran en el encabezado `Retry-After`. |
| 400 | como `bad_request` | La solicitud es incorrecta; `detail` puede indicar qué falla. |
| 404 | `not_found` | El elemento no existe. Una ruta fuera de la ruta secreta recibe un 404 sin más. |
| 409 | `busy` | Hay otra tarea en ejecución. Vuelve a intentarlo cuando haya terminado. |

## Intentos fallidos y fail2ban {#failed-attempts-and-fail2ban}

Un intento fallido con una clave cuenta como un inicio de sesión fallido, en el mismo contador (consulta [Límites de inicio de sesión](./security#sign-in-limits)): 5 fallos en 15 minutos bloquean la dirección durante 15 minutos, para las claves y para la página de inicio de sesión. Una clave válida no pone el contador a cero.

Cada fallo aparece en **Cuenta** → **Actividad reciente** como "Inicio de sesión fallido", con el motivo: `unknown API key`, `API key expired` o `API key not allowed from this address`. El jail del panel de fail2ban los lee, así que bloquea una dirección que sigue intentando adivinar (consulta [Protección contra fuerza bruta](./security#brute-force-protection-fail2ban)).

## El registro de actividad {#the-activity-log}

- Cada cambio hecho con una clave (todas las solicitudes salvo `GET` y `HEAD`) se anota como "Cambio mediante una clave de API", con el método y la ruta, como `POST /api/sites/3/git/deploy`.
- El usuario de estas entradas es tu nombre de usuario y el nombre de la clave, como `admin (key deploy-bot)`. Lo mismo ocurre con las entradas de lo que hizo la solicitud, como "Se desplegó desde Git".
- Las solicitudes rechazadas a una clave se anotan como "Clave de API rechazada"; la creación y la revocación de claves, como "Se creó una clave de API" y "Se revocó una clave de API".
- Las lecturas no se anotan. La lista de claves muestra cuándo se usó cada una por última vez, y desde dónde.

La propia clave nunca se escribe en ningún registro.

## Consejos de seguridad {#security-advice}

- **Usa claves de solo lectura siempre que puedas.** El monitoreo y los informes solo necesitan `GET`.
- **Una clave por script.** Su nombre aparece en el registro, y puedes revocarla sin romper las demás.
- **Permite solo las direcciones que la necesitan**, como tus runners de CI o tu servidor de copias de seguridad, cuando tienen direcciones fijas.
- **Deja que las claves caduquen, y rótalas**: crea una clave nueva, ponla en el script y luego revoca la anterior.
- **Mantén las claves fuera de los repositorios y de las direcciones.** Usa el almacén de secretos de tu CI o un archivo que solo pueda leer su propietario, y una variable en los comandos, para que la clave no acabe en el historial de la shell.
- **Revoca una clave de inmediato si puede haberse filtrado**, y luego revisa en **Actividad reciente** lo que hizo.
- Si el [firewall](./security#firewall-ufw) limita el puerto del panel a algunas direcciones, permite allí también la dirección del script.
