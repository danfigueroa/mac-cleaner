package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
	"github.com/danfigueroa/mac-cleaner/internal/report"
)

func newReportCommand(opts *options) *cobra.Command {
	var asJSON, asMarkdown bool

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Audita o disco e imprime o relatório, sem remover nada",
		Long: `Mede todos os alvos do catálogo e imprime o que encontrou.

Este comando nunca remove nada. É o passo de auditoria: serve para entender onde
o espaço foi parar antes de decidir o que fazer.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			rep, err := scanForReport(cmd.Context(), cmd.ErrOrStderr(), opts)
			if err != nil {
				return err
			}

			switch {
			case asJSON:
				return report.JSON(cmd.OutOrStdout(), rep)
			case asMarkdown:
				return report.Markdown(cmd.OutOrStdout(), rep)
			default:
				return report.Text(cmd.OutOrStdout(), rep)
			}
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "imprime o relatório como JSON")
	cmd.Flags().BoolVar(&asMarkdown, "markdown", false, "imprime o relatório como Markdown")
	cmd.MarkFlagsMutuallyExclusive("json", "markdown")

	return cmd
}

// scanForReport executa a varredura aplicando os filtros globais.
//
// Fica separado do comando porque a TUI faz exatamente o mesmo passo antes de
// desenhar a lista — e é importante que os dois caminhos vejam o mesmo conjunto
// de alvos, senão o relatório e a tela interativa passam a discordar.
func scanForReport(ctx context.Context, progress io.Writer, opts *options) (domain.Report, error) {
	dependencies, err := buildDeps(false)
	if err != nil {
		return domain.Report{}, err
	}

	rules, err := selectedRules(opts)
	if err != nil {
		return domain.Report{}, err
	}

	// O aviso de progresso vai para stderr: a varredura leva alguns segundos e
	// um terminal parado parece travado, mas stdout precisa continuar limpo para
	// `mac-cleaner report --json | jq`.
	fmt.Fprintf(progress, "Medindo %d alvos...\n", len(rules))

	rep, err := dependencies.scanner.Scan(ctx, dependencies.env, rules)
	if err != nil {
		return domain.Report{}, err
	}

	rep.FilterBySize(opts.minSize)
	return rep, nil
}
