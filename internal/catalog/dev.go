package catalog

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
)

// devRules cobre o lixo deixado por gerenciadores de pacote e toolchains.
//
// Sempre que a ferramenta oferece um comando oficial de limpeza, é ele que a
// regra usa. Um `rm -rf` no store do pnpm ou no modcache do Go remove os
// arquivos mas deixa o índice interno apontando para eles, e a próxima instalação
// falha de um jeito que ninguém liga à limpeza feita três dias antes.
func devRules() []domain.Rule {
	return []domain.Rule{
		{
			ID:       "npm-cache",
			Name:     "npm — cache global",
			Category: domain.CategoryDev,
			Risk:     domain.RiskRegenerable,
			What:     "Tarballs e metadados que o npm guarda de tudo que você já instalou.",
			Lose:     "Nada. O cache é só uma cópia local do que está no registro público.",
			Regen:    "Volta sozinho conforme você instala pacotes, ao custo de download.",
			Detect:   hasTool("npm"),
			Targets:  homePaths(".npm/_cacache"),
			Strategy: domain.RunCommand{Name: "npm", Args: []string{"cache", "clean", "--force"}},
		},
		{
			ID:       "pnpm-store",
			Name:     "pnpm — pacotes órfãos no store",
			Category: domain.CategoryDev,
			Risk:     domain.RiskSafe,
			What:     "O store central do pnpm, de onde os node_modules são ligados por hardlink.",
			Lose:     "Nada. `store prune` remove apenas o que nenhum projeto referencia.",
			Regen:    "Não precisa: o que está em uso permanece intacto.",
			Detect:   hasTool("pnpm"),
			Targets:  homePaths("Library/pnpm/store", ".pnpm-store"),
			Strategy: domain.RunCommand{Name: "pnpm", Args: []string{"store", "prune"}},
		},
		{
			ID:       "yarn-cache",
			Name:     "Yarn — cache global",
			Category: domain.CategoryDev,
			Risk:     domain.RiskRegenerable,
			What:     "Pacotes que o Yarn baixou e guardou para reaproveitar entre projetos.",
			Lose:     "Nada além da velocidade da próxima instalação offline.",
			Regen:    "Volta conforme você instala pacotes.",
			Detect:   hasTool("yarn"),
			Targets:  homePaths("Library/Caches/Yarn", ".yarn/berry/cache"),
			Strategy: domain.RunCommand{Name: "yarn", Args: []string{"cache", "clean"}},
		},
		{
			ID:       "go-modcache",
			Name:     "Go — cache de módulos",
			Category: domain.CategoryDev,
			Risk:     domain.RiskRegenerable,
			What:     "Código-fonte de todas as dependências que qualquer projeto Go seu já usou.",
			Lose:     "A capacidade de compilar sem rede até baixar tudo de novo.",
			Regen:    "`go build` rebaixa o necessário. Em projetos grandes leva minutos.",
			Detect:   hasTool("go"),
			Targets:  goModCacheTargets,
			// O modcache é somente-leitura por design (0444). Removê-lo com rm
			// falha em metade dos arquivos; `go clean` sabe ajustar as permissões.
			Strategy: domain.RunCommand{Name: "go", Args: []string{"clean", "-modcache"}},
		},
		{
			ID:       "go-buildcache",
			Name:     "Go — cache de compilação",
			Category: domain.CategoryDev,
			Risk:     domain.RiskSafe,
			What:     "Resultados intermediários de compilação, reaproveitados entre builds.",
			Lose:     "Nada. É puro resultado derivado do seu código.",
			Regen:    "A próxima compilação de cada projeto fica mais lenta uma vez só.",
			Detect:   hasTool("go"),
			Targets:  homePaths("Library/Caches/go-build"),
			Strategy: domain.RunCommand{Name: "go", Args: []string{"clean", "-cache"}},
		},
		{
			ID:       "gradle-caches",
			Name:     "Gradle — caches de dependências e wrappers",
			Category: domain.CategoryDev,
			Risk:     domain.RiskRegenerable,
			What:     "JARs baixados, resultados de build e as distribuições do próprio Gradle.",
			Lose:     "Nada do seu código. Projetos Android voltam a baixar tudo no próximo build.",
			Regen:    "Automático no próximo build, com download pesado.",
			Detect:   hasTool("gradle", "adb"),
			// caches/ e wrapper/ e não ~/.gradle inteiro: a raiz guarda
			// gradle.properties, que costuma ter credenciais de repositório.
			Targets:  homePaths(".gradle/caches", ".gradle/wrapper", ".gradle/daemon"),
			Strategy: domain.TrashTargets{},
		},
		{
			ID:       "maven-repo",
			Name:     "Maven — repositório local",
			Category: domain.CategoryDev,
			Risk:     domain.RiskRegenerable,
			What:     "Todos os artefatos que o Maven já baixou, em ~/.m2/repository.",
			Lose:     "Artefatos instalados localmente com `mvn install` que não estejam em nenhum repositório remoto.",
			Regen:    "Baixado de novo no próximo build. O que era só local precisa ser reconstruído.",
			Detect:   hasTool("mvn"),
			// settings.xml fica em ~/.m2 e costuma guardar senhas de repositório
			// privado. Só o subdiretório repository é tocado.
			Targets:  homePaths(".m2/repository"),
			Strategy: domain.TrashTargets{},
		},
		{
			ID:       "cocoapods-cache",
			Name:     "CocoaPods — cache e specs",
			Category: domain.CategoryDev,
			Risk:     domain.RiskRegenerable,
			What:     "Pods baixados e o clone do repositório de especificações.",
			Lose:     "Nada. O Podfile.lock continua determinando as versões.",
			Regen:    "`pod install` refaz o download e o clone dos specs.",
			Detect:   hasTool("pod"),
			Targets:  homePaths("Library/Caches/CocoaPods", ".cocoapods/repos"),
			Strategy: domain.TrashTargets{},
		},
		{
			ID:       "pip-cache",
			Name:     "pip — cache de wheels",
			Category: domain.CategoryDev,
			Risk:     domain.RiskRegenerable,
			What:     "Wheels e downloads que o pip guarda entre instalações.",
			Lose:     "Nada. Ambientes virtuais já criados não são afetados.",
			Regen:    "Volta conforme você instala pacotes.",
			Detect:   hasTool("pip3", "pip", "python3"),
			Targets:  homePaths("Library/Caches/pip"),
			Strategy: domain.RunCommand{Name: "pip3", Args: []string{"cache", "purge"}},
		},
		{
			ID:       "homebrew-cache",
			Name:     "Homebrew — downloads e versões antigas",
			Category: domain.CategoryDev,
			Risk:     domain.RiskSafe,
			What:     "Instaladores baixados e versões antigas de fórmulas já atualizadas.",
			Lose:     "Nada. Os programas instalados continuam funcionando.",
			Regen:    "Não precisa. É resíduo de instalações já concluídas.",
			Detect:   hasTool("brew"),
			Targets:  homePaths("Library/Caches/Homebrew"),
			Strategy: domain.RunCommand{Name: "brew", Args: []string{"cleanup", "-s", "--prune=all"}},
		},
		{
			ID:       "nvm-versions",
			Name:     "nvm — versões de Node fora de uso",
			Category: domain.CategoryDev,
			Risk:     domain.RiskReview,
			What:     "Instalações completas de Node que o nvm mantém lado a lado.",
			Lose:     "A versão sai do disco. Projetos com .nvmrc apontando para ela param até reinstalar.",
			Regen:    "`nvm install <versão>` traz de volta em segundos.",
			Detect:   func(env domain.Env) bool { return dirExists(env.HomePath(".nvm/versions/node")) },
			Targets:  nvmUnusedVersions,
			Strategy: domain.TrashTargets{},
		},
		{
			ID:       "swiftpm-cache",
			Name:     "Swift Package Manager — cache",
			Category: domain.CategoryDev,
			Risk:     domain.RiskRegenerable,
			What:     "Clones e artefatos que o SwiftPM guarda entre resoluções de pacote.",
			Lose:     "Nada. O Package.resolved continua fixando as versões.",
			Regen:    "A próxima resolução refaz os clones.",
			Detect:   hasTool("swift", "xcodebuild"),
			Targets:  homePaths("Library/Caches/org.swift.swiftpm", ".swiftpm/cache"),
			Strategy: domain.TrashTargets{},
		},
		{
			ID:       "pub-cache",
			Name:     "Dart e Flutter — pub cache",
			Category: domain.CategoryDev,
			Risk:     domain.RiskRegenerable,
			What:     "Pacotes que o pub baixou para projetos Dart e Flutter.",
			Lose:     "Nada. O pubspec.lock continua fixando as versões.",
			Regen:    "`flutter pub get` refaz o download.",
			Detect:   hasTool("dart", "flutter", "fvm"),
			Targets:  homePaths(".pub-cache/hosted", ".pub-cache/git"),
			Strategy: domain.TrashTargets{},
		},
		{
			ID:       "nuget-packages",
			Name:     ".NET — pacotes NuGet",
			Category: domain.CategoryDev,
			Risk:     domain.RiskRegenerable,
			What:     "Pacotes NuGet e caches HTTP do SDK .NET.",
			Lose:     "Nada. As referências do projeto continuam iguais.",
			Regen:    "`dotnet restore` baixa de novo.",
			Detect:   hasTool("dotnet"),
			Targets:  homePaths(".nuget/packages", ".local/share/NuGet/http-cache"),
			Strategy: domain.RunCommand{Name: "dotnet", Args: []string{"nuget", "locals", "all", "--clear"}},
		},
		{
			ID:       "cargo-registry",
			Name:     "Rust — registro e fontes do Cargo",
			Category: domain.CategoryDev,
			Risk:     domain.RiskRegenerable,
			What:     "Crates baixados e o índice do registro.",
			Lose:     "Nada. O Cargo.lock continua fixando as versões.",
			Regen:    "`cargo build` baixa de novo.",
			Detect:   hasTool("cargo"),
			Targets:  homePaths(".cargo/registry/cache", ".cargo/registry/src", ".cargo/git/checkouts"),
			Strategy: domain.TrashTargets{},
		},
	}
}

// goModCacheTargets respeita GOMODCACHE e GOPATH antes de assumir o padrão.
//
// Perguntar ao `go env` em vez de chutar ~/go/pkg/mod importa para quem move o
// GOPATH — apontar a regra para o diretório errado a faria reportar zero e o
// usuário concluiria que a ferramenta não enxerga o cache dele.
func goModCacheTargets(ctx context.Context, env domain.Env) ([]domain.Target, error) {
	if env.Query != nil {
		if out, err := env.Query(ctx, "go", "env", "GOMODCACHE"); err == nil {
			if path := strings.TrimSpace(string(out)); path != "" {
				return []domain.Target{{Path: path}}, nil
			}
		}
	}
	return []domain.Target{{Path: env.HomePath("go/pkg/mod")}}, nil
}

// nvmUnusedVersions lista as versões instaladas menos a que está ativa.
//
// Excluir a versão em uso não é conveniência, é requisito: removê-la quebra o
// PATH do shell e todo comando `node` da máquina, e o usuário não faria a ligação
// entre isso e a limpeza que acabou de aprovar.
func nvmUnusedVersions(ctx context.Context, env domain.Env) ([]domain.Target, error) {
	root := env.HomePath(".nvm/versions/node")

	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	inUse := currentNodeVersion(ctx, env)

	var targets []domain.Target
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == inUse {
			continue
		}
		targets = append(targets, domain.Target{
			Path:  filepath.Join(root, entry.Name()),
			Label: entry.Name(),
		})
	}
	return targets, nil
}

// currentNodeVersion devolve a versão ativa no formato do nvm ("v22.21.0"), ou
// string vazia se não der para determinar.
//
// Em caso de dúvida, o retorno vazio faz todas as versões aparecerem como
// candidatas — e como a regra é RiskReview, elas chegam ao usuário desmarcadas,
// uma a uma, com o número da versão visível. Errar para o lado de mostrar demais
// é recuperável; remover a versão ativa não.
func currentNodeVersion(ctx context.Context, env domain.Env) string {
	if env.Query == nil {
		return ""
	}
	out, err := env.Query(ctx, "node", "--version")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
