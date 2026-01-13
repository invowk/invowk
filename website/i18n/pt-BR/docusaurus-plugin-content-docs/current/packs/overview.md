---
sidebar_position: 1
---

# Visão Geral de Packs

:::warning Alpha — O Formato de Pack Pode Mudar
O formato e estrutura de packs ainda estão sendo finalizados. Embora nosso objetivo seja manter compatibilidade retroativa, **mudanças incompatíveis podem ocorrer** antes do release 1.0. Se você está distribuindo packs externamente, esteja preparado para atualizá-los ao fazer upgrade do Invowk.
:::

Packs são pastas autocontidas que agrupam um invkfile junto com seus arquivos de script. Eles são perfeitos para compartilhar comandos, criar toolkits reutilizáveis e distribuir automação entre equipes.

## O que é um Pack?

Um pack é um diretório com o sufixo `.invkpack`:

```
mytools.invkpack/
├── invkfile.cue          # Obrigatório: definições de comandos
├── scripts/               # Opcional: arquivos de script
│   ├── build.sh
│   └── deploy.sh
└── templates/             # Opcional: outros recursos
    └── config.yaml
```

## Por que Usar Packs?

- **Portabilidade**: Compartilhe um conjunto completo de comandos como uma única pasta
- **Autocontido**: Scripts são agrupados com o invkfile
- **Multiplataforma**: Caminhos com barra funcionam em qualquer lugar
- **Isolamento de namespace**: Nomenclatura RDNS previne conflitos
- **Fácil distribuição**: Compacte, compartilhe, descompacte

## Início Rápido

### Criar um Pack

```bash
invowk pack create mytools
```

Cria:
```
mytools.invkpack/
└── invkfile.cue
```

### Usar o Pack

Packs são descobertos automaticamente de:
1. Diretório atual
2. `~/.invowk/cmds/` (comandos do usuário)
3. Caminhos de busca configurados

```bash
# Listar comandos (comandos de pack aparecem automaticamente)
invowk cmd list

# Executar um comando de pack
invowk cmd mytools hello
```

### Compartilhar o Pack

```bash
# Criar um arquivo zip
invowk pack archive mytools.invkpack

# Compartilhar o arquivo zip
# Destinatários importam com:
invowk pack import mytools.invkpack.zip
```

## Estrutura de Pack

### Arquivos Obrigatórios

- **`invkfile.cue`**: Definições de comandos (deve estar na raiz do pack)

### Conteúdo Opcional

- **Scripts**: Shell scripts, arquivos Python, etc.
- **Templates**: Templates de configuração
- **Dados**: Quaisquer arquivos de suporte

### Exemplo de Estrutura

```
com.example.devtools.invkpack/
├── invkfile.cue
├── scripts/
│   ├── build.sh
│   ├── deploy.sh
│   └── utils/
│       └── helpers.sh
├── templates/
│   ├── Dockerfile.tmpl
│   └── config.yaml.tmpl
└── README.md
```

## Nomenclatura de Pack

Nomes de pasta de pack seguem estas regras:

| Regra | Válido | Inválido |
|-------|--------|----------|
| Terminar com `.invkpack` | `mytools.invkpack` | `mytools` |
| Começar com letra | `mytools.invkpack` | `123tools.invkpack` |
| Alfanumérico + pontos | `com.example.invkpack` | `my-tools.invkpack` |

### Nomenclatura RDNS

Recomendada para packs compartilhados:

```
com.company.projectname.invkpack
io.github.username.toolkit.invkpack
org.opensource.utilities.invkpack
```

## Caminhos de Script

Referencie scripts relativos à raiz do pack com **barras normais**:

```cue
// Dentro de mytools.invkpack/invkfile.cue
group: "mytools"

commands: [
    {
        name: "build"
        implementations: [{
            script: "scripts/build.sh"  // Relativo à raiz do pack
            target: {runtimes: [{name: "native"}]}
        }]
    },
    {
        name: "deploy"
        implementations: [{
            script: "scripts/utils/helpers.sh"  // Caminho aninhado
            target: {runtimes: [{name: "native"}]}
        }]
    }
]
```

**Importante:**
- Sempre use barras normais (`/`)
- Caminhos são relativos à raiz do pack
- Caminhos absolutos não são permitidos
- Não é possível escapar do diretório do pack (`../` é inválido)

## Comandos de Pack

| Comando | Descrição |
|---------|-----------|
| `invowk pack create` | Criar um novo pack |
| `invowk pack validate` | Validar estrutura do pack |
| `invowk pack list` | Listar packs descobertos |
| `invowk pack archive` | Criar arquivo zip |
| `invowk pack import` | Instalar de zip/URL |

## Descoberta

Packs são descobertos dessas localizações:

1. **Diretório atual** (maior prioridade)
2. **Comandos do usuário** (`~/.invowk/cmds/`)
3. **Caminhos de busca** (da configuração)

Comandos aparecem em `invowk cmd list` com sua origem:

```
Available Commands

From current directory:
  mytools build - Build the project [native*]

From user commands (~/.invowk/cmds):
  com.example.utilities hello - Greeting [native*]
```

## Próximos Passos

- [Criando Packs](./creating-packs) - Criar estrutura e organizar packs
- [Validando](./validating) - Garantir integridade do pack
- [Distribuindo](./distributing) - Compartilhar packs com outros
