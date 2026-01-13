---
sidebar_position: 2
---

# Início Rápido

Vamos executar seu primeiro comando Invowk em menos de 2 minutos. Sério, pegue um café antes se quiser - você terá tempo de sobra.

## Crie Seu Primeiro Invkfile

Navegue até qualquer diretório de projeto e inicialize um invkfile:

```bash
cd meu-projeto
invowk init
```

Isso cria um `invkfile.cue` com um comando de exemplo. Vamos dar uma olhada:

```cue
group: "myproject"
version: "1.0"
description: "My project commands"

commands: [
    {
        name: "hello"
        description: "Say hello!"
        implementations: [
            {
                script: "echo 'Hello from Invowk!'"
                target: {
                    runtimes: [{name: "native"}]
                }
            }
        ]
    }
]
```

Não se preocupe em entender tudo ainda - vamos cobrir isso em breve!

## Listar Comandos Disponíveis

Veja quais comandos estão disponíveis:

```bash
invowk cmd --list
# ou apenas
invowk cmd
```

Você verá algo como:

```
Available Commands
  (* = default runtime)

From current directory:
  myproject hello - Say hello! [native*] (linux, macos, windows)
```

Note como o comando é prefixado com `myproject` (o nome do grupo). Isso mantém os comandos organizados e evita conflitos de nomes.

## Executar um Comando

Agora vamos executá-lo:

```bash
invowk cmd myproject hello
```

Saída:

```
Hello from Invowk!
```

É isso! Você acabou de executar seu primeiro comando Invowk.

## Vamos Tornar Isso Mais Interessante

Edite seu `invkfile.cue` para adicionar um comando mais útil:

```cue
group: "myproject"
version: "1.0"
description: "My project commands"

commands: [
    {
        name: "hello"
        description: "Say hello!"
        implementations: [
            {
                script: "echo 'Hello from Invowk!'"
                target: {
                    runtimes: [{name: "native"}]
                }
            }
        ]
    },
    {
        name: "info"
        description: "Show system information"
        implementations: [
            {
                script: """
                    echo "=== System Info ==="
                    echo "User: $USER"
                    echo "Directory: $(pwd)"
                    echo "Date: $(date)"
                    """
                target: {
                    runtimes: [{name: "native"}]
                    platforms: [{name: "linux"}, {name: "macos"}]
                }
            }
        ]
    }
]
```

Agora execute:

```bash
invowk cmd myproject info
```

Você verá as informações do seu sistema exibidas de forma organizada.

## Experimente o Runtime Virtual

Um dos superpoderes do Invowk é o **runtime virtual** - um interpretador de shell integrado que funciona da mesma forma em todas as plataformas:

```cue
{
    name: "cross-platform"
    description: "Works the same everywhere!"
    implementations: [
        {
            script: "echo 'This runs identically on Linux, Mac, and Windows!'"
            target: {
                runtimes: [{name: "virtual"}]
            }
        }
    ]
}
```

O runtime virtual usa o interpretador [mvdan/sh](https://github.com/mvdan/sh), proporcionando comportamento consistente de shell POSIX em todas as plataformas.

## Próximos Passos

Você acabou de arranhar a superfície! Vá para [Seu Primeiro Invkfile](./your-first-invkfile) para aprender como criar comandos mais poderosos com:

- Múltiplas opções de runtime (native, virtual, container)
- Dependências que são validadas antes da execução
- Flags e argumentos de comando
- Variáveis de ambiente
- E muito mais!
