//go:build darwin && integration

// Teste de integração entre o catálogo e o guard, contra a máquina real.
//
//	go test -tags integration ./internal/service/ -v -run Guard
package service_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/danfigueroa/mac-cleaner/internal/adapter/cmdrunner"
	"github.com/danfigueroa/mac-cleaner/internal/adapter/osfs"
	"github.com/danfigueroa/mac-cleaner/internal/catalog"
	"github.com/danfigueroa/mac-cleaner/internal/domain"
	"github.com/danfigueroa/mac-cleaner/internal/guard"
	"github.com/danfigueroa/mac-cleaner/internal/service"
)

// TestGuardAceitaTudoQueOCatalogoProduz é o teste que amarra as duas metades do
// projeto.
//
// Os testes unitários do guard provam que ele rejeita o que deve rejeitar, e os
// do catálogo provam que as regras são bem formadas. Nenhum dos dois responde à
// pergunta que decide se a ferramenta funciona: os alvos que o catálogo produz
// *nesta máquina*, com os diretórios que ela realmente tem, passam pelo guard?
//
// Um "não" aqui significa que a limpeza abortaria com ErrGuardViolation na cara
// do usuário depois de ele já ter aprovado o plano.
func TestGuardAceitaTudoQueOCatalogoProduz(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("descobrindo o home: %v", err)
	}

	runner := cmdrunner.New()
	env := domain.Env{
		Home:     home,
		Root:     "/",
		LookPath: exec.LookPath,
		Query:    runner.Query,
	}

	filesystem := osfs.New()
	scanner := service.NewScanner(filesystem, filesystem)
	guardian := guard.New(home)

	report, err := scanner.Scan(t.Context(), env, catalog.All())
	if err != nil {
		t.Fatalf("varredura: %v", err)
	}

	plan := service.BuildPlan(report.Results)
	if plan.Empty() {
		t.Skip("nada a limpar nesta máquina — o teste não tem o que verificar")
	}

	verificados := 0
	for _, item := range plan.Items {
		if _, moveParaLixeira := item.Rule.Strategy.(domain.TrashTargets); !moveParaLixeira {
			continue
		}
		for _, target := range item.Finding.Targets {
			if err := guardian.Check(target.Path); err != nil {
				t.Errorf("a regra %q produziu um alvo que o guard rejeita:\n  %s\n  %v",
					item.Rule.ID, target.Path, err)
			}
			verificados++
		}
	}

	t.Logf("%d caminhos de %d regras passaram pelo guard", verificados, len(plan.Items))

	if verificados == 0 {
		t.Skip("nenhuma regra desta máquina usa a Lixeira")
	}
}
