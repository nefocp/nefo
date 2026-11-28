---
title: Loja de aplicativos
description: Instale apps PHP em um site, apps Docker atrás de um site próprio e serviços de banco de dados, e gerencie, atualize, faça backup e remova esses apps.
---

# Loja de aplicativos

A loja de aplicativos do Nefo instala 42 aplicações com um clique: apps PHP como WordPress, WooCommerce, Joomla, Nextcloud e Moodle em um site, apps Docker como Ghost, n8n, Gitea, Grafana e Jellyfin atrás de um site próprio, e PostgreSQL, MongoDB e Ollama como serviços em 127.0.0.1. Ela confere antes a memória e o disco do servidor.

O Nefo cria o que cada app precisa, como um banco de dados, um site, senhas e contêineres. Esta página trata dos tipos de app, do catálogo, da instalação e do gerenciamento dos apps que você instalou.

Abra **Loja de aplicativos** no menu à esquerda. A aba **Catálogo** lista os apps, e a aba **Instalados**, os apps deste servidor. Na página **Sites**, **Instalar pela loja de aplicativos** também leva até aqui.

## Três tipos de app {#three-kinds-of-apps}

| Tipo | Para onde vai | O que recebe |
|---|---|---|
| **App PHP** | Para um site PHP vazio que você criou | O seu próprio banco de dados, os seus arquivos no site, as regras de reescrita certas. Roda com o usuário Linux do site. |
| **App Docker** | Para um novo site que o Nefo cria para ele | Contêineres que escutam só em `127.0.0.1`, com o site na frente deles. |
| **Serviço** | Nenhum site | Contêineres que escutam só em `127.0.0.1`, para os seus próprios sites e apps. Um de cada por servidor. |

Os apps PHP e os apps Docker precisam do servidor web instalado: veja [Sites](./websites). Os apps Docker e os serviços também precisam do Docker. Instale-o na página **Software**; **Instalar Docker** na loja de aplicativos abre essa página. Veja [Docker](./docker).

## Quais apps posso instalar? {#the-catalog}

Na versão de catálogo 2026112701, a loja de aplicativos oferece 42 apps. Eles ficam agrupados em categorias: **Todos** mostra cada categoria com os seus apps, e o botão de uma categoria mostra só os apps dela. A pesquisa olha os nomes, as descrições e os nomes das categorias, e ignora os acentos (no painel em vietnamita, digitar "giam sat" encontra os apps de "Giám sát & thống kê").

A memória mínima é a RAM total do servidor abaixo da qual o Nefo recusa a instalação (o swap não conta).

| App | Tipo | Categoria | O que é | RAM mínima |
|---|---|---|---|---|
| WordPress | App PHP | Sites e conteúdo | Criador de sites e blogs | 450 MB |
| Joomla | App PHP | Sites e conteúdo | CMS para sites e portais | 450 MB |
| Drupal | App PHP | Sites e conteúdo | CMS para conteúdo estruturado, montado com o Composer | 900 MB |
| Ghost | App Docker | Sites e conteúdo | Blogs e newsletters | 900 MB |
| Directus | App Docker | Sites e conteúdo | CMS headless | 900 MB |
| Wiki.js | App Docker | Sites e conteúdo | Wiki | 900 MB |
| WooCommerce | App PHP | Lojas e negócios | WordPress com WooCommerce: uma loja virtual | 900 MB |
| OpenCart | App PHP | Lojas e negócios | Carrinho de compras | 900 MB |
| Odoo | App Docker | Lojas e negócios | ERP (edição Community) | 1800 MB |
| Stirling-PDF | App Docker | Produtividade | Ferramentas de PDF | 1800 MB |
| Memos | App Docker | Produtividade | Notas rápidas em Markdown em uma linha do tempo | 230 MB |
| linkding | App Docker | Produtividade | Gerenciador de favoritos | 450 MB |
| Vikunja | App Docker | Produtividade | Listas de tarefas, quadros Kanban e projetos | 450 MB |
| Actual Budget | App Docker | Produtividade | Orçamento pessoal | 450 MB |
| Kanboard | App Docker | Produtividade | Quadro Kanban para projetos | 230 MB |
| FreshRSS | App Docker | Produtividade | Leitor de notícias e blogs (RSS) | 450 MB |
| Mealie | App Docker | Produtividade | Receitas, planos de refeições e listas de compras | 900 MB |
| Nextcloud | App PHP | Arquivos e armazenamento | A sua própria nuvem para arquivos, agendas e contatos | 900 MB |
| RustFS | App Docker | Arquivos e armazenamento | Armazenamento de objetos compatível com S3 | 900 MB |
| FileBrowser Quantum | App Docker | Arquivos e armazenamento | Arquivos no navegador | 450 MB |
| Navidrome | App Docker | Mídia | Streaming de músicas da sua própria coleção | 450 MB |
| Jellyfin | App Docker | Mídia | Filmes, séries e músicas do seu servidor | 1800 MB |
| ntfy | App Docker | Comunicação | Notificações push por HTTP | 230 MB |
| Gotify | App Docker | Comunicação | Servidor de notificações com um app Android | 230 MB |
| listmonk | App Docker | Comunicação | Newsletters e listas de e-mail | 450 MB |
| Uptime Kuma | App Docker | Monitoramento e análise | Vigia os seus sites e serviços | 450 MB |
| Umami | App Docker | Monitoramento e análise | Análise de sites que respeita a privacidade | 900 MB |
| Grafana | App Docker | Monitoramento e análise | Dashboards e alertas | 900 MB |
| changedetection.io | App Docker | Monitoramento e análise | Vigia páginas web em busca de mudanças | 900 MB |
| Healthchecks | App Docker | Monitoramento e análise | Alertas quando tarefas cron param de rodar | 450 MB |
| Vaultwarden | App Docker | Segurança e rede | Servidor de gerenciador de senhas para os apps do Bitwarden | 450 MB |
| AdGuard Home | App Docker | Segurança e rede | Bloqueio de anúncios pelo seu próprio endereço DNS-over-HTTPS | 450 MB |
| Open WebUI | App Docker | IA | Interface de chat para o Ollama ou APIs compatíveis com a OpenAI | 1800 MB |
| Ollama | Serviço | IA | Roda modelos de linguagem abertos na CPU | 3584 MB |
| Laravel | App PHP | Desenvolvimento e automação | Um novo projeto Laravel com o seu banco de dados | 450 MB |
| code-server | App Docker | Desenvolvimento e automação | VS Code no navegador | 900 MB |
| IT-Tools | App Docker | Desenvolvimento e automação | Ferramentas de desenvolvimento no navegador | 230 MB |
| Gitea | App Docker | Desenvolvimento e automação | O seu próprio servidor Git | 450 MB |
| n8n | App Docker | Desenvolvimento e automação | Automação de fluxos de trabalho | 900 MB |
| PostgreSQL | Serviço | Bancos de dados | Servidor de banco de dados PostgreSQL | 900 MB |
| MongoDB | Serviço | Bancos de dados | Banco de dados de documentos MongoDB | 900 MB |
| Moodle | App PHP | Educação | Plataforma de aprendizagem | 900 MB |

O catálogo vem embutido no painel. O Nefo também procura uma vez por dia um catálogo mais novo publicado com as versões, e só o usa se a assinatura dele for válida. **Atualizar o catálogo** procura agora. O cabeçalho mostra a versão do catálogo e se ele está "embutido no painel" ou foi "baixado, assinatura verificada".

Um catálogo baixado pode conter apps que precisam de uma versão mais nova do Nefo, porque a receita deles usa algo que esta versão não tem. O Nefo deixa esses apps de fora e mantém o resto do catálogo. Se ele já conhecia o app, mantém a versão que conhece, para que as cópias instaladas continuem funcionando. O cabeçalho então diz, por exemplo, "2 apps do catálogo baixado precisam de um painel mais novo". Atualize o Nefo para recebê-los: veja [Atualização](./update). Os apps que um catálogo mais novo acrescenta mostram os nomes e as descrições em todos os idiomas que o catálogo tem.

Cada cartão mostra a versão, o tipo e as necessidades de memória e de disco do app. O botão **Instalar** dele abre a descrição, a licença, as necessidades e as notas, e o formulário de instalação. Um cartão que o servidor não consegue rodar mostra o motivo, e **Detalhes** no lugar. Um serviço instalado mostra **Gerenciar**. Um selo "Código-fonte disponível" marca uma licença que não é aprovada pela OSI.

## As verificações antes de uma instalação {#the-checks-before-an-install}

Quando você abre um app, o Nefo compara as necessidades dele com o servidor. Ele recusa a instalação quando:

- a memória total do servidor está abaixo do mínimo do app (em uma VPS que é um contêiner, conta o limite de memória dele);
- o espaço livre em disco é menor que as imagens e os dados do app mais 1 GB;
- a arquitetura ou os recursos do processador não servem para o app.

Ele avisa, mas deixa você instalar, quando o servidor tem menos memória do que o recomendado, quando há menos memória livre neste momento do que o app pode usar, ou quando os limites de memória de todos os apps instalados juntos passam da memória do servidor.

## Instalar um app PHP {#install-a-php-app}

1. Crie um site PHP vazio com uma versão do PHP que o app aceite: veja [Sites](./websites). As versões mínimas do PHP são: WordPress 7.4; Nextcloud e Laravel 8.2; WooCommerce, Joomla, Drupal e Moodle 8.3; OpenCart 8.0. O OpenCart aceita o PHP até a 8.3, e o Moodle até a 8.4.
2. Confira se o MariaDB (ou o MySQL) está instalado e rodando: veja [Bancos de dados](./databases).
3. Na loja de aplicativos, clique no botão **Instalar** do app.
4. Escolha o **Site**. A lista mostra só os sites PHP vazios com uma versão do PHP adequada. Se não houver nenhum, use **Adicionar um site PHP**.
5. Nos apps que pedem, preencha **Nome do site**, **Administrador** (evite `admin`: é o primeiro nome que os atacantes tentam), **E-mail do administrador** e **Senha do administrador**. Deixe a senha vazia para receber uma aleatória forte; uma senha que você digita precisa ter de 12 a 128 caracteres. O WooCommerce também pede o país e a moeda da loja.
6. Clique em **Instalar**. A instalação roda em segundo plano e aparece na lista de tarefas.

Quando ela terminar, o Nefo mostra uma única vez o **Endereço de login**, o **Administrador** e a **Senha do administrador**. Salve-os agora.

A pasta `public` do site precisa estar vazia; a página de boas-vindas do painel não atrapalha. Os arquivos do site não podem estar [bloqueados](./website-protection#lock-a-website-s-files). O Drupal e o Laravel são montados com o Composer, e o Moodle também precisa da pasta inteira do site: o site deles precisa ser novo e vazio, e a pasta do site vira o projeto, com `public` como a pasta web dele.

Um app PHP recebe:

- um novo banco de dados e um novo usuário do banco de dados, ligados ao site;
- os arquivos da aplicação, baixados e conferidos com o checksum do catálogo;
- as regras de reescrita do app, e limites do PHP mais altos quando o app precisa;
- uma tarefa cron rodada com o usuário do site para o Moodle (a cada minuto) e o WooCommerce (a cada cinco minutos).

Se a instalação falhar, o Nefo esvazia o site de novo e exclui o novo banco de dados. Os sites WordPress também aparecem nas [Ferramentas do WordPress](./wordpress).

## Instalar um app Docker {#install-a-docker-app}

1. No seu DNS, aponte o domínio do app para o endereço IP deste servidor, por exemplo `app.example.com` para `203.0.113.10`.
2. Na loja de aplicativos, clique no botão **Instalar** do app.
3. Informe o **Domínio do novo site** e, se quiser, **Outros domínios**.
4. Alguns apps também pedem um **E-mail do administrador**. Use um domínio real, não `.test` nem `.local`.
5. Clique em **Instalar**.
6. Quando a instalação terminar, ative o HTTPS na aba **SSL / HTTPS** do novo site: veja [Certificados SSL](./ssl). O app reinicia com o endereço `https://` dele. Alguns apps precisam do HTTPS para funcionar por completo, e o formulário de instalação avisa.

O jeito de obter a primeira conta de administrador depende do app, e o formulário de instalação diz qual é:

| Como | Apps | O que você faz |
|---|---|---|
| Criada pelo painel | RustFS, code-server, AdGuard Home, Odoo, Umami, Grafana, ntfy, Gotify, Memos, linkding, Navidrome, Vikunja, Actual Budget, listmonk, Kanboard, FreshRSS, Jellyfin, Mealie | Abra **Credenciais** na aba **Instalados**. O usuário é `admin` (Mealie: `changeme@example.com`; o Actual Budget só tem uma senha do servidor). |
| O seu e-mail e uma senha aleatória | Open WebUI, Directus, Wiki.js, Healthchecks | Abra **Credenciais**. |
| A primeira conta registrada | Ghost, Uptime Kuma, n8n, Gitea | Registre-se logo depois da instalação: a primeira conta vira a de administrador. |
| Um convite | Vaultwarden | Cadastre-se na página do app com o endereço de e-mail que o painel convidou. |
| Uma senha de uso único | FileBrowser Quantum | Abra **Credenciais**; o app pede uma nova senha no primeiro login. |
| Uma senha do site | Stirling-PDF, changedetection.io | O site pede o usuário `admin` e a senha de **Credenciais**. |
| Sem login | IT-Tools | Nada a fazer. |

Notas sobre alguns desses apps:

- **Memos** e **Vikunja**: o painel cria a conta e depois fecha os cadastros. Você pode abri-los de novo nas configurações do app.
- **ntfy**: ninguém consegue ler nem enviar mensagens sem fazer login. A conta `admin` faz parte das configurações do app, por isso a senha dela é a que aparece em **Credenciais**.
- **Mealie**: faça login com `changeme@example.com` e a senha de **Credenciais** (o painel substituiu a senha padrão) e depois defina o seu próprio endereço de e-mail no seu perfil.
- **Actual Budget** só abre por HTTPS: obtenha antes um certificado para o site dele.
- **Healthchecks** e **listmonk** só enviam e-mail por um serviço SMTP que você configurar no app; este servidor não envia e-mail por conta própria. Os alertas do Healthchecks para ntfy, Telegram, Slack ou webhooks funcionam sem isso.
- **Navidrome** e **Jellyfin**: veja [Pastas de músicas e vídeos](#music-and-video-folders).

Um app Docker recebe:

- um novo site que encaminha para o app. O site fica fechado (503) até a instalação terminar e os logins padrão do app serem bloqueados;
- contêineres rodados como um projeto Docker Compose, com um limite de memória e os logs rotacionados a cada 10 MB;
- os seus arquivos em `/var/lib/panel-apps/<name>`, que só o root pode ler: o arquivo Compose, as senhas geradas e os dados dos contêineres;
- portas publicadas só em `127.0.0.1`, nunca na internet;
- imagens fixadas em uma versão exata.

### Pastas de músicas e vídeos {#music-and-video-folders}

O Navidrome e o Jellyfin tocam arquivos de uma pasta do site deles, para que você possa acrescentar arquivos com o [gerenciador de arquivos](./files) ou por SFTP:

| App | Pasta | Vista pelo app como |
|---|---|---|
| Navidrome | `/var/www/<domain>/music` | `/music` |
| Jellyfin | `/var/www/<domain>/media` | `/media` |

O Nefo cria a pasta na instalação, pertencente ao usuário Linux do site, e a entrega ao app somente leitura: o app pode tocar os arquivos, mas não alterá-los. O Navidrome examina a pasta quando inicia e depois a cada hora. No Jellyfin, acrescente uma biblioteca e escolha uma pasta dentro de `/media`.

Esses arquivos fazem parte do site, por isso um backup do site os inclui, e excluir o site os exclui. Converter vídeo na hora consome muita CPU em um servidor pequeno: prefira arquivos que os seus dispositivos consigam tocar como estão.

## Instalar um serviço {#install-a-service}

O PostgreSQL, o MongoDB e o Ollama não têm site. Clique em **Instalar**: não há nada a preencher. O Nefo escolhe uma porta livre em `127.0.0.1` e cria as senhas. Os apps deste servidor conseguem chegar ao serviço; a internet não. Cada serviço pode ser instalado uma vez por servidor.

Abra **Credenciais** na aba **Instalados** para ver o endereço, o usuário, a senha e a **String de conexão**.

## As verificações de segurança nos apps Docker {#the-safety-checks-on-docker-apps}

A receita de um app Docker no catálogo é dado, nunca um script. O Nefo a transforma em um arquivo Compose e confere esse arquivo com as mesmas regras dos seus próprios projetos Compose, sem exceções:

- nada de contêineres privilegiados, de capabilities acrescentadas nem de dispositivos do host;
- nada de compartilhar a rede, os processos ou outros namespaces do servidor;
- nada de opções de segurança enfraquecidas nem de configurações do kernel;
- portas publicadas só em `127.0.0.1`;
- nenhuma montagem de pastas do sistema nem do socket do Docker;
- uma pasta do site (as músicas do Navidrome, a mídia do Jellyfin) só é montada em um contêiner que roda com o próprio usuário Linux do site, e somente leitura;
- as etapas de configuração que precisam de uma senha a enviam no corpo de uma requisição ou na entrada padrão do programa, nunca em uma linha de comando.

Veja [Docker](./docker) para essas regras. As senhas são geradas para cada instalação, e as senhas iniciais são removidas das configurações do contêiner depois da primeira inicialização. Um catálogo baixado só é usado se a assinatura dele for válida.

## Gerenciar os apps instalados {#manage-installed-apps}

A aba **Instalados** lista todos os apps instalados. Filtre com **Todos**, **Com site** ou **Serviços**, ou pesquise. As colunas são **App**, **Versão**, **Status** e **Memória** (o que os contêineres usam agora).

O status é um destes: **Rodando**, **Parado**, **Rodando em parte**, **Sem contêineres**, **Instalando** ou **A instalação falhou**. Um app PHP mostra **No site dele**, ou **Site suspenso**.

O menu de cada linha tem estas ações:

| Ação | Para | O que faz |
|---|---|---|
| **Abrir** | Apps com site | Abre o site do app. |
| **Credenciais** | Apps Docker e serviços | Mostra os dados de login. Cada abertura é gravada no log de atividades. |
| **Logs** | Apps Docker e serviços | As linhas mais recentes dos logs dos contêineres. |
| **Iniciar**, **Parar**, **Reiniciar** | Apps Docker e serviços | Inicia, para ou reinicia os contêineres. |
| **Atualizar** | Apps Docker e serviços, quando o catálogo tem uma versão mais nova | Veja abaixo. |
| **Fazer backup agora** | Apps Docker e serviços | Faz backup do app. A cópia fica na página **Backups**: veja [Backups](./backups). |
| **Configurações do site** | Apps com site | Abre as configurações do site. |
| **Desinstalar** | Serviços | Remove o serviço: veja abaixo. |
| **Excluir com o site** | Apps com site | Abre o site, onde você o exclui. |

### Atualizar um app {#update-an-app}

Quando o catálogo tem uma versão mais nova de um app Docker ou de um serviço, a coluna **Versão** a mostra, por exemplo "2.6.0 disponível". Clique em **Atualizar** e confirme. O Nefo:

1. faz antes um backup do app. Se o backup falhar, nada muda;
2. baixa as novas imagens antes de parar qualquer coisa;
3. reinicia o app com elas e confere se ele responde. Se não responder, a versão anterior volta a rodar.

Alguns apps recusam uma atualização com um clique quando a versão principal muda: "Esta atualização (…) muda a versão principal e exige etapas manuais. Veja o guia de atualização do app." Isso vale para PostgreSQL, Wiki.js, Odoo, Grafana, Gotify, Vikunja, Healthchecks, listmonk, Jellyfin e Mealie, e para uma mudança de versão menor do MongoDB.

Os apps PHP não são atualizados por aqui: "Atualize-o pela própria página de administração do app." Para o WordPress, use as [Ferramentas do WordPress](./wordpress).

Um app instalado antes de o catálogo existir mostra **Instalação antiga**. **Converter e atualizar** move os dados dele para a pasta da loja de aplicativos, roda-o com o Docker Compose e o atualiza, depois de um backup.

## Desinstalar um app {#uninstall-an-app}

- **Um serviço:** clique em **Desinstalar**, digite o nome dele e confirme. Os contêineres, os dados e as senhas dele são excluídos. Os backups dele são mantidos.
- **Um app com site:** exclua o site: veja [Excluir um site](./websites#delete-a-website). Para um app Docker, isso remove os contêineres, a pasta dele em `/var/lib/panel-apps` e as imagens que nenhum outro app usa. **Fazer backup do site antes**, marcado por padrão na janela de exclusão, também faz backup dos dados do app, como um backup do tipo **App**. Para um app PHP, marque **Excluir também os bancos de dados dele** para excluir também o banco de dados dele.

::: danger
Desinstalar exclui os dados do app para sempre. Faça backup antes com **Fazer backup agora** se você talvez precise deles de novo.
:::

## Fazer uma segunda cópia de um app {#make-a-second-copy-of-an-app}

Para fazer uma segunda cópia de um app Docker com os dados dele, clone o site dele: **Sites** → (site) → **Visão geral** → **Clonar…**. Veja [Clonar um site](./websites#docker-apps). O n8n, o Odoo e os apps instalados antes da loja de aplicativos não podem ser clonados. Um app PHP é clonado com o site e o banco de dados dele, como qualquer site PHP.
