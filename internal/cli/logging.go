package cli

import (
	"io"
	"log/slog"
)

// setupLogger instala o logger global em stderr.
//
// Este é o único ponto do programa que usa estado global do slog, e é
// deliberado: um logger é infraestrutura transversal, e enfiá-lo por parâmetro
// em toda assinatura só para evitar um default polui a API sem ganho. Os
// serviços aceitam um *slog.Logger injetado e caem neste quando recebem nil.
//
// Sempre stderr: stdout carrega a saída de dados (`report --json`) e precisa
// permanecer limpo para pipe.
func setupLogger(w io.Writer, verbose bool) {
	level := slog.LevelWarn
	if verbose {
		level = slog.LevelDebug
	}

	handler := slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}
