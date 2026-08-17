package domain

import "slices"

// Risk classifica o custo de remover um alvo.
//
// É o campo que substitui o julgamento do LLM no fluxo manual: em vez de o
// modelo decidir caso a caso o que é seguro, a decisão fica gravada no catálogo,
// revisável e testável. Ele governa o que vem pré-marcado na TUI.
type Risk int

const (
	// RiskSafe é cache puro: volta sozinho, sem custo perceptível.
	RiskSafe Risk = iota

	// RiskRegenerable volta, mas cobra tempo e banda de re-download.
	// Ex.: ~/go/pkg/mod, ~/.npm. Nada se perde além de paciência.
	RiskRegenerable

	// RiskReview pode conter algo que o usuário quer manter. Nunca é
	// pré-marcado e exige confirmação extra.
	RiskReview
)

// PreSelected informa se o alvo vem marcado por padrão na TUI.
//
// Apenas RiskSafe. Um default agressivo transforma "confirmar" em reflexo, e o
// dia em que ele apagar algo importante o usuário desinstala a ferramenta.
func (r Risk) PreSelected() bool { return r == RiskSafe }

func (r Risk) String() string {
	switch r {
	case RiskSafe:
		return "seguro"
	case RiskRegenerable:
		return "regenerável"
	case RiskReview:
		return "revisar"
	default:
		return "desconhecido"
	}
}

// Category agrupa regras no relatório e na TUI.
type Category string

// Categorias reconhecidas. Os valores são estáveis: aparecem em `--category` e
// no JSON do relatório.
const (
	CategoryDev      Category = "dev"
	CategorySystem   Category = "system"
	CategoryApps     Category = "apps"
	CategoryBigFiles Category = "bigfiles"
	CategoryProjects Category = "projects"
)

// Categories devolve as categorias na ordem em que aparecem na interface:
// das mais seguras e previsíveis para as que exigem mais atenção.
func Categories() []Category {
	return []Category{
		CategoryDev,
		CategorySystem,
		CategoryApps,
		CategoryProjects,
		CategoryBigFiles,
	}
}

// Title devolve o nome da categoria como exibido na interface.
func (c Category) Title() string {
	switch c {
	case CategoryDev:
		return "Ferramentas de desenvolvimento"
	case CategorySystem:
		return "Sistema"
	case CategoryApps:
		return "Aplicativos"
	case CategoryBigFiles:
		return "Arquivos grandes"
	case CategoryProjects:
		return "Projetos abandonados"
	default:
		return string(c)
	}
}

// Valid informa se a categoria é conhecida — usado para validar `--category`.
func (c Category) Valid() bool {
	return slices.Contains(Categories(), c)
}
