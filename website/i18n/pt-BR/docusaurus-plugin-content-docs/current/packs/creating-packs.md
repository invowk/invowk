---
sidebar_position: 2
---

# Criando Packs

Aprenda como criar, estruturar e organizar packs para seus comandos.

## Criação Rápida

Use `pack create` para criar a estrutura de um novo pack:

```bash
# Pack simples
invowk pack create mytools

# Nomenclatura RDNS
invowk pack create com.company.devtools

# Em diretório específico
invowk pack create mytools --path /path/to/packs

# Com diretório de scripts
invowk pack create mytools --scripts
```

## Estrutura Gerada

Pack básico:
```
mytools.invkpack/
└── invkfile.cue
```

Com `--scripts`:
```
mytools.invkpack/
├── invkfile.cue
└── scripts/
```

## Invkfile Template

O `invkfile.cue` gerado:

```cue
group: "mytools"
version: "1.0"
description: "Commands for mytools"

commands: [
    {
        name: "hello"
        description: "Say hello"
        implementations: [
            {
                script: """
                    echo "Hello from mytools!"
                    """
                target: {
                    runtimes: [{name: "native"}]
                }
            }
        ]
    }
]
```

## Criação Manual

Você também pode criar packs manualmente:

```bash
mkdir mytools.invkpack
touch mytools.invkpack/invkfile.cue
```

## Adicionando Scripts

### Inline vs Externo

Escolha baseado na complexidade:

```cue
commands: [
    // Simples: script inline
    {
        name: "quick"
        implementations: [{
            script: "echo 'Quick task'"
            target: {runtimes: [{name: "native"}]}
        }]
    },
    
    // Complexo: script externo
    {
        name: "complex"
        implementations: [{
            script: "scripts/complex-task.sh"
            target: {runtimes: [{name: "native"}]}
        }]
    }
]
```

### Organização de Scripts

```
mytools.invkpack/
├── invkfile.cue
└── scripts/
    ├── build.sh           # Scripts principais
    ├── deploy.sh
    ├── test.sh
    └── lib/               # Utilitários compartilhados
        ├── logging.sh
        └── validation.sh
```

### Caminhos de Script

Sempre use barras normais:

```cue
// Bom
script: "scripts/build.sh"
script: "scripts/lib/logging.sh"

// Ruim - vai falhar em algumas plataformas
script: "scripts\\build.sh"

// Ruim - escapa do diretório do pack
script: "../outside.sh"
```

## Arquivos de Environment

Inclua arquivos `.env` no seu pack:

```
mytools.invkpack/
├── invkfile.cue
├── .env                   # Configuração padrão
├── .env.example           # Template para usuários
└── scripts/
```

Referencie-os:

```cue
env: {
    files: [".env"]
}
```

## Documentação

Inclua um README para usuários:

```
mytools.invkpack/
├── invkfile.cue
├── README.md              # Instruções de uso
├── CHANGELOG.md           # Histórico de versões
└── scripts/
```

## Exemplos do Mundo Real

### Pack de Build Tools

```
com.company.buildtools.invkpack/
├── invkfile.cue
├── scripts/
│   ├── build-go.sh
│   ├── build-node.sh
│   └── build-python.sh
├── templates/
│   ├── Dockerfile.go
│   ├── Dockerfile.node
│   └── Dockerfile.python
└── README.md
```

```cue
group: "com.company.buildtools"
version: "1.0"
description: "Standardized build tools"

commands: [
    {
        name: "go"
        description: "Build Go project"
        implementations: [{
            script: "scripts/build-go.sh"
            target: {runtimes: [{name: "native"}]}
        }]
    },
    {
        name: "node"
        description: "Build Node.js project"
        implementations: [{
            script: "scripts/build-node.sh"
            target: {runtimes: [{name: "native"}]}
        }]
    },
    {
        name: "python"
        description: "Build Python project"
        implementations: [{
            script: "scripts/build-python.sh"
            target: {runtimes: [{name: "native"}]}
        }]
    }
]
```

### Pack DevOps

```
org.devops.k8s.invkpack/
├── invkfile.cue
├── scripts/
│   ├── deploy.sh
│   ├── rollback.sh
│   └── status.sh
├── manifests/
│   ├── deployment.yaml
│   └── service.yaml
└── .env.example
```

## Melhores Práticas

1. **Use nomenclatura RDNS**: Previne conflitos com outros packs
2. **Mantenha scripts focados**: Uma tarefa por script
3. **Inclua documentação**: README com exemplos de uso
4. **Versione seu pack**: Use versionamento semântico
5. **Apenas barras normais**: Compatibilidade multiplataforma
6. **Valide antes de compartilhar**: Execute `pack validate --deep`

## Validando Seu Pack

Antes de compartilhar, valide:

```bash
invowk pack validate mytools.invkpack --deep
```

Veja [Validando](./validating) para detalhes.

## Próximos Passos

- [Validando](./validating) - Garantir integridade do pack
- [Distribuindo](./distributing) - Compartilhar seu pack
