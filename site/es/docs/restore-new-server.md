---
title: Restaurar en un servidor nuevo
description: Encuentra las copias de seguridad de tu almacenamiento S3, SFTP o Google Drive desde un servidor Nefo nuevo, restaura con sus bases de datos los sitios web que todavía no existen en él y descifra a mano una copia de seguridad.
---

# Restaurar en un servidor nuevo

Si pierdes un servidor, un servidor nuevo con Nefo puede encontrar las copias de seguridad de tu almacenamiento remoto (compatible con S3, SFTP o Google Drive) con **Buscar copias de seguridad** y restaurarlas. Un sitio web que no existe en el servidor nuevo se vuelve a crear a partir de su copia de seguridad, con su dominio, sus alias, su versión de PHP, sus archivos y sus bases de datos. Esto funciona a partir de v0.24.0.

Esta página trata de cómo encontrar las copias de seguridad, qué crea una restauración y qué no, los demás tipos de copias de seguridad y cómo descifrar una copia a mano. Para las copias de seguridad en general, consulta [Copias de seguridad](./backups).

## Qué necesitas {#what-you-need}

- los datos de acceso del almacenamiento: el bucket y sus claves, la cuenta SFTP o un cliente OAuth de Google del mismo proyecto de Google Cloud;
- la frase de contraseña, si las copias están cifradas;
- un servidor con Nefo, un servidor web, las versiones de PHP que usan tus sitios web y MariaDB o MySQL (consulta [Primeros pasos](./first-steps#set-up-the-server)).

## ¿Cómo restauro un sitio web en un servidor nuevo? {#restore-on-another-server}

1. Instala Nefo en el servidor nuevo y luego el servidor web, PHP y MariaDB (consulta [Primeros pasos](./first-steps)).
2. Ve a **Copias de seguridad** → **Almacenamiento remoto** y configura el mismo almacenamiento: el mismo bucket y la misma carpeta, el mismo servidor SFTP y carpeta, o la misma carpeta de Google Drive. Para Google Drive, usa el mismo proyecto de Google Cloud (consulta [Google Drive](./backups#google-drive)). Marcar **Cifrar las copias de seguridad antes de subirlas** con la frase de contraseña anterior es opcional: también puedes escribir la frase de contraseña en el paso 4.
3. Selecciona **Buscar copias de seguridad** en la fila del almacenamiento. El panel enumera los archivos de copia de seguridad del almacenamiento que no están en su lista de copias de seguridad y lee el principio de cada uno (su `manifest.json`): qué respalda, cuándo se hizo, su tamaño, la versión de PHP y las bases de datos. No descarga el resto.
4. Si algunas copias de seguridad cifradas no se abren con la frase de contraseña del almacenamiento, escribe la frase con la que se subieron y selecciona **Leerlas**.
5. Selecciona **Agregar a la lista** en una copia de seguridad, o **Agregar todas a la lista (…)**, con el número de copias de seguridad que el panel pudo leer. Nada cambia en el servidor: las copias de seguridad están ahora en la lista de **Copias de seguridad**, guardadas solo en el almacenamiento.
6. Restaura cada una con **Restaurar**, como en [Restaurar una copia de seguridad](./backups#restore-a-backup). **Restaurar** en la lista de copias encontradas hace los pasos 5 y 6 de una vez.

Una copia de seguridad que el panel no puede leer se queda en la lista con el motivo, como una frase de contraseña incorrecta, un archivo dañado o un manifiesto que nombra un sitio web distinto de la carpeta en la que está. **Volver a buscar** lee de nuevo el almacenamiento.

Las copias de seguridad encontradas en un almacenamiento no tienen ninguna suma de verificación registrada en la lista. Su primera restauración lee todo el archivo comprimido una vez antes de cambiar nada, así que un archivo dañado se rechaza, y registra entonces la suma de verificación.

## Restaurar un sitio web que no existe aquí {#restoring-a-website-that-doesn-t-exist-here}

Cuando el sitio web de una copia de seguridad no existe en este servidor, el cuadro de diálogo de restauración lo indica, y la restauración lo crea:

- con el mismo dominio y los mismos alias. Si otro sitio web ya tiene un alias, el sitio web se crea sin sus alias;
- con la misma versión de PHP, o la más reciente instalada aquí, con un aviso en el registro;
- marcado como WordPress (o como la otra aplicación) con sus reglas de reescritura y sus propias directivas del servidor web, cuando la copia de seguridad las tiene;
- con sus archivos, que pertenecen al usuario del sitio web nuevo;
- con sus bases de datos y sus usuarios. Las copias de seguridad hechas con v0.24.0 y posteriores guardan la contraseña de cada usuario de base de datos, así que la configuración propia del sitio web (`wp-config.php` y similares) sigue funcionando. Con una copia de seguridad más antigua, el usuario recibe una contraseña nueva: WordPress la recibe automáticamente en `wp-config.php`; para otro software, cópiala desde **Bases de datos**.

Lo que no está en una copia de seguridad y por eso no se vuelve a crear: el certificado SSL (obtén uno nuevo en la pestaña **SSL / HTTPS** del sitio web cuando su DNS apunte al servidor nuevo), las tareas cron, las cuentas SFTP, los programas en segundo plano, las redirecciones, la protección con contraseña y los demás ajustes del sitio web.

Las copias de seguridad hechas con Nefo 0.24.1 o posterior registran el tipo de cada sitio web: un sitio web de aplicación (Node.js, Python, Java) se vuelve a crear con su entorno de ejecución, su comando de inicio, su puerto y su límite de memoria, y un sitio web de proxy inverso, con su dirección y sus opciones. Primero instala el entorno de ejecución de la aplicación en la página **Software**; si falta, la restauración se detiene y lo indica. Las copias hechas con versiones anteriores solo registran la versión de PHP, y un sitio web así se vuelve a crear como sitio web estático: créalo primero con el mismo dominio y el mismo tipo (consulta [Sitios web](./websites#create-a-website)) y luego restaura; la restauración solo reemplaza entonces sus archivos y sus bases de datos.

Una restauración nunca reemplaza un sitio web sin la confirmación: escribes su dominio en el cuadro de diálogo, exista o no. Restaurar sobre un sitio web que existe reemplaza sus archivos y sus bases de datos, como siempre.

::: tip
Una copia de seguridad de un sitio web contiene las contraseñas de los usuarios de sus bases de datos, junto a los archivos y los volcados. Trata las copias de seguridad con el mismo cuidado que el propio sitio web. Cifrar las copias remotas hace que no se puedan leer en el almacenamiento.
:::

## Otros tipos de copias de seguridad {#other-kinds-of-backups}

| Tipo | En un servidor nuevo |
|---|---|
| **Base de datos** | Se vuelve a crear si falta, con la contraseña de su usuario guardada en la copia de seguridad cuando la copia la tiene |
| **Carpeta** | Se vuelve a crear si falta |
| **Aplicación** | Instala primero la aplicación desde la Tienda de aplicaciones (sus imágenes se vuelven a descargar) y luego restaura |

## Descifrar una copia a mano {#decrypt-a-copy-by-hand}

También puedes descargar tú mismo un archivo de copia de seguridad del almacenamiento: desde la consola de S3, con un cliente SFTP o desde Google Drive en un navegador. Una copia cifrada (`.tar.gz.enc`) solo necesita su frase de contraseña. Descífrala con el programa `nefo`, en cualquier servidor Linux en el que esté instalado:

```sh
nefo decrypt-backup example.com_20261126-030000.tar.gz.enc
```

Te pide la frase de contraseña y escribe `example.com_20261126-030000.tar.gz` a su lado. Consulta [Línea de comandos](./cli#nefo-decrypt-backup).

El archivo comprimido tiene los archivos del sitio web en `files/` y los volcados de las bases de datos en `databases/`. Descomprímelo con `tar xzf`.

## Formato de las copias cifradas {#format-of-encrypted-copies}

Como referencia, y para descifrar una copia sin Nefo. Todos los números son big-endian.

| Desplazamiento | Tamaño | Contenido |
|---|---|---|
| 0 | 7 | `NEFOENC` (ASCII) |
| 7 | 1 | Versión del formato: 1 |
| 8 | 1 | Derivación de la clave: 1 = scrypt |
| 9 | 1 | Coste de scrypt, log2(N): 16 |
| 10 | 1 | r de scrypt: 8 |
| 11 | 1 | p de scrypt: 1 |
| 12 | 1 | Cifrado: 1 = AES-256-GCM |
| 13 | 1 | log2(tamaño del fragmento): 16, para 64 KiB |
| 14 | 2 | Cero |
| 16 | 32 | Sal, aleatoria para cada archivo |
| 48 | 12 | Comprobación de la clave |
| 60 | 4 | CRC-32 (IEEE) de los bytes 0 a 59 |

- scrypt(frase de contraseña, sal, N, r, p) da 44 bytes. Los bytes 0–31 son la clave AES-256; los bytes 32–43 deben ser iguales a la comprobación de la clave o, si no, la frase de contraseña es incorrecta.
- Después de la cabecera de 64 bytes vienen los fragmentos. El archivo comprimido se corta en trozos del tamaño del fragmento. El último trozo es más corto, posiblemente vacío, así que siempre hay tamaño ÷ fragmento + 1 trozos (redondeado hacia abajo, más uno).
- Cada trozo se sella con AES-256-GCM, con la cabecera de 64 bytes como datos adicionales, y el nonce: el número del trozo desde 0 (8 bytes), 3 bytes a cero y luego `01` para el último trozo y `00` para los demás. Se guarda como el texto cifrado seguido de la etiqueta de 16 bytes.
- El tamaño del archivo es 64 + tamaño + 16 × (tamaño ÷ 65536 + 1).
