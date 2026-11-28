---
title: Arquivos
description: Navegue, edite, envie, baixe, pesquise, compacte e extraia arquivos no navegador, e restaure itens excluídos da lixeira.
---

# Arquivos

A página **Arquivos** do Nefo é um gerenciador de arquivos no navegador: navegue, edite arquivos de texto de até 2 MB, envie até 4 GB de cada vez, baixe, busque um arquivo a partir de uma URL, pesquise por nome ou conteúdo, compacte e extraia, e restaure itens excluídos de uma lixeira de 30 dias. Dentro da pasta de um site, ele trabalha como o usuário do site e não consegue sair da pasta.

Esta página trata de cada um desses pontos. O painel roda como root, por isso ela também explica aonde você pode ir e o que ele se recusa a alterar.

## Como se localizar {#find-your-way}

O gerenciador de arquivos abre em `/var/www`, onde cada site tem a sua pasta, `/var/www/example.com`.

- Selecione o nome de uma pasta para abri-la. **Subir** vai para a pasta acima e **Atualizar** lê a pasta de novo.
- Cada parte do caminho no topo é clicável. O botão da casa (**Sites**) leva você de volta a `/var/www`.
- Para ir direto a qualquer pasta, selecione o botão do lápis (**Digitar um caminho**), digite um caminho absoluto como `/etc/nginx` e selecione **Ir**.
- A tabela mostra **Nome**, **Tamanho**, **Permissões**, **Proprietário** e **Modificado**. Uma pasta com mais de 5.000 itens mostra só os primeiros 5.000.

## Aonde você pode chegar {#what-you-can-reach}

Fora das pastas dos sites, você trabalha como root no sistema de arquivos inteiro.

Dentro da pasta de um site, `/var/www/example.com`, cada operação fica restrita a essa pasta, e um selo com o domínio aparece ao lado do caminho:

- Os caminhos não conseguem subir para fora com `..`, e os links simbólicos não conseguem apontar para fora da pasta. Um site invadido que plante um link para `/etc` não consegue enganar o painel para mexer em arquivos do sistema.
- Os novos arquivos e pastas pertencem ao usuário do site, para que o PHP dele possa usá-los.
- Mover, compactar ou extrair entre a pasta de um site e qualquer outra pasta é recusado. Baixe o arquivo e envie-o para onde você precisa.
- Enquanto os [arquivos do site estiverem bloqueados](./website-protection#lock-a-website-s-files), você só pode alterar as pastas graváveis dele e as pastas `tmp` e `logs`. No resto da pasta do site, o gerenciador de arquivos recusa, com uma mensagem de que os arquivos estão bloqueados.

Estas pastas não podem ser excluídas, renomeadas nem ter as permissões alteradas:

- `/`, `/bin`, `/boot`, `/dev`, `/etc`, `/home`, `/lib`, `/lib64`, `/opt`, `/proc`, `/root`, `/run`, `/sbin`, `/srv`, `/sys`, `/tmp`, `/usr`, `/var`;
- `/var/www`, `/var/lib`, `/var/log`, `/var/lib/nefo` e `/etc/nefo`;
- a própria pasta de cada site, como `/var/www/example.com` (os arquivos dentro dela podem ser alterados);
- a pasta da lixeira, `/var/lib/panel-trash`, e as pastas que a contêm.

## Editar um arquivo {#edit-a-file}

1. Selecione o nome do arquivo. Ele abre no editor.
2. Faça as alterações. Ctrl+S salva, e a tecla Tab insere uma tabulação.
3. Selecione **Salvar** e depois **Fechar**.

O editor abre arquivos de texto de até 2 MB. Arquivos binários e arquivos maiores são recusados: baixe-os. Um arquivo salvo mantém as permissões e o proprietário. Se você fechar o editor com alterações não salvas, o painel pergunta antes.

Para criar um arquivo, selecione **Novo arquivo**, informe um nome em **Nome do novo arquivo** e selecione **OK**. Para criar uma pasta, selecione **Nova pasta**. Os novos arquivos recebem as permissões `0644`, e as novas pastas, `0755`.

## Enviar arquivos {#upload-files}

1. Abra a pasta para a qual você quer enviar.
2. Selecione **Enviar** e escolha um ou mais arquivos.
3. Se já existir um arquivo com o mesmo nome, o painel pergunta antes de substituí-lo.

Uma barra de progresso mostra o envio. Um envio pode ter até 4 GB no total. Os arquivos enviados recebem as permissões `0644`. Dentro da pasta de um site, eles pertencem ao usuário do site; nas outras, ao root.

## Baixar um arquivo {#download-a-file}

Abra o menu de um arquivo e selecione **Baixar**. Só arquivos podem ser baixados. Para baixar uma pasta, compacte-a antes.

## Baixar de uma URL {#download-from-a-url}

O servidor pode buscar um arquivo na internet direto para uma pasta, sem passar pelo seu computador.

1. Abra a pasta de destino e selecione **Baixar de uma URL**.
2. Informe o **Endereço do arquivo (URL)** e selecione **Baixar**.
3. Um log mostra o progresso. Você pode selecionar **Ocultar (o download continua)**.

As regras:

- Só endereços `http` e `https`, com no máximo 5 redirecionamentos.
- Arquivos de até 2 GB.
- O painel nunca baixa da rede do próprio servidor: endereços de loopback, privados e link-local (como `127.0.0.1`, `10.x.x.x` e `169.254.169.254`) e os próprios endereços do servidor são recusados. Isso é conferido de novo a cada redirecionamento.
- O nome do arquivo vem do servidor ou da URL. Um arquivo existente nunca é substituído: o painel acrescenta ` (1)`, ` (2)` e assim por diante.
- O arquivo pertence ao proprietário da pasta, com as permissões `0640`.

## Pesquisar {#search}

1. Abra a pasta em que você quer pesquisar.
2. Digite na caixa de pesquisa (**Pesquisar nesta pasta**) e pressione Enter.
3. Na janela, ajuste **O nome contém**. Para pesquisar texto dentro dos arquivos, marque **Pesquisar também dentro dos arquivos** e preencha **Texto no arquivo**.
4. Selecione **Pesquisar**.

Maiúsculas e minúsculas são tratadas da mesma forma. Selecione um resultado para abrir a pasta dele. Um arquivo de texto também abre no editor.

Os limites evitam que uma única pesquisa ocupe o servidor:

| Limite | Valor |
|---|---|
| Itens examinados | 20.000 |
| Resultados | 200 |
| Tempo | 20 segundos |
| Pesquisa de texto | Pula arquivos binários e arquivos com mais de 2 MB |
| Tamanho da busca | 200 caracteres |

Os links simbólicos são listados, mas nunca seguidos. Uma pesquisa a partir de `/` pula `/proc`, `/sys` e `/dev`. Quando um limite é atingido, o painel avisa: pesquise em uma pasta menor ou use palavras mais precisas.

## Compactar e extrair {#compress-and-extract}

### Compactar {#compress}

1. Marque os arquivos e as pastas a empacotar.
2. Selecione **Compactar** na barra de seleção.
3. Informe o **Nome do arquivo compactado (.zip ou .tar.gz)** e selecione **OK**.

O arquivo compactado é criado na pasta atual. Os links simbólicos e os arquivos especiais ficam de fora.

### Extrair {#extract}

1. Abra o menu de um arquivo compactado e selecione **Extrair**.
2. O campo contém a pasta atual. Mude-o para extrair em outro lugar e depois selecione **OK**.

O painel extrai arquivos compactados `.zip`, `.tar.gz`, `.tgz` e `.tar`:

- Os arquivos existentes com o mesmo nome são sobrescritos.
- Os links simbólicos e os arquivos especiais do arquivo compactado são pulados.
- Os arquivos recebem as permissões `0644`, ou `0755` quando eram executáveis. Eles pertencem ao usuário do site dentro da pasta de um site, e ao root nas outras.
- Arquivos compactados com mais de 20 GB de conteúdo ou mais de 500.000 arquivos são recusados.

Compactar e extrair rodam como tarefas com um log ao vivo, por isso continuam se você sair da página.

## Renomear ou mover {#rename-or-move}

Abra o menu de um item e selecione **Renomear**, depois informe o novo nome e selecione **OK**. O novo nome é lido em relação à pasta atual, por isso um caminho como `old/index.html` ou `../index.html` move o item dentro do mesmo site ou da mesma parte do servidor.

## Mudar as permissões {#change-permissions}

Selecione as permissões de um item na coluna **Permissões**, ou abra o menu dele e selecione **Permissões**. Informe o novo valor em octal, por exemplo `0644` para arquivos ou `0755` para pastas, e selecione **OK**.

Os valores vão até `0777`. Os bits setuid, setgid e sticky não podem ser definidos aqui.

A coluna **Proprietário** mostra o proprietário e o grupo. O gerenciador de arquivos não muda proprietários. Use o [terminal](./system#terminal) para isso, por exemplo `chown -R example_com:example_com /var/www/example.com/public` com o próprio usuário do site.

## Excluir arquivos e a lixeira {#delete-files-and-the-trash}

1. Marque os itens, ou abra o menu de um deles, e selecione **Excluir**.
2. Por padrão, os itens vão para a lixeira: selecione **Mover para a lixeira**.
3. Para não passar pela lixeira, marque **Excluir definitivamente (sem passar pela lixeira)** e selecione **Excluir definitivamente**.

::: danger
Os itens excluídos definitivamente, inclusive as pastas com todo o seu conteúdo, não podem ser recuperados.
:::

### Restaurar da lixeira {#restore-from-the-trash}

1. Abra uma pasta do site cujos arquivos você quer de volta, ou qualquer pasta fora dos sites para o resto do servidor.
2. Selecione **Lixeira**.
3. Selecione **Restaurar** ao lado do item.

Cada site tem a sua própria lixeira, e o resto do servidor compartilha uma. O item volta para o caminho de onde veio, com o proprietário e as permissões dele. As pastas acima que faltarem são criadas. Se agora houver outra coisa nesse caminho, a restauração é recusada: tire-a de lá antes.

Na lixeira, você também pode **Excluir** um item para sempre, ou selecionar **Esvaziar a lixeira**. A lista mostra os 1.000 itens excluídos mais recentemente.

### Por quanto tempo os itens ficam {#how-long-items-stay}

Os itens ficam na lixeira por **30 dias**. O painel confere a cada hora e remove para sempre os itens mais antigos.

- A lixeira fica em `/var/lib/panel-trash/<domain>`, ou em `/var/lib/panel-trash/_server` para o resto do servidor. Só o root pode lê-la, por isso o usuário de um site não consegue chegar aos arquivos excluídos dele.
- Os itens na lixeira continuam ocupando espaço em disco. Exclua definitivamente os arquivos muito grandes.
- Um item com mais de 1 GB que fica em um disco diferente do da lixeira não pode ser movido para lá. Exclua-o definitivamente.
- Excluir um site também exclui a lixeira dele.
