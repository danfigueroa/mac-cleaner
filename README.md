# mac-cleaner

A CLI that audits macOS storage and reclaims space taken by caches and
regenerable leftovers — with no opaque heuristics and no `sudo`.

```
  mac-cleaner · 17.0 GB free of 245 GB (93% used)
  39.8 GB reclaimable across 13 targets

  DEVELOPER TOOLING  20.6 GB
  ▸ [x]   14.2 GB  Xcode — iOS device symbols                 regenerable
          Debug symbols for every iOS version of every device you have
          plugged in. Xcode keeps one set per version.
          you lose: the ability to debug on that device without reprocessing
          action: move 3 paths to the Trash
    [ ]    2.8 GB  Xcode — SwiftUI Preview simulators
    [ ]    1.5 GB  Go — module cache                          regenerable
    [ ]    1.0 GB  nvm — unused Node versions                 review

  APPLICATIONS  9.5 GB
    [ ]    6.2 GB  Claude Desktop — local session VMs         review
    [x]    3.2 GB  Electron apps — internal caches
  ──────────────────────────────────────────────────────────────────────
  selected: 20.4 GB
  ↑/k move · space toggle · a select all · enter clean · q quit
```

> The interface is in Portuguese; this document is in English. See
> [Language](#language) for why.

---

## Table of contents

- [The problem](#the-problem)
- [What makes this different](#what-makes-this-different)
- [Installation](#installation)
- [Usage](#usage)
- [Risk levels](#risk-levels)
- [Rule catalog](#rule-catalog)
- [Stack](#stack)
- [Project structure](#project-structure)
- [Architecture](#architecture)
- [The guard](#the-guard)
- [Development](#development)
- [Implementation details that decide whether the numbers are real](#implementation-details-that-decide-whether-the-numbers-are-real)
- [Known limitations](#known-limitations)

---

## The problem

A developer machine fills up by accumulating things nobody decided to keep:
debug symbols for iOS versions you no longer target, caches from five different
package managers, Docker images from projects that shipped a year ago,
`node_modules` from repos you cloned once.

None of it is visible in Finder, and almost all of it is regenerable. What's
missing isn't space — it's knowing **where** it went and **what it costs** to
get it back.

Commercial cleaners solve this with a progress bar and a "Clean" button. They
work, but they don't explain what they're deleting, and it is reasonable not to
trust a program that wants full disk access without saying what it will do with
it.

## What makes this different

**Every target explains itself.** Each rule declares what it is, what you lose
and how it comes back. There is a test in the catalog that fails if anyone adds
a rule without those three sentences.

**Removal prefers the tool's own official command.** `docker image prune`,
`xcrun simctl delete unavailable`, `go clean -modcache`, `brew cleanup`. Deleting
directories underneath those tools frees the bytes but corrupts their internal
state, and the failure surfaces days later, disconnected from the cause. Go's
module cache, for instance, is read-only by design: an `rm -rf` fails on half
the files.

**When there is no official command, it goes to the Trash** — through
`NSFileManager`, with Finder's "Put Back" metadata. Not a `mv ~/.Trash`, which
would leave the files there with no way to know where they came from.

**The CLI never uses `sudo`.** Targets that require root are measured, displayed
and accompanied by the exact command for you to run. The tool does not escalate
privileges under any circumstance.

**The numbers are honest.** Moving to the Trash does not free space — `df`
doesn't move a byte until you empty it. The interface reports "freed now" and
"moved to Trash" separately instead of summing them, because summing would
promise space that never shows up.

---

## Installation

```sh
go install github.com/danfigueroa/mac-cleaner/cmd/mac-cleaner@latest
```

The binary lands in `$(go env GOPATH)/bin`. If that isn't on your `PATH`:

```sh
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc && exec zsh
```

From a cloned repository:

```sh
make install     # installs into $(go env GOPATH)/bin
make build       # or just compile to ./bin/mac-cleaner
```

### Requirements

- **macOS.** The tool is darwin-only by nature: it uses `statfs`/`lstat` to
  measure allocated blocks, the `NSFileManager` API for the Trash, and it knows
  the layout of `~/Library`.
- **Go 1.26+** to build. The resulting binary has no runtime dependencies.
- **cgo enabled** (the default) for the native Trash. Without cgo it still
  compiles and works, falling back to Finder via `osascript` — just slower.

### Full Disk Access

Without this permission parts of `~/Library` answer "operation not permitted"
and silently drop out of the totals. The CLI detects this and reports how many
paths were unreadable — when that happens, the totals are a **floor**, not an
exact figure:

```
Atenção: 17 caminhos não puderam ser lidos, então os totais acima
estão subestimados.
```

To grant it: **System Settings › Privacy & Security › Full Disk Access**, then
add your terminal (Terminal, iTerm, Ghostty, VS Code…).

---

## Usage

### Interactive screen

```sh
mac-cleaner
```

Scans the disk, groups by category and opens the list. Targets classified as
**safe** come pre-selected; everything else does not. The row under the cursor
expands with the full explanation and the exact command that will run.

| Key | Action |
|---|---|
| `↑` `↓` or `k` `j` | move the cursor |
| `space` | toggle selection |
| `a` | select everything removable |
| `n` | deselect everything |
| `enter` | run the cleanup |
| `q` `esc` `ctrl+c` | quit without doing anything |

At the end, if anything went to the Trash, the screen offers to empty it — the
step that actually returns the space to the disk.

### Audit only, remove nothing

```sh
mac-cleaner report                 # report in the terminal
mac-cleaner report --json | jq     # machine-readable output
mac-cleaner report --markdown      # document to review or version
```

`report` never removes anything. Redirecting also works —
`mac-cleaner > audit.txt` detects the absence of a terminal and falls back to
the text report instead of dumping escape codes into a file.

### Non-interactive cleanup

```sh
mac-cleaner clean npm-cache go-buildcache    # specific targets, by ID
mac-cleaner clean --safe                     # everything classified as safe
mac-cleaner clean --safe --yes --empty-trash # no prompt, empties at the end
mac-cleaner clean npm-cache --dry-run        # show what it would do
```

The IDs are the ones in the second column of `mac-cleaner report`.

### Filters

```sh
mac-cleaner --category dev                   # developer tooling only
mac-cleaner --min-size 500MB                 # hide small targets
mac-cleaner --big-files 2GB                  # what counts as a "big file"
mac-cleaner --stale 180d                     # age of a "stale" project
mac-cleaner --verbose                        # detailed log on stderr
```

Categories: `dev`, `system`, `apps`, `projects`, `bigfiles`.

> A full scan takes ~8s on a full disk, because the `projects` and `bigfiles`
> categories walk the entire home directory. Any `--category` combination
> without those two skips the deep traversal and answers in ~2s.

### Exit codes

| Code | Meaning |
|---|---|
| `0` | success |
| `1` | error |
| `2` | invalid usage |
| `3` | nothing to clean |
| `130` | interrupted with Ctrl-C |

---

## Risk levels

What comes pre-selected is decided by the catalog — versioned and tested — not
case by case:

| Level | Meaning | Pre-selected |
|---|---|---|
| **safe** | Pure cache, comes back on its own, no noticeable cost | yes |
| **regenerable** | Comes back, but costs time and bandwidth to re-download | no |
| **review** | May contain something you want to keep | never |

An aggressive default would turn Enter into a reflex. The day that deletes
something important is the day the tool loses the trust of the person using it.

---

## Rule catalog

37 rules. Each one only appears if the corresponding tool exists on the machine.

### Developer tooling

| ID | Target | How it cleans |
|---|---|---|
| `npm-cache` | npm global cache | `npm cache clean --force` |
| `pnpm-store` | Orphaned packages in the store | `pnpm store prune` |
| `yarn-cache` | Yarn global cache | `yarn cache clean` |
| `go-modcache` | Go module cache | `go clean -modcache` |
| `go-buildcache` | Go build cache | `go clean -cache` |
| `pip-cache` | pip wheels | `pip3 cache purge` |
| `homebrew-cache` | Downloads and superseded versions | `brew cleanup -s --prune=all` |
| `nuget-packages` | NuGet packages | `dotnet nuget locals all --clear` |
| `gradle-caches` | Caches, wrappers and daemon | Trash |
| `maven-repo` | `~/.m2/repository` | Trash |
| `cocoapods-cache` | CocoaPods cache and specs | Trash |
| `swiftpm-cache` | Swift Package Manager cache | Trash |
| `pub-cache` | Dart and Flutter packages | Trash |
| `cargo-registry` | Cargo crates and index | Trash |
| `nvm-versions` | Unused Node versions | Trash |
| `xcode-deriveddata` | DerivedData | Trash |
| `xcode-devicesupport` | iOS device debug symbols | Trash |
| `xcode-previews` | SwiftUI Preview simulators | Trash |
| `xcode-archives` | Archives of shipped builds | Trash |
| `coresimulator-caches` | CoreSimulator caches | Trash |
| `coresimulator-unavailable` | Orphaned simulators | `xcrun simctl delete unavailable` |
| `docker-buildcache` | Build cache | `docker builder prune -a -f` |
| `docker-containers` | Stopped containers | `docker container prune -f` |
| `docker-images` | Unused images | `docker image prune -a -f` |
| `docker-volumes` | Dangling volumes | `docker volume prune -f` |

`nvm-versions` preserves the active version. The Docker rules query
`docker system df` so they report only what the daemon can actually reclaim —
measuring the VM's disk image would produce a number the cleanup never delivers.

### System and applications

| ID | Target | How it cleans |
|---|---|---|
| `user-caches` | `~/Library/Caches`, entry by entry | Trash |
| `user-logs` | `~/Library/Logs` | Trash |
| `installer-leftovers` | `~/Library/Updates` | Trash |
| `ios-backups` | Local iPhone/iPad backups | Trash |
| `electron-caches` | Internal caches of Chromium-based apps | Trash |
| `claude-vm-bundles` | Claude Desktop local session VMs | Trash |
| `jetbrains-caches` | JetBrains IDE indexes and logs | Trash |
| `browser-caches` | Chrome, Brave, Firefox, Edge, Safari | Trash |
| `system-wallpapers` | 4K wallpaper videos | **requires sudo** |
| `system-caches` | `/Library/Caches` | **requires sudo** |

`electron-caches` is generic: it finds `Cache`, `Code Cache`, `GPUCache`,
`CachedData` and `logs` inside any installed Electron app, including ones you
install tomorrow.

### Dynamic

| ID | Target | How it cleans |
|---|---|---|
| `projetos-abandonados` | `node_modules`, `.next`, `.nuxt`, `.turbo` in stale projects | Trash |
| `arquivos-grandes` | Loose files above the threshold | **lists only** |

`arquivos-grandes` never removes anything, deliberately. Those files live in
`Downloads` and `Documents`, and nothing about a 4 GB `.zip` distinguishes
forgotten junk from the only backup of a project. Listing is useful; deciding
for you is not.

---

## Stack

Every dependency here earns its place; there is no framework doing work the
standard library already does well.

### Language and runtime

| | |
|---|---|
| **Go 1.26** | Single static binary, no runtime to install. Goroutines make the parallel disk walk natural, and cgo gives direct access to the macOS Foundation API for the Trash. |
| **cgo + Objective-C** | `internal/adapter/trash` calls `NSFileManager.trashItemAtURL` through a `.m` file compiled alongside the Go code. It's the only way to get real Trash semantics, including "Put Back". |

### Runtime dependencies

The shipped binary depends on exactly two direct modules:

| Module | Role | Why this one |
|---|---|---|
| **[spf13/cobra](https://github.com/spf13/cobra)** | Commands, flags, help, shell completions | The de facto standard in the Go ecosystem (kubectl, gh, docker, hugo). Any Go developer reads it without a manual, and it generates completions for free. Kong and urfave/cli are good; Cobra is the one nobody has to learn. |
| **[charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea)** | Interactive TUI | Enforces the Elm architecture — a single `Update` function owns all state transitions — which makes the screen testable without a terminal. `internal/tui/model_test.go` drives it purely by sending messages. |

Pulled in transitively, but worth naming because the code uses them directly:

| Module | Role |
|---|---|
| **[charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss)** | Adaptive colors, so the palette stays legible in light and dark terminals |
| **`golang.org/x/sync/errgroup`** | Bounded parallel traversal with context cancellation |
| **`golang.org/x/sys/unix`** | `lstat` and `statfs`. The stdlib `syscall` package has been frozen since Go 1.4; `x/sys` is the maintained path for new code |

### Standard library, used deliberately

| Package | Where and why |
|---|---|
| `log/slog` | Structured logging, always to stderr so stdout stays pipeable for `report --json \| jq` |
| `context` | First parameter of everything doing I/O, propagated down to `exec.CommandContext` so Ctrl-C actually kills a running `docker prune` |
| `text/tabwriter` | Column alignment in the text report — misaligned numbers are exactly the kind of thing that stops people from reading them |
| `encoding/json` | The `--json` report and parsing `simctl list --json` / `docker system df` |
| `errors` | Sentinels wrapped with `%w`, `errors.Is`/`As`, and `errors.Join` to accumulate per-path failures |
| `testing` | Table-driven tests with `t.Parallel()`. Fakes are hand-written; there is no mock framework |

### Development tooling

| Tool | Role |
|---|---|
| **[golangci-lint v2](https://golangci-lint.run)** | 20 linters, including `depguard` enforcing the layer boundaries |
| **gofumpt + gci** | Formatting and import grouping, run through `make fmt` |
| **GitHub Actions** | CI on `macos-latest`: build, race-enabled tests, lint |

golangci-lint is pinned in a **separate `tools/` module** using the Go 1.24+
`tool` directive. This keeps the binary's `go.mod` to what it actually uses:
`go install` downloads Cobra and Bubble Tea, not the linter's ~200 transitive
dependencies. CI and local machines run the exact same linter version.

---

## Project structure

```
mac-cleaner/
├── cmd/
│   └── mac-cleaner/
│       └── main.go              36   Thin entrypoint. Signal handling, exit codes.
│                                     Contains no logic — see "Architecture".
├── internal/
│   ├── domain/                        Business types. Imports nothing but stdlib.
│   │   ├── bytes.go            115   Bytes type with decimal formatting and parsing
│   │   ├── env.go               67   The machine a rule is evaluated against
│   │   ├── errors.go            36   Sentinel errors wrapped across every layer
│   │   ├── finding.go          149   Finding, Result, Report, Volume, grouping
│   │   ├── plan.go              96   Plan, PlanItem, Outcome, Summary
│   │   ├── preview.go           46   Renders the exact command shown before acting
│   │   ├── risk.go              90   Risk levels and categories
│   │   └── rule.go             120   Rule, Target, and the cleanup strategies
│   │
│   ├── catalog/                       The knowledge: which directories are junk, and why.
│   │   ├── catalog.go           81   Registry, ID lookup, category filtering
│   │   ├── targets.go          151   Target-building helpers (paths, globs, exclusions)
│   │   ├── dev.go              272   15 rules: package managers and toolchains
│   │   ├── xcode.go            161   6 rules: Xcode and iOS simulators
│   │   ├── docker.go           139   4 rules: images, containers, volumes, build cache
│   │   ├── apps.go              83   4 rules: Electron, Claude, JetBrains, browsers
│   │   ├── system.go           104   6 rules: macOS caches, logs, sudo-only targets
│   │   └── dynamic/                   Rules that must scan the disk to find their targets
│   │       ├── survey.go       139   One shared traversal feeding both rules
│   │       ├── walk.go         116   Walk limits, skip lists, block-accurate sizing
│   │       ├── projects.go      89   Stale-project detection
│   │       └── bigfiles.go      36   Loose large files (report-only)
│   │
│   ├── guard/                         The single gate every removal passes through.
│   │   └── guard.go            291   Path validation. No dependencies beyond domain.
│   │
│   ├── service/                       Use cases. Owns the interfaces it consumes.
│   │   ├── filesystem.go        59   FileSystem and Volumer interfaces, FileInfo
│   │   ├── scanner.go          257   Scanner: measures the catalog in parallel
│   │   ├── walk.go             171   Tree traversal: inode dedup, volume boundary
│   │   ├── planner.go           37   Turns a selection into an executable Plan
│   │   └── cleaner.go          190   Executes a Plan after validating it whole
│   │
│   ├── adapter/                       Implementations of the service interfaces.
│   │   ├── osfs/
│   │   │   ├── osfs.go         101   Real disk via unix.Lstat and unix.Statfs
│   │   │   └── time_darwin.go   18   Timespec conversion, isolated per-arch
│   │   ├── memfs/
│   │   │   └── memfs.go        221   In-memory fake: hardlinks, volumes, EACCES
│   │   ├── cmdrunner/
│   │   │   └── cmdrunner.go    127   External commands; Query (read) vs Run (write)
│   │   └── trash/
│   │       ├── trash_darwin.h   11   C header for the Objective-C bridge
│   │       ├── trash_darwin.m   40   NSFileManager.trashItemAtURL
│   │       ├── trash_cgo.go     49   cgo binding (build tag: darwin && cgo)
│   │       ├── trash_nocgo.go   52   osascript fallback (build tag: !cgo)
│   │       └── empty.go         27   Empties the Trash through Finder
│   │
│   ├── report/                        Output rendering.
│   │   ├── text.go             135   Terminal report with tabwriter alignment
│   │   ├── json.go              99   Stable DTO, decoupled from internal types
│   │   └── markdown.go          77   Long-form document
│   │
│   ├── tui/                           Interactive screen. Elm architecture.
│   │   ├── model.go            191   State, dependencies, row building
│   │   ├── update.go           185   All state transitions and async commands
│   │   ├── view.go             185   Rendering
│   │   ├── keys.go              36   Key bindings and help strings
│   │   └── styles.go            37   Adaptive palette and spinner
│   │
│   └── cli/                           Commands and composition root.
│       ├── cli.go               73   Execute, exit-code mapping, user messages
│       ├── root.go              98   Root command, global flags, help text
│       ├── report.go            77   `report` subcommand
│       ├── clean.go            208   `clean` subcommand, plan preview, confirmation
│       ├── tui.go               76   Launches the TUI; falls back when not a TTY
│       ├── rules.go             27   Assembles static + dynamic rules per run
│       ├── deps.go              66   Composition root — the only place wiring happens
│       ├── flagtypes.go         62   Duration flag that accepts "90d"
│       ├── confirm.go           31   Yes/no prompt defaulting to no
│       ├── logging.go           25   slog setup on stderr
│       └── version.go           68   Version from ldflags or build info
│
├── tools/
│   └── go.mod                  226   Isolated module pinning golangci-lint
│
├── .github/workflows/ci.yml     37   Build, race tests and lint on macos-latest
├── .golangci.yml               160   20 linters + the layer rules
├── Makefile                     49   Development targets
├── CLAUDE.md                   275   Repository conventions (in Portuguese)
├── README.md                   719   This file
└── LICENSE                      21   MIT
```

### Why the folders are split this way

**`cmd/` holds only the entrypoint.** Everything else lives under `internal/`,
which the Go toolchain refuses to let external modules import. That isn't
bureaucracy: it means the entire API surface of this project is one binary, so
no package here has to be designed for external consumers.

**`domain/` is the base and imports nothing.** It's split by concept rather than
by "types.go" — `risk.go` holds the risk classification and its meaning,
`plan.go` holds everything about executing an approved selection. When you go
looking for why safe targets are pre-selected, the file name tells you where.

**`catalog/` is split by subject, not by size.** Someone adding a rule for a new
package manager opens `dev.go`; someone fixing Xcode paths opens `xcode.go`.
`targets.go` holds the shared helpers so the rule files stay declarative — they
read as data, which is what they are.

**`catalog/dynamic/` is a separate package** because those rules work differently
from the rest: they discover targets by walking the disk instead of naming known
paths, and they share a single traversal. Keeping them apart makes that
difference structural rather than a comment.

**`guard/` is one file, on purpose.** The guarantee it provides comes from being
auditable in one sitting. A guard spread over six files, importing half the
project, would still pass its tests and be worth much less.

**`service/` owns its interfaces.** `filesystem.go` declares what the scanner
needs from a disk; `osfs` and `memfs` implement it. The interface lives with the
consumer, which is how Go does abstraction — not in a central `port/` package.

**`adapter/` mirrors those interfaces one directory per implementation**, with
the test fake (`memfs`) sitting as a peer of the real one (`osfs`) rather than
hidden in a `testdata` corner. They're both implementations; neither is special.

**`tui/` follows the Elm split** the Bubble Tea framework expects: state,
transitions, rendering, input and styling in separate files. Mixing them is how
Bubble Tea code becomes unreadable at around 500 lines.

**`cli/` is the composition root** and the only package allowed to know every
other one. `deps.go` is the single place where a concrete type is chosen for an
interface. Nothing imports `cli` — enforced by the linter.

---

## Architecture

```
domain  ←  catalog  ←  service  ←  cli / tui
                          ↑
                       adapter
```

Read the arrows as "is imported by". `domain` imports nothing but the standard
library. Nobody imports `cli`.

**This is enforced by `depguard` in `.golangci.yml`**, not by convention.
Importing in the wrong direction breaks `make lint`, not a code review. The
rules are:

| Package | May import |
|---|---|
| `internal/domain` | stdlib only |
| `internal/catalog` | stdlib, `domain` |
| `internal/guard` | stdlib, `domain` |
| everything else | anything except `internal/cli` |

You can verify the rule is real rather than decorative:

```sh
echo 'package domain
import "github.com/spf13/cobra"
var _ = cobra.Command{}' > internal/domain/violation.go
make lint    # import 'github.com/spf13/cobra' is not allowed from list 'domain'
rm internal/domain/violation.go
```

### Interfaces

Interfaces exist only where there is a genuine second implementation, and they
are declared in the package that consumes them. There are four:

| Interface | Declared in | Implementations |
|---|---|---|
| `FileSystem` | `service/filesystem.go` | `osfs` (real disk), `memfs` (tests) |
| `Volumer` | `service/filesystem.go` | `osfs` |
| `Trasher` | `service/cleaner.go` | `trash` (cgo), `trash` (osascript), test fake |
| `Runner` | `service/cleaner.go` | `cmdrunner`, test fake |

Everything else is a concrete struct. A textbook hexagonal layout would put all
of these in a central `port/` package — that inverts Go's convention, where the
consumer declares the interface, and produces Java with different braces.

### Dependency injection

Constructor injection with functional options. No global state, no `init()` with
side effects:

```go
scanner := service.NewScanner(filesystem, volumer,
    service.WithConcurrency(16),
    service.WithLogger(logger),
)
```

The single exception is `log/slog`'s default logger, set once in `cli/logging.go`
— logging is cross-cutting infrastructure, and threading it through every
signature purely to avoid a default would pollute the API for no gain. Services
accept an injected `*slog.Logger` and fall back to the default when given nil.

---

## The guard

No path is removed without passing through `internal/guard`. It is the smallest
package in the repository on purpose: no dependencies beyond `domain`, no state,
no external configuration. The idea is that you can read the whole file in one
sitting and safely conclude what it lets through — a guarantee lost the moment
it needs context from six other packages.

Three lists organize it:

**Secret trees** — no exception whatsoever: `.ssh`, `.gnupg`, `.aws`, `.kube`,
`.config`, `.password-store`, `.docker`, Keychains, Mail, Messages, Safari,
Photos, iCloud Drive, Trash.

**Project trees** — the user's own work: `Documents`, `Desktop`, `Downloads`,
`Development`, `Projects`, `Pictures`, `Movies`, `Music`, `Applications`,
`Sites`. Forbidden, with one narrow exception: directories whose contents are
always reconstructible by a command — `node_modules`, `.next`, `.nuxt`,
`.turbo`. `dist`, `build` and `target` were deliberately left out, because in
some real project each of those is a hand-written folder.

**Structural roots** — `~/Library`, `~/Library/Caches`, `~/go`, `~/.nvm` and
friends. They cannot be removed; their contents can.

On top of that:

- Paths must be absolute and already normalized. `filepath.Clean` output must
  equal the input, which catches both traversal and malformed globs.
- Every path goes through `EvalSymlinks` and must still resolve inside the home
  directory. A link in `~/Library/Caches` pointing outside is rejected.
- The home directory itself is resolved at construction time. On macOS `/var` is
  a symlink to `/private/var`, so any home under `/var` would make every
  resolved path look like it escaped — and legitimate targets would be rejected.
- The **entire plan is validated before the first removal**. The user approved a
  set, not a sequence: if the fifth item violates the guard, the first must not
  have been deleted.

`guard_test.go` is written from the outside in — not from what the catalog
produces today, but from what must never be accepted, regardless of where it
comes from. That inversion is what keeps it meaningful when someone adds a new
rule a year from now.

---

## Development

```sh
make            # list available targets
make build      # compile to ./bin/mac-cleaner
make install    # install into $(go env GOPATH)/bin
make run ARGS="report --json"
make test       # tests
make test-race  # with the race detector
make cover      # coverage report in the browser
make lint       # golangci-lint, version pinned in tools/
make fmt        # gofumpt + gci
make tidy       # tidy both modules
```

### Tests

50 test functions, table-driven where it fits, with `t.Parallel()`. Fakes are
hand-written — the interfaces have one or two methods each, and a mock framework
would cost more to read than they do.

The ones that matter most:

- **`internal/guard`** — a hostile table of paths that *must* be rejected, plus
  the symlink-escape and home-behind-a-symlink scenarios.
- **`internal/catalog`** — fails if two rules claim the same space. Overlap
  produces no visible error; it produces something worse — a promised total
  larger than the space that exists to free.
- **`internal/service`** — `memfs` with hardlinks, distinct devices and
  permission-denied directories, covering the three details that make the
  measurement correct.
- **`internal/tui`** — drives the state machine by messages: pre-selection by
  risk, toggling, and that Enter builds a plan containing exactly what is
  checked.

### Integration tests

These run against the real machine and sit behind a build tag, outside CI:

```sh
go test -tags integration ./internal/adapter/osfs/   # compares measurement to `du`
go test -tags integration ./internal/service/        # guard × real catalog
go test -tags integration ./internal/adapter/trash/  # moves a real file
```

The first is what keeps the tool honest: a divergence above 5% against `du`
indicates a bug in block counting or hardlink deduplication. The second confirms
the guard accepts every path the catalog produces on this machine — a question
neither half's unit tests answer.

---

## Implementation details that decide whether the numbers are real

**Measurement.** Sums `st_blocks × 512` (allocated space), not `st_size`
(apparent size): a sparse 10 GB file can occupy 4 KB. Deduplicates by inode,
because pnpm and Homebrew use hardlinks at scale. Stops at volume boundaries, so
it doesn't descend through macOS firmlinks and measure the whole operating
system thinking it's user cache.

**Free space, not used space.** The header leads with free space because it's
the only number that matches `df` exactly. On APFS, `statfs` returns `f_bfree`
equal to `f_bavail`, and macOS `df` computes its "Used" column through a
different path that discounts purgeable space and snapshots — there is no way to
reproduce both numbers from `statfs`. The displayed percentage does match `df`'s
Capacity column.

**One traversal, not two.** The two dynamic rules share a single walk of the
home directory. This is not micro-optimization: with one traversal per rule,
measuring this catalog went from 3 to 94 seconds on a real machine — not because
the work doubled, but because deep concurrent scans competing with the 35 static
rules turn sequential reads into I/O contention. Measured in isolation the parts
summed to 29s; together they were 94s. With the shared traversal: 8s.

**Bounded concurrency.** `errgroup` with a limit of twice the CPU count, and a
non-blocking semaphore inside the walker: with no slot free, recursion continues
on the same goroutine. An unbounded walker opens one goroutine per directory and
the machine spends more time context-switching than in syscalls.

**Ctrl-C actually cancels.** The context reaches `exec.CommandContext` for
external commands. Without that, interrupting the CLI returns your prompt but
leaves a `docker image prune` running orphaned.

---

## Known limitations

- **macOS only.** Not a portability accident: the tool knows the `~/Library`
  layout, uses the system Trash API and measures blocks through `statfs`.
- **Without Full Disk Access the totals are a floor.** The CLI says so, but
  cannot work around it — it's a TCC restriction.
- **There is no scan cache.** A deliberate call: once the shared traversal
  brought the scan down to ~8s, a cache would show stale numbers at exactly the
  worst moment, right after you clean. `--category` provides the fast path.
- **Hardlink deduplication is per target.** Two distinct targets sharing an inode
  are counted twice — the same behavior as running `du` separately on each.
- **The Trash accumulates.** Moving there doesn't free space. The tool warns and
  offers to empty it, but emptying also removes whatever was already in there
  for other reasons.

---

## Language

Source comments, error messages, CLI output and test names are in Portuguese;
this README is in English. The split is intentional: the interface serves the
person running it, and the README serves whoever finds the repository.

`CLAUDE.md` documents this convention for contributors.

---

## License

[MIT](LICENSE).
