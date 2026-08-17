package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"runtime"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
)

// Scanner mede o espaço ocupado pelos alvos do catálogo.
type Scanner struct {
	fs      FileSystem
	volumer Volumer
	logger  *slog.Logger

	// sem é o teto de paralelismo compartilhado por todas as travessias. Um
	// walker sem limite abre uma goroutine por diretório e a máquina passa a
	// gastar mais tempo em troca de contexto do que em syscall — o limite é o
	// que torna o scan rápido, não o contrário.
	sem chan struct{}

	// ruleLimit controla quantas regras são medidas ao mesmo tempo.
	ruleLimit int
}

type scannerConfig struct {
	concurrency int
	logger      *slog.Logger
}

// ScannerOption configura um Scanner.
type ScannerOption func(*scannerConfig)

// WithConcurrency define o teto de travessias simultâneas.
func WithConcurrency(n int) ScannerOption {
	return func(c *scannerConfig) {
		if n > 0 {
			c.concurrency = n
		}
	}
}

// WithLogger injeta o logger. Sem ele, o Scanner usa slog.Default().
func WithLogger(l *slog.Logger) ScannerOption {
	return func(c *scannerConfig) {
		if l != nil {
			c.logger = l
		}
	}
}

// NewScanner monta um Scanner.
func NewScanner(filesystem FileSystem, volumer Volumer, opts ...ScannerOption) *Scanner {
	cfg := scannerConfig{
		// I/O de disco é o gargalo, não CPU, então vale passar do número de
		// núcleos: enquanto uma goroutine espera o syscall, outra avança.
		concurrency: runtime.NumCPU() * 2,
		logger:      slog.Default(),
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	return &Scanner{
		fs:        filesystem,
		volumer:   volumer,
		logger:    cfg.logger,
		sem:       make(chan struct{}, cfg.concurrency),
		ruleLimit: cfg.concurrency,
	}
}

// Scan avalia as regras aplicáveis e monta o relatório completo.
func (s *Scanner) Scan(ctx context.Context, env domain.Env, rules []domain.Rule) (domain.Report, error) {
	started := time.Now()

	volume, err := s.volumer.Volume(env.Home)
	if err != nil {
		return domain.Report{}, fmt.Errorf("lendo o volume de %s: %w", env.Home, err)
	}

	results, denied, err := s.measureRules(ctx, env, rules)
	if err != nil {
		return domain.Report{}, err
	}

	report := domain.Report{
		GeneratedAt: time.Now(),
		Volume:      volume,
		Results:     results,
		DeniedPaths: denied,
	}
	report.SortBySize()

	s.logger.Debug("varredura concluída",
		"duração", time.Since(started),
		"regras", len(results),
		"recuperável", report.Reclaimable().String(),
		"ilegíveis", len(denied))

	return report, nil
}

// measureRules mede cada regra em paralelo, preservando a ordem do catálogo.
func (s *Scanner) measureRules(
	ctx context.Context, env domain.Env, rules []domain.Rule,
) ([]domain.Result, []string, error) {
	type slot struct {
		result domain.Result
		denied []string
		filled bool
	}

	// Um slot por regra, escrito só pela goroutine da sua posição: sem mutex e
	// sem resultado dependente de quem terminou primeiro.
	slots := make([]slot, len(rules))

	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(s.ruleLimit)

	for i, rule := range rules {
		group.Go(func() error {
			if rule.Detect != nil && !rule.Detect(env) {
				s.logger.Debug("regra não se aplica a esta máquina", "regra", rule.ID)
				return nil
			}

			finding, denied, err := s.measureRule(groupCtx, env, rule)
			if err != nil {
				return fmt.Errorf("medindo a regra %s: %w", rule.ID, err)
			}
			if finding.Empty() {
				return nil
			}

			slots[i] = slot{
				result: domain.Result{Rule: rule, Finding: finding},
				denied: denied,
				filled: true,
			}
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return nil, nil, err
	}

	results := make([]domain.Result, 0, len(slots))
	var denied []string
	for _, sl := range slots {
		if !sl.filled {
			continue
		}
		results = append(results, sl.result)
		denied = append(denied, sl.denied...)
	}
	return results, denied, nil
}

// measureRule resolve os alvos de uma regra e mede cada um.
func (s *Scanner) measureRule(
	ctx context.Context, env domain.Env, rule domain.Rule,
) (domain.Finding, []string, error) {
	if rule.Targets == nil {
		return domain.Finding{}, nil, fmt.Errorf("regra %s não define Targets", rule.ID)
	}

	targets, err := rule.Targets(ctx, env)
	if err != nil {
		return domain.Finding{}, nil, err
	}

	finding := domain.Finding{RuleID: rule.ID}
	measured := make([]domain.Target, 0, len(targets))
	var denied []string

	for _, target := range targets {
		// Regras dinâmicas já descobrem o tamanho enquanto varrem; medi-las de
		// novo seria percorrer o disco duas vezes.
		if target.Measured {
			finding.Size += target.Size
			measured = append(measured, target)
			continue
		}

		size, targetDenied, err := s.MeasurePath(ctx, target.Path)
		if err != nil {
			return domain.Finding{}, nil, err
		}
		denied = append(denied, targetDenied...)

		// Caminho inexistente ou vazio não vira linha na tela.
		if size == 0 {
			continue
		}

		target.Size = size
		target.Measured = true
		finding.Size += size
		measured = append(measured, target)
	}

	finding.Targets = measured
	finding.Denied = len(denied)
	return finding, denied, nil
}

// MeasurePath devolve o espaço ocupado por um caminho, os caminhos ilegíveis
// encontrados no caminho, e erro apenas quando algo realmente inesperado ocorre.
//
// A deduplicação de hardlink vale por chamada. Dois alvos distintos que
// compartilhem o mesmo inode físico são contados duas vezes — o mesmo que
// acontece ao rodar `du` separadamente em cada um.
func (s *Scanner) MeasurePath(ctx context.Context, path string) (domain.Bytes, []string, error) {
	info, err := s.fs.Lstat(path)
	switch {
	case err == nil:
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, syscall.ENOTDIR):
		// A ferramenta não está instalada, ou o cache nunca foi criado.
		//
		// ENOTDIR entra aqui junto com ENOENT porque significa a mesma coisa do
		// ponto de vista da medição: o caminho não existe como o alvo esperava.
		// Acontece quando um componente intermediário é um arquivo, e uma regra
		// que gere um caminho assim não pode derrubar a varredura das outras 34.
		return 0, nil, nil
	case errors.Is(err, fs.ErrPermission):
		return 0, []string{path}, nil
	default:
		return 0, nil, fmt.Errorf("medindo %s: %w", path, err)
	}

	if !info.IsDir {
		return info.Size, nil, nil
	}

	state := &treeState{
		device: info.Device,
		seen:   make(map[uint64]struct{}),
		sem:    s.sem,
	}

	result, err := s.walkDir(ctx, path, state)
	if err != nil {
		return 0, nil, err
	}
	// O próprio diretório raiz também ocupa blocos.
	return result.size + info.Size, result.denied, nil
}
