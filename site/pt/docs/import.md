---
title: Importar sites
description: Traga sites do cPanel, aaPanel, HestiaCP, CyberPanel ou de outro servidor, com os seus arquivos, bancos de dados, certificados e tarefas cron.
---

# Importar sites

O Nefo importa sites de um backup completo do cPanel, com os bancos de dados e os usuários deles, os certificados e as tarefas cron, ou de um arquivo compactado do site e dumps SQL feitos pelo aaPanel, HestiaCP, CyberPanel ou qualquer outro servidor. Ele mostra um plano antes, não muda nada neste servidor até você confirmar, e remove o que criou se a importação falhar.

Há duas formas de importar:

- **Backup completo do cPanel**: todos os sites de uma conta do cPanel, com os bancos de dados e os usuários deles, os certificados e as tarefas cron.
- **Arquivo compactado do site e dumps SQL**: os arquivos de um site como um arquivo compactado `.tar.gz`, `.tgz` ou `.zip`, com dumps `.sql` ou `.sql.gz` opcionais. Use para o aaPanel, HestiaCP, CyberPanel ou qualquer outro servidor.

A importação nunca mexe no servidor antigo. O Nefo primeiro lê o arquivo compactado e mostra o que encontrou; nada muda neste servidor até você confirmar.

## Fazer o backup {#make-the-backup}

### cPanel {#cpanel}

No cPanel, abra **Files → Backup** (Arquivos → Backup) e clique em **Download a Full Account Backup** (baixar um backup completo da conta). Mantenha **Home Directory** (diretório inicial) como destino e clique em **Generate Backup** (gerar backup). O cPanel grava `backup-M.D.YYYY_HH-MM-SS_USER.tar.gz` na sua pasta pessoal e avisa você por e-mail quando ele estiver pronto.

Com acesso root ao WHM, rode `/scripts/pkgacct USER`: ele grava `/home/cpmove-USER.tar.gz`. Os dois arquivos têm a mesma estrutura, e o Nefo lê os dois, inclusive de versões antigas do cPanel.

Um backup parcial (os downloads "Home Directory" ou "MySQL Databases") não é um backup completo. Para esses, use **Arquivo compactado do site e dumps SQL**.

### aaPanel {#aapanel}

1. Abra **Website** (site), clique em **Backup** na linha do site e depois em **Backup now** (fazer backup agora). O aaPanel grava `/www/backup/site/DOMAIN/web_DOMAIN_YYYYMMDD_HHMMSS_xxxxxx.tar.gz` (versões mais antigas: `/www/backup/site/`).
2. Abra **Database** (banco de dados), clique em **Backup** na linha do banco de dados e depois em **Backup now**. O aaPanel grava `db_NAME_YYYYMMDD_HHMMSS_mysql_data.sql.gz` em `/www/backup/database/`.

O arquivo compactado do site tem a pasta do site no topo; o Nefo importa o que está dentro dela. O Nefo também lê a pasta antiga (`/www/wwwroot/DOMAIN`) no `.user.ini` do site e a substitui no banco de dados do WordPress.

### HestiaCP, CyberPanel e outros servidores {#hestiacp-cyberpanel-and-other-servers}

Os backups próprios deles empacotam cada site em arquivos compactados próprios (o HestiaCP os compacta com zstd), por isso faça um arquivo compactado simples, por SSH no servidor antigo:

```bash
# The website's files: the folder that holds index.php or index.html
tar czf /root/site.tar.gz -C /home/USER/web/example.com public_html
# Each database
mysqldump --single-transaction --routines --triggers DBNAME | gzip > /root/DBNAME.sql.gz
```

No HestiaCP a pasta é `/home/USER/web/DOMAIN/public_html`, no CyberPanel `/home/DOMAIN/public_html`, em um servidor comum geralmente `/var/www/html`. Um arquivo compactado cujo topo é uma pasta `public_html` também funciona: o Nefo importa o que está dentro dela.

## Colocar o arquivo compactado neste servidor {#get-the-archive-onto-this-server}

Qualquer uma destas formas:

- **SFTP ou scp** para uma pasta do root, por exemplo `scp backup.tar.gz root@NEW-SERVER:/root/`.
- **Arquivos**: envie o arquivo, ou use **Baixar de uma URL** (veja [Arquivos](./files)).
- **Um endereço web**: informe à importação um endereço `http` ou `https`. O Nefo baixa o arquivo para `/var/tmp/nefo-import`, no disco deste servidor, e o remove depois da importação. Endereços das próprias redes deste servidor são recusados.

O Nefo lê arquivos compactados de qualquer lugar, exceto das pastas do sistema (`/etc`, `/proc`, `/sys`, `/dev`, `/run`, `/boot`) e dos seus próprios dados. O disco precisa de espaço para os arquivos dos sites e cerca de duas vezes o tamanho dos dumps; o plano mostra o que é necessário e o que está livre.

## Importar passo a passo {#import-step-by-step}

1. Abra **Sites** e clique em **Importar**, ao lado de **Adicionar site**.
2. Escolha **Backup completo do cPanel** ou **Arquivo compactado do site e dumps SQL**.
3. Informe o caminho do arquivo (**Procurar** lista as pastas e os arquivos compactados deste servidor) ou o endereço web dele. Para um arquivo compactado do site, acrescente os dumps SQL.
4. Clique em **Analisar**. O Nefo lê o arquivo compactado inteiro uma vez, sem descompactá-lo; backups grandes levam alguns minutos. **Parar** interrompe a leitura.
5. Examine o plano e marque o que trazer (veja abaixo). Clique em **Importar**.
6. Acompanhe o log. Os novos sites ficam fechados (erro 503) até a importação terminar. **Parar a importação** remove o que foi criado até ali.
7. Quando terminar, a página lista os novos sites com **Abrir o site** e **Gerenciar**, os bancos de dados e o que merece atenção.

Se você sair da página, a importação continua; abra **Sites** → **Importar** de novo para vê-la. Um arquivo compactado analisado fica guardado por seis horas.

## O que é importado do cPanel {#what-is-imported-from-cpanel}

| No backup | No Nefo |
|---|---|
| O domínio principal, cada domínio adicional (addon) e cada subdomínio | Um site para cada um. O subdomínio interno do cPanel de um domínio adicional (`addon.main.com`) fica de fora. |
| Domínios estacionados (aliases) | Aliases do site do domínio principal, com o `www.` quando o servidor antigo o tinha. |
| Cada raiz do documento (`public_html`, `public_html/blog`, `addon.com`…) | A pasta `public` do site. Uma pasta de outro domínio importado dentro dela vai só para o site desse domínio. |
| A versão do PHP (`ea-php81`) | A mesma versão quando ela está instalada aqui; caso contrário, a padrão deste servidor. Um site sem arquivos PHP vira um site estático. |
| `mysql/DB.sql` | Um banco de dados com o mesmo nome quando ele está livre aqui; caso contrário, `NAME_2`, ligado ao site que o usa. |
| Os usuários dos bancos de dados e as senhas deles (`mysql.sql`) | Cada banco de dados recebe um usuário: o que o `wp-config.php` do WordPress cita; caso contrário, um com acesso a ele. O usuário mantém a senha antiga: o MariaDB aceita o hash da senha antiga como está, ou o Nefo usa a senha do `wp-config.php`. Caso contrário, o usuário recebe uma nova senha. |
| WordPress | Encontrado pelo `wp-config.php`. O Nefo marca o site como WordPress, aplica as regras de reescrita do WordPress, aponta o `wp-config.php` para o novo nome do banco de dados, usuário ou senha quando eles mudaram, e substitui a pasta antiga (`/home/USER/public_html`) pela nova no banco de dados. |
| Certificados (`apache_tls/`, `sslcerts/`, `ssl/`) | Instalados como o seu próprio certificado quando você os marca: só quando a chave está no backup, o certificado é válido hoje e cobre todos os nomes do site. Caso contrário, obtenha um do Let's Encrypt depois da importação. |
| Tarefas cron (`cron/USER`) | Mostradas, e instaladas só quando você as marca: com o próprio usuário do site (nunca o root), com as raízes do documento antigas e os programas PHP do cPanel substituídos pelos deste servidor. Um comando que ainda cita a pasta pessoal antiga fica marcado. |
| `.htaccess`, `.user.ini` | A pasta antiga é substituída pela nova, e as linhas do handler PHP do cPanel saem do `.htaccess`. |

O plano diz por que um domínio não pode ser importado, por exemplo quando um site daqui já tem o domínio.

## O que não é importado {#what-is-not-imported}

- **E-mail**: caixas de correio, encaminhadores, respostas automáticas e filtros. O Nefo não tem servidor de e-mail. Leve o e-mail para um serviço de e-mail antes de desligar o servidor antigo. Para o e-mail que os sites enviam, veja [E-mail e alertas](./mail-alerts).
- **Zonas DNS**: o Nefo não roda servidor DNS. Aponte os domínios para este servidor onde o DNS deles é gerenciado, ou com [DNS (Cloudflare)](./dns).
- **Contas FTP**: os sites daqui usam SFTP, uma conta por site (a aba **SFTP** do site).
- Listas de e-mail, bancos de dados PostgreSQL, os arquivos da pasta pessoal fora das raízes do documento e as próprias configurações do cPanel.

## Arquivos compactados de sites {#website-archives}

- Uma única pasta no topo do arquivo compactado (como nos backups do aaPanel) não é mantida: o que está dentro dela vira o site. Uma pasta pessoal com `public_html` dentro também funciona.
- O site pode ser um novo (o domínio, o `www.` e a versão do PHP são perguntados) ou um existente vazio: um site PHP ou estático cuja pasta `public` não tem nada além da página de boas-vindas e cujos arquivos não estão [bloqueados](./website-protection#lock-a-website-s-files).
- Cada dump SQL vira um novo banco de dados com um usuário do mesmo nome e uma nova senha. O `wp-config.php` de um WordPress é apontado para o dump que ele cita (ou para o único). Para outras aplicações, mude a configuração delas para o novo banco de dados, usuário e senha (**Mostrar login** na página **Bancos de dados**).

## Dumps do MySQL 8 {#dumps-from-mysql-8}

Os dumps feitos pelo MySQL 8, que o aaPanel e muitos servidores cPanel rodam, usam collations que o MariaDB não conhece, como `utf8mb4_0900_ai_ci`, e linhas que o MariaDB recusa. O Nefo os adapta enquanto eles são lidos, sem alterar os seus dados:

- As collations do MySQL 8 viram a mais próxima do MariaDB (`utf8mb4_unicode_520_ci`, `utf8mb4_bin` para as que diferenciam maiúsculas e minúsculas, ou a do próprio idioma, como `utf8mb4_vietnamese_ci`).
- Configurações que só o MySQL 8 conhece, como `DEFAULT ENCRYPTION='N'` e `SRID`, viram comentários simples.
- O `SET @@GLOBAL.GTID_PURGED` e as linhas `SQL_LOG_BIN` em volta dele ficam de fora.
- O `CREATE DATABASE` e o `USE` do próprio banco de dados do dump ficam de fora, para que ele vá para o novo banco de dados.

O texto dentro dos seus dados nunca é alterado. O log diz quantas linhas foram adaptadas.

## Como o Nefo trata os arquivos compactados {#how-nefo-treats-archives}

Um arquivo compactado de outro servidor pode conter qualquer coisa, por isso o Nefo confere cada entrada:

- Nomes com `..` ou um caminho absoluto, links simbólicos, hard links, dispositivos e FIFOs ficam de fora, e o log diz quantos.
- Os arquivos perdem o setuid e o setgid, e a permissão de gravação para o grupo e os outros. Tudo pertence ao próprio usuário Linux do site.
- Os arquivos são gravados só pela pasta do site: nada pode ir parar fora dela.
- Um arquivo com mais de 8 GB fica de fora; um arquivo compactado que se expande para muito mais do que o próprio tamanho (uma bomba de compactação) é recusado.
- O SQL roda com uma conta temporária do banco de dados que só pode usar o novo banco de dados, nunca como root.

Se a importação falhar ou for interrompida, o Nefo remove os sites e os bancos de dados que criou. Um site que ele não conseguiu remover fica suspenso, com uma nota dizendo que a importação não terminou.

## Depois da importação {#after-the-import}

1. **Teste antes do DNS**: aponte os domínios para este servidor no arquivo `hosts` do seu computador e abra os sites.
2. **DNS**: aponte o registro A (e AAAA) de cada domínio para este servidor.
3. **HTTPS**: assim que o DNS apontar para cá, obtenha certificados Let's Encrypt na aba **SSL / HTTPS** de cada site. Um certificado importado não é renovado pelo Nefo. Veja [Certificados SSL](./ssl).
4. **PHP**: confira a versão e as configurações do PHP de cada site na aba **PHP** dele.
5. **Tarefas cron**: acrescente as que você não importou na página **Tarefas cron**.
6. **E-mail**: confira se o e-mail funciona em outro lugar antes de desligar o servidor antigo.

## Se algo der errado {#if-something-goes-wrong}

- **"não é um backup completo do cPanel"**: o arquivo compactado não tem `userdata/main`. É um backup parcial ou um arquivo compactado simples: escolha **Arquivo compactado do site e dumps SQL**.
- **"usa outro banco de dados"**: o dump grava em um segundo banco de dados. Exporte cada banco de dados separadamente, sem `--databases` nem `--all-databases`.
- **"Não há espaço livre suficiente"**: libere espaço, ou importe menos sites de cada vez.
- **Um site WordPress mostra "Error establishing a database connection"** (erro ao estabelecer uma conexão com o banco de dados): abra o `wp-config.php` dele em **Arquivos** e compare `DB_NAME`, `DB_USER` e `DB_PASSWORD` com **Mostrar login** na página **Bancos de dados**.
