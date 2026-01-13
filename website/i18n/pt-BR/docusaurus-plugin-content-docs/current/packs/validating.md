---
sidebar_position: 3
---

# Validando Packs

Valide seus packs para garantir que estão corretamente estruturados e prontos para distribuição.

## Validação Básica

```bash
invowk pack validate ./mytools.invkpack
```

Saída para um pack válido:
```
Pack Validation
• Path: /home/user/mytools.invkpack
• Name: mytools

✓ Pack is valid

✓ Structure check passed
✓ Naming convention check passed
✓ Required files present
```

## Validação Profunda

Adicione `--deep` para também analisar e validar o invkfile:

```bash
invowk pack validate ./mytools.invkpack --deep
```

Saída:
```
Pack Validation
• Path: /home/user/mytools.invkpack
• Name: mytools

✓ Pack is valid

✓ Structure check passed
✓ Naming convention check passed
✓ Required files present
✓ Invkfile parses successfully
```

## O que é Validado

### Verificações de Estrutura

- Diretório do pack existe
- `invkfile.cue` está presente na raiz
- Sem packs aninhados (packs não podem conter outros packs)

### Verificações de Nomenclatura

- Nome da pasta termina com `.invkpack`
- Prefixo do nome segue as regras (começa com letra, alfanumérico + pontos)
- Sem caracteres inválidos (hífens, underscores)

### Verificações Profundas (com `--deep`)

- Invkfile é parseado sem erros
- Sintaxe CUE é válida
- Restrições do schema são atendidas
- Caminhos de script são válidos (relativos, dentro do pack)

## Erros de Validação

### Invkfile Ausente

```
Pack Validation
• Path: /home/user/bad.invkpack

✗ Pack validation failed with 1 issue(s)

  1. [structure] missing required invkfile.cue
```

### Nome Inválido

```
Pack Validation
• Path: /home/user/my-tools.invkpack

✗ Pack validation failed with 1 issue(s)

  1. [naming] pack name 'my-tools' contains invalid characters (hyphens not allowed)
```

### Pack Aninhado

```
Pack Validation
• Path: /home/user/parent.invkpack

✗ Pack validation failed with 1 issue(s)

  1. [structure] nested.invkpack: nested packs are not allowed
```

### Invkfile Inválido (deep)

```
Pack Validation
• Path: /home/user/broken.invkpack

✗ Pack validation failed with 1 issue(s)

  1. [invkfile] parse error at line 15: expected '}', found EOF
```

## Validação em Lote

Valide múltiplos packs:

```bash
# Validar todos os packs em um diretório
for pack in ./packs/*.invkpack; do
    invowk pack validate "$pack" --deep
done
```

## Integração com CI

Adicione validação de pack ao seu pipeline de CI:

```yaml
# Exemplo de GitHub Actions
- name: Validate packs
  run: |
    for pack in packs/*.invkpack; do
      invowk pack validate "$pack" --deep
    done
```

## Problemas Comuns

### Separadores de Caminho Errados

```cue
// Ruim - estilo Windows
script: "scripts\\build.sh"

// Bom - barras normais
script: "scripts/build.sh"
```

### Escapando do Diretório do Pack

```cue
// Ruim - tenta acessar diretório pai
script: "../outside/script.sh"

// Bom - permanece dentro do pack
script: "scripts/script.sh"
```

### Caminhos Absolutos

```cue
// Ruim - caminho absoluto
script: "/usr/local/bin/script.sh"

// Bom - caminho relativo
script: "scripts/script.sh"
```

## Melhores Práticas

1. **Valide antes de commitar**: Detecte problemas cedo
2. **Use `--deep`**: Captura erros no invkfile
3. **Valide no CI**: Previne que packs quebrados sejam distribuídos
4. **Corrija problemas imediatamente**: Não deixe dívida de validação acumular

## Próximos Passos

- [Criando Packs](./creating-packs) - Estruturar seu pack
- [Distribuindo](./distributing) - Compartilhar seu pack
