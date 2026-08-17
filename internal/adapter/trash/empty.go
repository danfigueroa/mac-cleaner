package trash

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Empty esvazia a Lixeira do usuário.
//
// Este é o passo que efetivamente devolve o espaço ao disco. Mover para a
// Lixeira apenas realoca os arquivos dentro do mesmo volume — sem esvaziar, o
// `df` não muda em um byte, e o usuário conclui com razão que a limpeza não fez
// nada.
//
// Delegamos ao Finder em vez de apagar ~/.Trash na mão porque a Lixeira não é um
// diretório só: cada volume montado tem a sua, e o Finder é quem sabe de todas.
func (Trasher) Empty(ctx context.Context) error {
	const script = `tell application "Finder" to empty trash`

	out, err := exec.CommandContext(ctx, "osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("esvaziando a Lixeira: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
