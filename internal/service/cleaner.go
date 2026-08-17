package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
)

// PathGuard valida um caminho antes de qualquer remoção.
type PathGuard interface {
	Check(path string) error
}

// Trasher move um caminho para a Lixeira do usuário.
type Trasher interface {
	Trash(path string) error
}

// Runner executa um comando que altera o sistema.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// Cleaner executa um plano aprovado.
type Cleaner struct {
	guard   PathGuard
	trasher Trasher
	runner  Runner
	logger  *slog.Logger
	dryRun  bool
}

type cleanerConfig struct {
	logger *slog.Logger
	dryRun bool
}

// CleanerOption configura um Cleaner.
type CleanerOption func(*cleanerConfig)

// WithCleanerLogger injeta o logger.
func WithCleanerLogger(l *slog.Logger) CleanerOption {
	return func(c *cleanerConfig) {
		if l != nil {
			c.logger = l
		}
	}
}

// WithDryRun faz o Cleaner percorrer o plano inteiro sem remover nada.
func WithDryRun(enabled bool) CleanerOption {
	return func(c *cleanerConfig) { c.dryRun = enabled }
}

// NewCleaner monta um Cleaner.
func NewCleaner(guard PathGuard, trasher Trasher, runner Runner, opts ...CleanerOption) *Cleaner {
	cfg := cleanerConfig{logger: slog.Default()}
	for _, opt := range opts {
		opt(&cfg)
	}

	return &Cleaner{
		guard:   guard,
		trasher: trasher,
		runner:  runner,
		logger:  cfg.logger,
		dryRun:  cfg.dryRun,
	}
}

// Validate confere o plano inteiro antes de qualquer remoção.
//
// A verificação é separada da execução de propósito. O usuário aprovou um
// conjunto, não uma sequência: se o quinto item viola o guard, o certo é não ter
// removido o primeiro. Além disso, uma violação significa que o catálogo produziu
// um alvo que ninguém previu — e seguir limpando os outros itens seria confiar
// num componente que acabou de se mostrar errado.
func (c *Cleaner) Validate(plan domain.Plan) error {
	for _, item := range plan.Items {
		if _, manual := item.Rule.Strategy.(domain.ManualOnly); manual {
			return fmt.Errorf("%w: %s", domain.ErrNeedsRoot, item.Rule.ID)
		}
		if !item.Rule.Removable() {
			return fmt.Errorf("%w: a regra %s é apenas informativa",
				domain.ErrGuardViolation, item.Rule.ID)
		}

		if _, trashes := item.Rule.Strategy.(domain.TrashTargets); !trashes {
			continue
		}
		for _, target := range item.Finding.Targets {
			if err := c.guard.Check(target.Path); err != nil {
				return fmt.Errorf("regra %s: %w", item.Rule.ID, err)
			}
		}
	}
	return nil
}

// Execute roda o plano e devolve o que aconteceu com cada item.
//
// A execução é sequencial. Não é limitação: `docker image prune` e
// `brew cleanup` mexem em estado global das próprias ferramentas e não são
// seguros em paralelo, e uma limpeza destrutiva com a saída embaralhada entre
// itens é impossível de acompanhar quando algo dá errado.
func (c *Cleaner) Execute(ctx context.Context, plan domain.Plan) (domain.Summary, error) {
	if err := c.Validate(plan); err != nil {
		return domain.Summary{}, err
	}

	summary := domain.Summary{Outcomes: make([]domain.Outcome, 0, len(plan.Items))}

	for _, item := range plan.Items {
		if err := ctx.Err(); err != nil {
			// Interrompido pelo usuário. Devolvemos o que já foi feito para que
			// a interface possa reportar com honestidade o estado em que parou.
			return summary, err
		}

		summary.Outcomes = append(summary.Outcomes, c.executeItem(ctx, item))
	}
	return summary, nil
}

func (c *Cleaner) executeItem(ctx context.Context, item domain.PlanItem) domain.Outcome {
	outcome := domain.Outcome{Rule: item.Rule, Reclaimed: item.Finding.Size}

	switch strategy := item.Rule.Strategy.(type) {
	case domain.RunCommand:
		outcome.Err = c.runCommand(ctx, strategy)

	case domain.TrashTargets:
		outcome.Trashed = true
		outcome.Err = c.trashAll(ctx, item)

	default:
		outcome.Err = fmt.Errorf("regra %s tem estratégia não executável", item.Rule.ID)
	}

	if outcome.Err != nil {
		c.logger.Debug("item falhou", "regra", item.Rule.ID, "erro", outcome.Err)
	}
	return outcome
}

func (c *Cleaner) runCommand(ctx context.Context, cmd domain.RunCommand) error {
	if c.dryRun {
		c.logger.Info("simulação: executaria comando", "comando", cmd.String())
		return nil
	}

	_, err := c.runner.Run(ctx, cmd.Name, cmd.Args...)
	return err
}

// trashAll move os alvos de uma regra para a Lixeira.
//
// Falhas por alvo são acumuladas em vez de abortarem no primeiro erro: um
// diretório que sumiu entre a medição e a limpeza não deve impedir os outros
// doze de serem removidos.
func (c *Cleaner) trashAll(ctx context.Context, item domain.PlanItem) error {
	var failures []error

	for _, target := range item.Finding.Targets {
		if err := ctx.Err(); err != nil {
			return err
		}

		// Segunda checagem, depois da que Validate já fez. Barata, e cobre a
		// janela entre a validação do plano e este instante — que é justamente
		// onde um symlink recém-criado apareceria.
		if err := c.guard.Check(target.Path); err != nil {
			return err
		}

		if c.dryRun {
			c.logger.Info("simulação: moveria para a Lixeira", "caminho", target.Path)
			continue
		}

		if err := c.trasher.Trash(target.Path); err != nil {
			failures = append(failures, err)
		}
	}

	return errors.Join(failures...)
}
