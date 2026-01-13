---
sidebar_position: 1
---

# Instalação

:::warning Software em Alfa
O Invowk está atualmente em **estágio alfa**. Embora busquemos estabilidade, espere mudanças incompatíveis entre versões enquanto estabilizamos o formato do invkfile, estrutura de packs e conjunto de funcionalidades. Recomendamos fixar uma versão específica para uso em produção e acompanhar os [lançamentos no GitHub](https://github.com/invowk/invowk/releases) para guias de migração.
:::

Bem-vindo ao Invowk! Vamos configurar tudo para você executar comandos rapidamente.

## Requisitos

Antes de instalar o Invowk, certifique-se de ter:

- **Go 1.25+** (apenas se compilar a partir do código-fonte)
- **Linux, macOS ou Windows** - O Invowk funciona em todos os três!

Para recursos de runtime em container, você também precisará de:
- **Docker** ou **Podman** instalado e em execução

## Métodos de Instalação

### A Partir do Código-Fonte (Recomendado por enquanto)

Se você tem o Go instalado, compilar a partir do código-fonte é simples:

```bash
git clone https://github.com/invowk/invowk
cd invowk
go build -o invowk .
```

Depois mova o binário para o seu PATH:

```bash
# Linux/macOS
sudo mv invowk /usr/local/bin/

# Ou adicione ao seu bin local
mv invowk ~/.local/bin/
```

### Usando Make (com mais opções)

O projeto inclui um Makefile com várias opções de compilação:

```bash
# Compilação padrão (binário stripped, tamanho menor)
make build

# Compilação de desenvolvimento (com símbolos de debug)
make build-dev

# Compilação comprimida (requer UPX)
make build-upx

# Instalar em $GOPATH/bin
make install
```

### Verificar Instalação

Uma vez instalado, verifique se tudo funciona:

```bash
invowk --version
```

Você deverá ver as informações de versão. Se receber um erro "command not found", certifique-se de que o binário está no seu PATH.

## Autocompletar do Shell

O Invowk suporta autocompletar com tab para bash, zsh, fish e PowerShell. Isso torna a digitação de comandos muito mais rápida!

### Bash

```bash
# Adicione ao ~/.bashrc
eval "$(invowk completion bash)"

# Ou instale para todo o sistema
invowk completion bash > /etc/bash_completion.d/invowk
```

### Zsh

```bash
# Adicione ao ~/.zshrc
eval "$(invowk completion zsh)"

# Ou instale no fpath
invowk completion zsh > "${fpath[1]}/_invowk"
```

### Fish

```bash
invowk completion fish > ~/.config/fish/completions/invowk.fish
```

### PowerShell

```powershell
invowk completion powershell | Out-String | Invoke-Expression

# Ou adicione ao $PROFILE para persistência
invowk completion powershell >> $PROFILE
```

## Próximos Passos

Agora que você tem o Invowk instalado, vá para o guia de [Início Rápido](./quickstart) para executar seu primeiro comando!
