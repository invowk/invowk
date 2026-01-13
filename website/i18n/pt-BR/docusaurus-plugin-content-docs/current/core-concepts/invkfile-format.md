---
sidebar_position: 1
---

# Formato do Invkfile

Invkfiles são escritos em [CUE](https://cuelang.org/), uma linguagem de configuração poderosa que é como JSON com superpoderes. Se você nunca usou CUE antes, não se preocupe - é intuitivo e você vai aprender rapidamente.

## Por que CUE?

Escolhemos CUE em vez de YAML ou JSON porque:

- **Validação integrada** - CUE detecta erros antes de você executar qualquer coisa
- **Sem pesadelos de indentação** - Diferente do YAML, um espaço mal posicionado não vai quebrar tudo
- **Comentários!** - Sim, você pode realmente escrever comentários
- **Tipagem segura** - O schema do Invowk garante que seu invkfile está correto
- **Templates** - Reduza repetição com o poderoso sistema de templates do CUE

## Sintaxe Básica

CUE parece com JSON, mas mais legível:

```cue
// Isso é um comentário
group: "myproject"
version: "1.0"

// Listas usam colchetes
commands: [
    {
        name: "hello"
        description: "A greeting command"
    }
]

// Strings multilinha usam aspas triplas
script: """
    echo "Line 1"
    echo "Line 2"
    """
```

Diferenças principais do JSON:
- Vírgulas não são necessárias após campos (embora sejam permitidas)
- Vírgulas finais são aceitas
- Comentários com `//`
- Strings multilinha com `"""`

## Visão Geral do Schema

Todo invkfile segue esta estrutura:

```cue
// Nível raiz
group: string           // Obrigatório: prefixo do namespace
version?: string        // Opcional: versão do invkfile (ex: "1.0")
description?: string    // Opcional: sobre o que é este arquivo
default_shell?: string  // Opcional: sobrescreve o shell padrão
workdir?: string        // Opcional: diretório de trabalho padrão
env?: #EnvConfig        // Opcional: configuração global de ambiente
depends_on?: #DependsOn // Opcional: dependências globais

// Obrigatório: pelo menos um comando
commands: [...#Command]
```

O sufixo `?` significa que um campo é opcional.

## O Campo Group

O `group` é o campo mais importante - ele define o namespace de todos os seus comandos:

```cue
group: "myproject"

commands: [
    {name: "build"},
    {name: "test"},
]
```

Esses comandos se tornam:
- `myproject build`
- `myproject test`

### Regras de Nomenclatura de Group

- Deve começar com uma letra
- Pode conter letras e números
- Pontos (`.`) criam namespaces aninhados
- Sem hífens, underscores ou espaços

**Válidos:**
- `myproject`
- `my.project`
- `com.company.tools`
- `frontend`

**Inválidos:**
- `my-project` (hífen)
- `my_project` (underscore)
- `.project` (começa com ponto)
- `123project` (começa com número)

### Nomenclatura RDNS

Para packs ou invkfiles compartilhados, recomendamos a nomenclatura RDNS (Reverse Domain Name System):

```cue
group: "com.company.devtools"
group: "io.github.username.project"
```

Isso evita conflitos ao combinar múltiplos invkfiles.

## Estrutura de Comandos

Cada comando tem esta estrutura:

```cue
{
    name: string                 // Obrigatório: nome do comando
    description?: string         // Opcional: texto de ajuda
    implementations: [...]       // Obrigatório: como executar o comando
    flags?: [...]                // Opcional: flags do comando
    args?: [...]                 // Opcional: argumentos posicionais
    env?: #EnvConfig             // Opcional: configuração de ambiente
    workdir?: string             // Opcional: diretório de trabalho
    depends_on?: #DependsOn      // Opcional: dependências
}
```

### Implementações

Um comando pode ter múltiplas implementações para diferentes plataformas/runtimes:

```cue
{
    name: "build"
    implementations: [
        // Implementação Unix
        {
            script: "make build"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            }
        },
        // Implementação Windows
        {
            script: "msbuild /p:Configuration=Release"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "windows"}]
            }
        }
    ]
}
```

O Invowk automaticamente escolhe a implementação correta para sua plataforma.

## Scripts

Scripts podem ser inline ou referenciar arquivos externos:

### Scripts Inline

```cue
// Linha única
script: "echo 'Hello!'"

// Multilinha
script: """
    #!/bin/bash
    set -e
    echo "Building..."
    go build ./...
    """
```

### Arquivos de Script Externos

```cue
// Relativo à localização do invkfile
script: "./scripts/build.sh"

// Apenas o nome do arquivo (extensões reconhecidas)
script: "deploy.sh"
```

Extensões reconhecidas: `.sh`, `.bash`, `.ps1`, `.bat`, `.cmd`, `.py`, `.rb`, `.pl`, `.zsh`, `.fish`

## Exemplo Completo

Aqui está um invkfile com todos os recursos:

```cue
group: "myapp"
version: "1.0"
description: "Build and deployment commands for MyApp"

// Ambiente global
env: {
    vars: {
        APP_NAME: "myapp"
        LOG_LEVEL: "info"
    }
}

// Dependências globais
depends_on: {
    tools: [{alternatives: ["sh", "bash"]}]
}

commands: [
    {
        name: "build"
        description: "Build the application"
        implementations: [
            {
                script: """
                    echo "Building $APP_NAME..."
                    go build -o bin/$APP_NAME ./...
                    """
                target: {
                    runtimes: [{name: "native"}]
                }
            }
        ]
        depends_on: {
            tools: [{alternatives: ["go"]}]
            filepaths: [{alternatives: ["go.mod"]}]
        }
    },
    {
        name: "deploy"
        description: "Deploy to production"
        implementations: [
            {
                script: "./scripts/deploy.sh"
                target: {
                    runtimes: [{name: "native"}]
                    platforms: [{name: "linux"}, {name: "macos"}]
                }
            }
        ]
        depends_on: {
            tools: [{alternatives: ["docker", "podman"]}]
            commands: [{alternatives: ["myapp build"]}]
        }
        flags: [
            {name: "env", description: "Target environment", required: true},
            {name: "dry-run", description: "Simulate deployment", type: "bool", default_value: "false"}
        ]
    }
]
```

## Dicas e Truques do CUE

### Reduzir Repetição

Use o sistema de templates do CUE para evitar repetição:

```cue
// Defina um template
_nativeUnix: {
    target: {
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }
}

commands: [
    {
        name: "build"
        implementations: [
            _nativeUnix & {script: "make build"}
        ]
    },
    {
        name: "test"
        implementations: [
            _nativeUnix & {script: "make test"}
        ]
    }
]
```

### Validação

Execute seu invkfile através do validador CUE:

```bash
cue vet invkfile.cue path/to/invkfile_schema.cue -d '#Invkfile'
```

Ou apenas tente listar comandos - o Invowk valida automaticamente:

```bash
invowk cmd --list
```

## Próximos Passos

- [Comandos e Grupos](./commands-and-groups) - Convenções de nomenclatura e hierarquias
- [Implementações](./implementations) - Plataformas, runtimes e scripts
