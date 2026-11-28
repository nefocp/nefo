---
title: Sites
description: Escolha um servidor web, crie sites PHP, estáticos, de app (Node.js, Python, Java) e de proxy reverso, e gerencie os domínios, as configurações do PHP, as opções de acesso, o SFTP, o tráfego, os logs, os clones e a exclusão deles.
---

# Sites

Um site no Nefo é um domínio servido pelo único servidor web do servidor, Nginx, Apache ou OpenLiteSpeed: um site PHP, um site estático, um app (Node.js, Python ou Java) atrás de um proxy, ou um proxy reverso para outro endereço. Cada site tem o seu próprio usuário Linux, a sua pasta e uma conta SFTP opcional, e pode ser clonado, ter backup ou ser excluído pelas configurações dele.

Esta página trata do servidor web, da criação de um site e de todas as configurações que um site tem: domínios, PHP, regras de reescrita, opções de acesso, SFTP, estatísticas de tráfego, logs, clonagem e exclusão. Os certificados têm a sua própria página, [Certificados SSL](./ssl), assim como o [bloqueio por país e o bloqueio de arquivos](./website-protection).

## Qual servidor web devo escolher? {#choose-the-web-server}

Um servidor roda um servidor web por vez, e o Nefo configura cada site para ele. Antes de você acrescentar o primeiro site, a página **Sites** pede que você escolha um:

| Servidor web | Escolha quando | Memória em repouso |
|---|---|---|
| Nginx | Os seus sites não precisam de arquivos `.htaccess`. É a escolha mais leve. | cerca de 15 MB |
| Apache | Os seus sites dependem de arquivos `.htaccess`, por exemplo plugins do WordPress ou sites trazidos de hospedagem compartilhada. | cerca de 40 MB |
| OpenLiteSpeed | Você quer as regras de reescrita dos arquivos `.htaccess` e o plugin LiteSpeed Cache para o WordPress. Ele também responde por HTTP/3. | cerca de 26 MB |

Para instalá-lo:

1. Abra **Sites**.
2. Em **Instalar o servidor web**, selecione Nginx, Apache ou OpenLiteSpeed.
3. Clique em **Instalar Nginx** (o botão traz o nome do servidor que você selecionou).

O Nefo instala o servidor web e a versão do PHP da sua distribuição com o PHP-FPM, e depois abre as portas 80 e 443 no firewall (ufw), mais a 443/udp para o HTTP/3 no OpenLiteSpeed. Isso leva de um a três minutos. O OpenLiteSpeed vem do próprio repositório de pacotes da LiteSpeed, e o console WebAdmin dele fica desligado.

Os três servidores web rodam o PHP em um pool do PHP-FPM separado para cada site. Só o Nginx consegue limitar as requisições por visitante; as outras configurações funcionam do mesmo jeito nos três.

### Trocar de servidor web {#switch-to-another-web-server}

O Nefo não consegue passar os sites existentes para outro servidor web, porque a configuração de cada site é escrita para o que está instalado. Para trocar:

1. Exclua todos os sites (veja [Excluir um site](#delete-a-website)). Faça backup deles antes, se precisar deles: veja [Backups](./backups).
2. Abra **Software** e encontre o cartão **Nginx, Apache ou OpenLiteSpeed**.
3. Clique em **Desinstalar** no servidor web atual, digite o nome dele (`nginx`, `apache` ou `openlitespeed`) e confirme.
4. Clique em **Instalar** no servidor web que você quer.

Desinstalar mantém as pastas dos sites em `/var/www`, os certificados e a conta no Let's Encrypt. O PHP continua instalado. O Adminer é removido, porque precisa do servidor web, e as regras de firewall que o Nefo acrescentou para os sites (portas 80 e 443) são excluídas.

O Nefo também se recusa a instalar um servidor web enquanto outro estiver instalado no servidor, mesmo um que você mesmo instalou: os dois precisam das portas 80 e 443. A página **Software** então o mostra como "instalado, não usado pelo painel". Remova-o antes.

## Criar um site {#create-a-website}

1. Aponte o registro DNS A do domínio para o endereço IP do seu servidor, por exemplo `203.0.113.10`.
2. Abra **Sites** e clique em **Adicionar site**.
3. Em **Domínio**, digite o nome sem `http://` nem `www.`, por exemplo `example.com`.
4. Deixe **Responder também em www.example.com** marcado se o site também deve responder em `www`. O Nefo o marca para você quando o domínio tem duas partes, como `example.com`.
5. Escolha o **Tipo** (veja a tabela abaixo).
6. Clique em **Criar site**.

O Nefo então mostra "example.com está pronto. Envie os seus arquivos para /var/www/example.com/public." Uma página de boas-vindas responde até você substituí-la.

| Tipo | O que o site faz |
|---|---|
| **PHP 8.3** (uma opção por versão do PHP instalada) | Serve os seus arquivos e roda os arquivos `.php` com essa versão do PHP. |
| **Estático (só HTML)** | Serve HTML, CSS, JavaScript e outros arquivos. Sem PHP. |
| **App (Node.js, Python, Java…) atrás de um proxy** | Encaminha cada requisição para um programa que escuta em uma porta deste servidor. |
| **Proxy reverso para outro endereço** | Encaminha cada requisição para um endereço que você informar: outro servidor, ou um serviço deste servidor que o Nefo não roda. Veja [Sites de proxy reverso](#reverse-proxy-websites). |

Um site não pode mudar de tipo depois: um site estático não pode virar um site PHP, um site de app não pode virar um site PHP ou estático, e um site de proxy reverso também não pode virar outro tipo. Crie um novo site. Para instalar o WordPress ou outra aplicação, veja a [Loja de aplicativos](./apps). Os apps Docker recebem o seu próprio site por lá.

O Nefo recusa um domínio que não é válido (escreva os nomes internacionais em punycode, `xn--…`), um nome que outro site já usa e um domínio cuja pasta já existe em `/var/www`.

### Sites de app (Node.js, Python, Java) {#app-websites-node-js-python}

Para o tipo **App (Node.js, Python, Java…) atrás de um proxy**, o formulário pede mais duas coisas:

- **Porta em que o app escuta (127.0.0.1)**: uma porta de 1024 a 65535. O Nefo recusa a própria porta do painel, a 3306 (MariaDB), a 6379 (Redis), a 7809 (o receptor dos webhooks do Git) e uma porta que outro site de app usa.
- **Comando de início (opcional)**: uma linha de no máximo 1000 caracteres, como `node server.js`. O Nefo o roda na pasta `app` do site, com o usuário Linux do site, com `PORT`, `HOST=127.0.0.1` e `NODE_ENV=production` definidos. Ele roda como o serviço systemd `panel-app-<user>.service` e reinicia 3 segundos depois de parar. Deixe o comando vazio se o programa for iniciado de outro jeito, por exemplo no Docker.

Coloque o seu código em `/var/www/example.com/app`, por [SFTP](#sftp-access) ou com o [deploy pelo Git](./git-deploy), que faz o deploy de um site de app nessa pasta. Depois, nas configurações do site:

1. Na aba **Runtime**, escolha o **Runtime**: **Padrão do servidor (node em /usr/local/bin)**, uma versão do Node.js que você instalou na página **Software**, o Python (`app/.venv`), que precisa do Python instalado na página **Software**, ou uma versão do Java que você instalou na página **Software**, mostrada como "Java 21 (OpenJDK 21.0.9)", por exemplo. A mesma aba muda o **Comando de início (opcional)**. Clique em **Salvar**.
2. Na aba **Visão geral**, em **Programa da aplicação**, clique em **Instalar dependências**. O Nefo roda `npm ci` (ou `npm install`, sem um `package-lock.json`) para um `package.json`. Para o Python, ele cria o ambiente virtual `app/.venv` e instala o `requirements.txt` com o pip. Ele roda com o usuário do site, por até 20 minutos.
3. Clique em **Reiniciar** (ou **Iniciar**) para rodar o app com as dependências dele.

O cartão **Programa da aplicação** aparece quando o site tem um comando de início. Ele também tem **Parar** e **Logs do app**. Na aba **Proxy reverso**, você pode mudar a porta. O servidor web encaminha as requisições para `127.0.0.1` nessa porta, com o endereço do visitante em `X-Real-IP` e `X-Forwarded-For`, e as conexões WebSocket funcionam. Para encaminhar para outro servidor, ou para um serviço deste servidor que o Nefo não inicia, crie um [site de proxy reverso](#reverse-proxy-websites).

A lista de sites tem um filtro **Java** assim que existe um site de app Java, ao lado dos filtros dos outros tipos.

#### Sites de app Java {#java-app-websites}

Para um programa Java, defina o **Comando de início (opcional)** como o comando que o inicia, por exemplo `java -jar app.jar`, e coloque o `app.jar` em `/var/www/example.com/app`. Escolha a versão do Java como o **Runtime**. Instale antes o Java na página **Software** (veja [Java](./software#java)).

- O serviço do app recebe o `JAVA_HOME` dessa versão e a pasta `bin` dela no início do `PATH`, por isso o `java` é essa versão. Além de `PORT` e `HOST=127.0.0.1`, que todo site de app recebe, ele recebe `SERVER_PORT` (a mesma porta) e `SERVER_ADDRESS=127.0.0.1`, que o Spring Boot lê. Uma aplicação Spring Boot então escuta na porta certa, só neste servidor, sem configurações próprias. Outros frameworks leem `PORT` ou precisam dela na sua própria configuração.
- **Limite de memória (MB)** aparece para um runtime Java. Ele define o heap máximo do Java (`-Xmx`), de 32 a 65536 MB, pelo `JDK_JAVA_OPTIONS`, que todo `java` que o comando de início roda lê, inclusive um script de início. Deixe vazio, e o Java usa até um quarto da memória do servidor. O processo usa um pouco mais de memória do que o heap dele. O Java mostra `NOTE: Picked up JDK_JAVA_OPTIONS: -Xmx512m` no início dos **Logs do app**.
- Os scripts de [deploy pelo Git](./git-deploy) do site rodam com o mesmo `JAVA_HOME` e o mesmo `PATH`, por isso `./mvnw package` ou `./gradlew build` compilam com a versão do Java do site. O limite de memória não vale para eles.
- **Instalar dependências** não é oferecido para um site Java: gere o jar no seu script de deploy ou antes de enviá-lo.
- Uma versão do Java que um site usa não pode ser desinstalada na página **Software**. Escolha outro runtime antes, ou exclua o site.

## Sites de proxy reverso {#reverse-proxy-websites}

Um site de proxy reverso encaminha cada requisição para um endereço que você escolhe: um site ou app em outro servidor, ou um serviço deste servidor que o Nefo não roda, como um programa que você mesmo iniciou ou um contêiner de um projeto Compose seu. O site tem o seu próprio domínio, certificado HTTPS, opções de acesso e firewall web, como qualquer outro site. Ele funciona com o Nginx, o Apache e o OpenLiteSpeed.

1. Abra **Sites** e clique em **Adicionar site**.
2. Digite o **Domínio** e escolha o tipo **Proxy reverso para outro endereço**.
3. Em **Encaminhar para**, digite o endereço: `http://` ou `https://`, um nome de host ou um endereço IP, uma porta opcional e um caminho opcional, por exemplo `http://198.51.100.7:8080` ou `https://backend.example.com/app`.
4. Escolha o **Cabeçalho Host enviado**: o domínio deste site, ou o host do upstream.
5. Clique em **Criar site**.

O Nefo mostra "example.com foi criado. Ele encaminha para … assim que o domínio dele apontar para este servidor." O endereço é guardado com a porta, por isso `https://backend.example.com/app` aparece como `https://backend.example.com:443/app`.

O caminho da requisição vai depois do caminho do endereço: com `http://198.51.100.7:8080/app`, uma visita a `https://example.com/shop?page=2` chega a `http://198.51.100.7:8080/app/shop?page=2`.

### As opções {#the-options}

Mude-as na aba **Proxy reverso** do site e clique em **Salvar**. Um novo site de proxy reverso vem com o WebSocket e os cabeçalhos X-Forwarded-\* ativados, e um limite de tempo de 60 segundos.

| Opção | O que faz |
|---|---|
| **Encaminhar para** | O endereço, como acima. |
| **Cabeçalho Host enviado** | O domínio do site envia o domínio que o visitante pediu: a maioria dos apps atrás de um proxy espera isso. **O host do upstream** envia o próprio nome do upstream (com a porta, quando ela não é 80 nem 443), para um servidor que só responde ao seu próprio nome. O próprio endereço do upstream nos redirecionamentos (`Location`) é então trocado pelo do site. |
| **WebSocket** | Repassa as conexões WebSocket (chat, atualizações ao vivo, terminais). |
| **Cabeçalhos X-Forwarded-\*** | Informa ao upstream o endereço do visitante e se a requisição veio por HTTPS: `X-Forwarded-For`, `X-Forwarded-Proto`, `X-Forwarded-Host` e `X-Real-IP`. Quando desativado, esses cabeçalhos são removidos, inclusive os que um visitante envia. |
| **Limite de tempo (segundos)** | Quanto tempo o upstream pode levar para responder, de 5 a 3600 segundos. A própria conexão precisa abrir em 30 segundos (ou no limite de tempo, se for menor). Um upstream mais lento faz o visitante receber uma página de erro: 504 com o Nginx, 502 com o Apache. |
| **Ignorar o certificado do upstream (autoassinado)** | Aparece para um endereço `https://`. Aceita qualquer certificado do upstream. |

::: warning
**Ignorar o certificado do upstream (autoassinado)** mantém criptografada a conexão com o upstream, mas qualquer um que consiga interceptá-la poderia se passar pelo upstream. Use só para um upstream na sua própria rede com um certificado autoassinado.
:::

Sem essa opção, o Nginx e o Apache conferem o certificado de um upstream `https://` com as autoridades certificadoras do servidor (`/etc/ssl/certs/ca-certificates.crt`), para o nome do cabeçalho Host: o domínio do site, ou o host do upstream, se você o escolheu. Um certificado autoassinado ou expirado faz o visitante receber uma página de erro (502 com o Nginx, 500 com o Apache).

### O que o Nefo recusa {#what-nefo-refuses}

- um endereço que não seja `http://` ou `https://` com um nome de host ou um endereço IP, uma porta opcional (1 a 65535) e um caminho, por exemplo um com nome de usuário, uma query (`?`), um fragmento (`#`), espaços, `..` no caminho, ou os endereços `0.0.0.0` e `::`;
- um nome de host que não pode ser encontrado ("O nome de host … não foi encontrado. Confira-o ou use o endereço IP dele.");
- o próprio painel: a porta do painel ou a `7809` (o receptor dos webhooks do Git) neste servidor, quer você escreva `127.0.0.1`, `localhost`, um dos próprios endereços IP do servidor ou um nome que aponte para eles;
- as portas 80 e 443 neste servidor: o site encaminharia para o próprio servidor web;
- um **Limite de tempo (segundos)** abaixo de 5 ou acima de 3600.

### Bom saber {#good-to-know}

- Com o Nginx, um nome de host é consultado quando as requisições chegam, com os servidores DNS do `/etc/resolv.conf`, por isso um nome que não resolve na inicialização nunca impede o Nginx de iniciar. Essas consultas não leem o `/etc/hosts`: para um nome que só o `/etc/hosts` conhece, use o endereço IP dele.
- O HTTPS, o redirecionamento para HTTPS e o HSTS funcionam como nos outros sites: veja [Certificados SSL](./ssl).
- **Suspender**, **Redirecionamento**, a proteção por senha, os endereços IP bloqueados, o limite de requisições, o [bloqueio por país](./website-protection#block-or-allow-countries) e o firewall web valem antes de uma requisição chegar ao upstream.
- As abas das regras de reescrita, da página 404 personalizada, da proteção contra hotlink e do bloqueio de arquivos não se aplicam a um site de proxy reverso e ficam ocultas. As suas próprias diretivas (a aba de configuração do servidor web) continuam valendo.
- O site continua tendo a pasta `/var/www/example.com` e o usuário Linux, para o SFTP e os backups, mas não serve esses arquivos.

### Com o OpenLiteSpeed {#with-openlitespeed}

O OpenLiteSpeed encaminha de um jeito diferente do Nginx e do Apache. A aba **Proxy reverso** avisa isso em uma nota, e **Ignorar o certificado do upstream (autoassinado)** não aparece lá:

- Ele nunca confere o certificado de um upstream `https://`: um certificado autoassinado funciona sem a opção.
- O **Limite de tempo (segundos)** não é aplicado: o OpenLiteSpeed não tem limite para uma resposta via proxy.
- Ele não envia o endereço do visitante em `X-Forwarded-For`: ele envia `X-Forwarded-Host`, e `X-Forwarded-Proto` nas requisições HTTPS. Com os **Cabeçalhos X-Forwarded-\*** desativados, o Nefo remove o `X-Forwarded-For` (e o `X-Real-IP` é sempre removido); o OpenLiteSpeed continua enviando o `X-Forwarded-Host`.
- As conexões WebSocket chegam ao upstream sem o caminho do endereço (`/live`, não `/app/live`), e não funcionam com um upstream `https://`.
- Os caminhos, o cabeçalho Host do upstream, a senha e os endereços bloqueados funcionam como nos outros servidores web.

## Abrir as configurações de um site {#open-a-website-s-settings}

Clique no domínio na lista, ou em **Configurações** no final da linha dele. As configurações abrem em um painel lateral com abas em quatro grupos. Algumas abas só aparecem para alguns sites:

| Grupo | Abas |
|---|---|
| **Básico** | **Visão geral**, **Domínios**, **PHP** (sites PHP), **Runtime** (sites de app), **Proxy reverso** (sites de proxy reverso) |
| **Segurança** | **SSL / HTTPS**, **Firewall web (WAF)**, **Bloquear acesso**, **Bloqueio por país**, **Bloqueio de arquivos** (não para apps Docker e sites de proxy reverso), **Proteção contra hotlink** (não para sites de proxy reverso), **Verificação de malware** |
| **Configuração** | **Reescrita** (sites PHP e estáticos), **Cache de página** (sites PHP no Nginx e no OpenLiteSpeed), **Configuração do Nginx** (com o nome do seu servidor web), **Redirecionamento**, **Proxy reverso** (sites de app: a porta), **Página 404** (não para sites de proxy reverso) |
| **Operações** | **WordPress** (sites WordPress), **Deploy pelo Git** (não para apps Docker), **SFTP**, **Backups**, **Monitoramento**, **Tráfego**, **Logs**, **Expiração e suspensão** |

A aba **Visão geral** mostra a **Pasta** do site (com **Abrir em Arquivos**), o **Usuário Linux** dele, os domínios, a versão do PHP ou o runtime, o servidor web, o HTTPS e o estado do firewall. Clique em **Adicionar uma nota** para escrever uma linha (até 200 caracteres) que aparece embaixo do domínio na lista, como o nome de um cliente.

## O usuário Linux e as pastas do site {#the-website-s-linux-user-and-folders}

Cada site tem o seu próprio usuário Linux, para que um site invadido não consiga ler nem alterar os outros. O nome vem do domínio: `example.com` vira `example_com`, e `blog.example.com` vira `blog_example_com` (com `_2`, `_3`… se o nome já estiver em uso). O usuário não tem senha nem shell, e não consegue entrar por SSH, a menos que você ative o [SFTP](#sftp-access). O PHP do site e o app dele rodam com esse usuário, e o PHP só consegue abrir arquivos dentro da pasta do site.

| Caminho | O que contém |
|---|---|
| `/var/www/example.com/public` | A pasta web (raiz do documento). Os arquivos do seu site ficam aqui. |
| `/var/www/example.com/logs` | O `php-error.log`, os erros do PHP do site. |
| `/var/www/example.com/tmp` | Os arquivos temporários do PHP, os uploads em andamento e as sessões. |
| `/var/www/example.com/app` | Só sites de app: o código do programa. |
| `/var/log/nginx/example.com.access.log` e `.error.log` | Os logs do servidor web para o site (`/var/log/apache2` no Apache, `/var/log/openlitespeed` no OpenLiteSpeed). |
| `/var/lib/nefo/certs/example.com` | O certificado do site. |
| `/var/cache/nginx/nefo/example.com` | Só no Nginx: o [cache de página](#page-cache), quando está ativado. |

Os arquivos do servidor web e do PHP-FPM que o Nefo grava começam com "Managed by Nefo. Do not edit" (gerenciado pelo Nefo, não edite). O Nefo os reescreve, por isso faça as suas alterações pelo painel.

## Domínios e aliases {#domains-and-aliases}

Na aba **Domínios**, **Nomes de domínio extras** lista os outros nomes que abrem o mesmo site, um por linha, como `www.example.com`. Um site pode ter até 20. Cada nome precisa apontar para este servidor e só pode pertencer a um site.

Um wildcard como `*.example.com` responde por todos os subdomínios. O certificado dele precisa de validação por DNS pelo Cloudflare: veja [Certificados SSL](./ssl).

Se o DNS do domínio está no Cloudflare, **Apontar o DNS para cá…**, na mesma aba, cria ou atualiza os registros A (e AAAA) de todos esses nomes, depois de uma prévia: veja [DNS (Cloudflare)](./dns#point-a-website-to-this-server).

Se o site tem HTTPS e você muda os nomes dele, o Nefo diz "Os nomes de domínio mudaram. Obtenha um novo certificado para que o HTTPS cubra todos eles." Clique em **Renovar certificado**.

## Versão e configurações do PHP {#php-version-and-settings}

A aba **PHP** de um site PHP tem estas configurações:

| Configuração | Padrão | Permitido |
|---|---|---|
| **Versão do PHP** | a versão escolhida na criação | qualquer versão do PHP instalada |
| **Limite de memória (MB)** | 256 | 32 a 4096 |
| **Maior upload (MB)** | 64 | 1 a 4096 (o limite de tamanho de requisição do servidor web acompanha) |
| **Execução mais longa de um script (segundos)** | 300 | 10 a 3600 |
| **Workers do PHP** | 5 | 1 a 200 |
| **Modo dos workers** | **Iniciar sob demanda (economiza memória)** | ou **Manter alguns prontos (mais rápido para sites movimentados)** |

**Workers do PHP** é quantas requisições o PHP do site atende ao mesmo tempo. Cada worker pode usar até o limite de memória, por isso workers × memória deve caber na RAM do servidor.

Por segurança, o Nefo desativa `exec`, `passthru`, `shell_exec`, `system`, `proc_open`, `popen` e `pcntl_exec` em todos os sites, porque eles deixam um site invadido rodar programas no servidor. Em **Permitir funções desativadas por segurança**, marque uma só se uma aplicação realmente precisar dela.

Clique em **Salvar**. Se o PHP-FPM ou o servidor web rejeitar o resultado, o Nefo coloca de volta as configurações anteriores. Para instalar outras versões do PHP ou extensões, o ionCube Loader, ou para mudar o `php.ini`, veja [Software](./software).

## Regras de reescrita e as suas próprias diretivas {#rewrite-rules-and-your-own-directives}

Na aba **Reescrita**, **Regras de reescrita** oferece regras prontas para aplicações comuns: **Padrão (front controller: index.php ou index.html)**, WordPress, Laravel, Symfony, CodeIgniter 4, Drupal, Joomla, OpenCart, Nextcloud, Moodle e phpBB para sites PHP, e Single-page app e Static site (clean URLs) para qualquer site. A aba mostra as regras da predefinição que você escolher. No Nginx, uma predefinição substitui o bloco `location /` padrão. No Apache, o próprio arquivo `.htaccess` do site pode substituir o front controller da predefinição, e no OpenLiteSpeed o front controller roda depois das regras do `.htaccess` do site. Nos dois, as regras de proteção da predefinição não podem ser desfeitas por arquivos `.htaccess`.

**Static site (clean URLs)** serve as páginas sem a terminação `.html`, do jeito que os geradores de sites como o VitePress (com `cleanUrls`) fazem os links para elas: `/docs/install` serve `docs/install.html`, e `/docs/` serve `docs/index.html`. Os outros arquivos são servidos como estão, e uma página que não existe continua recebendo um 404.

A predefinição phpBB é para um phpBB que você mesmo instala. Ela manda os endereços que não são arquivos para o `app.php` do phpBB, e nunca serve os arquivos de configuração do phpBB (`config.php`, `common.php`), as pastas `cache`, `files` (anexos), `store` (backups), `includes`, `phpbb` e `vendor` dele, nem `images/avatars/upload`: esses respondem 403.

A aba seguinte tem o nome do seu servidor web, por exemplo **Configuração do Nginx**. Em **Diretivas do Nginx** (ou **Diretivas do Apache**, **Diretivas do OpenLiteSpeed**), você pode acrescentar as suas próprias diretivas, até 32 KB, na sintaxe do servidor web:

- O Nginx as acrescenta dentro do bloco `server` do site, antes das regras de reescrita.
- O Apache as acrescenta no final do bloco `VirtualHost` do site.
- O OpenLiteSpeed as acrescenta no final do bloco `virtualHost` do site.

O servidor web confere a nova configuração antes de qualquer alteração. Se ele recusar, o Nefo mostra o erro e o site continua funcionando como antes. Clique em **Mostrar o arquivo de configuração completo** para ver o que o Nefo gera.

Sobre os arquivos `.htaccess`: o Apache os lê. O OpenLiteSpeed só usa as regras de reescrita deles, que o Nefo copia para a configuração do site segundos depois de uma alteração. O Nginx os ignora.

## Cache de página {#page-cache}

No Nginx, a aba **Cache de página** de um site PHP ativa o cache FastCGI do Nginx para ele: o Nginx guarda cada página que o PHP gera e a serve de novo sem rodar o PHP nem consultar o banco de dados. Ative a chave **Cache de página**.

- Uma página é guardada por 10 minutos, uma página que não existe (404) por 1 minuto. Se o PHP falhar ou parar de responder, o Nginx serve a última cópia que ainda tiver.
- O Nginx pergunta ao PHP toda vez nas requisições que não são GET nem HEAD (formulários), nos endereços com um `?`, para os visitantes com um cookie de login ou de carrinho (WordPress `wordpress_logged_in_`, `wp-postpass_`, `comment_author_`, WooCommerce `woocommerce_items_in_cart`, `wp_woocommerce_session_`, e o próprio `PHPSESSID` do PHP), e nas páginas em `/wp-admin/`, `/wp-login.php`, `/xmlrpc.php`, `/wp-json/`, `/wp-cron.php`, `/cart/`, `/checkout/` e `/my-account/`. As páginas que o PHP marca como privadas ou envia com um cookie também não são guardadas.
- Cada site tem o seu próprio cache, de até 256 MB, em `/var/cache/nginx/nefo/example.com`. Uma página que ninguém pediu em uma hora é descartada.

Depois de alterar uma página, clique em **Limpar o cache**: todos os visitantes recebem a nova versão na hora. Caso contrário, os visitantes sem login podem ver a página antiga por até 10 minutos. Desativar o cache, ou excluir o site, exclui a pasta dele.

Para conferir se funciona, abra uma página duas vezes em uma janela anônima, ou rode `curl -sI https://example.com/` duas vezes: o cabeçalho `X-Cache-Status` diz `MISS` e depois `HIT` (`BYPASS` para as requisições acima). Os arquivos estáticos não têm esse cabeçalho: o Nginx os serve diretamente de qualquer jeito.

No OpenLiteSpeed, a aba diz que o LSCache, o próprio cache de página dele, vem embutido: para o WordPress, instale o plugin LiteSpeed Cache; as outras aplicações o ativam com regras de cache no `.htaccess`. O Apache não tem cache de página no Nefo, nem aba.

## Suspender, redirecionar ou proteger um site com senha {#suspend-redirect-or-password-protect-a-website}

**Suspender.** Desative a chave na coluna **Rodando** da lista, ou **Site rodando** na aba **Visão geral**. Os visitantes passam a receber HTTP 503 com "This website is temporarily unavailable." (este site está temporariamente indisponível). Os arquivos e as configurações ficam, e o certificado continua sendo renovado. Para suspender vários sites, marque-os, escolha **Suspender** em **Ação em massa** e clique em **Aplicar**; **Rodar novamente** os ativa de novo.

**Redirecionar.** Na aba **Redirecionamento**, marque **Redirecionar todas as requisições para outro endereço** e informe o **Endereço**, como `https://example.org`. É um redirecionamento permanente (301) que mantém o caminho: `/page` vai para o novo endereço seguido de `/page`. Um redirecionamento só funciona enquanto o site está rodando.

**Senha.** Na aba **Bloquear acesso**, em **Proteção por senha**, marque **Proteger com uma senha** e informe um **Nome de usuário** e uma **Senha** de 8 a 72 caracteres. O navegador então os pede antes de mostrar o site. Deixe a senha vazia depois para manter a atual. A validação do Let's Encrypt e os webhooks do Git continuam chegando ao site.

A mesma aba bloqueia endereços IP e, no Nginx, limita as requisições por visitante: veja [Segurança](./security#rate-limit-and-blocked-addresses). Para barrar países inteiros, veja [Bloqueio por país](./website-protection#block-or-allow-countries).

## Página 404 personalizada e proteção contra hotlink {#custom-404-page-and-hotlink-protection}

**Página 404.** Na aba **Página 404**, marque **Usar a minha própria página 404** e envie a sua página como `public/404.html`. As páginas que a sua aplicação trata por conta própria, como as páginas do WordPress ou do Laravel, mantêm a sua própria página 404.

**Proteção contra hotlink.** Na aba **Proteção contra hotlink**, marque **Proteção contra hotlink para imagens e mídia**. Os outros sites passam a receber HTTP 403 quando incorporam os seus arquivos `jpg`, `jpeg`, `png`, `gif`, `webp`, `avif`, `svg`, `ico`, `bmp`, `mp4`, `webm`, `mp3`, `ogg` e `pdf`. O seu próprio site, os mecanismos de busca (Google, Bing, DuckDuckGo e Yahoo) e os visitantes que abrem um arquivo diretamente sempre o recebem. Liste até 20 outros sites em **Outros sites que podem exibi-los**, um por linha.

## Data de expiração {#expiry-date}

Na aba **Expiração e suspensão**, defina uma **Data de expiração** e clique em **Salvar**. O site funciona até o fim desse dia (hora do servidor); depois disso, o Nefo suspende o site automaticamente. Ele confere a cada hora. Clique em **Nunca** para remover a data. Nos últimos 7 dias, a lista mostra "expira" e a data embaixo do domínio. Isso é útil para sites de clientes pagos até uma data.

A mesma aba tem **Suspender este site**, para suspender o site agora.

## Acesso SFTP {#sftp-access}

O SFTP deixa você ou um cliente enviar arquivos com o FileZilla, o WinSCP ou qualquer app de SFTP. Os arquivos passam então a pertencer ao usuário do site, como deve ser. Ele precisa do servidor OpenSSH instalado.

1. Abra a aba **SFTP** do site e marque **Permitir login SFTP para este site**.
2. Escolha como a conta entra:
   - **Gerar uma nova senha forte**
   - **Escolher uma senha (pelo menos 12 caracteres)**, até 128 caracteres
   - **Manter a senha atual** (quando houver uma definida)
   - **Sem senha: entrar só com chaves SSH**
3. Opcional: cole **Chaves públicas SSH (opcional)**, uma por linha como no `authorized_keys`, até 50. As opções como `command=` são removidas.
4. Clique em **Salvar**. Uma senha gerada é mostrada uma única vez: copie-a agora.
5. Conecte-se com o **Host**, a **Porta**, o **Nome de usuário** e a **Pasta** mostrados. O host é o endereço com que você abriu o painel, e a porta é a porta SSH do servidor. A aba também mostra o endereço completo, por exemplo `sftp://example_com@panel.example.com/var/www/example.com`.

A conta fica presa à pasta do site. Ela só enxerga `/var/www/example.com`, no mesmo caminho que no painel e no PHP, e não consegue rodar comandos nem encaminhar portas. A política de senhas do servidor pode recusar uma senha que você escolher; o Nefo então mostra por quê. Enquanto os [arquivos do site estiverem bloqueados](./website-protection#lock-a-website-s-files), a conta SFTP só consegue alterar as pastas graváveis.

Desativar o SFTP remove a senha e as chaves e encerra as sessões abertas.

## Estatísticas de tráfego {#traffic-statistics}

A aba **Tráfego** lê os logs de acesso do servidor web para o site. Escolha o **Período**: **Hoje**, **7 dias**, **14 dias** ou **30 dias**. Ela mostra:

- **Requisições**, **Visitantes** (endereços IP diferentes), **Banda**, **Taxa de erros** (4xx e 5xx) e **Bots**
- as requisições e os visitantes por hora ou por dia
- **Páginas mais acessadas**, **Endereços IP mais frequentes**, **Sites de referência**, **Navegadores** e **Bots e scripts**

As verificações de certificado e as verificações do próprio [monitoramento](#monitoring) do painel não são contadas. O servidor guarda cerca de 14 dias de logs, por isso o tráfego mais antigo não é conhecido. Para logs muito grandes, só as 5.000.000 de linhas mais recentes são lidas. A coluna **Visitas, 7 dias** da lista de sites vem dos mesmos logs.

## Logs {#logs}

A aba **Logs** mostra as linhas mais recentes de:

- **Log de acesso (Nginx)** e **Log de erros (Nginx)** (com o nome do seu servidor web)
- **Erros do PHP**, para os sites PHP (`/var/www/example.com/logs/php-error.log`)
- **Logs do app**, para os sites de app e Docker

## Monitoramento {#monitoring}

A aba **Monitoramento** confere a cada poucos minutos se o site responde, e envia um alerta quando ele não responde. Ative **Monitorar este site**; a primeira verificação roda na hora.

O painel pede o endereço público do site, com `https://` quando ele tem um certificado (caso contrário, `http://`), pelo DNS, como qualquer visitante. Assim, um registro DNS errado, uma página de erro do Cloudflare ou um certificado expirado também aparecem. Os redirecionamentos para os próprios domínios do site (como de `http://` para `https://`, ou para `www.`) são seguidos. Um redirecionamento para outro domínio conta como uma resposta.

| Configuração | O que faz | Padrão |
|---|---|---|
| **Verificação** | com que frequência: a cada minuto, ou a cada 2, 5 ou 10 minutos | a cada 5 minutos |
| **Caminho** | a página a pedir, como `/health`; uma query string é permitida | `/` |
| **Texto que a página deve conter** | opcional; maiúsculas e minúsculas não importam | vazio |
| **Contar respostas 4xx, como 404, como fora do ar** | uma resposta 4xx faz a verificação falhar | desativado |

Uma verificação falha quando:

- nenhuma resposta chega em 10 segundos, o domínio não resolve, a conexão é recusada, ou o certificado expirou, não é confiável ou é de outro domínio
- a resposta é um erro HTTP 500 ou maior. Uma resposta 4xx conta como no ar, a menos que você conte as respostas 4xx como fora do ar
- o texto que a página deve conter não está nos primeiros 256 KB da página

O site fica **Fora do ar** depois de 2 verificações malsucedidas seguidas, e volta ao ar depois de uma verificação bem-sucedida. O painel então envia um alerta para cada canal em **Configurações** → **Alertas** (veja [Alertas](./mail-alerts#alerts)):

- quando ele cai, com o motivo, como "example.com is down (HTTP 502, 2 checks in a row)" (example.com está fora do ar, HTTP 502, 2 verificações seguidas), e de novo a cada 6 horas enquanto continuar fora do ar
- quando ele volta ao ar, com quanto tempo ficou fora
- quando o certificado que ele serve termina em menos de 14 dias, no máximo uma vez por dia

A aba mostra o estado e a última verificação, o uptime em 24 horas e em 7 dias, o tempo médio de resposta, os tempos de resposta das últimas 24 horas em um gráfico e as últimas falhas com os motivos delas. O tempo de resposta conta até a chegada dos cabeçalhos da resposta, inclusive a conexão. Na lista de sites, um site monitorado que está fora do ar mostra **Fora do ar** em vermelho na coluna **Rodando**, e o sino o lista.

- As verificações enviam o user agent `Nefo-Monitor/1.0`. Elas não são contadas na aba **Tráfego**.
- Os resultados são guardados por 7 dias. Excluir o site os exclui.
- Um site suspenso não é verificado.
- As verificações rodam neste servidor. Elas não conseguem avisar que o servidor inteiro caiu, nem um problema na rede entre os seus visitantes e o servidor: para isso, acrescente um serviço de monitoramento externo.

## Firewall web, verificação de malware, bloqueio por país, bloqueio de arquivos e backups {#web-firewall-malware-scan-country-blocking-file-lock-and-backups}

- **Firewall web (WAF)**: o ModSecurity com o OWASP Core Rule Set para este site: **Desativado**, **Só detectar: registrar os ataques, não bloquear nada** ou **Bloquear ataques**. Instale antes o firewall na página **Software**. Veja [Segurança](./security#web-application-firewall), que também trata da aba **Verificação de malware**.
- **Bloqueio por país**: recuse os visitantes de alguns países, ou deixe entrar só alguns. **Bloqueio de arquivos**: deixe o código do site somente leitura para o PHP e a conta SFTP dele, exceto as pastas em que ele precisa gravar. Veja [Bloqueio por país e de arquivos](./website-protection).
- **Backups**: **Fazer backup agora**, ou agende um backup **Todo dia às 03:00**. Veja [Backups](./backups).

## Clonar um site {#clone-a-website}

**Clonar** copia um site para um novo domínio: para começar um novo site a partir de um existente, ou para testar alterações em uma cópia. Funciona com sites PHP, estáticos e de proxy reverso, sites de app (Node.js, Python, Java) e apps Docker da [loja de aplicativos](./apps).

1. Abra a aba **Visão geral** do site e clique em **Clonar…**, em **Clonar este site**.
2. Digite o **Domínio da cópia**.
3. Deixe **Copiar os bancos de dados vinculados a este site** marcado para copiá-los também. A caixa não aparece para o WordPress, cujo banco de dados é sempre copiado, nem para os apps Docker, cujos dados são sempre copiados.
4. Clique em **Clonar**. O site continua rodando durante a cópia, que roda como uma tarefa com um log. Um app Docker para por um instante enquanto os dados dele são copiados.
5. Aponte o DNS do novo domínio para este servidor e depois ative o HTTPS na aba **SSL / HTTPS** dele. A cópia começa em `http://`.

### Sites PHP, estáticos e de proxy reverso {#php-static-and-reverse-proxy-websites}

A cópia recebe:

- um novo site do mesmo tipo (a mesma versão do PHP, estático, ou um proxy reverso para o mesmo endereço com as mesmas opções), com o seu próprio usuário Linux, o mesmo app, as configurações do PHP, as regras de reescrita e as diretivas próprias. Nas suas diretivas, a pasta do site antigo é substituída pela nova;
- uma cópia da pasta do site, `/var/www/example.com`, pertencente ao novo usuário, sem `logs`, `tmp` e a chave de deploy do Git (`.ssh`). Os links simbólicos são copiados como links; um link para um lugar dentro da pasta do site antigo passa a levar ao mesmo lugar no novo. No `public/.user.ini`, a pasta antiga é substituída pela nova;
- para cada banco de dados vinculado, MariaDB, MySQL ou PostgreSQL, um novo banco de dados e um novo usuário do mesmo tipo, com uma cópia dos dados, vinculados ao novo site;
- os programas em segundo plano do site, parados (veja abaixo).

Quando a cópia fica pronta, o Nefo mostra os dados de conexão dos novos bancos de dados MariaDB e MySQL (os PostgreSQL ficam na página **Bancos de dados**). Os próprios arquivos de configuração do app (como o `.env` ou o `config.php`) ainda citam o banco de dados e o usuário antigos: troque-os pelos novos. Os arquivos que citam a pasta antiga, como um cache das configurações do app, também continuam citando.

### Sites de app {#app-websites}

A cópia de um site de app recebe, além disso:

- uma porta própria, escolhida pelo Nefo, que o app recebe em `PORT`. Um app que define a sua própria porta precisa ser alterado para usar a nova;
- o mesmo **Runtime** e o mesmo **Comando de início (opcional)**; no comando, a pasta do site antigo é substituída pela nova. O serviço `panel-app-<user>.service` da cópia inicia assim que os arquivos e os bancos de dados são copiados;
- o `node_modules` com os links dele (`node_modules/.bin`), e um ambiente virtual Python `app/.venv` que funciona na nova pasta: os caminhos nos scripts dele, no `pyvenv.cfg` e nos arquivos `.pth` citam a pasta da cópia. As dependências não são instaladas de novo;
- os programas em segundo plano do site, com o mesmo comando, a mesma pasta e as mesmas variáveis (com os caminhos da pasta antiga substituídos), mas **parados**: até a cópia usar bancos de dados e filas próprios, eles trabalhariam nos do original. Inicie-os na página **Programas em segundo plano**.

O runtime ainda precisa estar instalado: uma versão do Node.js, o Python ou o Java removidos da página **Software** fazem a cópia ser recusada antes de começar.

### Apps Docker {#docker-apps}

A cópia do site de um app Docker é um novo site com uma nova instalação do app:

- a mesma versão e as mesmas imagens do original (também quando a loja de aplicativos tem uma mais nova), com os seus próprios contêineres e portas, e os mesmos limites de memória;
- uma cópia dos dados do app de `/var/lib/panel-apps/<name>`, feita com o original parado: o Nefo o para, copia os dados e o inicia de novo na hora. Os arquivos do usuário do site original passam a pertencer ao usuário do novo site na cópia;
- as mesmas contas e senhas do original (os dados copiados foram feitos com elas), mostradas em **Loja de aplicativos** → **Instalados**, como para qualquer app;
- o endereço do app definido como o novo domínio, para os apps que o recebem do painel (`url`, `DOMAIN` e configurações parecidas do app). Se o app guarda o seu próprio endereço nas suas próprias configurações, mude-o lá.

O Nefo confere antes a memória e o disco de que a cópia precisa (mais ou menos o tamanho dos dados do app, mais 512 MB).

O Nefo se recusa a clonar:

| App | Por quê |
|---|---|
| n8n | A cópia rodaria os workflows ativos (agendamentos e gatilhos) uma segunda vez. Exporte os workflows e importe-os em um novo n8n. |
| Odoo | A cópia rodaria também as ações agendadas e os servidores de e-mail: ela enviaria os mesmos e-mails, e buscaria e excluiria as mensagens da mesma caixa de correio. Faça backup dele, ou instale um novo Odoo. |
| Um app instalado antes da loja de aplicativos | Atualize-o antes (**Loja de aplicativos** → **Instalados**) e depois clone-o. |

Uma cópia de um app de monitoramento, como o Uptime Kuma, verifica os mesmos sites e envia as mesmas notificações do original: pause o que você não precisa em um dos dois.

### WordPress {#wordpress}

Um site WordPress é copiado como uma [cópia de staging](./wordpress#make-a-staging-copy): o banco de dados dele é sempre copiado, o `wp-config.php` é apontado para a cópia e o endereço do site é substituído em todos os lugares, por isso a cópia funciona na hora. Os mecanismos de busca recebem o pedido de não indexá-la; se a cópia for um site próprio, mude isso no WordPress em **Settings → Reading** (Configurações → Leitura). Ao contrário de uma cópia de staging, um clone não pode ser enviado de volta para o site de onde veio.

### O que não é copiado {#not-copied}

Os aliases, o certificado, a proteção por senha, os redirecionamentos e as configurações de firewall, o bloqueio por país, o bloqueio de arquivos, as tarefas cron, as configurações de deploy pelo Git, o acesso SFTP, o monitoramento e os agendamentos de backup.

Se um passo falhar, o Nefo remove tudo o que criou para a cópia: o site, o usuário Linux dele, os novos bancos de dados, os programas em segundo plano copiados e, para um app Docker, os novos contêineres e os dados deles. O original continua rodando.

## Excluir um site {#delete-a-website}

1. Abra a aba **Visão geral** do site.
2. Em **Excluir este site**, clique em **Excluir**.
3. A janela lista o que vai embora junto com o site em **Removidos junto com ele:** (o app Docker e os dados dele, a conta SFTP, as tarefas cron, os programas em segundo plano, o monitoramento, as configurações de deploy pelo Git, os agendamentos de backup).
4. Deixe **Fazer backup do site antes** marcado (o padrão), ou desmarque.
5. Marque **Excluir também os bancos de dados dele** para excluir também os bancos de dados vinculados. Desmarcado (o padrão), eles ficam na página **Bancos de dados**, sem ligação com nenhum site.
6. Digite o domínio para confirmar e clique em **Fazer backup e excluir** (ou **Excluir**, sem o backup).

### O backup feito antes {#the-backup-made-first}

Com **Fazer backup do site antes**, o Nefo primeiro faz um backup normal neste servidor: os arquivos do site e os bancos de dados MariaDB, MySQL e PostgreSQL dele e, para um app Docker, os dados dele como um segundo backup do tipo **App**. O site só é excluído se o backup der certo. Se ele falhar, a tarefa para com "the backup failed, so example.com was not deleted" (o backup falhou, por isso example.com não foi excluído) e nada é removido.

- O backup se chama `example.com_before-delete_<date>-<time>.tar.gz` (e `<user>_before-delete_….tar.gz` para os dados de um app Docker), em `/var/backups/panel/site/example.com/`.
- Como todo backup que você faz manualmente, ele é mantido até você excluí-lo na página **Backups**. Os agendamentos só excluem os seus próprios backups, e os agendamentos do site são removidos junto com ele, por isso nada o exclui.
- Ele fica só neste servidor: não é enviado para o armazenamento remoto. Envie-o pela página **Backups** se quiser uma cópia remota.
- A exclusão roda como uma tarefa com um log. O log dela cita o backup.

::: warning
Sem o backup, os arquivos do site (e os bancos de dados dele, se você marcar **Excluir também os bancos de dados dele**) não podem ser recuperados. A janela avisa isso quando você desmarca **Fazer backup do site antes**.
:::

### Posso trazer de volta um site excluído? {#bring-a-deleted-website-back}

Sim, com o backup feito antes da exclusão (ou com qualquer outro backup do site). Na página **Backups**, restaure o backup `…_before-delete_…` e digite o domínio para confirmar. A restauração cria o site de novo com o domínio, os aliases, a versão do PHP, as regras de reescrita, os arquivos e os bancos de dados. Um banco de dados que foi excluído volta com a senha que tinha, que os arquivos restaurados citam; um banco de dados que você manteve é ligado de novo ao site.

O que não é criado de novo: o certificado (obtenha um novo na aba **SSL / HTTPS**), as tarefas cron, o acesso SFTP, os programas em segundo plano, os redirecionamentos e a proteção por senha. Veja [Restaurar em um novo servidor](./restore-new-server#restoring-a-website-that-doesn-t-exist-here) para o que a restauração cria.

Os backups feitos pelo Nefo 0.24.1 ou posterior registram o tipo do site, por isso um site de app (Node.js, Python, Java) volta com o runtime, o comando de início, a porta e o limite de memória, e um site de proxy reverso, com o endereço e as opções; instale antes o runtime do app na página **Software**. Os backups feitos por versões mais antigas só registram a versão do PHP, por isso um site assim volta como um site estático: crie-o antes de novo com o mesmo domínio e o mesmo tipo e depois restaure. Para um app Docker, instale o app de novo no mesmo domínio pela **Loja de aplicativos** e depois restaure o backup dele do tipo **App**.

### O que o Nefo remove e o que mantém {#what-nefo-removes-and-keeps}

O Nefo remove:

- a pasta do site, `/var/www/example.com`, com todos os arquivos, inclusive um repositório Git e a chave de deploy dele
- o usuário e o grupo Linux, e o acesso SFTP
- a configuração do servidor web, o arquivo de senhas e as suas próprias diretivas
- o pool do PHP-FPM, o serviço do app ou o app Docker com os contêineres, os dados e as imagens não usadas dele
- o certificado, o cache de página e os logs do servidor web do site
- os programas em segundo plano do site, as tarefas cron que rodam com o usuário dele, os agendamentos de backup, o monitoramento e as configurações de deploy pelo Git dele
- a lixeira do site em **Arquivos** e os arquivos dele postos em quarentena pelo verificador de malware
- os bancos de dados vinculados, se você marcou **Excluir também os bancos de dados dele**

O Nefo mantém os backups que você já fez: eles ficam na página **Backups**.

Cópias de staging do WordPress: excluir um site em produção que tem uma cópia de staging mantém a cópia, que não pode mais ser enviada para um site em produção; a janela avisa isso. Excluir uma cópia de staging não muda o site em produção dela.

Enquanto um deploy pelo Git do site estiver rodando, o Nefo se recusa a excluí-lo ("Outra tarefa está rodando no servidor. Espere ela terminar."). Com o backup, a tarefa espera um deploy que começou durante o backup. Se uma etapa de limpeza falhar, o site é removido do painel mesmo assim, e o Nefo lista as etapas que falharam em "Algumas etapas de limpeza falharam:" (no log da tarefa, quando o backup foi feito antes).

Pela API: `DELETE /api/sites/{id}` com `{"confirm": "example.com", "delete_databases": false, "backup_first": true}`. Com `backup_first`, ela responde 202 com a tarefa. Sem ele, o site é excluído na hora (200, com os avisos).
