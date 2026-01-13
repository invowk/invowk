---
sidebar_position: 3
---

# Runtime Virtual

O runtime **virtual** usa o interpretador de shell POSIX-compatível integrado do Invowk (powered by [mvdan/sh](https://github.com/mvdan/sh)). Ele fornece comportamento de shell consistente em todas as plataformas sem exigir um shell externo.

## Como Funciona

Quando você executa um comando com o runtime virtual, o Invowk:

1. Faz o parse do script usando o parser de shell integrado
2. Executa-o em um ambiente embarcado tipo POSIX
3. Fornece utilitários core (echo, test, etc.) integrados

## Uso Básico

```cue
{
    name: "build"
    implementations: [{
        script: """
            echo "Building..."
            go build -o bin/app ./...
            echo "Done!"
            """
        target: {
            runtimes: [{name: "virtual"}]
        }
    }]
}
```

```bash
invowk cmd myproject build --runtime virtual
```

## Consistência Multiplataforma

O runtime virtual se comporta de forma idêntica no Linux, macOS e Windows:

```cue
{
    name: "setup"
    implementations: [{
        script: """
            # Isso funciona igual em todos os lugares!
            if [ -d "node_modules" ]; then
                echo "Dependencies already installed"
            else
                echo "Installing dependencies..."
                npm install
            fi
            """
        target: {
            runtimes: [{name: "virtual"}]
        }
    }]
}
```

Chega de "funciona na minha máquina" para scripts shell!

## Utilitários Integrados

O shell virtual inclui utilitários POSIX core:

| Utilitário | Descrição |
|------------|-----------|
| `echo` | Imprimir texto |
| `printf` | Saída formatada |
| `test` / `[` | Condicionais |
| `true` / `false` | Sair com 0/1 |
| `pwd` | Imprimir diretório de trabalho |
| `cd` | Mudar diretório |
| `read` | Ler entrada |
| `export` | Definir variáveis de ambiente |

### Utilitários Estendidos (u-root)

Quando habilitado na configuração, utilitários adicionais estão disponíveis:

```cue
// No seu arquivo de configuração
virtual_shell: {
    enable_uroot_utils: true
}
```

Isso adiciona utilitários como:
- `cat`, `head`, `tail`
- `grep`, `sed`, `awk`
- `ls`, `cp`, `mv`, `rm`
- `mkdir`, `rmdir`
- E muitos outros

## Recursos do Shell POSIX

O shell virtual suporta construções POSIX padrão:

### Variáveis

```cue
script: """
    NAME="World"
    echo "Hello, $NAME!"
    
    # Expansão de parâmetros
    echo "${NAME:-default}"
    echo "${#NAME}"  # Comprimento
    """
```

### Condicionais

```cue
script: """
    if [ "$ENV" = "production" ]; then
        echo "Production mode"
    elif [ "$ENV" = "staging" ]; then
        echo "Staging mode"
    else
        echo "Development mode"
    fi
    """
```

### Loops

```cue
script: """
    # Loop for
    for file in *.go; do
        echo "Processing $file"
    done
    
    # Loop while
    count=0
    while [ $count -lt 5 ]; do
        echo "Count: $count"
        count=$((count + 1))
    done
    """
```

### Funções

```cue
script: """
    greet() {
        echo "Hello, $1!"
    }
    
    greet "World"
    greet "Invowk"
    """
```

### Subshells e Substituição de Comandos

```cue
script: """
    # Substituição de comandos
    current_date=$(date +%Y-%m-%d)
    echo "Today is $current_date"
    
    # Subshell
    (cd /tmp && echo "In temp: $(pwd)")
    echo "Still in: $(pwd)"
    """
```

## Chamando Comandos Externos

O shell virtual pode chamar comandos externos instalados no seu sistema:

```cue
script: """
    # Chama o binário 'go' real
    go version
    
    # Chama o binário 'git' real
    git status
    """
```

Comandos externos são encontrados usando o PATH do sistema.

## Variáveis de Ambiente

Variáveis de ambiente funcionam da mesma forma que no native:

```cue
{
    name: "build"
    env: {
        vars: {
            BUILD_MODE: "release"
        }
    }
    implementations: [{
        script: """
            echo "Building in $BUILD_MODE mode"
            go build -ldflags="-s -w" ./...
            """
        target: {
            runtimes: [{name: "virtual"}]
        }
    }]
}
```

## Flags e Argumentos

Acesse flags e argumentos da mesma forma:

```cue
{
    name: "greet"
    args: [{name: "name", default_value: "World"}]
    implementations: [{
        script: """
            # Usando variável de ambiente
            echo "Hello, $INVOWK_ARG_NAME!"
            
            # Ou parâmetro posicional
            echo "Hello, $1!"
            """
        target: {
            runtimes: [{name: "virtual"}]
        }
    }]
}
```

## Limitações

### Sem Suporte a Interpretadores

O runtime virtual **não pode** usar interpretadores não-shell:

```cue
// Isso NÃO funcionará com o runtime virtual!
{
    name: "bad-example"
    implementations: [{
        script: """
            #!/usr/bin/env python3
            print("This won't work!")
            """
        target: {
            runtimes: [{
                name: "virtual"
                interpreter: "python3"  // ERRO: Não suportado
            }]
        }
    }]
}
```

Para Python, Ruby ou outros interpretadores, use o runtime native ou container.

### Recursos Específicos do Bash

Alguns recursos específicos do bash não estão disponíveis:

```cue
// Estes não funcionarão no runtime virtual:
script: """
    # Arrays do Bash (use $@ em vez disso)
    declare -a arr=(1 2 3)  # Não suportado
    
    # Expansão de parâmetros específica do Bash
    ${var^^}  # Maiúsculas - não suportado
    ${var,,}  # Minúsculas - não suportado
    
    # Substituição de processo
    diff <(cmd1) <(cmd2)  # Não suportado
    """
```

Mantenha construções POSIX-compatíveis para o runtime virtual.

## Validação de Dependências

Dependências são validadas contra as capacidades do shell virtual:

```cue
{
    name: "build"
    depends_on: {
        tools: [
            // Estas serão verificadas no ambiente do shell virtual
            {alternatives: ["go"]},
            {alternatives: ["git"]}
        ]
    }
    implementations: [{
        script: "go build ./..."
        target: {
            runtimes: [{name: "virtual"}]
        }
    }]
}
```

## Vantagens

- **Consistência**: Mesmo comportamento no Linux, macOS e Windows
- **Sem dependência de shell**: Funciona mesmo se o shell do sistema não estiver disponível
- **Portabilidade**: Scripts funcionam em todas as plataformas
- **Utilitários integrados**: Utilitários core sempre disponíveis
- **Startup mais rápido**: Sem processo de shell para iniciar

## Quando Usar Virtual

- **Scripts multiplataforma**: Quando o mesmo script deve funcionar em todos os lugares
- **Pipelines CI/CD**: Comportamento consistente entre agentes de build
- **Scripts shell simples**: Quando você não precisa de recursos específicos do bash
- **Ambientes embarcados**: Quando shells externos não estão disponíveis

## Configuração

Configure o shell virtual no seu arquivo de configuração do Invowk:

```cue
// ~/.config/invowk/config.cue (Linux)
// ~/Library/Application Support/invowk/config.cue (macOS)
// %APPDATA%\invowk\config.cue (Windows)

virtual_shell: {
    // Habilitar utilitários adicionais do u-root
    enable_uroot_utils: true
}
```

## Próximos Passos

- [Runtime Native](./native) - Para acesso completo ao shell
- [Runtime Container](./container) - Para execução isolada
