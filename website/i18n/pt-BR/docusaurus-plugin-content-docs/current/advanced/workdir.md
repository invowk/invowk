---
sidebar_position: 2
---

# Diretório de Trabalho

Controle onde seus comandos executam com a configuração `workdir`. Isso é especialmente útil para monorepos e projetos com estruturas de diretório complexas.

## Comportamento Padrão

Por padrão, comandos rodam no diretório atual (onde você invocou o `invowk`).

## Definindo o Diretório de Trabalho

### Nível de Comando

```cue
{
    name: "build frontend"
    workdir: "./frontend"  // Rodar no subdiretório frontend
    implementations: [{
        script: "npm run build"
        target: {runtimes: [{name: "native"}]}
    }]
}
```

### Nível de Implementação

```cue
{
    name: "build"
    implementations: [
        {
            script: "npm run build"
            target: {runtimes: [{name: "native"}]}
            workdir: "./web"  // Esta implementação roda em ./web
        },
        {
            script: "go build ./..."
            target: {runtimes: [{name: "native"}]}
            workdir: "./api"  // Esta implementação roda em ./api
        }
    ]
}
```

### Nível Raiz

```cue
group: "myproject"
workdir: "./src"  // Todos os comandos usam ./src por padrão

commands: [
    {
        name: "build"
        // Herda workdir: ./src
        implementations: [...]
    },
    {
        name: "test"
        workdir: "./tests"  // Sobrescrever para ./tests
        implementations: [...]
    }
]
```

## Tipos de Caminho

### Caminhos Relativos

Relativo à localização do invkfile:

```cue
workdir: "./frontend"
workdir: "../shared"
workdir: "src/app"
```

### Caminhos Absolutos

Caminhos completos do sistema:

```cue
workdir: "/opt/myapp"
workdir: "/home/user/projects/myapp"
```

### Variáveis de Ambiente

Expanda variáveis em caminhos:

```cue
workdir: "${HOME}/projects/myapp"
workdir: "${PROJECT_ROOT}/src"
```

## Precedência

Implementação sobrescreve comando, que sobrescreve raiz:

```cue
group: "myproject"
workdir: "./root"  // Padrão: ./root

commands: [
    {
        name: "build"
        workdir: "./command"  // Sobrescrita: ./command
        implementations: [
            {
                script: "make"
                workdir: "./implementation"  // Final: ./implementation
                target: {runtimes: [{name: "native"}]}
            }
        ]
    }
]
```

## Padrão Monorepo

Perfeito para monorepos com múltiplos pacotes:

```cue
group: "monorepo"

commands: [
    {
        name: "web build"
        workdir: "./packages/web"
        implementations: [{
            script: "npm run build"
            target: {runtimes: [{name: "native"}]}
        }]
    },
    {
        name: "api build"
        workdir: "./packages/api"
        implementations: [{
            script: "go build ./..."
            target: {runtimes: [{name: "native"}]}
        }]
    },
    {
        name: "mobile build"
        workdir: "./packages/mobile"
        implementations: [{
            script: "flutter build"
            target: {runtimes: [{name: "native"}]}
        }]
    }
]
```

## Diretório de Trabalho em Container

Para containers, o diretório atual é montado em `/workspace`:

```cue
{
    name: "build"
    implementations: [{
        script: """
            pwd  # /workspace
            ls   # Mostra seus arquivos do projeto
            """
        target: {
            runtimes: [{name: "container", image: "alpine"}]
        }
    }]
}
```

Com `workdir`, esse subdiretório se torna o diretório de trabalho do container:

```cue
{
    name: "build frontend"
    workdir: "./frontend"
    implementations: [{
        script: """
            pwd  # /workspace/frontend
            npm run build
            """
        target: {
            runtimes: [{name: "container", image: "node:20"}]
        }
    }]
}
```

## Caminhos Multiplataforma

Use barras para frente para compatibilidade multiplataforma:

```cue
// Bom - funciona em qualquer lugar
workdir: "./src/app"

// Evite - específico do Windows
workdir: ".\\src\\app"
```

O Invowk converte automaticamente para separadores de caminho nativos em tempo de execução.

## Exemplos do Mundo Real

### Divisão Frontend/Backend

```cue
commands: [
    {
        name: "start frontend"
        workdir: "./frontend"
        implementations: [{
            script: "npm run dev"
            target: {runtimes: [{name: "native"}]}
        }]
    },
    {
        name: "start backend"
        workdir: "./backend"
        implementations: [{
            script: "go run ./cmd/server"
            target: {runtimes: [{name: "native"}]}
        }]
    }
]
```

### Organização de Testes

```cue
commands: [
    {
        name: "test unit"
        workdir: "./tests/unit"
        implementations: [{
            script: "pytest"
            target: {runtimes: [{name: "native"}]}
        }]
    },
    {
        name: "test integration"
        workdir: "./tests/integration"
        implementations: [{
            script: "pytest"
            target: {runtimes: [{name: "native"}]}
        }]
    },
    {
        name: "test e2e"
        workdir: "./tests/e2e"
        implementations: [{
            script: "cypress run"
            target: {runtimes: [{name: "native"}]}
        }]
    }
]
```

### Build em Subdiretório

```cue
{
    name: "build"
    workdir: "./src"
    implementations: [{
        script: """
            # Agora em ./src
            go build -o ../bin/app ./...
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Melhores Práticas

1. **Use caminhos relativos**: Mais portável entre máquinas
2. **Barras para frente**: Compatível com múltiplas plataformas
3. **Nível raiz para padrões**: Sobrescreva onde necessário
4. **Mantenha caminhos curtos**: Mais fácil de entender

## Próximos Passos

- [Interpreters](./interpreters) - Usar interpretadores não-shell
- [Platform-Specific](./platform-specific) - Implementações por plataforma
