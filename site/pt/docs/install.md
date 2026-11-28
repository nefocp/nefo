---
title: Instalação
description: Instale o Nefo em um servidor Ubuntu ou Debian com um único comando e depois abra o painel pela primeira vez.
---

# Instalação

Para instalar o Nefo, rode um único comando como root em um servidor recém-instalado com Ubuntu 22.04, 24.04 ou 26.04 ou Debian 12 ou 13: `curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | sudo bash`. O instalador confere a assinatura Ed25519 da versão, instala o painel como o serviço systemd `nefo` em poucos segundos e mostra o endereço do painel e uma senha de administrador de uso único.

Esta página mostra como instalar o Nefo, o que o instalador faz e como abrir o painel pela primeira vez. Confira antes os [requisitos](./requirements).

O Nefo é instalado no Ubuntu 22.04, 24.04 e 26.04 e no Debian 12 e 13, em x86-64 ou ARM64. O instalador foi testado em servidores recém-instalados com cada um desses sistemas, em x86-64. A versão ARM64 ainda não foi testada em hardware ARM real.

## Rodar o instalador {#run-the-installer}

1. Conecte-se ao seu servidor por SSH.
2. Rode:

   ```sh
   curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | sudo bash
   ```

   Se você entrou como root e o `sudo` não está instalado (comum no Debian), deixe-o de fora:

   ```sh
   curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | bash
   ```

3. Espere até o instalador mostrar o endereço, o nome de usuário e a senha do painel. Isso costuma levar poucos segundos.

::: tip Comandos nesta documentação
Todos os outros comandos desta documentação são rodados como root no servidor, por isso aparecem sem `sudo`.
:::

## O que o instalador mostra {#what-the-installer-prints}

O final da saída é parecido com isto:

```
==> Downloading nefo-linux-amd64 (latest)
==> Verifying the release signature
==> Verifying checksum
==> Creating the configuration and the admin account
==> Installing the systemd service
==> Opening port 28443/tcp in ufw

Nefo is ready.

  URL:      https://203.0.113.10:28443/k3x9p2qa7m1z/
  Username: admin
  Password: ••••••••••••••••••••

Save this password now: it is stored only as a hash and cannot be shown again.
After signing in, change it and turn on two-factor authentication.
The certificate is self-signed, so your browser will warn you the first time.

If your provider has a cloud firewall (security group), allow TCP port 28443 there too.
Manage the service with: systemctl status|restart nefo   Logs: journalctl -u nefo
```

A sua porta, o seu caminho e a sua senha são diferentes: eles são escolhidos ao acaso para cada servidor.

::: warning Salve a senha agora
A senha só é mostrada uma vez. O painel guarda apenas um hash dela. Se você a perder, defina uma nova com `nefo reset-password` (veja [Linha de comando](./cli#nefo-reset-password)).
:::

O endereço usa o primeiro endereço IP do servidor. Se o seu provedor dá ao servidor um endereço privado e um público separado, troque o endereço na URL pelo IP público.

## O que o instalador faz {#what-the-installer-does}

1. **Confere o servidor.** Ele para se não for rodado como root, se faltar o systemd, o `curl` ou o `sha256sum`, se o sistema operacional ou a CPU não tiverem suporte, ou se o Nefo já estiver instalado (`/etc/nefo/config.json` existe).
2. **Baixa** o programa para a sua CPU (`nefo-linux-amd64` ou `nefo-linux-arm64`) e a lista de checksums das versões do projeto no GitHub.
3. **Confere a assinatura** da lista de checksums com a chave Ed25519 do projeto, que vem embutida no instalador. Uma versão sem assinatura válida é recusada: "Do not install this file." (Não instale este arquivo.) Um instalador de um build de desenvolvimento não tem chave, e avisa que a assinatura não é conferida.
4. **Confere o checksum SHA-256** do download. Uma divergência interrompe a instalação.
5. **Instala** o programa como `/usr/local/bin/nefo` e cria `/etc/nefo` e `/var/lib/nefo`, que só o root pode ler.
6. **Cria a configuração** em `/etc/nefo/config.json`: uma porta aleatória entre 20000 e 49999, um caminho secreto aleatório de 12 caracteres e o HTTPS ativado. Ele também cria a conta `admin` com uma senha aleatória de 20 caracteres.
7. **Instala o serviço systemd** `nefo`, inicia o serviço e o habilita na inicialização do sistema.
8. **Abre a porta do painel** no ufw, se o ufw estiver ativo, ou no firewalld, se for ele que estiver rodando. Se nenhum dos dois estiver ativo, ele não altera nenhum firewall.

A menos que você passe [opções de configuração](#set-the-server-up-in-the-same-run), o instalador não instala mais nada: nenhum servidor web, PHP ou banco de dados. O painel oferece configurar isso no seu primeiro login (veja [Primeiros passos](./first-steps#set-up-the-server)).

## Abrir o painel pela primeira vez {#open-the-panel-for-the-first-time}

1. Abra no navegador a URL que o instalador mostrou, exatamente como aparece. Ela começa com `https://` e termina com o caminho secreto e uma barra.
2. O navegador avisa que a conexão não é particular ou que o certificado não é confiável. Isso é esperado: o painel criou o seu próprio certificado autoassinado na primeira inicialização. Escolha a opção de continuar para o site. Na maioria dos navegadores, ela fica atrás de um botão como "Advanced" (Avançado).
3. Faça login com o nome de usuário `admin` e a senha do instalador.

Depois siga os [primeiros passos](./first-steps): troque a senha, ative a autenticação de dois fatores e dê ao painel o seu próprio domínio e um certificado gratuito, para que o aviso do navegador desapareça.

### O caminho secreto {#the-secret-path}

O painel só responde sob o seu caminho secreto, como `/k3x9p2qa7m1z/`. Qualquer outro endereço na porta dele retorna `404`, inclusive o simples `https://203.0.113.10:28443/`. Os scanners que testam os endereços comuns de painéis não encontram nada.

Mantenha o caminho em segredo, como uma senha. Se você esquecê-lo, rode `nefo info` no servidor (veja [Linha de comando](./cli#nefo-info)).

Se você digitar `http://` em vez de `https://`, o painel responde com um erro em vez da página de login.

## Abrir a porta no firewall de nuvem {#open-the-port-in-your-cloud-firewall}

Se o seu provedor tem um firewall fora do servidor (um security group ou firewall de nuvem), libere nele a porta TCP do painel. O instalador não consegue fazer isso por você. Mais tarde, quando você hospedar sites, libere também as portas TCP 80 e 443. Veja [Requisitos](./requirements#firewalls).

## Variáveis de ambiente {#environment-variables}

Você pode mudar o que o instalador faz com estas variáveis:

| Variável | Padrão | O que faz |
|---|---|---|
| `NEFO_VERSION` | `latest` | Instala uma versão específica, como `v0.1.0`, em vez da mais recente. |
| `NEFO_REPO` | `nefocp/nefo` | Baixa de outro repositório do GitHub (`owner/name`), como um fork. |
| `NEFO_FORCE` | não definida | Defina como `1` para tentar um sistema operacional sem suporte. A verificação da CPU continua valendo. |
| `NEFO_SETUP` | não definida | Opções para o `nefo setup`, para configurar o servidor na mesma execução. Veja [abaixo](#set-the-server-up-in-the-same-run). |

Coloque a variável antes de `bash`:

```sh
curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | NEFO_VERSION=v0.1.0 bash
```

Com `sudo`, escreva `| sudo NEFO_VERSION=v0.1.0 bash`.

O `NEFO_REPO` só muda de onde o instalador baixa. As atualizações posteriores vêm do repositório embutido no programa, ou do `update_url` no arquivo de configuração (veja [Atualização](./update#where-updates-come-from)).

## Configurar o servidor na mesma execução {#set-the-server-up-in-the-same-run}

O instalador também pode configurar o servidor, como a página de configuração do painel: um servidor web, versões do PHP, um servidor de banco de dados e extras. Passe as opções do [`nefo setup`](./cli#nefo-setup) depois de `bash -s --`:

```sh
curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | sudo bash -s -- --web nginx --php 8.4 --db mariadb --redis
```

Ou coloque-as na variável `NEFO_SETUP`:

```sh
curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | sudo NEFO_SETUP="--web nginx --php 8.4 --db mariadb" bash
```

O que muda com as opções de configuração:

- O instalador confere as opções logo depois do download, antes de configurar qualquer coisa. Uma opção errada, como `--web caddy`, faz ele parar, e você pode rodá-lo de novo. As opções só podem conter letras, dígitos e `. , = _ -`.
- Quando o painel já iniciou, o instalador roda o `nefo setup` com as suas opções e `--yes`, e mostra o log. Isso leva alguns minutos. O painel faz as instalações como uma das suas tarefas, por isso elas continuam mesmo que a sua sessão SSH termine: nesse caso, acompanhe-as pelo painel.
- O endereço, o nome de usuário e a senha são mostrados no final, depois da configuração.
- O painel não abre mais a página de configuração depois que você faz login.

Se a configuração parar, por exemplo porque um download falhou, o painel continua instalado. O instalador ainda mostra o endereço e a senha, depois diz que a configuração parou e sai com erro. Corrija o problema e rode o `nefo setup` com as mesmas opções: as partes já instaladas são puladas.

## Instalar de novo {#install-again}

O instalador se recusa a rodar quando `/etc/nefo/config.json` existe, por isso não consegue sobrescrever um painel que funciona. Para começar do zero, remova o Nefo antes (veja [Desinstalação](./uninstall)) e depois rode o instalador de novo.
