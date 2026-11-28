---
title: Registro de alterações
description: O que mudou em cada versão do Nefo, da mais nova para a mais antiga, e o que saber antes de atualizar.
---

# Registro de alterações

Todas as versões do Nefo, da mais nova para a mais antiga, com o que há de novo, o que mudou e o que foi corrigido, e o que saber antes de atualizar. Cada uma tem um link para a página dela no GitHub, que traz os programas para amd64 e arm64, o instalador, os checksums e a assinatura deles. Para atualizar, veja [Atualização](./docs/update).

::: warning Voltar depois de uma atualização
A v0.21.0, a v0.22.0, a v0.23.0 e a v0.24.0 alteram, cada uma, o formato do banco de dados do painel na primeira vez que iniciam. Depois disso, uma versão mais antiga se recusa a iniciar com esses dados. Se você talvez queira voltar, copie `/var/lib/nefo` antes de atualizar, como mostra a página [Atualização](./docs/update#if-an-update-fails).
:::

## v0.24.1 {#v0-24-1}

28 de novembro de 2026 · [Versão no GitHub](https://github.com/nefocp/nefo/releases/tag/v0.24.1)

Backups de sites que trazem de volta sites de aplicativo e de proxy reverso, uma verificação de segurança mais rápida e correções para os sete idiomas.

**Alterações**

- **O backup de um site registra o tipo do site,** então, ao restaurá-lo em um servidor que não tem esse site, um site de aplicativo (Node.js, Python, Java) volta com o ambiente de execução, o comando de início, a porta e o limite de memória, e um site de proxy reverso volta com o endereço e as opções. Instale antes o ambiente de execução do aplicativo na página **Software**. Backups feitos por versões mais antigas continuam voltando como sites estáticos. O backup de um site de aplicativo Docker é recusado, com os passos a seguir: instale primeiro o aplicativo pela loja de aplicativos e depois restaure o backup do aplicativo. Veja [Restaurar em um novo servidor](./docs/restore-new-server).
- **A verificação de segurança roda as verificações ao mesmo tempo,** com no máximo 5 segundos para cada programa que ela consulta e 25 segundos para a verificação inteira. Uma verificação que não termina a tempo fica marcada como **resultado parcial** em vez de segurar as outras. Veja [Verificação de segurança](./docs/security-check).

**Correções**

- Em um servidor ocupado, a verificação de segurança podia demorar muito e terminar com um erro do servidor.
- Em um contêiner com limite de memória, o dashboard contava como usados o cache de páginas e a memória que o kernel libera quando os programas precisam, então a memória parecia quase cheia depois de um backup ou de um download grande. As configurações de swap agora também mostram a memória do contêiner.
- Em um servidor com o repositório do PHP, o PHP mais novo do repositório (8.6 no Ubuntu 24.04) era tomado pela versão do PHP da própria distribuição: a página **Software** trocava a origem das duas, e um servidor web instalado depois recebia essa versão em vez da versão da distribuição.
- Os apps que um catálogo mais novo da loja de aplicativos acrescenta não tinham logo fora da página da loja.
- Nos idiomas com textos mais longos, o menu da conta cortava a lista de idiomas (agora em duas colunas), as abas do painel de um site cortavam os nomes (agora ocupam duas linhas) e a barra superior podia quebrar para uma segunda linha em uma tela estreita.
- Alguns textos da interface em chinês, russo e indonésio foram corrigidos.

## v0.24.0 {#v0-24-0}

27 de novembro de 2026 · [Versão no GitHub](https://github.com/nefocp/nefo/releases/tag/v0.24.0)

Sete idiomas na interface, backups no Google Drive, restauração em um novo servidor e uma verificação de segurança.

**Novidades**

- **A interface em sete idiomas:** chinês, espanhol, português, russo e indonésio, ao lado do inglês e do vietnamita, inclusive os textos da loja de aplicativos. Cada idioma é um arquivo separado que o navegador só baixa quando ele é usado, e os arquivos da interface agora são enviados compactados com gzip: o script principal passa de 478 KB para 163 KB.
- **Backups no Google Drive,** com um cliente OAuth próprio, um login com código de dispositivo, acesso só aos arquivos que o painel cria e uploads que continuam depois de uma interrupção. Veja [Google Drive](./docs/backups#google-drive).
- **Cópias remotas criptografadas:** cada armazenamento (S3, SFTP, Google Drive) pode criptografar os backups que recebe com uma frase secreta: scrypt e AES-256-GCM em blocos numerados, de modo que uma cópia cortada ou alterada é recusada. O `nefo decrypt-backup` abre uma cópia manualmente. Veja [Criptografar as cópias](./docs/backups#encrypt-the-copies).
- **Restauração em um novo servidor:** **Encontrar backups** lista os backups de um armazenamento que este painel não conhece, e uma restauração cria um site que ainda não existe aqui, com o domínio, os aliases, a versão do PHP, os arquivos e os bancos de dados com as suas senhas. Restaurar o backup de um site excluído o cria de novo da mesma forma. Veja [Restaurar em um novo servidor](./docs/restore-new-server).
- **Sites:** clone sites de app Node.js, Python e Docker, e sites com bancos de dados PostgreSQL. Ao excluir um site, você pode manter os bancos de dados dele, e por padrão um backup é feito antes. Um novo tipo de site, o proxy reverso, encaminha para qualquer endereço, no Nginx, no Apache e no OpenLiteSpeed. Veja [Sites](./docs/websites#reverse-proxy-websites).
- **Verificação de segurança:** 22 verificações rápidas do servidor (SSH, firewall, atualizações, bancos de dados ou Redis abertos para a internet, portas do Docker que escapam do ufw, arquivos acessíveis por HTTP, certificados, configurações do PHP…), cada uma com um link para a correção, quando você pedir ou todo dia. Veja [Verificação de segurança](./docs/security-check).
- **Alertas de login** para logins SSH (todos, ou só de um endereço novo), rajadas de tentativas de login SSH malsucedidas e logins no painel. Veja [Alertas de login](./docs/mail-alerts#sign-in-alerts).
- **Bloqueio por país** por site, com os dados gratuitos de países da DB-IP, baixados só quando um site passa a usá-los. Funciona também em sites de proxy reverso. Veja [Bloqueio por país e de arquivos](./docs/website-protection).
- **Bloqueio de arquivos** por site: o código fica somente leitura para o PHP e a conta SFTP do site, exceto as pastas em que ele precisa gravar, com um alerta diário opcional quando arquivos PHP mudam. O `nefo unlock-files` devolve os arquivos antes de você remover o Nefo.
- **Bancos de dados:** MariaDB 11.4, 11.8 e 12.3 LTS do repositório da MariaDB, e uma atualização de versão no lugar com um dump antes e um caminho de volta se ela falhar; uma troca entre MariaDB e MySQL que leva os bancos de dados junto; rotação do log de consultas lentas. Veja [Bancos de dados](./docs/databases#mariadb-versions).
- **PostgreSQL:** versões 15 a 19 do projeto PostgreSQL, acesso remoto por banco de dados (TLS, scram-sha-256, uma regra de firewall por endereço) e uma atualização de versão principal com um dump antes e um caminho de volta. Os dumps que as atualizações de versão guardam aparecem em uma lista, com um botão de excluir. Veja [PostgreSQL](./docs/postgresql).
- **Registros de e-mail na página DNS:** MX, um único registro SPF mesclado e DMARC para Google Workspace, Microsoft 365, Zoho Mail, Cloudflare Email Routing ou outro provedor, pelo Cloudflare. Veja [DNS (Cloudflare)](./docs/dns#set-up-email-records).
- **Software:** Java (OpenJDK 17, 21 e 25) para sites de app, o ionCube Loader para cada versão do PHP, e o PHP 8.6, oferecido sozinho assim que o repositório do PHP o tiver. Veja [Software](./docs/software).
- **Loja de aplicativos:** mais 15 apps, 42 no total: Grafana, ntfy, Gotify, Memos, linkding, Navidrome, Vikunja, Actual Budget, changedetection.io, Healthchecks, listmonk, Kanboard, FreshRSS, Jellyfin e Mealie. Veja [Loja de aplicativos](./docs/apps).

**Alterações**

- Os números de memória seguem o limite de memória do próprio contêiner (LXC), no dashboard, nos alertas, na configuração inicial e nas verificações da loja de aplicativos.
- Os números de disco da página de configuração foram corrigidos e contam as listas de pacotes do apt em um servidor que nunca as baixou.
- Um catálogo mais novo da loja de aplicativos com apps que um painel mais antigo não consegue rodar deixa de fora só esses apps. Os apps que um catálogo mais novo acrescenta mostram o nome em todos os lugares.

**Correções**

- O PHP 8.5 não podia ser instalado, porque o OPcache não tem mais um pacote próprio.
- No Ubuntu 26.04, instalar o PostgreSQL (ou outro serviço) podia falhar enquanto o systemd se reexecutava depois do apt. Agora o painel tenta o `systemctl` de novo enquanto o systemd não está acessível.
- O firewall web era oferecido onde a distribuição não tem módulo ModSecurity para o servidor web (Ubuntu 22.04 com Nginx). Agora ele aparece como não oferecido, com o motivo.
- O texto da página WordPress vazia terminava no meio de uma frase.

## v0.23.0 {#v0-23-0}

26 de novembro de 2026 · [Versão no GitHub](https://github.com/nefocp/nefo/releases/tag/v0.23.0)

Chaves de API, importação de sites, uma página de configuração, e mais servidores de banco de dados e versões do PHP.

**Novidades**

- **Chaves de API** para os seus próprios scripts, em **Configurações** → **Chaves de API**: somente leitura ou acesso total, uma lista de endereços permitidos e uma data de expiração. Uma chave é enviada em um cabeçalho `Authorization: Bearer` e guardada só como hash, e uma chave errada conta como uma tentativa de login malsucedida. Nenhuma chave abre o terminal, o Adminer, o phpMyAdmin, a conta ou as próprias chaves. Uma chave somente leitura não consegue ler senhas, arquivos, exportações nem downloads de backups. Veja [Chaves de API](./docs/api).
- **Importar sites** de um backup completo do cPanel (sites, bancos de dados com os seus usuários e senhas, certificados e tarefas cron), ou de um arquivo compactado do site e dumps SQL feitos pelo aaPanel ou por outro servidor. Arquivos compactados hostis são recusados, dumps do MySQL 8 são adaptados para o MariaDB, e uma importação que falha remove tudo o que criou. Veja [Importar sites](./docs/import).
- **Uma página de configuração** no primeiro login: escolha o servidor web, as versões do PHP, o servidor de banco de dados e os extras em uma única página, veja mais ou menos quanta memória e quanto disco eles precisam, e instale tudo como uma única tarefa. O `nefo setup` faz o mesmo na linha de comando, e o instalador aceita as mesmas opções. Sem opções, o instalador se comporta como antes. Veja [Primeiros passos](./docs/first-steps#set-up-the-server).
- **MySQL 8.4 LTS** como alternativa ao MariaDB (um ou outro por servidor), do repositório da Oracle, com a chave fixada. Ele só é oferecido em servidores amd64.
- **phpMyAdmin**, acessível só pelo login do painel.
- **PostgreSQL** na página **Bancos de dados**: bancos de dados, proprietários, importação e exportação, e backups.
- **Memcached**, escutando só no próprio servidor, sem UDP.
- **PHP 5.6 a 7.3**, para sites trazidos de servidores mais antigos. Elas só são instaladas depois que você confirma, e nunca podem ser a única versão do PHP do servidor.
- **Marcas de fim da vida útil para o PHP:** uma versão que passou do fim do suporte de segurança do PHP é marcada na página **Software** e nos sites dela. A versão do PHP da própria distribuição nunca é marcada, porque a distribuição continua aplicando correções nela. Veja [Software](./docs/software#end-of-life).
- **Composer** e **WP-CLI** na página **Software**.

**Alterações**

- No Ubuntu, as versões extras do PHP agora vêm de `packages.sury.org`, porque o `ppa:ondrej/php` não recebe mais atualizações. Um servidor que tem o PPA faz a troca uma vez, quando o painel inicia: os arquivos de origem do PPA são movidos para `/var/lib/nefo/apt-backup`, nada é instalado ou removido, e os pacotes PHP instalados são atualizados a partir da nova origem depois do próximo `apt-get update`.

**Correções**

- Um site criado como suspenso agora também é salvo como suspenso.
- As pastas que o painel cria para outros programas não recebem mais as permissões rígidas do próprio painel (umask 077).
- A versão do PHP da própria distribuição é reconhecida em um servidor recém-instalado, antes de o apt baixar as listas de pacotes.
- Um `@` sozinho (o próprio domínio) não quebra mais uma mensagem na página DNS.
- Uma tarefa que trava agora grava o stack trace no log do painel.

## v0.22.0 {#v0-22-0}

26 de novembro de 2026 · [Versão no GitHub](https://github.com/nefocp/nefo/releases/tag/v0.22.0)

Monitoramento de sites, mais canais de alerta e DNS do Cloudflare.

**Novidades**

- **Monitoramento de sites:** cada site é verificado a cada 1 a 10 minutos (o código de status, uma palavra-chave opcional e a expiração do certificado). Um site conta como fora do ar depois de duas verificações malsucedidas, e você recebe um alerta quando ele cai e quando volta. O painel mostra o uptime em 24 horas e em 7 dias, os tempos de resposta e as falhas recentes. As requisições do monitoramento ficam fora das estatísticas de tráfego. Veja [Monitoramento](./docs/websites#monitoring).
- **Alertas por Discord, Slack e webhook.** Os webhooks são assinados com HMAC-SHA256. As URLs salvas são mascaradas, e endereços do próprio servidor, endereços link-local e endereços de metadados de nuvem são recusados quando um alerta é enviado. Veja [E-mail e alertas](./docs/mail-alerts).
- **Uma página DNS para as zonas do Cloudflare:** liste, adicione, edite e exclua registros, ative ou desative o proxy, limpe o cache, e aponte o domínio de um site para este servidor, com uma prévia antes. Veja [DNS (Cloudflare)](./docs/dns).

## v0.21.0 {#v0-21-0}

26 de novembro de 2026 · [Versão no GitHub](https://github.com/nefocp/nefo/releases/tag/v0.21.0)

Ferramentas do sistema, chaves SSH e mais opções para os sites.

**Novidades**

- **Uma página Sistema:** atualizações do sistema operacional (atualizações de segurança marcadas, instalar todas ou só as de segurança, atualizações de segurança automáticas e um aviso quando é preciso reiniciar), um arquivo de swap e o swappiness, o fuso horário, a sincronização do relógio, o nome do host, o TCP BBR e a limpeza de disco. A verificação de atualizações em segundo plano espera 30 minutos depois que o painel inicia, e reaproveita as listas de pacotes atualizadas nas últimas 6 horas. Veja [Sistema](./docs/os).
- **Chaves SSH** na página **Firewall e SSH**: liste, adicione e remova. Chaves privadas, chaves fracas e duplicadas são recusadas, e o último meio de entrar não pode ser removido.
- **Sites:** desative o HTTPS e exclua o certificado, nos três servidores web; HSTS, com `includeSubDomains` como opção; um cache de página por site PHP no Nginx, com um botão para limpar; clone um site PHP ou estático com os seus arquivos, configurações e bancos de dados; envie uma cópia de staging do WordPress de volta para o site em produção, depois de um backup.

**Correções**

- As execuções do apt do próprio painel nunca se sobrepõem, e ele espera a trava do apt de outro programa em vez de falhar.
- Em um contêiner em que `/etc/hosts` é montado com bind mount, o nome do host é gravado no próprio arquivo. Um nome do host que o sistema recusa dá um erro claro.

## v0.20.2 {#v0-20-2}

26 de novembro de 2026 · [Versão no GitHub](https://github.com/nefocp/nefo/releases/tag/v0.20.2)

**Alterações**

- O Nginx agora compacta CSS, JavaScript, SVG e fontes. O Debian e o Ubuntu ativam a compactação só para HTML, por isso esses arquivos eram enviados sem compactação. As configurações que o `nginx.conf` já tem ficam como estão. Os servidores existentes recebem a mudança quando o painel atualizado inicia.

**Correções**

- Cada versão agora traz o catálogo assinado da loja de aplicativos. Antes, os painéis o procuravam uma vez por dia e registravam um HTTP 404 no log.

## v0.20.1 {#v0-20-1}

26 de novembro de 2026 · [Versão no GitHub](https://github.com/nefocp/nefo/releases/tag/v0.20.1)

**Correções**

- O instalador falhava em todos os sistemas com suporte: ele lia a versão do sistema em `/etc/os-release` no lugar da sua própria versão, e tentava baixar uma versão que não existe. Ele foi testado em servidores recém-instalados com Ubuntu 24.04 e Debian 12 depois da correção.

## v0.20.0 {#v0-20-0}

26 de novembro de 2026 · [Tag no GitHub](https://github.com/nefocp/nefo/releases/tag/v0.20.0)

A primeira versão pública. O instalador dela falhava (corrigido na v0.20.1), e o GitHub não tem arquivos de release para ela, só a tag. Instale uma versão mais nova.

Ela continha sites no Nginx, Apache ou OpenLiteSpeed com PHP-FPM, apps Node.js e Python, certificados Let's Encrypt, deploy pelo Git e as ferramentas do WordPress; a loja de aplicativos com apps PHP e Docker; MariaDB, Redis, o gerenciador de arquivos, o terminal web e backups em armazenamento compatível com S3 e SFTP; o firewall (ufw), o fail2ban e configurações de SSH com proteção contra bloqueio, o firewall de aplicação web e o verificador de malware; versões assinadas e o `nefo update`; e a interface em inglês e vietnamita.
