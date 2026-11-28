---
title: Chaves de API
description: Automatize o painel com chaves de API, para deploys de CI, backups e monitoramento, sem uma sessão no navegador. O que uma chave pode e não pode fazer, como chamar a API, exemplos e erros.
---

# Chaves de API

Uma chave de API deixa um script usar o painel do Nefo sem navegador, por exemplo para fazer o deploy de um site a partir da CI, iniciar um backup a partir de uma tarefa cron em outro servidor ou ler o estado dos sites a partir de uma ferramenta de monitoramento. Uma chave é somente leitura ou tem acesso total, pode ser limitada aos endereços que você listar e pode expirar.

O script envia a chave em cada requisição, como um token Bearer, para o mesmo endereço e caminho secreto do painel.

As chaves ficam em **Configurações** → **Chaves de API**.

## O que uma chave pode fazer {#what-a-key-can-do}

Cada chave tem um destes dois tipos de acesso:

- **Somente leitura**: só requisições `GET` e `HEAD`. Ela pode listar sites, bancos de dados e backups, ler logs de tarefas, estatísticas e o estado do servidor, mas não alterar nada. Ela também não pode ler o que lhe daria mais poder: senhas de bancos de dados e de apps, exportações de bancos de dados, arquivos (o gerenciador de arquivos), downloads de backups, valores do Redis e segredos de webhooks do Git. Essas requisições, e qualquer requisição que não seja `GET` ou `HEAD`, recebem `403` com `api_key_read_only`.
- **Acesso total**: todas as requisições que as próprias páginas do painel fazem, em seu nome.

Nem mesmo uma chave com acesso total pode:

- criar, listar ou revogar chaves de API;
- trocar a senha ou o login com dois fatores, ou encerrar a sessão (tudo o que fica em `/api/auth/`);
- abrir o terminal web ou o terminal de um contêiner;
- usar as ferramentas de banco de dados que ficam atrás do login do painel, o Adminer e o phpMyAdmin.

Essas respondem `403` com `api_key_forbidden`: faça login no painel para usá-las. Uma chave também não pode fazer login para obter uma sessão.

::: warning
Uma chave com acesso total é tão poderosa quanto você: ela pode gravar arquivos, acrescentar tarefas cron que rodam como root e instalar software. Guarde-a com o mesmo segredo que a senha do root. Uma chave somente leitura também é um segredo: ela pode ler as configurações, os logs e as listas dos seus sites e do servidor.
:::

## Criar uma chave {#create-a-key}

1. Vá até **Configurações** → **Chaves de API** e selecione **Nova chave**.
2. Informe um **Nome**, como `deploy-bot`: até 32 letras, dígitos, espaços, pontos, hifens e sublinhados. O nome aparece no log de atividades.
3. Escolha **Somente leitura** ou **Acesso total**.
4. Opcional: em **Endereços permitidos (opcional)**, informe os endereços IP ou as redes de onde a chave pode ser usada, como `203.0.113.7` ou `198.51.100.0/24`, IPv4 ou IPv6, separados por espaços ou quebras de linha. Deixe vazio para permitir qualquer endereço.
5. Escolha quando ela **Expira**: depois de 30, 90 ou 365 dias, ou **Nunca**.
6. Selecione **Nova chave**.

O painel mostra a chave uma única vez, com um botão **Copiar**: copie-a agora, você não vai vê-la de novo. Uma chave tem a forma `nefo_` seguido de 43 caracteres aleatórios. O painel guarda só um hash SHA-256 dela.

A lista mostra o nome e os primeiros caracteres de cada chave (como `nefo_Ab12cd…`), o acesso, os endereços permitidos, quando e de onde ela foi usada pela última vez (atualizado no máximo uma vez por minuto) e quando ela expira. Você pode ter até 20 chaves. Para revogar uma chave, selecione o ícone da lixeira dela e confirme: a próxima requisição com ela falha.

## Chamar a API {#call-the-api}

Envie a chave no cabeçalho `Authorization`, como um token Bearer. O endereço é o endereço do painel com o caminho secreto, depois `api/` e a rota:

```sh
export NEFO_KEY='nefo_…'
export NEFO='https://panel.example.com:PORT/<entrance>'

curl -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/sites"
```

- A chave só é lida do cabeçalho `Authorization`: nunca do endereço (`?key=`) nem de um cookie.
- Uma requisição com corpo JSON precisa de `Content-Type: application/json`. Campos desconhecidos são recusados.
- As requisições com chave não precisam do cabeçalho `X-Requested-With` que as próprias páginas do painel enviam.
- Use o painel por HTTPS com um certificado de verdade (**Configurações** → **Certificado**, veja [Segurança](./security#https-from-the-first-start)). Evite o `curl -k`: ele enviaria a chave para quem estiver no meio do caminho.

As tarefas longas, como backups, deploys e certificados, rodam em segundo plano. A requisição delas responde `202` com uma tarefa:

```json
{ "id": "a1b2c3", "kind": "backup", "target": "example.com", "status": "running", "log": [], "started": "…" }
```

Consulte `GET /api/jobs/{id}` até o `status` ser `succeeded` ou `failed`; `log` contém as linhas dela e `error`, o motivo da falha. O painel roda uma tarefa por vez: enquanto outra roda, uma nova responde `409` com `busy`.

## Exemplos {#examples}

As rotas são as que as páginas do painel usam. Troque `3` pelo `id` do seu site.

### Listar os sites {#list-the-websites}

```sh
curl -s -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/sites"
```

A resposta é `{"sites": [{"id": 3, "domain": "example.com", "ssl": true, …}]}`. Uma chave somente leitura basta.

### Fazer backup de um site {#back-up-a-website}

```sh
curl -s -X POST -H "Authorization: Bearer $NEFO_KEY" -H "Content-Type: application/json" \
  -d '{"kind": "site", "target": "example.com", "remote": true}' \
  "$NEFO/api/backups"
```

- `kind` é `site` (`target` é o domínio), `database` (o nome do banco de dados), `folder` (um caminho absoluto) ou `app` (o nome de um app da loja de aplicativos).
- `remote: true` também envia o backup para o armazenamento remoto, se ele estiver configurado (veja [Backups](./backups#remote-storage)).

Ela responde `202` com a tarefa. Para esperar por ela:

```sh
JOB=$(curl -s -X POST -H "Authorization: Bearer $NEFO_KEY" -H "Content-Type: application/json" \
  -d '{"kind": "database", "target": "shop", "remote": false}' "$NEFO/api/backups" | jq -r .id)
while [ "$(curl -s -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/jobs/$JOB" | jq -r .status)" = running ]; do
  sleep 5
done
```

### Fazer deploy pelo Git {#deploy-from-git}

Configure antes o repositório na aba **Deploy pelo Git** do site (veja [Deploy pelo Git](./git-deploy)). Depois, a partir da sua CI, quando os testes passarem:

```sh
curl -fsS -X POST -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/sites/3/git/deploy"
```

Ela não precisa de corpo e responde `202` com a tarefa. Em um workflow do GitHub Actions, guarde a chave e o endereço nos secrets do repositório:

```yaml
- name: Deploy
  run: curl -fsS -X POST -H "Authorization: Bearer ${{ secrets.NEFO_KEY }}" "${{ secrets.NEFO_URL }}/api/sites/3/git/deploy"
```

### Instalar um certificado {#install-a-certificate}

Para obter um certificado gratuito do Let's Encrypt (veja [Certificados SSL](./ssl)):

```sh
curl -s -X POST -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/sites/3/ssl"
```

Ela responde `202` com a tarefa. Para validar pelo DNS do Cloudflare, que os nomes wildcard exigem, envie `-H "Content-Type: application/json" -d '{"dns": true}'`.

Para instalar o seu próprio certificado, como um renovado em outro servidor, envie o certificado com a cadeia e a chave privada, em PEM, com até 256 KB no total. O `jq` monta o JSON a partir dos arquivos:

```sh
jq -n --rawfile cert fullchain.pem --rawfile key privkey.pem '{cert: $cert, key: $key}' |
  curl -s -X POST -H "Authorization: Bearer $NEFO_KEY" -H "Content-Type: application/json" \
    --data-binary @- "$NEFO/api/sites/3/certificate"
```

Ela responde `200` com o site e uma lista de `warnings`, ou `400` com o motivo, como uma chave que não corresponde ao certificado (veja [Usar o seu próprio certificado](./ssl#use-your-own-certificate)).

## Erros {#errors}

Os erros vêm em JSON, como `{"error": "invalid_api_key"}`, alguns com um `detail`.

| Status | `error` | Significado |
|---|---|---|
| 401 | `invalid_api_key` | A chave é desconhecida, foi revogada ou expirou, ou foi usada de um endereço que ela não permite. A resposta não diz qual; o log de atividades diz. |
| 403 | `api_key_read_only` | Uma chave somente leitura enviou uma requisição que não é `GET` nem `HEAD`, ou pediu uma senha, um arquivo, uma exportação ou um download de backup. |
| 403 | `api_key_forbidden` | Só uma sessão com login pode fazer isso (veja [O que uma chave pode fazer](#what-a-key-can-do)). |
| 429 | `too_many_attempts` | Tentativas malsucedidas demais a partir deste endereço. Espere `retry_after` segundos, também no cabeçalho `Retry-After`. |
| 400 | como `bad_request` | A requisição está errada; `detail` pode dizer o quê. |
| 404 | `not_found` | O item não existe. Um caminho fora do caminho secreto recebe um 404 simples. |
| 409 | `busy` | Outra tarefa está rodando. Tente de novo quando ela terminar. |

## Tentativas malsucedidas e o fail2ban {#failed-attempts-and-fail2ban}

Uma tentativa malsucedida com uma chave conta como um login malsucedido, na mesma contagem (veja [Limites de login](./security#sign-in-limits)): 5 falhas em 15 minutos bloqueiam o endereço por 15 minutos, para as chaves e para a página de login. Uma chave válida não zera a contagem.

Cada falha aparece em **Conta** → **Atividade recente** como "Login malsucedido", com o motivo: `unknown API key`, `API key expired` ou `API key not allowed from this address`. A jail do painel no fail2ban lê essas entradas, por isso bane um endereço que continua tentando adivinhar (veja [Proteção contra força bruta](./security#brute-force-protection-fail2ban)).

## O log de atividades {#the-activity-log}

- Cada alteração feita com uma chave (toda requisição, exceto `GET` e `HEAD`) é registrada como "Alteração por uma chave de API", com o método e o caminho, como `POST /api/sites/3/git/deploy`.
- O usuário dessas entradas é o seu nome de usuário e o nome da chave, como `admin (key deploy-bot)`. O mesmo vale para as entradas do que a requisição fez, como "Deploy feito pelo Git".
- As requisições recusadas a uma chave são registradas como "Chave de API recusada"; a criação e a revogação de chaves, como "Chave de API criada" e "Chave de API revogada".
- As leituras não são registradas. A lista de chaves mostra quando cada uma foi usada pela última vez, e de onde.

A própria chave nunca é gravada em nenhum log.

## Conselhos de segurança {#security-advice}

- **Use chaves somente leitura onde puder.** O monitoramento e os relatórios só precisam de `GET`.
- **Uma chave por script.** O nome dela aparece no log, e você pode revogá-la sem quebrar as outras.
- **Permita só os endereços que precisam**, como os seus runners de CI ou o seu servidor de backup, quando eles têm endereços fixos.
- **Deixe as chaves expirarem, e faça a rotação delas**: crie uma nova chave, coloque-a no script e depois revogue a antiga.
- **Mantenha as chaves fora dos repositórios e dos endereços.** Use o cofre de segredos da sua CI ou um arquivo que só o dono pode ler, e uma variável nos comandos, para que a chave não acabe no histórico do shell.
- **Revogue uma chave na hora se ela pode ter vazado**, e depois confira em **Atividade recente** o que ela fez.
- Se o [firewall](./security#firewall-ufw) limita a porta do painel a alguns endereços, permita lá também o endereço do script.
