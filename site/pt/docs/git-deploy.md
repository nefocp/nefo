---
title: Deploy pelo Git
description: Faça deploy de um site a partir do GitHub, GitLab, Gitea ou qualquer servidor Git, manualmente ou a cada push, com uma chave de deploy, um webhook assinado e um script de deploy.
---

# Deploy pelo Git

O Nefo faz deploy de um site a partir de um repositório Git no GitHub, GitLab, Gitea, Forgejo ou qualquer servidor Git, por HTTPS ou SSH, com uma chave de deploy por site. Ele faz o deploy quando você clica em um botão ou a cada push, por um webhook assinado, e depois roda o seu script de deploy. Tudo roda com o próprio usuário Linux do site.

O deploy pelo Git fica na aba **Deploy pelo Git** de cada site, exceto nos sites de apps Docker. Em um site cujos [arquivos estão bloqueados](./website-protection#lock-a-website-s-files), o painel os desbloqueia para o deploy e os bloqueia de novo depois.

## Antes de começar {#before-you-start}

- **O endereço do repositório.** O Nefo aceita três formas:
  - `https://github.com/owner/repo.git` para um repositório público;
  - `git@github.com:owner/repo.git` (a forma que o GitHub mostra) para um privado;
  - `ssh://git@git.example.com:2222/owner/repo.git` para um servidor SSH em outra porta.

  O `http://` simples é recusado, e também uma senha ou um token no endereço, porque eles apareceriam nos logs. Para um repositório privado, use o endereço SSH e a chave de deploy do site.
- **A pasta.** Escolha para onde vão os arquivos:
  - **A pasta web** (`/var/www/example.com/public`): para um site que é o próprio repositório, com `index.html` ou `index.php` no topo.
  - **A pasta do site** (`/var/www/example.com`): para projetos com a sua própria pasta `public`, como Laravel ou Symfony. O site então mostra a pasta `public` do repositório.

  Um site de app Node.js, Python ou Java não tem escolha: os arquivos vão para **A pasta do app** (`/var/www/example.com/app`), onde o programa dele roda. Coloque o `package.json` ou o `requirements.txt` no topo do repositório.
- **Uma pasta vazia para o primeiro deploy.** A página de boas-vindas do painel não conta, nem as pastas `logs`, `tmp` e `.ssh` na pasta do site, nem a pasta `.venv` de um app Python na pasta do app. Tire os outros arquivos de lá antes, ou faça o deploy na outra pasta.
- **O Git** não precisa estar instalado: o Nefo o instala no primeiro deploy, se ele faltar.

## Configurar o repositório {#set-up-the-repository}

1. Abra **Sites** e depois a aba **Deploy pelo Git** do site.
2. Informe o **Endereço do repositório**.
3. Informe a **Branch**, por exemplo `main`.
4. Em **Colocar os arquivos em**, escolha **A pasta web** ou **A pasta do site**. Um site de app mostra **A pasta do app**, sem nada para escolher.
5. Opcional: escreva um **Script de deploy (opcional)** (veja [O script de deploy](#the-deploy-script)).
6. Clique em **Salvar**.

Na primeira vez, o Nefo cria a chave de deploy e o segredo do webhook do site e diz "Configurações salvas. Adicione a chave de deploy e o webhook abaixo ao repositório e depois faça o deploy."

## Adicionar a chave de deploy (repositórios privados) {#add-the-deploy-key-private-repositories}

Um repositório HTTPS público não precisa de chave. Para um repositório privado, adicione a **Chave pública** do site, mostrada em **Chave de deploy**, ao repositório como uma chave de deploy somente leitura:

- **GitHub:** repositório → Settings → Deploy keys → Add deploy key. Deixe "Allow write access" (permitir acesso de escrita) desmarcado.
- **GitLab:** projeto → Settings → Repository → Deploy keys, sem acesso de escrita.
- **Gitea e Forgejo:** repositório → Settings → Deploy keys.

Cada site tem a sua própria chave ed25519, criada com o usuário do site em `/var/www/example.com/.ssh/panel_deploy`. **Nova chave** a substitui: a chave antiga para de funcionar na hora, por isso adicione a nova ao repositório e remova a antiga de lá.

A primeira conexão SSH aceita a chave de host do servidor Git e a guarda em `/var/www/example.com/.ssh/known_hosts`. Se a chave do servidor mudar depois, os deploys falham com "The Git server's SSH host key is not the one seen before." (a chave de host SSH do servidor Git não é a que foi vista antes). Se o servidor realmente trocou a chave, exclua esse arquivo e faça o deploy de novo.

## Fazer o deploy {#deploy}

Em **Fazer deploy**, clique em **Fazer deploy agora**. Se você mudou as configurações, salve-as antes. Um log mostra cada etapa:

1. O Nefo busca a branch no repositório.
2. Os arquivos rastreados recebem a versão da branch que está no repositório.
3. O script de deploy roda, se você escreveu um.

Os arquivos não rastreados ficam, por isso os uploads e os arquivos que o repositório ignora (como o `.env`) são mantidos a cada deploy. Depois do primeiro deploy, o Nefo remove a página de boas-vindas dele quando o repositório colocou um site em `public`.

::: warning
As alterações que você fizer no servidor em arquivos que o repositório rastreia são sobrescritas no próximo deploy. Faça commit delas no repositório.
:::

Depois de um deploy, a aba mostra o status (**Deploy feito** ou **Falhou**), o horário, os 12 primeiros caracteres do commit e o assunto dele. Uma busca pode levar até 10 minutos, o script até 15 minutos e o deploy inteiro até 30 minutos.

Quando uma busca falha, o log explica os motivos comuns:

| Mensagem | O que fazer |
|---|---|
| `The repository refused the deploy key` | Adicione ao repositório a chave mostrada no painel. |
| `The repository is private or does not exist` | Confira o endereço. Para um repositório privado, use o endereço SSH dele e adicione a chave de deploy. |
| `The repository has no branch …` | Confira a **Branch**. |
| `The Git server's SSH host key is not the one seen before` | Veja a seção da chave de deploy acima. |
| `The repository's host name was not found` | Confira o endereço. |
| `It took longer than 10 minutes` | Tente de novo; confira o tamanho do repositório e a rede. |

## O script de deploy {#the-deploy-script}

O script roda depois de cada deploy, na pasta do deploy, com o usuário do site, por no máximo 15 minutos. Ele pode ter até 2000 caracteres. Por exemplo, para um site Laravel:

```sh
composer install --no-dev --optimize-autoloader
php artisan migrate --force
```

Em um site de app, o script roda na pasta `app`, por exemplo `npm ci` seguido de `npm run build`. O `node` e o `npm` dele são os da versão do Node.js escolhida para o site, e em um site Python o `python` e o `pip` são os do ambiente virtual dele (`app/.venv`). Em um site Java, `java`, `javac` e `JAVA_HOME` são os da versão do Java escolhida para ele, então `./mvnw package` ou `./gradlew build` compilam com essa versão.

- Ele roda com `sh -e`: o script para no primeiro comando que falha, e o deploy é marcado como **Falhou**. Os novos arquivos já estão no lugar nesse momento.
- `php` é a versão do PHP do site, também para o Composer.
- `DEPLOY_COMMIT` contém o commit do deploy e `DEPLOY_BRANCH`, a branch.
- `HOME` é a pasta `tmp` do site.
- O script não pode usar `sudo` nem reiniciar serviços do sistema. Para reiniciar o programa de um site de app depois de um deploy, clique em **Reiniciar** na aba **Visão geral** do site.

## Deploy a cada push {#deploy-on-every-push}

Em **Deploy a cada push**, o Nefo mostra a **URL do webhook**, como `https://example.com/.panel/deploy/12`, e o **Segredo** dela. O webhook fica no próprio domínio do site, por isso o host do repositório nunca precisa chegar à porta do painel. O servidor web o repassa a um pequeno receptor do painel em `127.0.0.1:7809`.

::: warning
Sem HTTPS, a URL do webhook usa `http://`, e o GitLab enviaria o segredo sem criptografia. Ative o HTTPS antes: veja [Certificados SSL](./ssl).
:::

Cada push para a branch escolhida faz o deploy do site. Pushes para outras branches, pushes de tags, outros eventos e exclusões de branch são ignorados.

### GitHub {#github}

1. Abra o repositório → Settings → Webhooks → Add webhook.
2. Cole a **URL do webhook** como payload URL e o **Segredo** como secret.
3. Defina o content type como `application/json`.
4. Escolha "Just the push event" (só o evento push) e adicione o webhook.

O GitHub envia um teste (ping) na hora, e o painel responde, por isso a primeira entrega aparece como bem-sucedida.

### GitLab {#gitlab}

1. Abra o projeto → Settings → Webhooks.
2. Cole a **URL do webhook** como URL e o **Segredo** como secret token.
3. Marque Push events e adicione o webhook.

### Gitea e Forgejo {#gitea-and-forgejo}

1. Abra o repositório → Settings → Webhooks → Add webhook e escolha Gitea (ou Forgejo).
2. Cole a **URL do webhook** como target URL e o **Segredo** como secret.
3. Defina o content type como `application/json`, o disparo em eventos de push, e adicione o webhook.

O Gogs funciona do mesmo jeito: escolha Gogs no passo 1.

### Como o painel confere um webhook {#how-the-panel-checks-a-webhook}

- O GitHub, o Gitea, o Forgejo e o Gogs assinam cada webhook com o segredo (HMAC-SHA256). O GitLab envia o próprio segredo. Um webhook com assinatura errada ou ausente recebe HTTP 401 e é registrado no log com o endereço de quem enviou.
- A proteção por senha e o firewall web do site não se aplicam ao caminho do webhook, então você não precisa abrir uma exceção para ele.
- O Nefo roda uma tarefa por vez. Se outra tarefa estiver rodando quando um push chegar, o deploy espera e é tentado de novo a cada 15 segundos por até uma hora. Vários pushes nesse meio-tempo precisam de um único deploy: ele busca o commit mais novo.
- **Novo segredo** substitui o segredo. Os pushes são recusados até você colar o novo segredo no webhook do repositório.

O host do repositório mostra a resposta do painel na lista de entregas recentes: `deploying`, `queued: another task is running`, `ignored: not branch main`, ou um erro como `bad_signature`.

## O log do deploy {#the-deploy-log}

- A aba **Deploy pelo Git** mostra o log do deploy atual ou do último, inclusive de um que um push iniciou.
- **Conta** → **Atividade recente** lista cada deploy como "Deploy feito pelo Git" ou "Deploy pelo Git falhou", com o site, o commit e o que o iniciou (`manual` ou um push do host do repositório).

## Parar o deploy pelo Git {#stop-deploying-from-git}

Em **Parar o deploy pelo Git**, clique em **Parar o deploy pelo Git** e confirme. O Nefo esquece as configurações e exclui a chave de deploy. Os arquivos do deploy ficam na pasta, junto com a pasta `.git` do repositório.
