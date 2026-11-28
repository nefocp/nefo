---
title: Ferramentas do sistema
description: O dashboard, as tarefas cron, os serviços, os programas em segundo plano, os processos e as portas, os logins SSH, o terminal web, os logs, e como reiniciar o painel ou o servidor.
---

# Ferramentas do sistema

O **Dashboard** mostra ao vivo a CPU, a memória, o disco e a carga do servidor, com 7 dias de histórico, e lista o que precisa de atenção. As ferramentas de servidor do Nefo também gerenciam tarefas cron, serviços systemd, programas em segundo plano que precisam continuar rodando, processos e portas em escuta, logins SSH e um terminal root no navegador, e reiniciam o painel ou o servidor.

Esta página trata de cada uma dessas ferramentas, e de onde encontrar os logs. A maioria delas fica no grupo **Servidor** do menu à esquerda. As atualizações do sistema operacional, o swap, o fuso horário e o nome do host, o TCP BBR e a limpeza de disco ficam na página [Sistema](./os).

## Dashboard {#dashboard}

O **Dashboard** abre depois do login.

- O topo da página mostra o nome do servidor, o sistema operacional, o uptime, o número de CPUs e a memória. Em uma VPS que é um contêiner (LXC), a memória é o limite do contêiner, marcado como "limite do contêiner" (veja [Uma VPS que é um contêiner](./first-steps#a-vps-that-is-a-container)). Os botões ali permitem **Procurar atualizações**, abrir o **Terminal** e usar o menu **Energia**.
- Quatro medidores mostram ao vivo a **Carga do sistema**, a CPU, a **Memória** e o **Disco /**. Eles se atualizam a cada 5 segundos. A carga do sistema é a média de carga de 1 minuto dividida pelo número de CPUs.
- Os blocos contam sites, bancos de dados, contas SFTP, tarefas cron, backups e riscos de segurança. Selecione um bloco para abrir a página dele. Quando não há nenhum problema sério aberto, o bloco **Riscos de segurança** mostra quando a [verificação de segurança](./security-check) rodou pela última vez.
- **Histórico de recursos** desenha um gráfico de CPU, RAM, disco, carga ou rede, em 1 hora, 24 horas ou 7 dias. Embaixo do gráfico ficam os valores **Agora**, **Média** e **Máximo**. Para a rede, ele mostra as taxas de **Entrada** e **Saída** e os totais do período.
- **Precisa de atenção** lista os problemas que precisam de você, os piores primeiro.
- **Software instalado** mostra os principais componentes e o estado deles.

O painel registra a carga a cada minuto e guarda 7 dias de histórico. Depois de uma instalação nova, os gráficos aparecem depois de alguns minutos. A parte de baixo do menu à esquerda também mostra o uso atual de CPU e RAM.

### O que "Precisa de atenção" verifica {#what-needs-attention-checks}

- O login com dois fatores está desativado.
- O root pode fazer login por SSH com senha.
- O firewall está desativado.
- O certificado de um site não pôde ser renovado, expirou ou expira em até 14 dias.
- Um site expira em até 7 dias.
- Um backup agendado falhou.
- O disco está 90% cheio ou mais.
- Há atualizações de segurança do sistema operacional pendentes (veja [Atualizações](./os#updates)).
- O servidor precisa ser reiniciado para terminar de instalar as atualizações.
- A última [verificação de segurança](./security-check) encontrou problemas críticos ou avisos.
- Um site monitorado está fora do ar (veja [Monitoramento](./websites#monitoring)).

O sino no canto superior direito (**Notificações**) mostra a mesma lista. Os dois só ficam visíveis enquanto você está com a sessão aberta. Para receber uma mensagem por Telegram, e-mail, Discord, Slack ou um webhook, configure os [alertas](./mail-alerts).

## Tarefas cron {#cron-jobs}

As tarefas cron rodam comandos com um agendamento. Vá até **Tarefas cron** no menu à esquerda.

### Adicionar uma tarefa cron {#add-a-cron-job}

1. Selecione **Adicionar tarefa cron**.
2. Informe um **Nome**, com até 100 caracteres.
3. Escolha com quem **Executar como**: o root, ou o usuário de um site (mostrado com o domínio dele). Rode as tarefas de um site com o próprio usuário dele, e use o root só para tarefas do servidor.
4. Defina o **Agendamento**. Escolha um agendamento comum (**A cada 5 minutos**, **A cada hora**, **Todo dia às 03:00**, **Todo domingo às 04:00**, **Quando o servidor inicia**) ou digite o seu. Embaixo do campo, o painel descreve o agendamento em palavras.
5. Informe o **Comando**: um comando de shell em uma linha, com até 2000 caracteres, rodado com `/bin/sh`.
6. Deixe **Ativada** marcado e selecione **Salvar**.

Para o usuário de um site com PHP, o link **Usar o comando cron do WordPress** preenche o cron do WordPress, como `php8.3 /var/www/example.com/public/wp-cron.php`, a cada 5 minutos.

### A sintaxe do agendamento {#schedule-syntax}

Um agendamento tem cinco campos, na hora do servidor:

| Campo | Valores |
|---|---|
| minuto | 0–59 |
| hora | 0–23 |
| dia do mês | 1–31 |
| mês | 1–12 |
| dia da semana | 0–7 (0 e 7 são domingo) |

Em cada campo, use `*` para todos os valores, uma lista como `1,15`, um intervalo como `1-5` ou um passo como `*/10`. Você também pode usar um destes no lugar dos cinco campos: `@hourly`, `@daily` (ou `@midnight`), `@weekly`, `@monthly`, `@yearly` (ou `@annually`) e `@reboot`. O painel recusa números fora do intervalo, como o minuto 60.

| Agendamento | Roda |
|---|---|
| `*/5 * * * *` | a cada 5 minutos |
| `0 3 * * *` | todo dia às 03:00 |
| `30 2 * * 1` | toda segunda-feira às 02:30 |
| `0 0 1 * *` | à meia-noite do dia 1º de cada mês |

### Como as tarefas cron rodam {#how-cron-jobs-run}

- O painel grava as tarefas em `/etc/cron.d/panel`, por isso o cron do sistema as roda mesmo quando o painel está parado. Não edite esse arquivo: o painel o reescreve.
- A saída e os erros do comando são acrescentados ao log da tarefa, `/var/log/panel-cron/<id>.log`, que pertence ao usuário da tarefa.
- Um `%` no comando funciona como foi digitado: o painel o escapa para o cron.
- Uma tarefa para de rodar quando o usuário dela deixa de existir, por exemplo depois que você exclui o site.

### Rodar, conferir e alterar uma tarefa {#run-check-and-change-a-job}

- A chave da coluna **Ativada** ativa ou desativa uma tarefa. Uma tarefa desativada continua na lista.
- **Executar agora** roda a tarefa na hora, com o usuário dela, e mostra a saída ao vivo. Ela para depois de uma hora.
- **Logs**, no menu ⋯ da linha, mostra o final do log da tarefa. Os logs com mais de 5 MB são cortados para o último 1 MB.
- **Editar** e **Excluir** ficam no mesmo menu. Excluir uma tarefa também exclui o log dela.

Acima da lista, mostre **Todas** as tarefas ou só as **Ativadas** ou **Desativadas**, e pesquise por nome, comando ou usuário.

## Serviços {#services}

**Serviços** lista os serviços do sistema que o Nefo gerencia, se estiverem instalados:

| Serviço | Unidade systemd |
|---|---|
| Nefo | `nefo` |
| Nginx, Apache ou OpenLiteSpeed | `nginx`, `apache2`, `lshttpd` |
| PHP-FPM, um por versão instalada | como `php8.3-fpm` |
| MariaDB ou MySQL | `mariadb` ou `mysql` |
| PostgreSQL | como `postgresql@16-main` |
| Redis | `redis-server` |
| Memcached | `memcached` |
| fail2ban | `fail2ban` |
| SSH | `ssh` |
| Cron | `cron` |
| Firewall (ufw) | `ufw` |

Outros serviços, como o Docker ou o Postfix, não aparecem. Cada linha mostra o estado (**Rodando**, **Parado**, **Com falha**…), há quanto tempo o serviço está rodando, "não inicia com o sistema" quando for o caso, e o uso de memória.

As ações dependem do serviço:

| Serviço | Ações |
|---|---|
| Servidor web, PHP-FPM | **Iniciar**, **Parar**, **Reiniciar**, **Recarregar** |
| Nefo | **Reiniciar** |
| SSH | **Reiniciar**, **Recarregar** |
| Os outros | **Iniciar**, **Parar**, **Reiniciar** |

- **Parar** pede a sua confirmação, porque os sites que precisam do serviço param de funcionar.
- Antes de reiniciar ou recarregar o servidor web, o painel testa a configuração dele. Se o teste falhar, nada é reiniciado e o erro é mostrado.
- O SSH não pode ser parado pelo painel, para que você não fique trancado do lado de fora.
- Reiniciar o Nefo recarrega a página depois de alguns segundos.
- **Logs** mostra as últimas 150 linhas do journal do serviço.

O painel não muda se um serviço inicia com o sistema.

## Programas em segundo plano {#background-programs}

Os programas em segundo plano são comandos que precisam continuar rodando, como um worker de fila. O painel roda cada um como um serviço systemd, com o usuário Linux de um site, e o inicia de novo quando ele para. Você precisa de pelo menos um site antes.

### Adicionar um programa {#add-a-program}

1. Vá até **Programas em 2º plano** e selecione **Adicionar programa**.
2. Informe um **Nome**, com até 60 caracteres.
3. Escolha o **Site**. O programa roda com o usuário Linux desse site, nunca como root.
4. Se quiser, defina a **Pasta de trabalho**: uma pasta dentro da pasta do site, como `app`. Deixe vazio para usar a própria pasta do site.
5. Informe o **Comando**: um programa com os seus argumentos, em uma linha, com até 1000 caracteres.
6. Se quiser, acrescente **Variáveis de ambiente**: uma `NAME=value` por linha, até 20. No servidor, só o root pode lê-las.
7. Deixe **Iniciar agora e quando o servidor iniciar** marcado e selecione **Criar**.

Para um site PHP, o link **Usar o worker de fila do Laravel** preenche `php8.3 artisan queue:work --sleep=3 --tries=3 --max-time=3600`, com a versão do PHP do site.

### Como os programas em segundo plano rodam {#how-background-programs-run}

- Cada programa é um serviço systemd chamado `panel-daemon-<id>.service`.
- Quando ele sai, por qualquer motivo, o systemd o inicia de novo depois de 3 segundos.
- Na inicialização do sistema, ele inicia depois do MariaDB e do Redis.
- Ele roda com as proteções do systemd: `/usr`, `/boot` e `/etc` ficam somente leitura, `/home` e `/root` ficam ocultos, e ele tem o seu próprio `/tmp`.
- A pasta de trabalho precisa existir e ficar dentro da pasta do site, também através de links simbólicos.
- A saída dele vai para o journal do sistema.

### Controlar um programa {#control-a-program}

Cada linha mostra o estado (rodando, reiniciando, com falha, parando ou parado), quantas vezes ele reiniciou e o uso de memória.

- A linha mostra **Iniciar** (quando o programa está parado) ou **Reiniciar** (quando ele roda) como um botão. As outras ações, inclusive **Parar**, ficam no menu ⋯ da linha. **Parar** mantém o programa parado, também depois de uma reinicialização, até você iniciá-lo de novo.
- **Logs** mostra as últimas 200 linhas que o programa escreveu.
- **Editar** altera o programa. **Salvar** o reinicia com as novas configurações.
- **Excluir** o para e o remove.

Quando você exclui um site, os programas em segundo plano dele também são parados e removidos.

## Processos e portas {#processes-and-ports}

**Processos** mostra o que roda no servidor, em três abas: **Processos**, **Portas** e **Logins SSH**.

### Processos {#processes}

A lista mostra os processos mais ocupados, até 300. Ordene por PID, CPU % ou memória pelos títulos das colunas, e filtre por nome, usuário ou PID. **Atualizar a cada 5 segundos** mantém a lista em dia.

A CPU % é medida nos últimos segundos: 100% é um núcleo de CPU trabalhando o tempo todo.

Para parar um processo, selecione o botão de parar no final da linha dele e depois:

- **Parar** pede ao programa que termine e saia (sinal TERM). Os programas bem feitos salvam os seus dados antes.
- **Forçar parada** o encerra na hora (sinal KILL). Use só quando **Parar** não funcionar: os dados não salvos se perdem.

Um serviço rodado pelo systemd pode ser iniciado de novo pelo systemd. O painel não consegue parar o processo init do sistema (PID 1), o próprio painel, o processo que iniciou o painel nem threads do kernel. Cada parada fica registrada no log de atividades.

### Portas {#ports}

**Portas** lista as portas TCP e UDP em que os programas escutam, com o **Endereço**, o **Programa** e de onde ela é **Acessível a partir de**: **só deste servidor** ou **da rede**.

Uma porta aberta para a rede ainda pode estar fechada para fora pelo firewall. Veja [Firewall](./security#firewall-ufw).

## Logins SSH {#ssh-sign-ins}

**Processos** → **Logins SSH** lista os logins SSH e as tentativas malsucedidas dos últimos 30 dias, dos mais novos para os mais antigos, até 200. Cada linha mostra o horário, o resultado (**entrou**, **senha errada** ou **usuário desconhecido**), o usuário, o endereço IP e o método, como `publickey` ou `password`.

Mostre **Todos**, **Só os bem-sucedidos** ou **Só os malsucedidos**. A lista é lida do journal do serviço SSH.

Muitas tentativas malsucedidas geralmente significam que alguém está tentando adivinhar senhas. Desative o login por senha, ou instale o fail2ban para bloquear esses endereços. Veja [Segurança](./security#brute-force-protection-fail2ban).

## Terminal {#terminal}

**Terminal** abre um shell root (bash) no servidor, no seu navegador.

::: warning
O terminal é como uma sessão SSH como root: todo comando roda com todos os direitos, e um comando errado pode apagar dados ou parar o servidor. O painel não consegue desfazer o que você roda ali.
:::

- A sessão termina quando você sai da página, ou depois de 30 minutos sem digitação. Selecione **Reconectar** para abrir uma nova.
- Até 4 terminais podem ficar abertos ao mesmo tempo, inclusive os terminais de contêineres.
- O log de atividades registra quando um terminal foi aberto e fechado, e por quanto tempo. Ele não registra os comandos.

## Logs {#logs}

O Nefo não tem uma página única de logs. Cada log aparece ao lado daquilo a que pertence:

| Log | Onde |
|---|---|
| O log de acesso, o log de erros, os erros do PHP e os logs do app de um site | **Sites** → (site) → **Logs** |
| Os eventos do firewall de um site | **Sites** → (site) → **Firewall web (WAF)** |
| A saída de uma tarefa cron | **Tarefas cron** → **Logs** |
| O journal de um serviço | **Serviços** → **Logs** |
| A saída de um programa em segundo plano | **Programas em 2º plano** → **Logs** |
| Contêineres e projetos Compose | **Docker** |
| O que foi feito no painel | **Conta** → **Atividade recente** |
| O log do próprio serviço do painel | `journalctl -u nefo` no servidor |

Os logs de acesso, de erros e do PHP de um site mostram as últimas 300 linhas. Selecione **Atualizar** na janela do log para carregar as linhas novas.

## Reiniciar o painel ou o servidor {#restart-the-panel-or-reboot-the-server}

Vá até **Configurações** → **Painel** → **Energia**. O **Dashboard** tem as mesmas ações no menu **Energia** dele.

- **Reiniciar o painel** reinicia só o painel. Os sites, os bancos de dados e os outros serviços continuam rodando, e o painel volta em poucos segundos.
- **Reiniciar o servidor** reinicia o servidor inteiro. Os sites e os bancos de dados ficam fora do ar até ele voltar, geralmente de um a dois minutos. Digite o nome do servidor para confirmar.

Nos dois casos, a página se recarrega sozinha quando o painel volta a responder. Os dois são recusados enquanto uma tarefa em segundo plano roda, como uma instalação ou um backup: espere ela terminar. Os dois ficam registrados no log de atividades.

No servidor, você também pode reiniciar o painel com:

```sh
systemctl restart nefo
```
