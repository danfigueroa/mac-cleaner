# CLAUDE.md

Instruções para o Claude Code trabalhar neste repositório.

## O projeto

`mac-cleaner` é uma CLI em Go que audita o armazenamento de um Mac e remove
caches e resíduos regeneráveis. Ver `README.md` para a descrição completa.

Três propriedades definem o projeto. Qualquer mudança que as enfraqueça está
errada, mesmo que os testes passem:

1. **Todo alvo se explica.** Cada regra declara o que é, o que o usuário perde e
   como aquilo volta. Sem as três frases, a ferramenta vira um `rm -rf` com nome
   bonito.
2. **Os números são honestos.** Espaço movido para a Lixeira nunca é somado com
   espaço liberado. Totais subestimados por falta de permissão são anunciados.
3. **A CLI nunca usa `sudo`.** Alvos que exigem root são medidos e exibidos com o
   comando pronto, nunca executados.

---

## Commits

**Só faça commit quando eu pedir explicitamente.** Não commite por iniciativa
própria ao terminar uma tarefa.

### Divida ao máximo

Quando eu pedir para commitar, quebre o trabalho em **quantos commits fizerem
sentido logicamente** — quanto mais granular, melhor. Um commit gigante com
"implementa a feature" documenta mal e não dá para reverter em partes.

Cada commit deve:

- conter **uma única mudança lógica**;
- **compilar e passar nos testes sozinho** (`make lint && make test`);
- poder ser revertido isoladamente sem quebrar o que veio depois.

Ordem preferida quando há muita coisa: primeiro o que não muda comportamento
(tipos, infraestrutura, configuração), depois a lógica, depois os testes que a
cobrem, e por último documentação. Se um teste só faz sentido junto com o código
que ele testa, os dois vão no mesmo commit.

### Formato: Conventional Commits

```
<tipo>(<escopo>): <descrição no imperativo>

<corpo explicando POR QUÊ, não o quê>
```

**Tipos:**

| Tipo | Quando usar |
|---|---|
| `feat` | Nova funcionalidade visível ao usuário |
| `fix` | Correção de bug |
| `refactor` | Muda estrutura sem mudar comportamento |
| `perf` | Melhora desempenho |
| `test` | Adiciona ou corrige testes |
| `docs` | Só documentação |
| `build` | Build, Makefile, dependências, `go.mod` |
| `ci` | GitHub Actions |
| `chore` | Manutenção que não se encaixa acima |
| `style` | Formatação, sem mudança de lógica |

**Escopo:** o pacote afetado — `domain`, `catalog`, `guard`, `service`, `tui`,
`cli`, `report`, `osfs`, `trash`, `cmdrunner`, `memfs`. Omita quando a mudança
for transversal.

**Descrição:** em português, no imperativo, minúscula, sem ponto final, até 72
caracteres. Os tipos ficam em inglês por serem o padrão da convenção.

**Corpo:** obrigatório sempre que o "porquê" não for óbvio pelo título. Explique
a motivação e as consequências, não repita o diff — o diff já está no commit.
Quebre em 72 colunas.

**Rodapés:** `BREAKING CHANGE:` para mudanças incompatíveis, `Refs #123` para
issues.

### Exemplos

```
fix(catalog): não varrer ~/Library nas regras dinâmicas

shouldSkipDir devolvia "não pule" para toda a lista de pulos, quando o
raciocínio valia apenas para node_modules — que precisa ser visitado antes
de ser pulado.

O efeito era duplo: ~/Library era percorrida inteira, e os arquivos
encontrados lá se sobrepunham às regras específicas, inflando o total
prometido de 5,5 para 15,7 GB.
```

```
perf(catalog): compartilhar uma travessia entre as regras dinâmicas

Com uma varredura por regra, medir o catálogo passou de 3 para 94 segundos
numa máquina real. Não por dobrar o trabalho: duas travessias profundas
concorrendo com as 35 regras estáticas transformam leitura sequencial em
disputa por I/O. Isoladas somavam 29s; juntas, 94s.
```

```
feat(guard): permitir diretórios de build em território do usuário

node_modules e .next vivem dentro de Documents e Development, que o guard
bloqueia por completo. Sem uma exceção, a regra de projetos parados seria
impossível.

A exceção é estreita e vale só pelo nome final do caminho. dist, build e
target ficaram de fora: em algum projeto real cada um deles é uma pasta
escrita à mão.
```

### Nunca se coloque como autor

**Não adicione nenhuma forma de autoria ou atribuição** nos commits, nos PRs ou
em qualquer texto gerado. Especificamente, nunca inclua:

- `Co-Authored-By: Claude ...` ou qualquer outro `Co-Authored-By`
- `Generated with Claude Code` ou variações
- Links de sessão, `Claude-Session:`
- O emoji 🤖 ou menções à ferramenta

Os commits são meus. A mensagem termina no corpo ou nos rodapés convencionais
(`BREAKING CHANGE:`, `Refs`), e nada além disso.

---

## Comandos

```sh
make build      # compila em ./bin/mac-cleaner
make install    # instala em $(go env GOPATH)/bin
make test       # testes
make test-race  # com detector de corrida — use antes de commitar
make lint       # golangci-lint, versão pinada em tools/
make fmt        # gofumpt + gci
make tidy       # arruma os dois módulos
```

Antes de considerar qualquer trabalho pronto: `make lint && make test-race`.

Testes de integração rodam contra a máquina real e ficam fora do CI:

```sh
go test -tags integration ./internal/adapter/osfs/   # confere a medição com `du`
go test -tags integration ./internal/service/        # guard × catálogo real
go test -tags integration ./internal/adapter/trash/  # move um arquivo de verdade
```

---

## Arquitetura

```
domain  ←  catalog  ←  service  ←  cli / tui
                          ↑
                       adapter
```

`domain` importa apenas a stdlib. Ninguém importa `cli`. **A direção é imposta
pelo `depguard` no `.golangci.yml`** — não é convenção, quebra o lint.

Ao adicionar um pacote, decida a camada antes de escrever código e, se ele tiver
restrição de dependência, registre a regra no `.golangci.yml`.

### Interfaces

Declare interfaces **no pacote que as consome**, não num pacote central de
"ports". Só crie uma quando existir uma segunda implementação real — hoje são
quatro: sistema de arquivos, executor de comandos, Lixeira e volume. Todo o resto
é struct concreta.

Prefira uma ou duas funções por interface. Fakes de teste são escritos à mão; não
adicione framework de mock.

### Onde cada coisa mora

| Pacote | Papel |
|---|---|
| `internal/domain` | Tipos e regras de negócio. Zero dependências externas. |
| `internal/catalog` | O conhecimento: quais diretórios são lixo e por quê. |
| `internal/catalog/dynamic` | Regras que varrem o disco para achar seus alvos. |
| `internal/guard` | O portão único de remoção. |
| `internal/service` | Casos de uso. Dono das interfaces. |
| `internal/adapter/*` | Disco, comandos, Lixeira — e seus fakes. |
| `internal/report` | Saída em texto, JSON e Markdown. |
| `internal/tui` | Tela interativa. Arquitetura Elm, um arquivo por peça. |
| `internal/cli` | Comandos e composition root (`deps.go`). |

Nunca crie pacotes genéricos tipo `utils`, `helpers` ou `common`.

---

## Regras invioláveis

**O guard.** Nenhum caminho é removido sem passar por `internal/guard`. Mantenha
o pacote minúsculo e sem dependências além do `domain` — a garantia que ele
oferece vem de dar para auditá-lo de uma sentada, e isso se perde no instante em
que ele precisar de contexto de outros seis pacotes.

Ao mexer nele: **escreva o teste hostil primeiro**. A tabela em `guard_test.go` é
escrita de fora para dentro — não a partir do que o catálogo produz hoje, mas do
que jamais pode ser aceito, venha de onde vier.

**Nada de `sudo`.** Se um alvo exige root, a estratégia é `domain.ManualOnly` e a
CLI só exibe o comando. Não há exceção.

**Regras não se sobrepõem.** Dois alvos reivindicando o mesmo espaço não geram
erro visível — geram algo pior: o total prometido fica maior que o espaço que
existe. O teste `TestAlvosNaoSeSobrepoem` cobre isso. Ao adicionar uma regra que
casa com um glob amplo, atualize a lista de exclusão da regra de varredura
correspondente.

**Toda regra nova precisa de `What`, `Lose` e `Regen` preenchidos**, e de uma
`Strategy`. O teste do catálogo falha sem isso.

---

## Convenções de código

**Idioma.** Comentários, mensagens de erro, saída da CLI e nomes de teste em
**português**. Identificadores de código em inglês, seguindo o idioma da stdlib.

**Comentários explicam o porquê, não o quê.** Um comentário que parafraseia a
linha seguinte é ruído. Comente decisões, restrições do sistema operacional e
armadilhas — e prefira registrar o custo real de ter feito diferente, quando ele
for conhecido.

**Erros.** Sentinelas em `domain/errors.go`, embrulhados com `%w`, verificados
com `errors.Is`/`As`. Nunca compare erros por igualdade nem por texto.

**Contexto.** `context.Context` como primeiro parâmetro de tudo que faz I/O, e
propagado até o `exec.CommandContext`. A exceção documentada é o `Model` da TUI,
onde o Bubble Tea não oferece caminho melhor.

**Concorrência.** `errgroup` sempre com `SetLimit`. Um walker sem teto gasta mais
tempo em troca de contexto do que em syscall.

**Log.** `log/slog`, sempre em stderr — `stdout` carrega dados e precisa
continuar limpo para `mac-cleaner report --json | jq`.

**Construtores.** Injeção por parâmetro e functional options. Sem estado global,
sem `init()` com efeito colateral.

**Testes.** Table-driven com `t.Parallel()`. Nomes descrevem o comportamento
esperado, não o método (`TestRejeitaSymlinkQueEscapa`, não `TestCheck2`). Um
`t.Skip` que dispara na prática é um teste que não testa — prefira montar o
cenário de forma determinística.

**Medição de disco.** Sempre `st_blocks × 512`, nunca `st_size`. Deduplique por
inode. Pare na fronteira de volume.

---

## Ao adicionar uma regra ao catálogo

1. Escolha o arquivo por assunto: `dev.go`, `xcode.go`, `docker.go`, `system.go`,
   `apps.go`.
2. Preencha `What`, `Lose` e `Regen` como se estivesse explicando a alguém que
   vai decidir se aprova a remoção. É exatamente o que está acontecendo.
3. Prefira `domain.RunCommand` com o comando oficial da ferramenta. Só use
   `TrashTargets` quando não existir um.
4. Classifique o `Risk` com honestidade. `RiskSafe` significa "não custa nada
   perder" — na dúvida, `RiskReview`.
5. Rode `go test ./internal/catalog/` e confira o teste de sobreposição.
6. Rode `go test -tags integration ./internal/service/` para confirmar que o
   guard aceita os caminhos que a regra produz na máquina real.
