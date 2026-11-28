---
title: Sobre o Nefo
description: O que é o Nefo, para quem ele é, os princípios em que se baseia, o seu estado e a sua licença, e como obter ajuda ou relatar um bug.
---

# Sobre o Nefo

O Nefo é um painel de controle gratuito e de código aberto para um servidor Linux. Você o instala na sua própria VPS com um único comando e depois gerencia sites, certificados, bancos de dados, apps, arquivos, backups e segurança pelo navegador, em um de sete idiomas. Ele está em beta, é licenciado sob a GNU AGPLv3 e foi feito para uma VPS com 1 GB de RAM.

É um único programa, `/usr/local/bin/nefo`, com a interface web embutida, que roda como o serviço systemd `nefo`. Ele roda no Ubuntu 22.04, 24.04 e 26.04 e no Debian 12 e 13, em x86-64 e ARM64. A loja de aplicativos tem 42 apps, e a interface está em inglês, vietnamita, chinês, espanhol, português, russo e indonésio. Veja [O que é o Nefo?](./docs/) para tudo o que ele faz.

## Para quem ele é {#who-it-is-for}

O Nefo é para quem administra a própria VPS: alguém montando um primeiro servidor para um blog, um desenvolvedor fazendo deploy de um app, ou um freelancer ou agência que cuida de sites de clientes.

Cada servidor tem uma única conta do painel, `admin`. O Nefo não foi feito para vender hospedagem: ele não tem contas de revenda nem de clientes. Veja [Nefo e outros painéis](./compare) para saber quando outro painel atende melhor.

## Princípios {#principles}

- **Leve.** O Nefo foi feito para uma VPS com 1 GB de RAM, e o próprio painel usa cerca de {{ $release.ram }} de memória (medido em um servidor de teste rodando a v{{ $release.version }}). Ele não instala nada até você escolher, qualquer componente pode ser desinstalado de novo, e os recursos que manteriam serviços grandes rodando o tempo todo, como um servidor de e-mail, ficam de fora.
- **Todos os recursos são gratuitos.** Não há edição paga nem recurso bloqueado. Você não cria uma conta, e não há chave de licença.
- **Nada é enviado ao projeto.** Sem telemetria, sem estatísticas de uso, sem verificação de licença. A interface não carrega nada de outros sites. Por conta própria, o painel só entra em contato com o Let's Encrypt, para os certificados, e com as versões do projeto no GitHub, para o catálogo assinado da loja de aplicativos, e só procura atualizações quando você pede. Veja [Privacidade](./docs/#privacy).
- **Honesto sobre os seus limites.** A documentação diz o que o Nefo não faz, e o que ele se recusa a fazer e por quê. O [roadmap](./roadmap) lista o que não está planejado com a mesma clareza do que está.

## Estado {#status}

O Nefo está em beta. A primeira versão pública, a v0.20.0, saiu em 26 de novembro de 2026, e a versão atual é a v{{ $release.version }} ({{ $release.date }}). Veja o [registro de alterações](./changelog).

- Os números de versão começam com `0.`. Até a versão 1.0, só a versão mais recente recebe correções de segurança.
- Ainda não foi feita nenhuma auditoria de segurança externa. Veja a [política de segurança](./security-policy).
- O instalador foi testado em servidores recém-instalados com Ubuntu 22.04, 24.04 e 26.04 e Debian 12 e 13, em x86-64. O ARM64 ainda não foi testado em hardware ARM real.

Experimente o Nefo em um servidor que você possa reconstruir antes de confiar nele para sites importantes, e guarde cópias dos seus backups fora do servidor.

## Licença {#licence}

O Nefo é software livre sob a [GNU AGPLv3](https://github.com/nefocp/nefo/blob/main/LICENSE). Você pode usá-lo para qualquer fim, em quantos servidores quiser, e ler, alterar e compartilhar o seu código. Se você oferecer um Nefo modificado a outras pessoas por uma rede, a AGPL exige que você publique as suas alterações.

O código-fonte está no [GitHub](https://github.com/nefocp/nefo).

## Obter ajuda e relatar bugs {#get-help-and-report-bugs}

- **Leia a documentação primeiro.** [Solução de problemas](./docs/troubleshooting) cobre os problemas comuns, e as [Perguntas frequentes](./docs/faq), as dúvidas comuns.
- **Relate um bug** nas [issues do GitHub](https://github.com/nefocp/nefo/issues). Informe a sua versão (`nefo version`), o seu sistema operacional, os passos que levam ao problema, o que você esperava e o que aconteceu. Se o próprio painel falhar, acrescente o final do log dele: `journalctl -u nefo -n 50 --no-pager`.
- **Deixe de fora os segredos** em tudo o que você publicar: o caminho secreto do painel, senhas, chaves de API e nomes de domínio privados.
- **Relate um problema de segurança em particular,** nunca em uma issue pública. Veja a [política de segurança](./security-policy#report-a-vulnerability).

## Contribuir {#contribute}

- **Relate bugs** como descrito acima. Relatos de servidores reais são o que o Nefo mais precisa antes da 1.0.
- **Traduza o painel.** Copie `web/src/locales/en.json` para um arquivo com o código do seu idioma, traduza os valores (não as chaves) e registre o idioma em `web/src/i18n.js`.
- **Corrija issues e envie pull requests.** Leia antes o [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md). O Nefo roda como root nos servidores dos seus usuários, por isso cada alteração é revisada com a segurança em mente, e as regras de segurança para o código descritas lá não são opcionais. Para uma alteração maior, abra uma issue primeiro para combinar a abordagem.
- **Assine o CLA.** Quem contribui assina o [Contributor Licence Agreement](https://github.com/nefocp/nefo/blob/main/CLA.md) (acordo de licença do colaborador) uma única vez, ao abrir o primeiro pull request. Você mantém os direitos autorais das suas contribuições.
