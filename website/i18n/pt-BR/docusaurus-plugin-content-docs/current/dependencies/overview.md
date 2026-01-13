---
sidebar_position: 1
---

# Visão Geral de Dependências

Dependências permitem que você declare o que seu comando precisa antes de executar. O Invowk valida todas as dependências antecipadamente e fornece mensagens de erro claras quando algo está faltando.

## Por Que Declarar Dependências?

Sem verificações de dependência:
```bash
$ invowk cmd myproject build
./scripts/build.sh: line 5: go: command not found
```

Com verificações de dependência:
```
$ invowk cmd myproject build

✗ Dependencies not satisfied

Command 'build' has unmet dependencies:

Missing Tools:
  • go - not found in PATH

Install the missing tools and try again.
```

Muito melhor! Você sabe exatamente o que está errado antes de qualquer coisa executar.

## Tipos de Dependência

O Invowk suporta seis tipos de dependências:

| Tipo | Verifica |
|------|----------|
| [tools](./tools) | Binários no PATH |
| [filepaths](./filepaths) | Arquivos ou diretórios |
| [commands](./commands) | Outros comandos Invowk |
| [capabilities](./capabilities) | Capacidades do sistema (rede) |
| [env_vars](./env-vars) | Variáveis de ambiente |
| [custom_checks](./custom-checks) | Scripts de validação personalizados |

## Sintaxe Básica

Dependências são declaradas no bloco `depends_on`:

```cue
{
    name: "build"
    depends_on: {
        tools: [
            {alternatives: ["go"]}
        ]
        filepaths: [
            {alternatives: ["go.mod"]}
        ]
    }
    implementations: [...]
}
```

## O Padrão de Alternativas

Toda dependência usa uma lista `alternatives` com **semântica OU**:

```cue
// QUALQUER uma dessas ferramentas satisfaz a dependência
tools: [
    {alternatives: ["podman", "docker"]}
]

// QUALQUER um desses arquivos satisfaz a dependência
filepaths: [
    {alternatives: ["config.yaml", "config.json", "config.toml"]}
]
```

Se **qualquer** alternativa for encontrada, a dependência é satisfeita. O Invowk usa retorno antecipado - ele para de verificar assim que uma corresponde.

## Ordem de Validação

Dependências são validadas nesta ordem:

1. **env_vars** - Variáveis de ambiente (verificadas primeiro!)
2. **tools** - Binários no PATH
3. **filepaths** - Arquivos e diretórios
4. **capabilities** - Capacidades do sistema
5. **commands** - Outros comandos Invowk
6. **custom_checks** - Scripts de validação personalizados

Variáveis de ambiente são validadas primeiro, antes do Invowk definir qualquer ambiente no nível do comando. Isso garante que você está verificando o ambiente real do usuário.

## Níveis de Escopo

Dependências podem ser declaradas em três níveis:

### Nível Raiz (Global)

Aplica-se a todos os comandos no invkfile:

```cue
group: "myproject"

depends_on: {
    tools: [{alternatives: ["git"]}]  // Requerido por todos os comandos
}

commands: [...]
```

### Nível de Comando

Aplica-se a um comando específico:

```cue
{
    name: "build"
    depends_on: {
        tools: [{alternatives: ["go"]}]  // Requerido por este comando
    }
    implementations: [...]
}
```

### Nível de Implementação

Aplica-se a uma implementação específica:

```cue
{
    name: "build"
    implementations: [
        {
            script: "go build ./..."
            target: {runtimes: [{name: "container", image: "golang:1.21"}]}
            depends_on: {
                // Validado DENTRO do container
                tools: [{alternatives: ["go"]}]
            }
        }
    ]
}
```

### Herança de Escopo

Dependências são **combinadas** entre níveis:

```cue
group: "myproject"

// Nível raiz: requer git
depends_on: {
    tools: [{alternatives: ["git"]}]
}

commands: [
    {
        name: "build"
        // Nível do comando: também requer go
        depends_on: {
            tools: [{alternatives: ["go"]}]
        }
        implementations: [
            {
                script: "go build ./..."
                target: {runtimes: [{name: "native"}]}
                // Nível de implementação: também requer make
                depends_on: {
                    tools: [{alternatives: ["make"]}]
                }
            }
        ]
    }
]

// Dependências efetivas para "build": git + go + make
```

## Validação Consciente do Runtime

Dependências são validadas de acordo com o runtime:

| Runtime | Dependências Verificadas Contra |
|---------|--------------------------------|
| native | Sistema host |
| virtual | Shell virtual com built-ins |
| container | Dentro do container |

Isso é poderoso para comandos container:

```cue
{
    name: "build"
    implementations: [
        {
            script: "go build ./..."
            target: {
                runtimes: [{name: "container", image: "golang:1.21"}]
            }
            depends_on: {
                // Verificado DENTRO do container, não no host
                tools: [{alternatives: ["go"]}]
                filepaths: [{alternatives: ["/workspace/go.mod"]}]
            }
        }
    ]
}
```

## Mensagens de Erro

Quando dependências não são satisfeitas, o Invowk mostra um erro útil:

```
✗ Dependencies not satisfied

Command 'deploy' has unmet dependencies:

Missing Tools:
  • docker - not found in PATH
  • kubectl - not found in PATH

Missing Files:
  • Dockerfile - file not found

Missing Environment Variables:
  • AWS_ACCESS_KEY_ID - not set in environment

Install the missing tools and try again.
```

## Exemplo Completo

Aqui está um comando com múltiplos tipos de dependência:

```cue
{
    name: "deploy"
    description: "Deploy to production"
    depends_on: {
        // Verificar ambiente primeiro
        env_vars: [
            {alternatives: [{name: "AWS_ACCESS_KEY_ID"}, {name: "AWS_PROFILE"}]}
        ]
        // Verificar ferramentas requeridas
        tools: [
            {alternatives: ["docker", "podman"]},
            {alternatives: ["kubectl"]}
        ]
        // Verificar arquivos requeridos
        filepaths: [
            {alternatives: ["Dockerfile"]},
            {alternatives: ["k8s/deployment.yaml"]}
        ]
        // Verificar conectividade de rede
        capabilities: [
            {alternatives: ["internet"]}
        ]
        // Executar outros comandos primeiro
        commands: [
            {alternatives: ["myproject build"]},
            {alternatives: ["myproject test"]}
        ]
    }
    implementations: [
        {
            script: "./scripts/deploy.sh"
            target: {runtimes: [{name: "native"}]}
        }
    ]
}
```

## Próximos Passos

Aprenda sobre cada tipo de dependência em detalhes:

- [Tools](./tools) - Verificar binários no PATH
- [Filepaths](./filepaths) - Verificar arquivos e diretórios
- [Commands](./commands) - Exigir que outros comandos executem primeiro
- [Capabilities](./capabilities) - Verificar capacidades do sistema
- [Variáveis de Ambiente](./env-vars) - Verificar variáveis de ambiente
- [Verificações Personalizadas](./custom-checks) - Escrever scripts de validação personalizados
