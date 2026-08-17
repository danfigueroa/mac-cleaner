// Package catalog é o conhecimento da ferramenta: quais diretórios são lixo
// regenerável, quanto custa removê-los e qual o jeito certo de fazê-lo.
//
// É o que o fluxo manual com um LLM produzia caso a caso, convertido em dados
// versionados. Aqui isso pode ser revisado num diff, testado, e corrigido uma
// vez para todo mundo — em vez de ser redecidido, com resultado diferente, a
// cada execução.
//
// O pacote não executa nada. Ele descreve alvos; quem remove é o service, atrás
// do guard.
package catalog

import (
	"fmt"
	"slices"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
)

// All devolve o catálogo completo, na ordem canônica.
func All() []domain.Rule {
	var rules []domain.Rule
	rules = append(rules, devRules()...)
	rules = append(rules, xcodeRules()...)
	rules = append(rules, dockerRules()...)
	rules = append(rules, appsRules()...)
	rules = append(rules, systemRules()...)
	return rules
}

// ByIDs seleciona regras pelos identificadores informados.
//
// Um ID desconhecido é erro, e não um aviso: `mac-cleaner clean npm-cache
// go-modcache` com um typo silencioso limparia menos do que o usuário pediu e
// reportaria sucesso.
func ByIDs(ids []string) ([]domain.Rule, error) {
	index := make(map[string]domain.Rule, len(All()))
	for _, rule := range All() {
		index[rule.ID] = rule
	}

	selected := make([]domain.Rule, 0, len(ids))
	for _, id := range ids {
		rule, ok := index[id]
		if !ok {
			return nil, fmt.Errorf("%w: %q", domain.ErrRuleNotFound, id)
		}
		selected = append(selected, rule)
	}
	return selected, nil
}

// FilterByCategories devolve apenas as regras das categorias informadas. Uma
// lista vazia não filtra nada.
func FilterByCategories(rules []domain.Rule, categories []domain.Category) []domain.Rule {
	if len(categories) == 0 {
		return rules
	}

	filtered := make([]domain.Rule, 0, len(rules))
	for _, rule := range rules {
		if slices.Contains(categories, rule.Category) {
			filtered = append(filtered, rule)
		}
	}
	return filtered
}

// ParseCategories converte os valores de `--category`, rejeitando os desconhecidos.
func ParseCategories(values []string) ([]domain.Category, error) {
	categories := make([]domain.Category, 0, len(values))
	for _, value := range values {
		category := domain.Category(value)
		if !category.Valid() {
			return nil, fmt.Errorf("%w: %q (use dev, system, apps, projects ou bigfiles)",
				domain.ErrInvalidCategory, value)
		}
		categories = append(categories, category)
	}
	return categories, nil
}
