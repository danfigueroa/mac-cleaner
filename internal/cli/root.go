package cli

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
)

const rootLongHelp = `mac-cleaner audita o armazenamento do seu Mac e ajuda a recuperar espaço
ocupado por caches e resíduos regeneráveis.

Cada alvo encontrado explica o que é, o que você perde ao removê-lo e como ele
volta. A remoção prefere sempre o comando oficial da própria ferramenta
(docker system prune, xcrun simctl delete) e, quando não existe um, move para a
Lixeira em vez de apagar.

A CLI nunca usa sudo. Alvos que exigem root são medidos e exibidos com o comando
pronto para você executar, se quiser.

Códigos de saída:
  0  sucesso
  1  erro
  2  uso inválido
  3  nada a limpar
  130 interrompido com Ctrl-C`

// options reúne as flags compartilhadas entre os subcomandos.
//
// Uma struct única passada por ponteiro aos construtores de comando mantém o
// pacote sem variáveis de pacote mutáveis — cada comando declara o que lê, e
// dois testes podem montar árvores de comando independentes sem interferir.
type options struct {
	verbose bool
	rescan  bool

	categories []string
	minSize    domain.Bytes
	bigFiles   domain.Bytes
	stale      time.Duration
}

// Padrões escolhidos para que a primeira execução já seja útil sem flags.
const (
	defaultMinSize  = 10 * domain.Megabyte // abaixo disso não vale a linha na tela
	defaultBigFiles = domain.Gigabyte
	defaultStale    = 90 * 24 * time.Hour
)

func newRootCommand() *cobra.Command {
	opts := &options{
		minSize:  defaultMinSize,
		bigFiles: defaultBigFiles,
		stale:    defaultStale,
	}

	root := &cobra.Command{
		Use:   "mac-cleaner",
		Short: "Audita o armazenamento do Mac e recupera espaço com segurança",
		Long:  rootLongHelp,
		Args:  cobra.NoArgs,

		// A CLI reporta seus próprios erros em main, com controle sobre o código
		// de saída. Deixar o Cobra fazer isso duplicaria a mensagem.
		SilenceErrors: true,
		// Uso só é exibido em erro de parsing, não quando o comando roda e falha.
		SilenceUsage: true,

		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			// Log estruturado sempre em stderr, para que stdout permaneça
			// pipeable: `mac-cleaner report --json | jq`.
			setupLogger(cmd.ErrOrStderr(), opts.verbose)
		},

		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInteractive(cmd, opts)
		},
	}

	flags := root.PersistentFlags()
	flags.BoolVarP(&opts.verbose, "verbose", "v", false, "log detalhado em stderr")
	flags.BoolVar(&opts.rescan, "rescan", false, "ignora o cache de varredura e mede tudo de novo")
	flags.StringSliceVar(&opts.categories, "category", nil,
		"limita às categorias indicadas (dev, system, apps, projects, bigfiles)")
	flags.Var(&opts.minSize, "min-size", "omite alvos menores que este tamanho")
	flags.Var(&opts.bigFiles, "big-files", "tamanho mínimo para listar um arquivo solto")
	flags.Var(newDurationValue(&opts.stale), "stale",
		"idade a partir da qual um projeto é considerado abandonado (ex: 90d)")

	root.AddCommand(
		newCleanCommand(opts),
		newReportCommand(opts),
		newVersionCommand(),
	)

	return root
}
