---
title: Primeiros passos
description: O que fazer depois de instalar o Nefo, da página de configuração e da proteção da sua conta até o primeiro site, os backups e a verificação de segurança.
---

# Primeiros passos

Depois de [instalar o Nefo](./install), faça login com o endereço e a senha que o instalador mostrou, escolha o software na página de configuração, troque a senha, ative a autenticação de dois fatores, dê ao painel o seu próprio domínio, e depois crie o seu primeiro site, configure os backups e rode a verificação de segurança. Cada passo é curto.

Siga estes passos na ordem. Cada um tem um link para uma página com os detalhes.

## Fazer login {#sign-in}

1. Abra a URL que o instalador mostrou, como `https://203.0.113.10:28443/k3x9p2qa7m1z/`.
2. Aceite o aviso de certificado do navegador. Ele desaparece assim que o painel tem o seu próprio certificado (veja [abaixo](#give-the-panel-its-own-domain)).
3. Informe `admin` em **Nome de usuário** e a senha do instalador, e clique em **Entrar**.

Se você perdeu a URL ou a senha, rode `nefo info` ou `nefo reset-password` no servidor (veja [Linha de comando](./cli)).

## Configurar o servidor {#set-up-the-server}

Em um servidor novo, o painel abre **Configurar este servidor** logo depois do login. Em uma única página, você escolhe o que o servidor precisa, e o painel instala tudo de uma vez, uma parte depois da outra.

1. Escolha um **Servidor web**: **Nginx** (recomendado), **Apache** para sites que precisam de arquivos `.htaccess`, **OpenLiteSpeed**, ou **Nenhum** para um servidor sem sites, como um que só roda apps Docker.
2. Marque uma ou mais **Versões do PHP**. O servidor web sempre vem também com a versão do PHP da sua distribuição. As outras versões vêm do repositório `packages.sury.org/php`, no Ubuntu e no Debian, que o painel acrescenta ao servidor. Versões antigas que passaram do fim da vida útil não são oferecidas aqui: instale-as pela página **Software** se um site antigo precisar de uma.
3. Escolha o **Servidor de banco de dados**: **MariaDB** (recomendado), **MySQL** onde a página Software o oferecer, ou **Nenhum**. Em **MariaDB**, você pode escolher a versão: a da sua distribuição (o padrão) ou uma série LTS do repositório do próprio MariaDB, como **11.4 LTS do MariaDB**, onde ela for mais nova que a da sua distribuição. Veja [Versões do MariaDB](./databases#mariadb-versions).
4. Marque os **Extras** que você quer: Redis, Node.js, Python, Docker, Adminer e o firewall web. O ClamAV é oferecido em servidores com 2 GB de RAM ou mais, e o PostgreSQL e o Memcached onde a página Software os oferecer.
5. Confira o resumo no final. Ele mostra as CPUs, a RAM e o disco livre do servidor, e mais ou menos quanta RAM e quanto disco a sua escolha acrescenta. Se a escolha talvez não couber, um aviso diz isso, mas você ainda pode instalar.
6. Clique em **Instalar**.

A página começa com uma escolha adequada à RAM do servidor: Nginx, a versão mais nova do PHP e MariaDB, mais o Redis a partir de cerca de 1,5 GB de RAM. O Docker e os outros extras nunca vêm marcados.

O número de disco conta o que os pacotes ocupam depois de instalados, as cópias dos pacotes baixados que o apt guarda em `/var/cache/apt/archives` (**Pacotes baixados**, em **Limpeza** na página **Sistema**, os exclui), os primeiros arquivos de dados do servidor de banco de dados e os bancos de dados de vírus do ClamAV. Em um servidor cujas listas de pacotes nunca foram baixadas, ele conta também essas listas, e o resumo diz quanto do número elas representam: cerca de 450 MB no Ubuntu 22.04, 300 MB no 24.04, 250 MB no 26.04, 155 MB no Debian 12 e 175 MB no Debian 13.

O log mostra cada parte enquanto ela é instalada, e a configuração inteira é uma única tarefa. Se uma parte falhar, a configuração para ali e diz o que já está instalado. Corrija o problema e clique em **Instalar** de novo: as partes já instaladas são puladas.

Para configurar o servidor você mesmo, clique em **Pular por enquanto**. Você pode rodar a configuração de novo a qualquer momento em **Software** → **Executar a configuração novamente**, ou com [`nefo setup`](./cli#nefo-setup) no servidor. Você também pode [configurar o servidor durante a instalação do Nefo](./install#set-the-server-up-in-the-same-run).

O painel deixa de abrir a configuração depois do login quando ela termina ou é pulada, e em um servidor que já tem um servidor web ou um site.

### Uma VPS que é um contêiner {#a-vps-that-is-a-container}

Algumas VPSs são contêineres (LXC, OpenVZ), e o `/proc/meminfo` delas pode mostrar a memória do host inteiro em vez da sua. O Nefo então lê o limite do próprio contêiner (o cgroup de memória) e o usa em todos os lugares: no dashboard, nos gráficos de histórico e nos alertas, nos números de RAM desta configuração, nas verificações de memória da loja de aplicativos e nas configurações que ele escolhe para o MariaDB, o MySQL, o PostgreSQL, o Redis e o Memcached. Quando isso acontece, o dashboard mostra "limite do contêiner" ao lado da RAM. O ClamAV é oferecido a partir de 2 GB desse limite.

## Trocar a senha {#change-the-password}

Substitua a senha do instalador por uma que você guarde em um gerenciador de senhas.

1. No menu à esquerda, abra **Conta**. Você também pode abri-la pelo menu da conta, no canto superior direito: **Senha e 2FA**.
2. Em **Trocar senha**, preencha **Senha atual**, **Nova senha** e **Repita a nova senha**. A nova senha precisa ter de 12 a 72 caracteres.
3. Clique em **Trocar senha**.

O painel encerra a sua sessão em todos os dispositivos. Faça login de novo com a nova senha.

## Ativar a autenticação de dois fatores {#turn-on-two-factor-authentication}

Com a autenticação de dois fatores, o login pede a sua senha e um código de 6 dígitos de um app autenticador, como Google Authenticator, Authy ou 2FAS. Enquanto você não a ativa, o dashboard e o menu de notificações mostram "O login com dois fatores está desativado".

1. Abra **Conta**.
2. Em **Autenticação de dois fatores**, clique em **Configurar**.
3. Leia o QR code com o seu app autenticador, ou digite a **Chave de configuração**.
4. Informe o código de 6 dígitos que o app mostra e clique em **Ativar**.

A partir de agora, a página de login pede um **Código de dois fatores** depois da senha.

::: warning Mantenha um jeito de voltar a entrar
Se você perder o celular, só dá para desativar a autenticação de dois fatores pelo servidor, com `nefo reset-password --disable-2fa` (veja [Solução de problemas](./troubleshooting#lost-your-2fa-phone)). Mantenha o seu acesso SSH funcionando, ou guarde a chave de configuração em um lugar seguro.
:::

## Instalar um servidor web e o PHP {#install-a-web-server-and-php}

Os sites precisam de um servidor web. O Nefo roda um por servidor. Se a [configuração](#set-up-the-server) já instalou um, pule este passo.

1. Abra **Sites**. Em um servidor novo, a página mostra **Instalar o servidor web**.
2. Escolha um servidor web:
   - **Nginx**: rápido e leve, a melhor escolha, a menos que os seus sites precisem de arquivos `.htaccess`.
   - **Apache**: lê arquivos `.htaccess`, dos quais muitos plugins do WordPress e sites vindos de hospedagem compartilhada dependem. Usa um pouco mais de memória.
   - **OpenLiteSpeed**: lê as regras de reescrita dos arquivos `.htaccess` e tem o LiteSpeed Cache embutido. Vem do próprio repositório de pacotes da LiteSpeed.
3. Clique no botão de instalação, como **Instalar Nginx**.

O painel instala o servidor web e a versão do PHP da sua distribuição, e abre as portas 80 e 443 se o firewall (ufw) estiver ativado. Isso leva de um a três minutos, e o progresso aparece na página.

Para trocar de servidor web depois, você precisa excluir todos os sites antes. Acrescente mais versões do PHP, o MariaDB e outros componentes na página **Software**. Veja [Software](./software).

## Dar ao painel o seu próprio domínio {#give-the-panel-its-own-domain}

Com o seu próprio domínio, o painel ganha um certificado Let's Encrypt gratuito, e o aviso do navegador desaparece. Você precisa de um servidor web antes, porque é ele que responde ao Let's Encrypt na porta 80.

1. No seu provedor de DNS, crie um registro A para um nome como `panel.example.com`, apontando para o endereço IP do servidor. Ele precisa apontar direto para o servidor, sem proxy nem CDN na frente (no Cloudflare, use "DNS only", somente DNS).
2. Confira se a porta 80 está aberta, inclusive no firewall de nuvem do seu provedor.
3. No painel, abra **Configurações** → **Certificado**.
4. Em **Domínio do painel**, informe `panel.example.com` e clique em **Obter certificado**.

O painel primeiro confere se o domínio chega a este servidor e depois pede o certificado ao Let's Encrypt. Ele passa a usar o novo certificado em até um minuto e mostra o novo endereço, como `https://panel.example.com:28443/k3x9p2qa7m1z/`. Use esse endereço daqui em diante: o painel mantém a mesma porta e o mesmo caminho secreto. Com o endereço IP, o navegador volta a avisar, porque o certificado é para o domínio.

O certificado se renova automaticamente. Para voltar ao certificado autoassinado, clique em **Excluir** na mesma página.

Se o pedido falhar, veja [Solução de problemas](./troubleshooting#a-certificate-request-fails).

## Criar o seu primeiro site {#create-your-first-website}

1. Aponte o registro A do domínio, como `example.com`, para o endereço IP do servidor.
2. Abra **Sites** e clique em **Adicionar site**.
3. Informe o **Domínio** sem `http://` nem `www.`. Deixe **Responder também em www.example.com** marcado se você quiser os dois nomes.
4. Escolha o **Tipo**: uma versão do PHP, **Estático (só HTML)**, um app (Node.js, Python ou Java), ou um proxy reverso para outro endereço.
5. Clique em **Criar site**.
6. Clique em **Ativar HTTPS** ao lado do site para obter um certificado gratuito.

Envie os seus arquivos para `/var/www/example.com/public`, pela página **Arquivos** ou por SFTP. Para instalar o WordPress ou outra aplicação, use a **Loja de aplicativos**. Veja [Sites](./websites), [Certificados SSL](./ssl) e [Loja de aplicativos](./apps).

## Configurar os backups {#set-up-backups}

1. Abra **Backups**.
2. Em **Backups automáticos**, clique em **Adicionar agendamento**.
3. Escolha do que fazer backup (um site, um banco de dados ou uma pasta), com que frequência e o **Número de backups a manter**.
4. Clique em **Salvar agendamento**.

O backup de um site contém os arquivos dele e os bancos de dados ligados a ele. Os backups ficam guardados no servidor, em `/var/backups/panel`. Acrescente um **Armazenamento remoto** (compatível com S3, SFTP ou Google Drive, criptografado se você quiser) para que uma cópia sobreviva se o servidor for perdido. Veja [Backups](./backups).

## Ativar o firewall {#turn-on-the-firewall}

Se o dashboard diz "O firewall está desativado", qualquer porta que um programa abra fica acessível pela internet.

1. Abra **Firewall e SSH**.
2. Clique em **Ativar o firewall**.

O painel mantém abertos o SSH e a própria porta do painel. Se a página disser que o ufw não está instalado, instale-o antes com `apt install ufw`. Veja [Segurança](./security).

## Rodar a verificação de segurança {#run-the-security-check}

1. Abra **Verificação de segurança**, no grupo **Segurança** do menu à esquerda.
2. Clique em **Executar a verificação**.
3. Corrija o que aparece em **A corrigir**, começando pelas de gravidade **Crítica**: cada ocorrência tem um botão para a página que a corrige.

Depois disso, a verificação roda sozinha uma vez por dia, e o dashboard mostra o que ela encontrou. Veja [Verificação de segurança](./security-check).
