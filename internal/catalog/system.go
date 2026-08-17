package catalog

import "github.com/danfigueroa/mac-cleaner/internal/domain"

// cachesOwnedByOtherRules lista os subdiretórios de ~/Library/Caches que já têm
// regra própria.
//
// A varredura genérica de caches precisa cedê-los, senão o mesmo espaço entra
// duas vezes no total — e a regra específica é sempre a melhor das duas, porque
// sabe usar o comando oficial da ferramenta em vez de apagar arquivos.
var cachesOwnedByOtherRules = []string{
	"go-build",
	"Homebrew",
	"Yarn",
	"pip",
	"CocoaPods",
	"org.swift.swiftpm",
	"JetBrains",
	"Google",
	"BraveSoftware",
	"Firefox",
	"Microsoft Edge",
	"com.apple.Safari",
	"company.thebrowser.Browser",
}

// systemRules cobre resíduos do macOS e das pastas de biblioteca do usuário.
func systemRules() []domain.Rule {
	return []domain.Rule{
		{
			ID:       "user-caches",
			Name:     "macOS — caches de aplicativos",
			Category: domain.CategorySystem,
			Risk:     domain.RiskRegenerable,
			What: "O conteúdo de ~/Library/Caches, onde cada app guarda o que baixou ou " +
				"calculou para não repetir o trabalho.",
			Lose: "Nada permanente. Alguns apps ficam mais lentos na primeira abertura, " +
				"e uns poucos pedem login de novo.",
			Regen:    "Cada app recria o próprio cache conforme é usado.",
			Targets:  homeGlobExcluding("Library/Caches/*", cachesOwnedByOtherRules...),
			Strategy: domain.TrashTargets{},
		},
		{
			ID:       "user-logs",
			Name:     "macOS — logs de aplicativos",
			Category: domain.CategorySystem,
			Risk:     domain.RiskSafe,
			What:     "Logs de diagnóstico acumulados por apps em ~/Library/Logs.",
			Lose:     "Histórico de diagnóstico. Só importa se você estiver investigando um bug agora.",
			Regen:    "Novos logs passam a ser escritos imediatamente.",
			Targets:  homeGlobExcluding("Library/Logs/*", "JetBrains"),
			Strategy: domain.TrashTargets{},
		},
		{
			ID:       "installer-leftovers",
			Name:     "macOS — instaladores de atualização",
			Category: domain.CategorySystem,
			Risk:     domain.RiskSafe,
			What:     "Pacotes de atualização já aplicados que o sistema não removeu.",
			Lose:     "Nada. As atualizações já foram instaladas.",
			Regen:    "Não precisa. São resíduo de um processo concluído.",
			Targets:  homePaths("Library/Updates"),
			Strategy: domain.TrashTargets{},
		},
		{
			ID:       "ios-backups",
			Name:     "macOS — backups de iPhone e iPad",
			Category: domain.CategorySystem,
			Risk:     domain.RiskReview,
			What:     "Backups locais completos de aparelhos iOS feitos pelo Finder.",
			Lose: "O backup inteiro daquele aparelho. Se ele for a única cópia de fotos ou " +
				"mensagens, não há como recuperar depois.",
			Regen:    "Só refazendo o backup com o aparelho conectado.",
			Targets:  homeGlob("Library/Application Support/MobileSync/Backup/*"),
			Strategy: domain.TrashTargets{},
		},
		{
			ID:       "system-wallpapers",
			Name:     "macOS — vídeos 4K de papel de parede",
			Category: domain.CategorySystem,
			Risk:     domain.RiskRegenerable,
			What: "Os vídeos em 4K que o macOS baixa sozinho para os papéis de parede " +
				"aéreos e o protetor de tela. São dezenas de gigabytes em muitas máquinas.",
			Lose:    "Os papéis de parede animados voltam a exibir só a imagem estática.",
			Regen:   "O sistema rebaixa sob demanda quando você seleciona um deles.",
			Targets: systemPaths("Library/Application Support/com.apple.idleassetsd/Customer"),
			// Fora do home e pertencente ao root. A CLI mede, mostra e para aí.
			Strategy: domain.ManualOnly{
				Command: "sudo rm -rf '/Library/Application Support/com.apple.idleassetsd/Customer/4KSDR240FPS'",
			},
		},
		{
			ID:       "system-caches",
			Name:     "macOS — caches do sistema",
			Category: domain.CategorySystem,
			Risk:     domain.RiskRegenerable,
			What:     "Caches em /Library/Caches, compartilhados por todos os usuários da máquina.",
			Lose:     "Nada permanente, mas alguns serviços do sistema ficam lentos por um tempo.",
			Regen:    "Recriado pelo sistema conforme necessário.",
			Targets:  systemPaths("Library/Caches"),
			Strategy: domain.ManualOnly{Command: "sudo rm -rf /Library/Caches/*"},
		},
	}
}
