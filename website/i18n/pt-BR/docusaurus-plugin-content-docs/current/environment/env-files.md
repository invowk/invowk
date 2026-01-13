---
sidebar_position: 2
---

# Arquivos Env

Carregue variáveis de ambiente de arquivos `.env`. Este é um padrão comum para gerenciar configuração, especialmente secrets que não devem ser commitados no controle de versão.

## Uso Básico

```cue
{
    name: "build"
    env: {
        files: [".env"]
    }
    implementations: [...]
}
```

Com um arquivo `.env`:
```bash
# .env
API_KEY=secret123
DATABASE_URL=postgres://localhost/mydb
DEBUG=false
```

Variáveis são carregadas e disponíveis no seu script.

## Formato do Arquivo

Formato `.env` padrão:

```bash
# Comentários começam com #
KEY=value

# Valores entre aspas (espaços preservados)
MESSAGE="Hello World"
PATH_WITH_SPACES='/path/to/my file'

# Valor vazio
EMPTY_VAR=

# Sem valor (mesmo que vazio)
NO_VALUE

# Multilinhas (use aspas)
MULTILINE="line1
line2
line3"
```

## Arquivos Opcionais

Sufixe com `?` para tornar um arquivo opcional:

```cue
env: {
    files: [
        ".env",           // Obrigatório - erro se faltando
        ".env.local?",    // Opcional - ignorado se faltando
        ".env.secrets?",  // Opcional
    ]
}
```

Isso é útil para:
- Sobrescritas locais que podem não existir
- Arquivos específicos de ambiente
- Configurações específicas de desenvolvedor

## Ordem dos Arquivos

Arquivos são carregados em ordem; arquivos posteriores sobrescrevem os anteriores:

```cue
env: {
    files: [
        ".env",           // Config base
        ".env.${ENV}?",   // Sobrescritas específicas de ambiente
        ".env.local?",    // Sobrescritas locais (maior prioridade)
    ]
}
```

Exemplo com `ENV=production`:

```bash
# .env
API_URL=http://localhost:3000
DEBUG=true

# .env.production
API_URL=https://api.example.com
DEBUG=false

# .env.local (sobrescrita do desenvolvedor)
DEBUG=true
```

Resultado:
- `API_URL=https://api.example.com` (de .env.production)
- `DEBUG=true` (de .env.local)

## Resolução de Caminho

Caminhos são relativos à localização do invkfile:

```
project/
├── invkfile.cue
├── .env                  # files: [".env"]
├── config/
│   └── .env.prod         # files: ["config/.env.prod"]
└── src/
```

Para packs, caminhos são relativos à raiz do pack.

## Interpolação de Variáveis

Use `${VAR}` para incluir outras variáveis de ambiente:

```cue
env: {
    files: [
        ".env",
        ".env.${NODE_ENV}?",    // Usa valor de NODE_ENV
        ".env.${USER}?",        // Usa usuário atual
    ]
}
```

```bash
# Se NODE_ENV=production, carrega:
# - .env
# - .env.production (se existir)
# - .env.john (se existir e USER=john)
```

## Níveis de Escopo

Arquivos env podem ser carregados em múltiplos níveis:

### Nível Raiz

```cue
group: "myproject"

env: {
    files: [".env"]  // Carregado para todos os comandos
}

commands: [...]
```

### Nível de Comando

```cue
{
    name: "build"
    env: {
        files: [".env.build"]  // Apenas para este comando
    }
    implementations: [...]
}
```

### Nível de Implementação

```cue
{
    name: "build"
    implementations: [
        {
            script: "npm run build"
            target: {runtimes: [{name: "native"}]}
            env: {
                files: [".env.node"]  // Apenas para esta implementação
            }
        }
    ]
}
```

## Combinado com Variáveis

Use ambos arquivos e variáveis diretas:

```cue
env: {
    files: [".env"]
    vars: {
        // Estas sobrescrevem valores do .env
        OVERRIDE_VALUE: "from-invkfile"
    }
}
```

Variáveis em `vars` sempre sobrescrevem valores de arquivos no mesmo nível.

## Sobrescrita via CLI

Carregue arquivos adicionais em tempo de execução:

```bash
# Carregar arquivo extra
invowk cmd myproject build --env-file .env.custom

# Forma curta
invowk cmd myproject build -e .env.custom

# Múltiplos arquivos
invowk cmd myproject build -e .env.custom -e .env.secrets
```

Arquivos da CLI têm a maior prioridade e sobrescrevem todas as fontes definidas no invkfile.

## Exemplos do Mundo Real

### Desenvolvimento vs Produção

```cue
{
    name: "start"
    env: {
        files: [
            ".env",                    // Config base
            ".env.${NODE_ENV:-dev}?",  // Específico de ambiente
            ".env.local?",             // Sobrescritas locais
        ]
    }
    implementations: [{
        script: "node server.js"
        target: {runtimes: [{name: "native"}]}
    }]
}
```

### Gerenciamento de Secrets

```cue
{
    name: "deploy"
    env: {
        files: [
            ".env",                    // Config não sensível
            ".env.secrets?",           // Sensível - não está no git
        ]
    }
    implementations: [{
        script: """
            echo "Deploying with API_KEY..."
            ./deploy.sh
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

`.gitignore`:
```
.env.secrets
.env.local
```

### Multi-Ambiente

```
project/
├── invkfile.cue
├── .env                  # Padrões compartilhados
├── .env.development      # Configurações de dev
├── .env.staging          # Configurações de staging
└── .env.production       # Configurações de produção
```

```cue
{
    name: "deploy"
    env: {
        files: [
            ".env",
            ".env.${DEPLOY_ENV}",  // DEPLOY_ENV deve estar definido
        ]
    }
    depends_on: {
        env_vars: [
            {alternatives: [{name: "DEPLOY_ENV", validation: "^(development|staging|production)$"}]}
        ]
    }
    implementations: [...]
}
```

## Melhores Práticas

1. **Use `.env` para padrões**: Configuração base que funciona para todos
2. **Use `.env.local` para sobrescritas**: Configurações específicas de desenvolvedor, não no git
3. **Use `.env.{environment}` para ambientes**: Produção, staging, etc.
4. **Marque arquivos sensíveis como opcionais**: Podem não existir em todos os ambientes
5. **Não commite secrets**: Adicione `.env.secrets`, `.env.local` ao `.gitignore`

## Próximos Passos

- [Env Vars](./env-vars) - Definir variáveis diretamente
- [Precedence](./precedence) - Entender ordem de sobrescrita
