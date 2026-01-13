---
sidebar_position: 4
---

# Precedência de Ambiente

Quando a mesma variável é definida em múltiplos lugares, o Invowk segue uma ordem de precedência específica. Fontes com maior precedência sobrescrevem as de menor.

## Ordem de Precedência

Da maior para a menor prioridade:

| Prioridade | Fonte | Exemplo |
|------------|-------|---------|
| 1 | Vars da CLI | `--env-var KEY=value` |
| 2 | Arquivos env da CLI | `--env-file .env.local` |
| 3 | Vars de implementação | `implementations[].env.vars` |
| 4 | Arquivos de implementação | `implementations[].env.files` |
| 5 | Vars de comando | `command.env.vars` |
| 6 | Arquivos de comando | `command.env.files` |
| 7 | Vars raiz | `root.env.vars` |
| 8 | Arquivos raiz | `root.env.files` |
| 9 | Ambiente do sistema | Ambiente do host |
| 10 | Vars de plataforma | `platforms[].env` |

## Hierarquia Visual

```
CLI (maior prioridade)
├── --env-var KEY=value
└── --env-file .env.local
    │
Nível de Implementação
├── env.vars
└── env.files
    │
Nível de Comando
├── env.vars
└── env.files
    │
Nível Raiz
├── env.vars
└── env.files
    │
Ambiente do Sistema (menor do invkfile)
│
Env específico de plataforma
```

## Passo a Passo de Exemplo

Dado este invkfile:

```cue
group: "myproject"

// Nível raiz
env: {
    files: [".env"]
    vars: {
        API_URL: "http://root.example.com"
        LOG_LEVEL: "info"
    }
}

commands: [
    {
        name: "build"
        // Nível de comando
        env: {
            files: [".env.build"]
            vars: {
                API_URL: "http://command.example.com"
                BUILD_MODE: "development"
            }
        }
        implementations: [{
            script: "echo $API_URL $LOG_LEVEL $BUILD_MODE $NODE_ENV"
            target: {runtimes: [{name: "native"}]}
            // Nível de implementação
            env: {
                vars: {
                    BUILD_MODE: "production"
                    NODE_ENV: "production"
                }
            }
        }]
    }
]
```

E estes arquivos:

```bash
# .env
API_URL=http://envfile.example.com
DATABASE_URL=postgres://localhost/db

# .env.build
BUILD_MODE=release
CACHE_DIR=./cache
```

### Ordem de Resolução

1. **Começar com ambiente do sistema** (ex.: `PATH`, `HOME`)

2. **Carregar arquivos raiz** (`.env`):
   - `API_URL=http://envfile.example.com`
   - `DATABASE_URL=postgres://localhost/db`

3. **Aplicar vars raiz** (sobrescrever arquivos):
   - `API_URL=http://root.example.com` ← sobrescreve `.env`
   - `LOG_LEVEL=info`

4. **Carregar arquivos de comando** (`.env.build`):
   - `BUILD_MODE=release`
   - `CACHE_DIR=./cache`

5. **Aplicar vars de comando** (sobrescrever arquivos):
   - `API_URL=http://command.example.com` ← sobrescreve raiz
   - `BUILD_MODE=development` ← sobrescreve `.env.build`

6. **Aplicar vars de implementação**:
   - `BUILD_MODE=production` ← sobrescreve comando
   - `NODE_ENV=production`

### Resultado Final

```bash
API_URL=http://command.example.com    # De vars de comando
LOG_LEVEL=info                         # De vars raiz
BUILD_MODE=production                  # De vars de implementação
NODE_ENV=production                    # De vars de implementação
DATABASE_URL=postgres://localhost/db   # De arquivo .env
CACHE_DIR=./cache                      # De arquivo .env.build
```

### Com Sobrescrita da CLI

```bash
invowk cmd myproject build --env-var API_URL=http://cli.example.com
```

Agora `API_URL=http://cli.example.com` porque CLI tem a maior prioridade.

## Arquivos vs Vars no Mesmo Nível

Dentro do mesmo nível, `vars` sobrescreve `files`:

```cue
env: {
    files: [".env"]  // API_URL=from-file
    vars: {
        API_URL: "from-vars"  // Este ganha
    }
}
```

## Múltiplos Arquivos no Mesmo Nível

Arquivos são carregados em ordem; arquivos posteriores sobrescrevem anteriores:

```cue
env: {
    files: [
        ".env",           // API_URL=base
        ".env.local",     // API_URL=local (ganha)
    ]
}
```

## Variáveis Específicas de Plataforma

Variáveis de plataforma são aplicadas depois de tudo:

```cue
implementations: [{
    script: "echo $CONFIG_PATH"
    target: {
        runtimes: [{name: "native"}]
        platforms: [
            {name: "linux", env: {CONFIG_PATH: "/etc/app"}}
            {name: "macos", env: {CONFIG_PATH: "/usr/local/etc/app"}}
        ]
    }
    env: {
        vars: {
            OTHER_VAR: "value"
            // CONFIG_PATH não definido aqui
        }
    }
}]
```

`env` de plataforma só é aplicado se a plataforma corresponder e a variável ainda não estiver definida.

## Melhores Práticas

### Use Níveis Apropriados

```cue
// Raiz: compartilhado entre todos os comandos
env: {
    vars: {
        PROJECT_NAME: "myapp"
        VERSION: "1.0.0"
    }
}

// Comando: específico para este comando
{
    name: "build"
    env: {
        vars: {
            BUILD_TARGET: "production"
        }
    }
}

// Implementação: específico para este runtime
implementations: [{
    target: {runtimes: [{name: "container", image: "node:20"}]}
    env: {
        vars: {
            NODE_OPTIONS: "--max-old-space-size=4096"
        }
    }
}]
```

### Padrão de Sobrescrita

Config base em arquivos, sobrescritas em vars:

```cue
env: {
    files: [".env"]              // Padrões
    vars: {
        OVERRIDE_THIS: "value"   // Sobrescrita específica
    }
}
```

### Desenvolvimento Local

Use arquivos locais opcionais para sobrescritas de desenvolvedor:

```cue
env: {
    files: [
        ".env",          // Padrões commitados
        ".env.local?",   // Não commitado, sobrescritas pessoais
    ]
}
```

### CLI para Sobrescritas Temporárias

```bash
# Teste rápido com config diferente
invowk cmd myproject build -E DEBUG=true -E LOG_LEVEL=debug
```

## Debugando Precedência

Para ver valores finais, adicione saída de debug:

```cue
{
    name: "debug-env"
    implementations: [{
        script: """
            echo "API_URL=$API_URL"
            echo "LOG_LEVEL=$LOG_LEVEL"
            echo "BUILD_MODE=$BUILD_MODE"
            env | sort
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Próximos Passos

- [Env Files](./env-files) - Carregar de arquivos .env
- [Env Vars](./env-vars) - Definir variáveis diretamente
