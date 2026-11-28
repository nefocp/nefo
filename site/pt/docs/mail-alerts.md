---
title: E-mail e alertas
description: Deixe os seus sites enviarem e-mail por um provedor SMTP, e receba alertas por Telegram, e-mail, Discord, Slack ou um webhook quando o servidor precisa de atenção ou alguém faz login.
---

# E-mail e alertas

O Nefo não tem servidor de e-mail. Os sites enviam e-mail pelo Postfix, como um relay só de envio para o seu provedor SMTP (Gmail, SendGrid, Amazon SES e outros). Os alertas chegam até você por Telegram, e-mail, Discord, Slack ou um webhook assinado quando a CPU, a memória ou o disco estão altos, um serviço para, um backup falha, um site cai ou alguém faz login.

Esta página trata das duas coisas: enviar e-mail a partir dos sites, e os alertas. Para configurar os registros DNS de um provedor de e-mail (MX, SPF, DMARC), veja [DNS (Cloudflare)](./dns#set-up-email-records).

## Sem servidor de e-mail, de propósito {#no-mail-server-by-design}

O Nefo não hospeda caixas de correio e não recebe e-mail. Um servidor de e-mail completo custa memória e precisa de cuidado constante, o que não combina com uma VPS pequena. Mantenha as suas caixas de correio em um provedor de e-mail.

Para o e-mail de saída, o Nefo instala o Postfix como um relay só de envio. Ele escuta só neste servidor e repassa cada mensagem para o seu provedor (Gmail, SendGrid, Amazon SES e outros), com login na conta que você informar.

## Enviar e-mail a partir dos sites {#send-e-mail-from-websites}

Com o relay configurado, a função `mail()` do PHP funciona, porque o Postfix fornece o `/usr/sbin/sendmail`, que o PHP usa. Formulários de contato, redefinições de senha e avisos do WordPress passam a enviar e-mail sem nenhum plugin. Sem o relay, os sites não conseguem enviar e-mail.

### Configurar o relay {#set-up-the-relay}

1. Vá até **Configurações** → **E-mail**.
2. Escolha o seu **Provedor**. O painel preenche o **Servidor SMTP**, a **Porta** e a **Criptografia** para ele.
3. Informe o **Nome de usuário** e a **Senha** da sua conta no provedor. A dica embaixo de **Provedor** diz o que esse provedor espera.
4. Se quiser, preencha **Enviar tudo como (opcional)**, como `noreply@example.com`.
5. Na primeira vez, selecione **Instalar e salvar**: o painel instala o Postfix e o configura. Depois, o botão passa a ser **Salvar**.

A configuração roda em segundo plano, com um log ao vivo. Quando ela terminar, envie uma mensagem de teste (veja abaixo).

| Provedor | Servidor SMTP | Nome de usuário e senha |
|---|---|---|
| Gmail / Google Workspace | `smtp.gmail.com` | O seu endereço do Gmail, e uma senha de app (Conta do Google → Segurança → Verificação em duas etapas → Senhas de app), não a sua senha normal |
| Microsoft 365 / Outlook | `smtp.office365.com` | O seu endereço do Microsoft 365. O envio por SMTP precisa estar permitido para a caixa de correio |
| SendGrid | `smtp.sendgrid.net` | `apikey` (preenchido para você) e uma chave de API do SendGrid com a permissão Mail Send. Envie a partir de um remetente verificado |
| Mailgun | `smtp.mailgun.org` | As credenciais SMTP do seu domínio no Mailgun |
| Amazon SES | `email-smtp.us-east-1.amazonaws.com` | As suas credenciais SMTP do SES. Troque o servidor pelo endpoint da sua região, e envie a partir de um endereço ou domínio verificado |
| Brevo | `smtp-relay.brevo.com` | O seu login SMTP e a sua chave SMTP do Brevo |
| Outro servidor SMTP | O servidor do seu provedor | Porta 587 com STARTTLS, ou 465 com SSL/TLS. A porta 25 é bloqueada pela maioria dos provedores de nuvem |

Todas as predefinições usam a porta 587 com STARTTLS.

### Campos e regras {#fields-and-rules}

| Campo | O que informar |
|---|---|
| **Servidor SMTP** | O nome do host do provedor, ou um endereço IP |
| **Porta** | De 1 a 65535, geralmente 587 ou 465 |
| **Criptografia** | **STARTTLS** (porta 587), **SSL/TLS** (porta 465) ou **Nenhuma (só este servidor)** |
| **Nome de usuário**, **Senha** | Os dois ou nenhum, sem espaços. Deixe a **Senha** vazia para manter a que está salva |
| **Enviar tudo como (opcional)** | Um endereço de e-mail que substitui o remetente de cada mensagem |

- **Nenhuma (só este servidor)** só é aceita para um relay no próprio servidor (`localhost`, `127.0.0.1` ou `::1`). Sem criptografia, a senha atravessaria a internet em texto claro.
- Com STARTTLS ou SSL/TLS, o Postfix se recusa a enviar se a conexão não puder ser criptografada.
- Use **Enviar tudo como (opcional)** quando o seu provedor só aceita endereços de remetente verificados. Ele substitui o remetente de cada mensagem de cada site.
- A senha do provedor fica guardada em `/etc/postfix/sasl_passwd`, que só o root pode ler. Os sites nunca a veem.

### Enviar uma mensagem de teste {#send-a-test-message}

1. Embaixo do formulário, informe um endereço em **Enviar uma mensagem de teste para…**.
2. Selecione **Enviar teste**.

O painel acompanha a mensagem no log de e-mail e espera até um minuto pela resposta do provedor. O resultado diz se a mensagem foi enviada, se ainda não foi entregue (o Postfix continua tentando) ou se foi recusada pelo provedor, seguido da resposta do provedor. Se o provedor não responder em um minuto, a mensagem fica na fila e o Postfix continua tentando.

### Conferir o relay {#check-the-relay}

No topo do cartão:

- **Retransmitindo por …** significa que o Postfix roda e usa o seu provedor. **O Postfix não está rodando** significa que ele está parado.
- **… mensagens aguardando** conta as mensagens que ainda estão na fila. O Postfix continua tentando enviar uma mensagem por até 2 dias.
- **Último problema de entrega:** mostra o motivo mais recente pelo qual uma mensagem não pôde ser entregue, como o provedor o informou.

### Remover o relay {#remove-the-relay}

Selecione **Remover** e confirme. O painel desinstala o Postfix e esquece as configurações. Os sites não conseguem mais enviar e-mail.

## Alertas {#alerts}

Os alertas enviam uma mensagem para você por Telegram, e-mail, Discord, Slack ou o seu próprio webhook, quando o servidor precisa de atenção. Vá até **Configurações** → **Alertas**. Todos os canais são opcionais: configure quantos quiser, e cada alerta vai para todos eles ao mesmo tempo. Um canal que falha não impede os outros.

O painel verifica o servidor a cada minuto. Cada problema é avisado uma vez, de novo a cada 6 horas enquanto durar, e mais uma vez quando for resolvido. Cada mensagem começa com o nome do servidor entre colchetes. Os alertas por e-mail têm o assunto "Server alert:" (alerta do servidor) seguido do nome do servidor.

### O que envia um alerta {#what-sends-an-alert}

| Alerta | Enviado quando | Padrão |
|---|---|---|
| **CPU acima de (%)** | o uso de CPU na média de 5 minutos atinge o limite | 90 |
| **Memória acima de (%)** | o uso de memória atinge o limite | 90 |
| **Disco acima de (%)** | o disco (`/`) está cheio nessa proporção | 90 |
| **Um serviço para (Nginx, PHP, MariaDB…)** | um serviço que inicia com o sistema não está rodando | ativado |
| **Um backup, uma restauração ou uma renovação de certificado falha** | um backup, uma restauração, a renovação do certificado de um site ou a renovação do certificado do painel falha | ativado |
| Monitoramento de sites (a aba **Monitoramento** de cada site) | um site monitorado cai (2 verificações malsucedidas seguidas), volta ao ar, ou serve um certificado que termina em menos de 14 dias | desativado |
| **Logins SSH** | alguém faz login por SSH: todas as vezes, ou só de um endereço novo | **Nenhum alerta** |
| **Tentativas SSH malsucedidas em 10 minutos** | esse número de logins SSH falhou em 10 minutos | 0 (desativado) |
| **Logins no painel** | alguém faz login no painel: todas as vezes, ou só de um endereço novo | **Nenhum alerta** |
| **Avisar quando a verificação diária encontrar um novo problema** (na página **Verificação de segurança**) | a [verificação de segurança](./security-check) diária encontra um novo problema crítico ou aviso | desativado |
| **Avisar quando arquivos PHP mudarem** (a aba **Bloqueio de arquivos** de um site) | a verificação diária de um site bloqueado encontra arquivos PHP alterados, acrescentados ou removidos (veja [Bloqueio de arquivos](./website-protection#alerts-when-php-files-change)) | desativado |

- Defina um limite como 0 para desativá-lo. Os outros limites precisam estar entre 10 e 100.
- Os serviços vigiados são o servidor web (Nginx, Apache ou OpenLiteSpeed), o PHP-FPM, o MariaDB ou o MySQL, o PostgreSQL, o Redis, o Memcached e o fail2ban, quando instalados.
- Nada é verificado até que pelo menos um canal esteja configurado.
- O monitoramento de sites é ativado em cada site, e as verificações dele rodam mesmo sem canal: a aba **Monitoramento** mostra os resultados. Os alertas de site fora do ar se repetem a cada 6 horas, como os outros. Veja [Sites](./websites#monitoring).

### Alertas de login {#sign-in-alerts}

Receba uma mensagem quando alguém fizer login no servidor por SSH ou no painel, para que um login que não foi você não passe despercebido.

1. Vá até **Configurações** → **Alertas** e configure pelo menos um canal.
2. Em **Logins**, escolha para **Logins SSH** e **Logins no painel**:
   - **Nenhum alerta**;
   - **Todo login**;
   - **Só de um endereço novo**: um login de um endereço de onde a conta não fez login nos últimos 180 dias.
3. Opcional: em **Tentativas SSH malsucedidas em 10 minutos**, informe quantas tentativas malsucedidas enviam um alerta, de 3 a 100000. 0 desativa.
4. Clique em **Salvar**.

Um alerta de SSH cita a conta, o endereço, como a conta fez login e o horário, por exemplo:

```
[server1] 🔑 SSH sign-in: admin from 203.0.113.10 (new address) with a key, 2026-11-27 10:31:05 +07
```

Vários logins no mesmo minuto vêm em uma única mensagem. Uma rajada de tentativas malsucedidas cita os endereços que mais tentaram:

```
[server1] ⚠️ 57 failed SSH sign-in attempts in the last 10 minutes, from 198.51.100.7 (40), 203.0.113.5 (17).
```

Como funciona:

- Nada fica vigiando o log do SSH o tempo todo. Uma vez por minuto, o painel lê o que o serviço SSH (`ssh.service` ou `sshd.service`) gravou no journal do sistema desde a última leitura, e lembra onde parou. Os alertas de SSH não custam nada enquanto estão desativados.
- Quando você ativa os alertas de SSH, eles começam daquele momento: logins anteriores nunca são avisados. Os endereços dos logins SSH dos últimos 30 dias contam como conhecidos.
- Para o painel, os endereços dos logins do log de atividades contam como conhecidos.
- No IPv6, todos os endereços do mesmo /64 contam como um único endereço, já que a maioria dos provedores dá um /64 por cliente. Até 500 endereços são lembrados para o SSH e 500 para o painel.
- Se o painel ficou parado por mais de uma hora, ele só lê a última hora do log do SSH quando volta.
- As tentativas malsucedidas são senhas erradas e nomes de conta desconhecidos. Com o login por senha desativado, as chaves recusadas não contam. No máximo um alerta de rajada é enviado por hora.
- Para bloquear os endereços que continuam tentando, instale o fail2ban na página **Firewall e SSH**. Veja [Proteção contra força bruta](./security#brute-force-protection-fail2ban).

### Receber alertas pelo Telegram {#get-alerts-by-telegram}

1. No Telegram, abra o BotFather, envie `/newbot` e responda às perguntas dele. Ele responde com o token do bot: dígitos, dois-pontos e depois uma longa sequência de letras e dígitos.
2. Abra o seu novo bot e envie qualquer mensagem para ele. Um bot não consegue escrever para você antes que você escreva para ele.
3. Descubra o seu ID do chat, por exemplo com o userinfobot. É um número.
4. Em **Configurações** → **Alertas**, informe o **Token do bot** e o **ID do chat**.
5. Selecione **Salvar** e depois **Enviar um teste**.

Para enviar alertas a um grupo ou canal, acrescente o bot a ele antes. O ID do chat de um grupo começa com um sinal de menos. Para um canal público, você também pode informar o `@name` dele.

Depois de salvar, o token aparece como pontos. Deixe os pontos como estão para manter o token salvo. Se você apagar o **ID do chat** e salvar, o token também é removido.

### Receber alertas por e-mail {#get-alerts-by-e-mail}

Os e-mails de alerta são enviados direto pelo painel por um servidor SMTP, com configurações próprias, separadas do relay dos sites. Você pode usar a mesma conta do provedor.

1. Em **E-mail**, informe o **Servidor SMTP** e a **Porta** (587 por padrão).
2. Informe o **Nome de usuário** e a **Senha** da conta, se o servidor pedir login.
3. Informe os endereços **De** e **Para**.
4. Selecione **Salvar** e depois **Enviar um teste**.

Na porta 465, a conexão é criptografada desde o início. Nas outras portas, o painel usa STARTTLS quando o servidor o oferece. Se o servidor não o oferecer e houver um nome de usuário definido, o painel se recusa a enviar, para que a senha nunca viaje em texto claro. A senha salva aparece como pontos, como o token do Telegram.

### Receber alertas no Discord {#get-alerts-in-discord}

1. No Discord, abra as configurações do canal que deve receber os alertas e depois **Integrações** → **Webhooks** → **Novo webhook**.
2. Dê um nome ao webhook, se quiser, e depois selecione **Copiar URL do webhook**.
3. Em **Configurações** → **Alertas**, cole-a em **URL do webhook**, embaixo de **Discord**.
4. Selecione **Salvar** e depois **Enviar um teste**.

A URL tem a forma `https://discord.com/api/webhooks/<id>/<token>`. Os endereços em `discordapp.com` e as versões `ptb.` e `canary.` também funcionam, assim como `?thread_id=` para postar em uma thread. Os alertas com mais de 2000 caracteres, o limite do Discord, são cortados. As menções em um alerta, como `@everyone`, não notificam ninguém.

### Receber alertas no Slack {#get-alerts-in-slack}

1. Vá até [api.slack.com/apps](https://api.slack.com/apps), selecione **Create New App** → **From scratch** (criar um novo app do zero) e escolha o seu workspace.
2. Abra **Incoming Webhooks**, ative-o, depois selecione **Add New Webhook to Workspace** (adicionar um novo webhook ao workspace) e escolha o canal.
3. Copie a **Webhook URL**. Ela começa com `https://hooks.slack.com/services/`.
4. Em **Configurações** → **Alertas**, cole-a em **URL do webhook**, embaixo de **Slack**, selecione **Salvar** e depois **Enviar um teste**.

Só os incoming webhooks são aceitos: os webhooks de workflow do Slack esperam outros campos.

### Enviar alertas para um webhook {#send-alerts-to-a-webhook}

O webhook envia cada alerta como JSON para um endereço da sua escolha: o seu próprio script, o n8n, o Home Assistant, um chat que não está listado aqui. Em **Webhook**, informe a **URL do webhook** dele e, se o receptor confere assinaturas, um **Segredo para assinatura (opcional)** de 16 a 200 caracteres, sem espaços. Depois selecione **Salvar** e **Enviar um teste**.

Cada alerta é um `POST` com `Content-Type: application/json` e este corpo:

```json
{"server":"vps-1","text":"⚠️ The disk is 93% full.","time":"2026-11-26T08:15:00Z","kind":"alert"}
```

| Campo | Valor |
|---|---|
| `server` | O nome do host do servidor |
| `text` | O alerta, sem o nome do servidor. Os problemas resolvidos começam com "✅ Resolved:" (resolvido) |
| `time` | Quando ele foi enviado, em UTC, no formato RFC 3339 |
| `kind` | `alert`, ou `test` para **Enviar um teste** |

Qualquer resposta 2xx conta como entregue. O painel espera no máximo 10 segundos, não segue redirecionamentos e não mostra a resposta do receptor, só o código de status dela.

#### Verificar a assinatura {#verify-the-signature}

Com um segredo, cada requisição tem o cabeçalho `X-Nefo-Signature: sha256=` seguido do HMAC-SHA256 do corpo, em hexadecimal, com o segredo como chave. Calcule-o sobre o corpo exatamente como foi recebido, antes de interpretar o JSON, e compare em tempo constante. Em Python:

```python
import hashlib, hmac

def verify(secret: bytes, body: bytes, signature: str) -> bool:
    expected = "sha256=" + hmac.new(secret, body, hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected, signature)
```

Em Node.js, compare `'sha256=' + crypto.createHmac('sha256', secret).update(rawBody).digest('hex')` com o cabeçalho usando `crypto.timingSafeEqual`, depois de conferir que os dois têm o mesmo tamanho. O corpo assinado inclui o `time`: recuse requisições com mais de alguns minutos para impedir repetições (replays).

#### Endereços que um webhook não pode usar {#webhook-addresses}

- A URL começa com `https://`. Um endereço `http://` simples só funciona depois que você marca **Permitir um endereço http:// (sem criptografia, para um receptor na sua própria rede)**: sem criptografia, o alerta e a assinatura dele podem ser lidos no caminho.
- Nenhum nome de usuário nem senha na URL, nenhum `#`, e no máximo 2048 caracteres.
- Para que o webhook não consiga chegar aos serviços do próprio servidor, o painel nunca se conecta a endereços de loopback (`127.0.0.0/8`, `::1`, `localhost`), endereços link-local (`169.254.0.0/16`, `fe80::/10`), serviços de metadados de nuvem (`169.254.169.254`, `100.100.100.200`, `168.63.129.16`, `fd00:ec2::254`), `0.0.0.0/8` nem endereços multicast. Um nome de host é conferido de novo a cada envio de alerta, nos endereços para os quais ele resolve, por isso um nome que aponta para `127.0.0.1` também é recusado.
- As redes privadas (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `fd00::/8`) são permitidas, para um receptor em casa ou em um contêiner.

### URLs de webhook salvas {#saved-webhook-urls}

As URLs do Discord, do Slack e do webhook contêm o token delas, por isso depois de salvar elas aparecem como pontos, como o token do Telegram, e o segredo também. Deixe os pontos como estão para manter o valor salvo. Apague uma URL e salve para parar de usar aquele canal. Apagar a URL do webhook também remove o segredo dele e a permissão de http://.

### Enviar um alerta de teste {#send-a-test-alert}

**Enviar um teste** envia uma mensagem de teste por todos os canais salvos. Ele usa as configurações salvas, por isso selecione **Salvar** antes. O resultado mostra uma linha por canal, **Telegram**, **E-mail**, **Discord**, **Slack** ou **Webhook**, com **enviado** ou o erro. Se nenhum canal estiver configurado, o painel diz "Configure um canal primeiro e salve."

Para o Discord, o Slack e o webhook, o erro é um destes:

- **Não foi possível alcançar o webhook**, seguido do motivo, por exemplo `no such host` ou `connection refused`.
- **O webhook recusou a mensagem (HTTP 404…). Confira a URL dele.**: o receptor respondeu com algo diferente de um status 2xx. O Discord e o Slack acrescentam o motivo deles, como `Unknown Webhook` ou `invalid_token`. Um redirecionamento (3xx) também conta como recusa.
- **O webhook não respondeu em 10 segundos.**
- **… é um endereço de loopback, link-local ou de metadados de nuvem**: veja [Endereços que um webhook não pode usar](#webhook-addresses).

Quando um alerta real não pode ser enviado, o log do painel avisa, com o nome do canal, nunca com a URL dele.

### Os alertas e o dashboard {#alerts-and-the-dashboard}

A lista **Precisa de atenção** no **Dashboard** e o sino no canto superior direito só mostram os problemas enquanto você está com a sessão aberta, e eles verificam outras coisas, como o login com dois fatores e o firewall. Os alertas chegam até você quando você está longe. Veja [Ferramentas do sistema](./system#dashboard).
