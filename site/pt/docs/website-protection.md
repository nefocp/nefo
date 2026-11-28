---
title: Bloqueio por país e de arquivos
description: Bloqueie ou permita visitantes por país em um site, com os dados gratuitos de países da DB-IP, e deixe o código de um site somente leitura para o PHP dele, com um alerta quando arquivos PHP mudam.
---

# Bloqueio por país e de arquivos

O Nefo tem duas proteções por site, a partir da v0.24.0. O **Bloqueio por país** recusa os visitantes dos países que você listar, ou deixa entrar só esses países, com os dados gratuitos de países da DB-IP, no Nginx, no Apache e no OpenLiteSpeed. O bloqueio de arquivos deixa o código do site somente leitura para o próprio PHP e a conta SFTP dele, exceto as pastas em que ele precisa gravar, como a de uploads.

As duas ficam nas configurações do site, no grupo **Segurança**: **Sites** → (site) → **Bloqueio por país** e **Bloqueio de arquivos**. Nenhuma das duas custa nada enquanto está desativada: nenhum download, nenhum processo a mais, nada carregado no servidor web. Para o firewall de aplicação web, os limites de requisições e o verificador de malware, veja [Segurança](./security#website-security).

## Como bloqueio países em um site? {#block-or-allow-countries}

1. Abra **Sites**, clique no site e abra a aba **Bloqueio por país**.
2. Escolha **Bloquear os países abaixo** ou **Deixar entrar só os países abaixo**.
3. Digite o nome de um país (no idioma do painel) ou o código de duas letras dele no campo de pesquisa, e clique nele para acrescentá-lo. Remova um com o × da etiqueta dele.
4. Clique em **Salvar**.

Na primeira vez que algum site do servidor usa o bloqueio por país, o Nefo baixa o banco de dados de países do db-ip.com (cerca de 5 MB) antes de salvar, o que leva alguns segundos. Para desativá-lo de novo, escolha **Desativado: todos os países entram** e clique em **Salvar**.

O bloqueio por país funciona em todos os tipos de site, inclusive os de proxy reverso. Cada site tem a sua própria configuração.

Os visitantes bloqueados recebem **403 Forbidden**. Para mostrar a sua própria página, coloque um arquivo `403.html` na pasta `public` do site e aponte para ele a página de erro 403 do servidor web em **Sites** → (site) → **Configuração do Nginx** (Nginx: `error_page 403 /403.html;`; Apache: `ErrorDocument 403 /403.html`, que também funciona no `.htaccess`; OpenLiteSpeed: um bloco `errorpage 403`). Os visitantes bloqueados podem carregar o próprio `/403.html`.

### O que nunca é bloqueado {#what-is-never-blocked}

- As verificações do Let's Encrypt (`/.well-known/acme-challenge/`), para que os certificados continuem se renovando.
- O webhook do deploy pelo Git (`/.panel/deploy/…`). O painel confere a assinatura dele.
- `/403.html`.
- Endereços privados e outros endereços reservados (`10.0.0.0/8`, `192.168.0.0/16`, loopback etc.).
- As requisições do próprio servidor, como o WordPress chamando a si mesmo para as suas tarefas agendadas.

O próprio painel não é um site: ele tem a sua própria porta e nunca é bloqueado por país. Para limitar quem pode chegar ao painel, use o firewall (veja [Segurança](./security#firewall-ufw)).

### Atrás do Cloudflare {#behind-cloudflare}

Quando o registro DNS de um site passa pelo proxy do Cloudflare, todas as requisições vêm dos endereços do Cloudflare.

- **Nginx e Apache**: para as requisições que vêm dos endereços do Cloudflare, o Nefo consulta o endereço do visitante que o Cloudflare envia no cabeçalho `CF-Connecting-IP`. As requisições de qualquer outro lugar são consultadas pelo próprio endereço, por isso um cliente não consegue falsificar o cabeçalho. No Apache, o log de acesso do site e o PHP também passam a ver o endereço do visitante nas requisições que chegam pelo Cloudflare.
- **OpenLiteSpeed**: para as requisições dos endereços do Cloudflare, o Nefo usa o país que o Cloudflare envia no cabeçalho `CF-IPCountry`. Ative o **IP Geolocation** (geolocalização de IP) no Cloudflare (configurações de **Network**, rede, da zona). Sem isso, os visitantes que chegam pelo Cloudflare não são bloqueados e, com **Deixar entrar só os países abaixo**, eles entram.

O Nefo mantém uma lista embutida dos endereços do Cloudflare e a atualiza a partir do Cloudflare todo mês, junto com o banco de dados de países.

### Limites por servidor web {#limits-per-web-server}

| Servidor web | Como funciona | Limite |
|---|---|---|
| Nginx | O módulo `geo` embutido do Nginx, carregado só enquanto um site usa o bloqueio por país, só com os países que os sites listam | Nenhum. Memória: cerca de 2 a 5 MB para um país, cerca de 40 MB para listas grandes (Estados Unidos, Alemanha e Japão juntos) |
| Apache | O Apache verifica as faixas de endereços dos países listados uma por uma em cada requisição | No máximo 50.000 faixas de endereços por site, por isso países grandes são recusados (só os Estados Unidos têm cerca de 270.000). A China (15.000 faixas) ocupa cerca de 2,5 MB |
| OpenLiteSpeed | A consulta GeoIP do próprio OpenLiteSpeed no arquivo de banco de dados da DB-IP | Nenhum. Menos de 1 MB |

Uma lista pode ter até 100 países. No Apache, a aba mostra quantas faixas de endereços o site usa do limite. Se uma lista for grande demais, o Apache a recusa, e a mensagem cita os países que não cabem, os maiores primeiro, com o número de faixas de cada um. Deixe-os de fora, ou use o Nginx ou o OpenLiteSpeed, que lidam com qualquer país.

### Os dados de países {#the-country-data}

Os dados são o banco de dados gratuito **IP to Country Lite** da DB-IP, IP Geolocation by [DB-IP](https://db-ip.com), sob a licença Creative Commons Attribution 4.0. Ele cobre IPv4 e IPv6.

- O Nefo o baixa por HTTPS na primeira vez que um site usa o bloqueio por país.
- Antes de substituir a cópia antiga, ele confere se o arquivo é um banco de dados completo: o formato, o tamanho e se ele lista mais de 200 países.
- Ele o atualiza uma vez por mês (a DB-IP publica uma nova edição no início de cada mês). A aba mostra a edição em uso.
- Enquanto nenhum site usa o bloqueio por país, nada é baixado e o servidor web não carrega nada.

O banco de dados fica em `/var/lib/panel-geoip/`. Todos os usuários podem lê-lo, já que o servidor web o lê: são dados públicos.

## Como deixo os arquivos de um site somente leitura? {#lock-a-website-s-files}

A aba **Bloqueio de arquivos** deixa o código do site somente leitura para o PHP dele, o app dele e a conta SFTP dele. Um plugin com uma falha de segurança então não consegue mais alterar os arquivos do site nem acrescentar novos, exceto nas pastas que você permitir, como as de uploads e caches. Outros painéis chamam isso de proteção contra adulteração (tamper protection).

1. Abra **Sites**, clique no site e abra a aba **Bloqueio de arquivos**.
2. Confira a lista em **Pastas graváveis**. Para sites WordPress, Joomla, Laravel, Drupal, Nextcloud, OpenCart, Moodle, Symfony e CodeIgniter, o painel preenche as pastas da aplicação; **Usar as pastas do WordPress** (ou da aplicação do site) as coloca de volta. Escreva uma pasta por linha, relativa à pasta do site, por exemplo `public/wp-content/uploads`. `*` vale por um nome de pasta: `public/sites/*/files`. `tmp` e `logs` sempre continuam graváveis.
3. Para ser avisado quando arquivos PHP mudarem mesmo assim, marque **Avisar quando arquivos PHP mudarem** (veja [abaixo](#alerts-when-php-files-change)).
4. Clique em **Bloquear os arquivos**.

A aba passa a mostrar **Bloqueados desde** e a data. Para mudar as pastas graváveis ou o alerta depois, edite-os e clique em **Salvar**: os arquivos são bloqueados de novo com a nova lista. **Desbloquear os arquivos** devolve tudo ao usuário do site, com exatamente o proprietário, o grupo e as permissões que cada arquivo tinha antes.

A aba **Bloqueio de arquivos** não aparece para apps Docker, cujos arquivos pertencem aos contêineres deles, nem para sites de proxy reverso, que não servem arquivos.

As pastas predefinidas:

| Aplicação | Pastas graváveis |
|---|---|
| WordPress | `public/wp-content/uploads`, `public/wp-content/cache`, `public/wp-content/upgrade` |
| Joomla | `public/images`, `public/media`, `public/cache`, `public/administrator/cache`, `public/administrator/logs`, `public/tmp` |
| Laravel | `storage`, `bootstrap/cache` |
| Drupal | `public/sites/*/files` |
| Nextcloud | `data`, `public/config` |
| OpenCart | `storage`, `public/image/cache`, `public/image/catalog` |
| Moodle | `moodledata` |
| Symfony | `var` |
| CodeIgniter | `writable` |

Uma pasta gravável que ainda não existe é criada, para o usuário do site, quando você bloqueia os arquivos, se a pasta acima dela existir. A aba lista as que ela não encontrou.

### O que muda enquanto os arquivos estão bloqueados {#what-changes-while-the-files-are-locked}

- O PHP do site, o app dele e a conta SFTP dele não conseguem alterar, acrescentar, renomear ou excluir nada fora das pastas graváveis, nem mudar permissões ali. Os uploads e os caches continuam funcionando.
- O WordPress não consegue se atualizar, instalar ou atualizar plugins e temas pelo painel dele, nem reescrever o `wp-config.php` ou o `.htaccess`. Atualize-o pela aba **WordPress** (veja abaixo).
- O site continua funcionando: o servidor web e o PHP ainda leem todos os arquivos que conseguiam ler antes. Os arquivos que só o usuário do site conseguia ler, como o `wp-config.php` (modo `0600`), continuam ilegíveis para o servidor web.

Como funciona: os arquivos e as pastas que eram do usuário do site passam a pertencer ao root, sem permissão de gravação para mais ninguém. Os arquivos que o grupo do site podia ler mantêm esse grupo. Os arquivos que só o proprietário podia ler recebem o grupo de bloqueio próprio do site (`panel-lock-<uid>`, cujo único membro é o usuário do site). Nenhum módulo do kernel, nenhum `chattr`. O que o bloqueio alterou fica registrado em `/var/lib/nefo/filelock/`, por isso o desbloqueio restaura tudo exatamente, mesmo depois de uma pane.

### Operações do painel em um site bloqueado {#panel-operations-on-a-locked-website}

| Operação | Enquanto bloqueado |
|---|---|
| Aba **WordPress**: **Atualizar tudo**, modo de manutenção, modo de depuração, atualizações automáticas | O painel desbloqueia os arquivos, faz a alteração e os bloqueia de novo |
| Enviar uma cópia de staging para este site | Desbloqueado para o envio, depois bloqueado de novo |
| **Deploy pelo Git** (manual ou pelo webhook) | Desbloqueado para o deploy, depois bloqueado de novo |
| Restaurar um backup deste site (ou de uma pasta dele) | Desbloqueado para a restauração, depois bloqueado de novo |
| Verificação de malware: pôr um arquivo em quarentena ou restaurá-lo | Desbloqueado nesse meio-tempo, depois bloqueado de novo |
| Instalar as dependências do app (Node.js, Python) | Desbloqueado nesse meio-tempo, depois bloqueado de novo |
| **Arquivos** (o gerenciador de arquivos): editar, enviar, criar, renomear, excluir, extrair, mudar permissões | Permitido dentro das pastas graváveis e de `tmp`, `logs`; recusado no resto, com uma mensagem de que os arquivos do site estão bloqueados |
| Instalar um app (WordPress ou outro) no site | Recusado: desbloqueie os arquivos antes |
| Importar um site para este site (vazio) | Recusado: desbloqueie os arquivos antes |
| Clonar o site, uma cópia de staging dele, backups | Funcionam: eles só leem os arquivos, inclusive os que só o site conseguia ler (como o `wp-config.php`). A cópia pertence ao seu próprio site e não fica bloqueada |

Depois de um envio, um deploy ou uma restauração, os novos arquivos ficam sob o bloqueio como os outros; os arquivos que chegam em uma pasta gravável continuam sendo do site. Cada operação que desbloqueia os arquivos grava "Unlocking the files of example.com for this change" (desbloqueando os arquivos de example.com) e "Locking the files of example.com again" (bloqueando de novo os arquivos de example.com) no log da tarefa. Se o novo bloqueio falhar, a tarefa avisa e o painel tenta de novo a cada hora. Enquanto uma operação roda, a aba mostra que os arquivos estão desbloqueados para ela.

::: warning
O bloqueio impede o próprio usuário do site, não o root. Qualquer coisa que rode como root, como uma sessão SSH como root ou o terminal do painel, ainda pode alterar os arquivos. Antes de remover o painel, rode `nefo unlock-files` (veja [Desinstalação](./uninstall#unlock-the-websites-files)), ou os arquivos dos sites bloqueados continuam pertencendo ao root.
:::

### Alertas quando arquivos PHP mudam {#alerts-when-php-files-change}

Com **Avisar quando arquivos PHP mudarem**, o painel tira um snapshot (um checksum SHA-256 de cada arquivo PHP do site, inclusive nas pastas graváveis) quando bloqueia os arquivos, e uma vez por dia compara os arquivos PHP com ele. Quando arquivos foram alterados, acrescentados (um arquivo PHP em `uploads` é um sinal clássico de invasão) ou removidos, ele envia um alerta pelos canais configurados em **Configurações** → **Alertas**, e a aba lista os arquivos. Ele só avisa de novo quando a lista de diferenças muda.

Se foi você que fez as alterações, clique em **Fazer um novo snapshot**. As operações que desbloqueiam e bloqueiam os arquivos de novo, como uma atualização do WordPress, tiram um novo snapshot sozinhas.

A verificação diária lê no máximo 200.000 arquivos PHP ou 2 GB por site. A aba avisa quando ela parou nesse limite.
