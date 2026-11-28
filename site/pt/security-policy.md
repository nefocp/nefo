---
title: Política de segurança
description: Como relatar uma vulnerabilidade no Nefo, como as versões são assinadas e como verificar uma por conta própria, e o estado atual da segurança.
---

# Política de segurança

O Nefo roda como root e gerencia um servidor inteiro, por isso todo relato de vulnerabilidade é tratado como urgente. Esta página diz como relatar uma, como as versões são assinadas e como você pode verificar uma, e em que pé o Nefo está hoje. Ela segue o [SECURITY.md](https://github.com/nefocp/nefo/blob/main/SECURITY.md) do repositório.

## Relatar uma vulnerabilidade {#report-a-vulnerability}

Não abra uma issue, discussão ou pull request público. Relate em particular no GitHub:

1. Abra o [repositório do Nefo](https://github.com/nefocp/nefo) e vá até a aba **Security** (Segurança).
2. Escolha **Report a vulnerability** (Relatar uma vulnerabilidade), ou use o [link direto](https://github.com/nefocp/nefo/security/advisories/new). Só os mantenedores veem o seu relato.

Informe:

- a versão afetada (`nefo version`) e o sistema operacional;
- o que um atacante consegue fazer, e de que acesso ele precisa antes: nenhum, uma conta do painel, uma conta de shell…;
- os passos para reproduzir, ou uma prova de conceito;
- se você pretende publicar as suas descobertas, e quando.

### O que acontece depois {#what-happens-next}

| Etapa | Meta |
|---|---|
| O projeto confirma que recebeu o seu relato | em até 3 dias úteis |
| O projeto diz se confirma o problema e qual é a gravidade | em até 7 dias |
| O projeto lança uma correção para um problema crítico ou de alta gravidade | em até 14 dias após a confirmação |
| Divulgação pública | depois que a correção sair, ou no máximo 90 dias após o seu relato, em acordo com você |

Toda vulnerabilidade confirmada ganha um GitHub Security Advisory com um CVE. Você recebe o crédito, a menos que prefira ficar anônimo.

### Escopo {#scope}

No escopo: o programa `nefo`, a sua API e a sua interface, o instalador e a configuração que ele cria no servidor.

Fora do escopo:

- o aviso do navegador sobre o certificado autoassinado que o painel cria na primeira inicialização (isso é esperado);
- ataques que precisam de acesso root ao servidor, ou de acesso físico a ele;
- negação de serviço só pelo volume de tráfego;
- vulnerabilidades em software de terceiros que o painel instala, como Nginx, PHP ou MariaDB. Relate essas aos próprios projetos, mas avise-nos se o painel as configurar de forma insegura.

### Porto seguro (safe harbor) {#safe-harbour}

O projeto não vai tomar medidas legais contra você por uma pesquisa de segurança que siga esta política: você testa só em servidores que são seus ou que você tem permissão para testar, evita violações de privacidade e destruição de dados, e dá ao projeto um tempo razoável para corrigir o problema antes de divulgá-lo.

## Versões com suporte {#supported-versions}

Até a versão 1.0, só a versão mais recente recebe correções de segurança. Atualize pelo painel (**Configurações** → **Painel** → **Procurar atualizações**) ou com `nefo update` no servidor. Veja [Atualização](./docs/update).

## Como as versões são assinadas {#how-releases-are-signed}

O GitHub Actions gera cada versão a partir de uma tag de versão no repositório. Uma versão contém:

- `nefo-linux-amd64` e `nefo-linux-arm64`, os programas;
- `install.sh`, o instalador;
- `VERSION`;
- `checksums.txt`, os checksums SHA-256 dos dois programas e do instalador;
- `checksums.txt.sig`, uma assinatura Ed25519 de `checksums.txt`;
- `appstore-catalog.json` e `appstore-catalog.json.sig`, o catálogo da loja de aplicativos com a sua própria assinatura.

A chave pública do projeto vem embutida em todo programa Nefo e copiada no `install.sh`. Ela também está no repositório, como [`internal/update/release.pub`](https://github.com/nefocp/nefo/blob/main/internal/update/release.pub):

```
-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEA2c6nGagX7yp8Y8YftuOqW2WVprKe6h0hv+3s9iB+KJA=
-----END PUBLIC KEY-----
```

Três coisas conferem assinaturas antes de usar um download:

- **O instalador** confere a assinatura de `checksums.txt` e o checksum do programa. Se qualquer um dos dois estiver errado, ele para: "invalid signature: checksums.txt was not signed by the project. Do not install this file." (assinatura inválida: checksums.txt não foi assinado pelo projeto. Não instale este arquivo.)
- **`nefo update`**, e **Instalar atualização** no painel, fazem o mesmo com a chave embutida no programa que você já roda, e depois rodam o novo programa uma vez antes de substituir `/usr/local/bin/nefo`.
- **A loja de aplicativos** busca o catálogo uma vez por dia e o ignora se a assinatura não for válida.

### O que a assinatura prova e o que não prova {#what-the-signature-does-and-doesn-t-prove}

A assinatura mostra que os arquivos vêm do build de release do projeto e não foram alterados depois, no GitHub ou no caminho até o seu servidor.

- **As atualizações** são conferidas com a chave que está dentro do Nefo que você já tem, então uma versão alterada no GitHub não consegue ser instalada.
- **Uma primeira instalação** é diferente: o instalador que você baixa traz a chave com que ele confere. Se alguém conseguisse alterar o próprio `install.sh`, poderia alterar essa chave também. Para descartar isso, confira o `install.sh` você mesmo antes de rodá-lo, como mostrado abaixo.
- A chave de assinatura é guardada como um segredo do repositório no GitHub, e as versões são assinadas lá pelo GitHub Actions. Alguém que tomasse a conta do projeto no GitHub poderia assinar uma versão.

## Verificar uma versão por conta própria {#check-a-release-yourself}

Você precisa de `curl`, `sha256sum` e OpenSSL 3 (no Ubuntu 22.04 e posteriores, e no Debian 12 e posteriores). Este exemplo verifica a versão mais recente em um servidor amd64, como root; defina `V` com outra tag de versão para verificar aquela versão.

1. Baixe os checksums, a assinatura deles e o instalador:

   ```sh
   V=$(curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/VERSION)
   cd /tmp
   for f in checksums.txt checksums.txt.sig install.sh; do
     curl -fsSLO "https://github.com/nefocp/nefo/releases/download/$V/$f"
   done
   ```

2. Salve a chave pública desta página:

   ```sh
   cat > nefo-release.pub <<'EOF'
   -----BEGIN PUBLIC KEY-----
   MCowBQYDK2VwAyEA2c6nGagX7yp8Y8YftuOqW2WVprKe6h0hv+3s9iB+KJA=
   -----END PUBLIC KEY-----
   EOF
   ```

3. Confira a assinatura e depois o checksum do instalador:

   ```sh
   openssl pkeyutl -verify -pubin -inkey nefo-release.pub -rawin \
     -in checksums.txt -sigfile checksums.txt.sig
   grep " install.sh$" checksums.txt | sha256sum -c -
   ```

   O primeiro comando precisa mostrar `Signature Verified Successfully`, e o segundo, `install.sh: OK`. Se qualquer um falhar, não rode o instalador, e relate o problema como descrito acima.

4. Rode o instalador que você conferiu. Ele baixa o programa para a sua CPU e o confere com a mesma chave:

   ```sh
   NEFO_VERSION=$V bash install.sh
   ```

Para conferir um programa que você mesmo baixou, como `nefo-linux-amd64`, rode `grep " nefo-linux-amd64$" checksums.txt | sha256sum -c -` depois da etapa 3.

## Como o Nefo protege o servidor {#how-nefo-protects-the-server}

Um resumo curto; [Segurança](./docs/security) tem os detalhes:

- O painel só responde sob um caminho secreto, em uma porta aleatória. Qualquer outro endereço retorna `404`.
- Ele usa HTTPS desde a primeira inicialização, com TLS 1.2 ou mais recente.
- O login aceita autenticação de dois fatores. 5 tentativas malsucedidas em 15 minutos bloqueiam um endereço por 15 minutos. As senhas são guardadas como hashes bcrypt.
- Os logins e as alterações são gravados em um log de atividades, mantido por 90 dias.
- Toda página tem uma Content Security Policy rígida, e toda alteração precisa de um cabeçalho que outros sites não conseguem enviar.
- As [regras para o código](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md#security-rules-for-code) do projeto proíbem montar comandos de shell a partir da entrada: um programa recebe cada valor como um argumento próprio, e a entrada é conferida com uma lista de valores permitidos antes de chegar a um caminho de arquivo, a um arquivo de configuração ou a um comando. Os comandos que você mesmo escreve, como tarefas cron e scripts de deploy, rodam em um shell do jeito que você os escreveu.
- Regras de firewall e configurações de SSH que deixariam você trancado do lado de fora são recusadas.
- Cada site roda com o seu próprio usuário Linux, e as contas SFTP ficam presas à pasta do site.

## Estado atual {#current-status}

O Nefo está em beta. A primeira versão pública foi a v0.20.0, em novembro de 2026; o [registro de alterações](./changelog) lista a atual. Ainda não foi feita nenhuma auditoria de segurança externa.

Experimente o Nefo em um servidor que você possa reconstruir antes de confiar nele para sites importantes, guarde cópias dos seus backups fora do servidor e mantenha o Nefo atualizado. Uma revisão de segurança feita fora do projeto é uma das etapas antes da 1.0 (veja o [roadmap](./roadmap)). Se você revisar o código e encontrar algo, relate como descrito acima.
