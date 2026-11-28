---
title: Restaurar em um novo servidor
description: Encontre os backups do seu armazenamento S3, SFTP ou Google Drive a partir de um novo servidor Nefo, restaure sites que ainda não existem nele com os seus bancos de dados, e descriptografe uma cópia de backup manualmente.
---

# Restaurar em um novo servidor

Se um servidor for perdido, um novo servidor com o Nefo pode encontrar os backups no seu armazenamento remoto (compatível com S3, SFTP ou Google Drive) com **Encontrar backups**, e restaurá-los. Um site que não existe no novo servidor é criado de novo a partir do backup, com o domínio, os aliases, a versão do PHP, os arquivos e os bancos de dados. Isso funciona a partir da v0.24.0.

Esta página trata de como encontrar os backups, do que uma restauração cria e do que não cria, de outros tipos de backup e de como descriptografar uma cópia manualmente. Para os backups em geral, veja [Backups](./backups).

## Do que você precisa {#what-you-need}

- os dados de acesso do armazenamento: o bucket e as chaves dele, a conta SFTP ou um cliente OAuth do Google do mesmo projeto do Google Cloud;
- a frase secreta, se as cópias estiverem criptografadas;
- um servidor com o Nefo, um servidor web, as versões do PHP que os seus sites usam, e MariaDB ou MySQL (veja [Primeiros passos](./first-steps#set-up-the-server)).

## Como restauro um site em um novo servidor? {#restore-on-another-server}

1. Instale o Nefo no novo servidor e depois o servidor web, o PHP e o MariaDB (veja [Primeiros passos](./first-steps)).
2. Vá até **Backups** → **Armazenamento remoto** e configure o mesmo armazenamento: o mesmo bucket e a mesma pasta, o mesmo servidor e pasta SFTP, ou a mesma pasta do Google Drive. No Google Drive, use o mesmo projeto do Google Cloud (veja [Google Drive](./backups#google-drive)). Marcar **Criptografar os backups antes do envio** com a frase secreta antiga é opcional: você também pode informar a frase secreta no passo 4.
3. Selecione **Encontrar backups** na linha do armazenamento. O painel lista os arquivos de backup do armazenamento que não estão na sua lista de backups e lê o início de cada um (o `manifest.json` dele): do que ele faz backup, quando foi feito, o tamanho, a versão do PHP e os bancos de dados. Ele não baixa o resto.
4. Se alguns backups criptografados não abrirem com a frase secreta do armazenamento, informe a frase secreta com que eles foram enviados e selecione **Ler**.
5. Selecione **Adicionar à lista** em um backup, ou **Adicionar todos à lista (…)**, com o número de backups que o painel conseguiu ler. Nada muda no servidor: os backups agora estão na lista de **Backups**, guardados só no armazenamento.
6. Restaure cada um com **Restaurar**, como em [Restaurar um backup](./backups#restore-a-backup). **Restaurar** na lista de encontrados faz os passos 5 e 6 de uma vez.

Um backup que o painel não consegue ler fica na lista com o motivo, como uma frase secreta errada, um arquivo danificado ou um manifesto que cita outro site, diferente do da pasta em que ele está. **Procurar de novo** lê o armazenamento de novo.

Os backups encontrados em um armazenamento não têm checksum registrado na lista. A primeira restauração de cada um lê o arquivo compactado inteiro uma vez antes de alterar qualquer coisa, de modo que um arquivo danificado é recusado, e registra o checksum nesse momento.

## Restaurar um site que não existe aqui {#restoring-a-website-that-doesn-t-exist-here}

Quando o site de um backup não existe neste servidor, a janela de restauração avisa, e a restauração o cria:

- com o mesmo domínio e os mesmos aliases. Se um alias já for de outro site, o site é criado sem os aliases;
- com a mesma versão do PHP, ou a mais nova instalada aqui, com um aviso no log;
- marcado como WordPress (ou a outra aplicação), com as regras de reescrita e as diretivas próprias do servidor web, quando o backup as tem;
- com os arquivos, que passam a pertencer ao usuário do novo site;
- com os bancos de dados e os usuários deles. Os backups feitos pela v0.24.0 e posteriores guardam a senha de cada usuário do banco de dados, por isso a configuração do próprio site (`wp-config.php` e semelhantes) continua funcionando. Com um backup mais antigo, o usuário recebe uma nova senha: o WordPress a recebe no `wp-config.php` automaticamente; para outros softwares, copie-a de **Bancos de dados**.

O que não está em um backup e, portanto, não é criado de novo: o certificado SSL (obtenha um novo na aba **SSL / HTTPS** do site assim que o DNS dele apontar para o novo servidor), as tarefas cron, as contas SFTP, os programas em segundo plano, os redirecionamentos, a proteção por senha e as outras configurações do site.

Os backups feitos pelo Nefo 0.24.1 ou posterior registram o tipo de cada site: um site de app (Node.js, Python, Java) é criado de novo com o runtime, o comando de início, a porta e o limite de memória, e um site de proxy reverso, com o endereço e as opções. Instale antes o runtime do app na página **Software**; se ele faltar, a restauração para e avisa. Os backups feitos por versões mais antigas só registram a versão do PHP, e um site assim é criado de novo como um site estático: crie-o antes com o mesmo domínio e o mesmo tipo (veja [Sites](./websites#create-a-website)) e depois restaure; a restauração então só substitui os arquivos e os bancos de dados dele.

Uma restauração nunca substitui um site sem a confirmação: você digita o domínio na janela, exista o site ou não. Restaurar por cima de um site que existe substitui os arquivos e os bancos de dados dele, como sempre.

::: tip
O backup de um site guarda as senhas dos usuários dos bancos de dados dele, ao lado dos arquivos e dos dumps. Trate os backups como algo tão sensível quanto o próprio site. Criptografar as cópias remotas as mantém ilegíveis no armazenamento.
:::

## Outros tipos de backup {#other-kinds-of-backups}

| Tipo | Em um novo servidor |
|---|---|
| **Banco de dados** | Criado de novo se não existir, com a senha do usuário dele tirada do backup, quando o backup a tem |
| **Pasta** | Criada de novo se não existir |
| **App** | Instale antes o app pela loja de aplicativos (as imagens dele são baixadas de novo) e depois restaure |

## Descriptografar uma cópia manualmente {#decrypt-a-copy-by-hand}

Você também pode baixar um arquivo de backup do armazenamento por conta própria: pelo console do S3, com um cliente SFTP ou pelo Google Drive no navegador. Uma cópia criptografada (`.tar.gz.enc`) só precisa da frase secreta. Descriptografe-a com o programa `nefo`, em qualquer servidor Linux onde ele esteja instalado:

```sh
nefo decrypt-backup example.com_20261126-030000.tar.gz.enc
```

Ele pede a frase secreta e grava `example.com_20261126-030000.tar.gz` ao lado. Veja [Linha de comando](./cli#nefo-decrypt-backup).

O arquivo compactado tem os arquivos do site em `files/` e os dumps dos bancos de dados em `databases/`. Descompacte-o com `tar xzf`.

## Formato das cópias criptografadas {#format-of-encrypted-copies}

Para referência, e para descriptografar uma cópia sem o Nefo. Todos os números são big-endian.

| Deslocamento | Tamanho | Conteúdo |
|---|---|---|
| 0 | 7 | `NEFOENC` (ASCII) |
| 7 | 1 | Versão do formato: 1 |
| 8 | 1 | Derivação de chave: 1 = scrypt |
| 9 | 1 | Custo do scrypt, log2(N): 16 |
| 10 | 1 | r do scrypt: 8 |
| 11 | 1 | p do scrypt: 1 |
| 12 | 1 | Cifra: 1 = AES-256-GCM |
| 13 | 1 | log2(tamanho do bloco): 16, para 64 KiB |
| 14 | 2 | Zero |
| 16 | 32 | Salt, aleatório para cada arquivo |
| 48 | 12 | Verificação da chave |
| 60 | 4 | CRC-32 (IEEE) dos bytes 0 a 59 |

- scrypt(frase secreta, salt, N, r, p) gera 44 bytes. Os bytes 0–31 são a chave AES-256; os bytes 32–43 precisam ser iguais à verificação da chave, caso contrário a frase secreta está errada.
- Depois do cabeçalho de 64 bytes vêm os blocos. O arquivo compactado é cortado em pedaços do tamanho do bloco. O último pedaço é menor, possivelmente vazio, por isso há sempre tamanho ÷ bloco + 1 pedaços (arredondado para baixo, mais um).
- Cada pedaço é selado com AES-256-GCM, com o cabeçalho de 64 bytes como dados adicionais e o nonce: o número do pedaço a partir de 0 (8 bytes), 3 bytes zero e depois `01` para o último pedaço e `00` para os outros. Ele é guardado como o texto cifrado seguido da tag de 16 bytes.
- O tamanho do arquivo é 64 + tamanho + 16 × (tamanho ÷ 65536 + 1).
