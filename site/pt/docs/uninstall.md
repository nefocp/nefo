---
title: Desinstalação
description: Remova o Nefo de um servidor manualmente, e decida antes o que acontece com os sites e o software que ele configurou.
---

# Desinstalação

O Nefo não tem comando de desinstalação: você o remove manualmente, como root, em poucos passos. Desbloqueie os sites bloqueados com `nefo unlock-files`, pare e remova o serviço `nefo` e `/usr/local/bin/nefo`, feche a porta do painel e exclua `/etc/nefo` e `/var/lib/nefo`. Os seus sites, bancos de dados e software continuam rodando, a menos que você os remova antes.

Esta página mostra cada passo. Ela também explica o que fica no servidor, porque remover o painel não remove os sites e o software que você configurou com ele.

## O que fica quando você remove o Nefo {#what-stays-when-you-remove-nefo}

Os passos desta página removem o próprio painel: o programa, o serviço, a configuração e os dados dele. Tudo o que você configurou pelo painel fica e continua rodando:

- o servidor web, o PHP, o MariaDB, o Redis, o Docker e os outros componentes da página **Software**;
- os seus sites em `/var/www`, os usuários Linux deles e os seus bancos de dados;
- tarefas cron, programas em segundo plano, sites de app e contêineres Docker;
- os backups guardados no servidor, em `/var/backups/panel`.

Sem o painel, nada renova os certificados dos seus sites nem roda os seus backups agendados. Os certificados dos sites ficam guardados na pasta de dados do Nefo, `/var/lib/nefo/certs`. Se você excluir essa pasta enquanto os sites ainda usam HTTPS, o servidor web não consegue carregar os certificados deles.

Então decida antes:

- **Para remover tudo**, remova antes os seus sites e o software pelo painel (passo 2) e depois remova o Nefo.
- **Para manter os sites rodando** sem o painel, pule o passo 2 e mantenha `/var/lib/nefo/certs` quando excluir os dados (passo 7). Os certificados Let's Encrypt duram no máximo 90 dias, por isso configure outra forma de renová-los antes que expirem.

## 1. Guarde cópias do que você precisa {#_1-keep-copies-of-what-you-need}

1. Na página **Backups**, faça um backup novo de cada site e banco de dados que você quer manter, com **Fazer backup agora**.
2. Copie os backups para fora do servidor. Eles ficam em `/var/backups/panel`, e no seu armazenamento remoto, se você configurou um.

## 2. Remova os sites e o software pelo painel {#_2-remove-websites-and-software-in-the-panel}

Faça isso enquanto o painel ainda roda. O painel se recusa a desinstalar um componente enquanto algo ainda o usa, por isso siga esta ordem:

1. **Sites:** abra cada site e, na aba **Visão geral**, clique em **Excluir**. Marque **Excluir também os bancos de dados dele** se você não precisa deles. Desmarque **Fazer backup do site antes** se você já guardou um backup (passo 1). Isso exclui os arquivos do site, o usuário Linux dele e o certificado dele.
2. **Bancos de dados:** exclua os bancos de dados que sobraram.
3. **Docker:** exclua os contêineres e os projetos Compose que você criou.
4. **Tarefas cron** e **Programas em segundo plano:** exclua os que você criou.
5. **Software:** clique em **Desinstalar** em cada componente de que você não precisa mais, como Docker, MariaDB, Redis e o servidor web. No MariaDB e no Docker, a pasta de dados (`/var/lib/mysql` para o MariaDB, `/var/lib/docker` para o Docker) é mantida, a menos que você marque a caixa para excluí-la também.

A última versão do PHP fica, porque a configuração do servidor web precisa de uma. Se você não precisa dela, remova-a com o apt depois de remover o Nefo.

## 3. Anote a porta do painel {#_3-note-the-panel-s-port}

Você precisa da porta para fechá-la no firewall depois:

```sh
nefo info --port
```

## 4. Desbloqueie os arquivos dos sites {#unlock-the-websites-files}

Se você bloqueou os arquivos de alguns sites (**Sites** → (site) → **Bloqueio de arquivos**, veja [Bloqueio por país e de arquivos](./website-protection#lock-a-website-s-files)), os arquivos deles pertencem ao root enquanto estão bloqueados. Devolva-os aos sites antes de remover o painel, ou eles continuam sendo do root:

```sh
systemctl stop nefo
nefo unlock-files
```

O comando lista cada site que desbloqueou:

```
Unlocked the files of example.com
Unlocked the files of blog.example.com
```

Cada arquivo e pasta recebe de volta exatamente o proprietário, o grupo e as permissões que tinha quando foi bloqueado. Os arquivos que o root colocou em um site nesse meio-tempo continuam sendo do root. O comando também remove os grupos de bloqueio dos sites (`panel-lock-<uid>`). Ele lê os registros em `/var/lib/nefo/filelock`, por isso rode-o antes de excluir os dados do painel (passo 7). Rodá-lo de novo não faz nada: ele mostra `No website's files are locked.`

## 5. Pare e remova o serviço e o programa {#_5-stop-and-remove-the-service-and-the-program}

```sh
systemctl disable --now nefo
rm /etc/systemd/system/nefo.service
systemctl daemon-reload
rm /usr/local/bin/nefo
```

## 6. Feche a porta do painel {#_6-close-the-panel-s-port}

Se o ufw estiver ativado, exclua as regras da porta do painel. O instalador as acrescentou com o comentário `Nefo`.

1. Liste as regras com os números delas:

   ```sh
   ufw status numbered
   ```

2. Exclua a regra da porta do painel pelo número, por exemplo:

   ```sh
   ufw delete 3
   ```

3. O ufw costuma ter uma segunda regra para IPv6, marcada com `(v6)`. Os números mudam depois de cada exclusão, por isso liste as regras de novo e exclua essa também.

Se você usa o firewalld, com a porta 28443 como exemplo:

```sh
firewall-cmd --permanent --remove-port=28443/tcp
firewall-cmd --reload
```

Remova também a porta do firewall de nuvem do seu provedor.

## 7. Exclua a configuração e os dados {#_7-delete-the-configuration-and-the-data}

::: danger Não dá para desfazer
Isto exclui a conta do painel, as configurações dele, a lista de sites, os agendamentos de backup, o log de atividades, o certificado do próprio painel e os certificados dos seus sites. Confira se você guardou o que precisa (passo 1).
:::

Para excluir tudo:

```sh
rm -rf /etc/nefo /var/lib/nefo
```

Se os seus sites ficam (veja acima), mantenha os certificados deles e exclua o resto:

```sh
rm -rf /etc/nefo
find /var/lib/nefo -mindepth 1 -maxdepth 1 ! -name certs -exec rm -rf {} +
```

## 8. Opcional: remova o que o Nefo deixou em outros lugares {#_8-optional-remove-what-nefo-left-elsewhere}

Enquanto você o usava, o Nefo também criou arquivos fora das suas próprias pastas. Remova só o que você não precisa mais:

| Caminho | O que é |
|---|---|
| `/var/backups/panel` | Os backups guardados neste servidor |
| `/etc/cron.d/panel` e `/var/log/panel-cron` | As tarefas cron criadas no painel, e os logs delas. As tarefas continuam rodando até você excluir o arquivo. |
| `/var/lib/panel-trash` | Os arquivos excluídos no gerenciador de arquivos, guardados por 30 dias |
| `/var/lib/panel-quarantine` | Os arquivos postos de lado pelo verificador de malware |
| `/var/lib/panel-geoip` | O banco de dados de países do bloqueio por país |
| `/var/backups/nefo-postgresql` | Os dumps feitos antes de uma atualização de versão do PostgreSQL |
| `/var/lib/panel-acme` | A pasta que responde às verificações do Let's Encrypt. Remova-a só quando nenhum servidor web a usar mais. |
| `/etc/ssh/sshd_config.d/99-panel-sftp.conf` | As regras de SSH para as contas dos sites e o SFTP |
| `/etc/fail2ban/jail.d/panel.conf` e `/etc/fail2ban/filter.d/panel.conf` | As regras do fail2ban para o SSH e o painel, se você instalou o fail2ban pelo painel |
| `/etc/ssh/sshd_config.d/00-panel.conf` | As configurações de SSH que você salvou no painel |

::: warning Configurações de SSH
Se você mudou a porta do SSH ou desativou o login por senha no painel, essas configurações estão em `/etc/ssh/sshd_config.d/00-panel.conf`. Excluir o arquivo muda a forma como você faz login por SSH assim que o SSH reinicia. Mantenha o arquivo, a menos que você saiba que quer as configurações padrão de volta, e teste uma nova sessão SSH antes de fechar a atual.
:::
