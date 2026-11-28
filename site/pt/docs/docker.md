---
title: Docker
description: Rode contêineres e projetos Compose, abra um terminal em um contêiner, gerencie imagens e coloque um contêiner em um site atrás de um proxy reverso.
---

# Docker

O Nefo instala o Docker da sua distribuição e gerencia contêineres, projetos Compose e imagens na página **Docker**, com um terminal em cada contêiner. Ele publica as portas dos contêineres só em 127.0.0.1, para que você as acesse por um site, e recusa projetos Compose que enfraqueceriam o servidor, como contêineres privilegiados ou portas abertas para a internet.

Esta página trata de cada aba, das verificações de segurança que o painel faz nos projetos Compose, de como os apps Docker da loja de aplicativos se encaixam e de como tornar um contêiner público por um site.

## Instalar o Docker {#install-docker}

Vá até **Software** → **Docker** e selecione **Instalar**. O painel instala o `docker.io` da sua distribuição, com o plugin Compose quando houver um. O item **Docker** do menu aparece assim que o Docker é instalado.

Se o plugin Compose faltar depois, o painel o instala na primeira vez que você iniciar um projeto Compose.

Para desinstalar o Docker, veja [Software](./software#when-an-uninstall-is-refused). A desinstalação é recusada enquanto existirem contêineres, projetos Compose ou apps Docker.

## Por que as portas ficam em 127.0.0.1 {#why-ports-stay-on-127-0-0-1}

As portas que o Docker publica escapam do firewall ufw: um contêiner publicado em todos os endereços fica aberto para a internet, diga o ufw o que disser. Por isso o painel:

- publica os contêineres que inicia só em `127.0.0.1`;
- recusa projetos Compose que publiquem uma porta em qualquer outro endereço.

Para tornar um contêiner público, coloque um site na frente dele. Veja [Colocar um contêiner em um site](#put-a-container-on-a-website).

## Contêineres {#containers}

A aba **Contêineres** lista todos os contêineres do servidor, rodando ou não, com a imagem, o estado e as portas. Os contêineres dos projetos Compose e dos apps da loja de aplicativos também aparecem aqui.

A linha de cada contêiner tem um botão **Iniciar** ou **Parar**, e as outras ações ficam no menu dela:

| Ação | O que faz |
|---|---|
| **Iniciar** / **Parar** | Inicia ou para o contêiner |
| **Terminal** | Abre um shell dentro do contêiner. Só para contêineres rodando |
| **Reiniciar** | Reinicia o contêiner |
| **Logs** | Mostra as últimas 200 linhas da saída dele, com data e hora |
| **Excluir** | Remove o contêiner, mesmo enquanto ele roda |

::: warning
**Excluir** perde os dados do contêiner, a menos que eles estejam guardados em um volume.
:::

### Rodar um contêiner {#run-a-container}

1. Na aba **Contêineres**, selecione **Rodar um contêiner**.
2. Preencha:
   - **Imagem**, como `nginx:alpine`;
   - **Nome do contêiner**, como `my-app`;
   - **Porta neste servidor** (1024 a 65535) e **Porta no contêiner**, se o contêiner servir algo;
   - **Variáveis de ambiente (uma NAME=value por linha)**, como `TZ=UTC`.
3. Selecione **Iniciar**.

O painel baixa a imagem, se for preciso, e inicia o contêiner com a política de reinício `unless-stopped`, para que ele volte depois de uma reinicialização. A porta é publicada só em `127.0.0.1`.

### Terminal em um contêiner {#terminal-in-a-container}

Selecione **Terminal** para abrir um shell dentro de um contêiner que está rodando. O painel usa o `bash` quando a imagem o tem, e o `sh` caso contrário. A sessão termina quando você fecha a janela ou depois de 30 minutos sem digitação.

## Projetos Compose {#compose-projects}

Um projeto Compose é um `compose.yaml` que roda vários contêineres juntos. O painel guarda cada projeto em `/var/lib/panel-compose/<name>/compose.yaml`, que só o root pode ler, e o roda como o projeto Compose `panel-<name>`.

### Criar um projeto {#create-a-project}

1. Vá até **Docker** → **Compose** e selecione **Novo projeto**.
2. Informe um **Nome do projeto**: letras minúsculas, dígitos e hifens, com até 40 caracteres. Os contêineres recebem o nome `panel-<name>-<service>-1`.
3. Escreva o arquivo no campo `compose.yaml`. **Inserir exemplo** preenche um pequeno exemplo que funciona.
4. Selecione **Criar e iniciar**.

O painel confere o arquivo, salva-o e inicia os contêineres, baixando as imagens de que precisa. Os caminhos relativos no arquivo, como `./data:/data`, se referem à pasta do projeto.

Um projeto mínimo:

```yaml
services:
  web:
    image: traefik/whoami
    restart: unless-stopped
    ports:
      - "127.0.0.1:8081:80"
```

O `compose.yaml` pode ter até 64 KB.

### Editar um projeto {#edit-a-project}

Abra o menu de um projeto e selecione **Editar**. Altere o arquivo e selecione **Salvar e aplicar**. O painel confere o novo arquivo, salva-o e recria os contêineres cuja configuração mudou. Os contêineres dos serviços que você tirou do arquivo também são removidos.

### Ações do projeto {#project-actions}

| Ação | O que faz |
|---|---|
| **Iniciar** | Confere o arquivo e inicia os contêineres |
| **Parar** | Para os contêineres |
| **Reiniciar** | Reinicia os contêineres |
| **Atualizar imagens** | Confere o arquivo, baixa imagens mais novas e recria os contêineres |
| **Remover contêineres** | Para e remove os contêineres. Os volumes e o `compose.yaml` são mantidos |
| **Logs** | As últimas 200 linhas de cada contêiner |
| **Excluir** | Remove os contêineres, as redes e a pasta do projeto com o `compose.yaml`. Marque **Excluir também os volumes (os dados neles se perdem)** para excluir os volumes também. As imagens são mantidas |

### Verificações de segurança {#safety-checks}

Antes de salvar um projeto, e antes de cada início e atualização, o painel pede ao Compose a configuração final, com includes, extends e variáveis resolvidos. Ele recusa um projeto que enfraqueceria o servidor:

| Recusado | Por quê |
|---|---|
| `privileged: true` | Um contêiner privilegiado tem controle total do servidor |
| `network_mode`, `pid`, `ipc`, `userns_mode` ou `cgroup` definidos como `host` | O contêiner compartilharia o próprio namespace do servidor |
| Qualquer `cap_add` | Privilégios extras para o contêiner |
| Qualquer `devices` ou `device_cgroup_rules` | Dispositivos do host entregues a um contêiner |
| `security_opt` que não seja `no-new-privileges` | Opções como `seccomp=unconfined` ou `apparmor=unconfined` enfraquecem o isolamento do contêiner |
| Qualquer `sysctls` | Configurações do kernel |
| Uma porta sem `127.0.0.1` ou `::1` como endereço do host, como `"8080:80"`, ou uma porta sem número publicado | O Docker escutaria em todos os endereços, passando por cima do firewall |
| Um bind mount de `/etc`, `/root`, `/boot`, `/proc`, `/sys`, `/dev` ou de qualquer coisa dentro deles | As pastas do sistema dão a um contêiner o controle do servidor |
| Um bind mount do socket do Docker, `/var/run/docker.sock` ou `/run/docker.sock`, ou de uma pasta que o contenha, como `/`, `/var`, `/var/run` ou `/run` | O socket dá a um contêiner o controle do Docker e, portanto, do servidor |
| Um volume nomeado cuja opção `device` seja um desses caminhos, e secrets ou configs lidos de um desses caminhos | As mesmas pastas, montadas de outro jeito |

Os caminhos são conferidos como estão escritos e depois de seguir os links simbólicos. `/etc/localtime` e `/etc/timezone` podem ser montados somente leitura (`:ro`), para que um contêiner use o fuso horário do servidor.

Quando uma verificação falha, o editor mostra qual serviço e qual configuração foram recusados. Nada é salvo nem iniciado.

## Imagens {#images}

A aba **Imagens** lista as imagens locais com o tamanho, a data de criação e os contêineres que as usam (**Usada por**).

- **Remover** exclui uma imagem. Se houver contêineres que a usam, o painel diz quais e pergunta de novo antes de remover a imagem mesmo assim.
- **Remover não usadas** exclui todas as imagens que nenhum contêiner usa. Elas são baixadas de novo quando forem necessárias.

Não há um botão separado para baixar imagens. As imagens são baixadas quando você roda um contêiner, inicia um projeto Compose ou seleciona **Atualizar imagens**.

## Apps Docker da loja de aplicativos {#docker-apps-from-the-app-store}

A [loja de aplicativos](./apps) instala apps Docker como Ghost, Uptime Kuma, n8n e Gitea. Eles rodam como projetos Compose que o painel escreve a partir da receita de cada app:

- Os arquivos e os dados deles ficam em `/var/lib/panel-apps/<name>`, e o projeto Compose se chama `panelapp-<name>`. Eles não aparecem na aba **Compose**, mas os contêineres deles aparecem na aba **Contêineres**.
- Eles passam pelas mesmas verificações de segurança que os seus próprios projetos.
- Cada app Docker recebe o seu próprio site. Os apps de serviço, como PostgreSQL, MongoDB e Ollama, não têm site e escutam só em `127.0.0.1`.

Gerencie esses apps em **Loja de aplicativos** → **Instalados**, onde você os inicia, para, atualiza e faz backup deles. Veja [Loja de aplicativos](./apps) para removê-los, e [Backups](./backups#app-backups) para os backups deles.

## Colocar um contêiner em um site {#put-a-container-on-a-website}

Um site pode encaminhar os visitantes para a porta de um contêiner em `127.0.0.1`. O servidor web então funciona como um proxy reverso, e você pode dar ao site um certificado SSL.

1. Publique a porta do contêiner em `127.0.0.1`, por exemplo `127.0.0.1:8081:80` no `compose.yaml`, ou `8081` em **Porta neste servidor** em **Rodar um contêiner**.
2. Vá até **Sites** e selecione **Adicionar site**.
3. Informe o **Domínio**, como `app.example.com`.
4. Em **Tipo**, escolha **App (Node.js, Python, Java…) atrás de um proxy**.
5. Em **Porta em que o app escuta (127.0.0.1)**, informe a mesma porta, `8081`.
6. Deixe **Comando de início (opcional)** vazio: o Docker roda o app.
7. Selecione **Criar site**.

A porta precisa estar entre 1024 e 65535, e não pode ser a porta do painel, a 3306 (MariaDB), a 6379 (Redis) nem a 7809 (o receptor dos webhooks do Git). Dois sites de app não podem usar a mesma porta.

Depois obtenha um certificado para o site, veja [Certificados SSL](./ssl). Veja [Sites](./websites) para as outras configurações do site.

O tipo **Proxy reverso para outro endereço** também funciona para um contêiner: ele encaminha para qualquer endereço, como `http://127.0.0.1:8081`, com opções para o cabeçalho Host, WebSocket e um tempo limite. Veja [Sites de proxy reverso](./websites#reverse-proxy-websites).
