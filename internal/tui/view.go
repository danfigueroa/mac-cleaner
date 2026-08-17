package tui

import (
	"fmt"
	"strings"

	"github.com/danfigueroa/mac-cleaner/internal/domain"
)

// View desenha a tela inteira a cada quadro.
func (m Model) View() string {
	switch m.phase {
	case phaseScanning:
		return m.viewScanning()
	case phaseChoosing:
		return m.viewChoosing()
	case phaseCleaning:
		return m.viewCleaning()
	case phaseDone:
		return m.viewDone()
	case phaseFailed:
		return "" // o erro é impresso pela CLI, com o código de saída certo
	}
	return ""
}

func (m Model) viewScanning() string {
	return fmt.Sprintf("\n  %s medindo %d alvos no disco...\n\n",
		spinner(m.frame), len(m.rules))
}

func (m Model) viewCleaning() string {
	return fmt.Sprintf("\n  %s limpando...\n\n", spinner(m.frame))
}

func (m Model) viewChoosing() string {
	var out strings.Builder

	out.WriteString("\n")
	out.WriteString(m.header())
	out.WriteString("\n")

	for i, r := range m.rows {
		if r.isHeader() {
			fmt.Fprintf(&out, "\n  %s  %s\n",
				styleCategory.Render(strings.ToUpper(r.category.Title())),
				styleSubtle.Render(r.total.String()))
			continue
		}
		out.WriteString(m.renderRow(i, r))
	}

	out.WriteString(m.footer())
	return out.String()
}

func (m Model) header() string {
	volume := m.report.Volume
	return fmt.Sprintf("  %s\n  %s\n",
		styleTitle.Render(fmt.Sprintf("mac-cleaner · %s livres de %s (%.0f%% ocupado)",
			volume.Free, volume.Total, volume.UsedPercent())),
		styleSubtle.Render(fmt.Sprintf("%s recuperáveis em %d alvos",
			m.report.Reclaimable(), len(m.report.Results))))
}

// renderRow desenha um alvo. A coluna de tamanho vem antes do nome porque é ela
// que o usuário está comparando ao escolher o que limpar.
func (m Model) renderRow(index int, r row) string {
	rule := r.result.Rule

	cursor := "  "
	if index == m.cursor {
		cursor = styleCursor.Render("▸ ")
	}

	checkbox := "[ ]"
	switch {
	case !rule.Removable():
		// Não é marcável: ou exige root, e a CLI nunca escala privilégio, ou é
		// uma lista informativa, como a de arquivos grandes. Nos dois casos a
		// ação sugerida aparece na linha de detalhe, sob o cursor.
		checkbox = styleSubtle.Render("[-]")
	case m.chosen[rule.ID]:
		checkbox = styleOK.Render("[x]")
	}

	line := fmt.Sprintf("  %s%s %s  %-52s %s\n",
		cursor,
		checkbox,
		styleSize.Render(fmt.Sprintf("%9s", r.result.Finding.Size)),
		truncate(rule.Name, 52),
		riskHint(rule),
	)

	// O detalhe completo aparece só na linha sob o cursor. Mostrar as três
	// frases de todos os alvos ao mesmo tempo tornaria a lista ilegível; não
	// mostrá-las nunca devolveria o usuário à caixa-preta.
	if index == m.cursor {
		line += styleSubtle.Render(fmt.Sprintf(
			"        %s\n        você perde: %s\n        ação: %s\n",
			rule.What, rule.Lose, domain.CommandPreview(*r.result)))
	}
	return line
}

func riskHint(rule domain.Rule) string {
	if rule.NeedsRoot() {
		return styleSubtle.Render("requer sudo")
	}
	if !rule.Removable() {
		return styleSubtle.Render("só listagem")
	}
	switch rule.Risk {
	case domain.RiskRegenerable:
		return styleSubtle.Render("regenerável")
	case domain.RiskReview:
		return styleWarn.Render("revisar")
	case domain.RiskSafe:
		return ""
	}
	return ""
}

func (m Model) footer() string {
	var out strings.Builder

	out.WriteString("\n  " + strings.Repeat("─", 70) + "\n")
	fmt.Fprintf(&out, "  selecionado: %s\n", styleSize.Render(m.selectedTotal().String()))

	if len(m.report.DeniedPaths) > 0 {
		fmt.Fprintf(&out, "  %s\n", styleWarn.Render(fmt.Sprintf(
			"%d caminhos ilegíveis: os totais são um piso. "+
				"Conceda Acesso Total ao Disco ao terminal.", len(m.report.DeniedPaths))))
	}

	out.WriteString("  " + styleHelp.Render(strings.Join([]string{
		keyUp.help, keyToggle.help, keySelectAll.help,
		keySelectNone.help, keyConfirm.help, keyQuit.help,
	}, " · ")) + "\n")

	return out.String()
}

func (m Model) viewDone() string {
	var out strings.Builder

	if len(m.rows) == 0 {
		return "\n  Nada encontrado. O disco já está limpo segundo o catálogo.\n\n"
	}

	out.WriteString("\n  " + styleOK.Render("Limpeza concluída.") + "\n\n")
	fmt.Fprintf(&out, "  Liberado agora:       %s\n", styleSize.Render(m.summary.Reclaimed().String()))

	if trashed := m.summary.Trashed(); trashed > 0 {
		fmt.Fprintf(&out, "  Movido para a Lixeira: %s\n", styleSize.Render(trashed.String()))

		if m.trashEmptied {
			out.WriteString("\n  " + styleOK.Render("Lixeira esvaziada — o espaço já aparece livre.") + "\n")
		} else {
			// O aviso mais importante da tela. Sem esvaziar, o `df` não muda em
			// um byte e a limpeza parece não ter funcionado.
			out.WriteString("\n  " + styleWarn.Render(
				"Esse espaço ainda está ocupado: mover para a Lixeira não libera disco.") + "\n")
			out.WriteString("  " + styleHelp.Render("esvaziar a Lixeira agora? s/N") + "\n")
		}
	}

	for _, failure := range m.summary.Failures() {
		fmt.Fprintf(&out, "  %s\n", styleWarn.Render(
			fmt.Sprintf("falhou em %s: %v", failure.Rule.Name, failure.Err)))
	}

	out.WriteString("\n")
	return out.String()
}

// truncate corta preservando runes, para não partir um caractere acentuado ao
// meio e imprimir lixo no terminal.
func truncate(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit-1]) + "…"
}
