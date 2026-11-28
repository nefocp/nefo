---
title: Roadmap
description: No que se está trabalhando antes do Nefo 1.0, o que está em estudo, e o que não está planejado e por quê.
---

# Roadmap

O roadmap do Nefo não tem datas: um item sai quando funciona. Antes da versão 1.0, o trabalho vai para tornar confiável o que já existe: testes em hardware ARM64 real e com os serviços reais, um servidor de teste que roda por muito tempo, relatos de usuários de fora e uma revisão de segurança externa. Algumas ideias estão em estudo, e algumas coisas não estão planejadas, de propósito.

Esta página lista no que se está trabalhando, o que está em estudo e o que o Nefo não vai fazer. Para ver o que já saiu, leia o [registro de alterações](./changelog).

## Agora: rumo à 1.0 {#now-toward-1-0}

O Nefo está em beta. Antes da versão 1.0, o trabalho vai mais para tornar confiável o que já existe do que para recursos novos:

- **Testar o instalador em ARM64.** O instalador, com e sem a configuração do software, foi testado em servidores recém-instalados com Ubuntu 22.04, 24.04 e 26.04 e Debian 12 e 13, em x86-64. Os builds ARM64 são publicados, mas ainda não foram testados em hardware ARM real.
- **Testar com os serviços reais.** Backups em armazenamento compatível com S3 (como AWS S3 e Cloudflare R2) e no Google Drive, alertas por Telegram, Discord, Slack e webhook, e-mail por provedores SMTP e a API do Cloudflare. Alguns deles até agora só foram testados com serviços substitutos: o Google Drive, por exemplo, só com um serviço do Google simulado.
- **Rodar um servidor de teste por muito tempo,** com sites reais, para que os problemas que levam semanas para aparecer (logs que crescem, uso de disco, memória) sejam encontrados antes que você os encontre.
- **Usuários de fora e os seus relatos de bugs.** Cada relato com os passos para reproduzir ajuda. Veja [Relatar um bug](./about#get-help-and-report-bugs).
- **Uma revisão de segurança externa.** Ainda não foi feita nenhuma auditoria externa. Até que seja feita, trate o Nefo como um software beta que roda como root, e veja a [política de segurança](./security-policy).

Até a versão 1.0, só a versão mais recente recebe correções de segurança.

## Em estudo {#being-considered}

São ideias, não promessas. Algumas podem mudar, e algumas podem nunca sair.

- **Backups no Google Drive sem um cliente OAuth próprio.** Hoje cada servidor precisa de um cliente OAuth que você cria uma vez em um projeto do Google Cloud (veja [Google Drive](./docs/backups#google-drive)).
- **O endereço do próprio visitante atrás do proxy do Cloudflare,** nos logs de um site e no PHP com o Nginx, como uma configuração por site. Hoje um site Nginx atrás do proxy vê ali os endereços do Cloudflare; o bloqueio por país já consulta o endereço do visitante.
- **Uma interface do painel mais tranquila.**

O painel fala sete idiomas. Traduções para mais idiomas são bem-vindas: veja o [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md).

## O que não está planejado, e por quê {#not-planned-and-why}

O Nefo deve continuar leve o bastante para uma VPS com 1 GB de RAM, e pequeno o bastante para que o seu código possa ser verificado quanto à segurança. Cada item abaixo acrescentaria serviços rodando o tempo todo, portas abertas para a internet ou uma grande quantidade de código rodando como root. Por isso ficam de fora de propósito, e esta página diz o que o Nefo oferece no lugar.

| Não planejado | Por quê | O que o Nefo oferece no lugar |
|---|---|---|
| Um servidor de e-mail | O e-mail precisa de vários serviços rodando o tempo todo, filtros de spam e vírus que usam muita memória e um cuidado constante com a reputação do servidor. Muitos provedores também bloqueiam a porta 25 de saída. | Um relay SMTP pelo seu provedor de e-mail, para os sites e os alertas. Use um provedor de e-mail para as caixas de correio. Veja [E-mail e alertas](./docs/mail-alerts). |
| Um servidor DNS | Os servidores de nomes precisam responder à internet o tempo todo, e o ideal é rodá-los em mais de uma máquina. | Os registros DNS das suas zonas do Cloudflare, gerenciados pelo painel, e certificados wildcard pelo DNS do Cloudflare. Veja [DNS (Cloudflare)](./docs/dns). |
| Um servidor FTP | Mais um serviço, com as suas próprias portas abertas e as suas próprias contas. | Uma conta SFTP por site, presa à pasta do site, com senha ou chaves SSH. Veja [Acesso SFTP](./docs/websites#sftp-access). |
| Gerenciar vários servidores em um painel | Um painel que controla outros servidores precisa de agentes e de confiança entre as máquinas. Uma invasão em um deles chegaria a todos. | Um Nefo por servidor. As chaves de API permitem que os seus próprios scripts trabalhem com cada servidor. Veja [Chaves de API](./docs/api). |
| Contas de revenda e de clientes de hospedagem | Contas com os seus próprios limites, planos e cotas são outro produto, para vender hospedagem. O Nefo é para a pessoa que administra o servidor. | Uma conta de administrador. Para deixar um cliente enviar arquivos, dê ao site dele uma conta SFTP. |

Se você precisa de uma dessas coisas, outro painel pode atender melhor: veja [Nefo e outros painéis](./compare#when-another-panel-fits-you-better).
