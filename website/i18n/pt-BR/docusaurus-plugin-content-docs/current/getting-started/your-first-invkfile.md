---
sidebar_position: 3
---

# Seu Primeiro Invkfile

Agora que você executou seu primeiro comando, vamos criar algo mais prático. Criaremos um invkfile para um projeto típico com comandos de build, teste e deploy.

## Entendendo a Estrutura

Um invkfile tem uma estrutura simples:

```cue
group: "myproject"           // Obrigatório: namespace para seus comandos
version: "1.0"               // Opcional: versão deste invkfile
description: "My commands"   // Opcional: sobre o que é este arquivo

commands: [                  // Obrigatório: lista de comandos
    // ... seus comandos aqui
]
```

O `group` é obrigatório e se torna o prefixo para todos os seus comandos. Pense nele como um namespace que mantém seus comandos organizados e evita colisões com comandos de outros invkfiles.

## Um Exemplo do Mundo Real

Vamos criar um invkfile para um projeto Go:

```cue
group: "goproject"
version: "1.0"
description: "Commands for my Go project"

commands: [
    // Comando de build simples
    {
        name: "build"
        description: "Build the project"
        implementations: [
            {
                script: """
                    echo "Building..."
                    go build -o bin/app ./...
                    echo "Done! Binary at bin/app"
                    """
                target: {
                    runtimes: [{name: "native"}]
                }
            }
        ]
    },

    // Comando de teste com nome estilo subcomando
    {
        name: "test unit"
        description: "Run unit tests"
        implementations: [
            {
                script: "go test -v ./..."
                target: {
                    runtimes: [{name: "native"}, {name: "virtual"}]
                }
            }
        ]
    },

    // Teste com cobertura
    {
        name: "test coverage"
        description: "Run tests with coverage"
        implementations: [
            {
                script: """
                    go test -coverprofile=coverage.out ./...
                    go tool cover -html=coverage.out -o coverage.html
                    echo "Coverage report: coverage.html"
                    """
                target: {
                    runtimes: [{name: "native"}]
                }
            }
        ]
    },

    // Comando de limpeza
    {
        name: "clean"
        description: "Remove build artifacts"
        implementations: [
            {
                script: "rm -rf bin/ coverage.out coverage.html"
                target: {
                    runtimes: [{name: "native"}]
                    platforms: [{name: "linux"}, {name: "macos"}]
                }
            }
        ]
    }
]
```

Salve isso como `invkfile.cue` e tente:

```bash
invowk cmd --list
```

Você verá:

```
Available Commands
  (* = default runtime)

From current directory:
  goproject build - Build the project [native*]
  goproject test unit - Run unit tests [native*, virtual]
  goproject test coverage - Run tests with coverage [native*]
  goproject clean - Remove build artifacts [native*] (linux, macos)
```

## Nomes Estilo Subcomando

Note como `test unit` e `test coverage` criam uma hierarquia. Você os executa assim:

```bash
invowk cmd goproject test unit
invowk cmd goproject test coverage
```

Isso é apenas convenção de nomenclatura - espaços no nome criam uma sensação de subcomando. É ótimo para organizar comandos relacionados!

## Múltiplos Runtimes

O comando `test unit` permite tanto o runtime `native` quanto o `virtual`:

```cue
runtimes: [{name: "native"}, {name: "virtual"}]
```

O primeiro é o padrão. Você pode sobrescrevê-lo:

```bash
# Usar o padrão (native)
invowk cmd goproject test unit

# Usar explicitamente o runtime virtual
invowk cmd goproject test unit --runtime virtual
```

## Comandos Específicos por Plataforma

O comando `clean` só funciona no Linux e macOS (porque usa `rm -rf`):

```cue
platforms: [{name: "linux"}, {name: "macos"}]
```

Se você tentar executá-lo no Windows, o Invowk mostrará uma mensagem de erro útil explicando que o comando não está disponível na sua plataforma.

## Adicionando Dependências

Vamos tornar nosso comando de build mais inteligente verificando se o Go está instalado:

```cue
{
    name: "build"
    description: "Build the project"
    implementations: [
        {
            script: """
                echo "Building..."
                go build -o bin/app ./...
                echo "Done!"
                """
            target: {
                runtimes: [{name: "native"}]
            }
        }
    ]
    depends_on: {
        tools: [
            {alternatives: ["go"]}
        ]
        filepaths: [
            {alternatives: ["go.mod"], readable: true}
        ]
    }
}
```

Agora se você executar `invowk cmd goproject build` sem o Go instalado, você receberá:

```
✗ Dependencies not satisfied

Command 'build' has unmet dependencies:

Missing Tools:
  • go - not found in PATH

Install the missing tools and try again.
```

## Variáveis de Ambiente

Você pode definir variáveis de ambiente em diferentes níveis:

```cue
group: "goproject"

// Env no nível raiz aplica-se a TODOS os comandos
env: {
    vars: {
        GO111MODULE: "on"
    }
}

commands: [
    {
        name: "build"
        // Env no nível do comando aplica-se a este comando
        env: {
            vars: {
                CGO_ENABLED: "0"
            }
        }
        implementations: [
            {
                script: "go build -o bin/app ./..."
                // Env no nível da implementação é o mais específico
                env: {
                    vars: {
                        GOOS: "linux"
                        GOARCH: "amd64"
                    }
                }
                target: {
                    runtimes: [{name: "native"}]
                }
            }
        ]
    }
]
```

As variáveis de ambiente são mescladas, com níveis posteriores sobrescrevendo os anteriores.

## Próximos Passos

Agora você conhece o básico de criar invkfiles! Continue aprendendo sobre:

- [Conceitos Principais](../core-concepts/invkfile-format) - Mergulho profundo no formato do invkfile
- [Modos de Runtime](../runtime-modes/overview) - Aprenda sobre runtimes native, virtual e container
- [Dependências](../dependencies/overview) - Todas as formas de declarar dependências
