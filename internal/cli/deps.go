package cli

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/danfigueroa/mac-cleaner/internal/adapter/cmdrunner"
	"github.com/danfigueroa/mac-cleaner/internal/adapter/osfs"
	"github.com/danfigueroa/mac-cleaner/internal/adapter/trash"
	"github.com/danfigueroa/mac-cleaner/internal/domain"
	"github.com/danfigueroa/mac-cleaner/internal/guard"
	"github.com/danfigueroa/mac-cleaner/internal/service"
)

// deps reúne tudo que os comandos precisam.
//
// Este arquivo é o composition root: o único lugar do programa que sabe qual
// implementação concreta satisfaz cada interface. Todo o resto recebe o que
// precisa por parâmetro, o que é o que permite trocar o disco real por um fake
// nos testes sem nenhuma variável global no caminho.
type deps struct {
	env     domain.Env
	scanner *service.Scanner
	cleaner *service.Cleaner
	trasher trash.Trasher
	runner  *cmdrunner.Runner
}

func buildDeps(dryRun bool) (*deps, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("descobrindo o diretório do usuário: %w", err)
	}

	logger := slog.Default()
	runner := cmdrunner.New(cmdrunner.WithLogger(logger))

	// osfs.FS satisfaz FileSystem e Volumer. São duas interfaces porque são dois
	// papéis distintos — percorrer a árvore e consultar o volume — mesmo que
	// aqui um único tipo cumpra os dois.
	filesystem := osfs.New()
	trasher := trash.New()

	return &deps{
		env: domain.Env{
			Home:     home,
			Root:     "/",
			LookPath: runner.LookPath,
			Query:    runner.Query,
		},
		scanner: service.NewScanner(filesystem, filesystem, service.WithLogger(logger)),
		// O guard é construído a partir do home real. Ele é a única barreira
		// entre o catálogo e o disco, e por isso recebe o home explicitamente em
		// vez de descobri-lo sozinho.
		cleaner: service.NewCleaner(
			guard.New(home),
			trasher,
			runner,
			service.WithCleanerLogger(logger),
			service.WithDryRun(dryRun),
		),
		trasher: trasher,
		runner:  runner,
	}, nil
}
