---
sidebar_position: 3
---

# Comandos Específicos de Plataforma

Crie comandos que se comportam diferentemente no Linux, macOS e Windows. O Invowk automaticamente seleciona a implementação correta para a plataforma atual.

## Plataformas Suportadas

| Valor | Descrição |
|-------|-----------|
| `linux` | Distribuições Linux |
| `macos` | macOS (Darwin) |
| `windows` | Windows |

## Direcionamento Básico de Plataforma

Restrinja uma implementação a plataformas específicas:

```cue
{
    name: "open-browser"
    implementations: [
        {
            script: "xdg-open http://localhost:3000"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}]
            }
        },
        {
            script: "open http://localhost:3000"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "macos"}]
            }
        },
        {
            script: "start http://localhost:3000"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "windows"}]
            }
        }
    ]
}
```

## Todas as Plataformas (Padrão)

Se `platforms` for omitido, a implementação funciona em todas as plataformas:

```cue
{
    name: "build"
    implementations: [{
        script: "go build ./..."
        target: {
            runtimes: [{name: "native"}]
            // Sem platforms = funciona em qualquer lugar
        }
    }]
}
```

## Comandos Apenas Unix

Direcione tanto Linux quanto macOS:

```cue
{
    name: "check-permissions"
    implementations: [{
        script: """
            chmod +x ./scripts/*.sh
            ls -la ./scripts/
            """
        target: {
            runtimes: [{name: "native"}]
            platforms: [{name: "linux"}, {name: "macos"}]
        }
    }]
}
```

## Ambiente Específico de Plataforma

Defina variáveis de ambiente diferentes por plataforma:

```cue
{
    name: "configure"
    implementations: [{
        script: "echo \"Config: $CONFIG_PATH\""
        target: {
            runtimes: [{name: "native"}]
            platforms: [
                {
                    name: "linux"
                    env: {
                        CONFIG_PATH: "/etc/myapp/config.yaml"
                        CACHE_DIR: "/var/cache/myapp"
                    }
                },
                {
                    name: "macos"
                    env: {
                        CONFIG_PATH: "/usr/local/etc/myapp/config.yaml"
                        CACHE_DIR: "${HOME}/Library/Caches/myapp"
                    }
                },
                {
                    name: "windows"
                    env: {
                        CONFIG_PATH: "%APPDATA%\\myapp\\config.yaml"
                        CACHE_DIR: "%LOCALAPPDATA%\\myapp\\cache"
                    }
                }
            ]
        }
    }]
}
```

## Scripts Multiplataforma

Escreva um script que funciona em qualquer lugar:

```cue
{
    name: "build"
    implementations: [{
        script: """
            go build -o ${OUTPUT_NAME} ./...
            """
        target: {
            runtimes: [{name: "native"}]
            platforms: [
                {name: "linux", env: {OUTPUT_NAME: "bin/app"}},
                {name: "macos", env: {OUTPUT_NAME: "bin/app"}},
                {name: "windows", env: {OUTPUT_NAME: "bin/app.exe"}}
            ]
        }
    }]
}
```

## Templates CUE para Plataformas

Use templates CUE para reduzir repetição:

```cue
// Definir templates de plataforma
_linux: {name: "linux"}
_macos: {name: "macos"}
_windows: {name: "windows"}

_unix: [{name: "linux"}, {name: "macos"}]
_all: [{name: "linux"}, {name: "macos"}, {name: "windows"}]

commands: [
    {
        name: "clean"
        implementations: [
            // Implementação Unix
            {
                script: "rm -rf build/"
                target: {
                    runtimes: [{name: "native"}]
                    platforms: _unix
                }
            },
            // Implementação Windows
            {
                script: "rmdir /s /q build"
                target: {
                    runtimes: [{name: "native"}]
                    platforms: [_windows]
                }
            }
        ]
    }
]
```

## Listagem de Comandos

A lista de comandos mostra plataformas suportadas:

```
Available Commands
  (* = default runtime)

From current directory:
  myproject build - Build the project [native*] (linux, macos, windows)
  myproject clean - Clean artifacts [native*] (linux, macos)
  myproject deploy - Deploy to cloud [native*] (linux)
```

## Erro de Plataforma Não Suportada

Executar um comando em uma plataforma não suportada mostra um erro claro:

```
✗ Host not supported

Command 'deploy' cannot run on this host.

Current host:     windows
Supported hosts:  linux, macos

This command is only available on the platforms listed above.
```

## Exemplos do Mundo Real

### Informações do Sistema

```cue
{
    name: "sysinfo"
    implementations: [
        {
            script: """
                echo "Hostname: $(hostname)"
                echo "Kernel: $(uname -r)"
                echo "Memory: $(free -h | awk '/^Mem:/{print $2}')"
                """
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}]
            }
        },
        {
            script: """
                echo "Hostname: $(hostname)"
                echo "Kernel: $(uname -r)"
                echo "Memory: $(sysctl -n hw.memsize | awk '{print $0/1024/1024/1024 "GB"}')"
                """
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "macos"}]
            }
        },
        {
            script: """
                echo Hostname: %COMPUTERNAME%
                systeminfo | findstr "Total Physical Memory"
                """
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "windows"}]
            }
        }
    ]
}
```

### Instalação de Pacotes

```cue
{
    name: "install-deps"
    implementations: [
        {
            script: "apt-get install -y build-essential"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}]
            }
        },
        {
            script: "brew install coreutils"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "macos"}]
            }
        },
        {
            script: "choco install make"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "windows"}]
            }
        }
    ]
}
```

## Melhores Práticas

1. **Comece com multiplataforma**: Adicione específico de plataforma apenas quando necessário
2. **Use variáveis de ambiente**: Abstraia diferenças de plataforma
3. **Teste em todas as plataformas**: CI deve cobrir todas as plataformas suportadas
4. **Documente limitações**: Note quais plataformas são suportadas

## Próximos Passos

- [Interpreters](./interpreters) - Usar interpretadores não-shell
- [Working Directory](./workdir) - Controlar local de execução
