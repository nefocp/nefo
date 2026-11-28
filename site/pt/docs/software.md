---
title: Software
description: Instale e desinstale o servidor web, o PHP 5.6 a 8.6, o ionCube Loader, o MariaDB ou o MySQL, o PostgreSQL, o Redis, o Memcached, o Adminer, o phpMyAdmin, o Node.js, o Python, o Java, o Docker, o firewall web e o ClamAV, e gerencie as extensões do PHP e as configurações do php.ini.
---

# Software

O Nefo não instala nada até você escolher. A página **Software** instala e desinstala cada componente opcional, com mais ou menos quanta memória ele usa: o servidor web, o PHP 5.6 a 8.6 e o ionCube Loader, o MariaDB ou o MySQL, o PostgreSQL, o Redis, o Memcached, o Node.js, o Python, o Java, o Docker, o firewall web e o ClamAV. Ela se recusa a remover o que algo ainda usa.

Esta página trata do que você pode instalar, de quando uma desinstalação é recusada, do que uma desinstalação mantém e das configurações do PHP, do Node.js, do Python, do Java e do Redis. Um servidor pequeno continua leve quando só tem o que os seus sites usam.

As instalações e as desinstalações rodam como tarefas com um log ao vivo. O painel roda uma tarefa por vez. O software vem dos pacotes da sua distribuição, a menos que esta página diga outra coisa.

## O que você pode instalar {#what-you-can-install}

Cada cartão mostra mais ou menos quanta memória o item usa enquanto roda. O topo da página soma os itens instalados.

| Seção | Item | Memória, mais ou menos |
|---|---|---|
| **Nginx, Apache ou OpenLiteSpeed** | Nginx | cerca de 15 MB |
| | Apache | cerca de 40 MB |
| | OpenLiteSpeed | cerca de 26 MB |
| **Versões do PHP** | PHP 7.4, 8.0, 8.1, 8.2, 8.3, 8.4, 8.5 e 8.6 (e as mais novas assim que o repositório do PHP as tiver), e, em **Versões antigas**, 5.6, 7.0, 7.1, 7.2 e 7.3 | cerca de 25 MB cada |
| | ionCube Loader | nenhuma própria: ele é carregado nos workers do PHP |
| **Bancos de dados e cache** | MariaDB (o da sua distribuição, ou o 11.4, 11.8 ou 12.3 LTS do repositório do MariaDB), ou então MySQL 8.4 LTS | MariaDB: o buffer pool dele mais cerca de 100 MB (veja [Bancos de dados](./databases#how-mariadb-is-tuned)). MySQL: o buffer pool dele mais cerca de 150 MB, ou mais cerca de 300 MB a partir de cerca de 3,5 GB de RAM (veja [Bancos de dados](./databases#how-mysql-is-tuned)) |
| | PostgreSQL | cerca de 30 a 40 MB em repouso, crescendo até o `shared_buffers` mais cerca de 40 MB (veja [PostgreSQL](./postgresql#how-postgresql-is-tuned)) |
| | Redis | até 64, 128 ou 256 MB, conforme a RAM do servidor |
| | Memcached | até 64, 128 ou 256 MB, conforme a RAM do servidor; alguns MB quando vazio |
| | Adminer (interface de banco de dados) | nenhuma em repouso |
| | phpMyAdmin (interface de banco de dados) | nenhuma em repouso |
| **Runtimes** | Node.js | nenhuma em repouso |
| | Apps Python | nenhuma em repouso |
| | Java (OpenJDK) | nenhuma em repouso; o programa Java de um site de app usa o que o heap dele precisa |
| **Contêineres** | Docker | cerca de 90 MB, mais os seus contêineres |
| **Segurança** | Firewall web (ModSecurity) | cerca de 40 MB |
| | Antivírus ClamAV | cerca de 15 MB, e cerca de 1,4 GB de memória livre durante a verificação |
| **Ferramentas** | Composer | nenhuma, ele só roda quando é chamado |
| | WP-CLI | nenhuma, ele só roda quando é chamado |

## Instalar um item {#install-an-item}

1. Vá até **Software**.
2. Selecione **Instalar** no cartão do item.
3. Acompanhe o log. Quando a tarefa termina, o cartão mostra **Instalado** e a versão.

Alguns cartões oferecem então um link: **Abrir Bancos de dados**, **Abrir PostgreSQL**, **Abrir Redis**, **Abrir Memcached**, **Abrir Adminer**, **Abrir phpMyAdmin** ou **Abrir Docker**.

## Desinstalar um item {#uninstall-an-item}

1. Selecione **Desinstalar** no cartão do item. Para uma versão do PHP, essa opção fica no menu da versão.
2. Leia o que a janela diz que a desinstalação faz.
3. Para o MariaDB, o MySQL, o PostgreSQL e o Docker, você pode marcar a caixa para excluir os dados deles também. Você precisa então digitar o nome do item para confirmar.
4. Selecione **Desinstalar**.

### Quando uma desinstalação é recusada {#when-an-uninstall-is-refused}

O painel nunca remove um software que algo ainda usa. Quando um item não pode ser desinstalado, o cartão dele mostra o motivo no lugar de um botão que funcione.

| Item | Recusado enquanto |
|---|---|
| Servidor web | Existir qualquer site, no painel ou como um arquivo de configuração na pasta de sites do servidor web |
| Uma versão do PHP | Um site a usar, ou ela for a única versão do PHP instalada (a configuração do servidor web e o Adminer precisam de uma). Uma versão com suporte (7.4 ou mais nova) também fica enquanto for a única ao lado de [versões antigas](#older-php-versions) |
| Composer | Um script de deploy pelo Git rodar `composer` |
| WP-CLI | Existirem sites WordPress: as ferramentas do WordPress o rodam para eles |
| MariaDB ou MySQL | Ele tiver qualquer banco de dados que não sejam os próprios bancos de dados de sistema dele. O painel pergunta ao próprio servidor, por isso os bancos de dados feitos manualmente ou por outra ferramenta também contam. Se ele estiver parado e a pasta de dados dele tiver sido movida, o painel não consegue conferir e recusa |
| PostgreSQL | O PostgreSQL tiver qualquer banco de dados que não seja `postgres`, `template0` e `template1`, segundo o próprio PostgreSQL, ou estiver parado, de modo que o painel não consegue conferir |
| Node.js | Sites de app usarem o Node.js padrão do servidor, ou o `node` em `/usr/local/bin` não tiver sido instalado pelo painel |
| Uma linha em **Versões do Node.js** | Ela for a padrão do servidor, ou um site de app a tiver escolhido |
| Apps Python | Sites de app rodarem Python |
| Java (OpenJDK) | Um site de app rodar Java. Uma única versão do Java é recusada enquanto um site de app usar essa versão |
| Docker | Sites rodarem apps Docker, existirem projetos Compose, ou o Docker tiver contêineres, rodando ou parados |

O Redis, o Memcached, o Adminer, o phpMyAdmin, o ionCube Loader, o firewall web e o ClamAV sempre podem ser desinstalados.

### O que uma desinstalação mantém {#what-an-uninstall-keeps}

| Item | Mantido | Removido |
|---|---|---|
| Servidor web | As pastas dos sites em `/var/www`, os certificados, o PHP | O servidor web, a configuração dele feita pelo painel, o Adminer e as regras de firewall das portas 80 e 443 |
| Versão do PHP | | A versão, as extensões dela e as bibliotecas que só elas usavam |
| MariaDB | A pasta de dados `/var/lib/mysql`, a menos que você marque **Excluir também o diretório de dados (/var/lib/mysql)** e digite `mariadb`. Instalar o MariaDB de novo a usa | Os pacotes do servidor e do cliente, as configurações do MariaDB feitas pelo painel e o repositório do MariaDB com a chave dele, se o MariaDB veio de lá |
| MySQL | A pasta de dados `/var/lib/mysql`, a menos que você marque **Excluir também o diretório de dados (/var/lib/mysql)** e digite `mysql`. Instalar o MySQL de novo a usa | Os pacotes do servidor e do cliente, as configurações do MySQL feitas pelo painel e o repositório da Oracle com a chave dele |
| PostgreSQL | Os dados em `/var/lib/postgresql` e as configurações do cluster em `/etc/postgresql`, a menos que você marque **Excluir também os dados e as configurações (/var/lib/postgresql e /etc/postgresql)** e digite `postgresql`. Os pacotes são removidos sem purge, e instalar o PostgreSQL de novo inicia o mesmo cluster | Os pacotes do PostgreSQL, e o repositório apt.postgresql.org com a chave dele (`/etc/apt/sources.list.d/panel-pgdg.list` e `/etc/apt/keyrings/panel-pgdg.gpg`), se o painel os acrescentou |
| Redis | | O Redis, a extensão PHP dele e tudo o que está guardado no Redis. O PHP-FPM reinicia |
| Memcached | | O Memcached, a extensão PHP dele e tudo o que está em cache. O PHP-FPM reinicia |
| Adminer | Os seus bancos de dados | Os arquivos do Adminer e o site interno dele |
| phpMyAdmin | Os seus bancos de dados | Os arquivos do phpMyAdmin, os arquivos temporários dele, o site interno dele e as configurações dele |
| Node.js | As outras versões do Node.js. A pasta da padrão também, se um site de app escolheu essa versão | O `node`, o `npm` e o `npx` em `/usr/local/bin`, e a versão para a qual eles apontam |
| Apps Python | O próprio `python3`, de que o sistema precisa, e os ambientes virtuais nas pastas de apps antigas | O venv e os pacotes do pip |
| Java (OpenJDK) | | Todas as versões do Java instaladas, com os pacotes que só elas usavam |
| ionCube Loader | | O loader em todas as versões do PHP e `/usr/local/lib/ioncube`. O PHP-FPM recarrega, e os arquivos PHP codificados param de funcionar |
| Docker | As imagens, os volumes e o cache de build em `/var/lib/docker`, a menos que você marque **Excluir também todas as imagens, os volumes e o cache de build (/var/lib/docker)** e digite `docker`. Instalar o Docker de novo os encontra | O Docker e o plugin Compose dele |
| Firewall web | O log do firewall | O ModSecurity e as regras OWASP. O servidor web reinicia |
| ClamAV | | O ClamAV e os bancos de dados de vírus dele |
| Composer | Os sites Laravel e Drupal, que só precisavam dele para a instalação | `/usr/local/bin/composer` e o pacote `composer` da distribuição |
| WP-CLI | | `/usr/local/lib/panel/wp-cli.phar`. As ferramentas do WordPress o instalam de novo quando precisam |

::: danger
Excluir os dados do MariaDB, do MySQL, do PostgreSQL ou do Docker não pode ser desfeito. Confira se você tem um backup de tudo de que ainda precisa.
:::

## O menu mostra o que está instalado {#the-menu-shows-what-is-installed}

O menu à esquerda esconde as páginas de software que não está instalado:

- **Docker** aparece assim que o Docker é instalado.
- **WordPress** aparece assim que existe um site WordPress.
- Na página **Bancos de dados**, as abas **PostgreSQL**, **Redis** e **Memcached** aparecem assim que esse software é instalado, e **Consultas lentas**, assim que o MariaDB ou o MySQL é instalado.

**Sites**, **Bancos de dados** e **Software** sempre aparecem, porque oferecem as instalações. Uma página que você já tem aberta continua visível.

## Servidor web {#web-server}

O painel roda um servidor web por vez e configura cada site para ele:

- O Nginx é o mais leve. O PHP roda no PHP-FPM.
- O Apache lê os arquivos `.htaccess` que acompanham algumas aplicações PHP, e usa um pouco mais de memória.
- O OpenLiteSpeed lê as regras de reescrita dos arquivos `.htaccess` e tem o LiteSpeed Cache embutido para o WordPress. Ele vem do próprio repositório de pacotes da LiteSpeed.

Instalar um servidor web também instala a versão do PHP da sua distribuição, com o PHP-FPM. Se o firewall ufw estiver ativado, o painel abre as portas 80 e 443, e a 443/udp para o HTTP/3 com o OpenLiteSpeed.

O painel se recusa a instalar um segundo servidor web enquanto outro estiver instalado, porque os dois precisariam das portas 80 e 443.

Para trocar de servidor web:

1. Exclua antes todos os sites. Faça backup deles se você precisar deles depois.
2. Selecione **Desinstalar** no servidor web atual e digite o nome dele (`nginx`, `apache` ou `openlitespeed`) para confirmar.
3. Instale o outro servidor web.

Veja [Sites](./websites) para configurar os sites.

## Versões do PHP {#php-versions}

Você pode instalar várias versões do PHP lado a lado e escolher uma por site em **Sites** → (site) → **PHP**.

- A versão que a sua distribuição traz vem dos pacotes dela. As outras versões vêm do repositório de Ondřej Surý, `packages.sury.org/php`, tanto no Ubuntu quanto no Debian. O painel pergunta antes de acrescentar esse repositório ao servidor, e só aceita a chave de assinatura dele com a impressão digital `15058500 A0235D97 F5D10063 B188E2B6 95BD4743`.
- Cada versão é instalada com estas extensões: fpm, cli, mysql, curl, gd, mbstring, xml, zip, intl, bcmath e opcache. A partir do PHP 8.5, o OPcache faz parte do próprio PHP, por isso a 8.5 e as mais novas não têm um pacote opcache separado. Quando o Redis ou o Memcached está instalado, a extensão redis ou memcached também é acrescentada.
- Antes do PHP 8.0, o json é um pacote próprio e também é acrescentado. O PHP 5.6 também recebe o mcrypt, onde o repositório o tem.
- O painel muda o pool padrão do PHP-FPM da versão para workers sob demanda, por isso uma versão que nenhum site usa quase não custa memória.

As versões listadas seguem o repositório do PHP. Quando o `packages.sury.org/php` está no servidor (ele é acrescentado com a primeira versão do PHP que não é a da sua distribuição), a página oferece as versões que o repositório tem para o seu sistema, lidas das listas de pacotes do apt. Uma nova versão do PHP, como o PHP 8.7, aparece sozinha assim que o repositório a publica e as listas de pacotes são atualizadas (cada instalação roda `apt-get update`, e a tarefa diária do apt do seu sistema também). Uma versão que o repositório não tem não é oferecida. Antes de o repositório ser acrescentado, a página oferece as versões que o Nefo conhece, da 7.4 à 8.6. As versões instaladas, a versão da sua distribuição e as **Versões antigas** sempre aparecem.

Selecione **Gerenciar** em uma versão instalada para abrir as **Extensões** e as **Configurações** dela.

### Versões sem atualizações de segurança {#end-of-life}

Cada versão do PHP recebe atualizações de segurança do PHP até uma data definida (veja [php.net/supported-versions.php](https://www.php.net/supported-versions.php)): a 8.1 até 31 de dezembro de 2025, a 8.2 até 31 de dezembro de 2026, a 8.3 até 31 de dezembro de 2027, a 8.4 até 31 de dezembro de 2028, a 8.5 até 31 de dezembro de 2029 e a 8.6 até 31 de dezembro de 2030. A 7.4 e a 8.0 não recebem mais nenhuma. Depois dessa data, uma versão do `packages.sury.org` mostra o selo **Fim da vida útil — sem atualizações de segurança** e uma linha com a data, na página Software, na janela **Gerenciar** dela e nos menus de versão do PHP dos sites. O painel compara a data com a de hoje, por isso o selo aparece sozinho.

A própria versão do PHP da sua distribuição (a 8.1 no Ubuntu 22.04, a 8.3 no 24.04, a 8.5 no 26.04, a 8.2 no Debian 12, a 8.4 no Debian 13) nunca mostra o selo: a equipe de segurança da distribuição continua aplicando correções nela enquanto a versão da distribuição tiver suporte.

Tire os sites de uma versão com o selo quando puder. Você ainda pode instalá-la sem uma confirmação extra; só as [versões antigas](#older-php-versions) pedem.

### Versões antigas do PHP {#older-php-versions}

O PHP 5.6, 7.0, 7.1, 7.2 e 7.3 ficam em **Versões antigas**, recolhidas embaixo das outras versões. Elas estão lá para os sites trazidos de outro servidor (cPanel, aaPanel…) que ainda não rodam em um PHP mais novo.

::: danger Fim da vida útil: sem atualizações de segurança
O PHP parou as atualizações de segurança dessas versões há anos: a 5.6 em 2018, a 7.3 em 2021 (veja [php.net/eol.php](https://www.php.net/eol.php)). As vulnerabilidades conhecidas delas nunca serão corrigidas, por isso um site que roda em uma delas é mais fácil de invadir. Use-as só para manter um site antigo rodando enquanto você o atualiza, e depois passe-o para uma versão com suporte. Não as use para sites novos.
:::

- Cada uma mostra o selo **Fim da vida útil — sem atualizações de segurança**, na página Software e nos menus de versão do PHP dos sites.
- Instalar uma delas pede antes que você confirme esse risco.
- Uma versão com suporte (7.4 ou mais nova) precisa ser instalada antes, e a última versão com suporte não pode ser desinstalada enquanto houver versões antigas instaladas. Assim, uma versão antiga nunca é o único PHP do servidor: a configuração do servidor web, o Adminer, o Composer e as ferramentas do painel mantêm uma com suporte.
- A loja de aplicativos mantém as suas próprias versões mínimas: o WordPress precisa do PHP 7.4, e os outros apps de mais. As ferramentas do WordPress ainda gerenciam um site WordPress em uma versão antiga, mas não atualizam o próprio WordPress ali, porque o WordPress atual precisa do PHP 7.4. Elas ainda atualizam plugins e temas; o WP-CLI pula as atualizações que precisam de um PHP mais novo.
- O pool do PHP-FPM do site e as configurações do php.ini são os mesmos das versões mais novas: todas as configurações que o painel grava existem no PHP 5.6.

O `packages.sury.org/php` tem o PHP 5.6 a 7.3 para o Ubuntu 22.04, 24.04 e 26.04 e o Debian 12 e 13, em amd64 e arm64. A instalação também confere com o apt, e recusa com um motivo se o repositório não tiver a versão.

### Servidores que usaram o PPA ondrej/php {#ondrej-php-ppa}

As versões anteriores do painel pegavam as versões extras do PHP no Ubuntu do `ppa:ondrej/php`. Esse PPA está congelado e não recebe mais atualizações: o mantenedor dele agora publica os pacotes para o Ubuntu em `packages.sury.org/php` (veja o aviso em [launchpad.net/~ondrej/+archive/ubuntu/php](https://launchpad.net/~ondrej/+archive/ubuntu/php)).

Quando o painel inicia em um servidor Ubuntu que ainda tem o PPA, ele faz a troca do servidor uma vez, do jeito que o aviso do PPA descreve:

1. Ele acrescenta o `packages.sury.org/php` com a chave fixada dele (`/usr/share/keyrings/sury-php.gpg` e `/etc/apt/sources.list.d/sury-php.list`), a menos que já exista uma origem para ele.
2. Ele move o arquivo de origem do PPA de `/etc/apt/sources.list.d`, e a chave do PPA `/etc/apt/trusted.gpg.d/ondrej-ubuntu-php.gpg`, se houver, para `apt-backup` na pasta de dados do painel (`/var/lib/nefo/apt-backup`). Essa chave era confiável para todos os repositórios, e nada precisa mais dela. Os outros PPAs, mesmo do mesmo mantenedor, ficam como estão.

Nada é instalado, removido ou rebaixado de versão, e o painel não roda `apt-get update` na inicialização. A próxima atualização (a verificação de atualizações do painel, uma instalação ou o seu próprio `apt update`) lê a nova origem, e os pacotes PHP instalados recebem as próximas atualizações a partir dela. O log do painel diz o que ele alterou. Se a chave não puder ser baixada, o PPA fica e o painel tenta de novo na próxima inicialização. Uma linha do PPA no próprio `/etc/apt/sources.list`, ou em um arquivo que também lista outros repositórios, não é editada: o log pede que você a substitua manualmente.

### Extensões {#extensions}

A aba **Extensões** lista as extensões opcionais que têm um pacote para esta versão. Selecione **Instalar** ou **Remover**, e o PHP-FPM recarrega.

As extensões opcionais são: apcu, gmp, igbinary, imagick, imap, ldap, memcached, mongodb, msgpack, pgsql, redis, soap, sqlite3, ssh2, tidy, xdebug, xsl e yaml.

::: warning
O xdebug deixa mais lentos todos os sites daquela versão do PHP. Não o deixe instalado em um servidor de produção.
:::

### Configurações do php.ini {#php-ini-settings}

A aba **Configurações** define valores do php.ini para todos os sites desta versão do PHP. Deixe um campo vazio para usar o padrão do PHP e depois selecione **Salvar**.

| Configuração | Padrão | Valores permitidos |
|---|---|---|
| `date.timezone` | UTC | Um nome de fuso horário, como `Europe/Berlin` |
| `max_input_vars` | 1000 | 100 a 100000 |
| `max_input_time` | 60 | -1 a 3600 |
| `default_socket_timeout` | 60 | 1 a 600 |
| `session.gc_maxlifetime` | 1440 | 60 a 2592000 |
| `realpath_cache_size` | 4096K | Um número, opcionalmente seguido de K ou M |
| `display_errors` | Off | On ou Off |
| `expose_php` | Off | On ou Off |
| `allow_url_fopen` | On | On ou Off |
| `short_open_tag` | Off | On ou Off |
| `opcache.enable` | 1 | 1 ou 0 |
| `opcache.memory_consumption` | 128 | 32 a 2048 |
| `opcache.max_accelerated_files` | 10000 | 1000 a 1000000 |
| `opcache.validate_timestamps` | 1 | 1 ou 0 |
| `opcache.revalidate_freq` | 2 | 0 a 3600 |

Os valores são gravados em `/etc/php/<version>/fpm/conf.d/99-panel.ini` e conferidos com `php-fpm -t`. Se o PHP-FPM os rejeitar, o arquivo anterior volta e nada muda. Os limites de memória, de tamanho de upload e de tempo são definidos por site, nas configurações do site.

### ionCube Loader {#ioncube}

O cartão **ionCube Loader**, embaixo das versões do PHP, roda arquivos PHP codificados com o ionCube Encoder. Alguns scripts PHP comerciais precisam dele, como o WHMCS e outros softwares de cobrança ou de licenciamento, e os sites trazidos do cPanel muitas vezes precisam.

1. Instale antes as versões do PHP que os seus sites usam.
2. No cartão **ionCube Loader**, selecione **Instalar**.

O painel baixa os loaders da ionCube (`downloads.ioncube.com`) por HTTPS. Ele instala uma versão fixada, a 15.5.1, e confere o download com o checksum SHA-256 que o painel tem para ela, porque a ionCube não publica checksums nem assinaturas. Um download que não confere é recusado e nada muda. Funciona em servidores amd64 e arm64.

Os loaders vão para `/usr/local/lib/ioncube`, com a licença da ionCube (`LICENSE.txt`). Para cada versão do PHP instalada que o loader aceita, o painel grava `00-ioncube.ini` em `/etc/php/<version>/fpm/conf.d` e `/etc/php/<version>/cli/conf.d`, para que o loader carregue antes do OPcache, como a ionCube exige. Ele confere se `php<version> -v` inicia com o loader e depois recarrega o PHP-FPM. Se o PHP não iniciar com ele, o painel tira o arquivo de novo, para que essa versão continue funcionando sem o loader, e a tarefa diz qual versão falhou.

O cartão lista cada versão do PHP instalada com o que o loader faz para ela:

| Estado | Significado |
|---|---|
| **carregado** | O loader está carregado para o PHP-FPM e para a linha de comando |
| **não carregado** | Existe um loader para esta versão, mas ele não está carregado. Selecione **Carregar em todas as versões** no cartão |
| **A ionCube não faz loader para esta versão** | PHP 8.0: a ionCube nunca publicou um loader para ela |
| "ainda não disponível para o PHP 8.6" | Uma versão do PHP mais nova que os loaders que o painel instala (a 15.5.1 aceita até o PHP 8.5) |

A versão fixada tem loaders para o PHP 5.6, 7.0 a 7.4 e 8.1 a 8.5. Uma versão do PHP que você instalar depois também recebe o loader, quando existir um para ela. Desinstalar uma versão do PHP remove o `00-ioncube.ini` dela.

**Desinstalar** tira o loader de todas as versões do PHP, recarrega o PHP-FPM e exclui `/usr/local/lib/ioncube`. Os arquivos PHP codificados então param de funcionar.

Para conferir um site, coloque um arquivo como este na pasta dele e abra-o, e depois exclua o arquivo:

```php
<?php var_dump(extension_loaded('ionCube Loader'));
```

::: tip
O ionCube Loader tem uso gratuito, mas não é de código aberto. A licença da ionCube ([ioncube.com/legal/loaders/eula](https://www.ioncube.com/legal/loaders/eula)) permite usá-lo sem custo. O painel o baixa da ionCube quando você o instala, e não o distribui.
:::

## Node.js {#node-js}

O cartão **Node.js** instala a versão LTS mais nova do nodejs.org, conferida com o checksum SHA-256 publicado dela. Ela vai para `/opt/node-v<version>-linux-<arch>`, e o `node`, o `npm` e o `npx` são ligados em `/usr/local/bin`. Esse é o Node.js padrão do servidor, usado pelos sites de app que não escolhem uma versão.

### Várias versões do Node.js {#several-node-js-versions}

Depois que o Node.js está instalado, o cartão **Versões do Node.js** lista as três linhas LTS mais novas e qualquer linha instalada. Selecione **Instalar** para acrescentar uma linha ao lado das outras. Isso não muda o padrão.

Escolha uma linha para um site de app em **Sites** → (site) → **Runtime**. Uma linha que sites de app usam, e a padrão, não podem ser desinstaladas neste cartão. Desinstale a padrão no cartão **Node.js**.

A lista vem do nodejs.org, por isso o servidor precisa de acesso à internet para mostrá-la.

::: warning
Desinstalar o Node.js padrão também quebra as tarefas cron e os programas em segundo plano que chamam o `node`.
:::

## Python {#python}

O cartão **Apps Python** instala o `python3-venv` e o `python3-pip`, para que os sites de app possam rodar Python (Django, Flask, FastAPI…). Cada site de app Python recebe o seu próprio ambiente virtual em `app/.venv`. As ferramentas de compilação, como o `build-essential` e o `python3-dev`, não são instaladas, por isso um pacote que não tem um wheel pronto e precisa ser compilado falha na instalação até você acrescentá-las com o `apt`.

Desinstalar remove o venv e o pip. O próprio `python3` fica, porque o sistema precisa dele.

## Java {#java}

O cartão **Java (OpenJDK)** instala o OpenJDK da sua distribuição, para sites de app escritos em Java, como uma aplicação Spring Boot iniciada com `java -jar`. É o JDK headless (`openjdk-<version>-jdk-headless`): o `java`, mais o `javac`, o `jar` e as outras ferramentas de que um build em um script de [deploy pelo Git](./git-deploy) precisa, sem as partes de desktop. Ele vai para `/usr/lib/jvm/java-<version>-openjdk-<arch>` e recebe as atualizações de segurança com o resto do sistema.

**Instalar**, no cartão, instala a versão de suporte de longo prazo (LTS) mais nova que a sua distribuição tem. As versões disponíveis dependem do sistema:

| Sistema | Versões do Java |
|---|---|
| Ubuntu 22.04 | 17, 21, 25 |
| Ubuntu 24.04 | 17, 21, 25 |
| Ubuntu 26.04 | 17, 21, 25 |
| Debian 12 | 17 |
| Debian 13 | 21, 25 |

### Várias versões do Java {#several-java-versions}

Depois que o Java está instalado, o cartão **Versões do Java** lista as versões LTS que a sua distribuição oferece e qualquer versão instalada. Selecione **Instalar** para acrescentar uma versão ao lado das outras. Uma versão LTS mais nova aparece sozinha assim que a distribuição a tiver.

Escolha uma versão para um site de app em **Sites** → (site) → **Runtime** (veja [Sites de app Java](./websites#java-app-websites)). Uma versão que sites de app usam não pode ser desinstalada, e o cartão mostra que há apps usando-a. **Desinstalar**, no cartão **Java (OpenJDK)**, remove todas as versões instaladas, e é recusado enquanto algum site de app rodar Java.

Cada versão ocupa mais ou menos de 390 a 480 MB de disco, com os pacotes baixados. Nada roda enquanto nenhum site a usa.

## Redis {#redis}

O Redis é um cache em memória. O WordPress pode usá-lo com um plugin de cache de objetos do Redis.

Quando você instala o Redis, o painel:

- instala o `redis-server` e a extensão redis para cada versão do PHP instalada;
- limita a memória do Redis de acordo com o servidor: 64 MB abaixo de 1,5 GB de RAM, 128 MB a partir de 1,5 GB, 256 MB a partir de 3 GB;
- define a política `allkeys-lru`, para que o Redis descarte as chaves usadas há mais tempo quando estiver cheio, como um cache;
- deixa o Redis escutando em `127.0.0.1:6379`, acessível só a partir deste servidor.

Para examinar o Redis, vá até **Bancos de dados** → **Redis**. Veja [Bancos de dados](./databases#redis-key-browser).

Antes de desinstalar o Redis, desative o plugin de cache dos seus sites. Eles continuam funcionando sem o cache, um pouco mais lentos.

## PostgreSQL {#postgresql}

O cartão **PostgreSQL** instala a versão da sua distribuição, ou outra versão principal do repositório do projeto PostgreSQL (apt.postgresql.org), como você escolher na janela de instalação. O painel o ajusta para a memória do servidor em um arquivo `conf.d`, deixa-o escutando só em localhost e grava um `pg_hba.conf` que pede senha a todos, exceto ao usuário de sistema `postgres` no socket local. Os bancos de dados dele ficam em **Bancos de dados** → **PostgreSQL**. Veja [PostgreSQL](./postgresql) para as versões, a atualização de versão, o acesso remoto, as regras de importação e os backups.

Quando ele vem do apt.postgresql.org, o cartão diz **Do apt.postgresql.org**; caso contrário, **Da distribuição**. Desinstalá-lo também remove esse repositório, se o painel o acrescentou.

Quando o Adminer está instalado, o painel acrescenta a extensão `pgsql` do PHP para a versão do PHP do Adminer, para que o Adminer também abra bancos de dados PostgreSQL. A extensão só é acrescentada junto com o PostgreSQL.

## Memcached {#memcached}

O Memcached é um pequeno cache em memória, para sessões PHP e caches de aplicações (o driver de cache `memcached` do Laravel, por exemplo).

Quando você instala o Memcached, o painel:

- instala o `memcached` e a extensão memcached para cada versão do PHP instalada, e acrescenta a extensão às versões do PHP que você instalar depois;
- grava o `/etc/memcached.conf`: o Memcached escuta só em `127.0.0.1:11211`, por TCP, com o UDP desativado (`-U 0`), para que ele não possa ser usado para amplificar ataques;
- limita a memória dele de acordo com o servidor: 64 MB abaixo de 1,5 GB de RAM, 128 MB a partir de 1,5 GB, 256 MB a partir de 3 GB, e permite 512 conexões abaixo de 1,5 GB de RAM e 1024 a partir daí.

Quando a memória está cheia, o Memcached descarta os itens usados há mais tempo. Para ver os números dele, vá até **Bancos de dados** → **Memcached**.

Antes de desinstalar o Memcached, aponte as sessões e o cache dos seus sites para outro lugar: uma aplicação cujo driver de cache é o Memcached falha sem ele.

## Servidor de banco de dados, Adminer e phpMyAdmin {#mariadb-and-adminer}

O cartão **Servidor de banco de dados** instala o MariaDB ou o MySQL 8.4 LTS, ajustado à memória do servidor e acessível só a partir deste servidor. Um servidor tem um deles.

Enquanto nenhum dos dois está instalado, o cartão oferece a escolha, com o **MariaDB** selecionado: ele é mais leve e é o que o WordPress e a maioria das aplicações PHP rodam.

- Em **MariaDB**, você escolhe a versão quando o próprio repositório do MariaDB tem uma série LTS mais nova que a da sua distribuição: a da sua distribuição (o padrão, como "10.11, a da distribuição"), ou **11.4 LTS do MariaDB**, **11.8 LTS do MariaDB** ou **12.3 LTS do MariaDB** (no Ubuntu 26.04 e no Debian 13, só a 12.3). Cada uma diz até quando o MariaDB dá suporte a ela. Veja [Versões do MariaDB](./databases#mariadb-versions).
- O **MySQL 8.4 LTS** vem do repositório da Oracle e é oferecido no Ubuntu 22.04, 24.04 e 26.04 e no Debian 12 e 13, só em servidores amd64. Nos outros, o cartão diz por que ele não é oferecido.

Depois que um deles está instalado, o cartão mostra qual, a versão e de onde ele vem (**dos pacotes da distribuição**, **do repositório do MariaDB** ou **do repositório da Oracle**), a estimativa de memória, o número de bancos de dados, **Abrir Bancos de dados** e **Desinstalar**. Ele também oferece:

- **Atualizar versão**, no MariaDB, quando uma série LTS mais nova é oferecida: veja [Atualizar a versão do MariaDB](./databases#upgrade-mariadb).
- O outro servidor como alternativa, **MySQL 8.4 LTS no lugar** ou **MariaDB no lugar**, com um botão para trocar: o painel leva os bancos de dados, os usuários e as senhas deles, e volta atrás se algo falhar. Veja [Trocar entre o MariaDB e o MySQL](./databases#switch-between-mariadb-and-mysql).
- **Dumps guardados**: as exportações que uma atualização de versão ou uma troca fez, com a pasta, o tamanho e um botão para excluí-las. Veja [Dumps das atualizações de versão e das trocas](./databases#dumps-from-upgrades-and-switches).

O Adminer e o phpMyAdmin mostram os seus bancos de dados no navegador e só são acessíveis pelo login do painel. Os dois precisam de um servidor web e do PHP, e os dois funcionam com o MariaDB e o MySQL. Veja [Bancos de dados](./databases#phpmyadmin).

## Docker {#docker}

O cartão **Docker** instala o `docker.io` da sua distribuição, com o plugin Compose dele quando houver um. O item **Docker** do menu aparece então.

As portas que os contêineres Docker publicam escapam do firewall ufw. Por isso o painel publica as portas dos contêineres só em 127.0.0.1, e você as acessa por um site. Veja [Docker](./docker).

## Firewall web e ClamAV {#web-firewall-and-clamav}

- **Firewall web (ModSecurity)** instala o ModSecurity com o OWASP Core Rule Set. Ele precisa do servidor web. Depois de instalar, ative-o por site em **Sites** → (site) → **Firewall web (WAF)**. Onde a distribuição não tem módulo ModSecurity para o servidor web, o cartão diz por quê e não oferece instalação: o Ubuntu 22.04 não tem um para o Nginx. O Apache e o OpenLiteSpeed funcionam lá, e o Nginx funciona a partir do Ubuntu 24.04.
- **Antivírus ClamAV** é um mecanismo extra opcional para as verificações de malware. Os bancos de dados de vírus dele são baixados logo depois da instalação, por isso as verificações podem usá-lo alguns minutos depois. Ele serve para servidores com 2 GB de RAM ou mais.

Veja [Segurança](./security) para os dois.

## Ferramentas: Composer e WP-CLI {#tools}

O grupo **Ferramentas** mostra duas ferramentas de linha de comando que o painel instala quando algo precisa delas. Cada cartão mostra se a ferramenta está instalada, a versão, de onde ela vem e o que a usa. **Atualizar** instala a versão oficial mais recente da ferramenta. O painel a baixa por HTTPS com o checksum que o projeto publica ao lado dela, e recusa um download que não confere: a ferramenta instalada fica então como estava. O novo arquivo substitui o antigo em um único passo.

### Composer {#composer}

O Composer é o gerenciador de pacotes do PHP. Instalar o Laravel ou o Drupal pela loja de aplicativos o instala, e os scripts de deploy pelo Git e o terminal também podem rodá-lo.

- **Instalar** instala o pacote `composer` da sua distribuição, que recebe as atualizações de segurança com o resto do sistema.
- **Atualizar** instala a versão estável mais recente do getcomposer.org em `/usr/local/bin/composer`, conferida com o checksum SHA-256 publicado dela. Esse vem primeiro no `PATH`, por isso o `composer` no terminal, os scripts de deploy pelo Git e as próprias instalações do Laravel e do Drupal feitas pelo painel usam todos ele. O pacote da distribuição continua instalado.
- **Desinstalar** remove os dois. É recusado enquanto um script de deploy pelo Git rodar `composer`. Os sites Laravel e Drupal não o impedem: eles só precisam do Composer para a instalação e continuam funcionando sem ele. Instalar um deles de novo traz o Composer de volta.

### WP-CLI {#wp-cli}

O WP-CLI roda o WordPress pela linha de comando. As [Ferramentas do WordPress](./wordpress) o usam para as atualizações, o modo de manutenção, as senhas e as cópias de staging, e o instalam na primeira vez que precisam dele.

- **Instalar** e **Atualizar** baixam a versão mais recente do GitHub para `/usr/local/lib/panel/wp-cli.phar`, conferida com o checksum SHA-512 que a versão publica.
- **Desinstalar** é recusado enquanto existirem sites WordPress.
