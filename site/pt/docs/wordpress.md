---
title: Ferramentas do WordPress
description: Verifique e atualize os seus sites WordPress com um backup do banco de dados antes, e use o modo de manutenção, o modo de depuração, as atualizações automáticas, a redefinição de senhas e as cópias de staging.
---

# Ferramentas do WordPress

As ferramentas do WordPress do Nefo mostram a versão e as atualizações pendentes de cada site WordPress, e atualizam o WordPress, os plugins e os temas com um clique, depois de um backup do banco de dados. Elas também ativam e desativam o modo de manutenção, o modo de depuração e as atualizações automáticas, redefinem senhas de administradores e fazem uma cópia de staging que você pode enviar de volta para o site em produção.

Elas funcionam para os sites WordPress que o Nefo instalou: pela [loja de aplicativos](./apps), com o botão **Instalar WordPress** de um site PHP vazio, ou como uma cópia de staging. Um WordPress que você mesmo enviou não aparece. O item **WordPress** do menu à esquerda aparece assim que o servidor tem um site WordPress.

As ferramentas usam o wp-cli, que o Nefo baixa no primeiro uso a partir da versão mais recente dele e confere com o checksum SHA-512 publicado da versão. **Software** → **Ferramentas** mostra a versão dele e o atualiza (veja [Software](./software#tools)). O wp-cli roda com o próprio usuário Linux de cada site, com a versão do PHP do site, e não carrega plugins nem temas, por isso um plugin quebrado não atrapalha.

## Instalar o WordPress {#install-wordpress}

Duas formas:

- **Loja de aplicativos**: instale o WordPress ou o WooCommerce em um site PHP vazio. Veja [Loja de aplicativos](./apps).
- **A partir de um site PHP vazio**: abra a aba **Visão geral** do site e clique em **Instalar WordPress**. Preencha **Título do site**, **Nome de usuário do administrador** (evite `admin`: é o primeiro nome que os atacantes tentam), **E-mail do administrador** e **Senha do administrador**, ou deixe a senha vazia para receber uma forte. Uma senha que você digita precisa ter de 12 a 128 caracteres, como na loja de aplicativos. Clique em **Instalar WordPress**.

O WordPress precisa do MariaDB ou do MySQL: instale-o antes na página **Bancos de dados** (veja [Bancos de dados](./databases)). Ative o HTTPS antes de instalar, para que o WordPress comece com o endereço `https://` dele; se você ativá-lo depois, o Nefo faz a troca no WordPress para você. Guarde os dados de login quando eles aparecerem: a senha não é mostrada de novo.

## A lista de sites {#the-site-list}

Abra **WordPress** no menu à esquerda. Para cada site, a tabela mostra:

| Coluna | O que mostra |
|---|---|
| **Site** | O domínio e a versão do PHP. Clique nele para abrir a aba **WordPress** do site. |
| **Versão** | A versão do WordPress, com "WordPress … disponível" quando há uma mais nova, ou "Atualizado". |
| **Plugins e temas** | Quantos há e quantos têm atualização. **Mostrar as … atualizações disponíveis** os lista. |
| **Modos** | **Manutenção**, **Depuração** e **Atualizações automáticas**, quando estão ativados. |

O Nefo verifica cada site quando a página abre. **Verificar de novo**, no topo, verifica todos os sites de novo; o mesmo item no menu de um site verifica só esse site. Se um site não puder ser verificado, por exemplo porque o banco de dados dele está fora do ar, a linha diz "Não foi possível verificar o site:" e por quê.

O menu no final de cada linha contém as ações abaixo. As mesmas ações e chaves ficam na aba **WordPress** de cada site, junto com o **WP Admin**.

## Atualizar o WordPress, os plugins e os temas {#update-wordpress-plugins-and-themes}

1. Clique em **Atualizar tudo** na linha do site, ou na aba **WordPress** do site.
2. Confirme: "Fazer backup do banco de dados de example.com e depois atualizar o WordPress, todos os plugins e todos os temas?"

A atualização roda em segundo plano, com um log:

1. O Nefo exporta o banco de dados para `/var/www/example.com/tmp/panel-wp-backup-YYYYMMDD-HHMMSS.sql`. Só o usuário do site pode ler o arquivo, e ele não fica acessível pela web. O Nefo guarda os três mais recentes desses arquivos. Se o backup falhar, nada é atualizado.
2. Ele atualiza o WordPress e o banco de dados dele, depois todos os plugins e todos os temas.
3. Um site que estava em modo de manutenção continua nele.

Se alguns plugins ou temas não puderem ser atualizados, o log diz por quê e cita o backup do banco de dados. Em um site cujos [arquivos estão bloqueados](./website-protection#lock-a-website-s-files), as ferramentas os desbloqueiam para a atualização, o modo de manutenção, o modo de depuração e as atualizações automáticas, e os bloqueiam de novo depois. O próprio WordPress não consegue atualizar nada enquanto os arquivos estão bloqueados: atualize por aqui. Para voltar atrás, baixe o backup em **Arquivos** e importe-o na página **Bancos de dados**: veja [Bancos de dados](./databases). Para uma cópia completa dos arquivos também, use os [Backups](./backups).

## Modo de manutenção {#maintenance-mode}

Escolha **Ativar o modo de manutenção**, ou ative a chave **Modo de manutenção** na aba **WordPress** do site. Os visitantes passam a ver a página de manutenção do WordPress (HTTP 503). Ela continua ativa até você desativá-la no painel com **Desativar o modo de manutenção**.

O modo de manutenção é a página do próprio WordPress. Para fechar o site inteiro no servidor web, suspenda-o: veja [Sites](./websites).

## Modo de depuração {#debug-mode}

Escolha **Ativar o modo de depuração**, ou ative a chave **Modo de depuração**. O Nefo define `WP_DEBUG` e `WP_DEBUG_LOG` no `wp-config.php` e mantém o `WP_DEBUG_DISPLAY` desativado, para que os erros sejam gravados nos logs e nunca mostrados aos visitantes:

- os erros das requisições web vão para `/var/www/example.com/logs/php-error.log`: abra a aba **Logs** do site e escolha **Erros do PHP**;
- os erros das execuções do wp-cli vão para `/var/www/example.com/logs/wp-debug.log`: abra-o em **Arquivos**.

Desative-o com **Desativar o modo de depuração** quando terminar.

## Atualizações automáticas {#automatic-updates}

**Ativar as atualizações automáticas** deixa o WordPress atualizar tudo sozinho: o WordPress, inclusive as versões principais, todos os plugins e todos os temas.

**Desativar as atualizações automáticas** volta o WordPress ao padrão dele: ele continua instalando as versões de segurança e as versões menores do próprio WordPress, e os plugins e os temas não se atualizam sozinhos.

A configuração vale para os plugins e os temas instalados no momento em que você a ativa. Para plugins e temas que você acrescentar depois, desative-a e ative-a de novo.

## Redefinir a senha de um administrador {#reset-an-administrator-password}

1. Escolha **Redefinir a senha de um administrador**.
2. Escolha a conta em **Administrador**. A lista mostra os usuários com o papel de administrador.
3. Escolha **Gerar uma senha forte**, ou **Digitar uma senha** com 12 a 128 caracteres.
4. Clique em **Definir senha**.

O Nefo mostra a nova senha uma única vez, com um botão **Entrar**. A sessão da conta é encerrada em todos os dispositivos.

## Fazer uma cópia de staging {#make-a-staging-copy}

Uma cópia de staging é um segundo site com uma cópia do site, onde você pode testar alterações sem mexer no site em produção. O site em produção continua rodando durante a cópia.

1. Escolha **Clonar para staging**.
2. Confira o **Domínio do site de staging**. Ele é `staging.example.com`, a menos que você o mude.
3. Clique em **Criar cópia de staging**.
4. Aponte o DNS do domínio de staging para este servidor e depois ative o HTTPS na aba **SSL / HTTPS** dele.

A cópia recebe:

- um novo site PHP com a mesma versão do PHP, e o seu próprio usuário Linux;
- um novo banco de dados com uma cópia do conteúdo;
- uma cópia dos arquivos de `public`, com o `wp-config.php` apontado para o novo banco de dados. Os drop-ins de cache (`object-cache.php`, `advanced-cache.php`) e os links simbólicos ficam de fora;
- o endereço do site substituído pelo novo em todo o banco de dados;
- um pedido aos mecanismos de busca para não indexá-la;
- as mesmas regras de reescrita do site em produção.

O site de staging começa em `http://`, sem certificado. Se algum passo falhar, o Nefo remove tudo o que criou para a cópia.

O Nefo se recusa a fazer uma cópia quando:

- o MariaDB não está rodando;
- o domínio de staging já é usado por um site, ou a pasta dele existe em `/var/www`;
- o `wp-config.php` não está na pasta `public` do site;
- o `DB_NAME` no `wp-config.php` não é um valor simples entre aspas, ou o nome do banco de dados usa caracteres que não sejam letras minúsculas, dígitos e `_`;
- o banco de dados do site está em outro servidor;
- o site é uma rede multisite.

::: tip
Qualquer pessoa pode abrir o site de staging assim que o DNS dele apontar para o servidor. Para mantê-lo privado, proteja-o com uma senha na aba **Bloquear acesso** dele: veja [Sites](./websites).
:::

## Enviar a cópia de staging para o site em produção {#push-the-staging-copy-to-the-live-site}

Quando as alterações que você testou na cópia de staging estiverem prontas, copie-as de volta por cima do site em produção. O Nefo lembra de qual site cada cópia de staging foi feita. As cópias feitas antes de o Nefo ter este botão, e os [clones](./websites#clone-a-website) de um site, não estão ligados a um site em produção e não podem ser enviados.

1. Na linha do site de staging, escolha **Enviar para produção**, ou clique em **Enviar para produção** na aba **WordPress** dele. A lista de sites mostra "Cópia de staging de example.com" embaixo do site de staging.
2. Escolha o que enviar:
   - **Arquivos e banco de dados**;
   - **Só arquivos (temas, plugins, uploads)**: o site em produção mantém os posts, as páginas, os pedidos e as configurações dele;
   - **Só banco de dados (posts, páginas, configurações)**: o site em produção mantém os arquivos dele.
3. Digite o domínio do site em produção para confirmar e clique em **Enviar para produção**.

O envio roda como uma tarefa com o seu próprio log:

1. O Nefo faz backup dos arquivos e do banco de dados do site em produção. O backup fica na página [Backups](./backups). Se o backup falhar, nada é enviado.
2. O site em produção mostra a página de manutenção dele. Se ele já estava em modo de manutenção, continua nele depois do envio.
3. **Arquivos**: os arquivos da pasta `public` do site de staging substituem os do site em produção e passam a pertencer ao usuário Linux do site em produção. Os arquivos que a cópia de staging não tem são excluídos do site em produção; os arquivos com o mesmo tamanho e a mesma data ficam como estão. O site em produção mantém o seu próprio `wp-config.php`, os seus drop-ins de cache (`object-cache.php`, `advanced-cache.php`) e os seus links simbólicos. No `.user.ini` e no `wordfence-waf.php`, a pasta de staging é substituída pela de produção.
4. **Banco de dados**: o banco de dados de produção é esvaziado e recebe uma cópia do banco de dados de staging. O Nefo então desfaz o que a cópia de staging fez: o endereço de staging (com `http://` e `https://`, também em JSON) e a pasta de staging são substituídos pelos de produção em todos os lugares, e o site em produção recebe de volta o seu próprio endereço e a sua configuração para os mecanismos de busca. O cache de objetos é esvaziado, e o WordPress monta de novo as regras de reescrita na próxima visita.
5. O modo de manutenção é desativado, também quando um passo falhou.

::: warning
Um envio substitui o site em produção. O que foi acrescentado ao site em produção desde que a cópia de staging foi feita se perde: pedidos, comentários e novos usuários quando você envia o banco de dados, uploads quando você envia os arquivos. Se o site em produção não funcionar depois de um envio, restaure o backup do passo 1 na página **Backups**.
:::

O Nefo se recusa a enviar quando:

- o site não é uma cópia de staging, ou o site em produção dela foi excluído;
- você envia o banco de dados e o MariaDB não está rodando, o banco de dados de produção não está na página **Bancos de dados**, os dois sites usam o mesmo banco de dados, os prefixos das tabelas (`$table_prefix`) são diferentes, ou um motivo da lista acima vale para um dos dois `wp-config.php`.

As configurações do `wp-config.php` nunca são enviadas: altere-as você mesmo no site em produção.

## Excluir uma cópia de staging {#delete-a-staging-copy}

Para excluir a cópia de staging, exclua o site dela e marque **Excluir também os bancos de dados dele**: veja [Excluir um site](./websites#delete-a-website). Excluir uma cópia de staging não muda o site em produção dela.

Excluir o site em produção mantém a cópia de staging dele, mas a cópia não pode mais ser enviada: a janela de exclusão diz "Este é o site em produção da cópia de staging …".
