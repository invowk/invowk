---
sidebar_position: 1
---

# Interpretadores

Por padrão, o Invowk executa scripts usando um shell. Mas você pode usar outros interpretadores como Python, Ruby, Node.js ou qualquer executável que possa rodar scripts.

## Auto-Detecção a partir do Shebang

Quando um script começa com um shebang (`#!`), o Invowk automaticamente usa esse interpretador:

```cue
{
    name: "analyze"
    implementations: [{
        script: """
            #!/usr/bin/env python3
            import sys
            import json
            
            data = {"status": "ok", "python": sys.version}
            print(json.dumps(data, indent=2))
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

Padrões comuns de shebang:

| Shebang | Interpretador |
|---------|---------------|
| `#!/usr/bin/env python3` | Python 3 (portável) |
| `#!/usr/bin/env node` | Node.js |
| `#!/usr/bin/env ruby` | Ruby |
| `#!/usr/bin/env perl` | Perl |
| `#!/bin/bash` | Bash (caminho direto) |

## Interpretador Explícito

Especifique um interpretador diretamente na configuração do runtime:

```cue
{
    name: "script"
    implementations: [{
        script: """
            import sys
            print(f"Python {sys.version_info.major}.{sys.version_info.minor}")
            """
        target: {
            runtimes: [{
                name: "native"
                interpreter: "python3"  // Explícito
            }]
        }
    }]
}
```

O interpretador explícito tem precedência sobre a detecção de shebang.

## Interpretador com Argumentos

Passe argumentos para o interpretador:

```cue
{
    name: "unbuffered"
    implementations: [{
        script: """
            import time
            for i in range(5):
                print(f"Count: {i}")
                time.sleep(1)
            """
        target: {
            runtimes: [{
                name: "native"
                interpreter: "python3 -u"  // Saída sem buffer
            }]
        }
    }]
}
```

Mais exemplos:

```cue
// Perl com warnings
interpreter: "perl -w"

// Ruby com modo debug
interpreter: "ruby -d"

// Node com opções específicas
interpreter: "node --max-old-space-size=4096"
```

## Interpretadores em Containers

Interpretadores funcionam em containers também:

```cue
{
    name: "analyze"
    implementations: [{
        script: """
            #!/usr/bin/env python3
            import os
            print(f"Running in container at {os.getcwd()}")
            """
        target: {
            runtimes: [{
                name: "container"
                image: "python:3.11-alpine"
            }]
        }
    }]
}
```

Ou com interpretador explícito:

```cue
{
    name: "script"
    implementations: [{
        script: """
            console.log('Hello from Node in container!')
            console.log('Node version:', process.version)
            """
        target: {
            runtimes: [{
                name: "container"
                image: "node:20-alpine"
                interpreter: "node"
            }]
        }
    }]
}
```

## Acessando Argumentos

Argumentos funcionam da mesma forma com qualquer interpretador:

```cue
{
    name: "greet"
    args: [{name: "name", default_value: "World"}]
    implementations: [{
        script: """
            #!/usr/bin/env python3
            import sys
            import os
            
            # Via argumentos de linha de comando
            name = sys.argv[1] if len(sys.argv) > 1 else "World"
            print(f"Hello, {name}!")
            
            # Ou via variável de ambiente
            name = os.environ.get("INVOWK_ARG_NAME", "World")
            print(f"Hello again, {name}!")
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Interpretadores Suportados

Qualquer executável no PATH pode ser usado:

- **Python**: `python3`, `python`
- **JavaScript**: `node`, `deno`, `bun`
- **Ruby**: `ruby`
- **Perl**: `perl`
- **PHP**: `php`
- **Lua**: `lua`
- **R**: `Rscript`
- **Shell**: `bash`, `sh`, `zsh`, `fish`
- **Personalizado**: Qualquer executável

## Limitação do Runtime Virtual

O campo `interpreter` **não é suportado** com o runtime virtual:

```cue
// Isso NÃO vai funcionar!
{
    name: "bad"
    implementations: [{
        script: "print('hello')"
        target: {
            runtimes: [{
                name: "virtual"
                interpreter: "python3"  // ERRO!
            }]
        }
    }]
}
```

O runtime virtual usa o interpretador mvdan/sh embutido e não pode executar Python, Ruby ou outros interpretadores. Use runtime native ou container em vez disso.

## Comportamento de Fallback

Quando não há shebang e nenhum interpretador explícito:

- **Runtime native**: Usa o shell padrão do sistema
- **Runtime container**: Usa `/bin/sh -c`

## Melhores Práticas

1. **Use shebang para portabilidade**: Scripts funcionam standalone também
2. **Use `/usr/bin/env`**: Mais portável que caminhos diretos
3. **Interpretador explícito para scripts sem shebang**: Quando você não quer uma linha de shebang
4. **Combine imagem do container**: Garanta que o interpretador existe na imagem

## Próximos Passos

- [Working Directory](./workdir) - Controlar local de execução
- [Platform-Specific](./platform-specific) - Implementações por plataforma
