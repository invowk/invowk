---
sidebar_position: 2
---

# Comandos e Grupos

Todo comando no Invowk vive dentro de um **grupo**. Pense nos grupos como namespaces que organizam seus comandos e evitam conflitos de nomenclatura quando você combina múltiplos invkfiles.

## Entendendo Grupos

O campo `group` é obrigatório em todo invkfile e se torna o primeiro segmento de todos os nomes de comandos:

```cue
group: "myproject"

commands: [
    {name: "build"},
    {name: "test"},
    {name: "deploy"},
]
```

Esses comandos se tornam:
- `invowk cmd myproject build`
- `invowk cmd myproject test`
- `invowk cmd myproject deploy`

## Regras de Nomenclatura de Grupos

Grupos seguem convenções de nomenclatura específicas:

| Regra | Válido | Inválido |
|-------|--------|----------|
| Deve começar com letra | `myproject`, `Project1` | `1project`, `_project` |
| Apenas letras e números | `myproject`, `v2` | `my-project`, `my_project` |
| Pontos para aninhamento | `my.project`, `a.b.c` | `my..project`, `.project`, `project.` |

### Exemplos Válidos

```cue
group: "frontend"
group: "backend"
group: "my.project"
group: "com.company.tools"
group: "io.github.username.cli"
```

### Exemplos Inválidos

```cue
group: "my-project"     // Hífens não são permitidos
group: "my_project"     // Underscores não são permitidos
group: ".project"       // Não pode começar com ponto
group: "project."       // Não pode terminar com ponto
group: "my..project"    // Sem pontos consecutivos
group: "123project"     // Deve começar com letra
```

## Grupos Aninhados

Pontos criam namespaces hierárquicos, úteis para organizar projetos grandes:

```cue
group: "com.company.frontend"

commands: [
    {name: "build"},
    {name: "test"},
]
```

Comandos se tornam:
- `invowk cmd com.company.frontend build`
- `invowk cmd com.company.frontend test`

### Nomenclatura RDNS

Para packs ou invkfiles compartilhados, recomendamos a nomenclatura RDNS (Reverse Domain Name System):

```cue
group: "com.yourcompany.devtools"
group: "io.github.username.project"
group: "org.opensource.utilities"
```

Isso evita conflitos ao combinar comandos de múltiplas fontes.

## Nomes de Comandos

Dentro de um grupo, nomes de comandos podem incluir espaços para criar hierarquias estilo subcomandos:

```cue
group: "myproject"

commands: [
    {name: "test"},           // myproject test
    {name: "test unit"},      // myproject test unit
    {name: "test integration"}, // myproject test integration
    {name: "db migrate"},     // myproject db migrate
    {name: "db seed"},        // myproject db seed
]
```

### Regras de Nomenclatura de Comandos

| Regra | Válido | Inválido |
|-------|--------|----------|
| Deve começar com letra | `build`, `Test` | `1build` |
| Letras, números, espaços, hífens, underscores | `test unit`, `build-all` | `build@all` |

### Comandos Hierárquicos

Espaços em nomes de comandos criam hierarquias naturais:

```bash
# Listar todos os comandos
invowk cmd list

# Executar comando de nível superior
invowk cmd myproject test

# Executar comando aninhado
invowk cmd myproject test unit
```

Isso é puramente organizacional - não há relação especial pai-filho. Cada comando é independente.

## Descoberta de Comandos

O Invowk descobre comandos de múltiplas fontes em ordem de prioridade:

1. **Diretório atual** (maior prioridade)
2. **Diretório de comandos do usuário** (`~/.invowk/cmds/`)
3. **Caminhos de busca configurados** (do arquivo de configuração)

Ao listar comandos, você verá sua origem:

```
Available Commands
  (* = default runtime)

From current directory:
  myproject build - Build the project [native*] (linux, macos, windows)

From user commands (~/.invowk/cmds):
  utils hello - A greeting [native*] (linux, macos)
```

## Dependências de Comandos

Comandos podem depender de outros comandos. Sempre use o nome completo com prefixo do grupo:

```cue
group: "myproject"

commands: [
    {
        name: "build"
        implementations: [...]
    },
    {
        name: "test"
        implementations: [...]
        depends_on: {
            commands: [
                // Referencie pelo nome completo (grupo + nome do comando)
                {alternatives: ["myproject build"]}
            ]
        }
    },
    {
        name: "release"
        implementations: [...]
        depends_on: {
            commands: [
                // Pode depender de comandos de outros invkfiles também
                {alternatives: ["myproject build"]},
                {alternatives: ["myproject test"]},
                {alternatives: ["other.project lint"]},
            ]
        }
    }
]
```

### Dependências Entre Invkfiles

Comandos podem depender de comandos de outros invkfiles:

```cue
// Em frontend/invkfile.cue
group: "frontend"

commands: [
    {
        name: "build"
        depends_on: {
            commands: [
                // Depende do build do backend completar primeiro
                {alternatives: ["backend build"]}
            ]
        }
    }
]
```

## Por Que Grupos Importam

1. **Isolamento de Namespace** - Múltiplos invkfiles podem ter comandos `build` sem conflito
2. **Origem Clara** - Você sempre sabe de qual invkfile um comando vem
3. **Organização Lógica** - Use grupos aninhados para projetos grandes
4. **Autocompletar** - Grupos fornecem limites naturais para completar com tab

## Boas Práticas

### Para Projetos Pessoais

Mantenha simples:

```cue
group: "myapp"
```

### Para Projetos de Equipe

Use nomenclatura baseada na organização:

```cue
group: "teamname.projectname"
```

### Para Packs/Comandos Compartilhados

Use RDNS:

```cue
group: "com.company.toolname"
group: "io.github.username.projectname"
```

### Evite Nomes Genéricos

Não use nomes que podem conflitar:

```cue
// Ruim - muito genérico
group: "build"
group: "test"
group: "utils"

// Bom - com namespace
group: "myproject.build"
group: "mycompany.utils"
```

## Próximos Passos

- [Implementações](./implementations) - Aprenda sobre implementações de comandos específicas por plataforma
- [Modos de Runtime](../runtime-modes/overview) - Entenda execução native, virtual e container
