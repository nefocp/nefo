---
title: Requisitos
description: Os sistemas operacionais, os tipos de CPU, a memória, as portas e as configurações de firewall de que o Nefo precisa.
---

# Requisitos

O Nefo precisa de um servidor de 64 bits, x86-64 ou ARM64, com Ubuntu 22.04, 24.04 ou 26.04 ou Debian 12 ou 13, acesso root e systemd. Ele foi feito para uma VPS com 1 GB de RAM: o próprio painel usa cerca de 34 MB de memória, medidos em um servidor de teste depois da atualização para a v0.24.0.

Esta página lista o que o seu servidor precisa antes de você [instalar o Nefo](./install): o sistema operacional, o tipo de CPU, memória e disco, e as portas de rede.

## Sistema operacional {#operating-system}

O instalador aceita estes sistemas:

| Distribuição | Versões |
|---|---|
| Ubuntu | 22.04, 24.04, 26.04 |
| Debian | 12, 13 |

O instalador foi testado em servidores recém-instalados com cada um desses sistemas, em x86-64. A versão ARM64 ainda não foi testada em hardware ARM real.

Em qualquer outro sistema, o instalador para e diz que o sistema "is not supported yet" (ainda não tem suporte). Você pode definir `NEFO_FORCE=1` para tentar mesmo assim (veja [Instalação](./install#environment-variables)), mas o gerenciamento de sites precisa de `apt-get`, `systemctl` e `useradd`. Sem eles, a página **Sites** mostra "O gerenciamento de sites precisa de um servidor Debian ou Ubuntu com systemd, então não está disponível nesta máquina."

O instalador também precisa de:

- acesso root (entre como root ou use `sudo`);
- systemd;
- `curl` e `sha256sum`;
- `openssl`, para conferir a assinatura de uma versão.

Se o `curl` não estiver instalado, instale-o antes com `apt install curl`.

## CPU {#cpu}

O Nefo roda em x86 de 64 bits (`x86_64`, também chamado de amd64) e em ARM de 64 bits (`aarch64`, também chamado de arm64). Em qualquer outra CPU, o instalador para com "unsupported CPU architecture" (arquitetura de CPU sem suporte).

## Memória e disco {#memory-and-disk}

O Nefo foi feito para VPSs pequenas, com 1 GB de RAM. O próprio painel é um único programa de cerca de 25 MB e usa cerca de 34 MB de memória (medidos em um servidor de teste depois da atualização para a v0.24.0). Você instala só os componentes de que precisa. A página **Software** mostra mais ou menos quanta memória cada componente usa. Por exemplo, os servidores web usam cerca de 15 MB (Nginx), 26 MB (OpenLiteSpeed) ou 40 MB (Apache) em repouso.

Em uma VPS que é um contêiner (LXC, OpenVZ), o Nefo conta o limite de memória do próprio contêiner, não a memória do host (veja [Uma VPS que é um contêiner](./first-steps#a-vps-that-is-a-container)).

Um componente precisa de muito mais memória: o antivírus opcional ClamAV precisa de cerca de 1,4 GB de memória livre durante a verificação, por isso ele serve para servidores com 2 GB ou mais.

Em um servidor com 1 GB de memória, acrescente 1 a 2 GB de swap, para que um momento de pico não faça o kernel parar o MariaDB ou o PHP. Muitas imagens de VPS vêm sem swap. **Sistema** → **Swap** cria um arquivo de swap com um clique (veja [Swap](./os#swap)). Em um contêiner (OpenVZ, LXC), o kernel não permite swap: fale com o seu provedor.

O espaço em disco necessário depende dos seus sites, bancos de dados e backups. Os backups guardados no servidor ficam em `/var/backups/panel`, no mesmo disco dos seus sites. O dashboard mostra o uso do disco, e o painel avisa quando o disco está 90% cheio.

## Um servidor recém-instalado {#a-fresh-server}

Instale o Nefo em um servidor recém-instalado. O instalador não confere o que mais está instalado, mas o próprio painel configura o servidor web, o PHP, o MariaDB, o firewall e o SSH:

- Ele roda um único servidor web (Nginx, Apache ou OpenLiteSpeed). Ele se recusa a instalar um enquanto outro servidor web já estiver no servidor, porque os dois precisam das portas 80 e 443.
- Ele grava a sua própria configuração para o software que gerencia. Não instale o Nefo ao lado de outro painel de controle.

O instalador se recusa a rodar uma segunda vez no mesmo servidor: quando `/etc/nefo/config.json` existe, ele para com "the panel is already installed" (o painel já está instalado).

## Portas {#ports}

| Porta | Usada por | Quando |
|---|---|---|
| Uma porta TCP aleatória entre 20000 e 49999 | O painel | Desde a instalação. O instalador a mostra. |
| 80 e 443 (TCP) | O servidor web | Assim que você instala um servidor web pelo painel |
| 443 (UDP) | HTTP/3 no OpenLiteSpeed | Quando o OpenLiteSpeed é o servidor web |
| A sua porta SSH (22 por padrão) | SSH | Sempre |
| 3306 (TCP) | MariaDB ou MySQL | Só se você permitir o acesso remoto a um banco de dados, e só para os endereços que você listar |
| 5432 (TCP) | PostgreSQL | Só se você permitir o acesso remoto a um banco de dados PostgreSQL, e só para os endereços que você listar |

Alguns serviços escutam só no próprio servidor (127.0.0.1), por isso você não os abre: o receptor dos webhooks de push do Git (porta 7809, acessado pelo servidor web), as portas dos contêineres Docker e o relay de e-mail local.

## Firewalls {#firewalls}

### O firewall do servidor (ufw) {#the-firewall-on-the-server-ufw}

- Se o ufw estiver ativo na instalação, o instalador abre nele a porta do painel (ou no firewalld, se for ele que estiver rodando).
- Quando você instala um servidor web pelo painel e o ufw está ativo, o painel abre as portas 80 e 443. O painel não gerencia o firewalld: se o servidor usa o firewalld, abra essas portas você mesmo.
- A página **Firewall e SSH** gerencia o ufw. Se o ufw não estiver instalado, a página diz "O ufw não está instalado neste servidor." Instale-o com `apt install ufw`.

Quando você ativa o firewall pelo painel, ele mantém abertos o SSH e a porta do painel, e as portas 80 e 443 quando há um servidor web instalado. Veja [Segurança](./security).

### O firewall de nuvem do seu provedor {#your-provider-s-cloud-firewall}

Muitos provedores têm um firewall fora do servidor, muitas vezes chamado de security group ou firewall de nuvem. O instalador não consegue alterá-lo. Libere nele, você mesmo:

1. A sua porta SSH.
2. A porta TCP do painel, que o instalador mostra no final.
3. As portas TCP 80 e 443, assim que você hospedar sites. O Let's Encrypt confere os seus domínios pela porta 80, por isso não é possível emitir certificados enquanto ela estiver fechada.

## Um nome de domínio {#a-domain-name}

Você não precisa de um domínio para instalar o Nefo: você abre o painel pelo endereço IP do servidor. Você precisa de um para cada site, e pode dar ao painel o seu próprio domínio e um certificado gratuito (veja [Primeiros passos](./first-steps#give-the-panel-its-own-domain)). Aponte o registro A do domínio para o endereço IP do servidor.
