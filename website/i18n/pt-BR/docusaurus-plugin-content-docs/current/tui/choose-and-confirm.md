---
sidebar_position: 3
---

# Choose e Confirm

Componentes de seleção e confirmação para decisões do usuário.

## Choose

Selecione uma ou mais opções de uma lista.

### Uso Básico

```bash
invowk tui choose "Option 1" "Option 2" "Option 3"
```

### Opções

| Opção | Descrição |
|-------|-----------|
| `--title` | Prompt de seleção |
| `--limit` | Máximo de seleções (padrão: 1) |
| `--no-limit` | Seleções ilimitadas |
| `--cursor` | Caractere do cursor |
| `--selected` | Itens pré-selecionados |

### Seleção Única

```bash
# Básico
COLOR=$(invowk tui choose red green blue)
echo "You chose: $COLOR"

# Com título
ENV=$(invowk tui choose --title "Select environment" dev staging prod)
```

### Seleção Múltipla

```bash
# Multi-seleção limitada (até 3)
ITEMS=$(invowk tui choose --limit 3 "One" "Two" "Three" "Four" "Five")

# Multi-seleção ilimitada
ITEMS=$(invowk tui choose --no-limit "One" "Two" "Three" "Four" "Five")
```

Múltiplas seleções são retornadas como valores separados por nova linha:

```bash
SERVICES=$(invowk tui choose --no-limit --title "Select services to deploy" \
    api web worker scheduler)

echo "$SERVICES" | while read -r service; do
    echo "Deploying: $service"
done
```

### Opções Pré-Selecionadas

```bash
invowk tui choose --selected "Two" "One" "Two" "Three"
```

### Exemplos do Mundo Real

#### Seleção de Environment

```bash
ENV=$(invowk tui choose --title "Deploy to which environment?" \
    development staging production)

case "$ENV" in
    production)
        if ! invowk tui confirm "Are you sure? This is PRODUCTION!"; then
            exit 1
        fi
        ;;
esac

./deploy.sh "$ENV"
```

#### Seleção de Serviço

```bash
SERVICES=$(invowk tui choose --no-limit --title "Which services?" \
    api web worker cron)

for service in $SERVICES; do
    echo "Restarting $service..."
    systemctl restart "$service"
done
```

---

## Confirm

Prompt de confirmação sim/não.

### Uso Básico

```bash
invowk tui confirm "Are you sure?"
```

Retorna:
- Código de saída 0 se o usuário confirmar (sim)
- Código de saída 1 se o usuário recusar (não)

### Opções

| Opção | Descrição |
|-------|-----------|
| `--affirmative` | Label customizado para "sim" |
| `--negative` | Label customizado para "não" |
| `--default` | Padrão para sim |

### Exemplos

```bash
# Confirmação básica
if invowk tui confirm "Continue?"; then
    echo "Continuing..."
else
    echo "Cancelled."
fi

# Labels customizados
if invowk tui confirm --affirmative "Delete" --negative "Cancel" "Delete all files?"; then
    rm -rf ./temp/*
fi

# Padrão para sim (usuário apenas pressiona Enter)
if invowk tui confirm --default "Proceed with defaults?"; then
    echo "Using defaults..."
fi
```

### Execução Condicional

```bash
# Padrão simples
invowk tui confirm "Run tests?" && npm test

# Negação
invowk tui confirm "Skip build?" || npm run build
```

### Operações Perigosas

```bash
# Confirmação dupla para ações perigosas
if invowk tui confirm "Delete production database?"; then
    echo "This cannot be undone!" | invowk tui style --foreground "#FF0000" --bold
    if invowk tui confirm --affirmative "YES, DELETE IT" --negative "No, abort" "Type to confirm:"; then
        ./scripts/delete-production-db.sh
    fi
fi
```

### Em Scripts

```cue
{
    name: "clean"
    description: "Clean build artifacts"
    implementations: [{
        script: """
            echo "This will delete:"
            echo "  - ./build/"
            echo "  - ./dist/"
            echo "  - ./node_modules/"
            
            if invowk tui confirm "Proceed with cleanup?"; then
                rm -rf build/ dist/ node_modules/
                echo "Cleaned!" | invowk tui style --foreground "#00FF00"
            else
                echo "Cancelled."
            fi
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Padrões Combinados

### Seleção com Confirmação

```bash
ACTION=$(invowk tui choose --title "Select action" \
    "Deploy to staging" \
    "Deploy to production" \
    "Rollback" \
    "Cancel")

case "$ACTION" in
    "Cancel")
        exit 0
        ;;
    "Deploy to production")
        if ! invowk tui confirm --affirmative "Yes, deploy" "Deploy to PRODUCTION?"; then
            echo "Aborted."
            exit 1
        fi
        ;;
esac

echo "Executing: $ACTION"
```

### Wizard Multi-Etapa

```bash
# Etapa 1: Escolher ação
ACTION=$(invowk tui choose --title "What would you like to do?" \
    "Create new project" \
    "Import existing" \
    "Exit")

if [ "$ACTION" = "Exit" ]; then
    exit 0
fi

# Etapa 2: Obter detalhes
NAME=$(invowk tui input --title "Project name:")

# Etapa 3: Confirmar
echo "Action: $ACTION"
echo "Name: $NAME"

if invowk tui confirm "Create project?"; then
    # prosseguir
fi
```

## Próximos Passos

- [Filter e File](./filter-and-file) - Busca e seleção de arquivo
- [Visão Geral](./overview) - Todos os componentes TUI
