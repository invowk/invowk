---
sidebar_position: 3
---

# Referência de Schema de Configuração

:::warning Alpha — Opções de Configuração Podem Mudar
O schema de configuração ainda está sendo refinado. Opções **podem ser adicionadas, renomeadas ou removidas** entre releases enquanto finalizamos o conjunto de funcionalidades.
:::

Referência completa para o schema do arquivo de configuração do Invowk.

## Visão Geral

O arquivo de configuração usa formato [CUE](https://cuelang.org/) e está localizado em:

| Plataforma | Localização |
|------------|-------------|
| Linux      | `~/.config/invowk/config.cue` |
| macOS      | `~/Library/Application Support/invowk/config.cue` |
| Windows    | `%APPDATA%\invowk\config.cue` |

## Definição do Schema

```cue
// Estrutura de configuração raiz
#Config: {
    container_engine?: "podman" | "docker"
    search_paths?:     [...string]
    default_runtime?:  "native" | "virtual" | "container"
    virtual_shell?:    #VirtualShellConfig
    ui?:               #UIConfig
}

// Configuração do virtual shell
#VirtualShellConfig: {
    enable_uroot_utils?: bool
}

// Configuração de UI
#UIConfig: {
    color_scheme?: "auto" | "dark" | "light"
    verbose?:      bool
}
```

## Config

O objeto de configuração raiz.

```cue
#Config: {
    container_engine?: "podman" | "docker"
    search_paths?:     [...string]
    default_runtime?:  "native" | "virtual" | "container"
    virtual_shell?:    #VirtualShellConfig
    ui?:               #UIConfig
}
```

### container_engine

**Tipo:** `"podman" | "docker"`  
**Obrigatório:** Não  
**Padrão:** Auto-detectado

Especifica qual container runtime usar para execução de comandos baseada em container.

```cue
container_engine: "podman"
```

**Ordem de auto-detecção:**
1. Podman (preferido)
2. Docker (fallback)

### search_paths

**Tipo:** `[...string]`  
**Obrigatório:** Não  
**Padrão:** `["~/.invowk/cmds"]`

Diretórios adicionais para buscar invkfiles e packs.

```cue
search_paths: [
    "~/.invowk/cmds",
    "~/projects/shared-commands",
    "/opt/company/invowk-commands",
]
```

**Resolução de caminho:**
- Caminhos começando com `~` são expandidos para o diretório home do usuário
- Caminhos relativos são resolvidos do diretório de trabalho atual
- Caminhos inexistentes são ignorados silenciosamente

**Prioridade de busca (maior para menor):**
1. Diretório atual
2. Caminhos em `search_paths` (em ordem)
3. Padrão `~/.invowk/cmds`

### default_runtime

**Tipo:** `"native" | "virtual" | "container"`  
**Obrigatório:** Não  
**Padrão:** `"native"`

Define o modo de runtime padrão global para comandos que não especificam um runtime preferido.

```cue
default_runtime: "virtual"
```

| Valor | Descrição |
|-------|-----------|
| `"native"` | Executar usando o shell nativo do sistema |
| `"virtual"` | Executar usando o interpretador de shell embutido do Invowk |
| `"container"` | Executar dentro de um container |

### virtual_shell

**Tipo:** `#VirtualShellConfig`  
**Obrigatório:** Não  
**Padrão:** `{}`

Configuração para o virtual shell runtime.

```cue
virtual_shell: {
    enable_uroot_utils: true
}
```

### ui

**Tipo:** `#UIConfig`  
**Obrigatório:** Não  
**Padrão:** `{}`

Configuração de interface do usuário.

```cue
ui: {
    color_scheme: "dark"
    verbose: false
}
```

---

## VirtualShellConfig

Configuração para o virtual shell runtime (mvdan/sh).

```cue
#VirtualShellConfig: {
    enable_uroot_utils?: bool
}
```

### enable_uroot_utils

**Tipo:** `bool`  
**Obrigatório:** Não  
**Padrão:** `false`

Habilita utilitários u-root no ambiente virtual shell. Quando habilitado, fornece comandos adicionais além dos builtins básicos de shell.

```cue
virtual_shell: {
    enable_uroot_utils: true
}
```

**Utilitários disponíveis quando habilitado:**
- Operações de arquivo: `ls`, `cat`, `cp`, `mv`, `rm`, `mkdir`, `chmod`
- Processamento de texto: `grep`, `sed`, `awk`, `sort`, `uniq`
- E muitos mais utilitários core

---

## UIConfig

Configuração de interface do usuário.

```cue
#UIConfig: {
    color_scheme?: "auto" | "dark" | "light"
    verbose?:      bool
}
```

### color_scheme

**Tipo:** `"auto" | "dark" | "light"`  
**Obrigatório:** Não  
**Padrão:** `"auto"`

Define o esquema de cores para saída de terminal.

```cue
ui: {
    color_scheme: "auto"
}
```

| Valor | Descrição |
|-------|-----------|
| `"auto"` | Detectar do terminal (respeita `COLORTERM`, `TERM`, etc.) |
| `"dark"` | Cores otimizadas para terminais escuros |
| `"light"` | Cores otimizadas para terminais claros |

### verbose

**Tipo:** `bool`  
**Obrigatório:** Não  
**Padrão:** `false`

Habilita saída verbose por padrão para todos os comandos.

```cue
ui: {
    verbose: true
}
```

Equivalente a sempre passar `--verbose` na linha de comando.

---

## Exemplo Completo

Um arquivo de configuração totalmente documentado:

```cue
// Arquivo de Configuração Invowk
// ==============================
// Localização: ~/.config/invowk/config.cue

// Container Engine
// ----------------
// Qual container runtime usar: "podman" ou "docker"
// Se não especificado, Invowk auto-detecta (prefere Podman)
container_engine: "podman"

// Caminhos de Busca
// -----------------
// Diretórios adicionais para buscar invkfiles e packs
// Buscados em ordem após o diretório atual
search_paths: [
    // Comandos pessoais
    "~/.invowk/cmds",
    
    // Comandos compartilhados da equipe
    "~/work/shared-commands",
    
    // Comandos da organização
    "/opt/company/invowk-commands",
]

// Runtime Padrão
// --------------
// O runtime a usar quando um comando não especifica um
// Opções: "native", "virtual", "container"
default_runtime: "native"

// Configuração do Virtual Shell
// -----------------------------
// Configurações para o virtual shell runtime (mvdan/sh)
virtual_shell: {
    // Habilitar utilitários u-root para mais comandos de shell
    // Fornece ls, cat, grep, etc. no ambiente virtual
    enable_uroot_utils: true
}

// Configuração de UI
// ------------------
// Configurações de interface do usuário
ui: {
    // Esquema de cores: "auto", "dark" ou "light"
    // "auto" detecta das configurações do terminal
    color_scheme: "auto"
    
    // Habilitar saída verbose por padrão
    // Igual a sempre passar --verbose
    verbose: false
}
```

---

## Configuração Mínima

Se você está satisfeito com os padrões, uma configuração mínima pode ser:

```cue
// Apenas sobrescreva o que você precisa
container_engine: "docker"
```

Ou até mesmo um arquivo vazio (todos os padrões):

```cue
// Configuração vazia - usar todos os padrões
```

---

## Validação

Você pode validar seu arquivo de configuração usando CUE:

```bash
cue vet ~/.config/invowk/config.cue
```

Ou verificá-lo com Invowk:

```bash
invowk config show
```

Se houver algum erro, Invowk irá reportá-los ao carregar a configuração.
