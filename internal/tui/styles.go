package tui

import "github.com/charmbracelet/lipgloss"

// A paleta usa cores ANSI adaptativas em vez de valores fixos: o terminal do
// usuário pode estar em tema claro ou escuro, e um cinza escolhido para fundo
// preto vira invisível sobre fundo branco.
var (
	styleTitle = lipgloss.NewStyle().Bold(true)

	styleSubtle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#6C6C6C", Dark: "#9E9E9E"})

	styleCategory = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#005F87", Dark: "#5FAFD7"})

	styleCursor = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#AF5F00", Dark: "#FFAF5F"})

	styleSize = lipgloss.NewStyle().Bold(true)

	styleWarn = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#AF0000", Dark: "#FF8787"})

	styleOK = lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.AdaptiveColor{Light: "#005F00", Dark: "#87D787"})

	styleHelp = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#8A8A8A", Dark: "#767676"})
)

// spinnerFrames anima a espera durante a varredura.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func spinner(frame int) string {
	return spinnerFrames[frame%len(spinnerFrames)]
}
