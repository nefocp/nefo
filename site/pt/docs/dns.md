---
title: DNS (Cloudflare)
description: Gerencie pelo painel os registros DNS das suas zonas do Cloudflare, aponte os nomes de um site para este servidor, acrescente os registros MX, SPF e DMARC do seu provedor de e-mail e limpe o cache do Cloudflare.
---

# DNS (Cloudflare)

O Nefo não roda um servidor DNS. Se o DNS do seu domínio está hospedado no Cloudflare, o Nefo gerencia os registros dele pela API do Cloudflare, com um token de API: ele acrescenta, altera e exclui registros, aponta os nomes de um site para este servidor depois de uma prévia, acrescenta os registros MX, SPF e DMARC do seu provedor de e-mail e limpa o cache do Cloudflare.

Você salva o token em **Configurações** → **Cloudflare**. Depois:

- a página **DNS**, no grupo **Hospedagem** do menu à esquerda, lista as suas zonas e os registros delas, acrescenta, altera e exclui registros, e limpa o cache do Cloudflare;
- **Apontar o DNS para cá…**, na aba **Domínios** de um site, aponta os nomes do site para este servidor em um único passo;
- **Configurar e-mail**, na página DNS, acrescenta os registros MX, SPF e DMARC de um provedor de e-mail;
- os certificados wildcard são validados pelo mesmo token: veja [Certificados SSL](./ssl#wildcard-certificates-through-cloudflare-dns).

O Nefo só conversa com a API do Cloudflare, e o token nunca aparece nos logs nem nas mensagens do painel.

## O token de API {#the-api-token}

Um único token serve para tudo isso. Ele precisa destas permissões, para as zonas (domínios) que você usa neste servidor:

| Permissão | Para que serve |
|---|---|
| Zone → DNS → Edit | Listar, acrescentar, alterar e excluir registros, **Apontar o DNS para cá…** e a validação de certificados por DNS. |
| Zone → Zone → Read | Listar as zonas com o status e o plano delas. Sem ela, a lista de zonas pode ficar vazia, ou o Cloudflare se recusa a listar as zonas. |
| Zone → Cache Purge → Purge | **Limpar o cache** na página DNS. |

1. No painel do Cloudflare, abra **My Profile → API Tokens → Create Token** (meu perfil → tokens de API → criar token).
2. Comece pelo modelo **Edit zone DNS**. Ele dá Zone → DNS → Edit.
3. Clique em **+ Add more** (adicionar mais) e acrescente Zone → Zone → Read e Zone → Cache Purge → Purge.
4. Em **Zone Resources** (recursos de zona), inclua as zonas que você usa aqui, ou todas as zonas da conta.
5. Crie o token, copie-o e salve-o no Nefo: **Configurações** → **Cloudflare** → **Token de API** → **Verificar e salvar**.

Quando falta uma permissão ao token, o Nefo diz qual, por exemplo: "O Cloudflare recusou: falta ao token de API a permissão Zone → DNS → Edit para esta zona." Edite o token no Cloudflare (**My Profile → API Tokens**, depois **Edit** ao lado do token), acrescente a permissão para a zona e salve. Editar um token mantém o valor dele, por isso não há nada a mudar no Nefo. Se você excluiu ou trocou (roll) o token, salve o novo em **Configurações** → **Cloudflare**.

## A página DNS {#the-dns-page}

A página mostra uma zona por vez. Com várias zonas, escolha uma na lista do topo; o status dela (**Ativa**, ou **Pendente** enquanto os servidores de nomes do domínio ainda não são os do Cloudflare) e o plano aparecem ao lado.

A tabela lista os registros da zona: **Tipo**, **Nome** (`@` é a própria zona, `www` é `www.example.com`), **Conteúdo**, **Proxy** e **TTL**. Filtre por tipo, ou pesquise nos nomes e no conteúdo. Uma zona com mais de 5.000 registros mostra os primeiros 5.000; use o painel do Cloudflare para os outros.

O Nefo pode acrescentar e alterar registros A, AAAA, CNAME, TXT, MX e CAA. Outros tipos, como NS ou SRV, aparecem com "no Cloudflare": altere-os no painel do Cloudflare.

### Acrescentar ou alterar um registro {#add-or-change-a-record}

Clique em **Adicionar registro**, ou em **Editar** na linha de um registro. O tipo de um registro não pode mudar: exclua-o e acrescente um novo. O Nefo confere o registro antes de enviá-lo ao Cloudflare:

| Tipo | O que informar |
|---|---|
| A | Um endereço IPv4, como `203.0.113.10`. |
| AAAA | Um endereço IPv6, como `2001:db8::10`. |
| CNAME | O nome do host do qual este nome é um alias, como `example.com`. Ele não pode apontar para o próprio nome. |
| TXT | Uma linha de texto, com até 2.048 caracteres: texto simples, que o Cloudflare divide em strings de 255 caracteres, ou `"quoted strings"` de até 255 caracteres cada. Coloque aspas no valor inteiro ou em nenhuma parte dele. |
| MX | O nome do host do servidor de e-mail, como `mail.example.com`, e uma **Prioridade** de 0 a 65535 (a menor é tentada primeiro). |
| CAA | Uma **Tag**: `issue` ou `issuewild` com o domínio de uma CA, como `letsencrypt.org` (`;` não permite nenhuma CA), ou `iodef` com um endereço `mailto:` ou `https://`. As **Flags** são 0, ou 128 para crítico. |

O **TTL** é **Automático** (a escolha do Cloudflare) ou um tempo fixo de 1 minuto a 1 dia.

### Com proxy ou somente DNS {#proxied-or-dns-only}

Os registros A, AAAA e CNAME podem ficar **Com proxy** (a nuvem laranja do Cloudflare): os visitantes então chegam ao Cloudflare, que repassa as requisições web para o endereço e o esconde. Os registros com proxy sempre têm o TTL automático. **Somente DNS** (nuvem cinza) responde com o próprio endereço. Os outros tipos de registro são sempre somente DNS.

O proxy do Cloudflare só repassa HTTP e HTTPS nas portas 80 e 443 (e em algumas outras). Mantenha todo o resto como somente DNS: o SSH, o e-mail e o painel.

::: warning Mantenha o nome do painel como somente DNS
Se você abre o painel por um nome como `panel.example.com`, mantenha esse registro como **Somente DNS**. O proxy do Cloudflare não repassa a porta do painel, por isso, com a nuvem laranja, o painel deixa de abrir por esse nome. A página DNS e **Apontar o DNS para cá…** avisam quando o registro está com proxy.
:::

## Apontar um site para este servidor {#point-a-website-to-this-server}

1. Abra **Sites** e depois a aba **Domínios** do site.
2. Em **DNS no Cloudflare**, clique em **Apontar o DNS para cá…**.
3. Confira os endereços e as escolhas:
   - **Endereço IPv4 público deste servidor**, lido das interfaces de rede do servidor;
   - **Publicar também um endereço IPv6 (AAAA)**, marcado quando o servidor tem um endereço IPv6 público;
   - **Com proxy pelo Cloudflare (nuvem laranja)**.
4. Leia a prévia: para cada nome e tipo, o registro **Agora** e **Depois**, marcado como **novo**, **alterado**, **removido** ou **sem alteração**.
5. Clique em **Aplicar**.

O Nefo só altera os registros dos nomes do site, o domínio e os nomes de domínio extras; os registros de outros nomes nunca são tocados. Para cada nome:

- um registro A aponta para este servidor: um registro A existente é atualizado, ou um novo é criado, e os outros registros A do nome são removidos;
- com IPv6, o mesmo acontece com os registros AAAA dele. Sem IPv6, os registros AAAA que apontam para outro servidor são removidos, já que os visitantes IPv6 e o Let's Encrypt chegariam a esse outro servidor;
- um registro CNAME é substituído pelo registro A, a menos que ele aponte para outro nome do site (`www` → `example.com` fica como está);
- os nomes que não estão em nenhuma zona que o token enxerga ficam como estão, e a prévia os lista.

Quando você clica em **Aplicar**, o Nefo calcula as alterações de novo. Se os registros mudaram no Cloudflare desde a prévia, nada é alterado e o Nefo mostra a nova prévia: "Os registros no Cloudflare mudaram desde a prévia."

Se o servidor está atrás de NAT e só tem um endereço IPv4 privado, o Nefo diz "Nenhum endereço IPv4 público foi encontrado nas interfaces de rede deste servidor". Digite o endereço público que chega ao servidor. O Nefo não pergunta isso a nenhum serviço de fora.

### Let's Encrypt atrás da nuvem laranja {#let-s-encrypt-behind-the-orange-cloud}

O Let's Encrypt confere um site por HTTP simples. Com o proxy ativado, a configuração **Always Use HTTPS** (sempre usar HTTPS) do Cloudflare (**SSL/TLS → Edge Certificates**) responde a essa verificação com um redirecionamento antes de ela chegar ao servidor, e o certificado falha. Você pode:

- desativar o **Always Use HTTPS** no Cloudflare e ativar [Redirecionar HTTP para HTTPS](./ssl#redirect-http-to-https) no Nefo; ou
- marcar **Validar pelo DNS (Cloudflare)** na aba **SSL / HTTPS** do site e deixar as configurações do Cloudflare como estão.

Quando o site tiver um certificado, defina o modo de criptografia **SSL/TLS** do Cloudflare como **Full (strict)** (completo, estrito). Com **Flexible** (flexível), o Cloudflare chega ao servidor por HTTP, e **Redirecionar HTTP para HTTPS** o manda de volta para HTTPS, em um loop.

## Como configuro os registros DNS do meu e-mail? {#set-up-email-records}

O Nefo não hospeda caixas de correio (veja [E-mail e alertas](./mail-alerts)). Se o seu e-mail está hospedado no Google Workspace, no Microsoft 365, no Zoho Mail ou em outro provedor, ou é encaminhado pelo Cloudflare Email Routing, o Nefo pode acrescentar os registros DNS de que o provedor precisa:

| Registro | O que faz |
|---|---|
| MX | Diz quais servidores recebem e-mail para o domínio. |
| SPF (um registro TXT) | Diz quais servidores podem enviar e-mail pelo domínio. |
| DMARC (um registro TXT em `_dmarc`) | Diz aos servidores que recebem o que fazer com os e-mails que falham nas verificações de SPF e DKIM, e para onde enviar relatórios. |
| DKIM (um registro TXT ou CNAME) | Publica a chave com que o seu provedor assina os e-mails. |

Nada muda até você ver a lista de alterações e aplicá-la.

1. Abra **DNS** e escolha a zona.
2. Clique em **Configurar e-mail**.
3. Em **Provedor de e-mail**, escolha **Google Workspace**, **Microsoft 365**, **Zoho Mail**, **Cloudflare Email Routing** ou **Outro provedor**.
4. Opcional: em **Domínio de e-mail**, informe um nome da zona, como `mail` para `mail.example.com`. Deixe vazio para a própria zona.
5. Opcional: informe um **Endereço para relatórios DMARC (opcional)**, e cole o registro DKIM se você já o tiver (veja abaixo).
6. Clique em **Mostrar as alterações**. A tabela lista cada registro com o valor **Agora** e **Depois** e o que acontece com ele: novo, alterado, removido, sem alteração ou outro provedor.
7. Clique no botão de aplicar, como **Aplicar 4 alterações**.

Se você mudar um campo depois da prévia, clique em **Ver a prévia de novo** antes de aplicar. Se os registros no Cloudflare mudaram nesse meio-tempo, o Nefo recusa e mostra a nova prévia.

### O que cada provedor recebe {#what-each-provider-gets}

| Provedor | Registros MX | Include do SPF |
|---|---|---|
| **Google Workspace** | `smtp.google.com`, prioridade 1 | `include:_spf.google.com` |
| **Microsoft 365** | `example-com.mail.protection.outlook.com` (o seu domínio com hifens no lugar dos pontos), prioridade 0 | `include:spf.protection.outlook.com` |
| **Zoho Mail** | `mx.zoho.com` (10), `mx2.zoho.com` (20), `mx3.zoho.com` (50), com o domínio do **Data center da Zoho** escolhido: zoho.com, zoho.eu, zoho.in ou zoho.com.au | `include:zohomail.com`, ou `zohomail.eu`, `zohomail.in`, `zohomail.com.au` |
| **Cloudflare Email Routing** | `route1.mx.cloudflare.net`, `route2.mx.cloudflare.net`, `route3.mx.cloudflare.net` | `include:_spf.mx.cloudflare.net` |
| **Outro provedor** | Os servidores que você informar em **Servidores de e-mail (MX)**, até 5, cada um com uma prioridade | O domínio em **Include do SPF (opcional)** e os endereços em **Endereços de envio (opcional)**; sem nenhum dos dois, `mx` |

Alguns detalhes:

- **Google Workspace**: se a zona já tem os registros MX mais antigos do Google (`aspmx.l.google.com` e os outros), o Nefo os mantém e não acrescenta nada.
- **Microsoft 365**: a Microsoft dá a cada domínio o seu próprio host MX. Compare o valor da prévia com o que o Microsoft 365 admin center mostra para o domínio. Se for diferente, escolha **Outro provedor** e informe-o. Um registro MX que já aponta para o Microsoft 365 é mantido como está.
- **Zoho Mail**: escolha o data center em cujo endereço você faz login. Para outro data center da Zoho, escolha **Outro provedor** e informe os valores que a Zoho mostra.
- **Cloudflare Email Routing**: o e-mail só chega depois que você ativa o Email Routing para a zona no painel do Cloudflare e acrescenta lá um endereço de destino. Os registros de roteamento do Cloudflare que já existem mantêm as prioridades que o Cloudflare deu a eles.

### SPF: sempre um único registro {#spf-always-one-record}

Um domínio precisa ter exatamente um registro SPF; dois fazem a verificação SPF falhar. O Nefo nunca acrescenta um segundo:

- Se a zona tem um registro SPF, o Nefo mantém tudo o que há nele, acrescenta o include do provedor e mantém o final dele (`-all`, `~all` ou `?all`). Um registro sem final recebe `~all`.
- Se a zona tem vários registros SPF, o Nefo os mescla em um só: ele altera o primeiro e remove os outros.
- Se você configurou um relay SMTP em **Configurações** → **E-mail**, o registro SPF também permite o provedor do relay (por exemplo `include:sendgrid.net` para o SendGrid, `include:mailgun.org` para o Mailgun, `include:amazonses.com` para o Amazon SES), para que os e-mails que os seus sites enviam passem na verificação. A prévia avisa isso.
- Um registro SPF que precisa de mais de 10 consultas DNS (`include`, `a`, `mx`, `ptr`, `exists`, `redirect`) falha em muitos servidores que recebem. A prévia avisa; remova os includes que você não usa mais.

### Registros MX de outro provedor {#mx-records-of-another-provider}

Os registros MX que apontam para outro provedor ficam marcados como "outro provedor". Os e-mails iriam para os dois provedores, por isso eles precisam sair: marque **Substituir os registros MX existentes** para removê-los. O Nefo se recusa a aplicar as alterações até você fazer isso, ou até você mesmo remover esses registros.

::: warning
Os e-mails deixam de chegar ao provedor antigo assim que os registros MX dele são removidos. Leve antes as suas caixas de correio.
:::

### DMARC {#dmarc}

O Nefo acrescenta `v=DMARC1; p=none` em `_dmarc`, com `rua=mailto:` e o seu endereço quando você informa um **Endereço para relatórios DMARC (opcional)**. Com `p=none`, nenhum e-mail é rejeitado: os servidores que recebem só enviam relatórios. Quando os relatórios mostrarem que todos os seus e-mails passam, você pode deixar a política mais rígida (`p=quarantine`, depois `p=reject`) editando o registro na página DNS.

Se a zona já tem um registro DMARC, o Nefo o deixa como está.

### DKIM {#dkim}

O provedor de e-mail cria a chave DKIM, por isso o Nefo não consegue criá-la. Em **Assinatura DKIM**, a janela diz onde encontrá-la no seu provedor:

- **Google Workspace**: no Google Admin console, abra Apps → Google Workspace → Gmail → Authenticate email e gere um novo registro. O nome dele é `google._domainkey` e o tipo, TXT.
- **Microsoft 365**: no portal do Microsoft Defender, abra as configurações de DKIM do domínio. A Microsoft dá dois registros CNAME, `selector1._domainkey` e `selector2._domainkey`; a janela tem uma linha para cada um.
- **Zoho Mail**: no Zoho Mail Admin Console, abra as configurações de DKIM do domínio e acrescente um seletor. Informe o nome do seletor seguido de `._domainkey` e o valor TXT.
- **Cloudflare Email Routing** não precisa de registro DKIM aqui: o Cloudflare acrescenta o que precisa quando você ativa o Email Routing.
- **Outro provedor**: peça ao provedor o registro DKIM dele.

Cole o **Nome do registro** e o **Valor**, escolha o **Tipo** (TXT ou CNAME) e veja a prévia. Um nome como `google._domainkey` fica sob o domínio de e-mail; um nome completo também é aceito. Um registro DKIM com o mesmo nome é substituído. Deixe o valor vazio para acrescentar o registro DKIM depois: rode **Configurar e-mail** de novo, com o mesmo provedor, quando você o tiver. Depois termine a configuração no provedor (Google: **Start authentication**, iniciar autenticação; Microsoft e Zoho: ative ou verifique o DKIM).

Rodar **Configurar e-mail** de novo é seguro: os registros que já estão certos aparecem como "sem alteração".

Cada registro que o Nefo acrescenta, altera ou remove é gravado no log de atividades, como as outras alterações de DNS.

## Limpar o cache do Cloudflare {#purge-cloudflare-s-cache}

Com o proxy ativado, o Cloudflare guarda cópias dos seus arquivos. Depois de alterá-los, clique em **Limpar o cache** na página DNS:

- **Tudo** esvazia o cache do Cloudflare para a zona inteira. O Cloudflare então busca todos os arquivos no servidor de novo, por isso os sites podem ficar mais lentos por alguns minutos;
- **Estas URLs** limpa até 30 endereços da zona de cada vez, um por linha, como `https://example.com/style.css`.

Este é o cache do Cloudflare, não o [cache de página](./websites#page-cache) do próprio site neste servidor.

## Log de atividades {#activity-log}

Cada alteração fica registrada no log de atividades (**Conta** → **Atividade recente**): "Registro DNS adicionado", "Registro DNS alterado", "Registro DNS excluído" e "Cache do Cloudflare limpo", com o tipo, o nome e o conteúdo do registro. **Apontar o DNS para cá…** registra cada registro que altera.

## Mensagens {#messages}

| Mensagem | O que fazer |
|---|---|
| `No Cloudflare API token is saved.` ("Nenhum token de API do Cloudflare está salvo.") | Salve um token em **Configurações** → **Cloudflare**. |
| `Cloudflare refused: the API token lacks the permission …` ("O Cloudflare recusou: falta ao token de API a permissão …") | Acrescente ao token a permissão que ela cita: veja [O token de API](#the-api-token). |
| `Cloudflare did not accept this token.` ("O Cloudflare não aceitou este token.") | O token está errado, expirou ou foi excluído. Crie um novo. |
| `None of these names is in a Cloudflare zone the API token can see` ("Nenhum destes nomes está em uma zona do Cloudflare que o token de API vê") | Acrescente a zona aos **Zone Resources** do token, ou crie os registros onde o DNS do domínio está hospedado. |
| `Cloudflare does not answer for example.com yet (pending).` ("O Cloudflare ainda não responde por example.com (pending).") | Troque os servidores de nomes do domínio, no seu registrador, pelos dois que o Cloudflare mostra. |
| `Cloudflare refused: …` ("O Cloudflare recusou: …") | A própria mensagem do Cloudflare, por exemplo quando um CNAME e um registro A teriam o mesmo nome. |
