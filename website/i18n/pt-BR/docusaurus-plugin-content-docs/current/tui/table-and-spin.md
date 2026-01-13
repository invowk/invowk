---
sidebar_position: 5
---

# Table e Spin

Componentes de exibição de dados e indicador de carregamento.

## Table

Exibir e opcionalmente selecionar de dados tabulares.

### Uso Básico

```bash
# De um arquivo CSV
invowk tui table --file data.csv

# De stdin com separador
echo -e "name|age|city\nAlice|30|NYC\nBob|25|LA" | invowk tui table --separator "|"
```

### Opções

| Opção | Descrição |
|-------|-----------|
| `--file` | Arquivo CSV para exibir |
| `--separator` | Separador de coluna (padrão: `,`) |
| `--selectable` | Permitir seleção de linha |
| `--height` | Altura da tabela |

### Exemplos

```bash
# Exibir CSV
invowk tui table --file users.csv

# Separador customizado (TSV)
invowk tui table --file data.tsv --separator $'\t'

# Separado por pipe
cat data.txt | invowk tui table --separator "|"
```

### Tabelas Selecionáveis

```bash
# Selecionar uma linha
SELECTED=$(invowk tui table --file servers.csv --selectable)
echo "Selected: $SELECTED"
```

A linha selecionada é retornada como a linha CSV completa.

### Exemplos do Mundo Real

#### Exibir Lista de Servidores

```bash
# servers.csv:
# name,ip,status
# web-1,10.0.0.1,running
# web-2,10.0.0.2,running
# db-1,10.0.0.3,stopped

invowk tui table --file servers.csv
```

#### Selecionar e SSH

```bash
# Selecionar um servidor
SERVER=$(cat servers.csv | invowk tui table --selectable | cut -d',' -f2)
ssh "user@$SERVER"
```

#### Lista de Processos

```bash
ps aux --no-headers | awk '{print $1","$2","$11}' | \
    (echo "USER,PID,COMMAND"; cat) | \
    invowk tui table --selectable
```

---

## Spin

Mostrar um spinner enquanto executa um comando longo.

### Uso Básico

```bash
invowk tui spin --title "Installing..." -- npm install
```

### Opções

| Opção | Descrição |
|-------|-----------|
| `--title` | Título/mensagem do spinner |
| `--type` | Tipo de animação do spinner |
| `--show-output` | Mostrar saída do comando |

### Tipos de Spinner

Animações de spinner disponíveis:

- `line` - Linha simples
- `dot` - Pontos
- `minidot` - Pontos pequenos
- `jump` - Pontos pulando
- `pulse` - Ponto pulsante
- `points` - Pontos
- `globe` - Globo girando
- `moon` - Fases da lua
- `monkey` - Macaco
- `meter` - Medidor de progresso
- `hamburger` - Menu hamburger
- `ellipsis` - Reticências

```bash
invowk tui spin --type globe --title "Downloading..." -- curl -O https://example.com/file
invowk tui spin --type moon --title "Building..." -- make build
invowk tui spin --type pulse --title "Testing..." -- npm test
```

### Exemplos

```bash
# Spinner básico
invowk tui spin --title "Building..." -- go build ./...

# Com tipo específico
invowk tui spin --type dot --title "Installing dependencies..." -- npm install

# Tarefa de longa duração
invowk tui spin --title "Compiling assets..." -- webpack --mode production
```

### Spinners Encadeados

```bash
echo "Step 1/3: Dependencies"
invowk tui spin --title "Installing..." -- npm install

echo "Step 2/3: Build"
invowk tui spin --title "Building..." -- npm run build

echo "Step 3/3: Tests"
invowk tui spin --title "Testing..." -- npm test

echo "Done!" | invowk tui style --foreground "#00FF00" --bold
```

### Tratamento de Código de Saída

O comando spin retorna o código de saída do comando encapsulado:

```bash
if invowk tui spin --title "Testing..." -- npm test; then
    echo "Tests passed!"
else
    echo "Tests failed!"
    exit 1
fi
```

### Em Scripts

```cue
{
    name: "deploy"
    description: "Deploy with progress indication"
    implementations: [{
        script: """
            echo "Deploying application..."
            
            invowk tui spin --title "Building Docker image..." -- \
                docker build -t myapp .
            
            invowk tui spin --title "Pushing to registry..." -- \
                docker push myapp
            
            invowk tui spin --title "Updating Kubernetes..." -- \
                kubectl rollout restart deployment/myapp
            
            invowk tui spin --title "Waiting for rollout..." -- \
                kubectl rollout status deployment/myapp
            
            echo "Deployment complete!" | invowk tui style --foreground "#00FF00" --bold
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Padrões Combinados

### Selecionar e Executar com Spinner

```bash
# Escolher o que construir
PROJECT=$(invowk tui choose --title "Build which project?" api web worker)

# Construir com spinner
invowk tui spin --title "Building $PROJECT..." -- make "build-$PROJECT"
```

### Seleção de Tabela com Ação de Spinner

```bash
# Selecionar servidor
SERVER=$(invowk tui table --file servers.csv --selectable | cut -d',' -f1)

# Reiniciar com spinner
invowk tui spin --title "Restarting $SERVER..." -- ssh "$SERVER" "systemctl restart myapp"
```

## Próximos Passos

- [Format e Style](./format-and-style) - Formatação de texto
- [Visão Geral](./overview) - Todos os componentes TUI
