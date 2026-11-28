---
title: Solução de problemas
description: Correções para os problemas mais comuns com o Nefo, de um painel que você não consegue acessar a erros de certificado e sites que mostram 502 ou 503.
---

# Solução de problemas

A maioria dos problemas com o Nefo tem uma correção curta no servidor, como root: `nefo info` mostra um endereço do painel perdido, `nefo reset-password` define uma nova senha (com `--disable-2fa`, também desativa o login com dois fatores) e `journalctl -u nefo` mostra por que o painel não inicia. Esta página também trata de erros de certificado, de sites que mostram 502 ou 503 e de restaurações que falham.

A maioria das correções precisa de acesso SSH ao servidor como root. Se o SSH também não funcionar, use o console web no painel de controle do seu provedor.

## Você não consegue abrir o painel {#you-can-t-open-the-panel}

### Você esqueceu o endereço {#you-forgot-the-address}

Rode isto no servidor:

```sh
nefo info
```

A saída mostra a URL completa, com a porta e o caminho secreto: com o domínio do painel, se você deu um a ele, e com o primeiro endereço IP do servidor. Se esse endereço for privado, acrescente `--host` e o seu IP público, como `nefo info --host 203.0.113.10`.

### A página mostra 404 {#the-page-shows-404}

O painel só responde sob o seu caminho secreto, como `https://203.0.113.10:28443/k3x9p2qa7m1z/`. Qualquer outro endereço retorna `404`, inclusive o endereço sem o caminho. Confira o caminho com `nefo info`.

Confira também se o endereço começa com `https://`. Com `http://`, o painel responde com um erro em vez da página de login.

### A página não carrega de jeito nenhum {#the-page-doesn-t-load-at-all}

Quando o navegador não consegue se conectar, ou espera e depois desiste, o painel não está rodando ou algo bloqueia a conexão.

1. **Confira se o painel está rodando:**

   ```sh
   systemctl status nefo
   ```

   Se ele não estiver `active (running)`, veja [O painel não inicia](#the-panel-doesn-t-start).

2. **Confira o firewall de nuvem do seu provedor.** Ele precisa liberar a porta TCP do painel. Descubra a porta com `nefo info --port`.

3. **Confira se o seu endereço IP ainda está liberado.** Se você limitou a porta do painel ao seu próprio endereço IP, no ufw ou no firewall de nuvem, e o seu endereço IP mudou (comum em conexões residenciais e móveis), libere o novo. No ufw, com o seu novo endereço e a porta do painel:

   ```sh
   ufw allow from 198.51.100.7 to any port 28443 proto tcp
   ```

4. **Confira se o fail2ban bloqueou você.** Se você instalou o fail2ban pelo painel, ele bloqueia um endereço por uma hora depois de 10 tentativas de login no painel malsucedidas em 10 minutos (5 para o SSH). Desbloqueie o seu endereço:

   ```sh
   fail2ban-client set panel unbanip 198.51.100.7
   ```

   Para o SSH, use a jail `sshd` em vez de `panel`.

### "Tentativas malsucedidas demais" {#too-many-failed-attempts}

Depois de 5 tentativas de login malsucedidas em 15 minutos, o painel recusa novas tentativas do seu endereço por 15 minutos e mostra "Tentativas malsucedidas demais. Tente de novo em … segundos." Códigos de dois fatores errados também contam. Espere o tempo acabar. O bloqueio fica na memória, por isso `systemctl restart nefo` também o desfaz.

### O painel não inicia {#the-panel-doesn-t-start}

Leia o motivo no log:

```sh
journalctl -u nefo -n 50 --no-pager
```

| O log diz | O que fazer |
|---|---|
| `address already in use` | Outro programa usa a porta do painel. Pare esse programa, ou mude a porta em `/etc/nefo/config.json` (veja [Linha de comando](./cli#the-configuration-file)). |
| Um erro sobre `/etc/nefo/config.json` | O arquivo de configuração tem um erro, por exemplo depois que você o editou. Corrija a linha que a mensagem cita. |
| `database schema version … is newer than this panel supports` | Uma versão mais antiga do programa está rodando com dados de uma mais nova. Instale a versão mais recente de novo (veja [Atualização](./update#if-an-update-fails)). |

Depois da correção, rode `systemctl restart nefo`.

## Você esqueceu a senha {#you-forgot-the-password}

Defina uma nova senha aleatória no servidor:

```sh
nefo reset-password
```

Ele mostra a nova senha e encerra a sessão da conta em todos os lugares. Faça login com ela e depois escolha a sua própria senha em **Conta**.

## Perdeu o celular do 2FA {#lost-your-2fa-phone}

Se você não consegue mais obter códigos do seu app autenticador, desative a autenticação de dois fatores pelo servidor. Isso também define uma nova senha:

```sh
nefo reset-password --disable-2fa
```

Faça login com a nova senha e depois configure a autenticação de dois fatores de novo em **Conta**, com o seu novo celular.

### Os códigos são sempre "não válidos" {#codes-are-always-not-valid}

O código depende da hora. O painel aceita o código atual e os imediatamente anterior e posterior, e cada código só uma vez. Se todos os códigos falham ("Esse código não é válido."), confira se o relógio do servidor está certo:

```sh
timedatectl
```

`System clock synchronized` deve dizer `yes`. Confira também a hora no seu celular.

## O navegador avisa sobre o certificado {#the-browser-warns-about-the-certificate}

Enquanto o painel não tem o seu próprio certificado, ele usa um autoassinado, e todos os navegadores avisam sobre isso. Isso é esperado. Dê ao painel um domínio e um certificado gratuito para que o aviso desapareça (veja [Primeiros passos](./first-steps#give-the-panel-its-own-domain)).

Se o painel tem um certificado e o navegador continua avisando:

- Você abriu o painel pelo endereço IP. O certificado é para o domínio: use `https://panel.example.com:28443/k3x9p2qa7m1z/`.
- O certificado expirou porque não pôde ser renovado. Confira **Configurações** → **Certificado** e clique em **Obter um novo certificado**. As causas são as mesmas da próxima seção.

## Um pedido de certificado falha {#a-certificate-request-fails}

Antes de pedir ao Let's Encrypt, o painel confere se cada domínio chega a este servidor: ele coloca um arquivo de teste no servidor e o busca pelo domínio por HTTP, na porta 80. Se isso falhar, ele não pede nada e lista cada problema em "these names do not reach this server yet, so no certificate was requested" (estes nomes ainda não chegam a este servidor, por isso nenhum certificado foi pedido).

| A mensagem diz | Causa e correção |
|---|---|
| `no DNS record. Add an A record that points to this server's IP address.` | O domínio ainda não tem endereço. Acrescente o registro A no seu provedor de DNS. Um registro novo pode levar um tempo para funcionar em todos os lugares. |
| `no answer over HTTP on port 80 (…). Check the DNS record and the firewall.` | A porta 80 está fechada, ou o domínio aponta para outro servidor. Abra a porta 80 no ufw e no firewall de nuvem do seu provedor, e confira se o servidor web está rodando. |
| `answered HTTP …, but not from this server. Check that the DNS record points here.` | Outra coisa respondeu pelo domínio: o registro aponta para outro servidor, ou há um proxy ou CDN na frente, como o proxy do Cloudflare (a nuvem laranja). |

Cada linha começa com o domínio e os endereços para os quais ele aponta. Se esses não forem os endereços do seu servidor, corrija antes o registro DNS. Se aparecer um endereço IPv6 que não é do seu servidor, corrija ou remova o registro AAAA.

**Atrás do proxy do Cloudflare:**

- Para um site, defina o registro como "Somente DNS" enquanto o certificado é pedido, ou marque **Validar pelo DNS (Cloudflare)** na aba **SSL / HTTPS** do site. A validação por DNS precisa de um token de API do Cloudflare, que você acrescenta em **Configurações** → **Cloudflare**. Os nomes wildcard (`*.example.com`) sempre precisam de validação por DNS.
- Para o próprio domínio do painel, o registro precisa apontar direto para o servidor, sem o proxy.

**Para o próprio domínio do painel,** a aba **Certificado** diz "Isso precisa do servidor web" até você instalar um, porque é o servidor web que responde ao Let's Encrypt na porta 80. Instale-o antes na página **Sites**.

Se a verificação passar, mas o Let's Encrypt ainda recusar, o log da tarefa mostra o motivo dele. O Let's Encrypt limita quantas tentativas malsucedidas você pode fazer, por isso corrija a causa antes de tentar de novo.

Quando uma renovação automática falha, o painel mostra "A renovação do certificado falhou para example.com" e tenta de novo a cada 12 horas. O certificado continua funcionando até expirar.

## Um site mostra um erro {#a-website-shows-an-error}

| O que os visitantes veem | Causa | O que fazer |
|---|---|---|
| O texto "This website is temporarily unavailable." (HTTP 503) | O site está suspenso, manualmente ou porque a data de expiração dele passou. | Abra a aba **Expiração e suspensão** do site. Desmarque **Suspender este site** e apague ou mude a **Data de expiração**. Se a data continuar no passado, o painel suspende o site de novo dentro de uma hora. |
| Uma página "502 Bad Gateway" ou "503 Service Unavailable" do servidor web | O programa por trás do site não responde: o PHP-FPM para um site PHP, ou o app ou o contêiner para um site de app. O Nginx mostra 502; o Apache e o OpenLiteSpeed podem mostrar 503. | Para o PHP, abra **Serviços** e inicie ou reinicie o serviço PHP-FPM da versão do PHP do site, como `php8.3-fpm`. Para um app, o site mostra "App parado": leia os **Logs do app** na aba **Logs** dele. |
| Nenhuma resposta para nenhum site | O servidor web está parado. A página **Sites** diz que ele "não está rodando". | Inicie o servidor web na página **Serviços**. |
| HTTP 429 | O limite de requisições do site está ativado (só no Nginx), e o visitante enviou requisições demais. | Aumente o limite, ou defina-o como 0 para desativá-lo, na aba **Bloquear acesso** do site. |
| HTTP 403 | O endereço do visitante está entre os endereços bloqueados do site, ou o firewall de aplicação web bloqueou a requisição. | Confira **Endereços IP bloqueados** na aba **Bloquear acesso**, e os eventos recentes do firewall na aba **Firewall web (WAF)**. |

Para outros erros, leia o log de erros e os erros do PHP do site na aba **Logs** dele.

## O envio ou a restauração de um backup falha {#a-backup-upload-or-restore-fails}

- **Uma restauração falha com "the passphrase does not unlock it"** (a frase secreta não o desbloqueia). A cópia foi criptografada com uma frase secreta diferente da atual do armazenamento. Restaure de novo e informe essa frase secreta em **Frase secreta da criptografia (opcional)**. Veja [Criptografar as cópias](./backups#encrypt-the-copies).
- **Os envios para o Google Drive falham depois de uma semana.** A OAuth consent screen (tela de consentimento OAuth) ainda está em **Testing** (teste): mude-a para **In production** (em produção) no console do Google Cloud e depois selecione **Alterar** → **Conectar de novo** em **Backups** → **Armazenamento remoto**. Veja [Google Drive](./backups#google-drive).
- **O backup de um site é restaurado como um site estático.** O backup foi feito por uma versão anterior à 0.24.1, que só registra a versão do PHP. Para um site de app ou de proxy reverso, crie o site de novo com o tipo dele antes e depois restaure. Os backups feitos pela 0.24.1 ou posterior trazem o site de volta com o tipo dele. Veja [Restaurar um site que foi excluído](./backups#restore-a-website-that-was-deleted).

## Onde ficam os logs {#where-the-logs-are}

| Log | No servidor | No painel |
|---|---|---|
| O painel | `journalctl -u nefo` | — |
| Os logs de acesso e de erros de um site | `/var/log/nginx/`, `/var/log/apache2/` ou `/var/log/openlitespeed/`, como `example.com.access.log` e `example.com.error.log` | Sites → (site) → **Logs** |
| Os erros do PHP de um site | `/var/www/example.com/logs/php-error.log` | Sites → (site) → **Logs** → **Erros do PHP** |
| Um site de app | `journalctl -u panel-app-example_com` | Sites → (site) → **Logs** → **Logs do app** |
| Tarefas cron | `/var/log/panel-cron/` | **Tarefas cron** |
| Logins e alterações no painel | — | **Conta** → **Atividade recente** |

Quando você relatar um problema, inclua a saída do `nefo version` e as linhas de log relevantes. Remova antes as senhas e o caminho secreto do painel.
