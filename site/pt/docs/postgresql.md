---
title: PostgreSQL
description: Instale o PostgreSQL da sua distribuição ou as versões 15 a 19 do projeto PostgreSQL, crie bancos de dados e proprietários, permita o acesso remoto por TLS, importe e exporte, faça backup e atualize para uma versão principal mais nova.
---

# PostgreSQL

O Nefo roda o PostgreSQL ao lado do MariaDB ou do MySQL: a versão da sua distribuição, ou uma versão principal de 15 a 19 do próprio repositório do projeto PostgreSQL (apt.postgresql.org). A aba **Bancos de dados** → **PostgreSQL** cria bancos de dados com os seus proprietários, importa e exporta esses bancos, abre um banco de dados para endereços escolhidos por TLS e atualiza o servidor para uma versão principal mais nova, com um dump antes.

O PostgreSQL é o servidor de banco de dados que muitas aplicações Laravel, Django, Node.js e Odoo preferem. O MariaDB (ou o MySQL) e o PostgreSQL podem rodar lado a lado. Para o MariaDB e o MySQL, veja [Bancos de dados](./databases).

## Instalar o PostgreSQL {#install-postgresql}

1. Vá até **Software** → **PostgreSQL** e selecione **Instalar**.
2. Em **Versão**, escolha qual PostgreSQL instalar:
   - a versão da sua distribuição, como "PostgreSQL 16, a versão da distribuição", marcada como **Recomendado**. Ela é atualizada com o sistema;
   - uma versão do apt.postgresql.org, como "PostgreSQL 18 do apt.postgresql.org", e as outras versões principais que o repositório do projeto PostgreSQL oferece. Escolha uma quando uma aplicação precisar de uma versão mais nova (ou mais antiga) do que a que o seu sistema traz.
3. Selecione **Instalar** e espere um ou dois minutos. O log de progresso mostra cada etapa.

| Sistema | A versão da distribuição | Do apt.postgresql.org |
|---|---|---|
| Ubuntu 22.04 | 14 | 15, 16, 17, 18, 19 |
| Ubuntu 24.04 | 16 | 15, 17, 18, 19 |
| Ubuntu 26.04 | 18 | 15, 16, 17, 19 |
| Debian 12 | 15 | 16, 17, 18, 19 |
| Debian 13 | 17 | 15, 16, 18, 19 |

O painel oferece as versões principais que o projeto PostgreSQL ainda mantém. Em outro sistema, ou em um processador para o qual o repositório não tem pacotes, só a versão da distribuição é oferecida, e a janela diz por quê.

Quando você escolhe uma versão do apt.postgresql.org, o painel:

- baixa a chave de assinatura do repositório de `https://www.postgresql.org/media/keys/ACCC4CF8.asc` e a recusa, a menos que a impressão digital dela seja `B97B 0AFC AA1A 47F0 44F2 44A0 7FCC 7D46 ACCC 4CF8` (a chave "PostgreSQL Debian Repository");
- grava-a em `/etc/apt/keyrings/panel-pgdg.gpg` e acrescenta `/etc/apt/sources.list.d/panel-pgdg.list`, que confia nessa chave só para esse repositório;
- instala o pacote do servidor pelo nome, como `postgresql-18`. Ele nunca instala o pacote `postgresql` desse repositório, porque ele sempre puxa a versão principal mais nova, e uma atualização do sistema acrescentaria então um segundo servidor.

As atualizações de segurança de qualquer uma das versões chegam com as atualizações do sistema (página **Sistema**, ou `apt upgrade`).

O cartão **Software** passa a mostrar **Da distribuição** ou **Do apt.postgresql.org**, e o topo de **Bancos de dados** → **PostgreSQL** também.

Só uma versão do PostgreSQL roda por vez. O painel recusa:

- outra versão enquanto uma estiver instalada. [Atualize a versão](#upgrade-postgresql) dela, ou desinstale-a antes;
- uma versão diferente daquela cujos dados ainda estão no servidor, de uma instalação anterior (em `/var/lib/postgresql/<version>`). Instale essa versão de novo para usar os dados, e depois atualize a versão se quiser, ou exclua os dados antes.

O apt roda com o locale C, por isso o cluster que o pacote cria guarda o texto como `SQL_ASCII`. O Nefo então cria de novo esse cluster novo, ainda vazio, com UTF-8 (`C.UTF-8`). Um cluster que já existia antes da instalação nunca é recriado.

## Como o PostgreSQL é ajustado {#how-postgresql-is-tuned}

O Nefo grava as suas configurações em `/etc/postgresql/<version>/main/conf.d/90-nefo.conf`. As configurações de memória dependem da RAM do servidor:

| RAM do servidor | `shared_buffers` | `effective_cache_size` | `work_mem` | `maintenance_work_mem` | `max_connections` |
|---|---|---|---|---|---|
| Menos de 768 MB | 64MB | 256MB | 2MB | 16MB | 30 |
| 768 MB ou mais | 128MB | 512MB | 4MB | 32MB | 40 |
| 1,5 GB ou mais | 256MB | 1024MB | 4MB | 64MB | 50 |
| 3 GB ou mais | 512MB | 2048MB | 8MB | 128MB | 75 |
| 7 GB ou mais | 1024MB | 4096MB | 16MB | 256MB | 100 |

O `effective_cache_size` é cerca de metade da RAM, porque o mesmo servidor também roda o servidor web, o PHP e talvez o MariaDB. Em todo servidor, o Nefo também define `listen_addresses = 'localhost'`, `password_encryption = 'scram-sha-256'`, `random_page_cost = 1.1` (para armazenamento SSD) e `jit = off`. Em uma VPS que é um contêiner (LXC), a RAM é o limite de memória do contêiner (veja [Uma VPS que é um contêiner](./first-steps#a-vps-that-is-a-container)).

Ocioso e vazio, o PostgreSQL precisa de mais ou menos 30 a 40 MB para os seus poucos processos em segundo plano. À medida que o cache dele enche, ele cresce até o `shared_buffers` mais cerca de 40 MB, e cada conexão aberta acrescenta alguns MB. A página **Software** mostra o `shared_buffers` mais 40 MB.

::: warning
Não edite o `90-nefo.conf` nem o `pg_hba.conf`. Instalar o PostgreSQL de novo os reescreve. As suas próprias linhas no `pg_hba.conf` são mantidas quando você muda o acesso remoto: o painel só mexe nas linhas dele (veja [Acesso remoto](#postgresql-remote-access)).
:::

## Quem pode se conectar {#who-can-connect}

O Nefo grava `/etc/postgresql/<version>/main/pg_hba.conf`:

```
local   all  postgres                peer
local   all  all                     scram-sha-256
host    all  all       127.0.0.1/32  scram-sha-256
host    all  all       ::1/128       scram-sha-256
```

- Só o usuário de sistema `postgres` entra sem senha, pelo socket local. O painel gerencia o servidor desse jeito.
- Todos os outros precisam de uma senha, pelo socket ou por TCP a partir deste servidor. Ao contrário do padrão da distribuição, os outros usuários Linux não recebem login sem senha ("peer"), por isso o usuário Linux de um site não consegue abrir um role do banco de dados de mesmo nome sem a senha dele.
- Outras máquinas não conseguem se conectar, a menos que você permita o endereço delas para um banco de dados (veja [Acesso remoto](#postgresql-remote-access)). O PostgreSQL só escuta em localhost enquanto nenhum banco de dados permitir o acesso remoto.

## Criar um banco de dados PostgreSQL {#create-a-postgresql-database}

1. Vá até **Bancos de dados** → **PostgreSQL** e selecione **Adicionar banco de dados**.
2. Informe um nome em **Banco de dados**. Um role com o mesmo nome é o proprietário do banco de dados.
3. Deixe a **Senha** vazia para receber uma senha aleatória forte, ou digite a sua.
4. Opcional: escolha um site em **Site**. Um backup desse site passa a incluir este banco de dados, e excluir o site com os bancos de dados dele também o exclui.
5. Selecione **Criar banco de dados**.

O cartão com os dados de conexão mostra **Banco de dados**, **Usuário**, **Senha**, **Host** (`localhost`) e **Porta** (`5432`). Para um app Laravel, por exemplo, defina `DB_CONNECTION=pgsql`, `DB_HOST=127.0.0.1`, `DB_PORT=5432` e o banco de dados, o usuário e a senha do cartão.

As regras:

- Os nomes e as senhas seguem as [regras do MariaDB](./databases#create-a-database). Os nomes próprios do PostgreSQL também são recusados: `postgres`, `template0`, `template1`, `public`, `all` e qualquer nome que comece com `pg_`.
- Um nome não pode ser usado pelos dois servidores: se um banco de dados ou usuário do MariaDB se chama `shop`, um banco de dados PostgreSQL não pode ter esse nome, e vice-versa. O painel também recusa um nome que o PostgreSQL já usa para um banco de dados ou um role.
- O role pode fazer login, não é superusuário e não pode criar bancos de dados nem roles. Nenhum outro role pode se conectar ao banco de dados, porque o painel revoga o `CONNECT` de todos os outros.
- Os novos bancos de dados usam UTF-8 com o locale `C.UTF-8`.

A senha nunca chega ao PostgreSQL em texto claro: o painel só envia o verificador SCRAM-SHA-256 dela, por isso ela também não pode aparecer no log do PostgreSQL.

**Mostrar login**, **Nova senha** e **Excluir** funcionam como no MariaDB. Excluir também fecha as conexões abertas do banco de dados e remove as linhas de acesso remoto e as regras de firewall dele. Um banco de dados removido fora do painel fica marcado como **Não encontrado no PostgreSQL**.

## Outros servidores podem se conectar a um banco de dados PostgreSQL? {#postgresql-remote-access}

Sim, a partir da v0.24.0, um banco de dados por vez. Por padrão, o PostgreSQL só aceita conexões do próprio servidor. Você pode deixar endereços escolhidos se conectarem a um banco de dados, com o próprio usuário (role) e a própria senha desse banco, por TLS.

Antes de começar, instale o firewall ufw na página **Firewall e SSH** e ative-o. O painel recusa o acesso remoto enquanto o ufw não estiver instalado.

1. Vá até **Bancos de dados** → **PostgreSQL**, abra o menu do banco de dados e selecione **Acesso remoto**.
2. Em **Endereços permitidos**, informe um endereço. Selecione **Adicionar um endereço** para mais.
3. Selecione **Salvar**.

A linha então mostra "Remoto: 1" (o número de endereços), e o topo da aba mostra "1 aberto a outros servidores".

Conecte-se ao endereço IP público do servidor com o usuário e a senha de **Mostrar login**, e com TLS, por exemplo:

```sh
psql "host=203.0.113.10 port=5432 dbname=shop user=shop sslmode=require"
```

### Endereços permitidos {#allowed-addresses}

- Endereços IPv4 ou IPv6 únicos, como `198.51.100.7` ou `2001:db8::1`.
- Faixas na forma CIDR: IPv4 não mais largas que `/16` (como `198.51.100.0/24`), IPv6 não mais largas que `/48` (como `2001:db8:5::/48`).
- No máximo 10 endereços por banco de dados.
- Os endereços de loopback, link-local (`fe80::`), `0.0.0.0`, `::` e multicast são recusados.

### O que o painel altera {#what-the-panel-changes}

| Onde | Alteração |
|---|---|
| `pg_hba.conf` (PostgreSQL 16 e mais novo) | Uma linha no final, `include_if_exists nefo-remote.conf`, com um comentário. Os endereços permitidos ficam em `/etc/postgresql/<version>/main/nefo-remote.conf` |
| `pg_hba.conf` (PostgreSQL 14 e 15) | Um bloco no final, entre `# BEGIN Nefo remote access` e `# END Nefo remote access` |
| Essas linhas | Uma por endereço, só para este banco de dados e o próprio role dele, com login por senha: `hostssl "shop" "shop" 198.51.100.7/32 scram-sha-256` |
| Configurações do PostgreSQL | `/etc/postgresql/<version>/main/conf.d/95-nefo-remote.conf` define `listen_addresses = '*'`. Ele só existe enquanto pelo menos um banco de dados permitir o acesso remoto |
| Firewall (ufw) | Para cada endereço, uma regra que libera a porta do cluster (5432 por padrão) por TCP só a partir daquele endereço, com o comentário `PostgreSQL name` |

O painel nunca altera as outras linhas do `pg_hba.conf`. Quando as suas próprias linhas vêm antes das do painel, elas são lidas primeiro: uma linha `reject` sua para o mesmo endereço prevalece. Antes de o PostgreSQL recarregar um `pg_hba.conf` alterado, o painel pede ao PostgreSQL que o confira (`pg_hba_file_rules`). Se o PostgreSQL encontrar um erro, os arquivos antigos são colocados de volta e nada muda.

Se o cluster usa outra porta que não a 5432, as regras de firewall usam essa porta, e os dados de conexão a mostram.

### Criptografia {#encryption}

A janela diz se as conexões de outras máquinas são criptografadas:

- **As conexões de outras máquinas são criptografadas**: as linhas são `hostssl`, por isso o PostgreSQL recusa as conexões desses endereços que não usam TLS. Os clientes precisam de `sslmode=require` (ou mais rígido). O Debian e o Ubuntu criam os clusters com o certificado autoassinado do sistema (`/etc/ssl/certs/ssl-cert-snakeoil.pem`). Se o cluster tiver o TLS desativado, o painel cria o seu próprio certificado autoassinado (`nefo-ssl.crt` e `nefo-ssl.key`, ao lado do `pg_hba.conf`) e ativa o TLS no `95-nefo-remote.conf`; o TLS é desativado de novo junto com o último acesso remoto. Os dois certificados são autoassinados, por isso um cliente não consegue verificar o servidor com eles: `sslmode=require` criptografa, mas `verify-ca` e `verify-full` falham, a menos que você instale um certificado seu.
- **As conexões de outras máquinas não são criptografadas**: o PostgreSQL não tem TLS, por isso as linhas são `host`. Use um túnel SSH ou uma VPN para qualquer coisa sensível.

### Reinicializações {#restarts}

Um novo `listen_addresses` exige uma reinicialização; os endereços permitidos e o TLS só exigem um reload. Por isso o PostgreSQL reinicia na primeira vez que o acesso remoto é ativado, e de novo quando ele é desativado para o último banco de dados. Os sites perdem o banco de dados por alguns segundos, e as transações abertas são canceladas. Acrescentar ou remover endereços depois não o reinicia.

Se outra tarefa estiver rodando nesse momento, a janela diz **O PostgreSQL precisa reiniciar antes de aceitar conexões de outras máquinas.** Selecione **Reiniciar** na janela quando a tarefa terminar.

Se o PostgreSQL não iniciar com o `95-nefo-remote.conf`, o painel remove esse arquivo e inicia o PostgreSQL sem ele, para que os seus sites continuem funcionando.

::: warning
- Se o firewall estiver desativado, a porta fica aberta para todo mundo. O PostgreSQL ainda só deixa entrar os endereços listados, e só com a senha.
- Liste só endereços que você controla e que não mudam.
:::

Para desativar o acesso remoto, abra **Acesso remoto** de novo e selecione **Desativar**. Excluir um banco de dados também remove as linhas e as regras de firewall dele; quando ele era o último banco de dados com acesso remoto, o PostgreSQL reinicia para voltar a escutar só em localhost.

## Exportar e importar um banco de dados PostgreSQL {#export-and-import-a-postgresql-database}

**Exportar (.sql.gz)** baixa um dump SQL simples compactado com gzip, feito com o `pg_dump` como o próprio role do banco de dados, com `--no-owner --no-privileges`, por isso ele pode ser importado em qualquer banco de dados.

**Importar** aceita um arquivo `.sql` ou `.sql.gz` de até 4 GB:

- O `psql` roda como o próprio role do banco de dados, nunca como `postgres`, com a senha dele em um arquivo temporário que só o root pode ler.
- O arquivo roda em uma única transação com `ON_ERROR_STOP`: quando uma instrução falha, nada é importado e o erro cita a linha.
- Os comandos próprios do psql, as linhas que começam com uma barra invertida (como `\connect`, `\!` ou `\copy`), são recusados: o psql roda como root, e esses comandos poderiam rodar programas ou gravar arquivos no servidor. O painel inicia o psql no modo restrito (`\restrict` com uma chave aleatória) e tira as linhas `\restrict` e `\unrestrict` que as versões mais novas do `pg_dump` gravam. Um psql sem modo restrito (mais antigo que 17.6, 16.10, 15.14 ou 14.19) recusa qualquer importação: instale antes as atualizações do sistema.
- As instruções que criam ou alteram roles, entregam objetos a outros roles ou precisam de um superusuário falham. Exporte esse dump de novo com `pg_dump --no-owner --no-privileges`.
- O `SET transaction_timeout`, que o `pg_dump` 17 e posteriores gravam, é pulado nos servidores mais antigos.
- Depois, o painel define de novo a senha do role, caso o arquivo a tenha alterado.

Com **Esvaziar o banco de dados antes (excluir tudo o que há nele)** marcado, o banco de dados é apagado e criado de novo antes da importação, com o mesmo role como proprietário.

## Atualizar o PostgreSQL para uma versão mais nova {#upgrade-postgresql}

Você pode passar o servidor para uma versão principal mais nova do apt.postgresql.org, por exemplo da 15 da distribuição para a 17, mantendo todos os bancos de dados, usuários, senhas e configurações de acesso remoto.

1. Vá até **Bancos de dados** → **PostgreSQL** e selecione **Atualizar versão**. O botão aparece quando uma versão mais nova é oferecida e o PostgreSQL está rodando.
2. Escolha a **Nova versão**.
3. Leia **O que acontece** e depois marque **Entendo que os sites e as aplicações perdem o banco de dados durante a atualização.**
4. Selecione **Atualizar para 17** (a versão que você escolheu) e acompanhe o log.

O que o painel faz, nesta ordem:

1. Confere se a nova versão é mais nova, se há um único cluster (`main`), se não há pacotes de extensões instalados para a versão antiga (como `postgresql-15-postgis-3`) e se o disco tem espaço para o dobro dos dados mais 256 MB.
2. Salva todos os bancos de dados com o `pg_dumpall` em um único arquivo compactado, `/var/backups/nefo-postgresql/postgresql-<old>-before-<new>-<date>.sql.gz`, que só o root pode ler. Nada mudou ainda: se o dump falhar, a atualização para ali.
3. Remove o pacote `postgresql` da distribuição, que só aponta para uma versão (do apt.postgresql.org, ele apontaria para a mais nova e instalaria um segundo servidor), depois de marcar `postgresql-<old>` como instalado manualmente. Depois acrescenta o repositório apt.postgresql.org, se ele ainda não estiver lá, e instala `postgresql-<new>`. O cluster vazio que esse pacote cria é apagado. Se o apt trouxer qualquer outro servidor PostgreSQL junto, a atualização para e volta atrás.
4. Roda `pg_upgradecluster -v <new> -m upgrade <old> main`. O PostgreSQL fica parado nesse meio-tempo: cerca de um minuto para um servidor pequeno, mais para bancos de dados grandes. O `pg_upgrade` copia os arquivos de dados, por isso o cluster antigo fica como estava.
5. Dá ao novo cluster o `pg_hba.conf` antigo, o ajuste do painel (`conf.d/90-nefo.conf`) e as configurações de acesso remoto (a partir da versão 16, os endereços permitidos vão para o `nefo-remote.conf`), o reinicia e confere se ele roda na mesma porta com os mesmos bancos de dados, cada um respondendo.
6. Só então apaga o cluster antigo e remove os pacotes dele (`postgresql-<old>` e `postgresql-client-<old>`).

Se qualquer passo de 3 a 5 falhar, o painel volta atrás: apaga o novo cluster, remove os pacotes da nova versão, coloca de volta as configurações do cluster antigo e o inicia de novo. O log termina com "PostgreSQL 15 runs again with its data as before" (o PostgreSQL 15 roda de novo com os seus dados como antes). Os seus bancos de dados ficam como estavam. O repositório apt.postgresql.org fica, depois que pacotes foram instalados a partir dele, porque o `postgresql-common` e o `libpq5` foram atualizados a partir dele e, caso contrário, não receberiam mais atualizações; o pacote `postgresql` da distribuição não é colocado de volta.

A atualização de versão é recusada, com o motivo na janela, quando:

| Motivo | O que fazer |
|---|---|
| Há pacotes de extensões instalados para a versão antiga | O pg_upgrade precisa das mesmas extensões compiladas para a nova versão, que o painel não instala. Atualize manualmente com o `pg_upgradecluster` depois de instalá-las para a nova versão, ou remova as extensões antes |
| Existe mais de um cluster | O painel só gerencia o `main`. Atualize manualmente |
| Não há espaço suficiente em disco | Libere a quantidade que a mensagem informa |

Um banco de dados que o `pg_upgrade` não consegue levar, por exemplo uma tabela com uma coluna `regproc`, faz o passo 4 falhar; o painel então volta atrás como descrito.

### Os dumps guardados {#the-dumps-kept}

O arquivo salvo fica em `/var/backups/nefo-postgresql` de qualquer jeito, e a aba **PostgreSQL** o lista em **Dumps guardados**: para qual atualização ele foi feito (como "PostgreSQL 15 → 17"), quando, o tamanho e o caminho dele. Exclua-o com **Excluir este dump** quando tiver certeza de que tudo funciona.

Para restaurar tudo a partir dele em um servidor recém-instalado, manualmente:

```sh
zcat postgresql-15-before-17-20261127-101500.sql.gz | runuser -u postgres -- psql -X -d postgres
```

## PostgreSQL no Adminer {#postgresql-in-adminer}

O Adminer precisa da extensão `pgsql` do PHP para o PostgreSQL. O painel a instala para a versão do PHP que roda o Adminer, e só junto com o PostgreSQL: quando você instala o PostgreSQL com o Adminer já instalado, ou o Adminer com o PostgreSQL já instalado. Um Adminer instalado por uma versão anterior do painel, o build só para MySQL, é substituído ao mesmo tempo.

**Abrir no Adminer**, na linha de um banco de dados, abre o login PostgreSQL do Adminer com o servidor `localhost`, o usuário e o banco de dados preenchidos. Digite a senha de **Mostrar login**.

## PostgreSQL nos backups {#postgresql-in-backups}

Os backups de sites incluem os bancos de dados PostgreSQL do site, e um backup de banco de dados pode conter um banco de dados PostgreSQL, quando você pedir ou com agendamento. O dump é a saída simples do `pg_dump`, feita como o role do banco de dados, guardada como `postgres/<name>.sql` no arquivo compactado, ao lado dos dumps do MariaDB.

Os backups, as exportações e as importações usam o `pg_dump` e o `psql` da versão instalada, da distribuição ou do apt.postgresql.org, e a porta do cluster, também quando ela não é a 5432.

A restauração esvazia cada banco de dados (ele é apagado e criado de novo, com o mesmo role como proprietário) e importa o dump como esse role. Um banco de dados excluído desde o backup é criado de novo, com a senha do role tirada do backup quando o backup a tem (a partir da v0.24.0), ou, caso contrário, com uma nova senha.

## Desinstalar o PostgreSQL {#uninstall-postgresql}

Desinstale o PostgreSQL na página **Software**. O painel recusa enquanto o PostgreSQL tiver qualquer banco de dados que não seja `postgres`, `template0` e `template1`, inclusive os bancos de dados criados fora do painel. Ele também recusa enquanto o PostgreSQL estiver parado, porque não consegue conferir nesse caso. Os dados em `/var/lib/postgresql` e as configurações em `/etc/postgresql` ficam, a menos que você marque a caixa e digite `postgresql`. Veja [Software](./software#postgresql).

Desinstalar também remove o repositório apt.postgresql.org e a chave dele, se o painel os acrescentou, e as regras de firewall que sobraram para os bancos de dados PostgreSQL. Quando a extensão `pgsql` do PHP (acrescentada para o Adminer) ainda precisa do `libpq5`, o painel volta o `libpq5` para a versão da distribuição, para que ele receba de novo as atualizações de segurança do seu sistema.

A loja de aplicativos também tem o PostgreSQL como um serviço Docker em 127.0.0.1, para os apps que querem o seu próprio servidor. Esse não aparece na página **Bancos de dados**: veja [Loja de aplicativos](./apps#install-a-service).
