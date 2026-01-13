---
sidebar_position: 6
---

# Format e Style

Componentes de formatação e estilização de texto para saída de terminal bonita.

## Format

Formatar e renderizar texto como markdown, código ou emoji.

### Uso Básico

```bash
echo "# Hello World" | invowk tui format --type markdown
```

### Opções

| Opção | Descrição |
|-------|-----------|
| `--type` | Tipo de formato: `markdown`, `code`, `emoji` |
| `--language` | Linguagem para highlight de código |

### Markdown

Renderizar markdown com cores e formatação:

```bash
# De stdin
echo "# Heading\n\nSome **bold** and *italic* text" | invowk tui format --type markdown

# De arquivo
cat README.md | invowk tui format --type markdown
```

### Highlight de Código

Highlight de sintaxe de código:

```bash
# Especificar linguagem
cat main.go | invowk tui format --type code --language go

# Python
cat script.py | invowk tui format --type code --language python

# JavaScript
cat app.js | invowk tui format --type code --language javascript
```

### Conversão de Emoji

Converter shortcodes de emoji para emojis reais:

```bash
echo "Hello :wave: World :smile:" | invowk tui format --type emoji
# Saída: Hello 👋 World 😄
```

### Exemplos do Mundo Real

#### Exibir README

```bash
cat README.md | invowk tui format --type markdown
```

#### Mostrar Diff de Código

```bash
git diff | invowk tui format --type code --language diff
```

#### Mensagem de Boas-vindas

```bash
echo ":rocket: Welcome to MyApp :sparkles:" | invowk tui format --type emoji
```

---

## Style

Aplicar estilização de terminal ao texto.

### Uso Básico

```bash
invowk tui style --foreground "#FF0000" "Red text"
```

### Opções

| Opção | Descrição |
|-------|-----------|
| `--foreground` | Cor do texto (hex ou nome) |
| `--background` | Cor de fundo |
| `--bold` | Texto em negrito |
| `--italic` | Texto em itálico |
| `--underline` | Texto sublinhado |
| `--strikethrough` | Texto tachado |
| `--faint` | Texto esmaecido |
| `--border` | Estilo de borda |
| `--padding-*` | Padding (left, right, top, bottom) |
| `--margin-*` | Margin (left, right, top, bottom) |
| `--width` | Largura fixa |
| `--height` | Altura fixa |
| `--align` | Alinhamento de texto: `left`, `center`, `right` |

### Cores

Use cores hex ou nomes:

```bash
# Cores hex
invowk tui style --foreground "#FF0000" "Red"
invowk tui style --foreground "#00FF00" "Green"
invowk tui style --foreground "#0000FF" "Blue"

# Com fundo
invowk tui style --foreground "#FFFFFF" --background "#FF0000" "White on Red"
```

### Decorações de Texto

```bash
# Negrito
invowk tui style --bold "Bold text"

# Itálico
invowk tui style --italic "Italic text"

# Combinado
invowk tui style --bold --italic --underline "All decorations"

# Esmaecido
invowk tui style --faint "Subtle text"
```

### Piping

Estilizar texto de stdin:

```bash
echo "Important message" | invowk tui style --bold --foreground "#FF0000"
```

### Bordas

Adicionar bordas ao redor do texto:

```bash
# Borda simples
invowk tui style --border normal "Boxed text"

# Borda arredondada
invowk tui style --border rounded "Rounded box"

# Borda dupla
invowk tui style --border double "Double border"

# Com padding
invowk tui style --border rounded --padding-left 2 --padding-right 2 "Padded"
```

Estilos de borda: `normal`, `rounded`, `double`, `thick`, `hidden`

### Layout

```bash
# Largura fixa
invowk tui style --width 40 --align center "Centered"

# Com margins
invowk tui style --margin-left 4 "Indented text"

# Caixa com todas as opções
invowk tui style \
    --border rounded \
    --foreground "#FFFFFF" \
    --background "#333333" \
    --padding-left 2 \
    --padding-right 2 \
    --width 50 \
    --align center \
    "Styled Box"
```

### Exemplos do Mundo Real

#### Mensagens de Sucesso/Erro

```bash
# Sucesso
echo "Build successful!" | invowk tui style --foreground "#00FF00" --bold

# Erro
echo "Build failed!" | invowk tui style --foreground "#FF0000" --bold

# Aviso
echo "Deprecated feature" | invowk tui style --foreground "#FFA500" --italic
```

#### Cabeçalhos e Seções

```bash
# Cabeçalho principal
invowk tui style --bold --foreground "#00BFFF" "=== Project Setup ==="
echo ""

# Subcabeçalho
invowk tui style --foreground "#888888" "Configuration Options:"
```

#### Caixas de Status

```bash
# Caixa de info
invowk tui style \
    --border rounded \
    --foreground "#FFFFFF" \
    --background "#0066CC" \
    --padding-left 1 \
    --padding-right 1 \
    "ℹ️  Info: Server is running on port 3000"

# Caixa de aviso
invowk tui style \
    --border rounded \
    --foreground "#000000" \
    --background "#FFCC00" \
    --padding-left 1 \
    --padding-right 1 \
    "⚠️  Warning: API key will expire soon"
```

### Em Scripts

```cue
{
    name: "status"
    description: "Show system status"
    implementations: [{
        script: """
            invowk tui style --bold --foreground "#00BFFF" "System Status"
            echo ""
            
            # Verificar serviços
            if systemctl is-active nginx > /dev/null 2>&1; then
                echo "nginx: " | tr -d '\n'
                invowk tui style --foreground "#00FF00" "running"
            else
                echo "nginx: " | tr -d '\n'
                invowk tui style --foreground "#FF0000" "stopped"
            fi
            
            if systemctl is-active postgresql > /dev/null 2>&1; then
                echo "postgres: " | tr -d '\n'
                invowk tui style --foreground "#00FF00" "running"
            else
                echo "postgres: " | tr -d '\n'
                invowk tui style --foreground "#FF0000" "stopped"
            fi
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Padrões Combinados

### Saída Formatada

```bash
# Cabeçalho
invowk tui style --bold --foreground "#FFD700" "📦 Package Info"
echo ""

# Renderizar descrição do pacote como markdown
cat package.md | invowk tui format --type markdown
```

### Interativo com Saída Estilizada

```bash
NAME=$(invowk tui input --title "Project name:")

if invowk tui confirm "Create $NAME?"; then
    invowk tui spin --title "Creating..." -- mkdir -p "$NAME"
    echo "" 
    invowk tui style --foreground "#00FF00" --bold "✓ Created $NAME successfully!"
else
    invowk tui style --foreground "#FF0000" "✗ Cancelled"
fi
```

## Próximos Passos

- [Visão Geral](./overview) - Todos os componentes TUI
- [Input e Write](./input-and-write) - Entrada de texto
