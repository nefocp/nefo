---
title: O que é o Nefo?
description: O Nefo é um painel de controle gratuito e de código aberto que você instala no seu próprio servidor Linux para administrar sites, bancos de dados, apps, backups e segurança pelo navegador.
---

# O que é o Nefo?

O Nefo é um painel de controle gratuito e de código aberto (GNU AGPLv3) que você instala no seu próprio servidor Linux com um único comando. Pelo navegador, em um de sete idiomas, você gerencia sites e os seus certificados SSL, bancos de dados, apps, arquivos, backups e segurança. É um único programa, feito para uma VPS com 1 GB de RAM, e não envia telemetria.

Esta página explica o que o Nefo faz, para quem ele é, o que ele deixa de fora de propósito e o que ele envia pela rede.

## Um painel de controle em um único programa {#a-control-panel-in-one-program}

Você instala o Nefo em uma VPS com um único comando. É um único programa, `/usr/local/bin/nefo`, com a interface web embutida, que roda como o serviço systemd `nefo`. Depois, você gerencia o servidor pelo navegador: sites, certificados, bancos de dados, arquivos, backups, software e segurança.

- **Todos os recursos são gratuitos.** Não há plano pago nem taxa por site ou por conta.
- **Sem conta e sem servidor de licenças.** Nada precisa ser registrado ou ativado.
- **Feito para servidores pequenos.** O Nefo foi projetado para rodar em uma VPS com 1 GB de RAM, em x86-64 e ARM64. O próprio painel usa cerca de 34 MB de memória, medidos em um servidor de teste depois da atualização para a v0.24.0.
- **Sete idiomas na interface:** inglês, vietnamita, chinês, espanhol, português, russo e indonésio. Você escolhe o idioma na página de login ou no menu da conta, e o navegador só baixa o idioma que você usa.

Para começar, confira os [requisitos](./requirements), depois [instale o Nefo](./install) e siga os [primeiros passos](./first-steps).

## Para quem ele é {#who-it-is-for}

O Nefo é para quem administra a própria VPS: alguém montando um primeiro servidor para um blog, um desenvolvedor fazendo deploy de um app, ou um freelancer ou agência que cuida de sites de clientes.

Cada servidor tem uma única conta do painel, `admin`. O Nefo não tem logins separados para clientes ou revendedores.

## O que você pode fazer com ele {#what-you-can-do-with-it}

### Sites {#websites}

- Hospede sites PHP, estáticos e de app (Node.js, Python ou Java) no Nginx, no Apache ou no OpenLiteSpeed, e sites de proxy reverso que encaminham para outro endereço. Cada site tem o seu próprio usuário Linux e, se você quiser, uma conta SFTP presa à pasta dele. Clone um site para um novo domínio, e exclua um com um backup antes. Veja [Sites](./websites).
- Ative o HTTPS com certificados Let's Encrypt gratuitos que se renovam automaticamente, certificados wildcard pelo DNS do Cloudflare ou o seu próprio certificado. Veja [Certificados SSL](./ssl).
- Bloqueie ou permita visitantes por país, e deixe o código de um site somente leitura para o PHP dele. Veja [Bloqueio por país e de arquivos](./website-protection).
- Faça deploy a partir do GitHub, GitLab, Gitea ou qualquer servidor Git, quando você quiser ou a cada push. Veja [Deploy pelo Git](./git-deploy).
- Mantenha sites WordPress atualizados, com um backup do banco de dados antes de cada atualização, modo de manutenção e uma cópia de staging. Veja [Ferramentas do WordPress](./wordpress).
- Instale 42 aplicações com um clique: aplicações PHP como WordPress, Joomla, Nextcloud e Laravel, e aplicações Docker como Ghost, Uptime Kuma, n8n, Gitea, Grafana e Jellyfin. Veja [Loja de aplicativos](./apps).

### Dados {#data}

- Rode o MariaDB (o da sua distribuição, ou o 11.4, 11.8 ou 12.3 LTS) ou o MySQL 8.4, ajustados para servidores pequenos, com bancos de dados, usuários, importação e exportação de SQL e acesso remoto por banco de dados, e troque entre os dois levando os seus bancos de dados. O Adminer e o phpMyAdmin ficam atrás do login do painel. Explore as chaves do Redis e acrescente o Memcached. Veja [Bancos de dados](./databases).
- Rode o PostgreSQL, a versão da sua distribuição ou as versões 15 a 19 do projeto PostgreSQL, com os seus próprios bancos de dados e proprietários, acesso remoto por TLS e atualizações de versão principal. Veja [PostgreSQL](./postgresql).
- Importe sites de um backup completo do cPanel, ou de um arquivo compactado do site e dumps SQL feitos pelo aaPanel, HestiaCP, CyberPanel ou qualquer servidor. Veja [Importar sites](./import).
- Gerencie arquivos com um editor, upload e download, arquivos compactados e uma lixeira que guarda os arquivos excluídos por 30 dias. Veja [Arquivos](./files).
- Faça backup de sites, bancos de dados, pastas e apps com agendamento, neste servidor, em armazenamento compatível com S3, em um servidor SFTP ou no Google Drive, criptografados com uma frase secreta se você quiser, e restaure-os, inclusive em um novo servidor. Veja [Backups](./backups) e [Restaurar em um novo servidor](./restore-new-server).

### Servidor {#server}

- Instale e desinstale cada componente (servidor web, versões do PHP e o ionCube Loader, MariaDB ou MySQL, PostgreSQL, Redis, Node.js, Python, Java, Docker e mais). O Nefo se recusa a remover um componente que sites, bancos de dados ou contêineres ainda usam. Veja [Software](./software).
- Rode contêineres Docker e projetos Compose. O Nefo verifica os arquivos Compose em busca de configurações inseguras, como contêineres privilegiados ou portas abertas para a internet. Veja [Docker](./docker).
- Controle o firewall (ufw), o fail2ban, as configurações de SSH, um firewall de aplicação web por site e um verificador de malware. Veja [Segurança](./security).
- Rode a verificação de segurança: 22 verificações rápidas do servidor, cada uma com um link para a correção, quando você pedir ou todo dia. Veja [Verificação de segurança](./security-check).
- Use um terminal no navegador e gerencie serviços, processos, programas em segundo plano e tarefas cron. Veja [Ferramentas do sistema](./system).
- Acompanhe se cada site responde, com o uptime e os tempos de resposta. Veja [Monitoramento](./websites#monitoring).
- Deixe os sites enviarem e-mail por um relay SMTP, e receba alertas por Telegram, e-mail, Discord, Slack ou um webhook, inclusive quando alguém faz login. Veja [E-mail e alertas](./mail-alerts).
- Gerencie os registros DNS das suas zonas do Cloudflare, aponte o domínio de um site para este servidor e acrescente os registros MX, SPF e DMARC do seu provedor de e-mail. Veja [DNS (Cloudflare)](./dns).

## A segurança do próprio painel {#security-of-the-panel-itself}

O Nefo roda como root, por isso protege o seu próprio login:

- O painel só responde sob um caminho secreto, como `/k3x9p2qa7m1z/`. Qualquer outro endereço na porta dele retorna `404`.
- Ele usa HTTPS desde a primeira inicialização. No começo o certificado é autoassinado, e você pode substituí-lo por um certificado gratuito para o próprio domínio do painel.
- O login aceita autenticação de dois fatores. Depois de 5 tentativas malsucedidas em 15 minutos, um endereço fica bloqueado por 15 minutos.
- Os logins e as alterações são gravados em um log de atividades, que guarda as entradas por 90 dias.

## O que o Nefo não faz {#what-nefo-doesn-t-do}

- **Nenhum servidor de e-mail.** O Nefo não recebe e-mail nem hospeda caixas de correio. Os sites e os alertas enviam e-mail por um relay SMTP de um provedor de e-mail. Use um provedor de e-mail para as suas caixas de correio.
- **Nenhum servidor DNS ou FTP.** O Nefo gerencia os registros DNS dos seus domínios no Cloudflare, e cada site pode ter uma conta SFTP no lugar do FTP.
- **Um servidor por painel.** Cada servidor roda o seu próprio Nefo. Gerenciar vários servidores em um painel não está planejado (veja o [roadmap](../roadmap#not-planned-and-why)).
- **Nenhuma conta para clientes.** Há uma conta de administrador por servidor.
- **Nenhuma conta de e-mail, DNS ou FTP na importação.** Uma importação do cPanel traz os sites, os bancos de dados, os certificados e as tarefas cron. Veja [O que não é importado](./import#what-is-not-imported).
- **Só Ubuntu e Debian.** Ubuntu 22.04, 24.04 e 26.04, e Debian 12 e 13. Veja [Requisitos](./requirements).

## Estado {#status}

O Nefo é um software novo. Os números de versão começam com `0.`, e até a versão 1.0 só a versão mais recente recebe correções de segurança. Você vê a sua versão em **Configurações** → **Painel**, ou com `nefo version` no servidor.

O instalador foi testado em servidores recém-instalados com Ubuntu 22.04, 24.04 e 26.04 e Debian 12 e 13, em x86-64. A versão ARM64 ainda não foi testada em hardware ARM real. Ainda não foi feita nenhuma auditoria de segurança externa.

Experimente o Nefo em um servidor que você possa reconstruir antes de confiar nele para sites importantes, e guarde cópias dos seus backups fora do servidor. Relate problemas no [repositório do GitHub](https://github.com/nefocp/nefo).

O [registro de alterações](../changelog) lista o que mudou em cada versão, o [roadmap](../roadmap), no que se está trabalhando e o que não está planejado, e a [política de segurança](../security-policy), como relatar uma vulnerabilidade e verificar uma versão por conta própria.

## Licença {#licence}

O Nefo é software livre sob a GNU AGPLv3. Você pode usá-lo para qualquer fim, inclusive comercial. Se você oferecer um Nefo modificado a outras pessoas por uma rede, a AGPL exige que você publique as suas alterações.

## Privacidade {#privacy}

O Nefo não tem telemetria, estatísticas de uso nem verificação de licença. A interface web não carrega nada de outros sites: as fontes, os ícones e os scripts dela vêm embutidos no programa.

Por conta própria, o painel só se conecta a outros servidores para estas tarefas:

| Quando | Para onde | Por quê |
|---|---|---|
| Um certificado está para vencer (verificado a cada hora) | Let's Encrypt | Pedir ou renovar os certificados dos seus sites e do painel. A conta do painel no Let's Encrypt não tem endereço de e-mail. A menos que você use a validação por DNS, o painel primeiro busca um arquivo de teste no seu domínio por HTTP, para conferir que o domínio chega a este servidor. |
| Uma vez por dia, nos builds de release | As versões do projeto no GitHub | Baixar o catálogo assinado da loja de aplicativos. Um catálogo sem uma assinatura válida do projeto é ignorado. |

Todo o resto só acontece porque você pediu:

- **Atualizações:** o Nefo só procura uma nova versão quando você clica em **Procurar atualizações** ou roda `nefo update`. Ele nunca se atualiza sozinho. Veja [Atualização](./update).
- **Coisas que você configura:** alertas (Telegram, o seu servidor SMTP, Discord, Slack ou o seu webhook), monitoramento de sites (os seus sites, pelo domínio, como um visitante), backups remotos (o seu armazenamento S3 ou SFTP, ou o Google Drive), a validação por DNS e a página DNS (a API do Cloudflare), o relay de e-mail (o seu provedor de e-mail) e o bloqueio por país, que baixa o banco de dados de países da DB-IP e a lista de endereços do Cloudflare uma vez por mês enquanto um site o usa.
- **Software que você instala:** os pacotes vêm da sua distribuição pelo apt, e alguns componentes vêm dos seus próprios projetos, como o Node.js do nodejs.org, as versões extras do PHP do repositório packages.sury.org, o OpenLiteSpeed do repositório da LiteSpeed, as versões do MariaDB e do PostgreSQL dos repositórios dos seus projetos quando você as escolhe, e o ionCube Loader da ionCube. Alguns softwares instalados se conectam por conta própria: o ClamAV, por exemplo, baixa os seus bancos de dados de vírus.

A verificação de segurança não envia nada para fora do servidor. A maioria das requisições do painel se identifica como `Nefo/<version>`. Elas não incluem informações sobre o seu servidor ou os seus sites além do que a tarefa precisa, como os nomes de domínio em um pedido de certificado.
