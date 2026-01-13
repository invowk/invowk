---
sidebar_position: 4
---

# Filter e File

Componentes de busca e seleção de arquivos.

## Filter

Filtrar fuzzy através de uma lista de opções.

### Uso Básico

```bash
# De argumentos
invowk tui filter "apple" "banana" "cherry" "date"

# De stdin
ls | invowk tui filter
```

### Opções

| Opção | Descrição |
|-------|-----------|
| `--title` | Prompt do filtro |
| `--placeholder` | Placeholder de busca |
| `--limit` | Máximo de seleções (padrão: 1) |
| `--no-limit` | Seleções ilimitadas |
| `--indicator` | Indicador de seleção |

### Exemplos

```bash
# Filtrar arquivos
FILE=$(ls | invowk tui filter --title "Select file")

# Filtro multi-seleção
FILES=$(ls *.go | invowk tui filter --no-limit --title "Select Go files")

# Com placeholder
ITEM=$(invowk tui filter --placeholder "Type to search..." opt1 opt2 opt3)
```

### Exemplos do Mundo Real

#### Selecionar Branch Git

```bash
BRANCH=$(git branch --list | tr -d '* ' | invowk tui filter --title "Checkout branch")
git checkout "$BRANCH"
```

#### Selecionar Container Docker

```bash
CONTAINER=$(docker ps --format "{{.Names}}" | invowk tui filter --title "Select container")
docker logs -f "$CONTAINER"
```

#### Selecionar Processo para Matar

```bash
PID=$(ps aux | invowk tui filter --title "Select process" | awk '{print $2}')
if [ -n "$PID" ]; then
    kill "$PID"
fi
```

#### Filtrar Comandos

```bash
CMD=$(invowk cmd --list 2>/dev/null | grep "^  " | invowk tui filter --title "Run command")
# Extrair nome do comando e executá-lo
```

---

## File

Seletor de arquivos e diretórios com navegação.

### Uso Básico

```bash
# Selecionar qualquer arquivo do diretório atual
invowk tui file

# Iniciar em diretório específico
invowk tui file /home/user/documents
```

### Opções

| Opção | Descrição |
|-------|-----------|
| `--directory` | Mostrar apenas diretórios |
| `--hidden` | Mostrar arquivos ocultos |
| `--allowed` | Filtrar por extensões |
| `--cursor` | Caractere do cursor |
| `--height` | Altura do seletor |

### Exemplos

```bash
# Selecionar um arquivo
FILE=$(invowk tui file)
echo "Selected: $FILE"

# Apenas diretórios
DIR=$(invowk tui file --directory)

# Mostrar arquivos ocultos
FILE=$(invowk tui file --hidden)

# Filtrar por extensão
FILE=$(invowk tui file --allowed ".go,.md,.txt")

# Múltiplas extensões
CONFIG=$(invowk tui file --allowed ".yaml,.yml,.json,.toml")
```

### Navegação

No seletor de arquivos:
- **↑/↓**: Navegar
- **Enter**: Selecionar arquivo ou entrar no diretório
- **Backspace**: Ir para diretório pai
- **Esc/Ctrl+C**: Cancelar

### Exemplos do Mundo Real

#### Selecionar Arquivo de Configuração

```bash
CONFIG=$(invowk tui file --allowed ".yaml,.yml,.json" /etc/myapp)
echo "Using config: $CONFIG"
./myapp --config "$CONFIG"
```

#### Selecionar Diretório de Projeto

```bash
PROJECT=$(invowk tui file --directory ~/projects)
cd "$PROJECT"
code .
```

#### Selecionar Script para Executar

```bash
SCRIPT=$(invowk tui file --allowed ".sh,.bash" ./scripts)
if [ -n "$SCRIPT" ]; then
    chmod +x "$SCRIPT"
    "$SCRIPT"
fi
```

#### Selecionar Arquivo de Log

```bash
LOG=$(invowk tui file --allowed ".log" /var/log)
less "$LOG"
```

### Em Scripts

```cue
{
    name: "edit-config"
    description: "Edit a configuration file"
    implementations: [{
        script: """
            CONFIG=$(invowk tui file --allowed ".yaml,.yml,.json,.toml" ./config)
            
            if [ -z "$CONFIG" ]; then
                echo "No file selected."
                exit 0
            fi
            
            # Abrir no editor padrão
            ${EDITOR:-vim} "$CONFIG"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Padrões Combinados

### Buscar e Editar

```bash
# Encontrar arquivo por conteúdo, depois escolher dos resultados
FILE=$(grep -l "TODO" *.go 2>/dev/null | invowk tui filter --title "Select file with TODOs")
if [ -n "$FILE" ]; then
    vim "$FILE"
fi
```

### Diretório e Depois Arquivo

```bash
# Primeiro escolher diretório
DIR=$(invowk tui file --directory ~/projects)

# Depois escolher arquivo naquele diretório
FILE=$(invowk tui file "$DIR" --allowed ".go")

echo "Selected: $FILE"
```

## Próximos Passos

- [Table e Spin](./table-and-spin) - Exibição de dados e spinners
- [Visão Geral](./overview) - Todos os componentes TUI
