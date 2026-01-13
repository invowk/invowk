---
sidebar_position: 2
---

# Input e Write

Componentes de entrada de texto para coletar informações do usuário.

## Input

Entrada de texto de linha única com validação opcional.

### Uso Básico

```bash
invowk tui input --title "What is your name?"
```

### Opções

| Opção | Descrição |
|-------|-----------|
| `--title` | Texto do prompt |
| `--placeholder` | Texto de placeholder |
| `--value` | Valor inicial |
| `--password` | Ocultar entrada (para secrets) |
| `--char-limit` | Máximo de caracteres |

### Exemplos

```bash
# Com placeholder
invowk tui input --title "Email" --placeholder "user@example.com"

# Entrada de senha (oculta)
invowk tui input --title "Password" --password

# Com valor inicial
invowk tui input --title "Name" --value "John Doe"

# Comprimento limitado
invowk tui input --title "Username" --char-limit 20
```

### Capturando Saída

```bash
NAME=$(invowk tui input --title "Enter your name:")
echo "Hello, $NAME!"
```

### Em Scripts

```cue
{
    name: "create-user"
    implementations: [{
        script: """
            USERNAME=$(invowk tui input --title "Username:" --char-limit 20)
            EMAIL=$(invowk tui input --title "Email:" --placeholder "user@example.com")
            PASSWORD=$(invowk tui input --title "Password:" --password)
            
            echo "Creating user: $USERNAME ($EMAIL)"
            ./scripts/create-user.sh "$USERNAME" "$EMAIL" "$PASSWORD"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

---

## Write

Editor de texto multilinha para entradas mais longas como descrições ou mensagens de commit.

### Uso Básico

```bash
invowk tui write --title "Enter description"
```

### Opções

| Opção | Descrição |
|-------|-----------|
| `--title` | Título do editor |
| `--placeholder` | Texto de placeholder |
| `--value` | Conteúdo inicial |
| `--show-line-numbers` | Exibir números de linha |
| `--char-limit` | Máximo de caracteres |

### Exemplos

```bash
# Editor básico
invowk tui write --title "Description:"

# Com números de linha
invowk tui write --title "Code:" --show-line-numbers

# Com conteúdo inicial
invowk tui write --title "Edit message:" --value "Initial text here"
```

### Casos de Uso

#### Mensagem de Git Commit

```bash
MESSAGE=$(invowk tui write --title "Commit message:")
git commit -m "$MESSAGE"
```

#### Configuração Multilinha

```bash
CONFIG=$(invowk tui write --title "Enter YAML config:" --show-line-numbers)
echo "$CONFIG" > config.yaml
```

#### Release Notes

```bash
NOTES=$(invowk tui write --title "Release notes:")
gh release create v1.0.0 --notes "$NOTES"
```

### Em Scripts

```cue
{
    name: "commit"
    description: "Interactive commit with editor"
    implementations: [{
        script: """
            # Mostrar mudanças staged
            git diff --cached --stat
            
            # Obter mensagem de commit
            MESSAGE=$(invowk tui write --title "Commit message:")
            
            if [ -z "$MESSAGE" ]; then
                echo "Commit cancelled (empty message)"
                exit 1
            fi
            
            git commit -m "$MESSAGE"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Dicas

### Tratando Entrada Vazia

```bash
NAME=$(invowk tui input --title "Name:")
if [ -z "$NAME" ]; then
    echo "Name is required!"
    exit 1
fi
```

### Loop de Validação

```bash
while true; do
    EMAIL=$(invowk tui input --title "Email:")
    if echo "$EMAIL" | grep -qE '^[^@]+@[^@]+\.[^@]+$'; then
        break
    fi
    echo "Invalid email format, try again."
done
```

### Valores Padrão

```bash
# Usar padrão do shell se vazio
NAME=$(invowk tui input --title "Name:" --placeholder "Anonymous")
NAME="${NAME:-Anonymous}"
```

## Próximos Passos

- [Choose e Confirm](./choose-and-confirm) - Componentes de seleção
- [Visão Geral](./overview) - Todos os componentes TUI
