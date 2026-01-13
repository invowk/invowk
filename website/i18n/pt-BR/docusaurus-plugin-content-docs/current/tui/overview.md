---
sidebar_position: 1
---

# Visão Geral de Componentes TUI

Invowk inclui um conjunto de componentes de interface de terminal interativos inspirados no [gum](https://github.com/charmbracelet/gum). Use-os em seus scripts para criar prompts interativos, seleções e saída estilizada.

## Componentes Disponíveis

| Componente | Descrição | Caso de Uso |
|------------|-----------|-------------|
| [input](./input-and-write#input) | Entrada de texto de linha única | Nomes, caminhos, valores simples |
| [write](./input-and-write#write) | Editor de texto multilinha | Descrições, mensagens de commit |
| [choose](./choose-and-confirm#choose) | Selecionar de opções | Menus, escolhas |
| [confirm](./choose-and-confirm#confirm) | Prompt sim/não | Confirmações |
| [filter](./filter-and-file#filter) | Filtrar lista fuzzy | Buscar entre opções |
| [file](./filter-and-file#file) | Seletor de arquivo | Selecionar arquivos/diretórios |
| [table](./table-and-spin#table) | Exibir dados tabulares | CSV, tabelas de dados |
| [spin](./table-and-spin#spin) | Spinner com comando | Tarefas de longa duração |
| [format](./format-and-style#format) | Formatar texto (markdown, código) | Renderizar conteúdo |
| [style](./format-and-style#style) | Estilizar texto (cores, negrito) | Decorar saída |

## Exemplos Rápidos

```bash
# Obter entrada do usuário
NAME=$(invowk tui input --title "What's your name?")

# Escolher de opções
COLOR=$(invowk tui choose --title "Pick a color" red green blue)

# Confirmar ação
if invowk tui confirm "Continue?"; then
    echo "Proceeding..."
fi

# Mostrar spinner durante tarefa longa
invowk tui spin --title "Installing..." -- npm install

# Estilizar saída
echo "Success!" | invowk tui style --foreground "#00FF00" --bold
```

## Usando em Invkfiles

Componentes TUI funcionam muito bem dentro de scripts de comando:

```cue
{
    name: "setup"
    description: "Interactive project setup"
    implementations: [{
        script: """
            #!/bin/bash
            
            # Coletar informações
            NAME=$(invowk tui input --title "Project name:")
            TYPE=$(invowk tui choose --title "Type:" cli library api)
            
            # Confirmar
            echo "Creating $TYPE project: $NAME"
            if ! invowk tui confirm "Proceed?"; then
                echo "Cancelled."
                exit 0
            fi
            
            # Executar com spinner
            invowk tui spin --title "Creating project..." -- mkdir -p "$NAME"
            
            # Mensagem de sucesso
            echo "Project created!" | invowk tui style --foreground "#00FF00" --bold
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Padrões Comuns

### Input com Validação

```bash
while true; do
    EMAIL=$(invowk tui input --title "Email address:")
    if echo "$EMAIL" | grep -qE '^[^@]+@[^@]+\.[^@]+$'; then
        break
    fi
    echo "Invalid email format" | invowk tui style --foreground "#FF0000"
done
```

### Sistema de Menu

```bash
ACTION=$(invowk tui choose --title "What would you like to do?" \
    "Build project" \
    "Run tests" \
    "Deploy" \
    "Exit")

case "$ACTION" in
    "Build project") make build ;;
    "Run tests") make test ;;
    "Deploy") make deploy ;;
    "Exit") exit 0 ;;
esac
```

### Feedback de Progresso

```bash
echo "Step 1: Installing dependencies..."
invowk tui spin --title "Installing..." -- npm install

echo "Step 2: Building..."
invowk tui spin --title "Building..." -- npm run build

echo "Done!" | invowk tui style --foreground "#00FF00" --bold
```

### Cabeçalhos Estilizados

```bash
invowk tui style --bold --foreground "#00BFFF" "=== Project Setup ==="
echo ""
# ... resto do script
```

## Piping e Captura

A maioria dos componentes funciona com pipes:

```bash
# Pipe para filter
ls | invowk tui filter --title "Select file"

# Capturar saída
SELECTED=$(invowk tui choose opt1 opt2 opt3)
echo "You selected: $SELECTED"

# Pipe para estilização
echo "Important message" | invowk tui style --bold
```

## Códigos de Saída

Componentes usam códigos de saída para comunicar:

| Componente | Exit 0 | Exit 1 |
|------------|--------|--------|
| confirm | Usuário disse sim | Usuário disse não |
| input | Valor inserido | Cancelado |
| choose | Opção selecionada | Cancelado |
| filter | Opção selecionada | Cancelado |

Use em condicionais:

```bash
if invowk tui confirm "Delete files?"; then
    rm -rf ./temp
fi
```

## Próximos Passos

Explore cada componente em detalhes:

- [Input e Write](./input-and-write) - Entrada de texto
- [Choose e Confirm](./choose-and-confirm) - Seleção e confirmação
- [Filter e File](./filter-and-file) - Busca e seleção de arquivo
- [Table e Spin](./table-and-spin) - Exibição de dados e spinners
- [Format e Style](./format-and-style) - Formatação e estilização de texto
