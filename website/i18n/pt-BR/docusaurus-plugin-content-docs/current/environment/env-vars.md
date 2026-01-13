---
sidebar_position: 3
---

# Variáveis de Ambiente

Defina variáveis de ambiente diretamente no seu invkfile. Estas ficam disponíveis para seus scripts durante a execução.

## Uso Básico

```cue
{
    name: "build"
    env: {
        vars: {
            NODE_ENV: "production"
            API_URL: "https://api.example.com"
            DEBUG: "false"
        }
    }
    implementations: [{
        script: """
            echo "Building for $NODE_ENV"
            echo "API: $API_URL"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Sintaxe de Variáveis

Variáveis são pares chave-valor de strings:

```cue
vars: {
    // Valores simples
    NAME: "value"
    
    // Números (ainda são strings no shell)
    PORT: "3000"
    TIMEOUT: "30"
    
    // Tipo booleano (strings "true"/"false")
    DEBUG: "true"
    VERBOSE: "false"
    
    // Caminhos
    CONFIG_PATH: "/etc/myapp/config.yaml"
    OUTPUT_DIR: "./dist"
    
    // URLs
    API_URL: "https://api.example.com"
}
```

Todos os valores são strings. O shell interpreta conforme necessário.

## Referenciando Outras Variáveis

Referencie variáveis de ambiente do sistema:

```cue
vars: {
    // Usar variável do sistema
    HOME_CONFIG: "${HOME}/.config/myapp"
    
    // Com valor padrão
    LOG_LEVEL: "${LOG_LEVEL:-info}"
    
    // Combinar variáveis
    FULL_PATH: "${HOME}/projects/${PROJECT_NAME}"
}
```

Nota: Referências são expandidas em tempo de execução, não no momento da definição.

## Níveis de Escopo

### Nível Raiz

Disponível para todos os comandos:

```cue
group: "myproject"

env: {
    vars: {
        PROJECT_NAME: "myproject"
        VERSION: "1.0.0"
    }
}

commands: [
    {
        name: "build"
        // Recebe PROJECT_NAME e VERSION
        implementations: [...]
    },
    {
        name: "deploy"
        // Também recebe PROJECT_NAME e VERSION
        implementations: [...]
    }
]
```

### Nível de Comando

Disponível para comando específico:

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

Disponível para implementação específica:

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
        },
        {
            script: "go build ./..."
            target: {runtimes: [{name: "native"}]}
            env: {
                vars: {
                    CGO_ENABLED: "0"
                }
            }
        }
    ]
}
```

### Nível de Plataforma

Variáveis por plataforma:

```cue
implementations: [{
    script: "echo $PLATFORM_CONFIG"
    target: {
        runtimes: [{name: "native"}]
        platforms: [
            {
                name: "linux"
                env: {
                    PLATFORM_CONFIG: "/etc/myapp"
                    PLATFORM_NAME: "Linux"
                }
            },
            {
                name: "macos"
                env: {
                    PLATFORM_CONFIG: "/usr/local/etc/myapp"
                    PLATFORM_NAME: "macOS"
                }
            },
            {
                name: "windows"
                env: {
                    PLATFORM_CONFIG: "%APPDATA%\\myapp"
                    PLATFORM_NAME: "Windows"
                }
            }
        ]
    }
}]
```

## Combinado com Arquivos

Variáveis sobrescrevem valores de arquivos env:

```cue
env: {
    files: [".env"]  // Carregado primeiro
    vars: {
        // Estas sobrescrevem valores do .env
        OVERRIDE: "from-invkfile"
    }
}
```

## Sobrescrita via CLI

Sobrescreva em tempo de execução:

```bash
# Única variável
invowk cmd myproject build --env-var NODE_ENV=development

# Forma curta
invowk cmd myproject build -E NODE_ENV=development

# Múltiplas variáveis
invowk cmd myproject build -E NODE_ENV=dev -E DEBUG=true -E PORT=8080
```

Variáveis da CLI têm a maior prioridade.

## Exemplos do Mundo Real

### Configuração de Build

```cue
{
    name: "build"
    env: {
        vars: {
            NODE_ENV: "production"
            BUILD_TARGET: "es2020"
            SOURCEMAP: "false"
        }
    }
    implementations: [{
        script: "npm run build"
        target: {runtimes: [{name: "native"}]}
    }]
}
```

### Configuração de API

```cue
{
    name: "start"
    env: {
        vars: {
            API_HOST: "0.0.0.0"
            API_PORT: "3000"
            API_PREFIX: "/api/v1"
            CORS_ORIGIN: "*"
        }
    }
    implementations: [{
        script: "go run ./cmd/server"
        target: {runtimes: [{name: "native"}]}
    }]
}
```

### Valores Dinâmicos

```cue
{
    name: "release"
    env: {
        vars: {
            // Versão baseada no git
            GIT_SHA: "$(git rev-parse --short HEAD)"
            GIT_BRANCH: "$(git branch --show-current)"
            
            // Timestamp
            BUILD_TIME: "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
            
            // Combinar valores
            BUILD_ID: "${GIT_BRANCH}-${GIT_SHA}"
        }
    }
    implementations: [{
        script: """
            echo "Building $BUILD_ID at $BUILD_TIME"
            go build -ldflags="-X main.version=$BUILD_ID" ./...
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

### Configuração de Banco de Dados

```cue
{
    name: "db migrate"
    env: {
        vars: {
            DB_HOST: "${DB_HOST:-localhost}"
            DB_PORT: "${DB_PORT:-5432}"
            DB_NAME: "${DB_NAME:-myapp}"
            DB_USER: "${DB_USER:-postgres}"
            // Construir URL das partes
            DATABASE_URL: "postgres://${DB_USER}@${DB_HOST}:${DB_PORT}/${DB_NAME}"
        }
    }
    implementations: [{
        script: "migrate -database $DATABASE_URL up"
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Ambiente de Container

Variáveis são passadas para containers:

```cue
{
    name: "build"
    env: {
        vars: {
            GOOS: "linux"
            GOARCH: "amd64"
            CGO_ENABLED: "0"
        }
    }
    implementations: [{
        script: "go build -o /workspace/bin/app ./..."
        target: {
            runtimes: [{name: "container", image: "golang:1.21"}]
        }
    }]
}
```

## Melhores Práticas

1. **Use padrões**: `${VAR:-default}` para config opcional
2. **Mantenha secrets fora**: Não hardcode secrets; use arquivos env ou secrets externos
3. **Documente variáveis**: Adicione comentários explicando cada variável
4. **Use nomenclatura consistente**: Convenção `UPPER_SNAKE_CASE`
5. **Escopo apropriado**: Raiz para compartilhado, comando para específico

## Próximos Passos

- [Env Files](./env-files) - Carregar de arquivos .env
- [Precedence](./precedence) - Entender ordem de sobrescrita
