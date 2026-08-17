package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/danfigueroa/mac-cleaner/internal/catalog"
	"github.com/danfigueroa/mac-cleaner/internal/domain"
	"github.com/danfigueroa/mac-cleaner/internal/service"
)

type cleanFlags struct {
	assumeYes  bool
	dryRun     bool
	onlySafe   bool
	emptyTrash bool
}

func newCleanCommand(opts *options) *cobra.Command {
	flags := &cleanFlags{}

	cmd := &cobra.Command{
		Use:   "clean [regra...]",
		Short: "Remove os alvos indicados",
		Long: `Remove os alvos das regras indicadas, pedindo confirmação antes.

Use os identificadores que aparecem no relatório:

    mac-cleaner clean npm-cache go-buildcache

Ou limpe de uma vez tudo que é classificado como seguro:

    mac-cleaner clean --safe

Alvos sem comando oficial de limpeza vão para a Lixeira, e isso NÃO libera
espaço enquanto ela não for esvaziada. Use --empty-trash para esvaziar ao final.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runClean(cmd, args, opts, flags)
		},
	}

	cmd.Flags().BoolVarP(&flags.assumeYes, "yes", "y", false, "não pede confirmação")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, "mostra o que faria, sem remover nada")
	cmd.Flags().BoolVar(&flags.onlySafe, "safe", false, "seleciona todas as regras de risco seguro")
	cmd.Flags().BoolVar(&flags.emptyTrash, "empty-trash", false,
		"esvazia a Lixeira ao final, liberando o espaço de fato")

	return cmd
}

func runClean(cmd *cobra.Command, args []string, opts *options, flags *cleanFlags) error {
	ctx := cmd.Context()
	stdout, stderr := cmd.OutOrStdout(), cmd.ErrOrStderr()

	rules, err := selectRules(args, flags)
	if err != nil {
		return err
	}

	dependencies, err := buildDeps(flags.dryRun)
	if err != nil {
		return err
	}

	fmt.Fprintf(stderr, "Medindo %d alvos...\n", len(rules))
	report, err := dependencies.scanner.Scan(ctx, dependencies.env, rules)
	if err != nil {
		return err
	}

	plan := service.BuildPlan(report.Results)
	if plan.Empty() {
		fmt.Fprintln(stdout, "Nada a limpar nas regras selecionadas.")
		return domain.ErrNothingToClean
	}

	// Validar antes de mostrar o plano: se o guard vai barrar alguma coisa, o
	// usuário não deve nem ser convidado a aprovar.
	if invalid := dependencies.cleaner.Validate(plan); invalid != nil {
		return invalid
	}

	writePlan(stdout, report, plan)

	if !flags.assumeYes && !flags.dryRun {
		if !confirm(cmd.InOrStdin(), stdout, fmt.Sprintf("\nRemover %s?", plan.Total())) {
			fmt.Fprintln(stdout, "Cancelado. Nada foi removido.")
			return nil
		}
	}

	summary, err := dependencies.cleaner.Execute(ctx, plan)
	if err != nil {
		return err
	}

	writeSummary(stdout, summary, flags.dryRun)
	return finishTrash(ctx, cmd, dependencies, summary, flags)
}

// selectRules resolve o que limpar a partir dos argumentos ou de --safe.
func selectRules(args []string, flags *cleanFlags) ([]domain.Rule, error) {
	if flags.onlySafe {
		if len(args) > 0 {
			return nil, fmt.Errorf("%w: use --safe ou uma lista de regras, não os dois", errUsage)
		}

		var safe []domain.Rule
		for _, rule := range catalog.All() {
			// NeedsRoot fica de fora: a CLI não escala privilégio, e incluir
			// esses alvos num "--safe" só produziria erro na execução.
			if rule.Risk == domain.RiskSafe && rule.Removable() {
				safe = append(safe, rule)
			}
		}
		return safe, nil
	}

	if len(args) == 0 {
		return nil, fmt.Errorf("%w: informe ao menos uma regra, ou use --safe "+
			"(veja os identificadores com `mac-cleaner report`)", errUsage)
	}

	rules, err := catalog.ByIDs(args)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errUsage, err)
	}
	return rules, nil
}

// writePlan mostra exatamente o que vai acontecer, item a item.
//
// É o passo que o fluxo manual com um LLM fazia a cada etapa: exibir o comando
// literal antes de executá-lo. Sem isto, confirmar seria aprovar uma caixa-preta.
func writePlan(out io.Writer, report domain.Report, plan domain.Plan) {
	fmt.Fprintf(out, "\nSerão removidos %s:\n\n", plan.Total())

	for _, item := range plan.Items {
		result := domain.Result(item)

		fmt.Fprintf(out, "  %s — %s\n", item.Rule.Name, item.Finding.Size)
		fmt.Fprintf(out, "    o que é:  %s\n", item.Rule.What)
		fmt.Fprintf(out, "    você perde: %s\n", item.Rule.Lose)
		fmt.Fprintf(out, "    como volta: %s\n", item.Rule.Regen)
		fmt.Fprintf(out, "    ação:     %s\n\n", domain.CommandPreview(result))
	}

	if manual := service.ManualItems(report.Results); len(manual) > 0 {
		fmt.Fprintln(out, "Não serão tocados (exigem sudo):")
		for _, result := range manual {
			fmt.Fprintf(out, "  %s (%s)\n    %s\n",
				result.Rule.Name, result.Finding.Size, domain.CommandPreview(result))
		}
	}
}

func writeSummary(out io.Writer, summary domain.Summary, dryRun bool) {
	if dryRun {
		fmt.Fprintln(out, "\nSimulação concluída. Nada foi removido.")
		return
	}

	fmt.Fprintf(out, "\nLiberado agora: %s\n", summary.Reclaimed())

	if trashed := summary.Trashed(); trashed > 0 {
		fmt.Fprintf(out, "Movido para a Lixeira: %s "+
			"(ainda ocupa o disco até você esvaziá-la)\n", trashed)
	}

	for _, failure := range summary.Failures() {
		fmt.Fprintf(out, "Falhou em %s: %v\n", failure.Rule.Name, failure.Err)
	}
}

// finishTrash cuida do passo que de fato devolve o espaço ao disco.
//
// Sem ele, o usuário roda `df`, vê o mesmo número de antes e conclui que a
// ferramenta não fez nada — o que, do ponto de vista de espaço livre, é verdade.
func finishTrash(
	ctx context.Context, cmd *cobra.Command, dependencies *deps,
	summary domain.Summary, flags *cleanFlags,
) error {
	if flags.dryRun || summary.Trashed() == 0 {
		return nil
	}

	stdout := cmd.OutOrStdout()

	esvaziar := flags.emptyTrash
	if !esvaziar && !flags.assumeYes {
		esvaziar = confirm(cmd.InOrStdin(), stdout,
			"\nEsvaziar a Lixeira agora para liberar o espaço de verdade?")
	}

	if !esvaziar {
		fmt.Fprintln(stdout, "A Lixeira não foi esvaziada — o espaço ainda não aparece como livre.")
		return nil
	}

	if err := dependencies.trasher.Empty(ctx); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "Lixeira esvaziada.")
	return nil
}
