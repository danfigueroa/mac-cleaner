package cli

import (
	"context"
	"errors"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/danfigueroa/mac-cleaner/internal/report"
	"github.com/danfigueroa/mac-cleaner/internal/tui"
)

// runInteractive é o que acontece ao digitar `mac-cleaner` sem argumentos.
func runInteractive(cmd *cobra.Command, opts *options) error {
	// Sem terminal — saída redirecionada para arquivo ou cano — uma TUI só
	// produziria códigos de escape ilegíveis. O relatório em texto é a resposta
	// certa para `mac-cleaner > auditoria.txt`.
	if !isTerminal(os.Stdout) {
		rep, err := scanForReport(cmd.Context(), cmd.ErrOrStderr(), opts)
		if err != nil {
			return err
		}
		return report.Text(cmd.OutOrStdout(), rep)
	}

	rules, err := selectedRules(opts)
	if err != nil {
		return err
	}

	dependencies, err := buildDeps(false)
	if err != nil {
		return err
	}

	model := tui.New(tui.Config{
		Ctx:     cmd.Context(),
		Scanner: dependencies.scanner,
		Cleaner: dependencies.cleaner,
		Trasher: dependencies.trasher,
		Env:     dependencies.env,
		Rules:   rules,
		MinSize: opts.minSize,
	})

	program := tea.NewProgram(model, tea.WithContext(cmd.Context()))

	final, err := program.Run()
	if err != nil {
		// O Bubble Tea traduz o cancelamento do contexto no seu próprio erro.
		// Reconvertê-lo é o que faz o Ctrl-C sair com 130 em vez de 1.
		if errors.Is(err, tea.ErrProgramKilled) {
			return context.Canceled
		}
		return err
	}

	// O Bubble Tea não distingue "encerrou porque terminou" de "encerrou porque
	// falhou"; sem consultar o modelo final, um erro de limpeza sairia com
	// código zero.
	if model, ok := final.(tui.Model); ok {
		return model.Err()
	}
	return nil
}

// isTerminal informa se o arquivo é um terminal interativo.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
