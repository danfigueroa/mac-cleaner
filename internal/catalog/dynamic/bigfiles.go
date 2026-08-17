package dynamic

import "github.com/danfigueroa/mac-cleaner/internal/domain"

// maxBigFiles limita quantos arquivos entram na lista.
//
// Vinte linhas cabem numa tela e cobrem, na prática, quase todo o espaço que
// uma lista dessas revelaria. Uma lista de duzentos itens não seria lida.
const maxBigFiles = 20

// BigFiles monta a regra de arquivos grandes soltos.
//
// A estratégia é ReportOnly, e isso é o ponto central da regra. Arquivos grandes
// soltos vivem em Downloads, Desktop e Documents — território do usuário, onde o
// guard barra qualquer remoção, e com razão: um .zip de 4 GB pode ser lixo
// esquecido ou o único backup de um projeto, e nada no arquivo permite à
// ferramenta distinguir os dois casos.
//
// Encontrá-los tem valor; decidir por eles, não. Então a CLI lista, ordena por
// tamanho e sai da frente.
func bigFilesRule(shared *survey, minimum domain.Bytes) domain.Rule {
	return domain.Rule{
		ID:       "arquivos-grandes",
		Name:     "Arquivos grandes soltos",
		Category: domain.CategoryBigFiles,
		Risk:     domain.RiskReview,
		What: "Arquivos individuais acima de " + minimum.String() + " espalhados pelo seu " +
			"home: instaladores, imagens de disco, vídeos e exportações esquecidas.",
		Lose:    "Depende inteiramente do arquivo — só você sabe.",
		Regen:   "Nada aqui é regenerável. Por isso a CLI apenas lista.",
		Targets: shared.bigTargets,
		Strategy: domain.ReportOnly{
			Hint: "revise a lista e remova o que não precisar mais",
		},
	}
}
