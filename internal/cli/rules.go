package cli

import (
	"fmt"

	"github.com/danfigueroa/mac-cleaner/internal/catalog"
	"github.com/danfigueroa/mac-cleaner/internal/catalog/dynamic"
	"github.com/danfigueroa/mac-cleaner/internal/domain"
)

// selectedRules monta o conjunto de regras de uma execução.
//
// As regras dinâmicas são construídas aqui, e não no catálogo, porque dependem
// de valores que o usuário controla por flag: o tamanho a partir do qual um
// arquivo conta como grande e a idade a partir da qual um projeto conta como
// parado. Declará-las como variáveis de pacote as congelaria nos padrões.
func selectedRules(opts *options) ([]domain.Rule, error) {
	categories, err := catalog.ParseCategories(opts.categories)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errUsage, err)
	}

	rules := catalog.All()
	rules = append(rules, dynamic.Rules(opts.bigFiles, opts.stale)...)

	return catalog.FilterByCategories(rules, categories), nil
}
