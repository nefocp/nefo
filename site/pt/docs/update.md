---
title: Atualização
description: Atualize o Nefo pelo painel ou com nefo update, e saiba como as versões são verificadas e o que fazer se uma atualização falhar.
---

# Atualização

Para atualizar o Nefo, clique em **Procurar atualizações** em **Configurações** → **Painel**, ou rode `nefo update` como root. O painel baixa a versão mais recente, confere a assinatura Ed25519 e o checksum SHA-256 dela, substitui `/usr/local/bin/nefo` em um único passo e reinicia. Os sites continuam rodando. O Nefo nunca se atualiza sozinho.

Esta página mostra como atualizar o Nefo, como cada versão é verificada antes de ser instalada e o que fazer se uma atualização falhar. Até a versão 1.0, só a versão mais recente recebe correções de segurança, por isso mantenha o Nefo atualizado.

## Atualizar pelo painel {#update-from-the-panel}

1. Abra **Configurações** → **Painel**. A versão atual aparece ao lado do nome Nefo.
2. Clique em **Procurar atualizações**. Se você já tiver a versão mais recente, o painel diz "Esta é a versão mais recente."
3. Se houver uma versão mais nova, o painel diz "A versão v0.2.0 está disponível." Clique em **Instalar atualização** e confirme.

O progresso aparece na página. Quando a nova versão estiver no lugar, o painel reinicia, e a página se recarrega sozinha depois de cerca de 20 segundos. Os seus sites continuam rodando durante a reinicialização.

Você também pode atualizar pelo dashboard: clique em **Procurar atualizações** e depois em **Atualizar para v0.2.0**.

## Atualizar pela linha de comando {#update-from-the-command-line}

```sh
nefo update
```

```
Installed: v0.1.0
Latest:    v0.2.0
Release signature verified
Downloading nefo-linux-amd64 v0.2.0
Checked: Nefo v0.2.0 (commit 5e6f7a8)
Installed. The panel restarts now; reload the page in a few seconds.
```

Se você já tiver a versão mais recente, o comando mostra as duas versões e não muda nada. Para só verificar, rode `nefo update --check`.

## O que acontece durante uma atualização {#what-happens-during-an-update}

1. O painel lê o número da versão mais recente nas versões do projeto no GitHub.
2. Ele baixa a lista de checksums e a assinatura dela, e confere a assinatura com a chave do projeto, que vem embutida no painel.
3. Ele baixa o novo programa para a sua CPU e confere o checksum SHA-256 dele com a lista.
4. Ele roda o novo programa uma vez, para garantir que funciona neste servidor.
5. Ele substitui `/usr/local/bin/nefo` em um único passo, de modo que nunca existe um programa gravado pela metade.
6. Ele reinicia o serviço `nefo`. Se uma tarefa, como a instalação de um software, estiver rodando, o painel deixa que ela termine antes, por até 75 segundos.

Quando a nova versão inicia, ela atualiza o banco de dados do painel para o novo formato. Ela também põe em dia, de acordo com os seus modelos, os arquivos de configuração que gerencia (servidor web, pools do PHP-FPM). Se o servidor web rejeitar os novos arquivos, o painel coloca os antigos de volta.

## Como as versões são verificadas {#how-releases-are-verified}

Cada versão no GitHub contém os programas para amd64 e arm64, o `install.sh`, um arquivo `VERSION`, o `checksums.txt` com os checksums SHA-256 dos dois programas e do instalador, e o `checksums.txt.sig`, uma assinatura Ed25519 do `checksums.txt`.

Tanto o instalador quanto o atualizador recusam um `checksums.txt` que não esteja assinado com a chave do projeto, e um download cujo checksum não confere. Durante uma atualização, os erros são:

- "checksums.txt has no valid signature from the project: the update was not installed" (o checksums.txt não tem uma assinatura válida do projeto: a atualização não foi instalada);
- "the download does not match checksums.txt: not installed" (o download não confere com o checksums.txt: não instalado).

Assim, alguém que conseguisse alterar os arquivos da versão no GitHub, ou no caminho até o seu servidor, não consegue fazer o seu próprio programa ser instalado por uma atualização: a atualização é conferida com a chave embutida no Nefo que você já roda. Uma primeira instalação é diferente: o instalador que você baixa traz a sua própria cópia da chave. E a chave de assinatura é guardada como um segredo do repositório no GitHub, onde o GitHub Actions assina cada versão, então alguém que tomasse o repositório poderia assinar uma versão. Veja [O que a assinatura prova e o que não prova](../security-policy#what-the-signature-does-and-doesn-t-prove).

Um build de desenvolvimento, feito sem a chave do projeto, não consegue conferir assinaturas. Ele confere só o checksum, e o log dele diz "Development build: the release signature is not checked" (build de desenvolvimento: a assinatura da versão não é conferida).

## O que é mantido {#what-is-kept}

Uma atualização substitui só o programa, `/usr/local/bin/nefo`. Todo o resto fica:

- o arquivo de configuração `/etc/nefo/config.json`, inclusive a porta e o caminho secreto;
- a pasta de dados `/var/lib/nefo`: a sua conta e a autenticação de dois fatores, as configurações, os sites, os agendamentos, os certificados e o log de atividades;
- os seus sites, bancos de dados, arquivos e backups, e os serviços que os rodam.

Só o painel reinicia. Os sites, os bancos de dados e os outros serviços continuam rodando.

## Se uma atualização falhar {#if-an-update-fails}

**A atualização para antes de o novo programa ser instalado.** Isso acontece quando o download falha, quando a assinatura ou o checksum não conferem, ou quando o novo programa não roda neste servidor. Nada mudou: a versão antiga continua rodando. O erro aparece no log da tarefa no painel, ou no terminal. Tente de novo mais tarde, e relate o erro se ele continuar.

**A nova versão não inicia.** Confira o serviço e o log dele:

```sh
systemctl status nefo
journalctl -u nefo -n 50 --no-pager
```

Relate o problema no [repositório do GitHub](https://github.com/nefocp/nefo/issues) com essa saída. Para voltar à versão anterior, instale o programa dela manualmente. Este exemplo usa a `v0.1.0` em um servidor amd64:

```sh
cd /tmp
curl -fsSLO https://github.com/nefocp/nefo/releases/download/v0.1.0/nefo-linux-amd64
curl -fsSLO https://github.com/nefocp/nefo/releases/download/v0.1.0/checksums.txt
grep " nefo-linux-amd64$" checksums.txt | sha256sum -c -
install -m 0755 nefo-linux-amd64 /usr/local/bin/nefo
systemctl restart nefo
```

Só continue se o `sha256sum` mostrar `OK`. Em um servidor ARM, use `nefo-linux-arm64`.

::: warning Voltar para uma versão mais antiga
Se a versão mais nova já tiver alterado o banco de dados do painel, uma versão mais antiga se recusa a iniciar com "database schema version … is newer than this panel supports" (a versão do esquema do banco de dados … é mais nova do que este painel aceita). Nesse caso, só uma versão mais nova funciona com esses dados. Antes de uma atualização importante, você pode guardar uma cópia da pasta de dados: `systemctl stop nefo`, depois `cp -a /var/lib/nefo /root/nefo-data-copy` e depois `systemctl start nefo`.
:::

## Atualizações automáticas {#automatic-updates}

O Nefo não se atualiza sozinho, e não procura atualizações por conta própria. Ele só procura quando você clica em **Procurar atualizações** ou roda `nefo update`. Para saber das novas versões, acompanhe as versões do [repositório do GitHub](https://github.com/nefocp/nefo/releases). O [registro de alterações](../changelog) lista o que mudou em cada uma.

Isso vale só para o próprio painel. Os pacotes do sistema operacional são outro assunto: o painel os procura a cada poucas horas, mostra-os em **Sistema** → **Atualizações** e pode instalar as atualizações de segurança automaticamente. Veja [Sistema](./os#updates).

## De onde vêm as atualizações {#where-updates-come-from}

Por padrão, as atualizações vêm de `https://github.com/nefocp/nefo/releases/latest/download`. O campo `update_url` em `/etc/nefo/config.json` pode apontar para outro endereço que publique os mesmos arquivos. Ele precisa começar com `https://`; caso contrário, a busca por atualizações falha com "O update_url em config.json precisa ser um endereço https://." Deixe esse campo sem valor, a menos que você saiba que precisa dele.
