package catalog

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
)

// targetsFunc é a assinatura de domain.Rule.Targets, abreviada porque aparece em
// toda regra do catálogo.
type targetsFunc = func(context.Context, domain.Env) ([]domain.Target, error)

// homePaths monta alvos a partir de caminhos relativos ao home.
//
// Caminhos que não existem não são um problema: o Scanner os mede como zero e
// eles somem do relatório. Isso permite listar as duas localizações possíveis de
// um cache sem precisar descobrir antes qual delas a máquina usa.
func homePaths(relative ...string) targetsFunc {
	return func(_ context.Context, env domain.Env) ([]domain.Target, error) {
		targets := make([]domain.Target, 0, len(relative))
		for _, rel := range relative {
			targets = append(targets, domain.Target{Path: env.HomePath(rel)})
		}
		return targets, nil
	}
}

// systemPaths monta alvos a partir da raiz do sistema. Usado apenas por regras
// ManualOnly — nada fora do home é removido pela CLI.
func systemPaths(absolute ...string) targetsFunc {
	return func(_ context.Context, env domain.Env) ([]domain.Target, error) {
		targets := make([]domain.Target, 0, len(absolute))
		for _, abs := range absolute {
			targets = append(targets, domain.Target{Path: env.SystemPath(abs)})
		}
		return targets, nil
	}
}

// homeGlob expande padrões relativos ao home, um alvo por correspondência.
//
// O padrão não pode conter "**". A restrição é deliberada: "*" não atravessa
// separador, então a profundidade do que casa fica fixa no próprio padrão e dá
// para saber, lendo a regra, exatamente que nível da árvore ela alcança. Um
// glob recursivo transformaria cada regra numa aposta.
func homeGlob(patterns ...string) targetsFunc {
	for _, pattern := range patterns {
		if strings.Contains(pattern, "**") {
			panic("catalog: glob recursivo não é permitido em regra: " + pattern)
		}
	}

	return func(_ context.Context, env domain.Env) ([]domain.Target, error) {
		var targets []domain.Target
		for _, pattern := range patterns {
			matches, err := filepath.Glob(env.HomePath(pattern))
			if err != nil {
				// Só acontece com padrão malformado, que o panic acima e os
				// testes do catálogo já pegariam antes de chegar aqui.
				return nil, err
			}
			for _, match := range matches {
				targets = append(targets, domain.Target{
					Path:  match,
					Label: filepath.Base(match),
				})
			}
		}
		return targets, nil
	}
}

// homeGlobExcluding é como homeGlob, mas descarta as correspondências cujo nome
// final esteja na lista de exclusão.
//
// Serve às regras de varredura ampla, que precisam ceder o que já pertence a uma
// regra específica. Sem isso, ~/Library/Caches/go-build seria contado tanto pela
// regra do Go quanto pela varredura genérica, e o total prometido ao usuário
// passaria a ser maior que o espaço que existe para liberar.
func homeGlobExcluding(pattern string, exclude ...string) targetsFunc {
	excluded := make(map[string]struct{}, len(exclude))
	for _, name := range exclude {
		excluded[name] = struct{}{}
	}

	expand := homeGlob(pattern)
	return func(ctx context.Context, env domain.Env) ([]domain.Target, error) {
		all, err := expand(ctx, env)
		if err != nil {
			return nil, err
		}

		kept := all[:0]
		for _, target := range all {
			if _, skip := excluded[filepath.Base(target.Path)]; skip {
				continue
			}
			kept = append(kept, target)
		}
		return kept, nil
	}
}

// appSubdirs expande, para cada aplicativo encontrado, os subdiretórios
// indicados. Serve às regras que limpam o mesmo tipo de cache em muitos apps.
func appSubdirs(parent string, subdirs ...string) targetsFunc {
	return func(_ context.Context, env domain.Env) ([]domain.Target, error) {
		apps, err := filepath.Glob(env.HomePath(parent, "*"))
		if err != nil {
			return nil, err
		}

		var targets []domain.Target
		for _, app := range apps {
			// O glob casa com arquivos também — .DS_Store é o caso garantido em
			// qualquer Mac. Sem este filtro, a regra produz caminhos como
			// ".DS_Store/Cache", que não são "inexistentes" e sim "não é um
			// diretório", um erro de outra natureza.
			if !dirExists(app) {
				continue
			}

			appName := filepath.Base(app)
			for _, sub := range subdirs {
				targets = append(targets, domain.Target{
					Path:  filepath.Join(app, sub),
					Label: appName + " › " + sub,
				})
			}
		}
		return targets, nil
	}
}

// hasTool monta um Detect que aceita a regra se qualquer um dos executáveis
// estiver no PATH.
func hasTool(names ...string) func(domain.Env) bool {
	return func(env domain.Env) bool { return env.HasAnyTool(names...) }
}

// dirExists informa se o caminho existe e é um diretório.
//
// Mora aqui, junto dos demais auxiliares, porque é usado pelos construtores de
// alvo e não por nenhuma regra em particular.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
