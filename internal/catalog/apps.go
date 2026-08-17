package catalog

import "github.com/danfigueroa/mac-cleaner/internal/domain"

// electronCacheDirs são os subdiretórios de cache que todo app feito em Electron
// cria dentro da própria pasta em ~/Library/Application Support.
//
// É a mesma estrutura no Slack, Discord, Notion, VS Code, Cursor e Claude,
// porque todos herdam do Chromium. Uma regra genérica cobre os apps que o
// usuário instalar amanhã sem precisar de manutenção no catálogo.
var electronCacheDirs = []string{
	"Cache",
	"Code Cache",
	"GPUCache",
	"CachedData",
	"DawnCache",
	"DawnGraphiteCache",
	"DawnWebGPUCache",
	"ShaderCache",
	"logs",
}

// appsRules cobre aplicativos de desktop.
func appsRules() []domain.Rule {
	return []domain.Rule{
		{
			ID:       "electron-caches",
			Name:     "Apps Electron — caches internos",
			Category: domain.CategoryApps,
			Risk:     domain.RiskSafe,
			What: "Cache de rede, de código compilado e de shaders que apps baseados em Chromium " +
				"guardam dentro da própria pasta. Vale para VS Code, Cursor, Slack, Discord, " +
				"Notion, Claude e qualquer outro app Electron instalado.",
			Lose:     "Nada. Sessão, login e configurações ficam em outros diretórios e não são tocados.",
			Regen:    "Cada app recria o que precisa na próxima abertura.",
			Targets:  appSubdirs("Library/Application Support", electronCacheDirs...),
			Strategy: domain.TrashTargets{},
		},
		{
			ID:       "claude-vm-bundles",
			Name:     "Claude Desktop — máquinas virtuais de sessões locais",
			Category: domain.CategoryApps,
			Risk:     domain.RiskReview,
			What: "Imagens de disco das máquinas virtuais que o Claude Desktop cria para " +
				"executar código localmente. Cada sessão com ambiente próprio deixa um bundle.",
			Lose: "O estado de sessões locais antigas: arquivos criados dentro daquele ambiente " +
				"que você não tenha copiado para fora.",
			Regen:    "Uma VM nova é criada na próxima sessão local, vazia.",
			Targets:  homePaths("Library/Application Support/Claude/vm_bundles"),
			Strategy: domain.TrashTargets{},
		},
		{
			ID:       "jetbrains-caches",
			Name:     "JetBrains — caches e índices",
			Category: domain.CategoryApps,
			Risk:     domain.RiskRegenerable,
			What: "Índices, caches locais e logs de cada IDE da JetBrains, incluindo os de " +
				"versões que você já atualizou e não usa mais.",
			Lose:     "Nada de configuração. Só o índice, que fica em ~/Library/Caches.",
			Regen:    "A IDE reindexa os projetos ao abri-los. Em projeto grande leva minutos.",
			Targets:  homeGlob("Library/Caches/JetBrains/*", "Library/Logs/JetBrains/*"),
			Strategy: domain.TrashTargets{},
		},
		{
			ID:       "browser-caches",
			Name:     "Navegadores — cache de páginas",
			Category: domain.CategoryApps,
			Risk:     domain.RiskSafe,
			What:     "Cache de conteúdo web do Chrome, Brave, Firefox, Edge e Safari.",
			Lose:     "Nada. Histórico, senhas, abas e extensões ficam em outro lugar e permanecem.",
			Regen:    "Preenchido de novo conforme você navega. Os primeiros carregamentos ficam mais lentos.",
			Targets: homePaths(
				"Library/Caches/Google/Chrome",
				"Library/Caches/BraveSoftware",
				"Library/Caches/Firefox",
				"Library/Caches/Microsoft Edge",
				"Library/Caches/com.apple.Safari",
				"Library/Caches/company.thebrowser.Browser",
			),
			Strategy: domain.TrashTargets{},
		},
	}
}
