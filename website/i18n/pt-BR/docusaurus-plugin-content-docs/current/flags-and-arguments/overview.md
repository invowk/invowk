---
sidebar_position: 1
---

# Visão Geral de Flags e Argumentos

Comandos podem aceitar entrada do usuário através de **flags** (opções nomeadas) e **argumentos** (valores posicionais). Ambos são passados para seus scripts como variáveis de ambiente.

## Comparação Rápida

| Recurso | Flags | Argumentos |
|---------|-------|------------|
| Sintaxe | `--name=value` ou `--name value` | Posicional: `value1 value2` |
| Ordem | Qualquer ordem | Deve seguir ordem de posição |
| Boolean | Suportado (`--verbose`) | Não suportado |
| Acesso nomeado | `INVOWK_FLAG_NAME` | `INVOWK_ARG_NAME` |
| Múltiplos valores | Não | Sim (variadic) |

## Comando de Exemplo

```cue
{
    name: "deploy"
    description: "Deploy para um ambiente"
    
    // Flags - opções nomeadas
    flags: [
        {name: "dry-run", type: "bool", default_value: "false"},
        {name: "replicas", type: "int", default_value: "1"},
    ]
    
    // Argumentos - valores posicionais
    args: [
        {name: "environment", required: true},
        {name: "services", variadic: true},
    ]
    
    implementations: [{
        script: """
            echo "Deploying to $INVOWK_ARG_ENVIRONMENT"
            echo "Replicas: $INVOWK_FLAG_REPLICAS"
            echo "Dry run: $INVOWK_FLAG_DRY_RUN"
            echo "Services: $INVOWK_ARG_SERVICES"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

Uso:
```bash
invowk cmd myproject deploy production api web --dry-run --replicas=3
```

## Flags

Flags são opções nomeadas com sintaxe `--name=value`:

```cue
flags: [
    {name: "verbose", type: "bool", short: "v"},
    {name: "output", type: "string", short: "o", default_value: "./dist"},
    {name: "count", type: "int", default_value: "1"},
]
```

```bash
# Forma longa
invowk cmd myproject build --verbose --output=./build --count=5

# Forma curta
invowk cmd myproject build -v -o=./build
```

Recursos principais:
- Suportam aliases curtos (`-v` para `--verbose`)
- Valores tipados (string, bool, int, float)
- Opcional ou obrigatório
- Valores padrão
- Validação por regex

Veja [Flags](./flags) para detalhes.

## Argumentos

Argumentos são valores posicionais após o nome do comando:

```cue
args: [
    {name: "source", required: true},
    {name: "destination", default_value: "./output"},
    {name: "files", variadic: true},
]
```

```bash
invowk cmd myproject copy ./src ./dest file1.txt file2.txt
```

Recursos principais:
- Baseado em posição (ordem importa)
- Obrigatório ou opcional
- Valores padrão
- Último argumento pode ser variadic (aceita múltiplos valores)
- Valores tipados (string, int, float - mas não bool)

Veja [Positional Arguments](./positional-arguments) para detalhes.

## Acesso via Variável de Ambiente

Tanto flags quanto argumentos estão disponíveis como variáveis de ambiente:

### Variáveis de Flag

Prefixo: `INVOWK_FLAG_`

| Nome da Flag | Variável de Ambiente |
|--------------|---------------------|
| `verbose` | `INVOWK_FLAG_VERBOSE` |
| `dry-run` | `INVOWK_FLAG_DRY_RUN` |
| `output-file` | `INVOWK_FLAG_OUTPUT_FILE` |

### Variáveis de Argumento

Prefixo: `INVOWK_ARG_`

| Nome do Argumento | Variável de Ambiente |
|-------------------|---------------------|
| `environment` | `INVOWK_ARG_ENVIRONMENT` |
| `file-path` | `INVOWK_ARG_FILE_PATH` |

### Argumentos Variadic

Variáveis adicionais para múltiplos valores:

| Variável | Descrição |
|----------|-----------|
| `INVOWK_ARG_FILES` | Valores separados por espaço |
| `INVOWK_ARG_FILES_COUNT` | Número de valores |
| `INVOWK_ARG_FILES_1` | Primeiro valor |
| `INVOWK_ARG_FILES_2` | Segundo valor |

## Parâmetros Posicionais do Shell

Argumentos também estão disponíveis como parâmetros posicionais do shell:

```cue
{
    name: "greet"
    args: [
        {name: "first-name"},
        {name: "last-name"},
    ]
    implementations: [{
        script: """
            # Usando variáveis de ambiente
            echo "Hello, $INVOWK_ARG_FIRST_NAME $INVOWK_ARG_LAST_NAME!"
            
            # Ou usando parâmetros posicionais
            echo "Hello, $1 $2!"
            
            # Todos os argumentos
            echo "All: $@"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Misturando Flags e Argumentos

Flags podem aparecer em qualquer lugar na linha de comando:

```bash
# Todos equivalentes
invowk cmd myproject deploy production --dry-run api web
invowk cmd myproject deploy --dry-run production api web
invowk cmd myproject deploy production api web --dry-run
```

## Flags Reservadas

Alguns nomes de flag são reservados pelo Invowk:

- `env-file` / `-e` - Carregar ambiente de arquivo
- `env-var` / `-E` - Definir variável de ambiente

Não use esses nomes para suas flags.

## Saída de Ajuda

Flags e argumentos aparecem na ajuda do comando:

```bash
invowk cmd myproject deploy --help
```

```
Usage:
  invowk cmd myproject deploy <environment> [services]... [flags]

Arguments:
  environment          (required) - Target environment
  services             (optional) (variadic) - Services to deploy

Flags:
      --dry-run          Perform a dry run (default: false)
  -n, --replicas int     Number of replicas (default: 1)
  -h, --help             help for deploy
```

## Próximos Passos

- [Flags](./flags) - Opções nomeadas com sintaxe `--flag=value`
- [Positional Arguments](./positional-arguments) - Entrada baseada em valor
