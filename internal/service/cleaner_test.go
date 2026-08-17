package service_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
	"github.com/danfigueroa/mac-cleaner/internal/service"
)

// Os fakes abaixo são escritos à mão em vez de gerados. As três interfaces têm
// um método cada, e uma dependência de framework de mock custaria mais para ler
// do que estas trinta linhas.

type guardFake struct {
	rejeitar map[string]error
	checados []string
	mu       sync.Mutex
}

func (g *guardFake) Check(path string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.checados = append(g.checados, path)
	return g.rejeitar[path]
}

type trasherFake struct {
	movidos []string
	falhar  map[string]error
	mu      sync.Mutex
}

func (t *trasherFake) Trash(path string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err, falha := t.falhar[path]; falha {
		return err
	}
	t.movidos = append(t.movidos, path)
	return nil
}

type runnerFake struct {
	executados []string
	err        error
	mu         sync.Mutex
}

func (r *runnerFake) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executados = append(r.executados, strings.Join(append([]string{name}, args...), " "))
	return nil, r.err
}

func regraLixeira(id string, paths ...string) domain.Result {
	targets := make([]domain.Target, 0, len(paths))
	for _, path := range paths {
		targets = append(targets, domain.Target{Path: path, Size: 1_000, Measured: true})
	}
	return domain.Result{
		Rule: domain.Rule{
			ID:       id,
			Name:     id,
			Category: domain.CategoryDev,
			Strategy: domain.TrashTargets{},
		},
		Finding: domain.Finding{RuleID: id, Size: domain.Bytes(1_000 * len(paths)), Targets: targets},
	}
}

func regraComando(id, nome string, args ...string) domain.Result {
	return domain.Result{
		Rule: domain.Rule{
			ID:       id,
			Name:     id,
			Category: domain.CategoryDev,
			Strategy: domain.RunCommand{Name: nome, Args: args},
		},
		Finding: domain.Finding{RuleID: id, Size: 5_000},
	}
}

func TestCleanerMoveParaALixeira(t *testing.T) {
	t.Parallel()

	guardian, trasher, runner := &guardFake{}, &trasherFake{}, &runnerFake{}
	cleaner := service.NewCleaner(guardian, trasher, runner)

	plan := service.BuildPlan([]domain.Result{
		regraLixeira("caches", "/home/ana/Library/Caches/a", "/home/ana/Library/Caches/b"),
	})

	summary, err := cleaner.Execute(t.Context(), plan)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(summary.Failures()) != 0 {
		t.Errorf("falhas inesperadas: %v", summary.Failures())
	}

	quer := []string{"/home/ana/Library/Caches/a", "/home/ana/Library/Caches/b"}
	if !iguais(trasher.movidos, quer) {
		t.Errorf("movidos = %v, quer %v", trasher.movidos, quer)
	}

	// O espaço foi para a Lixeira, não liberado. Confundir os dois é o erro que
	// faz o usuário rodar `df`, não ver diferença e achar que nada aconteceu.
	if summary.Reclaimed() != 0 {
		t.Errorf("Reclaimed = %s, quer 0: o conteúdo foi para a Lixeira", summary.Reclaimed())
	}
	if summary.Trashed() != 2_000 {
		t.Errorf("Trashed = %s, quer 2.0 KB", summary.Trashed())
	}
}

// TestCleanerAbortaAntesDeRemoverQuandoGuardRejeita é a garantia central da
// execução: o usuário aprovou um conjunto, não uma sequência. Se qualquer item
// do plano viola o guard, nada pode ter sido removido — nem o que vinha antes.
func TestCleanerAbortaAntesDeRemoverQuandoGuardRejeita(t *testing.T) {
	t.Parallel()

	proibido := "/home/ana/Documents"
	guardian := &guardFake{rejeitar: map[string]error{
		proibido: domain.ErrGuardViolation,
	}}
	trasher, runner := &trasherFake{}, &runnerFake{}

	plan := service.BuildPlan([]domain.Result{
		regraLixeira("inocente", "/home/ana/Library/Caches/a"),
		regraLixeira("maliciosa", proibido),
	})

	_, err := service.NewCleaner(guardian, trasher, runner).Execute(t.Context(), plan)
	if !errors.Is(err, domain.ErrGuardViolation) {
		t.Fatalf("erro = %v, quer ErrGuardViolation", err)
	}
	if len(trasher.movidos) != 0 {
		t.Errorf("removeu %v apesar da violação — a validação tem que preceder toda remoção",
			trasher.movidos)
	}
}

func TestCleanerExecutaComandoNativo(t *testing.T) {
	t.Parallel()

	guardian, trasher, runner := &guardFake{}, &trasherFake{}, &runnerFake{}

	plan := service.BuildPlan([]domain.Result{
		regraComando("docker-images", "docker", "image", "prune", "-a", "-f"),
	})

	summary, err := service.NewCleaner(guardian, trasher, runner).Execute(t.Context(), plan)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	quer := []string{"docker image prune -a -f"}
	if !iguais(runner.executados, quer) {
		t.Errorf("executados = %v, quer %v", runner.executados, quer)
	}
	if len(trasher.movidos) != 0 {
		t.Errorf("comando nativo não deveria mover nada para a Lixeira: %v", trasher.movidos)
	}
	// Comando nativo libera o espaço de fato, ao contrário da Lixeira.
	if summary.Reclaimed() != 5_000 {
		t.Errorf("Reclaimed = %s, quer 5.0 KB", summary.Reclaimed())
	}
}

func TestCleanerRecusaAlvoQueExigeRoot(t *testing.T) {
	t.Parallel()

	// BuildPlan já descarta estes alvos, então montamos o plano na mão para
	// verificar que o Cleaner também se recusa — a segunda barreira existe
	// justamente para o dia em que alguém montar um plano por outro caminho.
	plan := domain.Plan{Items: []domain.PlanItem{{
		Rule: domain.Rule{
			ID:       "system-caches",
			Strategy: domain.ManualOnly{Command: "sudo rm -rf /Library/Caches/*"},
		},
		Finding: domain.Finding{RuleID: "system-caches", Size: 1_000},
	}}}

	_, err := service.NewCleaner(&guardFake{}, &trasherFake{}, &runnerFake{}).Execute(t.Context(), plan)
	if !errors.Is(err, domain.ErrNeedsRoot) {
		t.Errorf("erro = %v, quer ErrNeedsRoot", err)
	}
}

func TestBuildPlanDescartaAlvosQueExigemRoot(t *testing.T) {
	t.Parallel()

	comRoot := domain.Result{
		Rule: domain.Rule{
			ID:       "system-wallpapers",
			Strategy: domain.ManualOnly{Command: "sudo rm -rf /Library/..."},
		},
		Finding: domain.Finding{RuleID: "system-wallpapers", Size: 39_000_000_000},
	}

	plan := service.BuildPlan([]domain.Result{
		regraLixeira("caches", "/home/ana/Library/Caches/a"),
		comRoot,
	})

	if len(plan.Items) != 1 || plan.Items[0].Rule.ID != "caches" {
		t.Errorf("plano = %v, quer apenas a regra 'caches'", plan.Items)
	}
	if manual := service.ManualItems([]domain.Result{comRoot}); len(manual) != 1 {
		t.Error("ManualItems deveria devolver o alvo que exige root, para exibição")
	}
}

func TestCleanerDryRunNaoRemoveNada(t *testing.T) {
	t.Parallel()

	trasher, runner := &trasherFake{}, &runnerFake{}
	cleaner := service.NewCleaner(&guardFake{}, trasher, runner, service.WithDryRun(true))

	plan := service.BuildPlan([]domain.Result{
		regraLixeira("caches", "/home/ana/Library/Caches/a"),
		regraComando("docker-images", "docker", "image", "prune"),
	})

	if _, err := cleaner.Execute(t.Context(), plan); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(trasher.movidos) != 0 || len(runner.executados) != 0 {
		t.Errorf("simulação removeu de verdade: lixeira=%v comandos=%v",
			trasher.movidos, runner.executados)
	}
}

// TestCleanerContinuaAposFalhaEmUmAlvo cobre a corrida real: um diretório some
// entre a medição e a limpeza. Isso não pode impedir que os outros doze alvos da
// mesma regra sejam removidos.
func TestCleanerContinuaAposFalhaEmUmAlvo(t *testing.T) {
	t.Parallel()

	sumiu := "/home/ana/Library/Caches/sumiu"
	trasher := &trasherFake{falhar: map[string]error{sumiu: errors.New("não existe")}}

	plan := service.BuildPlan([]domain.Result{
		regraLixeira("caches", "/home/ana/Library/Caches/a", sumiu, "/home/ana/Library/Caches/b"),
	})

	summary, err := service.NewCleaner(&guardFake{}, trasher, &runnerFake{}).Execute(t.Context(), plan)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	quer := []string{"/home/ana/Library/Caches/a", "/home/ana/Library/Caches/b"}
	if !iguais(trasher.movidos, quer) {
		t.Errorf("movidos = %v, quer %v", trasher.movidos, quer)
	}
	if len(summary.Failures()) != 1 {
		t.Errorf("a falha deveria ser reportada, não engolida: %v", summary.Failures())
	}
}

func TestCleanerRespeitaCancelamento(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	plan := service.BuildPlan([]domain.Result{
		regraLixeira("caches", "/home/ana/Library/Caches/a"),
	})

	trasher := &trasherFake{}
	_, err := service.NewCleaner(&guardFake{}, trasher, &runnerFake{}).Execute(ctx, plan)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("erro = %v, quer context.Canceled", err)
	}
	if len(trasher.movidos) != 0 {
		t.Errorf("removeu apesar do cancelamento: %v", trasher.movidos)
	}
}

func iguais(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
