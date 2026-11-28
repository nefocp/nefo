---
title: Linha de comando
description: Todos os comandos do nefo e as suas opções (info, reset-password, update, setup, unlock-files, decrypt-backup), o arquivo de configuração, e como gerenciar o serviço nefo com systemctl e journalctl.
---

# Linha de comando

O programa `nefo`, em `/usr/local/bin/nefo`, roda o painel e tem comandos para o usuário root do servidor: `nefo info` mostra o endereço do painel, `nefo reset-password` define uma nova senha (e pode desativar o 2FA), `nefo update` instala a versão mais recente e `nefo setup` instala o software. Rode todos os comandos como root.

Esta página lista os comandos e depois mostra como gerenciar o serviço `nefo` e o que há no arquivo de configuração.

## Comandos {#commands}

| Comando | O que faz |
|---|---|
| `nefo info` | Mostra a URL do painel |
| `nefo reset-password` | Define uma nova senha aleatória e encerra todas as sessões |
| `nefo update` | Instala a versão mais recente e reinicia o painel |
| `nefo setup` | Instala um servidor web, versões do PHP, um servidor de banco de dados e extras |
| `nefo unlock-files` | Devolve os arquivos aos sites bloqueados (antes de você remover o painel) |
| `nefo decrypt-backup` | Descriptografa uma cópia de backup que um armazenamento remoto criptografou |
| `nefo version` | Mostra a versão |
| `nefo init` | Cria o arquivo de configuração e a primeira conta de administrador |
| `nefo serve` | Roda o painel (o systemd faz isso por você) |
| `nefo help` | Mostra a lista de comandos |

Rode `nefo <command> -h` para ver as opções de um comando. As opções funcionam com um ou dois hifens: `-port` e `--port` são a mesma coisa.

Todos os comandos, exceto `version`, `help` e `decrypt-backup`, leem o arquivo de configuração `/etc/nefo/config.json`. Para usar outro arquivo, acrescente `--config /path/to/config.json`, ou defina a variável de ambiente `NEFO_CONFIG`.

## nefo info {#nefo-info}

Mostra a URL do painel e onde ficam os arquivos dele.

```sh
nefo info
```

```
URL:    https://panel.example.com:28443/k3x9p2qa7m1z/
URL:    https://203.0.113.10:28443/k3x9p2qa7m1z/
Config: /etc/nefo/config.json
Data:   /var/lib/nefo
```

A primeira linha aparece quando o painel tem o seu próprio domínio (**Configurações** → **Certificado**). A segunda usa o primeiro endereço IP do servidor. Se o seu provedor dá ao servidor um endereço privado e um público separado, passe o público:

```sh
nefo info --host 203.0.113.10
```

| Opção | O que faz |
|---|---|
| `--host` | O nome do host ou o endereço IP a mostrar na URL |
| `--port` | Mostra só o número da porta, para scripts |
| `--config` | O arquivo de configuração a ler |

## nefo reset-password {#nefo-reset-password}

Define uma nova senha aleatória para a conta, encerra a sessão dela em todos os dispositivos e mostra a senha. Use quando você esqueceu a senha.

```sh
nefo reset-password
```

```
New password for admin: ••••••••••••••••••••
```

Se você também perdeu o celular com o seu app autenticador, acrescente `--disable-2fa`. Isso desativa a autenticação de dois fatores da conta:

```sh
nefo reset-password --disable-2fa
```

```
New password for admin: ••••••••••••••••••••
Two-factor authentication is off. Turn it on again after signing in.
```

| Opção | O que faz |
|---|---|
| `--disable-2fa` | Desativa também a autenticação de dois fatores |
| `--user` | O nome da conta (padrão `admin`, a única conta do painel) |
| `--config` | O arquivo de configuração a ler |

O painel continua rodando: você não precisa reiniciá-lo. A redefinição é gravada no log de atividades.

## nefo update {#nefo-update}

Consulta as versões do projeto no GitHub. Se houver uma versão mais nova, ele a baixa, confere a assinatura e o checksum dela, substitui o programa e reinicia o painel.

```sh
nefo update
```

Para só ver se há uma atualização disponível:

```sh
nefo update --check
```

```
Installed: v0.1.0
Latest:    v0.2.0
```

| Opção | O que faz |
|---|---|
| `--check` | Só mostra a versão instalada e a mais recente |
| `--config` | O arquivo de configuração a ler |

Veja [Atualização](./update) para o que a atualização confere e o que fazer se ela falhar.

## nefo setup {#nefo-setup}

Configura o servidor como a página de configuração do painel (veja [Primeiros passos](./first-steps#set-up-the-server)): instala um servidor web, versões do PHP, um servidor de banco de dados e extras, um depois do outro. As partes que já estão instaladas são puladas, por isso você pode rodá-lo de novo depois de uma falha. Quando ele termina, o painel não abre mais a página de configuração depois do login.

```sh
nefo setup --web nginx --php 8.3,8.4 --db mariadb --redis
```

```
This server: 1 CPU, 960 MB of RAM (250 MB in use), 19.5 GB of free disk.

What the setup installs, one after the other:
  1. Nginx and PHP 8.3  about 40 MB of RAM, 55 MB of disk
  2. PHP 8.4            about 25 MB of RAM, 50 MB of disk, from packages.sury.org
  3. MariaDB            about 228 MB of RAM, 300 MB of disk
  4. Redis              up to 64 MB of RAM, 10 MB of disk
In all: about 357 MB more RAM and 415 MB of disk.

Install? [y/N]
```

Depois que você responde `y`, ele mostra o log de cada instalação. Se uma falhar, ele para ali, diz o que já está instalado e sai com erro. Sem `--web`, `--php`, `--db` ou um extra, ele oferece a escolha recomendada para a RAM do servidor, como a página de configuração.

Enquanto o painel roda, o `nefo setup` pede ao painel que faça as instalações, como uma das tarefas dele: elas esperam qualquer outra tarefa, aparecem no painel e continuam mesmo que você feche a sessão SSH. Ele fala com o painel por um socket na pasta de dados que só o root pode abrir. Quando o painel não está rodando, o próprio `nefo setup` faz as instalações.

| Opção | O que faz |
|---|---|
| `--web` | `nginx`, `apache`, `openlitespeed` ou `none`. O servidor web vem com a versão do PHP da sua distribuição |
| `--php` | Versões do PHP separadas por vírgulas, como `8.3,8.4`, ou `none` |
| `--db` | `mariadb`, `mysql` ou `none`; `mariadb-11.4`, `mariadb-11.8` ou `mariadb-12.3` para uma série LTS do repositório do MariaDB, onde o servidor a oferecer (veja [Versões do MariaDB](./databases#mariadb-versions)). `--mariadb` é o mesmo que `--db mariadb` |
| `--redis`, `--node`, `--python`, `--docker` | Instala também o Redis, o Node.js (a versão LTS mais nova), o Python para sites de app, ou o Docker |
| `--adminer`, `--waf` | Instala também o Adminer ou o firewall web. Os dois precisam de um servidor web |
| `--clamav` | Instala também o ClamAV, em servidores com 2 GB de RAM ou mais |
| `--postgresql`, `--memcached` | Instala também o PostgreSQL ou o Memcached, onde a página Software os oferecer |
| `--yes`, `-y` | Instala sem perguntar. Necessário quando não há terminal, como em um script |
| `--dry-run` | Só mostra o que seria instalado |
| `--skip` | Não instala nada: o painel não abre mais a página de configuração depois do login |
| `--wait` | Quanto tempo esperar o painel iniciar, como `60s`. O instalador usa essa opção |
| `--check` | Só confere as opções. O instalador usa essa opção antes de configurar qualquer coisa |
| `--config` | O arquivo de configuração a ler |

O instalador aceita as mesmas opções, para configurar o servidor na mesma execução (veja [Instalação](./install#set-the-server-up-in-the-same-run)).

## nefo unlock-files {#nefo-unlock-files}

Devolve a cada site cujos arquivos estão bloqueados (**Sites** → (site) → **Bloqueio de arquivos**, veja [Bloqueio por país e de arquivos](./website-protection#lock-a-website-s-files)) os seus arquivos, com exatamente o proprietário, o grupo e as permissões que cada um tinha antes do bloqueio, e remove os grupos de bloqueio dos sites. Rode-o como root, com o painel parado, antes de remover o Nefo (veja [Desinstalação](./uninstall#unlock-the-websites-files)).

```sh
systemctl stop nefo
nefo unlock-files
```

```
Unlocked the files of example.com
Unlocked the files of blog.example.com
```

Quando nenhum site está bloqueado, ele mostra `No website's files are locked.`

| Opção | O que faz |
|---|---|
| `--config` | O arquivo de configuração a ler |

## nefo decrypt-backup {#nefo-decrypt-backup}

Descriptografa uma cópia de backup que um armazenamento remoto criptografou (um arquivo `.tar.gz.enc`), por exemplo uma que você baixou do Google Drive. Só a frase secreta é necessária: funciona em qualquer servidor, inclusive em um onde o painel não está configurado, e não lê o arquivo de configuração. Veja [Restaurar em um novo servidor](./restore-new-server#decrypt-a-copy-by-hand).

```sh
nefo decrypt-backup example.com_20261126-030000.tar.gz.enc
```

```
Passphrase:
Decrypted to example.com_20261126-030000.tar.gz
SHA-256: 3f1c…
```

A frase secreta nunca é passada na linha de comando, onde outros usuários poderiam vê-la. Em um terminal, ela é pedida sem aparecer na tela; caso contrário, é a primeira linha da entrada padrão ou do `-passphrase-file`.

| Opção | O que faz |
|---|---|
| `-o` | O arquivo de saída. O padrão é o nome da entrada sem `.enc`. `-o -` grava na saída padrão, por exemplo `nefo decrypt-backup -o - FILE.enc \| tar xz` |
| `-passphrase-file` | Lê a frase secreta da primeira linha deste arquivo |
| `-force` | Substitui o arquivo de saída se ele existir |

A saída é gravada em um arquivo temporário e só é renomeada quando a cópia inteira foi descriptografada, por isso uma frase secreta errada ou um arquivo danificado não deixam nada para trás (exceto com `-o -`, em que o final do arquivo é conferido por último). Um arquivo que não está criptografado é copiado como está, com um aviso. O SHA-256 que ele mostra é o que o painel registrou para o backup.

## nefo version {#nefo-version}

Mostra a versão e o commit a partir do qual ela foi gerada. `nefo --version` e `nefo -v` fazem o mesmo.

```sh
nefo version
```

```
Nefo v0.1.0 (commit 1a2b3c4)
```

## nefo init {#nefo-init}

Cria o arquivo de configuração e a primeira conta de administrador. O instalador o roda por você, então normalmente você não precisa dele. Ele se recusa a sobrescrever um arquivo de configuração existente.

Ele escolhe uma porta aleatória entre 20000 e 49999 e um caminho secreto aleatório, ativa o HTTPS e cria a conta `admin` com uma senha aleatória. Se a pasta de dados já tiver contas, ele as mantém, grava só o novo arquivo de configuração e não mostra senha.

| Opção | O que faz |
|---|---|
| `--config` | Onde gravar o arquivo de configuração (padrão `/etc/nefo/config.json`) |
| `--data` | A pasta de dados (padrão `/var/lib/nefo`) |
| `--listen` | O endereço em que escutar, como `:28443`, em vez de uma porta aleatória |
| `--host` | O nome do host ou o endereço IP a mostrar na URL impressa |
| `--dev` | Modo de desenvolvimento: HTTP simples em `127.0.0.1:8080` com o caminho `dev`. Nunca use em um servidor. |

## nefo serve {#nefo-serve}

Roda o painel. O serviço `nefo` o inicia com `nefo serve --config /etc/nefo/config.json`, então você não o roda manualmente. Uma segunda cópia falharia, porque o serviço já usa a porta.

## Gerenciar o serviço nefo {#manage-the-nefo-service}

| Comando | O que faz |
|---|---|
| `systemctl status nefo` | Mostra se o painel está rodando, e as últimas linhas do log |
| `systemctl restart nefo` | Reinicia o painel |
| `systemctl stop nefo` | Para o painel |
| `systemctl start nefo` | Inicia o painel |
| `journalctl -u nefo` | Mostra o log do painel |
| `journalctl -u nefo -f` | Acompanha o log conforme chegam novas linhas |
| `journalctl -u nefo -n 100 --no-pager` | Mostra as últimas 100 linhas |
| `journalctl -u nefo --since "1 hour ago"` | Mostra o log da última hora |

Reiniciar o painel não para os seus sites, bancos de dados ou outros serviços. Você também pode reiniciá-lo em **Configurações** → **Painel** → **Energia** → **Reiniciar o painel**.

Quando o painel para, ele deixa uma tarefa em andamento, como a instalação de um software, terminar antes, por até 75 segundos. Se o painel parar por causa de um erro, o systemd o inicia de novo depois de 3 segundos.

O arquivo do serviço é `/etc/systemd/system/nefo.service`:

```ini
[Unit]
Description=Nefo server control panel
Documentation=https://nefo.io
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/nefo serve --config /etc/nefo/config.json
Restart=on-failure
RestartSec=3
UMask=0077
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

O painel roda como root, porque gerencia o servidor inteiro.

## O arquivo de configuração {#the-configuration-file}

O `/etc/nefo/config.json` só pode ser lido pelo root. O instalador o grava, por exemplo:

```json
{
  "listen": ":28443",
  "entrance": "k3x9p2qa7m1z",
  "data_dir": "/var/lib/nefo",
  "tls": {
    "enabled": true
  }
}
```

| Campo | O que significa |
|---|---|
| `listen` | O endereço e a porta em que o painel escuta. `:28443` significa a porta 28443 em todos os endereços. |
| `entrance` | O caminho secreto. De 8 a 32 letras minúsculas ou dígitos. Qualquer outro caminho responde `404`. |
| `data_dir` | A pasta de dados: o banco de dados do painel `nefo.db`, os certificados e as chaves. |
| `tls.enabled` | HTTPS para o painel. Mantenha ativado. |
| `tls.cert`, `tls.key` | Caminhos opcionais para o seu próprio certificado e a sua chave para o painel. Eles têm prioridade sobre o certificado Let's Encrypt do painel e o autoassinado. |
| `acme_directory` | Autoridade certificadora opcional para os certificados. Vazio significa Let's Encrypt. Defina `https://acme-staging-v02.api.letsencrypt.org/directory` durante os testes: os navegadores não confiam nos certificados de staging, mas as tentativas malsucedidas não contam para os limites do Let's Encrypt. |
| `update_url` | Endereço opcional de onde as atualizações são baixadas. Vazio significa as versões do projeto no GitHub. Precisa começar com `https://`. |
| `dev` | Modo de desenvolvimento. Nunca defina em um servidor. |

Depois de editar o arquivo, reinicie o painel com `systemctl restart nefo`. Se o arquivo tiver um erro, o painel não inicia: `journalctl -u nefo` mostra o motivo.

::: warning Trocar a porta
Se você mudar a porta em `listen`, abra a nova porta antes, tanto no ufw (por exemplo `ufw allow 31234/tcp`) quanto no firewall de nuvem do seu provedor. Caso contrário, você não consegue acessar o painel depois da reinicialização.
:::
