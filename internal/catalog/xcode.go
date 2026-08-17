package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
)

// xcodeRules cobre o Xcode, que costuma ser o maior consumidor isolado de disco
// numa máquina de desenvolvimento — quase tudo dele é derivado e volta sozinho.
func xcodeRules() []domain.Rule {
	return []domain.Rule{
		{
			ID:       "xcode-deriveddata",
			Name:     "Xcode — DerivedData",
			Category: domain.CategoryDev,
			Risk:     domain.RiskSafe,
			What:     "Resultado de compilação, índices e logs de cada projeto que você abriu.",
			Lose:     "Nada do seu código. Apenas o build incremental já feito.",
			Regen:    "A próxima compilação de cada projeto recomeça do zero, uma vez só.",
			Detect:   hasTool("xcodebuild"),
			Targets:  homePaths("Library/Developer/Xcode/DerivedData"),
			Strategy: domain.TrashTargets{},
		},
		{
			ID:       "xcode-devicesupport",
			Name:     "Xcode — símbolos de dispositivos iOS",
			Category: domain.CategoryDev,
			Risk:     domain.RiskRegenerable,
			What: "Cópia dos símbolos de depuração de cada versão de iOS de cada aparelho " +
				"que você já conectou. O Xcode guarda um conjunto por versão, então o mesmo " +
				"iPhone aparece várias vezes.",
			Lose: "A capacidade de depurar em um aparelho com aquela versão de iOS sem reprocessar.",
			Regen: "O Xcode recopia do aparelho na próxima vez que você conectá-lo. " +
				"Leva alguns minutos e acontece sozinho.",
			Detect: hasTool("xcodebuild"),
			// Um alvo por versão, para poder manter a do aparelho em uso e
			// descartar as antigas. Em bloco, a decisão vira tudo-ou-nada.
			Targets:  homeGlob("Library/Developer/Xcode/iOS DeviceSupport/*", "Library/Developer/Xcode/watchOS DeviceSupport/*"),
			Strategy: domain.TrashTargets{},
		},
		{
			ID:       "xcode-previews",
			Name:     "Xcode — simuladores de Preview do SwiftUI",
			Category: domain.CategoryDev,
			Risk:     domain.RiskSafe,
			What:     "Simuladores descartáveis que o Xcode cria para renderizar os Previews do SwiftUI.",
			Lose:     "Nada. São instâncias temporárias, não seus projetos.",
			Regen:    "O Xcode recria na primeira vez que você abrir um Preview.",
			Detect:   hasTool("xcodebuild"),
			Targets:  homePaths("Library/Developer/Xcode/UserData/Previews"),
			Strategy: domain.TrashTargets{},
		},
		{
			ID:       "xcode-archives",
			Name:     "Xcode — Archives de builds publicados",
			Category: domain.CategoryDev,
			Risk:     domain.RiskReview,
			What:     "Os pacotes .xcarchive gerados a cada build enviado para distribuição.",
			Lose: "Os dSYMs daquela versão. Sem eles, crashes reportados por usuários " +
				"da build correspondente ficam ilegíveis para sempre.",
			Regen:    "Não volta. Só recompilando exatamente o mesmo commit com o mesmo Xcode.",
			Detect:   hasTool("xcodebuild"),
			Targets:  homeGlob("Library/Developer/Xcode/Archives/*"),
			Strategy: domain.TrashTargets{},
		},
		{
			ID:       "coresimulator-caches",
			Name:     "Simuladores iOS — caches",
			Category: domain.CategoryDev,
			Risk:     domain.RiskSafe,
			What:     "Caches do CoreSimulator: imagens de runtime já extraídas e dados temporários.",
			Lose:     "Nada. Os simuladores e os apps instalados neles continuam.",
			Regen:    "Recriado no próximo uso.",
			Detect:   hasTool("xcodebuild"),
			// Library/Logs/CoreSimulator não entra aqui: pertence à regra
			// user-logs, e dois alvos sobrepostos contariam o mesmo espaço duas
			// vezes no total prometido.
			Targets:  homePaths("Library/Developer/CoreSimulator/Caches"),
			Strategy: domain.TrashTargets{},
		},
		{
			ID:       "coresimulator-unavailable",
			Name:     "Simuladores iOS — instâncias órfãs",
			Category: domain.CategoryDev,
			Risk:     domain.RiskSafe,
			What: "Simuladores cujo runtime foi removido junto com uma versão antiga do Xcode. " +
				"Eles não aparecem mais na lista de dispositivos, mas continuam ocupando disco.",
			Lose:    "Nada. São inutilizáveis: o runtime que os executava não existe mais.",
			Regen:   "Não precisa. Simuladores novos são criados a partir dos runtimes atuais.",
			Detect:  hasTool("xcrun", "xcodebuild"),
			Targets: unavailableSimulators,
			// simctl atualiza o índice interno do CoreSimulator ao deletar.
			// Apagar os diretórios na mão deixa o índice apontando para o vazio.
			Strategy: domain.RunCommand{Name: "xcrun", Args: []string{"simctl", "delete", "unavailable"}},
		},
	}
}

// simctlDevice é o recorte que nos interessa da saída de `simctl list devices`.
type simctlDevice struct {
	UDID        string `json:"udid"`
	Name        string `json:"name"`
	DataPath    string `json:"dataPath"`
	IsAvailable bool   `json:"isAvailable"`
}

type simctlList struct {
	Devices map[string][]simctlDevice `json:"devices"`
}

// unavailableSimulators pergunta ao simctl quais dispositivos estão órfãos.
//
// Perguntar, em vez de medir o diretório Devices inteiro, é o que mantém o número
// honesto: `simctl delete unavailable` remove só um subconjunto, e reportar o
// total prometeria um espaço que a limpeza não entrega.
func unavailableSimulators(ctx context.Context, env domain.Env) ([]domain.Target, error) {
	if env.Query == nil {
		return nil, nil
	}

	out, err := env.Query(ctx, "xcrun", "simctl", "list", "devices", "--json")
	if err != nil {
		// Sem Xcode instalado ou sem as ferramentas de linha de comando. A regra
		// simplesmente não se aplica a esta máquina.
		//nolint:nilerr // ausência do simctl não é falha da varredura
		return nil, nil
	}

	var list simctlList
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, fmt.Errorf("interpretando a saída do simctl: %w", err)
	}

	var targets []domain.Target
	for runtime, devices := range list.Devices {
		for _, device := range devices {
			if device.IsAvailable || device.DataPath == "" {
				continue
			}
			// dataPath aponta para <UDID>/data; o diretório do dispositivo é o pai.
			targets = append(targets, domain.Target{
				Path:  filepath.Dir(device.DataPath),
				Label: fmt.Sprintf("%s (%s)", device.Name, shortRuntime(runtime)),
			})
		}
	}
	return targets, nil
}

// shortRuntime encurta "com.apple.CoreSimulator.SimRuntime.iOS-16-4" para "iOS-16-4".
func shortRuntime(identifier string) string {
	const prefix = "com.apple.CoreSimulator.SimRuntime."
	if len(identifier) > len(prefix) && identifier[:len(prefix)] == prefix {
		return identifier[len(prefix):]
	}
	return identifier
}
