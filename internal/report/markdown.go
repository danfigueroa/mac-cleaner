package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
)

// Markdown escreve a auditoria como documento.
//
// É a saída equivalente ao relatório estruturado que o fluxo manual produzia
// antes de qualquer remoção: serve para revisar com calma, versionar num
// repositório ou comparar o antes e o depois. Diferente do JSON, cabe numa
// conversa.
func Markdown(w io.Writer, rep domain.Report) error {
	var out strings.Builder

	out.WriteString("# Auditoria de armazenamento\n\n")
	fmt.Fprintf(&out, "_Gerado em %s_\n\n", rep.GeneratedAt.Format("02/01/2006 15:04"))

	fmt.Fprintf(&out, "**Disco `%s`** — %s livres de %s (%.0f%% ocupado).\n\n",
		rep.Volume.Path, rep.Volume.Free, rep.Volume.Total, rep.Volume.UsedPercent())

	if len(rep.Results) == 0 {
		out.WriteString("Nenhum alvo encontrado. O disco já está limpo segundo o catálogo.\n")
		_, err := io.WriteString(w, out.String())
		return err
	}

	fmt.Fprintf(&out, "**Recuperável: %s** em %d alvos.\n", rep.Reclaimable(), len(rep.Results))

	for _, group := range rep.GroupByCategory() {
		fmt.Fprintf(&out, "\n## %s — %s\n\n", group.Category.Title(), group.Total)

		for _, result := range group.Results {
			writeMarkdownEntry(&out, result)
		}
	}

	if len(rep.DeniedPaths) > 0 {
		fmt.Fprintf(&out, "\n---\n\n> **Os totais acima são um piso.** %d caminhos não puderam "+
			"ser lidos por falta de permissão. Conceda Acesso Total ao Disco ao seu terminal "+
			"em Ajustes do Sistema › Privacidade e Segurança e rode de novo.\n",
			len(rep.DeniedPaths))
	}

	_, err := io.WriteString(w, out.String())
	return err
}

// writeMarkdownEntry escreve um alvo com as três frases que permitem decidir
// sobre ele. É o formato longo de propósito — quem pediu markdown quer ler, não
// escanear uma tabela.
func writeMarkdownEntry(out *strings.Builder, result domain.Result) {
	fmt.Fprintf(out, "### %s — %s\n\n", result.Rule.Name, result.Finding.Size)

	fmt.Fprintf(out, "- **O que é:** %s\n", result.Rule.What)
	fmt.Fprintf(out, "- **O que você perde:** %s\n", result.Rule.Lose)
	fmt.Fprintf(out, "- **Como volta:** %s\n", result.Rule.Regen)
	fmt.Fprintf(out, "- **Risco:** %s\n", result.Rule.Risk)

	if result.Rule.NeedsRoot() {
		out.WriteString("- **Exige sudo:** a CLI não executa este comando.\n")
	}

	fmt.Fprintf(out, "\n```sh\n%s\n```\n\n", domain.CommandPreview(result))

	if paths := result.Finding.Paths(); len(paths) > 0 {
		out.WriteString("<details><summary>Caminhos</summary>\n\n")
		for _, path := range paths {
			fmt.Fprintf(out, "- `%s`\n", path)
		}
		out.WriteString("\n</details>\n\n")
	}
}
