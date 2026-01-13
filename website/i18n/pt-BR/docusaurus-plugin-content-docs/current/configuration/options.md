---
sidebar_position: 2
---

# Opções de Configuração

Esta página documenta todas as opções de configuração disponíveis para o Invowk.

## Schema de Configuração

O arquivo de configuração usa o formato CUE e segue este schema:

```cue
#Config: {
    container_engine?: "podman" | "docker"
    search_paths?: [...string]
    default_runtime?: "native" | "virtual" | "container"
    virtual_shell?: #VirtualShellConfig
    ui?: #UIConfig
}
```

## Referência de Opções

### container_engine

**Tipo:** `"podman" | "docker"`  
**Padrão:** Auto-detectado (prefere Podman se disponível)

Especifica qual container runtime usar para execução de comandos baseada em container.

```cue
container_engine: "podman"
```

Invowk irá auto-detectar container engines disponíveis se não especificado:
1. Primeiro verifica por Podman
2. Volta para Docker se Podman não estiver disponível
3. Retorna um erro se nenhum for encontrado (apenas quando container runtime é necessário)

### search_paths

**Tipo:** `[...string]`  
**Padrão:** `["~/.invowk/cmds"]`

Diretórios adicionais para buscar invkfiles. Caminhos são buscados em ordem após o diretório atual.

```cue
search_paths: [
    "~/.invowk/cmds",
    "~/projects/shared-commands",
    "/opt/company/invowk-commands",
]
```

**Ordem de Busca:**
1. Diretório atual (sempre buscado primeiro, maior prioridade)
2. Cada caminho em `search_paths` em ordem
3. `~/.invowk/cmds` (padrão, sempre incluído)

Comandos de caminhos anteriores sobrescrevem comandos com o mesmo nome de caminhos posteriores.

### default_runtime

**Tipo:** `"native" | "virtual" | "container"`  
**Padrão:** `"native"`

Define o modo de runtime padrão global para comandos que não especificam um runtime.

```cue
default_runtime: "virtual"
```

**Opções de Runtime:**
- `"native"` - Executar usando o shell nativo do sistema (bash, zsh, PowerShell, etc.)
- `"virtual"` - Executar usando o interpretador de shell embutido do Invowk (mvdan/sh)
- `"container"` - Executar dentro de um container (requer Docker ou Podman)

:::note
Comandos podem sobrescrever este padrão especificando seu próprio runtime no campo `implementations`.
:::

### virtual_shell

**Tipo:** `#VirtualShellConfig`  
**Padrão:** `{}`

Configura o comportamento do virtual shell runtime.

```cue
virtual_shell: {
    enable_uroot_utils: true
}
```

#### virtual_shell.enable_uroot_utils

**Tipo:** `bool`  
**Padrão:** `false`

Habilita utilitários u-root no virtual shell. Quando habilitado, comandos adicionais como `ls`, `cat`, `grep` e outros ficam disponíveis no ambiente virtual shell.

```cue
virtual_shell: {
    enable_uroot_utils: true
}
```

Isso é útil quando você quer que o virtual shell tenha mais capacidades além dos builtins básicos de shell, enquanto ainda evita execução de shell nativo.

### ui

**Tipo:** `#UIConfig`  
**Padrão:** `{}`

Configura as preferências de interface do usuário.

```cue
ui: {
    color_scheme: "dark"
    verbose: false
}
```

#### ui.color_scheme

**Tipo:** `"auto" | "dark" | "light"`  
**Padrão:** `"auto"`

Define o esquema de cores para a saída do Invowk.

```cue
ui: {
    color_scheme: "auto"
}
```

**Opções:**
- `"auto"` - Detectar das configurações do terminal (respeita `COLORTERM`, `TERM`, etc.)
- `"dark"` - Usar cores otimizadas para terminais escuros
- `"light"` - Usar cores otimizadas para terminais claros

#### ui.verbose

**Tipo:** `bool`  
**Padrão:** `false`

Habilita saída verbose por padrão. Quando habilitado, Invowk imprime informações adicionais sobre descoberta de comandos, validação de dependências e execução.

```cue
ui: {
    verbose: true
}
```

Isso é equivalente a sempre passar `--verbose` na linha de comando.

## Exemplo Completo

Aqui está um arquivo de configuração completo com todas as opções:

```cue
// Arquivo de Configuração Invowk
// Localizado em: ~/.config/invowk/config.cue

// Usar Podman como container engine
container_engine: "podman"

// Buscar invkfiles nesses diretórios
search_paths: [
    "~/.invowk/cmds",          // Comandos pessoais
    "~/work/shared-commands",   // Comandos compartilhados da equipe
]

// Usar virtual shell por padrão para portabilidade
default_runtime: "virtual"

// Configurações do virtual shell
virtual_shell: {
    // Habilitar utilitários u-root para mais comandos de shell
    enable_uroot_utils: true
}

// Preferências de UI
ui: {
    // Auto-detectar esquema de cores do terminal
    color_scheme: "auto"
    
    // Não ser verbose por padrão
    verbose: false
}
```

## Overrides de Variáveis de Ambiente

Algumas opções de configuração podem ser sobrescritas com variáveis de ambiente:

| Variável de Ambiente | Sobrescreve |
|---------------------|-------------|
| `INVOWK_CONFIG` | Caminho do arquivo de configuração |
| `INVOWK_VERBOSE` | `ui.verbose` (defina como `1` ou `true`) |
| `INVOWK_CONTAINER_ENGINE` | `container_engine` |

```bash
# Exemplo: Usar Docker ao invés do Podman configurado
INVOWK_CONTAINER_ENGINE=docker invowk cmd build

# Exemplo: Habilitar saída verbose para esta execução
INVOWK_VERBOSE=1 invowk cmd test
```

## Overrides de Linha de Comando

Todas as opções de configuração podem ser sobrescritas via flags de linha de comando:

```bash
# Sobrescrever arquivo de configuração
invowk --config /path/to/config.cue cmd list

# Sobrescrever configuração verbose
invowk --verbose cmd build

# Sobrescrever runtime para um comando
invowk cmd build --runtime container
```

Veja [Referência CLI](/docs/reference/cli) para todas as flags disponíveis.
