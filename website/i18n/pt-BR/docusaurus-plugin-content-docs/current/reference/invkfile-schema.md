---
sidebar_position: 2
---

# Referência de Schema do Invkfile

:::warning Alpha — Schema Pode Mudar
O schema do invkfile ainda está evoluindo. Campos, tipos e regras de validação **podem mudar entre releases** enquanto estabilizamos o formato. Sempre verifique o [changelog](https://github.com/invowk/invowk/releases) ao fazer upgrade.
:::

Referência completa para o schema do invkfile. Invkfiles usam formato [CUE](https://cuelang.org/) para definir comandos.

## Estrutura Raiz

Todo invkfile deve ter um `group` e pelo menos um comando:

```cue
#Invkfile: {
    group:          string    // Obrigatório - prefixo para todos os nomes de comando
    version?:       string    // Opcional - versão do schema (ex.: "1.0")
    description?:   string    // Opcional - descrever o propósito deste invkfile
    default_shell?: string    // Opcional - sobrescrever shell padrão
    workdir?:       string    // Opcional - diretório de trabalho padrão
    env?:           #EnvConfig      // Opcional - environment global
    depends_on?:    #DependsOn      // Opcional - dependências globais
    commands:       [...#Command]   // Obrigatório - pelo menos um comando
}
```

### group

**Tipo:** `string`  
**Obrigatório:** Sim

Um prefixo obrigatório para todos os nomes de comando deste invkfile. Deve começar com uma letra e pode conter segmentos separados por ponto.

```cue
// Nomes de grupo válidos
group: "build"
group: "my.project"
group: "com.example.tools"

// Inválidos
group: "123abc"     // Não pode começar com número
group: ".build"     // Não pode começar com ponto
group: "build."     // Não pode terminar com ponto
group: "my..tools"  // Não pode ter pontos consecutivos
```

### version

**Tipo:** `string` (padrão: `^[0-9]+\.[0-9]+$`)  
**Obrigatório:** Não  
**Padrão:** Nenhum

A versão do schema do invkfile. Versão atual é `"1.0"`.

```cue
version: "1.0"
```

### description

**Tipo:** `string`  
**Obrigatório:** Não

Um resumo do propósito deste invkfile. Mostrado ao listar comandos.

```cue
description: "Build and deployment commands for the web application"
```

### default_shell

**Tipo:** `string`  
**Obrigatório:** Não  
**Padrão:** Padrão do sistema

Sobrescrever o shell padrão para execução em runtime native.

```cue
default_shell: "/bin/bash"
default_shell: "pwsh"
```

### workdir

**Tipo:** `string`  
**Obrigatório:** Não  
**Padrão:** Diretório do invkfile

Diretório de trabalho padrão para todos os comandos. Pode ser absoluto ou relativo à localização do invkfile.

```cue
workdir: "./src"
workdir: "/opt/app"
```

### env

**Tipo:** `#EnvConfig`  
**Obrigatório:** Não

Configuração de environment global aplicada a todos os comandos. Veja [EnvConfig](#envconfig).

### depends_on

**Tipo:** `#DependsOn`  
**Obrigatório:** Não

Dependências globais que se aplicam a todos os comandos. Veja [DependsOn](#dependson).

### commands

**Tipo:** `[...#Command]`  
**Obrigatório:** Sim (pelo menos um)

Lista de comandos definidos neste invkfile. Veja [Command](#command).

---

## Command

Define um comando executável:

```cue
#Command: {
    name:            string               // Obrigatório
    description?:    string               // Opcional
    implementations: [...#Implementation] // Obrigatório - pelo menos um
    env?:            #EnvConfig           // Opcional
    workdir?:        string               // Opcional
    depends_on?:     #DependsOn           // Opcional
    flags?:          [...#Flag]           // Opcional
    args?:           [...#Argument]       // Opcional
}
```

### name

**Tipo:** `string` (padrão: `^[a-zA-Z][a-zA-Z0-9_ -]*$`)  
**Obrigatório:** Sim

O identificador do comando. Deve começar com uma letra.

```cue
name: "build"
name: "test unit"     // Espaços permitidos para comportamento tipo subcomando
name: "deploy-prod"
```

### description

**Tipo:** `string`  
**Obrigatório:** Não

Texto de ajuda para o comando.

```cue
description: "Build the application for production"
```

### implementations

**Tipo:** `[...#Implementation]`  
**Obrigatório:** Sim (pelo menos um)

As implementations executáveis. Veja [Implementation](#implementation).

### flags

**Tipo:** `[...#Flag]`  
**Obrigatório:** Não

Flags de linha de comando para este comando. Veja [Flag](#flag).

:::warning Flags Reservadas
`env-file` (curta `e`) e `env-var` (curta `E`) são flags de sistema reservadas e não podem ser usadas.
:::

### args

**Tipo:** `[...#Argument]`  
**Obrigatório:** Não

Argumentos posicionais para este comando. Veja [Argument](#argument).

---

## Implementation

Define como um comando é executado:

```cue
#Implementation: {
    script:      string       // Obrigatório - script inline ou caminho de arquivo
    target:      #Target      // Obrigatório - restrições de runtime e plataforma
    env?:        #EnvConfig   // Opcional
    workdir?:    string       // Opcional
    depends_on?: #DependsOn   // Opcional
}
```

### script

**Tipo:** `string` (não vazio)  
**Obrigatório:** Sim

Os comandos de shell a executar OU um caminho para um arquivo de script.

```cue
// Script inline
script: "echo 'Hello, World!'"

// Script multilinha
script: """
    echo "Building..."
    go build -o app .
    echo "Done!"
    """

// Referência de arquivo de script
script: "./scripts/build.sh"
script: "deploy.py"
```

**Extensões reconhecidas:** `.sh`, `.bash`, `.ps1`, `.bat`, `.cmd`, `.py`, `.rb`, `.pl`, `.zsh`, `.fish`

### target

**Tipo:** `#Target`  
**Obrigatório:** Sim

Define restrições de runtime e plataforma. Veja [Target](#target-1).

---

## Target

Especifica onde uma implementation pode executar:

```cue
#Target: {
    runtimes:   [...#RuntimeConfig]   // Obrigatório - pelo menos um
    platforms?: [...#PlatformConfig]  // Opcional
}
```

### runtimes

**Tipo:** `[...#RuntimeConfig]`  
**Obrigatório:** Sim (pelo menos um)

Os runtimes que podem executar esta implementation. O primeiro runtime é o padrão.

```cue
// Apenas native
runtimes: [{name: "native"}]

// Múltiplos runtimes
runtimes: [
    {name: "native"},
    {name: "virtual"},
]

// Container com opções
runtimes: [{
    name: "container"
    image: "golang:1.22"
    volumes: ["./:/app"]
}]
```

### platforms

**Tipo:** `[...#PlatformConfig]`  
**Obrigatório:** Não  
**Padrão:** Todas as plataformas

Restringir esta implementation a sistemas operacionais específicos.

```cue
// Apenas Linux e macOS
platforms: [
    {name: "linux"},
    {name: "macos"},
]
```

---

## RuntimeConfig

Configuração para um runtime específico:

```cue
#RuntimeConfig: {
    name: "native" | "virtual" | "container"
    
    // Para native e container:
    interpreter?: string
    
    // Apenas para container:
    enable_host_ssh?: bool
    containerfile?:   string
    image?:           string
    volumes?:         [...string]
    ports?:           [...string]
}
```

### name

**Tipo:** `"native" | "virtual" | "container"`  
**Obrigatório:** Sim

O tipo de runtime.

### interpreter

**Tipo:** `string`  
**Disponível para:** `native`, `container`  
**Padrão:** `"auto"` (detectar do shebang)

Especifica como executar o script.

```cue
// Auto-detectar do shebang
interpreter: "auto"

// Interpreter específico
interpreter: "python3"
interpreter: "node"
interpreter: "/usr/bin/ruby"

// Com argumentos
interpreter: "python3 -u"
interpreter: "/usr/bin/env perl -w"
```

:::note
Virtual runtime usa mvdan/sh que não pode executar interpreters não-shell.
:::

### enable_host_ssh

**Tipo:** `bool`  
**Disponível para:** `container`  
**Padrão:** `false`

Habilitar acesso SSH do container de volta ao host. Quando habilitado, Invowk inicia um servidor SSH e fornece credenciais via variáveis de ambiente:

- `INVOWK_SSH_HOST`
- `INVOWK_SSH_PORT`
- `INVOWK_SSH_USER`
- `INVOWK_SSH_TOKEN`

```cue
runtimes: [{
    name: "container"
    image: "alpine:latest"
    enable_host_ssh: true
}]
```

### containerfile / image

**Tipo:** `string`  
**Disponível para:** `container`

Especificar a fonte do container. Estes são **mutuamente exclusivos**.

```cue
// Usar uma imagem pré-construída
image: "alpine:latest"
image: "golang:1.22"

// Construir de um Containerfile
containerfile: "./Containerfile"
containerfile: "./docker/Dockerfile.build"
```

### volumes

**Tipo:** `[...string]`  
**Disponível para:** `container`

Montagens de volume no formato `host:container[:options]`.

```cue
volumes: [
    "./src:/app/src",
    "/tmp:/tmp:ro",
    "${HOME}/.cache:/cache",
]
```

### ports

**Tipo:** `[...string]`  
**Disponível para:** `container`

Mapeamentos de porta no formato `host:container`.

```cue
ports: [
    "8080:80",
    "3000:3000",
]
```

---

## PlatformConfig

```cue
#PlatformConfig: {
    name: "linux" | "macos" | "windows"
}
```

---

## EnvConfig

Configuração de environment:

```cue
#EnvConfig: {
    files?: [...string]         // Arquivos dotenv para carregar
    vars?:  [string]: string    // Variáveis de environment
}
```

### files

Arquivos dotenv para carregar. Arquivos são carregados em ordem; arquivos posteriores sobrescrevem anteriores.

```cue
env: {
    files: [
        ".env",
        ".env.local",
        ".env.${ENVIRONMENT}?",  // '?' significa opcional
    ]
}
```

### vars

Variáveis de environment como pares chave-valor.

```cue
env: {
    vars: {
        NODE_ENV: "production"
        DEBUG: "false"
    }
}
```

---

## DependsOn

Especificação de dependência:

```cue
#DependsOn: {
    tools?:         [...#ToolDependency]
    commands?:      [...#CommandDependency]
    filepaths?:     [...#FilepathDependency]
    capabilities?:  [...#CapabilityDependency]
    custom_checks?: [...#CustomCheckDependency]
    env_vars?:      [...#EnvVarDependency]
}
```

### ToolDependency

```cue
#ToolDependency: {
    alternatives: [...string]  // Pelo menos um - nomes de ferramentas
}
```

```cue
depends_on: {
    tools: [
        {alternatives: ["go"]},
        {alternatives: ["podman", "docker"]},  // Qualquer um funciona
    ]
}
```

### CommandDependency

```cue
#CommandDependency: {
    alternatives: [...string]  // Nomes de comandos
}
```

### FilepathDependency

```cue
#FilepathDependency: {
    alternatives: [...string]  // Caminhos de arquivo/diretório
    readable?:    bool
    writable?:    bool
    executable?:  bool
}
```

### CapabilityDependency

```cue
#CapabilityDependency: {
    alternatives: [...("local-area-network" | "internet")]
}
```

### EnvVarDependency

```cue
#EnvVarDependency: {
    alternatives: [...#EnvVarCheck]
}

#EnvVarCheck: {
    name:        string    // Nome da variável de environment
    validation?: string    // Padrão regex
}
```

### CustomCheckDependency

```cue
#CustomCheckDependency: #CustomCheck | #CustomCheckAlternatives

#CustomCheck: {
    name:             string  // Identificador do check
    check_script:     string  // Script a executar
    expected_code?:   int     // Código de saída esperado (padrão: 0)
    expected_output?: string  // Regex para corresponder saída
}

#CustomCheckAlternatives: {
    alternatives: [...#CustomCheck]
}
```

---

## Flag

Definição de flag de linha de comando:

```cue
#Flag: {
    name:          string    // Nome compatível com POSIX
    description:   string    // Texto de ajuda
    default_value?: string   // Valor padrão
    type?:         "string" | "bool" | "int" | "float"
    required?:     bool
    short?:        string    // Alias de caractere único
    validation?:   string    // Padrão regex
}
```

```cue
flags: [
    {
        name: "output"
        short: "o"
        description: "Output file path"
        default_value: "./build"
    },
    {
        name: "verbose"
        short: "v"
        description: "Enable verbose output"
        type: "bool"
    },
]
```

---

## Argument

Definição de argumento posicional:

```cue
#Argument: {
    name:          string    // Nome compatível com POSIX
    description:   string    // Texto de ajuda
    required?:     bool      // Deve ser fornecido
    default_value?: string   // Padrão se não fornecido
    type?:         "string" | "int" | "float"
    validation?:   string    // Padrão regex
    variadic?:     bool      // Aceita múltiplos valores (apenas último arg)
}
```

```cue
args: [
    {
        name: "target"
        description: "Build target"
        required: true
    },
    {
        name: "files"
        description: "Files to process"
        variadic: true
    },
]
```

**Variáveis de Environment para Argumentos:**
- `INVOWK_ARG_<NAME>` - O valor do argumento
- Para variadic: `INVOWK_ARG_<NAME>_COUNT`, `INVOWK_ARG_<NAME>_1`, `INVOWK_ARG_<NAME>_2`, etc.

---

## Exemplo Completo

```cue
group: "myapp"
version: "1.0"
description: "Build and deployment commands"

env: {
    files: [".env"]
    vars: {
        APP_NAME: "myapp"
    }
}

commands: [
    {
        name: "build"
        description: "Build the application"
        
        flags: [
            {
                name: "release"
                short: "r"
                description: "Build for release"
                type: "bool"
            },
        ]
        
        implementations: [
            {
                script: """
                    if [ "$INVOWK_FLAG_RELEASE" = "true" ]; then
                        go build -ldflags="-s -w" -o app .
                    else
                        go build -o app .
                    fi
                    """
                target: {
                    runtimes: [{name: "native"}]
                    platforms: [{name: "linux"}, {name: "macos"}]
                }
            },
            {
                script: """
                    $flags = if ($env:INVOWK_FLAG_RELEASE -eq "true") { "-ldflags=-s -w" } else { "" }
                    go build $flags -o app.exe .
                    """
                target: {
                    runtimes: [{name: "native", interpreter: "pwsh"}]
                    platforms: [{name: "windows"}]
                }
            },
        ]
        
        depends_on: {
            tools: [{alternatives: ["go"]}]
        }
    },
]
```
