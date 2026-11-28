---
title: Certificados SSL
description: Obtenha certificados Let's Encrypt gratuitos, certificados wildcard pelo DNS do Cloudflare, ou instale os seus próprios, e corrija as falhas comuns.
---

# Certificados SSL

O Nefo obtém um certificado Let's Encrypt gratuito para cada site com um clique e o renova sozinho. Ele também pode obter certificados wildcard pelo DNS do Cloudflare, instalar um certificado que você já tem, redirecionar HTTP para HTTPS e enviar HSTS. Antes de cada pedido, ele confere se cada nome chega ao servidor, para que as tentativas malsucedidas não contem para os limites do Let's Encrypt.

Esta página trata dos requisitos, do redirecionamento para HTTPS e do HSTS, dos certificados wildcard pelo DNS do Cloudflare, da instalação de um certificado que você já tem, de como desativar o HTTPS de novo e do que fazer quando um certificado falha.

## Antes de começar {#before-you-start}

Para um certificado normal, o Let's Encrypt confere por HTTP simples se cada nome do site chega ao seu servidor. Todos os nomes precisam passar: o domínio e cada um dos seus [nomes de domínio extras](./websites#domains-and-aliases).

- **DNS:** cada nome tem um registro A que aponta para o endereço IP deste servidor, por exemplo `203.0.113.10`. Se um nome também tiver um registro AAAA (IPv6), ele precisa apontar para este servidor também, ou ser removido.
- **Porta 80:** o servidor responde na porta 80 a partir da internet. O Nefo abre as portas 80 e 443 no ufw quando instala o servidor web. Se o seu provedor de hospedagem tiver um firewall próprio, abra a porta 80 lá também.
- **Proxy:** se o seu DNS estiver no Cloudflare com o proxy ativado (nuvem laranja), leia antes [Proxy do Cloudflare](#cloudflare-proxy-orange-cloud).

Ao ativar o HTTPS, você aceita o [Subscriber Agreement](https://letsencrypt.org/repository/) (contrato de assinante) do Let's Encrypt.

## Obter um certificado gratuito {#get-a-free-certificate}

1. Abra **Sites** e depois a aba **SSL / HTTPS** do site. Na lista de sites, **Ativar HTTPS**, na coluna **SSL**, abre a mesma aba.
2. Em **Certificado gratuito do Let's Encrypt**, deixe **Validar pelo DNS (Cloudflare)** desmarcado.
3. Clique em **Obter certificado**.

Um log mostra as etapas: o Nefo confere se cada nome chega a este servidor, pede o certificado ao Let's Encrypt, valida cada nome e ativa o HTTPS no servidor web. Ele termina com "Done. The certificate is valid until" (pronto, o certificado é válido até) e a data.

Quando um site recebe o seu primeiro certificado, o Nefo também passa um site WordPress que ele instalou para o endereço `https://` dele, e reinicia um app Docker com o endereço `https://` dele. Os arquivos ficam em `/var/lib/nefo/certs/example.com/`, como `fullchain.pem` e `privkey.pem`.

### A verificação antes do pedido {#the-check-before-the-request}

Antes de pedir ao Let's Encrypt, o Nefo coloca um arquivo de teste no servidor e o busca por cada nome via HTTP. Se um nome falhar, o Nefo nem envia o pedido, para que as tentativas malsucedidas não contem para os limites do Let's Encrypt. O log então diz "these names do not reach this server yet, so no certificate was requested:" (estes nomes ainda não chegam a este servidor, por isso nenhum certificado foi pedido), com uma linha por nome:

| Mensagem | O que corrigir |
|---|---|
| `blog.example.com: no DNS record. Add an A record that points to this server's IP address.` | Acrescente o registro A e espere até ele ficar visível. |
| `example.com (203.0.113.10): no answer over HTTP on port 80 (…). Check the DNS record and the firewall.` | Abra a porta 80 no firewall do seu provedor, e confira se o servidor web está rodando. |
| `example.com (198.51.100.7) answered HTTP 404, but not from this server. Check that the DNS record points here.` | O nome aponta para outro servidor: um endereço IP antigo, um registro AAAA em outro lugar, ou um proxy como o do Cloudflare. |

Os endereços entre parênteses são aqueles para os quais o nome resolve. Compare-os com o endereço IP do seu servidor.

## Redirecionar HTTP para HTTPS {#redirect-http-to-https}

Quando o site tiver um certificado, ative **Redirecionar HTTP para HTTPS** na aba **SSL / HTTPS** ou na aba **Visão geral**. Todo endereço `http://` passa a responder com um redirecionamento permanente (301) para `https://`. A validação do Let's Encrypt e os webhooks do Git continuam funcionando por HTTP.

Para que os navegadores usem HTTPS antes mesmo do redirecionamento, ative também o [HSTS](#hsts).

## HSTS {#hsts}

O HSTS (o cabeçalho `Strict-Transport-Security`) diz aos navegadores para abrir o site só por `https://`, mesmo quando alguém digita `http://` ou segue um link antigo. Ative **HSTS (Strict-Transport-Security)** na aba **SSL / HTTPS**. Ele precisa de um certificado e de **Redirecionar HTTP para HTTPS**; desativar o redirecionamento desativa o HSTS também.

O Nefo então envia `Strict-Transport-Security: max-age=15552000` (180 dias, cerca de 6 meses) com as respostas HTTPS do site, no Nginx, no Apache e no OpenLiteSpeed. Marque **Incluir subdomínios** para acrescentar `includeSubDomains`: os navegadores passam a usar HTTPS também para todos os subdomínios do domínio, por isso marque só quando todos eles tiverem HTTPS. O Nefo nunca acrescenta `preload`: é difícil sair da lista de preload dos navegadores, por isso envie um domínio para ela você mesmo, se for o caso.

::: warning
Um navegador que recebeu o cabeçalho recusa HTTP simples para o site por 6 meses depois da última visita, mesmo que você desative o HSTS ou o HTTPS depois. Ative o HSTS só em um site que vai continuar com HTTPS.
:::

No Nginx, o cabeçalho é definido para o bloco `server` inteiro do site. Se as suas próprias diretivas (veja [Sites](./websites#rewrite-rules-and-your-own-directives)) acrescentarem um `location` com um `add_header` próprio, o Nginx descarta os cabeçalhos do site nesse location: repita `add_header Strict-Transport-Security "max-age=15552000" always;` ali.

Para conferir, rode `curl -sI https://example.com/ | grep -i strict-transport`.

## Renovação automática {#automatic-renewal}

O Nefo confere os certificados quando inicia e depois a cada hora. Ele renova um certificado Let's Encrypt quando resta um terço da validade dele: 30 dias antes do fim, para um certificado de 90 dias. Você não precisa fazer nada, e um site suspenso continua sendo renovado.

Se uma renovação falhar, o Nefo tenta de novo a cada 12 horas. Enquanto isso:

- a lista de sites mostra "A renovação falhou" na coluna **SSL**;
- a aba **SSL / HTTPS** mostra "A última tentativa de obter um certificado falhou." com o erro;
- um alerta é enviado, se você configurou alertas para as renovações que falham: veja [E-mail e alertas](./mail-alerts).

O próprio certificado do painel, de **Configurações** → **Certificado** (veja [Segurança](./security#https-from-the-first-start)), é renovado do mesmo jeito: um terço da validade antes do fim, e de novo a cada 12 horas depois de uma falha. Uma renovação que falha fica registrada no log de atividades e envia o mesmo alerta.

O Nefo registra a sua conta no Let's Encrypt sem endereço de e-mail, por isso o Let's Encrypt não escreve para você sobre certificados que vão expirar. Conte com os alertas do painel.

Para renovar manualmente, por exemplo depois de corrigir um problema, clique em **Renovar certificado** na aba **SSL / HTTPS**.

## Certificados wildcard pelo DNS do Cloudflare {#wildcard-certificates-through-cloudflare-dns}

Um nome wildcard como `*.example.com` só pode ser validado pelo DNS. O Nefo faz isso pela API do Cloudflare, por isso o DNS do domínio precisa estar hospedado no Cloudflare. A validação por DNS também funciona para um site que ainda não está acessível por HTTP, por exemplo atrás do proxy do Cloudflare. Para um certificado, o Nefo só acrescenta e remove registros TXT `_acme-challenge`.

### Criar um token de API do Cloudflare {#create-a-cloudflare-api-token}

O mesmo token também serve para a [página DNS](./dns). As permissões dele estão listadas em um só lugar, em [O token de API](./dns#the-api-token): os certificados só precisam de Zone → DNS → Edit; a página DNS também pode usar Zone → Zone → Read e Zone → Cache Purge → Purge.

1. No painel do Cloudflare, abra **My Profile → API Tokens → Create Token** (meu perfil → tokens de API → criar token).
2. Use o modelo **Edit zone DNS**. Ele dá a permissão Zone → DNS → Edit.
3. Nos recursos de zona, inclua as zonas (domínios) que você usa neste servidor.
4. Crie o token e copie-o.

Veja o guia do Cloudflare para [criar um token de API](https://developers.cloudflare.com/fundamentals/api/get-started/create-token/).

### Salvar o token no Nefo {#save-the-token-in-nefo}

1. Abra **Configurações** e depois a aba **Cloudflare**.
2. Cole o token em **Token de API** e clique em **Verificar e salvar**.

O Nefo confere se o Cloudflare aceita o token e se ele está ativo, e depois mostra "Token salvo" com os últimos caracteres do token e as **Zonas** que ele pode usar. Use **Substituir o token de API** para trocá-lo, ou **Remover** para excluí-lo.

### Obter o certificado {#get-the-certificate}

1. Para um wildcard, abra a aba **Domínios** do site, acrescente `*.example.com` em **Nomes de domínio extras** e clique em **Salvar**.
2. Na aba **SSL / HTTPS**, marque **Validar pelo DNS (Cloudflare)**. Para um site com um nome wildcard, a opção vem marcada e não pode ser mudada: "Obrigatório: este site tem um nome wildcard (*.domínio)."
3. Clique em **Obter certificado**.

O Nefo acrescenta o registro TXT `_acme-challenge.example.com`, espera até 3 minutos até que os servidores de nomes do Cloudflare respondam com ele, valida e depois exclui o registro. O site lembra a escolha, por isso as renovações dele também usam a validação por DNS.

O certificado cobre os nomes; os visitantes ainda precisam de registros DNS que apontem os subdomínios para este servidor.

## Usar o seu próprio certificado {#use-your-own-certificate}

Instale o seu próprio certificado quando você comprou um de outro provedor, ou para um certificado de origem do Cloudflare.

1. Abra a aba **SSL / HTTPS** do site e vá até **Seu próprio certificado**.
2. Cole o **Certificado (PEM, com os intermediários)**: o certificado seguido da cadeia dele. Você também pode escolher um arquivo `.pem`, `.crt`, `.cer` ou `.txt`.
3. Cole a **Chave privada (PEM, sem criptografia)**, ou escolha um arquivo `.pem`, `.key` ou `.txt`.
4. Clique em **Instalar**.

O Nefo confere os arquivos antes de alterar qualquer coisa, e recusa:

- um arquivo que não é um certificado PEM, ou com mais de 64 KB;
- uma chave privada protegida por uma frase secreta. Remova-a antes, por exemplo com `openssl pkey -in key.pem -out plain.pem`;
- uma chave que não pertence ao certificado;
- um certificado que ainda não é válido, ou que expirou;
- um certificado que não cobre todos os nomes do site. Um nome wildcard do site precisa estar listado no certificado exatamente igual.

Ele instala o certificado com um aviso ("Instalado, com avisos:") quando os navegadores não vão confiar nele por conta própria, o que é esperado para um certificado de origem do Cloudflare usado atrás do proxy do Cloudflare, ou quando ele expira em até duas semanas.

O Nefo não renova o seu próprio certificado. Nas duas últimas semanas dele, a aba **SSL / HTTPS** diz "O Nefo não renova um certificado que você instalou. Instale um novo antes de" e a data, e **Precisa de atenção** no dashboard lista o site. O Nefo também lembra você uma vez por dia: o log de atividades registra isso como uma renovação que falhou, e o alerta das renovações que falham é enviado. Instale um novo certificado a tempo, ou volte para o Let's Encrypt com **Obter certificado**. A lista de sites marca esses sites com "cert. próprio".

## Desativar o HTTPS {#turn-off-https}

Para levar um site de volta para HTTP simples, clique em **Desativar HTTPS** no final da aba **SSL / HTTPS** dele e confirme. O Nefo então:

- serve o site só por `http://`: a parte HTTPS da configuração do servidor web dele é removida;
- desativa **Redirecionar HTTP para HTTPS** e o [HSTS](#hsts);
- exclui os arquivos do certificado em `/var/lib/nefo/certs/example.com/`, do Let's Encrypt ou os seus, para que nada os renove nem lembre você deles. A conta no Let's Encrypt fica, para os outros sites;
- passa um site WordPress que ele instalou de volta para o endereço `http://` dele, e reinicia um app Docker com o endereço `http://` dele.

Os visitantes que abrirem um endereço `https://` do site (de um favorito, de um resultado de busca ou de um link) passam a receber um erro do navegador, porque o servidor não tem mais um certificado para o nome. Se o site tinha HSTS, os navegadores que receberam o cabeçalho continuam recusando HTTP simples por até 6 meses, então para eles o site não abre de jeito nenhum até ter HTTPS de novo. Você pode obter um novo certificado a qualquer momento com **Obter certificado**.

Para substituir um certificado, você não precisa desativar o HTTPS: obtenha um novo certificado Let's Encrypt ou instale o seu próprio por cima. Excluir um site exclui o certificado dele também: veja [Sites](./websites). Desinstalar o servidor web mantém os certificados.

## Quando um certificado falha {#when-a-certificate-fails}

O erro aparece na aba **SSL / HTTPS** do site, em "A última tentativa de obter um certificado falhou.", e no log da tentativa.

### DNS ou porta 80 {#dns-or-port-80}

Veja [a verificação antes do pedido](#the-check-before-the-request). A maioria das falhas é um registro DNS que ainda não aponta para este servidor, ou a porta 80 fechada em um firewall do provedor.

### Proxy do Cloudflare (nuvem laranja) {#cloudflare-proxy-orange-cloud}

Com o proxy do Cloudflare ativado, o domínio resolve para os endereços do Cloudflare, e as requisições chegam ao Cloudflare antes do seu servidor. Configurações do Cloudflare como **Always Use HTTPS** (sempre usar HTTPS, um redirecionamento para HTTPS, em **SSL/TLS → Edge Certificates**) podem então impedir que a verificação chegue ao servidor sem alterações, e o Nefo informa que o nome "answered HTTP …, but not from this server" (respondeu HTTP …, mas não a partir deste servidor). Veja também [Let's Encrypt atrás da nuvem laranja](./dns#let-s-encrypt-behind-the-orange-cloud). Você tem três saídas:

- marcar **Validar pelo DNS (Cloudflare)** e obter o certificado pelo DNS. O proxy pode continuar ativado, e as renovações funcionam do mesmo jeito;
- instalar um certificado de origem do Cloudflare como o [seu próprio certificado](#use-your-own-certificate);
- desativar o proxy (somente DNS, nuvem cinza) para o nome. As renovações também conferem por HTTP, por isso deixe-o desativado.

### Registros CAA {#caa-records}

Se o seu domínio tem registros CAA no DNS, um deles precisa permitir `letsencrypt.org`. Caso contrário, o Let's Encrypt recusa o certificado, e o log mostra o erro dele, que menciona CAA. Veja [Let's Encrypt e CAA](https://letsencrypt.org/docs/caa/).

### Limites de taxa {#rate-limits}

O Let's Encrypt limita quantos certificados e quantas validações malsucedidas um domínio e uma conta podem ter. Quando você atinge um limite, o log mostra a mensagem do Let's Encrypt com o tempo de espera. Veja [os limites de taxa do Let's Encrypt](https://letsencrypt.org/docs/rate-limits/).

::: tip
Para testar sem gastar os limites, acrescente `"acme_directory": "https://acme-staging-v02.api.letsencrypt.org/directory"` ao `/etc/nefo/config.json` e reinicie o painel com `systemctl restart nefo`. Os navegadores não confiam nos certificados de staging. Remova a linha e reinicie de novo para obter certificados de verdade.
:::

### Validação por DNS pelo Cloudflare {#dns-validation-through-cloudflare}

| Mensagem | O que corrigir |
|---|---|
| `DNS validation needs a Cloudflare API token: add one in Settings → Cloudflare` | Salve um token na página **Configurações**, aba **Cloudflare**. |
| `Cloudflare did not accept this token.` | O token está errado, expirou ou não está ativo. Crie um novo. |
| `none of the Cloudflare zones this API token can use holds example.com` | Acrescente a zona do domínio aos recursos de zona do token no Cloudflare. |
| `the DNS record _acme-challenge.example.com did not appear on … in time` | Os servidores de nomes do Cloudflare demoraram. Tente de novo alguns minutos depois. |
