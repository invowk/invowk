---
sidebar_position: 2
---

# Flags

Flags são opções nomeadas passadas com sintaxe `--name=value`. São ideais para configurações opcionais, switches booleanos e configurações que não seguem uma ordem estrita.

## Definindo Flags

```cue
{
    name: "deploy"
    flags: [
        {
            name: "environment"
            description: "Ambiente alvo"
            required: true
        },
        {
            name: "dry-run"
            description: "Simular sem alterações"
            type: "bool"
            default_value: "false"
        },
        {
            name: "replicas"
            description: "Número de réplicas"
            type: "int"
            default_value: "1"
        }
    ]
    implementations: [...]
}
```

## Propriedades das Flags

| Propriedade | Obrigatória | Descrição |
|-------------|-------------|-----------|
| `name` | Sim | Nome da flag (alfanumérico, hífens, underscores) |
| `description` | Sim | Texto de ajuda |
| `type` | Não | `string`, `bool`, `int`, `float` (padrão: `string`) |
| `default_value` | Não | Valor padrão se não fornecido |
| `required` | Não | Deve ser fornecido (não pode ter default) |
| `short` | Não | Alias de uma letra |
| `validation` | Não | Padrão regex para o valor |

## Tipos

### String (Padrão)

```cue
{name: "message", description: "Mensagem personalizada", type: "string"}
// ou simplesmente
{name: "message", description: "Mensagem personalizada"}
```

```bash
invowk cmd myproject run --message="Hello World"
```

### Boolean

```cue
{name: "verbose", description: "Habilitar saída detalhada", type: "bool", default_value: "false"}
```

```bash
# Habilitar
invowk cmd myproject run --verbose
invowk cmd myproject run --verbose=true

# Desabilitar (explícito)
invowk cmd myproject run --verbose=false
```

Flags booleanas só aceitam `true` ou `false`.

### Integer

```cue
{name: "count", description: "Número de iterações", type: "int", default_value: "5"}
```

```bash
invowk cmd myproject run --count=10
invowk cmd myproject run --count=-1  # Negativo permitido
```

### Float

```cue
{name: "threshold", description: "Limite de confiança", type: "float", default_value: "0.95"}
```

```bash
invowk cmd myproject run --threshold=0.8
invowk cmd myproject run --threshold=1.5e-3  # Notação científica
```

## Obrigatório vs Opcional

### Flags Obrigatórias

```cue
{
    name: "target"
    description: "Destino do deploy"
    required: true  // Deve ser fornecido
}
```

```bash
# Erro: flag obrigatória faltando
invowk cmd myproject deploy
# Error: flag 'target' is required

# Sucesso
invowk cmd myproject deploy --target=production
```

Flags obrigatórias não podem ter um `default_value`.

### Flags Opcionais

```cue
{
    name: "timeout"
    description: "Timeout da requisição em segundos"
    type: "int"
    default_value: "30"  // Usado se não fornecido
}
```

```bash
# Usa padrão (30)
invowk cmd myproject request

# Sobrescrever
invowk cmd myproject request --timeout=60
```

## Aliases Curtos

Adicione atalhos de uma letra:

```cue
flags: [
    {name: "verbose", description: "Saída detalhada", type: "bool", short: "v"},
    {name: "output", description: "Arquivo de saída", short: "o"},
    {name: "force", description: "Forçar sobrescrita", type: "bool", short: "f"},
]
```

```bash
# Forma longa
invowk cmd myproject build --verbose --output=./dist --force

# Forma curta
invowk cmd myproject build -v -o=./dist -f

# Misto
invowk cmd myproject build -v --output=./dist -f
```

## Padrões de Validação

Valide valores de flag com regex:

```cue
flags: [
    {
        name: "env"
        description: "Nome do ambiente"
        validation: "^(dev|staging|prod)$"
        default_value: "dev"
    },
    {
        name: "version"
        description: "Versão semântica"
        validation: "^[0-9]+\\.[0-9]+\\.[0-9]+$"
    }
]
```

```bash
# Válido
invowk cmd myproject deploy --env=prod --version=1.2.3

# Inválido - falha antes da execução
invowk cmd myproject deploy --env=production
# Error: flag 'env' value 'production' does not match required pattern '^(dev|staging|prod)$'
```

## Acessando em Scripts

Flags estão disponíveis como variáveis de ambiente `INVOWK_FLAG_*`:

```cue
{
    name: "deploy"
    flags: [
        {name: "env", description: "Ambiente", required: true},
        {name: "dry-run", description: "Dry run", type: "bool", default_value: "false"},
        {name: "replica-count", description: "Réplicas", type: "int", default_value: "1"},
    ]
    implementations: [{
        script: """
            echo "Environment: $INVOWK_FLAG_ENV"
            echo "Dry run: $INVOWK_FLAG_DRY_RUN"
            echo "Replicas: $INVOWK_FLAG_REPLICA_COUNT"
            
            if [ "$INVOWK_FLAG_DRY_RUN" = "true" ]; then
                echo "Would deploy $INVOWK_FLAG_REPLICA_COUNT replicas to $INVOWK_FLAG_ENV"
            else
                ./deploy.sh "$INVOWK_FLAG_ENV" "$INVOWK_FLAG_REPLICA_COUNT"
            fi
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

### Convenção de Nomenclatura

| Nome da Flag | Variável de Ambiente |
|--------------|---------------------|
| `env` | `INVOWK_FLAG_ENV` |
| `dry-run` | `INVOWK_FLAG_DRY_RUN` |
| `output-file` | `INVOWK_FLAG_OUTPUT_FILE` |
| `retryCount` | `INVOWK_FLAG_RETRYCOUNT` |

Hífens se tornam underscores, maiúsculo.

## Exemplos do Mundo Real

### Comando de Build

```cue
{
    name: "build"
    description: "Build da aplicação"
    flags: [
        {name: "mode", description: "Modo de build", validation: "^(debug|release)$", default_value: "debug"},
        {name: "output", description: "Diretório de saída", short: "o", default_value: "./build"},
        {name: "verbose", description: "Saída detalhada", type: "bool", short: "v"},
        {name: "parallel", description: "Jobs paralelos", type: "int", short: "j", default_value: "4"},
    ]
    implementations: [{
        script: """
            mkdir -p "$INVOWK_FLAG_OUTPUT"
            
            VERBOSE=""
            if [ "$INVOWK_FLAG_VERBOSE" = "true" ]; then
                VERBOSE="-v"
            fi
            
            go build $VERBOSE -o "$INVOWK_FLAG_OUTPUT/app" ./...
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

### Comando de Deploy

```cue
{
    name: "deploy"
    description: "Deploy para a nuvem"
    flags: [
        {
            name: "env"
            description: "Ambiente alvo"
            short: "e"
            required: true
            validation: "^(dev|staging|prod)$"
        },
        {
            name: "version"
            description: "Versão para deploy"
            short: "v"
            validation: "^[0-9]+\\.[0-9]+\\.[0-9]+$"
        },
        {
            name: "dry-run"
            description: "Simular deploy"
            type: "bool"
            short: "n"
            default_value: "false"
        },
        {
            name: "timeout"
            description: "Timeout do deploy (segundos)"
            type: "int"
            default_value: "300"
        }
    ]
    implementations: [{
        script: """
            echo "Deploying version ${INVOWK_FLAG_VERSION:-latest} to $INVOWK_FLAG_ENV"
            
            ARGS="--timeout=$INVOWK_FLAG_TIMEOUT"
            if [ "$INVOWK_FLAG_DRY_RUN" = "true" ]; then
                ARGS="$ARGS --dry-run"
            fi
            
            ./scripts/deploy.sh "$INVOWK_FLAG_ENV" $ARGS
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Variações de Sintaxe de Flag

Todas funcionam:

```bash
# Sinal de igual
--output=./dist

# Separador de espaço
--output ./dist

# Curto com igual
-o=./dist

# Curto com valor
-o ./dist

# Toggle booleano (habilita)
--verbose
-v

# Booleano explícito
--verbose=true
--verbose=false
```

## Flags Reservadas

Não use esses nomes - são reservados pelo Invowk:

| Flag | Curto | Descrição |
|------|-------|-----------|
| `env-file` | `e` | Carregar ambiente de arquivo |
| `env-var` | `E` | Definir variável de ambiente |
| `help` | `h` | Mostrar ajuda |
| `runtime` | - | Sobrescrever runtime |

## Melhores Práticas

1. **Use nomes descritivos**: `--output-dir` não `--od`
2. **Forneça padrões quando faz sentido**: Reduzir entradas obrigatórias
3. **Adicione validação para valores restritos**: Falhe rápido em entrada inválida
4. **Use aliases curtos para flags comuns**: `-v`, `-o`, `-f`
5. **Flags booleanas devem ter padrão false**: Comportamento opt-in

## Próximos Passos

- [Positional Arguments](./positional-arguments) - Para valores ordenados e obrigatórios
- [Environment](../environment/overview) - Configurar ambiente do comando
