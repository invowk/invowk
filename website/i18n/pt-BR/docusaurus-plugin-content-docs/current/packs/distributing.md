---
sidebar_position: 4
---

# Distribuindo Packs

Compartilhe seus packs com colegas de equipe, entre organizações ou com o mundo.

## Criando Arquivos

Crie um arquivo zip para distribuição:

```bash
# Saída padrão: <pack-name>.invkpack.zip
invowk pack archive ./mytools.invkpack

# Caminho de saída customizado
invowk pack archive ./mytools.invkpack --output ./dist/mytools.zip
```

Saída:
```
Archive Pack

✓ Pack archived successfully

• Output: /home/user/dist/mytools.zip
• Size: 2.45 KB
```

## Importando Packs

### De Arquivo Local

```bash
# Instalar em ~/.invowk/cmds/
invowk pack import ./mytools.invkpack.zip

# Instalar em diretório customizado
invowk pack import ./mytools.invkpack.zip --path ./local-packs

# Sobrescrever existente
invowk pack import ./mytools.invkpack.zip --overwrite
```

### De URL

```bash
# Baixar e instalar
invowk pack import https://example.com/packs/mytools.zip

# De release do GitHub
invowk pack import https://github.com/user/repo/releases/download/v1.0/mytools.invkpack.zip
```

Saída:
```
Import Pack

✓ Pack imported successfully

• Name: mytools
• Path: /home/user/.invowk/cmds/mytools.invkpack

• The pack commands are now available via invowk
```

## Listando Packs Instalados

```bash
invowk pack list
```

Saída:
```
Discovered Packs

• Found 3 pack(s)

• current directory:
   ✓ local.project
      /home/user/project/local.project.invkpack

• user commands (~/.invowk/cmds):
   ✓ com.company.devtools
      /home/user/.invowk/cmds/com.company.devtools.invkpack
   ✓ io.github.user.utilities
      /home/user/.invowk/cmds/io.github.user.utilities.invkpack
```

## Métodos de Distribuição

### Compartilhamento Direto

1. Crie o arquivo: `invowk pack archive`
2. Compartilhe o arquivo zip (email, Slack, etc.)
3. Destinatário importa: `invowk pack import`

### Repositório Git

Inclua packs no seu repositório:

```
my-project/
├── src/
├── packs/
│   ├── devtools.invkpack/
│   └── testing.invkpack/
└── invkfile.cue
```

Membros da equipe obtêm os packs ao clonar o repositório.

### Releases do GitHub

1. Crie o arquivo
2. Anexe ao release do GitHub
3. Compartilhe a URL de download

```bash
# Destinatários instalam com:
invowk pack import https://github.com/org/repo/releases/download/v1.0.0/mytools.invkpack.zip
```

### Registro de Pacotes (Futuro)

Versões futuras podem suportar:
```bash
invowk pack install com.company.devtools@1.0.0
```

## Locais de Instalação

### Comandos do Usuário (Padrão)

```bash
invowk pack import mytools.zip
# Instalado em: ~/.invowk/cmds/mytools.invkpack/
```

Disponível globalmente para o usuário.

### Local do Projeto

```bash
invowk pack import mytools.zip --path ./packs
# Instalado em: ./packs/mytools.invkpack/
```

Disponível apenas neste projeto.

### Caminho de Busca Customizado

Configure caminhos de busca adicionais:

```cue
// ~/.config/invowk/config.cue
search_paths: [
    "/shared/company-packs"
]
```

Instale lá:
```bash
invowk pack import mytools.zip --path /shared/company-packs
```

## Gerenciamento de Versão

### Versionamento Semântico

Use version no seu invkfile:

```cue
group: "com.company.tools"
version: "1.2.0"
```

### Nomenclatura de Arquivo

Inclua a versão no nome do arquivo:

```bash
invowk pack archive ./mytools.invkpack --output ./dist/mytools-1.2.0.zip
```

### Processo de Upgrade

```bash
# Remover versão antiga
rm -rf ~/.invowk/cmds/mytools.invkpack

# Instalar nova versão
invowk pack import mytools-1.2.0.zip

# Ou use --overwrite
invowk pack import mytools-1.2.0.zip --overwrite
```

## Distribuição para Equipe

### Localização de Rede Compartilhada

```bash
# Admin publica
invowk pack archive ./devtools.invkpack --output /shared/packs/devtools.zip

# Membros da equipe importam
invowk pack import /shared/packs/devtools.zip
```

### Servidor de Pacotes Interno

Hospede packs em servidor HTTP interno:

```bash
# Membros da equipe importam via URL
invowk pack import https://internal.company.com/packs/devtools.zip
```

## Melhores Práticas

1. **Valide antes de arquivar**: `invowk pack validate --deep`
2. **Use versionamento semântico**: Rastreie mudanças claramente
3. **Inclua README**: Documente o uso do pack
4. **Nomenclatura RDNS**: Previne conflitos
5. **Changelog**: Documente o que mudou entre versões

## Exemplo de Workflow

```bash
# 1. Criar e desenvolver pack
invowk pack create com.company.mytools --scripts
# ... adicionar comandos e scripts ...

# 2. Validar
invowk pack validate ./com.company.mytools.invkpack --deep

# 3. Criar arquivo versionado
invowk pack archive ./com.company.mytools.invkpack \
    --output ./releases/mytools-1.0.0.zip

# 4. Distribuir (ex.: upload para release do GitHub)

# 5. Equipe importa
invowk pack import https://github.com/company/mytools/releases/download/v1.0.0/mytools-1.0.0.zip
```

## Próximos Passos

- [Visão Geral](./overview) - Conceitos de pack
- [Criando Packs](./creating-packs) - Construir seu pack
- [Validando](./validating) - Garantir integridade do pack
