---
title: Sistema
description: Instale as atualizações do sistema operacional, ative as atualizações de segurança automáticas, acrescente swap, defina o fuso horário, a sincronização do relógio e o nome do host, ative o TCP BBR e libere espaço em disco.
---

# Sistema

A página **Sistema** instala as atualizações do sistema operacional, com as atualizações de segurança marcadas e verificadas a cada poucas horas, ativa as atualizações de segurança automáticas, cria um arquivo de swap, define o fuso horário, a sincronização do relógio e o nome do host, ativa o TCP BBR e libera espaço em disco. Cada alteração feita nela fica registrada no log de atividades.

A página fica no grupo **Servidor** do menu à esquerda. Ela tem cinco abas: **Atualizações**, **Swap**, **Hora e nome**, **Rede** e **Limpeza**. O log de atividades fica em **Conta** → **Atividade recente**.

As alterações demoradas, como instalar atualizações ou criar um arquivo de swap, rodam como uma tarefa em segundo plano: a página mostra o progresso delas, e só uma tarefa roda por vez.

## Atualizações {#updates}

A aba **Atualizações** lista as atualizações pendentes do sistema operacional: cada pacote, a versão instalada e a nova, e de onde vem a nova versão (a suíte, como `noble-updates`). As atualizações de uma suíte de segurança (`noble-security`, `bookworm-security`…) ficam marcadas como **Segurança** e aparecem primeiro. Um pacote marcado como **novo** é instalado junto com uma atualização, como um novo kernel.

### Como o painel procura atualizações {#how-the-panel-checks-for-updates}

- O painel procura por conta própria: a cada hora ele olha a sua última verificação e, quando ela tem mais de 6 horas, roda `apt-get update` e pergunta ao apt o que uma atualização instalaria. Ele pula a verificação enquanto outra tarefa roda, e nos primeiros 30 minutos depois que o painel inicia.
- Abrir a página nunca roda o apt: ela mostra a última verificação, com o horário.
- **Verificar agora** verifica na hora, como uma tarefa.
- **Retidas por enquanto** lista as atualizações que o apt ainda não instala: pacotes que ele retém porque precisariam de outras mudanças, e as atualizações graduais (phased updates) do Ubuntu, que chegam aos servidores ao longo de alguns dias.

Quando há atualizações de segurança pendentes, **Precisa de atenção** no dashboard e o sino mostram "N atualizações de segurança disponíveis".

### Instalar as atualizações {#install-updates}

- **Instalar todas as atualizações** roda `apt-get upgrade --with-new-pkgs`. Ele instala os pacotes novos de que uma atualização precisa, como um novo kernel, e nunca remove um pacote.
- **Instalar só as atualizações de segurança** atualiza só os pacotes instalados cuja nova versão vem de uma suíte de segurança (`apt-get install --only-upgrade` com os nomes deles).

Os dois atualizam antes as listas de pacotes, rodam sem perguntas e mantêm os arquivos de configuração que você alterou. A lista é verificada de novo quando eles terminam.

::: warning Os serviços reiniciam por um instante
Atualizar o Nginx, o Apache, o OpenLiteSpeed, o PHP, o MariaDB ou o Redis reinicia esse serviço, por isso os sites podem não responder por alguns segundos. Atualizar o Docker o reinicia, junto com os contêineres. A página cita os serviços que as atualizações pendentes reiniciam, e a confirmação também. Instale essas atualizações em um horário tranquilo.
:::

### Reinicialização necessária {#restart-required}

Algumas atualizações, como um novo kernel ou uma nova biblioteca C, só ficam completas depois que o servidor reinicia. A aba diz "O servidor precisa ser reiniciado para terminar de instalar as atualizações." quando:

- `/var/run/reboot-required` existe (o Ubuntu o grava e lista os pacotes em `/var/run/reboot-required.pkgs`), ou
- há em `/boot` um kernel mais novo que o que está rodando (o Debian nem sempre grava o arquivo).

**Reiniciar em Configurações** abre **Configurações** → **Painel** → **Energia**, onde você pode reiniciar o servidor. O sino também mostra "O servidor precisa ser reiniciado" até você fazer isso.

### Atualizações de segurança automáticas {#automatic-security-updates}

A chave **Atualizações de segurança automáticas** deixa o servidor instalar as atualizações de segurança sozinho, todo dia:

- Se o `unattended-upgrades` não estiver instalado, o painel o instala antes, como uma tarefa. O auxiliar dele que espera o desligamento do servidor usa cerca de 20 MB de memória.
- O painel define `APT::Periodic::Update-Package-Lists "1";` e `APT::Periodic::Unattended-Upgrade "1";` em `/etc/apt/apt.conf.d/20auto-upgrades`, mantendo as outras linhas do arquivo, e ativa os timers diários do apt.
- Desativar a chave define `APT::Periodic::Unattended-Upgrade "0";`.

Quais atualizações são instaladas é decidido pelas próprias configurações do unattended-upgrades em `/etc/apt/apt.conf.d/50unattended-upgrades`: no Ubuntu e no Debian, só as atualizações de segurança. Ele não reinicia o servidor, a menos que você ative `Unattended-Upgrade::Automatic-Reboot` lá. Se outro arquivo em `/etc/apt/apt.conf.d` sobrepuser a configuração, o painel avisa e deixa o arquivo como estava.

## Swap {#swap}

O swap é um espaço em disco que o kernel usa quando a memória acaba. Em um servidor com 1 ou 2 GB de memória, 1 a 2 GB de swap evitam que o MariaDB e o PHP sejam parados pelo kernel quando um momento de pico precisa de mais memória do que o servidor tem.

A aba **Swap** mostra a memória, o swap em uso e cada arquivo ou partição de swap de `/proc/swaps`.

### Criar um arquivo de swap {#create-a-swap-file}

1. Escolha o **Tamanho**: 512 MB, 1 GB, 2 GB, 4 GB ou 8 GB. Só são oferecidos os tamanhos que deixam 1 GB de disco livre.
2. Selecione **Criar um arquivo de swap**.

O painel cria `/panel.swap`, que só o root pode ler, com o `fallocate`. Onde o sistema de arquivos não aceita isso, ou o `swapon` recusa esse arquivo, ele grava o arquivo com o `dd`, o que leva cerca de um minuto a cada poucos gigabytes. No btrfs, o arquivo é antes marcado como não copy-on-write. Depois o painel roda `mkswap` e `swapon`, e acrescenta `/panel.swap none swap sw 0 0` ao `/etc/fstab`, para que o swap volte a ser ativado depois de uma reinicialização.

O painel se recusa a criar um arquivo de swap:

- quando o disco não tem o tamanho do arquivo mais 1 GB livres;
- quando o servidor já tem um arquivo de swap que o painel não criou, como `/swapfile` ou `/swap.img`. O painel o deixa como está. Uma partição de swap não o impede;
- no ZFS, onde arquivos de swap não funcionam;
- em um contêiner (OpenVZ, LXC, Docker), onde o kernel não permite o `swapon`: "Este servidor não permite swap". Peça swap ao seu provedor, ou use um servidor virtual completo.

Se algo falhar, o painel remove o arquivo de novo.

### Remover o arquivo de swap {#remove-the-swap-file}

**Remover o arquivo de swap** desativa o swap com o `swapoff`, remove a linha dele do `/etc/fstab` e exclui `/panel.swap`. O que está no swap volta antes para a memória, por isso o painel recusa quando isso não caberia na memória livre.

### Swappiness {#swappiness}

O **Swappiness** (`vm.swappiness`, de 0 a 100) define com que facilidade o kernel move memória para o swap. O padrão é 60; 10 é adequado para um servidor, que então só usa o swap quando falta memória. O painel grava o valor em `/etc/sysctl.d/90-panel-swap.conf` e o aplica na hora.

## Hora e nome do host {#time-and-host-name}

A aba **Hora e nome** mostra a hora do servidor, o fuso horário e se o relógio está sincronizado.

### Fuso horário {#time-zone}

Escolha um fuso horário na lista (`timedatectl list-timezones`) e selecione **Salvar**. O painel o define com `timedatectl set-timezone` e reinicia o cron, para que as tarefas cron sigam o novo fuso horário. Outros serviços que já estavam rodando, como o MariaDB, passam a usar o novo fuso horário depois de reiniciarem.

### Sincronização do relógio {#clock-synchronization}

**Sincronizar o relógio (NTP)** ativa ou desativa a sincronização da hora (`timedatectl set-ntp`). Mantenha-a ativada: os certificados e os códigos de dois fatores precisam de um relógio certo. Ela precisa de um serviço de hora, como o `systemd-timesyncd`. Sem um, a chave fica acinzentada: instale-o com `apt install systemd-timesyncd`.

### Nome do host {#host-name}

Informe o novo **Nome do host** e selecione **Salvar**. Um nome do host tem letras, dígitos e hifens, com pontos entre as partes, no máximo 64 caracteres, e não pode começar nem terminar com hífen: `web1` ou `web1.example.com`.

O painel:

1. o define com `hostnamectl set-hostname`;
2. aponta para ele a linha `127.0.1.1` do `/etc/hosts` (o nome completo e a primeira parte dele), ou acrescenta essa linha;
3. em um servidor com cloud-init, grava `/etc/cloud/cloud.cfg.d/99-panel-hostname.cfg` com `preserve_hostname: true`, para que o cloud-init não coloque de volta o nome do provedor na próxima inicialização.

O nome do host não muda o endereço do painel nem nenhum certificado. Quando você reinicia o servidor em **Configurações** → **Painel** → **Energia**, você confirma digitando o nome do servidor: depois de uma mudança, digite o novo.

## Rede: TCP BBR {#network-tcp-bbr}

A aba **Rede** mostra o controle de congestionamento TCP em uso (geralmente `cubic`), a fila padrão (`net.core.default_qdisc`, geralmente `fq_codel`) e os algoritmos que o kernel tem.

O BBR é um controle de congestionamento do Google. Ele costuma deixar os sites e os downloads mais rápidos em conexões longas ou com perda de pacotes, como para visitantes em outro continente ou em redes móveis. Ativar o **TCP BBR**:

1. carrega o módulo `tcp_bbr`, quando o kernel o tem como módulo;
2. grava `/etc/modules-load.d/panel-bbr.conf` (`tcp_bbr`) e `/etc/sysctl.d/90-panel-bbr.conf` (`net.core.default_qdisc = fq` e `net.ipv4.tcp_congestion_control = bbr`);
3. roda `sysctl --system`.

As novas conexões usam o BBR na hora. A fila `fq` vale para as placas de rede depois da próxima reinicialização; o BBR funciona bem sem ela nesse meio-tempo.

O painel recusa quando o kernel em execução não tem o BBR, e em um contêiner, onde as configurações do kernel não podem ser alteradas. Ele então não guarda nada. Desativar o BBR exclui os dois arquivos e volta para `cubic` e `fq_codel`. Quando o BBR foi configurado fora do painel, a chave fica acinzentada.

## Limpeza de disco {#disk-cleanup}

A aba **Limpeza** mostra quanto espaço cada limpeza libera. **Limpar** pede a sua confirmação, roda a limpeza como uma tarefa e depois mostra o espaço liberado.

| Item | O que é removido |
|---|---|
| **Pacotes baixados** | Os arquivos de pacotes que o apt guarda em `/var/cache/apt/archives` depois de instalar (`apt-get clean`). |
| **Journal do sistema** | As entradas antigas do journal do sistema, até sobrarem 100 MB (`journalctl --rotate`, depois `journalctl --vacuum-size=100M`). |
| **Logs antigos** | Os logs em `/var/log` que o logrotate já rotacionou: nomes terminados em `.gz` ou em um número, como `syslog.1` ou `auth.log.2.gz`. |
| **Pacotes não usados** | Os pacotes de que nada precisa mais, como kernels antigos (`apt-get autoremove`). |
| **Docker** | As imagens que nenhum contêiner usa, e o cache de build (`docker image prune --all`, `docker builder prune --all`). Só aparece quando o Docker está instalado. |

- **Logs antigos** mantém os logs do servidor web em `/var/log/nginx`, `/var/log/apache2` e `/var/log/openlitespeed`, porque as estatísticas de tráfego dos sites os leem. Ele só exclui arquivos nas pastas do próprio root e nunca segue links.
- Em **Pacotes não usados**, a confirmação lista os pacotes. Se o apt fosse remover mais alguma coisa no momento em que você confirma, nada é removido: "A lista de pacotes não usados mudou".
- **Docker** nunca remove contêineres, parados ou não, nem volumes, onde os contêineres guardam os seus dados.

## Log de atividades {#activity-log}

Estas ações aparecem em **Conta** → **Atividade recente**: atualizações do sistema instaladas, atualizações de segurança automáticas ativadas ou desativadas, arquivo de swap criado ou removido, swappiness alterado, fuso horário alterado, sincronização do relógio ativada ou desativada, nome do host alterado, TCP BBR ativado ou desativado e espaço em disco limpo.
