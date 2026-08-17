//go:build !cgo

// Package trash move arquivos para a Lixeira do macOS.
package trash

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// osascriptTimeout evita que o Finder pendurado trave a limpeza inteira.
const osascriptTimeout = 30 * time.Second

// Trasher move caminhos para a Lixeira pedindo ao Finder.
//
// Este é o caminho de compatibilidade, usado só quando o binário é compilado sem
// cgo. É bem mais lento — cada chamada conversa com o Finder via Apple Events —
// mas preserva a propriedade que importa: o item vai para a Lixeira de verdade,
// com "Colocar de Volta" funcionando, em vez de ser movido na marra para
// ~/.Trash sem metadado nenhum.
type Trasher struct{}

// New devolve um Trasher.
func New() Trasher { return Trasher{} }

// Trash move o caminho para a Lixeira do usuário.
//
// Importante para quem chama: isto NÃO libera espaço em disco enquanto a Lixeira
// não for esvaziada.
func (Trasher) Trash(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), osascriptTimeout)
	defer cancel()

	// O caminho entra como um literal AppleScript, então aspas e barras
	// invertidas precisam ser escapadas — nomes de diretório de cache contêm
	// aspas com mais frequência do que se imagina.
	quoted := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(path)
	script := fmt.Sprintf(`tell application "Finder" to delete POSIX file "%s"`, quoted)

	out, err := exec.CommandContext(ctx, "osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("movendo %s para a Lixeira via Finder: %w: %s",
			path, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Native informa que esta implementação não usa a API do sistema diretamente.
func (Trasher) Native() bool { return false }
