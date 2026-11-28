---
title: Nefo e outros painéis
description: Como o Nefo se diferencia dos painéis de controle que a maioria dos donos de VPS usa, e quando outro painel é a melhor escolha.
---

# Nefo e outros painéis

O Nefo se diferencia da maioria dos painéis de controle de hospedagem em alguns pontos: todos os recursos são gratuitos, sem chave de licença nem conta no fornecedor; o próprio painel usa cerca de 34 MB de memória; o login fica em uma porta aleatória, atrás de um caminho secreto; e ele deixa de fora, de propósito, servidores de e-mail, DNS e FTP e contas de revenda.

Esta página compara o Nefo com os painéis de controle de hospedagem que as pessoas mais instalam em uma VPS. Ela não cita os nomes deles. Eles diferem entre si, por isso a coluna da direita diz quantos deles fazem algo: a maioria, muitos, alguns ou poucos.

A comparação foi conferida com os sites dos próprios painéis em novembro de 2026. Os painéis mudam os seus preços e recursos, por isso confira o painel que você está considerando antes de decidir.

## Visão geral {#at-a-glance}

| | Nefo | Outros painéis |
|---|---|---|
| Preço | Gratuito. Todos os recursos, sem edição paga | A maioria tem algo a pagar: uma licença por servidor, geralmente com preço pelo número de contas ou sites, ou uma edição paga ou complementos |
| Chave de licença ou conta no fornecedor | Nenhuma | Alguns exigem uma antes de rodar, e muitos exigem uma para os recursos pagos |
| Dados enviados ao fornecedor | Nenhum: sem telemetria, sem relatório de instalação, sem verificação de licença | Varia. Alguns coletam estatísticas de uso por padrão, e alguns instaladores informam o servidor ao fornecedor |
| Memória | O próprio painel usa cerca de 34 MB (medido em um servidor de teste rodando a v0.24.0). Feito para uma VPS com 1 GB de RAM | A maioria pede pelo menos 1 GB, e muitos recomendam 4 GB |
| Página de login | Uma porta aleatória e um caminho secreto. Qualquer outro endereço retorna `404` | A maioria usa uma porta fixa e conhecida. Poucos usam uma porta aleatória ou um caminho secreto |
| Atualizações do painel | Assinadas com Ed25519 e verificadas antes de instalar. Só quando você pede | Alguns documentam pacotes assinados; outros não dizem nada |
| Proteção contra bloqueio | Recusa regras de firewall e configurações de SSH que deixariam você trancado do lado de fora | Poucos têm uma para o firewall, como um retorno automático com prazo. Proteções para as configurações de SSH são raras |
| Código-fonte | Todo ele, sob a AGPLv3 | A maioria é fechada ou só tem o código-fonte disponível (source-available). Alguns são de código aberto |
| Servidores de e-mail e DNS | Não incluídos, de propósito | A maioria instala os dois |
| Vários servidores em um painel | Não. Um painel por servidor | Poucos gerenciam vários servidores em um painel, embora muitos possam compartilhar zonas DNS entre servidores |
| Contas de revenda e de clientes | Não. Uma conta de administrador | A maioria tem contas de revenda, e quase todos têm logins para clientes ou usuários |
| Sistemas operacionais | Ubuntu e Debian, em x86-64 e ARM64 | A maioria é só para Linux, e a maioria roda em mais distribuições Linux do que essas duas. Poucos rodam no Windows |
| Idade | Primeira versão pública em novembro de 2026. Beta | Muitos existem há 20 anos ou mais |

## O que o Nefo faz de diferente {#what-nefo-does-differently}

### Ele continua pequeno {#it-stays-small}

- O Nefo é um único programa, `/usr/local/bin/nefo`, de cerca de 25 MB, com a interface web embutida. Em um servidor de teste rodando a v0.24.0, ele usa cerca de 34 MB de memória. Os arquivos da interface são enviados compactados com gzip, e cada um dos sete idiomas só é baixado quando é usado.
- Ele não instala mais nada até você escolher. No primeiro login, uma página de configuração pergunta qual servidor web, quais versões do PHP, qual servidor de banco de dados e quais extras você quer, e mostra mais ou menos quanta memória e quanto disco a sua escolha acrescenta. O `nefo setup` faz o mesmo na linha de comando.
- Qualquer componente pode ser desinstalado de novo, e o menu esconde o que não está instalado. O Nefo se recusa a remover um componente que sites, bancos de dados ou contêineres ainda usam.
- Nenhum servidor de e-mail, DNS ou FTP roda em segundo plano, porque o Nefo não inclui nenhum.

### Todos os recursos são gratuitos, e nada é enviado ao projeto {#every-feature-is-free-and-nothing-phones-home}

- Não há edição paga, conta para criar nem chave de licença.
- Não há telemetria nem estatísticas de uso. Por conta própria, o painel só entra em contato com o Let's Encrypt, para emitir e renovar certificados, e com as versões do projeto no GitHub uma vez por dia, para o catálogo assinado da loja de aplicativos. Veja [Privacidade](./docs/#privacy).
- Ele nunca se atualiza sozinho, e só procura uma nova versão quando você pede.

### O login é difícil de encontrar {#the-sign-in-is-hard-to-find}

- O instalador escolhe uma porta aleatória entre 20000 e 49999 e um caminho secreto de 12 letras e dígitos. Qualquer outro endereço nessa porta retorna `404`.
- O painel usa HTTPS desde a primeira inicialização. O login aceita autenticação de dois fatores, e 5 tentativas malsucedidas em 15 minutos bloqueiam um endereço por 15 minutos. Os logins e as alterações vão para um log de atividades.
- As chaves de API para scripts podem ser somente leitura, limitadas aos endereços que você listar e configuradas para expirar. Nem mesmo uma chave com acesso total abre o terminal, as ferramentas de banco de dados ou as configurações da conta.

Veja [Segurança](./docs/security) e [Chaves de API](./docs/api).

### Ele recusa alterações que trancariam você do lado de fora ou quebrariam algo {#it-refuses-changes-that-would-lock-you-out-or-break-things}

- **Firewall:** a última regra que deixa o seu próprio endereço chegar ao SSH ou ao painel não pode ser excluída.
- **SSH:** configurações com as quais ninguém conseguiria fazer login são recusadas, como desativar o login por senha enquanto nenhuma conta tem uma chave.
- **Software:** um servidor web, uma versão do PHP ou um servidor de banco de dados que ainda está em uso não pode ser desinstalado.
- **Docker:** os projetos Compose são verificados antes de serem salvos ou iniciados. Contêineres privilegiados, portas abertas para a internet e montagens de pastas do sistema são recusados.

### Ele verifica o próprio servidor {#it-checks-its-own-server}

A [verificação de segurança](./docs/security-check) roda 22 verificações rápidas do servidor quando você pede ou uma vez por dia, como o login por senha no SSH, o firewall, atualizações de segurança pendentes, bancos de dados abertos para a internet, portas do Docker que escapam do ufw e arquivos que um site serve por engano, e liga cada ocorrência à página que a corrige. Ela não envia nada para fora do servidor.

### Os backups sobrevivem ao servidor {#backups-survive-the-server}

Os backups vão para o próprio servidor e para um armazenamento compatível com S3, um servidor SFTP ou o Google Drive, criptografados com uma frase secreta se você quiser. Um novo servidor os encontra no armazenamento e restaura os sites, com os seus bancos de dados. Veja [Restaurar em um novo servidor](./docs/restore-new-server).

### As versões são assinadas {#releases-are-signed}

A lista de checksums de cada versão é assinada com a chave Ed25519 do projeto. O instalador e o `nefo update` conferem a assinatura e o checksum antes de instalar qualquer coisa. A [política de segurança](./security-policy#check-a-release-yourself) mostra como verificar uma versão por conta própria.

### Você pode trazer os seus sites {#you-can-move-websites-in}

O Nefo importa sites dos backups de outros painéis: arquivos, bancos de dados com os seus usuários e senhas, certificados e tarefas cron. Veja [Importar sites](./docs/import) para os formatos que ele lê.

## Quando outro painel atende melhor {#when-another-panel-fits-you-better}

O Nefo deixa algumas coisas de fora de propósito (veja no [roadmap](./roadmap#not-planned-and-why) por quê). Escolha outro painel se você precisa de uma destas:

- **E-mail no mesmo servidor.** O Nefo não tem servidor de e-mail: nem caixas de correio, nem webmail, nem filtro de spam. Os sites e os alertas enviam e-mail pelo relay SMTP do seu provedor. A maioria dos outros painéis inclui um servidor de e-mail.
- **O seu próprio servidor DNS.** O Nefo não roda servidores de nomes. Ele gerencia os registros DNS dos seus domínios no Cloudflare.
- **FTP.** O Nefo não tem servidor FTP. Cada site pode ter uma conta SFTP, presa à pasta do site.
- **Muitos servidores em um só lugar.** Cada servidor roda o seu próprio Nefo, e você faz login em cada um separadamente.
- **Vender hospedagem.** O Nefo tem uma conta de administrador por servidor. Ele não tem revendedores, logins de clientes, planos de hospedagem nem cotas.
- **Um longo histórico, ou uma auditoria de segurança externa.** A primeira versão pública do Nefo saiu em novembro de 2026. Ele está em beta, e ainda não foi feita nenhuma auditoria de segurança externa.
- **Windows, ou um Linux que não seja Ubuntu nem Debian.** O Nefo roda só no Ubuntu 22.04, 24.04 e 26.04 e no Debian 12 e 13.

Se nada disso importa para você, [instale o Nefo](./docs/install) em um servidor que você possa reconstruir, e guarde uma cópia dos seus backups fora do servidor.
