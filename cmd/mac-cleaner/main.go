// Command mac-cleaner audita o armazenamento de um Mac e ajuda a recuperar
// espaço ocupado por caches e resíduos regeneráveis.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/danfigueroa/mac-cleaner/internal/cli"
)

// main não contém lógica: delega para run e traduz o erro em código de saída.
// O os.Exit fica sozinho aqui para que os defers de run() de fato executem —
// os.Exit no meio do programa os ignora silenciosamente.
func main() {
	os.Exit(run())
}

func run() int {
	// Ctrl-C precisa cancelar um scan em andamento e os comandos externos que
	// ele disparou. Sem isso, interromper a CLI deixa `docker system prune`
	// rodando órfão.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cli.Execute(ctx, os.Args[1:]); err != nil {
		if msg := cli.UserMessage(err); msg != "" {
			fmt.Fprintln(os.Stderr, msg)
		}
		return cli.ExitCode(err)
	}
	return 0
}
