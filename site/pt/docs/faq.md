---
title: Perguntas frequentes
description: Respostas curtas às perguntas comuns sobre o Nefo, o painel de controle gratuito para servidores Linux, sobre se ele é gratuito, a licença, os sistemas com suporte, memória, bancos de dados, Docker, migração de sites, segurança, privacidade, atualizações, desinstalação e idiomas.
---

# Perguntas frequentes

Respostas curtas às perguntas mais feitas sobre o Nefo, o painel de controle gratuito e de código aberto para um servidor Linux. Cada resposta começa pela resposta curta, depois dá os detalhes e o link para a página que trata do assunto. As informações valem para o Nefo v0.24.1.

## O que é o Nefo? {#what-is-nefo}

O Nefo é um painel de controle gratuito e de código aberto que você instala no seu próprio servidor Linux (uma VPS) com um único comando, para gerenciar sites, certificados SSL, bancos de dados, apps, arquivos, backups e segurança pelo navegador. É um único programa, `/usr/local/bin/nefo`, com a interface web embutida, que roda como o serviço systemd `nefo`. Veja [O que é o Nefo?](./).

## O Nefo é gratuito? {#is-nefo-free}

Sim. O Nefo é software livre sob a GNU AGPLv3, e todos os recursos são gratuitos para todo mundo, inclusive empresas. Não há edição paga, recurso bloqueado, chave de licença nem conta para criar. Veja [Sobre o Nefo](../about#licence).

## Em que o Nefo roda? {#what-does-nefo-run-on}

O Nefo roda no Ubuntu 22.04, 24.04 e 26.04 e no Debian 12 e 13, em x86 de 64 bits (amd64) e ARM de 64 bits (arm64), com acesso root e systemd. O instalador foi testado em servidores recém-instalados com cada um desses sistemas, em x86-64. O build arm64 ainda não foi testado em hardware ARM real, por isso experimente-o antes em um servidor que você possa reconstruir. Em outros sistemas, o instalador para, a menos que você o force. Veja [Requisitos](./requirements).

## Quanta RAM o Nefo precisa? {#how-much-ram-does-nefo-need}

O próprio painel usa cerca de 34 MB de memória, medidos em um servidor de teste depois da atualização para a v0.24.0, e o Nefo foi feito para uma VPS com 1 GB de RAM. O que você instala se soma a isso, e a página **Software** mostra mais ou menos quanto: o Nginx cerca de 15 MB, cada versão do PHP cerca de 25 MB, o MariaDB o seu buffer pool mais cerca de 100 MB (cerca de 228 MB em um servidor de 1 GB). Em um servidor de 1 GB, acrescente 1 a 2 GB de swap. Veja [Requisitos](./requirements#memory-and-disk).

## O Nefo aceita MySQL, PostgreSQL e Docker? {#does-nefo-support-mysql-postgresql-and-docker}

Sim. O Nefo roda o MariaDB (o da sua distribuição, ou o 11.4, 11.8 ou 12.3 LTS) ou o MySQL 8.4 LTS, um por servidor, e pode trocar entre os dois levando os seus bancos de dados. O PostgreSQL (a versão da sua distribuição, ou as versões 15 a 19 do projeto PostgreSQL) roda ao lado dele. O Docker roda contêineres e projetos Compose, e os apps Docker da loja de aplicativos. Veja [Bancos de dados](./databases), [PostgreSQL](./postgresql) e [Docker](./docker).

## Quais servidores web posso usar? {#which-web-servers-can-i-use}

Nginx, Apache ou OpenLiteSpeed, um por servidor. Você escolhe ao instalar o servidor web:

- O Nginx é o mais leve.
- O Apache lê os arquivos `.htaccess` que acompanham algumas aplicações PHP.
- O OpenLiteSpeed lê as regras de reescrita dos arquivos `.htaccess` e tem o LiteSpeed Cache embutido.

Com os três, o PHP roda no PHP-FPM, com um pool por site. Os sites também podem ser apps Node.js, Python ou Java, ou proxies reversos para outro endereço. Veja [Sites](./websites).

## Posso trocar de servidor web depois? {#can-i-switch-to-another-web-server-later}

Sim, mas só em um servidor sem sites. O painel se recusa a desinstalar um servidor web enquanto ele ainda serve sites, porque as configurações de cada site são escritas para aquele servidor web. Exclua os sites, desinstale o servidor web na página **Software** e depois instale o outro. Faça backup dos seus sites antes. Veja [Software](./software#web-server).

## Posso trazer os meus sites do cPanel ou do aaPanel? {#can-i-move-my-websites-from-another-panel}

Sim. O Nefo importa um backup completo do cPanel (sites, bancos de dados com os seus usuários e senhas, certificados e tarefas cron), e um arquivo compactado do site com os seus dumps SQL do aaPanel, HestiaCP, CyberPanel ou qualquer outro servidor. E-mail, zonas DNS e contas FTP não são importados. Veja [Importar sites](./import).

## O Nefo é seguro? {#is-nefo-safe}

O Nefo foi feito para ser seguro por padrão, mas é um software beta que roda como root e ainda não passou por uma auditoria de segurança externa. O painel só responde sob um caminho secreto em uma porta aleatória, usa HTTPS desde a primeira inicialização, aceita login com dois fatores, bloqueia um endereço depois de 5 tentativas de login malsucedidas em 15 minutos e registra cada alteração. As versões são assinadas com Ed25519, e a [verificação de segurança](./security-check) embutida lista as configurações que deixam o servidor exposto. Experimente-o antes em um servidor que você possa reconstruir, e guarde backups fora do servidor. Veja [Segurança](./security) e a [política de segurança](../security-policy).

## O Nefo envia dados para algum lugar? {#does-nefo-send-data-anywhere}

Sem telemetria, sem estatísticas de uso e sem verificação de licença. Por conta própria, o painel só entra em contato com o Let's Encrypt para emitir e renovar certificados e, nos builds de release, baixa o catálogo assinado da loja de aplicativos uma vez por dia. Todo o resto, como a busca por atualizações, os alertas, os backups remotos e o banco de dados de países do bloqueio por país, só acontece quando você pede ou configura. Veja [Privacidade](./#privacy).

## O Nefo faz backup no Google Drive? {#can-nefo-back-up-to-google-drive}

Sim, a partir da v0.24.0, com um cliente OAuth próprio que só enxerga os arquivos que o painel cria. O Nefo também faz backup em armazenamento compatível com S3 (Amazon S3, Cloudflare R2, Backblaze B2, Wasabi, MinIO) e em servidores SFTP, pode criptografar cada cópia remota com uma frase secreta, e um novo servidor pode encontrar os backups no armazenamento e restaurá-los. Veja [Backups](./backups#google-drive) e [Restaurar em um novo servidor](./restore-new-server).

## Como atualizo o Nefo? {#how-do-i-update-nefo}

Abra **Configurações** → **Painel** e clique em **Procurar atualizações**, ou rode `nefo update` como root. A atualização confere a assinatura e o checksum da versão antes de substituir o programa, e depois reinicia o painel; os sites continuam rodando. O Nefo nunca se atualiza sozinho. Veja [Atualização](./update).

## Como desinstalo o Nefo? {#how-do-i-uninstall-nefo}

O Nefo não tem comando de desinstalação: você o remove manualmente. Desbloqueie os sites bloqueados com `nefo unlock-files`, pare e remova o serviço `nefo` e `/usr/local/bin/nefo`, feche a porta do painel e exclua `/etc/nefo` e `/var/lib/nefo`. Os seus sites e o seu software continuam rodando, a menos que você os remova antes. Veja [Desinstalação](./uninstall).

## O Nefo pode hospedar o meu e-mail? {#can-nefo-host-my-e-mail}

Não. O Nefo não tem servidor de e-mail: ele não recebe e-mail nem hospeda caixas de correio. Os sites e os alertas podem enviar e-mail por um relay SMTP de um provedor como Gmail, SendGrid, Mailgun ou Amazon SES, e a página DNS pode acrescentar os registros MX, SPF e DMARC do seu provedor de e-mail. Use um provedor de e-mail para as suas caixas de correio. Veja [E-mail e alertas](./mail-alerts).

## Posso gerenciar vários servidores em um painel? {#can-i-manage-several-servers-from-one-panel}

Não. Instale o Nefo em cada servidor e gerencie cada um separadamente. Gerenciar vários servidores em um painel não está planejado: um painel que controla outros servidores precisa de agentes e de confiança entre as máquinas. Veja o [roadmap](../roadmap#not-planned-and-why).

## Posso dar aos meus clientes um login próprio? {#can-i-give-my-clients-their-own-login}

Não. Cada servidor tem uma única conta do painel, `admin`. O Nefo não tem contas separadas para clientes ou revendedores. Para deixar um cliente enviar arquivos para o site dele, dê a esse site uma conta SFTP, que fica presa à pasta do site. Veja [Sites](./websites#sftp-access).

## Em quais idiomas está o painel? {#which-languages-does-the-panel-speak}

Sete, a partir da v0.24.0: inglês, vietnamita, chinês (simplificado), espanhol, português, russo e indonésio, inclusive os textos da loja de aplicativos. O painel segue o idioma do seu navegador, e você pode trocá-lo na página de login ou no menu da conta. Cada idioma é um arquivo separado que o navegador só baixa quando ele é usado. Esta documentação está nos mesmos sete idiomas.

Traduções são bem-vindas. Copie `web/src/locales/en.json` para um arquivo com o código do seu idioma, traduza os valores e registre o idioma em `web/src/i18n.js`. Veja o [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md).

## Como relato um problema de segurança? {#how-do-i-report-a-security-problem}

Relate em particular, não em uma issue pública. No [repositório do GitHub](https://github.com/nefocp/nefo), abra a aba "Security" (Segurança) e escolha "Report a vulnerability" (Relatar uma vulnerabilidade). Informe a sua versão (`nefo version`), o seu sistema operacional e os passos para reproduzir o problema. Veja a [política de segurança](../security-policy), que segue o [SECURITY.md](https://github.com/nefocp/nefo/blob/main/SECURITY.md).

## Existe um registro de alterações e um roadmap? {#is-there-a-changelog-and-a-roadmap}

Sim. O [registro de alterações](../changelog) lista o que mudou em cada versão, da mais nova para a mais antiga, e o que saber antes de atualizar. O [roadmap](../roadmap) lista no que se está trabalhando antes da versão 1.0, o que está em estudo, e o que não está planejado e por quê. Para conferir você mesmo a assinatura de uma versão antes de instalá-la, veja [Verificar uma versão por conta própria](../security-policy#check-a-release-yourself).

## Como posso ajudar o projeto? {#how-can-i-support-the-project}

- Relate bugs com os passos para reproduzir, o seu sistema operacional e o `nefo version`.
- Traduza o painel para o seu idioma, ou melhore uma tradução.
- Corrija issues e envie pull requests (veja o [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md)).
- Dê uma estrela ao projeto no [GitHub](https://github.com/nefocp/nefo) e conte para outras pessoas.
