---
sidebar_position: 1
---

# Visão Geral de Ambiente

O Invowk fornece gerenciamento poderoso de variáveis de ambiente para seus comandos. Defina variáveis, carregue de arquivos e controle a precedência em múltiplos níveis.

## Exemplo Rápido

```cue
{
    name: "build"
    env: {
        // Carregar de arquivos .env
        files: [".env", ".env.local?"]  // ? significa opcional
        
        // Definir variáveis diretamente
        vars: {
            NODE_ENV: "production"
            BUILD_DATE: "$(date +%Y-%m-%d)"
        }
    }
    implementations: [{
        script: """
            echo "Building for $NODE_ENV"
            echo "Date: $BUILD_DATE"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Fontes de Ambiente

Variáveis vêm de múltiplas fontes, em ordem de precedência (mais alta primeiro):

1. **Flags da CLI** - `--env-var KEY=value`
2. **Arquivos env da CLI** - `--env-file .env.custom`
3. **Vars de implementação** - `env.vars` no nível de implementação
4. **Arquivos de implementação** - `env.files` no nível de implementação
5. **Vars de comando** - `env.vars` no nível de comando
6. **Arquivos de comando** - `env.files` no nível de comando
7. **Vars raiz** - `env.vars` no nível raiz
8. **Arquivos raiz** - `env.files` no nível raiz
9. **Ambiente do sistema** - Variáveis de ambiente do host

Fontes posteriores não sobrescrevem as anteriores.

## Níveis de Escopo

### Nível Raiz

Aplica-se a todos os comandos no invkfile:

```cue
group: "myproject"

env: {
    vars: {
        PROJECT_NAME: "myproject"
    }
}

commands: [...]  // Todos os comandos recebem PROJECT_NAME
```

### Nível de Comando

Aplica-se a um comando específico:

```cue
{
    name: "build"
    env: {
        vars: {
            BUILD_MODE: "release"
        }
    }
    implementations: [...]
}
```

### Nível de Implementação

Aplica-se a uma implementação específica:

```cue
{
    name: "build"
    implementations: [
        {
            script: "npm run build"
            target: {runtimes: [{name: "native"}]}
            env: {
                vars: {
                    NODE_ENV: "production"
                }
            }
        }
    ]
}
```

### Nível de Plataforma

Defina variáveis por plataforma:

```cue
implementations: [{
    script: "echo $CONFIG_PATH"
    target: {
        runtimes: [{name: "native"}]
        platforms: [
            {
                name: "linux"
                env: {CONFIG_PATH: "/etc/myapp/config"}
            },
            {
                name: "macos"
                env: {CONFIG_PATH: "/usr/local/etc/myapp/config"}
            }
        ]
    }
}]
```

## Arquivos Env

Carregue variáveis de arquivos `.env`:

```cue
env: {
    files: [
        ".env",           // Obrigatório - falha se faltando
        ".env.local?",    // Opcional - sufixo com ?
        ".env.${ENV}?",   // Interpolação - usa variável ENV
    ]
}
```

Arquivos são carregados em ordem; arquivos posteriores sobrescrevem os anteriores.

Veja [Env Files](./env-files) para detalhes.

## Variáveis de Ambiente

Defina variáveis diretamente:

```cue
env: {
    vars: {
        API_URL: "https://api.example.com"
        DEBUG: "true"
        VERSION: "1.0.0"
    }
}
```

Veja [Env Vars](./env-vars) para detalhes.

## Sobrescrita via CLI

Sobrescreva em tempo de execução:

```bash
# Definir uma única variável
invowk cmd myproject build --env-var NODE_ENV=development

# Definir múltiplas variáveis
invowk cmd myproject build -E NODE_ENV=dev -E DEBUG=true

# Carregar de um arquivo
invowk cmd myproject build --env-file .env.local

# Combinar
invowk cmd myproject build --env-file .env.local -E OVERRIDE=value
```

## Variáveis Embutidas

O Invowk fornece estas variáveis automaticamente:

| Variável | Descrição |
|----------|-----------|
| `INVOWK_CMD_NAME` | Nome completo do comando (ex.: `myproject build`) |
| `INVOWK_CMD_GROUP` | Grupo do comando (ex.: `myproject`) |
| `INVOWK_RUNTIME` | Runtime atual (native, virtual, container) |
| `INVOWK_WORKDIR` | Diretório de trabalho |

Mais variáveis de flag e argumento:
- `INVOWK_FLAG_*` - Valores de flag
- `INVOWK_ARG_*` - Valores de argumento

## Ambiente de Container

Para runtime container, o ambiente é passado para dentro do container:

```cue
{
    name: "build"
    env: {
        vars: {
            BUILD_ENV: "container"
        }
    }
    implementations: [{
        script: "echo $BUILD_ENV"  // Disponível dentro do container
        target: {
            runtimes: [{name: "container", image: "alpine"}]
        }
    }]
}
```

## Comandos Aninhados

Quando um comando invoca outro comando, algumas variáveis são isoladas:

**Isoladas (NÃO herdadas):**
- `INVOWK_ARG_*`
- `INVOWK_FLAG_*`

**Herdadas (comportamento UNIX normal):**
- Variáveis de `env.vars`
- Variáveis de nível de plataforma
- Ambiente do sistema

Isso impede que argumentos do comando pai vazem para comandos filhos.

## Próximos Passos

- [Env Files](./env-files) - Carregar de arquivos .env
- [Env Vars](./env-vars) - Definir variáveis diretamente
- [Precedence](./precedence) - Entender ordem de sobrescrita
