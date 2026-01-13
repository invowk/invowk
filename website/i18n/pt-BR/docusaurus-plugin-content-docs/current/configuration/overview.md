---
sidebar_position: 1
---

# Visão Geral de Configuração

Invowk usa um arquivo de configuração baseado em CUE para customizar seu comportamento. É aqui que você define suas preferências para container engines, caminhos de busca, padrões de runtime e mais.

## Localização do Arquivo de Configuração

O arquivo de configuração fica no diretório de configuração específico do seu SO:

| Plataforma | Localização |
|------------|-------------|
| Linux      | `~/.config/invowk/config.cue` |
| macOS      | `~/Library/Application Support/invowk/config.cue` |
| Windows    | `%APPDATA%\invowk\config.cue` |

Você também pode especificar um caminho customizado para o arquivo de configuração usando a flag `--config`:

```bash
invowk --config /path/to/my/config.cue cmd list
```

## Criando um Arquivo de Configuração

A forma mais fácil de criar um arquivo de configuração é usar o comando `config init`:

```bash
invowk config init
```

Isso cria um arquivo de configuração padrão com valores sensatos. Se um arquivo de configuração já existir, ele não será sobrescrito (segurança em primeiro lugar!).

## Visualizando Sua Configuração

Existem várias formas de inspecionar sua configuração atual:

### Mostrar Configuração Legível

```bash
invowk config show
```

Isso exibe sua configuração em um formato amigável e legível.

### Mostrar CUE Raw

```bash
invowk config dump
```

Isso exibe a configuração CUE raw, útil para debug ou copiar para outra máquina.

### Encontrar o Arquivo de Configuração

```bash
invowk config path
```

Isso imprime o caminho para seu arquivo de configuração. Prático quando você quer editá-lo diretamente.

## Definindo Valores de Configuração

Você pode modificar valores de configuração pela linha de comando:

```bash
# Definir o container engine
invowk config set container_engine podman

# Definir o runtime padrão
invowk config set default_runtime virtual

# Definir o esquema de cores
invowk config set ui.color_scheme dark
```

Ou simplesmente abra o arquivo de configuração no seu editor favorito:

```bash
# Linux/macOS
$EDITOR $(invowk config path)

# Windows PowerShell
notepad (invowk config path)
```

## Exemplo de Configuração

Aqui está como um arquivo de configuração típico se parece:

```cue
// ~/.config/invowk/config.cue

// Container engine: "podman" ou "docker"
container_engine: "podman"

// Diretórios adicionais para buscar invkfiles
search_paths: [
    "~/.invowk/cmds",
    "~/projects/shared-commands",
]

// Runtime padrão para comandos que não especificam um
default_runtime: "native"

// Configuração do virtual shell
virtual_shell: {
    enable_uroot_utils: true
}

// Preferências de UI
ui: {
    color_scheme: "auto"  // "auto", "dark" ou "light"
    verbose: false
}
```

## Hierarquia de Configuração

Ao executar um comando, Invowk mescla configuração de múltiplas fontes (fontes posteriores sobrescrevem as anteriores):

1. **Padrões internos** - Valores padrão sensatos para todas as opções
2. **Arquivo de configuração** - Suas configurações do `config.cue`
3. **Variáveis de ambiente** - Variáveis de ambiente `INVOWK_*`
4. **Flags de linha de comando** - Flags como `--verbose`, `--runtime`

Por exemplo, se seu arquivo de configuração define `verbose: false`, mas você executa com `--verbose`, o modo verbose será habilitado.

## Próximos Passos

Vá para [Opções de Configuração](./options) para uma referência completa de todas as configurações disponíveis.
