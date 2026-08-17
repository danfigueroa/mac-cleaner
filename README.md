# mac-cleaner

CLI para auditar o armazenamento de um Mac e recuperar espaço ocupado por caches
e resíduos regeneráveis — sem heurística opaca e sem `sudo`.

```
  mac-cleaner · 17.0 GB livres de 245 GB (93% ocupado)
  39.8 GB recuperáveis em 13 alvos

  FERRAMENTAS DE DESENVOLVIMENTO  20.6 GB
  ▸ [x]   14.2 GB  Xcode — símbolos de dispositivos iOS       regenerável
          Cópia dos símbolos de depuração de cada versão de iOS de cada
          aparelho que você já conectou.
          você perde: a capacidade de depurar naquele aparelho sem reprocessar
          ação: mover 3 caminhos para a Lixeira
    [ ]    2.8 GB  Xcode — simuladores de Preview do SwiftUI
    [ ]    1.5 GB  Go — cache de módulos                      regenerável
    [ ]    1.0 GB  nvm — versões de Node fora de uso          revisar

  APLICATIVOS  9.5 GB
    [ ]    6.2 GB  Claude Desktop — VMs de sessões locais     revisar
    [x]    3.2 GB  Apps Electron — caches internos
  ──────────────────────────────────────────────────────────────────────
  selecionado: 20.4 GB
  ↑/k mover · espaço marcar · a marcar tudo · enter limpar · q sair
```

---

## O problema

Um disco de máquina de desenvolvimento enche por acúmulo de coisas que ninguém
decidiu guardar: símbolos de depuração de versões de iOS que você não usa mais,
caches de cinco gerenciadores de pacote diferentes, imagens de Docker de projetos
encerrados, `node_modules` de repositórios que você clonou uma vez.

Nada disso é visível no Finder, e quase tudo é regenerável. O que falta não é
espaço — é saber **onde** ele foi parar e **quanto custa** recuperá-lo.

Ferramentas de limpeza comerciais resolvem isso com uma barra de progresso e um
botão "Limpar". Elas funcionam, mas não explicam o que estão apagando, e é
razoável não confiar num programa que quer acesso total ao seu disco sem dizer
o que vai fazer com ele.

## O que esta ferramenta faz de diferente

**Cada alvo declara três coisas antes de qualquer remoção**: o que é, o que você
perde e como aquilo volta. Não existe alvo sem essas três frases — há um teste
no catálogo que falha se alguém adicionar uma regra sem elas.

**A remoção prefere o comando oficial da própria ferramenta.** `docker image
prune`, `xcrun simctl delete unavailable`, `go clean -modcache`, `brew cleanup`.
Apagar diretórios por baixo dessas ferramentas libera os bytes mas corrompe o
estado interno delas, e a falha aparece dias depois, desconectada da causa. O
modcache do Go, por exemplo, é somente-leitura por design: um `rm -rf` falha na
metade dos arquivos.

**Quando não há comando oficial, vai para a Lixeira** — via `NSFileManager`, com
o metadado de "Colocar de Volta" do Finder. Não é um `mv ~/.Trash`, que deixaria
os arquivos lá sem nenhuma forma de saber de onde vieram.

**A CLI nunca usa `sudo`.** Alvos que exigem root são medidos, exibidos e
acompanhados do comando pronto para você colar. A ferramenta não escala
privilégio em nenhuma circunstância.

**Os números são honestos.** Mover para a Lixeira não libera espaço — o `df` não
muda um byte até você esvaziá-la. A interface separa "liberado agora" de "movido
para a Lixeira" em vez de somar os dois, porque somá-los prometeria um espaço
que não apareceria.

---

## Instalação

```sh
go install github.com/danfigueroa/mac-cleaner/cmd/mac-cleaner@latest
```

O binário vai para `$(go env GOPATH)/bin`. Se esse diretório não estiver no seu
`PATH`:

```sh
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc && exec zsh
```

A partir do repositório clonado:

```sh
make install     # instala em $(go env GOPATH)/bin
make build       # ou apenas compila em ./bin/mac-cleaner
```

### Requisitos

- **macOS** — a ferramenta é darwin-only por natureza: usa `statfs`/`lstat` para
  medir blocos, a API `NSFileManager` para a Lixeira e conhece a estrutura de
  `~/Library`.
- **Go 1.26+** para compilar. O binário resultante não tem dependências de runtime.
- **cgo habilitado** (o padrão) para a Lixeira nativa. Sem cgo, a ferramenta
  compila e funciona usando o Finder via `osascript`, só que mais devagar.

### Acesso Total ao Disco

Sem essa permissão, partes de `~/Library` respondem "permissão negada" e somem
da conta. A CLI detecta e avisa quantos caminhos ficaram ilegíveis — nesse caso
os totais são um **piso**, não um número exato:

```
Atenção: 17 caminhos não puderam ser lidos, então os totais acima
estão subestimados.
  Conceda Acesso Total ao Disco ao seu terminal em
  Ajustes do Sistema › Privacidade e Segurança › Acesso Total ao Disco.
```

Para conceder: **Ajustes do Sistema › Privacidade e Segurança › Acesso Total ao
Disco**, e adicione o seu terminal (Terminal, iTerm, Ghostty, VS Code…).

---

## Como rodar

### Tela interativa

```sh
mac-cleaner
```

Escaneia o disco, agrupa por categoria e abre a lista. Alvos de risco **seguro**
já vêm marcados; os demais, não. A linha sob o cursor mostra a explicação
completa e o comando exato que será executado.

| Tecla | Ação |
|---|---|
| `↑` `↓` ou `k` `j` | mover o cursor |
| `espaço` | marcar / desmarcar |
| `a` | marcar tudo que é removível |
| `n` | desmarcar tudo |
| `enter` | executar a limpeza |
| `q` `esc` `ctrl+c` | sair sem fazer nada |

Ao final, se algo foi para a Lixeira, a tela oferece esvaziá-la — que é o passo
que de fato devolve o espaço ao disco.

### Só auditar, sem remover nada

```sh
mac-cleaner report                 # relatório no terminal
mac-cleaner report --json | jq     # saída para script
mac-cleaner report --markdown      # documento para revisar ou versionar
```

O `report` nunca remove nada. Redirecionar a saída também funciona —
`mac-cleaner > auditoria.txt` detecta que não há terminal e cai no relatório em
texto em vez de cuspir códigos de escape.

### Limpeza não-interativa

```sh
mac-cleaner clean npm-cache go-buildcache    # alvos específicos, por ID
mac-cleaner clean --safe                     # tudo classificado como seguro
mac-cleaner clean --safe --yes --empty-trash # sem confirmação, e esvazia ao fim
mac-cleaner clean npm-cache --dry-run        # mostra o que faria
```

Os IDs são os que aparecem na segunda coluna do `mac-cleaner report`.

### Filtros

```sh
mac-cleaner --category dev                   # só ferramentas de desenvolvimento
mac-cleaner --min-size 500MB                 # omite alvos pequenos
mac-cleaner --big-files 2GB                  # o que conta como "arquivo grande"
mac-cleaner --stale 180d                     # idade de um projeto "parado"
mac-cleaner --verbose                        # log detalhado em stderr
```

Categorias: `dev`, `system`, `apps`, `projects`, `bigfiles`.

> A varredura completa leva ~8 s num disco cheio, porque as categorias
> `projects` e `bigfiles` percorrem o home inteiro. Qualquer combinação de
> `--category` sem essas duas pula a travessia profunda e responde em ~2 s.

### Códigos de saída

| Código | Significado |
|---|---|
| `0` | sucesso |
| `1` | erro |
| `2` | uso inválido |
| `3` | nada a limpar |
| `130` | interrompido com Ctrl-C |

---

## Níveis de risco

O que vem pré-marcado é decidido pelo catálogo — versionado e testado — e não
caso a caso:

| Nível | Significado | Pré-marcado |
|---|---|---|
| **seguro** | Cache puro, volta sozinho, sem custo perceptível | sim |
| **regenerável** | Volta, mas cobra tempo e banda de re-download | não |
| **revisar** | Pode conter algo que você quer manter | nunca |

Um padrão agressivo transformaria o Enter em reflexo. O dia em que isso apagasse
algo importante seria o dia em que a ferramenta perderia a confiança de quem a
usa.

---

## Catálogo

37 regras. Cada uma só aparece se a ferramenta correspondente existir na máquina.

### Ferramentas de desenvolvimento

| ID | Alvo | Como limpa |
|---|---|---|
| `npm-cache` | Cache global do npm | `npm cache clean --force` |
| `pnpm-store` | Pacotes órfãos no store | `pnpm store prune` |
| `yarn-cache` | Cache global do Yarn | `yarn cache clean` |
| `go-modcache` | Cache de módulos Go | `go clean -modcache` |
| `go-buildcache` | Cache de compilação Go | `go clean -cache` |
| `pip-cache` | Wheels do pip | `pip3 cache purge` |
| `homebrew-cache` | Downloads e versões antigas | `brew cleanup -s --prune=all` |
| `nuget-packages` | Pacotes NuGet | `dotnet nuget locals all --clear` |
| `gradle-caches` | Caches, wrappers e daemon | Lixeira |
| `maven-repo` | `~/.m2/repository` | Lixeira |
| `cocoapods-cache` | Cache e specs do CocoaPods | Lixeira |
| `swiftpm-cache` | Cache do Swift Package Manager | Lixeira |
| `pub-cache` | Pacotes Dart e Flutter | Lixeira |
| `cargo-registry` | Crates e índice do Cargo | Lixeira |
| `nvm-versions` | Versões de Node fora de uso | Lixeira |
| `xcode-deriveddata` | DerivedData | Lixeira |
| `xcode-devicesupport` | Símbolos de dispositivos iOS | Lixeira |
| `xcode-previews` | Simuladores de Preview do SwiftUI | Lixeira |
| `xcode-archives` | Archives de builds publicados | Lixeira |
| `coresimulator-caches` | Caches do CoreSimulator | Lixeira |
| `coresimulator-unavailable` | Simuladores órfãos | `xcrun simctl delete unavailable` |
| `docker-buildcache` | Cache de build | `docker builder prune -a -f` |
| `docker-containers` | Contêineres parados | `docker container prune -f` |
| `docker-images` | Imagens sem uso | `docker image prune -a -f` |
| `docker-volumes` | Volumes órfãos | `docker volume prune -f` |

`nvm-versions` preserva a versão ativa. As regras do Docker consultam
`docker system df` para reportar só o que o daemon realmente consegue liberar —
medir o diretório da VM daria um número que a limpeza não entrega.

### Sistema e aplicativos

| ID | Alvo | Como limpa |
|---|---|---|
| `user-caches` | `~/Library/Caches`, item a item | Lixeira |
| `user-logs` | `~/Library/Logs` | Lixeira |
| `installer-leftovers` | `~/Library/Updates` | Lixeira |
| `ios-backups` | Backups locais de iPhone/iPad | Lixeira |
| `electron-caches` | Caches internos de apps Chromium | Lixeira |
| `claude-vm-bundles` | VMs de sessões locais do Claude Desktop | Lixeira |
| `jetbrains-caches` | Índices e logs das IDEs JetBrains | Lixeira |
| `browser-caches` | Chrome, Brave, Firefox, Edge, Safari | Lixeira |
| `system-wallpapers` | Vídeos 4K de papel de parede | **exige sudo** |
| `system-caches` | `/Library/Caches` | **exige sudo** |

`electron-caches` é genérica: encontra `Cache`, `Code Cache`, `GPUCache`,
`CachedData` e `logs` em qualquer app Electron instalado, incluindo os que você
instalar amanhã.

### Dinâmicas

| ID | Alvo | Como limpa |
|---|---|---|
| `projetos-abandonados` | `node_modules`, `.next`, `.nuxt`, `.turbo` em projetos parados | Lixeira |
| `arquivos-grandes` | Arquivos soltos acima do limite | **só lista** |

`arquivos-grandes` nunca remove nada, e isso é deliberado. Eles vivem em
`Downloads` e `Documents`, e nada num `.zip` de 4 GB distingue lixo esquecido do
único backup de um projeto. Listar é útil; decidir por você, não.

---

## Tecnologias

| Tecnologia | Papel |
|---|---|
| **Go 1.26** | Linguagem. Binário único, sem runtime. |
| **[Cobra](https://github.com/spf13/cobra)** | Comandos, flags e completions de shell. |
| **[Bubble Tea](https://github.com/charmbracelet/bubbletea)** | TUI, arquitetura Elm. |
| **[Lipgloss](https://github.com/charmbracelet/lipgloss)** | Estilos com cores adaptativas (tema claro e escuro). |
| **`golang.org/x/sync/errgroup`** | Travessia paralela com limite e cancelamento. |
| **`golang.org/x/sys/unix`** | `lstat` e `statfs`. O pacote `syscall` está congelado desde o Go 1.4. |
| **cgo + Foundation** | `NSFileManager.trashItemAtURL` para a Lixeira nativa. |
| **`log/slog`** | Log estruturado, sempre em stderr. |
| **[golangci-lint v2](https://golangci-lint.run)** | 20 linters, incluindo `depguard` guardando as camadas. |
| **GitHub Actions** | CI em `macos-latest`. |

O `golangci-lint` é pinado num módulo `tools/` separado, via diretiva `tool` do
Go 1.24+. Isso mantém o `go.mod` do binário com apenas o que ele usa: quem roda
`go install` baixa Cobra e Bubble Tea, não as ~200 dependências do linter.

---

## Arquitetura

```
domain  ←  catalog  ←  service  ←  cli / tui
                          ↑
                       adapter
```

`domain` não importa nada além da stdlib. Ninguém importa `cli`. **A direção é
imposta pelo `depguard`**: importar na direção errada quebra o lint, não a
revisão de código.

```
mac-cleaner/
├── cmd/mac-cleaner/         main enxuto: sinais, código de saída
├── internal/
│   ├── domain/              tipos e regras de negócio, zero dependências
│   ├── catalog/             o conhecimento: quais diretórios são lixo e por quê
│   │   └── dynamic/         regras que varrem o disco para achar seus alvos
│   ├── guard/               o portão único de remoção
│   ├── service/             casos de uso; dono das interfaces
│   ├── adapter/
│   │   ├── osfs/            disco real (unix.Lstat, unix.Statfs)
│   │   ├── memfs/           sistema de arquivos falso, para testes
│   │   ├── cmdrunner/       execução de comandos externos
│   │   └── trash/           Lixeira via cgo, com fallback por osascript
│   ├── report/              saída em texto, JSON e Markdown
│   ├── tui/                 tela interativa (model/update/view/keys/styles)
│   └── cli/                 comandos e composition root
├── tools/                   módulo isolado só para o linter
└── .golangci.yml            18 linters + as regras de camada
```

Interfaces só existem onde há uma segunda implementação real, e são declaradas no
pacote que as consome — não num pacote `port/` central, que não é como Go organiza
abstrações. São quatro: sistema de arquivos, executor de comandos, Lixeira e
volume.

### O guard

Nenhum caminho é removido sem passar por `internal/guard`. Ele é o menor pacote
do repositório de propósito: sem dependências além do `domain`, sem estado, sem
configuração externa. A ideia é que dê para ler o arquivo inteiro numa sentada e
concluir com segurança o que ele deixa passar.

- **Território do usuário** (`Documents`, `Desktop`, `Downloads`, `Development`,
  `Pictures`…) é proibido, com uma exceção estreita: diretórios cujo conteúdo é
  sempre reconstruível por um comando — `node_modules`, `.next`, `.nuxt`,
  `.turbo`. `dist`, `build` e `target` ficaram de fora porque, em algum projeto
  real, cada um deles é uma pasta escrita à mão.
- **Diretórios de credenciais** (`.ssh`, `.gnupg`, `.aws`, `.kube`, Keychains,
  Mail, Mensagens) não têm exceção alguma.
- **Diretórios estruturais** (`~/Library`, `~/Library/Caches`, `~/go`, `~/.nvm`)
  não podem ser removidos; o conteúdo deles, sim.
- Todo caminho passa por `EvalSymlinks` e precisa continuar dentro do home. Um
  link em `~/Library/Caches` apontando para fora é rejeitado.
- O plano inteiro é validado **antes** da primeira remoção: o usuário aprovou um
  conjunto, não uma sequência.

---

## Desenvolvimento

```sh
make            # lista os alvos disponíveis
make build      # compila em ./bin/mac-cleaner
make install    # instala em $(go env GOPATH)/bin
make run ARGS="report --json"
make test       # testes
make test-race  # com detector de corrida
make cover      # relatório de cobertura no navegador
make lint       # golangci-lint (versão pinada em tools/)
make fmt        # gofumpt + gci
make tidy       # arruma os dois módulos
```

### Testes

50 testes, todos table-driven onde faz sentido, com `t.Parallel()`. Os fakes são
escritos à mão — as interfaces têm um ou dois métodos, e um framework de mock
custaria mais para ler do que eles.

Os mais importantes:

- **`internal/guard`** — 50 caminhos hostis que *têm* que ser rejeitados. Escrito
  de fora para dentro: não a partir do que o catálogo produz hoje, mas do que
  jamais deve ser aceito, venha de onde vier.
- **`internal/catalog`** — falha se duas regras reivindicarem o mesmo espaço.
  Sobreposição não dá erro visível; dá algo pior: o total prometido fica maior
  que o espaço que existe para liberar.
- **`internal/service`** — `memfs` com hardlinks, volumes distintos e diretórios
  sem permissão, cobrindo os três detalhes que fazem a medição estar certa.

### Testes de integração

Rodam contra a máquina real, atrás de uma tag, fora do CI:

```sh
go test -tags integration ./internal/adapter/osfs/   # compara a medição com `du`
go test -tags integration ./internal/service/        # guard × catálogo real
go test -tags integration ./internal/adapter/trash/  # move um arquivo de verdade
```

O primeiro é o que mantém a ferramenta honesta: divergência acima de 5% em
relação ao `du` indica bug na contagem de blocos ou na deduplicação de hardlink.
O segundo confirma que o guard aceita todos os caminhos que o catálogo produz
nesta máquina — os testes unitários das duas metades não respondem essa pergunta.

---

## Detalhes que decidem se os números são reais

**Medição.** Soma `st_blocks × 512` (espaço alocado), não `st_size` (tamanho
aparente): um arquivo esparso de 10 GB pode ocupar 4 KB. Deduplica por inode,
porque pnpm e Homebrew usam hardlink em escala. Para na fronteira de volume, para
não descer pelos firmlinks do macOS e medir o sistema operacional inteiro achando
que é cache do usuário.

**Espaço livre, não espaço usado.** O cabeçalho lidera com o espaço livre porque
é o único número que bate exatamente com o `df`. No APFS o `statfs` devolve
`f_bfree` igual a `f_bavail`, e o `df` do macOS calcula a coluna "Used" por outra
via, descontando espaço purgável e snapshots — não dá para reproduzir os dois
números ao mesmo tempo. A porcentagem exibida bate com a coluna Capacity do `df`.

**Uma travessia, não duas.** As duas regras dinâmicas compartilham uma varredura
do home. Não é microotimização: com uma travessia por regra, medir este catálogo
passou de 3 para 94 segundos numa máquina real — e não por dobrar o trabalho, mas
porque varreduras profundas concorrentes com as 35 regras estáticas transformam
leituras sequenciais em disputa por I/O. A soma isolada das partes era 29 s;
juntas, 94 s. Com a travessia unificada: 8 s.

**Concorrência com teto.** `errgroup` com limite igual ao dobro dos núcleos. Um
walker sem limite abre uma goroutine por diretório, e a máquina passa a gastar
mais tempo em troca de contexto do que em syscall — o limite é o que torna o scan
rápido, não o contrário.

**Ctrl-C cancela de verdade.** O contexto é propagado até o `exec.CommandContext`
dos comandos externos. Sem isso, interromper a CLI devolveria o terminal mas
deixaria um `docker image prune` rodando órfão.

---

## Limitações conhecidas

- **Só macOS.** Não é acidente de portabilidade: a ferramenta conhece a estrutura
  de `~/Library`, usa a API de Lixeira do sistema e mede blocos com `statfs`.
- **Sem Acesso Total ao Disco os totais são um piso.** A CLI avisa, mas não tem
  como contornar — é uma restrição de TCC do sistema.
- **Não há cache de varredura.** Foi uma decisão: depois que a travessia
  unificada baixou o scan para ~8 s, um cache mostraria números velhos exatamente
  no pior momento, logo depois de você limpar. `--category` dá o caminho rápido.
- **Deduplicação de hardlink vale por alvo.** Dois alvos distintos que
  compartilhem o mesmo inode são contados duas vezes — o mesmo comportamento de
  rodar `du` separadamente em cada um.
- **A Lixeira acumula.** Mover para lá não libera espaço. A ferramenta avisa e
  oferece esvaziar, mas esvaziar remove também o que já estava lá por outros
  motivos.

## Licença

[MIT](LICENSE).
