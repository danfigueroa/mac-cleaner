// Package cmdrunner executa comandos externos.
//
// Separa consulta de execução em dois métodos com garantias diferentes: Query é
// somente-leitura, tem prazo curto e pode falhar em silêncio; Run altera o
// sistema e reporta tudo. Um daemon do Docker travado deve fazer uma regra
// desaparecer do relatório, não pendurar a varredura inteira.
package cmdrunner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// defaultQueryTimeout limita quanto tempo uma consulta pode segurar o scan.
//
// Quinze segundos é generoso para `docker system df` num daemon saudável e curto
// o bastante para que um daemon travado — situação comum no Docker Desktop — não
// transforme uma varredura de segundos numa espera indefinida.
const defaultQueryTimeout = 15 * time.Second

// Runner executa comandos no sistema.
type Runner struct {
	logger       *slog.Logger
	queryTimeout time.Duration
}

// Option configura o Runner.
type Option func(*Runner)

// WithLogger injeta o logger.
func WithLogger(l *slog.Logger) Option {
	return func(r *Runner) {
		if l != nil {
			r.logger = l
		}
	}
}

// WithQueryTimeout ajusta o prazo das consultas.
func WithQueryTimeout(d time.Duration) Option {
	return func(r *Runner) {
		if d > 0 {
			r.queryTimeout = d
		}
	}
}

// New monta um Runner.
func New(opts ...Option) *Runner {
	runner := &Runner{
		logger:       slog.Default(),
		queryTimeout: defaultQueryTimeout,
	}
	for _, opt := range opts {
		opt(runner)
	}
	return runner
}

// LookPath resolve um executável no PATH.
func (r *Runner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

// Query executa um comando de leitura e devolve a saída padrão.
func (r *Runner) Query(ctx context.Context, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)

	out, err := cmd.Output()
	if err != nil {
		r.logger.Debug("consulta falhou",
			"comando", commandLine(name, args), "erro", err)
		return nil, fmt.Errorf("consultando %s: %w", name, err)
	}
	return out, nil
}

// Run executa um comando que altera o sistema.
//
// Devolve a saída combinada porque, quando `docker image prune` ou `brew cleanup`
// falham, o motivo costuma estar em stderr — e essa mensagem precisa chegar ao
// usuário, não sumir num erro genérico de código de saída.
func (r *Runner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	line := commandLine(name, args)
	r.logger.Debug("executando", "comando", line)

	// CommandContext, e não Command: sem isso um Ctrl-C durante a limpeza
	// devolve o terminal ao usuário mas deixa o comando rodando órfão.
	cmd := exec.CommandContext(ctx, name, args...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return out, fmt.Errorf("`%s` terminou com código %d: %s",
				line, exitErr.ExitCode(), firstLine(out))
		}
		return out, fmt.Errorf("executando `%s`: %w", line, err)
	}
	return out, nil
}

func commandLine(name string, args []string) string {
	if len(args) == 0 {
		return name
	}
	return name + " " + strings.Join(args, " ")
}

// firstLine extrai a primeira linha não vazia da saída, que é onde comandos de
// linha costumam pôr a mensagem de erro relevante.
func firstLine(out []byte) string {
	for line := range strings.SplitSeq(string(out), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return "sem saída"
}
