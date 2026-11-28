---
title: Backups
description: Faça backup de sites, bancos de dados, pastas e apps com agendamento e retenção, neste servidor, em armazenamento compatível com S3, em um servidor SFTP ou no Google Drive, criptografados se você quiser, e restaure-os com um clique, inclusive em um novo servidor.
---

# Backups

O Nefo faz backup de sites, bancos de dados, pastas e apps da loja de aplicativos como arquivos compactados `.tar.gz`, manualmente ou com um agendamento diário ou semanal, com retenção. Ele guarda cada backup no servidor e pode enviar uma cópia para um armazenamento compatível com S3, um servidor SFTP ou o Google Drive, criptografada com uma frase secreta se você quiser. Uma restauração só precisa do backup e do nome dele digitado para confirmar, inclusive em um novo servidor.

Esta página trata do que um backup contém, do backup manual e agendado, do armazenamento remoto e da criptografia dele, da restauração e dos alertas quando um backup falha. Para restaurar em outro servidor, veja [Restaurar em um novo servidor](./restore-new-server).

## O que um backup contém {#what-a-backup-contains}

| Tipo | Contém |
|---|---|
| **Site** | Os arquivos da pasta do site, `/var/www/example.com`, exceto as pastas `tmp` e `logs` do primeiro nível, e um dump SQL de cada banco de dados ligado ao site |
| **Banco de dados** | Um dump SQL de um banco de dados |
| **Pasta** | Qualquer pasta do servidor, como `/etc/nginx` ou `/opt/app`, com proprietários, permissões e links simbólicos |
| **App** | Um app Docker da loja de aplicativos: a pasta dele em `/var/lib/panel-apps`, as senhas salvas dele, os dados dos contêineres dele e dumps dos bancos de dados dele (veja [Backups de apps](#app-backups)) |

Ligue um banco de dados ao site dele quando você o criar (veja [Bancos de dados](./databases#create-a-database)), para que o backup do site o inclua. Os backups de sites deixam de fora os links simbólicos e os arquivos especiais.

Todo backup é um arquivo compactado `.tar.gz` com um `manifest.json`, os arquivos em `files/` e os dumps dos bancos de dados em `databases/`. Os dumps são feitos com `mariadb-dump --single-transaction`, por isso os sites continuam rodando.

O `manifest.json` do backup de um site também registra o que o site precisa para ser criado de novo em outro servidor: os aliases, a versão do PHP, as regras de reescrita, as diretivas próprias do servidor web e as senhas dos usuários dos bancos de dados dele.

Os backups neste servidor não são criptografados. Só o root pode lê-los. Cada armazenamento remoto pode criptografar as suas cópias com uma frase secreta antes de elas saírem do servidor (veja [Criptografar as cópias](#encrypt-the-copies)).

## Onde os backups ficam guardados {#where-backups-are-kept}

Todo backup é salvo primeiro neste servidor, em `/var/backups/panel`:

```
/var/backups/panel/site/example.com/example.com_20261126-030000.tar.gz
/var/backups/panel/database/shop/shop_20261126-030000.tar.gz
/var/backups/panel/folder/etc_nginx-1a2b3c4d/etc_nginx_20261126-030000.tar.gz
```

As pastas têm o modo `0700` e os arquivos, `0600`. O painel também registra um checksum SHA-256 de cada arquivo compactado. Os backups locais ocupam espaço em disco, por isso fique de olho no disco e em quantos backups os seus agendamentos mantêm.

Para copiar um backup para o seu computador, abra o menu dele e selecione **Baixar**.

## Fazer backup agora {#back-up-now}

1. Vá até **Backups** e selecione **Fazer backup agora**.
2. Escolha **Site**, **Banco de dados**, **Pasta** ou **App**. **App** aparece assim que um app da loja de aplicativos é instalado.
3. Escolha o site, o banco de dados ou o app em **Backup de**, ou digite um **Caminho da pasta**, como `/etc/nginx`. As pastas que já tiveram backup são sugeridas.
4. Se houver um armazenamento remoto configurado, você pode marcar **Enviar também para o armazenamento remoto**.
5. Selecione **Fazer backup agora**.

O log de progresso mostra cada etapa. O painel roda uma tarefa longa por vez, por isso espere uma tarefa em andamento terminar antes.

Para um site, você também pode ir até **Sites** → (site) → **Backups** e selecionar **Fazer backup agora**.

### Pastas que não podem ter backup {#folders-that-can-t-be-backed-up}

O painel recusa estas pastas, qualquer pasta dentro delas e qualquer pasta que contenha uma delas. Os links simbólicos são seguidos antes da verificação.

- `/`, o sistema inteiro;
- `/proc`, `/sys`, `/dev` e `/run`;
- os dados do painel, `/var/lib/nefo`, e os backups dele, `/var/backups/panel`;
- `/var/lib/panel-sftp`, `/var/lib/panel-acme` e `/var/lib/panel-quarantine`.

Por isso `/var` e `/var/lib` não podem ter backup inteiros, mas `/var/www` ou `/etc` podem.

## Backups automáticos {#automatic-backups}

1. Vá até **Backups** e selecione **Adicionar agendamento**.
2. Escolha **Site**, **Banco de dados**, **Pasta** ou **App**, e o alvo.
3. Em **Quando**, escolha **Todo dia** ou **Toda semana**. Para um backup semanal, escolha o **Dia**.
4. Escolha a **Hora**. É a hora do servidor, e o formulário mostra a diferença dela para o UTC.
5. Informe o **Número de backups a manter**, de 1 a 365.
6. Opcional: marque **Enviar também para o armazenamento remoto**.
7. Selecione **Salvar agendamento**.

Para um site há um atalho: **Sites** → (site) → **Backups** → **Todo dia às 03:00**. Ele mantém 7 backups.

A aba **Backups automáticos** lista os agendamentos com a última execução, **Concluído** ou **Falhou**. Abra **A última execução falhou** para ler o erro.

Como os agendamentos rodam:

- O painel confere a cada minuto. Se outra tarefa estiver rodando, o backup espera o minuto seguinte.
- Se o servidor estava desligado na hora planejada, o backup roda assim que o painel voltar a rodar.
- Um agendamento novo espera a sua primeira hora planejada. Para ter um backup na hora, use também **Fazer backup agora**.

### Retenção {#retention}

Depois de cada backup agendado, o painel exclui os backups mais antigos feitos por aquele agendamento além do número a manter, neste servidor e no armazenamento remoto. Os backups que você fez manualmente nunca são excluídos automaticamente.

O backup que o Nefo faz antes de excluir um site (**Fazer backup do site antes**) conta como feito manualmente: ele nunca é excluído automaticamente. O nome dele termina em `_before-delete_<date>-<time>.tar.gz`.

Excluir um agendamento o interrompe. Os backups existentes dele ficam. Excluir um site, um banco de dados ou um app também exclui os agendamentos dele, e os backups dele ficam na página **Backups**.

## Armazenamento remoto {#remote-storage}

Uma cópia em outro provedor protege você se este servidor for perdido. Vá até **Backups** → **Armazenamento remoto** e selecione **Configurar** (ou **Adicionar armazenamento** quando já houver um configurado). Você pode configurar armazenamento compatível com S3, um servidor SFTP e o Google Drive ao mesmo tempo: os backups são então enviados para cada um deles.

### Armazenamento compatível com S3 {#s3-compatible-storage}

Escolha **Armazenamento compatível com S3** e preencha:

| Campo | O que informar |
|---|---|
| **Endpoint (https://…)** | O endereço S3 do provedor, `https://` e só um host, sem caminho. Por exemplo `https://s3.eu-central-1.amazonaws.com` |
| **Bucket** | O nome do bucket |
| **Região** | A região do bucket. Vazio significa `us-east-1`. O Cloudflare R2 usa `auto` |
| **Pasta no bucket (opcional)** | Um prefixo, como `panel/` |
| **ID da chave de acesso** | |
| **Chave de acesso secreta** | Quando você alterar as configurações depois, deixe vazio para manter a atual |

Isso funciona com Amazon S3, Cloudflare R2, Backblaze B2, Wasabi e MinIO. Use uma chave de acesso que só possa usar esse bucket.

Selecione **Testar e salvar**. O painel grava um pequeno arquivo de teste, lê-o de volta e o exclui antes de salvar as configurações.

Um backup pode ter no máximo 5 GB em um armazenamento S3, o limite de um único upload.

### Servidor SFTP {#sftp-server}

Use outro servidor em que você possa entrar por SFTP (SSH), como uma storage box ou uma segunda VPS, com uma conta só para backups.

Escolha **Servidor SFTP** e preencha:

| Campo | O que informar |
|---|---|
| **Servidor** | Um nome de host ou endereço IP, como `backup.example.com` |
| **Porta** | Geralmente `22` |
| **Nome de usuário** | A conta no servidor de backup |
| **Entrar com** | **Senha** ou **Chave privada** |
| **Chave privada (OpenSSH ou PEM)** | A chave inteira, de `-----BEGIN` até `END-----`. A chave pública dela precisa estar em `~/.ssh/authorized_keys` no servidor de backup |
| **Frase secreta (só para uma chave criptografada)** | |
| **Pasta no servidor (opcional)** | Relativa à pasta pessoal da conta, a menos que comece com `/`. Ela é criada se for preciso |

Selecione **Testar e salvar**. O painel se conecta, grava, lista, lê e exclui um arquivo de teste, e depois salva as configurações. Depois que uma chave é salva, **Alterar** mostra a metade pública dela em **Chave pública, para o ~/.ssh/authorized_keys no servidor de backup**, para você copiá-la de novo.

### Fixação da chave de host {#host-key-pinning}

**Testar e salvar** confia na chave de host que o servidor SFTP apresenta naquele momento, e a mostra em **Chave de host confiável**. Toda conexão posterior precisa apresentar exatamente essa chave, ou o envio do backup falha.

Antes de confiar no armazenamento, compare a impressão digital com a chave do próprio servidor de backup. No servidor de backup, rode:

```sh
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
```

Se o servidor de backup foi reinstalado ou as chaves dele mudaram de propósito, selecione **Alterar** e **Testar e salvar** de novo para confiar na nova chave. O painel então avisa que a chave foi substituída.

::: warning
Uma chave de host alterada também pode significar que alguém está interceptando a conexão. Se você não esperava a mudança, confira o servidor de backup antes de confiar na nova chave.
:::

### O Nefo faz backup no Google Drive? {#google-drive}

Sim, a partir da v0.24.0. Os backups vão para uma pasta no seu Google Drive. O painel só pede acesso aos arquivos que ele mesmo cria (a permissão `drive.file` do Google): ele não consegue ver nem alterar mais nada no seu Drive. Uma pasta que você cria manualmente no Drive fica invisível para o painel, mesmo com o nome certo.

O Google Drive precisa de um cliente OAuth próprio, em um projeto do Google Cloud. Ele não custa nada, e você o cria uma única vez.

O Google Drive foi testado só com um serviço do Google simulado, não com uma conta do Google real.

#### Criar o cliente OAuth {#create-the-oauth-client}

Os nomes em negrito nestes passos são do Google, não do painel, e ficam como o Google os mostra em inglês.

1. Abra o [console do Google Cloud](https://console.cloud.google.com/) e crie um projeto, ou escolha um.
2. Em **APIs and services** → **Library** (APIs e serviços → biblioteca), ative a **Google Drive API**.
3. Configure a **OAuth consent screen** (tela de consentimento OAuth) com o tipo de usuário **External** (externo) e depois defina o status de publicação dela como **In production** (em produção).
4. Em **Credentials** (credenciais), crie um **OAuth client ID** do tipo **TVs and Limited Input devices** (TVs e dispositivos de entrada limitada).
5. Guarde o client ID (ele termina em `.apps.googleusercontent.com`) e o client secret.

::: warning
Enquanto o status de publicação da tela de consentimento for **Testing** (teste), o Google encerra o acesso do painel depois de 7 dias, e os envios passam a falhar. Defina-o como **In production**.
:::

#### Conectar {#connect}

1. Em **Armazenamento remoto**, selecione **Configurar** ou **Adicionar armazenamento**, e escolha **Google Drive** em **Tipo**. **Como criar o cliente OAuth** repete os passos acima.
2. Informe o **Client ID** e o **Client secret**.
3. Opcional: mude a **Pasta no Google Drive**. O padrão é `Nefo backups/<the server's host name>`. Separe as pastas com `/`; as pastas que faltarem são criadas.
4. Opcional: marque **Criptografar os backups antes do envio** (veja [Criptografar as cópias](#encrypt-the-copies)).
5. Selecione **Conectar ao Google Drive**. O painel mostra um endereço, `google.com/device`, e um código como `ABCD-EFGH`.
6. Em qualquer dispositivo (o celular serve), abra esse endereço, faça login com a conta do Google que vai guardar os backups, informe o código e permita o acesso.
7. O painel espera enquanto você faz isso, e mostra por quanto tempo o código continua válido. Quando o Google responde, o painel cria a pasta, grava, lista, lê e exclui ali um pequeno arquivo de teste, e salva as configurações.

A lista de **Armazenamento remoto** passa a mostrar a pasta, a conta do Google e um link, **Abrir no Google Drive**.

Você pode sair da página enquanto o painel espera: abra **Backups** → **Armazenamento remoto** de novo para ver o código.

#### Alterar ou conectar de novo {#change-or-reconnect}

Selecione **Alterar** na linha do Google Drive:

- Para mudar a pasta ou a criptografia, edite-as e selecione **Testar e salvar**. Não é preciso um novo login. Uma nova pasta é criada se for preciso. Os backups enviados antes ficam na pasta antiga, e o painel não os encontra mais lá para uma restauração.
- Para usar outra conta do Google ou outro cliente OAuth, ou depois que o acesso foi revogado, selecione **Conectar de novo** e informe o novo código. Deixe o **Client secret** vazio para manter o salvo do mesmo cliente.

**Testar e salvar** também conserta uma pasta que foi excluída ou movida para a lixeira do Drive: ele cria a pasta de novo.

Mantenha o mesmo projeto do Google Cloud. O Google dá a cada projeto acesso só aos arquivos que aquele projeto criou, por isso, com um cliente OAuth de outro projeto, o painel não consegue ver os backups que enviou antes.

#### Como os envios funcionam {#how-uploads-work}

- Cada backup é enviado em pedaços de 8 MB. Se a conexão cair ou o Google estiver ocupado, o painel pergunta ao Google quanto chegou e continua dali. Ele tenta de novo por alguns minutos antes de desistir.
- Antes de um envio, o painel confere o espaço livre da conta do Google e recusa na hora quando o backup não cabe.
- Os downloads para uma restauração também continuam de onde pararam.
- A pasta no Drive tem as mesmas subpastas que `/var/backups/panel`: `site/example.com/…`, `database/shop/…`.

#### Desconectar {#disconnect}

**Excluir** na linha do Google Drive esquece as configurações e avisa o Google de que o painel abre mão do acesso. Os backups já enviados ficam no seu Google Drive.

#### Quando algo dá errado {#when-something-goes-wrong}

| Mensagem | O que fazer |
|---|---|
| O acesso ao Google Drive foi revogado ou expirou | Selecione **Alterar** → **Conectar de novo**. Se isso acontecer cerca de 7 dias depois de conectar, defina antes o status de publicação da tela de consentimento como **In production**. |
| O Google Drive está cheio | Libere espaço na conta do Google ou compre mais armazenamento, e depois envie de novo com **Enviar**. |
| A pasta … do Google Drive foi excluída / está na lixeira do Drive | Restaure a pasta da lixeira do Drive (os backups dela voltam junto), ou selecione **Alterar** → **Testar e salvar** para criar uma pasta nova. |
| O Google recusou o cliente OAuth | Confira o client ID e o secret, e se o tipo do cliente é **TVs and Limited Input devices**. |
| O código expirou antes de ser digitado | Selecione **Conectar ao Google Drive** de novo para obter um novo código. |

### Criptografar as cópias {#encrypt-the-copies}

Cada armazenamento (S3, SFTP e Google Drive) pode criptografar os backups que recebe. A cópia no armazenamento fica então ilegível sem a frase secreta, para o provedor e para qualquer pessoa que entre na conta do armazenamento. A cópia neste servidor continua um `.tar.gz` normal.

1. No formulário do armazenamento, marque **Criptografar os backups antes do envio**.
2. Informe uma **Frase secreta** de pelo menos 12 caracteres, e a mesma em **Repita a frase secreta**. Uma frase de várias palavras funciona bem.
3. Selecione **Testar e salvar** (ou **Conectar ao Google Drive**). O arquivo de teste também é criptografado.

::: danger
Sem a frase secreta, os backups criptografados não podem ser restaurados: nem por você, nem pelo painel, nem por ninguém. Guarde-a fora deste servidor, por exemplo em um gerenciador de senhas. Se este servidor for perdido, a frase secreta é tudo de que você precisa, junto com o armazenamento, para recuperar os backups.
:::

O que muda:

- A cópia criptografada tem o nome do backup com `.enc` acrescentado: `example.com_20261126-030000.tar.gz.enc`.
- Na lista de **Backups**, a etiqueta do armazenamento na coluna **Armazenado em** mostra um cadeado em vez de uma nuvem.
- A linha do armazenamento em **Armazenamento remoto** mostra **Criptografado**.
- A frase secreta nunca é mostrada de novo. Para mantê-la quando você alterar outras configurações, deixe a **Frase secreta** vazia.

Trocar a frase secreta:

- Uma nova frase secreta vale só para os novos envios. As cópias já enviadas mantêm a frase secreta com que foram criptografadas; elas não são criptografadas de novo.
- Este servidor lembra as frases secretas anteriores (até 10), por isso as restaurações aqui continuam funcionando.
- Em outro servidor, você precisa da frase secreta que estava em uso quando a cópia foi enviada.
- Se você desativar a criptografia, as cópias já enviadas continuam criptografadas, e este servidor mantém a frase secreta delas para restaurá-las.

Como funciona: o painel criptografa o arquivo compactado neste servidor, enquanto o envia, com AES-256-GCM. A chave vem da frase secreta pelo scrypt, com um novo salt aleatório para cada arquivo, por isso duas cópias do mesmo backup parecem diferentes. O arquivo compactado é selado em pedaços de 64 KB, cada um numerado e o último marcado, de modo que uma cópia alterada, cortada ou montada com pedaços de outras cópias é recusada. Nenhuma chave fica guardada no servidor além da frase secreta nas configurações do painel. O formato do arquivo está descrito em [Restaurar em um novo servidor](./restore-new-server#format-of-encrypted-copies).

Criptografar custa pouco: derivar a chave leva cerca de um quarto de segundo e 64 MB de memória por um instante, uma vez por envio, e a cópia fica maior em um cabeçalho de 64 bytes e 16 bytes a cada 64 KB. Para o armazenamento S3, o painel lê o backup duas vezes, porque o S3 precisa do checksum da cópia criptografada antes de o envio começar.

### Enviar e remover armazenamentos {#uploading-and-removing-storage}

- Um backup feito sem **Enviar também para o armazenamento remoto** pode ser enviado depois: abra o menu dele e selecione **Enviar**. O painel o copia para cada armazenamento que ainda não o tem, criptografado para os armazenamentos que criptografam.
- A coluna **Armazenado em** mostra **Este servidor** e uma etiqueta para cada cópia remota: S3, SFTP ou Google Drive, com um cadeado para uma cópia criptografada.
- Remover as configurações do armazenamento não exclui os backups já enviados para lá.
- **Encontrar backups**, na linha de um armazenamento, lista os backups dele que não estão na lista, por exemplo em um novo servidor: veja [Restaurar em um novo servidor](./restore-new-server).

## Restaurar um backup {#restore-a-backup}

1. Vá até **Backups** e encontre o backup na lista.
2. Selecione **Restaurar** na linha dele.
3. Digite o nome que a janela pede (o domínio, o nome do banco de dados, o caminho da pasta ou o nome do app) e selecione **Restaurar**.

O painel primeiro confere o checksum SHA-256 do arquivo compactado, e recusa um arquivo danificado antes de alterar qualquer coisa. Se o arquivo não estiver mais neste servidor, ele é baixado do armazenamento remoto.

Se a cópia estiver criptografada, o painel a descriptografa enquanto a baixa, com a frase secreta do armazenamento (ou uma anterior que este servidor lembra). A conferência do SHA-256 roda então no arquivo compactado descriptografado, como em qualquer outro backup. Quando um backup só está no armazenamento remoto e a cópia dele está criptografada, a janela **Restaurar** tem um campo **Frase secreta da criptografia (opcional)**. Deixe-o vazio para usar a frase secreta do armazenamento. Informe a frase secreta que estava em uso quando a cópia foi enviada se o armazenamento foi configurado de novo desde então, por exemplo depois de você removê-lo e acrescentá-lo com uma nova frase secreta.

A restauração para antes de qualquer alteração, com uma mensagem no log, quando:

- a frase secreta não desbloqueia a cópia ("this copy is encrypted and the passphrase does not unlock it", esta cópia está criptografada e a frase secreta não a desbloqueia);
- a cópia foi alterada ou danificada ("the encrypted copy is damaged or was changed", a cópia criptografada está danificada ou foi alterada);
- a cópia está incompleta ("the encrypted copy is incomplete", a cópia criptografada está incompleta).

Se houver cópias em vários armazenamentos, o painel tenta o próximo.

::: danger
Uma restauração sobrescreve o conteúdo atual. As alterações feitas desde o backup se perdem.
:::

O que uma restauração substitui:

| Tipo | O que acontece |
|---|---|
| **Site** | Todos os arquivos da pasta do site são excluídos, exceto `tmp` e `logs`, e os arquivos do backup são colocados de volta, com o usuário do site como proprietário. Cada banco de dados do backup é esvaziado e preenchido a partir do dump dele. Um banco de dados que não existe mais é criado de novo, ligado ao site, com a senha do usuário dele tirada do backup (os backups feitos antes da v0.24.0 não a têm: o banco de dados então recebe uma nova senha). Um banco de dados que não está mais ligado a um site é ligado ao site restaurado. No WordPress, a senha atual do banco de dados é gravada no `wp-config.php` |
| **Banco de dados** | O banco de dados é apagado, criado de novo vazio e preenchido a partir do dump |
| **Pasta** | O backup é descompactado primeiro ao lado do conteúdo atual. Só quando isso funciona o conteúdo da pasta é substituído, por isso um backup quebrado deixa a pasta como estava. Os arquivos acrescentados depois do backup são excluídos. Os proprietários, as permissões e os links simbólicos voltam. Uma pasta que não existe é criada |
| **App** | Os contêineres do app param, a pasta e os dados dele são substituídos, os contêineres iniciam de novo e os dumps dos bancos de dados são carregados. As senhas salvas no backup também voltam |

Os dumps dos backups de sites e de bancos de dados são carregados como uma [importação](./databases#import) na página **Bancos de dados**: com um usuário temporário do MariaDB ou do MySQL que só pode usar aquele banco de dados, por isso um dump nunca grava em outro banco de dados.

### Restaurar um site que foi excluído {#restore-a-website-that-was-deleted}

Se o site de um backup não existe mais neste servidor, por exemplo porque você o excluiu, a restauração o cria de novo a partir do backup: o mesmo domínio e os mesmos aliases, a versão do PHP, as regras de reescrita, os arquivos e os bancos de dados. A janela de restauração avisa isso antes. O que um backup não guarda, como o certificado, as tarefas cron e o acesso SFTP, não é criado. Veja [Restaurar em um novo servidor](./restore-new-server#restoring-a-website-that-doesn-t-exist-here) para os detalhes.

Os backups feitos pelo Nefo 0.24.1 ou posterior registram o tipo de cada site, por isso um site que não existe mais volta como o mesmo tipo: um site de app (Node.js, Python, Java) com o runtime, o comando de início, a porta e o limite de memória, e um site de proxy reverso com o endereço e as opções. Se o runtime do app não estiver instalado, a restauração para e pede que você o instale antes na página **Software**. Os backups feitos por versões mais antigas só registram a versão do PHP: um site assim volta como um site estático, por isso crie-o de novo antes com o mesmo domínio e o mesmo tipo e depois restaure. Para um app Docker, instale o app de novo no mesmo domínio pela [loja de aplicativos](./apps), depois restaure o backup dele do tipo **App**, e o backup do site se a pasta do site tinha arquivos próprios, como as músicas do Navidrome.

## Restaurar em outro servidor {#restore-on-another-server}

Um novo servidor com o Nefo pode encontrar os backups no seu armazenamento remoto com **Encontrar backups**, acrescentá-los à lista dele e restaurá-los, inclusive os sites. Você precisa dos dados de acesso do armazenamento, e da frase secreta se as cópias estiverem criptografadas. Veja [Restaurar em um novo servidor](./restore-new-server).

## Backups de apps {#app-backups}

Os apps Docker da [loja de aplicativos](./apps) guardam os seus dados fora da pasta do site, em `/var/lib/panel-apps/<name>`. Eles recebem backups do tipo **App**:

- Fazer backup de um site que roda um app Docker também faz um backup do app, com o mesmo agendamento. A retenção exclui os backups antigos do app junto com os do site.
- Para fazer backup de um único app, escolha **App** em **Fazer backup agora** ou **Adicionar agendamento** na página **Backups**, ou vá até **Loja de aplicativos** → **Instalados** → **Fazer backup agora**. Os apps de serviço, como o PostgreSQL, não têm site, por isso dê a eles um agendamento próprio.
- Antes da atualização de um app, o painel faz um backup. Se esse backup falhar, a atualização não começa.

Dependendo do app, o painel pausa os contêineres dele enquanto copia os arquivos, ou faz antes o dump dos bancos de dados dele com as ferramentas próprias deles. Restaure os backups de apps na página **Backups**, como qualquer outro. Uma restauração precisa que o app esteja instalado. Depois que você desinstala um app, os backups dele ficam na lista com o nome e o logotipo dele: instale o app de novo (um app com site, no mesmo domínio) e depois restaure.

## Excluir um backup {#delete-a-backup}

Abra o menu de um backup e selecione **Excluir**. O backup é excluído deste servidor e do armazenamento remoto.

## Alertas quando um backup falha {#alerts-when-a-backup-fails}

O painel pode enviar um alerta por Telegram, e-mail, Discord, Slack ou um webhook quando um backup ou uma restauração falha. Vá até **Configurações** → **Alertas** e ative **Um backup, uma restauração ou uma renovação de certificado falha**. Veja [E-mail e alertas](./mail-alerts).

A aba **Backups automáticos** também mostra **Falhou** para um agendamento cuja última execução falhou.
