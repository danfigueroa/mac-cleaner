package report

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
)

// maxDeniedExamples limita quantos caminhos ilegíveis são listados. Uma lista de
// centenas afogaria a instrução que realmente resolve o problema.
const maxDeniedExamples = 3

// Text escreve o relatório para leitura no terminal.
func Text(w io.Writer, rep domain.Report) error {
	var out strings.Builder

	writeVolume(&out, rep.Volume)

	if len(rep.Results) == 0 {
		out.WriteString("\nNenhum alvo encontrado. O disco já está limpo segundo o catálogo.\n")
		writeDeniedWarning(&out, rep.DeniedPaths)
		_, err := io.WriteString(w, out.String())
		return err
	}

	fmt.Fprintf(&out, "\nRecuperável: %s em %d alvos\n", rep.Reclaimable(), len(rep.Results))

	for _, group := range rep.GroupByCategory() {
		writeGroup(&out, group)
	}

	writeManualSection(&out, rep)
	writeDeniedWarning(&out, rep.DeniedPaths)

	_, err := io.WriteString(w, out.String())
	return err
}

// writeVolume lidera com o espaço livre.
//
// É o número que o usuário veio conferir, e é o único dos três que bate
// exatamente com o `df`. A porcentagem também bate com a coluna Capacity dele.
func writeVolume(out *strings.Builder, volume domain.Volume) {
	fmt.Fprintf(out, "Disco %s — %s livres de %s (%.0f%% ocupado)\n",
		volume.Path, volume.Free, volume.Total, volume.UsedPercent())
}

func writeGroup(out *strings.Builder, group domain.CategoryGroup) {
	fmt.Fprintf(out, "\n%s — %s\n", strings.ToUpper(group.Category.Title()), group.Total)

	// tabwriter alinha as colunas pela largura real do conteúdo, o que importa
	// aqui porque os nomes das regras variam muito de tamanho e uma tabela
	// desalinhada é exatamente o tipo de coisa que faz alguém não ler os números.
	table := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, result := range group.Results {
		fmt.Fprintf(table, "  %s\t%s\t%s\t%s\n",
			result.Finding.Size,
			result.Rule.ID,
			result.Rule.Name,
			riskLabel(result.Rule),
		)
	}
	_ = table.Flush()
}

// riskLabel marca o que exige atenção. Alvos seguros não recebem rótulo: se tudo
// tem aviso, nada tem.
func riskLabel(rule domain.Rule) string {
	if rule.NeedsRoot() {
		return "requer sudo"
	}
	if !rule.Removable() {
		return "só listagem"
	}
	switch rule.Risk {
	case domain.RiskSafe:
		return ""
	case domain.RiskRegenerable:
		return "regenerável"
	case domain.RiskReview:
		return "revisar"
	default:
		return ""
	}
}

// writeManualSection lista o que a CLI não vai executar, com o comando pronto.
//
// A ferramenta nunca escala privilégio, mas esconder esses alvos seria pior:
// papéis de parede 4K do sistema chegam a dezenas de gigabytes, e o usuário
// precisa ao menos saber que estão lá.
func writeManualSection(out *strings.Builder, rep domain.Report) {
	var manual []domain.Result
	for _, result := range rep.Results {
		if !result.Rule.Removable() {
			manual = append(manual, result)
		}
	}
	if len(manual) == 0 {
		return
	}

	out.WriteString("\nA CLI NÃO REMOVE ESTES — a decisão fica com você:\n")
	for _, result := range manual {
		fmt.Fprintf(out, "  %s (%s)\n    %s\n",
			result.Rule.Name, result.Finding.Size, domain.CommandPreview(result))
	}
}

// writeDeniedWarning avisa que os números estão subestimados.
//
// É a diferença entre a ferramenta errar e a ferramenta mentir: sem Acesso Total
// ao Disco, partes de ~/Library respondem "permissão negada" e somem da conta
// sem deixar rastro. Quem lê o relatório precisa saber que o total é um piso.
func writeDeniedWarning(out *strings.Builder, denied []string) {
	if len(denied) == 0 {
		return
	}

	fmt.Fprintf(out, "\nAtenção: %d caminhos não puderam ser lidos, então os totais acima "+
		"estão subestimados.\n", len(denied))
	out.WriteString("  Conceda Acesso Total ao Disco ao seu terminal em\n")
	out.WriteString("  Ajustes do Sistema › Privacidade e Segurança › Acesso Total ao Disco.\n")

	for i, path := range denied {
		if i == maxDeniedExamples {
			fmt.Fprintf(out, "  ... e mais %d\n", len(denied)-maxDeniedExamples)
			break
		}
		fmt.Fprintf(out, "  · %s\n", path)
	}
}
