package dynamic

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
)

// buildDirNames são os diretórios de build que esta regra procura.
//
// A lista espelha a exceção do guard, e pelo mesmo motivo: "dist", "build" e
// "target" ficaram de fora porque, em algum projeto real, cada um deles é uma
// pasta com conteúdo escrito à mão. Os quatro abaixo são sempre reconstruíveis
// por um comando.
var buildDirNames = map[string]struct{}{
	"node_modules": {},
	".next":        {},
	".nuxt":        {},
	".turbo":       {},
}

// projectMarkers indicam a raiz de um projeto, e servem para estimar a última
// vez que alguém mexeu nele.
//
// A data de modificação do diretório só muda quando arquivos são criados ou
// removidos, não quando são editados — o que faria um projeto ativo parecer
// parado. O arquivo de manifesto é um sinal melhor, e o .git melhor ainda.
var projectMarkers = []string{".git", "package.json", "go.mod", "Cargo.toml", "pubspec.yaml"}

// abandonedProjectsRule monta a regra de diretórios de build em projetos parados.
func abandonedProjectsRule(shared *survey, staleAfter time.Duration) domain.Rule {
	return domain.Rule{
		ID:       "projetos-abandonados",
		Name:     "Projetos parados — diretórios de dependências",
		Category: domain.CategoryProjects,
		Risk:     domain.RiskReview,
		What: "node_modules, .next, .nuxt e .turbo de projetos em que ninguém mexe há " +
			formatarPrazo(staleAfter) + ". Nenhum deles contém código seu.",
		Lose: "Nada do seu trabalho. O projeto para de compilar até você reinstalar " +
			"as dependências.",
		Regen: "`npm install` (ou o gerenciador do projeto) refaz tudo, ao custo de " +
			"download e alguns minutos.",
		Targets:  shared.projectTargets,
		Strategy: domain.TrashTargets{},
	}
}

// ultimoToque estima quando o projeto foi mexido pela última vez.
//
// Usa o mais recente entre a data do diretório e a dos marcadores de projeto.
// Errar para o lado mais recente é deliberado: o custo de não listar um projeto
// abandonado é alguns gigabytes a menos no relatório, enquanto o custo de listar
// um projeto ativo é o usuário apagar as dependências do que está usando agora.
func ultimoToque(projeto string) time.Time {
	maisRecente := time.Time{}

	if info, err := os.Stat(projeto); err == nil {
		maisRecente = info.ModTime()
	}

	for _, marcador := range projectMarkers {
		info, err := os.Stat(filepath.Join(projeto, marcador))
		if err != nil {
			continue
		}
		if info.ModTime().After(maisRecente) {
			maisRecente = info.ModTime()
		}
	}
	return maisRecente
}

// formatarPrazo escreve a duração em dias ou meses, como uma pessoa diria.
func formatarPrazo(d time.Duration) string {
	dias := int(d.Hours() / 24)
	switch {
	case dias >= 60:
		return strconv.Itoa(dias/30) + " meses"
	case dias >= 30:
		return "um mês"
	case dias == 1:
		return "um dia"
	default:
		return strconv.Itoa(dias) + " dias"
	}
}
