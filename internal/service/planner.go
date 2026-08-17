package service

import "github.com/danfigueroa/mac-cleaner/internal/domain"

// BuildPlan converte os resultados selecionados pelo usuário num plano executável.
//
// Alvos que exigem root ficam de fora: a CLI nunca escala privilégio, e deixá-los
// no plano só produziria um erro no meio da execução. Eles voltam por
// ManualItems, para serem exibidos com o comando pronto.
func BuildPlan(results []domain.Result) domain.Plan {
	items := make([]domain.PlanItem, 0, len(results))

	for _, result := range results {
		// Removable cobre tanto os alvos que exigem root quanto os que são
		// apenas informativos, como a lista de arquivos grandes.
		if !result.Rule.Removable() {
			continue
		}
		if result.Finding.Empty() {
			continue
		}
		items = append(items, domain.PlanItem(result))
	}

	return domain.Plan{Items: items}
}

// ManualItems devolve os alvos que exigem root, para exibição.
func ManualItems(results []domain.Result) []domain.Result {
	var manual []domain.Result
	for _, result := range results {
		if _, needsRoot := result.Rule.Strategy.(domain.ManualOnly); needsRoot {
			manual = append(manual, result)
		}
	}
	return manual
}
