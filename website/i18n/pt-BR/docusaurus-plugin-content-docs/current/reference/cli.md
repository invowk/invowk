---
sidebar_position: 1
---

# Referência CLI

Referência completa para todos os comandos e flags de linha de comando do Invowk.

## Flags Globais

Estas flags estão disponíveis para todos os comandos:

| Flag | Curta | Descrição |
|------|-------|-----------|
| `--config` | | Caminho para arquivo de configuração (padrão: localização específica do SO) |
| `--help` | `-h` | Mostrar ajuda para o comando |
| `--verbose` | `-v` | Habilitar saída verbose |
| `--version` | | Mostrar informação de versão |

## Comandos

### invowk

O comando raiz. Executar `invowk` sem argumentos mostra a mensagem de ajuda.

```bash
invowk [flags]
invowk [command]
```

---

### invowk cmd

Executar comandos definidos em invkfiles.

```bash
invowk cmd [flags]
invowk cmd [command-name] [flags] [-- args...]
```

**Flags:**

| Flag | Curta | Descrição |
|------|-------|-----------|
| `--list` | `-l` | Listar todos os comandos disponíveis |
| `--runtime` | `-r` | Sobrescrever o runtime (deve ser permitido pelo comando) |

**Exemplos:**

```bash
# Listar todos os comandos disponíveis
invowk cmd --list
invowk cmd -l

# Executar um comando
invowk cmd build

# Executar um comando aninhado
invowk cmd test.unit

# Executar com um runtime específico
invowk cmd build --runtime container

# Executar com argumentos
invowk cmd greet -- "World"

# Executar com flags
invowk cmd deploy --env production
```

**Descoberta de Comandos:**

Comandos são descobertos de (em ordem de prioridade):
1. Diretório atual
2. `~/.invowk/cmds/`
3. Caminhos configurados em `search_paths`

---

### invowk init

Criar um novo invkfile no diretório atual.

```bash
invowk init [flags]
```

**Flags:**

| Flag | Curta | Descrição |
|------|-------|-----------|
| `--force` | `-f` | Sobrescrever invkfile existente |
| `--template` | `-t` | Template a usar: `default`, `minimal`, `full` |

**Templates:**

- `default` - Um template equilibrado com alguns comandos de exemplo
- `minimal` - Estrutura mínima de invkfile
- `full` - Template abrangente mostrando todas as funcionalidades

**Exemplos:**

```bash
# Criar um invkfile padrão
invowk init

# Criar um invkfile minimal
invowk init --template minimal

# Sobrescrever invkfile existente
invowk init --force
```

---

### invowk config

Gerenciar configuração do Invowk.

```bash
invowk config [command]
```

**Subcomandos:**

#### invowk config show

Exibir configuração atual em formato legível.

```bash
invowk config show
```

#### invowk config dump

Exibir configuração raw como CUE.

```bash
invowk config dump
```

#### invowk config path

Mostrar o caminho do arquivo de configuração.

```bash
invowk config path
```

#### invowk config init

Criar um arquivo de configuração padrão.

```bash
invowk config init
```

#### invowk config set

Definir um valor de configuração.

```bash
invowk config set <key> <value>
```

**Exemplos:**

```bash
# Definir container engine
invowk config set container_engine podman

# Definir runtime padrão
invowk config set default_runtime virtual

# Definir valor aninhado
invowk config set ui.color_scheme dark
```

---

### invowk pack

Gerenciar invowk packs (pastas de comando autocontidas).

```bash
invowk pack [command]
```

**Subcomandos:**

#### invowk pack create

Criar um novo invowk pack.

```bash
invowk pack create <name> [flags]
```

**Flags:**

| Flag | Curta | Descrição |
|------|-------|-----------|
| `--output` | `-o` | Diretório de saída (padrão: diretório atual) |

**Exemplos:**

```bash
# Criar um pack com nomenclatura RDNS
invowk pack create com.example.mytools
```

#### invowk pack validate

Validar um invowk pack.

```bash
invowk pack validate <path> [flags]
```

**Flags:**

| Flag | Curta | Descrição |
|------|-------|-----------|
| `--deep` | `-d` | Executar validação profunda (verifica arquivos de script, etc.) |

**Exemplos:**

```bash
# Validação básica
invowk pack validate ./mypack.invkpack

# Validação profunda
invowk pack validate ./mypack.invkpack --deep
```

#### invowk pack list

Listar todos os packs descobertos.

```bash
invowk pack list
```

#### invowk pack archive

Criar um arquivo ZIP de um pack.

```bash
invowk pack archive <path> [flags]
```

**Flags:**

| Flag | Curta | Descrição |
|------|-------|-----------|
| `--output` | `-o` | Caminho do arquivo de saída |

#### invowk pack import

Importar um pack de um arquivo ZIP ou URL.

```bash
invowk pack import <source> [flags]
```

**Flags:**

| Flag | Curta | Descrição |
|------|-------|-----------|
| `--output` | `-o` | Diretório de saída |

---

### invowk tui

Componentes de interface de terminal interativos para shell scripts.

```bash
invowk tui [command] [flags]
```

:::tip
Componentes TUI funcionam muito bem em scripts de invkfile! Eles fornecem prompts interativos, spinners, seletores de arquivo e mais.
:::

**Subcomandos:**

#### invowk tui input

Solicitar entrada de texto de linha única.

```bash
invowk tui input [flags]
```

**Flags:**

| Flag | Descrição |
|------|-----------|
| `--title` | Título para o prompt de entrada |
| `--placeholder` | Texto de placeholder |
| `--default` | Valor padrão |
| `--password` | Ocultar entrada (para senhas) |

#### invowk tui write

Editor de texto multilinha.

```bash
invowk tui write [flags]
```

**Flags:**

| Flag | Descrição |
|------|-----------|
| `--title` | Título para o editor |
| `--placeholder` | Texto de placeholder |
| `--value` | Valor inicial |

#### invowk tui choose

Selecionar de uma lista de opções.

```bash
invowk tui choose [options...] [flags]
```

**Flags:**

| Flag | Descrição |
|------|-----------|
| `--title` | Título para a seleção |
| `--limit` | Número máximo de seleções (padrão: 1) |

#### invowk tui confirm

Solicitar confirmação sim/não.

```bash
invowk tui confirm [prompt] [flags]
```

**Flags:**

| Flag | Descrição |
|------|-----------|
| `--default` | Valor padrão (true/false) |
| `--affirmative` | Texto para opção "sim" |
| `--negative` | Texto para opção "não" |

#### invowk tui filter

Filtrar fuzzy uma lista de opções.

```bash
invowk tui filter [options...] [flags]
```

Opções também podem ser fornecidas via stdin.

#### invowk tui file

Seletor de arquivo.

```bash
invowk tui file [path] [flags]
```

**Flags:**

| Flag | Descrição |
|------|-----------|
| `--directory` | Mostrar apenas diretórios |
| `--file` | Mostrar apenas arquivos |
| `--hidden` | Mostrar arquivos ocultos |

#### invowk tui table

Exibir e selecionar de uma tabela.

```bash
invowk tui table [flags]
```

Lê dados CSV ou TSV de stdin.

**Flags:**

| Flag | Descrição |
|------|-----------|
| `--separator` | Separador de coluna (padrão: `,`) |
| `--header` | Primeira linha é cabeçalho |

#### invowk tui spin

Mostrar um spinner enquanto executa um comando.

```bash
invowk tui spin [flags] -- command [args...]
```

**Flags:**

| Flag | Descrição |
|------|-----------|
| `--title` | Título do spinner |
| `--spinner` | Estilo do spinner |

#### invowk tui pager

Rolar através de conteúdo.

```bash
invowk tui pager [file] [flags]
```

Lê de arquivo ou stdin.

#### invowk tui format

Formatar texto com markdown, código ou emoji.

```bash
invowk tui format [text...] [flags]
```

**Flags:**

| Flag | Descrição |
|------|-----------|
| `--type` | Tipo de formato: `markdown`, `code`, `emoji` |
| `--language` | Linguagem para highlight de código |

#### invowk tui style

Aplicar estilos ao texto.

```bash
invowk tui style [text...] [flags]
```

**Flags:**

| Flag | Descrição |
|------|-----------|
| `--foreground` | Cor do texto (hex ou nome) |
| `--background` | Cor de fundo |
| `--bold` | Texto em negrito |
| `--italic` | Texto em itálico |
| `--underline` | Texto sublinhado |

---

### invowk completion

Gerar scripts de completion de shell.

```bash
invowk completion [shell]
```

**Shells:** `bash`, `zsh`, `fish`, `powershell`

**Exemplos:**

```bash
# Bash
eval "$(invowk completion bash)"

# Zsh
eval "$(invowk completion zsh)"

# Fish
invowk completion fish > ~/.config/fish/completions/invowk.fish

# PowerShell
invowk completion powershell | Out-String | Invoke-Expression
```

---

### invowk help

Mostrar ajuda para qualquer comando.

```bash
invowk help [command]
```

**Exemplos:**

```bash
invowk help
invowk help cmd
invowk help config set
```

## Códigos de Saída

| Código | Significado |
|--------|-------------|
| 0 | Sucesso |
| 1 | Erro geral |
| 2 | Comando não encontrado |
| 3 | Validação de dependência falhou |
| 4 | Erro de configuração |
| 5 | Erro de runtime |

## Variáveis de Ambiente

| Variável | Descrição |
|----------|-----------|
| `INVOWK_CONFIG` | Sobrescrever caminho do arquivo de configuração |
| `INVOWK_VERBOSE` | Habilitar saída verbose (`1` ou `true`) |
| `INVOWK_CONTAINER_ENGINE` | Sobrescrever container engine |
| `NO_COLOR` | Desabilitar saída colorida |
| `FORCE_COLOR` | Forçar saída colorida |
