---
sidebar_position: 3
---

# Argumentos Posicionais

Argumentos posicionais são valores passados por posição após o nome do comando. São ideais para entradas obrigatórias onde a ordem é natural (como `source` e `destination`).

## Definindo Argumentos

```cue
{
    name: "copy"
    args: [
        {
            name: "source"
            description: "Arquivo ou diretório de origem"
            required: true
        },
        {
            name: "destination"
            description: "Caminho de destino"
            required: true
        }
    ]
    implementations: [...]
}
```

Uso:
```bash
invowk cmd myproject copy ./src ./dest
```

## Propriedades dos Argumentos

| Propriedade | Obrigatória | Descrição |
|-------------|-------------|-----------|
| `name` | Sim | Nome do argumento (alfanumérico, hífens, underscores) |
| `description` | Sim | Texto de ajuda |
| `type` | Não | `string`, `int`, `float` (padrão: `string`) |
| `default_value` | Não | Valor padrão se não fornecido |
| `required` | Não | Deve ser fornecido (não pode ter default) |
| `validation` | Não | Padrão regex para o valor |
| `variadic` | Não | Aceita múltiplos valores (apenas último arg) |

## Tipos

### String (Padrão)

```cue
{name: "filename", description: "Arquivo para processar", type: "string"}
```

### Integer

```cue
{name: "count", description: "Número de itens", type: "int", default_value: "10"}
```

```bash
invowk cmd myproject generate 5
```

### Float

```cue
{name: "ratio", description: "Proporção de escala", type: "float", default_value: "1.0"}
```

```bash
invowk cmd myproject scale 0.5
```

Nota: Tipo Boolean **não é suportado** para argumentos. Use flags para opções booleanas.

## Obrigatório vs Opcional

### Argumentos Obrigatórios

```cue
args: [
    {name: "input", description: "Arquivo de entrada", required: true},
    {name: "output", description: "Arquivo de saída", required: true},
]
```

```bash
# Erro: argumento obrigatório faltando
invowk cmd myproject convert input.txt
# Error: argument 'output' is required

# Sucesso
invowk cmd myproject convert input.txt output.txt
```

### Argumentos Opcionais

```cue
args: [
    {name: "input", description: "Arquivo de entrada", required: true},
    {name: "format", description: "Formato de saída", default_value: "json"},
]
```

```bash
# Usa formato padrão (json)
invowk cmd myproject parse input.txt

# Sobrescrever formato
invowk cmd myproject parse input.txt yaml
```

### Regra de Ordenação

Argumentos obrigatórios devem vir antes dos argumentos opcionais:

```cue
// Bom
args: [
    {name: "input", required: true},      // Obrigatório primeiro
    {name: "output", required: true},     // Obrigatório segundo
    {name: "format", default_value: "json"}, // Opcional por último
]

// Ruim - causará erro de validação
args: [
    {name: "format", default_value: "json"}, // Opcional não pode vir primeiro
    {name: "input", required: true},
]
```

## Argumentos Variadic

O último argumento pode aceitar múltiplos valores:

```cue
{
    name: "process"
    args: [
        {name: "output", description: "Arquivo de saída", required: true},
        {name: "inputs", description: "Arquivos de entrada", variadic: true},
    ]
    implementations: [{
        script: """
            echo "Output: $INVOWK_ARG_OUTPUT"
            echo "Inputs: $INVOWK_ARG_INPUTS"
            echo "Count: $INVOWK_ARG_INPUTS_COUNT"
            
            for i in $(seq 1 $INVOWK_ARG_INPUTS_COUNT); do
                eval "file=\$INVOWK_ARG_INPUTS_$i"
                echo "Processing: $file"
            done
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

```bash
invowk cmd myproject process out.txt a.txt b.txt c.txt
# Output: out.txt
# Inputs: a.txt b.txt c.txt
# Count: 3
# Processing: a.txt
# Processing: b.txt
# Processing: c.txt
```

### Variáveis de Ambiente para Variadic

| Variável | Descrição |
|----------|-----------|
| `INVOWK_ARG_INPUTS` | Valores unidos por espaço |
| `INVOWK_ARG_INPUTS_COUNT` | Número de valores |
| `INVOWK_ARG_INPUTS_1` | Primeiro valor |
| `INVOWK_ARG_INPUTS_2` | Segundo valor |
| ... | E assim por diante |

## Padrões de Validação

Valide valores de argumento com regex:

```cue
args: [
    {
        name: "environment"
        description: "Ambiente alvo"
        required: true
        validation: "^(dev|staging|prod)$"
    },
    {
        name: "version"
        description: "Número da versão"
        validation: "^[0-9]+\\.[0-9]+\\.[0-9]+$"
    }
]
```

```bash
# Válido
invowk cmd myproject deploy prod 1.2.3

# Inválido
invowk cmd myproject deploy production
# Error: argument 'environment' value 'production' does not match pattern '^(dev|staging|prod)$'
```

## Acessando em Scripts

### Variáveis de Ambiente

```cue
{
    name: "greet"
    args: [
        {name: "first-name", required: true},
        {name: "last-name", default_value: "User"},
    ]
    implementations: [{
        script: """
            echo "Hello, $INVOWK_ARG_FIRST_NAME $INVOWK_ARG_LAST_NAME!"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

### Convenção de Nomenclatura

| Nome do Argumento | Variável de Ambiente |
|-------------------|---------------------|
| `name` | `INVOWK_ARG_NAME` |
| `file-path` | `INVOWK_ARG_FILE_PATH` |
| `outputDir` | `INVOWK_ARG_OUTPUTDIR` |

### Parâmetros Posicionais do Shell

Argumentos também estão disponíveis como `$1`, `$2`, etc.:

```cue
{
    name: "copy"
    args: [
        {name: "source", required: true},
        {name: "dest", required: true},
    ]
    implementations: [{
        script: """
            # Usando variáveis de ambiente
            cp "$INVOWK_ARG_SOURCE" "$INVOWK_ARG_DEST"
            
            # Ou parâmetros posicionais
            cp "$1" "$2"
            
            # Todos os argumentos
            echo "Args: $@"
            echo "Count: $#"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

### Compatibilidade de Shell

| Shell | Acesso Posicional |
|-------|-------------------|
| bash, sh, zsh | `$1`, `$2`, `$@`, `$#` |
| PowerShell | `$args[0]`, `$args[1]` |
| runtime virtual | `$1`, `$2`, `$@`, `$#` |
| container | `$1`, `$2`, `$@`, `$#` |

## Exemplos do Mundo Real

### Processamento de Arquivo

```cue
{
    name: "convert"
    description: "Converter formato de arquivo"
    args: [
        {
            name: "input"
            description: "Arquivo de entrada"
            required: true
        },
        {
            name: "output"
            description: "Arquivo de saída"
            required: true
        },
        {
            name: "format"
            description: "Formato de saída"
            default_value: "json"
            validation: "^(json|yaml|toml|xml)$"
        }
    ]
    implementations: [{
        script: """
            echo "Converting $1 to $2 as $3"
            ./converter --input="$1" --output="$2" --format="$3"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

### Operação Multi-Arquivo

```cue
{
    name: "compress"
    description: "Comprimir arquivos em arquivo"
    args: [
        {
            name: "archive"
            description: "Nome do arquivo de saída"
            required: true
            validation: "\\.(zip|tar\\.gz|tgz)$"
        },
        {
            name: "files"
            description: "Arquivos para comprimir"
            variadic: true
        }
    ]
    implementations: [{
        script: """
            if [ -z "$INVOWK_ARG_FILES" ]; then
                echo "No files specified!"
                exit 1
            fi
            
            # Usar a lista separada por espaço
            tar -czvf "$INVOWK_ARG_ARCHIVE" $INVOWK_ARG_FILES
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

### Deploy

```cue
{
    name: "deploy"
    description: "Deploy de serviços"
    args: [
        {
            name: "env"
            description: "Ambiente alvo"
            required: true
            validation: "^(dev|staging|prod)$"
        },
        {
            name: "replicas"
            description: "Número de réplicas"
            type: "int"
            default_value: "1"
        },
        {
            name: "services"
            description: "Serviços para deploy"
            variadic: true
        }
    ]
    implementations: [{
        script: """
            echo "Deploying to $INVOWK_ARG_ENV with $INVOWK_ARG_REPLICAS replicas"
            
            if [ -n "$INVOWK_ARG_SERVICES" ]; then
                for i in $(seq 1 $INVOWK_ARG_SERVICES_COUNT); do
                    eval "service=\$INVOWK_ARG_SERVICES_$i"
                    echo "Deploying $service..."
                    kubectl scale deployment/$service --replicas=$INVOWK_ARG_REPLICAS
                done
            else
                echo "Deploying all services..."
                kubectl scale deployment --all --replicas=$INVOWK_ARG_REPLICAS
            fi
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Misturando com Flags

Flags podem aparecer em qualquer lugar; argumentos são posicionais:

```bash
# Todos equivalentes
invowk cmd myproject deploy prod 3 --dry-run
invowk cmd myproject deploy --dry-run prod 3
invowk cmd myproject deploy prod --dry-run 3
```

Argumentos são extraídos em ordem, independentemente das posições das flags.

## Argumentos vs Subcomandos

Um comando não pode ter ambos argumentos e subcomandos. Se um comando tem subcomandos, argumentos são ignorados:

```
Warning: command has both args and subcommands!

Command 'deploy' defines positional arguments but also has subcommands.
Subcommands take precedence; positional arguments will be ignored.
```

Escolha uma abordagem:
- Use argumentos para comandos simples
- Use subcomandos para hierarquias de comando complexas

## Melhores Práticas

1. **Args obrigatórios primeiro**: Siga as regras de ordenação
2. **Use nomes significativos**: `source` e `dest` não `arg1` e `arg2`
3. **Valide quando possível**: Capture erros cedo
4. **Documente com descrições**: Ajude usuários a entender
5. **Prefira flags para valores opcionais**: Mais fácil de entender

## Próximos Passos

- [Flags](./flags) - Para valores opcionais nomeados
- [Environment](../environment/overview) - Configurar ambiente do comando
