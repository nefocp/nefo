---
title: Segurança
description: Como o Nefo protege o seu próprio login, e como usar a verificação de segurança, o firewall, o fail2ban, as configurações de SSH, o firewall de aplicação web, o bloqueio por país, o bloqueio de arquivos e o verificador de malware.
---

# Segurança

O Nefo roda como root, por isso protege antes de tudo o seu próprio login: um endereço secreto em uma porta aleatória, HTTPS desde a primeira inicialização, autenticação de dois fatores, limites de login e um log de atividades. Ele também gerencia o firewall do servidor (ufw), o fail2ban e as configurações de SSH com proteção contra bloqueio, e protege os sites com um firewall de aplicação web, limites de requisições, endereços bloqueados e um verificador de malware.

Esta página trata de:

- a proteção do próprio painel: o endereço oculto, o HTTPS, os limites de login, a autenticação de dois fatores, as sessões e o log de atividades;
- a [verificação de segurança](./security-check), que lista as configurações que deixam o servidor exposto e onde corrigir cada uma;
- o firewall do servidor (ufw), o fail2ban e as configurações de SSH, na página **Firewall e SSH**;
- a segurança dos sites: o firewall de aplicação web, os limites de requisições, os endereços bloqueados, o bloqueio por país, o bloqueio de arquivos e o verificador de malware.

O Nefo está em beta e ainda não passou por uma auditoria de segurança externa. Veja a [política de segurança](../security-policy).

## Verificação de segurança {#security-check}

A **Verificação de segurança**, no grupo **Segurança** do menu à esquerda, roda 22 verificações rápidas do servidor, como o login por senha no SSH, o firewall, as atualizações de segurança pendentes, os bancos de dados abertos para a internet, as portas do Docker, os arquivos que um site serve por engano e os certificados. Cada ocorrência tem um link para a página que a corrige. Ela roda quando você pede e uma vez por dia. Veja [Verificação de segurança](./security-check).

## A proteção do próprio painel {#the-panel-s-own-protection}

### Caminho de entrada oculto {#hidden-entrance-path}

O instalador escolhe uma porta aleatória entre 20000 e 49999 e um caminho secreto de 12 letras minúsculas e dígitos. O painel só responde sob esse caminho, por exemplo `https://203.0.113.10:28443/k3x9p2qa7m1z/`. Qualquer outra URL nessa porta retorna `404`, por isso um scanner não consegue saber que há um painel rodando ali.

Se você perder o endereço, rode isto no servidor:

```sh
nefo info
```

Ele mostra a URL com o primeiro endereço IP do servidor, e também com o domínio do painel, quando ele tiver um (veja abaixo). Se o servidor estiver atrás de um endereço público separado, rode `nefo info --host 203.0.113.10` para mostrar a URL com ele.

A porta e o caminho ficam guardados como `listen` e `entrance` em `/etc/nefo/config.json`. O painel não tem uma configuração para mudá-los. Para mudar o caminho manualmente, defina `entrance` com 8 a 32 letras minúsculas e dígitos e depois rode `systemctl restart nefo`.

### HTTPS desde a primeira inicialização {#https-from-the-first-start}

O painel serve HTTPS desde a primeira inicialização, com TLS 1.2 ou mais recente. Ele cria um certificado autoassinado em `/var/lib/nefo/tls/`, por isso o seu navegador avisa na primeira vez. Para se livrar do aviso, obtenha um certificado Let's Encrypt gratuito para um domínio que aponte para este servidor:

1. Instale o servidor web em **Sites**, se ainda não o fez. O Let's Encrypt confere o domínio na porta 80, e o servidor web responde.
2. Aponte o registro A do domínio para o servidor, sem proxy nem CDN.
3. Vá até **Configurações** → **Certificado**.
4. Informe o **Domínio do painel**, por exemplo `panel.example.com`, e selecione **Obter certificado**.
5. Abra o painel em `https://panel.example.com:28443/k3x9p2qa7m1z/`, com a sua própria porta e o seu próprio caminho.

O certificado se renova sozinho, como os certificados dos sites; uma renovação que falha é tentada de novo a cada 12 horas. Para removê-lo, selecione **Excluir**: o painel volta ao seu certificado autoassinado. Veja [Certificados SSL](./ssl) para os certificados em geral.

### Limites de login {#sign-in-limits}

- 5 tentativas malsucedidas em 15 minutos bloqueiam o endereço por 15 minutos. A página de login então diz "Tentativas malsucedidas demais. Tente de novo em … segundos."
- Uma senha errada e um código de dois fatores errado contam os dois. Uma senha atual errada nos formulários de senha e de 2FA da página **Conta** também conta.
- Os endereços IPv6 são contados por rede /64, porque um usuário geralmente controla uma /64 inteira.
- Um login bem-sucedido zera a contagem.
- Um nome de usuário errado e uma senha errada dão a mesma mensagem e levam o mesmo tempo, por isso a página de login não revela quais nomes de usuário existem.
- O painel conta o endereço que se conecta a ele e ignora o `X-Forwarded-For`. Se você colocar um proxy na frente do painel, todos os visitantes compartilham uma única contagem.

As senhas são guardadas como hashes bcrypt, nunca em texto puro.

::: tip
Se você se trancar do lado de fora, espere 15 minutos. A contagem fica na memória, por isso reiniciar o painel no servidor (`systemctl restart nefo`) também a zera.
:::

### Sessões e encerramento da sessão {#sessions-and-signing-out}

- Um login dura 12 horas. Depois disso, você faz login de novo, mesmo que estivesse ativo.
- Para encerrar a sessão, abra o menu da conta no canto superior direito e selecione **Sair**.
- Trocar a sua senha encerra a sua sessão em todos os dispositivos. O `nefo reset-password` no servidor também.
- O cookie de sessão não pode ser lido por scripts, só é enviado por HTTPS e só para o próprio caminho do painel.

### Cabeçalhos de segurança {#security-headers}

Toda página é enviada com uma Content Security Policy rígida, que só permite scripts do próprio painel. O painel também envia `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer` (que mantém o caminho secreto fora dos logs de outros sites) e uma `Permissions-Policy` que desativa a câmera, o microfone e a localização. Toda alteração precisa de um cabeçalho de requisição que outros sites não conseguem enviar, o que bloqueia a falsificação de requisições entre sites (CSRF).

## Autenticação de dois fatores {#two-factor-authentication}

A autenticação de dois fatores (2FA) pede um código de 6 dígitos de um app autenticador a cada login, para que uma senha roubada não baste. Qualquer app que aceite os códigos padrão baseados em tempo funciona, como Google Authenticator, Authy ou 2FAS. Enquanto o 2FA estiver desativado, o painel lista "O login com dois fatores está desativado" como um problema sério.

### Ativar o 2FA {#turn-on-2fa}

1. Vá até **Conta** no menu à esquerda. Você também pode abrir o menu da conta e selecionar **Senha e 2FA**.
2. Em **Autenticação de dois fatores**, selecione **Configurar**.
3. Leia o QR code com o seu app, ou digite nele a **Chave de configuração**. O app lista a conta como Nefo.
4. Informe o código de 6 dígitos que o app mostra e selecione **Ativar**.

A partir de agora, a página de login pede um **Código de dois fatores** depois da senha.

Os códigos mudam a cada 30 segundos. O painel também aceita o código anterior e o seguinte, caso os relógios tenham uma pequena diferença. Cada código só funciona uma vez: se um código for recusado, espere o próximo.

### Desativar o 2FA {#turn-off-2fa}

Em **Conta**, em **Autenticação de dois fatores**, informe a sua **Senha** e um **Código** atual e selecione **Desativar**.

### Se você perder o celular {#if-you-lose-your-phone}

O Nefo não tem códigos de backup. Você recupera o acesso pelo servidor: entre por SSH como root e rode:

```sh
nefo reset-password --disable-2fa
```

Isso define uma nova senha aleatória para a conta `admin`, desativa o 2FA, encerra todas as sessões e mostra a nova senha. Faça login com ela, troque a senha e configure o 2FA de novo. Veja [Linha de comando](./cli).

::: warning
Sem o seu celular, o acesso root ao servidor é o único caminho de volta ao painel. Guarde bem a sua chave SSH ou a senha do root, ou o console web do seu provedor.
:::

## Trocar a senha do painel {#change-the-panel-password}

1. Vá até **Conta** → **Trocar senha**.
2. Informe a sua **Senha atual** e depois a **Nova senha** duas vezes (**Repita a nova senha**). Ela precisa ter de 12 a 72 caracteres.
3. Selecione **Trocar senha**.

A sua sessão é encerrada em todos os dispositivos. Faça login de novo com a nova senha.

## Log de atividades {#activity-log}

**Conta** → **Atividade recente** mostra as 100 entradas mais recentes, com o horário, a ação, o usuário, o endereço IP e o detalhe. Mostre **Todos** os registros ou só as **Falhas**, e digite na caixa de filtro para pesquisar. O topo da página mostra o seu último login e quantos logins malsucedidos o log contém.

O log registra:

- logins, encerramentos de sessão e tentativas de login malsucedidas, com o motivo (nome de usuário ou senha errados, ou código de 2FA errado);
- trocas de senha, redefinições de senha pela linha de comando e o 2FA ativado ou desativado;
- as alterações feitas no painel: sites, certificados, bancos de dados, arquivos, backups, tarefas cron, serviços, regras de firewall, configurações e chaves SSH, software e Docker;
- a abertura e o fechamento do terminal, com quanto tempo ele ficou aberto;
- as reinicializações do painel e do servidor.

As renovações automáticas de certificados são registradas com o usuário `system`. As entradas são mantidas por 90 dias. Cada entrada também é gravada no log do serviço do painel, que você pode ler com `journalctl -u nefo`. O fail2ban lê as tentativas de login malsucedidas de lá.

## Firewall (ufw) {#firewall-ufw}

O Nefo gerencia o firewall do servidor com o ufw. Vá até **Firewall e SSH** → **Firewall (ufw)**.

Se a página disser que o ufw não está instalado, instale-o no servidor. O painel não o instala:

```sh
apt install ufw
```

### Ativar o firewall {#turn-on-the-firewall}

1. Selecione **Ativar o firewall** e confirme.
2. O painel primeiro libera a porta do SSH (comentário `SSH`) e a porta do painel (comentário `Nefo`). Quando há um servidor web instalado, ele também libera as portas 80 e 443 para os sites (comentário `Websites`), e a 443/UDP quando o OpenLiteSpeed responde por HTTP/3. Depois ele ativa o ufw. Ele acrescenta essas regras mesmo que você tenha acrescentado uma regra para uma dessas portas enquanto o firewall estava desativado, por exemplo uma limitada ao seu endereço, para que ativar o firewall nunca deixe você trancado do lado de fora. Para manter só a sua regra, exclua depois a regra para todos (veja o final de [O que o firewall recusa](#what-the-firewall-refuses)).

Você pode [acrescentar regras](#add-a-rule) antes de ativar o firewall, por exemplo para uma porta de que outro programa do servidor precisa. Enquanto o firewall está desativado, a página as mostra embaixo de **Ativar o firewall** como comandos do ufw, como `ufw allow 8080/tcp`, na coluna **Regra (comando do ufw)**, com cada par IPv4 e IPv6 mostrado uma única vez. Elas passam a valer quando você ativa o firewall. O ufw só numera as regras enquanto está ativado, por isso, para excluir uma delas, ative o firewall antes.

Todas as outras portas de entrada ficam fechadas, a menos que uma regra as libere. Quando você instala o servidor web depois, com o firewall ativado, o Nefo abre as portas 80 e 443 nesse momento.

Se o seu provedor tem um firewall de nuvem (security group), abra as mesmas portas lá também. O Nefo não consegue alterá-lo.

### Acrescentar uma regra {#add-a-rule}

1. Selecione **Adicionar regra**.
2. Escolha **Permitir** ou **Negar**.
3. Em **Porta, por ex. 8080**, informe uma porta, ou um intervalo como `6000:6010`.
4. Escolha o **Protocolo**: TCP, UDP ou TCP + UDP. Um intervalo precisa de um único protocolo: com TCP + UDP, ele vira TCP.
5. Em **Origem**, informe um endereço IP ou uma rede como `198.51.100.0/24`. Deixe vazio para permitir ou negar todo mundo.
6. Se quiser, acrescente um **Comentário** de até 60 caracteres, sem aspas.
7. Selecione **Adicionar**.

A página mostra o seu próprio endereço IP acima da lista, o que ajuda quando você libera uma porta só para você.

O ufw confere as regras de cima para baixo e usa a primeira que corresponder. Uma regra **Negar** com um endereço em **Origem** é colocada no topo da lista, para que ela valha mesmo em uma porta que uma regra posterior libera para todos. As outras regras são acrescentadas no final.

::: tip
Para manter um endereço longe de um único site, use as configurações de **Bloquear acesso** do site (veja abaixo).
:::

A lista também mostra as regras que você acrescentou com o ufw na linha de comando. **Limitar** libera uma porta, mas recusa por um tempo um endereço que abrir 6 conexões em 30 segundos (`ufw limit`). **Rejeitar** recusa como **Negar**, mas avisa o outro lado na hora.

### Excluir uma regra {#delete-a-rule}

Selecione o ícone da lixeira no final da linha da regra e confirme. Se as regras mudaram desde que você abriu a página, o painel recusa e pede que você recarregue, para nunca excluir a regra errada.

### O que o firewall recusa {#what-the-firewall-refuses}

Estas proteções impedem que você se tranque do lado de fora:

- Uma regra que libera a porta do SSH ou a porta do painel mostra **protegida** quando ela é o seu último meio de entrar a partir do endereço que você usa agora (mostrado acima da lista). Ela não pode ser excluída aqui. Quando outra regra também deixa o seu endereço entrar, a regra pode ser excluída.
- Uma regra **Negar** para a porta do SSH ou a porta do painel com a **Origem** vazia é recusada. Uma regra **Negar** para um endereço ou uma rede é permitida.

Para deixar só o seu próprio endereço chegar ao painel: acrescente uma regra **Permitir** para a porta do painel com o seu endereço em **Origem** e depois exclua a regra que libera a porta para qualquer lugar (e a cópia IPv6 dela). Se o seu endereço mudar depois, use o console web do seu provedor para rodar `ufw allow <port>/tcp`.

As regras de acesso remoto aos bancos de dados (a porta 3306 para o MariaDB e o MySQL, a porta do cluster, 5432 por padrão, para o PostgreSQL) são acrescentadas e removidas pela página **Bancos de dados**. Altere-as lá. Veja [Bancos de dados](./databases#allow-remote-access) e [PostgreSQL](./postgresql#postgresql-remote-access).

## Proteção contra força bruta (fail2ban) {#brute-force-protection-fail2ban}

O fail2ban bloqueia os endereços que continuam errando o login, no SSH e no painel.

### Instalar o fail2ban {#install-fail2ban}

1. Vá até **Firewall e SSH** → **Proteção contra força bruta (fail2ban)**.
2. Selecione **Instalar fail2ban**. A instalação roda em segundo plano, com um log ao vivo.

O Nefo configura duas jails em `/etc/fail2ban/jail.d/panel.conf`:

| Jail | Vigia | Bloqueia depois de | Tempo de bloqueio |
|---|---|---|---|
| `sshd` | Os logins SSH, inclusive o SFTP | 5 falhas em 10 minutos | 1 hora |
| `panel` | Os logins no painel que falham, ou que são recusados enquanto o endereço está bloqueado | 10 em 10 minutos | 1 hora |

Um endereço bloqueado não consegue chegar à porta que a jail protege. O próprio limite de login do painel (5 falhas em 15 minutos) age primeiro. Um endereço que continua tentando enquanto está bloqueado é então bloqueado no firewall pela jail `panel`.

O painel reescreve esse arquivo, e o filtro dele em `/etc/fail2ban/filter.d/panel.conf`, por isso não os edite manualmente. Quando você muda a porta do SSH, a jail `sshd` a acompanha.

### Ver e desbloquear endereços {#see-and-unblock-addresses}

A tabela lista cada **Serviço protegido**, as **Falhas recentes** dele e os endereços **Bloqueados agora**, com o número total de bloqueios. Para desbloquear um endereço, selecione **Desbloquear** ao lado dele.

Se o fail2ban estiver instalado, mas parado, inicie-o na página **Serviços**.

::: tip
Se você bloqueou a si mesmo no SSH, faça login no painel a partir da sua rede atual e selecione **Desbloquear**. Você também pode rodar `fail2ban-client set sshd unbanip 203.0.113.10` no **Terminal** do painel.
:::

## Configurações de SSH {#ssh-settings}

Vá até **Firewall e SSH** → **SSH**.

| Configuração | Opções |
|---|---|
| **Porta** | 1 a 65535 |
| **Login do root** | **Só com chave SSH (recomendado)**, **Com chave ou senha**, **Não permitido** |
| **Permitir login com senha** | Ativado ou desativado, para todas as contas |

Selecione **Salvar**. O painel grava as suas configurações em `/etc/ssh/sshd_config.d/00-panel.conf`, que tem prioridade sobre os outros arquivos de configuração do SSH. Ele também desativa o login keyboard-interactive. Antes de reiniciar o SSH, ele testa as novas configurações com `sshd -t`. Se o teste falhar, o arquivo antigo volta e nada muda.

::: warning
Mantenha a sua sessão SSH atual aberta depois de salvar. Abra uma nova conexão e confira se você consegue entrar antes de fechar a antiga.
:::

### O que as configurações de SSH recusam {#what-the-ssh-settings-refuse}

- Configurações com as quais ninguém conseguiria mais entrar por SSH. Por exemplo: desativar o login por senha enquanto nenhuma conta tem uma chave SSH; o login do root **Só com chave SSH** enquanto o root não tem chave; ou o root **Não permitido** enquanto nenhuma outra conta consegue entrar.
- Uma porta que já está ocupada: a própria porta do painel, a 80, a 443 ou qualquer porta em que outro programa escuta.
- Uma nova porta quando o SSH é iniciado pela ativação por socket do systemd. A página então mostra **ativado por socket** e o campo da porta fica bloqueado. Mude a porta pela linha de comando.

Para decidir se alguém ainda conseguiria entrar, o painel olha o root e as contas de pessoas (UID 1000 e acima) com um shell de login: as chaves delas em `~/.ssh/authorized_keys` (ou nos arquivos definidos por `AuthorizedKeysFile`), as senhas delas, e `AllowUsers`, `DenyUsers`, `AllowGroups` e `DenyGroups`. Ele não lê os blocos `Match`. A página lista quem consegue entrar agora, e como (chave ou senha). Quando o root não tem chave, ela também mostra um aviso com um botão **Adicionar chave** (veja [Chaves SSH](#ssh-keys)).

O SSH não pode ser parado pela página **Serviços**, só reiniciado ou recarregado.

::: warning
Com o login do root **Não permitido**, você precisa de outra conta que possa usar o `sudo`, ou vai conseguir entrar, mas não administrar o servidor por SSH. O Nefo não cria essa conta, e não confere se ela pode usar o `sudo`.
:::

### Mudar a porta do SSH {#change-the-ssh-port}

1. Informe a nova **Porta**, selecione **Salvar** e confirme.
2. Se o firewall estiver ativado, o painel abre a nova porta (comentário `SSH`) antes de o SSH passar para ela. Se o fail2ban estiver instalado, a jail `sshd` dele também passa para a nova porta.
3. Se o seu provedor tiver um firewall de nuvem, abra a nova porta lá também.
4. Teste uma nova conexão, por exemplo `ssh -p 2222 root@203.0.113.10`.
5. A regra de firewall da porta antiga fica, e deixa de ser protegida. Exclua-a em **Firewall (ufw)** quando a nova porta funcionar.

As contas SFTP dos seus sites se conectam pela mesma porta.

### Chaves SSH {#ssh-keys}

**Chaves SSH**, embaixo das configurações de SSH, lista as chaves públicas do root e das contas de pessoas (UID 1000 e acima, com um shell de login), agrupadas por conta. O Nefo as lê de `~/.ssh/authorized_keys`, ou dos arquivos definidos por `AuthorizedKeysFile`. Cada chave mostra o tipo e o tamanho (como `ed25519` ou `rsa 4096`), o comentário, a impressão digital SHA256 e as opções, se tiver alguma, como `from="…"` ou `command="…"`. Uma chave DSA, ou uma chave RSA com menos de 2048 bits, fica marcada como **fraca**: o OpenSSH atual as recusa. Acrescente uma nova chave e depois remova a fraca.

As contas SFTP dos seus sites não aparecem: as chaves delas ficam nas configurações SFTP de cada site.

#### Acrescentar uma chave {#add-a-key}

1. No seu computador, crie um par de chaves, se ainda não tiver um. Isto funciona do mesmo jeito no PowerShell do Windows 10 e posteriores:
   ```sh
   ssh-keygen -t ed25519
   ```
   A chave privada vai para `~/.ssh/id_ed25519` e fica no seu computador. A chave pública vai para `~/.ssh/id_ed25519.pub`.
2. Mostre a chave pública e copie a linha inteira, que começa com `ssh-ed25519`:
   ```sh
   cat ~/.ssh/id_ed25519.pub
   ```
3. No painel, em **Chaves SSH**, selecione **Adicionar chave**.
4. Escolha a **Conta**, cole a linha em **Chave pública** e selecione **Adicionar chave**.
5. Teste-a em um novo terminal, por exemplo `ssh root@203.0.113.10`.

O Nefo acrescenta a chave no final do `~/.ssh/authorized_keys` da conta. Quando a pasta ou o arquivo não existem, ele os cria, pertencentes à conta, com os modos 700 e 600. Ele recusa:

- uma chave privada, como uma que começa com `-----BEGIN OPENSSH PRIVATE KEY-----`. Se você colou uma chave privada em algum lugar por engano, crie um novo par de chaves;
- mais de uma linha. No PuTTYgen, copie a caixa **Public key for pasting into OpenSSH authorized_keys file** (chave pública para colar no arquivo authorized_keys do OpenSSH);
- chaves DSA, e chaves RSA com menos de 2048 bits;
- uma chave que a conta já tem;
- um arquivo `authorized_keys` ou uma pasta `.ssh` que seja um link simbólico, ou que não seja um arquivo ou uma pasta comum. Uma conta poderia apontá-lo para outro arquivo do servidor, por isso o Nefo não o lê nem o altera.

#### Remover uma chave {#remove-a-key}

Selecione o ícone da lixeira ao lado da chave e confirme. A confirmação identifica a chave pelo tipo, pela impressão digital e pelo comentário. Se ninguém conseguiria entrar por SSH sem essa chave, com as configurações atuais, o painel recusa: acrescente outra chave antes, ou permita o login com senha.

As chaves acrescentadas e removidas ficam registradas no log de atividades, com a conta e a impressão digital da chave.

### Desativar o login por senha {#turn-off-password-sign-in}

1. Em [Chaves SSH](#ssh-keys), acrescente a sua chave pública para o root, ou para a conta com que você entra.
2. Abra um novo terminal e entre com a chave. Se o SSH pedir uma senha, a chave não está sendo usada: confira se você a acrescentou à conta com que você entra.
3. Desative **Permitir login com senha** e selecione **Salvar**. Se ninguém tiver uma chave, o painel recusa (veja [O que as configurações de SSH recusam](#what-the-ssh-settings-refuse)).
4. Mantenha a sua sessão atual aberta, e confira uma nova conexão mais uma vez antes de fechá-la.

As contas SFTP dos seus sites mantêm o seu próprio método de login, senha ou chave, mesmo com o login por senha desativado para o SSH.

Enquanto o root puder entrar com senha, o painel lista "O root pode entrar por SSH com senha" como um problema sério.

## Segurança dos sites {#website-security}

Cada site tem as suas próprias configurações de segurança. Vá até **Sites**, abra o site selecionando o nome dele ou **Configurações**, e use as abas em **Segurança**.

### Firewall de aplicação web {#web-application-firewall}

O firewall de aplicação web (WAF) é o ModSecurity com o OWASP Core Rule Set, ambos gratuitos e de código aberto. Ele confere cada requisição e impede SQL injection, cross-site scripting (XSS), inclusão de arquivos, scanners conhecidos e outros ataques comuns. Ele funciona com o Nginx, o Apache e o OpenLiteSpeed.

1. Instale-o uma vez: **Software** → **Firewall web (ModSecurity)** → **Instalar**. Os sites mantêm o firewall desativado até você ativá-lo. Onde a distribuição não tem módulo ModSecurity para o servidor web, o cartão avisa e não oferece instalação: o Ubuntu 22.04 não tem um para o Nginx, enquanto o Apache e o OpenLiteSpeed funcionam lá, e o Nginx funciona a partir do Ubuntu 24.04.
2. Vá até **Sites** → (site) → **Firewall web (WAF)**.
3. Escolha um modo:
   - **Desativado**
   - **Só detectar: registrar os ataques, não bloquear nada**
   - **Bloquear ataques**: o visitante recebe HTTP 403.
4. Selecione **Salvar**.

Comece só detectando por alguns dias, e confira os eventos em busca de alarmes falsos antes de bloquear. Os sites que rodam o WordPress ou usam a predefinição de reescrita do WordPress ou do Drupal recebem automaticamente as exceções do conjunto de regras para essas aplicações.

**Eventos recentes do firewall** lista as 50 requisições mais recentes que as regras marcaram neste site: o horário, o endereço IP, a requisição, as regras que corresponderam e se ela foi **bloqueada** ou só **registrada**. Os eventos vêm de `/var/log/panel-waf/audit.log`, que é rotacionado todo dia e mantido por 7 dias.

Só os primeiros 13 MB de uma requisição são inspecionados, por isso os uploads grandes continuam funcionando. Se você remover o WAF na página **Software**, cada site mantém o seu modo, que volta a valer quando você reinstalar.

### Limite de requisições e endereços bloqueados {#rate-limit-and-blocked-addresses}

Vá até **Sites** → (site) → **Bloquear acesso**.

- **Limite de requisições (por segundo, por visitante, 0 = desativado)**: conta só as requisições de PHP e de app, não as imagens, o CSS ou os scripts. Rajadas de até cinco vezes o limite são permitidas; acima disso, os visitantes recebem HTTP 429. 10 serve para a maioria dos sites, e o valor mais alto é 1000. Só o Nginx tem essa configuração: no Apache e no OpenLiteSpeed ela não aparece.
- **Endereços IP bloqueados**: um endereço ou faixa IPv4 ou IPv6 por linha, como `203.0.113.7` ou `198.51.100.0/24`, até 200 por site. Os visitantes bloqueados recebem HTTP 403.

Selecione **Salvar**. Os dois funcionam sozinhos, sem o firewall de aplicação web, e em todos os tipos de site, inclusive os de proxy reverso. A mesma aba tem a **Proteção por senha**, descrita em [Sites](./websites#suspend-redirect-or-password-protect-a-website).

### Verificação de malware {#malware-scan}

O verificador procura web shells, backdoors, código oculto e scripts injetados nos arquivos de um site. Ele não muda nada até você pôr um arquivo em quarentena.

1. Vá até **Sites** → (site) → **Verificação de malware**.
2. Selecione **Verificar agora**. A verificação roda em segundo plano, com um log ao vivo.

O verificador lê a pasta do site, como `/var/www/example.com`. Ele pula as pastas `.git` e os arquivos com mais de 5 MB, e não segue links simbólicos. Ele confere os arquivos PHP (também o código PHP em arquivos com outros nomes), os arquivos JavaScript e HTML, e as imagens.

| Ocorrência | Gravidade |
|---|---|
| Web shell conhecido | crítico |
| Executa código enviado pelo visitante | crítico |
| Chama uma função cujo nome o visitante escolhe | crítico |
| Executa comandos de shell enviados pelo visitante | crítico |
| Executa código oculto (codificado) | alto |
| Decodifica dados ocultos enviados pelo visitante | alto |
| Monta uma função a partir do que o visitante envia | alto |
| Uploader que deixa o visitante escolher para onde vão os arquivos | alto |
| Injeção de script ofuscado | alto |
| Código PHP escondido em uma imagem | alto |
| Execução de código por preg_replace /e | verificar |
| Dados codificados longos ao lado de código que os decodifica | verificar |
| Texto muito ofuscado ao lado de código que o executa | verificar |
| Arquivo PHP em uma pasta de uploads | verificar |

Cada ocorrência mostra o arquivo, a linha e um trecho curto. Um arquivo PHP em uma pasta de uploads que só contém um comentário, como o `index.php` "Silence is golden" do WordPress, não é relatado.

O painel guarda a última verificação de cada site. Ele não verifica com agendamento: inicie uma verificação quando quiser.

### Quarentena e restauração {#quarantine-and-restore}

Selecione **Pôr em quarentena** ao lado de uma ocorrência para tirar o arquivo do site e movê-lo para `/var/lib/panel-quarantine`, que só o root pode ler. O painel registra o proprietário e as permissões do arquivo.

Em **Arquivos em quarentena**:

- **Restaurar** coloca o arquivo de volta onde estava, com o proprietário e as permissões dele. É recusado se agora existir um arquivo nesse caminho, ou se a pasta não existir mais.
- **Excluir** remove o arquivo para sempre, depois que você confirma.

Quando você exclui um site, os arquivos em quarentena dele também são excluídos.

::: warning
Pôr em quarentena um arquivo de que o site precisa, como o arquivo de um plugin, pode quebrar o site. Confira o trecho antes, e restaure o arquivo se for um alarme falso.
:::

### Verificar com o ClamAV {#scan-with-clamav}

O ClamAV é um mecanismo de verificação extra e opcional.

1. Instale o **Antivírus ClamAV** na página **Software**. Os bancos de dados de vírus dele são baixados em segundo plano por alguns minutos depois da instalação.
2. Em **Verificação de malware**, marque **Verificar também com o ClamAV (mais lento, precisa de cerca de 1,4 GB de memória livre)** e selecione **Verificar agora**.

O ClamAV precisa de cerca de 1,4 GB de memória livre durante a verificação, por isso ele serve para servidores com 2 GB ou mais. Se houver menos memória livre, o painel pula o ClamAV e avisa no relatório. O ClamAV roda com a menor prioridade de CPU e pula os arquivos com mais de 25 MB. As ocorrências dele são marcadas como alto.

### Bloqueio por país e bloqueio de arquivos {#country-blocking-and-file-lock}

Cada site também tem o **Bloqueio por país**, que recusa os visitantes dos países que você listar ou deixa entrar só esses, e o **Bloqueio de arquivos**, que deixa o código do site somente leitura para o PHP e a conta SFTP dele, exceto as pastas em que ele precisa gravar. Veja [Bloqueio por país e de arquivos](./website-protection).

## Histórico de logins SSH {#ssh-sign-in-history}

A lista de logins SSH e de tentativas malsucedidas fica na página **Processos**. Veja [Logins SSH](./system#ssh-sign-ins). Para receber uma mensagem quando alguém fizer login por SSH ou no painel, ou quando os logins SSH continuarem falhando, veja [Alertas de login](./mail-alerts#sign-in-alerts).
