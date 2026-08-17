package domain

import (
	"fmt"
	"strings"
)

// CommandPreview descreve, em uma linha, a ação que será executada.
//
// A CLI mostra isto antes de pedir confirmação. É o equivalente ao passo do
// fluxo manual em que o agente exibia o comando exato antes de agir — a
// diferença entre aprovar uma ação e autorizar uma caixa-preta.
func CommandPreview(result Result) string {
	switch strategy := result.Rule.Strategy.(type) {
	case RunCommand:
		return strategy.String()

	case ManualOnly:
		return strategy.Command

	case ReportOnly:
		return strategy.Hint

	case TrashTargets:
		count := len(result.Finding.Targets)
		switch count {
		case 0:
			return "nada a mover"
		case 1:
			return "mover para a Lixeira: " + result.Finding.Targets[0].Path
		default:
			return fmt.Sprintf("mover %d caminhos para a Lixeira", count)
		}

	default:
		return "ação desconhecida"
	}
}

// String devolve o comando como seria digitado no terminal.
func (c RunCommand) String() string {
	if len(c.Args) == 0 {
		return c.Name
	}
	return c.Name + " " + strings.Join(c.Args, " ")
}
