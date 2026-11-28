---
title: Bancos de dados
description: Instale o MariaDB ou o MySQL, escolha uma versão do MariaDB, atualize a versão do MariaDB, troque entre o MariaDB e o MySQL, crie bancos de dados, importe e exporte SQL, permita o acesso remoto, leia o log de consultas lentas e use o Adminer, o phpMyAdmin e o explorador de chaves do Redis.
---

# Bancos de dados

O Nefo roda um servidor de banco de dados compatível com MySQL por servidor: o MariaDB (o da sua distribuição, ou o 11.4, 11.8 ou 12.3 LTS do MariaDB) ou o MySQL 8.4 LTS, ajustado para uma VPS pequena e acessível só a partir do servidor. A página **Bancos de dados** cria bancos de dados e usuários, importa e exporta SQL, abre um banco de dados para endereços escolhidos e mostra as consultas lentas. O PostgreSQL pode rodar ao lado dele.

Esta página trata de:

- escolher o MariaDB ou o MySQL e uma versão do MariaDB, instalá-lo, como o Nefo o ajusta para um servidor pequeno, atualizar a versão do MariaDB e trocar de um para o outro;
- criar bancos de dados, mostrar os logins deles e trocar senhas;
- importação e exportação de SQL;
- acesso remoto para um banco de dados por vez;
- o log de consultas lentas;
- o Adminer e o phpMyAdmin, interfaces de banco de dados no navegador;
- o explorador de chaves do Redis, na aba **Redis** da mesma página, e o status do Memcached.

O PostgreSQL tem a sua própria aba **PostgreSQL** nesta página e a sua própria página na documentação: veja [PostgreSQL](./postgresql). O MongoDB, e o PostgreSQL como contêiner, estão disponíveis na [loja de aplicativos](./apps) como apps Docker que escutam só em 127.0.0.1. Esses apps Docker não aparecem na página **Bancos de dados**.

## Escolher o MariaDB ou o MySQL {#choose-mariadb-or-mysql}

Um servidor roda um único servidor de banco de dados compatível com MySQL: o MariaDB ou o MySQL 8.4 LTS. Você escolhe ao instalá-lo, na página **Bancos de dados**, no cartão **Servidor de banco de dados** da página **Software** ou na configuração inicial (**Configurar este servidor**). Enquanto um estiver instalado, o painel se recusa a instalar o outro; você pode [trocar](#switch-between-mariadb-and-mysql) de um para o outro depois, levando os seus bancos de dados.

| | MariaDB (recomendado) | MySQL 8.4 LTS |
|---|---|---|
| Vem de | Os pacotes da sua distribuição, ou uma série LTS do próprio repositório do MariaDB (veja [Versões do MariaDB](#mariadb-versions)) | O repositório da Oracle, `repo.mysql.com` (a trilha `mysql-8.4-lts`) |
| Memória, mais ou menos | O buffer pool dele mais cerca de 100 MB | O buffer pool dele mais cerca de 150 MB, ou mais cerca de 300 MB com o performance schema ativado, que o painel ativa a partir de cerca de 3,5 GB de RAM (portanto em servidores de 4 GB) |
| Suporte | Enquanto a sua distribuição der suporte a ele; uma série do repositório do MariaDB até a data de fim que o MariaDB definiu para ela | A Oracle dá suporte ao 8.4 LTS até 2032 |
| Sistemas | Todos os sistemas que o Nefo aceita | Ubuntu 22.04, 24.04 e 26.04 e Debian 12 e 13, só em amd64 (x86-64) |
| Bom para | WordPress e a maioria das aplicações PHP | Aplicações que precisam do próprio MySQL, ou uma equipe acostumada a ele |

Os dois funcionam com tudo o que há nesta página, com os backups, as cópias de sites, o staging do WordPress e as aplicações PHP da loja de aplicativos. O Adminer e o phpMyAdmin abrem os dois. Em um servidor arm64, ou em um sistema para o qual a Oracle não tem pacotes, a página **Software** só oferece o MariaDB e diz por quê.

## Instalar o MariaDB {#install-mariadb}

1. Vá até **Bancos de dados**.
2. Selecione **Instalar MariaDB (recomendado)**. Você também pode instalá-lo na página **Software**: mantenha **MariaDB** selecionado no cartão **Servidor de banco de dados** e selecione **Instalar MariaDB**.
3. Espere de um a dois minutos. O log de progresso mostra cada etapa.

O Nefo instala os pacotes `mariadb-server` e `mariadb-client` da sua distribuição. Depois grava as suas configurações, inicia o MariaDB e remove os usuários anônimos e o banco de dados `test` que algumas versões trazem.

Para instalar uma série mais nova do próprio repositório do MariaDB, veja [Versões do MariaDB](#mariadb-versions).

### Como o MariaDB é ajustado {#how-mariadb-is-tuned}

O Nefo grava as suas configurações em `/etc/mysql/mariadb.conf.d/90-panel.cnf`. A maior configuração de memória, o buffer pool do InnoDB, depende da RAM do servidor:

| RAM do servidor | `innodb_buffer_pool_size` |
|---|---|
| Menos de 1,5 GB | 128M |
| 1,5 GB ou mais | 256M |
| 3 GB ou mais | 512M |
| 7 GB ou mais | 1G |

As outras configurações são iguais em todo servidor:

| Configuração | Valor | Por quê |
|---|---|---|
| `bind-address` | `127.0.0.1` | O MariaDB só aceita conexões deste servidor |
| `skip-name-resolve` | ativado | Nenhuma consulta DNS a cada conexão |
| `performance_schema` | `OFF` | Economiza memória |
| `max_connections` | `100` | |
| `character-set-server` | `utf8mb4` | Unicode completo, inclusive emojis |
| `collation-server` | `utf8mb4_unicode_ci` | |

O MariaDB então usa mais ou menos o tamanho do buffer pool dele mais 100 MB. A página **Software** mostra essa estimativa.

::: warning
Não edite o `90-panel.cnf`. O painel o reescreve.
:::

## Quais versões do MariaDB posso instalar? {#mariadb-versions}

Por padrão, o MariaDB vem da sua distribuição. As correções de segurança dele chegam então com as atualizações do sistema, e nenhum outro repositório é acrescentado. Você pode, em vez disso, instalar uma série de suporte de longo prazo (LTS) do próprio repositório do MariaDB, `dlm.mariadb.com`, quando precisar de um MariaDB mais novo do que o da sua distribuição:

| Série | O MariaDB dá suporte até | Oferecida no |
|---|---|---|
| 11.4 LTS | 29 de maio de 2029 | Ubuntu 22.04 e 24.04, Debian 12 |
| 11.8 LTS | 4 de junho de 2028 | Ubuntu 22.04 e 24.04, Debian 12 |
| 12.3 LTS | 12 de junho de 2029 | Ubuntu 22.04, 24.04 e 26.04, Debian 12 e 13 |

Uma série só é oferecida onde é mais nova que o próprio MariaDB da distribuição:

| Sistema | O MariaDB da distribuição | Séries oferecidas |
|---|---|---|
| Ubuntu 22.04 | 10.6 | 11.4, 11.8, 12.3 |
| Ubuntu 24.04 | 10.11 | 11.4, 11.8, 12.3 |
| Debian 12 | 10.11 | 11.4, 11.8, 12.3 |
| Ubuntu 26.04 | 11.8 | 12.3 |
| Debian 13 | 11.8 | 12.3 |

O MariaDB 12.3 não instala o Galera (o software de cluster do MariaDB, que um servidor único não usa); o 11.4 e o 11.8 o trazem junto, como os pacotes deles exigem.

Para instalar uma série:

1. Vá até **Software**. No cartão **Servidor de banco de dados**, mantenha **MariaDB** selecionado.
2. Embaixo dele, escolha a versão, como **11.4 LTS do MariaDB**. A versão da sua distribuição, como "10.11, a da distribuição", continua sendo o padrão.
3. Selecione **Instalar MariaDB 11.4**.

A configuração inicial (**Configurar este servidor**) oferece as mesmas versões em **MariaDB**, e o `nefo setup --db mariadb-11.4` também (veja [Linha de comando](./cli#nefo-setup)).

O que o painel faz para uma série do repositório do MariaDB:

- Ele baixa a chave de assinatura do MariaDB de `https://supplychain.mariadb.com/MariaDB-Server-GPG-KEY` (ou do `keyserver.ubuntu.com`) e só a aceita se a impressão digital dela for exatamente `177F 4010 FE56 CA33 3630 0305 F165 6F24 C74C D1D8`, a "MariaDB Signing Key" que a documentação do MariaDB lista. Ele a salva como `/etc/apt/keyrings/panel-mariadb.gpg`, confiável só para esse repositório.
- Ele grava `/etc/apt/sources.list.d/panel-mariadb.list` com uma linha só para essa série, como `https://dlm.mariadb.com/repo/mariadb-server/11.4/repo/ubuntu noble main`. O seu servidor então recebe as versões de correção da 11.4 com as atualizações do sistema, nunca uma série mais nova por conta própria.
- Ele confere se o apt vai mesmo instalar os pacotes do MariaDB dessa série e depois instala o `mariadb-server` e o `mariadb-client`, ajustados como o MariaDB da distribuição.

Desinstalar o MariaDB também remove o arquivo do repositório e a chave.

O painel se recusa a:

- instalar uma série enquanto outro MariaDB estiver instalado: use **Atualizar versão** (veja abaixo);
- instalar um MariaDB mais antigo do que aquele cujos dados estão em `/var/lib/mysql`, mantidos de uma instalação anterior, porque ele não conseguiria lê-los. Instale essa versão ou uma mais nova, ou tire o diretório de dados de lá antes.

## Atualizar a versão do MariaDB {#upgrade-mariadb}

Você pode atualizar o MariaDB instalado para uma série LTS mais nova do repositório do MariaDB, como da 10.11 da distribuição para a 11.4, ou da 11.8 do Debian 13 para a 12.3, com os seus bancos de dados no lugar.

1. Vá até **Software**. No cartão **Servidor de banco de dados**, selecione **Atualizar versão**.
2. Escolha a série. A janela mostra o que vai acontecer e quanto espaço livre em disco é necessário.
3. Selecione **Atualizar para o MariaDB 11.4** e acompanhe o log.

A atualização roda nesta ordem:

1. Cada banco de dados é exportado, um arquivo SQL compactado com gzip para cada um, em uma nova pasta de `/var/backups/panel/db-server/`. O painel guarda esses dumps até você excluí-los.
2. Os pacotes da nova série são baixados, e os da série instalada são guardados, para que a volta funcione sem rede.
3. O MariaDB fica somente leitura, o painel conta as linhas de todas as tabelas e o MariaDB para. O diretório de dados dele é copiado para `/var/lib/mysql.before-11.4-<date>-<time>`, e o `/etc/mysql`, para a pasta dos dumps. A partir daqui, os sites não conseguem usar os bancos de dados até a atualização terminar, geralmente alguns minutos.
4. Os novos pacotes substituem os antigos e iniciam o MariaDB nos mesmos dados, ainda somente leitura. O `mariadb-upgrade` então atualiza as tabelas de sistema dele.
5. O painel confere se o MariaDB informa a nova versão, se cada banco de dados tem as mesmas tabelas com o mesmo número de linhas, views, rotinas, triggers e eventos, e se cada usuário de banco de dados do painel consegue entrar com a sua senha.
6. O MariaDB volta a aceitar gravações, e a cópia do diretório de dados é excluída.

Se qualquer passo a partir do 4 falhar, o painel coloca o MariaDB antigo de volta: remove os novos pacotes, restaura o `/etc/mysql` e o diretório de dados copiado, instala de novo os pacotes antigos e inicia o MariaDB. O log termina com o motivo. Os seus bancos de dados ficam então como estavam antes da atualização.

::: warning Sem volta para uma série mais antiga
Depois de uma atualização, o MariaDB atualizou os seus arquivos de dados e as tabelas de sistema, que um MariaDB mais antigo não consegue ler. O painel recusa um downgrade, como da 11.8 para a 11.4, e diz por quê. Para voltar, exporte os bancos de dados, desinstale o MariaDB com os dados, instale a versão mais antiga e importe-os.
:::

O painel precisa de espaço livre em disco para os dumps, a cópia do diretório de dados e os pacotes: mais ou menos uma vez e meia o diretório de dados mais 400 MB. Ele se recusa a começar sem isso. Ele também recusa enquanto o MariaDB estiver parado, ou quando a configuração do servidor tira o diretório de dados de `/var/lib/mysql`.

## Instalar o MySQL {#install-mysql}

1. Vá até **Bancos de dados** e selecione **Instalar o MySQL 8.4 em vez disso**. Na página **Software**, escolha **MySQL 8.4 LTS** no cartão **Servidor de banco de dados** e selecione **Instalar MySQL 8.4 LTS**.
2. Espere de dois a quatro minutos. O log de progresso mostra cada etapa.

O próprio Nefo acrescenta o repositório da Oracle, em vez de instalar o pacote `mysql-apt-config` da Oracle:

- Ele baixa a chave de assinatura de `https://repo.mysql.com/RPM-GPG-KEY-mysql-2025` e só a aceita se a impressão digital dela for exatamente `BCA4 3417 C3B4 85DD 128E C6D4 B7B3 B788 A8D3 785C`, a chave MySQL Release Engineering que a documentação do MySQL lista. Ele salva a chave como `/etc/apt/keyrings/panel-mysql.gpg`.
- O `/etc/apt/sources.list.d/panel-mysql.list` confia nessa chave só para esse repositório, e lista só a trilha `mysql-8.4-lts`.
- A Oracle renova a chave de tempos em tempos; a cópia atual expira em 23 de outubro de 2027. Uma vez por semana, o painel procura uma cópia renovada da mesma chave e substitui a antiga, para que o `apt-get update` continue funcionando.

Depois ele instala o `mysql-community-server` e o `mysql-community-client` sem perguntas. A conta `root` do MySQL não tem senha: ela entra só pelo socket local (`auth_socket`), como root no servidor, e o painel não guarda nenhuma senha do root. Por último, o painel remove qualquer usuário anônimo e o banco de dados `test`.

### Como o MySQL é ajustado {#how-mysql-is-tuned}

O Nefo grava `/etc/mysql/mysql.conf.d/90-panel.cnf` antes de o pacote iniciar o MySQL pela primeira vez, por isso o MySQL nunca roda com os padrões da Oracle no seu servidor. As configurações de memória dependem da RAM do servidor:

| RAM do servidor | `innodb_buffer_pool_size` | `max_connections` | `table_open_cache` | `temptable_max_ram` |
|---|---|---|---|---|
| Menos de 1,5 GB | 128M | 75 | 400 | 32M |
| 1,5 GB ou mais | 256M | 100 | 600 | 64M |
| 3 GB ou mais | 512M | 150 | 1000 | 128M |
| 7 GB ou mais | 1024M | 200 | 2000 | 256M |

As outras configurações:

| Configuração | Valor | Por quê |
|---|---|---|
| `bind-address` | `127.0.0.1` | O MySQL só aceita conexões deste servidor, até que um banco de dados permita o acesso remoto |
| `mysqlx` | `OFF` | Nenhum listener do X Protocol na porta 33060: o PHP e o painel usam o protocolo clássico |
| `disable_log_bin` | ativado | Nenhum log binário enchendo o disco: o servidor não tem réplicas |
| `performance_schema` | `OFF` abaixo de cerca de 3,5 GB de RAM, `ON` a partir daí (portanto em servidores de 4 GB) | Ele sozinho ocupa de 100 a 200 MB |
| `skip-name-resolve`, `host_cache_size` | ativado, `0` | Nenhuma consulta DNS a cada conexão |
| `innodb_log_buffer_size` | `16M` | O padrão do MySQL 8.4 é 64M |
| `character-set-server`, `collation-server` | `utf8mb4`, `utf8mb4_unicode_ci` | Os mesmos do MariaDB, para que os dumps passem de um para o outro |

O MySQL então usa mais ou menos o tamanho do buffer pool dele mais 150 MB, ou mais 300 MB com o performance schema ativado. A página **Software** mostra essa estimativa; a página **Serviços** mostra o que ele usa de fato.

Os usuários entram com `caching_sha2_password`, o padrão do MySQL 8.4, que o `mysqli` e o PDO do PHP aceitam. O `mysql_native_password` continua desativado, como o MySQL 8.4 o traz: ele faz o hash com SHA-1 sem salt, e o MySQL 9 o removeu. Por isso um hash de senha antigo de outro servidor, como os que o cPanel guarda, não pode ser reaproveitado no MySQL: um banco de dados trazido de lá precisa da sua senha, ou de uma nova.

No MySQL, o painel roda SQL com o comando `mysql` em vez de `mariadb`, exporta com o `mysqldump`, e o serviço se chama `mysql`.

::: warning
Não edite o `90-panel.cnf`. O painel o reescreve.
:::

## Posso trocar entre o MariaDB e o MySQL? {#switch-between-mariadb-and-mysql}

Sim, a partir da v0.24.0. Um servidor roda um único servidor de banco de dados, mas você pode substituir o MariaDB pelo MySQL 8.4 LTS, ou o MySQL pelo MariaDB, com os seus bancos de dados, os usuários deles e os privilégios deles.

1. Vá até **Software**. No cartão **Servidor de banco de dados**, em **MySQL 8.4 LTS no lugar** (ou **MariaDB no lugar**), selecione **Trocar para MySQL 8.4 LTS** (ou **Trocar para MariaDB**).
2. Leia a janela. Ela lista os bancos de dados e as contas que vão junto, e como cada conta vai entrar depois.
3. Digite `mysql` (ou `mariadb`) para confirmar e selecione o botão de troca.

Voltar para o MariaDB instala o MariaDB da sua distribuição. Sem nenhum banco de dados, a mesma tarefa só substitui um servidor pelo outro.

### O que a troca faz {#what-the-switch-does}

1. O painel baixa os pacotes do novo servidor e guarda os do antigo para a volta. Nada muda ainda para os seus sites.
2. O servidor antigo fica somente leitura: os sites ainda conseguem ler os bancos de dados, mas não gravar. O painel lê as contas e os privilégios dele, conta as linhas de todas as tabelas e exporta cada banco de dados para uma nova pasta de `/var/backups/panel/db-server/`.
3. O servidor antigo para. O diretório de dados dele é movido para `/var/lib/mysql.mariadb-<date>-<time>` (ou `.mysql-…`), e o `/etc/mysql` é copiado para a pasta dos dumps. Os pacotes dele são removidos. A partir daqui, os sites não conseguem usar os bancos de dados até a troca terminar.
4. O novo servidor é instalado como a página **Software** o instala, e fica somente leitura. O acesso remoto e o log de consultas lentas são configurados de novo como estavam.
5. As contas são criadas, cada banco de dados é importado, e os privilégios são concedidos de novo. Os dumps do MySQL 8 são adaptados para o MariaDB como a importação de sites os adapta: as collations `utf8mb4_0900_*` viram as do MariaDB (`utf8mb4_unicode_520_ci`, a collation do próprio idioma ou `utf8mb4_bin`) e as configurações só do MySQL ficam de fora. Os dumps do MariaDB 11 que vão para o MySQL recebem as collations do MySQL no lugar das `uca1400` do MariaDB (`utf8mb4_uca1400_ai_ci` vira `utf8mb4_0900_ai_ci`).
6. O painel confere se cada banco de dados tem as mesmas tabelas com o mesmo número de linhas, views, rotinas, triggers e eventos, e se cada usuário de banco de dados do painel entra com a sua senha. Então o servidor volta a aceitar gravações e o diretório de dados antigo é excluído.

Se qualquer passo a partir do 3 falhar, o painel remove o novo servidor, restaura a configuração e o diretório de dados do antigo, instala de novo os pacotes dele e o inicia: você volta para onde estava. Os dumps ficam em qualquer dos casos.

O tempo que leva depende do tamanho dos seus bancos de dados: a exportação, a importação e a contagem de linhas leem, cada uma, todos os dados uma vez.

### Contas e senhas {#accounts-and-passwords}

O MariaDB e o MySQL guardam as senhas de formas diferentes, por isso nem toda senha pode ir junto:

| Conta | Depois da troca |
|---|---|
| O usuário de um banco de dados da página **Bancos de dados** | A senha dele, que o painel conhece, em `localhost` e em cada endereço permitido para o acesso remoto |
| Uma conta que você mesmo criou com uma senha `mysql_native_password`, indo para o MariaDB | A senha dela, a partir do hash |
| Uma conta que entra pelo socket (`unix_socket` no MariaDB, `auth_socket` no MySQL) | Entra de novo pelo socket |
| Qualquer outra conta que você mesmo criou, como uma conta do MariaDB indo para o MySQL (o MySQL 8.4 mantém o `mysql_native_password` desativado) ou uma conta do MySQL indo para o MariaDB (o MariaDB não consegue ler `caching_sha2_password`) | Criada com uma nova senha aleatória e bloqueada. O log lista cada uma. Defina uma senha como root no servidor: `ALTER USER 'name'@'localhost' IDENTIFIED BY '…' ACCOUNT UNLOCK;` |

Os privilégios delas são concedidos de novo a partir do `SHOW GRANTS` do servidor antigo. Um privilégio que o outro servidor não tem fica de fora e é listado no log, assim como as concessões de roles e de proxy, que o painel não leva.

### O que continua funcionando {#what-keeps-working}

- Os sites mantêm as suas configurações: eles se conectam a `localhost`, pelo mesmo socket nos dois servidores.
- O phpMyAdmin e o Adminer abrem o novo servidor.
- Os backups: os novos backups usam o programa de exportação do novo servidor, e um backup feito antes da troca pode ser restaurado depois dela.
- O acesso remoto continua aberto para os mesmos endereços.

### Quando o painel recusa {#when-the-panel-refuses}

A janela lista o que precisa ser corrigido antes, em **Corrija isto antes:**, e o botão de troca fica desativado:

| Motivo | O que fazer |
|---|---|
| O painel não conhece a senha do usuário de um banco de dados (um banco de dados trazido só com o hash da senha) | Defina uma nova senha para esse banco de dados na página **Bancos de dados** e coloque-a nas configurações do site |
| Indo para o MySQL: tabelas que usam um dos mecanismos de armazenamento próprios do MariaDB (Aria e outros), sequences ou tabelas com versionamento de sistema | Converta-as, por exemplo com `ALTER TABLE … ENGINE=InnoDB`, ou apague-as |
| Indo para o MySQL: colunas `INET4`, `INET6` ou `UUID` | Mude-as, por exemplo para `VARCHAR` |
| Indo para o MySQL: views, triggers ou rotinas cujo nome de usuário do definer tem mais de 32 caracteres | Crie-as de novo com outro definer |
| Um nome de banco de dados que começa com hífen | Renomeie o banco de dados |
| Não há espaço livre suficiente em disco: os dumps e os dados importados ao lado do diretório de dados antigo precisam de mais ou menos o dobro do tamanho dos seus bancos de dados mais 700 MB | Libere espaço; exclua dumps antigos |

A troca também é recusada enquanto o servidor estiver parado, quando a configuração do servidor tira o diretório de dados de `/var/lib/mysql` e, para o MySQL, onde o MySQL não é oferecido (arm64, ou um sistema para o qual a Oracle não tem pacotes).

Algumas diferenças só aparecem quando os dados são importados, como um índice que o MariaDB marca como `IGNORED`, que o MySQL não conhece. A importação então falha e o painel volta para o servidor antigo, com o erro no log.

## Dumps das atualizações de versão e das trocas {#dumps-from-upgrades-and-switches}

O cartão **Servidor de banco de dados** da página **Software** lista as pastas em **Dumps guardados**: quando, o quê (como "MariaDB 10.11.14 → MariaDB 11.4"), se foi **Concluído**, **Revertido** ou **Falhou**, quantos bancos de dados, o tamanho deles e a pasta. Cada banco de dados é um arquivo, `001.sql.gz`, `002.sql.gz` e assim por diante, com um `manifest.json` que diz o banco de dados de cada arquivo, porque os nomes dos bancos de dados podem ter caracteres que um nome de arquivo não pode ter. Só o root pode ler a pasta.

Para trazer de volta um banco de dados, descompacte o arquivo dele e importe-o na página **Bancos de dados**. Para excluir uma pasta, selecione **Excluir estes dumps** (a lixeira ao lado dela). O painel recusa enquanto uma atualização de versão ou uma troca estiver rodando.

## Criar um banco de dados {#create-a-database}

1. Vá até **Bancos de dados** e selecione **Adicionar banco de dados**.
2. Informe um nome em **Banco de dados**. Um usuário com o mesmo nome é criado, e esse usuário só pode usar este banco de dados.
3. Deixe a **Senha** vazia para receber uma senha aleatória forte, ou digite a sua.
4. Opcional: escolha um site em **Site**. Um backup desse site passa a incluir este banco de dados.
5. Selecione **Criar banco de dados**.

Um cartão com o título **Dados de conexão de** seguido do nome do banco de dados mostra então **Banco de dados**, **Usuário**, **Senha** e **Host**. O host é sempre `localhost`.

As regras para nomes e senhas:

- Os nomes começam com uma letra e usam só `a-z`, `0-9` e `_`, com no máximo 32 caracteres. Estes nomes são reservados: `mysql`, `information_schema`, `performance_schema`, `sys`, `test`, `root`, `mariadb`, `phpmyadmin`, `debian_sys_maint`. O painel também recusa um nome que já é usado por um banco de dados ou por um usuário de banco de dados no MariaDB.
- As senhas têm de 12 a 64 caracteres, feitos de letras, dígitos e `! # % * + , . : = ? @ ^ _ ~ -`. Uma senha gerada tem 24 caracteres.

Os novos bancos de dados usam `utf8mb4` e `utf8mb4_unicode_ci`.

### Usuários e privilégios {#users-and-privileges}

Cada banco de dados tem um usuário, `name@localhost`, com todos os privilégios nesse banco de dados e nenhum nos outros. Com o acesso remoto ativado, esse usuário também recebe uma cópia para cada endereço permitido (veja [Permitir o acesso remoto](#allow-remote-access)). Fora o usuário temporário que roda as importações (veja [Importar](#import)), o painel não cria usuários extras nem altera privilégios. Se você precisar disso, use o comando `mariadb` (`mysql` no MySQL) no servidor, como root.

A lista mostra os bancos de dados que o painel criou. Um banco de dados que foi removido fora do painel fica marcado como **Não encontrado no MariaDB** (ou **Não encontrado no MySQL**).

## Mostrar o login de um banco de dados {#show-a-database-s-login}

Selecione **Mostrar login** na linha do banco de dados. O cartão com os dados de conexão mostra o nome, o usuário, a senha e o host, cada um com um botão de copiar. Se o Adminer ou o phpMyAdmin estiver instalado, o cartão também tem **Abrir no Adminer** ou **Abrir no phpMyAdmin**.

## Trocar a senha de um banco de dados {#change-a-database-password}

1. Abra o menu do banco de dados e selecione **Nova senha**.
2. Mantenha **Gerar uma senha aleatória forte**, ou escolha **Usar a minha própria senha** e digite-a. A sua própria senha segue as mesmas regras de quando você [cria um banco de dados](#create-a-database).
3. Selecione **Nova senha**.

O painel define a nova senha e a mostra no cartão com os dados de conexão. Se o banco de dados estiver ligado a um site WordPress que o painel criou, o painel também grava a nova senha no `wp-config.php` desse site. As outras aplicações deixam de se conectar até você atualizar as configurações delas.

Se o banco de dados permitir o acesso remoto, os usuários remotos também recebem a nova senha.

## Exportar e importar {#export-and-import}

### Exportar {#export}

Abra o menu do banco de dados e selecione **Exportar (.sql)**. O seu navegador baixa um arquivo SQL simples com um nome como `shop_20261126-143000.sql`.

O dump é feito com `mariadb-dump --single-transaction` (`mysqldump` no MySQL, também com `--set-gtid-purged=OFF`), por isso o site continua rodando durante a exportação. Ele inclui as rotinas armazenadas e os triggers.

### Importar {#import}

1. Abra o menu do banco de dados e selecione **Importar**.
2. Escolha um arquivo em **Arquivo SQL (.sql ou .sql.gz)**. Um arquivo `.sql.gz` é descompactado na hora.
3. Opcional: marque **Esvaziar o banco de dados antes (excluir todas as tabelas dele)**. O banco de dados é então apagado e criado de novo antes da importação, e o usuário dele mantém o acesso.
4. Selecione **Importar**.

Uma importação pode ter até 4 GB.

Um dump feito pelo outro servidor, como um backup de antes de uma [troca](#switch-between-mariadb-and-mysql), também pode ser importado: o painel reescreve os nomes das collations que este servidor não conhece enquanto o arquivo é lido (as `utf8mb4_0900_*` do MySQL 8 no MariaDB, as `uca1400` do MariaDB 11 no MySQL). No MySQL, ele também tira o `NO_AUTO_CREATE_USER`, que o MariaDB grava no SQL mode de cada trigger e rotina e que o MySQL 8 recusa. Nada mais no arquivo muda; um dump que usa outro banco de dados continua sendo recusado. Restaurar um backup de banco de dados funciona do mesmo jeito.

A importação roda como um usuário temporário do MariaDB, `nefo-import-` seguido do nome do banco de dados, com todos os privilégios nesse banco de dados e nenhum nos outros. Ele também pode definir o definer de triggers, views e rotinas armazenadas, por isso os dumps que os contêm, como as próprias exportações e os backups do painel, são importados como antes. O painel exclui esse usuário quando a importação termina, também quando ela falha. Só quando o arquivo criou triggers, views, rotinas ou eventos sem uma cláusula `DEFINER`, que então rodam como esse usuário, o painel o mantém: bloqueado, para que ninguém consiga entrar com ele, e com privilégios só nesse banco de dados. Excluir o banco de dados também o exclui.

No MySQL, os nomes de usuário param em 32 caracteres, por isso, para um nome de banco de dados longo, o nome do usuário é encurtado e um hash curto é acrescentado. Os definers são permitidos ali com `SET_ANY_DEFINER` e `ALLOW_NONEXISTENT_DEFINER`, no lugar do `SET USER` do MariaDB; sem `SYSTEM_USER`, o usuário continua sem conseguir fazer um objeto rodar como root. A primeira linha dos dumps das versões mais novas do MariaDB, `/*M!999999\- enable the sandbox mode */`, que o cliente do MySQL recusa, é pulada no MySQL.

::: warning
Se o arquivo usar outro banco de dados, por exemplo com `USE other_db`, `CREATE DATABASE other_db` ou `INSERT INTO other_db.posts`, o MariaDB recusa essa instrução e a importação para com uma mensagem que começa com "O arquivo SQL usa outro banco de dados, other_db." O que o arquivo fez antes dessa linha fica no banco de dados. Remova do arquivo as linhas `CREATE DATABASE` e `USE`, ou exporte o banco de dados de novo sem `--databases`, e depois importe-o de novo com **Esvaziar o banco de dados antes (excluir todas as tabelas dele)** marcado. As linhas `CREATE DATABASE` e `USE` que citam este mesmo banco de dados não atrapalham.
:::

## Permitir o acesso remoto {#allow-remote-access}

Por padrão, o MariaDB só aceita conexões do próprio servidor. Você pode deixar endereços escolhidos se conectarem a um banco de dados, por exemplo uma ferramenta de relatórios na máquina do seu escritório.

Antes de começar, instale e ative o firewall ufw na página **Firewall e SSH**. O painel recusa o acesso remoto enquanto o ufw não estiver instalado.

1. Abra o menu do banco de dados e selecione **Acesso remoto**.
2. Em **Endereços permitidos**, informe um endereço. Selecione **Adicionar um endereço** para mais.
3. Selecione **Salvar**.

Conecte-se com o usuário e a senha do banco de dados ao endereço IP público do servidor, porta 3306.

### Endereços permitidos {#allowed-addresses}

- Endereços IPv4 ou IPv6 únicos, como `203.0.113.10` ou `2001:db8::1`.
- Faixas IPv4 na forma CIDR, como `198.51.100.0/24`, não mais largas que `/16`.
- Faixas IPv6 não são possíveis, porque o MariaDB não consegue compará-las. Informe endereços IPv6 únicos.
- No máximo 10 endereços por banco de dados.
- Os endereços de loopback, `0.0.0.0` e multicast são recusados.

### O que o painel altera {#what-the-panel-changes}

| Onde | Alteração |
|---|---|
| Usuários do MariaDB | Para cada endereço, um usuário `name@address` com a mesma senha e os mesmos privilégios do usuário local |
| Firewall (ufw) | Para cada endereço, uma regra que libera a porta 3306/tcp só a partir desse endereço, com o comentário `MariaDB name` |
| Configurações do MariaDB | `/etc/mysql/mariadb.conf.d/99-panel-remote.cnf` faz o MariaDB escutar em todas as interfaces (`0.0.0.0`, ou `::` quando um endereço IPv6 é permitido). O arquivo só existe enquanto pelo menos um banco de dados permitir o acesso remoto |

O MySQL funciona do mesmo jeito: o arquivo é `/etc/mysql/mysql.conf.d/99-panel-remote.cnf`, o comentário das regras é `MySQL name`, e um usuário remoto é apagado e criado de novo quando muda, já que o MySQL não tem `CREATE OR REPLACE USER`. As mensagens abaixo então dizem MySQL.

O MariaDB reinicia na primeira vez que o acesso remoto é ativado, e de novo quando ele é desativado para o último banco de dados. Os sites perdem o banco de dados por alguns segundos. Se outra tarefa estiver rodando nesse momento, a janela diz "O MariaDB precisa reiniciar antes de aceitar conexões remotas." Selecione **Reiniciar MariaDB** quando a tarefa terminar.

Se o MariaDB não iniciar com o arquivo de acesso remoto, o painel remove o arquivo e inicia o MariaDB sem ele, para que os seus sites continuem funcionando.

::: warning
- Se o firewall estiver desativado, a porta 3306 fica aberta para todo mundo. O MariaDB continua aceitando só os endereços listados.
- As conexões não são criptografadas, a menos que o cliente peça TLS. Para dados sensíveis, use um túnel SSH ou uma VPN.
- Liste só endereços que você controla e que não mudam.
:::

Para desativar o acesso remoto, abra **Acesso remoto** de novo e selecione **Desativar**. Excluir um banco de dados também remove os usuários remotos e as regras de firewall dele.

## Encontrar consultas lentas {#find-slow-queries}

O MariaDB pode registrar cada consulta que leva mais do que um limite. Isso ajuda você a encontrar o que deixa um site lento.

1. Vá até **Bancos de dados** → **Consultas lentas**. A aba aparece assim que o MariaDB é instalado.
2. Defina **Mais lentas que** como um número de segundos, de 0,1 a 60.
3. Ative a chave **Registrar consultas lentas** e selecione **Aplicar**.

O MariaDB precisa estar rodando para você alterar essas configurações. A alteração vale na hora, para as conexões abertas depois dela, e continua depois de uma reinicialização: o painel a guarda em `/etc/mysql/mariadb.conf.d/99-panel-slowlog.cnf`, e o log é gravado em `/var/log/mysql/mariadb-slow.log`. No MySQL, as configurações ficam em `/etc/mysql/mysql.conf.d/99-panel-slowlog.cnf`, e o log é `/var/log/mysql/mysql-slow.log`.

O painel faz a rotação do log para que ele não encha o disco, no MariaDB e no MySQL. Uma vez por hora ele olha o log e, quando ele passa de 50 MB, ou a entrada mais antiga dele tem 7 dias, o painel o renomeia, faz o servidor abrir um novo (`FLUSH SLOW LOGS`, sem reinicialização) e compacta o antigo ao lado, como `mariadb-slow.log-YYYYMMDD-HHMMSS.gz` (`mysql-slow.log-…` no MySQL). Ele mantém os 4 mais recentes deles e exclui os mais antigos. A aba mostra quantos ele mantém agora. A tabela só lê o log atual.

A tabela mostra as 200 entradas mais recentes, das mais novas para as mais antigas, lidas dos últimos 4 MB do log:

| Coluna | Significado |
|---|---|
| **Hora** | Quando a consulta rodou |
| **Usuário e host** | Quem a rodou, como `shop@localhost` |
| **Banco de dados** | O banco de dados que ela usou |
| **Duração** | Quanto tempo ela levou, em segundos |
| **Linhas examinadas** | Quantas linhas o MariaDB leu |
| **Consulta** | A consulta. Selecione-a para vê-la inteira, até 2.000 caracteres. As consultas mais longas ficam marcadas como **Encurtada** |

Selecione **Atualizar** para ler o log de novo, e **Limpar log** para esvaziá-lo. **Limpar log** só esvazia o log atual; os logs antigos compactados ficam até a rotação excluí-los.

::: tip
O registro dá um pouco mais de trabalho ao servidor. Desative a chave e selecione **Aplicar** quando terminar.
:::

## Adminer {#adminer}

O Adminer deixa você navegar pelos seus bancos de dados e editá-los no navegador, como o phpMyAdmin.

### Instalar o Adminer {#install-adminer}

1. Instale antes um servidor web, na página **Software**. O Adminer roda nele com o PHP.
2. Vá até **Software** → **Adminer (interface de banco de dados)** e selecione **Instalar**.

O painel baixa o Adminer 6.3.1 (o build com todos os drivers do Adminer) e o confere com um checksum SHA-256 fixado antes de instalá-lo. Ele abre bancos de dados MariaDB ou MySQL, e bancos de dados PostgreSQL assim que o PostgreSQL estiver instalado (veja [PostgreSQL no Adminer](./postgresql#postgresql-in-adminer)). O driver SQLite dele recusa qualquer login, porque o SQLite não tem senha.

### Abrir o Adminer {#open-adminer}

1. Selecione **Abrir no Adminer** no topo da página **Bancos de dados**, ou no cartão com os dados de conexão de um banco de dados. **Software** → **Abrir Adminer** faz o mesmo.
2. O Adminer abre em uma nova aba, no endereço do seu painel seguido de `adminer/`, por exemplo `https://203.0.113.10:28443/k3x9p2qa7m1z/adminer/`.
3. Entre no Adminer com o usuário e a senha do banco de dados, de **Mostrar login**.

### Como o Adminer é protegido {#how-adminer-is-protected}

O Adminer só é acessível pelo login do painel:

- Ele roda em um site interno do seu servidor web que escuta só em 127.0.0.1, em uma porta aleatória entre 40000 e 59999.
- Esse site interno só responde a requisições que trazem um token secreto. As outras requisições recebem `404`.
- O painel só acrescenta o token às requisições de uma sessão do painel com login. Sem uma sessão do painel, `adminer/` responde `401`, e a requisição nunca chega ao Adminer.
- O cookie de sessão do próprio painel nunca é repassado ao Adminer.

Por isso o Adminer não pode ser acessado pela internet, e outros programas do servidor não conseguem usá-lo sem o token.

O Adminer pode importar arquivos de até 256 MB, e uma requisição pode rodar por até 600 segundos. Para dumps maiores, use **Importar** na página **Bancos de dados**.

Desinstalar o Adminer remove os arquivos dele e o site interno dele. Os seus bancos de dados não são tocados. Desinstalar o servidor web também remove o Adminer.

## phpMyAdmin {#phpmyadmin}

O phpMyAdmin é a outra interface de banco de dados, para o MariaDB e o MySQL. Instale a que você preferir, ou as duas: elas não compartilham arquivos.

### Instalar o phpMyAdmin {#install-phpmyadmin}

1. Instale antes um servidor web, na página **Software**. O phpMyAdmin roda nele com o PHP.
2. Vá até **Software** → **phpMyAdmin (interface de banco de dados)** e selecione **Instalar**.

O painel baixa o phpMyAdmin 5.2.3 (todos os idiomas) de `files.phpmyadmin.net` e o confere com um checksum SHA-256 fixado antes de descompactá-lo em `/usr/share/nefo-phpmyadmin`. Ele deixa de fora o script de configuração, os exemplos e a documentação local (os links de ajuda abrem o docs.phpmyadmin.net). O phpMyAdmin guarda os seus arquivos temporários em `/var/lib/nefo-phpmyadmin/tmp`.

O painel grava o `config.inc.php` do phpMyAdmin, com uma chave aleatória de criptografia dos cookies:

- Você entra com o usuário e a senha de um banco de dados, só no banco de dados deste servidor. Você não consegue escolher outro servidor, entrar sem senha nem entrar como `root`.
- O phpMyAdmin não procura novas versões nem envia relatórios de erro.
- O formulário de troca de senha dele fica oculto. Troque a senha de um banco de dados na página **Bancos de dados**, que também atualiza o `wp-config.php` de um site WordPress.

### Abrir o phpMyAdmin {#open-phpmyadmin}

1. Selecione **Abrir no phpMyAdmin** no topo da página **Bancos de dados**, no menu de um banco de dados ou no cartão com os dados de conexão. **Software** → **Abrir phpMyAdmin** faz o mesmo.
2. O phpMyAdmin abre em uma nova aba, no endereço do seu painel seguido de `phpmyadmin/`, por exemplo `https://203.0.113.10:28443/k3x9p2qa7m1z/phpmyadmin/`.
3. Entre com o usuário e a senha do banco de dados, de **Mostrar login**.

O phpMyAdmin é protegido como o Adminer (veja [Como o Adminer é protegido](#how-adminer-is-protected)), no seu próprio site interno, com o seu próprio token secreto. Sem uma sessão do painel, `phpmyadmin/` responde `401`, e uma chave de API também não consegue chegar a ele. Só os três pontos de entrada do phpMyAdmin rodam como PHP, e só os scripts, os estilos e as imagens dele são servidos como arquivos; qualquer outra coisa responde `404`, inclusive o script de configuração. O phpMyAdmin envia a sua própria Content-Security-Policy e confere os seus próprios tokens de formulário.

Como o Adminer, o phpMyAdmin pode importar arquivos de até 256 MB, e uma requisição pode rodar por até 600 segundos. Desinstalar o phpMyAdmin remove os arquivos, o site interno e as configurações dele. Os seus bancos de dados não são tocados. Desinstalar o servidor web também remove o phpMyAdmin.

## Explorador de chaves do Redis {#redis-key-browser}

Quando o Redis está instalado, a página **Bancos de dados** tem uma aba **Redis**. Para instalar o Redis, vá até **Software** → **Redis** → **Instalar**. Veja [Software](./software#redis) para saber como ele é configurado.

A aba mostra a versão do Redis, há quanto tempo ele está rodando, a **Memória usada** (com o limite de memória) e o número de **Chaves**. Se ela disser **O Redis está instalado, mas não está rodando.**, inicie o Redis na página **Serviços**.

### Encontrar chaves {#find-keys}

1. Escolha um banco de dados de 0 a 15. A lista mostra quantas chaves cada um tem.
2. Informe um padrão. `*` corresponde a qualquer coisa e `?` a um caractere, por exemplo `user:*`.
3. Selecione **Pesquisar**.

A pesquisa percorre as chaves em etapas de cerca de 100. Se houver mais, selecione **Continuar pesquisando**.

### Ver uma chave {#view-a-key}

Selecione uma chave para ver o **Tipo**, o **Expira em** e o **Tamanho** dela, e o valor dela:

| Tipo | O que o painel mostra |
|---|---|
| string | Os primeiros 64 KB do valor |
| list, hash, set, sorted set | Os primeiros 200 itens. Cada item é cortado em 4 KB |
| stream e outros tipos | Só o tipo e a expiração |

Os caracteres que não podem ser mostrados em valores binários são substituídos.

### Excluir chaves {#delete-keys}

- Para excluir uma chave, selecione-a e depois **Excluir chave**.
- Para excluir todas as chaves do banco de dados escolhido, selecione **Esvaziar banco de dados 0** (o botão mostra o número do banco de dados), digite o número para confirmar e selecione **Excluir todas as chaves**.

Os sites que usam o Redis como cache o reconstroem. Os dados guardados só no Redis se perdem.

O explorador não edita valores nem tempos de expiração. Use o `redis-cli` no servidor para isso.

Se o Redis tiver uma senha (`requirepass` em `/etc/redis/redis.conf`), o painel a lê desse arquivo e entra com ela.

## Excluir um banco de dados {#delete-a-database}

1. Abra o menu do banco de dados e selecione **Excluir**.
2. Digite o nome do banco de dados para confirmar e selecione **Excluir**.

::: danger
Isto exclui o banco de dados, todas as tabelas dele e o usuário dele. Não dá para desfazer. Os usuários remotos, as regras de firewall e os agendamentos de backup automático dele vão junto. Os backups já feitos ficam guardados na página **Backups**.
:::

Quando você exclui um site, os bancos de dados dele são mantidos, a menos que você marque **Excluir também os bancos de dados dele**: eles ficam aqui, sem ligação com nenhum site. Restaurar o backup do site os liga de novo.

## Desinstalar o MariaDB {#uninstall-mariadb}

Desinstale o MariaDB na página **Software**. O painel recusa enquanto o MariaDB tiver qualquer banco de dados que não sejam os próprios bancos de dados de sistema dele, inclusive os bancos de dados criados fora do painel. Veja [Software](./software) para o que uma desinstalação mantém.

Um MariaDB do repositório do MariaDB vai embora junto com o arquivo do repositório e a chave.

O MySQL é desinstalado do mesmo jeito, no cartão dele, com a mesma verificação. O painel também remove o repositório da Oracle e a chave dele. O diretório de dados `/var/lib/mysql` fica, a menos que você marque a caixa e digite `mysql`.

Para substituir um pelo outro mantendo os seus bancos de dados, [troque](#switch-between-mariadb-and-mysql) em vez disso.

## PostgreSQL {#postgresql}

O PostgreSQL roda ao lado do MariaDB ou do MySQL, na sua própria aba **PostgreSQL** desta página: a versão da sua distribuição ou as versões 15 a 19 do projeto PostgreSQL, com bancos de dados e proprietários, importação e exportação, acesso remoto por TLS, backups e atualizações de versão principal. Veja [PostgreSQL](./postgresql).

## Memcached {#memcached}

Quando o Memcached está instalado (**Software** → **Memcached**), a página **Bancos de dados** tem uma aba **Memcached**. Ela mostra a versão, há quanto tempo ele está rodando, a **Memória usada** com o limite, o número de **Itens**, a taxa de acertos e as conexões abertas. Ela também mostra o que o próprio Memcached informa: que ele escuta só em 127.0.0.1 e que o UDP está desativado.

**Reiniciar** esvazia o cache. Veja [Software](./software#memcached) para saber como ele é configurado.
