---
sidebar_position: 3
---

# Implementações

Todo comando precisa de pelo menos uma **implementação** - o código real que executa quando você invoca o comando. Implementações definem *o que* executa, *onde* executa (plataforma), e *como* executa (runtime).

## Estrutura Básica

Uma implementação tem três partes principais:

```cue
{
    name: "build"
    implementations: [
        {
            // 1. O script a executar
            script: "go build ./..."
            
            // 2. Restrições de target (runtime + plataforma)
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            }
        }
    ]
}
```

## Scripts

O campo `script` contém os comandos a serem executados. Pode ser inline ou referenciar um arquivo externo.

### Scripts Inline

Scripts de uma linha são simples:

```cue
script: "echo 'Hello, World!'"
```

Scripts multilinha usam aspas triplas:

```cue
script: """
    #!/bin/bash
    set -e
    echo "Building..."
    go build -o bin/app ./...
    echo "Done!"
    """
```

### Arquivos de Script Externos

Referencie um arquivo de script em vez de código inline:

```cue
// Relativo à localização do invkfile
script: "./scripts/build.sh"

// Apenas o nome do arquivo (se tiver extensão reconhecida)
script: "deploy.sh"
```

Extensões reconhecidas: `.sh`, `.bash`, `.ps1`, `.bat`, `.cmd`, `.py`, `.rb`, `.pl`, `.zsh`, `.fish`

### Quando Usar Scripts Externos

- **Inline**: Comandos rápidos e simples; mantém tudo em um arquivo
- **Externo**: Scripts complexos; reutilizáveis entre comandos; mais fácil de editar com syntax highlighting

## Restrições de Target

O campo `target` define onde e como a implementação executa.

### Runtimes

Toda implementação deve especificar pelo menos um runtime:

```cue
target: {
    runtimes: [
        {name: "native"},      // Shell do sistema
        {name: "virtual"},     // Shell POSIX integrado
        {name: "container", image: "alpine:latest"}  // Container
    ]
}
```

O **primeiro runtime** é o padrão. Usuários podem sobrescrever com `--runtime`:

```bash
# Usa o runtime padrão (primeiro da lista)
invowk cmd myproject build

# Sobrescreve para usar runtime container
invowk cmd myproject build --runtime container
```

Veja [Modos de Runtime](../runtime-modes/overview) para detalhes sobre cada runtime.

### Plataformas

Opcionalmente restrinja quais sistemas operacionais a implementação suporta:

```cue
target: {
    runtimes: [{name: "native"}]
    platforms: [
        {name: "linux"},
        {name: "macos"},
        {name: "windows"}
    ]
}
```

Se `platforms` for omitido, a implementação funciona em todas as plataformas.

Plataformas disponíveis: `linux`, `macos`, `windows`

## Múltiplas Implementações

Comandos podem ter múltiplas implementações para diferentes cenários:

### Implementações Específicas por Plataforma

```cue
{
    name: "clean"
    implementations: [
        // Implementação Unix
        {
            script: "rm -rf build/"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            }
        },
        // Implementação Windows
        {
            script: "rmdir /s /q build"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "windows"}]
            }
        }
    ]
}
```

O Invowk automaticamente seleciona a implementação correta para a plataforma atual.

### Implementações Específicas por Runtime

```cue
{
    name: "build"
    implementations: [
        // Build nativo rápido
        {
            script: "go build ./..."
            target: {
                runtimes: [{name: "native"}]
            }
        },
        // Build reproduzível em container
        {
            script: "go build -o /workspace/bin/app ./..."
            target: {
                runtimes: [{name: "container", image: "golang:1.21"}]
            }
        }
    ]
}
```

### Plataforma + Runtime Combinados

```cue
{
    name: "build"
    implementations: [
        // Linux/macOS com múltiplas opções de runtime
        {
            script: "make build"
            target: {
                runtimes: [
                    {name: "native"},
                    {name: "container", image: "ubuntu:22.04"}
                ]
                platforms: [{name: "linux"}, {name: "macos"}]
            }
        },
        // Windows apenas nativo
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

## Ambiente Específico por Plataforma

Plataformas podem definir suas próprias variáveis de ambiente:

```cue
{
    name: "deploy"
    implementations: [
        {
            script: "echo \"Deploying to $PLATFORM with config at $CONFIG_PATH\""
            target: {
                runtimes: [{name: "native"}]
                platforms: [
                    {
                        name: "linux"
                        env: {
                            PLATFORM: "Linux"
                            CONFIG_PATH: "/etc/app/config.yaml"
                        }
                    },
                    {
                        name: "macos"
                        env: {
                            PLATFORM: "macOS"
                            CONFIG_PATH: "/usr/local/etc/app/config.yaml"
                        }
                    }
                ]
            }
        }
    ]
}
```

## Configurações no Nível da Implementação

Implementações podem ter seu próprio ambiente, diretório de trabalho e dependências:

```cue
{
    name: "build"
    implementations: [
        {
            script: "npm run build"
            target: {
                runtimes: [{name: "native"}]
            }
            
            // Env específico da implementação
            env: {
                vars: {
                    NODE_ENV: "production"
                }
            }
            
            // Workdir específico da implementação
            workdir: "./frontend"
            
            // Dependências específicas da implementação
            depends_on: {
                tools: [{alternatives: ["node", "npm"]}]
                filepaths: [{alternatives: ["package.json"]}]
            }
        }
    ]
}
```

Estas sobrescrevem configurações do nível do comando quando esta implementação é selecionada.

## Seleção de Implementação

Quando você executa um comando, o Invowk seleciona uma implementação baseado em:

1. **Plataforma atual** - Filtra para implementações que suportam seu SO
2. **Runtime solicitado** - Se `--runtime` especificado, usa esse; caso contrário usa o padrão
3. **Primeiro match vence** - Usa a primeira implementação que corresponde a ambos os critérios

### Exemplos de Seleção

Dado este comando:

```cue
{
    name: "build"
    implementations: [
        {
            script: "make build"
            target: {
                runtimes: [{name: "native"}, {name: "virtual"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            }
        },
        {
            script: "msbuild"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "windows"}]
            }
        }
    ]
}
```

| Plataforma | Comando | Selecionado |
|------------|---------|-------------|
| Linux | `invowk cmd myproject build` | Primeira impl, runtime native |
| Linux | `invowk cmd myproject build --runtime virtual` | Primeira impl, runtime virtual |
| Windows | `invowk cmd myproject build` | Segunda impl, runtime native |
| Windows | `invowk cmd myproject build --runtime virtual` | Erro: nenhuma impl correspondente |

## Listagem de Comandos

A saída de `invowk cmd list` mostra runtimes e plataformas disponíveis:

```
Available Commands
  (* = default runtime)

From current directory:
  myproject build - Build the project [native*, virtual] (linux, macos)
  myproject clean - Clean artifacts [native*] (linux, macos, windows)
  myproject docker-build - Container build [container*] (linux, macos, windows)
```

- `[native*, virtual]` - Suporta runtimes native (padrão) e virtual
- `(linux, macos)` - Disponível apenas no Linux e macOS

## Usando Templates CUE

Reduza repetição com templates CUE:

```cue
// Defina templates reutilizáveis
_unixNative: {
    target: {
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }
}

_allPlatforms: {
    target: {
        runtimes: [{name: "native"}]
    }
}

commands: [
    {
        name: "build"
        implementations: [
            _unixNative & {script: "make build"}
        ]
    },
    {
        name: "test"
        implementations: [
            _unixNative & {script: "make test"}
        ]
    },
    {
        name: "version"
        implementations: [
            _allPlatforms & {script: "cat VERSION"}
        ]
    }
]
```

## Boas Práticas

1. **Comece simples** - Uma implementação é frequentemente suficiente
2. **Adicione plataformas conforme necessário** - Não especifique plataformas a menos que precise de comportamento específico
3. **Runtime padrão primeiro** - Coloque o runtime mais comum primeiro na lista
4. **Use templates** - Reduza repetição com o sistema de templates do CUE
5. **Mantenha scripts focados** - Uma tarefa por comando; encadeie com dependências

## Próximos Passos

- [Modos de Runtime](../runtime-modes/overview) - Mergulho profundo nos runtimes native, virtual e container
- [Dependências](../dependencies/overview) - Declare o que suas implementações precisam
