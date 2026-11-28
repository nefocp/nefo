---
title: Verificação de segurança
description: A verificação de segurança do Nefo encontra as configurações que deixam um servidor exposto (SSH, firewall, atualizações, bancos de dados abertos, portas do Docker, arquivos expostos, certificados, PHP) e liga cada ocorrência à sua correção, quando você pedir ou todo dia.
---

# Verificação de segurança

A verificação de segurança, nova na v0.24.0, roda 22 verificações rápidas do servidor: SSH que aceita senhas, um firewall desativado, atualizações de segurança pendentes, um banco de dados ou Redis aberto para a internet, portas do Docker que escapam do ufw, arquivos de sites que qualquer um pode baixar, certificados perto de expirar e configurações do PHP. Cada ocorrência tem uma gravidade e um botão para a página que a corrige.

Ela roda quando você a inicia, e uma vez por dia por padrão. Ela não envia nada para fora do seu servidor. Abra **Verificação de segurança** no grupo **Segurança** do menu à esquerda.

## Rodar a verificação {#run-the-check}

1. Clique em **Executar a verificação**. Isso leva poucos segundos.
2. Leia as ocorrências na aba **A corrigir**. Cada uma tem uma gravidade:
   - **Crítica**: corrija agora. Por exemplo, o root pode fazer login por SSH com senha, ou um site serve um dump de banco de dados.
   - **Aviso**: corrija em breve. Por exemplo, o firewall está desativado ou há atualizações de segurança pendentes.
   - **Nota**: vale a pena saber, muitas vezes está bem como está. Por exemplo, o SSH escuta na porta 22.
3. Clique no botão de uma ocorrência, como **Ir para Firewall e SSH**, para abrir o lugar que a corrige. Depois rode a verificação de novo.

Os contadores no topo mostram quantas ocorrências **Críticas**, **Avisos** e **Notas** ainda estão abertas, e quantas verificações foram **Aprovadas**. Clique em **Aprovadas**, no final da página, para ver cada verificação que não encontrou nada a corrigir. As verificações que não se aplicam ao servidor (sem Redis, sem sites, sem Docker…) aparecem em "Não se aplicam a este servidor".

## O que a verificação de segurança examina? {#what-is-checked}

| Verificação | Gravidade | Onde corrigir |
|---|---|---|
| O root pode fazer login por SSH com senha | Crítica | **Firewall e SSH** → SSH |
| O SSH aceita senhas para outras contas | Aviso | **Firewall e SSH** → SSH |
| O SSH escuta na porta 22 | Nota | **Firewall e SSH** → SSH |
| O firewall (ufw) não está instalado, ou está desativado | Aviso | **Firewall e SSH** |
| O fail2ban não está instalado, não está rodando ou não vigia o SSH (sem a jail `sshd`) | Aviso | **Firewall e SSH** → fail2ban, ou **Serviços** |
| O painel é acessado sem HTTPS | Crítica | `/etc/nefo/config.json` (veja abaixo) |
| O painel usa o seu certificado autoassinado | Nota | **Configurações** |
| Uma conta do painel não tem login com dois fatores | Crítica | **Conta** |
| Há atualizações de segurança pendentes | Aviso | **Sistema** |
| A busca por atualizações tem mais de uma semana, ou nunca rodou | Nota | **Sistema** |
| As atualizações de segurança automáticas estão desativadas | Aviso | **Sistema** |
| É preciso reiniciar para concluir as atualizações | Nota | **Configurações** |
| Uma conta que não é o root tem o ID de usuário 0 | Crítica | **Terminal** |
| Uma conta tem a senha vazia | Crítica | **Terminal** (`passwd -l <account>`) |
| MariaDB, MySQL, PostgreSQL, Redis, Valkey, Memcached ou MongoDB escuta em um endereço que não é 127.0.0.1 | Crítica quando o firewall deixa todo mundo chegar à porta; caso contrário, Nota | **Bancos de dados** |
| O Redis não tem senha | Crítica em um endereço público que o firewall deixa passar, Aviso em outro endereço público, Nota quando ele só escuta neste servidor | **Bancos de dados** → Redis |
| Um contêiner Docker publica portas em todos os endereços | Aviso | **Docker** |
| Um site não tem HTTPS | Aviso | **Sites** → (site) → **SSL / HTTPS** |
| Um site tem certificado, mas não redireciona para HTTPS | Nota | **Sites** → (site) → **SSL / HTTPS** |
| Um certificado expirou | Crítica | **Sites** → (site) → **SSL / HTTPS** |
| Um certificado expira em 14 dias ou menos | Aviso | **Sites** → (site) → **SSL / HTTPS** |
| Um site serve arquivos que deveriam ser privados | Crítica para segredos, Aviso para cópias e arquivos compactados | **Arquivos** |
| Um site tem arquivos ou pastas graváveis por todos | Aviso | **Arquivos** |
| O PHP mostra erros aos visitantes (`display_errors`) | Aviso | **Software** → PHP → **Gerenciar** |
| O PHP anuncia a sua versão (`expose_php`) | Nota | **Software** → PHP → **Gerenciar** |
| Há atualizações pendentes do núcleo, de plugins ou de temas do WordPress | Aviso | **Sites** → (site) → **WordPress** |
| O modo de depuração do WordPress (`WP_DEBUG`) está ativado | Aviso | **Sites** → (site) → **WordPress** |

Alguns detalhes:

- **Bancos de dados na rede**: a verificação lê em quais endereços cada programa escuta. Para um endereço público, ela olha o firewall: nenhum firewall, ou uma regra que deixa todo mundo chegar à porta, é crítico. Uma porta que o firewall só abre para alguns endereços (como o **Acesso remoto** de um banco de dados), ou que ele não abre, é uma nota. Um endereço de rede privada (10.x, 172.16–31.x, 192.168.x) é uma nota.
- **O Docker** grava as suas próprias regras de firewall antes das do ufw, por isso o ufw não bloqueia uma porta que um contêiner publica. Publique a porta em `127.0.0.1` (por exemplo `127.0.0.1:8080:80`) quando só um site deste servidor a usa.
- **Arquivos que um site serve**: a verificação lista o topo da pasta `public` de cada site e pede ao servidor web os arquivos que nunca deveriam ser públicos: arquivos `.env`, pastas `.git`, `.svn` e `.hg`, `.htpasswd`, dumps de banco de dados (`.sql`, `.sql.gz`, `.dump`, `.sqlite`…), cópias do `wp-config.php`, cópias de segurança (`.bak`, `.old`, `.orig`, `.save`, `.swp`, nomes terminados em `~`), `phpinfo.php` e `info.php`, e arquivos compactados cujo nome parece uma cópia do site (com "backup", "www", "site", "wordpress" ou o nome do domínio). Uma ocorrência significa que o servidor web respondeu com o arquivo. A configuração do Nginx do Nefo esconde os arquivos que começam com ponto, por isso no Nginx um arquivo `.env` é encontrado, mas não é relatado. Sites suspensos, que redirecionam para outro lugar, que pedem senha ou que são apps atrás de um proxy não são consultados.
- **Arquivos graváveis por todos**: a verificação percorre a pasta do site (`/var/www/<domain>`) sem seguir links simbólicos. Uma pasta com o sticky bit, como um `tmp` compartilhado, está bem. Use 644 para arquivos e 755 para pastas.
- **PHP**: a verificação lê os arquivos `php.ini` e `conf.d` de cada versão do PHP, e os pools do PHP-FPM, em busca de um `display_errors` ativado para um site.
- **As atualizações do WordPress** vêm da última verificação na página **WordPress** (ou na aba **WordPress** de um site). A própria verificação de segurança nunca roda o wp-cli. Depois que você atualiza um site, a próxima verificação ali mostra o que falta.
- **O painel sem HTTPS**: o Nefo ativa o HTTPS na instalação. Se ele estiver desativado, defina `"enabled": true` na parte `"tls"` de `/etc/nefo/config.json` e rode `systemctl restart nefo`. Um painel que só escuta em 127.0.0.1 não é relatado.

## Quanto tempo ela leva? {#limits}

A verificação lê os dados do próprio painel e alguns arquivos, e só pede ao próprio servidor web deste servidor (127.0.0.1) arquivos que existem na pasta de um site. Cada verificação tem um limite, por isso uma execução continua curta em um servidor com muitos sites:

| Limite | Valor |
|---|---|
| Uma execução que você inicia | No máximo 25 segundos. As verificações que não terminaram até lá aparecem como interrompidas no limite, e o resultado diz que é parcial |
| Cada programa que a verificação consulta (fail2ban, Docker, apt…) | No máximo 5 segundos; um programa que não responde a tempo fica de fora |
| Arquivos e pastas percorridos na pasta de um site | 20.000 (100.000 para todos os sites juntos) |
| Entradas lidas na pasta `public` de um site | 5.000 |
| Requisições ao servidor web deste servidor | 200 no total, 4 por vez, no máximo 3 segundos cada |

## Aceitar uma ocorrência {#accept-a-finding}

Algumas ocorrências estão bem no seu servidor: o SSH na porta 22 atrás do fail2ban, ou uma porta de banco de dados que você abriu de propósito para um endereço. Clique em **Aceitar** em uma ocorrência assim. Ela vai para a aba **Aceitas** e deixa de contar.

Uma ocorrência aceita volta quando os fatos dela mudam, por exemplo quando outra conta recebe o ID de usuário 0, ou um banco de dados passa a escutar em outro endereço. Ela também volta se foi corrigida e depois aparece de novo. Para trazê-la de volta você mesmo, abra a aba **Aceitas** e clique em **Mostrar de novo**. Aceitar e mostrar de novo ficam registrados no log de atividades.

## Verificação diária e alertas {#daily-check-and-alerts}

Com **Executar todo dia** ativado (o padrão), o painel roda a verificação uma vez por dia em segundo plano, durante a sua manutenção de hora em hora, mas nunca nos primeiros 10 minutos depois que o painel inicia. Assim, o dashboard sempre mostra um resultado recente.

Ative **Avisar quando a verificação diária encontrar um novo problema** (desativado por padrão) para receber uma mensagem nos seus canais de alerta (**Configurações** → **Alertas**) quando a verificação diária encontrar um problema crítico ou um aviso que a verificação anterior não tinha. Notas e ocorrências aceitas nunca são enviadas. Veja [Alertas](./mail-alerts#alerts).

## No dashboard {#on-the-dashboard}

Quando a última verificação encontrou problemas críticos ou avisos, **Precisa de atenção** no dashboard e o sino mostram "Verificação de segurança: 3 problemas a corrigir" com um botão **Ver**. Quando não há nenhum problema sério aberto, o bloco **Riscos de segurança** mostra quando a verificação rodou pela última vez, e clicar nele abre a verificação de segurança.

Para ser avisado sobre logins no SSH e no painel, veja [Alertas de login](./mail-alerts#sign-in-alerts).
