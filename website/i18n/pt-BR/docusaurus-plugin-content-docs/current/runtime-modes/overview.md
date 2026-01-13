---
sidebar_position: 1
---

# Visão Geral dos Modos de Runtime

O Invowk oferece três formas diferentes de executar comandos, cada uma com seus próprios pontos fortes. Escolha o runtime certo para seu caso de uso.

## Os Três Runtimes

| Runtime | Descrição | Melhor Para |
|---------|-----------|-------------|
| **native** | Shell padrão do sistema | Desenvolvimento diário, performance |
| **virtual** | Shell POSIX integrado | Scripts multiplataforma, portabilidade |
| **container** | Container Docker/Podman | Reprodutibilidade, isolamento |

## Comparação Rápida

```cue
commands: [
    // Native: usa o shell do seu sistema (bash, zsh, PowerShell, etc.)
    {
        name: "build native"
        implementations: [{
            script: "go build ./..."
            target: {runtimes: [{name: "native"}]}
        }]
    },
    
    // Virtual: usa shell POSIX-compatível integrado
    {
        name: "build virtual"
        implementations: [{
            script: "go build ./..."
            target: {runtimes: [{name: "virtual"}]}
        }]
    },
    
    // Container: executa dentro de um container
    {
        name: "build container"
        implementations: [{
            script: "go build -o /workspace/bin/app ./..."
            target: {runtimes: [{name: "container", image: "golang:1.21"}]}
        }]
    }
]
```

## Quando Usar Cada Runtime

### Runtime Native

Use **native** quando você quiser:
- Máxima performance
- Acesso a todas as ferramentas do sistema
- Recursos específicos do shell (bash completions, plugins do zsh)
- Integração com seu ambiente de desenvolvimento

```cue
target: {runtimes: [{name: "native"}]}
```

### Runtime Virtual

Use **virtual** quando você quiser:
- Comportamento consistente entre plataformas
- Scripts POSIX-compatíveis que funcionam em qualquer lugar
- Sem dependência de shell externo
- Debug mais simples de scripts shell

```cue
target: {runtimes: [{name: "virtual"}]}
```

### Runtime Container

Use **container** quando você quiser:
- Builds reproduzíveis
- Ambientes isolados
- Versões específicas de ferramentas
- Execução em ambiente limpo

```cue
target: {runtimes: [{name: "container", image: "golang:1.21"}]}
```

## Múltiplos Runtimes Por Comando

Comandos podem suportar múltiplos runtimes. O primeiro é o padrão:

```cue
{
    name: "build"
    implementations: [{
        script: "go build ./..."
        target: {
            runtimes: [
                {name: "native"},  // Padrão
                {name: "virtual"}, // Alternativa
                {name: "container", image: "golang:1.21"}  // Reproduzível
            ]
        }
    }]
}
```

### Sobrescrevendo em Tempo de Execução

```bash
# Usar padrão (native)
invowk cmd myproject build

# Sobrescrever para virtual
invowk cmd myproject build --runtime virtual

# Sobrescrever para container
invowk cmd myproject build --runtime container
```

## Listagem de Comandos

A lista de comandos mostra runtimes disponíveis com um asterisco marcando o padrão:

```
Available Commands
  (* = default runtime)

From current directory:
  myproject build - Build the project [native*, virtual, container] (linux, macos)
```

## Fluxo de Seleção de Runtime

```
                    ┌──────────────────┐
                    │  invowk cmd run  │
                    └────────┬─────────┘
                             │
                    ┌────────▼─────────┐
                    │ flag --runtime?  │
                    └────────┬─────────┘
                             │
              ┌──────────────┴──────────────┐
              │ Sim                         │ Não
              ▼                             ▼
    ┌─────────────────────┐    ┌─────────────────────┐
    │ Usar runtime        │    │ Usar primeiro       │
    │ especificado        │    │ runtime (padrão)    │
    └─────────────────────┘    └─────────────────────┘
```

## Validação de Dependências

Dependências são validadas de acordo com o runtime:

| Runtime | Dependências Validadas Contra |
|---------|-------------------------------|
| native | Shell e ferramentas do sistema host |
| virtual | Shell integrado com utilitários core |
| container | Shell e ambiente do container |

Isso significa que uma dependência `tools` como `go` é verificada:
- **native**: O `go` está no PATH do host?
- **virtual**: O `go` está disponível nos built-ins do shell virtual?
- **container**: O `go` está instalado na imagem do container?

## Próximos Passos

Aprofunde-se em cada runtime:

- [Runtime Native](./native) - Execução no shell do sistema
- [Runtime Virtual](./virtual) - Shell POSIX integrado
- [Runtime Container](./container) - Execução em Docker/Podman
